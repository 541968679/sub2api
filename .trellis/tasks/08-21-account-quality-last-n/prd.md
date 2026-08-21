# 账号质量最近N条（格+硬关闭）

## Goal

账号质量轨 \(Q_a\)（该号上**所有用户**）从「15 分钟 SQL 窗 + 5 分钟 tick」改成**站点级最近 N 条双 FIFO 窗**。管理端账号质量格与硬关闭 / 临时不可调度读**同一份** live \(Q_a\)，禁止再拆口径。

## Background

当前 \(Q_a\) 由维护任务每 5 分钟用 `usage_logs` + `ops_error_logs` 聚 15 分钟窗口，写入 Redis `account-quality:live` 并触发 `EvaluateAccountQualityHardClose`。格子 `POST /admin/accounts/quality-stats/batch` 另查同一套 15 分钟 SQL。配对轨 \(Q_{a,u}\) 已是每用户最近 N 条，与本任务无关。

## Confirmed Facts

1. \(Q_a\) 是账号级，吃该号全部用户的完成样本；不是智能调度 per-user `quality_window_n`。
2. 配对冷却只读 \(Q_{a,u}\)，不得改回读 `account-quality:live`。
3. 格子与硬关闭必须同一套 last-N 统计；未满 N 的指标不判断（格=样本不足；硬关闭不因该指标触发）。
4. 入窗规则与配对轨相同：失败只进 \(W_{ok}\)；同步成功无首字只进 \(W_{ok}\)；流式成功且有 `true_first_token_ms` / `first_token_ms` 两窗都进。\(W_{ok}\) 计入受 `schedule_use_failover_error_rate`。
5. 每条完成入窗即重算。15 分钟 SQL 与 5 分钟 tick **不再是** 本轨真相源。
6. 全站一个 N，Settings KV，默认 **20**，clamp **1–100**。旧 `min_success_samples` / `min_ttft_samples` 收敛为这一个 N；GET 回填旧字段为同一 N。
7. 不做考察期、不改客户端协议、不重写配对质量、不计费。

## Requirements

- R1. 站点级 N：`quality_hard_close_settings` 增加规范字段 `account_quality_window_n`（别名 `window_n` / `n`）。默认 20，范围 1–100。GET 同时回填 `min_success_samples`、`min_ttft_samples` 为 N。
- R2. 账号上两份 FIFO：\(W_{ttft}\)（成功且有首字 → p50）、\(W_{ok}\)（计入口径完成 → 成功率）。未满 N 不判断该指标。
- R3. 完成路径（usage 成功 + 计入的 ops 错误）入窗，含所有用户；不要求智能调度池成员。
- R4. `account-quality:live` 与 `quality-stats/batch` 与硬关闭都读 last-N 投影。JSON 旧字段名尽量不动；暴露 N 以便格子显示 `k/N`。
- R5. 硬关闭在入窗重算后读同一 live；`AccountQualityResumeActive` 仍豁免重关。
- R6. 5 分钟 tick 不再用 15 分钟 SQL 覆盖 live；可选把当前 last-N 快照进历史表。
- R7. 用户维 `users/quality-stats/batch`、配对 `quality_window_n`、豁免 HASH `u:`/`w:`/`a` 不在本任务改语义。

## Acceptance Criteria

- [ ] AC1. 默认 N=20；写入 0/101 被拒绝或 clamp 到 1–100；GET 三字段同值。映射 R1。
- [ ] AC2. 失败只增 \(W_{ok}\)；无首字成功只增 \(W_{ok}\)；有首字成功两窗都增。映射 R2、R3。
- [ ] AC3. \(W_{ttft}.len < N\) 不因 p50 硬关闭；\(W_{ok}.len < N\) 不因成功率硬关闭。映射 R2、R5。
- [ ] AC4. 格子 batch 与 live / 硬关闭数值来自同一 last-N，不再查 15 分钟 SQL 作为真相。映射 R4、R6。
- [ ] AC5. 用户 A 与用户 B 打同一号都进该号 \(Q_a\)；配对冷却仍只看 \(Q_{a,u}\)。映射 R3、R7。

## Out of Scope

- 考察期 / 恢复夹并发。
- 客户端改 `stream` / 端点 / body。
- 配对质量窗重写。
- 计费、展示 token。
- 用户列表 15 分钟质量格。

## Open Questions

无。合同已锁。
