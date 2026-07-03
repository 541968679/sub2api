# Image Channel Monitor

## Data Model

- `image_channel_monitors`: independent image monitor configuration.
  - `source_type=custom`: stores public HTTPS endpoint plus encrypted API key.
  - `source_type=account`: stores `account_id` only; account base URL/API key/proxy/TLS profile are resolved at run time.
  - Request fields: `model`, `prompt`, `size`, `quality`, `n`, `download_image`, `interval_seconds`, `timeout_seconds`.
- `image_channel_monitor_histories`: one row per run.
  - API timing: `api_header_ms`, `api_body_ms`, `api_total_ms`, `http_status`, `json_bytes`.
  - Image result flags: `has_url`, `has_b64_json`, `image_url_host`.
  - Optional returned-image download timing: `image_first_byte_ms`, `image_download_ms`, `image_bytes`, `image_content_type`, `image_width`, `image_height`.

## Key Files

- Backend schema/migration:
  - `backend/ent/schema/image_channel_monitor.go`
  - `backend/ent/schema/image_channel_monitor_history.go`
  - `backend/migrations/174_image_channel_monitors.sql`
- Backend service/repository/handler:
  - `backend/internal/service/image_channel_monitor_service.go`
  - `backend/internal/service/image_channel_monitor_runner.go`
  - `backend/internal/repository/image_channel_monitor_repo.go`
  - `backend/internal/handler/admin/image_channel_monitor_handler.go`
- Frontend:
  - `frontend/src/api/admin/imageChannelMonitor.ts`
  - `frontend/src/views/admin/ImageChannelMonitorView.vue`
  - route: `/admin/channels/image-monitor`

## Core Flow

1. Admin creates an image monitor from `渠道管理 -> 图片渠道监控`.
2. For custom source, the service validates the endpoint as a public HTTPS origin and encrypts the API key.
3. For account source, the service validates that the selected account is an OpenAI API-key account. It does not copy the account credential into the monitor row.
4. Manual run and scheduled run both call `POST {base}/v1/images/generations` with `response_format=url`.
5. The service records API response-header latency, response-body read latency, total API latency, JSON size, URL/b64 result shape, and optional image download metrics.
6. History is stored independently from the generic channel monitor history/rollup tables.

## Important Mechanisms

- Account source resolves current account settings at run time:
  - `Account.GetOpenAIBaseURL()`
  - `Account.GetOpenAIApiKey()`
  - account proxy URL
  - `TLSFingerprintProfileService.ResolveTLSProfile(account)`
- Image monitor uses the same `HTTPUpstream.DoWithTLS` path as account testing and OpenAI image gateway requests.
- The monitor forces `response_format=url`; `b64_json` responses are recorded with `has_b64_json=true` and status `failed`, because this monitor is specifically checking returned-image URL delivery.
- `download_image=false` still verifies image API generation and URL return. `download_image=true` adds a second-stage GET probe for the returned image URL.
- The runner is independent from `ChannelMonitorRunner`, so chat/responses monitor upstream syncs should not affect image monitor scheduling.

## Known Pitfalls

- `source_type=account` only supports OpenAI API-key accounts in the first version. OAuth/image bridge accounts are intentionally out of scope.
- Custom endpoint validation requires a public HTTPS origin with no path/query/fragment, matching the generic channel monitor SSRF boundary.
- Image download uses the shared SSRF-safe monitor HTTP client; private/local returned image URLs are rejected by design.
- `go generate ./cmd/server` may fail in this checkout because of Wire tool dependency or existing provider conflicts. If so, manually reconcile `backend/cmd/server/wire_gen.go`.
