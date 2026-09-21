package service

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// fillEmptyOpenAIResponseModelInSSELine copies the requested model onto an
// OpenAI-compatible SSE data line when the upstream omitted or left `model`
// empty. Non-empty upstream model names are left unchanged.
func fillEmptyOpenAIResponseModelInSSELine(line, requestedModel string) string {
	payload, ok := extractOpenAISSEDataLine(line)
	if !ok {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{") {
			return line
		}
		rewritten, changed := fillEmptyOpenAIResponseModel([]byte(trimmed), requestedModel)
		if !changed {
			return line
		}
		return strings.Replace(line, trimmed, string(rewritten), 1)
	}
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" || trimmed == "[DONE]" {
		return line
	}
	rewritten, changed := fillEmptyOpenAIResponseModel([]byte(payload), requestedModel)
	if !changed {
		return line
	}
	prefixLen := len(line) - len(payload)
	if prefixLen < 0 {
		return line
	}
	return line[:prefixLen] + string(rewritten)
}

func fillEmptyOpenAIResponseModelInSSEBody(body, requestedModel string) string {
	if strings.TrimSpace(requestedModel) == "" || body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	changed := false
	for i, line := range lines {
		next := fillEmptyOpenAIResponseModelInSSELine(line, requestedModel)
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

func applyClientFacingOpenAIResponseModel(body []byte, requestedModel string) []byte {
	if filled, ok := fillEmptyOpenAIResponseModel(body, requestedModel); ok {
		return filled
	}
	return body
}

// fillEmptyOpenAIResponseModel fills an empty/missing top-level `model` on
// OpenAI-compatible JSON, and `response.model` when a Responses `response`
// object is present. Missing `model` unmarshals to "" in Go clients, so it
// must be filled the same way as an explicit empty string. Non-empty names
// are left unchanged. `"error": null` is a normal Responses/CC field and is
// not treated as an error payload. Dedicated `type=error` / error-object
// payloads are skipped. Ping frames still get a model because clients that
// unmarshal every JSON object treat a missing field as "".
func fillEmptyOpenAIResponseModel(payload []byte, requestedModel string) ([]byte, bool) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" || len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload, false
	}
	root := gjson.ParseBytes(payload)
	if !root.IsObject() || skipOpenAIResponseModelFill(payload) {
		return payload, false
	}
	updated := payload
	changed := false
	if next, ok := setOpenAIResponseModelIfEmpty(updated, "model", requestedModel); ok {
		updated = next
		changed = true
	}
	if gjson.GetBytes(updated, "response").IsObject() {
		if next, ok := setOpenAIResponseModelIfEmpty(updated, "response.model", requestedModel); ok {
			updated = next
			changed = true
		}
	}
	if !changed {
		return payload, false
	}
	return updated, true
}

func skipOpenAIResponseModelFill(payload []byte) bool {
	typ := strings.TrimSpace(gjson.GetBytes(payload, "type").String())
	if typ == "error" {
		return true
	}
	errVal := gjson.GetBytes(payload, "error")
	if !errVal.Exists() || errVal.Type == gjson.Null || !errVal.IsObject() {
		return false
	}
	if gjson.GetBytes(payload, "choices").Exists() || gjson.GetBytes(payload, "response").IsObject() {
		return false
	}
	object := gjson.GetBytes(payload, "object").String()
	return !strings.HasPrefix(object, "chat.completion")
}

func setOpenAIResponseModelIfEmpty(payload []byte, path, requestedModel string) ([]byte, bool) {
	if !openAIResponseModelNeedsFill(gjson.GetBytes(payload, path)) {
		return payload, false
	}
	next, err := sjson.SetBytes(payload, path, requestedModel)
	if err != nil {
		return payload, false
	}
	return next, true
}

func openAIResponseModelNeedsFill(value gjson.Result) bool {
	if !value.Exists() || value.Type == gjson.Null {
		return true
	}
	if value.Type == gjson.String {
		return strings.TrimSpace(value.Str) == ""
	}
	return false
}
