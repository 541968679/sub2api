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
	"github.com/tidwall/gjson"
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
	// 客户端文案已中立化，不再点名 OpenAI 专有参数 max_output_tokens。
	require.Contains(t, string(failoverErr.ResponseBody), "maximum output length")
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
	require.True(t, failoverErr.NoAccountFailover)
	require.Equal(t, "api_error", gjson.GetBytes(failoverErr.ResponseBody, "error.type").String())
	require.Empty(t, rec.Body.String(), "reasoning-only preamble must stay buffered so another account can be tried")
}

func TestOpenAIMessagesStreamContentPartWithoutDeltasIsVisible(t *testing.T) {
	// Production gpt-5.6-terra / gpt-5.5 bridge shape: no output_text.delta,
	// text lives on content_part, output_text.done and completed.output are empty.
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_terra","model":"gpt-5.6-terra","status":"in_progress"}}`,
		`{"type":"response.in_progress","response":{"id":"resp_terra","status":"in_progress"}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
		`{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`,
		`{"type":"response.content_part.added","output_index":1,"content_index":0,"part":{"type":"output_text","text":"usable terra reply"}}`,
		`{"type":"response.output_text.done","output_index":1,"content_index":0,"text":""}`,
		`{"type":"response.content_part.done","output_index":1,"content_index":0,"part":{"type":"output_text","text":"usable terra reply"}}`,
		`{"type":"response.output_item.done","output_index":1,"item":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":""}]}}`,
		`{"type":"response.completed","response":{"id":"resp_terra","status":"completed","output":[{"type":"reasoning","summary":[]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":""}]}],"usage":{"input_tokens":23783,"output_tokens":29,"total_tokens":23812}}}`,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "usable terra reply")
	require.Contains(t, rec.Body.String(), "event: message_stop")
	require.NotContains(t, rec.Body.String(), "without assistant content")
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

func requireClaudeCodeAnthropicSSEStateMachine(t *testing.T, sse string) {
	t.Helper()
	hasMessage := false
	hasCurrentBlock := false
	for _, line := range strings.Split(sse, "\n") {
		if !strings.HasPrefix(line, "event: ") {
			continue
		}
		evt := strings.TrimSpace(strings.TrimPrefix(line, "event: "))
		switch evt {
		case "ping":
			continue
		case "message_start":
			hasMessage = true
		case "content_block_start":
			require.True(t, hasMessage, "content_block_start without a current message")
			hasCurrentBlock = true
		case "content_block_delta":
			require.True(t, hasMessage, "Received content_block_delta without a current message")
			require.True(t, hasCurrentBlock, "content_block_delta without a current content block")
		case "content_block_stop":
			hasCurrentBlock = false
		case "message_stop":
			hasMessage = false
			hasCurrentBlock = false
		case "error":
			hasMessage = false
			hasCurrentBlock = false
		}
	}
}

func firstAnthropicSSEEvent(sse string) string {
	for _, line := range strings.Split(sse, "\n") {
		if strings.HasPrefix(line, "event: ") {
			evt := strings.TrimSpace(strings.TrimPrefix(line, "event: "))
			if evt != "ping" {
				return evt
			}
		}
	}
	return ""
}

func TestOpenAIMessagesStreamMissingCreatedStartsWithMessageStart(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.output_text.delta","output_index":0,"delta":"usable compact summary"}`,
		`{"type":"response.completed","response":{"id":"resp_skip_created","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":3,"total_tokens":13}}}`,
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	body := rec.Body.String()
	require.Equal(t, "message_start", firstAnthropicSSEEvent(body))
	require.Contains(t, body, "usable compact summary")
	require.Contains(t, body, "event: message_stop")
	requireClaudeCodeAnthropicSSEStateMachine(t, body)
}

func TestOpenAIMessagesStreamEmptyAfterPingWritesSSEError(t *testing.T) {
	svc, c, rec, resp, account := messagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_empty_ping","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
		`{"type":"response.completed","response":{"id":"resp_empty_ping","status":"completed","output":[{"type":"reasoning","summary":[]}],"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}}`,
	)
	MarkOpenAIAnthropicTransportStreamStarted(c)

	result, err := svc.handleAnthropicStreamingResponse(
		resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
	)

	failoverErr := requireMessagesFailoverError(t, err)
	require.NotNil(t, result)
	require.True(t, failoverErr.NoAccountFailover)
	require.Contains(t, rec.Body.String(), "event: error")
	require.Contains(t, rec.Body.String(), "without assistant content")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
	require.True(t, OpenAIAnthropicResponseTerminated(c))
}

func TestEmptyVisibleOutputError_WritesSSEAfterTransportStarted(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	_, c, rec, _, account := messagesTestStream(t)
	require.NoError(t, writeAnthropicCompactKeepalive(c))

	err := svc.newOpenAIEmptyVisibleOutputError(c, account, "rid",
		"Upstream messages stream completed without assistant content or tool output")
	require.NotNil(t, err)
	require.True(t, err.NoAccountFailover)
	require.Equal(t, "api_error", gjson.GetBytes(err.ResponseBody, "error.type").String())
	require.Contains(t, rec.Body.String(), "event: ping")
	require.Contains(t, rec.Body.String(), "event: error")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
	require.True(t, OpenAIAnthropicResponseTerminated(c))
}
