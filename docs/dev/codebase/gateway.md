# API Gateway

> Unified API entry points, account scheduling, upstream protocol conversion,
> failover, and usage recording.

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
| Service | `backend/internal/service/openai_image_trace.go` | Temporary `OPENAI_IMAGE_TRACE_LOG` diagnostics for `gpt-image-2` generations. |
| Service | `backend/internal/service/antigravity_gateway_service.go` | Antigravity native request/response conversion and forwarding. |
| Service | `backend/internal/service/ratelimit_service.go` | Maps upstream errors to account state, temporary unschedulable windows, and rate limits. |

## Core Flow

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
  -> routes/gateway.go bridge preflight for eligible Claude-GPT requests
  -> if no OpenAI bridge account is eligible:
  -> GatewayHandler.Messages
  -> SelectAccountWithLoadAwareness(... forcePlatform=antigravity ...)
  -> AntigravityGatewayService.Forward()
  -> Antigravity request/response transformer
```

### Antigravity `/v1/messages` OpenAI Claude-GPT Bridge

```
/v1/messages or /antigravity/v1/messages with API key bound to an Antigravity group
  -> routes/gateway.go
  -> OpenAIGatewayHandler.ShouldUseClaudeGPTBridge()
     -> read and reset request body
     -> parse original Claude request model
     -> preflight OpenAI scheduler with RequireClaudeGPTBridge=true
     -> release any acquired preflight slot
  -> if bridge account exists:
     -> OpenAIGatewayHandler.MessagesClaudeGPTBridge()
     -> group model access check uses the original Claude model
     -> SelectAccountWithSchedulerForClaudeGPTBridge()
     -> account.ResolveClaudeGPTBridgeModel(Claude model)
     -> ForwardAsAnthropic() reuses existing Claude -> OpenAI Responses -> Claude conversion
     -> RecordUsage with original Claude model as user-facing/billing model
  -> if preflight or first selection fails before OpenAI upstream:
     -> reset request body
     -> fall back to native Gateway.Messages Antigravity path
```

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
`openai_messages_*.go`. The local overlay is intentionally smaller: Antigravity
bridge preflight/scheduler selection, preserving bridge body
`prompt_cache_key`, stripping upstream `session_id` and `conversation_id`
headers after request construction, bridge usage fields, display cache override,
and display-token downstream rewriting. Non-bridge OpenAI Messages should follow
upstream prompt-cache/session/continuation behavior.

### OpenAI Responses / Chat / WS Current Sync Point

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
     -> OpenAI: gpt-5.5, gpt-5.4, gpt-5.4-mini
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

OpenAI Claude-GPT bridge mappings are intentionally hidden from OpenAI-platform
`/v1/models` output when the mapping key is a Claude-family request model and
the mapped upstream model is a distinct GPT/OpenAI model. This keeps bridge-only
Claude presentation names out of downstream OpenAI key discovery while leaving
normal OpenAI aliases and Antigravity bridge scheduling unchanged.

## Important Mechanisms

| Mechanism | Notes |
|-----------|-------|
| Mixed scheduling | Anthropic/Gemini groups may include Antigravity accounts with `mixed_scheduling=true`, but only entry points with an Antigravity conversion branch should use them. |
| Chat Completions isolation | `/v1/chat/completions` currently converts only to Anthropic Messages upstream. It must disable Antigravity mixed scheduling, otherwise an Antigravity OAuth token can be sent to Anthropic and return 401 `Invalid bearer token`. |
| Group model access control | Handlers reject models blocked by the group blacklist or not present in a non-empty whitelist before account selection. Responses payloads also validate `tools[].type == "image_generation"` entries with an explicit `model`, so image tools cannot bypass group restrictions. |
| OpenAI Claude-GPT bridge | Antigravity `/v1/messages` can preflight OpenAI bridge accounts bound to the same Antigravity group. Eligibility requires `RequireClaudeGPTBridge`, enabled account extra, and a Claude model mapping hit on the OpenAI account. The conversion core follows upstream Messages behavior; local overlay only controls Antigravity dispatch, bridge header stripping, bridge usage/display semantics, and scheduler eligibility. |
| Public usage query | `/v1/usage*` uses API key authentication but intentionally skips billing enforcement and group-assignment enforcement so users can inspect exhausted, expired, or ungrouped keys. Public records/stats/trend endpoints must force the authenticated API key ID server-side and must not accept a user-supplied API key ID. |
| OAuth 401 recovery | OAuth accounts should invalidate token cache, force refresh, and become temporarily unschedulable on 401. They should not go directly to permanent `SetError`. Antigravity OAuth follows the same rule. |
| Sticky sessions | Selection may prefer a session-bound account, but the account still has to pass platform, model, rate limit, quota, cost-window, and group-membership checks. Local response-id account bindings are namespaced by group to avoid cross-group previous-response reuse. |
| OpenAI image trace logs | `OPENAI_IMAGE_TRACE_LOG=true` emits structured `openai.images.trace` events for `/v1/images/generations` with `model=gpt-image-2` only. Fields are limited to safe timing/correlation data (`request_id`, `client_request_id`, `trace_id`, `account_id`, model, size, quality, stream, status, timestamps, upstream request id); prompts, image bytes/base64, auth headers, cookies, API keys, and full bodies must not be logged. |
| OpenAI Images account opt-out | `extra.openai_images_endpoint_enabled=false` excludes an OpenAI OAuth/API-key account from independent `/v1/images/*` scheduling only. It must not disable OpenAI chat/responses/embeddings, Claude-GPT bridge, or Codex `/v1/responses` image tool injection. |
| OpenAI endpoint capabilities | `credentials.openai_capabilities` restricts OpenAI API-key endpoint scheduling for chat completions and embeddings. Missing config means default capabilities are allowed. This is independent from Images endpoint opt-out and Codex image-generation bridge settings. |
| Group custom models list | `groups.models_list_config` only customizes `GET /v1/models` output. For OpenAI and Antigravity it can only narrow the curated discovery lists. It is ignored by scheduling and billing paths; model access continues to use group allow/block lists and account capabilities. |
| OpenAI Images upstream 400 passthrough | `OpenAIGatewayHandler.Images` binds an Images request context after parsing `/v1/images/*`. `OpenAIGatewayService.handleErrorResponse` uses that context to return upstream 400 user errors, such as invalid image dimensions, as downstream 400 with the upstream `error.message` and `error.type` instead of masking them as generic 502. Keep this scoped to Images requests. |
| OpenAI OAuth image timeout/retry | The Codex `/responses` image tool path retries fast no-header transport failures up to 3 total attempts with short backoff. It also wraps the full upstream wait/body read in an image-generation timeout: 1K = 180s, 2K = 240s, 4K/unknown = 360s. Timeout errors return `image_generation_timeout` (504) before any non-streaming response is written; no-header retry exhaustion returns `image_generation_upstream_unreachable` (502). |

## Known Pitfalls

- **Compatibility path selecting Antigravity**: `/v1/chat/completions` is not an Antigravity native entry point. If it selects an Antigravity account, the request can send an Antigravity bearer token to Anthropic upstream and produce `Authentication failed (401): Invalid bearer token`, while the same account remains usable on `/antigravity/v1/messages`.
- **Antigravity OAuth 401 false positive**: Antigravity OAuth 401 does not always mean the refresh token is invalid. It can be caused by protocol/upstream path mismatch, stale token cache, or a transient upstream state. Use temporary unschedulable plus token refresh instead of permanent `status=error`.
- **Image trace is temporary and opt-in**: Keep `OPENAI_IMAGE_TRACE_LOG` disabled by default. It is for targeted local/production timing windows and should be turned off after sampling.
- **OpenAI Images 400s are usually client input**: Invalid image size, unsupported image options, and similar upstream 400s should remain visible to the client on `/v1/images/*`. Do not require `OPENAI_IMAGE_TRACE_LOG` for this behavior.
- **OpenAI image timeout is not account failover**: The first version treats long generation timeout as a client-facing 504 instead of switching accounts, because previous sampling showed the elapsed time is dominated by upstream generation rather than Sub2API scheduling. Revisit only with evidence that account switching materially improves completion probability.
- **Embeddings fixture has two gates**: scheduler selection requires both endpoint capability and model support. If `/v1/embeddings` returns 503 `no available accounts`, check `credentials.model_mapping` for the requested embedding model before debugging the forwarder. If it returns upstream 404, check the account base URL and upstream service support for `/v1/embeddings`.
- **Future upstream syncs must stay staged**: OpenAI/Codex, billing, quota, risk-control, and account-page updates should continue as small batches with guard/unit/smoke gates. Avoid all-at-once merges that can silently delete local secondary-development features.
