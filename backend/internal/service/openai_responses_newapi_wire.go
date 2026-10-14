package service

import (
	"bytes"
	"io"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Older QuantumNous new-api builds unmarshal Responses created_at into a Go
// int. A JSON number such as 1785990796.0 fails that unmarshal, the
// response.completed frame is dropped, and the usage log falls back to a
// 1-token completion. The same builds record end_reason=eof unless they see
// a data: [DONE] line. Integer timestamps and streams that already contain
// [DONE] are left unchanged. A stream with no terminal event must not gain a
// synthetic [DONE]; that would hide a truncated upstream.

func jsonNumberIsWholeInteger(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return true
	}
	return !strings.ContainsAny(raw, ".eE")
}

func coerceOpenAIResponsesCreatedAt(body []byte, path string) ([]byte, bool) {
	node := gjson.GetBytes(body, path)
	if node.Type != gjson.Number || jsonNumberIsWholeInteger(node.Raw) {
		return body, false
	}
	next, err := sjson.SetBytes(body, path, node.Int())
	if err != nil || bytes.Equal(next, body) {
		return body, false
	}
	return next, true
}

// normalizeOpenAIResponsesNewAPIWireData coerces non-integer created_at
// values to int64. Every other field, including usage, is preserved.
func normalizeOpenAIResponsesNewAPIWireData(body []byte) ([]byte, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, false
	}
	changed := false
	for _, path := range []string{"created_at", "response.created_at"} {
		next, ok := coerceOpenAIResponsesCreatedAt(body, path)
		if !ok {
			continue
		}
		body = next
		changed = true
	}
	return body, changed
}

func normalizeOpenAIResponsesNewAPIWireLine(line string) string {
	if !strings.Contains(line, "created_at") {
		return line
	}
	data, ok := extractOpenAISSEDataLine(line)
	if !ok {
		return line
	}
	trimmed := strings.TrimSpace(data)
	if trimmed == "" || trimmed == "[DONE]" {
		return line
	}
	next, changed := normalizeOpenAIResponsesNewAPIWireData([]byte(data))
	if !changed {
		return line
	}
	prefixLen := len(line) - len(data)
	if prefixLen >= 0 && line[prefixLen:] == data {
		return line[:prefixLen] + string(next)
	}
	return "data: " + string(next)
}

func normalizeOpenAIResponsesNewAPIWireSSEText(body string) string {
	if body == "" || !strings.Contains(body, "created_at") {
		return body
	}
	lines := strings.Split(body, "\n")
	changed := false
	for i, line := range lines {
		next := normalizeOpenAIResponsesNewAPIWireLine(line)
		if next != line {
			lines[i] = next
			changed = true
		}
	}
	if !changed {
		return body
	}
	return strings.Join(lines, "\n")
}

// applyOpenAIResponsesNewAPIWireBody rewrites a Responses JSON object or an
// SSE body. Chat Completions callers must not use it.
func applyOpenAIResponsesNewAPIWireBody(body []byte) []byte {
	if len(body) == 0 || !bytes.Contains(body, []byte("created_at")) {
		return body
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		next, changed := normalizeOpenAIResponsesNewAPIWireData(trimmed)
		if !changed {
			return body
		}
		return next
	}
	text := string(body)
	next := normalizeOpenAIResponsesNewAPIWireSSEText(text)
	if next == text {
		return body
	}
	return []byte(next)
}

func writeOpenAIResponsesStreamDone(w io.Writer) error {
	if w == nil {
		return nil
	}
	_, err := io.WriteString(w, "data: [DONE]\n\n")
	return err
}
