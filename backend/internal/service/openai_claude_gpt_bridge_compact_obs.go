package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Lifecycle observability for Claude→GPT bridge prompt-too-long vs Claude Code compact.
// Production runs at INFO: these events must stay Info/Warn so operators can tell:
//   - prompt_too_long without later compact_detected → client did not send (or we did not recognize) compact
//   - compact_detected + compact_failed → client sent compact, compression failed
//   - compact_detected + compact_succeeded → client sent compact and gateway finished successfully
const (
	claudeGPTBridgeObsContextKey          = "claude_gpt_bridge_obs"
	claudeGPTBridgeCompactRecoveryUsedKey = "claude_gpt_bridge_compact_recovery_used"
	claudeGPTBridgePromptTooLongMsg       = "claude_gpt_bridge.prompt_too_long"
	claudeGPTBridgeCompactDetectedMsg     = "claude_gpt_bridge.compact_detected"
	claudeGPTBridgeCompactUnrecognizedMsg = "claude_gpt_bridge.compact_unrecognized"
	claudeGPTBridgeCompactSucceededMsg    = "claude_gpt_bridge.compact_succeeded"
	claudeGPTBridgeCompactFailedMsg       = "claude_gpt_bridge.compact_failed"
)

// claudeGPTBridgeObs is request-scoped correlation data for PTL/compact lifecycle logs.
type claudeGPTBridgeObs struct {
	SessionKey    string
	OriginalModel string
	BillingModel  string
	UpstreamModel string
	BodyBytes     int
	MessageCount  int
	ClientStream  bool
	BridgeMode    bool
	CompactMapped bool
	RequestPath   string
	UserAgent     string
}

func setClaudeGPTBridgeObs(c *gin.Context, obs claudeGPTBridgeObs) {
	if c == nil {
		return
	}
	c.Set(claudeGPTBridgeObsContextKey, obs)
}

func getClaudeGPTBridgeObs(c *gin.Context) claudeGPTBridgeObs {
	if c == nil {
		return claudeGPTBridgeObs{}
	}
	v, ok := c.Get(claudeGPTBridgeObsContextKey)
	if !ok {
		return claudeGPTBridgeObs{}
	}
	obs, _ := v.(claudeGPTBridgeObs)
	return obs
}

func markClaudeGPTBridgeCompactRecoveryUsed(c *gin.Context) {
	if c != nil {
		c.Set(claudeGPTBridgeCompactRecoveryUsedKey, true)
	}
}

func claudeGPTBridgeCompactRecoveryUsed(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get(claudeGPTBridgeCompactRecoveryUsedKey)
	if !ok {
		return false
	}
	used, _ := v.(bool)
	return used
}

func getAPIKeyUserIDFromContext(c *gin.Context) int64 {
	if c == nil {
		return 0
	}
	v, exists := c.Get("api_key")
	if !exists {
		return 0
	}
	apiKey, ok := v.(*APIKey)
	if !ok || apiKey == nil {
		return 0
	}
	return apiKey.UserID
}

func claudeGPTBridgeObsBaseFields(c *gin.Context, account *Account, requestID string) []zap.Field {
	obs := getClaudeGPTBridgeObs(c)
	path := obs.RequestPath
	ua := obs.UserAgent
	if c != nil && c.Request != nil {
		if path == "" {
			path = c.Request.URL.Path
		}
		if ua == "" {
			ua = c.Request.UserAgent()
		}
	}
	fields := []zap.Field{
		zap.String("request_id", strings.TrimSpace(requestID)),
		zap.Int64("user_id", getAPIKeyUserIDFromContext(c)),
		zap.Int64("api_key_id", getAPIKeyIDFromContext(c)),
		zap.String("session_key_sha256", hashSensitiveValueForLog(obs.SessionKey)),
		zap.String("original_model", obs.OriginalModel),
		zap.String("billing_model", obs.BillingModel),
		zap.String("upstream_model", obs.UpstreamModel),
		zap.Int("body_bytes", obs.BodyBytes),
		zap.Int("message_count", obs.MessageCount),
		zap.Bool("stream", obs.ClientStream),
		zap.Bool("bridge_mode", obs.BridgeMode),
		zap.String("request_path", path),
		zap.String("user_agent", truncateString(ua, 120)),
	}
	if account != nil {
		fields = append(fields,
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.String("account_type", account.Type),
			zap.String("platform", account.Platform),
		)
	}
	return fields
}

func logClaudeGPTBridgePromptTooLong(c *gin.Context, account *Account, requestID, source string) {
	fields := claudeGPTBridgeObsBaseFields(c, account, requestID)
	fields = append(fields,
		zap.String("source", strings.TrimSpace(source)),
		zap.Int("status_code", 413),
		zap.String("client_message", claudeCodePromptTooLongClientMessage),
	)
	logger.L().Info(claudeGPTBridgePromptTooLongMsg, fields...)
}

func logClaudeGPTBridgeCompactDetected(c *gin.Context, account *Account) {
	fields := claudeGPTBridgeObsBaseFields(c, account, "")
	obs := getClaudeGPTBridgeObs(c)
	fields = append(fields, zap.Bool("compact_model_mapped", obs.CompactMapped))
	logger.L().Info(claudeGPTBridgeCompactDetectedMsg, fields...)
}

func logClaudeGPTBridgeCompactUnrecognized(c *gin.Context, account *Account, lastUserText string) {
	fields := claudeGPTBridgeObsBaseFields(c, account, "")
	fields = append(fields,
		zap.Bool("has_summary_task_marker", strings.Contains(lastUserText, "Your task is to create a detailed summary")),
		zap.Int("last_user_text_chars", len(lastUserText)),
	)
	logger.L().Info(claudeGPTBridgeCompactUnrecognizedMsg, fields...)
}

func logClaudeGPTBridgeCompactSucceeded(
	c *gin.Context,
	account *Account,
	result *OpenAIForwardResult,
	recoveryUsed bool,
	startedAt time.Time,
) {
	requestID := ""
	fields := claudeGPTBridgeObsBaseFields(c, account, "")
	if result != nil {
		requestID = strings.TrimSpace(result.RequestID)
		fields = append(fields,
			zap.String("request_id", requestID),
			zap.String("response_id", strings.TrimSpace(result.ResponseID)),
			zap.Int("input_tokens", result.Usage.InputTokens),
			zap.Int("output_tokens", result.Usage.OutputTokens),
			zap.Int("cache_read_tokens", result.Usage.CacheReadInputTokens),
			zap.Bool("client_output_started", result.ClientOutputStarted),
			zap.Bool("skip_continuation_binding", result.SkipContinuationBinding),
		)
		if result.UpstreamModel != "" {
			fields = append(fields, zap.String("upstream_model", result.UpstreamModel))
		}
	}
	if !startedAt.IsZero() {
		fields = append(fields, zap.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	}
	fields = append(fields, zap.Bool("recovery_used", recoveryUsed))
	logger.L().Info(claudeGPTBridgeCompactSucceededMsg, fields...)
}

func logClaudeGPTBridgeCompactFailed(
	c *gin.Context,
	account *Account,
	requestID string,
	err error,
	recoveryUsed bool,
	startedAt time.Time,
) {
	fields := claudeGPTBridgeObsBaseFields(c, account, requestID)
	reason, statusCode := classifyClaudeGPTBridgeCompactFailure(err)
	fields = append(fields,
		zap.String("reason", reason),
		zap.Int("status_code", statusCode),
		zap.Bool("recovery_used", recoveryUsed),
	)
	if !startedAt.IsZero() {
		fields = append(fields, zap.Int64("duration_ms", time.Since(startedAt).Milliseconds()))
	}
	if err != nil {
		fields = append(fields,
			zap.String("error_type", fmt.Sprintf("%T", err)),
			zap.String("error_message", truncateString(sanitizeUpstreamErrorMessage(err.Error()), 240)),
		)
	}
	logger.L().Warn(claudeGPTBridgeCompactFailedMsg, fields...)
}

func classifyClaudeGPTBridgeCompactFailure(err error) (reason string, statusCode int) {
	if err == nil {
		return "unknown", 0
	}
	var upstreamErr *UpstreamFailoverError
	if errors.As(err, &upstreamErr) && upstreamErr != nil {
		statusCode = upstreamErr.StatusCode
		msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(upstreamErr.ResponseBody)))
		if msg == "" {
			msg = strings.ToLower(err.Error())
		}
		switch {
		case strings.Contains(msg, "context window") || strings.Contains(msg, "context length") || strings.Contains(msg, "prompt is too long"):
			return "upstream_context", statusCode
		case strings.Contains(msg, "could not be read to completion"):
			return "stream_incomplete", statusCode
		case strings.Contains(msg, "without a terminal"):
			return "no_terminal", statusCode
		case strings.Contains(msg, "without a usable summary") || strings.Contains(msg, "compact recovery failed"):
			return "empty_summary", statusCode
		case strings.Contains(msg, "context canceled") || strings.Contains(msg, "context cancelled"):
			return "client_cancel", statusCode
		case statusCode >= 500:
			return "upstream_5xx", statusCode
		case statusCode >= 400:
			return "upstream_4xx", statusCode
		default:
			return "upstream_error", statusCode
		}
	}
	if isOpenAICompactContextLengthError(err) {
		return "upstream_context", 0
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "context canceled") || strings.Contains(msg, "context cancelled"):
		return "client_cancel", 0
	case strings.Contains(msg, "context window") || strings.Contains(msg, "context length"):
		return "upstream_context", 0
	case strings.Contains(msg, "could not be read to completion"):
		return "stream_incomplete", 0
	default:
		return "error", 0
	}
}

func emitClaudeGPTBridgeCompactOutcome(
	c *gin.Context,
	account *Account,
	result *OpenAIForwardResult,
	err error,
	startedAt time.Time,
) {
	recoveryUsed := claudeGPTBridgeCompactRecoveryUsed(c)
	if err != nil {
		requestID := ""
		if result != nil {
			requestID = result.RequestID
		}
		logClaudeGPTBridgeCompactFailed(c, account, requestID, err, recoveryUsed, startedAt)
		return
	}
	logClaudeGPTBridgeCompactSucceeded(c, account, result, recoveryUsed, startedAt)
}

// maybeLogClaudeGPTBridgeCompactUnrecognized logs when a request looks like a
// compact attempt but fails the strict Claude Code marker check.
func maybeLogClaudeGPTBridgeCompactUnrecognized(c *gin.Context, account *Account, req *apicompat.AnthropicRequest) {
	if req == nil || isClaudeCodeCompactAnthropicRequest(req) {
		return
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.TrimSpace(req.Messages[i].Role) != "user" {
			continue
		}
		text := anthropicMessageText(req.Messages[i].Content)
		if strings.Contains(text, "Your task is to create a detailed summary") {
			logClaudeGPTBridgeCompactUnrecognized(c, account, text)
		}
		return
	}
}
