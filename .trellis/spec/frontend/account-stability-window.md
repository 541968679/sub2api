# Account Stability Window

## Scenario: open quality history and per-account hard-close from the account list

### 1. Scope / Trigger

- Trigger: admin clicks the account-list combined quality cell, or the row **稳定性** action. Combined account cells show p50 **and display p95** (same `AccountQualityCell` as the user list). Display p95 is not a hard-close or scheduling input.
- Users list / smart-schedule header quality cells are clickable (`AccountQualityCell` `subject="user"` `mode="combined"`). Combined user cells show p50 **and p95**. They open `UserQualityDialog` (last-N curve + failover/bridge + per-user window N; **no** hard-close form). History is `GET /admin/users/:id/quality-history`, never account quality-history.
- Chart reads snapshot history (overlapping last-N windows). The account stability dialog **defaults to hiding the p95 series** (`readShowP95Preference` is false until localStorage `sub2api.account-stability.show-p95=1`). That default stays; this task only unhides p95 on the combined cell, not the curve. The live last-N Claude→GPT bridge error rate is shown in this dialog only (not `AccountQualityCell`). Form writes account overlay only. Global master switch lives on Settings → Gateway.
- Shared template is the same KV as Settings → Gateway. Dialog **保存模板** overwrites that one template (does not change the master switch). **应用模板** copies it into the form; account save is separate. There is no live `use_global` bind. The template is site-wide gates only — it must not include smart-schedule `soft_cooldown` (that flag lives on the user×platform policy).
- User-schedule quality-gate forms reuse the same threshold fields (p50 / success rate / samples / condition). Save/apply is not bound to a `user_id`. User-gate save keeps `enabled` and `pause_minutes`. Apply only fills the current user’s form; account / `user_quality_gate_patch` save is separate.
- Per-user quality gates are edited in the account user-schedule cell / edit modal, not this dialog.

### 2. Signatures

- `GET /admin/accounts/:id/quality-history` → `{ items, from, to }`
- `GET /admin/accounts/:id/quality-hard-close` → `{ overlay, resolved, global_enabled }`
- `PUT /admin/accounts/:id/quality-hard-close` body = overlay **top-level**
- `GET/PUT /admin/settings/quality-hard-close`
- `isQualityHardCloseReason(reason)` / `isQualityHardClosePaused(until, reason)` in `frontend/src/utils/accountQualityHardClose.ts`
- Reason prefix `quality_hard_close`

### 3. Contracts

- Success rate: UI percent, API `0–1` via `percentToSuccessRate` / `successRateToPercent`.
- Account save writes `use_global: false` plus the form thresholds. Saving settings / 保存模板 must not enable any account.
- Empty history → EmptyState. Live bridge error rate uses `POST /admin/accounts/quality-stats/batch` for the open account; no samples → `—`, never `0%`. Display-only; not a hard-close input. Quality pause banner when reason prefix matches and until is future. Banner **立即恢复调度** calls `recoverState` (same as the account action menu) and emits `recovered`; it does not wait for `pause_minutes`. Next maintenance tick may pause again if the live window still misses.
- Do not add a second full form to `EditAccountModal`.

### 4. Validation & Error Matrix

| Condition | UI |
| --- | --- |
| history empty | EmptyState, no chart |
| global_enabled false | hint on dialog; account save still allowed |
| save overlay fails | `useAppStore` error toast |
| settings card Enter in page form | submits page-level settings, not this card (same as 529 card) |

### 5. Good / Base / Bad Cases

- Good: click 首字 on an account with snapshots → dialog titled with name, p50/success lines; p95 stays off until the show-p95 toggle.
- Base: click `—` still opens dialog (empty chart + form).
- Bad: UsersView quality cell opens `AccountStabilityDialog` or account quality-history; settings save does not PUT any account overlay.

### 6. Tests Required

- Cell clickable emit; default not clickable.
- Dialog: empty chart; history series; overlay PUT shape; pause banner; live bridge rate / empty dash.
- AccountsView opens `AccountStabilityDialog`; UsersView / header row open `UserQualityDialog`.
- Settings card save percent → 0–1; page save does not write hard-close.

### 7. Wrong vs Correct

- Wrong: wrap PUT body as `{ overlay: {...} }`, or send success rate as 90 instead of 0.9.
- Correct: top-level overlay fields; convert percent at the form boundary.
