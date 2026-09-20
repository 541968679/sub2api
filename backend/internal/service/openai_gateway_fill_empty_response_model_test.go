//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFillEmptyOpenAIResponseModel(t *testing.T) {
	t.Parallel()

	t.Run("fills empty string", func(t *testing.T) {
		t.Parallel()
		got, changed := fillEmptyOpenAIResponseModel(
			[]byte(`{"id":"chatcmpl_1","object":"chat.completion.chunk","model":"","choices":[{"index":0,"delta":{"content":"hi"}}]}`),
			"kimi-k3",
		)
		require.True(t, changed)
		require.Equal(t, "kimi-k3", gjson.GetBytes(got, "model").String())
		require.Equal(t, "hi", gjson.GetBytes(got, "choices.0.delta.content").String())
	})

	t.Run("fills missing model on chat chunk", func(t *testing.T) {
		t.Parallel()
		got, changed := fillEmptyOpenAIResponseModel(
			[]byte(`{"id":"chatcmpl_1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hi"}}]}`),
			"kimi-k3",
		)
		require.True(t, changed)
		require.Equal(t, "kimi-k3", gjson.GetBytes(got, "model").String())
	})

	t.Run("fills whitespace and null", func(t *testing.T) {
		t.Parallel()
		ws, wsChanged := fillEmptyOpenAIResponseModel([]byte(`{"object":"chat.completion","model":"  "}`), "gpt-5.4")
		require.True(t, wsChanged)
		require.Equal(t, "gpt-5.4", gjson.GetBytes(ws, "model").String())

		nul, nulChanged := fillEmptyOpenAIResponseModel([]byte(`{"object":"chat.completion","model":null}`), "gpt-5.4")
		require.True(t, nulChanged)
		require.Equal(t, "gpt-5.4", gjson.GetBytes(nul, "model").String())
	})

	t.Run("preserves non-empty upstream model", func(t *testing.T) {
		t.Parallel()
		in := []byte(`{"object":"chat.completion.chunk","model":"kimi-k2","choices":[]}`)
		got, changed := fillEmptyOpenAIResponseModel(in, "kimi-k3")
		require.False(t, changed)
		require.Equal(t, "kimi-k2", gjson.GetBytes(got, "model").String())
	})

	t.Run("does not inject model onto error payloads", func(t *testing.T) {
		t.Parallel()
		in := []byte(`{"error":{"message":"nope","type":"api_error"}}`)
		got, changed := fillEmptyOpenAIResponseModel(in, "kimi-k3")
		require.False(t, changed)
		require.Equal(t, string(in), string(got))
	})

	t.Run("fills empty response.model", func(t *testing.T) {
		t.Parallel()
		got, changed := fillEmptyOpenAIResponseModel(
			[]byte(`{"type":"response.created","response":{"id":"resp_1","model":""}}`),
			"gpt-5.4",
		)
		require.True(t, changed)
		require.Equal(t, "gpt-5.4", gjson.GetBytes(got, "response.model").String())
		require.False(t, gjson.GetBytes(got, "model").Exists())
	})
}

func TestFillEmptyOpenAIResponseModelInSSELine(t *testing.T) {
	t.Parallel()

	require.Equal(t, "data: [DONE]", fillEmptyOpenAIResponseModelInSSELine("data: [DONE]", "kimi-k3"))
	require.Equal(t, "event: ping", fillEmptyOpenAIResponseModelInSSELine("event: ping", "kimi-k3"))

	got := fillEmptyOpenAIResponseModelInSSELine(
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"","choices":[{"index":0,"delta":{"content":"hi"}}]}`,
		"kimi-k3",
	)
	require.True(t, strings.HasPrefix(got, "data: "))
	payload, ok := extractOpenAISSEDataLine(got)
	require.True(t, ok)
	require.Equal(t, "kimi-k3", gjson.Get(payload, "model").String())
	require.Equal(t, "hi", gjson.Get(payload, "choices.0.delta.content").String())
}

func TestForwardAsRawChatCompletions_FillsEmptyStreamModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	beginUpstreamResponseModelObservation(c)

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"","choices":[{"index":0,"delta":{"content":"hello"}}]}`,
		"",
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_empty_model"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	downstream := rec.Body.String()
	require.Contains(t, downstream, "data: [DONE]")
	var sawFilled bool
	for _, line := range strings.Split(downstream, "\n") {
		payload, ok := extractOpenAISSEDataLine(line)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(payload)
		if trimmed == "" || trimmed == "[DONE]" {
			continue
		}
		require.Equal(t, "kimi-k3", gjson.Get(payload, "model").String(), payload)
		if gjson.Get(payload, "choices.0.delta.content").String() == "hello" {
			sawFilled = true
		}
	}
	require.True(t, sawFilled)
	require.Equal(t, "", observedUpstreamResponseModel(c), "audit must see the empty upstream model, not the filled client value")
}

func TestForwardAsRawChatCompletions_FillsEmptyJSONModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	beginUpstreamResponseModelObservation(c)

	upstreamJSON := `{"id":"chatcmpl_json","object":"chat.completion","model":"","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_empty_json_model"}},
		Body:       io.NopCloser(strings.NewReader(upstreamJSON)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "kimi-k3", gjson.Get(rec.Body.String(), "model").String())
	require.Equal(t, "pong", gjson.Get(rec.Body.String(), "choices.0.message.content").String())
	require.Equal(t, "", observedUpstreamResponseModel(c))
}

func TestForwardAsRawChatCompletions_PreservesUpstreamStreamModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"kimi-k2","choices":[{"index":0,"delta":{"content":"hello"}}]}`,
		"",
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"kimi-k2","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_keep_model"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"model":"kimi-k2"`)
	require.NotContains(t, rec.Body.String(), `"model":"kimi-k3"`)
}
