# OpenAI 长上下文计费开关

## Goal

给管理员一个 Settings KV 开关，用来关闭 OpenAI GPT-5.4 / 5.5 / 5.6 族的**会话级**长上下文溢价。默认开启，与现网行为一致。本切片只做开关，不做阈值/倍率编辑器。

## User value

运营可以在不改客户端、不改渠道区间定价、不改 Gemini 超额双倍的前提下，关掉 OpenAI 会话级 272000 / 2.0 / 1.5 溢价。关掉后按基础单价扣费，usage snapshot 也不再标记长上下文已应用。

## Background

- 现网：`InputTokens + CacheReadTokens > 272000`（严格大于）时，整段会话 input / cache read / cache write × 2.0，output × 1.5。
- 实现：`billing_service.go` 的 `shouldApplySessionLongContextPricing`、`computeTokenBreakdown`、`applyModelSpecificPricingPolicy`。
- OpenAI 网关把 `CostBreakdown` 快照写进 usage log（`long_context_applied` + 阈值/倍率）。
- 渠道区间定价会把 `applyLongCtx=false`，因此区间通道本身不会叠会话级长上下文。本开关不得破坏该副作用。
- 管理后台目前无法关闭该溢价。
- Gemini 200K 超额双倍（`RecordUsageWithLongContext` / `gemini_v1beta_handler.go`）不在范围。

## Locked decisions

1. 占用 checkout `E:\cursor project\api2sub` 写产品代码。禁止为这个任务开隔离 worktree。
2. Settings key：`openai_long_context_billing_enabled`，默认 **true**。
3. 缺失 / 非法值一律视为 **enabled**（与 `codex_compact_v2_fallback_enabled` 的 `!= "false"` 一致）。
4. 闸门必须盖住 **unified + fallback** 全部 OpenAI token 计费路径，最佳位置靠近 `shouldApplySessionLongContextPricing` / `computeTokenBreakdown`。不要只包一层网关 wrapper。
5. BillingService 通过注入的 SettingService（或等价缓存读）读开关。
6. 管理后台一个 Toggle，中英 i18n。
7. **不要**暴露到 `GET /api/v1/settings/public`，除非管理表单加载依赖它。本仓管理设置走 admin `GetAllSettings`，因此保持 admin-only。
8. 关：无长上下文倍率，`long_context_applied=false`，不把倍率写成已应用快照。开：272000 / 2.0 / 1.5 不变。
9. 不做阈值/倍率编辑器。不改展示 token 算法，不发明 display workaround。
10. 不 commit / push / 生产 SSH / 部署。不混入已有 pool-sticky 脏文件。

## Requirements

- R1. Settings KV `openai_long_context_billing_enabled`，默认 true；缺失/非法 = enabled。
- R2. `shouldApplySessionLongContextPricing`（或同等热路径）在开关关闭时返回 false，使 `CalculateCost` 与 `CalculateCostUnified` 都不再加会话级倍率。
- R3. 开关关闭时：`CostBreakdown.LongContextApplied=false`，threshold/multiplier 快照字段保持零值（现有 `computeTokenBreakdown` 只在 applied 时写入）。
- R4. 开关开启或 SettingService 为 nil：现有 GPT-5.4/5.5/5.6 测试与 272000 边界行为不变。
- R5. 渠道区间定价仍使 `applyLongCtx=false`，不叠会话级长上下文。
- R6. Admin Settings 一个 Toggle + zh/en。不进 public settings。
- R7. Gemini `CalculateCostWithLongContext` 超额双倍路径不改。
- R8. 更新 `docs/dev/codebase/billing.md` 长上下文不变量，并追加 `docs/dev/CHANGELOG_CUSTOM.md`。

## Acceptance Criteria

- [ ] AC1. 无 KV / 非法值 / SettingService=nil：`gpt-5.4` 且 input+cacheRead=272001 仍应用 2.0/1.5，`long_context_applied=true`。映射 R1、R4。
- [ ] AC2. KV=`false`：同样 token 不计溢价，`long_context_applied=false`，快照倍率不写。映射 R2、R3。
- [ ] AC3. KV=`true`：与 AC1 相同溢价。映射 R4。
- [ ] AC4. 渠道区间定价 + 超阈值：仍 `long_context_applied=false`，费用走区间价。映射 R5。
- [ ] AC5. 既有 `TestCalculateCost_OpenAIGPT54LongContext*` 在默认开启下继续通过。映射 R4。
- [ ] AC6. Admin GET/PUT 能读写该字段；`GET /api/v1/settings/public` 不含该 key。映射 R6。
- [ ] AC7. 管理后台 Features 页有 Toggle，中英 copy 说明默认开启与 272000 行为。映射 R6。

## Out of scope

- 阈值 / 倍率数字编辑器（后续切片）
- Gemini 200K excess-only doubling
- 展示 token / `actual_cost` 恒等式改造
- 客户端改 `stream` / 端点 / body
- commit / push / 生产部署
- 上游同步 worktree 规则

## Open questions

无。Brandon 已锁定本切片范围。
