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
| APIKey.group_id | `backend/internal/service/api_key.go` | Scheduling group bound to the user request. |

## Key Files

| Layer | File | Responsibility |
|-------|------|----------------|
| Handler | `backend/internal/handler/gateway_handler.go` | Anthropic Messages, Gemini compatibility, and Antigravity native entry points. |
| Handler | `backend/internal/handler/gateway_handler_chat_completions.go` | `/v1/chat/completions` compatibility entry for Anthropic groups. |
| Handler | `backend/internal/handler/group_model_access.go` | Shared group model access checks, including Responses image tool validation. |
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
  -> GatewayHandler.Messages
  -> SelectAccountWithLoadAwareness(... forcePlatform=antigravity ...)
  -> AntigravityGatewayService.Forward()
  -> Antigravity request/response transformer
```

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

## Important Mechanisms

| Mechanism | Notes |
|-----------|-------|
| Mixed scheduling | Anthropic/Gemini groups may include Antigravity accounts with `mixed_scheduling=true`, but only entry points with an Antigravity conversion branch should use them. |
| Chat Completions isolation | `/v1/chat/completions` currently converts only to Anthropic Messages upstream. It must disable Antigravity mixed scheduling, otherwise an Antigravity OAuth token can be sent to Anthropic and return 401 `Invalid bearer token`. |
| Group model access control | Handlers reject models blocked by the group blacklist or not present in a non-empty whitelist before account selection. Responses payloads also validate `tools[].type == "image_generation"` entries with an explicit `model`, so image tools cannot bypass group restrictions. |
| OAuth 401 recovery | OAuth accounts should invalidate token cache, force refresh, and become temporarily unschedulable on 401. They should not go directly to permanent `SetError`. Antigravity OAuth follows the same rule. |
| Sticky sessions | Selection may prefer a session-bound account, but the account still has to pass platform, model, rate limit, quota, and cost-window checks. |
| OpenAI image trace logs | `OPENAI_IMAGE_TRACE_LOG=true` emits structured `openai.images.trace` events for `/v1/images/generations` with `model=gpt-image-2` only. Fields are limited to safe timing/correlation data (`request_id`, `client_request_id`, `trace_id`, `account_id`, model, size, quality, stream, status, timestamps, upstream request id); prompts, image bytes/base64, auth headers, cookies, API keys, and full bodies must not be logged. |

## Known Pitfalls

- **Compatibility path selecting Antigravity**: `/v1/chat/completions` is not an Antigravity native entry point. If it selects an Antigravity account, the request can send an Antigravity bearer token to Anthropic upstream and produce `Authentication failed (401): Invalid bearer token`, while the same account remains usable on `/antigravity/v1/messages`.
- **Antigravity OAuth 401 false positive**: Antigravity OAuth 401 does not always mean the refresh token is invalid. It can be caused by protocol/upstream path mismatch, stale token cache, or a transient upstream state. Use temporary unschedulable plus token refresh instead of permanent `status=error`.
- **Image trace is temporary and opt-in**: Keep `OPENAI_IMAGE_TRACE_LOG` disabled by default. It is for targeted local/production timing windows and should be turned off after sampling.
