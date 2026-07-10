//go:build unit

package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func messagesResponsesSSE(events ...string) string {
	var b strings.Builder
	for _, event := range events {
		fmt.Fprintf(&b, "data: %s\n\n", event)
	}
	return b.String()
}

func messagesTestStream(t *testing.T, events ...string) (*OpenAIGatewayService, *gin.Context, *httptest.ResponseRecorder, *http.Response, *Account) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Request-Id": []string{"rid-messages-regression"}},
		Body:       io.NopCloser(strings.NewReader(messagesResponsesSSE(events...))),
	}
	return &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}, c, rec, resp, rawChatCompletionsTestAccount()
}

func handleMessagesTestStream(t *testing.T, events ...string) (*OpenAIForwardResult, error, *httptest.ResponseRecorder) {
	t.Helper()
	svc, c, rec, resp, account := messagesTestStream(t, events...)
	result, err := svc.handleAnthropicStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)
	return result, err, rec
}

func requireMessagesFailoverError(t *testing.T, err error) *UpstreamFailoverError {
	t.Helper()
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr), "expected UpstreamFailoverError, got %T: %v", err, err)
	return failoverErr
}

func TestOpenAIMessagesBufferedResponseFailedDoesNotBecomeEmptySuccess(t *testing.T) {
	svc, c, rec, resp, account := messagesTestStream(t,
		`{"type":"response.failed","response":{"id":"resp_failed","status":"failed","error":{"code":"rate_limit_error","message":"Rate limit reached"},"output":[],"usage":{"input_tokens":81443,"output_tokens":0,"total_tokens":81443}}}`,
	)

	result, err := svc.handleAnthropicBufferedStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.Nil(t, result)
	failoverErr := requireMessagesFailoverError(t, err)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.False(t, rec.Result().StatusCode == http.StatusOK && strings.Contains(rec.Body.String(), `"content":[{"type":"text","text":""}]`))
}

func TestOpenAIMessagesStreamContextLengthBeforeVisibleOutputIsClientError(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_context","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.failed","response":{"id":"resp_context","status":"failed","error":{"code":"context_length_exceeded","message":"Your input exceeds the context window of this model."},"output":[],"usage":{"input_tokens":271533,"output_tokens":0,"total_tokens":271533}}}`,
	)

	failoverErr := requireMessagesFailoverError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "context window")
	require.False(t, rec.Result().StatusCode == http.StatusOK && strings.Contains(rec.Body.String(), "message_stop"))
}

func TestOpenAIMessagesStreamIncompleteWithoutVisibleOutputIsClientError(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_incomplete","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.incomplete","response":{"id":"resp_incomplete","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"reasoning","summary":[]}],"usage":{"input_tokens":69229,"output_tokens":68,"total_tokens":69297}}}`,
	)

	failoverErr := requireMessagesFailoverError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "max_output_tokens")
	require.False(t, rec.Result().StatusCode == http.StatusOK && strings.Contains(rec.Body.String(), "message_stop"))
}

func TestOpenAIMessagesStreamReasoningOnlyTriggersFailoverBeforeEmptyReply(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_reasoning","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"internal reasoning"}`,
		`{"type":"response.reasoning_summary_text.done","output_index":0}`,
		`{"type":"response.completed","response":{"id":"resp_reasoning","status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"internal reasoning"}]}],"usage":{"input_tokens":68328,"output_tokens":24,"total_tokens":68352}}}`,
	)

	failoverErr := requireMessagesFailoverError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Empty(t, rec.Body.String(), "reasoning-only preamble must stay buffered so another account can be tried")
}

func TestOpenAIMessagesStreamReplaysTerminalOnlyText(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_text","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.completed","response":{"id":"resp_text","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"still here"}]}],"usage":{"input_tokens":69000,"output_tokens":3,"total_tokens":69003}}}`,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "still here")
	require.Contains(t, rec.Body.String(), "event: message_stop")
}

func TestOpenAIMessagesStreamReplaysTerminalOnlyToolCall(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_tool","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.completed","response":{"id":"resp_tool","status":"completed","output":[{"type":"function_call","call_id":"call_terminal","name":"Read","arguments":"{\"file_path\":\"README.md\"}"}],"usage":{"input_tokens":69000,"output_tokens":4,"total_tokens":69004}}}`,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"tool_use"`)
	require.Contains(t, rec.Body.String(), `"name":"Read"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"tool_use"`)
}

func TestOpenAIMessagesStreamNormalIncrementalToolCallRemainsUsable(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_tool_delta","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_delta","name":"Read","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"file_path\":\"README.md\"}"}`,
		`{"type":"response.function_call_arguments.done","output_index":0,"arguments":"{\"file_path\":\"README.md\"}"}`,
		`{"type":"response.completed","response":{"id":"resp_tool_delta","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105}}}`,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"tool_use"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"tool_use"`)
}

func TestOpenAIMessagesStreamFailureAfterVisibleOutputDoesNotBecomeRetryable(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_partial","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"partial answer"}`,
		`{"type":"response.failed","response":{"id":"resp_partial","status":"failed","error":{"code":"server_error","message":"processing stopped"},"output":[],"usage":{"input_tokens":100,"output_tokens":3,"total_tokens":103}}}`,
	)

	require.Error(t, err)
	require.NotNil(t, result)
	require.True(t, result.ClientOutputStarted)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "partial visible output must not be replayed on another account")
	require.Contains(t, rec.Body.String(), "partial answer")
	require.Contains(t, rec.Body.String(), "event: error")
}

func TestOpenAIMessagesStreamPreVisibleKeepaliveDoesNotCommitFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Request-Id": []string{"rid-pre-visible-keepalive"}},
		Body:       pr,
	}
	cfg := rawChatCompletionsTestConfig()
	cfg.Gateway.StreamKeepaliveInterval = 1
	svc := &OpenAIGatewayService{cfg: cfg}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = fmt.Fprint(pw, messagesResponsesSSE(
			`{"type":"response.created","response":{"id":"resp_keepalive","model":"gpt-5.5","status":"in_progress"}}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
		))
		for range 5 {
			time.Sleep(300 * time.Millisecond)
			_, _ = fmt.Fprint(pw, messagesResponsesSSE(
				`{"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"internal reasoning"}`,
			))
		}
		_, _ = fmt.Fprint(pw, messagesResponsesSSE(
			`{"type":"response.failed","response":{"id":"resp_keepalive","status":"failed","error":{"code":"rate_limit_error","message":"temporary limit"},"output":[]}}`,
		))
	}()

	result, err := svc.handleAnthropicStreamingResponse(
		resp, c, rawChatCompletionsTestAccount(), true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	require.NotNil(t, result)
	require.False(t, result.ClientOutputStarted)
	require.NotNil(t, requireMessagesFailoverError(t, err))
	require.True(t, OpenAIAnthropicTransportStreamStarted(c))
	require.Contains(t, rec.Body.String(), "event: ping")
	require.NotContains(t, rec.Body.String(), "internal reasoning")
}
