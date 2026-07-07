# Image Channel Monitor

## Data Model

- `image_channel_monitors`: independent image monitor configuration.
  - `source_type=custom`: stores public HTTPS endpoint plus encrypted API key.
  - `source_type=account`: stores `account_id` only; account base URL/API key/proxy/TLS profile are resolved at run time.
  - Custom-source proxy binding uses optional `proxy_id` plus denormalized `proxy_name`; proxy credentials stay in the proxy table.
  - Request fields: `model`, `prompt`, `size`, `quality`, `n`, `download_image`, `interval_seconds`, `timeout_seconds`.
  - User-facing display config (migration 178): `public_visible` (default false) and `public_name` (empty string falls back to the monitor name). Only `public_visible=true` monitors appear on the user channel status page.
- `image_channel_monitor_histories`: one row per run.
  - API timing: `api_header_ms`, `api_body_ms`, `api_total_ms`, `http_status`, `json_bytes`.
  - Image result flags: `has_url`, `has_b64_json`, `image_url_host`.
  - Optional returned-image download timing: `image_first_byte_ms`, `image_download_ms`, `image_bytes`, `image_content_type`, `image_width`, `image_height`.

## Key Files

- Backend schema/migration:
  - `backend/ent/schema/image_channel_monitor.go`
  - `backend/ent/schema/image_channel_monitor_history.go`
  - `backend/migrations/174_image_channel_monitors.sql`
  - `backend/migrations/175_image_channel_monitor_proxy.sql`
- Backend service/repository/handler:
  - `backend/internal/service/image_channel_monitor_service.go`
  - `backend/internal/service/image_channel_monitor_runner.go`
  - `backend/internal/repository/image_channel_monitor_repo.go`
  - `backend/internal/handler/admin/image_channel_monitor_handler.go`
  - manual ad-hoc routes:
    - `POST /api/v1/admin/image-channel-monitors/:id/manual-test`
    - `GET /api/v1/admin/image-channel-monitors/:id/manual-test/:runID`
    - `POST /api/v1/admin/image-channel-monitors/:id/manual-test/:runID/cancel`
- Frontend:
  - `frontend/src/api/admin/imageChannelMonitor.ts`
  - `frontend/src/views/admin/ImageChannelMonitorView.vue`
  - route: `/admin/channels/image-monitor`

## Core Flow

1. Admin creates an image monitor from `渠道管理 -> 图片渠道监控`.
2. For custom source, the service validates the endpoint as a public HTTPS origin and encrypts the API key.
3. For account source, the service validates that the selected account is an OpenAI API-key account. It does not copy the account credential into the monitor row.
4. Row-level immediate run starts asynchronously and immediately returns runtime status; the frontend polls `GET /admin/image-channel-monitors/:id/status` for stage updates.
5. Scheduled runs and row-level immediate runs call `POST {base}/v1/images/generations` with `response_format=url`.
6. The service records API response-header latency, response-body read latency, total API latency, JSON size, URL/b64 result shape, and optional image download metrics.
7. History is stored independently from the generic channel monitor history/rollup tables.
8. The manual testing panel calls `POST /admin/image-channel-monitors/:id/manual-test` concurrently for selected image monitors. The POST only starts an in-memory manual run and immediately returns `run_id`, `running`, `stage`, timestamps, and optional concurrent-batch metadata (`batch_id`, `batch_size`, `batch_index`).
9. The frontend polls `GET /admin/image-channel-monitors/:id/manual-test/:runID` until `running=false`. It can cancel a running manual run via `POST /admin/image-channel-monitors/:id/manual-test/:runID/cancel`.
10. Completed or canceled manual runs are added to a browser-local manual history list. These ad-hoc tests reuse monitor source credentials/proxy/TLS resolution, support text-to-image (`/v1/images/generations`) and image-to-image (`/v1/images/edits`) probes, but do not update scheduled runtime status or persist backend history.
11. Manual concurrent testing is frontend-orchestrated: `concurrency=N` expands every selected monitor into `N` independent manual runs, all sharing one `batch_id`. The record table and detail dialog show batch index/size and average elapsed time for the batch.
12. The manual testing UI renders as a fixed-viewport split console: the left column stacks Parameters (collapsible: mode/model/merged-size/quality/n/concurrency/timeout/download, prompt, input image, preset dropdown + save/delete) → Channels (internal-scroll list with selected-count pill, select-all/clear, and a search filter) → a persistent Start-test CTA bar that swaps to progress + cancel while running; the right column is a unified record table (running/completion banner + toolbar + internal-scroll table with a sticky header) that merges in-flight manual runs with browser-local history. The panel fits one viewport — scrolling occurs only inside the channel list and the table, never the page.

## Important Mechanisms

- Account source resolves current account settings at run time:
  - `Account.GetOpenAIBaseURL()`
  - `Account.GetOpenAIApiKey()`
  - account proxy URL
  - `TLSFingerprintProfileService.ResolveTLSProfile(account)`
- Custom source resolves `proxy_id` through `ProxyRepository.GetByID` at run time. The selected proxy is used for both the image API request and the returned-image download probe.
- Image monitor uses the same `HTTPUpstream.DoWithTLS` path as account testing and OpenAI image gateway requests.
- Runtime status is in-memory and non-persistent. It exposes `running`, `stage`, `message`, timestamps, and next-check countdown data for UI polling; durable results remain in `image_channel_monitor_histories`.
- Manual test run status is also in-memory and non-persistent. It is intentionally separate from scheduled runtime status so ad-hoc tests can run without changing row-level countdowns or persisted history.
- Manual test cancellation uses a per-run `context.CancelFunc` stored alongside the in-memory run status. Canceled runs return `canceled=true`, `stage=canceled`, and are not overwritten by a later background completion.
- Manual test cancellation is still added to browser-local manual history. Canceled entries have no generated-image result, but keep the selected monitor, prompt, parameters, elapsed time, and final `canceled` state.
- Manual concurrent-test batches are intentionally not stored in `image_channel_monitor_histories`; that table remains scheduled/row-level durable history. The backend manual run DTO carries `batch_id`, `batch_size`, and `batch_index` so polling/cancel responses keep the batch marker, while the frontend local history stores `batch_average_elapsed_ms` after aggregating same-batch records.
- Response format is a per-monitor / per-manual-test choice (migration 179): `url`, `b64_json`, or `''` (omit the parameter, follow the upstream default). Only `url` mode treats a `b64_json` response as a delivery failure; the other two accept both shapes. Existing rows were backfilled to `url`, matching the previously forced behavior. Each check's format is recorded in `image_channel_monitor_histories.response_format` and shown in the scheduled history dialog and manual record detail.
- Manual tests always accept `b64_json` as a previewable image result regardless of the selected format; only scheduled checks in `url` mode mark it as failed.
- `imageMonitorMaxResponseBytes` is 24 MiB because b64_json inlines the whole image into the API response body.
- Inline `b64_json` payloads never pass through the URL download probe, so `fillImageMonitorInlineImageInfo` decodes them to fill `image_bytes`, sniffed `image_content_type`, and `image_width`/`image_height` (both scheduled and manual paths). The manual records table (`image_info` column) and detail dialog, plus the scheduled history dialog, display these returned-image values; the manual UI shows an amber mismatch warning when the request pinned a concrete `WIDTHxHEIGHT` size that differs from the actual dimensions (`omit`/`auto` requests are never flagged).
- History retention: raw `image_channel_monitor_histories` rows are physically deleted after 30 days (`imageMonitorHistoryRetentionDays`) by `ImageChannelMonitorService.RunDailyMaintenance`, invoked from `OpsCleanupService.runCleanupOnce` (same daily cron + Redis leader lock as the native channel monitor maintenance). No rollup tables — all charts aggregate raw rows on the fly.
- Status timeline aggregation (`image_channel_monitor_repo.go`): `AggregateTimeline` buckets by epoch-floor (`24h`→600s, `7d`→7200s, `30d`→86400s buckets, mapping fixed in `imageMonitorTimelineWindows`); `ComputeWindowStats` returns availability (`(operational+degraded)/total*100`) plus avg/max API and avg download latency; `ListRecentHistoryForMonitors` batch-fetches the last 60 checks per monitor (ROW_NUMBER window, no N+1) for the admin row strips and user cards; `ComputeAvailabilityForMonitors` computes 7/15/30d availability for many monitors in one FILTER-aggregate query.
- Admin surfaces: `GET /admin/image-channel-monitors` embeds `timeline` (last 60 points) + `availability_7d` per item; `GET /admin/image-channel-monitors/:id/timeline?window=24h|7d|30d` powers `ImageMonitorStatusDialog.vue` (chart.js mixed chart: two latency lines + red failure bars, spanGaps=false so empty buckets break the line). The row strip reuses the user-side `MonitorTimeline.vue` component.
- User surfaces: `GET /api/v1/image-channel-monitors` and `GET /api/v1/image-channel-monitors/:id/status` (`ImageChannelMonitorUserHandler`), gated by the same `channel_monitor_enabled` setting as the native monitor page plus per-monitor `public_visible`. The user DTO is a strict whitelist — never expose internal name (when `public_name` set), endpoint, hosts/IPs, exit IP, error messages, `error_stage`, image URLs, proxy/account info. `TestImageMonitorPublicListItemFieldWhitelist` snapshots the JSON keys; extend it deliberately when adding fields. `/monitor` renders the section via `ImageMonitorCard.vue` (composes MonitorMetricPair/MonitorAvailabilityRow/MonitorTimeline; `latest_status="empty"` maps to the neutral unknown badge) and `ImageMonitorDetailDialog.vue` (7/15/30d availability + avg latency). All three availability windows ship in the list response, so window switching is purely client-side (no per-card detail fetches like the native monitor).
- `download_image=false` still verifies image API generation and URL return. `download_image=true` adds a second-stage GET probe for the returned image URL. When the downloaded response is an image and is at most 16 MiB, manual runs include a `returned_image_data` data URL so browser-local history can preserve the generated image instead of depending only on a temporary upstream URL.
- Manual test results include best-effort network metadata for display and browser-local history: exit IP from `https://api.ipify.org?format=text` through the same source proxy path, API request URL/host/DNS IPs, and returned-image download URL/host/DNS IPs. DNS IPs are local resolver results, so SOCKS remote-DNS behavior or CDN routing can still differ from the actual upstream edge used. IP geolocation is intentionally not implemented yet.
- The admin UI supports four size modes: omit the `size` request field, send `auto`, send OpenAI standard presets (`1024x1024`, `1536x1024`, `1024x1536`), or pass through a custom `WIDTHxHEIGHT` value for upstreams/models that support custom dimensions.
- The manual testing panel stores parameter presets in browser localStorage (`sub2api:image-channel-monitor:manual-presets:v1`). Presets include mode, model, prompt, size mode/value, quality, `n`, concurrency, download toggle, timeout, and an optional image-to-image input image reference. Uploaded image bytes are stored separately in IndexedDB (`sub2api-image-channel-monitor` / `manual-images`) and restored when the preset is selected.
- Manual testing history is also browser-local (`sub2api:image-channel-monitor:manual-history:v1`). It stores the latest 50 completed/canceled manual runs with timing, final status, request settings, batch metadata, batch average elapsed time, prompt, input image reference, generated image reference, and fallback generated-image URL. Image bytes are stored in IndexedDB rather than localStorage.
- The manual record table supports search, status/mode/channel filters, time-order sorting, and field visibility toggles. Each row opens a detail dialog for the full prompt, parameters, timing metrics, network metadata, input image, generated image, and image download link.
- The image monitor page uses `TablePageLayout` in fixed mode for both panels. The regular DataTable uses the default styled slot; the manual testing panel passes `bareTable` so the shared `#table` card chrome and DataTable-oriented `:deep(table/th/td)` styling are stripped (guarded by `:not(.is-bare)`), letting the panel render its own fixed-height split console that reuses only the layout's viewport-height management.
- Fitting the manual panel in one viewport is a flex-height contract: the console fills the bare `#table` slot with `h-full min-h-0`; the left column and right column are `flex flex-col min-h-0`; the channel list and the results-table wrapper are the `flex-1 min-h-0 overflow-y-auto` regions; Parameters and the CTA bar are `flex-none`. The results table header stays visible via `position: sticky; top: 0` on its `th` (defined in the SFC `<style scoped>` as `.mtbl-th`, using an inset box-shadow bottom border so it survives sticky). Below `lg` the console falls back to stacked columns and `TablePageLayout` mobile-mode restores page scroll.
- The merged size dropdown is a single `manualSizeChoice` get/set computed over the unchanged `size_mode`/`size`/`custom_size` fields (`omit`/`auto`/preset-dimension/`custom`), so payload building (`resolvedManualSizeFromSettings`) is untouched.
- The running/completion banner and CTA progress are derived from `manualBatchStats` (running/done/ok/failed/canceled counts over the current `manualResults` batch), `manualBatchProgress`, and same-batch average elapsed time; `manualBatchDismissed` (reset on each run start) controls the post-run banner.
- The detail dialog's latency waterfall stacks `api_header_ms` (start→headers) + `api_body_ms` (headers→body phase) + `image_download_ms` (download phase). These are sequential non-overlapping durations in `image_channel_monitor_service.go` (header measured from request start, body from `bodyStart` after headers, download from its own start), so summing/stacking them is correct; do not add `api_total_ms` as a fourth segment (it already ≈ header+body).
- Each row shows a runtime status bar with current stage plus next-check countdown.
- The runner is independent from `ChannelMonitorRunner`, so chat/responses monitor upstream syncs should not affect image monitor scheduling.

## Known Pitfalls

- `source_type=account` only supports OpenAI API-key accounts in the first version. OAuth/image bridge accounts are intentionally out of scope.
- Custom endpoint validation requires a public HTTPS origin with no path/query/fragment, matching the generic channel monitor SSRF boundary.
- Image download now uses the shared `HTTPUpstream` so configured proxies also apply to returned-image download. Private/local returned image URLs still depend on the global URL allowlist behavior in the upstream HTTP layer.
- Manual tests must stay asynchronous from the browser's perspective. Keeping the POST synchronous can hit the frontend Axios 30s timeout and surface as the generic `Network error. Please check your connection.` while the backend is still generating/downloading.
- The manual panel runs inside `TablePageLayout` fixed mode via the `bareTable` prop, which is the mechanism that keeps the page from scrolling. It only works because the panel supplies its own internal scroll regions (channel list + results-table wrapper) and a proper flex-height chain (`h-full`/`min-h-0` throughout). If you add new content to the left column, keep it `flex-none` or give it its own bounded scroll — otherwise it can squeeze the `flex-1` channel list. Do NOT revert to `scrollMode=page` (that reintroduces whole-page scrolling, which this redesign specifically removed); the earlier "fixed mode clips non-table content" problem is resolved by `bareTable` stripping the `overflow-hidden` card chrome.
- `go generate ./cmd/server` may fail in this checkout because of Wire tool dependency or existing provider conflicts. If so, manually reconcile `backend/cmd/server/wire_gen.go`.
