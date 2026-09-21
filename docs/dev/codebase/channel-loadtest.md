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
- `api_mode`: `chat_completions` (`/v1/chat/completions`) or `responses` (`/v1/responses`).
- `stream_mode`: `auto` (profile mix), `stream`, or `sync`.
- Estimated input > 2M tokens requires `confirm_cost`.
- Concurrency cap 80, total cap 500.
- Optional `proxy_id` (0 = direct).
- Contract issues: `model_missing` / `model_empty` / `model_mismatch`.

## Known Pitfalls

- Results table fills when the run finishes; live widgets are inflight/peak/done.
- 380K SLA p99 bodies can 413 on some vendors.
- Do not log or return the pasted API key.
