package service

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type ImageBillingStrategy string

const (
	ImageBillingStrategyTier      ImageBillingStrategy = "tier"
	ImageBillingStrategyMegapixel ImageBillingStrategy = "megapixel"

	ImageBillingTier1K        = "1K"
	ImageBillingTier2K        = "2K"
	ImageBillingTier4K        = "4K"
	ImageBillingTierMegapixel = "megapixel"
)

type ImageQualityPrices map[string]float64
type ImageQualityMultipliers map[string]float64

type ImageTierRule struct {
	TierLabel string   `json:"tier_label"`
	MaxPixels *int64   `json:"max_pixels,omitempty"`
	Price     *float64 `json:"price,omitempty"`
}

type ImageSizeInfo struct {
	Raw    string
	Width  int
	Height int
	Pixels int64
	Valid  bool
	Auto   bool
}

type ImageBillingResolution struct {
	UnitPrice   float64
	BillingTier string
	Strategy    ImageBillingStrategy
}

var openAIImageSizePattern = regexp.MustCompile(`^\s*(\d{1,6})\s*x\s*(\d{1,6})\s*$`)

func NormalizeImageBillingStrategy(strategy ImageBillingStrategy) ImageBillingStrategy {
	switch ImageBillingStrategy(strings.ToLower(strings.TrimSpace(string(strategy)))) {
	case ImageBillingStrategyMegapixel:
		return ImageBillingStrategyMegapixel
	default:
		return ImageBillingStrategyTier
	}
}

func NormalizeImageQuality(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(raw))
	case "auto":
		return "auto"
	default:
		return "auto"
	}
}

func ParseImageSize(raw string) ImageSizeInfo {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "auto") {
		return ImageSizeInfo{Raw: trimmed, Auto: true}
	}
	matches := openAIImageSizePattern.FindStringSubmatch(trimmed)
	if len(matches) != 3 {
		return ImageSizeInfo{Raw: trimmed}
	}
	width, errW := strconv.Atoi(matches[1])
	height, errH := strconv.Atoi(matches[2])
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return ImageSizeInfo{Raw: trimmed}
	}
	return ImageSizeInfo{
		Raw:    trimmed,
		Width:  width,
		Height: height,
		Pixels: int64(width) * int64(height),
		Valid:  true,
	}
}

func DefaultImageTierRules(price1K, price2K, price4K *float64) []ImageTierRule {
	return []ImageTierRule{
		{TierLabel: ImageBillingTier1K, MaxPixels: imageBillingInt64Ptr(1048576), Price: price1K},
		{TierLabel: ImageBillingTier2K, MaxPixels: imageBillingInt64Ptr(2359296), Price: price2K},
		{TierLabel: ImageBillingTier4K, MaxPixels: nil, Price: price4K},
	}
}

func ResolveImageBilling(resolved *ResolvedPricing, size ImageSizeInfo, quality string) ImageBillingResolution {
	if resolved == nil {
		return ImageBillingResolution{}
	}
	strategy := NormalizeImageBillingStrategy(resolved.ImageBillingStrategy)
	if strategy == ImageBillingStrategyMegapixel {
		if price := resolveMegapixelPrice(resolved, quality); size.Valid && price > 0 {
			return ImageBillingResolution{
				UnitPrice:   (float64(size.Pixels) / 1000000.0) * price,
				BillingTier: ImageBillingTierMegapixel,
				Strategy:    ImageBillingStrategyMegapixel,
			}
		}
		return ImageBillingResolution{Strategy: ImageBillingStrategyMegapixel}
	}

	if size.Valid {
		for _, rule := range normalizedImageTierRules(resolved.ImageTierRules, resolved.RequestTiers) {
			if rule.Price == nil || *rule.Price <= 0 {
				continue
			}
			if rule.MaxPixels == nil || size.Pixels <= *rule.MaxPixels {
				multiplier := resolveImageQualityMultiplier(resolved, quality)
				return ImageBillingResolution{
					UnitPrice:   *rule.Price * multiplier,
					BillingTier: rule.TierLabel,
					Strategy:    ImageBillingStrategyTier,
				}
			}
		}
	}

	return ImageBillingResolution{Strategy: ImageBillingStrategyTier}
}

func ParseImageQualityPricesJSON(raw string) ImageQualityPrices {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var parsed map[string]float64
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	prices := make(ImageQualityPrices, len(parsed))
	for key, value := range parsed {
		quality := NormalizeImageQuality(key)
		if quality == "" || value < 0 {
			continue
		}
		prices[quality] = value
	}
	if len(prices) == 0 {
		return nil
	}
	return prices
}

func ParseImageQualityMultipliersJSON(raw string) ImageQualityMultipliers {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var parsed map[string]float64
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	multipliers := make(ImageQualityMultipliers, len(parsed))
	for key, value := range parsed {
		quality := NormalizeImageQuality(key)
		if quality == "" || value < 0 {
			continue
		}
		multipliers[quality] = value
	}
	if len(multipliers) == 0 {
		return nil
	}
	return multipliers
}

func ParseImageTierRulesJSON(raw string) []ImageTierRule {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var rules []ImageTierRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil
	}
	return normalizeImageTierRules(rules)
}

func ImageQualityPricesJSON(prices ImageQualityPrices) *string {
	if len(prices) == 0 {
		return nil
	}
	normalized := make(map[string]float64, len(prices))
	for quality, price := range prices {
		normalized[NormalizeImageQuality(quality)] = price
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil
	}
	value := string(data)
	return &value
}

func ImageQualityMultipliersJSON(multipliers ImageQualityMultipliers) *string {
	if len(multipliers) == 0 {
		return nil
	}
	normalized := make(map[string]float64, len(multipliers))
	for quality, multiplier := range multipliers {
		normalized[NormalizeImageQuality(quality)] = multiplier
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil
	}
	value := string(data)
	return &value
}

func ImageTierRulesJSON(rules []ImageTierRule) *string {
	rules = normalizeImageTierRules(rules)
	if len(rules) == 0 {
		return nil
	}
	data, err := json.Marshal(rules)
	if err != nil {
		return nil
	}
	value := string(data)
	return &value
}

func resolveMegapixelPrice(resolved *ResolvedPricing, quality string) float64 {
	quality = NormalizeImageQuality(quality)
	if resolved.ImageQualityPrices != nil {
		if price, ok := resolved.ImageQualityPrices[quality]; ok && price > 0 {
			return price
		}
	}
	return resolved.ImageMegapixelPrice
}

func resolveImageQualityMultiplier(resolved *ResolvedPricing, quality string) float64 {
	quality = NormalizeImageQuality(quality)
	if resolved.ImageQualityMultipliers != nil {
		if multiplier, ok := resolved.ImageQualityMultipliers[quality]; ok && multiplier >= 0 {
			return multiplier
		}
	}
	// OpenAI image quality defaults to auto when omitted. Keep auto at 1.0 so
	// legacy tier pricing remains unchanged unless an administrator opts in.
	return 1.0
}

func normalizedImageTierRules(custom []ImageTierRule, fallback []PricingInterval) []ImageTierRule {
	if len(custom) > 0 {
		return normalizeImageTierRules(custom)
	}
	rules := make([]ImageTierRule, 0, len(fallback))
	for _, tier := range fallback {
		label := strings.TrimSpace(tier.TierLabel)
		if label == "" {
			continue
		}
		rules = append(rules, ImageTierRule{
			TierLabel: label,
			MaxPixels: tier.MaxTokensInt64(),
			Price:     tier.PerRequestPrice,
		})
	}
	return normalizeImageTierRules(rules)
}

func normalizeImageTierRules(rules []ImageTierRule) []ImageTierRule {
	out := make([]ImageTierRule, 0, len(rules))
	for _, rule := range rules {
		label := strings.ToUpper(strings.TrimSpace(rule.TierLabel))
		if label == "" {
			continue
		}
		if rule.MaxPixels != nil && *rule.MaxPixels <= 0 {
			continue
		}
		out = append(out, ImageTierRule{
			TierLabel: label,
			MaxPixels: rule.MaxPixels,
			Price:     rule.Price,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := maxPixelsSortValue(out[i].MaxPixels)
		right := maxPixelsSortValue(out[j].MaxPixels)
		if left == right {
			return out[i].TierLabel < out[j].TierLabel
		}
		return left < right
	})
	return out
}

func maxPixelsSortValue(v *int64) int64 {
	if v == nil {
		return int64(^uint64(0) >> 1)
	}
	return *v
}

func imageBillingInt64Ptr(v int64) *int64 {
	return &v
}

func (iv PricingInterval) MaxTokensInt64() *int64 {
	if iv.MaxTokens == nil {
		return nil
	}
	value := int64(*iv.MaxTokens)
	return &value
}
