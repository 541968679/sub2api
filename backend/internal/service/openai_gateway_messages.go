package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const openAIClaudeGPTBridgeServiceContextKey = "openai_claude_gpt_bridge"

const (
	openAIAnthropicCompactKeepaliveContextKey       = "openai_anthropic_compact_keepalive"
	openAIAnthropicTransportStreamStartedContextKey = "openai_anthropic_transport_stream_started"
	openAIAnthropicSemanticOutputStartedContextKey  = "openai_anthropic_semantic_output_started"
	openAIAnthropicResponseTerminatedContextKey     = "openai_anthropic_response_terminated"
)

type openAIAnthropicCompactKeepaliveConfig struct {
	interval time.Duration
}

// MarkOpenAIAnthropicTransportStreamStarted records that the response is now
// an SSE stream. Transport-only events such as ping do not commit failover.
func MarkOpenAIAnthropicTransportStreamStarted(c *gin.Context) {
	if c != nil {
		c.Set(openAIAnthropicTransportStreamStartedContextKey, true)
	}
}

// OpenAIAnthropicTransportStreamStarted reports whether any Anthropic SSE bytes
// have been sent, including transport-only keepalive events.
func OpenAIAnthropicTransportStreamStarted(c *gin.Context) bool {
	if c == nil {
		return false
	}
	started, _ := c.Get(openAIAnthropicTransportStreamStartedContextKey)
	return started == true
}

// MarkOpenAIAnthropicResponseTerminated prevents panic/error fallbacks from
// appending a second terminal event after a completed response or SSE error.
func MarkOpenAIAnthropicResponseTerminated(c *gin.Context) {
	if c != nil {
		c.Set(openAIAnthropicResponseTerminatedContextKey, true)
	}
}

func OpenAIAnthropicResponseTerminated(c *gin.Context) bool {
	if c == nil {
		return false
	}
	terminated, _ := c.Get(openAIAnthropicResponseTerminatedContextKey)
	return terminated == true
}

func markOpenAIAnthropicSemanticOutputStarted(c *gin.Context) {
	if c != nil {
		c.Set(openAIAnthropicSemanticOutputStartedContextKey, true)
	}
}

func openAIAnthropicSemanticOutputStarted(c *gin.Context) bool {
	if c == nil {
		return false
	}
	started, _ := c.Get(openAIAnthropicSemanticOutputStartedContextKey)
	return started == true
}

func (s *OpenAIGatewayService) enableOpenAIAnthropicCompactKeepalive(
	c *gin.Context,
	clientStream bool,
	intervalOverride time.Duration,
) {
	if c == nil || !clientStream {
		return
	}
	interval := intervalOverride
	if interval <= 0 && s != nil && s.cfg != nil && s.cfg.Gateway.StreamKeepaliveInterval > 0 {
		interval = time.Duration(s.cfg.Gateway.StreamKeepaliveInterval) * time.Second
	}
	if interval <= 0 {
		return
	}
	c.Set(openAIAnthropicCompactKeepaliveContextKey, openAIAnthropicCompactKeepaliveConfig{interval: interval})
}

func openAIAnthropicCompactKeepaliveInterval(c *gin.Context) time.Duration {
	if c == nil {
		return 0
	}
	value, ok := c.Get(openAIAnthropicCompactKeepaliveContextKey)
	if !ok {
		return 0
	}
	settings, ok := value.(openAIAnthropicCompactKeepaliveConfig)
	if !ok || settings.interval <= 0 {
		return 0
	}
	return settings.interval
}

func isOpenAIClaudeGPTBridgeForward(c *gin.Context) bool {
	if c == nil {
		return false
	}
	enabled, _ := c.Get(openAIClaudeGPTBridgeServiceContextKey)
	return enabled == true
}

func logClaudeGPTBridgeRawUsage(stage, requestID string, accountID int64, originalModel, billingModel, upstreamModel string, inputTokens, outputTokens, cachedTokens int, stream bool) {
	logger.L().Info("openai claude-gpt bridge raw upstream usage",
		zap.String("stage", stage),
		zap.String("request_id", requestID),
		zap.Int64("account_id", accountID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Int("raw_input_tokens", inputTokens),
		zap.Int("raw_output_tokens", outputTokens),
		zap.Int("raw_cached_tokens", cachedTokens),
		zap.Bool("stream", stream),
	)
}

func logClaudeGPTBridgeUpstreamRequest(req *http.Request, promptCacheKey string, accountID int64, originalModel, billingModel, upstreamModel string, body []byte) {
	if req == nil {
		return
	}
	logger.L().Info("openai claude-gpt bridge upstream request diagnostics",
		zap.Int64("account_id", accountID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("body_has_prompt_cache_key", strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()) != ""),
		zap.String("body_prompt_cache_key_sha256", hashSensitiveValueForLog(gjson.GetBytes(body, "prompt_cache_key").String())),
		zap.Bool("arg_has_prompt_cache_key", strings.TrimSpace(promptCacheKey) != ""),
		zap.String("arg_prompt_cache_key_sha256", hashSensitiveValueForLog(promptCacheKey)),
		zap.Bool("header_has_session_id", strings.TrimSpace(req.Header.Get("session_id")) != ""),
		zap.String("header_session_id_sha256", hashSensitiveValueForLog(req.Header.Get("session_id"))),
		zap.Bool("header_has_conversation_id", strings.TrimSpace(req.Header.Get("conversation_id")) != ""),
		zap.String("header_conversation_id_sha256", hashSensitiveValueForLog(req.Header.Get("conversation_id"))),
		zap.Int("instructions_chars", len(gjson.GetBytes(body, "instructions").String())),
		zap.Int("input_item_count", len(gjson.GetBytes(body, "input").Array())),
		zap.Int("tool_count", len(gjson.GetBytes(body, "tools").Array())),
		zap.Int("request_body_bytes", len(body)),
	)
}

func logClaudeGPTBridgeConvertedUsage(stage, requestID string, accountID int64, originalModel, billingModel, upstreamModel string, usage apicompat.AnthropicUsage, stream bool) {
	logger.L().Info("openai claude-gpt bridge converted anthropic usage",
		zap.String("stage", stage),
		zap.String("request_id", requestID),
		zap.Int64("account_id", accountID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Int("input_tokens", usage.InputTokens),
		zap.Int("output_tokens", usage.OutputTokens),
		zap.Int("cache_creation_tokens", usage.CacheCreationInputTokens),
		zap.Int("cache_read_tokens", usage.CacheReadInputTokens),
		zap.Bool("stream", stream),
	)
}

func writeAnthropicJSONResponse(c *gin.Context, status int, resp *apicompat.AnthropicResponse) {
	if c == nil {
		return
	}
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if mult := getDisplayTokenMultipliers(c); mult != nil {
		body, err := json.Marshal(resp)
		if err == nil {
			body = rewriteNonStreamUsageTokens(body, mult)
			c.Data(status, "application/json; charset=utf-8", body)
			MarkOpenAIAnthropicResponseTerminated(c)
			return
		}
	}
	c.JSON(status, resp)
	MarkOpenAIAnthropicResponseTerminated(c)
}

func rewriteAnthropicSSEUsageTokens(sse string, mult *DisplayTokenMultipliers) string {
	if mult == nil || !mult.IsNonTrivial() {
		return sse
	}
	lines := strings.Split(sse, "\n")
	for i, line := range lines {
		lines[i] = RewriteSSEUsageTokens(line, mult)
	}
	return strings.Join(lines, "\n")
}

func (s *OpenAIGatewayService) applyClaudeGPTBridgeDisplayCacheOverride(
	ctx context.Context,
	usage *apicompat.ResponsesUsage,
	requestID string,
	accountID int64,
	originalModel string,
	billingModel string,
	upstreamModel string,
	stream bool,
) int {
	if usage == nil {
		return 0
	}
	if s == nil || s.settingService == nil {
		return 0
	}
	settings, err := s.settingService.GetOpenAIClaudeGPTBridgeCacheDisplaySettings(ctx)
	if err != nil {
		logger.L().Warn("openai claude-gpt bridge cache display settings unavailable",
			zap.Error(err),
			zap.String("request_id", requestID),
			zap.Int64("account_id", accountID),
		)
		return 0
	}
	if settings == nil || !settings.Enabled {
		return 0
	}

	upstreamCachedTokens := 0
	if usage.InputTokensDetails != nil {
		upstreamCachedTokens = usage.InputTokensDetails.CachedTokens
	}
	percent := settings.MinPercent
	if settings.MaxPercent > settings.MinPercent {
		percent += rand.Float64() * (settings.MaxPercent - settings.MinPercent)
	}
	inputTokens := usage.InputTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	cachedTokens := int(math.Round(float64(inputTokens) * percent / 100))
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	if cachedTokens > inputTokens {
		cachedTokens = inputTokens
	}
	if usage.InputTokensDetails == nil {
		usage.InputTokensDetails = &apicompat.ResponsesInputTokensDetails{}
	}
	usage.InputTokensDetails.CachedTokens = cachedTokens

	logger.L().Info("openai claude-gpt bridge generated cache display applied",
		zap.String("request_id", requestID),
		zap.Int64("account_id", accountID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Int("raw_input_tokens", usage.InputTokens),
		zap.Int("upstream_cached_tokens", upstreamCachedTokens),
		zap.Int("display_cached_tokens", cachedTokens),
		zap.Float64("min_percent", settings.MinPercent),
		zap.Float64("max_percent", settings.MaxPercent),
		zap.Float64("chosen_percent", percent),
		zap.Bool("stream", stream),
	)
	return cachedTokens
}

// ForwardAsAnthropic accepts an Anthropic Messages request body, converts it
// to OpenAI Responses API format, forwards to the OpenAI upstream, and converts
// the response back to Anthropic Messages format. This enables Claude Code
// clients to access OpenAI models through the standard /v1/messages endpoint.
func (s *OpenAIGatewayService) ForwardAsAnthropic(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	promptCacheKey string,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	bridgeMode := isOpenAIClaudeGPTBridgeForward(c)

	// 1. Parse Anthropic request
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	anthropicDigestReq := cloneAnthropicRequestForDigest(&anthropicReq)
	// Keep the untouched transcript for compact recovery. API-key compatibility
	// may trim the forwarding copy to the latest 12 messages below.
	anthropicCompactReq := cloneAnthropicRequestForDigest(&anthropicReq)
	originalModel := anthropicReq.Model
	applyOpenAICompatModelNormalization(&anthropicReq)
	normalizedModel := anthropicReq.Model
	clientStream := anthropicReq.Stream // client's original stream preference

	// 2. Model mapping
	billingModel := resolveOpenAIForwardModel(account, normalizedModel, defaultMappedModel)
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	anthropicCompactRequest := isClaudeCodeCompactAnthropicRequest(anthropicCompactReq)
	if anthropicCompactRequest {
		s.enableOpenAIAnthropicCompactKeepalive(c, clientStream, 0)
	}
	anthropicCompactModelMapped := false
	anthropicCompactRequestedModel := billingModel
	var anthropicCompactFallbackUpstreamModels []string
	if anthropicCompactRequest && account != nil {
		if compactModel, matched := account.ResolveCompactMappedModel(billingModel); matched {
			compactModel = strings.TrimSpace(compactModel)
			if compactModel != "" {
				billingModel = compactModel
				upstreamModel = normalizeOpenAIModelForUpstream(account, compactModel)
				anthropicCompactModelMapped = true
			}
		}
		anthropicCompactFallbackUpstreamModels = resolveOpenAICompactFallbackForwardModels(
			account, anthropicCompactRequestedModel, billingModel,
		)
	}
	promptCacheKey = strings.TrimSpace(promptCacheKey)
	apiKeyID := getAPIKeyIDFromContext(c)
	anthropicDigestChain := ""
	anthropicMatchedDigestChain := ""
	compatPromptCacheInjected := false
	if promptCacheKey == "" && shouldAutoInjectPromptCacheKeyForCompat(upstreamModel) {
		promptCacheKey = promptCacheKeyFromAnthropicMetadataSession(&anthropicReq)
		if promptCacheKey == "" {
			promptCacheKey = deriveAnthropicCacheControlPromptCacheKey(&anthropicReq)
		}
		if promptCacheKey == "" {
			anthropicDigestChain = buildOpenAICompatAnthropicDigestChain(anthropicDigestReq)
			if reusedKey, matchedChain := s.findOpenAICompatAnthropicDigestPromptCacheKey(account, apiKeyID, anthropicDigestChain); reusedKey != "" {
				promptCacheKey = reusedKey
				anthropicMatchedDigestChain = matchedChain
			} else {
				promptCacheKey = promptCacheKeyFromAnthropicDigest(anthropicDigestChain)
			}
		}
		compatPromptCacheInjected = promptCacheKey != ""
	}
	compatReplayTrimmed := false
	compatReplayGuardEnabled := shouldAutoInjectPromptCacheKeyForCompat(upstreamModel)
	compatContinuationEnabled := openAICompatContinuationEnabled(account, upstreamModel)
	previousResponseID := ""
	if compatContinuationEnabled {
		previousResponseID = s.getOpenAICompatSessionResponseID(ctx, c, account, promptCacheKey)
	}
	compatContinuationDisabled := compatContinuationEnabled &&
		s.isOpenAICompatSessionContinuationDisabled(ctx, c, account, promptCacheKey)
	compatTurnState := ""
	// OAuth/Plus relies on session_id + x-codex-turn-state; trimming to a
	// sliding 12-message window makes the cached prefix stall at system/tools.
	// Keep full replay there so upstream prompt caching can grow turn by turn.
	if compatReplayGuardEnabled && account.Type != AccountTypeOAuth && previousResponseID == "" && !compatContinuationDisabled {
		compatReplayTrimmed = applyAnthropicCompatFullReplayGuard(&anthropicReq)
	}

	// 3. Convert Anthropic → Responses after compatibility-only replay guard.
	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("convert anthropic to responses: %w", err)
	}

	// Upstream always uses streaming (upstream may not support sync mode).
	// The client's original preference determines the response format.
	responsesReq.Stream = true
	isStream := true

	// 3b. Handle BetaFastMode → service_tier: "priority"
	if containsBetaToken(c.GetHeader("anthropic-beta"), claude.BetaFastMode) {
		responsesReq.ServiceTier = "priority"
	}

	responsesReq.Model = upstreamModel
	if previousResponseID != "" {
		responsesReq.PreviousResponseID = previousResponseID
		trimAnthropicCompatResponsesInputToLatestTurn(responsesReq)
	}
	if compatReplayGuardEnabled && account.Type != AccountTypeOAuth {
		appendOpenAICompatClaudeCodeTodoGuard(responsesReq)
	}

	logFields := []zap.Field{
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("normalized_model", normalizedModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
		zap.Bool("stream", isStream),
	}
	if anthropicCompactRequest {
		logFields = append(logFields,
			zap.Bool("anthropic_compact_request", true),
			zap.Bool("anthropic_compact_model_mapped", anthropicCompactModelMapped),
			zap.Int("anthropic_compact_full_message_count", len(anthropicCompactReq.Messages)),
		)
		if len(anthropicCompactFallbackUpstreamModels) > 0 {
			logFields = append(logFields,
				zap.Strings("anthropic_compact_fallback_upstream_models", anthropicCompactFallbackUpstreamModels),
			)
		}
	}
	if compatPromptCacheInjected {
		logFields = append(logFields,
			zap.Bool("compat_prompt_cache_key_injected", true),
			zap.String("compat_prompt_cache_key_sha256", hashSensitiveValueForLog(promptCacheKey)),
		)
	}
	if compatReplayTrimmed {
		logFields = append(logFields,
			zap.Bool("compat_full_replay_trimmed", true),
			zap.Int("compat_messages_after_trim", len(anthropicReq.Messages)),
		)
	}
	if previousResponseID != "" {
		logFields = append(logFields,
			zap.Bool("compat_previous_response_id_attached", true),
			zap.String("compat_previous_response_id", truncateOpenAIWSLogValue(previousResponseID, openAIWSIDValueMaxLen)),
		)
	}
	if compatTurnState != "" {
		logFields = append(logFields, zap.Bool("compat_turn_state_attached", true))
	}
	logger.L().Debug("openai messages: model mapping applied", logFields...)

	// 4. Marshal Responses request body, then apply OAuth codex transform
	responsesBody, err := json.Marshal(responsesReq)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	if account.Platform == PlatformGrok {
		return s.forwardGrokResponses(ctx, c, account, responsesBody, originalModel, clientStream, startTime)
	}

	if account.Type == AccountTypeOAuth {
		var reqBody map[string]any
		if err := json.Unmarshal(responsesBody, &reqBody); err != nil {
			return nil, fmt.Errorf("unmarshal for codex transform: %w", err)
		}
		codexResult := applyCodexOAuthTransformWithOptions(reqBody, codexOAuthTransformOptions{
			SkipDefaultInstructions: true,
			PreserveToolCallIDs:     true,
		})
		forcedTemplateText := ""
		if s.cfg != nil {
			forcedTemplateText = s.cfg.Gateway.ForcedCodexInstructionsTemplate
		}
		templateUpstreamModel := upstreamModel
		if codexResult.NormalizedModel != "" {
			templateUpstreamModel = codexResult.NormalizedModel
		}
		existingInstructions, _ := reqBody["instructions"].(string)
		if strings.TrimSpace(existingInstructions) == "" {
			existingInstructions = extractPromptLikeInstructionsFromInput(reqBody)
		}
		if _, err := applyForcedCodexInstructionsTemplate(reqBody, forcedTemplateText, forcedCodexInstructionsTemplateData{
			ExistingInstructions: strings.TrimSpace(existingInstructions),
			OriginalModel:        originalModel,
			NormalizedModel:      normalizedModel,
			BillingModel:         billingModel,
			UpstreamModel:        templateUpstreamModel,
		}); err != nil {
			return nil, err
		}
		ensureCodexOAuthInstructionsField(reqBody)
		if shouldAutoInjectPromptCacheKeyForCompat(upstreamModel) {
			appendOpenAICompatClaudeCodeTodoGuardToRequestBody(reqBody)
		}
		if codexResult.NormalizedModel != "" {
			upstreamModel = codexResult.NormalizedModel
		}
		if codexResult.PromptCacheKey != "" {
			promptCacheKey = codexResult.PromptCacheKey
		}
		if bridgeMode {
			if promptCacheKey != "" {
				reqBody["prompt_cache_key"] = promptCacheKey
			}
		} else {
			delete(reqBody, "prompt_cache_key")
		}
		if shouldAutoInjectPromptCacheKeyForCompat(upstreamModel) {
			compatTurnState = s.getOpenAICompatSessionTurnState(ctx, c, account, promptCacheKey)
		}
		// OAuth codex transform forces stream=true upstream, so always use
		// the streaming response handler regardless of what the client asked.
		isStream = true
		responsesBody, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("remarshal after codex transform: %w", err)
		}
	}

	// For API key accounts (including OpenAI-compatible upstream gateways),
	// ensure promptCacheKey is also propagated via the request body so that
	// upstreams using the Responses API can derive a stable session identifier
	// from prompt_cache_key. This makes our Anthropic /v1/messages compatibility
	// path behave more like a native Responses client.
	if account.Type == AccountTypeAPIKey {
		if trimmedKey := strings.TrimSpace(promptCacheKey); trimmedKey != "" {
			var reqBody map[string]any
			if err := json.Unmarshal(responsesBody, &reqBody); err != nil {
				return nil, fmt.Errorf("unmarshal for prompt cache key injection: %w", err)
			}
			if existing, ok := reqBody["prompt_cache_key"].(string); !ok || strings.TrimSpace(existing) == "" {
				reqBody["prompt_cache_key"] = trimmedKey
				updated, err := json.Marshal(reqBody)
				if err != nil {
					return nil, fmt.Errorf("remarshal after prompt cache key injection: %w", err)
				}
				responsesBody = updated
			}
		}
	}

	// 4c. Apply OpenAI fast policy (may filter service_tier or block the request).
	// Mirrors the Claude anthropic-beta "fast-mode-2026-02-01" filter, but keyed
	// on the body-level service_tier field (priority/flex).
	updatedBody, policyErr := s.applyOpenAIFastPolicyToBody(ctx, account, upstreamModel, responsesBody)
	if policyErr != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(policyErr, &blocked) {
			writeAnthropicError(c, http.StatusForbidden, "forbidden_error", blocked.Message)
		}
		return nil, policyErr
	}
	responsesBody = updatedBody

	// 5. Get access token
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	// 6. Build upstream request
	upstreamCtx, releaseUpstreamCtx := detachStreamUpstreamContext(ctx, isStream)
	upstreamReq, err := s.buildUpstreamRequest(upstreamCtx, c, account, responsesBody, token, isStream, promptCacheKey, false)
	releaseUpstreamCtx()
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}

	// Override session_id with a deterministic UUID derived from the isolated
	// session key, ensuring different API keys produce different upstream sessions.
	if promptCacheKey != "" {
		isolatedSessionID := generateSessionUUID(isolateOpenAISessionID(apiKeyID, promptCacheKey))
		upstreamReq.Header.Set("session_id", isolatedSessionID)
		if upstreamReq.Header.Get("conversation_id") != "" {
			upstreamReq.Header.Set("conversation_id", isolatedSessionID)
		}
	}
	if account.Type == AccountTypeOAuth {
		// Anthropic Messages compatibility uses the ChatGPT Codex SSE endpoint.
		// Match airgate-openai's request shape: the SSE endpoint does not need
		// the Responses experimental beta header, and forcing originator can make
		// ChatGPT select a different internal continuation path.
		upstreamReq.Header.Del("OpenAI-Beta")
		upstreamReq.Header.Del("originator")
	}
	if account.Type == AccountTypeOAuth && promptCacheKey != "" && strings.TrimSpace(c.GetHeader("conversation_id")) == "" {
		upstreamReq.Header.Del("conversation_id")
	}
	if compatTurnState != "" && upstreamReq.Header.Get("x-codex-turn-state") == "" {
		upstreamReq.Header.Set("x-codex-turn-state", compatTurnState)
	}
	if bridgeMode {
		upstreamReq.Header.Del("session_id")
		upstreamReq.Header.Del("conversation_id")
		logClaudeGPTBridgeUpstreamRequest(upstreamReq, promptCacheKey, account.ID, originalModel, billingModel, upstreamModel, responsesBody)
	}

	// 7. Send request
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	var resp *http.Response
	if anthropicCompactRequest {
		resp, err = s.doOpenAIAnthropicRequestWithCompactKeepalive(
			c, upstreamReq, proxyURL, account.ID, account.Concurrency,
		)
	} else {
		resp, err = s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	}
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		if anthropicCompactRequest {
			return nil, s.newOpenAIStreamFailoverError(
				c, account, false, "", nil, "OpenAI compact upstream request failed: "+safeErr,
			)
		}
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Kind:               "request_error",
			Message:            safeErr,
		})
		writeAnthropicError(c, http.StatusBadGateway, "api_error", "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	// 8. Handle error response with failover
	if resp.StatusCode >= 400 {
		respBody := s.readUpstreamErrorBody(resp)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		if anthropicCompactRequest && isOpenAICompactContextLengthHTTPError(resp.StatusCode, upstreamMsg, respBody) {
			initialUsage, _ := extractOpenAIUsageFromJSONBytes(respBody)
			candidates := append([]string{upstreamModel}, anthropicCompactFallbackUpstreamModels...)
			logger.L().Warn("openai_messages.compact_http_context_recovery_started",
				zap.Int64("account_id", account.ID),
				zap.String("model", originalModel),
				zap.String("upstream_model", upstreamModel),
				zap.Strings("candidate_upstream_models", candidates),
				zap.Int("upstream_status", resp.StatusCode),
				zap.String("upstream_request_id", resp.Header.Get("x-request-id")),
			)
			result, recoveryErr := s.runAnthropicCompactRecoveryWithModelFallbacks(
				ctx, c, account, anthropicCompactReq, token, bridgeMode, originalModel,
				candidates, startTime, initialUsage, clientStream, resp.Header.Get("x-request-id"),
			)
			if recoveryErr == nil && result != nil && result.SkipContinuationBinding {
				s.deleteOpenAICompatSessionContinuation(ctx, c, account, promptCacheKey)
			}
			return result, recoveryErr
		}
		if anthropicCompactRequest && len(anthropicCompactFallbackUpstreamModels) > 0 &&
			isOpenAICompactModelUnavailableHTTP(resp.StatusCode, upstreamMsg, respBody) {
			logger.L().Warn("openai_messages.compact_model_unavailable_fallback_started",
				zap.Int64("account_id", account.ID),
				zap.String("model", originalModel),
				zap.String("upstream_model", upstreamModel),
				zap.Strings("fallback_upstream_models", anthropicCompactFallbackUpstreamModels),
				zap.Int("upstream_status", resp.StatusCode),
				zap.String("upstream_request_id", resp.Header.Get("x-request-id")),
			)
			result, recoveryErr := s.runAnthropicCompactRecoveryWithModelFallbacks(
				ctx, c, account, anthropicCompactReq, token, bridgeMode, originalModel,
				anthropicCompactFallbackUpstreamModels, startTime, OpenAIUsage{}, clientStream,
				resp.Header.Get("x-request-id"),
			)
			if recoveryErr == nil && result != nil && result.SkipContinuationBinding {
				s.deleteOpenAICompatSessionContinuation(ctx, c, account, promptCacheKey)
			}
			return result, recoveryErr
		}
		if previousResponseID != "" && (isOpenAICompatPreviousResponseNotFound(resp.StatusCode, upstreamMsg, respBody) || isOpenAICompatPreviousResponseUnsupported(resp.StatusCode, upstreamMsg, respBody)) {
			if isOpenAICompatPreviousResponseUnsupported(resp.StatusCode, upstreamMsg, respBody) {
				s.disableOpenAICompatSessionContinuation(ctx, c, account, promptCacheKey)
			} else {
				s.deleteOpenAICompatSessionResponseID(ctx, c, account, promptCacheKey)
			}
			logger.L().Info("openai messages: previous_response_id unavailable, retrying without continuation",
				zap.Int64("account_id", account.ID),
				zap.String("previous_response_id", truncateOpenAIWSLogValue(previousResponseID, openAIWSIDValueMaxLen)),
				zap.String("upstream_model", upstreamModel),
			)
			return s.ForwardAsAnthropic(ctx, c, account, body, promptCacheKey, defaultMappedModel)
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
			s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, upstreamModel)
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && (isPoolModeRetryableStatus(resp.StatusCode) || isOpenAITransientProcessingError(resp.StatusCode, upstreamMsg, respBody)),
			}
		}
		// Non-failover error: return Anthropic-formatted error to client
		return s.handleAnthropicErrorResponse(resp, c, account, billingModel)
	}

	// 9. Handle normal response
	// Upstream is always streaming; choose response format based on client preference.
	var result *OpenAIForwardResult
	var handleErr error
	if anthropicCompactRequest {
		result, handleErr = s.handleAnthropicCompactStreamingResponse(
			ctx, c, account, resp, bridgeMode, originalModel, billingModel, upstreamModel,
			anthropicCompactFallbackUpstreamModels, startTime, anthropicCompactReq, token, clientStream,
		)
	} else if clientStream {
		result, handleErr = s.handleAnthropicStreamingResponse(resp, c, account, bridgeMode, originalModel, billingModel, upstreamModel, startTime)
	} else {
		// Client wants JSON: buffer the streaming response and assemble a JSON reply.
		result, handleErr = s.handleAnthropicBufferedStreamingResponse(resp, c, account, bridgeMode, originalModel, billingModel, upstreamModel, startTime)
	}

	// Propagate ServiceTier and ReasoningEffort to result for billing
	if handleErr == nil && result != nil {
		if result.SkipContinuationBinding {
			s.deleteOpenAICompatSessionContinuation(ctx, c, account, promptCacheKey)
		}
		if account.Type == AccountTypeOAuth && promptCacheKey != "" && !result.SkipContinuationBinding {
			if turnState := strings.TrimSpace(resp.Header.Get("x-codex-turn-state")); turnState != "" {
				s.bindOpenAICompatSessionTurnState(ctx, c, account, promptCacheKey, turnState)
			}
		}
		if compatContinuationEnabled && promptCacheKey != "" && result.ResponseID != "" && !result.SkipContinuationBinding {
			s.bindOpenAICompatSessionResponseID(ctx, c, account, promptCacheKey, result.ResponseID)
		}
		if promptCacheKey != "" && anthropicDigestChain != "" {
			s.bindOpenAICompatAnthropicDigestPromptCacheKey(account, apiKeyID, anthropicDigestChain, promptCacheKey, anthropicMatchedDigestChain)
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

	// Extract and save Codex usage snapshot from response headers (for OAuth accounts)
	if handleErr == nil && account.Type == AccountTypeOAuth {
		if snapshot := ParseCodexRateLimitHeaders(resp.Header); snapshot != nil {
			s.updateCodexUsageSnapshot(ctx, account.ID, snapshot)
		}
	}

	return result, handleErr
}

// handleAnthropicErrorResponse reads an upstream error and returns it in
// Anthropic error format.
func (s *OpenAIGatewayService) handleAnthropicErrorResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestedModel ...string,
) (*OpenAIForwardResult, error) {
	return s.handleCompatErrorResponse(resp, c, account, writeAnthropicError, requestedModel...)
}

// handleAnthropicBufferedStreamingResponse reads all Responses SSE events from
// the upstream streaming response, finds the terminal event (response.completed
// / response.incomplete / response.failed), converts the complete response to
// Anthropic Messages JSON format, and writes it to the client.
// This is used when the client requested stream=false but the upstream is always
// streaming.
func (s *OpenAIGatewayService) handleAnthropicBufferedStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	bridgeMode bool,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	finalResponse, usage, acc, err := s.readOpenAICompatBufferedTerminal(resp, "openai messages buffered", requestID)
	if err != nil {
		return nil, err
	}

	if finalResponse == nil {
		return nil, s.newOpenAIStreamFailoverError(c, account, false, requestID, nil,
			"OpenAI messages stream ended without a terminal response event")
	}

	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	if bridgeMode && finalResponse.Usage != nil {
		rawCachedTokens := 0
		if finalResponse.Usage.InputTokensDetails != nil {
			rawCachedTokens = finalResponse.Usage.InputTokensDetails.CachedTokens
		}
		logClaudeGPTBridgeRawUsage("buffered_terminal", requestID, accountID, originalModel, billingModel, upstreamModel, finalResponse.Usage.InputTokens, finalResponse.Usage.OutputTokens, rawCachedTokens, false)
		s.applyClaudeGPTBridgeDisplayCacheOverride(c.Request.Context(), finalResponse.Usage, requestID, accountID, originalModel, billingModel, upstreamModel, false)
		usage = copyOpenAIUsageFromResponsesUsage(finalResponse.Usage)
	}

	if strings.TrimSpace(finalResponse.Status) == "failed" {
		payload, _ := json.Marshal(gin.H{"type": "response.failed", "response": finalResponse})
		return nil, s.openAIMessagesTerminalFailureError(c, account, requestID, finalResponse, payload)
	}

	// When the terminal event has an empty output array, reconstruct from
	// accumulated delta events so the client receives the full content.
	acc.SupplementResponseOutput(finalResponse)

	anthropicResp := apicompat.ResponsesToAnthropic(finalResponse, originalModel)
	if !anthropicResponseHasVisibleOutput(anthropicResp) {
		result := &OpenAIForwardResult{
			RequestID:     requestID,
			ResponseID:    finalResponse.ID,
			Usage:         usage,
			Model:         originalModel,
			BillingModel:  billingModel,
			UpstreamModel: upstreamModel,
			Stream:        false,
			Duration:      time.Since(startTime),
		}
		if strings.TrimSpace(finalResponse.Status) == "incomplete" &&
			finalResponse.IncompleteDetails != nil &&
			strings.TrimSpace(finalResponse.IncompleteDetails.Reason) == "max_output_tokens" {
			return result, s.newOpenAIStreamClientError(c, account, requestID, http.StatusBadRequest,
				"invalid_request_error",
				"OpenAI response reached max_output_tokens before producing assistant content; reduce the conversation context or output budget and try again.")
		}
		return result, s.newOpenAIStreamFailoverError(c, account, false, requestID, nil,
			"OpenAI messages response completed without assistant content or tool output")
	}
	if bridgeMode {
		logClaudeGPTBridgeConvertedUsage("buffered_response", requestID, accountID, originalModel, billingModel, upstreamModel, anthropicResp.Usage, false)
	}

	if s.responseHeaderFilter != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	}
	writeAnthropicJSONResponse(c, http.StatusOK, anthropicResp)

	return &OpenAIForwardResult{
		RequestID:     requestID,
		ResponseID:    finalResponse.ID,
		Usage:         usage,
		Model:         originalModel,
		BillingModel:  billingModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) openAIMessagesTerminalFailureError(
	c *gin.Context,
	account *Account,
	requestID string,
	response *apicompat.ResponsesResponse,
	payload []byte,
) *UpstreamFailoverError {
	message := extractOpenAISSEErrorMessage(payload)
	if message == "" {
		message = "OpenAI response failed before producing assistant content"
	}
	code := ""
	errType := ""
	if response != nil && response.Error != nil {
		code = strings.TrimSpace(response.Error.Code)
		errType = strings.TrimSpace(response.Error.Type)
		if message == "" {
			message = strings.TrimSpace(response.Error.Message)
		}
	}
	if strings.EqualFold(code, "context_length_exceeded") ||
		(strings.Contains(strings.ToLower(message), "context window") && strings.Contains(strings.ToLower(message), "exceed")) {
		return s.newOpenAIStreamClientError(c, account, requestID, http.StatusBadRequest,
			"invalid_request_error", message)
	}
	if openAIStreamFailedEventShouldFailover(payload, message) {
		return s.newOpenAIStreamFailoverError(c, account, false, requestID, payload, message)
	}
	if errType == "" {
		errType = code
	}
	if !strings.EqualFold(errType, "invalid_request_error") {
		errType = "invalid_request_error"
	}
	return s.newOpenAIStreamClientError(c, account, requestID, http.StatusBadRequest, errType, message)
}

func anthropicResponseHasVisibleOutput(resp *apicompat.AnthropicResponse) bool {
	if resp == nil {
		return false
	}
	for _, block := range resp.Content {
		switch strings.TrimSpace(block.Type) {
		case "text":
			if strings.TrimSpace(block.Text) != "" {
				return true
			}
		case "tool_use", "server_tool_use":
			return true
		}
	}
	return false
}

func isOpenAICompatResponsesTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.completed", "response.done", "response.incomplete", "response.failed":
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) recordOpenAIMessagesStreamUpstreamError(c *gin.Context, account *Account, upstreamRequestID, kind, message string) {
	if c == nil {
		return
	}
	message = sanitizeUpstreamErrorMessage(message)
	setOpsUpstreamError(c, http.StatusBadGateway, message, "")
	event := OpsUpstreamErrorEvent{
		Platform:           PlatformOpenAI,
		UpstreamStatusCode: http.StatusBadGateway,
		UpstreamRequestID:  strings.TrimSpace(upstreamRequestID),
		Kind:               kind,
		Message:            message,
	}
	if account != nil {
		event.Platform = account.Platform
		event.AccountID = account.ID
		event.AccountName = account.Name
	}
	appendOpsUpstreamError(c, event)
}

func isOpenAICompatDoneSentinelLine(line string) bool {
	payload, ok := extractOpenAISSEDataLine(line)
	return ok && strings.TrimSpace(payload) == "[DONE]"
}

func (s *OpenAIGatewayService) readOpenAICompatBufferedTerminal(
	resp *http.Response,
	logPrefix string,
	requestID string,
) (*apicompat.ResponsesResponse, OpenAIUsage, *apicompat.BufferedResponseAccumulator, error) {
	return s.readOpenAICompatBufferedTerminalWithKeepalive(resp, logPrefix, requestID, nil, nil)
}

func (s *OpenAIGatewayService) readOpenAICompatBufferedTerminalWithKeepalive(
	resp *http.Response,
	logPrefix string,
	requestID string,
	c *gin.Context,
	afterKeepalive func(),
) (*apicompat.ResponsesResponse, OpenAIUsage, *apicompat.BufferedResponseAccumulator, error) {
	acc := apicompat.NewBufferedResponseAccumulator()
	var usage OpenAIUsage
	if resp == nil || resp.Body == nil {
		return nil, usage, acc, errors.New("upstream response body is nil")
	}

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	streamInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamDataIntervalTimeout > 0 {
		streamInterval = time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout) * time.Second
	}
	var timeoutCh <-chan time.Time
	var timeoutTimer *time.Timer
	resetTimeout := func() {
		if streamInterval <= 0 {
			return
		}
		if timeoutTimer == nil {
			timeoutTimer = time.NewTimer(streamInterval)
			timeoutCh = timeoutTimer.C
			return
		}
		if !timeoutTimer.Stop() {
			select {
			case <-timeoutTimer.C:
			default:
			}
		}
		timeoutTimer.Reset(streamInterval)
	}
	stopTimeout := func() {
		if timeoutTimer == nil {
			return
		}
		if !timeoutTimer.Stop() {
			select {
			case <-timeoutTimer.C:
			default:
			}
		}
	}
	resetTimeout()
	defer stopTimeout()

	keepaliveInterval := openAIAnthropicCompactKeepaliveInterval(c)
	var keepaliveTicker *time.Ticker
	if keepaliveInterval > 0 {
		keepaliveTicker = time.NewTicker(keepaliveInterval)
		defer keepaliveTicker.Stop()
	}
	var keepaliveCh <-chan time.Time
	if keepaliveTicker != nil {
		keepaliveCh = keepaliveTicker.C
	}

	type scanEvent struct {
		line string
		err  error
	}
	events := make(chan scanEvent, 16)
	done := make(chan struct{})
	go func() {
		defer close(events)
		for scanner.Scan() {
			select {
			case events <- scanEvent{line: scanner.Text()}:
			case <-done:
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case events <- scanEvent{err: err}:
			case <-done:
			}
		}
	}()
	defer close(done)

	var parser openAICompatSSEFrameParser
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				if frame, ok := parser.Finish(); ok {
					payload := openAICompatPayloadWithEventType(frame.Data, frame.EventType)
					var event apicompat.ResponsesStreamEvent
					if err := json.Unmarshal([]byte(payload), &event); err == nil {
						acc.ProcessEvent(&event)
						if isOpenAICompatResponsesTerminalEvent(event.Type) && event.Response != nil {
							if event.Usage != nil {
								usage = copyOpenAIUsageFromResponsesUsage(event.Usage)
								if event.Response.Usage == nil {
									event.Response.Usage = event.Usage
								}
							}
							if event.Response.Usage != nil {
								usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
							}
							return event.Response, usage, acc, nil
						}
					}
				}
				return nil, usage, acc, nil
			}
			resetTimeout()
			if ev.err != nil {
				if !errors.Is(ev.err, context.Canceled) && !errors.Is(ev.err, context.DeadlineExceeded) {
					logger.L().Warn(logPrefix+": read error",
						zap.Error(ev.err),
						zap.String("request_id", requestID),
					)
				}
				return nil, usage, acc, ev.err
			}

			if isOpenAICompatDoneSentinelLine(ev.line) {
				return nil, usage, acc, nil
			}
			frame, ok := parser.AddLine(ev.line)
			if !ok {
				continue
			}
			payload := openAICompatPayloadWithEventType(frame.Data, frame.EventType)

			var event apicompat.ResponsesStreamEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				logger.L().Warn(logPrefix+": failed to parse event",
					zap.Error(err),
					zap.String("request_id", requestID),
				)
				continue
			}

			acc.ProcessEvent(&event)

			if isOpenAICompatResponsesTerminalEvent(event.Type) && event.Response != nil {
				if event.Usage != nil {
					usage = copyOpenAIUsageFromResponsesUsage(event.Usage)
					if event.Response.Usage == nil {
						event.Response.Usage = event.Usage
					}
				}
				if event.Response.Usage != nil {
					usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
				}
				return event.Response, usage, acc, nil
			}

		case <-timeoutCh:
			_ = resp.Body.Close()
			logger.L().Warn(logPrefix+": data interval timeout",
				zap.String("request_id", requestID),
				zap.Duration("interval", streamInterval),
			)
			return nil, usage, acc, fmt.Errorf("stream data interval timeout")

		case <-keepaliveCh:
			if err := writeAnthropicCompactKeepalive(c); err != nil {
				_ = resp.Body.Close()
				return nil, usage, acc, fmt.Errorf("write compact keepalive: %w", err)
			}
			if afterKeepalive != nil {
				afterKeepalive()
			}
		}
	}
}

// handleAnthropicStreamingResponse reads Responses SSE events from upstream,
// converts each to Anthropic SSE events, and writes them to the client.
// When StreamKeepaliveInterval is configured, it uses a goroutine + channel
// pattern to send Anthropic ping events during periods of upstream silence,
// preventing proxy/client timeout disconnections.
func (s *OpenAIGatewayService) handleAnthropicStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	bridgeMode bool,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := resp.Header.Get("x-request-id")

	headersWritten := false
	writeStreamHeaders := func() {
		if headersWritten {
			return
		}
		headersWritten = true
		if s.responseHeaderFilter != nil {
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		}
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
	}

	state := apicompat.NewResponsesEventToAnthropicState()
	state.Model = originalModel
	var usage OpenAIUsage
	responseID := ""
	var firstTokenMs *int
	firstChunk := true
	clientDisconnected := false
	clientOutputStarted := false
	clientVisibleOutputStarted := false
	var pendingClientSSE []string
	var terminalResponse *apicompat.ResponsesResponse
	var terminalFailureErr *UpstreamFailoverError
	var terminalStreamErr error
	streamDiag := newOpenAIMessagesStreamDiagnostic()
	resetKeepaliveTimer := func() {}

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	streamInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamDataIntervalTimeout > 0 {
		streamInterval = time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout) * time.Second
	}
	var intervalTicker *time.Ticker
	if streamInterval > 0 {
		intervalTicker = time.NewTicker(streamInterval)
		defer intervalTicker.Stop()
	}
	var intervalCh <-chan time.Time
	if intervalTicker != nil {
		intervalCh = intervalTicker.C
	}

	// resultWithUsage builds the final result snapshot.
	resultWithUsage := func() *OpenAIForwardResult {
		return &OpenAIForwardResult{
			RequestID:           requestID,
			ResponseID:          responseID,
			Usage:               usage,
			Model:               originalModel,
			BillingModel:        billingModel,
			UpstreamModel:       upstreamModel,
			Stream:              true,
			Duration:            time.Since(startTime),
			FirstTokenMs:        firstTokenMs,
			ClientDisconnect:    clientDisconnected,
			ClientOutputStarted: clientOutputStarted,
		}
	}

	flushPendingClientSSE := func() {
		if clientDisconnected || len(pendingClientSSE) == 0 {
			return
		}
		writeStreamHeaders()
		for _, sse := range pendingClientSSE {
			if _, err := fmt.Fprint(c.Writer, sse); err != nil {
				clientDisconnected = true
				logger.L().Info("openai messages stream: client disconnected during buffered flush",
					zap.String("request_id", requestID),
				)
				break
			}
			clientOutputStarted = true
		}
		pendingClientSSE = pendingClientSSE[:0]
		if !clientDisconnected {
			c.Writer.Flush()
			resetKeepaliveTimer()
		}
	}

	// processDataLine handles a single "data: ..." SSE line from upstream.
	processDataLine := func(payload string) bool {
		if firstChunk {
			firstChunk = false
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}

		var event apicompat.ResponsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			logger.L().Warn("openai messages stream: failed to parse event",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
			return false
		}
		streamDiag.Record(event)

		isTerminalEvent := isOpenAICompatResponsesTerminalEvent(event.Type)
		if isTerminalEvent {
			if event.Response != nil {
				terminalResponse = event.Response
				if id := strings.TrimSpace(event.Response.ID); id != "" {
					responseID = id
				}
				if event.Response.Usage != nil {
					if bridgeMode {
						accountID := int64(0)
						if account != nil {
							accountID = account.ID
						}
						rawCachedTokens := 0
						if event.Response.Usage.InputTokensDetails != nil {
							rawCachedTokens = event.Response.Usage.InputTokensDetails.CachedTokens
						}
						logClaudeGPTBridgeRawUsage("stream_terminal", requestID, accountID, originalModel, billingModel, upstreamModel, event.Response.Usage.InputTokens, event.Response.Usage.OutputTokens, rawCachedTokens, true)
						s.applyClaudeGPTBridgeDisplayCacheOverride(c.Request.Context(), event.Response.Usage, requestID, accountID, originalModel, billingModel, upstreamModel, true)
					}
					usage = copyOpenAIUsageFromResponsesUsage(event.Response.Usage)
				}
			}
			if event.Usage != nil {
				usage = copyOpenAIUsageFromResponsesUsage(event.Usage)
			}
		}

		// Convert to Anthropic events
		events := apicompat.ResponsesEventToAnthropicEvents(&event, state)
		isFailedEvent := strings.TrimSpace(event.Type) == "response.failed"
		wroteClientSSE := false
		if !clientDisconnected {
			for _, evt := range events {
				if isFailedEvent && (evt.Type == "message_delta" || evt.Type == "message_stop") {
					continue
				}
				if bridgeMode && evt.Usage != nil {
					accountID := int64(0)
					if account != nil {
						accountID = account.ID
					}
					logClaudeGPTBridgeConvertedUsage("stream_event", requestID, accountID, originalModel, billingModel, upstreamModel, *evt.Usage, true)
				}
				sse, err := apicompat.ResponsesAnthropicEventToSSE(evt)
				if err != nil {
					logger.L().Warn("openai messages stream: failed to marshal event",
						zap.Error(err),
						zap.String("request_id", requestID),
					)
					continue
				}
				if mult := getDisplayTokenMultipliers(c); mult != nil {
					sse = rewriteAnthropicSSEUsageTokens(sse, mult)
				}
				if !clientVisibleOutputStarted {
					pendingClientSSE = append(pendingClientSSE, sse)
					if anthropicStreamEventHasVisibleOutput(evt) {
						clientVisibleOutputStarted = true
						flushPendingClientSSE()
						if clientDisconnected {
							break
						}
					}
					continue
				}
				writeStreamHeaders()
				if _, err := fmt.Fprint(c.Writer, sse); err != nil {
					clientDisconnected = true
					logger.L().Info("openai messages stream: client disconnected, continuing to drain upstream for billing",
						zap.String("request_id", requestID),
					)
					break
				}
				clientOutputStarted = true
				wroteClientSSE = true
			}
		}
		if isFailedEvent {
			payloadBytes := []byte(payload)
			failureErr := s.openAIMessagesTerminalFailureError(c, account, requestID, event.Response, payloadBytes)
			if clientVisibleOutputStarted && !clientDisconnected {
				writeStreamHeaders()
				errType := strings.TrimSpace(gjson.GetBytes(failureErr.ResponseBody, "error.type").String())
				errMessage := strings.TrimSpace(gjson.GetBytes(failureErr.ResponseBody, "error.message").String())
				if errType == "" {
					errType = "api_error"
				}
				if errMessage == "" {
					errMessage = "Upstream response failed after partial output"
				}
				if _, err := fmt.Fprint(c.Writer, buildAnthropicStreamErrorSSE(errType, errMessage)); err == nil {
					clientOutputStarted = true
					c.Writer.Flush()
					resetKeepaliveTimer()
					MarkOpenAIAnthropicResponseTerminated(c)
				}
				terminalStreamErr = fmt.Errorf("upstream response failed after partial output: %s", errMessage)
			} else {
				terminalFailureErr = failureErr
			}
		}
		if wroteClientSSE && !clientDisconnected {
			c.Writer.Flush()
			resetKeepaliveTimer()
		}
		return isTerminalEvent
	}

	// finalizeStream sends any remaining Anthropic events and returns the result.
	finalizeStream := func() (*OpenAIForwardResult, error) {
		if terminalFailureErr != nil {
			return resultWithUsage(), terminalFailureErr
		}
		if terminalStreamErr != nil {
			return resultWithUsage(), terminalStreamErr
		}
		if finalEvents := apicompat.FinalizeResponsesAnthropicStream(state); len(finalEvents) > 0 && !clientDisconnected {
			wroteClientSSE := false
			for _, evt := range finalEvents {
				sse, err := apicompat.ResponsesAnthropicEventToSSE(evt)
				if err != nil {
					continue
				}
				if mult := getDisplayTokenMultipliers(c); mult != nil {
					sse = rewriteAnthropicSSEUsageTokens(sse, mult)
				}
				if !clientVisibleOutputStarted {
					pendingClientSSE = append(pendingClientSSE, sse)
					if anthropicStreamEventHasVisibleOutput(evt) {
						clientVisibleOutputStarted = true
						flushPendingClientSSE()
						if clientDisconnected {
							break
						}
					}
					continue
				}
				writeStreamHeaders()
				if _, err := fmt.Fprint(c.Writer, sse); err != nil {
					clientDisconnected = true
					logger.L().Info("openai messages stream: client disconnected during final flush",
						zap.String("request_id", requestID),
					)
					break
				}
				clientOutputStarted = true
				wroteClientSSE = true
			}
			if wroteClientSSE && !clientDisconnected {
				c.Writer.Flush()
				resetKeepaliveTimer()
			}
		}
		if !clientVisibleOutputStarted {
			result := resultWithUsage()
			fields := []zap.Field{
				zap.String("request_id", requestID),
				zap.String("response_id", responseID),
				zap.String("model", originalModel),
				zap.String("upstream_model", upstreamModel),
			}
			if account != nil {
				fields = append(fields, zap.Int64("account_id", account.ID))
			}
			fields = append(fields, streamDiag.ZapFields()...)
			logger.L().Warn("openai_messages.stream_completed_without_visible_output", fields...)
			if terminalResponse != nil && terminalResponse.IncompleteDetails != nil &&
				strings.TrimSpace(terminalResponse.Status) == "incomplete" &&
				strings.TrimSpace(terminalResponse.IncompleteDetails.Reason) == "max_output_tokens" {
				return result, s.newOpenAIStreamClientError(c, account, requestID, http.StatusBadRequest,
					"invalid_request_error",
					"OpenAI response reached max_output_tokens before producing assistant content; reduce the conversation context or output budget and try again.")
			}
			return result, s.newOpenAIStreamFailoverError(c, account, false, requestID, nil,
				"OpenAI messages stream completed without assistant content or tool output")
		}
		flushPendingClientSSE()
		MarkOpenAIAnthropicResponseTerminated(c)
		return resultWithUsage(), nil
	}

	// handleScanErr logs scanner errors if meaningful.
	handleScanErr := func(err error) {
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("openai messages stream: read error",
				zap.Error(err),
				zap.String("request_id", requestID),
			)
		}
	}
	missingTerminalErr := func() (*OpenAIForwardResult, error) {
		result := resultWithUsage()
		if clientDisconnected {
			return result, fmt.Errorf("stream usage incomplete: missing terminal event")
		}
		message := "OpenAI messages stream ended before a terminal event"
		if !clientVisibleOutputStarted {
			return result, s.newOpenAIStreamFailoverError(c, account, false, requestID, nil, message)
		}
		s.recordOpenAIMessagesStreamUpstreamError(c, account, requestID, "stream_missing_terminal", message)
		return result, fmt.Errorf("stream usage incomplete: missing terminal event")
	}
	processFrame := func(frame openAICompatSSEFrame) bool {
		payload := openAICompatPayloadWithEventType(frame.Data, frame.EventType)
		return processDataLine(payload)
	}

	// ── Determine keepalive interval ──
	keepaliveInterval := time.Duration(0)
	if s.cfg != nil && s.cfg.Gateway.StreamKeepaliveInterval > 0 {
		keepaliveInterval = time.Duration(s.cfg.Gateway.StreamKeepaliveInterval) * time.Second
	}

	// ── No keepalive: fast synchronous path (no goroutine overhead) ──
	if streamInterval <= 0 && keepaliveInterval <= 0 {
		var parser openAICompatSSEFrameParser
		for scanner.Scan() {
			line := scanner.Text()
			if isOpenAICompatDoneSentinelLine(line) {
				return missingTerminalErr()
			}
			frame, ok := parser.AddLine(line)
			if !ok {
				continue
			}
			if processFrame(frame) {
				return finalizeStream()
			}
		}
		if err := scanner.Err(); err != nil {
			handleScanErr(err)
			return resultWithUsage(), fmt.Errorf("stream usage incomplete: %w", err)
		}
		if frame, ok := parser.Finish(); ok {
			if strings.TrimSpace(frame.Data) == "[DONE]" {
				return missingTerminalErr()
			}
			if processFrame(frame) {
				return finalizeStream()
			}
		}
		return missingTerminalErr()
	}

	// ── With keepalive: goroutine + channel + select ──
	type scanEvent struct {
		line string
		err  error
	}
	events := make(chan scanEvent, 16)
	done := make(chan struct{})
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
	defer close(done)

	var keepaliveTimer *time.Timer
	if keepaliveInterval > 0 {
		keepaliveTimer = time.NewTimer(keepaliveInterval)
		defer keepaliveTimer.Stop()
	}
	var keepaliveCh <-chan time.Time
	if keepaliveTimer != nil {
		keepaliveCh = keepaliveTimer.C
	}
	resetKeepaliveTimer = func() {
		if keepaliveTimer == nil {
			return
		}
		if !keepaliveTimer.Stop() {
			select {
			case <-keepaliveTimer.C:
			default:
			}
		}
		keepaliveTimer.Reset(keepaliveInterval)
	}
	var parser openAICompatSSEFrameParser

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				// Upstream closed
				if frame, ok := parser.Finish(); ok {
					if strings.TrimSpace(frame.Data) == "[DONE]" {
						return missingTerminalErr()
					}
					if processFrame(frame) {
						return finalizeStream()
					}
				}
				return missingTerminalErr()
			}
			if ev.err != nil {
				handleScanErr(ev.err)
				return resultWithUsage(), fmt.Errorf("stream usage incomplete: %w", ev.err)
			}
			line := ev.line
			if isOpenAICompatDoneSentinelLine(line) {
				return missingTerminalErr()
			}
			frame, ok := parser.AddLine(line)
			if !ok {
				continue
			}
			if processFrame(frame) {
				return finalizeStream()
			}

		case <-intervalCh:
			lastRead := time.Unix(0, atomic.LoadInt64(&lastReadAt))
			if time.Since(lastRead) < streamInterval {
				continue
			}
			if clientDisconnected {
				return resultWithUsage(), fmt.Errorf("stream usage incomplete after timeout")
			}
			logger.L().Warn("openai messages stream: data interval timeout",
				zap.String("request_id", requestID),
				zap.String("model", originalModel),
				zap.Duration("interval", streamInterval),
			)
			return resultWithUsage(), fmt.Errorf("stream data interval timeout")

		case <-keepaliveCh:
			if clientDisconnected {
				continue
			}
			// Send Anthropic-format ping event
			writeStreamHeaders()
			if _, err := fmt.Fprint(c.Writer, "event: ping\ndata: {\"type\":\"ping\"}\n\n"); err != nil {
				// Client disconnected
				logger.L().Info("openai messages stream: client disconnected during keepalive",
					zap.String("request_id", requestID),
				)
				clientDisconnected = true
				continue
			}
			MarkOpenAIAnthropicTransportStreamStarted(c)
			c.Writer.Flush()
			resetKeepaliveTimer()
		}
	}
}

func anthropicStreamEventHasVisibleOutput(evt apicompat.AnthropicStreamEvent) bool {
	switch strings.TrimSpace(evt.Type) {
	case "content_block_start":
		if evt.ContentBlock == nil {
			return false
		}
		switch strings.TrimSpace(evt.ContentBlock.Type) {
		case "tool_use", "server_tool_use":
			return true
		}
	case "content_block_delta":
		if evt.Delta == nil {
			return false
		}
		return strings.TrimSpace(evt.Delta.Text) != "" ||
			strings.TrimSpace(evt.Delta.PartialJSON) != ""
	}
	return false
}

type openAIMessagesStreamDiagnostic struct {
	EventTypes                  map[string]int
	OutputItemTypes             map[string]int
	FinalOutputTypes            map[string]int
	OutputTextDeltaBytes        int
	ReasoningDeltaBytes         int
	FunctionArgumentsDeltaBytes int
	TerminalType                string
	TerminalStatus              string
	TerminalIncompleteReason    string
	TerminalErrorCode           string
	TerminalOutputCount         int
	TerminalInputTokens         int
	TerminalOutputTokens        int
	TerminalMessageTextBytes    int
	TerminalReasoningTextBytes  int
}

func newOpenAIMessagesStreamDiagnostic() *openAIMessagesStreamDiagnostic {
	return &openAIMessagesStreamDiagnostic{
		EventTypes:       make(map[string]int),
		OutputItemTypes:  make(map[string]int),
		FinalOutputTypes: make(map[string]int),
	}
}

func (d *openAIMessagesStreamDiagnostic) Record(evt apicompat.ResponsesStreamEvent) {
	if d == nil {
		return
	}
	eventType := strings.TrimSpace(evt.Type)
	if eventType == "" {
		eventType = "<empty>"
	}
	d.EventTypes[eventType]++
	if evt.Item != nil {
		itemType := strings.TrimSpace(evt.Item.Type)
		if itemType == "" {
			itemType = "<empty>"
		}
		d.OutputItemTypes[itemType]++
	}
	switch eventType {
	case "response.output_text.delta":
		d.OutputTextDeltaBytes += len(evt.Delta)
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		d.ReasoningDeltaBytes += len(evt.Delta)
	case "response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
		d.FunctionArgumentsDeltaBytes += len(evt.Delta)
	}
	if evt.Response == nil {
		return
	}
	d.TerminalType = eventType
	d.TerminalStatus = strings.TrimSpace(evt.Response.Status)
	d.TerminalOutputCount = len(evt.Response.Output)
	for _, output := range evt.Response.Output {
		outputType := strings.TrimSpace(output.Type)
		if outputType == "" {
			outputType = "<empty>"
		}
		d.FinalOutputTypes[outputType]++
		switch outputType {
		case "message":
			for _, part := range output.Content {
				if strings.TrimSpace(part.Type) == "output_text" {
					d.TerminalMessageTextBytes += len(part.Text)
				}
			}
		case "reasoning":
			for _, summary := range output.Summary {
				d.TerminalReasoningTextBytes += len(summary.Text)
			}
		}
	}
	if evt.Response.IncompleteDetails != nil {
		d.TerminalIncompleteReason = strings.TrimSpace(evt.Response.IncompleteDetails.Reason)
	}
	if evt.Response.Error != nil {
		d.TerminalErrorCode = strings.TrimSpace(evt.Response.Error.Code)
	}
	if evt.Response.Usage != nil {
		d.TerminalInputTokens = evt.Response.Usage.InputTokens
		d.TerminalOutputTokens = evt.Response.Usage.OutputTokens
	}
}

func (d *openAIMessagesStreamDiagnostic) ZapFields() []zap.Field {
	if d == nil {
		return nil
	}
	return []zap.Field{
		zap.Any("responses_event_types", d.EventTypes),
		zap.Any("responses_output_item_types", d.OutputItemTypes),
		zap.Any("responses_final_output_types", d.FinalOutputTypes),
		zap.Int("output_text_delta_bytes", d.OutputTextDeltaBytes),
		zap.Int("reasoning_delta_bytes", d.ReasoningDeltaBytes),
		zap.Int("function_arguments_delta_bytes", d.FunctionArgumentsDeltaBytes),
		zap.String("terminal_event_type", d.TerminalType),
		zap.String("terminal_status", d.TerminalStatus),
		zap.String("terminal_incomplete_reason", d.TerminalIncompleteReason),
		zap.String("terminal_error_code", d.TerminalErrorCode),
		zap.Int("terminal_output_count", d.TerminalOutputCount),
		zap.Int("terminal_input_tokens", d.TerminalInputTokens),
		zap.Int("terminal_output_tokens", d.TerminalOutputTokens),
		zap.Int("terminal_message_text_bytes", d.TerminalMessageTextBytes),
		zap.Int("terminal_reasoning_text_bytes", d.TerminalReasoningTextBytes),
	}
}

// writeAnthropicError writes an error response in Anthropic Messages API format.
func writeAnthropicError(c *gin.Context, statusCode int, errType, message string) {
	if OpenAIAnthropicResponseTerminated(c) {
		return
	}
	if OpenAIAnthropicTransportStreamStarted(c) {
		if _, err := fmt.Fprint(c.Writer, buildAnthropicStreamErrorSSE(errType, message)); err == nil {
			c.Writer.Flush()
			MarkOpenAIAnthropicResponseTerminated(c)
		}
		return
	}
	c.JSON(statusCode, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
	MarkOpenAIAnthropicResponseTerminated(c)
}

func buildAnthropicStreamErrorSSE(errType, message string) string {
	payload, err := json.Marshal(gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
	if err != nil {
		return "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":\"upstream error\"}}\n\n"
	}
	return "event: error\ndata: " + string(payload) + "\n\n"
}

func copyOpenAIUsageFromResponsesUsage(usage *apicompat.ResponsesUsage) OpenAIUsage {
	if usage == nil {
		return OpenAIUsage{}
	}
	result := OpenAIUsage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}
	if usage.InputTokensDetails != nil {
		result.CacheReadInputTokens = usage.InputTokensDetails.CachedTokens
		result.CacheCreationInputTokens = usage.InputTokensDetails.CacheWriteTokens
	}
	return result
}
