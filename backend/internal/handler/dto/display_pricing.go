package dto

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// DisplayPricingConfig holds per-model display override settings.
type DisplayPricingConfig struct {
	DisplayInputPrice     *float64
	DisplayOutputPrice    *float64
	DisplayCacheReadPrice *float64
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
			DisplayCacheReadPrice: p.DisplayCacheReadPrice,
			CacheTransferRatio:    p.CacheTransferRatio,
		}
	}
	return m
}

func hasDisplayOverride(p *service.GlobalModelPricing) bool {
	return p.DisplayInputPrice != nil || p.DisplayOutputPrice != nil || p.DisplayCacheReadPrice != nil ||
		(p.CacheTransferRatio != nil && *p.CacheTransferRatio > 0)
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

// stripCacheTransferIfChannel returns a config copy without CacheTransferRatio
// when the usage log was billed through a channel (ChannelID != nil).
// Channel overrides set their own pricing; the global cache transfer should not apply.
func stripCacheTransferIfChannel(cfg *DisplayPricingConfig, channelID *int64) *DisplayPricingConfig {
	if cfg == nil || channelID == nil || cfg.CacheTransferRatio == nil {
		return cfg
	}
	copy := *cfg
	copy.CacheTransferRatio = nil
	return &copy
}

// ApplyDisplayTransform modifies a user-facing UsageLog DTO in-place to use display prices.
// The actual_cost field is never changed — only tokens and per-component costs are adjusted
// so that the user sees display prices while being charged the real amount.
// Rate multiplier is NOT changed here; use ApplyUserDisplayRate for that.
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

	// Step 2: Display price replacement + token rescaling.
	// When display prices differ from real prices, rescale tokens so that
	// displayTokens × displayPrice = realTokens × realPrice (cost unchanged).
	// Rate multiplier is untouched — it's only changed by ApplyUserDisplayRate.

	// Snapshot component sum before rescaling — used to compute delta for TotalCost.
	// This preserves any non-component cost (per-request, image output, etc.).
	oldComponentSum := d.InputCost + d.OutputCost + d.CacheCreationCost + d.CacheReadCost

	// Input tokens rescaling
	if cfg.DisplayInputPrice != nil && *cfg.DisplayInputPrice > 0 && d.InputTokens > 0 && d.InputCost > 0 {
		displayTokens := d.InputCost / *cfg.DisplayInputPrice
		d.InputTokens = int(math.Round(displayTokens))
		d.InputCost = float64(d.InputTokens) * *cfg.DisplayInputPrice
	}

	// Output tokens rescaling
	if cfg.DisplayOutputPrice != nil && *cfg.DisplayOutputPrice > 0 && d.OutputTokens > 0 && d.OutputCost > 0 {
		displayTokens := d.OutputCost / *cfg.DisplayOutputPrice
		d.OutputTokens = int(math.Round(displayTokens))
		d.OutputCost = float64(d.OutputTokens) * *cfg.DisplayOutputPrice
	}

	// Cache read tokens rescaling
	if cfg.DisplayCacheReadPrice != nil && *cfg.DisplayCacheReadPrice > 0 && d.CacheReadTokens > 0 && d.CacheReadCost > 0 {
		displayTokens := d.CacheReadCost / *cfg.DisplayCacheReadPrice
		d.CacheReadTokens = int(math.Round(displayTokens))
		d.CacheReadCost = float64(d.CacheReadTokens) * *cfg.DisplayCacheReadPrice
	}

	// Apply component cost delta to TotalCost. This correctly handles:
	// - per-request billing (component costs are all 0 → delta is 0 → TotalCost unchanged)
	// - image output cost (not in components → preserved in TotalCost)
	// - token rounding adjustments (small deltas applied)
	newComponentSum := d.InputCost + d.OutputCost + d.CacheCreationCost + d.CacheReadCost
	d.TotalCost += newComponentSum - oldComponentSum
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
		InputTokens:     clone.InputTokens,
		OutputTokens:    clone.OutputTokens,
		CacheReadTokens: clone.CacheReadTokens,
		InputCost:       clone.InputCost,
		OutputCost:      clone.OutputCost,
		CacheReadCost:   clone.CacheReadCost,
		TotalCost:       clone.TotalCost,
	}
}

// DisplayUsageFields holds the user-visible values for admin dual-column display.
type DisplayUsageFields struct {
	InputTokens     int     `json:"display_input_tokens"`
	OutputTokens    int     `json:"display_output_tokens"`
	CacheReadTokens int     `json:"display_cache_read_tokens"`
	InputCost       float64 `json:"display_input_cost"`
	OutputCost      float64 `json:"display_output_cost"`
	CacheReadCost   float64 `json:"display_cache_read_cost"`
	TotalCost       float64 `json:"display_total_cost"`
}

// ApplyUserDisplayRate applies a user-group level display rate multiplier transform.
// This is the ONLY place where the displayed rate_multiplier is changed.
// actual_cost is NEVER changed.
// Token counts and costs are scaled so that new_total_cost × displayRate ≈ actual_cost.
func ApplyUserDisplayRate(d *UsageLog, displayRate float64) {
	currentRate := d.RateMultiplier
	if displayRate <= 0 || displayRate == currentRate {
		return
	}
	scale := currentRate / displayRate

	if d.InputTokens > 0 {
		d.InputTokens = int(math.Round(float64(d.InputTokens) * scale))
		d.InputCost *= scale
	}
	if d.OutputTokens > 0 {
		d.OutputTokens = int(math.Round(float64(d.OutputTokens) * scale))
		d.OutputCost *= scale
	}
	if d.CacheReadTokens > 0 {
		d.CacheReadTokens = int(math.Round(float64(d.CacheReadTokens) * scale))
		d.CacheReadCost *= scale
	}
	if d.CacheCreationTokens > 0 {
		d.CacheCreationTokens = int(math.Round(float64(d.CacheCreationTokens) * scale))
		d.CacheCreationCost *= scale
	}
	d.ImageOutputCost *= scale

	// Scale TotalCost directly instead of summing components, so per-request
	// billing cost and any other non-component cost is preserved.
	d.TotalCost *= scale
	d.RateMultiplier = displayRate
}

// BuildUserDisplayPricingMap merges user-level display overrides on top of the global display map.
// Priority: user-level > global-level. Only non-nil user fields replace global values.
func BuildUserDisplayPricingMap(globalMap DisplayPricingMap, userOverrides []service.UserModelPricingOverride) DisplayPricingMap {
	merged := make(DisplayPricingMap, len(globalMap))
	for k, v := range globalMap {
		clone := *v
		merged[k] = &clone
	}
	for i := range userOverrides {
		o := &userOverrides[i]
		if !o.Enabled || o.Model == "" {
			continue
		}
		if o.DisplayInputPrice == nil && o.DisplayOutputPrice == nil && o.DisplayCacheReadPrice == nil &&
			o.CacheTransferRatio == nil {
			continue
		}
		key := toLowerModel(o.Model)
		existing := merged[key]
		if existing == nil {
			existing = &DisplayPricingConfig{}
			merged[key] = existing
		}
		if o.DisplayInputPrice != nil {
			existing.DisplayInputPrice = o.DisplayInputPrice
		}
		if o.DisplayOutputPrice != nil {
			existing.DisplayOutputPrice = o.DisplayOutputPrice
		}
		if o.DisplayCacheReadPrice != nil {
			existing.DisplayCacheReadPrice = o.DisplayCacheReadPrice
		}
		if o.CacheTransferRatio != nil {
			existing.CacheTransferRatio = o.CacheTransferRatio
		}
	}
	return merged
}
