# Channel Monitor

> Admin-managed upstream health checks for OpenAI, Anthropic, and Gemini
> compatible channels. Monitors store encrypted credentials, optional request
> templates, latest history, and daily availability rollups.

## Data Model

| Entity / field | Location | Notes |
| --- | --- | --- |
| `channel_monitors` | `backend/ent/schema/channel_monitor.go` | Monitor definition, encrypted API key, provider, model list, request snapshot, and OpenAI `api_mode`. |
| `channel_monitor_request_templates` | `backend/ent/schema/channel_monitor_request_template.go` | Reusable request headers/body snapshots, scoped by provider and `api_mode`. |
| `channel_monitor_histories` | `backend/ent/schema/channel_monitor_history.go` | Per-check status, latency, ping latency, message, and timestamp. |
| `channel_monitor_daily_rollups` | `backend/migrations/` | Aggregated availability data for longer windows. |
| `api_mode` migration | `backend/migrations/158_channel_monitor_openai_api_mode.sql` | Adds `chat_completions` / `responses` constraints and defaults. |

## Key Files

| Layer | File | Responsibility |
| --- | --- | --- |
| Handler | `backend/internal/handler/admin/channel_monitor_handler.go` | Admin CRUD, run-now, history DTOs. |
| Handler | `backend/internal/handler/admin/channel_monitor_template_handler.go` | Request template CRUD, associated monitors, apply-to-monitor API. |
| Service | `backend/internal/service/channel_monitor_service.go` | Monitor validation, encryption, scheduling hooks, check orchestration. |
| Service | `backend/internal/service/channel_monitor_checker.go` | Provider adapters, OpenAI Chat/Responses request construction, response parsing. |
| Service | `backend/internal/service/channel_monitor_validate.go` | Provider, endpoint, interval, and `api_mode` defaults/validation. |
| Repository | `backend/internal/repository/channel_monitor_repo.go` | Monitor persistence, history, availability, rollups. |
| Repository | `backend/internal/repository/channel_monitor_template_repo.go` | Template persistence and snapshot application. |
| Frontend | `frontend/src/components/admin/monitor/MonitorFormDialog.vue` | Monitor create/edit form, OpenAI protocol selector. |
| Frontend | `frontend/src/components/admin/monitor/MonitorTemplateManagerDialog.vue` | Template management and template protocol selector. |
| Frontend | `frontend/src/components/admin/monitor/MonitorAdvancedRequestConfig.vue` | Advanced request headers/body editor with protocol-aware protected fields. |

## Core Flow

Admin monitor save:

```text
MonitorFormDialog.vue
  -> adminAPI.channelMonitor.create/update
    -> ChannelMonitorHandler
      -> ChannelMonitorService.Create/Update
        -> validate provider + api_mode + request body override
        -> encrypt API key when needed
        -> channelMonitorRepository Create/Update
        -> scheduler.Schedule when available
```

Scheduled or manual check:

```text
ChannelMonitorService.RunNow / scheduler
  -> runChecksConcurrent
    -> CheckOptions includes api_mode + request snapshot
    -> ChannelMonitorChecker.Check
      -> providerAdapterFor(provider, api_mode)
      -> /v1/chat/completions or /v1/responses for OpenAI
      -> parse response text and write history rows
```

Template application:

```text
MonitorTemplateManagerDialog.vue
  -> list/select associated monitors
  -> POST /admin/channel-monitor-templates/:id/apply
    -> repository filters template_id + provider + api_mode
    -> copies api_mode, headers, body mode, and body snapshot
```

## Important Mechanisms

- `api_mode` is only selectable for OpenAI monitors/templates.
- Empty or historical `api_mode` values are normalized to `chat_completions`.
- Anthropic and Gemini monitors are forced to `chat_completions` for storage and display compatibility.
- OpenAI `responses` checks call `/v1/responses` with `instructions`, `input`, and `max_output_tokens`.
- Responses output parsing prefers `output_text`, then falls back to nested `output[].content[].text`.
- Request templates are protocol-scoped. Applying a template filters by both provider and `api_mode` to avoid copying a Chat body template onto a Responses monitor.
- `replace` body mode validates protected body fields against the selected protocol before saving.

## Known Pitfalls

- Adding monitor fields requires updating handler DTOs, service structs, repository Ent mapping, frontend API types, and both i18n locales.
- Ent schema changes require a raw SQL migration; Ent auto-migrate is not used.
- `go generate ./cmd/server` may fail on this repo because of known duplicate payment provider bindings; reconcile Wire manually when DI changes are needed.
- `api_mode` must not affect billing, account scheduling, authentication, or payment paths.
