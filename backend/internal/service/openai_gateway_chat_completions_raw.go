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
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

var openaiCCRawAllowedHeaders = map[string]bool{
	"accept-language": true,
	"user-agent":      true,
}

// forwardAsRawChatCompletions 直转客户端的 Chat Completions 请求到上游
// `{base_url}/v1/chat/completions`，**不**做 CC↔Responses 协议转换。
//
// 适用场景：account.platform=openai && account.type=apikey && 上游已被探测确认
// 不支持 /v1/responses 端点（如 DeepSeek/Kimi/GLM/Qwen 等第三方 OpenAI 兼容上游）。
//
// 与 ForwardAsChatCompletions 的关键差异：
//
//   - 不调用 apicompat.ChatCompletionsToResponses，body 仅做模型 ID 改写
//   - 上游 URL 拼到 /v1/chat/completions 而非 /v1/responses
//   - 流式响应 SSE 直接透传给客户端（上游 chunk 已是 CC 格式）
//   - 非流式响应 JSON 直接透传，仅按需提取 usage
//   - 不应用 codex OAuth transform（APIKey 路径无 OAuth）
//   - 不注入 prompt_cache_key（OAuth 专属机制）
//
// 调用入口：openai_gateway_chat_completions.go::ForwardAsChatCompletions
// 在函数顶部按 openai_compat.ResolveUpstreamAPI(InboundChatCompletions) 分流。
func (s *OpenAIGatewayService) forwardAsRawChatCompletions(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	// 1. Parse minimal fields needed for routing/billing
	originalModel := gjson.GetBytes(body, "model").String()
	if originalModel == "" {
		writeChatCompletionsError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}
	clientStream := gjson.GetBytes(body, "stream").Bool()
	serviceTier := extractOpenAIServiceTierFromBody(body)

	// 2. Resolve model mapping (same as ForwardAsChatCompletions)
	billingModel := resolveOpenAIForwardModel(account, originalModel, defaultMappedModel)
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	grokCacheIdentity := ""
	if account.Platform == PlatformGrok {
		grokCacheIdentity = resolveGrokCacheIdentity(c, body, "", upstreamModel)
	}
	reasoningEffort := extractOpenAIReasoningEffortFromBody(body, upstreamModel, billingModel, originalModel)

	// 3. Rewrite model in body (no protocol conversion)
	upstreamBody := body
	if upstreamModel != originalModel {
		upstreamBody = ReplaceModelInBody(body, upstreamModel)
	}
	if normalizedBody, normalized := NormalizeGLMOpenAIReasoningEffort(upstreamBody, upstreamModel); normalized {
		upstreamBody = normalizedBody
	}

	// 4. Apply OpenAI fast policy on the CC body
	updatedBody, policyErr := s.applyOpenAIFastPolicyToBody(ctx, account, upstreamModel, upstreamBody)
	if policyErr != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(policyErr, &blocked) {
			writeChatCompletionsError(c, http.StatusForbidden, "permission_error", blocked.Message)
		}
		return nil, policyErr
	}
	upstreamBody = updatedBody
	token, tokenKind, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("account %d missing %s credential", account.ID, tokenKind)
	}
	var bridgeUsage OpenAIUsage
	if account.Platform == PlatformGrok {
		bridgedBody, usage, bridged, bridgeErr := s.bridgeGrokComposerImageInputs(ctx, c, account, upstreamBody, token)
		if bridgeErr != nil {
			return nil, bridgeErr
		}
		if bridged {
			upstreamBody = bridgedBody
			addGrokOpenAIUsage(&bridgeUsage, usage)
		}
	}
	forceUpstreamSSE := shouldForceSyncInboundUpstreamSSE(account, s.cfg, clientStream)
	if clientStream || forceUpstreamSSE {
		var usageErr error
		upstreamBody, usageErr = ensureOpenAIChatStreamUsage(upstreamBody)
		if usageErr != nil {
			return nil, fmt.Errorf("enable stream usage: %w", usageErr)
		}
	}
	if forceUpstreamSSE {
		if patched, setErr := sjson.SetBytes(upstreamBody, "stream", true); setErr == nil {
			upstreamBody = patched
		}
	}
	if account.Platform == PlatformGrok {
		upstreamBody, err = stripGrokChatPromptCacheKey(upstreamBody)
		if err != nil {
			return nil, fmt.Errorf("remove Responses-only Grok prompt cache key: %w", err)
		}
	}

	logger.L().Debug("openai chat_completions raw: forwarding without protocol conversion",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", clientStream),
	)

	// 5. Build upstream request
	var targetURL string
	if account.Platform == PlatformGrok {
		targetURL, err = buildGrokChatCompletionsURLForAccount(account)
	} else {
		baseURL := account.GetOpenAIBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		var validatedURL string
		validatedURL, err = s.validateUpstreamBaseURL(baseURL)
		if err == nil {
			targetURL = buildOpenAIChatCompletionsURL(validatedURL)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	if clientStream || forceUpstreamSSE {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}

	// Whitelist passthrough headers (subset of openaiAllowedHeaders relevant to CC).
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if openaiCCRawAllowedHeaders[lowerKey] {
			for _, v := range values {
				upstreamReq.Header.Add(key, v)
			}
		}
	}
	customUA := account.GetOpenAIUserAgent()
	if customUA == "" && account.Platform == PlatformGrok {
		customUA = "sub2api-grok/1.0"
	}
	if customUA != "" {
		upstreamReq.Header.Set("user-agent", customUA)
	}
	if account.Platform == PlatformGrok {
		applyGrokCacheHeaders(upstreamReq.Header, grokCacheIdentity)
	}
	SetActualOpenAIUpstreamEndpoint(c, grokChatRawEndpoint)

	// 账号级请求头覆写（仅 openai api_key 账号启用时生效）
	account.ApplyHeaderOverrides(upstreamReq.Header)

	// 6. Send request
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	var stageClk *openAIStreamStageClock
	if clientStream {
		stageClk = s.beginOpenAIStreamStageTiming(c, account, originalModel, "chat_raw", startTime)
	}
	stageClk.MarkDoStart()
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		stageClk.Complete(c)
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	stageClk.MarkHeaders(resp.Header.Get("x-request-id"))
	defer func() { _ = resp.Body.Close() }()

	// 7. Handle error response with failover
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		if account.Platform == PlatformGrok {
			s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
		}
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			upstreamDetail := ""
			if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
				maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
				if maxBytes <= 0 {
					maxBytes = 2048
				}
				upstreamDetail = truncateString(string(respBody), maxBytes)
			}
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
			stageClk.Complete(c)
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && (isPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody)),
			}
		}
		stageClk.Complete(c)
		return s.handleChatCompletionsErrorResponse(resp, c, account)
	}
	if account.Platform == PlatformGrok {
		s.updateGrokUsageSnapshot(ctx, account, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
	}

	// 8. Forward response
	var result *OpenAIForwardResult
	if clientStream {
		result, err = s.streamRawChatCompletions(c, resp, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime, len(upstreamBody), account)
	} else if isOpenAIJSONResponse(resp) {
		result, err = s.bufferRawChatCompletions(c, resp, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
	} else {
		result, err = s.bufferRawChatCompletionsFromSSE(c, resp, account, originalModel, billingModel, upstreamModel, reasoningEffort, serviceTier, startTime)
	}
	stageClk.Complete(c)
	if result != nil {
		addGrokOpenAIUsage(&result.Usage, bridgeUsage)
		if account.Platform == PlatformGrok {
			result.UpstreamEndpoint = grokChatRawEndpoint
		}
	}
	return result, err
}

// streamRawChatCompletions 透传上游 CC SSE 流到客户端，并提取 usage（包括
// 末尾 [DONE] 之前的 chunk 中的 usage 字段，按 OpenAI CC 协议）。
func (s *OpenAIGatewayService) streamRawChatCompletions(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
	requestBodyLen int,
	account *Account,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var usage OpenAIUsage
	var firstTokenMs *int
	clientDisconnected := false
	clientOutputStarted := false
	pendingLines := make([]string, 0, 8)
	refusalDetector := newOpenAIChatSilentRefusalDetector(requestBodyLen)
	stageClk := getOpenAIStreamStageClock(c)

	writeStreamHeaders := func() {
		if s.responseHeaderFilter != nil {
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		}
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
	}
	writeLine := func(line string) bool {
		if !clientOutputStarted {
			writeStreamHeaders()
			for _, pending := range pendingLines {
				if _, werr := c.Writer.WriteString(pending + "\n"); werr != nil {
					clientDisconnected = true
					logger.L().Debug("openai chat_completions raw: client disconnected while flushing pending stream",
						zap.Error(werr),
						zap.String("request_id", requestID),
					)
					return false
				}
			}
			pendingLines = pendingLines[:0]
			clientOutputStarted = true
		}
		if _, werr := c.Writer.WriteString(line + "\n"); werr != nil {
			clientDisconnected = true
			logger.L().Debug("openai chat_completions raw: client disconnected, continuing to drain upstream for billing",
				zap.Error(werr),
				zap.String("request_id", requestID),
			)
			return false
		}
		return true
	}
	maybeMarkClientFlush := func() {
		if clientDisconnected || !clientOutputStarted {
			return
		}
		c.Writer.Flush()
		// Client-true flush is the first successful Flush after useful upstream was seen
		// (includes silent-refusal pending release that finally delivers useful content).
		if stageClk != nil && !stageClk.firstUsefulUpstreamAt.IsZero() {
			stageClk.MarkFirstClientFlush()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		refusalDetector.ObserveSSELine(line)
		if payload, ok := extractOpenAISSEDataLine(line); ok {
			stageClk.MarkFirstSSE()
			trimmedPayload := strings.TrimSpace(payload)
			if trimmedPayload != "[DONE]" {
				usageOnlyChunk := isOpenAIChatUsageOnlyStreamChunk(payload)
				if u := extractCCStreamUsage(payload); u != nil {
					usage = *u
				}
				if firstTokenMs == nil && !usageOnlyChunk {
					elapsed := int(time.Since(startTime).Milliseconds())
					firstTokenMs = &elapsed
				}
				if openAIChatPayloadStartsUsefulOutput(payload) {
					stageClk.MarkFirstUsefulUpstream()
				}
			}
		}

		outLine := line
		if mult := getDisplayTokenMultipliers(c); mult != nil {
			outLine = rewriteOpenAIChatSSEUsageTokens(outLine, mult)
		}
		if !clientDisconnected {
			if !clientOutputStarted && !refusalDetector.ShouldReleaseClientOutput() {
				pendingLines = append(pendingLines, outLine)
			} else {
				_ = writeLine(outLine)
			}
		}
		if line == "" {
			if !clientDisconnected && clientOutputStarted {
				maybeMarkClientFlush()
			}
			continue
		}
		if !clientDisconnected && clientOutputStarted {
			maybeMarkClientFlush()
		}
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("openai chat_completions raw: stream read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
		}
	}
	if !clientDisconnected && !clientOutputStarted {
		if refusalDetector.IsSilentRefusal() {
			return nil, newOpenAISilentRefusalFailoverError(c, account, requestID)
		}
		for _, pending := range pendingLines {
			if !writeLine(pending) {
				break
			}
		}
		pendingLines = pendingLines[:0]
		if !clientDisconnected && clientOutputStarted {
			maybeMarkClientFlush()
		}
	}

	return &OpenAIForwardResult{
		RequestID:       requestID,
		Usage:           usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		ReasoningEffort: reasoningEffort,
		ServiceTier:     serviceTier,
		Stream:          true,
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
	}, nil
}

// bufferRawChatCompletions 透传上游 CC 非流式 JSON 响应。
func ensureOpenAIChatStreamUsage(body []byte) ([]byte, error) {
	updated, err := sjson.SetBytes(body, "stream_options.include_usage", true)
	if err != nil {
		return body, err
	}
	return updated, nil
}

func rewriteOpenAIChatSSEUsageTokens(line string, mult *DisplayTokenMultipliers) string {
	if mult == nil || !mult.IsNonTrivial() {
		return line
	}
	payload, ok := extractOpenAISSEDataLine(line)
	if !ok || strings.TrimSpace(payload) == "" || strings.TrimSpace(payload) == "[DONE]" {
		return line
	}
	if !gjson.Get(payload, "usage").Exists() {
		return line
	}
	rewritten := rewriteOpenAIChatUsageTokens([]byte(payload), "usage", mult)
	return "data: " + string(rewritten)
}

func (s *OpenAIGatewayService) bufferRawChatCompletions(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, fmt.Errorf("read upstream body: %w", err)
	}

	var ccResp apicompat.ChatCompletionsResponse
	var usage OpenAIUsage
	if err := json.Unmarshal(respBody, &ccResp); err == nil && ccResp.Usage != nil {
		usage = OpenAIUsage{
			InputTokens:  ccResp.Usage.PromptTokens,
			OutputTokens: ccResp.Usage.CompletionTokens,
		}
		if ccResp.Usage.PromptTokensDetails != nil {
			usage.CacheReadInputTokens = ccResp.Usage.PromptTokensDetails.CachedTokens
		}
	}

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Writer.Header().Set("Content-Type", ct)
	} else {
		c.Writer.Header().Set("Content-Type", "application/json")
	}
	if mult := getDisplayTokenMultipliers(c); mult != nil {
		respBody = rewriteOpenAIChatUsageTokens(respBody, "usage", mult)
	}
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(respBody)

	return &OpenAIForwardResult{
		RequestID:       requestID,
		Usage:           usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		ReasoningEffort: reasoningEffort,
		ServiceTier:     serviceTier,
		Stream:          false,
		Duration:        time.Since(startTime),
	}, nil
}

type rawChatSSEAccumulator struct {
	id                string
	model             string
	systemFingerprint string
	serviceTier       string
	finishReason      string
	created           int64
	content           strings.Builder
	reasoning         strings.Builder
	usage             *OpenAIUsage
	toolCalls         map[int]*apicompat.ChatToolCall
	hasMessage        bool
}

func (a *rawChatSSEAccumulator) usable() bool {
	return a != nil && (a.hasMessage || a.finishReason != "" || a.usage != nil || len(a.toolCalls) > 0)
}

func (a *rawChatSSEAccumulator) processPayload(payload string) {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" || trimmed == "[DONE]" {
		return
	}
	var chunk apicompat.ChatCompletionsChunk
	if err := json.Unmarshal([]byte(trimmed), &chunk); err != nil {
		return
	}
	if chunk.ID != "" {
		a.id = chunk.ID
	}
	if chunk.Model != "" {
		a.model = chunk.Model
	}
	if chunk.Created != 0 {
		a.created = chunk.Created
	}
	if chunk.SystemFingerprint != "" {
		a.systemFingerprint = chunk.SystemFingerprint
	}
	if chunk.ServiceTier != "" {
		a.serviceTier = chunk.ServiceTier
	}
	if u := extractCCStreamUsage(trimmed); u != nil {
		copied := *u
		a.usage = &copied
	}
	for _, choice := range chunk.Choices {
		if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
			a.finishReason = strings.TrimSpace(*choice.FinishReason)
		}
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			_, _ = a.content.WriteString(*choice.Delta.Content)
			a.hasMessage = true
		}
		if reasoning := apicompat.ChatDeltaReasoningText(choice.Delta); reasoning != "" {
			_, _ = a.reasoning.WriteString(reasoning)
			a.hasMessage = true
		}
		for i, toolCall := range choice.Delta.ToolCalls {
			a.mergeToolCall(toolCall, i)
		}
	}
}

func (a *rawChatSSEAccumulator) mergeToolCall(toolCall apicompat.ChatToolCall, fallback int) {
	idx := fallback
	if toolCall.Index != nil {
		idx = *toolCall.Index
	}
	if a.toolCalls == nil {
		a.toolCalls = make(map[int]*apicompat.ChatToolCall)
	}
	stored, ok := a.toolCalls[idx]
	if !ok {
		copyCall := toolCall
		if strings.TrimSpace(copyCall.Type) == "" {
			copyCall.Type = "function"
		}
		a.toolCalls[idx] = &copyCall
		return
	}
	if toolCall.ID != "" {
		stored.ID = toolCall.ID
	}
	if toolCall.Type != "" {
		stored.Type = toolCall.Type
	}
	if toolCall.Function.Name != "" {
		stored.Function.Name = toolCall.Function.Name
	}
	if toolCall.Function.Arguments != "" {
		stored.Function.Arguments += toolCall.Function.Arguments
	}
}

func (a *rawChatSSEAccumulator) assembledToolCalls() []apicompat.ChatToolCall {
	if len(a.toolCalls) == 0 {
		return nil
	}
	idxs := make([]int, 0, len(a.toolCalls))
	for idx := range a.toolCalls {
		idxs = append(idxs, idx)
	}
	sort.Ints(idxs)
	out := make([]apicompat.ChatToolCall, 0, len(idxs))
	for _, idx := range idxs {
		tc := *a.toolCalls[idx]
		tc.Index = nil
		out = append(out, tc)
	}
	return out
}

func (a *rawChatSSEAccumulator) chatResponse(originalModel string) apicompat.ChatCompletionsResponse {
	model := a.model
	if model == "" {
		model = originalModel
	}
	toolCalls := a.assembledToolCalls()
	var contentRaw json.RawMessage
	if a.content.Len() == 0 && len(toolCalls) > 0 {
		contentRaw = json.RawMessage("null")
	} else {
		contentRaw, _ = json.Marshal(a.content.String())
	}
	finish := a.finishReason
	if finish == "" {
		if len(toolCalls) > 0 {
			finish = "tool_calls"
		} else {
			finish = "stop"
		}
	}
	resp := apicompat.ChatCompletionsResponse{
		ID:                a.id,
		Object:            "chat.completion",
		Created:           a.created,
		Model:             model,
		SystemFingerprint: a.systemFingerprint,
		ServiceTier:       a.serviceTier,
		Choices: []apicompat.ChatChoice{{
			Index: 0,
			Message: apicompat.ChatMessage{
				Role:             "assistant",
				Content:          contentRaw,
				ReasoningContent: a.reasoning.String(),
				ToolCalls:        toolCalls,
			},
			FinishReason: finish,
		}},
	}
	if a.usage != nil {
		resp.Usage = &apicompat.ChatUsage{
			PromptTokens:     a.usage.InputTokens,
			CompletionTokens: a.usage.OutputTokens,
			TotalTokens:      a.usage.InputTokens + a.usage.OutputTokens,
		}
		if a.usage.CacheReadInputTokens > 0 {
			resp.Usage.PromptTokensDetails = &apicompat.ChatTokenDetails{
				CachedTokens: a.usage.CacheReadInputTokens,
			}
		}
	}
	return resp
}

func (s *OpenAIGatewayService) bufferRawChatCompletionsFromSSE(
	c *gin.Context,
	resp *http.Response,
	account *Account,
	originalModel string,
	billingModel string,
	upstreamModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resolveOpenAIBufferedRequestID(resp, c)
	if account == nil {
		account = &Account{Platform: PlatformOpenAI}
	}

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	acc := &rawChatSSEAccumulator{}
	processLine := func(line string) {
		payload, ok := extractOpenAISSEDataLine(line)
		if !ok {
			return
		}
		acc.processPayload(payload)
	}

	finish := func() (*OpenAIForwardResult, error) {
		if !acc.usable() {
			writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Upstream stream ended without a terminal chat chunk")
			return nil, fmt.Errorf("upstream stream ended without terminal chat chunk")
		}
		chatResp := acc.chatResponse(originalModel)
		body, marshalErr := json.Marshal(chatResp)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal buffered raw chat completions: %w", marshalErr)
		}
		if s.responseHeaderFilter != nil && resp != nil {
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		}
		if mult := getDisplayTokenMultipliers(c); mult != nil {
			body = rewriteOpenAIChatUsageTokens(body, "usage", mult)
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", body)
		usage := OpenAIUsage{}
		if acc.usage != nil {
			usage = *acc.usage
		}
		return &OpenAIForwardResult{
			RequestID:       requestID,
			Usage:           usage,
			Model:           originalModel,
			BillingModel:    billingModel,
			UpstreamModel:   upstreamModel,
			ReasoningEffort: reasoningEffort,
			ServiceTier:     serviceTier,
			Stream:          false,
			Duration:        time.Since(startTime),
		}, nil
	}

	timeoutSeconds := 0
	if s.cfg != nil {
		timeoutSeconds = s.cfg.Gateway.StreamDataIntervalTimeout
	}
	interval := chatBufferedStreamInterval(timeoutSeconds)
	if interval <= 0 {
		for scanner.Scan() {
			processLine(scanner.Text())
		}
		if failover := s.handleChatBufferedReadError(c, account, requestID, scanner.Err()); failover != nil && !acc.usable() {
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
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return finish()
			}
			if ev.err != nil {
				if failover := s.handleChatBufferedReadError(c, account, requestID, ev.err); failover != nil && !acc.usable() {
					return nil, failover
				}
				return finish()
			}
			processLine(ev.line)
		case <-ticker.C:
			lastRead := time.Unix(0, atomic.LoadInt64(&lastReadAt))
			if time.Since(lastRead) < interval {
				continue
			}
			if acc.usable() {
				return finish()
			}
			logger.L().Warn("openai chat_completions raw buffered: stream data interval timeout",
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

// extractCCStreamUsage 从单个 CC 流式 chunk 的 payload 中提取 usage 字段。
// CC 协议中 usage 仅出现在末尾 chunk（且仅当客户端请求 stream_options.include_usage
// 时），但上游可能在多个 chunk 中重复——总是用最新值。
func isOpenAIChatUsageOnlyStreamChunk(payload string) bool {
	if strings.TrimSpace(payload) == "" {
		return false
	}
	if !gjson.Get(payload, "usage").Exists() {
		return false
	}
	choices := gjson.Get(payload, "choices")
	return choices.Exists() && choices.IsArray() && len(choices.Array()) == 0
}

func extractCCStreamUsage(payload string) *OpenAIUsage {
	usageResult := gjson.Get(payload, "usage")
	if !usageResult.Exists() || !usageResult.IsObject() {
		return nil
	}
	u := OpenAIUsage{
		InputTokens:  int(gjson.Get(payload, "usage.prompt_tokens").Int()),
		OutputTokens: int(gjson.Get(payload, "usage.completion_tokens").Int()),
	}
	if cached := gjson.Get(payload, "usage.prompt_tokens_details.cached_tokens"); cached.Exists() {
		u.CacheReadInputTokens = int(cached.Int())
	}
	return &u
}

// buildOpenAIChatCompletionsURL 拼接上游 Chat Completions 端点 URL。
//
//   - base 已是 /chat/completions：原样返回
//   - base 以 /v1 结尾：追加 /chat/completions
//   - 其他情况：追加 /v1/chat/completions
//
// 与 buildOpenAIResponsesURL 是姐妹函数。
func buildOpenAIChatCompletionsURL(base string) string {
	return buildOpenAIEndpointURL(base, "/v1/chat/completions")
}
