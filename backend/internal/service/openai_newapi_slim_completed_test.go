package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAINewAPISlimCompletedHelper(t *testing.T) {
	t.Parallel()

	t.Run("null usage fields are omitted and completion_tokens is never added", func(t *testing.T) {
		src := []byte(`{"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":null,"completion_tokens":null,"input_tokens_details":{"cached_tokens":2,"image_tokens":null}}}}`)
		usage, ok := extractNewAPISlimUsageFromResponsesData(src)
		require.True(t, ok)
		require.Equal(t, 10, usage.InputTokens)
		require.Equal(t, 5, usage.OutputTokens)
		require.True(t, usage.HasCached)
		require.Equal(t, 2, usage.CachedTokens)

		line := buildNewAPISlimCompletedSSELine("resp_1", usage)
		data, ok := extractOpenAISSEDataLine(line)
		require.True(t, ok)
		require.Equal(t, "response.completed", gjson.Get(data, "type").String())
		require.Equal(t, "resp_1", gjson.Get(data, "response.id").String())
		require.Equal(t, int64(10), gjson.Get(data, "response.usage.input_tokens").Int())
		require.Equal(t, int64(5), gjson.Get(data, "response.usage.output_tokens").Int())
		require.Equal(t, int64(15), gjson.Get(data, "response.usage.total_tokens").Int())
		require.Equal(t, int64(2), gjson.Get(data, "response.usage.input_tokens_details.cached_tokens").Int())
		require.False(t, gjson.Get(data, "response.usage.completion_tokens").Exists())
		require.False(t, gjson.Get(data, "response.output").Exists())
		require.NotContains(t, data, "null")
	})

	t.Run("cached_tokens omitted when missing or null", func(t *testing.T) {
		missing, ok := extractNewAPISlimUsageFromResponsesData([]byte(`{"type":"response.completed","response":{"id":"r","usage":{"input_tokens":1,"output_tokens":2}}}`))
		require.True(t, ok)
		require.False(t, missing.HasCached)
		require.False(t, gjson.GetBytes(buildNewAPISlimCompletedData("r", missing), "response.usage.input_tokens_details").Exists())

		nullable, ok := extractNewAPISlimUsageFromResponsesData([]byte(`{"type":"response.completed","response":{"id":"r","usage":{"input_tokens":1,"output_tokens":2,"input_tokens_details":{"cached_tokens":null}}}}`))
		require.True(t, ok)
		require.False(t, nullable.HasCached)
	})

	t.Run("cached_tokens zero is kept as a number", func(t *testing.T) {
		usage, ok := extractNewAPISlimUsageFromResponsesData([]byte(`{"type":"response.completed","response":{"id":"r","usage":{"input_tokens":1,"output_tokens":2,"input_tokens_details":{"cached_tokens":0}}}}`))
		require.True(t, ok)
		require.True(t, usage.HasCached)
		require.Equal(t, 0, usage.CachedTokens)
		require.Equal(t, int64(0), gjson.GetBytes(buildNewAPISlimCompletedData("r", usage), "response.usage.input_tokens_details.cached_tokens").Int())
	})

	t.Run("total_tokens is input plus output", func(t *testing.T) {
		data := buildNewAPISlimCompletedData("r", newAPISlimUsage{InputTokens: 3, OutputTokens: 7})
		require.Equal(t, int64(10), gjson.GetBytes(data, "response.usage.total_tokens").Int())
	})

	t.Run("output_tokens==0 does not slim", func(t *testing.T) {
		require.False(t, shouldSlimNewAPICompleted(true, "response.completed", 0))
		require.True(t, shouldSlimNewAPICompleted(true, "response.completed", 5))
		require.False(t, shouldSlimNewAPICompleted(false, "response.completed", 5))
		require.False(t, shouldSlimNewAPICompleted(true, "response.failed", 5))
	})

	t.Run("failed is not a soft terminal and does not synthesize", func(t *testing.T) {
		require.False(t, isNewAPISlimSoftTerminal("response.failed"))
		require.True(t, isNewAPISlimSoftTerminal("response.done"))
		require.True(t, isNewAPISlimSoftTerminal("response.incomplete"))
		require.True(t, isNewAPISlimSoftTerminal("response.cancelled"))
		require.True(t, isNewAPISlimSoftTerminal("response.canceled"))
		require.False(t, shouldSynthesizeNewAPISlimCompleted(true, false, false, false, 5))
		require.False(t, shouldSynthesizeNewAPISlimCompleted(true, false, true, true, 5))
		require.False(t, shouldSynthesizeNewAPISlimCompleted(true, true, true, false, 5))
		require.False(t, shouldSynthesizeNewAPISlimCompleted(true, false, true, false, 0))
		require.True(t, shouldSynthesizeNewAPISlimCompleted(true, false, true, false, 5))
	})
}

func fatCompletedSSE(id string, outputTokens int) string {
	return `data: {"type":"response.completed","response":{"id":"` + id + `","status":"completed","temperature":1,"max_output_tokens":128,"store":true,"reasoning":{"effort":"medium"},"output":[{"type":"message","content":[{"type":"output_text","text":"hello from completed snapshot"}]}],"usage":{"input_tokens":10,"output_tokens":` + strconv.Itoa(outputTokens) + `,"total_tokens":` + strconv.Itoa(10+outputTokens) + `,"completion_tokens":` + strconv.Itoa(outputTokens) + `,"input_tokens_details":{"cached_tokens":2}}}}`
}

func storeGatewayNewAPISlimCompletedCache(t *testing.T, enabled bool, userIDs ...int64) {
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
		slimCompleted:        enabled,
		slimCompletedUserIDs: append([]int64(nil), userIDs...),
		expiresAt:            time.Now().Add(time.Hour).UnixNano(),
	})
}

func newSlimCompletedTestService(t *testing.T, global bool, userIDs ...int64) *OpenAIGatewayService {
	t.Helper()
	storeGatewayNewAPISlimCompletedCache(t, global, userIDs...)
	parts := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	globalValue := "false"
	if global {
		globalValue = "true"
	}
	return &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				StreamDataIntervalTimeout: 0,
				StreamKeepaliveInterval:   0,
				MaxLineSize:               defaultMaxLineSize,
			},
		},
		settingService: NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{
			SettingKeyOpenAINewAPISlimCompleted:        globalValue,
			SettingKeyOpenAINewAPISlimCompletedUserIDs: "[" + strings.Join(parts, ",") + "]",
		}}, &config.Config{}),
	}
}

type rejectCompletedWriter struct {
	gin.ResponseWriter
}

func (w *rejectCompletedWriter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("response.completed")) {
		return 0, errors.New("simulated client disconnect")
	}
	return w.ResponseWriter.Write(p)
}

func (w *rejectCompletedWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func runNewAPISlimCompletedStream(t *testing.T, passthrough bool, svc *OpenAIGatewayService, ctx context.Context, body string, display bool, disconnectOnCompleted bool) (string, *OpenAIUsage, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	if display {
		SetDisplayTokenMultipliers(c, openAITestDisplayMultipliers())
	}
	if disconnectOnCompleted {
		c.Writer = &rejectCompletedWriter{ResponseWriter: c.Writer}
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}
	if passthrough {
		result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")
		if result == nil || result.usage == nil {
			return rec.Body.String(), nil, err
		}
		return rec.Body.String(), result.usage, err
	}
	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")
	if result == nil || result.usage == nil {
		return rec.Body.String(), nil, err
	}
	return rec.Body.String(), result.usage, err
}

func slimCompletedSSEBody(extra ...string) string {
	lines := []string{
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
	}
	lines = append(lines, extra...)
	return strings.Join(lines, "\n")
}

func expectedDisplaySlimUsage(t *testing.T, line string) (input, output, cached int) {
	t.Helper()
	rewritten := rewriteOpenAIResponsesSSEUsageTokens(line, openAITestDisplayMultipliers())
	data, ok := extractOpenAISSEDataLine(rewritten)
	require.True(t, ok)
	return int(gjson.Get(data, "response.usage.input_tokens").Int()),
		int(gjson.Get(data, "response.usage.output_tokens").Int()),
		int(gjson.Get(data, "response.usage.input_tokens_details.cached_tokens").Int())
}

func assertSlimCompletedShape(t *testing.T, body string, id string, input, output, cached int, hasCached bool) {
	t.Helper()
	completed := collectSSEEventsByType(body, "response.completed")
	require.Len(t, completed, 1)
	data := completed[0]
	require.Equal(t, id, gjson.Get(data, "response.id").String())
	require.Equal(t, int64(input), gjson.Get(data, "response.usage.input_tokens").Int())
	require.Equal(t, int64(output), gjson.Get(data, "response.usage.output_tokens").Int())
	require.Equal(t, int64(input+output), gjson.Get(data, "response.usage.total_tokens").Int())
	require.False(t, gjson.Get(data, "response.output").Exists())
	require.False(t, gjson.Get(data, "response.temperature").Exists())
	require.False(t, gjson.Get(data, "response.usage.completion_tokens").Exists())
	require.False(t, gjson.Get(data, "response.status").Exists())
	if hasCached {
		require.Equal(t, int64(cached), gjson.Get(data, "response.usage.input_tokens_details.cached_tokens").Int())
	} else {
		require.False(t, gjson.Get(data, "response.usage.input_tokens_details").Exists())
	}
}

func collectSSEEventsByType(body, eventType string) []string {
	var out []string
	for _, raw := range strings.Split(body, "\n") {
		data, ok := extractOpenAISSEDataLine(raw)
		if !ok {
			continue
		}
		if gjson.Get(data, "type").String() == eventType {
			out = append(out, data)
		}
	}
	return out
}

func TestOpenAINewAPISlimCompletedPassthroughAndHTTP(t *testing.T) {
	userCtx := context.WithValue(context.Background(), ctxkey.UserID, int64(220))
	fatLine := fatCompletedSSE("resp_fat", 5)
	fat := slimCompletedSSEBody(fatLine, `data: [DONE]`)
	zero := slimCompletedSSEBody(fatCompletedSSE("resp_zero", 0), `data: [DONE]`)
	doneLine := `data: {"type":"response.done","response":{"id":"resp_done","usage":{"input_tokens":10,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}}}}`
	doneOnly := slimCompletedSSEBody(doneLine, `data: [DONE]`)
	incompleteOnly := slimCompletedSSEBody(
		`data: {"type":"response.incomplete","response":{"id":"resp_incomplete","usage":{"input_tokens":10,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}}}}`,
		`data: [DONE]`,
	)
	displayDoneInput, displayDoneOutput, displayDoneCached := expectedDisplaySlimUsage(t, doneLine)
	doneNoMarker := slimCompletedSSEBody(
		`data: {"type":"response.done","response":{"id":"resp_eof","usage":{"input_tokens":10,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}}}}`,
	)
	failed := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		`data: {"type":"response.failed","response":{"id":"resp_fail","status":"failed","temperature":1,"output":[{"type":"message"}],"usage":{"input_tokens":10,"output_tokens":5},"error":{"message":"upstream failed"}}}`,
	}, "\n")
	displayInput, displayOutput, displayCached := expectedDisplaySlimUsage(t, fatLine)

	for _, passthrough := range []bool{true, false} {
		name := "http"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			t.Run("whitelist 220 slims fat completed after display rewrite", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, false, 220)
				body, usage, err := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, fat, true, false)
				require.NoError(t, err)
				require.Equal(t, 10, usage.InputTokens)
				require.Equal(t, 5, usage.OutputTokens)
				require.Equal(t, 2, usage.CacheReadInputTokens)
				assertSlimCompletedShape(t, body, "resp_fat", displayInput, displayOutput, displayCached, true)
				require.NotContains(t, body, "completion_tokens")
				require.NotContains(t, body, `"temperature"`)
			})

			t.Run("non-allowlisted user keeps fat completed", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, false, 220)
				other := context.WithValue(context.Background(), ctxkey.UserID, int64(221))
				body, usage, err := runNewAPISlimCompletedStream(t, passthrough, svc, other, fat, false, false)
				require.NoError(t, err)
				require.Equal(t, 5, usage.OutputTokens)
				require.Contains(t, body, `"temperature"`)
				require.Contains(t, body, `"output"`)
				require.Len(t, collectSSEEventsByType(body, "response.completed"), 1)
			})

			t.Run("output_tokens==0 keeps original completed", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, true)
				body, usage, err := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, zero, false, false)
				require.NoError(t, err)
				require.Equal(t, 0, usage.OutputTokens)
				require.Contains(t, body, `"temperature"`)
				require.Contains(t, body, `"output"`)
			})

			t.Run("synthesizes one slim completed before DONE", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, false, 220)
				body, usage, err := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, doneOnly, false, false)
				require.NoError(t, err)
				require.Equal(t, 5, usage.OutputTokens)
				assertSlimCompletedShape(t, body, "resp_done", 10, 5, 2, true)
				doneIdx := strings.Index(body, "data: [DONE]")
				completedIdx := strings.Index(body, `"type":"response.completed"`)
				require.GreaterOrEqual(t, doneIdx, 0)
				require.GreaterOrEqual(t, completedIdx, 0)
				require.Less(t, completedIdx, doneIdx)
			})

			t.Run("synthesized completed uses display-rewritten usage", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, false, 220)
				body, usage, err := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, doneOnly, true, false)
				require.NoError(t, err)
				require.Equal(t, 10, usage.InputTokens)
				require.Equal(t, 5, usage.OutputTokens)
				require.Equal(t, 2, usage.CacheReadInputTokens)
				assertSlimCompletedShape(t, body, "resp_done", displayDoneInput, displayDoneOutput, displayDoneCached, true)
			})

			t.Run("incomplete without completed synthesizes before DONE", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, false, 220)
				body, usage, err := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, incompleteOnly, false, false)
				require.NoError(t, err)
				require.Equal(t, 5, usage.OutputTokens)
				assertSlimCompletedShape(t, body, "resp_incomplete", 10, 5, 2, true)
				require.Less(t, strings.Index(body, `"type":"response.completed"`), strings.Index(body, "data: [DONE]"))
			})

			t.Run("synthesizes at clean EOF without DONE", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, false, 220)
				body, usage, err := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, doneNoMarker, false, false)
				require.NoError(t, err)
				require.Equal(t, 5, usage.OutputTokens)
				assertSlimCompletedShape(t, body, "resp_eof", 10, 5, 2, true)
			})

			t.Run("existing completed is not synthesized twice", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, false, 220)
				body, _, err := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, fat, false, false)
				require.NoError(t, err)
				require.Len(t, collectSSEEventsByType(body, "response.completed"), 1)
			})

			t.Run("failed event is not slimmed", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, true)
				body, _, err := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, failed, false, false)
				require.Error(t, err)
				require.Empty(t, collectSSEEventsByType(body, "response.completed"))
				failedEvents := collectSSEEventsByType(body, "response.failed")
				require.NotEmpty(t, failedEvents)
				require.Equal(t, "response.failed", gjson.Get(failedEvents[0], "type").String())
				require.True(t, gjson.Get(failedEvents[0], "response.temperature").Exists() || gjson.Get(failedEvents[0], "error").Exists())
			})

			t.Run("client disconnect does not write synthesized completed", func(t *testing.T) {
				svc := newSlimCompletedTestService(t, false, 220)
				body, usage, _ := runNewAPISlimCompletedStream(t, passthrough, svc, userCtx, doneOnly, false, true)
				require.NotNil(t, usage)
				require.Equal(t, 5, usage.OutputTokens)
				require.Empty(t, collectSSEEventsByType(body, "response.completed"))
			})
		})
	}
}
