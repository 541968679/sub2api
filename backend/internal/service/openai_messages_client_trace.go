package service

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// anthropicClientSafePrefixes are sentences this gateway wrote. Upstream
// wording is not in the list, so it cannot ride out on /v1/messages.
var anthropicClientSafePrefixes = []string{
	"Prompt is too long",
	"Upstream request failed",
	"Upstream service temporarily unavailable",
	"Upstream service overloaded",
	"Upstream rate limit exceeded",
	"Upstream authentication failed",
	"Upstream access forbidden",
	"Upstream gateway error",
	"Upstream messages",
	"Upstream response reached the maximum output length",
	"Request blocked by upstream cyber-security policy",
	"service_tier=",
}

// anthropicClientMessageIDs keeps one local msg_ id for a single Messages
// response. Tool-call ids stay, because the client sends those back.
type anthropicClientMessageIDs struct {
	local string
}

func newClientAnthropicMessageID() string {
	return "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (m *anthropicClientMessageIDs) id() string {
	if m == nil {
		return newClientAnthropicMessageID()
	}
	if m.local == "" {
		m.local = newClientAnthropicMessageID()
	}
	return m.local
}

// applyBody replaces an Anthropic message id. Nested tool ids are left as-is.
func (m *anthropicClientMessageIDs) applyBody(payload []byte) []byte {
	if m == nil || len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload
	}
	root := gjson.ParseBytes(payload)
	if !root.IsObject() {
		return payload
	}
	switch root.Get("type").String() {
	case "message":
		if root.Get("id").Type != gjson.String {
			return payload
		}
		next, err := sjson.SetBytes(payload, "id", m.id())
		if err != nil {
			return payload
		}
		return next
	case "message_start":
		if root.Get("message.id").Type != gjson.String {
			return payload
		}
		next, err := sjson.SetBytes(payload, "message.id", m.id())
		if err != nil {
			return payload
		}
		return next
	default:
		return payload
	}
}

func (m *anthropicClientMessageIDs) applyLine(line string) string {
	payload, prefix, found := splitClientModelDataLine(line)
	if !found {
		return line
	}
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" || trimmed == "[DONE]" || !strings.HasPrefix(trimmed, "{") {
		return line
	}
	rewritten := m.applyBody([]byte(trimmed))
	if string(rewritten) == trimmed {
		return line
	}
	return prefix + string(rewritten)
}

func (m *anthropicClientMessageIDs) applySSE(sse string) string {
	if m == nil || sse == "" || !strings.Contains(sse, `"id"`) {
		return sse
	}
	lines := strings.Split(sse, "\n")
	changed := false
	for i, line := range lines {
		next := m.applyLine(line)
		if next != line {
			lines[i] = next
			changed = true
		}
	}
	if !changed {
		return sse
	}
	return strings.Join(lines, "\n")
}

func anthropicClientSafeSentence(message string) bool {
	message = strings.TrimSpace(message)
	if message == "" {
		return false
	}
	for _, prefix := range anthropicClientSafePrefixes {
		if strings.HasPrefix(message, prefix) {
			return true
		}
	}
	return false
}

func anthropicUpstreamContextWindow(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	hasContext := strings.Contains(lower, "context window") ||
		strings.Contains(lower, "context length") ||
		strings.Contains(lower, "context_length_exceeded")
	hasOverflow := strings.Contains(lower, "exceed") ||
		strings.Contains(lower, "too long") ||
		strings.Contains(lower, "maximum")
	return hasContext && hasOverflow
}

// PresentAnthropicUpstreamError is the client sentence for /v1/messages.
// Gateway-authored sentences stay. Context-window failures use the fixed 413
// sentence. Every other upstream sentence becomes the same status template
// Chat Completions and Responses already use. Ops records are unchanged.
func PresentAnthropicUpstreamError(c *gin.Context, status int, errType, message string) (int, string, string) {
	message = strings.TrimSpace(message)
	errType = strings.TrimSpace(errType)
	if IsOpenAIWaitTimeoutOpsError(message, "", message) {
		status, errType, message, _ = ApplyClientErrorTemplate(ClientErrorTemplate502Unavailable, ClientErrorProfileOpenAI)
		return status, errType, message
	}
	if anthropicClientSafeSentence(message) {
		message = scrubBridgeClientText(c, message)
		message = ScrubClientErrorDisclosure(message)
		if message == "" {
			message = "Upstream request failed"
		}
		if errType == "" {
			errType = "api_error"
		}
		return status, errType, message
	}
	if anthropicUpstreamContextWindow(message) {
		status, errType, message, _ = ApplyClientErrorTemplate(ClientErrorTemplate413, ClientErrorProfileOpenAI)
		return status, errType, message
	}
	ctx := context.Background()
	if c != nil && c.Request != nil && c.Request.Context() != nil {
		ctx = c.Request.Context()
	}
	return ResolveUpstreamClientError(ctx, ClientErrorProfileOpenAI, "", status, nil)
}
