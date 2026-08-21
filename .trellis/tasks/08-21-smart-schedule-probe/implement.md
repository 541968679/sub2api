# 实现清单：智能调度考察期

用户已说开始实现。`task.py start` 已把本任务翻到 `in_progress`。本文件是后端实现清单。

## `start` 前

- [x] Q1 已决并更正：`paused` 长期手选；无「暂停解除」自动链；无默认下一态；离开必须显式选 考察 / 调度 / 豁免期 / 冷却
- [x] Q2 考察 `and` 两窗满、一好一坏 → 回冷
- [x] Q3 允许调度→手选考察（不经冷却、清零、夹紧）
- [x] Q4 上线不回填；已调度维持调度
- [x] `implement.jsonl` / `check.jsonl` 指向本任务 research + 只读的已交付配对合同
- [x] 用户明确说开始实现后再 `task.py start`

## 顺序

1. [x] **状态解析**：`PairAdmissionProbing`、`ParsePairAdmissionState` 接受 `probing`。省略仍 `resumed`（旧 resume 入口）。**禁止**「从暂停且未显式指定其它态 → `probing`」。非法值仍 `SMART_SCHEDULE_ADMISSION_INVALID`。单测：空/显式五态/非法；从暂停省略不得进考察。不要加 `unpause` 状态名。
2. [x] **Redis 考察标记**：`IsProbing` / 进入 / 清除。`expirePairCooldown` 改为到期进考察（清冷却、清窗、写标记、不写 `u:`/`w:`）。**禁止**部署扫描回填。**禁止**在「只清 paused」路径写 probing。单测：到期不是调度、无宽限、窗为空；无标记读作非考察。
3. [x] **`SetPairAdmission`**：手选考察（含从调度拉回、从暂停**显式**手选考察）按合同清冷却、清窗、清宽限、写标记、夹紧。手选调度 / 豁免期 / 冷却 / 暂停顺带清考察标记。手选调度清窗且无 `w:`；手选豁免期清窗 + 现网 `MarkUserResume`。手选暂停只写长期暂停，**不**登记「解除后默认考察」。从暂停离开必须带显式目标态。
4. [x] **热路径 cap**：`resolvePairSlotAcquire` 在 probing 时用 `min(N, cap)` 或 N。单测：有 cap、无 cap、N>cap、非考察仍原 cap。
5. [x] **毕业 / 回冷**：`ObservePairCompletion` + `admitsScheduleUser`。毕业保留窗、清标记。回冷先走现网 `pairQualityBlocks`；考察再加 `and` 混窗专则。单测：\(W_{ok}<N\) 留下；\(W_{ttft}\) 空/未满不挡毕业；\(W_{ttft}\) 满且 p50 不过走回冷；`and` 两窗满一好一坏走回冷；调度态同一混窗 **不** 因专则回冷。
6. [x] **豁免期满**：不进考察；保留窗；按调度评价。单测与现网期满可立刻冷对齐，并断言无 probing 标记。
7. [x] **前端**：状态机 + 五态切换器（任意态可跳，调度态也可选考察）+ i18n + 考察 cap 徽章 + 过滤。无「解除暂停」按钮、无暗默认 probing。`will_cool` 仍读配对窗。GET 读 `probing` + `probe_cap`。
8. [x] **规格**：`.trellis/spec/backend/account-user-schedule.md` 把「到期同可调度」改成到期进考察，并补五态 / 反死锁 / 暂停无自动出口。`docs/dev/CHANGELOG_CUSTOM.md`。

不要改：客户端协议、账号 last-N / 硬关闭、`08-21-smart-schedule-recovery-probe` 目录、同步假 TTFT。不要回填已调度配对。不要实现「取消暂停 → 默认考察」。

## 校验

```text
go test -tags=unit ./internal/service -count=1
go test -tags=unit ./internal/repository -count=1
pnpm --dir frontend exec vitest run src/composables/__tests__/smartSchedulePoolAdmission.spec.ts src/views/admin/__tests__/UserSmartScheduleView.spec.ts
```

补：到期进考察；清暂停不写 probing；从暂停离开须显式目标态；调度手选拉回考察；考察 cap；\(W_{ok}<N\) 不毕业；\(W_{ttft}\) 未满不挡毕业；`and` 混窗回冷；手选调度清窗无 `w:`；手选豁免期满留窗进调度；无标记不回填；跨用户不串；账号 15m/last-N 单测不回归。

## 回滚

回旧二进制；Redis 考察标记被忽略，配对按调度满额。可留脏 key。不需要回填脚本。
