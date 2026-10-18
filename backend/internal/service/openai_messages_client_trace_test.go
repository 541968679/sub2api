//go:build unit

package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestWriteAnthropicJSONResponseReplacesUpstreamMessageID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	writeAnthropicJSONResponse(c, http.StatusOK, &apicompat.AnthropicResponse{
		ID:    "resp_upstream_secret",
		Type:  "message",
		Role:  "assistant",
		Model: "kimi-k3",
		Content: []apicompat.AnthropicContentBlock{{
			Type: "text",
			Text: "ok",
		}},
	})

	body := rec.Body.String()
	require.NotContains(t, body, "resp_upstream_secret")
	require.True(t, strings.HasPrefix(gjson.Get(body, "id").String(), "msg_"))
	require.Equal(t, "ok", gjson.Get(body, "content.0.text").String())
	require.Equal(t, "kimi-k3", gjson.Get(body, "model").String())
}

func TestMessagesStreamHidesUpstreamResponseID(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_vendor_42","model":"deepseek-v4","status":"in_progress"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"hello"}`,
		`{"type":"response.completed","response":{"id":"resp_vendor_42","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":3,"output_tokens":1,"total_tokens":4}}}`,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_vendor_42", result.ResponseID)
	require.Contains(t, rec.Body.String(), "hello")
	require.NotContains(t, rec.Body.String(), "resp_vendor_42")
	require.NotContains(t, rec.Body.String(), "deepseek-v4")
	id := gjson.Get(rec.Body.String(), "message.id").String()
	if id == "" {
		for _, line := range strings.Split(rec.Body.String(), "\n") {
			if !strings.Contains(line, "message_start") && !strings.HasPrefix(strings.TrimSpace(line), "data:") {
				continue
			}
			if got := gjson.Get(strings.TrimPrefix(strings.TrimSpace(line), "data:"), "message.id").String(); got != "" {
				id = got
				break
			}
		}
	}
	require.True(t, strings.HasPrefix(id, "msg_"), "client message id = %q", id)
}

func TestPresentAnthropicUpstreamErrorHidesHostAndDialect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	status, errType, message := PresentAnthropicUpstreamError(
		c,
		http.StatusBadGateway,
		"new_api_error",
		`upstream https://api.vendor.example/v1 failed, request id resp_abc, account "ops@example.com"`,
	)
	require.Equal(t, http.StatusBadGateway, status)
	require.Equal(t, "upstream_error", errType)
	require.Equal(t, "Upstream service temporarily unavailable", message)

	status, errType, message = PresentAnthropicUpstreamError(
		c,
		http.StatusBadRequest,
		"invalid_request_error",
		"Your input exceeds the context window of this model. See https://api.vendor.example/docs",
	)
	require.Equal(t, http.StatusRequestEntityTooLarge, status)
	require.Equal(t, "invalid_request_error", errType)
	require.True(t, strings.HasPrefix(message, "Prompt is too long"))
	require.NotContains(t, message, "vendor.example")
}

func TestMessagesStreamErrorHidesUpstreamSentence(t *testing.T) {
	_, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_err","model":"glm-5","status":"in_progress"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"partial"}`,
		`{"type":"response.failed","response":{"id":"resp_err","status":"failed","error":{"type":"new_api_error","message":"see https://relay.example/v1 request chatcmpl-vendor"},"output":[]}}`,
	)

	require.Error(t, err)
	body := rec.Body.String()
	require.Contains(t, body, "partial")
	require.Contains(t, body, "event: error")
	require.NotContains(t, body, "relay.example")
	require.NotContains(t, body, "chatcmpl-vendor")
	require.NotContains(t, body, "resp_err")
	require.NotContains(t, body, "new_api_error")
	require.NotContains(t, body, "glm-5")
}
