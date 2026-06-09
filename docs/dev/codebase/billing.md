# 计费系统 (Billing & Pricing)

> 模型定价解析、费用计算、费率乘数、展示变换。
> 五级定价链（User → Channel → Global → LiteLLM → 内置兜底）+ 两条独立费率乘数（用户扣费 vs 账号 quota）+ 展示变换体系。

## OpenAI Claude-GPT Bridge Billing

Antigravity `/v1/messages` requests served by the OpenAI Claude-GPT bridge use
the original Claude request model as the user-facing and billing model. The GPT
upstream model is stored only as `upstream_model` for admin visibility.

Usage and billing rules:

- `usage_logs.model` and `usage_logs.requested_model` are the original Claude
  request model, for example `claude-opus-4-8`.
- `usage_logs.upstream_model` stores the OpenAI upstream model, for example
  `gpt-5.5`, only when it differs from the display model.
- Pricing lookup uses `BillingModelSourceRequested`, so Claude pricing and the
  Antigravity group/user rate chain apply even though the upstream account is
  OpenAI.
- Token counts come from the OpenAI upstream response after the existing
  Anthropic response conversion path. The bridge does not derive token counts
  from the GPT model name.
- Bridge mode preserves the body-level `prompt_cache_key` for OpenAI upstream
  requests, keeping the request body close to the normal OpenAI path so upstream
  caching can still work. It still removes upstream `session_id` and
  `conversation_id` headers before sending the request.
- By default, OpenAI `input_tokens_details.cached_tokens` is converted to
  Anthropic-style `cache_read_tokens`, and stored ordinary input tokens are
  `raw_input_tokens - cache_read_tokens`, matching the existing OpenAI usage
  accounting path.
- Admin setting `openai_claude_gpt_bridge_cache_display_settings` can enable a
  bridge-only cache display override with `min_percent` and `max_percent`
  between `0` and `100`. When enabled, the bridge randomly selects a percentage
  in that range and directly sets the bridge base `cache_read_tokens` to that
  share of upstream `input_tokens`; it does not use upstream `cached_tokens` as
  the base and does not add to or scale upstream cache values. The generated
  base value is written to the usage record and participates in Claude-model
  billing.
- When `users.downstream_usage_token_mode=display`, the OpenAI Messages bridge
  rewrites the returned Anthropic JSON/SSE `usage` with the same display pricing
  chain used by usage-log display. This rewrite is response-only; both the
  downstream response and user-facing usage-log DTO are transformed from the
  same generated base usage.
- Bridge diagnostics log the token-only values at three points: raw upstream
  Responses usage, converted Anthropic usage, and final usage-log values. These
  logs do not include request or response content.
- User DTOs continue to hide `upstream_model`; admin DTOs expose it.

Compatibility notes for custom billing/ops features:

- Prompt-cache status is user/request-platform oriented. For bridge rows, the
  dashboard platform filter should prefer `groups.platform` over
  `accounts.platform`, so Antigravity users' cache behavior remains visible in
  the Antigravity cache dashboard even when the upstream account is OpenAI.
- Account-cost statistics are upstream-account oriented. Bridge rows keep using
  the GPT upstream model for `account_stats_cost`, because the admin account
  cost should represent the OpenAI account's real upstream consumption.
- Custom account-stats pricing rules match the request group platform plus the
  upstream model. In an Antigravity bridge group, write these rules as
  `platform=antigravity, model=gpt-5.5` or leave `platform` empty; a rule written
  as `platform=openai, model=gpt-5.5` will not match that Antigravity group.

## 数据模型

| 实体/类型 | 位置 | 说明 |
|-----------|------|------|
| ModelPricing | `billing_service.go:44-60` | 统一内部定价：input/output/cache per-token 价格，含 Priority tier 与长上下文字段 |
| LiteLLMModelPricing | `pricing_service.go:55-75` | 远程定价数据：含 priority tier、long context 乘数 |
| GlobalModelPricing | `global_model_pricing.go:16-42` | 全局定价覆盖：管理员手动配置，跨所有渠道生效。含展示单价字段 |
| UserModelPricingOverride | `user_model_pricing.go:13-36` | 用户级定价覆盖：per-user per-model，优先级最高。含展示单价字段 |
| ChannelModelPricing | `channel.go:53-86` | 通道级定价：模型列表 + 计费模式 + 价格 + 区间 |
| PricingInterval | `channel.go:89-108` | 按上下文窗口分段定价：(min, max] 左开右闭 |
| CostInput | `billing_service.go:430-442` | 计费输入：模型、tokens、GroupID、UserID、费率乘数 |
| CostBreakdown | `billing_service.go:102-111` | 计费输出：input/output/image_output/cache 各项费用 + ActualCost |
| ResolvedPricing | `model_pricing_resolver.go:17-38` | 解析后的定价：模式 + 基础价 + 区间 + DefaultPerRequestPrice + 来源 |

### 计费模式

| 模式 | 常量 | 说明 |
|------|------|------|
| token | `BillingModeToken` | 按 token 计价（默认），支持上下文窗口分段 |
| per_request | `BillingModePerRequest` | 按请求次数计价，支持大小档位 |
| image | `BillingModeImage` | 按图片计价 |

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| **Resolver** | `service/model_pricing_resolver.go` | 五级定价解析链（User → Channel → Global → LiteLLM → Fallback） |
| **Pricing** | `service/pricing_service.go` | LiteLLM 远程定价加载、缓存、查找 |
| **Billing** | `service/billing_service.go` | 费用计算核心：CalculateCostUnified, computeTokenBreakdown |
| **Global Pricing** | `service/global_model_pricing.go` | GlobalModelPricing 实体 + Repository 接口 |
| **Global Cache** | `service/global_model_pricing_cache.go` | 全局覆盖内存缓存（惰性加载，Invalidate 触发刷新） |
| **Global Service** | `service/global_model_pricing_service.go` | 管理后台 CRUD，CUD 后调用 `cache.Invalidate()` |
| **User Model Pricing** | `service/user_model_pricing.go` | 用户级模型定价覆盖实体 + Repository 接口 |
| **Channel Types** | `service/channel.go` | 通道/定价类型定义、区间匹配 |
| **Channel Service** | `service/channel_service.go` | 通道缓存、定价查找 (O(1)) |
| **Account** | `service/account.go:81-93` | Account.BillingRateMultiplier()（仅用于账号 quota） |
| **Group** | `ent/schema/group.go:45-47` | Group.rate_multiplier（分组默认费率） |
| **User-Group Rate** | `service/user_group_rate.go` | 用户级费率覆盖 + 展示倍率 |
| **Display Transform** | `handler/dto/display_pricing.go` | 展示变换核心：ApplyDisplayTransform + ApplyUserDisplayRate |
| **Usage Handler** | `handler/usage_handler.go:149-161` | 串联调用展示变换的入口 |
| **Pricing Page** | `handler/pricing_page_handler.go` | 用户计价页 API，展示单价直传 |
| **Admin (Global)** | `frontend/.../model-pricing/ModelPricingDetailDialog.vue` | 全局覆盖管理 UI |
| **Admin (User)** | `frontend/.../user/UserModelPricingModal.vue` | 用户级定价覆盖管理 UI |
| **Admin (Rate)** | `frontend/.../group/GroupRateMultipliersModal.vue` | 分组级费率乘数+展示倍率管理 UI |
| **Admin (User Groups)** | `frontend/.../user/UserAllowedGroupsModal.vue` | 用户级分组配置+展示倍率管理 UI |

## 核心流程

### 五级定价解析（叠加式）

```
ModelPricingResolver.Resolve(ctx, PricingInput{Model, GroupID, UserID})
  ├─ 1. resolveBasePricing(model) —— 构建基础定价（LiteLLM 为底 + Global 叠加）
  │   ├─ BillingService.GetModelPricing(model) —— 完整 ModelPricing
  │   │   ├─ PricingService.GetModelPricing(model) [LiteLLM 远程数据]
  │   │   │   → 精确匹配 → 版本归一化 → 模糊匹配 → 系列匹配
  │   │   │   → 含 Priority tier 单价、LongContext 阈值/倍率、Cache 5m/1h 分级
  │   │   └─ getFallbackPricing(model) [内置兜底]
  │   │       → 系列检测(opus/sonnet/haiku) → 版本解析 → OpenAI归一化
  │   │
  │   └─ GlobalPricingCache.Get(model) [全局覆盖]
  │       → 内存 map O(1) 查询（首次访问时惰性加载全部 enabled 条目）
  │       → applyGlobalPricingOverride(pricing, gp)
  │         · 非 nil 字段替换基础价
  │         · Priority 字段同步到覆盖价
  │         · CacheWritePrice 同时写入 Creation/5m/1h
  │         · LongContext、SupportsCacheBreakdown 从 LiteLLM 继承
  │       → Mode 与 DefaultPerRequestPrice 从 GlobalModelPricing 字段读出
  │
  ├─ 2. applyChannelOverrides [通道定价]
  │   → ChannelService.GetChannelModelPricing(groupID, model)
  │   → 缓存查找 (10min TTL) → 平台隔离过滤
  │   ├─ Token 模式: 覆盖 input/output/cache 价格 + 区间
  │   └─ Per-request/Image: 覆盖档位价格
  │
  └─ 3. applyUserModelPricingOverride [用户级定价覆盖，优先级最高]
      → UserModelPricingRepo.GetByUserAndModel(userID, model)
      → 非 nil 字段替换 BasePricing，Priority 字段同步
```

### 费用计算

```
CalculateCostUnified(CostInput)
  → Resolve pricing (if not pre-resolved)
  → 按 BillingMode 分发:
    ├─ Token 模式: calculateTokenCost()
    │   → GetIntervalPricing(totalContextTokens) 选择价格档位
    │   → 检查 long context 乘数（GPT-5.4: input×2.0, output×1.5）
    │   → computeTokenBreakdown():
    │       InputCost       = inputTokens × inputPrice
    │       OutputCost      = textOutputTokens × outputPrice
    │       ImageOutputCost = imageOutputTokens × imageOutputPrice
    │       CacheCreateCost = 5m tokens × 5m price + 1h tokens × 1h price
    │       CacheReadCost   = cacheReadTokens × cacheReadPrice
    │       TotalCost  = sum of above
    │       ActualCost = TotalCost × rateMultiplier
    │
    ├─ Per-request 模式: calculatePerRequestCost()
    │   → 按 sizeTier 查档位价格 → unitPrice × requestCount
    │   → TotalCost = unitPrice × count
    │   → ActualCost = TotalCost × rateMultiplier
    │   注意：各组件费用 (InputCost 等) 全为 0，费用只在 TotalCost 中
    │
    └─ Image 模式: CalculateCostUnified / ResolveImageBilling()
        → megapixel: pixels / 1,000,000 * image_megapixel_price
        → megapixel + image_quality_prices: low/medium/high/auto 可覆盖 USD/MP 单价
        → tier: 按 image_tier_rules 像素阈值命中 1K/2K/4K 基础价
        → tier + image_quality_multipliers: 基础价 * quality 乘数
        → OpenAI 未传 quality 时按 auto 处理；auto 默认乘数为 1，未配置乘数时保持旧档位价
        → size=auto / 缺失 / 无法解析时不猜像素，回退 per_request_price
```

### 费率乘数体系

系统中有**两条独立的**费率乘数，用于不同目的：

```
┌─────────────────────────────────────────────────────────────────┐
│ 用户扣费倍率 (rateMultiplier)                                    │
│ ActualCost = TotalCost × rateMultiplier                         │
│ 用于：余额扣减 / 订阅 quota 消耗 / API Key 限额                    │
│                                                                 │
│ 解析优先级（gateway_service.go:8354-8362）:                       │
│   用户专属分组倍率 (user_group_rate 表)                            │
│     └─ 回退: 分组默认倍率 (group.rate_multiplier)                 │
│       └─ 回退: 系统默认倍率 (config.default.rate_multiplier)      │
│                                                                 │
│ 设置入口:                                                        │
│   - 用户管理 → 分组配置 (UserAllowedGroupsModal)                  │
│   - 分组管理 → 用户专属倍率 (GroupRateMultipliersModal)            │
│   - 分组创建/编辑 → 分组默认倍率 (GroupsView)                      │
│   - config.yaml → default.rate_multiplier                       │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ 账号 Quota 倍率 (AccountRateMultiplier)                          │
│ AccountQuotaCost = TotalCost × AccountRateMultiplier            │
│ 用于：上游账号的 quota 消耗速度控制                                  │
│ 与用户扣费完全独立，用户看不到此值                                    │
│                                                                 │
│ 来源: account.RateMultiplier 字段（默认 1.0，0 = 免费）            │
│ 设置入口: 账号管理 → 编辑账号 (EditAccountModal)                    │
└─────────────────────────────────────────────────────────────────┘

注意: 两条倍率不相乘，各自独立计算各自的费用。
```

### 展示变换体系

展示变换用于**修改用户看到的使用记录**，不影响真实扣费（`actual_cost` 永远不变）。

```
使用记录 API 返回前的处理流程 (usage_handler.go:149-161):

  原始 UsageLog DTO
    │
    ├─ 1. ApplyDisplayTransform(dto, displayPricingConfig)
    │   来源: 全局模型展示单价 + 用户级模型展示单价（叠加合并）
    │   作用: 根据展示单价反算 token 数（cost 不变，rate 不变）
    │   影响字段: InputTokens, OutputTokens, CacheReadTokens 及对应 Cost
    │   不影响: CacheCreationTokens, RateMultiplier, ActualCost
    │   安全: 使用 delta 方式更新 TotalCost，不会丢失按次计费/图片计费的费用
    │
    └─ 2. ApplyUserDisplayRate(dto, displayRate)
        来源: user_group_rate 表的 display_rate_multiplier
        作用: 等比缩放所有 token/cost，修改 RateMultiplier
        公式: scale = realRate / displayRate, 所有 token 和 cost ×scale
        影响字段: 全部 token/cost + RateMultiplier + TotalCost
        不影响: ActualCost
        注意: TotalCost 直接 ×scale（不是重新求和），正确处理按次计费

恒等式: TotalCost × RateMultiplier ≈ ActualCost（存在极小 token 取整误差）
```

#### 展示单价 vs 展示倍率

| 维度 | 展示单价 | 展示倍率 |
|------|---------|---------|
| 设置粒度 | per-model（全局或用户级） | per-user-per-group |
| 设置入口 | 模型配置 / 用户模型定价 | 用户管理 / 分组管理 |
| 影响范围 | 只改 token 数和组件 cost | 改所有 token/cost + 倍率数字 |
| 不影响 | RateMultiplier, ActualCost | ActualCost |
| 串联关系 | 先执行 | 后执行（在展示单价变换结果上再缩放） |

#### 用户可见倍率的位置

| 位置 | 展示的值 | 来源 |
|------|---------|------|
| API 密钥页 — 分组徽章 | `userRateMultiplier` | user_group_rate 的 display_rate → rate → 分组默认 |
| API 密钥页 — 分组选择下拉 | 同上 | 同上 |
| 使用记录 — tooltip 倍率 | `rate_multiplier` | 后端 DTO（经过两层展示变换） |
| 使用记录 — CSV 导出 | `rate_multiplier` | 同上 |
| 可用渠道页 — 分组徽章 | `userRateMultiplier` | 同上 user_group_rate |
| 模型计价页 | 不显示倍率 | 只展示单价（直传，不涉及倍率） |

## 重要机制

| 机制 | 说明 | 相关文件 |
|------|------|---------|
| 定价远程同步 | 每 10 分钟检查远程 LiteLLM JSON 更新，SHA256 校验 | `pricing_service.go` |
| 通道缓存 | 10 分钟 TTL，singleflight 防击穿，O(1) 哈希查找 | `channel_service.go` |
| 全局覆盖缓存 | 惰性加载整表 map，CUD 触发 Invalidate；O(1) 查询 | `global_model_pricing_cache.go` |
| 区间匹配 | 左开右闭 (min, max]，500 tokens 在 (0,1000] 区间内 | `channel.go` |
| Long context | GPT-5.4 超过阈值后 input×2.0、output×1.5 | `billing_service.go` |
| Cache 分级计费 | 5m/1h 两档 cache creation 价格，需 SupportsCacheBreakdown | `billing_service.go` |
| Priority tier | service_tier="priority" 时使用 priority 价格 | `billing_service.go` |
| 兜底价格匹配 | 按系列(opus/sonnet)→版本→OpenAI归一化→安全降级 | `billing_service.go` |
| 计费模型来源 | billing_model_source: requested/upstream/channel_mapped | `channel_handler.go` |
| 全局覆盖叠加 | 非 nil 字段替换基础价，其余从 LiteLLM 继承 | `model_pricing_resolver.go` |
| 展示变换 delta | ApplyDisplayTransform 用 delta 更新 TotalCost，不丢失按次/图片费用 | `display_pricing.go` |
| 展示倍率缩放 | ApplyUserDisplayRate 直接 ×scale TotalCost，不重新求和 | `display_pricing.go` |

## 已知陷阱

- **Channel 定价优先级高于 Global**：叠加顺序是 LiteLLM → Global → Channel → User；后者覆盖前者，但只替换非 nil 字段
- **全局覆盖是叠加式而非替换式**（2026-04-14 修复）：非 nil 字段覆盖，未被覆盖的字段（Priority 差价、长上下文倍率等）继承自 LiteLLM
- **区间左开右闭**：1000 tokens 匹配 (0,1000] 而不是 (1000,∞)，容易搞混边界
- **费率乘数为 0 = 免费**：Account.RateMultiplier 或 UserGroupRate 设为 0 是合法的
- **LiteLLM 模型名不一定匹配**：GetModelPricing 有复杂的模糊匹配链
- **前端显示 $/1M tokens**：前端以百万 token 为单位，后端存储 per-token 单价
- **全局覆盖缓存必须在 CUD 后 Invalidate**：新增直接写 repo 的路径必须同步调用 Invalidate
- **按次计费的组件费用全为 0**：TotalCost 不等于各组件之和，展示变换必须用 delta/直接缩放，不能重新求和
- **ImageOutputCost 是独立字段**：不包含在 OutputCost 中，但包含在 TotalCost 中
- **展示倍率只有一个来源**：user_group_rate.display_rate_multiplier（2026-05-04 统一）。模型级 display_rate_multiplier 字段仍在 DB 中但不再使用
- **展示变换的 token 取整误差**：小额请求（极少 token）时 round() 误差占比较大，导致 TotalCost × Rate 与 ActualCost 有微小偏差
### Long-context display pricing snapshot (2026-06-09)

Token billing snapshots long-context pricing on each `usage_logs` row with
`long_context_applied`, `long_context_input_threshold`,
`long_context_input_multiplier`, and `long_context_output_multiplier`.

`computeTokenBreakdown` sets the snapshot only when session-level long-context
pricing is actually applied. Channel interval pricing keeps
`long_context_applied=false` because intervals already encode context-window
price tiers.

User-facing display transforms do not re-evaluate long-context rules. Before
calling `ApplyDisplayTransform`, DTO mapping copies the display price config and
multiplies only the per-request effective prices when the usage snapshot says
long context applied:

```
display_input_price *= long_context_input_multiplier
display_output_price *= long_context_output_multiplier
display_cache_read_price *= long_context_input_multiplier
```

This preserves `actual_cost` and prevents long-context requests from showing
inflated token counts when the configured display price is the short-context
base price. Custom display prices still scale tokens by the custom price ratio,
but do not get an extra long-context token amplification.

### Display cache premium handling (2026-05-06)

User-facing display pricing keeps cache-read token counts unchanged. When both
`display_cache_read_price` and `display_input_price` are configured, the display
layer calculates:

```
display_cache_read_tokens = real_cache_read_tokens
display_cache_read_cost = real_cache_read_tokens * display_cache_read_price
cache_premium = max(0, real_cache_read_cost - display_cache_read_cost)
display_input_cost = real_input_cost + cache_premium
display_input_tokens = round(display_input_cost / display_input_price)
```

`actual_cost` and `rate_multiplier` are unchanged by model display pricing. If
`display_input_price` is missing, cache-read usage display remains real, so the
display layer does not manufacture unexplained input tokens or silently drop the
cache premium. Output display pricing continues to affect only output
tokens/cost and never absorbs cache premium.

`cache_transfer_ratio` is deprecated by soft deletion. The database columns remain
in `global_model_pricing` and `user_model_pricing_overrides` for rollback and old
data compatibility, but the backend no longer reads or writes them, admin/user
pricing APIs no longer expose them, and the frontend no longer renders the field.

### 下游响应 token 模式 (2026-06-01)

`users.downstream_usage_token_mode` controls only the token counts returned in
downstream Claude Messages, Antigravity, and OpenAI HTTP Responses / Chat
Completions response `usage` fields:

- `real` is the default and returns the real upstream token counts.
- `display` reuses `display_token_rewrite.go` to rewrite Claude/Antigravity
  and OpenAI HTTP response usage tokens with the same display pricing chain used
  by usage logs: global display prices plus user model display overrides, then
  user-group display rate scaling when a group is present.
- OpenAI `input_tokens` includes cached tokens, so the rewrite splits
  `input_tokens_details.cached_tokens` out first, keeps cached token counts on
  the cache line during model display pricing, moves any cache-read display
  price premium into non-cached input tokens, then recombines input plus cached
  tokens and recomputes `total_tokens`. User-group display rate scaling is
  applied after this balancing step.
- API keys without a group can still use `display`; model display prices apply
  and group display rate scaling is treated as `1`.
- Billing, stored usage logs, `actual_cost`, quota deduction, and usage query
  behavior remain unchanged.

### User Model Pricing Validation (2026-06-02)

User-level model pricing overrides can directly replace real per-token prices in
`ModelPricingResolver.applyUserModelPricingOverride`. The authoritative guard is
`UserModelPricingService`: create, update, and batch upsert reject negative,
`NaN`, and infinite real/display price values before any repository write.
`display_rate_multiplier` is also rejected unless it is positive and finite.

The admin HTTP request structs keep matching early validation tags, but the
service layer is the required enforcement point for internal callers. Migration
`147_user_model_pricing_non_negative_constraints.sql` adds `NOT VALID`
PostgreSQL CHECK constraints so new inserts/updates are blocked even if an
unvalidated write path is introduced later, without scanning historical rows
during startup.
