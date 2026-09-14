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

func TestForwardAsRawChatCompletions_StripsEmptyToolCallIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"weather"}],"stream":true}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_example","type":"function","function":{"name":"web_search","arguments":""}}]}}]}`,
		"",
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"","type":"function","function":{"name":"","arguments":"{\"query\":"}}]}}]}`,
		"",
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"","type":"function","function":{"name":"","arguments":"\"example\"}"}}]}}]}`,
		"",
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"deepseek-v4-flash","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_tool_identity"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          rawChatCompletionsTestConfig(),
		httpUpstream: upstream,
	}
	account := rawChatCompletionsTestAccount()

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	downstream := rec.Body.String()
	require.Contains(t, downstream, `"id":"call_example"`)
	require.Contains(t, downstream, `"name":"web_search"`)
	require.Contains(t, downstream, `{\"query\":`)
	require.Contains(t, downstream, `\"example\"}`)
	require.Contains(t, downstream, "data: [DONE]")
	require.NotContains(t, downstream, `"id":""`)
	require.NotContains(t, downstream, `"name":""`)

	followUpSeen := false
	for _, line := range strings.Split(downstream, "\n") {
		payload, ok := extractOpenAISSEDataLine(line)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(payload)
		if trimmed == "" || trimmed == "[DONE]" {
			continue
		}
		delta := gjson.Get(payload, "choices.0.delta")
		if !delta.Exists() || !delta.Get("tool_calls").Exists() {
			continue
		}
		id := delta.Get("tool_calls.0.id")
		if id.String() == "call_example" {
			require.Equal(t, "web_search", delta.Get("tool_calls.0.function.name").String())
			continue
		}
		require.False(t, id.Exists(), "empty id must be stripped: %s", payload)
		require.False(t, delta.Get("tool_calls.0.function.name").Exists(), "empty name must be stripped: %s", payload)
		require.NotEmpty(t, delta.Get("tool_calls.0.function.arguments").String())
		followUpSeen = true
	}
	require.True(t, followUpSeen)
}
