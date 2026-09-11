package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func anthropicAssistantMsg(t *testing.T, blocks string) *AnthropicRequest {
	t.Helper()
	return &AnthropicRequest{
		Model:     "deepseek-v4-flash",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{Role: "user", Content: json.RawMessage(`"what's the weather?"`)},
			{Role: "assistant", Content: json.RawMessage(blocks)},
			{Role: "user", Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"toolu_1","content":"sunny"}]`)},
		},
	}
}

const anthropicThinkingToolTurn = `[
{"type":"thinking","thinking":"user wants weather, call the tool"},
{"type":"text","text":"checking"},
{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{"city":"SF"}}
]`

func TestAnthropicToChatCompletionsRequest_ThinkingBecomesReasoningContentOnToolTurn(t *testing.T) {
	out, err := AnthropicToChatCompletionsRequest(anthropicAssistantMsg(t, anthropicThinkingToolTurn))
	require.NoError(t, err)

	var assistant *ChatMessage
	for i := range out.Messages {
		if out.Messages[i].Role == "assistant" {
			assistant = &out.Messages[i]
			break
		}
	}
	require.NotNil(t, assistant, "assistant message must survive the bridge")
	require.Equal(t, "user wants weather, call the tool", assistant.ReasoningContent,
		"tool-turn thinking must be replayed as reasoning_content")
	require.Len(t, assistant.ToolCalls, 1)
	require.Equal(t, `"checking"`, string(assistant.Content), "text/tool_use handling must stay unchanged")
}

func TestAnthropicToChatCompletionsRequest_ReasoningContentSerializesOnWire(t *testing.T) {
	out, err := AnthropicToChatCompletionsRequest(anthropicAssistantMsg(t, anthropicThinkingToolTurn))
	require.NoError(t, err)

	payload, err := json.Marshal(out)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"reasoning_content":"user wants weather, call the tool"`)
}

func TestAnthropicToChatCompletionsRequest_ThinkingDroppedOnPlainTextTurn(t *testing.T) {
	out, err := AnthropicToChatCompletionsRequest(anthropicAssistantMsg(t, `[
{"type":"thinking","thinking":"just answering"},
{"type":"text","text":"hello"}
]`))
	require.NoError(t, err)

	var assistant *ChatMessage
	for i := range out.Messages {
		if out.Messages[i].Role == "assistant" && len(out.Messages[i].ToolCalls) == 0 {
			assistant = &out.Messages[i]
			break
		}
	}
	require.NotNil(t, assistant)
	require.Empty(t, assistant.ReasoningContent)
}
