# Account User Schedule + Pair Concurrency + Quality Gate

## Scenario: independent allow / deny / pair-cap / quality-gate

### 1. Scope / Trigger

- Trigger: account-user admission, per-account-per-user concurrency, and optional per-user live-quality gates are a cross-layer contract (SQL join row, Redis snapshot meta, Redis pair slot, Redis live quality, admin DTO, list inline patch).
- Do not fold pair limits or quality gates into `IsSchedulable()`.
- Quality gate result is pair exclude (clear sticky, reselect). It must not set `TempUnschedulableUntil` or depend on the hard-close master switch.
- Leftover `accounts.user_schedule_mode` is not the write or hot-path source of truth.

### 2. Signatures

- DB `account_schedule_users`: PK `(account_id, user_id)`; `allow bool`; `deny bool`; `max_concurrency int null` (`NULL` or `>= 1`); optional quality columns `quality_max_p50_ttft_ms`, `quality_min_success_rate`, `quality_min_success_samples`, `quality_min_ttft_samples`, `quality_condition` (`or`/`and`). A gate is enabled only when p50 or success rate is set; samples/condition are modifiers and must not create a map entry by themselves.
- Domain: `Account.AllowUserIDs []int64`, `DenyUserIDs []int64`, `UserConcurrency map[int64]int`, `UserQualityGates map[int64]QualityHardCloseSettings`.
- `Account.AllowsScheduleUser(userID int64) bool` — identity only.
- `Account.QualityGateBlocksUser(userID int64, stats *AccountQualityStats) bool`
- `Account.AdmitsScheduleUser(userID int64, stats *AccountQualityStats) bool` — identity plus quality gate.
- `Account.PairMaxConcurrency(userID int64) int` — `0` means no pair cap.
- Redis pair slot: `concurrency:account_user:{accountID}:{userID}:{platform}` (zset). Empty platform is `_`. Pair Get/Acquire use a 90s live window + current process prefix (not the account-slot 15min TTL). `ClearAccountSlots` also deletes `concurrency:account_user:{accountID}:*`. Detached Get ctx must copy `ScheduleLookupPlatform` so hydrate does not read `_`.
- Redis live quality: `account-quality:last-n:{accountID}` is the \(Q_a\) source of truth (site-wide N, all users; TTL 7d). Completions ingest and project JSON to `account-quality:live:{accountID}` (no resume fields). Reader = selection `Get` (prefers last-N, then legacy live JSON). Cache miss / nil stats fail open. The 5-minute tick must not `Replace` live from a 15-minute SQL window. Pair cooldown stays on \(Q_{a,u}\), not this account cell. Admin user-list quality is a separate Redis window `user-quality:last-n:{userID}` (\(Q_u\), same N, this user across all accounts) and does not feed gates or hard-close.
- Redis resume overlay (Track A): `account-quality:resume:{accountID}` (HASH, TTL 20m). `MarkUserResume` / `MarkAccountResume` write HASH fields only (`u:{userID}`, `a`). `Get` merges the overlay onto last-N / live stats. Do not SCAN-delete last-N keys. Resume HASH must survive even when an account has no last-N key. Legacy live JSON `resume_users` / `account_resume_until` is migrated with `HSETNX` then stripped from the live key. This overlay is **not** the smart-schedule 豁免期.
- Admin update JSON fields: `allow_user_ids`, `deny_user_ids`, `user_concurrencies`, `user_concurrency_patch`, `user_quality_gates`, `user_quality_gate_patch`.
- Legacy write still accepted: `user_schedule_mode` + `schedule_user_ids` (replaces one list, does not clear caps or gates).
- Admin UI may save/apply the site-wide quality threshold template (`GET/PUT /admin/settings/quality-hard-close` metric fields). The template is not stored per user and must not include `user_id`. Apply fills the current gate form only; persist still goes through `user_quality_gates` / `user_quality_gate_patch`. User-gate forms expose p50 / success rate / two sample floors / condition. They do not have `pause_minutes`. **立即恢复** (`POST /admin/accounts/:id/quality-resume` `{user_id}`) keeps the gate and writes a 15-minute resume-HASH grace (`u:{userID}`). While the grace is active, `QualityGateBlocksUser` does not block that pair. Account hard-close **立即恢复调度** / `recover-state` writes HASH field `a` (`account_resume_until` after merge) so the next tick does not re-pause on the same window.

### 3. Contracts

Read `schedule_users[]`: `{id, email, deleted, allow, deny, max_concurrency?, quality_max_p50_ttft_ms?, quality_min_success_rate?, quality_min_success_samples?, quality_min_ttft_samples?, quality_condition?, quality_blocked?, quality_resumed_until?, quality_window_until?}`. Union of users with any of the four attributes. Hydrate stamps runtime chips from the live cache: `quality_blocked` when the pair is excluded; `quality_resumed_until` while the 已恢复 chip is active; `quality_window_until` while a new window is accumulating after click-已恢复 or the 15-minute auto flip. **立即恢复** writes HASH `u:{userID}=now+15m` and `w:{userID}=now+30m`. Click 已恢复 (`start_window: true`) deletes `u:` and sets `w:{userID}=now+15m`. Fail-open lasts until `w:` expires.

Write (pointers; omitted = no write; empty slice = clear that set):

| Field | Semantics |
| --- | --- |
| `allow_user_ids` | Replace allow set |
| `deny_user_ids` | Replace deny set |
| `user_concurrencies` | Replace-all caps (`max_concurrency >= 1`) |
| `user_concurrency_patch` | Merge one user; `max_concurrency` null/0 deletes that cap |
| `user_quality_gates` | Replace-all gates; empty array clears all gates |
| `user_quality_gate_patch` | Merge one user; all quality fields null/omitted deletes that gate |

Defaults when a gate is enabled: condition `or`, success samples 20, TTFT samples 10 (independent floors; this is not `account_quality_window_n`). Unconfigured metrics are not judged; under-sampled metrics are not judged. Reuse `EvaluateAccountQualityHardClose` breach rules with `Enabled=true` (no pause). User gates still read live \(Q_a\); they do not use pair `quality_window_n`.

Bulk may send allow/deny (or legacy mode+ids). Bulk must not send `user_concurrencies`, `user_concurrency_patch`, `user_quality_gates`, or `user_quality_gate_patch`. Bulk allow/deny must keep existing caps and gates.

Snapshot meta must copy `AllowUserIDs`, `DenyUserIDs`, `UserConcurrency`, `UserQualityGates`. A missing field unmarshals empty for that field only: old snapshots without `UserQualityGates` have no gates (fail open), and still honor allow/deny/caps. Until `account_changed`, do not treat a missing gate map as unrestricted identity. A gate map entry requires at least one judged metric (p50 or success rate); samples/condition are modifiers. last-N ingest is the live write path; do not `Replace`+SCAN-delete last-N. Active resume HASH keys must survive, including when no last-N keys exist. `Get` / ingest-time hard-close must merge resume so `EvaluateHardClose` sees `account_resume_until`.

### 4. Validation & Error Matrix

| Condition | Error |
| --- | --- |
| Unknown user id on lists/caps | `USER_SCHEDULE_UNKNOWN_USER` / `USER_CONCURRENCY_UNKNOWN_USER` |
| Unknown user id on gates (`userID<=0`) | `USER_QUALITY_GATE_UNKNOWN_USER` |
| Explicit cap `< 1` on replace-all or patch set | `USER_CONCURRENCY_MIN` |
| Invalid p50 / rate / samples / condition | `USER_QUALITY_GATE_INVALID` |
| Both `user_concurrencies` and `user_concurrency_patch` | `USER_CONCURRENCY_CONFLICT` |
| Both `user_quality_gates` and `user_quality_gate_patch` | `USER_QUALITY_GATE_CONFLICT` |
| Bulk writes caps | `BULK_USER_CONCURRENCY_FORBIDDEN` |
| Bulk writes gates | `BULK_USER_QUALITY_GATE_FORBIDDEN` |
| Legacy `schedule_user_ids` without `user_schedule_mode` | `USER_SCHEDULE_MODE_REQUIRED` |
| Legacy allow/deny with empty ids | `USER_SCHEDULE_USERS_REQUIRED` |

Runtime (not HTTP): `userID<=0` and any rule exists (allow/deny/cap/gate) → fail closed. Deny hit → reject. Nonempty allow miss → reject. Quality breach → admission fail, clear sticky, reselect; never pair-full / WaitPlan. Pair-full → exclude + reselect, never WaitPlan/429 for that reason alone. Live quality cache miss → do not block.

### 5. Good / Base / Bad Cases

- Good: allow `[16]` + cap `{16:5}` → user 16 schedulable, pair max 5; user 7 rejected.
- Base: all four empty → same as unrestricted (account + user global only).
- Good: deny `[16]` + cap `{16:5}` → user 16 rejected; cap ignored.
- Good: quality-gate-only user 16 + live p50 breach → user 16 excluded; other users still get the account; `schedulable` unchanged.
- Good: quality gate configured but live cache missing / samples below min → do not block.
- Good: pair current `>= N` and another eligible account exists → other account, no 429.
- Bad: emit `AccountWaitPlan` because the pair is full or a quality gate blocked.
- Bad: omit new snapshot fields (restricted accounts look open).
- Bad: fold the quality gate into `IsSchedulable()` (whole account dropped).

### 6. Tests Required

- `AllowsScheduleUser`: unrestricted / deny+cap / allow+cap / allow miss+cap / `userID=0` with any rule including quality-gate-only / empty allow / empty deny.
- `QualityGateBlocksUser` / `AdmitsScheduleUser`: breach / under-sampled / nil stats / no gate / or vs and / quality-gate-only `userID=0`.
- Pair-full reselect on Anthropic/Gemini, OpenAI advanced scheduler, OpenAI `LoadBatchEnabled=false`, and model-routing wait loop (must not WaitPlan).
- Sticky: identity deny or quality-gate block clears pin; pair-full keeps pin.
- Admin: replace-all vs patch; legacy mode+ids does not clear caps or gates; bulk rejects cap and gate fields; restore-default writes four empties.
- Snapshot: `buildSchedulerMetadataAccount` copies the four fields.
- Maintenance: tick `Replace`s live stats (sampled rows persist; empty-sample rows are included so Redis can DEL).

### 7. Wrong vs Correct

#### Wrong

```go
result, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
if !result.Acquired {
    return waitPlan(account) // pair-full must not land here
}
```

#### Correct

```go
result, pairFull, err := s.tryAcquireAccountAndPairSlot(ctx, account)
if pairFull {
    localExcluded[account.ID] = struct{}{}
    continue
}
```

## Common Mistake: condition-only / samples-only ghost gate

**Symptom**: `userID<=0` is fail-closed, or chips/API show a gate, but live quality never actually blocks anyone.

**Cause**: default `or` / sample 20/10 were written as a map entry without p50 or success rate. `hasUserScheduleRules()` then treats the account as restricted.

**Prevention**: `qualityGateHasMetric` / `qualityGateHasConfiguredColumn` require p50 or success rate. Admin replace/patch, snapshot copy, and list empty-save must drop modifier-only rows.

## Common Mistake: `Replace` SCAN deletes a just-written resume

**Symptom**: 立即恢复 succeeds, the next 5-minute tick (or a quiet period with no 15-minute traffic) schedules the user again, then the following window with samples blocks them on the same stale 15-minute stats. Hard-close recover can also re-pause on the next tick.

**Cause**: resume lived inside `account-quality:live:{id}`. `RunTick` only puts recent-traffic accounts into `Replace`, then SCAN DELs every other live key. A resume-only account, or an empty candidate set, lost the grace. A Get-then-Set on the same live key also raced with `Mark*Resume`.

**Prevention**: store grace in `account-quality:resume:{id}` HASH. `Replace` writes sample JSON only. Tests must cover: account absent from `allStats`, empty `Replace`, two user fields on one HASH, and `Replace` mutating the caller map for `EvaluateHardClose`.

## Scenario: user × platform smart schedule

### 1. Scope / Trigger

- Trigger: user-page per-platform closed pool + quality + pair cooldown + pool-member cap. Cross-layer: two SQL tables, user Redis cache, pair cooldown HASH, pair-quality windows, every `admitsScheduleUser` path, UsersView modal.
- Independent of `account_schedule_users`. Do not write this policy onto shared account scheduler snapshots. Do not fold into `IsSchedulable()` or `SetTempUnschedulable`.
- Lookup key is `SmartScheduleLookupPlatform(account, hint, bundle)`: OpenAI + (Claude-GPT bridge or AG-group) uses `antigravity` only while `bundle.EnabledPolicy(antigravity) != nil`; otherwise those requests keep `openai`. Native OpenAI groups always stay `account.Platform`. Once AG is on, never fall back across pools. Scheduler eligibility for bridge stays `account.Platform == openai`.
- Track A account quality (`account-quality:live`, 15m / 5min cells, hard-close) is unchanged. Smart-schedule cooldown must not read that live quality number.
- Track B pair quality `Q_{a,u,p}` is `(account_id, user_id, platform)` and feeds smart-schedule cooldown + the pool 配对质量 column. openai and antigravity pools are fully independent. When selectable composite is on, the cell adds `K current/limit · C current/limit` from the sched TTFT window (`ttft_slow_count`, `ttft_consecutive_slow`, `quality_sched_max_slow_in_window`, `quality_sched_max_consecutive_slow`). Hide that row when composite is off.

### 2. Signatures

- DB `user_smart_schedule_policies`: unique `(user_id, platform)`; `enabled bool`; quality columns same shape as pair gates; two independent windows — `quality_min_ttft_samples` (N首字) and `quality_min_success_samples` (N成功率), each default 10, clamp **1–100**. Compat API `quality_window_samples` / `quality_window_n` echo `max(N首字, N成功率)` and are not write-path truth. Probe in-flight: `probe_concurrency_mode` (`follow_n` default / `custom`) + optional `probe_concurrency` (1–100, required when custom). `follow_n` uses **N成功率**. This is **not** `account_quality_window_n` and does **not** split account-side user-schedule N or \(Q_u\). `cooldown_minutes` 1–1440 default 15. `soft_cooldown BOOLEAN NOT NULL DEFAULT FALSE` (migration 218): omitted PUT = false (hard). Soft/hard applies to automatic quality cooldown **and** admin `SetCooldown`. `probe_latency_v2 BOOLEAN NOT NULL DEFAULT FALSE` (migration 219): omitted PUT = false (257). Copy copies this flag. Site quality-template schema must **not** include it. Platforms: `anthropic|openai|gemini|antigravity|grok`.
- DB `user_smart_schedule_accounts`: PK `(user_id, platform, account_id)` (migration 211; do not edit 202/204/207/208). Dual membership allowed. `max_concurrency` null or ≥1; `paused bool` default false (migration 207). AG tab may hold `account.IsOpenAI()` members; other tabs stay platform-locked. CASCADE from users/accounts. PUT replace-all is per-platform and must not delete the other pool's row. Client writes ignore `paused`.
- Domain: `SmartSchedulePolicy` / `EnabledPolicy(userID, platform)` — nil when missing, disabled, or `MemberCount()==0`. For OAI + bridge/AG-group, AG nil/empty/disabled keeps the **openai** closed-pool lookup (today's production path). Do not fail-open to account-side just because AG is off. When AG is enabled with members, lookup is antigravity only; a pool miss rejects and must not fall back to openai.
- Redis user cache: `smart-schedule:user:{userID}` JSON of all platforms; invalidate on PUT/copy. The JSON **must** include latency-gate columns: `quality_max_slow_in_window`, `quality_max_consecutive_slow`, `quality_max_p50_duration_ms`, `quality_sched_window_n`, `quality_sched_max_slow_in_window`, `quality_sched_max_consecutive_slow`, and `probe_latency_v2`. Dropping the sched columns makes `SchedCompositeEnabled()` false on a Lookup hit, so selectable evaluation falls back to legacy p50 (window = N首字). Unconfigured columns stay null. TTL 10m.
- Redis 考察预检 samples: `account-quality:precheck:{accountID}` LIST of `{ts,uid,ok,ttft,dur}` (cap 400, TTL 48h). Completions ingest here in addition to last-N. Do **not** reuse `account-quality:last-n` (no per-sample user_id/ts). Precheck reads exclude this user and samples older than `cooldown_minutes`.
- Redis precheck-fail hard wait: `smart-schedule:cooldown-hard:{platform}:{accountID}` HASH `u:{userID}`. Soft-cooldown ingest and `SoftEndCooldown` skip that pair. Clear on leave-cooldown / admin `SetCooldown` / next `EnterProbe`. Key TTL may only lengthen (other users share the HASH); do not `Expire` down.
- Redis cooldown: `smart-schedule:cooldown:{platform}:{accountID}` HASH `u:{userID}=untilUnix`. Hot-path `StartCooldown` is `HSETNX` only (no extend). Admin switcher `SetCooldown` uses `HSET` overwrite. TTL may only be lengthened, never shortened (other users share the key). Platform is in the KEY so one pool's TTL cannot cut the other.
- Redis soft cooldown: `smart-schedule:soft-cool:{platform}:{accountID}` HASH `u:{userID}` = `{samples:[{ts,ok,ttft,dur}], n_ttft, n_ok}`. One window per cooling pair: same-platform pool samples **after that pair’s cooldown write**, **exclude self**. Evaluate with `since = now - cooldown_minutes` (same formula as v2 precheck). Legacy blobs without `samples` are an empty window (not a meet). Do not reuse \(Q_u\) or other pairs’ \(Q_{a,u}\). `StartCooldown` zeros the field only when `HSETNX` succeeds; `SetCooldown` always zeros. Leaving cooldown (`ClearCooldown`, wall-clock expire, or soft early exit) deletes the field. TTL only lengthens (`cooldown_minutes` + buffer). Hard policy must not ingest this key.
- Redis pair quality: `smart-schedule:pair-quality:{platform}:{accountID}` HASH `u:{userID}` = two FIFO windows (`W_ttft`, `W_ok`). Trend list `smart-schedule:pair-quality-trend:{platform}:{accountID}:{userID}` (TTL 24h). Event list `smart-schedule:pair-quality-events:{platform}:{accountID}:{userID}` (TTL 7d). Event `soft_cooldown_end` is soft early-exit only; do **not** reuse `cooldown_end`. Soft-end does **not** write `probe_enter`. Wall-clock still uses `expiry_zero` (when probe v2 is off) then `probe_enter`.
- Redis pair 豁免期: `smart-schedule:resume:{platform}:{accountID}` HASH `u:{userID}` + `w:{userID}` (TTL 40m). Same 15m/30m grace as Track A, but keyed by pool platform. Do not read or write `account-quality:resume` from smart-schedule paths.
- Admin: `GET /admin/users/:id/smart-schedule`; `PUT /admin/users/:id/smart-schedule/:platform`; `POST .../copy` `{from_platform}`; `POST /admin/accounts/:id/smart-schedule-resume` `{user_id, state?, platform?}`. Pair quality: pool member `pair_quality` + `will_cool`; `POST /admin/users/:id/smart-schedule/pair-quality`; `GET /admin/users/:id/smart-schedule/pair-quality/:accountId`; `GET /admin/users/:id/smart-schedule/:platform/accounts/:account_id/pair-quality`. Pair-quality GET ships `probe` / `sched` / `soft` blocks plus `metrics_phase`. Top-level p50/counts/K/C alias the **active phase** window (`recentLatencySamples` of that stage’s N), not the full FIFO. Display-only `p95_ttft_ms` / `ttft_p95_ms` use that same TTFT slice; they are not admission inputs and must not gain `quality_max_p95_*` Settings. `will_cool` / `quality_reason` use the current admission knobs. `state` is `paused|cooling|probing|resumed|selectable|pinned`; omitted `state` is `resumed` (豁免期 write default, **not** `pinned`, **not** a pause-lift default, and **not** `probing`). Invalid `state` → `SMART_SCHEDULE_ADMISSION_INVALID`. Pause of a non-member → `SMART_SCHEDULE_UNKNOWN_ACCOUNT`. Redis probe HASH: `smart-schedule:probe:{platform}:{accountID}` field `u:{userID}`. Redis pin HASH: `smart-schedule:pinned:{platform}:{accountID}` field `u:{userID}`, **no TTL**. Miss / no mark = not probing / not pinned (no deploy backfill). GET hydrates `pinned: true`. Do **not** reuse `resumed` for long-term exemption.
- Pool account details: `GET /admin/accounts?ids=&lite=1`. AG tab **must omit** `platform` (or request both antigravity+openai). `platform=antigravity&ids=` drops OpenAI dual-members after add/save/refresh.

### 3. Contracts

PUT body: `{enabled, quality_*, quality_min_ttft_samples, quality_min_success_samples, quality_window_samples|quality_window_n, probe_concurrency_mode, probe_concurrency, cooldown_minutes, soft_cooldown, probe_latency_v2, accounts:[{account_id, max_concurrency?}]}`. Omitted `soft_cooldown` = false (hard). Omitted `probe_latency_v2` = false (257). Copy copies enabled/thresholds/both N/cooldown/`soft_cooldown`/`probe_latency_v2`/probe settings only, never members or caps; do not echo the two N into one value. Do not copy account-quality N into `probe_concurrency`. Site quality-template schema must **not** include `soft_cooldown` or `probe_latency_v2`. GET shows `enabled=false` when the stored row is enabled but the pool is empty. GET echoes the two real N columns; `quality_window_samples` / `quality_window_n` are compat aliases = `max(N首字, N成功率)`. Also echo `probe_concurrency_mode` / `probe_concurrency` (omit/empty mode → `follow_n`), `soft_cooldown`, and `probe_latency_v2`. GET hydrates member `soft_cooldown_progress` **only** when the platform is soft **and** that member is cooling. Launch: both stored N already equal that user’s previous single N — do not default N首字 smaller.

Resume (`state=resumed` / 豁免期) is **manual only**. It `HDEL`s that pair’s cooldown field, **zeros both pair windows**, clears `paused` and the probe mark, and writes `smart-schedule:resume:{platform}:{accountID}` `u:`+`w:` grace. Completions during grace **do enter** the windows; evaluate is skipped until `u:`/`w:` expire. After grace the pair is 调度 (`selectable`): **keep** in-grace windows, then evaluate with selectable rules (may cool immediately). Grace end does **not** enter 考察. Track A `account-quality:resume` is unchanged.

`selectable` (调度) clears `paused`, `HDEL`s cooldown, **zeros both windows**, `ClearPairResume` (deletes this pool’s `u:` **and** `w:`), and clears the probe mark. There is **no** 15-minute watching fail-open. Re-accumulate N before cooldown. Do not clear Track A resume.

`probing` (考察) is entered by **hard cooldown wall-clock expiry** or by admin (including 调度→考察). Soft early-exit does **not** enter 考察. **v2 off (257):** `EnterProbe` / `enterProbeFromCooldown` `HDEL`s cooldown, **zeros both windows**, `ClearPairResume`, `HSET` probe mark. Graduate still allows unfilled TTFT. **v2 on:** only those hard-expiry / admin-考察 triggers run 考察预检 once (`EvalQuality` on precheck samples + 考察期 knobs). fail → pair cooldown reason `考察预检` + hard-wait (no probe mark). pass → zero pair windows, stay selectable (no probe cap). pending → zero pair windows, then formal probe. Formal probe uses the same `EvalQuality` on `Q_{a,u}`: C ready at C, K ready at K, p50 full N; empty window cannot graduate; no Hold; do not re-read last-N \(Q_a\). Shared `EvalQuality` is per configured metric: enter/meet is AND, exit is OR. Smart-schedule `quality_condition` is still stored (copy/PUT/editor) but **ignored at runtime**. Track A account gates still honor `or`/`and`. In-flight pair cap = `min(desired, member cap)` or desired if unset. `desired` is **N成功率** when `probe_concurrency_mode=follow_n` (omit/empty), or `probe_concurrency` (1–100) when `custom`. Member `max_concurrency` is a hard ceiling. This is **not** account-quality global N. Invalid custom (0, >100, custom without a number) → `SMART_SCHEDULE_INVALID_QUALITY`; do not silently fall back. 257 graduate requires `W_ok` full N成功率; configured success-rate threshold must pass; `ttft_samples < N首字` does **not** block; if `W_ttft` is full (N首字) and p50 is configured, p50 must pass. No time-based graduate. No traffic → stay probing. Selectable knobs / fail path stay on `pairQualitySelectableBlocksWithReasons` / `EvalQuality(SchedQualityKnobs)`.

`cooling` clears `paused` and the probe mark, `HSET`s cooldown to `now+policy.cooldown_minutes` and deletes `u:`/`w:`. Cooling/paused pairs do not ingest (not into \(Q_{a,u}\) and not as soft-window producers). Auto **hard** cooldown expiry enters **考察**, not 调度: `EnterProbe` / `enterProbeFromCooldown` (`HDEL` + zero windows + write probe mark + `ClearUserResume`, no time grace; v2 may 预检). Soft policy may **also** leave cooling early: after a non-cooling completion (including pin), ingest that sample into other same-user-same-platform cooling peers’ soft windows (timestamped samples, `since = now - cooldown_minutes`) and evaluate `softCooldownMeets` = `EvalQuality(..., SchedQualityKnobs) == pass`. Meet = every **configured** selectable metric is ready and passing (AND-enter). Any configured metric fail OR-exits (stay cooling). Underfull ≠ pass. Do **not** treat `pairQualityBlocks == false` (fail-open) as pass. On meet: `SoftEndCooldown` → **selectable** (`ClearProbing`, zero pair windows like 预检 pass, event `soft_cooldown_end`). Do **not** MarkProbing, do **not** run 考察预检, do **not** write `probe_enter`. Only hard countdown expiry calls `EnterProbe`. Wall-clock `cooldown_minutes` remains the ceiling. The request that cools account A must not enter A’s new soft window. Hard policy must not ingest soft windows. Pinned leftover-cooldown peers are not soft-window targets.

`paused` is **long-lived manual only**. Sets the membership flag, invalidates `smart-schedule:user:{userID}`, `HDEL`s cooldown, deletes `u:`/`w:`, and **clears** the probe mark and pin mark. **No implicit unpause** and **no default next state**. Leaving pause requires an explicit `state` ∈ {`probing`,`selectable`,`resumed`,`cooling`,`pinned`} (or write `paused` again). Clearing the paused flag must not write `probing` or `pinned`.

`pinned` (长期豁免) is **manual only**. Enter (`state=pinned` only): `HDEL` cooldown, clear probe mark, `ClearPairResume` (do **not** write pair `u:`/`w:`, do **not** write Track A resume), `HSET` pin mark, **keep** pair windows. Full member cap. Windows may keep ingesting. **Never evaluate, never `StartCooldown`** until the admin leaves. Leave requires an explicit next state (`paused` / `cooling` / `probing` / `selectable` / `resumed`). **No implicit timeout**. Cooldown expiry still → `probing`, never `pinned`.

Hot path (`admitsScheduleUser`): `EnabledPolicy` hit → pool miss reject; pool hit ignores that pair’s allow/deny/gate/cap; `paused` reject (account stays in the pool); **`pinned` admit and skip evaluate / `StartCooldown`** (check pin **before** leftover cooldown); active cooldown reject; pair 豁免期 `smart-schedule:resume` `u:`/`w:` fail-open (no evaluate, no graduate) **only when not probing**; leftover pair resume during probing still evaluates (graduate); probing vs selectable then judge **only** `Q_{a,u}` (pair windows) via `EvalQuality`. Do not read `account-quality:resume` here. Unfilled metrics stay pending (do not pass, do not fail). `ok_samples < N成功率` does not judge success; `ttft_samples < N` does not judge that fan’s p50 (K/C ready at K/C). Smart-schedule `quality_condition` is ignored. Selectable breach → `HSETNX` cooldown then reject. Probing may graduate (keep windows) or cool. Pair cap uses pool member N, except while probing (`resolvePairSlotAcquire` / pair occupancy must honor probe cap; follow_n desired = N成功率). `pinned` uses the member cap, not the probe cap. Account hard-close / `IsSchedulable()` still applies (whole-account gate). Otherwise the legacy scenario in this file. Do not fold pause, probing, or pin into `IsSchedulable()`. Do not backfill existing pairs on deploy.

Ingest: failure → `W_ok` only; sync success without first token → `W_ok` only; streaming success with `true_first_token_ms` or `first_token_ms` → both. `W_ok` uses the same counted-error policy as account track (`schedule_use_failover_error_rate`) via `ClassifyOpsErrorRateCalibers`. Empty `schedule_error_whitelist` matches pre-feature production: Recovered stays off schedule unless the failover toggle is on; Claude–GPT bridge and the legacy `IsAccountQualityRoutingModelMiss` rail stay hardcoded excludes. New families (client request 400, 400 URF, long context, pair concurrency, unrestricted group-no-account, routing 503, protocol mismatch) write `Success=false` unless checked. Full families: `ops-schedule-error-caliber.md`. Cooling / paused pairs do not ingest. `pinned` pairs **do** ingest and must not evaluate / `StartCooldown`. `will_cool` on the pool row uses pair windows + saved thresholds, not account 15m cells; skip `will_cool` while pinned.

### 4. Validation & Error Matrix

| Condition | Error / runtime |
| --- | --- |
| `enabled=true` and zero members | `SMART_SCHEDULE_EMPTY_POOL` (write). Runtime: treat as disabled (legacy). |
| Unknown / wrong-platform account id | `SMART_SCHEDULE_INVALID_ACCOUNT` |
| Duplicate `account_id` in one PUT | `SMART_SCHEDULE_DUPLICATE_ACCOUNT` |
| Account platform does not match the tab (except AG tab + OpenAI) | `SMART_SCHEDULE_PLATFORM_MISMATCH` |
| `cooldown_minutes` out of 1–1440 | `SMART_SCHEDULE_INVALID_COOLDOWN` |
| Invalid quality metric / condition / N outside 1–100 / invalid probe concurrency | `SMART_SCHEDULE_INVALID_QUALITY` |
| Copy `from_platform` missing or same | `SMART_SCHEDULE_COPY_INVALID` |
| `userID<=0` or cache miss | legacy rules only |

### 5. Good / Base / Bad Cases

- Good: OpenAI enabled + pool `[A,B]` + A pair-window p50 breach (full N) → A excluded and cooled; B still selectable; Anthropic requests for the same user stay legacy. Account 15m live breach alone must not cool.
- Base: no policy rows → identical to the allow/deny/gate/cap scenario above.
- Good: last pool member CASCADE-deleted → `EnabledPolicy` nil → legacy, not platform-wide empty reject.
- Good: platform `soft_cooldown=true`, pair A cooling, pair B (same user/platform, not cooling) completes with a full passing soft window → A goes **selectable** via `soft_cooldown_end` (not 考察); wall-clock minutes still bound the wait if the window never meets. Hard expiry still `EnterProbe`.
- Bad: copy OpenAI members onto Gemini. Bad: `Expire` a shared cooldown HASH down to a shorter user’s TTL. Bad: snapshot-copy user policy. Bad: treat `pairQualityBlocks==false` as a soft meet. Bad: write the cooling request into that pair’s own new soft window.

### 6. Tests Required

- `EnabledPolicy`: off / empty / one member; pool miss vs hit; ignore legacy deny/gate/cap when enabled.
- Cooldown: `HSETNX` no-extend; **hard** expiry zeros pair windows then enters probing (no `w:`, `ClearUserResume`, **never pin**); resume deletes only that pair and zeros windows; HASH TTL never shortens another user. `selectable` must not `MarkUserQualityWindow`. Soft cooldown: default hard; omitted PUT = false; CopyPlatform copies the flag; quality template does not write it; `softCooldownMeets` underfull / C∨K∨p50 full-fail / AND-enter OR-exit (`quality_condition` ignored) / unconfigured-skip / time-window `since=now-cooldown_minutes` / legacy blob empty; Observe peer ingest (pin yes, cooling self no, hard no ingest); A/B pair isolation; re-select/`SetCooldown` zeros the soft window; TTL only lengthens; event `soft_cooldown_end` ≠ `cooldown_end`; SoftEnd → selectable (not `EnterProbe`); hydrate progress only when soft + cooling. Probe: hard expiry→probe; soft-end ≠ probe; graduate `W_ok`-only (sync, no TTFT); leftover `u:`/`w:` must not skip graduate; pause does not auto-probe or auto-pin; 调度→考察 zeros; no backfill. Pin: enter/leave; omit `state` ≠ pinned; N successes while pinned do not cool; leave to selectable can cool again.
- Admin: reject enabled+empty; GET masks empty as disabled; copy omits accounts; copy probe settings as their own fields (not account-quality N); delete-last auto-disables.
- Probe concurrency: omit → follow_n; follow_n + cap / no cap; custom 2 with cap 5 → 2; custom 10 with cap 3 → 3; custom 0 / missing number rejected.
- Select/sticky: Anthropic, OpenAI advanced+fallback, WS, Gemini all go through `admitsScheduleUser` + injected cache.

### 7. Wrong vs Correct

#### Wrong

```go
if policy.Enabled { // even when AccountIDs is empty
    return contains(policy.AccountIDs, account.ID)
}
redis.Expire(ctx, cooldownKey, shortUserTTL) // shortens sibling fields
```

#### Correct

```go
if policy := cache.EnabledPolicy(userID, SmartScheduleLookupPlatform(account, hint, bundle)); policy != nil {
    return admitSmartSchedule(ctx, account, policy, pairQuality)
}
return account.AdmitsScheduleUser(userID, live)
// Expire only if newTTL > current TTL
```

## Common Mistake: smart-schedule cooldown reads account 15m live

**Symptom**: one noisy user cools the pair because the shared account cell is bad; or `selectable` still fail-opens for 15 minutes.

**Cause**: `admitsScheduleUser` used `account-quality:live` p50/success, and `selectable` wrote `w:` via `MarkUserQualityWindow`.

**Prevention**: smart-schedule cooldown reads only `Q_{a,u}`. `selectable` calls `ClearUserResume`. `resumed` still uses the existing `u:`+`w:` overlay and zeros pair windows on enter.

## Common Mistake: enabled empty pool fail-closes the user

**Symptom**: last pool account is deleted (or save/cache is stale with `enabled=true` and zero members); that user gets no accounts on the platform even though legacy allow/deny would have worked.

**Cause**: `EnabledPolicy` treated `enabled=true` as a closed allow-list even when `AccountIDs` was empty, so every candidate was a pool miss.

**Prevention**: `EnabledPolicy` returns nil when `MemberCount()==0`. Admin GET also shows `enabled=false` for that empty row. Writes still reject `enabled=true` + empty members.

## Common Mistake: cooldown HASH Expire shortens another user

**Symptom**: user A has a 60-minute pair cooldown on account X; user B breaches the same account with a 15-minute cooldown; A’s remaining cooldown disappears after ~15 minutes.

**Cause**: cooldown lives in one HASH per account×platform (`smart-schedule:cooldown:{platform}:{accountID}`). A naive `Expire(key, B.cooldown)` cuts the key TTL for every field of that platform.

**Prevention**: `HSETNX` the field; set/extend key TTL only when the new expiry is later than the current TTL. Tests must cover two users on one HASH.

## Common Mistake: soft cooldown treats fail-open as a meet (or SoftEnd enters 考察)

**Symptom**: a cooling pair graduates to probe after one or two peer samples, or after a cache miss, even though N is not full. Or a true soft meet drops the pair into 考察 / 预检.

**Cause**: `softCooldownMeets` reused `pairQualityBlocks` / `!pairQualityBlocks` (underfull fail-open), ingested the request that just cooled account A into A’s new soft window, or `SoftEndCooldown` called `enterProbeFromCooldown`.

**Prevention**: meet only via `EvalQuality(..., SchedQualityKnobs) == pass` (AND-enter; underfull = pending). Soft-end goes **selectable** (`ClearProbing`, zero pair windows). Only hard countdown expiry calls `EnterProbe`. Soft windows are timestamped (`since = now - cooldown_minutes`); legacy blobs without `samples` are empty. Exclude self, skip pinned leftovers, start after that pair’s cooldown write. Hard policy never writes `smart-schedule:soft-cool`. Event is `soft_cooldown_end`, not `cooldown_end` or `probe_enter`.

## Common Mistake: soft cooldown save snaps back to hard

**Symptom**: admin picks 软 and saves; the switch immediately shows 硬 again. Other policy fields persist.

**Cause**: `applyPlatformView` sets `softCooldown = Boolean(view.soft_cooldown)`. A stale backend, a missing `soft_cooldown` column (migration 218 not applied), or a 0-row overlay UPDATE yields GET/PUT echo without `soft_cooldown: true`.

**Prevention**: overlay UPDATE must affect one policy row. After a successful PUT, `applyWrittenSoftCooldown` restores the flag from the write payload (same pattern as window N). Apply 218 before serving the new binary. Site quality template still omits this field.

## Common Mistake: pause lift defaults to probing

**Symptom**: clearing `paused` (or omitting `state` on the resume endpoint) drops the pair into 考察 and clamps cap to N.

**Cause**: treating omitted `state` = `resumed` as a pause-exit default, or writing the probe HASH when only the paused flag is cleared.

**Prevention**: `paused` has no automatic exit. `ParsePairAdmissionState("")` stays `resumed` (豁免期 write default). Enter probing only from **hard** cooldown expiry or an explicit `state=probing`. Soft-end is selectable. Tests must cover pause → omitted state ≠ probing.

## Common Mistake: reuse `resumed` for long-term exemption

**Symptom**: a pair that should stay exempt forever starts evaluating after 15–30 minutes, or omit-`state` silently pins everyone.

**Cause**: `pinned` was stored as `u:`/`w:` without TTL, or `ParsePairAdmissionState("")` was changed to `pinned`.

**Prevention**: `pinned` is a separate Redis HASH with no TTL. `resumed` still writes the 15m/30m overlay. Empty `state` stays `resumed`. Tests must cover omit ≠ pinned and N successes while pinned do not cool.

## Common Mistake: leftover probe mark clamps a pinned pair

**Symptom**: a long-exempt pair is limited to `probe_cap` / window N even though the admin chose 长期豁免.

**Cause**: `resolvePairSlotAcquire` checked `IsProbing` before `IsPinned`. Enter pin clears the probe mark, but a leftover probe field would still clamp the member cap.

**Prevention**: check `IsPinned` first and return the member cap. Tests must cover leftover probe + pin.

## Common Mistake: leftover `u:`/`w:` blocks probe graduate

**Symptom**: pair last-N already has N counted completes (often N successes) but the pair stays `probing`. Account/user quality cells may also show “successes > N”.

**Cause**: leftover pair `smart-schedule:resume` `u:`/`w:` (or historically the shared `account-quality:resume` HASH) stayed live when entering probe. `ObservePairCompletion` / `admitsScheduleUser` then skipped evaluate, so windows filled past N with no graduate.

**Prevention**: enter probe always `ClearPairResume` for that platform. While `IsProbing`, do not skip evaluate for leftover pair grace. 豁免期 fail-open is only when there is no probe mark. Track A `account-quality:resume` must not skip smart-schedule evaluate. Tests must cover “N successes in probe + leftover pair resume → graduate”.

## Common Mistake: unpooled cheap-tier escape uses group platform

**Symptom**: mixed / Claude-GPT bridge clears an Antigravity (or OpenAI) pool session pin because the group is `anthropic`.

**Cause**: `shouldEscapeSessionStickyForCheaperTier` called `lookupEnabledSmartPolicy(..., group.Platform)`.

**Prevention**: same ruler as `admitsScheduleUser` — `lookupEnabledSmartPolicy(ctx, lookup, userID, sticky.Platform)`. `userID<=0` stays today's fail-open (unpooled), not a new pooled policy. `previous_response` must not use this escape.

## Common Mistake: leftover `AllowsScheduleUser` on a select path

**Symptom**: VIP quality gate works on Anthropic but not OpenAI / Gemini / WS.

**Cause**: a sticky or candidate site still calls identity-only `AllowsScheduleUser`.

**Prevention**: production selection/sticky-clear must go through `admitsScheduleUser` / `AdmitsScheduleUser` + live cache. After edits, `rg AllowsScheduleUser` in `backend/internal/service` — leftover sites should only be the identity function, `AdmitsScheduleUser` internals, and identity tests.

## Common Mistake: not every select path reselects

**Symptom**: pair-full returns WaitPlan or `ErrNoAvailableAccounts` even though another account is free.

**Cause**: only the main load-aware loop was updated; model-routing wait and OpenAI `LoadBatchEnabled=false` still treated pair-full like account-full.

**Prevention**: grep `tryAcquireAccountSlot` / `WaitPlan` after any pair-cap change and keep `tryAcquireAccountAndPairSlot` + `if pairFull { exclude; continue }`. Layer 3 / routing wait must skip IDs that just returned `pairFull` in this request (`pairFullIDs`); do not WaitPlan from the pre-acquire candidate list. After `AcquireAccountSlotWithWaitTimeout`, attach the pair slot (`AttachPairSlotAfterAccountWait`); never forward holding only the account slot.

## Common Mistake: WaitPlan after a same-request pairFull acquire

**Symptom**: snapshot occupancy is still `< N`, but `tryAcquireAccountAndPairSlot` returns `pairFull`; the request waits on that account or forwards after waking with only the account slot.

**Cause**: Layer 2 records `pairFull` by removing the account from `available`, then Layer 3 / routing wait iterates the original `candidates` (or stale `routingPairCounts`). Handler wait paths call `AcquireAccountSlotWithWaitTimeout` and skip `concurrency:account_user:{accountID}:{userID}:{platform}`.

**Prevention**: keep a this-request `pairFullIDs` set and skip it in every WaitPlan loop. Wake must `AttachPairSlotAfterAccountWait` (hold account slot + `AcquireAccountUserSlot`); on `pairFull` release the account slot and reselect. Do not treat Recovered ops rows as pair-full.

## Scenario: AG pool OpenAI dual-membership + lookup platform

### 1. Scope / Trigger

- Trigger: Antigravity smart-schedule tab may hold OpenAI accounts (bridge on or off). Same account may also sit in the openai pool. This is a cross-layer contract (PK, sanitize, lookup helper, Redis keys, admin resume/hydrate, AG candidate merge).
- Do not change `ResolveClaudeGPTBridgeModel`, empty-stream converters, stored billing, or `actual_cost`.
- Scheduler eligibility for Claude→GPT bridge stays `account.Platform == openai`. The **policy / Redis** key is `SmartScheduleLookupPlatform`, not the scheduler platform.

### 2. Signatures

- `smartScheduleAccountMatchesTab(acc, tab)`: same-platform always matches; **only** extra exception is `tab==antigravity && acc.IsOpenAI()`. Anthropic / gemini / grok tabs stay locked.
- `SmartScheduleLookupPlatform(account, hint, bundle)`: OpenAI + (`hint.RequireClaudeGPTBridge` or `hint.GroupPlatform==antigravity`) → `antigravity` only when `bundle.EnabledPolicy(antigravity) != nil`; else `openai`. Native OpenAI groups and non-OpenAI accounts stay `account.Platform`. Hint from `ctxkey.RequireClaudeGPTBridge`, `ctxkey.Group.Platform`, else `ctxkey.ForcePlatform`. The helper must see the user policy bundle (or equivalent) so admission, pair slots, unpooled, Observe, and hydrate share one rule.
- `uniqueSmartScheduleMembershipPlatform(bundle, accountID)`: the single membership platform, or `""` when the account is in more than one pool.
- `SmartScheduleRedisPlatform(platform)`: empty → `_` so unset callers cannot collide with a real platform.
- DB PK `user_smart_schedule_accounts (user_id, platform, account_id)` via additive migration `211_user_smart_schedule_account_pk.sql`. Keep `idx_user_smart_schedule_accounts_account_id`. Do not edit 202/204/207/208.
- Admin write: `PUT /admin/users/:id/smart-schedule/:platform` (tab is the member `platform`). Resume: `POST /admin/accounts/:id/smart-schedule-resume` `{user_id, state?, platform?}`. Pair-quality batch/detail take the current tab platform.
- Redis occupancy / cooldown / soft-cool / probe / pin / pair-quality / pair resume: platform is in the KEY (see the user×platform scenario above). Do not read pre-211 keys as fallback.

### 3. Contracts

- AG tab PUT may include OpenAI ids without `openai_claude_gpt_bridge_enabled`. Member row `platform` is the **tab** (`antigravity`), not `account.Platform`.
- Adding an OpenAI account to the AG pool must not delete the openai-pool row (decision B).
- Bridge / AG-group OpenAI traffic looks up **openai** while AG is off/empty/missing, and **antigravity** only while AG is enabled with members. Admission, pair acquire, unpooled cheaper-tier, `ObservePairCompletion`, and hydrate must share that helper. Native OpenAI groups keep `openai`.
- AG policy nil / empty / disabled → keep the openai closed pool (do **not** fail-open account-side just because AG is off). Once AG is on, a pool miss rejects; **never** fall back to the openai pool.
- `ObservePairCompletion.Platform` must be the request lookup platform. Empty platform + dual membership → **do not ingest** (do not pick the first `HasAccount`).
- Pool-page resume / pair-quality hydrate pass tab `platform`. Account-page omitted `platform` resolves to `account.Platform` only and must not mutate the AG dual-membership row.
- Admin AG candidates: merge `platform=antigravity&lite=1` and `platform=openai&lite=1`; filter-add may choose antigravity / openai / all-in-this-tab. Do not pull anthropic / gemini / grok on the AG tab.
- Pool column `claude_gpt_bridge` is read-only extra. Non-OpenAI cells render `—`. Not a PUT field.
- Deploy residual: old Redis keys without `{platform}` become orphans and expire by TTL. In-flight occupancy / cooldown reset. Do not dual-read old keys.

### 4. Validation & Error Matrix

| Condition | Error / runtime |
| --- | --- |
| OpenAI id on AG tab PUT | 200; member stored as `platform=antigravity` |
| OpenAI id on anthropic / gemini / grok tab | `SMART_SCHEDULE_PLATFORM_MISMATCH` |
| Duplicate id in one PUT | `SMART_SCHEDULE_DUPLICATE_ACCOUNT` |
| Bridge extra off, account in AG pool, `schedulable=true` | Pool write OK; `ResolveClaudeGPTBridgeModel` still false; bridge select misses the account |
| AG `EnabledPolicy` nil/empty | Keep openai closed pool if that policy is enabled; not account-side fail-open |
| `ObservePairCompletion` empty platform + two memberships | No ingest |
| Resume omit `platform` on account page | Mutates `account.Platform` row only |

### 5. Good / Base / Bad Cases

- Good: user 12 with AG off / no AG pool: group 15 bridge admits via the openai closed pool (same as today). With AG on and members: group 15 admits via antigravity only; group 19 native GPT still uses the openai pool and openai Redis keys.
- Good: same OpenAI id cooled on openai does not cool the AG pair (and the reverse).
- Base: no AG policy → group 15 keeps today's openai closed pool. Deploy with the switch off is a no-op for user 12.
- Bad: `SelectAccountWithSchedulerForClaudeGPTBridge` platform rewritten to antigravity (zero OpenAI candidates).
- Bad: AG disabled / empty / missing fail-opens to account-side allow/deny (that makes the switch change production on deploy).
- Bad: AG enabled still falls back to `lookupEnabledSmartPolicy(..., openai)` after an AG pool miss.
- Bad: `ObservePairCompletion` map-range first `HasAccount` when both pools contain the id.

### 6. Tests Required

- `sanitizePoolMembers`: OpenAI→AG allowed without bridge extra; OpenAI→anthropic still `SMART_SCHEDULE_PLATFORM_MISMATCH`; native AG→AG still allowed.
- `SmartScheduleLookupPlatform`: AG off/empty/missing + bridge/AG-group OpenAI → openai; AG on with members → antigravity; native GPT → openai.
- Dual persist: one `account_id` two rows; AG PUT does not drop the openai row.
- Cooldown / occupancy / probe / pin / pair-quality / pair-resume isolation across platform.
- `ObservePairCompletion` dual-membership without platform skips ingest.
- AG disabled / empty / missing: bridge admits via openai closed pool; out-of-openai-pool rejected if openai is enabled.
- AG on with members: openai-only members rejected; no fail-open.
- `ResolveClaudeGPTBridgeModel` still false when extra is off.
- Frontend: AG tab can add OpenAI candidates; `claude_gpt_bridge` column visible; other tabs stay locked.
- Empty-stream tests unchanged.

### 7. Wrong vs Correct

#### Wrong

```go
req.Platform = PlatformAntigravity // SelectAccountWithSchedulerForClaudeGPTBridge
key := PlatformAntigravity         // even when AG is off
policy := cache.EnabledPolicy(userID, key)
if policy == nil {
    return account.AdmitsScheduleUser(userID, live) // fail-open because AG is off
}
```

#### Correct

```go
// scheduler eligibility stays openai; closed-pool key follows the AG switch
key := SmartScheduleLookupPlatform(account, hint, bundle)
if policy := cache.EnabledPolicy(userID, key); policy != nil {
    return admitSmartSchedule(ctx, account, policy, pairQuality)
}
return account.AdmitsScheduleUser(userID, live)
```

## Common Mistake: AG-off fail-open because lookup always returns antigravity

**Symptom**: Deploying with the AG switch off changes user 12: group 15 bridge stops using the openai closed pool and fail-opens to account-side allow/deny.

**Cause**: `SmartScheduleLookupPlatform` returned `antigravity` for every OAI + bridge/AG-group request. `EnabledPolicy(antigravity)` was then nil, so admission treated the user as unpooled.

**Prevention**: The helper must read the user policy bundle. AG nil / disabled / empty keeps `openai`. Only `EnabledPolicy(antigravity) != nil` switches the lookup key. Admission, pair slots, unpooled, Observe, and hydrate must share that helper.

## Common Mistake: rewrite Claude-GPT scheduler platform to antigravity

**Symptom**: group 15 bridge returns no accounts even though OpenAI ids are in the AG pool.

**Cause**: `isOpenAIAccountEligibleForScheduleRequest` requires `account.Platform == schedulerPlatform`. Changing the scheduler platform to antigravity drops every OpenAI candidate.

**Prevention**: keep `SelectAccountWithSchedulerForClaudeGPTBridge` on openai. Change only `SmartScheduleLookupPlatform` / Redis / pair ingest.

## Common Mistake: Observe / hydrate picks the first dual-membership pool

**Symptom**: group 15 completion writes openai pair-quality, or AG 豁免期 fail-opens the openai pool.

**Cause**: empty `obs.Platform` plus `for platform, policy := range bundle.Policies { if policy.HasAccount(...) }`, or resume/pair-quality batch keyed by `account_id` only.

**Prevention**: stamp the request lookup platform. If platform is still empty and `uniqueSmartScheduleMembershipPlatform` returns `""`, skip ingest. Pool-page APIs pass tab `platform`. Account-page omit stays `account.Platform`.

## Common Mistake: AG pool hydrate uses `platform=antigravity&ids=`

**Symptom**: Adding an OpenAI account (e.g. loveapi) to the AG tab succeeds, then the row vanishes after add / Save / refresh. Candidates still list it.

**Cause**: `loadPoolDetails` called `GET /admin/accounts?platform=antigravity&ids=...`. The accounts list intersects `ids` with `account.Platform`, so OpenAI ids never come back. `poolAccounts` then filters out missing ids.

**Prevention**: Fetch AG pool rows by `ids` + `lite=1` only (no platform), or merge AG+OpenAI lists. Candidate merge is not enough.

## Common Mistake: read old Redis keys after the platform shard

**Symptom**: AG cooldown HASH TTL still cuts the openai sibling, or deploy “keeps” in-flight state by dual-reading.

**Cause**: fallback `GET smart-schedule:cooldown:{accountID}` when the new `{platform}` key misses.

**Prevention**: all read/write/clear/hydrate/delete use `{platform}` keys. Old keys are orphans; accept the deploy reset. Do not dual-read.

## Common Mistake: user cache JSON drops sched N/K/C

**Symptom**: Admin GET shows sched K/C, every pair cooldown reason is p50, many single-request first-token overruns do not cool.

**Cause**: `cachedSmartSchedulePolicy` omitted latency-gate columns. Lookup cache hit → `SchedCompositeEnabled()` false → `pairQualityLegacyP50Blocked` (window = N首字).

**Prevention**: `cachedSmartScheduleBundleFrom` / `toBundle` must copy `quality_max_slow_in_window`, `quality_max_consecutive_slow`, `quality_max_p50_duration_ms`, `quality_sched_window_n`, `quality_sched_max_slow_in_window`, `quality_sched_max_consecutive_slow`. Round-trip tests must not depend on `soft_cooldown`. After a fix deploy, stale JSON stays legacy until TTL, PUT invalidate, or `DEL smart-schedule:user:{id}`.
