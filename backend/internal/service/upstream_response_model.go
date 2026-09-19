package service

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	upstreamResponseModelObserverContextKey = "upstream_response_model_observer"
	upstreamResponseModelMaxLength          = 200
)

// upstreamResponseModelObserver tracks one forwarding attempt (or one WS turn).
// A terminal declaration wins over an earlier declaration; otherwise the first
// declaration is retained. Observation never affects the forwarding path or billing.
type upstreamResponseModelObserver struct {
	first    string
	terminal string
	conflict bool
}

func (o *upstreamResponseModelObserver) Observe(model string, terminal bool) {
	model = normalizeObservedUpstreamResponseModel(model)
	if model == "" {
		return
	}
	current := o.Model()
	if current != "" && !strings.EqualFold(current, model) {
		o.conflict = true
	}
	if terminal {
		o.terminal = model
		return
	}
	if o.first == "" {
		o.first = model
	}
}

func normalizeObservedUpstreamResponseModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return ""
	}
	runes := []rune(model)
	if len(runes) > upstreamResponseModelMaxLength {
		model = string(runes[:upstreamResponseModelMaxLength])
	}
	return model
}

func (o *upstreamResponseModelObserver) ObserveOpenAI(payload []byte, eventType string) {
	model := firstValidTrimmedGJSONString(payload, "response.model", "model")
	o.Observe(model, isUpstreamResponseModelTerminalEvent(eventType))
}

func (o *upstreamResponseModelObserver) ObserveAnthropic(payload []byte) {
	model := firstValidTrimmedGJSONString(payload, "message.model", "model")
	o.Observe(model, false)
}

func (o *upstreamResponseModelObserver) ObserveGemini(payload []byte) {
	model := firstValidTrimmedGJSONString(
		payload,
		"modelVersion",
		"response.modelVersion",
		"response.response.modelVersion",
	)
	// Gemini streaming has no universal terminal event carrying modelVersion;
	// treating each declaration as terminal retains the latest chunk.
	o.Observe(model, true)
}

func (o *upstreamResponseModelObserver) Model() string {
	if o == nil {
		return ""
	}
	if o.terminal != "" {
		return o.terminal
	}
	return o.first
}

func (o *upstreamResponseModelObserver) Conflict() bool {
	return o != nil && o.conflict
}

func beginUpstreamResponseModelObservation(c *gin.Context) *upstreamResponseModelObserver {
	observer := &upstreamResponseModelObserver{}
	if c != nil {
		c.Set(upstreamResponseModelObserverContextKey, observer)
	}
	return observer
}

func upstreamResponseModelObserverFromContext(c *gin.Context) *upstreamResponseModelObserver {
	if c == nil {
		return nil
	}
	value, ok := c.Get(upstreamResponseModelObserverContextKey)
	if !ok {
		return nil
	}
	observer, _ := value.(*upstreamResponseModelObserver)
	return observer
}

func observedUpstreamResponseModel(c *gin.Context) string {
	return upstreamResponseModelObserverFromContext(c).Model()
}

func firstValidTrimmedGJSONString(payload []byte, paths ...string) string {
	if len(payload) == 0 {
		return ""
	}
	for _, path := range paths {
		value := gjson.GetBytes(payload, path)
		if !value.Exists() || value.Type != gjson.String {
			continue
		}
		if text := strings.TrimSpace(value.String()); text != "" {
			if !gjson.ValidBytes(payload) {
				return ""
			}
			return text
		}
	}
	return ""
}

func isUpstreamResponseModelTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case "response.completed", "response.done", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
		return true
	default:
		return false
	}
}

func upstreamModelMismatch(sentModel, responseModel string) *bool {
	responseModel = strings.TrimSpace(responseModel)
	if responseModel == "" {
		return nil
	}
	sentModel = strings.TrimSpace(sentModel)
	mismatch := sentModel == "" || !upstreamModelsMatchForAudit(sentModel, responseModel)
	return &mismatch
}

func upstreamModelsMatchForAudit(sentModel, responseModel string) bool {
	if strings.EqualFold(sentModel, responseModel) {
		return true
	}
	sentGrokModel := canonicalGrokBuildRuntimeModel(sentModel)
	return sentGrokModel != "" && sentGrokModel == canonicalGrokBuildRuntimeModel(responseModel)
}

func canonicalGrokBuildRuntimeModel(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "grok-4.5", "grok-4.5-latest", "grok-4.5-build":
		return "grok-4.5-build"
	case "grok-4.6", "grok-4.6-latest", "grok-4.6-build":
		return "grok-4.6-build"
	default:
		return ""
	}
}

func upstreamSentModel(requestedModel, upstreamModel string) string {
	sentModel := strings.TrimSpace(upstreamModel)
	if sentModel == "" {
		sentModel = strings.TrimSpace(requestedModel)
	}
	return sentModel
}

func attachUpstreamResponseModelAudit(log *UsageLog, sentModel, responseModel string) {
	if log == nil {
		return
	}
	responseModel = normalizeObservedUpstreamResponseModel(responseModel)
	if responseModel == "" {
		return
	}
	copied := responseModel
	log.UpstreamResponseModel = &copied
	log.UpstreamModelMismatch = upstreamModelMismatch(sentModel, responseModel)
}

func observeOpenAIResponseBody(c *gin.Context, body []byte) {
	observer := upstreamResponseModelObserverFromContext(c)
	if observer == nil || len(body) == 0 {
		return
	}
	observer.ObserveOpenAI(body, strings.TrimSpace(gjson.GetBytes(body, "type").String()))
}

func observeOpenAISSELine(c *gin.Context, line string) {
	observer := upstreamResponseModelObserverFromContext(c)
	if observer == nil {
		return
	}
	data, ok := extractOpenAISSEDataLine(line)
	if !ok || data == "" || data == "[DONE]" {
		return
	}
	observer.ObserveOpenAI([]byte(data), strings.TrimSpace(gjson.Get(data, "type").String()))
}

func observeOpenAISSEBody(observer *upstreamResponseModelObserver, body string) {
	if observer == nil || strings.TrimSpace(body) == "" {
		return
	}
	eventType := ""
	var data strings.Builder
	flush := func() {
		payload := strings.TrimSpace(data.String())
		if payload != "" {
			observer.ObserveOpenAI([]byte(payload), eventType)
		}
		eventType = ""
		data.Reset()
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		if value, ok := strings.CutPrefix(line, "event:"); ok {
			eventType = strings.TrimSpace(value)
			continue
		}
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(value))
		}
	}
	flush()
}

func observeAnthropicResponseBody(c *gin.Context, body []byte) {
	observer := upstreamResponseModelObserverFromContext(c)
	if observer == nil || len(body) == 0 {
		return
	}
	observer.ObserveAnthropic(body)
}

func observeAnthropicSSEData(c *gin.Context, data string) {
	observer := upstreamResponseModelObserverFromContext(c)
	if observer == nil || data == "" || data == "[DONE]" {
		return
	}
	observer.ObserveAnthropic([]byte(data))
}

func observeGeminiResponseBody(c *gin.Context, body []byte) {
	observer := upstreamResponseModelObserverFromContext(c)
	if observer == nil || len(body) == 0 {
		return
	}
	observer.ObserveGemini(body)
}

func observeAndStampForwardResult(c *gin.Context, result *ForwardResult) *ForwardResult {
	if result == nil {
		return nil
	}
	if model := observedUpstreamResponseModel(c); model != "" {
		result.UpstreamResponseModel = model
	}
	return result
}

func observeAndStampOpenAIForwardResult(c *gin.Context, result *OpenAIForwardResult) *OpenAIForwardResult {
	if result == nil {
		return nil
	}
	if model := observedUpstreamResponseModel(c); model != "" {
		result.UpstreamResponseModel = model
	}
	return result
}

func observeGeminiSSEData(c *gin.Context, data string) {
	observer := upstreamResponseModelObserverFromContext(c)
	if observer == nil || data == "" || data == "[DONE]" {
		return
	}
	observer.ObserveGemini([]byte(data))
}

func (s *AntigravityGatewayService) observeAntigravityGeminiSSELine(c *gin.Context, line string) {
	if s == nil {
		return
	}
	data, ok := extractAnthropicSSEDataLine(line)
	if !ok || data == "" || data == "[DONE]" {
		return
	}
	observeGeminiSSEData(c, data)
}
