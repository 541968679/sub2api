# Channel Load Test

> Admin concurrency soak for OpenAI-compatible upstreams (kimi / glm / custom
> base URL). Replays user-363 traffic shape, scores customer TTFT/TPOT SLA, and
> flags empty `model` fields. Runs inside the backend with Go `net/http` so the
> User-Agent stays `Go-http-client/2.0`.

## Data Model

In-memory runs only. API keys from the start form are never persisted. Last 8
runs are kept in process memory; one run may be active at a time.

## Key Files

| Layer | File | Responsibility |
| --- | --- | --- |
| Engine | `backend/internal/pkg/loadtest` | Payload profiles, SSE parse, contract inspect, TTFT/TPOT |
| Service | `backend/internal/service/channel_loadtest_service.go` | Account credential resolve, job lifecycle |
| Handler | `backend/internal/handler/admin/channel_loadtest_handler.go` | Admin HTTP |
| Routes | `backend/internal/server/routes/admin.go` | `/api/v1/admin/channel-loadtest/runs` |
| Frontend | `frontend/src/views/admin/ChannelLoadtestView.vue` | 渠道管理 → 并发压测 |
| CLI | `tools/kimi-loadtest` | Same engine copied for command-line use |

## Core Flow

```
ChannelLoadtestView
  -> POST /admin/channel-loadtest/runs  (account_id XOR base_url+api_key)
  -> ChannelLoadtestService.Start
      -> account.GetOpenAIBaseURL + api_key  OR  urlvalidator
      -> optional proxy_id (0/omit = direct; account default if omitted)
      -> loadtest.Run via proxyurl/proxyutil (HTTP/SOCKS5)
  -> GET /admin/channel-loadtest/runs/:id  (poll 1s)
  -> POST .../stop
```

## Important Mechanisms

- Profiles are **tier templates**. Choosing a profile in Admin replaces the traffic-tier table (`presetForProfile` / `loadtest.PresetTiers`). `smoke` → `40×80`; `general` → `40×4K / 30×16K / 20×32K / 10×64K`; `user363-sla` → `50/38/10/2 × 50K/80K/160K/380K`; stream/sync/mixed normalize bucket weights to 100 absolute rows (largest-remainder).
- Admin UI always runs from the tier table (`{input_tokens, count}`). Request count is Σ count (1–500, max 64 rows). Form no longer exposes `total` / `size_cap` / fixed `input_tokens`; start sends `total=Σcount`, `size_cap=0`, `input_tokens=0`. Output cap is run-level `max_tokens`. Rows use weighted round-robin.
- The admin SLA button fills the same four SLA rows and also sets concurrency 50. “Reset tiers” restores the current profile mix.
- Excel export: `GET /api/v1/admin/channel-loadtest/runs/:id/export` (finished runs only). Workbook sheets: 测试条件, 总览, 首字延迟, 总耗时, 成功率. Metric sheets are `构造输入大小(K) × n/p50/p75/p90` (success-rate sheet uses n/ok/rate). No SLA PASS/FAIL. Built by `loadtest.BuildExcelReport` (excelize).
- `api_mode`: `chat_completions` (`/v1/chat/completions`) or `responses` (`/v1/responses`).
- `stream_mode`: `auto` (profile mix), `stream`, or `sync`. Tier rows start as stream, then follow `stream_mode`.
- Estimated input > 2M tokens requires `confirm_cost`. With tiers, the estimate is `Σ input_tokens × count`.
- Concurrency cap 80, total cap 500. With tiers, snapshot `total` is the sum of counts.
- Optional `proxy_id` (0 = direct).
- Contract issues: `model_missing` / `model_empty` / `model_mismatch`.

## Known Pitfalls

- Live widgets are in-flight, peak, done, success, and RPM. RPM counts only `outcome=success` in the trailing 60 seconds, plus the peak 60-second window and the run average. Failures are excluded.
- 380K SLA p99 bodies can 413 on some vendors.
- kimi-k3 (any `kimi-*` model) is always posted to `/v1/chat/completions` with a `messages` body and `max_completion_tokens`. A run-level Responses mode does not apply to those models. The body does not include a synthetic assistant turn.
- TTFT is time to the first output token, including `reasoning_content`. Answer text is `first_content_ms` and can be much later on thinking models. Input/output t/s are token counts divided by the whole request duration.
- Do not log or return the pasted API key.
