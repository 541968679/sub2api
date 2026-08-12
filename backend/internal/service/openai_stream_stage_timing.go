package service

import (
	"encoding/json"
	"math/rand"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const openAIStreamStageClockCtxKey = "openai_stream_stage_clock"

// streamStageTimingRand is the sampling source; tests may replace it.
var streamStageTimingRand = rand.Float64

// openAIStreamStageClock records client-true TTFT stage timestamps for sampled streams.
// Nil receivers are no-ops so the hot path pays only a nil check when the gate is off.
type openAIStreamStageClock struct {
	start                 time.Time
	doStart               time.Time
	headersAt             time.Time
	firstSSEAt            time.Time
	firstUsefulUpstreamAt time.Time
	firstClientFlushAt    time.Time

	accountID         int64
	model             string
	path              string
	inboundEndpoint   string
	upstreamEndpoint  string
	upstreamRequestID string
	completed         bool
}

func resolveOpenAIStreamStageInboundEndpoint(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Get("_gateway_inbound_endpoint"); ok {
		if s, ok := v.(string); ok {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				return trimmed
			}
		}
	}
	if c.Request != nil && c.Request.URL != nil {
		return strings.TrimSpace(c.Request.URL.Path)
	}
	return ""
}

func streamStageTimingAccountAllowed(ids []int64, accountID int64) bool {
	if len(ids) == 0 {
		return true
	}
	for _, id := range ids {
		if id == accountID {
			return true
		}
	}
	return false
}

func shouldAcceptOpenAIStreamStageSample(cfg *config.GatewayConfig, accountID int64) bool {
	if cfg == nil || !cfg.StreamStageTimingEnabled {
		return false
	}
	if !streamStageTimingAccountAllowed(cfg.StreamStageTimingAccountIDs, accountID) {
		return false
	}
	rate := cfg.StreamStageTimingSampleRate
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	return streamStageTimingRand() < rate
}

func (s *OpenAIGatewayService) beginOpenAIStreamStageTiming(
	c *gin.Context,
	account *Account,
	model, path string,
	startTime time.Time,
) *openAIStreamStageClock {
	if s == nil || s.cfg == nil {
		return nil
	}
	// Reuse an existing sampled clock (e.g. invalid_encrypted_content retry) so we do not
	// re-roll the sample gate mid-request.
	if existing := getOpenAIStreamStageClock(c); existing != nil {
		return existing
	}
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	if !shouldAcceptOpenAIStreamStageSample(&s.cfg.Gateway, accountID) {
		return nil
	}
	if startTime.IsZero() {
		startTime = time.Now()
	}
	clk := &openAIStreamStageClock{
		start:            startTime,
		accountID:        accountID,
		model:            strings.TrimSpace(model),
		path:             strings.TrimSpace(path),
		inboundEndpoint:  resolveOpenAIStreamStageInboundEndpoint(c),
		upstreamEndpoint: GetActualOpenAIUpstreamEndpoint(c),
	}
	if c != nil {
		c.Set(openAIStreamStageClockCtxKey, clk)
	}
	return clk
}

// ResetForUpstreamRetry clears Do/body-stage stamps so a same-request upstream retry
// remeasures do_wait / SSE / flush against the successful attempt.
func (clk *openAIStreamStageClock) ResetForUpstreamRetry() {
	if clk == nil || clk.completed {
		return
	}
	clk.doStart = time.Time{}
	clk.headersAt = time.Time{}
	clk.firstSSEAt = time.Time{}
	clk.firstUsefulUpstreamAt = time.Time{}
	clk.firstClientFlushAt = time.Time{}
	clk.upstreamRequestID = ""
}

func getOpenAIStreamStageClock(c *gin.Context) *openAIStreamStageClock {
	if c == nil {
		return nil
	}
	v, ok := c.Get(openAIStreamStageClockCtxKey)
	if !ok || v == nil {
		return nil
	}
	clk, _ := v.(*openAIStreamStageClock)
	return clk
}

func (clk *openAIStreamStageClock) MarkDoStart() {
	if clk == nil || !clk.doStart.IsZero() {
		return
	}
	clk.doStart = time.Now()
}

func (clk *openAIStreamStageClock) MarkHeaders(upstreamRequestID string) {
	if clk == nil || !clk.headersAt.IsZero() {
		return
	}
	clk.headersAt = time.Now()
	if id := strings.TrimSpace(upstreamRequestID); id != "" {
		clk.upstreamRequestID = id
	}
}

func (clk *openAIStreamStageClock) MarkFirstSSE() {
	if clk == nil || !clk.firstSSEAt.IsZero() {
		return
	}
	clk.firstSSEAt = time.Now()
}

func (clk *openAIStreamStageClock) MarkFirstUsefulUpstream() {
	if clk == nil || !clk.firstUsefulUpstreamAt.IsZero() {
		return
	}
	clk.firstUsefulUpstreamAt = time.Now()
}

func (clk *openAIStreamStageClock) MarkFirstClientFlush() {
	if clk == nil || !clk.firstClientFlushAt.IsZero() {
		return
	}
	clk.firstClientFlushAt = time.Now()
}

func (clk *openAIStreamStageClock) msSinceStart(t time.Time) int {
	if clk == nil || clk.start.IsZero() || t.IsZero() {
		return -1
	}
	ms := int(t.Sub(clk.start).Milliseconds())
	if ms < 0 {
		return 0
	}
	return ms
}

func (clk *openAIStreamStageClock) stageSnapshot() (preDo, doWait, firstSSE, firstUseful, firstFlush, duration int) {
	if clk == nil {
		return -1, -1, -1, -1, -1, -1
	}
	preDo = -1
	doWait = -1
	if !clk.doStart.IsZero() {
		preDo = int(clk.doStart.Sub(clk.start).Milliseconds())
		if preDo < 0 {
			preDo = 0
		}
	}
	if !clk.doStart.IsZero() && !clk.headersAt.IsZero() {
		doWait = int(clk.headersAt.Sub(clk.doStart).Milliseconds())
		if doWait < 0 {
			doWait = 0
		}
	}
	firstSSE = clk.msSinceStart(clk.firstSSEAt)
	firstUseful = clk.msSinceStart(clk.firstUsefulUpstreamAt)
	firstFlush = clk.msSinceStart(clk.firstClientFlushAt)
	duration = int(time.Since(clk.start).Milliseconds())
	if duration < 0 {
		duration = 0
	}
	return preDo, doWait, firstSSE, firstUseful, firstFlush, duration
}

// Complete emits one secret-free completed stage line. Safe to call multiple times.
func (clk *openAIStreamStageClock) Complete(c *gin.Context) {
	if clk == nil || clk.completed {
		return
	}
	clk.completed = true
	if ep := GetActualOpenAIUpstreamEndpoint(c); ep != "" {
		clk.upstreamEndpoint = ep
	}
	if clk.inboundEndpoint == "" {
		clk.inboundEndpoint = resolveOpenAIStreamStageInboundEndpoint(c)
	}
	preDo, doWait, firstSSE, firstUseful, firstFlush, duration := clk.stageSnapshot()
	logger.LegacyPrintf(
		"service.openai_gateway",
		"[OpenAI stream_stage] completed account_id=%d path=%s model=%s inbound_endpoint=%s upstream_endpoint=%s upstream_request_id=%s pre_do_ms=%d do_wait_ms=%d first_sse_ms=%d first_useful_upstream_ms=%d first_client_flush_ms=%d duration_ms=%d",
		clk.accountID,
		clk.path,
		clk.model,
		clk.inboundEndpoint,
		clk.upstreamEndpoint,
		clk.upstreamRequestID,
		preDo,
		doWait,
		firstSSE,
		firstUseful,
		firstFlush,
		duration,
	)
}

func openAIChatPayloadStartsUsefulOutput(payload string) bool {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" || trimmed == "[DONE]" {
		return false
	}
	if isOpenAIChatUsageOnlyStreamChunk(trimmed) {
		return false
	}
	var chunk apicompat.ChatCompletionsChunk
	if err := json.Unmarshal([]byte(trimmed), &chunk); err != nil {
		if gjson.Get(trimmed, "choices.0.delta.content").String() != "" {
			return true
		}
		if gjson.Get(trimmed, "choices.0.delta.reasoning_content").String() != "" {
			return true
		}
		if gjson.Get(trimmed, "choices.0.delta.reasoning").String() != "" {
			return true
		}
		if gjson.Get(trimmed, "choices.0.delta.tool_calls").Exists() && len(gjson.Get(trimmed, "choices.0.delta.tool_calls").Array()) > 0 {
			return true
		}
		return false
	}
	return chatChunkStartsResponsesOutput(&chunk)
}
