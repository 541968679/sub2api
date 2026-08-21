# Frontend user-quality last-N wiring

Date: 2026-08-21  
Scope: admin user list + smart-schedule header user row. Pair-quality and account quality columns were not changed.

## Surfaces

| Surface | Before | After |
|---|---|---|
| `UsersView` | Two read-only cells: `quality_ttft` (p50/p95) + `quality_success_rate` | One `quality_ttft` column labeled 质量 / Quality. Combined clickable `AccountQualityCell` (`subject=user`) |
| `AdminUserListRowTable` (smart-schedule header) | Same two cells | Same combined clickable cell; emits `open-user-quality` |
| Account list / pool `quality_ttft` | Combined account cell → `AccountStabilityDialog` | Unchanged |
| Pair-quality | Separate cell + dialog | Unchanged |

## Cell reuse

`AccountQualityCell` gained `subject?: 'account' | 'user'` (default `account`).

- Layout stays the account combined cell: p50, success rate, failover success rate, `首字 k/N · 成功 k/N`.
- `subject=user` only swaps click/aria copy and `data-test` (`user-quality-cell-button`).
- Metric labels still use `admin.accounts.quality.*` so the cell is not a third design.

## Dialog

`AccountStabilityDialog` stays account-only (hard-close, pause, failover toggle, `GET /admin/accounts/:id/quality-history`).

Thin clone: `frontend/src/components/admin/user/UserQualityDialog.vue`

- Chart + terminal/failover rates + bridge rate + window counts.
- No hard-close form, no pause/resume, no failover toggle.
- Title: 用户质量 · {email/username}.
- Window copy: 最近 N 条（该用户、全部账号）.

## APIs

Aligned to `research/user-quality-api-contract.md` (same batch field set as account last-N; history is the user-scoped sibling of account history). `quality_success_rate` is merged away so the list matches the one-cell UI contract; saved `user-hidden-columns` still listing that key is ignored.

| Call | Path | Notes |
|---|---|---|
| Live last-N | `POST /admin/users/quality-stats/batch` `{ user_ids }` | Same `AccountQualityStats` shape: `window_n` / `account_quality_window_n` / `n`, TTFT, success, `failover_error_*`, `terminal_error_*`, bridge. N = site `account_quality_window_n`. |
| History | `GET /admin/users/:id/quality-history?from=&to=` | Same `{ items, from, to }` as account history. **Do not** call `GET /admin/accounts/:id/quality-history` with a user id. |
| Window N fallback | `GET /admin/settings/quality-hard-close` | `account_quality_window_n` when live stats omit N. |

`frontend/src/api/admin/users.ts`:

- `getBatchQualityStats` comment updated from “last 15 minutes” to last-N.
- `getQualityHistory(id)` added and exported on `usersAPI`.

If the backend sibling publishes a different history path, change only `getQualityHistory`.

## i18n

Replaced `admin.users.quality.ttftHint` / `successRateHint` (最近 15 分钟) with:

- `admin.users.columns.quality`
- `admin.users.quality.combinedHint`
- `clickToOpen` / `openShort` / `openAria` / `title` / `chartHint` / `noDataHint` / `windowScope` / `failoverTitle` / `bridgeHint` / `loadFailed`

zh + en both updated. Account quality / pair-quality strings were not rewritten.

## Column visibility

`quality_success_rate` is merged away (same pattern as account list). Visibility and batch fetch key off `quality_ttft` only. Saved `user-hidden-columns` still listing `quality_success_rate` is ignored.

## Tests

- `AccountQualityCell.spec.ts` — `subject=user`
- `UserQualityDialog.spec.ts` — user history + no account history
- `AdminUserListRowTable.spec.ts` — one cell, emit
- `adminUserListRow.spec.ts` — column keys
- `UsersView.spec.ts` / `UsersView.stability.spec.ts` — one column, click opens user dialog
- `UserSmartScheduleView.spec.ts` — header cell click opens user dialog, not account stability
