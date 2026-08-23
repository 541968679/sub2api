# Design: OpenAI 长上下文计费开关

## Architecture

```
Admin Settings Toggle
  → PUT /api/v1/admin/settings  (admin-only DTO)
  → SettingService.UpdateSettings
  → settings KV: openai_long_context_billing_enabled = "true"|"false"
  → refreshCachedSettings (in-process 60s cache, save 时立即刷新)

Billing hot path
  CalculateCost / CalculateCostUnified
    → computeTokenBreakdown(applyLongCtx)
      → applyLongCtx && shouldApplySessionLongContextPricing
          1. isOpenAILongContextBillingEnabled()  // NEW gate, default ON
          2. pricing threshold / multipliers
          3. input+cacheRead > threshold
```

闸门放在 `shouldApplySessionLongContextPricing`，而不是某个 gateway wrapper：

| 路径 | 现有 `applyLongCtx` | 新开关 |
|------|---------------------|--------|
| `calculateCostInternal` (fallback) | 恒为 true | 必须读开关 |
| `CalculateCostUnified` 无区间 | true | 必须读开关 |
| `CalculateCostUnified` 有区间 | false | 不需要再判；保持区间优先 |

`applyModelSpecificPricingPolicy` 仍可补齐模型定价元数据（272000 / 2.0 / 1.5）。开关只控制**是否应用**，不删除定价字段。展示变换只看 usage snapshot 的 `long_context_applied`，关开关后不会再乘展示单价。

## Data / contracts

| Layer | Field | Notes |
|-------|--------|--------|
| KV | `openai_long_context_billing_enabled` | 字符串 `"true"` / `"false"` |
| `SystemSettings` | `OpenAILongContextBillingEnabled bool` | parse: `!= "false"` |
| Admin DTO GET | `openai_long_context_billing_enabled` bool | 默认 true |
| Admin DTO PUT | `*bool` | nil = 保持原值 |
| Public settings | **absent** | 管理表单走 `GetAllSettings` |
| Usage snapshot | 不变 | 关：`applied=false`，倍率字段不写 |

Parse（与 `codex_compact_v2_fallback_enabled` 对齐）：

```
trim(raw) != "false"  → enabled
missing / "" / "true" / "1" / "yes" / garbage → enabled
```

## SettingService read

Billing 热路径禁止每次打 DB。复用 Backend Mode 式进程内 cache：

- `atomic.Value` + `singleflight`，TTL 60s，DB 错误 5s 后重试
- `GetValue` not found / error → **true**（fail-open，保持现网溢价）
- `UpdateSettings` → `refreshCachedSettings` 立刻写入新值
- `SettingService == nil` 或 `BillingService.settingService == nil` → true

BillingService 构造保持 `NewBillingService(cfg, pricing)` 两参，测试无感。Wire 用 `ProvideBillingService` 注入 SettingService。

## Admin UI

Features 页新增一张卡（计费，不是 Display 展示层）：

- Toggle 绑 `form.openai_long_context_billing_enabled`
- 默认 true
- loadSettings 已按 key 回填；save payload 显式带上该字段
- i18n：`admin.settings.features.openaiLongContextBilling.*`

不要放进 Display tab：那个 tab 只改展示变换，本开关改 **stored billing**。

## Compatibility

- 现网无此 KV：视为 on，扣费不变。
- 渠道区间定价：`applyLongCtx=false` 仍短路，不叠会话级溢价。
- Gemini `CalculateCostWithLongContext`：不读此开关。
- 展示层：只消费 snapshot；关开关后新日志不再带 applied 快照，展示单价不再乘 2.0/1.5。
- 不改 `actual_cost` 恒等式以外的东西；关开关时 stored `actual_cost` 不再含长上下文溢价，这是产品意图。

## Rollback

Admin 把开关拨回 ON，或删除 KV。无需 migration。无需重启（cache 在 save 时刷新；最坏 60s）。

## Follow-up (not this slice)

阈值 / 倍率编辑器。若做，仍走同一 KV 族，且必须继续尊重「区间定价禁用会话级长上下文」。
