package service

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/tidwall/gjson"
)

type newAPISlimUsage struct {
	InputTokens  int
	OutputTokens int
	CachedTokens int
	HasCached    bool
}

type newAPISlimStreamState struct {
	enabled         bool
	wroteCompleted  bool
	sawSoftTerminal bool
	lastDisplay     newAPISlimUsage
	hasLastDisplay  bool
}

func shouldSlimNewAPICompleted(enabled bool, eventType string, billingOutputTokens int) bool {
	return enabled && eventType == "response.completed" && billingOutputTokens != 0
}

func isNewAPISlimSoftTerminal(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.done", "response.incomplete", "response.cancelled", "response.canceled":
		return true
	default:
		return false
	}
}

func shouldSynthesizeNewAPISlimCompleted(enabled, wroteCompleted, sawSoftTerminal, clientDisconnected bool, billingOutputTokens int) bool {
	return enabled && !wroteCompleted && sawSoftTerminal && !clientDisconnected && billingOutputTokens != 0
}

func extractNewAPISlimResponseID(data []byte, fallback string) string {
	if id := strings.TrimSpace(gjson.GetBytes(data, "response.id").String()); id != "" {
		return id
	}
	return fallback
}

func extractNewAPISlimUsageFromResponsesData(data []byte) (newAPISlimUsage, bool) {
	if len(data) == 0 || !gjson.ValidBytes(data) {
		return newAPISlimUsage{}, false
	}
	usageNode := gjson.GetBytes(data, "response.usage")
	if !usageNode.Exists() || !usageNode.IsObject() {
		return newAPISlimUsage{}, false
	}
	usage := newAPISlimUsage{
		InputTokens:  int(gjson.GetBytes(data, "response.usage.input_tokens").Int()),
		OutputTokens: int(gjson.GetBytes(data, "response.usage.output_tokens").Int()),
	}
	cached := gjson.GetBytes(data, "response.usage.input_tokens_details.cached_tokens")
	if cached.Type == gjson.Number {
		usage.HasCached = true
		usage.CachedTokens = int(cached.Int())
	}
	return usage, true
}

func buildNewAPISlimCompletedData(id string, usage newAPISlimUsage) []byte {
	payload := struct {
		Type     string `json:"type"`
		Response struct {
			ID    string `json:"id"`
			Usage struct {
				InputTokens        int `json:"input_tokens"`
				OutputTokens       int `json:"output_tokens"`
				TotalTokens        int `json:"total_tokens"`
				InputTokensDetails *struct {
					CachedTokens int `json:"cached_tokens"`
				} `json:"input_tokens_details,omitempty"`
			} `json:"usage"`
		} `json:"response"`
	}{}
	payload.Type = "response.completed"
	payload.Response.ID = id
	payload.Response.Usage.InputTokens = usage.InputTokens
	payload.Response.Usage.OutputTokens = usage.OutputTokens
	payload.Response.Usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	if usage.HasCached {
		payload.Response.Usage.InputTokensDetails = &struct {
			CachedTokens int `json:"cached_tokens"`
		}{CachedTokens: usage.CachedTokens}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return nil
	}
	return bytes.TrimSpace(buf.Bytes())
}

func buildNewAPISlimCompletedSSELine(id string, usage newAPISlimUsage) string {
	data := buildNewAPISlimCompletedData(id, usage)
	if len(data) == 0 {
		return ""
	}
	return "data: " + string(data)
}

func (st *newAPISlimStreamState) noteRewrittenData(eventType string, rewrittenData []byte) {
	if !st.enabled {
		return
	}
	if eventType == "response.completed" {
		st.wroteCompleted = true
	}
	if isNewAPISlimSoftTerminal(eventType) {
		st.sawSoftTerminal = true
	}
	if eventType == "response.completed" || isNewAPISlimSoftTerminal(eventType) {
		if usage, ok := extractNewAPISlimUsageFromResponsesData(rewrittenData); ok {
			st.lastDisplay = usage
			st.hasLastDisplay = true
		}
	}
}

func (st *newAPISlimStreamState) slimCompletedLine(line, responseID string, billingOutputTokens int) (string, bool) {
	if st == nil || !st.enabled {
		return line, false
	}
	data, ok := extractOpenAISSEDataLine(line)
	if !ok {
		return line, false
	}
	eventType := strings.TrimSpace(gjson.Get(data, "type").String())
	if !shouldSlimNewAPICompleted(st.enabled, eventType, billingOutputTokens) {
		return line, false
	}
	usage, extracted := extractNewAPISlimUsageFromResponsesData([]byte(data))
	if !extracted {
		return line, false
	}
	id := extractNewAPISlimResponseID([]byte(data), responseID)
	slimLine := buildNewAPISlimCompletedSSELine(id, usage)
	if slimLine == "" {
		return line, false
	}
	st.wroteCompleted = true
	return slimLine, true
}

func (st *newAPISlimStreamState) shouldSynthesize(clientDisconnected bool, billingOutputTokens int) bool {
	if st == nil {
		return false
	}
	return shouldSynthesizeNewAPISlimCompleted(st.enabled, st.wroteCompleted, st.sawSoftTerminal, clientDisconnected, billingOutputTokens)
}

func slimUsageFromBillingWithRewrite(responseID string, billing *OpenAIUsage, mult *DisplayTokenMultipliers) newAPISlimUsage {
	usage := newAPISlimUsage{}
	if billing != nil {
		usage.InputTokens = billing.InputTokens
		usage.OutputTokens = billing.OutputTokens
		if billing.CacheReadInputTokens != 0 {
			usage.HasCached = true
			usage.CachedTokens = billing.CacheReadInputTokens
		}
	}
	data := buildNewAPISlimCompletedData(responseID, usage)
	line := "data: " + string(data)
	if mult != nil {
		line = rewriteOpenAIResponsesSSEUsageTokens(line, mult)
		if rewritten, ok := extractOpenAISSEDataLine(line); ok {
			data = []byte(rewritten)
		}
	}
	if extracted, ok := extractNewAPISlimUsageFromResponsesData(data); ok {
		return extracted
	}
	return usage
}

func (st *newAPISlimStreamState) synthesizeLine(responseID string, billing *OpenAIUsage, mult *DisplayTokenMultipliers) string {
	usage := st.lastDisplay
	if !st.hasLastDisplay {
		usage = slimUsageFromBillingWithRewrite(responseID, billing, mult)
	}
	st.wroteCompleted = true
	return buildNewAPISlimCompletedSSELine(responseID, usage)
}
