# Account Quality Hard Close

## Scenario: opt-in pause when live last-N account quality \(Q_a\) breaches thresholds

### 1. Scope / Trigger

- Trigger: admin wants to auto-pause an account for N minutes when the **live last-N** account quality window \(Q_a\) (same stats as the account list cells) crosses configured p50 / success-rate limits.
- \(Q_a\) is site-wide last-N (default **20**, clamp 1–100), all users on that account. It is not the smart-schedule pair window \(Q_{a,u}\).
- Default OFF at both global and account layers. Both must be enabled or evaluation is a no-op.
- Pause uses `SetTempUnschedulable` + reason prefix `quality_hard_close`. Do not add a pause column or flip `schedulable`.
- If `TempUnschedulableUntil` is still in the future, skip (cooldown-once; do not overwrite 529/401).
- Evaluate on each completed ingest (and as a tick fallback) using the same last-N live stats as the grid, not snapshot rows or a 15-minute SQL window.

### 2. Signatures

- Settings KV `quality_hard_close_settings` (`SettingKeyQualityHardCloseSettings`).
- Account extra key `quality_hard_close` (`AccountExtraQualityHardClose`).
- `GET/PUT /api/v1/admin/settings/quality-hard-close`
- `GET/PUT /api/v1/admin/accounts/:id/quality-hard-close`
- `EvaluateAccountQualityHardClose(stats, resolved, alreadyPaused) (shouldPause, reason)`
- `ProvideAccountQualityMaintenanceService` must `SetHardCloseEvaluator(...)`.
- Pair `ToAccountQualityStats` must keep projecting only p50 / success / sample counts. Do **not** copy pair display `P95TTFTMs` into `AccountQualityStats` (hard-close still ignores p95; leaking it would invite a later reader).

### 3. Contracts

Global body (`QualityHardCloseSettings`):

```json
{
  "enabled": false,
  "max_p50_ttft_ms": 3000,
  "min_success_rate": 0.9,
  "pause_minutes": 30,
  "account_quality_window_n": 20,
  "window_n": 20,
  "n": 20,
  "min_success_samples": 20,
  "min_ttft_samples": 20,
  "condition": "or"
}
```

Canonical N is `account_quality_window_n` (aliases `window_n` / `n`). GET echoes `min_success_samples` and `min_ttft_samples` to the same N. Parse order: explicit N → `min_success_samples` → `min_ttft_samples` → 20. Do not `min(old success, old TTFT)`. Overlay sample / N fields do not change the site-wide window.

Null `max_p50_ttft_ms` / `min_success_rate` = metric not configured. Not exposed on public settings.

Account hard-close after GET/Resolve always has both sample floors = N. User-quality-gate settings without an explicit account N keep independent floors (default success 20 / TTFT 10). Pair cooldown stays on \(Q_{a,u}\).

Account GET returns `{ overlay, resolved, global_enabled }`. Account PUT body is the overlay **top-level** (not wrapped in `overlay`):

```json
{
  "enabled": false,
  "use_global": true,
  "max_p50_ttft_ms": null,
  "min_success_rate": null,
  "pause_minutes": null,
  "min_success_samples": null,
  "min_ttft_samples": null,
  "condition": null
}
```

`use_global` omitted still defaults true for old rows. New UI always saves `use_global: false` with explicit thresholds. PUT merges only `extra.quality_hard_close`. Account `Update` that replaces Extra must preserve this key (same as `quota_used`).

The global KV is the **single shared template** plus the master switch. Account dialogs save/apply that template; a new save overwrites the old template and must not flip `enabled`.

Resolve: evaluate iff `global.enabled && overlay.enabled`. `use_global=true` (legacy) → all thresholds from the shared template. Else non-null overlay fields override, null falls back to the template. `resolved.enabled` is that AND. If both metrics unresolved, skip.

Reason example: `quality_hard_close:p50=3200,success=0.82`.

### 4. Validation & Error Matrix

| Condition | Error |
| --- | --- |
| pause_minutes not in 1–1440 | 400 |
| min_success_rate not in (0,1] when set | 400 |
| max_p50_ttft_ms < 1 when set | 400 |
| samples < 1 | 400 |
| condition not `or`/`and` | 400 |
| invalid account id | 400 |

Runtime: under-sampled metrics are not judged. `or` = any judged breach. `and` = all judged metrics breached. Zero judged metrics → no pause. Clamp pause minutes 1–1440 before writing until.

### 5. Good / Base / Bad Cases

- Good: both layers on, 25 successes + 5 errors, success_rate 0.83, min 0.90, or → pause 30m.
- Base: global off or account off → no pause even if stats are terrible.
- Bad: already temp-unschedulable → skip; TTFT count `< N` → p50 not judged; Edit Account `extra:{}` must not delete overlay.

### 6. Tests Required

- Defaults off; resolved.enabled gate; min samples; or/and; unconfigured metric ignored; already-paused skip.
- Settings GET defaults / PUT round-trip / invalid reject.
- Account PUT merges only overlay key; invalid pause 400.

### 7. Wrong vs Correct

- Wrong: evaluate snapshot history, a 15-minute SQL window, or pause by setting `schedulable=false`.
- Correct: last-N live \(Q_a\) (same as the grid) + `SetTempUnschedulable` + skip if already paused. Unfilled windows do not fire that metric.

## Common Mistake: overlay N pretends to change the window

**Symptom**: account stability dialog lets an admin type a per-account N; cells / hard-close still use the site-wide window.

**Cause**: overlay JSON can store `account_quality_window_n` / old min-sample fields, but `ResolveAccountQualityHardClose` always echos the global N. The FIFO window is site-wide.

**Prevention**: keep N editable only on `GET/PUT /admin/settings/quality-hard-close`. Account overlay UI must show that N read-only and must not save overlay N as a window override. Pair `quality_window_n` stays a different field.
