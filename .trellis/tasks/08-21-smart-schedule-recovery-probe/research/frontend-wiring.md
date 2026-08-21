# Frontend wiring: pair quality + window N

Date: 2026-08-21. Backend contract file was not present when this landed; field names follow the locked product contract. If `research/backend-api-contract.md` appears later, backend field names win.

## Policy N

- UI: one field `windowN` (1–100, default 10).
- Write (`PUT /admin/users/:id/smart-schedule/:platform`):
  - `quality_window_n` (preferred)
  - also echoes `quality_min_success_samples` and `quality_min_ttft_samples` as the same N for old backends
- Read: `resolveSmartScheduleWindowN` prefers `quality_window_n`, then `quality_window_samples`, then min of the two legacy sample fields (backend contract).

## Pair quality (wired)

| Call | Path | Used by |
| --- | --- | --- |
| `getSmartSchedulePairQualityBatch(userId, accountIds)` | `POST /admin/users/:id/smart-schedule/pair-quality` body `{ account_ids }` | pool table + `will_cool` |
| `getSmartSchedulePairQualityDetail(userId, accountId)` | `GET /admin/users/:id/smart-schedule/pair-quality/:accountId` | pair quality dialog |

Expected live row (normalized):

```ts
{ ttft_p50_ms?, success_rate?, ttft_samples, ok_samples, n }
```

Aliases accepted on read: `p50_ttft_ms`, `ttft_count`, `ok_count`, `success_samples`, `quality_window_n`, `quality_window_samples`. Batch also accepts `stats` if the backend copies the quality-stats envelope.

Detail: `{ current? | live?, snapshots: [{ captured_at | ts, ...row }], events: [{ at | ts, type }] }`.

Event `type` values: `cooldown_start` / `cooldown_end` / `resumed` (豁免期) / `selectable` / `expiry_zero`.

## Backend shipped (2026-08-21)

- Batch `POST /admin/users/:id/smart-schedule/pair-quality` and detail `GET .../pair-quality/:accountId` (plus platform-prefixed alias).
- Policy GET/PUT echo `quality_window_n` and `quality_window_samples` as the same N. Frontend still also writes both legacy min-sample fields as N.
- Pair-quality events persist on cooldown / 豁免期 / 可调度 / expiry zero.

## Not called (intentionally)

- `GET /admin/accounts/:id/quality-history` — account 15m track A only.
- No probe / concurrency-clamp APIs.

## Admission

- `will_cool` / unsaved preview use pair windows only.
- `resumed` chip = 豁免期 (`u:` and remaining `w:`).
- Clicking 可调度 clears local grace (no 15m `w:` fail-open).
