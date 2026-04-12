# 计费系统 (Billing & Pricing)

> 模型定价解析、费用计算、费率乘数。三级定价链（Channel → LiteLLM → 内置兜底）+ 三级费率乘数（Account × Group × User-Group）。

## 数据模型

| 实体/类型 | 位置 | 说明 |
|-----------|------|------|
| ModelPricing | `billing_service.go:44-60` | 统一内部定价：input/output/cache per-token 价格 |
| LiteLLMModelPricing | `pricing_service.go:55-75` | 远程定价数据：含 priority tier、long context 乘数 |
| ChannelModelPricing | `channel.go:53-86` | 通道级定价：模型列表 + 计费模式 + 价格 + 区间 |
| PricingInterval | `channel.go:89-108` | 按上下文窗口分段定价：(min, max] 左开右闭 |
| CostInput | `billing_service.go:415-428` | 计费输入：模型、tokens、GroupID、费率乘数 |
| CostBreakdown | `billing_service.go:102-110` | 计费输出：input/output/cache/image 各项费用 |
| ResolvedPricing | `model_pricing_resolver.go:39-50` | 解析后的定价：模式 + 基础价 + 区间 + 来源 |

### 计费模式

| 模式 | 常量 | 说明 |
|------|------|------|
| token | `BillingModeToken` | 按 token 计价（默认），支持上下文窗口分段 |
| per_request | `BillingModePerRequest` | 按请求次数计价，支持大小档位 |
| image | `BillingModeImage` | 按图片计价 |

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| **Resolver** | `backend/internal/service/model_pricing_resolver.go` | 三级定价解析链 |
| **Pricing** | `backend/internal/service/pricing_service.go` | LiteLLM 远程定价加载、缓存、查找 |
| **Billing** | `backend/internal/service/billing_service.go` | 费用计算核心：CalculateCostUnified, computeTokenBreakdown |
| **Channel Types** | `backend/internal/service/channel.go` | 通道/定价类型定义、区间匹配 |
| **Channel Service** | `backend/internal/service/channel_service.go:141-463` | 通道缓存、定价查找 (O(1)) |
| **Channel Repo** | `backend/internal/repository/channel_repo_pricing.go` | 定价持久化：CRUD + 区间管理 |
| **Channel Handler** | `backend/internal/handler/admin/channel_handler.go` | 通道管理 API |
| **Account** | `backend/internal/service/account.go:79-91` | Account.BillingRateMultiplier() |
| **Group** | `backend/internal/service/group.go:17,71-81` | Group.RateMultiplier |
| **User-Group Rate** | `backend/internal/service/user_group_rate.go` | 用户级费率覆盖 |
| **Frontend** | `frontend/src/components/admin/channel/PricingEntryCard.vue` | 定价配置 UI |
| **Frontend** | `frontend/src/components/admin/group/GroupRateMultipliersModal.vue` | 费率乘数管理 UI |

## 核心流程

### 三级定价解析

```
ModelPricingResolver.Resolve(model, groupID)
  ├─ 1. resolveBasePricing(model)
  │   ├─ PricingService.GetModelPricing(model) [LiteLLM 远程数据]
  │   │   → 精确匹配 → 版本归一化 → 模糊匹配 → 系列匹配
  │   └─ getFallbackPricing(model) [内置兜底，billing_service.go:136-273]
  │       → 系列检测(opus/sonnet/haiku) → 版本解析 → OpenAI归一化
  │
  ├─ 2. ChannelService.GetChannelModelPricing(groupID, model) [通道定价]
  │   → 缓存查找 (10min TTL) → 平台隔离过滤
  │
  └─ 3. applyChannelOverrides(channelPricing, resolved)
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
| 区间匹配 | 左开右闭 (min, max]，500 tokens 在 (0,1000] 区间内 | `channel.go:110-121` |
| Long context | GPT-5.4 超过阈值后 input×2.0、output×1.5 | `billing_service.go:650-659` |
| Cache 分级计费 | 5m/1h 两档 cache creation 价格，需 SupportsCacheBreakdown | `billing_service.go:558-568` |
| Priority tier | service_tier="priority" 时使用 priority 价格（通常更贵） | `billing_service.go:497-510` |
| 兜底价格匹配 | 按系列(opus/sonnet)→版本→OpenAI归一化→安全降级 | `billing_service.go:275-333` |
| 计费模型来源 | billing_model_source: requested/upstream/channel_mapped 决定用哪个模型名查价格 | `channel_handler.go:28-112` |

## 已知陷阱

- **Channel 定价优先级最高**：配了 Channel 定价后完全覆盖 LiteLLM 默认价，如果 Channel 漏配某个价格字段会变成 0
- **区间左开右闭**：1000 tokens 匹配 (0,1000] 而不是 (1000,∞)，容易搞混边界
- **费率乘数为 0 = 免费**：Account.RateMultiplier 设为 0 是合法的，意味着该账号免费
- **LiteLLM 模型名不一定匹配**：LiteLLM JSON 中的模型名可能和实际请求的不一致，GetModelPricing 有复杂的模糊匹配链
- **前端显示 $/1M tokens**：前端配置界面以百万 token 为单位，后端存储 per-token 单价，注意换算
