package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

// openAIChatPassthroughZeroUsagePaths are Chat Completions usage fields the
// display rewriter can change. Passthrough puts an upstream 0 back after that
// rewrite so cache residual cannot turn prompt_tokens or completion_tokens
// from 0 into a positive count.
var openAIChatPassthroughZeroUsagePaths = []string{
	"usage.prompt_tokens",
	"usage.completion_tokens",
	"usage.total_tokens",
	"usage.prompt_tokens_details.cached_tokens",
	"usage.cache_creation_input_tokens",
	"usage.cache_write_tokens",
	"usage.prompt_tokens_details.cache_write_tokens",
	"usage.input_tokens_details.cache_write_tokens",
}

// forwardRawChatCompletionsPassthrough posts the inbound Chat Completions body
// to the account's /v1/chat/completions URL. It replaces Authorization, keeps
// the Chat Completions header allowlist, and returns the upstream status and
// body without model rewrite, Kimi validation, usage injection, silent-refusal
// holdback, or truncated-stream failover. Client-facing usage still uses the
// raw Chat Completions display-token rewrite. Billing keeps the usage captured
// before that rewrite, and an upstream usage field that is 0 stays 0.
// Failover is only HTTP 429 and 529. Pool-mode same-account retry stays on 429.
func (s *OpenAIGatewayService) forwardRawChatCompletionsPassthrough(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
	clientStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	reasoningEffort := extractOpenAIReasoningEffortFromBody(body, originalModel)
	upstreamBody, policyErr := s.applyOpenAIFastPolicyToBody(ctx, account, originalModel, body)
	if policyErr != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(policyErr, &blocked) {
			writeChatCompletionsError(c, http.StatusForbidden, "permission_error", blocked.Message)
		}
		return nil, policyErr
	}
	serviceTier := extractOpenAIServiceTierFromBody(upstreamBody)

	token, tokenKind, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("account %d missing %s credential", account.ID, tokenKind)
	}

	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	targetURL := buildOpenAIChatCompletionsURL(validatedURL)

	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	if clientStream {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			lowerKey := strings.ToLower(key)
			if !openaiCCRawAllowedHeaders[lowerKey] {
				continue
			}
			for _, v := range values {
				upstreamReq.Header.Add(key, v)
			}
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		upstreamReq.Header.Set("user-agent", customUA)
	}
	account.ApplyHeaderOverrides(upstreamReq.Header)

	if c != nil {
		c.Set("openai_passthrough", true)
	}
	SetActualOpenAIUpstreamEndpoint(c, grokChatRawEndpoint)
	SetOpsUpstreamModel(c, originalModel)
	setOpsUpstreamRequestBody(c, upstreamBody)

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	var stageClk *openAIStreamStageClock
	if clientStream {
		stageClk = s.beginOpenAIStreamStageTiming(c, account, originalModel, "chat_passthrough", startTime)
	}
	stageClk.MarkDoStart()
	resp, err := s.doOpenAIUpstreamWithHeaderWait(ctx, c, account, upstreamReq, proxyURL, true, originalModel)
	if err != nil {
		stageClk.Complete(c)
		return nil, err
	}
	stageClk.MarkHeaders(resp.Header.Get("x-request-id"))
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		stageClk.Complete(c)
		if shouldFailoverOpenAIPassthroughResponse(resp.StatusCode) {
			failoverErr := s.handleFailoverErrorResponsePassthrough(ctx, resp, c, account, upstreamBody)
			var upstreamFailover *UpstreamFailoverError
			if errors.As(failoverErr, &upstreamFailover) && account.IsPoolMode() && isPoolModeRetryableStatus(resp.StatusCode) {
				upstreamFailover.RetryableOnSameAccount = true
			}
			return nil, failoverErr
		}
		return nil, s.handleErrorResponsePassthrough(ctx, resp, c, account, upstreamBody)
	}

	var result *OpenAIForwardResult
	if clientStream || !isOpenAIJSONResponse(resp) {
		result, err = s.streamRawChatCompletionsPassthrough(c, resp, originalModel, reasoningEffort, serviceTier, startTime)
	} else {
		result, err = s.bufferRawChatCompletionsPassthrough(c, resp, originalModel, reasoningEffort, serviceTier, startTime)
	}
	stageClk.Complete(c)
	if result != nil {
		result.Stream = clientStream
		result.UpstreamEndpoint = grokChatRawEndpoint
	}
	return result, err
}

// streamRawChatCompletionsPassthrough copies upstream SSE bytes to the client.
// A clean end with no finish_reason, usage, or [DONE] stays a successful write.
func (s *OpenAIGatewayService) streamRawChatCompletionsPassthrough(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := ""
	if resp != nil {
		requestID = resp.Header.Get("x-request-id")
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	if strings.TrimSpace(c.Writer.Header().Get("Content-Type")) == "" {
		if ct := strings.TrimSpace(resp.Header.Get("Content-Type")); ct != "" {
			c.Writer.Header().Set("Content-Type", ct)
		} else {
			c.Writer.Header().Set("Content-Type", "text/event-stream")
		}
	}
	// Line framing drops the upstream Content-Length. Chunked encoding keeps
	// the body available as each line arrives.
	c.Writer.Header().Del("Content-Length")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(resp.StatusCode)

	scanner := bufio.NewScanner(resp.Body)
	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	var usage OpenAIUsage
	var firstTokenMs *int
	clientDisconnected := false
	stageClk := getOpenAIStreamStageClock(c)
	displayMult := getDisplayTokenMultipliers(c)
	for scanner.Scan() {
		line := scanner.Text()
		if payload, ok := extractOpenAISSEDataLine(line); ok && strings.TrimSpace(payload) != "[DONE]" {
			stageClk.MarkFirstSSE()
			observeOpenAISSELine(c, line)
			if parsed := extractCCStreamUsage(payload); parsed != nil {
				usage = *parsed
			}
			if firstTokenMs == nil && !isOpenAIChatUsageOnlyStreamChunk(payload) {
				elapsed := int(time.Since(startTime).Milliseconds())
				firstTokenMs = &elapsed
				stageClk.MarkFirstUsefulUpstream()
			}
		}
		if clientDisconnected {
			continue
		}
		outLine := rewriteOpenAIChatPassthroughSSELine(line, displayMult)
		if _, werr := io.WriteString(c.Writer, outLine+"\n"); werr != nil {
			clientDisconnected = true
			continue
		}
		c.Writer.Flush()
		if firstTokenMs != nil {
			stageClk.MarkFirstClientFlush()
		}
	}
	if scanErr := scanner.Err(); scanErr != nil && !errors.Is(scanErr, context.Canceled) && !errors.Is(scanErr, context.DeadlineExceeded) {
		logger.L().Warn("openai chat_completions passthrough: stream read error",
			zap.Error(scanErr),
			zap.String("request_id", requestID),
			zap.String("model", originalModel),
		)
	}

	return &OpenAIForwardResult{
		RequestID:             requestID,
		Usage:                 usage,
		Model:                 originalModel,
		BillingModel:          originalModel,
		UpstreamModel:         originalModel,
		UpstreamResponseModel: observedUpstreamResponseModel(c),
		ReasoningEffort:       reasoningEffort,
		ServiceTier:           serviceTier,
		Stream:                true,
		Duration:              time.Since(startTime),
		FirstTokenMs:          firstTokenMs,
	}, nil
}

func (s *OpenAIGatewayService) bufferRawChatCompletionsPassthrough(
	c *gin.Context,
	resp *http.Response,
	originalModel string,
	reasoningEffort *string,
	serviceTier *string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := ""
	if resp != nil {
		requestID = resp.Header.Get("x-request-id")
	}
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeChatCompletionsError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, fmt.Errorf("read upstream body: %w", err)
	}
	observeOpenAIResponseBody(c, respBody)
	usage := openAIChatPassthroughUsage(respBody)
	outbound := rewriteOpenAIChatPassthroughDisplayUsage(respBody, getDisplayTokenMultipliers(c))

	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	if !bytes.Equal(outbound, respBody) {
		c.Writer.Header().Del("Content-Length")
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, outbound)

	return &OpenAIForwardResult{
		RequestID:             requestID,
		Usage:                 usage,
		Model:                 originalModel,
		BillingModel:          originalModel,
		UpstreamModel:         originalModel,
		UpstreamResponseModel: observedUpstreamResponseModel(c),
		ReasoningEffort:       reasoningEffort,
		ServiceTier:           serviceTier,
		Stream:                false,
		Duration:              time.Since(startTime),
	}, nil
}

func openAIChatPassthroughUsage(body []byte) OpenAIUsage {
	if parsed := extractCCStreamUsage(string(body)); parsed != nil {
		return *parsed
	}
	var usage OpenAIUsage
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), defaultMaxLineSize)
	for scanner.Scan() {
		payload, ok := extractOpenAISSEDataLine(scanner.Text())
		if !ok {
			continue
		}
		if parsed := extractCCStreamUsage(payload); parsed != nil {
			usage = *parsed
		}
	}
	return usage
}

func rewriteOpenAIChatPassthroughSSELine(line string, mult *DisplayTokenMultipliers) string {
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
	return "data: " + string(rewriteOpenAIChatPassthroughDisplayUsage([]byte(payload), mult))
}

func rewriteOpenAIChatPassthroughDisplayUsage(body []byte, mult *DisplayTokenMultipliers) []byte {
	if len(body) == 0 || mult == nil || !mult.IsNonTrivial() || !gjson.GetBytes(body, "usage").Exists() {
		return body
	}
	return preserveOpenAIChatZeroUsageFields(body, rewriteOpenAIChatUsageTokens(body, "usage", mult))
}

func preserveOpenAIChatZeroUsageFields(original, rewritten []byte) []byte {
	restoredPromptOrCompletion := false
	for _, path := range openAIChatPassthroughZeroUsagePaths {
		originalNode := gjson.GetBytes(original, path)
		if !openAIJSONNumberIsZero(originalNode) {
			continue
		}
		if openAIJSONNumberIsZero(gjson.GetBytes(rewritten, path)) {
			continue
		}
		updated, err := sjson.SetBytes(rewritten, path, 0)
		if err != nil {
			return rewritten
		}
		rewritten = updated
		if path == "usage.prompt_tokens" || path == "usage.completion_tokens" {
			restoredPromptOrCompletion = true
		}
	}
	total := gjson.GetBytes(original, "usage.total_tokens")
	if !restoredPromptOrCompletion || total.Type != gjson.Number || total.Num == 0 {
		return rewritten
	}
	prompt := int(gjson.GetBytes(rewritten, "usage.prompt_tokens").Int())
	completion := int(gjson.GetBytes(rewritten, "usage.completion_tokens").Int())
	updated, err := sjson.SetBytes(rewritten, "usage.total_tokens", prompt+completion)
	if err != nil {
		return rewritten
	}
	return updated
}

func openAIJSONNumberIsZero(node gjson.Result) bool {
	return node.Exists() && node.Type == gjson.Number && node.Num == 0
}
