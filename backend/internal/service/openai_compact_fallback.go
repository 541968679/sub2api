package service

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const openAINativeCompactionV2Key = "openai_native_compaction_v2"

func MarkOpenAINativeCompactionV2(c *gin.Context) {
	if c != nil {
		c.Set(openAINativeCompactionV2Key, true)
	}
}

func isOpenAINativeCompactionV2(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get(openAINativeCompactionV2Key)
	if !ok {
		return false
	}
	marked, _ := v.(bool)
	return marked
}

func isExplicitOpenAICompactRequest(c *gin.Context, body []byte) bool {
	return isOpenAIResponsesCompactPath(c) || HasCompactionTriggerInInput(body)
}

func (s *OpenAIGatewayService) resolveOpenAICompactFallbackModel(account *Account, requestedModel string) string {
	requestedModel = strings.TrimSpace(requestedModel)
	if account != nil {
		if mapped, matched := account.ResolveCompactMappedModel(requestedModel); matched {
			if mapped = strings.TrimSpace(mapped); mapped != "" {
				return mapped
			}
		}
	}
	if s == nil || s.cfg == nil {
		return ""
	}
	fallback := strings.TrimSpace(s.cfg.Gateway.OpenAICompactModel)
	if fallback == "" {
		return ""
	}
	return strings.TrimSpace(resolveOpenAIAccountUpstreamModelForRequest(account, fallback, false))
}

func isOpenAIContextWindowError(upstreamMsg string, upstreamBody []byte) bool {
	match := func(text string) bool {
		lower := strings.ToLower(strings.TrimSpace(text))
		if lower == "" {
			return false
		}
		if strings.Contains(lower, "context_too_large") || strings.Contains(lower, "context_length_exceeded") {
			return true
		}
		if strings.Contains(lower, "maximum context length") || strings.Contains(lower, "max context length") {
			return true
		}
		hasExceeded := strings.Contains(lower, "exceed") || strings.Contains(lower, "too large") || strings.Contains(lower, "too long")
		if strings.Contains(lower, "context window") && hasExceeded {
			return true
		}
		if strings.Contains(lower, "context length") && hasExceeded {
			return true
		}
		return strings.Contains(lower, "token limit") &&
			strings.Contains(lower, "context") &&
			hasExceeded
	}

	if match(upstreamMsg) {
		return true
	}
	if len(upstreamBody) == 0 {
		return false
	}
	for _, path := range []string{
		"error.message",
		"response.error.message",
		"message",
		"error.code",
		"response.error.code",
		"code",
	} {
		if match(gjson.GetBytes(upstreamBody, path).String()) {
			return true
		}
	}
	return false
}

func isOpenAICompactModelFailure(statusCode int, upstreamMsg string, upstreamBody []byte) bool {
	if isOpenAIContextWindowError(upstreamMsg, upstreamBody) {
		return true
	}
	if statusCode != http.StatusBadRequest && statusCode != http.StatusNotFound {
		return false
	}

	values := []string{
		extractUpstreamErrorCode(upstreamBody),
		upstreamMsg,
		gjson.GetBytes(upstreamBody, "error.type").String(),
		gjson.GetBytes(upstreamBody, "response.error.code").String(),
		gjson.GetBytes(upstreamBody, "response.error.type").String(),
	}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		switch value {
		case "model_not_found", "model_not_available", "unsupported_model", "invalid_model":
			return true
		}
		if isExplicitOpenAIModelAvailabilityMessage(value) {
			return true
		}
	}
	if strings.EqualFold(strings.TrimSpace(gjson.GetBytes(upstreamBody, "response.status").String()), "failed") ||
		strings.EqualFold(strings.TrimSpace(gjson.GetBytes(upstreamBody, "status").String()), "failed") {
		for _, path := range []string{
			"error.message", "error.code", "error.type",
			"response.error.message", "response.error.code", "response.error.type",
		} {
			if strings.TrimSpace(gjson.GetBytes(upstreamBody, path).String()) != "" {
				return false
			}
		}
		return strings.TrimSpace(upstreamMsg) == ""
	}
	return false
}

func isExplicitOpenAIModelAvailabilityMessage(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, phrase := range []string{
		"model not found",
		"model does not exist",
		"model is unavailable",
		"model is not available",
		"model is unsupported",
		"model is not supported",
		"unsupported model",
	} {
		if strings.Contains(value, phrase) {
			return true
		}
	}
	if strings.HasPrefix(value, "the model ") || strings.HasPrefix(value, "model ") {
		return strings.Contains(value, " does not exist") ||
			strings.Contains(value, " was not found") ||
			strings.Contains(value, " is unavailable") ||
			strings.Contains(value, " is not available")
	}
	return false
}

// prepareOpenAICompactFallbackRetry returns a body for one safe, same-account
// retry. Callers invoke it only before any downstream response has been
// written; it changes the model and deliberately leaves path, trigger, and
// native-v2 context state untouched.
func (s *OpenAIGatewayService) prepareOpenAICompactFallbackRetry(
	c *gin.Context,
	account *Account,
	requestedModel string,
	currentBody []byte,
	statusCode int,
	upstreamMsg string,
	upstreamBody []byte,
	alreadyRetried bool,
) ([]byte, string, bool) {
	if alreadyRetried || !isExplicitOpenAICompactRequest(c, currentBody) ||
		!isOpenAICompactModelFailure(statusCode, upstreamMsg, upstreamBody) {
		return currentBody, "", false
	}
	fallbackModel := s.resolveOpenAICompactFallbackModel(account, requestedModel)
	currentModel := strings.TrimSpace(gjson.GetBytes(currentBody, "model").String())
	if fallbackModel == "" || strings.EqualFold(fallbackModel, currentModel) {
		return currentBody, "", false
	}
	retryBody := ReplaceModelInBody(currentBody, fallbackModel)
	if strings.EqualFold(strings.TrimSpace(gjson.GetBytes(retryBody, "model").String()), currentModel) {
		return currentBody, "", false
	}
	return retryBody, fallbackModel, true
}

func (s *OpenAIGatewayService) appendOpenAICompactFallbackRetryOps(
	c *gin.Context,
	account *Account,
	resp *http.Response,
	payload []byte,
	message string,
	passthrough bool,
) {
	if account == nil {
		return
	}
	statusCode := http.StatusBadRequest
	requestID := ""
	if resp != nil {
		statusCode = resp.StatusCode
		requestID = resp.Header.Get("x-request-id")
	}
	detail := ""
	if s != nil && s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		detail = truncateString(string(payload), maxBytes)
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		ProxyID:              opsUpstreamProxyID(account),
		ProxyName:            opsUpstreamProxyName(account),
		Platform:             account.Platform,
		AccountID:            account.ID,
		AccountName:          account.Name,
		UpstreamStatusCode:   statusCode,
		UpstreamRequestID:    requestID,
		Passthrough:          passthrough,
		Kind:                 "retry",
		Reason:               "compact_model_fallback",
		Message:              sanitizeUpstreamErrorMessage(strings.TrimSpace(message)),
		Detail:               detail,
		UpstreamResponseBody: detail,
	})
}
