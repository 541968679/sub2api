package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/tiktoken-go/tokenizer"
)

const (
	kimiK3MaxCompletionTokens int64 = 1048576
	kimiK3DefaultBudget       int64 = 131072
	kimiK3MessageOverhead           = 4
)

const (
	kimiK3ErrMaxCompletion    = "max_completion_tokens exceeds kimi-k3 maximum 1048576"
	kimiK3ErrMaxTokens        = "max_tokens exceeds kimi-k3 maximum 1048576"
	kimiK3ErrPredictionType   = "prediction.type must be content"
	kimiK3ErrReasoningEffort  = "reasoning_effort must be low, high, or max"
	kimiK3ErrStopBytes        = "stop string exceeds 32 bytes"
	kimiK3ErrStopCount        = "stop supports at most 5 strings"
	kimiK3ErrStopType         = "stop must be a string or an array of strings"
	kimiK3ErrTopLogprobsRange = "top_logprobs must be an integer from 0 to 20"
	kimiK3ErrTopLogprobsFlag  = "top_logprobs requires logprobs=true"
	kimiK3ErrFunctionName     = "function name must match ^[a-zA-Z_][a-zA-Z0-9-_]{0,127}$"
	kimiK3ErrEmptyContent     = "content must not be empty"
	kimiK3ErrMessageStructure = "message does not match the kimi-k3 message structure"
	kimiK3ErrSpecifiedTool    = "tool_choice 'specified' is incompatible with thinking enabled"
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
// limits. A non-empty rejectMessage leaves out as the original body.
// budgetReject means the caller returns HTTP 403. Every other reject is HTTP
// 400. changed means out replaced a string image_url with {"url":...}.
// video_url is left unchanged. Logprob arrays are never invented.
func AdaptKimiK3ChatBody(body []byte) (out []byte, rejectMessage string, changed bool, budgetReject bool) {
	budget, msg := resolveKimiK3OutputBudget(body)
	if msg != "" {
		return body, msg, false, false
	}
	if msg := rejectKimiK3Prediction(body); msg != "" {
		return body, msg, false, false
	}
	if msg := rejectKimiK3ReasoningEffort(body); msg != "" {
		return body, msg, false, false
	}
	if msg := rejectKimiK3Stop(body); msg != "" {
		return body, msg, false, false
	}
	if msg := rejectKimiK3TopLogprobs(body); msg != "" {
		return body, msg, false, false
	}
	if msg := rejectKimiK3Tools(body); msg != "" {
		return body, msg, false, false
	}
	if msg := rejectKimiK3Messages(body); msg != "" {
		return body, msg, false, false
	}
	if msg := rejectKimiK3SpecifiedToolChoice(body); msg != "" {
		return body, msg, false, false
	}
	if msg, over := rejectKimiK3InputBudget(body, budget); over {
		return body, msg, false, true
	}
	rewritten, did := rewriteKimiK3StringImageURLs(body)
	if !did {
		return body, "", false, false
	}
	return rewritten, "", true, false
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

func resolveKimiK3OutputBudget(body []byte) (int64, string) {
	completion, msg, hasCompletion := kimiK3LimitField(body, "max_completion_tokens", kimiK3ErrMaxCompletion)
	if msg != "" {
		return 0, msg
	}
	legacy, msg, hasLegacy := kimiK3LimitField(body, "max_tokens", kimiK3ErrMaxTokens)
	if msg != "" {
		return 0, msg
	}
	switch {
	case hasCompletion:
		return completion, ""
	case hasLegacy:
		return legacy, ""
	default:
		return kimiK3DefaultBudget, ""
	}
}

func kimiK3LimitField(body []byte, field, exceedMessage string) (int64, string, bool) {
	v := gjson.GetBytes(body, field)
	if !v.Exists() {
		return 0, "", false
	}
	n, ok := kimiK3JSONInt(v)
	if !ok || n < 0 {
		return 0, fmt.Sprintf("%s must be an integer from 0 to 1048576", field), false
	}
	if n > kimiK3MaxCompletionTokens {
		return 0, exceedMessage, false
	}
	return n, "", true
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

func rejectKimiK3Messages(body []byte) string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() {
		return ""
	}
	if !messages.IsArray() {
		return kimiK3ErrMessageStructure
	}
	msg := ""
	messages.ForEach(func(_, message gjson.Result) bool {
		if errMsg := rejectKimiK3Message(message); errMsg != "" {
			msg = errMsg
			return false
		}
		return true
	})
	return msg
}

func rejectKimiK3Message(message gjson.Result) string {
	raw := strings.TrimSpace(message.Raw)
	if message.Type != gjson.JSON || !strings.HasPrefix(raw, "{") {
		return kimiK3ErrMessageStructure
	}
	if message.Get("tools").Exists() {
		return rejectKimiK3DynamicToolMessage(message)
	}
	return rejectKimiK3StandardMessage(message)
}

func rejectKimiK3DynamicToolMessage(message gjson.Result) string {
	if message.Get("role").String() != "system" {
		return kimiK3ErrMessageStructure
	}
	tools := message.Get("tools")
	if !tools.IsArray() {
		return kimiK3ErrMessageStructure
	}
	extra := false
	message.ForEach(func(key, _ gjson.Result) bool {
		switch key.String() {
		case "role", "tools":
			return true
		default:
			extra = true
			return false
		}
	})
	if extra {
		return kimiK3ErrMessageStructure
	}
	msg := ""
	tools.ForEach(func(_, tool gjson.Result) bool {
		if errMsg := rejectKimiK3ToolDefinition(tool); errMsg != "" {
			msg = errMsg
			return false
		}
		return true
	})
	return msg
}

func rejectKimiK3ToolDefinition(tool gjson.Result) string {
	raw := strings.TrimSpace(tool.Raw)
	if tool.Type != gjson.JSON || !strings.HasPrefix(raw, "{") {
		return kimiK3ErrMessageStructure
	}
	if tool.Get("type").String() != "function" {
		return kimiK3ErrMessageStructure
	}
	fn := tool.Get("function")
	if !fn.Exists() || !strings.HasPrefix(strings.TrimSpace(fn.Raw), "{") {
		return kimiK3ErrMessageStructure
	}
	name := fn.Get("name")
	if name.Type != gjson.String || !kimiK3FunctionName.MatchString(name.String()) {
		return kimiK3ErrFunctionName
	}
	if !fn.Get("parameters").Exists() {
		return kimiK3ErrMessageStructure
	}
	return ""
}

func rejectKimiK3StandardMessage(message gjson.Result) string {
	role := message.Get("role")
	switch role.String() {
	case "system", "user", "assistant", "tool":
		if role.Type != gjson.String {
			return kimiK3ErrMessageStructure
		}
	default:
		return kimiK3ErrMessageStructure
	}
	if msg := rejectKimiK3Content(message.Get("content")); msg != "" {
		return msg
	}
	if role.String() == "tool" {
		callID := message.Get("tool_call_id")
		if !callID.Exists() || callID.Type != gjson.String {
			return kimiK3ErrMessageStructure
		}
	}
	if name := message.Get("name"); name.Exists() && name.Type != gjson.String {
		return kimiK3ErrMessageStructure
	}
	if partial := message.Get("partial"); partial.Exists() && partial.Type != gjson.True && partial.Type != gjson.False {
		return kimiK3ErrMessageStructure
	}
	return ""
}

func rejectKimiK3Content(content gjson.Result) string {
	if !content.Exists() {
		return kimiK3ErrMessageStructure
	}
	switch content.Type {
	case gjson.String:
		if content.String() == "" {
			return kimiK3ErrEmptyContent
		}
		return ""
	case gjson.JSON:
		raw := strings.TrimSpace(content.Raw)
		if !strings.HasPrefix(raw, "[") {
			return kimiK3ErrMessageStructure
		}
		parts := content.Array()
		if len(parts) == 0 {
			return kimiK3ErrEmptyContent
		}
		for _, part := range parts {
			if msg := rejectKimiK3ContentPart(part); msg != "" {
				return msg
			}
		}
		return ""
	default:
		return kimiK3ErrMessageStructure
	}
}

func rejectKimiK3ContentPart(part gjson.Result) string {
	raw := strings.TrimSpace(part.Raw)
	if part.Type != gjson.JSON || !strings.HasPrefix(raw, "{") {
		return kimiK3ErrMessageStructure
	}
	switch part.Get("type").String() {
	case "text":
		if part.Get("text").Type != gjson.String {
			return kimiK3ErrMessageStructure
		}
	case "image_url", "video_url":
		media := part.Get(part.Get("type").String())
		if media.Type == gjson.String {
			return ""
		}
		if media.Type == gjson.JSON && strings.HasPrefix(strings.TrimSpace(media.Raw), "{") && media.Get("url").Type == gjson.String {
			return ""
		}
		return kimiK3ErrMessageStructure
	default:
		return kimiK3ErrMessageStructure
	}
	return ""
}

type kimiK3TokenInputs struct {
	texts    []string
	bytes    int
	media    int
	messages int
}

func rejectKimiK3InputBudget(body []byte, budget int64) (string, bool) {
	inputs := collectKimiK3TokenInputs(body)
	overhead := inputs.media + inputs.messages*kimiK3MessageOverhead
	if int64(inputs.bytes+overhead)+budget <= kimiK3MaxCompletionTokens {
		return "", false
	}
	input := overhead
	if inputs.bytes > openAIInputTokensEstimateMaxBytes {
		approx := inputs.bytes / 4
		if approx < 1 {
			approx = 1
		}
		input += approx
	} else if len(inputs.texts) > 0 {
		codec, err := tokenizer.Get(tokenizer.O200kBase)
		if err != nil {
			return kimiK3BudgetMessage(inputs.bytes+overhead, budget), true
		}
		for _, text := range inputs.texts {
			n, countErr := codec.Count(strings.TrimSpace(text))
			if countErr != nil {
				return kimiK3BudgetMessage(inputs.bytes+overhead, budget), true
			}
			input += n
		}
	}
	if int64(input) > kimiK3MaxCompletionTokens || int64(input)+budget > kimiK3MaxCompletionTokens {
		return kimiK3BudgetMessage(input, budget), true
	}
	return "", false
}

func kimiK3BudgetMessage(input int, budget int64) string {
	return fmt.Sprintf("input tokens %d plus output budget %d exceed the 1048576 context window", input, budget)
}

func collectKimiK3TokenInputs(body []byte) kimiK3TokenInputs {
	var inputs kimiK3TokenInputs
	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		messages.ForEach(func(_, message gjson.Result) bool {
			inputs.messages++
			collectKimiK3MessageTokens(&inputs, message)
			return true
		})
	}
	tools := gjson.GetBytes(body, "tools")
	if tools.IsArray() {
		tools.ForEach(func(_, tool gjson.Result) bool {
			collectKimiK3ToolTokens(&inputs, tool)
			return true
		})
	}
	return inputs
}

func collectKimiK3MessageTokens(inputs *kimiK3TokenInputs, message gjson.Result) {
	if message.Get("tools").Exists() && !message.Get("content").Exists() {
		tools := message.Get("tools")
		if tools.IsArray() {
			tools.ForEach(func(_, tool gjson.Result) bool {
				collectKimiK3ToolTokens(inputs, tool)
				return true
			})
		}
		return
	}
	addKimiK3Text(inputs, message.Get("name"))
	addKimiK3Text(inputs, message.Get("tool_call_id"))
	content := message.Get("content")
	switch content.Type {
	case gjson.String:
		addKimiK3Text(inputs, content)
	case gjson.JSON:
		if !strings.HasPrefix(strings.TrimSpace(content.Raw), "[") {
			return
		}
		content.ForEach(func(_, part gjson.Result) bool {
			switch part.Get("type").String() {
			case "text":
				addKimiK3Text(inputs, part.Get("text"))
			case "image_url", "video_url":
				inputs.media++
			}
			return true
		})
	}
}

func collectKimiK3ToolTokens(inputs *kimiK3TokenInputs, tool gjson.Result) {
	fn := tool.Get("function")
	addKimiK3Text(inputs, fn.Get("name"))
	addKimiK3Text(inputs, fn.Get("description"))
	if params := fn.Get("parameters"); params.Exists() {
		inputs.texts = append(inputs.texts, params.Raw)
		inputs.bytes += len(params.Raw)
	}
}

func addKimiK3Text(inputs *kimiK3TokenInputs, value gjson.Result) {
	if value.Type != gjson.String {
		return
	}
	text := value.String()
	inputs.texts = append(inputs.texts, text)
	inputs.bytes += len(text)
}

func rejectKimiK3SpecifiedToolChoice(body []byte) string {
	choice := gjson.GetBytes(body, "tool_choice")
	if !choice.Exists() {
		return ""
	}
	raw := strings.TrimSpace(choice.Raw)
	if choice.Type == gjson.JSON && strings.HasPrefix(raw, "{") {
		return kimiK3ErrSpecifiedTool
	}
	return ""
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
