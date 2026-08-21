# 用户质量格 last-N（与账号格一致）

## Goal

管理端**用户列表**与智能调度页头用户行的质量格，从「15 分钟 SQL」改成与**账号质量格**同一套 last-N 双 FIFO 窗。人口是**该用户跨全部账号**，不是该号跨全部用户。

## Background

账号格 \(Q_a\) 已是站点级 last-N（`account_quality_window_n`，默认 20）：\(W_{ttft}\) / \(W_{ok}\)，完成入窗即重算，格子读 last-N 投影，含 failover 对照。用户格仍走 `POST /admin/users/quality-stats/batch` 的 15 分钟 SQL，且强制 `failover_error_count=0`。点击账号格打开质量历史快照；用户格无对等历史。

## Confirmed Facts

1. 用户格人口 = 该 `user_id` 在所有账号上的完成样本；不是配对 \(Q_{a,u}\)，不是账号 \(Q_a\)。
2. 两窗与入样规则与账号 \(Q_a\) 相同：失败只进 \(W_{ok}\)；同步成功无首字只进 \(W_{ok}\)；流式成功且有首字两窗都进。\(W_{ok}\) 计入受 `schedule_use_failover_error_rate`。未满 N 的指标不判断。
3. N 复用全局 `account_quality_window_n`（默认 20，1–100）。不新增用户专用 N。
4. 指标与账号格相同：p50 TTFT、成功率、**含 failover 的错误率对照**。用户 batch 不得再把 `failover_error_count` 钉成 0。
5. 入窗挂钩与账号 last-N 相同：gateway / OpenAI `RecordUsage` 成功 + 计入的 ops 错误。按 `user_id` 入窗；`user_id` 为空的错误不进用户窗（账号窗仍可进）。
6. 用户列表 / 页头必须读 last-N batch，不得再以 15 分钟 SQL 为真相。15 分钟 SQL 仅在其它调用方仍需要时保留。
7. 点击弹窗需要用户维历史（5 分钟 tick 快照 last-N，或对等入窗快照），否则弹窗为空。
8. 不做配对质量、账号硬关闭、考察期、客户端协议、计费。

## Requirements

- R1. 同一套 last-N 两窗与入样规则，key = `user_id`，该用户全部账号共享一窗。
- R2. N = 全局 `account_quality_window_n`。
- R3. `POST /admin/users/quality-stats/batch` 返回与账号格相同字段形（`ttft`/`ok` 计数、`n`/`window_n`/`account_quality_window_n`、success_rate、含 failover 的 error_rate、p50）。
- R4. 完成路径入窗（usage 成功 + 计入 ops 错误）。
- R5. 用户维质量历史 API + 5 分钟 tick 快照，供点击弹窗；契约写在 `research/user-quality-api-contract.md`。
- R6. 用户列表 / 智能调度页头质量格展示与账号格一致（p50、成功率、含 failover 对照、k/N）；点击打开用户历史弹窗。

## Acceptance Criteria

- [ ] AC1. 用户 A 与用户 B 的完成互不进入对方用户窗。映射 R1、R4。
- [ ] AC2. `schedule_use_failover_error_rate` 计入口径的失败进入 \(W_{ok}\)；用户 batch 的 `failover_error_count` 不再恒为 0。映射 R3、R4。
- [ ] AC3. 用户列表 batch 读 last-N，不再查 15 分钟 SQL 作为列表真相。映射 R3。
- [ ] AC4. batch / 投影上的 N 来自全局 `account_quality_window_n`（默认 20）。映射 R2。
- [ ] AC5. `GET /admin/users/:id/quality-history` 返回 last-N 快照点；无流量用户 `items: []`。映射 R5、R6。

## Out of Scope

- 配对 \(Q_{a,u}\)、账号硬关闭、考察期。
- 客户端改 `stream` / 端点 / body。
- 计费、展示 token。
- 新增第二个站点 N 旋钮。

## Open Questions

无。合同已锁。
