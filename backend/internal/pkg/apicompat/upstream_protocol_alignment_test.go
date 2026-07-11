package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesToAnthropicRequest_MergesInstructionsAndDeveloperRole(t *testing.T) {
	req := &ResponsesRequest{
		Model:        "claude-sonnet-4-20250514",
		Instructions: "Top-level instruction.",
		Input: json.RawMessage(`[
			{"role":"developer","content":[{"type":"input_text","text":"Developer instruction."}]},
			{"role":"user","content":"hello"}
		]`),
	}

	got, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)

	var system string
	require.NoError(t, json.Unmarshal(got.System, &system))
	require.Equal(t, "Top-level instruction.\n\nDeveloper instruction.", system)
	require.Len(t, got.Messages, 1)
	require.Equal(t, "user", got.Messages[0].Role)
}

func TestParallelToolCallsRoundTripsBetweenChatAndResponses(t *testing.T) {
	for _, value := range []bool{false, true} {
		value := value
		responses, err := ChatCompletionsToResponses(&ChatCompletionsRequest{
			Model:             "gpt-4o",
			Messages:          []ChatMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}},
			ParallelToolCalls: &value,
		})
		require.NoError(t, err)
		require.NotNil(t, responses.ParallelToolCalls)
		require.Equal(t, value, *responses.ParallelToolCalls)

		chat, err := ResponsesToChatCompletionsRequest(responses)
		require.NoError(t, err)
		require.NotNil(t, chat.ParallelToolCalls)
		require.Equal(t, value, *chat.ParallelToolCalls)
	}
}
