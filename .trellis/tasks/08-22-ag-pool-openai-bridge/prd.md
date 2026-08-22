# Antigravity 智能调度池允许 OpenAI 账号

## Goal

管理员能把 OpenAI 账号加入某用户的 **antigravity** 智能调度池，让 Antigravity 分组上的 Claude-GPT 桥接走独立闭池，而不再被迫把桥接号塞进该用户的 openai 池（与原生 GPT 分组抢同一闭池）。

入池不要求桥开关。桥接请求能不能打到这个号，仍走现有 `openai_claude_gpt_bridge_enabled` + 账号级 mapping。池表多一列只读展示该开关。

openai 池与 antigravity 池在 **AG 闭池开启时完全独立**：同一 OpenAI 账号可以同时在两池，但占用、冷却、probe、pin、pair-quality、pair cap、ObservePairCompletion 都按 **(user, account, platform)** 隔离。AG 关着时这些请求仍走 openai 分片（与今天一致）。AG 开着时不要回落混用。

## User value

生产用户 12（`gybilly2023@gmail.com`）分组 15 `antigravity billy` 几乎全是桥接，但只有 openai 策略、没有 AG 策略。桥接热路径按 `account.Platform=openai` 查闭池，于是分组 15 与分组 19 原生 GPT 共用同一 openai 闭池。修完后：AG 池可加 OAI 号；**AG 开关关着时 lookup 仍是 openai（对用户 12 是 no-op）**；开关开且有成员时桥接/AG 分组只查 AG 池（决策 C）。AG 开着时两池 Redis 状态互不影响。

## Background

写路径：`sanitizePoolMembers` 要求 `account.Platform == tab`，AG tab 加 OpenAI 号会 `SMART_SCHEDULE_PLATFORM_MISMATCH`。前端候选列表 `platform=activePlatform`，筛选添加锁定当前 tab。

主键（落地前）：`user_smart_schedule_accounts (account_id, user_id)`，一个号只能进一个 tab。落地后改为 `(user_id, platform, account_id)`。

热路径：`lookupEnabledSmartPolicy(..., account.Platform)`。`SelectAccountWithSchedulerForClaudeGPTBridge` 默认 `PlatformOpenAI`。准入、pair 槽、未入池 cheaper-tier 逃逸都用 `account.Platform`。

桥接资格（不变）：`ResolveClaudeGPTBridgeModel` 要求 extra 桥开关 + 账号级 mapping 命中且映射 ≠ 请求模型。

空流：48h 有 41 条 502 在已选中的账号 1724。与「加不进池」不是同一根因。本任务不修空流（决策 F）。

## Requirements

- R1. AG 智能调度 tab 的 PUT / 筛选添加 / typeahead 允许 `platform=openai` 账号。**不要求** `openai_claude_gpt_bridge_enabled`。未开桥的 OAI 号也可以进 AG 池。
- R2. AG 池表格增加一列，只读展示该号当前 `extra.openai_claude_gpt_bridge_enabled`（来自账号 extra，不是成员行可写副本）。
- R3. 调度资格保持现状：即使号在 AG 池且 `schedulable=true`，桥关着则桥接请求仍不能打到该号。原生 AG 号不受此列影响。
- R4. AG 智能调度开关表示「用不用 AG 闭池」。Lookup、准入、pair 槽、未入池判定、ObservePairCompletion、hydrate 必须同一 helper。**AG 策略 nil / disabled / 空池：** 桥接或 AG 分组上的 OpenAI 号继续查 **openai**（今天的生产路径）；关开关上线对用户 12 是 no-op。**禁止**因为 AG 关了就 fail-open 到账号侧。**AG 策略已启用且有成员：** 这些请求只查 **antigravity**；池未命中即拒；**禁止**再回落 openai 池。独立性只在 AG 开着时成立。
- R5. 原生 OpenAI 分组（如分组 19）继续用该用户的 openai 策略 + `account.Platform=openai`。不得把原生 GPT 选号改成查 AG 池。
- R6. 非 AG tab 的平台匹配不变：openai tab 仍只能加 OpenAI 号；anthropic / gemini / grok tab 仍拒绝跨平台。
- R7. 不改存储计费、`actual_cost`、展示 token、cache-read 上限。
- R8. 网关 524 / 空流相关验收不得要求客户端改协议、`stream`、端点或 body。
- R9. 新 UI 文案中英同步。
- R10. 新 SQL 从当时 `main` max+1 起（当前 max=210 → **211**）。禁止改历史 migration，禁止复用已占用号。
- R11. AG 开着时 openai 池与 AG 池 Redis 状态完全独立。占用 / 冷却 / probe / pin / pair-quality / pair cap 按 `(user, account, platform)` 隔离。AG 关着时桥接/AG 分组走 openai 分片。不要回落混用。

## Acceptance Criteria

- [x] AC1. 管理员在用户智能调度 **antigravity** tab 把一个**未开桥**的 OpenAI 号加入池并 Save，返回 200，不再 `SMART_SCHEDULE_PLATFORM_MISMATCH`。
- [x] AC2. 同一操作对已开桥的 OpenAI 号同样成功。
- [x] AC3. AG 池表能看到该号桥开关开/关（只读）。关桥后刷新列变关，不必改池成员。
- [x] AC4. 桥关着的 OAI 号在 AG 池且 `schedulable=true` 时，分组 15 类桥接请求仍不能选中该号（`ResolveClaudeGPTBridgeModel` 失败）。原生 AG 号选号不受该列影响。
- [x] AC5. 用户已启用 AG 策略且池内有合格桥接号时，分组 15 桥接请求按 AG 闭池准入（池外 OAI 号拒），不再要求该号也在 openai 池里。
- [x] AC6. 同一用户的原生 OpenAI 分组请求仍只吃 openai 闭池；某号同时在 AG 池不会让分组 19 被 AG 策略误伤。
- [x] AC7. AG tab 候选/筛选添加能列出 OpenAI 号（含未开桥），且能与原生 AG 号区分（平台 + 桥接列）。
- [x] AC8. 单测覆盖：sanitize 允许 OAI→AG；sanitize 仍拒 OAI→anthropic；桥关号在池但不被桥接选中；lookup 按新决策 C（AG 关/空/缺失走 openai，AG 开只走 AG）；双池 persist；两池冷却/占用隔离；pair/cooldown/resume 与准入同一 lookup 平台。
- [x] AC9. 本任务无空流代码改动；现有空流单测期望不变。
- [x] AC10. 同一 OpenAI 号在 openai 池冷却，不影响该号在 AG 池的占用/冷却/probe/pin/pair-quality；反之亦然。

## Out of scope

- 空流 / 断流修复（F=否）。
- 改 `ResolveClaudeGPTBridgeModel`、桥接预检状态机、计费/展示。
- 让未开桥的 OAI 号服务桥接请求。
- 把 OpenAI 号变成 `platform=antigravity`。
- 客户端改 `stream` / 端点 / body。
- `git push` / 生产部署。
- 独立风险审查（实现测完即停，留给下一步）。

## Decision table（已拍板 2026-08-22）

Brandon：**按推荐**锁定 A–G，并覆盖 design 原「Redis 仍按 (account,user) 共享」拟议 → **完全独立**。

| ID | 决策 | 锁定 | 说明 |
|---|---|---|---|
| **A** | 双池主键 | **`(user_id, platform, account_id)`** | 允许同一 OAI 号同时在 openai 池和 AG 池。新 SQL **211**（main max=210）。禁止改 202/204/207/208。 |
| **B** | 已在 openai 池的桥接号加入 AG | **共存** | 加入 AG **不**删除 openai 池行。 |
| **C** | 热路径策略平台 | **AG 开且有成员才查 antigravity** | AG 关/空/缺失：桥接/AG 分组 OAI 仍查 openai（关开关 = 今天生产路径）。AG 开：只查 AG 闭池，池未命中即拒，不回落 openai。原生 GPT 分组始终 openai。 |
| **D** | 池列语义 | **只读 extra** | `extra.openai_claude_gpt_bridge_enabled`。不写 PUT body。 |
| **E** | AG tab 候选 | **全部 OAI（含未开桥）+ 原生 AG** | 其它 tab 仍锁当前平台。 |
| **F** | 空流是否纳入 | **否** | 不改 converter / empty-stream。 |
| **G** | 迁移编号 | **`211_*.sql` additive** | 禁止改历史 migration。 |
| **Redis** | 两池隔离 | **`(user, account, platform)`** | 覆盖原「共享占用与冷却」拟议。platform 进 Redis key，避免共用 HASH TTL。 |

## 空流（记录，非目标）

现象与历史修复见 `research/empty-stream-related.md`。仍开放：是否另开子任务。验收禁止逼客户端改 stream。

## Notes

- 复杂任务。决策 A–G 已确认；实现写在占用中 `main` checkout，只改本任务文件，不碰无关 dirty 文件。
- Phase 3 wrap（2026-08-22）：Brandon 批准 spec 收尾 + 本任务 commit。不 push / 不 deploy。生产切流前用户 12 必须先有 AG 池；发版后旧 Redis key 靠 TTL 消失，在飞占用/冷却会重置。
- 证据与对照：`research/current-vs-proposed.md`。
