# Risk control and content moderation

## Data model

- `content_moderation_logs` stores redacted moderation outcomes, matched keywords, violation counters, notification state, queue delay, and upstream latency.
- Runtime configuration is stored in Settings KV under `risk_control_enabled`, `content_moderation_config`, `cyber_session_block_enabled`, and `cyber_session_block_ttl_seconds`.
- Flagged input hashes and cyber session blocks use Redis. They are enforcement caches, not billing records.
- Cyber-blocked upstream turns use usage request type `4` (`cyber`) so real upstream token usage remains auditable.

## Key files

- Backend service: `backend/internal/service/content_moderation*.go`, `openai_cyber_policy.go`, `openai_cyber_session_block.go`.
- Persistence/cache: `backend/internal/repository/content_moderation_repo.go`, `content_moderation_hash_cache.go`, `gateway_cache.go`.
- Gateway integration: `backend/internal/handler/content_moderation_helper.go`, gateway protocol handlers, `openai_gateway_handler.go`, and `openai_images.go`.
- Admin API: `backend/internal/handler/admin/content_moderation_handler.go` and `backend/internal/server/routes/admin.go`.
- Frontend: `frontend/src/views/admin/RiskControlView.vue`, `frontend/src/api/admin/riskControl.ts`, router/sidebar feature gating, and Settings feature switches.

## Core flow

1. The gateway extracts the latest user content for Anthropic Messages, OpenAI Responses/Chat, Gemini, WebSocket Responses, or OpenAI Images.
2. When risk control and moderation are enabled for the group/model, pre-block mode evaluates keywords/hashes and then the configured moderation API according to strategy.
3. A preflight block returns before billing eligibility, concurrency acquisition, account scheduling, or upstream forwarding. Therefore it must not deduct quota or create a normal usage charge.
4. Observe mode queues redacted audit work without changing the forwarded request or billing result.
5. Upstream `cyber_policy` failures are passed through without failover, recorded in moderation/Ops logs, and may block only the derived session for the configured TTL. Real upstream token usage remains billable and is marked as request type `cyber`.

## Important mechanisms

- Moderation scope supports all/selected groups, include/exclude model filters, category thresholds, keyword-only/API-only/combined strategies, API-key health and freezing, and admin exemption from automatic bans.
- Input summaries are redacted before persistence. Raw request bodies and image bytes are not stored in audit records.
- Content moderation notification uses the fork's existing `EmailService`; the upstream notification-template subsystem is intentionally not imported.
- Settings are exposed through the existing public/admin Settings KV chain and are hidden by default.

## Fork-local invariants

- Never modify stored billing, `actual_cost`, quota deduction, display-token transforms, effective display prices, or real cache-read token counts for moderation display purposes.
- Preserve curated model lists, default-model fallback, Claude-GPT bridge mapping, OpenAI Images permissions/billing, and existing scheduler/failover rules.
- Preflight blocks are free because they happen before billing and forwarding. Upstream cyber blocks are different: charge only real upstream usage and do not retry another account.
- WebSocket moderation applies to the first `response.create` and every later turn.
- New UI copy must remain present in both Chinese and English locales; the route is admin-only and gated by the public risk-control switch.

## Known pitfalls

- The original local table migration is `153_content_moderation.sql`; do not reintroduce upstream migration `135` or `156`. Extensions use a new local migration number.
- Wire generation may fail because the locally installed Wire tool lacks its own module dependency. When manually reconciling `wire_gen.go`, compile `./cmd/server` to verify the graph.
- Do not treat cyber hard blocks as generic upstream failures: generic failover would repeat prohibited input on another account.
