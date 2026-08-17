package service

import (
	"context"
	"math"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// trueCostUnitPrices are per-component unit prices for applyTrueCost.
// Each field is resolved independently: user display_* → global display_* → LiteLLM → 0.
// Billing InputPrice / Channel / User billing prices are never used.
type trueCostUnitPrices struct {
	Input         float64
	Output        float64
	CacheWrite5m  float64
	CacheWrite1h  float64
	CacheRead     float64
	ImagePerToken float64
	ImagePerImage float64
}

func trueCostDisplayModel(log *UsageLog) string {
	if log == nil {
		return ""
	}
	if model := strings.TrimSpace(log.RequestedModel); model != "" {
		return model
	}
	return strings.TrimSpace(log.Model)
}

func firstDisplayPrice(prices ...*float64) *float64 {
	for _, price := range prices {
		if price != nil {
			return price
		}
	}
	return nil
}

func pickTrueCostUnit(display *float64, litellm float64) float64 {
	if display != nil {
		return *display
	}
	if litellm > 0 {
		return litellm
	}
	return 0
}

func liteLLMTrueCostPrices(ps *PricingService, model string) trueCostUnitPrices {
	var out trueCostUnitPrices
	if ps == nil || strings.TrimSpace(model) == "" {
		return out
	}
	// PricingService.GetModelPricing is LiteLLM + its fuzzy/family match only.
	// Do not call BillingService.GetModelPricing: that injects hardcoded billing fallbacks.
	lp := ps.GetModelPricing(model)
	if lp == nil {
		return out
	}
	out.Input = lp.InputCostPerToken
	out.Output = lp.OutputCostPerToken
	out.CacheRead = lp.CacheReadInputTokenCost
	out.CacheWrite5m = lp.CacheCreationInputTokenCost
	out.CacheWrite1h = lp.CacheCreationInputTokenCostAbove1hr
	out.ImagePerToken = lp.OutputCostPerImageToken
	out.ImagePerImage = lp.OutputCostPerImage
	if out.CacheWrite1h <= 0 {
		out.CacheWrite1h = out.CacheWrite5m
	}
	return out
}

func resolveTrueCostUnitPrices(ctx context.Context, resolver *ModelPricingResolver, userID int64, model string) trueCostUnitPrices {
	var user *UserModelPricingOverride
	var global *GlobalModelPricing
	var pricing *PricingService
	if resolver != nil {
		if resolver.billingService != nil {
			pricing = resolver.billingService.pricingService
		}
		if resolver.userModelPricingRepo != nil && userID > 0 && model != "" {
			if override, err := resolver.userModelPricingRepo.GetByUserAndModel(ctx, userID, model); err == nil && override != nil && override.Enabled {
				user = override
			}
		}
		if resolver.globalPricingCache != nil && model != "" {
			if gp := resolver.globalPricingCache.Get(model); gp != nil && gp.Enabled {
				global = gp
			}
		}
	}

	litellm := liteLLMTrueCostPrices(pricing, model)

	var userIn, userOut, userRead, userWrite5m, userWrite1h *float64
	if user != nil {
		userIn = user.DisplayInputPrice
		userOut = user.DisplayOutputPrice
		userRead = user.DisplayCacheReadPrice
		userWrite5m = user.DisplayCacheCreationPrice
		userWrite1h = firstDisplayPrice(user.DisplayCacheCreation1hPrice, user.DisplayCacheCreationPrice)
	}
	var globalIn, globalOut, globalRead, globalWrite5m, globalWrite1h *float64
	if global != nil {
		globalIn = global.DisplayInputPrice
		globalOut = global.DisplayOutputPrice
		globalRead = global.DisplayCacheReadPrice
		globalWrite5m = global.DisplayCacheCreationPrice
		globalWrite1h = firstDisplayPrice(global.DisplayCacheCreation1hPrice, global.DisplayCacheCreationPrice)
	}

	write5m := pickTrueCostUnit(firstDisplayPrice(userWrite5m, globalWrite5m), litellm.CacheWrite5m)
	return trueCostUnitPrices{
		Input:         pickTrueCostUnit(firstDisplayPrice(userIn, globalIn), litellm.Input),
		Output:        pickTrueCostUnit(firstDisplayPrice(userOut, globalOut), litellm.Output),
		CacheWrite5m:  write5m,
		CacheWrite1h:  pickTrueCostUnit(firstDisplayPrice(userWrite1h, globalWrite1h), litellm.CacheWrite1h),
		CacheRead:     pickTrueCostUnit(firstDisplayPrice(userRead, globalRead), litellm.CacheRead),
		ImagePerToken: litellm.ImagePerToken,
		ImagePerImage: litellm.ImagePerImage,
	}
}

func computeTrueCostAmount(log *UsageLog, prices trueCostUnitPrices) float64 {
	if log == nil {
		return 0
	}
	write5m := log.CacheCreation5mTokens
	write1h := log.CacheCreation1hTokens
	if write5m == 0 && write1h == 0 {
		write5m = log.CacheCreationTokens
	}
	amount := float64(log.InputTokens)*prices.Input +
		float64(log.OutputTokens)*prices.Output +
		float64(write5m)*prices.CacheWrite5m +
		float64(write1h)*prices.CacheWrite1h +
		float64(log.CacheReadTokens)*prices.CacheRead +
		float64(log.ImageOutputTokens)*prices.ImagePerToken
	if log.ImageCount > 0 && prices.ImagePerImage > 0 {
		amount += float64(log.ImageCount) * prices.ImagePerImage
	}
	return amount
}

// applyTrueCost writes true_cost / true_cost_rate after ActualCost is already set.
// Failures are logged and leave both fields nil; ActualCost is never changed.
func applyTrueCost(ctx context.Context, usageLog *UsageLog, rate float64, resolver *ModelPricingResolver) {
	if usageLog == nil {
		return
	}
	savedActual := usageLog.ActualCost
	defer func() {
		usageLog.ActualCost = savedActual
		if rec := recover(); rec != nil {
			logger.L().Warn("true_cost.apply_panic",
				zap.Any("recover", rec),
				zap.Int64("user_id", usageLog.UserID),
				zap.Int64("account_id", usageLog.AccountID),
			)
			usageLog.TrueCost = nil
			usageLog.TrueCostRate = nil
		}
	}()
	if resolver == nil {
		return
	}
	model := trueCostDisplayModel(usageLog)
	prices := resolveTrueCostUnitPrices(ctx, resolver, usageLog.UserID, model)
	cost := computeTrueCostAmount(usageLog, prices) * rate
	if !isFiniteTrueCost(cost) || !isFiniteTrueCost(rate) {
		logger.L().Warn("true_cost.non_finite",
			zap.Float64("cost", cost),
			zap.Float64("rate", rate),
			zap.Int64("user_id", usageLog.UserID),
			zap.Int64("account_id", usageLog.AccountID),
		)
		return
	}
	usageLog.TrueCostRate = &rate
	usageLog.TrueCost = &cost
}

func isFiniteTrueCost(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
