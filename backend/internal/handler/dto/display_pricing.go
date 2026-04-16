package dto

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// DisplayPricingConfig holds per-model display override settings.
type DisplayPricingConfig struct {
	DisplayInputPrice     *float64
	DisplayOutputPrice    *float64
	DisplayRateMultiplier *float64
	CacheTransferRatio    *float64
}

// DisplayPricingMap maps lowercase model name → display config.
type DisplayPricingMap map[string]*DisplayPricingConfig

// BuildDisplayPricingMap builds a lookup map from all enabled global model pricing entries.
func BuildDisplayPricingMap(pricings []service.GlobalModelPricing) DisplayPricingMap {
	m := make(DisplayPricingMap)
	for i := range pricings {
		p := &pricings[i]
		if !hasDisplayOverride(p) {
			continue
		}
		m[toLowerModel(p.Model)] = &DisplayPricingConfig{
			DisplayInputPrice:     p.DisplayInputPrice,
			DisplayOutputPrice:    p.DisplayOutputPrice,
			DisplayRateMultiplier: p.DisplayRateMultiplier,
			CacheTransferRatio:    p.CacheTransferRatio,
		}
	}
	return m
}

func hasDisplayOverride(p *service.GlobalModelPricing) bool {
	return p.DisplayInputPrice != nil || p.DisplayOutputPrice != nil ||
		p.DisplayRateMultiplier != nil || (p.CacheTransferRatio != nil && *p.CacheTransferRatio > 0)
}

func toLowerModel(model string) string {
	b := make([]byte, len(model))
	for i := range model {
		c := model[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// ApplyDisplayTransform modifies a user-facing UsageLog DTO in-place to use display values.
// The actual_cost field is never changed — only tokens, costs, and rate_multiplier are adjusted
// so that the user sees display prices while being charged the real amount.
func ApplyDisplayTransform(d *UsageLog, cfg *DisplayPricingConfig) {
	if cfg == nil {
		return
	}

	// Step 1: Cache transfer — move a portion of cache_read tokens to input tokens.
	if cfg.CacheTransferRatio != nil && *cfg.CacheTransferRatio > 0 && d.CacheReadTokens > 0 {
		transfer := int(math.Round(float64(d.CacheReadTokens) * *cfg.CacheTransferRatio))
		if transfer > d.CacheReadTokens {
			transfer = d.CacheReadTokens
		}
		d.InputTokens += transfer
		d.CacheReadTokens -= transfer
		// Redistribute costs proportionally
		if d.CacheReadCost > 0 {
			transferredCost := d.CacheReadCost * float64(transfer) / float64(transfer+d.CacheReadTokens)
			d.InputCost += transferredCost
			d.CacheReadCost -= transferredCost
		}
	}

	// Step 2: Price/rate replacement + token rescaling.
	// Goal: displayTokens × displayPrice × displayRate = actualCost (unchanged)
	// We compute a scale factor per cost component and adjust tokens accordingly.

	displayRate := d.RateMultiplier
	if cfg.DisplayRateMultiplier != nil && *cfg.DisplayRateMultiplier > 0 {
		displayRate = *cfg.DisplayRateMultiplier
	}

	// Input tokens rescaling
	if cfg.DisplayInputPrice != nil && *cfg.DisplayInputPrice > 0 && d.InputTokens > 0 {
		realInputCostTotal := d.InputCost * d.RateMultiplier // cost with real rate
		// displayTokens × displayPrice × displayRate = realInputCostTotal
		displayTokens := realInputCostTotal / (*cfg.DisplayInputPrice * displayRate)
		d.InputTokens = int(math.Round(displayTokens))
		d.InputCost = float64(d.InputTokens) * *cfg.DisplayInputPrice
	}

	// Output tokens rescaling
	if cfg.DisplayOutputPrice != nil && *cfg.DisplayOutputPrice > 0 && d.OutputTokens > 0 {
		realOutputCostTotal := d.OutputCost * d.RateMultiplier
		displayTokens := realOutputCostTotal / (*cfg.DisplayOutputPrice * displayRate)
		d.OutputTokens = int(math.Round(displayTokens))
		d.OutputCost = float64(d.OutputTokens) * *cfg.DisplayOutputPrice
	}

	// Cache read tokens: if input price is overridden but cache isn't, scale cache similarly
	if d.CacheReadTokens > 0 && d.CacheReadCost > 0 {
		realCacheCostTotal := d.CacheReadCost * d.RateMultiplier
		// Use real cache price for display (no override for cache read display price)
		if d.RateMultiplier != displayRate {
			cachePrice := d.CacheReadCost / float64(d.CacheReadTokens)
			displayTokens := realCacheCostTotal / (cachePrice * displayRate)
			d.CacheReadTokens = int(math.Round(displayTokens))
			d.CacheReadCost = float64(d.CacheReadTokens) * cachePrice
		}
	}

	// Recalculate total_cost from display components
	d.TotalCost = d.InputCost + d.OutputCost + d.CacheCreationCost + d.CacheReadCost
	d.RateMultiplier = displayRate
	// actual_cost is NEVER changed
}

// ComputeDisplayFields computes display values for admin DTO (for dual-column comparison).
// Returns nil if no display override is configured for this model.
func ComputeDisplayFields(d *UsageLog, cfg *DisplayPricingConfig) *DisplayUsageFields {
	if cfg == nil {
		return nil
	}
	// Clone the DTO, apply transform, extract display fields
	clone := *d
	ApplyDisplayTransform(&clone, cfg)
	return &DisplayUsageFields{
		InputTokens:    clone.InputTokens,
		OutputTokens:   clone.OutputTokens,
		CacheReadTokens: clone.CacheReadTokens,
		InputCost:      clone.InputCost,
		OutputCost:     clone.OutputCost,
		CacheReadCost:  clone.CacheReadCost,
		TotalCost:      clone.TotalCost,
		RateMultiplier: clone.RateMultiplier,
	}
}

// DisplayUsageFields holds the user-visible values for admin dual-column display.
type DisplayUsageFields struct {
	InputTokens    int     `json:"display_input_tokens"`
	OutputTokens   int     `json:"display_output_tokens"`
	CacheReadTokens int    `json:"display_cache_read_tokens"`
	InputCost      float64 `json:"display_input_cost"`
	OutputCost     float64 `json:"display_output_cost"`
	CacheReadCost  float64 `json:"display_cache_read_cost"`
	TotalCost      float64 `json:"display_total_cost"`
	RateMultiplier float64 `json:"display_rate_multiplier"`
}
