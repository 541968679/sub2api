# OpenAI Claude-GPT Bridge For Antigravity Groups

Date: 2026-06-02

## Summary

This change lets an OpenAI account serve Claude Messages requests for existing
Antigravity groups without migrating users, subscriptions, API keys, or group
platforms. The OpenAI account explicitly opts into the bridge, binds the target
Antigravity group, and uses its existing `credentials.model_mapping` to map a
Claude request model such as `claude-opus-4-8` to a GPT upstream model such as
`gpt-5.5`.

The user-facing contract remains Antigravity/Claude:

- users continue to call `/v1/messages` or `/antigravity/v1/messages` with the
  same API key;
- billing and usage display use the original Claude request model;
- the GPT upstream model is stored in `usage_logs.upstream_model` for admin
  visibility;
- native Antigravity remains the fallback path before any OpenAI upstream call
  is made.

## Implementation Notes

Account configuration:

- Added `extra.openai_claude_gpt_bridge_enabled` on OpenAI accounts.
- OpenAI accounts may bind OpenAI groups by default.
- When the bridge switch is enabled, OpenAI accounts may also bind Antigravity
  groups.
- OpenAI accounts still cannot bind Anthropic or Gemini groups through this
  bridge.
- Claude-GPT mapping is account-global and remains in
  `credentials.model_mapping`; there is no group-level mapping.

Bridge eligibility:

- `account.platform == openai`;
- `extra.openai_claude_gpt_bridge_enabled == true`;
- the account is bound to the current Antigravity group;
- `credentials.model_mapping` explicitly matches the requested Claude model;
- the mapped upstream model is non-empty and different from the request model.

Gateway routing:

- OpenAI groups keep the existing `/v1/messages` OpenAI Messages bridge path.
- Antigravity `/v1/messages` now does an OpenAI bridge preflight before native
  Antigravity forwarding.
- The preflight reads and resets the request body so native Antigravity can
  still consume the original request if the bridge is not eligible.
- If a bridge account is selected, forwarding enters the existing OpenAI
  `ForwardAsAnthropic` conversion path.
- After the OpenAI upstream path is entered, the request no longer falls back to
  Antigravity; it uses existing OpenAI account failover behavior.

Scheduler behavior:

- Added a bridge-only OpenAI scheduler flag so normal OpenAI scheduling is not
  affected.
- Bridge scheduler candidates must pass the bridge flag and model-mapping hit.
- Slim scheduler metadata now preserves `credentials.model_mapping` and
  `extra.openai_claude_gpt_bridge_enabled`.
- A stale bridge candidate can refresh its full DB record before being rejected,
  avoiding false negatives after an admin enables the bridge.

Billing and usage:

- `usage_logs.model` and `usage_logs.requested_model` stay as the original
  Claude model.
- `usage_logs.upstream_model` stores the mapped GPT model when it differs.
- Billing uses the requested Claude model source.
- Token counts use the real upstream usage after the existing Anthropic response
  conversion path.
- Bridge forwarding preserves body-level `prompt_cache_key`, including the
  derived key used by the normal OpenAI Messages path. This keeps the bridge
  upstream body close to normal OpenAI traffic and allows upstream OpenAI cache
  behavior to work. Bridge mode still removes upstream `session_id` and
  `conversation_id` headers before the OpenAI request is sent.
- Without a cache display override, OpenAI
  `input_tokens_details.cached_tokens` is converted to Anthropic-style
  `cache_read_tokens`. Stored ordinary input tokens are
  `raw_input_tokens - cache_read_tokens`, while pricing still resolves against
  the requested Claude model.
- Admin setting `openai_claude_gpt_bridge_cache_display_settings` can enable a
  bridge-only display/billing cache override. It stores `enabled`,
  `min_percent`, and `max_percent`; the backend validates
  `0 <= min_percent <= max_percent <= 100`.
- When that override is enabled, each bridge response randomly chooses a
  percentage in the configured range and directly sets
  `cache_read_tokens = round(raw_input_tokens * percent / 100)`, clamped to the
  upstream input-token count. This value is locally generated from upstream
  `input_tokens`; it is not calculated from, added to, or scaled from upstream
  `cached_tokens`. The generated value replaces upstream `cached_tokens` for
  downstream Anthropic usage, usage-record display, and billing. The raw
  upstream `cached_tokens` value is still logged only as a diagnostic.
- OpenAI `/v1/messages` and Antigravity bridge `/v1/messages` now apply the
  same downstream display-token rewrite hook used by the ordinary gateway
  paths. When a user is configured for downstream display tokens, the HTTP/SSE
  response usage is rewritten with the same display pricing chain as usage-log
  display, while `OpenAIForwardResult.Usage` remains the bridge-generated base
  usage that is recorded and billed.
- Bridge diagnostics log token-only values for raw upstream usage, converted
  Anthropic usage, and final usage-log storage. These logs are used to verify
  whether repeated cache values such as `18.9k` originate upstream or in local
  conversion/storage.
- User usage APIs hide `upstream_model`; admin usage APIs expose it.
- Prompt-cache status dashboards use the request group platform, so bridge rows
  remain visible under Antigravity cache statistics.
- Account-cost statistics remain upstream-account oriented and use the GPT
  upstream model for OpenAI account cost visibility.

Frontend behavior:

- OpenAI account create/edit/bulk-edit forms expose the Claude-GPT bridge switch.
- When enabled, the group selector shows OpenAI and Antigravity groups.
- When disabled, Antigravity group selections are removed before submit.
- Existing OpenAI model mapping UI is reused for Claude-to-GPT mappings.

## Verification Performed

Local real request verification:

- Endpoint: `http://127.0.0.1:18081/antigravity/v1/messages`.
- Request model: `claude-opus-4-8`.
- Bridge account selected: OpenAI account `41`.
- Upstream model: `gpt-5.5`.
- Downstream response status: `200`.
- Downstream response model: `claude-opus-4-8`.
- Downstream usage: `input_tokens=23`, `output_tokens=19`.
- Usage row stored `model=claude-opus-4-8`,
  `requested_model=claude-opus-4-8`, and `upstream_model=gpt-5.5`.

Cache-read regression verification:

- Diagnosis found the fixed `18944` value in the raw OpenAI Responses SSE usage
  at `response.usage.input_tokens_details.cached_tokens`; local JSON parsing was
  not inventing that value.
- The same requests also logged stable upstream cache/session signals:
  `body_has_prompt_cache_key=true`, `header_has_session_id=true`, and
  `header_has_conversation_id=true`. These were derived from the Claude
  `metadata.user_id` path and forwarded into the OpenAI/Codex upstream request.
- A first mitigation removed those cache/session identifiers, which avoided the
  fixed cache display but also prevented normal upstream cache reuse.
- The current mitigation restores body-level `prompt_cache_key` forwarding while
  continuing to suppress `session_id` and `conversation_id` headers.
- Focused tests now assert bridge requests forward body `prompt_cache_key` but
  omit upstream session headers, while non-bridge OpenAI Messages behavior still
  forwards both prompt and session identity.
- Focused tests also assert the bridge cache display override ignores upstream
  `cached_tokens`, including a fixed upstream `18944`, and returns/writes the
  configured percentage-derived cache value instead.
- Focused tests cover both buffered JSON and streaming Anthropic SSE display
  usage rewrite, so downstream returned usage stays aligned with the configured
  display-token mode.
- Real local Claude Code verification on 2026-06-03 used
  `openai_claude_gpt_bridge_cache_display_settings={"enabled":true,"min_percent":60,"max_percent":70}`
  through Antigravity API key `5`. The upstream Responses terminal event
  reported `raw_input_tokens=22273`, `raw_cached_tokens=7680`, and
  `raw_output_tokens=94`; the bridge generated `display_cached_tokens=14946`
  with `chosen_percent=67.1041`. The stored usage row `15774` kept
  `model=requested_model=claude-opus-4-8`, `upstream_model=gpt-5.5`,
  `input_tokens=7327`, `cache_read_tokens=14946`, and `output_tokens=94`.
  Claude Code's downstream display-mode usage showed `input_tokens=16149`,
  `cache_read_input_tokens=14946`, and `output_tokens=188`, matching the same
  display-token transform used by user-facing usage-log DTOs.

Additional checks covered during implementation:

- bridge account eligibility for enabled/disabled flag, platform, group binding,
  and mapping hit/miss;
- account validation allowing Antigravity groups only when the OpenAI bridge is
  enabled;
- native Antigravity fallback before OpenAI upstream selection;
- user-facing model and billing model staying Claude;
- admin-only GPT upstream visibility;
- prompt-cache status filtering by request group platform.

## Operational Notes

How to configure a bridge account:

1. Edit an OpenAI account.
2. Enable Claude-GPT bridge.
3. Bind the target Antigravity group.
4. Add model mapping in the existing OpenAI model mapping editor, for example:

```json
{
  "claude-opus-4-8": "gpt-5.5"
}
```

Users do not need to regenerate API keys. Subscription users can keep using
their current Antigravity group because the bridge account is attached to that
group from the OpenAI account side.

## Known Residual Issues

The bridge does not currently change `/v1/models`. Clients may still discover
models from the native Antigravity path rather than the bridge mapping.

The bridge does not currently change `/antigravity/v1/messages/count_tokens`.
Claude Code token counting remains native Antigravity-side unless this route is
explicitly bridged later.

The downstream Anthropic response conversion still follows the generic
OpenAI-to-Anthropic compatibility path. If OpenAI upstream reports
`cached_tokens` and the bridge cache display override is disabled, the response
body includes converted Anthropic cache usage and the bridge usage record
preserves it. If the override is enabled, downstream response usage and usage
records use the generated percentage-derived cache value instead.

The bridge currently forwards body `prompt_cache_key` to preserve upstream
OpenAI cache behavior. Repeated raw upstream cache values such as `18.9k` must
still be debugged by checking upstream request diagnostics and raw upstream
usage. They should not leak to user-visible cache display when
`openai_claude_gpt_bridge_cache_display_settings.enabled` is true.

Context-window behavior is client-side plus upstream-side:

- Claude Code does not read local Codex `~/.codex/config.toml`.
- Codex `model_context_window` and `model_auto_compact_token_limit` only affect
  Codex when Codex itself is the client.
- Claude Code auto-compaction must be configured through Claude Code settings or
  environment variables, for example `CLAUDE_CODE_AUTO_COMPACT_WINDOW` and
  `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE`.
- If Claude Code believes a Claude model has a 1M context window, it may delay
  compaction according to that client-side window.
- The actual upstream accept/reject limit is still the mapped GPT model's real
  context window. For local Codex metadata, `gpt-5.5` reported
  `context_window=272000` and `max_context_window=272000`; overriding Codex
  config did not change that catalog value.
- The safe bridge compaction window should be treated as
  `min(client_compaction_window, upstream_gpt_context_window)` with extra margin
  for output and tool calls.

Recommended Claude Code bridge test configuration when mapping to a roughly
272k-context GPT model:

```powershell
$env:CLAUDE_CODE_AUTO_COMPACT_WINDOW = "240000"
$env:CLAUDE_AUTOCOMPACT_PCT_OVERRIDE = "85"
claude
```

Possible future improvements:

- Store optional account-level bridge metadata such as advertised context window
  and upstream context window.
- Bridge or override `/models` for Antigravity groups when a bridge account is
  intentionally preferred.
- Add a bridge-aware `/messages/count_tokens` path.
- Add an early request-size guard that rejects or warns before sending an
  obviously oversized request to the GPT upstream.
- Consider a future platform-native implementation where OpenAI credentials can
  be added directly as an Antigravity account subtype, while still reusing the
  same protocol conversion path.

## Related Files

- `backend/internal/server/routes/gateway.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/account.go`
- `backend/internal/service/admin_service.go`
- `backend/internal/repository/scheduler_cache.go`
- `backend/internal/repository/usage_log_repo.go`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/components/common/GroupSelector.vue`
- `docs/dev/codebase/account.md`
- `docs/dev/codebase/gateway.md`
- `docs/dev/codebase/model-mapping.md`
- `docs/dev/codebase/billing.md`
