# Pin API contract

Locked 2026-08-21. Sixth pair admission state. UI **长期豁免**. API **`pinned`**. Do not reuse `resumed`.

## 1. Scope / Trigger

Admin can pin a user×account pair so smart-schedule keeps admitting it at the full member cap and never evaluates pair quality / never `StartCooldown` until the admin leaves. Cross-layer: Redis pin HASH, `SetPairAdmission`, `admitsScheduleUser`, `ObservePairCompletion`, GET member `pinned`, admission switcher.

## 2. Signatures

```
POST /admin/accounts/:id/smart-schedule-resume
{ "user_id": int64, "state"?: "paused"|"cooling"|"probing"|"resumed"|"selectable"|"pinned" }

ParsePairAdmissionState(raw string) (string, error)
  raw == "" → "resumed"
  raw == "pinned" → "pinned"

Redis: HSET smart-schedule:pinned:{accountID} u:{userID} <unix>
       HGET / HDEL same field. No key TTL.
GET  /admin/users/:id/smart-schedule  member.pinned: true | absent/false
```

Hot path:

```
SmartScheduleLookup.IsPinned(ctx, accountID, userID) bool
UserSmartScheduleCache.MarkPinned / ClearPinned / IsPinnedBatch
```

## 3. Contracts

### Enter (`state=pinned` only)

| Action | Do |
| --- | --- |
| Cooldown | `HDEL` this pair |
| Probe mark | `HDEL` |
| `u:` / `w:` | do **not** write; `ClearUserResume` leftover chips |
| `MarkUserResume` | **no** |
| Pair windows | keep; ingest may continue |
| Pin mark | `HSET` no TTL |
| GET | `pinned: true` |

### Leave

Admin must pick `paused` / `cooling` / `probing` / `selectable` / `resumed`. `HDEL` pin, then that state's existing side effects. No implicit timeout.

### Hot path

1. Account `IsSchedulable()` / hard-close still apply (whole-account).
2. `paused` still rejects.
3. If pinned: admit (pair cap still enforced in slot acquire). Skip evaluate and `StartCooldown`.
4. Cooldown expiry → `probing`, never `pinned`.
5. No deploy backfill. Redis miss = not pinned.

`admitsScheduleUser` order: pause → **pin** → cooldown → existing 豁免期 / probe / evaluate.

`ObservePairCompletion` order: pause → **pin (ingest, return)** → cooldown skip → ingest + evaluate.

`StartCooldown` while pinned is a no-op.

### Omit `state`

Still `resumed` (豁免期 write default). **Not** `pinned`. **Not** a pause-exit default.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| `state=pinned` | 200, `{state:"pinned", pinned:true}` |
| omit / `""` | `state=resumed`, pin mark cleared if it was set |
| `state=pin` / `long_exempt` / `unpause` | `SMART_SCHEDULE_ADMISSION_INVALID` |
| `user_id<=0` | `SMART_SCHEDULE_RESUME_INVALID` |
| Redis miss | not pinned |

## 5. Good / Base / Bad

- Good: cooling pair → `state=pinned` → cooldown gone, probe gone, no `u:`/`w:`, subsequent N successes do not cool.
- Good: pinned → `state=selectable` → windows zeroed; next full-N breach cools.
- Base: omit `state` after pause → `resumed`, not `pinned`, not `probing`.
- Good: cooldown wall-clock expiry → `IsProbing==true`, `IsPinned==false`.
- Bad: treat leftover `u:`/`w:` as `pinned`.
- Bad: `Expire` the pin HASH (drops sibling users).
- Bad: check cooldown before pin (leftover cooldown would reject a pinned pair).

## 6. Tests Required

- Enter / leave.
- Omit state ≠ pinned.
- Expiry still probing.
- N successes while pinned do not cool.
- Leave to selectable can cool again.
- Pause does not become pinned.

## 7. Wrong vs Correct

#### Wrong

```go
if state == "" {
    return PairAdmissionPinned, nil // omit must stay 豁免期
}
lookup.StartCooldown(ctx, accountID, userID, minutes, now) // while IsPinned
expirePairCooldown → MarkPinned
```

#### Correct

```go
if state == "" {
    return PairAdmissionResumed, nil
}
if lookup.IsPinned(ctx, accountID, userID) {
    return true // admit; do not evaluate
}
expirePairCooldown → MarkProbing // never pinned
```
