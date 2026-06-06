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
| APIKey.group_id | `backend/internal/service/api_key.go` | Scheduling group bound to the user request. |

## Key Files

| Layer | File | Responsibility |
|-------|------|----------------|
| Handler | `backend/internal/handler/gateway_handler.go` | Anthropic Messages, Gemini compatibility, and Antigravity native entry points. |
| Handler | `backend/internal/handler/gateway_handler_chat_completions.go` | `/v1/chat/completions` compatibility entry for Anthropic groups. |
| Handler | `backend/internal/handler/group_model_access.go` | Shared group model access checks, including Responses image tool validation. |
| Handler | `backend/internal/handler/openai_gateway_handler.go` | OpenAI-compatible gateway plus Anthropic Messages bridge for OpenAI and Antigravity bridge preflight. |
| Handler | `backend/internal/handler/openai_images.go` | OpenAI-compatible `/v1/images/*` entry, image request parsing orchestration, scheduling, and usage submission. |
| Service | `backend/internal/service/gateway_service.go` | Account selection, mixed scheduling, sticky sessions, and Anthropic upstream request building. |
| Service | `backend/internal/service/gateway_forward_as_chat_completions.go` | Chat Completions -> Responses -> Anthropic Messages conversion and forwarding. |
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

## Important Mechanisms

| Mechanism | Notes |
|-----------|-------|
| Mixed scheduling | Anthropic/Gemini groups may include Antigravity accounts with `mixed_scheduling=true`, but only entry points with an Antigravity conversion branch should use them. |
| Chat Completions isolation | `/v1/chat/completions` currently converts only to Anthropic Messages upstream. It must disable Antigravity mixed scheduling, otherwise an Antigravity OAuth token can be sent to Anthropic and return 401 `Invalid bearer token`. |
| Group model access control | Handlers reject models blocked by the group blacklist or not present in a non-empty whitelist before account selection. Responses payloads also validate `tools[].type == "image_generation"` entries with an explicit `model`, so image tools cannot bypass group restrictions. |
| OpenAI Claude-GPT bridge | Antigravity `/v1/messages` can preflight OpenAI bridge accounts bound to the same Antigravity group. Eligibility requires `RequireClaudeGPTBridge`, enabled account extra, and a Claude model mapping hit on the OpenAI account. The conversion core follows upstream Messages behavior; local overlay only controls Antigravity dispatch, bridge header stripping, bridge usage/display semantics, and scheduler eligibility. |
| Public usage query | `/v1/usage*` uses API key authentication but intentionally skips billing enforcement and group-assignment enforcement so users can inspect exhausted, expired, or ungrouped keys. Public records/stats/trend endpoints must force the authenticated API key ID server-side and must not accept a user-supplied API key ID. |
| OAuth 401 recovery | OAuth accounts should invalidate token cache, force refresh, and become temporarily unschedulable on 401. They should not go directly to permanent `SetError`. Antigravity OAuth follows the same rule. |
| Sticky sessions | Selection may prefer a session-bound account, but the account still has to pass platform, model, rate limit, quota, and cost-window checks. |
| OpenAI image trace logs | `OPENAI_IMAGE_TRACE_LOG=true` emits structured `openai.images.trace` events for `/v1/images/generations` with `model=gpt-image-2` only. Fields are limited to safe timing/correlation data (`request_id`, `client_request_id`, `trace_id`, `account_id`, model, size, quality, stream, status, timestamps, upstream request id); prompts, image bytes/base64, auth headers, cookies, API keys, and full bodies must not be logged. |
| OpenAI Images account opt-out | `extra.openai_images_endpoint_enabled=false` excludes an OpenAI OAuth/API-key account from independent `/v1/images/*` scheduling only. It must not disable OpenAI chat/responses/embeddings, Claude-GPT bridge, or Codex `/v1/responses` image tool injection. |
| OpenAI Images upstream 400 passthrough | `OpenAIGatewayHandler.Images` binds an Images request context after parsing `/v1/images/*`. `OpenAIGatewayService.handleErrorResponse` uses that context to return upstream 400 user errors, such as invalid image dimensions, as downstream 400 with the upstream `error.message` and `error.type` instead of masking them as generic 502. Keep this scoped to Images requests. |
| OpenAI OAuth image timeout/retry | The Codex `/responses` image tool path retries fast no-header transport failures up to 3 total attempts with short backoff. It also wraps the full upstream wait/body read in an image-generation timeout: 1K = 180s, 2K = 240s, 4K/unknown = 360s. Timeout errors return `image_generation_timeout` (504) before any non-streaming response is written; no-header retry exhaustion returns `image_generation_upstream_unreachable` (502). |

## Known Pitfalls

- **Compatibility path selecting Antigravity**: `/v1/chat/completions` is not an Antigravity native entry point. If it selects an Antigravity account, the request can send an Antigravity bearer token to Anthropic upstream and produce `Authentication failed (401): Invalid bearer token`, while the same account remains usable on `/antigravity/v1/messages`.
- **Antigravity OAuth 401 false positive**: Antigravity OAuth 401 does not always mean the refresh token is invalid. It can be caused by protocol/upstream path mismatch, stale token cache, or a transient upstream state. Use temporary unschedulable plus token refresh instead of permanent `status=error`.
- **Image trace is temporary and opt-in**: Keep `OPENAI_IMAGE_TRACE_LOG` disabled by default. It is for targeted local/production timing windows and should be turned off after sampling.
- **OpenAI Images 400s are usually client input**: Invalid image size, unsupported image options, and similar upstream 400s should remain visible to the client on `/v1/images/*`. Do not require `OPENAI_IMAGE_TRACE_LOG` for this behavior.
- **OpenAI image timeout is not account failover**: The first version treats long generation timeout as a client-facing 504 instead of switching accounts, because previous sampling showed the elapsed time is dominated by upstream generation rather than Sub2API scheduling. Revisit only with evidence that account switching materially improves completion probability.
