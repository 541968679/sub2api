package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

// cursorResponsesUnsupportedFields are top-level Responses API parameters that
// Codex upstreams reject with "Unsupported parameter: ...". They must be
// stripped when forwarding a raw client body through the Responses-shape
// short-circuit in ForwardAsChatCompletions (see isResponsesShape branch).
// The normal Chat Completions → Responses conversion path is unaffected
// because ChatCompletionsRequest has no fields for these parameters — unknown
// fields are dropped naturally by json.Unmarshal. Kept semantically in sync
// with the list in openai_gateway_service.go:2034 used by the /v1/responses
// passthrough path.
var cursorResponsesUnsupportedFields = []string{
	"prompt_cache_retention",
	"safety_identifier",
	"metadata",
	"stream_options",
}

// ForwardAsChatCompletions accepts a Chat Completions request body, converts it
// to OpenAI Responses API format, forwards to the OpenAI upstream, and converts
// the response back to Chat Completions format.
//
// 历史背景：该函数原本对所有 OpenAI 账号无差别走 CC→Responses 转换 + /v1/responses
// 端点——这在 OAuth（ChatGPT 内部 API 仅支持 Responses）和官方 APIKey 账号上是
// 正确的，但 sub2api 接入 DeepSeek/Kimi/GLM 等第三方 OpenAI 兼容上游后假设破裂：
// 这些上游普遍只支持 /v1/chat/completions，无 /v1/responses 端点。
//
// 当前路由策略（按 inbound + extra，详见 openai_compat.ResolveUpstreamAPI）：
//   - APIKey 账号 + 入站 CC 判定上游为 Chat Completions
//     （force_chat_completions / passthrough / auto+Rsupp=false）
//     → 走 forwardAsRawChatCompletions 直转，不做协议转换
//   - 其他所有情况（OAuth、auto+未探测/支持、force_responses）→ 走原有
//     CC→Responses 转换路径。auto 在两路都可用时仍转 Responses。
func (s *OpenAIGatewayService) ForwardAsChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	promptCacheKey string,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	restrictionResult := s.detectCodexClientRestriction(c, account, body)
	logCodexCLIOnlyDetection(ctx, c, account, getAPIKeyIDFromContext(c), restrictionResult, body)
	if restrictionResult.Enabled && !restrictionResult.Matched {
		MarkOpsClientBusinessLimited(c, OpsClientBusinessLimitedReasonLocalPolicyDenied)
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"type":    "forbidden_error",
				"message": CodexClientRestrictionMessage(restrictionResult),
			},
		})
		return nil, errors.New("codex_cli_only restriction: only codex official clients are allowed")
	}
	if account.Platform == PlatformGrok {
		if account.IsGrokOAuth() {
			if eligible, reason := grokChatResponsesBridgeEligibility(body); eligible {
				return s.forwardGrokChatCompletionsViaResponses(ctx, c, account, body, promptCacheKey, defaultMappedModel)
			} else {
				logger.L().Debug("grok chat_completions: using raw fallback",
					zap.Int64("account_id", account.ID),
					zap.String("reason", reason),
				)
			}
		}
		return s.forwardAsRawChatCompletions(ctx, c, account, body, defaultMappedModel)
	}

	// 入口分流：按入站 CC + extra 选上游。passthrough 走 raw CC；
	// auto+未探测仍走下方 Responses 转换（存量兼容）。
	if account.Type == AccountTypeAPIKey &&
		openai_compat.ResolveUpstreamAPI(openai_compat.InboundChatCompletions, account.Extra) == openai_compat.UpstreamChatCompletions {
		return s.forwardAsRawChatCompletions(ctx, c, account, body, defaultMappedModel)
	}

	startTime := time.Now()

	// 1. Parse Chat Completions request
	var chatReq apicompat.ChatCompletionsRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		return nil, fmt.Errorf("parse chat completions request: %w", err)
	}
	originalModel := chatReq.Model
	clientStream := chatReq.Stream

	// 2. Resolve model mapping early so compat prompt_cache_key injection can
	// derive a stable seed from the final upstream model family.
	billingModel := resolveOpenAIForwardModel(account, originalModel, defaultMappedModel)
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)

	promptCacheKey = strings.TrimSpace(promptCacheKey)
	if promptCacheKey == "" {
		// Honor client session_id / conversation_id / body prompt_cache_key
		// even when the caller passed an empty key (same sources as ExtractSessionID).
		promptCacheKey = explicitOpenAIRequestSessionID(c, body)
	}
	compatPromptCacheInjected := false
	if promptCacheKey == "" && shouldAutoInjectPromptCacheKeyForCompat(upstreamModel) {
		promptCacheKey = deriveCompatPromptCacheKey(&chatReq, upstreamModel)
		compatPromptCacheInjected = promptCacheKey != ""
	}

	// 3. Build the upstream (Responses API) body.
	//
	// Cursor compatibility: some clients (notably Cursor cloud) send Responses
	// API shaped bodies — `input: [...]` with no `messages` field — to the
	// /v1/chat/completions URL. Running those through ChatCompletionsToResponses
	// would silently drop Cursor's `input` array (the struct has no Input field)
	// and produce `input: null`, which Codex upstreams reject with
	// "Invalid type for 'input': expected a string, but got an object".
	//
	// Detect that shape and forward the raw body as-is, only rewriting `model`
	// to the resolved upstream model. The downstream codex OAuth transform will
	// still normalize store/stream/instructions/etc.
	isResponsesShape := !gjson.GetBytes(body, "messages").Exists() && gjson.GetBytes(body, "input").Exists()

	var (
		responsesReq  *apicompat.ResponsesRequest
		responsesBody []byte
		err           error
	)
	if isResponsesShape {
		responsesBody, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, fmt.Errorf("rewrite model in responses-shape body: %w", err)
		}
		// Strip Responses API parameters that no Codex upstream accepts.
		// Because this branch forwards the raw body (the normal path rebuilds
		// it from ChatCompletionsRequest and drops unknown fields naturally),
		// we must filter these fields explicitly here — otherwise the upstream
		// rejects the request with "Unsupported parameter: ...".
		for _, field := range cursorResponsesUnsupportedFields {
			if stripped, derr := sjson.DeleteBytes(responsesBody, field); derr == nil {
				responsesBody = stripped
			}
		}
		responsesBody, normalizedServiceTier, err := normalizeResponsesBodyServiceTier(responsesBody)
		if err != nil {
			return nil, fmt.Errorf("normalize service_tier in responses-shape body: %w", err)
		}
		// Minimal stub populated from the raw body so downstream billing
		// propagation (ServiceTier, ReasoningEffort) keeps working.
		responsesReq = &apicompat.ResponsesRequest{
			Model:       upstreamModel,
			ServiceTier: normalizedServiceTier,
		}
		if effort := gjson.GetBytes(responsesBody, "reasoning.effort").String(); effort != "" {
			responsesReq.Reasoning = &apicompat.ResponsesReasoning{Effort: effort}
		}
	} else {
		// Normal path: convert Chat Completions → Responses.
		// ChatCompletionsToResponses sets Stream=true; API Key sync flips it
		// back to false after OAuth transform (which is skipped for API keys).
		responsesReq, err = apicompat.ChatCompletionsToResponses(&chatReq)
		if err != nil {
			return nil, fmt.Errorf("convert chat completions to responses: %w", err)
		}
		responsesReq.Model = upstreamModel
		normalizeResponsesRequestServiceTier(responsesReq)
		responsesBody, err = json.Marshal(responsesReq)
		if err != nil {
			return nil, fmt.Errorf("marshal responses request: %w", err)
		}
	}

	logFields := []zap.Field{
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", clientStream),
		zap.Bool("responses_shape", isResponsesShape),
	}
	if compatPromptCacheInjected {
		logFields = append(logFields,
			zap.Bool("compat_prompt_cache_key_injected", true),
			zap.String("compat_prompt_cache_key_sha256", hashSensitiveValueForLog(promptCacheKey)),
		)
	}
	logger.L().Debug("openai chat_completions: model mapping applied", logFields...)

	if account.Type == AccountTypeOAuth {
		var reqBody map[string]any
		if err := json.Unmarshal(responsesBody, &reqBody); err != nil {
			return nil, fmt.Errorf("unmarshal for codex transform: %w", err)
		}
		codexResult := applyCodexOAuthTransformWithOptions(reqBody, codexOAuthTransformOptions{
			SkipDefaultInstructions: !isResponsesShape,
		})
		if codexResult.NormalizedModel != "" {
			upstreamModel = codexResult.NormalizedModel
		}
		if codexResult.PromptCacheKey != "" {
			promptCacheKey = codexResult.PromptCacheKey
		} else if promptCacheKey != "" {
			reqBody["prompt_cache_key"] = promptCacheKey
		}
		if !isResponsesShape {
			ensureCodexOAuthInstructionsField(reqBody)
		}
		responsesBody, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("remarshal after codex transform: %w", err)
		}
	}

	if account.Type == AccountTypeAPIKey {
		if trimmedKey := strings.TrimSpace(promptCacheKey); trimmedKey != "" {
			var reqBody map[string]any
			if err := json.Unmarshal(responsesBody, &reqBody); err != nil {
				return nil, fmt.Errorf("unmarshal for prompt cache key injection: %w", err)
			}
			if existing, ok := reqBody["prompt_cache_key"].(string); !ok || strings.TrimSpace(existing) == "" {
				reqBody["prompt_cache_key"] = trimmedKey
				responsesBody, err = json.Marshal(reqBody)
				if err != nil {
					return nil, fmt.Errorf("remarshal after prompt cache key injection: %w", err)
				}
			}
		}
	}

	// 4b. Apply OpenAI fast policy (may filter service_tier or block the request).
	updatedBody, policyErr := s.applyOpenAIFastPolicyToBody(ctx, account, upstreamModel, responsesBody)
	if policyErr != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(policyErr, &blocked) {
			writeChatCompletionsError(c, http.StatusForbidden, "permission_error", blocked.Message)
		}
		return nil, policyErr
	}
	responsesBody = updatedBody

	// 5. Get access token
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// 6. Build upstream request.
	// API Key + client sync + official/empty base_url: keep S2 (upstream JSON).
	// Custom midstream base_url: upstream SSE + local buffer so CF sees bytes.
	// OAuth transform already forced stream=true above.
	upstreamStream := true
	if account.Type == AccountTypeAPIKey && !clientStream {
		if shouldForceSyncInboundUpstreamSSE(account, s.cfg, clientStream) {
			if patched, setErr := sjson.SetBytes(responsesBody, "stream", true); setErr == nil {
				responsesBody = patched
			}
			logger.L().Debug("openai chat_completions: sync_inbound_upstream_sse=true",
				zap.Int64("account_id", account.ID),
				zap.String("upstream_model", upstreamModel),
			)
		} else {
			upstreamStream = false
			if patched, setErr := sjson.SetBytes(responsesBody, "stream", false); setErr == nil {
				responsesBody = patched
			}
			logger.L().Debug("openai chat_completions: upstream_non_stream=true",
				zap.Int64("account_id", account.ID),
				zap.String("upstream_model", upstreamModel),
			)
		}
	}
	upstreamReq, err := s.buildUpstreamRequest(ctx, c, account, responsesBody, token, upstreamStream, promptCacheKey, false)
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}

	if promptCacheKey != "" {
		apiKeyID := getAPIKeyIDFromContext(c)
		upstreamReq.Header.Set("session_id", generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey)))
	}

	// 7. Send request
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.doOpenAIUpstreamWithHeaderWait(ctx, c, account, upstreamReq, proxyURL, false, originalModel)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	// 8. Handle error response with failover
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			upstreamDetail := ""
			upstreamDetail = sanitizeOpsUpstreamDetail(respBody)
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				Kind:               "failover",
				Message:            upstreamMsg,
				Detail:             upstreamDetail,
			})
			if s.rateLimitService != nil {
				s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			}
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && (isPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody)),
			}
		}
		return s.handleChatCompletionsErrorResponse(resp, c, account)
	}

	// 9. Handle normal response
	var result *OpenAIForwardResult
	var handleErr error
	if clientStream {
		result, handleErr = s.handleChatStreamingResponse(resp, c, originalModel, billingModel, upstreamModel, startTime, account)
	} else if isOpenAIJSONResponse(resp) {
		result, handleErr = s.handleChatNonStreamResponsesJSON(resp, c, originalModel, billingModel, upstreamModel, startTime, account)
	} else {
		result, handleErr = s.handleChatBufferedStreamingResponse(resp, c, originalModel, billingModel, upstreamModel, startTime, account)
	}
	if GetOpsCyberPolicy(c) != nil {
		return nil, errOpenAICyberPolicyForwarded
	}

	// Propagate ServiceTier and ReasoningEffort to result for billing
	if handleErr == nil && result != nil {
		if result.ResponseID != "" {
			s.bindHTTPResponseAccount(ctx, c, account, result.ResponseID)
		}
		if responsesReq.ServiceTier != "" {
			st := responsesReq.ServiceTier
			result.ServiceTier = &st
		}
		if responsesReq.Reasoning != nil && responsesReq.Reasoning.Effort != "" {
			re := responsesReq.Reasoning.Effort
			result.ReasoningEffort = &re
		}
	}

	// Extract and save Codex usage snapshot from response headers (for OAuth accounts).
	// 排除 spark 影子:其 codex_* 仅由 QueryUsage(/wham/usage bengalfox)更新(外审第7轮 P1)。
	if handleErr == nil && account.Type == AccountTypeOAuth && !account.IsShadow() {
		if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
			s.updateCodexUsageSnapshot(ctx, account.ID, snapshot)
		}
	}

	return result, handleErr
}

func normalizeResponsesRequestServiceTier(req *apicompat.ResponsesRequest) {
	if req == nil {
		return
	}
	req.ServiceTier = normalizedOpenAIServiceTierValue(req.ServiceTier)
}

func normalizeResponsesBodyServiceTier(body []byte) ([]byte, string, error) {
	if len(body) == 0 {
		return body, "", nil
	}
	rawServiceTier := gjson.GetBytes(body, "service_tier").String()
	if rawServiceTier == "" {
		return body, "", nil
	}
	normalizedServiceTier := normalizedOpenAIServiceTierValue(rawServiceTier)
	if normalizedServiceTier == "" {
		trimmed, err := sjson.DeleteBytes(body, "service_tier")
		return trimmed, "", err
	}
	if normalizedServiceTier == rawServiceTier {
		return body, normalizedServiceTier, nil
	}
	trimmed, err := sjson.SetBytes(body, "service_tier", normalizedServiceTier)
	return trimmed, normalizedServiceTier, err
}

func normalizedOpenAIServiceTierValue(raw string) string {
	normalized := normalizeOpenAIServiceTier(raw)
	if normalized == nil {
		return ""
	}
	return *normalized
}

// handleChatCompletionsErrorResponse reads an upstream error and returns it in
// OpenAI Chat Completions error format.
func (s *OpenAIGatewayService) handleChatCompletionsErrorResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
) (*OpenAIForwardResult, error) {
	return s.handleCompatErrorResponse(resp, c, account, writeChatCompletionsError)
}

func cloneResponsesResponse(resp *apicompat.ResponsesResponse) *apicompat.ResponsesResponse {
	if resp == nil {
		return nil
	}
	cloned := *resp
	cloned.Usage = cloneResponsesUsage(resp.Usage)
	return &cloned
}

func cloneResponsesUsage(usage *apicompat.ResponsesUsage) *apicompat.ResponsesUsage {
	if usage == nil {
		return nil
	}
	cloned := *usage
	if usage.InputTokensDetails != nil {
		details := *usage.InputTokensDetails
		cloned.InputTokensDetails = &details
	}
	if usage.OutputTokensDetails != nil {
		details := *usage.OutputTokensDetails
		cloned.OutputTokensDetails = &details
	}
	return &cloned
}

func isOpenAIJSONResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/event-stream") {
		return false
	}
	return strings.Contains(ct, "application/json")
}

func resolveOpenAIBufferedRequestID(resp *http.Response, c *gin.Context) string {
	if resp != nil {
		for _, key := range []string{"x-request-id", "openai-request-id", "x-openai-request-id"} {
			if v := strings.TrimSpace(resp.Header.Get(key)); v != "" {
				return v
			}
		}
	}
	if c != nil && c.Request != nil {
		ctx := c.Request.Context()
		if v, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		if v, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func isOpenAIHTTP2PeerReset(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "internal_error") {
		return true
	}
	if strings.Contains(msg, "http2") && (strings.Contains(msg, "stream") || strings.Contains(msg, "internal")) {
		return true
	}
	return strings.Contains(msg, "stream error") && strings.Contains(msg, "received from peer")
}

func chatBufferedHasUsableTerminal(resp *apicompat.ResponsesResponse) bool {
	if resp == nil {
		return false
	}
	return strings.TrimSpace(resp.Status) != "failed"
}

func chatBufferedStreamInterval(timeoutSeconds int) time.Duration {
	if timeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func (s *OpenAIGatewayService) handleChatBufferedReadError(
	c *gin.Context,
	account *Account,
	requestID string,
	err error,
) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	logger.L().Warn("openai chat_completions buffered: read error",
		zap.Error(err),
		zap.String("request_id", requestID),
		zap.Int64("account_id", account.ID),
	)
	if isOpenAIHTTP2PeerReset(err) {
		return s.newOpenAIStreamFailoverError(c, account, false, requestID, nil, err.Error())
	}
	return nil
}

// finishChatCompletionsFromResponsesResponse converts a terminal Responses
// object to Chat Completions JSON. Shared by SSE-buffer and non-stream JSON.
func (s *OpenAIGatewayService) finishChatCompletionsFromResponsesResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestID string,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
	finalResponse *apicompat.ResponsesResponse,
	acc *apicompat.BufferedResponseAccumulator,
) (*OpenAIForwardResult, error) {
	if finalResponse == nil {
		writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Upstream stream ended without a terminal response event")
		return nil, fmt.Errorf("upstream stream ended without terminal event")
	}

	usage := OpenAIUsage{}
	if finalResponse.Usage != nil {
		usage = copyOpenAIUsageFromResponsesUsage(finalResponse.Usage)
	}
	responseID := strings.TrimSpace(finalResponse.ID)

	if strings.TrimSpace(finalResponse.Status) == "failed" {
		payload, _ := json.Marshal(gin.H{"type": "response.failed", "response": finalResponse})
		if hit, code, msg := detectOpenAICyberPolicy(payload); hit {
			MarkOpsCyberPolicy(c, CyberPolicyMark{Code: code, Message: msg, Body: truncateString(string(payload), 4096), UpstreamStatus: http.StatusOK, UpstreamInTok: usage.InputTokens, UpstreamOutTok: usage.OutputTokens})
			if msg == "" {
				msg = "Request blocked by upstream cyber-security policy"
			}
			writeChatCompletionsError(c, http.StatusBadRequest, "invalid_request_error", msg)
			return nil, errOpenAICyberPolicyForwarded
		}
		message := ""
		if finalResponse.Error != nil {
			message = strings.TrimSpace(finalResponse.Error.Message)
		}
		if openAIStreamFailedEventShouldFailover(payload, message) {
			return nil, s.newOpenAIStreamFailoverError(c, account, false, requestID, payload, message)
		}
		message = s.recordOpenAIStreamUpstreamError(c, account, false, requestID, "http_error", payload, message)
		if status, errType, errMsg, matched := applyOpenAIStreamFailedErrorPassthroughRule(c, account.Platform, payload, message); matched {
			if errMsg == "" {
				errMsg = message
			}
			MarkResponseCommitted(c)
			writeChatCompletionsError(c, status, errType, errMsg)
			return nil, fmt.Errorf("upstream response failed (passthrough): %s", errMsg)
		}
		writeChatCompletionsError(c, http.StatusBadGateway, "upstream_error", message)
		return nil, fmt.Errorf("upstream response failed: %s", message)
	}

	if acc != nil {
		acc.SupplementResponseOutput(finalResponse)
	}

	chatResp := apicompat.ResponsesToChatCompletions(finalResponse, originalModel)

	if s.responseHeaderFilter != nil && resp != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	body, marshalErr := json.Marshal(chatResp)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal chat completions response: %w", marshalErr)
	}
	if mult := getDisplayTokenMultipliers(c); mult != nil {
		body = rewriteOpenAIChatUsageTokens(body, "usage", mult)
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)

	return &OpenAIForwardResult{
		RequestID:     requestID,
		ResponseID:    responseID,
		Usage:         usage,
		Model:         originalModel,
		BillingModel:  billingModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) handleChatNonStreamResponsesJSON(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
	accountOpt ...*Account,
) (*OpenAIForwardResult, error) {
	requestID := resolveOpenAIBufferedRequestID(resp, c)
	account := &Account{Platform: PlatformOpenAI}
	if len(accountOpt) > 0 && accountOpt[0] != nil {
		account = accountOpt[0]
	}

	raw, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			return nil, err
		}
		if failover := s.handleChatBufferedReadError(c, account, requestID, err); failover != nil {
			return nil, failover
		}
		writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Failed to read upstream JSON response")
		return nil, fmt.Errorf("read non-stream responses json: %w", err)
	}

	var finalResponse apicompat.ResponsesResponse
	err = json.Unmarshal(raw, &finalResponse)
	looksEmpty := strings.TrimSpace(finalResponse.ID) == "" && strings.TrimSpace(finalResponse.Status) == ""
	if err != nil || looksEmpty {
		if wrapped := gjson.GetBytes(raw, "response"); wrapped.Exists() {
			if wrapErr := json.Unmarshal([]byte(wrapped.Raw), &finalResponse); wrapErr != nil {
				writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Upstream JSON response was not a Responses object")
				return nil, fmt.Errorf("unmarshal wrapped responses json: %w", wrapErr)
			}
		} else if err != nil {
			writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Upstream JSON response was not a Responses object")
			return nil, fmt.Errorf("unmarshal responses json: %w", err)
		} else {
			writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Upstream JSON response was not a Responses object")
			return nil, fmt.Errorf("unmarshal responses json: empty response object")
		}
	}

	return s.finishChatCompletionsFromResponsesResponse(
		resp, c, account, requestID, originalModel, billingModel, upstreamModel, startTime, &finalResponse, nil,
	)
}

// handleChatBufferedStreamingResponse reads Responses SSE events from the
// upstream until a terminal event, converts to a Chat Completions JSON
// response, and writes it to the client. completed / done / incomplete
// return immediately; failed uses the existing finish error path. Missing
// terminals still wait for EOF, interval timeout, or H2 reset.
func (s *OpenAIGatewayService) handleChatBufferedStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
	accountOpt ...*Account,
) (*OpenAIForwardResult, error) {
	requestID := resolveOpenAIBufferedRequestID(resp, c)
	account := &Account{Platform: PlatformOpenAI}
	if len(accountOpt) > 0 && accountOpt[0] != nil {
		account = accountOpt[0]
	}

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var finalResponse *apicompat.ResponsesResponse
	acc := apicompat.NewBufferedResponseAccumulator()

	firstFrameTimer, firstFrameCh := beginOpenAIWaitTimer(s.openAIWaitTimeoutSettingsForAccount(account).FirstUsefulFrameDuration())
	defer stopOpenAIWaitTimer(firstFrameTimer)
	firstFrameStartedAt := time.Now()
	usefulFrameSeen := false
	stopFirstFrameWatch := func() {
		if usefulFrameSeen {
			return
		}
		usefulFrameSeen = true
		stopOpenAIWaitTimer(firstFrameTimer)
		firstFrameCh = nil
	}

	processLine := func(line string) bool {
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			return false
		}
		payload := line[6:]

		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			logger.L().Warn("openai chat_completions buffered: failed to parse event",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			return false
		}

		if openAIStreamDataStartsClientOutput(payload, event.Type) {
			stopFirstFrameWatch()
		}

		acc.ProcessEvent(&event)

		if isOpenAICompatResponsesTerminalEvent(event.Type) && event.Response != nil {
			finalResponse = event.Response
			return true
		}
		return false
	}
	finish := func() (*OpenAIForwardResult, error) {
		return s.finishChatCompletionsFromResponsesResponse(
			resp, c, account, requestID, originalModel, billingModel, upstreamModel, startTime, finalResponse, acc,
		)
	}

	timeoutSeconds := 0
	if s.cfg != nil {
		timeoutSeconds = s.cfg.Gateway.StreamDataIntervalTimeout
	}
	interval := chatBufferedStreamInterval(timeoutSeconds)
	if interval <= 0 && firstFrameCh == nil {
		for scanner.Scan() {
			if processLine(scanner.Text()) {
				return finish()
			}
		}
		if failover := s.handleChatBufferedReadError(c, account, requestID, scanner.Err()); failover != nil && !chatBufferedHasUsableTerminal(finalResponse) {
			return nil, failover
		}
		return finish()
	}

	type scanEvent struct {
		line string
		err  error
	}
	events := make(chan scanEvent, 16)
	done := make(chan struct{})
	defer close(done)
	var lastReadAt int64
	atomic.StoreInt64(&lastReadAt, time.Now().UnixNano())
	sendEvent := func(ev scanEvent) bool {
		select {
		case events <- ev:
			return true
		case <-done:
			return false
		}
	}
	go func() {
		defer close(events)
		for scanner.Scan() {
			atomic.StoreInt64(&lastReadAt, time.Now().UnixNano())
			if !sendEvent(scanEvent{line: scanner.Text()}) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = sendEvent(scanEvent{err: err})
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var tickerC <-chan time.Time
	if interval > 0 {
		tickerC = ticker.C
	} else {
		ticker.Stop()
	}
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return finish()
			}
			if ev.err != nil {
				if failover := s.handleChatBufferedReadError(c, account, requestID, ev.err); failover != nil && !chatBufferedHasUsableTerminal(finalResponse) {
					return nil, failover
				}
				return finish()
			}
			if processLine(ev.line) {
				return finish()
			}
		case <-firstFrameCh:
			if usefulFrameSeen || chatBufferedHasUsableTerminal(finalResponse) {
				firstFrameCh = nil
				continue
			}
			ctx := context.Background()
			if c != nil && c.Request != nil {
				ctx = c.Request.Context()
			}
			silentFailover, timeoutErr := s.openAIFirstUsefulFrameTimeoutErr(ctx, c, account, originalModel, false, time.Since(firstFrameStartedAt), false)
			if !silentFailover {
				writeOpenAIChatStreamErrorEvent(c, OpenAIFirstUsefulFrameTimeoutMarker)
			}
			return nil, timeoutErr
		case <-tickerC:
			lastRead := time.Unix(0, atomic.LoadInt64(&lastReadAt))
			if time.Since(lastRead) < interval {
				continue
			}
			if chatBufferedHasUsableTerminal(finalResponse) {
				return finish()
			}
			logger.L().Warn("openai chat_completions buffered: stream data interval timeout",
				zap.Int64("account_id", account.ID),
				zap.String("request_id", requestID),
				zap.Duration("interval", interval),
			)
			if s.rateLimitService != nil && c != nil && c.Request != nil {
				s.rateLimitService.HandleStreamTimeout(c.Request.Context(), account, originalModel)
			} else if s.rateLimitService != nil {
				s.rateLimitService.HandleStreamTimeout(context.Background(), account, originalModel)
			}
			return nil, s.newOpenAIStreamFailoverError(c, account, false, requestID, nil, "stream data interval timeout")
		}
	}
}

// handleChatStreamingResponse reads Responses SSE events from upstream,
// converts each to Chat Completions SSE chunks, and writes them to the client.
func (s *OpenAIGatewayService) handleChatStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
	accountOpt ...*Account,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")
	account := &Account{Platform: PlatformOpenAI}
	if len(accountOpt) > 0 && accountOpt[0] != nil {
		account = accountOpt[0]
	}

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	state := apicompat.NewResponsesEventToChatState()
	state.Model = originalModel
	// The gateway is part of the billing chain, so downstream usage must not
	// depend on whether the client explicitly requested stream usage.
	state.IncludeUsage = true

	var usage OpenAIUsage
	var firstTokenMs *int
	var hopFirstTokenMs *int
	firstChunk := true
	responseID := ""
	var terminalErr error

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	firstFrameTimer, firstFrameCh := beginOpenAIWaitTimer(s.openAIWaitTimeoutSettingsForAccount(account).FirstUsefulFrameDuration())
	defer stopOpenAIWaitTimer(firstFrameTimer)
	firstFrameStartedAt := time.Now()
	usefulFrameSeen := false
	stopFirstFrameWatch := func() {
		if usefulFrameSeen {
			return
		}
		usefulFrameSeen = true
		stopOpenAIWaitTimer(firstFrameTimer)
		firstFrameCh = nil
	}

	resultWithUsage := func() *OpenAIForwardResult {
		return &OpenAIForwardResult{
			RequestID:     requestID,
			ResponseID:    responseID,
			Usage:         usage,
			Model:         originalModel,
			BillingModel:  billingModel,
			UpstreamModel: upstreamModel,
			Stream:        true,
			Duration:      time.Since(startTime),
			FirstTokenMs:  firstTokenMs,
			HopFirstTokenMs: hopFirstTokenMs,
		}
	}

	processDataLine := func(payload string) bool {
		if firstChunk {
			firstChunk = false
			stampRequestFirstTokenMs(&firstTokenMs, c, startTime)
			stampHopFirstTokenMs(&hopFirstTokenMs, startTime)
		}

		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			logger.L().Warn("openai chat_completions stream: failed to parse event",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			return false
		}

		if openAIStreamDataStartsClientOutput(payload, event.Type) {
			stopFirstFrameWatch()
		}

		isTerminalEvent := isOpenAICompatResponsesTerminalEvent(event.Type)
		if responseID == "" && event.Response != nil {
			responseID = strings.TrimSpace(event.Response.ID)
		}
		if isTerminalEvent {
			usageSeen := false
			if event.Usage != nil {
				usage = copyOpenAIUsageFromResponsesUsage(event.Usage)
				usageSeen = true
			}
			if event.Response != nil && event.Response.Usage != nil {
				usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
				usageSeen = true
			}
			if mult := getDisplayTokenMultipliers(c); mult != nil && usageSeen {
				displayUsage := applyOpenAIResponsesUsageDisplayMultipliers(&usage, mult)
				if event.Response == nil {
					event.Response = &apicompat.ResponsesResponse{}
				}
				event.Response = cloneResponsesResponse(event.Response)
				if event.Response.Usage == nil {
					event.Response.Usage = &apicompat.ResponsesUsage{}
				}
				event.Response.Usage = cloneResponsesUsage(event.Response.Usage)
				event.Response.Usage.InputTokens = displayUsage.InputTokens
				event.Response.Usage.OutputTokens = displayUsage.OutputTokens
				event.Response.Usage.TotalTokens = displayUsage.InputTokens + displayUsage.OutputTokens
				if displayUsage.CacheReadInputTokens > 0 || displayUsage.CacheCreationInputTokens > 0 || event.Response.Usage.InputTokensDetails != nil {
					if event.Response.Usage.InputTokensDetails == nil {
						event.Response.Usage.InputTokensDetails = &apicompat.ResponsesInputTokensDetails{}
					}
					event.Response.Usage.InputTokensDetails.CachedTokens = displayUsage.CacheReadInputTokens
					event.Response.Usage.InputTokensDetails.CacheWriteTokens = displayUsage.CacheCreationInputTokens
				}
			}
		}
		if strings.TrimSpace(event.Type) == "response.failed" {
			payloadBytes := []byte(payload)
			if hit, code, msg := detectOpenAICyberPolicy(payloadBytes); hit {
				MarkOpsCyberPolicy(c, CyberPolicyMark{Code: code, Message: msg, Body: truncateString(payload, 4096), UpstreamStatus: http.StatusOK, UpstreamInTok: usage.InputTokens, UpstreamOutTok: usage.OutputTokens})
				if msg == "" {
					msg = "Request blocked by upstream cyber-security policy"
				}
				body, _ := json.Marshal(gin.H{"error": gin.H{"type": "invalid_request_error", "code": code, "message": msg}})
				fmt.Fprintf(c.Writer, "data: %s\n\ndata: [DONE]\n\n", body) //nolint:errcheck
				c.Writer.Flush()
				terminalErr = errOpenAICyberPolicyForwarded
				return true
			}
			message := extractOpenAISSEErrorMessage(payloadBytes)
			if openAIStreamFailedEventShouldFailover(payloadBytes, message) {
				terminalErr = s.newOpenAIStreamFailoverError(c, account, false, requestID, payloadBytes, message)
				return true
			}
			message = s.recordOpenAIStreamUpstreamError(c, account, false, requestID, "http_error", payloadBytes, message)
			status, errType, errMsg := http.StatusBadGateway, "upstream_error", message
			if matchedStatus, matchedType, matchedMsg, matched := applyOpenAIStreamFailedErrorPassthroughRule(c, account.Platform, payloadBytes, message); matched {
				status, errType, errMsg = matchedStatus, matchedType, matchedMsg
				if errMsg == "" {
					errMsg = message
				}
				MarkResponseCommitted(c)
			}
			if !c.Writer.Written() {
				writeChatCompletionsError(c, status, errType, errMsg)
			} else {
				errorPayload, _ := json.Marshal(gin.H{"error": gin.H{"type": errType, "message": errMsg}})
				_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", errorPayload)
				c.Writer.Flush()
			}
			terminalErr = fmt.Errorf("upstream response failed: %s", errMsg)
			return true
		}

		chunks := apicompat.ResponsesEventToChatChunks(&event, state)
		for _, chunk := range chunks {
			sse, err := apicompat.ChatChunkToSSE(chunk)
			if err != nil {
				logger.L().Warn("openai chat_completions stream: failed to marshal chunk",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				continue
			}
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				logger.L().Info("openai chat_completions stream: client disconnected",
					zap.String("request_id", requestID),
				)
				return true
			}
		}
		if len(chunks) > 0 {
			c.Writer.Flush()
		}
		return false
	}

	finalizeStream := func() (*OpenAIForwardResult, error) {
		if finalChunks := apicompat.FinalizeResponsesChatStream(state); len(finalChunks) > 0 {
			for _, chunk := range finalChunks {
				sse, err := apicompat.ChatChunkToSSE(chunk)
				if err != nil {
					continue
				}
				fmt.Fprint(c.Writer, sse) //nolint:errcheck
			}
		}
		// Send [DONE] sentinel
		fmt.Fprint(c.Writer, "data: [DONE]\n\n") //nolint:errcheck
		c.Writer.Flush()
		return resultWithUsage(), nil
	}

	handleScanErr := func(err error) {
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("openai chat_completions stream: read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
		}
	}

	// Determine keepalive interval
	keepaliveInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamKeepaliveInterval > 0 {
		keepaliveInterval = time.Duration(s.cfg.Gateway.StreamKeepaliveInterval) * time.Second
	}

	// No keepalive and no first-frame gate: fast synchronous path
	if keepaliveInterval <= 0 && firstFrameCh == nil {
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
				continue
			}
			if processDataLine(line[6:]) {
				return resultWithUsage(), terminalErr
			}
		}
		handleScanErr(scanner.Err())
		return finalizeStream()
	}

	// With keepalive: goroutine + channel + select
	type scanEvent struct {
		line string
		err  error
	}
	events := make(chan scanEvent, 16)
	done := make(chan struct{})
	sendEvent := func(ev scanEvent) bool {
		select {
		case events <- ev:
			return true
		case <-done:
			return false
		}
	}
	go func() {
		defer close(events)
		for scanner.Scan() {
			if !sendEvent(scanEvent{line: scanner.Text()}) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = sendEvent(scanEvent{err: err})
		}
	}()
	defer close(done)

	var keepaliveC <-chan time.Time
	if keepaliveInterval > 0 {
		keepaliveTicker := time.NewTicker(keepaliveInterval)
		defer keepaliveTicker.Stop()
		keepaliveC = keepaliveTicker.C
	}
	lastDataAt := time.Now()

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return finalizeStream()
			}
			if ev.err != nil {
				handleScanErr(ev.err)
				return finalizeStream()
			}
			lastDataAt = time.Now()
			line := ev.line
			if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
				continue
			}
			if processDataLine(line[6:]) {
				return resultWithUsage(), terminalErr
			}

		case <-firstFrameCh:
			if usefulFrameSeen {
				firstFrameCh = nil
				continue
			}
			ctx := context.Background()
			if c != nil && c.Request != nil {
				ctx = c.Request.Context()
			}
			silentFailover, timeoutErr := s.openAIFirstUsefulFrameTimeoutErr(ctx, c, account, originalModel, false, time.Since(firstFrameStartedAt), false)
			if !silentFailover {
				writeOpenAIChatStreamErrorEvent(c, OpenAIFirstUsefulFrameTimeoutMarker)
				return resultWithUsage(), timeoutErr
			}
			return nil, timeoutErr

		case <-keepaliveC:
			if time.Since(lastDataAt) < keepaliveInterval {
				continue
			}
			// Send SSE comment as keepalive
			if _, err := fmt.Fprint(c.Writer, ":\n\n"); err != nil {
				logger.L().Info("openai chat_completions stream: client disconnected during keepalive",
					zap.String("request_id", requestID),
				)
				return resultWithUsage(), nil
			}
			c.Writer.Flush()
		}
	}
}

// writeChatCompletionsError writes an error response in OpenAI Chat Completions format.
func writeChatCompletionsError(c *gin.Context, statusCode int, errType, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}
