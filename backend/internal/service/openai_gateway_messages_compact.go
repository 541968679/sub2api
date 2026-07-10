package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

var (
	openAIAnthropicCompactChunkTargetChars = 300_000
	openAIAnthropicCompactMergeTargetChars = 180_000
)

const (
	openAIAnthropicCompactFallbackMaxChunks     = 40
	openAIAnthropicCompactChunkMaxOutputTokens  = 6_000
	openAIAnthropicCompactMergeMaxOutputTokens  = 12_000
	openAIAnthropicCompactEmergencyMaxRunes     = 90_000
	openAIAnthropicCompactFallbackMinSplitRunes = 4_000
	openAIAnthropicCompactChunkMaxSplitDepth    = 8
	openAIAnthropicCompactChunkSplitBudget      = 40
	openAIAnthropicCompactMergeMaxDepth         = 6
	openAIAnthropicCompactMergeAttemptBudget    = 32
	openAIAnthropicCompactFallbackReasoning     = "low"
	openAIAnthropicCompactFallbackClientMessage = "Claude Code compact recovery failed before producing a usable summary"
)

var errOpenAICompactContextLengthExceeded = errors.New("OpenAI compact context_length_exceeded")

type openAICompactContextLengthError struct {
	statusCode int
	message    string
}

type openAIAnthropicCompactHTTPResult struct {
	response *http.Response
	err      error
}

type openAIAnthropicCompactCancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	stop   func() bool
	once   sync.Once
}

func (b *openAIAnthropicCompactCancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(func() {
		if b.stop != nil {
			b.stop()
		}
		if b.cancel != nil {
			b.cancel()
		}
	})
	return err
}

func (s *OpenAIGatewayService) doOpenAIAnthropicRequestWithCompactKeepalive(
	c *gin.Context,
	req *http.Request,
	proxyURL string,
	accountID int64,
	accountConcurrency int,
) (*http.Response, error) {
	interval := openAIAnthropicCompactKeepaliveInterval(c)
	if req == nil {
		return nil, errors.New("anthropic compact upstream request is nil")
	}
	requestCtx, cancel := context.WithCancel(req.Context())
	req = req.Clone(requestCtx)
	var stopClientCancel func() bool
	if c != nil && c.Request != nil {
		stopClientCancel = context.AfterFunc(c.Request.Context(), cancel)
	}
	releaseRequest := func() {
		if stopClientCancel != nil {
			stopClientCancel()
		}
		cancel()
	}

	resultCh := make(chan openAIAnthropicCompactHTTPResult)
	abandoned := make(chan struct{})
	defer close(abandoned)
	go func() {
		resp, err := s.httpUpstream.Do(req, proxyURL, accountID, accountConcurrency)
		select {
		case resultCh <- openAIAnthropicCompactHTTPResult{response: resp, err: err}:
		case <-abandoned:
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
		}
	}()

	var ticker *time.Ticker
	if interval > 0 {
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
	}
	var tickerCh <-chan time.Time
	if ticker != nil {
		tickerCh = ticker.C
	}
	var requestDone <-chan struct{}
	if c != nil && c.Request != nil {
		requestDone = c.Request.Context().Done()
	}
	for {
		select {
		case result := <-resultCh:
			if result.err != nil || result.response == nil || result.response.Body == nil {
				releaseRequest()
			} else {
				result.response.Body = &openAIAnthropicCompactCancelBody{
					ReadCloser: result.response.Body,
					cancel:     cancel,
					stop:       stopClientCancel,
				}
			}
			return result.response, result.err
		case <-tickerCh:
			if err := writeAnthropicCompactKeepalive(c); err != nil {
				releaseRequest()
				return nil, fmt.Errorf("write compact keepalive: %w", err)
			}
		case <-requestDone:
			releaseRequest()
			return nil, c.Request.Context().Err()
		}
	}
}

func writeAnthropicCompactKeepalive(c *gin.Context) error {
	if c == nil || c.Writer == nil {
		return errors.New("anthropic compact keepalive writer is nil")
	}
	if !c.Writer.Written() {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.WriteHeader(http.StatusOK)
	}
	written, err := fmt.Fprint(c.Writer, "event: ping\ndata: {\"type\":\"ping\"}\n\n")
	if written > 0 {
		MarkOpenAIAnthropicTransportStreamStarted(c)
		c.Writer.Flush()
	}
	return err
}

func (e *openAICompactContextLengthError) Error() string {
	if e == nil {
		return errOpenAICompactContextLengthExceeded.Error()
	}
	message := strings.TrimSpace(e.message)
	if message == "" {
		message = "upstream compact request exceeded the context window"
	}
	if e.statusCode > 0 {
		return fmt.Sprintf("%s (status %d): %s", errOpenAICompactContextLengthExceeded, e.statusCode, message)
	}
	return fmt.Sprintf("%s: %s", errOpenAICompactContextLengthExceeded, message)
}

func (e *openAICompactContextLengthError) Unwrap() error {
	return errOpenAICompactContextLengthExceeded
}

func isClaudeCodeCompactAnthropicRequest(req *apicompat.AnthropicRequest) bool {
	if req == nil {
		return false
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(req.Messages[i].Role) != "user" {
			continue
		}
		return looksLikeClaudeCodeCompactPrompt(anthropicMessageText(req.Messages[i].Content))
	}
	return false
}

func anthropicMessageText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var blocks []apicompat.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(block.Type) == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func looksLikeClaudeCodeCompactPrompt(text string) bool {
	text = strings.TrimSpace(text)
	if !strings.Contains(text, "Your task is to create a detailed summary") {
		return false
	}
	for _, marker := range []string{
		"<analysis>",
		"<summary>",
		"All user messages",
		"Pending Tasks",
		"Current Work",
	} {
		if !strings.Contains(text, marker) {
			return false
		}
	}
	return true
}

func (s *OpenAIGatewayService) handleAnthropicCompactStreamingResponse(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	resp *http.Response,
	bridgeMode bool,
	originalModel string,
	billingModel string,
	upstreamModel string,
	compactFallbackUpstreamModels []string,
	startTime time.Time,
	fullAnthropicReq *apicompat.AnthropicRequest,
	token string,
	clientStream bool,
) (*OpenAIForwardResult, error) {
	requestID := strings.TrimSpace(resp.Header.Get("x-request-id"))
	finalResponse, usage, acc, err := s.readOpenAICompatBufferedTerminalWithKeepalive(
		resp, "openai messages compact buffered", requestID, c, nil,
	)
	if err != nil {
		return nil, s.newOpenAIStreamFailoverError(c, account, false, requestID, nil,
			"OpenAI compact stream could not be read to completion")
	}
	if finalResponse == nil {
		return nil, s.newOpenAIStreamFailoverError(c, account, false, requestID, nil,
			"OpenAI compact stream ended without a terminal response")
	}
	acc.SupplementResponseOutput(finalResponse)

	if isOpenAIResponsesCompactModelUnavailable(finalResponse) && len(compactFallbackUpstreamModels) > 0 {
		logger.L().Warn("openai_messages.compact_terminal_model_unavailable_fallback_started",
			zap.String("request_id", requestID),
			zap.Int64("account_id", account.ID),
			zap.String("model", originalModel),
			zap.String("upstream_model", upstreamModel),
			zap.Strings("fallback_upstream_models", compactFallbackUpstreamModels),
		)
		return s.runAnthropicCompactRecoveryWithModelFallbacks(
			ctx, c, account, fullAnthropicReq, token, bridgeMode, originalModel,
			compactFallbackUpstreamModels, startTime, usage, clientStream, requestID,
		)
	}

	if compactResponseNeedsRecovery(finalResponse) {
		logger.L().Warn("openai_messages.compact_recovery_started",
			zap.String("request_id", requestID),
			zap.Int64("account_id", account.ID),
			zap.String("model", originalModel),
			zap.String("upstream_model", upstreamModel),
			zap.String("terminal_status", strings.TrimSpace(finalResponse.Status)),
			zap.String("terminal_error_code", compactResponseErrorCode(finalResponse)),
			zap.String("terminal_incomplete_reason", compactResponseIncompleteReason(finalResponse)),
			zap.Int("terminal_output_count", len(finalResponse.Output)),
			zap.Int("initial_input_tokens", usage.InputTokens),
			zap.Int("initial_output_tokens", usage.OutputTokens),
			zap.Int("full_message_count", len(fullAnthropicReq.Messages)),
		)
		candidates := append([]string{upstreamModel}, compactFallbackUpstreamModels...)
		return s.runAnthropicCompactRecoveryWithModelFallbacks(
			ctx, c, account, fullAnthropicReq, token, bridgeMode, originalModel,
			candidates, startTime, usage, clientStream, requestID,
		)
	}

	if strings.TrimSpace(finalResponse.Status) == "failed" {
		payload, _ := json.Marshal(gin.H{"type": "response.failed", "response": finalResponse})
		return nil, s.openAIMessagesTerminalFailureError(c, account, requestID, finalResponse, payload)
	}
	return s.writeAnthropicCompactFinalResponse(
		c, account, resp.Header, finalResponse, usage, bridgeMode, originalModel,
		billingModel, upstreamModel, startTime, clientStream, requestID,
	)
}

func compactResponseNeedsRecovery(resp *apicompat.ResponsesResponse) bool {
	if resp == nil {
		return true
	}
	if isOpenAIResponsesContextLengthExceeded(resp) {
		return true
	}
	status := strings.TrimSpace(resp.Status)
	if status == "failed" {
		return false
	}
	return !anthropicResponseHasVisibleOutput(apicompat.ResponsesToAnthropic(resp, resp.Model))
}

func isOpenAICompactContextLengthHTTPError(statusCode int, upstreamMsg string, upstreamBody []byte) bool {
	if statusCode < http.StatusBadRequest {
		return false
	}
	code := strings.TrimSpace(gjson.GetBytes(upstreamBody, "error.code").String())
	if strings.EqualFold(code, "context_length_exceeded") {
		return true
	}
	lower := strings.ToLower(strings.Join([]string{
		upstreamMsg,
		gjson.GetBytes(upstreamBody, "error.message").String(),
		string(upstreamBody),
	}, " "))
	return (strings.Contains(lower, "context window") || strings.Contains(lower, "context length")) &&
		(strings.Contains(lower, "exceed") || strings.Contains(lower, "too long") || strings.Contains(lower, "maximum"))
}

func isOpenAIResponsesCompactModelUnavailable(resp *apicompat.ResponsesResponse) bool {
	if resp == nil || resp.Error == nil || isOpenAIResponsesContextLengthExceeded(resp) {
		return false
	}
	return isOpenAICompactUnavailableText(resp.Error.Code + " " + resp.Error.Message)
}

func isOpenAICompactModelUnavailableHTTP(statusCode int, upstreamMsg string, upstreamBody []byte) bool {
	if isOpenAICompactContextLengthHTTPError(statusCode, upstreamMsg, upstreamBody) {
		return false
	}
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, 529:
		return true
	}
	if statusCode >= 500 {
		return true
	}
	return isOpenAICompactUnavailableText(upstreamMsg + " " + string(upstreamBody))
}

func isOpenAICompactModelUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	var failoverErr *UpstreamFailoverError
	if errors.As(err, &failoverErr) {
		return isOpenAICompactUnavailableText(string(failoverErr.ResponseBody))
	}
	return isOpenAICompactUnavailableText(err.Error())
}

func isOpenAICompactUnavailableText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" || strings.Contains(lower, "context_length_exceeded") ||
		(strings.Contains(lower, "context window") && strings.Contains(lower, "exceed")) {
		return false
	}
	for _, pattern := range []string{
		"rate_limit", "rate limit", "too many requests", "usage limit", "quota",
		"resource exhausted", "temporarily unavailable", "service unavailable",
		"selected model is at capacity", "server is overloaded", "slow_down",
		"unsupported model", "unknown model", "no available account", "no available accounts",
	} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return strings.Contains(lower, "unavailable") && strings.Contains(lower, "model")
}

func (s *OpenAIGatewayService) runAnthropicCompactRecoveryWithModelFallbacks(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	fullAnthropicReq *apicompat.AnthropicRequest,
	token string,
	bridgeMode bool,
	originalModel string,
	candidateUpstreamModels []string,
	startTime time.Time,
	initialUsage OpenAIUsage,
	clientStream bool,
	initialRequestID string,
) (*OpenAIForwardResult, error) {
	candidates := compactModelFallbackCandidates(candidateUpstreamModels, "")
	if len(candidates) == 0 {
		return nil, s.newOpenAIStreamFailoverError(c, account, false, initialRequestID, nil,
			"OpenAI compact recovery has no candidate model")
	}

	runningUsage := initialUsage
	var lastResult *OpenAIForwardResult
	var lastErr error
	for i, candidate := range candidates {
		candidate = normalizeOpenAIModelForUpstream(account, candidate)
		if candidate == "" {
			continue
		}
		result, err := s.runAnthropicCompactRecovery(
			ctx, c, account, fullAnthropicReq, token, bridgeMode, originalModel,
			candidate, candidate, startTime, runningUsage, clientStream, initialRequestID,
		)
		if err == nil {
			return result, nil
		}
		lastResult, lastErr = result, err
		if result != nil {
			runningUsage = result.Usage
		}
		semanticOutputStarted := openAIAnthropicSemanticOutputStarted(c) ||
			(result != nil && result.ClientOutputStarted)
		if i+1 >= len(candidates) || !isOpenAICompactModelUnavailableError(err) || semanticOutputStarted {
			s.handleAnthropicCompactRecoveryExhaustedError(ctx, account, err)
			return result, err
		}
		logger.L().Warn("openai_messages.compact_recovery_model_switch",
			zap.Int64("account_id", account.ID),
			zap.String("model", originalModel),
			zap.String("failed_upstream_model", candidate),
			zap.String("next_upstream_model", normalizeOpenAIModelForUpstream(account, candidates[i+1])),
			zap.Error(err),
		)
	}
	if lastErr == nil {
		lastErr = s.newOpenAIStreamFailoverError(c, account, false, initialRequestID, nil,
			"OpenAI compact recovery exhausted candidate models")
	}
	s.handleAnthropicCompactRecoveryExhaustedError(ctx, account, lastErr)
	return lastResult, lastErr
}

func (s *OpenAIGatewayService) handleAnthropicCompactRecoveryExhaustedError(
	ctx context.Context,
	account *Account,
	err error,
) {
	var upstreamErr *UpstreamFailoverError
	if !errors.As(err, &upstreamErr) || upstreamErr == nil {
		return
	}
	message := strings.TrimSpace(extractUpstreamErrorMessage(upstreamErr.ResponseBody))
	if !s.shouldFailoverOpenAIUpstreamResponse(upstreamErr.StatusCode, message, upstreamErr.ResponseBody) {
		return
	}
	s.handleOpenAIAccountUpstreamError(
		ctx, account, upstreamErr.StatusCode, upstreamErr.ResponseHeaders, upstreamErr.ResponseBody,
	)
}

func isOpenAIResponsesContextLengthExceeded(resp *apicompat.ResponsesResponse) bool {
	if resp == nil || resp.Error == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(resp.Error.Code), "context_length_exceeded") {
		return true
	}
	message := strings.ToLower(strings.TrimSpace(resp.Error.Message))
	return strings.Contains(message, "context window") && strings.Contains(message, "exceed")
}

func isOpenAICompactContextLengthError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errOpenAICompactContextLengthExceeded) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(lower, "context_length_exceeded") {
		return true
	}
	return (strings.Contains(lower, "context window") || strings.Contains(lower, "context length")) &&
		(strings.Contains(lower, "exceed") || strings.Contains(lower, "too large"))
}

func compactResponseErrorCode(resp *apicompat.ResponsesResponse) string {
	if resp == nil || resp.Error == nil {
		return ""
	}
	return strings.TrimSpace(resp.Error.Code)
}

func compactResponseIncompleteReason(resp *apicompat.ResponsesResponse) string {
	if resp == nil || resp.IncompleteDetails == nil {
		return ""
	}
	return strings.TrimSpace(resp.IncompleteDetails.Reason)
}

func (s *OpenAIGatewayService) runAnthropicCompactRecovery(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	fullAnthropicReq *apicompat.AnthropicRequest,
	token string,
	bridgeMode bool,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
	initialUsage OpenAIUsage,
	clientStream bool,
	initialRequestID string,
) (*OpenAIForwardResult, error) {
	compactPrompt, transcript := buildAnthropicCompactFallbackTranscript(fullAnthropicReq)
	chunks := splitAnthropicCompactTranscriptChunks(
		transcript, openAIAnthropicCompactChunkTargetChars, openAIAnthropicCompactFallbackMaxChunks,
	)
	if len(chunks) == 0 {
		return nil, s.newOpenAIStreamFailoverError(c, account, false, initialRequestID, nil,
			"OpenAI compact recovery transcript is empty")
	}

	totalUsage := initialUsage
	chunkSummaries := make([]string, 0, len(chunks))
	lastRequestID := initialRequestID
	remainingChunkSplits := openAIAnthropicCompactChunkSplitBudget
	for i, chunk := range chunks {
		logger.L().Info("openai_messages.compact_chunk_attempt",
			zap.Int64("account_id", account.ID),
			zap.String("upstream_model", upstreamModel),
			zap.Int("chunk_index", i+1),
			zap.Int("chunk_count", len(chunks)),
			zap.Int("chunk_chars", runeLen(chunk)),
		)
		leafSummaries, chunkUsage, requestID, err := s.summarizeAnthropicCompactChunk(
			ctx, c, account, token, upstreamModel, chunk,
			fmt.Sprintf("%d/%d", i+1, len(chunks)), 0, &remainingChunkSplits,
		)
		totalUsage = addOpenAIUsage(totalUsage, chunkUsage)
		lastRequestID = firstNonEmpty(requestID, lastRequestID)
		if err != nil {
			return compactRecoveryResult(lastRequestID, "", totalUsage, originalModel, billingModel, upstreamModel, clientStream, startTime),
				s.wrapAnthropicCompactRecoveryError(c, account, lastRequestID, err)
		}
		chunkSummaries = append(chunkSummaries, leafSummaries...)
	}

	summaries := make([]string, 0, len(chunkSummaries))
	for i, summary := range chunkSummaries {
		summaries = append(summaries, fmt.Sprintf("## Chunk %d/%d\n%s", i+1, len(chunkSummaries), summary))
	}

	finalResponse, mergeUsage, mergeRequestID, err := s.mergeAnthropicCompactSummaries(
		ctx, c, account, token, upstreamModel, compactPrompt, summaries,
		openAIAnthropicCompactMergeTargetChars, 0,
	)
	totalUsage = addOpenAIUsage(totalUsage, mergeUsage)
	lastRequestID = firstNonEmpty(mergeRequestID, lastRequestID)
	if err != nil {
		return compactRecoveryResult(lastRequestID, compactResponseID(finalResponse), totalUsage, originalModel, billingModel, upstreamModel, clientStream, startTime),
			s.wrapAnthropicCompactRecoveryError(c, account, lastRequestID, err)
	}
	if finalResponse == nil {
		return nil, s.newOpenAIStreamFailoverError(c, account, false, lastRequestID, nil,
			openAIAnthropicCompactFallbackClientMessage)
	}
	finalResponse.Usage = responsesUsageFromOpenAIUsage(totalUsage)
	result, writeErr := s.writeAnthropicCompactFinalResponse(
		c, account, nil, finalResponse, totalUsage, bridgeMode, originalModel,
		billingModel, upstreamModel, startTime, clientStream, lastRequestID,
	)
	if result != nil {
		result.SkipContinuationBinding = true
	}
	return result, writeErr
}

func (s *OpenAIGatewayService) summarizeAnthropicCompactChunk(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	upstreamModel string,
	chunk string,
	chunkLabel string,
	depth int,
	remainingSplits *int,
) ([]string, OpenAIUsage, string, error) {
	prompt := fmt.Sprintf("Chunk %s from a Claude Code conversation transcript:\n\n%s", chunkLabel, chunk)
	response, usage, requestID, err := s.runOpenAIAnthropicCompactRecoveryRequest(
		ctx, c, account, token, upstreamModel, openAIAnthropicCompactChunkInstructions(),
		prompt, openAIAnthropicCompactChunkMaxOutputTokens,
	)
	contextExceeded := isOpenAIResponsesContextLengthExceeded(response) || isOpenAICompactContextLengthError(err)
	if contextExceeded {
		chunkRunes := runeLen(chunk)
		if depth >= openAIAnthropicCompactChunkMaxSplitDepth ||
			chunkRunes <= openAIAnthropicCompactFallbackMinSplitRunes ||
			remainingSplits == nil || *remainingSplits <= 0 {
			return nil, usage, requestID, &openAICompactContextLengthError{
				message: fmt.Sprintf(
					"compact recovery chunk %s still exceeds the context window at %d characters after bounded splitting",
					chunkLabel, chunkRunes,
				),
			}
		}

		nextTarget := ceilDiv(chunkRunes, 2)
		if nextTarget < openAIAnthropicCompactFallbackMinSplitRunes {
			nextTarget = openAIAnthropicCompactFallbackMinSplitRunes
		}
		parts := splitTextByRuneLimit(chunk, nextTarget)
		if len(parts) <= 1 {
			return nil, usage, requestID, &openAICompactContextLengthError{
				message: fmt.Sprintf("compact recovery chunk %s could not be split below the upstream context limit", chunkLabel),
			}
		}
		for _, part := range parts {
			if runeLen(part) >= chunkRunes {
				return nil, usage, requestID, &openAICompactContextLengthError{
					message: fmt.Sprintf("compact recovery chunk %s split made no progress", chunkLabel),
				}
			}
		}
		*remainingSplits = *remainingSplits - 1

		logger.L().Warn("openai_messages.compact_chunk_context_split",
			zap.Int64("account_id", account.ID),
			zap.String("upstream_model", upstreamModel),
			zap.String("chunk_label", chunkLabel),
			zap.Int("chunk_chars", chunkRunes),
			zap.Int("split_count", len(parts)),
			zap.Int("split_depth", depth+1),
		)

		summaries := make([]string, 0, len(parts))
		lastRequestID := requestID
		for i, part := range parts {
			partSummaries, partUsage, partRequestID, partErr := s.summarizeAnthropicCompactChunk(
				ctx, c, account, token, upstreamModel, part,
				fmt.Sprintf("%s.%d/%d", chunkLabel, i+1, len(parts)), depth+1, remainingSplits,
			)
			usage = addOpenAIUsage(usage, partUsage)
			lastRequestID = firstNonEmpty(partRequestID, lastRequestID)
			if partErr != nil {
				return nil, usage, lastRequestID, partErr
			}
			summaries = append(summaries, partSummaries...)
		}
		return summaries, usage, lastRequestID, nil
	}
	if err != nil {
		return nil, usage, requestID, err
	}
	if response == nil || strings.TrimSpace(response.Status) == "failed" {
		message := "OpenAI compact recovery chunk failed"
		if response != nil && response.Error != nil && strings.TrimSpace(response.Error.Message) != "" {
			message = response.Error.Message
		}
		return nil, usage, requestID, errors.New(message)
	}
	summary := strings.TrimSpace(openAIResponsesOutputText(response))
	if summary == "" {
		return nil, usage, requestID, fmt.Errorf("OpenAI compact recovery chunk %s produced no summary", chunkLabel)
	}
	return []string{summary}, usage, requestID, nil
}

func compactRecoveryResult(requestID, responseID string, usage OpenAIUsage, originalModel, billingModel, upstreamModel string, stream bool, start time.Time) *OpenAIForwardResult {
	return &OpenAIForwardResult{
		RequestID:     requestID,
		ResponseID:    responseID,
		Usage:         usage,
		Model:         originalModel,
		BillingModel:  billingModel,
		UpstreamModel: upstreamModel,
		Stream:        stream,
		Duration:      time.Since(start),
	}
}

func (s *OpenAIGatewayService) wrapAnthropicCompactRecoveryError(
	c *gin.Context,
	account *Account,
	requestID string,
	err error,
) error {
	var upstreamErr *UpstreamFailoverError
	if errors.As(err, &upstreamErr) {
		return upstreamErr
	}
	return s.newOpenAIStreamFailoverError(c, account, false, requestID, nil, err.Error())
}

func compactResponseID(resp *apicompat.ResponsesResponse) string {
	if resp == nil {
		return ""
	}
	return resp.ID
}

func (s *OpenAIGatewayService) mergeAnthropicCompactSummaries(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	upstreamModel string,
	compactPrompt string,
	summaries []string,
	targetChars int,
	depth int,
) (*apicompat.ResponsesResponse, OpenAIUsage, string, error) {
	remainingAttempts := openAIAnthropicCompactMergeAttemptBudget
	return s.mergeAnthropicCompactSummariesWithBudget(
		ctx, c, account, token, upstreamModel, compactPrompt, summaries,
		targetChars, depth, &remainingAttempts,
	)
}

func (s *OpenAIGatewayService) mergeAnthropicCompactSummariesWithBudget(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	upstreamModel string,
	compactPrompt string,
	summaries []string,
	targetChars int,
	depth int,
	remainingAttempts *int,
) (*apicompat.ResponsesResponse, OpenAIUsage, string, error) {
	if len(summaries) == 0 {
		return nil, OpenAIUsage{}, "", errors.New("compact recovery merge summaries are empty")
	}
	if targetChars <= 0 {
		targetChars = openAIAnthropicCompactMergeTargetChars
	}
	if depth > openAIAnthropicCompactMergeMaxDepth || remainingAttempts == nil || *remainingAttempts <= 0 {
		emergency := buildAnthropicCompactEmergencySummary(compactPrompt, summaries)
		return buildAnthropicCompactEmergencyResponse(upstreamModel, emergency, OpenAIUsage{}), OpenAIUsage{}, "", nil
	}

	groups := groupAnthropicCompactSummariesForMerge(compactPrompt, summaries, targetChars)
	if len(groups) > 1 {
		totalUsage := OpenAIUsage{}
		reduced := make([]string, 0, len(groups))
		lastRequestID := ""
		for i, group := range groups {
			groupResp, groupUsage, requestID, err := s.mergeAnthropicCompactSummariesWithBudget(
				ctx, c, account, token, upstreamModel, compactPrompt, group, targetChars, depth+1, remainingAttempts,
			)
			totalUsage = addOpenAIUsage(totalUsage, groupUsage)
			lastRequestID = firstNonEmpty(requestID, lastRequestID)
			if err != nil {
				return groupResp, totalUsage, lastRequestID, err
			}
			text := strings.TrimSpace(openAIResponsesOutputText(groupResp))
			if text == "" {
				return groupResp, totalUsage, lastRequestID, fmt.Errorf("compact recovery merge group %d produced no text", i+1)
			}
			reduced = append(reduced, fmt.Sprintf("## Summary group %d/%d\n%s", i+1, len(groups), text))
		}
		finalResp, finalUsage, requestID, err := s.mergeAnthropicCompactSummariesWithBudget(
			ctx, c, account, token, upstreamModel, compactPrompt, reduced, targetChars, depth+1, remainingAttempts,
		)
		totalUsage = addOpenAIUsage(totalUsage, finalUsage)
		return finalResp, totalUsage, firstNonEmpty(requestID, lastRequestID), err
	}

	mergePrompt := buildAnthropicCompactMergePrompt(compactPrompt, summaries)
	logger.L().Info("openai_messages.compact_merge_attempt",
		zap.Int64("account_id", account.ID),
		zap.String("upstream_model", upstreamModel),
		zap.Int("merge_depth", depth),
		zap.Int("summary_count", len(summaries)),
		zap.Int("merge_chars", runeLen(mergePrompt)),
	)
	*remainingAttempts = *remainingAttempts - 1
	finalResponse, usage, requestID, err := s.runOpenAIAnthropicCompactRecoveryRequest(
		ctx, c, account, token, upstreamModel, openAIAnthropicCompactMergeInstructions(),
		mergePrompt, openAIAnthropicCompactMergeMaxOutputTokens,
	)
	if err != nil && !isOpenAICompactContextLengthError(err) {
		return finalResponse, usage, requestID, err
	}
	if finalResponse == nil && !isOpenAICompactContextLengthError(err) {
		return nil, usage, requestID, errors.New("compact recovery merge response is nil")
	}
	if isOpenAIResponsesContextLengthExceeded(finalResponse) || isOpenAICompactContextLengthError(err) {
		nextTarget := targetChars / 2
		if nextTarget < openAIAnthropicCompactFallbackMinSplitRunes {
			nextTarget = openAIAnthropicCompactFallbackMinSplitRunes
		}
		retry := retryAnthropicCompactFallbackSummaries(compactPrompt, summaries, nextTarget)
		if len(retry) > 0 && nextTarget < targetChars {
			retryResp, retryUsage, retryRequestID, retryErr := s.mergeAnthropicCompactSummariesWithBudget(
				ctx, c, account, token, upstreamModel, compactPrompt, retry, nextTarget, depth+1, remainingAttempts,
			)
			usage = addOpenAIUsage(usage, retryUsage)
			return retryResp, usage, firstNonEmpty(retryRequestID, requestID), retryErr
		}
		emergency := buildAnthropicCompactEmergencySummary(compactPrompt, summaries)
		return buildAnthropicCompactEmergencyResponse(upstreamModel, emergency, usage), usage, requestID, nil
	}
	if strings.TrimSpace(finalResponse.Status) == "failed" {
		return finalResponse, usage, requestID, fmt.Errorf("compact recovery merge failed: %s", compactResponseErrorMessage(finalResponse))
	}
	if strings.TrimSpace(openAIResponsesOutputText(finalResponse)) == "" {
		return finalResponse, usage, requestID, errors.New("compact recovery merge produced no summary text")
	}
	return finalResponse, usage, requestID, nil
}

func (s *OpenAIGatewayService) runOpenAIAnthropicCompactRecoveryRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	upstreamModel string,
	instructions string,
	userText string,
	maxOutputTokens int,
) (*apicompat.ResponsesResponse, OpenAIUsage, string, error) {
	body, requestModel, err := s.buildOpenAIAnthropicCompactRecoveryBody(
		account, upstreamModel, instructions, userText, maxOutputTokens,
	)
	if err != nil {
		return nil, OpenAIUsage{}, "", err
	}
	body, err = s.applyOpenAIFastPolicyToBody(ctx, account, requestModel, body)
	if err != nil {
		return nil, OpenAIUsage{}, "", err
	}

	upstreamCtx, release := detachUpstreamContext(ctx)
	req, err := s.buildUpstreamRequest(upstreamCtx, c, account, body, token, true, "", false)
	release()
	if err != nil {
		return nil, OpenAIUsage{}, "", err
	}
	req.Header.Del("conversation_id")
	req.Header.Del("session_id")
	req.Header.Del("x-codex-turn-state")
	req.Header.Del("x-codex-turn-metadata")
	if account.Type == AccountTypeOAuth {
		req.Header.Del("OpenAI-Beta")
		req.Header.Del("originator")
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.doOpenAIAnthropicRequestWithCompactKeepalive(
		c, req, proxyURL, account.ID, account.Concurrency,
	)
	if err != nil {
		return nil, OpenAIUsage{}, "", fmt.Errorf("compact recovery upstream request failed: %s", sanitizeUpstreamErrorMessage(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	requestID := strings.TrimSpace(resp.Header.Get("x-request-id"))
	if resp.StatusCode >= 400 {
		respBody := s.readUpstreamErrorBody(resp)
		usage, _ := extractOpenAIUsageFromJSONBytes(respBody)
		message := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		if isOpenAICompactContextLengthHTTPError(resp.StatusCode, message, respBody) {
			return nil, usage, requestID, &openAICompactContextLengthError{
				statusCode: resp.StatusCode,
				message:    message,
			}
		}
		return nil, usage, requestID, s.newOpenAIAnthropicCompactRecoveryHTTPError(
			c, account, resp.StatusCode, resp.Header, respBody, requestID, message,
		)
	}

	finalResponse, usage, acc, err := s.readOpenAICompatBufferedTerminalWithKeepalive(
		resp, "openai messages compact recovery", requestID, c, nil,
	)
	if err != nil {
		return nil, usage, requestID, err
	}
	if finalResponse == nil {
		return nil, usage, requestID, errors.New("compact recovery stream ended without terminal response")
	}
	acc.SupplementResponseOutput(finalResponse)
	if strings.TrimSpace(finalResponse.Status) == "failed" {
		if isOpenAIResponsesContextLengthExceeded(finalResponse) {
			return finalResponse, usage, requestID, &openAICompactContextLengthError{
				statusCode: http.StatusBadRequest,
				message:    compactResponseErrorMessage(finalResponse),
			}
		}
		payload, _ := json.Marshal(gin.H{"type": "response.failed", "response": finalResponse})
		return finalResponse, usage, requestID,
			s.openAIMessagesTerminalFailureError(c, account, requestID, finalResponse, payload)
	}
	return finalResponse, usage, requestID, nil
}

func (s *OpenAIGatewayService) newOpenAIAnthropicCompactRecoveryHTTPError(
	c *gin.Context,
	account *Account,
	statusCode int,
	headers http.Header,
	responseBody []byte,
	requestID string,
	message string,
) *UpstreamFailoverError {
	if statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError &&
		!s.shouldFailoverOpenAIUpstreamResponse(statusCode, message, responseBody) {
		return s.newOpenAIStreamClientError(
			c, account, requestID, statusCode, "invalid_request_error", message,
		)
	}

	setOpsUpstreamError(c, statusCode, message, "")
	event := OpsUpstreamErrorEvent{
		Platform:           PlatformOpenAI,
		UpstreamStatusCode: statusCode,
		UpstreamRequestID:  strings.TrimSpace(requestID),
		Kind:               "failover",
		Message:            message,
	}
	if account != nil {
		event.Platform = account.Platform
		event.AccountID = account.ID
		event.AccountName = account.Name
	}
	appendOpsUpstreamError(c, event)
	return &UpstreamFailoverError{
		StatusCode:             statusCode,
		ResponseBody:           append([]byte(nil), responseBody...),
		ResponseHeaders:        headers.Clone(),
		RetryableOnSameAccount: account != nil && account.IsPoolMode() && isPoolModeRetryableStatus(statusCode),
	}
}

func (s *OpenAIGatewayService) buildOpenAIAnthropicCompactRecoveryBody(
	account *Account,
	upstreamModel string,
	instructions string,
	userText string,
	maxOutputTokens int,
) ([]byte, string, error) {
	content, err := json.Marshal([]apicompat.ResponsesContentPart{{Type: "input_text", Text: userText}})
	if err != nil {
		return nil, upstreamModel, err
	}
	input, err := json.Marshal([]apicompat.ResponsesInputItem{{Role: "user", Content: content}})
	if err != nil {
		return nil, upstreamModel, err
	}
	store := false
	req := apicompat.ResponsesRequest{
		Model:           upstreamModel,
		Instructions:    instructions,
		Input:           input,
		MaxOutputTokens: &maxOutputTokens,
		Stream:          true,
		Store:           &store,
		Reasoning:       &apicompat.ResponsesReasoning{Effort: openAIAnthropicCompactFallbackReasoning},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, upstreamModel, err
	}

	requestModel := upstreamModel
	if account.Type == AccountTypeOAuth {
		var requestMap map[string]any
		if err := json.Unmarshal(body, &requestMap); err != nil {
			return nil, requestModel, err
		}
		codexResult := applyCodexOAuthTransformWithOptions(requestMap, codexOAuthTransformOptions{
			SkipDefaultInstructions: true,
			PreserveToolCallIDs:     true,
		})
		if codexResult.NormalizedModel != "" {
			requestModel = codexResult.NormalizedModel
		}
		ensureCodexOAuthInstructionsField(requestMap)
		delete(requestMap, "prompt_cache_key")
		body, err = json.Marshal(requestMap)
		if err != nil {
			return nil, requestModel, err
		}
	}
	return body, requestModel, nil
}

func (s *OpenAIGatewayService) writeAnthropicCompactFinalResponse(
	c *gin.Context,
	account *Account,
	upstreamHeaders http.Header,
	finalResponse *apicompat.ResponsesResponse,
	usage OpenAIUsage,
	bridgeMode bool,
	originalModel string,
	billingModel string,
	upstreamModel string,
	startTime time.Time,
	clientStream bool,
	requestID string,
) (*OpenAIForwardResult, error) {
	if finalResponse == nil {
		return nil, errors.New("compact final response is nil")
	}
	if finalResponse.Usage == nil {
		finalResponse.Usage = responsesUsageFromOpenAIUsage(usage)
	}
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	if bridgeMode && finalResponse.Usage != nil {
		cachedTokens := 0
		if finalResponse.Usage.InputTokensDetails != nil {
			cachedTokens = finalResponse.Usage.InputTokensDetails.CachedTokens
		}
		logClaudeGPTBridgeRawUsage(
			"compact_final", requestID, accountID, originalModel, billingModel, upstreamModel,
			finalResponse.Usage.InputTokens, finalResponse.Usage.OutputTokens, cachedTokens, clientStream,
		)
		s.applyClaudeGPTBridgeDisplayCacheOverride(
			c.Request.Context(), finalResponse.Usage, requestID, accountID,
			originalModel, billingModel, upstreamModel, clientStream,
		)
		usage = copyOpenAIUsageFromResponsesUsage(finalResponse.Usage)
	}

	anthropicResp := apicompat.ResponsesToAnthropic(finalResponse, originalModel)
	if !anthropicResponseHasVisibleOutput(anthropicResp) {
		return compactRecoveryResult(requestID, finalResponse.ID, usage, originalModel, billingModel, upstreamModel, clientStream, startTime),
			s.newOpenAIStreamFailoverError(c, account, false, requestID, nil,
				"OpenAI compact recovery completed without a usable summary")
	}
	if bridgeMode {
		logClaudeGPTBridgeConvertedUsage(
			"compact_final", requestID, accountID, originalModel, billingModel,
			upstreamModel, anthropicResp.Usage, clientStream,
		)
	}
	if s.responseHeaderFilter != nil && upstreamHeaders != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), upstreamHeaders, s.responseHeaderFilter)
	}
	result := compactRecoveryResult(
		requestID, finalResponse.ID, usage, originalModel, billingModel,
		upstreamModel, clientStream, startTime,
	)
	result.ClientOutputStarted = true
	if clientStream {
		markOpenAIAnthropicSemanticOutputStarted(c)
		if err := writeAnthropicResponseAsSSE(c, anthropicResp); err != nil {
			return result, err
		}
	} else {
		writeAnthropicJSONResponse(c, http.StatusOK, anthropicResp)
	}
	return result, nil
}

func writeAnthropicResponseAsSSE(c *gin.Context, resp *apicompat.AnthropicResponse) error {
	if c == nil || resp == nil {
		return errors.New("anthropic SSE response is nil")
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Status(http.StatusOK)

	start := *resp
	start.Content = []apicompat.AnthropicContentBlock{}
	start.StopReason = ""
	start.StopSequence = nil
	start.Usage.OutputTokens = 0
	events := []apicompat.AnthropicStreamEvent{{Type: "message_start", Message: &start}}
	for i, block := range resp.Content {
		idx := i
		startBlock := block
		switch strings.TrimSpace(startBlock.Type) {
		case "text":
			startBlock.Text = ""
		case "thinking":
			startBlock.Thinking = ""
		}
		events = append(events, apicompat.AnthropicStreamEvent{
			Type: "content_block_start", Index: &idx, ContentBlock: &startBlock,
		})
		switch strings.TrimSpace(block.Type) {
		case "text":
			if block.Text != "" {
				events = append(events, apicompat.AnthropicStreamEvent{
					Type: "content_block_delta", Index: &idx,
					Delta: &apicompat.AnthropicDelta{Type: "text_delta", Text: block.Text},
				})
			}
		case "thinking":
			if block.Thinking != "" {
				events = append(events, apicompat.AnthropicStreamEvent{
					Type: "content_block_delta", Index: &idx,
					Delta: &apicompat.AnthropicDelta{Type: "thinking_delta", Thinking: block.Thinking},
				})
			}
		}
		events = append(events, apicompat.AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
	}
	stopReason := strings.TrimSpace(resp.StopReason)
	if stopReason == "" {
		stopReason = "end_turn"
	}
	events = append(events,
		apicompat.AnthropicStreamEvent{
			Type:  "message_delta",
			Delta: &apicompat.AnthropicDelta{StopReason: stopReason, StopSequence: resp.StopSequence},
			Usage: &resp.Usage,
		},
		apicompat.AnthropicStreamEvent{Type: "message_stop"},
	)
	for _, event := range events {
		sse, err := apicompat.ResponsesAnthropicEventToSSE(event)
		if err != nil {
			return err
		}
		if mult := getDisplayTokenMultipliers(c); mult != nil {
			sse = rewriteAnthropicSSEUsageTokens(sse, mult)
		}
		if _, err := fmt.Fprint(c.Writer, sse); err != nil {
			return err
		}
	}
	c.Writer.Flush()
	MarkOpenAIAnthropicResponseTerminated(c)
	return nil
}

func buildAnthropicCompactFallbackTranscript(req *apicompat.AnthropicRequest) (string, string) {
	if req == nil {
		return "", ""
	}
	compactIndex := -1
	compactPrompt := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(req.Messages[i].Role) != "user" {
			continue
		}
		text := anthropicContentTextForCompactFallback(req.Messages[i].Content)
		if looksLikeClaudeCodeCompactPrompt(text) {
			compactIndex = i
			compactPrompt = text
			break
		}
	}
	parts := make([]string, 0, len(req.Messages)+1)
	if systemText := strings.TrimSpace(anthropicContentTextForCompactFallback(req.System)); systemText != "" {
		parts = append(parts, "### System\n"+systemText)
	}
	for i, message := range req.Messages {
		if i == compactIndex {
			continue
		}
		text := strings.TrimSpace(anthropicContentTextForCompactFallback(message.Content))
		if text == "" {
			continue
		}
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "message"
		}
		parts = append(parts, fmt.Sprintf("### Message %d (%s)\n%s", i+1, role, text))
	}
	return compactPrompt, strings.Join(parts, "\n\n")
}

func anthropicContentTextForCompactFallback(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var blocks []apicompat.AnthropicContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return strings.TrimSpace(string(raw))
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch strings.TrimSpace(block.Type) {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "thinking":
			if block.Thinking != "" {
				parts = append(parts, "[thinking]\n"+block.Thinking)
			}
		case "tool_use", "server_tool_use":
			input := strings.TrimSpace(string(block.Input))
			if input == "" {
				input = "{}"
			}
			parts = append(parts, fmt.Sprintf("[tool_use id=%s name=%s]\n%s", block.ID, block.Name, input))
		case "tool_result", "web_search_tool_result":
			content := anthropicContentTextForCompactFallback(block.Content)
			if content == "" {
				content = strings.TrimSpace(string(block.Content))
			}
			prefix := fmt.Sprintf("[tool_result tool_use_id=%s]", block.ToolUseID)
			if block.IsError {
				prefix += " [error]"
			}
			parts = append(parts, prefix+"\n"+content)
		case "image":
			parts = append(parts, "[image omitted]")
		default:
			if encoded, err := json.Marshal(block); err == nil {
				parts = append(parts, string(encoded))
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func splitAnthropicCompactTranscriptChunks(text string, targetChars, maxChunks int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if targetChars <= 0 {
		targetChars = openAIAnthropicCompactChunkTargetChars
	}
	if maxChunks <= 0 {
		maxChunks = openAIAnthropicCompactFallbackMaxChunks
	}
	sections := strings.Split(text, "\n\n### ")
	var chunks []string
	current := ""
	for i, section := range sections {
		if i > 0 {
			section = "### " + section
		}
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		if runeLen(section) > targetChars {
			if current != "" {
				chunks = append(chunks, current)
				current = ""
			}
			chunks = append(chunks, splitTextByRuneLimit(section, targetChars)...)
			continue
		}
		candidate := section
		if current != "" {
			candidate = current + "\n\n" + section
		}
		if current != "" && runeLen(candidate) > targetChars {
			chunks = append(chunks, current)
			current = section
			continue
		}
		current = candidate
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	if len(chunks) <= maxChunks {
		return chunks
	}
	return splitTextByRuneLimit(text, ceilDiv(runeLen(text), maxChunks))
}

func splitTextByRuneLimit(text string, targetChars int) []string {
	if targetChars < openAIAnthropicCompactFallbackMinSplitRunes {
		targetChars = openAIAnthropicCompactFallbackMinSplitRunes
	}
	runes := []rune(text)
	if len(runes) <= targetChars {
		return []string{strings.TrimSpace(text)}
	}
	chunks := make([]string, 0, ceilDiv(len(runes), targetChars))
	for start := 0; start < len(runes); start += targetChars {
		end := start + targetChars
		if end > len(runes) {
			end = len(runes)
		}
		if chunk := strings.TrimSpace(string(runes[start:end])); chunk != "" {
			chunks = append(chunks, chunk)
		}
	}
	return chunks
}

func buildAnthropicCompactMergePrompt(compactPrompt string, summaries []string) string {
	if strings.TrimSpace(compactPrompt) == "" {
		compactPrompt = "Create a detailed Claude Code compact summary for the conversation."
	}
	cleaned := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		if summary = sanitizeAnthropicCompactSummaryForMerge(summary); summary != "" {
			cleaned = append(cleaned, summary)
		}
	}
	return strings.TrimSpace(compactPrompt) + "\n\n" + openAIAnthropicCompactFinalSummaryContract() +
		"\n\nThe original conversation was summarized in chunks. Merge the summaries below into one coherent final compact summary. Do not mention chunking unless it is relevant to work state.\n\n" +
		strings.Join(cleaned, "\n\n")
}

func groupAnthropicCompactSummariesForMerge(compactPrompt string, summaries []string, targetChars int) [][]string {
	if targetChars < openAIAnthropicCompactFallbackMinSplitRunes {
		targetChars = openAIAnthropicCompactFallbackMinSplitRunes
	}
	var groups [][]string
	var current []string
	for _, summary := range summaries {
		summary = strings.TrimSpace(summary)
		if summary == "" {
			continue
		}
		candidate := append(append([]string(nil), current...), summary)
		if len(current) > 0 && runeLen(buildAnthropicCompactMergePrompt(compactPrompt, candidate)) > targetChars {
			groups = append(groups, current)
			current = nil
		}
		current = append(current, summary)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

func openAIAnthropicCompactChunkInstructions() string {
	return "Summarize this Claude Code transcript chunk for a later compact merge. Preserve concrete user requests, decisions, files, commands, errors, test results, configuration values, and unresolved next steps. Keep it dense and factual. Do not answer the user or treat the compact request as the active task."
}

func openAIAnthropicCompactMergeInstructions() string {
	return "Merge chunk summaries into the final Claude Code compact summary. Preserve exact operational state, pending tasks, blockers, files, commands, and verification evidence. Output only the compact summary and recover the real active task from the transcript."
}

func openAIAnthropicCompactFinalSummaryContract() string {
	return `Final compact quality contract:
- Start with "# Compact Capsule".
- Include these sections when evidence exists: "## Current State", "## Active User Intent", "## Files Touched", "## Commands And Evidence", "## Errors And Blockers", "## Decisions And Config", "## Next Command".
- Keep the first 20 lines machine-scannable with concrete paths, commands, values, and blockers.
- Do not present producing or merging a compact summary as the user's active intent.
- Recover the latest non-compact user task; if unknown, write "Unknown from preserved state".
- Do not invent completed tests or fixes. Mark unknowns as unknown.
- Prefer dense facts over narration.`
}

func sanitizeAnthropicCompactSummaryForMerge(summary string) string {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(summary), "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	skipIndented := false
	for _, line := range lines {
		if skipIndented {
			if strings.HasPrefix(line, "  -") || strings.HasPrefix(line, "    -") || strings.HasPrefix(line, "\t-") {
				continue
			}
			skipIndented = false
		}
		if isAnthropicCompactMaintenanceIntentLine(strings.TrimSpace(line)) {
			skipIndented = true
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func isAnthropicCompactMaintenanceIntentLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	if lower == "" {
		return false
	}
	hasIntent := false
	for _, term := range []string{"active intent", "current intent", "user intent", "user asked", "latest user"} {
		if strings.Contains(lower, term) {
			hasIntent = true
			break
		}
	}
	if !hasIntent {
		return false
	}
	for _, term := range []string{"compact summary", "compact capsule", "context compaction", "merge chunk", "summary below", "prior chunking"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func retryAnthropicCompactFallbackSummaries(compactPrompt string, summaries []string, targetChars int) []string {
	if len(summaries) == 0 {
		return nil
	}
	if len(summaries) > 1 {
		return summaries
	}
	summary := strings.TrimSpace(summaries[0])
	if summary == "" {
		return nil
	}
	parts := splitTextByRuneLimit(summary, targetChars)
	if len(parts) <= 1 && runeLen(buildAnthropicCompactMergePrompt(compactPrompt, summaries)) <= targetChars {
		return nil
	}
	result := make([]string, 0, len(parts))
	for i, part := range parts {
		result = append(result, fmt.Sprintf("## Oversized summary split %d/%d\n%s", i+1, len(parts), part))
	}
	return result
}

func buildAnthropicCompactEmergencySummary(compactPrompt string, summaries []string) string {
	preserved := strings.TrimSpace(strings.Join(summaries, "\n\n"))
	if preserved == "" {
		preserved = strings.TrimSpace(compactPrompt)
	}
	if preserved == "" {
		preserved = openAIAnthropicCompactFallbackClientMessage
	}
	preserved = trimRunesMiddle(preserved, openAIAnthropicCompactEmergencyMaxRunes)
	return "# Compact Capsule\n\n## Current State\n- The proxy used its emergency compact fallback after upstream merge context limits.\n\n## Active User Intent\n- Continue the original task from the preserved state below.\n\n## Preserved State\n" + preserved + "\n\n## Next Command\n- Resume the latest non-compact user task from this state."
}

func buildAnthropicCompactEmergencyResponse(model, summary string, usage OpenAIUsage) *apicompat.ResponsesResponse {
	if strings.TrimSpace(model) == "" {
		model = "compact-fallback"
	}
	return &apicompat.ResponsesResponse{
		ID:     fmt.Sprintf("compact_fallback_%d", time.Now().UnixNano()),
		Object: "response",
		Model:  model,
		Status: "completed",
		Output: []apicompat.ResponsesOutput{{
			Type: "message", Role: "assistant", Status: "completed",
			Content: []apicompat.ResponsesContentPart{{Type: "output_text", Text: summary}},
		}},
		Usage: responsesUsageFromOpenAIUsage(usage),
	}
}

func trimRunesMiddle(text string, maxRunes int) string {
	runes := []rune(text)
	if maxRunes <= 0 {
		return ""
	}
	if len(runes) <= maxRunes {
		return text
	}
	head := maxRunes * 2 / 3
	tail := maxRunes - head
	return string(runes[:head]) + "\n\n[... middle omitted by compact emergency guard ...]\n\n" + string(runes[len(runes)-tail:])
}

func openAIResponsesOutputText(resp *apicompat.ResponsesResponse) string {
	if resp == nil {
		return ""
	}
	var parts []string
	for _, output := range resp.Output {
		if strings.TrimSpace(output.Type) != "message" {
			continue
		}
		for _, part := range output.Content {
			if strings.TrimSpace(part.Type) == "output_text" && part.Text != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func compactResponseErrorMessage(resp *apicompat.ResponsesResponse) string {
	if resp == nil || resp.Error == nil {
		return ""
	}
	if message := strings.TrimSpace(resp.Error.Message); message != "" {
		return message
	}
	return strings.TrimSpace(resp.Error.Code)
}

func responsesUsageFromOpenAIUsage(usage OpenAIUsage) *apicompat.ResponsesUsage {
	result := &apicompat.ResponsesUsage{
		InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
		TotalTokens: usage.InputTokens + usage.OutputTokens,
	}
	if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 {
		result.InputTokensDetails = &apicompat.ResponsesInputTokensDetails{
			CachedTokens:     usage.CacheReadInputTokens,
			CacheWriteTokens: usage.CacheCreationInputTokens,
		}
	}
	return result
}

func addOpenAIUsage(a, b OpenAIUsage) OpenAIUsage {
	return OpenAIUsage{
		InputTokens:              a.InputTokens + b.InputTokens,
		OutputTokens:             a.OutputTokens + b.OutputTokens,
		CacheCreationInputTokens: a.CacheCreationInputTokens + b.CacheCreationInputTokens,
		CacheReadInputTokens:     a.CacheReadInputTokens + b.CacheReadInputTokens,
		ImageOutputTokens:        a.ImageOutputTokens + b.ImageOutputTokens,
	}
}

func runeLen(text string) int { return len([]rune(text)) }

func ceilDiv(a, b int) int {
	if b <= 0 {
		return 0
	}
	return (a + b - 1) / b
}
