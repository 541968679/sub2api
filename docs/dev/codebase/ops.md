# Ops Module

> Admin operations dashboard, alert rules, account availability rollups, and
> runtime account-state indicators.

## Data Model

| Data | Source | Notes |
|------|--------|-------|
| `OpsAlertRule.metric_type` | `backend/internal/service/ops_models.go` | Alert evaluator switch key. |
| `AccountAvailability.TempUnschedulableUntil` | `backend/internal/service/ops_account_availability.go` | Non-nil future timestamp means the account is temporarily unschedulable. |
| `accounts.temp_unschedulable_until` | database column | Set by rate-limit, token-refresh, stream-timeout, and OpenAI transport guards. |

## Key Files

| Layer | File | Role |
|------|------|------|
| Middleware | `backend/internal/handler/ops_error_logger.go` | Captures HTTP error bodies and upstream-attempt context for asynchronous Ops persistence. |
| Handler | `backend/internal/handler/admin/ops_alerts_handler.go` | Validates alert metric types and CRUD payloads. |
| Service | `backend/internal/service/ops_alert_evaluator_service.go` | Computes rule metric values. |
| Service | `backend/internal/service/ops_account_availability.go` | Builds account availability snapshots used by alert metrics. |
| Frontend API | `frontend/src/api/admin/ops.ts` | Metric type union and admin API calls. |
| Frontend UI | `frontend/src/views/admin/ops/components/OpsAlertRulesCard.vue` | Metric picker definitions and recommended thresholds. |

## Core Flow

```
admin creates/updates alert rule
  -> ops_alerts_handler.go validates metric_type
  -> ops repository persists rule

evaluator tick
  -> ops_alert_evaluator_service.go: computeRuleMetric()
  -> OpsService.GetAccountAvailability(platform, group_id)
  -> count accounts matching the selected metric condition
```

## Important Mechanisms

- `account_error_count` intentionally excludes accounts with a non-nil
  `TempUnschedulableUntil`, so temporary auto-evictions are not double-counted
  as permanent account errors.
- `account_temp_unscheduled_count` counts accounts whose
  `TempUnschedulableUntil` is in the future. It is meant for proxy, credential,
  rate-limit, or transport failures that automatically remove an account from
  scheduling for a bounded window.
- Frontend metric definitions and backend handler allow-list must stay in sync;
  otherwise an admin can select a metric that the API rejects, or the API can
  accept a metric that is not discoverable in the UI.
- `opsCaptureWriter` instances are pooled. Release clears the embedded Gin
  writer, so every delegated response-writer method must tolerate a nil inner
  writer in case an outer middleware or late streaming callback retains the
  wrapper past its request lifetime. While acquired, calls delegate unchanged.

## Known Pitfalls

- Expired temporary-unschedulable windows must not count. Always compare
  `TempUnschedulableUntil` against `time.Now().UTC()`.
- `SetTempUnschedulable` does not change `account.status`; availability and
  alerting code must inspect the temp-unschedulable field explicitly.
- Do not add a new method to `opsCaptureWriter` by relying only on the embedded
  `gin.ResponseWriter`; an embedded call after pool release can panic. Add an
  explicit nil-guarded delegate and extend `ops_capture_writer_nil_test.go`.
