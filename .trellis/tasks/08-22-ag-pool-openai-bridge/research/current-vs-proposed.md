# Research: AG 智能调度池 vs 桥接热路径（现状）

Date: 2026-08-22. Planning only. No SSH this pass.

## Confirmed code facts

### Write path: OpenAI 进不了 AG 池

`sanitizePoolMembers` (`backend/internal/service/user_smart_schedule_service.go:555-557`) 要求 `normalizeSmartSchedulePlatform(acc.Platform) == platform`（tab 平台）。不满足则 `SMART_SCHEDULE_PLATFORM_MISMATCH`。**不读** `openai_claude_gpt_bridge_enabled`。

前端 `loadCandidates` (`useUserSmartScheduleEditor.ts:785`) 用 `{ platform: activePlatform, lite: '1' }`。AG tab 只拉 `platform=antigravity`。筛选添加对话框 `hide-platform`，文案写死「平台已锁定为 {platform}」。

`account.md` / spec `account-user-schedule.md` 写明：成员 `account.platform` 必须等于行 `platform`；mixed-scheduling 的 AG 号仍留在 antigravity tab。这是**写路径**约束，不是桥接资格。

### PK：一个号只能进一个 tab

`user_smart_schedule_accounts` PK `(account_id, user_id)`（migration 202；Ent `field.ID("account_id", "user_id")`）。`platform` 只是冗余列 + `(user_id, platform)` 索引。

后果：同一 OpenAI 号不能同时出现在 openai 池和 antigravity 池。`SetMemberPaused` SQL 也只按 `(user_id, account_id)` 更新，没有 platform。

当前 `main` 最大 migration 号是 **210**。若改主键，新 SQL 必须从 **211** 起，禁止改 202。

### 热路径：策略 key 是 `account.Platform`，不是 group platform

所有智能调度查找都走 `lookupEnabledSmartPolicy(..., account.Platform)`：

| 调用点 | 文件 | 桥接时实际 platform |
|---|---|---|
| 准入 | `account_user_schedule.go:235` | openai |
| pair 槽 / 占用 | `account_user_concurrency.go:38` | openai |
| 未入池 cheaper-tier 逃逸 | `account_unpooled_schedule.go:92,125` | sticky / account.Platform = openai |

桥接选号 `SelectAccountWithSchedulerForClaudeGPTBridge` **不传** `platformOverride`，内部默认 `PlatformOpenAI`（`openai_account_scheduler.go:1806-1848`）。候选过滤 `isOpenAIAccountEligibleForScheduleRequest` 要求 `account.Platform == openai`，桥接另要求 `ResolveClaudeGPTBridgeModel`（必须 `extra.openai_claude_gpt_bridge_enabled` + 账号级 mapping 命中）。

因此：分组 15（antigravity、几乎全是桥接）与分组 19（原生 GPT）共享用户 12 的 **openai 闭池**。用户 12 没有 antigravity 策略。

### 桥接资格（调度，不是入池）

`ResolveClaudeGPTBridgeModel`（`account.go:1144-1157`）：

1. `IsOpenAIClaudeGPTBridgeEnabled()`（openai + extra bool）
2. 账号级 `model_mapping` 命中（`ModelMappingSourceAccount`）
3. 映射非空且 ≠ 请求 Claude 模型

入池放宽 **不得** 放宽这条。`schedulable=true` 且在 AG 池里、但桥关着的 OpenAI 号：原生 AG 路径不会选它（平台不符）；桥接路径也不会选它（Resolve 失败）。

### Redis / 配对质量不是按 platform 分片

- pair 槽：`concurrency:account_user:{accountID}:{userID}`
- 冷却 / probe / pin / pair-quality：HASH 按 account，field `u:{userID}`

双池共存时，同一号的占用、冷却、考察、钉选会在两个 tab **共享**。`ObservePairCompletion` 遍历 `bundle.Policies` 找第一个 `HasAccount`，双池时会命中不确定的那条策略。这是决策 A 的隐藏成本。

### 池表已有 `platform_type`

`UserSmartScheduleView.vue` `allPoolColumns` 已有 `platform_type`。新列应是桥接开关，不是再做一个平台列。Lite 列表保留 `extra`（只剥 credentials / schedule_users），所以 `extra.openai_claude_gpt_bridge_enabled` 已在候选/池账号 DTO 上，不必为展示单独落库。

## Production evidence (already gathered)

- User `gybilly2023@gmail.com` = users.id 12
- Group 15 `antigravity billy`：几乎全是 Claude-GPT 桥；原生 AG 号 212 为 error
- User 12：openai 智能调度 ENABLED（p50 10s, success 0.85, n=10, cooldown 3m）；无 antigravity 策略
- 48h：807 ops；约 134 桥接启发式；389 条分组 19 `/v1/responses` 路由 503；41 条空流 502 在账号 1724 `tokenbits 012`

## Proposed overlay (pending A–G)

写路径：AG tab 允许 `account.Platform=openai` 成员，**不**要求桥开关。

读路径：桥接 / AG 分组请求的智能调度 lookup 改为 **请求侧 platform=antigravity**（与现有 `account.Platform` 冲突，见 design）。原生 GPT 分组 19 仍 lookup openai。

调度资格：保持 `ResolveClaudeGPTBridgeModel`。
