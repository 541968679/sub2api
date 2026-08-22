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
- Redis pair slot: `concurrency:account_user:{accountID}:{userID}` (zset, same Lua as account/user slots).
- Redis live quality: `account-quality:last-n:{accountID}` is the \(Q_a\) source of truth (site-wide N, all users; TTL 7d). Completions ingest and project JSON to `account-quality:live:{accountID}` (no resume fields). Reader = selection `Get` (prefers last-N, then legacy live JSON). Cache miss / nil stats fail open. The 5-minute tick must not `Replace` live from a 15-minute SQL window. Pair cooldown stays on \(Q_{a,u}\), not this account cell. Admin user-list quality is a separate Redis window `user-quality:last-n:{userID}` (\(Q_u\), same N, this user across all accounts) and does not feed gates or hard-close.
- Redis resume overlay: `account-quality:resume:{accountID}` (HASH, TTL 20m). `MarkUserResume` / `MarkAccountResume` write HASH fields only (`u:{userID}`, `a`). `Get` merges the overlay onto last-N / live stats. Do not SCAN-delete last-N keys. Resume HASH must survive even when an account has no last-N key. Legacy live JSON `resume_users` / `account_resume_until` is migrated with `HSETNX` then stripped from the live key.
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

- Trigger: user-page per-`account.Platform` closed pool + quality + pair cooldown + pool-member cap. Cross-layer: two SQL tables, user Redis cache, pair cooldown HASH, pair-quality windows, every `admitsScheduleUser` path, UsersView modal.
- Independent of `account_schedule_users`. Do not write this policy onto shared account scheduler snapshots. Do not fold into `IsSchedulable()` or `SetTempUnschedulable`.
- Selection key is `account.Platform` (mixed-scheduling Antigravity accounts use the antigravity policy).
- Track A account quality (`account-quality:live`, 15m / 5min cells, hard-close) is unchanged. Smart-schedule cooldown must not read that live quality number.
- Track B pair quality `Q_{a,u}` is `(account_id, user_id)` only and feeds smart-schedule cooldown + the pool 配对质量 column.

### 2. Signatures

- DB `user_smart_schedule_policies`: unique `(user_id, platform)`; `enabled bool`; quality columns same shape as pair gates; one window size N stored in both `quality_min_success_samples` and `quality_min_ttft_samples` (default 10, clamp **1–100**). API field `quality_window_samples` (alias `quality_window_n`). Probe in-flight: `probe_concurrency_mode` (`follow_n` default / `custom`) + optional `probe_concurrency` (1–100, required when custom). This is **not** `account_quality_window_n`. `cooldown_minutes` 1–1440 default 15. Platforms: `anthropic|openai|gemini|antigravity|grok`.
- DB `user_smart_schedule_accounts`: PK `(user_id, account_id)`; redundant `platform`; `max_concurrency` null or ≥1; `paused bool` default false (migration 207). Member `account.platform` must equal row `platform`. CASCADE from users/accounts. PUT replace-all restores `paused` for members that remain. Client writes ignore `paused`.
- Domain: `SmartSchedulePolicy` / `EnabledPolicy(userID, platform)` — nil when missing, disabled, or `MemberCount()==0`.
- Redis user cache: `smart-schedule:user:{userID}` JSON of all platforms; invalidate on PUT/copy.
- Redis cooldown: `smart-schedule:cooldown:{accountID}` HASH `u:{userID}=untilUnix`. Hot-path `StartCooldown` is `HSETNX` only (no extend). Admin switcher `SetCooldown` uses `HSET` overwrite. TTL may only be lengthened, never shortened (other users share the key).
- Redis pair quality: `smart-schedule:pair-quality:{accountID}` HASH `u:{userID}` = two FIFO windows (`W_ttft`, `W_ok`). Trend list `smart-schedule:pair-quality-trend:{accountID}:{userID}` (TTL 24h). Event list `smart-schedule:pair-quality-events:{accountID}:{userID}` (TTL 7d).
- Admin: `GET /admin/users/:id/smart-schedule`; `PUT /admin/users/:id/smart-schedule/:platform`; `POST .../copy` `{from_platform}`; `POST /admin/accounts/:id/smart-schedule-resume` `{user_id, state?}`. Pair quality: pool member `pair_quality` + `will_cool`; `POST /admin/users/:id/smart-schedule/pair-quality`; `GET /admin/users/:id/smart-schedule/pair-quality/:accountId`; `GET /admin/users/:id/smart-schedule/:platform/accounts/:account_id/pair-quality`. `state` is `paused|cooling|probing|resumed|selectable|pinned`; omitted `state` is `resumed` (豁免期 write default, **not** `pinned`, **not** a pause-lift default, and **not** `probing`). Invalid `state` → `SMART_SCHEDULE_ADMISSION_INVALID`. Pause of a non-member → `SMART_SCHEDULE_UNKNOWN_ACCOUNT`. Redis probe HASH: `smart-schedule:probe:{accountID}` field `u:{userID}`. Redis pin HASH: `smart-schedule:pinned:{accountID}` field `u:{userID}`, **no TTL**. Miss / no mark = not probing / not pinned (no deploy backfill). GET hydrates `pinned: true`. Do **not** reuse `resumed` for long-term exemption.
- Pool account details: existing `GET /admin/accounts?platform=&ids=`.

### 3. Contracts

PUT body: `{enabled, quality_*, quality_window_samples|quality_window_n, probe_concurrency_mode, probe_concurrency, cooldown_minutes, accounts:[{account_id, max_concurrency?}]}`. Copy copies enabled/thresholds/N/cooldown/probe settings only, never members or caps. Do not copy account-quality N into `probe_concurrency`. GET shows `enabled=false` when the stored row is enabled but the pool is empty. GET echoes `quality_window_samples`, `quality_window_n`, and both old sample fields as the same N, plus `probe_concurrency_mode` / `probe_concurrency` (omit/empty mode → `follow_n`).

Resume (`state=resumed` / 豁免期) is **manual only**. It `HDEL`s that pair’s cooldown field, **zeros both pair windows**, clears `paused` and the probe mark, and writes `account-quality:resume` `u:`+`w:` grace. Completions during grace **do enter** the windows; evaluate is skipped until `u:`/`w:` expire. After grace the pair is 调度 (`selectable`): **keep** in-grace windows, then evaluate with selectable rules (may cool immediately). Grace end does **not** enter 考察.

`selectable` (调度) clears `paused`, `HDEL`s cooldown, **zeros both windows**, `ClearUserResume` (deletes `u:` **and** `w:`), and clears the probe mark. There is **no** 15-minute watching fail-open. Re-accumulate N before cooldown.

`probing` (考察) is entered by **cooldown wall-clock expiry** or by admin (including 调度→考察). Enter: `HDEL` cooldown, **zero both windows**, `ClearUserResume` (no `u:`/`w:`), `HSET` probe mark. In-flight pair cap = `min(desired, member cap)` or desired if unset. `desired` is window N when `probe_concurrency_mode=follow_n` (omit/empty), or `probe_concurrency` (1–100) when `custom`. Member `max_concurrency` is a hard ceiling. This is **not** account-quality global N. Invalid custom (0, >100, custom without a number) → `SMART_SCHEDULE_INVALID_QUALITY`; do not silently fall back. During probing, ingest and evaluate `Q_{a,u}`. Graduate → 调度: **keep** windows, `HDEL` probe mark, lift to member cap. Probe cool uses the same or/and as pair cooldown (unfilled metrics do not participate) plus a **probe-only** override: `and` + both windows full + one pass one fail → `StartCooldown` (anti-deadlock). That override must **not** change `and` for selectable. Graduate requires `W_ok` full N; configured success-rate threshold must pass; empty/unfilled `W_ttft` does **not** block; if `W_ttft` is full, p50 must pass. No time-based graduate. No traffic → stay probing.

`cooling` clears `paused` and the probe mark, `HSET`s cooldown to `now+policy.cooldown_minutes` and deletes `u:`/`w:`. Cooling/paused pairs do not ingest. Auto cooldown expiry enters **考察**, not 调度: `HDEL` + zero windows + write probe mark + `ClearUserResume`, no time grace.

`paused` is **long-lived manual only**. Sets the membership flag, invalidates `smart-schedule:user:{userID}`, `HDEL`s cooldown, deletes `u:`/`w:`, and **clears** the probe mark and pin mark. **No implicit unpause** and **no default next state**. Leaving pause requires an explicit `state` ∈ {`probing`,`selectable`,`resumed`,`cooling`,`pinned`} (or write `paused` again). Clearing the paused flag must not write `probing` or `pinned`.

`pinned` (长期豁免) is **manual only**. Enter (`state=pinned` only): `HDEL` cooldown, clear probe mark, `ClearUserResume` (do **not** write `u:`/`w:`, do **not** `MarkUserResume`), `HSET` pin mark, **keep** pair windows. Full member cap. Windows may keep ingesting. **Never evaluate, never `StartCooldown`** until the admin leaves. Leave requires an explicit next state (`paused` / `cooling` / `probing` / `selectable` / `resumed`). **No implicit timeout**. Cooldown expiry still → `probing`, never `pinned`.

Hot path (`admitsScheduleUser`): `EnabledPolicy` hit → pool miss reject; pool hit ignores that pair’s allow/deny/gate/cap; `paused` reject (account stays in the pool); **`pinned` admit and skip evaluate / `StartCooldown`** (check pin **before** leftover cooldown); active cooldown reject; 豁免期 `u:`/`w:` fail-open (no evaluate, no graduate) **only when not probing**; leftover resume during probing still evaluates (graduate / and-mixed); probing vs selectable then judge **only** `Q_{a,u}` (pair windows). Unfilled metrics do not participate (same or/and as account track). Count `< N` → that metric does not cooldown. Selectable breach → `HSETNX` cooldown then reject. Probing may graduate (keep windows) or cool (including and-mixed override). Pair cap uses pool member N, except while probing (`resolvePairSlotAcquire` / pair occupancy must honor probe cap). `pinned` uses the member cap, not the probe cap. Account hard-close / `IsSchedulable()` still applies (whole-account gate). Otherwise the legacy scenario in this file. Do not fold pause, probing, or pin into `IsSchedulable()`. Do not backfill existing pairs on deploy.

Ingest: failure → `W_ok` only; sync success without first token → `W_ok` only; streaming success with `true_first_token_ms` or `first_token_ms` → both. `W_ok` uses the same counted-error policy as account track (`schedule_use_failover_error_rate`) via `ClassifyOpsErrorRateCalibers` — client noise, group-no-account (any phase/status), routing 503, and protocol mismatch must not write `Success=false`. Full families: `ops-schedule-error-caliber.md`. Cooling / paused pairs do not ingest. `pinned` pairs **do** ingest and must not evaluate / `StartCooldown`. `will_cool` on the pool row uses pair windows + saved thresholds, not account 15m cells; skip `will_cool` while pinned.

### 4. Validation & Error Matrix

| Condition | Error / runtime |
| --- | --- |
| `enabled=true` and zero members | `SMART_SCHEDULE_EMPTY_POOL` (write). Runtime: treat as disabled (legacy). |
| Unknown / wrong-platform account id | `SMART_SCHEDULE_INVALID_ACCOUNT` |
| `cooldown_minutes` out of 1–1440 | `SMART_SCHEDULE_INVALID_COOLDOWN` |
| Invalid quality metric / condition / N outside 1–100 / invalid probe concurrency | `SMART_SCHEDULE_INVALID_QUALITY` |
| Copy `from_platform` missing or same | `SMART_SCHEDULE_COPY_INVALID` |
| `userID<=0` or cache miss | legacy rules only |

### 5. Good / Base / Bad Cases

- Good: OpenAI enabled + pool `[A,B]` + A pair-window p50 breach (full N) → A excluded and cooled; B still selectable; Anthropic requests for the same user stay legacy. Account 15m live breach alone must not cool.
- Base: no policy rows → identical to the allow/deny/gate/cap scenario above.
- Good: last pool member CASCADE-deleted → `EnabledPolicy` nil → legacy, not platform-wide empty reject.
- Bad: copy OpenAI members onto Gemini. Bad: `Expire` a shared cooldown HASH down to a shorter user’s TTL. Bad: snapshot-copy user policy.

### 6. Tests Required

- `EnabledPolicy`: off / empty / one member; pool miss vs hit; ignore legacy deny/gate/cap when enabled.
- Cooldown: `HSETNX` no-extend; expiry zeros pair windows then enters probing (no `w:`, `ClearUserResume`, **never pin**); resume deletes only that pair and zeros windows; HASH TTL never shortens another user. `selectable` must not `MarkUserQualityWindow`. Probe: expiry→probe; graduate `W_ok`-only (sync, no TTFT); leftover `u:`/`w:` must not skip graduate; probe `and` mixed→cool; pause does not auto-probe or auto-pin; 调度→考察 zeros; no backfill. Pin: enter/leave; omit `state` ≠ pinned; N successes while pinned do not cool; leave to selectable can cool again.
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
if policy := cache.EnabledPolicy(userID, account.Platform); policy != nil {
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

**Cause**: cooldown lives in one HASH per account (`smart-schedule:cooldown:{accountID}`). A naive `Expire(key, B.cooldown)` cuts the key TTL for every field.

**Prevention**: `HSETNX` the field; set/extend key TTL only when the new expiry is later than the current TTL. Tests must cover two users on one HASH.

## Common Mistake: pause lift defaults to probing

**Symptom**: clearing `paused` (or omitting `state` on the resume endpoint) drops the pair into 考察 and clamps cap to N.

**Cause**: treating omitted `state` = `resumed` as a pause-exit default, or writing the probe HASH when only the paused flag is cleared.

**Prevention**: `paused` has no automatic exit. `ParsePairAdmissionState("")` stays `resumed` (豁免期 write default). Enter probing only from cooldown expiry or an explicit `state=probing`. Tests must cover pause → omitted state ≠ probing.

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

**Cause**: `UserQualityResumeActive` is shared with 豁免期 and account-quality 立即恢复. `expirePairCooldown` used to `HDEL` cooldown + zero windows + mark probe without `ClearUserResume`. `ObservePairCompletion` / `admitsScheduleUser` then skipped evaluate while grace was live, so windows filled past N with no graduate. Grace lasts 15–30m and can be refreshed.

**Prevention**: enter probe always HDEL `u:`/`w:`. While `IsProbing`, do not skip evaluate for leftover grace. 豁免期 fail-open is only when there is no probe mark. Tests must cover “N successes in probe + leftover resume → graduate”.

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

**Cause**: Layer 2 records `pairFull` by removing the account from `available`, then Layer 3 / routing wait iterates the original `candidates` (or stale `routingPairCounts`). Handler wait paths call `AcquireAccountSlotWithWaitTimeout` and skip `concurrency:account_user:{accountID}:{userID}`.

**Prevention**: keep a this-request `pairFullIDs` set and skip it in every WaitPlan loop. Wake must `AttachPairSlotAfterAccountWait` (hold account slot + `AcquireAccountUserSlot`); on `pairFull` release the account slot and reselect. Do not treat Recovered ops rows as pair-full.
