package apicompat

import "testing"

func TestAnthropicEventToResponses_ThinkingAfterTextKeepsMessageOutput(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	i0, i1 := 0, 1
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &i0, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &i0, Delta: &AnthropicDelta{Type: "text_delta", Text: "answer"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &i0})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &i1, ContentBlock: &AnthropicContentBlock{Type: "thinking"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &i1, Delta: &AnthropicDelta{Type: "thinking_delta", Thinking: "hmm"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &i1})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	var completed *ResponsesStreamEvent
	for i := range events {
		if events[i].Type == "response.completed" {
			completed = &events[i]
		}
	}
	if completed == nil || completed.Response == nil {
		t.Fatalf("response.completed was not emitted")
	}
	outputs := completed.Response.Output
	if len(outputs) != 2 {
		t.Fatalf("response.completed carries %d output items, want 2 (message + reasoning): %+v", len(outputs), outputs)
	}
	if outputs[0].Type != "message" {
		t.Fatalf("output[0].type = %q, want message", outputs[0].Type)
	}
	if len(outputs[0].Content) != 1 || outputs[0].Content[0].Text != "answer" {
		t.Errorf("assistant text lost: output[0].content = %+v", outputs[0].Content)
	}
	if outputs[1].Type != "reasoning" {
		t.Errorf("output[1].type = %q, want reasoning", outputs[1].Type)
	}

	var addedIndexes []int
	for _, evt := range events {
		if evt.Type == "response.output_item.added" {
			addedIndexes = append(addedIndexes, evt.OutputIndex)
		}
	}
	if len(addedIndexes) != 2 || addedIndexes[0] == addedIndexes[1] {
		t.Errorf("output_item.added indexes = %v, want two distinct values", addedIndexes)
	}
}

func TestAnthropicEventToResponses_MultipleTextBlocksAdvanceContentIndex(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	state.Model = "claude-sonnet-4-5"

	var events []ResponsesStreamEvent
	feed := func(evt *AnthropicStreamEvent) {
		events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
	}

	i0, i1 := 0, 1
	feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &i0, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &i0, Delta: &AnthropicDelta{Type: "text_delta", Text: "first"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &i0})
	feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &i1, ContentBlock: &AnthropicContentBlock{Type: "text"}})
	feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &i1, Delta: &AnthropicDelta{Type: "text_delta", Text: "second"}})
	feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &i1})
	feed(&AnthropicStreamEvent{Type: "message_stop"})

	var partAdded []int
	for _, evt := range events {
		if evt.Type == "response.content_part.added" {
			partAdded = append(partAdded, evt.ContentIndex)
		}
	}
	if len(partAdded) != 2 {
		t.Fatalf("content_part.added emitted %d times, want 2", len(partAdded))
	}
	if partAdded[0] != 0 || partAdded[1] != 1 {
		t.Errorf("content_part.added indexes = %v, want [0 1]", partAdded)
	}

	byIndex := map[int]string{}
	for _, evt := range events {
		if evt.Type == "response.output_text.delta" {
			byIndex[evt.ContentIndex] += evt.Delta
		}
	}
	if byIndex[0] != "first" || byIndex[1] != "second" {
		t.Errorf("output_text.delta grouped by content_index = %v, want {0:first 1:second}", byIndex)
	}
}

func TestAnthropicEventToResponses_ItemLifecycleIsBalanced(t *testing.T) {
	for _, tc := range []struct {
		name   string
		blocks []*AnthropicContentBlock
	}{
		{"text then thinking", []*AnthropicContentBlock{{Type: "text"}, {Type: "thinking"}}},
		{"thinking then text", []*AnthropicContentBlock{{Type: "thinking"}, {Type: "text"}}},
		{"text then tool_use", []*AnthropicContentBlock{{Type: "text"}, {Type: "tool_use", ID: "toolu_1", Name: "t"}}},
		{"text thinking text", []*AnthropicContentBlock{{Type: "text"}, {Type: "thinking"}, {Type: "text"}}},
		{"thinking text tool_use", []*AnthropicContentBlock{{Type: "thinking"}, {Type: "text"}, {Type: "tool_use", ID: "toolu_2", Name: "t"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := NewAnthropicEventToResponsesState()
			state.Model = "claude-sonnet-4-5"

			var events []ResponsesStreamEvent
			feed := func(evt *AnthropicStreamEvent) {
				events = append(events, AnthropicEventToResponsesEvents(evt, state)...)
			}

			feed(&AnthropicStreamEvent{Type: "message_start", Message: &AnthropicResponse{ID: "msg_1"}})
			for i, block := range tc.blocks {
				idx := i
				feed(&AnthropicStreamEvent{Type: "content_block_start", Index: &idx, ContentBlock: block})
				switch block.Type {
				case "text":
					feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "text_delta", Text: "t"}})
				case "thinking":
					feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "thinking_delta", Thinking: "r"}})
				case "tool_use":
					feed(&AnthropicStreamEvent{Type: "content_block_delta", Index: &idx, Delta: &AnthropicDelta{Type: "input_json_delta", PartialJSON: "{}"}})
				}
				feed(&AnthropicStreamEvent{Type: "content_block_stop", Index: &idx})
			}
			feed(&AnthropicStreamEvent{Type: "message_stop"})

			openIDs := map[string]int{}
			var order []string
			for _, evt := range events {
				switch evt.Type {
				case "response.output_item.added":
					if evt.Item == nil {
						t.Fatalf("output_item.added without item")
					}
					openIDs[evt.Item.ID]++
					order = append(order, evt.Item.ID)
				case "response.output_item.done":
					if evt.Item == nil {
						t.Fatalf("output_item.done without item")
					}
					openIDs[evt.Item.ID]--
				}
			}
			for id, n := range openIDs {
				if n != 0 {
					t.Errorf("item %s: output_item.added/done imbalance %+d", id, n)
				}
			}

			var completed *ResponsesStreamEvent
			for i := range events {
				if events[i].Type == "response.completed" {
					completed = &events[i]
				}
			}
			if completed == nil || completed.Response == nil {
				t.Fatalf("response.completed was not emitted")
			}
			if len(completed.Response.Output) != len(order) {
				t.Fatalf("response.completed carries %d outputs, want %d (one per announced item)",
					len(completed.Response.Output), len(order))
			}
			for i, id := range order {
				if completed.Response.Output[i].ID != id {
					t.Errorf("output[%d].id = %q, want %q (announcement order must be preserved)",
						i, completed.Response.Output[i].ID, id)
				}
			}
		})
	}
}
