# 计费系统 (Billing & Pricing)

> 模型定价解析、费用计算、费率乘数。四级定价链（Channel → Global → LiteLLM → 内置兜底）+ 三级费率乘数（Account × Group × User-Group）。

## 数据模型

| 实体/类型 | 位置 | 说明 |
|-----------|------|------|
| ModelPricing | `billing_service.go:44-60` | 统一内部定价：input/output/cache per-token 价格，含 Priority tier 与长上下文字段 |
| LiteLLMModelPricing | `pricing_service.go:55-75` | 远程定价数据：含 priority tier、long context 乘数 |
| GlobalModelPricing | `global_model_pricing.go:12-27` | 全局定价覆盖：管理员手动配置，跨所有渠道生效 |
| ChannelModelPricing | `channel.go:53-86` | 通道级定价：模型列表 + 计费模式 + 价格 + 区间 |
| PricingInterval | `channel.go:89-108` | 按上下文窗口分段定价：(min, max] 左开右闭 |
| CostInput | `billing_service.go:415-428` | 计费输入：模型、tokens、GroupID、费率乘数 |
| CostBreakdown | `billing_service.go:102-110` | 计费输出：input/output/cache/image 各项费用 |
| ResolvedPricing | `model_pricing_resolver.go:16-38` | 解析后的定价：模式 + 基础价 + 区间 + DefaultPerRequestPrice + 来源 |

### 计费模式

| 模式 | 常量 | 说明 |
|------|------|------|
| token | `BillingModeToken` | 按 token 计价（默认），支持上下文窗口分段 |
| per_request | `BillingModePerRequest` | 按请求次数计价，支持大小档位 |
| image | `BillingModeImage` | 按图片计价 |

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| **Resolver** | `backend/internal/service/model_pricing_resolver.go` | 四级定价解析链（Channel → Global → LiteLLM → Fallback） |
| **Pricing** | `backend/internal/service/pricing_service.go` | LiteLLM 远程定价加载、缓存、查找 |
| **Billing** | `backend/internal/service/billing_service.go` | 费用计算核心：CalculateCostUnified, computeTokenBreakdown |
| **Global Pricing** | `backend/internal/service/global_model_pricing.go` | GlobalModelPricing 实体 + Repository 接口 |
| **Global Cache** | `backend/internal/service/global_model_pricing_cache.go` | 全局覆盖内存缓存（惰性加载，Invalidate 触发刷新） |
| **Global Service** | `backend/internal/service/global_model_pricing_service.go` | 管理后台 CRUD，CUD 后调用 `cache.Invalidate()` |
| **Global Repo** | `backend/internal/repository/global_model_pricing_repo.go` | SQL CRUD |
| **Global Handler** | `backend/internal/handler/admin/model_pricing_handler.go` | 全局覆盖管理 API（`/admin/model-pricing`） |
| **Channel Types** | `backend/internal/service/channel.go` | 通道/定价类型定义、区间匹配 |
| **Channel Service** | `backend/internal/service/channel_service.go:141-463` | 通道缓存、定价查找 (O(1)) |
| **Channel Repo** | `backend/internal/repository/channel_repo_pricing.go` | 定价持久化：CRUD + 区间管理 |
| **Channel Handler** | `backend/internal/handler/admin/channel_handler.go` | 通道管理 API |
| **Account** | `backend/internal/service/account.go:79-91` | Account.BillingRateMultiplier() |
| **Group** | `backend/internal/service/group.go:17,71-81` | Group.RateMultiplier |
| **User-Group Rate** | `backend/internal/service/user_group_rate.go` | 用户级费率覆盖 |
| **Frontend (Channel)** | `frontend/src/components/admin/channel/PricingEntryCard.vue` | 通道定价配置 UI |
| **Frontend (Global)** | `frontend/src/components/admin/model-pricing/ModelPricingTab.vue` | 全局覆盖管理 UI（`/admin/model-config`） |
| **Frontend (Rate)** | `frontend/src/components/admin/group/GroupRateMultipliersModal.vue` | 费率乘数管理 UI |

## 核心流程

### 四级定价解析（叠加式）

```
ModelPricingResolver.Resolve(ctx, PricingInput{Model, GroupID})
  ├─ 1. resolveBasePricing(model) —— 构建基础定价（LiteLLM 为底 + Global 叠加）
  │   ├─ BillingService.GetModelPricing(model) —— 完整 ModelPricing
  │   │   ├─ PricingService.GetModelPricing(model) [LiteLLM 远程数据]
  │   │   │   → 精确匹配 → 版本归一化 → 模糊匹配 → 系列匹配
  │   │   │   → 含 Priority tier 单价、LongContext 阈值/倍率、Cache 5m/1h 分级
  │   │   └─ getFallbackPricing(model) [内置兜底，billing_service.go:136-273]
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
  ├─ 2. ChannelService.GetChannelModelPricing(groupID, model) [通道定价]
  │   → 缓存查找 (10min TTL) → 平台隔离过滤
  │
  └─ 3. applyChannelOverrides(channelPricing, resolved) [通道叠加在最外层]
      ├─ Token 模式: 覆盖 input/output/cache 价格 + 区间
      └─ Per-request/Image: 覆盖档位价格
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
    │       InputCost  = inputTokens × inputPrice
    │       OutputCost = outputTokens × outputPrice
    │       CacheCreateCost = 5m tokens × 5m price + 1h tokens × 1h price
    │       CacheReadCost = cacheReadTokens × cacheReadPrice
    │       TotalCost = sum of above
    │       ActualCost = TotalCost × rateMultiplier
    │
    └─ Per-request 模式: calculatePerRequestCost()
        → 按 sizeTier 查档位价格 → unitPrice × requestCount
```

### 费率乘数叠加

```
最终费用 = TotalCost × rateMultiplier

rateMultiplier 来源（gateway_service.go:7262-7277）:
  1. getUserGroupRateMultiplier(userID, groupID, groupDefault)
     ├─ 优先: UserGroupRate 表中该用户在该 Group 的个性化费率
     └─ 回退: Group.RateMultiplier（组默认费率）
  2. Account.BillingRateMultiplier()（账号级别，通常 1.0）
  → 两者相乘得最终 rateMultiplier
```

## 重要机制

| 机制 | 说明 | 相关文件 |
|------|------|---------|
| 定价远程同步 | 每 10 分钟检查远程 LiteLLM JSON 更新，SHA256 校验 | `pricing_service.go:181,283` |
| 通道缓存 | 10 分钟 TTL，singleflight 防击穿，O(1) 哈希查找 | `channel_service.go:141-463` |
| 全局覆盖缓存 | 惰性加载整表 map，CUD 触发 Invalidate；O(1) 查询 | `global_model_pricing_cache.go` |
| 区间匹配 | 左开右闭 (min, max]，500 tokens 在 (0,1000] 区间内 | `channel.go:110-121` |
| Long context | GPT-5.4 超过阈值后 input×2.0、output×1.5 | `billing_service.go:650-659` |
| Cache 分级计费 | 5m/1h 两档 cache creation 价格，需 SupportsCacheBreakdown | `billing_service.go:558-568` |
| Priority tier | service_tier="priority" 时使用 priority 价格（通常更贵） | `billing_service.go:497-510` |
| 兜底价格匹配 | 按系列(opus/sonnet)→版本→OpenAI归一化→安全降级 | `billing_service.go:275-333` |
| 计费模型来源 | billing_model_source: requested/upstream/channel_mapped 决定用哪个模型名查价格 | `channel_handler.go:28-112` |
| 全局覆盖叠加语义 | 非 nil 字段替换基础价，其余从 LiteLLM 继承（保留 Priority/长上下文/5m1h 分级） | `model_pricing_resolver.go:applyGlobalPricingOverride` |

## 已知陷阱

- **Channel 定价优先级最高**：叠加顺序是 LiteLLM → Global → Channel；后者覆盖前者，但 Channel 覆盖只替换「非 nil 字段」，不会把未填的字段变成 0
- **全局覆盖是叠加式而非替换式**（2026-04-14 修复）：
  - 修复前 `ToModelPricing()` 只设 5 个字段，导致 Priority tier 单价归零、GPT-5.4 长上下文双倍费丢失、缓存 5m/1h 分级失效
  - 修复后改为在 LiteLLM 完整定价上"叠加非 nil 字段"，未被覆盖的字段（Priority 差价、长上下文倍率等）继承自 LiteLLM
- **Global 覆盖的 Anthropic 网关曾静默失效**（2026-04-14 修复）：
  - 修复前 `gateway_service.go:resolveChannelPricing` 只在 `Source==Channel` 时返回 resolved，只配全局覆盖的情形会回落到 `CalculateCost` 旧路径，旧路径不查 GlobalPricingCache
  - 修复后放宽为 `Source==Channel || Source==Global`，两者都走 CalculateCostUnified
- **Global 覆盖的 BillingMode 曾被忽略**（2026-04-14 修复）：
  - 修复前 `Resolve` 把 Mode 硬编码为 `BillingModeToken`，只在渠道叠加分支改；全局配 `per_request` → 按 token 计费 → 单价全 0 → 免费
  - 修复后 `resolveBasePricing` 返回 mode 字段，`Resolve` 原样塞进 `ResolvedPricing`
- **UpdateOverride 曾把未提供字段清零**（2026-04-14 修复）：PATCH 漏带某字段会把已有价格覆盖成 nil；现已改为"非 nil 才覆盖"的差量更新
- **区间左开右闭**：1000 tokens 匹配 (0,1000] 而不是 (1000,∞)，容易搞混边界
- **费率乘数为 0 = 免费**：Account.RateMultiplier 设为 0 是合法的，意味着该账号免费
- **LiteLLM 模型名不一定匹配**：LiteLLM JSON 中的模型名可能和实际请求的不一致，GetModelPricing 有复杂的模糊匹配链
- **前端显示 $/1M tokens**：前端配置界面以百万 token 为单位，后端存储 per-token 单价，注意换算
- **全局覆盖缓存必须在 CUD 后 Invalidate**：`GlobalModelPricingService.Create/Update/Delete` 已在成功写库后调用 `cache.Invalidate()`；如果将来新增直接写 repo 的路径，必须同步调用 Invalidate，否则改动不会在热路径生效
