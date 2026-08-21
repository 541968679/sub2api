# User Quality Last-N

## Scenario: admin user-list quality matches account last-N, keyed by user

### 1. Scope / Trigger

- Trigger: admin user list and smart-schedule header user row show the same last-N cell as the account grid, but the population is **this user across all accounts** (\(Q_u\)).
- Same two FIFO windows and site-wide `account_quality_window_n` (default 20, 1–100) as \(Q_a\). Do not add a second settings knob.
- Ingest on the same completion hooks as account last-N (gateway / OpenAI usage success + counted ops errors). `user_id` missing → skip \(Q_u\); \(Q_a\) may still ingest.
- User list batch must not use 15-minute SQL `GetUserQualityStatsBatch`. That SQL may remain for other callers; it is not list truth.
- Click opens `UserQualityDialog` (curve + failover/bridge). No hard-close, pause, or pair-quality.

### 2. Signatures

- Redis `user-quality:last-n:{userID}` — same JSON shape as account last-N; TTL 7d.
- Table `user_quality_snapshots`: unique `(user_id, captured_at)`; 5-minute UTC truncation; 7-day retention.
- `POST /api/v1/admin/users/quality-stats/batch` `{ user_ids }` → last-N `stats[id]` with `n` / `window_n` / `account_quality_window_n`, success/error, p50, `failover_*`.
- `GET /api/v1/admin/users/:id/quality-history?from=&to=` — same range rules as account history (`NormalizeAccountQualityHistoryRange`).
- `UserQualityLastNCache` / `UserQualitySnapshotRepository` / `GetUserLastNStatsBatch` / `ListUserHistory`.

### 3. Contracts

- Failover inclusion uses the same `schedule_use_failover_error_rate` ingest switch as \(Q_a\). Do not pin `failover_error_count=0`.
- Empty windows are not snapshotted. Cache miss → empty stats + stamped N, not 15-minute SQL.
- Pair \(Q_{a,u}\) and account hard-close stay unchanged.

### 4. Validation & Error Matrix

| Condition | Error |
| --- | --- |
| invalid user id | 400 Invalid user ID |
| invalid `from` / `to` | 400 Invalid from/to |
| `to-from` > 7d or `from` after `to` | 400 via `NormalizeAccountQualityHistoryRange` |
| maintenance service nil | 503 User quality history / service unavailable |

### 5. Good / Base / Bad Cases

- Good: user A and user B completions never share a window; both users on one account share that user's window.
- Base: no last-N key → empty rates + N stamped; history `items: []`.
- Bad: list batch calling 15-minute SQL; user dialog fetching account quality-history.

### 6. Tests Required

- A/B isolation; failover counted in \(W_{ok}\); batch last-N not 15m SQL; N from `account_quality_window_n`.
- History default 24h; max 7d rejected.

### 7. Wrong vs Correct

- Wrong: second N setting, user-dimension 15-minute SQL as list truth, or `failover_error_count` forced to 0.
- Correct: same FIFO math and N as \(Q_a\); key = `user_id`; history from `user_quality_snapshots`.
