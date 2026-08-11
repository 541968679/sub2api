## 2026-08-11 - fix(admin): persist OpenAI advanced scheduler weight overrides

### What
- Settings save payload now includes OpenAI advanced scheduler sticky-weighted /
  subscription-priority toggles, top-k override, and all score weight overrides
  that were already editable and loadable in System Settings.
- Added a frontend regression test asserting the save payload carries these
  fields after load.

### Why
`saveSettings()` only submitted `openai_advanced_scheduler_enabled`. Weight and
related override edits looked successful in the UI but were never sent to the
backend, so reopening settings always showed empty/default values.

### Verification
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/SettingsView.spec.ts -t "submits OpenAI advanced scheduler weight overrides"`

### Affected files
`frontend/src/views/admin/SettingsView.vue`,
`frontend/src/views/admin/__tests__/SettingsView.spec.ts`, this changelog.

## 2026-08-10 - deploy: production v0.1.206

### What
- Released and deployed `v0.1.206` (`2b4fa84a0`) to production as `ghcr.io/541968679/sub2api:0.1.206`.
- Includes admin user balance burn-rate (opt-in, 5m window, 15s poll, $/h|/min unit switch).

### Verification
- GitHub Actions Release `31378438136` success
- Production health OK; image pin `0.1.206`; container healthy

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-10 - feat(admin): user balance burn-rate column (opt-in)

### What
- Admin users list: optional **消耗速度** column from trailing **5 minutes** `usage_logs.actual_cost`.
- **Default off** toolbar toggle; only when enabled does the UI request/display and poll every **15s**.
- Display unit switchable **$/h** ↔ **$/min** (frontend conversion; 2 decimals like balance).
- New lightweight API `POST /api/v1/admin/dashboard/users-burn-rate` (5m window only; does not re-scan 30d users-usage batch).

### Why
Operators want a quick signal of who is burning balance right now without always-on cost of scanning long usage windows.

### Verification
- `go test -tags=unit ./internal/pkg/usagestats -run TestUserBurnRatePerHour -count=1`
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/UsersView.spec.ts`

### Affected files
`usagestats` types + test, `usage_log_repo.go`, `dashboard_service.go`, `dashboard_handler.go`, `routes/admin.go`, `api_contract_test.go` stub, `frontend/src/api/admin/dashboard.ts`, `UsersView.vue`, i18n zh/en, this changelog.

## 2026-08-10 - deploy: production v0.1.205

### What
- Released and deployed `v0.1.205` (`0bf08d3f4`) to production as `ghcr.io/541968679/sub2api:0.1.205`.
- Includes page-local account drag reorder, Codex image generation rendering fallback, and upstream-sync-guard critical signatures.

### Verification
- GitHub Actions Release `31361301241` success
- Production health OK; image pin `0.1.205`; container healthy

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-09 - feat(admin): page-local drag reorder for accounts

### What
- Admin account list: drag handle on the select cell (left of checkbox) reorders **current page** rows.
- Backend `PUT /api/v1/admin/accounts/reorder` rewrites `extra.list_order` from the submitted top-to-bottom id list (rank multiset preserved when possible).
- Keep existing **移到顶部** button; `list_order` marked scheduler-neutral so pin/reorder does not rebuild scheduler buckets.

### Why
Repeated “move to top” is clumsy for arbitrary mid-list swaps; page-local drag is the minimal useful reorder without full-list UX complexity.

### Verification
- `go test -tags=unit ./internal/service -run "TestMoveAccountToTop|TestReorderAccounts|TestComputeAccountListOrderSlots|TestCreateAccount_PinsNewAccountToListTop" -count=1`
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/accountListOrder.spec.ts`

### Affected files
`admin_service.go`, `account_handler.go`, `admin.go` routes, `account_repo.go`, `accounts.ts`, `AccountsView.vue`, `accountListOrder.ts`, i18n zh/en, this changelog.

## 2026-08-09 - fix(admin): create-account type cards readable + wider dialog

### What
- Create Account dialog uses `full` width (~96vw / 96rem) so multi-zone form is no longer squeezed.
- BaseDialog `extra-wide` / `full` max widths raised for multi-column account editors.
- Platform picker and **账号类型** cards: 1–2 column responsive grid, `min-w-0` + `break-words`, larger touch targets — stops garbled/wrapping text in the left zone.

### Why
After 3-zone layout, account-type chips were forced into a narrow column (`sm:grid-cols-4`), so Chinese labels wrapped into unreadable fragments.

### Verification
- `pnpm --dir frontend exec vitest run src/components/account/__tests__/CreateAccountModal.spec.ts`

### Affected files
`BaseDialog.vue`, `CreateAccountModal.vue`, this changelog.

## 2026-08-08 - fix(admin): surface move-to-top next to account checkbox

### What
- Account list: **移到顶部** is a direct icon button on the select cell (right of checkbox, left of name), not buried under “更多”.
- Removed the duplicate entry from `AccountActionMenu`.
- Default select column width 48 → 72 to fit checkbox + button.

### Why
Operators pin accounts frequently; hiding the control in the overflow menu slowed list reordering.

### Affected files
`AccountsView.vue`, `AccountActionMenu.vue`, this changelog.

## 2026-08-08 - fix(admin): create-account 3-zone layout + pin new accounts to top

### What
- `CreateAccountModal` step-1 form uses the same **extra-wide 3-zone layout** as Edit/Bulk: 账号配置 / 分组 / 其他功能 (other collapsed by default).
- Group selector uses `variant="panel"` + quick filters.
- **Bugfix**: new accounts now write `extra.list_order = UnixMilli()` on create (same pin key as “移到顶部”), so they appear at the top of the admin list instead of sinking below previously pinned rows.

### Why
Create UI had drifted from the redesigned edit layout. List sorting always prepends `list_order DESC`, so accounts without a pin rank fell below every “moved to top” account even when sorting by `created_at desc`.

### Verification
- `pnpm --dir frontend exec vitest run src/components/account/__tests__/CreateAccountModal.spec.ts`
- `go test -tags=unit ./internal/service -run 'TestMoveAccountToTop|TestCreateAccount_Pins' -count=1`

### Affected files
`CreateAccountModal.vue`, `admin_service.go`, `admin_service_move_to_top_test.go`, this changelog.

## 2026-08-08 - fix(admin): unify bulk edit entry + sync 3-zone layout

### What
- Account bulk actions bar: **编辑已选账号** / **按筛选批量编辑** both open the same `BulkEditAccountModal` (scope differs); removed the confusing “批量更新” primary that looked like a second feature.
- `BulkEditAccountModal` layout aligned with `EditAccountModal`: **extra-wide** dialog + 3 zones (账号配置 / 分组 / 其他功能), other zone collapsed by default.
- Added bulk fields already present in single edit: `fallback_only`, `model_mapping_strict_scheduling`.
- Group selector uses `variant="panel"` + quick filters like single edit.

### Why
Operators saw two bulk buttons that felt like different features, and bulk edit UI had drifted from the redesigned single-account editor.

### Verification
- `pnpm --dir frontend exec vitest run src/components/account/__tests__/BulkEditAccountModal.spec.ts src/views/admin/__tests__/AccountsView.bulkEdit.spec.ts`

### Affected files
`AccountBulkActionsBar.vue`, `BulkEditAccountModal.vue`, bulk-edit specs, i18n zh/en, this changelog.

## 2026-08-08 - fix(admin): display-balance used/total save appears stuck

### What
- Account usage cell **已用 / 总额** save no longer force-calls `getUsage(source=active)` after `POST .../display-balance`.
- Force probe timeouts previously caused a successful DB write to look like a failed save (editor stayed open, error only in console).
- Save now optimistically updates the usage hero from the API response, emits `account-updated` so the list row keeps `extra`, and shows success/error toasts.
- Inputs use text + `inputmode=decimal` with safe parsing (avoids number-input/`trim` edge cases).

### Why
Operators could not reliably save display used/total on API-key accounts when upstream balance probe was slow or failed.

### Verification
- `pnpm --dir frontend exec vitest run src/components/account/__tests__/AccountUsageCell.spec.ts -t "display-balance"`

### Affected files
`frontend/src/components/account/AccountUsageCell.vue`, `AccountsView.vue`, i18n zh/en, `AccountUsageCell.spec.ts`, this changelog.

## 2026-08-07 - feat(account): per-account model_mapping strict scheduling toggle

### What
- Per-account switch stored in `accounts.extra.model_mapping_strict_scheduling` (default **false** / missing).
- **Off**: non-empty `model_mapping` still allows legacy fallbacks (platform default mapping / OpenAI DefaultModels) for unlisted request models.
- **On**: non-empty `model_mapping` is a strict scheduling allowlist for that account only.
- Empty mapping still allows all. Admin global setting was **not** used (account-scoped only).
- Edit dialog: toggle under 「其他功能」; zone defaults to expanded.

### Why
Strict whitelist is needed for accounts like zerocode/xiaoyao without changing behavior for every other account that relies on DefaultModels fallback.

### Verification
- `go test -tags=unit ./internal/service -run 'TestAccountIsModelSupported|TestAccountIsModelSupported_PerAccountStrictScheduling' -count=1`

### Affected files
`backend/internal/service/account.go`, `account_wildcard_test.go`, `backend/internal/repository/scheduler_cache.go` (snapshot Extra must keep `model_mapping_strict_scheduling`), `EditAccountModal.vue`, i18n zh/en, this changelog.

## 2026-08-07 - fix(admin): account model mapping not persisting in edit dialog

### What
- After saving account model whitelist/mapping, the list-row account used for reopen now keeps the **submitted** `credentials.model_mapping` even if the update API response omits it.
- `buildModelMappingObject` falls back across whitelist/mapping tab state so a mode-tab mismatch no longer silently deletes mapping.
- `normalizeModelMappingRecord` hardens reload of `credentials.model_mapping` (object or JSON string).
- Edit dialog auto-expands the "other" zone when a mapping already exists so reopen does not look empty/hidden.

### Why
Operators reported adding model mapping, clicking update, then reopening the account editor with an empty mapping UI. The editor hydrates only from the list-row account object; partial update responses and mode-tab mismatches could drop `model_mapping` on the next open.

### Verification
- `pnpm exec vitest run src/composables/__tests__/useModelWhitelist.spec.ts src/components/account/__tests__/EditAccountModal.spec.ts`

### Affected files
`frontend/src/composables/useModelWhitelist.ts`, `frontend/src/components/account/EditAccountModal.vue`, related unit tests, this changelog.

## 2026-08-07 - feat(admin): display-only account balance used/total

### What
- Upstream balance probe stores **used USD** (New API `total_used`, credit_grants, subscription usage).
- Unlimited upstream no longer shows bare "无限": falls back to **已用 $X**; with manual total shows **$used / $total**.
- Admin can **edit used + total** (display only) via account usage cell; **refresh** force-probes upstream and overwrites used.
- API: `POST /admin/accounts/:id/display-balance` (`used_usd`, `total_usd`, clear flags). No scheduling/admission/billing impact even when over total.

### Why
Third-party New API tokens often return `unlimited_quota=true` with real spend in `total_used`; operators need honest used display and optional prepaid package total (e.g. 25/125).

### Affected files
`upstream_balance_probe.go`, `account_usage_service.go`, `burn_rate.go`, `account_handler.go`, `routes/admin.go`, FE `AccountUsageCell.vue`, accounts API, types, i18n, this changelog.

## 2026-08-07 - feat(admin): account row button to open error requests

### What
- Account list actions: next to Usage, add **Errors** button opening `/admin/usage?tab=errors&account_id=…` in a new tab.
- Usage page deep-link: `tab=errors` switches to error-requests tab and seeds `account_id` / `user_id` into error filters.
- Error request account filter prefers `account_name` query for instant label display.

### Why
Operators need one-click drill-down from a stuck/degraded account into its error request history without manual filter picking.

### Affected files
`frontend/src/views/admin/AccountsView.vue`, `UsageView.vue`, `ErrorRequestFilters.vue`, i18n zh/en, this changelog.

## 2026-08-07 - feat(admin): clear account concurrency button + stream debug logs

### What
- Admin account menu: one-click **Clear concurrency slots** (`POST /api/v1/admin/accounts/:id/clear-concurrency`).
- Clears Redis account concurrency slots + wait counters only; does **not** change schedulable/status and does **not** touch sticky/session-limit or scheduling escape logic.
- Debug logs: `account_slot_acquired` / `account_slot_released`, OpenAI `stream_debug` (headers / first_client_output / completed) with header_ms vs first_token_ms vs duration_ms.

### Why
Ops need a safe manual unblock when account concurrency piles up without reintroducing v0.1.199 sticky-escape scheduling changes. Stream timing logs help reconcile upstream TTFT vs Sub2API metrics.

### Verification
- `go test -tags=unit ./internal/service -run TestCleanupStaleProcessSlots -count=1`
- `go build ./internal/handler/admin ./cmd/server`

### Affected files
`backend/internal/repository/concurrency_cache.go`, `concurrency_service.go`, `handler/admin/account_handler.go`, `server/routes/admin.go`, `openai_gateway_service.go`, stubs/tests, FE accounts API/menu/i18n, VERSION `0.1.201`, this changelog.

## 2026-08-07 - fix(admin): align error-request filters with usage-record search UX

### What
- Rewrite 使用记录 → 错误请求 filters: user/API key/account use focus dropdown + search-by-name; group/model use searchable Select (by name, not raw numeric ID).
- Keep error-only filters: platform, bridge, upstream model, keyword, multi-select status codes.
- Query builder now sends `user_id` / `api_key_id` / `group_id` / `account_id` resolved from name pickers.

### Why
Error tab used plain text ID fields with no auto-dropdown/match, which was unusable for operators who only know group/account names.

### Verification
- `pnpm --dir frontend exec vitest run src/components/admin/usage/__tests__/ErrorRequestFilters.spec.ts`
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/UsageView.spec.ts`

### Affected files
`frontend/src/components/admin/usage/ErrorRequestFilters.vue`, `frontend/src/views/admin/UsageView.vue`, `frontend/src/components/admin/usage/__tests__/ErrorRequestFilters.spec.ts`, i18n zh/en, this changelog.

## 2026-08-07 - release: v0.1.200 admin recharge history manage (hotfix on 0.1.198)

### What
- Hotfix release based on production baseline `v0.1.198` (does not re-introduce `v0.1.199` sticky/concurrency changes).
- Admin Users → More → manage/delete user balance and concurrency history records.
- Delete reuses existing redeem-code delete API and does not reverse balances.

### Why
Ship the recharge-history admin tool to production while production remains on the 0.1.198 rollback line after the billing anomaly.

### Verification
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/UsersView.spec.ts src/components/admin/user/__tests__/UserBalanceHistoryManageModal.spec.ts`

### Affected files
Frontend admin users view, balance history manage modal, i18n, tests.

## 2026-08-06 - feat(admin): account list column order and resize

### What
- Account management column settings support reordering (up/down) with pinned select/name/actions.
- Desktop table headers support drag-to-resize; order and widths persist in `localStorage` (`account-column-layout`).
- DataTable gains optional `resizableColumns` + `column-resize` event and `Column.width` / `minWidth`.
- Reset control restores default order and clears custom widths.

### Why
Admins need to put frequently edited columns (e.g. priority next to schedulable) where they work and widen cramped cells without code changes.

### Verification
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/accountColumnLayout.spec.ts`

### Affected files
`frontend/src/components/common/types.ts`, `frontend/src/components/common/DataTable.vue`, `frontend/src/views/admin/accountColumnLayout.ts`, `frontend/src/views/admin/AccountsView.vue`, `frontend/src/views/admin/__tests__/accountColumnLayout.spec.ts`, i18n zh/en, this changelog.

## 2026-08-06 - fix(admin): account edit modal wrongly clears group checkboxes

### What
- Fix EditAccountModal bridge-group cleanup so same-platform groups are never wiped on open (root cause of intermittent wrong checkboxes).
- Run bridge cleanup only after form hydration, with a sync guard so intermediate toggle resets cannot strip groups.
- Do not re-hydrate the edit form when list auto-refresh replaces the same account object (same id) mid-dialog.
- Prefer submitted `group_ids` in the update emit; backend Update now returns post-bind `group_ids`.
- List merge keeps prior `group_ids`/`groups` when a payload omits them.
- CreateAccountModal gets the same platform-scoped watcher condition.

### Why
Opening account edit sometimes showed different group checkboxes than the real membership; Update without touching groups then persisted the wrong set. Intermittency came from platform-specific strip logic + whether the groups catalog had loaded + auto-refresh re-syncing the form.

### Verification
- `pnpm --dir frontend exec vitest run src/components/account/__tests__/EditAccountModal.spec.ts`

### Affected files
`frontend/src/components/account/EditAccountModal.vue`, `frontend/src/components/account/CreateAccountModal.vue`, `frontend/src/components/account/__tests__/EditAccountModal.spec.ts`, `frontend/src/views/admin/AccountsView.vue`, `backend/internal/service/account_service.go`, this changelog.

## 2026-08-04 - feat(admin): user list auto-refresh for live concurrency

### What
- User management toolbar gains auto-refresh (5/10/15/30s), same pattern as accounts.
- Silent incremental list refresh updates `current_concurrency` without full-page loading spinner.
- Preference persisted in `localStorage` (`user-auto-refresh`).
- Accounts auto-refresh default interval changed from 30s to 5s for new preferences.

### Why
Admins need live concurrency on user and account lists without manually reloading the page when investigating occupancy mismatches.

### Verification
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/UsersView.spec.ts`

### Affected files
`frontend/src/views/admin/UsersView.vue`, `frontend/src/views/admin/AccountsView.vue`, `frontend/src/i18n/locales/zh.ts`, `frontend/src/i18n/locales/en.ts`, `frontend/src/views/admin/__tests__/UsersView.spec.ts`, this changelog.

## 2026-08-03 - feat(admin): risk-control auto-ban notification inbox

### What
- When content-moderation auto-ban newly disables a user, create an admin inbox event.
- Admin header shows a shield badge with unread count; panel supports mark read, clear (global for all admins), and quick unban.
- Read state is per admin; clear/delete removes the message for every admin without touching audit logs or ban status.

### Why
Admins previously only discovered auto-bans by manually opening risk-control logs; users already received ban emails.

### Verification
- `go test -tags=unit ./internal/service -run TestContentModerationAutoBan`
- `go build ./internal/server`

### Affected files
`backend/migrations/195_admin_risk_ban_notifications.sql`, `backend/internal/service/content_moderation.go`, `backend/internal/repository/content_moderation_repo.go`, `backend/internal/handler/admin/content_moderation_handler.go`, `backend/internal/server/routes/admin.go`, `frontend/src/api/admin/riskControl.ts`, `frontend/src/stores/banNotifications.ts`, `frontend/src/components/common/BanNotificationBell.vue`, `frontend/src/components/layout/AppHeader.vue`, `frontend/src/App.vue`, i18n zh/en, `docs/dev/codebase/risk-control.md`, this changelog.

## 2026-08-03 - docs: graded assessment for upstream v0.1.152..main

### What
- Draft multi-batch assessment for selective sync after large baseline `v0.1.152` / `b73d8c3ef`.
- Proposed first wave B1鈥揃2鈥揃3鈥揃7鈥揃9; defer high-risk B4 billing fields, B5 profit-control, B8 security-audit, B12 upstream async image pending product decisions.
- Linked from `UPSTREAM_SYNC.md`; no product code changes in this step.

### Why
AGENTS.md requires an explicit assessment table before upstream-sync code changes; volume is ~860+ commits and needs graded handling with fork-local boundaries.

### Verification
- Document-only; commit themes counted from `git log b73d8c3ef..upstream/main` and path churn; fork feature presence checked via tree/grep.

### Affected files
`docs/dev/UPSTREAM_SYNC_ASSESSMENT_v0152_to_main.md`, `docs/dev/UPSTREAM_SYNC.md`, this changelog.

## 2026-08-03 - docs: pin large upstream-sync baseline to v0.1.152

### What
- Record fork release **0.1.195** as mapping to large-sync upstream **`v0.1.152` / `b73d8c3ef`** (2026-07-14 graded batch), not to later hotfixes.
- Demote 2026-07-27 item-ID sanitization to hotfix-only (does not raise content ceiling).
- Add machine-readable pin `docs/dev/UPSTREAM_BASE.json` and refresh銆屽綋鍓嶇姸鎬併€峴o pending eval window is `b73d8c3ef..upstream/main`.

### Why
Selective small-feature cherry-picks were being mistaken for the sync baseline; version numbers (fork 0.1.195 vs upstream 0.1.170) had no explicit mapping.

### Verification
- Document-only; cross-checked against `docs/dev/UPSTREAM_SYNC.md` batch entries, `b73d8c3ef`, and current `upstream/main` tip.

### Affected files
`docs/dev/UPSTREAM_SYNC.md`, `docs/dev/UPSTREAM_BASE.json`, this changelog.

## 2026-08-03 - deploy: production v0.1.197

### What
- Released and deployed `v0.1.197` to production via GHCR.
- Includes: account move-to-top, mobile user column settings, default concurrency sort; also New API `/api/usage/token` balance (`v0.1.196` image content via main).

### Verification
- Release workflow success: run `30805113999`
- `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`, health check passed
- revision `734787d60`, version `0.1.197`, healthy, `/health` OK
- digest `sha256:ceb2c59c869e7bcfb07c0ea9e9dd5485030fa41ff9ec558ec3395a099c8581fd`

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-03 - fix(admin): mobile user column settings + concurrency sort

### What
- Users admin: column-settings / filter dropdowns no longer open off-screen on mobile (`left-0` on small screens, `right-0` on md+).
- Default user list sort is **current concurrency high 鈫?low**; concurrency column is visible by default.

### Why
Mobile toolbar put the column button on the left while the menu used `absolute right-0`, pushing it off-viewport; default sort was still `created_at`.

### Affected files
`frontend/src/views/admin/UsersView.vue`, this changelog.

## 2026-08-03 - feat(admin): account list move-to-top

### What
- Admin account action menu: **绉诲埌椤堕儴 / Move to top**.
- Persists pin rank in `account.extra.list_order` (Unix ms); list queries always order by `list_order DESC` first.

### Why
Operators need a quick way to pin frequently managed accounts to the top of the accounts table without changing scheduler priority.

### Verification
- `go test -tags=unit ./internal/service -run TestMoveAccountToTop -count=1`

### Affected files
`backend/internal/service/admin_service.go`,
`backend/internal/service/admin_service_move_to_top_test.go`,
`backend/internal/repository/account_repo.go`,
`backend/internal/handler/admin/account_handler.go`,
`backend/internal/server/routes/admin.go`,
`frontend/src/components/admin/account/AccountActionMenu.vue`,
`frontend/src/views/admin/AccountsView.vue`,
`frontend/src/api/admin/accounts.ts`,
`frontend/src/i18n/locales/{zh,en}.ts`,
this changelog.

## 2026-08-03 - fix: New API /api/usage/token balance (token-bits)

### What
- Third-party balance probe adds New API `GET {origin}/api/usage/token` (Bearer sk-...).
- Converts `total_available / quota_per_unit` to USD; supports `unlimited_quota` UI flag.
- Strips trailing `/v1` from account base_url when calling New API admin-style routes.

### Why
token-bits and other New API gateways do not expose OpenAI `credit_grants` or Sub2API `/v1/usage`; balance is only on `/api/usage/token`.

### Verification
- Live: `GET https://api.token-bits.com/api/usage/token` 鈫?`token_usage` JSON
- `go test -tags=unit ./internal/service -run "ProbeUpstream|NewAPI|OriginFrom" -count=1`

### Affected files
`backend/internal/service/upstream_balance_probe.go`,
`backend/internal/service/upstream_balance_probe_test.go`,
`backend/internal/service/account_usage_service.go`,
`frontend/src/components/account/AccountUsageCell.vue`,
`frontend/src/types/index.ts`,
`frontend/src/i18n/locales/{zh,en}.ts`,
`docs/dev/codebase/account.md`,
this changelog.

## 2026-08-03 - deploy: production v0.1.195

### What
- Released and deployed `v0.1.195` to production via GHCR.
- API key balance probe + burn-rate/ETA; Sub2API/ZeroCode `/v1/usage` balance path.

### Verification
- Release workflow success: run `30778084505`
- `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`, health check passed
- revision `f588f50ab`, version `0.1.195`, healthy, `/health` OK
- digest `sha256:d14ccb1b5957a1be5c22224328766a7dc5123e6af7d5f2c8f5362ebd584ae58d`

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-03 - fix: Sub2API/ZeroCode balance via GET /v1/usage

### What
- Third-party balance probe prefers `GET {base}/v1/usage` (Sub2API public usage summary: `balance` / `remaining`) before OpenAI `credit_grants`.
- Fixes ZeroCode (`zerocode.kaynlab.com`) accounts showing 浣欓涓嶅彲鐢?when `credit_grants` is 404.

### Why
ZeroCode is a Sub2API-compatible gateway; wallet balance is exposed on `/v1/usage`, not OpenAI dashboard billing paths.

### Verification
- Live probe: `GET https://zerocode.kaynlab.com/v1/usage` with account key 鈫?`balance鈮?8004`
- `go test -tags=unit ./internal/service -run "ProbeUpstream|Sub2API|IsOfficial" -count=1`

### Affected files
`backend/internal/service/upstream_balance_probe.go`,
`backend/internal/service/upstream_balance_probe_test.go`,
`docs/dev/codebase/account.md`,
this changelog.

## 2026-08-03 - feat: API key balance + burn-rate / remaining-time ETA

### What
- OpenAI/Anthropic API Key accounts: probe third-party OpenAI-shape billing balance (`credit_grants` then `subscription+usage`) via account `base_url`; show balance as hero in usage cell.
- Burn-rate + remaining-time ETA for API Key (USD samples) and OAuth 7d remaining% samples; sliding-window linear fit; recharge/reset starts a new epoch.
- OAI Pro/Prolite fleet badge: 7d pool burn-rate (capacity units/h) + ETA from process-local samples.
- Kill-switch: `SUB2API_UPSTREAM_BALANCE_PROBE=0`.

### Why
Operators need prepaid balance visibility and 鈥渉ow long until empty鈥?for keys and OAuth pools, not only consumed usage.

### Verification
- `go test -tags=unit ./internal/service -run "BurnRate|ProbeUpstream|JoinOpenAI|RemainingPct|SerializeParse|SupportsUpstream|AggregateOpenAIOauthFleet" -count=1`
- Frontend typecheck / vitest as available for AccountUsageCell and fleet i18n.

### Affected files
`backend/internal/service/burn_rate.go`,
`backend/internal/service/burn_rate_test.go`,
`backend/internal/service/upstream_balance_probe.go`,
`backend/internal/service/upstream_balance_probe_test.go`,
`backend/internal/service/account_usage_service.go`,
`backend/internal/service/account_usage_fleet.go`,
`frontend/src/components/account/AccountUsageCell.vue`,
`frontend/src/views/admin/AccountsView.vue`,
`frontend/src/types/index.ts`,
`frontend/src/api/admin/accounts.ts`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
`docs/dev/codebase/account.md`,
this changelog.

## 2026-08-02 - deploy: production v0.1.194

### What
- Released and deployed `v0.1.194` to production via GHCR.
- OAI fleet badge: used/capacity pool model + two-column accounts toolbar layout.

### Verification
- Release workflow success: run `30745498824`
- `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`, health check passed
- revision `48787ba6a`, version `0.1.194`, healthy, `/health` OK
- digest `sha256:050a076d41c98f3a73018fd4b2976e41fca07391b7cd52bfa504bbb57ea816a8`

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-02 - fix: OAI fleet used/capacity pool + two-column layout

### What
- Fleet aggregate is **used/capacity** (e.g. 375/725), bar fill = used梅capacity.
- Accounts page layout: left ops toolbar column, right fleet card column (same row).
- Capacity Pro脳100 + Prolite脳25; missing snapshots still count capacity.

### Why
Bare percent sum misled operators; card placement had been fighting the toolbar.

### Verification
- `go test -tags=unit ./internal/service -run "TestAggregateOpenAIOauthFleetUsage|TestOpenAIOauthFleetPlanCapacity" -count=1`
- `pnpm exec vitest run src/views/admin/__tests__/AccountsView.oauthFleetUsage.spec.ts`

### Affected files
`backend/internal/service/account_usage_fleet.go`,
`backend/internal/service/account_usage_fleet_test.go`,
`frontend/src/api/admin/accounts.ts`,
`frontend/src/views/admin/AccountsView.vue`,
`frontend/src/components/account/UsageProgressBar.vue`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
`frontend/src/views/admin/__tests__/AccountsView.oauthFleetUsage.spec.ts`,
`docs/dev/codebase/account.md`,
this changelog.

## 2026-08-02 - fix: place larger OAI fleet card under create-account actions

### What
- Moved the OAI Pro pool card to sit **below** the account action toolbar
  (under 娣诲姞璐﹀彿), not as a tiny top-right corner chip.
- Enlarged typography, padding, and progress tracks; shows `used/capacity`.

### Why
The previous right-corner placement was too far up/right and too small to read.

### Verification
- Manual: `/admin/accounts` 鈥?filters left, actions right, fleet card under actions.

### Affected files
`frontend/src/views/admin/AccountsView.vue`, this changelog.

## 2026-08-02 - fix: restore accounts page toolbar layout

### What
- Reverted the fleet badge mobile `flex-col`/`order` toolbar rewrite that scrambled
  filters and action buttons on `/admin/accounts`.
- Fleet + AI Credits stay in the original right-side compact slot under
  `flex-wrap-reverse justify-between`.

### Why
Local accounts page ops area was unusable after the mobile layout experiment.

### Verification
- Manual: `/admin/accounts` toolbar matches pre-change structure (filters left, actions right).

### Affected files
`frontend/src/views/admin/AccountsView.vue`, this changelog.

## 2026-08-02 - fix: OAI fleet badge uses used/capacity pool (e.g. 375/725)

### What
- Replaced bare weighted-percent sum (e.g. misleading "386%") with pool units:
  capacity = Pro脳100 + Prolite脳25; used = 危 used% 脳 plan units; UI `used/capacity`.
- Progress bar fill is `used/capacity脳100`. Missing snapshots still occupy capacity.

### Why
Operators need 375/725 style pool pressure, not an unbounded percent sum.

### Verification
- `go test -tags=unit ./internal/service -run "TestAggregateOpenAIOauthFleetUsage|TestOpenAIOauthFleetPlanCapacity" -count=1`
- `pnpm exec vitest run src/views/admin/__tests__/AccountsView.oauthFleetUsage.spec.ts`

### Affected files
`backend/internal/service/account_usage_fleet.go`,
`backend/internal/service/account_usage_fleet_test.go`,
`frontend/src/api/admin/accounts.ts`,
`frontend/src/views/admin/AccountsView.vue`,
`frontend/src/components/account/UsageProgressBar.vue`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
`frontend/src/views/admin/__tests__/AccountsView.oauthFleetUsage.spec.ts`,
`docs/dev/codebase/account.md`,
this changelog.

## 2026-08-02 - deploy: production v0.1.193

### What
- Released and deployed `v0.1.193` to production via GHCR.
- Includes: OpenAI OAuth Pro/Prolite fleet 5h/7d used usage badge (filter-independent),
  used labels, progress bars, mobile layout.

### Verification
- Release workflow success (workflow_dispatch): run `30741788133`
- `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`, health check passed
- revision `a7b37e0c0`, version `0.1.193`, healthy, `/health` OK
- digest `sha256:66feb930867e4ea3ca4b19f0418814699a6ed2fe1fac49550dfea0257da47dd8`

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-02 - fix: fleet badge mobile layout + wider used bars

### What
- OAI Pro pool strip is full-width on small screens and ordered above filters/actions.
- Detail text wraps; progress tracks are wider on mobile (`trackWidthClass` on
  `UsageProgressBar`, table cells keep default `w-8`).

### Why
Initial fleet badge only relied on flex-wrap and was cramped / hard to read on phones.

### Verification
- `pnpm exec vitest run src/components/account/__tests__/UsageProgressBar.spec.ts src/views/admin/__tests__/AccountsView.oauthFleetUsage.spec.ts`

### Affected files
`frontend/src/views/admin/AccountsView.vue`,
`frontend/src/components/account/UsageProgressBar.vue`,
this changelog.

## 2026-08-02 - fix: fleet badge labels used% + progress bars

### What
- OAI Pro pool badge now marks **宸茬敤 / Used** (not remaining).
- Renders 5h/7d with the same `UsageProgressBar` as per-account usage windows.

### Why
Plain "12%" was ambiguous (used vs remaining); operators also wanted bar affordance.

### Verification
- `pnpm exec vitest run src/views/admin/__tests__/AccountsView.oauthFleetUsage.spec.ts`

### Affected files
`frontend/src/views/admin/AccountsView.vue`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
`frontend/src/views/admin/__tests__/AccountsView.oauthFleetUsage.spec.ts`,
this changelog.

## 2026-08-02 - feat: OpenAI OAuth Pro/Prolite fleet 5h/7d usage badge

### What
- Account management page shows a permanent top-right summary of all OpenAI
  OAuth **Pro/Prolite parent** accounts' upstream 5h/7d usage.
- Aggregation: weighted **percent sum** (prolite = 1/4 of pro); can exceed 100%.
- Scope is **independent of list filters**; excludes shadows and non-pro plans;
  includes all statuses; missing snapshots skip that window and increment miss counts.
- New admin API: `GET /api/v1/admin/accounts/openai-oauth-fleet-usage`.

### Why
Operators need a single glance at total Pro-pool pressure without changing
group/search filters or paging through accounts.

### Verification
- `go test -tags=unit ./internal/service -run "TestAggregateOpenAIOauthFleetUsage|TestOpenAIOauthFleetPlanWeight" -count=1`
- Frontend i18n spec for fleet badge copy

### Affected files
`backend/internal/service/account_usage_fleet.go`,
`backend/internal/service/account_usage_fleet_test.go`,
`backend/internal/handler/admin/account_handler.go`,
`backend/internal/server/routes/admin.go`,
`frontend/src/api/admin/accounts.ts`,
`frontend/src/views/admin/AccountsView.vue`,
`frontend/src/views/admin/__tests__/AccountsView.oauthFleetUsage.spec.ts`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
`docs/dev/codebase/account.md`,
this changelog.

## 2026-08-02 - deploy: production v0.1.192

### What
- Released and deployed `v0.1.192` to production via GHCR.
- Includes: group capacity used scoped to group API keys; user management
  "view usage" opens usage page filtered by user.

### Verification
- Release workflow success (retry after Docker Hub timeout); digest
  `sha256:cb885fa24132549ea281e399754924bc601d57d82493712e8901e9aa5edbe5c0`
- `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`, health check passed
- revision `ab308b741`, version `0.1.192`, healthy, `/health` OK

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-02 - fix: group capacity concurrency used is group-scoped

### What
- Group management capacity **concurrency used** now sums live request slots on
  this group's API keys (group-scoped), not account-wide concurrency (shared).
- Sessions/RPM used only count accounts that configure those limits (no longer
  add unlimited-side accounts into used).
- Tooltip clarifies used vs max semantics.

### Why
Shared accounts made every bound group show the account's total occupancy as if
it belonged to that group alone ("缁熻鐨勬槸鎬荤殑").

### Verification
- `go test -tags=unit ./internal/service -run TestGetAllGroupCapacity -count=1`
- `go build ./cmd/server`

### Affected files
`backend/internal/service/group_capacity_service.go`,
`backend/internal/service/group_capacity_service_test.go`,
`backend/internal/repository/group_capacity_api_key_lister.go`,
`backend/internal/repository/wire.go`,
`backend/cmd/server/wire_gen.go`,
`frontend/src/components/common/GroupCapacityBadge.vue`,
`frontend/src/i18n/locales/{zh,en}.ts`, this changelog.

## 2026-08-02 - deploy: production v0.1.191

### What
- Released and deployed `v0.1.191` to production via GHCR.
- Fix: subscription total/avg/usage-rate metrics scoped to current term; rate capped at 100%.

### Verification
- Release workflow success; digest `sha256:8785cde9121abc5d19d010a4b548396ce9c32036d2255acd8e1d672ad684b879`
- `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`, health check passed
- Container healthy, internal `/health` OK

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-02 - fix: subscription usage metrics scoped to current term

### What
- Admin subscription **total consumed** only sums `usage_logs.actual_cost` inside
  the current term `[starts_at, expires_at)` (excludes pre-reactivation history).
- **Daily-limit usage rate** is capped at 100% for display and server sort.
- Sort SQL and i18n hints match the same current-term semantics.

### Why
Reactivating a subscription resets `starts_at` but reuses `subscription_id`, so
lifetime SUM inflated avg daily and pushed usage rate well above 100%.

### Verification
- `go test -tags=unit ./internal/service -run "TestSubscriptionActiveDays|TestEnrichAdminListStats" -count=1`
- `go test -tags=unit ./internal/repository -run "TestIsSubscriptionUsageMetricSort|TestSubscriptionUsageMetricOrder" -count=1`

### Affected files
`backend/internal/repository/usage_log_repo.go`,
`backend/internal/repository/user_subscription_repo.go`,
`backend/internal/service/subscription_service.go`,
`backend/internal/service/user_subscription.go`,
`backend/internal/service/subscription_admin_enrich_test.go`,
`backend/internal/handler/dto/types.go`,
`frontend/src/i18n/locales/{zh,en}.ts`,
`docs/dev/codebase/subscription.md`, this changelog.

## 2026-08-02 - deploy: production v0.1.190

### What
- Released and deployed `v0.1.190` to production via GHCR (`ghcr.io/541968679/sub2api:latest`).
- Includes subscription group user rates, admin subscription usage columns + sort, and usage filter recent/browse dropdowns.

### Verification
- Release workflow success; image revision `a7bf77b73`, version label `0.1.190`
- `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`, health check passed
- `status=running health=healthy`, internal `/health` OK

### Affected files
docs only for this entry: `docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-08-02 - feat: sort subscription usage metric columns

### What
- Admin subscription list can server-sort by **total consumed**, **avg daily**, and **daily limit usage rate**.
- Sorting uses SQL expressions aligned with list enrichment (SUM actual_cost, active-days avg, avg/daily_limit).

### Why
Operators need to rank heavy users / high utilization subscriptions without exporting data.

### Verification
- `go test -tags=unit ./internal/repository -run "TestIsSubscriptionUsageMetricSort|TestSubscriptionUsageMetricOrder" -count=1`

### Affected files
`backend/internal/repository/user_subscription_repo.go`,
`backend/internal/repository/user_subscription_usage_sort_test.go`,
`frontend/src/views/admin/SubscriptionsView.vue`,
`docs/dev/codebase/subscription.md`, this changelog.

## 2026-08-02 - feat: usage filter recent user/account dropdowns

### What
- Admin usage filters for **user** and **account** open a dropdown on focus even with empty input.
- Dropdown order: **recently selected** (localStorage MRU) first, then browse list (users by last_active_at, accounts by last_used_at).
- Dropdown is teleported to `body` with fixed positioning to avoid clipped/incomplete lists inside overflow containers.
- Typing still debounced remote-searches; selection updates MRU.

### Why
Empty-input browse was impossible (search required text), network lag made typeahead flaky, and nested overflow clipped the menu.

### Verification
- `pnpm exec vue-tsc --noEmit`

### Affected files
`frontend/src/components/admin/usage/UsageFilters.vue`,
`frontend/src/composables/useRecentPicks.ts`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`, this changelog.

## 2026-08-02 - feat: subscription-group user rates + admin usage columns

### What
- User group config modal now shows the user's linked **subscription groups**
  and allows editing dedicated billing/display rate overrides (access still
  comes from subscriptions, not `allowed_groups`).
- Admin subscription list returns and displays lifetime total consumed USD,
  average daily usage, daily-limit utilization, and per-user rate overrides.
- Subscription list supports inline edit/save of the user's rate for that
  subscription group via existing `group_rates_full` user update API.

### Why
Admins could not see or edit subscription-group rates from user management,
and subscription ops needed quick visibility into daily-limit utilization
without opening the cost-analysis page.

### Verification
- `go test -tags=unit ./internal/service -run "TestSubscriptionActiveDays|TestEnrichAdminListStats" -count=1`

### Affected files
`backend/internal/service/user_subscription.go`,
`backend/internal/service/subscription_service.go`,
`backend/internal/service/subscription_admin_enrich_test.go`,
`backend/internal/service/wire.go`,
`backend/cmd/server/wire_gen.go`,
`backend/internal/repository/usage_log_repo.go`,
`backend/internal/handler/dto/types.go`,
`backend/internal/handler/dto/mappers.go`,
`backend/internal/handler/admin/subscription_handler.go`,
`frontend/src/components/admin/user/UserAllowedGroupsModal.vue`,
`frontend/src/views/admin/SubscriptionsView.vue`,
`frontend/src/types/index.ts`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
`docs/dev/codebase/subscription.md`, this changelog.

## 2026-08-01 - feat: mobile accordion for account/group three-zone forms

### What
- Account edit and group create/edit three-zone forms now adapt for mobile:
  - Stacked accordion cards with full-width tappable headers + chevron
  - Zone 1 always open; secondary zones collapsible
  - Account **groups** default open on mobile; group **models** and all **other** zones default collapsed
  - PC keeps three-column layout; zone 2 always visible on `lg+`
- Group selector `panel` variant uses shorter max-height on small screens

### Why
Mobile previously forced full expansion of advanced zones, making long forms hard to use.

### Affected files
Frontend: `components/account/EditAccountModal.vue`, `views/admin/GroupsView.vue`, `components/common/GroupSelector.vue`

## 2026-08-01 - feat: group create/edit modal three-zone PC layout

### What
- Admin group **create** and **edit** dialogs use `extra-wide` width and a three-column layout on PC (`lg+`):
  1. **Basic Config** 鈥?name, description, platform, copy accounts, rate/RPM, exclusive, subscription type + USD limits
  2. **Model Control** 鈥?model allow/block lists and `/v1/models` custom list
  3. **Other Features** 鈥?pricing (image/video/web/peak) and platform-advanced options; collapsed by default on PC, always open on mobile
- Opening create/edit resets the Other Features expand state.

### Why
Group create/edit was a long single column mixing daily setup with rare advanced options.

### Affected files
Frontend: `views/admin/GroupsView.vue`, `i18n/locales/{zh,en}.ts`

## 2026-08-01 - feat: account edit modal three-zone PC layout

### What
- Admin **Edit Account** dialog (`EditAccountModal`) uses `extra-wide` width and a three-column layout on PC (`lg+`): Account Config / Group Config / Other Features.
- Mobile keeps a stacked flow; Other Features is always expanded on small screens and collapsed by default on PC.
- Group selector gains platform / subscription-type one-click select chips and a taller `panel` variant.
- Billing rate multiplier and expiry move to Other Features; model restriction and Claude-GPT bridge stay at the top of that zone.

### Why
Long single-column edit form mixed everyday settings with advanced options, making account and group assignment hard to manage.

### Affected files
Frontend: `components/account/EditAccountModal.vue`, `components/common/GroupSelector.vue`, `components/common/__tests__/GroupSelector.spec.ts`, `i18n/locales/{zh,en}.ts`

## 2026-08-01 - feat: remember account list platform and filters

### What
- Admin accounts page restores last **platform**, type, status, privacy, group, and search from `localStorage` on next visit.

### Why
Operators usually work within one platform/filter set; re-selecting every time is friction.

### Affected files
`frontend/src/views/admin/AccountsView.vue`

## 2026-08-01 - feat: account list TTFT p50/p95 quality metrics

### What
- Quality TTFT batch stats now return **p50 / p95 / max** in addition to avg (15m window).
- Account list 鈥滈瀛椻€?cell shows **p50** (primary) + **p95** (tail); tooltip has avg/max/sample count.
- Coloring uses p50 thresholds; p95 uses slightly higher thresholds for tail risk.

### Why
Mean TTFT is easily skewed by one or two pathological requests; median + tail percentiles give a fuller picture.

### Affected files
`backend/internal/service/account_quality.go`, `account_usage_service.go`, `repository/usage_log_repo.go`, tests; `frontend/src/components/account/AccountQualityCell.vue`, `api/admin/accounts.ts`, i18n

## 2026-08-01 - feat: account list open usage by account + hide antigravity usage cards

### What
- Account management actions: **鏌ョ湅浣跨敤璁板綍 / Usage** opens `/admin/usage` in a new tab with `account_id` (+ `account_name`) applied.
- Admin usage page reads `account_id` from query and resolves the account filter label.
- Removed Antigravity ratio card and credit usage curve from admin usage page (no longer needed).

### Why
Jump from an account row into filtered usage logs; drop unused Antigravity dashboard cards.

### Affected files
`frontend/src/views/admin/AccountsView.vue`, `frontend/src/views/admin/UsageView.vue`, `frontend/src/components/admin/usage/UsageFilters.vue`, `frontend/src/i18n/locales/{zh,en}.ts`

## 2026-08-01 - feat: account list inline concurrency, priority, fallback-only

### What
- Admin account table: inline edit **concurrency** (max) and **priority**; toggle **fallback_only** (鍏滃簳璋冨害) like schedulable.
- New/default columns: concurrency, priority, fallback_only visible; one-time column-layout migration unhides priority.
- Backend list sort supports `concurrency`.

### Why
Operators frequently tweak concurrency/priority/fallback without opening the full edit modal.

### Affected files
`frontend/src/views/admin/AccountsView.vue`, `frontend/src/components/account/AccountInlineNumberCell.vue`, `frontend/src/i18n/locales/{zh,en}.ts`, `backend/internal/repository/account_repo.go`

## 2026-08-01 - feat: admin redeem list batch aggregation + split generate modal

### What
- Admin redeem list aggregates codes by generation `batch_id` (legacy codes without batch_id shown as singleton rows).
- Batch detail drawer with per-code status; batch actions: copy unused, delete unused.
- Generate UI is a wide modal: left config, right latest codes (continuous generate overwrites right panel).
- Every generation now always writes `batch_id` (not only when per-user batch limit is enabled).

### Why
Operators sell codes in batches; per-code list rows do not scale. Continuous generation needs a persistent result panel.

### Affected files
Backend: `service/redeem_batch.go`, `service/admin_service.go`, `service/redeem_service.go`, `repository/redeem_code_repo.go`, `handler/admin/redeem_handler.go`, `handler/dto/*`, `server/routes/admin.go`  
Frontend: `views/admin/RedeemView.vue`, `api/admin/redeem.ts`, `types/index.ts`, `i18n/locales/{zh,en}.ts`

## 2026-08-01 - feat: inline redeem on purchase page + redeem buy notice

### What
- 鍏呭€?璁㈤槄椤碉紙`PaymentView`锛夌殑鍏呭€?Tab 涓庤闃?Tab 鍚勫鍔犲唴宓屽厬鎹㈠崱鐗囷紝鐢ㄦ埛鏃犻渶璺宠浆 `/redeem` 鍗冲彲鍏戞崲銆?
- 鍏戞崲椤靛鍔犵鐞嗗憳鍙厤缃殑銆岃喘涔板厬鎹㈢爜銆嶇函鏂囨湰璇存槑锛圫ettings KV `redeem_page.buy_notice`锛夈€?
- 绠＄悊绔€岄〉闈㈠唴瀹广€嶆柊澧炪€屽厬鎹㈤〉銆峊ab锛涘厖鍊煎叕鍛婁笌鍏戞崲璇存槑缁熶竴涓烘俯鍜岀豢鑹插ぇ瀛楀彿妯箙锛堝幓鎺夌孩鑹蹭笌鎰熷徆鍙峰渾鏍囷級銆?

### Why
灏嗗厬鎹㈢爜浣滀负闂存帴鏀粯閫氶亾锛氬悗鍙扮敓鎴愮爜 鈫?澶栭儴閿€鍞珯鍞崠 鈫?鐢ㄦ埛璐悗鍏戞崲锛涘苟鍦ㄥ厖鍊?璁㈤槄鍏ュ彛闄嶄綆鎿嶄綔璺緞銆?

### Affected files
Backend: `handler/admin/redeem_page_handler.go`, `handler/redeem_page_handler.go`, `handler/handler.go`, `handler/wire.go`, `cmd/server/wire_gen.go`, `server/routes/user.go`, `server/routes/admin.go`  
Frontend: `api/redeemPage.ts`, `components/common/SoftNoticeBanner.vue`, `components/user/InlineRedeemCard.vue`, `components/admin/page-content/{Purchase,Redeem}ContentForm.vue`, `views/user/{Payment,Redeem}View.vue`, `views/admin/PageContentView.vue`, `i18n/locales/{zh,en}.ts`, `api/index.ts`

## 2026-07-31 - fix: escape previous_response sticky on fallback-only accounts

### What
- When a `previous_response` sticky target is `fallback_only` and any primary peer is available, release the sticky selection and fall through to primary load-balance.
- Harden `getExtraBool` to accept string/number truthy values for scheduler-critical flags.

### Why
Production residual traffic on safeapi after primary recovery was partly multi-turn previous_response sticky to the fallback account.

### Affected files
`backend/internal/service/openai_account_scheduler.go`,
`backend/internal/service/account.go`,
`backend/internal/service/account_fallback_only_test.go`

## 2026-07-31 - feat: account fallback-only hard scheduling tier

### What
- Added account flag `extra.fallback_only` (**浠呬綔鍏滃簳璋冨害**): selected only when no non-fallback peer remains in the eligible candidate pool.
- OpenAI load-balance and Anthropic/general load-aware selection partition primary vs fallback; session sticky to a fallback account escapes when a primary peer is available.
- Scheduler slim snapshot retains `fallback_only`; admin create/edit toggles write the extra key.

### Why
Soft numeric priority still load-balances low-priority accounts. Operators need a true last-resort upstream that stays unused while any primary channel works.

### Verification
- Unit tests for `IsFallbackOnly`, `preferPrimaryAccounts`, `preferPrimaryOpenAICandidates`.

### Affected files
`backend/internal/service/account.go`,
`backend/internal/service/account_fallback_only_test.go`,
`backend/internal/service/openai_account_scheduler.go`,
`backend/internal/service/gateway_service.go`,
`backend/internal/service/openai_gateway_service.go`,
`backend/internal/repository/scheduler_cache.go`,
`backend/internal/handler/dto/types.go`,
`backend/internal/handler/dto/mappers.go`,
`frontend/src/types/index.ts`,
`frontend/src/components/account/EditAccountModal.vue`,
`frontend/src/components/account/CreateAccountModal.vue`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`

## 2026-07-31 - feat: account list TTFT and success-rate columns

### What
- Added admin account-list columns **棣栧瓧 (TTFT)** and **鎴愬姛鐜?(success rate)** over a **rolling 15-minute** window.
- Batch API `POST /api/v1/admin/accounts/quality-stats/batch` aggregates successes/`first_token_ms` from `usage_logs` and failures from `ops_error_logs` (status 鈮?400, exclude count_tokens).
- Frontend loads metrics only when the columns are visible (default visible), with ~30s snapshot cache and tooltip sample counts.

### Why
Operators need to compare upstream API account quality directly on the account table without leaving for Ops dashboards.

### Verification
- Unit tests for quality stats builder, service batch path, and repo SQL mock.
- Frontend AccountsView tests updated for the new batch API mock.

### Affected files
`backend/internal/service/account_quality.go`,
`backend/internal/service/account_quality_test.go`,
`backend/internal/service/account_usage_service.go`,
`backend/internal/service/account_usage_quality_stats_test.go`,
`backend/internal/repository/usage_log_repo.go`,
`backend/internal/repository/usage_log_repo_quality_stats_test.go`,
`backend/internal/handler/admin/account_handler.go`,
`backend/internal/handler/admin/account_today_stats_cache.go`,
`backend/internal/server/routes/admin.go`,
`frontend/src/api/admin/accounts.ts`,
`frontend/src/components/account/AccountQualityCell.vue`,
`frontend/src/views/admin/AccountsView.vue`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
`frontend/src/views/admin/__tests__/AccountsView.*.spec.ts`,
`.trellis/tasks/07-31-account-quality-columns/*`

## 2026-07-31 - ops: production DB GPT-5.6 Terra/Luna price cut sync

### What
- Updated production `global_model_pricing` for **Terra** (鈭?0%) and **Luna** (鈭?0%) to track OpenAI 2026-07-30 cut.
- Preserved existing platform markup: **display 鈮?1脳 official**, **billing 鈮?2脳 official**.
- Sol / generic `gpt-5.6` left unchanged (official Sol price did not change).
- Also scaled matching **user overrides** that were still on launch-era 1脳 official rates (`user_id=16` terra/luna). Custom Sol overrides (users 1/16/220) left as-is.
- Restarted `sub2api` to reload `GlobalPricingCache`.

### Why
Repo defaults were updated, but production still served old prices from DB global overrides (higher priority than LiteLLM JSON).

### Verification
- Pre/post SQL snapshot in transaction; COMMIT applied.
- `docker compose restart sub2api` 鈫?container running, `{"status":"ok"}` on `/health`.
- Final global display: Terra `$2/$12`, Luna `$0.20/$1.20`; billing Terra `$4/$24`, Luna `$0.40/$2.40` (per MTok).

### Affected
Production DB tables `global_model_pricing`, `user_model_pricing_overrides` (terra/luna only); runtime cache via container restart. No code deploy required for this ops step.

## 2026-07-31 - fix: sync GPT-5.6 Terra/Luna default prices to OpenAI 2026-07-30 cut

### What
- Updated packaged LiteLLM pricing and static billing/pricing fallbacks so GPT-5.6 tiers match OpenAI Standard after the 2026-07-30 price cut.
- **Sol** unchanged: `$5 / $30` input/output per 1MTok.
- **Terra** 鈭?0%: `$2.50/$15` 鈫?**`$2 / $12`** (cache write `$2.50`, cache read `$0.20`).
- **Luna** 鈭?0%: `$1/$6` 鈫?**`$0.20 / $1.20`** (cache write `$0.25`, cache read `$0.02`).
- Split previously shared Sol-level fallback so Terra/Luna no longer resolve to `$5/$30` when dynamic pricing is missing.
- Also fixed resources JSON where Terra/Luna were incorrectly cloned as Sol prices.

### Why
OpenAI reduced Luna by 80% and Terra by 20% on 2026-07-30; local default display/billing prices still used launch-era rates (and Sol fallbacks for all tiers).

### Verification
- `go test -tags=unit ./internal/service -run "OpenAIGPT56|Gpt56" -count=1` (PASS)

### Affected files
`backend/data/model_pricing.json`,
`backend/resources/model-pricing/model_prices_and_context_window.json`,
`backend/internal/service/billing_service.go`,
`backend/internal/service/pricing_service.go`,
`backend/internal/service/billing_service_test.go`,
`backend/internal/service/pricing_service_test.go`,
`docs/dev/codebase/billing.md`,
this changelog.

## 2026-07-31 - deploy: production v0.1.183 (display cache M/伪)

### What
- Released and deployed `v0.1.183` to production via GHCR (`ghcr.io/541968679/sub2api:latest`).
- Image revision `9ad06529a`, version label `0.1.183`, digest `sha256:7e164c3002f4d6dcf16f1ba2f01d1ba98c55c1fbe4f984eb2410af25f9227f8b`.
- Defaults in this build: display cache max mult **M=1.3**, output residual growth ratio **伪=1.5**.

### Why
Ship the bounded cache amplify allocator after Codex OAuth multi-turn validation.

### Verification
- GitHub Actions Release run `30599717871` success
- `bash /opt/sub2api/update.sh --skip-a2 --skip-invokeai`, health check passed
- Container healthy; internal + public `/health` return `{"status":"ok"}`

### Affected files
`docs/dev/DEPLOYMENT.md`, this changelog.

## 2026-07-31 - feat: display cache amplify cap (M) + output residual ratio (伪)

### What
- Display layer: cache_read may amplify up to `M` (`display_cache_token_max_mult`, default **1.3**); residual cache premium prefers **output** under growth ratio `伪` (`display_output_residual_growth_ratio`, default **1.5**), overflow to input.
- Admin Settings: new **灞曠ず灞?/ Display** tab for global M/伪; Claude-GPT bridge cache display controls moved here from Gateway.
- User model pricing modal: optional per-user M override (NULL/empty inherits global).
- Admin usage table: real vs display **cache share** = `cache_read / (input+output+cache_read+cache_creation)`.

### Why
Concentrating amplify on input collapsed visible cache share; unbounded cache amplify filled client context too fast. Bounded M + 伪-capped output residual balances both.

### Verification
- `go test -tags=unit` on `internal/service` (AllocateDisplay*, DisplayToken_*) and `internal/handler/dto` (ApplyDisplayTransform*)
- `go build ./cmd/server`

### Affected files
`backend/internal/service/display_token_alloc.go`, `display_token_rewrite.go`, `model_pricing_resolver.go`, `setting_service.go`, `settings_view.go`, `domain_constants.go`, `admin_service.go`, `user.go`,
`backend/internal/handler/dto/display_pricing.go`, `usage_handler.go`, `gateway_handler.go`,
`backend/internal/handler/admin/setting_handler.go`, `user_handler.go`,
`backend/internal/repository/user_repo.go`,
`backend/migrations/194_display_cache_token_max_mult.sql`,
`backend/ent/schema/user.go` (+ minimal ent user field patches),
`backend/cmd/server/wire_gen.go`,
`frontend/src/views/admin/SettingsView.vue`, `api/admin/settings.ts`,
`frontend/src/components/admin/user/UserModelPricingModal.vue`,
`frontend/src/components/admin/usage/UsageTable.vue`,
`frontend/src/i18n/locales/{zh,en}.ts`,
this changelog.

## 2026-07-30 - feat: purchase page emergency payment notice (page content managed)

### What
Added a prominent red notice banner on the user `/purchase` page that is shared
by both the recharge and subscription tabs. The notice text is managed under
Admin 鈫?椤甸潰鍐呭 鈫?鍏呭€艰闃呴〉 (settings key `purchase_page.notice`). When the
setting has never been saved, the built-in default is shown:
銆岀嚎涓婃敮浠樻笭閬撴殏涓嶅彲鐢紝濡傞渶娴嬭瘯璇疯仈绯诲鏈峷x锛歵qrzfwidc銆? Saving an empty
string disables the banner.

### Why
Online payment channels are temporarily unavailable. Operators need a highly
visible user-facing notice plus an admin-editable control path without waiting
for another deploy when channels recover.

### Verification
- Backend compile of handler package and wire wiring
- Frontend PaymentView loads `/user/purchase-page` alongside checkout info
- Admin Page Content hub exposes the new purchase tab

### Affected files
`backend/internal/handler/admin/purchase_page_handler.go`,
`backend/internal/handler/purchase_page_handler.go`,
`backend/internal/handler/handler.go`,
`backend/internal/handler/wire.go`,
`backend/cmd/server/wire_gen.go`,
`backend/internal/server/routes/admin.go`,
`backend/internal/server/routes/user.go`,
`frontend/src/api/purchasePage.ts`,
`frontend/src/api/index.ts`,
`frontend/src/components/admin/page-content/PurchaseContentForm.vue`,
`frontend/src/views/admin/PageContentView.vue`,
`frontend/src/views/user/PaymentView.vue`,
`frontend/src/views/user/__tests__/PaymentView.spec.ts`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
this changelog.

## 2026-07-30 - fix: preserve Codex Desktop image extension routing

### What
The OpenAI Codex image bridge now detects a declared `image_gen` namespace in
top-level tools or Responses Lite `input[].additional_tools`. For those modern
Codex Desktop requests, the gateway preserves the namespace and matching
namespace tool choice without injecting the legacy flat `image_generation`
tool, forcing `tool_choice: "auto"`, or adding legacy bridge instructions.
Clients without an actual namespace declaration keep the existing hosted
Responses image-tool fallback.

### Why
When both tool forms were forwarded, the model could choose the hosted
Responses image tool instead of the desktop extension. The image bytes then
arrived as a valid base64 `image_generation_call`, but the client extension did
not execute, so no image file or `saved_path` was produced and Codex Desktop
showed no image. The modern extension owns the `/v1/images/generations` call,
local persistence, and display lifecycle.

### Verification
- `go test ./internal/service -run 'Test(EnsureOpenAIResponsesImageGenerationTool|ApplyCodexImageGenerationBridgeInstructions|ApplyCodexOAuthTransform|OpenAIGatewayService_Forward_CodexImageBridge)' -count=1`
- `go test ./internal/service -count=1`
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0 run ./... --timeout=30m`
- `go test -tags=unit ./... -count=1`
- `git diff --check`

### Affected files
`backend/internal/service/openai_codex_transform.go`,
`backend/internal/service/openai_codex_transform_test.go`,
`backend/internal/service/openai_gateway_service_hotpath_test.go`,
`docs/dev/codebase/gateway.md`, this changelog.

## 2026-07-30 - test: wait for canceled image artifact writer shutdown

### What
The manual image cancellation race integration test now waits until the mocked
download response body is closed before allowing its temporary artifact
directory to be cleaned up.

### Why
The canceled status becomes observable before the detached download worker has
fully returned. GitHub Actions could therefore finish the assertion and start
`t.TempDir()` cleanup while the worker was still creating or removing an
artifact file, producing an intermittent `directory not empty` failure even
though the cancellation behavior was correct.

### Verification
- `go test -tags=integration ./internal/service -run TestImageChannelManualCancelWinsAgainstInFlightArtifactCommit -count=20`
- `go test -tags=unit ./...`

### Affected files
`backend/internal/service/image_channel_monitor_manual_core_test.go`, this
changelog.

## 2026-07-30 - chore: clear release security and formatting gates

### What
Upgraded frontend `axios` to `1.19.0` and `postcss` to the patched `8.5.25`
resolution, which also upgrades the axios `form-data` dependency to
`4.0.6`. Removed security exceptions that are no longer needed, renewed the two
unfixed `xlsx` exceptions through 2026-10-30, and applied the pending Go format
fix in the OpenAI compatibility types.

### Why
The previous `main` CI run was blocked by one Go formatting finding and by
frontend audit findings with available patched dependency versions. SheetJS
still has no patched npm release, so its existing admin-export-only mitigation
remains time-bounded instead of being treated as resolved.

### Verification
- `go test -tags=unit ./...`
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0 run ./... --timeout=30m`
- `pnpm install --frozen-lockfile` with pnpm 9.15.9
- `pnpm run typecheck`
- `pnpm run lint:check`
- `pnpm run build`
- `pnpm audit --prod --audit-level=high` plus
  `tools/check_pnpm_audit_exceptions.py`
- `gofmt -d backend/internal/pkg/apicompat/types.go`

### Affected files
`.github/audit-exceptions.yml`,
`backend/internal/pkg/apicompat/types.go`,
`frontend/package.json`, `frontend/pnpm-lock.yaml`, this changelog.

## 2026-07-29 - fix: finalize Codex image bridge results

### What
Responses image-generation output items with a complete base64 `result` are
now normalized to `status: "completed"` when the upstream omits the status or
leaves it as queued/generating/in-progress. The normalization covers streamed
`response.output_item.done` events, terminal response output, reconstructed
SSE-to-JSON output, and direct non-streaming Responses JSON.

### Why
The Codex image bridge successfully generated image bytes, but Codex Desktop
kept the returned item in a generating state and did not render or save the
image because the final status was missing. Explicit failure states and image
items without a result remain unchanged.

### Verification
- `go test -tags=unit ./internal/service -run "NormalizeCompletedImageGeneration|NormalizeResponsesStreamingTerminalOutput|HandleSSEToJSON_ReconstructsImageGeneration"`
- `go test -tags=unit ./internal/service`
- `git diff --check -- <affected files>`

### Affected files
`backend/internal/service/openai_gateway_service.go`,
`backend/internal/service/openai_gateway_service_test.go`,
`docs/dev/codebase/gateway.md`, this changelog.

## 2026-07-29 - feat: bulk edit OpenAI image routing controls

### What
OpenAI OAuth/API-key bulk editing can now independently apply Images endpoint
scheduling and the Codex image tool bridge as explicit enabled/disabled account
overrides. Leaving either apply checkbox clear preserves every selected
account's current value.

### Why
These controls already existed in single-account editing, but administrators
had to update image-capable OAuth account pools one account at a time. The
bridge bulk control omits "inherit global" because the existing incremental
JSONB merge API cannot remove an account-level override safely.

### Verification
- `pnpm run test:run -- BulkEditAccountModal` (19 tests passed)
- `pnpm run typecheck`
- `pnpm run lint:check`
- `git diff --check -- <affected files>`

### Affected files
`frontend/src/components/account/BulkEditAccountModal.vue`,
`frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`,
`docs/dev/codebase/account.md`, this changelog.

## 2026-07-29 - fix: preserve Chat Completions structured outputs on OAuth CC鈫扲esponses

### What
Chat Completions `response_format` is no longer dropped when converting to the
Responses API for OAuth / GPT Pro accounts. `json_schema` and `json_object`
now map to Responses `text.format`, preserving name/strict/schema payload so
strict structured-output probes (e.g. hvoy schema constraint adherence) receive
a constrained upstream request instead of unconstrained free-form text.

### Why
The OAuth path always converts Chat Completions 鈫?Responses via
`ChatCompletionsToResponses`. `ChatCompletionsRequest` lacked
`response_format` and `ResponsesText` lacked `format`, so structured-output
constraints were silently discarded before Codex OAuth transform. Detectors
that require pure schema-adherent JSON therefore failed even on GPT Pro 20x.

### Affected files
`backend/internal/pkg/apicompat/types.go`,
`backend/internal/pkg/apicompat/chatcompletions_to_responses.go`,
`backend/internal/pkg/apicompat/chatcompletions_to_responses_structured_output_test.go`,
`backend/internal/service/openai_codex_transform_structured_output_test.go`,
this changelog.

## 2026-07-29 - perf: lower local frontend dev memory defaults

### What
Local Vite dev no longer enables `vite-plugin-checker` by default (it previously
ran both `typescript` and `vueTsc`, roughly doubling language-server cost).
Checker is opt-in via `VITE_DEV_CHECKER=1` or `pnpm run dev:check` (vue-tsc only).
Dev server defaults to `127.0.0.1`, disables `preTransformRequests`, and tightens
file-watch ignores. Documented local RSS breakdown and lite workflows in
`DEV_GUIDE.md`.

### Why
Measured local stack: Vite alone ~2GB working set on this codebase; backend
`server.exe` ~120MB; PostgreSQL host ~360MB. Frontend was the blocker for light
local debugging.

### Affected files
`frontend/vite.config.ts`,
`frontend/package.json`,
`frontend/scripts/dev-with-checker.mjs`,
`DEV_GUIDE.md`,
this changelog.

## 2026-07-29 - fix: user dashboard quick-actions grid with few visible cards

### What
Made the user dashboard "Quick Actions" primary row use 1/2/3 columns based on
how many cards are actually visible (top-up and tutorial are optional).

### Why
After an empty host-DB install, `payment_enabled=false` and empty `tutorial_url`
left only "Get API Key" in a hard-coded `sm:grid-cols-3` grid, so the lone card
sat in the first column and looked broken. This is settings-driven visibility,
not a Docker volume layout bug 鈥?but empty defaults make it show up.

### Affected files
`frontend/src/components/user/dashboard/UserDashboardQuickActions.vue`,
this changelog.

## 2026-07-29 - fix: login success stuck on login page (legal consent race)

### What
Fixed a race where `applySettings` 鈫?`enforceLegalConsentSettings` cleared the
just-established session because legal consent was not yet accepted. Fresh
logins now keep the session so `LegalConsentDialog` can complete; force-logout
only runs when a *prior* consent version is stale. Admin login default redirect
is `/admin/dashboard`.

### Why
After login API succeeded, public-settings application wiped tokens, so
navigation to the dashboard bounced back to `/login`.

### Affected files
`frontend/src/utils/legalConsent.ts`,
`frontend/src/stores/auth.ts`,
`frontend/src/views/auth/LoginView.vue`,
`frontend/src/utils/__tests__/legalConsent.spec.ts`,
`frontend/src/stores/__tests__/auth.spec.ts`,
this changelog.

## 2026-07-29 - fix: empty host Postgres blocked login (no admin user)

### What
After switching formal local DBs from Docker to host `postgresql-x64-16`, the
shared `sub2api` database had all migrations but **zero users**, so
`POST /api/v1/auth/login` always returned 401. Seeded the local admin from
`backend/config.yaml` (`admin@sub2api.local` / `admin123456`), and added missing
`payment_orders.credit_amount` (Ent field without a prior SQL migration) plus
migration `193_add_payment_order_credit_amount.sql`.

### Why
Setup only creates the admin during the setup wizard / auto-setup path. A
freshly provisioned host DB with migrations alone leaves `users` empty.

### Verification
- `POST http://127.0.0.1:18081/api/v1/auth/login` 鈫?200 with admin tokens
- Same path via Vite proxy `15174` 鈫?200

### Affected files
`backend/migrations/193_add_payment_order_credit_amount.sql`, this changelog
(DB row seed is local-only, not committed).

## 2026-07-29 - fix: Vite dev public-settings inject less noisy / more reliable

### What
Pointed the local Vite proxy at `http://127.0.0.1:18081` (not `localhost` /
historical `8080`), added loopback fetch fallbacks when injecting
`window.__APP_CONFIG__`, and rate-limited the "鏃犳硶鑾峰彇鍏紑閰嶇疆" warning so a
temporarily down backend no longer floods the console on every HTML request.

### Why
When PostgreSQL/backend were unavailable, each page load hit
`inject-public-settings` and logged `fetch failed` / `afterConnectMultiple`.
On Windows, Node `localhost` dual-stack races also make this flakier than
`127.0.0.1`.

### Affected files
`frontend/.env.development.local`, `frontend/vite.config.ts`,
`frontend/vite.config.js`, this changelog.

## 2026-07-29 - fix: CCS import no longer false-reports "not installed"

### What
Fixed API Keys "Import to CCS" so a successful `ccswitch://` launch no longer
shows the error "CC-Switch is not installed or the protocol handler is not
registered". The page now opens the deeplink via a same-document anchor click
and shows an optimistic success toast (with a soft fallback hint). Grok鈫扖odex
still shows the existing metadata catalog warning.

### Why
The previous detector used `document.hasFocus()` after only 100ms. On Windows,
CC-Switch often imports successfully without stealing browser focus, so the
heuristic reported failure even when import worked. Custom protocol success
cannot be detected reliably from the page.

### Affected files
`frontend/src/views/user/KeysView.vue`,
`frontend/src/utils/ccswitchImport.ts`,
`frontend/src/utils/__tests__/ccswitchImport.spec.ts`,
`frontend/src/i18n/locales/zh.ts`,
`frontend/src/i18n/locales/en.ts`,
this changelog.

## 2026-07-27 - sync: import upstream Responses item-ID sanitization

### What
Imported upstream commits `c5d9d5794` and `1891faa68`, including their required
call-input type helper behavior from `fd64d07e6`. Invalid replayed message and
call-input IDs are now removed before OpenAI API-key Responses forwarding, and
the OAuth Codex filter uses the same predicate. The sanitizer rebuilds the
`input` array once so allocation growth stays linear for long conversations.

### Why
Production rejected a replayed `custom_tool_call.id=item_...` because the
upstream expected the custom-tool namespace. The synchronized upstream behavior
removes that foreign ID instead of fabricating an upstream object reference,
while preserving `call_id` for call/output pairing.

### Compatibility And Verification
- Reconciled the upstream change into the fork's current
  `openai_gateway_service.go::Forward`; the obsolete deleted
  `openai_gateway_forward.go` was not restored.
- No local `ctc_` generation enhancement was added. This is the upstream
  sanitizer behavior only.
- Focused API-key/OAuth item-ID and linear-allocation tests passed.
- The committed-range upstream sync guard from `b39f5fe01` passed.
  `go test -tags=unit ./... -count=1` passed across all backend packages;
  `internal/service` completed in 100.218 seconds.
- Billing/display tokens, cache-read quantities, `actual_cost`, model lists,
  Claude-GPT, Images/Batch Image, scheduling/failover, Ops, settings,
  migrations, frontend i18n, and routes are unchanged.

### Affected files
`backend/internal/service/openai_codex_transform.go`,
`backend/internal/service/openai_gateway_service.go`,
`backend/internal/service/openai_responses_item_id.go`,
`backend/internal/service/openai_gateway_apikey_item_id_test.go`,
`docs/dev/UPSTREAM_SYNC.md`, `docs/dev/codebase/gateway.md`, this changelog,
and the incident debug report.

## 2026-07-27 - test: cover production Claude-GPT HTTP 413 compact recovery

### What
Added regression coverage for the production context-overflow response shape:
HTTP 413 with `invalid_request_error`, the `Prompt is too long` message, and no
`error.code`. The tests cover both ordinary Claude-GPT generation and a
client-initiated compact request that must enter server-side chunk recovery.
A negative case verifies that an unrelated HTTP 413 byte-limit error is not
misclassified as a context-window overflow.

### Why
The first production HTTP 413 looked like an unhandled upstream response when
viewed in isolation. The complete production sequence showed that Sub2API had
normalized it into Claude Code's reactive-compact contract, Claude Code sent a
compact request, Sub2API recovered that request, and both compact and the
compressed generation retry returned HTTP 200. Explicit 413 fixtures prevent
future changes from accidentally covering only the historically observed 400
shape.

### Verification
- Read-only production logs for one affected session showed generation HTTP
  413 at `2026-07-27 09:50:55 +08:00`. The same API key sent a compact request
  one second later; the bridge logged chunk and merge recovery before returning
  200, followed by a smaller compressed-generation retry.
- The compact path logged `openai_messages.compact_recovery_started`, a chunk
  attempt, and a merge attempt before returning 200.
- A fresh local Claude Code 2.1.220 session against `127.0.0.1:18081` reached
  the real GPT OAuth limit on round four: generation body `1,293,847` bytes ->
  HTTP 413; client `source=compact` body `984,388` bytes -> HTTP 200 in 31.166s;
  automatic compressed retry body `345,862` bytes -> HTTP 200 in 3.284s; final
  CLI output was exactly `ACK-R413-4`.
- The session persisted `compact_boundary` with `trigger=auto`,
  `isCompactSummary=true`, and the final ACK after compaction.
- Focused regression tests for the exact production envelope, compact recovery,
  and non-context HTTP 413 misclassification passed.
- `go test -tags=unit ./... -count=1` passed; `internal/service` completed in
  135.959 seconds.
- `go test -tags=integration ./... -count=1` passed; `internal/service`
  completed in 88.376 seconds.
- `golangci-lint` 2.9.0 reported `0 issues`; the CGO-disabled production build
  completed successfully.

### Affected files
`backend/internal/service/openai_gateway_messages_prompt_too_long_test.go`,
`backend/internal/service/openai_gateway_messages_compact_test.go`,
`docs/dev/codebase/gateway.md`, and this changelog.

## 2026-07-27 - docs: diagnose Responses custom-tool item ID rejection

### What
Documented the production `/responses` HTTP 400 caused by replaying a
`custom_tool_call` with a generic `item_...` ID into the OpenAI OAuth upstream.
The investigation traced the full loop from Chat Completions -> Responses
fallback output generation, through client history replay and Codex OAuth input
filtering, to the upstream `ctc` namespace validation.

### Why
The error looked superficially like an upstream account or model problem, but
production logs and code inspection show a protocol compatibility defect. The
fallback bridge generates `item_<24 hex>` for custom tool calls, while tool
continuation preserves that ID and the upstream requires `ctc...`. Recording
the boundary prevents account rotation, quota changes, or model remapping from
being used as ineffective mitigations.

### Verification
- Read-only production inspection confirmed one request at
  `2026-07-27 09:53:03 +08:00`: `/responses`, `gpt-5.6-sol`, OpenAI OAuth
  account `1633`, HTTP 400, request body size 327,105 bytes, with
  `input[100].id=item_643ea17567eacddfb6003ee2` rejected in favor of `ctc`.
- The same account served another `gpt-5.6-sol` Responses request successfully
  immediately afterward, excluding account/token/model-wide failure.
- The rejected ID exactly matches the local `generateItemID()` shape, and the
  fallback bridge uses that helper for `custom_tool_call` in both streaming and
  non-streaming output.
- Focused existing Codex transform unit tests passed and confirmed the current
  continuation/reference-preservation behavior.

### Affected files
`docs/dev/codebase/gateway.md`,
`memory/2026-07-27-responses-custom-tool-item-id-debug-report.md`, and this
changelog. No business code, configuration, production state, push, or deploy
was changed.

## 2026-07-26 - fix: restore release quality gates before production deploy

### What
Restored the local and GitHub release gates uncovered while preparing the
Claude-GPT compaction changes for production:

- upgraded `golang.org/x/text` from `v0.37.0` to `v0.39.0` and aligned the
  related `golang.org/x/*` modules, removing the reachable `GO-2026-5970`
  vulnerability;
- scoped two unit-only service tests with the `unit` build tag and repaired
  the account repository integration test recorder setup/isolation;
- made the default public TLS fingerprint capture integration skip only when
  its TCP endpoint is unavailable, while custom capture URLs and reachable
  fingerprint mismatches remain strict failures;
- restored repository scheduler consistency by suppressing no-op temporary
  unschedule side effects, synchronizing model-rate-limit clears into the
  scheduler cache, and excluding Spark shadow accounts from CRS mappings;
- aligned the best-effort usage-log queue-full integration contract with the
  existing backpressure behavior so billing records wait for drain until the
  request context is canceled instead of silently dropping under load;
- aligned the synchronous batched usage-log queue-full integration contract
  with the same bounded backpressure behavior, preventing the full integration
  suite from waiting forever on a deliberately saturated queue;
- brought the real `golangci-lint v2.9.0` gate to zero issues by checking
  close/type-assertion/builder results, applying the configured formatter, and
  removing helpers that had no production callers (including an unconnected
  Grok quota auto-pause implementation);
- aligned stale Claude-GPT template, Grok CLI, and Grok Codex catalog tests
  with the current product contracts; and
- added the missing Chinese and English Grok OpenAI-group-access strings used
  by both account create and edit forms.

### Why
The production release preflight found that the previous fork `main` CI could
not compile several integration packages, the full frontend suite had five
regressions, and `govulncheck` found an actually reachable infinite-loop
vulnerability through `golang.org/x/text`. Those are release blockers even
though the focused Claude-GPT bridge tests and real long-context OAuth test had
already passed.

### Verification
- The five previously failing frontend files passed: 5 files / 34 tests.
- Focused service unit and integration tests passed.
- The repository integration test passed against the real local PostgreSQL
  test transaction: `TestTempUnschedulableFieldsLoadedByGetByIDAndGetByIDs`.
- The four repository release-gate regressions were reproduced independently
  and then passed against real testcontainers PostgreSQL/Redis after the fix.
- The full integration run exposed the stale synchronous queue-full fixture;
  its bounded cancellation regression, the repository integration package,
  and the complete integration suite passed after correction.
- `golangci-lint v2.9.0 run ./...` reports `0 issues`.
- `govulncheck ./...` reports `0` reachable vulnerabilities.
- From final pre-release HEAD, the complete unit and integration suites passed
  with `-count=1`; the integration run used real testcontainers PostgreSQL
  18.1 and Redis 8.4, including the bounded queue-saturation regressions.
- All 155 frontend Vitest files / 892 tests passed, followed by frontend
  typecheck, ESLint, and the production Vite build.
- Both the static backend build and the release-style `embed` build with
  injected `0.1.176` version/commit metadata passed.
- A repository-script-managed local restart reported backend `18081`, frontend
  `15174`, PostgreSQL, and Redis ready. Real HTTP checks returned 200 for
  `/health`, public settings, and the frontend login page; `/v1/messages`
  returned the expected 401 through the registered gateway auth chain for
  missing and invalid keys. Startup logs showed no panic, fatal, or migration
  error, and the hidden Anthropic bridge auto-compact default remained false.
- The host/proxy path returned malformed compressed npm audit responses even
  under Node 20 / pnpm 9.15.9, so the GitHub `Security Scan` workflow remains
  the authoritative frontend audit gate before tagging and deployment.
- Full release-gate verification is recorded by the deployment entry after the
  production rollout completes.

### Affected files
`backend/go.mod`, `backend/go.sum`, service/repository test files under
`backend/internal`, affected frontend component/locale tests and locale files,
and this changelog.

## 2026-07-26 - fix: close Claude-GPT reactive context compaction loop

### What
Claude-GPT bridge generation overflow now returns the Claude Code-recognized
HTTP 413 `invalid_request_error` contract with a stable `Prompt is too long`
message. The contract covers direct HTTP failures plus buffered and streaming
Responses terminals before visible output. Hidden pre-generation auto-compact
is now opt-in by default.

Compact recovery also has local convergence budgets: 24,000 runes for the
client compact prompt used during merge, 24,000 runes per chunk summary, and
48,000 runes per intermediate/final merge summary. Oversized text preserves
the head and tail with an explicit omission marker.

### Why
Mapped GPT/Codex windows can reject a 1M-advertised Claude Code conversation
before Claude Code's preventive compact threshold. The previous ordinary 400
did not trigger reactive compact. Real OAuth testing then exposed a second
failure: Codex removes unsupported `max_output_tokens`, so recovery summaries
could echo hundreds of thousands of characters and exceed the client's
300-second first-event timeout.

### Compatibility And Verification
- Non-bridge Messages keep their existing 400 behavior. A bridge stream that
  already emitted visible output terminates without the prompt-too-long marker,
  preventing replay of a partial answer. Existing client compact recovery,
  account failover, usage aggregation, stored billing, quota deduction,
  display-token accounting, cache-read quantities, and `actual_cost` remain
  authoritative.
- TDD RED/GREEN checkpoints cover direct HTTP, buffered/streaming SSE,
  code/message detection, passthrough priority, non-bridge behavior, partial
  output, keepalive, and compact prompt/chunk/merge budgets.
- A real Claude Code 2.1.220 session through local `127.0.0.1:18081` and a real
  GPT/Codex OAuth upstream completed the full chain: 1,285,010-byte generation
  -> HTTP 413 -> 909,560-byte compact -> HTTP 200 in 43.420s ->
  `compact_boundary`/`isCompactSummary=true` -> automatic generation -> exact
  `ACK-F3`. A separate 1.22 MiB recovery test exercised seven real GPT chunks,
  recursive merge, and exact `ACK-3`.
- Full backend verification passed with
  `go test -tags=unit ./... -count=1` (`internal/service` 97.707s,
  `internal/handler` 25.349s), plus focused compact/prompt-too-long tests and
  `git diff --check`.

### Affected files
`backend/internal/config/config.go`,
`backend/internal/handler/openai_gateway_handler_test.go`,
`backend/internal/service/openai_gateway_messages.go`,
`backend/internal/service/openai_gateway_messages_compact.go`,
`backend/internal/service/openai_gateway_messages_compact_test.go`,
`backend/internal/service/openai_gateway_messages_prompt_too_long_test.go`,
`deploy/config.example.yaml`, `docs/dev/codebase/gateway.md`, this investigation,
and this changelog.

## 2026-07-25 - docs: capture Claude-GPT context compaction investigation

### What
Recorded the production failure, the boundary between client-initiated compact
and fork-local hidden pre-generation auto-compact, the verified Claude Code
prompt-too-long behavior, rejected client-configuration workarounds, and the
candidate reactive compact error-contract fix.

### Why
Claude Code can advertise a larger context window than the mapped GPT/Codex
upstream accepts. The bridge currently returns the upstream
`context_length_exceeded` message as an ordinary 400 that Claude Code does not
classify as `prompt_too_long`, so its own visible compact/retry flow does not
start.

### Scope
- Documentation only; no runtime code, configuration, production service, or
  deployment was changed.
- Compared upstream `main` at `2e2638c01`, merged context-error PRs #3548,
  #3859, #3868, #3870, and #3873, plus open PRs #3808 and #4756. Upstream fixes
  error swallowing/failover/configurable passthrough but has no merged Claude
  Code reactive-compact error contract; #4756 is the same hidden adapter-side
  compaction design and remains default-off upstream.
- Clarified that the fork has stronger client-initiated compact recovery than
  upstream, while its direct 400 with the original context-window message is
  the remaining compatibility gap rather than a missed upstream patch.
- This was the state on 2026-07-25. The 2026-07-26 entry above supersedes it:
  the runtime contract and real long-conversation end-to-end test are complete.

## 2026-07-25 - fix: record actual OpenAI upstream endpoints in error logs

### What
Ops error logs now prefer the runtime OpenAI/Grok upstream endpoint over the
platform-level default. The OpenAI Responses-to-Chat compatibility path records
`/v1/chat/completions` before sending upstream, while native HTTP, passthrough,
and Responses WebSocket paths reset the runtime value to `/v1/responses` (with
HTTP subpaths such as `/compact` preserved).
The admin error-log list renders differing endpoints as an explicit
`inbound -> upstream` mapping instead of hiding the upstream value in a tooltip.

### Why
The generic endpoint derivation treated every OpenAI text request as native
Responses. A downstream `/v1/responses` request converted to upstream
`/v1/chat/completions` therefore appeared in `ops_error_logs.upstream_endpoint`
as `/v1/responses`, including `Unsupported content type` failures. A stale
runtime value could also survive an in-request account switch unless each
actual transport overwrote it.

### Compatibility And Verification
- `inbound_endpoint` remains the normalized downstream client endpoint;
  `upstream_endpoint` now reflects the endpoint selected for the actual OpenAI
  upstream attempt.
- Recovered failover logs persist the normalized endpoint on each upstream
  error event and attribute the top-level row to the last failed attempt, even
  when a later account succeeds through a different endpoint.
- Requests rejected before an upstream transport is selected continue using
  the existing best-effort platform derivation.
- Routing, conversion, account selection, billing, quota deduction, usage
  accounting, and stored request/response bodies are unchanged.
- Verified focused endpoint/error reproductions and the complete unit-tagged
  `internal/handler` and `internal/service` test packages, the backend-wide
  unit suite, the focused error-table component test, frontend typecheck, lint,
  and production build.

## 2026-07-25 - feat: expose OpenAI API-key Responses upstream routing

### What
Added an account-level HTTP Responses route selector to the OpenAI API-key
create and edit forms. Admins can follow automatic capability probing, force
native upstream `/v1/responses`, or force the `/v1/chat/completions`
compatibility bridge. The edit form also shows the persisted automatic probe
result.

### Why
An upstream can temporarily fail or return an obsolete capability probe result.
Admins previously had no UI path to use the backend's existing
`openai_responses_mode` override, leaving valid native Responses endpoints stuck
behind the Chat compatibility conversion.

### Compatibility And Verification
- Manual mode is stored in `accounts.extra.openai_responses_mode` and takes
  precedence over `openai_responses_supported`; choosing Auto removes the
  override and resumes probe-based routing.
- Automatic probes may continue updating their own result but cannot overwrite
  an explicit manual mode.
- WebSocket mode, endpoint scheduling, model mapping, billing, usage recording,
  cache-read quantities, Images, Compact, and Claude-GPT behavior are unchanged.
- Verified focused create/edit component tests, backend routing-priority tests,
  frontend typecheck, lint, and production build.

## 2026-07-25 - ops: disable production Claude-GPT pre-generation auto-compact

### What
Disabled the fork-local hidden pre-generation Claude-GPT bridge compact pass in
production by setting `gateway.anthropic_bridge_auto_compact_enabled: false` in
the persisted `/app/data/config.yaml`, then restarted only the Sub2API service.
Client-initiated Claude Code compact handling and recovery remain enabled.

### Why
The bridge-side pass synchronously called `/responses/compact` before ordinary
generation without exposing that phase to the client. This could inflate
time-to-first-token, repeat on stateless history replays, and add an upstream
compact call that the client did not initiate.

### Verification
- Backed up the prior production config to
  `/opt/sub2api/config.yaml.before-auto-compact-disable.20260725-092702.bak`.
- Confirmed the running container reads the persisted value as `false`.
- Production Sub2API returned to `running` / `healthy` with restart count `0`;
  internal `/health` returned `{"status":"ok"}`.
- Startup log scan found no config load error, fatal error, or panic, and no
  hidden `anthropic bridge history compacted` event appeared after restart.

## 2026-07-25 - deploy: production Sub2API `v0.1.175`

### What
Deployed the Claude-GPT stable cache-session and non-overlapping cache-write
accounting fix to production using the GitHub Actions-built GHCR image.

### Deploy
- Tag: `v0.1.175`
- Commit: `80d9fd818ed248458772335df802fd691f6db6e5`
- Image: `ghcr.io/541968679/sub2api:latest` (version label `0.1.175`)
- Image digest / running image ID:
  `sha256:9122cd929b70eb99fdef46f495d3cf178bbd858f35f35a39d85e02351642a38d`
- Release: https://github.com/541968679/sub2api/releases/tag/v0.1.175
- Release workflow: https://github.com/541968679/sub2api/actions/runs/30150931815
- Prod: running, healthy, restart count `0`; internal and public `/health`
  returned `{"status":"ok"}`
- Rollback pointer: restored to the verified `v0.1.174` GHCR digest
  `sha256:2f3101aa66bdabd47eb00eaf433ef52d3fe5f92d41e2f8acb661c20319f0a427`

### Notes
The production deployment pulled the published GHCR image and recreated only
the Sub2API service; AIClient2API and InvokeAI were intentionally skipped. The
post-deploy log scan found no panic, fatal error, migration failure, or
database/Redis startup failure.

## 2026-07-25 - fix: preserve Claude-GPT cache sessions and separate cache-write display tokens

### What
Claude-GPT bridge requests now retain the deterministic API-key-isolated
`session_id` sent upstream. The bridge cache display override calculates its
random cache-read share from `max(upstream input_tokens - cache_write_tokens, 0)`
instead of the full upstream input count.

### Why
The bridge previously generated a stable session header and then deleted it
immediately before dispatch, causing repeated cold-cache writes. It also allowed
locally generated cache-read tokens to overlap real upstream cache-write tokens,
which could force displayed ordinary input to zero and make the token breakdown
arithmetically impossible.

### Verification
- Focused unit tests first reproduced both defects, then passed after the fix.
- Broader Claude-GPT/cache-write service tests and `internal/pkg/apicompat`
  cache tests passed; `go build ./cmd/server` passed.
- Two identical real bridge requests through account `3007` returned upstream
  `input_tokens=54883`, cache write `0`, and real cache read `3840 -> 54016` with
  the same non-empty session hash. Stored usage rows `17191/17192` preserved the
  invariant `input + cache_read + cache_write = upstream input`.

### Files
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/service/openai_gateway_messages_compact.go`
- `backend/internal/service/openai_compat_model_test.go`
- `docs/dev/codebase/billing.md`
- `docs/dev/codebase/gateway.md`
- `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md`

## 2026-07-25 - deploy: production Sub2API `v0.1.174`

### What
Deployed the `claude-opus-5` model wiring and forced 1M context beta support for
Opus 4.8/5 to production using the GitHub Actions-built GHCR image.

### Deploy
- Tag: `v0.1.174`
- Commit: `fc543d1503b06a2b4c2e2eddacfcfc5ea41fc96e`
- Image: `ghcr.io/541968679/sub2api:latest` (version label `0.1.174`)
- Running image ID: `sha256:2f3101aa66bdabd47eb00eaf433ef52d3fe5f92d41e2f8acb661c20319f0a427`
- Release: https://github.com/541968679/sub2api/releases/tag/v0.1.174
- Release workflow: https://github.com/541968679/sub2api/actions/runs/30143663118
- Prod: running, healthy, restart count `0`; internal and public `/health`
  returned `{"status":"ok"}`
- Migration: `192_add_opus5_to_default_model_mapping.sql` applied; the persisted
  platform mapping (`1/1`) and Antigravity account mappings (`3/3`) include
  `claude-opus-5`

### Notes
The Release workflow passed. The repository CI and Security Scan workflows are
not fully green: they still report the integration-helper and external TLS
capture failures seen on `v0.1.173`, plus the existing Go/dependency advisory
baseline. This feature did not change the failing helpers or dependency
manifests, and no deployment failure was observed.

---
## 2026-07-25 - feat: force 1M context beta for Opus 4.8 / Opus 5

### What
Sub2API now always injects `context-1m-2025-08-07` for Claude Opus 4.8 and
Opus 5 families, regardless of whether the client sends that beta header or
uses a `[1m]` model-name suffix. Model IDs exposed to users stay clean
(`claude-opus-5`, `claude-opus-4-8`) 鈥?1M is a transport beta, not a public
model slug.

Injection covers Anthropic OAuth/API-key, API-key passthrough (incl. Kiro),
Vertex Anthropic, Bedrock body `anthropic_beta`, count_tokens, and Antigravity
upstream passthrough. Default beta-policy whitelist for `context-1m` also
includes these Opus families so policy evaluation does not strip the forced
token.

### Why
Claude Code treats bare `claude-opus-5` as a 200k client-side window unless the
1M beta is present on the request. Operators want Opus 4.8/5 to always use the
1M window without requiring users to pick `鈥1m]` models or edit local config.

### Files
- `backend/internal/pkg/claude/constants.go` (+tests)
- `backend/internal/service/gateway_service.go` (+tests in gateway_beta_test.go)
- `backend/internal/service/antigravity_gateway_service.go`
- `backend/internal/service/settings_view.go`

### Note
Client UI compact meters may still show 200k if the CLI maps bare model IDs to
200k locally; server-side upstream requests for Opus 4.8/5 still carry the 1M
beta after this change.

## 2026-07-25 - feat: expose claude-opus-5 in account test model lists

### What
Completed the `claude-opus-5` model wiring that previously only had pricing
data. Account-test model dropdowns, Antigravity default/strict mapping,
Bedrock default mapping, curated gateway discovery, frontend whitelist
presets, and OpenAI Claude鈫扜PT bridge template now include `claude-opus-5`.

### Why
Pricing already had `claude-opus-5` in `backend/data/model_pricing.json`, but
`GET /api/v1/admin/accounts/:id/models` builds the test dropdown from
`claude.DefaultModels` / platform default mapping keys 鈥?not from the pricing
file. Without those curated lists, the model was billable but not selectable
for account connection tests.

### Files
- `backend/internal/pkg/claude/constants.go` (+tests)
- `backend/internal/domain/constants.go` (+tests)
- `backend/internal/pkg/antigravity/claude_types.go` (+tests)
- `backend/internal/pkg/antigravity/request_transformer.go`
- `backend/internal/service/models_list_policy.go` (+tests)
- `backend/internal/service/antigravity_model_mapping_test.go`
- `backend/internal/handler/gateway_models_list_test.go`
- `backend/migrations/192_add_opus5_to_default_model_mapping.sql`
- `backend/resources/model-pricing/model_prices_and_context_window.json`
- `frontend/src/composables/useModelWhitelist.ts` (+tests)
- `frontend/src/components/admin/account/accountModelSort.ts` (+tests)
- `frontend/src/components/account/AccountStatusIndicator.vue`
- `frontend/src/components/account/AccountUsageCell.vue`
- `frontend/src/components/keys/UseKeyModal.vue`

## 2026-07-24 - feat: Claude鈫扜PT bridge pre-generation auto-compact (v1)

### What
Server-side auto-compact for oversized Anthropic Messages 鈫?OpenAI Responses
bridge history **before** generation, so Opus/Haiku鈫抔pt-5.* long sessions no
longer depend on Claude Code client compact alone.

Release/deploy hardening for this production push:

- Default production compose image is now `ghcr.io/541968679/sub2api:latest`.
- `deploy/update.sh` now refuses non-GHCR Sub2API images, pulls the published
  GHCR image, records the previous GHCR digest for rollback, and never runs
  `docker build` for the main service on the production host.

Aligned with upstream PR #4756, with fork adaptations:

1. Gate on **mapped upstream** model (`gpt-5*`), not Claude display names.
2. Skip when the client already initiated a compact request.
3. Compact model from account compact mapping (no global `OpenAICompactModel`).
4. Fail-open: compact errors leave the original generation body unchanged.
5. Merge compact usage into generation usage for billing.
6. Default **enabled** (upstream PR defaulted off); threshold 512KiB; timeout 600s.

### Why
Production context-window failures on Claude鈫扜PT were mostly generation-time
HTTP 400 (`context_length_exceeded`). Client compact only runs when Claude Code
decides to compact; generation path had no pre-gen shrink. Local synthetic tests
did not reproduce production Opus loads; this is the reliable server-side fix.

### Config
- `gateway.anthropic_bridge_auto_compact_enabled` (default `true`)
- `gateway.anthropic_bridge_auto_compact_input_bytes` (default `524288`, min `65536`)
- `gateway.anthropic_bridge_auto_compact_timeout_seconds` (default `600`, range `30-1800`)
- Env: `GATEWAY_ANTHROPIC_BRIDGE_AUTO_COMPACT_*`

### Files
- `backend/internal/config/config.go` (+tests)
- `backend/internal/service/openai_anthropic_bridge_compact.go` (+tests)
- `backend/internal/service/openai_gateway_messages.go` (hook after GetAccessToken)
- `deploy/.env.example`, `deploy/config.example.yaml`, `deploy/docker-compose.yml`
- `deploy/update.sh`, `docs/dev/DEPLOYMENT.md`

### Verify
- `go test -tags=unit ./internal/config -run 'TestLoad.*AnthropicBridgeAutoCompact' -count=1`
- `go test -tags=unit ./internal/service -run 'TestAnthropicBridge|TestMaybeAutoCompact|TestIsMappedGPT5|TestForwardAsAnthropicAutoCompact' -count=1`
- Real local Claude Code CLI (`claude.exe` 2.1.211) two-turn request through
  local Sub2API: 560288-char seed returned `default-seed-ok`; follow-up returned
  `ZC-DEFAULT-AUTOCOMPACT-CANARY-20260724`; backend logged
  `openai messages: anthropic bridge history compacted` with input bytes reduced
  from 620842 to 247542 before generation.
- `bash -n deploy/update.sh`

### Notes
- OAuth OpenAI only; skips Grok, API-key, non-gpt-5 mapped models, client compact.
- Disable in emergency: `GATEWAY_ANTHROPIC_BRIDGE_AUTO_COMPACT_ENABLED=false`.

---
## 2026-07-24 - deploy: production Sub2API `v0.1.173`

### What
Deployed Claude-GPT bridge pre-generation auto-compact and GHCR-only deployment
hardening to production.

### Deploy
- Tag: `v0.1.173`
- Commit: `8ca41688ff7e61d75c0cefe2401231cfb5f6eb22`
- Image: `ghcr.io/541968679/sub2api:latest` (version label `0.1.173`)
- Image digest: `ghcr.io/541968679/sub2api@sha256:c4b76d93d79f1ba486e9935fbdce4080307aa292db6fc59dd817ae967899b9d4`
- Release: https://github.com/541968679/sub2api/releases/tag/v0.1.173
- Release workflow: https://github.com/541968679/sub2api/actions/runs/30075182232
- Prod: running, healthy, `/health` `{"status":"ok"}`

---
## 2026-07-23 - deploy: production Sub2API `v0.1.172`

### What
Deployed Haiku Claude-GPT bridge empty-output mitigations to production.

### Deploy
- Tag: `v0.1.172`
- Commit: `e5754a80d7ed43c0fc6a756f9b1eccef7283dd0f`
- Image: `ghcr.io/541968679/sub2api:latest` (version label `0.1.172`)
- Release: https://github.com/541968679/sub2api/actions/runs/29984803855
- Prod: running, healthy, `/health` `{"status":"ok"}`

---
## 2026-07-23 - fix: Haiku鈫扜PT empty completed output mitigations

### What
Gateway P0 mitigations for Claude Code Haiku鈫扜PT-5.* bridge empty completed
streams ("Connection closed" / no assistant text):

1. Default reasoning effort for Haiku-class Claude models to `low` when the
   client does not set `output_config.effort`.
2. After bridge model rewrite, raise `max_output_tokens` floor to 1024 for
   Haiku鈫抮easoning traffic and strip sampling params on GPT-5.*.
3. Mark empty completed (stream/non-stream) Anthropic conversions as
   `UpstreamFailoverError.NoAccountFailover` so the handler does not burn the
   multi-account pool on request-shaped failures.

### Why
Production Haiku bridge traffic (large Claude Code context + small max_tokens +
default medium reasoning on GPT-5.*) often completed with zero visible text.
Multi-account failover then multiplied error noise without changing the outcome.

### Files
- `backend/internal/pkg/apicompat/types.go` (`minReasoningMaxOutputTokens`)
- `backend/internal/pkg/apicompat/anthropic_to_responses.go` (+tests)
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/service/openai_gateway_service.go` (+test)
- `backend/internal/service/gateway_service.go` (`NoAccountFailover`)
- `backend/internal/handler/openai_gateway_handler.go` (+test)
- `docs/dev/codebase/gateway.md`

### Verify
- `go test -tags=unit ./internal/pkg/apicompat -run 'TestAnthropicToResponses_Haiku|TestApplyClaudeHaiku' -count=1`
- `go test -tags=unit ./internal/service -run TestEmptyVisibleOutputError -count=1`
- `go test -tags=unit ./internal/handler -run TestOpenAIEmptyVisibleOutput -count=1`
- Local HTTP + Claude Code smoke against `http://127.0.0.1:18081`:
  Haiku bridge 鈫?gpt-5.4 returned visible text with large Claude Code context
  (`local-haiku-ok-2`, exit 0, no empty-output failover).

---
## 2026-07-23 - fix: populate ops error upstream_model for Claude-GPT bridge

### What
Ops error logs for Claude-GPT bridge empty-output failures left
`upstream_model` empty, so the admin error-request table only showed the
client Claude model (e.g. haiku) without the mapped GPT model (e.g. luna).

### Why
`setOpsEndpointContext` was called with an empty upstream model before account
selection/mapping, and ForwardAsAnthropic never wrote the final upstream model
into ops context.

### Fix
- Set ops upstream model after bridge/channel mapping is resolved in Messages.
- Set ops upstream model inside ForwardAsAnthropic after final mapping/compact.
- Frontend model cell falls back to `model` as request name and still shows
  mapping when both sides exist.

### Files
- `backend/internal/service/ops_upstream_context.go` (+test)
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/ops_error_logger.go`
- `frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`

### Verify
- `go test -tags=unit ./internal/service -run TestSetOpsUpstreamModel -count=1`
- `go test -tags=unit ./internal/handler -run 'TestOps|TestMessagesClaudeGPT|TestOpenAIMessages' -count=1`

---
## 2026-07-22 - feat: admin error-requests tab with filtered error rate

### What
Upgraded the admin Usage page 鈥淓rror Requests鈥?area into a full independent tab
with dedicated filters (including multi-select status codes and Claude-GPT
bridge), terminal request-level error-rate stats, and richer list columns.

### Why
Operators need filter-scoped error rates (e.g. user + haiku + bridge + 502) to
debug intermittent bridge failures; the previous tab mixed usage UI and lacked
filters/stats.

### Fix
- Backend: extend ops error filters (upstream_model, bridge, error_type, user鈥?;
  add `GET /admin/ops/errors/stats` with S1 rate formula (deduped terminal
  errors / (success usage + biz-scope terminal errors)); mark
  `is_claude_gpt_bridge` on list rows.
- Frontend: errors tab hides usage charts/table; ErrorRequestFilters +
  ErrorRequestStatsCards; OpsErrorLogTable shows bridge + user/account.

### Files
- `backend/internal/service/ops_models.go`, `ops_port.go`, `ops_service.go`
- `backend/internal/repository/ops_repo.go`, tests
- `backend/internal/handler/admin/ops_handler.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/views/admin/UsageView.vue`, tests
- `frontend/src/components/admin/usage/ErrorRequest*.vue`
- `frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`
- `frontend/src/api/admin/ops.ts`, i18n zh/en
- `docs/dev/ERROR_REQUESTS_TAB_PRD_2026-07-22.md`
- `docs/dev/ERROR_REQUESTS_TAB_DESIGN_2026-07-22.md`

### Verify
- `go test -tags=unit ./internal/repository -run TestBuildOpsErrorLogsWhere -count=1`
- `pnpm --dir frontend exec vitest run src/views/admin/__tests__/UsageView.spec.ts`

---
## 2026-07-22 - fix: Claude-GPT bridge template overwrite + bulk apply

### What
Fixed Claude-GPT bridge mapping template application so template rows overwrite
existing same-source mappings, and added bulk-edit support for applying the
template across selected/filtered OpenAI accounts.

### Why
1. "Apply template" skipped any `from` key already present in model mapping, so
   editing the template (e.g. haiku 鈫?gpt-5.6-luna) could not update accounts
   that still had the old haiku 鈫?gpt-5.4 row.
2. Bulk edit only toggled the bridge switch and could not apply the shared
   local template to many accounts at once.

### Fix
- Shared helpers: overwrite-on-apply merge for template rows; draft is preferred
  over saved localStorage when the editor is open (edit then apply without a
  separate save).
- Single create/edit modals use the overwrite helper and report added/updated counts.
- Bulk edit exposes edit/apply template under Claude-GPT bridge; apply merges
  template keys into each account's existing `model_mapping` (non-template keys
  preserved), enables `openai_claude_gpt_bridge_enabled`, and persists immediately.

### Files
- `frontend/src/composables/useModelWhitelist.ts`
- `frontend/src/composables/__tests__/useModelWhitelist.spec.ts`
- `frontend/src/components/account/EditAccountModal.vue`
- `frontend/src/components/account/CreateAccountModal.vue`
- `frontend/src/components/account/BulkEditAccountModal.vue`
- `frontend/src/components/account/__tests__/BulkEditAccountModal.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

### Verify
- `pnpm --dir frontend exec vitest run src/composables/__tests__/useModelWhitelist.spec.ts src/components/account/__tests__/BulkEditAccountModal.spec.ts`

---
## 2026-07-17 - fix: Allow Grok-compatible API-key upstreams and model tests

### What
Fixed Grok API-key accounts configured with OpenAI-compatible public upstreams
such as `https://api.aisenyu.com/v1`, and restored Grok models in the admin
account model-test list.

### Why
Grok API-key traffic was sharing the official OAuth/CLI base-URL allowlist, so
compatible public hosts were rejected as `host is not allowed`. The admin
available-models endpoint also had no Grok branch, so Grok accounts fell through
to the Anthropic model list.

### Fix
- Keep official Grok OAuth/CLI traffic on the strict xAI/Grok host allowlist.
- Allow Grok API-key accounts to use public HTTPS compatible base URLs while
  still rejecting insecure/private hosts.
- Route Grok API-key account tests through `/v1/chat/completions`, matching
  OpenAI-compatible providers; keep OAuth tests on `/v1/responses`.
- Return xAI/Grok default models plus account mapping keys for Grok account
  model tests.

### Files
- `backend/internal/pkg/xai/oauth.go` (+tests)
- `backend/internal/service/openai_gateway_grok.go` (+tests)
- `backend/internal/service/account_test_service.go`
- `backend/internal/service/openai_gateway_chat_completions_raw.go`
- `backend/internal/service/grok_media.go`
- `backend/internal/handler/admin/account_handler.go` (+tests)

### Verify
- `go test -tags=unit ./internal/pkg/xai -count=1`
- `go test -tags=unit ./internal/handler/admin -run 'TestAccountHandlerGetAvailableModels_GrokReturnsGrokModels' -count=1 -v`
- `go test -tags=unit ./internal/service -run 'Test(BuildGrokResponsesRequest|BuildGrokMediaEndpointURLForAPIKey|AccountTestServiceGrokAPIKey|ForwardAsChatCompletionsForGrok|ForwardGrokResponsesAPIKey)' -count=1 -v`
- Broader `go test -tags=unit ./internal/pkg/xai ./internal/handler/admin ./internal/service -count=1` still fails in unrelated existing service tests:
  `TestOpenAIHandleErrorResponse_NoRuleKeepsDefault` and
  `TestOpenAIGatewayService_Forward_LogsInstructionsRequiredDetails`.

---
## 2026-07-17 - deploy: production Sub2API `v0.1.169`

### What
Deployed the Grok-compatible API-key upstream fix to production via GHCR pull/up
(no server-side docker build).

### Verify
- Release workflow: `29588643287` succeeded for tag `v0.1.169`.
- GHCR manifests: `ghcr.io/541968679/sub2api:0.1.169` and `:latest` exist.
- Production Compose: `sub2api.image` resolves to `ghcr.io/541968679/sub2api:latest`.
- Image: `ghcr.io/541968679/sub2api:latest`
- Version label: `0.1.169`
- Revision: `e9f6938331283c2c0d5ea07f82bc46bb9025f0c7`
- Container: running, healthy
- `GET /health`: `{"status":"ok"}`

### Notes
- Restarted only the `sub2api` service with `docker compose up -d --no-deps sub2api`.
- The compose run reported an existing orphan `a2-proxy` container; no cleanup was performed.

---
## 2026-07-15 - deploy: production Sub2API `v0.1.168`

### What
Deployed Grok Codex multi-turn / models-fallback release to production via GHCR pull/up (no server-side docker build).

### Verify
- Image: `ghcr.io/541968679/sub2api:latest`
- Version label: `0.1.168`
- Revision: `f38c7f0d5ffb8d4f4af21317a144de45f220ba28`
- Container: running, healthy
- `GET /health`: `{"status":"ok"}`

### Notes
- Tag `v0.1.168` Release Actions succeeded before deploy.
- Desktop picker still may hide custom Grok under Statsig whitelist; runtime with `model=grok-4.5` (UI 鑷畾涔? was verified locally before ship.

---
## 2026-07-15 - fix: Desktop Grok missing when ChatGPT models catalog times out

### Root cause (not xhigh filtering)
Codex Desktop uses headers that force Sub2API onto the Codex **manifest** path
(`GET /v1/models` 鈫?proxy `chatgpt.com/backend-api/codex/models`). When that
upstream request times out (observed on this machine), the handler returned
502/`upstream_error` and **never reached Grok injection**. Desktop then only
shows its local GPT-oriented catalog and Grok cannot be selected 鈥?even though
the OpenAI-list path already had `grok-4.5`.

Also aligned Grok ModelInfo with GPT rows: `tool_mode=null`,
`use_responses_lite=false` (was `code_mode_only` / lite=true).

### Fix
- On OAuth missing or ChatGPT catalog fetch failure: return empty Codex catalog
  shell + inject Grok (always 200 with grok-4.5 when access enabled)
- Inject entry: advertise xhigh (picker), clamp on wire; tool_mode null; lite false
- Local `~/.codex` catalogs refreshed to match

### Files
- `backend/internal/handler/openai_codex_models_handler.go`
- `backend/internal/service/openai_codex_models_grok_inject.go` (+tests)
- `frontend/src/utils/codexGrokCatalog.ts`

---
## 2026-07-15 - fix: Codex Desktop hides Grok when effort=xhigh

### What
OpenAI-group keys already returned `grok-4.5` on Desktop `/v1/models`, but Codex
Desktop still did not list Grok in the model picker.

### Why
User `config.toml` has `model_reasoning_effort = "xhigh"` (and plan mode xhigh).
GPT catalog entries include effort `xhigh`; Grok catalog only listed
low/medium/high. Desktop filters the picker by the currently selected effort,
so Grok was hidden.

### Fix
- Advertise `xhigh` in Codex Grok ModelInfo (`/v1/models` inject + frontend
  `model-catalog-grok` template); gateway still clamps xhigh鈫抙igh for xAI.
- Refresh local `~/.codex` catalogs with xhigh + Fast tier metadata.

### Files
- `backend/internal/service/openai_codex_models_grok_inject.go` (+tests)
- `frontend/src/utils/codexGrokCatalog.ts`
- local `~/.codex/model-catalog-*.json`, `models_cache.json` (not committed)

### Verify
- Live OpenAI key Desktop headers: manifest includes grok-4.5
- Local catalog efforts: low/medium/high/xhigh
- Unit inject tests pass

---
## 2026-07-15 - align Grok Codex multi-turn fixes with upstream

### Context
User ModelInput 422 matches known upstream issues. Upstream already fixed:
- PR #3982: drop Codex `additional_tools` (ModelInput deserialize)
- PR #4242 / ff639ba7: strip reasoning `content:null` (xAI 422)
- Issue #4223 still open: compaction blob wording with Grok+Codex

### What we ported / tightened
- Always run `sanitizeGrokResponsesInput` + `sanitizeGrokReasoningNullContent`
  (including compact-preserve path 鈥?previously skipped additional_tools)
- Also drop `encrypted_content:null`
- Keep local turn-2 fixes: empty `summary` for encrypted reasoning, decrypt recovery

### Files
- `backend/internal/service/openai_gateway_grok.go`
- `backend/internal/service/openai_gateway_grok_test.go`

---
## 2026-07-15 - fix: Grok turn-2 "compaction blob" is incomplete reasoning.encrypted_content

### What
Second Desktop message failed with xAI:
`Could not decode the compaction blob. Ensure it is unmodified from the compact response.`
This is **not** real remote compaction (turn 2 is far too early).

### Root cause
Codex multi-turn echoes `reasoning.encrypted_content` from turn 1. If `summary`
is missing or JSON null, xAI rejects it with that misleading "compaction blob"
message. Repro: `encrypted_content` alone 鈫?400; same blob + `summary:[]` 鈫?200.

### Fix
- Proactive: `ensureGrokReasoningEncryptedSummary` sets missing/null summary to `[]`
- Reactive: on compaction-blob / encrypted_content decrypt 400, drop encrypted
  reasoning once and retry (OpenAI-style invalid_encrypted_content recovery)
- Applied on HTTP Grok forward + WS鈫扝TTP bridge

### Files
- `backend/internal/service/openai_gateway_grok.go`
- `backend/internal/service/openai_ws_http_bridge.go`
- `backend/internal/service/openai_gateway_grok_test.go`

### Verify
- Unit: `TestEnsureGrokReasoningEncryptedSummaryAddsEmptySummary`
- Live: T2 with `{type:reasoning, encrypted_content:...}` only 鈫?200

---
## 2026-07-15 - fix: Grok/xAI errors show full message (not bare `{`)

### What
Codex Desktop multi-turn failures only showed a truncated `{` instead of the real
xAI message (e.g. compaction blob decode errors).

### Why
xAI returns `{"code":"...","error":"<string>"}` while we only parsed
`error.message`. Empty message + stream JSON body left Desktop unable to render
the error.

### Fix
- `extractUpstreamErrorMessage` understands string-form `error`
- Grok 400 bodies normalized to OpenAI `{error:{message,type,code}}`
- Stream requests get SSE error events with full message
- HTTP 400 surfaces real upstream message (not generic 502)

### Files
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/openai_gateway_grok.go`
- `backend/internal/service/openai_ws_http_bridge.go`
- `backend/internal/service/openai_gateway_grok_test.go`

---
## 2026-07-15 - fix: Grok compaction blob integrity (Codex multi-turn)

### What
xAI 400: `Could not decode the compaction blob. Ensure it is unmodified from the compact response.`
when Codex Desktop continued a long Grok thread after remote compaction.

### Why
Normal Grok request patching rewrote tools / free-tier cache identity / input rebuild
around the opaque compaction item. The blob is integrity-bound to the compact response.

### Fix
When body has compaction context (`type=compaction` / `compaction_trigger` / compact path):
- only sjson-set model + drop always-unsupported top-level fields
- skip tool filter, free-tier tool injection, prompt_cache_key rewrite, full JSON remashal
- same for HTTP forward and WS鈫扝TTP bridge

### Files
- `backend/internal/service/openai_gateway_grok.go`
- `backend/internal/service/openai_ws_http_bridge.go`
- `backend/internal/service/openai_gateway_grok_cache.go`
- `backend/internal/service/openai_gateway_grok_test.go`

### Verify
- Unit: `TestPatchGrokResponsesBodyPreservesCompactionBlobAndTools`
- Local stack restarted via `scripts/dev-stack.ps1 restart -SkipAIClient`

---
## 2026-07-15 - fix: Grok HTTP multi-turn previous_response_id hard 400

### What
Codex Desktop multi-turn on Grok keys failed on turn 2: HTTP `POST /v1/responses`
with `previous_response_id` was hard-rejected (`only supported on Responses
WebSocket v2`). Client often only showed a truncated `{` error body.

### Why
Grok has no Responses WS v2. Desktop still multi-turns over plain HTTP (not only
WS). The WS鈫扝TTP bridge already strips `previous_response_id`; HTTP handler did not.

### Fix
- Grok platform groups and Grok text models: strip `previous_response_id` on HTTP
  and continue (same parity as WS bridge).
- OpenAI non-Grok models: still reject with the WS v2 message.
- Compaction-safe cache identity: do not rewrite `prompt_cache_key` / inject free-tier
  tools when the body carries compaction context (avoids xAI
  "Could not decode the compaction blob").
- Preserve explicit client `tools:[]` when applying free-tier cache defaults.

### Files
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_gateway_handler_test.go`
- `backend/internal/service/openai_gateway_grok_cache.go`

### Verify
- Unit: `TestOpenAIResponses_GrokHTTPStripsPreviousResponseID*`
- Live: Grok key turn1 + turn2 with `previous_response_id` 鈫?200 (no WS-v2 400)

### Note
Stripping `previous_response_id` without a server-side response store means pure
delta-only second turns may lack prior context unless the client resends history
or uses the WS bridge replay path for tool calls. Hard failure is fixed first.

---
## 2026-07-15 - Codex Desktop Grok model visibility (service_tier)

### What
Codex Desktop still hid `grok-4.5` even though CLI saw it and `/v1/responses` worked. Root cause was incomplete Codex ModelInfo injection: missing `additional_speed_tiers` / `service_tiers` while local `config.toml` uses `service_tier = "fast"`, plus incomplete `available_in_plans` on stale client cache entries. Inject now clones a GPT manifest template, always upgrades existing Grok rows, and guarantees plan + Fast tier metadata.

### Why
Desktop filters the model picker by plan membership and selected service tier. API key / base URL were fine (local Sub2API).

### Files
- `backend/internal/service/openai_codex_models_grok_inject.go`
- `backend/internal/service/openai_codex_models_grok_inject_test.go`
- Local client caches refreshed: `~/.codex/models_cache.json`, `model-catalog-505k.json` (not committed)

### Verify
- Unit: `go test -tags=unit ./internal/service -run TestInjectGrokModelsIntoCodexManifest`
- Live Desktop headers: `GET /v1/models` includes `grok-4.5` with `additional_speed_tiers=["fast"]` and non-empty `available_in_plans`
- Restart Codex Desktop after cache refresh

---
# Sub2API 浜屾寮€鍙戝彉鏇存棩蹇?

> 璁板綍鎵€鏈夌浉瀵逛簬涓婃父 (Wei-Shaw/sub2api) 鐨勮嚜瀹氫箟淇敼銆傛瘡娆′簩娆″紑鍙戝彉鏇村繀椤诲湪姝よ褰曪紝渚夸簬鍚堝苟涓婃父鏇存柊鏃惰拷韪樊寮傘€?

## [2026-07-15] fix: Grok account usage query (billing probe + free 24h estimate)

**Upstream sources** (ported, not full merge):
- `c896cacf6` / PR #4188 鈥?free quota probing and billing display
- `30d4301be` / PR #4231 鈥?rolling 24h free quota estimate

**Root cause**:
1. `AccountUsageService.GetUsage` never branched on `PlatformGrok`, so Grok OAuth
   accounts fell through `CanGetUsage()` into the Anthropic usage path and failed.
2. `GrokQuotaFetcher` was not wired into `AccountUsageService`.
3. Manual probe only hit Responses with rate-limit headers; Free accounts often
   have no authoritative usage percent, so the UI stayed empty/unknown.
4. Probe default model was still `grok-4.3` while the gateway default is `grok-4.5`.

**What changed**:
- Add xAI billing client (`internal/pkg/xai/billing.go`) and hybrid
  `GrokQuotaService.QueryQuota` (billing first, active probe for Free).
- Wire `GrokQuotaFetcher` + `GrokQuotaService` into usage service; list/detail
  usage probes billing without consuming model quota when possible.
- Free tier shows rolling 24h local token estimate (2M); paid shows billing %.
- Admin probe button uses `QueryQuota`; frontend `AccountUsageCell` shows
  weekly/24h bars; i18n keys `grokWeeklyUsage` / `grokFreeQuota24hHint`.
- Default probe/test model aligned to `grok-4.5` (`grokDefaultResponsesModel`).

**Affected files** (main):
- Backend: `pkg/xai/billing*.go`, `service/grok_quota_*.go`,
  `service/account_usage_service.go`, `service/wire.go`, `cmd/server/wire_gen.go`,
  `handler/admin/grok_oauth_handler.go`, `repository/account_repo.go`
- Frontend: `api/admin/grok.ts`, `AccountUsageCell.vue`, `GrokQuotaProbeCell.vue`,
  `types/index.ts`, `i18n` zh/en

**Tests**:
- `go test -tags=unit ./internal/pkg/xai/ ./internal/service/ -run 'TestGrokQuota|TestAccountTestService_.*Grok'`
- `go test -tags=unit ./internal/handler/admin/ -run GrokOAuthHandler`
- `vitest` AccountUsageCell + GrokQuotaProbeCell

**Frontend follow-up (same day)**:
- Restored local API paths (`/admin/grok-oauth/...` POST query/reset) after upstream
  port accidentally switched to non-existent `/admin/grok/...` routes.
- Fixed `AccountUsageCell` typecheck: add `subscription_tier` fields; drop unsupported
  `getUsage(..., force)` third argument for this fork.

## [2026-07-15] fix: Codex Desktop gets Grok models via manifest routing

**Affected files**: `server/routes/gateway.go`, `openai_codex_models_handler.go`,
tests, this changelog.

**Why**: CLI sees Grok because it calls `GET /v1/models?client_version=...`
(Codex manifest + inject). Desktop often omits `client_version` and only sends
Codex UA/Originator, so it hit the plain OpenAI list shape and never showed
Grok slugs in the Desktop picker.

**What changed**: Serve Codex manifest when OpenAI-group request is identified
as an official Codex client (UA/Originator), not only when `client_version` is
set; fall back to `Version` header for upstream catalog version.

## [2026-07-15] fix: Grok reasoning effort clamps xhigh鈫抙igh; Codex catalog drops xhigh

**Affected files**: `openai_gateway_grok.go`, `openai_codex_models_grok_inject.go`,
`frontend/src/utils/codexGrokCatalog.ts`, tests, this changelog.

**Why**: Codex could select Extra High (`xhigh`) for Grok because our catalog
advertised it, but xAI Grok only accepts low/medium/high. Passthrough caused
upstream failures.

**What changed**:
- Forward path clamps `reasoning.effort` / `reasoning_effort` values above high
  (xhigh/max/ultra/鈥? to `high` for non-composer Grok models.
- Composer models still strip reasoning fields entirely.
- Codex inject + local catalog helpers only list low/medium/high.

## [2026-07-15] fix: scheduler cache keeps Grok OpenAI-group access flag

**Affected files**: `repository/scheduler_cache.go`,
`service/openai_gateway_service.go` (stale Extra refresh),
`service/openai_gateway_model_availability.go`, tests, this changelog.

**Why**: OpenAI-group keys selecting `grok-4.5` returned 404 "not supported by
any configured account" even when a bound Grok account had
`grok_openai_group_access_enabled=true`. The scheduler snapshot Extra whitelist
kept `openai_claude_gpt_bridge_enabled` but stripped the Grok access flag, so
eligibility always failed.

**What changed**: Whitelist `grok_openai_group_access_enabled` in scheduler
Extra filtering; reload from DB when Grok-access eligibility fails; diagnose
availability against the Grok schedule pool for OpenAI鈫扜rok requests.

## [2026-07-15] feat: OpenAI /v1/models always surfaces grok-4.5

**Affected files**: `models_list_policy.go`, `gateway_handler.go`,
`openai_codex_models_handler.go`, `admin_service.go` (models-list candidates),
tests, this changelog.

**Why**: Per-group custom models lists only offered a fixed OpenAI curated
subset (no Grok). Forcing every OpenAI group to be re-edited for production is
risky. Operators want a simple default: OpenAI-group keys see `grok-4.5` in
`/v1/models` (and Codex manifest) without per-group ops.

**What changed**:
- Curated OpenAI discovery includes `grok-4.5`.
- After custom-list filtering, still ensure `grok-4.5` is present.
- Codex manifest always injects at least `grok-4.5` (+ extra access models).
- Admin group models-list candidates for OpenAI include Grok text models.
- Scheduling is unchanged: still requires Grok account opt-in + group bind.

## [2026-07-15] fix: Codex manifest injects Grok models for OpenAI-group access

**Affected files**: `backend/internal/service/openai_codex_models_grok_inject.go`,
`handler/openai_codex_models_handler.go`, tests, this changelog.

**Why**: Codex CLI/Desktop calls `GET /v1/models?client_version=...`, which is
routed to the ChatGPT Codex manifest proxy 鈥?**not** the ordinary
`Gateway.Models` discovery path that merges Grok text models. After enabling
Grok OpenAI-group access, Codex still only saw gpt-* slugs.

**What changed**: After fetching the upstream Codex manifest, inject ModelInfo
entries for bound opt-in Grok text models; drop upstream ETag when body is
modified so clients do not cache the pre-injection document.

## [2026-07-15] feat: OpenAI groups can access bound Grok accounts (per-account opt-in)

**Affected files**:
- Backend: `service/account.go`, `service/admin_service.go`, `service/grok_openai_group_access.go`,
  `service/openai_gateway_service.go`, `service/openai_account_scheduler.go`,
  `service/gateway_service.go`, `handler/gateway_handler.go`, `handler/openai_gateway_handler.go`,
  `handler/openai_chat_completions.go`, tests
- Frontend: `CreateAccountModal.vue`, `EditAccountModal.vue`, `zh.ts`/`en.ts`,
  `GrokManagementReachability.spec.ts`
- Docs: this changelog

**Why**: OpenAI-group API keys could not see or schedule Grok models/accounts
(platform isolation). Operators need controlled sharing of Grok capacity into
specific OpenAI groups without requiring a second Grok-group key.

**Product rules (frozen)**:
1. Each Grok account (OAuth and API-key) has `extra.grok_openai_group_access_enabled`
   (default off). Only when enabled may it bind **specific OpenAI groups**.
2. Billing is unchanged for the OpenAI-group key (group rate / subscription /
   platform-quota identity stay on the OpenAI group). Requests with a Grok model
   still price that model via the normal Grok model pricing path.
3. Custom models lists never auto-append Grok models; only explicitly listed IDs appear.

**What changed**:
- Bind validation: opt-out Grok 鈫?Grok groups only; opt-in Grok 鈫?Grok + OpenAI groups.
- OpenAI-compatible schedule resolves Grok text models to the Grok pool with
  access eligibility; gpt models stay on the OpenAI pool.
- `/v1/models` merges Grok text models for non-custom OpenAI discovery when
  bound opt-in Grok accounts exist.
- WS/responses/chat use the access-aware selector; previous_response sticky is
  not reused across OpenAI鈫擥rok access routing.
- Admin UI toggle + i18n for the opt-in control.

## [2026-07-15] fix: Grok strips orphan tool_choice (Codex 400 hang)

**Affected files**: `backend/internal/service/openai_gateway_grok.go`,
`openai_ws_http_bridge.go`, tests, this changelog.

**Why**: Codex sends `tool_choice` with tools that Grok does not support (or empty
tools). After filtering tools away, `tool_choice` could remain 鈫?xAI 400
`A tool_choice was set on the request but no tools were specified.` Streaming
clients then appear to hang and may surface a truncated `{` error body.

**What changed**: Always reconcile `tool_choice` when no valid tools remain;
re-run sanitize after free-tier cache identity injection (HTTP + WS bridge).

## [2026-07-15] fix: WS multi-turn usage_logs.request_id overflow (varchar 64)

**Affected files**: `backend/internal/handler/openai_gateway_handler.go`,
`backend/internal/handler/turn_usage_record_context_test.go`, this changelog.

**Why**: Per-turn billing context appended `:turn:<full-upstream-uuid>` to the
connection request id. With the `local:` prefix this exceeded `usage_logs.request_id`
varchar(64), so WS Grok turns completed but usage insert failed
(`pq: value too long for type character varying(64)`).

**What changed**: Compact suffix `:t:<turn>-<last8>` so stored request ids stay 鈮?4.

## [2026-07-15] fix: Grok Responses WS HTTP bridge must call xAI, not ChatGPT Codex

**Affected files**: `backend/internal/service/openai_ws_http_bridge.go`, this changelog.

**Why**: After opening Grok WS ingress, multi-turn still failed: the shared WS鈫扝TTP
bridge built upstream requests via OpenAI passthrough (`chatgpt.com/.../codex/responses`)
with a Grok OAuth token 鈫?upstream 401 鈥淐ould not parse your authentication token鈥?
No successful usage was recorded; repeated failures temp-unschedulable the only Grok account.

**What changed**: For `account.IsGrok()`, the bridge now reuses
`patchGrokResponsesBody` + `buildGrokResponsesRequest` (CLI proxy / api.x.ai path).

**Local ops note (this machine)**: Grok OAuth account `3004` must use an outbound
proxy that can reach `cli-chat-proxy.grok.com` (bound to proxy id 18 = `127.0.0.1:10808`).
Without it, requests hang ~21s then 502 and produce no usage rows.

## [2026-07-15] fix: Grok Codex model catalog includes required ModelInfo fields

**Affected files**: `frontend/src/utils/codexGrokCatalog.ts`, tests, this changelog.

**Why**: Codex CLI rejects incomplete `model_catalog_json` entries with
`missing field supports_reasoning_summaries` (strict serde ModelInfo).

**What changed**: Catalog template now includes the required capability flags
(`supports_reasoning_summaries`, `apply_patch_tool_type`, `tool_mode`, etc.)
aligned with a real Codex ModelInfo shape, not a sparse subset.

## [2026-07-15] fix: silence Codex 鈥淢odel metadata for grok-4.5 not found鈥?after Grok import

**Affected files**:
- `frontend/src/utils/codexGrokCatalog.ts` (+ unit tests)
- `frontend/src/components/keys/UseKeyModal.vue` (+ tests)
- `frontend/src/views/user/KeysView.vue` (CCS import tip)
- `frontend/src/i18n/locales/{zh,en}.ts`
- this changelog

**Why**: CCS one-click import for Grok correctly sets `model = "grok-4.5"` but CC Switch鈥檚
Codex deeplink template does **not** write `model_context_window` / `model_catalog_json`.
Codex then warns that Grok metadata is missing and uses fallback ModelInfo.

**What changed**:
- Ship a portable `model-catalog-grok.json` + Codex `config.toml` template with 1M context
  and relative catalog pointer (Use Key 鈫?Codex CLI / WebSocket for Grok groups).
- After CCS import of a Grok key, show a warning tip explaining the catalog gap.
- Local ops note: patch `~/.codex/config.toml` + write catalog when verifying.

## [2026-07-15] fix: Grok-group CCS import uses model grok-4.5 (not Claude)

**Affected files**:
- `frontend/src/utils/ccswitchImport.ts` (new, upstream-aligned resolver)
- `frontend/src/utils/__tests__/ccswitchImport.spec.ts`
- `frontend/src/views/user/KeysView.vue`
- this changelog

**Why**: Grok-group API keys imported via 銆屽鍏ュ埌 CCS銆?wrote
`model = "claude-sonnet-4-5"` because Codex model selection only had
openai vs non-openai buckets, and Grok fell into
`ccs_import_anthropic_codex_model`.

**What changed** (minimal, upstream-style):
- Extract `resolveCcSwitchImportConfig` / `buildCcSwitchImportDeeplink` like
  upstream `ccswitchImport`.
- Explicit `platform=grok` 鈫?`app=codex`, `model=grok-4.5` (matches UseKeyModal).
- OpenAI still uses admin `ccs_import_codex_model`; Anthropic鈫扖odex still uses
  `ccs_import_anthropic_codex_model`. Grok no longer reuses the Anthropic setting.

**Note**: Upstream maps unknown platforms to Claude without a model; Grok is
OpenAI-compatible Responses, so we intentionally set Codex + `grok-4.5` rather
than copying that fallthrough.

## [2026-07-15] fix: Grok Responses WebSocket ingress 鈫?HTTP/SSE bridge (Codex multi-turn)

**Affected files**:
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/service/openai_ws_forwarder.go`
- `backend/internal/service/openai_ws_http_bridge.go`
- `backend/internal/handler/openai_gateway_handler_test.go`
- `backend/internal/service/openai_ws_http_bridge_test.go`
- `README.md`
- this changelog

**Why**: Codex (`wire_api=responses`) multi-turn / tool continuation for Grok-group
keys failed: first HTTP turn worked, then client preferred Responses WebSocket
ingress (501 hard reject) or HTTP `previous_response_id` (400 WS-v2 only).

**What changed** (requirement A, minimal patch 鈥?not full upstream WS cache/pool):
- Remove Grok-only 501 gate on `ResponsesWebSocket`.
- Schedule Grok WS ingress with `requiredTransport=http_sse` and
  `requestPlatform=grok` so only Grok accounts are selected.
- Force Grok accounts onto the existing client-WS 鈫?upstream HTTP/SSE bridge
  (including multi-turn with `previous_response_id` via bridge replay).
- OpenAI WS path unchanged (still requires ws_v2 when not forced to bridge).

**Compatibility**: OpenAI/Grok platform isolation preserved. OpenAI-key cross-platform
Grok routing (requirement B) is **not** included.

**Tests**: handler regression (Grok no longer 501); bridge decision forces Grok;
end-to-end multi-turn Grok WS鈫扝TTP bridge unit test.

## [2026-07-15] docs: Grok Codex multi-turn and OpenAI-key cross-platform research

**Affected files**: `docs/dev/GROK_CODEX_AND_CROSS_PLATFORM_RESEARCH_2026-07-15.md`, `docs/dev/codebase/README.md`, `docs/dev/codebase/gateway.md`, this changelog.

**Compatibility**: Documentation only. No runtime, schema, route, billing, scheduler, or deployment behavior changed.

**Details**:
- Records two independent requirements after ZeroCode + Codex investigation:
  - **A**: Grok-group API keys fail Codex multi-turn because this fork rejects Grok Responses WebSocket ingress while HTTP `previous_response_id` requires WS v2; upstream bridges client WS to HTTP/SSE for Grok.
  - **B**: OpenAI-group keys cannot see or schedule `grok-4.5` under current platform isolation; not delivered by upstream Grok WS work.
- Documents why Grok WS was intentionally left unsupported (platform isolation, HTTP/SSE capability boundary, avoid half-importing upstream WS cache/pool), empirical probes, risk boundaries, and implementation options (minimal A patch vs separate B PRD).
- Indexes the research doc from `docs/dev/codebase/README.md` (module table + gateway row).

## [2026-07-14] docs: Close the selective v0.1.152 sync ledger

**Affected files**: public README, upstream-sync ledger, and this changelog.

**Compatibility**: Documentation-only closeout of the selective official
v0.1.152 alignment. No runtime, schema, generated code, dependency, route,
setting, billing, scheduling, or deployment behavior changed.

**Details**:
- Records the six implementation batches, prior behavior already present,
  migration renumbering, fork-local protection boundaries, and the exact
  upstream tag target.
- Documents the deliberate exclusion of the upstream-only Responses compact
  writer wrapper and Grok WebSocket cache/pool changes because the owning
  protocols are not enabled in this fork.
- Keeps `backend/cmd/server/VERSION` at local `0.1.164` instead of downgrading
  to the official tag's older VERSION artifact.
- Adds public Grok OAuth/API-key, Grok Build, and OpenCode setup guidance that
  matches the fork's HTTP/SSE capability boundary.
- Final verification passed all backend unit packages, 151 frontend test files
  / 855 tests, typecheck, lint, Ent stability, production frontend/server
  builds, and both sync-guard modes. Integration/Wire checks reproduced only
  the documented pre-existing missing fixtures/providers.

## [2026-07-14] test: Complete alpha-search public group contract

**Affected files**: public API contract fixture and this changelog.

**Compatibility**: Contract-test-only completion of upstream `e5af699d0`.
Runtime responses already exposed the nullable field; no handler, DTO, billing,
schema, migration, frontend, or route behavior changed.

**Details**:
- Adds `web_search_price_per_call: null` to the `/api/v1/groups/available`
  snapshot so the fixture matches the public DTO introduced by the alpha-search
  billing batch.
- The omission was found by the final `go test -tags=unit ./...` gate; all
  other backend packages had passed.

## [2026-07-14] chore: Complete Ent generator dependency checksums

**Affected files**: backend Go module checksums and this changelog.

**Compatibility**: Dependency metadata only. No module version, generated Ent
source, runtime graph, schema, migration, billing, gateway, frontend, or
deployment behavior changed.

**Details**:
- `go generate ./ent` completed without changing generated source and added the
  missing CLI transitive checksums. The table/rendering checksums match official
  v0.1.152; the additional `mousetrap` checksum is required by the Windows Go
  toolchain when resolving Cobra.
- Re-running Ent generation is stable after the checksum completion.

## [2026-07-14] fix: Restore OAuth Messages identity and Grok OpenCode adapter

**Affected files**: OpenAI Codex identity helper, Anthropic Messages forwarding,
OpenAI Responses request construction, Grok forwarding regressions, API-key use
modal, focused tests, gateway/account module documentation, and this changelog.

**Upstream compatibility**: Selective behavior-level alignment of upstream
`d5b47c214` and `ad18ee7c4`.

**Details**:
- OpenAI OAuth requests translated from Anthropic Messages retain the existing
  bridge-specific body and session/conversation behavior, then restore a
  complete, internally paired Codex `User-Agent`, `originator`, `version`, and
  `OpenAI-Beta` identity immediately before sending to ChatGPT.
- Official Codex user agents and valid versions remain intact; missing identity
  falls back to the bundled Codex CLI values, and third-party user agents are
  normalized by the existing final identity pairing rule.
- Grok Messages forwarding remains isolated on its xAI adapter: it keeps the
  Grok transport user agent, never receives Codex `originator` or `version`,
  and only passes an explicitly supplied `OpenAI-Beta` value.
- Grok OpenCode examples now use `@ai-sdk/openai`, whose Responses adapter
  matches the configured Sub2API Grok endpoint. Grok Build configuration paths
  remain correct on Unix and Windows.
- Verified focused and extended OpenAI Messages/Grok service tests,
  `cmd/server` compilation, and both API-key modal test suites. Billing,
  display-token accounting, real cache-read quantities, curated/default
  models, Claude-GPT bridge routing, OpenAI Images, scheduling/failover, Ops,
  settings, migrations, routes, and i18n remain unchanged.

## [2026-07-14] sync: Align v0.1.152 admin selection and Grok onboarding UI

**Affected files**: Admin user lookup service/repository/DTO/API, Fast/Flex
settings UI, Grok quota presentation, Grok account forms, API-key use modal,
frontend types/i18n/tests, focused backend tests, account module documentation,
and this changelog.

**Upstream compatibility**: Selective behavior-level alignment of upstream
`0464856c4`, `cbddb57de`, the frontend portion of `d9e466ad3`, and the Grok
onboarding portion of `038b25c0b`.

**Details**:
- Replaces manual Fast/Flex numeric user-ID rows with debounced email search,
  selected-user tags, duplicate filtering, and non-destructive hydration of
  saved IDs. Historical unresolved IDs stay visible and removable.
- Adds an explicit administrator-only `include_deleted=true` user lookup and
  includes deleted users in the existing admin usage search response. Ordinary
  user reads still apply the soft-delete filter.
- Displays Grok quota bars as remaining capacity: full quota is a full green
  bar, low/exhausted capacity shrinks and changes color. Other platform usage
  bars keep their existing used-percentage semantics.
- Completes Grok API-key account form defaults/placeholders and adds Grok
  Build plus OpenCode configuration examples to the user API-key modal.
- Preserves the fork's existing Grok OAuth/API-key forwarding, scheduling,
  billing/display-token accounting, curated model lists, Claude-GPT bridge,
  OpenAI Images, default-model fallback, Ops logging, public/admin settings,
  migrations, and routes.
- Verified focused backend unit tests, `cmd/server` compilation, 52 frontend
  regression tests, frontend type checking, and frontend lint checking. No
  service start, migration, push, or deployment occurred in this batch.

## [2026-07-13] feat: Add isolated Grok prompt caching and safe Chat bridging

**Affected files**: Grok cache identity and Chat bridge services, Grok
Responses/raw Chat forwarding, OpenAI-compatible endpoint attribution,
scheduling session extraction, focused tests, account module documentation,
and this changelog.

**Upstream compatibility**: Selective behavior-level alignment of upstream
`0478fd366` and `7050070aa`.

**Details**:
- Derives a stable Grok prompt-cache UUID from downstream API-key ID, mapped
  model, and explicit/content-derived conversation seed. Raw tenant/session
  identifiers are never sent upstream, and identical seeds remain isolated
  across API keys and model mappings.
- Grok OAuth Responses requests receive the isolated `prompt_cache_key` and
  conversation header. Tool-free requests may receive native search tools with
  `tool_choice=none` to select the cache-capable free OAuth route; any explicit
  client tool intent prevents this augmentation.
- Plain-text Grok OAuth Chat Completions requests use Responses only when a
  strict shape check, cache identity, and `grok-4.5` mapped-model gate all pass.
  Tools, images, developer/tool roles, stop/reasoning parameters, small token
  caps, unknown fields, API-key accounts, and other mapped models stay on raw
  `/v1/chat/completions`.
- Usage/Ops records now take the actual forwarding endpoint from the result or
  request context, so dynamically bridged and raw Grok Chat requests are not
  misattributed.
- Cached input remains the upstream-reported real quantity and flows through
  existing billing/display logic unchanged; the bridge does not fabricate
  cache tokens or alter stored `actual_cost`.
- Verified all Grok-focused service tests, endpoint attribution handler tests,
  and `cmd/server` compilation. No migration, frontend, service start, push, or
  deployment occurred in this batch.

## [2026-07-13] sync: Align Grok CLI routing and quota safety from v0.1.152

**Affected files**: Grok account URL/OAuth credentials, shared upstream
transport, Responses/Chat/Messages/media/WebSocket bridge forwarding, account
connection tests, quota persistence/repository, OpenAI-compatible diagnostics,
billing fallback, Wire wiring, unit tests, account module documentation, and
this changelog.

**Upstream compatibility**: Selective behavior-level alignment of upstream
`3375b4ed2`, `f187f08ae`, `038b25c0b`, `aeb34d200`, `d9e466ad3`,
`1dedb2097`, and `8a22dc734`.

**Details**:
- New and legacy Grok OAuth accounts use the official CLI proxy when their
  base URL is blank or the canonical `api.x.ai` URL. Custom URLs remain
  untouched; API-key accounts continue to default to the public xAI API.
- Exact CLI-proxy requests receive the stable Grok Build identity at the final
  shared transport boundary. The optional version override accepts only
  canonical semver values at or above `0.2.93`.
- Grok Responses forwarding and account tests now support both OAuth and xAI
  API-key credentials. Composer reasoning fields and Codex-only
  `additional_tools` input carriers are removed before xAI forwarding.
- Quota exhaustion observed on either success or error responses is persisted
  as an account rate limit, with monotonic reset extension and an immediate
  in-memory scheduling block. Retry-After, request-window, and token-window
  reset boundaries are respected; no-reset exhaustion uses a bounded fallback.
- OpenAI-compatible Responses, Chat, Messages, count_tokens, and logs diagnose
  Grok groups against the Grok platform rather than reporting OpenAI-account
  availability.
- Added fail-closed Grok 4.3/4.5/Build/Composer fallback prices including real
  cached-input rates. Stored billing, quota deduction, `actual_cost`, display
  transforms, and real cache-read token quantities are otherwise unchanged.
- Verified focused repository/service/handler/admin unit tests and `cmd/server`
  compilation. No migration, frontend route/i18n change, service start, push,
  or deployment occurred in this batch.

## [2026-07-13] sync: Align v0.1.152 protocol compatibility fixes

**Affected files**: OpenAI Responses compatibility types/tests, Codex input
filter/tests, Responses compact request normalization/tests, and this changelog.

**Upstream compatibility**: Selective behavior-level alignment of upstream
`5015b7a1c`, `4d4ba64bf`, and the native `remote_compaction_v2` routing portion
of `84bb7d070`.

**Details**:
- Accept `tool_search_call.arguments` as an object during Responses output,
  response, and stream-event decoding while retaining the existing internal
  raw-JSON string representation and object-shaped wire output.
- Strip client-replayed non-`msg*` IDs from `type=message` items when Codex
  continuation references are preserved, without mutating caller-owned input.
- Keep `remote_compaction_v2` requests with `stream:true` on the native
  `/responses` route; explicit `/responses/compact` requests retain the fork's
  existing unary normalization and scheduler capability requirement.
- Verified focused apicompat, service (`unit` tag), and handler regression
  suites plus `git diff --check`. Billing/display-token accounting, curated
  models, Claude-GPT bridge, image generation, fallback, scheduling/failover,
  Ops settings, migrations, frontend routes, and i18n were not changed.

## [2026-07-13] feat: Add Codex alpha search with per-call billing

**Affected files**: OpenAI alpha-search handler/service/routes, endpoint
normalization, embedded-frontend bypass, group schema/Ent/repository/admin DTOs,
API-key auth snapshots, billing/usage recording, migration `191`, admin group
form/types/i18n, tests, and this changelog.

**Upstream compatibility**: Selective behavior-level alignment of upstream
`52071d391`, `7cbb36f27`, `64a2a3172`, `e5af699d0`, and `b0fa2b352`.

**Details**:
- Added authenticated `POST /v1/alpha/search`, `/alpha/search`, and
  `/backend-api/codex/alpha/search` routes for OpenAI groups. The evolving
  request/response JSON is passed through without schema narrowing; model
  mapping, account scheduling, concurrency, failover, response headers, and
  Ops endpoint attribution reuse the existing OpenAI gateway stack.
- A successful 2xx upstream search creates exactly one per-request billing
  unit. Non-2xx responses are passed through without billing. The default price
  is `$0.01` per call, while groups may override it or set zero for free calls.
- Per-call search cost uses the resolved base user/group multiplier and does not
  inherit subscription peak-rate factors. Stored `billing_mode`,
  `rate_multiplier`, `total_cost`, and `actual_cost` remain mutually
  explainable; token and cache-read quantities remain zero and unchanged.
- Added nullable `groups.web_search_price_per_call` through idempotent migration
  `191`, Ent generation, repositories, DTOs, auth-cache snapshot version `11`,
  and bilingual admin create/edit controls. The bare `/alpha/search` alias now
  bypasses the embedded SPA middleware.
- Verified focused service/handler/repository/route tests, embedded frontend
  tests, Ent package compilation, frontend typecheck/lint, sync guard, and
  whitespace checks. No push, deployment, or local service restart occurred.

## [2026-07-12] feat: Move error-request viewing from user usage to admin usage

**Affected files**: `frontend/src/views/user/UsageView.vue`, `frontend/src/views/admin/UsageView.vue`, `frontend/src/views/admin/__tests__/UsageView.spec.ts`, `docs/dev/CHANGELOG_CUSTOM.md`
**Compatibility**: Frontend only. Makes error-request viewing admin-only. The user-side `allow_user_view_error_requests` setting and `/usage/errors` API are retained but the user tab no longer renders.
**Details**:
- User usage view (`/usage`): the error-request tab is hidden unconditionally (`errorViewEnabled` forced false). The tab bar disappears and only the usage records section renders; the setting/API are kept dormant for future re-enablement.
- Admin usage view (`/admin/usage`): added an "閿欒璇锋眰 / Error Requests" tab alongside "浣跨敤璁板綍" and "鐢ㄦ埛鎺掕", lazily mounted like the ranking tab. It reuses the existing Ops error infrastructure 鈥?`opsAPI.listErrorLogs` (`/admin/ops/errors`, `view=errors`), `OpsErrorLogTable` (self-paginating), and `OpsErrorDetailModal` (`error-type="request"`) 鈥?scoped to the page's date range plus group/account filters (converted to RFC3339 full-day bounds).
- Errors reload on filter apply/refresh when the tab is active; i18n `usage.tabs.errors` already existed in zh/en.
- Verified: typecheck + lint clean; admin UsageView spec updated (3 tabs, new lazy-load-and-fetch test) and user/admin specs green; live check confirmed the admin tab fires `GET /admin/ops/errors?...view=errors` (200) with the correct date bounds and the user view shows no error tab.

## [2026-07-11] sync: Complete selective alignment through upstream e316ebf5

**Affected files**: consolidated upstream-alignment branch and verification ledger.

**Upstream compatibility**: Behavior-level selective alignment through
`e316ebf52838a89d57fc790981cce7520f819ac8`; fork-local contracts remain
authoritative and assessed exclusions are documented.

**Details**:
- Completed the final usage ranking/CSV, Anthropic dateline, Anthropic API-key Bearer, and committed-range guard gaps found by the closing audit.
- Verified all backend unit packages, Ent stability, production-style server build, 837 frontend tests, typecheck, lint, frontend build, and both sync-guard modes.
- Confirmed no source deletion or historical migration SQL modification relative to the isolated-worktree baseline; the original main checkout was not modified.
- Integration-tag compilation remains blocked by existing missing test fixtures (`cacheRecorder`, `newMockSettingRepo`); Wire regeneration remains blocked by existing handwritten provider-set gaps. Checked-in generated code builds and tests successfully.
- No push, pull request, local service start, or deployment was performed.

## [2026-07-11] fix: Check committed upstream-sync ranges in the fork guard

**Affected files**: `backend/tools/upstream-sync-guard/main.go`, `backend/tools/upstream-sync-guard/main_test.go`, `docs/dev/CHANGELOG_CUSTOM.md`, `docs/dev/UPSTREAM_SYNC.md`

**Upstream compatibility**: Guard/test/documentation only. No product source, schema, migration content, billing, gateway, scheduler, frontend behavior, push, or deployment changed.

**Details**:
- Added `--base <revision>` to compare `BASE..HEAD`, so a completed upstream-sync batch cannot hide a committed deletion or outward rename of a protected fork-local path.
- The same committed range now rejects modifications, deletions, or renames of historical migrations below `150`. Invalid revisions report the exact attempted range and Git error.
- Kept the no-argument behavior unchanged: `go run ./tools/upstream-sync-guard` still checks `HEAD` against the current working tree.
- Added real temporary-Git-repository tests for committed protected-path deletion, committed historical-migration modification, default uncommitted checks, and invalid base diagnostics.
- Verified with `go test ./tools/upstream-sync-guard -count=1`, `go test ./tools/upstream-sync-guard -cover`, default guard execution, `go run ./tools/upstream-sync-guard --base e79c6f88a`, and `git diff --check`.

## [2026-07-11] feat: Support Bearer auth for Anthropic-compatible API-key accounts

**Affected files**: account auth helper/test, gateway request builders, model
sync, create/edit forms, credentials helper/tests, bilingual locales, and docs.

**Upstream compatibility**: Behavior adaptation of `7869b7fe3`; existing
accounts remain on `x-api-key` unless Bearer is explicitly selected.

**Details**:
- One strict helper removes both candidate auth headers before writing exactly
  one across account test, model sync, messages, passthrough, and count_tokens.
- Create/edit forms omit the default, hydrate Bearer, and delete it on reset.
- OAuth and fork-local billing/display/cache-read/`actual_cost`, Claude-GPT,
  Images, fallback, scheduler/failover, Ops, settings, routes, and migrations
  remain unchanged.
- Focused backend tests, 53 frontend tests, typecheck, and whitespace checks
  passed. No push/deployment.

## [2026-07-11] feat: Align usage ranking, latency health, and BOM CSV export

**Affected files**: admin user-breakdown handler/repository/types; admin and user usage views; ranking/table components; CSV/latency utilities; bilingual i18n; focused tests; usage documentation.

**Upstream compatibility**: Behavior-level adaptation of `b062b3664`, `1a3cc2a78`, and `aee9a7ba9`. The fork's single-file locale structure, requested-model analytics, user-view display transformation, user comparison drawer, and existing usage layout remain authoritative.

**Details**:
- Added an allowlisted per-user ranking query with independent input/output/cache-creation/cache-read totals. Stored `actual_cost`, account cost, and token quantities are read-only aggregates; real cache-read quantities are not rewritten.
- Added a lazy admin ranking view and drilldown back to filtered usage details. Existing chart metrics, user comparison drawer, routes, and browser column preferences are retained; legacy first-token/duration hidden keys migrate to the combined latency column.
- Added shared latency health thresholds and compact long-duration formatting to admin and user usage tables. This is presentation-only and does not change Ops error details, persistence, scheduling, or billing.
- Intentionally restored user CSV export after the earlier UI removal. It pages through the user-owned `/usage` contract, exports only user-visible fields, uses display-transformed token/cost values already returned by that endpoint, escapes spreadsheet formulas, and writes UTF-8 BOM bytes for Chinese Excel compatibility. No admin account cost or internal account/user columns are exported.
- Verified focused Go repository/handler tests, Vitest ranking/latency/CSV/admin/user view suites, frontend typecheck/build, sync guard, and whitespace checks. The repository lint command is blocked in this isolated install because `vue-eslint-parser` is only transitive and is not linked for `.eslintrc.cjs`; no dependency metadata was changed in this sync batch. No push/deploy.

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

## 閺嶇厧绱＄拠瀛樻

```
## [閺冦儲婀 缁鍩? 缁犫偓閻厽寮挎潻?

**瑜板崬鎼烽懠鍐ㄦ纯**: 濞戝寮烽惃鍕侀崸?閺傚洣娆?
**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺勵垰鎯侀崣顖濆厴娑撳簼绗傚〒鍛婃纯閺傛澘鍟跨粣?
**閸欐ɑ娲跨拠锔藉剰**:
- 閸忚渹缍嬫穱顔芥暭閸愬懎顔?

**閸忓疇浠?Issue/PR**: #xxx閿涘牆顩ч張澶涚礆
```

---

## 閸欐ɑ娲跨拋鏉跨秿

## [2026-07-11] merge: Integrate bridge hardening into upstream alignment

**Affected files**: Claude-GPT bridge routing/count-token handler, service, routes, focused tests and docs; image-channel manual edit test UI/API and focused tests; upstream-sync guard catalog and tests.
**Compatibility**: High-sensitivity branch integration. Merges `main@e091d99bb` into `codex/upstream-alignment-20260711@e462c04f2` while preserving both the fork-local bridge hardening and the upstream-alignment scheduler, quota-platform, request-body, header-override, Grok route, billing/display, and image contracts.
**Details**:
- Reconciled the independently added `count_tokens` implementation around the current scheduler signature, platform quota eligibility, configurable lenient JSON/body limits, account header overrides, bridge route diagnostics, Ops context, ready-path upstream counting, simple-mode platform candidates, and bounded local estimation.
- Kept Grok `count_tokens` explicitly unsupported, retained bridge mapping intent without native fallback, and replaced the obsolete second account scan with `ClaudeGPTBridgeRouteDecision.MappedUpstreamModel`.
- Updated upstream-sync protection to require the diagnosis-carried mapped model and the 8 MiB tokenizer bound instead of the removed `ResolveClaudeGPTBridgeCountUpstreamModel` helper.
- Preserved stored billing, quota deduction, `actual_cost`, display-token transformations, real cache-read token quantities, curated/default models, OpenAI Images/Batch Image, scheduler/failover, Ops settings, routes, and bilingual locale contracts.
- Verification passed: backend `go test -tags=unit ./...`; frontend 143 files / 841 tests, typecheck, ESLint, and production build; CGO-disabled server build; upstream-sync guard in default and `--base 0e24044d` modes; `git diff --check`.

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
## [2026-07-11] fix: Harden bridge candidacy, cancel handling, and route observability after second-round review

**Affected files**: `backend/internal/service/account.go`, `backend/internal/handler/openai_gateway_handler.go`, `backend/internal/handler/openai_claude_gpt_bridge_route.go`, `backend/internal/handler/openai_gateway_count_tokens.go`, `backend/internal/service/openai_claude_gpt_bridge_routing.go`, `backend/internal/service/openai_claude_gpt_bridge_routing_test.go`, `backend/internal/service/openai_claude_gpt_bridge_forward_test.go`, `backend/internal/handler/openai_claude_gpt_bridge_route_test.go`, `backend/internal/handler/openai_gateway_count_tokens_test.go`, `backend/internal/server/routes/gateway_bridge_dispatch_test.go`, `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md`, `docs/dev/codebase/gateway.md`
**Compatibility**: Low risk. Tightens bridge candidacy to the documented account-level explicit-mapping contract (platform default mappings never create bridge intent), aligns Messages cancel semantics with the Responses path, and completes route_decision observability. No schema, frontend, or wiring changes.
**Details**:
- Independent second-round multi-agent review of the P0/P1 delivery (59 agents, 9 confirmed findings) drove this round; full record in the investigation doc status section.
- `ResolveClaudeGPTBridgeModel` now requires `ModelMappingSourceAccount`: an admin-saved OpenAI platform default mapping (including `claude-*` wildcards) no longer turns every bridge-enabled account into a candidate for every Claude model, which under strict routing would have permanently hijacked native Antigravity requests onto the GPT upstream.
- Bridge Messages error path gains the same `openAIClientRequestCanceled` early return as Responses: a client cancel records no account failure, no account switch, and never continues failover with a canceled context (previously one cancel could down-rank up to maxAccountSwitches+1 healthy accounts).
- `route_decision` events add spec-mandated `attempt` and `terminal_outcome` fields; selection-race re-diagnosis measures real `latency_ms` instead of always zero.
- Coverage backfill for review-confirmed test gaps: real-path two-request 429 regression (upstream 429 through `ForwardAsAnthropic` really persists `RateLimitResetAt`) plus `UpstreamFailoverError.ResponseHeaders` population; routes-level end-to-end tests of the real dispatch switch for `/v1/messages`, `/antigravity/v1/messages`, and `count_tokens` with native-not-called sentinels; bridge count ready-path tests (mapped-model upstream count, 500-to-local-estimate degradation) via a new `SetHTTPUpstreamForTest` injector.

## [2026-07-11] fix: Reuse manual image-edit input pool and restore multipart submission

**Affected files**: `frontend/src/utils/imageChannelManualTest.ts`, `frontend/src/utils/imageChannelManualTest.test.ts`, `frontend/src/views/admin/ImageChannelMonitorView.vue`, `frontend/src/views/admin/ImageChannelMonitorView.manual.test.ts`, `frontend/src/api/admin/imageChannelMonitor.ts`, `frontend/src/api/admin/imageChannelMonitor.image.test.ts`, `frontend/src/i18n/locales/zh.ts`, `frontend/src/i18n/locales/en.ts`
**Compatibility**: Low risk, admin image-monitor manual tests only. No backend change; the backend already accepted per-request multipart uploads regardless of duplicated pixel content.
**Details**:
- Manual image-edit runs no longer require one exclusive input image per concurrent request (c16 previously demanded 16 distinct uploads). The pool now needs at least 1 image and assigns images to runs in round-robin order; the assignment lives in `buildManualRunRequests` and is returned per request, so the uploaded blob can never drift from the payload's `input_image_name`/`input_image_type`.
- Fixed every manual edit run failing instantly with `api_key_id is required for gateway manual tests` even in direct-probe mode: the client-wide axios `Content-Type: application/json` default made axios 1.x rewrite the edit `FormData` through `formDataToJSON` into a JSON body, so the backend JSON binding saw zero values for every real field (`execution_mode`, `api_key_id`, `client_run_id`, batch fields), and an empty `execution_mode` defaults to `gateway_account` whenever the manual gateway is configured. `manualTest` now posts `FormData` with an explicit `multipart/form-data` override (same idiom as the tutorial-page upload API).
- Input-pool UI: the counter chip reads "宸查€?X 寮?/ N 鏉¤姹?, the empty-pool warning explains that one image can be reused, and a neutral hint appears when the pool is smaller than the planned run count.
- Regression coverage: utils round-robin distribution, single-image reuse across all runs, and empty-pool rejection; a view-level launch of 3 concurrent edit runs reusing one uploaded image; API-layer assertions that edit runs post multipart with the explicit override while generate runs stay plain JSON.
- Verification: targeted vitest suites (utils 24, view 20, API 6 tests), `pnpm run typecheck`, `pnpm run lint:check`, and a live browser run against the local stack 鈥?4 concurrent direct-probe edit requests sharing one input image all reached the backend as multipart `direct_probe` (HTTP 200) and completed with real generated 1536x1024 images via URL delivery.

## [2026-07-11] fix: Claude-GPT bridge strict routing (P0)

**Affected files**: `backend/internal/service/openai_claude_gpt_bridge_routing.go`, `backend/internal/service/openai_claude_gpt_bridge_routing_test.go`, `backend/internal/handler/openai_claude_gpt_bridge_route.go`, `backend/internal/handler/openai_claude_gpt_bridge_route_test.go`, `backend/internal/handler/openai_gateway_handler.go`, `backend/internal/server/routes/gateway.go`, `backend/tools/upstream-sync-guard/main.go`, `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md`, `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md`, `docs/dev/codebase/gateway.md`
**Compatibility**: Medium risk, Antigravity bridge groups only. Native-only groups keep identical behavior (`not_configured` is the only native path). Behavior change: a configured bridge whose accounts are all temporarily blocked now returns bridge 429/503 instead of silently retrying through the (possibly empty) native Antigravity pool; admin-paused bridge accounts also stay on bridge 503.
**Details**:
- Implemented the 2026-07-10 investigation P0: `ResolveClaudeGPTBridgeRoute` diagnoses `not_configured/ready/rate_limited/unavailable/probe_error` from `AccountRepository.ListByGroup` without acquiring scheduler slots, separating stable mapping intent from instantaneous capacity.
- `routes/gateway.go` dispatches Antigravity `/v1/messages` by route action; `rate_limited` returns Anthropic 429 `rate_limit_error` with `Retry-After` (earliest future recovery, rounded up, min 1s), `unavailable` returns 503 `overloaded_error`, `probe_error` returns 503 `api_error`, and protocol errors return canonical 400 instead of masquerading as a native miss.
- Removed `ShouldUseClaudeGPTBridge`, the hidden `markOpenAIClaudeGPTBridgeFallback` native fallback, and its context key. Selection races and mid-request mapping deletion re-diagnose once (`respondClaudeGPTBridgeSelectionRace`): pure rate limit 鈫?429, otherwise 鈫?bridge-side 503.
- Multi-account bridge failover is preserved; when every attempt fails with 429 the final response stays 429 and propagates a validated upstream `Retry-After` (positive integer, 鈮?6400s).
- Route decisions emit `openai_claude_gpt_bridge.route_decision` (state, candidate/schedulable/rate-limited counts, retry_at, decision_source, latency) with no account identities.
- Added the two-request 429 regression (`429 鈫?cooldown 鈫?next request must be 429, never native`) plus the section-10 test matrix for diagnosis states, Retry-After bounds, streaming-aware race errors, and body preservation for native fallthrough. Updated upstream-sync-guard signatures (including the stale `writeCustomModelsList` entry).
- Post-review hardening (multi-agent adversarial review): Messages forward-path `UpstreamFailoverError` now carries `ResponseHeaders` so the exhausted-all-429 Retry-After propagation actually fires in production; group-blocked models return a stable 403 before capacity 429/503; `Retry-After` from `RateLimitResetAt` is capped at 86400s; simple run mode diagnoses candidates platform-wide to match the scheduler pool instead of silently regressing unbound bridge accounts to native; a rate limit expiring between schedulability checks re-classifies as schedulable instead of 503.

## [2026-07-11] feat: Claude-GPT bridge-aware count_tokens (P1)

**Affected files**: `backend/internal/service/openai_gateway_count_tokens.go`, `backend/internal/service/openai_gateway_count_tokens_test.go`, `backend/internal/handler/openai_gateway_count_tokens.go`, `backend/internal/handler/openai_gateway_count_tokens_test.go`, `backend/internal/service/openai_endpoint_url.go`, `backend/internal/server/routes/gateway.go`, `backend/go.mod`, `backend/go.sum`, `docs/dev/codebase/gateway.md`
**Compatibility**: Medium-low risk. Manual port of official upstream `e316ebf5` count_tokens (PR #3497 + #3635 semantics) with a fork-only bridge adaptation. Groups without a bridge mapping keep the native count path; OpenAI-platform groups gain real token counting instead of a hardcoded 404.
**Details**:
- OpenAI-group `/v1/messages/count_tokens` converts the Anthropic request via `AnthropicToResponses` and calls `POST /v1/responses/input_tokens` (API-key `base_url` aware); OAuth 401/403/404 missing-scope/unsupported falls back to a local tiktoken estimate and never rate-limits, temp-unschedules, or errors the account.
- Antigravity groups with an explicit bridge mapping use `CountTokensClaudeGPTBridge`: `ready` counts upstream with the mapped GPT model (scheduler slot released immediately; bridge-lenient mode answers any upstream failure with a 200 local estimate while keeping `HandleUpstreamError` account bookkeeping), and `rate_limited/unavailable/probe_error` return a 200 local estimate without touching the native pool.
- count_tokens keeps zero usage/billing/concurrency side effects; group model access and billing eligibility checks match the Messages gates.
- Added `github.com/tiktoken-go/tokenizer v0.8.0`; local estimation sample expectations match official upstream exactly (o200k_base default, cl100k_base for gpt-3.5/gpt-4-era models). Estimates log `count_tokens_estimated=true` with an `estimate_reason`.
- Post-review hardening: local estimation is bounded at 8 MiB 鈥?larger converted inputs use a bytes/4 approximation instead of feeding the tokenizer (local-compute DoS guard); bridge count preflight returns a proper 413/400 on body-read errors instead of handing native a consumed empty body; the degraded path reuses the diagnosis-carried mapped model instead of a second account scan; the bridge count path records the same ops request/endpoint/selected-account context as the other count paths.

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

**褰卞搷鑼冨洿**: backend/internal/{pkg/openai/constants.go, service/{openai_model_alias.go,openai_codex_transform.go,models_list_policy.go,pricing_service.go,billing_service.go}(+tests), handler/gateway_models_list_test.go}, backend/resources/model-pricing/model_prices_and_context_window.json, frontend/src/{composables/useModelWhitelist.ts(+test),components/keys/UseKeyModal.vue(+test)}, docs/dev/codebase/{model-mapping.md,billing.md}
**涓婃父鍏煎鎬?*: 涓綆椋庨櫓銆傛寜涓婃父 `6cea1c35` 澧為噺鎺ュ叆 `gpt-5.6-sol`銆乣gpt-5.6-terra`銆乣gpt-5.6-luna`锛屼絾涓嶅仛澶ц寖鍥?upstream merge锛屼笉绉婚櫎鏈湴 GPT-5.5-pro/date銆丆odex Spark銆丆laude-GPT bridge銆佸浘鐗囬€氶亾銆佸睍绀哄€嶇巼绛変簩寮€閫昏緫銆?
**鍙樻洿璇︽儏**:
- OpenAI 榛樿妯″瀷銆乣/v1/models` curated discovery銆佸墠绔?OpenAI 鐧藉悕鍗?棰勮銆丱penCode 閰嶇疆鍔犲叆 GPT-5.6 涓変釜瀹樻柟鍙樹綋銆?
- Codex/OpenAI 妯″瀷褰掍竴鏀寔 `gpt5.6-*`銆乣openai/gpt5.6-*`銆乺easoning-effort 鍚庣紑銆佹棩鏈熷悗缂€鍜?compact 鍚庣紑锛岄伩鍏嶆柊妯″瀷钀藉叆鏃х殑 `gpt-5 -> gpt-5.4` 鍏煎鍏滃簳銆?
- LiteLLM 璧勬簮鏂囦欢鍔犲叆涓婃父 GPT-5.6 pricing/context/service-tier 瀛楁锛涘姩鎬佷环鏍间粛浼樺厛锛岄潤鎬佸厹搴曚粎鍦ㄤ环鏍艰祫婧愮己澶辨椂鍚敤锛屼笖涓嶆敼鍙樼敤鎴?娓犻亾/鍏ㄥ眬/display rate 瑙ｆ瀽閾俱€?
- 榛樿 Claude-GPT bridge 妯℃澘淇濇寔 `claude-opus-4-8/4-7 -> gpt-5.5`銆佸叾浠?Claude 4.x -> `gpt-5.4`锛屽彧鏂板鍙€?OpenAI 棰勮锛屼笉闅愬紡鍗囩骇榛樿妗ユ帴鐩爣銆?
- 楠岃瘉锛歚go test -tags=unit ./internal/pkg/openai ./internal/service ./internal/handler` 閫氳繃锛沗node -e "JSON.parse(...model_prices_and_context_window.json...)"` 閫氳繃锛沗pnpm test:run src/composables/__tests__/useModelWhitelist.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts` 閫氳繃锛沗pnpm exec eslint src/composables/useModelWhitelist.ts src/composables/__tests__/useModelWhitelist.spec.ts src/components/keys/UseKeyModal.vue src/components/keys/__tests__/UseKeyModal.spec.ts` 閫氳繃銆?

## [2026-07-08] feat: 缃戝叧涓婃父缃戠粶閿欒鍙厤缃噸璇?

**褰卞搷鑼冨洿**: backend/internal/{repository/http_upstream.go(+test), service/{http_upstream_port.go,setting_service.go,settings_view.go,domain_constants.go,wire.go,setting_service_update_test.go}, handler/{admin/setting_handler.go,dto/settings.go}, cmd/server/wire_gen.go}, frontend/src/{api/admin/settings.ts,views/admin/SettingsView.vue,i18n/locales/{zh,en}.ts}, docs/dev/codebase/gateway.md
**涓婃父鍏煎鎬?*: 涓綆椋庨櫓銆傜粺涓€ HTTPUpstream 鍑虹珯灞傛柊澧炰紶杈撻敊璇厹搴曪紱浠呭鏈敹鍒?HTTP 鍝嶅簲鐨勮繛鎺ュけ璐?瓒呮椂/EOF/DNS 绛夌綉缁滈敊璇敓鏁堬紝涓嶉噸璇曚笂娓?4xx/5xx 鍝嶅簲锛涗笉鍙噸鏀?request body 涓嶉噸璇曘€?
**鍙樻洿璇︽儏**:
- 鏂板绯荤粺璁剧疆 `gateway_network_retry_max`锛屼綅浜庡悗鍙般€岀郴缁熻缃?- 缃戝叧鏈嶅姟 - 璇锋眰杞彂琛屼负銆嶏紝鍙栧€?0-10锛岄粯璁?2锛? 琛ㄧず鍏抽棴閲嶈瘯銆?
- `repository.HTTPUpstream` 澶栧眰澧炲姞缃戠粶閿欒閲嶈瘯锛氶粯璁ゆ渶澶氶噸璇?2 娆★紙鎬诲皾璇?3 娆★級锛岀煭閫€閬匡紱瑙﹀彂鏃跺啓 `upstream_network_retry` 鏃ュ織锛涘凡鏈変笓鐢ㄩ噸璇曞惊鐜殑 OpenAI OAuth 鍥剧墖 `/responses` 宸ュ叿璺緞閫氳繃涓婁笅鏂囧叧闂叏灞€閲嶈瘯锛岄伩鍏嶆鏁板彔鍔犮€?
- 璁剧疆鏈嶅姟灏嗚瀛楁骞跺叆缃戝叧杞彂琛屼负缂撳瓨锛屼繚瀛樺悗鍒锋柊鐑矾寰勭紦瀛橈紱admin settings API 鏀寔鏈紶瀛楁娌跨敤鏃у€煎苟璁板綍瀹¤ diff銆?
- 鍓嶇琛ラ綈绫诲瀷銆侀粯璁ゅ€笺€佷繚瀛?payload 鍜屼腑鑻辨枃鏂囨銆?
- 楠岃瘉锛歚go test -tags=unit ./internal/repository ./internal/service ./internal/handler/admin ./cmd/server` 閫氳繃锛沗pnpm run typecheck` 閫氳繃銆?

## [2026-07-08] fix: 鍥剧墖娓犻亾鐩戞帶鎵嬪姩鍙傛暟鍖哄鍔犲唴閮ㄤ笅鎷夋粴鍔?

**褰卞搷鑼冨洿**: frontend/src/views/admin/ImageChannelMonitorView.vue, docs/dev/codebase/image-channel-monitor.md
**涓婃父鍏煎鎬?*: 浣庨闄┿€備粎璋冩暣鎵嬪姩妫€娴嬮潰鏉垮乏渚у弬鏁伴厤缃尯鍩熺殑甯冨眬婊氬姩杈圭晫锛屼笉鏀规帴鍙ｃ€佹娴嬮€昏緫鎴栨寔涔呭寲缁撴瀯銆?
**鍙樻洿璇︽儏**:
- 鎵嬪姩妫€娴嬪乏渚у弬鏁伴厤缃潡鏀逛负鍥哄畾鏍囬 + 鏈夐珮搴﹁竟鐣岀殑鍐呴儴婊氬姩姝ｆ枃锛屽唴瀹硅繃楂樻椂鍙悜涓嬫粴鍒伴璁?妯℃澘閫夋嫨鍖哄煙銆?
- 淇濇寔鎵嬪姩闈㈡澘鐨勫浐瀹氳鍙ｈ璁★細涓嶆仮澶嶆暣椤垫粴鍔紝Channels 鍒楄〃鍜屽簳閮ㄥ紑濮?鍙栨秷 CTA 浠嶆寜鍘熷唴閮ㄦ粴鍔ㄥ竷灞€宸ヤ綔銆?
- 鏇存柊 image-channel-monitor 妯″潡鏂囨。锛岃褰曞弬鏁版鏂囦篃鏄乏渚у唴閮ㄦ粴鍔ㄥ尯鍩熶箣涓€锛屽悗缁柊澧炴帶浠朵笉鑳藉啀娆￠殣钘忓簳閮ㄦ帶鍒堕」銆?

## [2026-07-07] feat: 鍥剧墖娓犻亾鐩戞帶鎵嬪姩妫€娴嬫敮鎸佸苟鍙戞壒娆?

**褰卞搷鑼冨洿**: backend/internal/{service/{image_channel_monitor_types.go,image_channel_monitor_service.go(+test)},handler/admin/image_channel_monitor_handler.go}, frontend/src/{api/admin/imageChannelMonitor.ts,views/admin/ImageChannelMonitorView.vue,i18n/locales/{zh,en}.ts}, docs/dev/codebase/image-channel-monitor.md
**涓婃父鍏煎鎬?*: 浣庨闄┿€傛墜鍔ㄦ娴嬩粛鏄紓姝ュ唴瀛?run + 鍓嶇鏈湴鍘嗗彶锛屼笉鏀?`image_channel_monitor_histories` 瀹氭椂鐩戞帶鍘嗗彶琛紝涔熶笉鏀瑰彉 scheduled check 璇箟銆?
**鍙樻洿璇︽儏**:
- 鎵嬪姩妫€娴嬪弬鏁板尯鏂板骞跺彂鏁帮紝鐐瑰嚮寮€濮嬪悗鎸?`閫変腑娓犻亾鏁?脳 骞跺彂鏁癭 灞曞紑鐙珛妫€娴嬭褰曪紱鍓嶇闄愬埗鍗曟笭閬撳苟鍙?1-20銆佸崟杞€昏褰?100 鏉★紝閬垮厤璇搷浣滃帇鍨祻瑙堝櫒鎴栦笂娓搞€?
- 鍚庣 manual run 璇锋眰/鍝嶅簲鏂板 `batch_id`銆乣batch_size`銆乣batch_index`锛岃疆璇笌鍙栨秷鍝嶅簲淇濇寔鍚屼竴鎵规鏍囪瘑锛沗StartManualCheck` 鍗曟祴瑕嗙洊鎵规瀛楁淇濈暀銆?
- 鍓嶇 `manualResults` 浠庢寜娓犻亾 ID 瀛樺偍鏀逛负鎸夊崟鏉?recordId 瀛樺偍锛屽悓涓€娓犻亾鍙悓鏃舵樉绀哄鏉″苟鍙戣褰曪紱鎵嬪姩璁板綍琛ㄦ柊澧炪€屾壒娆°€嶅垪锛岃鎯呭脊绐楁柊澧炴壒娆?搴忓彿/骞冲潎鑰楁椂鎸囨爣銆?
- 娴忚鍣ㄦ湰鍦版墜鍔ㄥ巻鍙蹭繚瀛樻壒娆″瓧娈典笌 `batch_average_elapsed_ms`锛涘悓鎵硅褰曞畬鎴愭椂鍥炲～骞冲潎鑰楁椂锛屾棫鍘嗗彶/棰勮鏁版嵁鍏煎榛樿鍊硷紱鎵嬪姩棰勮鍚屾淇濆瓨骞跺彂鏁般€?
- 楠岃瘉锛歚pnpm --dir frontend run typecheck` 閫氳繃锛沗go test -tags=unit ./internal/service -run TestImageChannelMonitorStartManualCheckRunsAsyncAndPollsResult` 閫氳繃銆?

## [2026-07-06] feat: 鍥剧墖娓犻亾鐩戞帶/鎵嬪姩娴嬭瘯鏀寔 response_format 鎷垮浘鏂瑰紡閫夋嫨

**褰卞搷鑼冨洿**: backend/{migrations/179, ent/schema/{image_channel_monitor,image_channel_monitor_history}.go(+regen), internal/service/{image_channel_monitor_types.go, image_channel_monitor_service.go(+test)}, internal/repository/image_channel_monitor_repo.go, internal/handler/admin/image_channel_monitor_handler.go}, frontend/src/{api/admin/imageChannelMonitor.ts, views/admin/ImageChannelMonitorView.vue, i18n/locales/{zh,en}.ts}
**涓婃父鍏煎鎬?*: 浣庨闄┿€傛柊澧炶縼绉?179锛坢onitors/histories 鍚勫姞 response_format 鍒?瀛橀噺鍥炲～ 'url' 涓庢棫寮哄埗琛屼负涓€鑷达級;imageMonitorMaxResponseBytes 2MB鈫?4MB(瀹圭撼 b64 鍐呰仈澶у浘);閰嶅悎 8611221ba(缃戝叧閫忎紶鏄惧紡 response_format)銆?
**鍙樻洿璇︽儏**:
- 娓犻亾鐩戞帶涓庢墜鍔ㄦ祴璇曞潎鍙€夋嬁鍥炬柟寮?URL / Base64 / 涓嶄紶(璺熼殢涓婃父榛樿),瀵瑰簲 payload 甯?response_format=url / b64_json / 鐪佺暐鍙傛暟;JSON 涓?multipart(鍥剧敓鍥?edits)涓ゆ潯璺緞鍚屾銆?
- 璇箟:浠?url 妯″紡涓?b64 杩斿洖瑙嗕负浜や粯澶辫触(缁存寔鏃х洃鎺ц涔?;b64_json/涓嶄紶妯″紡鎺ュ彈 b64 杩斿洖涓烘甯?鍐呰仈鍥剧墖鍏冩暟鎹?灏哄/澶у皬)鐓у父瑙ｆ瀽銆?
- 鍘嗗彶璁板綍:姣忔妫€鏌ョ殑鎷垮浘鏂瑰紡鍐欏叆 histories 骞跺湪瀹氭椂鍘嗗彶寮圭獥鏂板銆屾嬁鍥炬柟寮忋€嶅垪;鎵嬪姩妫€娴嬭褰曡鎯呭脊绐楁柊澧炲悓鍚嶆寚鏍?鎵嬪姩棰勭疆(preset)涓庢湰鍦板巻鍙插悓姝ヤ繚瀛樿瀛楁,鏃ф暟鎹洖钀?url銆?
- 鏂板缓娓犻亾/鎵嬪姩娴嬭瘯琛ㄥ崟榛樿 url(琛屼负涓嶅彉),闇€瑕佹祴 base64 鎴栬窡闅忎笂娓告椂鏄惧紡鍒囨崲銆?
- 楠岃瘉:鍚庣鏂板涓夋€?payload/璋冨害鎺ュ彈鎬у崟娴?鍏ㄩ噺 unit 閫氳繃;鍓嶇 typecheck/lint/鐩稿叧 vitest 閫氳繃;娴忚鍣ㄥ疄娴嬬紪杈戣〃鍗曞洖濉?搴撴敼 b64_json 鍚庢纭樉绀?銆佹墜鍔ㄩ潰鏉块€夐」銆佸巻鍙插垪娓叉煋,鏃犳帶鍒跺彴鎶ラ敊銆?

## [2026-07-06] feat: 鍥剧墖娓犻亾鐩戞帶鐘舵€佹椂闂寸嚎 + 鐢ㄦ埛渚у叕寮€灞曠ず

**褰卞搷鑼冨洿**: backend/{migrations/178, ent/schema/image_channel_monitor.go(+regen), internal/service/{image_channel_monitor_types.go, image_channel_monitor_service.go(+test), ops_cleanup_service.go, wire.go}, internal/repository/image_channel_monitor_repo.go, internal/handler/{image_channel_monitor_user_handler.go(鏂?test), handler.go, wire.go, admin/image_channel_monitor_handler.go}, internal/server/routes/{admin.go, user.go}, cmd/server/wire_gen.go(鎵嬪伐瀵归綈)}, frontend/src/{api/{admin/imageChannelMonitor.ts, imageChannelMonitor.ts(鏂?}, components/{admin/ImageMonitorStatusDialog.vue(鏂?, user/monitor/{ImageMonitorCard.vue(鏂?, ImageMonitorDetailDialog.vue(鏂?, __tests__/ImageMonitorCard.spec.ts(鏂?}}, views/{admin/ImageChannelMonitorView.vue, user/ChannelStatusView.vue}, i18n/locales/{zh,en}.ts}
**涓婃父鍏煎鎬?*: 浣庨闄┿€傛柊澧炶縼绉?178锛坕mage_channel_monitors 鍔?public_visible/public_name 涓ゅ垪锛夛紱`NewOpsCleanupService` 绛惧悕鍔?imageChannelMonitorSvc 鍙傛暟锛坵ire_gen 宸叉墜宸ュ榻愶級锛沗Handlers` 瀹瑰櫒鍔?ImageChannelMonitorUser锛沘dmin List 鍝嶅簲姣忛」杩藉姞 timeline/availability_7d 瀛楁锛堝閲忥紝涓嶇牬鍧忔棫娑堣垂鏂癸級銆傝璁℃枃妗?docs/superpowers/specs/2026-07-06-image-monitor-status-timeline-design.md銆?
**鍙樻洿璇︽儏**:
- 绠＄悊绔洃鎺у垪琛ㄦ瘡琛屽唴宓岃糠浣犵姸鎬佹潯锛堝鐢ㄧ敤鎴蜂晶 MonitorTimeline 60 鏍规煴锛? 7 澶╁彲鐢ㄧ巼锛涙柊澧炪€岀姸鎬佽鎯呫€嶅脊绐楋細24h/7d/30d 绐楀彛鍒囨崲 + chart.js 娣峰悎鍥撅紙API 鎬昏€楁椂/鍥剧墖涓嬭浇涓ゆ潯鎶樼嚎 + 澶辫触娆℃暟绾㈣壊鏌憋紝绌烘《鏂嚎锛? 鍙敤鐜?娆℃暟/澶辫触/骞冲潎/鏈€澶ц€楁椂姹囨€诲崱銆?
- 鏁版嵁绛栫暐锛氫笉寤?rollup 琛紝鍏ㄩ儴瀵瑰師濮嬪巻鍙插疄鏃?SQL 鑱氬悎锛坋poch-floor 鍒嗘《 24h鈫?0min/7d鈫?h/30d鈫?d锛涙壒閲忚繎 60 娆?ROW_NUMBER 娑?N+1锛涗笁绐楀彛鍙敤鐜囧崟鏉?FILTER 鑱氬悎锛夈€?
- 鍘嗗彶淇濈暀锛氭縺娲?DeleteHistoryBefore 姝讳唬鐮侊紝RunDailyMaintenance 鐗╃悊鍒?30 澶╁墠鏄庣粏锛屾寕杩?ops 姣忔棩娓呯悊锛堝悓 cron/棰嗗閿侊級锛屼慨澶嶅巻鍙茶〃鏃犻檺澧為暱闂銆?
- 姣忔笭閬撳叕寮€閰嶇疆锛歱ublic_visible锛堥粯璁や笉鍏紑锛? public_name锛堟帺鐩栧唴閮ㄥ懡鍚嶏紝绌哄洖钀芥笭閬撳悕锛夛紝缂栬緫琛ㄥ崟鏂板銆岀敤鎴蜂晶灞曠ず銆嶅尯鍧椼€?
- 鐢ㄦ埛渚?/monitor 娓犻亾鐘舵€侀〉鏂板銆岀敓鍥炬笭閬撱€嶅垎缁勶細鍗＄墖锛堢敓鍥捐€楁椂/鍥剧墖涓嬭浇/绐楀彛鍙敤鐜?60 鏍规椂闂寸嚎锛宔mpty 鐘舵€佷腑鎬у窘绔狅級+ 绠€鐗堣鎯呭脊绐楋紙7/15/30d 鍙敤鐜?骞冲潎鑰楁椂锛夛紱鍒楄〃涓€娆″甫鍥炰笁绐楀彛鍙敤鐜囷紝绐楀彛鍒囨崲绾墠绔紱璺熼殢椤甸潰 channel_monitor_enabled 闂ㄧ涓庤嚜鍔ㄥ埛鏂般€?
- 瀹夊叏绾㈢嚎锛氱敤鎴蜂晶 DTO 鐧藉悕鍗曪紙缁濅笉涓嬪彂鍐呴儴鍚?endpoint/host/IP/閿欒娑堟伅/error_stage/鍥剧墖 URL/浠ｇ悊璐﹀彿淇℃伅锛夛紝鐧藉悕鍗?JSON key 蹇収娴嬭瘯鍏滃簳銆?
- 楠岃瘉锛氬悗绔叏閲?unit 閫氳繃锛堝惈 9 涓柊鐢ㄤ緥锛夛紱鍓嶇 typecheck/lint/鍏ㄩ噺 vitest 620 鐢ㄤ緥閫氳繃锛堝惈鏂板崱鐗?spec锛夛紱鏈湴娉ㄥ叆 3 澶╁惈澶辫触/闄嶇骇鏁版嵁娴忚鍣ㄥ疄娴嬶細琛屽唴鏉?寮圭獥涓夌獥鍙?鎶樼嚎澶辫触鏌?鐢ㄦ埛渚ф帺鍚嶅崱鐗?璇︽儏寮圭獥/鍝嶅簲鍑€鍖栨娊鏌ュ叏閮ㄦ纭紝楠岃瘉鏁版嵁宸叉竻鐞嗐€?

## [2026-07-06] feat: 鍥剧墖娓犻亾鐩戞帶琛ュ叏杩斿洖鍥剧墖灏哄/澶у皬淇℃伅

**褰卞搷鑼冨洿**: backend/internal/service/image_channel_monitor_service.go(+test), frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**涓婃父鍏煎鎬?*: 浣庨闄┿€傚悗绔粎鍦?b64_json 鍒嗘敮琛ュ～ history 宸叉湁瀛楁锛坕mage_bytes/image_content_type/image_width/image_height锛夛紝涓嶆敼鐘舵€佸垽瀹氫笌璇锋眰琛屼负锛涘墠绔墜鍔ㄦ娴嬭〃鏍兼柊澧炲彲閫夊垪銆?
**鍙樻洿璇︽儏**:
- 鍚庣锛氫笂娓歌繑鍥?b64_json 鏃讹紙gpt-image-1 甯告€侊級鍘熷厛瀹屽叏涓嶈В鏋愬浘鐗囧厓鏁版嵁锛屾柊澧?`fillImageMonitorInlineImageInfo` 瑙ｇ爜 base64 濉厖瀛楄妭鏁般€佸梾鎺?content-type銆丏ecodeConfig 鍙栧楂橈紝瀹氭椂涓庢墜鍔ㄨ矾寰勫叡鐢紱URL+涓嬭浇璺緞鍘熸湁閫昏緫涓嶅彉銆?
- 鎵嬪姩妫€娴嬭褰曡〃鏂板"杩斿洖鍥剧墖"鍒楋紙榛樿鏄剧ず锛屽彲鍦ㄥ瓧娈甸€夋嫨鍣ㄥ叧闂級锛氭樉绀哄疄闄呭楂?+ 鏂囦欢澶у皬锛涘綋璇锋眰 size 涓哄叿浣?WxH 涓斾笌瀹為檯涓嶄竴鑷存椂鐞ョ弨鑹插姞 鈿?璀︾ず锛宼ooltip 娉ㄦ槑璇锋眰灏哄锛坥mit/auto 涓嶆瘮瀵癸級銆?
- 鏌ョ湅璇︽儏寮圭獥鏂板"瀹為檯灏哄/鍥剧墖澶у皬/鍥剧墖鏍煎紡"涓夐」鎸囨爣锛屼笉涓€鑷存椂瀹為檯灏哄鏍囬粍骞跺湪鎸囨爣鍖轰笅鏂规樉绀烘暣鍙ユ彁绀恒€?
- 瀹氭椂鐩戞帶鍘嗗彶寮圭獥"鍥剧墖"鍒楃敱浠呭楂樻敼涓?瀹介珮 路 澶у皬"銆?
- 楠岃瘉锛氬悗绔柊澧炲崟娴嬶紙1x1 PNG b64 鏂█瀹介珮/瀛楄妭/content-type锛夛紝TestImageChannelMonitor* 鍏ㄨ繃锛涘墠绔?typecheck/lint 閫氳繃锛涙湰鍦版祻瑙堝櫒娉ㄥ叆涓€鑷?涓嶄竴鑷?鏃犲浘涓夌被璁板綍锛屽疄娴嬭〃鏍煎垪銆佽绀烘牱寮忋€乼ooltip銆佽鎯呭脊绐楁覆鏌撳潎姝ｇ‘锛屾棤鎺у埗鍙版姤閿欍€?

## [2026-07-04] feat: 瀵煎叆 CCS 瀹㈡埛绔€夋嫨鎵╁睍鈥斺€攁nthropic 瀵嗛挜鏀寔 Codex 瀹㈡埛绔?

**褰卞搷鑼冨洿**: backend/internal/{service/{domain_constants.go, setting_service.go, settings_view.go}, handler/{setting_handler.go, dto/settings.go, admin/setting_handler.go}, server/api_contract_test.go}, frontend/src/{views/user/KeysView.vue, views/admin/SettingsView.vue, api/admin/settings.ts, stores/app.ts, types/index.ts, i18n/locales/{zh,en}.ts}
**涓婃父鍏煎鎬?*: 浣庨闄┿€傛柊澧?Settings KV `ccs_import_anthropic_codex_model`锛堥暅鍍?`ccs_import_codex_model` 鍏ㄩ摼锛岄粯璁ょ┖锛夛紱KeysView 瀵煎叆寮圭獥閫昏緫閲嶅啓涓烘暟鎹┍鍔ㄣ€傝嫢涓婃父鍚庣画涔熸敼 CCS 瀵煎叆闇€浜哄伐姣斿銆?
**鍙樻洿璇︽儏**:
- 瀹㈡埛绔€夋嫨寮圭獥浠?浠?antigravity"鎵╁睍鍒?anthropic + antigravity 骞冲彴锛歛nthropic 瀵嗛挜鍙€?Claude Code / Codex锛圕odex 璧版牴璺緞 `/responses` Responses鈫扐nthropic 妗ワ紝deeplink `app=codex`锛夛紱antigravity 淇濇寔 Claude Code / Gemini CLI锛堟寜浜у搧鍐崇瓥涓嶆彁渚?Codex锛宍/antigravity/*` 涓嬫棤 /responses 璺敱锛夛紱openai/gemini 骞冲彴浠嶆棤寮圭獥鐩存帴鏄犲皠銆?
- 璋冪爺缁撹锛坈c-switch v3.16.5 婧愮爜锛夛細deeplink `app` 鐧藉悕鍗曚负 claude/codex/gemini/opencode/openclaw/hermes锛?*涓嶆敮鎸?claude-desktop**锛圲I 鏈夎椤电浣?parser 鎷掔粷锛夛紱Claude Code CLI 涓庢闈㈢増鍏辩敤 ~/.claude/settings.json锛宍app=claude` 涓€涓叆鍙ｈ鐩栦袱鑰咃紝寮圭獥鏂囨宸叉敞鏄庛€?
- 鏂板绠＄悊绔缃?CCS 瀵煎叆榛樿妯″瀷锛圓nthropic 瀵嗛挜 鈫?Codex 瀹㈡埛绔級"锛歛nthropic 瀵嗛挜閫?Codex 瀵煎叆鏃跺啓鍏?deeplink `model` 鍙傛暟锛屽簲濉湰绔欏彲璋冨害鐨?Claude 妯″瀷鎴栧凡閰嶇疆娓犻亾鏄犲皠鐨勬ā鍨嬪悕锛涚暀绌哄垯 cc-switch 鍥炶惤 gpt-5-codex銆?
- 椤哄甫淇涓ゅ瀛橀噺娴嬭瘯鎹熷潖锛堣 unit-tag 缂栬瘧閿欒鎺╃洊锛夛細`NewUsageHandler` 绛惧悕婕傜Щ鑷?api_contract_test 缂栬瘧澶辫触锛況edeem/history fixture 缂?`batch_redeem_limit_per_user` 瀛楁銆?
- 楠岃瘉锛歡o test -tags=unit ./... 鍏ㄨ繃锛涘墠绔?typecheck/lint/SettingsView+app spec 鍏ㄨ繃锛涙湰鍦版祻瑙堝櫒 E2E 瀹炴祴鍥涚骞冲彴瀵嗛挜鐨勫脊绐楅€夐」涓?deeplink 鍙傛暟锛堝惈绠＄悊绔缃繚瀛樷啋鍏紑璁剧疆涓嬪彂鈫抎eeplink model 鍙傛暟鍏ㄩ摼锛夈€?

## [2026-07-04] feat: 妯″瀷閰嶇疆椤垫墍鏈夎鍙垹闄も€斺€旂洿閫氳鍒犻櫎=鎸佷箙鍖栭殣钘?鍙仮澶?

**褰卞搷鑼冨洿**: backend/internal/{domain/constants.go, service/{domain_constants.go, setting_service.go, wire.go, global_model_pricing_service.go(+test), setting_service_model_mapping_test.go, model_pricing_resolver.go}, handler/admin/model_pricing_handler.go, server/routes/admin.go}, backend/cmd/server/wire_gen.go(鎵嬪伐瀵归綈), frontend/src/{api/admin/modelPricing.ts, components/admin/model-pricing/ModelPricingTab.vue, i18n/locales/{zh,en}.ts}, docs/dev/codebase/model-mapping.md
**涓婃父鍏煎鎬?*: 浣庨闄┿€傛柊澧?Settings KV `model_pricing_hidden_models` 涓?`GET/PUT /admin/model-pricing/hidden-models`;`NewModelPricingHandler` 澧炲姞 settingService 鍙傛暟(wire_gen 宸叉墜宸ュ榻?;鍒楄〃 source 绛涢€夋柊澧炵壒娈婂€?`hidden`銆備笉鏀逛换浣曡璐?璋冨害琛屼负銆?
**鍙樻洿璇︽儏**:
- 鐩撮€氳(璇锋眰=涓婃父,鏉ヨ嚜 LiteLLM 鐩綍/瑕嗙洊,鏃犳槧灏勬潯鐩彲鍒?鏂板"鍒犻櫎"鎸夐挳:纭鍚庢妸妯″瀷鍔犲叆闅愯棌闆嗗悎,鍒楄〃涓嶅啀鏄剧ず;浠呭奖鍝嶆ā鍨嬮厤缃垪琛ㄥ睍绀?涓嶅奖鍝嶈璐逛笌璇锋眰杞彂銆?
- 鏉ユ簮绛涢€夋柊澧?宸查殣钘?瑙嗗浘:鍒楀嚭鍏ㄩ儴闅愯棌妯″瀷(鍚洰褰曚腑宸蹭笉瀛樺湪鐨勫悕瀛?琛?stub 淇濊瘉鍙仮澶?,琛屽唴"鎭㈠"涓€閿繕鍘熴€?
- 闅愯棌姘镐笉鍚炴帀鐪熷疄鏄犲皠:妯″瀷鑷韩鏄湁鏁堟槧灏勯敭鏃?鍗充娇琚殣钘?鏄犲皠琛屼繚鎸佸彲瑙併€?
- 鐪熷疄鏄犲皠鏉＄洰琛屼负涓嶅彉(鍒犻櫎鏄犲皠=浠庡钩鍙伴粯璁ゆ槧灏勮〃绉婚櫎鏉＄洰)銆?

## [2026-07-04] fix: 妯″瀷閰嶇疆椤垫槧灏勮〃褰诲簳閲嶆瀯锛堣鑹蹭笉鍐嶅潔缂╋級+ 娴嬭瘯杩炴帴妯″瀷鍒楄〃骞跺叆骞冲彴鏄犲皠

**褰卞搷鑼冨洿**: backend/internal/service/global_model_pricing_service.go(+test), backend/internal/handler/admin/account_handler.go(+test), backend/internal/pkg/antigravity/claude_types.go, backend/migrations/177_add_fable5_to_default_model_mapping.sql, frontend/src/components/admin/model-pricing/{ModelPricingTab.vue, ModelMappingInlinePopover.vue, modelPricingRows.ts(鏂?, __tests__/modelPricingRows.spec.ts(鏂?}, frontend/src/api/admin/modelPricing.ts, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/model-mapping.md
**涓婃父鍏煎鎬?*: 涓闄┿€俙billing_basis_hint` 鏂板 `mapping_target`/`mapped_from` 瀛楁骞舵柊澧炲鏁?`billing_basis_hints`锛屽崟鏁板瓧娈典繚鐣欏吋瀹癸紱`GET /admin/accounts/:id/models` 鍚勫垎鏀苟鍏ュ钩鍙扮骇榛樿鏄犲皠閿紱杩佺Щ 177 涓轰簩寮€鑷湁缂栧彿銆傚悎骞朵笂娓告椂鑻ヤ笂娓镐篃鏀逛簡妯″瀷閰嶇疆椤甸渶浜哄伐姣斿銆?
**鍙樻洿璇︽儏**:
- 淇鏄犲皠瑙掕壊鍧嶇缉锛氭棫瀹炵幇鎸?same_name > upstream_only > requested_only 鏀舵暃鍗曚竴瑙掕壊锛屾ā鍨?鏃㈡槸鏄犲皠閿張鏄埆浜虹殑鏄犲皠鐩爣"鏃讹紙濡?claude-sonnet-4-5 -> claude-fable-5 涓?claude-sonnet-4-5-20250929 -> claude-sonnet-4-5锛夎嚜韬槧灏勪粠鍒楄〃娑堝け锛屽墠绔妸涓婃父鍚嶇敾鍥炶姹傚悕锛?娣诲姞鏄犲皠鍚庢槧灏勭洰鏍囪鏀瑰洖鍘熷悕"鐨勬牴鍥狅級銆傜幇鍦?hint 鍚屾椂鎼哄甫 mapping_target 涓?mapped_from锛屼笖"鍏ㄩ儴"瑙嗗浘鎸夊钩鍙板悇鍙戜竴鏉?hint銆?
- 鍓嶇琛屾帹瀵奸噸鍐欙紙modelPricingRows.ts锛夛細鏄犲皠琛屽彧鐢辨槧灏勯敭鑷繁鐨勬潯鐩骇鐢燂紝涓嶅啀浠庣洰鏍囨潯鐩弽鍚戝睍寮€+鍘婚噸浜掕俯锛涚函鏄犲皠鐩爣妯″瀷淇濈暀鑷繁鐨勭洿閫氳锛堜慨澶?claude-fable-5 鍙姹備絾鏄犲皠琛ㄩ噷娌℃湁璇ヨ姹傛ā鍨?锛夛紱鎵€鏈夌洿閫氳鎻愪緵"娣诲姞鏄犲皠"鍏ュ彛锛屽脊绐楅濉?from=to=妯″瀷鍚嶏紙淇"澶ч噺琛屾棤娉曠紪杈?鍒犻櫎"鈥斺€旂湡瀹炴潯鐩墠鏈夊垹闄わ紝鐩撮€氳缂栬緫鍗冲缓鏉＄洰锛夈€?
- 淇濆瓨鏄犲皠鐨勫钩鍙颁笌褰撳墠渚涘簲鍟?tab 涓嶅悓鏃惰嚜鍔ㄥ垏 tab锛岄伩鍏?娣诲姞鎴愬姛浣嗙湅涓嶅埌"銆?
- 娴嬭瘯杩炴帴妯″瀷鍒楄〃锛堣处鍙风鐞嗭級锛欰ntigravity 闈為€忎紶璐﹀彿鏀逛负鎸夌敓鏁堟槧灏勮〃璇锋眰閿敓鎴愶紙鍚?[1m]/[2m] 鍙樹綋锛夛紝Claude/Gemini/OpenAI 璐﹀彿骞跺叆瀵瑰簲骞冲彴榛樿鏄犲皠閿紙淇"鏂版坊鍔犳槧灏勭殑璇锋眰妯″瀷鍦ㄦ祴璇曡繛鎺ュ垪琛ㄧ湅涓嶅埌"锛夈€?
- 杩佺Щ 177锛氫负淇濆瓨杩囩殑 antigravity_default_model_mapping 璁剧疆鍙婅处鍙风骇 model_mapping 鍥炲～ claude-fable-5 鍚屽悕鏄犲皠锛堜繚瀛樿〃鏁翠綋鏇挎崲鍐呯疆琛紝鏃╀簬 fable-5 鐨勫瓨閲忚〃缂鸿鏉″鑷?Antigravity 婕忚皟搴︼級銆?

## [2026-07-04] feat: redesign manual image test into a fixed-viewport split console

**Affected files**: frontend/src/views/admin/ImageChannelMonitorView.vue, frontend/src/components/layout/TablePageLayout.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/image-channel-monitor.md
**Upstream compatibility**: frontend-only fork-local layout rework. It does not change backend APIs, schemas, scheduling, manual history storage, browser-local storage keys, or polling/cancel logic. `TablePageLayout` gains an additive `bareTable` prop (default `false`) guarded by `:not(.is-bare)`, so all other consumers are unaffected.
**Change details**:
- Reworked the manual-test panel into a fixed-viewport split console (`bareTable` slot): left column stacks Parameters (collapsible) 鈫?Channels (internal scroll) 鈫?a persistent Start-test CTA bar; right column is the records table with an internal scroll area and a sticky header. The whole panel fits one viewport 鈥?scrolling happens only inside the channel list and the table, never the page.
- Panel switcher moved from two large cards to a compact header + segmented tabs (A), reclaiming ~90px of vertical space.
- Parameters: two-column grid, prompt spans full width, and the separate size-mode + size selects were merged into one dropdown (with a "custom鈥? entry) backed by a `manualSizeChoice` get/set computed over the unchanged `size_mode`/`size`/`custom_size` trio. Collapsing the card shows a one-line summary of chips.
- Presets condensed to dropdown + save/delete at the card foot; naming moved into a save dialog.
- Channels: row list with selected-count pill, select-all/clear, and a channel search filter (`manualFilteredTargets`); internal scroll keeps the page height bounded regardless of channel count.
- Results: running/completion banner (progress x/y, ok/fail counts, cancel) driven by new `manualBatchStats`/`manualBatchProgress` computeds derived from `manualResults` 鈥?zero API changes. Default columns trimmed (mode/model/size hidden by default via the existing field-visibility state); numeric columns right-aligned with `tabular-nums`; compact text actions.
- Detail dialog: added a latency waterfall over the existing `api_header_ms` (start鈫抙eaders) / `api_body_ms` (headers鈫抌ody phase) / `image_download_ms` (download phase) 鈥?confirmed sequential non-overlapping phase durations in `image_channel_monitor_service.go`, so they stack correctly; dropped the now-redundant raw timing metrics.
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
- Added per-key billing-object metadata to mapping hints so multi-source mappings can display the correct `璁¤垂瀵硅薄` for every source key.
- Changed the model configuration table to expand multi-source default mappings into one row per mapping relationship instead of hiding extra mappings behind `+N`.
- Edit, delete, and billing-object actions now operate on each expanded row's source mapping key, so all effective mappings have their own operation entry.
- Added regression coverage for multi-source upstream-only mappings and same-name mappings with aliases.

## [2026-07-03] fix: make effective default mappings fully editable

**Affected files**: backend/internal/service/{setting_service.go,wire.go,global_model_pricing_service.go,global_model_pricing_service_test.go,setting_service_model_mapping_test.go}, frontend/src/api/admin/modelPricing.ts, frontend/src/components/admin/model-pricing/ModelPricingTab.vue, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: fork-local admin model configuration correction. No schema, migration, image-channel monitoring, push, or deployment changes.
**Change details**:
- Changed Antigravity default mapping persistence so a saved table is treated as the full effective table. Saving `{}` now intentionally means no default mappings, preventing deleted built-in mappings from reappearing after reload.
- Changed model-pricing hints to return `mapping_key` and mark effective default mapping key rows editable, including built-in/runtime default and LiteLLM-discovered mapping rows.
- Enabled `璁¤垂瀵硅薄` editing for same-name and upstream-only mapping relationship rows by saving against `mapping_key` instead of the row's pricing model name.
- Updated frontend edit/delete/billing-object actions to operate on `mapping_key`; this fixes rows where the visible pricing model is the mapped target rather than the requested source.
- Verified: targeted service tests for editable hints and empty Antigravity overrides; `pnpm --dir frontend run typecheck`.

## [2026-07-03] fix: add editable billing object for default model mappings

**Affected files**: backend/internal/domain/constants.go, backend/internal/service/{account.go,setting_service.go,global_model_pricing_service.go,gateway_service.go,openai_gateway_service.go}, backend/internal/handler/admin/account_handler.go, backend/internal/server/routes/admin.go, frontend/src/components/admin/model-pricing/{ModelPricingTab.vue,ModelMappingInlinePopover.vue}, frontend/src/api/admin/{accounts.ts,modelPricing.ts}, frontend/src/i18n/locales/{zh,en}.ts, frontend/src/views/admin/ChannelsView.vue, docs/dev/codebase/model-mapping.md
**Upstream compatibility**: fork-local admin model configuration fix. It adds Settings KV entries and admin APIs for per-platform default mapping billing-object overrides, but does not add schema/migration changes and does not touch image channel monitoring.
**Change details**:
- Replaced the model configuration table's derived "鏄犲皠瑙掕壊" label with an editable "璁¤垂瀵硅薄" field for platform default mapping key rows.
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
- Changed the user usage token cell for image requests from request size display to billed-tier display, e.g. `1寮狅紙2K璁¤垂锛塦.
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
- The table-label and per-row billing behavior from this earlier entry was corrected by the later "editable billing object" change above; model configuration now uses `璁¤垂瀵硅薄` with only `requested` and `mapped` choices.
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
- Added an independent scheduler/runner, admin CRUD/run/history endpoints under `/api/v1/admin/image-channel-monitors`, and an admin submenu at `娓犻亾绠＄悊 -> 鍥剧墖娓犻亾鐩戞帶`.
- Added focused service tests for account-source request construction and `b64_json` response handling.
- Verified: `go generate ./ent`; `go test ./internal/service -run TestImageChannelMonitor -count=1`; `go test ./internal/service ./internal/repository ./internal/handler/admin ./cmd/server -run TestDoesNotExist -count=0`; `pnpm run typecheck`. `go generate ./cmd/server` was attempted but blocked by a local Wire tool `go.sum` missing entry, so `wire_gen.go` was manually reconciled.

## [2026-07-03] feat: redesign login page visuals to Figma v2 (purple gradient)

**Affected files**: frontend/src/views/auth/LoginView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: frontend-only visual layer rewrite of the login view; all login logic (auth store flow, Turnstile, TOTP 2FA modal, legal consent dialog, LinuxDo/WeChat/OIDC OAuth sections, backend-mode/password-reset flags, admin login_page overrides) is preserved unchanged. Diverges further from upstream login UI; watch this file on upstream merges.
**Change details**:
- Rebuilt template per the Figma v2 design (file 5DlRiTxu0w28djyDCdl1Xf, frames 25:2 / 25:75): left purple-gradient hero (#2563EB鈫?7C3AED鈫?EC4899) with brand tile, admin-overridable badge/heading/description, a static "live usage bill" sample card, three model cards (Opus 4.7 / GPT-5.4 / Gemini 3.1 Pro) and a 7脳24 / 100% / 0 stats row; right light-theme form with trust badges, mail/lock input icons, gradient submit button, outline register button, and two capability cards (gpt-image-2 / tutorials).
- Mobile: gradient hero with the form card floating over it, forgot-password link, trust chips, and key-usage/docs links (previously desktop-only nav pills).
- Wired the previously unused `login_page.description` admin override into the hero paragraph; form switched from dark to light theme (Turnstile theme dark鈫抣ight).
- i18n: replaced `auth.login.features.*`, `postLoginInfo`, `postLoginDetails`, `keyUsageLink` with new v2 keys (billCard*, modelCard*, stat*, trustBadge*, cap*, mobileHero*, registerButton) in both zh and en; login form title default changed to 娆㈣繋鍥炴潵 / Welcome back, hero heading defaults to 鐧诲綍鍚庯紝鍗冲埢鎺ュ叆 / 鏈€鏂版棗鑸版ā鍨?
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
- Added effective unit-price fields to user usage DTOs and changed user/API-key usage tooltips to use those configured prices instead of reverse-deriving unit prices from rounded display tokens. Explicit display-price overrides win; otherwise the backend resolves the configured model price through the existing User 鈫?Channel 鈫?Global 鈫?LiteLLM/Fallback pricing chain.
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
**Upstream compatibility**: Manual partial sync from upstream commit `db0414233ce324903adc72e858374086da158b4b` (`feat: 閫傞厤 sonnet5`). This intentionally excludes the same upstream commit's unrelated `backend/internal/pkg/anthropicfp/dateline.go` changes and does not include any unfinished local OpenAI/Image work from the current conversation.
**Change details**:
- Added `claude-sonnet-5` to the Claude OAuth default model list so `/v1/models` can expose the model.
- Added the Bedrock default mapping `claude-sonnet-5 -> us.anthropic.claude-sonnet-5-v1`; existing Bedrock region-prefix adjustment still rewrites it according to account `aws_region`.
- Changed the default `context-1m-2025-08-07` beta policy from blanket filter to a Sonnet 5 whitelist: Sonnet 5 direct/Vertex/Bedrock IDs pass, non-whitelisted models continue to filter the beta token.
- Added frontend whitelist/preset entries for Anthropic Sonnet 5 and Bedrock Sonnet 5 so admins can pick the model in account mapping UI.
- Added regression tests for the default Claude model list, Bedrock mapping constants, Bedrock region adjustment, and the Sonnet 5-only 1M context beta whitelist.
- Verified: `go test -tags=unit ./internal/pkg/claude ./internal/domain ./internal/service -count=1`; `pnpm --dir frontend run typecheck`; `pnpm --dir frontend run build`; `go build -tags embed -trimpath ./cmd/server`; `git diff --check`.

## [2026-07-02] feat(billing): display cache creation price 鈥?缂撳瓨鍒涘缓绾冲叆灞曠ず鏀惧ぇ浣撶郴 + 鐢ㄦ埛渚у彲瑙佹€?

**Affected files**: backend/migrations/171_add_display_cache_creation_price.sql, backend/internal/service/{global_model_pricing,user_model_pricing,user_model_pricing_service,global_model_pricing_service}.go, backend/internal/repository/{global_model_pricing_repo,user_model_pricing_repo}.go, backend/internal/handler/admin/{model_pricing_handler,user_model_pricing_handler,usage_handler}.go, backend/internal/handler/dto/display_pricing{,_test}.go, backend/tools/upstream-sync-guard/main.go, frontend/src/types/index.ts, frontend/src/api/admin/{usage,modelPricing,userModelPricing}.ts, frontend/src/views/user/UsageView.vue, frontend/src/views/KeyUsageView.vue, frontend/src/components/admin/usage/{UsageTable,UserViewCompareDrawer}.vue, frontend/src/components/admin/{model-pricing/ModelPricingDetailDialog,user/UserModelPricingModal}.vue, frontend/src/i18n/locales/{zh,en}.ts, docs/dev/codebase/billing.md
**Upstream compatibility**: additive, fork-local銆傛柊澧?DB 鍒?`display_cache_creation_price`锛坓lobal_model_pricing + user_model_pricing_overrides锛孨ULL=鏈厤缃?琛屼负闆跺彉鍖栵級锛汥isplayUsageFields 澧炲姞涓や釜 admin 濂戠害瀛楁锛涚敤鎴?DTO 鏃犳柊 JSON 瀛楁銆倁pstream-sync-guard 宸茬櫥璁?`DisplayCacheCreationPrice` 鍏抽敭绛惧悕銆?
**Change details**:
- 鑳屾櫙锛歛nthropic 骞冲彴璁板綍锛堝 claude-fable-5锛宨nput=2/output=38/cache_creation=42778/$0.54锛夊湪鐢ㄦ埛渚?token 寰堝皯浣嗗緢璐?鈥斺€旂紦瀛樺垱寤?token/鎴愭湰姝ゅ墠瀹屽叏涓嶅弬涓庡睍绀烘崲绠楋紝涓旂敤鎴峰彲鐢?cache_creation_cost/tokens 鍙嶆帹鐪熷疄缂撳瓨鍐欏崟浠枫€?
- 鏍稿績锛坉isplay_pricing.go锛夛細鏂板垎鏀湪 ApplyDisplayTransform 涓妸缂撳瓨鍒涘缓 token 鐩存帴鎸夊睍绀轰环鍙嶇畻鏀惧ぇ锛坉isplay_tokens = 鐪熷疄鎴愭湰 梅 灞曠ず浠凤紝cost 淇濇寔瀹堟亽锛夛紝**涓?cache-read 鐨?premium 鎶樺叆 input 鏈哄埗鍒绘剰涓嶅悓**锛堢敤鎴锋槑纭姹傦細鐩存帴鏀惧ぇ缂撳瓨鍒涘缓鑷韩 token 鏁帮級銆傚畧鍗細灞曠ず浠?0 && tokens>0 && cost>0锛屼笉渚濊禆 display_input_price銆傜嚎鎬у彉鎹?鈫?鑱氬悎缁勪笌閫愯澶╃劧绛変环锛孏etUserDisplayAggregateGroups 闆舵敼鍔ㄣ€?
- 5m/1h 缁嗗垎锛氭柊 helper rescaleCacheCreationBreakdown 绛夋瘮缂╂斁 + 鍑忔硶瀵煎嚭锛屼繚璇?5m+1h==total锛汚pplyUserDisplayRate 鍚屾鎺ュ叆锛堜慨澶嶆棦鏈?缁嗗垎涓嶉殢灞曠ず鍊嶇巼缂╂斁"bug锛夈€?
- 闀夸笂涓嬫枃锛歟ffectiveDisplayPricingForUsageLog 瀵规柊浠蜂箻 LongContextInputMultiplier銆?
- 閰嶇疆閾撅細migration 171锛堝惈 user 琛?NOT VALID 闈炶礋绾︽潫锛屾ā鏉?147锛夆啋 瀹炰綋/涓や釜 raw-SQL repo 鍏ㄦ灇涓剧偣锛坓lobal 4 澶勩€乽ser 5 澶勶級鈫?鏍￠獙锛坴alidateUserModelPricingOverride锛夆啋 admin API锛坓lobal create/partial-update applyFloat銆乽ser create/update/batch锛夆啋 鍓嶇涓や釜瀹氫环琛ㄥ崟锛?/MTok 鍙屽悜鎹㈢畻銆乤pplyDisplaySuggested 浠?cache_write_price 鍙栧缓璁€硷級鈫?i18n zh/en銆?
- Admin 鍙锛欴isplayUsageFields + ComputeDisplayFields 澧炲姞 display_cache_creation_tokens/cost锛沀sageTable 鍙屽垪 tooltip 澧炶锛沀serViewCompareDrawer config_used 鍥炰紶灞曠ず鍒涘缓浠枫€?
- 鐢ㄦ埛渚у彲瑙佹€э紙姝ゅ墠瀹屽叏涓嶆樉绀猴級锛歎sageView.vue 涓?KeyUsageView.vue 鐨?token 寰界珷锛坅mber 鍥炬爣+1h 鏍囩锛夈€乼oken tooltip锛?m/1h 缁嗗垎锛夈€佹垚鏈?tooltip銆乼oken 鍚堣鍧囨覆鏌?cache creation锛沘dmin 涓撳睘 TTL override "R" 寰界珷浠嶄笉涓嬪彂鐢ㄦ埛銆俇sageView.spec.ts 涓や釜鏂█"鐢ㄦ埛渚ч殣钘忕紦瀛樺垱寤?鐨勬棫瑙勬牸娴嬭瘯宸插弽杞€?
- 骞冲彴杈圭晫锛堣蒋 gate锛岃瑙?billing.md 2026-07-02 鑺傦級锛歰penai 鍘熺敓/antigravity OAuth/妗ユ帴/gemini 琛?cache_creation 鎭?0 鈫?no-op锛沘ntigravity 鍒嗙粍鐨?upstream 涓浆/apikey 鍨嬭处鍙疯涓?openai relay 閫忎紶琛岃嫢鍛戒腑宸查厤缃殑 claude-* 妯″瀷浼氬悓鏍锋崲绠楋紙璇箟姝ｇ‘锛夈€?
- **鏈壒涓嶆敼**锛歞isplay_token_rewrite.go锛堜笅娓稿搷搴?CacheCreateMult 浠嶆亽 1.0锛夛紱claude-gpt 妗ユ帴 openai_claude_gpt_bridge_cache_display_settings锛涚湡瀹炶璐归摼銆備笅娓镐竴鑷存€у闇€璺熻繘锛屽墠缃负 gateway_service.go OAuth 娴佸紡 extractSSEUsagePatch 璁¤垂姹℃煋淇锛圥LAN 鏂囨。 Phase 0锛屾湭瀹炴柦锛夈€?
- Verified: `go build ./...`銆乣go test -tags=unit ./internal/handler/... ./internal/service/... ./internal/repository/...` 鍏ㄨ繃锛堟柊澧?8 涓?display_pricing 鐢ㄤ緥锛氭斁澶?鐙珛瀹堝崼/no-op/涓?read premium 澶嶅悎/闀夸笂涓嬫枃鍗曟缂╂斁/ComputeDisplayFields/鍊嶇巼缁嗗垎涓€鑷存€э級锛沗./internal/server -run Contract` 浠?redeem/history 涓€澶?*鏃㈡湁**澶辫触锛堝熀绾垮悓鏍峰け璐ワ紝涓庢湰鏀瑰姩鏃犲叧锛夛紱鍓嶇 typecheck + lint:check + vitest 鍏ㄩ噺 101 鏂囦欢/603 鐢ㄤ緥鍏ㄨ繃銆?

## [2026-07-02] fix(billing): 娴佸紡璁¤垂 patch 鍏堜簬灞曠ず鏀瑰啓鎻愬彇 鈥斺€?淇 display 妯″紡鐪熷疄鎵ｈ垂姹℃煋

**Affected files**: backend/internal/service/gateway_service.go, backend/internal/service/gateway_service_streaming_test.go
**Upstream compatibility**: 鍗曡閲嶆帓,fork-local銆?
**Change details**:
- 鏍瑰洜:processSSEEvent 鍏堝鍏变韩 SSE event map 鍋氬睍绀烘敼鍐?ApplyDisplayMultipliersToUsageMap 灏卞湴鍙樺紓),鍚?extractSSEUsagePatch 浠庡悓涓€ map 鎻愬彇璁¤垂 鈫?mergeSSEUsagePatch 鈫?ForwardResult.Usage 鈫?calculateTokenCost銆俙downstream_usage_token_mode=display`(migration 169 璧锋柊鐢ㄦ埛榛樿)涓斿睍绀哄€嶇巼闈炲钩鍑℃椂,**鐪熷疄鎵ｈ垂鎸夊睍绀?token 璁＄畻**(鐢熶骇宸查厤缃睍绀哄€嶇巼,姹℃煋宸插疄闄呭彂鐢?銆?
- 淇硶:extractSSEUsagePatch 涓婄Щ鍒?cache TTL override(鍒绘剰褰卞搷璁¤垂褰掔被,淇濇寔鍦ㄥ墠)涔嬪悗銆乨isplay 鏀瑰啓涔嬪墠;display 鏀瑰啓浠嶄綔鐢ㄤ簬鍙戠粰瀹㈡埛绔殑搴忓垪鍖栧璞?灞曠ず璇箟涓嶅彉銆傞『甯︿慨澶?marshal 澶辫触鍥為€€璺緞"瀹㈡埛绔鐪熷疄鍊笺€佽璐圭敤灞曠ず鍊?鐨勪笉鑷唇銆?
- 褰卞搷闈?鎵€鏈夎蛋 GatewayService 娴佸紡璺緞鐨勮处鍙?anthropic OAuth/SetupToken/ServiceAccount/APIKey + antigravity 鍒嗙粍 apikey 鍨嬭处鍙?銆?*琛屼负鍙樺寲:display 妯″紡鐢ㄦ埛鐨勬祦寮忔墸璐逛粠姹℃煋鍊兼仮澶嶄负鐪熷疄鍊?*(宸叉媿鏉垮彧淇+璁板綍,涓嶅仛鍘嗗彶淇)銆傚叾浣欒矾寰勭粡涓夎疆鎺㈢储鏍稿疄鍧囦负"鍏堟彁鍙栧悗鏀瑰啓",瀹夊叏:passthrough 娴佸紡/闈炴祦寮忋€佹爣鍑嗛潪娴佸紡銆乧laude-gpt 妗ユ帴(response-only)銆丱penAI 鍘熺敓鍏ㄨ矾寰勩€乤ntigravity(hook 鍙樺紓 usageToMap 鍏ㄦ柊鎷疯礉,璁¤垂璧扮嫭绔嬬疮璁″瓧娈?銆?
- 绾?缁垮洖褰?TestGatewayService_StreamingDisplayModeBillsRealTokens(淇鍓嶇孩)銆乀estGatewayService_StreamingDisplayModeKeepsTTLOverrideBeforeBillingPatch(TTL 褰掔被浠嶅厛浜庢彁鍙?銆?

## [2026-07-02] feat(billing): cache_write_1h_price 鈥斺€?1h 缂撳瓨鍒涘缓鎸夋孩浠峰垎妗ｈ璐?

**Affected files**: backend/migrations/172_add_cache_write_1h_price.sql, backend/internal/service/{global_model_pricing,global_model_pricing_service,model_pricing_resolver}.go, backend/internal/repository/global_model_pricing_repo.go, backend/internal/handler/admin/model_pricing_handler.go, backend/internal/service/model_pricing_resolver_test.go, frontend/src/api/admin/modelPricing.ts, frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: additive銆傛柊鍒?NULL = 鍘嗗彶琛屼负閫愬瓧鑺備笉鍙?鍥炲綊閽夋祴璇?銆?
**Change details**:
- 鑳屾櫙:瀹樻柟缂撳瓨鍐欏叆鍒嗕袱妗?5m=1.25脳杈撳叆浠?1h=2脳杈撳叆浠?銆傘€?026-07-02 淇銆戣蛋 LiteLLM 婧愪环鐨勬ā鍨?sonnet-5/fable-5)鏈氨鎸夊畼鏂瑰垎妗ｆ纭璐光€斺€旂敓浜?sonnet-5 绾?1h 琛岄殣鍚?$4.0/MTok = 瀹樻柟浼樻儬鏈?1h 浠?2脳$2),缁忓畼鏂逛环鐩〃鏍稿疄,鍘?1h 婧环婕忚"璇婃柇涓嶆垚绔嬨€傝鍘嬪钩鐨勬槸閰嶄簡鍏ㄥ眬 cache_write_price 瑕嗙洊鐨勬ā鍨?opus 绯诲垪 $10 骞充环銆乻onnet-4-6 $5 骞充环):鍗曚竴瑕嗙洊浠峰悓鍐欎笁妗?1h 婧环鏃犳硶琛ㄨ揪鈥斺€旀湰瀛楁鍗充负姝よ€岃銆?
- 鍏ㄥ眬瀹氫环瑕嗙洊鏂板 cache_write_1h_price(migration 172):閰嶇疆鍚?applyGlobalPricingOverride 鍗曠嫭鍐?CacheCreation1hPrice 骞跺己鍒?SupportsCacheBreakdown=true,computeCacheCreationCost 鎸?5m脳p5m+1h脳p1h 鍒嗘。;admin 琛ㄥ崟鍔?1h 缂撳瓨鍐欏叆浠?杈撳叆妗?$/MTok),i18n zh/en銆?
- **杩愯惀鍔ㄤ綔**:閮ㄧ讲鍚庣粰 claude-sonnet-5 / claude-fable-5 绛変腑杞ā鍨嬮厤缃?1h 浠?鎸変笂娓稿疄闄呮墸璐瑰彛寰?;姝ゅ悗鏂拌姹傜湡瀹炴垚鏈鍏?1h 婧环(admin 鎴愭湰涓庣敤鎴?actual_cost 鍚屾鍙樺寲)銆?
- 娴嬭瘯:绾?1h 鐢熶骇褰㈢姸(66061 tokens)鎸?1h 浠疯璐广€佹贩鍚堣鍒嗘。銆佹湭閰嶇疆鏃跺钩浠疯涓哄洖褰掗拤銆?

## [2026-07-02] feat(billing): 涓嬫父鍝嶅簲 usage 缂撳瓨鍒涘缓灞曠ず鏀瑰啓(real/display 鍙屾ā寮?

**Affected files**: backend/internal/service/display_token_rewrite{,_test}.go, docs/dev/codebase/billing.md
**Upstream compatibility**: fork-local銆俽eal 妯″紡闆跺彉鍖?display 妯″紡浠呭湪閰嶇疆浜?display_cache_creation_price 鐨勬ā鍨嬩笂婵€娲汇€?
**Change details**:
- computeDisplayTokenMultipliers 鎺ュ叆缂撳瓨鍒涘缓:CacheCreateMult(鏃犳槑缁嗗洖閫€,5m 妗ｅ彛寰勫榻愯璐瑰洖閫€)+ CacheCreate5mMult/CacheCreate1hMult 鍒嗘。鍊嶇巼(鐪熷疄妗ｄ环梅灞曠ず鍒涘缓浠?;displayTokenPricingConfig/涓や釜 merge 鍑芥暟琛ョ湡瀹炰环涓庡睍绀轰环绠￠亾;IsNonTrivial 绾冲叆鍒嗘。鍒ゆ柇(浠呴厤灞曠ず鍒涘缓浠峰嵆鍙縺娲绘敼鍐欓摼)銆?
- 鏂?helper computeDisplayCacheCreationBreakdown:鏈夊祵濂?5m/1h 鏄庣粏鏃舵寜妗ｅ弽绠?displayTotal脳灞曠ず浠?== 5m脳p5m+1h脳p1h,涓?usage 椤垫垚鏈弽绠楀彛寰勯€?token 涓€鑷?鍚函 1h 涓浆娴侀噺),display1h 鍑忔硶瀵煎嚭淇濊瘉 5m+1h==椤跺眰;鏃犳槑缁嗛€€鍖栧崟涓€鍊嶇巼銆傛帴鍏?rewriteSeparatedUsageTokens(passthrough 娴佸紡/闈炴祦寮?妗ユ帴,椤跺眰涓庡祵濂楀悓姝?sjson 鍥炲啓)涓?ApplyDisplayMultipliersToUsageMap(鎵樼娴佸紡+antigravity hook;antigravity map 鏃犲祵濂?琛屼负涓嶅彉)銆俛pplyOpenAIResponsesUsageDisplayMultipliers 鐨?CacheCreationInputTokens 鏀逛负鍚岃鍒欑缉鏀?妗ユ帴鎭?0,no-op)銆?
- RateScale(灞曠ず鍊嶇巼灞?鍦ㄥ垎妗ｅ弽绠楀悗澶嶅悎,涓?ApplyUserDisplayRate 涓茶仈璇箟涓€鑷淬€?
- 鍓嶇疆渚濊禆:鍚屾棩鐨勬祦寮忚璐?patch 椤哄簭淇(鍚﹀垯缂撳瓨鍒涘缓璁¤垂浼氳鏈敼鍐欐薄鏌?銆?
- Verified: go build/vet;display token 鍏ㄩ儴鐢ㄤ緥(鏃㈡湁 11 + 鏂板 8:鍒嗘。鍊嶇巼璁＄畻/鐢ㄦ埛绾ц鐩栦紭鍏?宓屽鍚屾/绾?1h 鐢熶骇褰㈢姸/RateScale 澶嶅悎/鏃犲祵濂楀洖閫€/OpenAI 缁撴瀯缂╂斁/trivial no-op);gateway 娴佸紡涓?handler/repository 鍏ㄩ噺鍗曟祴閫氳繃銆?

## [2026-07-02] feat(billing): 5m/1h 缂撳瓨鍒嗘。浠锋牸閰嶇疆闈㈣ˉ鍏紙鐢ㄦ埛绾х湡瀹炰环 + 鍏ㄥ眬/鐢ㄦ埛绾у睍绀轰环 + LiteLLM 鍙傝€冿級

**Affected files**: backend/migrations/173_add_cache_tier_pricing_fields.sql, backend/internal/service/{global_model_pricing,user_model_pricing,user_model_pricing_service,global_model_pricing_service,model_pricing_resolver,display_token_rewrite}.go, backend/internal/repository/{global_model_pricing_repo,user_model_pricing_repo}.go, backend/internal/handler/admin/{model_pricing_handler,user_model_pricing_handler,usage_handler}.go, backend/internal/handler/dto/display_pricing{,_test}.go, backend/internal/service/{display_token_rewrite_test,model_pricing_resolver_test}.go, backend/tools/upstream-sync-guard/main.go, frontend/src/api/admin/{modelPricing,userModelPricing,usage}.ts, frontend/src/components/admin/{model-pricing/ModelPricingDetailDialog,user/UserModelPricingModal,usage/UserViewCompareDrawer}.vue, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: additive銆俶igration 173 鏂板涓夊垪鍧?NULL=琛屼负闆跺彉鍖?LiteLLMPrices 杞借嵎鍔?cache_write_1h_price(鏉ヨ嚜 litellm 鐨?cache_creation_input_token_cost_above_1hr)銆?
**Change details**:
- **鐢ㄦ埛绾х湡瀹?1h 浠?* `user_model_pricing_overrides.cache_write_1h_price`:applyUserModelPricingOverride 涓庡叏灞€鍚岃涔?鍗曠嫭鍐?CacheCreation1hPrice + 寮哄埗 SupportsCacheBreakdown),鐢ㄦ埛绾т篃鑳借〃杈?1h 婧环鍒嗘。璁¤垂銆?
- **灞曠ず浠峰垎妗?* `display_cache_creation_1h_price`(鍏ㄥ眬 + 鐢ㄦ埛绾?:
  - usage-log 灞曠ず(ApplyDisplayTransform):琛屾湁 5m/1h 缁嗗垎涓斾袱妗ｅ睍绀轰环涓嶅悓鏃?鎸夌湡瀹炴。浠锋瘮渚?r=1h/5m,鏉ヨ嚜瀹氫环鏉＄洰鐨?RealCacheWritePrice/RealCacheWrite1hPrice,鏈煡鏃舵寜 1:1)鎷嗗垎瀹為檯钀藉簱鎴愭湰,鍚勬。鐙珛鍙嶇畻灞曠ず token 鈥斺€?鎴愭湰鎬婚鎸夋瀯閫犲畧鎭?鍙厤 5m 妗ｅ睍绀轰环鏃朵繚鎸佹棦鏈?鎬绘垚鏈弽绠?璺緞(鍥炲綊閽?銆?
  - 涓嬫父鏀瑰啓(computeDisplayTokenMultipliers):CacheCreate1hMult 鍒嗘瘝鏀圭敤 1h 灞曠ず浠?鏈厤鍥為€€ 5m 妗ｅ睍绀轰环),涓や晶鍙ｅ緞涓€鑷淬€?
  - 闀夸笂涓嬫枃鍏嬮殕瀵?1h 灞曠ず浠峰悓涔?LongContextInputMultiplier;hasDisplayOverride/BuildUserDisplayPricingMap/merge 鍑芥暟鍏ㄩ摼绾冲叆銆?
- **閰嶇疆鐣岄潰琛ュ叏**:鍏ㄥ眬瀹氫环瀵硅瘽妗?LiteLLM 鍙傝€冨尯 + 璁¤垂鍖?1h 杈撳叆妗嗗甫 litellm placeholder + 灞曠ず鍖?1h 杈撳叆妗?+ applyDisplaySuggested 浠?litellm 1h 鍙栧缓璁?銆佺敤鎴峰畾浠锋ā鎬佹(LiteLLM 鍙傝€冭 + 鐪熷疄/灞曠ず涓や釜 1h 杈撳叆妗?+ 寤鸿鍊?+ $/MTok 鍙屽悜鎹㈢畻)銆佸姣旀娊灞?config_used 灞曠ず 1h 灞曠ず浠?i18n zh/en銆?
- **鍙ｅ緞绛旂枒**(鐢ㄦ埛鎻愰棶,billing.md 浜︽湁璁拌浇):鎵€鏈夋敮鎸佺紦瀛樼殑 Claude 妯″瀷閮芥湁 5m/1h 涓ゆ。,鏄惁鍑虹幇鍙栧喅浜庤皟鐢ㄦ柟璇锋眰鐨?TTL;鏃犲垎妗ｄ环鐨勬ā鍨嬭蛋骞充环鍥為€€(total 脳 CacheCreationPricePerToken);涓婃父鏈繑鍥?5m/1h 缁嗗垎鏃跺叏閮ㄦ寜 5m 浠疯璐?璁¤垂涓庡睍绀轰袱渚т竴鑷?銆?
- Verified: go build/vet 鍏ㄨ繃;鏂板 6 涓崟娴?dto 鍒嗘。鍙嶇畻/1:1 鍏滃簳/鍗曚环鍥炲綊閽?resolver 鐢ㄦ埛绾?1h,display_token 1h 灞曠ず浠峰€嶇巼/鐢ㄦ埛绾?1h 鐪熷疄浠?;鍚庣鍏ㄩ噺 unit 娴嬭瘯銆佸墠绔?typecheck+lint+603 鐢ㄤ緥鍏ㄨ繃銆?

## [2026-07-02] docs: record Hajimi candidate 4K key availability failure

**Affected files**: docs/dev/OPENAI_IMAGE_URL_RELAY_4K_DIAGNOSTICS_2026-06-30.md, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: documentation only; no backend/frontend runtime behavior, route, database, billing, i18n, or migration changes.
**Change details**:
- Recorded the new `hajimicc.top` native 4K candidate key check by key fingerprint only; the full key is stored only in the ignored local test-secret registry under `tmp/image-channel-secrets/`.
- Documented that quality c1 and concurrency c2/c4/c8 all fail before generation with HTTP 503: `No available channel for model gpt-image-2 under group 4K-3锛堝師鐢燂級 (distributor)`.
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
- Recorded the `gpt image 2 楂樿川閲廯 group permission finding, the native 4K quality smoke result, and the `c2/c4/c8` 4K concurrency baseline.
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
- Renamed the keys-page guide heading from "Getting Started" / 寮€濮嬩娇鐢?to "Quick Tutorial" / 蹇€熸暀绋? and added `keys.guide.detailedTutorial` plus `admin.settings.site.tutorialUrl*` i18n keys (zh + en).
- Added `tutorialUrl` to the app store (ref, applySettings parse, fallback cached object, export) and `tutorial_url` to the PublicSettings type and admin settings API types/mapping.
- Verified with `go build ./...`, `go test -tags=unit ./internal/handler/dto -run SchemaDoesNotDrift`, `go test -tags=unit ./internal/server -run TestAPIContracts`, `go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin`, `pnpm --dir frontend run typecheck`, `pnpm --dir frontend run lint:check`, `pnpm --dir frontend exec vitest run src/stores/__tests__/app.spec.ts src/views/admin/__tests__/SettingsView.spec.ts`, and a live `GET /api/v1/settings/public` showing `tutorial_url`.

## [2026-06-20] feat: admin-configurable CCS import model for OpenAI/Codex

**Affected files**: backend/internal/service/domain_constants.go, backend/internal/service/setting_service.go, backend/internal/service/settings_view.go, backend/internal/handler/dto/settings.go, backend/internal/handler/setting_handler.go, backend/internal/handler/admin/setting_handler.go, backend/internal/server/api_contract_test.go, frontend/src/types/index.ts, frontend/src/stores/app.ts, frontend/src/api/admin/settings.ts, frontend/src/views/admin/SettingsView.vue, frontend/src/views/user/KeysView.vue, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: adds a new public Settings-KV key `ccs_import_codex_model` (string, default `gpt-5-codex`) following the existing `api_base_url` / `hide_ccs_import_button` plumbing exactly. Additive 鈥?could conflict if upstream restructures the settings DTO/struct chain or the KeysView CC Switch deeplink builder.
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
- Step 3 stays informational (Claude Code / Gemini CLI tool chips) rather than carrying its own action button: a guide-level "use key" button would be ambiguous about which key it opens when the user has several. Instead, aligned the zh wording so the card points users at the table 鈥?changed step3 title and the "浣跨敤 Key" references in step3Desc/step3DescNoCcs to "浣跨敤瀵嗛挜", matching the per-row table button (`keys.useKey` = 浣跨敤瀵嗛挜). English already used "Use Key", so en is unchanged.
- Verified with `pnpm --dir frontend run typecheck` and `pnpm --dir frontend run lint:check`.

## [2026-06-19] fix: user-facing usage statistics must show display values, not raw

**Affected files**: backend/internal/handler/usage_handler.go, backend/internal/pkg/usagestats/usage_log_types.go, backend/internal/repository/usage_log_repo.go, backend/internal/service/account_usage_service.go, backend/internal/service/usage_service.go, backend/internal/handler/usage_handler_request_type_test.go, backend/internal/handler/usage_handler_display_aggregate_test.go
**Issue**: User-side aggregate stats endpoints summed raw `usage_logs` columns and returned **real** token counts / unit prices, while the per-row usage records list already applied the display-pricing transform (灞曠ず鍗曚环/灞曠ず鍊嶇巼, the "token 鏀惧ぇ鏈哄埗"). So the dashboard/usage stat cards leaked real tokens and did not reconcile with the records list. Design rule: users must only ever see display values; real tokens/prices are internal.
**Change details**:
- `GET /api/v1/usage/stats` (Stats), `/usage/dashboard/trend` (DashboardTrend), `/usage/dashboard/models` (DashboardModels) now aggregate from the same display-transformed records the user sees (`loadAllDisplayedPublicUsageRecords` + `aggregateDisplayedPublicUsageStats` / `aggregateDisplayedPublicUsageTrend` / new `aggregateDisplayedModelStats`) 鈥?exact row-for-row reconciliation with the records list for the selected range.
- `GET /api/v1/usage/dashboard/stats` (DashboardStats) all-time + today token/cost totals now use display values. All-time is unbounded (heaviest user ~247k rows), so it uses per-group SQL aggregation: new repo `GetUserDisplayAggregateGroups` groups by every field the display transform branches on (model, group_id, rate_multiplier, long_context snapshot) and the handler applies the transform once per group and sums (`aggregateDisplayedGroups`). API-key counts, RPM/TPM, and `actual_cost` are unchanged (actual_cost is never altered by the transform).
- New `usagestats.DisplayAggregateGroup` type; new method added to `UsageLogRepository` interface + `UsageService` passthrough.
- `POST /usage/dashboard/api-keys-usage` left as-is 鈥?it only returns `actual_cost` (real money the user pays), which the display transform never changes, so it does not leak tokens/prices.
- New unit test `usage_handler_display_aggregate_test.go` proves per-group aggregation reconciles exactly with per-row summation (and preserves real values when no display override exists).
- Verified: `go -C backend build ./...` (exit 0), `go vet` clean, `go test -tags=unit ./internal/handler/... ./internal/service/... ./internal/pkg/usagestats/...` pass. Pre-existing unrelated failure `TestUsageLogRepositoryGetStatsWithFiltersAlwaysReturnsAccountCost` (stale 8-col sqlmock vs 11-col `GetStatsWithFilters`) also fails on unmodified `main` 鈥?not caused by this change.
**Known follow-ups (not in this change)**:
- `GET /v1/usage` (API-key dashboard, `GatewayHandler.Usage` 鈫?`buildUsageData` + `GetAPIKeyModelStats`) still returns raw tokens, while its siblings `/v1/usage/stats|trend|records` already show display values. Fixing it needs the pricing/display services on `GatewayHandler` (Wire DI) or pushing the display aggregation into the service layer.
- Pricing data finding (config, not code): `global_model_pricing` bills `cache_read` at a flat $2.00/M for `claude-opus-4-8`/`claude-sonnet-4-6`/`gpt-5.4`/`gpt-5.5` while displaying $0.25鈥?.50/M; for cache-heavy users (cache_read 鈮?90% of tokens) this dominates the bill. Confirmed by the operator as intentional config (not a bug) 鈥?left unchanged.

## [2026-06-19] fix: user dashboard cards go stale across midnight + `/v1/usage` raw-token leak

**Affected files**: frontend/src/views/user/DashboardView.vue, backend/internal/handler/gateway_handler.go, backend/internal/handler/usage_handler.go, backend/cmd/server/wire_gen.go, backend/internal/handler/usage_handler_display_aggregate_test.go
**Issue A (stale dashboard)**: A user reported the home dashboard "浠婃棩璇锋眰/浠婃棩娑堣垂/浠婃棩 Token" cards showing the *previous* day's stats while the balance was correct. Root cause: the balance is refreshed by a global 60s timer in the auth store (`stores/auth.ts` `startAutoRefresh`), but the summary cards were fetched only once in `DashboardView.vue` `onMounted` 鈥?no polling, no refetch-on-focus, no day-rollover handling. A tab left open across midnight keeps showing the load-day's "浠婃棩". Backend was verified correct (today query returns the right count; no Redis dashboard cache 鈥?only `sched:*`/`sticky_session:*` keys).
**Issue B (`/v1/usage` leak)**: The audit of user-facing token surfaces found `GET /v1/usage` and `/antigravity/v1/usage` (`GatewayHandler.Usage` 鈫?`buildUsageData` + `GetAPIKeyModelStats`) were the only remaining endpoints returning **raw** token counts, while their siblings `/v1/usage/{stats,trend,records}` already show display values.
**Change details**:
- Frontend: `DashboardView.vue` now silently refetches the summary stats (no full-page spinner) on `visibilitychange`/window `focus` and on a 60s visible-only interval, with listener cleanup in `onBeforeUnmount`. The cards now stay live like the balance and self-correct across midnight within ~60s. The date-range picker still only drives the trend/model widgets (unchanged).
- Backend: `GatewayHandler.Usage` now produces display values. Added `modelPricingService` + `userModelPricingService` to `GatewayHandler` (constructor + `wire_gen.go` hand-edit). `buildUsageData` rewritten to compute today/all-time via per-group display aggregation (`GetUserDisplayAggregateGroups` scoped to the API key); model stats now come from display-transformed records. `actual_cost`, RPM/TPM, avg duration are unchanged.
- Refactor (no behavior change): extracted `loadDisplayedUsageRecords`, `buildDisplayPricingMapForUser`, `loadUserGroupDisplayRates` as free functions and made `aggregateDisplayedGroups` a free function, so both `UsageHandler` (JWT) and `GatewayHandler` (API key) share one display path. `UsageHandler` methods now delegate to them.
- Verified: `go build ./...` (exit 0), `go vet` clean, `go test -tags=unit ./internal/handler/...` pass; frontend `typecheck` + `lint:check` + `build` all pass.

## [2026-06-19] feat(subscription): mixed/bundle subscription 鈥?Phase 1 backend MVP

**Affected files**: backend/migrations/168_subscription_plan_member_groups.sql, backend/ent/schema/{subscription_plan,payment_order}.go (+ regenerated ent), backend/internal/service/{payment_config_plans,payment_config_service,subscription_service,payment_order,payment_fulfillment}.go, backend/internal/handler/payment_handler.go, backend/internal/service/payment_config_plans_member_test.go
**Upstream compatibility**: additive, fork-local. New `member_group_ids JSONB NOT NULL DEFAULT '[]'` columns on `subscription_plans` + `payment_orders`; empty = legacy single-group plan/order 鈫?identical behavior. No change to the gateway/billing/quota/cache hot path (everything stays keyed by `(user_id, group_id)`). Upstream has no mixed-subscription concept; the new columns/fields are additive and safe across upstream syncs.
**Change details**:
- A subscription plan can now bundle multiple subscription-type groups. Effective member set = `unique(group_id 鈭?member_group_ids)`, with `group_id` kept as the primary/representative group (price/sort/display/back-compat).
- One purchase fans out into N independent `user_subscription` rows (one per member group), each with its own quota pool from that group's own `daily/weekly/monthly_limit_usd`. The user switches the API key's group (or uses multiple keys) to access each 鈥?chosen "separate quota pools + multi-group switch" model, so each group stays single-platform and the gateway dispatch is untouched.
- `PlanMemberGroupIDs(plan)` (payment_config_plans.go) computes the effective set; `AssignOrExtendSubscriptionToGroups` (subscription_service.go) reuses the existing per-`(user,group)` `AssignOrExtendSubscription` without a wrapping tx (so partial failures commit and resume).
- Order creation snapshots the member set onto `payment_orders` (`createOrderInTx`); `doSub` (payment_fulfillment.go) fans out with per-group idempotency markers `SUBSCRIPTION_SUCCESS:<gid>` (and `SUBSCRIPTION_MEMBER_SKIPPED:<gid>` for a dead non-primary member), writing the suffix-less `SUBSCRIPTION_SUCCESS` only after every member succeeds. Legacy single-group orders short-circuit exactly as before.
- Admin plan Create/Update DTOs accept `member_group_ids` (normalized: drop 鈮?, dedup, remove primary, must be existing subscription-type groups, cap 10). Public `GetPlans`/`GetCheckoutInfo` expose `member_group_ids` + `member_groups` (per-member platform/name/limits/scopes).
- Refund intentionally untouched (this deployment has refunds disabled); documented limitation: a future bundle refund would only roll back the primary group.
- Verified: `go generate ./ent`, `go build ./...` (exit 0), `go vet` clean, `go test ./internal/service` (untagged) + `go test -tags=unit ./internal/service/...` all pass.
**Pending (Phase 2/3)**: redeem-code/distribution bundle support + admin assign-by-plan; frontend (admin plan editor multi-select, purchase page member-group display, zh/en i18n).

## [2026-06-19] feat(subscription): mixed/bundle subscription 鈥?Phase 3 frontend

**Affected files**: frontend/src/types/payment.ts, frontend/src/views/admin/orders/PlanEditDialog.vue, frontend/src/components/payment/SubscriptionPlanCard.vue, frontend/src/i18n/locales/{zh,en}.ts
**Upstream compatibility**: additive UI on top of the Phase 1 backend. No behavior change for single-group plans (no member groups selected 鈫?renders exactly as before).
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

## [2026-06-14] feat: cost-analysis module 鈥?subscription cost/profit stats

**Affected files**: backend/internal/pkg/usagestats/usage_log_types.go, backend/internal/service/account_usage_service.go, backend/internal/repository/usage_log_repo.go, backend/internal/service/dashboard_service.go, backend/internal/handler/admin/dashboard_handler.go, backend/internal/server/routes/admin.go, frontend/src/api/admin/costAnalysis.ts, frontend/src/views/admin/cost/SubscriptionProfitView.vue, frontend/src/components/layout/AppSidebar.vue, frontend/src/router/index.ts, frontend/src/i18n/locales/{zh,en}.ts
**Purpose**: New admin "Cost Analysis" (鎴愭湰鍒嗘瀽) sidebar module; first page = per-subscription cost/profit for monthly / daily-limited users, so the operator can see real margin per subscription/plan.
**Change details**:
- New endpoint `GET /api/v1/admin/dashboard/subscription-profit?start_date&end_date&purchase_price_per_mtok`.
- Repo `GetSubscriptionProfitRaw` aggregates per `subscription_id`: joins user_subscriptions 鈫?(LATERAL latest paid subscription payment_order 鈫?subscription_plans) 鈫?groups 鈫?users 鈫?usage_logs. INNER JOIN on the paid order excludes redeem-code / admin-granted subscriptions. Filters subscriptions by `starts_at` range; `deleted_at IS NULL`.
- Cost basis: real_cost_rmb = total tokens 脳 purchase price (RMB / million tokens), default 0.25 (= 楼10 / 40M tokens), passed as a query param driven by a UI input persisted in localStorage (no settings/Wire change in v1). Revenue = plan list price. Consumed "$" = SUM(actual_cost). Derived: avg 楼/$, real cost 楼/$, profit multiple, equivalent full-days (consumed$ 梅 daily_limit_usd), cache rate; plus summary + by-plan rollups (loss / <2x counts).
- Frontend: new collapsible nav group 鎴愭湰鍒嗘瀽 (expandOnly) in AppSidebar; routes `/admin/cost-analysis` 鈫?redirect 鈫?`/admin/cost-analysis/subscriptions`; SubscriptionProfitView (control bar + summary cards + by-plan + detail table, multiple color-coded). Added to simple-mode restrictedPaths. New i18n keys nav.costAnalysis / nav.costSubscriptionProfit and costAnalysis.* in zh + en.
- Verified: `CGO_ENABLED=0 go -C backend build ./...` (exit 0); `pnpm --dir frontend run typecheck` + `lint:check` (both exit 0). Not yet runtime-tested against live data; no DB migration (uses existing columns).

## [2026-06-14] fix: wrap SubscriptionProfitView in AppLayout (sidebar)

**Affected files**: frontend/src/views/admin/cost/SubscriptionProfitView.vue
**Issue**: The cost-analysis page rendered bare content so the left sidebar vanished 鈥?admin views must wrap their template in `<AppLayout>` (which renders AppSidebar + AppHeader). Wrapped the page in `<AppLayout>` and imported it. Verified: `typecheck` + `lint:check` exit 0.

## [2026-06-14] feat: cost-analysis subscription view 鈥?active-by-default + per-dollar cost mode

**Affected files**: backend/internal/pkg/usagestats/usage_log_types.go, backend/internal/service/{account_usage_service,dashboard_service}.go, backend/internal/repository/usage_log_repo.go, backend/internal/handler/admin/dashboard_handler.go, frontend/src/api/admin/costAnalysis.ts, frontend/src/views/admin/cost/SubscriptionProfitView.vue, frontend/src/i18n/locales/{zh,en}.ts
**Change details**:
- Default now shows **currently-active subscriptions** with no date picking required: `active_only` query param defaults true 鈫?repo filters `status='active' AND starts_at <= now() AND expires_at > now()`. Date range is optional (active_only=false 鈫?filter by starts_at, history mode).
- Added **cost basis mode**: `cost_mode=per_mtok` (real cost = total tokens 脳 楼/M, default 0.25) or `per_dollar` (real cost = consumed $ 脳 楼/$). Endpoint params renamed: `purchase_price` + `cost_mode` (was `purchase_price_per_mtok`). Summary echoes cost_mode + purchase_price. The per_dollar path is the simple form (consumed_usd 脳 rate); finer 楼/$ valuation nuances deferred per user.
- Frontend: "浠呭綋鍓嶆湁鏁堣闃? checkbox (default on, hides date inputs), cost-basis selector with dynamic unit label, localStorage persists price + mode. New i18n keys activeOnly/activeHint/costMode/unitPerMtok/unitPerDollar (zh+en).
- Verified: `go -C backend build ./...`, `pnpm --dir frontend typecheck` + `lint:check` all exit 0.

## [2026-06-13] feat: manual OAuth refresh-token update for accounts

**Affected files**: backend/internal/handler/admin/account_handler.go, backend/internal/server/routes/admin.go, backend/internal/handler/admin/account_handler_refresh_token_test.go, frontend/src/api/admin/accounts.ts, frontend/src/components/admin/account/UpdateRefreshTokenModal.vue, frontend/src/components/admin/account/AccountActionMenu.vue, frontend/src/views/admin/AccountsView.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts, docs/dev/CHANGELOG_CUSTOM.md
**Upstream compatibility**: additive local admin feature; no schema/migration and no billing/gateway routing changes. Reuses the existing per-platform OAuth refresh path and the existing `accounts.credentials` JSONB column.
**Change details**:
- Added `POST /api/v1/admin/accounts/:id/refresh-token` (`AccountHandler.UpdateRefreshToken`) so an admin can paste a new OAuth refresh token when the stored one has expired/revoked 鈥?distinct from the existing auto `/:id/refresh` (which reuses the stored token) and from full Re-authorize.
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
- The go.mod bump to 1.26.4 made all four hardcoded `go version | grep -q 'go1.26.2'` verify steps fail (CI, golangci-lint, security scan, release), which blocked the v0.1.139 GHCR image publish. Bumped all four to go1.26.4 鈥?same root cause as the Dockerfile builder image fix.

---

## [2026-06-12] fix(ui): legal consent dialog auto-passes scroll gate when terms do not overflow

**Affected files**: frontend/src/components/auth/LegalConsentDialog.vue, frontend/src/components/auth/__tests__/LegalConsentDialog.spec.ts
**Upstream compatibility**: fork-only feature (legal consent), no upstream overlap.
**Change details**:
- P2 from pre-deploy review: `scrolledToBottom` was only ever set by a scroll event, which never fires when the rendered terms fit inside the dialog (short admin-configured content, tall screens). The accept button then stays permanently disabled 鈥?bricking login/registration for all users.
- On dialog open, after render, the gate now auto-passes when `scrollHeight <= clientHeight + 4`. Spec updated to mock overflow dimensions before the gate check; added a no-overflow auto-pass case.

---

## [2026-06-12] fix(billing): per-turn billing request id for multi-turn OpenAI WebSocket connections

**Affected files**: backend/internal/handler/openai_gateway_handler.go, backend/internal/handler/turn_usage_record_context_test.go
**Upstream compatibility**: fork-side fix for a regression introduced by the phase-6b upstream sync (87f2a29c); watch for upstream's own fix when syncing later.
**Change details**:
- P0 found in pre-deploy review: phase 6b made async usage-record tasks inherit the request context, so every turn of an OpenAI WS connection resolved the same billing request id (`client:<connection-uuid>`). Turns 2..N then collided on the `usage_billing_dedup`/`usage_logs (request_id, api_key_id)` keys 鈥?tokens were neither billed nor logged (silent revenue loss for Codex WS-mode multi-turn traffic).
- Added `turnUsageRecordContext` which suffixes both `ctxkey.ClientRequestID` and `ctxkey.RequestID` with the per-turn upstream response id (falling back to the turn number) inside the WS `AfterTurn` hook. This covers the forwarder, HTTP-bridge, and passthrough adapter paths, which all share that hook. Unit tests added.

---

## [2026-06-12] fix(billing): normalize usage-log image size to billing tier (migration 156 compatibility)

**Affected files**: backend/internal/service/image_billing_size.go (new, ported from upstream), backend/internal/service/image_billing_size_test.go (new), backend/internal/service/openai_gateway_service.go, backend/internal/service/gateway_service.go
**Upstream compatibility**: partial port of upstream's image billing size classifier; the forward-result audit fields (image_input_size/image_output_size/image_size_source/image_size_breakdown) are still unsynced 鈥?finish that on a later sync, then move normalization back to the parse points like upstream.
**Change details**:
- P1 found in pre-deploy review: migration 156 adds CHECK `usage_logs_image_billing_size_check` (image_count > 0 requires image_size IN 1K/2K/4K/mixed), but the fork's OpenAI image paths still write raw request sizes ("1024x1024", "auto", "") 鈥?after deploy every OpenAI image-generation usage-log INSERT would violate the constraint: user charged, row silently dropped.
- Ported upstream's pure classifier functions (ClassifyImageBillingTier / NormalizeImageBillingTierOrDefault / ResolveImageBillingSize) and normalized image_size at both usage-log write points (`normalizedImageBillingSizePtr`), covering images/responses/WS-bridge and the Anthropic-side path. Upstream's classifier tests ported as-is.

---

## [2026-06-12] fix(pricing): add claude-fable-5 to checked-in fallback pricing

**Affected files**: backend/resources/model-pricing/model_prices_and_context_window.json
**Upstream compatibility**: additive entry copied verbatim from the live remote pricing cache (backend/data/model_pricing.json); upstream may add it later 鈥?dedupe on sync.
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
- Added missing `//go:build unit` tag to `announcement_service_test.go` 鈥?it references `userRepoStub` defined in unit-tagged `admin_service_delete_test.go`, so untagged builds (`go vet ./...`, plain `go test ./...`) failed to compile the service package.
- Added `long_context_applied: false` to the `GET /api/v1/usage` expected payload in the API contract test 鈥?the field was intentionally added to the usage DTO by the long-context pricing snapshot work (a5bba54f) but the contract expectation was not updated.

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
- The synced content header is `# ZeroCode API 浣跨敤鏂囨。` with `鏈€鍚庢洿鏂帮細2026-05-25`.

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
- Renamed the user-side distribution entry and page title to "Become an Agent" / "鎴愪负浠ｇ悊" while leaving admin distribution management unchanged.
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
- Reworked chapter 4 into separate `CLI 鐗堟湰锛氬畨瑁呬笌閰嶇疆` and `Desktop 妗岄潰鐗堬細瀹夎涓庨厤缃甡 sections, matching chapter 3's version-based tutorial structure.
- Moved Codex CLI installation, CC-Switch setup, manual configuration, WebSocket option, and verification into one CLI flow.
- Added a full Codex Desktop flow for install, CC-Switch configuration, local project startup, and Desktop-specific troubleshooting.

## [2026-05-25] docs: make API Keys CCS import the primary setup path

**Affected files**: docs/API_USAGE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Updated Claude Code CLI, Codex CLI, and Codex Desktop setup flows to use the API Keys page `瀵煎叆鍒?CCS` action as the primary configuration method.
- Clarified that the API Keys import action maps Anthropic groups to Claude Code, OpenAI groups to Codex, and Gemini groups to Gemini CLI.
- Reframed manual file copying and the `浣跨敤` modal as fallback paths; Claude Code Desktop remains the manual application-level setup path.

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

## [2026-05-14] fix(frontend): 琛ラ綈鍒嗛攢绠＄悊涓枃鏂囨

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

## [2026-05-14] feat: 閻劍鍩涙笟褍娴橀悧鍥﹀▏閻劏顔囪ぐ鏇炵潔缁€鍝勬槀鐎甸晲绗岀拹銊╁櫤

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

## [2026-05-14] fix: 缁愪礁鍤崶鍓у鐠愩劑鍣洪崡鏇氱幆闁板秶鐤嗛崗銉ュ經

**Affected files**: frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue, frontend/src/i18n/locales/zh.ts, frontend/src/i18n/locales/en.ts
**Upstream compatibility**: low risk, admin model pricing UI only
**Change details**:
- Made the `low` / `medium` / `high` / `auto` image quality price fields a labeled subsection under megapixel image billing.
- Clarified that empty quality prices fall back to the default megapixel price.
- Verified with `pnpm run typecheck`.

## [2026-05-14] feat: 閸ュ墽澧栧锝勭秴鐠伮ゅ瀭閺€顖涘瘮 quality 娑旀ɑ鏆?

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

## [2026-05-12] feat(aiclient2api): Kiro 閸欏秳鍞紓鎾崇摠娴兼壆鐣绘稉?conversationId 缁嬪啿鐣鹃崠?

**瑜板崬鎼烽懠鍐ㄦ纯**: `aiclient2api/src/providers/claude/claud*: 閺冪姴鍟跨粣渚婄礄aiclient2api 閺勵垳瀚粩?fork閿?
**閸欐ɑ娲跨拠锔藉剰**:
- 閺傛澘顤?`deriveStableConversationId(metadata)`: 娴?Claude Code 閻?`metadata.user_id` 娑擃厽褰侀崣?session_id閿涘ash 娑撹櫣鈥樼€规碍鈧?UUID閿涘奔濞囬崥灞肩娴兼俺鐦介惃鍕閺?turn 閸忓彉闊?conversationId閿涘苯鎯庨悽?Amazon Q 閺堝秴濮熺粩顖欑瑐娑撳鏋冪紓鎾崇摠
- 閺傛澘顤?`filterBillingHeaderFromSystem()`: 鏉╁洦鎶?system prompt 娑擃厽鐦℃潪顕€鍏橀崣妯兼畱 `x-anthropic-billing-header`閿涘潏ch= 鐎涙顔岄敍澶涚礉娣囨繃瀵?prompt 缁嬪啿鐣?
- 閺傛澘顤?`_estimateCacheMetrics(requestBody)` + `_countMessageTokens(msg)`: 娴犲氦顕Ч鍌欑秼娴兼壆鐣荤紓鎾崇摠 token 閳?妫ｆ牞鐤嗛幎?cache_creation閿涘苯鎮楃紒顓＄枂閹?system + tools + 閸樺棗褰堕崜宥囩磻閹躲儰璐?cache_read閿涘nput_tokens 閸欘亣顓搁張鈧崥搴濈閺夆剝鏌婂☉鍫熶紖
- `_countMessageTokens` 濮濓絿鈥樻径鍕倞閹碘偓閺?content block 缁鐎烽敍鍧眅xt/thinking/tool_use/tool_result閿涘绱濈紓鎾崇摠閻滃洣绮?~45% 閹绘劕宕岄懛?~83%
- 濞翠礁绱￠崫宥呯安閻?message_start 閸?message_delta 娴滃娆㈡担璺ㄦ暏娴兼壆鐣婚崐鍏兼禌娴狅絿鈥栫紓鏍垳 0

## [2026-05-12] feat: antigravity 閸掑棛绮嶉幒銉ュ弳 Kiro 閸欏秳鍞敍鍫熸煙濡?B閿?

**瑜板崬鎼烽懠鍐ㄦ纯**: `backend/internal/service/account.go`, `backend/internal/service/gateway_service.go`, `backend/internal/pkg/antigravity/claude_types.go`, `backend/internal/service/account_anthropic_passthrough_test.go`, `frontend/vite.config.ts`, `docs/dev/KIRO_PROXY.md`
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娑擃厾鐡戦妴淇檃ccount.go` 閻?`IsAnthropicAPIKeyPassthroughEnabled` 閸?`GetBaseURL` 閺€閫涚啊閺夆€叉闁槒绶敍娌梘ateway_service.go` 閻ㄥ嫭膩閸ㄥ鏁幐浣诡梾閺屻儱濮炴禍?passthrough bypass閿涙稐绗傚〒姝屽闁插秵鐎潻娆庣昂閸戣姤鏆熼棁鈧幍瀣З閸氬牆鑻熼妴?
**閸欐ɑ娲跨拠锔藉剰**:
- 閺€鎯х磾閺傝顢?A閿涘牐鐭鹃悽鍗炵湴閸ョ偤鈧偓閿涘绱濋柌鍥╂暏閺傝顢?B閿涙iro 鐠愶箑褰块柊宥囩枂娑?`platform=antigravity` + `type=apikey` + `passthrough=true`閿涘瞼娲块幒銉ュ棘娑?antigravity 閸掑棛绮?load-aware 鐠嬪啫瀹?
- `IsAnthropicAPIKeyPassthroughEnabled()`: 閺€鎯ь啍楠炲啿褰撮梽鎰煑閿涘奔绮犻崣顏呭复閸?anthropic 閺€閫涜礋閸氬本妞傞幒銉ュ綀 antigravity
- `GetBaseURL()`: antigravity passthrough 鐠愶箑褰挎稉宥呭晙閼奉亜濮╅幏鍏煎复 `/antigravity` 閸氬海绱戦敍鍫滅矌 Google Cloud Code 閸樼喓鏁?apikey 鐠愶箑褰块棁鈧憰渚婄礆
- `isModelSupportedByAccountWithContext()` / `isModelSupportedByAccount()`: antigravity passthrough 鐠愶箑褰跨捄瀹犵箖濡€崇€烽弰鐘茬殸濡偓閺屻儻绱濋幒銉ュ綀閹碘偓閺堝膩閸?
- `DefaultModels()`: 娑?Claude 濡€崇€烽悽鐔稿灇 `[1m]`/`[2m]` 娑撳﹣绗呴弬鍥╃崶閸欙絽鎮楃紓鈧崣妯圭秼閿涘矁袙閸?Claude Code 鐎广垺鍩涚粩顖浤侀崹瀣墡妤犲奔绗夐柅姘崇箖閻ㄥ嫰妫舵０?
- `vite.config.ts`: 閺傛澘顤?`/antigravity` 娴狅絿鎮婄捄顖氱窞閿涘本婀伴崷鏉跨磻閸欐垶妞傞崜宥囶伂 dev server 濮濓絿鈥樻潪顒€褰傞崚鏉挎倵缁?
- 閺囧瓨鏌?`docs/dev/KIRO_PROXY.md` 閺傚洦銆傞敍宀冾唶瑜版洖鐣弫瀛樻煙濡楀牄鈧線鍘ょ純顔筋劄妤犮倕鎷伴幒鎺撶叀鏉╁洨鈻兼稉顓炲絺閻滄壆娈?4 娑擃亜娼?

## [2026-05-12] feat(deploy): AIClient2API 濮濓絽绱℃稉濠勫殠閻㈢喍楠?+ Web UI 閸忣剛缍夐崣顖濐問闂?

**瑜板崬鎼烽懠鍐ㄦ纯**: 閻㈢喍楠?`/opt/sub2api/.env`閵嗕梗/opt/sub2api/docker-compose.yml`閵嗕梗/etc/caddy/Caddyfile`閵嗕竼loudflare DNS (`a2.zerocode.kaynlab.com`)閿涘畭deploy/docker-compose.yml`閵嗕梗docs/dev/KIRO_PROXY.md`
**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礄娴犲懐鏁撴禍褔鍎寸純鏌ュ帳缂?+ 閺堫兛绮ㄦ惔?compose/閺傚洦銆傞敍?
**閸欐ɑ娲跨拠锔藉剰**:
- 鐎瑰本鍨?AIClient2API 閻㈢喍楠囬柈銊ц閿涙ork `541968679/AIClient2API` 閳?閸︺劎鏁撴禍褎婀囬崝鈥虫珤 `git clone + docker build` 閳?闁俺绻?`update.sh --only-a2` 闁劎璁?
- 閻㈢喍楠?`.env` 鐞涖儱鍘?`SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true` 閸?`SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true`閿涘苯鍘戠拋?sub2api 闁俺绻?`http://aiclient2api:3000` 鐠嬪啰鏁ら崘鍛秹 sidecar閿涘牊婀伴崷?dev 閺堫亜鎯庨悽?allowlist 閹碘偓娴犮儲鐥呴柆鍥у煂閿?
- 娣囶喖顦?aiclient2api healthcheck閿涙瓪localhost` 閸︺劌顔愰崳銊ュ敶娴兼ê鍘涚憴锝嗙€介崚?IPv6 `::1`閿涘奔绲鹃張宥呭閸欘亞娲冮崥?IPv4 `0.0.0.0:3000`閿涘本鏁兼稉?`127.0.0.1:3000`
- 閸忣剛缍?Web UI閿涙碍鏌婃晶?Cloudflare DNS A 鐠佹澘缍?`a2.zerocode.kaynlab.com 閳?172.245.247.80`閿涘湒NS Only閿涘绱濋弬鏉款杻 Caddy vhost 閸欏秳鍞崚鏉款問娑撶粯婧€ `127.0.0.1:3000`
- compose 缂?aiclient2api 缂佹垵鐣鹃崚鏉款問娑撶粯婧€ `127.0.0.1:3000`閿涘牅绗夌€电懓鍙曠純鎴炴瘹闂囪绱濇禒鍛返 Caddy 閺堫剚婧€閸欏秳鍞敍澶涚礉Docker 閸愬懐缍?DNS 閸氬本妞傛禒宥呭讲閻?
- 閸欙絼鎶ら妴涔別b UI 鐠佸潡妫堕崷鏉挎絻閵嗕竼addyfile 缁€杞扮伐閵嗕浇鐤嗛幑銏＄ウ缁嬪鍑￠崗銊╁劥鐠佹澘缍嶉崷?`docs/dev/KIRO_PROXY.md`
- **瑜版挸澧犻崣顖滄暏闁炬崘鐭?*閿涙瓫nthropic 閸掑棛绮?API Key 閳?sub2api 缂冩垵鍙?閳?AIClient2API (`http://aiclient2api:3000/claude-kiro-oauth`) 閳?Kiro API 閳?Claude 缁鍨Ο鈥崇€?

## [2026-05-11] feat: Kiro 閸欏秳鍞€佃甯撮敍鍧卬thropic 閸掑棛绮嶅鏌モ偓姘剧礉antigravity 閸掑棛绮嶉柆妤冩殌閿?

**瑜板崬鎼烽懠鍐ㄦ纯**: `backend/internal/service/gateway_service.go`, `backend/internal/service/account.go`, `frontend/src/components/account/CreateAccountModal.vue`, `frontend/src/components/account/EditAccountModal.vue`, `AIClient2API` 鐎涙劙銆嶉惄? `docs/dev/KIRO_PROXY.md`
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娑擃厾鐡戦崘鑼崐閿涘疅ateway_service.go 閸斻劋绨?passthrough 閸掑棙鏁崪?selectAccount 濞翠胶鈻?
**閸欐ɑ娲跨拠锔藉剰**:
- 闁俺绻?AIClient2API 鐎涙劙銆嶉惄顔肩殺 Kiro 鐠愶箑褰块崣宥勫敩娑?Anthropic Messages API閿涘苯鍟€娴?anthropic 楠炲啿褰?API Key 閺傜懓绱￠幒銉ュ弳 sub2api閿涘牆鍑＄捄鎴︹偓姘剧礉闁俺绻?`/v1/messages` 缁旑垳鍋ｉ崣顖涱劀鐢晲濞囬悽?Kiro 閻?Claude 濡€崇€烽敍?
- `gateway_service.go`: passthrough 鏉烆剙褰傞崜宥嗙閻炲棙膩閸ㄥ鎮曟稉顓犳畱 `[1m]`/`[2m]` 缁涘绗傛稉瀣瀮缁愭褰涢崥搴ｇ磻閿涘湑laude Code 鐎广垺鍩涚粩顖欑窗鐢附顒濋崥搴ｇ磻閿涘瓲iro 娑撳秷鐦戦崚顐礆
- `gateway_service.go`: antigravity 閸掑棛绮嶉柅澶夌瑝閸掓媽澶勯崣閿嬫閸ョ偤鈧偓閸?anthropic passthrough 鐠愶箑褰块敍鍫熸煙濡?A閿涙俺鐭鹃悽鍗炵湴閸ョ偤鈧偓閿涘奔绗夐弨纭呭閸欓攱膩閸ㄥ绱?
- 閸撳秶顏?`CreateAccountModal` / `EditAccountModal`: 閹碘晛鐫?`anthropic_passthrough` 瀵偓閸忚櫕妯夌粈鍝勫煂 antigravity 楠炲啿褰?apikey 鐠愶箑褰?
- AIClient2API 娓氀傛叏閺€?`claude-kiro.js` 閻ㄥ嫯闊╂禒鑺ユ暈閸忋儻绱濋幎濠佺稊閼板懐娈?娴ｆ洖顦?077"閺€閫涜礋閸斻劍鈧?`${model}` 閸欐﹢鍣洪敍宀冾唨濡€崇€烽懛顏喰炴稉搴ゎ嚞濮瑰倷绔撮懛瀵告畱閸氬秴鐡ч敍鍫濐洤 `claude-opus-4-7`閿?
- **闁鏆€闂傤噣顣?*閿涘牐顕涚憴?`docs/dev/KIRO_PROXY.md`閿涘绱?
  1. antigravity 閸掑棛绮嶇€圭偞绁存禒宥嗗Г `claude-opus-4-7[1m]` 濡€崇€烽柨娆掝嚖閿涘瞼鏋掓导鑲╃椽鐠囨垶婀悽鐔告櫏閹存牞铔嬫禍鍡楀従娴犳牞鐭惧?
  2. antigravity 閸掑棛绮嶉惃?key 閺冪姵纭堕崷?sub2 楠炲啿褰撮懢宄板絿妫版繂瀹虫穱鈩冧紖
  3. API 鐠嬪啰鏁ら柅鐔峰閸嬪繑鍙冮敍灞炬弓閸嬫氨缍夌紒婊堟懠鐠侯垰鍨庨弸?
- 鐎瑰本鏆ｇ€佃甯撮弬瑙勵攳閵嗕礁鍑￠惌銉ユ綑閵嗕線浠愰悾娆撴６妫版ɑ甯撻弻銉︽煙閸氭垵娼庣拋鏉跨秿閸?`docs/dev/KIRO_PROXY.md`

## [2026-05-10] infra: 瀵洖鍙?AIClient2API 娴ｆ粈璐?Kiro 閸欏秳鍞€涙劙銆嶉惄?

**瑜板崬鎼烽懠鍐ㄦ纯**: 妞ゅ湱娲版径鏍劥娓氭繆绂嗛敍鍧凟:\cursor project\AIClient2API`閿涘鈧梗docs/dev/KIRO_PROXY.md`
**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礉娑撳秳鎱ㄩ弨?sub2api 娴狅絿鐖?
**閸欐ɑ娲跨拠锔藉剰**:
- 瀵洖鍙?[AIClient2API](https://github.com/justlovemaki/AIClient2API)閿?600+ stars閿涘缍旀稉?Kiro 閸欏秴鎮滄禒锝囨倞鐎涙劙銆嶉惄?
- sub2api 閺堫剝闊╂稉宥嗘暜閹?Kiro 楠炲啿褰撮敍宀勨偓姘崇箖 AIClient2API 鐏?Kiro 鐠愶箑褰块崣宥勫敩娑?Anthropic Messages API閿涘苯鍟€娴?API Key 閺傜懓绱￠幒銉ュ弳 sub2api
- 鐎佃甯寸捄顖氱窞閿涙ub2api Anthropic API Key 鐠愶箑褰?閳?`base_url` 閹稿洤鎮?`http://{A2閸︽澘娼儅:3000/claude-kiro-oauth` 閳?AIClient2API 鏉烆剙褰傞懛?Kiro 娑撳﹥鐖?
- 閺傛澘顤?`docs/dev/KIRO_PROXY.md` 閺傚洦銆傜拋鏉跨秿鐎瑰本鏆ｇ€佃甯撮弬瑙勵攳

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

## [2026-05-07] fix(frontend): 鐠併垽妲勬總妤咁樀娴犻攱鐗哥粭锕€褰?$ 閳?妤?

**瑜板崬鎼烽懠鍐ㄦ纯**: `frontend/src/components/payment/SubscriptionPlanCard.vue`, `frontend/src/views/admin/orders/AdminPaymentPlansView.vue`
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ骸鍟跨粣渚婄礉娴犲懏绉归崣濠傚缁旑垱膩閺夋寧鏋冮張?
**閸欐ɑ娲跨拠锔藉剰**:
- 娣囶喖顦茬拋銏ゆ婵傛顦甸崡锛勫娴犻攱鐗搁崪灞藉灊缁惧灝甯禒閿嬫▔缁€?`$` 閼板矂娼?`妤糮 閻ㄥ嫰妫舵０姗堢礄婵傛顦垫禒閿嬬壐閺勵垯姹夊鎴濈閿?
- 娣囶喖顦茬粻锛勬倞閸氬骸褰存總妤咁樀閸掓銆冩い鍏哥幆閺嶇厧鍨崥灞剧壉閻?`$` 閳?`妤糮 闁挎瑨顕?
- 濞夈劍鍓伴崠鍝勫瀻閿涙艾顨滄鎰幆閺嶇》绱檖rice/original_price閿涘璐?CNY 閻?`妤糮閿涙稓鏁ら柌蹇涙妫版繐绱檇aily_limit_usd 缁涘绱氭稉?USD 閻?`$`

## [2026-05-07] fix: avoid permanent error on setup-token 401

**Affected files**: backend/internal/service/ratelimit_service.go, backend/internal/service/ratelimit_service_401_test.go, docs/dev/codebase/account.md
**Upstream compatibility**: low risk, OAuth error-policy bug fix
**Change details**:
- Changed 401 handling to treat `setup-token` accounts as OAuth-like accounts via `account.IsOAuth()`, matching gateway credential routing.
- A first 401 for setup-token accounts now invalidates token state and marks the account temporarily unschedulable instead of immediately setting `status=error`.
- Added unit coverage for Anthropic setup-token `Invalid bearer token` responses.

## [2026-05-07] docs: 娴兼ê瀵?Codex 閹恒儱鍙嗛弫娆戔柤

**Affected files**: docs/API_USAGE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Renamed chapter 4 from "OpenAI Codex CLI 閹恒儱鍙嗛幐鍥у础" to "Codex 閹恒儱鍙嗛幐鍥у础".
- Clarified that Codex CLI and Codex desktop share the same `.codex/config.toml` and `.codex/auth.json` files, so CC-Switch can manage both with one configuration.
- Removed the WSL2-based Windows installation path and simplified Windows setup to native Node.js/npm installation.

## [2026-05-07] docs: 鐠嬪啯鏆ｉ弫娆戔柤楠炲啿褰存い鍝勭碍楠炲墎些闂?Linux 鐎瑰顥婇柊宥囩枂

**Affected files**: docs/API_USAGE.md
**Upstream compatibility**: docs-only; no runtime behavior changes
**Change details**:
- Reordered tutorial installation and configuration platform instructions to Windows first, then macOS.
- Removed Linux-specific installation/configuration paths and commands from Claude Code and Codex setup sections.
- Updated screenshot notes and platform selectors to reference only Windows and macOS.

<!--
缁€杞扮伐閺夛紕娲伴敍?

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

## [2026-05-04] fix(frontend): 閸忓懎鈧壈顓归梼鍛淬€夐棃?UI 娴兼ê瀵?

**瑜板崬鎼烽懠鍐ㄦ纯**: `frontend/src/views/user/PaymentView.vue`, `frontend/src/components/payment/SubscriptionPlanCard.vue`
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ骸鍟跨粣渚婄礉娴犲懏绉归崣濠傚缁旑垱膩閺夊灝鎷伴弽宄扮础
**閸欐ɑ娲跨拠锔藉剰**:
- 娣囶喖顦查崣鍏呮櫠鐠併垽妲勯弽蹇旂垼妫?i18n key 闁挎瑨顕ら敍鍧刾ayment.tabSubscription` 閳?`payment.tabSubscribe`閿涘绱濇稊瀣閺勫墽銇氶崢鐔奉潗 key 閼板矂娼稉顓熸瀮缂堟槒鐦?
- 婢舵艾顨滄鎰娴犲孩铆閸氭垹缍夐弽鍏煎笓閸掓鏁兼稉铏规棻閸氭垵鍨悰銊﹀笓閸掓绱濈涵顔荤箽閸忔娊鏁穱鈩冧紖娑撳秷顫﹂幋顏呮焽
- 缁夊娅庢總妤咁樀閸楋紕澧栭崪宀冾吂闂冨懐鈥樼拋銈呭隘閸╃喓娈戦獮鍐插酱閺嶅洩鐦?badge閿涘湦penAI閵嗕竸ntigravity 缁涘绱?

## [2026-05-04] docs: 閺傛澘顤?API 娴ｈ法鏁ら弬鍥ㄣ€傞敍鍫濐吂閹村嘲鎮滈敍?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `docs/API_USAGE.md`閿涘牊鏌婃晶鐑囩礆

**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礄缁绢垱鏌婃晶鐐存瀮娴犺绱?
**閸欐ɑ娲跨拠锔藉剰**:
- 閺傛澘顤冮棃銏犳倻鐎广垺鍩涢惃?API 娴ｈ法鏁ら弬鍥ㄣ€傞敍宀冾洬閻?Claude Code閿涘湑LI / Desktop / VS Code / JetBrains閿涘鎷?OpenAI Codex CLI 閻ㄥ嫬鐣ㄧ憗鍛村帳缂冾喖鍙忓ù浣衡柤
- 閸栧懎鎯堥獮鍐插酱濞夈劌鍞介崗鍛偓鍏肩ウ缁嬪鈧焦膩閸ㄥ鍨悰銊ｂ偓涓凱I 缁旑垳鍋ｉ崣鍌濃偓鍐︹偓浣筋吀鐠愮顕╅弰搴涒偓涓桝Q
- 妫板嫮鏆€閹搭亜娴橀崡鐘辩秴缁楋讣绱欓崥顐ｇ垼濞夈劏顕╅弰搴礆閿涘苯绶熼崥搴ｇ敾鐞涖儱鍘栫€圭偤妾幋顏勬禈

---

## [2026-05-02] progress: v0.1.117 閸氬牆鑻熸宀冪槈娑撳簼鑵戦弬?i18n 鐞涖儵缍?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/i18n/index.ts`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/i18n/locales/zh.ts`
- `docs/dev/CHANGELOG_CUSTOM.md`
- `docs/dev/UPSTREAM_SYNC.md`

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- Low. 瑜版挸澧犻弨鐟板З闂嗗棔鑵戦崷銊ュ缁?i18n 姒涙顓荤拠顓♀枅閵嗕焦褰冮崐鍏肩壐瀵繐鎷版稉顓熸瀮閺傚洦顢嶇悰銉╃秷閿涘奔绗夐弨鐟板綁閸氬海顏稉姘闁槒绶妴?
- 閸氬海鐢绘俊鍌涚亯娑撳﹥鐖剁紒褏鐢婚弬鏉款杻 i18n key閿涘矂娓剁憰浣烘埛缂侇厺绻氶幐?`en.ts` / `zh.ts` key 鐟曞棛娲婃稉鈧懛娣偓?

**瑜版挸澧犳潻娑樺**:
- 瀹告彃婀悪顒傜彌 worktree `E:\cursor project\api2sub-v117`閵嗕礁鍨庨弨?`sync/upstream-v0.1.117` 閸氬牆鑻熸稉濠冪埗 `v0.1.117`閵?
- 瀹告彃鐣幋鎰拱閸︾増褰佹禍銈忕窗
  - `37519fcb` merge v0.1.117
  - `511e419b` fix(frontend): default locale and interpolation for v117
  - `64b5dff2` fix(frontend): add zh login locale keys
  - `243eae93` fix(frontend): add missing zh dashboard labels
  - `9ca7e522` fix(frontend): complete v117 zh locale coverage
- 瀹歌尙鈥樼拋銈勭瑐濞?tag `v0.1.117` 閸?`backend/cmd/server/VERSION` 娴犲秳璐?`0.1.116`閿涘苯娲滃銈夈€夐棃銏犱箯娑撳﹨顫楅弰鍓с仛 `v0.1.116` 閺勵垯绗傚〒鍝ュ閺堫剚鏋冩禒鑸电哺閸氬函绱濇稉宥勫敩鐞涖劏绻嶇悰宀勬晩閸掑棙鏁妴?
- 閺堫剙婀存宀冪槈閺堝秴濮熼敍?
  - 閸撳秶顏敍姝歨ttp://localhost:5180`
  - 閸氬海顏敍姝歨ttp://localhost:18082`
  - 閸氬海顏棁鈧憰浣蜂簰 `RUN_MODE=standard` 鏉╂劘顢戦敍灞芥儊閸掓瑧顓搁悶鍡楁喅娓氀勭埉娴兼岸娈ｉ挊蹇旂闁挾顓搁悶鍡欑搼閼挎粌宕熼妴?

**閸欐ɑ娲跨拠锔藉剰**:
- 姒涙顓荤拠顓♀枅閺€閫涜礋娑擃厽鏋冮敍灞借嫙娣囶喖顦?vue-i18n 閹绘帒鈧吋鐗稿蹇ョ礉鐏?`${amount}` 鏉╂瑧琚崘娆愮《閺€閫涜礋 `{amount}`閵?
- 鐞涖儵缍堥惂璇茬秿妞ゅ吀鑵戦弬?key閿涘矂浼╅崗宥夘浕濞嗏剝澧﹀鈧惂璇茬秿妞ゅ灚妯夌粈?`auth.login.*`閵?
- 鐞涖儵缍堟禒顏囥€冮惄妯烘彥閹瑰嘲鍙嗛崣锝勮厬閺?key閵?
- 鐞涖儵缍?v117 閺傛澘顤?娴滃苯绱戞い鐢告桨娑擃厽鏋?key閿涘矁顩惄鏍€夐棃銏犲敶鐎瑰箍鈧胶娅ヨぐ鏇€夐柊宥囩枂閵嗕礁鐣炬禒鐑姐€夐柊宥囩枂閵嗕焦膩閸ㄥ鍘ょ純顔衡偓浣鼓侀崹瀣暰娴犳灚鈧竸PI Key 娴ｈ法鏁ゅ鏇烆嚤閵嗕浇澶勯崣?閻劍鍩?娴狅絿鎮?娴ｈ法鏁ょ拋鏉跨秿閵嗕礁鍘栭崐?閺€顖欑帛/鐎规矮鐜い鐢电搼閸栧搫鐓欓妴?
- 娑撹桨鍞惍浣疯厬閻╁瓨甯村鏇犳暏娴ｅ棜瀚抽弬鍥у瘶娑旂喓宸辨径杈╂畱 `common.done` 閸氬本顒炵悰銉ュ帠 en/zh 閺傚洦顢嶉妴?

**妤犲矁鐦夌紒鎾寸亯**:
- `pnpm typecheck` 闁俺绻冮妴?
- i18n key 鐎佃鐦紒鎾寸亯閿涙瓪missing zh count 0`閵?
- 濞村繗顫嶉崳銊ㄥ殰閸斻劌瀵查幎鑺ョ叀闁俺绻冮敍姝?pricing`閵嗕梗/keys`閵嗕梗/admin/model-config`閵嗕梗/admin/page-content`閵嗕梗/admin/users`閵嗕梗/admin/accounts`閵嗕梗/admin/proxies`閵嗕梗/admin/usage` 閸у洦婀崣鎴犲箛 raw i18n key閿涘奔绡冮弮?intlify missing-key 鐠€锕€鎲￠妴?
- 閹惰姤鐓＄粻锛勬倞閸涙娅ヨぐ鏇熲偓浣锋櫠閺嶅繐鐣弫瀛樻▔缁€鐚寸窗娴狀亣銆冮惄妯糕偓浣界箥缂佸娲冮幒褋鈧胶鏁ら幋椋庮吀閻炲棎鈧礁鍨庣紒鍕吀閻炲棎鈧焦绗柆鎾额吀閻炲棎鈧浇顓归梼鍛吀閻炲棎鈧浇澶勯崣椋庮吀閻炲棎鈧焦膩閸ㄥ鍘ょ純顔衡偓渚€銆夐棃銏犲敶鐎瑰箍鈧浇顓归崡鏇狀吀閻炲棎鈧礁鍘栭崐濂稿帳缂冾喚鐡戦妴?

**閸撯晙缍戝▔銊﹀壈娴滃銆?*:
- 婵″倹鐏夊ù蹇氼潔閸ｃ劋绮涢弰鍓с仛鐏忔垿鍣洪懣婊冨礋閹存牕褰夐柌蹇撴倳閿涘奔绱崗鍫熺閻炲棙妫?localStorage / 闁偓閸戞椽鍣搁惂浼欑幢娑斿澧?simple-mode 閻ц缍嶉幀浣稿讲閼崇晫绱︾€涙ü绨?`run_mode='simple'`閵?
- 娑撳瓨妞?Playwright 閸欘亞鏁ゆ禍搴㈡拱閸︾増濞婇弻銉礉瀹歌弓绮犳笟婵婄娑擃厾些闂勩倧绱濋張顏冪箽閻ｆ瑥婀?`package.json`閵?

## [2026-05-01] docs: 閺傛澘顤?Codex 閸掓繂顫愰崠鏍嚛閺?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `AGENTS.md`
- `docs/dev/CHANGELOG_CUSTOM.md`

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- Low. Documentation-only change.

**閸欐ɑ娲跨拠锔藉剰**:
- 閸╄桨绨?`CLAUDE.md` 閹绘劗鍋?Codex 閸忋儱褰涚拠瀛樻閿涘奔绻氶悾娆愮仸閺嬪嫪绱崗鍫涒偓涔debase 閺傚洦銆傚▽澶嬬┅閵嗕垢npm-only閵嗕笒nt/Wire 閻㈢喐鍨氶妴涔竨sh/deploy 闂団偓閹哄牊娼堢粵澶庮潐閸?
- 閺傛澘顤冮崗鎶芥暛閺傚洣娆㈢槐銏犵穿閿涘苯鍙ч懕鏂挎倵缁旑垰鍙嗛崣锝冣偓浣虹秹閸忓磭鍎圭捄顖氱窞閵嗕笒nt/migrations閵嗕礁澧犵粩顖氬弳閸欙絻鈧線鍎寸純鎻掓嫲瀹搞儱鍙块弬鍥︽
- 閺嶏繝鐛欓崗鎶芥暛鐠侯垰绶為獮鍓佇╅梽銈呯秼閸?checkout 娑擃厺绗夌€涙ê婀惃?`deploy/remote_exec.py`閵嗕梗tools/secret_scan.py` 娴ｆ粈璐熼崗鎶芥暛閺傚洣娆㈠鏇犳暏

## [2026-05-01] fix(frontend): cache_transfer_ratio 閸?display_rate_multiplier 閺冪姵纭舵穱顔芥暭

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue`
- `frontend/src/components/admin/user/UserModelPricingModal.vue`

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- Low. Frontend-only change.

**閸欐ɑ娲跨拠锔藉剰**:
- `Number(val) || null` 濡€崇础鐏?`0` 鐠囶垵娴嗘稉?`null`閿涘苯鎮楃粩顖氭▕闁插繑娲块弬?`if != nil` 鐠哄疇绻冪拠銉ョ摟濞堢绱濈€佃壈鍤ч崐鍏兼￥濞夋洝顫︽穱顔芥暭娑?0
- 閺囨寧宕叉稉?`toNullableNum()` 鏉堝懎濮崙鑺ユ殶閿涙氨鈹栭崐?NaN 閳?null閿涘本婀侀弫鍫熸殶鐎涙绱欓崥?0閿涘鍟?number
- 閸氬本妞傛穱顔碱槻娴滃棗鍙忕仦鈧Ο鈥崇€风€规矮鐜?dialog 閸滃瞼鏁ら幋椋庨獓鐎规矮鐜?modal 娑撱倕顦?

## [2026-05-01] fix(display): skip cache transfer for channel-override usage logs

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/handler/dto/display_pricing.go` 閳?add `stripCacheTransferIfChannel` helper
- `backend/internal/handler/dto/mappers.go` 閳?call helper in `UsageLogFromService` and `UsageLogFromServiceAdmin`

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- Low. Changes are in dto layer display logic only.

**閸欐ɑ娲跨拠锔藉剰**:
- 瑜?usage log 缂佸繗绻冨〒鐘讳壕鐠伮ゅ瀭閿涘湑hannelID 闂堢偟鈹栭敍澶嬫閿涘畳isplay transform 娑撳秴鍟€鎼存梻鏁ら崗銊ョ湰閻?CacheTransferRatio
- 娣囶喖顦叉禍鍡樼闁捁顩惄鏍︾幆閺嶉棿绲剧紓鎾崇摠鏉烆剛些娴犲秶鏁撻弫鍫濐嚤閼峰鏁ら幋椋庢箙閸掓壆娈?token 閸掑棗绔锋稉搴＄杽闂勫懓顓哥拹閫涚瑝娑撯偓閼峰娈?bug

## [2026-04-30] feat(admin): add cache status dashboard module

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/handler/admin/dashboard_handler.go` 閳?add `/admin/dashboard/cache-status` handler.
- `backend/internal/repository/usage_log_repo.go` 閳?aggregate cache read/create stats from `usage_logs`.
- `frontend/src/views/admin/DashboardView.vue` 閳?add admin dashboard cache status module.
- `frontend/src/api/admin/dashboard.ts` / `frontend/src/i18n/locales/*` 閳?add API types and copy.

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- Low. This is an additive admin dashboard feature; likely conflicts only if upstream edits the same dashboard files.

**閸欐ɑ娲跨拠锔藉剰**:
- Add cache read rate, cache creation rate, request hit rate, prompt token total, trend buckets, and per-model cache status.
- Support `1h`, `6h`, `24h`, and `7d` windows. Default platform is `antigravity`, with an `all` option.
- Status levels: `insufficient` for fewer than 5 requests, `healthy` for read rate >= 50%, `watch` for 20%-50%, and `unhealthy` below 20%.

## [2026-04-30] fix(repository): restore Redis concurrency slot Lua compatibility

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/repository/concurrency_cache.go` 閳?remove `TIME` calls from write-capable Redis Lua scripts.

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- Low. The behavior and key layout are unchanged; only the timestamp source moves from Redis Lua to Go.

**閸欐ɑ娲跨拠锔藉剰**:
- Pass current Unix seconds from Go into `acquireScript`, `getCountScript`, and `cleanupExpiredSlotsScript`.
- Fix Redis error `Write commands not allowed after non deterministic commands`, which caused `gateway.user_slot_acquire_failed` and immediate IDE retry on `/antigravity/v1/messages`.
- Verified locally with `claude-opus-4-7` Antigravity messages endpoint returning 200 through `http://127.0.0.1:8081`.

## [2026-04-30] fix(antigravity): stabilize Claude Opus cache inputs

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/pkg/antigravity/request_transformer.go` 閳?normalize cache-sensitive request fields before forwarding to Antigravity v1internal.
- `backend/internal/pkg/antigravity/request_transformer_test.go` 閳?add regression tests for billing-header filtering and metadata session normalization.

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- Low. The change is scoped to Antigravity Claude request transformation; upstream sync conflicts should be limited to the same transformer tests if upstream edits this area.

**閸欐ɑ娲跨拠锔藉剰**:
- Drop dynamic `x-anthropic-billing-header` system lines before building `systemInstruction`, so per-request `cch=` changes do not perturb the upstream implicit cache key.
- Normalize JSON-form `metadata.user_id` from new Claude CLI clients. Prefer stable `device_id`, fall back to `session_id`, and preserve plain string user IDs.
- Keeps non-billing system text intact and preserves existing generated fallback session IDs when metadata is absent.

## [2026-04-28] fix(antigravity): 閺勬儳绱￠崠鏍侀崹瀣Ё鐏忓嫬鍨归梽銈呭弳閸欙絽鑻熼梾鎰瀹告彃鐡ㄩ崷銊╊暕鐠?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/components/account/CreateAccountModal.vue` - Antigravity 鐠愶箑褰块弬鏉跨紦瀵湱鐛ラ惃鍕Ё鐏忓嫬鍨归梽銈嗗瘻闁筋喗鏁兼稉鐑樻▔瀵繑鏋冪€涙瀵滈柦顕嗙礉妫板嫯顔曢幐澶愭尦闂呮劘妫屽鎻掔摠閸︺劍妲х亸鍕┾偓?
- `frontend/src/components/account/EditAccountModal.vue` - Antigravity 鐠愶箑褰跨紓鏍帆瀵湱鐛ラ崥灞绢劄娑撳﹨鍫禍銈勭鞍閵?
- `frontend/src/components/admin/model-pricing/AntigravityMappingCard.vue` - 閸忋劌鐪?Antigravity 姒涙顓婚弰鐘茬殸缂傛牞绶い鐢垫畱閸掔娀娅庨崗銉ュ經閺€閫涜礋閺勬儳绱￠弬鍥х摟閹稿鎸抽妴?

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- 缁绢垰澧犵粩顖欐唉娴滄帊绱崠鏍电礉娑撳秵鏁奸崣妯烘倵缁旑垱妲х亸鍕掗弸鎰潐閸掓瑱绱遍崥灞绢劄娑撳﹥鐖堕弮鏈电秵閸愯尙鐛婇妴?

**閸欐ɑ娲跨拠锔藉剰**:
- 鐟欙絽鍠?Antigravity 閺勭姴鐨犳稉顓炲毉閻?`claude-opus-4.7` / `claude-opus-4-7` 缁鎶€闁插秴顦叉い瑙勬閿涘瞼鏁ら幋鐑芥娴犮儱褰傞悳鏉垮灩闂勩倕鍙嗛崣锝囨畱闂傤噣顣介妴?
- 鐠愶箑褰垮鍦崶娑擃厼顕?Claude 4.x 閻愮懓褰?閻厽铆缁惧灝鍟撳▔鏇炰粵閸氬瞼琚弰鐘茬殸閸掋倖鏌囬敍宀勪缉閸忓秴鎻╅幑鐑筋暕鐠佹儳鍟€濞嗏剝妯夌粈鐑樺灗濞ｈ濮為崥宀€琚柌宥咁槻閺勭姴鐨犻妴?
- `濡€崇€烽柊宥囩枂` 娑撴槒銆冮幙宥勭稊閸掓藟閸忓懐娲块幒銉ф畱閳ユ粌鍨归梽銈嗘Ё鐏忓嫧鈧繃瀵滈柦顕嗙礉闁灝鍘よ箛鍛淬€忛崗鍫熷ⅵ瀵偓閺勭姴鐨犵紓鏍帆 popover 閹靛秷鍏橀崚鐘绘珟閵?

## [2026-04-28] fix(antigravity): 閺囧瓨鏌婃妯款吇鐎广垺鍩涚粩顖滃閺堫剙鍩?1.23.2

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/pkg/antigravity/oauth.go` 閳?姒涙顓?`ANTIGRAVITY_USER_AGENT_VERSION` 娴?`1.21.9` 閺囧瓨鏌婇崚?`1.23.2`
- `backend/internal/pkg/antigravity/oauth_test.go` 閳?閺囧瓨鏌婃妯款吇 User-Agent 閺傤叀鈻?
- `deploy/docker-compose.yml` 閳?闁繋绱?`ANTIGRAVITY_USER_AGENT_VERSION`
- `deploy/.env.example` 閳?鐞涖儱鍘?Antigravity User-Agent 閻楀牊婀伴柊宥囩枂鐠囧瓨妲?

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- 娴ｅ酣顥撻梽鈺嬬幢娴犲懏娲块弬浼寸帛鐠?User-Agent 閻楀牊婀伴敍灞肩矝閸忎浇顔忔潻鎰攽閻滎垰顣ㄩ柅姘崇箖 `ANTIGRAVITY_USER_AGENT_VERSION` 鐟曞棛娲婇妴?

**閸欐ɑ娲跨拠锔藉剰**:
- Google Antigravity 娑撳娴囨い闈涚秼閸?stable 娑撳娴囩捄顖氱窞娑?`stable/1.23.2-...`閿涘本婀伴崷浼寸帛鐠併倓绮涙稉?`antigravity/1.21.9 windows/amd64`閵?
- 娑撳﹥鐖舵潻鏂挎礀 `This version of Antigravity is no longer supported. Please upgrade to receive the latest features.` 閺冭绱濇导妯哄帥閹偓閻?User-Agent 閻楀牊婀版潻鍥ㄦ＋閵?
- 閺囧瓨鏌婃妯款吇閸婄厧鑻熺悰銉ュ帠闁劎璁查悳顖氼暔閸欐﹢鍣洪敍宀勪缉閸忓秶鏁撴禍褍顔愰崳銊ユ礈閺堫亝妯夊蹇氼啎缂冾喚澧楅張顒冣偓宀€鎴风紒顓濆▏閻劍妫€广垺鍩涚粩顖涘瘹缁惧箍鈧?

## [2026-04-27] feat(antigravity): 濞ｈ濮炵紓鎾崇摠鐠囧﹥鏌囬弮銉ョ箶

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/config/config.go` 閳?Gateway struct 閺傛澘顤?`LogCacheDiagnostics` 鐎涙顔?+ Viper 姒涙顓婚崐鍏兼暈閸?
- `backend/internal/pkg/antigravity/request_transformer.go` 閳?閺傛澘顤?`CacheDiagnostics` 缂佹挻鐎担鎾虫嫲 `ExtractCacheDiagnostics()` 閸戣姤鏆?
- `backend/internal/service/antigravity_gateway_service.go` 閳?Forward() 娑擃厽鍧婇崝鐘侯嚞濮?閸濆秴绨查梼鑸殿唽鐠囧﹥鏌囬弮銉ョ箶

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- 缁绢垱鏌婃晶鐑囩礉娑撳秴濂栭崫宥勭瑐濞撶鎮庨獮?

**閸欐ɑ娲跨拠锔藉剰**:
- 閼冲本娅欓敍姝漧aude-opus-4-7 鐠囬攱鐪扮紒?Antigravity 楠炲啿褰存潪顒€褰傞崥?0% 缂傛挸鐡ㄩ崨鎴掕厬閿涘矁鈧苯鎮撶捄顖氱窞閻?claude-opus-4-6 閺?99.7% 缂傛挸鐡ㄩ崨鎴掕厬閻?
- 閺傛澘顤?`gateway.log_cache_diagnostics` 闁板秶鐤嗗鈧崗绛圭礄姒涙顓婚崗鎶芥４閿涘绱濋悽鐔堕獓閻滎垰顣ㄩ柅姘崇箖 `GATEWAY_LOG_CACHE_DIAGNOSTICS=true` 閸氼垳鏁?
- 瀵偓閸氼垰鎮楃拋鏉跨秿閿涙essionId閵嗕够ystemInstruction hash/prefix/per-part hash閵嗕恭ontents 缂佹挻鐎妴涔絥stable_part 閺勫孩鏋?
- 閸氬本妞傜拋鏉跨秿娑撳﹥鐖舵潻鏂挎礀閻?cache_read/cache_creation tokens

**鐠嬪啰鐖虹紒鎾诡啈閿涘牊鍩呴懛?2026-04-30閿?*:

缂佸繐顦挎潪顔垮嚡娴狅綀鐦栭弬顓ㄧ礉鐎规矮缍呴崚棰佺瑐濞撴悂娈ｅ蹇曠处鐎涙ê銇戦弫鍫㈡畱娑撱倓閲滈悪顒傜彌閸ョ姷绀岄敍?

1. **systemInstruction 娑?`x-anthropic-billing-header` block 閻?`cch=` 鐎涙顔屽В蹇旑偧鐠囬攱鐪伴柈钘夊綁**
   - Claude Code CLI 閸?system prompt 閺佹壆绮嶉惃鍕儑娑撯偓娑?text block 濞夈劌鍙?`x-anthropic-billing-header: cc_version=2.1.12x.xxx; cc_entrypoint=cli; cch=xxxxx;`
   - `cch`閿涘潏ontext content hash閿涘鐦℃潪顔碱嚠鐠囨繈鍏橀崣姗堢礉鐎佃壈鍤?systemInstruction 閻?Part[2] hash 娑撳秶菙鐎?
   - 娴ｅ棔绮犻弫鐗堝祦閻绱濋柈銊ュ瀻鐢?billing header 閻ㄥ嫯顕Ч鍌欑矝閻掓儼鍏橀崨鎴掕厬缂傛挸鐡ㄩ敍宀冾嚛閺勫簼绗傚〒鍝ョ处鐎涙ü绗夌€瑰苯鍙忔笟婵婄 system instruction prefix 閸栧綊鍘?
   - 娣囶喖顦查弬鐟版倻閿涙艾婀?`buildSystemInstruction` 娑擃叀绻冨?`x-anthropic-billing-header` 瀵偓婢跺娈?system block

2. **`metadata.user_id` JSON 鐞氼偅鏆ｆ稉顏嗘暏娴?sessionId**
   - 閺傛壆澧?Claude CLI 閸欐垿鈧?`metadata.user_id = {"device_id":"...","account_uuid":"","session_id":"xxx"}`
   - `request_transformer.go:161-163` 鐏忓棙鏆ｆ稉?JSON 鐎涙顑佹稉鑼纯閹恒儴绁撮崐鑲╃舶 `innerRequest.SessionID`
   - 閼宠棄鎳℃稉顓犵处鐎涙娈戠拠閿嬬湴閿涙瓪metadata_user_id` 娑撹櫣鈹栭敍鍧癳ssionId 閺勵垱鏆熺€?hash閿涘鍨ㄩ崣顏呮箒 `device_id`閿涘牊妫?session_id 鐎涙顔岄敍?
   - 娑撳秷鍏橀崨鎴掕厬缂傛挸鐡ㄩ惃鍕嚞濮瑰偊绱癭metadata_user_id` 閸栧懎鎯?`session_id` UUID閿涘牊鐦℃稉?Claude Code 娴兼俺鐦芥稉宥呮倱閿?
   - 娣囶喖顦查弬鐟版倻閿涙矮绮?JSON 娑擃厽褰侀崣?`session_id` 鐎涙顔岄崡鏇犲娴ｈ法鏁ら敍灞惧灗娴犲懐鏁?`device_id` 娴ｆ粈璐?sessionId

**娣囶喖顦查悩鑸碘偓?*閿?026-04-30 瀹告彃婀?`request_transformer.go` 閽€钘夋勾鏉╁洦鎶?billing header 娑撳氦顫夐懠鍐ㄥ `metadata.user_id`閿涘矁鐦栭弬顓熸）韫囨绱戦崗鍐插讲閸︺劎鏁撴禍褔鐛欑拠浣虹处鐎涙ê鎳℃稉顓炴倵閸忔娊妫撮妴?

## [2026-04-27] feat(openai): 濞ｈ濮?GPT-5.5 / GPT-5.5 Pro 濡€崇€烽弨顖涘瘮

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/pkg/openai/constants.go` 閳?DefaultModels 閸掓銆?
- `backend/internal/service/openai_codex_transform.go` 閳?codexModelMap + normalizeCodexModel
- `backend/internal/service/billing_service.go` 閳?fallback 鐎规矮鐜妴涔琫tFallbackPricing閵嗕巩sOpenAIGPT54Model
- `backend/resources/model-pricing/model_prices_and_context_window.json` 閳?閸斻劍鈧礁鐣炬禒閿嬫蒋閻?

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- 娑撳﹥鐖?v0.1.112 鐏忔碍婀ǎ璇插 GPT-5.5 閺€顖涘瘮閿涙稐绗傚〒姝屽閸氬海鐢诲ǎ璇插闂団偓娴滃搫浼愮€靛綊缍堥崶娑橆槱閺傚洣娆?

**閸欐ɑ娲跨拠锔藉剰**:
- 閼冲本娅欓敍姝刾enAI 娴?2026-04-23 閸欐垵绔?GPT-5.5閿涘奔绗傚〒鍛婃弓鐠虹喕绻橀敍娑樺斧 normalizeCodexModel 娑?`gpt-5.5` 娴兼俺顫?`gpt-5` 閸忔粌绨抽柅鏄忕帆闂堟瑩绮梽宥囬獓娑?`gpt-5.1`閿涘苯顕遍懛纾嬵嚞濮瑰倷绗夐柅?
- 閺傛澘顤冨Ο鈥崇€烽敍姝歡pt-5.5`閿?5/$30 per MTok閿涘鈧梗gpt-5.5-pro`閿?30/$180 per MTok閿?
- codexModelMap 閸栧懎鎯?reasoning effort 閸氬海绱戦崣妯圭秼閿涘潱one/low/medium/high/xhigh閿涘寮?chat-latest
- 闂€澶哥瑐娑撳鏋冪€规矮鐜径宥囨暏 GPT-5.4 閻ㄥ嫰妲囬崐纭风礄272K input tokens, 2x input / 1.5x output閿?

## [2026-04-21] ops(deploy): 娑?docker-compose 娑撳閲滈張宥呭閸旂姵妫╄箛妤勭枂鏉?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `deploy/docker-compose.yml` 閳?`sub2api` / `postgres` / `redis` 閸氬嫬濮?`logging: { driver: json-file, options: { max-size: 50m, max-file: 5 } }`

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- 娴犲懓鎷烽崝鐘茬摟濞堢绱濇稉宥嗘暭閸斻劍妫﹂張澶愬帳缂冾噯绱辨稉濠冪埗閼汇儵鍣搁崘?compose 缂佹挻鐎棁鈧禍鍝勪紣鐎靛綊缍堝銈勭瑏濞?

**閸欐ɑ娲跨拠锔藉剰**:
- 閼冲本娅欓敍?026-04-20 閺?23:01 閻㈢喍楠囬張铏诡梿閻╂ê鍟撳鈥愁嚤閼锋潙鐣烽張鐚寸礄`rsyslogd: No space left on device`閿涘绱濋弽鐟版礈閺?Docker 姒涙顓?`json-file` 閺冦儱绻旀す鍗炲З閺冪姾鐤嗘潪顑跨瑐闂勬劧绱漙sub2api` 鐎圭懓娅掗幐?~4.3 GB/婢垛晝鐤粔顖ょ礉8 婢垛晝鐤拋?~37 GB閿涘矁鈧鏁栭弽鍦磸閿涙盯鍣搁崥顖氭倵 `docker compose up` 闁插秴缂撶€圭懓娅掓い鍝勭敨閸掔娀娅庨弮?`*-json.log`閿涘瞼顥嗛惄妯诲娴?100% 闂勫秴娲?45%
- 娣囶喖顦查敍姘槨鐎圭懓娅掓稉濠囨 5 鑴?50 MB = 250 MB閿涘奔绗佺€圭懓娅掗崥鍫ｎ吀閺堚偓婢?~750 MB閿涘奔绮犲銈勭瑝娴兼艾鍟€鐞氼偄顔愰崳銊︽）韫囨澧﹂悥鍡欘梿閻?
- 閻㈢喐鏅ョ捄顖氱窞閿涙瓭ommit 閳?push 閳?`python deploy/remote_exec.py --update`閿涘潉update.sh` 鐟欙箑褰?`docker compose up -d`閿涘苯顔愰崳銊╁櫢瀵ょ儤妞傞弬?`logging` 闁板秶鐤嗛幍宥堟儰娴ｅ稄绱?
- 閸氬海鐢诲鍛閿涙埃鎲?濞撳懐鎮?15.84 GB build cache 閸?24 娑?dangling 闂€婊冨剼閿涙稈鎲?`ops_error_logger` 閸?postgres 娑撳秴褰叉潏鐐閻ゎ垳濯柌宥堢槸閸掗攱妫╄箛妤嬬礉闂団偓閸旂娀鈧喓宸奸梽鎰煑

## [2026-04-21] docs(sales): 閸掓繄澧楅柨鈧崬顔诲敩閻炲棙澧滈崘?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `docs/sales/SALES_HANDBOOK.md` 閳?**閺傛澘缂?*閵嗗倿娼伴崥鎴犲缁斿绱戦崣鎴ｂ偓?/ AI 瀹搞儱鍙挎稉顏冩眽閻劍鍩涢惃鍕敘閸烆喕鍞悶鍡樺閸愬矉绱? 缁旂媴绱版禍褍鎼ф稉鈧崣銉ㄧ樈 / 閺嶇绺鹃崡鏍仯 / 閼宠棄濮忓〒鍛礋 / 娴ｈ法鏁ゅù浣衡柤 / 鐎规矮鐜憴鍕灟 / FAQ / 闁库偓閸烆喛鐦介張?/ 鐟欙箒鎻〒鐘讳壕 / 闂勫嫬缍嶉妴鍌涘閺堝鍙挎担鎾诲櫨妫版繐绱欏Ч鍥╁芳閵嗕焦膩閸ㄥ宕熸禒鏋偓渚€顩婚崗鍛喘閹姰鈧浇绻戦悙鐧哥礆閻ｆ瑧鈹栭敍鍧勯埢?____`閿涘绱濋柨鈧崬顔藉瘻瑜版挻妫╅弨璺ㄧ摜閻滄澘婧€婵夘偄鍟撻妴?
- `.gitignore` 濞夈劍鍓伴敍姝歞ocs/*` 鐞氼偄鎷烽悾銉礉閹绘劒姘﹂張顒佹瀮娴犲爼娓?`git add -f`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 缁绢垱鏌婃晶鐐存瀮濡楋綇绱濇稉搴濈瑐濞撳憡妫ら崘鑼崐閿涙矖docs/sales/` 閺勵垯绨╁鈧稉鎾崇潣閻╊喖缍?

**閸欐ɑ娲跨拠锔藉剰**:
- 閸楁牜鍋ｉ弶銉︾爱娴滃簼鍞惍浣风皑鐎圭儑绱欐稉澶婂礂鐠侇喖鍚嬬€瑰箍鈧胶鐭橀幀褌绱扮拠婵勨偓浣哄晬閺傤厹鈧礁顦块弨顖欑帛闁岸浜鹃妴涔€OTP閵嗕甫ey 缁狙囶杺鎼达讣绱氶敍灞炬￥閼峰棝鈧?
- 鐎规矮鐜粩鐘哄Ν閸欘亜鍟撻張鍝勫煑閿涘澅oken 閸欏苯鎮?/ cache hit / 闂€澶哥瑐娑撳鏋冮崐宥囧芳 / Priority-Flex 濡楋絼缍?/ USD閳墫NY閿涘绱濇稉宥呭晸閺佹澘鐡?
- FAQ 閹稿鏁崜?/ 閹恒儱鍙?/ 鐠伮ゅ瀭 / 缁嬪啿鐣鹃幀?/ 鐎瑰鍙忔禍鏃傜矋閿涙稑鎯?Claude Code + Cursor 閸忚渹缍嬮幒銉ュ弳閸涙垝鎶?
- 鐠囨繃婀抽崥顐＄瑏娑擃亜绱戦崷铏瑰閺?+ 娴滄柨銇囧鍌濐唴鎼存柨顕?+ 娑撴挳妫稉鈧懘姘侀弶?

**閸忓疇浠?Issue/PR**: 閳?

---

## [2026-04-20] fix: 娣囶喖顦?Gemini 鐠愶附鍩?OAuth 閸掗攱鏌?Token 鐡掑懏妞?

**瑜板崬鎼烽懠鍐ㄦ纯**: backend/internal/service/account.go
**娑撳﹥鐖堕崗鐓庮啇閹?*: 閸欘垵鍏樻稉搴濈瑐濞撶鎮撻崠鍝勭厵娣囶喗鏁奸崘鑼崐閿涘苯鎮庨獮鑸垫濞夈劍鍓?
**閸欐ɑ娲跨拠锔藉剰**:
- OAuth token refresh 鐡掑懏妞傛禒?10s 閺€閫涜礋 30s
- 閺傛澘顤冮柌宥堢槸闁槒绶敍鍫熸付婢?3 濞嗏槄绱濋幐鍥ㄦ殶闁偓闁尅绱?

**閸忓疇浠?Issue/PR**: 閺冪媴绱欑痪澶哥瑐閹烘帗鐓￠崣鎴犲箛閿?
-->

## [2026-04-19] feat(admin/usage): "閻劍鍩涚憴鍡氼潡鐎佃鐦?閹惰棄鐪介崜宥囶伂濞?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/api/admin/usage.ts` 閳?閺傛澘顤?`getUserViewPreview(logId)` API 娑?`UserViewPreview` / `UserViewSnapshot` / `UserViewConfigUsed` 缁鐎烽敍娑欏瘯鏉炶棄鍩?`adminUsageAPI` 姒涙顓荤€电厧鍤?
- `frontend/src/components/admin/usage/UserViewCompareDrawer.vue` 閳?**閺傛澘缂?*閵嗗倸鐔€娴?`BaseDialog` 閻?extra-wide 鐎电鐦藉鍡礉鐏炴洜銇?real / user_view 閸欏苯鍨€佃鐦?+ 瀹割喖绱?閿涙稑鍨庣紒鍕剁窗Tokens / Costs / Invariants閿涙盯銆婇柈銊ョ潔缁€?`config_used`閿涘牆鎯?`has_user_override` badge閿涘绱盿ctual_cost 娑撳秳绔撮懛瀛樻缁俱垼澹婇崨濠咁劅
- `frontend/src/components/admin/usage/UsageTable.vue` 閳?閺傛澘顤?`userViewClick` emit 娑?`<template #cell-actions>` 濞撳弶鐓?eye 閹稿鎸?
- `frontend/src/views/admin/UsageView.vue` 閳?`allColumns` 閺堫偄鐔弬鏉款杻 `actions` 閸掓绱盽ALWAYS_VISIBLE` 閸栧懎鎯?`actions`閿涙稒鏌婃晶?`userViewLogId/userViewOpen/handleUserViewClick/closeUserViewDrawer` 閻樿埖鈧椒绗屾径鍕倞閿涙矖<UsageTable>` 閻╂垵鎯?`@userViewClick`閿涙稒膩閺夋寧婀幐鍌濇祰 `<UserViewCompareDrawer>`
- `frontend/src/i18n/locales/zh.ts`閵嗕梗en.ts` 閳?`admin.usage` 閼哄倻鍋ｉ弬鏉款杻 actions/viewUserPerspective/userView* 缁?16 娑?key

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- 娴犲懓鎷烽崝鐘插灙娑撳海绮嶆禒璁圭礉閺堫亝鏁奸崝銊у箛閺堝鍨〒鍙夌厠閿涙稐绗傚〒姝屽閺€鐟板З admin usage 鐞涖劎娈戦崚妤冪波閺嬪嫸绱濋棁鈧憰浣瑰Ω `actions` 閸掓鎷烽崝鐘诲櫢閸嬫艾宓嗛崣?

**閸欐ɑ娲跨拠锔藉剰**:
- 娑撳孩妲伴弮銉ユ倵缁旑垱顔?`GET /admin/usage/:id/user-view` 闁板秴顨滈敍宀勬４閻滎垯绨?缁狅紕鎮婇崨妯烘倵閸欐壆娲块幒銉ф箙閻劍鍩涢崜宥囶伂鐟欏棜顫?閻ㄥ嫬浼愭担婊勭ウ閳ユ柡鈧梻顓搁悶鍡楁喅閻愮懓鍤悰灞界啲 eye 閸ョ偓鐖?閳?閹惰棄鐪介幏澶嬪复閸?閳?瀹革箑褰哥€佃鐦?real(缁狅紕鎮婇崨妯款潒鐟? vs user_view(閻劍鍩涚€圭偤妾惇瀣煂)閿涘苯鑻熼弽鍥ㄦ暈閸濐亙绨?display 闁板秶鐤嗛悽鐔告櫏閿涘牆鎯堥崗銊ョ湰 vs 閻劍鍩涚憰鍡欐磰閺夈儲绨敍?
- 閹惰棄鐪介懛顏勫З闂呮劘妫岄崗?0 鐎涙顔屽▓纰夌礉闁灝鍘ら崳顏堢叾閿涙矟iff 閸掓ぞ浜掔痪?缂?+ 閻ф儳鍨庡В鏃囥€冩潏鐐杹婢?缂傗晛鐨?
- `pnpm typecheck` 闁俺绻冮敍娌梡npm build` 閸︺劋绗岄張顒佹暭閸斻劍妫ら崗宕囨畱 PricingView.vue 娑撳﹥婀?cnyRate TS 闁挎瑱绱欐导姘崇樈瀵偓婵澧犲鎻掔摠閸︺劎娈戦張顏呭絹娴溿倖鏁奸崝顭掔礆閿涘奔绗夐梼璇差敚瑜版挸澧犲▓?

## [2026-04-19] feat(admin/usage): 閺傛澘顤?閻劍鍩涚憴鍡氼潡"鐎佃鐦０鍕潔閹恒儱褰涢敍鍫濇倵缁旑垱顔岄敍?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/handler/admin/usage_handler.go` 閳?`UsageHandler` 閺傛澘顤?`userModelPricingService` 娓氭繆绂嗛敍娑欐煀婢?`GetUserViewPreview` handler 娑撳酣鍘ゆ總?DTO閿涘潉UserViewPreviewResponse` / `UserViewSnapshot` / `UserViewConfigUsed` / `snapshotFromDTO`閿?
- `backend/internal/server/routes/admin.go` 閳?濞夈劌鍞?`GET /api/v1/admin/usage/:id/user-view`
- `backend/cmd/server/wire_gen.go` 閳?`admin.NewUsageHandler` 鐠嬪啰鏁ゆ晶鐐端?`userModelPricingService` 閸欏倹鏆熼敍鍧刧o generate` 閸ョ娀銆嶉惄?Wire 瀹告彃鐡ㄩ崷銊ф畱婢舵氨绮︾€规岸妫舵０妯恒亼鐠愩儻绱濋弫鍛閸?patch閿涙稐绗夎ぐ鍗炴惙閸旂喕鍏橀敍?
- `backend/internal/handler/admin/usage_cleanup_handler_test.go`閵嗕梗usage_handler_request_type_test.go` 閳?閸氬本顒?`NewUsageHandler` 閺傛壆顒烽崥宥忕礄婢舵矮绱舵稉鈧稉?nil閿?

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- 缁绢垱鏌婃晶鐐殿伂閻?+ 閺嬪嫰鈧姴鍤遍弫鐗堟汞娴ｅ秴澧犳稉鈧担宥嗗絻閸欏偊绱濇稉搴濈瑐濞?admin usage handler 閺€鐟板З閸欘垵鍏樻禍褏鏁撶亸蹇撳暱缁愪緤绱濇担鍡楀棘閺佷即銆庢惔蹇撳綁閸栨牕顔愰弰鎾圭槕閸?

**閸欐ɑ娲跨拠锔藉剰**:
- 閻╊喚娈戦敍姘鳖吀閻炲棗鎲抽幒鎺撶叀閺屾劒閲滈悽銊﹀煕閿涘牆顩?gybilly2023閿?閸撳秶顏€圭偤妾惇瀣煂閻?token / 閹存劖婀?閺勵垰鎯佺粭锕€鎮?`cache_transfer_ratio` + `display_input_price` 缁?婵傜鏅?闁板秶鐤嗘０鍕埂閿涘瞼娲伴崜宥呮暜娑撯偓閸旂偞纭堕弰顖滄瑜版洝顕氶悽銊﹀煕鐠愶箑褰挎禍鑼簜閻?
- 閺傜増甯撮崣锝咁嚠閸楁洘娼?usage_log 闁插秵鏌婄捄鎴滅瑏鐏?transform閿涙艾鍙忕仦鈧?display 娴?閳?user model overrides閿涘潉BuildUserDisplayPricingMap`閿涘鍟?user group display rate閿涘潉ApplyUserDisplayRate`閿涘绱濇潻鏂挎礀 `real` / `user_view` 娑撱倕鍨€佃鐦?+ `config_used` 闁板秶鐤嗗┃顖涚爱閿涘牆鎯?`has_user_override`閵嗕梗user_group_rate`閿?
- 鐎瑰苯鍙忔径宥囨暏 `dto.UsageLogFromService` / `ApplyDisplayTransform` / `ApplyUserDisplayRate` / `BuildUserDisplayPricingMap`閿涘奔绗夐崘娆愭煀鐠侊紕鐣婚柅鏄忕帆
- 娑撳秴濮╅悳鐗堟箒閸掓銆冮弻銉嚄闁槒绶垾鏂衡偓鎿緼dminUsageLog.DisplayFields` 娴犲秵瀵滈崗銊ョ湰 displayMap 缁犳绱欐穱婵囧瘮閸氭垵鎮楅崗鐓庮啇閿?
- 瀹稿弶婀伴崷?`go run ./cmd/server` 妤犲矁鐦夌捄顖滄暠濮濓絿鈥樺▔銊ュ斀閵嗕笩in 閺?radix 閸愯尙鐛?panic
- 閸撳秶顏崗銉ュ經娑撳孩濞婄仦?UI 瀵板懍绗呮稉鈧▓鍨絹娴?

## [2026-04-19] feat(pricing): 濡€崇€锋禒閿嬬壐鐞涖劌鎮撻弮璺虹潔缁€?CNY 鐎圭偘绮柌鎴︻杺閿涘牊瀵滈崗鍛偓鑲╊吀閻炲棙宕茬粻妤冨芳閿?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/views/user/PricingView.vue` 閳?娴犻攱鐗哥悰銊ュ幢閻楀洭銆婇柈銊ュ USD閳墫NY 閹广垻鐣?banner閿涘牅绮庨崷?`payment_cny_per_usd > 0` 閺冭埖妯夌粈鐚寸礆閿涙矖formatTokenPrice` / `formatPerRequest` 閹峰棔璐?`tokenPrimary`/`tokenSecondary` + `perRequestPrimary`/`perRequestSecondary` 閸ユ稐閲?helper閿涙NY 娑撹櫣鐭栨担鎾插瘜閺勫墽銇氶敍瀛禨D 閸旂姵瀚崣宄扮毈閻忔澘鐡ч崜顖涙▔缁€鐚寸幢閺堫亪鍘ょ純顔藉床缁犳宸奸弮鎯板殰閸斻劑鈧偓閸栨牔璐熼崡鏇氱 USD 閺勫墽銇?
- `frontend/src/i18n/locales/{zh,en}.ts` 閳?閺傛澘顤?`pricing.cnyBanner`閿涙稑鍨径鏉戝箵閹哄鈥栫紓鏍垳 `$/MTok` 閺€閫涜礋閵嗗矁绶崗銉ょ幆 / MTok閵嗗秲鈧瓥nput / MTok閵嗗秷顔€閸楁洖鍘撻弽鑹板殰鐢箑绔电粔宥囶儊閸欏嚖绱盽unitHint` 閺€鐟板晸娑撻缚顕╅弰?妤?/ $ 閸氼偂绠熼惃鍕蓟鐢胶顫掗弬鍥攳

**閺傚洦顢?*閿涙氨鏁ら幋閿嬪房閺夊啳瀵栭崶鏉戝敶閻ㄥ嫬鐫嶇粈鐑樷偓褎鏋冪€涙绱檅anner 閺傚洦顢嶉妴浣稿礋娴ｅ秷顕╅弰搴礆閿涘奔绗夐崝?i18n 闁插苯鍙炬禒鏍︾瑹閸斺剝鏋冨鍫涒偓?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ簺鈧倻鍑介崜宥囶伂 + i18n 鐞涘苯鍞存穱顔芥暭閵?

**閸欐ɑ娲跨拠锔藉剰**:
1. 鐟欏棜顫庣粵鏍殣閿涙NY 娑撴眹鈧箒SD 鏉堝懌鈧倹鐦℃稉顏冪幆閺嶇厧宕熼崗鍐╃壐 `妤?.50 ($5.00)` 閸氬矁顢戦敍娑樹箯娓氀呯煐娴?CNY 閺勵垳鏁ら幋宄扮杽闂勫懏澧哥拹褰掑櫤缁狙嶇礉閸欒櫕瀚崣宄板敶閻忔澘鐡?$ 閺勵垱鍑藉┃鎰贩閹?
2. 妞ゅ爼鍎存稉鈧▎鈩冣偓?banner 鐠囧瓨妲戦幑銏㈢暬閻滃浄绱檂妤?.7 / 1 USD 璺?閺夈儴鍤滈崗鍛偓鑲╊吀閻炲摲閿涘绱濋崡鏇炲帗閺嶅ジ鍣风亸鍙樼瑝闁插秴顦?鑴?0.7"
3. 闁偓閸栨牠鈧槒绶敍姘鳖吀閻炲棗鎲抽張顏堝帳缂?`payment_cny_per_usd`閿涘牆鈧棿璐?0 閹?null閿涘鍟?banner 閼奉亜濮╅梾鎰閵嗕焦澧嶉張澶婂礋閸忓啯鐗搁崣顏呮▔缁€?USD閿涘奔绗岄弨鐟板З閸撳秴鐣崗銊ょ閼疯揪绱濋柆鍨帳閸戣櫣骞?`妤?` 娑斿琚惃鍕磽鐢?
4. 閹傜幆濮ｆ柨顕В鏃撶礄鑴?0閵嗕礁鐣奸弬閫涚幆 鑴?0.7 缁涘绱氬鎻掓躬娑撳﹥鏌熺拋鈥茬幆濡€崇础鐠囧瓨妲戦柌宀冾唹鏉╁浄绱濇禒閿嬬壐鐞涖劍婀伴煬顐＄瑝閸愬秴褰旈崝?鐢倸婧€鐢瓕顫嗘禒?閸掓绱濇穱婵囧瘮鐞涖劍鐗搁獮鎻掑櫍

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮瑰偊绱欓幒?pricing-page 閺傚洦顢嶉弨褰掆偓鐙呯礆

---

## [2026-04-19] docs(architecture): 閺傛澘顤冩い鍦窗閹垛偓閺堫垱鐏﹂弸鍕瀮濡?+ CLAUDE.md 鐟欏嫬鍨?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `docs/dev/ARCHITECTURE.md` 閳?閺傛澘顤冮妴鍌炪€婄仦鍌氬弳閸欙絾鏋冨锝忕礉鐟曞棛娲婇幎鈧張顖涚垽閵嗕礁澧犻崥搴ｎ伂閻╊喖缍嶉崚鍡楃湴閵嗕浇顕Ч鍌滄晸閸涜棄鎳嗛張鐔粹偓涔刬re DI 鐟佸懘鍘ら弬鐟扮础閵嗕讣ettings/PublicSettings KV 濡€崇础閵嗕浇绺肩粔鑽ゅ鐎规哎鈧胶绱︾€涙鐡ラ悾銉ｂ偓浣筋吇鐠囦焦宸块弶鍐︹偓浣鼓侀崹瀣暰娴犵柉袙閺嬫劧绱遍崜宥囶伂閻ㄥ嫯鐭鹃悽?store/api client/鐢啫鐪?i18n/閸欏秹顩痪锕€鐣鹃敍? 娑擃亜鐖剁憴浣哥磻閸欐垳鎹㈤崝锛勬畱閵嗗本濡遍崘娆忕础閵嗗秵膩閺夊尅绱欓弬鏉款杻 setting 鐎涙顔?/ 閺傛澘顤冪€涙劗绮ㄩ弸?setting / 閺傛澘顤冮悽銊﹀煕 API / 閺傛澘顤?ent 鐎涙顔?/ 閺傛澘顤冮崜宥囶伂妞?/ 閺傛澘顤?i18n 闁款噯绱氶敍娑欐拱閸︽澘瀵查惃鍕┾偓灞藉嚒閻儱娼欓悙骞库偓宥嗙閸楁洩绱橶ire 娑撹鍏辨径杈Е閵嗕梗docs/dev` gitignore閵嗕笩it Bash POSIX 鐠侯垰绶為弨鐟板晸閵嗕箘indows 缁旑垰褰涢崘鑼崐缁涘绱氶敍娑櫮侀崸妤佺箒鎼达附鏋冨锝咁嚤閼?
- `docs/dev/codebase/README.md` 閳?閸︺劍娓舵稉濠冩煙閸旂姳绔村▓纰夌礉閹跺﹥鐏﹂弸鍕瀮濡楋絽鐣炬担宥勮礋閵嗗苯鍘涚拠缁樻拱閺嬭埖鐎妴浣稿晙閹稿膩閸ф銆冨ǎ鍗炲弳閵嗗秶娈戦崗銉ュ經
- `CLAUDE.md` 閳?Quick Reference 妞ゅ爼鍎撮崝?ARCHITECTURE.md閿涙悲ey Development Rules 缁?3 閺夆剝鏌婃晶鐐偓灞惧赴缁鳖澀鍞惍浣稿閸忓牐顕?ARCHITECTURE.md閵?閵嗗奔缍嶉弮鑸垫纯閺?ARCHITECTURE.md閵嗗稄绱欓弬鏉款杻濡€虫健閵嗕焦鏁肩捄銊ュ瀼闂堛垻瀹崇€规哎鈧礁褰傞悳鐗堟煀閸ф垯鈧焦濞婇崙鍝勫讲婢跺秶鏁ゅΟ鈩冩緲閸ユ稓琚憴锕€褰傞弶鈥叉閿涘绱遍崢鐔粹偓瀛媜debase Map閵嗗秷顫夐崚娆戠椽閸欒渹绮?3 妞よ櫣些閸?4閿涘苯鎮楃紒?4閳?0 閸忋劑鍎?+1

**娑撳﹥鐖堕崗鐓庮啇閹?*: 闂嗚翰鈧倻鍑介弬鍥ㄣ€傞妴?

**閸欐ɑ娲跨拠锔藉剰**:
1. 閺傚洦銆傜€规矮缍呴敍姘仸閺嬪嫭鏋冨锝勭瑝閺勵垱膩閸?deep-dive閿涘矁鈧本妲搁妴宀冩硶閸掑洭娼扮痪锕€鐣?+ 閸忋儱褰涚€佃壈鍩呴妴宥冣偓鍌浤侀崸妤冪矎閼哄倻鎴风紒顓熸杹 `codebase/{module}.md`閵?
2. 濡剝婢樼粩鐘哄Ν閿涘煪?閿涘娲块幒銉﹀Ρ鐏忚精鍏橀悽顭掔窗濮ｅ繑娼柈鐣岀舶娴滃棗鍙挎担鎾舵畱閺傚洣娆㈢捄顖氱窞閸滃矂銆庢惔蹇ョ礉濮ｆ柣鈧瞼鐡戞稉瀣偧閸欏牆绶遍悳鐗堟嚋缁鳖澀绔撮柆宥冣偓宥呮彥瀵板牆顦块妴?
3. 瀹歌尙鐓￠崸鎴礄鎼?閿涘濡告导姘冀婢跺秷淇惃?Wire / docs/dev / Git Bash / Windows 缁旑垰褰涚粵澶夌皑閺佸懎鍙忛柈銊︾焽濞ｂ偓閿涘矂浼╅崗宥勭瑓濞嗏€冲嫉閼鸿鲸妞傞梻鏉戭槻閻╂ǜ鈧?

**閸忓疇浠?Issue/PR**: 閺冪媴绱欓弶銉ㄥ殰娴兼俺鐦介幀鑽ょ波閿?

---

## [2026-04-19] feat(login-page): 瀹革附鐖弨閫涜礋 6 瀵姴宕遍悧鍥风礉閸氬牆鑻熼幒銊ョ畭闁偓鐠囧嘲鑻熺粔濠氭珟閸擃垱鐖ｆ０妯活唽

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/views/auth/LoginView.vue` 閳?閸掔娀娅庨崜顖涚垼妫?`<p>` 娴犮儱寮?`loginDescription` computed閿涙稓瀚粩瀣畱閹恒劌绠嶉柇鈧拠宄版健缁夊娅庨敍娌桭eatureKey` 閹碘晛鍩?6閿涘牆濮?`tutorial` / `referral`閿涘绱盽featureCards` 闁板秶鐤嗛崝鐘辫⒈瀵姴宕遍敍鍫ユ綒閼?/ 閻滎偆鐭囬敍澶婅嫙閸氬嫰鍘ら崶鐐垼閿涘潌ook-open / gift閿涘绱盽featureHighlightTerms{Zh,En}` 鐞?tutorial 閸?referral 娑撱倗绮嶆妯瑰瘨鐠囧稄绱眊rid 娴?2鑴? 鐠嬪啩璐?2鑴?閿涘牅绮涢弰?`sm:grid-cols-2`閿?
- `frontend/src/i18n/locales/{zh,en}.ts` 閳?`auth.login.features.*` 閺傛澘顤?`tutorial.{title,desc}`閿涙矖auth.login.referral` 缂佹挻鐎禒?`{tag,title,body}` 閸氬牆鑻熸潻?`features.referral.{title,desc}`閿涘本顒滈弬鍥ㄥ瘻閵嗗苯褰查崢瀣級閵嗗秴甯崚娆戠翱缁犫偓

**閺傚洦顢?*: `features.tutorial` 閺傚洤鐡ф稉銉︾壐娴ｈ法鏁ら悽銊﹀煕缂佹瑥鐣鹃崢鐔告瀮閵嗕繖features.referral.desc` 娑撹桨绗傛稉鈧▎鈥冲窗娴ｅ秶顭堥惃鍕竾缂傗晝澧楅敍鍫熷房閺夊啫甯囩紓鈺嬬礆閵嗗倸鍙炬担娆忓幢閻楀浄绱檓etered / quality / models / enterprise閿涘鐣崗銊︾梾閸斻劊鈧繖auth.login.description` i18n 闁款喕绻氶悾娆庣稻娑撳秴鍟€濞撳弶鐓嬮妴?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ簺鈧倻鍑介崜宥囶伂 + i18n 缂佹挻鐎拫鍐╂殻閵?

**閸欐ɑ娲跨拠锔藉剰**:
1. 閸擃垱鐖ｆ０妯活唽閿涘牄鈧矂娼伴崥鎴濈磻閸欐垼鈧懎鎷伴崶銏ゆЕ閻ㄥ嫬顦垮Ο鈥崇€锋稉顓℃祮缁旀瑢鈧腹鈧负鈧稄绱氶幐澶愭付濮瑰倸鍨归梽銈忕礉`auth.login.description` 闁款喗娈忛弮鏈电箽閻ｆ瑩浼╅崗宥呭従娴犳牗缍旈崷銊ョ穿閻劊鈧?
2. 閺傛澘顤冪粭?5 瀵姴宕遍妴灞界暚閸犲嫮娈戦崚婵嗩劅閼板懏鏆€缁嬪鈧稄绱伴棃鎺曞閿涘潉#22D3EE`閿涘瀵屾０姗堢礉book-open 閸ョ偓鐖ｉ妴?
3. 閹恒劌绠嶉柇鈧拠铚傜矤閻欘剛鐝涢崸妤€褰夋稉铏诡儑 6 瀵姴宕遍敍姘卞缚缁绱檂#F472B6`閿涘瀵屾０姗堢礉gift 閸ョ偓鐖ｉ妴鍌涘伎鏉╂澘甯囩紓鈺€璐熸稉鈧崣銉礉閵嗗奔璧撮崢姘殯閸?/ 閹镐胶鐢绘潻鏂惧墤閵嗗秳琚辨径鍕暏娑撳顣介懝鏌ョ彯娴滎喖宸辩拫鍐︹偓?
4. 閹烘帒鍨敍姝硂w1 = metered + quality閿涘ow2 = models + tutorial閿涘ow3 = enterprise + referral閿涘本瀵滈妴灞剧壋韫囧啩鐜崐?閳?娴溠冩惂閼宠棄濮?閳?鏉╂盯妯?閹恒劌绠嶉妴宥堝殰閻掕埖鏁归弶鐔粹偓?

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮?

---

## [2026-04-19] style(login-page): 4 瀵?feature 閸椔ゎ潒鐟欏濮為柌?+ 閸忔娊鏁拠宥夌彯娴?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/views/auth/LoginView.vue` 閳?濮ｅ繐绱堕崡鈩冩煀婢х偤銆婇柈銊ゅ瘜妫版澹婇崗澶婄敨閵嗕梗10鑴?0` 鐢箒澹婇崶鐐垼閸фぜ鈧梗17px` 缁鐖ｆ０妯糕偓涔?4px` 濮濓絾鏋冮敍娑欏伎鏉╀即鍣烽悧鐟扮暰閸忔娊鏁拠宥忕礄娴犻攱鐗搁妴?鐡掑懘鐝幀褌鐜В?閵嗕梗Opus 4.7` / `GPT-5.4` / `Gemini 3.1 Pro`閵?瀵偓缁? 缁涘绱氶悽?`splitWithTerms` 閸︺劏绻嶇悰灞炬閹峰棙顔岄獮鍓佹暏娑撳顣介懝鎻掑缁绱遍弬鏉款杻 `FeatureKey` 缁鐎烽妴涔scapeRegExp`/`splitWithTerms` 鏉堝懎濮崙鑺ユ殶娴犮儱寮锋稉顓″娑撱倕顨滄妯瑰瘨鐠囧秷銆冮敍娑欏腹楠炲潡鍊嬬拠宄版健 padding / 閺嶅洭顣界€涙褰块悾銉︽暪閿涘矁顔€ 4 瀵姴宕遍悧鍥ф躬鐟欏棜顫庣仦鍌滈獓娑撳﹥娲跨粣浣稿毉

**閺傚洦顢?*: 娑撳秴褰夐妴淇檃uth.login.features.*.{title,desc}` 閸?`auth.login.referral.*` 閸忋劑鍎存稉搴濈瑐娑撯偓娑擃亝褰佹禍銈勭閼疯揪绱濋張顒侇偧缁绢垵顫嬬憴澶婄湴閺€鐟板З閵?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ簺鈧倸褰ч弨鍦瑜版洟銆夐弽閿嬫緲 + 缂佸嫪娆㈢痪褍鍞撮柈銊╁帳缂冾喓鈧?

**閸欐ɑ娲跨拠锔藉剰**:
1. 濮ｅ繐绱堕崡鈩冩箒閻欘剛鐝涙稉濠氼暯閼硅绱版禒閿嬬壐閿涘牓娼氱紒鍖＄礆/ 閸濅浇宸濋敍鍫ｆ憫閿? 濡€崇€烽敍鍫紶閿? 娴间椒绗熼敍鍫㈡儉閻濃偓閿涘绱濋崶鐐垼閼冲本娅?+ 妤傛ü瀵掔拠?+ 妞ゅ爼鍎?2px 閸忓鐢柈鍊熺閻偓闁板秷澹婇崣妯糕偓?
2. 妤傛ü瀵掔拠宥嗘Ц鐟欏棜顫庣憴鍕灟閿涘奔绗夐弰顖涙瀮濡楀牞绱伴悽銊ょ娴?`featureHighlightTermsZh|En` 閸︺劏鍓奸張顒勫櫡婢圭増妲戦敍宀冪箥鐞涘本妞傞悽銊︻劀閸掓瑦濯堕幓蹇氬牚娑撹绱濋崠褰掑帳閸掓澘姘ㄩ崠?`<span>` 閸欐鐭栭崝鐘哄閿涙铂18n 閺傚洦顢嶉弨鐟板З閸氬氦瀚㈠▽鈥虫嚒娑擃叏绱濋崣顏呮Ц娑撳秹鐝禍顕嗙礉娑撳秵濮ら柨娆嶁偓?
3. 閸楋紕澧?shell閿涙瓪rounded-[22px]` + 濞撴劕褰夋惔?+ 閺囨潙宸遍梼鏉戝 + hover 閺冭泛褰夋禍顕嗙礉閺佺繝缍嬫担鎾诲櫤閺勫孩妯夌搾鍛扮箖閹恒劌绠嶉崸妞尖偓?
4. 閹恒劌绠嶉崸妤嬬窗padding 娴?`p-5` 鐠嬪啫鍩?`px-5 py-4`閿涘本鐖ｆ０?18閳?6閿涘矁顔€鐟欏棜顫庨悞锔惧仯閽€钘夋躬 4 瀵姴宕遍悧鍥︾瑐閵?

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮瑰偊绱欓幒銉ょ瑐閺?feature 閸楋繝鍣哥拋鎹愵吀閿?

---

## [2026-04-19] feat(login-page): 瀹革附鐖拃銉╂敘閸栫儤鏁奸悧鍫窗4 瀵?feature 閸?+ 閹恒劌绠嶉柇鈧拠?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/views/auth/LoginView.vue` 閳?閸掔娀娅庡锔界埉娑撳宕愰崠铏规畱 feature pills閵嗕焦膩閸ㄥ鐫嶇粈铏圭秹閺嶇鈧? 瀵姵妫?feature cards 閸滃奔绗夐崘宥勫▏閻劎娈?`modelChannels` / `paymentCnyPerUsd` / `loginSupportedModelsTitle` / `loginModelsDesc`閿涙稒鏌婃晶?2鑴? 閻?4 瀵?feature 閸楋紕澧栭敍鍫ｎ吀缁犳鐫橀幀?`featureCards`閿涘绗岄幒銊ョ畭闁偓鐠囧嘲宸辩拫鍐ㄥ隘閸?
- `frontend/src/i18n/locales/{zh,en}.ts` 閳?閺傛澘顤?`auth.login.features.{metered,quality,models,enterprise}.{title,desc}` + `auth.login.referral.{tag,title,body}` 娑撱倗绮嶉柨顕嗙幢娣囨繄鏆€ `featurePrice`閵嗕梗featureUnifiedApi*` 缁涘妫柨顔荤瑝閸旑煉绱欓柆鍨帳瑜板崬鎼烽崗鏈电铂缂佸嫪娆?/ 闂冨弶顒涙稉濠冪埗閸愯尙鐛婇敍澶涚礉閸欘亝妲搁惂璇茬秿妞ゅ灚膩閺夊じ绗夐崘宥呯穿閻?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ簺鈧倸澧犵粩顖涚壉閺夊潡鍣搁崘?+ 閺傛澘顤?i18n閿涙稑鎮楃粩顖樷偓浣规殶閹诡喖绨辨稉宥呭З閵?

**閸欐ɑ娲跨拠锔藉剰**:
1. 妞ゅ爼鍎撮崠杞扮矝閻?badge / 娑撱倛顢戦弽鍥暯 / description 缂佸嫭鍨氶敍灞鹃儴閻劋绠ｉ崜宥囨畱缁狅紕鎮婇崨妯哄讲缂傛牞绶憰鍡欐磰閺堝搫鍩楅敍鍧刲ogin_page.*` settings 鐎涙顔岄敍澶堚偓?
2. 娑撳宕愰崠杞扮濞嗏剝鏂佺€?4 瀵姴宕遍悧?+ 1 瀵姵甯归獮鍧楀€嬬拠宄板幢閿涘矁顫嬬憴澶婄湴缁狙嶇窗feature 閸椻槄绱欐稉顓熲偓褎绻侀懝鎻掔俺閿涘鍟?閹恒劌绠嶉崡鈽呯礄闂堟帞璞㈠〒鎰綁 + 閼窖冨帨閹诲繗绔熼敍澶嬪Ω闁插秶鍋ｉ幏澶婄磻閵?
3. 4 瀵姴宕遍悧鍥х秼閸撳秷铔?i18n 绾剛绱惍渚婄礄閺傚洦顢嶇粙鍐茬暰閿涘绱濋崥搴ｇ敾閼汇儵娓剁粻锛勬倞閸涙ê褰茬紓鏍帆閿涘苯濮炵€涙顔岄崚?`LoginPageContent` 閸楀啿褰查妴?
4. 閹恒劌绠嶉柇鈧拠?`body` 娑撳搫宕版担宥囶焾閿涘瞼鐡戦張鈧紒鍫熸瀮濡楀牏鈥樼€规艾鎮楅惄瀛樺复閺€?i18n 閹存牕宕岀痪褌璐熺粻锛勬倞閸涙ê褰茬紓鏍帆鐎涙顔岄妴?
5. 缁狅紕鎮婇崨妯肩椽鏉堟垵娅掗柌宀€娈?`supportedModelsTitle`閵嗕梗modelsDesc` 娑撱倕鐡у▓鍨拱濞喡ゆ崳娑撳秴鍟€瑜板崬鎼烽惂璇茬秿妞ゅ灚瑕嗛弻鎿勭礄娣囨繄鏆€鐎涙顔岄弳鍌欑瑝閸掔媴绱濋崥搴ｇ敾缂佺喍绔村〒鍛倞閿涘鈧?

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮?

---

## [2026-04-18] fix(settings): 閻ц缍嶆い鍏哥幆閺嶇厧濮╅幀浣稿 + 娣囶喖顦查崗鍛偓鑲╊吀閻炲棔绻氱€涙顕ゅ〒鍛敄濞夈劌鍞界粵澶庮啎缂?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/service/settings_view.go` 閳?`PublicSettings` 閺傛澘顤?`PaymentCNYPerUSD float64`
- `backend/internal/service/setting_service.go` 閳?`GetPublicSettings` 鐠囪褰?`SettingCNYPerUSD`閿涙矖GetPublicSettingsForInjection` 濞夈劌鍙嗛崠鍨倳缂佹挻鐎担鎾虫倱濮濄儲鏌婃晶鐐茬摟濞?
- `backend/internal/handler/dto/settings.go` 閳?閸忣剙绱戠拋鍓х枂 DTO 閺傛澘顤?`payment_cny_per_usd`
- `backend/internal/handler/setting_handler.go` 閳?閸?`GetPublicSettings` 閸濆秴绨查柌灞斤綖閸忓懏鏌婄€涙顔?
- `frontend/src/types/index.ts` 閳?`PublicSettings` 閹恒儱褰涢弬鏉款杻 `payment_cny_per_usd: number`
- `frontend/src/stores/app.ts` 閳?姒涙顓荤粚娲帳缂冾喛藟姒?`payment_cny_per_usd: 0`
- `frontend/src/i18n/locales/zh.ts`閵嗕梗en.ts` 閳?`featurePrice` 閺€閫涜礋鐢?`{price}` 閸楃姳缍呴惃鍕侀弶鍖＄幢閺傛澘顤?`featurePriceDefault` 娴ｆ粈璐熼張顏堝帳缂冾喗妞傞惃鍕礀闁偓閺傚洦顢?
- `frontend/src/views/auth/LoginView.vue` 閳?閺傛澘顤?`paymentCnyPerUsd` ref閿涘畭onMounted` 娴犲骸鍙曞鈧拋鍓х枂鐠囪褰囬敍娌爀ature pill 閹稿鍘ょ純顔煎З閹焦瑕嗛弻鎿勭礉閺堫亪鍘ょ純顔兼礀闁偓
- `frontend/src/api/admin/settings.ts` 閳?閺傛澘顤?`systemSettingsToUpdateRequest(SystemSettings) => UpdateSettingsRequest` 閺勭姴鐨犻崙鑺ユ殶閿涙稒鏁為崗?`settingsAPI`
- `frontend/src/views/admin/RechargeConfigView.vue` 閳?`save()` 閸?`getSettings()` 閸愬秵鏆ｆ担?`updateSettings(...)`閿涘苯褰х憰鍡欐磰 `payment_cny_per_usd` / `payment_bonus_tiers`

**娑撳﹥鐖堕崗鐓庮啇閹?*:
- 閸氬海顏弬鏉款杻鐎涙顔屾稉鍝勫讲闁鎷烽崝鐙呯礉閸氬牆鑻熸稉濠冪埗閺冩儼瀚㈡稉濠冪埗娑旂喐鏁奸崝?`PublicSettings` / 閸忣剙绱戠拋鍓х枂 handler閿涘瞼鏆€閹板繐鍟跨粣浣风秴缂冾噯绱欓崸鍥﹁礋缂佹挻鐎担鎾崇啲闁劍鍨?return 鐎涙顔岄崚妤勩€冮敍?
- 閸撳秶顏弬鏉款杻閻?`systemSettingsToUpdateRequest` 閺勵垱婀伴崷棰佺癌瀵偓瀹搞儱鍙块崙鑺ユ殶閿涘瞼瀚粩瀣╃艾娑撳﹥鐖?

**閸欐ɑ娲跨拠锔藉剰**:
- Bug 1 閳?閻ц缍嶆い鍏哥幆閺嶈偐鈥栫紓鏍垳閿涙瓪LoginView` 閸樼喎鍘涘〒鍙夌厠 `t('auth.login.featurePrice')` 閻ㄥ嫰娼ら幀浣规瀮濡?`'0.6 / 1$ 鐠?`閿涘奔绗?admin 閸?閸忓懎鈧偐顓搁悶?鐠佸墽鐤嗛惃?`payment_cny_per_usd` 鐎瑰苯鍙忛懘閬嶆尙閵嗗倻骞囩亸鍡氼嚉濮瑰洨宸奸柅姘崇箖 `/api/v1/settings/public` 閺嗘挳婀堕敍鍫滅瑢 SSR 濞夈劌鍙嗙捄顖氱窞娣囨繃瀵旀稉鈧懛杈剧礆閿涘苯澧犵粩顖濐嚢閸欐牕鎮楁禒?`{price} / 1$ 鐠х﹫ 濡剝婢樺〒鍙夌厠閿涙稐璐?0 閹存牗婀柊宥囩枂閺冭泛娲栭柅鈧崚?`featurePriceDefault` 闂堟瑦鈧焦鏋冨鍫涒偓?
- Bug 2 閳?"濮ｅ繑顐奸柈銊ц瀵偓閺€鐐暈閸愬矁顫﹂柌宥囩枂"閿涙氨婀″锝嗙壌閸ョ姳绗夐弰顖炲劥缂冭尪鍓奸張顑锯偓鍌氭倵缁?`UpdateSettingsRequest` 缂佹繂銇囨径姘殶 `bool` / `string` 鐎涙顔岄弰?*闂堢偞瀵氶柦?*閿涘瓰SON 閸欏秴绨崚妤€瀵查弮鍓佸繁婢跺崬鐡у▓鍏哥窗鐞氼偄锝?`false` / `""`閿涙矖RechargeConfigView.save()` 閸欘亜褰?`payment_cny_per_usd` 娑?`payment_bonus_tiers`閿涘andler 缂佈呯敾閺嬪嫰鈧姴鐣弫?`SystemSettings` 楠?`SetMultiple` 閸ョ偛鍟撻敍灞筋嚤閼?`registration_enabled`閵嗕梗site_name`閵嗕副IDC/LinuxDo 瀵偓閸忓磭鐡戠悮顐︽饯姒涙ɑ绔荤粚鎭掆偓鍌欐叏婢跺秹鍣伴悽銊︽付鐏忓繑鏁奸崝顭掔窗`RechargeConfigView` 閸忓牊濯虹€瑰本鏆?settings閿涘瞼鏁ら弬鏉跨紦閻ㄥ嫭妲х亸鍕毐閺佹媽娴嗛幋鎰嚞濮瑰倷缍嬮敍灞藉晙鐟曞棛娲婃稉銈勯嚋 payment 鐎涙顔岄崣鎴濆毉閿涘奔濞囬崶鐐插晸閺?鐠囩粯妫崐鐓庡晸閺冄冣偓?閿涘矂浼╅崗宥堫嚖濞撳懐鈹栭妴鍌氬殶閹诡喚琚€涙顔岄敍鍧剆mtp_password` 缁涘绱氶崷銊︽Ё鐏忓嫬鍤遍弫棰佽厬閺佸懏鍓伴悾娆戔敄閿涘苯鎮楃粩?缁屽搫鈧壈鐑︽潻鍥洬閻?鐎瑰牊濮㈢紒褏鐢婚悽鐔告櫏閵?

**妤犲矁鐦夐弬鐟扮础**:
- `go build ./...` 闁俺绻冮敍娑樺缁?`pnpm run typecheck` 闁俺绻冮敍娌╝ndler 閻╃鍙ч崡鏇熺ゴ闁俺绻冮敍鍧癳rvice 鐏炲倸褰?`gemini_oauth_service_test.go` 妫板嫬鐡ㄩ崷銊ф畱 mock 閹恒儱褰涙稉宥呯暚閺佹潙濂栭崫宥忕礉閺堫亝鏌婃晶鐐寸ゴ鐠囨洖銇戠拹銉礆
- 閹靛浼愰敍姘帠閸婅偐顓搁悶鍡曠箽鐎?`cny_per_usd=0.8` 閳?閻ц缍嶆い鍨▔缁€?`0.8 / 1$ 鐠х﹫閿涙稑鎮撻弮鍓侀兇缂佺喕顔曠純顕€鍣?瀵偓閺€鐐暈閸?缁涘绱戦崗鍏呯箽閹镐胶鏁ら幋铚傜閸撳秶娈戦崐闂寸瑝閸?


**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/ent/schema/ai_credit_snapshot.go` 閳?閺?Ent schema閿涙瓪AICreditSnapshot { email, credit_type, amount, captured_at }` + 婢跺秴鎮庣槐銏犵穿
- `backend/ent/aicreditsnapshot/`閵嗕梗backend/ent/aicreditsnapshot*.go` 閳?Ent 閻㈢喐鍨氭禒锝囩垳閿涘潉go generate ./ent`閿?
- `backend/migrations/110_add_ai_credit_snapshots.sql` 閳?瀵ら缚銆?+ `(email, captured_at)` 娑?`(captured_at)` 缁便垹绱?
- `backend/internal/service/credit_snapshot.go` 閳?`CreditSnapshot` 缂佹挻鐎妴涔reditSnapshotRepository`閵嗕梗AntigravityUsageAggregator`閵嗕梗AntigravityUsageRatio` 閸濆秴绨茬猾璇茬€?
- `backend/internal/service/credit_snapshot_service.go` 閳?`CreditSnapshotService`閿?5 閸掑棝鎸?ticker 鐎规碍妞傞柌鍥ㄧ壉閵嗕梗TriggerManualCapture`閿?0 缁夋帟绻樼粙瀣敶閸愬嘲宓堥柨渚婄礆閵嗕梗GetAntigravityUsageRatio`閿涘牏娴夐柇濠氬櫚閺嶉鍋ｅ锝呮倻 delta 濮瑰倸鎷?+ `usage_logs` 閼辨艾鎮庨敍?
- `backend/internal/repository/credit_snapshot_repo.go` 閳?閸╄桨绨?Ent 閻ㄥ嫪绮ㄦ惔鎾崇杽閻滃府绱橧nsert/ListInRange/GetLatestBefore閿?
- `backend/internal/repository/antigravity_usage_aggregator.go` 閳?閻欘剛鐝涚亸蹇斿复閸欙絽鐤勯悳甯窗`SELECT COUNT + SUM(total_cost) FROM usage_logs WHERE account_id = ANY($1) AND created_at 閳?[start,end)`
- `backend/internal/handler/admin/usage_handler.go` 閳?`NewUsageHandler` 閸?`creditSnapshotService` 娓氭繆绂嗛敍娑欐煀婢?`StatsAntigravity` / `RefreshAntigravityStats`閿涙稒褰侀崣?`parseStatsDateRange` 鏉堝懎濮崙鑺ユ殶
- `backend/internal/handler/admin/{usage_cleanup_handler_test,usage_handler_request_type_test}.go` 閳?stub 鐞涖儵缍堥弬鏉垮棘閺侀缍?`nil`
- `backend/internal/server/routes/admin.go` 閳?`GET /admin/usage/stats/antigravity`閵嗕梗POST /admin/usage/stats/antigravity/refresh`
- `backend/internal/service/wire.go` 閳?閺傛澘顤?`ProvideCreditSnapshotService` 楠炶泛鍙?`ProviderSet`
- `backend/internal/repository/wire.go` 閳?`NewCreditSnapshotRepository` / `NewAntigravityUsageAggregator` 閸旂姴鍙?`ProviderSet`
- `backend/cmd/server/wire_gen.go` 閳?閹靛濮╃紓鏍ㄥ笓閺?Repo + Service + Handler 娓氭繆绂嗛敍鍫滃瘜楠?`go generate` 閸ョ姴宸婚崣?Payment 闁插秴顦茬紒鎴濈暰婢惰精瑙﹂敍灞惧瘻閻滅増婀佸Ο鈥崇础閹绘帒鍙嗛敍?
- `frontend/src/api/admin/usage.ts` 閳?閺傛澘顤?`AntigravityUsageRatio` 缁鐎烽妴涔etAntigravityStats`閵嗕梗refreshAntigravityStats`
- `frontend/src/components/admin/usage/AntigravityRatioCard.vue` 閳?閺傛壆绮嶆禒璁圭窗4 閸掓瀵氶弽鍥у幢 + 閵嗗瞼鐝涢崡鎶藉櫚閺嶆灚鈧秵瀵滈柦?+ 闁插洦鐗辨稉宥堝喕/閸愬嘲宓堥幓鎰仛
- `frontend/src/views/admin/UsageView.vue` 閳?瀵洖鍙嗛崡锛勫閿涘奔绗岄悳鐗堟箒 `UsageStatsCards` 閸忚京鏁?`DateRangePicker`閿涘苯鎮撴稉鈧崚閿嬫煀闁炬崘鐭剧憴锕€褰?
- `frontend/src/i18n/locales/{zh,en}.ts` 閳?閺傛澘顤?`usage.antigravity.*` 閺傚洦顢?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ簺鈧倹澧嶉張澶嬫煀婢х偞鏋冩禒?鐎涙顔岄崸鍥﹁礋 additive閿涙稐绮?`admin/usage_handler.go` 閺嬪嫰鈧姴娅掗崝鐘插棘閺佸府绱欐稉濠冪埗閼汇儵鍣搁弸?handler 閸掓繂顫愰崠鏍劮閸氬秹娓堕崥灞绢劄閿涘绱盽wire_gen.go` 娴犲秹娓堕幍瀣紣閸氬牆鑻熼妴淇橝ntigravityUsageAggregator` 閸掔粯鍓板▽鈩冨复閸?`UsageLogRepository` 閹恒儱褰涢敍宀勪缉閸忓秵妫╅崥搴㈡暭閸斻劌宕勯崙鐘差槱 stub閵?

**閸欐ɑ娲跨拠锔藉剰**:
1. Antigravity AI Credits 娴ｆ瑩顤傛稉宥呭讲閸ョ偞鍑介弻銉嚄閿涘牐绻欑粩?API 閸欘亞绮拌ぐ鎾冲閸婄》绱氶敍灞芥礈濮濄倖鏌婃晶?`ai_credit_snapshots` 鐞涖劊鈧繖CreditSnapshotService` 濮?15 閸掑棝鎸撻崥顖氬З娑撯偓濞嗭繝鍣伴弽鍑ょ窗閹?`credentials.email` 閸樺鍣搁敍鍫濇倱 Google 鐠愶箑褰块崗鍙橀煩 credits閿涘绱濇径宥囨暏 `AccountUsageService.GetUsage` 閻?3 閸掑棝鎸撶紓鎾崇摠鐏炲倹濯烘担娆擃杺閿涘矂浼╅崗宥夘杺婢?API 閸樺濮忛妴?
2. 閼辨艾鎮庨崣锝呯窞閿涙艾顕В蹇庨嚋 email 閸?`[start - 30 min lookback, end]` 閸愬懐娈戣箛顐ゅ弾閹稿妞傞梻鏉戝磳鎼村繗铔嬮惄鎼佸仸鐎电櫢绱濈槐顖氬濮濓絽鎮?delta閵嗗倽绀嬮崥?delta閿涘牆鍘栭崐?闁插秶鐤嗛敍澶庣儲鏉╁洢鈧倹娣抽悽鐔哥槷閻?`quota_per_credit = SUM(total_cost) / total_credits`閵嗕梗calls_per_credit = COUNT(*) / total_credits`閿涘畭total_credits == 0` 閺冩儼绻戦崶?null閿涘牆澧犵粩顖氱潔缁€?闁插洦鐗辨稉宥堝喕"閹绘劗銇氶敍澶堚偓?
3. 閹靛濮╃憴锕€褰傞幒銉ュ經 `POST .../refresh` 閸?30 缁夋帟绻樼粙瀣敶閸愬嘲宓堥柨渚婄礄`sync.Mutex + lastManualAt`閿涘绱濋崘宄板祱閺堢喎鍞存潻鏂挎礀 `manual_refresh_throttled=true` 楠炴湹绗夐柌宥咁槻閹垫捁绻欑粩顖樷偓鍌滎吀閻炲棗鎲崇拠顖滃仯娑撳秳绱伴弨鎯с亣 API 閸樺濮忛妴?
4. 閸撳秶顏崡锛勫閹恒儱鍙嗛悳鐗堟箒 `startDate`/`endDate`閿涘畭loadStats()` 缂佹挻娼崥搴¤嫙鐞涘本濯?antigravity 閼辨艾鎮庨敍娑樸亼鐠愩儱褰?`console.error` 娑撳秹妯嗛弬顓濆瘜濞翠胶鈻奸妴?
5. 妤犲矁鐦夐敍姝歞ocker exec sub2api-pg-dev psql` 绾喛顓?migration 110 鎼存梻鏁ら妴涔i_credit_snapshots` 鐞涖劎绮ㄩ弸鍕劀绾噯绱遍張顒€婀撮崥顖氬З閸?`[CreditSnapshot] Scheduler started` 娑撳氦鐭鹃悽?`GET/POST /api/v1/admin/usage/stats/antigravity(/refresh)` 閸у洤鍑″▔銊ュ斀閵?

**閸忓疇浠?Issue/PR**: 閺?

---

## [2026-04-18] fix(keys): 娣囶喗顒滈妴灞藉弳闂傘劍瀵氶崡妞尖偓宥夊櫡 CC-Switch 閻ㄥ嫪绗呮潪钘夋勾閸р偓

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/components/keys/GettingStartedGuide.vue` 閳?缁楊兛绨╁銉ょ瑓鏉炶姤瀵滈柦?`href` 娴?`github.com/nicepkg/cc-switch/releases`閿涘牓鏁婄拠顖欑波鎼存搫绱氶弨閫涜礋 `github.com/farion1231/cc-switch/releases`閿涘牆鐣奸弬閫涚波鎼存搫绱?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ簺鈧倷绗傚〒姝屽閺堫亙濞囬悽銊︻劃闁剧偓甯撮崚娆愭￥閸愯尙鐛婇妴?

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮?

---

## [2026-04-18] refactor(page-content): 閸氬牆鑻熼妴宀冾吀娴犵兘銆夐弬鍥攳閵嗗秴鎷伴妴宀€娅ヨぐ鏇€夐弬鍥攳閵嗗秳璐熺紒鐔剁 Tab 妞?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/views/admin/PageContentView.vue` 閳?閺傛澘顤冮崥鍫濊嫙閻栨儼顫嬮崶鎾呯窗`AppLayout` + 閸忓彉闊╂径鎾劥 + 娑撱倓閲?tab閿涘牊膩閸ㄥ顓告禒鐑姐€?/ 閻ц缍嶆い纰夌礆 + `?tab=pricing|login` URL 閸氬本顒?+ `<KeepAlive>` 娣囨繄鏆€鐞涖劌宕熸潏鎾冲弳娑撳秳娑径?
- `frontend/src/components/admin/page-content/PricingContentForm.vue` 閳?閻?`PricingPageView.vue` 閸撱儱鍤?AppLayout/妞ゅ灚鐖ｆ０妯烘倵瀵版鍩岄敍灞肩矌娣囨繄鏆€閹绘劗銇氶崡掳鈧椒琚卞▓?textarea閵嗕椒绻氱€涙ɑ瀵滈柦?
- `frontend/src/components/admin/page-content/LoginContentForm.vue` 閳?閻?`LoginPageView.vue` 閸撱儱鍤?AppLayout/妞ゅ灚鐖ｆ０妯烘倵瀵版鍩岄敍灞肩箽閻ｆ瑤绗佺紒?8 鐎涙顔?+ 濞撳懐鈹?娣囨繂鐡?妫板嫯顫?
- `frontend/src/views/admin/PricingPageView.vue`閵嗕梗frontend/src/views/admin/LoginPageView.vue` 閳?閸掔娀娅?
- `frontend/src/router/index.ts` 閳?閺?`/admin/page-content` 鐠侯垳鏁遍敍娌?admin/pricing-page`閵嗕梗/admin/login-page` 娣囨繄鏆€娑?redirect 閸掔増鏌婄捄顖氱窞楠炶泛鐢稉?`?tab=` 閸欏倹鏆熼敍宀冣偓浣峰姛缁涘彞绗夋径杈ㄦ櫏
- `frontend/src/components/layout/AppSidebar.vue` 閳?缁狅紕鎮婇崨妯规櫠鏉堣鐖崢缁樺竴娑撱倖娼弮褔銆嶉敍灞芥値閹存劒绔撮弶掳鈧矂銆夐棃銏℃瀮濡楀牄鈧?
- `frontend/src/i18n/locales/{zh,en}.ts` 閳?閸?`nav.pricingPage` / `nav.loginPage`閿涙稒鏌婃晶?`nav.pageContent` + `admin.pageContent.{title,description,tabs.{pricing,login}}`閿涙稐绻氶悾?`admin.pricingPage.*` / `admin.loginPage.*`閿涘牅琚辨稉顏勭摍缂佸嫪娆㈡禒宥囧姧濞戝牐鍨傞敍?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ簺鈧倸褰ч崝銊ュ缁旑垽绱濋崥搴ｎ伂 handler 閸滃矁顔曠純?key 娑撳秴褰夐妴?

**閸欐ɑ娲跨拠锔藉剰**:
1. 閸氬牆鑻熼崝銊︽簚閿涙矮琚遍崸妤呭厴閺勵垬鈧苯澧犻崣浼淬€夐棃銏℃瀮濡楀牏顓搁悶鍡愨偓宥忕礉閹峰棔琚辨稉顏冩櫠鏉堣鐖弶锛勬窗閸嬪繐鍟戞担娆欑幢閺堫亝娼垫俊鍌涚亯鏉╂顩﹂崝鐘虫煀妞ょ敻娼伴敍鍫滅伐婵″倷鍗庣悰銊ф磸閵?04 妞ょ绱氱紒鐔剁閺€鎹愮箻鏉╂瑤閲?tab 妞ら潧宓嗛崣顖樷偓?
2. Tab 閸掑洦宕查柅姘崇箖 URL `?tab=...` 閸氬本顒為敍灞肩┒娴滃孩绻侀柧鐐复 + 濞村繗顫嶉崳銊ュ鏉?閸氬酣鈧偓閿涙稒婀幐鍥х暰閺冨爼绮拋?`pricing`閵?
3. `<KeepAlive>` 娣囨繄鏆€鐎涙劗绮嶆禒鍓佸Ц閹緤绱濋悽銊﹀煕閸︺劋琚辨稉?tab 娑斿妫块崚鍥ㄥ床閺冭埖婀穱婵嗙摠閻ㄥ嫮绱潏鎴滅瑝娴兼矮娑妴?
4. 閼颁浇鐭惧鍕箽閻?redirect 閸掔増鏌婄捄顖氱窞閿涘本妫稊锔绢劮楠炶櫕绮︽潻鍥ㄦ诞閵?

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮瑰偊绱欑槐褎甯存稉銈嗩偧閺傚洦顢嶉崝鐔诲厴閸氬牆鑻熼敍?

---

## [2026-04-18] feat(login-page): 缁狅紕鎮婇崨妯哄讲缂傛牞绶惂璇茬秿妞ゅ灚鏋冨?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/service/domain_constants.go` 閳?閺傛澘顤?8 娑?`SettingKeyLoginPage*` 鐢悂鍣?
- `backend/internal/service/settings_view.go` 閳?`LoginPageContent` 缂佹挻鐎敍鍧杝on tag + `IsEmpty`閿涘绱盽PublicSettings.LoginPage *LoginPageContent`
- `backend/internal/service/setting_service.go` 閳?`GetPublicSettings` 閸?8 娑?key 閸掔増澹掗柌蹇氼嚢閸欐牕鍨悰顭掔幢閺傛澘顤?`buildLoginPageContent`閿涘牏鈹栫€涙顔?trim 閸氬孩鏆ｆ担?nil 閸栨牭绱氶敍娌桮etPublicSettingsForInjection` 閻ㄥ嫬灏堕崥?struct 娑旂喎濮?`login_page`
- `backend/internal/handler/dto/settings.go` 閳?`PublicSettings` DTO 閸?`LoginPage *LoginPageContent`閿涙稒鏌婃晶?`dto.LoginPageContent`
- `backend/internal/handler/setting_handler.go` 閳?閸忣剙绱?`/settings/public` 鏉堟挸鍤弰鐘茬殸 + `toDTOLoginPageContent` 鏉堝懎濮崙鑺ユ殶
- `backend/internal/handler/admin/login_page_handler.go` 閳?閺傛澘顤冮敍娆窫T/PUT `/admin/login-page/content`閿涙稑鐡у▓鐢甸獓 trim + 闂€鍨閺嶏繝鐛欓敍鍧癶ort 255 / long 500閿?
- `backend/internal/handler/handler.go` + `wire.go` + `backend/cmd/server/wire_gen.go` 閳?`AdminHandlers.LoginPage` + provider閿涘本澧滈崝銊﹀絻閸?wire_gen 娑?pricing-page 娣囨繃瀵旈崥灞肩濡€崇础
- `backend/internal/server/routes/admin.go` 閳?`registerLoginPageRoutes`
- `frontend/src/api/loginPage.ts` 閳?閺傛澘顤?API client閿涘潉getAdminLoginPageContent` / `updateAdminLoginPageContent` / `resetAdminLoginPageContent`閿?
- `frontend/src/api/index.ts` 閳?鐎电厧鍤?
- `frontend/src/types/index.ts` 閳?`LoginPageContent` 閹恒儱褰涢敍娌桺ublicSettings.login_page?` 閸欘垶鈧鐡у▓?
- `frontend/src/views/auth/LoginView.vue` 閳?8 婢?`t('auth.login.xxx')` 閺囨寧宕叉稉?`loginXxx` computed閿涙稒鐦℃稉?computed 闁晫鏁?`pickLoginText` 閸?fallback閿涘牏鈹栨稉?閺堫亜鐣炬稊澶嬫閻?i18n 閸樼喐鏋冮敍?
- `frontend/src/views/admin/LoginPageView.vue` 閳?閺傛澘顤冪粻锛勬倞閸涙绱潏鎴︺€夐敍? 娑擃亜鐨崚鍡欑矋閿涘牐鎯€闁库偓/濡€崇€烽崠?閻ц缍嶅鍡礆8 娑擃亜鐡у▓浣冦€冮崡?+ 妫板嫯顫嶉柧鐐复 + 娣囨繂鐡?+ 閹垹顦叉妯款吇閿涘牆鐢?confirm閿涘绱辨穱婵嗙摠/閹垹顦查崥搴ば曢崣?`appStore.fetchPublicSettings(true)` 缁斿鍩㈢拋鈺佸従娴犳牗婀崚閿嬫煀閻ㄥ嫰銆夐棃銏㈡箙閸掔増鏌婇崐?
- `frontend/src/components/layout/AppSidebar.vue` 閳?`adminNavItems` 婢х偛濮為妴宀€娅ヨぐ鏇€夐弬鍥攳閵嗗秴鍙嗛崣?
- `frontend/src/router/index.ts` 閳?`/admin/login-page` 鐠侯垳鏁?
- `frontend/src/i18n/locales/{zh,en}.ts` 閳?`nav.loginPage` + `admin.loginPage.*`閿涘澅itle/description/preview/fallbackHint/sections/fields 8 妞?save/reset/reset-confirm閿?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娑擃厹鈧繖PublicSettings` 缂佹挻鐎悮顐ｅ⒖鐏炴洩绱檚ervice + DTO + TS 缁鐎烽敍澶涚礉娑撳﹥鐖堕懟銉ョ殺閺夈儲鏁奸崝銊ㄧ箹娑擃亞绮ㄩ弸鍕付鐟曚礁鎮撳銉幢閺傛澘顤?key 閸涜棄鎮曢悽?`login_page.*` 閸涜棄鎮曠粚娲？閿涘奔绗夋稉搴㈡＆閺?key 閸愯尙鐛婇妴鍌濈熅閻?/ handler / 閸撳秶顏弬鍥︽闁姤妲搁弬鏉款杻閿涘奔绗夌憰鍡欐磰娑撳﹥鐖堕妴淇檞ire_gen.go` 娴犲秹娓堕幍瀣З閸氬牆鑻熼妴?

**閸欐ɑ娲跨拠锔藉剰**:
1. 8 娑?settings key閿涘潉login_page.badge` / `heading_line1` / `heading_line2` / `description` / `supported_models_title` / `models_desc` / `form_title` / `form_subtitle`閿涘绔存稉鈧€电懓绨?i18n `auth.login.*` 闁插瞼娈戦拃銉╂敘閺傚洦顢嶇€涙顔岄妴?
2. 娴犵粯鍓扮€涙顔岀粚鍝勭摟缁楋缚瑕?閳?閸氬海顏潻鏂挎礀閻?`LoginPage` 鐎涙劗绮ㄩ弸鍕礋 nil閿涘潉omitempty` 閺佺繝缍?omit閿涘绱濋崜宥囶伂閹峰じ绗夐崚鏉挎皑缂佈呯敾閻?`t('auth.login.xxx')`閿涘奔鑵戦懟鍗炲瀼閹广垼鍤滈崝銊ф晸閺佸牄鈧?
3. 缁狅紕鎮婇崨妯圭箽鐎涙ê鎮楃拫鍐暏 `appStore.fetchPublicSettings(true)` 瀵搫鍩楅柌宥嗘煀閹峰褰?public settings閿涘矂浼╅崗宥呭従娴犳牕鍑￠幍鎾崇磻閻ㄥ嫰銆夐棃銏㈡箙閸掔増妫悧鍫涒偓?
4. 閵嗗本浠径宥夌帛鐠併們鈧? 閹靛綊鍣洪崘娆忓弳缁岃桨瑕嗛敍灞肩瑝閺勵垳澧块悶鍡楀灩 key閿涙稖顕㈡稊澶嬫纯閺勫海鈥橀敍灞肩瑬娑撳秶鏁ら崝鐘插灩闂勩倖甯撮崣锝冣偓?
5. SSR 濞夈劌鍙嗛惃?`window.__APP_CONFIG__` 娑旂喎鎮撳銉︽纯閺傚府绱檂GetPublicSettingsForInjection`閿涘绱濇＃鏍偧濞撳弶鐓嬮惂璇茬秿妞ら潧姘ㄩ弰顖涙付缂佸牊鏋冨鍫礉娑撳秹妫仦蹇嬧偓?
6. 妤犲矁鐦夐敍姝歝url /api/v1/settings/public | grep login_page` 閳?閺堫亙绻氱€涙ɑ妞傞弮?key閿涙稓娅ヨぐ鏇炴倵 `curl /admin/login-page/content` 鏉╂柨娲?8 鐎涙顔岄崗銊р敄鐎电钖勯敍娑楃箽鐎涙ê鎮?public 閹恒儱褰涘鈧慨瀣箲閸?`login_page` 鐎涙劗绮ㄩ弸鍕┾偓?

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮瑰偊绱欑紒顓溾偓灞灸侀崹瀣吀娴犵兘銆夐弬鍥攳閵嗗稄绱?

---

## [2026-04-18] fix(pricing-page): 缁狅紕鎮婇崨妯肩椽鏉堟垿銆夐張顏冪箽鐎涙ɑ妞傛０鍕綖姒涙顓婚弬鍥攳

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/handler/admin/pricing_page_handler.go` 閳?鐎电厧鍤?`DefaultPricingPageIntro` / `DefaultPricingPageEducation` 鐢悂鍣洪敍娌桮et` 閸?settings 閺堫亜鍟?/ 缁岃桨瑕嗛弮璺烘礀閽€钘夊煂姒涙顓婚崐纭风幢`loadValue` 婢舵矮绔存稉?fallback 閸忋儱寮?
- `backend/internal/handler/pricing_page_handler.go` 閳?閸掔姵甯€閺堫剙婀存妯款吇鐢悂鍣洪敍灞筋槻閻?`admin.Default*`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ簺鈧倻鍑界€涙顔岀痪褑鐨熼弫杈剧礉閺?schema / 鐠侯垳鏁遍崣妯哄閵?

**閸欐ɑ娲跨拠锔藉剰**: 閸樼喎鍘涚粻锛勬倞閸涙绻樼紓鏍帆妞ゅ灚妞?settings 闁插矁绻曞▽鈥冲晸閸忋儻绱濇稉銈勯嚋 textarea 闁姤妲哥粚铏规畱閿涘奔绲鹃悽銊﹀煕鐠佲€茬幆妞ら潧寮甸弰鍓с仛閻ㄥ嫭妲?handler 閸愬懐鐤嗘妯款吇閺傚洦顢嶉敍灞筋嚤閼锋番鈧瞼绱潏鎴滅瑝閸掓壆鏁ら幋椋庢箙閸掓壆娈戞稉婊嗐偪閵嗗秲鈧倻骞囬崷?admin Get 閹恒儱褰涙稉搴ｆ暏閹磋渹鏅堕崗杈╂暏閸氬奔绔存禒钘夌埗闁插骏绱濈粻锛勬倞閸涙顑囨稉鈧▎陇绻橀弶銉ユ皑閼崇晫婀呴崚鑸偓宀€鏁ら幋閿嬵劃閸掕鐤勯梽鍛躬閻娈戦崘鍛啇閵嗗稄绱濋惄瀛樺复閺€鐟版皑鐞涘被鈧?

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮瑰偊绱欐稉濠冩蒋閸欐ɑ娲块惃鍕倵缂侇叏绱?

---

## [2026-04-18] feat(pricing-page): 閺傛澘顤冮悽銊﹀煕閵嗗本膩閸ㄥ顓告禒鏋偓宥夈€?+ 缁狅紕鎮婇崨妯哄讲缂傛牞绶弬鍥攳

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/migrations/109_add_show_on_pricing_page.sql` 閳?`global_model_pricing` 閺傛澘顤?`show_on_pricing_page BOOLEAN`
- `backend/internal/service/global_model_pricing.go` 閳?`GlobalModelPricing` 閸?`ShowOnPricingPage` 鐎涙顔岄敍娑欏复閸欙絾鏌婃晶?`ListForPricingPage`
- `backend/internal/repository/global_model_pricing_repo.go` 閳?閹碘偓閺?SELECT/INSERT/UPDATE 閸氬本顒為弬鏉跨摟濞堢绱遍弬鏉款杻 `ListForPricingPage`
- `backend/internal/service/global_model_pricing_service.go` 閳?`GlobalOverride` DTO 閸?`show_on_pricing_page`閿涙矖ToGlobalOverride` 閸氬本顒為敍娑欐煀婢?`ListForPricingPage` 閺傝纭?
- `backend/internal/handler/admin/model_pricing_handler.go` 閳?Create/Update 鐠囬攱鐪?DTO 閸?`show_on_pricing_page *bool`
- `backend/internal/handler/admin/pricing_page_handler.go` 閳?閺傛澘顤冮敍娆窫T/PUT `/admin/pricing-page/content`閿涘矁顕伴崘?`settings` KV 娑撱倓閲?key
- `backend/internal/handler/pricing_page_handler.go` 閳?閺傛澘顤冮悽銊﹀煕娓氀嶇窗GET `/user/pricing-page`閿涘矁浠涢崥鍫滆⒈濞堝灚鏋冨?+ 閹?provider 閸掑棛绮嶉惃鍕潔缁€杞扮幆閺?
- `backend/internal/handler/handler.go` 閳?`AdminHandlers.PricingPage`閵嗕梗Handlers.PricingPage` 閺傛澘鐡у▓?
- `backend/internal/handler/wire.go` 閳?濞夈劌鍞?`NewPricingPageHandler` / `NewPricingPageAdminHandler`
- `backend/cmd/server/wire_gen.go` 閳?閹靛濮╃紓鏍ㄥ笓閺?handler 娓氭繆绂嗛敍鍧刧o generate` 閸︺劋瀵岄獮鎻掑嚒妫板嫬鍘涙径杈Е閿涘本瀵滈悳鐗堟箒濡€崇础閹绘帒鍙嗛敍?
- `backend/internal/server/routes/admin.go` 閳?`registerPricingPageRoutes`
- `backend/internal/server/routes/user.go` 閳?濞夈劌鍞?`/user/pricing-page`
- `frontend/src/api/pricingPage.ts` 閳?閺傛澘顤?API client閿涘牏鏁ら幋?Get + 缁狅紕鎮婇崨?Get/Update閿?
- `frontend/src/api/index.ts` 閳?鐎电厧鍤?`pricingPageAPI`
- `frontend/src/api/admin/modelPricing.ts` 閳?`GlobalOverride`/`CreateOverrideRequest`/`UpdateOverrideRequest` 閸?`show_on_pricing_page`
- `frontend/src/views/user/PricingView.vue` 閳?閺傛澘顤冮悽銊﹀煕妞ょ绱版稉澶庡Ν閸愬懎顔愰敍鍫熸拱缁旀瑨顓告禒閿嬆佸?/ 鐠佲€茬幆濡€崇础缁夋垶娅?/ 閹稿閽╅崣鏉垮瀻缂佸嫮娈戞禒閿嬬壐鐞涱煉绱氶敍瀛rkdown 閻?`marked@17` + `DOMPurify` 濞撳弶鐓?
- `frontend/src/views/admin/PricingPageView.vue` 閳?閺傛澘顤冪粻锛勬倞閸涙﹢銆夐敍姘⒈濞?textarea 缂傛牞绶?+ 娣囨繂鐡?+ 閹稿洤鎮滃Ο鈥崇€烽柊宥囩枂閻ㄥ嫬绱╃€?
- `frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue` 閳?缂傛牞绶€电鐦藉鍡楀閵嗗苯婀拋鈥茬幆妞ら潧鐫嶇粈鎭掆偓宥呯磻閸?
- `frontend/src/components/layout/AppSidebar.vue` 閳?閻劍鍩?娑擃亙姹夋笟褑绔熼弽蹇旀煀婢х偑鈧本膩閸ㄥ顓告禒鏋偓宥堝綅閸楁洩绱辩粻锛勬倞閸涙ü鏅舵潏瑙勭埉閺傛澘顤冮妴宀冾吀娴犵兘銆夐弬鍥攳閵嗗秴鍙嗛崣锝忕幢閺傛澘顤?`PriceTagIcon`
- `frontend/src/router/index.ts` 閳?閺傛澘顤?`/pricing` 娑?`/admin/pricing-page` 鐠侯垳鏁?
- `frontend/src/i18n/locales/{zh,en}.ts` 閳?閺傛澘顤?`pricing.*`閵嗕梗admin.pricingPage.*`閵嗕梗admin.modelPricing.showOnPricingPage` 闁款喕浜掗崣?`nav.modelPricing`閵嗕梗nav.pricingPage`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娑擃厹鈧倹鏌婃晶鐐茬摟濞?`show_on_pricing_page` 娴ｅ秳绨?`global_model_pricing` 鐞涱煉绱濇潻浣盒╅弰?additive閿涘奔绗傚〒姝屽鐏忓棙娼电€电顕氱悰銊х波閺嬪嫬浠涢弨鐟板З闂団偓閹靛濮╅崥鍫濊嫙閵嗕径andler / 鐠侯垳鏁遍崸鍥﹁礋閺傛澘顤冮敍灞肩瑝鐟曞棛娲婃稉濠冪埗閺傚洣娆㈤惃鍕＆閺堝鐭惧鍕┾偓淇檞ire_gen.go` 閹靛濮╃紓鏍帆閿涘牆娲滄稉璇插叡 Wire 閻㈢喐鍨氭０鍕帥婢惰精瑙﹂敍瀹峆rovidePaymentConfigService` 缁涘鍣告径宥囩拨鐎规熬绱氶敍灞芥値楠炴湹绗傚〒鍛婃闂団偓閻ｆ瑦鍓伴妴?

**閸欐ɑ娲跨拠锔藉剰**:
1. 缁狅紕鎮婇崨妯哄讲閸︺劊鈧本膩閸ㄥ鍘ょ純?閳?濡€崇€风拠锔藉剰閵嗗秹鍣烽崟楣冣偓澶堚偓灞芥躬鐠佲€茬幆妞ら潧鐫嶇粈鎭掆偓宥忕礉閹貉冨煑閸濐亙绨哄Ο鈥崇€烽崙铏瑰箛閸︺劎鏁ら幋铚傛櫠閻ㄥ嫯顓告禒鐑姐€夐敍宀€瀚粩瀣╃艾鐠伮ゅ瀭 `enabled` 瀵偓閸忕偨鈧?
2. 缁狅紕鎮婇崨妯哄讲閸?`/admin/pricing-page` 缂傛牞绶稉銈嗩唽 Markdown 閺傚洦顢嶉敍鍫熸拱缁旀瑨顓告禒閿嬆佸蹇嬧偓浣筋吀娴犻攱膩瀵繒顫栭弲顕嗙礆閿涘奔绻氱€涙ê鍩?`settings` 鐞涖劎娈?`pricing_page.intro_markdown` / `pricing_page.education_markdown` 娑撱倓閲?key閵嗗倹婀穱婵嗙摠閺冨墎鏁ら幋铚傛櫠閸ョ偠鎯ら崚?handler 閸愬懐鐤嗘妯款吇閺傚洦顢嶉妴?
3. 閻劍鍩?`/pricing` 妞ゅ吀绔村▎鈩冨閸欐牞浠涢崥鍫熷复閸欙綇绱版潻鏂挎礀娑撱倖顔岄弬鍥攳 + 閹?provider 閸掑棛绮嶉惃鍕潔缁€杞扮幆閺嶈壈銆冮妴鍌氱潔缁€杞扮幆閻ㄥ嫪绱崗鍫㈤獓閿涙氨鏁ら幋椋庨獓 display override > 閸忋劌鐪?display override > 閻喎鐤勯崡鏇氱幆閿涘潚allback閿涘鈧?
4. 娴犻攱鐗哥悰?per-token 娴犻攱瀵?$/MTok 閺勫墽銇氶敍瀹瞖r_request 閹?$/濞?閺勫墽銇氶妴?
5. i18n 瀹歌尪藟 zh/en 鐎瑰本鏆ｉ柨顔尖偓绗衡偓?

**閸忓疇浠?Issue/PR**: 閺堫剙婀存禍灞界磻闂団偓濮?

---

## [2026-04-17] feat(billing): 閻劍鍩涚痪褎膩閸ㄥ鐣炬禒鐤洬閻?(User Model Pricing Override)

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/migrations/106_add_user_model_pricing_overrides.sql` 閳?閺傛澘顤冪悰?
- `backend/internal/service/user_model_pricing.go` 閳?鐎圭偘缍?+ 娴犳挸鍋嶉幒銉ュ經
- `backend/internal/service/user_model_pricing_service.go` 閳?娑撴艾濮熼柅鏄忕帆鐏?
- `backend/internal/repository/user_model_pricing_repo.go` 閳?閸樼喓鏁?SQL 鐎圭偟骞?
- `backend/internal/service/model_pricing_resolver.go` 閳?PricingInput 婢х偛濮?UserID, Resolve 婢х偛濮為悽銊﹀煕缁狙嗩洬閻╂牕褰旈崝?
- `backend/internal/service/gateway_service.go` 閳?娴肩娀鈧?UserID 閸掓澘鐣炬禒鐤掗弸鎰版懠鐠?
- `backend/internal/handler/dto/display_pricing.go` 閳?閺傛澘顤?BuildUserDisplayPricingMap
- `backend/internal/handler/usage_handler.go` 閳?娴ｈ法鏁ら悽銊﹀煕缁狙冪潔缁€楦款洬閻?
- `backend/internal/handler/admin/user_model_pricing_handler.go` 閳?Admin CRUD API
- `backend/internal/service/global_model_pricing_service.go` 閳?閸掓銆冩晶鐐插 user_override_count, 鐠囷附鍎忔晶鐐插 user_overrides
- `backend/internal/service/admin_service.go` 閳?閻劍鍩涢崚鐘绘珟閺冨墎楠囬懕鏃€绔婚悶?
- `backend/internal/handler/handler.go` 閳?AdminHandlers 婢х偛濮?UserModelPricing 鐎涙顔?
- `backend/internal/handler/wire.go` 閳?濞夈劌鍞介弬?handler
- `backend/internal/repository/wire.go` 閳?濞夈劌鍞介弬?repo
- `backend/internal/service/wire.go` 閳?濞夈劌鍞介弬?service
- `backend/internal/server/routes/admin.go` 閳?濞夈劌鍞介弬鎷岀熅閻?
- `frontend/src/api/admin/userModelPricing.ts` 閳?閸撳秶顏?API 鐎广垺鍩涚粩?
- `frontend/src/components/admin/user/UserModelPricingModal.vue` 閳?缁狅紕鎮婂Ο鈩冣偓浣诡攱
- `frontend/src/views/admin/UsersView.vue` 閳?閻劍鍩涢幙宥勭稊閼挎粌宕熸晶鐐插"濡€崇€风€规矮鐜?閸忋儱褰?
- `frontend/src/i18n/locales/en.ts` 閳?閸ヤ粙妾崠鏍ㄦ瀮濡?

**鐠囧瓨妲?*: 閺傛澘顤冮悽銊﹀煕缁狙勀侀崹瀣暰娴犵柉顩惄鏍у閼虫枻绱濋弨顖涘瘮缁狅紕鎮婇崨妯硅礋閻楃懓鐣鹃悽銊﹀煕閻ㄥ嫮澹掔€规碍膩閸ㄥ顔曠純顕嗙窗
1. 閻喎鐤勭拋陇鍨傛禒閿嬬壐鐟曞棛娲婇敍鍧昻put_price, output_price, cache_write_price, cache_read_price閿?
2. 鐏炴洜銇氭禒閿嬬壐鐟曞棛娲婇敍鍧塱splay_input_price, display_output_price, display_rate_multiplier, cache_transfer_ratio閿?

鐎瑰本鏆ｇ€规矮鐜导妯哄帥缁狙囨懠閿涙氨鏁ら幋?> 濞撶娀浜?> 閸忋劌鐪?> LiteLLM/Fallback閵嗗倷绗夎ぐ鍗炴惙閻滅増婀侀惃鍕弿鐏炩偓鐟曞棛娲婇妴浣圭闁捁顩惄鏍モ偓浣稿瀻缂佸嫬鈧秶宸奸崪宀€鏁ら幋宄板瀻缂佸嫬鈧秶宸奸張鍝勫煑閵?

## [2026-04-17] feat(billing): 閻劍鍩涚痪褍鐫嶇粈鍝勨偓宥囧芳 (User Display Rate Multiplier)

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/migrations/104_add_display_rate_multiplier.sql` 閳?閺傛澘顤?
- `backend/internal/service/user_group_rate.go` 閳?閹碘晛鐫?UserGroupRateEntry, GroupRateMultiplierInput, 閺傛澘顤?UserGroupRateData
- `backend/internal/repository/user_group_rate_repo.go` 閳?閺€顖涘瘮 display_rate_multiplier 鐠囪鍟?
- `backend/internal/handler/dto/display_pricing.go` 閳?閺傛澘顤?ApplyUserDisplayRate()
- `backend/internal/handler/usage_handler.go` 閳?娴ｈ法鏁ょ拋鏉跨秿鎼存梻鏁ら悽銊﹀煕缁狙冪潔缁€鍝勫綁閹?
- `backend/internal/handler/api_key_handler.go` 閳?/groups/rates 鏉╂柨娲栫仦鏇犮仛閸婂秶宸?
- `backend/internal/service/api_key_service.go` 閳?閺傛澘顤?GetUserGroupRatesFull()
- `backend/internal/service/admin_service.go` 閳?UpdateUser 閺€顖涘瘮 GroupRatesFull
- `backend/internal/handler/admin/user_handler.go` 閳?閺€顖涘瘮 group_rates_full
- `frontend/src/types/index.ts` 閳?閺傛澘顤?UserGroupRateData, group_display_rates
- `frontend/src/api/groups.ts` 閳?鏉╂柨娲?UserGroupRateData
- `frontend/src/views/user/KeysView.vue` 閳?GroupBadge 鐏炴洜銇氱仦鏇犮仛閸婂秶宸?
- `frontend/src/components/admin/user/UserAllowedGroupsModal.vue` 閳?鐏炴洜銇氶崐宥囧芳缂傛牞绶玌I
- `frontend/src/i18n/locales/{en,zh}.ts` 閳?閸ヤ粙妾崠?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ骸鍟跨粣渚€顥撻梽鈺嬬礉閺傛澘顤冪€涙顔岄崪灞炬煙濞夋洩绱濇稉宥勬叏閺€鍦箛閺堝鈧槒绶?

**閸欐ɑ娲跨拠锔藉剰**:
- 缁狅紕鎮婇崨妯哄讲娑撶儤鐦℃稉顏嗘暏閹村嘲婀В蹇庨嚋閸掑棛绮嶇拋鍓х枂閻欘剛鐝涢惃?鐏炴洜銇氶崐宥囧芳"閿涘瞼鏁ら幋椋庢箙閸掓澘鐫嶇粈鍝勨偓宥囧芳閼板矂娼惇鐔风杽鐠伮ゅ瀭閸婂秶宸?
- 鐏炴洜銇氶崐宥囧芳閻欘剛鐝涙禍搴ｆ埂鐎圭偘绗撶仦鐐测偓宥囧芳閿涘苯宓嗘担璺ㄦ暏閹磋渹濞囬悽銊ュ瀻缂佸嫰绮拋銈呪偓宥囧芳娑旂喎褰查崡鏇犲鐠佹儳鐫嶇粈鍝勨偓宥囧芳
- 娴ｈ法鏁ょ拋鏉跨秿闁俺绻冪紓鈺傛杹 token 閺佷即鍣虹€圭偟骞囬懛顏呭攪閿涙瓫ctual_cost 娑撳秴褰夐敍瀹紀tal_cost 鑴?display_rate 閳?actual_cost
- 娑撳孩膩閸ㄥ楠囩仦鏇犮仛娴犻攱鐗搁柧鎯х础閸欑姴濮為敍宀€鏁ら幋椋庨獓娴兼ê鍘涚痪褎娲挎?

## [2026-04-16] fix(pricing): 娣囶喖顦茬紓鏍帆閻劍鍩涚仦鏇犮仛鐠佸墽鐤嗛崥搴⒛侀崹瀣╃幆閺嶅吋甯撮崣?00闁挎瑨顕?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/repository/global_model_pricing_repo.go`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礉娣囶喖顦查懛顏勭箒瀵洖鍙嗛惃鍒g

**閸欐ɑ娲跨拠锔藉剰**:
- `GetByID` 閸?`GetByModel` 閺傝纭?SELECT 娴?18 閸掓ぞ绲?Scan 閸欘亝甯撮弨?14 娑擃亜鐡у▓?
- 濠曞繑甯€娴?`display_input_price`, `display_output_price`, `display_rate_multiplier`, `cache_transfer_ratio` 閸ユ稐閲滅€涙顔?
- 瑜?display 鐎涙顔屾稉?NULL 閺冭泛浼撶亸鏂剧瑝閹躲儵鏁婇敍宀冾啎缂冾喕绨￠棃?NULL 閸婄厧鎮楄箛鍛箛 500

## [2026-04-16] feat(deploy): 鐎瑰鍙忛柈銊ц閼存碍婀伴敍灞炬暜閹镐浇鍤滈崝銊ユ礀濠?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `deploy/update.sh`閿涘牊鏌婃晶鐑囩礆

**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礉閺傛澘顤冮悪顒傜彌閺傚洣娆?

**閸欐ɑ娲跨拠锔藉剰**:
- 閺嬪嫬缂撻崚棰佸閺?staging tag閿涘本妫梹婊冨剼閸︺劍鐎鐑樻埂闂傜繝绻氶幐浣风瑝閸?
- 娣囨繄鏆€娑撳﹣绔存稉顏嗗閺堫剟鏆呴崓?(`sub2api-custom:prev`) 閻劋绨崡铏閸ョ偞绮?
- 闁劎璁查崥?health check 婢惰精瑙﹂懛顏勫З閸ョ偞绮撮崚鏉垮娑撯偓閻楀牊婀?
- 閺€顖涘瘮 `--rollback` 閹靛濮╅崶鐐寸泊
- 閸忋劏绻冪粙瀣）韫囨顔囪ぐ鏇炲煂 `/opt/sub2api/deploy.log`

## [2026-04-16] feat(branding): 閺傛澘顤冨楦跨殶鐎瑰鍙忔稉搴Ｇ旂€规碍鐨电拹銊ф畱娑撱倗澧楃划妤冨Ш閸ョ偓鐖?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/public/logo-gateway-fortress.svg`
- `frontend/public/logo-gateway-vault.svg`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礉娴犲懏鏌婃晶鐐烘饯閹礁鎼ч悧宀冪カ濠?

**閸欐ɑ娲跨拠锔藉剰**:
- 閺傛澘顤?`logo-gateway-fortress.svg`閿涘本鏌熼崥鎴濅焊閳ユ粍濮㈤惄?+ 閸╄櫣顢呯拋鐐煢閸€崇€块垾婵撶礉閻劌甯ら柌宥咁嚠缁夋壆绮ㄩ弸鍕繁閸栨牕鐣ㄩ崗銊ｂ偓浣呵旈崶鎭掆偓浣稿讲娣嚶ょ閻ㄥ嫮顑囨稉鈧崡鎷岃杽
- 閺傛澘顤?`logo-gateway-vault.svg`閿涘本鏌熼崥鎴濅焊閳ユ粓鍣炬惔鎾绘， + 缁嬪啿鐣炬稉顓熺亼閳ユ繐绱濋柅姘崇箖閺囧鐭栭惃鍕，濡楀棗鎷伴柨浣藉П鐠囶厺绠熺粣浣稿毉閸欘垶娼幍妯碱吀娑撳氦绁禍褍鐣ㄩ崗銊﹀妳
- 娑撱倗澧楅柈鑺ョ槷閸撳秹娼伴惃鍕煙濡楀牊娲挎径褑鍎愰妴浣规纯閸樻岸鍣搁敍灞肩喘閸忓牊婀囬崝鈾€鈧粌鐣ㄩ崗銊ｂ偓浣呵旂€规哎鈧線娼拫鎵佲偓婵堟畱閸濅胶澧濊箛鍐╂

## [2026-04-16] feat(branding): 閺傛澘顤冩稉銈囧閸樼喎鍨遍崶鐐垼婢跺洭鈧鏌熷?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/public/logo-gateway-orbit.svg`
- `frontend/public/logo-gateway-portal.svg`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礉娴犲懏鏌婃晶鐐烘饯閹礁鎼ч悧宀冪カ濠?

**閸欐ɑ娲跨拠锔藉剰**:
- 閺傛澘顤?`logo-gateway-orbit.svg`閿涘本鏌熼崥鎴濅焊閳ユ粎缍夌紒婊€鑵戦弸?/ 閹貉冨煑闂?/ 鐠嬪啫瀹抽懞鍌滃仯閳ユ繐绱濋弽绋跨妇閺勵垳骞嗚ぐ銏＄湽閼辨矮绗屾稉澶庣熅閹恒儱鍙?
- 閺傛澘顤?`logo-gateway-portal.svg`閿涘本鏌熼崥鎴濅焊閳ユ粌鍙嗛崣?/ 闁岸浜?/ 缂冩垵鍙ч梻銊﹀煕閳ユ繐绱濋弽绋跨妇閺勵垰鍨庣仦鍌炴，濡楀棔绗岄崥鎴濈妇閼辨艾鎮?
- 娑撱倗澧楅柈钘夊煝閹板繘浼╁鈧稉濠冪埗 `sub2api` 鐢瓕顫嗛惃鍕摟濮ｅ秴瀵查崙鐘辩秿闁姴鐎烽敍灞肩喘閸忓牆缂撶粩瀣╃稑閼奉亜绻侀惃鍕惂閻楀矁鐦戦崚?

## [2026-04-16] feat(branding): 閸ョ偓鐖ｉ柌宥嗙€稉鍝勫斧閸掓稓缍夐崗鍏呰厬閺嬨垽鈧姴鐎烽敍宀勪缉瀵偓娑撳﹥鐖剁憴鍡氼潕閸忓疇浠?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/public/logo-gateway-mark.svg`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礉娴犲懏娲块弬鎷屽殰鐎规矮绠熼崫浣哄鐠у嫭绨?

**閸欐ɑ娲跨拠锔藉剰**:
- 鐏忓棔绗傛稉鈧悧鍫濅焊閸戠姳缍嶇€涙鐦濋惃鍕禈閺嶅洭鍣搁弸鍕礋閳ユ粌鍙氭潏鐟拌埌缂冩垵鍙ч弽绋跨妇 + 娑撳鐭惧Ч鍥粵閼哄倻鍋ｉ垾婵堟畱閸樼喎鍨辩粭锕€褰块敍宀勪缉閸忓秷顔€娴滈缚浠堥幆鍐插煂娑撳﹥鐖?`sub2api` 姒涙顓荤憴鍡氼潕
- 娣囨繄鏆€瑜版挸澧犵粩娆戝仯閼奉亜绻侀惃鍕箒閽冩繂绨抽崪宀勬綒缂佸じ瀵岄懝璇х礉娴犮儰绻氱拠浣告嫲閻滅増婀佹＃鏍€夐妴浣告倵閸欑増瀵滈柦顔衡偓浣稿幢閻楀洭鐝禍顔荤矝閻掑墎绮烘稉鈧?
- 閺傛澘娴橀弽鍥ㄦ纯瀵缚鐨熼垾婊嗕粵閸氬牄鈧浇鐨熸惔锔衡偓浣稿瀻閸欐垟鈧繄娈戞禍褍鎼х拠顓濈疅閿涘矁鈧奔绗夐弰顖氱摟濮ｅ秹鈧姴鐎烽敍灞肩┒娴滃骸鎮楃紒顓犲缁斿鎼ч悧灞藉

## [2026-04-16] feat(branding): 閺傛澘顤冪拹鏉戞値 AI 缂冩垵鍙х拠顓濈疅閻?SVG 閸ョ偓鐖ｉ弬瑙勵攳

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/public/logo-gateway-mark.svg`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礉娴犲懏鏌婃晶鐐烘饯閹礁鎼ч悧宀冪カ濠ф劧绱濇稉宥嗘禌閹诡澀绗傚〒鎼佺帛鐠併倖鏋冩禒?

**閸欐ɑ娲跨拠锔藉剰**:
- 閺傛澘顤冩稉鈧悧鍫㈡暏娴?Sub2API 閻ㄥ嫬鎼ч悧灞芥禈閺嶅洦鏌熷鍫礉瀵ゅ墎鐢婚悳鐗堟箒濞ｈ精鎽戞惔鏇氱瑢闂堟帞璞㈤崚鎷屾憫閼瑰弶绗庨崣妯兼畱鐟欏棜顫庣拠顓♀枅閿涘矂浼╅崗宥勭瑢妫ｆ牠銆夐崪灞芥倵閸欐壆娈戞稉鏄忓娴ｆ挾閮撮崜鑼额棁
- 閸ョ偓鐖ｇ拠顓濈疅娴犲骸宕熺痪顖氬殤娴ｆ洖鐡уВ宥堢箻娑撯偓濮濄儲鏁归弫娑樺煂閳ユ粎缍夐崗?/ 鐠侯垳鏁?/ 閼辨艾鎮庨崚鍡楀絺閳ユ繐绱濋柅姘崇箖娑擃厽鐏戝蹇撳殤娴ｆ洑瀵岃ぐ銏犳嫲閼哄倻鍋ｇ粩顖滃仯瀵搫瀵?API Gateway 娴溠冩惂鐠囧棗鍩嗘惔?
- 鐠у嫭绨担璺ㄦ暏 SVG 閻垽鍣洪弽鐓庣础閿涘奔绌舵禍搴℃倵缂侇厼婀崥搴″酱 `site_logo`閵嗕胶鐝悙褰掝浕妞ょ偣鈧公avicon 鐎电厧鍤崪宀冩儉闁库偓閻椻晜鏋℃稉顓烆槻閻?

## [2026-04-16] fix: AI Credits 鐞氼偂澶嶉弮鍫曟濞翠浇顕ら弽鍥﹁礋缁夘垰鍨庨懓妤€鏁栫€佃壈鍤х拹锕€褰块柨浣哥暰 5 鐏忓繑妞?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/service/antigravity_credits_overages.go`
- `backend/internal/service/antigravity_credits_overages_test.go`

**娑撳﹥鐖堕崗鐓庮啇閹?*: 閺冪姴鍟跨粣渚婄礄娴滃苯绱戦弬鏉款杻閸旂喕鍏橀敍?

**閸欐ɑ娲跨拠锔藉剰**:
- `shouldMarkCreditsExhausted` 娑?`"resource has been exhausted"` 閸忔娊鏁拠宥呭爱闁板秳绨?Google API 閹碘偓閺?429 閸濆秴绨查敍鍫濆瘶閹奉兛澶嶉弮?RPM 闂勬劖绁﹂敍澶涚礉鐎佃壈鍤?credits 鐞氼偊鏁婄拠顖涚垼鐠侀璐熼懓妤€鏁栭妴鍌欑閺冿箒顕ら弽鍥ц埌閹存劘鍤滈柨渚婄礄`isCreditsExhausted` 闂冪粯顒涢柌宥堢槸 閳?`clearCreditsExhausted` 濮橀晲绗夌憴锕€褰傞敍澶涚礉鐠愶箑褰跨悮顐︽敚鐎规艾鐣弫?5 鐏忓繑妞傞妴?
- 缁夊娅庢潻鍥︾艾鐎硅姤纭鹃惃?`"resource has been exhausted"` 閸忔娊鏁拠宥忕礉閸忔湹缍戦崗鎶芥暛鐠囧稄绱檂insufficient credit`閵嗕梗credit exhausted` 缁涘绱氬鑼跺喕婢剁喓绨跨涵?
- `shouldMarkCreditsExhausted` 閹烘帡娅?429 閻樿埖鈧胶鐖滈敍灞煎閺冨爼妾哄ù浣风瑝鎼存柨鍨界€规矮璐熺粔顖氬瀻閼版鏁?

---

## [2026-04-16] feat(admin): 濡€崇€风€规矮鐜い闈涙値楠炶埖妲х亸?CRUD + 濡€崇€峰ù瀣槸閿涘苯鍨归梽銈嗘＋ mapping tab

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `frontend/src/views/admin/ModelConfigView.vue`閿?*婢堆冪畽缁墽鐣?*閿涙艾鍨归梽?mapping tab 閸忋劑鍎村Ο鈩冩緲閸?script閿涘苯褰ф穱婵堟殌 pricing 閸?rate 娑撱倓閲?tab閿?
- `frontend/src/components/admin/model-pricing/ModelMappingInlinePopover.vue`閿?*閺傛澘缂?*閿?
- `frontend/src/components/admin/model-pricing/ModelTestDialog.vue`閿?*閺傛澘缂?*閿?
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`閿涘牐銆冮弽濂搞€婇柈銊ュ"+ 濞ｈ濮為弰鐘茬殸"閹稿鎸抽敍娑滎攽閹垮秳缍旈崚妤€濮?缂傛牞绶弰鐘茬殸"閸?濞村鐦?娑撱倓閲滈弶鈥叉閺勫墽銇氶幐澶愭尦閿涙稒甯撮崗銉よ⒈娑擃亝鏌婄紒鍕閿?
- `frontend/src/i18n/locales/zh.ts` & `en.ts`閿涘牊鏌婃晶?~20 閺?key閿涙碍妲х亸?CRUD + 濡€崇€峰ù瀣槸閿?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈹库偓鍌氬弿闁劑娉︽稉顓炴躬娴滃苯绱戦悪顒佹箒閻ㄥ嫭膩閸ㄥ鍘ょ純顔炬櫕闂堫潿鈧精PI 婢跺秶鏁ら悳鐗堟箒閻?`adminAPI.accounts.getAntigravityDefaultModelMapping` / `updateAntigravityDefaultModelMapping`閿涘牅绗傚〒绋垮嚒閺堝绱氶敍灞间簰閸?SSE 濞村鐦幒銉ュ經 `POST /admin/accounts/:id/test`閵?

**閼冲本娅?*:

娑撳﹣绔存潪顔藉Ω濡€崇€风€规矮鐜い鐢稿櫢閺嬪嫪璐?閸欏苯鍨Ο鈥崇€烽崥?+ 鐠伮ゅ瀭濡€崇础"妞嬪孩鐗搁崥搴礉閻劍鍩涢崣宥夘洯閿?閺勭姴鐨犻崗宕囬兇閸滃矁顓哥拹瑙勀佸蹇庣瑝閼虫垝鎱ㄩ弨?閵嗗倻绮＄拋銊啈閿?
- 鐠伮ゅ瀭濡€崇础娣囨繄鏆€閸欘亣顕伴敍鍫熸拱闊偅妲告禒搴㈡Ё鐏忓嫬鍙х化缁樺腹閺傤厾娈戦弽鍥╊劮閿涘奔绗夐弰顖氬讲闁板秶鐤嗙仦鐐粹偓褝绱?
- 閺勭姴鐨犻崗宕囬兇**鎼存棁顕?*閼宠姤鏁奸敍灞肩瑬閸愬啿鐣鹃幎濞库偓灞灸侀崹瀣Ё鐏忓嫨鈧秶瀚粩?tab 閸氬牆鑻熼崚鏉跨暰娴犵兘銆夐敍鍫濇倵缂侇厽绗庢潻娑樺灩闂勩倗瀚粩?tab閿?
- 濡€崇€峰ù瀣槸閸旂喕鍏橀幖顒€鍩岀€规矮鐜い浣冾攽閹垮秳缍旈柌灞戒粵閹存劕鐨幐澶愭尦

閺傜懓鎮滅涵顔肩暰閸氬孩婀版潪顔肩杽閺傝棄浜ゆ惔鏇犳畱閸氬牆鑻熼妴?

**閸欐ɑ娲跨拠锔藉剰**:

1. **閺傛澘缂?`ModelMappingInlinePopover.vue`**閿涘瀫210 鐞涘矉绱氶敍?
   - 娑撳顫掗幙宥勭稊閿涙碍鏌婃晶鐐存Ё鐏忓嫸绱檓ode="add"閿? 娣囶喗鏁奸弰鐘茬殸閿涘潰ode="edit"閿? 閸掔娀娅庨弰鐘茬殸閿涘潒dit 濡€崇础鎼存洟鍎撮幐澶愭尦閿?
   - 娑撱倓閲?input閿涙俺顕Ч鍌浤侀崹瀣倳 + 娑撳﹥鐖跺Ο鈥崇€烽崥宥忕礉娑撳鏌熺敮锔跨鐞涘瞼浼嗙€涙褰佺粈?閸氬苯鎮曢弰鐘茬殸閻╁瓨甯存繅顐ゆ祲閸氬苯鈧?
   - 鐠ф壆骞囬張?API閿涙瓪GET /admin/accounts/antigravity/default-model-mapping` 鐠囪鍙忕悰?閳?鐏炩偓闁劋鎱ㄩ弨?閳?`PUT` 閺佺銆冮崘娆忔礀
   - 閺€鐟版倳閸︾儤娅欓敍鍧媎it 閺冭埖濡?from 娑旂喐鏁兼禍鍡礆濮濓絿鈥樻径鍕倞閿涙艾鍘?delete 閺?key 閸?set 閺?key/value
   - Teleport + fixed 鐎规矮缍呴敍鍫濆棘閼?ModelPricingInlinePopover 鐠佹崘顓搁敍澶涚礉閼奉亜濮╅柆鍨磻鐟欏棗褰涙潏鍦櫕
   - Enter 娣囨繂鐡ㄩ妴浣哄鐎?inline 闁挎瑨顕ら崣宥夘洯

2. **閺傛澘缂?`ModelTestDialog.vue`**閿涘瀫160 鐞涘矉绱氶敍?
   - 娴犲骸甯?`ModelConfigView.vue` 閻?mapping tab 閸欏厖鏅跺ù瀣槸闂堛垺婢橀幖顒冪讣閿涘矂鈧槒绶崺鐑樻拱娣囨繄鏆€
   - 閸ュ搫鐣炬导鐘插弳 `model` prop閿涘牅绮犵悰灞惧瘻闁筋喛袝閸欐垶妞傞柨浣哥暰閿涘绱濇稉宥呭晙闂団偓鐟曚焦膩閸ㄥ绗呴幏?
   - 閸愬懘鍎撮崝鐘烘祰 Antigravity 鐠愶箑褰块崚妤勩€冮敍鍫滅矌 active / schedulable / 閺?error 閻ㄥ嫸绱?
   - SSE 濞翠礁绱″☉鍫ｅ瀭 `/api/v1/admin/accounts/:id/test`閿涘矁袙閺?`test_start / content / test_complete / error` 娴滃娆㈢猾璇茬€?
   - `testRunning` 閺冨爼妯嗗銏犲彠闂?dialog 闁灝鍘ら悽銊﹀煕鐠囶垱鎼锋担?

3. **`ModelPricingTab.vue` 閹恒儱鍙?*閿?
   - 鐞涖劍鐗告い鍫曞劥閿涘牊鎮崇槐銏ｎ攽閸欏厖鏅堕妴浣稿煕閺傜増瀵滈柦顔间箯娓氀嶇礆閺傛澘顤?+ 濞ｈ濮為弰鐘茬殸"閹稿鎸抽敍宀勬晪閻?ref 閻劋绨?popover 鐎规矮缍?
   - 鐞涘本鎼锋担婊冨灙娑撳瀵滈柦顕嗙礄閺夆€叉閺勫墽銇氶敍澶涚窗
     - 閳?**缂傛牞绶弰鐘茬殸**閿涙矮绮?`canEditMapping` 鐞涘矉绱檋int type=requested_only 閹?requested_equals_upstream閿?
     - 閳?**濞村鐦Ο鈥崇€?*閿涙瓪canTest` 鐞涘矉绱欓張?billing_basis_hint 閹?provider=antigravity閿?
     - 閴?閺屻儳婀呯拠锔藉剰 / 閸掓稑缂撶€规矮鐜敍姘閺堝顢戦敍鍫滅箽閹镐礁甯悰灞艰礋閿?
   - `handleMappingSaved` 娴滃娆㈤崶鐐剁殶鐠嬪啰鏁?`loadData` 閺佺銆冮崚閿嬫煀閿涘牊妲х亸鍕綁閸栨牕濂栭崫宥嗗閺堝绐橀弽鍥ф嫲 related_models閿?
   - `RowDisplay` 閹恒儱褰涢幍?`canEditMapping` / `canTest` 鐎涙顔岄敍灞芥躬 `displayRows` computed 闁插本瀵?hint 缁鐎烽幒銊ヮ嚤

4. **閸掔娀娅庨弮?mapping tab**閿?
   - `ModelConfigView.vue` 娴?350 鐞涘瞼绨跨粻鈧崚?40 鐞涘矉绱濋崣顏冪箽閻?pricing 閸?rate 娑撱倓閲?tab + 韫囧懓顩﹂惃?AppLayout 婢?
   - 閸樺棗褰?URL 閸忕厧顔愰敍姝?tab=mapping` 鐞氼偉鍤滈崝銊ユ礀闁偓閸?pricing
   - 閺?i18n key閿涘潉admin.modelConfig.antigravityMapping` / `testTitle` 缁涘绱氶弳鍌涙弓濞撳懐鎮婇敍宀€鏆€閻偓娑撳秶鏁ゆ稉宥呭閸濆秷顢戞稉鐚寸礉閸氬海鐢婚崣顖炴娑撳﹥鐖堕崥灞绢劄娑撯偓鐠ч攱绔婚梽?

**妤犲矁鐦?*:
- `pnpm run typecheck` 闁俺绻?
- 閸撳秶顏?dev server 閻戭參鍣告潪钘夋倵閹靛绁村ù浣衡柤閿?
  - 閻?+ 濞ｈ濮為弰鐘茬殸" 閳?婵?from/to 閳?娣囨繂鐡?閳?鐞涖劍鐗?reload 閺傜増妲х亸鍕毉閻?
  - 閻愯鐓囩悰?缂傛牞绶弰鐘茬殸" 閳?閺€閫涚瑐濞撶鎮?閳?娣囨繂鐡?閳?閸掓銆冮弴瀛樻煀閿涙稑绐橀弽鍥ф嫲 +N 鐠佲剝鏆熷锝団€橀懕鏂垮З
  - 缂傛牞绶?popover 鎼存洟鍎撮悙?閸掔娀娅庨弰鐘茬殸" 閳?绾喛顓?閳?鐠囥儲妲х亸鍕矤鐞涖劋鑵戝☉鍫濄亼
  - 閻愯鐓囩悰?濞村鐦? 閳?dialog 瀵懓鍤?閳?闁澶勯崣?閳?閸欐垿鈧?閳?濞翠礁绱℃潏鎾冲毉濮濓絿鈥橀弰鍓с仛
  - 閺?mapping tab 瑜拌绨冲☉鍫濄亼閿涘苯褰ч崜?Pricing 閸?Rate Multipliers 娑撱倓閲?tab

**瀹歌尙鐓￠梽鎰煑 / 閺堫亝娼垫潻顓濆敩**:
- `upstream_only` 缁鐎烽惃鍕攽閿涘牅绮庢担婊€璐熼弰鐘茬殸 value 鐎涙ê婀妴浣规￥閸氬苯鎮曢懛顏呮Ё鐏忓嫸绱氭稉宥嗗絹娓?缂傛牞绶弰鐘茬殸"閹稿鎸抽敍娑樼秼閸?Antigravity 姒涙顓婚弰鐘茬殸闁插本顒濈猾璇茬€锋稉铏光敄閿涘牊澧嶉張?value 闁姤婀侀崥灞芥倳閼奉亝妲х亸鍕剁礆閿涘苯鐤勯梽鍛￥瑜板崬鎼?
- 鐠愶箑褰跨痪?`credentials.model_mapping` 閻ㄥ嫮顓搁悶鍡曠矝鐠ф澘甯拹锕€褰跨紓鏍帆閻ｅ矂娼伴敍灞炬拱濞嗏剝鐥呴張澶婃値楠炶绱欓悽銊﹀煕閺勫海鈥橀崣顏囶洣濮瑰倸閽╅崣鎵獓閺勭姴鐨犵粻锛勬倞閸氬牆鍙嗛敍?
- 閺?`admin.modelConfig.*` 娑撳娈?mapping 閻╃鍙?i18n key 閺嗗倻鏆€閺堫亝绔婚悶?

## [2026-04-15] feat(admin): 濡€崇€风€规矮鐜い鍨箒鎼达缚绱崠鏍电礄娑撳鍨濈痪?tab / 閸愬懓浠?popover / 瀵ら缚顔呮禒?/ billing hint閿?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/service/global_model_pricing_service.go`閿涘湣odelPricingListItem/Detail 閸旂姴鐡у▓鐐光偓涔籾ggestPricing閵嗕巩sAntigravityStubModel閵嗕竸ntigravity 閸欏秵澹?mapping value閿?
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`閿涘牅绗呴崚鎺斿殠 tab 缁涙盯鈧娅掗妴涔mputePriceDelta 濞戙劏绌奸弻鎾瑰閵嗕焦濮岄崣?banner閵嗕巩nline popover 閹恒儱鍙嗛妴浣筋攽缁狙冪獦閺嶅浄绱?
- `frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue`閿涘牆缂撶拋顔荤幆鐏炴洜銇?+ 鎼存梻鏁ら幐澶愭尦閿?
- `frontend/src/components/admin/model-pricing/ModelPricingInlinePopover.vue`閿涘牊鏌婂鐚寸礉308 鐞涘矉绱?
- `frontend/src/api/admin/modelPricing.ts`閿涘牏琚崹瀣⒖閸忓拑绱皊uggested_prices/suggested_from/billing_basis_hint閿?
- `frontend/src/i18n/locales/zh.ts` & `en.ts`閿涘瀫20 閺夆剝鏌?key閿?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娑擃厾鐡戦妴鍌涘閺堝鏁奸崝銊╂肠娑擃厼婀禍灞界磻閻欘剚婀侀惃鍕┾偓灞灸侀崹瀣暰娴犳灚鈧秶顓搁悶鍡欐櫕闂堫澁绱?026-04-12 閺傛澘顤冮惃?ModelPricingTab 閸滃瞼娴夐崗铏箛閸斺剝鏌熷▔鏇氱瑐濞撻晲绗夌€涙ê婀敍澶涚礉娑撳簼绗傚〒闀愬瘜缁炬寧妫ら崘鑼崐閵嗕敬lobalModelPricing 鐎圭偘缍嬪▽鈩冩箒閺傛澘顤?DB 鐎涙顔岄敍宀勬祩 migration閵嗗倿娓剁憰浣烘殌閹板繒娈戦弰顖欑瑐濞撳憡婀弶銉ㄥ缂?`ModelPricingListItem` / `ModelPricingDetail` 婢х偛濮炵€涙顔岄弮鎯邦洣闁灝鍘ら崪灞炬拱濞嗏剝鏌婃晶鐐茬摟濞堥潧鎳￠崥宥呭暱缁愪降鈧?

**閼冲本娅?*:

濮濄倕澧犻妴灞灸侀崹瀣帳缂?閳?濡€崇€风€规矮鐜妴宄奱b 瀹歌尪鍏樺锝団€樼仦鏇犮仛 Gemini/Antigravity 缁涙盯鈧绮ㄩ弸婊愮礉娴ｅ棛顓搁悶鍡楁喅閻喐顒滄担璺ㄦ暏鐠囥儵銆夐棃銏㈩吀閻炲棗鍙忕仦鈧€规矮鐜弮鎯扮箷閺堝娲撴稉顏嗘閻愮櫢绱?
1. 鐞涖劍鐗搁柌灞剧槨娑擃亙鐜弽鐓庣摟濞堥潧鍩屾惔鏇熸降閼?LiteLLM 鏉╂ɑ妲哥悮?global/channel 鐟曞棛娲婇惇瀣╃瑝濞撳拑绱濋崣顏呮箒 input/output 閸掓婀佺粻鈧崡鏇㈩杹閼硅绱漜ache 閸掓鐣崗銊︾梾閺?
2. 閺夈儲绨粵娑⑩偓?Tab 妞ゅ搫绨弰顖樷偓灞藉弿闁?/ 閸忋劌鐪憰鍡欐磰 / 濞撶娀浜剧憰鍡欐磰 / 娴?LiteLLM閵嗗稄绱濇担鍡楃杽闂勫懓顓哥拹閫涚喘閸忓牏楠囬弰?`Channel > Global > LiteLLM`閿涘矂銆庢惔蹇撳冀娴滃棔绗栨い鐢告桨濞屸剝婀佹禒璁崇秿娴ｅ秶鐤嗙拠瀛樻鏉╂瑤閲滄导妯哄帥缁?
3. 閺€閫涚娑擃亝膩閸ㄥ娈?input 娴犵柉顩﹂悙褰掓惍缁楁柨娴橀弽鍥ц剨閸忋劌鐫?dialog 閳?缂堢粯澹?閳?閺€?閳?娣囨繂鐡?閳?閸忔娊妫撮敍灞筋嚠妤傛﹢顣剁拫鍐ㄥ棘閸︾儤娅欐径顏堝櫢
4. 娑撳﹣绔存潪顔克夐惃?Antigravity 娑撴挻婀?stub閿涘潉gemini-3-pro-high`閵嗕梗gpt-oss-120b-medium`閵嗕梗tab_flash_lite_preview` 缁?8+ 娑擃亷绱氭稉鈧幒?`-`閿涘瞼顓搁悶鍡楁喅閺冪姳绮犳稉瀣閿涙稐绗栨潻娆庣昂濡€崇€峰☉澶婂挤鐠愶箑褰跨痪褎妲х亸鍕剁礉娑撳孩绗柆鎾崇暰娴犻娈?`billing_model_source` 閺堝搫鍩楀铏规祲閸?

**鐠佹崘顓搁崘宕囩摜**閿?

缂佸繗绻?Explore+Plan 鐎涙劒鍞崚鍡樼€介敍灞藉彠闁款喖褰傞悳甯窗`model_pricing_resolver.go` 閻?`resolveBasePricing(model)` 閺€璺哄煂閻?`model` 瀹歌尙绮￠弰顖濐潶 `BillingModelSource` 鏉╁洦鎶ら惃?`billingModel`閿涘苯鍙忕仦鈧憰鍡欐磰閻ㄥ嫭鐓＄悰?key **婢垛晝鍔х捄鐔兼濮ｅ繋閲滅拠閿嬬湴閹碘偓鐏炵偞绗柆鎾舵畱 billing_model_source**閵嗗倷绡冪亸杈ㄦЦ鐠囧閮寸紒鐔峰嚒鐎圭偠宸濇稉鈧懛杈剧礉缂傝櫣娈戦崣顏呮Ц**鐠佲晝顓搁悶鍡楁喅閻鍩屾潻娆庨嚋闂呮劕绱＄悰灞艰礋**閵嗗倸娲滃銈嗘拱鏉烆噣鈧?*閺傝顢?A**閿涘牆澧犵粩顖涙缁€娲瀵繗顢戞稉鐚寸礆閿涘奔绗夐崝鐘叉倵缁旑垰鐡у▓纰夌礉闂?migration閵?

**閸欐ɑ娲跨拠锔藉剰**:

1. **缁涙盯鈧銆庢惔?+ 鐏炲倻楠囩拠瀛樻**閿涙ourceTabs 妞ゅ搫绨弨閫涜礋 `閸忋劑鍎?/ 閺堝绗柆鎾诡洬閻?/ 閺堝鍙忕仦鈧憰鍡欐磰 / 娴?LiteLLM`閿涙奔ource label 閸欏厖鏅堕崝?閳?閸ョ偓鐖ｉ敍瀹ver 閺勫墽銇?娴兼ê鍘涚痪褝绱板〒鐘讳壕 > 閸忋劌鐪?> LiteLLM"tooltip閵?
2. **瀹割喖绱撴妯瑰瘨**閿涙瓪formatPrice` 闁插秵鐎稉?`computePriceDelta`閿涘矁绻戦崶?`{text, className, tooltip}`閵嗗倷浜?LiteLLM 娑撳搫鐔€閸戝棜顓哥粻妤冩祲鐎靛湱娅ㄩ崚鍡樼槷瀹割喖绱撻敍灞?% 閸愬懓顫嬫担婊呯搼閸氬被鈧倹瀹氭禒?`text-rose-600`閵嗕浇绌兼禒?`text-emerald-600`閵嗕胶鐡戦崥灞惧灗閺冪姴鐔€閸?`text-primary-600`閵嗕胶鍑?LiteLLM 姒涙顓婚悘鑸偓淇che_write/cache_read 娑撯偓楠炶泛鎯庨悽銊ｂ偓鍌涚槨娑擃亝鏆熺€涙ぞ绗?`title` 閺勫墽銇?LiteLLM 閸╁搫鍣?$X 璺?瀹割喖绱?+Y%"閵?
3. **閹舵ê褰?banner閿涘牐顓哥拹鐟扮唨閸戝棜顕╅弰搴礆**閿涙tats 閸椻€茬瑓閺傜懓濮?`<details>` 閹舵ê褰旈崸妤嬬礉姒涙顓婚弨鎯版崳閵嗗倸鐫嶅鈧憴锝夊櫞 requested/upstream/channel_mapped 娑撳顫掗崺鍝勫櫙閸氼偂绠?+ "濞撶娀浜炬妯款吇 channel_mapped閿涘本妫ゅ〒鐘讳壕鐠侯垰绶炴妯款吇 requested"閵?
4. **閸愬懓浠?popover 缂傛牞绶?*閿?
   - 閺傛澘缂?`ModelPricingInlinePopover.vue`閿涙瓖eleport 閸?body 闁灝鍘ょ悰銊︾壐 overflow 鐟佷礁鍨忛敍娌爄xed 鐎规矮缍呴懛顏勫З闁灝绱戠憴鍡楀經鏉堝湱鏅敍鍫滅瑓閺?閳?娑撳﹥鏌熼妴浣稿礁娓?閳?瀹革箑顕鎰剁礆閿? 娑擃亝鐗宠箛鍐х幆閺嶇厧鐡у▓?+ enabled 婢跺秹鈧顢?+ 娣囨繂鐡?閸掔娀娅?鐠囷妇绮忕拋鍓х枂 3 閹稿鎸抽敍娑欑槨娑擃亜鐡у▓闈涚敨 LiteLLM 閸╁搫鍣?placeholder閿涙宝nter 閹绘劒姘?
   - 鐞涖劍鐗?4 娑擃亙鐜弽?`<td>` 閸?`@click` 鐟欙箑褰?popover + `cursor-pointer hover:bg-primary-50/50`
   - 娣囨繂鐡ㄩ弮?*娑撳秵鏆ｇ悰?reload**閿涘瞼鍩楃紒鍕 `handleInlineSaved` 鐏忓崬婀撮弴鎸庡床 items 楠炶泛妯婇柌蹇旀纯閺?stats.global_override_count
   - Popover 娣囨繄鏆€閸?override 閻?provider/notes/image_output_price/per_request_price 缁涘鐡у▓纰夌礄PATCH 瀹割噣鍣洪敍澶涚礉闁灝鍘ゅ〒鍛存祩
   - `< lg` 閺傤厾鍋?`window.matchMedia('(max-width: 1023px)')` 閸ョ偤鈧偓閸掓澘甯?dialog閿涙硞tub 濡€崇€烽敍鍫ユ付鐟曚線鍘?provider/notes/瀵ら缚顔呮禒鍑ょ礆娑旂喎娲栭柅鈧崚?dialog
   - 缁涙盯鈧娅掓稉瀣煙閸旂姷浼嗛懝鎻掔毈鐎涙褰佺粈?閻愮懓鍤悰銊︾壐娑擃厾娈戞禒閿嬬壐閺佹澘鐡ч崣顖氭彥闁喓绱潏?
5. **Antigravity stub 閸欘垶鍘ょ純?+ 瀵ら缚顔呮禒?*閿?
   - 鐞涖劍鐗搁柧鍛應閸ョ偓鐖ｇ€?stub 鐞?tooltip 閸掑洦宕叉稉?閸掓稑缂撶€规矮鐜?
   - 閸氬海顏?`ModelPricingDetail` 閸?`SuggestedPrices` / `SuggestedFrom` 鐎涙顔岄敍灞肩矌閸︺劍妫?LiteLLM + 閺?global_override 閺冭泛锝為崗?
   - 閺?`suggestPricing` 閺傝纭堕幐澶変簰娑撳鎽奸崠褰掑帳閿涙碍妯夊蹇旀Ё鐏忓嫯銆冮敍鍧則ab_flash_lite_preview 閳?gemini-2.5-flash-lite`閵嗕梗gpt-oss-120b-medium 閳?gpt-4o-mini`閿涘鍟?閸撱儳顬?`-high/-low/-medium` 濡楋絼缍呴崥搴ｇ磻 閳?閸撱儳顬?`-thinking` 閳?Gemini 閻楀牊婀伴梽宥囬獓閿?.x 閳?2.5閿?
   - `ModelPricingDetailDialog.vue` 閸?Global Override section 妞ゅ爼鍎寸仦鏇犮仛"棣冩寱 瀵ら缚顔呮禒鍑ょ礄閺夈儴鍤?xxx閿涘?鎼存梻鏁?鐞涘矉绱濋悙鐟板毊鎼存梻鏁ら幎濠傗偓鐓庯綖閸?form閿涘牓娓剁粻锛勬倞閸涙鈥樼拋銈勭箽鐎涙﹫绱濇稉宥堝殰閸斻劌鍙嗘惔鎿勭礆
   - 娣囶喖顦叉稉鈧稉顏勫娴ｆ粎鏁?bug閿涙瓪pricingService.GetModelPricing` 鐢附膩缁﹤灏柊宥忕礉鐎?Antigravity 娑撴挻婀?stub 娴兼岸鏁婄拠顖氬爱闁板秴鍩屾稉宥囨祲閸忓磭娈?LiteLLM 濡€崇€锋禒閿嬬壐閵嗗倹鏌婃晶?`isAntigravityStubModel` 濡偓濞村绱檓odel 閸?Antigravity mapping keys 娴ｅ棔绗夐崷?LiteLLM 缁墽鈥樺Ο鈥崇€烽崚妤勩€冮敍澶涚礉鐠囷附鍎忛幒銉ュ經鐎?stub 鐠哄疇绻?LiteLLM 楠炴儼铔?suggestPricing閿涘奔绗岄崚妤勩€冮幒銉ュ經閻ㄥ嫮绨跨涵顔煎爱闁板秷顕㈡稊澶夌閼?
6. **閸欏苯鍨Ο鈥崇€烽崥?+ 鐠伮ゅ瀭濡€崇础閸?*閿涘牐鍑禒锝堢箖 badge 閺傝顢嶉崥搴ｆ畱閺堚偓缂佸牆鑸伴幀渚婄礆閿?
   閻劍鍩涢崣宥夘洯鐏?badge 婢额亝濞婄挒鈽呯礉娴滃孩妲搁幎濠佷繆閹垱褰侀崡鍥﹁礋濮濓絽绱＄悰銊︾壐閸掓せ鈧柡鈧梻娲块幒銉ょ秼閻?鐎广垺鍩涚粩顖濐嚞濮瑰倸鎮?/ 娑撳﹥鐖堕崥?/ 鐠伮ゅ瀭濡€崇础"娑撳鍘撶紒鍕妇閺呯儤膩閸ㄥ鈧?
   - 閸氬海顏?`ModelPricingListItem.BillingBasisHint` 娴犲骸宕熺€涙顑佹稉鎻掑磳缁狙傝礋缂佹挻鐎担?`{ type, related_models }`
     娑撳顫?type閿?
     - `requested_equals_upstream`閳ユ柡鈧柨鎮撻崥宥嗘Ё鐏忓嫭鍨ㄧ痪?LiteLLM 濡€崇€烽敍宀冾嚞濮瑰倸鎮?= 娑撳﹥鐖堕崥?
     - `upstream_only`閳ユ柡鈧梹膩閸ㄥ妲搁弰鐘茬殸 value閿涘苯顓归幋椋庮伂娑撳秶娲块幒銉嚞濮瑰倸鐣犻敍娉乪lated_models 閸掓鍤幍鈧張澶嬫Ё鐏忓嫭绨拠閿嬬湴閸氬稄绱欓弨顖涘瘮婢舵艾顕稉鈧敍?
     - `requested_only`閳ユ柡鈧梹膩閸ㄥ妲搁弰鐘茬殸 key閿涘矁顫﹂弰鐘茬殸閸掓澘鍙炬禒鏍ф倳鐎涙绱眗elated_models 閸楁洖鍘撶槐鐘辫礋娑撳﹥鐖堕惄顔界垼
     娴兼ê鍘涚痪?`same_name > upstream_only > requested_only`閿涙硞ameName 閹懎鍠屾稊鐔凤綖 related_models 閹佃儻娴?鐞氼偉鐨濋弰鐘茬殸閸掔増鍨?娣団剝浼呴敍宀勪缉閸忓秳淇婇幁顖欐丢婢?
   - 閸撳秶顏?`ModelPricingTab.vue` 閹跺﹤甯?Model 閸楁洖鍨幏鍡樺灇閵嗗矁顕Ч鍌浤侀崹瀣倳 / 娑撳﹥鐖跺Ο鈥崇€烽崥宥冣偓宥呭蓟閸掓绱濋獮鑸垫煀婢х偑鈧矁顓哥拹瑙勀佸蹇嬧偓宥呭灙閿涘牆褰х拠缁樼垼缁涙拝绱伴幐澶庮嚞濮?/ 閹稿绗傚〒?/ 鐠囬攱鐪?娑撳﹥鐖堕敍?
     濮ｅ繗顢戦弽瑙勫祦 hint 閹恒劌顕辨稉銈呭灙鐏炴洜銇氶崐纭风窗
     - `requested_equals_upstream`閿涙矮琚遍崚妤冩祲閸?= model 閼奉亣闊╅敍宀冨 related_models 闂堢偟鈹栫仦鏇犮仛 `+N` 鐏忓繐绐橀弽?+ hover 閸掓鍙?
     - `requested_only`閿涙俺顕Ч?= model閿涘奔绗傚〒?= related_models[0]
     - `upstream_only`閿涙俺顕Ч?= related_models[0]閿?N 鐞涖劎銇氭径姘嚠娑撯偓閿涘绱濇稉濠冪埗 = model
   - Provider / Channels 閸掓鏁兼稉?`xl:table-cell`閿? 1280px 闂呮劘妫岄敍澶涚礉閼哄倻娓风€硅棄瀹?
   - 鐠伮ゅ瀭濡€崇础閸?*娑撳秴褰茬紓鏍帆**閿涘苯娲滄稉鍝勭暊娑撳秵妲告潻娆愭蒋鐠佹澘缍嶉惃鍕潣閹€鈧柡鈧柨鐣犻弰顖欑矤閺勭姴鐨犻崗宕囬兇閼奉亜濮╅幒銊︽焽閻ㄥ嫬鐫嶇粈鐑樼垼缁涙拝绱濈€圭偤妾拋陇鍨傞崺鍝勫櫙閻㈣精顕Ч鍌涘鐏炵偞绗柆鎾舵畱 `billing_model_source` 閸愬啿鐣?
   - banner 閻ㄥ嫬鐫嶅鈧崘鍛啇闁插矁藟娑撯偓閺?`billingBasisColumnNote` 鐠€锕€鎲″蹇氼嚛閺勫函绱濋弰搴ｂ€橀崨濠勭叀閻劍鍩?鏉╂瑤绔撮崚妤€褰х拠?+ 鐎圭偤妾悽杈ㄧ闁挸鍠呯€?

**妤犲矁鐦?*:
- `pnpm run typecheck` 闁俺绻?
- `go build ./...` 闁俺绻冮敍瀹峠o vet ./internal/service/` 閺冪姴鎲＄拃?
- 閺堫剙婀?API 鐎圭偞绁撮敍?
  - `provider=antigravity` 鏉╂柨娲?30 閺夆槄绱濋崥?type 閸掑棗绔风粭锕€鎮庢０鍕埂閿?
    - `requested_equals_upstream`閿涙瓪claude-opus-4-6-thinking`閿涘澁elated_models=[opus-4-5-20251101, opus-4-5-thinking, opus-4-6] 鐞涖劎銇氱悮?3 娑擃亣顕Ч鍌涙Ё鐏忓嫬鍩岄敍澶堚偓涔laude-sonnet-4-6`閿涘牐顫?haiku-4-5 / haiku-4-5-20251001 閺勭姴鐨犻崚甯礆閵嗕梗gemini-3.1-flash-image`閿涘牐顫?3 娑?image 濡€崇€烽弰鐘茬殸閸掑府绱氱粵?
    - `requested_only`閿涙瓪claude-haiku-4-5 閳?claude-sonnet-4-6`閵嗕梗claude-opus-4-6 閳?claude-opus-4-6-thinking`閵嗕梗gemini-3-pro-preview 閳?gemini-3-pro-high` 缁?
    - `upstream_only`閿涙ntigravity 姒涙顓婚弰鐘茬殸閻?value 閸╃儤婀伴柈鑺ユ箒閸氬苯鎮曢懛顏呮Ё鐏忓嫸绱濋幍鈧禒銉︽拱缁鍩嗛弳鍌涙濞屸剝鏆熼幑顔光偓鏂衡偓鏃囩箹閺勵垳顑侀崥鍫熸殶閹诡噣娉﹂悳鎵Ц閻ㄥ嫰顣╅張?
  - `GET /admin/model-pricing/gemini-3-pro-high` 閳?瀵ら缚顔呮禒閿嬫降閼?`gemini-2.5-pro`
  - `GET /admin/model-pricing/tab_flash_lite_preview` 閳?瀵ら缚顔呮禒閿嬫降閼?`gemini-2.5-flash-lite`
  - `GET /admin/model-pricing/gpt-oss-120b-medium` 閳?瀵ら缚顔呮禒閿嬫降閼?`gpt-4o-mini`閿涘牅绠ｉ崜宥堫潶 LiteLLM 濡紕纭﹂崠褰掑帳濮光剝鐓嬮幋?`1.25e-6 / 1e-5` 闁挎瑤鐜敍灞藉嚒娣囶喖顦查敍?
  - `GET /admin/model-pricing/claude-opus-4-6-thinking` 閳?濮濓絽鐖舵潻鏂挎礀 LiteLLM 娴犻攱鐗搁敍灞肩瑝鐟欙箑褰?suggestPricing

**瀹歌尙鐓￠梽鎰煑**:
- 閺勬儳绱″楦款唴娴犻攱妲х亸鍕€?`antigravityProprietarySuggestMap` 闂団偓鐟曚礁婀?Google/OpenAI 閸欐垶鏌婂Ο鈥崇€烽弮鍓佹樊閹躲倧绱濋惄顔煎閸欘亜顕?`tab_flash_lite_preview` / `gpt-oss-120b-medium` 娑撱倖娼?
- Popover 娴犲懏鏁幐?4 娑擃亝鐗宠箛鍐х幆閺嶇厧鐡у▓纰夌幢provider/notes/image_output_price/per_request_price/billing_mode 娴犲秹娓剁挧鏉垮斧 dialog閿涘牓鈧俺绻?popover 閻?鐠囷妇绮忕拋鍓х枂閳?閹稿鎸崇捄瀹犳祮閿?
- 閺傝顢?A 閻ㄥ嫪绻氱€瑰牓鈧瀚ㄩ敍姘弓閺夈儴瀚㈤崙铏瑰箛"閸氬奔绔村Ο鈥崇€烽崷銊ょ瑝閸?billing_model_source 娑撳娓剁憰浣风瑝閸氬奔鐜?閻ㄥ嫬鐤勯梽鍛瑹閸斺€虫簚閺咁垽绱濋棁鈧憰浣稿磳缁狙冨煂閺傝顢?B閿涘牏绮?GlobalModelPricing 閸?billing_model_source 鐎涙顔?+ 娴滃瞼娣紓鎾崇摠閿涘绱濋張顒侇偧娑撳秹妯嗘繅鐐额嚉閹碘晛鐫?

## [2026-04-15] fix(admin): 濡€崇€风€规矮鐜い?Gemini/Antigravity 鏉╁洦鎶ゆ径杈ㄦ櫏

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `backend/internal/service/global_model_pricing_service.go`閿涘潚ilterItems 閸掝偄鎮曢崠褰掑帳 + Antigravity 濡€崇€风悰銉ュ弿閿?
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`閿涘湙emini 娑撳濯?value 鐎靛綊缍堥敍?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈹库偓淇檉ilterItems`/`ListAllModels` 閺勵垯绨╁鈧?2026-04-12 閺傛澘顤冮惃鍕埠娑撯偓鐎规矮鐜粻锛勬倞閻ｅ矂娼伴敍鍫ｎ潌娑撳鏋冮敍澶涚礉娑撳﹥鐖跺▽鈩冩箒閸氬苯鎮曢崙鑺ユ殶閿涙稑鏁稉鈧崣顖濆厴閸愯尙鐛婇悙瑙勬Ц `domain.ResolveAntigravityDefaultMapping` 閻ㄥ嫬绱╅崗銉ｂ偓?

**閼冲本娅?*:
缁狅紕鎮婇崥搴″酱閵嗗本膩閸ㄥ鍘ょ純?閳?濡€崇€风€规矮鐜妴宄奱b 闁插矉绱漰rovider 娑撳濯洪柅?Gemini 閹?Antigravity 閺冭泛鍨悰銊よ礋缁屾亽鈧倹鐗撮崶鐙呯窗

1. **Gemini**閿涙艾澧犵粩顖欑瑓閹?value 閺?`vertex_ai`閿涘奔绲?LiteLLM JSON 闁?Gemini 鐎硅埖妫岄惃?`litellm_provider` 鐎涙顔岀€圭偤妾崐鍏兼Ц `gemini`閿涘湙oogle AI Studio閿涘鍨ㄧ敮锕€鎮楃紓鈧惃?`vertex_ai-language-models` / `vertex_ai-vision-models` / `vertex_ai-embedding-models`閿涘湸ertex AI閿涘绱漙filterItems` 閻?`strings.ToLower(item.Provider) != providerLower` 娑撱儲鐗搁惄鍝ョ搼閸栧綊鍘ゆ稉鈧稉顏堝厴閸涙垝绗夋稉顓溾偓?
2. **Antigravity**閿涙ntigravity 閺勵垯绨╁鈧懛顏嗙埡楠炲啿褰撮敍瀛teLLM 闁插奔绗夌€涙ê婀禒璁崇秿 `antigravity` provider 閺夛紕娲伴敍娑樻倱閺?`DefaultAntigravityModelMapping` 闁插苯鐣炬稊澶屾畱 Antigravity 閸欘垳鏁ゅΟ鈥崇€烽敍鍫濐洤 `gemini-3-pro-high`閵嗕梗tab_flash_lite_preview`閿涘鐗撮張顑跨瑝閸︺劌鍨悰銊︾亣娑撶偓娼靛┃鎰剁礄LiteLLM + 閸忋劌鐪憰鍡欐磰閿涘鍣烽妴?

**閸欐ɑ娲跨拠锔藉剰**:
- 閹惰棄鍤?`providerMatches(item, providerLower, antigravityModelSet)` 閹跺﹣寮楅弽鑲╂祲缁涘鏁兼稉鍝勫焼閸氬秵鍔呴惌銉窗
  - `gemini` 閳?閸栧綊鍘?`gemini` 閹?`vertex_ai` 閸撳秶绱?
  - `openai` 閳?閸栧綊鍘?`openai` 閹?`text-completion-openai`
  - `antigravity` 閳?閸栧綊鍘?`provider=antigravity` 閹存牗膩閸ㄥ鎮曢崨鎴掕厬 `domain.ResolveAntigravityDefaultMapping()` 閻?key
  - 閸忚泛鐣犻敍鍧卬thropic/bedrock 缁涘绱氶埆?娣囨繄鏆€閸樼喍寮楅弽鑲╂祲缁?
- `ListAllModels` 閸氬牆鑻熼梼鑸殿唽閺傛澘顤冩稉鈧潪顕€浜堕崢?`ResolveAntigravityDefaultMapping()`閿涘苯顕?LiteLLM 閸滃苯鍙忕仦鈧憰鍡欐磰闁姤鐥呴張澶屾畱濡€崇€烽崥宥埶夋稉鈧弶?provider=antigravity 閻?stub ListItem閿涘奔绻氱拠?Antigravity 娑撴挻婀佸Ο鈥崇€烽崷銊ュ灙鐞涖劑鍣烽崣顖濐潌閸欘垳顓搁妴?
- 閸撳秶顏?`ModelPricingTab.vue` 閻ㄥ嫪绗呴幏澶嬪Ω `<option value="vertex_ai">Gemini</option>` 閺€閫涜礋 `value="gemini"`閿涘奔绗岄崥搴ｎ伂閺傛澘鍩嗛崥宥咁嚠姒绘劑鈧?
- `modelSet` 閸氬牆鑻熷顏嗗箚閺傛澘顤冮惃鍕晸閸忋儳鈥樻穱?Antigravity stub 閸樺鍣搁弮?dedup 閸╁搫鍣€瑰本鏆ｉ敍鍫滅閸?all-overrides 瀵邦亞骞嗗蹇撳晸 modelSet閿涘苯浼撻崣鎴﹀櫢婢跺稄绱辨稉鈧挧铚傛叏閹哄绱氶妴?

**妤犲矁鐦?*:
- `go build ./internal/service/ ./internal/handler/admin/` 闁俺绻?
- `go vet ./internal/service/` 閺冪姴鎲＄拃?
- `pnpm run typecheck` 閺冪娀鏁婄拠?

## [2026-04-15] feat(tools): 閺傛澘顤冮崶鍓у閻㈢喐鍨?API 閸樺濮忓ù瀣槸閼存碍婀?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `tools/image_stress_test.py`閿涘牊鏌婃晶鐑囩礉閸楁洘鏋冩禒?Python 瀵倹顒為崢瀣ゴ閼存碍婀伴敍瀵?80 鐞涘矉绱?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 缁绢垱鏌婃晶鐐差吂閹撮顏銉ュ徔閿涘奔绗夌憴锔绢潾 backend/frontend/deploy閿涘本妫ゆ稉濠冪埗閸愯尙鐛婃搴ㄦ珦閵?

**閼冲本娅?*:
鐎广垺鍩涢崣宥夘洯闁俺绻?API 鐠嬪啰鏁?Gemini 閸ュ墽澧栭悽鐔稿灇濡€崇€烽敍鍧刧emini-3-pro-image` / `gemini-2.5-flash-image` 缁涘绱氶弮鍫曟晩鐠囶垳宸煎鍫ョ彯閿涘矂娓剁憰浣风娑擃亜褰叉径宥囧箛閵嗕礁褰茬拠濠冩焽閻ㄥ嫬浼愰崗宄板箵鐎规矮缍呴梻顕€顣介崚鏉跨俺閸戝搫婀稉濠冪埗鐠愶箑褰垮Ч鐘偓浣界殶鎼达箑娅掗妴浣界箷閺?Anthropic 閸忕厧顔愮紙鏄忕槯鐏炲倶鈧?

**閸欐ɑ娲跨拠锔藉剰**:
- 閻?`httpx[http2]` + `asyncio` 鐎圭偟骞囬崣妤佸付楠炶泛褰傞崢瀣ゴ
- 閺€顖涘瘮娑撱倖娼崗銉ュ經鐠侯垰绶為惃鍕嚠濮ｆ棑绱?
  1. `gemini-native`閿涙瓪POST /v1beta/models/{model}:generateContent`
  2. `anthropic-messages`閿涙瓪POST /v1/messages`閿涘牐铔?`GeminiMessagesCompatService` 缂堟槒鐦х仦鍌︾礆
- 娑旂喐鏁幐?`--stream` 鐠?`:streamGenerateContent`閿涘苯鎳℃稉顓濆敩閻線鍣?`handleGeminiStreamToNonStreaming` 閻ㄥ嫭绁﹀蹇撳瀻閺€?
- 闁挎瑨顕ら崚鍡欒鐎靛綊缍堥張宥呭缁旑垳娈戞径杈Е娣団€冲娇閿涙瓪empty_stream` / `safety_block` / `google_config_error` / `signature_error` / `overloaded_529` / `rate_limit_429` / `gateway_5xx` / `auth_401_403` / `client_4xx` / `timeout` / `network_error`
- 閻楃懓鍩嗙拠鍡楀焼 "200 OK 娴ｅ棙妫ら崶?閿涘潉candidates[0].content.parts` 闁插本妫?`inlineData`閿涘本鍨?`finishReason` 鐏炵偘绨?safety 缁紮绱氶垾鏂衡偓?鏉╂瑦妲哥€广垺鍩涢張鈧€硅妲楅幎濠傜暊瑜?bug 閹躲儳娈?case
- 濮ｅ繋閲滅拠閿嬬湴鐠佹澘缍?`X-Request-ID`閿涘畭summary.md` 娴兼艾鍨崙?top 婢惰精瑙?request_id 娓氬じ绨?SSH 閸掔増婀囬崝鈥虫珤閸忓疇浠堥弮銉ョ箶
- 鏉堟挸鍤紒鎾寸€敍姝歰utput/stress-<timestamp>/{run.json, requests.jsonl, summary.md}`閿涘畭output/` 瀹告彃婀?`.gitignore`
- 姒涙顓婚惄顔界垼 `https://zerocode.kaynlab.com`閿涘瓑PI key 娴?`$SUB2API_KEY` 鐠囪褰?
- Windows 閸欏銈介敍姘冲殰閸斻劍濡?stdout/stderr 闁插秹鍘ょ純顔昏礋 UTF-8 闁灝鍘?cp936 娑旇京鐖?

**娴ｈ法鏁?*:
```bash
export SUB2API_KEY=sk-xxx
python tools/image_stress_test.py --total 50 --concurrency 5 --mode gemini-native
```

鐎瑰本鏆ｉ幍褑顢戝ù浣衡柤閿涘牆鍟嬮悜?閳?閸╄櫣鍤?閳?楠炶泛褰傞幍?閳?濡€崇础鐎佃鐦?閳?濡€崇€风€佃鐦?閳?濞翠礁绱￠敍澶庮潌 `tools/image_stress_test.py` 濡€虫健濞夈劑鍣存い鍫曞劥閵?

---

## [2026-04-15] feat: 閺傛澘顤冩导浣风瑹瀵邦喕淇婇弨顖欑帛閺傜懓绱?

**瑜板崬鎼烽懠鍐ㄦ纯**: backend/internal/payment/, frontend/src/views/admin/
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ骸鍟跨粣渚€顥撻梽鈺嬬礉閺傛澘顤冮弬鍥︽娑撹桨瀵?
**閸欐ɑ娲跨拠锔藉剰**:
- 閺傛澘顤?payment/provider/wechat_work.go
- 濞ｈ濮?WeChatWorkProvider 鐎圭偟骞?PaymentProvider 閹恒儱褰?
- 閸撳秶顏粻锛勬倞妞ゅ灚鏌婃晶鐐扮磼娑撴艾浜曟穱鈩冩暜娴犳﹢鍘ょ純顔裤€冮崡?
- config.yaml 閺傛澘顤?payment.wechat_work 闁板秶鐤嗗▓?

**閸忓疇浠?Issue/PR**: #12

## [2026-04-14] chore(deploy): remote_exec.py 婢х偛濮?--update 韫囶偅宓庨弬鐟扮础闁灝绱?MSYS2 鐠侯垰绶炴潪顒佸床

**瑜板崬鎼烽懠鍐ㄦ纯**:
- `deploy/remote_exec.py`閿?*閺?tracked閿涘本婀伴崷鐗堟暭閸?*閿?gitignore 娑擃叏绱遍崶鐘叉儓閺勫孩鏋?SSH 閸戭叀鐦夋稉宥呭弳鎼存搫绱?
- `CLAUDE.md`閿涘澋orkflow + 閻㈢喍楠囬張宥呭閸ｃ劎鐝烽懞鍌︾礆
- `docs/dev/UPSTREAM_SYNC.md`閿涘牓鍎寸純鍙夊瘹娴犮倛瀵栨笟瀣剁礆

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴犲懎濂栭崫宥嗘拱閸︽澘浼愭担婊勭ウ閿涘奔绗夊☉澶婂挤娴犺缍嶆稉濠冪埗閺傚洣娆㈤妴?

**閼冲本娅?*:
2026-04-14 v0.1.112 閸氬牆鑻熺€瑰本鍨氶崙鍡楊槵闁劎璁查弮璁圭礉閸?Git Bash 娑撳澧界悰?
`python deploy/remote_exec.py "/opt/sub2api/update.sh"` 閹?
`bash: line 1: D:/program: No such file or directory` 婢惰精瑙﹂妴?
鐎规矮缍呴崥搴ｂ€樼拋銈嗘Ц MSYS2 argv path conversion閿涙it Bash 娴兼碍濡告禒璁崇秿閻鎹ｉ弶銉ュ剼
POSIX 缂佹繂顕捄顖氱窞閻?argv 閸欏倹鏆熼敍鍧?opt/...`閿涘鍊撻幃鍕祮閹?Windows 鐠侯垰绶為崥搴㈠娴溿倗绮?
Python閿涘奔绨弰?argv[1] 閸欐ɑ鍨氭禍?`D:\program files\...\opt\sub2api\update.sh`閿?
SSH 鏉╂粎顏弨璺哄煂娑撯偓娑擃亙绗夌€涙ê婀惃鍕熅瀵板嫯鍤滈悞璺恒亼鐠愩儯鈧?

**閸欐ɑ娲跨拠锔藉剰**:
- `deploy/remote_exec.py`
  - 閺傛澘顤?`SHORTCUTS` 鐎涙鍚€ + `--update` 韫囶偅宓庨弬鐟扮础閿涘苯鍞撮柈銊ф暏 Python 鐎涙顑佹稉鎻掔摟闂堛垽鍣?
    `"bash /opt/sub2api/update.sh"`閿涘苯鐣崗銊х搏鏉?MSYS2 argv 鏉烆剚宕?
  - 閺傛澘顤?`--env` 濡€崇础娴?`REMOTE_CMD` 閻滎垰顣ㄩ崣姗€鍣虹拠璇叉嚒娴犮倧绱欐担鍡曠矝闂団偓闁板秴鎮?
    `MSYS_NO_PATHCONV=1` 閹靛秷鍏樼拋?Git Bash 娑撳秷娴?env 闁插瞼娈戠捄顖氱窞閿涙稐缍旀稉?escape hatch閿?
  - 閺傛澘顤冪紒鎾寸€崠?docstring 鐠囧瓨妲?MSYS2 闂勭兘妲洪崪灞芥磽缁?workaround 娴兼ê鍘涚痪?
  - `run()` 姒涙顓?timeout 娴?300s 閹绘劕宕岄崚?600s閿涘矂鈧倿鍘?Docker build 閸︾儤娅?
  - 鏉堟挸鍤?decode 閸?`errors="replace"`閿涘矂浼╅崗宥勭癌鏉╂稑鍩楀Ч鈩冪厠閺?UnicodeDecodeError

- `CLAUDE.md` workflow 濮濄儵顎?4/5 娑撳簺鈧瞼鏁撴禍褎婀囬崝鈥虫珤閵嗗秶鐝烽懞?
  - 闁劎璁查崨鎴掓姢閺€閫涜礋 `python deploy/remote_exec.py --update`
  - 鏉╄棄濮?MSYS2 gotcha 鐠€锕€鎲￠崪灞惧瘹閸?remote_exec.py docstring 閻ㄥ嫬绱╅悽?
  - 閻㈢喍楠囬張宥呭閸?SSH 鐎涙顔岀拠瀛樻 ad-hoc 閸涙垝鎶ゆ禒鍛存娑撳秳浜?`/` 瀵偓婢跺娈戦崨鎴掓姢

- `docs/dev/UPSTREAM_SYNC.md`
  - 閺堫剚顐奸柈銊ц閺夛紕娲版潻钘夊瀹告煡鍎寸純鍙夌垼鐠?
  - 闁劎璁查幐鍥︽姢閼煎啩绶ラ弨鍦暏 `--update` 楠炶埖鏁為弰搴㈡＋閻劍纭剁悮顐㈢磾閻劎娈戦崢鐔锋礈

**闁劎璁叉宀冪槈**:
- `python deploy/remote_exec.py --update` 缁旑垰鍩岀粩顖濈獓闁熬绱皃ull閿涘牆鍑?up-to-date閿涘鍟?
  docker build 閳?docker compose up 閳?health check `{"status":"ok"}` 閳?ps 閺勫墽銇?
  sub2api 鐎圭懓娅?`Up 8 seconds (healthy)`閵?

**閸忓疇浠?*: 閺?issue閵嗗倷鎱ㄦ径宥嗙爱娴?2026-04-14 v0.1.112 閸氬本顒為柈銊ц鏉╁洨鈻兼稉顓炲絺閻滆埇鈧?

---

## [2026-04-14] fix(billing): 娣囶喖顦查崗銊ョ湰濡€崇€风€规矮鐜憰鍡欐磰閸?Anthropic 缂冩垵鍙ф径杈ㄦ櫏閸欏﹤顦挎径鍕吀鐠愯绱″ú?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- backend/internal/service/model_pricing_resolver.go閿涘牊鐗宠箛鍐掗弸鎰珤闁插秴鍟撻敍?
- backend/internal/service/global_model_pricing.go閿涘牆鍨归梽銈嗘箒 bug 閻?ToModelPricing閿?
- backend/internal/service/global_model_pricing_cache.go閿涘牊鏌婃晶鐑囩礆
- backend/internal/service/global_model_pricing_service.go閿涘牊鏁為崗銉х处鐎涙ê鑻熼崷?CUD 閺冭泛銇戦弫鍫礆
- backend/internal/service/gateway_service.go閿涘澁esolveChannelPricing 閸氬本妞傞幒銉ュ綀 Global 閺夈儲绨敍?
- backend/internal/service/wire.go閿涘湧rovider set 鏉╄棄濮?NewGlobalPricingCache閿?
- backend/cmd/server/wire_gen.go閿涘牊澧滈崝銊ユ倱濮?DI 閹恒儳鍤庨敍?
- backend/internal/handler/admin/model_pricing_handler.go閿涘湶pdateOverride 瀹割噣鍣洪弴瀛樻煀閿?
- backend/internal/service/model_pricing_resolver_test.go閿涘牊鏌婃晶?5 娑擃亜娲栬ぐ鎺撶ゴ鐠囨洩绱?

**娑撳﹥鐖堕崗鐓庮啇閹?*: 妤傛ê瀹抽崣顖濆厴娴溠呮晸閸愯尙鐛?閳ユ柡鈧?鐟欙箑寮锋稉濠冪埗 resolver 娑?gateway_service 閻ㄥ嫭鐗宠箛?
鐠伮ゅ瀭鐠侯垰绶為敍灞间簰閸?wire_gen.go閵嗗倸鎮庨獮鏈电瑐濞撳憡妞傛俊鍌涚亯鐎规ɑ鏌熼柌宥嗙€禍?ModelPricingResolver 閹?
GatewayService.calculateTokenCost 闂団偓鐟曚線鍣搁弬鐗堟殻閸氬牊婀版穱顔碱槻閵?

**閼冲本娅?*:
鐎孤ゎ吀缁狅紕鎮婇崥搴″酱"濡€崇€烽柊宥囩枂 閳?Pricing"妞ょ敻娼伴惃鍕┾偓灞藉弿鐏炩偓鐟曞棛娲婇妴宥呭閼宠姤妲搁崥锔绢伂閸掓壆顏悽鐔告櫏閿?
閸欐垹骞囩€瑰啫婀径姘蒋鐠侯垰绶炴稉濠咁潶闂堟瑩绮紒鏇＄箖閹存牔娑径鍗炵摟濞堢绱濈拠锕侇潌閺堫剚顐?commit 鐠囧瓨妲戦妴?

**閸欐ɑ娲跨拠锔藉剰**閿涘牊瀵?bug 鐎电懓绨叉穱顔碱槻閿?

- **Bug A 閳?Anthropic 缂冩垵鍙ч悜顓＄熅瀵板嫮绮潻鍥у弿鐏炩偓鐟曞棛娲?*
  `gateway_service.go:resolveChannelPricing` 閸樼喐婀伴崣顏勬躬 `Source==Channel` 閺冩儼绻戦崶?
  resolved閿涘苯顕遍懛娣偓灞藉涧闁板秳绨￠崗銊ョ湰鐟曞棛娲婇妴浣圭梾闁板秵绗柆鎾扁偓宥囨畱閹懎鑸版导姘礀閽€钘夊煂 `CalculateCost` 閺?
  鐠侯垰绶為妴鍌涙＋鐠侯垰绶炵€瑰苯鍙忔稉宥嗙叀 GlobalPricingRepository閿涘苯鍙忕仦鈧憰鍡欐磰 閳?闂堟瑩绮径杈ㄦ櫏閵嗗倷鎱ㄦ径宥忕窗
  閺€鎯ь啍閺夆€叉娑?`Source==Channel || Source==Global`閿涘苯鎮撻弮鏈电箽閻ｆ瑥鍤遍弫鏉挎倳娴犮儱鍣虹亸?diff閵?

- **Bug B 閳?ResolvedPricing.Mode 韫囩晫鏆愰崗銊ョ湰鐟曞棛娲婇惃?BillingMode**
  閸?`Resolve` 閹?`Mode` 绾剛绱惍浣疯礋 `BillingModeToken`閿涘苯褰ч崷銊︾闁挸褰旈崝鐘插瀻閺€顖炲櫡閺€骞库偓?
  閸氬孩鐏夐敍姘鳖吀閻炲棗鎲抽崷銊ュ弿鐏炩偓鐟曞棛娲婇柌宀勨偓?`per_request` / `image` 閳?閸氬海顏禒宥嗗瘻 token 鐠伮ゅ瀭 閳?
  閸楁洑鐜崗銊よ礋 0 閳?閻劍鍩涢崗宥堝瀭閵嗗倷鎱ㄦ径宥忕窗`resolveBasePricing` 鏉╂柨娲?`(pricing, mode,
  defaultPerRequestPrice, source)` 閸ユ稑鍘撶紒鍕剁礉`Resolve` 閸樼喐鐗辨繅鐐剁箻 `ResolvedPricing`閵?

- **Bug C 閳?ToModelPricing 娑撱垹銇?Priority/闂€澶哥瑐娑撳鏋?缂傛挸鐡ㄩ崚鍡欓獓鐎涙顔?*
  閸?`GlobalModelPricing.ToModelPricing()` 閸欘亣顔?5 娑擃亜鐡у▓纰夌礉鐎佃壈鍤?Priority tier 閸楁洑鐜?
  瑜版帡娴傞妴涓烶T-5.4 闂€澶哥瑐娑撳鏋冮崣灞解偓宥堝瀭娑撱垹銇戦妴浣虹处鐎?5m/1h 閸掑棛楠囨径杈ㄦ櫏缁涘鈧倷鎱ㄦ径宥忕窗
  1. 閸掔娀娅庣拠銉︽煙濞?
  2. `resolveBasePricing` 閸忓牅绮?`BillingService.GetModelPricing` 閹峰灝鐣弫鏉戠唨绾偓鐎规矮鐜?
     閿涘牆鎯?LiteLLM 閻ㄥ嫭澧嶉張澶婄摟濞堢绱氶敍灞藉晙閻?`applyGlobalPricingOverride` 閹跺﹤鍙忕仦鈧憰鍡欐磰閻?
     闂?nil 鐎涙顔岄崣鐘插娑撳﹤骞撻敍娑滎嚔娑斿绗?`applyTokenOverrides`閿涘牊绗柆鎾诡洬閻╂牭绱氱€瑰苯鍙忕€靛綊缍堥敍?
     閸栧懏瀚?Priority 鐎涙顔屾稉搴ゎ洬閻╂牔鐜崥灞绢劄閵嗕梗CacheWritePrice` 閸氬本妞傞崘娆忓弳 5m/1h閵?
  3. 閺堫亣顫︾憰鍡欐磰閻ㄥ嫬鐡у▓纰夌礄Priority 閸楁洑鐜顔衡偓渚€鏆辨稉濠佺瑓閺傚洤鈧秶宸肩粵澶涚礆缂佈勫閼?LiteLLM 閸╄櫣顢呴妴?

- **Bug D 閳?濮ｅ繋閲滅拠閿嬬湴娑撯偓濞?SQL 閺冪姷绱︾€?*
  閸樼喎鐤勯悳鏉挎躬閻戭叀鐭惧鍕嚠 `global_model_pricing` 鐞涖劍鐦＄拠閿嬬湴娑撯偓濞?`SELECT`閵嗗倷鎱ㄦ径宥忕窗閺傛澘顤?
  `GlobalPricingCache`閿涘澃ync.RWMutex + 閹増鈧冨鏉炴枻绱氶敍宀勵浕濞喡ゎ問闂傤喗妞傛稉鈧▎鈩冣偓褑顕伴崗銉﹀閺?
  `enabled=true` 閺夛紕娲伴崚鏉垮敶鐎?map閿涘苯鎮楃紒?O(1) 閺屻儴顕楅敍娑氼吀閻炲棗鎮楅崣鏉挎躬 Create/Update/
  Delete 閸氬氦鐨熼悽?`Invalidate()` 濞撳懐鈹栫紓鎾崇摠閵?

- **Bug E 閳?resolveBasePricing 娴ｈ法鏁?context.Background**
  閸樼喎鐤勯悳棰佹丢瀵啳鐨熼悽銊ㄢ偓?ctx 鐎佃壈鍤х拠閿嬬湴鐡掑懏妞傞弮鐘崇《娴肩娀鈧帇鈧倷鎱ㄦ径宥忕窗缂傛挸鐡ㄩ崠鏍︾閸氬海鍎圭捄顖氱窞娑撳秴鍟€鏉?DB閿?
  ctx 闂傤噣顣介懛顏嗗姧濞戝牆銇戦敍娑楃矌閸︺劎绱︾€涙﹢顩诲▎鈥冲鏉炶姤妞傞悽?background ctx 閹笛嗩攽娑撯偓濞嗏剝鈧冨弿闁插繑鐓＄拠顫偓?

- **Bug F 閳?UpdateOverride 閹跺﹥澧嶉張澶嬫弓閹绘劒绶电€涙顔屽〒鍛存祩**
  閸?handler 鐎?`InputPrice` 缁涘瀵氶柦鍫濈摟濞堝灚妫ら弶鈥叉鐠у鈧》绱漃ATCH 濠曞繐鐢禒璁崇秿娑撯偓娑擃亜鐡у▓鐢稿厴娴?
  閹跺﹤鍑￠張澶夌幆閺嶈壈顩惄鏍ㄥ灇 nil閵嗗倷鎱ㄦ径宥忕窗缂佺喍绔撮弨閫涜礋"闂?nil 閹靛秷顩惄?閻ㄥ嫬妯婇柌蹇旀纯閺傚府绱欐稉?
  `Model` / `Provider` / `Enabled` 鐎涙顔岄惃鍕槱閻炲棗顕鎰剁礆閵嗗倽顩﹀〒鍛存珟閺屾劒閲滄禒閿嬬壐鐠?
  delete 鐟曞棛娲婇崥搴ㄥ櫢瀵ゆ亽鈧?

**閸ョ偛缍婂ù瀣槸**閿涘潉model_pricing_resolver_test.go` 閺傛澘顤冮敍?
1. `TestResolve_GlobalOverride_PreservesPriorityAndLongContext` 閳?鐟曞棛娲?input/output
   閸氬酣鐛欑拠?Priority 閸氬本顒為妴渚€鏆辨稉濠佺瑓閺傚洭妲囬崐?閸婂秶宸?缂傛挸鐡?5m/1h 娴?LiteLLM 缂佈勫
2. `TestResolve_GlobalOverride_CacheWriteSyncsAllCacheFields` 閳?鐟曞棛娲?CacheWritePrice
   閸?Creation/5m/1h 娑撳鐡у▓闈涘弿闁劌鎮撳?
3. `TestResolve_GlobalOverride_DisabledIsIgnored` 閳?enabled=false 娑撳秶鏁撻弫?
4. `TestResolve_GlobalOverride_BillingModeRespected` 閳?per_request 濡€崇础濮濓絿鈥樻导鐘烩偓?
   BillingMode 閸?DefaultPerRequestPrice
5. `TestResolve_ChannelOverride_BeatsGlobalOverride` 閳?娴兼ê鍘涚痪?Channel > Global

閹碘偓閺堝鏌婂ù瀣槸闁俺绻冮敍娑欐＆閺?`./internal/service/...` 閸楁洖鍘撳ù瀣槸婵傛ぞ娆㈤崗銊ц雹閿?6 缁夋帪绱氶敍?
`go build ./...` 闁俺绻冮妴?

**閸忓疇浠?Issue/PR**: 閺冪媴绱欓張顒€婀寸€孤ゎ吀閸欐垹骞囬敍?

---

## [2026-04-14] feat(frontend): 娴狅絿鎮婇幍褰掑櫤鐎电厧鍙嗛弨顖涘瘮 host:port:user:pass 缁涘鐣濋崘娆愮壐瀵?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- frontend/src/views/admin/ProxiesView.vue
- frontend/src/i18n/locales/{zh,en}.ts

**娑撳﹥鐖堕崗鐓庮啇閹?*: 缁绢垰澧犵粩顖涙暭閸旑煉绱濇禒鍛⒖鐏炴洝袙閺嬫劙鈧槒绶崪?UI 閺傚洦顢嶉敍娑欐弓鐟欙妇顫崥搴ｎ伂 API閵嗗倸鎮庨獮鏈电瑐濞撴瓕瀚㈤弨?`parseProxyUrl` 閹?`batchInputPlaceholder/Hint` 閸欘垵鍏樻禍褏鏁撻崘鑼崐閵?

**閸欐ɑ娲跨拠锔藉剰**:
- `parseProxyUrl` 娴犲骸宕熸稉鈧?URL 濮濓絽鍨幍鈺佺潔娑撳搫娲撳▓?fallback 鐟欙絾鐎介敍?
  - A. `protocol://[user:pass@]host:port`閿涘牆甯張澶涚礉閸楀繗顔呴弶銉ㄥ殰鐞涘苯鍞撮敍灞肩喘閸忓牏楠囬張鈧姗堢礆
  - B. `user:pass@host:port`閿涘牊鏌婇敍灞炬￥閸楀繗顔呴崜宥囩磻閿?
  - C. `host:port:user:pass`閿涘牊鏌婇敍瀛璻oxyScrape / 911 缁绶垫惔鏂挎櫌鐢瓕顫嗛弽鐓庣础閿涙稑鐦戦惍浣风箽閻ｆ瑨顢戠亸鐐閺堝娼粚铏规鐎涙顑侀敍?
  - D. `host:port`閿涘牊鏌婇敍灞炬￥鐠併倛鐦夐敍?
  - 閹绘劕褰囬崙?`buildResult` 鏉堝懎濮崙鑺ユ殶缂佺喍绔撮崑姘鳖伂閸?娑撶粯婧€閺嶏繝鐛欓妴?
- 閸?韫囶偅宓庡ǎ璇插"Tab 妞ゅ爼鍎撮弬鏉款杻"姒涙顓婚崡蹇氼唴"娑撳濯洪敍鍧刡atchDefaultProtocol`閿涘矂绮拋?`http`閿涘绱濈粻鈧崘娆愮壐瀵?B/C/D 閻ㄥ嫯顢戞导姘殰閻劏绻栨稉顏勫礂鐠侇噯绱遍崚鍥ㄥ床閺冨爼鈧俺绻?`@update:modelValue` 鐟欙箑褰?`parseBatchInput` 闁插秶鐣婚敍灞炬￥闂団偓閻劍鍩涢柌宥嗘煀缂傛牞绶弬鍥ㄦ拱閵?
- 閸忔娊妫村鍦崶閺冭泛婀?`closeCreateModal` 闁插矂鍣哥純?`batchDefaultProtocol`閵?
- i18n閿涙碍澧块崗?`batchInputPlaceholder`閵嗕梗batchInputHint` 缁€杞扮伐閿涙稒鏌婃晶?`batchDefaultProtocol`閵嗕梗batchDefaultProtocolHint` 娑撱倖娼?key閿涘牅鑵戦懟鍗炲蓟鐠囶厼顕鎰剁礆閵?
- 閸氬海顏?`BatchCreate` 閹恒儱褰涙稉宥呭綁閿涘牅绮涢幒銉︽暪 `{protocol,host,port,username,password}`閿涘绱濋弮鐘绘付鏉╀胶些閵?

**閸忓疇浠?Issue/PR**: 閺?

## [2026-04-13] feat: Gemini Google One 閹靛綊鍣?Refresh Token 鐎电厧鍙?

**瑜板崬鎼烽懠鍐ㄦ纯**:
- backend/internal/pkg/geminicli/{constants.go, token_types.go}
- backend/internal/service/{gemini_oauth.go, gemini_oauth_service.go, gemini_oauth_service_test.go}
- backend/internal/repository/gemini_oauth_client.go
- backend/internal/handler/admin/gemini_oauth_handler.go
- backend/internal/server/routes/admin.go
- frontend/src/api/admin/gemini.ts
- frontend/src/composables/useGeminiOAuth.ts
- frontend/src/components/account/CreateAccountModal.vue
- frontend/src/i18n/locales/{zh,en}.ts

**娑撳﹥鐖堕崗鐓庮啇閹?*: 娑擃參顥撻梽?閳?GeminiOAuthClient 閹恒儱褰涢弬鏉款杻 GetUserInfo閿涙保reateAccountModal 婢舵艾顦╅弶鈥叉閸氬牆鑻熼敍灞芥値楠炴湹绗傚〒鍛婃閸欘垵鍏橀崘鑼崐

**閸欐ɑ娲跨拠锔藉剰**:
- 閸氬海顏敍?
  - `geminicli` 閺傛澘顤?`UserInfoURL` 鐢悂鍣?+ `UserInfo` 缁鐎烽敍鍫濐槻閻?Google userinfo 缁旑垳鍋ｉ敍?
  - `GeminiOAuthClient` 閹恒儱褰涢弬鏉款杻 `GetUserInfo(ctx, accessToken, proxyURL)`閿涙矖geminiOAuthClient` 鐎圭偟骞?+ 濞村鐦?mock 閸氬本顒為弴瀛樻煀
  - `GeminiTokenInfo` 閸?`Email` 鐎涙顔岄敍娌桞uildAccountCredentials` 閸?email 闂堢偟鈹栭弮璺哄晸閸?`credentials.email`閿涘牅绗?Antigravity 鐎靛綊缍堥敍灞筋槻閻劏澶勯崣宄板灙鐞涖劍鎮崇槐?`credentials->email` 缁便垹绱╅敍?
  - 閺傛澘顤?`ValidateGoogleOneRefreshToken` 閺堝秴濮熼弬瑙勭《閿涙efresh 閳?閸ョ偛锝?RT 閳?`GetUserInfo` 閹?email閿涘牆銇戠拹銉﹀ⅵ warning 娑撳秹妯嗛弬顓ㄧ礆閳?`fetchProjectID`閿涘牆绻€闂団偓閿涘鍟?`FetchGoogleOneTier`閿涘牆銇戠拹銉ユ礀閽€?free閿?
  - 閺傛澘顤?`POST /admin/gemini/oauth/refresh-token` handler + 鐠侯垳鏁卞▔銊ュ斀
- 閸撳秶顏敍?
  - `useGeminiOAuth` 閸?`validateGoogleOneRefreshToken` 閺傝纭堕敍瀹峛uildCredentials` 闁繋绱?email
  - `CreateAccountModal`閿涙瓪isEmailAsNameAvailable` 鐠侊紕鐣荤仦鐐粹偓褏绮烘稉鈧?Antigravity / Gemini+google_one 閻?閻劑鍋栫粻鍙樼稊娑撻缚澶勯崣宄版倳"瀵偓閸忕绱盽handleValidateRefreshToken` 閸?gemini 閸掑棙鏁敍娑欐煀婢?`handleGeminiGoogleOneValidateRT`閿涘牆鎯婇悳?RT 閳?閸楁洑閲滈崚娑樼紦閿?
  - OAuthAuthorizationFlow 閻?`show-refresh-token-option` 閹碘晛鐫嶇憰鍡欐磰 `gemini + google_one`
  - zh/en i18n 鐞涖儵缍?`admin.accounts.oauth.gemini` 閻?RT 閹靛綊鍣虹€电厧鍙嗛弬鍥攳
- 闂勬劕鍩楅敍姘矌閺€顖涘瘮 `google_one`閿涙被T 韫囧懘銆忛悽鍗炲敶缂?Gemini CLI OAuth client 缁涙儳褰傞敍鍫ｅ殰瀵?client 閻?RT 娴兼碍濮?`unauthorized_client`閿涘矂鏁婄拠顖涘絹缁€鍝勫嚒閸栧懎鎯堥惄绋跨安鐠囧瓨妲戦敍?

## [2026-04-12] feat: 缂佺喍绔村Ο鈥崇€风€规矮鐜粻锛勬倞閻ｅ矂娼?

**瑜板崬鎼烽懠鍐ㄦ纯**: backend(migrations, service, repository, handler, routes, wire), frontend(views, components, api, i18n)
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈺嬬礉閺傛澘顤冮崝鐔诲厴閿涘奔绗夋穱顔芥暭閻滅増婀佺拋陇鍨傞柅鏄忕帆
**閸欐ɑ娲跨拠锔藉剰**:
- 閺傛澘顤?`global_model_pricing` 閺佺増宓佹惔鎾广€冮敍灞炬暜閹镐胶顓搁悶鍡楁喅鐠佸墽鐤嗛崗銊ョ湰濡€崇€风€规矮鐜憰鍡欐磰
- 鐎规矮鐜憴锝嗙€介柧鐐⒖鐏炴洑璐熼敍娆砲annel 閳?Global 閳?LiteLLM 閳?Fallback閿涘牆鎮滄稉瀣悑鐎圭櫢绱濈悰銊よ礋缁岀儤妞傜悰灞艰礋娑撳秴褰夐敍?
- 閸氬海顏弬鏉款杻 GlobalModelPricingRepository閵嗕笩lobalModelPricingService閵嗕府odelPricingHandler
- 閺傛澘顤?API 缁旑垳鍋?GET/POST/PUT/DELETE /admin/model-pricing閿涘苯鎯堢拹鍦芳娑旀ɑ鏆熷鍌濐潔
- PricingService 閺傛澘顤?GetAllModels() 閺傝纭舵笟娑氼吀閻炲棗鎮楅崣鏉跨潔缁€鐑樺閺?LiteLLM 濡€崇€?
- 閸撳秶顏Ο鈥崇€烽柊宥囩枂妞ゅ灚鏁兼稉?Tab 鐢啫鐪敍姘侀崹瀣暰娴犲嚖绱欓弬鏉款杻閿涘 濡€崇€烽弰鐘茬殸閿涘牏骞囬張澶涚礆| 鐠愬湱宸煎鍌濐潔閿涘牊鏌婃晶鐑囩礆
- 濡€崇€风€规矮鐜?Tab閿涙艾鍙忓Ο鈥崇€烽崚妤勩€?+ 閹兼粎鍌?缁涙盯鈧?+ 閸忋劌鐪憰鍡欐磰缂傛牞绶鍦崶 + 濞撶娀浜剧憰鍡欐磰鐏炴洜銇?
- 鐠愬湱宸煎鍌濐潔 Tab閿涙艾褰х拠璇茬潔缁€鍝勬倗閸掑棛绮嶇拹鍦芳娑旀ɑ鏆熼敍宀勬懠閹恒儱鍩岄崚鍡欑矋缁狅紕鎮婃い?
- 娑擃叀瀚抽弬?i18n 缂堟槒鐦х€瑰本鏆?

## [2026-04-12] feat: 濡€崇€烽柊宥囩枂妞ょ敻娼板ǎ璇插濡€崇€峰ù瀣槸閸旂喕鍏?

**瑜板崬鎼烽懠鍐ㄦ纯**: frontend/src/views/admin/ModelConfigView.vue, i18n
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈺嬬礉娴犲懎澧犵粩顖涙暭閸?
**閸欐ɑ娲跨拠锔藉剰**:
- ModelConfigView 閺€閫涜礋瀹革箑褰哥敮鍐ㄧ湰閿涙艾涔忔笟褎妲х亸鍕帳缂冾噯绱濋崣鍏呮櫠濡€崇€峰ù瀣槸
- 濞村鐦崠鍝勭厵閿涙俺澶勯崣鐑解偓澶嬪閿涘牐鍤滈崝銊┾偓澶岊儑娑撯偓娑擃亜褰查悽顭掔礉閸欘垱澧滈崝銊ュ瀼閹诡澁绱氶妴浣鼓侀崹瀣╃瑓閹峰鈧焦褰佺粈楦跨槤鏉堟挸鍙?
- 婢跺秶鏁?POST /admin/accounts/:id/test API閿涘SE 濞翠礁绱＄仦鏇犮仛娑撳﹥鐖堕崫宥呯安
- 缂佸牏顏搴㈢壐鏉堟挸鍤崠鍝勭厵閿涘矁澹婅ぐ鈺佸隘閸掑棴绱檆yan=娣団剝浼? green=閸愬懎顔? red=闁挎瑨顕? emerald=閹存劕濮涢敍?

## [2026-04-12] feat: 閻欘剛鐝?濡€崇€烽柊宥囩枂"缁狅紕鎮婃い鐢告桨 閳?Antigravity 閸忋劌鐪妯款吇閺勭姴鐨?

**瑜板崬鎼烽懠鍐ㄦ纯**: 閸撳秴鎮楃粩顖氼樋閺傚洣娆?
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娑擃參顥撻梽鈺嬬礉閺傛澘顤冮弬鍥︽娑撹桨瀵岄敍灞肩稻娣囶喗鏁兼禍?account.go 閻ㄥ嫰绮拋銈嗘Ё鐏忓嫬娲栭柅鈧柅鏄忕帆閸?wire_gen.go
**閸欐ɑ娲跨拠锔藉剰**:
- 閸氬海顏? 閺傛澘顤?setting key `antigravity_default_model_mapping`閿涘苯鐡ㄩ崒銊ユ躬 settings 鐞?
- 閸氬海顏? SettingService 閺傛澘顤?Get/Set 閺傝纭?
- 閸氬海顏? AccountHandler 閺傛澘顤?PUT API閿涘奔鎱ㄩ弨?GET API 娴兼ê鍘涚拠?settings
- 閸氬海顏? domain.constants.go 閺傛澘顤?`GetAntigravityDefaultMappingOverride` 閸戣姤鏆熼崣姗€鍣?
- 閸氬海顏? account.go 娑?`resolveModelMapping` 閺€閫涜礋鐠嬪啰鏁?`domain.ResolveAntigravityDefaultMapping()`
- 閸氬海顏? wire_gen.go 濞夈劌鍙?override 閸戣姤鏆?+ settingService 娴肩姴鍙?AccountHandler
- 閸撳秶顏? 閺傛澘缂?ModelConfigView.vue閿涘牏瀚粩瀣€夐棃顫礉缁狅紕鎮婇崨妯哄讲鐟欎緤绱?
- 閸撳秶顏? 閺傛澘顤冪捄顖滄暠 `/admin/model-config`閵嗕椒鏅舵潏瑙勭埉閼挎粌宕熸い?
- 閸撳秶顏? accounts API 閺傛澘顤?`updateAntigravityDefaultModelMapping`
- 閸撳秶顏? zh.ts/en.ts 閺傛澘顤?modelConfig i18n 閺傚洦婀?
- 娴兼ê鍘涚痪? 閸楁洝澶勯崣鐤殰鐎规矮绠熼弰鐘茬殸 > 閸忋劌鐪弰鐘茬殸閿涘澃ettings閿? 閸愬懐鐤嗘妯款吇閿涘潏onstants.go閿?

## [2026-04-12] fix: Antigravity 閹靛綊鍣洪崚娑樼紦鐠愶箑褰?allow_overages 閺堫亞鏁撻弫?

**瑜板崬鎼烽懠鍐ㄦ纯**: frontend/src/components/account/CreateAccountModal.vue
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈺嬬礉閸楁洝顢戞穱顔芥暭
**閸欐ɑ娲跨拠锔藉剰**:
- 閹靛綊鍣洪崚娑樼紦閺?`extra` 绾剛绱惍浣疯礋 `{}`閿涘本鏁兼稉楦跨殶閻?`buildAntigravityExtra()`閿涘本顒滅涵顔荤炊闁?`allow_overages` 閸?`mixed_scheduling`

## [2026-04-12] fix: TypeScript 缁鐎烽柨娆掝嚖 ApiResponse 閺傤叀鈻?

**瑜板崬鎼烽懠鍐ㄦ纯**: frontend/src/api/client.ts
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈺嬬礉缁鐎烽弬顓♀枅娣囶喖顦?
**閸欐ɑ娲跨拠锔藉剰**:
- `as Record<string, unknown>` 閺€閫涜礋 `as unknown as Record<string, unknown>`閿涘本绉烽梽?TS2352 缂傛牞鐦ч柨娆掝嚖

## [2026-04-12] feat: 鐠愶箑褰块崚妤勩€冮弰鍓с仛闁喚顔?+ AI Credits 濮瑰洦鈧?

**瑜板崬鎼烽懠鍐ㄦ纯**: frontend/src/views/admin/AccountsView.vue
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娑擃參顥撻梽鈺嬬礉AccountsView 閺€鐟板З鏉堝啫顦块敍灞芥値楠炶埖妞傚▔銊﹀壈
**閸欐ɑ娲跨拠锔藉剰**:
- 鐠愶箑褰块崥宥囆炴稉瀣煙閺勫墽銇氶柇顔绢唸閿涘苯鍚嬬€?`credentials.email`閿涘湏ntigravity閿涘鎷?`extra.email_address`閿涘湏nthropic閿?
- 缁涙盯鈧鐖崣鍏呮櫠閺傛澘顤?AI Credits 濮瑰洦鈧粯鐖ｇ粵鎾呯礉瀵倹顒為懢宄板絿楠炶埖瀵滈柇顔绢唸閸樺鍣?
- `load()` 閸?`reload()` 閸у洩袝閸欐垶鐪归幀璇插煕閺?

## [2026-04-12] feat: 閹兼粎鍌ㄩ弨顖涘瘮閹稿鍋栫粻杈ㄧ叀閹垫崘澶勯崣?

**瑜板崬鎼烽懠鍐ㄦ纯**: backend/internal/repository/account_repo.go
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈺嬬礉閹兼粎鍌ㄩ弶鈥叉閹碘晛鐫?
**閸欐ɑ娲跨拠锔藉剰**:
- 鐠愶箑褰块幖婊呭偍娴犲簼绮庨崠褰掑帳 `name` 閹碘晛鐫嶆稉鍝勬倱閺冭泛灏柊?`credentials.email` 閸?`extra.email_address`閿涘牅濞囬悽?sqljson.StringContains閿?

## [2026-04-12] fix: Antigravity refresh_token 閺堫亙绻氱€涙ê顕遍懛纾嬪閸欒渹绗夐崣顖濈殶鎼?

**瑜板崬鎼烽懠鍐ㄦ纯**: backend/internal/service/antigravity_oauth_service.go
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈺嬬礉閸ョ偛锝為柅鏄忕帆
**閸欐ɑ娲跨拠锔藉剰**:
- `ValidateRefreshToken` 閸掗攱鏌婇崥?Google 娑撳秷绻戦崶鐐存煀 refresh_token閿涘苯顕遍懛鏉戠摠閸?credentials 娑撹櫣鈹?
- 閺傛澘顤冮崶鐐诧綖闁槒绶敍姘洤閺嬫粌鍩涢弬鏉挎惙鎼存柧鑵?refresh_token 娑撹櫣鈹栭敍灞煎▏閻劎鏁ら幋铚傜炊閸忋儳娈戦崢鐔奉潗閸?

## [2026-04-12] feat: 閹靛綊鍣虹€电厧鍙嗛弨顖涘瘮娴ｈ法鏁ら柇顔绢唸娴ｆ粈璐熺拹锕€褰块崥宥囆?

**瑜板崬鎼烽懠鍐ㄦ纯**: frontend/src/components/account/CreateAccountModal.vue, frontend/src/i18n/locales/zh.ts, en.ts
**娑撳﹥鐖堕崗鐓庮啇閹?*: 娴ｅ酣顥撻梽鈺嬬礉閺傛澘顤?UI 闁銆?
**閸欐ɑ娲跨拠锔藉剰**:
- 閺傛澘顤?`useEmailAsName` 闁銆嶉敍灞肩矌 Antigravity 楠炲啿褰撮崣顖濐潌
- 閸曢箖鈧鎮楅梾鎰閸氬秶袨鏉堟挸鍙嗗鍡礉閹靛綊鍣洪崪灞藉礋娑?OAuth 閸掓稑缂撻崸鍥﹀▏閻劑鍋栫粻鍙樼稊娑撳搫鎮曠粔?
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

## [2026-07-11] feat: Add bounded API-key concurrency sorting

**Affected files**: API-key repository/service, user key table, API contract, and focused tests.

**Compatibility**: Resource-bounded adaptation of upstream `5debe1db3`; ordinary database-backed pagination/sorts remain unchanged.

**Details**:
- `sort_by=current_concurrency` loads the filtered key set, obtains Redis counts in batches of 500, applies stable concurrency/ID ordering, and then paginates.
- The expensive sort is capped at 10,000 filtered keys; larger sets receive a typed bad request instead of unbounded Ent/Redis/memory work.
- Latest-use IP enrichment runs only for the final page after sorting, preserving the fork's IP column without querying usage logs for the whole candidate set.
- Existing search/status/group filters, column preferences, quotas, auth cache, concurrency admission, billing/display/cache-read behavior, and normal database sorts are unchanged.

## [2026-07-11] feat: Improve sidebar home navigation and scroll continuity

**Affected files**: shared app sidebar, app UI store, and focused frontend contract tests.

**Compatibility**: Low-risk adaptation of upstream `20008264f` and `c7e44a83a`; no route definitions or public-setting contracts changed.

**Details**:
- The sanitized custom/default logo and site name now link admins to `/admin/dashboard` and regular users to `/dashboard`, while preserving mobile menu close behavior.
- The actual sidebar navigation container saves its in-memory scroll offset before unmount and restores it after remount, without persisting account data or changing public-settings caching.
- Existing custom SVG colors, sanitized logo URLs, nested menu expansion, feature flags, i18n menu labels, and route guards remain unchanged.

## [2026-07-11] fix: Batch Ops statistics, group capacity, and Redis slot cleanup

**Affected files**: backend repository/service/admin handler Ops, group-capacity and concurrency-cache files, focused tests, and Ops documentation.

**Compatibility**: Selectively adapted upstream `f3a3a0869`, `3f2ef6046`, and `72ccd1b11` without schema, migration, frontend, route, or setting changes.

**Details**:
- Periodic account-slot cleanup scans existing `concurrency:account:*` sorted sets instead of loading every schedulable database account; user slots and wait counters are outside the pattern.
- Realtime Ops statistics use a group-filtered lightweight account projection; canceled client/database requests end silently instead of writing a second error response.
- All-group capacity uses one active-ID query, one schedulable account projection, and batched concurrency/session/RPM reads. Empty groups remain visible and shared accounts contribute independently to each bound group.
- Capacity SQL preserves current soft-delete, active/schedulable, temporary-pause, expiry auto-pause, overload and rate-limit filters. Spark shadow capacity remains eligible; billing/display/cache-read and scheduler score/failover behavior are unchanged.

## [2026-07-11] feat: Make initial migration timeout configurable

**Affected files**: setup configuration/tests, deploy environment example, and four supported Compose variants.

**Compatibility**: Low-risk adaptation of upstream `36d5f4e4c`; no migration content, runtime config schema, image source, or deployment execution changed.

**Details**:
- `SETUP_MIGRATION_TIMEOUT_SECONDS` controls only the initial `ApplyMigrations` context. Unset, invalid, zero, or negative values keep the 60-second default.
- The variable is documented and forwarded by dev, local, standalone, and production Compose files, all while retaining the fork's GHCR production image path.
- Current migrations including Spark `188/189` and peak-rate `190` are unchanged; no service was started, pushed, or deployed.

## [2026-07-11] feat: Add guarded OpenAI quota auto-pause thresholds

**Affected files**: OpenAI scheduler/sticky/snapshot filtering, Ops settings/cache/Wire, account/Ops admin UI, usage-window help text, bilingual i18n, and focused tests.

**Compatibility**: Medium-risk selective adaptation of upstream `ead471d64`, `8b7a82270`, `c9caadb37`, and tooltip portion of `c256a5441`; no schema or migration.

**Details**:
- OpenAI parent accounts can be skipped when persisted upstream 5h/7d usage reaches an account or global threshold. Global defaults are disabled at zero; each account can override or explicitly exempt either window.
- Checks run before TopK and at sticky, previous-response, and fresh DB rechecks. Expired windows fail open so traffic can refresh stale usage, while bindings remain available for later resumption.
- Spark shadows are explicitly excluded and keep their independent quota dimension. The policy does not mutate `schedulable`, fabricate cooldown timestamps, alter billing/display/cache-read data, or change Images/Batch Image and Claude-GPT behavior.
- Ops settings reuse existing JSON KV with non-blocking stale-while-revalidate caching; account overrides reuse `extra`. Unrelated `eba204632` OAuth/privacy changes were intentionally not adopted.

## [2026-07-11] fix: Reconcile merged locale patch coverage

**Affected files**: final Chinese and English locale patch objects.

**Compatibility**: UI-only; no runtime billing or scheduling behavior changed.

**Details**:
- Restored final runtime paths for multi-file account selection, scheduler-score help, ungrouped scores, used quota, and peak-rate settings.
- Kept the keys in the final recursive locale patches so duplicate historical locale sections cannot hide them.
- Verified sidebar, public URL sanitization, and global runtime locale coverage (8 tests).

## [2026-07-11] docs: Record upstream exclusions and permanent migration gap

**Affected files**: upstream-sync ledger and architecture migration rules.

**Compatibility**: Documentation-only; no runtime behavior changed.

**Details**:
- Recorded the privacy, deployment-provenance, and existing-release-workflow reasons for excluding upstream IP geolocation, online binary rollback, and exact-tag runtime resolution.
- Corrected the usage-log ledger to reflect that API-key latest-IP row close/iteration handling is present.
- Marked migration number `183` as a permanent historical gap. New migrations continue from the current maximum and never backfill an already published gap.

## [2026-07-11] chore: Complete the existing Wire CLI checksum set

**Affected files**: backend Go module checksums only.

**Compatibility**: Dependency metadata only; no dependency version or runtime graph changed.

**Details**:
- Added the missing `github.com/google/subcommands v1.2.0` checksums required by the already pinned Wire `v0.7.0` CLI.
- Wire now starts and reports the repository's existing handwritten-provider gaps instead of failing before analysis. The checked-in `wire_gen.go` remains unchanged and passes `cmd/server` unit tests and the production-style server build.

## [2026-07-11] feat: Normalize Anthropic OAuth client dateline fingerprints

**Affected files**: Anthropic fingerprint helper, gateway request transform, Settings KV/admin DTO and UI, bilingual locales, API contracts, focused tests, and gateway documentation.

**Compatibility**: Selective adaptation of upstream `59e9356c5`. Default-on and explicitly disableable; no schema or migration.

**Details**:
- Normalizes four apostrophe variants and slash date separators in the specific `Today's date is YYYY-MM-DD.` system sentence.
- Message content is scanned only inside `<system-reminder>` tags. User prose, tool input/results, invalid JSON, and mixed separators remain byte-identical.
- Scope is limited to Anthropic OAuth/Setup Token. API Key, non-Anthropic, OpenAI Claude-GPT bridge, Images, Batch Image, scheduler/failover, billing/display-token accounting, real cache-read quantities, and stored `actual_cost` are unchanged.
- Added a default-true admin Settings KV toggle with bilingual UI. The setting is not public and adds no route.
- Verified focused Go packages, admin/API settings contracts, 20 frontend settings/i18n tests, typecheck, and `git diff --check`.

## [2026-08-10] fix: Render completed Codex image results without saved paths

**Affected files**: Codex image bridge context, OpenAI Responses streaming/non-streaming handling, focused gateway tests, and gateway documentation.

**Compatibility**: Response-side fallback only for Codex requests with the image bridge enabled. No route, setting, schema, migration, billing, scheduler, native Images, or WebSocket change.

**Details**:
- Preserve the upstream `image_generation_call` and add an assistant `output_text` message containing a Markdown image data URI when a completed result has supported PNG, WebP, or JPEG bytes but no equivalent renderable message.
- Streaming emits the synthetic `response.output_item.done` message before the successful terminal event, uses valid SSE blank-line framing, and includes the same item in terminal `response.output`.
- SSE-to-JSON conversion merges image items into an already non-empty terminal output before creating the fallback message. Existing data URIs are deduplicated; failed, empty, unknown-format, and generic Responses outputs remain unchanged.
- Focused Codex image bridge and existing image-status regression tests pass. The full `internal/service` unit package remains blocked by pre-existing AuthService SQLite fixtures missing `users.display_cache_token_max_mult`; the failure is outside the affected OpenAI gateway files.

## [2026-08-10] chore: Protect Codex image rendering fallback during upstream sync

**Affected files**: `backend/tools/upstream-sync-guard/{main.go,main_test.go}`, `docs/dev/UPSTREAM_SYNC.md`, `docs/dev/codebase/gateway.md`, and `docs/dev/CHANGELOG_CUSTOM.md`.

**Compatibility**: Guard and documentation only. No image response implementation, route, setting, schema, migration, billing, scheduler, native Images, WebSocket, push, or deployment change.

**Details**:
- Recorded the real Codex Desktop failure mode, local RED/GREEN commits (`c95b45a97`, `2167a9e65`), and the user's successful 2026-08-10 client verification as an authoritative fork-local contract.
- Defined an explicit upstream replacement gate covering visible client rendering, streaming, non-streaming, SSE-to-JSON, existing-text merge, deduplication, supported image formats, and failure/unknown-format boundaries.
- Added the guard regression in `21e50fa7b` and the matching critical-signature catalog in `0bf08d3f4`, protecting the image-bridge response context, assistant-message synthesis and MIME helpers, and representative tests from silent removal during future syncs.









