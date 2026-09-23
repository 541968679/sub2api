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

- Profiles: `smoke`, `user363`, `user363-stream`, `user363-sync`, `user363-sla`.
- Optional `tiers`: each row is `{input_tokens, count}`. A non-empty table is the whole run (absolute counts, sum 1–500, at most 64 rows). It does not rescale to `total`, does not apply `size_cap` or the single `input_tokens` pin, and uses the run-level `max_tokens` for every row. Rows are spread with weighted round-robin. An empty table keeps preset sampling, including the SLA sheet's paired outputs 200/600/1300/7000.
- The admin SLA button fills `50×50000`, `38×80000`, `10×160000`, `2×380000` and still sets the old preset knobs. Clearing the table returns to preset sampling. `user363` weight buckets are not expanded into rows.
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
