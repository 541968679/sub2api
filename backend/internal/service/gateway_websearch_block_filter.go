package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"unsafe"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	blockTypeServerToolUse       = "server_tool_use"
	blockTypeWebSearchToolResult = "web_search_tool_result"
)

var (
	patternServerToolUse       = []byte(`"server_tool_use"`)
	patternWebSearchToolResult = []byte(`"web_search_tool_result"`)
)

// FilterWebSearchHistoryBlocks removes replayed web-search blocks that the
// selected upstream did not produce or cannot accept.
func FilterWebSearchHistoryBlocks(body []byte, mappedModel string) []byte {
	if !bytes.Contains(body, patternServerToolUse) && !bytes.Contains(body, patternWebSearchToolResult) {
		return body
	}

	stripAll := requiresWebSearchHistoryStrip(mappedModel)
	jsonStr := *(*string)(unsafe.Pointer(&body))
	msgsRes := gjson.Get(jsonStr, "messages")
	if !msgsRes.Exists() || !msgsRes.IsArray() {
		return body
	}

	var messages []any
	if err := json.Unmarshal(sliceRawFromBody(body, msgsRes), &messages); err != nil {
		return body
	}

	modified := false
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msgMap["content"].([]any)
		if !ok {
			continue
		}

		var filtered []any
		for i, block := range content {
			blockMap, isMap := block.(map[string]any)
			if isMap && shouldStripWebSearchBlock(blockMap, stripAll) {
				if filtered == nil {
					filtered = make([]any, 0, len(content))
					filtered = append(filtered, content[:i]...)
				}
				continue
			}
			if filtered != nil {
				filtered = append(filtered, block)
			}
		}
		if filtered == nil {
			continue
		}
		modified = true
		if len(filtered) == 0 {
			placeholder := "(content removed)"
			if role, _ := msgMap["role"].(string); role == "assistant" {
				placeholder = "(assistant content removed)"
			}
			filtered = []any{map[string]any{"type": "text", "text": placeholder}}
		}
		msgMap["content"] = filtered
	}

	if !modified {
		return body
	}
	msgsBytes, err := json.Marshal(messages)
	if err != nil {
		return body
	}
	out, err := sjson.SetRawBytes(body, "messages", msgsBytes)
	if err != nil {
		return body
	}
	return out
}

func requiresWebSearchHistoryStrip(model string) bool {
	id := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(id, "deepseek-"),
		strings.HasPrefix(id, "kimi-"),
		strings.HasPrefix(id, "moonshot-"),
		strings.HasPrefix(id, "glm-"),
		strings.HasPrefix(id, "minimax-m"):
		return true
	case (strings.HasPrefix(id, "qwen-") ||
		strings.HasPrefix(id, "qwen2-") ||
		strings.HasPrefix(id, "qwen3-") ||
		strings.HasPrefix(id, "qwen4-")) && strings.Contains(id, "-thinking"):
		return true
	default:
		return false
	}
}

func shouldStripWebSearchBlock(block map[string]any, stripAll bool) bool {
	blockType, _ := block["type"].(string)
	switch blockType {
	case blockTypeServerToolUse:
		if stripAll {
			return true
		}
		id, _ := block["id"].(string)
		return strings.HasPrefix(id, webSearchToolUseIDPrefix)
	case blockTypeWebSearchToolResult:
		if stripAll {
			return true
		}
		id, _ := block["tool_use_id"].(string)
		return strings.HasPrefix(id, webSearchToolUseIDPrefix)
	default:
		return false
	}
}
