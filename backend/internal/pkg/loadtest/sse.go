package loadtest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"
)

type usageSnap struct {
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	TotalTokens      int
}

type streamStats struct {
	FirstSSE      time.Duration
	FirstToken    time.Duration
	FirstContent  time.Duration
	Chunks        int
	ContentChars  int
	FinishReason  string
	ResponseModel string
	Usage         usageSnap
	SawDone       bool
	SawContent    bool
	SawToolCall   bool
	Truncated     bool
	ParseErrors   int

	Inspected            int
	ModelMissing         int
	ModelEmpty           int
	ModelPresent         int
	ModelMismatch        int
	ResponseModelMissing int
	FirstModel           string
	LastModel            string
	MismatchValue        string
	RoleOnlyFirst        bool
	UpstreamError        string
}

func consumeSSE(r io.Reader, t0 time.Time, abortAfterContent bool, abort func(), requested string) streamStats {
	var st streamStats
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			parseSSELine(bytes.TrimSpace(line), t0, &st, requested)
			if abortAfterContent && st.SawContent && abort != nil {
				abort()
				break
			}
		}
		if err != nil {
			if err != io.EOF {
				st.Truncated = true
			} else if !st.SawDone && st.FinishReason == "" {
				st.Truncated = st.Chunks > 0
			}
			break
		}
	}
	return st
}

func parseSSELine(line []byte, t0 time.Time, st *streamStats, requested string) {
	if len(line) == 0 || line[0] == ':' {
		return
	}
	if !bytes.HasPrefix(line, []byte("data:")) {
		return
	}
	payload := bytes.TrimSpace(line[5:])
	if len(payload) == 0 {
		return
	}
	st.Chunks++
	if st.FirstSSE == 0 {
		st.FirstSSE = time.Since(t0)
	}
	if bytes.Equal(payload, []byte("[DONE]")) {
		st.SawDone = true
		return
	}
	firstJSON := st.Inspected == 0 && st.ParseErrors == 0
	inspectJSONObject(payload, requested, st, firstJSON)
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		st.ParseErrors++
		return
	}
	if msg := nestedErrorMessage(obj); msg != "" && st.UpstreamError == "" {
		st.UpstreamError = msg
	}
	if st.LastModel != "" {
		st.ResponseModel = st.LastModel
	} else if m, _ := obj["model"].(string); m != "" && st.ResponseModel == "" {
		st.ResponseModel = m
	}
	if u, ok := obj["usage"].(map[string]any); ok {
		st.Usage = parseUsage(u)
	}
	if ingestResponsesEvent(obj, t0, st) {
		return
	}
	choices, _ := obj["choices"].([]any)
	if len(choices) == 0 {
		return
	}
	ch, _ := choices[0].(map[string]any)
	if ch == nil {
		return
	}
	if fr, _ := ch["finish_reason"].(string); fr != "" {
		st.FinishReason = fr
	}
	delta, _ := ch["delta"].(map[string]any)
	if delta == nil {
		if msg, ok := ch["message"].(map[string]any); ok {
			delta = msg
		}
	}
	if delta == nil {
		return
	}
	if c := deltaContent(delta["reasoning_content"]); c != "" {
		noteFirstToken(st, t0)
	}
	if c := deltaContent(delta["reasoning"]); c != "" {
		noteFirstToken(st, t0)
	}
	if c := deltaContent(delta["content"]); c != "" {
		st.SawContent = true
		st.ContentChars += len([]rune(c))
		if st.FirstContent == 0 {
			st.FirstContent = time.Since(t0)
		}
		noteFirstToken(st, t0)
	}
	if _, ok := delta["tool_calls"]; ok {
		st.SawToolCall = true
		noteFirstToken(st, t0)
		if st.FirstContent == 0 {
			st.FirstContent = time.Since(t0)
		}
	}
}

func noteFirstToken(st *streamStats, t0 time.Time) {
	if st.FirstToken == 0 {
		st.FirstToken = time.Since(t0)
	}
}

func deltaContent(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, part := range t {
			m, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if s, _ := m["text"].(string); s != "" {
				b.WriteString(s)
			}
		}
		return b.String()
	default:
		return ""
	}
}

func ingestResponsesEvent(obj map[string]any, t0 time.Time, st *streamStats) bool {
	typ, _ := obj["type"].(string)
	if !strings.HasPrefix(typ, "response.") {
		return false
	}
	if delta, ok := obj["delta"].(string); ok && strings.TrimSpace(delta) != "" {
		st.SawContent = true
		st.ContentChars += len([]rune(delta))
		if st.FirstContent == 0 {
			st.FirstContent = time.Since(t0)
		}
		noteFirstToken(st, t0)
	}
	if resp, ok := obj["response"].(map[string]any); ok {
		ingestResponsesBody(resp, st)
	}
	switch typ {
	case "response.completed":
		st.FinishReason = "stop"
		st.SawDone = true
	case "response.failed", "response.incomplete":
		if st.FinishReason == "" {
			st.FinishReason = strings.TrimPrefix(typ, "response.")
		}
	}
	return true
}

func ingestResponsesBody(resp map[string]any, st *streamStats) {
	if m, _ := resp["model"].(string); m != "" {
		st.ResponseModel = m
		st.LastModel = m
	}
	if u, ok := resp["usage"].(map[string]any); ok {
		st.Usage = parseUsage(u)
	}
	if text, _ := resp["output_text"].(string); strings.TrimSpace(text) != "" {
		st.SawContent = true
		n := len([]rune(text))
		if n > st.ContentChars {
			st.ContentChars = n
		}
	}
	if st.SawContent {
		return
	}
	outputs, _ := resp["output"].([]any)
	for _, item := range outputs {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content, _ := m["content"].([]any)
		for _, block := range content {
			bm, ok := block.(map[string]any)
			if !ok {
				continue
			}
			if c := deltaContent(bm["text"]); c != "" {
				st.SawContent = true
				st.ContentChars += len([]rune(c))
			}
		}
	}
}

func parseUsage(u map[string]any) usageSnap {
	s := usageSnap{
		PromptTokens:     jsonInt(u["prompt_tokens"]),
		CompletionTokens: jsonInt(u["completion_tokens"]),
		TotalTokens:      jsonInt(u["total_tokens"]),
	}
	if s.PromptTokens == 0 {
		s.PromptTokens = jsonInt(u["input_tokens"])
	}
	if s.CompletionTokens == 0 {
		s.CompletionTokens = jsonInt(u["output_tokens"])
	}
	if details, ok := u["prompt_tokens_details"].(map[string]any); ok {
		s.CachedTokens = jsonInt(details["cached_tokens"])
	}
	if s.CachedTokens == 0 {
		if details, ok := u["input_tokens_details"].(map[string]any); ok {
			s.CachedTokens = jsonInt(details["cached_tokens"])
		}
	}
	if s.CachedTokens == 0 {
		s.CachedTokens = jsonInt(u["cache_read_input_tokens"])
	}
	return s
}

func jsonInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	default:
		return 0
	}
}

func parseJSONCompletion(body []byte, requested string) streamStats {
	var st streamStats
	inspectJSONObject(body, requested, &st, true)
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		st.ParseErrors++
		return st
	}
	if msg := nestedErrorMessage(obj); msg != "" {
		st.UpstreamError = msg
	}
	if st.LastModel != "" {
		st.ResponseModel = st.LastModel
	} else if m, _ := obj["model"].(string); m != "" {
		st.ResponseModel = m
	}
	if u, ok := obj["usage"].(map[string]any); ok {
		st.Usage = parseUsage(u)
	}
	if _, hasChoices := obj["choices"]; !hasChoices {
		ingestResponsesBody(obj, &st)
		if resp, ok := obj["response"].(map[string]any); ok {
			ingestResponsesBody(resp, &st)
		}
		if st.SawContent && st.FinishReason == "" {
			st.FinishReason = "stop"
		}
		return st
	}
	choices, _ := obj["choices"].([]any)
	if len(choices) == 0 {
		return st
	}
	ch, _ := choices[0].(map[string]any)
	if ch == nil {
		return st
	}
	if fr, _ := ch["finish_reason"].(string); fr != "" {
		st.FinishReason = fr
	}
	msg, _ := ch["message"].(map[string]any)
	if msg == nil {
		return st
	}
	if c := deltaContent(msg["content"]); c != "" {
		st.SawContent = true
		st.ContentChars = len([]rune(c))
	}
	if _, ok := msg["tool_calls"]; ok {
		st.SawToolCall = true
	}
	return st
}
