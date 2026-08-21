# 设计：用户质量格 last-N

已决合同见 `prd.md`、`research/user-quality-api-contract.md`。

## 边界

| 轨 | 窗口 | 谁用 |
| --- | --- | --- |
| \(Q_a\) 账号 | 全站一个 N、两份 FIFO，该号全部用户 | 账号格、硬关闭（本任务不改语义） |
| \(Q_u\) 用户 | **同一 N**、两份 FIFO，该用户全部账号 | 用户列表格、智能调度页头用户行、用户历史弹窗 |
| \(Q_{a,u}\) 配对 | 每用户每平台一个 N（默认 10） | 仅智能调度冷却（不改） |

N 默认 **20**，1–100，来自 `quality_hard_close_settings.account_quality_window_n`。

## 两窗

复用 `ApplyAccountQualityLastNIngest` / `ToAccountQualityStats`：

- **\(W_{ttft}\)**：成功且 `true_first_token_ms` 或 `first_token_ms` ≥ 0。满 N 才允许判 p50。
- **\(W_{ok}\)**：`schedule_use_failover_error_rate` 计入口径的完成。满 N 才允许判成功率。

`user_id` 缺失的 ops 错误：**不**进 \(Q_u\)（与旧用户 SQL「`user_id IS NOT NULL`」一致）；账号 \(Q_a\) 仍可入窗。

## Redis

```text
user-quality:last-n:{userID}   JSON 与账号 last-N 同形 { n, ttft[], ok[], p50, rate, use_failover, updated_at }  TTL 7d
```

不写 `account-quality:live` / resume HASH。用户格不参与硬关闭。

`Get`：有 last-N 则投影；无 last-N 则空统计 + 盖上全局 N（不回退 15 分钟 SQL）。

## 数据流

```text
完成（usage 成功 / 计入的 ops 错误，且 user_id>0）
  → append 该 user 的 W_ttft / W_ok（FIFO 截断 N）
  → 重算 p50 / 成功率
  → 写 user-quality:last-n

格子 POST /admin/users/quality-stats/batch
  → 批量读 user last-N 投影
  → 旧字段名 + n / window_n / account_quality_window_n
  → ApplyAccountQualityScheduleCaliber（与账号格同一开关）

5 分钟 tick（可挂在现有 account-quality maintenance 之后，或独立 leader lock）
  → SCAN user-quality:last-n:* → 非空则 upsert user_quality_snapshots
  → 删 captured_at < now-7d
  → 禁止 GetUserQualityStatsBatch(15m) 写用户格
```

完成挂钩：与账号观察者同一处再调用户观察者（`observeAccountQualitySuccess` / `observeAccountQualityErrors` 旁路）。用户观察者**不过**智能调度池 / 暂停 / 配对冷却过滤。

## 历史

表 `user_quality_snapshots`：unique `(user_id, captured_at)`；`captured_at` 截到 5 分钟 UTC；字段与账号快照同形（`window_seconds` 可仍 900）。

`GET /api/v1/admin/users/:id/quality-history?from=&to=`：默认 24h，最大 7d，与账号历史同一套 `NormalizeAccountQualityHistoryRange`。

## 兼容

- batch 保留 `success_count` / `error_count` / `success_rate` / `p50_ttft_ms` / `ttft_samples`。
- `terminal_*` / `failover_*`：与账号 last-N 相同投影（toggle 开则对照计入 \(W_{ok}\)）。
- `GetUserQualityStatsBatch` 15 分钟 SQL **不再**被用户列表调用；可留作死代码或其它只读用途，不得再当列表真相。
- 前端用户列表：`quality_ttft` 改为 `mode="combined"` + 可点击（与账号格同形：p50 / 率 / 含 failover / k/N）。`quality_success_rate` 列保留以免打乱已存列设置，数据仍来自同一 last-N。

## 残留

- 入窗 Get-then-Set 与账号 / 配对一样非原子。
- Gemini 原生路径若未走现有 usage/ops 挂钩，与账号轨同一缺口。
- 格子 30s HTTP 缓存可能略滞后于刚入窗的样本。
