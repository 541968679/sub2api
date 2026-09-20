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
		return line
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

// fillEmptyOpenAIResponseModel fills an empty/missing top-level `model` on
// Chat Completions JSON, and `response.model` when a Responses `response`
// object is present. Returns the original payload when nothing should change.
func fillEmptyOpenAIResponseModel(payload []byte, requestedModel string) ([]byte, bool) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" || len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload, false
	}
	updated := payload
	changed := false
	if shouldFillTopLevelOpenAIResponseModel(updated) {
		if next, ok := setOpenAIResponseModelIfEmpty(updated, "model", requestedModel); ok {
			updated = next
			changed = true
		}
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

func shouldFillTopLevelOpenAIResponseModel(payload []byte) bool {
	model := gjson.GetBytes(payload, "model")
	if model.Exists() {
		return openAIResponseModelNeedsFill(model)
	}
	object := gjson.GetBytes(payload, "object").String()
	if strings.HasPrefix(object, "chat.completion") {
		return true
	}
	return gjson.GetBytes(payload, "choices").Exists()
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
