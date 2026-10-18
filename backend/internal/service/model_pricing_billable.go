package service

import (
	"context"
	"fmt"
	"strings"
)

// ErrModelPriceNotConfigured is returned when a request names a model that
// resolves to no positive token, per-request, or image price.
var ErrModelPriceNotConfigured = fmt.Errorf("model price is not configured")

// RequireBillablePrice rejects a model that would otherwise be served for free.
// Simple mode does not call this. LiteLLM and configured global, channel, or
// user prices all count. A zero-price row does not.
func (r *ModelPricingResolver) RequireBillablePrice(ctx context.Context, model string, groupID, userID *int64) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("%w", ErrModelPriceNotConfigured)
	}
	if r == nil {
		return fmt.Errorf("%w: %s", ErrModelPriceNotConfigured, model)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolved := r.Resolve(ctx, PricingInput{Model: model, GroupID: groupID, UserID: userID})
	if !resolvedHasBillablePrice(resolved) {
		return fmt.Errorf("%w: %s", ErrModelPriceNotConfigured, model)
	}
	return nil
}

func resolvedHasBillablePrice(resolved *ResolvedPricing) bool {
	if resolved == nil {
		return false
	}
	switch resolved.Mode {
	case BillingModePerRequest, BillingModeImage:
		if resolved.DefaultPerRequestPrice > 0 || resolved.ImageMegapixelPrice > 0 {
			return true
		}
		for _, tier := range resolved.RequestTiers {
			if tier.PerRequestPrice != nil && *tier.PerRequestPrice > 0 {
				return true
			}
		}
		for _, rule := range resolved.ImageTierRules {
			if rule.Price != nil && *rule.Price > 0 {
				return true
			}
		}
		return false
	default:
		if pricingHasPositiveTokenPrice(resolved.BasePricing) {
			return true
		}
		for _, interval := range resolved.Intervals {
			if (interval.InputPrice != nil && *interval.InputPrice > 0) ||
				(interval.OutputPrice != nil && *interval.OutputPrice > 0) ||
				(interval.CacheReadPrice != nil && *interval.CacheReadPrice > 0) ||
				(interval.CacheWritePrice != nil && *interval.CacheWritePrice > 0) ||
				(interval.PerRequestPrice != nil && *interval.PerRequestPrice > 0) {
				return true
			}
		}
		return false
	}
}

func pricingHasPositiveTokenPrice(pricing *ModelPricing) bool {
	if pricing == nil {
		return false
	}
	return pricing.InputPricePerToken > 0 ||
		pricing.OutputPricePerToken > 0 ||
		pricing.CacheReadPricePerToken > 0 ||
		pricing.CacheCreationPricePerToken > 0 ||
		pricing.ImageOutputPricePerToken > 0
}
