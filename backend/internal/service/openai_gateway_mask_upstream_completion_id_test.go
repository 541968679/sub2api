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

func TestChatCompletionIDMask_ReplacesTopLevelAndKeepsToolCallID(t *testing.T) {
	mask := &chatCompletionIDMask{}
	body := []byte(`{"id":"chatcmpl-quark-1","object":"chat.completion","model":"kimi-k3","choices":[{"message":{"tool_calls":[{"id":"call_quark_keep","type":"function","function":{"name":"run","arguments":"{}"}}]}}]}`)
	first := mask.applyBytes(body)
	second := mask.applyBytes(body)

	require.NotContains(t, gjson.GetBytes(first, "id").String(), "quark")
	require.Equal(t, gjson.GetBytes(first, "id").String(), gjson.GetBytes(second, "id").String())
	require.True(t, strings.HasPrefix(gjson.GetBytes(first, "id").String(), "chatcmpl-"))
	require.Equal(t, "call_quark_keep", gjson.GetBytes(first, "choices.0.message.tool_calls.0.id").String())
	errBody := []byte(`{"error":{"message":"upstream said no","type":"upstream_type"},"id":"chatcmpl-quark-err"}`)
	require.False(t, shouldMaskChatCompletionID(errBody))
	maskedErr := mask.applyBytes(errBody)
	require.True(t, strings.HasPrefix(gjson.GetBytes(maskedErr, "id").String(), "local-"))
	require.NotContains(t, string(maskedErr), "quark")
	require.Contains(t, string(maskedErr), "upstream said no")
}

func TestForwardAsRawChatCompletions_MasksUpstreamCompletionID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("json", func(t *testing.T) {
		body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}]}`)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"id":"chatcmpl-quark-json","object":"chat.completion","model":"kimi-k3","choices":[{"message":{"role":"assistant","content":"ok","tool_calls":[{"id":"call_keep","type":"function","function":{"name":"run","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`,
			)),
		}}
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

		_, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
		require.NoError(t, err)
		written := rec.Body.String()
		require.NotContains(t, written, "quark")
		require.True(t, strings.HasPrefix(gjson.Get(written, "id").String(), "chatcmpl-"))
		require.Equal(t, "call_keep", gjson.Get(written, "choices.0.message.tool_calls.0.id").String())
		require.Equal(t, "ok", gjson.Get(written, "choices.0.message.content").String())
	})

	t.Run("sse", func(t *testing.T) {
		body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}],"stream":true}`)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		upstreamBody := strings.Join([]string{
			`data: {"id":"chatcmpl-quark-a","object":"chat.completion.chunk","model":"kimi-k3","choices":[{"index":0,"delta":{"content":"hi"}}]}`,
			"",
			`data: {"id":"chatcmpl-quark-b","object":"chat.completion.chunk","model":"kimi-k3","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_keep","type":"function","function":{"name":"run","arguments":"{}"}}]}}]}`,
			"",
			"data: [DONE]",
			"",
		}, "\n")
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(upstreamBody)),
		}}
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

		_, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
		require.NoError(t, err)
		written := rec.Body.String()
		require.NotContains(t, written, "quark")
		require.Contains(t, written, `"id":"call_keep"`)
		var seen string
		for _, line := range strings.Split(written, "\n") {
			payload, ok := extractOpenAISSEDataLine(line)
			if !ok || strings.TrimSpace(payload) == "[DONE]" {
				continue
			}
			id := gjson.Get(payload, "id").String()
			require.True(t, strings.HasPrefix(id, "chatcmpl-"))
			if seen == "" {
				seen = id
			}
			require.Equal(t, seen, id)
		}
		require.NotEmpty(t, seen)
	})

	t.Run("sse assembled for non-stream", func(t *testing.T) {
		body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}]}`)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		upstreamBody := strings.Join([]string{
			`data: {"id":"chatcmpl-quark-sse","object":"chat.completion.chunk","model":"kimi-k3","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
			"",
			`data: {"id":"chatcmpl-quark-sse","object":"chat.completion.chunk","model":"kimi-k3","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			"",
		}, "\n")
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(upstreamBody)),
		}}
		svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

		_, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
		require.NoError(t, err)
		written := rec.Body.String()
		require.NotContains(t, written, "quark")
		require.True(t, strings.HasPrefix(gjson.Get(written, "id").String(), "chatcmpl-"))
		require.Equal(t, "ok", gjson.Get(written, "choices.0.message.content").String())
	})
}

func TestForwardAsRawChatCompletions_PassthroughMasksUpstreamCompletionID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl-quark-pass","object":"chat.completion","model":"kimi-k2","choices":[{"message":{"role":"assistant","content":"ok"}}]}`,
		)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	_, err := svc.forwardAsRawChatCompletions(context.Background(), c, openAIChatPassthroughTestAccount(), body, "gpt-4o-mini")
	require.NoError(t, err)
	written := rec.Body.String()
	require.NotContains(t, written, "quark")
	require.True(t, strings.HasPrefix(gjson.Get(written, "id").String(), "chatcmpl-"))
	require.Equal(t, "kimi-k2", gjson.Get(written, "model").String())
	require.Equal(t, "ok", gjson.Get(written, "choices.0.message.content").String())
}
