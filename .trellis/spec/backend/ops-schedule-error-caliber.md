# Ops schedule error caliber and attention alerts

## Scenario: pair/account schedule ingest must not treat client or routing misses as hop failures

### 1. Scope / Trigger

- Trigger: an `ops_error_logs` row is classified for (a) pair-quality / account last-N ingest, (b) account-dimension 15m SQL `ErrorCount`, (c) admin error-list badges/filters, (d) the dedicated `ops_attention_count` alert.
- Cross-layer: `ClassifyOpsErrorRateCalibers` is the Go source of truth. SQL twins must stay in lockstep. List filter and alert count use the SQL twins, not in-memory paging.
- Does **not** change client HTTP, SLA / user error rate, compare rate (except that schedule exclusion is independent), billing / `actual_cost`, Claude–GPT bridge exclusion, or `classifyNoAccountError` 404/503 routing.
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

### 2.1 Schedule error whitelist (preset families only)

Settings KV key: `schedule_error_whitelist` (`SettingKeyScheduleErrorWhitelist`).

JSON shape:

```json
{
  "families": {
    "client_invalid_request": true,
    "client_wrapped_400_urf": true,
    "client_context_too_long": true,
    "pair_concurrency": true,
    "group_no_account": true,
    "routing_model_miss": true,
    "routing_pool_empty": true,
    "protocol_mismatch": true
  }
}
```

`true` = in whitelist = **exclude** from pair cooldown / account last-N / account 15m schedule `ErrorCount` (`CountedInAccountScheduleRate`).  
Uncheck a family = that family counts toward schedule again.  
Missing key / empty / invalid JSON = factory default (all families `true`).

No free-text needles. No custom LIKE. No new admin page — checkbox group on the existing settings page. Save accepts only known family ids + bool.

| id | Default on | Match |
| --- | --- | --- |
| `client_invalid_request` | yes | `error_type=invalid_request_error` **and** `error_phase=request` |
| `client_wrapped_400_urf` | yes | status=400 and message contains `upstream request failed` (must look at status) |
| `client_context_too_long` | yes | 413 / prompt too long / context window / array too long |
| `pair_concurrency` | yes | 429 + `Concurrency limit exceeded for account` |
| `group_no_account` | yes | group-has-no-account wording, any phase/status |
| `routing_model_miss` | yes | existing `IsAccountQualityRoutingModelMiss` safety rails unchanged |
| `routing_pool_empty` | yes | `error_phase=routing` and status=503 |
| `protocol_mismatch` | yes | Chat Completions endpoint / Unsupported content type / Invalid URL |

Hot read: short cache or settings invalidation. `ClassifyOpsErrorRateCalibers` and `SQLScheduleQualityExcludedPredicateWith` must use the same snapshot. Account 15m `ErrorCount` uses the generated exclude; user-dimension quality SQL must not.

### 2.2 Safety rails (hard)

- SQL needles are code constants only. Admins cannot inject custom LIKE.
- Never exclude `Upstream request failed` without looking at status.
- status=502 `Upstream request failed` **always counts toward schedule**. Config cannot whitelist it away (`AND NOT` hard rail in SQL; Go returns false from exclude).
- Save rejects unknown family keys.

### 3. Contracts

**Schedule ingest** (`observePairQualityErrors` / `observeAccountQualityErrors`): skip `Observe*(Success=false)` when `!CountedInAccountScheduleRate`.

`CountedInAccountScheduleRate` is the previous terminal/compare/failover result **and not** `IsScheduleQualityExcludedWith` (enabled whitelist families + existing bridge). Recovered stays on the existing failover toggle. Compare default is unchanged (routing miss + bridge only).

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

| Input | Schedule counted (default WL) | Attention | Notes |
| --- | --- | --- | --- |
| 400 `invalid_request_error` `phase=request` | no | no | |
| 400 `invalid_request_error` `phase=upstream` (no other family) | **yes** | no | narrower than whole-type exclude |
| 400 `Upstream request failed` | no | no | |
| 502 `Upstream request failed` | **yes** | no | default `mapUpstreamError`; any config |
| 413 / prompt too long | no | no | |
| 429 concurrency-for-account | no | no | |
| 404 or 502 + not supported by any configured account | no | **yes** | off `group_no_account` → schedule **yes**, attention still yes |
| routing 503 | no | **yes** | |
| Chat Completions / Unsupported content type / Invalid URL | no | **yes** | |
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
- `group_no_account` off → 502 group-no-account counts toward schedule; on → excluded. Attention stays true.
- 502 `Upstream request failed` counts under any whitelist.
- 400 `Upstream request failed` still excluded at factory default.
- List second `Apply` with body-only group-no-account text → `needs_ops_attention=true`.
- Recovered / bridge / `UseFailover` unchanged.
- Settings validate rejects unknown family ids.
- SQL predicate strings contain the 502-unrestricted group-no-account fragment and do **not** require `error_phase <> 'upstream'` for that fragment.
- Pair/account observe: 502 group-no-account does not call `Observe*(Success=false)` when family on; 502 `Upstream request failed` does.
- Error list filter: `needs_ops_attention=true` is applied in SQL.
- Alert compute: noise-only window → 0; attention-only window → N; fire payload has `top`.
- Handler: `ops_attention_count` accepted; threshold 0 allowed.

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
