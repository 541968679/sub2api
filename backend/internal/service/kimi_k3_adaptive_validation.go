package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const kimiK3MaxCompletionTokens int64 = 1048576

const (
	kimiK3ErrMaxCompletion    = "max_completion_tokens exceeds kimi-k3 maximum 1048576"
	kimiK3ErrContextWindow    = "max_completion_tokens 1048576 plus a non-empty prompt exceeds the 1048576 context window"
	kimiK3ErrPredictionType   = "prediction.type must be content"
	kimiK3ErrReasoningEffort  = "reasoning_effort must be low, high, or max"
	kimiK3ErrStopBytes        = "stop string exceeds 32 bytes"
	kimiK3ErrStopCount        = "stop supports at most 5 strings"
	kimiK3ErrStopType         = "stop must be a string or an array of strings"
	kimiK3ErrTopLogprobsRange = "top_logprobs must be an integer from 0 to 20"
	kimiK3ErrTopLogprobsFlag  = "top_logprobs requires logprobs=true"
	kimiK3ErrFunctionName     = "function name must match ^[a-zA-Z_][a-zA-Z0-9-_]{0,127}$"
)

// Official Kimi pattern, including the unescaped hyphen range between '9' and '_'.
var kimiK3FunctionName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9-_]{0,127}$`)

// isKimiK3UpstreamModel is true for kimi-k3 after a case-insensitive trim and
// at most one vendor/ prefix. kimi-k2 and deeper paths do not match.
func (s *OpenAIGatewayService) kimiK3AdaptiveValidationEnabled(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return false
	}
	return s.settingService.IsKimiK3AdaptiveValidationEnabled(ctx)
}

func isKimiK3UpstreamModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if _, rest, ok := strings.Cut(model, "/"); ok {
		model = rest
	}
	return model == "kimi-k3"
}

// AdaptKimiK3ChatBody checks one Chat Completions body against the Kimi K3
// limits. A non-empty rejectMessage is a local HTTP 400 and out is the
// original body. changed means out replaced a string image_url with
// {"url":...}. Absent fields are skipped. Logprob arrays are never invented.
func AdaptKimiK3ChatBody(body []byte) (out []byte, rejectMessage string, changed bool) {
	if msg := rejectKimiK3MaxCompletion(body); msg != "" {
		return body, msg, false
	}
	if msg := rejectKimiK3Prediction(body); msg != "" {
		return body, msg, false
	}
	if msg := rejectKimiK3ReasoningEffort(body); msg != "" {
		return body, msg, false
	}
	if msg := rejectKimiK3Stop(body); msg != "" {
		return body, msg, false
	}
	if msg := rejectKimiK3TopLogprobs(body); msg != "" {
		return body, msg, false
	}
	if msg := rejectKimiK3Tools(body); msg != "" {
		return body, msg, false
	}
	rewritten, did := rewriteKimiK3StringImageURLs(body)
	if !did {
		return body, "", false
	}
	return rewritten, "", true
}

func kimiK3JSONInt(v gjson.Result) (int64, bool) {
	if v.Type != gjson.Number {
		return 0, false
	}
	raw := strings.TrimSpace(v.Raw)
	if raw == "" || strings.ContainsAny(raw, ".eE+") {
		return 0, false
	}
	return v.Int(), raw == fmt.Sprintf("%d", v.Int())
}

func rejectKimiK3MaxCompletion(body []byte) string {
	v := gjson.GetBytes(body, "max_completion_tokens")
	if !v.Exists() {
		return ""
	}
	n, ok := kimiK3JSONInt(v)
	if !ok {
		return ""
	}
	if n > kimiK3MaxCompletionTokens {
		return kimiK3ErrMaxCompletion
	}
	if n == kimiK3MaxCompletionTokens {
		messages := gjson.GetBytes(body, "messages")
		if messages.IsArray() && len(messages.Array()) > 0 {
			return kimiK3ErrContextWindow
		}
	}
	return ""
}

func rejectKimiK3Prediction(body []byte) string {
	v := gjson.GetBytes(body, "prediction")
	if !v.Exists() {
		return ""
	}
	raw := strings.TrimSpace(v.Raw)
	if v.Type != gjson.JSON || !strings.HasPrefix(raw, "{") || v.Get("type").Type != gjson.String || v.Get("type").String() != "content" {
		return kimiK3ErrPredictionType
	}
	return ""
}

func rejectKimiK3ReasoningEffort(body []byte) string {
	v := gjson.GetBytes(body, "reasoning_effort")
	if !v.Exists() {
		return ""
	}
	switch v.String() {
	case "low", "high", "max":
		if v.Type == gjson.String {
			return ""
		}
	}
	return kimiK3ErrReasoningEffort
}

func rejectKimiK3Stop(body []byte) string {
	v := gjson.GetBytes(body, "stop")
	if !v.Exists() {
		return ""
	}
	switch v.Type {
	case gjson.String:
		if len(v.String()) > 32 {
			return kimiK3ErrStopBytes
		}
		return ""
	case gjson.JSON:
		raw := strings.TrimSpace(v.Raw)
		if !strings.HasPrefix(raw, "[") {
			return kimiK3ErrStopType
		}
		items := v.Array()
		if len(items) > 5 {
			return kimiK3ErrStopCount
		}
		for _, item := range items {
			if item.Type != gjson.String {
				return kimiK3ErrStopType
			}
			if len(item.String()) > 32 {
				return kimiK3ErrStopBytes
			}
		}
		return ""
	default:
		return kimiK3ErrStopType
	}
}

func rejectKimiK3TopLogprobs(body []byte) string {
	v := gjson.GetBytes(body, "top_logprobs")
	if !v.Exists() {
		return ""
	}
	n, ok := kimiK3JSONInt(v)
	if !ok || n < 0 || n > 20 {
		return kimiK3ErrTopLogprobsRange
	}
	if gjson.GetBytes(body, "logprobs").Type != gjson.True {
		return kimiK3ErrTopLogprobsFlag
	}
	return ""
}

func rejectKimiK3Tools(body []byte) string {
	tools := gjson.GetBytes(body, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return ""
	}
	msg := ""
	tools.ForEach(func(_, tool gjson.Result) bool {
		raw := strings.TrimSpace(tool.Raw)
		if tool.Type != gjson.JSON || !strings.HasPrefix(raw, "{") {
			return true
		}
		typ := tool.Get("type")
		if typ.Exists() && typ.String() != "function" {
			return true
		}
		fn := tool.Get("function")
		if !fn.Exists() {
			if typ.String() == "function" {
				msg = kimiK3ErrFunctionName
				return false
			}
			return true
		}
		name := fn.Get("name")
		if name.Type != gjson.String || !kimiK3FunctionName.MatchString(name.String()) {
			msg = kimiK3ErrFunctionName
			return false
		}
		return true
	})
	return msg
}

func rewriteKimiK3StringImageURLs(body []byte) ([]byte, bool) {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body, false
	}
	out := body
	changed := false
	failed := false
	messages.ForEach(func(mi, msg gjson.Result) bool {
		if failed {
			return false
		}
		content := msg.Get("content")
		if !content.IsArray() {
			return true
		}
		content.ForEach(func(ci, part gjson.Result) bool {
			if failed {
				return false
			}
			img := part.Get("image_url")
			if img.Type != gjson.String {
				return true
			}
			path := fmt.Sprintf("messages.%d.content.%d.image_url", mi.Int(), ci.Int())
			patched, err := sjson.SetBytes(out, path, map[string]string{"url": img.String()})
			if err != nil {
				failed = true
				return false
			}
			out = patched
			changed = true
			return true
		})
		return !failed
	})
	if failed {
		return body, false
	}
	return out, changed
}
