//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const testClaudeCodePromptTooLongPrefix = "Prompt is too long"

func markPromptTooLongBridge(c interface{ Set(string, any) }) {
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)
}

func requirePromptTooLongContract(t *testing.T, status int, body []byte) {
	t.Helper()
	require.Equal(t, http.StatusRequestEntityTooLarge, status)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(body, "error.type").String())
	message := gjson.GetBytes(body, "error.message").String()
	require.True(t, strings.HasPrefix(message, testClaudeCodePromptTooLongPrefix), message)
	require.Contains(t, strings.ToLower(message), "context window")
}

func promptTooLongFailedEvent(message string) string {
	return `{"type":"response.failed","response":{"id":"resp_context","status":"failed","error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"` + message + `"},"output":[],"usage":{"input_tokens":271533,"output_tokens":0,"total_tokens":271533}}}`
}

func TestClaudeGPTBridgeBufferedContextLengthUsesPromptTooLongContract(t *testing.T) {
	svc, c, _, resp, account := messagesTestStream(t,
		promptTooLongFailedEvent("Your input exceeds the context window of this model."),
	)
	markPromptTooLongBridge(c)

	result, err := svc.handleAnthropicBufferedStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.Nil(t, result)
	failoverErr := requireMessagesFailoverError(t, err)
	requirePromptTooLongContract(t, failoverErr.StatusCode, failoverErr.ResponseBody)
}

func TestClaudeGPTBridgeContextLengthMessageFallbackUsesPromptTooLongContract(t *testing.T) {
	svc, c, _, resp, account := messagesTestStream(t,
		`{"type":"response.failed","response":{"id":"resp_context","status":"failed",`+
			`"error":{"type":"invalid_request_error","message":"The maximum context length was exceeded."},`+
			`"output":[]}}`,
	)
	markPromptTooLongBridge(c)

	result, err := svc.handleAnthropicBufferedStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.Nil(t, result)
	failoverErr := requireMessagesFailoverError(t, err)
	requirePromptTooLongContract(t, failoverErr.StatusCode, failoverErr.ResponseBody)
}

func TestClaudeGPTBridgeStreamingContextLengthUsesPromptTooLongContractBeforeVisibleOutput(t *testing.T) {
	svc, c, rec, resp, account := messagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_context","model":"gpt-5.5","status":"in_progress"}}`,
		promptTooLongFailedEvent("Your input exceeds the context window of this model."),
	)
	markPromptTooLongBridge(c)

	result, err := svc.handleAnthropicStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.NotNil(t, result)
	require.False(t, result.ClientOutputStarted)
	require.Empty(t, rec.Body.String())
	failoverErr := requireMessagesFailoverError(t, err)
	requirePromptTooLongContract(t, failoverErr.StatusCode, failoverErr.ResponseBody)
}

func TestClaudeGPTBridgePromptTooLongOverridesConfiguredPassthroughRule(t *testing.T) {
	svc, c, _, resp, account := messagesTestStream(t,
		promptTooLongFailedEvent("Your input exceeds the context window of this model."),
	)
	markPromptTooLongBridge(c)
	responseStatus := http.StatusBadRequest
	customMessage := "legacy context window message"
	rule := &model.ErrorPassthroughRule{
		ID:              101,
		Name:            "legacy context window override",
		Enabled:         true,
		Priority:        1,
		Platforms:       []string{PlatformOpenAI},
		ErrorCodes:      []int{http.StatusBadRequest},
		Keywords:        []string{"context_length_exceeded"},
		MatchMode:       model.MatchModeAll,
		ResponseCode:    &responseStatus,
		CustomMessage:   &customMessage,
		PassthroughBody: false,
	}
	passthrough := &ErrorPassthroughService{}
	passthrough.setLocalCache([]*model.ErrorPassthroughRule{rule})
	BindErrorPassthroughService(c, passthrough)

	result, err := svc.handleAnthropicBufferedStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.Nil(t, result)
	failoverErr := requireMessagesFailoverError(t, err)
	requirePromptTooLongContract(t, failoverErr.StatusCode, failoverErr.ResponseBody)
}

func TestClaudeGPTBridgeDirectHTTPContextLengthUsesPromptTooLongContract(t *testing.T) {
	rec, c := responseFailedRecorder(t, "/v1/messages", []byte(`{"model":"claude-opus-4-8","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`))
	markPromptTooLongBridge(c)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"rid-http-context"}},
		Body: io.NopCloser(bytes.NewBufferString(
			`{"error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"Your input exceeds the context window of this model."}}`,
		)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsAnthropic(
		context.Background(), c, rawChatCompletionsTestAccount(),
		[]byte(`{"model":"claude-opus-4-8","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`),
		"", "gpt-5.5",
	)

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusRequestEntityTooLarge, c.Writer.Status())
	body := rec.Body.Bytes()
	require.Equal(t, "error", gjson.GetBytes(body, "type").String())
	requirePromptTooLongContract(t, c.Writer.Status(), body)
}

func TestClaudeGPTBridgeDirectHTTP413WithoutErrorCodeUsesPromptTooLongContract(t *testing.T) {
	rec, c := responseFailedRecorder(t, "/v1/messages", []byte(`{"model":"claude-sonnet-5","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`))
	markPromptTooLongBridge(c)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusRequestEntityTooLarge,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"rid-http-413-context"}},
		Body: io.NopCloser(bytes.NewBufferString(
			`{"error":{"message":"Prompt is too long: this request exceeds the context window for the selected model.","type":"invalid_request_error"},"type":"error"}`,
		)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsAnthropic(
		context.Background(), c, rawChatCompletionsTestAccount(),
		[]byte(`{"model":"claude-sonnet-5","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`),
		"", "gpt-5.6-terra",
	)

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusRequestEntityTooLarge, c.Writer.Status())
	body := rec.Body.Bytes()
	require.Equal(t, "error", gjson.GetBytes(body, "type").String())
	requirePromptTooLongContract(t, c.Writer.Status(), body)
}

func TestClaudeGPTBridgeDirectHTTP413WithoutContextMeaningIsNotPromptTooLong(t *testing.T) {
	rec, c := responseFailedRecorder(t, "/v1/messages", []byte(`{"model":"claude-sonnet-5","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`))
	markPromptTooLongBridge(c)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusRequestEntityTooLarge,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"rid-http-413-body"}},
		Body: io.NopCloser(bytes.NewBufferString(
			`{"error":{"message":"Request body exceeds the configured byte limit.","type":"invalid_request_error"},"type":"error"}`,
		)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsAnthropic(
		context.Background(), c, rawChatCompletionsTestAccount(),
		[]byte(`{"model":"claude-sonnet-5","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`),
		"", "gpt-5.6-terra",
	)

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusRequestEntityTooLarge, c.Writer.Status())
	require.NotContains(t, rec.Body.String(), testClaudeCodePromptTooLongPrefix)
}

func TestNonBridgeMessagesContextLengthKeepsExistingClientError(t *testing.T) {
	svc, c, _, resp, account := messagesTestStream(t,
		promptTooLongFailedEvent("Your input exceeds the context window of this model."),
	)

	result, err := svc.handleAnthropicBufferedStreamingResponse(
		resp, c, account, false,
		"gpt-5.5", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.Nil(t, result)
	failoverErr := requireMessagesFailoverError(t, err)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.False(t, strings.HasPrefix(
		gjson.GetBytes(failoverErr.ResponseBody, "error.message").String(),
		testClaudeCodePromptTooLongPrefix,
	))
}

func TestClaudeGPTBridgeContextLengthAfterVisibleOutputDoesNotInviteReplay(t *testing.T) {
	svc, c, rec, resp, account := messagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_partial","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"partial answer"}`,
		promptTooLongFailedEvent("Your input exceeds the context window of this model."),
	)
	markPromptTooLongBridge(c)

	result, err := svc.handleAnthropicStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.NotNil(t, result)
	require.True(t, result.ClientOutputStarted)
	require.Contains(t, rec.Body.String(), "partial answer")
	require.Contains(t, rec.Body.String(), "event: error")
	require.NotContains(t, rec.Body.String(), testClaudeCodePromptTooLongPrefix)
}

func TestClaudeGPTBridgeStreamingPromptTooLongOverridesConfiguredPassthroughRule(t *testing.T) {
	svc, c, _, resp, account := messagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_context","model":"gpt-5.5","status":"in_progress"}}`,
		promptTooLongFailedEvent("Your input exceeds the context window of this model."),
	)
	markPromptTooLongBridge(c)
	responseStatus := http.StatusBadRequest
	customMessage := "legacy stream context message"
	rule := &model.ErrorPassthroughRule{
		ID:              102,
		Name:            "legacy stream context override",
		Enabled:         true,
		Priority:        1,
		Platforms:       []string{PlatformOpenAI},
		ErrorCodes:      []int{http.StatusBadRequest},
		Keywords:        []string{"context_length_exceeded"},
		MatchMode:       model.MatchModeAll,
		ResponseCode:    &responseStatus,
		CustomMessage:   &customMessage,
		PassthroughBody: false,
	}
	passthrough := &ErrorPassthroughService{}
	passthrough.setLocalCache([]*model.ErrorPassthroughRule{rule})
	BindErrorPassthroughService(c, passthrough)

	result, err := svc.handleAnthropicStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.NotNil(t, result)
	failoverErr := requireMessagesFailoverError(t, err)
	requirePromptTooLongContract(t, failoverErr.StatusCode, failoverErr.ResponseBody)
}
