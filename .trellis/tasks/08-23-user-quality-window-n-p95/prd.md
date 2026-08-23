# 用户全局质量：每用户窗口 N + 格子显示 P95

## Goal

管理员能按**用户**调整该用户全局质量 \(Q_u\)（该用户、全部账号）的 last-N 口径，并在用户质量格子里直接看到 P95。不再全站绑死账号质量 N。

## User value

不同用户请求密度不同：有的看 10 次尾部更准，有的要 50 次才稳。改这个用户的 N 不影响账号列表 \(Q_a\)、硬关闭，也不影响智能调度配对质量 \(Q_{a,u}\)。格子不用再靠 tooltip 猜 P95。

## Background

- \(Q_u\) 已是 Redis last-N（`user-quality:last-n:{userID}`），与 \(Q_a\) 同一套 FIFO / P50 / P95 算法。
- 现网采集和格子都读全站 `account_quality_window_n`（默认 20）。旧规格写死「不要第二颗 N」。本任务**有意改掉**这条：N 变成每用户可覆盖。
- 用户质量格子 `combined` 模式只露 p50 / 成功率 / failover / k/N；P95 已在 stats 和 tooltip / 详情曲线里，格子本体没有。
- 智能调度「窗口样本数 N」是 \(Q_{a,u}\)，默认 10，本任务不动。

## Locked decisions（Brandon 绑定）

1. 占用 checkout `E:\cursor project\api2sub` 写产品代码。禁止为这个任务开隔离 worktree。
2. 口径是 \(Q_u\)：该用户、全部账号。不是配对质量，不是账号质量。
3. N **每用户单独可调**。未设置回落到全站 `account_quality_window_n`（默认 20）。
4. 编辑入口：用户质量详情弹窗（点用户质量格子打开的 `UserQualityDialog`）。
5. 范围 1–100，与账号质量 N 同一套 clamp。
6. 用户质量格子（`subject=user` + `combined`）直接显示 P95。账号格子布局不改。
7. 不改 stored billing / `actual_cost` / 展示变换。不改 \(Q_{a,u}\)。不要求客户端改协议。
8. 不 commit / push / deploy（需另一次明确批准）。

## Requirements

- R1. `users.quality_window_n` 可空。`NULL` = 继承全站账号质量 N；`1–100` = 该用户 \(Q_u\) 覆盖。
- R2. \(Q_u\) 采集、列表 batch、历史投影、格子 k/N 都用**该用户解析后的 N**。\(Q_a\) 采集继续只用全站账号质量 N。
- R3. 管理员在 `UserQualityDialog` 能看到当前生效 N、改覆盖值、清回继承。保存后 Redis 窗口立刻按新 N 裁剪，不必等下一请求。
- R4. `PUT /admin/users/:id`：`quality_window_n` omit = 不改；`null` = 清回继承；数字 = 设覆盖（clamp 1–100）。
- R5. 用户质量 `combined` 格子在 p50 下显示 P95（有样本才显示数值，否则 `—`）。
- R6. AdminUser / 质量 stats 能区分「覆盖」和「继承」：stats 的 `n` / `window_n` 是解析后的生效值；用户 DTO 的 `quality_window_n` 是覆盖（null = 继承）。
- R7. 单测 + 前端单测覆盖 R1–R6。中英 i18n 同步。

## Acceptance Criteria

- [ ] AC1. 用户 A 覆盖 N=10、用户 B 继承全站 20：两人各自窗口互不影响；同一账号上的完成分别进各自 \(Q_u\)。映射 R1、R2。
- [ ] AC2. 改全站账号质量 N 只改 \(Q_a\) 和继承用户的 \(Q_u\)，已覆盖用户不变。映射 R2。
- [ ] AC3. 弹窗把 N 从 20 改成 8 并保存：下一次 batch / 格子 k/N=8，Redis FIFO 不超过 8。映射 R3、R4。
- [ ] AC4. 弹窗清回继承：覆盖清空，生效 N 回到全站值。映射 R3、R4、R6。
- [ ] AC5. 用户质量格子可见 `p95` 标签和 `p95_ttft_ms`；账号 `combined` 格子仍不新增 P95 行。映射 R5。
- [ ] AC6. 配对质量格子 / 智能调度 windowN / 计费字段无改动。映射 Locked 7。

## Out of scope

- 智能调度配对质量 \(Q_{a,u}\) 的 N / P95
- 账号质量格子改布局
- 每用户硬关闭阈值（账号侧 user quality gate 已有）
- commit / push / 生产部署
- 新全站 Settings 键（不是第二颗全站 N）

## Open questions

无。Brandon 已确认：\(Q_u\)、每用户可调、弹窗改 N、格子显示 P95。
