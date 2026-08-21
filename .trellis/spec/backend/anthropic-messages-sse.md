# Anthropic Messages SSE (Responses bridge)

## Scenario: Claude Code consumes OpenAI Responses via Anthropic SSE

### 1. Scope / Trigger
- Trigger: converting OpenAI Responses SSE into Anthropic Messages SSE (`ResponsesEventToAnthropicEvents`), including Claude-GPT bridge compact and ordinary `/v1/messages`.
- Native Anthropic / Antigravity passthrough is out of this spec.
- Codex remote compaction V2 (`/v1/responses`) is out of this spec.

### 2. Signatures
- `ensureResponsesAnthropicMessageStart(state) []AnthropicStreamEvent`
- `prependResponsesAnthropicContentEvents(state, events) []AnthropicStreamEvent`
- `OpenAIGatewayService.newOpenAIEmptyVisibleOutputError(c, account, requestID, message) *UpstreamFailoverError`
- `buildAnthropicStreamErrorSSE(errType, message) string`
- Context flags: `OpenAIAnthropicTransportStreamStarted`, `OpenAIAnthropicSemanticOutputStarted`, `OpenAIAnthropicResponseTerminated`

### 3. Contracts
- Legal Claude Code order: `message_start` → `content_block_start` → `content_block_delta`* → `content_block_stop` → `message_delta` → `message_stop`.
- `event: ping` may appear anytime as transport keepalive. Ping is not semantic output and must not block failover.
- Any `content_block_start` / `content_block_delta` must be preceded by `message_start`, even if upstream never sent `response.created`.
- Repeat `ensureResponsesAnthropicMessageStart` is a no-op once `MessageStartSent`.
- Do not synthesize a preamble content block or empty `text_delta` only to reset a client watchdog.

### 4. Validation & Error Matrix
| Compact / empty-visible outcome | Downstream | Failover |
| --- | --- | --- |
| No bytes written yet (including no ping) | HTTP 502 JSON, `api_error`, empty-visible message | `NoAccountFailover` |
| Transport started (ping or flushed SSE), no visible text/tool | `event: error` + `MarkOpenAIAnthropicResponseTerminated` | `NoAccountFailover` |
| Semantic text/tool already started | Do not rewrite as empty compact | No compact account replay |
| Recovery produced visible summary | Full legal SSE via `writeAnthropicResponseAsSSE` | n/a |

Never HTTP 200 + empty `message_stop` / `end_turn` for a failed compact.

### 5. Good/Base/Bad Cases
- Good: `response.created` then text deltas — first event remains `message_start` from created; helper no-ops later.
- Base: first upstream event is `output_text.delta` or `output_item.added` — converter prepends `message_start`.
- Bad: emitting `content_block_delta` with no current message → Claude Code `Received content_block_delta without a current message`, empty compact `<summary>`, amnesia.

### 6. Tests Required
- Missing `response.created` for text, function_call, thinking, content_part.
- Claude Code state-machine fixture: `content_block_delta` without a current message fails.
- Empty compact: unstarted → 502 JSON; after ping → `event: error`; no successful empty `message_stop`.
- Existing empty-output / compact / prompt-too-long suites must still pass.

### 7. Wrong vs Correct
#### Wrong
Send `content_block_delta` as soon as Responses `output_text.delta` arrives, assuming `response.created` already happened. Treat a completed empty compact as HTTP 200 `end_turn`.
#### Correct
`prependResponsesAnthropicContentEvents` before every `content_block_*` batch. Empty compact uses the 502 / `event: error` matrix above.
