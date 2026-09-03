//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldAcceptOpenAIStreamStageSample_GateAndAllowlist(t *testing.T) {
	t.Parallel()

	require.False(t, shouldAcceptOpenAIStreamStageSample(nil, 1))
	require.False(t, shouldAcceptOpenAIStreamStageSample(&config.GatewayConfig{
		StreamStageTimingEnabled:    false,
		StreamStageTimingSampleRate: 1,
	}, 1))
	require.False(t, shouldAcceptOpenAIStreamStageSample(&config.GatewayConfig{
		StreamStageTimingEnabled:    true,
		StreamStageTimingSampleRate: 0,
	}, 1))
	require.False(t, shouldAcceptOpenAIStreamStageSample(&config.GatewayConfig{
		StreamStageTimingEnabled:    true,
		StreamStageTimingSampleRate: 1,
		StreamStageTimingAccountIDs: []int64{99},
	}, 1))
	require.True(t, shouldAcceptOpenAIStreamStageSample(&config.GatewayConfig{
		StreamStageTimingEnabled:    true,
		StreamStageTimingSampleRate: 1,
		StreamStageTimingAccountIDs: []int64{99},
	}, 99))
	require.True(t, shouldAcceptOpenAIStreamStageSample(&config.GatewayConfig{
		StreamStageTimingEnabled:    true,
		StreamStageTimingSampleRate: 1,
	}, 1))
}

func TestShouldAcceptOpenAIStreamStageSample_UsesSampleRate(t *testing.T) {
	orig := streamStageTimingRand
	t.Cleanup(func() { streamStageTimingRand = orig })

	streamStageTimingRand = func() float64 { return 0.5 }
	require.False(t, shouldAcceptOpenAIStreamStageSample(&config.GatewayConfig{
		StreamStageTimingEnabled:    true,
		StreamStageTimingSampleRate: 0.2,
	}, 1))
	require.True(t, shouldAcceptOpenAIStreamStageSample(&config.GatewayConfig{
		StreamStageTimingEnabled:    true,
		StreamStageTimingSampleRate: 0.6,
	}, 1))
}

func TestOpenAIStreamStageClock_StageOrdering(t *testing.T) {
	t.Parallel()

	start := time.Now()
	clk := &openAIStreamStageClock{start: start, accountID: 7, path: "test", model: "m"}
	time.Sleep(5 * time.Millisecond)
	clk.MarkDoStart()
	time.Sleep(15 * time.Millisecond)
	clk.MarkHeaders("rid-1")
	time.Sleep(10 * time.Millisecond)
	clk.MarkFirstSSE()
	time.Sleep(20 * time.Millisecond)
	clk.MarkFirstUsefulUpstream()
	time.Sleep(25 * time.Millisecond)
	clk.MarkFirstClientFlush()

	preDo, doWait, firstSSE, firstUseful, firstFlush, duration := clk.stageSnapshot()
	require.GreaterOrEqual(t, preDo, 0)
	require.GreaterOrEqual(t, doWait, 10)
	require.GreaterOrEqual(t, firstSSE, doWait)
	require.GreaterOrEqual(t, firstUseful, firstSSE)
	require.GreaterOrEqual(t, firstFlush, firstUseful)
	require.GreaterOrEqual(t, duration, firstFlush)
	require.Equal(t, "rid-1", clk.upstreamRequestID)

	// Nil clock is a no-op.
	var nilClk *openAIStreamStageClock
	nilClk.MarkDoStart()
	nilClk.MarkFirstClientFlush()
	nilClk.Complete(nil)
}

type delayedFlushRecorder struct {
	*httptest.ResponseRecorder
	delay     time.Duration
	mu        sync.Mutex
	flushedAt time.Time
}

func (w *delayedFlushRecorder) Flush() {
	if w.delay > 0 {
		time.Sleep(w.delay)
	}
	w.mu.Lock()
	if w.flushedAt.IsZero() {
		w.flushedAt = time.Now()
	}
	w.mu.Unlock()
}

func TestStreamStageTiming_ResponsesHTTP_DelayedUsefulAndFlushLag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	start := time.Now()
	body := &delayedSSEBody{chunks: []delayedSSEChunk{
		{line: `data: {"type":"response.created","response":{"id":"r1"}}`},
		{line: `data: {"type":"response.in_progress","response":{"id":"r1"}}`},
		{line: `data: {"type":"response.output_item.added","item":{"type":"message"}}`, delay: 40 * time.Millisecond},
		{line: `data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":1,"output_tokens":1}}}`},
	}}
	rec := &delayedFlushRecorder{ResponseRecorder: httptest.NewRecorder(), delay: 30 * time.Millisecond}
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	clk := &openAIStreamStageClock{start: start, accountID: 1, path: "responses_http", model: "gpt-5.4"}
	clk.MarkDoStart()
	time.Sleep(20 * time.Millisecond)
	clk.MarkHeaders("rid_stage_resp")
	c.Set(openAIStreamStageClockCtxKey, clk)

	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid_stage_resp"}},
		Body:       io.NopCloser(body),
	}
	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, start, "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.firstTokenMs)

	preDo, doWait, firstSSE, firstUseful, firstFlush, _ := clk.stageSnapshot()
	require.GreaterOrEqual(t, doWait, 15)
	require.GreaterOrEqual(t, firstUseful, firstSSE)
	require.GreaterOrEqual(t, firstFlush, firstUseful)
	require.GreaterOrEqual(t, firstUseful-doWait, 25, "body_gap: useful should lag headers")
	require.GreaterOrEqual(t, firstFlush-firstUseful, 20, "flush can lag useful-upstream via delayed Flush")
	_ = preDo

	// usage first_token_ms follows the first SSE frame (response.created), matching
	// Claude-GPT bridge — not the delayed output_item.added used for client flush.
	require.Less(t, *result.firstTokenMs, firstUseful)
	require.LessOrEqual(t, *result.firstTokenMs, firstSSE+15)
	require.NotNil(t, result.trueFirstTokenMs)
	require.GreaterOrEqual(t, *result.trueFirstTokenMs, firstUseful-15)
}

func TestStreamStageTiming_ChatFallback_DelayedUseful(t *testing.T) {
	gin.SetMode(gin.TestMode)
	start := time.Now()
	body := &delayedSSEBody{chunks: []delayedSSEChunk{
		{line: `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant"}}]}`},
		{line: `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":""}}]}`},
		{line: `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"reasoning_content":"think"}}]}`, delay: 35 * time.Millisecond},
		{line: `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`},
		{line: `data: [DONE]`},
	}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	clk := &openAIStreamStageClock{start: start, accountID: 2, path: "responses_chat_fallback", model: "gpt-5.4"}
	clk.doStart = start
	clk.headersAt = start.Add(5 * time.Millisecond)
	c.Set(openAIStreamStageClockCtxKey, clk)

	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid_fb"}},
		Body:       io.NopCloser(body),
	}
	result, err := svc.streamChatCompletionsAsResponses(c, resp, "gpt-5.4", nil, false, nil, "gpt-5.4", "gpt-5.4", nil, nil, start)
	require.NoError(t, err)
	require.NotNil(t, result.FirstTokenMs)

	_, doWait, firstSSE, firstUseful, firstFlush, _ := clk.stageSnapshot()
	require.GreaterOrEqual(t, firstUseful, firstSSE)
	require.GreaterOrEqual(t, firstFlush, firstUseful)
	require.GreaterOrEqual(t, firstUseful-doWait, 25)
}

func TestStreamStageTiming_RawChat_SilentRefusalPendingFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	start := time.Now()
	body := &delayedSSEBody{chunks: []delayedSSEChunk{
		{line: `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant"}}]}`},
		{line: `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":""}}]}`},
		{line: `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hi"}}]}`, delay: 45 * time.Millisecond},
		{line: `data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`},
		{line: `data: [DONE]`},
	}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	clk := &openAIStreamStageClock{start: start, accountID: 3, path: "chat_raw", model: "gpt-5.4"}
	clk.doStart = start
	clk.headersAt = start.Add(2 * time.Millisecond)
	c.Set(openAIStreamStageClockCtxKey, clk)

	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid_raw"}},
		Body:       io.NopCloser(body),
	}
	// Large request body enables silent-refusal pending holdback.
	result, err := svc.streamRawChatCompletions(c, resp, "gpt-5.4", "gpt-5.4", "gpt-5.4", nil, nil, start, openAISilentRefusalMinRequestBodyBytes, &Account{ID: 3})
	require.NoError(t, err)
	require.NotNil(t, result.FirstTokenMs)

	_, _, firstSSE, firstUseful, firstFlush, _ := clk.stageSnapshot()
	require.GreaterOrEqual(t, firstUseful, firstSSE)
	require.GreaterOrEqual(t, firstFlush, firstUseful)
	// first_token_ms marks early role chunk; client flush waits for pending release of useful content.
	require.GreaterOrEqual(t, firstFlush-*result.FirstTokenMs, 30, "first_client_flush must reflect pending release, not only first_token read")
	require.Contains(t, rec.Body.String(), `"content":"hi"`)
}

func TestBeginOpenAIStreamStageTiming_DisabledIsNil(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	require.Nil(t, svc.beginOpenAIStreamStageTiming(nil, &Account{ID: 1}, "m", "p", time.Now()))

	svc.cfg.Gateway.StreamStageTimingEnabled = true
	svc.cfg.Gateway.StreamStageTimingSampleRate = 1
	clk := svc.beginOpenAIStreamStageTiming(nil, &Account{ID: 1}, "m", "p", time.Now())
	require.NotNil(t, clk)
	clk.Complete(nil)
}

func TestBeginOpenAIStreamStageTiming_ReusesClockAcrossUpstreamRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.StreamStageTimingEnabled = true
	svc.cfg.Gateway.StreamStageTimingSampleRate = 1

	start := time.Now()
	first := svc.beginOpenAIStreamStageTiming(c, &Account{ID: 42}, "m", "responses_http", start)
	require.NotNil(t, first)
	first.MarkDoStart()
	first.MarkHeaders("rid-old")
	first.ResetForUpstreamRetry()
	require.True(t, first.doStart.IsZero())
	require.True(t, first.headersAt.IsZero())
	require.Empty(t, first.upstreamRequestID)

	second := svc.beginOpenAIStreamStageTiming(c, &Account{ID: 42}, "m", "responses_http", start)
	require.Same(t, first, second)
	second.MarkDoStart()
	second.MarkHeaders("rid-new")
	require.Equal(t, "rid-new", second.upstreamRequestID)
}

func TestOpenAIChatPayloadStartsUsefulOutput(t *testing.T) {
	t.Parallel()
	require.False(t, openAIChatPayloadStartsUsefulOutput(`{"choices":[{"delta":{"role":"assistant"}}]}`))
	require.False(t, openAIChatPayloadStartsUsefulOutput(`{"choices":[{"delta":{"content":""}}]}`))
	require.True(t, openAIChatPayloadStartsUsefulOutput(`{"choices":[{"delta":{"content":"x"}}]}`))
	require.True(t, openAIChatPayloadStartsUsefulOutput(`{"choices":[{"delta":{"reasoning_content":"r"}}]}`))
	require.True(t, openAIChatPayloadStartsUsefulOutput(`{"choices":[{"delta":{"tool_calls":[{"index":0}]}}]}`))
}

func TestOpenAIStreamDataMarksFirstToken_MatchesBridgeFirstFrame(t *testing.T) {
	t.Parallel()
	require.False(t, openAIStreamDataMarksFirstToken(""))
	require.False(t, openAIStreamDataMarksFirstToken("   "))
	require.False(t, openAIStreamDataMarksFirstToken("[DONE]"))
	require.True(t, openAIStreamDataMarksFirstToken(`{"type":"response.created","response":{"id":"r1"}}`))
	require.True(t, openAIStreamDataMarksFirstToken(`{"type":"response.in_progress"}`))
	require.True(t, openAIStreamDataMarksFirstToken(`{"type":"response.output_item.added"}`))
	require.False(t, openAIStreamDataStartsClientOutput(`{"type":"response.created"}`, "response.created"))
	require.True(t, openAIStreamDataStartsClientOutput(`{"type":"response.output_item.added"}`, "response.output_item.added"))
}

func TestOpenAIStreamShouldCommitDownstream_PreambleToggle(t *testing.T) {
	t.Parallel()
	created := `{"type":"response.created","response":{"id":"r1"}}`
	useful := `{"type":"response.output_item.added","item":{"type":"message"}}`
	failed := `{"type":"response.failed"}`

	require.False(t, openAIStreamShouldCommitDownstream(created, "response.created", false))
	require.True(t, openAIStreamShouldCommitDownstream(created, "response.created", true))
	require.True(t, openAIStreamShouldCommitDownstream(useful, "response.output_item.added", false))
	require.True(t, openAIStreamShouldCommitDownstream(useful, "response.output_item.added", true))
	require.False(t, openAIStreamShouldCommitDownstream(failed, "response.failed", false))
	require.False(t, openAIStreamShouldCommitDownstream(failed, "response.failed", true), "failed must stay uncommitted so pre-output failover can still rewrite JSON")
}

func storeGatewayFlushPreambleCache(t *testing.T, enabled bool, userIDs ...int64) {
	t.Helper()
	prev := gatewayForwardingCache.Load()
	t.Cleanup(func() {
		if prev != nil {
			gatewayForwardingCache.Store(prev)
			return
		}
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	})
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
		flushPreamble:        enabled,
		flushPreambleUserIDs: append([]int64(nil), userIDs...),
		expiresAt:            time.Now().Add(time.Hour).UnixNano(),
	})
}

func TestStreamStageTiming_ResponsesHTTP_FlushPreambleOn_FirstFlushBeforeUseful(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storeGatewayFlushPreambleCache(t, true)

	start := time.Now()
	body := &delayedSSEBody{chunks: []delayedSSEChunk{
		{line: `data: {"type":"response.created","response":{"id":"r1"}}`},
		{line: `data: {"type":"response.in_progress","response":{"id":"r1"}}`},
		{line: `data: {"type":"response.output_item.added","item":{"type":"message"}}`, delay: 40 * time.Millisecond},
		{line: `data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":1,"output_tokens":1}}}`},
	}}
	rec := &delayedFlushRecorder{ResponseRecorder: httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	clk := &openAIStreamStageClock{start: start, accountID: 1, path: "responses_http", model: "gpt-5.4"}
	clk.MarkDoStart()
	clk.MarkHeaders("rid_flush_preamble")
	c.Set(openAIStreamStageClockCtxKey, clk)

	svc := &OpenAIGatewayService{
		cfg: rawChatCompletionsTestConfig(),
		settingService: NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{
			SettingKeyOpenAIResponsesFlushPreamble: "true",
		}}, &config.Config{}),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid_flush_preamble"}},
		Body:       io.NopCloser(body),
	}
	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, start, "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.firstTokenMs)

	_, _, firstSSE, firstUseful, firstFlush, _ := clk.stageSnapshot()
	require.GreaterOrEqual(t, firstSSE, 0)
	require.GreaterOrEqual(t, firstUseful, firstSSE)
	require.GreaterOrEqual(t, firstFlush, firstSSE)
	require.Less(t, firstFlush, firstUseful, "preamble flush should reach downstream before delayed output_item.added")
	require.Contains(t, rec.Body.String(), `"type":"response.created"`)
	require.LessOrEqual(t, *result.firstTokenMs, firstSSE+15)
	require.NotNil(t, result.trueFirstTokenMs)
	require.GreaterOrEqual(t, *result.trueFirstTokenMs, firstUseful-15)
}

func TestStreamStageTiming_ResponsesPassthrough_FlushPreambleOn_FirstFlushBeforeUseful(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storeGatewayFlushPreambleCache(t, true)

	start := time.Now()
	body := &delayedSSEBody{chunks: []delayedSSEChunk{
		{line: `data: {"type":"response.created","response":{"id":"r1"}}`},
		{line: `data: {"type":"response.in_progress","response":{"id":"r1"}}`},
		{line: `data: {"type":"response.output_item.added","item":{"type":"message"}}`, delay: 40 * time.Millisecond},
		{line: `data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":1,"output_tokens":1}}}`},
	}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	clk := &openAIStreamStageClock{start: start, accountID: 1, path: "responses_passthrough", model: "gpt-5.4"}
	clk.MarkDoStart()
	clk.MarkHeaders("rid_flush_preamble_pt")
	c.Set(openAIStreamStageClockCtxKey, clk)

	svc := &OpenAIGatewayService{
		cfg: rawChatCompletionsTestConfig(),
		settingService: NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{
			SettingKeyOpenAIResponsesFlushPreamble: "true",
		}}, &config.Config{}),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid_flush_preamble_pt"}},
		Body:       io.NopCloser(body),
	}
	result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1}, start, "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.firstTokenMs)

	_, _, firstSSE, firstUseful, firstFlush, _ := clk.stageSnapshot()
	require.GreaterOrEqual(t, firstUseful, firstSSE)
	require.Less(t, firstFlush, firstUseful, "passthrough preamble should flush before delayed output_item.added")
	require.Contains(t, rec.Body.String(), `"type":"response.created"`)
	require.NotNil(t, result.trueFirstTokenMs)
	require.GreaterOrEqual(t, *result.trueFirstTokenMs, firstUseful-15)
}

func TestStreamStageTiming_ResponsesHTTP_FlushPreambleUserAllowlist_FirstFlushBeforeUseful(t *testing.T) {
	gin.SetMode(gin.TestMode)
	storeGatewayFlushPreambleCache(t, false, 7)

	start := time.Now()
	body := &delayedSSEBody{chunks: []delayedSSEChunk{
		{line: `data: {"type":"response.created","response":{"id":"r1"}}`},
		{line: `data: {"type":"response.in_progress","response":{"id":"r1"}}`},
		{line: `data: {"type":"response.output_item.added","item":{"type":"message"}}`, delay: 40 * time.Millisecond},
		{line: `data: {"type":"response.completed","response":{"id":"r1","usage":{"input_tokens":1,"output_tokens":1}}}`},
	}}
	rec := &delayedFlushRecorder{ResponseRecorder: httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.UserID, int64(7)))
	c.Request = req

	clk := &openAIStreamStageClock{start: start, accountID: 1, path: "responses_http", model: "gpt-5.4"}
	clk.MarkDoStart()
	clk.MarkHeaders("rid_flush_preamble_user")
	c.Set(openAIStreamStageClockCtxKey, clk)

	svc := &OpenAIGatewayService{
		cfg: rawChatCompletionsTestConfig(),
		settingService: NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{
			SettingKeyOpenAIResponsesFlushPreamble:        "false",
			SettingKeyOpenAIResponsesFlushPreambleUserIDs: "[7]",
		}}, &config.Config{}),
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid_flush_preamble_user"}},
		Body:       io.NopCloser(body),
	}
	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, start, "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.firstTokenMs)

	_, _, firstSSE, firstUseful, firstFlush, _ := clk.stageSnapshot()
	require.GreaterOrEqual(t, firstSSE, 0)
	require.GreaterOrEqual(t, firstUseful, firstSSE)
	require.GreaterOrEqual(t, firstFlush, firstSSE)
	require.Less(t, firstFlush, firstUseful, "allowlisted user should flush preamble before delayed output_item.added")
	require.Contains(t, rec.Body.String(), `"type":"response.created"`)
	require.NotNil(t, result.trueFirstTokenMs)
	require.GreaterOrEqual(t, *result.trueFirstTokenMs, firstUseful-15)
}

func TestOpenAIForwardResult_ScheduleFirstTokenMs(t *testing.T) {
	t.Parallel()
	display := 80
	trueMs := 2400
	require.Nil(t, (*OpenAIForwardResult)(nil).ScheduleFirstTokenMs())
	require.Nil(t, (&OpenAIForwardResult{FirstTokenMs: &display}).ScheduleFirstTokenMs())
	require.Equal(t, 80, *(&OpenAIForwardResult{HopFirstTokenMs: &display}).ScheduleFirstTokenMs())
	require.Equal(t, 2400, *(&OpenAIForwardResult{FirstTokenMs: &display, TrueFirstTokenMs: &trueMs}).ScheduleFirstTokenMs())
}
