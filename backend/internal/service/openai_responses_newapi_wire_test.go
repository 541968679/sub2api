package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAIResponsesNewAPIWireData_CoercesFloatCreatedAtOnly(t *testing.T) {
	t.Parallel()

	floatEvent := []byte(`{"type":"response.completed","response":{"id":"resp_1","created_at":1785990796.0,"usage":{"input_tokens":10,"output_tokens":232,"reasoning_tokens":5}}}`)
	next, changed := normalizeOpenAIResponsesNewAPIWireData(floatEvent)
	require.True(t, changed)
	require.Equal(t, "1785990796", gjson.GetBytes(next, "response.created_at").Raw)
	require.Equal(t, int64(10), gjson.GetBytes(next, "response.usage.input_tokens").Int())
	require.Equal(t, int64(232), gjson.GetBytes(next, "response.usage.output_tokens").Int())
	require.Equal(t, int64(5), gjson.GetBytes(next, "response.usage.reasoning_tokens").Int())
	require.False(t, gjson.GetBytes(next, "response.usage.output_tokens_details").Exists())
	require.NotContains(t, string(next), "1785990796.0")

	topLevel := []byte(`{"id":"resp_1","created_at":1785990796.5,"usage":{"input_tokens":1,"output_tokens":2}}`)
	next, changed = normalizeOpenAIResponsesNewAPIWireData(topLevel)
	require.True(t, changed)
	require.Equal(t, "1785990796", gjson.GetBytes(next, "created_at").Raw)

	exponent := []byte(`{"created_at":1.785990796e9}`)
	next, changed = normalizeOpenAIResponsesNewAPIWireData(exponent)
	require.True(t, changed)
	require.Equal(t, "1785990796", gjson.GetBytes(next, "created_at").Raw)

	integer := []byte(`{"type":"response.created","response":{"created_at":1785990796,"usage":{"input_tokens":1,"output_tokens":2}}}`)
	next, changed = normalizeOpenAIResponsesNewAPIWireData(integer)
	require.False(t, changed)
	require.Equal(t, integer, next)

	noTimestamp := []byte(`{"type":"response.output_text.delta","delta":"created_at is just text"}`)
	next, changed = normalizeOpenAIResponsesNewAPIWireData(noTimestamp)
	require.False(t, changed)
	require.Equal(t, noTimestamp, next)
}

func TestNormalizeOpenAIResponsesNewAPIWireLine_PreservesPrefixAndDone(t *testing.T) {
	t.Parallel()

	line := `data: {"type":"response.completed","response":{"created_at":1785990796.0}}`
	rewritten := normalizeOpenAIResponsesNewAPIWireLine(line)
	require.True(t, strings.HasPrefix(rewritten, "data: "))
	data, ok := extractOpenAISSEDataLine(rewritten)
	require.True(t, ok)
	require.Equal(t, "1785990796", gjson.Get(data, "response.created_at").Raw)

	require.Equal(t, "data: [DONE]", normalizeOpenAIResponsesNewAPIWireLine("data: [DONE]"))
	unchanged := `data: {"type":"response.output_text.delta","delta":"hi"}`
	require.Equal(t, unchanged, normalizeOpenAIResponsesNewAPIWireLine(unchanged))
	require.Equal(t, "event: response.completed", normalizeOpenAIResponsesNewAPIWireLine("event: response.completed"))
}

func TestApplyOpenAIResponsesNewAPIWireBody_JSONAndSSE(t *testing.T) {
	t.Parallel()

	jsonBody := []byte("  {\"id\":\"resp_1\",\"created_at\":1785990796.0,\"usage\":{\"output_tokens\":4}}")
	rewritten := applyOpenAIResponsesNewAPIWireBody(jsonBody)
	require.Equal(t, "1785990796", gjson.GetBytes(rewritten, "created_at").Raw)
	require.Equal(t, int64(4), gjson.GetBytes(rewritten, "usage.output_tokens").Int())

	sse := "data: {\"type\":\"response.created\",\"response\":{\"created_at\":1785990796}}\n\n"
	require.Equal(t, sse, string(applyOpenAIResponsesNewAPIWireBody([]byte(sse))))
}

func TestResponsesNewAPIWire_AppendsDoneOnlyAfterTerminalEvent(t *testing.T) {
	completed := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		`data: {"type":"response.completed","response":{"id":"resp_ok","created_at":1785990796.0,"usage":{"input_tokens":9,"output_tokens":4}}}`,
		"",
	}, "\n")
	withDone := completed + "data: [DONE]\n\n"
	truncated := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"h\"}\n\n"
	failed := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"partial"}`,
		`data: {"type":"response.failed","response":{"id":"resp_fail","error":{"message":"policy block"}}}`,
		"",
	}, "\n")

	for _, passthrough := range []bool{true, false} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name+" completed without done", func(t *testing.T) {
			body, usage, err := runResponsesNewAPIWireStream(t, passthrough, completed)
			require.NoError(t, err)
			require.NotNil(t, usage)
			require.Equal(t, 9, usage.InputTokens)
			require.Equal(t, 4, usage.OutputTokens)
			require.NotContains(t, body, "1785990796.0")
			require.Contains(t, body, `"created_at":1785990796`)
			require.Equal(t, 1, strings.Count(body, "data: [DONE]"))
		})
		t.Run(name+" keeps a single done", func(t *testing.T) {
			body, _, err := runResponsesNewAPIWireStream(t, passthrough, withDone)
			require.NoError(t, err)
			require.Equal(t, 1, strings.Count(body, "data: [DONE]"))
			require.NotContains(t, body, "1785990796.0")
		})
		t.Run(name+" truncated stream stays incomplete", func(t *testing.T) {
			body, _, err := runResponsesNewAPIWireStream(t, passthrough, truncated)
			require.Error(t, err)
			require.Contains(t, err.Error(), "missing terminal event")
			require.NotContains(t, body, "data: [DONE]")
		})
		t.Run(name+" failed terminal still ends with done", func(t *testing.T) {
			body, _, err := runResponsesNewAPIWireStream(t, passthrough, failed)
			require.Error(t, err)
			require.Contains(t, err.Error(), "upstream response failed")
			require.Contains(t, body, "response.failed")
			require.Equal(t, 1, strings.Count(body, "data: [DONE]"))
		})
	}
}

func runResponsesNewAPIWireStream(t *testing.T, passthrough bool, body string) (string, *OpenAIUsage, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}
	if passthrough {
		result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
		if result == nil || result.usage == nil {
			return rec.Body.String(), nil, err
		}
		return rec.Body.String(), result.usage, err
	}
	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
	if result == nil || result.usage == nil {
		return rec.Body.String(), nil, err
	}
	return rec.Body.String(), result.usage, err
}
