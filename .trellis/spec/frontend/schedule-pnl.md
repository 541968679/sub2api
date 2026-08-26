# Admin Schedule PnL Cells

## Scenario: pool + user-list schedule profit

### 1. Scope / Trigger

- Trigger: admin smart-schedule pool and users list now show this-user × this-account (or enabled-pool total) profit instead of account-wide usage windows.
- Do not reuse `AccountUsageCell` on the smart-schedule pool page. Do not change AccountsView usage cells.

### 2. Signatures

- `POST /admin/users/smart-schedule/pnl/summaries` `{ user_ids }` + `timezone`
- `POST /admin/users/:id/smart-schedule/pnl/pairs` `{ account_ids }` + `timezone`
- `GET /admin/users/:id/smart-schedule/pnl/trend?range=24h|today|yesterday|7d&account_id=&timezone=`
- Column key: `schedule_pnl`, inserted after `smart_schedule`.
- Cells: `SmartSchedulePnlCell`, `UserSchedulePnlCell`, shared `SchedulePnlTrendDialog`.
- Formatters: `frontend/src/composables/schedulePnl.ts`.

### 3. Contracts

- Column key `claude_gpt_bridge` is a separate read-only extra column (`account.extra.openai_claude_gpt_bridge_enabled`). It must not reuse `schedule_pnl` fetch, cells, or formatters.
- Pool `SmartSchedulePnlCell` shows today revenue / cost / profit / margin. It does not render the 7-day **PnL** window. Users-list `UserSchedulePnlCell` still shows today + 7-day.
- API-key / non-oauth rows still show readable upstream balance (`extra.upstream_balance_usd`, which is New API wallet+subscription remaining when both were probed). OAuth rows replace that balance line with a compact 7-day **quota** bar (`UsageProgressBar`) read from cached account extra (`passive_usage_7d_*` or OpenAI `codex_7d_*`). No `/usage` fetch and no `AccountUsageCell`. Missing snapshot renders `—`.
- Pair `SchedulePnlTrendDialog` (has `account`) may show wallet / subscription remaining from `extra.upstream_balance_wallet_usd` / `upstream_balance_subscription_usd`. User-level dialog (no `account`) stays chart-only. Do not split those amounts on `AccountUsageCell`.
- When a non-oauth row has readable `extra.upstream_balance_usd` / `usage.balance_usd` and `extra.burn_samples` (kind `balance_usd`) can fit a rate, the pool cell compares that balance burn $/h with today account `actual_cost` implied $/h (`todayStats.cost` / elapsed local hours). Compact `对齐` / `偏离` cue only. OAuth rows omit the burn cue. Display-only; do not change stored billing.
- Balance is optional extra, not a substitute for PnL. Missing balance or insufficient samples omit the burn cue.
- `null` window / `null` metric renders `—`. Do not coerce to `$0.00` or `0.0%`.
- Users without an enabled platform pool stay `—`.
- Existing users-list `usage` column stays account-wide `actual_cost`.
- Pool load: `getBatchQualityStats` / `getBatchTodayStats` / `getSmartSchedulePnlPairs` each `.catch` to empty. A PnL 500 must not fail the account list.
- Trend empty buckets stay `null`, not 0.

### 4. Validation & Error Matrix

| Condition | UI |
| --- | --- |
| Summary missing or both windows null | `—` |
| Revenue 0 (margin null) | profit shown; margin `—` |
| PnL request fails during pool load | pool still renders; PnL cells `—` |
| Invalid trend range | dialog error from API; do not invent points |

### 5. Good / Base / Bad Cases

- Good: click pair cell or user `schedule_pnl` opens the same dialog; pair pass `account_id`, user total omits it.
- Base: post-deploy with no new rows → every cell `—`.
- Bad: `Promise.all([listed, pnl])` without isolating `pnl`.
- Bad: putting `schedule_pnl` before `smart_schedule` or replacing the usage column.

### 6. Tests Required

- `schedulePnl.spec.ts` formatters, empty-window helpers, and balance-vs-cost burn compare.
- `SmartSchedulePnlCell.spec.ts` today-only layout, API-key balance, oauth 7-day quota bar, and burn cue.
- `adminUserListRow.spec.ts` column order.
- `useUserSmartScheduleEditor.spec.ts` PnL failure isolation.
- `UserSmartScheduleView.spec.ts` / `UsersView.spec.ts` cell + dialog wiring.
- `AccountUsageCell` specs unchanged.

### 7. Wrong vs Correct

**Wrong**: Show account-wide `A$` / `U$` / req / token on the smart-schedule pool page.

**Correct**: `SmartSchedulePnlCell` with this-user × this-account today revenue/cost/profit/margin. API-key rows keep readable upstream balance and the optional burn cue. OAuth rows swap the balance line for the cached 7-day quota bar.

**Wrong**: Fail the whole pool load when PnL returns 500.

**Correct**: Catch secondary requests independently; listed accounts still render.
