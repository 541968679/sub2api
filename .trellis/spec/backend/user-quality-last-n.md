# User Quality Last-N

## Scenario: admin user-list quality is last-N keyed by user, with a per-user window N

### 1. Scope / Trigger

- Trigger: admin user list and smart-schedule header user row show last-N quality for **this user across all accounts** (\(Q_u\)).
- Same two FIFO windows and P50/P95 math as \(Q_a\). Window N is **per-user**: `users.quality_window_n` (1–100) overrides; `NULL` inherits site `account_quality_window_n` (default 20). This is not a second site-wide Settings knob and not smart-schedule pair N.
- Ingest on the same completion hooks as account last-N. `user_id` missing → skip \(Q_u\); \(Q_a\) still uses site N.
- User list batch must not use 15-minute SQL `GetUserQualityStatsBatch`.
- Click opens `UserQualityDialog` (curve + failover/bridge + edit N). Combined user cell shows p50, **p95**, success, failover, k/N. No hard-close, pause, or pair-quality.

### 2. Signatures

- Redis `user-quality:last-n:{userID}` — account last-N JSON plus optional `override_n`. TTL 7d.
- `users.quality_window_n` INT NULL (migration 212).
- `PUT /admin/users/:id` `quality_window_n`: omit unchanged; `0`/empty inherit; `1–100` override. Save resizes Redis FIFO immediately.
- `POST /api/v1/admin/users/quality-stats/batch` `{ user_ids }` → last-N `stats[id]` with this user's resolved `n` / `window_n` / `account_quality_window_n`.
- `GET /api/v1/admin/users/:id/quality-history?from=&to=` — same range rules as account history.
- `UserQualityLastNCache` (`IngestUserLastN` + `ResizeUserLastN`) / `GetUserLastNStatsBatch` / `ApplyUserQualityWindowN`.

### 3. Contracts

- Failover inclusion uses the same `schedule_use_failover_error_rate` ingest switch as \(Q_a\).
- Empty windows are not snapshotted. Cache miss → empty stats + stamped resolved N, not 15-minute SQL.
- Changing site N updates \(Q_a\) and inheriting \(Q_u\) users only. Override users stay on their N.
- Pair \(Q_{a,u}\) and account hard-close stay unchanged. Smart-schedule soft cooldown has its own per-cooling-pair window; do not reuse \(Q_u\) as that sample source.

### 4. Validation & Error Matrix

| Condition | Error |
| --- | --- |
| invalid user id | 400 Invalid user ID |
| invalid `from` / `to` | 400 Invalid from/to |
| `to-from` > 7d or `from` after `to` | 400 via `NormalizeAccountQualityHistoryRange` |
| maintenance service nil | 503 User quality history / service unavailable |

### 5. Good / Base / Bad Cases

- Good: user A N=10 and user B inherit 20 never share a window; both users on one account share that user's window.
- Base: no last-N key → empty rates + resolved N stamped; history `items: []`.
- Bad: list batch calling 15-minute SQL; user dialog fetching account quality-history; ingesting \(Q_u\) with site N when the user has an override.

### 6. Tests Required

- A/B isolation; override vs inherit stamp; resize FIFO on save; failover counted in \(W_{ok}\); batch last-N not 15m SQL.
- History default 24h; max 7d rejected.

### 7. Wrong vs Correct

- Wrong: bind every user's \(Q_u\) to site `account_quality_window_n`; 15-minute SQL as list truth; `failover_error_count` forced to 0.
- Correct: per-user override or inherit; FIFO math shared with \(Q_a\); key = `user_id`; history from `user_quality_snapshots`.
