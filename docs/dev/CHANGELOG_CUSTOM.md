# Sub2API 浜屽紑鍙樻洿鏃ュ織

> 璁板綍鎵€鏈夌浉瀵逛簬涓婃父 (Wei-Shaw/sub2api) 鐨勮嚜瀹氫箟淇敼銆傛瘡娆′簩寮€鍙樻洿蹇呴』鍦ㄦ璁板綍锛屼究浜庡悎骞朵笂娓告洿鏂版椂杩借釜宸紓銆?

## [2026-07-11] feat: Add linked OpenAI Spark shadow accounts

**Affected files**: account Ent schema/generated code, migrations `188`/`189`,
account admin handler/repository/services, OpenAI scheduler/token/header/quota/
rate-limit/WebSocket paths, account export, admin frontend, i18n, and tests.

**Compatibility**: Medium risk, constrained to explicitly created shadows.
Ordinary accounts and fork-local billing/display/cache-read, curated models,
Claude-GPT bridge, Images, fallback, failover, platform quotas, Ops, settings,
and unrelated routes retain their contracts.

**Details**:
- Added one-parent/one-shadow persistence and admin creation. Shadows inherit
  parent groups/proxy and resolve parent OAuth/FedRAMP credentials at request
  time without copying tokens.
- Separated Spark model eligibility, cooldowns, 429 handling, quota query, and
  `codex_*` snapshots while failing closed on invalid parent credentials.
- Guarded refresh/privacy/test/reset, credentials, CRS, proxy/type changes,
  deletion, import/export, and frontend actions against detached shadows.
- Added focused backend and frontend regression coverage. No push/deploy.

## 鏍煎紡璇存槑

```
## [鏃ユ湡] 绫诲埆: 绠€鐭弿杩?

**褰卞搷鑼冨洿**: 娑夊強鐨勬ā鍧?鏂囦欢
**涓婃父鍏煎鎬?*: 鏄惁鍙兘涓庝笂娓告洿鏂板啿绐?
**鍙樻洿璇︽儏**:
- 鍏蜂綋淇敼鍐呭

**鍏宠仈 Issue/PR**: #xxx锛堝鏈夛級
```

---

## 鍙樻洿璁板綍

## [2026-07-11] feat: Add persisted API-key table column settings

**Affected files**: user API-key table, bilingual locale keys, and focused frontend contract tests.
**Upstream compatibility**: Adapts `b244f850e` and its latest-IP column migration to the fork's current Key page and shared icon system.
**Details**:
- Keeps name/actions fixed, lets users toggle all other columns, and persists a validated hidden-column list in browser local storage.
- Hides rate-limit, last-used time, and last-used IP by default. Versioned preference migration hides the newly introduced IP column for existing users without resetting their other choices.
- Malformed/stale preferences fall back safely; no backend setting, API-key permission, quota/billing value, group display data, or route changes.

## [2026-07-11] feat: Show each API key's latest usage IP

**Affected files**: API-key repository/service/DTO, user key types/table/i18n, and focused backend/frontend tests.
**Upstream compatibility**: Behavior-level port of `e0d149d51` plus the query resource fix `7a11b39d6`.
**Details**:
- Enriches one user-owned API-key page with a single batched window query over usage logs, choosing the latest non-empty IP by `created_at` then log ID.
- Supports PostgreSQL and the SQLite repository test dialect, and propagates query scan, iteration, and close errors instead of returning partial data as success.
- The value is response-only: it is not persisted on API keys or added to auth caches, and it does not change usage-log writes, billing, quota deduction, Ops attribution, or public key-usage routes.

## [2026-07-11] feat: Support drag-and-drop multi-file account imports

**Affected files**: account data import modal and its frontend integration test.
**Upstream compatibility**: Low-risk UI adaptation of `728bb1bc9`; the existing backend import contract and fork-local auto-proxy option remain authoritative.
**Details**:
- Accepts multiple selected or dropped JSON exports and merges accounts/proxies in file order before one existing import API call.
- Preserves the first valid export type/version and accumulates `skipped_shadows`; any parse error aborts the whole frontend submission before the API call.
- Does not rewrite, deduplicate, or validate account credentials/models/groups in the browser, and does not change account headers, scheduling, failover, billing/display/cache-read, or migration behavior.

 ## [2026-07-11] feat: Complete subscription peak-rate billing alignment

**Affected files**: group Ent/schema/repository/DTO/auth snapshots, normal and
OpenAI gateway billing, available-channel/payment/subscription APIs, admin/user
frontend, public timezone settings, migration `190`, and focused tests/docs.
**Upstream compatibility**: Adapted `915c60b15`, `1034f576d`, and `11a3da65c`
onto the fork-local billing, media, Batch Image, payment bundle, and settings
contracts instead of replacing shared files wholesale.
**Details**:
- Adds subscription-only same-day peak windows, permits a `0x` peak factor,
  clears peak state when switching to standard, and labels windows with server
  timezone metadata from the existing public-settings injection path.
- Applies peak only to token billing after normal user/group rate resolution;
  image-output tokens follow token billing, while per-image, Grok video/media,
  and Batch Image settlement remain independent.
 - Keeps actual cache-read quantities and display-billing explainability intact;
   API-key snapshots carry the full peak configuration to cached request paths.

## [2026-07-11] fix: Sanitize public branding URLs and HTML-escape site settings

**Affected files**: public-settings URL consumers in shared layout/auth/home views, email HTML builders, embedded page title injection, and focused backend/frontend tests.
**Upstream compatibility**: Selective adaptation of `bfb827b87` and `15c59be78` to the fork's monolithic locales and current page layout. Existing locale keys were retained rather than duplicated.
**Details**:
- Routes every current `doc_url` consumer through the existing HTTP(S)-only URL sanitizer and every current `site_logo` consumer through the existing relative/data-image-aware sanitizer.
- HTML-escapes configured site names in verification, password reset, SMTP test email, and the embedded browser title; password reset links are escaped before entering HTML attributes and fallback text.
- Does not change Settings KV persistence, public-setting DTOs, authentication routes, billing/display/cache-read behavior, model lists/defaults, Claude-GPT bridge, Images, scheduler/failover, or Ops behavior.

## [2026-07-11] fix: Harden scheduler outbox deduplication and cleanup

**Affected files**: scheduler outbox repository/interface/service, account outbox payload construction, migration runner, migrations `186/187`, and focused unit/integration tests.
**Upstream compatibility**: Behavior-level port of the outbox chain from `34e66ec0a` through `f069c9ae0`; upstream migration numbers were reassigned to the fork's next free sequence.
**Details**:
- Replaces timing-window deduplication with a stable partial unique key, releases that key when events are claimed, and repairs invalid concurrent indexes before migration retry.
- Cleans consumed rows only after the watermark is committed, under a PostgreSQL advisory lock and with a ten-second grace period for sequence-allocation/commit races.
- Normalizes typed-nil group payloads so logically identical events share the same key. Candidate eligibility, Grok buckets, advanced scheduler weights/sticky behavior, bridge/Images capability metadata, billing, settings, and frontend contracts are unchanged.

## [2026-07-11] feat: Add guarded API-key account header overrides

**Affected files**: account header policy/service, Anthropic and OpenAI API-key forwarding/probes/models/WS/Images paths, account create/edit/bulk UI, bilingual locales, and focused tests.
**Upstream compatibility**: Selective integration of `ec7b20649` plus audit fixes from `31b6e0d94`; adjacent Spark-shadow and later beta-refactor prerequisites were not imported.
**Details**:
- Allows explicitly enabled Anthropic/OpenAI API-key accounts to override a validated set of outbound headers across real forwarding and account probes.
- Blocks authentication, cookies, content framing, connection, WebSocket handshake, and per-request session headers; applies case-insensitive replacement without duplicate wire forms.
- Uses an overridden `anthropic-beta` value for matching body capability sanitization before existing CCH signing, while OAuth/PAT, Grok, Gemini, Antigravity, Bedrock, FedRAMP identity, bridge/Images gates, billing/display/cache-read, model mapping, and scheduling remain unchanged.
## [2026-07-11] fix: Strip Codex image namespace declarations safely

**Affected files**: image-generation intent helpers, Codex request transforms,
Spark HTTP/WS raw-payload stripping, focused tests, and gateway/upstream-sync
documentation.
**Upstream compatibility**: Selective TDD port of `d3a1835ed`. Upstream tool
bridge `e316ebf52`, Ops capture fix `151b9265f`, and compact recovery
`c67c1ff7e` were audited as already present or equivalently enhanced locally.
**Details**:
- Recognizes the exact `image_gen` namespace in top-level tools, Responses Lite
  `additional_tools`, and namespace-shaped `tool_choice` values.
- Extends the existing Spark strip across those locations and drops empty
  additional-tool carriers, while preserving custom `imagegen` tools,
  `tool_search`, and all non-image namespaces.
- Does not import the absent account explicit-tool-policy control plane or
  replace the fork's 0.1.151 tool bridge and Claude compact recovery code.
- Preserves Claude-GPT bridge eligibility, native/basic OpenAI Images, Batch
  Image settlement, stored billing, display/cache-read transforms, default
  model fallback, scheduler/failover, and Ops attribution.

## [2026-07-11] fix: Prevent billed usage-log loss under queue pressure

**Affected files**: usage-record defaults, usage-log repository batching, gateway usage-log fallback, and focused tests.
**Upstream compatibility**: Selective reliability port of `a1b2b32e0`; the later API-key LastUsedIP rows-close fix `7a11b39d6` is not applicable because that query feature is absent from this baseline.
**Details**:
- Makes synchronous overflow handling the default and applies request-context backpressure instead of silently dropping a full batch queue.
- Falls back to synchronous persistence when best-effort creation reports any failure, using a detached bounded context if the original billing context has already expired.
- Successful async writes are never duplicated; billing failures still skip usage-log creation. Stored billing, display transformations, real cache-read tokens, Batch Image, and Grok media settlement are unchanged.
## [2026-07-11] fix: Align Google gateway authentication and frontend session reliability

**Affected files**: Google API-key middleware and tests; Anthropic token refresher and gateway forwarder; frontend API client, auth store, router, payment polling views, focused tests; account/gateway/sync documentation.
**Upstream compatibility**: Behavior-level reconciliation of `29a5fcd25` and the setup-token refresh portion of `99da30819`; shared fork-local gateway, scheduler, frontend routes, and stores were extended rather than replaced.
**Details**:
- Enforces IP ACLs, exclusive-group authorization, explicit expiry, and quota limits on the Google-compatible API-key middleware, including simple-mode authorization parity.
- Allows Anthropic setup-token accounts through the background refresher while retaining `NeedsRefresh` as the expiry gate; the current `ListActive` refresh architecture already includes setup-token accounts.
- Makes the Anthropic forwarder tolerate a nil Gin context in optional metadata/tool-rewrite paths.
- Bounds token refresh requests, clears local auth after logout API failure, loads public settings before payment/risk-control guards, and prevents overlapping payment polls. Stripe popup initialization now clears its fallback timeout and reads `auth_token`.
- Preserves PAT static-token behavior, OpenAI/Grok isolation, Claude-GPT bridge, Images gates, curated/default models, scheduling/failover, Ops/settings, routes/i18n, stored billing, `actual_cost`, display-token transforms, and real cache-read quantities.
- No schema, migration, push, or deployment.

## [2026-07-11] fix: Preserve credentials and usage on gateway edge paths

**Affected files**: `backend/internal/service/gateway_service.go`, focused scheduler-snapshot and streaming regression tests, and upstream-sync documentation.
**Upstream compatibility**: Narrow reliability alignment from upstream `29a5fcd25`; selection eligibility, billing formulas, and response transforms are unchanged.
**Details**:
- Hydrates the model-routing sticky wait-plan account before returning it, so compact scheduler snapshots cannot reach forwarding without the full credential record.
- Continues processing the current and subsequent upstream SSE events after a client write failure, preserving input, output, and real cache-read usage for billing and Ops records.
- Preserves sticky bindings, wait limits, account capability checks, stored billing, `actual_cost`, display-token transforms, and cache-read token quantities.

## [2026-07-11] fix: Align Go and AWS security baselines

**Affected files**: `backend/go.mod`, `backend/go.sum`, root/backend/deploy Dockerfiles, backend/release/security workflows, and upstream-sync documentation.
**Upstream compatibility**: Exact security alignment of upstream `a4f942d8a` and `25a716960`; no runtime product contract or fork-local business logic changed.
**Details**:
- Upgraded the Go module, build images, and CI version checks to Go 1.26.5 to include the upstream standard-library TLS security fix.
- Upgraded AWS SDK core/eventstream/S3 and their coupled internal modules to the target versions that fix the EventStream decoder denial-of-service advisory.
- Reconciled the older fork-local `backend/Dockerfile` and `deploy/Dockerfile` version pins with the root production build without changing the GHCR deployment workflow.
- Preserved Batch Image settlement/provider behavior, billing/display/cache invariants, bridge/Images/Grok routing, scheduling, settings, migrations, and frontend contracts.

## [2026-07-11] fix: Expose real image-output token breakdown in usage views

**Affected files**: usage DTO mapper/contracts, frontend usage types/helpers, admin/user usage tables, and bilingual labels.
**Upstream compatibility**: Low-risk display-only alignment of `ef5ad0fb1`; stored billing, quota deduction, `actual_cost`, display-token rewrites, and cache-read quantities are unchanged.
**Details**:
- Exposed the already persisted `image_output_tokens` value through user/admin usage DTOs.
- Split displayed output into text-output and image-output quantities without deriving a unit price from cost or rewriting either stored token count.
- Added helper regression coverage for mixed and defensive token breakdowns.

## [2026-07-11] feat: Add secure OpenAI Codex PAT account authentication

**Affected files**: OpenAI account/OAuth/token services, ChatGPT request headers,
admin PAT handler/route, refresh/sync paths, HTTP/WS/Images probes, account UI,
bilingual locales, tests, and account/sync documentation.
**Upstream compatibility**: Manual contract-first port of `32df33a1c` from
alignment baseline `19bd42ca5`; fork-local hot paths were reconciled, not replaced.
**Details**:
- Added Codex `at-` validation, PAT credential mode, explicit revalidation,
  stale OAuth-field cleanup, background-refresh exclusion, and FedRAMP headers.
- Added secure account creation whose response omits credentials/raw PAT values;
  extras retain only a SHA-256 fingerprint and validation errors do not echo
  upstream bodies.
- Preserved API-key auth-cache names, platform isolation, bridge and Images
  controls, billing/display/cache invariants, Ops, curated models, and scheduling.
- No migration, push, or deployment.

## [2026-07-11] feat: Add guarded OpenAI quota query and reset controls

**Affected files**: OpenAI quota service/token provider/account helpers, admin OAuth handler/routes/Wire, account API/usage component, bilingual i18n, focused tests, and account documentation
**Upstream compatibility**: Manual port of `b81694929` plus the confirmation and credit-expiration follow-ups from `30adee43b` and `dfb36e45f`; shared account, Wire, locale, and usage files were reconciled instead of replaced.
**Details**:
- Added admin-only OpenAI OAuth quota query and reset-credit consumption through the account usage cell, including sanitized credit expiration details.
- Required explicit reset confirmation and a validated UUID-v4 `redeem_request_id`; the frontend keeps one stable ID across retry of the same action and the backend forwards it unchanged.
- Reused final Codex identity pairing so upstream quota requests always carry a matched account/fallback User-Agent and originator.
- Added minimal PAT token-provider compatibility: `personalAccessToken`, `personal_access_token`, and `codex_pat` OAuth-shaped accounts use the stored access token without entering refresh locking.
- Preserved the independent Grok quota probe, OpenAI/Grok platform isolation, account scheduling/cooldowns, user-platform quota, public/admin settings, i18n/routes outside the added endpoints, billing/display-token/cache-read invariants, bridge, Images, and curated/default model behavior.
- Explicitly excluded the later Spark linked-shadow-account schema, scheduling, and usage feature from this batch.
## [2026-07-11] feat: Add the OpenAI advanced scheduler control plane

**Affected files**: OpenAI scheduler/config and scheduler snapshot metadata; Settings KV/service/admin DTOs and handler; admin account score response/repository query; Settings and Accounts Vue views, types, API contracts, bilingual i18n, focused tests; deployment, gateway, sync, and changelog documentation
**Upstream compatibility**: Manual behavior-level port from `f26ca5661` and audit `0fd2e9216` on baseline `19bd42ca5`; fork-local gateway and WS hot paths were preserved instead of replaced.
**Details**:
- Added total-gated sticky-weighted and paid-subscription-priority controls, DB TopK and nine scheduler weight overrides, effective-value reporting, audit field tracking, and bilingual admin controls.
- Added TTFT/error/concurrency-full sticky escape with explicit config defaults; escaped requests keep the original sticky binding and still use the fork's filtered candidate pool.
- Added base and per-group scheduler score observability to the admin account list using a single union load batch and effective scheduler weights.
- Kept non-secret OpenAI OAuth `plan_type` in scheduler snapshots while continuing to strip access and refresh tokens.
- Preserved RequiredCapability/Images, Claude-GPT bridge eligibility, WS v2 transport selection, OpenAI/Grok isolation, group/model/compact/runtime filtering, local account blocking, and previous-response mobility rules.
- Did not change billing, platform quota deduction, display-price/token transforms, cache-read token quantities, `actual_cost`, curated models, default fallback, Ops behavior, migrations, routes, or public settings.
- Verified affected backend packages, explicit scheduler protection tests, frontend typecheck/lint/build and focused tests, upstream-sync guard, and diff checks.

## [2026-07-11] feat: Add compatible Codex engine fingerprint controls

**Affected files**: OpenAI identity/fingerprint package, codex-only detector and
gateway entries, Settings/admin API, OpenAI OAuth account UI, bilingual locales,
tests, and account/sync docs.
**Upstream compatibility**: Manual TDD port of `819fda34d` and `4b321142b` from
integrated baseline `7bf5fd15c`, reconciled with PAT and fork-local UI/settings.
**Details**:
- Added deny-first blacklists, strict allowlists, optional engine versions,
  app-server controls, structured signals, and actionable version messages.
- Preserved legacy behavior while policy is unconfigured. Version/fingerprint
  gates activate only after explicit admin configuration; presets are UI-only.
- No migration. API-key cache/name, PAT, billing/display/cache-read,
  curated/default models, bridge, Images, Grok, quota, scheduler and Ops remain.

## [2026-07-11] fix: Reconcile merged public contracts and auth-cache identity

**Affected files**: `backend/internal/service/{setting_service.go,api_key_auth_cache_impl.go}`, `backend/internal/server/api_contract_test.go`
**Upstream compatibility**: Merge-integration correction only; no upstream subsystem was replaced.
**Details**:
- Added `risk_control_enabled` to the HTML-injected public settings payload so first paint and fetched public settings expose the same feature flags.
- Updated public group and usage API contract snapshots for the new Grok video pricing and usage metadata fields.
- Preserved the API key display name across auth-cache snapshot round trips; the JSON field already existed, so old cache entries remain backward compatible.
- Verified the public-settings schema guard, server API contracts, and API-key cache round-trip tests.

## [2026-07-11] fix: Harden Codex WebSocket scheduling and add quota-headroom scoring

**Affected files**: OpenAI account scheduler/config, Responses WebSocket handler, tool-continuation analysis, WebSocket disconnect classification, focused tests, deployment example, and gateway/sync documentation
**Upstream compatibility**: Selective port from `0fd2e9216`, `a2cf297d9`, and `0a5f34a2`; the fork's existing scheduler, platform isolation, bridge eligibility, Images capability gates, and billing paths were extended instead of replaced.
**Details**:
- Made `previousResponseCanMove` an explicit scheduler input and only allows cross-account migration when every tool-output `call_id` is reconstructable from in-band call context or `item_reference` data.
- Added opt-in `quota_headroom` scheduler weight backed by existing Codex quota snapshots. The default is zero, stale/missing snapshots are neutral, and near-exhausted short windows are penalized.
- Treats Windows `wsarecv: ... forcibly closed by the remote host` errors as normal client disconnects in both ingress and passthrough relay paths.
- Preserves reasoning-effort usage metadata across mapped/upstream/original model candidates, including GPT-5.6 `max` and suffix-derived effort after OAuth model normalization; passthrough WebSocket turns track the current value alongside service tier.
- Preserved Grok/OpenAI platform isolation, Claude-GPT bridge-only eligibility, OpenAI Images native/basic fallback, platform quota accounting, Ops context, stored billing, display-token transforms, and cache-token invariants.
- Audited but did not fold the independent OpenAI PAT authentication (`32df33a1c`) or Codex engine-fingerprint control plane (`819fda34d`, `4b321142b`) into this scheduler/WS batch; both require separate API/settings/frontend reconciliation.
- Audited OpenAI quota query/reset readiness (`b81694929`) and later reset-credit UI updates; this remains a separate admin/Wire/frontend batch, while the scheduler-facing headroom factor is complete here.

## [2026-07-11] feat: Complete Grok image and video gateway billing loop

**Affected files**: Grok media handler/routes, group and usage Ent schemas/generated code, group/auth-cache/repository mappings, media billing and usage persistence, migration `181_grok_media_billing.sql`, focused tests and gateway documentation
**Upstream compatibility**: High-risk selective port of the final Grok media behavior through target `e316ebf5`; fork-local gateway and billing implementations were extended instead of replaced.
**Details**:
- Exposed Grok-only image generation/edit and video generation/status endpoints with platform-isolated scheduling, sticky video status routing, bounded failover, and usage recording.
- Reused content moderation before concurrency, billing eligibility, scheduling, and forwarding, so locally blocked Grok media requests do not deduct balance or platform quota.
- Added group-level independent video rate and 480p/720p/1080p per-second prices, official Grok default image/video rate cards, and persisted video count, resolution, and duration metadata.
- Added additive migration `181`; historical migrations were not edited. Existing Grok groups are media-enabled and newly created Grok groups default to media-enabled.
- Preserved stored billing/display-token separation, real cache-read token quantities, `actual_cost` semantics, user/channel/global price resolution for token requests, Claude-GPT bridge routing, curated model lists, OpenAI Images account controls, default-model fallback, Ops capture, and platform quota accounting.
- Verified Ent generation, media/service/repository/handler/routes tests, upstream-sync guard, and diff checks.

## [2026-07-11] feat: Grok/xAI OAuth and OpenAI-compatible gateway foundation

**Affected files**: `backend/internal/{pkg/xai,repository/grok_oauth_client.go,service/{grok_*,openai_gateway_grok.go,openai_account_scheduler.go,account.go},handler/admin/grok_oauth_handler.go,server/routes/{admin,gateway}.go,cmd/server/wire_gen.go}` plus focused tests and `frontend/src/{api/admin/grok.ts,composables/useGrokOAuth.ts}`
**Upstream compatibility**: High-risk hot-path adaptation. Grok support was ported manually from the upstream alignment target instead of replacing the fork's OpenAI gateway, scheduler, billing, or route files.
**Details**:
- Added xAI OAuth exchange/refresh, token provider, quota probing, quota snapshot persistence, and admin OAuth endpoints.
- Added Grok Responses, Chat Completions, and Anthropic Messages conversion/forwarding through the existing OpenAI-compatible gateway service.
- Platformized OpenAI-compatible scheduling so Grok requests only select Grok accounts and ordinary OpenAI requests cannot select Grok accounts; runtime-blocked Grok accounts are excluded from both legacy and advanced scheduler paths.
- Preserved the fork-local Claude-GPT bridge eligibility contract, curated OpenAI model discovery, OpenAI Images feature gate, default-model fallback, usage/display accounting fields, and Ops response-commit tracking.
- At this core checkpoint Grok `count_tokens` and WebSocket Responses were explicitly unsupported and media HTTP exposure was deferred. The later Grok media billing entry supersedes the media portion; the target upstream still has no independent Grok WebSocket implementation.
- Added focused regression coverage for OAuth, quota, protocol conversion, platform-isolated scheduling, runtime blocking, admin routes, and DI construction.

## [2026-07-11] feat: Integrate upstream risk control without replacing fork-local gateway behavior

**Affected files**: backend moderation repository/service/admin API and protocol gateway integrations, Settings KV, Ops/cyber usage paths, migration `182_content_moderation_extensions.sql`; frontend risk-control view/API/router/sidebar/settings/i18n; `docs/dev/codebase/risk-control.md`
**Upstream compatibility**: Medium-high risk, manually reconciled. Upstream commits `fff4a300c`, `0eca600ff`, `91da81599`, `0d5c6f7cc`, `23f3d426c`, `1b2d8873b`, `c40a74d98`, `b62b573f7`, and `815bc6c9b` were staged in sequence and then adapted to the fork.
**Details**:
- Added admin-managed moderation config, logs, keyword/hash blocking, group/model scopes, thresholds, API-key health, retention, notification, and auto-ban controls.
- Added preflight moderation to Anthropic Messages, OpenAI Responses/Chat/WebSocket/Images, and Gemini before billing, concurrency, scheduling, and forwarding, so locally blocked requests do not deduct quota.
- Added upstream `cyber_policy` passthrough, audit/Ops recording, request type `cyber`, and optional session-only Redis blocking without account failover.
- Preserved fork-local display billing/cache-token invariants, curated model lists, Claude-GPT bridge, OpenAI image generation controls, default-model fallback, scheduler/failover, Ops settings, and existing `EmailService`.
- Reused existing local migration `153` for the base table and assigned new extension migration `182`; upstream migration numbers `135` and `156` were removed to avoid history collisions.

## [2026-07-11] fix: Harden released Ops capture writers

**Affected files**: `backend/internal/handler/{ops_error_logger.go,ops_capture_writer_nil_test.go}`, `docs/dev/{UPSTREAM_SYNC.md,codebase/ops.md,CHANGELOG_CUSTOM.md`
**Upstream compatibility**: Low risk. Manual narrow port of upstream commits `89a551b96` and `bc3cb2902`; local Ops middleware and logging behavior remain in place.
**Details**:
- Added explicit guards for every `gin.ResponseWriter` method delegated by `opsCaptureWriter` so late access after pool release cannot dereference a nil embedded writer.
- Preserved direct delegation while the writer is acquired, including error-body capture, headers, flushing, hijacking, close notification, HTTP/2 push, status, size, and written state.
- Added regression coverage for the complete released-writer interface and retained the existing pool reset coverage.
- No frontend, API route, schema, migration, setting, billing, model discovery, Claude-GPT bridge, OpenAI Images, or scheduling behavior changed.

## [2026-07-11] feat: Codex models manifest passthrough

**Affected files**: backend/internal/{handler/openai_codex_models_handler.go,service/openai_codex_models_service.go,server/routes/gateway.go}(+tests), docs/dev/{UPSTREAM_SYNC.md,codebase/gateway.md}
**Upstream compatibility**: Medium-low risk. Manual narrow port of upstream PR `Wei-Shaw/sub2api#3800` / merge commit `b6d2df24`; no broad upstream merge and no replacement of fork-local curated model discovery.
**Details**:
- OpenAI-group `GET /v1/models?client_version=...` now returns the live ChatGPT Codex models manifest expected by Codex desktop custom providers; plain `/v1/models` keeps the existing curated OpenAI list.
- Added the authenticated native compatibility path `GET /backend-api/codex/models`.
- Manifest requests select schedulable OpenAI OAuth accounts only, preserving group priority/LRU eligibility while skipping API-key accounts in mixed groups.
- Upstream requests forward Codex client/account headers, `client_version`, `If-None-Match`, and account proxy configuration; downstream responses preserve JSON, ETag, and 304 semantics.
- Added an 8 MiB response bound that rejects oversized manifests rather than returning truncated JSON.
- Verified the manifest service, account selection, route registration/dispatch, full handler/routes/httpclient packages, full service package, and a CGO-disabled server build. Full repository unit tests have one unrelated existing API-contract snapshot mismatch for the concurrently added `gateway_network_retry_max` setting.

**Related upstream PR**: `Wei-Shaw/sub2api#3800`

## [2026-07-10] feat: OpenAI GPT-5.6 sol/terra/luna support

**影响范围**: backend/internal/{pkg/openai/constants.go, service/{openai_model_alias.go,openai_codex_transform.go,models_list_policy.go,pricing_service.go,billing_service.go}(+tests), handler/gateway_models_list_test.go}, backend/resources/model-pricing/model_prices_and_context_window.json, frontend/src/{composables/useModelWhitelist.ts(+test),components/keys/UseKeyModal.vue(+test)}, docs/dev/codebase/{model-mapping.md,billing.md}
**上游兼容性**: 中低风险。按上游 `6cea1c35` 增量接入 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`，但不做大范围 upstream merge，不移除本地 GPT-5.5-pro/date、Codex Spark、Claude-GPT bridge、图片通道、展示倍率等二开逻辑。
**变更详情**:
- OpenAI 默认模型、`/v1/models` curated discovery、前端 OpenAI 白名单/预设、OpenCode 配置加入 GPT-5.6 三个官方变体。
- Codex/OpenAI 模型归一支持 `gpt5.6-*`、`openai/gpt5.6-*`、reasoning-effort 后缀、日期后缀和 compact 后缀，避免新模型落入旧的 `gpt-5 -> gpt-5.4` 兼容兜底。
- LiteLLM 资源文件加入上游 GPT-5.6 pricing/context/service-tier 字段；动态价格仍优先，静态兜底仅在价格资源缺失时启用，且不改变用户/渠道/全局/display rate 解析链。
- 默认 Claude-GPT bridge 模板保持 `claude-opus-4-8/4-7 -> gpt-5.5`、其他 Claude 4.x -> `gpt-5.4`，只新增可选 OpenAI 预设，不隐式升级默认桥接目标。
- 验证：`go test -tags=unit ./internal/pkg/openai ./internal/service ./internal/handler` 通过；`node -e "JSON.parse(...model_prices_and_context_window.json...)"` 通过；`pnpm test:run src/composables/__tests__/useModelWhitelist.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts` 通过；`pnpm exec eslint src/composables/useModelWhitelist.ts src/composables/__tests__/useModelWhitelist.spec.ts src/components/keys/UseKeyModal.vue src/components/keys/__tests__/UseKeyModal.spec.ts` 通过。

## [2026-07-08] feat: 网关上游网络错误可配置重试

**影响范围**: backend/internal/{repository/http_upstream.go(+test), service/{http_upstream_port.go,setting_service.go,settings_view.go,domain_constants.go,wire.go,setting_service_update_test.go}, handler/{admin/setting_handler.go,dto/settings.go}, cmd/server/wire_gen.go}, frontend/src/{api/admin/settings.ts,views/admin/SettingsView.vue,i18n/locales/{zh,en}.ts}, docs/dev/codebase/gateway.md
**上游兼容性**: 中低风险。统一 HTTPUpstream 出站层新增传输错误兜底；仅对未收到 HTTP 响应的连接失败/超时/EOF/DNS 等网络错误生效，不重试上游 4xx/5xx 响应；不可重放 request body 不重试。
**变更详情**:
- 新增系统设置 `gateway_network_retry_max`，位于后台「系统设置 - 网关服务 - 请求转发行为」，取值 0-10，默认 2；0 表示关闭重试。
- `repository.HTTPUpstream` 外层增加网络错误重试：默认最多重试 2 次（总尝试 3 次），短退避；触发时写 `upstream_network_retry` 日志；已有专用重试循环的 OpenAI OAuth 图片 `/responses` 工具路径通过上下文关闭全局重试，避免次数叠加。
- 设置服务将该字段并入网关转发行为缓存，保存后刷新热路径缓存；admin settings API 支持未传字段沿用旧值并记录审计 diff。
- 前端补齐类型、默认值、保存 payload 和中英文文案。
- 验证：`go test -tags=unit ./internal/repository ./internal/service ./internal/handler/admin ./cmd/server` 通过；`pnpm run typecheck` 通过。

## [2026-07-08] fix: 图片渠道监控手动参数区增加内部下拉滚动

**影响范围**: frontend/src/views/admin/ImageChannelMonitorView.vue, docs/dev/codebase/image-channel-monitor.md
**上游兼容性**: 低风险。仅调整手动检测面板左侧参数配置区域的布局滚动边界，不改接口、检测逻辑或持久化结构。
**变更详情**:
- 手动检测左侧参数配置块改为固定标题 + 有高度边界的内部滚动正文，内容过高时可向下滚到预设/模板选择区域。
- 保持手动面板的固定视口设计：不恢复整页滚动，Channels 列表和底部开始/取消 CTA 仍按原内部滚动布局工作。
- 更新 image-channel-monitor 模块文档，记录参数正文也是左侧内部滚动区域之一，后续新增控件不能再次隐藏底部控制项。

## [2026-07-07] feat: 图片渠道监控手动检测支持并发批次

**影响范围**: backend/internal/{service/{image_channel_monitor_types.go,image_channel_monitor_service.go(+test)},handler/admin/image_channel_monitor_handler.go}, frontend/src/{api/admin/imageChannelMonitor.ts,views/admin/ImageChannelMonitorView.vue,i18n/locales/{zh,en}.ts}, docs/dev/codebase/image-channel-monitor.md
**上游兼容性**: 低风险。手动检测仍是异步内存 run + 前端本地历史，不改 `image_channel_monitor_histories` 定时监控历史表，也不改变 scheduled check 语义。
**变更详情**:
- 手动检测参数区新增并发数，点击开始后按 `选中渠道数 × 并发数` 展开独立检测记录；前端限制单渠道并发 1-20、单轮总记录 100 条，避免误操作压垮浏览器或上游。
- 后端 manual run 请求/响应新增 `batch_id`、`batch_size`、`batch_index`，轮询与取消响应保持同一批次标识；`StartManualCheck` 单测覆盖批次字段保留。
- 前端 `manualResults` 从按渠道 ID 存储改为按单条 recordId 存储，同一渠道可同时显示多条并发记录；手动记录表新增「批次」列，详情弹窗新增批次/序号/平均耗时指标。
- 浏览器本地手动历史保存批次字段与 `batch_average_elapsed_ms`；同批记录完成时回填平均耗时，旧历史/预设数据兼容默认值；手动预设同步保存并发数。
- 验证：`pnpm --dir frontend run typecheck` 通过；`go test -tags=unit ./internal/service -run TestImageChannelMonitorStartManualCheckRunsAsyncAndPollsResult` 通过。

## [2026-07-06] feat: 图片渠道监控/手动测试支持 response_format 拿图方式选择

**影响范围**: backend/{migrations/179, ent/schema/{image_channel_monitor,image_channel_monitor_history}.go(+regen), internal/service/{image_channel_monitor_types.go, image_channel_monitor_service.go(+test)}, internal/repository/image_channel_monitor_repo.go, internal/handler/admin/image_channel_monitor_handler.go}, frontend/src/{api/admin/imageChannelMonitor.ts, views/admin/ImageChannelMonitorView.vue, i18n/locales/{zh,en}.ts}
**上游兼容性**: 低风险。新增迁移 179（monitors/histories 各加 response_format 列,存量回填 'url' 与旧强制行为一致）;imageMonitorMaxResponseBytes 2MB→24MB(容纳 b64 内联大图);配合 8611221ba(网关透传显式 response_format)。
**变更详情**:
- 渠道监控与手动测试均可选拿图方式:URL / Base64 / 不传(跟随上游默认),对应 payload 带 response_format=url / b64_json / 省略参数;JSON 与 multipart(图生图 edits)两条路径同步。
- 语义:仅 url 模式下 b64 返回视为交付失败(维持旧监控语义);b64_json/不传模式接受 b64 返回为正常,内联图片元数据(尺寸/大小)照常解析。
- 历史记录:每次检查的拿图方式写入 histories 并在定时历史弹窗新增「拿图方式」列;手动检测记录详情弹窗新增同名指标;手动预置(preset)与本地历史同步保存该字段,旧数据回落 url。
- 新建渠道/手动测试表单默认 url(行为不变),需要测 base64 或跟随上游时显式切换。
- 验证:后端新增三态 payload/调度接受性单测,全量 unit 通过;前端 typecheck/lint/相关 vitest 通过;浏览器实测编辑表单回填(库改 b64_json 后正确显示)、手动面板选项、历史列渲染,无控制台报错。

## [2026-07-06] feat: 图片渠道监控状态时间线 + 用户侧公开展示

**影响范围**: backend/{migrations/178, ent/schema/image_channel_monitor.go(+regen), internal/service/{image_channel_monitor_types.go, image_channel_monitor_service.go(+test), ops_cleanup_service.go, wire.go}, internal/repository/image_channel_monitor_repo.go, internal/handler/{image_channel_monitor_user_handler.go(新+test), handler.go, wire.go, admin/image_channel_monitor_handler.go}, internal/server/routes/{admin.go, user.go}, cmd/server/wire_gen.go(手工对齐)}, frontend/src/{api/{admin/imageChannelMonitor.ts, imageChannelMonitor.ts(新)}, components/{admin/ImageMonitorStatusDialog.vue(新), user/monitor/{ImageMonitorCard.vue(新), ImageMonitorDetailDialog.vue(新), __tests__/ImageMonitorCard.spec.ts(新)}}, views/{admin/ImageChannelMonitorView.vue, user/ChannelStatusView.vue}, i18n/locales/{zh,en}.ts}
**上游兼容性**: 低风险。新增迁移 178（image_channel_monitors 加 public_visible/public_name 两列）；`NewOpsCleanupService` 签名加 imageChannelMonitorSvc 参数（wire_gen 已手工对齐）；`Handlers` 容器加 ImageChannelMonitorUser；admin List 响应每项追加 timeline/availability_7d 字段（增量，不破坏旧消费方）。设计文档 docs/superpowers/specs/2026-07-06-image-monitor-status-timeline-design.md。
**变更详情**:
- 管理端监控列表每行内嵌迷你状态条（复用用户侧 MonitorTimeline 60 根柱）+ 7 天可用率；新增「状态详情」弹窗：24h/7d/30d 窗口切换 + chart.js 混合图（API 总耗时/图片下载两条折线 + 失败次数红色柱，空桶断线）+ 可用率/次数/失败/平均/最大耗时汇总卡。
- 数据策略：不建 rollup 表，全部对原始历史实时 SQL 聚合（epoch-floor 分桶 24h→10min/7d→2h/30d→1d；批量近 60 次 ROW_NUMBER 消 N+1；三窗口可用率单条 FILTER 聚合）。
- 历史保留：激活 DeleteHistoryBefore 死代码，RunDailyMaintenance 物理删 30 天前明细，挂进 ops 每日清理（同 cron/领导锁），修复历史表无限增长问题。
- 每渠道公开配置：public_visible（默认不公开）+ public_name（掩盖内部命名，空回落渠道名），编辑表单新增「用户侧展示」区块。
- 用户侧 /monitor 渠道状态页新增「生图渠道」分组：卡片（生图耗时/图片下载/窗口可用率/60 根时间线，empty 状态中性徽章）+ 简版详情弹窗（7/15/30d 可用率+平均耗时）；列表一次带回三窗口可用率，窗口切换纯前端；跟随页面 channel_monitor_enabled 门禁与自动刷新。
- 安全红线：用户侧 DTO 白名单（绝不下发内部名/endpoint/host/IP/错误消息/error_stage/图片 URL/代理账号信息），白名单 JSON key 快照测试兜底。
- 验证：后端全量 unit 通过（含 9 个新用例）；前端 typecheck/lint/全量 vitest 620 用例通过（含新卡片 spec）；本地注入 3 天含失败/降级数据浏览器实测：行内条/弹窗三窗口/折线失败柱/用户侧掩名卡片/详情弹窗/响应净化抽查全部正确，验证数据已清理。

## [2026-07-06] feat: 图片渠道监控补全返回图片尺寸/大小信息

**影响范围**: backend/internal/service/image_channel_monitor_service.go(+test), frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**上游兼容性**: 低风险。后端仅在 b64_json 分支补填 history 已有字段（image_bytes/image_content_type/image_width/image_height），不改状态判定与请求行为；前端手动检测表格新增可选列。
**变更详情**:
- 后端：上游返回 b64_json 时（gpt-image-1 常态）原先完全不解析图片元数据，新增 `fillImageMonitorInlineImageInfo` 解码 base64 填充字节数、嗅探 content-type、DecodeConfig 取宽高，定时与手动路径共用；URL+下载路径原有逻辑不变。
- 手动检测记录表新增"返回图片"列（默认显示，可在字段选择器关闭）：显示实际宽高 + 文件大小；当请求 size 为具体 WxH 且与实际不一致时琥珀色加 ⚠ 警示，tooltip 注明请求尺寸（omit/auto 不比对）。
- 查看详情弹窗新增"实际尺寸/图片大小/图片格式"三项指标，不一致时实际尺寸标黄并在指标区下方显示整句提示。
- 定时监控历史弹窗"图片"列由仅宽高改为"宽高 · 大小"。
- 验证：后端新增单测（1x1 PNG b64 断言宽高/字节/content-type），TestImageChannelMonitor* 全过；前端 typecheck/lint 通过；本地浏览器注入一致/不一致/无图三类记录，实测表格列、警示样式、tooltip、详情弹窗渲染均正确，无控制台报错。

## [2026-07-04] feat: 导入 CCS 客户端选择扩展——anthropic 密钥支持 Codex 客户端

**影响范围**: backend/internal/{service/{domain_constants.go, setting_service.go, settings_view.go}, handler/{setting_handler.go, dto/settings.go, admin/setting_handler.go}, server/api_contract_test.go}, frontend/src/{views/user/KeysView.vue, views/admin/SettingsView.vue, api/admin/settings.ts, stores/app.ts, types/index.ts, i18n/locales/{zh,en}.ts}
**上游兼容性**: 低风险。新增 Settings KV `ccs_import_anthropic_codex_model`（镜像 `ccs_import_codex_model` 全链，默认空）；KeysView 导入弹窗逻辑重写为数据驱动。若上游后续也改 CCS 导入需人工比对。
**变更详情**:
- 客户端选择弹窗从"仅 antigravity"扩展到 anthropic + antigravity 平台：anthropic 密钥可选 Claude Code / Codex（Codex 走根路径 `/responses` Responses→Anthropic 桥，deeplink `app=codex`）；antigravity 保持 Claude Code / Gemini CLI（按产品决策不提供 Codex，`/antigravity/*` 下无 /responses 路由）；openai/gemini 平台仍无弹窗直接映射。
- 调研结论（cc-switch v3.16.5 源码）：deeplink `app` 白名单为 claude/codex/gemini/opencode/openclaw/hermes，**不支持 claude-desktop**（UI 有该页签但 parser 拒绝）；Claude Code CLI 与桌面版共用 ~/.claude/settings.json，`app=claude` 一个入口覆盖两者，弹窗文案已注明。
- 新增管理端设置"CCS 导入默认模型（Anthropic 密钥 → Codex 客户端）"：anthropic 密钥选 Codex 导入时写入 deeplink `model` 参数，应填本站可调度的 Claude 模型或已配置渠道映射的模型名；留空则 cc-switch 回落 gpt-5-codex。
- 顺带修复两处存量测试损坏（被 unit-tag 编译错误掩盖）：`NewUsageHandler` 签名漂移致 api_contract_test 编译失败；redeem/history fixture 缺 `batch_redeem_limit_per_user` 字段。
- 验证：go test -tags=unit ./... 全过；前端 typecheck/lint/SettingsView+app spec 全过；本地浏览器 E2E 实测四种平台密钥的弹窗选项与 deeplink 参数（含管理端设置保存→公开设置下发→deeplink model 参数全链）。

## [2026-07-04] feat: 模型配置页所有行可删除——直通行删除=持久化隐藏(可恢复)

**影响范围**: backend/internal/{domain/constants.go, service/{domain_constants.go, setting_service.go, wire.go, global_model_pricing_service.go(+test), setting_service_model_mapping_test.go, model_pricing_resolver.go}, handler/admin/model_pricing_handler.go, server/routes/admin.go}, backend/cmd/server/wire_gen.go(手工对齐), frontend/src/{api/admin/modelPricing.ts, components/admin/model-pricing/ModelPricingTab.vue, i18n/locales/{zh,en}.ts}, docs/dev/codebase/model-mapping.md
**上游兼容性**: 低风险。新增 Settings KV `model_pricing_hidden_models` 与 `GET/PUT /admin/model-pricing/hidden-models`;`NewModelPricingHandler` 增加 settingService 参数(wire_gen 已手工对齐);列表 source 筛选新增特殊值 `hidden`。不改任何计费/调度行为。
**变更详情**:
- 直通行(请求=上游,来自 LiteLLM 目录/覆盖,无映射条目可删)新增"删除"按钮:确认后把模型加入隐藏集合,列表不再显示;仅影响模型配置列表展示,不影响计费与请求转发。
- 来源筛选新增"已隐藏"视图:列出全部隐藏模型(含目录中已不存在的名字,补 stub 保证可恢复),行内"恢复"一键还原。
- 隐藏永不吞掉真实映射:模型自身是有效映射键时(即使被隐藏)映射行保持可见。
- 真实映射条目行为不变(删除映射=从平台默认映射表移除条目)。

## [2026-07-04] fix: 模型配置页映射表彻底重构（角色不再坍缩）+ 测试连接模型列表并入平台映射

**影响范围**: backend/internal/service/global_model_pricing_service.go(+test), backend/internal/handler/admin/account_handler.go(+test), backend/internal/pkg/antigravity/claude_types.go, backend/migrations/177_add_fable5_to_default_model_mapping.sql, frontend/src/components/admin/model-pricing/{ModelPricingTab.vue, ModelMappingInlinePopover.vue, modelPricingRows.ts(新), __tests__/modelPricingRows.spec.ts(新)}, frontend/src/api/admin/modelPricing.ts, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/model-mapping.md
**上游兼容性**: 中风险。`billing_basis_hint` 新增 `mapping_target`/`mapped_from` 字段并新增复数 `billing_basis_hints`，单数字段保留兼容；`GET /admin/accounts/:id/models` 各分支并入平台级默认映射键；迁移 177 为二开自有编号。合并上游时若上游也改了模型配置页需人工比对。
**变更详情**:
- 修复映射角色坍缩：旧实现按 same_name > upstream_only > requested_only 收敛单一角色，模型"既是映射键又是别人的映射目标"时（如 claude-sonnet-4-5 -> claude-fable-5 且 claude-sonnet-4-5-20250929 -> claude-sonnet-4-5）自身映射从列表消失，前端把上游名画回请求名（"添加映射后映射目标被改回原名"的根因）。现在 hint 同时携带 mapping_target 与 mapped_from，且"全部"视图按平台各发一条 hint。
- 前端行推导重写（modelPricingRows.ts）：映射行只由映射键自己的条目产生，不再从目标条目反向展开+去重互踩；纯映射目标模型保留自己的直通行（修复"claude-fable-5 可请求但映射表里没有该请求模型"）；所有直通行提供"添加映射"入口，弹窗预填 from=to=模型名（修复"大量行无法编辑/删除"——真实条目才有删除，直通行编辑即建条目）。
- 保存映射的平台与当前供应商 tab 不同时自动切 tab，避免"添加成功但看不到"。
- 测试连接模型列表（账号管理）：Antigravity 非透传账号改为按生效映射表请求键生成（含 [1m]/[2m] 变体），Claude/Gemini/OpenAI 账号并入对应平台默认映射键（修复"新添加映射的请求模型在测试连接列表看不到"）。
- 迁移 177：为保存过的 antigravity_default_model_mapping 设置及账号级 model_mapping 回填 claude-fable-5 同名映射（保存表整体替换内置表，早于 fable-5 的存量表缺该条导致 Antigravity 漏调度）。

## [2026-07-04] feat: redesign manual image test into a fixed-viewport split console

**Affected files**: frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/components/layout/TablePageLayout.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: frontend-only fork-local layout rework. It does not change backend APIs, schemas, scheduling, manual history storage, browser-local storage keys, or polling/cancel logic. `TablePageLayout` gains an additive `bareTable` prop (default `false`) guarded by `:not(.is-bare)`, so all other consumers are unaffected.
**Change details**:
- Reworked the manual-test panel into a fixed-viewport split console (`bareTable` slot): left column stacks Parameters (collapsible) → Channels (internal scroll) → a persistent Start-test CTA bar; right column is the records table with an internal scroll area and a sticky header. The whole panel fits one viewport — scrolling happens only inside the channel list and the table, never the page.
- Panel switcher moved from two large cards to a compact header + segmented tabs (A), reclaiming ~90px of vertical space.
- Parameters: two-column grid, prompt spans full width, and the separate size-mode + size selects were merged into one dropdown (with a "custom…" entry) backed by a `manualSizeChoice` get/set computed over the unchanged `size_mode`/`size`/`custom_size` trio. Collapsing the card shows a one-line summary of chips.
- Presets condensed to dropdown + save/delete at the card foot; naming moved into a save dialog.
- Channels: row list with selected-count pill, select-all/clear, and a channel search filter (`manualFilteredTargets`); internal scroll keeps the page height bounded regardless of channel count.
- Results: running/completion banner (progress x/y, ok/fail counts, cancel) driven by new `manualBatchStats`/`manualBatchProgress` computeds derived from `manualResults` — zero API changes. Default columns trimmed (mode/model/size hidden by default via the existing field-visibility state); numeric columns right-aligned with `tabular-nums`; compact text actions.
- Detail dialog: added a latency waterfall over the existing `api_header_ms` (start→headers) / `api_body_ms` (headers→body phase) / `image_download_ms` (download phase) — confirmed sequential non-overlapping phase durations in `image_channel_monitor_service.go`, so they stack correctly; dropped the now-redundant raw timing metrics.
- Added the field-popover outside-click-to-close handler.
- New i18n keys (zh/en in sync): config, collapse/expand, selectAll/clearSelection, searchTargets, selectedOfTotal, noTargets, startWithCount, testingProgress, ctaHint, batchRunning/batchComplete, resultOk/resultFail, waterfall, savePresetTitle.
- Verified: `pnpm run typecheck`; `pnpm run lint:check`; Vite dev-server transform of all four changed modules returns HTTP 200 with the new markup and no compile errors. Live authenticated screenshot not captured (no local admin credentials on hand); layout mechanics validated in a standalone prototype using the same flex/overflow approach.

## [2026-07-04] fix: keep manual image channel selection reachable

**Affected files**: frontend/src/views/admin/ImageChannelMonitorView.vue, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: frontend-only fork-local image monitor layout fix. It does not change backend APIs, schemas, scheduling, manual history storage, or monitor behavior.
**Change details**:
- Removed sticky positioning from the left manual-test configuration column so the full page can scroll down to the channel-selection controls.
- Documented the layout pitfall: the left column can exceed viewport height, and sticky positioning makes lower controls unreachable.
- Verified: `pnpm run typecheck`; `git diff --check`; `Invoke-WebRequest http://127.0.0.1:15174/admin/channels/image-monitor`.

## [2026-07-04] feat: reorganize manual image test records

**Affected files**: frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: frontend-only fork-local image monitor UI enhancement. It does not change backend APIs, schemas, scheduling, or browser-local storage keys.
**Change details**:
- Reworked the manual image testing panel into a two-column layout: configuration, prompt, preset, input image, and channel selection stay on the left; manual test records are managed on the right.
- Replaced the separate manual-history entry point with a unified record table that combines in-flight manual runs and browser-local history.
- Added table search, status/mode/channel filters, newest/oldest sorting, field visibility toggles, per-row details, and generated-image download actions.
- Kept manual history storage and IndexedDB image preservation unchanged so existing saved records remain compatible.
- Verified: `pnpm run typecheck`; `pnpm run build`; `git diff --check`; `Invoke-WebRequest http://127.0.0.1:15174/admin/channels/image-monitor`.

## [2026-07-04] feat: record manual image test network metadata

**Affected files**: backend/internal/service/image_channel_monitor_*.go, backend/internal/handler/admin/image_channel_monitor_handler.go, frontend/src/api/admin/imageChannelMonitor.ts, frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: additive fork-local image monitor manual testing enhancement. It extends manual-run result payloads and browser-local manual history details, without changing image monitor database schemas or scheduled history tables.
**Change details**:
- Confirmed canceled manual tests are stored in browser-local manual history with final `canceled` state, elapsed time, prompt, and parameters.
- Added best-effort manual-test network metadata: exit IP via the same proxy path, API request URL/host/DNS IPs, and returned-image download URL/host/DNS IPs.
- Displayed the network metadata in current manual test result cards and the manual-history detail dialog.
- Intentionally deferred IP geolocation; it would require an IP database or external lookup service and a clearer privacy/update policy.
- Verified: `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`; `git diff --check`; `Invoke-WebRequest http://127.0.0.1:15174/admin/channels/image-monitor`.

## [2026-07-04] fix: allow manual image monitor panel to page-scroll

**Affected files**: frontend/src/views/admin/ImageChannelMonitorView.vue, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: frontend-only fork-local image monitor layout fix. It does not change backend APIs, schemas, monitor scheduling, or manual-test persistence.
**Change details**:
- Switched the image monitor page to `TablePageLayout` page-scroll mode only while the manual testing panel is active.
- Kept the regular monitor list in fixed table-scroll mode so the DataTable behavior is unchanged.
- Root cause: the manual testing form was rendered inside the table slot of `TablePageLayout`; fixed mode wraps that slot in a fixed-height `overflow-hidden` card, so the channel-selection section was clipped instead of becoming scrollable.
- Verified: `pnpm run typecheck`; `git diff --check`; `Invoke-WebRequest http://127.0.0.1:15174/admin/channels/image-monitor`.

## [2026-07-04] feat: add detailed manual image test history

**Affected files**: backend/internal/service/image_channel_monitor_*.go, frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: additive fork-local image monitor manual testing enhancement. It keeps detailed manual history in browser-local storage and IndexedDB, and does not change the image monitor database schema or scheduled monitor history tables.
**Change details**:
- Added an explicit manual-history dialog entry in the image monitor manual testing panel.
- Persisted detailed manual test history with request settings, prompt, elapsed time, stage timings, final status, input image reference, generated image reference, and fallback generated-image URL.
- Stored manual input/generated image bytes in IndexedDB (`sub2api-image-channel-monitor` / `manual-images`) while keeping only metadata and references in localStorage.
- Allowed image-to-image presets to save and restore the uploaded input image with the preset settings.
- Added downloaded-image data URL capture for successful manual URL results up to 16 MiB, so generated images can be viewed from manual history after the upstream URL expires.
- Verified: `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`; `git diff --check`; `Invoke-WebRequest http://127.0.0.1:15174/admin/channels/image-monitor`.

## [2026-07-04] feat: add manual image test timing history and cancellation

**Affected files**: backend/internal/service/image_channel_monitor_*.go, backend/internal/handler/admin/image_channel_monitor_handler.go, backend/internal/server/routes/admin.go, frontend/src/api/admin/imageChannelMonitor.ts, frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: additive fork-local image monitor manual testing enhancement. It adds only an in-memory manual-run cancel path plus browser-local manual history; it does not change image monitor schemas or scheduled monitor history tables.
**Change details**:
- Added per-manual-run cancellation with `POST /admin/image-channel-monitors/:id/manual-test/:runID/cancel`, backed by a run-scoped `context.CancelFunc`.
- Added live elapsed-time display for running manual tests and final elapsed time in local manual history.
- Added browser-local manual test history under `sub2api:image-channel-monitor:manual-history:v1`, keeping the latest 50 completed/canceled runs with compact previews and request settings.
- Updated the manual testing UI with per-run cancel, cancel-all, history display, and clear-history controls.
- Verified: `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`; `git diff --check`.

## [2026-07-04] feat: add manual image test presets

**Affected files**: frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: frontend-only fork-local UX enhancement for the dedicated image monitor manual testing panel. It stores presets in browser localStorage and does not change backend schemas or APIs.
**Change details**:
- Added a manual-test preset toolbar that can save the current mode/model/prompt/size/quality/n/download/timeout settings, apply a selected preset, update it, or delete it.
- Persisted presets under `sub2api:image-channel-monitor:manual-presets:v1`; uploaded image files are intentionally not stored.
- Updated Chinese and English i18n strings plus the image monitor module documentation.
- Verified: `pnpm run typecheck`; `git diff --check`.

## [2026-07-04] fix: restrict production service ports to loopback

**Affected files**: deploy/docker-compose.yml, deploy/.env.example
**Upstream compatibility**: deployment hardening only. No backend, frontend, schema, image, or API behavior changes.
**Change details**:
- Changed the Docker Compose default app port binding from `0.0.0.0:8080` to `127.0.0.1:8080`, keeping public access through host Caddy on 80/443.
- Changed PostgreSQL and Redis published ports to `127.0.0.1:5432` and `127.0.0.1:6379` to prevent public database/cache exposure.
- Updated `.env.example` so new deployments default to loopback binding.
- Production hotfix applied on `root@172.245.247.80` with backup `docker-compose.yml.bak-security-20260703-163646`; verified public `8080`, `5432`, and `6379` are closed while `https://zerocode.kaynlab.com/health` returns `{"status":"ok"}`.

## [2026-07-03] fix: key mapping rows by requested model

**Affected files**: backend/internal/domain/constants.go, backend/internal/domain/constants_test.go, backend/internal/service/global_model_pricing_service_test.go, frontend/src/components/admin/model-pricing/ModelPricingTab.vue, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: fork-local admin model mapping correction. No schema, migration, image-channel monitoring, push, or deployment changes.
**Change details**:
- Added Anthropic LiteLLM alias defaults such as `claude-4-sonnet-20250514 -> claude-sonnet-4-20250514`, so those request models are treated as mapping records instead of plain pricing rows.
- Changed the frontend mapping table to use the request model name as the unique row key. If the same request model appears from a pricing row and an upstream aggregate row, only one editable row is rendered.
- Added regression coverage for Anthropic alias mapping discovery and the requested-model alias example.

## [2026-07-03] fix: expand every default mapping into an editable row

**Affected files**: backend/internal/service/global_model_pricing_service.go, backend/internal/service/global_model_pricing_service_test.go, frontend/src/api/admin/modelPricing.ts, frontend/src/components/admin/model-pricing/ModelPricingTab.vue, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: fork-local admin model configuration correction. No schema, migration, image-channel monitoring, push, or deployment changes.
**Change details**:
- Added per-key billing-object metadata to mapping hints so multi-source mappings can display the correct `计费对象` for every source key.
- Changed the model configuration table to expand multi-source default mappings into one row per mapping relationship instead of hiding extra mappings behind `+N`.
- Edit, delete, and billing-object actions now operate on each expanded row's source mapping key, so all effective mappings have their own operation entry.
- Added regression coverage for multi-source upstream-only mappings and same-name mappings with aliases.

## [2026-07-03] fix: make effective default mappings fully editable

**Affected files**: backend/internal/service/{setting_service.go,wire.go,global_model_pricing_service.go,global_model_pricing_service_test.go,setting_service_model_mapping_test.go}, frontend/src/api/admin/modelPricing.ts, frontend/src/components/admin/model-pricing/ModelPricingTab.vue, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: fork-local admin model configuration correction. No schema, migration, image-channel monitoring, push, or deployment changes.
**Change details**:
- Changed Antigravity default mapping persistence so a saved table is treated as the full effective table. Saving `{}` now intentionally means no default mappings, preventing deleted built-in mappings from reappearing after reload.
- Changed model-pricing hints to return `mapping_key` and mark effective default mapping key rows editable, including built-in/runtime default and LiteLLM-discovered mapping rows.
- Enabled `计费对象` editing for same-name and upstream-only mapping relationship rows by saving against `mapping_key` instead of the row's pricing model name.
- Updated frontend edit/delete/billing-object actions to operate on `mapping_key`; this fixes rows where the visible pricing model is the mapped target rather than the requested source.
- Verified: targeted service tests for editable hints and empty Antigravity overrides; `pnpm --dir frontend run typecheck`.

## [2026-07-03] fix: add editable billing object for default model mappings

**Affected files**: backend/internal/domain/constants.go, backend/internal/service/{account.go,setting_service.go,global_model_pricing_service.go,gateway_service.go,openai_gateway_service.go}, backend/internal/handler/admin/account_handler.go, backend/internal/server/routes/admin.go, frontend/src/components/admin/model-pricing/{ModelPricingTab.vue,ModelMappingInlinePopover.vue}, frontend/src/api/admin/{accounts.ts,modelPricing.ts}, frontend/src/i18n/locales/{zh,en}.ts, frontend/src/views/admin/ChannelsView.vue, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: fork-local admin model configuration fix. It adds Settings KV entries and admin APIs for per-platform default mapping billing-object overrides, but does not add schema/migration changes and does not touch image channel monitoring.
**Change details**:
- Replaced the model configuration table's derived "映射角色" label with an editable "计费对象" field for platform default mapping key rows.
- Added per-platform `*_default_model_mapping_billing_object` settings and `GET/PUT /api/v1/admin/accounts/default-model-mapping-billing-objects/:platform`; valid values are only `requested` and `mapped`.
- Kept the default behavior as `requested`, so existing traffic still prices by the client-requested model unless an administrator explicitly selects `mapped`.
- Applied the billing-object override only to platform default mappings. Account-level `credentials.model_mapping`, channel `billing_model_source`, and token/image billing mode remain separate mechanisms.
- Added the initial `mapping_editable` backend flag. The later "make effective default mappings fully editable" entry above supersedes the first custom-only editability rule.
- Restored channel edit support for existing channel billing sources after removing the mistaken model-config channel billing-basis panel.

## [2026-07-03] fix: show billed image tier in user usage records

**Affected files**: backend/internal/handler/dto/types.go, backend/internal/handler/dto/mappers.go, frontend/src/types/index.ts, frontend/src/views/user/UsageView.vue
**Upstream compatibility**: small user-facing usage display adjustment. It exposes the existing usage log `billing_tier` field to regular usage DTOs and changes only the user usage table image token cell.
**Change details**:
- Added `billing_tier` to regular user usage records so image rows can display the actual billed tier.
- Changed the user usage token cell for image requests from request size display to billed-tier display, e.g. `1张（2K计费）`.
- Kept image quality visible under the billed-tier label and intentionally removed request-size text from that cell.
- Verified: `go test -tags=unit ./internal/handler/dto`; `pnpm --dir frontend exec eslint src/views/user/UsageView.vue src/types/index.ts`; `git diff --check`.
- Note: full frontend `pnpm --dir frontend run typecheck` is currently blocked by unrelated `ImageChannelMonitorView.vue` `number` vs `Timeout` errors.

## [2026-07-03] fix: make manual image channel tests asynchronous

**Affected files**: backend/internal/service/image_channel_monitor_*.go, backend/internal/handler/admin/image_channel_monitor_handler.go, backend/internal/server/routes/admin.go, frontend/src/api/admin/imageChannelMonitor.ts, frontend/src/views/admin/ImageChannelMonitorView.vue, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: fork-local image monitor UX/runtime fix. It keeps the dedicated image monitor module isolated from the generic channel monitor and does not add schema changes.
**Change details**:
- Changed manual image tests so `POST /admin/image-channel-monitors/:id/manual-test` starts an in-memory async run and returns `run_id` plus current status immediately.
- Added `GET /admin/image-channel-monitors/:id/manual-test/:runID` for polling request stages and final preview results.
- Updated the manual testing panel to poll each selected channel independently, show the current stage while running, and render metrics/images as soon as a channel completes.
- Root cause: manual tests previously held the browser request open through image generation and optional image download; the frontend Axios 30s timeout surfaced this as generic `Network error. Please check your connection.` even when the backend job continued.
- Verified: `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`; `git diff --check`.

## [2026-07-03] feat: add manual image channel test panel

**Affected files**: backend/internal/service/image_channel_monitor_*.go, backend/internal/handler/admin/image_channel_monitor_handler.go, backend/internal/server/routes/admin.go, frontend/src/api/admin/imageChannelMonitor.ts, frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: additive fork-local image monitor tooling. It reuses existing image monitor sources and HTTP upstream/proxy/TLS resolution, but keeps ad-hoc manual checks separate from scheduler state and persisted history.
**Change details**:
- Added `POST /admin/image-channel-monitors/:id/manual-test` for ad-hoc image checks against an existing image monitor source.
- Manual checks support text-to-image via `/v1/images/generations` and image-to-image via multipart `/v1/images/edits`, collect request/response/download timings, and return preview data without writing monitor history.
- Added a top-card switch in the admin image monitor page between scheduled channel monitoring and a manual testing panel.
- The manual panel supports configurable model/prompt/size/quality/n/timeout/download options, file upload for image-to-image, multi-channel selection, concurrent requests, per-channel status, metrics, stage list, and immediate preview as each channel finishes.
- Verified: `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`; `git diff --check`.

## [2026-07-03] fix: expose provider-aware default mapping controls

**Affected files**: backend/internal/service/global_model_pricing_service.go, backend/internal/service/global_model_pricing_service_test.go, frontend/src/components/admin/model-pricing/ModelPricingTab.vue, frontend/src/views/admin/ChannelsView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: admin model-config UI/backend-list fix. No schema, migration, Ent, image-channel monitoring, pricing formula, quota, push, or deployment changes.
**Change details**:
- Fixed provider-aware default mapping hints in the model pricing list so non-Antigravity mapping rows receive `billing_basis_hint`.
- The table-label and per-row billing behavior from this earlier entry was corrected by the later "editable billing object" change above; model configuration now uses `计费对象` with only `requested` and `mapped` choices.
- Channel `billing_model_source` remains a separate channel form setting and is not edited from the model configuration table.
- Verified: `go test -tags=unit ./internal/service -run "TestGlobalModelPricingListPrefersOverrideProvider|TestGlobalModelPricingListAddsProviderMappingHintWithoutFilter|TestAccountPlatformDefaultModelMapping|TestAccountGetMappedModel|TestAccountResolveMappedModel|TestOpenAIAccountResolveClaudeGPTBridgeModel" -count=1`; `pnpm run typecheck`.

## [2026-07-03] fix: align image monitor size options with OpenAI image API

**Affected files**: backend/ent/schema/image_channel_monitor.go, backend/migrations/176_image_channel_monitor_size_default.sql, backend/internal/service/image_channel_monitor_*.go, frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: small fork-local image monitor adjustment. It does not change generic channel monitoring or gateway request behavior; image monitor now stores blank `size` as intentional omission and passes custom sizes through to upstream validation.
**Change details**:
- Changed image monitor default `size` to blank so the monitor can omit the `size` request field instead of forcing `1024x1024`.
- Replaced the incorrect 1K/2K/4K square preset selector with size modes: omit `size`, send `auto`, use OpenAI standard presets (`1024x1024`, `1536x1024`, `1024x1536`), or enter a custom `WIDTHxHEIGHT` value.
- Added service regression coverage for omitting blank `size` and passing custom dimensions through unchanged.
- Verified: `go generate ./ent`; `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`; `git diff --check`.

## [2026-07-03] feat: optimize image channel monitor runtime controls

**Affected files**: backend/ent/schema/image_channel_monitor.go, backend/migrations/175_image_channel_monitor_proxy.sql, backend/internal/service/image_channel_monitor_*.go, backend/internal/repository/image_channel_monitor_repo.go, backend/internal/handler/admin/image_channel_monitor_handler.go, backend/internal/server/routes/admin.go, backend/cmd/server/wire_gen.go, frontend/src/api/admin/imageChannelMonitor.ts, frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: additive fork-local extension to the dedicated image monitor. It keeps the generic channel monitor untouched and adds only optional columns/API fields plus an in-memory runtime status endpoint.
**Change details**:
- Added optional custom-source proxy binding (`proxy_id`, `proxy_name`) for image monitors and applies the resolved proxy to both the image generation API request and returned-image download probe.
- Changed manual `POST /admin/image-channel-monitors/:id/run` to start checks asynchronously and return runtime status immediately, avoiding frontend network errors while long image generation continues in the background.
- Added `GET /admin/image-channel-monitors/:id/status` with per-monitor running/stage/message timestamps and next-check countdown data for UI polling.
- Updated the admin image monitor page with size presets, custom-source proxy selection, and a per-row status bar showing current stage and next scheduled check countdown.
- Verified: `go generate ./ent`; `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`.

## [2026-07-03] feat: add dedicated image channel monitor

**Affected files**: backend/ent/schema/image_channel_monitor*.go, backend/migrations/174_image_channel_monitors.sql, backend/internal/service/image_channel_monitor_*.go, backend/internal/repository/image_channel_monitor_repo.go, backend/internal/handler/admin/image_channel_monitor_handler.go, backend/internal/server/routes/admin.go, backend/cmd/server/wire_gen.go, frontend/src/api/admin/imageChannelMonitor.ts, frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/router/index.ts, frontend/src/components/layout/AppSidebar.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: additive fork-local module. It does not modify the existing generic channel monitor schema, provider adapters, rollups, or user-facing channel status view. Future upstream changes to the generic monitor should have limited conflict surface except shared DI/router/sidebar files.
**Change details**:
- Added independent image monitor tables for monitor configuration and per-run timing history, with custom API source and OpenAI API-key account source.
- Custom source stores an encrypted API key and public HTTPS base endpoint; account source stores only `account_id` and resolves the current account base URL, API key, proxy, concurrency, and TLS profile at run time.
- Image checks call `/v1/images/generations` with `response_format=url`, record API header/body/total timing, response shape (`has_url`, `has_b64_json`), returned URL host, and optional returned-image download timing/size/dimensions.
- Added an independent scheduler/runner, admin CRUD/run/history endpoints under `/api/v1/admin/image-channel-monitors`, and an admin submenu at `渠道管理 -> 图片渠道监控`.
- Added focused service tests for account-source request construction and `b64_json` response handling.
- Verified: `go generate ./ent`; `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`. `go generate ./cmd/server` was attempted but blocked by a local Wire tool `go.sum` missing entry, so `wire_gen.go` was manually reconciled.

## [2026-07-03] feat: redesign login page visuals to Figma v2 (purple gradient)

**Affected files**: frontend/src/views/auth/LoginView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: frontend-only visual layer rewrite of the login view; all login logic (auth store flow, Turnstile, TOTP 2FA modal, legal consent dialog, LinuxDo/WeChat/OIDC OAuth sections, backend-mode/password-reset flags, admin login_page overrides) is preserved unchanged. Diverges further from upstream login UI; watch this file on upstream merges.
**Change details**:
- Rebuilt template per the Figma v2 design (file 5DlRiTxu0w28djyDCdl1Xf, frames 25:2 / 25:75): left purple-gradient hero (#2563EB→#7C3AED→#EC4899) with brand tile, admin-overridable badge/heading/description, a static "live usage bill" sample card, three model cards (Opus 4.7 / GPT-5.4 / Gemini 3.1 Pro) and a 7×24 / 100% / 0 stats row; right light-theme form with trust badges, mail/lock input icons, gradient submit button, outline register button, and two capability cards (gpt-image-2 / tutorials).
- Mobile: gradient hero with the form card floating over it, forgot-password link, trust chips, and key-usage/docs links (previously desktop-only nav pills).
- Wired the previously unused `login_page.description` admin override into the hero paragraph; form switched from dark to light theme (Turnstile theme dark→light).
- i18n: replaced `auth.login.features.*`, `postLoginInfo`, `postLoginDetails`, `keyUsageLink` with new v2 keys (billCard*, modelCard*, stat*, trustBadge*, cap*, mobileHero*, registerButton) in both zh and en; login form title default changed to 欢迎回来 / Welcome back, hero heading defaults to 登录后，即刻接入 / 最新旗舰模型.
- Verified: `pnpm --dir frontend run typecheck`, `lint:check`, i18n locale spec suite, plus live check on the dev stack (127.0.0.1:15174/login desktop + 390px iframe mobile viewport; admin session backed up and restored).

## [2026-07-03] fix: complete provider-aware model config UI

**Affected files**: backend/internal/service/global_model_pricing_service.go, backend/internal/service/global_model_pricing_service_test.go, frontend/src/components/admin/model-pricing/ModelPricingTab.vue, frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue, frontend/src/components/admin/model-pricing/ModelPricingInlinePopover.vue, frontend/src/components/admin/model-pricing/ModelMappingInlinePopover.vue, frontend/src/components/admin/model-pricing/ModelTestDialog.vue, frontend/src/components/admin/model-pricing/modelPricingOptions.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: fork-local admin model-config UI and provider filtering behavior. No schema, migration, Ent, image-channel monitoring, billing formula, quota, push, or deployment changes.
**Change details**:
- Centralized provider normalization/options for Anthropic, OpenAI, Gemini, and Antigravity so model pricing, default mappings, detail dialogs, inline quick edits, and model tests use the same platform vocabulary.
- Added provider selection to model tests and account loading, so tests schedule against accounts from the selected provider instead of defaulting to Antigravity for every non-OpenAI/Gemini case.
- Replaced free-text provider editing in the model pricing detail dialog with a provider select, and made inline quick edit support provider plus billing mode changes without opening the full dialog.
- Updated global model pricing list/detail behavior so an override provider is visible and participates in provider filtering, ensuring newly changed provider values can be selected, listed, and scheduled consistently.
- Verified: `go test -tags=unit ./internal/service -run TestGlobalModelPricingListPrefersOverrideProvider -count=1`; `go test -tags=unit ./internal/service -run "TestGlobalModelPricingListPrefersOverrideProvider|TestAccountPlatformDefaultModelMapping|TestAccountGetMappedModel|TestAccountResolveMappedModel|TestOpenAIAccountResolveClaudeGPTBridgeModel" -count=1`; `pnpm run typecheck`; `pnpm run build`.

## [2026-07-03] feat: add provider-aware default model mappings

**Affected files**: backend/internal/domain/constants.go, backend/internal/handler/admin/account_handler.go, backend/internal/server/routes/admin.go, backend/internal/service/account.go, backend/internal/service/domain_constants.go, backend/internal/service/global_model_pricing_service.go, backend/internal/service/setting_service.go, backend/internal/service/wire.go, frontend/src/api/admin/accounts.ts, frontend/src/api/admin/modelPricing.ts, frontend/src/components/admin/model-pricing/ModelMappingInlinePopover.vue, frontend/src/components/admin/model-pricing/ModelPricingTab.vue, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: fork-local admin model-config and scheduling behavior. No schema, migration, Ent, unrelated monitoring, billing formula, or quota changes.
**Change details**:
- Added provider selection when admins add or edit default model mappings from the model configuration page, supporting Anthropic, OpenAI, Gemini, and Antigravity instead of always writing Antigravity.
- Added platform-scoped default mapping settings and admin APIs at `/api/v1/admin/accounts/default-model-mapping/:platform`, while keeping the legacy Antigravity endpoint compatible.
- Wired platform default mappings into account model resolution so configured OpenAI/Anthropic/Gemini mappings can rewrite upstream model names and be schedulable without turning those platforms into restrictive allowlists. Antigravity keeps its strict built-in allowlist behavior.
- Updated model pricing list hints/filtering so mapped request models appear under their selected provider.
- Verified in a clean detached worktree containing only this feature: `go test -tags=unit ./internal/service -run "TestAccountPlatformDefaultModelMapping|TestAccountGetMappedModel|TestAccountResolveMappedModel|TestOpenAIAccountResolveClaudeGPTBridgeModel" -count=1`; `pnpm run typecheck`; `go test -tags=unit ./internal/service -count=1`; `pnpm run build`.

## [2026-07-02] fix: allow admin reassignment of expired subscriptions

**Affected files**: backend/internal/service/subscription_service.go, backend/internal/service/subscription_assign_idempotency_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: backend subscription grant fix. No schema, migration, route, frontend, billing formula, or deployment changes.
**Change details**:
- Fixed admin subscription assignment for users who already have an expired same-group subscription, such as an expired GPT monthly-card grant created by a previous redeem code.
- Reactivating an expired same-group subscription now resets `starts_at`, `expires_at`, status, assigned admin metadata, notes, and daily/weekly/monthly usage windows instead of returning `SUBSCRIPTION_ASSIGN_CONFLICT` because old notes or validity differ.
- Preserved active-subscription idempotency and conflict checks so duplicate admin requests do not silently extend active subscriptions.
- Verified: `go test -tags=unit ./internal/service -run "TestAssignSubscription|TestBulkAssignSubscription|TestNormalizeAssignValidityDays|TestDetectAssignSemanticConflictCases"`; `go test -tags=unit ./internal/service`; local API smoke with a temporary `admin_api_key` and expired subscription row, then DB/settings restored.

## [2026-07-02] fix: align user model pricing override fields

**Affected files**: frontend/src/components/admin/user/UserModelPricingModal.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only admin UI polish. No backend, API payload, billing/display calculation, schema, or route behavior changes.
**Change details**:
- Reordered the user-level model pricing display override fields to mirror the billing override order: input, output, cache write, 1h cache write, cache read.
- Added user-modal-specific display cache write/read labels so the left and right override columns use consistent wording while preserving the existing `display_cache_creation*` payload fields.
- Verified: `pnpm --dir frontend run typecheck`; `pnpm --dir frontend run lint:check`; `git diff --check`.

## [2026-07-02] merge: integrate staged upstream sync with display billing fixes

**Affected files**: codex/upstream-sync-20260627 merge set, dev-services.yml, docs/dev/CHANGELOG_CUSTOM.md, docs/dev/UPSTREAM_SYNC.md, backend/internal/handler/admin/usage_handler.go, backend/internal/handler/dto/display_pricing.go, backend/internal/handler/dto/mappers.go, frontend/src/api/admin/usage.ts, frontend/src/components/admin/usage/UserViewCompareDrawer.vue, frontend/src/components/admin/usage/__tests__/UserViewCompareDrawer.spec.ts
**Upstream compatibility**: local integration merge. No push, deployment, migration execution, quota mutation, stored usage mutation, or real billing formula change in this merge resolution.
**Change details**:
- Merged the staged upstream safety-sync branch `codex/upstream-sync-20260627` into the display-billing integration branch, resolving conflicts only in the dev-console manifest and upstream-sync documentation.
- Preserved the display-billing invariants fixed earlier: user-facing model unit prices come from configured/effective prices, not usage-cost reverse math; cache-read token counts remain real; cache-read display deltas fold into input display cost/tokens when needed.
- Combined the local `dev-services.yml` managed-stack entry with upstream-sync's `cwd`, backend health check, `full`, `stop`, and status variants while keeping the repository rule that normal service actions go through `scripts/dev-stack.ps1`.
- Tightened the admin user-view calculation drawer so only the real billing layer may show an implicit `cost/tokens` unit price. The user display layer now uses only backend-supplied effective display prices, including cache-creation display prices, and otherwise shows no invented unit price.
- Verified: `go test -tags=unit ./internal/handler/dto ./internal/handler/admin`; `go test -tags=unit ./internal/handler ./internal/handler/admin ./internal/handler/dto ./internal/service ./internal/repository ./internal/pkg/apicompat ./internal/pkg/openai ./cmd/server`; `pnpm --dir frontend run test:run -- src/components/admin/usage/__tests__/UserViewCompareDrawer.spec.ts src/views/user/__tests__/UsageView.spec.ts`; `pnpm --dir frontend run test:run -- src/components/admin/usage/__tests__/UserViewCompareDrawer.spec.ts src/views/user/__tests__/UsageView.spec.ts src/router/__tests__/title.spec.ts src/views/admin/__tests__/SettingsView.spec.ts`; `pnpm --dir frontend run typecheck`; `pnpm --dir frontend run lint:check`.

## [2026-07-02] feat: expose admin user-view cost calculation process

**Affected files**: AGENTS.md, docs/dev/ARCHITECTURE.md, docs/dev/codebase/billing.md, backend/internal/handler/admin/usage_handler.go, backend/cmd/server/wire_gen.go, frontend/src/api/admin/usage.ts, frontend/src/components/admin/usage/UserViewCompareDrawer.vue, frontend/src/components/admin/usage/__tests__/UserViewCompareDrawer.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: fork-local admin debugging UI and documentation. No database, stored billing, quota, push, or deployment changes.
**Change details**:
- Added the display-billing invariants to the repository entry rules: user display prices must come from configured/effective pricing, cache-read tokens stay real, cache-read display deltas fold into input, and displayed bills must remain explainable from displayed tokens, unit prices, and rate multiplier.
- Aligned the admin user-view preview endpoint with the same effective unit-price resolver path as user usage endpoints, including User -> Channel -> Global -> LiteLLM/Fallback pricing.
- Added real-layer and user-display-layer cost calculation process panels to the admin user perspective comparison drawer, showing token components, unit prices, component subtotal, other cost, `total_cost x rate`, `actual_cost`, and the diff.
- Added frontend coverage for the fable/cache-read style calculation process so the drawer preserves the explainable display-bill invariant.

## [2026-07-02] fix: use configured display unit prices in user usage

**Affected files**: backend/cmd/server/wire_gen.go, backend/internal/handler/usage_handler.go, backend/internal/handler/gateway_handler.go, backend/internal/handler/dto/types.go, backend/internal/handler/dto/mappers.go, backend/internal/handler/dto/display_pricing.go, backend/internal/handler/dto/display_pricing_test.go, backend/internal/service/model_pricing_resolver.go, backend/internal/service/model_pricing_resolver_test.go, backend/internal/service/display_token_rewrite.go, backend/internal/service/display_token_rewrite_test.go, frontend/src/utils/usagePricing.ts, frontend/src/types/index.ts, frontend/src/views/user/UsageView.vue, frontend/src/views/KeyUsageView.vue, frontend/src/views/user/__tests__/UsageView.spec.ts, docs/dev/codebase/billing.md
**Upstream compatibility**: fork-local user display and billing presentation fix. No database, route, stored usage, real billing, quota, push, or deployment changes.
**Change details**:
- Added effective unit-price fields to user usage DTOs and changed user/API-key usage tooltips to use those configured prices instead of reverse-deriving unit prices from rounded display tokens. Explicit display-price overrides win; otherwise the backend resolves the configured model price through the existing User → Channel → Global → LiteLLM/Fallback pricing chain.
- Removed the user tooltip fallback that computed model unit price from `cost / tokens`; if the backend cannot resolve a unit price, the frontend shows an empty value instead of inventing one from usage costs.
- Fixed the fable-style small-token rounding case where input cost `$0.000025` and displayed input tokens `3` produced a false `$8.3333/M` tooltip even though the configured display input price is `$10/M`.
- Preserved real cache-read token quantities in user usage display transforms and downstream display-mode response rewrites; display-rate scaling now keeps cache-read cost tied to cache-read tokens/unit price and folds the cache-read rate delta into input display tokens/cost so the displayed bill remains explainable.
- Added focused backend and frontend regression coverage for configured unit prices and non-scaled cache-read counts.

## [2026-07-02] fix: restore local dev-console manifest

**Affected files**: dev-services.yml, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: local developer tooling only. No runtime, database, billing, frontend, or deployment behavior changes.
**Change details**:
- Restored the missing repository-root `dev-services.yml` so the local dev-console can register and reload the Sub2API project instead of failing with `Missing config`.
- Modeled the console-managed entrypoint around the canonical `scripts/dev-stack.ps1` workflow and kept backend/frontend/sidecars as monitor services, preserving the repository rule that normal local service actions go through the dev-stack script.
- Recorded Sub2API's strict local ports (`18081`, `15174`, `3000`, `3100`, `13200`) in the manifest for dev-console status, health checks, and project board grouping.

## [2026-07-02] sync: Sonnet 5 production-only upstream patch

**Affected files**: backend/internal/pkg/claude/constants.go, backend/internal/domain/constants.go, backend/internal/service/settings_view.go, backend/internal/service/gateway_beta_test.go, backend/internal/service/bedrock_request_test.go, backend/internal/domain/constants_test.go, backend/internal/pkg/claude/constants_test.go, frontend/src/composables/useModelWhitelist.ts, docs/dev/UPSTREAM_SYNC.md, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: Manual partial sync from upstream commit `db0414233ce324903adc72e858374086da158b4b` (`feat: 适配 sonnet5`). This intentionally excludes the same upstream commit's unrelated `backend/internal/pkg/anthropicfp/dateline.go` changes and does not include any unfinished local OpenAI/Image work from the current conversation.
**Change details**:
- Added `claude-sonnet-5` to the Claude OAuth default model list so `/v1/models` can expose the model.
- Added the Bedrock default mapping `claude-sonnet-5 -> us.anthropic.claude-sonnet-5-v1`; existing Bedrock region-prefix adjustment still rewrites it according to account `aws_region`.
- Changed the default `context-1m-2025-08-07` beta policy from blanket filter to a Sonnet 5 whitelist: Sonnet 5 direct/Vertex/Bedrock IDs pass, non-whitelisted models continue to filter the beta token.
- Added frontend whitelist/preset entries for Anthropic Sonnet 5 and Bedrock Sonnet 5 so admins can pick the model in account mapping UI.
- Added regression tests for the default Claude model list, Bedrock mapping constants, Bedrock region adjustment, and the Sonnet 5-only 1M context beta whitelist.
- Verified: `go test -tags=unit ./internal/pkg/claude ./internal/domain ./internal/service -count=1`; `pnpm --dir frontend run typecheck`; `pnpm --dir frontend run build`; `go build -tags embed -trimpath ./cmd/server`; `git diff --check`.

## [2026-07-02] feat(billing): display cache creation price — 缓存创建纳入展示放大体系 + 用户侧可见性

**Affected files**: backend/migrations/171_add_display_cache_creation_price.sql, backend/internal/service/{global_model_pricing,user_model_pricing,user_model_pricing_service,global_model_pricing_service}.go, backend/internal/repository/{global_model_pricing_repo,user_model_pricing_repo}.go, backend/internal/handler/admin/{model_pricing_handler,user_model_pricing_handler,usage_handler}.go, backend/internal/handler/dto/display_pricing{,_test}.go, backend/tools/upstream-sync-guard/main.go, frontend/src/types/index.ts, frontend/src/api/admin/{usage,modelPricing,userModelPricing}.ts, frontend/src/views/user/UsageView.vue, frontend/src/views/KeyUsageView.vue, frontend/src/components/admin/usage/{UsageTable,UserViewCompareDrawer}.vue, frontend/src/components/admin/{model-pricing/ModelPricingDetailDialog,user/UserModelPricingModal}.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/billing.md
**Upstream compatibility**: additive, fork-local。新增 DB 列 `display_cache_creation_price`（global_model_pricing + user_model_pricing_overrides，NULL=未配置=行为零变化）；DisplayUsageFields 增加两个 admin 契约字段；用户 DTO 无新 JSON 字段。upstream-sync-guard 已登记 `DisplayCacheCreationPrice` 关键签名。
**Change details**:
- 背景：anthropic 平台记录（如 claude-fable-5，input=2/output=38/cache_creation=42778/$0.54）在用户侧"token 很少但很贵"——缓存创建 token/成本此前完全不参与展示换算，且用户可用 cache_creation_cost/tokens 反推真实缓存写单价。
- 核心（display_pricing.go）：新分支在 ApplyDisplayTransform 中把缓存创建 token 直接按展示价反算放大（display_tokens = 真实成本 ÷ 展示价，cost 保持守恒），**与 cache-read 的 premium 折入 input 机制刻意不同**（用户明确要求：直接放大缓存创建自身 token 数）。守卫：展示价>0 && tokens>0 && cost>0，不依赖 display_input_price。线性变换 → 聚合组与逐行天然等价，GetUserDisplayAggregateGroups 零改动。
- 5m/1h 细分：新 helper rescaleCacheCreationBreakdown 等比缩放 + 减法导出，保证 5m+1h==total；ApplyUserDisplayRate 同步接入（修复既有"细分不随展示倍率缩放"bug）。
- 长上下文：effectiveDisplayPricingForUsageLog 对新价乘 LongContextInputMultiplier。
- 配置链：migration 171（含 user 表 NOT VALID 非负约束，模板 147）→ 实体/两个 raw-SQL repo 全枚举点（global 4 处、user 5 处）→ 校验（validateUserModelPricingOverride）→ admin API（global create/partial-update applyFloat、user create/update/batch）→ 前端两个定价表单（$/MTok 双向换算、applyDisplaySuggested 从 cache_write_price 取建议值）→ i18n zh/en。
- Admin 可视：DisplayUsageFields + ComputeDisplayFields 增加 display_cache_creation_tokens/cost；UsageTable 双列 tooltip 增行；UserViewCompareDrawer config_used 回传展示创建价。
- 用户侧可见性（此前完全不显示）：UsageView.vue 与 KeyUsageView.vue 的 token 徽章（amber 图标+1h 标签）、token tooltip（5m/1h 细分）、成本 tooltip、token 合计均渲染 cache creation；admin 专属 TTL override "R" 徽章仍不下发用户。UsageView.spec.ts 两个断言"用户侧隐藏缓存创建"的旧规格测试已反转。
- 平台边界（软 gate，详见 billing.md 2026-07-02 节）：openai 原生/antigravity OAuth/桥接/gemini 行 cache_creation 恒 0 → no-op；antigravity 分组的 upstream 中转/apikey 型账号行与 openai relay 透传行若命中已配置的 claude-* 模型会同样换算（语义正确）。
- **本批不改**：display_token_rewrite.go（下游响应 CacheCreateMult 仍恒 1.0）；claude-gpt 桥接 openai_claude_gpt_bridge_cache_display_settings；真实计费链。下游一致性如需跟进，前置为 gateway_service.go OAuth 流式 extractSSEUsagePatch 计费污染修复（PLAN 文档 Phase 0，未实施）。
- Verified: `go build ./...`、`go test -tags=unit ./internal/handler/... ./internal/service/... ./internal/repository/...` 全过（新增 8 个 display_pricing 用例：放大/独立守卫/no-op/与 read premium 复合/长上下文单次缩放/ComputeDisplayFields/倍率细分一致性）；`./internal/server -run Contract` 仅 redeem/history 一处**既有**失败（基线同样失败，与本改动无关）；前端 typecheck + lint:check + vitest 全量 101 文件/603 用例全过。

## [2026-07-02] fix(billing): 流式计费 patch 先于展示改写提取 —— 修复 display 模式真实扣费污染

**Affected files**: backend/internal/service/gateway_service.go, backend/internal/service/gateway_service_streaming_test.go
**Upstream compatibility**: 单行重排,fork-local。
**Change details**:
- 根因:processSSEEvent 先对共享 SSE event map 做展示改写(ApplyDisplayMultipliersToUsageMap 就地变异),后 extractSSEUsagePatch 从同一 map 提取计费 → mergeSSEUsagePatch → ForwardResult.Usage → calculateTokenCost。`downstream_usage_token_mode=display`(migration 169 起新用户默认)且展示倍率非平凡时,**真实扣费按展示 token 计算**(生产已配置展示倍率,污染已实际发生)。
- 修法:extractSSEUsagePatch 上移到 cache TTL override(刻意影响计费归类,保持在前)之后、display 改写之前;display 改写仍作用于发给客户端的序列化对象,展示语义不变。顺带修复 marshal 失败回退路径"客户端见真实值、计费用展示值"的不自洽。
- 影响面:所有走 GatewayService 流式路径的账号(anthropic OAuth/SetupToken/ServiceAccount/APIKey + antigravity 分组 apikey 型账号)。**行为变化:display 模式用户的流式扣费从污染值恢复为真实值**(已拍板只修复+记录,不做历史修正)。其余路径经三轮探索核实均为"先提取后改写",安全:passthrough 流式/非流式、标准非流式、claude-gpt 桥接(response-only)、OpenAI 原生全路径、antigravity(hook 变异 usageToMap 全新拷贝,计费走独立累计字段)。
- 红/绿回归:TestGatewayService_StreamingDisplayModeBillsRealTokens(修复前红)、TestGatewayService_StreamingDisplayModeKeepsTTLOverrideBeforeBillingPatch(TTL 归类仍先于提取)。

## [2026-07-02] feat(billing): cache_write_1h_price —— 1h 缓存创建按溢价分档计费

**Affected files**: backend/migrations/172_add_cache_write_1h_price.sql, backend/internal/service/{global_model_pricing,global_model_pricing_service,model_pricing_resolver}.go, backend/internal/repository/global_model_pricing_repo.go, backend/internal/handler/admin/model_pricing_handler.go, backend/internal/service/model_pricing_resolver_test.go, frontend/src/api/admin/modelPricing.ts, frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: additive。新列 NULL = 历史行为逐字节不变(回归钉测试)。
**Change details**:
- 背景:官方缓存写入分两档(5m=1.25×输入价,1h=2×输入价)。【2026-07-02 修正】走 LiteLLM 源价的模型(sonnet-5/fable-5)本就按官方分档正确计费——生产 sonnet-5 纯 1h 行隐含 $4.0/MTok = 官方优惠期 1h 价(2×$2),经官方价目表核实,原"1h 溢价漏计"诊断不成立。被压平的是配了全局 cache_write_price 覆盖的模型(opus 系列 $10 平价、sonnet-4-6 $5 平价):单一覆盖价同写三档,1h 溢价无法表达——本字段即为此而设。
- 全局定价覆盖新增 cache_write_1h_price(migration 172):配置后 applyGlobalPricingOverride 单独写 CacheCreation1hPrice 并强制 SupportsCacheBreakdown=true,computeCacheCreationCost 按 5m×p5m+1h×p1h 分档;admin 表单加"1h 缓存写入价"输入框($/MTok),i18n zh/en。
- **运营动作**:部署后给 claude-sonnet-5 / claude-fable-5 等中转模型配置 1h 价(按上游实际扣费口径);此后新请求真实成本计入 1h 溢价(admin 成本与用户 actual_cost 同步变化)。
- 测试:纯 1h 生产形状(66061 tokens)按 1h 价计费、混合行分档、未配置时平价行为回归钉。

## [2026-07-02] feat(billing): 下游响应 usage 缓存创建展示改写(real/display 双模式)

**Affected files**: backend/internal/service/display_token_rewrite{,_test}.go, docs/dev/codebase/billing.md
**Upstream compatibility**: fork-local。real 模式零变化;display 模式仅在配置了 display_cache_creation_price 的模型上激活。
**Change details**:
- computeDisplayTokenMultipliers 接入缓存创建:CacheCreateMult(无明细回退,5m 档口径对齐计费回退)+ CacheCreate5mMult/CacheCreate1hMult 分档倍率(真实档价÷展示创建价);displayTokenPricingConfig/两个 merge 函数补真实价与展示价管道;IsNonTrivial 纳入分档判断(仅配展示创建价即可激活改写链)。
- 新 helper computeDisplayCacheCreationBreakdown:有嵌套 5m/1h 明细时按档反算(displayTotal×展示价 == 5m×p5m+1h×p1h,与 usage 页成本反算口径逐 token 一致,含纯 1h 中转流量),display1h 减法导出保证 5m+1h==顶层;无明细退化单一倍率。接入 rewriteSeparatedUsageTokens(passthrough 流式/非流式+桥接,顶层与嵌套同步 sjson 回写)与 ApplyDisplayMultipliersToUsageMap(托管流式+antigravity hook;antigravity map 无嵌套,行为不变)。applyOpenAIResponsesUsageDisplayMultipliers 的 CacheCreationInputTokens 改为同规则缩放(桥接恒 0,no-op)。
- RateScale(展示倍率层)在分档反算后复合,与 ApplyUserDisplayRate 串联语义一致。
- 前置依赖:同日的流式计费 patch 顺序修复(否则缓存创建计费会被本改写污染)。
- Verified: go build/vet;display token 全部用例(既有 11 + 新增 8:分档倍率计算/用户级覆盖优先/嵌套同步/纯 1h 生产形状/RateScale 复合/无嵌套回退/OpenAI 结构缩放/trivial no-op);gateway 流式与 handler/repository 全量单测通过。

## [2026-07-02] feat(billing): 5m/1h 缓存分档价格配置面补全（用户级真实价 + 全局/用户级展示价 + LiteLLM 参考）

**Affected files**: backend/migrations/173_add_cache_tier_pricing_fields.sql, backend/internal/service/{global_model_pricing,user_model_pricing,user_model_pricing_service,global_model_pricing_service,model_pricing_resolver,display_token_rewrite}.go, backend/internal/repository/{global_model_pricing_repo,user_model_pricing_repo}.go, backend/internal/handler/admin/{model_pricing_handler,user_model_pricing_handler,usage_handler}.go, backend/internal/handler/dto/display_pricing{,_test}.go, backend/internal/service/{display_token_rewrite_test,model_pricing_resolver_test}.go, backend/tools/upstream-sync-guard/main.go, frontend/src/api/admin/{modelPricing,userModelPricing,usage}.ts, frontend/src/components/admin/{model-pricing/ModelPricingDetailDialog,user/UserModelPricingModal,usage/UserViewCompareDrawer}.vue, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: additive。migration 173 新增三列均 NULL=行为零变化;LiteLLMPrices 载荷加 cache_write_1h_price(来自 litellm 的 cache_creation_input_token_cost_above_1hr)。
**Change details**:
- **用户级真实 1h 价** `user_model_pricing_overrides.cache_write_1h_price`:applyUserModelPricingOverride 与全局同语义(单独写 CacheCreation1hPrice + 强制 SupportsCacheBreakdown),用户级也能表达 1h 溢价分档计费。
- **展示价分档** `display_cache_creation_1h_price`(全局 + 用户级):
  - usage-log 展示(ApplyDisplayTransform):行有 5m/1h 细分且两档展示价不同时,按真实档价比例(r=1h/5m,来自定价条目的 RealCacheWritePrice/RealCacheWrite1hPrice,未知时按 1:1)拆分实际落库成本,各档独立反算展示 token —— 成本总额按构造守恒;只配 5m 档展示价时保持既有"总成本反算"路径(回归钉)。
  - 下游改写(computeDisplayTokenMultipliers):CacheCreate1hMult 分母改用 1h 展示价(未配回退 5m 档展示价),两侧口径一致。
  - 长上下文克隆对 1h 展示价同乘 LongContextInputMultiplier;hasDisplayOverride/BuildUserDisplayPricingMap/merge 函数全链纳入。
- **配置界面补全**:全局定价对话框(LiteLLM 参考区 + 计费区 1h 输入框带 litellm placeholder + 展示区 1h 输入框 + applyDisplaySuggested 从 litellm 1h 取建议)、用户定价模态框(LiteLLM 参考行 + 真实/展示两个 1h 输入框 + 建议值 + $/MTok 双向换算)、对比抽屉 config_used 展示 1h 展示价;i18n zh/en。
- **口径答疑**(用户提问,billing.md 亦有记载):所有支持缓存的 Claude 模型都有 5m/1h 两档,是否出现取决于调用方请求的 TTL;无分档价的模型走平价回退(total × CacheCreationPricePerToken);上游未返回 5m/1h 细分时全部按 5m 价计费(计费与展示两侧一致)。
- Verified: go build/vet 全过;新增 6 个单测(dto 分档反算/1:1 兜底/单价回归钉,resolver 用户级 1h,display_token 1h 展示价倍率/用户级 1h 真实价);后端全量 unit 测试、前端 typecheck+lint+603 用例全过。

## [2026-07-02] docs: record Hajimi candidate 4K key availability failure

**Affected files**: docs/dev/OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: documentation only; no backend/frontend runtime behavior, route, database, billing, i18n, or migration changes.
**Change details**:
- Recorded the new `hajimicc.top` native 4K candidate key check by key fingerprint only; the full key is stored only in the ignored local test-secret registry under `tmp/image-channel-secrets/`.
- Documented that quality c1 and concurrency c2/c4/c8 all fail before generation with HTTP 503: `No available channel for model gpt-image-2 under group 4K-3（原生） (distributor)`.
- Recorded that no image URL host or no-proxy direct download can be measured for this candidate key until the upstream group has an available `gpt-image-2` channel.
- Added the current no-proxy direct-access probe for the existing `www.geek2api.com` image URL host, including the observed ~10s first-byte latency.

## [2026-07-01] docs: record Hajimi native 4K image channel diagnostics

**Affected files**: docs/dev/OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: documentation only; no backend/frontend runtime behavior, route, database, billing, i18n, or migration changes.
**Change details**:
- Recorded the direct `hajimicc.top` native 4K image-channel quality smoke test using the existing long 4K storyboard prompt.
- Documented visual text-clarity findings for the generated contact sheet.
- Recorded `c2/c8/c16` concurrency results under a 4-minute test limit, including API latency, image download latency, body throughput, strict end-to-end success count, and URL/base64 response shape.

## [2026-07-01] docs: record Hajimi native-vs-relay current-exit retest

**Affected files**: docs/dev/OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: documentation only; no backend/frontend runtime behavior, route, database, billing, i18n, or migration changes.
**Change details**:
- Recorded a native `hajimicc.top` versus relay `zerocode.kaynlab.com` retest for the Hajimi native 4K channel.
- Documented that `curl.exe` still observed a Tokyo exit despite the intended Hong Kong switch.
- Recorded `quality-c1` and `c2/c8/c16` results, including image download throughput improvement, relay c16 HTTP 429 failures, and URL-only response shape.

## [2026-06-30] docs: record OpenAI image URL relay 4K diagnostics

**Affected files**: docs/dev/OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md, docs/dev/ARCHITECTURE.md, docs/dev/codebase/README.md, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: documentation only; no backend/frontend runtime behavior, route, database, billing, i18n, or migration changes.
**Change details**:
- Added a production diagnostic record for OpenAI API-key image URL relay behavior after the `v0.1.151` forced-URL hotfix.
- Recorded the `gpt image 2 高质量` group permission finding, the native 4K quality smoke result, and the `c2/c4/c8` 4K concurrency baseline.
- Documented the timing split between Sub2API API URL response latency and downstream image URL download latency.
- Recorded the completed Japan-proxy `c2/c8/c16` timing run, including API pre-body latency, image URL pre-body latency, body download time, throughput, URL hosts, and URL/base64 response shape.

## [2026-06-29] hotfix: force URL responses for OpenAI API-key images

**Affected files**: backend/internal/service/openai_images.go, backend/internal/service/openai_images_test.go
**Upstream compatibility**: fork-local production performance guard for OpenAI-compatible API-key image forwarding. No API route, database, billing, frontend, i18n, or migration changes.
**Change details**:
- Forced API-key `/v1/images/generations` JSON requests to send `response_format: "url"` upstream even when downstream clients explicitly request `b64_json`.
- Forced API-key `/v1/images/edits` multipart requests to rewrite or append `response_format=url`, covering image-edit clients that submit multipart form fields.
- This intentionally trades off `b64_json` compatibility for the API-key relay path to prevent downstream request shape from reintroducing multi-megabyte base64 image response bodies and response-download long tails.
- Verified with unit coverage for JSON explicit-format override, multipart override, API-key generations forwarding, and API-key edits forwarding.

## [2026-06-29] fix: OpenAI image API-key fallback user-agent

**Affected files**: backend/internal/service/openai_images.go, backend/internal/service/openai_images_test.go
**Upstream compatibility**: fork-local OpenAI-compatible image forwarding hardening. No API route, database, billing, frontend, i18n, or migration changes.
**Change details**:
- Added a fallback `User-Agent: node` for OpenAI API-key `/v1/images/generations` and `/v1/images/edits` upstream requests when neither the downstream client nor the account `credentials.user_agent` provides one.
- Preserved the existing precedence: account `credentials.user_agent` overrides client UA; client UA is otherwise passed through; fallback is used only to avoid Go's default `Go-http-client/1.1` on image upstreams.
- Added unit coverage for default fallback, client UA passthrough, and account UA override.
- Verified: `go test ./internal/service -run 'TestBuildOpenAIImagesRequest_APIKeyUserAgentFallback|TestOpenAIGatewayServiceForwardImages_APIKey'`; `go test ./internal/service`.

## [2026-06-29] perf: OpenAI API-key image relay URL-format default

**Affected files**: backend/internal/service/openai_images.go, backend/internal/service/openai_images_test.go
**Upstream compatibility**: fork-local performance optimization for OpenAI-compatible API-key image forwarding. No route, database, billing, frontend, i18n, or migration changes.
**Change details**:
- For API-key JSON image requests that do not explicitly set `response_format`, Sub2API now sends `response_format: "url"` upstream. Explicit client formats such as `b64_json` are preserved.
- The optimization avoids upstreams returning multi-megabyte `b64_json` payloads when the client did not ask for base64. In the 4K diagnostic case this reduced response bodies from ~7-8MB to ~5.7KB and removed the previous 35-40s post-generation body-download tail.
- Non-streaming image responses now begin writing downstream when upstream response headers arrive, while still buffering the copied body for usage/image-count extraction after completion.
- Verified with unit coverage for default URL format, explicit format preservation, response body copy/buffering, and API-key forwarding. Live diagnostics: `1024x576 low` no-format request returned `has_b64_json=false`, `wire_response_bytes=484`, and `body_after_headers_ms=15.9`; 4K `c2` URL-format relay returned `has_b64_json=false`, `wire_response_bytes=5732`, with body-after-headers `0.43s` and `2.20s`.

## [2026-06-29] chore: register project with local dev-console

**Affected files**: dev-services.yml, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: local development tooling only; no backend/frontend runtime behavior, migration, route, billing, gateway, or i18n changes.
**Change details**:
- Added `dev-services.yml` so the standalone dev-console can show Sub2API as its own project board.
- Registered monitor entries for backend (`18081`), frontend (`15174`), optional AIClient2API (`3000`/`3100`), and optional new-api (`13200`).
- Added a `dev-stack` control entry that routes normal start/restart/status/stop actions through `scripts/dev-stack.ps1`, preserving this repo's local startup rule instead of directly launching `air` or `pnpm dev`.
- Verified registration with `devconsole.py register --root`, `devconsole.py list`, and dev-console `GET /api/ping`.

## [2026-06-29] sync: upstream OpenAI Images route batch

**Affected files**: backend/internal/service/openai_codex_transform.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_ws_forwarder.go, backend/internal/service/openai_images_responses.go, backend/internal/service/image_output_accounting.go, backend/internal/service/*openai*image*_test.go, backend/internal/service/image_output_accounting_test.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of OpenAI Images route behavior from `e5f7836b`, `0da1fe28`, `2c14efea`, `da30c599`, and `381d1d6d`. Deferred `36721d35`, `1e2e8b1d`, and `ef5ad0fb` for separate capability-cooldown, pricing, and frontend-display batches.
**Change details**:
- Codex `/v1/responses` image bridge now sets `tool_choice: "auto"` when the bridge injects or preserves an `image_generation` tool and the client did not provide an explicit tool choice; the same helper is used by HTTP and WS ingress paths.
- OpenAI image-output accounting now counts only real image outputs from `data` arrays (`url`/`b64_json`) and ignores empty `image_generation.completed` events, preventing false image-output billing on text-only Responses payloads.
- OAuth `/v1/images/generations` and `/v1/images/edits` bridging to Responses now forwards `n` for supported image models while keeping `dall-e-3` at single-image behavior.
- Retryable OpenAI Images upstream errors embedded in Responses SSE bodies are converted into `UpstreamFailoverError` before any downstream response is written, with standard JSON error bodies and cloned upstream headers for existing failover/ops handling.
- Fork-local impact: no frontend-visible change, no route/i18n/settings/migration change, no curated model list or Claude-GPT bridge change. Intentional impact is limited to OpenAI image generation, image billing counter correctness, and existing account failover behavior for retryable image upstream failures.
- Verified: `go test -tags=unit ./internal/service -run "Test(EnsureOpenAIResponsesImageGenerationTool|OpenAIGatewayService_Forward_CodexImageBridgeSetsToolChoiceAuto|OpenAIGatewayService_Forward_StripsImageGenerationToolForSparkAPIKey|OpenAIImageOutputCounter|BuildOpenAIImagesResponsesRequest|OpenAIGatewayServiceForwardImages_OAuth)" -count=1`; `go test -tags=unit ./internal/service -count=1`; `git diff --check`.

## [2026-06-28] sync: upstream OpenAI gateway/probe compatibility batch

**Affected files**: backend/internal/pkg/openai/constants.go, backend/internal/pkg/openai/instructions_gpt5_5.txt, backend/internal/pkg/openai/instructions_test.go, backend/internal/service/openai_gateway_chat_completions.go, backend/internal/service/openai_gateway_chat_completions_raw.go, backend/internal/service/gateway_request.go, backend/internal/service/gateway_request_test.go, backend/internal/service/openai_apikey_responses_probe.go, backend/internal/service/*openai*_test.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of `00d68ff6`, `dbdbfb11`, `89cfe24a`, and `b88f8e4c`. OpenAI chat transport-error failover parity was already present and left unchanged; PAT auth, quota-readiness, and codex-detect engine-fingerprint changes remain deferred for separate assessment.
**Change details**:
- Added upstream GPT-5.5 Codex instructions and made non-specific GPT-5.x Codex prompt fallback use the latest embedded prompt while keeping explicit Codex model IDs on this fork's existing default Codex prompt.
- Updated OAuth `/v1/chat/completions` bridge handling so converted chat requests keep an empty `instructions` field instead of injecting the default long Codex instructions.
- Added GLM raw chat-completions reasoning effort normalization (`low`/`medium`/`high` -> `high`; `x-high`/`max`/`ultracode` -> `max`) after account model mapping resolves to a `glm-*` upstream model.
- Hardened OpenAI API-key `/v1/responses` probing by selecting a concrete mapped upstream model, sending a required function-call probe, reading a bounded response body, and treating 2xx responses without `function_call` output as unsupported.
- Preserved fork-local TLS fingerprint probing, `codex_cli_only` chat-completions restriction, account scheduling/failover boundaries, billing/display-token accounting, curated model-list policy, Claude-GPT bridge behavior, OpenAI Images behavior, default-model fallback, migrations, routes, frontend i18n, subscriptions, and payment behavior.
- Verified: `go test -tags=unit ./internal/pkg/openai -run TestCodexBaseInstructionsForModel -count=1`; `go test -tags=unit ./internal/service -run "Test(ForwardAsChatCompletions_OAuthDoesNotInjectDefaultInstructions|NormalizeGLMOpenAIReasoningEffort|ForwardAsRawChatCompletions_NormalizesGLMReasoningEffort|OpenAIResponsesProbePayloadRequiresFunctionCall|SelectResponsesProbeModel|DecideResponsesProbeSupport)$" -count=1`; `go test -tags=unit ./internal/pkg/openai -count=1`; `go test -tags=unit ./internal/service -run "Test.*(OpenAI|Responses|ChatCompletions|GLM|Codex|Probe|TransportError|RawChat)" -count=1`; `git diff --check`.

## [2026-06-28] sync: upstream Claude Code no-cch detection test batch

**Affected files**: backend/internal/service/claude_code_validator_test.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of `5cb8cdd3` as a local test-only adaptation. Evaluated `30adee43` but did not apply it because this fork no longer contains the upstream `OpenAIQuotaResetCell.vue` entry point or any `openaiQuotaReset` frontend references.
**Change details**:
- Added a Claude Code validator regression test proving no-cch billing blocks still cannot bypass the required Claude Code User-Agent check.
- Kept existing local positive coverage for no-cch billing blocks via `TestClaudeCodeValidator_BillingBlockAnyEntrypointCountsAsSystemPrompt`.
- Did not import `6cfb7898`; no cch-signing or Claude mimicry runtime behavior was changed.
- Fork-local impact: no runtime behavior change, no frontend-visible change, no billing/display-token, model-list, routing, account scheduling, subscription, payment, migration, or i18n behavior change. The only code change is test coverage for the existing Claude Code/Codex compatibility path.
- Verified: `go test -tags=unit ./internal/service -run "TestClaudeCodeValidator" -count=1`; `git diff --check`.

## [2026-06-27] feature: redeem code batch per-user limit

**Affected files**: backend/ent/schema/redeem_code.go, backend/ent/*redeemcode*, backend/migrations/170_redeem_code_batch_user_limit.sql, backend/internal/repository/redeem_code_repo.go, backend/internal/service/redeem_code.go, backend/internal/service/redeem_service.go, backend/internal/service/admin_service.go, backend/internal/handler/admin/redeem_handler.go, backend/internal/handler/dto/types.go, backend/internal/handler/dto/mappers.go, frontend/src/views/admin/RedeemView.vue, frontend/src/api/admin/redeem.ts, frontend/src/types/index.ts, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/redeem.md, docs/dev/codebase/README.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: fork-local admin/user redeem-code behavior. Additive DB fields and a partial unique index preserve existing codes and unrestricted batches.
**Change details**:
- Added optional generated redeem-code batch metadata and a per-batch switch so admins can make each user redeem at most one code from the current generated batch.
- Enforced the limit in `RedeemService.Redeem` before granting benefits and translated the DB unique-index fallback into `REDEEM_BATCH_LIMIT_EXCEEDED` for concurrent redemptions.
- Added the management UI checkbox, API/request/DTO fields, and Chinese/English i18n copy.
- Documented the redeem-code flow and the concurrency pitfall in `docs/dev/codebase/redeem.md`.
- Verified: `go generate ./ent`; `go test -tags=unit ./internal/service ./internal/repository ./internal/handler/admin`; `pnpm run typecheck`; `pnpm run lint:check`.

## [2026-06-27] sync: upstream OpenAI images and overloaded error verification batch

**Affected files**: docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: evaluated `9491de0a`, `b0d5592a`, and `cc7612bd`; no runtime code was changed because equivalent local commits already exist (`ae83aa9b` for content-moderation refusals, existing Images incomplete handling in `openai_images_responses.go`, and `92ec4294` for overloaded error code detection).
**Change details**:
- Confirmed OpenAI Images content-moderation refusals already return 400 `content_policy_violation` without failover retry.
- Confirmed OpenAI Images `response.incomplete` and no-output soft-failure handling already record ops diagnostics and preserve same-account retry behavior.
- Confirmed OpenAI overloaded/slow-down transient errors already trigger failover classification.
- Fork-local impact: no new code behavior change in this batch; it is a synchronization audit/documentation entry to avoid duplicate cherry-picks of already-ported OpenAI/Images fixes.
- Verified: `go test -tags=unit ./internal/service -run "Test(ExtractImagesUpstreamError|ImagesOAuthNonStreaming|ExtractModelRefusal|IsOpenAITransientProcessingError|OpenAIStreamingResponseFailedBeforeOutput(ServerOverloadedCode|CapacityError|ReturnsFailover)|OpenAIGatewayService_Forward_TransientProcessingErrorTriggersFailover)" -count=1`; `git diff --check`.

## [2026-06-27] sync: upstream auth promo and frontend title batch

**Affected files**: backend/internal/service/auth_email_binding.go, backend/internal/service/auth_service_email_bind_test.go, backend/internal/handler/auth_oauth_pending_flow_test.go, backend/internal/service/registration_email_policy.go, backend/internal/service/registration_email_policy_test.go, backend/internal/handler/admin/promo_handler.go, backend/internal/service/promo_service.go, frontend/src/App.vue, frontend/src/i18n/index.ts, frontend/src/router/index.ts, frontend/src/router/title.ts, frontend/src/router/__tests__/title.spec.ts, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of `ecedc7c8`, `2dc1387b`, and `952be871`, plus a local wildcard registration email suffix policy adaptation required by the upstream email-bind tests.
**Change details**:
- Email identity binding now enforces the registration email suffix whitelist, closing an OAuth pending-flow bypass.
- Registration email suffix whitelist now supports `*.domain` and `@*.domain` entries, normalized to `@*.domain`, matching subdomains only.
- Promo-code editing now allows admins to clear an existing expiry date.
- Custom-page document titles now refresh when route, site settings, custom menu items, admin state, or locale changes.
- Resolved frontend title conflicts by preserving this fork's existing auth/backend-mode/simple-mode route guard behavior and not importing unrelated upstream compliance-dialog context.
- Fork-local impact: auth policy becomes stricter when suffix whitelist is configured; promo expiry clearing affects admin promo operations; frontend-visible impact is limited to browser tab title refresh. No changes to billing/display-token accounting, curated model lists, Claude-GPT bridge, OpenAI Images, account scheduling, subscriptions, database migrations, API routes, or payment order amounts.
- Verified: `go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin -run "Test.*(Email|Bind|OAuth|Suffix|Promo|PromoCode|Pending)" -count=1`; `pnpm --dir frontend run test:run src/router/__tests__/title.spec.ts`; `pnpm --dir frontend run typecheck`; `pnpm --dir frontend run lint:check`; `git diff --check`.

## [2026-06-27] sync: upstream Claude Code detection and Vertex beta filtering batch

**Affected files**: backend/internal/service/claude_code_validator.go, backend/internal/service/claude_code_validator_test.go, backend/internal/service/gateway_service.go, backend/internal/service/gateway_anthropic_vertex_beta_filter_test.go, backend/internal/service/gateway_request.go, backend/internal/service/header_util.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of `e3e31bd4`, `40e1cc14`, and `efffd5d7`, plus the minimal helper surface from `ddf91e9a` required by the Vertex beta tests. The larger `ddf91e9a` count_tokens/API-key passthrough behavior and `6cfb7898` cch-signing deletion remain deferred.
**Change details**:
- Claude Code auto mode now treats any `cc_entrypoint=` marker as a Claude Code system prompt, not only `cc_entrypoint=cli`.
- Vertex Anthropic service-account forwarding now filters unsupported `anthropic-beta` tokens before setting the upstream header.
- Vertex request body sanitization now uses the final filtered beta header when deciding whether to strip `body.context_management`.
- Preserved fork-local ops request-body capture by calling `setOpsUpstreamRequestBody(c, vertexBody)` after the final Vertex body sanitize step.
- Adapted upstream Vertex beta tests to this fork's 2-return-value `buildUpstreamRequest` signature.
- Fork-local impact: no frontend-visible UI changes, no database migrations, no i18n/routes changes, and no changes to display-token/display-pricing accounting, curated model lists, Claude-GPT bridge dispatch, OpenAI Images, subscriptions, account scheduling, or billing. Intentional impact is limited to Claude Code client detection and Anthropic Vertex request header/body compatibility.
- Verified: `go test -tags=unit ./internal/service -run "TestClaudeCodeValidator|Test.*Vertex.*Beta|Test.*Anthropic.*Vertex|Test.*Beta.*Filter" -count=1`; `git diff --check`.

## [2026-06-27] sync: upstream small auth/ops/keys/payment guard batch

**Affected files**: backend/internal/service/auth_service.go, backend/internal/service/openai_gateway_chat_completions.go, frontend/src/views/admin/ops/OpsDashboard.vue, frontend/src/components/keys/UseKeyModal.vue, frontend/src/components/payment/PaymentProviderDialog.vue, frontend/src/components/payment/ProviderCard.vue, frontend/src/views/admin/SettingsView.vue, frontend/src/views/admin/__tests__/SettingsView.spec.ts, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of `82576e0a`, `9707dedc`, `ae5e980d`, `28e7adef`, and `65ad7df4`. The `codex_cli_only` chat-completions change conflicted in the fork-local OpenAI raw Chat fallback path and was reconciled by adding the restriction check before the existing local APIKey Responses/Chat split.
**Change details**:
- Fixed email auth identity creation error handling so a shadowed `err` no longer swallows failures.
- Constrained ops dashboard trend cards so the admin monitoring layout cannot grow unbounded.
- Enforced `codex_cli_only` account policy on `/v1/chat/completions`, including APIKey raw Chat fallback, without changing account scheduling or display-token accounting.
- Added `CLAUDE_CODE_ATTRIBUTION_HEADER=0` to Claude Code terminal usage templates in the key usage modal.
- Normalized empty/null payment provider `supported_types` so admin payment provider cards remain visible.
- Fork-local impact: no changes to billing/display-pricing math, curated model lists, Claude-GPT bridge dispatch, OpenAI images, subscriptions/bundle fulfillment, migrations, routes, or i18n. Intentional frontend-visible impact is limited to ops layout, key usage templates, and admin payment provider display.
- Verified: `go test -tags=unit ./internal/service -run "Test.*Auth|Test.*Email|Test.*OAuth|Test.*Register" -count=1`; `go test -tags=unit ./internal/service -run "Test.*(Codex|ChatCompletions|CLIOnly|ClientRestriction|RawChat|ResponsesChat)" -count=1`; `pnpm --dir frontend run test:run src/views/admin/__tests__/SettingsView.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts`; `pnpm --dir frontend run typecheck`; `pnpm --dir frontend run lint:check`; `git diff --check`.

## [2026-06-27] sync: upstream runtime compatibility batch

**Affected files**: .dockerignore, Dockerfile, deploy/Dockerfile, backend/internal/service/ratelimit_service.go, backend/internal/service/ratelimit_service_anthropic_window_limit_test.go, backend/internal/repository/http_upstream.go, backend/internal/repository/decompress_response_test.go, backend/internal/service/gateway_service.go, backend/internal/service/gateway_streaming_test.go, backend/internal/service/gemini_messages_compat_service.go, backend/internal/service/gemini_messages_compat_service_test.go, backend/internal/handler/openai_chat_completions.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/handler/openai_gateway_endpoint_normalization_test.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of `ad135854`, `f6e0ebc6`, `c1c28ac7`, `6c7203d8`, `6c2db4f4`, and `bab8a9a9`. No frontend UI change. Preserved fork-local scheduling/failover signatures, OpenAI usage-record worker context, WebSocket per-turn account handling, and did not import unrelated upstream risk-control/content-moderation helpers.
**Change details**:
- Docker production build context now includes `docs/legal` so admin compliance/legal assets remain available in image builds.
- Anthropic official account 5h/7d window exhaustion now persists the longer cooldown before temporary-unschedulable fallback rules; reconciled to this fork's 5-argument `RateLimitService.HandleUpstreamError` signature and existing rate-limit persistence path.
- Upstream HTTP repository responses with `Content-Encoding: zstd` are decompressed before downstream parsing/error handling.
- Streaming gateway now preserves SSE `event:error` raw data as a typed upstream error so ops logs show the real upstream error body instead of a generic stream failure.
- Gemini Messages compatibility now strips unsupported schema fields before forwarding tools to Gemini.
- OpenAI usage records now log `/v1/chat/completions` for API-key accounts forced/probed into raw Chat Completions, including `/responses`, `/messages`, raw chat, and Responses WebSocket recording paths. The manual port kept fork-local `turnAccount` WebSocket accounting and added endpoint resolver tests.
- Fork-local impact: no changes to display-token/display-pricing accounting, curated model lists, Claude-GPT bridge dispatch, OpenAI image generation, default-model fallback, i18n, migrations, or routes. Intentional impact is limited to runtime packaging, rate-limit cooldown choice, upstream body decoding, ops-log fidelity, Gemini request compatibility, and OpenAI upstream endpoint metadata.
- Verified: `go test -tags=unit ./internal/service -run "TestHandleUpstreamError_AnthropicWindowLimitPreemptsTempUnschedRule|Test.*Anthropic.*Window|Test.*Cooldown" -count=1`; `go test -tags=unit ./internal/repository -run "Test.*Decompress|Test.*Zstd|Test.*ContentEncoding" -count=1`; `go test -tags=unit ./internal/service -run "TestHandleStreamingResponse_(SSEErrorEvent|StreamReadError|FailoverBody|EmptyStream|SpecialCharacters)" -count=1`; `go test -tags=unit ./internal/service -run "Test(ConvertClaudeToolsToGeminiTools|CleanToolSchema|GeminiMessagesCompatServiceForward)" -count=1`; `go test -tags=unit ./internal/handler -run "Test(OpenAIUpstreamEndpoint|ResolveOpenAIUpstreamEndpoint)" -count=1`; `git diff --check`.

## [2026-06-27] sync: upstream low-risk tooling/auth/compat gateway batch

**Affected files**: skills/sub2api-admin/SKILL.md, skills/sub2api-admin/references/admin-cli.md, skills/sub2api-admin/scripts/sub2api-admin.js, backend/internal/service/token_refresh_service_test.go, backend/internal/pkg/apicompat/chatcompletions_to_responses.go, backend/internal/pkg/apicompat/chatcompletions_responses_test.go, backend/internal/service/gateway_service.go, backend/internal/service/gateway_non_streaming_response_test.go, backend/internal/handler/gateway_handler.go, backend/internal/handler/gateway_handler_intercept_test.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of small upstream fixes only; no Grok/PAT/codex-detect UI stack included. Local rate-limit service signature, admin skill install-path convention, and previous refresh-token invalidation behavior were preserved.
**Change details**:
- Added `SUB2API_JWT` fallback support to the bundled `sub2api-admin` skill and docs while keeping the local `~/.codex/skills/...` invocation path.
- Added test coverage for `app_session_terminated` and `refresh_token_invalidated` as non-retryable refresh errors; production code already contained the merged non-retryable markers.
- Changed apicompat Chat Completions -> Responses tool conversion so default tool `strict` is false, with focused schema tests.
- Added failover handling for non-streaming upstream HTTP 2xx responses whose bodies are not valid JSON; adapted the upstream helper to this fork's 5-argument `RateLimitService.HandleUpstreamError` signature.
- Extended `max_tokens=1` Haiku probe interception to streaming requests.
- Verified: `node --check skills/sub2api-admin/scripts/sub2api-admin.js`; `go test -tags=unit ./internal/service -run "TestIsNonRetryableRefreshError|TestNonRetryableRefreshError" -count=1`; `go test -tags=unit ./internal/pkg/apicompat`; `go test -tags=unit ./internal/service -run "Test.*Non.*JSON|Test.*NonStreaming.*Response|Test.*Failover.*Non" -count=1`; `go test -tags=unit ./internal/handler -run "Test.*Intercept|Test.*Haiku|Test.*Warmup|Test.*Suggestion" -count=1`; `git diff --check`.

## [2026-06-27] docs: require upstream-sync assessment table before each batch

**Affected files**: AGENTS.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: repository workflow documentation only; no runtime behavior change.
**Change details**:
- Added a mandatory upstream-sync preflight rule requiring an assessment table before every sync batch.
- The table must cover feature behavior, affected modules, frontend visibility, tests, fork-local secondary-development relationships, expected impact, risk, and handling strategy.
- Made the fork-local impact column mandatory for custom areas such as billing/display-token accounting, curated model lists, Claude-GPT bridge, OpenAI image generation, default-model fallback, scheduling/failover, ops logging, settings, migrations, i18n, and routes.

## [2026-06-27] sync: upstream Codex Spark image tool strip

**Affected files**: backend/internal/service/openai_codex_transform.go, backend/internal/service/openai_codex_transform_test.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_service_hotpath_test.go, backend/internal/service/openai_ws_forwarder.go, backend/internal/service/openai_ws_forwarder_ingress_test.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged sync of `01127820`; preserves fork-local request-body mutation/patch behavior and WS fast-policy flow.
**Change details**:
- Strips client-supplied `image_generation` tools for `gpt-5.3-codex-spark` and its effort aliases because Spark is text-only and upstream rejects that tool with `invalid_request_error`.
- Applies the strip in OAuth Codex transforms, HTTP `/responses` forwarding for APIKey/OAuth paths, and Responses WebSocket ingress.
- Reconciled upstream conflicts by adapting the HTTP path to the fork-local `reqBody` + `bodyModified` + `disablePatch` mechanism and keeping the local WS fast-policy/ops flow.
- Verified: `go test -tags=unit ./internal/service -run "Test(ApplyCodexOAuthTransform_StripsImageGenerationToolForSpark|ApplyCodexOAuthTransform_StripsImageGenerationToolForSparkAlias|ApplyCodexOAuthTransform_KeepsImageGenerationToolForNonSpark|OpenAIGatewayService_Forward_StripsImageGenerationToolForSparkAPIKey|StripCodexSparkImageGenerationToolFromRawPayload)" -count=1`; `git diff --check`.

## [2026-06-27] sync: upstream passthrough function-call argument dedupe

**Affected files**: backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_passthrough_function_args_test.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: clean staged cherry-pick of `2b49d662`; applies after the existing local display-token rewrite and response.failed sanitization paths.
**Change details**:
- Normalized OpenAI Responses passthrough function-call `arguments` fields when upstream sends the same JSON argument string duplicated in a single event payload.
- Applied the normalization to streaming passthrough events, corrected SSE response bodies, output item payloads, and completed response output arrays.
- Added focused tests covering raw Responses passthrough and forced Chat Completions fallback output.
- Verified: `go test -tags=unit ./internal/service -run "Test(HandleStreamingResponsePassthroughDeduplicatesFunctionCallArguments|ForwardResponsesChatCompletionsFallbackKeepsFunctionArgumentsSingle|Dedupe|PassthroughFunction)" -count=1`; `git diff --check`.

## [2026-06-27] sync: upstream model availability 404 safety fix

**Affected files**: backend/internal/handler/gateway_handler.go, backend/internal/handler/gateway_handler_chat_completions.go, backend/internal/handler/gateway_handler_responses.go, backend/internal/handler/gemini_v1beta_handler.go, backend/internal/handler/no_account_error.go, backend/internal/handler/no_account_error_test.go, backend/internal/handler/openai_chat_completions.go, backend/internal/handler/openai_embeddings.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/handler/openai_images.go, backend/internal/handler/ops_error_logger.go, backend/internal/service/gateway_model_availability.go, backend/internal/service/gateway_model_availability_test.go, backend/internal/service/openai_gateway_model_availability.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged upstream sync of `fcd3bc12`; preserves fork-local OpenAI default-model fallback, Claude-GPT bridge fallback, compact unsupported error handling, and ops logging context.
**Change details**:
- Added conservative model-availability diagnosis helpers so "group has accounts but none support this requested model" returns 404 `model_not_found` instead of a misleading 503.
- Kept 503 behavior for transient exhaustion, empty account pools, diagnosis failures, and model-empty paths.
- Threaded the classifier through Anthropic/OpenAI/Gemini gateway account-selection failure paths, including chat completions, responses, embeddings, images, and count-tokens.
- Added ops routing-capacity markers needed by the upstream handler changes and kept routing-capacity events categorized as routing errors.
- Reconciled local conflicts by preserving default mapped-model fallback for OpenAI Chat Completions and Claude-GPT bridge fallback behavior before applying the 404 classifier.
- Verified: `go test -tags=unit ./internal/service -run "Test.*ModelAvailability" -count=1`; `go test -tags=unit ./internal/handler -run "Test.*NoAccount" -count=1`; `git diff --check`.

## [2026-06-27] sync: upstream OpenAI/apicompat/images safety batch 1

**Affected files**: backend/internal/pkg/apicompat/openai.go, backend/internal/pkg/apicompat/openai_test.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_service_test.go, backend/internal/service/openai_gateway_service_codex_cli_only_test.go, backend/internal/service/openai_gateway_chat_completions.go, backend/internal/service/openai_gateway_chat_completions_raw.go, backend/internal/service/openai_upstream_transport_error_handle_test.go, backend/internal/service/token_refresh_service.go, backend/internal/service/openai_images_responses.go, backend/internal/service/openai_images_incomplete_test.go, docs/dev/UPSTREAM_SYNC.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged upstream sync only; no full upstream merge. The local fork's display-token rewrite behavior, OpenAI image trace logging, custom model discovery, billing/display semantics, and gateway account failover behavior are preserved.
**Change details**:
- Cherry-picked/manual-ported upstream apicompat fixes for custom tool schema normalization and single-chunk `tool_call` argument deduplication.
- Sanitized verbose OpenAI `response.failed` event payloads before forwarding to clients while preserving usage/error handling in local streaming and passthrough paths.
- Recognized `server_is_overloaded`, `slow_down`, selected-model-at-capacity, and processing-error `response.failed` messages as retryable failover events before generic `invalid_request` non-retryable filtering.
- Treated `refresh_token_invalidated` as a non-retryable OAuth refresh credential failure.
- Let Chat Completions transport errors return `UpstreamFailoverError` so the gateway can switch accounts instead of writing a hard 502 from the transport path.
- Images no-output handling now distinguishes content-policy text refusals (400, no retry) from true empty upstream responses (retryable same-account failover), with upstream SSE error/incomplete helpers and focused tests.
- Verified: `go test -tags=unit ./internal/pkg/apicompat`; `go test -tags=unit ./internal/service -run "Test(ExtractImagesUpstreamError|SummarizeNoOutputBody|ImagesOAuthNonStreaming|ExtractModelRefusal|HandleOpenAIUpstreamTransportError|ForwardAsRawChatCompletions_TransportErrorFailsOver|IsOpenAITransientProcessingError|OpenAIStreamingResponseFailed|OpenAIStreamingPassthroughResponseFailed|NonRetryableRefreshError)" -count=1 -v`; `git diff --check`.

## [2026-06-26] chore: satisfy CI lint annotations

**Affected files**: backend/cmd/server/main.go, backend/ent/schema/mixins/soft_delete.go, backend/internal/server/http.go, backend/internal/service/credit_snapshot_service.go, backend/internal/service/credit_snapshot_service_test.go, backend/internal/service/distribution.go, backend/internal/service/image_generation_intent.go, backend/internal/service/image_output_accounting.go, backend/internal/service/display_token_rewrite.go, backend/internal/service/openai_messages_bridge.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_compat_prompt_cache_key.go, backend/internal/service/openai_ws_forwarder.go, backend/internal/service/payment_amounts.go, backend/internal/service/payment_config_service.go, backend/internal/pkg/antigravity/schema_cleaner.go, backend/internal/pkg/tlsfingerprint/dialer_capture_test.go, backend/internal/repository/ops_repo.go, backend/internal/repository/usage_log_repo.go, backend/internal/repository/usage_log_repo_request_type_test.go, backend/internal/repository/antigravity_usage_aggregator.go, backend/internal/repository/announcement_read_repo.go, backend/internal/repository/global_model_pricing_repo.go, backend/internal/handler/admin/tutorial_page_handler.go, backend/internal/handler/admin/pricing_page_handler.go, backend/internal/handler/admin/model_pricing_handler.go, backend/internal/handler/pricing_page_handler.go, backend/tools/smoke/main.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: no intended behavior change except returning an upload write error if closing the destination file fails. The rest only makes existing ignored cleanup/write errors explicit or satisfies staticcheck/gofmt annotations for golangci-lint.
**Change details**:
- Logged scheduled credit snapshot capture failures instead of dropping the returned error.
- Made intentionally ignored `strings.Builder.WriteString`, `Rows.Close`, uploaded multipart file close, and cleanup remove errors explicit.
- Propagated destination-file close failures from tutorial image upload as a write failure and cleaned up the partial file.
- Added a nil filter guard for ops error-log query building, removed an ineffectual distribution assignment, used a direct pricing content response conversion, and kept current h2c behavior with precise staticcheck suppressions.
- Removed a stray local type declaration from Antigravity schema cleaning, added precise unused suppressions for retained helper/request types across bridge, websocket, image accounting, payment, and pricing code, documented an intentional `time.Time` location-identity comparison in the credit snapshot test, formatted the pricing repository/handler files, and updated the stale usage stats SQL mock column list.
- Made the default TLS fingerprint capture-server integration test skip only certificate validity failures from the bundled external URL so an expired external cert does not block unrelated releases; explicit `TLSFINGERPRINT_CAPTURE_URL` overrides still fail on TLS validity errors.

## [2026-06-26] chore: track frontend `form-data` audit exception

**Affected files**: .github/audit-exceptions.yml, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: CI/security metadata only; no runtime behavior change.
**Change details**:
- Added a short-lived audit exception for `form-data` GHSA-hmw2-7cc7-3qxx because the browser frontend does not use Node-side multipart field-name or filename construction, and the current lockfile already resolves `form-data` to 4.0.5.
- Kept the exception expiring on 2026-07-10 so the next axios/jsdom dependency refresh must revisit it.

## [2026-06-26] fix: default new users to downstream display usage tokens

**Affected files**: backend/internal/service/user.go, backend/ent/schema/user.go, backend/migrations/169_default_downstream_usage_token_mode_display.sql, backend/internal/service/admin_service_update_user_rpm_test.go, backend/internal/service/user_defaults_test.go, backend/ent/schema/auth_identity_schema_test.go, frontend/src/components/admin/user/UserEditModal.vue, docs/dev/codebase/billing.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: fork-local default behavior for the existing `users.downstream_usage_token_mode` setting. Explicit `real` remains supported; existing users keep their stored mode. New users and missing internal values now default to `display`.
**Change details**:
- Changed `NormalizeDownstreamUsageTokenMode` and the shared default constant so empty or internal fallback values resolve to `display`.
- Changed the Ent schema default and added migration 169 to update the PostgreSQL column default for production.
- Updated the admin user edit modal fallback from `real` to `display` so unset legacy payloads match the backend default.
- Updated focused service/schema tests and billing documentation to lock the default.

## [2026-06-26] improve: curate OpenAI and Antigravity `/v1/models` discovery lists

**Affected files**: backend/internal/service/models_list_policy.go, backend/internal/service/admin_service.go, backend/internal/service/models_list_policy_test.go, backend/internal/handler/gateway_handler.go, backend/internal/handler/gateway_models_list_test.go, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: fork-local presentation policy for model discovery. It only changes `/v1/models`, `/antigravity/models`, `/antigravity/v1/models`, and the admin custom-model-list candidate choices for OpenAI/Antigravity. Scheduling, group allow/block checks, account model mapping, bridge forwarding, billing, and usage recording are unchanged.
**Change details**:
- Added shared `GatewayModelDiscoveryIDsForPlatform` policy: OpenAI exposes only `gpt-5.5`, `gpt-5.4`, `gpt-5.4-mini`; Antigravity exposes only `claude-opus-4-8`, `claude-opus-4-7`, `claude-opus-4-6`, `claude-haiku-4-5`, `claude-sonnet-4-6`.
- `GatewayHandler.Models` now returns these curated lists before account-derived `model_mapping` aggregation for OpenAI/Antigravity. Group `models_list_config` can narrow the curated list but cannot expand it.
- `/antigravity/models` and `/antigravity/v1/models` now use the same curated Antigravity discovery list while preserving display names from the Antigravity default model metadata.
- Admin `GET /api/v1/admin/groups/:id/models-list-candidates` uses the same curated candidates for OpenAI/Antigravity so the group custom-list UI cannot select models that the gateway will hide.
- Verified: `go test -tags=unit ./internal/handler -run 'TestGatewayHandlerModels|TestGatewayHandlerAntigravityModels'`; `go test -tags=unit ./internal/service -run 'TestGatewayModelDiscoveryIDsForPlatform|TestGetGroupModelsListCandidates_UsesGatewayDiscoveryPolicy'`.

## [2026-06-26] fix: hide Claude-GPT bridge-only mappings from OpenAI `/v1/models`

**Affected files**: backend/internal/service/gateway_service.go, backend/internal/service/gateway_hotpath_optimization_test.go, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: fork-local guard around the existing additive OpenAI Claude-GPT bridge. It only changes the presentation model list returned for OpenAI-platform API keys; model allow/block checks, model mapping, account scheduling, billing, usage recording, and Antigravity bridge forwarding are unchanged.
**Change details**:
- Root cause: `GatewayService.GetAvailableModels` aggregates `credentials.model_mapping` keys from schedulable accounts. OpenAI bridge accounts are still OpenAI accounts, so a mapping such as `claude-opus-4-8 -> gpt-5.5` was included in OpenAI-platform `/v1/models` discovery.
- Added a narrow service-layer filter that hides bridge-only Claude-family mapping keys from OpenAI `/v1/models` when `extra.openai_claude_gpt_bridge_enabled=true` and the mapping resolves to a distinct upstream OpenAI model.
- Preserved normal OpenAI model aliases such as `gpt-alias -> gpt-5.4`; when a group only has bridge-only Claude mappings, the model-list path falls back to platform defaults instead of exposing Claude IDs.
- Added a focused regression test for mixed OpenAI alias + Claude-GPT bridge mappings and bridge-only fallback behavior.
- Verified: `go test -tags=unit ./internal/service -run 'TestGetAvailableModels'` passes.

## [2026-06-21] feat: hide in-app tutorial page, route tutorial entries to a configurable (Feishu) link

**Affected files**: backend/internal/service/domain_constants.go, backend/internal/service/settings_view.go, backend/internal/service/setting_service.go, backend/internal/handler/dto/settings.go, backend/internal/handler/setting_handler.go, backend/internal/handler/admin/setting_handler.go, backend/internal/server/api_contract_test.go, frontend/src/types/index.ts, frontend/src/stores/app.ts, frontend/src/api/admin/settings.ts, frontend/src/views/admin/SettingsView.vue, frontend/src/router/index.ts, frontend/src/components/layout/AppSidebar.vue, frontend/src/components/user/dashboard/UserDashboardQuickActions.vue, frontend/src/components/keys/GettingStartedGuide.vue, frontend/src/views/user/KeysView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: additive public Settings-KV field (`tutorial_url`) following the existing `doc_url` pattern across constants/view/service/DTO/public+admin handlers; no schema migration, Wire, gateway, billing, or pricing changes. The in-app `/tutorial` view component (TutorialView.vue) and the admin tutorial-content editor are left in place but the user route is now a redirect, so existing installs lose nothing. May conflict with upstream if the public-settings struct chain, the sidebar nav, or the keys guide is refactored upstream.
**Change details**:
- Added a new public, admin-configurable setting `tutorial_url` (the external/Feishu tutorial link), threaded through the full `doc_url` chain: `SettingKeyTutorialURL` constant, both `settings_view.go` structs, the public-settings fetch/view/update in `setting_service.go` (including `PublicSettingsInjectionPayload` so the SSR drift test stays green), the public + admin DTOs, the public handler, and the admin GET/UPDATE handler plus its change-tracking diff.
- Updated `api_contract_test.go` expected JSON for both the admin settings GET and the public settings payload to include `tutorial_url`.
- Hid the in-app tutorial page: the `/tutorial` route is now a redirect to `/dashboard` (TutorialView.vue retained but unrouted).
- Routed all tutorial entry points to the configurable link, shown only when `tutorial_url` is set: the dashboard "view tutorial" card now opens the link in a new tab; the sidebar "Tutorial" nav item renders as an external link (added an `external?: string` field to NavItem and switched both user/personal nav render blocks to `<component :is>` so it emits an `<a target=_blank>`); and the keys-page guide gained a "Detailed Tutorial" button (new `tutorialUrl` prop passed from KeysView).
- Renamed the keys-page guide heading from "Getting Started" / 开始使用 to "Quick Tutorial" / 快速教程, and added `keys.guide.detailedTutorial` plus `admin.settings.site.tutorialUrl*` i18n keys (zh + en).
- Added `tutorialUrl` to the app store (ref, applySettings parse, fallback cached object, export) and `tutorial_url` to the PublicSettings type and admin settings API types/mapping.
- Verified with `go build ./...`, `go test -tags=unit ./internal/handler/dto -run SchemaDoesNotDrift`, `go test -tags=unit ./internal/server -run TestAPIContracts`, `go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin`, `pnpm --dir frontend run typecheck`, `pnpm --dir frontend run lint:check`, `pnpm --dir frontend exec vitest run src/stores/__tests__/app.spec.ts src/views/admin/__tests__/SettingsView.spec.ts`, and a live `GET /api/v1/settings/public` showing `tutorial_url`.

## [2026-06-20] feat: admin-configurable CCS import model for OpenAI/Codex

**Affected files**: backend/internal/service/domain_constants.go, backend/internal/service/setting_service.go, backend/internal/service/settings_view.go, backend/internal/handler/dto/settings.go, backend/internal/handler/setting_handler.go, backend/internal/handler/admin/setting_handler.go, backend/internal/server/api_contract_test.go, frontend/src/types/index.ts, frontend/src/stores/app.ts, frontend/src/api/admin/settings.ts, frontend/src/views/admin/SettingsView.vue, frontend/src/views/user/KeysView.vue, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: adds a new public Settings-KV key `ccs_import_codex_model` (string, default `gpt-5-codex`) following the existing `api_base_url` / `hide_ccs_import_button` plumbing exactly. Additive — could conflict if upstream restructures the settings DTO/struct chain or the KeysView CC Switch deeplink builder.
**Change details**:
- Root cause of the reported issue: the "Import to CC Switch" deeplink built in `KeysView.executeCcsImport` never sent a `model` param, so cc-switch's `build_codex_settings` fell back to its built-in default `gpt-5-codex` (verified against farion1231/cc-switch `src-tauri/src/deeplink/provider.rs`). The model was therefore not controllable from Sub2API.
- Added public setting `ccs_import_codex_model` (default `gpt-5-codex`) and wired it through the full Settings-KV chain: constant, public-keys list, both map->struct assemblies, the injection payload + `GetPublicSettingsForInjection`, the updates map (TrimSpace), `settings_view` PublicSettings/SettingsView structs, public + admin DTOs, admin request struct, admin response mappers, and the admin change-diff list.
- Admin UI: new text input under OEM > "Hide CCS Import Button" in SettingsView, bound to `form.ccs_import_codex_model`, with zh/en labels/hint/placeholder. Loaded via the existing bulk `Object.entries(settings)` assign; saved via the existing payload mapper.
- KeysView: for the `openai` platform only, `executeCcsImport` now appends `model=<ccs_import_codex_model>` to the deeplink when the setting is non-empty; an empty setting omits the param and preserves cc-switch's legacy `gpt-5-codex` default. Other platforms unchanged (per scope decision).
- Test debt fixed incidentally so the server unit package compiles/passes: added missing `stubUsageLogRepo` methods `GetSubscriptionProfitRaw` and `GetUserDisplayAggregateGroups` (from the recent subscription work), and refreshed two pre-existing api_contract_test snapshot drifts (`ccs_import_codex_model`, `registration_approval_required`).
- Verified: `go build ./...`, `go test -tags=unit ./internal/handler/dto -run SchemaDoesNotDrift`, `go test -tags=unit ./internal/server -run TestAPIContracts`, frontend `typecheck`, SettingsView.spec (12) and app.spec (22) all green.

## [2026-06-20] feat: redesign API Keys getting-started guide into cards + direct CC Switch downloads

**Affected files**: frontend/src/components/keys/GettingStartedGuide.vue, frontend/src/i18n/locales/zh.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only UI change to the user API Keys page guide; no backend, schema, Wire, gateway, billing, or new i18n keys (reuses existing `keys.guide.*`; only edits zh step-3 wording). Could conflict if the guide component is refactored upstream.
**Change details**:
- Replaced the single inline-pill "Getting Started" bar with a compact header row plus a responsive card grid (sm:grid-cols-3, or sm:grid-cols-2 when CCS is hidden). Each step is now a full card (number badge + icon + title + 2-line clamped description + action), surfacing the previously-unused step descriptions while keeping the height impact on the keys table minimal.
- Moved the "Usage Rules" and dismiss buttons into the header row so they do not consume card-grid height.
- Step 2 now offers separate direct download buttons for Windows (.msi) and macOS (.dmg) instead of a single GitHub releases-page link.
- Download URLs are resolved at runtime from the GitHub Releases API (farion1231/cc-switch) because asset file names embed the version and have no stable "latest" URL. Results are cached in localStorage for 24h to respect GitHub's unauthenticated rate limit, and both buttons fall back to the releases page on any fetch/parse failure so they are never dead links. The fetch is skipped entirely when admin has hidden CCS (`hide_ccs_import_button`).
- Step 3 stays informational (Claude Code / Gemini CLI tool chips) rather than carrying its own action button: a guide-level "use key" button would be ambiguous about which key it opens when the user has several. Instead, aligned the zh wording so the card points users at the table — changed step3 title and the "使用 Key" references in step3Desc/step3DescNoCcs to "使用密钥", matching the per-row table button (`keys.useKey` = 使用密钥). English already used "Use Key", so en is unchanged.
- Verified with `pnpm --dir frontend run typecheck` and `pnpm --dir frontend run lint:check`.

## [2026-06-19] fix: user-facing usage statistics must show display values, not raw

**Affected files**: backend/internal/handler/usage_handler.go, backend/internal/pkg/usagestats/usage_log_types.go, backend/internal/repository/usage_log_repo.go, backend/internal/service/account_usage_service.go, backend/internal/service/usage_service.go, backend/internal/handler/usage_handler_request_type_test.go, backend/internal/handler/usage_handler_display_aggregate_test.go
**Issue**: User-side aggregate stats endpoints summed raw `usage_logs` columns and returned **real** token counts / unit prices, while the per-row usage records list already applied the display-pricing transform (展示单价/展示倍率, the "token 放大机制"). So the dashboard/usage stat cards leaked real tokens and did not reconcile with the records list. Design rule: users must only ever see display values; real tokens/prices are internal.
**Change details**:
- `GET /api/v1/usage/stats` (Stats), `/usage/dashboard/trend` (DashboardTrend), `/usage/dashboard/models` (DashboardModels) now aggregate from the same display-transformed records the user sees (`loadAllDisplayedPublicUsageRecords` + `aggregateDisplayedPublicUsageStats` / `aggregateDisplayedPublicUsageTrend` / new `aggregateDisplayedModelStats`) — exact row-for-row reconciliation with the records list for the selected range.
- `GET /api/v1/usage/dashboard/stats` (DashboardStats) all-time + today token/cost totals now use display values. All-time is unbounded (heaviest user ~247k rows), so it uses per-group SQL aggregation: new repo `GetUserDisplayAggregateGroups` groups by every field the display transform branches on (model, group_id, rate_multiplier, long_context snapshot) and the handler applies the transform once per group and sums (`aggregateDisplayedGroups`). API-key counts, RPM/TPM, and `actual_cost` are unchanged (actual_cost is never altered by the transform).
- New `usagestats.DisplayAggregateGroup` type; new method added to `UsageLogRepository` interface + `UsageService` passthrough.
- `POST /usage/dashboard/api-keys-usage` left as-is — it only returns `actual_cost` (real money the user pays), which the display transform never changes, so it does not leak tokens/prices.
- New unit test `usage_handler_display_aggregate_test.go` proves per-group aggregation reconciles exactly with per-row summation (and preserves real values when no display override exists).
- Verified: `go -C backend build ./...` (exit 0), `go vet` clean, `go test -tags=unit ./internal/handler/... ./internal/service/... ./internal/pkg/usagestats/...` pass. Pre-existing unrelated failure `TestUsageLogRepositoryGetStatsWithFiltersAlwaysReturnsAccountCost` (stale 8-col sqlmock vs 11-col `GetStatsWithFilters`) also fails on unmodified `main` — not caused by this change.
**Known follow-ups (not in this change)**:
- `GET /v1/usage` (API-key dashboard, `GatewayHandler.Usage` → `buildUsageData` + `GetAPIKeyModelStats`) still returns raw tokens, while its siblings `/v1/usage/stats|trend|records` already show display values. Fixing it needs the pricing/display services on `GatewayHandler` (Wire DI) or pushing the display aggregation into the service layer.
- Pricing data finding (config, not code): `global_model_pricing` bills `cache_read` at a flat $2.00/M for `claude-opus-4-8`/`claude-sonnet-4-6`/`gpt-5.4`/`gpt-5.5` while displaying $0.25–0.50/M; for cache-heavy users (cache_read ≈ 90% of tokens) this dominates the bill. Confirmed by the operator as intentional config (not a bug) — left unchanged.

## [2026-06-19] fix: user dashboard cards go stale across midnight + `/v1/usage` raw-token leak

**Affected files**: frontend/src/views/user/DashboardView.vue, backend/internal/handler/gateway_handler.go, backend/internal/handler/usage_handler.go, backend/cmd/server/wire_gen.go, backend/internal/handler/usage_handler_display_aggregate_test.go
**Issue A (stale dashboard)**: A user reported the home dashboard "今日请求/今日消费/今日 Token" cards showing the *previous* day's stats while the balance was correct. Root cause: the balance is refreshed by a global 60s timer in the auth store (`stores/auth.ts` `startAutoRefresh`), but the summary cards were fetched only once in `DashboardView.vue` `onMounted` — no polling, no refetch-on-focus, no day-rollover handling. A tab left open across midnight keeps showing the load-day's "今日". Backend was verified correct (today query returns the right count; no Redis dashboard cache — only `sched:*`/`sticky_session:*` keys).
**Issue B (`/v1/usage` leak)**: The audit of user-facing token surfaces found `GET /v1/usage` and `/antigravity/v1/usage` (`GatewayHandler.Usage` → `buildUsageData` + `GetAPIKeyModelStats`) were the only remaining endpoints returning **raw** token counts, while their siblings `/v1/usage/{stats,trend,records}` already show display values.
**Change details**:
- Frontend: `DashboardView.vue` now silently refetches the summary stats (no full-page spinner) on `visibilitychange`/window `focus` and on a 60s visible-only interval, with listener cleanup in `onBeforeUnmount`. The cards now stay live like the balance and self-correct across midnight within ~60s. The date-range picker still only drives the trend/model widgets (unchanged).
- Backend: `GatewayHandler.Usage` now produces display values. Added `modelPricingService` + `userModelPricingService` to `GatewayHandler` (constructor + `wire_gen.go` hand-edit). `buildUsageData` rewritten to compute today/all-time via per-group display aggregation (`GetUserDisplayAggregateGroups` scoped to the API key); model stats now come from display-transformed records. `actual_cost`, RPM/TPM, avg duration are unchanged.
- Refactor (no behavior change): extracted `loadDisplayedUsageRecords`, `buildDisplayPricingMapForUser`, `loadUserGroupDisplayRates` as free functions and made `aggregateDisplayedGroups` a free function, so both `UsageHandler` (JWT) and `GatewayHandler` (API key) share one display path. `UsageHandler` methods now delegate to them.
- Verified: `go build ./...` (exit 0), `go vet` clean, `go test -tags=unit ./internal/handler/...` pass; frontend `typecheck` + `lint:check` + `build` all pass.

## [2026-06-19] feat(subscription): mixed/bundle subscription — Phase 1 backend MVP

**Affected files**: backend/migrations/168_subscription_plan_member_groups.sql, backend/ent/schema/{subscription_plan,payment_order}.go (+ regenerated ent), backend/internal/service/{payment_config_plans,payment_config_service,subscription_service,payment_order,payment_fulfillment}.go, backend/internal/handler/payment_handler.go, backend/internal/service/payment_config_plans_member_test.go
**Upstream compatibility**: additive, fork-local. New `member_group_ids JSONB NOT NULL DEFAULT '[]'` columns on `subscription_plans` + `payment_orders`; empty = legacy single-group plan/order → identical behavior. No change to the gateway/billing/quota/cache hot path (everything stays keyed by `(user_id, group_id)`). Upstream has no mixed-subscription concept; the new columns/fields are additive and safe across upstream syncs.
**Change details**:
- A subscription plan can now bundle multiple subscription-type groups. Effective member set = `unique(group_id ∪ member_group_ids)`, with `group_id` kept as the primary/representative group (price/sort/display/back-compat).
- One purchase fans out into N independent `user_subscription` rows (one per member group), each with its own quota pool from that group's own `daily/weekly/monthly_limit_usd`. The user switches the API key's group (or uses multiple keys) to access each — chosen "separate quota pools + multi-group switch" model, so each group stays single-platform and the gateway dispatch is untouched.
- `PlanMemberGroupIDs(plan)` (payment_config_plans.go) computes the effective set; `AssignOrExtendSubscriptionToGroups` (subscription_service.go) reuses the existing per-`(user,group)` `AssignOrExtendSubscription` without a wrapping tx (so partial failures commit and resume).
- Order creation snapshots the member set onto `payment_orders` (`createOrderInTx`); `doSub` (payment_fulfillment.go) fans out with per-group idempotency markers `SUBSCRIPTION_SUCCESS:<gid>` (and `SUBSCRIPTION_MEMBER_SKIPPED:<gid>` for a dead non-primary member), writing the suffix-less `SUBSCRIPTION_SUCCESS` only after every member succeeds. Legacy single-group orders short-circuit exactly as before.
- Admin plan Create/Update DTOs accept `member_group_ids` (normalized: drop ≤0, dedup, remove primary, must be existing subscription-type groups, cap 10). Public `GetPlans`/`GetCheckoutInfo` expose `member_group_ids` + `member_groups` (per-member platform/name/limits/scopes).
- Refund intentionally untouched (this deployment has refunds disabled); documented limitation: a future bundle refund would only roll back the primary group.
- Verified: `go generate ./ent`, `go build ./...` (exit 0), `go vet` clean, `go test ./internal/service` (untagged) + `go test -tags=unit ./internal/service/...` all pass.
**Pending (Phase 2/3)**: redeem-code/distribution bundle support + admin assign-by-plan; frontend (admin plan editor multi-select, purchase page member-group display, zh/en i18n).

## [2026-06-19] feat(subscription): mixed/bundle subscription — Phase 3 frontend

**Affected files**: frontend/src/types/payment.ts, frontend/src/views/admin/orders/PlanEditDialog.vue, frontend/src/components/payment/SubscriptionPlanCard.vue, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: additive UI on top of the Phase 1 backend. No behavior change for single-group plans (no member groups selected → renders exactly as before).
**Change details**:
- `types/payment.ts`: added `PlanMemberGroup` interface and `member_group_ids` + `member_groups` on `SubscriptionPlan`.
- Admin `PlanEditDialog.vue`: added a "Bundle groups (additional)" checkbox list of subscription-type groups (excluding the primary), bound to `planForm.member_group_ids`; the primary group is auto-pruned from the member set when it changes; payload now sends `member_group_ids`; edit pre-fills from the plan (admin list returns the raw ent struct).
- Purchase `SubscriptionPlanCard.vue`: when `member_groups.length > 1`, renders an "Included" section listing each member group (platform-colored name + its own daily/weekly/monthly limit) plus a note that each group has an independent quota pool and the user switches the API key group / uses one key per group; single-group plans keep the original quota box via `v-else`.
- i18n: added `payment.planCard.{includedGroups,bundleQuotaNote}` and `payment.admin.{memberGroups,memberGroupsHint}` to both `zh.ts` and `en.ts` base blocks (both files use `mergeLocale(base, patch)` deep-merge; keys added to the base `payment` block).
- Verified: frontend `typecheck` + `lint:check` + `build` all pass.
**Still pending (Phase 2)**: redeem-code/distribution bundle support + admin assign-by-plan; optional admin plans-list bundle badge.

## [2026-06-19] fix: show user-facing Dashboard in admin's "My Account" sidebar section

**Affected files**: frontend/src/components/layout/AppSidebar.vue, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only sidebar navigation tweak; no backend, route-guard, schema, Wire, gateway, or billing changes. Low merge-conflict risk (single line + comment in AppSidebar.vue).
**Change details**:
- Admins (role `admin`) previously had no entry to the user-facing `/dashboard` because `personalNavItems` was built with `buildSelfNavItems(false)`, intentionally dropping the Dashboard item from the admin "My Account" section. The route itself already allowed access (`/dashboard` meta is `requiresAuth: true, requiresAdmin: false`); only the menu entry was missing.
- Flipped `personalNavItems` to `buildSelfNavItems(true)` so the admin "My Account" section now includes the user-side Dashboard link (distinct from `/admin/dashboard` in the admin section).
- Updated the accompanying comment to reflect that Dashboard is now included.

## [2026-06-16] feat: make registration approval configurable

**Affected files**: backend/internal/service/domain_constants.go, backend/internal/service/settings_view.go, backend/internal/service/setting_service.go, backend/internal/service/auth_service.go, backend/internal/service/auth_oauth_email_flow.go, backend/internal/handler/dto/settings.go, backend/internal/handler/admin/setting_handler.go, backend/internal/handler/auth_oauth_pending_flow.go, frontend/src/api/admin/settings.ts, frontend/src/views/admin/SettingsView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/auth.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: additive local settings/auth policy feature; no schema migration, Wire, gateway, billing, pricing, or deployment behavior changes. Existing installs default to requiring approval when the new setting is missing.
**Change details**:
- Added `registration_approval_required` to the Settings KV flow and admin settings API/UI. The default is `true`, preserving the existing pending-approval registration policy.
- Changed email registration, direct OAuth first-login registration, and pending OAuth email-completion account creation to choose initial status from the new setting: `pending_approval` when enabled, `active` when disabled.
- Kept `registration_enabled` as the separate registration-entry gate; it still controls whether new applications/registrations can be submitted at all.
- Delayed token-pair generation for active pending-OAuth email-completion accounts until after identity binding transaction commit, avoiding pre-commit refresh-token issuance.
- Added backend unit coverage for approval-disabled email registration and OAuth email-completion creation, plus frontend SettingsView coverage for saving the new switch.
- Verified with `go test -tags=unit ./internal/service -run 'TestAuthService_Register_(Success|ApprovalDisabledCreatesActiveUserWithToken)|TestRegisterOAuthEmailAccount(ApprovalDisabledCreatesActiveUser|CreatesPendingApprovalUserWithoutTokenPair)'`, `go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin`, `pnpm -C frontend exec vitest run src/views/admin/__tests__/SettingsView.spec.ts`, `pnpm -C frontend run typecheck`, and `git diff --check`.

## [2026-06-15] fix: show all subscriptions in cost-analysis profit view

**Affected files**: backend/internal/pkg/usagestats/usage_log_types.go, backend/internal/repository/usage_log_repo.go, backend/internal/service/dashboard_service.go, frontend/src/api/admin/costAnalysis.ts, frontend/src/views/admin/cost/SubscriptionProfitView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/billing.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: admin analytics/UI fix only; no schema, migration, Wire, gateway, or billing mutation changes. The endpoint response is additive (`source`, `has_paid_order`) and keeps existing cost fields.
**Change details**:
- Changed subscription cost/profit aggregation from paid-order-only to all matching `user_subscriptions`; latest paid subscription orders now only provide revenue/plan attribution. Redeem/admin/default/system subscriptions remain visible with zero revenue and a source tag.
- Constrained usage aggregation to the subscription validity window so usage outside `starts_at`/`expires_at` is not pulled into the page.
- Reworked the detail table to show complete subscription context in fewer columns: user, plan, group, source, revenue, subscription id, usage, cost, cache/full-days, profit, status, and date range.
- Updated zh/en copy and codebase billing docs to document the new visibility and revenue attribution rules.

## [2026-06-15] fix: sort admin users by current concurrency

**Affected files**: backend/internal/handler/admin/user_handler.go, backend/internal/handler/admin/user_handler_activity_test.go, frontend/src/views/admin/UsersView.vue, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: admin UI/API behavior fix only; no schema, migration, Wire, billing, or gateway routing changes. Reuses the existing Redis-backed user concurrency load API already used by the user list response.
**Change details**:
- Changed the admin Users table so clicking the "Concurrency" column requests `sort_by=current_concurrency` instead of sorting by the configured concurrency limit.
- Added a `current_concurrency` virtual sort path in `UserHandler.List`: it fetches the filtered user set, reads current Redis concurrency counts, sorts by current occupancy, then applies the requested page slice before returning the existing paginated response shape.
- Kept normal database-backed user sorts unchanged, including `email`, `balance`, `status`, `last_used_at`, `last_active_at`, and `created_at`.
- Added a unit regression test proving `sort_by=current_concurrency` orders by real-time occupancy while preserving the displayed configured concurrency value.
- Verified with `go test -tags=unit ./internal/handler/admin -run "TestUserHandlerList(SortsByCurrentConcurrency|IncludesActivityFieldsAndSortParams)$" -count=1` from `backend`, and `pnpm --dir frontend run typecheck`.

## [2026-06-14] feat: cache-hit rate card on admin usage page

**Affected files**: backend/internal/pkg/usagestats/usage_log_types.go, backend/internal/repository/usage_log_repo.go, frontend/src/api/admin/usage.ts, frontend/src/components/admin/usage/UsageStatsCards.vue, frontend/src/components/admin/usage/__tests__/UsageStatsCards.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: additive local admin feature; no schema/migration, no Ent regen, no new route, and no Wire changes. Extends the existing `GET /api/v1/admin/usage/stats` aggregation over existing `usage_logs` columns, so it inherits the usage page's full filter set (user/api-key/account/group/model/request-type/billing/date-range).
**Change details**:
- Added a "Cache Hit Rate" summary card to the admin usage page (`UsageStatsCards`), reusing the project's canonical cache formula: read rate = `cache_read / (input + cache_read + cache_creation)`, plus creation rate and per-request hit rate. Identical definition to the dashboard cache-status module (`fillCacheStatusSummary`), so the two views never disagree.
- Extended `UsageStats` (and the `AdminUsageStatsResponse` TS type) with `total_cache_read_tokens`, `total_cache_creation_tokens`, `cache_hit_requests`, `cache_read_rate`, `cache_creation_rate`, `request_hit_rate`. Rates are computed server-side via the existing `cacheStatusRate` helper to keep one source of truth.
- `GetStatsWithFilters` now also aggregates `SUM(cache_read_tokens)`, `SUM(cache_creation_tokens)`, and `COUNT(*) FILTER (WHERE cache_read_tokens > 0)` in the same filtered query; the `Stats` handler serializes the struct unchanged.
- Card tooltip documents the data-quality caveats (Antigravity does not report `cache_creation`; OpenAI/Claude-GPT bridge `cache_read` may be a display-override value), advising group filtering to a single platform for a clean read.
- Added i18n keys `usage.cacheHitTitle/cacheCreationRate/cacheRequestHitRate/cacheHitHint` to both zh.ts and en.ts.
- Verified with `go build ./internal/... ./cmd/...`, `go vet ./internal/repository ./internal/pkg/usagestats`, `pnpm --dir frontend run typecheck`, and `pnpm --dir frontend exec vitest run src/components/admin/usage/__tests__/UsageStatsCards.spec.ts` (2/2 passing).

## [2026-06-14] feat: cost-analysis module — subscription cost/profit stats

**Affected files**: backend/internal/pkg/usagestats/usage_log_types.go, backend/internal/service/account_usage_service.go, backend/internal/repository/usage_log_repo.go, backend/internal/service/dashboard_service.go, backend/internal/handler/admin/dashboard_handler.go, backend/internal/server/routes/admin.go, frontend/src/api/admin/costAnalysis.ts, frontend/src/views/admin/cost/SubscriptionProfitView.vue, frontend/src/components/layout/AppSidebar.vue, frontend/src/router/index.ts, frontend/src/i18n/locales/{zh,en}.ts
**Purpose**: New admin "Cost Analysis" (成本分析) sidebar module; first page = per-subscription cost/profit for monthly / daily-limited users, so the operator can see real margin per subscription/plan.
**Change details**:
- New endpoint `GET /api/v1/admin/dashboard/subscription-profit?start_date&end_date&purchase_price_per_mtok`.
- Repo `GetSubscriptionProfitRaw` aggregates per `subscription_id`: joins user_subscriptions → (LATERAL latest paid subscription payment_order → subscription_plans) → groups → users → usage_logs. INNER JOIN on the paid order excludes redeem-code / admin-granted subscriptions. Filters subscriptions by `starts_at` range; `deleted_at IS NULL`.
- Cost basis: real_cost_rmb = total tokens × purchase price (RMB / million tokens), default 0.25 (= ¥10 / 40M tokens), passed as a query param driven by a UI input persisted in localStorage (no settings/Wire change in v1). Revenue = plan list price. Consumed "$" = SUM(actual_cost). Derived: avg ¥/$, real cost ¥/$, profit multiple, equivalent full-days (consumed$ ÷ daily_limit_usd), cache rate; plus summary + by-plan rollups (loss / <2x counts).
- Frontend: new collapsible nav group 成本分析 (expandOnly) in AppSidebar; routes `/admin/cost-analysis` → redirect → `/admin/cost-analysis/subscriptions`; SubscriptionProfitView (control bar + summary cards + by-plan + detail table, multiple color-coded). Added to simple-mode restrictedPaths. New i18n keys nav.costAnalysis / nav.costSubscriptionProfit and costAnalysis.* in zh + en.
- Verified: `CGO_ENABLED=0 go -C backend build ./...` (exit 0); `pnpm --dir frontend run typecheck` + `lint:check` (both exit 0). Not yet runtime-tested against live data; no DB migration (uses existing columns).

## [2026-06-14] fix: wrap SubscriptionProfitView in AppLayout (sidebar)

**Affected files**: frontend/src/views/admin/cost/SubscriptionProfitView.vue
**Issue**: The cost-analysis page rendered bare content so the left sidebar vanished — admin views must wrap their template in `<AppLayout>` (which renders AppSidebar + AppHeader). Wrapped the page in `<AppLayout>` and imported it. Verified: `typecheck` + `lint:check` exit 0.

## [2026-06-14] feat: cost-analysis subscription view — active-by-default + per-dollar cost mode

**Affected files**: backend/internal/pkg/usagestats/usage_log_types.go, backend/internal/service/{account_usage_service,dashboard_service}.go, backend/internal/repository/usage_log_repo.go, backend/internal/handler/admin/dashboard_handler.go, frontend/src/api/admin/costAnalysis.ts, frontend/src/views/admin/cost/SubscriptionProfitView.vue, frontend/src/i18n/locales/{zh,en}.ts
**Change details**:
- Default now shows **currently-active subscriptions** with no date picking required: `active_only` query param defaults true → repo filters `status='active' AND starts_at <= now() AND expires_at > now()`. Date range is optional (active_only=false → filter by starts_at, history mode).
- Added **cost basis mode**: `cost_mode=per_mtok` (real cost = total tokens × ¥/M, default 0.25) or `per_dollar` (real cost = consumed $ × ¥/$). Endpoint params renamed: `purchase_price` + `cost_mode` (was `purchase_price_per_mtok`). Summary echoes cost_mode + purchase_price. The per_dollar path is the simple form (consumed_usd × rate); finer ¥/$ valuation nuances deferred per user.
- Frontend: "仅当前有效订阅" checkbox (default on, hides date inputs), cost-basis selector with dynamic unit label, localStorage persists price + mode. New i18n keys activeOnly/activeHint/costMode/unitPerMtok/unitPerDollar (zh+en).
- Verified: `go -C backend build ./...`, `pnpm --dir frontend typecheck` + `lint:check` all exit 0.

## [2026-06-13] feat: manual OAuth refresh-token update for accounts

**Affected files**: backend/internal/handler/admin/account_handler.go, backend/internal/server/routes/admin.go, backend/internal/handler/admin/account_handler_refresh_token_test.go, frontend/src/api/admin/accounts.ts, frontend/src/components/admin/account/UpdateRefreshTokenModal.vue, frontend/src/components/admin/account/AccountActionMenu.vue, frontend/src/views/admin/AccountsView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: additive local admin feature; no schema/migration and no billing/gateway routing changes. Reuses the existing per-platform OAuth refresh path and the existing `accounts.credentials` JSONB column.
**Change details**:
- Added `POST /api/v1/admin/accounts/:id/refresh-token` (`AccountHandler.UpdateRefreshToken`) so an admin can paste a new OAuth refresh token when the stored one has expired/revoked — distinct from the existing auto `/:id/refresh` (which reuses the stored token) and from full Re-authorize.
- Default `validate=true` clones the account in memory, injects the pasted refresh token, and reuses `refreshSingleAccount` to exchange it for a fresh access token per platform (Claude/OpenAI/Gemini/Antigravity) before persisting; on success it calls `ClearAccountError` to re-enable a previously errored account. `validate=false` saves the merged credentials without an upstream call (e.g. when the upstream/proxy is temporarily unreachable).
- Credentials are key-merged (not overwritten) so `access_token`/`project_id`/`oauth_type`/`client_id`/`scope` are preserved; the refresh token value is never logged (audit line records operator/account/platform/validated only).
- Frontend: new "Update Refresh Token" row action (oauth accounts only) opening a new `UpdateRefreshTokenModal` with a token textarea, a "validate before saving" toggle, and an optional OpenAI `client_id` field; on success the account row is patched in place via the existing `handleAccountUpdated`. Added paired zh/en i18n keys under `admin.accounts`.
- Verified with `go test -tags=unit ./internal/handler/admin -run TestUpdateRefreshToken -count=1`, `go build ./...`, `pnpm --dir frontend run typecheck`, and `pnpm --dir frontend run lint:check`.

## [2026-06-13] fix: expose Codex auth export in account export dialog

**Affected files**: frontend/src/views/admin/AccountsView.vue, frontend/src/components/admin/account/AccountActionMenu.vue, frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: admin UI discoverability fix only; reuses the existing Codex export API and does not change schema, billing, gateway routing, or the default Sub2API data-bundle export contract.
**Change details**:
- Added an explicit export-format selector to the admin account export dialog so Codex `auth.json` export is discoverable from the top-level Export button instead of only the per-row overflow menu.
- Routed the Codex format option through the existing `exportCodexAuth` API and kept the original Sub2API data-bundle export as the default behavior.
- Kept single-account Codex export in the row action menu and made the visibility check tolerant of legacy OpenAI `official` account type labels while the backend still validates required OAuth token fields before exporting.
- Added a frontend regression test that opens the export dialog and asserts the Codex format option is visible.
- Verified with `pnpm run test:run -- src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`, `pnpm run typecheck`, and `pnpm run lint:check`.

---

## [2026-06-13] feat: export OpenAI OAuth accounts as Codex auth

**Affected files**: backend/internal/handler/admin/account_data.go, backend/internal/handler/admin/account_data_handler_test.go, frontend/src/api/admin/accounts.ts, frontend/src/components/admin/account/AccountActionMenu.vue, frontend/src/views/admin/AccountsView.vue, frontend/src/types/index.ts, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, docs/dev/codebase/account.md
**Upstream compatibility**: additive admin export format and UI action only; no schema, billing, gateway routing, or existing Sub2API export/import JSON contract changes.
**Change details**:
- Added `GET /api/v1/admin/accounts/data?format=codex` to export only complete OpenAI OAuth credentials as Codex `auth.json` compatible payloads with `auth_mode=chatgpt`, `OPENAI_API_KEY=null`, OAuth tokens, account id, and last refresh time.
- Preserved existing account selection/filter/export options, while making `mark_exported=true` for Codex exports mark only accounts that actually enter the Codex payload.
- Added an OpenAI OAuth account-row action that downloads a single Codex auth JSON file, plus Chinese/English i18n and frontend types/API wiring.
- Investigated CC-Switch import support and did not add one-click import: the public `ccswitch://v1/import?resource=provider&app=codex...` path requires API key/endpoint provider input and creates a custom Codex provider, not an OpenAI Official / ChatGPT OAuth account with token-bundle auth.
- Verified with `go test ./internal/handler/admin -run "TestExportData(CodexFormat|IncludesSecrets|WithoutProxies|LimitAndOnlyUnexported|MarkExportedUsesExportedAccounts)" -count=1`, `go test ./internal/handler/admin -run "TestExportData|TestImportData" -count=1`, and `pnpm run typecheck` in `frontend`.

---

## [2026-06-12] feat: require admin approval for self-service account applications

**Affected files**: backend/internal/domain/constants.go, backend/internal/service/auth_service.go, backend/internal/service/auth_oauth_email_flow.go, backend/internal/service/admin_service.go, backend/internal/handler/auth_handler.go, backend/internal/handler/auth_oauth_pending_flow.go, backend/internal/handler/auth_linuxdo_oauth.go, backend/internal/handler/auth_oidc_oauth.go, backend/internal/handler/auth_wechat_oauth.go, backend/internal/handler/admin/user_handler.go, frontend/src/api/auth.ts, frontend/src/stores/auth.ts, frontend/src/utils/authError.ts, frontend/src/views/auth/RegisterView.vue, frontend/src/views/auth/EmailVerifyView.vue, frontend/src/views/auth/LoginView.vue, frontend/src/views/auth/LinuxDoCallbackView.vue, frontend/src/views/auth/OidcCallbackView.vue, frontend/src/views/auth/WechatCallbackView.vue, frontend/src/views/admin/UsersView.vue, frontend/src/components/admin/user/UserEditModal.vue, frontend/src/types/index.ts, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, docs/dev/codebase/auth.md
**Upstream compatibility**: local auth/access-control policy change; no schema migration, billing, gateway routing, pricing, or deployment behavior changes.
**Change details**:
- Added `pending_approval` as a user status and made email/OAuth self-service registration create pending users without issuing access or refresh tokens.
- Blocked pending users from login with `USER_PENDING_APPROVAL`, while preserving existing active-user login behavior.
- Updated LinuxDo, OIDC, WeChat, and pending OAuth account-completion flows to return a pending application response and avoid recording successful login unless a token pair is issued.
- Extended admin user update/filter UI and APIs so administrators can see pending users and approve them by setting status to `active`.
- Updated frontend auth stores, registration/email verification/OAuth callback views, and login error mapping to handle pending application responses without storing auth state.
- Added unit coverage for pending registration, pending login, OAuth pending account creation, and admin approval.
- Verified with `go test -tags=unit ./internal/service`, `go test -tags=unit ./internal/handler`, `pnpm --dir frontend exec vitest run src/stores/__tests__/auth.spec.ts src/views/auth/__tests__/EmailVerifyView.spec.ts`, `pnpm --dir frontend run typecheck`, and `pnpm --dir frontend run build`.

---

## [2026-06-12] improve: one-click OpenAI Claude-GPT bridge mapping template

**Affected files**: frontend/src/composables/useModelWhitelist.ts, frontend/src/components/account/CreateAccountModal.vue, frontend/src/components/account/EditAccountModal.vue, frontend/src/components/account/__tests__/CreateAccountModal.spec.ts, frontend/src/components/account/__tests__/EditAccountModal.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: admin UX improvement only; no backend, schema, billing, or gateway behavior changes.
**Change details**:
- Added a shared OpenAI Claude-GPT bridge mapping template for common Claude requests such as `claude-opus-4-8`, `claude-opus-4-7`, `claude-sonnet-4-6`, and `claude-haiku-4-5` mapped to `gpt-5.5` / `gpt-5.4`.
- Added one-click template buttons next to the OpenAI Claude-GPT bridge toggle in both create and edit account modals.
- Added local-browser editing for the common Claude-GPT bridge template, stored in `localStorage` with a restore-default action.
- Template application switches to model-mapping mode, preserves existing mappings, and only appends missing defaults.
- Added focused Vitest coverage for create/edit payloads and verified the target specs plus ESLint.

---

## [2026-06-12] improve: admin account sorting and test-model ordering

**Affected files**: backend/internal/repository/account_repo.go, backend/internal/repository/account_repo_sort_integration_test.go, frontend/src/views/admin/AccountsView.vue, frontend/src/components/admin/account/AccountTableFilters.vue, frontend/src/components/admin/account/AccountTestModal.vue, frontend/src/components/account/AccountTestModal.vue, frontend/src/components/admin/account/accountModelSort.ts, frontend/src/components/admin/account/__tests__/accountModelSort.spec.ts, frontend/src/components/admin/account/__tests__/AccountTestModal.spec.ts, frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: admin UX improvement only; no schema, billing, gateway, or deployment behavior changes.
**Change details**:
- Added an explicit account-list sort selector for newest/oldest added, platform, type, availability, name, recent use, and priority while preserving server-side pagination.
- Extended account repository ordering to support `platform`, `type`, and computed `availability`, where active, schedulable, non-rate-limited, non-temporarily-unschedulable accounts sort as available.
- Switched the default account-list request ordering to newest-added first for easier account organization.
- Centralized account connection-test model ordering so mainstream/newer models such as Opus 4.8, GPT-5.5, and GPT-5.4 appear first, including compact spellings like `opus48` and `gpt55`.
- Verified with `pnpm -C frontend exec vitest run src/components/admin/account/__tests__/accountModelSort.spec.ts src/components/admin/account/__tests__/AccountTestModal.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`, `go test -tags=integration ./internal/repository -run 'TestAccountRepoSuite/TestListWithFilters_SortBy(TypeAsc|AvailabilityDesc|PriorityDesc)'`, `git diff --check`, and `pnpm -C frontend run typecheck` (currently blocked by unrelated pre-existing auth/register TypeScript errors in `src/api/auth.ts` and `src/stores/auth.ts`).

---

## [2026-06-12] chore(deps): bump axios to 1.17.0 and override js-cookie >=3.0.8

**Affected files**: frontend/package.json, frontend/pnpm-lock.yaml
**Upstream compatibility**: pure dependency bump; the js-cookie pnpm override can be dropped once ahooks/@lobehub pull a patched version.
**Change details**:
- Security Scan's pnpm audit gate flagged 11 high advisories on axios <=1.15.0 (prototype-pollution gadgets, NO_PROXY bypasses, Proxy-Authorization leaks, ReDoS) and 1 on js-cookie 3.0.5 (prototype hijack in assign()). Bumped axios to 1.17.0; js-cookie is transitive (ahooks/@lobehub/ui, js-beautify) so forced >=3.0.8 via pnpm.overrides.
- Frontend typecheck/tests/build re-verified green after the bump. Not part of the v0.1.139 image; rides the next release tag.

---

## [2026-06-12] ci: bump hardcoded Go version checks to 1.26.4

**Affected files**: .github/workflows/backend-ci.yml, .github/workflows/release.yml, .github/workflows/security-scan.yml
**Upstream compatibility**: keep these "Verify Go version" greps in sync with the go.mod `go` directive on every sync that bumps Go.
**Change details**:
- The go.mod bump to 1.26.4 made all four hardcoded `go version | grep -q 'go1.26.2'` verify steps fail (CI, golangci-lint, security scan, release), which blocked the v0.1.139 GHCR image publish. Bumped all four to go1.26.4 — same root cause as the Dockerfile builder image fix.

---

## [2026-06-12] fix(ui): legal consent dialog auto-passes scroll gate when terms do not overflow

**Affected files**: frontend/src/components/auth/LegalConsentDialog.vue, frontend/src/components/auth/__tests__/LegalConsentDialog.spec.ts
**Upstream compatibility**: fork-only feature (legal consent), no upstream overlap.
**Change details**:
- P2 from pre-deploy review: `scrolledToBottom` was only ever set by a scroll event, which never fires when the rendered terms fit inside the dialog (short admin-configured content, tall screens). The accept button then stays permanently disabled — bricking login/registration for all users.
- On dialog open, after render, the gate now auto-passes when `scrollHeight <= clientHeight + 4`. Spec updated to mock overflow dimensions before the gate check; added a no-overflow auto-pass case.

---

## [2026-06-12] fix(billing): per-turn billing request id for multi-turn OpenAI WebSocket connections

**Affected files**: backend/internal/handler/openai_gateway_handler.go, backend/internal/handler/turn_usage_record_context_test.go
**Upstream compatibility**: fork-side fix for a regression introduced by the phase-6b upstream sync (87f2a29c); watch for upstream's own fix when syncing later.
**Change details**:
- P0 found in pre-deploy review: phase 6b made async usage-record tasks inherit the request context, so every turn of an OpenAI WS connection resolved the same billing request id (`client:<connection-uuid>`). Turns 2..N then collided on the `usage_billing_dedup`/`usage_logs (request_id, api_key_id)` keys — tokens were neither billed nor logged (silent revenue loss for Codex WS-mode multi-turn traffic).
- Added `turnUsageRecordContext` which suffixes both `ctxkey.ClientRequestID` and `ctxkey.RequestID` with the per-turn upstream response id (falling back to the turn number) inside the WS `AfterTurn` hook. This covers the forwarder, HTTP-bridge, and passthrough adapter paths, which all share that hook. Unit tests added.

---

## [2026-06-12] fix(billing): normalize usage-log image size to billing tier (migration 156 compatibility)

**Affected files**: backend/internal/service/image_billing_size.go (new, ported from upstream), backend/internal/service/image_billing_size_test.go (new), backend/internal/service/openai_gateway_service.go, backend/internal/service/gateway_service.go
**Upstream compatibility**: partial port of upstream's image billing size classifier; the forward-result audit fields (image_input_size/image_output_size/image_size_source/image_size_breakdown) are still unsynced — finish that on a later sync, then move normalization back to the parse points like upstream.
**Change details**:
- P1 found in pre-deploy review: migration 156 adds CHECK `usage_logs_image_billing_size_check` (image_count > 0 requires image_size IN 1K/2K/4K/mixed), but the fork's OpenAI image paths still write raw request sizes ("1024x1024", "auto", "") — after deploy every OpenAI image-generation usage-log INSERT would violate the constraint: user charged, row silently dropped.
- Ported upstream's pure classifier functions (ClassifyImageBillingTier / NormalizeImageBillingTierOrDefault / ResolveImageBillingSize) and normalized image_size at both usage-log write points (`normalizedImageBillingSizePtr`), covering images/responses/WS-bridge and the Anthropic-side path. Upstream's classifier tests ported as-is.

---

## [2026-06-12] fix(pricing): add claude-fable-5 to checked-in fallback pricing

**Affected files**: backend/resources/model-pricing/model_prices_and_context_window.json
**Upstream compatibility**: additive entry copied verbatim from the live remote pricing cache (backend/data/model_pricing.json); upstream may add it later — dedupe on sync.
**Change details**:
- P2 from pre-deploy review: claude-fable-5 is enabled for routing/billing but missing from the checked-in fallback pricing file. If the remote pricing download fails on a fresh container, billing would fall back to claude-sonnet-4 rates ($3/$15 vs real $10/$50, ~70% undercharge). Added the entry ($10/MTok input, $50/MTok output, cache rates included).

---

## [2026-06-11] fix: bump Dockerfile Go builder to 1.26.4 to match go.mod

**Affected files**: Dockerfile
**Upstream compatibility**: build-only; keep in sync with go.mod `go` directive on future syncs.
**Change details**:
- The upstream sync bumped `backend/go.mod` to `go 1.26.4`, but the Docker builder stayed on `golang:1.26.2-alpine`. Official golang images set `GOTOOLCHAIN=local`, so the production `docker build --no-cache` in update.sh would fail with "go.mod requires go >= 1.26.4". Bumped `GOLANG_IMAGE` to `golang:1.26.4-alpine` (verified the tag exists on Docker Hub). CI is unaffected (uses `go-version-file: backend/go.mod`).

---

## [2026-06-11] test: align four stale test expectations with intentional behavior changes

**Affected files**: backend/ent/schema/auth_identity_schema_test.go, backend/internal/server/api_contract_test.go, backend/internal/service/openai_account_scheduler_test.go, backend/internal/service/openai_ws_v2/passthrough_relay_internal_test.go
**Upstream compatibility**: test-only; no runtime behavior change.
**Change details**:
- `auth_identity_schema_test`: User.signup_source validator now intentionally allows github/google/dingtalk (migrations 152/154); test expected "github" to be rejected. Updated allowed list and use "not-a-source" as the invalid probe.
- `api_contract_test` (admin settings x2): fc9bc4fc added `legal_consent` to GET /api/v1/admin/settings. Set explicit legal consent settings in both subtest setups and added the object to expected JSON (avoids depending on the long default copy).
- `openai_account_scheduler_test` (SchedulerMetrics): the phase-8a sticky guard `openAIStickyAccountMatchesGroup` rejects sticky bindings for accounts not bound to the request group; the new test's account fixture lacked `GroupIDs`, so the sticky hit silently fell through to load-balance. Fixture now binds the account to the group.
- `passthrough_relay_internal_test`: `isTokenEvent` intentionally no longer counts terminal events (`response.completed`/`response.done`) as first-token signals (beb91eef); updated expectation to False.

---

## [2026-06-11] test: fix pre-deploy check failures (build tag + API contract)

**Affected files**: backend/internal/service/announcement_service_test.go, backend/internal/server/api_contract_test.go
**Upstream compatibility**: test-only; no runtime behavior change.
**Change details**:
- Added missing `//go:build unit` tag to `announcement_service_test.go` — it references `userRepoStub` defined in unit-tagged `admin_service_delete_test.go`, so untagged builds (`go vet ./...`, plain `go test ./...`) failed to compile the service package.
- Added `long_context_applied: false` to the `GET /api/v1/usage` expected payload in the API contract test — the field was intentionally added to the usage DTO by the long-context pricing snapshot work (a5bba54f) but the contract expectation was not updated.

---

## [2026-06-11] docs: refresh Claude Code repo guide

**Affected files**: CLAUDE.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: docs-only; no runtime, API, schema, billing, or deployment behavior change.
**Change details**:
- Rewrote the root `CLAUDE.md` to point future Claude sessions at the repository doc chain (`AGENTS.md` -> `docs/dev/ARCHITECTURE.md` -> `docs/dev/codebase/*.md`) instead of duplicating module maps.
- Documented the repo-specific local dev entrypoint via `scripts/dev-stack.ps1`/`.cmd`, the enforced local ports (`18081` backend, `15174` frontend), and the optional `-SkipAIClient` / `-IncludeNewAPI` flags.
- Added the backend, frontend, and root build/test/lint commands that are actually used in this checkout, including package-scoped `go test -run ...` and Vitest single-spec examples.
- Summarized the big-picture architecture that spans multiple files: setup vs normal boot, Wire DI, route-family/protocol dispatch, gateway handler/service split, Settings KV as the runtime config spine, and frontend public-settings injection in both Vite dev and embedded production modes.
- Captured project-specific pitfalls from the current docs and repo state, including the known Wire generation issue, Windows config override path, pnpm-only workflow, and the README reverse-proxy requirement for `underscores_in_headers on;`.

## [2026-06-11] feat: make legal consent terms admin-editable and versioned

**Affected files**: backend/internal/service/domain_constants.go, backend/internal/service/settings_view.go, backend/internal/service/setting_service.go, backend/internal/handler/dto/settings.go, backend/internal/handler/setting_handler.go, backend/internal/handler/admin/setting_handler.go, frontend/src/utils/legalConsent.ts, frontend/src/components/auth/LegalConsentDialog.vue, frontend/src/views/auth/LoginView.vue, frontend/src/views/auth/RegisterView.vue, frontend/src/views/auth/EmailVerifyView.vue, frontend/src/stores/app.ts, frontend/src/stores/auth.ts, frontend/src/views/admin/SettingsView.vue, frontend/src/api/admin/settings.ts, frontend/src/types/index.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, related tests.
**Upstream compatibility**: Settings KV/API/frontend auth-flow extension only; no database migration, gateway routing, billing, pricing, or deployment contract change.
**Change details**:
- Added `legal_consent.*` Settings KV keys for enablement, version, content, confirmation phrase, and minimum read seconds, with the internal-research/non-commercial/no-online-recharge terms as defaults.
- Exposed `legal_consent` through admin settings, public settings, and SSR `window.__APP_CONFIG__` injection so auth pages can use the current configured version before first async refresh.
- Updated registration, login, and email-verification consent flows to resolve dynamic terms settings and store acceptance against the configured version; changing the version now invalidates previous local acceptances.
- Added runtime enforcement after public settings load so already-authenticated users are logged out if their stored acceptance does not match the current legal consent version.
- Added an admin settings editor under Security for enabling/disabling confirmation, editing the version, body, confirmation phrase, and read countdown.
- Verified with `go test -tags=unit ./internal/service -run "TestSettingService_(GetPublicSettings_ExposesLegalConsentSettings|UpdateSettings_LegalConsentSettings)$" -count=1`, `go test -tags=unit ./internal/handler/dto -run TestPublicSettingsInjectionPayload_SchemaDoesNotDrift -count=1`, `go test -tags=unit ./internal/handler ./internal/handler/dto ./internal/handler/admin -count=1`, `pnpm exec vitest run src/utils/__tests__/legalConsent.spec.ts src/components/auth/__tests__/LegalConsentDialog.spec.ts src/stores/__tests__/auth.spec.ts`, `pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts`, `pnpm run typecheck`, and `pnpm build`.
- Broader `go test -tags=unit ./internal/service -count=1` still fails in existing `TestOpenAIGatewayService_OpenAIAccountSchedulerMetrics` (`openai_account_scheduler_test.go:1306`, metric value `0` expected `>= 1`), unrelated to legal consent settings.

## [2026-06-11] test: verify display-token amplification with long-context pricing

**Affected files**: backend/internal/handler/dto/display_pricing_test.go, backend/internal/service/display_token_rewrite_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: test-only coverage for existing display pricing and downstream display-token rewrite behavior; no production logic, schema, API, pricing resource, or deployment change.
**Change details**:
- Added a usage-log DTO regression proving long-context effective display prices and user-group display-rate token amplification compose without extra long-context token amplification.
- Added a downstream display-token rewrite regression proving short-price token amplification ratios remain invariant when both real and display prices are lifted by the GPT long-context multipliers.
- Verified with `go test -tags=unit ./internal/handler/dto -run "LongContext.*Display|ApplyUserDisplayRate"`, `go test -tags=unit ./internal/service -run "DisplayToken_LongContext|DisplayToken_ComputeMultipliers|DisplayToken_ClaudeUsageRewrite"`, `go test -tags=unit ./internal/service -run "Billing|Pricing|LongContext|DisplayToken|UserModelPricing|GlobalModelPricing"`, `go test -tags=unit ./internal/handler -run "Usage|Display|LongContext|Pricing"`, and `git diff --check`.

## [2026-06-11] copy: position legal terms as internal research use

**Affected files**: frontend/src/utils/legalConsent.ts, frontend/src/components/auth/LegalConsentDialog.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, frontend/src/utils/__tests__/legalConsent.spec.ts, frontend/src/components/auth/__tests__/LegalConsentDialog.spec.ts, frontend/src/stores/__tests__/auth.spec.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only legal-consent copy and version update; no backend schema, API, gateway, billing, or deployment contract change.
**Change details**:
- Reframed the legal dialog as "Use Terms and Disclaimer" for an internal research/testing platform instead of public service terms.
- Added explicit copy that the platform is non-commercial, does not provide online recharge, does not accept external customers, and is limited to authorized internal technical testing.
- Updated prohibited conduct and enforcement wording to cover public operation, API resale, top-up/resale/distribution, external integrations, platform information disclosure, abuse, scraping, and pressure attacks.
- Bumped the legal consent version to `2026-06-11-internal-research-v2` and changed stored consent validation to require the new internal-authorized-use attestation and exact confirmation phrase.
- Added validation for pending registration consent so stale pre-upgrade session payloads cannot bypass the new confirmation text.
- Verified with `pnpm exec vitest run src/utils/__tests__/legalConsent.spec.ts src/components/auth/__tests__/LegalConsentDialog.spec.ts src/stores/__tests__/auth.spec.ts`, `pnpm run typecheck`, and `pnpm build`.

## [2026-06-11] feat: require legal consent on registration and login

**Affected files**: frontend/src/components/auth/LegalConsentDialog.vue, frontend/src/utils/legalConsent.ts, frontend/src/views/auth/RegisterView.vue, frontend/src/views/auth/LoginView.vue, frontend/src/views/auth/EmailVerifyView.vue, frontend/src/stores/auth.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, frontend/src/components/auth/__tests__/LegalConsentDialog.spec.ts, frontend/src/utils/__tests__/legalConsent.spec.ts, frontend/src/stores/__tests__/auth.spec.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only legal-consent gate; no backend schema, gateway, billing, or API contract change. Existing registered users are forced out of locally persisted frontend auth once per current legal-consent version when the new app build loads.
**Change details**:
- Added a reusable legal consent dialog for registration and post-login flows with a read-time countdown, required scroll-to-bottom, explicit terms and region attestations, and an exact typed confirmation phrase.
- Added local per-user/per-version consent persistence and a pending registration consent handoff so email-verification registration records acceptance after the user is created.
- Updated login and 2FA success paths to require current-version consent before redirecting to the dashboard; users who already accepted the current version are not prompted again.
- Updated auth restoration so locally persisted sessions without a current legal-consent record are cleared instead of bypassing the login confirmation flow.
- Added Chinese and English legal-consent copy covering region restrictions, disclaimer, prohibited conduct, enforcement, account/API key security, and service availability risk.
- Verified with `pnpm exec vitest run src/stores/__tests__/auth.spec.ts src/utils/__tests__/legalConsent.spec.ts src/components/auth/__tests__/LegalConsentDialog.spec.ts src/views/auth/__tests__/EmailVerifyView.spec.ts`, `pnpm run typecheck`, `pnpm run test:run`, `pnpm build`, and HTTP 200 checks for `/register` and `/login` on the local frontend dev server.

## [2026-06-10] upstream-sync: add Claude Fable 5 support

**Affected files**: backend/internal/domain/constants.go, backend/internal/domain/constants_test.go, backend/internal/pkg/antigravity/claude_types.go, backend/internal/pkg/antigravity/claude_types_test.go, backend/internal/pkg/antigravity/request_transformer.go, backend/internal/pkg/claude/constants.go, backend/internal/service/antigravity_model_mapping_test.go, backend/internal/service/bedrock_request.go, backend/internal/service/bedrock_request_test.go, frontend/src/components/account/AccountStatusIndicator.vue, frontend/src/components/account/AccountUsageCell.vue, frontend/src/components/keys/UseKeyModal.vue, frontend/src/components/keys/__tests__/UseKeyModal.spec.ts, frontend/src/composables/__tests__/useModelWhitelist.spec.ts, frontend/src/composables/useModelWhitelist.ts
**Upstream compatibility**: cherry-picked upstream `d662c97302586edfd711a4a2b3a19fe2a95aa1e1` as local commit `170b4972`; conflict resolution retained the current branch's existing Opus 4.8 and Bedrock baseline while applying the Claude Fable 5 model, mapping, whitelist, and focused Bedrock ID/cache-control support. No database migration, pricing resource, or deployment change.
**Change details**:
- Added `claude-fable-5` to Claude, Antigravity, and Bedrock default model mappings, model lists, UI whitelist presets, account usage/status labels, and generated OpenCode config.
- Added focused regression coverage for Claude/Antigravity model exposure, default mapping passthrough, Bedrock model ID resolution, and frontend whitelist/OpenCode config rendering.
- Verified with `go test -tags=unit ./internal/domain ./internal/pkg/antigravity ./internal/service -run "TestDefault|TestAntigravity|TestIsBedrockClaude45OrNewer|TestResolveBedrockModelID" -count=1`, `pnpm --dir frontend test:run src/composables/__tests__/useModelWhitelist.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts`, and `git diff --check`.

## [2026-06-10] fix: normalize OpenAI Responses account-test URLs

**Affected files**: backend/internal/service/account_test_service.go, backend/internal/service/openai_apikey_responses_probe.go, backend/internal/service/account_test_service_openai_test.go, docs/dev/codebase/account.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: OpenAI API-key account-test and capability-probe URL normalization only; no schema, frontend, billing, scheduling, or gateway contract changes.
**Change details**:
- Reused the shared OpenAI endpoint URL builder for API-key Responses account tests so root base URLs now call `/v1/responses` instead of `/responses`.
- Reused the same builder in the automatic API-key Responses capability probe so `openai_responses_supported` is learned from the real Responses endpoint.
- Added regression coverage for root base URLs in both the direct admin account-test path and the capability-probe path.
- Verified with `go test -tags=unit ./internal/service -run "TestAccountTestService_OpenAI" -count=1`, `git diff --check`, and a real local admin test request against account `2988` returning HTTP 200 plus `test_complete success=true`.

## [2026-06-10] fix: tolerate compatible OpenAI account-test streams

**Affected files**: backend/internal/service/account_test_service.go, backend/internal/service/account_test_service_openai_test.go, docs/dev/codebase/account.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: account-test parser hardening only; no API contract, account schema, billing, scheduling, or frontend behavior changes.
**Change details**:
- Relaxed OpenAI account-test stream completion for compatibility providers: once a Responses or Chat Completions test stream emits valid content, EOF or `[DONE]` is accepted as a successful connectivity probe instead of requiring `response.completed`.
- Added tolerance for Chat Completions chunks returned through the Responses test parser, mapping `delta.content` and `delta.reasoning_content` into existing account-test content events.
- Preserved failure behavior for empty OpenAI streams that end before any content, completion marker, or terminal chat chunk.
- Handled final SSE lines without a trailing newline so the last content chunk or `[DONE]` marker is not discarded at EOF.
- Added regression coverage for empty stream failure, Responses content plus EOF, Responses content plus `[DONE]`, Chat Completions chunks through the Responses parser, and raw Chat Completions content plus EOF.
- Verified with `go test -tags=unit ./internal/service -run "TestAccountTestService_OpenAI(EmptyStreamBeforeCompletedFails|ResponsesPathAccepts|ChatCompletionsPathAccepts|APIKeyForceChatCompletions)" -count=1`, `go test -tags=unit ./internal/service -run "TestAccountTestService_OpenAI" -count=1`, and `git diff --check`.

## [2026-06-10] fix: align OpenAI account test with raw chat-compatible upstreams

**Affected files**: backend/internal/service/account_test_service.go, backend/internal/service/account_test_service_openai_test.go, docs/dev/codebase/account.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: backend account-test behavior now follows the existing OpenAI API-key gateway capability flag for third-party OpenAI-compatible upstreams; no schema, account credential, billing, scheduling, or frontend contract changes.
**Change details**:
- Changed OpenAI API-key account connection tests to use `/v1/chat/completions` when `openai_compat.ShouldUseResponsesAPI(account.extra)` is false, matching the real gateway path used for DeepSeek/Kimi/GLM/Qwen-style upstreams.
- Added a Chat Completions test payload and SSE parser for admin account tests, mapping `delta.content` and `delta.reasoning_content` chunks into the existing test UI content events.
- Preserved the existing `/v1/responses` account-test path for OpenAI OAuth accounts and API-key accounts that support Responses.
- Added a regression test proving `force_chat_completions` accounts no longer fail before contacting upstream and send the expected `/v1/chat/completions` request.
- Verified with `go test -tags=unit ./internal/service -run TestAccountTestService_OpenAIAPIKeyForceChatCompletionsUsesRawChatTestPath -count=1`, `go test -tags=unit ./internal/service -run "TestAccountTestService_OpenAI" -count=1`, and `git diff --check`.

## [2026-06-10] fix: batch account deletion after cross-page selection

**Affected files**: frontend/src/views/admin/AccountsView.vue, frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts, docs/dev/codebase/account.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only account-management deletion hardening; reuses the existing single-account delete API and does not change backend contracts, scheduling, billing, or account data shape.
**Change details**:
- Changed selected-account bulk deletion to snapshot selected IDs and delete them through the existing bounded batched helper instead of firing one unbounded `Promise.all` over every selected account.
- Keeps successfully deleted accounts removed from selection while retaining failed IDs selected so admins can retry only the failed deletions.
- Reused the same 10-account batch behavior as exported-account cleanup, preventing cross-page selections from overwhelming the browser/backend or aborting the UI flow after the first failed delete.
- Added an AccountsView regression test that selects 12 accounts across filtered results, verifies deletion starts in a 10-request batch, continues to the second batch after a failure, and leaves only the failed ID selected.
- Verified with `pnpm --dir frontend test:run src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`.

## [2026-06-09] feat: account export count and exported-state options

**Affected files**: backend/internal/handler/admin/account_data.go, backend/internal/service/account.go, backend/internal/service/admin_service.go, backend/internal/handler/admin/account_data_handler_test.go, backend/internal/handler/admin/admin_service_stub_test.go, frontend/src/views/admin/AccountsView.vue, frontend/src/api/admin/accounts.ts, frontend/src/types/index.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/account.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: additive account export query parameters and account `extra.exported_at` metadata; existing export/import JSON format remains compatible and no database migration is required.
**Change details**:
- Added account export options for a maximum account count, exporting only accounts without `extra.exported_at`, and marking exported accounts by writing `extra.exported_at` after a successful export.
- Fixed export count parsing so number inputs cannot trigger a runtime `.trim is not a function` error.
- Added a destructive toolbar action to delete accounts with `extra.exported_at` under the current account filters, using batched existing delete calls after confirmation.
- Preserved selected-account export precedence while applying the new count and unexported filters to selected or filtered export flows.
- Added an optional hidden "Exported At" account-table column and Chinese/English UI text for the new export controls.
- Added focused backend handler tests for count-limited unexported export and post-export marking.

## [2026-06-09] fix: snapshot long-context billing for display pricing

**Affected files**: backend/internal/service/billing_service.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/gateway_service.go, backend/internal/service/usage_log.go, backend/internal/repository/usage_log_repo.go, backend/ent/schema/usage_log.go, backend/migrations/167_usage_log_long_context_snapshot.sql, backend/internal/handler/dto/display_pricing.go, backend/internal/handler/dto/mappers.go, backend/internal/handler/dto/types.go, frontend/src/types/index.ts, docs/dev/codebase/billing.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: additive usage log fields and DTO response fields; no request parameter changes and no pricing-page UI changes.
**Change details**:
- Added usage-log long-context snapshot fields for whether long-context pricing applied, the threshold, and the input/output multipliers used by the request.
- Propagated the snapshot from token cost calculation through OpenAI/standard gateway usage recording and repository insert/select paths.
- Adjusted user/admin display DTO mapping to copy display pricing config and apply the snapshot as an effective per-request display price before the existing display transform.
- Added unit coverage for long-context threshold boundaries, channel interval exclusion, repository persistence/scan compatibility, display-token behavior, and a fake-upstream OpenAI Responses HTTP flow.

## [2026-06-09] fix: support cross-page account selection

**Affected files**: frontend/src/views/admin/AccountsView.vue, frontend/src/components/admin/account/AccountBulkActionsBar.vue, frontend/src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only account-management selection fix; reuses the existing admin account list API and does not change backend contracts, account scheduling, billing, or account mutations.
**Change details**:
- Added a "select all filtered" action to the account bulk-actions bar so admins can select account IDs across paginated results.
- Fetches account IDs in 1000-row pages using the current filter and sort snapshot, then writes the deduplicated IDs into the existing table-selection state.
- Caches selected account platform/type metadata from visible and fetched rows so bulk-edit option gating remains correct after cross-page selection.
- Added focused AccountsView coverage for selecting IDs from multiple filtered pages.

## [2026-06-09] feat: expose distribution wallet refund totals

**Affected files**: backend/internal/service/distribution.go, backend/internal/repository/distribution_repo.go, frontend/src/types/index.ts, frontend/src/views/user/DistributionView.vue, frontend/src/views/admin/DistributionView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: additive API response field for distribution wallets; no schema or billing behavior changes.
**Change details**:
- Added a derived `total_refunded` wallet field based on all positive `asset_refund` ledger entries.
- Displayed cumulative refunds on the approved-agent page and in the admin distribution agent accounts table.
- Keeps the visible reconciliation relationship complete: total recharged equals balance plus gross spend minus refunded amount.

## [2026-06-09] feat: show customer usage lookup link in agent center

**Affected files**: frontend/src/views/user/DistributionView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only agent center copy/link enhancement; uses the existing public `/key-usage` route and does not change API key auth, usage storage, or billing behavior.
**Change details**:
- Added customer usage lookup guidance inside the approved-agent tutorial, with the fully joined lookup URL based on the current site origin plus `/key-usage`.
- Included the same customer usage lookup URL in generated API key delivery text so agents can send customers to the public usage page.
- Added Chinese and English labels for the customer usage lookup guidance.

## [2026-06-09] fix: align public API key usage display totals

**Affected files**: backend/internal/handler/usage_handler.go, backend/internal/handler/usage_handler_public_alignment_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped to API-key-authenticated public usage endpoints used by `/key-usage`; does not change usage storage, billing deduction, admin dashboards, or authenticated user dashboard endpoints.
**Change details**:
- Changed public `/v1/usage/stats` and `/v1/usage/trend` to aggregate from the same display-transformed records used by `/v1/usage/records`, including user model display pricing and user group display rate multipliers.
- Batched the public stats/trend source query at 1000 rows per page so totals cover the full selected date range instead of the visible table page.
- Added handler tests asserting records, stats, and trend totals match for actual cost, display cost, and display-transformed token counts.

## [2026-06-09] fix: sync Phase 8C usage window, ops metric, and select UI

**Affected files**: backend/internal/repository/account_repo.go, backend/internal/service/account_service.go, backend/internal/service/account_usage_service.go, backend/internal/service/account_usage_session_window_test.go, backend/internal/service/ops_alert_evaluator_service.go, backend/internal/service/ops_alert_evaluator_service_test.go, backend/internal/handler/admin/ops_alerts_handler.go, backend/internal/server/api_contract_test.go, backend/internal/service/*_test.go, frontend/src/components/account/UsageProgressBar.vue, frontend/src/components/account/__tests__/UsageProgressBar.spec.ts, frontend/src/components/common/Select.vue, frontend/src/api/admin/ops.ts, frontend/src/views/admin/ops/components/OpsAlertRulesCard.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/account.md, docs/dev/codebase/ops.md, docs/dev/codebase/README.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped Phase 8C sync from `upstream/main@be017445`, covering upstream `16bc8769`, `f20e6bf7`, and `f5cecea5`; does not change billing, account scheduling selection policy, payment, auth, or OpenAI bridge behavior.
**Change details**:
- Synced active 5h usage `ResetsAt` back into `accounts.session_window_end` and zeroed expired setup-token 5h windows before rendering.
- Added `account_temp_unscheduled_count` as a backend/frontend Ops alert metric for accounts currently inside a temporary unschedulable window.
- Replaced hard-coded UsageProgressBar reset text with i18n keys and distinguished stale positive-utilization windows as pending refresh.
- Increased common Select dropdown option area from `max-h-60` to `max-h-80` so 7+ item status filters are not visually hidden.

## [2026-06-09] fix: sync Phase 8B OpenAI transport and response header guards

**Affected files**: backend/internal/service/openai_upstream_transport_error.go, backend/internal/service/openai_upstream_transport_error_test.go, backend/internal/service/openai_upstream_transport_error_handle_test.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_responses_chat_fallback.go, backend/internal/service/openai_gateway_chat_completions.go, backend/internal/service/openai_gateway_chat_completions_test.go, backend/internal/service/openai_gateway_service_test.go, backend/internal/service/gateway_forward_as_chat_completions.go, backend/internal/service/gateway_forward_as_chat_completions_test.go, backend/internal/service/gateway_forward_as_responses.go, backend/internal/service/gateway_forward_as_responses_test.go, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped Phase 8B sync from `upstream/main@be017445`, covering upstream `217f8599`, `d251487d`, and `154e0ed6`; preserves local Claude-GPT bridge, OpenAI image endpoint, Codex image bridge, display-pricing/model-mapping, and Phase 8A group isolation behavior.
**Change details**:
- Converted OpenAI transport-layer failures without HTTP status codes into failover errors, while temporarily unscheduling accounts for persistent proxy/DNS/routing faults.
- Added API-key Chat Completions -> Responses `prompt_cache_key` body propagation and kept `session_id` isolated by API key.
- Forced non-streaming buffered JSON responses to `application/json` after upstream SSE headers are filtered through, preventing downstream stream misclassification.
- Added unit coverage for transport error classification/handling, API-key prompt-cache propagation, and JSON Content-Type correction on buffered responses.

## [2026-06-09] fix: sync Phase 8A API key group and OpenAI sticky guards

**Affected files**: backend/internal/repository/api_key_repo.go, backend/internal/server/middleware/api_key_auth.go, backend/internal/server/middleware/api_key_auth_test.go, backend/internal/service/admin_service.go, backend/internal/service/api_key_auth_cache.go, backend/internal/service/api_key_auth_cache_impl.go, backend/internal/service/api_key_service_cache_test.go, backend/internal/service/openai_account_scheduler.go, backend/internal/service/openai_account_scheduler_test.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_ws_state_store.go, backend/internal/service/channel_service.go, backend/internal/service/channel_service_test.go, backend/internal/handler/openai_gateway_handler.go, docs/dev/codebase/account.md, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped Phase 8A sync from `upstream/main@be017445`, covering upstream `1a86c6ce`, `a4362963`, `87dd5f5d`, and `9a0e4398`; does not merge proxy fallback, quota, risk-control, payment, DingTalk, or account-page re-layout changes.
**Change details**:
- Revalidated API key exclusive-group access at request time by loading user `allowed_groups` and group `is_exclusive` into the auth path and auth cache.
- Invalidated API key auth cache when admin user `allowed_groups` changes, so removed exclusive-group access does not survive until cache TTL expiry.
- Added OpenAI sticky-session group checks so stale session-bound accounts outside the current group are cleared before selection continues.
- Namespaced local OpenAI response-id account bindings by group and stripped mismatched WSv2 first-packet `previous_response_id` when the current group did not hit the sticky previous-response binding.
- Added focused unit coverage for exclusive-group API key rejection, auth-cache round trip fields, sticky group clearing, and `previous_response_id` body stripping.

## [2026-06-09] feat: show API key balance on public usage page

**Affected files**: frontend/src/views/KeyUsageView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only enhancement for the local public `/key-usage` page; reuses the existing API-key-authenticated `/v1/usage` summary endpoint without changing gateway authentication, billing, usage storage, or backend routes.
**Change details**:
- Added an available balance/quota card to `/key-usage` so customers can see the queried API key's wallet balance, fixed key quota remaining, subscription remaining quota, or rate-limit window remaining from the existing `/v1/usage` response.
- Kept records, stats, and trend queries unchanged; the balance summary is loaded alongside `/v1/usage/records`, `/v1/usage/stats`, and `/v1/usage/trend` using the same browser-local Bearer API key.
- Added Chinese and English labels for the new balance states and details.

## [2026-06-07] feat: sync Phase 7 upstream model sync

**Affected files**: backend/internal/service/upstream_models.go, backend/internal/service/upstream_models_test.go, backend/internal/handler/admin/account_handler.go, backend/internal/handler/admin/account_handler_available_models_test.go, backend/internal/handler/admin/admin_service_stub_test.go, backend/internal/server/routes/admin.go, frontend/src/api/admin/accounts.ts, frontend/src/components/account/CreateAccountModal.vue, frontend/src/components/account/EditAccountModal.vue, frontend/src/components/account/ModelWhitelistSelector.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/account.md, docs/dev/codebase/README.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped Phase 7 sync from `upstream/main@f868f7cb`; adds admin-only model-list sync without changing billing, authentication, payment, account scheduling, display pricing, model mapping resolution, Claude-GPT bridge, OpenAI image endpoint scheduling, or Codex image bridge behavior.
**Change details**:
- Added upstream model-list fetching for saved accounts and create-flow preview credentials, including OpenAI API key, Anthropic OAuth/API key, Gemini API key/OAuth where supported, Antigravity OAuth, and compatible Antigravity API-key base URLs.
- Added admin APIs `POST /api/v1/admin/accounts/:id/models/sync-upstream` and `POST /api/v1/admin/accounts/models/sync-upstream-preview`.
- Added frontend sync controls to account whitelist editors and Antigravity mapping editors; sync results only append missing models or mappings and never remove or replace local entries.
- Kept preview sync in memory only: it reads form `platform`, `type`, `base_url`, and `api_key` and does not create or update accounts.

## [2026-06-07] feat: sync Phase 7 channel monitor OpenAI API mode

**Affected files**: backend/internal/handler/admin/channel_monitor_handler.go, backend/internal/handler/admin/channel_monitor_template_handler.go, backend/internal/service/channel_monitor_*.go, backend/internal/repository/channel_monitor_*.go, frontend/src/api/admin/channelMonitor.ts, frontend/src/api/admin/channelMonitorTemplate.ts, frontend/src/constants/channelMonitor.ts, frontend/src/components/admin/monitor/Monitor*.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/channel-monitor.md, docs/dev/codebase/README.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped Phase 7 sync from `upstream/main@f868f7cb`; keeps historical and empty `api_mode` as `chat_completions`, only lets OpenAI monitors/templates opt into `responses`, and does not change billing, authentication, payment, or account scheduling paths.
**Change details**:
- Added OpenAI `api_mode` to Channel Monitor create/update/list responses, repository Ent mapping, scheduled check options, and frontend API types/UI.
- Added protocol-aware OpenAI checks: `chat_completions` keeps `/v1/chat/completions`; `responses` uses `/v1/responses` with `instructions`, `input`, and `max_output_tokens`, parsing `output_text` first and nested output content as fallback.
- Scoped request templates by provider and `api_mode`; template application now filters matching monitors by both provider and `api_mode` so Chat and Responses request bodies are not mixed.
- Added Chinese/English UI labels and codebase documentation for the monitor flow.

## [2026-06-07] docs: sync Phase 7 Sub2API admin skill

**Affected files**: skills/sub2api-admin/SKILL.md, skills/sub2api-admin/agents/openai.yaml, skills/sub2api-admin/references/admin-cli.md, skills/sub2api-admin/scripts/sub2api-admin.js, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped Phase 7 sync from `upstream/main@f868f7cb`; documentation/tooling only, with no runtime, schema, API, deployment, or global Codex skill installation changes.
**Change details**:
- Added the upstream `sub2api-admin` repository skill and bundled admin CLI reference/script for AI-assisted Sub2API admin API operations.
- Kept the skill as repo-local documentation/tooling; it is not wired into backend/frontend runtime and does not install into the workstation global skill registry.
- Preserved the upstream safety notes around admin API keys and account exports so credentials are not printed in chat or logs.

## [2026-06-07] fix: sync Phase 6A OpenAI error and stream terminal fixes

**Affected files**: backend/internal/handler/openai_gateway_handler.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_chat_completions_raw.go, backend/internal/service/openai_silent_refusal.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 6A scoped sync from `upstream/main@635ad81c`; covers OpenAI/Codex error and stream terminal correctness only, without changing pricing, display token, distribution, public `/key-usage`, Claude-GPT bridge routing, or account page UI.
**Change details**:
- Added API-key non-streaming Responses fallback when an upstream returns SSE in a body with the wrong content type, matching the existing OAuth heuristic without masking valid JSON.
- Normalized streamed Responses terminal events so `response.completed`/`response.done` with empty or null `response.output` gets reconstructed from accumulated text/tool/image deltas before reaching clients.
- Added the upstream OpenAI silent-refusal detector and connected it to the raw Chat Completions streaming path so large empty stop-without-usage streams can fail over before any downstream output is written.
- Preserved upstream `response.failed`/protocol errors already written to the client, and mapped exhausted silent-refusal failover to a clear upstream-error message.
- Verified with `go test -tags=unit ./internal/service -run "OpenAI.*SSE|OpenAI.*Stream|SilentRefusal|ChatCompletions|Responses|Images|GatewayService"`, `go test -tags=unit ./internal/handler -run "OpenAI|Stream|Failed|Images|Gateway"`, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-07] fix: sync Phase 6B OpenAI usage context and response-id binding

**Affected files**: backend/internal/handler/gateway_handler.go, backend/internal/handler/gateway_handler_chat_completions.go, backend/internal/handler/gateway_handler_responses.go, backend/internal/handler/gemini_v1beta_handler.go, backend/internal/handler/openai_chat_completions.go, backend/internal/handler/openai_embeddings.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/handler/openai_images.go, backend/internal/server/middleware/client_request_id.go, backend/internal/service/openai_gateway_chat_completions.go, backend/internal/service/openai_gateway_service.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 6B scoped sync from `upstream/main@635ad81c`; covers OpenAI usage preservation, request correlation context, and HTTP response-id account binding only. Pricing defaults, global/user model pricing, display pricing, distribution, public `/key-usage`, Claude-GPT bridge routing, and image trace safety remain unchanged.
**Change details**:
- Usage-record worker tasks now copy `client_request_id` and request id from the original request context into the detached recording context, so async usage rows keep request correlation after Gin request cancellation.
- The client request id middleware now echoes `X-Client-Request-ID` for existing or generated ids while keeping the logger context behavior unchanged.
- OpenAI Responses, passthrough, SSE-to-JSON fallback, and Chat Completions compatibility paths now retain the upstream response id in `OpenAIForwardResult`.
- HTTP Responses/Chat paths bind the upstream response id to the selected account through the existing OpenAI WS sticky state store, allowing later `previous_response_id` continuations to reuse the same account without adding schema or pricing changes.
- Chat Completions streaming conversion always requests/emits a usage chunk for gateway billing completeness, while display-token rewriting stays downstream-only and real usage remains unmodified.
- Verified with `go test -tags=unit ./internal/handler -run "UsageRecord|OpenAI|Gateway"`, `go test -tags=unit ./internal/service -run "OpenAI|ResponseID|Usage|ChatCompletions"`, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-07] fix: sync Phase 6C OpenAI websocket failover

**Affected files**: backend/internal/handler/openai_gateway_handler.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 6C scoped sync from `upstream/main@635ad81c`; the remaining local delta was OpenAI Responses WebSocket account failover after upstream WS rate-limit errors. Other Phase 6C WS fixes for tool-output continuation, terminal-event timing, usage parsing/deduplication, model fallback, and Codex image bridge injection were already present from earlier Phase 3/4/6B syncs.
**Change details**:
- Wrapped OpenAI `/v1/responses` WebSocket ingress forwarding in the same failover pattern used by local OpenAI HTTP handlers: failed account IDs are excluded, account switch metrics are recorded, and the next schedulable OpenAI account is selected when the service returns an `UpstreamFailoverError`.
- Reacquires the user concurrency slot before retrying a WS upstream after a failed turn, while releasing the failed account slot immediately to avoid leaking account concurrency.
- Added a WS-specific failover-exhausted close mapper so 429 and transient upstream failures close the client socket with retryable WebSocket status/reason instead of a generic internal error.
- Kept endpoint-capability scheduling, local account image endpoint switch, Codex image bridge injection, Claude-GPT bridge routing, display-token usage semantics, and pricing untouched.
- Verified with `go test -tags=unit ./internal/service -run "OpenAIWS|WebSocket|HTTPBridge|RateLimit|ResponseID|Usage|CodexImage|ToolContinuation"`, `go test -tags=unit ./internal/handler -run "OpenAI.*WebSocket|OpenAIMessages|ClaudeGPTBridge|Endpoint|Images"`, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go test -tags=unit ./internal/service ./internal/handler -run "OpenAI|Codex|Responses|Chat|Messages|WS|Usage|OAuth|Image|Bridge"`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-07] fix: sync Phase 6D/6E OpenAI request hotpath and apicompat audit

**Affected files**: backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_service_hotpath_test.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/handler/gateway_helper.go, backend/internal/handler/gateway_helper_hotpath_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 6D scoped sync from `upstream/main@635ad81c` for OpenAI request body retention/OOM hardening. Phase 6E apicompat bridge redesign, reasoning-only/DeepSeek handling, and tool pairing were audited and already match the target upstream package, so no duplicate apicompat edits were made.
**Change details**:
- Bound the OpenAI parsed-request cache to the exact request body hash/length so failover or retry paths cannot reuse a mutable map decoded from a previous upstream attempt.
- Added safe cache helpers for handler pre-validation and Claude Code client detection, replacing direct raw map storage in Gin context while preserving backward-compatible reads for lightweight detection.
- Released the parsed-request cache before OpenAI upstream failover and after successful HTTP response handling, reducing large request body/map retention across streaming response processing.
- Switched OpenAI request reserialization and empty-base64-image cleanup to the upstream non-HTML-escaping JSON encoder helper, preserving request content while avoiding extra escaping churn.
- Extracted reasoning effort and service tier for usage records from the final request body bytes instead of retaining the full decoded request map solely for those scalar fields.
- Confirmed Phase 6E apicompat code has no local diff against `upstream/main@635ad81c`; focused tests cover Responses <-> Chat Completions lifecycle, DeepSeek/reasoning-only streams, and Responses-to-Anthropic tool pairing.
- Kept pricing defaults, global/user model pricing, display pricing/display token, Claude-GPT bridge overlay, distribution, public `/key-usage`, image trace safety, and account scheduling controls untouched.
- Verified with `go test -tags=unit ./internal/service -run "OpenAI.*Hotpath|GetOpenAIRequestBodyMap|ExtractOpenAI|SanitizeEmptyBase64|Forward|ResponseID|Usage"`, `go test -tags=unit ./internal/handler -run "SetClaudeCodeClientContext|OpenAI|FunctionCallOutput"`, `go test -tags=unit ./internal/pkg/apicompat -run "ChatCompletions|Responses|DeepSeek|Reasoning|Tool|Pairing|Lifecycle|Invariants"`, `go test -tags=unit ./internal/service -run "ResponsesChatFallback|ForwardAsAnthropic|ChatCompletions|DeepSeek|Tool|Pairing|CodexTransform"`, `go run ./tools/upstream-sync-guard`, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go test -tags=unit ./internal/service ./internal/handler -run "OpenAI|Codex|Responses|Chat|Messages|WS|Usage|OAuth|Image|Bridge"`, and `git diff --check`.

## [2026-06-07] fix: sync Phase 6F OpenAI OAuth runtime fixes

**Affected files**: backend/internal/service/ratelimit_service.go, backend/internal/service/ratelimit_service_401_test.go, backend/internal/service/openai_oauth_service.go, backend/internal/service/openai_oauth_service_refresh_test.go, backend/internal/service/openai_privacy_service.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 6F scoped sync from `upstream/main@635ad81c`; covers OpenAI OAuth 401 credential safety and token-refresh enrichment only. Codex used-percent snapshot self-heal, OpenAI HTTP2 response-header timeout, and endpoint capability routing were audited and already present locally, so no duplicate account-page or scheduler rewrites were made.
**Change details**:
- OAuth 401 handling now invalidates token caches and marks the account temporarily unschedulable without persisting the request-start `account.Credentials` snapshot, preventing a concurrent fresh refresh token from being rolled back by an old snapshot.
- OpenAI OAuth `RefreshAccountToken` now enriches the existing-access-token/no-refresh-token path using the same ChatGPT backend best-effort account metadata flow as normal token refresh.
- Added ChatGPT subscriptions fallback enrichment for `subscription_expires_at` when `accounts/check` reports plan metadata but omits entitlement expiry.
- Kept OAuth privacy-disable best-effort behavior and proxy handling intact, while making backend URLs package-overridable for unit tests only.
- Preserved pricing defaults, global/user model pricing, display pricing/display token, Claude-GPT bridge overlay, distribution, public `/key-usage`, image trace safety, and account page layout.
- Verified with `go test -tags=unit ./internal/service -run "OAuth401|RateLimitService_HandleUpstreamError_OAuth401|OpenAIOAuthService_RefreshAccountToken_NoRefreshTokenUsesExistingAccessToken|OpenAITokenRefresher|OpenAITokenProvider"`, `go test -tags=unit ./internal/service -run "CodexSnapshot|ShouldRefreshOpenAICodexSnapshot|OpenAICodex|Endpoint|Capability|OpenAIAccountScheduler"`, `go test -tags=unit ./internal/config ./internal/repository -run "ResponseHeaderTimeout|HTTP2|HTTPUpstream"`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-07] fix: sync Phase 6G Codex and Claude Code mimicry fixes

**Affected files**: backend/internal/pkg/claude/constants.go, backend/internal/pkg/openai/constants.go, backend/internal/service/account.go, backend/internal/service/account_openai_passthrough_test.go, backend/internal/service/claude_code_validator.go, backend/internal/service/claude_code_validator_test.go, backend/internal/service/identity_service.go, backend/internal/service/openai_client_restriction_detector.go, backend/internal/service/openai_client_restriction_detector_test.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_service_codex_cli_only_test.go, backend/internal/service/openai_oauth_passthrough_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 6G scoped sync from `upstream/main@635ad81c`; covers Codex/Claude Code client mimicry and request fingerprint fidelity only. It intentionally does not merge account-page UI/settings, pricing, quota, risk-control, channel monitor, or marketing/login/payment changes.
**Change details**:
- Updated Claude Code mimicry defaults to CLI `2.1.161`, package version `0.94.0`, Node runtime `v24.3.0`, and removed `redact-thinking` from the default full-mimicry beta list while keeping the local Claude-GPT bridge overlay unchanged.
- Aligned the default Claude fingerprint used by identity rewriting with the shared Claude constants so generated metadata and outbound headers stay in sync.
- Added Claude Code validator compatibility for `/v1/messages/count_tokens` and billing-attribution system blocks that contain `x-anthropic-billing-header` plus `cc_entrypoint=cli`.
- Updated the Codex OAuth fallback User-Agent to the newer structured `codex_cli_rs/0.125.0 (...)` form and injected `x-codex-installation-id` into OAuth Codex `client_metadata` when an account device id is available.
- Added a backend-only allowed-client hook for `codex_cli_only_allowed_clients` and global allowed-client inputs, with account JSONB parsing tests. No admin UI/settings persistence was added in this sub-batch.
- Added `codex-auto-review` to OpenAI default models and switched synthetic Codex default instructions to the upstream model-aware helper where available.
- Preserved pricing defaults, global/user model pricing, display pricing/display token, Claude-GPT bridge routing/usage semantics, distribution, public `/key-usage`, image trace safety, and local docs/dev-stack behavior.
- Verified with `go test -tags=unit ./internal/service -run "ClaudeCodeValidator|CodexClientRestriction|CodexCLIOnly|CodexTransform|OpenAI.*Hotpath|OpenAIGatewayService|GetCodexCLIOnlyAllowedClients"` and `go test -tags=unit ./internal/pkg/openai ./internal/pkg/claude`.

## [2026-06-07] fix: sync Phase 6.5 long-context cache billing multipliers

**Affected files**: backend/internal/service/billing_service.go, backend/internal/service/billing_service_test.go, backend/internal/service/model_pricing_resolver_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 6.5 scoped sync of upstream long-context billing fixes `b9509e82` and `ed2aac25`; this only changes how existing model pricing metadata is applied when long-context pricing is already triggered. It does not write model prices, change global/user pricing configuration, or alter display pricing/display-token semantics.
**Change details**:
- Long-context pricing now applies the input-side multiplier to `cache_read_tokens`, matching OpenAI GPT-5.4/GPT-5.5 long-context semantics where cache reads are input-side replays.
- Long-context pricing now applies the same input-side multiplier to cache creation cost, including standard cache writes and `5m`/`1h` ephemeral cache creation breakdown prices.
- Added regression tests proving below-threshold cache read/write prices remain at base price, while above-threshold cache read/write prices are multiplied.
- Added a local pricing resolver regression that locks user-level model pricing as the final override over channel/global/base pricing while preserving inherited long-context metadata.
- Preserved global/user model pricing values, display pricing, display token, Claude-GPT bridge usage semantics, distribution, public `/key-usage`, image trace safety, and local docs/dev-stack behavior.
- Verified with `go test -tags=unit ./internal/service -run "OpenAIGPT54LongContextAppliesMultiplierToCache|OpenAIGPT54NoLongContextKeepsCache|LongContextAppliesMultiplierToCacheCreation5mAnd1h|UserOverride_BeatsChannelGlobal"` and `go test -tags=unit ./internal/service -run "Billing|Pricing|LongContext|DisplayToken|UserModelPricing|GlobalModelPricing"`.
- Real-request smoke after refreshing local fixtures passed with `go run ./tools/smoke --suite openai,bridge,images,custom` (28/28). OpenAI responses/chat, Claude-GPT bridge, Images upstream 400 passthrough, distribution, pricing, public `/key-usage`, announcements, usage errors, and group models-list checks are covered. `go run ./tools/smoke --suite embeddings` now reaches the OpenAI API-key account, but that account's upstream base URL returns `404 page not found` for `/v1/embeddings`; this is recorded as a fixture/upstream endpoint compatibility issue, not a Sub2API routing failure.

## [2026-06-07] docs: record phased OpenAI/Codex upstream sync closeout

**Affected files**: docs/dev/UPSTREAM_SYNC.md, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: documentation-only closeout for the staged upstream sync through Phase 6.5; no runtime, schema, API, pricing, or deployment behavior changes.
**Change details**:
- Added a current upstream-sync summary for `codex/openai-codex-upstream-sync` documenting the manual staged sync from `upstream/main@635ad81c`, the features already synced, preserved local overlays, and deferred upstream items.
- Updated the gateway codebase note with the current OpenAI/Codex flow, local Claude-GPT bridge overlay boundaries, request hotpath/usage/WS/OAuth/Codex mimicry fixes, long-context billing guardrails, and real-request smoke status.
- Recorded that `openai,bridge,images,custom` real-request smoke passes against the current dev stack, while embeddings reaches the API-key upstream and currently fails at that upstream's `/v1/embeddings` endpoint with 404.

## [2026-06-06] docs: record local GitHub CLI credential recovery

**Affected files**: AGENTS.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: local agent-operations documentation only; no runtime or deployment behavior changes.
**Change details**:
- Documented the expected `gh` account, `gh auth status` verification, browser/device login fallback, and safe local PAT recovery path for this workstation.
- Kept PAT values out of repository documentation and explicitly documented that tokens must not be printed, pasted into chat, committed, or logged.
- Verified current `gh` login is stored in the Windows keyring for account `541968679`.

## [2026-06-06] feat: add account-level OpenAI images endpoint scheduling toggle

**Affected files**: backend/internal/service/account.go, backend/internal/service/codex_image_generation_bridge.go, backend/internal/service/openai_account_scheduler_test.go, backend/internal/repository/account_repo_compact_extra_test.go, backend/tools/smoke/main.go, frontend/src/components/account/CreateAccountModal.vue, frontend/src/components/account/EditAccountModal.vue, frontend/src/components/account/__tests__/CreateAccountModal.spec.ts, frontend/src/components/account/__tests__/EditAccountModal.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/account.md, docs/dev/codebase/gateway.md
**Upstream compatibility**: local Phase 4.5 overlay only. The switch is intentionally independent from upstream Codex image-generation bridge controls and from later Phase 5 quota/risk-control/usage-error features.
**Change details**:
- Added `extra.openai_images_endpoint_enabled` as an account-level opt-out for independent OpenAI-compatible Images endpoints. Missing, null, or non-boolean values remain enabled for compatibility; JSON boolean `false` excludes the account from `/v1/images/generations` and `/v1/images/edits` scheduling.
- Kept the switch independent from Codex `/v1/responses` image-generation bridge injection, OpenAI chat/responses, embeddings, Claude-GPT bridge, display-token behavior, and billing/pricing semantics.
- Routed scheduler and legacy load-awareness selection through the same `SupportsOpenAIImageCapability` helper and kept the extra key scheduler-relevant so account snapshot refreshes happen when the toggle changes.
- Added Create/Edit account UI controls with Chinese/English i18n; enabled state omits the extra key, disabled state writes `false`.
- Hardened smoke fixture selection so images smoke does not choose OpenAI accounts with `openai_images_endpoint_enabled=false`.

## [2026-06-06] fix: include user model display pricing in admin usage rows

**Affected files**: backend/internal/handler/admin/usage_handler.go, frontend/src/components/admin/usage/UsageTable.vue, frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts, frontend/src/types/index.ts, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts
**Upstream compatibility**: local custom display-pricing behavior only. This preserves real billing and stored usage costs while making the admin usage list reflect the owning user's display-pricing override data.
**Change details**:
- Loaded user-level display-pricing overrides per usage-row owner in the admin usage list before building admin DTOs.
- Added `display_fields` typing and an admin tooltip section that shows the owning user's display-priced token/cost values separately from real stored costs.
- Added frontend coverage for admin rows that include user display fields and corrected the `$ / 1M tokens` test expectations.
- Verified with `pnpm run test:run -- UsageTable`, `go test -tags=unit ./internal/handler/dto ./internal/handler/admin`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-06] fix: preserve Anthropic tool IDs in OpenAI bridge

**Affected files**: backend/internal/service/openai_gateway_messages.go, backend/internal/service/openai_compat_model_test.go
**Upstream compatibility**: staged upstream sync phase 3 sub-batch only. Wires the upstream `PreserveToolCallIDs` option into the local OpenAI Messages compatibility path while keeping local Claude-GPT bridge prompt-cache/header behavior unchanged.
**Change details**:
- Preserved Anthropic `tool_use.id` / `tool_result.tool_use_id` values when OAuth `/v1/messages` requests are transformed into OpenAI Responses input.
- Added an end-to-end `ForwardAsAnthropic` regression test that verifies `toolu_*` call IDs are not rewritten to `fc_*`.
- Verified with `go test -tags=unit ./internal/service -run "ForwardAsAnthropic_OAuthPreservesAnthropicToolCallIDs|ForwardAsAnthropic_ClaudeGPTBridge|ApplyCodexOAuthTransform|FilterCodexInput"`, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go test -tags=unit ./internal/service ./internal/handler`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-06] feat: sync phase 3 Codex transform compatibility

**Affected files**: backend/internal/service/openai_codex_transform.go, backend/internal/service/openai_codex_transform_test.go, backend/internal/service/openai_model_mapping_test.go
**Upstream compatibility**: staged upstream sync phase 3 sub-batch only. Imports upstream Codex transform behavior without deleting local Claude-GPT bridge prompt-cache semantics or local GPT-5.5/GPT-5.5-pro mappings.
**Change details**:
- Added upstream Codex model aliases, version suffix handling, and unknown-model preservation while keeping local GPT-5.5/GPT-5.5-pro aliases and date suffixes.
- Added Codex base-instructions fallback from `internal/pkg/openai`, reasoning encrypted-content include injection, client metadata installation-id helper, and a `PreserveToolCallIDs` transform option.
- Fixed legacy `call_` to `fc_` call-id normalization and added tests for preserving native tool call IDs when the bridge path needs it.
- Preserved local body `prompt_cache_key` behavior for Claude-GPT bridge; upstream's body deletion was intentionally not imported.
- Verified with `go test -tags=unit ./internal/service -run "ResolveOpenAIForwardModel|NormalizeCodexModel|NormalizeOpenAIModelForUpstream|ApplyCodexOAuthTransform|FilterCodexInput|CodexClientMetadata|ForwardAsAnthropic|ClaudeGPTBridge"`, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go test -tags=unit ./internal/service ./internal/handler`, and `go run ./tools/upstream-sync-guard`.

## [2026-06-06] chore: sync phase 2 schema and migration union

**Affected files**: backend/migrations/150_affiliate_ledger_audit_snapshots.sql, backend/migrations/151_image_generation_group_controls.sql, backend/migrations/152_allow_email_oauth_provider_types.sql, backend/migrations/153_content_moderation.sql, backend/migrations/154_add_dingtalk_provider_type.sql, backend/migrations/155_remove_ops_retry_replay.sql, backend/migrations/156_usage_log_image_size_metadata.sql, backend/migrations/157_redeem_code_expires_at.sql, backend/migrations/158_channel_monitor_openai_api_mode.sql, backend/migrations/159_seed_openai_monitor_templates.sql, backend/migrations/160_extend_user_provider_default_grants_check.sql, backend/migrations/161_subscription_expiry_notify_enabled.sql, backend/migrations/162_user_platform_quotas.sql, backend/migrations/163_group_models_list_config.sql, backend/migrations/164_deleted_api_key_audit.sql, backend/migrations/165_ops_error_log_api_key_prefix.sql, backend/migrations/166_add_ops_error_logs_user_time_index_notx.sql, backend/ent/schema, backend/ent, backend/internal/domain/models_list_config.go, backend/internal/service/group_models_list.go, backend/internal/service/group.go, backend/internal/repository/api_key_repo.go, backend/internal/handler/dto
**Upstream compatibility**: staged upstream sync phase 2 only. Adds upstream DB/Ent shape as an additive union while preserving local custom migrations and schema such as AI credit snapshots, display token/pricing, distribution, custom announcements, model pricing, and local gateway metadata.
**Change details**:
- Added upstream migrations as local 150-166 without rewriting historical local migration numbers. Upstream `144_add_opus48_to_model_mapping.sql` was intentionally skipped because local migration 146 already mirrors that change.
- Added the upstream `user_platform_quota` schema plus group image controls, group `/v1/models` list config, usage image metadata, channel monitor API mode, redeem-code expiry, and OAuth/DingTalk enum expansion.
- Regenerated Ent after schema union and kept local generated custom entities, including `aicreditsnapshot`.
- Exposed new group fields in read-side service/DTO mapping only; admin write paths are deferred to the later frontend/API feature phase to avoid accidental zero-value overwrites.
- Verified with `go run ./tools/upstream-sync-guard` and `go test -tags=unit ./internal/repository ./internal/service ./internal/handler`.

## [2026-06-06] fix: sync phase 1 low-coupling upstream security fixes

**Affected files**: backend/go.mod, backend/go.sum, backend/internal/handler/api_key_handler.go, backend/internal/handler/api_key_handler_security_test.go, backend/internal/service/api_key_service.go, backend/internal/service/api_key_service_security_test.go, backend/internal/service/openai_images.go, backend/internal/service/openai_images_responses.go, backend/internal/service/openai_images_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: staged upstream sync phase 1 only. Mirrors upstream fixes `11b60171`, `0ae33296`, and `381d1d6d` without merging OpenAI/Codex hotpath refactors or Ent schema changes.
**Change details**:
- Returned 404 instead of 403 when an authenticated user requests another user's API key ID, preventing an API key ID oracle.
- HTML-escaped API key names on create/update and also applied the same protection to local distribution-created API keys.
- Preserved real upstream Images HTTP errors for non-failover cases so OpenAI-compatible Images clients receive the upstream status, type, code, param, and message instead of a generic 502.
- Updated the backend module to Go 1.26.4 and upgraded the selected x/* dependencies following the phase plan.
- Verified with `go run ./tools/upstream-sync-guard`, `go test -tags=unit ./internal/handler ./internal/service`, `pnpm run test:run -- menuLocaleCoverage`, `pnpm run typecheck`, and `go run ./tools/smoke --suite quick,custom`.

## [2026-06-06] test: add upstream sync phase 0 guards and smoke checks

**Affected files**: backend/tools/upstream-sync-guard/main.go, backend/tools/smoke/main.go, frontend/src/i18n/__tests__/menuLocaleCoverage.spec.ts, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, frontend/src/components/account/AccountUsageCell.vue, frontend/src/components/account/__tests__/AccountUsageCell.spec.ts, frontend/src/components/charts/ModelDistributionChart.vue, frontend/src/components/charts/GroupDistributionChart.vue, frontend/src/components/charts/__tests__/ModelDistributionChart.spec.ts, frontend/src/components/charts/__tests__/GroupDistributionChart.spec.ts, frontend/src/views/admin/DashboardView.vue, frontend/src/views/auth/__tests__/EmailVerifyView.spec.ts, frontend/src/composables/usePersistedPageSize.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: phase 0 protection only; no upstream runtime merge yet. The guard is intended to stop future upstream-sync phases from deleting local custom features.
**Change details**:
- Added an upstream-sync guard that fails on protected local feature deletion, historical migration rewrites, duplicate migration numbers, and missing custom feature signatures.
- Added reusable real HTTP smoke tooling for quick/custom/openai/images/bridge/embeddings suites. The quick/custom suites reuse the local dev database and write JSON reports under tmp/smoke.
- Added frontend i18n/menu coverage so router/sidebar/static translation keys must exist in both zh/en locales and cannot render raw variable names.
- Fixed existing frontend test baselines and numeric formatting edge cases that blocked the phase 0 test gate without changing upstream-sync behavior.
- Verified with `powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev-stack.ps1 restart -SkipAIClient`, `go test -tags=unit ./internal/server ./internal/handler ./internal/service`, `pnpm run typecheck`, `pnpm run test:run`, `go run ./tools/upstream-sync-guard`, and `go run ./tools/smoke --suite quick,custom`.

## [2026-06-06] fix: sync upstream OpenAI response.failed handling

**Affected files**: backend/internal/handler/stream_error_event.go, backend/internal/handler/stream_error_event_test.go, backend/internal/handler/gateway_handler.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/handler/openai_chat_completions.go, backend/internal/handler/openai_images.go, backend/internal/service/openai_codex_transform.go, backend/internal/service/openai_gateway_messages.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 3 OpenAI/Codex core sync from `upstream/main@1f423ae0`; local Claude-GPT bridge and `OPENAI_IMAGE_TRACE_LOG` behavior remain preserved.
**Change details**:
- Added Responses-protocol `response.failed` SSE emission when `/responses` streams have already flushed headers, including bare `/responses` and Codex direct route variants.
- Avoided appending generic fallback errors when OpenAI forwarding already wrote an upstream terminal error event.
- Kept Anthropic and Chat Completions legacy stream error formats for non-Responses endpoints.
- Fixed the OpenAI Claude-GPT bridge Codex instruction transform so forced instruction templates can see original Anthropic system/developer text without injecting the generic default instructions first.
- Verified with `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go test -tags=unit ./internal/service ./internal/handler`, and `go run ./tools/upstream-sync-guard`.

## [2026-06-06] fix: sync upstream OpenAI responses chat fallback

**Affected files**: backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_responses_chat_fallback.go, backend/internal/service/openai_gateway_responses_chat_fallback_test.go, backend/internal/service/openai_gateway_chat_completions_raw.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 3 OpenAI/Codex core sync from `upstream/main@1f423ae0`; API key accounts marked as not supporting `/v1/responses` now serve Responses clients through `/v1/chat/completions` without changing local Claude-GPT bridge or display-token behavior.
**Change details**:
- Added `/v1/responses` -> `/v1/chat/completions` fallback for OpenAI API key accounts whose responses support mode is forced off or probe state says unsupported.
- Converted upstream Chat Completions JSON/SSE responses back into Responses JSON/SSE for downstream clients, including DeepSeek reasoning-only streams and usage-only stream chunks.
- Extended JSON usage extraction to accept Chat Completions `prompt_tokens` / `completion_tokens` fields when this fallback path reads upstream usage.
- Verified with focused fallback tests, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go test -tags=unit ./internal/service ./internal/handler`, and `go run ./tools/upstream-sync-guard`.

## [2026-06-06] fix: sync upstream raw chat completions usage and URL handling

**Affected files**: backend/internal/service/openai_endpoint_url.go, backend/internal/service/openai_gateway_chat_completions_raw.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_responses_chat_fallback_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 3 OpenAI/Codex core sync from `upstream/main@1f423ae0`; scoped to OpenAI API key raw Chat Completions forwarding and shared OpenAI endpoint URL construction.
**Change details**:
- Forced raw `/v1/chat/completions` stream forwarding to request `stream_options.include_usage=true`, so upstream usage is available for billing even when the client omitted the option.
- Continued draining upstream SSE after downstream client disconnects, preserving usage extraction without writing more data to the disconnected client.
- Added a raw Chat Completions header allowlist so Codex/OAuth-specific headers like `session_id`, `conversation_id`, and `x-codex-turn-state` are not forwarded to third-party API-key upstreams.
- Added shared OpenAI endpoint URL construction for versioned compatible base URLs such as `/api/paas/v4`, covering Responses and Chat Completions.
- Routed raw Chat Completions non-streaming reads through the existing upstream response-size guard and kept display-token rewriting downstream-only.
- Verified with focused raw/fallback tests, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go test -tags=unit ./internal/service ./internal/handler`, and `go run ./tools/upstream-sync-guard`.

## [2026-06-06] fix: sync upstream OpenAI Messages bridge core

**Affected files**: backend/internal/service/openai_gateway_messages.go, backend/internal/service/openai_messages_bridge.go, backend/internal/service/openai_messages_continuation.go, backend/internal/service/openai_messages_digest_session.go, backend/internal/service/openai_messages_replay_guard.go, backend/internal/service/openai_messages_todo_guard.go, backend/internal/service/openai_compat_prompt_cache_key.go, backend/internal/service/openai_tool_continuation.go, backend/internal/service/openai_ws_forwarder.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/service/openai_compat_model_test.go, backend/internal/service/openai_tool_continuation_test.go, backend/internal/handler/openai_gateway_handler_test.go, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 3 OpenAI/Codex Messages sync from `upstream/main@1f423ae0`; upstream Anthropic-to-Responses conversion, Codex transform, terminal-event parsing, continuation, digest session, replay guard, and todo guard are used as the core while local Antigravity scheduling, bridge usage, display cache, display-token rewrite, and session-header stripping are preserved.
**Change details**:
- Rebased `ForwardAsAnthropic` on the upstream Messages flow, including `previous_response_id` continuation for API-key compat, Anthropic digest-derived prompt cache keys, replay trimming, Claude Code todo guard injection, `response.failed`/missing-terminal handling, and raw SSE frame parsing.
- Kept the local Claude-GPT bridge overlay: Antigravity preflight remains outside the core, bridge requests still preserve body `prompt_cache_key`, and upstream `session_id` / `conversation_id` headers are deleted after request construction.
- Preserved bridge usage semantics: downstream model/requested model remain Claude, `upstream_model` remains GPT, bridge cache-display override and display-token SSE/non-stream rewriting still run after upstream terminal usage is parsed.
- Extended Codex tool-output detection from only `function_call_output` to `tool_search_output`, `custom_tool_call_output`, and `mcp_tool_call_output` in HTTP validation and WS continuation checks, keeping tool continuation behavior aligned with upstream.
- Kept local `toolu_*` preservation by validating tool call IDs by type rather than by fixed input index, since upstream todo guard can prepend developer input.
- Verified with `go test -tags=unit ./internal/service -run "ForwardAsAnthropic|ClaudeGPTBridge|OpenAICompat|ToolContinuation|ReplayGuard|PromptCache|CodexTransform"`, `go test -tags=unit ./internal/handler -run "OpenAIMessages|ClaudeGPTBridge|FunctionCallOutput"`, `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`, `go test -tags=unit ./internal/service ./internal/handler`, `go run ./tools/upstream-sync-guard`, `git diff --check`, and `go run ./tools/smoke --suite openai,bridge`.
- Local smoke note: the dev PostgreSQL `schema_migrations` table had stale checksums for already-applied `150-166` migrations from a prior branch state; the local dev DB records were updated to match the current migration files so the backend could start for real-request smoke. No migration files were changed.

## [2026-06-06] feat: add OpenAI embeddings endpoint and endpoint capability scheduling

**Affected files**: backend/internal/handler/endpoint.go, backend/internal/handler/openai_embeddings.go, backend/internal/server/routes/gateway.go, backend/internal/service/account.go, backend/internal/service/http_upstream_profile.go, backend/internal/service/openai_account_scheduler.go, backend/internal/service/openai_embeddings.go, backend/internal/service/upstream_context.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 4 OpenAI Embeddings sync from `upstream/main@1f423ae0`; scoped to OpenAI API key embeddings and endpoint capability scheduling, without changing the local Claude-GPT bridge scheduler path.
**Change details**:
- Added OpenAI-compatible `POST /v1/embeddings` for OpenAI groups, including request validation, OpenAI API-key forwarding, upstream response passthrough, usage extraction, and usage-log recording.
- Added `credentials.openai_capabilities` endpoint gating with `chat_completions` and `embeddings`; missing configuration remains backward-compatible and allows existing OpenAI API key accounts to serve chat completions.
- Updated `/v1/responses`, `/v1/chat/completions`, native OpenAI `/v1/messages`, and OpenAI WS initial account selection to require the chat-completions capability, while the Claude-GPT bridge still uses `SelectAccountWithSchedulerForClaudeGPTBridge`.
- Added the minimal upstream context/profile helpers needed by embeddings forwarding, and kept pool-mode retry behavior on the existing local default status-code list.
- Verified with `go test -tags=unit ./internal/handler -run "Endpoint|Embeddings"`, `go test -tags=unit ./internal/service -run "Embeddings|OpenAIAccountScheduler|OpenAIImage|PoolMode"`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-06] fix: bridge oversized OpenAI websocket requests through HTTP

**Affected files**: backend/internal/config/config.go, backend/internal/config/config_test.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_ws_forwarder.go, backend/internal/service/openai_ws_http_bridge.go, backend/internal/service/openai_ws_http_bridge_test.go, backend/internal/service/image_output_accounting.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 4 OpenAI WS sync from `upstream/main@1f423ae0`; scoped to oversized Responses WebSocket ingress frames and replay continuity, without changing Antigravity Claude-GPT bridge dispatch or fallback semantics.
**Change details**:
- Added configurable OpenAI WS client read limit and HTTP bridge threshold defaults so frames above the old 16 MiB WS limit can keep the downstream WS connection while using `/v1/responses` SSE upstream.
- Added `proxyOpenAIWSHTTPBridgeTurn` to strip WS-only fields, force HTTP streaming, relay SSE events as WS messages, preserve terminal usage parsing, and surface upstream HTTP/SSE errors as WS error events.
- Preserved tool-call replay context across bridge turns so follow-up `function_call_output` frames can become self-contained HTTP `/responses` requests without forwarding stale `previous_response_id`.
- Added shared image-output counting helpers required by the WS bridge; independent Images endpoint routing/accounting remains a later Phase 4 sub-batch.
- Kept local Claude-GPT bridge, display-token, display-pricing, distribution, public `/key-usage`, and docs/dev-stack paths untouched by this sub-batch.
- Verified with `go test -tags=unit ./internal/service -run "OpenAIWSHTTPBridge|HTTPBridge|OpenAIWS.*Bridge|WebSocket"`, `go test -tags=unit ./internal/service -run "OpenAIWS|HTTPBridge|WebSocket|ClaudeGPTBridge|DisplayToken|Pricing"`, `go test -tags=unit ./internal/handler -run "OpenAI.*WebSocket|OpenAIMessages|ClaudeGPTBridge|Endpoint|Images"`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-06] fix: sync upstream OpenAI Images API-key streaming and image cooldown

**Affected files**: backend/internal/handler/openai_images.go, backend/internal/pkg/ctxkey/ctxkey.go, backend/internal/service/image_generation_intent.go, backend/internal/service/model_rate_limit.go, backend/internal/service/openai_images.go, backend/internal/service/ratelimit_service.go, backend/internal/service/model_rate_limit_test.go, backend/internal/service/ratelimit_service_openai_test.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 4 OpenAI Images sync from `upstream/main@1f423ae0`; scoped to API-key `/v1/images/*` streaming/error handling and image-generation cooldown, preserving local `OPENAI_IMAGE_TRACE_LOG` and existing image billing semantics.
**Change details**:
- Added image-generation intent helpers and context marking so `/v1/images/*` requests honor group `allow_image_generation` and OpenAI image-specific model-rate-limit scope.
- API-key Images forwarding now uses the detached upstream context, OpenAI HTTP upstream profile, upstream error-body helper, configured pool-mode retry status policy, and upstream 400/error passthrough path.
- API-key image streaming now supports keepalive comments, idle timeout error events, downstream disconnect drain-for-billing, fallback JSON accounting, image output size accounting, and response usage extraction from streamed image events.
- Added OpenAI image 429 cooldown handling that writes `openai:image_generation` model-rate-limit scope instead of disabling/rate-limiting the whole OpenAI account when the upstream error is image-specific.
- Kept `ImageSize` / `ImageSizeInfo` / `ImageQuality` as the local real-billing inputs and retained safe `OPENAI_IMAGE_TRACE_LOG` timing/correlation log points without logging prompts, image bytes, auth, cookies, API keys, or full bodies.
- Verified with `go test -tags=unit ./internal/service -run "OpenAI.*Images|ImageOutput|ImageTrace|ModelRateLimit|Handle429_OpenAIImage|CalculateOpenAI429|OpenAIImageRateLimit"`, `go test -tags=unit ./internal/handler -run "OpenAI.*Images|Images|GroupModel"`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-06] feat: add OpenAI account endpoint capabilities and Codex image bridge override

**Affected files**: backend/internal/config/config.go, backend/internal/service/codex_image_generation_bridge.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_ws_forwarder.go, frontend/src/components/account/CreateAccountModal.vue, frontend/src/components/account/EditAccountModal.vue, frontend/src/components/account/__tests__/EditAccountModal.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, frontend/src/types/index.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 4 account-management minimal union from `upstream/main@1f423ae0`; scoped to OpenAI API-key endpoint capabilities and account-level Codex image-generation bridge override, without bringing in upstream account page re-layout, Codex session import, or model sync preview.
**Change details**:
- Added `gateway.codex_image_generation_bridge_enabled` as a default-off global fallback and `extra.codex_image_generation_bridge` account override for Codex `/v1/responses` image-generation tool injection.
- Kept backward compatibility for legacy `extra.codex_image_generation_bridge_enabled` and nested `extra.openai.*` values, while frontend saves the new field and removes the legacy key.
- Gated HTTP and WS Codex image-generation bridge injection by the account override/global fallback without changing independent `/v1/images/*` scheduling, local Claude-GPT bridge dispatch, display-token behavior, or Antigravity fallback semantics.
- Added OpenAI API Key account Create/Edit controls for `credentials.openai_capabilities` with `chat_completions` and `embeddings`, preserving the backward-compatible default when both are selected.
- Added Chinese/English i18n keys and EditAccountModal regressions covering endpoint capability save, minimum-one capability behavior, and legacy Codex image bridge migration.
- Verified with `go test -tags=unit ./internal/service -run "CodexImageGenerationBridge|ImageGenerationBridge|OpenAIWS|OpenAIGatewayService"`, `pnpm run typecheck`, `pnpm run test:run -- EditAccountModal CreateAccountModal BulkEditAccountModal`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-06] fix: preserve image generation group permissions in API key auth cache

**Affected files**: backend/internal/repository/api_key_repo.go, backend/internal/service/api_key_auth_cache.go, backend/internal/service/api_key_auth_cache_impl.go, backend/internal/service/api_key_service_cache_test.go, backend/tools/smoke/main.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 4 OpenAI Images/Embeddings real-request validation hardening; scoped to API-key auth hot path and smoke fixture selection, without changing pricing, Claude-GPT bridge, distribution, public `/key-usage`, or account scheduling semantics.
**Change details**:
- Fixed `GetByKeyForAuth` to select `groups.allow_image_generation`; otherwise the lightweight API-key auth path hydrated `apiKey.Group.AllowImageGeneration=false` even when the database group enabled images.
- Added `AllowImageGeneration` to the API-key auth cache snapshot and bumped the snapshot version to invalidate old cached group snapshots.
- Added a snapshot round-trip regression test so image permissions are preserved through auth cache DB-load and cache-hit paths.
- Hardened `backend/tools/smoke` to load ignored `tmp/smoke/local.env`, use platform-specific local keys without printing secrets, and select fixtures by real capability: OpenAI chat/responses, image-capable OpenAI group, embeddings-capable OpenAI API-key group, and Antigravity bridge key.
- Tightened real-request assertions so `/v1/responses`, `/v1/chat/completions`, `/v1/images/generations` invalid-size passthrough, and `/v1/embeddings` must return their expected statuses instead of accepting broad 2xx-4xx ranges.
- Verified with `go test -tags=unit ./internal/service -run "APIKeyService_SnapshotRoundTrip_PreservesAllowImageGeneration|OpenAI.*Images|ImageGeneration|Embeddings|CodexImageGenerationBridge"`, `go test -tags=unit ./internal/server ./internal/handler -run "Embeddings|OpenAI.*Images|ImageConcurrency"`, `go run ./tools/upstream-sync-guard`, `git diff --check`, and `go run ./tools/smoke --suite openai,images,embeddings`.
- Local smoke note: OpenAI chat/responses and images invalid-size passthrough pass against the current dev stack; embeddings is blocked by fixture availability because the local database currently has no active OpenAI `apikey` upstream account in any downstream-key group.

## [2026-06-06] fix: sync Phase 5A upstream stability and safety fixes

**Affected files**: backend/internal/service/leader_lock.go, backend/internal/repository/leader_lock_cache.go, backend/internal/service/dashboard_aggregation_service.go, backend/internal/service/subscription_expiry_service.go, backend/internal/service/payment_order_expiry_service.go, backend/internal/repository/session_limit_cache.go, backend/internal/repository/user_msg_queue_cache.go, backend/internal/setup/setup.go, backend/internal/repository/account_repo.go, backend/internal/repository/api_key_repo.go, backend/internal/service/admin_service.go, backend/internal/handler/openai_stream_validation.go, backend/internal/handler/gateway_handler_chat_completions.go, backend/internal/handler/gateway_handler_responses.go, backend/internal/handler/openai_chat_completions.go, backend/internal/handler/openai_gateway_handler.go, backend/cmd/server/wire_gen.go, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 5A scoped sync from `upstream/main@635ad81c`; this sub-stage only covers operational stability and safety fixes and intentionally does not merge quota, risk-control, usage error requests, group models-list UI, pricing, distribution, or account-page re-layout.
**Change details**:
- Added a Redis-backed leader lock for existing dashboard aggregation, subscription-expiry, and payment-order-expiry background tasks so multi-instance deployments do not run the same periodic job concurrently.
- Added Redis Lua `redis.replicate_commands()` compatibility for scripts that call `TIME`, preserving existing session-limit and user message queue semantics.
- Changed setup database bootstrap to connect to the maintenance `postgres` database before creating/connecting to the configured target database.
- Refreshed scheduler account snapshots after clearing temporary unschedulable state.
- When deleting a user, API keys are deleted first with deleted-key audit support when available; auth caches are invalidated for each key and for the user.
- Treated allowed proxy-quality HTTP statuses as pass results and added OpenAI-compatible `stream` field type validation for chat completions/responses/messages ingress.
- Preserved local custom features: pricing/display token, distribution, public `/key-usage`, Claude-GPT bridge, AI credit snapshot, announcement surfaces, image trace logging, and dev-stack/docs.
- Verified with `go test -tags=unit ./internal/service -run "DeleteUser|ProxyQuality"`, `go test -tags=unit ./internal/server -run TestAPIContracts`, `go test -tags=unit ./internal/setup ./internal/repository ./internal/service ./internal/handler ./internal/server`, `go run ./tools/upstream-sync-guard`, and `git diff --check`.

## [2026-06-06] feat: sync Phase 5 usage errors and group models list

**Affected files**: backend/internal/handler/admin/group_handler.go, backend/internal/handler/gateway_handler.go, backend/internal/handler/admin/ops_handler.go, backend/internal/handler/ops_error_logger.go, backend/internal/repository/group_repo.go, backend/internal/repository/ops_repo.go, backend/internal/server/routes/admin.go, backend/internal/service/admin_service.go, backend/internal/service/ops_*.go, backend/tools/smoke/main.go, backend/tools/upstream-sync-guard/main.go, frontend/src/api/admin/groups.ts, frontend/src/api/admin/ops.ts, frontend/src/components/admin/group/GroupModelsListConfigPanel.vue, frontend/src/types/index.ts, frontend/src/views/admin/GroupsView.vue, frontend/src/views/admin/groupsModelsList.ts, frontend/src/views/admin/__tests__/groupsModelsList.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: Phase 5B/5C scoped sync from `upstream/main@635ad81c`; this entry records the usage failed/error request display already committed in `ed0c9b98` and the current group custom `/v1/models` list integration. Quota, risk-control/content moderation, channel monitor OpenAI API mode, account quota auto-pause, payment/login/marketing updates, and account-page re-layout remain deferred.
**Change details**:
- Added user-facing usage error request APIs and frontend usage tab in Phase 5B while preserving local ops panels and accepting upstream removal of ops retry/replay.
- Added group `models_list_config` create/update persistence, admin candidate model endpoint, and gateway filtering for `GET /v1/models`; this affects only the displayed model list and does not change scheduling, model mapping, allow/block lists, billing, or Claude-GPT bridge behavior.
- Added a minimal Groups page panel with Chinese/English i18n for configuring the custom `/v1/models` list without replacing the local group rate, RPM override, distribution, or OpenAI Messages controls.
- Removed remaining ops retry/replay code and frontend retry API exports to match accepted upstream deletion and local migration `155_remove_ops_retry_replay.sql`; normal gateway failover, account-pool retry, 429/5xx cooldown, and request error display remain intact.
- Extended `backend/tools/smoke` custom suite to check usage error request APIs, `/v1/models` response shape, and group models-list candidates without writing pricing or billing configuration.
- Extended `backend/tools/upstream-sync-guard` with signatures for usage errors and group models-list route/UI/gateway plumbing.
- Verified locally with `go test -tags=unit ./internal/handler ./internal/service ./internal/repository -run "Usage|Ops|Error|APIKey|Deleted"`, `go test -tags=unit ./internal/handler ./internal/service ./internal/repository -run "Group|ModelsList|GatewayModels"`, `go test -tags=unit ./internal/handler ./internal/service ./internal/repository ./internal/server`, `go test -tags=unit ./cmd/server`, `pnpm run typecheck`, `pnpm run test:run`, `go run ./tools/upstream-sync-guard`, `git diff --check`, migration duplicate check for new `150+` migrations, and `go run ./tools/smoke --suite custom,bridge` (25/25 passed).
- Full local smoke `go run ./tools/smoke --suite quick,custom,openai,bridge,images,embeddings` passed 32/33 checks; the only failure is fixture availability for embeddings because the current dev DB has no active OpenAI API-key upstream account bound to the downstream key group. OpenAI responses/chat, images invalid-size passthrough, bridge, usage errors, distribution, pricing, announcements, and group models-list checks passed.

## [2026-06-05] feat: extend announcements across popup, dashboard banner, and API key rules surfaces

**Affected files**: backend/ent/schema/announcement.go, backend/ent/schema/announcement_read.go, backend/migrations/148_extend_announcements_surfaces.sql, backend/migrations/149_announcement_reads_drop_read_at_default.sql, backend/internal/domain/announcement.go, backend/internal/service/announcement.go, backend/internal/service/announcement_service.go, backend/internal/repository/announcement_repo.go, backend/internal/repository/announcement_read_repo.go, backend/internal/handler/announcement_handler.go, backend/internal/handler/admin/announcement_handler.go, backend/internal/handler/dto/announcement.go, backend/internal/server/routes/user.go, frontend/src/types/index.ts, frontend/src/api/announcements.ts, frontend/src/api/admin/announcements.ts, frontend/src/stores/announcements.ts, frontend/src/views/admin/AnnouncementsView.vue, frontend/src/views/user/DashboardView.vue, frontend/src/components/user/dashboard/DashboardAnnouncementBanner.vue, frontend/src/components/keys/GettingStartedGuide.vue, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, docs/dev/codebase/announcements.md, docs/dev/codebase/README.md
**Why**: reuse the existing announcement system for daily popup scheduling, a dashboard top banner, and editable API key usage rules without adding a separate settings module.
**Change details**:
- Added announcement `surface` and `popup_frequency` fields plus nullable popup/banner dismissal timestamps on `announcement_reads`.
- Added user `surface` filtering, backend-computed `should_popup`, popup-dismiss and banner-dismiss endpoints, and admin create/update/list support for the new fields.
- Updated the global popup queue to rely on `should_popup`, and separated popup dismissal, banner dismissal, and read-state behavior.
- Added an admin surface/frequency editor, a dashboard banner component, and an API key usage-rules modal before the getting-started steps.
- Documented the announcement module flow, state semantics, nullable read-state repository pitfall, and immutable follow-up migration for dropping the legacy `read_at` default.
- Verified with `go test -tags=unit ./internal/service ./internal/repository ./internal/handler/... ./internal/server/...`, `pnpm run typecheck`, `pnpm run lint:check`, and `pnpm build`.

## [2026-06-04] feat: surface configurable support contact in user flows

**Affected files**: frontend/src/components/common/SupportContactBar.vue, frontend/src/components/common/__tests__/SupportContactBar.spec.ts, frontend/src/components/user/dashboard/UserDashboardQuickActions.vue, frontend/src/views/user/PaymentView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: frontend-only enhancement that reuses the existing public `contact_info` setting; no backend API or settings schema changes.
**Change details**:
- Added a compact reusable support contact bar that reads `appStore.contactInfo`, fetches public settings when needed, and offers a copy action.
- Displayed the contact bar in the user dashboard quick-actions card and at the bottom of the purchase/payment selection page so support contact is easier to find without occupying a full card.
- Updated admin settings helper text in Chinese and English to document the new dashboard and payment/redeem/profile/menu display locations.
- Added component coverage for empty configuration, settings fetch, and copy behavior.

## [2026-06-04] ops: sync production tutorial page content

**Affected files/data**: production `settings.tutorial_page.content`, `docs/dev/CHANGELOG_CUSTOM.md`
**Upstream compatibility**: data-only production content sync; no runtime, schema, API, or image changes.
**Change details**:
- Synced the production tutorial page Markdown from the verified local development database value.
- Production backup files were created before the update: `/opt/sub2api/backups/tutorial_page.content.20260604T014422Z.sql` and `/opt/sub2api/backups/tutorial_page.content.20260604T014422Z.md`.
- Verified the production value changed from md5 `80db5e44a43fac0679b841a9c9939299`, length `19206`, updated `2026-05-05 21:31:10 +08`, to md5 `111eb6bfb4d253a288485d62481ee7a9`, length `21687`, updated `2026-06-04 09:44:23 +08`.
- The synced content header is `# ZeroCode API 使用文档` with `最后更新：2026-05-25`.

## [2026-06-03] docs: refresh Claude-GPT bridge production handoff

**Affected files**: `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md`, `docs/dev/DEPLOYMENT.md`, `docs/dev/PRODUCTION_CUSTOM_IMAGE_DEPLOY.md`, `docs/dev/codebase/README.md`, `docs/dev/ARCHITECTURE.md`, `docs/dev/CHANGELOG_CUSTOM.md`
**Upstream compatibility**: documentation-only; no runtime, schema, API, or deployment behavior changes.
**Change details**:
- Recorded the current verified production bridge deployment: `v0.1.137`, revision `e385b9ac7d7e840658cbcb4f7f9f8f11b1954b81`, image `ghcr.io/541968679/sub2api:latest`, version label `0.1.137`, healthy `/health`.
- Clarified that the current Release workflow publishes GHCR images only from `v*` tags or `workflow_dispatch`; pushing `main` alone does not refresh `latest`.
- Added the admin UI handoff for OpenAI account bridge configuration and Gateway Forwarding Behavior cache-display settings.
- Updated the codebase documentation index dates and descriptions for account, model mapping, billing, gateway, and the bridge handoff document.

## [2026-06-03] fix: suppress derived upstream cache/session keys in Claude-GPT bridge

**Affected files**: backend/internal/service/openai_gateway_messages.go, backend/internal/service/openai_compat_model_test.go, docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md, docs/dev/codebase/billing.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped to the custom OpenAI Claude-GPT bridge for Antigravity groups; normal OpenAI `/v1/messages` still forwards explicit prompt/session keys.
**Change details**:
- Traced the fixed `raw_cached_tokens=18944` value to raw OpenAI Responses SSE usage at `response.usage.input_tokens_details.cached_tokens`, then found bridge requests were also forwarding stable upstream cache/session signals derived from Claude `metadata.user_id`.
- Kept real upstream `cached_tokens` preservation, but stopped bridge mode from injecting or forwarding `prompt_cache_key`, `session_id`, and `conversation_id` to OpenAI/Codex upstreams.
- Preserved local `metadata.user_id`-derived sticky account scheduling, so bridge account selection still remains stable without creating upstream cache identity.
- Added regression coverage proving bridge OAuth/API-key forwards omit cache/session identifiers while non-bridge OpenAI Messages behavior still forwards them.
- Verified with focused unit tests and a real local `/v1/messages` bridge request: diagnostics logged all upstream cache/session flags as false, downstream response model stayed `claude-opus-4-8`, and usage row `15770` stored `upstream_model=gpt-5.5`, `input_tokens=25`, `output_tokens=8`, `cache_read_tokens=0`.

## [2026-06-03] fix: generate Claude-GPT bridge cache display from admin percent range

**Affected files**: backend/internal/service/openai_gateway_messages.go, backend/internal/service/setting_service.go, backend/internal/service/settings_view.go, backend/internal/service/domain_constants.go, backend/internal/service/openai_compat_model_test.go, backend/internal/service/setting_service_update_test.go, backend/internal/handler/admin/setting_handler.go, backend/internal/handler/dto/settings.go, frontend/src/api/admin/settings.ts, frontend/src/views/admin/SettingsView.vue, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md, docs/dev/codebase/billing.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped to OpenAI-backed Claude-GPT bridge requests from Antigravity groups; ordinary OpenAI cache accounting and native Antigravity forwarding remain unchanged.
**Change details**:
- Restored body-level `prompt_cache_key` forwarding for bridge OpenAI upstream requests while continuing to remove `session_id` and `conversation_id` headers, keeping the bridge body closer to normal OpenAI traffic so upstream cache can work.
- Added admin setting `openai_claude_gpt_bridge_cache_display_settings` with `enabled`, `min_percent`, and `max_percent`; backend and frontend validation require `0 <= min_percent <= max_percent <= 100`.
- When enabled, bridge responses directly generate a random display/billing cache-read value from the configured percentage range over upstream `input_tokens`, replacing upstream `cached_tokens` for downstream Anthropic usage and usage records.
- Clarified and covered with tests that the generated cache value is not derived from, added to, or scaled from upstream `cached_tokens`; upstream cache data is only diagnostic when the override is enabled.
- Restored downstream display-token rewriting for OpenAI Messages / Antigravity bridge `/v1/messages`, including streaming Anthropic SSE, so users configured for display-mode downstream usage see response usage aligned with usage-log display.
- Kept raw upstream `cached_tokens` logging as diagnostics only, so fixed upstream values such as `18944` can still be traced without leaking into user-visible bridge cache display when the override is enabled.
- Added focused coverage for prompt-cache body forwarding, cache display override, 60%-70% range validation, fixed upstream `18944` rejection, downstream display usage rewrite, and settings persistence/range validation.
- Verified with a real local Claude Code request through Antigravity API key `5`: upstream reported `raw_cached_tokens=7680`, the bridge generated `display_cached_tokens=14946` from `raw_input_tokens=22273` at `67.1041%`, usage row `15774` stored `model=requested_model=claude-opus-4-8`, `upstream_model=gpt-5.5`, `input_tokens=7327`, `cache_read_tokens=14946`, and downstream Claude Code display-mode usage showed `input_tokens=16149`, `cache_read_input_tokens=14946`, `output_tokens=188`.

## [2026-06-02] feat: merge upstream Antigravity Opus 4.8 support

**Affected files**: `backend/internal/domain/constants.go`, `backend/internal/pkg/antigravity/claude_types.go`, `backend/internal/pkg/antigravity/request_transformer.go`, `backend/internal/pkg/claude/constants.go`, `backend/internal/service/antigravity_model_mapping_test.go`, `backend/internal/service/bedrock_request_test.go`, `backend/migrations/146_add_opus48_to_model_mapping.sql`, `frontend/src/composables/useModelWhitelist.ts`, `frontend/src/components/account/AccountStatusIndicator.vue`, `frontend/src/components/account/AccountUsageCell.vue`, `docs/dev/UPSTREAM_SYNC.md`, `docs/dev/codebase/model-mapping.md`
**Upstream compatibility**: mirrors upstream `Wei-Shaw/sub2api` commit `514ac5c6` for `claude-opus-4-8`; migration filename is adapted from upstream `144_add_opus48_to_model_mapping.sql` to local `146_add_opus48_to_model_mapping.sql` because this fork already uses migration numbers 144 and 145.
**Change details**:
- Added `claude-opus-4-8` to Antigravity default mapping, exposed model list, request-transformer model metadata, and adaptive high-tier Opus detection.
- Added Bedrock default mapping for `claude-opus-4-8 -> us.anthropic.claude-opus-4-8-v1` with region-prefix adjustment coverage.
- Added frontend Claude/Antigravity model whitelist entries, preset mappings, account status alias, and Antigravity usage grouping.
- Added migration coverage for existing Antigravity accounts that already persist `credentials.model_mapping`, preserving unrelated local migration numbering.

## [2026-06-02] fix: normalize Antigravity system-role messages

**Affected files**: `backend/internal/pkg/antigravity/request_transformer.go`, `backend/internal/pkg/antigravity/request_transformer_test.go`, `docs/dev/CHANGELOG_CUSTOM.md`
**Upstream compatibility**: scoped Antigravity request-transformer compatibility fix; preserves existing top-level `system` handling while avoiding invalid Gemini `contents[].role=system` payloads.
**Change details**:
- Extracted `messages[].role=system` entries from Antigravity Claude requests before building Gemini `contents`, including case-insensitive `system` roles.
- Merged extracted text content into `systemInstruction` alongside top-level `system`, reusing existing OpenCode prompt and `x-anthropic-billing-header` filtering.
- Added focused transformer coverage proving downstream Gemini `contents` only contain `user`/`model` roles and message-level system text is preserved in `systemInstruction`.

## [2026-06-02] fix: reject negative user model pricing overrides

**Affected files**: backend/internal/service/user_model_pricing_service.go, backend/internal/service/user_model_pricing_service_test.go, backend/internal/handler/admin/user_model_pricing_handler.go, backend/migrations/147_user_model_pricing_non_negative_constraints.sql, frontend/src/components/admin/user/UserModelPricingModal.vue, docs/dev/codebase/billing.md
**Upstream compatibility**: scoped validation hardening for admin user-level model pricing; valid zero and positive prices remain supported.
**Change details**:
- Added service-layer validation for create, update, and batch upsert so user-level real/display price overrides cannot be negative, NaN, or infinite.
- Rejected non-positive or non-finite `display_rate_multiplier` for user model pricing overrides.
- Added PostgreSQL `NOT VALID` CHECK constraints to block new invalid writes without scanning historical rows during startup.
- Added focused unit coverage for the negative update path that can otherwise record negative usage costs.

## [2026-06-02] feat: add OpenAI Claude-GPT bridge for Antigravity groups

**Affected files**: backend/internal/service/account.go, backend/internal/service/admin_service.go, backend/internal/service/openai_account_scheduler.go, backend/internal/service/openai_gateway_service.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/server/routes/gateway.go, frontend/src/components/account/CreateAccountModal.vue, frontend/src/components/account/EditAccountModal.vue, frontend/src/components/account/BulkEditAccountModal.vue, frontend/src/components/common/GroupSelector.vue, frontend/src/types/index.ts, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, docs/dev/codebase/account.md, docs/dev/codebase/model-mapping.md, docs/dev/codebase/gateway.md, docs/dev/codebase/billing.md
**Upstream compatibility**: additive account-side routing feature; existing Antigravity subscriptions, API keys, and group platforms remain unchanged.
**Change details**:
- Added `extra.openai_claude_gpt_bridge_enabled` for OpenAI accounts and allowed enabled bridge accounts to bind Antigravity groups while still rejecting Anthropic/Gemini bindings.
- Reused existing `credentials.model_mapping` as the account-global Claude-to-GPT mapping source, requiring an explicit non-self mapping hit before bridge scheduling.
- Added Antigravity `/v1/messages` bridge preflight: eligible requests route through OpenAI `ForwardAsAnthropic`, while pre-upstream misses reset the request body and fall back to native Antigravity.
- Kept user-facing usage records and billing on the original Claude requested model while storing the GPT upstream model in `upstream_model` for admin visibility.
- Added admin account form controls for enabling the bridge and selecting OpenAI plus Antigravity groups when enabled.

## [2026-06-02] fix: make local Antigravity Claude-GPT bridge requests schedulable

**Affected files**: backend/internal/server/routes/gateway.go, backend/internal/repository/scheduler_cache.go, backend/internal/repository/scheduler_cache_unit_test.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_account_scheduler.go, backend/internal/handler/admin/account_handler_available_models_test.go, backend/internal/service/antigravity_model_mapping_test.go, backend/internal/server/api_contract_test.go, docs/dev/codebase/gateway.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped routing and scheduler metadata fix for the additive OpenAI Claude-GPT bridge; native Antigravity fallback remains unchanged when no eligible bridge account exists.
**Change details**:
- Reused the `/v1/messages` Anthropic Messages dispatch handler for `/antigravity/v1/messages`, so Claude Code configurations with `ANTHROPIC_BASE_URL=/antigravity` also preflight OpenAI bridge accounts.
- Preserved `extra.openai_claude_gpt_bridge_enabled` in slim scheduler metadata and added a bridge-only DB refresh path before stale scheduler snapshot candidates are rejected.
- Updated stale unit-test expectations for current OpenAI model-list merge behavior, Antigravity unknown Claude/Gemini passthrough, and handler/service constructor signatures.
- Preserved native Antigravity routing for bridge misses and kept `/antigravity/v1/messages/count_tokens`, `/models`, and `/usage` unchanged.
- Verified with a real local Claude Code-style request to `http://localhost:18081/antigravity/v1/messages`: `claude-opus-4-8` returned `200` through OpenAI account `41`, downstream response model stayed `claude-opus-4-8`, usage tokens were `23/19`, and the usage row stored `upstream_model=gpt-5.5`.

## [2026-06-02] fix: classify bridge cache status by request group platform

**Affected files**: backend/internal/repository/usage_log_repo.go, backend/internal/repository/usage_log_repo_request_type_test.go, docs/dev/codebase/billing.md, docs/dev/codebase/account.md
**Upstream compatibility**: scoped dashboard/statistics compatibility fix for the additive OpenAI Claude-GPT bridge; user billing, usage rows, scheduler selection, and native Antigravity AI Credits aggregation are unchanged.
**Change details**:
- Changed prompt-cache status platform filtering to prefer `groups.platform` over `accounts.platform`, so OpenAI bridge rows from Antigravity groups appear in the Antigravity cache-status dashboard.
- Treated `platform=all` as no platform filter in cache-status SQL, matching the existing handler/frontend semantics.
- Added unit coverage for the `all` filter and group-platform precedence.
- Documented that Antigravity AI Credits usage aggregation intentionally remains native Antigravity upstream-account scope, while bridge account-cost rules should target `platform=antigravity` plus the GPT upstream model or leave platform empty.

## [2026-06-02] docs: record OpenAI Claude-GPT bridge implementation notes

**Affected files**: docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md, docs/dev/codebase/README.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: documentation-only; records the custom OpenAI account-side bridge design, verification, and residual compatibility risks.
**Change details**:
- Added a dedicated bridge handoff document covering account configuration, eligibility, scheduler behavior, gateway routing, billing/usage rules, frontend behavior, and local real-request verification.
- Recorded residual issues for `/models`, `/messages/count_tokens`, Claude Code context compaction, Codex config isolation, and GPT upstream context-window limits.
- Linked the bridge document from the codebase documentation index for future maintenance.

## [2026-06-02] fix: normalize OpenAI cached tokens in Antigravity bridge usage

**Affected files**: backend/internal/handler/openai_gateway_handler.go, backend/internal/service/channel.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_record_usage_test.go, backend/internal/service/billing_service.go, backend/internal/service/pricing_service.go, backend/internal/service/billing_service_test.go, backend/internal/service/pricing_service_test.go, docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md, docs/dev/codebase/billing.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped to the custom OpenAI Claude-GPT bridge for Antigravity groups; ordinary OpenAI cache-read accounting remains unchanged.
**Change details**:
- Added a bridge usage flag so Antigravity Claude-GPT requests treat OpenAI `cached_tokens` as ordinary input tokens when writing usage records and calculating user billing.
- Prevented fixed OpenAI prompt/session cache values such as `18.9k` from appearing as Claude `cache_read_tokens` in usage records.
- Kept user-facing model and billing model on the original Claude request model while preserving `upstream_model=gpt-5.5` for admin visibility.
- Corrected local static fallback pricing so `gpt-5.5` no longer inherits `gpt-5.4` fallback prices, and added the missing `gpt-5.4-nano` fallback.
- Verified with focused unit tests and a real local `/antigravity/v1/messages` bridge request. This cache-zero behavior was later reverted by the follow-up cache-read preservation fix below.

## [2026-06-02] fix: preserve Claude-GPT bridge cache-read usage

**Affected files**: backend/internal/handler/openai_gateway_handler.go, backend/internal/service/channel.go, backend/internal/service/openai_gateway_messages.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_gateway_record_usage_test.go, docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md, docs/dev/codebase/billing.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: scoped to the custom OpenAI Claude-GPT bridge for Antigravity groups; ordinary OpenAI usage recording is unchanged.
**Change details**:
- Replaced the previous bridge cache-zero flag with a diagnostic-only bridge marker, so OpenAI `input_tokens_details.cached_tokens` is preserved as Anthropic-style `cache_read_tokens`.
- Restored the existing OpenAI token split for bridge usage: stored ordinary input tokens are `raw_input_tokens - cached_tokens`, and cache-read pricing uses the requested Claude model.
- Added bridge-only token diagnostics for raw upstream Responses usage, converted Anthropic usage, and final usage-log storage. These logs include request/account/model IDs and token counts only, not request or response content.
- Updated bridge billing docs to treat repeated values such as `18.9k` as a debugging target that must be traced to raw upstream, conversion, or storage before being accepted as normal.
- Verified with focused unit tests for bridge model billing and cache-read preservation.

## [2026-06-01] docs: record A2 Kiro Opus empty stream staged fix

**Affected files**: `docs/dev/KIRO_PROXY.md`, `docs/dev/CHANGELOG_CUSTOM.md`, `E:\cursor project\AIClient2API\docs\KIRO_OPUS_47_48_EMPTY_STREAM_DEBUG_2026-06-01.md`, `E:\cursor project\AIClient2API\docs\SUB2API_INTEGRATION.md`, `E:\cursor project\AIClient2API\docs\CHANGELOG_CUSTOM.md`, `E:\cursor project\AIClient2API\src\providers\claude\claude-kiro.js`, `E:\cursor project\AIClient2API\tests\kiro-stream-usage-estimation.test.js`
**Upstream compatibility**: Sub2API documentation-only; production behavior change is in the AIClient2API sidecar and keeps the same Sub2API route/API contract.
**Change details**:
- Recorded the investigation of intermittent empty Claude Code replies for Kiro `claude-opus-4-7` / `claude-opus-4-8`, including the key diagnostic where AIClient2API received stream bytes but parsed `jsonObjects=0`.
- Documented the staged AIClient2API parser fix: byte buffering, AWS event stream frame parsing, split-frame buffering, and `text` fallback compatibility.
- Recorded local verification: focused A2 tests passed, 18 local real `claude-opus-4-8` rows after restart had no `output_tokens=0`, and `claude-opus-4-6` still returned normal SSE content with usage row `15667`.

## [2026-06-01] fix: align downstream display usage cache balancing with usage logs

**Affected files**: `backend/internal/service/display_token_rewrite.go`, `backend/internal/service/display_token_rewrite_test.go`, `backend/internal/service/openai_gateway_service_test.go`, `docs/dev/codebase/billing.md`
**Upstream compatibility**: custom downstream display-token response behavior only; billing, stored usage logs, quota deduction, and real-mode downstream responses remain unchanged.
**Change details**:
- Changed downstream display usage rewriting to match usage-log display pricing for cache reads: cache-read token counts stay on the cache line, and lower display cache-read pricing is balanced by adding the cache premium to displayed input tokens.
- Kept user-group display rate scaling as a second step after model display-price balancing, so all token buckets scale consistently with usage records.
- Updated OpenAI Responses/Chat Completions tests so `cached_tokens` stays aligned with usage records while `input_tokens` and `total_tokens` still reflect display balancing.

## [2026-06-01] feat: extend downstream display usage tokens to OpenAI HTTP

**Affected files**: `backend/internal/service/display_token_rewrite.go`, `backend/internal/service/openai_gateway_service.go`, `backend/internal/service/openai_gateway_chat_completions.go`, `backend/internal/handler/openai_gateway_handler.go`, `backend/internal/handler/openai_chat_completions.go`, `docs/dev/codebase/billing.md`
**Upstream compatibility**: scoped custom downstream response behavior for user opt-in display token mode; billing, stored usage, actual cost, and OpenAI WebSocket/Image/Gemini paths remain unchanged.
**Change details**:
- Extended `users.downstream_usage_token_mode=display` from Claude/Antigravity to OpenAI HTTP `/v1/responses` and `/v1/chat/completions` downstream `usage` fields.
- Added OpenAI-specific usage rewriting that splits `cached_tokens` out of `input_tokens` and applies cache-read display multipliers only to cached input tokens.
- Kept real token accounting for `OpenAIForwardResult.Usage`, usage logs, quota deduction, and billing while rewriting only the bytes returned to the client.
- Reused the existing display pricing chain, including user model display pricing overrides and user-group display rate scaling, without using account cost multipliers.
- Added focused unit coverage for Responses/Chat Completions non-streaming, streaming, SSE-to-JSON fallback, cache-token math, real-mode no-op behavior, and include-usage behavior.

## [2026-06-01] fix: add Anthropic API-key passthrough stream keepalive

**Affected files**: `backend/internal/service/gateway_service.go`, `backend/internal/service/gateway_anthropic_apikey_passthrough_test.go`
**Upstream compatibility**: mirrors upstream `Wei-Shaw/sub2api` commit `164e2f61` for Anthropic API-key passthrough streaming keepalive; adapted to local display-usage rewrite logic.
**Change details**:
- Added downstream Anthropic-native `event: ping` keepalive events to API-key passthrough streams when `gateway.stream_keepalive_interval` is configured, preventing idle proxy/CDN disconnects during quiet upstream periods.
- Suppressed keepalive writes while an SSE event is partially forwarded so ping frames cannot interleave into an unfinished upstream event.
- Added focused tests for idle keepalive emission and partial-event non-interleaving.

## [2026-06-01] docs: clarify cross-repository agent rules

**Affected files**: `AGENTS.md`, `docs/dev/RELATED_PROJECTS.md`, `docs/dev/ARCHITECTURE.md`, `docs/dev/CHANGELOG_CUSTOM.md`, `E:\cursor project\AIClient2API\AGENTS.md`, `E:\cursor project\AIClient2API\docs\SUB2API_INTEGRATION.md`, `E:\cursor project\new-api\AGENTS.md`, `E:\cursor project\new-api\docs\SUB2API_INTEGRATION.md`, `E:\cursor project\new-api\web\default\AGENTS.md`, `E:\cursor project\InvokeAI\AGENTS.md`, `E:\cursor project\InvokeAI\docs\SUB2API_INTEGRATION.md`
**Upstream compatibility**: documentation and agent-rule boundaries only; no Sub2API runtime, database, API, or deployment behavior changes.
**Change details**:
- Added a Sub2API-side cross-repository index in `docs/dev/RELATED_PROJECTS.md` and pointed the main `AGENTS.md` and architecture docs at it.
- Clarified that `api2sub`, AIClient2API, new-api, and InvokeAI each use their own repository-root `AGENTS.md` as the rule entry point.
- Documented port ownership, startup boundaries, changelog ownership, and cross-repository contract update rules.
- Added or updated sibling-project Sub2API integration docs so future work started from a child repository still sees the correct Sub2API relationship.

## [2026-06-01] docs: require GHCR for future Sub2API main deploys

**Affected files**: AGENTS.md, docs/dev/ARCHITECTURE.md, docs/dev/DEPLOYMENT.md, docs/dev/PRODUCTION_CUSTOM_IMAGE_DEPLOY.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: documentation-only deployment rule change; no runtime behavior changes.
**Change details**:
- Recorded that future Sub2API main-service production deploys must use the GitHub Actions-built GHCR image ghcr.io/541968679/sub2api:latest or an explicitly approved tag.
- Marked the production-host docker build / sub2api-custom:latest path as legacy and no longer acceptable for future main-service deploys.
- Clarified that deploy/update.sh must not be used for Sub2API main-service deployment while it still builds sub2api-custom:*; sidecar-only GHCR pull flows remain documented separately.

## [2026-06-01] fix: migrate installed OpenAI GPT Image model dimensions

**Affected files**: `E:\cursor project\InvokeAI\invokeai\app\services\shared\sqlite_migrator\migrations\migration_33.py`, `E:\cursor project\InvokeAI\invokeai\app\services\shared\sqlite\sqlite_util.py`, `E:\cursor project\InvokeAI\tests\app\services\shared\sqlite_migrator\migrations\test_migration_33.py`
**Upstream compatibility**: InvokeAI fork-only SQLite migration; updates installed external OpenAI GPT Image model metadata so existing environments match the newer starter model capabilities.
**Change details**:
- Added migration 33 to update existing OpenAI GPT Image model records (`gpt-image-2`, `gpt-image-1.5`, `gpt-image-1`, `gpt-image-1-mini`) from fixed `aspect_ratio_sizes` / `allowed_aspect_ratios` to custom dimensions guarded by `max_image_size=4096x4096`.
- Root cause: starter model metadata changes only affect newly installed/synced models; already-installed local records kept old fixed-size metadata, so the frontend still hid quick size controls.
- Verified the local runtime database advanced to migration version 33 and `openai-gpt-image-2` now has `max_image_size=4096x4096` with fixed-size fields cleared.
- Verification: migration unit test `3 passed`; quick-size frontend tests `15 passed`; Ruff checks passed.

## [2026-06-01] chore: standardize InvokeAI local development startup

**Affected files**: `E:\cursor project\InvokeAI\scripts\dev-stack.ps1`, `E:\cursor project\InvokeAI\AGENTS.md`, `E:\cursor project\InvokeAI\invokeai\frontend\web\CLAUDE.md`, `E:\cursor project\InvokeAI\invokeai\frontend\web\vite.config.mts`, AGENTS.md
**Upstream compatibility**: local development tooling and documentation only; no Sub2API runtime behavior changes.
**Change details**:
- Changed InvokeAI local development to a single script-managed entry point that runs backend and frontend as separate managed processes.
- Added `-Service all|backend|frontend`, `-BackendPort`, and `-FrontendPort` support, with defaults `127.0.0.1:9090` and `127.0.0.1:15175`.
- Kept backend local config CPU/API-only and enabled `dev_reload: true`; frontend uses Vite HMR and proxies to the configured backend URL.
- Updated InvokeAI and Sub2API agent rules to forbid ad hoc `invokeai-web`, `pnpm dev`, or `make frontend-dev` startup for normal local development.
- Verified PowerShell script parsing, non-mutating `status`, frontend `pnpm run lint:tsc`, and a real script-managed restart with backend on `9090`, frontend on `15175`, and no Vite listener left on `5173`.

## [2026-06-01] fix: remove account rate from downstream display token rewrite

**Affected files**: backend/internal/service/display_token_rewrite.go, backend/internal/handler/gateway_handler.go, backend/internal/service/display_token_rewrite_test.go
**Upstream compatibility**: scoped bug fix for user-configured Claude/Antigravity downstream `usage` token display mode; billing and stored usage remain unchanged.
**Change details**:
- Removed the obsolete account rate multiplier from downstream display-token multiplier calculation.
- Kept downstream display token rewriting aligned to model display prices and user group display-rate scaling only.
- Added regression coverage so equal real/display prices produce a no-op multiplier even when legacy account rate data is high.

## [2026-06-01] fix: stabilize InvokeAI local frontend entrypoint

**Affected files**: E:\cursor project\InvokeAI\scripts\dev-stack.ps1, E:\cursor project\InvokeAI\invokeai\app\api_app.py, E:\cursor project\InvokeAI\invokeai\frontend\web\src\i18n.ts, E:\cursor project\InvokeAI\invokeai\frontend\web\src\app\store\enhancers\reduxRemember\driver.ts, E:\cursor project\InvokeAI\invokeai\frontend\web\src\app\components\AppErrorBoundaryFallback.tsx, E:\cursor project\InvokeAI\invokeai\frontend\web\src\common\components\Loading\Loading.tsx, E:\cursor project\InvokeAI\invokeai\frontend\web\src\common\components\InformationalPopover\constants.ts, E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\ui\components\Notifications.tsx, E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\system\components\InvokeAILogoComponent.tsx, E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\system\components\AboutModal\AboutModal.tsx, E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\nodes\components\sidePanel\workflow\WorkflowLibrary\WorkflowListItem.tsx, E:\cursor project\InvokeAI\AGENTS.md
**Upstream compatibility**: local development behavior only, except frontend static asset imports are made compatible with current Vite.
**Change details**:
- Made the managed local backend set `INVOKEAI_DEV_FRONTEND_URL`; when present, backend `/` redirects to `http://127.0.0.1:15175` instead of serving the bundled UI, while API routes on port 9090 continue to work.
- Replaced Vite 7-incompatible imports from `public/...` with public URL references and switched i18n to the existing HTTP backend path.
- Sorted touched frontend imports so the Vite ESLint overlay no longer blocks the local UI during development.
- Allowed unauthenticated client-state persistence reads/writes to no-op instead of blocking Redux rehydration, fixing the local 15175 page getting stuck on `Loading` before the login screen.
- Verified `pnpm run lint:tsc`, `ruff check invokeai/app/api_app.py`, `http://127.0.0.1:9090/` redirecting with 307, and `http://127.0.0.1:15175/` rendering the login page in the browser.

## [2026-06-01] fix: expose OpenAI Images upstream 400 errors

**Affected files**: backend/internal/handler/openai_images.go, backend/internal/service/openai_images_context.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/error_passthrough_runtime_test.go, docs/dev/codebase/gateway.md
**Upstream compatibility**: scoped OpenAI Images error mapping change; generic OpenAI Responses, Chat Completions, Anthropic, and Gemini gateway error masking remains unchanged.
**Change details**:
- Added an explicit Gin context marker for parsed `/v1/images/generations` and `/v1/images/edits` requests.
- Changed OpenAI gateway error handling so Images upstream 400 user errors return downstream 400 with the upstream `error.message` and `error.type` instead of generic 502.
- Kept the behavior independent of `OPENAI_IMAGE_TRACE_LOG`, which remains only an opt-in timing diagnostic.
- Added regression coverage for an upstream invalid image size error such as `4096x1752` not being divisible by 16.

## [2026-05-31] feat: user-level downstream usage token mode

**Affected files**: backend/ent/schema/user.go, backend/migrations/145_add_user_downstream_usage_token_mode.sql, backend/internal/service/display_token_rewrite.go, backend/internal/handler/gateway_handler.go, backend/internal/service/api_key_auth_cache*.go, backend/internal/handler/admin/user_handler.go, frontend/src/components/admin/user/UserEditModal.vue, frontend/src/types/index.ts, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: scoped custom behavior for Claude Messages / Antigravity downstream `usage` token fields; billing and stored usage remain unchanged.
**Change details**:
- Added `users.downstream_usage_token_mode` with `real` / `display` values and default `real`, plus Ent schema/generated code and migration 145.
- Added admin user API/DTO/frontend edit support so admins can opt specific users into display-token downstream responses.
- Added the mode to API key auth snapshots and bumped the snapshot version to rebuild old auth cache entries.
- Restored display token multiplier injection only when the authenticated user's mode is `display`; no-group API keys keep model display pricing and use group display scaling `1`.
- Extended display token multiplier calculation to merge user model display pricing overrides on top of global display pricing.
- Added focused unit coverage for admin updates, auth cache snapshots, and display token multipliers.

## [2026-05-31] fix: preserve InvokeAI external provider user context

**Affected files**: `E:\cursor project\InvokeAI\invokeai\app\services\external_generation\external_generation_default.py`, `E:\cursor project\InvokeAI\tests\app\services\external_generation\test_external_generation_service.py`, docs/dev/codebase/invokeai-poc.md, docs/dev/INVOKEAI_SIDECAR.md
**Upstream compatibility**: InvokeAI fork-only request-context fix; no Sub2API runtime or database behavior changes.
**Change details**:
- Fixed `ExternalGenerationService` request rebuilding so refreshed model capabilities and size bucketing preserve the original `ExternalGenerationRequest.user_id`.
- Root cause: the InvokeAI queue item and provider config were correctly scoped to the same user, but `_refresh_model_capabilities()` rebuilt the request without `user_id`, causing OpenAI multiuser config lookup to fail with `OpenAI provider is not configured for this user`.
- Replaced manual request reconstruction with `dataclasses.replace(...)` in both request-rebuild paths so future request fields are preserved automatically.
- Added regression coverage for preserving `user_id` during model capability refresh and request bucketization.
- Verification: `14 passed, 2 warnings` for `test_external_generation_service.py`; `13 passed, 2 warnings` for `test_external_provider_adapters.py`; `3 passed, 2 warnings` for `test_external_image_generation.py`.

## [2026-05-31] fix: allow custom OpenAI image dimensions in InvokeAI sidecar

**Affected files**: `E:\cursor project\InvokeAI\invokeai\backend\model_manager\starter_models.py`, `E:\cursor project\InvokeAI\tests\app\routers\test_model_manager.py`, `E:\cursor project\InvokeAI\tests\app\services\external_generation\test_external_generation_service.py`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\controlLayers\store\paramsSlice.test.ts`, docs/dev/codebase/invokeai-poc.md, docs/dev/INVOKEAI_SIDECAR.md
**Upstream compatibility**: InvokeAI fork-only external model metadata change; no Sub2API runtime or database behavior changes.
**Change details**:
- Removed fixed `aspect_ratio_sizes` / `allowed_aspect_ratios` presets from OpenAI GPT Image starter models so InvokeAI no longer locks width/height to preset resolutions in the advanced dimensions controls.
- Added a `4096x4096` maximum image size guard for OpenAI GPT Image starter models while preserving custom width/height passthrough to Sub2API.
- Kept fixed preset behavior for other external providers such as Gemini, Seedream, Qwen, and DALL-E where model metadata still declares presets.
- Verified backend and frontend regression coverage: `32 passed, 2 warnings in 12.32s`; `9 passed` for `paramsSlice.test.ts`.

## [2026-05-31] feat: add gpt-image-2 starter model to InvokeAI sidecar

**Affected files**: `E:\cursor project\InvokeAI\invokeai\app\services\external_generation\providers\openai.py`, `E:\cursor project\InvokeAI\invokeai\backend\model_manager\starter_models.py`, `E:\cursor project\InvokeAI\tests\app\services\external_generation\test_external_provider_adapters.py`, `E:\cursor project\InvokeAI\tests\app\services\external_generation\test_startup.py`, docs/dev/codebase/invokeai-poc.md, docs/dev/INVOKEAI_SIDECAR.md
**Upstream compatibility**: InvokeAI fork-only external provider change; no Sub2API runtime or database behavior changes.
**Change details**:
- Added `gpt-image-2` to the InvokeAI OpenAI external provider GPT Image model set so it uses the GPT Image payload shape with `output_format`.
- Added `external://openai/gpt-image-2` as an InvokeAI starter model so configured OpenAI/Sub2API providers can sync and install it from the UI/backend starter model flow.
- Documented that InvokeAI's OpenAI Base URL must be the Sub2API gateway origin without `/v1`, because the provider appends `/v1/images/generations` and `/v1/images/edits`.
- Verified with focused backend tests: `3 passed, 2 warnings in 0.29s`.

## [2026-05-31] feat: add InvokeAI sidecar deployment path

**Affected files**: deploy/docker-compose.yml, deploy/.env.example, deploy/update.sh, docs/dev/ARCHITECTURE.md, docs/dev/DEPLOYMENT.md, docs/dev/INVOKEAI_SIDECAR.md, `E:\cursor project\InvokeAI\.github\workflows\docker-publish.yml`
**Upstream compatibility**: deployment-only Sub2API change; no runtime gateway/database behavior changes. InvokeAI remains a separate sibling repository and is not vendored into Sub2API.
**Change details**:
- Added an `invokeai` Compose sidecar using `ghcr.io/541968679/invokeai-sub2api:latest`, loopback host bind `127.0.0.1:9090`, `/opt/invokeai/root` persistence, and CPU-only runtime settings.
- Extended `deploy/update.sh` with `--only-invokeai` and `--skip-invokeai`, while keeping the AIClient2API `--only-a2`/`--skip-a2` pattern.
- Added InvokeAI sidecar environment examples and deployment documentation, including the API-client-only rule: no GPU/CUDA/local model inference in this deployment.
- Added the InvokeAI GHCR workflow in the sibling InvokeAI repository; it builds `docker/Dockerfile` with `GPU_DRIVER=cpu` for `linux/amd64`.

## [2026-05-31] ops: expose InvokeAI public debug endpoint

**Affected files**: docs/dev/DEPLOYMENT.md, docs/dev/INVOKEAI_SIDECAR.md, production `/etc/caddy/Caddyfile`
**Upstream compatibility**: ops-only; no Sub2API runtime code changes.
**Change details**:
- Added production Caddy vhost `invokeai.172.245.247.80.sslip.io` reverse-proxying to loopback-only InvokeAI at `127.0.0.1:9090`.
- Verified public HTTPS access and `/api/v1/auth/status`; Caddy obtained a Let's Encrypt certificate automatically.
- Documented the public debug URL without recording any InvokeAI admin password or API key.

## [2026-05-31] fix: canonicalize OpenAI compact model aliases before billing

**Affected files**: backend/internal/service/openai_model_alias.go, backend/internal/service/openai_codex_transform.go, backend/internal/service/pricing_service.go, backend/internal/service/billing_service.go, backend/internal/service/openai_codex_transform_test.go, backend/internal/service/pricing_service_test.go, backend/internal/service/billing_service_test.go
**Upstream compatibility**: minimal upstream alias-normalization backport; low risk, pricing/billing lookup only
**Change details**:
- Added shared OpenAI/Codex model alias canonicalization so compact or namespaced spellings such as `gpt5.5` and `openai/gpt5.5` resolve to `gpt-5.5` before transform, static pricing, and billing fallback lookup.
- Preserved local GPT-5.5 Pro pricing by resolving `gpt5.5-pro` to `gpt-5.5-pro` before the generic GPT-5.5 fallback.
- Added unit coverage for compact GPT-5.5, GPT-5.4, and GPT-5.3 Codex aliases plus pricing fallback behavior.
- Verification: targeted service tests pass; full `go test -tags=unit ./...` still fails in pre-existing server constructor, admin handler, and Antigravity mapping tests unrelated to this patch.

## [2026-05-30] feat: enable InvokeAI API-only multi-image queue concurrency

**Affected files**: `E:\cursor project\InvokeAI\invokeai\app\services\session_processor\session_processor_default.py`, `E:\cursor project\InvokeAI\invokeai\app\services\session_queue\session_queue_sqlite.py`, `E:\cursor project\InvokeAI\invokeai\app\services\session_queue\session_queue_base.py`, `E:\cursor project\InvokeAI\invokeai\app\services\session_processor\session_processor_common.py`, `E:\cursor project\InvokeAI\invokeai\app\services\config\config_default.py`, `E:\cursor project\InvokeAI\invokeai\app\api\dependencies.py`, `E:\cursor project\InvokeAI\invokeai\app\api\routers\session_queue.py`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\services\api\endpoints\queue.ts`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\services\api\index.ts`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\services\events\setEventListeners.tsx`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\queue\hooks\useCancelCurrentQueueItem.ts`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\queue\hooks\useCurrentQueueItemDestination.ts`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\queue\hooks\useCurrentQueueItemId.ts`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\ui\layouts\DockviewTabCanvasViewer.tsx`, `E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\ui\layouts\DockviewTabCanvasWorkspace.tsx`, `E:\cursor project\InvokeAI\tests\app\services\test_session_processor_parallel.py`, `E:\cursor project\InvokeAI\tests\app\services\session_queue\test_session_queue_status_sequence.py`, `E:\cursor project\InvokeAI\tests\app\services\session_queue\test_session_queue_status_event_isolation.py`, `E:\cursor project\InvokeAI\tests\app\services\session_queue\test_session_queue_clear.py`, docs/dev/codebase/invokeai-poc.md, docs/dev/codebase/README.md
**Upstream compatibility**: InvokeAI PoC sidecar behavior change; no Sub2API runtime or database changes. Potential upstream conflict area is InvokeAI queue/session processor internals and queue UI state.
**Change details**:
- Replaced InvokeAI's single session processor worker with a configurable worker pool; `session_queue_concurrency` defaults to `4` for API-only multi-image generation.
- Made SQLite queue dequeue atomically promote pending rows to `in_progress`, added `get_current_items`, and preserved old single-current compatibility fields.
- Updated queue cancellation/clear behavior for multiple active items so non-admin actions remain scoped to that user's queue items.
- Added `GET /api/v1/queue/{queue_id}/current_items` and updated React queue hooks/progress indicators to use all active items where needed.
- Added focused backend regression coverage for parallel execution, concurrency limits, worker wake-up, multi-current cancellation, redaction, and clear scoping.
- Verified backend with `31 passed, 2 warnings in 5.56s`; verified frontend with `pnpm run lint:tsc` exit code 0.

## [2026-05-30] feat: add account group select-all control

**Affected files**: frontend/src/components/common/GroupSelector.vue, frontend/src/components/account/CreateAccountModal.vue, frontend/src/components/account/EditAccountModal.vue, frontend/src/components/account/BulkEditAccountModal.vue, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, frontend/src/components/common/__tests__/GroupSelector.spec.ts, docs/dev/codebase/account.md
**Upstream compatibility**: frontend-only account management UI enhancement; no API or database changes
**Change details**:
- Added an optional select-all / deselect-all control to the shared group selector.
- Enabled the control in account creation, account editing, and account bulk editing group sections.
- Kept the control scoped with `show-toggle-all` so other `GroupSelector` reuse sites keep their previous UI.
- Preserved platform-filtered behavior: select-all only adds currently selectable groups, and deselect-all only removes currently selectable groups.
- Added focused Vitest coverage and updated account module documentation.

## [2026-05-30] docs: record gpt-image-2 timeout fix retest

**Affected files**: docs/dev/OPENAI_IMAGE_TIMEOUT_RETEST_2026-05-30.md, docs/dev/ARCHITECTURE.md, docs/dev/codebase/README.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Added a standalone record for the `gpt-image-2` non-return / latency fix, including problem boundary, code behavior, verification commands, retest matrix, and post-fix conclusions.
- Captured the 36-request retest summary: concurrency 4, 2K/4K x auto/medium/high, 36/36 success, no fast failures, no client timeouts, no service timeouts, max duration 65.578s.
- Documented current timeout guidance, client timeout recommendations, and the next larger-sample analysis plan for future optimization.
- Linked the new retest record from the architecture navigation and codebase module index.

## [2026-05-30] fix: bound gpt-image-2 OAuth generation waits and retry fast transport failures

**Affected files**: backend/internal/service/openai_images_responses.go, backend/internal/service/openai_images_test.go, backend/internal/handler/openai_images.go, docs/dev/codebase/gateway.md
**Upstream compatibility**: OpenAI Images OAuth gateway behavior change only; no database schema or API contract expansion beyond clearer error types
**Change details**:
- Added bounded generation windows for the Codex `/responses` image tool path: 1K 180s, 2K 240s, and 4K/unknown 360s.
- Added short retry handling for fast no-header transport failures such as EOF / connection reset / forcibly closed upstream connections, up to 3 total attempts on the same account.
- Added typed client-facing image errors: `image_generation_timeout` as 504 for long no-output waits and `image_generation_upstream_unreachable` as 502 for transport retry exhaustion.
- Preserved non-streaming behavior so timeout errors are returned before any response body is written; streaming requests emit a typed SSE error if the timeout happens after streaming starts.
- Added focused service tests covering retry success, retry exhaustion, and non-streaming timeout behavior.

## [2026-05-29] fix: repair official WeChat Pay checkout fallback

**Affected files**: backend/internal/payment/provider/wxpay.go, backend/internal/payment/provider/wxpay_test.go, backend/internal/service/payment_order.go, backend/internal/service/payment_order_result_test.go, frontend/src/components/payment/providerConfig.ts, frontend/src/components/payment/__tests__/providerConfig.spec.ts, docs/dev/codebase/payment.md, docs/dev/codebase/README.md
**Upstream compatibility**: payment subsystem bug fix; no database schema changes; provider config adds optional WeChat scene fields
**Change details**:
- Restored optional official WeChat Pay admin fields for `mpAppId`, `h5AppName`, and `h5AppUrl`, matching backend support and existing i18n guidance.
- Added official WeChat H5-to-Native fallback so merchants without H5 permission can still return a desktop-scan QR code instead of failing checkout.
- Classified common WeChat H5 and JSAPI upstream errors into explicit frontend-facing reasons instead of generic `PAYMENT_GATEWAY_ERROR`.
- Added focused Go and Vitest coverage for the WeChat fallback, error classification, and provider config field exposure.
- Added `docs/dev/codebase/payment.md` documenting payment data flow, provider files, WeChat JSAPI/H5/Native behavior, and production pitfalls.

## [2026-05-29] fix: fallback Kiro Opus 4.8 stream usage accounting

**Affected files**: `E:\cursor project\AIClient2API\src\providers\claude\claude-kiro.js`, `E:\cursor project\AIClient2API\tests\kiro-stream-usage-estimation.test.js`, `docs/dev/KIRO_PROXY.md`
**Upstream compatibility**: AIClient2API sidecar-only runtime fix plus Sub2API documentation; no Sub2API gateway code changes
**Change details**:
- Diagnosed `claude-opus-4-8` Claude Code CLI failures where Kiro stream usage sometimes omitted `contextUsagePercentage`, causing Sub2API usage rows to record zero output tokens.
- Preserved the existing cache-read estimation path and added lightweight AIClient2API fallbacks: estimate input tokens from the request body when Kiro usage stats are missing, then estimate output tokens from already-emitted stream characters only if normal output token counting still returns zero.
- Kept the fallback cheap: no tokenizer per stream chunk, only string length accumulation during emitted text/thinking/tool deltas and one final `ceil(chars / 4)` calculation.
- Verified with focused Jest coverage and a local Sub2API passthrough request; new usage row `15242` recorded `input_tokens=2584`, `output_tokens=1`, and `cache_read_tokens=4417`.
- Recorded the Kiro/AIClient2API troubleshooting conclusion in `docs/dev/KIRO_PROXY.md`; AIClient2API commits: `bf5c750` and `d2d337c`.

## [2026-05-29] fix: add AIClient2API Claude Opus 4.8 Kiro model support

**Affected files**: `E:\cursor project\AIClient2API\src\providers\claude\claude-kiro.js`, `E:\cursor project\AIClient2API\src\providers\provider-models.js`
**Upstream compatibility**: mirrors official AIClient2API upstream commit `66950dc` for the Opus 4.8 model entries only; avoids merging unrelated AtlasCloud and UI changes
**Change details**:
- Added `claude-opus-4-8` to the Kiro provider model list.
- Added the Kiro upstream mapping `claude-opus-4-8 -> claude-opus-4.8`.
- Added a 1,000,000 token context window entry for Opus 4.8 and restarted the local dev stack.

## [2026-05-29] fix: validate EasyPay API base URL

**Affected files**: backend/internal/payment/provider/easypay.go, backend/internal/payment/provider/easypay_refund_test.go, frontend/src/views/user/paymentUx.ts, frontend/src/views/user/__tests__/paymentUx.spec.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: low risk; rejects invalid EasyPay runtime configuration earlier
**Change details**:
- Added EasyPay `apiBase` validation so enabled instances must use an absolute `http(s)` URL and cannot save values like `11` that later become `11/mapi.php`.
- Kept endpoint-path normalization for valid EasyPay URLs such as `/mapi.php`, `/submit.php`, and `/api.php`.
- Stopped mapping provider misconfiguration errors to the generic WeChat unavailable prompt, allowing the real configuration error to surface.

## [2026-05-29] fix: repair WeChat Pay mobile QR fallback

**Affected files**: backend/internal/handler/payment_handler.go, backend/internal/service/payment_order.go, backend/internal/service/payment_service.go, backend/internal/service/payment_order_result_test.go, frontend/src/components/payment/paymentFlow.ts, frontend/src/components/payment/__tests__/paymentFlow.spec.ts, frontend/src/types/payment.ts, frontend/src/views/user/PaymentView.vue, frontend/src/views/user/__tests__/PaymentView.spec.ts, docs/dev/codebase/payment.md
**Upstream compatibility**: low risk; scoped to official WeChat checkout request routing and mobile QR fallback
**Change details**:
- Added explicit `is_wechat_browser` request context so the backend can honor frontend overrides instead of always trusting the WeChat User-Agent.
- Added `force_native_qr` for WeChat mobile fallback; when set, backend clears OpenID/mobile/WeChat context after resume-token restoration so the order uses Native QR instead of returning OAuth/JSAPI again.
- Preserved `wechat_resume_token` on the fallback request so OAuth callback orders keep their original amount, order type, and plan context.
- Added frontend and backend regression coverage for the WeChat mobile fallback request shape and force-native normalization.

## [2026-05-28] docs: clarify new-api sibling subproject relationship

**Affected files**: AGENTS.md, DEV_GUIDE.md, docs/dev/ARCHITECTURE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Clarified that `E:\cursor project\new-api` is an optional sibling subproject managed by local tooling, not a Git submodule.
- Documented that the current scope is local dev-stack orchestration only, with production deployment and Sub2API gateway/account wiring deferred to follow-up work.
- Recorded the generated compose file location and the rule to avoid changing `new-api/docker-compose.dev.yml` just for local port conflicts.

## [2026-05-28] chore: add optional new-api local subproject integration

**Affected files**: scripts/dev-stack.ps1, AGENTS.md, DEV_GUIDE.md, docs/dev/ARCHITECTURE.md
**Upstream compatibility**: local development tooling and documentation only; no Sub2API runtime behavior changes
**Change details**:
- Added optional `-IncludeNewAPI`, `-NewAPIPath`, and `-NewAPIPort` support to the local dev-stack script.
- Starts the sibling `E:\cursor project\new-api` backend through a generated Docker Compose file instead of modifying the new-api checkout.
- Maps new-api to `127.0.0.1:13200` by default to avoid the existing AIClient2API `3000/3100` ports.
- Documented the new optional subproject port and startup command in the agent entry point, development guide, and architecture pitfalls.

## [2026-05-25] feat: manage distribution API key lifecycle

**Affected files**: backend/internal/service/distribution.go, backend/internal/repository/distribution_repo.go, backend/internal/handler/distribution_handler.go, backend/internal/server/routes/user.go, backend/internal/server/routes/admin.go, backend/internal/service/user_service.go, backend/internal/repository/migrations_runner.go, backend/internal/repository/migrations_runner_checksum_test.go, backend/migrations/144_distribution_api_key_recharge_wallet_totals.sql, frontend/src/api/distribution.ts, frontend/src/api/admin/distribution.ts, frontend/src/types/index.ts, frontend/src/views/user/DistributionView.vue, frontend/src/views/admin/DistributionView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/distribution.md
**Upstream compatibility**: distribution API/UI behavior change; additive routes with legacy `/void` retained as disable-only compatibility
**Change details**:
- Added user/admin distribution API-key asset operations for recharge, disable, enable, and remaining-quota refund.
- Changed legacy distribution asset void behavior to disable/expire assets without wallet refund, and moved API-key refund semantics to explicit `/refund` routes.
- Added API-key asset list fields for key name, quota used, quota remaining, tracked exchange rate, and estimated refundable RMB.
- Added wallet total-spend repair migration for historical API-key recharge ledger actions.
- Updated user/admin distribution pages with lifecycle actions, localized strings, and refund/recharge wallet refresh behavior.

## [2026-05-25] fix: correct distribution asset refund accounting

**Affected files**: backend/internal/service/distribution.go, backend/internal/repository/distribution_repo.go, backend/migrations/143_recompute_distribution_wallet_totals.sql, docs/dev/codebase/distribution.md
**Upstream compatibility**: distribution wallet accounting and data repair migration
**Change details**:
- Changed distribution wallet lifetime counters so asset refunds restore balance without increasing `total_recharged`; only positive admin adjustments count as recharge, and only generation actions count as spend.
- Allowed distribution API-key void/refund finalization when the underlying unused API key was already disabled or soft-deleted outside the distribution asset flow, while rejecting keys with nonzero `quota_used`.
- Added an idempotent migration to recompute historical wallet totals from ledger actions and backfill refunds for unused distribution API-key assets whose underlying keys were already disabled/deleted without asset refund records.

## [2026-05-25] feat: optimize become-agent asset history layout

**Affected files**: frontend/src/views/user/DistributionView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/distribution.md
**Upstream compatibility**: frontend-only user distribution page layout change; distribution APIs unchanged
**Change details**:
- Removed the separate generated-results section and moved recently generated codes/API keys into the generated-assets action area for immediate copy.
- Combined generated assets and wallet ledger into one tabbed history panel.
- Added debounced generated-asset search using the existing user asset-list search parameter, with localized placeholders and empty states.

## [2026-05-25] fix: avoid i18n placeholder parsing in distribution API key copy text

**Affected files**: frontend/src/views/user/DistributionView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: frontend-only bug fix
**Change details**:
- Moved the generated API key curl JSON example out of the vue-i18n message string so `{"model":...}` is no longer parsed as an i18n placeholder in production builds.
- Kept translatable sentence fragments for the API key usage instructions and assembled the full copy text in code.

## [2026-05-25] feat: align public key usage page with user usage view

**Affected files**: backend/internal/server/middleware/api_key_auth.go, backend/internal/server/routes/gateway.go, backend/internal/handler/usage_handler.go, frontend/src/views/KeyUsageView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/gateway.md
**Upstream compatibility**: additive public usage endpoints and frontend-only public page redesign
**Change details**:
- Added API-key-authenticated `/v1/usage/records`, `/v1/usage/stats`, and `/v1/usage/trend` endpoints for the public usage page.
- Kept public usage endpoints outside billing and group-assignment enforcement so exhausted, expired, or ungrouped keys can inspect their own usage.
- Forced public records/stats/trend queries to the authenticated API key ID and user ID instead of accepting a user-controlled key selector.
- Reworked `/key-usage` into an unbranded usage-records view matching the signed-in `/usage` layout style, with the API key selector removed and replaced by a direct API key input.
- Removed public-page brand/logo/docs/GitHub/footer/home navigation surfaces and added localized labels for the new query controls.

## [2026-05-25] fix: disable key-usage brand home navigation

**Affected files**: frontend/src/views/KeyUsageView.vue
**Upstream compatibility**: frontend-only public page navigation tweak
**Change details**:
- Changed the `/key-usage` page header brand from a `/home` router link into static branding so clicking ZeroCode no longer opens the old home page.

## [2026-05-25] feat: expose public API key usage query entry

**Affected files**: backend/internal/server/routes/gateway.go, backend/internal/server/routes/gateway_test.go, frontend/src/views/HomeView.vue, frontend/src/views/auth/LoginView.vue, frontend/src/router/index.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/gateway.md
**Upstream compatibility**: additive public entry and route-order change for `/v1/usage`; model gateway calls remain group-checked
**Change details**:
- Kept `/v1/usage` behind API key authentication but moved it before the gateway group-assignment middleware so exhausted, expired, or ungrouped keys can still query their own usage.
- Added public homepage and login-page links to the existing `/key-usage` page so users can find the API key usage query without signing in.
- Added localized labels and a route title key for the public usage page.
- Documented the public usage query flow and added route coverage for ungrouped keys.

## [2026-05-25] feat: promote become-agent entry points

**Affected files**: frontend/src/components/layout/AppSidebar.vue, frontend/src/components/user/dashboard/UserDashboardQuickActions.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: frontend-only navigation and dashboard promotion change; distribution APIs unchanged
**Change details**:
- Moved the user-side "Become an Agent" menu entry directly below Usage in the sidebar.
- Added a highlighted sidebar treatment with subtle shine and a HOT badge for the agent entry.
- Added a prominent quick-action banner on the user dashboard linking to the agent application page.

## [2026-05-25] feat: rename and explain user agent application page

**Affected files**: frontend/src/views/user/DistributionView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: frontend-only user distribution page copy and layout change; user/admin distribution APIs unchanged
**Change details**:
- Renamed the user-side distribution entry and page title to "Become an Agent" / "成为代理" while leaving admin distribution management unchanged.
- Added an application-page explanation of the agent model, covering low-cost supply, fast delivery, and asset/customer management benefits.
- Replaced the approved-state application record card with an agent usage guide and kept the application record visible only for non-approved states.

## [2026-05-25] docs: expand Codex Desktop tutorial setup

**Affected files**: docs/API_USAGE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Replaced the terse Codex Desktop installation note with actionable download, platform selection, and installation guidance.
- Clarified that ZeroCode setup should use CC-Switch first, then restart Codex Desktop so it reads the shared `.codex/config.toml` and `.codex/auth.json` files.
- Added an explicit jump from the Codex Desktop install section to the existing `4.3.1` CC-Switch configuration flow.

## [2026-05-25] docs: align Codex tutorial structure with Claude Code chapter

**Affected files**: docs/API_USAGE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Reworked chapter 4 into separate `CLI 版本：安装与配置` and `Desktop 桌面版：安装与配置` sections, matching chapter 3's version-based tutorial structure.
- Moved Codex CLI installation, CC-Switch setup, manual configuration, WebSocket option, and verification into one CLI flow.
- Added a full Codex Desktop flow for install, CC-Switch configuration, local project startup, and Desktop-specific troubleshooting.

## [2026-05-25] docs: make API Keys CCS import the primary setup path

**Affected files**: docs/API_USAGE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Updated Claude Code CLI, Codex CLI, and Codex Desktop setup flows to use the API Keys page `导入到 CCS` action as the primary configuration method.
- Clarified that the API Keys import action maps Anthropic groups to Claude Code, OpenAI groups to Codex, and Gemini groups to Gemini CLI.
- Reframed manual file copying and the `使用` modal as fallback paths; Claude Code Desktop remains the manual application-level setup path.

## [2026-05-25] feat: restrict distribution API key groups

**Affected files**: backend/internal/service/distribution.go, backend/internal/service/api_key_service.go, backend/internal/handler/distribution_handler.go, backend/internal/server/routes/user.go, backend/internal/service/domain_constants.go, backend/internal/service/setting_service.go, frontend/src/views/admin/DistributionView.vue, frontend/src/views/user/DistributionView.vue, frontend/src/api/distribution.ts, frontend/src/api/admin/distribution.ts, frontend/src/types/index.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/distribution.md
**Upstream compatibility**: distribution settings/API behavior change; existing unset configs now expose no API key groups to agents
**Change details**:
- Added `distribution_api_key_group_ids` Settings KV to let admins select active standard groups exposed to distribution agents.
- Added `GET /api/v1/distribution/api-key-groups` and changed the agent page to use it instead of `/groups/available`.
- Enforced the whitelist in distribution API key generation and added a distribution-specific key creation path so the whitelist, not the agent user's own group permissions, is the permission source.
- Added admin UI multi-select, i18n strings, and distribution module documentation.

## [2026-05-24] fix: hide user-facing cache-write usage display

**Affected files**: frontend/src/views/user/UsageView.vue, frontend/src/components/user/usage/UsageMetricTrendChart.vue, frontend/src/components/user/dashboard/UserDashboardStats.vue, frontend/src/components/user/dashboard/UserDashboardCharts.vue, frontend/src/components/charts/TokenUsageTrend.vue, frontend/src/views/KeyUsageView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: frontend-only user-facing display change; cache-write billing fields and admin configuration remain unchanged
**Change details**:
- Removed cache-write/cache-creation as a selectable metric from the user usage trend chart.
- Hid cache-write/cache-creation token and cost breakdown rows in the user usage records table and tooltips.
- Hid cache-creation totals from the user dashboard and public API-key usage query while keeping cache-read display.
- Added focused frontend regression coverage for user usage chart and tooltip output.

## [2026-05-24] fix: keep usage records table visible under trend chart

**Affected files**: frontend/src/components/layout/TablePageLayout.vue, frontend/src/views/user/UsageView.vue
**Upstream compatibility**: frontend-only layout fix; usage APIs unchanged
**Change details**:
- Added a scroll-area header slot to the shared table layout and moved the user usage trend chart out of the fixed filters section so the records table keeps visible scroll height.
- Added page-scroll mode to the shared table layout and enabled it for the user usage page so the full usage page scrolls naturally instead of compressing the records table into a fixed viewport.
- Removed the CSV export button and user usage CSV export logic from the usage records page.

## [2026-05-24] feat: add user usage trend chart

**Affected files**: backend/internal/handler/usage_handler.go, backend/internal/service/usage_service.go, frontend/src/views/user/UsageView.vue, frontend/src/components/user/usage/UsageMetricTrendChart.vue, frontend/src/api/usage.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: additive user usage UI and trend API filter change; existing usage list/stats behavior unchanged
**Change details**:
- Added a compact usage trend chart above the user usage records table that follows the current API key and date-range filters.
- Fixed the user dashboard trend endpoint to accept optional `api_key_id` with ownership validation, so chart data can match filtered usage records.
- Added selectable chart metrics with total actual cost and total tokens always shown, plus at most two optional extra metrics.
- Added focused backend and frontend tests for API-key-filtered trend data and metric-selection limits.

## [2026-05-24] fix: compact API keys getting started guide

**Affected files**: frontend/src/components/keys/GettingStartedGuide.vue, frontend/src/views/user/KeysView.vue
**Upstream compatibility**: frontend-only API keys page presentation change; key management behavior unchanged
**Change details**:
- Replaced the API keys page getting-started guide's large header-plus-card layout with a compact responsive action bar.
- Kept the create key, CC Switch download, tool hints, and dismiss actions while removing the tall descriptive step cards.
- Merged search, group/status filters, refresh, and create-key actions into one responsive toolbar line.
- Reduced the page gap above the API keys table so more vertical space is available for the table.

## [2026-05-23] fix: enlarge login marketing cards and reduce heading gap

**Affected files**: frontend/src/views/auth/LoginView.vue
**Upstream compatibility**: frontend-only login page presentation change
**Change details**:
- Replaced the login marketing panel's space-between layout with a fixed-gap vertical flow so the heading no longer floats far above the cards.
- Increased feature card minimum height, padding, icon size, title size, and description size so each card carries more visual weight.

## [2026-05-23] feat: simplify login marketing cards and add gpt-image-2 promotion

**Affected files**: frontend/src/views/auth/LoginView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: frontend-only login page presentation change; auth flow unchanged
**Change details**:
- Reduced the desktop login marketing panel from six compact feature cards to four equal 2x2 cards.
- Removed the visible "official-grade service quality" card from the login page messaging.
- Added a dedicated gpt-image-2 image generation card with Chinese and English copy and highlight terms.
- Increased card spacing, minimum height, icon size, and copy rhythm so the left panel reads less crowded.

## [2026-05-23] fix: compact subscription purchase layout

**Affected files**: frontend/src/views/user/PaymentView.vue, frontend/src/components/payment/SubscriptionPlanCard.vue
**Upstream compatibility**: frontend-only layout density change; subscription order flow unchanged
**Change details**:
- Compressed the active-subscription area into a compact horizontal summary so it no longer dominates the subscription tab.
- Changed subscription plan browsing to a denser 3-column desktop grid.
- Reduced plan card height, price scale, quota spacing, and feature rows so the desktop view can show at least six plans at once.

## [2026-05-23] refactor: restore purchase page tab layout

**Affected files**: frontend/src/views/user/PaymentView.vue, frontend/src/components/payment/SubscriptionPlanCard.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: frontend-only layout change; payment APIs and order flow unchanged
**Change details**:
- Restored the purchase page to a unified tab layout with separate recharge and subscription tabs across desktop and mobile.
- Relaxed the recharge flow into account, bonus, amount/method, and credit-summary sections instead of a tight two-column checkout.
- Relaxed subscription plan cards and the subscription confirmation flow with wider cards, larger price treatment, expanded quota/features, and active-subscription summary cards.

## [2026-05-22] fix: prevent production deploy from restarting with upstream image

**Affected files**: deploy/docker-compose.yml, deploy/.env.example, deploy/update.sh, docs/dev/PRODUCTION_CUSTOM_IMAGE_DEPLOY.md
**Upstream compatibility**: production deploy safety fix; default public compose image remains configurable
**Change details**:
- Made the Sub2API compose image configurable through `SUB2API_IMAGE` instead of hard-coding `weishaw/sub2api:latest`.
- Updated `deploy/update.sh` to generate a controlled `docker-compose.override.yml` that pins production restarts to the locally built `sub2api-custom:latest` image.
- Forced Sub2API container recreation on main-app deploys so Docker Compose cannot reuse a container created from an older image ID.
- Added post-deploy image-name and image-ID verification so deployments fail and rollback if Compose starts a different image than the one just built.
- Documented that production deployments must verify both health and the running `sub2api` image.

## [2026-05-22] feat: add admin subscription quota adjustment

**Affected files**: backend/internal/service/subscription_service.go, backend/internal/service/user_subscription_port.go, backend/internal/repository/user_subscription_repo.go, backend/internal/handler/admin/subscription_handler.go, backend/internal/server/routes/admin.go, frontend/src/views/admin/SubscriptionsView.vue, frontend/src/api/admin/subscriptions.ts, frontend/src/types/index.ts, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: admin-only feature; preserves existing subscription quota data model
**Change details**:
- Added `POST /api/v1/admin/subscriptions/:id/adjust-quota` to set daily, weekly, and/or monthly used quota values for a user subscription.
- Invalidates subscription billing caches after manual quota adjustments so gateway eligibility uses the updated usage immediately.
- Added an admin subscription-management dialog for target remaining quota or target used quota, with zh/en UI strings.
- Added unit coverage for selected usage updates and invalid input handling.

## [2026-05-19] ops(aiclient2api): align production deploy with CI-built image flow

**Affected files**: `deploy/.env.example`, `deploy/docker-compose.yml`, `deploy/update.sh`, `docs/dev/DEPLOYMENT.md`, `docs/dev/KIRO_PROXY.md`, `docs/dev/CHANGELOG_CUSTOM.md`
**Upstream compatibility**: deployment-only change for the AIClient2API sidecar; Sub2API application behavior is unchanged
**Change details**:
- Changed the production `aiclient2api` service to use `ghcr.io/541968679/aiclient2api:latest` by default, with `AICLIENT2API_IMAGE` available for overrides.
- Added `AICLIENT2API_IMAGE` to the deployment environment example.
- Reworked `update.sh --only-a2` to pull the CI-built image through Docker Compose and restart the sidecar instead of building AIClient2API on the production host.
- Updated deployment/Kiro docs to record the CI image flow, GHCR pull access requirement, and remove the stale A2 on-host build instructions.

## [2026-05-19] docs(deploy): record AIClient2API production sidecar quick reference

**Affected files**: `docs/dev/DEPLOYMENT.md`, `docs/dev/CHANGELOG_CUSTOM.md`
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Added the production server, SSH key path, server-side source/config paths, image name, deploy log, and common A2 deploy commands to the deployment handbook.
- Documented post-deploy verification commands for `docker compose ps`, `aiclient2api` logs, and `/opt/sub2api/deploy.log`.
- Clarified that production AIClient2API is a Sub2API Compose sidecar bound to `127.0.0.1:3000`, while Sub2API reaches it through Docker DNS at `http://aiclient2api:3000/claude-kiro-oauth`.

## [2026-05-19] ops(aiclient2api): add optional sing-box proxy sidecar

**Affected files**: `deploy/docker-compose.a2-proxy.yml`, `deploy/a2-proxy/sing-box.config.json.example`, `docs/dev/KIRO_PROXY.md`
**Upstream compatibility**: deployment-only optional overlay; default compose and runtime behavior are unchanged
**Change details**:
- Added an optional `a2-proxy` sing-box sidecar compose overlay for AIClient2API upstream proxy testing.
- Added a direct-only sing-box config template with internal HTTP (`10809`) and SOCKS (`10808`) inbounds, ready for later outbound node replacement.
- Documented production activation steps and the correct Docker-internal A2 proxy URL (`http://a2-proxy:10809`).

## [2026-05-19] docs: record OpenAI image timing diagnostics progress

**Affected files**: `docs/dev/OPENAI_IMAGE_TIMING_DIAGNOSTICS_2026-05-19.md`, `docs/dev/ARCHITECTURE.md`, `docs/dev/codebase/README.md`
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Added a standalone progress document for the `gpt-image-2` latency investigation, including local trace setup, observed request IDs, timing breakdown, and conclusions.
- Documented the current finding that the successful local baseline spent nearly all server-side time waiting for upstream image result/body data.
- Linked the progress document from the architecture navigation and gateway module index so it is reachable from the documentation root.

## [2026-05-18] feat: add opt-in OpenAI image timing trace logs

**Affected files**: backend/internal/handler/openai_images.go, backend/internal/handler/openai_gateway_handler.go, backend/internal/service/openai_image_trace.go, backend/internal/service/openai_images.go, backend/internal/service/openai_images_responses.go, backend/internal/service/openai_gateway_service.go, backend/internal/service/openai_images_test.go, docs/dev/codebase/gateway.md
**Upstream compatibility**: low risk; disabled by default and scoped to `/v1/images/generations` with `model=gpt-image-2`
**Change details**:
- Added `OPENAI_IMAGE_TRACE_LOG=true` gated structured events for image request timing: request received, auth done, account slot acquired, upstream start/headers/body done, downstream response built/write done, and usage task submitted.
- Kept trace fields limited to safe correlation and timing values; prompts, image/base64 payloads, auth headers, cookies, API keys, and full request bodies are not logged.
- Covered trace gating and safe fields with focused unit coverage, and documented the temporary diagnostic workflow in the gateway module notes.

## [2026-05-18] fix: align OpenAI OAuth image forwarding headers with account test path

**Affected files**: backend/internal/service/openai_images_responses.go, backend/internal/service/openai_images_test.go
**Upstream compatibility**: low risk; scoped to OAuth-backed OpenAI image generation/edit forwarding
**Change details**:
- Changed OAuth image forwarding to build a dedicated Codex `/responses` upstream request matching the successful account-test image path.
- Stopped propagating third-party client `User-Agent`, `originator`, `session_id`, and `conversation_id` headers into image OAuth upstream requests; default User-Agent now falls back to Codex CLI when the account has no custom UA.
- Added coverage proving OAuth image forwarding sends `originator=opencode`, Codex CLI UA, and no session/conversation headers.

## [2026-05-17] docs(poc): link InvokeAI canvas validation setup

**Affected files**: `docs/dev/codebase/README.md`, `docs/dev/codebase/invokeai-poc.md`
**Upstream compatibility**: documentation-only; no Sub2API runtime behavior changes
**Change details**:
- Documented the external InvokeAI source checkout and runtime root used for the canvas PoC.
- Recorded the intended integration flow: InvokeAI runs independently on port 9090 and calls Sub2API's OpenAI-compatible image API on port 18081.
- Captured local startup command, API key placeholder, and known PoC pitfalls for `gpt-image-2` validation.

## [2026-05-17] feat: InvokeAI per-user external OpenAI provider config

**Affected files**: E:\cursor project\InvokeAI\invokeai\app\api\routers\app_info.py, E:\cursor project\InvokeAI\invokeai\app\services\user_external_provider_configs\, E:\cursor project\InvokeAI\invokeai\app\services\external_generation\providers\openai.py, E:\cursor project\InvokeAI\invokeai\app\invocations\external_image_generation.py, E:\cursor project\invokeai-sub2api-poc\invokeai.yaml, docs/dev/codebase/invokeai-poc.md
**Upstream compatibility**: external InvokeAI checkout change; Sub2API runtime unchanged
**Change details**:
- Enabled InvokeAI PoC multiuser mode and strict password checking in the runtime config.
- Added InvokeAI SQLite migration/service for per-user external provider credentials, with OpenAI generation resolving API key/base URL from the current queue item's user.
- Kept single-user `api_keys.yaml` compatibility and documented that multiuser config deletion does not remove shared external model records.

## [2026-05-17] chore: add InvokeAI local dev-stack script

**Affected files**: E:\cursor project\InvokeAI\scripts\dev-stack.ps1, E:\cursor project\InvokeAI\scripts\dev-stack.cmd, E:\cursor project\InvokeAI\.gitignore, docs/dev/codebase/invokeai-poc.md
**Upstream compatibility**: external InvokeAI checkout tooling change; Sub2API runtime unchanged
**Change details**:
- Added an InvokeAI local process script with start/restart/stop/status actions, fixed runtime root, fixed `127.0.0.1:9090`, hidden background process launch, process state tracking, and logs under `tmp/dev-stack/logs`.
- The script enforces multiuser config values and writes `invokeai.yaml` as UTF-8 without BOM to avoid Windows GBK decode failures.
- Verified `restart` starts InvokeAI and `status` reports the managed process listening on port 9090.

## [2026-05-17] feat: disable InvokeAI setup with built-in admin for local PoC

**Affected files**: E:\cursor project\InvokeAI\invokeai\app\api\dependencies.py, E:\cursor project\InvokeAI\invokeai\app\api\routers\auth.py, E:\cursor project\InvokeAI\invokeai\app\services\config\config_default.py, E:\cursor project\InvokeAI\invokeai\app\services\users\users_common.py, E:\cursor project\InvokeAI\invokeai\frontend\web\src\features\auth\components\LoginPage.tsx, E:\cursor project\InvokeAI\scripts\dev-stack.ps1, docs/dev/codebase/invokeai-poc.md
**Upstream compatibility**: external InvokeAI checkout behavior change for the local PoC
**Change details**:
- Added built-in administrator config and startup enforcement so local InvokeAI creates/repairs `admin` / `admin123`.
- Disabled the public `/api/v1/auth/setup` path when built-in admin mode is enabled, while keeping normal login available.
- Updated the login field to accept the `admin` username and verified `/status`, `/setup`, and `/login` behavior against the running local service.
- Removed the frontend `/setup` page entry from the built UI so direct browser access to `http://127.0.0.1:9090/setup` no longer shows the administrator creation form.

## [2026-05-15] fix(gateway): preserve Anthropic web search beta

**Affected files**: backend/internal/service/gateway_service.go
**Upstream compatibility**: low risk; scoped to Claude Code OAuth passthrough request header construction
**Change details**:
- Preserved incoming `Anthropic-Beta` feature flags such as `web-search-2025-03-05` when building Claude Code mimic headers.
- Continued to avoid forwarding unrelated client fingerprint headers upstream.
- Restores native Claude web search server-tool requests that depend on the beta header.

<## [2026-05-14] fix(gateway): return real usage tokens downstream

**Affected files**: `backend/internal/handler/gateway_handler.go`
**Upstream compatibility**: scoped behavior rollback for gateway responses; billing and stored usage remain unchanged
**Change details**:
- Stopped injecting display token multipliers into gateway request context, so Claude/Antigravity response `usage` token fields are returned as the real upstream values.
- Kept existing display pricing helpers for user/admin usage-log UI; only downstream API response token rewriting is disabled.

## [2026-05-15] fix: default production Antigravity forwarding to prod endpoint

**Affected files**: deploy/.env.example, deploy/docker-compose.yml, deploy/docker-compose.standalone.yml, deploy/docker-compose.local.yml
**Upstream compatibility**: deployment configuration only; no application code changes
**Change details**:
- Added `GATEWAY_ANTIGRAVITY_FORWARD_BASE_URL=prod` to the example environment so production gateway requests use `cloudcode-pa.googleapis.com`.
- Passed `GATEWAY_ANTIGRAVITY_FORWARD_BASE_URL` through Docker Compose with a `prod` default to avoid accidentally forwarding production Code Assist project IDs to the daily sandbox endpoint.
- Added Antigravity User-Agent version passthrough to standalone/local compose variants for consistency with the production compose file.

## [2026-05-15] fix: clarify user subscription redeem support

**Affected files**: frontend/src/views/user/RedeemView.vue, frontend/src/api/redeem.ts, frontend/src/api/index.ts, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts
**Upstream compatibility**: frontend-only wording and type alignment
**Change details**:
- Updated the user redeem page to explicitly state that balance and subscription redeem codes are supported.
- Displayed subscription redeem success with the returned subscription group name and validity days when available.
- Removed button-like type labels from the redeem form so the hint stays informational.
- Aligned frontend redeem API types with the backend response fields for subscription codes.

## [2026-05-15] fix: align distribution asset generation

**Affected files**: backend/internal/service/distribution.go, backend/internal/handler/distribution_handler.go, backend/internal/repository/distribution_repo.go, backend/ent/schema/redeem_code.go, backend/ent/migrate/schema.go, backend/migrations/142_expand_redeem_code_length.sql, backend/cmd/server/wire_gen.go, frontend/src/views/user/DistributionView.vue, frontend/src/api/distribution.ts, frontend/src/types/index.ts, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts
**Upstream compatibility**: moderate risk; extends distribution generation behavior and redeem code schema length
**Change details**:
- Expanded redeem code storage to 64 characters so generated formatted codes fit the database and balance code generation no longer fails on insert.
- Changed distribution subscription code generation to select an existing subscription plan, charge `plan price * agent discount`, and generate a redeem code for the plan group and validity.
- Required distribution API keys to bind a concrete group and added full copyable API base URL, key, and usage instructions in the distributor UI.
- Kept wallet ledger row handling closed before ledger insert during balance adjustments in the distribution transaction path.

## [2026-05-15] fix: close distribution wallet rows before ledger insert

**Affected files**: backend/internal/repository/distribution_repo.go
**Upstream compatibility**: low risk; repository transaction handling fix only
**Change details**:
- Closed the `UPDATE ... RETURNING` result set before inserting the distribution wallet ledger row in admin balance adjustment.
- Prevents PostgreSQL transaction/driver errors caused by executing the ledger insert while the previous result set is still open.

## [2026-05-15] fix: prevent distribution wallet balance adjustment panic

**Affected files**: backend/internal/repository/distribution_repo.go
**Upstream compatibility**: low risk, scoped to distribution wallet ledger writes
**Change details**:
- Removed a deferred close on a wallet update row set that was later explicitly closed before inserting the ledger row.
- Prevented a nil row-set panic during balance redeem code generation after the wallet deduction succeeds.
- Verified /api/v1/distribution/redeem-codes/balance now creates the redeem code, distribution asset, and wallet ledger entry.

## [2026-05-15] fix: refine distribution admin management

**Affected files**: backend/internal/repository/distribution_repo.go, frontend/src/views/admin/DistributionView.vue, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts
**Upstream compatibility**: low risk; admin distribution UI and wallet ledger write fix only
**Change details**:
- Merged distribution applications and wallet accounts into one admin agent account table to reduce duplicated page space.
- Clarified subscription-code ratio wording as an agent cost ratio: 20% off / 80% cost should be entered as `0.8`.
- Changed distribution wallet ledger `created_by` binding to pass either a concrete admin ID or SQL NULL, avoiding driver issues during admin balance adjustment.

## [2026-05-15] feat: add distribution asset controls and agent ratios

**Affected files**: backend/migrations/140_add_distribution_assets.sql, backend/migrations/141_distribution_agent_rates_and_asset_refunds.sql, backend/internal/service/distribution.go, backend/internal/repository/distribution_repo.go, backend/internal/handler/distribution_handler.go, backend/internal/server/routes/, backend/internal/service/api_key_service.go, frontend/src/views/user/DistributionView.vue, frontend/src/views/admin/DistributionView.vue, frontend/src/api/distribution.ts, frontend/src/api/admin/distribution.ts, frontend/src/types/index.ts, frontend/src/i18n/locales/, docs/dev/codebase/distribution.md
**Upstream compatibility**: medium risk; adds distribution tables, APIs, and admin UI without changing normal recharge, normal balance, or existing redeem-code behavior
**Change details**:
- Added a `distribution_assets` ledger for distribution-generated balance codes, subscription codes, and API key packages, including original face value, original RMB cost, expiry data, linked generated record, and refund markers.
- Persisted generated assets in the same transaction as distribution redeem-code/API-key creation and added user/admin asset lists with copy and void actions.
- Voiding an unused asset now expires/disables the underlying redeem code or API key and refunds the original recorded RMB cost to the distribution wallet with ledger action `asset_refund`.
- Added per-agent ratio overrides for `rmb_per_usd_override` and `subscription_discount_override`; effective precedence is agent override first, then global setting.
- Updated frontend API types, bilingual UI strings, and distribution module documentation.

## [2026-05-14] fix(frontend): 补齐分销管理中文文案

**Affected files**: `frontend/src/i18n/locales/zh.ts`
**Upstream compatibility**: frontend locale-only fix; no backend or API behavior changes
**Change details**:
- Added missing Chinese locale entries for the expanded admin distribution page, including settings, wallet stats, wallet actions, and error messages.
- Fixed the Chinese UI fallback where keys such as `admin.distribution.settings.title` were rendered directly.

## [2026-05-14] docs: record GitHub PAT storage procedure

## [2026-05-14] feat(admin,gateway): add group-level model blacklist/whitelist control

**Affected files**: `backend/internal/service/group.go`, `backend/internal/service/admin_service.go`, `backend/internal/repository/group_repo.go`, `backend/internal/repository/api_key_repo.go`, `backend/internal/handler/group_model_access.go`, `backend/internal/handler/gateway_handler.go`, `backend/internal/handler/gateway_handler_chat_completions.go`, `backend/internal/handler/gateway_handler_responses.go`, `backend/internal/handler/openai_gateway_handler.go`, `backend/internal/handler/openai_images.go`, `backend/internal/handler/gemini_v1beta_handler.go`, `backend/internal/handler/admin/group_handler.go`, `backend/internal/handler/dto/types.go`, `backend/internal/handler/dto/mappers.go`, `backend/ent/schema/group.go`, `backend/migrations/138_add_group_model_access_control.sql`, `frontend/src/views/admin/GroupsView.vue`, `frontend/src/types/index.ts`, `frontend/src/i18n/locales/en.ts`, `frontend/src/i18n/locales/zh.ts`
**Upstream compatibility**: additive admin/API and gateway enforcement change; no pricing or public model display behavior changes
**Change details**:
- Added `blocked_models` and `allowed_models` to groups as JSONB-backed admin-only configuration with normalize/trim/dedupe handling.
- Enforced blacklist-first, whitelist-second model access checks before gateway account selection across OpenAI chat/responses/images, Gemini, and generic gateway paths.
- Added Responses image tool validation so `tools[].type == "image_generation"` entries cannot bypass group model restrictions.
- Extended the admin group create/edit modal to save and restore both lists, and updated English/Chinese locale copy.
- Kept the normal user-facing group DTO shallow so the new access-control fields remain admin-only.

**Verification**:
- `go test -tags=unit ./internal/service -run TestGroupIsModelAllowed`
- `go test -tags=unit ./internal/handler -run TestDisallowedResponsesImageToolModel`
- `pnpm run typecheck` in `frontend/`
- Broad backend unit test sweep still has a pre-existing unrelated failure in `TestAntigravityGatewayService_GetMappedModel`.

**Affected files**: docs/dev/SECURITY_OPERATIONS.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Documented that GitHub fork push PATs are stored in Git Credential Manager, not embedded in Git remote URLs or repository files.
- Recorded the tokenless `origin` remote URL convention for `541968679/sub2api`.
- Added rotation guidance for removing or replacing the stored GitHub credential.

## [2026-05-14] feat: 鐢ㄦ埛渚у浘鐗囦娇鐢ㄨ褰曞睍绀哄昂瀵镐笌璐ㄩ噺

**Affected files**: frontend/src/views/user/UsageView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: low risk, user usage UI/export display only
**Change details**:
- Updated user usage image rows to show image count, requested image size, and requested image quality without exposing billing tiers or pricing formulas.
- Added image count, image size, and image quality columns to the user CSV usage export.
- Added Chinese and English i18n labels for image size and image quality.
- Verified with `pnpm run typecheck`.

## [2026-05-14] chore: document local dev-stack startup

**Affected files**: AGENTS.md, DEV_GUIDE.md, backend/.air.toml, scripts/dev-stack.ps1, scripts/dev-stack.cmd, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: local development tooling and docs only; production runtime unchanged
**Change details**:
- Documented the local port convention for backend `18081` and frontend `15174`.
- Added an `air` hot-reload config for local backend development.
- Added Windows `dev-stack` wrappers for consistent local start/restart/stop workflows.
- Kept production deployment ports independent from local development ports.

## [2026-05-14] fix: display pricing usage token rewrite

**Affected files**: backend/internal/handler/gateway_handler.go, backend/internal/service/display_token_rewrite.go, backend/internal/service/gateway_service.go, backend/internal/service/antigravity_gateway_service.go
**Upstream compatibility**: scoped to user-facing usage token display transforms; actual billing cost is unchanged
**Change details**:
- Computes effective display token multipliers from account rate, user group rate, display rate, and model display prices.
- Rewrites Claude/Antigravity streaming and non-streaming usage token fields so user-visible token counts align with display pricing.
- Leaves actual billing and stored actual cost based on the existing real pricing path.
- Verified by backend compile through targeted unit tests and frontend build.

## [2026-05-14] fix: 绐佸嚭鍥剧墖璐ㄩ噺鍗曚环閰嶇疆鍏ュ彛

**Affected files**: frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: low risk, admin model pricing UI only
**Change details**:
- Made the `low` / `medium` / `high` / `auto` image quality price fields a labeled subsection under megapixel image billing.
- Clarified that empty quality prices fall back to the default megapixel price.
- Verified with `pnpm run typecheck`.

## [2026-05-14] feat: 鍥剧墖妗ｄ綅璁¤垂鏀寔 quality 涔樻暟

**Affected files**: backend/internal/service/image_billing.go, backend/internal/service/image_billing_test.go, backend/internal/service/global_model_pricing.go, backend/internal/service/global_model_pricing_service.go, backend/internal/service/model_pricing_resolver.go, backend/internal/handler/admin/model_pricing_handler.go, backend/internal/repository/global_model_pricing_repo.go, backend/migrations/137_add_image_quality_multipliers.sql, frontend/src/api/admin/modelPricing.ts, frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/codebase/billing.md
**Upstream compatibility**: additive DB/API/UI change; existing tier pricing remains unchanged when multipliers are unset
**Change details**:
- Added `image_quality_multipliers` for tier image billing so the matched `1K/2K/4K` price can be multiplied by `low/medium/high/auto`.
- Defaulted omitted/unknown image quality to `auto`, and left the effective multiplier at `1.0` unless an administrator configures a multiplier.
- Kept `image_quality_prices` as megapixel-mode USD/MP overrides; tier mode now uses the separate multiplier map.
- Added admin UI fields for quality multipliers under image tier billing, with `auto` defaulting to `1`.
- Verified with `go test -tags=unit ./internal/service -run "ImageBilling|GlobalModelPricing|ModelPricingResolver"`, `go test -tags=unit ./internal/handler/admin -run "ModelPricing"`, `go test -tags=unit ./internal/service ./internal/repository -run "ImageBilling|GlobalModelPricing|ModelPricingResolver"`, and `pnpm run typecheck`.
- Full `go test -tags=unit ./internal/handler/admin ./internal/repository` still has an unrelated existing failure in `TestAccountHandlerGetAvailableModels_OpenAIOAuthUsesExplicitModelMapping` where the test expects 1 model but receives 13.

## [2026-05-14] feat: add first-stage distribution system

**Affected files**: backend/migrations/139_add_distribution_agents.sql, backend/internal/service/distribution.go, backend/internal/repository/distribution_repo.go, backend/internal/handler/distribution_handler.go, backend/internal/server/routes/{user,admin}.go, frontend/src/views/{user,admin}/DistributionView.vue, frontend/src/api/distribution.ts, frontend/src/api/admin/distribution.ts, frontend/src/router/index.ts, frontend/src/components/layout/AppSidebar.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/distribution.md
**Upstream compatibility**: medium risk; adds a new domain, tables, routes, DI providers, and frontend pages.
**Change details**:
- Added distribution agent application, admin review, independent wallet schema, and wallet ledger schema.
- Added user APIs for distribution summary, application submission, and wallet ledger viewing.
- Added admin APIs for listing and reviewing distribution applications.
- Added user/admin frontend pages and sidebar/router entries for distribution.
- Documented the distribution module and first-release scope.
- Deferred recharge discount, redeem-code generation, API key package generation, and subscription coupon cashback until business rules are confirmed.

## [2026-05-14] feat: extend distribution system with generation and wallet management

**Affected files**: backend/internal/service/distribution.go, backend/internal/repository/distribution_repo.go, backend/internal/handler/distribution_handler.go, backend/internal/server/routes/user.go, backend/internal/server/routes/admin.go, backend/internal/service/domain_constants.go, backend/internal/service/setting_service.go, backend/internal/service/user_service.go, backend/internal/repository/api_key_repo.go, backend/internal/repository/redeem_code_repo.go, backend/internal/repository/group_repo.go, backend/internal/repository/user_repo.go, backend/cmd/server/wire_gen.go, frontend/src/api/distribution.ts, frontend/src/api/admin/distribution.ts, frontend/src/views/user/DistributionView.vue, frontend/src/views/admin/DistributionView.vue, frontend/src/types/index.ts, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, docs/dev/codebase/distribution.md
**Upstream compatibility**: additive feature expansion; existing application/review flow preserved
**Change details**:
- Added distribution settings stored in Settings KV: RMB-per-USD generation ratio and subscription-code discount ratio.
- Reworked distribution wallet semantics to use RMB balance as the displayed/recorded unit.
- Added user-side generation flows for balance redeem codes, subscription redeem codes, and fixed-quota API keys.
- Added admin wallet controls for settings, wallet listing, freeze/unfreeze, manual adjustment, and ledger review.
- Wired generation paths through transactions so wallet deduction and generated assets commit together.
- Updated user and admin distribution views to expose the new controls and generation results.

## [2026-05-12] feat(aiclient2api): Kiro 鍙嶄唬缂撳瓨浼扮畻涓?conversationId 绋冲畾鍖?

**褰卞搷鑼冨洿**: `aiclient2api/src/providers/claude/claud*: 鏃犲啿绐侊紙aiclient2api 鏄嫭绔?fork锛?
**鍙樻洿璇︽儏**:
- 鏂板 `deriveStableConversationId(metadata)`: 浠?Claude Code 鐨?`metadata.user_id` 涓彁鍙?session_id锛宧ash 涓虹‘瀹氭€?UUID锛屼娇鍚屼竴浼氳瘽鐨勬墍鏈?turn 鍏变韩 conversationId锛屽惎鐢?Amazon Q 鏈嶅姟绔笂涓嬫枃缂撳瓨
- 鏂板 `filterBillingHeaderFromSystem()`: 杩囨护 system prompt 涓瘡杞兘鍙樼殑 `x-anthropic-billing-header`锛坈ch= 瀛楁锛夛紝淇濇寔 prompt 绋冲畾
- 鏂板 `_estimateCacheMetrics(requestBody)` + `_countMessageTokens(msg)`: 浠庤姹備綋浼扮畻缂撳瓨 token 鈥?棣栬疆鎶?cache_creation锛屽悗缁疆鎶?system + tools + 鍘嗗彶鍓嶇紑鎶ヤ负 cache_read锛宨nput_tokens 鍙鏈€鍚庝竴鏉℃柊娑堟伅
- `_countMessageTokens` 姝ｇ‘澶勭悊鎵€鏈?content block 绫诲瀷锛坱ext/thinking/tool_use/tool_result锛夛紝缂撳瓨鐜囦粠 ~45% 鎻愬崌鑷?~83%
- 娴佸紡鍝嶅簲鐨?message_start 鍜?message_delta 浜嬩欢浣跨敤浼扮畻鍊兼浛浠ｇ‖缂栫爜 0

## [2026-05-12] feat: antigravity 鍒嗙粍鎺ュ叆 Kiro 鍙嶄唬锛堟柟妗?B锛?

**褰卞搷鑼冨洿**: `backend/internal/service/account.go`, `backend/internal/service/gateway_service.go`, `backend/internal/pkg/antigravity/claude_types.go`, `backend/internal/service/account_anthropic_passthrough_test.go`, `frontend/vite.config.ts`, `docs/dev/KIRO_PROXY.md`
**涓婃父鍏煎鎬?*: 涓瓑銆俙account.go` 鐨?`IsAnthropicAPIKeyPassthroughEnabled` 鍜?`GetBaseURL` 鏀逛簡鏉′欢閫昏緫锛沗gateway_service.go` 鐨勬ā鍨嬫敮鎸佹鏌ュ姞浜?passthrough bypass锛涗笂娓歌嫢閲嶆瀯杩欎簺鍑芥暟闇€鎵嬪姩鍚堝苟銆?
**鍙樻洿璇︽儏**:
- 鏀惧純鏂规 A锛堣矾鐢卞眰鍥為€€锛夛紝閲囩敤鏂规 B锛欿iro 璐﹀彿閰嶇疆涓?`platform=antigravity` + `type=apikey` + `passthrough=true`锛岀洿鎺ュ弬涓?antigravity 鍒嗙粍 load-aware 璋冨害
- `IsAnthropicAPIKeyPassthroughEnabled()`: 鏀惧骞冲彴闄愬埗锛屼粠鍙帴鍙?anthropic 鏀逛负鍚屾椂鎺ュ彈 antigravity
- `GetBaseURL()`: antigravity passthrough 璐﹀彿涓嶅啀鑷姩鎷兼帴 `/antigravity` 鍚庣紑锛堜粎 Google Cloud Code 鍘熺敓 apikey 璐﹀彿闇€瑕侊級
- `isModelSupportedByAccountWithContext()` / `isModelSupportedByAccount()`: antigravity passthrough 璐﹀彿璺宠繃妯″瀷鏄犲皠妫€鏌ワ紝鎺ュ彈鎵€鏈夋ā鍨?
- `DefaultModels()`: 涓?Claude 妯″瀷鐢熸垚 `[1m]`/`[2m]` 涓婁笅鏂囩獥鍙ｅ悗缂€鍙樹綋锛岃В鍐?Claude Code 瀹㈡埛绔ā鍨嬫牎楠屼笉閫氳繃鐨勯棶棰?
- `vite.config.ts`: 鏂板 `/antigravity` 浠ｇ悊璺緞锛屾湰鍦板紑鍙戞椂鍓嶇 dev server 姝ｇ‘杞彂鍒板悗绔?
- 鏇存柊 `docs/dev/KIRO_PROXY.md` 鏂囨。锛岃褰曞畬鏁存柟妗堛€侀厤缃楠ゅ拰鎺掓煡杩囩▼涓彂鐜扮殑 4 涓潙

## [2026-05-12] feat(deploy): AIClient2API 姝ｅ紡涓婄嚎鐢熶骇 + Web UI 鍏綉鍙闂?

**褰卞搷鑼冨洿**: 鐢熶骇 `/opt/sub2api/.env`銆乣/opt/sub2api/docker-compose.yml`銆乣/etc/caddy/Caddyfile`銆丆loudflare DNS (`a2.zerocode.kaynlab.com`)锛宍deploy/docker-compose.yml`銆乣docs/dev/KIRO_PROXY.md`
**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紙浠呯敓浜ч儴缃查厤缃?+ 鏈粨搴?compose/鏂囨。锛?
**鍙樻洿璇︽儏**:
- 瀹屾垚 AIClient2API 鐢熶骇閮ㄧ讲锛欶ork `541968679/AIClient2API` 鈫?鍦ㄧ敓浜ф湇鍔″櫒 `git clone + docker build` 鈫?閫氳繃 `update.sh --only-a2` 閮ㄧ讲
- 鐢熶骇 `.env` 琛ュ厖 `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true` 鍜?`SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true`锛屽厑璁?sub2api 閫氳繃 `http://aiclient2api:3000` 璋冪敤鍐呯綉 sidecar锛堟湰鍦?dev 鏈惎鐢?allowlist 鎵€浠ユ病閬囧埌锛?
- 淇 aiclient2api healthcheck锛歚localhost` 鍦ㄥ鍣ㄥ唴浼樺厛瑙ｆ瀽鍒?IPv6 `::1`锛屼絾鏈嶅姟鍙洃鍚?IPv4 `0.0.0.0:3000`锛屾敼涓?`127.0.0.1:3000`
- 鍏綉 Web UI锛氭柊澧?Cloudflare DNS A 璁板綍 `a2.zerocode.kaynlab.com 鈫?172.245.247.80`锛圖NS Only锛夛紝鏂板 Caddy vhost 鍙嶄唬鍒板涓绘満 `127.0.0.1:3000`
- compose 缁?aiclient2api 缁戝畾鍒板涓绘満 `127.0.0.1:3000`锛堜笉瀵瑰叕缃戞毚闇诧紝浠呬緵 Caddy 鏈満鍙嶄唬锛夛紝Docker 鍐呯綉 DNS 鍚屾椂浠嶅彲鐢?
- 鍙ｄ护銆乄eb UI 璁块棶鍦板潃銆丆addyfile 绀轰緥銆佽疆鎹㈡祦绋嬪凡鍏ㄩ儴璁板綍鍦?`docs/dev/KIRO_PROXY.md`
- **褰撳墠鍙敤閾捐矾**锛歛nthropic 鍒嗙粍 API Key 鈫?sub2api 缃戝叧 鈫?AIClient2API (`http://aiclient2api:3000/claude-kiro-oauth`) 鈫?Kiro API 鈫?Claude 绯诲垪妯″瀷

## [2026-05-11] feat: Kiro 鍙嶄唬瀵规帴锛坅nthropic 鍒嗙粍宸查€氾紝antigravity 鍒嗙粍閬楃暀锛?

**褰卞搷鑼冨洿**: `backend/internal/service/gateway_service.go`, `backend/internal/service/account.go`, `frontend/src/components/account/CreateAccountModal.vue`, `frontend/src/components/account/EditAccountModal.vue`, `AIClient2API` 瀛愰」鐩? `docs/dev/KIRO_PROXY.md`
**涓婃父鍏煎鎬?*: 涓瓑鍐茬獊锛実ateway_service.go 鍔ㄤ簡 passthrough 鍒嗘敮鍜?selectAccount 娴佺▼
**鍙樻洿璇︽儏**:
- 閫氳繃 AIClient2API 瀛愰」鐩皢 Kiro 璐﹀彿鍙嶄唬涓?Anthropic Messages API锛屽啀浠?anthropic 骞冲彴 API Key 鏂瑰紡鎺ュ叆 sub2api锛堝凡璺戦€氾紝閫氳繃 `/v1/messages` 绔偣鍙甯镐娇鐢?Kiro 鐨?Claude 妯″瀷锛?
- `gateway_service.go`: passthrough 杞彂鍓嶆竻鐞嗘ā鍨嬪悕涓殑 `[1m]`/`[2m]` 绛変笂涓嬫枃绐楀彛鍚庣紑锛圕laude Code 瀹㈡埛绔細甯︽鍚庣紑锛孠iro 涓嶈瘑鍒級
- `gateway_service.go`: antigravity 鍒嗙粍閫変笉鍒拌处鍙锋椂鍥為€€鍒?anthropic passthrough 璐﹀彿锛堟柟妗?A锛氳矾鐢卞眰鍥為€€锛屼笉鏀硅处鍙锋ā鍨嬶級
- 鍓嶇 `CreateAccountModal` / `EditAccountModal`: 鎵╁睍 `anthropic_passthrough` 寮€鍏虫樉绀哄埌 antigravity 骞冲彴 apikey 璐﹀彿
- AIClient2API 渚т慨鏀?`claude-kiro.js` 鐨勮韩浠芥敞鍏ワ紝鎶婁綔鑰呯殑"浣曞2077"鏀逛负鍔ㄦ€?`${model}` 鍙橀噺锛岃妯″瀷鑷О涓庤姹備竴鑷寸殑鍚嶅瓧锛堝 `claude-opus-4-7`锛?
- **閬楃暀闂**锛堣瑙?`docs/dev/KIRO_PROXY.md`锛夛細
  1. antigravity 鍒嗙粍瀹炴祴浠嶆姤 `claude-opus-4-7[1m]` 妯″瀷閿欒锛岀枒浼肩紪璇戞湭鐢熸晥鎴栬蛋浜嗗叾浠栬矾寰?
  2. antigravity 鍒嗙粍鐨?key 鏃犳硶鍦?sub2 骞冲彴鑾峰彇棰濆害淇℃伅
  3. API 璋冪敤閫熷害鍋忔參锛屾湭鍋氱綉缁滈摼璺垎鏋?
- 瀹屾暣瀵规帴鏂规銆佸凡鐭ュ潙銆侀仐鐣欓棶棰樻帓鏌ユ柟鍚戝潎璁板綍鍦?`docs/dev/KIRO_PROXY.md`

## [2026-05-10] infra: 寮曞叆 AIClient2API 浣滀负 Kiro 鍙嶄唬瀛愰」鐩?

**褰卞搷鑼冨洿**: 椤圭洰澶栭儴渚濊禆锛坄E:\cursor project\AIClient2API`锛夈€乣docs/dev/KIRO_PROXY.md`
**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紝涓嶄慨鏀?sub2api 浠ｇ爜
**鍙樻洿璇︽儏**:
- 寮曞叆 [AIClient2API](https://github.com/justlovemaki/AIClient2API)锛?600+ stars锛変綔涓?Kiro 鍙嶅悜浠ｇ悊瀛愰」鐩?
- sub2api 鏈韩涓嶆敮鎸?Kiro 骞冲彴锛岄€氳繃 AIClient2API 灏?Kiro 璐﹀彿鍙嶄唬涓?Anthropic Messages API锛屽啀浠?API Key 鏂瑰紡鎺ュ叆 sub2api
- 瀵规帴璺緞锛歴ub2api Anthropic API Key 璐﹀彿 鈫?`base_url` 鎸囧悜 `http://{A2鍦板潃}:3000/claude-kiro-oauth` 鈫?AIClient2API 杞彂鑷?Kiro 涓婃父
- 鏂板 `docs/dev/KIRO_PROXY.md` 鏂囨。璁板綍瀹屾暣瀵规帴鏂规

## [2026-05-10] docs: document Kiro Gateway sidecar integration

**Affected files**: docs/dev/codebase/kiro-gateway.md, docs/dev/codebase/README.md
**Upstream compatibility**: docs-only; records a local sidecar integration without merging external code
**Change details**:
- Added a Kiro Gateway sidecar module note for `E:\cursor project\kiro-gateway`, including local startup commands and Sub2API Anthropic API Key account mapping.
- Documented that Kiro Gateway account management is file-based through `credentials.json`, and that startup requires at least one valid Kiro account.
- Recorded the current local blocker: detected Kiro IDE credential file exists, but token refresh returns 401 and must be refreshed before the service can stay running.

## [2026-05-08] fix: reuse Antigravity token provider for quota probes

**Affected files**: backend/internal/service/antigravity_quota_fetcher.go, backend/internal/service/antigravity_quota_fetcher_test.go, backend/internal/service/wire.go, backend/cmd/server/wire_gen.go, docs/dev/codebase/account.md
**Upstream compatibility**: low risk, Antigravity account status/usage probe fix only
**Change details**:
- Changed Antigravity quota/AI Credits probes to resolve OAuth access tokens through `AntigravityTokenProvider` instead of reading `credentials.access_token` directly.
- Kept setup-token and upstream account fallback behavior, while allowing OAuth probes to run when only `refresh_token` is present.
- Updated Wire provider wiring so `AntigravityQuotaFetcher` is constructed with the shared token provider, matching model test and gateway request token lifecycle.
- Added focused unit coverage for provider-backed token resolution and refresh-token-only OAuth probe eligibility.

## [2026-05-08] fix: pin pnpm in Docker builds

**Affected files**: Dockerfile, deploy/Dockerfile
**Upstream compatibility**: build-only fix; runtime behavior unchanged
**Change details**:
- Pinned Docker build pnpm installation to `pnpm@9.15.9` instead of `pnpm@latest`.
- Avoided pnpm 10/11 `approve-builds` behavior breaking non-interactive Docker builds when esbuild/vue-demi postinstall scripts are needed.
- Verified a full local Docker image build succeeds with the pinned pnpm version.

## [2026-05-08] fix: prevent Antigravity OAuth false auth errors on Chat Completions

**Affected files**: backend/internal/handler/gateway_handler_chat_completions.go, backend/internal/service/gateway_service.go, backend/internal/service/ratelimit_service.go, backend/internal/service/ratelimit_service_401_test.go, backend/internal/service/gateway_multiplatform_test.go, docs/dev/codebase/gateway.md, docs/dev/codebase/account.md, docs/dev/codebase/README.md
**Upstream compatibility**: medium risk; changes gateway account selection for `/v1/chat/completions` compatibility requests and OAuth 401 state handling.
**Change details**:
- Production logs showed one `/v1/chat/completions` request on 2026-05-08 12:41:40 selected Antigravity accounts 145, 146, and 144 in sequence, received upstream 401 `Invalid bearer token`, and marked them error while `/antigravity/v1/messages` was still succeeding.
- Added a context flag that disables Antigravity mixed scheduling for the Anthropic Chat Completions compatibility path, so that path only selects native Anthropic accounts until an Antigravity-specific Chat Completions conversion exists.
- Changed OAuth 401 handling so Antigravity OAuth accounts follow the same cache invalidation, forced refresh, and temporary-unschedulable path as other OAuth accounts instead of permanent `SetError`.
- Added regression coverage for mixed-scheduling isolation and updated the OAuth 401 expectations.

## [2026-05-07] fix(frontend): 璁㈤槄濂楅浠锋牸绗﹀彿 $ 鈫?楼

**褰卞搷鑼冨洿**: `frontend/src/components/payment/SubscriptionPlanCard.vue`, `frontend/src/views/admin/orders/AdminPaymentPlansView.vue`
**涓婃父鍏煎鎬?*: 浣庡啿绐侊紝浠呮秹鍙婂墠绔ā鏉挎枃鏈?
**鍙樻洿璇︽儏**:
- 淇璁㈤槄濂楅鍗＄墖浠锋牸鍜屽垝绾垮師浠锋樉绀?`$` 鑰岄潪 `楼` 鐨勯棶棰橈紙濂楅浠锋牸鏄汉姘戝竵锛?
- 淇绠＄悊鍚庡彴濂楅鍒楄〃椤典环鏍煎垪鍚屾牱鐨?`$` 鈫?`楼` 閿欒
- 娉ㄦ剰鍖哄垎锛氬椁愪环鏍硷紙price/original_price锛変负 CNY 鐢?`楼`锛涚敤閲忛檺棰濓紙daily_limit_usd 绛夛級涓?USD 鐢?`$`

## [2026-05-07] fix: avoid permanent error on setup-token 401

**Affected files**: backend/internal/service/ratelimit_service.go, backend/internal/service/ratelimit_service_401_test.go, docs/dev/codebase/account.md
**Upstream compatibility**: low risk, OAuth error-policy bug fix
**Change details**:
- Changed 401 handling to treat `setup-token` accounts as OAuth-like accounts via `account.IsOAuth()`, matching gateway credential routing.
- A first 401 for setup-token accounts now invalidates token state and marks the account temporarily unschedulable instead of immediately setting `status=error`.
- Added unit coverage for Anthropic setup-token `Invalid bearer token` responses.

## [2026-05-07] docs: 浼樺寲 Codex 鎺ュ叆鏁欑▼

**Affected files**: docs/API_USAGE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Renamed chapter 4 from "OpenAI Codex CLI 鎺ュ叆鎸囧崡" to "Codex 鎺ュ叆鎸囧崡".
- Clarified that Codex CLI and Codex desktop share the same `.codex/config.toml` and `.codex/auth.json` files, so CC-Switch can manage both with one configuration.
- Removed the WSL2-based Windows installation path and simplified Windows setup to native Node.js/npm installation.

## [2026-05-07] docs: 璋冩暣鏁欑▼骞冲彴椤哄簭骞剁Щ闄?Linux 瀹夎閰嶇疆

**Affected files**: docs/API_USAGE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Reordered tutorial installation and configuration platform instructions to Windows first, then macOS.
- Removed Linux-specific installation/configuration paths and commands from Claude Code and Codex setup sections.
- Updated screenshot notes and platform selectors to reference only Windows and macOS.

<!--
绀轰緥鏉＄洰锛?

## [2026-05-06] chore: add read-only Antigravity usage audit script

**Affected files**: tools/audit_antigravity_usage.py
**Upstream compatibility**: low risk, standalone tooling only
**Change details**:
- Added a psql-based read-only audit script for Antigravity usage mismatch investigations.
- Reports local usage by account/API key/client, AI Credits snapshot deltas by email, credits-vs-local reconciliation, suspicious API keys with multiple IPs/User-Agents, duplicate request IDs, billing dedup summaries, and missing client attribution fields.
- Supports `DATABASE_URL` or `--database-url`, explicit `--start`/`--end` windows, and `--sql-only` for review or server-side execution.

## [2026-05-06] feat: add Antigravity per-request AI Credits sampling

**Affected files**: backend/migrations/134_add_antigravity_credit_request_samples.sql, backend/internal/service/antigravity_credit_sampler.go, backend/internal/repository/antigravity_credit_sample_repo.go, backend/internal/service/antigravity_gateway_service.go, backend/internal/service/gateway_service.go, backend/internal/{service,repository}/wire.go, backend/cmd/server/wire_gen.go
**Upstream compatibility**: low risk when disabled; diagnostic path is gated by `SUB2API_ANTIGRAVITY_CREDIT_SAMPLE_ACCOUNT_IDS`
**Change details**:
- Added `antigravity_credit_request_samples` to store request-linked before/after AI Credits balances, delta, account/API key/user/request IDs, timestamps, confidence, and fetch errors.
- Added an Antigravity credit sampler that captures a balance before forwarding and writes request samples after the usage log is persisted.
- Wired the sampler into Antigravity Claude/Gemini forwarding and Gateway usage recording.
- Sampling is disabled by default; enable with comma-separated account IDs in `SUB2API_ANTIGRAVITY_CREDIT_SAMPLE_ACCOUNT_IDS`.
- Concurrent requests on the same sampled account can still blur before/after attribution; prefer temporarily low account concurrency for the diagnostic window.

## [2026-05-06] security: rotate local admin password

**Affected files**: local PostgreSQL `users` table, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: no upstream code impact; local credential rotation only
**Change details**:
- Rotated the local administrator password for `admin@sub2api.local` by updating `users.password_hash` in the local `sub2api` database.
- Verified that the new password matches the stored bcrypt hash.
- Did not record the plaintext password or password hash in repository files.

## [2026-05-06] fix: avoid IPv6 localhost Caddy upstream failures

**Affected files**: deploy/Caddyfile, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: deployment configuration only; low risk
**Change details**:
- Changed the Caddy reverse proxy upstream from `localhost:8080` to `127.0.0.1:8080`.
- Prevents Caddy from intermittently resolving `localhost` to IPv6 `::1` while Docker publishes Sub2API only on IPv4, which caused `connect: connection refused` 502s during production traffic.

## [2026-05-06] docs: document admin password rotation

**Affected files**: deploy/README.md, deploy/.env.example, docs/dev/SECURITY_OPERATIONS.md, AGENTS.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Documented that `ADMIN_PASSWORD` is first-run bootstrap only and does not rotate an installed admin account.
- Added an operational bcrypt-based admin password rotation procedure with `token_version` handling when that column exists.
- Added a security operations checklist for suspected credential compromise without recording any real password or hash.

## [2026-05-06] feat: add Antigravity credit usage curve

**Affected files**: backend/internal/service/credit_snapshot*.go, backend/internal/repository/antigravity_usage_aggregator.go, backend/internal/handler/admin/usage_handler.go, backend/internal/server/routes/admin.go, frontend/src/api/admin/usage.ts, frontend/src/components/admin/usage/AntigravityUsageCurveChart.vue, frontend/src/views/admin/UsageView.vue, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: low risk, additive admin-only API and UI
**Change details**:
- Added `GET /api/v1/admin/usage/stats/antigravity/curve` to aggregate `ai_credit_snapshots` deltas with Antigravity request count, token count, quota cost, and actual cost by hour/day.
- Added per-window derived ratios including credits/request, quota/credit, and tokens/credit, plus a simple median-based spike score.
- Added an admin Usage page line chart comparing AI Credits, requests, tokens, quota cost, and credits/request for the selected time range.

## [2026-05-06] chore: automate Docker disk cleanup after deploy

**Affected files**: deploy/update.sh, deploy/docker-cleanup.sh, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: deployment script only; low risk
**Change details**:
- Added post-deploy Docker cleanup for BuildKit cache older than `DOCKER_BUILD_CACHE_MAX_AGE` (default `24h`).
- Added dangling image cleanup after successful health checks while preserving tagged rollback images.
- Logs post-cleanup Docker disk usage to `/opt/sub2api/deploy.log`.
- Added a reusable daily cleanup script for cron/system scheduling.

## [2026-05-06] fix: repair Antigravity credit curve bucket matching

**Affected files**: backend/internal/service/credit_snapshot_service.go
**Upstream compatibility**: low risk, aggregation bug fix only
**Change details**:
- Changed Antigravity credit curve bucket lookup keys from `time.Time` values to Unix seconds so PostgreSQL timestamp locations and request time locations still match the same hour/day window.

## [2026-05-06] fix: align Antigravity credit curve usage buckets to app timezone

**Affected files**: backend/internal/repository/antigravity_usage_aggregator.go
**Upstream compatibility**: low risk, aggregation bug fix only
**Change details**:
- Changed Antigravity usage window aggregation to truncate `usage_logs.created_at` in the configured application timezone before returning buckets, matching the credit snapshot curve buckets.

## [2026-05-06] fix: include historical Antigravity accounts in usage curve

**Affected files**: backend/internal/service/credit_snapshot.go, backend/internal/service/credit_snapshot_service.go, backend/internal/repository/antigravity_usage_aggregator.go
**Upstream compatibility**: low risk, aggregation bug fix only
**Change details**:
- Changed Antigravity request/cost/token aggregation to join `usage_logs` with `accounts.platform='antigravity'` instead of filtering by the currently active account ID list.
- Restored historical request counts for soft-deleted or rotated Antigravity accounts so credit curve windows match historical usage logs.

## [2026-05-06] fix: reduce Antigravity credit curve sampling lag

**Affected files**: backend/internal/service/credit_snapshot_service.go, backend/internal/service/credit_snapshot_service_test.go
**Upstream compatibility**: low risk, aggregation-only display fix
**Change details**:
- Changed Antigravity credit snapshot deltas to be attributed across the interval between the previous and current snapshot instead of assigning all credits to the current snapshot bucket.
- Weighted credit attribution by hourly usage cost, then actual cost, tokens, and call count, with a snapshot-bucket fallback for intervals without usage.
- Added unit coverage for weighted interval attribution and no-usage fallback behavior.

## [2026-05-06] docs: document Antigravity credit cost analysis

**Affected files**: docs/dev/ANTIGRAVITY_CREDIT_COST_ANALYSIS_2026-05-06.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Documented the production analysis explaining why balance revenue per Antigravity AI Credit fell after cache-heavy traffic increased.
- Recorded period, daily, user-level, model-level, and same-day metrics used to distinguish cache-read pricing effects from account leakage.
- Added follow-up recommendations for Antigravity-specific pricing calibration and leakage alerts.

## [2026-05-06] fix: shift cache display premium into input display

**Affected files**: backend/internal/handler/dto/display_pricing.go, backend/internal/handler/dto/display_pricing_test.go, backend/internal/handler/admin/model_pricing_handler.go, backend/internal/handler/admin/user_model_pricing_handler.go, backend/internal/handler/admin/usage_handler.go, backend/internal/service/global_model_pricing.go, backend/internal/service/global_model_pricing_service.go, backend/internal/service/user_model_pricing.go, backend/internal/repository/global_model_pricing_repo.go, backend/internal/repository/user_model_pricing_repo.go, frontend/src/api/admin/modelPricing.ts, frontend/src/api/admin/userModelPricing.ts, frontend/src/api/admin/usage.ts, frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue, frontend/src/components/admin/user/UserModelPricingModal.vue, frontend/src/components/admin/usage/UserViewCompareDrawer.vue, frontend/src/i18n/locales/en.ts, frontend/src/i18n/locales/zh.ts, docs/dev/codebase/billing.md
**Upstream compatibility**: display/API/UI behavior change; DB columns retained for rollback compatibility
**Change details**:
- Changed user-facing model display pricing so cache-read tokens stay at the real token count and cache-read cost uses `display_cache_read_price`.
- Moves positive cache-read premium into displayed input cost/tokens only when both `display_cache_read_price` and `display_input_price` are configured; otherwise cache-read usage display remains real. `actual_cost` and `rate_multiplier` remain unchanged.
- Soft-deprecated `cache_transfer_ratio`: backend no longer reads/writes it, admin/user pricing APIs no longer expose it, and frontend forms/compare drawer no longer render it. Existing DB columns remain.
- Added DTO unit coverage for cache premium transfer, missing display input price fallback, and display map behavior.

## [2026-05-04] fix(frontend): 鍏呭€艰闃呴〉闈?UI 浼樺寲

**褰卞搷鑼冨洿**: `frontend/src/views/user/PaymentView.vue`, `frontend/src/components/payment/SubscriptionPlanCard.vue`
**涓婃父鍏煎鎬?*: 浣庡啿绐侊紝浠呮秹鍙婂墠绔ā鏉垮拰鏍峰紡
**鍙樻洿璇︽儏**:
- 淇鍙充晶璁㈤槄鏍忔爣棰?i18n key 閿欒锛坄payment.tabSubscription` 鈫?`payment.tabSubscribe`锛夛紝涔嬪墠鏄剧ず鍘熷 key 鑰岄潪涓枃缈昏瘧
- 澶氬椁愭椂浠庢í鍚戠綉鏍兼帓鍒楁敼涓虹旱鍚戝垪琛ㄦ帓鍒楋紝纭繚鍏抽敭淇℃伅涓嶈鎴柇
- 绉婚櫎濂楅鍗＄墖鍜岃闃呯‘璁ゅ尯鍩熺殑骞冲彴鏍囪瘑 badge锛圤penAI銆丄ntigravity 绛夛級

## [2026-05-04] docs: 鏂板 API 浣跨敤鏂囨。锛堝鎴峰悜锛?

**褰卞搷鑼冨洿**:
- `docs/API_USAGE.md`锛堟柊澧烇級

**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紙绾柊澧炴枃浠讹級
**鍙樻洿璇︽儏**:
- 鏂板闈㈠悜瀹㈡埛鐨?API 浣跨敤鏂囨。锛岃鐩?Claude Code锛圕LI / Desktop / VS Code / JetBrains锛夊拰 OpenAI Codex CLI 鐨勫畨瑁呴厤缃叏娴佺▼
- 鍖呭惈骞冲彴娉ㄥ唽鍏呭€兼祦绋嬨€佹ā鍨嬪垪琛ㄣ€丄PI 绔偣鍙傝€冦€佽璐硅鏄庛€丗AQ
- 棰勭暀鎴浘鍗犱綅绗︼紙鍚爣娉ㄨ鏄庯級锛屽緟鍚庣画琛ュ厖瀹為檯鎴浘

---

## [2026-05-02] progress: v0.1.117 鍚堝苟楠岃瘉涓庝腑鏂?i18n 琛ラ綈

**褰卞搷鑼冨洿**:
- `frontend/src/i18n/index.ts`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `docs/dev/CHANGELOG_CUSTOM.md`
- `docs/dev/UPSTREAM_SYNC.md`

**涓婃父鍏煎鎬?*:
- Low. 褰撳墠鏀瑰姩闆嗕腑鍦ㄥ墠绔?i18n 榛樿璇█銆佹彃鍊兼牸寮忓拰涓枃鏂囨琛ラ綈锛屼笉鏀瑰彉鍚庣涓氬姟閫昏緫銆?
- 鍚庣画濡傛灉涓婃父缁х画鏂板 i18n key锛岄渶瑕佺户缁繚鎸?`en.ts` / `zh.ts` key 瑕嗙洊涓€鑷淬€?

**褰撳墠杩涘害**:
- 宸插湪鐙珛 worktree `E:\cursor project\api2sub-v117`銆佸垎鏀?`sync/upstream-v0.1.117` 鍚堝苟涓婃父 `v0.1.117`銆?
- 宸插畬鎴愭湰鍦版彁浜わ細
  - `37519fcb` merge v0.1.117
  - `511e419b` fix(frontend): default locale and interpolation for v117
  - `64b5dff2` fix(frontend): add zh login locale keys
  - `243eae93` fix(frontend): add missing zh dashboard labels
  - `9ca7e522` fix(frontend): complete v117 zh locale coverage
- 宸茬‘璁や笂娓?tag `v0.1.117` 鍐?`backend/cmd/server/VERSION` 浠嶄负 `0.1.116`锛屽洜姝ら〉闈㈠乏涓婅鏄剧ず `v0.1.116` 鏄笂娓哥増鏈枃浠舵粸鍚庯紝涓嶄唬琛ㄨ繍琛岄敊鍒嗘敮銆?
- 鏈湴楠岃瘉鏈嶅姟锛?
  - 鍓嶇锛歚http://localhost:5180`
  - 鍚庣锛歚http://localhost:18082`
  - 鍚庣闇€瑕佷互 `RUN_MODE=standard` 杩愯锛屽惁鍒欑鐞嗗憳渚ф爮浼氶殣钘忔笭閬撶鐞嗙瓑鑿滃崟銆?

**鍙樻洿璇︽儏**:
- 榛樿璇█鏀逛负涓枃锛屽苟淇 vue-i18n 鎻掑€兼牸寮忥紝灏?`${amount}` 杩欑被鍐欐硶鏀逛负 `{amount}`銆?
- 琛ラ綈鐧诲綍椤典腑鏂?key锛岄伩鍏嶉娆℃墦寮€鐧诲綍椤垫樉绀?`auth.login.*`銆?
- 琛ラ綈浠〃鐩樺揩鎹峰叆鍙ｄ腑鏂?key銆?
- 琛ラ綈 v117 鏂板/浜屽紑椤甸潰涓枃 key锛岃鐩栭〉闈㈠唴瀹广€佺櫥褰曢〉閰嶇疆銆佸畾浠烽〉閰嶇疆銆佹ā鍨嬮厤缃€佹ā鍨嬪畾浠枫€丄PI Key 浣跨敤寮曞銆佽处鍙?鐢ㄦ埛/浠ｇ悊/浣跨敤璁板綍銆佸厖鍊?鏀粯/瀹氫环椤电瓑鍖哄煙銆?
- 涓轰唬鐮佷腑鐩存帴寮曠敤浣嗚嫳鏂囧寘涔熺己澶辩殑 `common.done` 鍚屾琛ュ厖 en/zh 鏂囨銆?

**楠岃瘉缁撴灉**:
- `pnpm typecheck` 閫氳繃銆?
- i18n key 瀵规瘮缁撴灉锛歚missing zh count 0`銆?
- 娴忚鍣ㄨ嚜鍔ㄥ寲鎶芥煡閫氳繃锛歚/pricing`銆乣/keys`銆乣/admin/model-config`銆乣/admin/page-content`銆乣/admin/users`銆乣/admin/accounts`銆乣/admin/proxies`銆乣/admin/usage` 鍧囨湭鍙戠幇 raw i18n key锛屼篃鏃?intlify missing-key 璀﹀憡銆?
- 鎶芥煡绠＄悊鍛樼櫥褰曟€佷晶鏍忓畬鏁存樉绀猴細浠〃鐩樸€佽繍缁寸洃鎺с€佺敤鎴风鐞嗐€佸垎缁勭鐞嗐€佹笭閬撶鐞嗐€佽闃呯鐞嗐€佽处鍙风鐞嗐€佹ā鍨嬮厤缃€侀〉闈㈠唴瀹广€佽鍗曠鐞嗐€佸厖鍊奸厤缃瓑銆?

**鍓╀綑娉ㄦ剰浜嬮」**:
- 濡傛灉娴忚鍣ㄤ粛鏄剧ず灏戦噺鑿滃崟鎴栧彉閲忓悕锛屼紭鍏堟竻鐞嗘棫 localStorage / 閫€鍑洪噸鐧伙紱涔嬪墠 simple-mode 鐧诲綍鎬佸彲鑳界紦瀛樹簡 `run_mode='simple'`銆?
- 涓存椂 Playwright 鍙敤浜庢湰鍦版娊鏌ワ紝宸蹭粠渚濊禆涓Щ闄わ紝鏈繚鐣欏湪 `package.json`銆?

## [2026-05-01] docs: 鏂板 Codex 鍒濆鍖栬鏄?

**褰卞搷鑼冨洿**:
- `AGENTS.md`
- `docs/dev/CHANGELOG_CUSTOM.md`

**涓婃父鍏煎鎬?*:
- Low. Documentation-only change.

**鍙樻洿璇︽儏**:
- 鍩轰簬 `CLAUDE.md` 鎻愮偧 Codex 鍏ュ彛璇存槑锛屼繚鐣欐灦鏋勪紭鍏堛€乧odebase 鏂囨。娌夋穩銆乸npm-only銆丒nt/Wire 鐢熸垚銆乸ush/deploy 闇€鎺堟潈绛夎鍒?
- 鏂板鍏抽敭鏂囦欢绱㈠紩锛屽叧鑱斿悗绔叆鍙ｃ€佺綉鍏崇儹璺緞銆丒nt/migrations銆佸墠绔叆鍙ｃ€侀儴缃插拰宸ュ叿鏂囦欢
- 鏍￠獙鍏抽敭璺緞骞剁Щ闄ゅ綋鍓?checkout 涓笉瀛樺湪鐨?`deploy/remote_exec.py`銆乣tools/secret_scan.py` 浣滀负鍏抽敭鏂囦欢寮曠敤

## [2026-05-01] fix(frontend): cache_transfer_ratio 鍜?display_rate_multiplier 鏃犳硶淇敼

**褰卞搷鑼冨洿**:
- `frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue`
- `frontend/src/components/admin/user/UserModelPricingModal.vue`

**涓婃父鍏煎鎬?*:
- Low. Frontend-only change.

**鍙樻洿璇︽儏**:
- `Number(val) || null` 妯″紡灏?`0` 璇浆涓?`null`锛屽悗绔樊閲忔洿鏂?`if != nil` 璺宠繃璇ュ瓧娈碉紝瀵艰嚧鍊兼棤娉曡淇敼涓?0
- 鏇挎崲涓?`toNullableNum()` 杈呭姪鍑芥暟锛氱┖鍊?NaN 鈫?null锛屾湁鏁堟暟瀛楋紙鍚?0锛夆啋 number
- 鍚屾椂淇浜嗗叏灞€妯″瀷瀹氫环 dialog 鍜岀敤鎴风骇瀹氫环 modal 涓ゅ

## [2026-05-01] fix(display): skip cache transfer for channel-override usage logs

**褰卞搷鑼冨洿**:
- `backend/internal/handler/dto/display_pricing.go` 鈥?add `stripCacheTransferIfChannel` helper
- `backend/internal/handler/dto/mappers.go` 鈥?call helper in `UsageLogFromService` and `UsageLogFromServiceAdmin`

**涓婃父鍏煎鎬?*:
- Low. Changes are in dto layer display logic only.

**鍙樻洿璇︽儏**:
- 褰?usage log 缁忚繃娓犻亾璁¤垂锛圕hannelID 闈炵┖锛夋椂锛宒isplay transform 涓嶅啀搴旂敤鍏ㄥ眬鐨?CacheTransferRatio
- 淇浜嗘笭閬撹鐩栦环鏍间絾缂撳瓨杞Щ浠嶇敓鏁堝鑷寸敤鎴风湅鍒扮殑 token 鍒嗗竷涓庡疄闄呰璐逛笉涓€鑷寸殑 bug

## [2026-04-30] feat(admin): add cache status dashboard module

**褰卞搷鑼冨洿**:
- `backend/internal/handler/admin/dashboard_handler.go` 鈥?add `/admin/dashboard/cache-status` handler.
- `backend/internal/repository/usage_log_repo.go` 鈥?aggregate cache read/create stats from `usage_logs`.
- `frontend/src/views/admin/DashboardView.vue` 鈥?add admin dashboard cache status module.
- `frontend/src/api/admin/dashboard.ts` / `frontend/src/i18n/locales/*` 鈥?add API types and copy.

**涓婃父鍏煎鎬?*:
- Low. This is an additive admin dashboard feature; likely conflicts only if upstream edits the same dashboard files.

**鍙樻洿璇︽儏**:
- Add cache read rate, cache creation rate, request hit rate, prompt token total, trend buckets, and per-model cache status.
- Support `1h`, `6h`, `24h`, and `7d` windows. Default platform is `antigravity`, with an `all` option.
- Status levels: `insufficient` for fewer than 5 requests, `healthy` for read rate >= 50%, `watch` for 20%-50%, and `unhealthy` below 20%.

## [2026-04-30] fix(repository): restore Redis concurrency slot Lua compatibility

**褰卞搷鑼冨洿**:
- `backend/internal/repository/concurrency_cache.go` 鈥?remove `TIME` calls from write-capable Redis Lua scripts.

**涓婃父鍏煎鎬?*:
- Low. The behavior and key layout are unchanged; only the timestamp source moves from Redis Lua to Go.

**鍙樻洿璇︽儏**:
- Pass current Unix seconds from Go into `acquireScript`, `getCountScript`, and `cleanupExpiredSlotsScript`.
- Fix Redis error `Write commands not allowed after non deterministic commands`, which caused `gateway.user_slot_acquire_failed` and immediate IDE retry on `/antigravity/v1/messages`.
- Verified locally with `claude-opus-4-7` Antigravity messages endpoint returning 200 through `http://127.0.0.1:8081`.

## [2026-04-30] fix(antigravity): stabilize Claude Opus cache inputs

**褰卞搷鑼冨洿**:
- `backend/internal/pkg/antigravity/request_transformer.go` 鈥?normalize cache-sensitive request fields before forwarding to Antigravity v1internal.
- `backend/internal/pkg/antigravity/request_transformer_test.go` 鈥?add regression tests for billing-header filtering and metadata session normalization.

**涓婃父鍏煎鎬?*:
- Low. The change is scoped to Antigravity Claude request transformation; upstream sync conflicts should be limited to the same transformer tests if upstream edits this area.

**鍙樻洿璇︽儏**:
- Drop dynamic `x-anthropic-billing-header` system lines before building `systemInstruction`, so per-request `cch=` changes do not perturb the upstream implicit cache key.
- Normalize JSON-form `metadata.user_id` from new Claude CLI clients. Prefer stable `device_id`, fall back to `session_id`, and preserve plain string user IDs.
- Keeps non-billing system text intact and preserves existing generated fallback session IDs when metadata is absent.

## [2026-04-28] fix(antigravity): 鏄惧紡鍖栨ā鍨嬫槧灏勫垹闄ゅ叆鍙ｅ苟闅愯棌宸插瓨鍦ㄩ璁?

**褰卞搷鑼冨洿**:
- `frontend/src/components/account/CreateAccountModal.vue` - Antigravity 璐﹀彿鏂板缓寮圭獥鐨勬槧灏勫垹闄ゆ寜閽敼涓烘樉寮忔枃瀛楁寜閽紝棰勮鎸夐挳闅愯棌宸插瓨鍦ㄦ槧灏勩€?
- `frontend/src/components/account/EditAccountModal.vue` - Antigravity 璐﹀彿缂栬緫寮圭獥鍚屾涓婅堪浜や簰銆?
- `frontend/src/components/admin/model-pricing/AntigravityMappingCard.vue` - 鍏ㄥ眬 Antigravity 榛樿鏄犲皠缂栬緫椤电殑鍒犻櫎鍏ュ彛鏀逛负鏄惧紡鏂囧瓧鎸夐挳銆?

**涓婃父鍏煎鎬?*:
- 绾墠绔氦浜掍紭鍖栵紝涓嶆敼鍙樺悗绔槧灏勮В鏋愯鍒欙紱鍚屾涓婃父鏃朵綆鍐茬獊銆?

**鍙樻洿璇︽儏**:
- 瑙ｅ喅 Antigravity 鏄犲皠涓嚭鐜?`claude-opus-4.7` / `claude-opus-4-7` 绫讳技閲嶅椤规椂锛岀敤鎴烽毦浠ュ彂鐜板垹闄ゅ叆鍙ｇ殑闂銆?
- 璐﹀彿寮圭獥涓 Claude 4.x 鐐瑰彿/鐭í绾垮啓娉曞仛鍚岀被鏄犲皠鍒ゆ柇锛岄伩鍏嶅揩鎹烽璁惧啀娆℃樉绀烘垨娣诲姞鍚岀被閲嶅鏄犲皠銆?
- `妯″瀷閰嶇疆` 涓昏〃鎿嶄綔鍒楄ˉ鍏呯洿鎺ョ殑鈥滃垹闄ゆ槧灏勨€濇寜閽紝閬垮厤蹇呴』鍏堟墦寮€鏄犲皠缂栬緫 popover 鎵嶈兘鍒犻櫎銆?

## [2026-04-28] fix(antigravity): 鏇存柊榛樿瀹㈡埛绔増鏈埌 1.23.2

**褰卞搷鑼冨洿**:
- `backend/internal/pkg/antigravity/oauth.go` 鈥?榛樿 `ANTIGRAVITY_USER_AGENT_VERSION` 浠?`1.21.9` 鏇存柊鍒?`1.23.2`
- `backend/internal/pkg/antigravity/oauth_test.go` 鈥?鏇存柊榛樿 User-Agent 鏂█
- `deploy/docker-compose.yml` 鈥?閫忎紶 `ANTIGRAVITY_USER_AGENT_VERSION`
- `deploy/.env.example` 鈥?琛ュ厖 Antigravity User-Agent 鐗堟湰閰嶇疆璇存槑

**涓婃父鍏煎鎬?*:
- 浣庨闄╋紱浠呮洿鏂伴粯璁?User-Agent 鐗堟湰锛屼粛鍏佽杩愯鐜閫氳繃 `ANTIGRAVITY_USER_AGENT_VERSION` 瑕嗙洊銆?

**鍙樻洿璇︽儏**:
- Google Antigravity 涓嬭浇椤靛綋鍓?stable 涓嬭浇璺緞涓?`stable/1.23.2-...`锛屾湰鍦伴粯璁や粛涓?`antigravity/1.21.9 windows/amd64`銆?
- 涓婃父杩斿洖 `This version of Antigravity is no longer supported. Please upgrade to receive the latest features.` 鏃讹紝浼樺厛鎬€鐤?User-Agent 鐗堟湰杩囨棫銆?
- 鏇存柊榛樿鍊煎苟琛ュ厖閮ㄧ讲鐜鍙橀噺锛岄伩鍏嶇敓浜у鍣ㄥ洜鏈樉寮忚缃増鏈€岀户缁娇鐢ㄦ棫瀹㈡埛绔寚绾广€?

## [2026-04-27] feat(antigravity): 娣诲姞缂撳瓨璇婃柇鏃ュ織

**褰卞搷鑼冨洿**:
- `backend/internal/config/config.go` 鈥?Gateway struct 鏂板 `LogCacheDiagnostics` 瀛楁 + Viper 榛樿鍊兼敞鍐?
- `backend/internal/pkg/antigravity/request_transformer.go` 鈥?鏂板 `CacheDiagnostics` 缁撴瀯浣撳拰 `ExtractCacheDiagnostics()` 鍑芥暟
- `backend/internal/service/antigravity_gateway_service.go` 鈥?Forward() 涓坊鍔犺姹?鍝嶅簲闃舵璇婃柇鏃ュ織

**涓婃父鍏煎鎬?*:
- 绾柊澧烇紝涓嶅奖鍝嶄笂娓稿悎骞?

**鍙樻洿璇︽儏**:
- 鑳屾櫙锛歝laude-opus-4-7 璇锋眰缁?Antigravity 骞冲彴杞彂鍚?0% 缂撳瓨鍛戒腑锛岃€屽悓璺緞鐨?claude-opus-4-6 鏈?99.7% 缂撳瓨鍛戒腑鐜?
- 鏂板 `gateway.log_cache_diagnostics` 閰嶇疆寮€鍏筹紙榛樿鍏抽棴锛夛紝鐢熶骇鐜閫氳繃 `GATEWAY_LOG_CACHE_DIAGNOSTICS=true` 鍚敤
- 寮€鍚悗璁板綍锛歴essionId銆乻ystemInstruction hash/prefix/per-part hash銆乧ontents 缁撴瀯銆乽nstable_part 鏄庢枃
- 鍚屾椂璁板綍涓婃父杩斿洖鐨?cache_read/cache_creation tokens

**璋冪爺缁撹锛堟埅鑷?2026-04-30锛?*:

缁忓杞凯浠ｈ瘖鏂紝瀹氫綅鍒颁笂娓搁殣寮忕紦瀛樺け鏁堢殑涓や釜鐙珛鍥犵礌锛?

1. **systemInstruction 涓?`x-anthropic-billing-header` block 鐨?`cch=` 瀛楁姣忔璇锋眰閮藉彉**
   - Claude Code CLI 鍦?system prompt 鏁扮粍鐨勭涓€涓?text block 娉ㄥ叆 `x-anthropic-billing-header: cc_version=2.1.12x.xxx; cc_entrypoint=cli; cch=xxxxx;`
   - `cch`锛坈ontext content hash锛夋瘡杞璇濋兘鍙橈紝瀵艰嚧 systemInstruction 鐨?Part[2] hash 涓嶇ǔ瀹?
   - 浣嗕粠鏁版嵁鐪嬶紝閮ㄥ垎甯?billing header 鐨勮姹備粛鐒惰兘鍛戒腑缂撳瓨锛岃鏄庝笂娓哥紦瀛樹笉瀹屽叏渚濊禆 system instruction prefix 鍖归厤
   - 淇鏂瑰悜锛氬湪 `buildSystemInstruction` 涓繃婊?`x-anthropic-billing-header` 寮€澶寸殑 system block

2. **`metadata.user_id` JSON 琚暣涓敤浣?sessionId**
   - 鏂扮増 Claude CLI 鍙戦€?`metadata.user_id = {"device_id":"...","account_uuid":"","session_id":"xxx"}`
   - `request_transformer.go:161-163` 灏嗘暣涓?JSON 瀛楃涓茬洿鎺ヨ祴鍊肩粰 `innerRequest.SessionID`
   - 鑳藉懡涓紦瀛樼殑璇锋眰锛歚metadata_user_id` 涓虹┖锛坰essionId 鏄暟瀛?hash锛夋垨鍙湁 `device_id`锛堟棤 session_id 瀛楁锛?
   - 涓嶈兘鍛戒腑缂撳瓨鐨勮姹傦細`metadata_user_id` 鍖呭惈 `session_id` UUID锛堟瘡涓?Claude Code 浼氳瘽涓嶅悓锛?
   - 淇鏂瑰悜锛氫粠 JSON 涓彁鍙?`session_id` 瀛楁鍗曠嫭浣跨敤锛屾垨浠呯敤 `device_id` 浣滀负 sessionId

**淇鐘舵€?*锛?026-04-30 宸插湪 `request_transformer.go` 钀藉湴杩囨护 billing header 涓庤鑼冨寲 `metadata.user_id`锛岃瘖鏂棩蹇楀紑鍏冲彲鍦ㄧ敓浜ч獙璇佺紦瀛樺懡涓悗鍏抽棴銆?

## [2026-04-27] feat(openai): 娣诲姞 GPT-5.5 / GPT-5.5 Pro 妯″瀷鏀寔

**褰卞搷鑼冨洿**:
- `backend/internal/pkg/openai/constants.go` 鈥?DefaultModels 鍒楄〃
- `backend/internal/service/openai_codex_transform.go` 鈥?codexModelMap + normalizeCodexModel
- `backend/internal/service/billing_service.go` 鈥?fallback 瀹氫环銆乬etFallbackPricing銆乮sOpenAIGPT54Model
- `backend/resources/model-pricing/model_prices_and_context_window.json` 鈥?鍔ㄦ€佸畾浠锋潯鐩?

**涓婃父鍏煎鎬?*:
- 涓婃父 v0.1.112 灏氭湭娣诲姞 GPT-5.5 鏀寔锛涗笂娓歌嫢鍚庣画娣诲姞闇€浜哄伐瀵归綈鍥涘鏂囦欢

**鍙樻洿璇︽儏**:
- 鑳屾櫙锛歄penAI 浜?2026-04-23 鍙戝竷 GPT-5.5锛屼笂娓告湭璺熻繘锛涘師 normalizeCodexModel 涓?`gpt-5.5` 浼氳 `gpt-5` 鍏滃簳閫昏緫闈欓粯闄嶇骇涓?`gpt-5.1`锛屽鑷磋姹備笉閫?
- 鏂板妯″瀷锛歚gpt-5.5`锛?5/$30 per MTok锛夈€乣gpt-5.5-pro`锛?30/$180 per MTok锛?
- codexModelMap 鍖呭惈 reasoning effort 鍚庣紑鍙樹綋锛坣one/low/medium/high/xhigh锛夊強 chat-latest
- 闀夸笂涓嬫枃瀹氫环澶嶇敤 GPT-5.4 鐨勯槇鍊硷紙272K input tokens, 2x input / 1.5x output锛?

## [2026-04-21] ops(deploy): 涓?docker-compose 涓変釜鏈嶅姟鍔犳棩蹇楄疆杞?

**褰卞搷鑼冨洿**:
- `deploy/docker-compose.yml` 鈥?`sub2api` / `postgres` / `redis` 鍚勫姞 `logging: { driver: json-file, options: { max-size: 50m, max-file: 5 } }`

**涓婃父鍏煎鎬?*:
- 浠呰拷鍔犲瓧娈碉紝涓嶆敼鍔ㄦ棦鏈夐厤缃紱涓婃父鑻ラ噸鍐?compose 缁撴瀯闇€浜哄伐瀵归綈姝や笁娈?

**鍙樻洿璇︽儏**:
- 鑳屾櫙锛?026-04-20 鏅?23:01 鐢熶骇鏈虹鐩樺啓婊″鑷村畷鏈猴紙`rsyslogd: No space left on device`锛夛紝鏍瑰洜鏄?Docker 榛樿 `json-file` 鏃ュ織椹卞姩鏃犺疆杞笂闄愶紝`sub2api` 瀹瑰櫒鎸?~4.3 GB/澶╃疮绉紝8 澶╃疮璁?~37 GB锛岃€楀敖鏍圭洏锛涢噸鍚悗 `docker compose up` 閲嶅缓瀹瑰櫒椤哄甫鍒犻櫎鏃?`*-json.log`锛岀鐩樻墠浠?100% 闄嶅洖 45%
- 淇锛氭瘡瀹瑰櫒涓婇檺 5 脳 50 MB = 250 MB锛屼笁瀹瑰櫒鍚堣鏈€澶?~750 MB锛屼粠姝や笉浼氬啀琚鍣ㄦ棩蹇楁墦鐖嗙鐩?
- 鐢熸晥璺緞锛歝ommit 鈫?push 鈫?`python deploy/remote_exec.py --update`锛坄update.sh` 瑙﹀彂 `docker compose up -d`锛屽鍣ㄩ噸寤烘椂鏂?`logging` 閰嶇疆鎵嶈惤浣嶏級
- 鍚庣画寰呭姙锛氣憼 娓呯悊 15.84 GB build cache 鍜?24 涓?dangling 闀滃儚锛涒憽 `ops_error_logger` 鍦?postgres 涓嶅彲杈炬椂鐤媯閲嶈瘯鍒锋棩蹇楋紝闇€鍔犻€熺巼闄愬埗

## [2026-04-21] docs(sales): 鍒濈増閿€鍞唬鐞嗘墜鍐?

**褰卞搷鑼冨洿**:
- `docs/sales/SALES_HANDBOOK.md` 鈥?**鏂板缓**銆傞潰鍚戠嫭绔嬪紑鍙戣€?/ AI 宸ュ叿涓汉鐢ㄦ埛鐨勯攢鍞唬鐞嗘墜鍐岋紝9 绔狅細浜у搧涓€鍙ヨ瘽 / 鏍稿績鍗栫偣 / 鑳藉姏娓呭崟 / 浣跨敤娴佺▼ / 瀹氫环瑙勫垯 / FAQ / 閿€鍞瘽鏈?/ 瑙﹁揪娓犻亾 / 闄勫綍銆傛墍鏈夊叿浣撻噾棰濓紙姹囩巼銆佹ā鍨嬪崟浠枫€侀鍏呬紭鎯犮€佽繑鐐癸級鐣欑┖锛坄鈻?____`锛夛紝閿€鍞寜褰撴棩鏀跨瓥鐜板満濉啓銆?
- `.gitignore` 娉ㄦ剰锛歚docs/*` 琚拷鐣ワ紝鎻愪氦鏈枃浠堕渶 `git add -f`

**涓婃父鍏煎鎬?*: 绾柊澧炴枃妗ｏ紝涓庝笂娓告棤鍐茬獊锛沗docs/sales/` 鏄簩寮€涓撳睘鐩綍

**鍙樻洿璇︽儏**:
- 鍗栫偣鏉ユ簮浜庝唬鐮佷簨瀹烇紙涓夊崗璁吋瀹广€佺矘鎬т細璇濄€佺啍鏂€佸鏀粯閫氶亾銆乀OTP銆並ey 绾ч搴︼級锛屾棤鑷嗛€?
- 瀹氫环绔犺妭鍙啓鏈哄埗锛坱oken 鍙屽悜 / cache hit / 闀夸笂涓嬫枃鍊嶇巼 / Priority-Flex 妗ｄ綅 / USD鈫扖NY锛夛紝涓嶅啓鏁板瓧
- FAQ 鎸夊敭鍓?/ 鎺ュ叆 / 璁¤垂 / 绋冲畾鎬?/ 瀹夊叏浜旂粍锛涘惈 Claude Code + Cursor 鍏蜂綋鎺ュ叆鍛戒护
- 璇濇湳鍚笁涓紑鍦虹増鏈?+ 浜斿ぇ寮傝搴斿 + 涓撮棬涓€鑴氭ā鏉?

**鍏宠仈 Issue/PR**: 鈥?

---

## [2026-04-20] fix: 淇 Gemini 璐︽埛 OAuth 鍒锋柊 Token 瓒呮椂

**褰卞搷鑼冨洿**: backend/internal/service/account.go
**涓婃父鍏煎鎬?*: 鍙兘涓庝笂娓稿悓鍖哄煙淇敼鍐茬獊锛屽悎骞舵椂娉ㄦ剰
**鍙樻洿璇︽儏**:
- OAuth token refresh 瓒呮椂浠?10s 鏀逛负 30s
- 鏂板閲嶈瘯閫昏緫锛堟渶澶?3 娆★紝鎸囨暟閫€閬匡級

**鍏宠仈 Issue/PR**: 鏃狅紙绾夸笂鎺掓煡鍙戠幇锛?
-->

## [2026-04-19] feat(admin/usage): "鐢ㄦ埛瑙嗚瀵规瘮"鎶藉眽鍓嶇娈?

**褰卞搷鑼冨洿**:
- `frontend/src/api/admin/usage.ts` 鈥?鏂板 `getUserViewPreview(logId)` API 涓?`UserViewPreview` / `UserViewSnapshot` / `UserViewConfigUsed` 绫诲瀷锛涙寕杞藉埌 `adminUsageAPI` 榛樿瀵煎嚭
- `frontend/src/components/admin/usage/UserViewCompareDrawer.vue` 鈥?**鏂板缓**銆傚熀浜?`BaseDialog` 鐨?extra-wide 瀵硅瘽妗嗭紝灞曠ず real / user_view 鍙屽垪瀵规瘮 + 宸紓%锛涘垎缁勶細Tokens / Costs / Invariants锛涢《閮ㄥ睍绀?`config_used`锛堝惈 `has_user_override` badge锛夛紱actual_cost 涓嶄竴鑷存椂绾㈣壊鍛婅
- `frontend/src/components/admin/usage/UsageTable.vue` 鈥?鏂板 `userViewClick` emit 涓?`<template #cell-actions>` 娓叉煋 eye 鎸夐挳
- `frontend/src/views/admin/UsageView.vue` 鈥?`allColumns` 鏈熬鏂板 `actions` 鍒楋紱`ALWAYS_VISIBLE` 鍖呭惈 `actions`锛涙柊澧?`userViewLogId/userViewOpen/handleUserViewClick/closeUserViewDrawer` 鐘舵€佷笌澶勭悊锛沗<UsageTable>` 鐩戝惉 `@userViewClick`锛涙ā鏉挎湯鎸傝浇 `<UserViewCompareDrawer>`
- `frontend/src/i18n/locales/zh.ts`銆乣en.ts` 鈥?`admin.usage` 鑺傜偣鏂板 actions/viewUserPerspective/userView* 绛?16 涓?key

**涓婃父鍏煎鎬?*:
- 浠呰拷鍔犲垪涓庣粍浠讹紝鏈敼鍔ㄧ幇鏈夊垪娓叉煋锛涗笂娓歌嫢鏀瑰姩 admin usage 琛ㄧ殑鍒楃粨鏋勶紝闇€瑕佹妸 `actions` 鍒楄拷鍔犻噸鍋氬嵆鍙?

**鍙樻洿璇︽儏**:
- 涓庢槰鏃ュ悗绔 `GET /admin/usage/:id/user-view` 閰嶅锛岄棴鐜簡"绠＄悊鍛樺悗鍙扮洿鎺ョ湅鐢ㄦ埛鍓嶇瑙嗚"鐨勫伐浣滄祦鈥斺€旂鐞嗗憳鐐瑰嚮琛屽熬 eye 鍥炬爣 鈫?鎶藉眽鎷夋帴鍙?鈫?宸﹀彸瀵规瘮 real(绠＄悊鍛樿瑙? vs user_view(鐢ㄦ埛瀹為檯鐪嬪埌)锛屽苟鏍囨敞鍝簺 display 閰嶇疆鐢熸晥锛堝惈鍏ㄥ眬 vs 鐢ㄦ埛瑕嗙洊鏉ユ簮锛?
- 鎶藉眽鑷姩闅愯棌鍏?0 瀛楁娈碉紝閬垮厤鍣煶锛沝iff 鍒椾互绾?缁?+ 鐧惧垎姣旇〃杈炬斁澶?缂╁皬
- `pnpm typecheck` 閫氳繃锛沗pnpm build` 鍦ㄤ笌鏈敼鍔ㄦ棤鍏崇殑 PricingView.vue 涓婃湁 cnyRate TS 閿欙紙浼氳瘽寮€濮嬪墠宸插瓨鍦ㄧ殑鏈彁浜ゆ敼鍔級锛屼笉闃诲褰撳墠娈?

## [2026-04-19] feat(admin/usage): 鏂板"鐢ㄦ埛瑙嗚"瀵规瘮棰勮鎺ュ彛锛堝悗绔锛?

**褰卞搷鑼冨洿**:
- `backend/internal/handler/admin/usage_handler.go` 鈥?`UsageHandler` 鏂板 `userModelPricingService` 渚濊禆锛涙柊澧?`GetUserViewPreview` handler 涓庨厤濂?DTO锛坄UserViewPreviewResponse` / `UserViewSnapshot` / `UserViewConfigUsed` / `snapshotFromDTO`锛?
- `backend/internal/server/routes/admin.go` 鈥?娉ㄥ唽 `GET /api/v1/admin/usage/:id/user-view`
- `backend/cmd/server/wire_gen.go` 鈥?`admin.NewUsageHandler` 璋冪敤澧炶ˉ `userModelPricingService` 鍙傛暟锛坄go generate` 鍥犻」鐩?Wire 宸插瓨鍦ㄧ殑澶氱粦瀹氶棶棰樺け璐ワ紝鏁呮墜鍔?patch锛涗笉褰卞搷鍔熻兘锛?
- `backend/internal/handler/admin/usage_cleanup_handler_test.go`銆乣usage_handler_request_type_test.go` 鈥?鍚屾 `NewUsageHandler` 鏂扮鍚嶏紙澶氫紶涓€涓?nil锛?

**涓婃父鍏煎鎬?*:
- 绾柊澧炵鐐?+ 鏋勯€犲嚱鏁版湯浣嶅墠涓€浣嶆彃鍙傦紝涓庝笂娓?admin usage handler 鏀瑰姩鍙兘浜х敓灏忓啿绐侊紝浣嗗弬鏁伴『搴忓彉鍖栧鏄撹瘑鍒?

**鍙樻洿璇︽儏**:
- 鐩殑锛氱鐞嗗憳鎺掓煡鏌愪釜鐢ㄦ埛锛堝 gybilly2023锛?鍓嶇瀹為檯鐪嬪埌鐨?token / 鎴愭湰"鏄惁绗﹀悎 `cache_transfer_ratio` + `display_input_price` 绛?濂稿晢"閰嶇疆棰勬湡锛岀洰鍓嶅敮涓€鍔炴硶鏄櫥褰曡鐢ㄦ埛璐﹀彿浜茬溂鐪?
- 鏂版帴鍙ｅ鍗曟潯 usage_log 閲嶆柊璺戜笁灞?transform锛氬叏灞€ display 浠?鈫?user model overrides锛坄BuildUserDisplayPricingMap`锛夆啋 user group display rate锛坄ApplyUserDisplayRate`锛夛紝杩斿洖 `real` / `user_view` 涓ゅ垪瀵规瘮 + `config_used` 閰嶇疆婧簮锛堝惈 `has_user_override`銆乣user_group_rate`锛?
- 瀹屽叏澶嶇敤 `dto.UsageLogFromService` / `ApplyDisplayTransform` / `ApplyUserDisplayRate` / `BuildUserDisplayPricingMap`锛屼笉鍐欐柊璁＄畻閫昏緫
- 涓嶅姩鐜版湁鍒楄〃鏌ヨ閫昏緫鈥斺€擿AdminUsageLog.DisplayFields` 浠嶆寜鍏ㄥ眬 displayMap 绠楋紙淇濇寔鍚戝悗鍏煎锛?
- 宸叉湰鍦?`go run ./cmd/server` 楠岃瘉璺敱姝ｇ‘娉ㄥ唽銆丟in 鏃?radix 鍐茬獊 panic
- 鍓嶇鍏ュ彛涓庢娊灞?UI 寰呬笅涓€娈垫彁浜?

## [2026-04-19] feat(pricing): 妯″瀷浠锋牸琛ㄥ悓鏃跺睍绀?CNY 瀹炰粯閲戦锛堟寜鍏呭€肩鐞嗘崲绠楃巼锛?

**褰卞搷鑼冨洿**:
- `frontend/src/views/user/PricingView.vue` 鈥?浠锋牸琛ㄥ崱鐗囬《閮ㄥ姞 USD鈫扖NY 鎹㈢畻 banner锛堜粎鍦?`payment_cny_per_usd > 0` 鏃舵樉绀猴級锛沗formatTokenPrice` / `formatPerRequest` 鎷嗕负 `tokenPrimary`/`tokenSecondary` + `perRequestPrimary`/`perRequestSecondary` 鍥涗釜 helper锛欳NY 涓虹矖浣撲富鏄剧ず锛孶SD 鍔犳嫭鍙峰皬鐏板瓧鍓樉绀猴紱鏈厤缃崲绠楃巼鏃惰嚜鍔ㄩ€€鍖栦负鍗曚竴 USD 鏄剧ず
- `frontend/src/i18n/locales/{zh,en}.ts` 鈥?鏂板 `pricing.cnyBanner`锛涘垪澶村幓鎺夌‖缂栫爜 `$/MTok` 鏀逛负銆岃緭鍏ヤ环 / MTok銆嶃€孖nput / MTok銆嶈鍗曞厓鏍艰嚜甯﹀竵绉嶇鍙凤紱`unitHint` 鏀瑰啓涓鸿鏄?楼 / $ 鍚箟鐨勫弻甯佺鏂囨

**鏂囨**锛氱敤鎴锋巿鏉冭寖鍥村唴鐨勫睍绀烘€ф枃瀛楋紙banner 鏂囨銆佸崟浣嶈鏄庯級锛屼笉鍔?i18n 閲屽叾浠栦笟鍔℃枃妗堛€?

**涓婃父鍏煎鎬?*: 浣庛€傜函鍓嶇 + i18n 琛屽唴淇敼銆?

**鍙樻洿璇︽儏**:
1. 瑙嗚绛栫暐锛欳NY 涓汇€乁SD 杈呫€傛瘡涓环鏍煎崟鍏冩牸 `楼3.50 ($5.00)` 鍚岃锛涘乏渚х矖浣?CNY 鏄敤鎴峰疄闄呮墸璐归噺绾э紝鍙虫嫭鍙峰唴鐏板瓧 $ 鏄函婧愪緷鎹?
2. 椤堕儴涓€娆℃€?banner 璇存槑鎹㈢畻鐜囷紙`楼0.7 / 1 USD 路 鏉ヨ嚜鍏呭€肩鐞哷锛夛紝鍗曞厓鏍奸噷灏变笉閲嶅"脳 0.7"
3. 閫€鍖栭€昏緫锛氱鐞嗗憳鏈厤缃?`payment_cny_per_usd`锛堝€间负 0 鎴?null锛夆啋 banner 鑷姩闅愯棌銆佹墍鏈夊崟鍏冩牸鍙樉绀?USD锛屼笌鏀瑰姩鍓嶅畬鍏ㄤ竴鑷达紝閬垮厤鍑虹幇 `楼0` 涔嬬被鐨勫紓甯?
4. 鎬т环姣斿姣旓紙脳10銆佸畼鏂逛环 脳 0.7 绛夛級宸插湪涓婃柟璁′环妯″紡璇存槑閲岃杩囷紝浠锋牸琛ㄦ湰韬笉鍐嶅彔鍔?甯傚満甯歌浠?鍒楋紝淇濇寔琛ㄦ牸骞插噣

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹傦紙鎺?pricing-page 鏂囨鏀归€狅級

---

## [2026-04-19] docs(architecture): 鏂板椤圭洰鎶€鏈灦鏋勬枃妗?+ CLAUDE.md 瑙勫垯

**褰卞搷鑼冨洿**:
- `docs/dev/ARCHITECTURE.md` 鈥?鏂板銆傞《灞傚叆鍙ｆ枃妗ｏ紝瑕嗙洊鎶€鏈爤銆佸墠鍚庣鐩綍鍒嗗眰銆佽姹傜敓鍛藉懆鏈熴€乄ire DI 瑁呴厤鏂瑰紡銆丼ettings/PublicSettings KV 妯″紡銆佽縼绉荤害瀹氥€佺紦瀛樼瓥鐣ャ€佽璇佹巿鏉冦€佹ā鍨嬪畾浠疯В鏋愶紱鍓嶇鐨勮矾鐢?store/api client/甯冨眬/i18n/鍙嶉绾﹀畾锛? 涓父瑙佸紑鍙戜换鍔＄殑銆屾妱鍐欏紡銆嶆ā鏉匡紙鏂板 setting 瀛楁 / 鏂板瀛愮粨鏋?setting / 鏂板鐢ㄦ埛 API / 鏂板 ent 瀛楁 / 鏂板鍓嶇椤?/ 鏂板 i18n 閿級锛涙湰鍦板寲鐨勩€屽凡鐭ュ潙鐐广€嶆竻鍗曪紙Wire 涓诲共澶辫触銆乣docs/dev` gitignore銆丟it Bash POSIX 璺緞鏀瑰啓銆乄indows 绔彛鍐茬獊绛夛級锛涙ā鍧楁繁搴︽枃妗ｅ鑸?
- `docs/dev/codebase/README.md` 鈥?鍦ㄦ渶涓婃柟鍔犱竴娈碉紝鎶婃灦鏋勬枃妗ｅ畾浣嶄负銆屽厛璇绘湰鏋舵瀯銆佸啀鎸夋ā鍧楄〃娣卞叆銆嶇殑鍏ュ彛
- `CLAUDE.md` 鈥?Quick Reference 椤堕儴鍔?ARCHITECTURE.md锛汯ey Development Rules 绗?3 鏉℃柊澧炪€屾帰绱唬鐮佸墠鍏堣 ARCHITECTURE.md銆?銆屼綍鏃舵洿鏂?ARCHITECTURE.md銆嶏紙鏂板妯″潡銆佹敼璺ㄥ垏闈㈢害瀹氥€佸彂鐜版柊鍧戙€佹娊鍑哄彲澶嶇敤妯℃澘鍥涚被瑙﹀彂鏉′欢锛夛紱鍘熴€孋odebase Map銆嶈鍒欑紪鍙蜂粠 3 椤虹Щ鍒?4锛屽悗缁?4鈥?0 鍏ㄩ儴 +1

**涓婃父鍏煎鎬?*: 闆躲€傜函鏂囨。銆?

**鍙樻洿璇︽儏**:
1. 鏂囨。瀹氫綅锛氭灦鏋勬枃妗ｄ笉鏄ā鍧?deep-dive锛岃€屾槸銆岃法鍒囬潰绾﹀畾 + 鍏ュ彛瀵艰埅銆嶃€傛ā鍧楃粏鑺傜户缁斁 `codebase/{module}.md`銆?
2. 妯℃澘绔犺妭锛埪?锛夌洿鎺ユ妱灏辫兘鐢細姣忔潯閮界粰浜嗗叿浣撶殑鏂囦欢璺緞鍜岄『搴忥紝姣斻€岀瓑涓嬫鍙堝緱鐜版懜绱竴閬嶃€嶅揩寰堝銆?
3. 宸茬煡鍧戯紙搂6锛夋妸浼氬弽澶嶈俯鐨?Wire / docs/dev / Git Bash / Windows 绔彛绛変簨鏁呭叏閮ㄦ矇娣€锛岄伩鍏嶄笅娆″張鑺辨椂闂村鐩樸€?

**鍏宠仈 Issue/PR**: 鏃狅紙鏉ヨ嚜浼氳瘽鎬荤粨锛?

---

## [2026-04-19] feat(login-page): 宸︽爮鏀逛负 6 寮犲崱鐗囷紝鍚堝苟鎺ㄥ箍閭€璇峰苟绉婚櫎鍓爣棰樻

**褰卞搷鑼冨洿**:
- `frontend/src/views/auth/LoginView.vue` 鈥?鍒犻櫎鍓爣棰?`<p>` 浠ュ強 `loginDescription` computed锛涚嫭绔嬬殑鎺ㄥ箍閭€璇峰潡绉婚櫎锛沗FeatureKey` 鎵╁埌 6锛堝姞 `tutorial` / `referral`锛夛紱`featureCards` 閰嶇疆鍔犱袱寮犲崱锛堥潚鑹?/ 鐜矇锛夊苟鍚勯厤鍥炬爣锛坆ook-open / gift锛夛紱`featureHighlightTerms{Zh,En}` 琛?tutorial 鍜?referral 涓ょ粍楂樹寒璇嶏紱grid 浠?2脳2 璋冧负 2脳3锛堜粛鏄?`sm:grid-cols-2`锛?
- `frontend/src/i18n/locales/{zh,en}.ts` 鈥?`auth.login.features.*` 鏂板 `tutorial.{title,desc}`锛沗auth.login.referral` 缁撴瀯浠?`{tag,title,body}` 鍚堝苟杩?`features.referral.{title,desc}`锛屾鏂囨寜銆屽彲鍘嬬缉銆嶅師鍒欑簿绠€

**鏂囨**: `features.tutorial` 鏂囧瓧涓ユ牸浣跨敤鐢ㄦ埛缁欏畾鍘熸枃銆俙features.referral.desc` 涓轰笂涓€娆″崰浣嶇鐨勫帇缂╃増锛堟巿鏉冨帇缂╋級銆傚叾浣欏崱鐗囷紙metered / quality / models / enterprise锛夊畬鍏ㄦ病鍔ㄣ€俙auth.login.description` i18n 閿繚鐣欎絾涓嶅啀娓叉煋銆?

**涓婃父鍏煎鎬?*: 浣庛€傜函鍓嶇 + i18n 缁撴瀯璋冩暣銆?

**鍙樻洿璇︽儏**:
1. 鍓爣棰樻锛堛€岄潰鍚戝紑鍙戣€呭拰鍥㈤槦鐨勫妯″瀷涓浆绔欌€︹€︺€嶏級鎸夐渶姹傚垹闄わ紝`auth.login.description` 閿殏鏃朵繚鐣欓伩鍏嶅叾浠栨綔鍦ㄥ紩鐢ㄣ€?
2. 鏂板绗?5 寮犲崱銆屽畬鍠勭殑鍒濆鑰呮暀绋嬨€嶏細闈掕壊锛坄#22D3EE`锛変富棰橈紝book-open 鍥炬爣銆?
3. 鎺ㄥ箍閭€璇蜂粠鐙珛鍧楀彉涓虹 6 寮犲崱锛氱帿绮夛紙`#F472B6`锛変富棰橈紝gift 鍥炬爣銆傛弿杩板帇缂╀负涓€鍙ワ紝銆屼赴鍘氬鍔?/ 鎸佺画杩斾剑銆嶄袱澶勭敤涓婚鑹查珮浜己璋冦€?
4. 鎺掑垪锛歳ow1 = metered + quality锛宺ow2 = models + tutorial锛宺ow3 = enterprise + referral锛屾寜銆屾牳蹇冧环鍊?鈫?浜у搧鑳藉姏 鈫?杩涢樁/鎺ㄥ箍銆嶈嚜鐒舵敹鏉熴€?

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹?

---

## [2026-04-19] style(login-page): 4 寮?feature 鍗¤瑙夊姞閲?+ 鍏抽敭璇嶉珮浜?

**褰卞搷鑼冨洿**:
- `frontend/src/views/auth/LoginView.vue` 鈥?姣忓紶鍗℃柊澧為《閮ㄤ富棰樿壊鍏夊甫銆乣10脳10` 甯﹁壊鍥炬爣鍧椼€乣17px` 绮楁爣棰樸€乣14px` 姝ｆ枃锛涙弿杩伴噷鐗瑰畾鍏抽敭璇嶏紙浠锋牸銆?瓒呴珮鎬т环姣?銆乣Opus 4.7` / `GPT-5.4` / `Gemini 3.1 Pro`銆?寮€绁? 绛夛級鐢?`splitWithTerms` 鍦ㄨ繍琛屾椂鎷嗘骞剁敤涓婚鑹插姞绮楋紱鏂板 `FeatureKey` 绫诲瀷銆乣escapeRegExp`/`splitWithTerms` 杈呭姪鍑芥暟浠ュ強涓嫳涓ゅ楂樹寒璇嶈〃锛涙帹骞块個璇峰潡 padding / 鏍囬瀛楀彿鐣ユ敹锛岃 4 寮犲崱鐗囧湪瑙嗚灞傜骇涓婃洿绐佸嚭

**鏂囨**: 涓嶅彉銆俙auth.login.features.*.{title,desc}` 鍜?`auth.login.referral.*` 鍏ㄩ儴涓庝笂涓€涓彁浜や竴鑷达紝鏈绾瑙夊眰鏀瑰姩銆?

**涓婃父鍏煎鎬?*: 浣庛€傚彧鏀圭櫥褰曢〉鏍锋澘 + 缁勪欢绾у唴閮ㄩ厤缃€?

**鍙樻洿璇︽儏**:
1. 姣忓紶鍗℃湁鐙珛涓婚鑹诧細浠锋牸锛堥潚缁匡級/ 鍝佽川锛堣摑锛? 妯″瀷锛堢传锛? 浼佷笟锛堢惀鐝€锛夛紝鍥炬爣鑳屾櫙 + 楂樹寒璇?+ 椤堕儴 2px 鍏夊甫閮借窡鐫€閰嶈壊鍙樸€?
2. 楂樹寒璇嶆槸瑙嗚瑙勫垯锛屼笉鏄枃妗堬細鐢ㄤ竴浠?`featureHighlightTermsZh|En` 鍦ㄨ剼鏈噷澹版槑锛岃繍琛屾椂鐢ㄦ鍒欐媶鎻忚堪涓诧紝鍖归厤鍒板氨鍖?`<span>` 鍙樼矖鍔犺壊锛沬18n 鏂囨鏀瑰姩鍚庤嫢娌″懡涓紝鍙槸涓嶉珮浜紝涓嶆姤閿欍€?
3. 鍗＄墖 shell锛歚rounded-[22px]` + 娓愬彉搴?+ 鏇村己闃村奖 + hover 鏃跺彉浜紝鏁翠綋浣撻噺鏄庢樉瓒呰繃鎺ㄥ箍鍧椼€?
4. 鎺ㄥ箍鍧楋細padding 浠?`p-5` 璋冨埌 `px-5 py-4`锛屾爣棰?18鈫?6锛岃瑙嗚鐒︾偣钀藉湪 4 寮犲崱鐗囦笂銆?

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹傦紙鎺ヤ笂鏉?feature 鍗￠噸璁捐锛?

---

## [2026-04-19] feat(login-page): 宸︽爮钀ラ攢鍖烘敼鐗堬細4 寮?feature 鍗?+ 鎺ㄥ箍閭€璇?

**褰卞搷鑼冨洿**:
- `frontend/src/views/auth/LoginView.vue` 鈥?鍒犻櫎宸︽爮涓嬪崐鍖虹殑 feature pills銆佹ā鍨嬪睍绀虹綉鏍笺€? 寮犳棫 feature cards 鍜屼笉鍐嶄娇鐢ㄧ殑 `modelChannels` / `paymentCnyPerUsd` / `loginSupportedModelsTitle` / `loginModelsDesc`锛涙柊澧?2脳2 鐨?4 寮?feature 鍗＄墖锛堣绠楀睘鎬?`featureCards`锛変笌鎺ㄥ箍閭€璇峰己璋冨尯鍧?
- `frontend/src/i18n/locales/{zh,en}.ts` 鈥?鏂板 `auth.login.features.{metered,quality,models,enterprise}.{title,desc}` + `auth.login.referral.{tag,title,body}` 涓ょ粍閿紱淇濈暀 `featurePrice`銆乣featureUnifiedApi*` 绛夋棫閿笉鍔紙閬垮厤褰卞搷鍏朵粬缁勪欢 / 闃叉涓婃父鍐茬獊锛夛紝鍙槸鐧诲綍椤垫ā鏉夸笉鍐嶅紩鐢?

**涓婃父鍏煎鎬?*: 浣庛€傚墠绔牱鏉块噸鍐?+ 鏂板 i18n锛涘悗绔€佹暟鎹簱涓嶅姩銆?

**鍙樻洿璇︽儏**:
1. 椤堕儴鍖轰粛鐢?badge / 涓よ鏍囬 / description 缁勬垚锛屾部鐢ㄤ箣鍓嶇殑绠＄悊鍛樺彲缂栬緫瑕嗙洊鏈哄埗锛坄login_page.*` settings 瀛楁锛夈€?
2. 涓嬪崐鍖轰竴娆℃斁瀹?4 寮犲崱鐗?+ 1 寮犳帹骞块個璇峰崱锛岃瑙夊眰绾э細feature 鍗★紙涓€ф繁鑹插簳锛夆啋 鎺ㄥ箍鍗★紙闈掔豢娓愬彉 + 鑽у厜鎻忚竟锛夋妸閲嶇偣鎷夊紑銆?
3. 4 寮犲崱鐗囧綋鍓嶈蛋 i18n 纭紪鐮侊紙鏂囨绋冲畾锛夛紝鍚庣画鑻ラ渶绠＄悊鍛樺彲缂栬緫锛屽姞瀛楁鍒?`LoginPageContent` 鍗冲彲銆?
4. 鎺ㄥ箍閭€璇?`body` 涓哄崰浣嶇锛岀瓑鏈€缁堟枃妗堢‘瀹氬悗鐩存帴鏀?i18n 鎴栧崌绾т负绠＄悊鍛樺彲缂栬緫瀛楁銆?
5. 绠＄悊鍛樼紪杈戝櫒閲岀殑 `supportedModelsTitle`銆乣modelsDesc` 涓ゅ瓧娈垫湰娆¤捣涓嶅啀褰卞搷鐧诲綍椤垫覆鏌擄紙淇濈暀瀛楁鏆備笉鍒狅紝鍚庣画缁熶竴娓呯悊锛夈€?

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹?

---

## [2026-04-18] fix(settings): 鐧诲綍椤典环鏍煎姩鎬佸寲 + 淇鍏呭€肩鐞嗕繚瀛樿娓呯┖娉ㄥ唽绛夎缃?

**褰卞搷鑼冨洿**:
- `backend/internal/service/settings_view.go` 鈥?`PublicSettings` 鏂板 `PaymentCNYPerUSD float64`
- `backend/internal/service/setting_service.go` 鈥?`GetPublicSettings` 璇诲彇 `SettingCNYPerUSD`锛沗GetPublicSettingsForInjection` 娉ㄥ叆鍖垮悕缁撴瀯浣撳悓姝ユ柊澧炲瓧娈?
- `backend/internal/handler/dto/settings.go` 鈥?鍏紑璁剧疆 DTO 鏂板 `payment_cny_per_usd`
- `backend/internal/handler/setting_handler.go` 鈥?鍦?`GetPublicSettings` 鍝嶅簲閲屽～鍏呮柊瀛楁
- `frontend/src/types/index.ts` 鈥?`PublicSettings` 鎺ュ彛鏂板 `payment_cny_per_usd: number`
- `frontend/src/stores/app.ts` 鈥?榛樿绌洪厤缃ˉ榻?`payment_cny_per_usd: 0`
- `frontend/src/i18n/locales/zh.ts`銆乣en.ts` 鈥?`featurePrice` 鏀逛负甯?`{price}` 鍗犱綅鐨勬ā鏉匡紱鏂板 `featurePriceDefault` 浣滀负鏈厤缃椂鐨勫洖閫€鏂囨
- `frontend/src/views/auth/LoginView.vue` 鈥?鏂板 `paymentCnyPerUsd` ref锛宍onMounted` 浠庡叕寮€璁剧疆璇诲彇锛沠eature pill 鎸夐厤缃姩鎬佹覆鏌擄紝鏈厤缃洖閫€
- `frontend/src/api/admin/settings.ts` 鈥?鏂板 `systemSettingsToUpdateRequest(SystemSettings) => UpdateSettingsRequest` 鏄犲皠鍑芥暟锛涙敞鍏?`settingsAPI`
- `frontend/src/views/admin/RechargeConfigView.vue` 鈥?`save()` 鍏?`getSettings()` 鍐嶆暣浣?`updateSettings(...)`锛屽彧瑕嗙洊 `payment_cny_per_usd` / `payment_bonus_tiers`

**涓婃父鍏煎鎬?*:
- 鍚庣鏂板瀛楁涓哄彲閫夎拷鍔狅紝鍚堝苟涓婃父鏃惰嫢涓婃父涔熸敼鍔?`PublicSettings` / 鍏紑璁剧疆 handler锛岀暀鎰忓啿绐佷綅缃紙鍧囦负缁撴瀯浣撳熬閮ㄦ垨 return 瀛楁鍒楄〃锛?
- 鍓嶇鏂板鐨?`systemSettingsToUpdateRequest` 鏄湰鍦颁簩寮€宸ュ叿鍑芥暟锛岀嫭绔嬩簬涓婃父

**鍙樻洿璇︽儏**:
- Bug 1 鈥?鐧诲綍椤典环鏍肩‖缂栫爜锛歚LoginView` 鍘熷厛娓叉煋 `t('auth.login.featurePrice')` 鐨勯潤鎬佹枃妗?`'0.6 / 1$ 璧?`锛屼笌 admin 鍦?鍏呭€肩鐞?璁剧疆鐨?`payment_cny_per_usd` 瀹屽叏鑴遍挬銆傜幇灏嗚姹囩巼閫氳繃 `/api/v1/settings/public` 鏆撮湶锛堜笌 SSR 娉ㄥ叆璺緞淇濇寔涓€鑷达級锛屽墠绔鍙栧悗浠?`{price} / 1$ 璧穈 妯℃澘娓叉煋锛涗负 0 鎴栨湭閰嶇疆鏃跺洖閫€鍒?`featurePriceDefault` 闈欐€佹枃妗堛€?
- Bug 2 鈥?"姣忔閮ㄧ讲寮€鏀炬敞鍐岃閲嶇疆"锛氱湡姝ｆ牴鍥犱笉鏄儴缃茶剼鏈€傚悗绔?`UpdateSettingsRequest` 缁濆ぇ澶氭暟 `bool` / `string` 瀛楁鏄?*闈炴寚閽?*锛孞SON 鍙嶅簭鍒楀寲鏃剁己澶卞瓧娈典細琚～ `false` / `""`锛沗RechargeConfigView.save()` 鍙彂 `payment_cny_per_usd` 涓?`payment_bonus_tiers`锛宧andler 缁х画鏋勯€犲畬鏁?`SystemSettings` 骞?`SetMultiple` 鍥炲啓锛屽鑷?`registration_enabled`銆乣site_name`銆丱IDC/LinuxDo 寮€鍏崇瓑琚潤榛樻竻绌恒€備慨澶嶉噰鐢ㄦ渶灏忔敼鍔細`RechargeConfigView` 鍏堟媺瀹屾暣 settings锛岀敤鏂板缓鐨勬槧灏勫嚱鏁拌浆鎴愯姹備綋锛屽啀瑕嗙洊涓や釜 payment 瀛楁鍙戝嚭锛屼娇鍥炲啓鏄?璇绘棫鍊煎啓鏃у€?锛岄伩鍏嶈娓呯┖銆傚嚟鎹被瀛楁锛坄smtp_password` 绛夛級鍦ㄦ槧灏勫嚱鏁颁腑鏁呮剰鐣欑┖锛屽悗绔?绌哄€艰烦杩囪鐩?瀹堟姢缁х画鐢熸晥銆?

**楠岃瘉鏂瑰紡**:
- `go build ./...` 閫氳繃锛涘墠绔?`pnpm run typecheck` 閫氳繃锛沨andler 鐩稿叧鍗曟祴閫氳繃锛坰ervice 灞傚彈 `gemini_oauth_service_test.go` 棰勫瓨鍦ㄧ殑 mock 鎺ュ彛涓嶅畬鏁村奖鍝嶏紝鏈柊澧炴祴璇曞け璐ワ級
- 鎵嬪伐锛氬厖鍊肩鐞嗕繚瀛?`cny_per_usd=0.8` 鈫?鐧诲綍椤垫樉绀?`0.8 / 1$ 璧穈锛涘悓鏃剁郴缁熻缃噷"寮€鏀炬敞鍐?绛夊紑鍏充繚鎸佺敤鎴蜂箣鍓嶇殑鍊间笉鍙?


**褰卞搷鑼冨洿**:
- `backend/ent/schema/ai_credit_snapshot.go` 鈥?鏂?Ent schema锛歚AICreditSnapshot { email, credit_type, amount, captured_at }` + 澶嶅悎绱㈠紩
- `backend/ent/aicreditsnapshot/`銆乣backend/ent/aicreditsnapshot*.go` 鈥?Ent 鐢熸垚浠ｇ爜锛坄go generate ./ent`锛?
- `backend/migrations/110_add_ai_credit_snapshots.sql` 鈥?寤鸿〃 + `(email, captured_at)` 涓?`(captured_at)` 绱㈠紩
- `backend/internal/service/credit_snapshot.go` 鈥?`CreditSnapshot` 缁撴瀯銆乣CreditSnapshotRepository`銆乣AntigravityUsageAggregator`銆乣AntigravityUsageRatio` 鍝嶅簲绫诲瀷
- `backend/internal/service/credit_snapshot_service.go` 鈥?`CreditSnapshotService`锛?5 鍒嗛挓 ticker 瀹氭椂閲囨牱銆乣TriggerManualCapture`锛?0 绉掕繘绋嬪唴鍐峰嵈閿侊級銆乣GetAntigravityUsageRatio`锛堢浉閭婚噰鏍风偣姝ｅ悜 delta 姹傚拰 + `usage_logs` 鑱氬悎锛?
- `backend/internal/repository/credit_snapshot_repo.go` 鈥?鍩轰簬 Ent 鐨勪粨搴撳疄鐜帮紙Insert/ListInRange/GetLatestBefore锛?
- `backend/internal/repository/antigravity_usage_aggregator.go` 鈥?鐙珛灏忔帴鍙ｅ疄鐜帮細`SELECT COUNT + SUM(total_cost) FROM usage_logs WHERE account_id = ANY($1) AND created_at 鈭?[start,end)`
- `backend/internal/handler/admin/usage_handler.go` 鈥?`NewUsageHandler` 鍔?`creditSnapshotService` 渚濊禆锛涙柊澧?`StatsAntigravity` / `RefreshAntigravityStats`锛涙彁鍙?`parseStatsDateRange` 杈呭姪鍑芥暟
- `backend/internal/handler/admin/{usage_cleanup_handler_test,usage_handler_request_type_test}.go` 鈥?stub 琛ラ綈鏂板弬鏁颁綅 `nil`
- `backend/internal/server/routes/admin.go` 鈥?`GET /admin/usage/stats/antigravity`銆乣POST /admin/usage/stats/antigravity/refresh`
- `backend/internal/service/wire.go` 鈥?鏂板 `ProvideCreditSnapshotService` 骞跺叆 `ProviderSet`
- `backend/internal/repository/wire.go` 鈥?`NewCreditSnapshotRepository` / `NewAntigravityUsageAggregator` 鍔犲叆 `ProviderSet`
- `backend/cmd/server/wire_gen.go` 鈥?鎵嬪姩缂栨帓鏂?Repo + Service + Handler 渚濊禆锛堜富骞?`go generate` 鍥犲巻鍙?Payment 閲嶅缁戝畾澶辫触锛屾寜鐜版湁妯″紡鎻掑叆锛?
- `frontend/src/api/admin/usage.ts` 鈥?鏂板 `AntigravityUsageRatio` 绫诲瀷銆乣getAntigravityStats`銆乣refreshAntigravityStats`
- `frontend/src/components/admin/usage/AntigravityRatioCard.vue` 鈥?鏂扮粍浠讹細4 鍒楁寚鏍囧崱 + 銆岀珛鍗抽噰鏍枫€嶆寜閽?+ 閲囨牱涓嶈冻/鍐峰嵈鎻愮ず
- `frontend/src/views/admin/UsageView.vue` 鈥?寮曞叆鍗＄墖锛屼笌鐜版湁 `UsageStatsCards` 鍏辩敤 `DateRangePicker`锛屽悓涓€鍒锋柊閾捐矾瑙﹀彂
- `frontend/src/i18n/locales/{zh,en}.ts` 鈥?鏂板 `usage.antigravity.*` 鏂囨

**涓婃父鍏煎鎬?*: 浣庛€傛墍鏈夋柊澧炴枃浠?瀛楁鍧囦负 additive锛涗粎 `admin/usage_handler.go` 鏋勯€犲櫒鍔犲弬鏁帮紙涓婃父鑻ラ噸鏋?handler 鍒濆鍖栫鍚嶉渶鍚屾锛夛紱`wire_gen.go` 浠嶉渶鎵嬪伐鍚堝苟銆俙AntigravityUsageAggregator` 鍒绘剰娌℃帴鍏?`UsageLogRepository` 鎺ュ彛锛岄伩鍏嶆棩鍚庢敼鍔ㄥ崄鍑犲 stub銆?

**鍙樻洿璇︽儏**:
1. Antigravity AI Credits 浣欓涓嶅彲鍥炴函鏌ヨ锛堣繙绔?API 鍙粰褰撳墠鍊硷級锛屽洜姝ゆ柊澧?`ai_credit_snapshots` 琛ㄣ€俙CreditSnapshotService` 姣?15 鍒嗛挓鍚姩涓€娆￠噰鏍凤細鎸?`credentials.email` 鍘婚噸锛堝悓 Google 璐﹀彿鍏变韩 credits锛夛紝澶嶇敤 `AccountUsageService.GetUsage` 鐨?3 鍒嗛挓缂撳瓨灞傛媺浣欓锛岄伩鍏嶉澶?API 鍘嬪姏銆?
2. 鑱氬悎鍙ｅ緞锛氬姣忎釜 email 鍦?`[start - 30 min lookback, end]` 鍐呯殑蹇収鎸夋椂闂村崌搴忚蛋鐩搁偦瀵癸紝绱姞姝ｅ悜 delta銆傝礋鍚?delta锛堝厖鍊?閲嶇疆锛夎烦杩囥€傛淳鐢熸瘮鐜?`quota_per_credit = SUM(total_cost) / total_credits`銆乣calls_per_credit = COUNT(*) / total_credits`锛宍total_credits == 0` 鏃惰繑鍥?null锛堝墠绔睍绀?閲囨牱涓嶈冻"鎻愮ず锛夈€?
3. 鎵嬪姩瑙﹀彂鎺ュ彛 `POST .../refresh` 鍔?30 绉掕繘绋嬪唴鍐峰嵈閿侊紙`sync.Mutex + lastManualAt`锛夛紝鍐峰嵈鏈熷唴杩斿洖 `manual_refresh_throttled=true` 骞朵笉閲嶅鎵撹繙绔€傜鐞嗗憳璇偣涓嶄細鏀惧ぇ API 鍘嬪姏銆?
4. 鍓嶇鍗＄墖鎺ュ叆鐜版湁 `startDate`/`endDate`锛宍loadStats()` 缁撴潫鍚庡苟琛屾媺 antigravity 鑱氬悎锛涘け璐ュ彧 `console.error` 涓嶉樆鏂富娴佺▼銆?
5. 楠岃瘉锛歚docker exec sub2api-pg-dev psql` 纭 migration 110 搴旂敤銆乣ai_credit_snapshots` 琛ㄧ粨鏋勬纭紱鏈湴鍚姩鍚?`[CreditSnapshot] Scheduler started` 涓庤矾鐢?`GET/POST /api/v1/admin/usage/stats/antigravity(/refresh)` 鍧囧凡娉ㄥ唽銆?

**鍏宠仈 Issue/PR**: 鏃?

---

## [2026-04-18] fix(keys): 淇銆屽叆闂ㄦ寚鍗椼€嶉噷 CC-Switch 鐨勪笅杞藉湴鍧€

**褰卞搷鑼冨洿**:
- `frontend/src/components/keys/GettingStartedGuide.vue` 鈥?绗簩姝ヤ笅杞芥寜閽?`href` 浠?`github.com/nicepkg/cc-switch/releases`锛堥敊璇粨搴擄級鏀逛负 `github.com/farion1231/cc-switch/releases`锛堝畼鏂逛粨搴擄級

**涓婃父鍏煎鎬?*: 浣庛€備笂娓歌嫢鏈娇鐢ㄦ閾炬帴鍒欐棤鍐茬獊銆?

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹?

---

## [2026-04-18] refactor(page-content): 鍚堝苟銆岃浠烽〉鏂囨銆嶅拰銆岀櫥褰曢〉鏂囨銆嶄负缁熶竴 Tab 椤?

**褰卞搷鑼冨洿**:
- `frontend/src/views/admin/PageContentView.vue` 鈥?鏂板鍚堝苟鐖惰鍥撅細`AppLayout` + 鍏变韩澶撮儴 + 涓や釜 tab锛堟ā鍨嬭浠烽〉 / 鐧诲綍椤碉級 + `?tab=pricing|login` URL 鍚屾 + `<KeepAlive>` 淇濈暀琛ㄥ崟杈撳叆涓嶄涪澶?
- `frontend/src/components/admin/page-content/PricingContentForm.vue` 鈥?鐢?`PricingPageView.vue` 鍓ュ嚭 AppLayout/椤垫爣棰樺悗寰楀埌锛屼粎淇濈暀鎻愮ず鍗°€佷袱娈?textarea銆佷繚瀛樻寜閽?
- `frontend/src/components/admin/page-content/LoginContentForm.vue` 鈥?鐢?`LoginPageView.vue` 鍓ュ嚭 AppLayout/椤垫爣棰樺悗寰楀埌锛屼繚鐣欎笁缁?8 瀛楁 + 娓呯┖/淇濆瓨/棰勮
- `frontend/src/views/admin/PricingPageView.vue`銆乣frontend/src/views/admin/LoginPageView.vue` 鈥?鍒犻櫎
- `frontend/src/router/index.ts` 鈥?鏂?`/admin/page-content` 璺敱锛沗/admin/pricing-page`銆乣/admin/login-page` 淇濈暀涓?redirect 鍒版柊璺緞骞跺甫涓?`?tab=` 鍙傛暟锛岃€佷功绛句笉澶辨晥
- `frontend/src/components/layout/AppSidebar.vue` 鈥?绠＄悊鍛樹晶杈规爮鍘绘帀涓ゆ潯鏃ч」锛屽悎鎴愪竴鏉°€岄〉闈㈡枃妗堛€?
- `frontend/src/i18n/locales/{zh,en}.ts` 鈥?鍒?`nav.pricingPage` / `nav.loginPage`锛涙柊澧?`nav.pageContent` + `admin.pageContent.{title,description,tabs.{pricing,login}}`锛涗繚鐣?`admin.pricingPage.*` / `admin.loginPage.*`锛堜袱涓瓙缁勪欢浠嶇劧娑堣垂锛?

**涓婃父鍏煎鎬?*: 浣庛€傚彧鍔ㄥ墠绔紝鍚庣 handler 鍜岃缃?key 涓嶅彉銆?

**鍙樻洿璇︽儏**:
1. 鍚堝苟鍔ㄦ満锛氫袱鍧楅兘鏄€屽墠鍙伴〉闈㈡枃妗堢鐞嗐€嶏紝鎷嗕袱涓晶杈规爮鏉＄洰鍋忓啑浣欙紱鏈潵濡傛灉杩樿鍔犳柊椤甸潰锛堜緥濡備华琛ㄧ洏銆?04 椤碉級缁熶竴鏀捐繘杩欎釜 tab 椤靛嵆鍙€?
2. Tab 鍒囨崲閫氳繃 URL `?tab=...` 鍚屾锛屼究浜庢繁閾炬帴 + 娴忚鍣ㄥ墠杩?鍚庨€€锛涙湭鎸囧畾鏃堕粯璁?`pricing`銆?
3. `<KeepAlive>` 淇濈暀瀛愮粍浠剁姸鎬侊紝鐢ㄦ埛鍦ㄤ袱涓?tab 涔嬮棿鍒囨崲鏃舵湭淇濆瓨鐨勭紪杈戜笉浼氫涪銆?
4. 鑰佽矾寰勪繚鐣?redirect 鍒版柊璺緞锛屾棫涔︾骞虫粦杩囨浮銆?

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹傦紙绱ф帴涓ゆ鏂囨鍔熻兘鍚堝苟锛?

---

## [2026-04-18] feat(login-page): 绠＄悊鍛樺彲缂栬緫鐧诲綍椤垫枃妗?

**褰卞搷鑼冨洿**:
- `backend/internal/service/domain_constants.go` 鈥?鏂板 8 涓?`SettingKeyLoginPage*` 甯搁噺
- `backend/internal/service/settings_view.go` 鈥?`LoginPageContent` 缁撴瀯锛坖son tag + `IsEmpty`锛夛紱`PublicSettings.LoginPage *LoginPageContent`
- `backend/internal/service/setting_service.go` 鈥?`GetPublicSettings` 鍔?8 涓?key 鍒版壒閲忚鍙栧垪琛紱鏂板 `buildLoginPageContent`锛堢┖瀛楁 trim 鍚庢暣浣?nil 鍖栵級锛沗GetPublicSettingsForInjection` 鐨勫尶鍚?struct 涔熷姞 `login_page`
- `backend/internal/handler/dto/settings.go` 鈥?`PublicSettings` DTO 鍔?`LoginPage *LoginPageContent`锛涙柊澧?`dto.LoginPageContent`
- `backend/internal/handler/setting_handler.go` 鈥?鍏紑 `/settings/public` 杈撳嚭鏄犲皠 + `toDTOLoginPageContent` 杈呭姪鍑芥暟
- `backend/internal/handler/admin/login_page_handler.go` 鈥?鏂板锛欸ET/PUT `/admin/login-page/content`锛涘瓧娈电骇 trim + 闀垮害鏍￠獙锛坰hort 255 / long 500锛?
- `backend/internal/handler/handler.go` + `wire.go` + `backend/cmd/server/wire_gen.go` 鈥?`AdminHandlers.LoginPage` + provider锛屾墜鍔ㄦ彃鍏?wire_gen 涓?pricing-page 淇濇寔鍚屼竴妯″紡
- `backend/internal/server/routes/admin.go` 鈥?`registerLoginPageRoutes`
- `frontend/src/api/loginPage.ts` 鈥?鏂板 API client锛坄getAdminLoginPageContent` / `updateAdminLoginPageContent` / `resetAdminLoginPageContent`锛?
- `frontend/src/api/index.ts` 鈥?瀵煎嚭
- `frontend/src/types/index.ts` 鈥?`LoginPageContent` 鎺ュ彛锛沗PublicSettings.login_page?` 鍙€夊瓧娈?
- `frontend/src/views/auth/LoginView.vue` 鈥?8 澶?`t('auth.login.xxx')` 鏇挎崲涓?`loginXxx` computed锛涙瘡涓?computed 閮界敤 `pickLoginText` 鍋?fallback锛堢┖涓?鏈畾涔夋椂鐢?i18n 鍘熸枃锛?
- `frontend/src/views/admin/LoginPageView.vue` 鈥?鏂板绠＄悊鍛樼紪杈戦〉锛? 涓皬鍒嗙粍锛堣惀閿€/妯″瀷鍖?鐧诲綍妗嗭級8 涓瓧娈佃〃鍗?+ 棰勮閾炬帴 + 淇濆瓨 + 鎭㈠榛樿锛堝甫 confirm锛夛紱淇濆瓨/鎭㈠鍚庤Е鍙?`appStore.fetchPublicSettings(true)` 绔嬪埢璁╁叾浠栨湭鍒锋柊鐨勯〉闈㈢湅鍒版柊鍊?
- `frontend/src/components/layout/AppSidebar.vue` 鈥?`adminNavItems` 澧炲姞銆岀櫥褰曢〉鏂囨銆嶅叆鍙?
- `frontend/src/router/index.ts` 鈥?`/admin/login-page` 璺敱
- `frontend/src/i18n/locales/{zh,en}.ts` 鈥?`nav.loginPage` + `admin.loginPage.*`锛坱itle/description/preview/fallbackHint/sections/fields 8 椤?save/reset/reset-confirm锛?

**涓婃父鍏煎鎬?*: 涓€俙PublicSettings` 缁撴瀯琚墿灞曪紙service + DTO + TS 绫诲瀷锛夛紝涓婃父鑻ュ皢鏉ユ敼鍔ㄨ繖涓粨鏋勯渶瑕佸悓姝ワ紱鏂板 key 鍛藉悕鐢?`login_page.*` 鍛藉悕绌洪棿锛屼笉涓庢棦鏈?key 鍐茬獊銆傝矾鐢?/ handler / 鍓嶇鏂囦欢閮芥槸鏂板锛屼笉瑕嗙洊涓婃父銆俙wire_gen.go` 浠嶉渶鎵嬪姩鍚堝苟銆?

**鍙樻洿璇︽儏**:
1. 8 涓?settings key锛坄login_page.badge` / `heading_line1` / `heading_line2` / `description` / `supported_models_title` / `models_desc` / `form_title` / `form_subtitle`锛変竴涓€瀵瑰簲 i18n `auth.login.*` 閲岀殑钀ラ攢鏂囨瀛楁銆?
2. 浠绘剰瀛楁绌哄瓧绗︿覆 鈫?鍚庣杩斿洖鐨?`LoginPage` 瀛愮粨鏋勪负 nil锛坄omitempty` 鏁翠綋 omit锛夛紝鍓嶇鎷夸笉鍒板氨缁х画鐢?`t('auth.login.xxx')`锛屼腑鑻卞垏鎹㈣嚜鍔ㄧ敓鏁堛€?
3. 绠＄悊鍛樹繚瀛樺悗璋冪敤 `appStore.fetchPublicSettings(true)` 寮哄埗閲嶆柊鎷夊彇 public settings锛岄伩鍏嶅叾浠栧凡鎵撳紑鐨勯〉闈㈢湅鍒版棫鐗堛€?
4. 銆屾仮澶嶉粯璁ゃ€? 鎵归噺鍐欏叆绌轰覆锛屼笉鏄墿鐞嗗垹 key锛涜涔夋洿鏄庣‘锛屼笖涓嶇敤鍔犲垹闄ゆ帴鍙ｃ€?
5. SSR 娉ㄥ叆鐨?`window.__APP_CONFIG__` 涔熷悓姝ユ洿鏂帮紙`GetPublicSettingsForInjection`锛夛紝棣栨娓叉煋鐧诲綍椤靛氨鏄渶缁堟枃妗堬紝涓嶉棯灞忋€?
6. 楠岃瘉锛歚curl /api/v1/settings/public | grep login_page` 鈫?鏈繚瀛樻椂鏃?key锛涚櫥褰曞悗 `curl /admin/login-page/content` 杩斿洖 8 瀛楁鍏ㄧ┖瀵硅薄锛涗繚瀛樺悗 public 鎺ュ彛寮€濮嬭繑鍥?`login_page` 瀛愮粨鏋勩€?

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹傦紙缁€屾ā鍨嬭浠烽〉鏂囨銆嶏級

---

## [2026-04-18] fix(pricing-page): 绠＄悊鍛樼紪杈戦〉鏈繚瀛樻椂棰勫～榛樿鏂囨

**褰卞搷鑼冨洿**:
- `backend/internal/handler/admin/pricing_page_handler.go` 鈥?瀵煎嚭 `DefaultPricingPageIntro` / `DefaultPricingPageEducation` 甯搁噺锛沗Get` 鍦?settings 鏈啓 / 绌轰覆鏃跺洖钀藉埌榛樿鍊硷紱`loadValue` 澶氫竴涓?fallback 鍏ュ弬
- `backend/internal/handler/pricing_page_handler.go` 鈥?鍒犳帀鏈湴榛樿甯搁噺锛屽鐢?`admin.Default*`

**涓婃父鍏煎鎬?*: 浣庛€傜函瀛楁绾ц皟鏁达紝鏃?schema / 璺敱鍙樺寲銆?

**鍙樻洿璇︽儏**: 鍘熷厛绠＄悊鍛樿繘缂栬緫椤垫椂 settings 閲岃繕娌″啓鍏ワ紝涓や釜 textarea 閮芥槸绌虹殑锛屼絾鐢ㄦ埛璁′环椤靛張鏄剧ず鐨勬槸 handler 鍐呯疆榛樿鏂囨锛屽鑷淬€岀紪杈戜笉鍒扮敤鎴风湅鍒扮殑涓滆タ銆嶃€傜幇鍦?admin Get 鎺ュ彛涓庣敤鎴蜂晶鍏辩敤鍚屼竴浠藉父閲忥紝绠＄悊鍛樼涓€娆¤繘鏉ュ氨鑳界湅鍒般€岀敤鎴锋鍒诲疄闄呭湪鐪嬬殑鍐呭銆嶏紝鐩存帴鏀瑰氨琛屻€?

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹傦紙涓婃潯鍙樻洿鐨勫悗缁級

---

## [2026-04-18] feat(pricing-page): 鏂板鐢ㄦ埛銆屾ā鍨嬭浠枫€嶉〉 + 绠＄悊鍛樺彲缂栬緫鏂囨

**褰卞搷鑼冨洿**:
- `backend/migrations/109_add_show_on_pricing_page.sql` 鈥?`global_model_pricing` 鏂板 `show_on_pricing_page BOOLEAN`
- `backend/internal/service/global_model_pricing.go` 鈥?`GlobalModelPricing` 鍔?`ShowOnPricingPage` 瀛楁锛涙帴鍙ｆ柊澧?`ListForPricingPage`
- `backend/internal/repository/global_model_pricing_repo.go` 鈥?鎵€鏈?SELECT/INSERT/UPDATE 鍚屾鏂板瓧娈碉紱鏂板 `ListForPricingPage`
- `backend/internal/service/global_model_pricing_service.go` 鈥?`GlobalOverride` DTO 鍔?`show_on_pricing_page`锛沗ToGlobalOverride` 鍚屾锛涙柊澧?`ListForPricingPage` 鏂规硶
- `backend/internal/handler/admin/model_pricing_handler.go` 鈥?Create/Update 璇锋眰 DTO 鍔?`show_on_pricing_page *bool`
- `backend/internal/handler/admin/pricing_page_handler.go` 鈥?鏂板锛欸ET/PUT `/admin/pricing-page/content`锛岃鍐?`settings` KV 涓や釜 key
- `backend/internal/handler/pricing_page_handler.go` 鈥?鏂板鐢ㄦ埛渚э細GET `/user/pricing-page`锛岃仛鍚堜袱娈垫枃妗?+ 鎸?provider 鍒嗙粍鐨勫睍绀轰环鏍?
- `backend/internal/handler/handler.go` 鈥?`AdminHandlers.PricingPage`銆乣Handlers.PricingPage` 鏂板瓧娈?
- `backend/internal/handler/wire.go` 鈥?娉ㄥ唽 `NewPricingPageHandler` / `NewPricingPageAdminHandler`
- `backend/cmd/server/wire_gen.go` 鈥?鎵嬪姩缂栨帓鏂?handler 渚濊禆锛坄go generate` 鍦ㄤ富骞插凡棰勫厛澶辫触锛屾寜鐜版湁妯″紡鎻掑叆锛?
- `backend/internal/server/routes/admin.go` 鈥?`registerPricingPageRoutes`
- `backend/internal/server/routes/user.go` 鈥?娉ㄥ唽 `/user/pricing-page`
- `frontend/src/api/pricingPage.ts` 鈥?鏂板 API client锛堢敤鎴?Get + 绠＄悊鍛?Get/Update锛?
- `frontend/src/api/index.ts` 鈥?瀵煎嚭 `pricingPageAPI`
- `frontend/src/api/admin/modelPricing.ts` 鈥?`GlobalOverride`/`CreateOverrideRequest`/`UpdateOverrideRequest` 鍔?`show_on_pricing_page`
- `frontend/src/views/user/PricingView.vue` 鈥?鏂板鐢ㄦ埛椤碉細涓夎妭鍐呭锛堟湰绔欒浠锋ā寮?/ 璁′环妯″紡绉戞櫘 / 鎸夊钩鍙板垎缁勭殑浠锋牸琛級锛孧arkdown 鐢?`marked@17` + `DOMPurify` 娓叉煋
- `frontend/src/views/admin/PricingPageView.vue` 鈥?鏂板绠＄悊鍛橀〉锛氫袱娈?textarea 缂栬緫 + 淇濆瓨 + 鎸囧悜妯″瀷閰嶇疆鐨勫紩瀵?
- `frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue` 鈥?缂栬緫瀵硅瘽妗嗗姞銆屽湪璁′环椤靛睍绀恒€嶅紑鍏?
- `frontend/src/components/layout/AppSidebar.vue` 鈥?鐢ㄦ埛/涓汉渚ц竟鏍忔柊澧炪€屾ā鍨嬭浠枫€嶈彍鍗曪紱绠＄悊鍛樹晶杈规爮鏂板銆岃浠烽〉鏂囨銆嶅叆鍙ｏ紱鏂板 `PriceTagIcon`
- `frontend/src/router/index.ts` 鈥?鏂板 `/pricing` 涓?`/admin/pricing-page` 璺敱
- `frontend/src/i18n/locales/{zh,en}.ts` 鈥?鏂板 `pricing.*`銆乣admin.pricingPage.*`銆乣admin.modelPricing.showOnPricingPage` 閿互鍙?`nav.modelPricing`銆乣nav.pricingPage`

**涓婃父鍏煎鎬?*: 涓€傛柊澧炲瓧娈?`show_on_pricing_page` 浣嶄簬 `global_model_pricing` 琛紝杩佺Щ鏄?additive锛屼笂娓歌嫢灏嗘潵瀵硅琛ㄧ粨鏋勫仛鏀瑰姩闇€鎵嬪姩鍚堝苟銆侶andler / 璺敱鍧囦负鏂板锛屼笉瑕嗙洊涓婃父鏂囦欢鐨勬棦鏈夎矾寰勩€俙wire_gen.go` 鎵嬪姩缂栬緫锛堝洜涓诲共 Wire 鐢熸垚棰勫厛澶辫触锛宍ProvidePaymentConfigService` 绛夐噸澶嶇粦瀹氾級锛屽悎骞朵笂娓告椂闇€鐣欐剰銆?

**鍙樻洿璇︽儏**:
1. 绠＄悊鍛樺彲鍦ㄣ€屾ā鍨嬮厤缃?鈫?妯″瀷璇︽儏銆嶉噷鍕鹃€夈€屽湪璁′环椤靛睍绀恒€嶏紝鎺у埗鍝簺妯″瀷鍑虹幇鍦ㄧ敤鎴蜂晶鐨勮浠烽〉锛岀嫭绔嬩簬璁¤垂 `enabled` 寮€鍏炽€?
2. 绠＄悊鍛樺彲鍦?`/admin/pricing-page` 缂栬緫涓ゆ Markdown 鏂囨锛堟湰绔欒浠锋ā寮忋€佽浠锋ā寮忕鏅級锛屼繚瀛樺埌 `settings` 琛ㄧ殑 `pricing_page.intro_markdown` / `pricing_page.education_markdown` 涓や釜 key銆傛湭淇濆瓨鏃剁敤鎴蜂晶鍥炶惤鍒?handler 鍐呯疆榛樿鏂囨銆?
3. 鐢ㄦ埛 `/pricing` 椤典竴娆℃媺鍙栬仛鍚堟帴鍙ｏ細杩斿洖涓ゆ鏂囨 + 鎸?provider 鍒嗙粍鐨勫睍绀轰环鏍艰〃銆傚睍绀轰环鐨勪紭鍏堢骇锛氱敤鎴风骇 display override > 鍏ㄥ眬 display override > 鐪熷疄鍗曚环锛坒allback锛夈€?
4. 浠锋牸琛?per-token 浠锋寜 $/MTok 鏄剧ず锛宲er_request 鎸?$/娆?鏄剧ず銆?
5. i18n 宸茶ˉ zh/en 瀹屾暣閿€笺€?

**鍏宠仈 Issue/PR**: 鏈湴浜屽紑闇€姹?

---

## [2026-04-17] feat(billing): 鐢ㄦ埛绾фā鍨嬪畾浠疯鐩?(User Model Pricing Override)

**褰卞搷鑼冨洿**:
- `backend/migrations/106_add_user_model_pricing_overrides.sql` 鈥?鏂板琛?
- `backend/internal/service/user_model_pricing.go` 鈥?瀹炰綋 + 浠撳偍鎺ュ彛
- `backend/internal/service/user_model_pricing_service.go` 鈥?涓氬姟閫昏緫灞?
- `backend/internal/repository/user_model_pricing_repo.go` 鈥?鍘熺敓 SQL 瀹炵幇
- `backend/internal/service/model_pricing_resolver.go` 鈥?PricingInput 澧炲姞 UserID, Resolve 澧炲姞鐢ㄦ埛绾ц鐩栧彔鍔?
- `backend/internal/service/gateway_service.go` 鈥?浼犻€?UserID 鍒板畾浠疯В鏋愰摼璺?
- `backend/internal/handler/dto/display_pricing.go` 鈥?鏂板 BuildUserDisplayPricingMap
- `backend/internal/handler/usage_handler.go` 鈥?浣跨敤鐢ㄦ埛绾у睍绀鸿鐩?
- `backend/internal/handler/admin/user_model_pricing_handler.go` 鈥?Admin CRUD API
- `backend/internal/service/global_model_pricing_service.go` 鈥?鍒楄〃澧炲姞 user_override_count, 璇︽儏澧炲姞 user_overrides
- `backend/internal/service/admin_service.go` 鈥?鐢ㄦ埛鍒犻櫎鏃剁骇鑱旀竻鐞?
- `backend/internal/handler/handler.go` 鈥?AdminHandlers 澧炲姞 UserModelPricing 瀛楁
- `backend/internal/handler/wire.go` 鈥?娉ㄥ唽鏂?handler
- `backend/internal/repository/wire.go` 鈥?娉ㄥ唽鏂?repo
- `backend/internal/service/wire.go` 鈥?娉ㄥ唽鏂?service
- `backend/internal/server/routes/admin.go` 鈥?娉ㄥ唽鏂拌矾鐢?
- `frontend/src/api/admin/userModelPricing.ts` 鈥?鍓嶇 API 瀹㈡埛绔?
- `frontend/src/components/admin/user/UserModelPricingModal.vue` 鈥?绠＄悊妯℃€佹
- `frontend/src/views/admin/UsersView.vue` 鈥?鐢ㄦ埛鎿嶄綔鑿滃崟澧炲姞"妯″瀷瀹氫环"鍏ュ彛
- `frontend/src/i18n/locales/en.ts` 鈥?鍥介檯鍖栨枃妗?

**璇存槑**: 鏂板鐢ㄦ埛绾фā鍨嬪畾浠疯鐩栧姛鑳斤紝鏀寔绠＄悊鍛樹负鐗瑰畾鐢ㄦ埛鐨勭壒瀹氭ā鍨嬭缃細
1. 鐪熷疄璁¤垂浠锋牸瑕嗙洊锛坕nput_price, output_price, cache_write_price, cache_read_price锛?
2. 灞曠ず浠锋牸瑕嗙洊锛坉isplay_input_price, display_output_price, display_rate_multiplier, cache_transfer_ratio锛?

瀹屾暣瀹氫环浼樺厛绾ч摼锛氱敤鎴?> 娓犻亾 > 鍏ㄥ眬 > LiteLLM/Fallback銆備笉褰卞搷鐜版湁鐨勫叏灞€瑕嗙洊銆佹笭閬撹鐩栥€佸垎缁勫€嶇巼鍜岀敤鎴峰垎缁勫€嶇巼鏈哄埗銆?

## [2026-04-17] feat(billing): 鐢ㄦ埛绾у睍绀哄€嶇巼 (User Display Rate Multiplier)

**褰卞搷鑼冨洿**:
- `backend/migrations/104_add_display_rate_multiplier.sql` 鈥?鏂板
- `backend/internal/service/user_group_rate.go` 鈥?鎵╁睍 UserGroupRateEntry, GroupRateMultiplierInput, 鏂板 UserGroupRateData
- `backend/internal/repository/user_group_rate_repo.go` 鈥?鏀寔 display_rate_multiplier 璇诲啓
- `backend/internal/handler/dto/display_pricing.go` 鈥?鏂板 ApplyUserDisplayRate()
- `backend/internal/handler/usage_handler.go` 鈥?浣跨敤璁板綍搴旂敤鐢ㄦ埛绾у睍绀哄彉鎹?
- `backend/internal/handler/api_key_handler.go` 鈥?/groups/rates 杩斿洖灞曠ず鍊嶇巼
- `backend/internal/service/api_key_service.go` 鈥?鏂板 GetUserGroupRatesFull()
- `backend/internal/service/admin_service.go` 鈥?UpdateUser 鏀寔 GroupRatesFull
- `backend/internal/handler/admin/user_handler.go` 鈥?鏀寔 group_rates_full
- `frontend/src/types/index.ts` 鈥?鏂板 UserGroupRateData, group_display_rates
- `frontend/src/api/groups.ts` 鈥?杩斿洖 UserGroupRateData
- `frontend/src/views/user/KeysView.vue` 鈥?GroupBadge 灞曠ず灞曠ず鍊嶇巼
- `frontend/src/components/admin/user/UserAllowedGroupsModal.vue` 鈥?灞曠ず鍊嶇巼缂栬緫UI
- `frontend/src/i18n/locales/{en,zh}.ts` 鈥?鍥介檯鍖?

**涓婃父鍏煎鎬?*: 浣庡啿绐侀闄╋紝鏂板瀛楁鍜屾柟娉曪紝涓嶄慨鏀圭幇鏈夐€昏緫

**鍙樻洿璇︽儏**:
- 绠＄悊鍛樺彲涓烘瘡涓敤鎴峰湪姣忎釜鍒嗙粍璁剧疆鐙珛鐨?灞曠ず鍊嶇巼"锛岀敤鎴风湅鍒板睍绀哄€嶇巼鑰岄潪鐪熷疄璁¤垂鍊嶇巼
- 灞曠ず鍊嶇巼鐙珛浜庣湡瀹炰笓灞炲€嶇巼锛屽嵆浣跨敤鎴蜂娇鐢ㄥ垎缁勯粯璁ゅ€嶇巼涔熷彲鍗曠嫭璁惧睍绀哄€嶇巼
- 浣跨敤璁板綍閫氳繃缂╂斁 token 鏁伴噺瀹炵幇鑷唇锛歛ctual_cost 涓嶅彉锛宼otal_cost 脳 display_rate 鈮?actual_cost
- 涓庢ā鍨嬬骇灞曠ず浠锋牸閾惧紡鍙犲姞锛岀敤鎴风骇浼樺厛绾ф洿楂?

## [2026-04-16] fix(pricing): 淇缂栬緫鐢ㄦ埛灞曠ず璁剧疆鍚庢ā鍨嬩环鏍兼帴鍙?00閿欒

**褰卞搷鑼冨洿**:
- `backend/internal/repository/global_model_pricing_repo.go`

**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紝淇鑷繁寮曞叆鐨刡ug

**鍙樻洿璇︽儏**:
- `GetByID` 鍜?`GetByModel` 鏂规硶 SELECT 浜?18 鍒椾絾 Scan 鍙帴鏀?14 涓瓧娈?
- 婕忔帀浜?`display_input_price`, `display_output_price`, `display_rate_multiplier`, `cache_transfer_ratio` 鍥涗釜瀛楁
- 褰?display 瀛楁涓?NULL 鏃跺伓灏斾笉鎶ラ敊锛岃缃簡闈?NULL 鍊煎悗蹇呯幇 500

## [2026-04-16] feat(deploy): 瀹夊叏閮ㄧ讲鑴氭湰锛屾敮鎸佽嚜鍔ㄥ洖婊?

**褰卞搷鑼冨洿**:
- `deploy/update.sh`锛堟柊澧烇級

**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紝鏂板鐙珛鏂囦欢

**鍙樻洿璇︽儏**:
- 鏋勫缓鍒颁复鏃?staging tag锛屾棫闀滃儚鍦ㄦ瀯寤烘湡闂翠繚鎸佷笉鍙?
- 淇濈暀涓婁竴涓増鏈暅鍍?(`sub2api-custom:prev`) 鐢ㄤ簬鍗虫椂鍥炴粴
- 閮ㄧ讲鍚?health check 澶辫触鑷姩鍥炴粴鍒板墠涓€鐗堟湰
- 鏀寔 `--rollback` 鎵嬪姩鍥炴粴
- 鍏ㄨ繃绋嬫棩蹇楄褰曞埌 `/opt/sub2api/deploy.log`

## [2026-04-16] feat(branding): 鏂板寮鸿皟瀹夊叏涓庣ǔ瀹氭皵璐ㄧ殑涓ょ増绮楃姺鍥炬爣

**褰卞搷鑼冨洿**:
- `frontend/public/logo-gateway-fortress.svg`
- `frontend/public/logo-gateway-vault.svg`

**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紝浠呮柊澧為潤鎬佸搧鐗岃祫婧?

**鍙樻洿璇︽儏**:
- 鏂板 `logo-gateway-fortress.svg`锛屾柟鍚戝亸鈥滄姢鐩?+ 鍩虹璁炬柦鍫″瀿鈥濓紝鐢ㄥ帤閲嶅绉扮粨鏋勫己鍖栧畨鍏ㄣ€佺ǔ鍥恒€佸彲淇¤禆鐨勭涓€鍗拌薄
- 鏂板 `logo-gateway-vault.svg`锛屾柟鍚戝亸鈥滈噾搴撻棬 + 绋冲畾涓灑鈥濓紝閫氳繃鏇寸矖鐨勯棬妗嗗拰閿佽姱璇箟绐佸嚭鍙潬鎵樼涓庤祫浜у畨鍏ㄦ劅
- 涓ょ増閮芥瘮鍓嶉潰鐨勬柟妗堟洿澶ц儐銆佹洿鍘氶噸锛屼紭鍏堟湇鍔♀€滃畨鍏ㄣ€佺ǔ瀹氥€侀潬璋扁€濈殑鍝佺墝蹇冩櫤

## [2026-04-16] feat(branding): 鏂板涓ょ増鍘熷垱鍥炬爣澶囬€夋柟妗?

**褰卞搷鑼冨洿**:
- `frontend/public/logo-gateway-orbit.svg`
- `frontend/public/logo-gateway-portal.svg`

**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紝浠呮柊澧為潤鎬佸搧鐗岃祫婧?

**鍙樻洿璇︽儏**:
- 鏂板 `logo-gateway-orbit.svg`锛屾柟鍚戝亸鈥滅綉缁滀腑鏋?/ 鎺у埗闈?/ 璋冨害鑺傜偣鈥濓紝鏍稿績鏄幆褰㈡眹鑱氫笌涓夎矾鎺ュ叆
- 鏂板 `logo-gateway-portal.svg`锛屾柟鍚戝亸鈥滃叆鍙?/ 閫氶亾 / 缃戝叧闂ㄦ埛鈥濓紝鏍稿績鏄垎灞傞棬妗嗕笌鍚戝績鑱氬悎
- 涓ょ増閮藉埢鎰忛伩寮€涓婃父 `sub2api` 甯歌鐨勫瓧姣嶅寲鍑犱綍閫犲瀷锛屼紭鍏堝缓绔嬩綘鑷繁鐨勫搧鐗岃瘑鍒?

## [2026-04-16] feat(branding): 鍥炬爣閲嶆瀯涓哄師鍒涚綉鍏充腑鏋㈤€犲瀷锛岄伩寮€涓婃父瑙嗚鍏宠仈

**褰卞搷鑼冨洿**:
- `frontend/public/logo-gateway-mark.svg`

**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紝浠呮洿鏂拌嚜瀹氫箟鍝佺墝璧勬簮

**鍙樻洿璇︽儏**:
- 灏嗕笂涓€鐗堝亸鍑犱綍瀛楁瘝鐨勫浘鏍囬噸鏋勪负鈥滃叚杈瑰舰缃戝叧鏍稿績 + 涓夎矾姹囪仛鑺傜偣鈥濈殑鍘熷垱绗﹀彿锛岄伩鍏嶈浜鸿仈鎯冲埌涓婃父 `sub2api` 榛樿瑙嗚
- 淇濈暀褰撳墠绔欑偣鑷繁鐨勬繁钃濆簳鍜岄潚缁夸富鑹诧紝浠ヤ繚璇佸拰鐜版湁棣栭〉銆佸悗鍙版寜閽€佸崱鐗囬珮浜粛鐒剁粺涓€
- 鏂板浘鏍囨洿寮鸿皟鈥滆仛鍚堛€佽皟搴︺€佸垎鍙戔€濈殑浜у搧璇箟锛岃€屼笉鏄瓧姣嶉€犲瀷锛屼究浜庡悗缁嫭绔嬪搧鐗屽寲

## [2026-04-16] feat(branding): 鏂板璐村悎 AI 缃戝叧璇箟鐨?SVG 鍥炬爣鏂规

**褰卞搷鑼冨洿**:
- `frontend/public/logo-gateway-mark.svg`

**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紝浠呮柊澧為潤鎬佸搧鐗岃祫婧愶紝涓嶆浛鎹笂娓搁粯璁ゆ枃浠?

**鍙樻洿璇︽儏**:
- 鏂板涓€鐗堢敤浜?Sub2API 鐨勫搧鐗屽浘鏍囨柟妗堬紝寤剁画鐜版湁娣辫摑搴曚笌闈掔豢鍒拌摑鑹叉笎鍙樼殑瑙嗚璇█锛岄伩鍏嶄笌棣栭〉鍜屽悗鍙扮殑涓昏壊浣撶郴鍓茶
- 鍥炬爣璇箟浠庡崟绾嚑浣曞瓧姣嶈繘涓€姝ユ敹鏁涘埌鈥滅綉鍏?/ 璺敱 / 鑱氬悎鍒嗗彂鈥濓紝閫氳繃涓灑寮忓嚑浣曚富褰㈠拰鑺傜偣绔偣寮哄寲 API Gateway 浜у搧璇嗗埆搴?
- 璧勬簮浣跨敤 SVG 鐭㈤噺鏍煎紡锛屼究浜庡悗缁湪鍚庡彴 `site_logo`銆佺珯鐐归椤点€乫avicon 瀵煎嚭鍜岃惀閿€鐗╂枡涓鐢?

## [2026-04-16] fix: AI Credits 琚复鏃堕檺娴佽鏍囦负绉垎鑰楀敖瀵艰嚧璐﹀彿閿佸畾 5 灏忔椂

**褰卞搷鑼冨洿**:
- `backend/internal/service/antigravity_credits_overages.go`
- `backend/internal/service/antigravity_credits_overages_test.go`

**涓婃父鍏煎鎬?*: 鏃犲啿绐侊紙浜屽紑鏂板鍔熻兘锛?

**鍙樻洿璇︽儏**:
- `shouldMarkCreditsExhausted` 涓?`"resource has been exhausted"` 鍏抽敭璇嶅尮閰嶄簡 Google API 鎵€鏈?429 鍝嶅簲锛堝寘鎷复鏃?RPM 闄愭祦锛夛紝瀵艰嚧 credits 琚敊璇爣璁颁负鑰楀敖銆備竴鏃﹁鏍囧舰鎴愯嚜閿侊紙`isCreditsExhausted` 闃绘閲嶈瘯 鈫?`clearCreditsExhausted` 姘镐笉瑙﹀彂锛夛紝璐﹀彿琚攣瀹氬畬鏁?5 灏忔椂銆?
- 绉婚櫎杩囦簬瀹芥硾鐨?`"resource has been exhausted"` 鍏抽敭璇嶏紝鍏朵綑鍏抽敭璇嶏紙`insufficient credit`銆乣credit exhausted` 绛夛級宸茶冻澶熺簿纭?
- `shouldMarkCreditsExhausted` 鎺掗櫎 429 鐘舵€佺爜锛屼复鏃堕檺娴佷笉搴斿垽瀹氫负绉垎鑰楀敖

---

## [2026-04-16] feat(admin): 妯″瀷瀹氫环椤靛悎骞舵槧灏?CRUD + 妯″瀷娴嬭瘯锛屽垹闄ゆ棫 mapping tab

**褰卞搷鑼冨洿**:
- `frontend/src/views/admin/ModelConfigView.vue`锛?*澶у箙绮剧畝**锛氬垹闄?mapping tab 鍏ㄩ儴妯℃澘鍜?script锛屽彧淇濈暀 pricing 鍜?rate 涓や釜 tab锛?
- `frontend/src/components/admin/model-pricing/ModelMappingInlinePopover.vue`锛?*鏂板缓**锛?
- `frontend/src/components/admin/model-pricing/ModelTestDialog.vue`锛?*鏂板缓**锛?
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`锛堣〃鏍奸《閮ㄥ姞"+ 娣诲姞鏄犲皠"鎸夐挳锛涜鎿嶄綔鍒楀姞"缂栬緫鏄犲皠"鍜?娴嬭瘯"涓や釜鏉′欢鏄剧ず鎸夐挳锛涙帴鍏ヤ袱涓柊缁勪欢锛?
- `frontend/src/i18n/locales/zh.ts` & `en.ts`锛堟柊澧?~20 鏉?key锛氭槧灏?CRUD + 妯″瀷娴嬭瘯锛?

**涓婃父鍏煎鎬?*: 浣庨闄┿€傚叏閮ㄩ泦涓湪浜屽紑鐙湁鐨勬ā鍨嬮厤缃晫闈€侫PI 澶嶇敤鐜版湁鐨?`adminAPI.accounts.getAntigravityDefaultModelMapping` / `updateAntigravityDefaultModelMapping`锛堜笂娓稿凡鏈夛級锛屼互鍙?SSE 娴嬭瘯鎺ュ彛 `POST /admin/accounts/:id/test`銆?

**鑳屾櫙**:

涓婁竴杞妸妯″瀷瀹氫环椤甸噸鏋勪负"鍙屽垪妯″瀷鍚?+ 璁¤垂妯″紡"椋庢牸鍚庯紝鐢ㄦ埛鍙嶉锛?鏄犲皠鍏崇郴鍜岃璐规ā寮忎笉鑳戒慨鏀?銆傜粡璁ㄨ锛?
- 璁¤垂妯″紡淇濈暀鍙锛堟湰韬槸浠庢槧灏勫叧绯绘帹鏂殑鏍囩锛屼笉鏄彲閰嶇疆灞炴€э級
- 鏄犲皠鍏崇郴**搴旇**鑳芥敼锛屼笖鍐冲畾鎶娿€屾ā鍨嬫槧灏勩€嶇嫭绔?tab 鍚堝苟鍒板畾浠烽〉锛堝悗缁笎杩涘垹闄ょ嫭绔?tab锛?
- 妯″瀷娴嬭瘯鍔熻兘鎼埌瀹氫环椤佃鎿嶄綔閲屽仛鎴愬皬鎸夐挳

鏂瑰悜纭畾鍚庢湰杞疄鏂藉交搴曠殑鍚堝苟銆?

**鍙樻洿璇︽儏**:

1. **鏂板缓 `ModelMappingInlinePopover.vue`**锛垀210 琛岋級锛?
   - 涓夌鎿嶄綔锛氭柊澧炴槧灏勶紙mode="add"锛? 淇敼鏄犲皠锛坢ode="edit"锛? 鍒犻櫎鏄犲皠锛坋dit 妯″紡搴曢儴鎸夐挳锛?
   - 涓や釜 input锛氳姹傛ā鍨嬪悕 + 涓婃父妯″瀷鍚嶏紝涓嬫柟甯︿竴琛岀伆瀛楁彁绀?鍚屽悕鏄犲皠鐩存帴濉浉鍚屽€?
   - 璧扮幇鏈?API锛歚GET /admin/accounts/antigravity/default-model-mapping` 璇诲叏琛?鈫?灞€閮ㄤ慨鏀?鈫?`PUT` 鏁磋〃鍐欏洖
   - 鏀瑰悕鍦烘櫙锛坋dit 鏃舵妸 from 涔熸敼浜嗭級姝ｇ‘澶勭悊锛氬厛 delete 鏃?key 鍐?set 鏂?key/value
   - Teleport + fixed 瀹氫綅锛堝弬鑰?ModelPricingInlinePopover 璁捐锛夛紝鑷姩閬垮紑瑙嗗彛杈圭晫
   - Enter 淇濆瓨銆佺孩瀛?inline 閿欒鍙嶉

2. **鏂板缓 `ModelTestDialog.vue`**锛垀160 琛岋級锛?
   - 浠庡師 `ModelConfigView.vue` 鐨?mapping tab 鍙充晶娴嬭瘯闈㈡澘鎼縼锛岄€昏緫鍩烘湰淇濈暀
   - 鍥哄畾浼犲叆 `model` prop锛堜粠琛屾寜閽Е鍙戞椂閿佸畾锛夛紝涓嶅啀闇€瑕佹ā鍨嬩笅鎷?
   - 鍐呴儴鍔犺浇 Antigravity 璐﹀彿鍒楄〃锛堜粎 active / schedulable / 鏃?error 鐨勶級
   - SSE 娴佸紡娑堣垂 `/api/v1/admin/accounts/:id/test`锛岃В鏋?`test_start / content / test_complete / error` 浜嬩欢绫诲瀷
   - `testRunning` 鏃堕樆姝㈠叧闂?dialog 閬垮厤鐢ㄦ埛璇搷浣?

3. **`ModelPricingTab.vue` 鎺ュ叆**锛?
   - 琛ㄦ牸椤堕儴锛堟悳绱㈣鍙充晶銆佸埛鏂版寜閽乏渚э級鏂板"+ 娣诲姞鏄犲皠"鎸夐挳锛岄敋鐐?ref 鐢ㄤ簬 popover 瀹氫綅
   - 琛屾搷浣滃垪涓夋寜閽紙鏉′欢鏄剧ず锛夛細
     - 鈬?**缂栬緫鏄犲皠**锛氫粎 `canEditMapping` 琛岋紙hint type=requested_only 鎴?requested_equals_upstream锛?
     - 鈻?**娴嬭瘯妯″瀷**锛歚canTest` 琛岋紙鏈?billing_basis_hint 鎴?provider=antigravity锛?
     - 鉁?鏌ョ湅璇︽儏 / 鍒涘缓瀹氫环锛氭墍鏈夎锛堜繚鎸佸師琛屼负锛?
   - `handleMappingSaved` 浜嬩欢鍥炶皟璋冪敤 `loadData` 鏁磋〃鍒锋柊锛堟槧灏勫彉鍖栧奖鍝嶆墍鏈夊窘鏍囧拰 related_models锛?
   - `RowDisplay` 鎺ュ彛鎵?`canEditMapping` / `canTest` 瀛楁锛屽湪 `displayRows` computed 閲屾寜 hint 绫诲瀷鎺ㄥ

4. **鍒犻櫎鏃?mapping tab**锛?
   - `ModelConfigView.vue` 浠?350 琛岀簿绠€鍒?40 琛岋紝鍙繚鐣?pricing 鍜?rate 涓や釜 tab + 蹇呰鐨?AppLayout 澹?
   - 鍘嗗彶 URL 鍏煎锛歚?tab=mapping` 琚嚜鍔ㄥ洖閫€鍒?pricing
   - 鏃?i18n key锛坄admin.modelConfig.antigravityMapping` / `testTitle` 绛夛級鏆傛湭娓呯悊锛岀暀鐫€涓嶇敤涓嶅奖鍝嶈涓猴紝鍚庣画鍙殢涓婃父鍚屾涓€璧锋竻闄?

**楠岃瘉**:
- `pnpm run typecheck` 閫氳繃
- 鍓嶇 dev server 鐑噸杞藉悗鎵嬫祴娴佺▼锛?
  - 鐐?+ 娣诲姞鏄犲皠" 鈫?濉?from/to 鈫?淇濆瓨 鈫?琛ㄦ牸 reload 鏂版槧灏勫嚭鐜?
  - 鐐规煇琛?缂栬緫鏄犲皠" 鈫?鏀逛笂娓稿悕 鈫?淇濆瓨 鈫?鍒楄〃鏇存柊锛涘窘鏍囧拰 +N 璁℃暟姝ｇ‘鑱斿姩
  - 缂栬緫 popover 搴曢儴鐐?鍒犻櫎鏄犲皠" 鈫?纭 鈫?璇ユ槧灏勪粠琛ㄤ腑娑堝け
  - 鐐规煇琛?娴嬭瘯" 鈫?dialog 寮瑰嚭 鈫?閫夎处鍙?鈫?鍙戦€?鈫?娴佸紡杈撳嚭姝ｇ‘鏄剧ず
  - 鏃?mapping tab 褰诲簳娑堝け锛屽彧鍓?Pricing 鍜?Rate Multipliers 涓や釜 tab

**宸茬煡闄愬埗 / 鏈潵杩唬**:
- `upstream_only` 绫诲瀷鐨勮锛堜粎浣滀负鏄犲皠 value 瀛樺湪銆佹棤鍚屽悕鑷槧灏勶級涓嶆彁渚?缂栬緫鏄犲皠"鎸夐挳锛涘綋鍓?Antigravity 榛樿鏄犲皠閲屾绫诲瀷涓虹┖锛堟墍鏈?value 閮芥湁鍚屽悕鑷槧灏勶級锛屽疄闄呮棤褰卞搷
- 璐﹀彿绾?`credentials.model_mapping` 鐨勭鐞嗕粛璧板師璐﹀彿缂栬緫鐣岄潰锛屾湰娆℃病鏈夊悎骞讹紙鐢ㄦ埛鏄庣‘鍙姹傚钩鍙扮骇鏄犲皠绠＄悊鍚堝叆锛?
- 鏃?`admin.modelConfig.*` 涓嬬殑 mapping 鐩稿叧 i18n key 鏆傜暀鏈竻鐞?

## [2026-04-15] feat(admin): 妯″瀷瀹氫环椤垫繁搴︿紭鍖栵紙涓嬪垝绾?tab / 鍐呰仈 popover / 寤鸿浠?/ billing hint锛?

**褰卞搷鑼冨洿**:
- `backend/internal/service/global_model_pricing_service.go`锛圡odelPricingListItem/Detail 鍔犲瓧娈点€乻uggestPricing銆乮sAntigravityStubModel銆丄ntigravity 鍙嶆壂 mapping value锛?
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`锛堜笅鍒掔嚎 tab 绛涢€夊櫒銆乧omputePriceDelta 娑ㄨ穼鏌撹壊銆佹姌鍙?banner銆乮nline popover 鎺ュ叆銆佽绾у窘鏍囷級
- `frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue`锛堝缓璁环灞曠ず + 搴旂敤鎸夐挳锛?
- `frontend/src/components/admin/model-pricing/ModelPricingInlinePopover.vue`锛堟柊寤猴紝308 琛岋級
- `frontend/src/api/admin/modelPricing.ts`锛堢被鍨嬫墿鍏咃細suggested_prices/suggested_from/billing_basis_hint锛?
- `frontend/src/i18n/locales/zh.ts` & `en.ts`锛垀20 鏉℃柊 key锛?

**涓婃父鍏煎鎬?*: 涓瓑銆傛墍鏈夋敼鍔ㄩ泦涓湪浜屽紑鐙湁鐨勩€屾ā鍨嬪畾浠枫€嶇鐞嗙晫闈紙2026-04-12 鏂板鐨?ModelPricingTab 鍜岀浉鍏虫湇鍔℃柟娉曚笂娓镐笉瀛樺湪锛夛紝涓庝笂娓镐富绾挎棤鍐茬獊銆侴lobalModelPricing 瀹炰綋娌℃湁鏂板 DB 瀛楁锛岄浂 migration銆傞渶瑕佺暀鎰忕殑鏄笂娓告湭鏉ヨ嫢缁?`ModelPricingListItem` / `ModelPricingDetail` 澧炲姞瀛楁鏃惰閬垮厤鍜屾湰娆℃柊澧炲瓧娈靛懡鍚嶅啿绐併€?

**鑳屾櫙**:

姝ゅ墠銆屾ā鍨嬮厤缃?鈫?妯″瀷瀹氫环銆峊ab 宸茶兘姝ｇ‘灞曠ず Gemini/Antigravity 绛涢€夌粨鏋滐紝浣嗙鐞嗗憳鐪熸浣跨敤璇ラ〉闈㈢鐞嗗叏灞€瀹氫环鏃惰繕鏈夊洓涓棝鐐癸細
1. 琛ㄦ牸閲屾瘡涓环鏍煎瓧娈靛埌搴曟潵鑷?LiteLLM 杩樻槸琚?global/channel 瑕嗙洊鐪嬩笉娓咃紝鍙湁 input/output 鍒楁湁绠€鍗曢鑹诧紝cache 鍒楀畬鍏ㄦ病鏍?
2. 鏉ユ簮绛涢€?Tab 椤哄簭鏄€屽叏閮?/ 鍏ㄥ眬瑕嗙洊 / 娓犻亾瑕嗙洊 / 浠?LiteLLM銆嶏紝浣嗗疄闄呰璐逛紭鍏堢骇鏄?`Channel > Global > LiteLLM`锛岄『搴忓弽浜嗕笖椤甸潰娌℃湁浠讳綍浣嶇疆璇存槑杩欎釜浼樺厛绾?
3. 鏀逛竴涓ā鍨嬬殑 input 浠疯鐐归搮绗斿浘鏍囧脊鍏ㄥ睆 dialog 鈫?缈绘壘 鈫?鏀?鈫?淇濆瓨 鈫?鍏抽棴锛屽楂橀璋冨弬鍦烘櫙澶噸
4. 涓婁竴杞ˉ鐨?Antigravity 涓撴湁 stub锛坄gemini-3-pro-high`銆乣gpt-oss-120b-medium`銆乣tab_flash_lite_preview` 绛?8+ 涓級涓€鎺?`-`锛岀鐞嗗憳鏃犱粠涓嬫墜锛涗笖杩欎簺妯″瀷娑夊強璐﹀彿绾ф槧灏勶紝涓庢笭閬撳畾浠风殑 `billing_model_source` 鏈哄埗寮虹浉鍏?

**璁捐鍐崇瓥**锛?

缁忚繃 Explore+Plan 瀛愪唬鍒嗘瀽锛屽叧閿彂鐜帮細`model_pricing_resolver.go` 鐨?`resolveBasePricing(model)` 鏀跺埌鐨?`model` 宸茬粡鏄 `BillingModelSource` 杩囨护鐨?`billingModel`锛屽叏灞€瑕嗙洊鐨勬煡琛?key **澶╃劧璺熼殢姣忎釜璇锋眰鎵€灞炴笭閬撶殑 billing_model_source**銆備篃灏辨槸璇寸郴缁熷凡瀹炶川涓€鑷达紝缂虹殑鍙槸**璁╃鐞嗗憳鐪嬪埌杩欎釜闅愬紡琛屼负**銆傚洜姝ゆ湰杞€?*鏂规 A**锛堝墠绔槑绀洪殣寮忚涓猴級锛屼笉鍔犲悗绔瓧娈碉紝闆?migration銆?

**鍙樻洿璇︽儏**:

1. **绛涢€夐『搴?+ 灞傜骇璇存槑**锛歴ourceTabs 椤哄簭鏀逛负 `鍏ㄩ儴 / 鏈夋笭閬撹鐩?/ 鏈夊叏灞€瑕嗙洊 / 浠?LiteLLM`锛汼ource label 鍙充晶鍔?鈸?鍥炬爣锛宧over 鏄剧ず"浼樺厛绾э細娓犻亾 > 鍏ㄥ眬 > LiteLLM"tooltip銆?
2. **宸紓楂樹寒**锛歚formatPrice` 閲嶆瀯涓?`computePriceDelta`锛岃繑鍥?`{text, className, tooltip}`銆備互 LiteLLM 涓哄熀鍑嗚绠楃浉瀵圭櫨鍒嗘瘮宸紓锛屄?% 鍐呰浣滅瓑鍚屻€傛定浠?`text-rose-600`銆佽穼浠?`text-emerald-600`銆佺瓑鍚屾垨鏃犲熀鍑?`text-primary-600`銆佺函 LiteLLM 榛樿鐏般€俢ache_write/cache_read 涓€骞跺惎鐢ㄣ€傛瘡涓暟瀛椾笂 `title` 鏄剧ず"LiteLLM 鍩哄噯 $X 路 宸紓 +Y%"銆?
3. **鎶樺彔 banner锛堣璐瑰熀鍑嗚鏄庯級**锛歴tats 鍗′笅鏂瑰姞 `<details>` 鎶樺彔鍧楋紝榛樿鏀惰捣銆傚睍寮€瑙ｉ噴 requested/upstream/channel_mapped 涓夌鍩哄噯鍚箟 + "娓犻亾榛樿 channel_mapped锛屾棤娓犻亾璺緞榛樿 requested"銆?
4. **鍐呰仈 popover 缂栬緫**锛?
   - 鏂板缓 `ModelPricingInlinePopover.vue`锛歍eleport 鍒?body 閬垮厤琛ㄦ牸 overflow 瑁佸垏锛沠ixed 瀹氫綅鑷姩閬垮紑瑙嗗彛杈圭晫锛堜笅鏂?鈫?涓婃柟銆佸彸渚?鈫?宸﹀榻愶級锛? 涓牳蹇冧环鏍煎瓧娈?+ enabled 澶嶉€夋 + 淇濆瓨/鍒犻櫎/璇︾粏璁剧疆 3 鎸夐挳锛涙瘡涓瓧娈靛甫 LiteLLM 鍩哄噯 placeholder锛汦nter 鎻愪氦
   - 琛ㄦ牸 4 涓环鏍?`<td>` 鍔?`@click` 瑙﹀彂 popover + `cursor-pointer hover:bg-primary-50/50`
   - 淇濆瓨鏃?*涓嶆暣琛?reload**锛岀埗缁勪欢 `handleInlineSaved` 灏卞湴鏇挎崲 items 骞跺樊閲忔洿鏂?stats.global_override_count
   - Popover 淇濈暀鍘?override 鐨?provider/notes/image_output_price/per_request_price 绛夊瓧娈碉紙PATCH 宸噺锛夛紝閬垮厤娓呴浂
   - `< lg` 鏂偣 `window.matchMedia('(max-width: 1023px)')` 鍥為€€鍒板師 dialog锛泂tub 妯″瀷锛堥渶瑕侀厤 provider/notes/寤鸿浠凤級涔熷洖閫€鍒?dialog
   - 绛涢€夊櫒涓嬫柟鍔犵伆鑹插皬瀛楁彁绀?鐐瑰嚮琛ㄦ牸涓殑浠锋牸鏁板瓧鍙揩閫熺紪杈?
5. **Antigravity stub 鍙厤缃?+ 寤鸿浠?*锛?
   - 琛ㄦ牸閾呯瑪鍥炬爣瀵?stub 琛?tooltip 鍒囨崲涓?鍒涘缓瀹氫环"
   - 鍚庣 `ModelPricingDetail` 鍔?`SuggestedPrices` / `SuggestedFrom` 瀛楁锛屼粎鍦ㄦ棤 LiteLLM + 鏃?global_override 鏃跺～鍏?
   - 鏂?`suggestPricing` 鏂规硶鎸変互涓嬮摼鍖归厤锛氭樉寮忔槧灏勮〃锛坄tab_flash_lite_preview 鈫?gemini-2.5-flash-lite`銆乣gpt-oss-120b-medium 鈫?gpt-4o-mini`锛夆啋 鍓ョ `-high/-low/-medium` 妗ｄ綅鍚庣紑 鈫?鍓ョ `-thinking` 鈫?Gemini 鐗堟湰闄嶇骇锛?.x 鈫?2.5锛?
   - `ModelPricingDetailDialog.vue` 鍦?Global Override section 椤堕儴灞曠ず"馃挕 寤鸿浠凤紙鏉ヨ嚜 xxx锛壜?搴旂敤"琛岋紝鐐瑰嚮搴旂敤鎶婂€煎～鍏?form锛堥渶绠＄悊鍛樼‘璁や繚瀛橈紝涓嶈嚜鍔ㄥ叆搴擄級
   - 淇涓€涓壇浣滅敤 bug锛歚pricingService.GetModelPricing` 甯︽ā绯婂尮閰嶏紝瀵?Antigravity 涓撴湁 stub 浼氶敊璇尮閰嶅埌涓嶇浉鍏崇殑 LiteLLM 妯″瀷浠锋牸銆傛柊澧?`isAntigravityStubModel` 妫€娴嬶紙model 鍦?Antigravity mapping keys 浣嗕笉鍦?LiteLLM 绮剧‘妯″瀷鍒楄〃锛夛紝璇︽儏鎺ュ彛瀵?stub 璺宠繃 LiteLLM 骞惰蛋 suggestPricing锛屼笌鍒楄〃鎺ュ彛鐨勭簿纭尮閰嶈涔変竴鑷?
6. **鍙屽垪妯″瀷鍚?+ 璁¤垂妯″紡鍒?*锛堣凯浠ｈ繃 badge 鏂规鍚庣殑鏈€缁堝舰鎬侊級锛?
   鐢ㄦ埛鍙嶉灏?badge 澶娊璞★紝浜庢槸鎶婁俊鎭彁鍗囦负姝ｅ紡琛ㄦ牸鍒椻€斺€旂洿鎺ヤ綋鐜?瀹㈡埛绔姹傚悕 / 涓婃父鍚?/ 璁¤垂妯″紡"涓夊厓缁勫績鏅烘ā鍨嬨€?
   - 鍚庣 `ModelPricingListItem.BillingBasisHint` 浠庡崟瀛楃涓插崌绾т负缁撴瀯浣?`{ type, related_models }`
     涓夌 type锛?
     - `requested_equals_upstream`鈥斺€斿悓鍚嶆槧灏勬垨绾?LiteLLM 妯″瀷锛岃姹傚悕 = 涓婃父鍚?
     - `upstream_only`鈥斺€旀ā鍨嬫槸鏄犲皠 value锛屽鎴风涓嶇洿鎺ヨ姹傚畠锛況elated_models 鍒楀嚭鎵€鏈夋槧灏勬簮璇锋眰鍚嶏紙鏀寔澶氬涓€锛?
     - `requested_only`鈥斺€旀ā鍨嬫槸鏄犲皠 key锛岃鏄犲皠鍒板叾浠栧悕瀛楋紱related_models 鍗曞厓绱犱负涓婃父鐩爣
     浼樺厛绾?`same_name > upstream_only > requested_only`锛泂ameName 鎯呭喌涔熷～ related_models 鎵胯浇"琚皝鏄犲皠鍒版垜"淇℃伅锛岄伩鍏嶄俊鎭涪澶?
   - 鍓嶇 `ModelPricingTab.vue` 鎶婂師 Model 鍗曞垪鎷嗘垚銆岃姹傛ā鍨嬪悕 / 涓婃父妯″瀷鍚嶃€嶅弻鍒楋紝骞舵柊澧炪€岃璐规ā寮忋€嶅垪锛堝彧璇绘爣绛撅細鎸夎姹?/ 鎸変笂娓?/ 璇锋眰=涓婃父锛?
     姣忚鏍规嵁 hint 鎺ㄥ涓ゅ垪灞曠ず鍊硷細
     - `requested_equals_upstream`锛氫袱鍒楃浉鍚?= model 鑷韩锛岃嫢 related_models 闈炵┖灞曠ず `+N` 灏忓窘鏍?+ hover 鍒楀叏
     - `requested_only`锛氳姹?= model锛屼笂娓?= related_models[0]
     - `upstream_only`锛氳姹?= related_models[0]锛?N 琛ㄧず澶氬涓€锛夛紝涓婃父 = model
   - Provider / Channels 鍒楁敼涓?`xl:table-cell`锛? 1280px 闅愯棌锛夛紝鑺傜渷瀹藉害
   - 璁¤垂妯″紡鍒?*涓嶅彲缂栬緫**锛屽洜涓哄畠涓嶆槸杩欐潯璁板綍鐨勫睘鎬р€斺€斿畠鏄粠鏄犲皠鍏崇郴鑷姩鎺ㄦ柇鐨勫睍绀烘爣绛撅紝瀹為檯璁¤垂鍩哄噯鐢辫姹傛墍灞炴笭閬撶殑 `billing_model_source` 鍐冲畾
   - banner 鐨勫睍寮€鍐呭閲岃ˉ涓€鏉?`billingBasisColumnNote` 璀﹀憡寮忚鏄庯紝鏄庣‘鍛婄煡鐢ㄦ埛"杩欎竴鍒楀彧璇?+ 瀹為檯鐢辨笭閬撳喅瀹?

**楠岃瘉**:
- `pnpm run typecheck` 閫氳繃
- `go build ./...` 閫氳繃锛宍go vet ./internal/service/` 鏃犲憡璀?
- 鏈湴 API 瀹炴祴锛?
  - `provider=antigravity` 杩斿洖 30 鏉★紝鍚?type 鍒嗗竷绗﹀悎棰勬湡锛?
    - `requested_equals_upstream`锛歚claude-opus-4-6-thinking`锛坮elated_models=[opus-4-5-20251101, opus-4-5-thinking, opus-4-6] 琛ㄧず琚?3 涓姹傛槧灏勫埌锛夈€乣claude-sonnet-4-6`锛堣 haiku-4-5 / haiku-4-5-20251001 鏄犲皠鍒帮級銆乣gemini-3.1-flash-image`锛堣 3 涓?image 妯″瀷鏄犲皠鍒帮級绛?
    - `requested_only`锛歚claude-haiku-4-5 鈫?claude-sonnet-4-6`銆乣claude-opus-4-6 鈫?claude-opus-4-6-thinking`銆乣gemini-3-pro-preview 鈫?gemini-3-pro-high` 绛?
    - `upstream_only`锛欰ntigravity 榛樿鏄犲皠鐨?value 鍩烘湰閮芥湁鍚屽悕鑷槧灏勶紝鎵€浠ユ湰绫诲埆鏆傛椂娌℃暟鎹€斺€旇繖鏄鍚堟暟鎹泦鐜扮姸鐨勯鏈?
  - `GET /admin/model-pricing/gemini-3-pro-high` 鈫?寤鸿浠锋潵鑷?`gemini-2.5-pro`
  - `GET /admin/model-pricing/tab_flash_lite_preview` 鈫?寤鸿浠锋潵鑷?`gemini-2.5-flash-lite`
  - `GET /admin/model-pricing/gpt-oss-120b-medium` 鈫?寤鸿浠锋潵鑷?`gpt-4o-mini`锛堜箣鍓嶈 LiteLLM 妯＄硦鍖归厤姹℃煋鎴?`1.25e-6 / 1e-5` 閿欎环锛屽凡淇锛?
  - `GET /admin/model-pricing/claude-opus-4-6-thinking` 鈫?姝ｅ父杩斿洖 LiteLLM 浠锋牸锛屼笉瑙﹀彂 suggestPricing

**宸茬煡闄愬埗**:
- 鏄惧紡寤鸿浠锋槧灏勮〃 `antigravityProprietarySuggestMap` 闇€瑕佸湪 Google/OpenAI 鍙戞柊妯″瀷鏃剁淮鎶わ紝鐩墠鍙 `tab_flash_lite_preview` / `gpt-oss-120b-medium` 涓ゆ潯
- Popover 浠呮敮鎸?4 涓牳蹇冧环鏍煎瓧娈碉紱provider/notes/image_output_price/per_request_price/billing_mode 浠嶉渶璧板師 dialog锛堥€氳繃 popover 鐨?璇︾粏璁剧疆鈥?鎸夐挳璺宠浆锛?
- 鏂规 A 鐨勪繚瀹堥€夋嫨锛氭湭鏉ヨ嫢鍑虹幇"鍚屼竴妯″瀷鍦ㄤ笉鍚?billing_model_source 涓嬮渶瑕佷笉鍚屼环"鐨勫疄闄呬笟鍔″満鏅紝闇€瑕佸崌绾у埌鏂规 B锛堢粰 GlobalModelPricing 鍔?billing_model_source 瀛楁 + 浜岀淮缂撳瓨锛夛紝鏈涓嶉樆濉炶鎵╁睍

## [2026-04-15] fix(admin): 妯″瀷瀹氫环椤?Gemini/Antigravity 杩囨护澶辨晥

**褰卞搷鑼冨洿**:
- `backend/internal/service/global_model_pricing_service.go`锛坒ilterItems 鍒悕鍖归厤 + Antigravity 妯″瀷琛ュ叏锛?
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`锛圙emini 涓嬫媺 value 瀵归綈锛?

**涓婃父鍏煎鎬?*: 浣庨闄┿€俙filterItems`/`ListAllModels` 鏄簩寮€ 2026-04-12 鏂板鐨勭粺涓€瀹氫环绠＄悊鐣岄潰锛堣涓嬫枃锛夛紝涓婃父娌℃湁鍚屽悕鍑芥暟锛涘敮涓€鍙兘鍐茬獊鐐规槸 `domain.ResolveAntigravityDefaultMapping` 鐨勫紩鍏ャ€?

**鑳屾櫙**:
绠＄悊鍚庡彴銆屾ā鍨嬮厤缃?鈫?妯″瀷瀹氫环銆峊ab 閲岋紝provider 涓嬫媺閫?Gemini 鎴?Antigravity 鏃跺垪琛ㄤ负绌恒€傛牴鍥狅細

1. **Gemini**锛氬墠绔笅鎷?value 鏄?`vertex_ai`锛屼絾 LiteLLM JSON 閲?Gemini 瀹舵棌鐨?`litellm_provider` 瀛楁瀹為檯鍊兼槸 `gemini`锛圙oogle AI Studio锛夋垨甯﹀悗缂€鐨?`vertex_ai-language-models` / `vertex_ai-vision-models` / `vertex_ai-embedding-models`锛圴ertex AI锛夛紝`filterItems` 鐨?`strings.ToLower(item.Provider) != providerLower` 涓ユ牸鐩哥瓑鍖归厤涓€涓兘鍛戒笉涓€?
2. **Antigravity**锛欰ntigravity 鏄簩寮€鑷爺骞冲彴锛孡iteLLM 閲屼笉瀛樺湪浠讳綍 `antigravity` provider 鏉＄洰锛涘悓鏃?`DefaultAntigravityModelMapping` 閲屽畾涔夌殑 Antigravity 鍙敤妯″瀷锛堝 `gemini-3-pro-high`銆乣tab_flash_lite_preview`锛夋牴鏈笉鍦ㄥ垪琛ㄦ灇涓炬潵婧愶紙LiteLLM + 鍏ㄥ眬瑕嗙洊锛夐噷銆?

**鍙樻洿璇︽儏**:
- 鎶藉嚭 `providerMatches(item, providerLower, antigravityModelSet)` 鎶婁弗鏍肩浉绛夋敼涓哄埆鍚嶆劅鐭ワ細
  - `gemini` 鈫?鍖归厤 `gemini` 鎴?`vertex_ai` 鍓嶇紑
  - `openai` 鈫?鍖归厤 `openai` 鎴?`text-completion-openai`
  - `antigravity` 鈫?鍖归厤 `provider=antigravity` 鎴栨ā鍨嬪悕鍛戒腑 `domain.ResolveAntigravityDefaultMapping()` 鐨?key
  - 鍏跺畠锛坅nthropic/bedrock 绛夛級鈫?淇濈暀鍘熶弗鏍肩浉绛?
- `ListAllModels` 鍚堝苟闃舵鏂板涓€杞亶鍘?`ResolveAntigravityDefaultMapping()`锛屽 LiteLLM 鍜屽叏灞€瑕嗙洊閮芥病鏈夌殑妯″瀷鍚嶈ˉ涓€鏉?provider=antigravity 鐨?stub ListItem锛屼繚璇?Antigravity 涓撴湁妯″瀷鍦ㄥ垪琛ㄩ噷鍙鍙銆?
- 鍓嶇 `ModelPricingTab.vue` 鐨勪笅鎷夋妸 `<option value="vertex_ai">Gemini</option>` 鏀逛负 `value="gemini"`锛屼笌鍚庣鏂板埆鍚嶅榻愩€?
- `modelSet` 鍚堝苟寰幆鏂板鐨勫啓鍏ョ‘淇?Antigravity stub 鍘婚噸鏃?dedup 鍩哄噯瀹屾暣锛堜箣鍓?all-overrides 寰幆婕忓啓 modelSet锛屽伓鍙戦噸澶嶏紱涓€璧蜂慨鎺夛級銆?

**楠岃瘉**:
- `go build ./internal/service/ ./internal/handler/admin/` 閫氳繃
- `go vet ./internal/service/` 鏃犲憡璀?
- `pnpm run typecheck` 鏃犻敊璇?

## [2026-04-15] feat(tools): 鏂板鍥剧墖鐢熸垚 API 鍘嬪姏娴嬭瘯鑴氭湰

**褰卞搷鑼冨洿**:
- `tools/image_stress_test.py`锛堟柊澧烇紝鍗曟枃浠?Python 寮傛鍘嬫祴鑴氭湰锛寏580 琛岋級

**涓婃父鍏煎鎬?*: 绾柊澧炲鎴风宸ュ叿锛屼笉瑙︾ backend/frontend/deploy锛屾棤涓婃父鍐茬獊椋庨櫓銆?

**鑳屾櫙**:
瀹㈡埛鍙嶉閫氳繃 API 璋冪敤 Gemini 鍥剧墖鐢熸垚妯″瀷锛坄gemini-3-pro-image` / `gemini-2.5-flash-image` 绛夛級鏃堕敊璇巼寰堥珮锛岄渶瑕佷竴涓彲澶嶇幇銆佸彲璇婃柇鐨勫伐鍏峰幓瀹氫綅闂鍒板簳鍑哄湪涓婃父璐﹀彿姹犮€佽皟搴﹀櫒銆佽繕鏄?Anthropic 鍏煎缈昏瘧灞傘€?

**鍙樻洿璇︽儏**:
- 鐢?`httpx[http2]` + `asyncio` 瀹炵幇鍙楁帶骞跺彂鍘嬫祴
- 鏀寔涓ゆ潯鍏ュ彛璺緞鐨勫姣旓細
  1. `gemini-native`锛歚POST /v1beta/models/{model}:generateContent`
  2. `anthropic-messages`锛歚POST /v1/messages`锛堣蛋 `GeminiMessagesCompatService` 缈昏瘧灞傦級
- 涔熸敮鎸?`--stream` 璧?`:streamGenerateContent`锛屽懡涓唬鐮侀噷 `handleGeminiStreamToNonStreaming` 鐨勬祦寮忓垎鏀?
- 閿欒鍒嗙被瀵归綈鏈嶅姟绔殑澶辫触淇″彿锛歚empty_stream` / `safety_block` / `google_config_error` / `signature_error` / `overloaded_529` / `rate_limit_429` / `gateway_5xx` / `auth_401_403` / `client_4xx` / `timeout` / `network_error`
- 鐗瑰埆璇嗗埆 "200 OK 浣嗘棤鍥?锛坄candidates[0].content.parts` 閲屾棤 `inlineData`锛屾垨 `finishReason` 灞炰簬 safety 绫伙級鈥斺€?杩欐槸瀹㈡埛鏈€瀹规槗鎶婂畠褰?bug 鎶ョ殑 case
- 姣忎釜璇锋眰璁板綍 `X-Request-ID`锛宍summary.md` 浼氬垪鍑?top 澶辫触 request_id 渚夸簬 SSH 鍒版湇鍔″櫒鍏宠仈鏃ュ織
- 杈撳嚭缁撴瀯锛歚output/stress-<timestamp>/{run.json, requests.jsonl, summary.md}`锛宍output/` 宸插湪 `.gitignore`
- 榛樿鐩爣 `https://zerocode.kaynlab.com`锛孉PI key 浠?`$SUB2API_KEY` 璇诲彇
- Windows 鍙嬪ソ锛氳嚜鍔ㄦ妸 stdout/stderr 閲嶉厤缃负 UTF-8 閬垮厤 cp936 涔辩爜

**浣跨敤**:
```bash
export SUB2API_KEY=sk-xxx
python tools/image_stress_test.py --total 50 --concurrency 5 --mode gemini-native
```

瀹屾暣鎵ц娴佺▼锛堝啋鐑?鈫?鍩虹嚎 鈫?骞跺彂鎵?鈫?妯″紡瀵规瘮 鈫?妯″瀷瀵规瘮 鈫?娴佸紡锛夎 `tools/image_stress_test.py` 妯″潡娉ㄩ噴椤堕儴銆?

---

## [2026-04-15] feat: 鏂板浼佷笟寰俊鏀粯鏂瑰紡

**褰卞搷鑼冨洿**: backend/internal/payment/, frontend/src/views/admin/
**涓婃父鍏煎鎬?*: 浣庡啿绐侀闄╋紝鏂板鏂囦欢涓轰富
**鍙樻洿璇︽儏**:
- 鏂板 payment/provider/wechat_work.go
- 娣诲姞 WeChatWorkProvider 瀹炵幇 PaymentProvider 鎺ュ彛
- 鍓嶇绠＄悊椤垫柊澧炰紒涓氬井淇℃敮浠橀厤缃〃鍗?
- config.yaml 鏂板 payment.wechat_work 閰嶇疆娈?

**鍏宠仈 Issue/PR**: #12

## [2026-04-14] chore(deploy): remote_exec.py 澧炲姞 --update 蹇嵎鏂瑰紡閬垮紑 MSYS2 璺緞杞崲

**褰卞搷鑼冨洿**:
- `deploy/remote_exec.py`锛?*鏈?tracked锛屾湰鍦版敼鍔?*锛?gitignore 涓紱鍥犲惈鏄庢枃 SSH 鍑瘉涓嶅叆搴擄級
- `CLAUDE.md`锛坵orkflow + 鐢熶骇鏈嶅姟鍣ㄧ珷鑺傦級
- `docs/dev/UPSTREAM_SYNC.md`锛堥儴缃叉寚浠よ寖渚嬶級

**涓婃父鍏煎鎬?*: 浠呭奖鍝嶆湰鍦板伐浣滄祦锛屼笉娑夊強浠讳綍涓婃父鏂囦欢銆?

**鑳屾櫙**:
2026-04-14 v0.1.112 鍚堝苟瀹屾垚鍑嗗閮ㄧ讲鏃讹紝鍦?Git Bash 涓嬫墽琛?
`python deploy/remote_exec.py "/opt/sub2api/update.sh"` 鎶?
`bash: line 1: D:/program: No such file or directory` 澶辫触銆?
瀹氫綅鍚庣‘璁ゆ槸 MSYS2 argv path conversion锛欸it Bash 浼氭妸浠讳綍鐪嬭捣鏉ュ儚
POSIX 缁濆璺緞鐨?argv 鍙傛暟锛坄/opt/...`锛夋倓鎮勮浆鎴?Windows 璺緞鍚庢墠浜ょ粰
Python锛屼簬鏄?argv[1] 鍙樻垚浜?`D:\program files\...\opt\sub2api\update.sh`锛?
SSH 杩滅鏀跺埌涓€涓笉瀛樺湪鐨勮矾寰勮嚜鐒跺け璐ャ€?

**鍙樻洿璇︽儏**:
- `deploy/remote_exec.py`
  - 鏂板 `SHORTCUTS` 瀛楀吀 + `--update` 蹇嵎鏂瑰紡锛屽唴閮ㄧ敤 Python 瀛楃涓插瓧闈㈤噺
    `"bash /opt/sub2api/update.sh"`锛屽畬鍏ㄧ粫杩?MSYS2 argv 杞崲
  - 鏂板 `--env` 妯″紡浠?`REMOTE_CMD` 鐜鍙橀噺璇诲懡浠わ紙浣嗕粛闇€閰嶅悎
    `MSYS_NO_PATHCONV=1` 鎵嶈兘璁?Git Bash 涓嶈浆 env 閲岀殑璺緞锛涗綔涓?escape hatch锛?
  - 鏂板缁撴瀯鍖?docstring 璇存槑 MSYS2 闄烽槺鍜屽洓绉?workaround 浼樺厛绾?
  - `run()` 榛樿 timeout 浠?300s 鎻愬崌鍒?600s锛岄€傞厤 Docker build 鍦烘櫙
  - 杈撳嚭 decode 鍔?`errors="replace"`锛岄伩鍏嶄簩杩涘埗姹℃煋鏃?UnicodeDecodeError

- `CLAUDE.md` workflow 姝ラ 4/5 涓庛€岀敓浜ф湇鍔″櫒銆嶇珷鑺?
  - 閮ㄧ讲鍛戒护鏀逛负 `python deploy/remote_exec.py --update`
  - 杩藉姞 MSYS2 gotcha 璀﹀憡鍜屾寚鍚?remote_exec.py docstring 鐨勫紩鐢?
  - 鐢熶骇鏈嶅姟鍣?SSH 瀛楁璇存槑 ad-hoc 鍛戒护浠呴檺涓嶄互 `/` 寮€澶寸殑鍛戒护

- `docs/dev/UPSTREAM_SYNC.md`
  - 鏈閮ㄧ讲鏉＄洰杩藉姞宸查儴缃叉爣璁?
  - 閮ㄧ讲鎸囦护鑼冧緥鏀圭敤 `--update` 骞舵敞鏄庢棫鐢ㄦ硶琚純鐢ㄧ殑鍘熷洜

**閮ㄧ讲楠岃瘉**:
- `python deploy/remote_exec.py --update` 绔埌绔窇閫氾細pull锛堝凡 up-to-date锛夆啋
  docker build 鈫?docker compose up 鈫?health check `{"status":"ok"}` 鈫?ps 鏄剧ず
  sub2api 瀹瑰櫒 `Up 8 seconds (healthy)`銆?

**鍏宠仈**: 鏃?issue銆備慨澶嶆簮浜?2026-04-14 v0.1.112 鍚屾閮ㄧ讲杩囩▼涓彂鐜般€?

---

## [2026-04-14] fix(billing): 淇鍏ㄥ眬妯″瀷瀹氫环瑕嗙洊鍦?Anthropic 缃戝叧澶辨晥鍙婂澶勮璐规紡娲?

**褰卞搷鑼冨洿**:
- backend/internal/service/model_pricing_resolver.go锛堟牳蹇冭В鏋愬櫒閲嶅啓锛?
- backend/internal/service/global_model_pricing.go锛堝垹闄ゆ湁 bug 鐨?ToModelPricing锛?
- backend/internal/service/global_model_pricing_cache.go锛堟柊澧烇級
- backend/internal/service/global_model_pricing_service.go锛堟敞鍏ョ紦瀛樺苟鍦?CUD 鏃跺け鏁堬級
- backend/internal/service/gateway_service.go锛坮esolveChannelPricing 鍚屾椂鎺ュ彈 Global 鏉ユ簮锛?
- backend/internal/service/wire.go锛圥rovider set 杩藉姞 NewGlobalPricingCache锛?
- backend/cmd/server/wire_gen.go锛堟墜鍔ㄥ悓姝?DI 鎺ョ嚎锛?
- backend/internal/handler/admin/model_pricing_handler.go锛圲pdateOverride 宸噺鏇存柊锛?
- backend/internal/service/model_pricing_resolver_test.go锛堟柊澧?5 涓洖褰掓祴璇曪級

**涓婃父鍏煎鎬?*: 楂樺害鍙兘浜х敓鍐茬獊 鈥斺€?瑙﹀強涓婃父 resolver 涓?gateway_service 鐨勬牳蹇?
璁¤垂璺緞锛屼互鍙?wire_gen.go銆傚悎骞朵笂娓告椂濡傛灉瀹樻柟閲嶆瀯浜?ModelPricingResolver 鎴?
GatewayService.calculateTokenCost 闇€瑕侀噸鏂版暣鍚堟湰淇銆?

**鑳屾櫙**:
瀹¤绠＄悊鍚庡彴"妯″瀷閰嶇疆 鈫?Pricing"椤甸潰鐨勩€屽叏灞€瑕嗙洊銆嶅姛鑳芥槸鍚︾鍒扮鐢熸晥锛?
鍙戠幇瀹冨湪澶氭潯璺緞涓婅闈欓粯缁曡繃鎴栦涪澶卞瓧娈碉紝璇﹁鏈 commit 璇存槑銆?

**鍙樻洿璇︽儏**锛堟寜 bug 瀵瑰簲淇锛?

- **Bug A 鈥?Anthropic 缃戝叧鐑矾寰勭粫杩囧叏灞€瑕嗙洊**
  `gateway_service.go:resolveChannelPricing` 鍘熸湰鍙湪 `Source==Channel` 鏃惰繑鍥?
  resolved锛屽鑷淬€屽彧閰嶄簡鍏ㄥ眬瑕嗙洊銆佹病閰嶆笭閬撱€嶇殑鎯呭舰浼氬洖钀藉埌 `CalculateCost` 鏃?
  璺緞銆傛棫璺緞瀹屽叏涓嶆煡 GlobalPricingRepository锛屽叏灞€瑕嗙洊 鈫?闈欓粯澶辨晥銆備慨澶嶏細
  鏀惧鏉′欢涓?`Source==Channel || Source==Global`锛屽悓鏃朵繚鐣欏嚱鏁板悕浠ュ噺灏?diff銆?

- **Bug B 鈥?ResolvedPricing.Mode 蹇界暐鍏ㄥ眬瑕嗙洊鐨?BillingMode**
  鍘?`Resolve` 鎶?`Mode` 纭紪鐮佷负 `BillingModeToken`锛屽彧鍦ㄦ笭閬撳彔鍔犲垎鏀噷鏀广€?
  鍚庢灉锛氱鐞嗗憳鍦ㄥ叏灞€瑕嗙洊閲岄€?`per_request` / `image` 鈫?鍚庣浠嶆寜 token 璁¤垂 鈫?
  鍗曚环鍏ㄤ负 0 鈫?鐢ㄦ埛鍏嶈垂銆備慨澶嶏細`resolveBasePricing` 杩斿洖 `(pricing, mode,
  defaultPerRequestPrice, source)` 鍥涘厓缁勶紝`Resolve` 鍘熸牱濉炶繘 `ResolvedPricing`銆?

- **Bug C 鈥?ToModelPricing 涓㈠け Priority/闀夸笂涓嬫枃/缂撳瓨鍒嗙骇瀛楁**
  鍘?`GlobalModelPricing.ToModelPricing()` 鍙 5 涓瓧娈碉紝瀵艰嚧 Priority tier 鍗曚环
  褰掗浂銆丟PT-5.4 闀夸笂涓嬫枃鍙屽€嶈垂涓㈠け銆佺紦瀛?5m/1h 鍒嗙骇澶辨晥绛夈€備慨澶嶏細
  1. 鍒犻櫎璇ユ柟娉?
  2. `resolveBasePricing` 鍏堜粠 `BillingService.GetModelPricing` 鎷垮畬鏁村熀纭€瀹氫环
     锛堝惈 LiteLLM 鐨勬墍鏈夊瓧娈碉級锛屽啀鐢?`applyGlobalPricingOverride` 鎶婂叏灞€瑕嗙洊鐨?
     闈?nil 瀛楁鍙犲姞涓婂幓锛涜涔変笌 `applyTokenOverrides`锛堟笭閬撹鐩栵級瀹屽叏瀵归綈锛?
     鍖呮嫭 Priority 瀛楁涓庤鐩栦环鍚屾銆乣CacheWritePrice` 鍚屾椂鍐欏叆 5m/1h銆?
  3. 鏈瑕嗙洊鐨勫瓧娈碉紙Priority 鍗曚环宸€侀暱涓婁笅鏂囧€嶇巼绛夛級缁ф壙鑷?LiteLLM 鍩虹銆?

- **Bug D 鈥?姣忎釜璇锋眰涓€娆?SQL 鏃犵紦瀛?*
  鍘熷疄鐜板湪鐑矾寰勫 `global_model_pricing` 琛ㄦ瘡璇锋眰涓€娆?`SELECT`銆備慨澶嶏細鏂板
  `GlobalPricingCache`锛坰ync.RWMutex + 鎯版€у姞杞斤級锛岄娆¤闂椂涓€娆℃€ц鍏ユ墍鏈?
  `enabled=true` 鏉＄洰鍒板唴瀛?map锛屽悗缁?O(1) 鏌ヨ锛涚鐞嗗悗鍙板湪 Create/Update/
  Delete 鍚庤皟鐢?`Invalidate()` 娓呯┖缂撳瓨銆?

- **Bug E 鈥?resolveBasePricing 浣跨敤 context.Background**
  鍘熷疄鐜颁涪寮冭皟鐢ㄨ€?ctx 瀵艰嚧璇锋眰瓒呮椂鏃犳硶浼犻€掋€備慨澶嶏細缂撳瓨鍖栦箣鍚庣儹璺緞涓嶅啀杩?DB锛?
  ctx 闂鑷劧娑堝け锛涗粎鍦ㄧ紦瀛橀娆″姞杞芥椂鐢?background ctx 鎵ц涓€娆℃€у叏閲忔煡璇€?

- **Bug F 鈥?UpdateOverride 鎶婃墍鏈夋湭鎻愪緵瀛楁娓呴浂**
  鍘?handler 瀵?`InputPrice` 绛夋寚閽堝瓧娈垫棤鏉′欢璧嬪€硷紝PATCH 婕忓甫浠讳綍涓€涓瓧娈甸兘浼?
  鎶婂凡鏈変环鏍艰鐩栨垚 nil銆備慨澶嶏細缁熶竴鏀逛负"闈?nil 鎵嶈鐩?鐨勫樊閲忔洿鏂帮紙涓?
  `Model` / `Provider` / `Enabled` 瀛楁鐨勫鐞嗗榻愶級銆傝娓呴櫎鏌愪釜浠锋牸璇?
  delete 瑕嗙洊鍚庨噸寤恒€?

**鍥炲綊娴嬭瘯**锛坄model_pricing_resolver_test.go` 鏂板锛?
1. `TestResolve_GlobalOverride_PreservesPriorityAndLongContext` 鈥?瑕嗙洊 input/output
   鍚庨獙璇?Priority 鍚屾銆侀暱涓婁笅鏂囬槇鍊?鍊嶇巼/缂撳瓨 5m/1h 浠?LiteLLM 缁ф壙
2. `TestResolve_GlobalOverride_CacheWriteSyncsAllCacheFields` 鈥?瑕嗙洊 CacheWritePrice
   鍚?Creation/5m/1h 涓夊瓧娈靛叏閮ㄥ悓姝?
3. `TestResolve_GlobalOverride_DisabledIsIgnored` 鈥?enabled=false 涓嶇敓鏁?
4. `TestResolve_GlobalOverride_BillingModeRespected` 鈥?per_request 妯″紡姝ｇ‘浼犻€?
   BillingMode 鍜?DefaultPerRequestPrice
5. `TestResolve_ChannelOverride_BeatsGlobalOverride` 鈥?浼樺厛绾?Channel > Global

鎵€鏈夋柊娴嬭瘯閫氳繃锛涙棦鏈?`./internal/service/...` 鍗曞厓娴嬭瘯濂椾欢鍏ㄧ豢锛?6 绉掞級锛?
`go build ./...` 閫氳繃銆?

**鍏宠仈 Issue/PR**: 鏃狅紙鏈湴瀹¤鍙戠幇锛?

---

## [2026-04-14] feat(frontend): 浠ｇ悊鎵归噺瀵煎叆鏀寔 host:port:user:pass 绛夌畝鍐欐牸寮?

**褰卞搷鑼冨洿**:
- frontend/src/views/admin/ProxiesView.vue
- frontend/src/i18n/locales/{zh,en}.ts

**涓婃父鍏煎鎬?*: 绾墠绔敼鍔紝浠呮墿灞曡В鏋愰€昏緫鍜?UI 鏂囨锛涙湭瑙︾鍚庣 API銆傚悎骞朵笂娓歌嫢鏀?`parseProxyUrl` 鎴?`batchInputPlaceholder/Hint` 鍙兘浜х敓鍐茬獊銆?

**鍙樻洿璇︽儏**:
- `parseProxyUrl` 浠庡崟涓€ URL 姝ｅ垯鎵╁睍涓哄洓娈?fallback 瑙ｆ瀽锛?
  - A. `protocol://[user:pass@]host:port`锛堝師鏈夛紝鍗忚鏉ヨ嚜琛屽唴锛屼紭鍏堢骇鏈€楂橈級
  - B. `user:pass@host:port`锛堟柊锛屾棤鍗忚鍓嶇紑锛?
  - C. `host:port:user:pass`锛堟柊锛孭roxyScrape / 911 绫讳緵搴斿晢甯歌鏍煎紡锛涘瘑鐮佷繚鐣欒灏炬墍鏈夐潪绌虹櫧瀛楃锛?
  - D. `host:port`锛堟柊锛屾棤璁よ瘉锛?
  - 鎻愬彇鍑?`buildResult` 杈呭姪鍑芥暟缁熶竴鍋氱鍙?涓绘満鏍￠獙銆?
- 鍦?蹇嵎娣诲姞"Tab 椤堕儴鏂板"榛樿鍗忚"涓嬫媺锛坄batchDefaultProtocol`锛岄粯璁?`http`锛夛紝绠€鍐欐牸寮?B/C/D 鐨勮浼氬鐢ㄨ繖涓崗璁紱鍒囨崲鏃堕€氳繃 `@update:modelValue` 瑙﹀彂 `parseBatchInput` 閲嶇畻锛屾棤闇€鐢ㄦ埛閲嶆柊缂栬緫鏂囨湰銆?
- 鍏抽棴寮圭獥鏃跺湪 `closeCreateModal` 閲岄噸缃?`batchDefaultProtocol`銆?
- i18n锛氭墿鍏?`batchInputPlaceholder`銆乣batchInputHint` 绀轰緥锛涙柊澧?`batchDefaultProtocol`銆乣batchDefaultProtocolHint` 涓ゆ潯 key锛堜腑鑻卞弻璇榻愶級銆?
- 鍚庣 `BatchCreate` 鎺ュ彛涓嶅彉锛堜粛鎺ユ敹 `{protocol,host,port,username,password}`锛夛紝鏃犻渶杩佺Щ銆?

**鍏宠仈 Issue/PR**: 鏃?

## [2026-04-13] feat: Gemini Google One 鎵归噺 Refresh Token 瀵煎叆

**褰卞搷鑼冨洿**:
- backend/internal/pkg/geminicli/{constants.go, token_types.go}
- backend/internal/service/{gemini_oauth.go, gemini_oauth_service.go, gemini_oauth_service_test.go}
- backend/internal/repository/gemini_oauth_client.go
- backend/internal/handler/admin/gemini_oauth_handler.go
- backend/internal/server/routes/admin.go
- frontend/src/api/admin/gemini.ts
- frontend/src/composables/useGeminiOAuth.ts
- frontend/src/components/account/CreateAccountModal.vue
- frontend/src/i18n/locales/{zh,en}.ts

**涓婃父鍏煎鎬?*: 涓闄?鈥?GeminiOAuthClient 鎺ュ彛鏂板 GetUserInfo锛汣reateAccountModal 澶氬鏉′欢鍚堝苟锛屽悎骞朵笂娓告椂鍙兘鍐茬獊

**鍙樻洿璇︽儏**:
- 鍚庣锛?
  - `geminicli` 鏂板 `UserInfoURL` 甯搁噺 + `UserInfo` 绫诲瀷锛堝鐢?Google userinfo 绔偣锛?
  - `GeminiOAuthClient` 鎺ュ彛鏂板 `GetUserInfo(ctx, accessToken, proxyURL)`锛沗geminiOAuthClient` 瀹炵幇 + 娴嬭瘯 mock 鍚屾鏇存柊
  - `GeminiTokenInfo` 鍔?`Email` 瀛楁锛沗BuildAccountCredentials` 鍦?email 闈炵┖鏃跺啓鍏?`credentials.email`锛堜笌 Antigravity 瀵归綈锛屽鐢ㄨ处鍙峰垪琛ㄦ悳绱?`credentials->email` 绱㈠紩锛?
  - 鏂板 `ValidateGoogleOneRefreshToken` 鏈嶅姟鏂规硶锛歳efresh 鈫?鍥炲～ RT 鈫?`GetUserInfo` 鎷?email锛堝け璐ユ墦 warning 涓嶉樆鏂級鈫?`fetchProjectID`锛堝繀闇€锛夆啋 `FetchGoogleOneTier`锛堝け璐ュ洖钀?free锛?
  - 鏂板 `POST /admin/gemini/oauth/refresh-token` handler + 璺敱娉ㄥ唽
- 鍓嶇锛?
  - `useGeminiOAuth` 鍔?`validateGoogleOneRefreshToken` 鏂规硶锛宍buildCredentials` 閫忎紶 email
  - `CreateAccountModal`锛歚isEmailAsNameAvailable` 璁＄畻灞炴€х粺涓€ Antigravity / Gemini+google_one 鐨?鐢ㄩ偖绠变綔涓鸿处鍙峰悕"寮€鍏筹紱`handleValidateRefreshToken` 鍔?gemini 鍒嗘敮锛涙柊澧?`handleGeminiGoogleOneValidateRT`锛堝惊鐜?RT 鈫?鍗曚釜鍒涘缓锛?
  - OAuthAuthorizationFlow 鐨?`show-refresh-token-option` 鎵╁睍瑕嗙洊 `gemini + google_one`
  - zh/en i18n 琛ラ綈 `admin.accounts.oauth.gemini` 鐨?RT 鎵归噺瀵煎叆鏂囨
- 闄愬埗锛氫粎鏀寔 `google_one`锛汻T 蹇呴』鐢卞唴缃?Gemini CLI OAuth client 绛惧彂锛堣嚜寤?client 鐨?RT 浼氭姤 `unauthorized_client`锛岄敊璇彁绀哄凡鍖呭惈鐩稿簲璇存槑锛?

## [2026-04-12] feat: 缁熶竴妯″瀷瀹氫环绠＄悊鐣岄潰

**褰卞搷鑼冨洿**: backend(migrations, service, repository, handler, routes, wire), frontend(views, components, api, i18n)
**涓婃父鍏煎鎬?*: 浣庨闄╋紝鏂板鍔熻兘锛屼笉淇敼鐜版湁璁¤垂閫昏緫
**鍙樻洿璇︽儏**:
- 鏂板 `global_model_pricing` 鏁版嵁搴撹〃锛屾敮鎸佺鐞嗗憳璁剧疆鍏ㄥ眬妯″瀷瀹氫环瑕嗙洊
- 瀹氫环瑙ｆ瀽閾炬墿灞曚负锛欳hannel 鈫?Global 鈫?LiteLLM 鈫?Fallback锛堝悜涓嬪吋瀹癸紝琛ㄤ负绌烘椂琛屼负涓嶅彉锛?
- 鍚庣鏂板 GlobalModelPricingRepository銆丟lobalModelPricingService銆丮odelPricingHandler
- 鏂板 API 绔偣 GET/POST/PUT/DELETE /admin/model-pricing锛屽惈璐圭巼涔樻暟姒傝
- PricingService 鏂板 GetAllModels() 鏂规硶渚涚鐞嗗悗鍙板睍绀烘墍鏈?LiteLLM 妯″瀷
- 鍓嶇妯″瀷閰嶇疆椤垫敼涓?Tab 甯冨眬锛氭ā鍨嬪畾浠凤紙鏂板锛墊 妯″瀷鏄犲皠锛堢幇鏈夛級| 璐圭巼姒傝锛堟柊澧烇級
- 妯″瀷瀹氫环 Tab锛氬叏妯″瀷鍒楄〃 + 鎼滅储/绛涢€?+ 鍏ㄥ眬瑕嗙洊缂栬緫寮圭獥 + 娓犻亾瑕嗙洊灞曠ず
- 璐圭巼姒傝 Tab锛氬彧璇诲睍绀哄悇鍒嗙粍璐圭巼涔樻暟锛岄摼鎺ュ埌鍒嗙粍绠＄悊椤?
- 涓嫳鏂?i18n 缈昏瘧瀹屾暣

## [2026-04-12] feat: 妯″瀷閰嶇疆椤甸潰娣诲姞妯″瀷娴嬭瘯鍔熻兘

**褰卞搷鑼冨洿**: frontend/src/views/admin/ModelConfigView.vue, i18n
**涓婃父鍏煎鎬?*: 浣庨闄╋紝浠呭墠绔敼鍔?
**鍙樻洿璇︽儏**:
- ModelConfigView 鏀逛负宸﹀彸甯冨眬锛氬乏渚ф槧灏勯厤缃紝鍙充晶妯″瀷娴嬭瘯
- 娴嬭瘯鍖哄煙锛氳处鍙烽€夋嫨锛堣嚜鍔ㄩ€夌涓€涓彲鐢紝鍙墜鍔ㄥ垏鎹級銆佹ā鍨嬩笅鎷夈€佹彁绀鸿瘝杈撳叆
- 澶嶇敤 POST /admin/accounts/:id/test API锛孲SE 娴佸紡灞曠ず涓婃父鍝嶅簲
- 缁堢椋庢牸杈撳嚭鍖哄煙锛岃壊褰╁尯鍒嗭紙cyan=淇℃伅, green=鍐呭, red=閿欒, emerald=鎴愬姛锛?

## [2026-04-12] feat: 鐙珛"妯″瀷閰嶇疆"绠＄悊椤甸潰 鈥?Antigravity 鍏ㄥ眬榛樿鏄犲皠

**褰卞搷鑼冨洿**: 鍓嶅悗绔鏂囦欢
**涓婃父鍏煎鎬?*: 涓闄╋紝鏂板鏂囦欢涓轰富锛屼絾淇敼浜?account.go 鐨勯粯璁ゆ槧灏勫洖閫€閫昏緫鍜?wire_gen.go
**鍙樻洿璇︽儏**:
- 鍚庣: 鏂板 setting key `antigravity_default_model_mapping`锛屽瓨鍌ㄥ湪 settings 琛?
- 鍚庣: SettingService 鏂板 Get/Set 鏂规硶
- 鍚庣: AccountHandler 鏂板 PUT API锛屼慨鏀?GET API 浼樺厛璇?settings
- 鍚庣: domain.constants.go 鏂板 `GetAntigravityDefaultMappingOverride` 鍑芥暟鍙橀噺
- 鍚庣: account.go 涓?`resolveModelMapping` 鏀逛负璋冪敤 `domain.ResolveAntigravityDefaultMapping()`
- 鍚庣: wire_gen.go 娉ㄥ叆 override 鍑芥暟 + settingService 浼犲叆 AccountHandler
- 鍓嶇: 鏂板缓 ModelConfigView.vue锛堢嫭绔嬮〉闈紝绠＄悊鍛樺彲瑙侊級
- 鍓嶇: 鏂板璺敱 `/admin/model-config`銆佷晶杈规爮鑿滃崟椤?
- 鍓嶇: accounts API 鏂板 `updateAntigravityDefaultModelMapping`
- 鍓嶇: zh.ts/en.ts 鏂板 modelConfig i18n 鏂囨湰
- 浼樺厛绾? 鍗曡处鍙疯嚜瀹氫箟鏄犲皠 > 鍏ㄥ眬鏄犲皠锛坰ettings锛? 鍐呯疆榛樿锛坈onstants.go锛?

## [2026-04-12] fix: Antigravity 鎵归噺鍒涘缓璐﹀彿 allow_overages 鏈敓鏁?

**褰卞搷鑼冨洿**: frontend/src/components/account/CreateAccountModal.vue
**涓婃父鍏煎鎬?*: 浣庨闄╋紝鍗曡淇敼
**鍙樻洿璇︽儏**:
- 鎵归噺鍒涘缓鏃?`extra` 纭紪鐮佷负 `{}`锛屾敼涓鸿皟鐢?`buildAntigravityExtra()`锛屾纭紶閫?`allow_overages` 鍜?`mixed_scheduling`

## [2026-04-12] fix: TypeScript 绫诲瀷閿欒 ApiResponse 鏂█

**褰卞搷鑼冨洿**: frontend/src/api/client.ts
**涓婃父鍏煎鎬?*: 浣庨闄╋紝绫诲瀷鏂█淇
**鍙樻洿璇︽儏**:
- `as Record<string, unknown>` 鏀逛负 `as unknown as Record<string, unknown>`锛屾秷闄?TS2352 缂栬瘧閿欒

## [2026-04-12] feat: 璐﹀彿鍒楄〃鏄剧ず閭 + AI Credits 姹囨€?

**褰卞搷鑼冨洿**: frontend/src/views/admin/AccountsView.vue
**涓婃父鍏煎鎬?*: 涓闄╋紝AccountsView 鏀瑰姩杈冨锛屽悎骞舵椂娉ㄦ剰
**鍙樻洿璇︽儏**:
- 璐﹀彿鍚嶇О涓嬫柟鏄剧ず閭锛屽吋瀹?`credentials.email`锛圓ntigravity锛夊拰 `extra.email_address`锛圓nthropic锛?
- 绛涢€夋爮鍙充晶鏂板 AI Credits 姹囨€绘爣绛撅紝寮傛鑾峰彇骞舵寜閭鍘婚噸
- `load()` 鍜?`reload()` 鍧囪Е鍙戞眹鎬诲埛鏂?

## [2026-04-12] feat: 鎼滅储鏀寔鎸夐偖绠辨煡鎵捐处鍙?

**褰卞搷鑼冨洿**: backend/internal/repository/account_repo.go
**涓婃父鍏煎鎬?*: 浣庨闄╋紝鎼滅储鏉′欢鎵╁睍
**鍙樻洿璇︽儏**:
- 璐﹀彿鎼滅储浠庝粎鍖归厤 `name` 鎵╁睍涓哄悓鏃跺尮閰?`credentials.email` 鍜?`extra.email_address`锛堜娇鐢?sqljson.StringContains锛?

## [2026-04-12] fix: Antigravity refresh_token 鏈繚瀛樺鑷磋处鍙蜂笉鍙皟搴?

**褰卞搷鑼冨洿**: backend/internal/service/antigravity_oauth_service.go
**涓婃父鍏煎鎬?*: 浣庨闄╋紝鍥炲～閫昏緫
**鍙樻洿璇︽儏**:
- `ValidateRefreshToken` 鍒锋柊鍚?Google 涓嶈繑鍥炴柊 refresh_token锛屽鑷村瓨鍏?credentials 涓虹┖
- 鏂板鍥炲～閫昏緫锛氬鏋滃埛鏂板搷搴斾腑 refresh_token 涓虹┖锛屼娇鐢ㄧ敤鎴蜂紶鍏ョ殑鍘熷鍊?

## [2026-04-12] feat: 鎵归噺瀵煎叆鏀寔浣跨敤閭浣滀负璐﹀彿鍚嶇О

**褰卞搷鑼冨洿**: frontend/src/components/account/CreateAccountModal.vue, frontend/src/i18n/locales/zh.ts, en.ts
**涓婃父鍏煎鎬?*: 浣庨闄╋紝鏂板 UI 閫夐」
**鍙樻洿璇︽儏**:
- 鏂板 `useEmailAsName` 閫夐」锛屼粎 Antigravity 骞冲彴鍙
- 鍕鹃€夊悗闅愯棌鍚嶇О杈撳叆妗嗭紝鎵归噺鍜屽崟涓?OAuth 鍒涘缓鍧囦娇鐢ㄩ偖绠变綔涓哄悕绉?
## [2026-07-06] fix: Preserve explicit OpenAI Images response_format

**Affected files**: `backend/internal/service/openai_images.go`, `backend/internal/service/openai_images_test.go`
**Compatibility**: Low risk. API-key image forwarding still defaults missing `response_format` to `url`, but explicit downstream values such as `b64_json` are no longer overwritten.
**Details**:
- JSON image requests now add `response_format=url` only when the downstream request omits `response_format`.
- Multipart image requests now preserve an explicit `response_format` field and only append `url` when the field is absent.
- Updated OpenAI Images tests to cover explicit `b64_json` preservation and multipart defaulting behavior.

## [2026-07-08] fix: Do not default missing OpenAI Images response_format

**Affected files**: `backend/internal/service/openai_images.go`, `backend/internal/service/openai_images_test.go`
**Compatibility**: Medium risk. Downstream requests that omit `response_format` now follow the upstream default instead of forcing URL responses, reducing compatibility failures with upstreams that reject the parameter.
**Details**:
- JSON image requests now rewrite only the model when `response_format` is absent.
- Multipart image requests now preserve explicit `response_format` fields but no longer append one when absent.
- Updated OpenAI Images tests to assert omitted `response_format` remains omitted through the API-key forwarding path.

## [2026-07-09] fix: Stabilize high-concurrency image monitor manual polling

**Affected files**: `frontend/src/api/admin/imageChannelMonitor.ts`, `frontend/src/views/admin/ImageChannelMonitorView.vue`, `backend/internal/handler/admin/image_channel_monitor_handler.go`, `backend/internal/handler/admin/image_channel_monitor_handler_test.go`, `docs/dev/codebase/image-channel-monitor.md`
**Compatibility**: Low risk. Adds a metadata-only status option and longer manual-test request timeout without changing the default admin UI image preview behavior.
**Details**:
- Added `include_image_data=false` support for manual-run status polling so the backend can omit the large `returned_image_data` field while preserving URLs and timing metadata.
- Manual test launch/status API calls now use a timeout derived from the selected monitor timeout instead of the shared 30s Axios default.
- Added a handler regression test for omitting inline manual result image data.

## [2026-07-09] fix: Restore manual image previews and show actual return mode

**Affected files**: `frontend/src/views/admin/ImageChannelMonitorView.vue`, `frontend/src/i18n/locales/zh.ts`, `frontend/src/i18n/locales/en.ts`, `docs/dev/codebase/image-channel-monitor.md`
**Compatibility**: Low risk. The manual-test UI again requests image data for completed records so generated images are visible immediately; request `response_format` remains user-selected and is not forced.
**Details**:
- Restored completed manual status polling to include returned image data, fixing high-concurrency batches where `b64_json` or downloaded-image previews had no visible image source.
- Added an actual-return column and detail metric that distinguishes URL, `b64_json`, mixed URL+`b64_json`, and data URLs carried in the `url` field.
- Compactly displays `data:` image URLs in network details so an inline URL payload is visible without flooding the dialog with base64 text.

## [2026-07-10] fix: Map OpenAI GPT-5.6 cache write usage

**Affected files**: `backend/internal/service/openai_gateway_service.go`, `backend/internal/service/openai_usage_tokens.go`, `backend/internal/service/display_token_rewrite.go`, `backend/internal/service/openai_gateway_messages.go`, `backend/internal/service/openai_gateway_chat_completions.go`, `backend/internal/pkg/apicompat/types.go`, `backend/internal/pkg/apicompat/responses_to_chatcompletions.go`, `backend/internal/pkg/apicompat/chatcompletions_responses_bridge.go`, `backend/internal/service/openai_embeddings.go`, `backend/internal/service/openai_ws_v2/passthrough_relay.go`, `backend/internal/service/billing_service.go`, `backend/internal/service/pricing_service.go`, `backend/internal/service/openai_codex_transform.go`, `backend/internal/service/openai_model_alias.go`, `backend/resources/model-pricing/model_prices_and_context_window.json`
**Compatibility**: Low risk. Adds official OpenAI `cache_write_tokens` parsing as a compatibility alias for local cache creation accounting, updates GPT-5.6 cache write pricing to the documented 1.25x input rate, and prevents cache-write tokens from being billed/displayed as ordinary input tokens.
**Details**:
- OpenAI HTTP/SSE, embeddings, and WS passthrough usage parsing now maps `cache_write_tokens` from top-level or token-details usage objects into local `cache_creation_tokens`.
- OpenAI usage recording now treats cache-write tokens as a prompt/input component and subtracts them from ordinary input tokens before billing.
- Display-token rewriting now scales official `cache_write_tokens` in Responses, Chat Completions, and usage-map shapes, while recomputing displayed `input_tokens`/`total_tokens` from uncached input + cache read + cache write components.
- Responses-to-Chat and Chat-to-Responses compatibility structs/converters now preserve `cache_write_tokens`, so serialized streaming conversions do not drop cache-write details.
- GPT-5.6 Sol/Terra/Luna pricing now includes `cache_creation_input_token_cost=6.25e-6`, with fallback policy filling missing dynamic entries from `input_price * 1.25`.
- Bare `gpt-5.6` now normalizes as its own GPT-5.6 family model for backend billing/fallback logic instead of falling through to the older GPT-5.4 family.
- Priority service-tier cache-write cost now scales with the priority input-token price instead of staying at the base cache-write rate.
- Added targeted regression coverage for official cache-write fields, display-token amplification, ordinary input-token deduction, and GPT-5.6 cache creation pricing.

## [2026-07-10] fix: Preserve new GPT-5.6 models in OpenAI `/v1/models`

**Affected files**: `backend/internal/service/models_list_policy.go`, `backend/internal/service/models_list_policy_test.go`, `backend/internal/handler/gateway_handler.go`, `backend/internal/handler/gateway_models_list_test.go`, `docs/dev/codebase/gateway.md`
**Compatibility**: Low risk. OpenAI groups with intentionally narrowed custom `/v1/models` lists remain narrowed; stale full-default OpenAI lists are upgraded at runtime so Codex can discover newly curated GPT-5.6 models.
**Details**:
- Added `ExpandGatewayModelDiscoveryCustomList` to recognize the legacy full OpenAI discovery list (`gpt-5.5`, `gpt-5.4`, `gpt-5.4-mini`) and expand it to the current curated list including `gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna`.
- `GatewayHandler.Models` now applies this compatibility expansion before filtering curated OpenAI discovery IDs with a group custom models list.
- Added regression coverage for the stale full-list upgrade while keeping intentionally narrowed custom lists narrow.

## [2026-07-10] fix: Add Codex metadata to OpenAI `/v1/models`

**Affected files**: `backend/internal/handler/gateway_handler.go`, `backend/internal/handler/gateway_models_list_test.go`, `docs/dev/codebase/gateway.md`
**Compatibility**: Low risk. The OpenAI-compatible list keeps the standard `id/object/created/owned_by` model fields and adds optional Codex client discovery metadata only.
**Details**:
- OpenAI `/v1/models` entries now include `supported_endpoint_types`, `supported_session_modes`, `actual_model_returned`, `input_modalities`, `output_modalities`, and `supported_modalities`, matching the metadata shape Codex-style custom provider model pickers use to recognize Responses and Chat Completions support.
- The metadata is presentation-only and does not affect model routing, account scheduling, model access checks, billing, or usage recording.
- Added handler regression coverage for the Codex metadata on GPT-5.6 discovery entries.

## [2026-07-10] fix: Make manual image tests reproduce independent real gateway requests

**Affected files**: `backend/internal/service/image_channel_monitor_service.go`, `backend/internal/service/image_channel_monitor_types.go`, `backend/internal/service/image_channel_monitor_manual_core.go`, `backend/internal/service/image_channel_manual_gateway.go`, `backend/internal/service/image_channel_manual_b64_stream.go`, `backend/internal/handler/admin/image_channel_monitor_handler.go`, `backend/internal/handler/openai_images.go`, `backend/internal/handler/openai_gateway_handler.go`, `backend/internal/service/openai_images.go`, `backend/internal/service/openai_images_response_spool.go`, `frontend/src/api/admin/imageChannelMonitor.ts`, `frontend/src/api/client.ts`, `frontend/src/utils/imageChannelManualTest.ts`, `frontend/src/views/admin/ImageChannelMonitorView.vue`, `deploy/config.example.yaml`, `README_CN.md`, `docs/dev/codebase/image-channel-monitor.md`, `docs/dev/codebase/gateway.md`
**Compatibility**: Medium risk. Manual tests now exercise one complete real gateway request per run and store generated images as short-lived artifacts. Production image response delivery no longer retries generation on another account after a local delivery failure.
**Details**:
- Added `gateway_group`, isolated `gateway_account`, and legacy `direct_probe` execution modes. Concurrent generate/edit runs carry independent request bodies and edit images; `client_run_id` safely deduplicates lost control-response retries within one process.
- Gateway launch recovery reuses the same payload and `client_run_id` across transient `0/408/425/429/5xx` responses until success or user cancellation. Non-idempotent `direct_probe` launches are not replayed. Cancel-all immediately ends the local batch and unlocks the next run while backend cancellation retries continue in the background; late launch responses are still canceled without leaking an older batch into a newer batch.
- Split gateway, delivery, and observation status. Metadata-only polling no longer transports large image data; launch/status/cancel calls use a fixed 15-second control-plane timeout, while artifact transfer keeps the operation-derived timeout. Observation uses the run's captured execution mode: direct probes have a wall-clock deadline, while gateway runs remain observable until a backend terminal/expired result because real requests can chain runtime-configured network, OAuth transport retries, pool-mode retries, and account failovers.
- Stream root `data[]` direct-field `b64_json` and base64 data URLs from the gateway spool into bounded artifact files while preserving real data indexes. HTTP(S) URL delivery uses an isolated SSRF-safe client, safe redirects, context-bounded retry for transport errors/interrupted bodies/408/425/429/5xx, and concurrent per-image downloads.
- Send each edit run as its own multipart binary upload, with a 20 MiB request limit, 1 MiB memory threshold, and temporary-file cleanup. Browser input/output images remain Blobs in IndexedDB and their object URLs are revoked with the view lifecycle.
- Preserve successful artifacts when sibling images fail. The result remains degraded with the failing stage while delivery stays succeeded; the UI downloads the first actual artifact index instead of assuming index 0.
- Retry transient artifact delivery failures with capped exponential backoff until the backend's completion-relative 30-minute retention deadline, including after page refresh; terminal 404/409/410 responses are not retried.
- Reject diagnostic API keys with IP ACLs because loopback gateway requests cannot reproduce the external caller IP.
- Classify local image response spool failures and oversized generated responses as local delivery failures: return a clear 500, do not switch accounts or regenerate/rebill, and do not penalize the healthy upstream account. Client-canceled image requests also skip account failure reporting. Genuine upstream body interruption remains failover-eligible before downstream commit.
- Raised the deployment example response limit from 8 MiB to the code default of 128 MiB and documented the 8 MiB memory-to-disk spool threshold. Added a config regression test to prevent the example from overriding the image-safe default, and clean orphaned spool/artifact files older than their retention window.
- Added regression coverage for `c20` independent launch orchestration, simultaneous same-`client_run_id` deduplication, immediate local cancel while control retries continue, late launch cancellation, client-cancel account health, IndexedDB recovery, and per-run Blob URL cleanup.
- Verification: `go test ./... -count=1`, the targeted service `-race` suite, `pnpm run test:run` (109 files / 670 tests), `pnpm run typecheck`, `pnpm run lint:check`, a production Vite build to a temporary output directory, and targeted frontend utility coverage (93.98% lines / 82.22% branches / 100% functions) all passed. The repository-managed local stack reported backend/frontend/PostgreSQL/Redis ready, and both `/health` and `/admin/channels/image-monitor` returned HTTP 200.

## [2026-07-10] fix: Recover Claude-GPT compact requests from empty replies

**Affected files**: `backend/internal/service/openai_gateway_messages.go`, `backend/internal/service/openai_gateway_messages_compact.go`, `backend/internal/service/openai_gateway_messages_compact_test.go`, `backend/internal/service/openai_gateway_messages_empty_output_test.go`, `backend/internal/service/account.go`, `backend/internal/service/openai_messages_continuation.go`, `backend/internal/service/openai_model_mapping.go`, `backend/internal/service/openai_gateway_service.go`, `backend/internal/handler/openai_gateway_handler.go`, `backend/internal/handler/openai_gateway_handler_test.go`, `backend/internal/pkg/apicompat/responses_to_anthropic.go`, `backend/internal/pkg/apicompat/anthropic_responses_test.go`, `docs/dev/codebase/gateway.md`, `memory/2026-07-10-claude-gpt-empty-replies-debug-report.md`
**Compatibility**: Medium risk. The change intentionally delays Anthropic SSE preamble/thinking until visible output so failed attempts remain eligible for account failover. Normal successful content is preserved, while compact recovery may issue bounded additional upstream summary requests.
**Details**:
- Identified and repaired one long-context Claude Code empty-output failure mode: the upstream can return HTTP/SSE context overflow, `response.failed`, incomplete/no-terminal output, or reasoning without visible text. A later manual compact succeeded despite adjacent `count_tokens` 503 responses, so compact is no longer treated as the universal or latest-timeout root cause; see the follow-up investigation entry below.
- Buffered non-visible Anthropic stream events and stopped converting terminal failures into normal `message_stop/end_turn`, preserving account failover before any visible response is written.
- Replayed terminal `response.output` text and tool arguments when deltas were absent, while ignoring stale tool-argument deltas from an earlier output index.
- Preserved the full pre-guard Anthropic transcript for compact recovery, including API-key requests normally limited by the 12-message replay guard.
- Added bounded chunk summarization, recursive split-on-overflow, hierarchical merge, emergency-summary fallback, complete retry usage accumulation, and stateless recovery headers/continuation handling.
- Added compact-only model mapping and configurable fallbacks, including a default Spark-to-`gpt-5.4-mini` fallback that can be explicitly disabled with an empty list.
- Added standard pings during compact header waits and pre-visible Messages body silence, using a resettable idle timer while keeping transport state separate from semantic output; final failures after a ping use Anthropic SSE `event: error`, and client disconnect cancels detached recovery work without penalizing the account.
- Restored the complete Anthropic SSE lifecycle when visible text exists only in terminal `response.output`, including a synthesized `message_start` before the replayed content.
- Marked successful/error Anthropic responses terminal so panic and generic error fallbacks cannot append a duplicate event after `message_stop` or a prior error.
- Added regression coverage for HTTP/SSE overflow, reasoning-only/empty terminal output, full-history preservation, split budgets, merge shrinking, usage accounting, stateless headers, cancellation, pre-visible keepalive failover, terminal text/tool reconstruction, duplicate-terminal suppression, and post-ping SSE errors.

## [2026-07-10] docs: record Claude-GPT intermittent timeout investigation and repair design

**Affected files**: `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md`, `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md`, `docs/dev/codebase/README.md`, `docs/dev/codebase/gateway.md`, `docs/dev/CHANGELOG_CUSTOM.md`
**Compatibility**: Documentation-only. No runtime route, scheduler, account state, count-token behavior, billing, schema, frontend, deployment, or production state changed in this entry.
**Details**:
- Recorded that a manual Claude Code compact completed from `preTokens=256786` to `postTokens=6151` in 98.48 seconds and passed three post-compact canary turns even though adjacent `count_tokens` calls returned 503. This separates the real count-token compatibility gap from the latest timeout root-cause analysis.
- Documented the highest-confidence latest timeout chain: an OpenAI bridge request returns `usage_limit_reached` 429, the only bridge account enters cooldown, boolean preflight misclassifies temporary unavailability as no bridge, the retry falls into an empty native Antigravity pool, and Claude Code eventually reports a generic operation timeout.
- Compared the fork against official `upstream/main=e316ebf52838a89d57fc790981cce7520f819ac8` and release `v0.1.151`: official count-token, response.failed, transport failover, missing-terminal, and application-error work is reusable, but official upstream has no Antigravity account-side Claude-GPT bridge and therefore no direct strict-routing fix.
- Specified a P0 structured bridge route decision (`not_configured`, `ready`, `rate_limited`, `unavailable`, `probe_error`) that separates stable mapping intent from transient scheduler state, removes hidden native fallback after bridge intent is established, and returns consistent Anthropic 429/503 semantics with `Retry-After`.
- Specified a separate P1 adaptation of official `/v1/responses/input_tokens` and OAuth/local-tokenizer fallback for bridge-aware `count_tokens`, with no usage, billing, concurrency, or native-pool side effects.
- Added the planned file map, two-request 429 regression, broader test matrix, observability fields, canary rollout, rollback, acceptance criteria, and ordered next-session implementation checklist.
## [2026-07-11] feat: Restore revoked subscriptions without widening billing queries

**Affected files**: `backend/internal/{repository/user_subscription_repo.go,repository/billing_cache.go,service/{subscription_service,user_subscription,user_subscription_port,billing_cache_service}.go,handler/admin/subscription_handler.go,handler/dto/{types,mappers}.go,server/routes/admin.go}`, focused backend tests, `frontend/src/{api/admin/subscriptions.ts,views/admin/SubscriptionsView.vue,types/index.ts,i18n/locales/{zh,en}.ts}`, and subscription/upstream-sync docs.

**Compatibility**: Medium risk, constrained to administrator subscription management. User subscription APIs and billing/quota eligibility retain the normal soft-delete scope. No schema, migration, stored billing, `actual_cost`, display token/cost, cache-read token, distribution, bundle, payment, bridge, Images, scheduler, or deployment behavior changed.

**Details**:

- Fixed revoke to produce admin-visible soft-deleted history and added explicit POST revoke plus restore endpoints while retaining DELETE revoke compatibility.
- Added revoked timestamps/status mapping, administrator all-status/revoked filtering and detail visibility, bilingual restore UI, and API route tests.
- Added fresh-read/conflict checks and an atomic conditional restore; expired formerly-active subscriptions restore as expired, and migration `016` remains the final concurrent uniqueness guard.
- Made local L1 and Redis billing-cache invalidation synchronous after revoke/restore, added cross-instance invalidation, and bound its Redis subscriber to service shutdown.
- Preserved the fork-local subscription quota adjustment UI and the already-integrated expired-assignment reactivation path.

## [2026-07-11] feat: Add Grok admin frontend and media pricing reachability

**Affected files**: `frontend/src/{api/admin/{grok,index}.ts,composables/useGrokOAuth.ts,components/account/{CreateAccountModal,EditAccountModal,OAuthAuthorizationFlow,AccountUsageCell,GrokQuotaProbeCell}.vue,components/admin/account/{AccountTableFilters,ReAuthAccountModal}.vue,components/common/{PlatformIcon,PlatformTypeBadge,GroupBadge}.vue,views/admin/{GroupsView.vue,groupsMediaPricing.ts},types/index.ts,utils/platformColors.ts,i18n/locales/{zh,en}.ts}` and focused frontend tests; `docs/dev/codebase/account.md`
**Upstream compatibility**: Medium risk. Manually reconciles Grok management reachability and the latest media rate-card semantics with the fork-local account/group forms; it does not replace the fork's monolithic locales or curated-model and billing/display customizations.
**Details**:
- Added Grok OAuth/API-key create and edit flows, OAuth reauthorization, admin account filtering/presentation, and an explicit OAuth quota probe using the current `/api/v1/admin/grok-oauth/*` route contract.
- Added Grok group selection plus image default-price hints and independent per-second video pricing controls (`video_rate_independent`, `video_rate_multiplier`, `video_price_480p`, `video_price_720p`, `video_price_1080p`). Current default hints are `$0.02` per image and `$0.05/s`, `$0.07/s`, `$0.25/s` for 480p/720p/1080p video.
- Preserved the existing OpenAI Images endpoint toggle, Claude-GPT bridge controls, curated model-list behavior, account scheduling/failover surfaces, and single-file Chinese/English locale layout.
- Added focused regression tests for Grok management reachability, OAuth error handling, account credential/mapping persistence, fork-local controls, and media pricing defaults.

## [2026-07-11] test: expand upstream-sync protection for fork-local contracts

**Affected files**: `backend/tools/upstream-sync-guard/main.go`, `backend/tools/upstream-sync-guard/main_test.go`, `docs/dev/CHANGELOG_CUSTOM.md`, `docs/dev/UPSTREAM_SYNC.md`
**Compatibility**: Guard/test-only. No product implementation, runtime route, schema, migration, billing, frontend behavior, push, or deployment changed.
**Details**:
- Expanded protected-path coverage for ImageChannelMonitor, bundle subscription `member_group_ids`, OpenAI Images endpoint controls, long-context usage snapshots, model-pricing row/provider/billing-object/hidden-model configuration, and 5m/1h cache pricing.
- Added critical signatures for the ImageChannelMonitor schema/routes/manual lifecycle/browser artifact recovery, subscription-plan and payment-order group snapshots, usage-log persistence, model-pricing contracts, and real/display cache-tier fields.
- Added conditional signatures for bridge-aware `count_tokens`: the guard accepts the `bf5825074` baseline where the dedicated files do not exist, then permanently enforces their bridge routing, local-estimate fallback, and no-native-fallback signatures once the later `b06190970` implementation becomes an ancestor of the alignment branch.
- Added guard self-tests that verify representative fork-local paths remain protected and every currently applicable signature matches the checkout.

## [2026-07-11] fix: Allow independent Sub2API frontend/backend control in dev control

**Affected files**: `dev-services.yml`, `scripts/dev-stack.ps1`, `DEV_GUIDE.md`
**Compatibility**: Low risk. Existing command-line whole-stack actions are unchanged; the new foreground component action is used by dev control only.
**Details**:
- Registered the Sub2API backend and frontend as separate managed services instead of monitor-only entries, so each now has start, stop, and restart controls.
- Added `dev-stack.ps1 run -Component backend|frontend` to keep each process tree attached to the dev control runner while continuing to enforce repository startup and port rules.
- Run the dev-control-managed backend with `GIN_MODE=release` so route-table debug output does not delay runner process tracking during startup; Air hot reload remains enabled.
- Removed the duplicate aggregate managed service from the manifest; dev control project-level actions still operate the backend and frontend together without competing for the same ports.
- Documented the dev control-specific foreground commands and retained the existing whole-stack CLI workflow.

 ## [2026-07-11] feat: Add user platform USD quotas without changing billing semantics

**Affected files**: `backend/ent/schema/user_platform_quota.go`, `backend/migrations/162_user_platform_quotas.sql`, `backend/migrations/180_allow_grok_user_platform_quota.sql`, `backend/internal/repository/user_platform_quota_repo.go`, `backend/internal/repository/billing_cache.go`, `backend/internal/service/user_platform_quota_port.go`, `backend/internal/service/billing_cache_service.go`, `backend/internal/service/user_platform_quota_flusher.go`, `backend/internal/service/auth_service.go`, `backend/internal/service/setting_service.go`, `backend/internal/handler/user_platform_quota.go`, `backend/internal/handler/admin/user_platform_quota.go`, `frontend/src/components/admin/user/UserPlatformQuotaModal.vue`, `frontend/src/components/user/UserPlatformQuotaCell.vue`, `frontend/src/views/admin/UsersView.vue`, `frontend/src/views/admin/SettingsView.vue`, `frontend/src/views/user/DashboardView.vue`

**Compatibility**: Medium risk, isolated behind per-user configured limits. Existing users have no quota records unless configured. Subscription-mode requests remain outside this balance-mode quota. Stored billing, quota deduction, `actual_cost`, display-token transforms, user/channel/global pricing, curated model lists, account scheduling, and Claude-GPT bridge routing are unchanged.

**Details**:
- Added daily, weekly, and rolling-30-day USD limits per user and platform for Anthropic, OpenAI, Gemini, Antigravity, and Grok, with additive migrations and an Ent schema.
- Added Redis eligibility caching, short-lived no-record sentinels, atomic usage accumulation, dirty-key persistence, and a database flusher. Database lookup remains the fallback when Redis is unavailable.
- Enforced limits before standard balance-mode requests and accumulated the final charged cost after billing. The quota path consumes billing output; it does not recalculate model prices or rewrite usage/display tokens.
- Preserved forced-platform attribution for bridge and compatibility routes so Claude-GPT and OpenAI image requests are charged to the selected platform rather than inferred from model text.
- Added user/admin APIs, admin per-user editing and window reset, dashboard usage display, system registration defaults, and per-auth-source overrides for the four locally supported auth sources.
 - Added Grok to the platform constraint through migration `180`; historical migration `162` remains unchanged.
 - Verification: focused Go package tests, tagged quota unit tests, Ent generation, frontend typecheck, 46 focused Vitest tests, and production frontend build passed.

## [2026-07-11] feat: Align OpenAI/Codex compatibility through upstream 0.1.151

**Affected files**: `backend/internal/pkg/apicompat/*`, `backend/internal/pkg/openai/request.go`, `backend/internal/pkg/ctxkey/ctxkey.go`, `backend/internal/service/openai_*`, `backend/internal/service/{account_test_service,account_usage_service,setting_service,settings_view}.go`, `backend/internal/server/middleware/api_key_auth.go`, `backend/internal/handler/{dto/settings.go,admin/admin_helpers_test.go}`, `frontend/src/{api/admin/settings.ts,views/admin/SettingsView.vue,i18n/locales/en.ts,i18n/locales/zh.ts}`
**Compatibility**: Medium risk. Manually ports the OpenAI/Codex protocol deltas from `upstream/main@e316ebf52838` without replacing fork-local gateway, billing, model discovery, Images, or Claude-GPT bridge code.
**Details**:
- Preserved custom/freeform tools through Responses-to-Chat fallback and added client-executed `tool_search`, namespace child flatten/restore, collision rejection, and valid `tool_choice` filtering.
- Paired outbound OAuth Codex `originator` with the final User-Agent across HTTP, passthrough, WebSocket, quota probes, and account tests; raised the fallback identity to the upstream minimum `0.144.0`. The Messages compatibility bridge remains a no-originator path.
- Added user-scoped Fast/Flex rules sourced only from the authenticated API-key owner context. User-specific rules precede global rules while the fork-local default priority-filter rule remains unchanged. Added explicit `force_priority`, validation, DTO, and admin UI support.
- Added top-level `cache_creation_input_tokens` compatibility while preserving the existing nested `cache_write_tokens` representation. Conversion selects one cache-creation value rather than summing aliases, so real billing and display-token accounting remain unchanged.
- Added RED/GREEN contract coverage for custom/tool-search/namespace conversion, paired Codex identity, authenticated user-scoped Fast/Flex forwarding, and cache-creation streaming/non-streaming round trips.
- Verified the focused backend packages and the complete `internal/service` package, then passed frontend typecheck, lint, all 109 Vitest files (670 tests), and the production build. `upstream-sync-guard` and `git diff --check` also passed.
## [2026-07-11] feat: Add upstream Batch Image workflow without replacing fork image or billing paths

**Affected files**: `backend/ent/schema/batch_image_*.go`, `backend/migrations/184_batch_image_workflow.sql`, `backend/internal/{handler,repository,service}/batch_image*`, `backend/internal/server/routes/gateway.go`, `backend/internal/service/{group,admin_service,usage_billing}.go`, `frontend/src/{api,composables,views}/**/*BatchImage*`, `frontend/src/views/admin/GroupsView.vue`, `docs/BATCH_IMAGE_MVP.md`, `docs/dev/codebase/batch-image.md`

**Compatibility**: Medium risk, disabled by default. The feature requires both global and queue configuration plus an eligible Gemini group. Existing OpenAI Images, ImageChannelMonitor, ordinary billing/display-token accounting, Claude-GPT bridge, Grok routing, curated models, account scheduling, and platform quotas remain on their existing paths.

**Details**:
- Manually adapted the upstream Batch Image chain through `upstream/main@e316ebf52838` instead of cherry-picking over fork hot paths. Added Gemini API and optional Vertex providers, an idempotent Redis worker, result indexing/download/cleanup, bounded failure recovery, and user/admin UI.
- Added one additive migration at local sequence `184` for jobs/items/events, group gates/multipliers, and `users.frozen_balance`; no historical migration was modified.
- Added immutable per-job pricing snapshots and idempotent reserve/capture/release operations. Only successful images are captured, failed or cancelled work releases unused holds, and ordinary usage billing keeps its original deduction semantics.
- Added authenticated, owner-scoped routes under `/v1/images/batches`, route reachability coverage, group/global permission tests, end-to-end provider/settlement/download smoke coverage, settlement failure/recovery tests, and frontend access-gate tests.
- Documented the preservation boundary and rollout defaults in `docs/dev/codebase/batch-image.md`.

## [2026-07-11] feat: Align payment, redeem, and affiliate behavior without removing distribution

**Affected files**: payment providers/services/handlers/frontend, redeem services/admin UI, affiliate repository/admin UI, migration `185`, and payment/redeem/distribution/affiliate module docs.
**Compatibility**: Medium risk, protected by focused backend/frontend tests. Distribution and bundle subscription contracts are intentionally retained.
**Details**:
- Added Airwallex, currency-aware amount handling, pending-refund finalization, stale fulfillment lease recovery, provider response hardening, and custom EasyPay methods.
- Payment and subscription confirmation totals now format with the selected provider currency instead of a hard-coded CNY symbol.
- Added redeem expiration enforcement, restricted batch update, balance-redeem affiliate accrual, and pre-transaction invitation validation while retaining local batch-per-user rules.
- Added admin affiliate invite/rebate/transfer records, exact payment-order audit linkage, transfer snapshots, and matured frozen quota in overview. The additive schema change is migration `185`.
- Added opt-in subscription USD-to-CNY conversion with a default-off compatibility lock and admin plan charge preview.
- Rejected upstream distribution deletion and retained the fork-local RMB wallet, ledger, assets, API-key lifecycle, routes, UI, and settings. Retained bundle `member_group_ids`, per-group fulfillment idempotency, local `CreditAmount`, first-recharge bonuses, and forced WeChat Native QR fallback.

## [2026-07-11] fix: Harden redeem, subscription-window, and fulfillment concurrency

**Affected files**: `backend/internal/repository/{user_repo,user_subscription_repo}.go`, `backend/internal/service/{redeem_service,subscription_service,user_subscription_port,payment_fulfillment}.go`, API-key middleware and focused tests; `docs/dev/{UPSTREAM_SYNC,CHANGELOG_CUSTOM}.md`, `docs/dev/codebase/{redeem,payment}.md`

**Compatibility**: Targeted manual adaptation of upstream `fc66a30ff`. It does
not replace fork-local payment bundles, affiliate handling, billing/display
transforms, media frozen-balance settlement, or platform quotas.

**Details**:
- Negative balance/concurrency redemption now applies an atomic database floor
  at zero instead of reading and clamping stale user values in memory.
- Expired subscription windows use compare-and-set on the observed window start.
  API-key middleware completes maintenance synchronously, reloads the database
  snapshot, and rechecks limits before authorizing the request.
- Payment bundle member assignment and its per-group audit commit in one outer
  transaction; L1/Redis cache invalidation occurs after commit and is retried
  for already-audited groups. Subscription redemption uses the same deferred
  post-commit cache rule.
- Existing stale fulfillment lease/takeover behavior was audited and left
  unchanged because it was already present from the earlier alignment batch.
- Verified focused RED/GREEN regressions, all backend unit tests, and targeted
  race tests. No frontend, migration, generated Ent, push, or deployment change.
## [2026-07-11] feat: Add persistent group table column preferences and used quota

**Affected files**: `frontend/src/views/admin/GroupsView.vue`, `frontend/src/views/admin/GroupsView.columnSettings.spec.ts`, `frontend/src/i18n/locales/en.ts`, `frontend/src/i18n/locales/zh.ts`

**Compatibility**: Low risk, frontend-only. Name and actions remain fixed, persisted hidden keys are validated, and hiding all consumers suppresses the corresponding summary request.

**Details**:
- Added per-browser group table column visibility preferences with a compact column menu.
- Added an independent used-quota column backed by the existing 30-day `total_cost` group summary.
- The UI does not derive prices from tokens or recalculate billing; stored cost, `actual_cost`, display-token transforms, cache-read quantities, subscription quota, and capacity calculations are unchanged.
- Added a static regression contract for fixed columns, persisted-key validation, used-quota source, and conditional summary loading.
## [2026-07-11] fix: Align gateway protocol conversion and bounded request parsing

**Affected files**: `backend/internal/pkg/apicompat/*`, `backend/internal/pkg/httputil/body.go`, gateway handlers, `backend/internal/service/gateway_{request,service,websearch_block_filter}.go`, and focused tests.

**Compatibility**: Medium risk, adapted from `178550987`, `ad8afc8a2`, `867616fca`, `40c563c4a`, and `53a5c45bd` without replacing fork-local gateway, billing, Images, or scheduler paths.

**Details**:
- Responses-to-Anthropic now combines top-level instructions with system/developer input in order; Chat/Responses fallback preserves explicit `parallel_tool_calls` true and false.
- Replayed web-search blocks are removed only from the forwarded copy when locally emulated or incompatible with the mapped third-party model; ordinary and genuine official Anthropic history remains byte-identical.
- Gateway JSON reading tolerates raw control bytes inside strings and a UTF-8 BOM while enforcing the existing normalized body limit. Structurally invalid JSON remains invalid.
- Parse diagnostics contain only error type, body length, and syntax offset. Unlike upstream, this fork intentionally does not log request body head/tail or user prompt content.
- Stored billing, `actual_cost`, display/cache-read transforms, Claude-GPT routing, OpenAI Images, Batch Image, Grok media, model selection, and account scheduling are unchanged.

 ## [2026-07-11] feat: Import Codex session accounts without weakening fork account contracts

**Affected files**: `backend/internal/handler/admin/account_codex_import*.go`, `backend/internal/server/routes/{admin.go,admin_codex_session_import_contract_test.go}`, `backend/internal/service/{openai_token_provider.go,token_refresher.go}` and focused tests; `frontend/src/{api/admin/accounts.ts,components/admin/account/CodexSessionImportModal.vue,views/admin/AccountsView.vue,types/index.ts,i18n/locales/{zh,en}.ts,__tests__/integration/codex-session-import.spec.ts}`; `docs/dev/{UPSTREAM_SYNC.md,codebase/account.md}`

**Compatibility**: Medium risk, selectively adapted from upstream `fda1ed459`, `f788e6bdb`, `32df33a1c`, `a5638a4e5`, and `6bd248fd1`. No migration. Existing PAT creation/security, account proxy/group bindings, scheduling/failover, credential persistence, Claude-GPT bridge, OpenAI Images, billing/display/cache-read invariants, public settings, curated models, and unrelated Vertex behavior remain unchanged.

**Details**:
- Added idempotent admin `POST /api/v1/admin/accounts/import/codex-session` parsing raw access tokens, Codex auth JSON, JSON arrays/streams, and mixed line input.
- Complete sessions prefer `chatgpt_user_id`, reject cross-user matches inside a shared `chatgpt_account_id`, and retain account-id fallback for legacy rows missing user identity. Access-only sessions use only an access-token SHA-256 fingerprint, so shared workspace/user metadata cannot silently merge separate credentials.
- Existing refresh/client/id-token fields survive an access-only update; imported credential extras cannot overwrite protected OAuth identity/token fields. Token cache invalidation follows successful account updates.
- Access-only OAuth accounts never enter the refresh path. A still-valid token remains usable; an expired token reports the missing refresh token explicitly. Existing Codex PAT accounts retain their separate non-refreshing classification.
 - Added a standalone bilingual account-page dialog that preserves fork proxy/group, concurrency, priority, billing-rate, load-factor, default-group, and update-existing controls without rewriting the existing OAuth/PAT creation flows.
 - Added parser, expiry, identity, access-only, credential-preservation, handler, route, frontend API/UI, account-page regression, typecheck, and lint verification.

 ## [2026-07-11] fix: Align account pagination, user model stats, and OpenAI model sync

**Affected files**: `backend/internal/repository/{account_repo.go,account_repo_integration_test.go,usage_log_repo.go,usage_log_repo_request_type_test.go}`, `backend/internal/service/{upstream_models.go,openai_models_url_test.go}`, `docs/dev/{UPSTREAM_SYNC.md,CHANGELOG_CUSTOM.md}`, `docs/dev/codebase/data-consistency.md`

**Compatibility**: Low-risk selective adaptation of upstream `fd004bdd8`,
`e236bff1e`, and `f881ff7cb`. No schema, migration, generated Ent, frontend,
route, setting, push, or deployment change.

**Details**:
- Clone the mutable Ent account query before `Count`, keeping pagination totals
  and returned items under the same effective predicates.
- Aggregate user model summaries by requested model through the existing
  source-aware query. Preserve direct sums of token fields, `total_cost`,
  `actual_cost`, and account cost; no display or billing transform changed.
- Build OpenAI model-discovery URLs through the shared version-aware endpoint
  helper, so `/v2`, `/v4`, and similar bases retain their version path.
- Added RED/GREEN regressions for the pagination invariant, requested-model
  grouping and cost/cache columns, and non-v1 model URLs.
 - Preserved fork-local pricing/display invariants, curated/default models,
   Claude-GPT bridge, OpenAI Images, Batch Image, Grok media, platform quotas,
   scheduler/failover, ops logging, settings, i18n, and routes.

## [2026-07-11] fix: Complete Grok image-model and account-usage surfaces

**Affected files**: `backend/internal/service/openai_images*.go`, `frontend/src/components/account/AccountUsageCell.vue`, `frontend/src/components/account/__tests__/AccountUsageCell.spec.ts`, `frontend/src/composables/__tests__/useGrokOAuth.spec.ts`, `frontend/src/types/index.ts`, `frontend/src/i18n/locales/{en,zh}.ts`

**Compatibility**: Low-risk selective adaptation of the still-missing parts of upstream `b480545c1`. Existing Grok quota collection, local usage aggregation, media billing, size sanitization, fixed quota probing, and composer alias handling remain authoritative.

**Details**:
- The OpenAI Images request parser now recognizes `grok-imagine`, `grok-imagine-edit`, and the `grok-imagine-image*` family as native image models while continuing to reject ordinary text models.
- Grok OAuth account cells now consume the existing backend usage DTO and show local requests/tokens, account cost, user cost, request/token quota windows, retry delay, entitlement, status, last probe, and last observed-header time.
- Account and user costs are displayed directly from backend `cost` and `user_cost`; the frontend does not derive prices from token counts or change stored billing, `actual_cost`, quota deductions, cache-read quantities, Grok media multipliers, or scheduling.
- Completed bilingual recovery guidance for every structured Grok OAuth error code emitted by the current backend. The composable already used the shared structured-error extractor, so no duplicate error parser was introduced.
- Added RED/GREEN regressions for image parsing, Grok usage rendering, direct cost fields, over-limit quota percentages, and OAuth structured errors. Focused Go tests, 16 frontend tests, typecheck, affected-file ESLint, and `git diff --check` passed.

## [2026-07-11] feat: Add guarded admin user role management

**Affected files**: admin user handler/service contracts and tests, admin user create/edit API/UI, bilingual role labels, and focused frontend tests.

**Compatibility**: High-sensitivity permission change selectively adapted from the role-owned parts of `64fdc11ec` and `7918b1a9c`. No migration or public registration change.

**Details**:
- Admin-created users may explicitly be `user` or `admin`; omitted roles still default to `user`, and all other values return a typed bad request.
- Service-level guards reject self-demotion and demoting the last remaining administrator, so bypassing the UI/handler cannot remove all admin access.
- Role changes reuse the existing auth-cache invalidation path and emit actor/target/old/new role audit metadata without logging personal data.
- Existing user group rates, platform quotas, default subscriptions, balances, concurrency, billing/display-token behavior, and public registration remain unchanged.

## [2026-07-11] fix: Gate admin scheduler-score calculation by column visibility

**Affected files**: admin account-list handler/API, account-table column persistence, and focused backend/frontend tests.

**Compatibility**: Low-risk performance adaptation of upstream `6ae5fc31b`; scheduler scoring and account selection are unchanged.

**Details**:
- Account-list responses omit scheduler scores by default and enter the expensive OpenAI candidate-pool scoring path only when `include_scheduler_score=1` is explicit.
- The scheduler-score column is hidden by default, including a one-time migration for existing saved layouts; explicitly showing it reloads the current list with score inclusion enabled.
- Preserved fork-local account columns such as `exported_at`, Codex/Spark controls, filters, sorting, selection, and auto-refresh parameter synchronization.
- Added backend default-off and frontend visibility/persistence regressions. Focused Go, five Vitest cases, affected ESLint, and `git diff --check` passed.

## [2026-07-11] feat: Isolate Anthropic Fable limits and bound reset-less 429s

**Affected files**: `backend/internal/service/{ratelimit_service,model_rate_limit,account_usage_service,anthropic_rate_limit_alignment_test}.go`, `frontend/src/components/account/{AccountUsageCell.vue,__tests__/AccountUsageCell.spec.ts}`, `frontend/src/types/index.ts`, and account/upstream-sync documentation.

**Compatibility**: Selectively adapts upstream `3866da508` and `b3f796972` without adding a migration or reviving the removed 429 admin setting.

**Details**:
- Reset-less Anthropic 429s use a fixed five-second account cooldown.
- Rejected `7d_oi` windows limit only the Fable family and keep Sonnet/Opus schedulable; the existing 5h/7d whole-account behavior is unchanged.
- Fable utilization/reset is cached in account extra, returned in `UsageInfo`, and conditionally displayed as `7d F`.
- Stored billing, quota deduction, `actual_cost`, display prices/tokens, real cache-read quantities, Spark shadow isolation, advanced scheduler/failover, Claude-GPT bridge, Images, curated/default models, Ops, settings, routes, and bilingual locale files are unchanged.

## [2026-07-11] feat: Show stats-only API-key concurrency

**Affected files**: concurrency Redis/service, shared gateway helper, OpenAI Responses WebSocket, API-key service/DTO/Wire, user key table/i18n/types, and focused tests.

**Compatibility**: High-sensitivity selective adaptation of upstream `089a7b7fa`; no schema or migration.

**Details**:
- Tracks each API key in an independent `concurrency:api_key:*` sorted set after the existing user slot succeeds. This is observation only and never gates admission or changes user/account limits.
- Shared Claude/OpenAI Chat/Responses/Gemini paths use the existing user-slot helper; Responses WebSocket tracks each active turn explicitly. Release functions remove both user and API-key stats slots on every registered exit path.
- Redis tracking/count errors fail open and render zero instead of failing requests or key management.
- API-key list/detail responses and the persisted key table expose current concurrency while retaining latest-use IP, quota/group filters, and existing columns. Billing, display tokens, cache-read quantities, `actual_cost`, scheduler/failover, Images/Batch Image, and routes are unchanged.

## [2026-07-11] fix: Align response.failed and committed-stream Ops semantics

**Affected files**: OpenAI gateway native/passthrough/Chat/Messages services, Ops upstream context, gateway handlers, error logger, focused tests, and gateway/Ops module docs.

**Compatibility**: High-risk gateway behavior selectively adapted from `1da3501af`, `8f97953e5`, `7918b1a9c`, and `5aba53d54`. No migration, frontend, route, or setting change.

**Details**:
- HTTP-200 `response.failed` terminals now apply semantic, platform-scoped passthrough rules across native Responses, passthrough, Chat, and Messages.
- Context-window failures remain client errors; transient failures fail over only before output; partial output is never replayed on another account.
- Failed terminals return before successful usage submission. Existing cyber-policy auditing remains intact and display pricing, cache-read quantities, stored billing/`actual_cost`, Images, Batch Image, WebSocket, and scheduler behavior are unchanged.
- Local errors emitted after SSE committed HTTP 200 are recorded once by Ops only when no upstream context already owns the log; intended status drives severity while stored wire status remains 200.
