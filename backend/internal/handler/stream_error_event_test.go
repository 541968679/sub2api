package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGinContextForEndpoint(t *testing.T, endpoint string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, endpoint, nil)
	return c, w
}

func parseResponsesFailedSSE(t *testing.T, body string) (map[string]any, map[string]any) {
	t.Helper()
	require.True(t, strings.HasPrefix(body, "event: response.failed\n"), "got: %q", body)
	require.True(t, strings.HasSuffix(body, "\n\n"))

	lines := strings.SplitN(strings.TrimSuffix(body, "\n\n"), "\n", 2)
	require.Len(t, lines, 2)
	require.True(t, strings.HasPrefix(lines[1], "data: "))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "data: ")), &parsed))
	assert.Equal(t, "response.failed", parsed["type"])
	_, hasSeq := parsed["sequence_number"]
	assert.False(t, hasSeq)

	resp, ok := parsed["response"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "response", resp["object"])
	assert.Equal(t, "failed", resp["status"])

	errObj, ok := resp["error"].(map[string]any)
	require.True(t, ok)
	return resp, errObj
}

func TestOpenAIHandleStreamingAwareError_ResponsesStreamingEmitsResponseFailed(t *testing.T) {
	c, w := newGinContextForEndpoint(t, EndpointResponses)
	h := &OpenAIGatewayHandler{}

	h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "retry later", true)

	resp, errObj := parseResponsesFailedSSE(t, w.Body.String())
	id, _ := resp["id"].(string)
	assert.True(t, strings.HasPrefix(id, "resp_"))
	assert.Equal(t, "rate_limit_exceeded", errObj["code"])
	assert.Equal(t, "retry later", errObj["message"])
}

func TestOpenAIHandleStreamingAwareError_ResponsesStreamingIncludesModelAndRequestID(t *testing.T) {
	c, w := newGinContextForEndpoint(t, EndpointResponses)
	c.Request = c.Request.WithContext(context.WithValue(
		c.Request.Context(),
		ctxkey.RequestID,
		"fd277bc5-ff7e-45d1-8aa9-f54e1df318f1",
	))
	setOpsRequestContext(c, "gpt-5.5", true, []byte(`{"model":"gpt-5.5"}`))
	h := &OpenAIGatewayHandler{}

	h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", "boom", true)

	resp, _ := parseResponsesFailedSSE(t, w.Body.String())
	assert.Equal(t, "resp_fd277bc5ff7e45d18aa9f54e1df318f1", resp["id"])
	assert.Equal(t, "gpt-5.5", resp["model"])
}

func TestOpenAIHandleStreamingAwareError_ChatCompletionsStreamingKeepsLegacy(t *testing.T) {
	c, w := newGinContextForEndpoint(t, EndpointChatCompletions)
	h := &OpenAIGatewayHandler{}

	h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", "boom", true)

	assert.True(t, strings.HasPrefix(w.Body.String(), "event: error\n"), "got: %q", w.Body.String())
}

func TestGatewayHandleStreamingAwareError_ResponsesStreamingEmitsResponseFailed(t *testing.T) {
	c, w := newGinContextForEndpoint(t, EndpointResponses)
	h := &GatewayHandler{}

	h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", "upstream gone", true)

	_, errObj := parseResponsesFailedSSE(t, w.Body.String())
	assert.Equal(t, "upstream_error", errObj["code"])
	assert.Equal(t, "upstream gone", errObj["message"])
}

func TestInboundIsResponses_CoversAllRoutes(t *testing.T) {
	cases := []struct {
		route string
		want  bool
	}{
		{"/v1/responses", true},
		{"/v1/responses/compact", true},
		{"/responses", true},
		{"/responses/compact", true},
		{"/backend-api/codex/responses", true},
		{"/backend-api/codex/responses/compact", true},
		{"/v1/chat/completions", false},
		{"/v1/messages", false},
		{"/", false},
		{"/responses-fake", false},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			c, _ := newGinContextForEndpoint(t, tc.route)
			assert.Equal(t, tc.want, inboundIsResponses(c))
		})
	}
}

func TestOpenAIForwardErrorAlreadyCommunicated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointResponses, nil)

	before := c.Writer.Size()
	_, _ = c.Writer.WriteString("event: response.failed\n")

	assert.True(t, openAIForwardErrorAlreadyCommunicated(c, before, errors.New("upstream response failed: policy blocked")))
	assert.True(t, openAIForwardErrorAlreadyCommunicated(c, before, errors.New("non-streaming openai protocol error: response.failed")))
	assert.False(t, openAIForwardErrorAlreadyCommunicated(c, before, errors.New("temporary network failure")))
	assert.False(t, openAIForwardErrorAlreadyCommunicated(c, c.Writer.Size(), errors.New("upstream response failed: policy blocked")))
}
