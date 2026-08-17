//go:build unit

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestOpenAIMessagesStreamContentPart_DoesNotDoubleEmitOrMarkThinkingVisible(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_terra","model":"gpt-5.6-terra","status":"in_progress"}}`,
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
	body := rec.Body.String()
	require.Equal(t, 1, strings.Count(body, "usable terra reply"))
	require.NotContains(t, body, "gpt-5.6-terra")
	require.NotContains(t, body, "OpenAI")
	// Empty reasoning item still becomes an empty thinking block once text is flushed.
	require.Contains(t, body, `"type":"thinking"`)
}

func TestOpenAIMessagesStreamEmptyContentPartDoesNotCommitFailover(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_empty_part","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`,
		`{"type":"response.failed","response":{"id":"resp_empty_part","status":"failed","error":{"code":"rate_limit_error","message":"temporary limit"},"output":[]}}`,
	)
	require.NotNil(t, result)
	failoverErr := requireMessagesFailoverError(t, err)
	require.Equal(t, 502, failoverErr.StatusCode)
	require.False(t, result.ClientOutputStarted)
	require.NotContains(t, rec.Body.String(), "message_stop")
}

func TestOpenAIMessagesStreamRefusalPartIsNotVisibleSuccess(t *testing.T) {
	result, err, rec := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_refusal","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"refusal","text":"I cannot help with that"}}`,
		`{"type":"response.content_part.done","output_index":0,"content_index":0,"part":{"type":"refusal","text":"I cannot help with that"}}`,
		`{"type":"response.completed","response":{"id":"resp_refusal","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"refusal","text":"I cannot help with that"}]}],"usage":{"input_tokens":10,"output_tokens":4,"total_tokens":14}}}`,
	)
	failoverErr := requireMessagesFailoverError(t, err)
	require.NotNil(t, result)
	require.True(t, failoverErr.NoAccountFailover)
	require.Equal(t, 502, failoverErr.StatusCode)
	require.NotContains(t, rec.Body.String(), "I cannot help with that")
}

func TestCompactNeedsRecovery_FalseAfterContentPartSupplement(t *testing.T) {
	acc := apicompat.NewBufferedResponseAccumulator()
	acc.ProcessEvent(&apicompat.ResponsesStreamEvent{
		Type: "response.content_part.added",
		Part: &apicompat.ResponsesContentPart{Type: "output_text", Text: "compact summary from part"},
	})
	resp := &apicompat.ResponsesResponse{
		Status: "completed",
		Output: []apicompat.ResponsesOutput{{
			Type:    "message",
			Role:    "assistant",
			Content: []apicompat.ResponsesContentPart{{Type: "output_text", Text: ""}},
		}},
	}
	acc.SupplementResponseOutput(resp)
	require.False(t, compactResponseNeedsRecovery(resp))
}

func TestOpenAIMessagesBufferedContentPartFillsEmptyTerminal(t *testing.T) {
	svc, c, rec, resp, account := messagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_buf","model":"gpt-5.6-terra","status":"in_progress"}}`,
		`{"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"output_text","text":"buffered terra reply"}}`,
		`{"type":"response.output_text.done","output_index":0,"content_index":0,"text":""}`,
		`{"type":"response.content_part.done","output_index":0,"content_index":0,"part":{"type":"output_text","text":"buffered terra reply"}}`,
		`{"type":"response.completed","response":{"id":"resp_buf","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":""}]}],"usage":{"input_tokens":100,"output_tokens":4,"total_tokens":104}}}`,
	)
	result, err := svc.handleAnthropicBufferedStreamingResponse(
		resp, c, account, true,
		"claude-sonnet-4-6", "gpt-5.6-terra", "gpt-5.6-terra", time.Now(),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, strings.Count(rec.Body.String(), "buffered terra reply"))
	require.NotContains(t, rec.Body.String(), `"type":"thinking"`)
}

func TestCompactNeedsRecovery_ReasoningOnlyStillTrue(t *testing.T) {
	resp := &apicompat.ResponsesResponse{
		Status: "completed",
		Output: []apicompat.ResponsesOutput{{
			Type:    "reasoning",
			Summary: []apicompat.ResponsesSummary{{Type: "summary_text", Text: "internal"}},
		}},
	}
	require.True(t, compactResponseNeedsRecovery(resp))
}
