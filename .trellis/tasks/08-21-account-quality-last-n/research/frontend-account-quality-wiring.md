# Frontend wiring: account quality last-N (Q_a)

Date: 2026-08-21. Backend contract file was not present when this landed. Field name used: `account_quality_window_n` (1–100, default 20). If `research/account-quality-api-contract.md` appears later, backend field names win.

This is **not** smart-schedule pair N (`quality_window_n`, default 10). Pair column / 豁免期 copy was left in place; only the “not account 15-minute quality” contrast was updated.

## Settings (wired)

| Call | Path | Field |
| --- | --- | --- |
| `getQualityHardCloseSettings()` | `GET /admin/settings/quality-hard-close` | read `account_quality_window_n`, else `min_success_samples`, else `min_ttft_samples`, else 20 |
| `updateQualityHardCloseSettings()` | `PUT /admin/settings/quality-hard-close` | write `account_quality_window_n` **and** echo both `min_success_samples` / `min_ttft_samples` as the same N |

UI: one input on the quality hard-close card. Dual “最少成功样本 / 最少 TTFT 样本” removed.

## Grid + dialog (wired, existing endpoints)

| Call | Path | Used by |
| --- | --- | --- |
| `getBatchQualityStats(accountIds)` | `POST /admin/accounts/quality-stats/batch` | account list + smart-schedule 账号质量 column |
| `getQualityHistory(id)` | `GET /admin/accounts/:id/quality-history` | stability chart |
| `getQualityHardClose(id)` / `updateQualityHardClose(id)` | `GET/PUT /admin/accounts/:id/quality-hard-close` | overlay; one N, echoed onto both legacy sample floors |

`AccountQualityStats` still accepts `window_seconds`. Optional last-N aliases on the same object: `account_quality_window_n`, `window_n`, `n`. Cell shows k/N, p50, success rate. N on the grid also comes from the settings GET if the batch payload has no N yet.

## User-schedule account gates (wired, copy + one N)

Account-side user quality gates (EditAccountModal / AccountUserScheduleCell) now have one N input. Save still sends `quality_min_success_samples` and `quality_min_ttft_samples` as that N. This is Q_a judging, **not** `quality_window_n` on smart-schedule.

## Overlay N (locked)

Backend stores overlay N / old min-sample fields but `resolved` always uses the site-wide window. The cheaper product is: **global N is the window**. The stability dialog shows N read-only and writes overlay sample/N fields as null. “Save template” keeps the current global N (does not copy a per-account or pair N into `account_quality_window_n`).

## Pending / leftover

- Batch/history may start echoing `window_n` / drop `window_seconds`. Frontend already reads aliases; copy no longer assumes 900s.
- Resume/fail-open chips still use `ACCOUNT_QUALITY_WINDOW_SECONDS = 900` (time-based 已恢复). Not 豁免期. Backend may change this independently.
- Admin user-list quality (`admin.users.quality`) still says 15 minutes. That is user-scoped quality, not Q_a.
- Snapshot cadence (“5-minute tick”) is no longer claimed in account-quality copy; actual persist interval is backend-owned.
- User-gate N is a judging sample floor on the shared \(Q_a\) window, not a per-user window size.
- Smart-schedule “Apply template” still copies account-template N into pair `quality_window_n` (probe files; leave alone).
