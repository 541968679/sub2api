package apicompat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectAnthropicText(events []AnthropicStreamEvent) string {
	var b strings.Builder
	for _, evt := range events {
		if evt.Type == "content_block_delta" && evt.Delta != nil && evt.Delta.Type == "text_delta" {
			b.WriteString(evt.Delta.Text)
		}
	}
	return b.String()
}

func TestContentPartHarvest_NoDuplicateAcrossAddedDoneItemAndCompleted(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	var events []AnthropicStreamEvent
	for _, evt := range []*ResponsesStreamEvent{
		{Type: "response.created", Response: &ResponsesResponse{ID: "resp_dup", Model: "gpt-5.6-terra"}},
		{Type: "response.output_item.added", OutputIndex: 0, Item: &ResponsesOutput{Type: "message", Role: "assistant"}},
		{Type: "response.content_part.added", OutputIndex: 0, ContentIndex: 0, Part: &ResponsesContentPart{Type: "output_text", Text: "same reply"}},
		{Type: "response.output_text.done", OutputIndex: 0, ContentIndex: 0, Text: "same reply"},
		{Type: "response.content_part.done", OutputIndex: 0, ContentIndex: 0, Part: &ResponsesContentPart{Type: "output_text", Text: "same reply"}},
		{Type: "response.output_item.done", OutputIndex: 0, Item: &ResponsesOutput{
			Type: "message", Role: "assistant",
			Content: []ResponsesContentPart{{Type: "output_text", Text: "same reply"}},
		}},
		{Type: "response.completed", Response: &ResponsesResponse{
			ID: "resp_dup", Status: "completed",
			Output: []ResponsesOutput{{
				Type: "message", Role: "assistant",
				Content: []ResponsesContentPart{{Type: "output_text", Text: "same reply"}},
			}},
		}},
	} {
		events = append(events, ResponsesEventToAnthropicEvents(evt, state)...)
	}
	assert.Equal(t, "same reply", collectAnthropicText(events))
	assert.Equal(t, 1, strings.Count(collectAnthropicText(events), "same reply"))
}

func TestContentPartHarvest_IgnoresRefusalAndImageParts(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.created", Response: &ResponsesResponse{ID: "resp_nt"},
	}, state)

	refusal := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.content_part.added", OutputIndex: 0, ContentIndex: 0,
		Part: &ResponsesContentPart{Type: "refusal", Text: "I cannot help with that"},
	}, state)
	image := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.content_part.done", OutputIndex: 0, ContentIndex: 1,
		Part: &ResponsesContentPart{Type: "output_image", Text: "data:image/png;base64,xxxx"},
	}, state)
	assert.Empty(t, refusal)
	assert.Empty(t, image)
	assert.Equal(t, "", ResponsesVisibleTextFromPart(&ResponsesContentPart{Type: "refusal", Text: "nope"}))
	assert.Equal(t, "", ResponsesVisibleTextFromPart(&ResponsesContentPart{Type: "output_image", Text: "img"}))
}

func TestContentPartHarvest_NilPartAndItemDoNotPanic(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	assert.NotPanics(t, func() {
		_ = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{Type: "response.content_part.added"}, state)
		_ = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{Type: "response.content_part.done"}, state)
		_ = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{Type: "response.output_item.done"}, state)
		_ = ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{Type: "response.output_text.done"}, state)
		_ = responsesVisibleTextFromStreamEvent(nil)
		_ = ResponsesVisibleTextFromPart(nil)
		acc := NewBufferedResponseAccumulator()
		acc.ProcessEvent(nil)
		acc.SupplementResponseOutput(nil)
	})
}

func TestContentPartHarvest_ClosesOpenReasoningBeforeText(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.created", Response: &ResponsesResponse{ID: "resp_rs"},
	}, state)
	ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.output_item.added", OutputIndex: 0,
		Item: &ResponsesOutput{Type: "reasoning"},
	}, state)
	require.True(t, state.ContentBlockOpen)
	require.Equal(t, "thinking", state.CurrentBlockType)

	events := ResponsesEventToAnthropicEvents(&ResponsesStreamEvent{
		Type: "response.content_part.added", OutputIndex: 1, ContentIndex: 0,
		Part: &ResponsesContentPart{Type: "output_text", Text: "after thinking"},
	}, state)
	require.NotEmpty(t, events)
	assert.Equal(t, "content_block_stop", events[0].Type)
	assert.Equal(t, "content_block_start", events[1].Type)
	require.NotNil(t, events[1].ContentBlock)
	assert.Equal(t, "text", events[1].ContentBlock.Type)
	assert.Equal(t, "after thinking", collectAnthropicText(events))
	assert.False(t, state.ContentBlockOpen, "text snapshot should close the text block")
}

func TestContentPartHarvest_AddedPlusIdenticalDeltaDuplicatesAcrossBlocks(t *testing.T) {
	state := NewResponsesEventToAnthropicState()
	var events []AnthropicStreamEvent
	for _, evt := range []*ResponsesStreamEvent{
		{Type: "response.created", Response: &ResponsesResponse{ID: "resp_mix"}},
		{Type: "response.content_part.added", OutputIndex: 0, ContentIndex: 0, Part: &ResponsesContentPart{Type: "output_text", Text: "Hello"}},
		{Type: "response.output_text.delta", OutputIndex: 0, ContentIndex: 0, Delta: "Hello"},
	} {
		events = append(events, ResponsesEventToAnthropicEvents(evt, state)...)
	}
	// Residual mixed-shape risk: snapshot close + later identical delta.
	// Production terra/haiku 72 had no output_text.delta.
	assert.Equal(t, "HelloHello", collectAnthropicText(events))
}

func TestBufferedAccumulator_AddedPlusIdenticalDeltasConcatenates(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type: "response.content_part.added",
		Part: &ResponsesContentPart{Type: "output_text", Text: "Hello"},
	})
	acc.ProcessEvent(&ResponsesStreamEvent{Type: "response.output_text.delta", Delta: "Hello"})
	resp := &ResponsesResponse{Status: "completed"}
	acc.SupplementResponseOutput(resp)
	require.Len(t, resp.Output, 1)
	require.Len(t, resp.Output[0].Content, 1)
	assert.Equal(t, "HelloHello", resp.Output[0].Content[0].Text)
}

func TestBufferedAccumulator_DoesNotOverwriteNonEmptyTerminal(t *testing.T) {
	acc := NewBufferedResponseAccumulator()
	acc.ProcessEvent(&ResponsesStreamEvent{
		Type: "response.content_part.added",
		Part: &ResponsesContentPart{Type: "output_text", Text: "from part"},
	})
	resp := &ResponsesResponse{
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:    "message",
			Content: []ResponsesContentPart{{Type: "output_text", Text: "from terminal"}},
		}},
	}
	acc.SupplementResponseOutput(resp)
	assert.Equal(t, "from terminal", resp.Output[0].Content[0].Text)
}

func TestResponsesToAnthropic_ReasoningOnlyHasNoVisibleText(t *testing.T) {
	anth := ResponsesToAnthropic(&ResponsesResponse{
		Status: "completed",
		Output: []ResponsesOutput{{
			Type:    "reasoning",
			Summary: []ResponsesSummary{{Type: "summary_text", Text: "internal only"}},
		}},
	}, "claude-sonnet-4-6")
	require.NotNil(t, anth)
	visible := false
	for _, b := range anth.Content {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			visible = true
		}
		if b.Type == "tool_use" || b.Type == "server_tool_use" {
			visible = true
		}
	}
	assert.False(t, visible)
}
