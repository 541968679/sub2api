# Research: probe-api-contract

- **Query**: 智能调度考察期后端 API / Redis / 前端水合合同
- **Scope**: backend（本任务后端落地后的权威字段）
- **Date**: 2026-08-21

## Findings

### State strings

API / Redis / Go constants (do not invent a sixth state or `unpause`):

| UI | `state` | Who enters |
| --- | --- | --- |
| 暂停 | `paused` | Manual only. Long-lived. No implicit unpause. |
| 冷却 | `cooling` | Auto on breach, or manual. |
| 考察 | `probing` | Auto on cooldown expiry, or manual (including 调度→考察). |
| 调度 | `selectable` | Auto on probe graduate, or manual skip. |
| 豁免期 | `resumed` | **Manual only.** API name unchanged. |

`POST /admin/accounts/:id/smart-schedule-resume` body:

```json
{ "user_id": 16, "state": "probing" }
```

- Omitted / empty `state` → `resumed` (legacy 豁免期 write default).
- That omitted default is **not** a pause-lift default and **must not** become `probing`.
- Invalid `state` → `SMART_SCHEDULE_ADMISSION_INVALID`.
- Leaving `paused` requires an explicit next `state` ∈ {`probing`,`selectable`,`resumed`,`cooling`} (or write `paused` again).

### Switcher response (`PairAdmissionResult`)

```json
{
  "account_id": 7,
  "user_id": 16,
  "state": "probing",
  "probing": true,
  "probe_cap": 4
}
```

| Field | When |
| --- | --- |
| `state` | One of the five strings above. |
| `cooldown_until` | Only for `cooling` (RFC3339 / Go `time.Time` JSON). |
| `probing` | `true` iff this write landed in 考察. Always present (`false` otherwise). |
| `probe_cap` | Present only when `state=probing`. In-flight cap actually enforced. |

`probe_cap` = `min(desired, member_pair_cap)` if the member has a cap ≥ 1, else desired.

`desired` comes from this user×platform policy:

| `probe_concurrency_mode` | `probe_concurrency` | desired |
| --- | --- | --- |
| omit / empty / `follow_n` | ignored | window N (`quality_window_samples`, 1–100, default 10) |
| `custom` | required integer 1–100 | that number |

Member `max_concurrency` is a hard ceiling. **Not** account-quality global N. Invalid custom (0, >100, custom without a number) → `SMART_SCHEDULE_INVALID_QUALITY` (do not silently fall back).

GET `/admin/users/:id/smart-schedule` also echoes policy fields `probe_concurrency_mode` and `probe_concurrency` so the form can round-trip. Copy-from-platform copies these as their own fields; it must not copy account-quality N into `probe_concurrency`.

### GET pool row — how the UI knows a pair is probing

`GET /admin/users/:id/smart-schedule` → `platforms.*.accounts[]`:

```json
{
  "account_id": 7,
  "max_concurrency": 4,
  "current_concurrency": 1,
  "paused": false,
  "probing": true,
  "probe_cap": 4,
  "cooldown_until": null,
  "pair_quality": { "ok_count": 0, "ttft_count": 0, "n": 10 },
  "will_cool": false
}
```

UI rules:

1. `paused === true` → 暂停. Ignore leftover probe bits.
2. `cooldown_until` in the future → 冷却.
3. **`probing === true` → 考察.** Occupancy badge denominator = `probe_cap` (never 999, never “uncapped”).
4. Resume overlay `u:`/`w:` still active → 豁免期 (`resumed`). `probing` is false after a manual 豁免期 write.
5. Else → 调度 (`selectable`). Member cap / 999 display unchanged.

Redis miss / omitted `probing` / `probing: false` = **not probing**. Do not backfill existing selectable rows after deploy.

`will_cool` still uses pair windows + saved thresholds (not account 15m). During probing it may preview a standard breach; the probe-only `and` mixed cool is enforced on the hot path even if `will_cool` is false.

### Redis

| Key | Field | Value |
| --- | --- | --- |
| `smart-schedule:probe:{accountID}` | `u:{userID}` | enter unix (any non-empty value means probing) |
| `smart-schedule:cooldown:{accountID}` | `u:{userID}` | until unix (unchanged) |

No probe TTL. No `u:`/`w:` grace on enter-probing or expiry.

### Events (`pair-quality` detail list)

`probe_enter` | `probe_graduate` plus existing `cooldown_start` / `cooldown_end` / `resumed` / `selectable` / `expiry_zero`.  
Expiry writes `expiry_zero` then `probe_enter`. Graduate writes `probe_graduate` and **does not** zero windows. No `unpause` / `pause_lifted` events.

### Side effects (write)

| Target | Cooldown | Windows | `u:`/`w:` | Probe mark | Cap |
| --- | --- | --- | --- | --- | --- |
| Enter probing (expiry or admin) | clear | **zero** | clear | set | probe cap |
| Probe → selectable (auto) | — | **keep** | — | clear | member cap |
| Manual selectable | clear | **zero** | `ClearUserResume` (no `w:`) | clear | member cap |
| Manual 调度→考察 | clear | **zero** | clear | set | probe cap |
| Manual 豁免期 | clear | **zero** | `MarkUserResume` | clear | member cap |
| Grace end → selectable | — | **keep** | natural expiry | none | member cap, then evaluate |
| Manual cooling / paused | existing | no ingest | clear | clear | reject |
| Leave paused | **no independent action** | — | — | — | next explicit state |

### Related Specs

- `.trellis/spec/backend/account-user-schedule.md`
- `.trellis/tasks/08-21-smart-schedule-probe/prd.md`

## Caveats / Not Found

- Old binaries ignore the probe HASH and treat the pair as selectable (full cap). Rollback needs no backfill script.
- Frontend reads `probing` + `probe_cap` (with leftover aliases). Missing mark / omitted `probing` is not probing.
