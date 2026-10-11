package service

import (
	"bytes"
	"strings"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// chatCompletionIDMask replaces upstream Chat Completions ids with one
// gateway id for a single response. Tool-call ids and other nested ids stay,
// because clients echo those on the next turn.
type chatCompletionIDMask struct {
	local string
}

func newClientChatCompletionID() string {
	return "chatcmpl-" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func (m *chatCompletionIDMask) localID() string {
	if m.local == "" {
		m.local = newClientChatCompletionID()
	}
	return m.local
}

// applyBytes rewrites a top-level completion id. Payloads that are not a
// chat completion, and empty ids, are returned unchanged.
func (m *chatCompletionIDMask) applyBytes(payload []byte) []byte {
	if !shouldMaskChatCompletionID(payload) {
		return payload
	}
	next, err := sjson.SetBytes(payload, "id", m.localID())
	if err != nil {
		return payload
	}
	return next
}

// applyBody masks one JSON completion or each SSE / NDJSON line in a buffer.
func (m *chatCompletionIDMask) applyBody(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	if gjson.ValidBytes(body) && gjson.ParseBytes(body).IsObject() {
		return m.applyBytes(body)
	}
	if !bytes.Contains(body, []byte(`"id"`)) {
		return body
	}
	lines := bytes.Split(body, []byte("\n"))
	changed := false
	for i, line := range lines {
		next := []byte(m.applyLine(string(line)))
		if !bytes.Equal(next, line) {
			lines[i] = next
			changed = true
		}
	}
	if !changed {
		return body
	}
	return bytes.Join(lines, []byte("\n"))
}

func (m *chatCompletionIDMask) applyLine(line string) string {
	payload, ok := extractOpenAISSEDataLine(line)
	if !ok {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{") {
			return line
		}
		rewritten := m.applyBytes([]byte(trimmed))
		if string(rewritten) == trimmed {
			return line
		}
		return strings.Replace(line, trimmed, string(rewritten), 1)
	}
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" || trimmed == "[DONE]" {
		return line
	}
	rewritten := m.applyBytes([]byte(trimmed))
	if string(rewritten) == trimmed {
		return line
	}
	prefixLen := len(line) - len(payload)
	if prefixLen < 0 {
		return line
	}
	return line[:prefixLen] + string(rewritten)
}

func shouldMaskChatCompletionID(payload []byte) bool {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return false
	}
	id := gjson.GetBytes(payload, "id")
	if !id.Exists() || id.Type != gjson.String || strings.TrimSpace(id.Str) == "" {
		return false
	}
	object := gjson.GetBytes(payload, "object").String()
	if object == "chat.completion" || object == "chat.completion.chunk" {
		return true
	}
	return gjson.GetBytes(payload, "choices").Exists()
}
