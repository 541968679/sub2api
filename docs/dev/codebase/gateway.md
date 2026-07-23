# API Gateway

> Unified API entry points, account scheduling, upstream protocol conversion,
> failover, and usage recording.

## Spark Shadow Routing

An OpenAI Spark shadow is selected and accounted as its own scheduling record,
but `resolveCredentialAccount` dereferences `parent_account_id` before reading
tokens or building ChatGPT/FedRAMP headers. HTTP and WebSocket paths keep the
shadow ID for connection ownership, usage snapshots, and Spark rate limits;
the parent supplies authentication identity and the inherited proxy.

Spark eligibility requires an explicit Spark mapping and rejects default-model
fallback. Global OAuth quota/429 headers do not consume the Spark lane, while
Spark 429 state is not written to the parent. Parent invalidity, expiry, and
auth/transport cooldowns still make the shadow unavailable. This separation
must not feed into ordinary billing, display/cache-read transforms,
Claude-GPT bridge routing, native Images, or Ops/public settings.

## Data Model

| Entity/field | Location | Notes |
|--------------|----------|-------|
| Group.platform | `backend/internal/service/group.go` | Native platform for the group and default scheduling scope. |
| Group.blocked_models / allowed_models | `backend/internal/service/group.go` | Group-level model access control evaluated before account scheduling. |
| Account.platform/type/status | `backend/internal/service/account.go` | Core inputs for scheduling and upstream token lookup. |
| Account.extra.mixed_scheduling | `backend/internal/service/account.go` | Whether an Antigravity account may join Anthropic/Gemini mixed scheduling. |
| Account.extra.openai_claude_gpt_bridge_enabled | `backend/internal/service/account.go` | Whether an OpenAI account may serve Claude-GPT bridge requests for bound Antigravity groups. |
| Account.credentials.openai_capabilities | `backend/internal/service/account.go` | Optional OpenAI API-key endpoint capability list, currently used by chat completions and embeddings scheduling. |
| Account.extra.openai_images_endpoint_enabled | `backend/internal/service/account.go` | Local account-level opt-out for independent `/v1/images/*` scheduling only. |
| APIKey.group_id | `backend/internal/service/api_key.go` | Scheduling group bound to the user request. |

## Key Files

| Layer | File | Responsibility |
|-------|------|----------------|
| Handler | `backend/internal/handler/gateway_handler.go` | Anthropic Messages, Gemini compatibility, and Antigravity native entry points. |
| Handler | `backend/internal/handler/gateway_handler_chat_completions.go` | `/v1/chat/completions` compatibility entry for Anthropic groups. |
| Handler | `backend/internal/handler/group_model_access.go` | Shared group model access checks, including Responses image tool validation. |
| Handler | `backend/internal/handler/openai_gateway_handler.go` | OpenAI-compatible gateway plus Anthropic Messages bridge for OpenAI and Antigravity bridge preflight. |
| Handler | `backend/internal/handler/openai_embeddings.go` | OpenAI-compatible `/v1/embeddings` entry for API-key OpenAI upstream accounts. |
| Handler | `backend/internal/handler/openai_images.go` | OpenAI-compatible `/v1/images/*` entry, image request parsing orchestration, scheduling, and usage submission. |
| Service | `backend/internal/service/gateway_service.go` | Account selection, mixed scheduling, sticky sessions, and Anthropic upstream request building. |
| Service | `backend/internal/service/gateway_forward_as_chat_completions.go` | Chat Completions -> Responses -> Anthropic Messages conversion and forwarding. |
| Service | `backend/internal/service/openai_account_scheduler.go` | OpenAI account scheduler, endpoint capability checks, image capability checks, bridge eligibility, and WS failover selection. |
| Service | `backend/internal/service/openai_embeddings.go` | OpenAI API-key embeddings forwarding, model mapping, upstream response passthrough, and embeddings usage extraction. |
| Service | `backend/internal/service/openai_images.go` | OpenAI Images API key forwarding, request normalization, and direct image response handling. |
| Service | `backend/internal/service/openai_images_responses.go` | OpenAI OAuth image forwarding through Codex `/responses` and stream/non-stream transformation. |
| Service | `backend/internal/service/openai_gateway_grok.go` | Grok Responses and Anthropic Messages compatibility, xAI request/response conversion, failover signals, and quota snapshots. |
| Handler | `backend/internal/handler/grok_media.go` | Grok image/video request validation, moderation, concurrency, Grok-only scheduling, failover, and usage submission. |
| Service | `backend/internal/service/grok_media.go` | Grok image/video request normalization, xAI forwarding, response metadata, video status binding, and failover signals. |
| Service | `backend/internal/service/openai_image_trace.go` | Temporary `OPENAI_IMAGE_TRACE_LOG` diagnostics for `gpt-image-2` generations. |
| Service | `backend/internal/service/antigravity_gateway_service.go` | Antigravity native request/response conversion and forwarding. |
| Service | `backend/internal/service/ratelimit_service.go` | Maps upstream errors to account state, temporary unschedulable windows, and rate limits. |

## Core Flow

### Grok Image And Video Media

```
/v1/images/* or /v1/videos with a Grok group
  -> parse model/media metadata
  -> group media permission and content moderation
  -> user concurrency and platform billing eligibility
  -> SelectAccountWithSchedulerForCapability(... platform=grok ...)
  -> account concurrency and ForwardGrokMedia()
  -> bounded same-account retry / account failover
  -> bind video request ID for status polling
  -> RecordUsage()
     -> image per-request or video per-second pricing
     -> balance/subscription and user-platform quota accounting
     -> usage_logs video_count/resolution/duration persistence
```

OpenAI groups keep using `openai_images.go`; Grok media routing does not bypass
the local OpenAI Images account capability switch or alter Claude-GPT bridge
eligibility. Moderation runs before billing and scheduling, so a local block has
no quota or usage side effect.

### Grok + Codex multi-turn and OpenAI-key cross-platform Grok

Research (not yet implemented): [../GROK_CODEX_AND_CROSS_PLATFORM_RESEARCH_2026-07-15.md](../GROK_CODEX_AND_CROSS_PLATFORM_RESEARCH_2026-07-15.md).

- **Requirement A (implemented 2026-07-15)**: Grok-group keys accept Codex Responses WebSocket ingress and force the HTTP/SSE bridge (`requiredTransport=http_sse`, platform stays `grok`). Multi-turn/tool continuation no longer 501. Full upstream Grok WS cache/pool is still not imported.
- **Requirement B**: OpenAI-group keys do not list or schedule `grok-4.5` under platform isolation; independent of the WS bridge work.
- Platform for OpenAI-compatible routing is derived from `API Key -> Group.Platform`, not from the request `model` field.

### `/v1/chat/completions` Anthropic Compatibility

```
GatewayHandler.ChatCompletions
  -> parse Chat Completions body / model / stream
  -> SelectAccountWithLoadAwareness(...)
     -> compatibility context disables Antigravity mixed scheduling
     -> select native Anthropic account only
  -> ForwardAsChatCompletions()
     -> Chat Completions -> Responses -> Anthropic Messages
     -> build Anthropic upstream request
  -> upstream response -> Chat Completions response
  -> RecordUsage
```

### Antigravity Native Entry

```
/antigravity/v1/messages
  -> routes/gateway.go ClaudeGPTBridgeRoute() strict-routing diagnosis
  -> only when the diagnosis is not_configured (no bridge mapping intent):
  -> GatewayHandler.Messages
  -> SelectAccountWithLoadAwareness(... forcePlatform=antigravity ...)
  -> AntigravityGatewayService.Forward()
  -> Antigravity request/response transformer
```

### Antigravity `/v1/messages` OpenAI Claude-GPT Bridge (strict routing)

```
/v1/messages or /antigravity/v1/messages with API key bound to an Antigravity group
  -> routes/gateway.go
  -> OpenAIGatewayHandler.ClaudeGPTBridgeRoute()
     -> read and reset request body; protocol errors return canonical 400
     -> OpenAIGatewayService.ResolveClaudeGPTBridgeRoute()
        -> AccountRepository.ListByGroup diagnosis, no scheduler slots
        -> not_configured | ready | rate_limited | unavailable | probe_error
  -> not_configured: native Gateway.Messages (the ONLY native path)
  -> ready:
     -> OpenAIGatewayHandler.MessagesClaudeGPTBridge()
     -> group model access check uses the original Claude model
     -> SelectAccountWithSchedulerForClaudeGPTBridge()
     -> account.ResolveClaudeGPTBridgeModel(Claude model)
     -> ForwardAsAnthropic() reuses existing Claude -> OpenAI Responses -> Claude conversion
     -> RecordUsage with original Claude model as user-facing/billing model
  -> rate_limited: Anthropic 429 rate_limit_error + Retry-After (earliest recovery, ceil, min 1)
  -> unavailable: Anthropic 503 overloaded_error
  -> probe_error: Anthropic 503 api_error
```

The 2026-07-10 boolean-preflight defect is fixed: configured-but-temporarily-
unavailable bridges no longer masquerade as "no bridge" and never reach the
native Antigravity pool. If the real scheduler selection fails right after a
`ready` preflight (429 cooldown race) or the mapping is deleted mid-request,
`respondClaudeGPTBridgeSelectionRace` re-diagnoses once: pure rate limit
returns 429 + Retry-After, everything else returns a bridge-side 503. When all
bridge accounts are exhausted by upstream 429s, the final failover response
stays 429 and propagates a validated (positive integer, <= 86400s) upstream
`Retry-After`. Evidence and design are in
[../OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md](../OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md).
Route decisions emit the structured log event
`openai_claude_gpt_bridge.route_decision` with state/candidate counts,
`decision_source=preflight|selection_race|count_tokens_preflight`,
`attempt`, `terminal_outcome` (the route-layer terminal action, e.g.
`dispatch_native`, `dispatch_bridge`, `rate_limited_429`), and no account
identities.

Bridge candidacy requires the mapping hit to come from account-level
`credentials.model_mapping` (`ModelMappingSourceAccount`). Admin-configured
platform default mappings (`openai_default_model_mapping`, including
wildcards) never create bridge intent — they serve OpenAI-platform model
compatibility only. Client cancellation during a bridge Messages forward
returns early without recording an account failure, without an account
switch, and without continuing failover on the canceled context (same guard
as the Responses path).

### `/v1/messages/count_tokens` dispatch

```
POST /v1/messages/count_tokens or /antigravity/v1/messages/count_tokens
  -> routes/gateway.go countTokensHandler
  -> OpenAI group: OpenAIGatewayHandler.CountTokens
     -> convert Anthropic request via apicompat.AnthropicToResponses
     -> POST {base}/v1/responses/input_tokens (API-key base_url aware)
     -> OAuth 401/403/404 missing-scope/unsupported: local tiktoken estimate,
        never rate-limits, temp-unschedules, or errors the account
  -> Antigravity group with explicit bridge mapping:
     OpenAIGatewayHandler.CountTokensClaudeGPTBridge
     -> ready: bridge account counts upstream with the mapped GPT model
        (scheduler slot released immediately; bridge-lenient mode answers any
        upstream failure with a 200 local estimate while keeping
        HandleUpstreamError account bookkeeping)
     -> rate_limited/unavailable/probe_error: 200 local tiktoken estimate,
        no upstream call, never the native pool
  -> Antigravity group without bridge mapping / other platforms:
     native GatewayHandler.CountTokens (unchanged)
```

count_tokens never acquires user/account concurrency slots and never writes
usage or cost. The local estimator ports the official upstream tiktoken
implementation (o200k_base by default, cl100k_base for gpt-3.5/gpt-4-era
models); estimation sample expectations match upstream exactly. Converted
inputs larger than 8 MiB skip the tokenizer and use a bytes/4 approximation
(local-compute DoS guard). Estimate responses log
`count_tokens_estimated=true` with an `estimate_reason`.

The scheduler metadata cache must retain both
`credentials.model_mapping` and `extra.openai_claude_gpt_bridge_enabled`.
Bridge scheduling also performs a DB refresh for stale snapshot candidates
before rejecting them, so a recently-enabled bridge account is not incorrectly
treated as ineligible only because an older slim scheduler snapshot omitted the
bridge flag.

Once the bridge path has selected an OpenAI account and entered upstream
forwarding or failover, it does not fall back to Antigravity. It continues with
the existing OpenAI account failover rules so a failed GPT upstream attempt does
not double-charge or duplicate-send the same request through native
Antigravity.

The Messages bridge core follows upstream Sub2API/OpenAI behavior:
Anthropic-to-Responses conversion, Codex OAuth request-body transform,
tool-use/tool-result pairing, `response.failed` and missing-terminal handling,
Anthropic digest sessions, `previous_response_id` continuation, replay guard,
and Claude Code todo guard injection live in
`backend/internal/service/openai_gateway_messages.go` plus
`openai_messages_*.go`. The local overlay also owns Antigravity bridge
preflight/scheduler selection, preserving bridge body `prompt_cache_key`, bridge
usage fields, display cache override, display-token downstream rewriting, and
Claude Code compact recovery. Non-bridge OpenAI Messages should otherwise
follow upstream prompt-cache/session/continuation behavior.

For OpenAI OAuth, the Messages bridge marks request construction as
compatibility mode so it can preserve the bridge-specific body and
session/conversation rules. Immediately before dispatch, it restores the full
Codex identity tuple (`User-Agent`, `originator`, `version`, and
`OpenAI-Beta: responses=experimental`) and runs the final official-client
pairing check. This restoration is explicitly excluded from `platform=grok`;
Grok Messages continue through `buildGrokResponsesRequest` with the xAI
transport identity and no Codex `originator` or `version` leakage.

Claude Code emits a hidden compact request after the conversation reaches its
context threshold. The bridge keeps an untouched transcript snapshot before the
API-key 12-message replay guard. It buffers `message_start` and thinking until
visible text or tool output exists, so a failed, incomplete, missing-terminal,
or reasoning-only attempt can still switch OpenAI accounts instead of becoming
a successful empty Anthropic response. Standard `ping` events may be sent while
waiting for compact upstream headers or any silent Messages SSE body. An idle
timer resets only after bytes are flushed downstream, so the configured interval
is the actual maximum downstream silence before a ping even while the upstream
emits non-visible reasoning. Transport output is tracked separately from semantic
output so those pings do not block model/account failover.

If compact fails because the context is too large or produces no visible
summary, the bridge recovers from the full snapshot: it summarizes bounded
chunks, recursively bisects individual chunks that still exceed the context
window, and hierarchically merges the summaries. HTTP and SSE overflow usage is
accumulated across attempts. Recovery requests clear conversation/session and
turn-state headers, and recovered responses skip `previous_response_id`
binding because they use `store=false`. Accounts may configure
`compact_model_mapping` and `compact_model_fallbacks`; Spark compact requests
fall back to `gpt-5.4-mini` by default unless an explicit empty fallback list
disables that behavior. Recursive depth and split budgets bound upstream work,
with an emergency local capsule as the final merge fallback. Client cancellation
is bridged into these otherwise detached recovery requests so abandoned compact
work releases its upstream response and account concurrency slot.

The compact pre-header keepalive is a standard Anthropic `ping`. It keeps proxy
and TCP/SSE idle timers alive, but it cannot legally use the empty
`content_block_delta` watchdog reset introduced for newer Claude Code versions
until a real content block has started. Do not create a synthetic preamble block
only to send no-op deltas: doing so would commit semantic stream state and make
cross-account recovery unsafe.

Successful `message_stop` and downstream `event: error` writes mark the
Anthropic response terminal. Handler panic/error fallbacks consult that state so
they cannot append a second terminal event. A canceled downstream request exits
the Messages handler without reporting the selected account as unhealthy.

### Grok OpenAI-Compatible HTTP Flow

`platform=grok` groups enter the OpenAI-compatible handler for HTTP Responses,
Chat Completions, and Messages. The handler passes an explicit Grok platform to
the scheduler; candidate listing, sticky rechecks, DB rechecks, model capability
checks, and runtime blocking all preserve that platform boundary. The selected
account then uses the xAI OAuth/API-key token provider and Grok-specific forward
adapter.

The fork's Claude-GPT bridge remains a separate Antigravity-group path with
`RequireClaudeGPTBridge=true`; Grok routing does not participate in that bridge.
Grok `count_tokens` and WebSocket Responses fail explicitly instead of falling
through to another platform. Media service code is not registered as an HTTP
handler until the risk-control and billing persistence batches are complete.

### OpenAI Responses / Chat / WS Current Sync Point

#### Advanced scheduler control plane

The Settings KV `openai_advanced_scheduler_enabled` is the total gate. When it
is false, sticky weighting, subscription priority, DB `TopK`, and DB weight
overrides are ignored. Enabling it allows independent sticky-weighted and paid
ChatGPT subscription-priority modes plus overrides for `TopK` and nine score
weights. The settings API returns effective values, and the account list shows
base and per-group score snapshots computed from the effective weights.

All advanced candidates still pass fork-local platform, group, runtime, model,
compact, transport, endpoint/Images capability, and `RequireClaudeGPTBridge`
checks before ranking. Hard previous-response affinity also revalidates the
requested endpoint and Images capability. Weighted sticky fallback can use
only the filtered pool; stale out-of-group session bindings are deleted.

Subscription priority uses the non-secret scheduler metadata field
`credentials.plan_type`. Scheduler snapshots retain that field while still
removing access and refresh tokens. The scheduler tries subscription accounts
first and then regular accounts; if neither acquires a slot, it prefers a
regular wait plan and falls back to a busy subscription wait plan only when the
regular pool cannot serve the request.

`gateway.openai_scheduler` controls sticky escape independently of the Settings
KV total gate. It defaults to enabled with TTFT EWMA `15000ms` and error-rate
EWMA `0.5`; a full sticky slot also escapes. Escape selects a healthy fallback
without overwriting the original session binding. Setting
`sticky_escape_enabled: false` restores legacy sticky waiting.

The staged sync through Phase 8B intentionally keeps OpenAI/Codex hot paths
close to `upstream/main@be017445` while retaining local overlays. The currently
synced behavior includes:

- Responses and Chat Completions error semantics, including `response.failed`,
  SSE terminal reconstruction, silent-refusal failover, and upstream error
  passthrough where a downstream response has already been written.
- Request context propagation for async usage recording, client request id echo,
  upstream response-id account binding, and `completion_tokens_details`
  preservation.
- Responses WebSocket oversized HTTP bridge, rate-limit failover, account switch
  metrics, terminal-event timing corrections, and usage deduplication.
- Phase 8A group isolation: sticky session and response-id bindings must match
  the current group before reuse; stale `previous_response_id` values are
  stripped from the first WSv2 packet when the current group did not hit a
  previous-response binding and the payload is not a tool-output continuation.
- Phase 8B transport/header hardening: OpenAI transport errors without HTTP
  status codes return `UpstreamFailoverError`, persistent proxy/DNS/routing
  faults temporarily unschedule the account, API-key Chat Completions injects
  missing `prompt_cache_key` into the converted Responses body, and buffered
  non-streaming JSON outputs overwrite any upstream `text/event-stream`
  `Content-Type` before writing downstream.
- Request body hotpath hardening: parsed request maps are scoped to the exact
  body hash/length, released after validation/forwarding, and not retained just
  to extract scalar usage fields.
- OAuth runtime safety: upstream 401 invalidates caches and marks temporary
  unschedulable without persisting stale credentials over a concurrent token
  refresh.
- Codex/Claude Code mimicry: current CLI/package/runtime fingerprints, client
  metadata injection, count-tokens validation, billing-block recognition, and
  backend-only allowed-client hooks.

The Phase 6.5 billing change is deliberately narrow: long-context pricing now
applies the input-side multiplier to cache reads and cache creation when the
configured model pricing metadata already triggers long-context mode. It must
not write model prices, override global/user model pricing, or change display
pricing/display-token behavior.

### OpenAI Images Diagnostics

```
/v1/images/generations
  -> OpenAIGatewayHandler.Images
  -> ParseOpenAIImagesRequest
  -> NewOpenAIImageTrace (only OPENAI_IMAGE_TRACE_LOG=true, generations, gpt-image-2)
  -> SelectAccountWithSchedulerForImages
  -> ForwardImages
     -> API key upstream /v1/images/*, or OAuth Codex /responses image tool
     -> upstream response read
     -> downstream response write
  -> RecordUsage task submission
```

OpenAI Images endpoint scheduling also checks
`account.extra.openai_images_endpoint_enabled`. Only JSON boolean `false`
disables an account for `/v1/images/generations` and `/v1/images/edits`; missing
or invalid values remain enabled. This does not control Codex `/v1/responses`
`image_generation` tool injection, which remains governed by the separate Codex
image bridge setting.

For the 2026-06-30 production URL-response diagnostics, see
[`../OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md`](../OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md).
That record distinguishes Sub2API API latency from direct downstream image URL
download latency and tracks the native 4K `gpt-image-2` channel tests.

### OpenAI Embeddings

```
/v1/embeddings
  -> OpenAIGatewayHandler.Embeddings
  -> validate JSON body and requested model
  -> ResolveChannelMappingAndRestrict
  -> SelectAccountWithSchedulerForCapability(... OpenAIEndpointCapabilityEmbeddings)
     -> account must be OpenAI API-key type
     -> account.credentials.openai_capabilities must include embeddings, or be unset
     -> account model_mapping must support the requested embedding model
  -> OpenAIGatewayService.ForwardEmbeddings
     -> map model through account/channel mapping
     -> POST to account.GetOpenAIBaseURL() + /v1/embeddings
     -> pass upstream status/body through and extract usage
  -> RecordUsage
```

Embeddings are lower priority than the Codex/OpenAI main request path, but the
route is part of the synced OpenAI-compatible surface. Local real-request smoke
now reaches the configured OpenAI API-key account; the current dev fixture
returns `404 page not found` from its upstream `/v1/embeddings` endpoint, which
means the Sub2API route and account selection are working but the fixture base
URL or upstream service is not embeddings-compatible.

### API Key Usage Query

```
/v1/usage
  -> API key authentication only
  -> skip billing enforcement and group-assignment enforcement
  -> GatewayHandler.Usage
     -> quota_limited: key quota, rate-limit windows, expiry, and key usage summary
     -> unrestricted: subscription progress or wallet balance plus key usage summary

/v1/usage/records
/v1/usage/stats
/v1/usage/trend
  -> API key authentication only
  -> skip billing enforcement and group-assignment enforcement
  -> UsageHandler public methods
     -> force UserID and APIKeyID from the authenticated API key context
     -> return only this key's usage rows, selected-range stats, and trend data
```

This endpoint backs the public `/key-usage` frontend page. It must remain usable
without a logged-in browser session and must not require the API key to be
assigned to a scheduling group.

### Group Custom `/v1/models` List

```
GET /v1/models with API key
  -> GatewayHandler.Models
  -> resolve group platform, including any force-platform context
  -> if platform has a curated discovery list:
     -> OpenAI: gpt-5.6-sol, gpt-5.6-terra, gpt-5.6-luna, gpt-5.5, gpt-5.4, gpt-5.4-mini
     -> Antigravity: claude-opus-4-8, claude-opus-4-7, claude-opus-4-6, claude-haiku-4-5, claude-sonnet-4-6
     -> optionally narrow it with group.models_list_config
     -> return without consulting account model_mapping
  -> GatewayService.GetAvailableModels(group, platform)
  -> if group.models_list_config.enabled && models is non-empty:
     -> filter configured model IDs against available models
     -> fall back to platform default IDs only when no account-derived models exist
     -> return the configured order in OpenAI-compatible list shape
  -> otherwise return account-derived models or platform defaults
```

The group custom models list is presentation-only. It changes the response body
of `GET /v1/models` for that group, but it must not affect model allow/block
checks, model mapping, account scheduling, billing, usage recording, or the
Claude-GPT bridge. OpenAI and Antigravity have curated discovery lists that are
not expanded by account mappings; the group custom list can only narrow those
curated lists. The admin candidate endpoint
`GET /api/v1/admin/groups/:id/models-list-candidates` uses the same curated
lists for OpenAI and Antigravity, and uses schedulable account model mappings
or platform defaults for other platforms. Saving the setting persists only
`groups.models_list_config`.

When OpenAI curated discovery grows, stale custom lists that exactly represent a
previous full default set are treated as compatibility lists, not as an explicit
request to hide newly curated models. For example, the legacy full OpenAI list
`gpt-5.5`, `gpt-5.4`, `gpt-5.4-mini` is expanded at runtime to include
`gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna`. Intentionally narrowed
custom lists stay narrowed.

OpenAI `/v1/models` entries include Codex-compatible optional metadata:
`supported_endpoint_types`, `supported_session_modes`, `actual_model_returned`,
`input_modalities`, `output_modalities`, and `supported_modalities`. These
fields are presentation/discovery hints for Codex-style clients only; they do
not change model routing, access checks, account scheduling, billing, or usage
recording.

OpenAI Claude-GPT bridge mappings are intentionally hidden from OpenAI-platform
`/v1/models` output when the mapping key is a Claude-family request model and
the mapped upstream model is a distinct GPT/OpenAI model. This keeps bridge-only
Claude presentation names out of downstream OpenAI key discovery while leaving
normal OpenAI aliases and Antigravity bridge scheduling unchanged.

### Codex Models Manifest

```
GET /v1/models?client_version=... with an OpenAI-group API key
  -> GatewayHandler.Models
  -> detect the Codex client_version query
  -> OpenAIGatewayHandler.CodexModels

GET /backend-api/codex/models with an OpenAI-group API key
  -> existing gateway authentication and group middleware
  -> OpenAIGatewayHandler.CodexModels
     -> OpenAIGatewayService.SelectAccountForCodexModels(group)
        -> select a schedulable OpenAI OAuth account only
        -> skip OpenAI API-key accounts in mixed groups
     -> OpenAIGatewayService.FetchCodexModelsManifest(...)
        -> GET https://chatgpt.com/backend-api/codex/models
        -> forward client_version, OAuth/account headers, Codex headers,
           If-None-Match, and the selected account proxy
        -> pass through the manifest body, ETag, and 304 status
```

The Codex desktop custom-provider flow requests `{base_url}/models` and uses
the `client_version` query to distinguish its manifest request. Because the
documented Sub2API Codex `base_url` ends in `/v1`, the primary compatibility
route is `/v1/models?client_version=...`; `/backend-api/codex/models` is also
available for clients that call the native ChatGPT path directly. An ordinary
`GET /v1/models` without `client_version` continues to return the curated
OpenAI-compatible list described above.

Manifest discovery reflects the selected OAuth account's live ChatGPT/Codex
entitlements, so it intentionally bypasses `groups.models_list_config`. It does
not change model access checks, request routing, billing, or usage recording.
The proxy rejects bodies larger than 8 MiB rather than returning truncated
JSON. Fetch failures are returned immediately so the Codex client can use its
own cached manifest fallback.

## Important Mechanisms

### HTTP 200 `response.failed` handling

OpenAI Responses, Responses passthrough, Chat Completions conversion, and
Anthropic Messages conversion treat `response.failed` as a failed terminal
event even though the upstream HTTP status is 200. Before any client output,
the gateway normalizes `response.error`, infers a semantic status, and applies
the configured error-passthrough rules using the selected account's platform.
An OpenAI rule cannot match a Grok account and vice versa.

Transient capacity failures still return an `UpstreamFailoverError` before
client output. Context-window and other client errors do not switch accounts;
Messages preserves its client-error envelope, while a configured passthrough
rule may override status/message. Once visible output has been emitted, the
gateway sends an in-band protocol error and must not replay the request through
another account.

Ordinary failed terminals never become a successful `OpenAIForwardResult`, so
handlers do not submit successful usage recording or billing. The existing
`cyber_policy` audit path remains separate and retains its real upstream usage
snapshot. Display-token rewriting, real cache-read quantities, stored
`actual_cost`, Claude-GPT bridge cache overrides, Images, Batch Image, WebSocket
forwarding, and scheduler eligibility are unchanged.

### Codex image namespace declarations

Codex clients can advertise image generation either as the flat Responses
`image_generation` tool or as an `image_gen` namespace. The namespace can
appear in top-level `tools`, in a Responses Lite `input[].additional_tools`
carrier, or in `tool_choice`.

- Image-intent checks recognize all three locations.
- The existing Spark compatibility strip removes only flat image tools and the
  exact `image_gen` namespace declarations. Empty `additional_tools` carriers
  and an image-only `tool_choice` are removed with them.
- Ordinary custom tools named `imagegen`, `tool_search`, and every other
  namespace remain unchanged. This preserves the fork's Codex 0.1.151
  custom/tool-search/namespace Chat fallback bridge.
- The strip is protocol normalization only. It does not route requests through
  native/basic OpenAI Images, change Batch Image, recalculate billing, rewrite
  display/cache-read usage, or alter Claude-GPT bridge eligibility, model
  fallback, account scheduling, or Ops attribution.

| Mechanism | Notes |
|-----------|-------|
| Mixed scheduling | Anthropic/Gemini groups may include Antigravity accounts with `mixed_scheduling=true`, but only entry points with an Antigravity conversion branch should use them. |
| Chat Completions isolation | `/v1/chat/completions` currently converts only to Anthropic Messages upstream. It must disable Antigravity mixed scheduling, otherwise an Antigravity OAuth token can be sent to Anthropic and return 401 `Invalid bearer token`. |
| Group model access control | Handlers reject models blocked by the group blacklist or not present in a non-empty whitelist before account selection. Responses payloads also validate `tools[].type == "image_generation"` entries with an explicit `model`, so image tools cannot bypass group restrictions. |
| OpenAI Claude-GPT bridge | Antigravity `/v1/messages` resolves a strict route decision before dispatch. Bridge configuration intent (enabled account extra + explicit Claude model mapping on a bound OpenAI account) is separated from instantaneous schedulability: only `not_configured` reaches native; `rate_limited` returns 429 + Retry-After and `unavailable`/`probe_error` return 503, implementing the 2026-07-10 strict-routing plan. The conversion core follows upstream Messages behavior; local overlay owns Antigravity dispatch, bridge header stripping, usage/display semantics, scheduler eligibility, and compact recovery. |
| Haiku→GPT empty-output mitigation | Haiku-class Claude models default to reasoning effort `low` when `output_config.effort` is unset. After bridge upstream model assignment, `ApplyClaudeHaikuBridgeUpstreamAdjustments` floors `max_output_tokens` at 1024 for Haiku→GPT-5.* and strips sampling params. Empty completed streams/non-stream conversions set `UpstreamFailoverError.NoAccountFailover` so the OpenAI Messages handler does not multi-account switch on request-shaped failures. |
| Public usage query | `/v1/usage*` uses API key authentication but intentionally skips billing enforcement and group-assignment enforcement so users can inspect exhausted, expired, or ungrouped keys. Public records/stats/trend endpoints must force the authenticated API key ID server-side and must not accept a user-supplied API key ID. |
| OAuth 401 recovery | OAuth accounts should invalidate token cache, force refresh, and become temporarily unschedulable on 401. They should not go directly to permanent `SetError`. Antigravity OAuth follows the same rule. |
| Optional Gin context | Internal Anthropic forwarding helpers can be called without a Gin context. User-Agent inspection, identity metadata, and tool-rewrite context storage must be guarded; normal HTTP paths retain their existing behavior. |
| Frontend session/payment state | Refresh requests use a finite timeout, logout clears local credentials even when server revocation fails, and payment/risk-control route guards load public settings before deciding. Payment status, QR, and Stripe popup polling must remain single-flight; Stripe polling reads the canonical `auth_token` key and popup initialization cancels its fallback timeout. |
| Sticky sessions | Selection may prefer a session-bound account, but the account still has to pass platform, model, rate limit, quota, cost-window, and group-membership checks. Local response-id account bindings are namespaced by group to avoid cross-group previous-response reuse. |
| Codex WS continuation mobility | A Responses WebSocket request may move away from its `previous_response_id` account only when every tool-output `call_id` is covered by an in-band tool-call context item or matching `item_reference`. Partial coverage keeps hard affinity so upstream can resolve the missing call from the response chain. |
| Scheduler quota headroom | `gateway.openai_ws.scheduler_score_weights.quota_headroom` is opt-in (`0` by default). Fresh Codex 7d/5h snapshots influence advanced-scheduler scores; missing or older-than-8h snapshots are neutral, and a 5h window below 10% remaining reduces the factor. This changes account selection only, never billing or quota deduction. |
| Advanced scheduler rollback | `openai_advanced_scheduler_enabled=false` disables both submodes and all DB TopK/weight overrides. It does not disable the base scheduler configured under `gateway.openai_ws`. |
| Sticky escape | `gateway.openai_scheduler.sticky_escape_*` is a config-level safety policy. Escape keeps the old session binding intact and never bypasses group, platform, capability, bridge, or runtime eligibility. |
| OpenAI effort metadata candidates | Usage recording checks mapped/upstream model first and then billing/original model names. This preserves GPT-5.6 `max` and suffix-derived effort when OAuth normalization removes the suffix; it does not rewrite requests or alter price calculation. |
| OpenAI image trace logs | `OPENAI_IMAGE_TRACE_LOG=true` emits structured `openai.images.trace` events for `/v1/images/generations` with `model=gpt-image-2` only. Fields are limited to safe timing/correlation data (`request_id`, `client_request_id`, `trace_id`, `account_id`, model, size, quality, stream, status, timestamps, upstream request id); prompts, image bytes/base64, auth headers, cookies, API keys, and full bodies must not be logged. |
| OpenAI Images account opt-out | `extra.openai_images_endpoint_enabled=false` excludes an OpenAI OAuth/API-key account from independent `/v1/images/*` scheduling only. It must not disable OpenAI chat/responses/embeddings, Claude-GPT bridge, or Codex `/v1/responses` image tool injection. |
| OpenAI endpoint capabilities | `credentials.openai_capabilities` restricts OpenAI API-key endpoint scheduling for chat completions and embeddings. Missing config means default capabilities are allowed. This is independent from Images endpoint opt-out and Codex image-generation bridge settings. |
| Group custom models list | `groups.models_list_config` only customizes `GET /v1/models` output. For OpenAI and Antigravity it can only narrow the curated discovery lists, except stale full-default OpenAI lists are expanded to include newly curated GPT-5.6 models. It is ignored by scheduling and billing paths; model access continues to use group allow/block lists and account capabilities. |
| Codex model discovery metadata | OpenAI `/v1/models` response objects include optional Codex client capability fields so custom-provider model pickers can recognize Responses and Chat Completions support. These fields are not authoritative for backend scheduling or billing. |
| Codex models manifest | OpenAI-group requests to `/v1/models?client_version=...` and `/backend-api/codex/models` proxy the selected OpenAI OAuth account's ChatGPT manifest. API-key accounts are ineligible for this discovery path; ETag/304 and an 8 MiB response limit are preserved. |
| HTTPUpstream network retry | `repository.HTTPUpstream` retries transport/network failures that happen before an HTTP response is received, such as connection reset, timeout, EOF, DNS failure, and `Network error. Please check your connection.`. Admin settings key `gateway_network_retry_max` controls retries (`0..10`, default `2`). It does not retry upstream HTTP 4xx/5xx responses and only retries requests whose body can be replayed. Paths with their own explicit retry loop, such as the OpenAI OAuth image `/responses` tool path, can disable this global retry through request context to avoid multiplicative retries. |
| Anthropic OAuth dateline normalization | Admin Settings KV `enable_client_dateline_normalization` defaults to true. After existing Anthropic OAuth body rewrites, the gateway canonicalizes supported hidden apostrophe/date-separator variants in top-level system text and tagged `<system-reminder>` text. It applies only to Anthropic OAuth/Setup Token accounts; API Key, OpenAI Claude-GPT bridge, user prose, tool content, billing/display/cache-read fields, models, and scheduling are outside its scope. An explicit false disables it. |
| OpenAI Images upstream 400 passthrough | `OpenAIGatewayHandler.Images` binds an Images request context after parsing `/v1/images/*`. `OpenAIGatewayService.handleErrorResponse` uses that context to return upstream 400 user errors, such as invalid image dimensions, as downstream 400 with the upstream `error.message` and `error.type` instead of masking them as generic 502. Keep this scoped to Images requests. |
| OpenAI OAuth image timeout/retry | The Codex `/responses` image tool path retries fast no-header transport failures up to 3 total attempts with short backoff. It also wraps the full upstream wait/body read in an image-generation timeout: 1K = 180s, 2K = 240s, 4K/unknown = 360s. Timeout errors return `image_generation_timeout` (504) before any non-streaming response is written; no-header retry exhaustion returns `image_generation_upstream_unreachable` (502). |
| OpenAI Images response delivery | Non-streaming `/v1/images/*` responses use an 8 MiB memory threshold and spill larger bodies to a temporary file, while `gateway.upstream_response_read_max_bytes` remains a 128 MiB total bound. Local spool creation/write failure or exceeding that configured total bound happens after upstream generation: it returns `local_delivery_error` without account failover or an account-health penalty, avoiding duplicate generation and billing. Upstream body read interruption remains eligible for failover before any downstream bytes are committed. |

## Known Pitfalls

- **Codex manifest URL depends on `/v1`**: for a Codex custom provider, configure `base_url` as the Sub2API origin plus `/v1`. A root-only URL makes the desktop client request `/models`, which this compatibility route does not register. Manifest discovery also requires at least one schedulable OpenAI OAuth account in the key's group; OpenAI API-key accounts cannot provide the ChatGPT manifest.
- **Bridge cooldown is not a bridge miss**: fixed by the 2026-07-10 strict routing. `ResolveClaudeGPTBridgeRoute()` classifies "configured but temporarily blocked" as `rate_limited`/`unavailable` (bridge 429/503) instead of `false`-into-native. When debugging bridge errors, read the `openai_claude_gpt_bridge.route_decision` log event (state, candidate/schedulable/rate-limited counts, retry_at, decision_source) before touching account state. An admin who wants a group to genuinely return to native must remove the bridge mapping or disable the account bridge switch — pausing the account (`schedulable=false`) now yields bridge 503, not native fallback. See [the investigation and repair design](../OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md).
- **Empty completed Haiku bridge is request-shaped**: Claude Code Haiku background tasks with large context and small `max_tokens` mapped onto GPT-5.* medium reasoning often complete with zero visible assistant text. Do not multi-account failover that class of failure; fix request shape (low effort + higher max_output floor) instead of burning the pool. Log marker: `openai_messages.empty_visible_output_no_account_failover`.
- **Compatibility path selecting Antigravity**: `/v1/chat/completions` is not an Antigravity native entry point. If it selects an Antigravity account, the request can send an Antigravity bearer token to Anthropic upstream and produce `Authentication failed (401): Invalid bearer token`, while the same account remains usable on `/antigravity/v1/messages`.
- **Antigravity OAuth 401 false positive**: Antigravity OAuth 401 does not always mean the refresh token is invalid. It can be caused by protocol/upstream path mismatch, stale token cache, or a transient upstream state. Use temporary unschedulable plus token refresh instead of permanent `status=error`.
- **Image trace is temporary and opt-in**: Keep `OPENAI_IMAGE_TRACE_LOG` disabled by default. It is for targeted local/production timing windows and should be turned off after sampling.
- **OpenAI Images 400s are usually client input**: Invalid image size, unsupported image options, and similar upstream 400s should remain visible to the client on `/v1/images/*`. Do not require `OPENAI_IMAGE_TRACE_LOG` for this behavior.
- **OpenAI image timeout is not account failover**: The first version treats long generation timeout as a client-facing 504 instead of switching accounts, because previous sampling showed the elapsed time is dominated by upstream generation rather than Sub2API scheduling. Revisit only with evidence that account switching materially improves completion probability.
- **Embeddings fixture has two gates**: scheduler selection requires both endpoint capability and model support. If `/v1/embeddings` returns 503 `no available accounts`, check `credentials.model_mapping` for the requested embedding model before debugging the forwarder. If it returns upstream 404, check the account base URL and upstream service support for `/v1/embeddings`.
- **Future upstream syncs must stay staged**: OpenAI/Codex, billing, quota, risk-control, and account-page updates should continue as small batches with guard/unit/smoke gates. Avoid all-at-once merges that can silently delete local secondary-development features.
