# Admin error surfaces and capture points

## Surfaces in scope

- List column「响应内容」: `frontend/src/views/admin/ops/components/OpsErrorLogTable.vue`
  - Shared by Ops, admin Usage errors, account/user inspect dialogs.
  - Today: `formatSmartMessage(log.message)` only. List DTO has no upstream fields.
- Detail: `frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue`
  - One `primaryResponseBody` via `resolvePrimaryResponseBody` (picks a single payload).
  - Upstream cards: correlated rows, preview via `resolveUpstreamPayload` then `error_body`.
- Out of scope user console: `UserErrorRequestsTable.vue` + `UserErrorDetailModal.vue`

## Why admin currently sees generic text

1. `ops_error_logger` stores `ErrorMessage` from the client-facing write.
2. Failover-exhausted and stream helpers often call `setOpsUpstreamError` with the mapped sentence and a self-built wrapper JSON.
3. `setOpsUpstreamError` last-write-wins; empty detail wipes nothing useful because detail was never set, then later generic message overwrites a good extract.
4. `LogUpstreamErrorBody` gates many Detail assignments; it is a stdout flag, not an admin-visibility flag.

## Client mapping that must stay

- `mapUpstreamError` in `openai_gateway_handler.go` (401/403/429/529/5xx/default).
- Claude/Gemini switch in `gateway_service.go` (400 raw passthrough is existing).
- OpenAI non-failover `handleErrorResponse` may still forward extracted `error.message` on 400/default.

## Extractors already available

- `extractUpstreamErrorMessage` / `extractUpstreamErrorCode` in `gateway_service.go` understand `error.message` and `error.code` (new-api included).
