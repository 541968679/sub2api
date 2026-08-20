# 账号管理 (Account)

> 管理 AI 平台账号（Antigravity/Anthropic/OpenAI/Gemini/Grok），包括 OAuth 导入、批量创建、状态监控、AI Credits 和配额追踪。

## Per-account user schedule

Allow list, deny list, per-user pair concurrency, and optional per-user quality gates are four independent configs on `account_schedule_users` (`allow`, `deny`, `max_concurrency`, plus `quality_*` columns). The leftover `accounts.user_schedule_mode` column is no longer the write/hot-path source of truth. Create-account stays empty. Spark shadows do not inherit parent rules; new shadows start empty.

A user may sit on allow, deny, a cap, and a quality gate at once. Runtime identity priority is deny → allow-whitelist miss → then optional quality gate → explicit pair cap N≥1 → default unrestricted (account + user global concurrency only). `0` / null / omit means no pair cap. A quality gate is enabled when p50 or success rate is set; samples/condition are modifiers. Unconfigured metrics and under-sampled windows are not judged. Live 15-minute stats come from Redis `account-quality:live:{accountID}` (maintenance tick writes; selection reads; cache miss fails open). Manual **立即恢复** grace lives in a separate HASH `account-quality:resume:{accountID}` so the 5-minute `Replace` SCAN can delete stale live keys without wiping the grace. A quality block is admission fail for that pair only: clear sticky and reselect. It must not change `schedulable` / `TempUnschedulableUntil` or fold into `IsSchedulable()`. Restore-default writes four empties. Bulk edit may overwrite allow and/or deny (checkbox per list) and never writes caps or quality gates. Quality-gate **保存模板 / 应用模板** reuse the same site-wide hard-close threshold KV; the template is not bound to a user. **立即恢复** keeps the gate and writes a 15-minute resume-HASH grace so the current window cannot immediately exclude that pair again.

Admin edit/bulk keep the last-column user-schedule zone and reuse `OpenAIFastPolicyUserSelector`. The account list `user_schedule` column is default-visible: all four empty shows `—`; otherwise union chips with allow/deny tags, a quality-state chip (`质量` / `已停` / `已恢复`), plus an inline pair-cap mark (`user_concurrency_patch`) and inline quality patch (`user_quality_gate_patch`). **立即恢复** shows `已恢复` and force-admits the pair. Clicking `已恢复`, or waiting 15 minutes, returns to `质量` and starts a new accumulation window (`quality_window_until`). List `schedulable` stacks the schedule toggle and fallback-only toggle. `concurrency` keeps the inline max editor plus the remaining capacity badges. `quality_ttft` shows p50 and success rate together. Scheduler evaluation is documented in [gateway.md](./gateway.md#account-user-schedule-filter).

## Upstream rate (scheduling overlay)

`accounts.upstream_rate_multiplier` is a scheduling-only field (migration 203). It is not `rate_multiplier` / `BillingRateMultiplier()`. Changing it must not change `actual_cost` or quota.

- Editable for every account type.
- Defaults: `oauth` / `apikey` → `0.15`; `setup-token` / `upstream` / `bedrock` / `service_account` → `1`.
- Create omit → type default. Old Redis snapshots missing the field use the type default, not billing `1.0`.
- Admin surfaces: create/edit/bulk, accounts list inline column, smart-schedule pool inline column, import/export.

Among already-eligible accounts, selection prefers the lowest upstream rate. Same rate falls through to priority / load / last_used / Sub2 score. Sticky pins that still admit are not broken for a cheaper peer. `fallback_only` stays a hard partition; the overlay applies inside the active partition. See [gateway.md](./gateway.md#upstream-rate-overlay).

## User × platform smart schedule

Admin writes this only on the user smart-schedule page (`UsersView` smart-schedule column → `/admin/users/:id/smart-schedule`). The users table column shows enabled platforms and pool counts via `POST /admin/users/smart-schedule/summaries`. In-page chrome is a single row (`返回用户列表` + `pageDescription`); the long title/subtitle block is gone (`AppHeader` still uses `titleKey`). Below that is one `UsersView` list row (`AdminUserListRowTable`: email/id/role/groups/subscriptions/balance/burn-rate/usage/concurrency/smart-schedule/schedule-pnl/TTFT/success/status/last active/last used/created/actions — no username). The actions column matches UsersView (edit / usage / errors / toggle status / more). Header burn rate reuses the UsersView `UserBurnRateCell` and `POST /admin/dashboard/users-burn-rate` (trailing 5m `actual_cost` → `$/h`; do not derive from cost/tokens). First-paint `loading` waits only for `getSmartSchedule` + pool member details (`lite=1` ids list), not the platform candidate page-walk. Candidates load lazily on add-search focus, filtered-add open, or one-click “add scheduling” (those buttons stay clickable at count 0 so they can trigger the load). `loadAll({ pickPlatform: true })` skips the `activePlatform` watch reload. Manual/auto refresh is silent: keep the current DOM, spin the refresh icon, patch pool + visible header extras (quality/usage/concurrency/subscriptions/burn-rate/smart-schedule from drafts); do not set full-page `loading`/`userLoading`. `statsLoading` only spins quality/today cells; the pool DataTable uses virtual scroll and is not skeleton-replaced. Account `GET ?lite=1` skips schedule_users/quality-live hydrate, window-cost N+1, and credential secrets. Pool **编辑** must `GET /admin/accounts/:id` before opening `EditAccountModal`; passing the lite row leaves credentials, extra mappings, and user-schedule empty. The modal only re-hydrates when it opens or the account id changes, so a same-id lite→full swap would not refill the form. Then two stretched side-by-side cards on `lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]` (not a locked ~28rem left chimney): left quality-threshold editor is three compact rows (email + enable + platform tabs; 3-col inline p50/success/samples/condition/cooldown; copy/save), right is pool title + horizontally dense add+filter. The pool table and bulk bar sit full-width under those cards. User TTFT/success use `POST /admin/users/quality-stats/batch`; today/30d usage uses `POST /admin/dashboard/users-usage`. Do not derive prices from cost/tokens. The per-platform enable control is the quality-card switch that PUTs immediately; quality/cooldown/pool-cap edits still use Save. An empty pool cannot be turned on. Removing the last member turns the platform off and persists that disable. Storage is `user_smart_schedule_policies` + `user_smart_schedule_accounts` (migration 202; `sort_order` in 204), not `account_schedule_users`. One policy row per `(user_id, platform)` for `anthropic` / `openai` / `gemini` / `antigravity` / `grok`.

When `enabled=true`, that user×platform is a closed allow-list: pool miss rejects; in-pool ignores the account-side allow/deny/gate/cap for that pair. Quality still reuses `EvaluateAccountQualityHardClose` and the live 15-minute cache. A breach writes pair cooldown `smart-schedule:cooldown:{accountID}` HASH field `u:{userID}` via `HSETNX` (no extend, not `TempUnschedulable`). Pool **切换状态** (`POST /admin/accounts/:id/smart-schedule-resume` `{user_id, state}`) writes Redis first, then DB `paused`: `paused` (stay in pool + clear cooldown/resume), `cooling` (`HSET` overwrite + clear resume), `resumed` (`HDEL` + resume grace; omitted `state` is this), or `selectable` (`HDEL` + start-window `w:`). `SetCooldown` returns HSET errors so a failed cooling write does not unpause. Pause patches the user Redis bundle in place (`ApplyMemberPaused`) instead of only `DEL`. The switch live-state uses `member.paused` even when the chip is `stopped`; a session-local pause overlay survives a stale auto-refresh. `paused` is `user_smart_schedule_accounts.paused` (migration 207); PUT replace-all keeps it for remaining members. Hot-path `StartCooldown` stays `HSETNX`. A paused pair is skipped after the pool hit and before cooldown; it is not `accounts.schedulable` and must not fold into `IsSchedulable()`. Pair cap comes from the pool member, not `account.UserConcurrency`. GET/PUT responses hydrate read-only `current_concurrency` from Redis pair slots (`GetAccountUserConcurrencyBatch`) for every pool member (capped and uncapped), plus `cooldown_until` from the pair cooldown HASH; writes ignore those fields. Hydrate is admin-read only and does not acquire pair slots or change scheduling. Each member also carries `sort_order` (nullable INT on `user_smart_schedule_accounts`, migration 204): this is **pool display order only**. Empty/null keeps relative id order until the first auto-sort or 移到顶部. The view includes `default_platform` so the admin page opens the relevant tab. Empty enabled saves are rejected; removing the last member (admin pool save **or account delete**) turns the platform off and returns to the legacy account-side rules. Soft-delete does not fire FK CASCADE, so `account_repo.Delete` must delete `user_smart_schedule_accounts` for that `account_id` in the same transaction, persist `enabled=false` when a `(user_id, platform)` pool becomes empty, then `DEL smart-schedule:user:{userID}` and `HDEL smart-schedule:cooldown:{accountID} u:{userID}`. GET omits members that `GetByIDs` cannot load. PUT strips those already-in-pool missing IDs so a poisoned draft can save; newly added unknown IDs still return `SMART_SCHEDULE_UNKNOWN_ACCOUNT`. An enabled row with zero members is treated as disabled on the hot path so the user is not fail-closed. Copy-from-platform copies enable/thresholds/cooldown only. Selection still uses `account.Platform` (mixed-scheduling Antigravity accounts stay on the antigravity tab). Do not copy this policy onto scheduler snapshots.

The pool table reuses the account-list `DataTable` cells: account capacity (inline max + `AccountCapacityCell`), pair occupancy/cap, clickable quality (`AccountStabilityDialog`), today stats, groups, **调度利润** (`SmartSchedulePnlCell`, this user × this account today PnL; not `AccountUsageCell`, no 7-day PnL window; API-key rows keep readable upstream balance, OAuth rows replace that line with the cached 7-day quota bar from extra), schedulable/fallback toggles, **池内顺序** (`user_smart_schedule_accounts.sort_order` only), **调度优先级** (live `accounts.priority`, same field as Account Management), upstream rate, last used, and the account action set (edit / stability / usage / errors / more). When a non-oauth row has readable `extra.upstream_balance_usd` (or usage `balance_usd`), the same cell compares cached `extra.burn_samples` balance burn $/h with today account `actual_cost` implied $/h and shows a compact 对齐/偏离 cue. Display-only; stored billing is unchanged. Column settings live in the full-width table toolbar (`smart-schedule-pool-*` keys), not the right add/filter card. UsersView and the smart-schedule header add a `schedule_pnl` column after `smart_schedule` (pool totals of enabled-platform pairs only). Pair/user cells open `SchedulePnlTrendDialog` (default 24h hourly). Revenue is `SUM(actual_cost)` only on `true_cost IS NOT NULL` rows. AccountsView `AccountUsageCell` is unchanged. GET smart-schedule hydrates read-only member `priority` from live accounts; the pool column binds that live account field and never `sort_order` or a membership snapshot. Pool remove is “remove from pool”, not account delete. Pair-cap edits stay draft until Save; account concurrency / priority / upstream rate / schedulable save immediately via the account update API. Inline priority edits PATCH `accounts.priority` like the accounts page. Pool **移到顶部**, **排序（手动）**, and interval **自动排序** must never write `accounts.priority` or `extra.list_order`. They persist `user_smart_schedule_accounts.sort_order` immediately via `PATCH /admin/users/:id/smart-schedule/:platform/sort-order` (Save of caps is not required; refresh keeps the order). PUT Save also sends `sort_order` so a later replace-all does not wipe it. **移到顶部** assigns that member 1 and the rest 2..N (existing `sort_order` then id). **自动排序** reorders current-platform members by admission → this-user producing (`pair_current > 0`, binary) → `upstream_rate_multiplier` (lower first; type default when missing/negative) → pair headroom → account concurrency → existing `account.priority` (read-only tie-break) → `id`, then writes 1..N to `sort_order` only. It does not change membership, is not Save-gated, and does not use account-global `last_used`. A producing expensive account can still rank above an idle cheaper peer; rate only breaks ties inside the same producing bucket. Empty/null pair cap still means no extra cap (save semantics unchanged; `0` / null is not 999). The hot path still writes and releases Redis pair slots `concurrency:account_user:{accountID}:{userID}` for every enabled closed-pool member request so occupancy can be counted; uncapped is count-only and never `pair_full`. Users not in a smart-schedule closed pool keep the old rule (pair slots only when a real cap N≥1). The pair badge always shows this user’s live occupancy: `current / max` when cap ≥ 1, or `current / 999` when uncapped (999 is display-only, never a real cap, never `account.concurrency`). GET hydrates `current_concurrency` for all pool members from those pair keys, plus read-only `cooldown_until` from HASH `smart-schedule:cooldown:{accountID}` field `u:{userID}`. `default_platform` prefers an enabled platform, then the most recently updated / largest pool. The page opens that tab (zuoge85 lands on openai). The pool toolbar is three distinct regions: **add** and **in-pool filters** live in the right card (compact `w-fit` add bar: left = typeahead + filtered-add + one-click currently-scheduling presets; right = manual refresh + auto-refresh dropdown + 排序（手动） + interval auto-sort dropdown; filters use account-list chrome: search / type / schedulable / admission); **bulk** is full-width with the table under the cards (select page / select filtered / clear, batch remove-from-pool, batch/all pair cap — never account delete). The filtered-add dialog reuses `AccountTableFilters` (platform locked to the active tab) plus schedulable / currently-scheduling / proxy, previews candidates not already in this platform pool, then add-selected or add-all-matching. Membership edits stay Save-gated. Pool filters cover type, account schedulable, and admission (`selectable` / `resumed` / `will_cool` / `cooling` / `paused` / `pair_full` / `stopped` / `unsaved_preview`). Live locks are `paused` (durable membership flag) and Redis pair cooldown (`cooling`). A saved+enabled gate miss without a cooldown HASH field is `will_cool` (hint: next selected request will `StartCooldown`). An unsaved tighter form, or a disabled platform, is `unsaved_preview` — not a lock; Resume is disabled there. After cooldown expires with stats still failing, the chip returns to `will_cool`, never a permanent 质量拦截. Successful Resume clears cooldown and writes `UserQualityResumeActive` grace; the pool column shows `resumed` / selectable. `GET /admin/accounts/quality-stats/batch` does not currently return `resume_users`; the page keeps session-local grace after Resume and will also read those fields if present. Auto-sort ranks preview/hint with will-cool (still pickable), worse than selectable/resumed but better than pair-full / cooling / stopped. Batch remove/apply-cap run on the current filtered selection and stay Save-gated. Auto-refresh copies the account-list dropdown (5/10/15/30s), lives in the add-bar right cluster, spins the trigger refresh icon plus countdown while on, and skips overwriting dirty membership/caps. Interval auto-sort is a toggle next to **排序（手动）**. It does not start a second timer: it follows the auto-refresh interval, writes the same ranking silently after each auto-refresh finishes, persists enable under smart-schedule-auto-sort, and skips while a sort/refresh is in flight. Enabling it alone does nothing until auto-refresh is on; when both are on the sort button reuses the refresh countdown. Sorting is client-side; default and post auto-sort / 移到顶部 is `sort_order asc` (`smart-schedule-pool-sort`). A leftover stored `priority` sort is rewritten to `sort_order`. Column settings reuse the account-list visibility / order / width pattern, persisted under `smart-schedule-pool-*` keys so they do not share the account list layout. Select, name, and actions stay pinned. The hot path still uses `accounts.priority` unchanged.

## Grok account routing and quota state

Grok has two credential modes with different default upstreams:

```
OAuth account
  -> Account.GetGrokBaseURL()
     -> blank/canonical api.x.ai URL => cli-chat-proxy.grok.com
     -> explicit custom URL => preserve exactly
  -> GrokTokenProvider refreshes OAuth access token
  -> HTTPUpstream final boundary injects stable Grok CLI identity

API-key account
  -> Account.GetGrokBaseURL()
     -> blank => api.x.ai/v1
  -> credential api_key is used as Bearer token
```

The final transport injects CLI headers only when `URL.Hostname()` exactly
matches `cli-chat-proxy.grok.com`. This intentionally excludes the public xAI
API and custom-compatible upstreams. `XAI_GROK_CLI_VERSION` is accepted only
when it is canonical semver and not older than the bundled stable version.

Responses, raw Chat Completions, Anthropic Messages conversion, media,
account tests, quota probes, and the HTTP WebSocket bridge all feed xAI quota
headers into the same snapshot/rate-limit reconciliation:

```
xAI headers/status
  -> parseGrokQuotaSnapshot()
  -> UpdateExtra(grok_usage_snapshot)
  -> grokRateLimitResetAt()
  -> runtime scheduling block
  -> SetRateLimitedIfLater()
  -> scheduler snapshot/outbox refresh
```

Successful responses can consume the last request or token, so `remaining=0`
is treated as a real rate limit even on HTTP 200. Concurrent observations may
only extend the persisted reset boundary; an older response must never shorten
it. A temporary unschedulable boundary can still keep the runtime block longer
than the persisted quota reset.

Grok request sanitization removes unsupported Composer reasoning parameters,
Codex-only `additional_tools` input items, unsupported tool types, and existing
xAI-incompatible fields without changing the caller-owned request. Grok 4.3,
4.5, Build, and Composer fallback pricing includes cached-input prices so
missing dynamic pricing fails closed rather than billing at zero.

Prompt caching uses a tenant-isolated UUID derived from the downstream API-key
ID, mapped model, and the stable conversation seed. The raw `session_id`,
`conversation_id`, `X-Grok-Conv-Id`, or `prompt_cache_key` is never forwarded
as the upstream cache key. The Grok-native conversation header participates in
scheduling only when the authenticated group platform is Grok.

For OAuth Chat Completions, a strict compatibility gate may route simple
plain-text `grok-4.5` requests through Responses to obtain cached-input usage.
Any request shape whose semantics could change—tools/functions, structured
image content, developer/tool roles, stop/reasoning parameters, unknown fields,
small token limits, or non-4.5 mappings—stays on raw Chat Completions. The
forward result records the actual endpoint so Ops and usage rows do not infer
the wrong upstream path.

Admin account UI supports both Grok OAuth and API-key creation. API-key forms
default to `https://api.x.ai/v1`; the user key modal generates Grok Build
`config.toml` and an OpenCode `@ai-sdk/openai` Responses-provider example
without exposing account credentials. Grok request/token quota bars display
remaining capacity rather
than consumed percentage. The remaining-capacity mode is opt-in on the shared
progress bar, so Anthropic, OpenAI, Gemini, and Antigravity usage displays keep
their historical used-percentage behavior.

Fast/Flex rules keep storing `user_ids`, but the settings UI resolves them
through administrator email search. `GET /admin/users/:id?include_deleted=true`
is an explicit read-only history path used to label previously selected deleted
users; the default endpoint continues to hide soft-deleted users. Failed
hydration leaves the numeric ID visible instead of dropping the saved rule.

## Anthropic API-Key Upstream Authentication

Anthropic API-key accounts may set
`extra.anthropic_apikey_auth_scheme=authorization_bearer` for compatible
upstreams that require `Authorization: Bearer <api_key>`. Missing, invalid,
OAuth, and non-Anthropic values preserve the historical `x-api-key` default.
The default is omitted from `extra`, so existing accounts remain unchanged.

`anthropic_apikey_auth.go` is the single resolver and header injector. It
deletes both possible auth headers before writing exactly one. Account tests,
model sync, normal and passthrough messages, and both count_tokens paths use it.
OAuth keeps its existing Bearer token and beta/header policy.

This setting does not change model mapping/fallback, scheduler/failover, Ops,
Claude-GPT, Images, billing, quota deduction, `actual_cost`, display tokens,
or cache-read values.

## OpenAI OAuth Pro/Prolite Fleet Usage Summary

Admin accounts page shows a **filter-independent** badge for the global
OpenAI OAuth Pro/Prolite pool (5h and 7d upstream Codex windows).

```
GET /api/v1/admin/accounts/openai-oauth-fleet-usage
  -> AccountUsageService.GetOpenAIOauthFleetUsage
  -> ListAllWithFilters(platform=openai, type=oauth, no status filter)
  -> aggregateOpenAIOauthFleetUsage
  -> attachFleet7dBurnRate (process-local sample ring)
```

Inclusion: OpenAI OAuth **parent** accounts only (`!IsShadow()`), with
`credentials.plan_type` in `{pro, chatgptpro, prolite}`. Plus/team/free and
other plans are excluded. Status/schedulable/rate-limit do **not** exclude.

Pool model (used/capacity units — not a bare percent sum):

- Capacity: Pro = 100 units, Prolite = 25 units (1/4 of Pro)
  - Example: 7 Pro + 1 Prolite → capacity = 725
- Used (per window): `Σ used_percent × (plan_capacity/100)`
  - Example: 7 Pro at 50% + 1 Prolite at 100% → used = 350 + 25 = 375
- UI fraction: `used/capacity` (e.g. `375/725`)
- Bar fill: `used/capacity × 100` (same example ≈ 51.7%)
- Missing `codex_{5h|7d}_used_percent`: still counts toward capacity;
  does not add used for that window (`missing_*` counters)
- Snapshot util uses `buildCodexUsageProgressFromExtra` (expired windows zero)

**7d burn-rate / ETA** (pool-level): samples `capacity - used_7d` over time
(process-local ring), sliding-window linear fit → `burn_rate_7d_per_hour`
(capacity units/h) and `burn_eta_7d_seconds`. Insufficient samples →
`burn_insufficient` without a fake ETA.

UI: `AccountsView` badge (next to AI Credits) with **已用/容量**, fraction
labels, progress bars, and 7d burn line. Mobile: full-width strip above
filters. Independent of list filters.

Read-only admin surface. Does not change scheduling, billing, or single-account
usage cells.

## OpenAI OAuth credential families

OpenAI OAuth rows are still `type=oauth`, but credentials come from three
admin paths that must not be mixed:

| Input | Entry | Stored |
|-------|--------|--------|
| OAuth refresh token | 手动输入 RT / **更新 RT** 纯字符串 | `credentials.refresh_token`；校验时向上游换 AT |
| ChatGPT `/api/auth/session` or Codex session JSON | **导入 Codex 会话**；**更新 RT** 也可贴同一 JSON | JWT `accessToken` → `access_token`；`sessionToken` 只记 `extra.session_token_present`，**永不**写成 `refresh_token` |
| Codex PAT `at-*` | 添加账号 → Codex PAT | `auth_mode=personalAccessToken`；不走 OAuth refresh |

`POST /admin/accounts/:id/refresh-token` classifies `{...}` JSON with the same
`normalizeCodexImportEntry` field paths as Codex import. Session JSON updates
that account’s AT (and metadata). If the row already has a real RT, the RT is
kept. Both sides having a different `chatgpt_account_id` is rejected; a missing
id on the row is backfilled. PAT rows reject session JSON
(`OPENAI_SESSION_AUTH_MODE_MISMATCH`). Default validate checks JWT expiry and
does **not** treat the JSON as an RT. `{ "refresh_token": "..." }` still uses
the RT path. JSON that also contains `refresh_token` / `tokens.refresh_token`
(typical Codex `auth.json`) identity-checks first, then hands the extracted RT
to the same OAuth/skip-validate path — a stale JWT must not block that handoff.
JSON arrays are rejected (single-account update, not bulk import).

## API Key upstream balance + burn-rate

OpenAI/Anthropic **API Key** accounts probe third-party balance APIs on the
account `base_url` (not Console session cookies):

```
GET /admin/accounts/:id/usage
  -> getAPIKeyBalanceUsage
  -> ProbeUpstreamBalance (third-party host order)
       1) {base}/v1/usage → balance|remaining (Sub2API / ZeroCode)
       2) {origin}/api/usage/token → New API token_usage (token-bits / one-api)
            total_available / quota_per_unit → USD; unlimited_quota → balance_unlimited
       3) {base}/v1/dashboard/billing/credit_grants → total_available
       4) subscription + usage → hard_limit_usd - total_usage/100
  -> burn_samples ring in account.extra + ComputeBurnRate
```

`origin` strips trailing `/v1` from account `base_url` so New API routes are not nested under `/v1`.
Official `api.openai.com` / `api.anthropic.com` try OpenAI-shape billing first.

Auth: OpenAI Bearer; Anthropic uses existing `x-api-key` / Bearer scheme.
Kill-switch: `SUB2API_UPSTREAM_BALANCE_PROBE=0|false|off`.

`UsageInfo` fields: `balance_usd`, `balance_*`, `burn_rate_per_hour`,
`burn_rate_unit` (`usd`|`percent`), `burn_eta_seconds`, `burn_insufficient`.

Admin usage cell: **balance is the hero**; today stats / local quota bars are
muted. OAuth accounts show 7d burn as `%/h · ETA` under the 7d bar (5h has no
burn-rate). Samples reset on balance/remaining increase (recharge / window
reset). Does not auto-disable accounts or change billing.

## OpenAI Spark Shadow Accounts

Spark shadows are linked OpenAI OAuth scheduling records. They own an explicit
Spark model mapping and an independent Spark quota/rate-limit dimension, but
never own authentication credentials. `accounts.parent_account_id` points to
the real OAuth account and `accounts.quota_dimension` is `spark`; migrations
`188` and `189` enforce the parent relation and one active Spark shadow per
parent.

Creation uses `POST /api/v1/admin/accounts/:id/shadow`, inherits the parent's
proxy and groups (or the OpenAI default group), and seeds only the Spark model
mapping. Runtime token/header resolution dereferences the parent on every
request. The shadow retains scheduling identity, concurrency, Spark cooldowns,
and `codex_*` usage snapshots; parent credential invalidity and auth/transport
cooldowns still fail closed.

Parent proxy changes propagate to the shadow; direct shadow proxy changes are
rejected. Parent deletion removes the shadow and invalid parent type changes
are blocked. Generic/Codex exports skip shadows and report `skipped_shadows`.
Privacy, token refresh, quota reset, credential persistence, and CRS sync never
treat a shadow as a standalone OAuth account.

The feature does not alter stored billing, quota deduction, `actual_cost`,
display-token or cache-read invariants, curated models, Claude-GPT bridge,
native OpenAI Images, default fallback, Ops attribution, or public settings.

## Anthropic Fable Rate-Limit Window

Anthropic OAuth responses can expose a `7d_oi` window for the Fable model
family. Its utilization/reset snapshot is stored in the existing account
`extra` map and returned as `seven_day_fable`; the account usage cell renders it
as `7d F` beside, rather than instead of, the existing 5h, 7d, and Sonnet
windows.

When `7d_oi` is rejected, only the `claude-fable-5` model family scope is added
to `extra.model_rate_limits`. Sonnet, Opus, and other models remain schedulable.
The ordinary Anthropic 5h/7d windows continue to use the account-level cooldown
and session-window path. A 429 with no usable reset header gets a fixed five
second fallback cooldown so account selection can fail over without repeatedly
selecting the same failing account.

This is scheduling metadata only. It does not alter real token/cache quantities,
quota deduction, stored billing, `actual_cost`, display pricing/transforms,
Spark shadow dimensions, Claude-GPT bridge rules, or advanced scheduler scores.

## OpenAI Codex Personal Access Tokens

OpenAI OAuth-type accounts can be created with a Codex personal access token
(`at-...`) without changing API-key accounts or normal OAuth authorization.
PAT accounts remain `platform=openai` and `type=oauth`; `auth_mode` identifies
the credential mode, background refresh is skipped, and explicit refresh
revalidates the token while removing stale OAuth-only fields.

Key files are `service/openai_codex_pat_service.go`,
`service/openai_chatgpt_headers.go`,
`handler/admin/openai_codex_pat_handler.go`, and the account creation UI.
The admin flow validates Codex `whoami`, preserves model/compact mappings and
fork-local account controls, stores the token only in account credentials, and
returns an account DTO with credentials removed. Request extras cannot duplicate
token material; `account.extra` stores only `access_token_sha256`.

FedRAMP identity is forwarded on existing OpenAI ChatGPT HTTP, WebSocket,
Images, account-test, usage-probe, and model-manifest requests. PAT detection is
restricted to OpenAI OAuth accounts and cannot affect Grok or OpenAI API-key
auth-cache identity/name snapshots. Billing, quota deduction, `actual_cost`,
display tokens/prices, cache-read invariants, Ops, migrations, curated models,
default-model fallback, Images gates, bridge eligibility, and scheduling remain
unchanged.

## OpenAI Quota Query And Reset

OpenAI OAuth accounts expose an admin-only quota action in the existing account
usage cell. `GET /api/v1/admin/openai/accounts/:id/quota` reads ChatGPT/Codex
rate-limit windows and reset-credit metadata. `POST
/api/v1/admin/openai/accounts/:id/reset-quota` consumes one reset credit only
after an explicit confirmation payload.

Core flow:

```text
AccountUsageCell / OpenAIQuotaResetCell
  -> queryOpenAIQuota(accountID)
  -> OpenAIOAuthHandler.QueryQuota
  -> OpenAIQuotaService.QueryUsage
  -> /backend-api/wham/usage
  -> optional sanitized credit-expiration detail query

Confirmed reset action
  -> generate one UUID-v4 redeem_request_id
  -> POST { confirm: true, redeem_request_id }
  -> handler and service validate the UUID
  -> upstream consume receives the same ID unchanged
```

Important mechanisms and pitfalls:

- The backend rejects every non-OpenAI or non-OAuth account before token or
  upstream access. Grok keeps its separate probe-only service and UI.
- The quota service uses the existing OpenAI token provider, account proxy, and
  privacy client. Personal-access-token auth modes reuse their static access
  token and skip OAuth refresh locking.
- Final outbound `User-Agent` and `originator` are paired through
  `enforceCodexIdentityHeaders`; account custom User-Agent remains authoritative.
- Reset requires `confirm=true` plus a valid UUID-v4. The frontend reuses one
  action ID after a failed request, so transport retries cannot mint a new
  upstream idempotency key and consume another credit.
- Queries and resets do not write account cooldowns, quota snapshots, scheduler
  state, usage logs, stored billing, `actual_cost`, display tokens, or cache-read
  quantities. No public/admin Settings KV or new route page is introduced.

## Codex Client And Engine Fingerprint Policy

OpenAI OAuth accounts with `extra.codex_cli_only=true` can optionally apply a
global client policy after account selection and before upstream forwarding.
It supports strict official-client recognition, min/max engine versions, a
deny-first blacklist, two-factor whitelist, app-server admission, and engine
signals.

Key files are `pkg/openai/{allowed_client,engine_fingerprint_signal,request}.go`,
`service/openai_client_restriction_detector.go`, and
`service/setting_service_codex_policy.go`. Responses and Chat Completions use
the same policy. Account create/edit/bulk-edit store the account app-server
opt-in as `extra.codex_cli_only_allow_app_server`.

An unconfigured policy uses legacy detection: existing accounts do not suddenly
require a version or `x-codex-*` header. Version parsing is required only when
min/max is configured. Empty signals disable the fingerprint gate; the UI seed
is an explicit preset, never a runtime default. Required signals are ANDed and
variants within one signal are ORed. Detection is OpenAI-OAuth-only and occurs
before forwarding/billing. Billing, quota, display/cache-read transforms,
Images, bridge, curated/default models, scheduling, platform quota, and Ops are
unchanged.
## Grok Admin Reachability And Media Pricing

Grok accounts are reachable from the existing account and group management
surfaces without replacing the fork-local forms or monolithic locale files.

Data model:

- `AccountPlatform` and `GroupPlatform` include `grok`; Grok accounts use the
  existing OAuth or API-key account types and account credential/extra JSON.
- Grok groups expose image prices plus independent video multiplier controls:
  `video_rate_independent`, `video_rate_multiplier`, and per-second
  `video_price_480p`, `video_price_720p`, `video_price_1080p` prices.
- Blank Grok image prices display the current `$0.02` per-image fallback.
  Blank video prices display `$0.05/s`, `$0.07/s`, and `$0.25/s` for 480p,
  720p, and 1080p respectively.

Key files:

- `frontend/src/api/admin/grok.ts` and
  `frontend/src/composables/useGrokOAuth.ts`: OAuth and quota admin clients.
- `frontend/src/components/account/{CreateAccountModal,EditAccountModal,OAuthAuthorizationFlow}.vue`:
  API-key/OAuth creation and editing.
- `frontend/src/components/admin/account/ReAuthAccountModal.vue`: Grok OAuth
  reauthorization using the shared authorization flow.
- `frontend/src/components/account/GrokQuotaProbeCell.vue`: explicit quota
  probe for OAuth accounts; xAI does not expose a quota-reset operation.
- `frontend/src/views/admin/GroupsView.vue` and `groupsMediaPricing.ts`: Grok
  group selection and media pricing controls/default hints.

Core flow:

```text
Create/Reauthorize Grok OAuth account
  -> POST /api/v1/admin/grok-oauth/auth-url
  -> browser authorization callback code/state
  -> POST /api/v1/admin/grok-oauth/exchange-code
  -> persist credentials through the existing account create/update API

Probe Grok quota
  -> POST /api/v1/admin/grok-oauth/accounts/:id/quota/query
  -> render observed request/token rate-limit windows
```

Important mechanisms and pitfalls:

- API-key creation defaults to `https://api.x.ai/v1`; OAuth creation and edit
  preserve model mappings in the existing credentials object.
- Empty media-price inputs must be normalized before submission. Create sends
  `null`; edit sends the backend clear sentinel for an explicitly cleared
  price instead of an empty string that would fail JSON decoding.
- Grok media pricing is display/configuration reachability only. This frontend
  change does not alter stored billing, quota deduction, `actual_cost`, display
  token transforms, or the fork's price-resolution chain.
- Keep the fork-local curated model list, OpenAI Images endpoint control,
  Claude-GPT bridge controls, account scheduling/failover, and monolithic
  `zh.ts`/`en.ts` locale structure when syncing later upstream UI changes.

## Grok/xAI OAuth And Quota

Grok accounts use `platform=grok` with either OAuth credentials or an API key.
OAuth exchange/refresh is implemented by `internal/pkg/xai`,
`repository/grok_oauth_client.go`, `service/grok_oauth_service.go`, and
`service/grok_token_provider.go`. Admin endpoints are registered under
`/api/v1/admin/grok-oauth`.

Quota probes flow from `GrokQuotaService` through `GrokQuotaFetcher` to the xAI
quota endpoint and persist normalized request/token windows in `Account.Extra`.
Scheduling treats stale snapshots as informational, while active retry-after or
runtime block state excludes the account. The scheduler platform is explicit:
Grok requests cannot select OpenAI accounts and OpenAI requests cannot select
Grok accounts.

Known boundaries: Grok `count_tokens` is unsupported; WebSocket Responses is not
enabled until the HTTP/SSE bridge is reconciled; media HTTP routes wait for the
content-moderation and media-billing batch.

## OpenAI Claude-GPT Bridge For Antigravity Groups

OpenAI accounts can opt into an account-side Claude-GPT bridge with
`extra.openai_claude_gpt_bridge_enabled=true`. This is a routing capability of
the OpenAI account; it does not migrate subscriptions, API keys, or the target
group platform.

Data model:

- The bridge switch is stored in `account.extra.openai_claude_gpt_bridge_enabled`.
- Claude-to-GPT mapping stays in the existing OpenAI
  `account.credentials.model_mapping`, for example
  `{ "claude-opus-4-8": "gpt-5.5" }`.
- OpenAI accounts may bind OpenAI groups by default. When the bridge switch is
  enabled, they may additionally bind Antigravity groups.
- OpenAI accounts still cannot bind Anthropic or Gemini groups through this
  bridge.

Important mechanisms:

- Bridge eligibility requires OpenAI platform, enabled extra flag, an explicit
  account-level model mapping hit, and a mapped model that is non-empty and
  different from the requested Claude model.
- Create/edit/bulk account validation uses the effective extra payload before
  validating group bindings, so the same request can enable the bridge and bind
  an Antigravity group.
- Turning the bridge off in the frontend removes Antigravity group selections so
  stale cross-platform bindings are not submitted.
- The mapping is account-global. There is no group-level or account-group-level
  Claude-GPT mapping.

## API Key Exclusive Group Runtime Guard

API keys are validated against exclusive-group authorization both when they are
created and when they are used.

Data model:

- `users.allowed_groups` is the source of truth for standard exclusive groups.
- Subscription groups still use active subscription checks instead of
  `allowed_groups`.
- The lightweight API-key auth path stores `allowed_groups` and group
  `is_exclusive` in `APIKeyAuthSnapshot`, so cache hits enforce the same rule as
  DB reads.

Important mechanisms:

- `backend/internal/server/middleware/api_key_auth.go` rejects an API key with
  `GROUP_NOT_ALLOWED` when its bound group is exclusive and the owner no longer
  has that group in `allowed_groups`.
- `backend/internal/repository/api_key_repo.go:GetByKeyForAuth` must select
  user allowed groups and group exclusivity fields; removing either field
  weakens runtime enforcement.
- `backend/internal/service/admin_service.go:UpdateUser` invalidates API-key
  auth cache when `allowed_groups` changes, so permission removals do not wait
  for cache TTL expiry.
- `backend/internal/server/middleware/api_key_auth_google.go` must enforce the
  same IP ACL, exclusive-group, runtime expiry, and quota gates before the
  simple-mode billing bypass. Google response formatting differs, but simple
  mode is not an authorization bypass.
- Background Anthropic refresh accepts both OAuth and setup-token account types
  through `Account.IsOAuth()`. The refresh service lists active accounts first,
  and `NeedsRefresh` remains the expiry gate; do not add a second stale
  repository candidate query.

## OpenAI Images Endpoint Scheduling

OpenAI OAuth/API-key accounts can opt out of independent Images endpoint
scheduling with `extra.openai_images_endpoint_enabled=false`.

This switch only affects `/v1/images/generations` and `/v1/images/edits`.
Missing, null, or non-boolean values default to enabled for backward
compatibility. It is intentionally separate from
`extra.codex_image_generation_bridge`, which only controls Codex
`/v1/responses` image tool injection.

Implementation notes:

- Create/Edit account forms save `false` only when disabled; re-enabling removes
  the extra key.
- Bulk edit exposes independent apply checkboxes for Images endpoint scheduling
  and the Codex image tool bridge when every target is an OpenAI OAuth/API-key
  account. Selected fields write explicit booleans; unselected fields are left
  unchanged. The bridge bulk control intentionally offers enabled/disabled only
  because the incremental JSONB merge endpoint cannot delete an account-level
  override to restore global inheritance.
- The scheduler reads the same `Account.SupportsOpenAIImageCapability()` helper
  in both scheduler and load-awareness fallback paths.
- `openai_images_endpoint_enabled` is scheduler-relevant, so updating it must
  enqueue scheduler outbox work and refresh account snapshots.

## OpenAI API-Key upstream endpoint routing

`accounts.extra` for `platform=openai` + `type=apikey`:

| Key | Values / missing |
|---|---|
| `openai_responses_mode` | `auto` (omit/illegal), `force_responses`, `force_chat_completions`, `passthrough` |
| `openai_responses_supported` | bool; missing = Unknown = treat as Responses-capable for **auto** |
| `openai_chat_completions_supported` | bool; display/persist only; **never** changes auto routing |

Routing is `(inbound endpoint, extra) → upstream`. `passthrough` maps inbound CC→upstream CC and inbound Responses→upstream Responses and does not fall back to a protocol bridge when a probe is false. `auto` keeps today's default: if Responses is supported or unprobed, inbound CC still converts to Responses. Missing new fields must not change the path.

Capability probe (`ProbeOpenAIAPIKeyResponsesSupport`) POSTs `/v1/responses` then `/v1/chat/completions` (15s each, same header override / TLS / proxy) and writes both flags in one `UpdateExtra`. Responses still requires 2xx `function_call`; CC only checks endpoint-shaped JSON (no tool_calls requirement). Transport failure leaves that key unwritten.

Triggers: single `Create` (added 2026-08-20), `BatchCreate`, and `Update` only when `credentials` is present. Extra-only saves do not re-probe. No backfill job.

Admin create/edit UI: four modes. Label passthrough as 原样映射 / native mapping so it is not confused with `extra.openai_passthrough` (request body) or WS mode `passthrough`.

## OpenAI API-Key Account Connection Tests

The admin account-test endpoint must follow the same upstream capability
decision as the real OpenAI API-key gateway path.

Important mechanisms:

- API-key accounts that support Responses continue to test with the shared
  OpenAI endpoint URL builder: root base URLs such as `https://example.com`
  map to `https://example.com/v1/responses`, while versioned base URLs such as
  `https://example.com/v1` map to `https://example.com/v1/responses`.
- `passthrough` connectivity tests the existing Responses primary path.
  `force_chat_completions` and `auto` + `openai_responses_supported=false`
  test `{base_url}/v1/chat/completions`, matching raw Chat Completions
  forwarding. Missing mode/probe fields keep today's Responses test.
- The Chat Completions test stream maps upstream `delta.content` and
  `delta.reasoning_content` chunks into the existing account-test SSE
  `content` events, so DeepSeek/Kimi/GLM/Qwen-style compatible upstreams can be
  validated from the admin UI instead of failing before the request is sent.
- Account-test stream parsing is intentionally connectivity-oriented: once a
  Responses or Chat Completions stream emits valid content, EOF or `[DONE]`
  completes the test even when a compatible upstream omits
  `response.completed`. Empty streams still fail before reporting success.
- The Responses test parser also tolerates Chat Completions-style chunks from
  compatible upstreams and handles the final SSE line even when it lacks a
  trailing newline.

## Upstream Model Sync

Admins can fetch a live model list from an account's upstream model-list API and
append missing entries to the local whitelist or Antigravity mapping editor.

Data model:

- No new persisted schema is added. Saved-account sync reads the existing
  account credentials from DB.
- Create-flow preview builds a temporary in-memory account from
  `platform`, `type`, `base_url`, and `api_key`; it does not create or update an
  account.
- The returned model IDs are used only by the frontend to append missing local
  entries.

Key files:

- `backend/internal/service/upstream_models.go`: builds provider-specific
  model-list requests and parses OpenAI-style `data`, Gemini-style `models`,
  and array responses.
- `backend/internal/handler/admin/account_handler.go`: exposes
  `POST /api/v1/admin/accounts/:id/models/sync-upstream` and
  `POST /api/v1/admin/accounts/models/sync-upstream-preview`.
- `frontend/src/components/account/ModelWhitelistSelector.vue`: sync button for
  saved accounts and create-flow preview credentials.
- `frontend/src/components/account/EditAccountModal.vue`: Antigravity saved
  account mapping sync.
- `frontend/src/components/account/CreateAccountModal.vue`: temporary preview
  credentials for API-key account creation, including Antigravity compatible
  upstream mappings.

Important mechanisms:

- Sync is append-only. Existing whitelist entries and Antigravity mappings are
  never deleted or replaced by the sync result.
- Saved-account sync can use stored credentials, proxy assignment, and provider
  token providers.
- Preview sync only uses form credentials and never persists secrets.
- Antigravity OAuth uses the Cloud Code `FetchAvailableModels` path.
  Antigravity API-key sync intentionally requires a compatible gateway base URL
  ending in `/antigravity`.
- This feature does not alter billing, display pricing, model mapping
  resolution, Claude-GPT bridge behavior, OpenAI image endpoint scheduling, or
  Codex image bridge settings.

## 数据模型

| 实体/字段 | 位置 | 说明 |
|-----------|------|------|
| Account entity | `backend/ent/schema/account.go` | 主表，包含 name, platform, type, status 等 |
| credentials (JSONB) | 同上 | OAuth token 数据：access_token, refresh_token, email, project_id, plan_type, expires_at。API Key / Bedrock 另有 `pool_mode`、`pool_mode_retry_count`、可选 `pool_mode_hard_eviction` |
| extra (JSONB) | 同上 | 平台特有配置：allow_overages, mixed_scheduling, privacy_mode, model_rate_limits |
| Account DTO | `backend/internal/handler/dto/types.go:133` | API 响应结构，包含 credentials 和 extra 完整输出 |
| AccountUsageInfo | `frontend/src/types/index.ts:793` | 账号用量信息，含 ai_credits 数组 |
| WindowStats | `frontend/src/types/index.ts:770` | 今日统计（requests, tokens, cost），不含 ai_credits |

### 邮箱存储位置（重要）

| 平台 | 邮箱字段位置 | 来源 |
|------|-------------|------|
| Antigravity | `credentials.email` | Google OAuth UserInfo API |
| Anthropic | `extra.email_address` | Anthropic OAuth 响应 |
| Gemini (google_one) | `credentials.email` | Google OAuth UserInfo API（仅 RT 批量导入路径会写入；OAuth 授权码路径目前不写入） |

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| **Handler** | `backend/internal/handler/admin/account_handler.go` | REST API：List, Create, BatchCreate, GetStats, GetUsage |
| **Handler** | `backend/internal/handler/admin/antigravity_oauth_handler.go` | Antigravity OAuth：GenerateAuthURL, ExchangeCode, RefreshToken |
| **Handler** | `backend/internal/handler/admin/gemini_oauth_handler.go` | Gemini OAuth：GenerateAuthURL, ExchangeCode, GetCapabilities, RefreshToken（仅 google_one） |
| **Service** | `backend/internal/service/admin_service.go` | 业务逻辑：CreateAccount, ListAccounts |
| **Service** | `backend/internal/service/antigravity_oauth_service.go` | OAuth 流程：ValidateRefreshToken, RefreshToken, BuildAccountCredentials |
| **Service** | `backend/internal/service/gemini_oauth_service.go` | Gemini OAuth 流程：ExchangeCode, RefreshToken, ValidateGoogleOneRefreshToken, BuildAccountCredentials, FetchGoogleOneTier |
| **Service** | `backend/internal/service/antigravity_quota_fetcher.go` | AI Credits + 配额获取：FetchQuota → LoadCodeAssist |
| **Service** | `backend/internal/service/antigravity_credits_overages.go` | Credits 耗尽检测、超量请求重试逻辑 |
| **Service** | `backend/internal/service/account_usage_service.go` | 用量统计：GetAccountUsageInfo, GetTodayStats |
| **Repository** | `backend/internal/repository/account_repo.go` | 数据查询：ListWithFilters (搜索 name + email) |
| **API Client** | `backend/internal/pkg/antigravity/client.go` | HTTP 调用：RefreshToken, GetUserInfo, LoadCodeAssist, FetchAvailableModels |
| **Frontend View** | `frontend/src/views/admin/AccountsView.vue` | 账号列表页：表格、搜索、AI Credits 汇总 |
| **Frontend Component** | `frontend/src/components/account/CreateAccountModal.vue` | 创建弹窗：单个 + 批量导入 |
| **Frontend Component** | `frontend/src/components/account/EditAccountModal.vue` | 编辑弹窗 |
| **Frontend Component** | `frontend/src/components/admin/account/UpdateRefreshTokenModal.vue` | 手动更新 OAuth refresh token 弹窗（RT 过期恢复） |
| **Frontend Component** | `frontend/src/components/account/BulkEditAccountModal.vue` | 批量编辑弹窗 |
| **Frontend Component** | `frontend/src/components/common/GroupSelector.vue` | 账号/公告等场景复用的分组多选器；账号表单通过 `show-toggle-all` 开启全选/取消全选 |
| **Frontend Component** | `frontend/src/components/account/AccountUsageCell.vue` | 用量单元格：展示 5h/7d 窗口 + AI Credits |
| **Frontend Composable** | `frontend/src/composables/useAntigravityOAuth.ts` | Antigravity OAuth 前端逻辑：validateRefreshToken, buildCredentials |
| **Frontend API** | `frontend/src/api/admin/accounts.ts` | 账号相关 API 调用封装 |
| **Frontend API** | `frontend/src/api/admin/antigravity.ts` | Antigravity OAuth API：refreshAntigravityToken |

## 核心流程

### Antigravity 批量导入（Refresh Token）

```
用户输入多行 refresh_token
  → CreateAccountModal.vue: handleAntigravityValidateRT()
    → 逐个循环:
      → useAntigravityOAuth.ts: validateRefreshToken(rt, proxyId)
        → POST /api/v1/admin/antigravity/oauth/refresh-token
          → antigravity_oauth_handler.go: RefreshToken()
            → antigravity_oauth_service.go: ValidateRefreshToken()
              → s.RefreshToken() → client.RefreshToken() [Google OAuth]
              → 回填原始 refresh_token（Google 不返回新 RT）
              → client.GetUserInfo() → 获取 email
              → loadProjectIDWithRetry() → client.LoadCodeAssist() → 获取 project_id + plan_type
        ← AntigravityTokenInfo { access_token, refresh_token, email, project_id, plan_type }
      → buildCredentials(tokenInfo) → { access_token, refresh_token, email, ... }
      → 命名: useEmailAsName ? email : form.name + #index
      → buildAntigravityExtra() → { allow_overages?, mixed_scheduling? }
      → POST /api/v1/admin/accounts → account_handler.go: Create()
        → admin_service.go: CreateAccount()
```

### Gemini Google One 批量导入（Refresh Token）

```
用户输入多行 refresh_token（RT 必须由内置 Gemini CLI OAuth client 签发）
  → CreateAccountModal.vue: handleGeminiGoogleOneValidateRT()
    → 逐个循环:
      → useGeminiOAuth.ts: validateGoogleOneRefreshToken(rt, proxyId)
        → POST /api/v1/admin/gemini/oauth/refresh-token
          → gemini_oauth_handler.go: RefreshToken()
            → gemini_oauth_service.go: ValidateGoogleOneRefreshToken()
              → s.RefreshToken(ctx, "google_one", rt, ...) → oauthClient.RefreshToken()
              → 回填原 refresh_token（Google 不返回新 RT）
              → oauthClient.GetUserInfo() → email（失败仅 warn 不阻断）
              → fetchProjectID() → project_id（必需；失败则该 RT 标记失败）
              → FetchGoogleOneTier() → tier_id + drive_storage_* extra（失败回落 google_one_free）
        ← GeminiTokenInfo { access_token, refresh_token, email, project_id, tier_id, extra }
      → buildCredentials(tokenInfo) → { access_token, refresh_token, email, tier_id, oauth_type: "google_one", ... }
      → 命名: useEmailAsName ? email : (多个→form.name #i, 单个→form.name)
      → POST /api/v1/admin/accounts → account_handler.go: Create()
```

限制：RT 必须由内置 Gemini CLI OAuth client（ID `681255809395-...`）签发；自建 client 的 RT 会返回 `unauthorized_client` 错误，提示中已包含对应说明。code_assist 与 ai_studio 暂不支持批量 RT 导入（ai_studio 依赖运营方自配 OAuth client，code_assist 的 project_id 失败率更高）。

### 手动更新 Refresh Token（OAuth 账号 RT 过期场景）

当 OAuth 账号的 refresh_token 过期/失效（自动刷新与 `/:id/refresh` 都会失败、账号被标记 `status=error`）时，管理员可粘贴一个新的 refresh_token 手动恢复，无需走完整的浏览器重新授权。

```
AccountActionMenu.vue: "Update Refresh Token"（仅 type=oauth 可见）
  → UpdateRefreshTokenModal.vue: handleSubmit()
    → accounts.ts: updateRefreshToken(id, rt, { validate, clientId? })
      → POST /api/v1/admin/accounts/:id/refresh-token
        → account_handler.go: UpdateRefreshToken()
          → 合并新 RT 到账号现有 credentials（深拷贝，保留 access_token/project_id/oauth_type/client_id/scope）
          → validate=true（默认）：克隆账号注入新 RT → refreshSingleAccount()
              → 各平台 RefreshAccountToken() 向上游换取新 access_token（校验 RT 是否可用）
              → 落库新凭证 + InvalidateToken；成功后 ClearAccountError() 重新启用账号
              → 校验失败则不落库（账号保留原过期凭证）
          → validate=false：直接 UpdateAccount(merged) + ClearAccountError() + InvalidateToken（不调用上游）
        ← 更新后的账号（前端 patchAccountInList 原地刷新行）
```

与已有动作的区别：`/:id/refresh`（自动刷新，复用账号已存 RT）、重新授权（完整 OAuth 浏览器流程）、本接口（手动粘贴新 RT）。refresh_token 值不会写入日志。

### 账号列表 + 搜索

```
AccountsView.vue: load() / reload()
  → GET /api/v1/admin/accounts?search=xxx&platform=&page=&...
    → account_handler.go: List()
      → admin_service.go: ListAccounts()
        → account_repo.go: ListWithFilters()
          搜索 OR 条件: name ILIKE | credentials->email LIKE | extra->email_address LIKE
  → refreshTodayStatsBatch() → POST /admin/accounts/today-stats/batch
  → refreshAICreditsTotal() → 逐个 GET /admin/accounts/:id/usage（按 email 去重）
```

### 账号跨页选择 + 批量删除

```
AccountsView.vue: selectAllFilteredAccounts()
  → 按当前筛选/排序快照分页调用 GET /api/v1/admin/accounts
  → 收集去重后的 account.id 写入表格选择状态

AccountsView.vue: handleBulkDelete()
  → 快照当前选中 ID
  → 二次确认后复用 deleteAccountIdsInBatches()
  → 每 10 个账号一批调用 DELETE /api/v1/admin/accounts/:id
  → Promise.allSettled 统计每批结果，成功项移出选择，失败项保留以便重试
```

重要机制：

- 跨页全选只收集当前筛选条件下的账号 ID，不改变后端列表或删除接口契约。
- 批量删除不能一次性对 `selIds` 使用 `Promise.all`，否则大量跨页选择会同时发出过多 DELETE 请求，且任一失败会让 UI 过早进入错误分支。
- 普通批量删除和“删除已导出账号”共用同一个 10 并发分批删除 helper，单个账号删除失败不会阻断后续批次。

### 账号数据导出

```
AccountsView.vue: handleExportData()
  → GET /api/v1/admin/accounts/data?limit=&only_unexported=&mark_exported=&include_proxies=
    → account_data.go: ExportData()
      → resolveExportAccounts()
        - 选中账号优先：ids 存在时只解析这些账号
        - 未选中时按当前列表筛选、排序分批读取
        - limit 限制最终导出账号数量
        - only_unexported 跳过 extra.exported_at 非空的账号
      → resolveExportProxies()（include_proxies=true 时）
      → mark_exported=true 时批量写入 extra.exported_at

AccountsView.vue: handleExportCodexAuth()
  → GET /api/v1/admin/accounts/data?format=codex&ids=&limit=1
    → account_data.go: ExportData()
      → resolveExportAccounts()
      → 仅保留 platform=openai、type=oauth 且具备完整 id_token/access_token/refresh_token/account_id 的账号
      → 返回 Codex auth.json 形状的 JSON 对象数组

AccountsView.vue: handleDeleteExportedAccounts()
  → 按当前筛选条件分页调用 GET /api/v1/admin/accounts
  → 前端筛出 extra.exported_at 非空账号
  → 二次确认后按 10 个一批调用 DELETE /api/v1/admin/accounts/:id
```

重要机制：

- 导出格式仍是 `DataPayload{exported_at, proxies, accounts}`，账号凭据和代理密码会原样包含在 JSON 中。
- `format=codex` 是额外导出格式，面向 OpenAI OAuth 账号输出 Codex `auth.json` 兼容对象：`auth_mode=chatgpt`、`OPENAI_API_KEY=null`、`tokens.{id_token,access_token,refresh_token,account_id}`、`last_refresh`。
- Codex 导出跳过非 OpenAI OAuth 账号和 token 不完整的账号；`account_id` 优先使用 `credentials.chatgpt_account_id`，缺失时回退到 `credentials.account_id`。
- `format=codex&mark_exported=true` 只标记实际进入 Codex payload 的账号，不能把同批中不兼容或 token 不完整的账号误标为已导出。
- CC-Switch 一键导入暂不接入账号导出：公开 `ccswitch://v1/import?resource=provider&app=codex...` 协议导入的是 API key/endpoint 形式的第三方 Codex provider，并生成 `model_provider="custom"`；它不能表达 OpenAI Official / ChatGPT OAuth token bundle。
- “已导出”标记存放在 `account.extra.exported_at`，不需要数据库迁移；空字符串或缺失都视为未导出。
- `mark_exported` 只标记本次实际进入导出 payload 的账号；如果同时传 `only_unexported`，已导出账号不会被重复标记。
- “删除已导出账号”按钮只作用于当前筛选条件下 `extra.exported_at` 非空的账号，不会忽略页面筛选直接删除全库。
- 前端账号表提供可切换的“导出时间”列，默认隐藏，必要时从列设置里打开查看。

### Codex 会话账号导入

```
AccountsView.vue: “导入 Codex 会话”
  -> CodexSessionImportModal.vue
     - raw access token / Codex auth JSON / JSON 数组 / 逐行混合输入
     - 代理、OpenAI 分组、并发、优先级、计费倍率、上游倍率、负载因子
     - update_existing / skip_default_group_bind
  -> POST /api/v1/admin/accounts/import/codex-session
     -> account_codex_import.go: ImportCodexSession()
        -> parseCodexSessionImportEntries()
        -> normalizeCodexImportEntry()
           - JWT exp/email/chatgpt_account_id/chatgpt_user_id/plan/organization
           - sessionToken 只记录存在性并告警，不写 refresh_token
           - credential_extras 不能覆盖 OAuth token/identity 保护字段
        -> buildCodexAccountIndex(existing OpenAI OAuth accounts)
        -> full session: user id -> token fingerprint -> account id fallback
        -> access-only: access token SHA-256 fingerprint only
        -> AdminService.UpdateAccount/CreateAccount()
        -> update 后失效 token cache
```

重要机制：

- **完整会话身份优先级**：携带 `refresh_token` 时，`chatgpt_user_id` 是第一身份；共享 `chatgpt_account_id` 只在双方 user id 不冲突时作为兼容回退，支持存量缺失 user id 的账号回填。
- **access-only 隔离**：不携带 `refresh_token` 时只按 access-token 指纹判断同一项。相同 workspace、user 或 email 不能导致两个短期凭据合并；完全相同 token 的重复导入仍可幂等更新。
- **凭据保护**：access-only 更新已有完整 OAuth 账号时保留 `refresh_token`、`client_id`、`id_token` 和账号到期/自动暂停设置，避免用短期会话降级可自动续期账号。
- **到期处理**：新建 access-only 账号必须能从 JWT/JSON 解析 token 到期时间，或由请求显式给出更早的账号到期时间；账号强制开启到期自动暂停。过期 token 被拒绝。
- **PAT 边界**：Codex PAT 仍由 `/admin/openai/create-from-codex-pat` 与 `IsOpenAIPersonalAccessToken()` 管理。会话导入不会重写 PAT 凭据或刷新策略；普通 access-only OAuth 同样不尝试 refresh，过期后返回明确错误。
- **无迁移**：身份、导入来源与 token 指纹均写入现有 `credentials`/`extra` JSON；不新增表或列。

### AI Credits 获取链路

```
AccountsView.vue: refreshAICreditsTotal()
  → 过滤 antigravity 账号 → 按 credentials.email 去重
  → Promise.allSettled: GET /api/v1/admin/accounts/:id/usage
    → account_handler.go: GetUsage()
      → account_usage_service.go: GetAccountUsageInfo()
        → antigravity_quota_fetcher.go: FetchQuota()
          → client.LoadCodeAssist() → PaidTierInfo.AvailableCredits
  → 汇总 ai_credits[].amount
```

### Setup Token 5h Usage Window

```
active usage poll
  -> account_usage_service.go: syncActiveToPassive()
     -> account_repo.go: UpdateExtra(session_window_utilization)
     -> account_repo.go: UpdateSessionWindowEnd(resets_at)

account list / usage cell
  -> account_usage_service.go: estimateSetupTokenUsage()
     -> read Account.SessionWindowEnd as the 5h reset time
     -> zero utilization when the stored window end is already expired
  -> UsageProgressBar.vue
     -> show usage.resetNow for truly idle windows
     -> show usage.resetPending when utilization is still positive but resets_at is stale
```

`UpdateSessionWindowEnd` intentionally updates only `session_window_end`; it does
not overwrite `session_window_start` or `session_window_status`, because those
can be written by the request-path rate-limit/session-window logic.

## 重要机制

| 机制 | 说明 | 相关文件 |
|------|------|---------|
| refresh_token 回填 | Google OAuth 刷新不返回新 RT，ValidateRefreshToken 需回填原始值 | `antigravity_oauth_service.go:228` |
| AI Credits 动态获取 | 不存储在 DB，每次通过 LoadCodeAssist API 实时查询；OAuth 账号必须经 `AntigravityTokenProvider` 取 token，不能直接读 `credentials.access_token` | `antigravity_quota_fetcher.go`, `antigravity_token_provider.go` |
| AI Credits 历史快照 | 为运营分析"credits 消耗 / 每 credit 额度"，`CreditSnapshotService` 每 15 分钟按 email 去重采样到 `ai_credit_snapshots`；`GetAntigravityUsageRatio` 走正向 delta 聚合 | `service/credit_snapshot_service.go`、`migrations/110_add_ai_credit_snapshots.sql` |
| Credits 去重 | 同 Google 账号（同 email）共享 AI Credits 余额，汇总时按 email 去重 | `AccountsView.vue:refreshAICreditsTotal`，`credit_snapshot_service.go:captureOnce` |
| Credits 耗尽检测 | 关键词匹配（"insufficient credit" 等）→ 标记 model_rate_limits["AICredits"] 5h 冷却 | `antigravity_credits_overages.go:36-49` |
| OpenAI Claude-GPT bridge | Bridge 请求挂在 Antigravity 分组下，但真实上游账号是 OpenAI，不消耗 Antigravity AI Credits；`AntigravityUsageAggregator` 继续按 `accounts.platform=antigravity` 聚合，避免污染 credits-per-call / quota-per-credit 比率 | `antigravity_usage_aggregator.go`, `openai_gateway_service.go` |
| 超量请求重试 | 免费配额耗尽后，如 allow_overages=true，注入 enabledCreditTypes: ["GOOGLE_ONE_AI"] 重试 | `antigravity_credits_overages.go:172` |
| 隐私模式设置 | 创建/刷新账号后自动调用 setUserSettings 设置隐私 | `antigravity_oauth_service.go:256` |
| 批量 vs 单创建 | 批量走 handleAntigravityValidateRT()，单创建走 handleAntigravityExchange()，extra 构建需两处一致 | `CreateAccountModal.vue` |
| 账号分组全选 | 创建、编辑、批量编辑共用 `GroupSelector` 的 `show-toggle-all` 入口；全选/取消全选只作用于当前可选分组，保留平台过滤外的既有 `group_ids` | `GroupSelector.vue`, `CreateAccountModal.vue`, `EditAccountModal.vue`, `BulkEditAccountModal.vue` |
| 跨页批量删除 | 跨页选择后的删除必须通过 `deleteAccountIdsInBatches` 以 10 个账号为一批执行，并保留失败 ID 供重试 | `AccountsView.vue`, `AccountsView.bulkEdit.spec.ts` |
| 账号质量列（首字/成功率） | 列表默认显示最近 **15 分钟**滚动窗口的平均 TTFT 与**当前调度**成功率。成功与首字来自 `usage_logs`。账号维同时算两套错误：`terminal_error_*`（客户端 `status>=400`）与 `failover_error_*`（`COALESCE(upstream_status_code, status_code)>=400`，含 Recovered 429/503）。两边都排除 count_tokens、**Claude→GPT 桥接**、以及**客户端/路由的模型不存在或不受支持**。`ErrorCount` / 质量格默认用 terminal；`quality_hard_close_settings.schedule_use_failover_error_rate=true` 时才拷 failover。桥接成功/失败另计 `bridge_*`，**只在** `AccountStabilityDialog` 展示，**不进质量格、不进门槛/硬关闭**。**质量弹窗**（账号列表/智能调度池点质量格）同时显示两套错误率 + 调度开关；用户智能调度门槛区和系统设置质量硬关闭卡共用同一 KV 开关。**单元格**只画 p50 + 调度成功率（`success_count > 0` 且样本达到 min）；空窗口或仅有失败、无成功 usage 显示 `—`。JSON 空窗口的 `success_rate` / `error_rate` / `bridge_error_rate` 必须是 `null`，不能是 `0`。账号列表与智能调度池走 `POST /admin/accounts/quality-stats/batch`（cache key 带开关）；用户列表/页头走 `POST /admin/users/quality-stats/batch`（用户维仍把模型不存在的客户端 4xx 计为失败，`failover_error_count=0`）。列隐藏时不请求；30s 缓存。曲线默认隐藏 p95 | `account_quality.go`, `usage_log_repo.go`, `accountQualityStats.ts`, `AccountsView.vue`, `AccountQualityCell.vue`, `AccountStabilityDialog.vue`, `UserSmartScheduleView.vue` |
| 账号质量历史快照 | 维护任务每 5 分钟把上述同一套 15 分钟滚动窗口落进 `account_quality_snapshots`（有样本才写，保留 7 天，多实例 leader lock）。`GET /admin/accounts/:id/quality-history` 默认最近 24 小时、最大 7 天。硬关闭评估不要改这份 SQL，挂 `AccountQualityMaintenanceService.EvaluateHardClose` / `SetHardCloseEvaluator` | `account_quality_maintenance.go`, `account_quality_snapshot_repo.go`, `migrations/199_account_quality_snapshots.sql` |
| 账号质量硬关闭 | 默认全关。全局 KV `quality_hard_close_settings` + 账号 `extra.quality_hard_close` 两层都开才评估本轮 live 15 分钟质量；越界则 `SetTempUnschedulable(..., "quality_hard_close:...")`，不改 `schedulable`。同一 JSON 里的 `schedule_use_failover_error_rate`（默认 `false`）决定 live `ErrorCount` 用 terminal 还是 failover；不是 per-account overlay。已有未过期临时停调度则跳过（冷却一次，也不覆盖 529/401）。不进 public settings | `account_quality_hard_close.go`, `GET/PUT /admin/settings/quality-hard-close`, `GET/PUT /admin/accounts/:id/quality-hard-close` |
| 仅作兜底调度 (`fallback_only`) | `extra.fallback_only=true` 时账号进入硬兜底层：同池存在任意非兜底候选时永不负载均衡选中；全部 primary 不可用/被排除后才用兜底。与 soft `priority` 解耦。会话粘性若钉在兜底号且 primary 可用会逃逸；`previous_response` 多轮粘性保留 | `account.go` `IsFallbackOnly`, `preferPrimary*`, `openai_account_scheduler.go`, `gateway_service.go` |
| 池模式硬错误停调度 | API Key / Bedrock 账号级开关 `credentials.pool_mode_hard_eviction`。必须先 `IsPoolMode()`。缺省/false = 旧池模式（上游错误不标记本地状态）。两开时余额不足、额度用尽、Key 过期/用户停用/订阅无效走 `handleAuthError` → `SetError`，不落入 429/403 计数。自定义错误码白名单仍优先。无新 Settings KV / 邮件 / webhook / 迁移。Gemini 原先只把 401/403/529 交给 `HandleUpstreamError`，所以 402 / quota-exhausted 429 / credit-balance 400 必须在 `handleGeminiUpstreamError` 里用同一个 `isPoolModeHardMaintenanceError` 提前转发；**调用点和 helper 必须同 commit 发布**，不要再只改 Gemini 一侧 | `account.go` `IsPoolModeHardEviction`, `ratelimit_service.go` `isPoolModeHardMaintenanceError`, `gemini_messages_compat_service.go` `handleGeminiUpstreamError`, `CreateAccountModal.vue`, `EditAccountModal.vue` |
| Gemini RT client 绑定 | Google OAuth 的 refresh_token 绑定签发它的 client_id；google_one 批量导入强制用内置 Gemini CLI client，自建 client 的 RT 报 unauthorized_client | `gemini_oauth_service.go:ValidateGoogleOneRefreshToken` |

## 已知陷阱

- **邮箱双来源**：Antigravity 存 `credentials.email`，Anthropic 存 `extra.email_address`，Gemini google_one RT 批量导入也会写 `credentials.email`。搜索和展示都需兼容两处。
- **批量/单创建分支**：批量导入和单个 OAuth 导入是两个独立代码路径，修改 extra/credentials 构建逻辑时必须两处都改。
- **AI Credits 不在 WindowStats 中**：`getBatchTodayStats` 返回的是 `WindowStats`（requests/tokens/cost），不含 ai_credits。Credits 需单独调 `getUsage` API。
- **Credits 消耗冷启动窗**：`ai_credit_snapshots` 需要至少两条相邻采样才能算 delta。新部署或新窗口内无采样时 `GetAntigravityUsageRatio` 返回 `credits_consumed=0` + 比率 null；前端卡片显示"采样不足"。如果窗口内出现负 delta（充值/重置），只跳过该对不报错，但那一段消耗会丢。
- **质量快照是重叠窗口**：每个点都是当时的 15 分钟滚动窗口，相邻 5 分钟点会重叠；无成功/错误/TTFT/桥接样本的账号不写空行，曲线会有缺口。调度 `ErrorCount` 必须排除 Claude→GPT 桥接失败以及客户端/路由的模型不存在（`SQLExcludeAccountQualityRoutingModelMiss`）；桥接错误率走 `bridge_*` 字段，不要把桥接失败折回门槛。`GetAccountQualityStatsBatch` 的桥接谓词必须与 `IsClaudeGPTBridgeError` / 错误页 bridge 过滤同一出口。用户维 `GetUserQualityStatsBatch` 仍计入模型不存在的客户端 4xx，且 `failover_error_count=0`。用户维 SQL 必须写 `COUNT(*) FILTER (WHERE FALSE) AS failover_error_count`，**禁止** `FILTER (WHERE 0)`：PostgreSQL 要求 FILTER 参数是 boolean，整数 0 会 `pq: argument of FILTER must be type boolean, not type integer`，`POST /admin/users/quality-stats/batch` 500 后用户列表和智能调度页头的首字/成功率会一起变空；账号 batch 不受影响。账号维始终 FILTER 两套错误率；**禁止**无开关把唯一 `ErrorCount` 硬切成 `COALESCE(upstream_status_code, status_code)`。默认 `schedule_use_failover_error_rate=false`，Recovered 200/429 只进对照口径。打开开关后 `ApplyAccountQualityScheduleCaliber` 才把 failover 拷进 `ErrorCount`。账号列表 / `AccountStabilityDialog` batch 立刻按开关重算（cache key 带 `failover:`）；智能调度/硬关闭读 live Redis `account-quality:live:{id}`，下一 5 分钟 tick 才跟上，不要手写 rebuild / 回填 snapshot。保存质量模板必须带上开关，避免冲掉。缺失 `failover_error_count` 不得把用户质量格整格置空（成功率仍走 `success_count` + 用户终态 `error_count`）。
- **质量硬关闭默认全关**：部署后不会自动停号。评估用维护任务同一 tick 的 live stats，不是快照行；账号 overlay 只允许 `UpdateExtra` 合并 `quality_hard_close`，不要用 `Update()` 整表替换 Extra。
- **池模式硬错误是 SetError，不是质量硬关闭**：`pool_mode_hard_eviction` 只在池模式开启时生效；命中余额/额度/租户死号时写 `status=error`，不要改成 `SetTempUnschedulable` 或折进 `quality_hard_close`。`extractUpstreamErrorCode` 读不到本仓库顶层 `{code,message}`，判定器必须自己读顶层 `code`。关掉池模式时同时 `delete` `pool_mode_hard_eviction`，不要回填 `false`。
- **临时不可调度**：token 刷新失败时标记 `temp_unschedulable_until`，到期后自动重试。如果 refresh_token 为空则永远失败。
- **setup-token 401 处理**：`setup-token` 在网关里按 OAuth/Bearer 凭证使用，401 首次命中应走临时不可调度和 token 缓存失效，不应直接标记 `status=error`。
- **Antigravity usage 401 误判**：账号用量/AI Credits 探测必须和模型测试、真实网关请求一样走 `AntigravityTokenProvider`。如果直接读取 DB 中过期的 `credentials.access_token`，会在 refresh token 正常时偶发 401，并让前端误显示“需要重新授权”。
- **Antigravity OAuth 401 状态处理**：OAuth 账号的 401 应优先临时不可调度并触发 token 缓存失效/刷新，不能直接永久 `SetError`。特别是 `/v1/chat/completions` 这类 Anthropic 兼容路径若误选 Antigravity 账号，会因上游路径不匹配返回 `Invalid bearer token`，但账号在 Antigravity 原生路径仍然可用。
