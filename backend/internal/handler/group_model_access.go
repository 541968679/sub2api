package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/tidwall/gjson"
)

const groupModelAccessDeniedMessage = "model is not allowed for this group"

func isGroupModelAllowed(group *service.Group, model string) bool {
	return group == nil || group.IsModelAllowed(model)
}

func disallowedResponsesImageToolModel(group *service.Group, body []byte) string {
	if group == nil || len(body) == 0 {
		return ""
	}
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return ""
	}
	for _, tool := range tools.Array() {
		if tool.Get("type").String() != "image_generation" {
			continue
		}
		model := strings.TrimSpace(tool.Get("model").String())
		if model != "" && !group.IsModelAllowed(model) {
			return model
		}
	}
	return ""
}
