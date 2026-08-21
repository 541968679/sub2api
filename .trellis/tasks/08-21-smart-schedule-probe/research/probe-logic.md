# Research: probe-logic

- **Query**: 把已锁的智能调度考察期合同收成可勾选清单；去掉「暂停解除默认进考察」；保留 Q2–Q4
- **Scope**: mixed（已锁产品合同 + 现网挂钩指针；Q1 经用户更正）
- **Date**: 2026-08-21

## Findings

### Files Found

| File Path | Description |
| --- | --- |
| `.trellis/tasks/08-21-smart-schedule-probe/prd.md` | 本任务需求与验收；Q1 更正 + Q2–Q4 已并入 |
| `.trellis/tasks/08-21-smart-schedule-probe/design.md` | 挂钩、状态机、反死锁 |
| `.trellis/tasks/08-21-smart-schedule-probe/research/pause-no-auto-unpause.md` | Q1 更正：暂停无自动解除 |
| `.trellis/spec/backend/account-user-schedule.md` | 闭池 + 配对冷却现网合同；L153 仍写「到期同可调度」 |
| `backend/internal/service/account_user_schedule.go` | `admitsScheduleUser` L230–269 |
| `backend/internal/service/account_user_concurrency.go` | `resolvePairSlotAcquire` L33–45 现只读 `PairCap` |
| `backend/internal/service/user_smart_schedule.go` | `PairAdmission*` 常量 L210–215，无 `probing` |
| `backend/internal/service/user_smart_schedule_service.go` | `ParsePairAdmissionState` L253–264；`SetPairAdmission` L271–336；`ObservePairCompletion` L80–113 |
| `backend/internal/service/smart_schedule_pair_quality.go` | `pairQualityBlocks` L343–348 |
| `backend/internal/service/account_quality_hard_close.go` | `EvaluateAccountQualityHardClose` and/or L329–394 |
| `backend/internal/service/smart_schedule_pair_quality_test.go` | `TestPairQualityBlocks_UnderNAndOr` L53–74：and 一满一好则不冷 |
| `backend/internal/repository/user_smart_schedule_cache.go` | `expirePairCooldown` L221–227 现到期当调度 |
| `frontend/src/composables/smartSchedulePoolAdmission.ts` | 池状态机，尚无 `probing` |
| `frontend/src/components/admin/smart-schedule/SmartScheduleAdmissionSwitch.vue` | 四态菜单，源 `PAIR_ADMISSION_LIVE_STATES` |
| `.trellis/tasks/08-21-smart-schedule-recovery-probe/` | 已交付配对 last-N；**只读、不覆盖** |
| `.trellis/tasks/08-21-account-quality-last-n/` | 账号 last-N / 硬关闭；**本任务不改** |

### Code Patterns

现网到期：`CooldownActive` 见过期 unix → `expirePairCooldown` → `HDEL` + `ZeroPairQuality`（`expiry_zero`），之后无标记即调度。`resolvePairSlotAcquire` 闭池成员返回 `policy.PairCap`（0 = 不夹）。`ParsePairAdmissionState` 只认 `paused|cooling|resumed|selectable`，空 = `resumed`。`paused` 是 SQL 旗标，现网没有「解除暂停」自动过渡。

`pairQualityBlocks` 委托 `EvaluateAccountQualityHardClose`。未满的指标不进入 and/or。`and` 要 **已参加判断的指标全部越界** 才冷。因此 `and` + \(W_{ttft}\) 满且 p50 越界 + \(W_{ok}\) 满且成功率过 → `pairQualityBlocks==false`（见 `TestPairQualityBlocks_UnderNAndOr` L70–73）。考察里若同时因成功率过不了毕业、因 `and` 不了回冷，就会夹着空转——Q2 专则补上：回冷却。

调度态保持现网 `and`，不用这条专则。

规格 L153 仍写「Auto cooldown expiry is the same as 可调度」。本任务改到期进考察；实现阶段改规格，现在不要改 `.trellis/spec/`。

省略 `state` = `resumed` 是豁免期写入默认，**不是**暂停解除默认；实现时不得借此、也不得改成从暂停默认 `probing`。

### 合同清单（逐条已锁）

**任务边界**

- [x] 新任务 `08-21-smart-schedule-probe`，标题「智能调度考察期」
- [x] 不复用、不覆盖 `08-21-smart-schedule-recovery-probe`
- [x] 本 planning 回合不改业务代码、不 `task.py start`、不 commit / push / deploy
- [x] 不改客户端协议
- [x] 不发明同步假 TTFT
- [x] 不改账号质量 last-N / 硬关闭
- [x] 配对冷却仍只读 \(Q_{a,u}\)

**状态（UI / API）**

- [x] 暂停 `paused` — 仅手动进入、仅手动离开；长期保持；跳过；不入窗；**无自动解除、无默认下一态**
- [ ] 冷却 `cooling` — 越界自动或手动；墙钟 `HSETNX`；不入窗
- [ ] 考察 `probing` — 冷却到期自动；也可手动（含从调度拉回）；评价配对质量
- [ ] 调度 `selectable` — 考察毕业自动；也可手动跳过
- [ ] 豁免期 `resumed` — **仅手动**；现网 `u:` 15m + `w:` 至 30m；宽限内不评价；宽限内入窗
- [x] 不发明第六态 / 「解除暂停」中间态

**路径**

- [ ] 唯一自动链：冷却到期 → 考察 → 调度（到期 **只** 进考察）
- [ ] 手动快路径：豁免期 →（宽限期满）→ 调度
- [x] **没有**「取消暂停 → 默认考察 / 默认调度」。离开暂停 = 再手选 考察 / 调度 / 豁免期 / 冷却
- [x] 清 `paused` 不得暗进 `probing`
- [x] 允许调度手选拉回考察：不经冷却、两窗清零、并发夹紧

**考察并发**

- [ ] = 策略 N（与双窗同一 N，默认 10，范围 1–100）
- [ ] 有成员 cap：`min(N, cap)`
- [ ] 无 cap：N
- [ ] 只限 in-flight

**进入考察（到期或管理员，含调度拉回）**

- [ ] 清冷却
- [ ] 两窗清零
- [ ] 不写 `u:` / `w:` 宽限

**考察 → 调度（毕业）**

- [ ] 保留窗口
- [ ] 必须 \(W_{ok}\) 已满 N
- [ ] 若配置了成功率门槛：成功率在门槛内
- [ ] \(W_{ttft}\) 空 / 未满（同步-only）**不挡**
- [ ] \(W_{ttft}\) 已满则 p50 必须过（否则回冷却）
- [ ] 无按时间毕业
- [ ] 无流量：留下等待，不毕业

**考察 → 冷却**

- [ ] 与现网配对冷却相同的 or/and
- [ ] 未满的指标不参加
- [x] **考察专则（Q2）**：`and`、两窗都满、一好一坏 → 回冷却（不要夹着空转）
- [x] 专则只作用于考察；调度仍用现网 `and`

**手动调度**

- [ ] 清冷却
- [ ] 两窗清零
- [ ] 无 `w:` 宽限
- [ ] 成员原 cap
- [ ] 清考察标记

**手动豁免期**

- [ ] 清冷却
- [ ] 两窗清零
- [ ] 现网时间宽限
- [ ] 成员原 cap
- [ ] 清考察标记
- [ ] 宽限期满 → 调度，保留宽限期内窗口，然后再评价

**上线不回填（Q4）**

- [x] 已在调度的配对维持调度、原 cap
- [x] 只有新的冷却到期（或新手选考察）进考察
- [x] Redis miss / 无标记 = 非考察
- [x] 禁止部署扫描回填

### 反死锁 / 灵活跳转（2026-08-21 原则）

- [x] 管理员可随时跳到：暂停 / 冷却 / 考察 / 调度 / 豁免期
- [x] 自动冷却到期 → 考察 only
- [x] 同步-only / \(W_{ttft}\) 空永不挡毕业（\(W_{ok}\) 满 N）
- [x] 考察 `and` 混窗 → 冷却
- [x] 考察无流量 = 留下等待；管理员可强制调度 / 豁免期
- [x] 无按时间毕业
- [x] 不发明额外状态
- [x] 暂停无自动出口、无默认目的地

### 已决决策（禁止再发明相反答案）

1. **Q1 更正（覆盖「默认进考察」建议）**：不存在暂停解除场景。`paused` 是管理员故意切入的长期态。离开也必须手选下一态。无 implicit unpause，无默认目的地。删除一切「取消暂停 → 默认考察」。
2. **Q2**：考察内 `and`、两窗都满、一好一坏 → 回冷却。调度态仍用现网 `and`。
3. **Q3**：允许从调度手动拉回考察。
4. **Q4**：上线不回填。

### Related Specs

- `.trellis/spec/backend/account-user-schedule.md` — 闭池、冷却 HASH、豁免期 / 可调度、配对窗
- `.trellis/tasks/08-21-smart-schedule-recovery-probe/research/pair-quality-n-window.md` — 已交付双窗合同（只读）
- `.trellis/tasks/08-21-smart-schedule-probe/research/pause-no-auto-unpause.md` — 暂停无自动出口

## Caveats / Not Found

- 现网 **没有** `probing` 标记、没有到期进考察、没有考察 cap。
- 现网 **没有** 「暂停解除」自动过渡；Q1 更正与现网一致（只有手选 `SetPairAdmission`）。
- 规格 L153 与本任务冲突（到期同可调度）；实现时改规格，planning 不改 `.trellis/spec/`。
- Q1–Q4 已决，不再阻塞。`and` 混窗专则是考察覆盖，不是改 `EvaluateAccountQualityHardClose` 全局语义。
- 账号 last-N 任务并行，本清单明确排除那条轨。
- 本回合仍不 start。
