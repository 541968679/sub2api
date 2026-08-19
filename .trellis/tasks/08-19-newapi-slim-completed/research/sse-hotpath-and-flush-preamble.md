# Research — NewAPI slim completed 锚点

Date: 2026-08-19  
Scope: Sub2API OpenAI Responses HTTP SSE only. No product code in this file.

## Problem (locked)

Customer NewAPI will not be patched. Fat or empty-`output[]` `response.completed` events confuse NewAPI. Sub2API must optionally slim that event. Do not disable/derank account 1685. Does not fix Codex disconnect after NewAPI already stopped reading, or upstream `response.failed` / 1685 connection failed.

## Settings pattern to copy

`openai_responses_flush_preamble` + `openai_responses_flush_preamble_user_ids`:

| Layer | Location |
|-------|----------|
| Keys | `backend/internal/service/domain_constants.go` (`SettingKeyOpenAIResponsesFlushPreamble*`) |
| Defaults | `setting_service.go` GetAllSettings defaults: `"false"` / `"[]"` |
| Cache | `cachedGatewayForwardingSettings` + `getGatewayForwardingSettingsCached` + `GetMultiple` key list |
| Gate | `IsOpenAIResponsesFlushPreambleEnabled(ctx)`: global true → all; else whitelist via `ctxkey.UserID` |
| Normalize | drop `<=0`, dedupe; invalid JSON → empty |
| DTO / admin | `handler/dto/settings.go`, `handler/admin/setting_handler.go` (get/update/changed) |
| View | `settings_view.go` |
| Admin UI | `SettingsView.vue` + `OpenAIFastPolicyUserSelector`; form default / normalize / save |
| API types | `frontend/src/api/admin/settings.ts` |
| i18n | `frontend/src/i18n/locales/{zh,en}.ts` `admin.settings.gatewayForwarding.flushPreamble*` |
| Tests | `gateway_dateline_normalization_test.go` `TestOpenAIResponsesFlushPreambleSettingDefaultOff`; `SettingsView.spec.ts` stubs |

Not a public setting. No `api_keys` column. Not in `PublicSettings` / injection schema.

## SSE hot paths that must both apply

1. **Passthrough** — `handleStreamingResponsePassthrough` (`openai_gateway_service.go` ~4281)
   - Order today: sanitize / model replace → `parseSSEUsageBytes(dataBytes, usage)` → `rewriteOpenAIResponsesSSEUsageTokens(line, mult)` → write `line`.
   - `[DONE]` sets `sawDone` and is written as a data line.
   - `clientDisconnected`: keep draining upstream; do not write more to client.

2. **Non-passthrough** — inner `processSSELine` in `handleStreamingResponse` (~5360)
   - Extra work first: tool corrector, image normalize, `normalizeResponsesStreamingTerminalOutput`, Codex image inject.
   - Then display rewrite into `lineForDownstream`, **write**, then `parseSSEUsageBytes(dataBytes, usage)` on **pre-rewrite** bytes.
   - Billing-real is always `dataBytes`, never the rewritten/slim line.

`writeMarkedCodexCompactV2Stream` early-returns before either loop. Out of scope; do not slim that path.

## Usage parse / display rewrite

`parseSSEUsageBytes` (~5870):

- Skips `[DONE]`.
- **Skips `len(data) < 72`** — never slim *before* parse, or a short slim payload can drop billing usage.
- Types: `response.completed`, `response.done`, `response.incomplete`, `response.cancelled`, `response.canceled`.
- **Not** `response.failed` for this helper (failed is parsed separately in the failed branch).
- Fields: `input_tokens`, `output_tokens`, `cached_tokens`, cache-write variants, `image_tokens`. No `total_tokens` stored on `OpenAIUsage`.

`rewriteOpenAIResponsesSSEUsageTokens` (`display_token_rewrite.go` ~793): same terminal types; rewrites `response.usage` in place via `sjson`. Slim must run **after** this so client integers match display-mode SSE.

`openAIStreamEventIsTerminal` also treats `[DONE]` and `response.failed` as terminal. Synthesis trigger is **not** “any terminal”: only done / incomplete / cancelled / canceled, and only if no `response.completed` yet.

## Slim contract (product)

```json
{"type":"response.completed","response":{"id":"<id>","usage":{"input_tokens":N,"output_tokens":M,"total_tokens":N+M}}}
```

Optional only: `response.usage.input_tokens_details.cached_tokens` when the display-rewritten source has a **number** (not null / missing). Build usage fresh — do not copy the upstream usage object. Do not add `completion_tokens`. Strip `output`, `reasoning`, `temperature`, `max_output_tokens`, `store`, `status`, etc.

- `output_tokens == 0` (billing-real parsed): do not slim, do not synthesize.
- Already have completed: slim that one in place; never emit a second.
- Soft terminal + no completed + billing `output_tokens != 0`: emit one slim completed **before** `[DONE]` (or at stream end if no `[DONE]`).
- `clientDisconnected`: no extra writes.
- Default global gate **false**. Canary user **220** via whitelist after deploy; do not compile `[220]` as the KV default (default stays `[]`, same as flush-preamble).

## Out of path

- WS (`openai_ws_forwarder.go`, `openai_ws_v2`)
- Anthropic `/v1/messages` empty-output tests (`openai_gateway_messages_empty_output_test.go`) — different protocol
- Non-stream JSON / SSE-to-JSON
- Account scheduler / 1685
- NewAPI itself

## Recommended helper file

`backend/internal/service/openai_newapi_slim_completed.go` + `*_test.go` so `openai_gateway_service.go` only calls a once-per-stream gate + per-line / end-of-stream hooks.

## Existing stream tests to extend (do not regress)

- `openai_gateway_service_test.go` passthrough cases
- `openai_gateway_passthrough_function_args_test.go`
- `openai_gateway_service_hotpath_test.go` (already has `output:[]` completed)
- `openai_stream_stage_timing_test.go` flush-preamble cache helper `storeGatewayFlushPreambleCache`
- `frontend/src/views/admin/__tests__/SettingsView.spec.ts` form defaults + save payload
