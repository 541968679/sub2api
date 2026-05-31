package service

import (
	"context"
	"math"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const displayTokenMultipliersKey = "display_token_multipliers"

type DisplayTokenMultipliers struct {
	InputMult       float64
	OutputMult      float64
	CacheReadMult   float64
	CacheCreateMult float64
}

type displayTokenPricingConfig struct {
	InputPrice            float64
	OutputPrice           float64
	CacheReadPrice        float64
	DisplayInputPrice     *float64
	DisplayOutputPrice    *float64
	DisplayCacheReadPrice *float64
}

func (m *DisplayTokenMultipliers) IsNonTrivial() bool {
	return m.InputMult != 1.0 || m.OutputMult != 1.0 || m.CacheReadMult != 1.0 || m.CacheCreateMult != 1.0
}

func SetDisplayTokenMultipliers(c *gin.Context, m *DisplayTokenMultipliers) {
	if c == nil {
		return
	}
	if m != nil && m.IsNonTrivial() {
		c.Set(displayTokenMultipliersKey, m)
		return
	}
	if c.Keys != nil {
		delete(c.Keys, displayTokenMultipliersKey)
	}
}

func getDisplayTokenMultipliers(c *gin.Context) *DisplayTokenMultipliers {
	if c == nil {
		return nil
	}
	v, exists := c.Get(displayTokenMultipliersKey)
	if !exists {
		return nil
	}
	m, _ := v.(*DisplayTokenMultipliers)
	return m
}

func (s *GatewayService) MaybeSetDisplayTokenMultipliers(ctx context.Context, c *gin.Context, apiKey *APIKey, model string) {
	if s == nil || apiKey == nil || apiKey.User == nil {
		return
	}
	if NormalizeDownstreamUsageTokenMode(apiKey.User.DownstreamUsageTokenMode) != DownstreamUsageTokenModeDisplay {
		return
	}

	userID := apiKey.User.ID
	if userID == 0 {
		userID = apiKey.UserID
	}
	rateMultiplier := 1.0
	displayRateMultiplier := 1.0
	var groupID *int64
	if apiKey.GroupID != nil && *apiKey.GroupID > 0 {
		groupID = apiKey.GroupID
		if apiKey.Group != nil {
			rateMultiplier = s.GetUserGroupRateMultiplier(ctx, userID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
			displayRateMultiplier = s.GetUserGroupDisplayRateMultiplier(ctx, userID, *apiKey.GroupID, rateMultiplier)
		}
	}

	mult := s.ComputeDisplayTokenMultipliers(ctx, model, userID, groupID, rateMultiplier, displayRateMultiplier)
	SetDisplayTokenMultipliers(c, mult)
}

// ComputeDisplayTokenMultipliers computes effective per-token-type multipliers
// that replicate the admin panel's display transform chain.
func (s *GatewayService) ComputeDisplayTokenMultipliers(
	ctx context.Context,
	model string,
	userID int64,
	groupID *int64,
	rateMultiplier float64,
	displayRateMultiplier float64,
) *DisplayTokenMultipliers {
	mult := &DisplayTokenMultipliers{
		InputMult:       1.0,
		OutputMult:      1.0,
		CacheReadMult:   1.0,
		CacheCreateMult: 1.0,
	}

	// Layer 1: display pricing (real_price / display_price)
	pricing := s.resolveDisplayTokenPricing(ctx, model, userID, groupID)
	mult.InputMult = displayTokenMultiplier(pricing.InputPrice, pricing.DisplayInputPrice)
	mult.OutputMult = displayTokenMultiplier(pricing.OutputPrice, pricing.DisplayOutputPrice)
	mult.CacheReadMult = displayTokenMultiplier(pricing.CacheReadPrice, pricing.DisplayCacheReadPrice)

	// Layer 2: user group display rate (rate_multiplier / display_rate_multiplier)
	if displayRateMultiplier > 0 && displayRateMultiplier != rateMultiplier {
		scale := rateMultiplier / displayRateMultiplier
		mult.InputMult *= scale
		mult.OutputMult *= scale
		mult.CacheReadMult *= scale
		mult.CacheCreateMult *= scale
	}

	return mult
}

func (s *GatewayService) resolveDisplayTokenPricing(ctx context.Context, model string, userID int64, groupID *int64) displayTokenPricingConfig {
	if ctx == nil {
		ctx = context.Background()
	}
	var cfg displayTokenPricingConfig
	if s == nil {
		return cfg
	}

	var userIDPtr *int64
	if userID > 0 {
		userIDPtr = &userID
	}
	if s.resolver != nil {
		resolved := s.resolver.Resolve(ctx, PricingInput{Model: model, GroupID: groupID, UserID: userIDPtr})
		if resolved != nil && resolved.BasePricing != nil {
			cfg.InputPrice = resolved.BasePricing.InputPricePerToken
			cfg.OutputPrice = resolved.BasePricing.OutputPricePerToken
			cfg.CacheReadPrice = resolved.BasePricing.CacheReadPricePerToken
		}
		if s.resolver.globalPricingCache != nil {
			if gp := s.resolver.globalPricingCache.Get(model); gp != nil {
				mergeGlobalDisplayTokenPricing(&cfg, gp)
			}
		}
		if userID > 0 && s.resolver.userModelPricingRepo != nil {
			if override, err := s.resolver.userModelPricingRepo.GetByUserAndModel(ctx, userID, model); err == nil && override != nil && override.Enabled {
				mergeUserDisplayTokenPricing(&cfg, override)
			}
		}
	}
	return cfg
}

func mergeGlobalDisplayTokenPricing(cfg *displayTokenPricingConfig, pricing *GlobalModelPricing) {
	if cfg == nil || pricing == nil {
		return
	}
	if cfg.InputPrice <= 0 && pricing.InputPrice != nil {
		cfg.InputPrice = *pricing.InputPrice
	}
	if cfg.OutputPrice <= 0 && pricing.OutputPrice != nil {
		cfg.OutputPrice = *pricing.OutputPrice
	}
	if cfg.CacheReadPrice <= 0 && pricing.CacheReadPrice != nil {
		cfg.CacheReadPrice = *pricing.CacheReadPrice
	}
	if pricing.DisplayInputPrice != nil {
		cfg.DisplayInputPrice = pricing.DisplayInputPrice
	}
	if pricing.DisplayOutputPrice != nil {
		cfg.DisplayOutputPrice = pricing.DisplayOutputPrice
	}
	if pricing.DisplayCacheReadPrice != nil {
		cfg.DisplayCacheReadPrice = pricing.DisplayCacheReadPrice
	}
}

func mergeUserDisplayTokenPricing(cfg *displayTokenPricingConfig, pricing *UserModelPricingOverride) {
	if cfg == nil || pricing == nil {
		return
	}
	if cfg.InputPrice <= 0 && pricing.InputPrice != nil {
		cfg.InputPrice = *pricing.InputPrice
	}
	if cfg.OutputPrice <= 0 && pricing.OutputPrice != nil {
		cfg.OutputPrice = *pricing.OutputPrice
	}
	if cfg.CacheReadPrice <= 0 && pricing.CacheReadPrice != nil {
		cfg.CacheReadPrice = *pricing.CacheReadPrice
	}
	if pricing.DisplayInputPrice != nil {
		cfg.DisplayInputPrice = pricing.DisplayInputPrice
	}
	if pricing.DisplayOutputPrice != nil {
		cfg.DisplayOutputPrice = pricing.DisplayOutputPrice
	}
	if pricing.DisplayCacheReadPrice != nil {
		cfg.DisplayCacheReadPrice = pricing.DisplayCacheReadPrice
	}
}

func displayTokenMultiplier(realPrice float64, displayPrice *float64) float64 {
	if realPrice > 0 && displayPrice != nil && *displayPrice > 0 {
		return realPrice / *displayPrice
	}
	return 1.0
}

// RewriteSSEUsageTokens rewrites token fields in a Claude SSE data line.
// Only processes message_start and message_delta events (2 per stream).
func RewriteSSEUsageTokens(line string, mult *DisplayTokenMultipliers) string {
	if mult == nil || !mult.IsNonTrivial() {
		return line
	}

	dataPrefix := "data: "
	if !strings.HasPrefix(line, dataPrefix) {
		return line
	}
	data := line[len(dataPrefix):]

	eventType := gjson.Get(data, "type").String()
	if eventType != "message_start" && eventType != "message_delta" {
		return line
	}

	var usagePath string
	if eventType == "message_start" {
		usagePath = "message.usage"
	} else {
		usagePath = "usage"
	}

	usageNode := gjson.Get(data, usagePath)
	if !usageNode.Exists() {
		return line
	}

	modified := []byte(data)
	modified = rewriteTokenField(modified, usagePath+".input_tokens", mult.InputMult)
	modified = rewriteTokenField(modified, usagePath+".output_tokens", mult.OutputMult)
	modified = rewriteTokenField(modified, usagePath+".cache_read_input_tokens", mult.CacheReadMult)
	modified = rewriteTokenField(modified, usagePath+".cache_creation_input_tokens", mult.CacheCreateMult)

	return dataPrefix + string(modified)
}

func rewriteTokenField(json []byte, path string, multiplier float64) []byte {
	v := gjson.GetBytes(json, path)
	if !v.Exists() || v.Int() == 0 {
		return json
	}
	newVal := int(math.Round(float64(v.Int()) * multiplier))
	result, err := sjson.SetBytes(json, path, newVal)
	if err != nil {
		return json
	}
	return result
}

// ApplyDisplayMultipliersToUsageMap modifies a usage map in-place (for antigravity hook).
func ApplyDisplayMultipliersToUsageMap(m map[string]any, mult *DisplayTokenMultipliers) {
	if mult == nil || !mult.IsNonTrivial() {
		return
	}
	applyMapMult(m, "input_tokens", mult.InputMult)
	applyMapMult(m, "output_tokens", mult.OutputMult)
	applyMapMult(m, "cache_read_input_tokens", mult.CacheReadMult)
	applyMapMult(m, "cache_creation_input_tokens", mult.CacheCreateMult)
}
func applyMapMult(m map[string]any, key string, multiplier float64) {
	v, ok := m[key]
	if !ok {
		return
	}
	switch val := v.(type) {
	case int:
		if val > 0 {
			m[key] = int(math.Round(float64(val) * multiplier))
		}
	case int64:
		if val > 0 {
			m[key] = int(math.Round(float64(val) * multiplier))
		}
	case float64:
		if val > 0 {
			m[key] = int(math.Round(val * multiplier))
		}
	}
}

// rewriteNonStreamUsageTokens rewrites token fields in a non-streaming JSON response body.
func rewriteNonStreamUsageTokens(body []byte, mult *DisplayTokenMultipliers) []byte {
	if mult == nil || !mult.IsNonTrivial() {
		return body
	}
	usageNode := gjson.GetBytes(body, "usage")
	if !usageNode.Exists() {
		return body
	}
	body = rewriteTokenField(body, "usage.input_tokens", mult.InputMult)
	body = rewriteTokenField(body, "usage.output_tokens", mult.OutputMult)
	body = rewriteTokenField(body, "usage.cache_read_input_tokens", mult.CacheReadMult)
	body = rewriteTokenField(body, "usage.cache_creation_input_tokens", mult.CacheCreateMult)
	return body
}
