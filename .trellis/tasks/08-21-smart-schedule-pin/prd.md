# 智能调度长期豁免

## Goal

给智能调度配对入池增加第六态 **长期豁免**：界面「长期豁免」，API **`pinned`**（禁止复用 `resumed`）。管理员手选后，该用户×账号在完整成员 cap 下持续准入，**永不评价、永不 `StartCooldown`**，直到管理员再选手选下一态。无隐式超时。

## Background

现网五态：`paused` / `cooling` / `probing` / `selectable` / `resumed`。`resumed`（豁免期）是短时间 fail-open（`u:` 15m + `w:` 至 30m），期满进调度并开始评价。管理员需要一条**无期限**的豁免，不能靠反复点豁免期，也不能把 `resumed` 改成永不过期。

本任务 **新建** 第六态。不回填已有配对。不改客户端协议。账号硬关闭 / `IsSchedulable()` 仍是整号门闩，本态不覆盖。

## Confirmed Facts

1. 入池开关 POST `/admin/accounts/:id/smart-schedule-resume` `{user_id, state?}`。省略 `state` **必须仍是 `resumed`**，不是 `pinned`。
2. `resumed` 写 `account-quality:resume` 的 `u:`/`w:`。`pinned` **不得**写这些字段，也不得 `MarkUserResume`。
3. 考察 mark：Redis HASH `smart-schedule:probe:{accountID}` field `u:{userID}`，无 TTL。`pinned` 用同形：`smart-schedule:pinned:{accountID}` field `u:{userID}`，**无 TTL**。Miss = 未钉住（不上线回填）。
4. 冷却到期现网只进 `probing`，永不进 `resumed`。本任务保持：**冷却到期仍进考察，永不进 `pinned`**。
5. `paused` 无自动出口。离开暂停必须手选下一态；清 `paused` 不得暗进 `pinned`。
6. 热路径 `admitsScheduleUser` / `ObservePairCompletion` 已有 probing / 豁免期分流。`pinned` 在暂停之后、冷却/评价之前短路：准入（仍受暂停 / 账号不可调度 / 配对 cap），跳过评价与 `StartCooldown`。
7. 配对窗在钉住期间可以继续 ingest；不得因此触发冷却。

## Requirements

### 状态表（界面 / API）

| 界面 | API | 谁能进入 | Cap | 评价 / 冷却 |
| --- | --- | --- | --- | --- |
| 豁免期 | `resumed` | 永不自动 | 成员原 cap | fail-open `u:` 15m + `w:` 30m，期满后调度 |
| **长期豁免** | **`pinned`** | **仅手动** | 成员原 cap | **永不评价、永不 StartCooldown，直到管理员离开** |

其它四态行为不变。禁止把 `pinned` 做成 `resumed` 的别名或无 TTL 的 `u:`/`w:`。

### 进入 `pinned`（仅显式 `state=pinned`）

- 清冷却、清考察 mark。
- **不写** `u:`/`w:`。
- **不** `MarkUserResume`。
- 不要把两窗清零（窗可继续 ingest）。
- 清 leftover `u:`/`w:` 以免和短期豁免期芯片叠在一起（清 ≠ 写）。
- GET 水合 `pinned: true`。Redis miss = 未钉住。

### 离开 `pinned`

- 管理员必须再选下一态：`paused` / `cooling` / `probing` / `selectable` / `resumed`。
- **无隐式超时**。冷却到期路径不得把配对写成 `pinned`。
- 离开后走该态既有副作用（例如 `selectable` 清窗后可再冷却）。

### 热路径

- `IsSchedulable()` / 账号硬关闭仍先于本态（整号门闩）。
- `admitsScheduleUser`：已暂停则拒绝；已 `pinned` 则准入并跳过评价；再检查冷却。
- `ObservePairCompletion`：暂停不 ingest；`pinned` 要 ingest、不评价、不 `StartCooldown`；冷却仍不 ingest。
- `resolvePairSlotAcquire`：`pinned` 用成员原 cap，不夹考察 cap。
- 不上线回填。

### API / UI

- `ParsePairAdmissionState` 接受 `pinned`。空串仍是 `resumed`。非法值 `SMART_SCHEDULE_ADMISSION_INVALID`。
- 开关菜单第六项：长期豁免。
- 省略 PUT/POST `state` ≠ `pinned`。
- 暂停不得自动变成 `pinned`。

## Acceptance Criteria

- [ ] 显式 `state=pinned` 进入长期豁免：清冷却、清考察、不写 `u:`/`w:`、不 `MarkUserResume`、GET `pinned: true`。
- [ ] 省略 `state` 仍是 `resumed`，不是 `pinned`。
- [ ] 冷却到期仍进 `probing`，永不 `pinned`。
- [ ] 钉住期间 N 次成功（或越界窗）不 `StartCooldown`；窗可 ingest。
- [ ] 离开到 `selectable` 后，满 N 越界可再次冷却。
- [ ] 暂停不会自动变成 `pinned`。
- [ ] 账号 `IsSchedulable()==false` 时整号仍不可选（本态不覆盖）。
- [ ] 已有配对不上线回填。
- [ ] 已写 `research/pin-api-contract.md`；已更新 `.trellis/spec/backend/account-user-schedule.md` 与 `docs/dev/CHANGELOG_CUSTOM.md`。

## Out of Scope

- 回填历史配对。
- 改 `resumed` 时长或把 `u:`/`w:` 改成永不过期。
- 账号质量硬关闭 / last-N 轨。
- 客户端改协议 / `stream` / 换端点。
- git commit / push / 生产部署。

## Technical Notes

- Redis：`smart-schedule:pinned:{accountID}` HASH `u:{userID}`，无 TTL。
- 热路径读 `IsPinned`；admin 写 `MarkPinned` / `ClearPinned`；GET `IsPinnedBatch`。
- 进入其它态时清 pin mark（与清 probe mark 对称）。
