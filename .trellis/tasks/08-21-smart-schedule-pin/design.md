# Design: smart-schedule `pinned`

## Boundaries

- Persist pin like probe: Redis HASH only. No SQL column. No user-bundle JSON field.
- `resumed` stays the short grace (`u:`/`w:`). `pinned` is a separate mark.
- Occupied checkout stays on `main`. This is fork-local schedule work, not an upstream merge.

## Data flow

```
Admin POST /admin/accounts/:id/smart-schedule-resume {user_id, state}
  → ParsePairAdmissionState
  → SetPairAdmission
      pinned: ClearCooldown, ClearProbing, ClearUserResume, MarkPinned
      other: ClearPinned then existing case body
  → GET hydrates member.pinned via IsPinnedBatch

Hot path
  IsSchedulable()                # unchanged whole-account gate
  admitsScheduleUser
      paused → reject
      pinned → admit (skip evaluate / StartCooldown)
      cooldown → reject
      probing / selectable / resumed → existing
  ObservePairCompletion
      paused → no ingest
      pinned → ingest, return
      cooldown → no ingest
      else ingest + evaluate
  resolvePairSlotAcquire
      probing → probe cap
      else member cap (pinned uses this)
```

## Contracts

### Write

`POST /admin/accounts/:id/smart-schedule-resume`

| Field | Rule |
| --- | --- |
| `user_id` | required `> 0` |
| `state` omitted / `""` | `resumed` |
| `state=pinned` | enter long exemption |
| other | existing five states |
| invalid | `SMART_SCHEDULE_ADMISSION_INVALID` |

Enter `pinned`:

- `HDEL` cooldown field
- `HDEL` probe field
- `HDEL` resume `u:` and `w:` (clear leftover 豁免期 chips; do not `MarkUserResume`)
- `HSET` `smart-schedule:pinned:{accountID}` `u:{userID}` (unix now, no key TTL)
- do **not** `ZeroPairQuality`

Leave `pinned`: `HDEL` pin field, then the chosen state's existing side effects.

### Read

`GET /admin/users/:id/smart-schedule` member:

```json
{ "pinned": true }
```

Miss / no mark = omit or `false`. Do not invent pin from leftover `u:`/`w:` or from `paused`.

Hydrate skip leftover pin bits when the row is already `paused` or has a future `cooldown_until` (same as leftover probe).

Response `PairAdmissionResult` may echo `pinned: true` when `state=pinned`.

### Redis

| Key | Field | TTL |
| --- | --- | --- |
| `smart-schedule:pinned:{accountID}` | `u:{userID}` | **none** |

Shared-key Expire must not be used to “timeout” a pin. Sibling users share the HASH.

### Hot-path order (must)

`admitsScheduleUser`: pause → **pin** → cooldown → 豁免期 / probe / evaluate.

`ObservePairCompletion`: pause → **pin (ingest, return)** → cooldown → ingest → evaluate.

`expirePairCooldown`: `HDEL` cooldown; if still pinned, stop (do not `MarkProbing`); else zero windows + `MarkProbing` + clear `u:`/`w:`. Never `MarkPinned`.

`StartCooldown` (hot path / cache): no-op while pinned.

`evaluateSmartSchedulePairQuality`: if pinned, return true (no graduate, no cool).

## Compatibility

- No backfill. Existing selectable / resumed / probing pairs stay as they are.
- `ParsePairAdmissionState("")` stays `resumed`.
- Pause lift still requires an explicit next state; clearing `paused` must not write pin.
- Account hard-close / `IsSchedulable()` unchanged.

## Tradeoffs

| Option | Why rejected / chosen |
| --- | --- |
| Reuse `resumed` with no TTL | Rejected. Locked contract: new API `pinned`. |
| Persist pin on membership SQL | Rejected. Probe is Redis-only; pin follows probe. |
| Zero windows on enter | Rejected. Windows may keep ingesting. |
| Pin after cooldown check | Rejected. Leftover cooldown must not reject a pinned pair. |

## Rollback

- Delete `smart-schedule:pinned:*` keys (or redeploy without readers: miss = not pinned).
- UI sixth menu item is additive; old clients that never send `state=pinned` keep 豁免期 default.
