package service

import (
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

func (m *DisplayTokenMultipliers) IsNonTrivial() bool {
	return m.InputMult != 1.0 || m.OutputMult != 1.0 || m.CacheReadMult != 1.0 || m.CacheCreateMult != 1.0
}

func SetDisplayTokenMultipliers(c *gin.Context, m *DisplayTokenMultipliers) {
	if m != nil && m.IsNonTrivial() {
		c.Set(displayTokenMultipliersKey, m)
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

// ComputeDisplayTokenMultipliers computes effective per-token-type multipliers
// that replicate the admin panel's display transform chain.
func (s *GatewayService) ComputeDisplayTokenMultipliers(
	c *gin.Context,
	model string,
	accountRateMultiplier float64,
	rateMultiplier float64,
	displayRateMultiplier float64,
) *DisplayTokenMultipliers {
	mult := &DisplayTokenMultipliers{
		InputMult:       1.0,
		OutputMult:      1.0,
		CacheReadMult:   1.0,
		CacheCreateMult: 1.0,
	}

	// Layer 1: display pricing (real_price * account_rate / display_price)
	if s.resolver != nil && s.resolver.globalPricingCache != nil {
		pricing := s.resolver.globalPricingCache.Get(model)
		if pricing != nil {
			if pricing.InputPrice != nil && *pricing.InputPrice > 0 &&
				pricing.DisplayInputPrice != nil && *pricing.DisplayInputPrice > 0 {
				mult.InputMult = (*pricing.InputPrice * accountRateMultiplier) / *pricing.DisplayInputPrice
			} else {
				mult.InputMult = accountRateMultiplier
			}

			if pricing.OutputPrice != nil && *pricing.OutputPrice > 0 &&
				pricing.DisplayOutputPrice != nil && *pricing.DisplayOutputPrice > 0 {
				mult.OutputMult = (*pricing.OutputPrice * accountRateMultiplier) / *pricing.DisplayOutputPrice
			} else {
				mult.OutputMult = accountRateMultiplier
			}

			if pricing.CacheReadPrice != nil && *pricing.CacheReadPrice > 0 &&
				pricing.DisplayCacheReadPrice != nil && *pricing.DisplayCacheReadPrice > 0 {
				mult.CacheReadMult = (*pricing.CacheReadPrice * accountRateMultiplier) / *pricing.DisplayCacheReadPrice
			} else {
				mult.CacheReadMult = accountRateMultiplier
			}

			mult.CacheCreateMult = accountRateMultiplier
		} else {
			mult.InputMult = accountRateMultiplier
			mult.OutputMult = accountRateMultiplier
			mult.CacheReadMult = accountRateMultiplier
			mult.CacheCreateMult = accountRateMultiplier
		}
	} else {
		mult.InputMult = accountRateMultiplier
		mult.OutputMult = accountRateMultiplier
		mult.CacheReadMult = accountRateMultiplier
		mult.CacheCreateMult = accountRateMultiplier
	}

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
