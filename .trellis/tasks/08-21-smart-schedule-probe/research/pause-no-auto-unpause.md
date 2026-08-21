# Research: pause-no-auto-unpause

- **Query**: 用户更正 Q1——暂停是长期手选，不存在「暂停解除默认进考察」
- **Scope**: mixed（产品合同更正；对照现网 `paused` 写入）
- **Date**: 2026-08-21

## Findings

### 更正（覆盖此前建议）

此前 planning 建议过「暂停解除 → 默认考察（清零、并发=`min(N,cap)`），管理员仍可手选其它态」。用户否定该场景：

- `paused` **总是手动**、**长期**。管理员故意切入。
- 离开暂停 **也是手动**：必须显式选下一态（考察 / 调度 / 豁免期 / 冷却）。
- **没有** implicit unpause，**没有**默认目的地。
- 删除 / 改写任何「取消暂停 → 默认考察」需求。
- 仍保持：管理员可随时手跳 暂停 / 冷却 / 考察 / 调度 / 豁免期（灵活、不死锁）。
- Q2–Q4 不动：`and` 混合 → 冷却；调度可拉回考察；不回填。

自动链因此只剩：

```text
冷却到期 → 考察 → 调度
```

豁免期仍是仅手动快路径。暂停不出现在自动链上。

### Files Found

| File Path | Description |
| --- | --- |
| `.trellis/tasks/08-21-smart-schedule-probe/prd.md` | Decisions §1、AC6、Out of Scope 已按更正改写 |
| `.trellis/tasks/08-21-smart-schedule-probe/design.md` | 状态机去掉「暂停解除默认态」 |
| `.trellis/tasks/08-21-smart-schedule-probe/implement.md` | `SetPairAdmission` 禁止清暂停暗写 probing |
| `.trellis/tasks/08-21-smart-schedule-probe/research/probe-logic.md` | 合同清单已去自动 unpause |
| `backend/internal/service/user_smart_schedule_service.go` | 现网 `SetPairAdmission`：`paused` 为显式手选 |
| `frontend/src/composables/smartSchedulePoolAdmission.ts` | 现网切换器：切走暂停需再点一态 |

### Code Patterns

现网入池 `paused` 是 `user_smart_schedule_accounts.paused`（或等价 SQL 旗标），由管理员 `SetPairAdmission(state=paused)` 写入。热路径 `admitsScheduleUser` 见暂停即拒，不入窗。没有墙钟、没有 TTL、没有「解除后进哪」的后续队列。

离开暂停的唯一现网路径是再调一次 `SetPairAdmission` 并带目标 `state`。`ParsePairAdmissionState` 省略 `state` 现网落到 `resumed`——这是豁免期写入默认，**不得**解释成「暂停解除默认」，也 **不得** 改成默认 `probing`。

### External References

无。纯本仓库产品合同。

### Related Specs

- `.trellis/spec/backend/account-user-schedule.md` — 闭池准入；`paused` 跳过
- `.trellis/tasks/08-21-smart-schedule-probe/prd.md` — Decisions Q1

## Caveats / Not Found

- 仓库内此前若出现「暂停解除默认进考察」，以本更正与 `prd.md` Decisions 为准。
- 不把省略 `state` = `resumed` 改成 `probing`。
- 本文件不授权改业务代码。
