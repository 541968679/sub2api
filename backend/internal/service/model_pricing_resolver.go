package service

import (
	"context"
)

// PricingSource 定价来源标识
const (
	PricingSourceChannel  = "channel"
	PricingSourceGlobal   = "global"
	PricingSourceLiteLLM  = "litellm"
	PricingSourceFallback = "fallback"
	PricingSourceUser     = "user"
)

// ResolvedPricing 统一定价解析结果
type ResolvedPricing struct {
	// Mode 计费模式
	Mode BillingMode

	// Token 模式：基础定价（来自 LiteLLM 或 fallback）
	BasePricing *ModelPricing

	// Token 模式：区间定价列表（如有，覆盖 BasePricing 中的对应字段）
	Intervals []PricingInterval

	// 按次/图片模式：分层定价
	RequestTiers []PricingInterval

	// 按次/图片模式：默认价格（未命中层级时使用）
	DefaultPerRequestPrice float64

	// 来源标识
	Source string // "channel", "litellm", "fallback"

	// 是否支持缓存细分
	SupportsCacheBreakdown bool
}

// ModelPricingResolver 统一模型定价解析器。
// 解析链：(LiteLLM/Fallback 为底) → Global 叠加 → Channel 叠加 → User 叠加。
// 注意：Global、Channel、User 均为"叠加覆盖"语义——仅替换非 nil 字段，其余保留底层值。
// 这样可以在不丢失 Priority tier、长上下文倍率、缓存 5m/1h 分级等字段的前提下
// 让管理员自定义单价。
type ModelPricingResolver struct {
	channelService       *ChannelService
	billingService       *BillingService
	globalPricingCache   *GlobalPricingCache        // 可选，nil 时跳过全局覆盖
	userModelPricingRepo UserModelPricingRepository // 可选，nil 时跳过用户级覆盖
}

// NewModelPricingResolver 创建定价解析器实例
func NewModelPricingResolver(channelService *ChannelService, billingService *BillingService, globalPricingCache *GlobalPricingCache, userModelPricingRepo UserModelPricingRepository) *ModelPricingResolver {
	return &ModelPricingResolver{
		channelService:       channelService,
		billingService:       billingService,
		globalPricingCache:   globalPricingCache,
		userModelPricingRepo: userModelPricingRepo,
	}
}

// PricingInput 定价解析输入
type PricingInput struct {
	Model   string
	GroupID *int64 // nil 表示不检查渠道
	UserID  *int64 // nil 表示不检查用户级覆盖
}

// Resolve 解析模型定价。
// 1. 获取基础定价（LiteLLM/Fallback 为底，若存在全局覆盖则叠加之）
// 2. 如果指定了 GroupID，查找渠道定价并再次叠加
// 3. 如果指定了 UserID，查找用户级定价覆盖并最终叠加
func (r *ModelPricingResolver) Resolve(ctx context.Context, input PricingInput) *ResolvedPricing {
	var chPricing *ChannelModelPricing
	if input.GroupID != nil && r.channelService != nil {
		chPricing = r.channelService.GetChannelModelPricing(ctx, *input.GroupID, input.Model)
		if chPricing != nil {
			mode := chPricing.BillingMode
			if mode == "" {
				mode = BillingModeToken
			}
			if mode == BillingModePerRequest || mode == BillingModeImage {
				resolved := &ResolvedPricing{
					Mode:   mode,
					Source: PricingSourceChannel,
				}
				r.applyRequestTierOverrides(chPricing, resolved)
				return resolved
			}
		}
	}

	// 1. 获取基础定价（含全局覆盖）
	basePricing, baseMode, defaultPerRequest, source := r.resolveBasePricing(input.Model)

	resolved := &ResolvedPricing{
		Mode:                   baseMode,
		BasePricing:            basePricing,
		DefaultPerRequestPrice: defaultPerRequest,
		Source:                 source,
		SupportsCacheBreakdown: basePricing != nil && basePricing.SupportsCacheBreakdown,
	}

	// 2. 如果有 GroupID，尝试渠道覆盖
	if chPricing != nil {
		resolved.Source = PricingSourceChannel
		r.applyTokenOverrides(chPricing, resolved)
	} else if input.GroupID != nil {
		r.applyChannelOverrides(ctx, *input.GroupID, input.Model, resolved)
	}

	// 3. 如果有 UserID，尝试用户级定价覆盖（优先级最高）
	if input.UserID != nil && r.userModelPricingRepo != nil {
		r.applyUserModelPricingOverride(ctx, *input.UserID, input.Model, resolved)
	}

	return resolved
}

// resolveBasePricing 构建基础定价：先从 LiteLLM/Fallback 取完整 ModelPricing，
// 再用全局覆盖的非 nil 字段叠加。
//
// 返回：(定价, 计费模式, 按次默认价, 来源)。
//   - 计费模式：若全局覆盖指定了 BillingMode 则用之，否则默认 Token
//   - 按次默认价：仅当模式为 per_request/image 且全局覆盖设置了 PerRequestPrice 时有值
//
// 为什么不简单地把 Global 覆盖构造成独立的 ModelPricing：因为 LiteLLM 的
// ModelPricing 包含 Priority tier 单价、长上下文阈值/倍率、缓存 5m/1h 分级等
// 字段——这些都是管理员在前端 UI 里没法配置的，但后端计费仍然依赖它们。
// 若直接替换，GPT-5.4 超过 272K 的双倍输入费、Claude 的 priority tier 溢价
// 都会静默消失。叠加式实现保证这些字段被保留。
func (r *ModelPricingResolver) resolveBasePricing(model string) (*ModelPricing, BillingMode, float64, string) {
	// 1. 先从 LiteLLM/Fallback 获取完整基础定价
	basePricing, err := r.billingService.GetModelPricing(model)
	baseSource := PricingSourceLiteLLM
	if err != nil {
		// GetModelPricing 未找到模型时返回 err；这里不记录 debug 日志避免
		// 对未知自定义模型频繁噪声
		basePricing = nil
		baseSource = PricingSourceFallback
	}

	// 2. 查询全局覆盖并叠加
	if r.globalPricingCache != nil {
		if gp := r.globalPricingCache.Get(model); gp != nil && gp.Enabled {
			if basePricing == nil {
				basePricing = &ModelPricing{}
			}
			applyGlobalPricingOverride(basePricing, gp)

			mode := gp.BillingMode
			if mode == "" {
				mode = BillingModeToken
			}
			defaultPerRequest := 0.0
			if gp.PerRequestPrice != nil {
				defaultPerRequest = *gp.PerRequestPrice
			}
			return basePricing, mode, defaultPerRequest, PricingSourceGlobal
		}
	}

	return basePricing, BillingModeToken, 0, baseSource
}

// applyGlobalPricingOverride 将全局覆盖的非 nil 字段叠加到基础定价上。
// 语义与 applyTokenOverrides（渠道覆盖）保持一致：非 nil 替换、Priority 字段同步。
//
// 为什么 Priority 字段也被同步：管理员在前端只配置一个"输入价格"，没有区分
// 普通和 priority tier。叠加后让 Priority 等于普通价是合理默认——相当于声明
// "这个模型我按固定价计费，不再区分 tier"。如果 LiteLLM 底层有 priority
// 单价而我们想保留它，只要不配全局覆盖即可（即不叠加）。
func applyGlobalPricingOverride(pricing *ModelPricing, gp *GlobalModelPricing) {
	if gp.InputPrice != nil {
		pricing.InputPricePerToken = *gp.InputPrice
		pricing.InputPricePerTokenPriority = *gp.InputPrice
	}
	if gp.OutputPrice != nil {
		pricing.OutputPricePerToken = *gp.OutputPrice
		pricing.OutputPricePerTokenPriority = *gp.OutputPrice
	}
	if gp.CacheWritePrice != nil {
		pricing.CacheCreationPricePerToken = *gp.CacheWritePrice
		pricing.CacheCreation5mPrice = *gp.CacheWritePrice
		pricing.CacheCreation1hPrice = *gp.CacheWritePrice
	}
	if gp.CacheReadPrice != nil {
		pricing.CacheReadPricePerToken = *gp.CacheReadPrice
		pricing.CacheReadPricePerTokenPriority = *gp.CacheReadPrice
	}
	if gp.ImageOutputPrice != nil {
		pricing.ImageOutputPricePerToken = *gp.ImageOutputPrice
	}
}

// applyChannelOverrides 应用渠道定价覆盖
func (r *ModelPricingResolver) applyChannelOverrides(ctx context.Context, groupID int64, model string, resolved *ResolvedPricing) {
	chPricing := r.channelService.GetChannelModelPricing(ctx, groupID, model)
	if chPricing == nil {
		return
	}

	resolved.Source = PricingSourceChannel
	resolved.Mode = chPricing.BillingMode
	if resolved.Mode == "" {
		resolved.Mode = BillingModeToken
	}

	switch resolved.Mode {
	case BillingModeToken:
		r.applyTokenOverrides(chPricing, resolved)
	case BillingModePerRequest, BillingModeImage:
		r.applyRequestTierOverrides(chPricing, resolved)
	}
}

// applyTokenOverrides 应用 token 模式的渠道覆盖
func (r *ModelPricingResolver) applyTokenOverrides(chPricing *ChannelModelPricing, resolved *ResolvedPricing) {
	// 过滤掉所有价格字段都为空的无效 interval
	validIntervals := filterValidIntervals(chPricing.Intervals)

	// 如果有有效的区间定价，使用区间
	if len(validIntervals) > 0 {
		resolved.Intervals = validIntervals
		return
	}

	// 否则用 flat 字段覆盖 BasePricing
	if resolved.BasePricing == nil {
		resolved.BasePricing = &ModelPricing{}
	}

	if chPricing.InputPrice != nil {
		resolved.BasePricing.InputPricePerToken = *chPricing.InputPrice
		resolved.BasePricing.InputPricePerTokenPriority = *chPricing.InputPrice
	}
	if chPricing.OutputPrice != nil {
		resolved.BasePricing.OutputPricePerToken = *chPricing.OutputPrice
		resolved.BasePricing.OutputPricePerTokenPriority = *chPricing.OutputPrice
	}
	if chPricing.CacheWritePrice != nil {
		resolved.BasePricing.CacheCreationPricePerToken = *chPricing.CacheWritePrice
		resolved.BasePricing.CacheCreation5mPrice = *chPricing.CacheWritePrice
		resolved.BasePricing.CacheCreation1hPrice = *chPricing.CacheWritePrice
	}
	if chPricing.CacheReadPrice != nil {
		resolved.BasePricing.CacheReadPricePerToken = *chPricing.CacheReadPrice
		resolved.BasePricing.CacheReadPricePerTokenPriority = *chPricing.CacheReadPrice
	}
	if chPricing.ImageOutputPrice != nil {
		resolved.BasePricing.ImageOutputPricePerToken = *chPricing.ImageOutputPrice
	}
}

// applyRequestTierOverrides 应用按次/图片模式的渠道覆盖
func (r *ModelPricingResolver) applyRequestTierOverrides(chPricing *ChannelModelPricing, resolved *ResolvedPricing) {
	resolved.RequestTiers = filterValidIntervals(chPricing.Intervals)
	if chPricing.PerRequestPrice != nil {
		resolved.DefaultPerRequestPrice = *chPricing.PerRequestPrice
	}
}

// filterValidIntervals 过滤掉所有价格字段都为空的无效 interval。
// 前端可能创建了只有 min/max 但无价格的空 interval。
func filterValidIntervals(intervals []PricingInterval) []PricingInterval {
	var valid []PricingInterval
	for _, iv := range intervals {
		if iv.InputPrice != nil || iv.OutputPrice != nil ||
			iv.CacheWritePrice != nil || iv.CacheReadPrice != nil ||
			iv.PerRequestPrice != nil {
			valid = append(valid, iv)
		}
	}
	return valid
}

// GetIntervalPricing 根据 context token 数获取区间定价。
// 如果有区间列表，找到匹配区间并构造 ModelPricing；否则直接返回 BasePricing。
func (r *ModelPricingResolver) GetIntervalPricing(resolved *ResolvedPricing, totalContextTokens int) *ModelPricing {
	if len(resolved.Intervals) == 0 {
		return resolved.BasePricing
	}

	iv := FindMatchingInterval(resolved.Intervals, totalContextTokens)
	if iv == nil {
		return resolved.BasePricing
	}

	return intervalToModelPricing(iv, resolved.SupportsCacheBreakdown)
}

// intervalToModelPricing 将区间定价转换为 ModelPricing
func intervalToModelPricing(iv *PricingInterval, supportsCacheBreakdown bool) *ModelPricing {
	pricing := &ModelPricing{
		SupportsCacheBreakdown: supportsCacheBreakdown,
	}
	if iv.InputPrice != nil {
		pricing.InputPricePerToken = *iv.InputPrice
		pricing.InputPricePerTokenPriority = *iv.InputPrice
	}
	if iv.OutputPrice != nil {
		pricing.OutputPricePerToken = *iv.OutputPrice
		pricing.OutputPricePerTokenPriority = *iv.OutputPrice
	}
	if iv.CacheWritePrice != nil {
		pricing.CacheCreationPricePerToken = *iv.CacheWritePrice
		pricing.CacheCreation5mPrice = *iv.CacheWritePrice
		pricing.CacheCreation1hPrice = *iv.CacheWritePrice
	}
	if iv.CacheReadPrice != nil {
		pricing.CacheReadPricePerToken = *iv.CacheReadPrice
		pricing.CacheReadPricePerTokenPriority = *iv.CacheReadPrice
	}
	return pricing
}

// GetRequestTierPrice 根据层级标签获取按次价格
func (r *ModelPricingResolver) GetRequestTierPrice(resolved *ResolvedPricing, tierLabel string) float64 {
	for _, tier := range resolved.RequestTiers {
		if tier.TierLabel == tierLabel && tier.PerRequestPrice != nil {
			return *tier.PerRequestPrice
		}
	}
	return 0
}

// GetRequestTierPriceByContext 根据 context token 数获取按次价格
func (r *ModelPricingResolver) GetRequestTierPriceByContext(resolved *ResolvedPricing, totalContextTokens int) float64 {
	iv := FindMatchingInterval(resolved.RequestTiers, totalContextTokens)
	if iv != nil && iv.PerRequestPrice != nil {
		return *iv.PerRequestPrice
	}
	return 0
}

// applyUserModelPricingOverride 将用户级定价覆盖的非 nil 字段叠加到 resolved 上。
// 语义与 applyGlobalPricingOverride 一致：非 nil 替换、Priority 字段同步。
// 用户级覆盖优先级最高：User > Channel > Global > LiteLLM/Fallback。
func (r *ModelPricingResolver) applyUserModelPricingOverride(ctx context.Context, userID int64, model string, resolved *ResolvedPricing) {
	override, err := r.userModelPricingRepo.GetByUserAndModel(ctx, userID, model)
	if err != nil || override == nil || !override.Enabled {
		return
	}

	hasBillingOverride := override.InputPrice != nil || override.OutputPrice != nil ||
		override.CacheWritePrice != nil || override.CacheReadPrice != nil
	if !hasBillingOverride {
		return
	}

	if resolved.BasePricing == nil {
		resolved.BasePricing = &ModelPricing{}
	}

	if override.InputPrice != nil {
		resolved.BasePricing.InputPricePerToken = *override.InputPrice
		resolved.BasePricing.InputPricePerTokenPriority = *override.InputPrice
	}
	if override.OutputPrice != nil {
		resolved.BasePricing.OutputPricePerToken = *override.OutputPrice
		resolved.BasePricing.OutputPricePerTokenPriority = *override.OutputPrice
	}
	if override.CacheWritePrice != nil {
		resolved.BasePricing.CacheCreationPricePerToken = *override.CacheWritePrice
		resolved.BasePricing.CacheCreation5mPrice = *override.CacheWritePrice
		resolved.BasePricing.CacheCreation1hPrice = *override.CacheWritePrice
	}
	if override.CacheReadPrice != nil {
		resolved.BasePricing.CacheReadPricePerToken = *override.CacheReadPrice
		resolved.BasePricing.CacheReadPricePerTokenPriority = *override.CacheReadPrice
	}

	resolved.Source = PricingSourceUser
}
