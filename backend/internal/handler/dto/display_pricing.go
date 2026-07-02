package dto

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// DisplayPricingConfig holds per-model display override settings.
type DisplayPricingConfig struct {
	DisplayInputPrice         *float64
	DisplayOutputPrice        *float64
	DisplayCacheReadPrice     *float64
	DisplayCacheCreationPrice *float64
}

// DisplayPricingMap maps lowercase model name to display config.
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
			DisplayInputPrice:         p.DisplayInputPrice,
			DisplayOutputPrice:        p.DisplayOutputPrice,
			DisplayCacheReadPrice:     p.DisplayCacheReadPrice,
			DisplayCacheCreationPrice: p.DisplayCacheCreationPrice,
		}
	}
	return m
}

func hasDisplayOverride(p *service.GlobalModelPricing) bool {
	return p.DisplayInputPrice != nil || p.DisplayOutputPrice != nil || p.DisplayCacheReadPrice != nil || p.DisplayCacheCreationPrice != nil
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

func stripCacheTransferIfChannel(cfg *DisplayPricingConfig, channelID *int64) *DisplayPricingConfig {
	return cfg
}

func effectiveDisplayPricingForUsageLog(d *UsageLog, cfg *DisplayPricingConfig) *DisplayPricingConfig {
	if d == nil || cfg == nil || !d.LongContextApplied {
		return cfg
	}
	clone := *cfg
	if d.LongContextInputMultiplier > 0 {
		if clone.DisplayInputPrice != nil {
			value := *clone.DisplayInputPrice * d.LongContextInputMultiplier
			clone.DisplayInputPrice = &value
		}
		if clone.DisplayCacheReadPrice != nil {
			value := *clone.DisplayCacheReadPrice * d.LongContextInputMultiplier
			clone.DisplayCacheReadPrice = &value
		}
		if clone.DisplayCacheCreationPrice != nil {
			value := *clone.DisplayCacheCreationPrice * d.LongContextInputMultiplier
			clone.DisplayCacheCreationPrice = &value
		}
	}
	if d.LongContextOutputMultiplier > 0 && clone.DisplayOutputPrice != nil {
		value := *clone.DisplayOutputPrice * d.LongContextOutputMultiplier
		clone.DisplayOutputPrice = &value
	}
	return &clone
}

// ApplyDisplayTransform modifies a user-facing UsageLog DTO in-place to use display prices.
// The actual_cost field is never changed. Rate multiplier is not changed here;
// use ApplyUserDisplayRate for that.
func ApplyDisplayTransform(d *UsageLog, cfg *DisplayPricingConfig) {
	if cfg == nil {
		return
	}

	oldComponentSum := d.InputCost + d.OutputCost + d.CacheCreationCost + d.CacheReadCost
	inputCostForDisplay := d.InputCost

	// Keep cache-read tokens unchanged. Cache premium is only explainable when
	// both display cache and display input prices exist.
	if cfg.DisplayCacheReadPrice != nil && *cfg.DisplayCacheReadPrice > 0 &&
		cfg.DisplayInputPrice != nil && *cfg.DisplayInputPrice > 0 &&
		d.CacheReadTokens > 0 && d.CacheReadCost > 0 {
		realCacheReadCost := d.CacheReadCost
		displayCacheReadCost := float64(d.CacheReadTokens) * *cfg.DisplayCacheReadPrice
		d.CacheReadCost = displayCacheReadCost

		cachePremium := realCacheReadCost - displayCacheReadCost
		if cachePremium > 0 {
			inputCostForDisplay += cachePremium
		}
	}

	if cfg.DisplayInputPrice != nil && *cfg.DisplayInputPrice > 0 && d.InputTokens > 0 && inputCostForDisplay > 0 {
		displayTokens := inputCostForDisplay / *cfg.DisplayInputPrice
		d.InputTokens = int(math.Round(displayTokens))
		d.InputCost = float64(d.InputTokens) * *cfg.DisplayInputPrice
	}

	if cfg.DisplayOutputPrice != nil && *cfg.DisplayOutputPrice > 0 && d.OutputTokens > 0 && d.OutputCost > 0 {
		displayTokens := d.OutputCost / *cfg.DisplayOutputPrice
		d.OutputTokens = int(math.Round(displayTokens))
		d.OutputCost = float64(d.OutputTokens) * *cfg.DisplayOutputPrice
	}

	// Cache creation: unlike cache-read (tokens kept, premium folded into input),
	// cache-creation tokens are back-computed directly from the real cost at the
	// display price — the same amplification shape as input/output above.
	if cfg.DisplayCacheCreationPrice != nil && *cfg.DisplayCacheCreationPrice > 0 && d.CacheCreationTokens > 0 && d.CacheCreationCost > 0 {
		realTokens := d.CacheCreationTokens
		displayTokens := d.CacheCreationCost / *cfg.DisplayCacheCreationPrice
		d.CacheCreationTokens = int(math.Round(displayTokens))
		d.CacheCreationCost = float64(d.CacheCreationTokens) * *cfg.DisplayCacheCreationPrice
		rescaleCacheCreationBreakdown(d, realTokens)
	}

	newComponentSum := d.InputCost + d.OutputCost + d.CacheCreationCost + d.CacheReadCost
	d.TotalCost += newComponentSum - oldComponentSum
	// actual_cost is never changed.
}

// rescaleCacheCreationBreakdown keeps the 5m/1h breakdown summing to the new
// CacheCreationTokens total after it was rewritten from realTokens. One tier is
// scaled proportionally and the other derived by subtraction so rounding can
// never break the 5m+1h == total invariant.
func rescaleCacheCreationBreakdown(d *UsageLog, realTokens int) {
	if realTokens <= 0 || (d.CacheCreation5mTokens <= 0 && d.CacheCreation1hTokens <= 0) {
		return
	}
	if d.CacheCreation1hTokens <= 0 {
		d.CacheCreation5mTokens = d.CacheCreationTokens
		return
	}
	if d.CacheCreation5mTokens <= 0 {
		d.CacheCreation1hTokens = d.CacheCreationTokens
		return
	}
	factor := float64(d.CacheCreationTokens) / float64(realTokens)
	scaled5m := int(math.Round(float64(d.CacheCreation5mTokens) * factor))
	if scaled5m > d.CacheCreationTokens {
		scaled5m = d.CacheCreationTokens
	}
	d.CacheCreation5mTokens = scaled5m
	d.CacheCreation1hTokens = d.CacheCreationTokens - scaled5m
}

// ComputeDisplayFields computes display values for admin DTO (for dual-column comparison).
// Returns nil if no display override is configured for this model.
func ComputeDisplayFields(d *UsageLog, cfg *DisplayPricingConfig) *DisplayUsageFields {
	if cfg == nil {
		return nil
	}
	clone := *d
	ApplyDisplayTransform(&clone, cfg)
	return &DisplayUsageFields{
		InputTokens:         clone.InputTokens,
		OutputTokens:        clone.OutputTokens,
		CacheReadTokens:     clone.CacheReadTokens,
		CacheCreationTokens: clone.CacheCreationTokens,
		InputCost:           clone.InputCost,
		OutputCost:          clone.OutputCost,
		CacheReadCost:       clone.CacheReadCost,
		CacheCreationCost:   clone.CacheCreationCost,
		TotalCost:           clone.TotalCost,
	}
}

// DisplayUsageFields holds the user-visible values for admin dual-column display.
type DisplayUsageFields struct {
	InputTokens         int     `json:"display_input_tokens"`
	OutputTokens        int     `json:"display_output_tokens"`
	CacheReadTokens     int     `json:"display_cache_read_tokens"`
	CacheCreationTokens int     `json:"display_cache_creation_tokens"`
	InputCost           float64 `json:"display_input_cost"`
	OutputCost          float64 `json:"display_output_cost"`
	CacheReadCost       float64 `json:"display_cache_read_cost"`
	CacheCreationCost   float64 `json:"display_cache_creation_cost"`
	TotalCost           float64 `json:"display_total_cost"`
}

// ApplyUserDisplayRate applies a user-group level display rate multiplier transform.
// This is the only place where the displayed rate_multiplier is changed.
// actual_cost is never changed.
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
		realTokens := d.CacheCreationTokens
		d.CacheCreationTokens = int(math.Round(float64(d.CacheCreationTokens) * scale))
		d.CacheCreationCost *= scale
		rescaleCacheCreationBreakdown(d, realTokens)
	}
	d.ImageOutputCost *= scale

	// Scale TotalCost directly so per-request and other non-component costs survive.
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
		if o.DisplayInputPrice == nil && o.DisplayOutputPrice == nil && o.DisplayCacheReadPrice == nil && o.DisplayCacheCreationPrice == nil {
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
		if o.DisplayCacheCreationPrice != nil {
			existing.DisplayCacheCreationPrice = o.DisplayCacheCreationPrice
		}
	}
	return merged
}
