# Ops schedule error caliber and attention alerts

## Scenario: pair/account schedule ingest must not treat client or routing misses as hop failures

### 1. Scope / Trigger

- Trigger: an `ops_error_logs` row is classified for (a) pair-quality / account last-N ingest, (b) account-dimension 15m SQL `ErrorCount`, (c) admin error-list badges/filters, (d) the dedicated `ops_attention_count` alert.
- Cross-layer: `ClassifyOpsErrorRateCalibers` is the Go source of truth. SQL twins must stay in lockstep. List filter and alert count use the SQL twins, not in-memory paging.
- Schedule-exclude toggles do **not** change SLA / user error rate, compare rate, billing / `actual_cost`, Claude–GPT bridge exclusion, or `classifyNoAccountError` 404/503 routing.
- Client HTTP stays on the legacy status map until an admin sets a replacement template on the same Settings KV. Factory templates reproduce the old gateway / OpenAI / Gemini sentences. Account error passthrough, Anthropic 4xx body replay, model-not-found, and silent refusal still run before that resolver.
- Does **not** drop the phase/status safety rails on `IsAccountQualityRoutingModelMiss` as a whole. The 502 “no account in group supports this model” leak is a **separate** family that ignores phase/status.
- `needs_ops_attention` / `ops_attention_count` do **not** follow the schedule whitelist. Group-model / routing 503 / protocol still page even if that family is unchecked (then the row both cools and alerts — that is “count toward schedule again”).

### 2. Signatures

Go (names may match these or obvious aliases; tests lock behavior):

```go
func IsGroupNoAccountForModel(message, body string) bool
func IsScheduleClientNoise(status int, phase, errorType, message string) bool
func IsScheduleProtocolMismatch(message string) bool
func IsOpsAttentionError(status int, phase, errorType, message, body string) bool
func IsScheduleQualityExcluded(status int, phase, errorType, message, body string) bool
func IsScheduleQualityExcludedWith(status int, phase, errorType, message, body string, wl ScheduleErrorWhitelist) bool
func IsScheduleQualityExcludedMatch(in ScheduleErrorMatchInput, wl ScheduleErrorWhitelist) bool

func SQLGroupNoAccountForModelPredicate() string
func SQLScheduleClientNoisePredicate() string
func SQLScheduleProtocolMismatchPredicate() string
func SQLOpsAttentionPredicate() string
func SQLScheduleQualityExcludedPredicate() string
func SQLScheduleQualityExcludedPredicateWith(prefix string, wl ScheduleErrorWhitelist) string
```

`OpsErrorRateCalibers` gains `NeedsOpsAttention bool`. Existing three counted-in flags stay.

List: `OpsErrorLog.NeedsOpsAttention bool` (`needs_ops_attention`).  
Filter: `OpsErrorLogFilter.NeedsOpsAttention *bool` from query `needs_ops_attention=true|false`.

Alert metric: `ops_attention_count` (window count of rows matching `SQLOpsAttentionPredicate`, optional existing `platform` / `group_id` filters).  
Not a percent metric.

Default seeded rule name: `需运维：组模型/路由/协议`. Unique on `ops_alert_rules.name`. `ON CONFLICT (name) DO NOTHING`.

### 2.1 Schedule error whitelist (preset families + custom rules)

Settings KV key: `schedule_error_whitelist` (`SettingKeyScheduleErrorWhitelist`).

JSON shape:

```json
{
  "families": {
    "client_invalid_request": false,
    "client_wrapped_400_urf": false,
    "client_context_too_long": false,
    "pair_concurrency": false,
    "group_no_account": false,
    "routing_pool_empty": false,
    "protocol_mismatch": false
  },
  "custom": [
    {
      "id": "c_01h...",
      "enabled": true,
      "error_type": "upstream_error",
      "phase": "upstream",
      "status_code": 400,
      "provider_error_code": "channel:no_available_key",
      "message_contains": "",
      "replace_with": "",
      "schedule_exclude": true
    }
  ],
  "replacements": [],
  "family_replace": {
    "client_invalid_request": "",
    "client_wrapped_400_urf": "",
    "client_context_too_long": "",
    "pair_concurrency": "",
    "group_no_account": "",
    "routing_pool_empty": "",
    "protocol_mismatch": ""
  }
}
```

`schedule_exclude` omitted on a custom rule means the old behavior: enabled ⇒ exclude. Explicit `false` replaces the client sentence only and the row still counts toward schedule.

`replacements` omitted or empty at resolve time means the factory status map (401/403/429/529/5xx plus locked catch-all `status_default`). GET expands that factory list for the admin page. Builtin match fields (status codes) stay factory-owned. Admins edit `enabled`, `template`, and `schedule_exclude`. The catch-all is locked: `enabled=true`, `schedule_exclude=false`. Unknown template ids are rejected. `family_replace` values are catalog template ids; empty means that family does not rewrite the client. Unknown family-replace keys, including `routing_model_miss`, are dropped.

Client resolve order for an upstream failure that reached `mapUpstreamError`: OpenAI wait-timeout is forced to 502 `Upstream service temporarily unavailable` (a custom rule cannot leak the marker or turn it into another sentence); then the first enabled custom rule with `replace_with` whose phase is empty or `upstream`; then `family_replace` matched as phase `upstream` (so `client_invalid_request` and `routing_pool_empty` do not rewrite client HTTP); then enabled builtin status rules by priority; then the catch-all. Gateway 529 uses `overloaded_error`; OpenAI and Gemini paths use `upstream_error` for that same sentence. Gemini ignores `error.type`.

`families` `true` = in whitelist = **exclude** from pair cooldown / account last-N / account 15m schedule `ErrorCount` (`CountedInAccountScheduleRate`).  
Uncheck a family = that family counts toward schedule again.  
Missing key / `{}` / `families: {}` / invalid JSON / all `false` + empty `custom` = **no new excludes**. Same schedule ingest as production before this feature.

Legacy `IsAccountQualityRoutingModelMiss` (phase≠upstream, status∈400/403/404/503, model-not-found / whitelist wording) is **hardcoded** and is **not** a checkbox. Putting it on the whitelist with default false would let 404 `model_not_found` cool accounts again. Save may still accept `routing_model_miss` from old payloads; it is dropped on persist and has no effect.

Custom rules: filled fields AND. Empty field = do not constrain. `message_contains` is a case-insensitive **literal** substring against `error_message` / `error_body` / `upstream_error_message` / `provider_error_code`. SQL LIKE must escape `\` `%` `_` `'`; no free wildcards / raw SQL. Cap 50 rules; message ≤ 200. Disabled rules persist but do not match.

PUT `/admin/settings/schedule-error-whitelist`: omit `custom` = keep existing custom; `custom: []` = clear. Unknown family ids still 400.

POST `/admin/settings/schedule-error-whitelist/from-error` `{error_id, mode: structured|message, replace_with?}`: server re-reads the log. `structured` prefills `error_type` + `phase` + `status_code` + `provider_error_code` (no full message). `message` uses the list primary text. Same fingerprint → enable existing. A non-empty `replace_with` updates the stored sentence; an empty one keeps the sentence already stored. 502 + `upstream request failed` is rejected.

UI: one global page `/admin/error-policy` and the same `ScheduleErrorWhitelistPanel` on the `UsageErrorInspectDialog` whitelist tab. The page configures preset status mappings, family schedule checkboxes, family replacement, and custom rules together. From-log actions live on `OpsErrorLogTable` and `OpsErrorDetailModal` (Ops / Usage / inspect share the table) and can attach `replace_with`. Not a SettingsView card. UI opens with every new family unchecked and `family_replace` empty. Replacement and schedule exclude are both account-independent.

| id | Default on | Match |
| --- | --- | --- |
| `client_invalid_request` | no | `error_type=invalid_request_error` **and** `error_phase=request` |
| `client_wrapped_400_urf` | no | status=400 and message contains `upstream request failed` (must look at status) |
| `client_context_too_long` | no | 413 / prompt too long / context window / array too long |
| `pair_concurrency` | no | 429 + `Concurrency limit exceeded for account` |
| `group_no_account` | no | group-has-no-account wording, any phase/status (502 group-gap is new; 404 group-gap is already covered by the hardcoded miss) |
| `routing_pool_empty` | no | `error_phase=routing` and status=503 |
| `protocol_mismatch` | no | Chat Completions endpoint / Unsupported content type / Invalid URL |

Hot read: short cache or settings invalidation. `ClassifyOpsErrorRateCalibers` and `SQLScheduleQualityExcludedPredicateWith` must use the same snapshot, including `provider_error_code` / `upstream_error_message` on the Go input. Account 15m `ErrorCount` uses the generated exclude; user-dimension quality SQL must not. List badges / 15m SQL recompute on the next read after save. Pair / account last-N live windows are not rewritten; only later `Observe*(Success=false)` ingest sees the new custom rule.

### 2.2 Safety rails (hard)

- Preset family needles stay code constants.
- Custom `message_contains` is a literal substring only. Escape before SQL LIKE. Admins cannot inject `%` `_` wildcards or raw SQL.
- Never exclude `Upstream request failed` without looking at status.
- status=502 `Upstream request failed` **always counts toward schedule**. Config / custom rules cannot whitelist it away (`AND NOT` hard rail in SQL; Go returns false from exclude).
- Save rejects unknown family keys.
- PUT that omits `custom` must not wipe custom rules (old family-only clients). The same applies to omitted `replacements` / `family_replace`. A save that echoes the expanded GET factory list persists those rows and keeps factory behavior.
- Builtin status `schedule_exclude` defaults false. Turning it on adds `status IN (...)` to the schedule SQL and still `AND NOT` the 502 `Upstream request failed` and wait-timeout hard rails.
- Wait-timeout client text stays `Upstream service temporarily unavailable`. The marker never reaches the client.
- `needs_ops_attention` does not follow replacement or whitelist toggles.

### 3. Contracts

**Schedule ingest** (`observePairQualityErrors` / `observeAccountQualityErrors`): skip `Observe*(Success=false)` when `!CountedInAccountScheduleRate`.

`CountedInAccountScheduleRate` is the previous terminal/compare/failover result **and not** `IsScheduleQualityExcludedWith` (hardcoded legacy routing miss + enabled new whitelist families + existing bridge). Recovered stays on the existing failover toggle. Compare default is unchanged (routing miss + bridge only).

**Account 15m SQL `ErrorCount`**: `AND NOT (SQLScheduleQualityExcludedPredicateWith(current whitelist))` in addition to existing bridge/status guards. Do **not** apply this exclude to user-dimension quality SQL.

**Attention**: `NeedsOpsAttention` / `SQLOpsAttentionPredicate` match (independent of whitelist):

| Family | Predicate |
| --- | --- |
| Group has no account for model | message/body contains `not supported by any configured account` OR `supporting model:` OR `no account supports` — **any** phase/status |
| Legacy routing miss | existing `IsAccountQualityRoutingModelMiss` (status in 400/403/404/503, phase ≠ upstream, type not upstream/overloaded/rate_limit, model-not-found / whitelist wording) |
| Routing pool empty | `error_phase=routing` AND status=503 |
| Protocol mismatch | message contains `not supported on the Chat Completions endpoint` OR `Unsupported content type` OR `Invalid URL` |

**Client noise** (schedule exclude when family on, attention=false): `invalid_request_error` **only at** `error_phase=request`; status=400 wrapped same-semantics including `Upstream request failed` **only when status=400**; prompt/context/413; request/429 `Concurrency limit exceeded for account`.

Hop passthrough `invalid_request_error` with `error_phase=upstream` (and no other family) **counts toward schedule**.

**Must stay on schedule** (attention=false): status=502 `Upstream request failed` / upstream unavailable / forbidden; real upstream 429; internal stream abort that matches none of the enabled families.

Alert event on fire: `dimensions.top` is up to 5 `{group_id, model, count}`; `dimensions.error_list_filter` is `needs_ops_attention=true`. Description includes the same top breakdown.

List badge re-apply (`applyErrorLogCalibers`) must pass `error_body` through so ingest and list badges share the same input. Do not change ingest semantics to chase badges.

### 4. Validation & Error Matrix

| Input | Schedule counted (default WL = empty) | Attention | Notes |
| --- | --- | --- | --- |
| 400 `invalid_request_error` `phase=request` | **yes** | no | check family to exclude |
| 400 `invalid_request_error` `phase=upstream` (no other family) | **yes** | no | narrower than whole-type exclude |
| 400 `Upstream request failed` | **yes** | no | check `client_wrapped_400_urf` to exclude |
| 502 `Upstream request failed` | **yes** | no | default `mapUpstreamError`; any config |
| 413 / prompt too long | **yes** | no | check family to exclude |
| 429 concurrency-for-account | **yes** | no | check family to exclude |
| 404 + not supported by any configured account | no | **yes** | hardcoded legacy miss |
| 502 + not supported by any configured account | **yes** | **yes** | new unrestricted family; check `group_no_account` to exclude |
| routing 503 | **yes** | **yes** | check `routing_pool_empty` to exclude |
| Chat Completions / Unsupported content type / Invalid URL | **yes** | **yes** | check `protocol_mismatch` to exclude |
| Recovered | existing | no | |
| Claude–GPT bridge | no | no | |
| unknown `ops_attention_count` on old binary | n/a | n/a | create/eval skip unknown metric |

Alert rule create: `ops_attention_count` allowed; threshold ≥ 0; not clamped to 0–100.

### 5. Good / Base / Bad Cases

- **Good**: 502 group-model gap no longer writes pair `W_ok=false`; list shows attention badge; default rule fires with top `group_id`+model.
- **Base**: real 502 `Upstream request failed` still cools the pair.
- **Bad**: matching `Upstream request failed` without status → excludes real hops. Matching all `invalid_request` as attention → pages on user typos. Matching all `invalid_request_error` regardless of phase → hop 400 passthrough no longer cools.

### 6. Tests Required

- `ClassifyOpsErrorRateCalibers` table: every row in §4.
- hop `phase=upstream` + `invalid_request_error` + 400 **counts**; `phase=request` of the same type does **not**.
- Empty / all-off whitelist still excludes 404 `model_not_found` (hardcoded miss) and does **not** exclude 400 request `invalid_request_error`.
- `group_no_account` off → 502 group-no-account counts toward schedule; on → excluded. Attention stays true.
- 502 `Upstream request failed` counts under any whitelist.
- 400 `Upstream request failed` counts at factory default; excluded only when `client_wrapped_400_urf` is on.
- List second `Apply` with body-only group-no-account text → `needs_ops_attention=true`.
- Recovered / bridge / `UseFailover` unchanged.
- Settings validate rejects unknown family ids.
- Custom message / structured AND excludes schedule; 502 URF still counts when a matching custom rule exists.
- SQL custom predicate escapes LIKE metacharacters.
- PUT omitting `custom` keeps existing custom; `custom: []` clears.
- SQL predicate strings contain the 502-unrestricted group-no-account fragment and do **not** require `error_phase <> 'upstream'` for that fragment.
- Pair/account observe: 502 group-no-account does not call `Observe*(Success=false)` when family on; 502 `Upstream request failed` does.
- Error list filter: `needs_ops_attention=true` is applied in SQL.
- Alert compute: noise-only window → 0; attention-only window → N; fire payload has `top`.
- Handler: `ops_attention_count` accepted; threshold 0 allowed.
- Factory `ResolveUpstreamClientError` matches the legacy gateway / OpenAI / Gemini sentences. Empty replacement config leaves those sentences in place.
- Repeat from-log upsert with empty `replace_with` keeps the stored sentence. A non-empty catalog id updates it.

### 7. Wrong vs Correct

#### Wrong

```go
if strings.Contains(strings.ToLower(message), "upstream request failed") {
    return true // schedule exclude
}
```

#### Correct

```go
if status == 400 && strings.Contains(strings.ToLower(message), "upstream request failed") {
    return true // client-noise exclude only
}
```

#### Wrong

```go
if typ == "invalid_request_error" {
    return true // schedule exclude, any phase
}
```

#### Correct

```go
if typ == "invalid_request_error" && strings.EqualFold(phase, "request") {
    return true // client family only
}
```

#### Wrong

```go
// Drop phase/status rails on the entire routing-miss predicate
return strings.Contains(msg, "not supported by any configured account")
```

applied *inside* `IsAccountQualityRoutingModelMiss` without keeping the 400/403/404/503 + non-upstream rails for the other wordings (`unsupported model`, `does not exist`).

#### Correct

Keep the rails on the legacy miss. Put the unrestricted group-no-account wording in `IsGroupNoAccountForModel` and union it into schedule-exclude + attention.

## Common Mistake: silent terra leak after exclude

**Symptom**: `gpt-5.6-terra` mapping gap no longer cools account 1719, and nobody notices.

**Cause**: schedule exclude without `needs_ops_attention` and without `ops_attention_count`. Global `error_rate > 5%` does not move.

**Prevention**: attention flag + seeded dedicated rule. Changelog must name the metric and the error-list filter.

## Common Mistake: treating pairing 429 as hop 429

**Symptom**: high-concurrency users keep cooling healthy accounts, or the attention pager fires on every busy pair.

**Cause**: all status=429 excluded from schedule *or* all 429 marked attention.

**Prevention**: only `Concurrency limit exceeded for account` is client/pair noise. Real upstream 429 stays on schedule and is not attention.

## Common Mistake: whole-type invalid_request exclude

**Symptom**: hop-passthrough 400 (`error_phase=upstream`, `invalid_request_error`) no longer cools a bad account.

**Cause**: whitelist family matched every `invalid_request_error` regardless of phase.

**Prevention**: `client_invalid_request` requires `error_phase=request`.
