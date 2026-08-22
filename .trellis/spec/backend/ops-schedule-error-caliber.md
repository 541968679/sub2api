# Ops schedule error caliber and attention alerts

## Scenario: pair/account schedule ingest must not treat client or routing misses as hop failures

### 1. Scope / Trigger

- Trigger: an `ops_error_logs` row is classified for (a) pair-quality / account last-N ingest, (b) account-dimension 15m SQL `ErrorCount`, (c) admin error-list badges/filters, (d) the dedicated `ops_attention_count` alert.
- Cross-layer: `ClassifyOpsErrorRateCalibers` is the Go source of truth. SQL twins must stay in lockstep. List filter and alert count use the SQL twins, not in-memory paging.
- Does **not** change client HTTP, SLA / user error rate, compare rate (except that schedule exclusion is independent), billing / `actual_cost`, Claude–GPT bridge exclusion, or `classifyNoAccountError` 404/503 routing.
- Does **not** drop the phase/status safety rails on `IsAccountQualityRoutingModelMiss` as a whole. The 502 “no account in group supports this model” leak is a **separate** family that ignores phase/status.

### 2. Signatures

Go (names may match these or obvious aliases; tests lock behavior):

```go
func IsGroupNoAccountForModel(message, body string) bool
func IsScheduleClientNoise(status int, phase, errorType, message string) bool
func IsScheduleProtocolMismatch(message string) bool
func IsOpsAttentionError(status int, phase, errorType, message, body string) bool
func IsScheduleQualityExcluded(status int, phase, errorType, message, body string) bool

func SQLGroupNoAccountForModelPredicate() string
func SQLScheduleClientNoisePredicate() string
func SQLScheduleProtocolMismatchPredicate() string
func SQLOpsAttentionPredicate() string
func SQLScheduleQualityExcludedPredicate() string
```

`OpsErrorRateCalibers` gains `NeedsOpsAttention bool`. Existing three counted-in flags stay.

List: `OpsErrorLog.NeedsOpsAttention bool` (`needs_ops_attention`).  
Filter: `OpsErrorLogFilter.NeedsOpsAttention *bool` from query `needs_ops_attention=true|false`.

Alert metric: `ops_attention_count` (window count of rows matching `SQLOpsAttentionPredicate`, optional existing `platform` / `group_id` filters).  
Not a percent metric.

Default seeded rule name: `需运维：组模型/路由/协议`. Unique on `ops_alert_rules.name`. `ON CONFLICT (name) DO NOTHING`.

### 3. Contracts

**Schedule ingest** (`observePairQualityErrors` / `observeAccountQualityErrors`): skip `Observe*(Success=false)` when `!CountedInAccountScheduleRate`.

`CountedInAccountScheduleRate` is the previous terminal/compare/failover result **and not** `IsScheduleQualityExcluded` (client noise, ops-attention families, existing routing miss, existing bridge). Recovered stays on the existing failover toggle.

**Account 15m SQL `ErrorCount`**: `AND NOT (SQLScheduleQualityExcludedPredicate())` in addition to existing bridge/status guards. Do **not** apply this exclude to user-dimension quality SQL.

**Attention**: `NeedsOpsAttention` / `SQLOpsAttentionPredicate` match:

| Family | Predicate |
| --- | --- |
| Group has no account for model | message/body contains `not supported by any configured account` OR `supporting model:` OR `no account supports` — **any** phase/status |
| Legacy routing miss | existing `IsAccountQualityRoutingModelMiss` (status in 400/403/404/503, phase ≠ upstream, type not upstream/overloaded/rate_limit, model-not-found / whitelist wording) |
| Routing pool empty | `error_phase=routing` AND status=503 |
| Protocol mismatch | message contains `not supported on the Chat Completions endpoint` OR `Unsupported content type` OR `Invalid URL` |

**Client noise** (schedule exclude, attention=false): `invalid_request_error`; status=400 wrapped same-semantics including `Upstream request failed` **only when status=400**; prompt/context/413; request/429 `Concurrency limit exceeded for account`.

**Must stay on schedule** (attention=false): status=502 `Upstream request failed` / upstream unavailable / forbidden; real upstream 429; internal stream abort that matches none of the families above.

Alert event on fire: `dimensions.top` is up to 5 `{group_id, model, count}`; `dimensions.error_list_filter` is `needs_ops_attention=true`. Description includes the same top breakdown.

### 4. Validation & Error Matrix

| Input | Schedule counted | Attention | Notes |
| --- | --- | --- | --- |
| 400 `invalid_request_error` | no | no | |
| 400 `Upstream request failed` | no | no | |
| 502 `Upstream request failed` | **yes** | no | default `mapUpstreamError` |
| 413 / prompt too long | no | no | |
| 429 concurrency-for-account | no | no | |
| 404 or 502 + not supported by any configured account | no | **yes** | |
| routing 503 | no | **yes** | |
| Chat Completions / Unsupported content type / Invalid URL | no | **yes** | |
| Recovered | existing | no | |
| Claude–GPT bridge | no | no | |
| unknown `ops_attention_count` on old binary | n/a | n/a | create/eval skip unknown metric |

Alert rule create: `ops_attention_count` allowed; threshold ≥ 0; not clamped to 0–100.

### 5. Good / Base / Bad Cases

- **Good**: 502 group-model gap no longer writes pair `W_ok=false`; list shows attention badge; default rule fires with top `group_id`+model.
- **Base**: real 502 `Upstream request failed` still cools the pair.
- **Bad**: matching `Upstream request failed` without status → excludes real hops. Matching all `invalid_request` as attention → pages on user typos.

### 6. Tests Required

- `ClassifyOpsErrorRateCalibers` table: every row in §4.
- SQL predicate strings contain the 502-unrestricted group-no-account fragment and do **not** require `error_phase <> 'upstream'` for that fragment.
- Pair/account observe: 502 group-no-account does not call `Observe*(Success=false)`; 502 `Upstream request failed` does.
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
