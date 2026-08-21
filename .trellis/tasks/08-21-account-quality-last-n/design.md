# 设计：账号质量 last-N（格 + 硬关闭）

已决合同见 `prd.md`、`research/account-quality-api-contract.md`。

## 边界

| 轨 | 窗口 | 谁用 |
| --- | --- | --- |
| \(Q_a\) 账号 | 全站一个 N、两份 FIFO，每条完成重算 | 账号格、硬关闭、`account-quality:live` |
| \(Q_{a,u}\) 配对 | 每用户每平台一个 N（默认 10） | 仅智能调度冷却 + 配对质量列 |

N（账号）默认 **20**，1–100。不是 `user_smart_schedule_policies.quality_window_n`。

账号 overlay 的旧最少样本字段不再单独改窗；`resolved` 的样本地板 = 全局 N。门槛（p50 / 成功率 / 暂停 / or-and）仍可 overlay。

## 两窗

与配对轨同一入样：

- **\(W_{ttft}\)**：成功且 `true_first_token_ms` 或 `first_token_ms` ≥ 0。满 N 才允许判 p50。
- **\(W_{ok}\)**：`schedule_use_failover_error_rate` 计入口径的完成。满 N 才允许判成功率。

一条请求：失败 → 只 \(W_{ok}\)；同步成功无首字 → 只 \(W_{ok}\)；流式成功有首字 → 两窗。

`or`/`and`：未配置、未满 N 的指标不进入判断。

## Redis

```text
account-quality:last-n:{accountID}   JSON { n, ttft[], ok[], p50, rate, updated_at }  TTL 7d
account-quality:live:{accountID}     由 last-N 投影的 AccountQualityStats（无 resume）
account-quality:resume:{accountID}   不变（u:/w:/a）
```

`Get`：有 last-N 则投影 + 合并 resume；无 last-N 则空（不回退 15 分钟 SQL）。

`Replace`（15 分钟 SQL 覆盖 + SCAN 删 key）**不再**作为 \(Q_a\) 写入路径。tick 不得用 SQL 候选集删 last-N key。

## 数据流

```text
完成（usage 成功 / 计入的 ops 错误，任意用户）
  → append W_ttft / W_ok（FIFO 截断 N）
  → 重算 p50 / 成功率
  → 写 last-n + 投影 live
  → 合并 resume 后 EvaluateHardClose（单账号）

格子 POST /admin/accounts/quality-stats/batch
  → 批量读 last-N 投影（同一 live）
  → 旧字段名 + n / window_n / account_quality_window_n

5 分钟 tick
  → SCAN last-n → 非空则 upsert 历史快照 → 删过期行
  → 可用同一 last-N 再跑一遍硬关闭（兜底）
  → 禁止 GetAccountQualityStatsBatch(15m) 写 live
```

完成挂钩：与配对同一处（`observePairQualitySuccess` / `observePairQualityErrors`）再调账号观察者。账号观察者**不过**智能调度池 / 暂停 / 配对冷却过滤。

## 设置

KV `quality_hard_close_settings`：

- 规范：`account_quality_window_n`
- 读入别名：`window_n`、`n`
- 无显式 N：先 `min_success_samples`，再 `min_ttft_samples`，否则 20（与前端 `resolveAccountQualityWindowN` 一致；不要 `min(20,10)`）
- GET：三字段都回填为 clamp 后的 N

## 兼容

- `quality-stats/batch` 保留 `success_count` / `error_count` / `success_rate` / `p50_ttft_ms` / `ttft_samples` 等。
- `window_seconds` 可仍回 900，格子以 N 为准。
- `terminal_*` / `failover_*`：last-N 只保留调度口径的 \(W_{ok}\)。toggle 开则 `failover_error_count = error_count`，关则 `terminal_error_count = error_count`。`bridge_*` 不进窗。
- 用户维 15 分钟 batch 不动。

## 残留

- 入窗 Get-then-Set 与配对一样非原子。
- Gemini 原生路径若未走现有 usage/ops 挂钩，与配对轨同一缺口。
- 格子 30s HTTP 缓存可能略滞后于刚入窗的样本。
