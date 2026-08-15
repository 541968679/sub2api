package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

const (
	openAICodexCompactV2FallbackKey = "openai_codex_compact_v2_fallback"
	codexCompactV2SummaryTag        = "conversation_summary"
	codexCompactV2EncryptedPrefix   = "sub2api_compact_v2:"
	codexCompactV2MaxBufferBytes    = 8 << 20
)

// Neutral summary instruction for third-party Chat/Responses upstreams that
// do not understand compaction_trigger. Keep vendor-neutral; do not mention
// Codex or Sub2API internals.
const codexCompactV2SummaryPrompt = "Summarize the conversation so far into a compact continuation brief. Preserve decisions, file paths, constraints, and unfinished work. Do not greet the user or ask a question. Reply with the summary only."

type compactV2Class int

const (
	compactV2FailOpen compactV2Class = iota
	compactV2Passthrough
	compactV2Synthesize
	compactV2Empty
)

type compactV2Prep struct {
	Body               []byte
	SynthesizeResponse bool
	Applied            bool
}

func markCodexCompactV2Fallback(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(openAICodexCompactV2FallbackKey, true)
}

func isCodexCompactV2FallbackMarked(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, ok := c.Get(openAICodexCompactV2FallbackKey)
	if !ok {
		return false
	}
	enabled, ok := value.(bool)
	return ok && enabled
}

func isOpenAICompactionItemType(itemType string) bool {
	switch strings.ToLower(strings.TrimSpace(itemType)) {
	case "compaction", "compaction_summary", "context_compaction":
		return true
	default:
		return false
	}
}

func accountAllowsCodexCompactV2Fallback(account *Account) bool {
	if account == nil {
		return false
	}
	if account.Type != AccountTypeAPIKey {
		return false
	}
	if account.Platform == PlatformGrok {
		return false
	}
	return true
}

func (s *OpenAIGatewayService) isCodexCompactV2FallbackEnabled(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return true
	}
	return s.settingService.IsCodexCompactV2FallbackEnabled(ctx)
}

func (s *OpenAIGatewayService) prepareCodexCompactV2Fallback(ctx context.Context, c *gin.Context, account *Account, body []byte) compactV2Prep {
	prep := compactV2Prep{Body: body}
	if s == nil || !s.isCodexCompactV2FallbackEnabled(ctx) {
		return prep
	}
	if !accountAllowsCodexCompactV2Fallback(account) {
		return prep
	}
	if isOpenAICompatMessagesBridgeContext(c) {
		return prep
	}
	hasTrigger, hasHistory := hasCodexCompactV2InputSignal(body)
	if !hasTrigger && !hasHistory {
		return prep
	}
	rewritten, changed, err := rewriteCodexCompactV2Request(body)
	if err != nil {
		logger.L().Warn("codex.compact_v2.request_rewrite_failed", zap.Error(err))
		return prep
	}
	if changed {
		prep.Body = rewritten
		prep.Applied = true
	}
	if hasTrigger {
		prep.SynthesizeResponse = true
		markCodexCompactV2Fallback(c)
	}
	return prep
}

func hasCodexCompactV2InputSignal(body []byte) (hasTrigger bool, hasHistory bool) {
	if len(body) == 0 {
		return false, false
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return false, false
	}
	input.ForEach(func(_, item gjson.Result) bool {
		typ := item.Get("type").String()
		if typ == "compaction_trigger" {
			hasTrigger = true
			return true
		}
		if isOpenAICompactionItemType(typ) {
			hasHistory = true
		}
		return true
	})
	return hasTrigger, hasHistory
}

func compactSummaryText(summary gjson.Result) string {
	if !summary.Exists() {
		return ""
	}
	if summary.Type == gjson.String {
		return strings.TrimSpace(summary.String())
	}
	if !summary.IsArray() {
		return ""
	}
	parts := make([]string, 0, len(summary.Array()))
	for _, item := range summary.Array() {
		if text := strings.TrimSpace(item.Get("text").String()); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// compactRecoverableText prefers visible summary_text, then our minted
// encrypted_content payload. Codex's ResponseItem::Compaction only keeps
// encrypted_content on the next turn, so the prefix is how API-key fallback
// avoids silent amnesia. Official / third-party blobs without the prefix are
// left alone.
func compactRecoverableText(item gjson.Result) string {
	if summary := compactSummaryText(item.Get("summary")); summary != "" {
		return summary
	}
	return decodeCodexCompactV2EncryptedContent(item.Get("encrypted_content").String())
}

func encodeCodexCompactV2EncryptedContent(summary string) string {
	return codexCompactV2EncryptedPrefix + strings.TrimSpace(summary)
}

func decodeCodexCompactV2EncryptedContent(blob string) string {
	blob = strings.TrimSpace(blob)
	if !strings.HasPrefix(blob, codexCompactV2EncryptedPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(blob, codexCompactV2EncryptedPrefix))
}

func codexCompactV2UserMessage(text string) map[string]any {
	return map[string]any{
		"type": "message",
		"role": "user",
		"content": []any{map[string]any{
			"type": "input_text",
			"text": text,
		}},
	}
}

func rewriteCodexCompactV2Request(body []byte) ([]byte, bool, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, false, nil
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return body, false, nil
	}

	converted := make([]any, 0, len(input.Array())+1)
	changed := false
	input.ForEach(func(_, raw gjson.Result) bool {
		if !raw.IsObject() {
			var value any
			if err := json.Unmarshal([]byte(raw.Raw), &value); err == nil {
				converted = append(converted, value)
			}
			return true
		}
		itemType := strings.TrimSpace(raw.Get("type").String())
		switch {
		case itemType == "compaction_trigger":
			changed = true
			converted = append(converted, codexCompactV2UserMessage(codexCompactV2SummaryPrompt))
		case isOpenAICompactionItemType(itemType):
			if summary := compactRecoverableText(raw); summary != "" {
				changed = true
				converted = append(converted, codexCompactV2UserMessage(
					"<"+codexCompactV2SummaryTag+">\n"+summary+"\n</"+codexCompactV2SummaryTag+">",
				))
				return true
			}
			var value any
			if err := json.Unmarshal([]byte(raw.Raw), &value); err == nil {
				converted = append(converted, value)
			}
		default:
			var value any
			if err := json.Unmarshal([]byte(raw.Raw), &value); err == nil {
				converted = append(converted, value)
			}
		}
		return true
	})
	if !changed {
		return body, false, nil
	}

	encoded, err := json.Marshal(converted)
	if err != nil {
		return nil, false, fmt.Errorf("encode compact v2 input: %w", err)
	}
	out, err := sjson.SetRawBytes(body, "input", encoded)
	if err != nil {
		return nil, false, fmt.Errorf("set compact v2 input: %w", err)
	}
	if tools := gjson.GetBytes(out, "tools"); tools.IsArray() && len(tools.Array()) > 0 {
		next, setErr := sjson.SetBytes(out, "tool_choice", "none")
		if setErr != nil {
			return nil, false, setErr
		}
		out = next
	}
	return out, true, nil
}

func chatMessagePlainTextForCompact(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &parts); err != nil {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part.Text); trimmed != "" {
			texts = append(texts, trimmed)
		}
	}
	return strings.TrimSpace(strings.Join(texts, "\n"))
}

func buildCodexCompactV2Responses(summary, model, responseID string, usage *apicompat.ResponsesUsage) (*apicompat.ResponsesResponse, error) {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return nil, fmt.Errorf("compact response carries no summary text")
	}
	id := strings.TrimSpace(responseID)
	if id == "" {
		id = "resp_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	out := &apicompat.ResponsesResponse{
		ID:     id,
		Object: "response",
		Model:  model,
		Status: "completed",
		Output: []apicompat.ResponsesOutput{{
			Type:             "compaction",
			ID:               "cmp_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			Status:           "completed",
			EncryptedContent: encodeCodexCompactV2EncryptedContent(summary),
			Summary: []apicompat.ResponsesSummary{{
				Type: "summary_text",
				Text: summary,
			}},
		}},
		Usage: usage,
	}
	return out, nil
}

func buildCodexCompactV2FromChat(resp *apicompat.ChatCompletionsResponse, model string) (*apicompat.ResponsesResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("compact response is nil")
	}
	summary := ""
	if len(resp.Choices) > 0 {
		message := resp.Choices[0].Message
		summary = chatMessagePlainTextForCompact(message.Content)
		if summary == "" {
			summary = strings.TrimSpace(message.ReasoningContent)
		}
	}
	var usage *apicompat.ResponsesUsage
	if resp.Usage != nil {
		usage = apicompat.ChatUsageToResponsesUsage(resp.Usage)
	}
	return buildCodexCompactV2Responses(summary, model, resp.ID, usage)
}

func buildCodexCompactV2SSE(finalResponse []byte) ([]byte, bool) {
	if len(finalResponse) == 0 || !gjson.ValidBytes(finalResponse) || !gjson.ParseBytes(finalResponse).IsObject() {
		return nil, false
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, finalResponse); err != nil {
		return nil, false
	}
	response := compacted.Bytes()
	if strings.TrimSpace(gjson.GetBytes(response, "id").String()) == "" {
		next, err := sjson.SetBytes(response, "id", "resp_"+strings.ReplaceAll(uuid.NewString(), "-", ""))
		if err != nil {
			return nil, false
		}
		response = next
	}
	if usage := gjson.GetBytes(response, "usage"); usage.Exists() && !codexCompactV2UsageParsable(usage) {
		next, err := sjson.DeleteBytes(response, "usage")
		if err != nil {
			return nil, false
		}
		response = next
	}

	var buf bytes.Buffer
	outputIndex := 0
	appendEvent := func(eventType string, data []byte) {
		_, _ = buf.WriteString("event: ")
		_, _ = buf.WriteString(eventType)
		_, _ = buf.WriteString("\ndata: ")
		_, _ = buf.Write(data)
		_, _ = buf.WriteString("\n\n")
	}
	for _, item := range gjson.GetBytes(response, "output").Array() {
		if !item.IsObject() {
			continue
		}
		event, err := sjson.SetBytes([]byte(`{"type":"response.output_item.done"}`), "output_index", outputIndex)
		if err != nil {
			return nil, false
		}
		event, err = sjson.SetRawBytes(event, "item", []byte(item.Raw))
		if err != nil {
			return nil, false
		}
		appendEvent("response.output_item.done", event)
		outputIndex++
	}
	completed, err := sjson.SetRawBytes([]byte(`{"type":"response.completed"}`), "response", response)
	if err != nil {
		return nil, false
	}
	appendEvent("response.completed", completed)
	return buf.Bytes(), true
}

func codexCompactV2UsageParsable(usage gjson.Result) bool {
	if !usage.IsObject() {
		return false
	}
	for _, field := range []string{"input_tokens", "output_tokens", "total_tokens"} {
		if usage.Get(field).Type != gjson.Number {
			return false
		}
	}
	return true
}

func extractTextFromResponsesItem(item gjson.Result) string {
	if text := compactSummaryText(item.Get("summary")); text != "" {
		return text
	}
	if text := strings.TrimSpace(item.Get("content").String()); text != "" && item.Get("content").Type == gjson.String {
		return text
	}
	if item.Get("content").IsArray() {
		parts := make([]string, 0)
		for _, part := range item.Get("content").Array() {
			if text := strings.TrimSpace(part.Get("text").String()); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return ""
}

func classifyCodexCompactV2JSON(body []byte) (compactV2Class, string) {
	if len(body) == 0 || !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return compactV2FailOpen, ""
	}
	output := gjson.GetBytes(body, "output")
	if !output.Exists() {
		return compactV2FailOpen, ""
	}
	if !output.IsArray() {
		return compactV2FailOpen, ""
	}
	summary := ""
	for _, item := range output.Array() {
		if isOpenAICompactionItemType(item.Get("type").String()) {
			return compactV2Passthrough, ""
		}
		if text := extractTextFromResponsesItem(item); text != "" {
			if summary != "" {
				summary += "\n"
			}
			summary += text
		}
	}
	if strings.TrimSpace(summary) == "" {
		return compactV2Empty, ""
	}
	return compactV2Synthesize, strings.TrimSpace(summary)
}

func classifyCodexCompactV2SSE(buf []byte) (compactV2Class, string, string) {
	if len(buf) == 0 {
		return compactV2FailOpen, "", ""
	}
	sawEvent := false
	summary := ""
	responseID := ""
	for _, line := range bytes.Split(buf, []byte("\n")) {
		payload, ok := extractOpenAISSEDataLine(string(line))
		if !ok {
			continue
		}
		payload = strings.TrimSpace(payload)
		if payload == "" || payload == "[DONE]" {
			continue
		}
		if !gjson.Valid(payload) {
			continue
		}
		sawEvent = true
		root := gjson.Parse(payload)
		eventType := root.Get("type").String()
		if id := strings.TrimSpace(root.Get("response.id").String()); id != "" && responseID == "" {
			responseID = id
		}
		if id := strings.TrimSpace(root.Get("id").String()); id != "" && responseID == "" && strings.HasPrefix(eventType, "response.") {
			responseID = id
		}
		switch eventType {
		case "response.output_item.done":
			item := root.Get("item")
			if isOpenAICompactionItemType(item.Get("type").String()) {
				return compactV2Passthrough, "", responseID
			}
			if text := extractTextFromResponsesItem(item); text != "" {
				if summary != "" {
					summary += "\n"
				}
				summary += text
			}
		case "response.completed":
			class, completedSummary := classifyCodexCompactV2JSON([]byte(root.Get("response").Raw))
			if class == compactV2Passthrough {
				return compactV2Passthrough, "", responseID
			}
			if completedSummary != "" {
				summary = completedSummary
			}
			if id := strings.TrimSpace(root.Get("response.id").String()); id != "" {
				responseID = id
			}
		}
	}
	if !sawEvent {
		return compactV2FailOpen, "", responseID
	}
	if strings.TrimSpace(summary) == "" {
		return compactV2Empty, "", responseID
	}
	return compactV2Synthesize, strings.TrimSpace(summary), responseID
}

func logCodexCompactV2Outcome(account *Account, via, outcome string) {
	fields := []zap.Field{
		zap.String("component", "openai.codex_compact_v2"),
		zap.String("via", via),
		zap.String("compact_v2_outcome", outcome),
	}
	if account != nil {
		fields = append(fields,
			zap.Int64("account_id", account.ID),
			zap.String("account_type", string(account.Type)),
			zap.String("platform", string(account.Platform)),
		)
	}
	logger.L().Info("codex.compact_v2."+outcome, fields...)
}

func writeCodexCompactV2SSE(c *gin.Context, statusCode int, payload []byte) {
	if c == nil {
		return
	}
	header := c.Writer.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(statusCode)
	_, _ = c.Writer.Write(payload)
	c.Writer.Flush()
}

func writeCodexCompactV2Failure(c *gin.Context, message string) {
	if c == nil {
		return
	}
	if strings.TrimSpace(message) == "" {
		message = "Upstream compact request produced no summary"
	}
	payload, err := json.Marshal(map[string]any{
		"type": "response.failed",
		"response": map[string]any{
			"id":     "resp_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
			"object": "response",
			"status": "failed",
			"output": []any{},
			"error": map[string]any{
				"code":    "compact_v2_empty_summary",
				"message": message,
			},
		},
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{"type": "api_error", "message": message},
		})
		return
	}
	var buf bytes.Buffer
	buf.WriteString("event: response.failed\ndata: ")
	buf.Write(payload)
	buf.WriteString("\n\n")
	writeCodexCompactV2SSE(c, http.StatusOK, buf.Bytes())
}

func applyCodexCompactV2ToResponsesJSON(body []byte, model, responseID string) ([]byte, compactV2Class, error) {
	class, summary := classifyCodexCompactV2JSON(body)
	switch class {
	case compactV2Passthrough, compactV2FailOpen:
		return body, class, nil
	case compactV2Empty:
		return nil, class, fmt.Errorf("compact response carries no summary text")
	default:
		if strings.TrimSpace(responseID) == "" {
			responseID = gjson.GetBytes(body, "id").String()
		}
		var usage *apicompat.ResponsesUsage
		if raw := gjson.GetBytes(body, "usage"); raw.Exists() && raw.IsObject() {
			var parsed apicompat.ResponsesUsage
			if err := json.Unmarshal([]byte(raw.Raw), &parsed); err == nil {
				usage = &parsed
			}
		}
		resp, err := buildCodexCompactV2Responses(summary, model, responseID, usage)
		if err != nil {
			return nil, compactV2Empty, err
		}
		encoded, err := json.Marshal(resp)
		if err != nil {
			return nil, compactV2FailOpen, err
		}
		return encoded, compactV2Synthesize, nil
	}
}

func compactV2LooksLikeSSE(buf []byte) bool {
	trimmed := bytes.TrimSpace(buf)
	if bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte("data:")) {
		return true
	}
	return bytes.Contains(buf, []byte("\nevent:")) || bytes.Contains(buf, []byte("\ndata:"))
}

func extractChatPlainTextFromUpstream(body []byte) string {
	var ccResp apicompat.ChatCompletionsResponse
	if err := json.Unmarshal(body, &ccResp); err == nil && len(ccResp.Choices) > 0 {
		message := ccResp.Choices[0].Message
		if text := chatMessagePlainTextForCompact(message.Content); text != "" {
			return text
		}
		if text := strings.TrimSpace(message.ReasoningContent); text != "" {
			return text
		}
	}
	var collected strings.Builder
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, ok := extractOpenAISSEDataLine(string(line))
		if !ok {
			continue
		}
		if text := gjson.Get(payload, "choices.0.delta.content"); text.Exists() && text.Type == gjson.String {
			collected.WriteString(text.String())
		}
	}
	return strings.TrimSpace(collected.String())
}

func applyCodexCompactV2ToSSE(buf []byte, model string) ([]byte, compactV2Class, error) {
	class, summary, responseID := classifyCodexCompactV2SSE(buf)
	switch class {
	case compactV2Passthrough, compactV2FailOpen:
		return buf, class, nil
	case compactV2Empty:
		return nil, class, fmt.Errorf("compact response carries no summary text")
	default:
		resp, err := buildCodexCompactV2Responses(summary, model, responseID, nil)
		if err != nil {
			return nil, compactV2Empty, err
		}
		encoded, err := json.Marshal(resp)
		if err != nil {
			return buf, compactV2FailOpen, err
		}
		payload, ok := buildCodexCompactV2SSE(encoded)
		if !ok {
			return buf, compactV2FailOpen, nil
		}
		return payload, compactV2Synthesize, nil
	}
}

func extractUsageFromCompactBuffer(buf []byte) OpenAIUsage {
	if parsed, ok := extractOpenAIUsageFromJSONBytes(buf); ok {
		return parsed
	}
	for _, line := range bytes.Split(buf, []byte("\n")) {
		payload, ok := extractOpenAISSEDataLine(string(line))
		if !ok {
			continue
		}
		if parsed, ok := extractOpenAIUsageFromJSONBytes([]byte(payload)); ok {
			return parsed
		}
		if raw := gjson.Get(payload, "response.usage"); raw.Exists() && raw.IsObject() {
			if parsed, ok := extractOpenAIUsageFromJSONBytes([]byte(raw.Raw)); ok {
				return parsed
			}
		}
	}
	return OpenAIUsage{}
}

func (s *OpenAIGatewayService) writeMarkedCodexCompactV2Stream(
	c *gin.Context,
	account *Account,
	resp *http.Response,
	model, via string,
) (OpenAIUsage, string, bool) {
	if !isCodexCompactV2FallbackMarked(c) || resp == nil {
		return OpenAIUsage{}, "", false
	}
	buf, ok := readLimitedUpstream(resp.Body, codexCompactV2MaxBufferBytes)
	if !ok {
		logCodexCompactV2Outcome(account, via, "fail_open")
		writeCodexCompactV2Failure(c, "Upstream compact response exceeded buffer limit")
		return OpenAIUsage{}, "", true
	}
	usage := extractUsageFromCompactBuffer(buf)
	var (
		out   []byte
		class compactV2Class
		err   error
	)
	if compactV2LooksLikeSSE(buf) {
		out, class, err = applyCodexCompactV2ToSSE(buf, model)
	} else {
		var jsonOut []byte
		jsonOut, class, err = applyCodexCompactV2ToResponsesJSON(buf, model, "")
		switch class {
		case compactV2Synthesize, compactV2Passthrough:
			if payload, ok := buildCodexCompactV2SSE(jsonOut); ok {
				out = payload
			} else if class == compactV2Synthesize {
				class = compactV2FailOpen
				out = buf
			} else {
				out = buf
			}
		default:
			out = jsonOut
		}
	}
	switch class {
	case compactV2Passthrough:
		logCodexCompactV2Outcome(account, via, "passthrough")
		writeCodexCompactV2SSE(c, http.StatusOK, out)
	case compactV2FailOpen:
		logCodexCompactV2Outcome(account, via, "fail_open")
		writeCodexCompactV2SSE(c, http.StatusOK, buf)
	case compactV2Empty:
		logCodexCompactV2Outcome(account, via, "empty_summary")
		writeCodexCompactV2Failure(c, err.Error())
	default:
		logCodexCompactV2Outcome(account, via, "synthesized")
		writeCodexCompactV2SSE(c, http.StatusOK, out)
	}
	responseID := extractOpenAIResponseIDFromJSONBytes(out)
	if responseID == "" {
		responseID = extractOpenAIResponseIDFromJSONBytes(buf)
	}
	return usage, responseID, true
}

func (s *OpenAIGatewayService) consumeMarkedCodexCompactV2NonStream(
	c *gin.Context,
	account *Account,
	body []byte,
	model, via string,
) (next []byte, empty bool, replace bool) {
	if !isCodexCompactV2FallbackMarked(c) {
		return body, false, false
	}
	if compactV2LooksLikeSSE(body) {
		out, class, _ := applyCodexCompactV2ToSSE(body, model)
		switch class {
		case compactV2Empty:
			logCodexCompactV2Outcome(account, via, "empty_summary")
			return nil, true, true
		case compactV2Synthesize:
			logCodexCompactV2Outcome(account, via, "synthesized")
			if final, ok := extractCodexFinalResponse(string(out)); ok {
				return final, false, true
			}
			return out, false, true
		case compactV2Passthrough:
			logCodexCompactV2Outcome(account, via, "passthrough")
			return body, false, false
		default:
			logCodexCompactV2Outcome(account, via, "fail_open")
			return body, false, false
		}
	}
	out, class, _ := applyCodexCompactV2ToResponsesJSON(body, model, "")
	switch class {
	case compactV2Empty:
		logCodexCompactV2Outcome(account, via, "empty_summary")
		return nil, true, true
	case compactV2Synthesize:
		logCodexCompactV2Outcome(account, via, "synthesized")
		return out, false, true
	case compactV2Passthrough:
		logCodexCompactV2Outcome(account, via, "passthrough")
		return body, false, false
	default:
		logCodexCompactV2Outcome(account, via, "fail_open")
		return body, false, false
	}
}

func (s *OpenAIGatewayService) finishCodexCompactV2FromChat(
	c *gin.Context,
	account *Account,
	resp *http.Response,
	originalModel, billingModel, upstreamModel string,
	reasoningEffort, serviceTier *string,
	clientStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	requestID := ""
	if resp != nil {
		requestID = resp.Header.Get("x-request-id")
	}
	body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, fmt.Errorf("read compact chat body: %w", err)
	}
	var (
		ccResp apicompat.ChatCompletionsResponse
		synth  *apicompat.ResponsesResponse
	)
	if unmarshalErr := json.Unmarshal(body, &ccResp); unmarshalErr == nil {
		synth, err = buildCodexCompactV2FromChat(&ccResp, originalModel)
	} else if summary := extractChatPlainTextFromUpstream(body); summary != "" {
		var usage *apicompat.ResponsesUsage
		if parsed, ok := extractOpenAIUsageFromJSONBytes(body); ok {
			usage = &apicompat.ResponsesUsage{
				InputTokens:  parsed.InputTokens,
				OutputTokens: parsed.OutputTokens,
				TotalTokens:  parsed.InputTokens + parsed.OutputTokens,
			}
		}
		synth, err = buildCodexCompactV2Responses(summary, originalModel, "", usage)
	} else {
		logCodexCompactV2Outcome(account, "cc", "fail_open")
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{"type": "api_error", "message": "Failed to parse upstream compact response"},
		})
		return nil, fmt.Errorf("parse compact chat response: %w", unmarshalErr)
	}
	if err != nil {
		logCodexCompactV2Outcome(account, "cc", "empty_summary")
		if clientStream {
			writeCodexCompactV2Failure(c, err.Error())
		} else {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{"type": "api_error", "message": err.Error()},
			})
		}
		return nil, err
	}
	encoded, err := json.Marshal(synth)
	if err != nil {
		return nil, err
	}
	logCodexCompactV2Outcome(account, "cc", "synthesized")
	if clientStream {
		payload, ok := buildCodexCompactV2SSE(encoded)
		if !ok {
			writeCodexCompactV2Failure(c, "Failed to build compact SSE payload")
			return nil, fmt.Errorf("build compact sse")
		}
		writeCodexCompactV2SSE(c, http.StatusOK, payload)
	} else {
		c.JSON(http.StatusOK, synth)
	}
	usage := extractUsageFromCompactBuffer(body)
	return &OpenAIForwardResult{
		RequestID:       requestID,
		Usage:           usage,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		ReasoningEffort: reasoningEffort,
		ServiceTier:     serviceTier,
		Stream:          clientStream,
		Duration:        time.Since(startTime),
	}, nil
}

func readLimitedUpstream(r io.Reader, limit int64) ([]byte, bool) {
	if r == nil {
		return nil, false
	}
	if limit <= 0 {
		limit = codexCompactV2MaxBufferBytes
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, false
	}
	if int64(len(data)) > limit {
		return data[:limit], false
	}
	return data, true
}
