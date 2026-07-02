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
	DisplayCacheCreationPrice *float64 // 5m 档展示价；1h 档未配时也作用于 1h
	UnitInputPrice            *float64
	UnitOutputPrice           *float64
	UnitCacheReadPrice        *float64
	HasDisplayOverride        bool
	// DisplayCacheCreation1hPrice 是 1h 档展示价（nil = 回退 5m 档展示价）。
	// 配置后按档反算展示 token，需要真实档价比例来拆分单一的 cache_creation_cost。
	DisplayCacheCreation1hPrice *float64
	// RealCacheWritePrice/RealCacheWrite1hPrice 来自定价条目的真实档价，
	// 仅用于按比例拆分成本（比例未知时按 1:1 处理），不会出现在任何用户可见输出中。
	RealCacheWritePrice   *float64
	RealCacheWrite1hPrice *float64
}

// DisplayPricingMap maps lowercase model name to display config.
type DisplayPricingMap map[string]*DisplayPricingConfig

// BuildDisplayPricingMap builds a lookup map from all enabled global model pricing entries.
func BuildDisplayPricingMap(pricings []service.GlobalModelPricing) DisplayPricingMap {
	m := make(DisplayPricingMap)
	for i := range pricings {
		p := &pricings[i]
		hasOverride := hasDisplayOverride(p)
		if !hasOverride && !hasUnitPrice(p) {
			continue
		}
		m[toLowerModel(p.Model)] = &DisplayPricingConfig{
			DisplayInputPrice:           p.DisplayInputPrice,
			DisplayOutputPrice:          p.DisplayOutputPrice,
			DisplayCacheReadPrice:       p.DisplayCacheReadPrice,
			DisplayCacheCreationPrice:   p.DisplayCacheCreationPrice,
			UnitInputPrice:              firstPrice(p.DisplayInputPrice, p.InputPrice),
			UnitOutputPrice:             firstPrice(p.DisplayOutputPrice, p.OutputPrice),
			UnitCacheReadPrice:          firstPrice(p.DisplayCacheReadPrice, p.CacheReadPrice),
			HasDisplayOverride:          hasOverride,
			DisplayCacheCreation1hPrice: p.DisplayCacheCreation1hPrice,
			RealCacheWritePrice:         p.CacheWritePrice,
			RealCacheWrite1hPrice:       p.CacheWrite1hPrice,
		}
	}
	return m
}

func hasDisplayOverride(p *service.GlobalModelPricing) bool {
	return p.DisplayInputPrice != nil || p.DisplayOutputPrice != nil || p.DisplayCacheReadPrice != nil ||
		p.DisplayCacheCreationPrice != nil || p.DisplayCacheCreation1hPrice != nil
}

func hasUnitPrice(p *service.GlobalModelPricing) bool {
	return p.InputPrice != nil || p.OutputPrice != nil || p.CacheReadPrice != nil
}

func firstPrice(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
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

func cloneDisplayPricingConfig(cfg *DisplayPricingConfig) *DisplayPricingConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	return &clone
}

// DisplayPricingConfigForModel returns a cloned display config for a model.
func DisplayPricingConfigForModel(displayMap DisplayPricingMap, model string) *DisplayPricingConfig {
	if displayMap == nil {
		return nil
	}
	return cloneDisplayPricingConfig(displayMap[toLowerModel(model)])
}

// ApplyResolvedUnitPrices adds effective resolver prices to a display config.
// Explicit display prices still win; resolver prices are only the configured
// model-price fallback and are never derived from usage costs.
func ApplyResolvedUnitPrices(cfg *DisplayPricingConfig, inputPrice, outputPrice, cacheReadPrice float64) *DisplayPricingConfig {
	if cfg == nil {
		cfg = &DisplayPricingConfig{}
	}
	if cfg.DisplayInputPrice == nil && inputPrice > 0 {
		cfg.UnitInputPrice = &inputPrice
	}
	if cfg.DisplayOutputPrice == nil && outputPrice > 0 {
		cfg.UnitOutputPrice = &outputPrice
	}
	if cfg.DisplayCacheReadPrice == nil && cacheReadPrice > 0 {
		cfg.UnitCacheReadPrice = &cacheReadPrice
	}
	if cfg.HasDisplayOverride || cfg.UnitInputPrice != nil || cfg.UnitOutputPrice != nil || cfg.UnitCacheReadPrice != nil {
		return cfg
	}
	return nil
}

func stripCacheTransferIfChannel(cfg *DisplayPricingConfig, channelID *int64) *DisplayPricingConfig {
	return cfg
}

func EffectiveDisplayPricingForUsageLog(d *UsageLog, cfg *DisplayPricingConfig) *DisplayPricingConfig {
	if d == nil || cfg == nil || !d.LongContextApplied {
		return cfg
	}
	clone := *cfg
	if d.LongContextInputMultiplier > 0 {
		if clone.DisplayInputPrice != nil {
			value := *clone.DisplayInputPrice * d.LongContextInputMultiplier
			clone.DisplayInputPrice = &value
		}
		if clone.UnitInputPrice != nil {
			value := *clone.UnitInputPrice * d.LongContextInputMultiplier
			clone.UnitInputPrice = &value
		}
		if clone.DisplayCacheReadPrice != nil {
			value := *clone.DisplayCacheReadPrice * d.LongContextInputMultiplier
			clone.DisplayCacheReadPrice = &value
		}
		if clone.UnitCacheReadPrice != nil {
			value := *clone.UnitCacheReadPrice * d.LongContextInputMultiplier
			clone.UnitCacheReadPrice = &value
		}
		if clone.DisplayCacheCreationPrice != nil {
			value := *clone.DisplayCacheCreationPrice * d.LongContextInputMultiplier
			clone.DisplayCacheCreationPrice = &value
		}
		if clone.DisplayCacheCreation1hPrice != nil {
			value := *clone.DisplayCacheCreation1hPrice * d.LongContextInputMultiplier
			clone.DisplayCacheCreation1hPrice = &value
		}
	}
	if d.LongContextOutputMultiplier > 0 && clone.DisplayOutputPrice != nil {
		value := *clone.DisplayOutputPrice * d.LongContextOutputMultiplier
		clone.DisplayOutputPrice = &value
	}
	if d.LongContextOutputMultiplier > 0 && clone.UnitOutputPrice != nil {
		value := *clone.UnitOutputPrice * d.LongContextOutputMultiplier
		clone.UnitOutputPrice = &value
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
	if price := firstPrice(cfg.UnitInputPrice, cfg.DisplayInputPrice); price != nil && *price > 0 {
		d.DisplayInputPrice = price
	}
	if price := firstPrice(cfg.UnitOutputPrice, cfg.DisplayOutputPrice); price != nil && *price > 0 {
		d.DisplayOutputPrice = price
	}
	if price := firstPrice(cfg.UnitCacheReadPrice, cfg.DisplayCacheReadPrice); price != nil && *price > 0 {
		d.DisplayCacheReadPrice = price
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
		display5mPrice := *cfg.DisplayCacheCreationPrice
		display1hPrice := display5mPrice
		if cfg.DisplayCacheCreation1hPrice != nil && *cfg.DisplayCacheCreation1hPrice > 0 {
			display1hPrice = *cfg.DisplayCacheCreation1hPrice
		}
		hasBreakdown := d.CacheCreation5mTokens > 0 || d.CacheCreation1hTokens > 0
		if hasBreakdown && display1hPrice != display5mPrice {
			// 分档展示价：用真实档价比例拆分成本，各档独立反算。
			// 只用比例（r=1h/5m）拆分实际落库成本，成本总额天然守恒。
			ratio := 1.0
			if cfg.RealCacheWritePrice != nil && *cfg.RealCacheWritePrice > 0 &&
				cfg.RealCacheWrite1hPrice != nil && *cfg.RealCacheWrite1hPrice > 0 {
				ratio = *cfg.RealCacheWrite1hPrice / *cfg.RealCacheWritePrice
			}
			w5m := float64(d.CacheCreation5mTokens)
			w1h := float64(d.CacheCreation1hTokens) * ratio
			cost5m := d.CacheCreationCost * w5m / (w5m + w1h)
			cost1h := d.CacheCreationCost - cost5m
			display5m := int(math.Round(cost5m / display5mPrice))
			display1h := int(math.Round(cost1h / display1hPrice))
			d.CacheCreation5mTokens = display5m
			d.CacheCreation1hTokens = display1h
			d.CacheCreationTokens = display5m + display1h
			d.CacheCreationCost = float64(display5m)*display5mPrice + float64(display1h)*display1hPrice
		} else {
			// 单一展示价：总成本反算（1h 溢价已含在成本里，天然按档放大）。
			realTokens := d.CacheCreationTokens
			displayTokens := d.CacheCreationCost / display5mPrice
			d.CacheCreationTokens = int(math.Round(displayTokens))
			d.CacheCreationCost = float64(d.CacheCreationTokens) * display5mPrice
			rescaleCacheCreationBreakdown(d, realTokens)
		}
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
	if cfg == nil || !cfg.HasDisplayOverride {
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

	oldTotal := d.TotalCost
	oldComponentSum := usageLogComponentSum(d)
	otherCost := oldTotal - oldComponentSum
	if math.Abs(otherCost) < 1e-12 {
		otherCost = 0
	}
	scaledOtherCost := otherCost * scale
	targetTotal := oldTotal * scale

	if d.OutputTokens > 0 || d.OutputCost > 0 {
		d.OutputTokens, d.OutputCost = scaleDisplayedTokenComponent(d.OutputTokens, d.OutputCost, scale, d.DisplayOutputPrice)
	}
	if d.CacheReadTokens > 0 && d.DisplayCacheReadPrice != nil && *d.DisplayCacheReadPrice > 0 {
		d.CacheReadCost = float64(d.CacheReadTokens) * *d.DisplayCacheReadPrice
	}
	if d.CacheCreationTokens > 0 {
		realTokens := d.CacheCreationTokens
		d.CacheCreationTokens = int(math.Round(float64(d.CacheCreationTokens) * scale))
		d.CacheCreationCost *= scale
		rescaleCacheCreationBreakdown(d, realTokens)
	}
	d.ImageOutputCost *= scale

	inputTargetCost := targetTotal - scaledOtherCost - d.OutputCost - d.CacheCreationCost - d.CacheReadCost - d.ImageOutputCost
	if inputTargetCost < 0 {
		inputTargetCost = 0
	}
	if d.InputTokens > 0 || d.InputCost > 0 || inputTargetCost > 0 {
		if d.DisplayInputPrice != nil && *d.DisplayInputPrice > 0 && inputTargetCost > 0 {
			d.InputTokens = roundDisplayTokensFromCost(inputTargetCost, *d.DisplayInputPrice)
			d.InputCost = float64(d.InputTokens) * *d.DisplayInputPrice
		} else {
			if d.InputTokens > 0 {
				d.InputTokens = int(math.Round(float64(d.InputTokens) * scale))
			}
			d.InputCost = inputTargetCost
		}
	}

	d.TotalCost = scaledOtherCost + usageLogComponentSum(d)
	d.RateMultiplier = displayRate
}

func scaleDisplayedTokenComponent(tokens int, cost float64, scale float64, price *float64) (int, float64) {
	targetCost := cost * scale
	if price != nil && *price > 0 && targetCost > 0 {
		displayTokens := roundDisplayTokensFromCost(targetCost, *price)
		return displayTokens, float64(displayTokens) * *price
	}
	if tokens > 0 {
		tokens = int(math.Round(float64(tokens) * scale))
	}
	return tokens, targetCost
}

func roundDisplayTokensFromCost(cost float64, price float64) int {
	if price <= 0 {
		return 0
	}
	return int(math.Round(cost/price + 1e-9))
}

func usageLogComponentSum(d *UsageLog) float64 {
	if d == nil {
		return 0
	}
	return d.InputCost + d.OutputCost + d.CacheCreationCost + d.CacheReadCost + d.ImageOutputCost
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
		if o.InputPrice == nil && o.OutputPrice == nil && o.CacheReadPrice == nil &&
			o.DisplayInputPrice == nil && o.DisplayOutputPrice == nil && o.DisplayCacheReadPrice == nil &&
			o.DisplayCacheCreationPrice == nil && o.DisplayCacheCreation1hPrice == nil {
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
			existing.UnitInputPrice = o.DisplayInputPrice
			existing.HasDisplayOverride = true
		} else if o.InputPrice != nil {
			existing.UnitInputPrice = o.InputPrice
		}
		if o.DisplayOutputPrice != nil {
			existing.DisplayOutputPrice = o.DisplayOutputPrice
			existing.UnitOutputPrice = o.DisplayOutputPrice
			existing.HasDisplayOverride = true
		} else if o.OutputPrice != nil {
			existing.UnitOutputPrice = o.OutputPrice
		}
		if o.DisplayCacheReadPrice != nil {
			existing.DisplayCacheReadPrice = o.DisplayCacheReadPrice
			existing.UnitCacheReadPrice = o.DisplayCacheReadPrice
			existing.HasDisplayOverride = true
		} else if o.CacheReadPrice != nil {
			existing.UnitCacheReadPrice = o.CacheReadPrice
		}
		if o.DisplayCacheCreationPrice != nil {
			existing.DisplayCacheCreationPrice = o.DisplayCacheCreationPrice
			existing.HasDisplayOverride = true
		}
		if o.DisplayCacheCreation1hPrice != nil {
			existing.DisplayCacheCreation1hPrice = o.DisplayCacheCreation1hPrice
			existing.HasDisplayOverride = true
		}
		// 用户级真实档价（若配置）替换成本拆分比例的来源
		if o.CacheWritePrice != nil {
			existing.RealCacheWritePrice = o.CacheWritePrice
		}
		if o.CacheWrite1hPrice != nil {
			existing.RealCacheWrite1hPrice = o.CacheWrite1hPrice
		}
	}
	return merged
}
