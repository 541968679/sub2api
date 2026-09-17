package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// issue #5601: strict Responses clients (Rust serde, Codex / Grok CLI) treat
// created_at as required. Synthesized Chat→Responses and Anthropic→Responses
// objects must emit it. Native Responses passthrough keeps upstream bytes.

func responseObjectOf(t *testing.T, evt ResponsesStreamEvent) map[string]any {
	t.Helper()
	m := marshalEvent(t, evt)
	resp, ok := m["response"].(map[string]any)
	require.True(t, ok, "event must carry a response object: %v", m)
	return resp
}

func requireCreatedAt(t *testing.T, resp map[string]any) int64 {
	t.Helper()
	raw, ok := resp["created_at"]
	require.True(t, ok, "response object must carry created_at")
	value, ok := raw.(float64)
	require.True(t, ok, "created_at must be a number, got %T", raw)
	require.Greater(t, int64(value), int64(0), "created_at must be a valid unix timestamp")
	return int64(value)
}

func TestWire_CreatedAtPresentEvenAtZero(t *testing.T) {
	resp := responseObjectOf(t, ResponsesStreamEvent{
		Type:     "response.created",
		Response: &ResponsesResponse{ID: "resp_1", Object: "response", Status: "in_progress"},
	})
	require.Contains(t, resp, "created_at", "created_at must not use omitempty")
	require.EqualValues(t, 0, resp["created_at"])
}

func TestChatCompletionsResponseToResponses_CarriesCreatedAt(t *testing.T) {
	t.Run("uses_upstream_created_when_present", func(t *testing.T) {
		out := ChatCompletionsResponseToResponses(&ChatCompletionsResponse{
			ID:      "chatcmpl_1",
			Created: 1700000000,
			Model:   "deepseek-v4-flash",
			Choices: []ChatChoice{{Message: ChatMessage{Role: "assistant", Content: json.RawMessage(`"hi"`)}}},
		}, "deepseek-v4-flash", nil, false, nil)
		require.EqualValues(t, 1700000000, out.CreatedAt, "reuse upstream created instead of stamping a new time")
	})

	t.Run("stamps_now_when_upstream_omits_created", func(t *testing.T) {
		out := ChatCompletionsResponseToResponses(&ChatCompletionsResponse{
			ID:      "chatcmpl_2",
			Model:   "deepseek-v4-flash",
			Choices: []ChatChoice{{Message: ChatMessage{Role: "assistant", Content: json.RawMessage(`"hi"`)}}},
		}, "deepseek-v4-flash", nil, false, nil)
		require.Greater(t, out.CreatedAt, int64(0))
	})

	t.Run("nil_upstream_response_still_stamps", func(t *testing.T) {
		out := ChatCompletionsResponseToResponses(nil, "deepseek-v4-flash", nil, false, nil)
		require.Greater(t, out.CreatedAt, int64(0), "empty upstream still must produce a parseable object")
	})
}

func TestChatCompletionsToResponsesStream_CreatedAtStableAcrossEvents(t *testing.T) {
	state := NewChatCompletionsToResponsesStreamState("deepseek-v4-flash")
	require.Greater(t, state.Created, int64(0), "state must already capture a timestamp")

	var chunk ChatCompletionsChunk
	require.NoError(t, json.Unmarshal(
		[]byte(`{"choices":[{"index":0,"delta":{"content":"hi"}}]}`), &chunk))

	events := ChatCompletionsChunkToResponsesEvents(&chunk, state)
	events = append(events, FinalizeChatCompletionsResponsesStream(state)...)

	seen := map[string]int64{}
	for _, evt := range events {
		if evt.Response == nil {
			continue
		}
		seen[evt.Type] = requireCreatedAt(t, responseObjectOf(t, evt))
	}

	require.Contains(t, seen, "response.created")
	require.Contains(t, seen, "response.completed")
	require.Equal(t, state.Created, seen["response.created"])
	require.Equal(t, seen["response.created"], seen["response.completed"],
		"created_at must stay constant across the stream")
}

func TestAnthropicToResponsesResponse_StampsCreatedAt(t *testing.T) {
	out := AnthropicToResponsesResponse(&AnthropicResponse{
		ID:      "msg_1",
		Type:    "message",
		Role:    "assistant",
		Model:   "claude-sonnet-4-20250514",
		Content: []AnthropicContentBlock{{Type: "text", Text: "hi"}},
	})
	require.Greater(t, out.CreatedAt, int64(0),
		"Anthropic responses have no timestamp; the gateway must stamp one")
}

func TestAnthropicEventToResponsesStream_CreatedAtStableAcrossEvents(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-20250514"
	require.Greater(t, state.Created, int64(0), "state must already capture a timestamp")

	var events []ResponsesStreamEvent
	for _, raw := range []string{
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"usage":{"input_tokens":3,"output_tokens":0}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`,
		`{"type":"message_stop"}`,
	} {
		var evt AnthropicStreamEvent
		require.NoError(t, json.Unmarshal([]byte(raw), &evt))
		events = append(events, AnthropicEventToResponsesEvents(&evt, state)...)
	}
	events = append(events, FinalizeAnthropicResponsesStream(state)...)

	seen := map[string]int64{}
	for _, evt := range events {
		if evt.Response == nil {
			continue
		}
		seen[evt.Type] = requireCreatedAt(t, responseObjectOf(t, evt))
	}

	require.Contains(t, seen, "response.created")
	require.Contains(t, seen, "response.completed")
	require.Equal(t, state.Created, seen["response.created"])
	require.Equal(t, seen["response.created"], seen["response.completed"],
		"created_at must stay constant across the stream")
}

func TestResponsesStreamEvent_CreatedAtSurvivesUnmarshalRemarshal(t *testing.T) {
	upstream := []byte(`{"type":"response.completed","response":{"id":"resp_9","object":"response",` +
		`"created_at":1700000123,"model":"gpt-5.5","status":"completed","output":[]}}`)

	var evt ResponsesStreamEvent
	require.NoError(t, json.Unmarshal(upstream, &evt))
	require.EqualValues(t, 1700000123, evt.Response.CreatedAt)

	require.EqualValues(t, 1700000123, requireCreatedAt(t, responseObjectOf(t, evt)))
}
