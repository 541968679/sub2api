# 用户智能调度恢复考察期

## Goal

智能调度冷却改用 **账号×用户** 自己的质量：窗口 = 最近 N 条完成样本（用户参数区可配），每完成一条重算；冷却结束回到可调度时窗口清零再累计。账号级质量仍是 15 分钟窗口 / 5 分钟间隔，供格子和硬关闭，不参与这对冷却。

考察期本阶段不做。完整陈述：`research/pair-quality-n-window.md`。

## Background

管理员按 `(user, platform)` 配封闭账号池。热路径 `admitsScheduleUser` 只认池成员、`paused`、配对冷却和质量门槛；开启后忽略账号侧旧 allow/deny/gate/cap。

当前入池状态分两层，容易混：

| 层 | 状态 | 是否真锁 | 热路径行为 |
| --- | --- | --- | --- |
| 管理员可写 | `paused` | 是 | 池内跳过，不写冷却。`user_smart_schedule_accounts.paused` |
| 管理员可写 | `cooling` | 是 | `smart-schedule:cooldown:{accountID}` field `u:{userID}` 未到期则拒绝 |
| 管理员可写 | `resumed` | 否 | 清暂停+冷却；写 `account-quality:resume` `u:`（15m 芯片）+ `w:`（30m 观察）。期间质量门槛 **fail-open** |
| 管理员可写 | `selectable` | 否 | 清暂停+冷却；删 `u:`，写 `w:` 15m。观察窗内同样 **fail-open** |
| 仅展示 | `stopped` | 账号级 | `status/schedulable/temp_unschedulable/rate_limit` 不可调度 |
| 仅展示 | `pair_full` | 运行时 | 配对占用 ≥ 真实 cap |
| 仅展示 | `will_cool` | 否 | 已保存门槛未达标，冷却尚未 `HSETNX` |
| 仅展示 | `unsaved_preview` | 否 | 草稿门槛未达标 |

`resumed` / `selectable` 今天的设计目标是「避免用暂停前的陈旧 15 分钟窗口立刻再冷却」，**不是**限流考察。配对 cap 恢复后仍是池成员原值；未设 cap 时热路径不限（UI 999 仅展示）。

## Confirmed Facts

1. 质量窗口是 **账号级** 15 分钟 Redis `account-quality:live:{accountID}`，不是 user×account。见 `.trellis/spec/backend/account-user-schedule.md` L21、`account_quality.go` L12–22。
2. 写入方只有维护任务 `RunTick`，间隔 `AccountQualitySnapshotInterval = 5m`。热路径只 `Get`。见 `account_quality_maintenance.go` L136–177。
3. `EvaluateAccountQualityHardClose`：未配置指标不判；**样本不足不判**（默认成功样本 20、TTFT 样本 10）。`stats == nil` 不拦截。见 `account_quality_hard_close.go` L329–371。
4. 智能调度热路径：门槛命中且 `UserQualityResumeActive`（`u:` 或 `w:` 未过期）则直接放行，**不写冷却**。见 `account_user_schedule.go` L230–264、`account_quality.go` L336–344。
5. 管理员切 `resumed`：`MarkUserResume` → `u:=now+15m`、`w:=now+30m`。切 `selectable`：`MarkUserQualityWindow` → 删 `u:`、`w:=now+15m`。见 `user_smart_schedule_service.go` L222–282、`account_quality_live_cache.go` L153–171。
6. 冷却自动到期后没有考察态：`CooldownActive` 变 false 即按可调度准入，cap 回到池成员原值。`resolvePairSlotAcquire` 只读 `policy.PairCap`，不看恢复/冷却。见 `account_user_concurrency.go` L33–44。
7. 冷却一旦写入，质量变好也不会提前解除；要等到期或管理员清除。测试：`user_smart_schedule_test.go` L126–140。
8. 账号级质量硬关闭 / `IsSchedulable()` / `TempUnschedulableUntil` 不在本任务范围。智能调度不得折进整号暂停。
9. 窗口计算与配对判定是两套逻辑。窗口按账号（或展示用用户）聚合 15 分钟已完成日志；判定用该用户门槛 / `u:` `w:` 宽限 / 冷却 HASH 去读账号窗口。没有 `(account_id, user_id)` 调度窗口。详见 `research/pair-cooldown-vs-quality-window.md`。
10. 配对冷却是墙钟锁：`HSETNX` `until=now+cooldown_minutes`。过期 `HDEL` 后无残留。质量变好不提前解。旧 `account_schedule_users` 门槛越界只排除、不写这对冷却。
11. 样本门槛约束的是窗口里已完成行，不约束 in-flight。热路径读的是最多 5 分钟前的 Redis live；管理端质量格是现查 SQL 15m + 30s 缓存。
12. **Q1 已决**：冷却自动到期默认进考察，不得直接满额。只有管理员显式 `selectable` 跳过。

## Problem

因果链（暂停或冷却 → 可调度）：

```text
paused / cooling 解除
  → 立刻按池 cap（或无 cap）准入
  → 该用户积压请求一次性打到此号
  → 号尚未恢复，TTFT 变差
  → 质量窗口最多 5 分钟才刷新，且要凑够 10/20 样本才判
  → 若处于 resumed/selectable 的 u:/w: 宽限，即使已越界也不冷却
  → 慢请求已经打出去了
```

用户指出的两点都成立：

- **冷却判定偏晚 / 偏钝**：5 分钟快照 + 默认 10/20 样本 + cache miss fail-open。冷却期无流量时窗口常被清空，到期后更是「样本不足 → 放行」。
- **缺真正的中间态**：现有 `resumed` 是质量豁免，不是限并发考察。从暂停点「已恢复」或冷却到期，都会把并发打满。
- **样本数 ≠ 打满上界**：门槛齐样前，配对 cap（或无限）已经决定能同时进多少。样本=10、并发=50 时，冷却触发前飞出去的是 50 不是 10。窗口还不含进行中的请求。

## Requirements

- R1. 账号质量 \(Q_a\)：窗口 15 分钟、间隔 5 分钟、全局一份。计算路径与现网一致。不作为智能调度配对冷却输入。
- R2. 配对首字质量：仅 `(account_id, user_id)` 最近 N 条 **成功且有首字** 的完成请求。其他用户打同一号不进窗。N 由原最少成功/TTFT 样本两字段改成。失败、无首字的同步成功 **不进** \(W_{ttft}\)。
- R3. 每入 \(W_{ttft}\) 一条即重算配对 p50。\(W_{ttft}\) 条数 `< N` 不因首字门槛写冷却。冷却结束回到可调度时该窗清零再累计。
- R4. 智能调度冷却的首字判断只读配对 \(W_{ttft}\) 的 p50，不读账号 15m live。冷却锁仍是 `HSETNX` + `cooldown_minutes`。
- R4b. 入池状态展示：「已恢复」改名为「豁免期」。豁免期内入窗、不判断；期满再判断。可调度清零攒 N，无时间豁免。
- R5. 池表 **新增一列「配对质量」**，展示该用户×该账号的 \(Q_{a,u}\)（至少首字 p50 / 窗口条数）。现有账号质量列保留，仍走轨 A。
- R6. 点击配对质量列打开详情：最近趋势（配对窗快照，不是账号 5 分钟 \(Q_a\) 历史），以及该配对的冷却 / 恢复记录。
- R7. 未开智能调度的旧门槛、账号硬关闭、账号质量格，仍用轨 A。
- R8. 配对成功率用 \(W_{ok}\)：最近 N 条计入口径的完成请求（含失败，failover 开关有效）。与 \(W_{ttft}\) 共用 N、各攒各的。该窗 `< N` 不因成功率冷却。
- R9. 不改客户端协议。本阶段不做考察期 / 恢复夹并发。

考察期旧 R1–R8 暂停，不作为本阶段验收。

## Acceptance Criteria

本阶段（配对 N 条窗）草案，Q8/Q9 确认后冻结：

- [ ] AC1. 账号质量格 / 5 分钟 tick / 硬关闭仍只用 15 分钟 \(Q_a\)，数值不因配对冷却改口径。映射 R1、R6。
- [ ] AC2. 智能调度冷却越界只由该配对最近 N 条 \(Q_{a,u}\) 触发，不读 `account-quality:live`。映射 R2、R5。
- [ ] AC3. 该配对每完成一条入窗样本即重算 \(Q_{a,u}\)；failover 开关与账号轨同一配置。映射 R3。
- [ ] AC4. 冷却到期或管理员解除冷却后，\(Q_{a,u}\) 为空；再次冷却最早发生在新窗口攒满之后（若 Q8 确认未满不判）。映射 R4。
- [ ] AC5. 用户 A 的慢请求不进入用户 B 在同一账号上的 \(Q_{a,u}\)。映射 R2。

## Out of Scope

- 账号级 `schedulable` / 临时不可调度 / 529 过载整号冷却。
- 账号页用户门槛「立即恢复」合同重做（除非 resume HASH 字段冲突必须兼容）。
- 把质量窗口改成 user×account 全局口径（考察计数可以是配对局部，账号 15m 列保持账号口径）。
- 跨用户复制策略、自动排序、PnL。
- 降低全局默认 10/20 样本（只在考察路径用更短的齐样条件）。

## Open Questions

- 考察期、Q6、Q7 **暂停**。
- **Q8 已决**：窗口没攒满不冷却。\(W_{ttft}.len < N\) 不因 p50 冷却；\(W_{ok}.len < N\) 不因成功率冷却。
- **Q9 已决**：智能调度参数区原来的最少成功样本 / 最少 TTFT 样本改成同一个 N。两窗齐样都是 N。
- **Q10 已决**：配对冷却仍判成功率。与首字共用 N，但是两份队列。
- **Q11 已决**：\(W_{ttft}\) / \(W_{ok}\) 只计入该账号×该用户的请求，不吃其他用户打这个号的样本。
- **Q12 已决**：不是「保留冷却前窗口」。
  - **豁免期**（原名「已恢复」）：一段时间完全不判断、不写冷却。时长仍用现网 `u:` 15 分钟芯片 + `w:` 最长 30 分钟。界面状态名改为「豁免期」。
  - **可调度**：无时间豁免。两窗清零，重新攒 N；未满 N 不冷却。去掉点可调度仍写 15 分钟 `w:`。
  - **冷却自动到期**：按可调度，清零再攒 N。
- **Q13 已决**：豁免期内新完成 **入两窗**，期满后再判断。期满时若已满 N，可以马上冷却（用的是豁免期内的新样本，不是冷却前窗口）。
- **Q14 已决**：只改中文「豁免期」。API / 代码仍是 `resumed`。
- **Q15 已决**：N 默认 10。范围 **1–100**。

## Technical Notes

- 热路径：`backend/internal/service/account_user_schedule.go` `admitsScheduleUser` L230–264。
- 配对 cap：`backend/internal/service/account_user_concurrency.go` `resolvePairSlotAcquire` L33–44。
- 状态写入：`backend/internal/service/user_smart_schedule_service.go` `SetPairAdmission` L222–282。
- 质量评价：`backend/internal/service/account_quality_hard_close.go` L329–371。
- 规格：`.trellis/spec/backend/account-user-schedule.md` Scenario: user × platform smart schedule。
- 前端状态机：`frontend/src/composables/smartSchedulePoolAdmission.ts`。
