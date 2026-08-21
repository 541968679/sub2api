# 用户质量 last-N：API 契约

配对 `quality_window_n`（默认 10）与本轨无关。本文件只描述用户维 \(Q_u\)（该用户跨全部账号）。

N 与账号 \(Q_a\) **共用** `quality_hard_close_settings.account_quality_window_n`（默认 20，1–100）。不新增用户专用 Settings 字段。

## `POST /api/v1/admin/users/quality-stats/batch`

Body 不变：`{ "user_ids": number[] }`（去重、上限 200，与账号 batch 相同）。

`stats[id]` 字段形与 `POST /admin/accounts/quality-stats/batch` 对齐，语义改为用户 last-N：

| 字段 | 语义 |
| --- | --- |
| `n` / `window_n` / `account_quality_window_n` | 全站 N，格子用 `k/N` |
| `success_count` / `error_count` | \(W_{ok}\) 成败条数（`k = success+error`） |
| `success_rate` / `error_rate` | 由 \(W_{ok}\) 算出；`k==0` 为 null |
| `ttft_samples` | \(W_{ttft}\) 条数 |
| `p50_ttft_ms` / `avg_ttft_ms` / `p95_ttft_ms` / `max_ttft_ms` | 仅来自 \(W_{ttft}\)；0 条为 null |
| `window_seconds` | 可仍为 900；展示以 N 为准 |
| `schedule_use_failover_error_rate` | 入窗时的调度开关回声 |
| `terminal_error_count` / `failover_error_count` | 与账号 last-N 相同投影；**禁止**再钉 `failover_error_count=0` |
| `bridge_*` | last-N 不维护，可为 0 / null |

未满 N：格子按 `ttft_samples < N` / `k < N` 显示样本不足。用户格不触发账号硬关闭。

列表与智能调度页头**必须**读本接口的 last-N 投影，不得再把 15 分钟 SQL `GetUserQualityStatsBatch` 当真相。

## `GET /api/v1/admin/users/:id/quality-history?from=&to=`

与账号 `GET /admin/accounts/:id/quality-history` 同形：

```json
{
  "items": [
    {
      "captured_at": "2026-08-21T07:00:00Z",
      "window_seconds": 900,
      "success_count": 10,
      "error_count": 1,
      "success_rate": 0.909,
      "avg_ttft_ms": 400,
      "p50_ttft_ms": 300,
      "p95_ttft_ms": 900,
      "max_ttft_ms": 1200,
      "ttft_samples": 10
    }
  ],
  "from": "2026-08-20T07:05:00Z",
  "to": "2026-08-21T07:05:00Z"
}
```

- 省略 `from`/`to` → `to=now`，`from=to-24h`。
- `to-from` > 7d 或 `from` > `to` → 400（`NormalizeAccountQualityHistoryRange`）。
- 非法 user id → 400。
- 维护服务不可用 → 503。
- 每个点是该 `captured_at` 时的用户 last-N \(Q_u\)，不是互斥 5 分钟桶。
- 空窗不落库；无行 → `items: []` 加规范化 `from`/`to`。

前端：用户列表 / 页头 `AccountQualityCell` `mode="combined"` 可点击，打开用户历史弹窗（只读曲线 + 含 failover 对照；**无**硬关闭表单）。

## 不要读错

| 字段 | 轨 |
| --- | --- |
| `account_quality_window_n` | 账号 \(Q_a\) **与**用户 \(Q_u\) 共用，默认 20 |
| `quality_window_n` / `quality_window_samples` | 智能调度配对 \(Q_{a,u}\)，默认 10 |
