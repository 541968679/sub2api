# Account Quality Snapshots

## Scenario: persist the live last-N account quality window \(Q_a\)

### 1. Scope / Trigger

- Trigger: admin needs a time series of the **same** last-N TTFT / success-rate window shown on the account list (\(Q_a\), all users on that account).
- Source of truth is Redis `account-quality:last-n:{accountID}` (two FIFO windows, site-wide N default 20). Completions (usage success + counted ops errors) ingest and recompute immediately. The 5-minute tick only snapshots current last-N into history and may re-run hard-close. It must **not** recompute from a 15-minute SQL window or `Replace` live keys from `GetAccountQualityStatsBatch`.
- User-dimension `users/quality-stats/batch` still uses the 15-minute SQL path. Pair cooldown stays on \(Q_{a,u}\).
- Adjacent snapshot points may look similar when traffic is quiet. Empty windows (no success, no error, no TTFT) are not stored. last-N does not maintain `bridge_*`.
- Hard-close evaluation must use the **live** last-N stats (ingest path or tick fallback), not snapshot rows. Attach via `SetHardCloseEvaluator`.
- Account overlay writes go through `UpdateAccountExtra` (JSONB merge of `extra.quality_hard_close` only). `UpdateAccount` must preserve that key when the edit form replaces Extra, same as `quota_used`.

### 2. Signatures

- Table `account_quality_snapshots`: unique `(account_id, captured_at)`; `captured_at` truncated to 5-minute UTC; `window_seconds` may still be 900 (display uses N). Redis last-N key TTL 7d.
- `AccountQualitySnapshotInterval = 5m`, `AccountQualitySnapshotRetention = 7d`, `AccountQualityHistoryDefaultRange = 24h`, `AccountQualityHistoryMaxRange = 7d`, batch cap 200.
- `GET /api/v1/admin/accounts/:id/quality-history?from=&to=` (RFC3339, admin auth).
- `AccountQualityHardCloseEvaluator.EvaluateHardClose(ctx, stats map[int64]*AccountQualityStats)`
- `AccountQualityMaintenanceService.SetHardCloseEvaluator(eval)`

### 3. Contracts

History response:

```json
{
  "items": [
    {
      "captured_at": "2026-08-14T07:00:00Z",
      "window_seconds": 900,
      "success_count": 10,
      "error_count": 1,
      "success_rate": 0.909,
      "avg_ttft_ms": 400,
      "p50_ttft_ms": 300,
      "p95_ttft_ms": 900,
      "max_ttft_ms": 1200,
      "ttft_samples": 10
    }
  ],
  "from": "2026-08-13T07:05:00Z",
  "to": "2026-08-14T07:05:00Z"
}
```

Omitted `from`/`to` → `to=now`, `from=to-24h`. Normalize the range once and use that window for both query and response. TTFT / `success_rate` stay null when that snapshot had no applicable samples (error-only rows).

Maintenance tick: leader lock `account-quality:maintenance:leader` → SCAN `account-quality:last-n:*` → chunk 200 → project last-N to `AccountQualityStats` (stamp `n` / `window_n` / `account_quality_window_n`) → upsert non-empty → delete `captured_at < now-7d` in batches of 500 → `EvaluateHardClose(liveStats)` (no-op if unset). Do **not** `Replace` live keys from SQL. Ingest writes last-N and a live JSON projection (no resume fields). Selection `Get` prefers last-N, else falls back to legacy live JSON, then merges the resume HASH. Resume HASH keys must survive even when the account is not in this tick's last-N set.

### 4. Validation & Error Matrix

| Condition | Error |
| --- | --- |
| invalid account id | 400 Invalid account ID |
| invalid `from` / `to` | 400 Invalid from/to |
| `to-from` > 7d or `from` after `to` | 400 via `NormalizeAccountQualityHistoryRange` |
| maintenance service nil | 503 Account quality history unavailable |

### 5. Good / Base / Bad Cases

- Good: account with mixed success/error over 24h returns 5m points of the last-N window at each `captured_at`.
- Base: no rows in range → `items: []` plus normalized `from`/`to`.
- Bad: 8-day range rejected; all-zero stats not upserted; tick must not overwrite live from 15-minute SQL.

### 6. Tests Required

- Empty-sample skip; error-only null TTFT; 200-id last-N chunking; upsert idempotent on `(account_id, captured_at)`.
- History default 24h `from`/`to`; max 7d rejected; invalid timestamps 400.
- `EvaluateHardClose` no-op without evaluator; called with last-N live stats after upsert; tick does not `Replace` live from SQL.

### 7. Wrong vs Correct

- Wrong: treat each snapshot as a disjoint 5-minute request bucket, re-aggregate history from `usage_logs` in the handler, or let the tick `Replace` live from a 15-minute SQL window.
- Correct: each point is the live last-N \(Q_a\) at `captured_at`; the chart reads the snapshot table only; ingest is the write path for live.
