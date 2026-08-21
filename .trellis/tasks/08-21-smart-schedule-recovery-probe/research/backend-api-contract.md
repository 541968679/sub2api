# Backend API contract: pair quality + window N

Shipped 2026-08-21. Backend field names win if frontend aliases differ.

Response envelope is the usual `{ code, message, data }`. Fields below are `data`.

## Policy N

`GET/PUT /admin/users/:id/smart-schedule/:platform`

Authoritative: `quality_window_samples` (int, **1–100**, default **10**).

Also echoed / accepted as the same N:

- `quality_window_n` (frontend alias)
- `quality_min_success_samples`
- `quality_min_ttft_samples`

Explicit `quality_window_samples` / `quality_window_n` outside 1–100 → `SMART_SCHEDULE_INVALID_QUALITY`.
Missing N + one legacy sample → that value (clamped). Both legacy → `min`, then clamp. Both missing → 10.

Copy carries N. No new SQL column; both sample columns store the same N.

## Pool row (`GET /admin/users/:id/smart-schedule`)

Each `accounts[]` member includes:

```json
{
  "pair_quality": {
    "p50_ttft_ms": 120,
    "ttft_p50_ms": 120,
    "success_rate": 0.9,
    "ttft_count": 10,
    "ttft_samples": 10,
    "ok_count": 10,
    "ok_samples": 10,
    "n": 10
  },
  "will_cool": false
}
```

Canonical names: `p50_ttft_ms`, `ttft_count`, `ok_count`, `n`.
Aliases for the already-wired UI: `ttft_p50_ms`, `ttft_samples`, `ok_samples`.
`will_cool` is always present (false is not omitted). It uses pair windows + saved thresholds, not account 15m cells. 豁免期 / paused / cooling → `will_cool=false`.

## Batch (frontend primary)

`POST /admin/users/:id/smart-schedule/pair-quality`

```json
{ "account_ids": [7, 8] }
```

```json
{
  "pairs": {
    "7": { "p50_ttft_ms": 120, "ttft_p50_ms": 120, "success_rate": 0.9, "ttft_count": 10, "ttft_samples": 10, "ok_count": 10, "ok_samples": 10, "n": 10 }
  }
}
```

Empty `account_ids` returns every pool member for that user.

## Detail

Canonical: `GET /admin/users/:id/smart-schedule/:platform/accounts/:account_id/pair-quality`

Frontend alias: `GET /admin/users/:id/smart-schedule/pair-quality/:accountId` (platform inferred from the pool).

```json
{
  "account_id": 7,
  "user_id": 16,
  "n": 10,
  "live": { "p50_ttft_ms": 120, "ttft_p50_ms": 120, "success_rate": 0.9, "ttft_count": 10, "ttft_samples": 10, "ok_count": 10, "ok_samples": 10, "n": 10 },
  "current": { "p50_ttft_ms": 120, "ttft_p50_ms": 120, "success_rate": 0.9, "ttft_count": 10, "ttft_samples": 10, "ok_count": 10, "ok_samples": 10, "n": 10 },
  "snapshots": [
    {
      "ts": 1755740000,
      "captured_at": "2026-08-21T03:00:00Z",
      "p50_ttft_ms": 120,
      "ttft_p50_ms": 120,
      "success_rate": 0.9,
      "ttft_count": 10,
      "ttft_samples": 10,
      "ok_count": 10,
      "ok_samples": 10,
      "n": 10
    }
  ],
  "events": [
    { "ts": 1755740000, "at": "2026-08-21T03:00:00Z", "type": "cooldown_start", "until": 1755740900, "detail": "" }
  ]
}
```

`live` and `current` are the same snapshot. `ts` is unix seconds; `captured_at` / `at` are RFC3339.

Event `type`: `cooldown_start` | `cooldown_end` | `resumed` | `selectable` | `expiry_zero`.

## Admission states (unchanged API)

`POST /admin/accounts/:id/smart-schedule-resume` `{ "user_id", "state?" }`

- `resumed` = 豁免期 (UI copy). Zeros both windows + existing `u:`/`w:` time grace. No evaluate during grace; completions still ingest.
- `selectable` = 可调度. Zeros both windows. **No** `w:` watching fail-open.
- Auto expiry = same as `selectable`.

Do not call account `quality-history` for this dialog.
