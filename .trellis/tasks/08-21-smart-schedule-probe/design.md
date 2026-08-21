# 设计：智能调度考察期

合同权威源：`prd.md` + `research/probe-logic.md`。Q1–Q4 已锁；**Q1 经用户更正：无「暂停解除」自动链、无默认下一态**。本文件只落挂钩与形状，不重开合同。本回合不实现、不 `task.py start`。

原则：**越灵活越好，避免状态死锁。** 不发明第六态。不发明 `unpause`。

## 边界

| 轨 | 本任务 |
| --- | --- |
| \(Q_{a,u}\) 配对双窗 / 配对冷却 | **沿用** 已交付 last-N。评价与回冷仍走这对。进入考察时清零；毕业保留。 |
| \(Q_a\) 账号质量 / 硬关闭 | **不改**。进行中任务 `08-21-account-quality-last-n`。 |
| 客户端协议 | **不改**。不发明同步假 TTFT。 |
| 旧任务目录 `08-21-smart-schedule-recovery-probe` | **只读**。不覆盖。 |

## 状态机（已锁）

```text
自动（仅此一条）:
  越界或手选 ──► 冷却 ──(墙钟到期)──► 考察 ──(毕业)──► 调度
                    ▲                  │
                    └──── 回冷 ────────┘
                         （含 and 两窗满、一好一坏）

手动（任意时刻可跳）:
  暂停 / 冷却 / 考察 / 调度 / 豁免期

暂停:
  仅手动进入；长期保持；无自动解除；无默认下一态。
  离开暂停 = 再手选 考察 / 调度 / 豁免期 / 冷却（走该态副作用）。
  禁止：清 paused → 暗进 probing / selectable。

调度 ──手选拉回──► 考察（不经冷却、清零、夹紧）

手动快路径:
  手选豁免期 ──(u: 15m + w: 至 30m，期内不评价)──► 调度（保留宽限期窗口，再评价）
```

仅手动：暂停（跳过、不入窗，长期）；豁免期。自动到期 **只** 进考察。

上线不回填：已在调度的配对维持调度；只有新的冷却到期或新手选考察才进考察。Redis miss / 无标记 = 非考察。

## 反死锁 / 灵活跳转

| 规则 | 行为 |
| --- | --- |
| 管理员跳转 | 随时可去暂停 / 冷却 / 考察 / 调度 / 豁免期。调度态不禁用「考察」。 |
| 暂停出口 | **无自动出口**。离开必须显式点下一态。不得暗默认 `probing` 或 `selectable`。 |
| 自动到期 | 只进考察，不进调度、不进豁免期。 |
| 同步 / 空 TTFT | \(W_{ttft}\) 空或未满不挡毕业（\(W_{ok}\) 满 N）。 |
| 考察 `and` 混窗 | 两窗都满、一好一坏 → 冷却，不空转。 |
| 考察无流量 | 留下等待。不发明等待态。管理员可强制调度 / 豁免期。 |
| 毕业钟 | 无。禁止用 `u:`/`w:` / 墙钟当毕业条件。 |
| 状态集合 | 就是上表五态。禁止 `stuck` / `probe_waiting` / `probe_mixed` / `unpause`。 |

## 并发（已锁）

考察 in-flight cap：

- 策略 N（与双窗同一 `quality_window_samples`，默认 10，1–100）。
- 成员 cap ≥ 1：`min(N, cap)`。
- 无 cap：N。禁止用前端展示用 999。

挂钩：`resolvePairSlotAcquire` 在闭池命中且该配对处于考察时，返回上述 cap，且必须 `trackOccupancy=true`。调度 / 豁免期仍用成员原 cap（0 = 只计数不夹）。暂停 / 冷却不抢槽（准入已拒）。

## 进入 / 离开时窗口与宽限（已锁）

| 动作 | 冷却 | 两窗 | `u:`/`w:` | 随后 cap |
| --- | --- | --- | --- | --- |
| 进考察（到期，或手选含调度拉回） | 清 | **清零** | 不写（清掉若有） | 考察 cap |
| 考察毕业 → 调度 | 无 | **保留** | 无 | 成员原 cap |
| 考察 → 冷却 | `HSETNX` | 冷却后不入窗 | 清 | 拒 |
| 手选调度 | 清 | **清零** | 清（无 `w:`） | 成员原 cap |
| 手选豁免期 | 清 | **清零** | 现网 `MarkUserResume` | 成员原 cap |
| 豁免期满 → 调度 | 无 | **保留** | 自然过期 | 成员原 cap，然后评价 |
| 手选冷却 / 暂停 | 现网 | 不入窗 | 清 | 拒 |
| 离开暂停 | **无独立动作** | — | — | 由管理员点的下一态决定 |

手选其它态时顺带 `HDEL` 考察标记。

没有「取消暂停」行，也没有 UI/API 默认目的地。`SetPairAdmission` 从 `paused` 离开必须带显式 `state` ∈ {`probing`,`selectable`,`resumed`,`cooling`}（或再写回 `paused`）。不得把「只清 paused 旗标」做成进考察。省略 `state` 的现网解析仍是 `resumed`（豁免期写入默认，**不是**暂停解除默认，也 **不得** 改成从暂停默认 `probing`）。

## 毕业 vs 回冷（已锁）

在考察态，每次入窗后（以及选号读 live 时）用 \(Q_{a,u}\)：

**毕业（→ 调度，保留窗口）** 同时成立：

1. \(W_{ok}.len = N\)
2. 若配置了成功率门槛：成功率在门槛内
3. \(W_{ttft}\) 空或未满：不挡
4. \(W_{ttft}\) 已满：p50 必须过（否则走回冷，不是毕业）
5. 禁止用墙钟 / `u:`/`w:` 当毕业条件

**回冷**：

1. 现网 `pairQualityBlocks`（`EvaluateAccountQualityHardClose` + 策略门槛 + or/and + 未满不参加）为 true → 回冷。
2. **考察专则**：`and` 且 \(W_{ok}.len=N\) 且 \(W_{ttft}.len=N\) 且两指标一过一越界（上式为 false、毕业也不成立）→ 回冷。
3. \(W_{ok}<N\)：既不毕业也不回冷，留下等待。

调度态只用第 1 条，不用第 2 条。

豁免期内不走毕业、不走回冷。期满后该配对已是调度，用调度规则评价（可立刻回冷）。

## 持久化形状（技术，非产品重开）

现网可写态拆在三处：`paused` SQL 旗标、冷却 HASH、`account-quality:resume` 的 `u:`/`w:`。考察是非时间态，需要独立标记。

建议（可在实现前微调，不得改合同）：

- Redis HASH `smart-schedule:probe:{accountID}` field `u:{userID}` = 进入 unix（或 `1`）。无毕业 TTL。
- 进入：`HSET` 标记 + `ClearCooldown` + `ZeroPairQuality` + `ClearUserResume`。
- 毕业：`HDEL` 标记，**不** `ZeroPairQuality`。
- 回冷 / 手选其它态：`HDEL` 标记，再走该态现网副作用。
- 冷却到期：改 `expirePairCooldown`：现有 `HDEL` + `ZeroPairQuality` 之后 **写入考察标记**，不再落到「无标记 = 调度」。
- 读：`IsProbing`；热路径 cap 与 UI 水合都读它。
- Redis miss / 无标记 = 非考察。与 Q4「已在调度的不回填」一致。
- **不要**在清 `paused` 时自动 `HSET` 考察标记。

不要用 `w:` 冒充考察。不要把考察写进 `IsSchedulable()`。

`ParsePairAdmissionState` / 管理端 `state` 增加 `probing`。非法值仍 `SMART_SCHEDULE_ADMISSION_INVALID`。省略 `state` 仍是 `resumed`（旧 resume 入口）。**禁止**「从暂停且未带显式其它态 → `probing`」。

事件类型建议：`probe_enter` / `probe_graduate` / 现有 `cooldown_*` / `resumed` / `selectable` / `expiry_zero`（到期清窗可与 `probe_enter` 同一次或紧挨着）。详情列表给前端。不要发明 `unpause` / `pause_lifted` 自动事件。

## 数据流

```text
冷却 HASH 到期
  → expirePairCooldown: HDEL 冷却 + ZERO 两窗 + 写 probing + 不写 u:/w:

选号 admitsScheduleUser
  → paused / 冷却未到期 → 拒
  → 豁免期 u:/w: → 放行（不评价、不毕业）
  → 否则读 Q_{a,u}
       若 probing: 可毕业则清标记保留窗；可回冷（含 and 混窗）则 HSETNX
       若 调度: 现网只回冷、不毕业、不用 and 混窗专则

抢槽 resolvePairSlotAcquire
  → 闭池成员 + probing → min(N, cap) 或 N
  → 否则现网 PairCap

完成 ObservePairCompletion
  → paused / 冷却 → 丢弃
  → 否则入窗
  → 豁免期 → 结束
  → probing → 毕业或回冷（含 and 混窗专则）；W_ok<N 则留下
  → 调度 → 仅回冷（标准 or/and）

SetPairAdmission
  → 目标态显式；paused 只写长期暂停，不排队「解除后进考察」
  → 从任意态手选 probing / selectable / resumed / cooling / paused 均可
```

## 前端

- `PoolAdmissionState` / `PairAdmissionLiveState` / `PAIR_ADMISSION_LIVE_STATES` / 过滤器增加 `probing`。
- 切换器五项：暂停、冷却、考察、调度、豁免期。调度态也列出考察。文案 zh+en。
- 从暂停切走：必须再点下一态。**没有**「解除暂停」按钮。**不得**暗默认 `probing` 或 `selectable`。
- 考察行占用徽章分母 = 考察 cap，不是 999、不是「未设 cap 就当无限」。
- `will_cool` 仍只看配对窗 + 已存门槛；考察中同样可显示将回冷，但真锁仍是冷却 HASH。
- 不复用账号质量格。不改客户端调用协议。

## 兼容 / 回滚 / 上线

- 旧二进制不认 `probing` 标记：按无标记 = 调度（满额）。回滚即失去夹并发。
- GET 多一个 `probing` / 池行状态；旧前端不认识时不要让整页挂掉（未知态当调度展示可接受，但新前端必须认）。
- **不回填**：部署后已在调度的配对没有标记，继续调度。只有之后新到期的冷却走考察。
- 不改客户端、计费、`IsSchedulable()`、账号 last-N。

## 风险文件

- `account_user_schedule.go` `admitsScheduleUser`
- `account_user_concurrency.go` `resolvePairSlotAcquire`
- `user_smart_schedule_service.go` `SetPairAdmission` / `ObservePairCompletion` / `ParsePairAdmissionState`
- `user_smart_schedule_cache.go` `expirePairCooldown`
- `smartSchedulePoolAdmission.ts` / `SmartScheduleAdmissionSwitch.vue` / `UserSmartScheduleView.vue` / zh.ts+en.ts
- `.trellis/spec/backend/account-user-schedule.md`（现文「到期同可调度」须改成到期进考察）
