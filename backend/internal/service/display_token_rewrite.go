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

type displayTokenMultiplierProvider interface {
	ComputeDisplayTokenMultipliers(ctx context.Context, model string, userID int64, groupID *int64, rateMultiplier float64, displayRateMultiplier float64) *DisplayTokenMultipliers
	GetUserGroupRateMultiplier(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) float64
	GetUserGroupDisplayRateMultiplier(ctx context.Context, userID, groupID int64, fallback float64) float64
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
	if s == nil {
		return
	}
	maybeSetDisplayTokenMultipliers(ctx, c, apiKey, model, s)
}

func (s *OpenAIGatewayService) MaybeSetDisplayTokenMultipliers(ctx context.Context, c *gin.Context, apiKey *APIKey, model string) {
	if s == nil {
		return
	}
	maybeSetDisplayTokenMultipliers(ctx, c, apiKey, model, s)
}

func maybeSetDisplayTokenMultipliers(ctx context.Context, c *gin.Context, apiKey *APIKey, model string, provider displayTokenMultiplierProvider) {
	if provider == nil || apiKey == nil || apiKey.User == nil {
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
			rateMultiplier = provider.GetUserGroupRateMultiplier(ctx, userID, *apiKey.GroupID, apiKey.Group.RateMultiplier)
			displayRateMultiplier = provider.GetUserGroupDisplayRateMultiplier(ctx, userID, *apiKey.GroupID, rateMultiplier)
		}
	}

	mult := provider.ComputeDisplayTokenMultipliers(ctx, model, userID, groupID, rateMultiplier, displayRateMultiplier)
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
	var resolver *ModelPricingResolver
	if s != nil {
		resolver = s.resolver
	}
	return computeDisplayTokenMultipliers(ctx, model, userID, groupID, rateMultiplier, displayRateMultiplier, resolver)
}

func (s *OpenAIGatewayService) ComputeDisplayTokenMultipliers(
	ctx context.Context,
	model string,
	userID int64,
	groupID *int64,
	rateMultiplier float64,
	displayRateMultiplier float64,
) *DisplayTokenMultipliers {
	var resolver *ModelPricingResolver
	if s != nil {
		resolver = s.resolver
	}
	return computeDisplayTokenMultipliers(ctx, model, userID, groupID, rateMultiplier, displayRateMultiplier, resolver)
}

func computeDisplayTokenMultipliers(
	ctx context.Context,
	model string,
	userID int64,
	groupID *int64,
	rateMultiplier float64,
	displayRateMultiplier float64,
	resolver *ModelPricingResolver,
) *DisplayTokenMultipliers {
	mult := &DisplayTokenMultipliers{
		InputMult:       1.0,
		OutputMult:      1.0,
		CacheReadMult:   1.0,
		CacheCreateMult: 1.0,
	}

	// Layer 1: display pricing (real_price / display_price)
	pricing := resolveDisplayTokenPricing(ctx, model, userID, groupID, resolver)
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
	var resolver *ModelPricingResolver
	if s != nil {
		resolver = s.resolver
	}
	return resolveDisplayTokenPricing(ctx, model, userID, groupID, resolver)
}

func resolveDisplayTokenPricing(ctx context.Context, model string, userID int64, groupID *int64, resolver *ModelPricingResolver) displayTokenPricingConfig {
	if ctx == nil {
		ctx = context.Background()
	}
	var cfg displayTokenPricingConfig
	if resolver == nil {
		return cfg
	}

	var userIDPtr *int64
	if userID > 0 {
		userIDPtr = &userID
	}
	resolved := resolver.Resolve(ctx, PricingInput{Model: model, GroupID: groupID, UserID: userIDPtr})
	if resolved != nil && resolved.BasePricing != nil {
		cfg.InputPrice = resolved.BasePricing.InputPricePerToken
		cfg.OutputPrice = resolved.BasePricing.OutputPricePerToken
		cfg.CacheReadPrice = resolved.BasePricing.CacheReadPricePerToken
	}
	if resolver.globalPricingCache != nil {
		if gp := resolver.globalPricingCache.Get(model); gp != nil {
			mergeGlobalDisplayTokenPricing(&cfg, gp)
		}
	}
	if userID > 0 && resolver.userModelPricingRepo != nil {
		if override, err := resolver.userModelPricingRepo.GetByUserAndModel(ctx, userID, model); err == nil && override != nil && override.Enabled {
			mergeUserDisplayTokenPricing(&cfg, override)
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

func rewriteOpenAIResponsesUsageTokens(body []byte, usagePath string, mult *DisplayTokenMultipliers) []byte {
	if mult == nil || !mult.IsNonTrivial() || usagePath == "" {
		return body
	}
	usageNode := gjson.GetBytes(body, usagePath)
	if !usageNode.Exists() {
		return body
	}

	inputPath := usagePath + ".input_tokens"
	outputPath := usagePath + ".output_tokens"
	cachedPath := usagePath + ".input_tokens_details.cached_tokens"
	totalPath := usagePath + ".total_tokens"

	input := int(gjson.GetBytes(body, inputPath).Int())
	output := int(gjson.GetBytes(body, outputPath).Int())
	cached := int(gjson.GetBytes(body, cachedPath).Int())
	displayInput, displayOutput, displayCached := computeOpenAIDisplayUsage(input, output, cached, mult)

	var err error
	if gjson.GetBytes(body, inputPath).Exists() {
		body, err = sjson.SetBytes(body, inputPath, displayInput)
		if err != nil {
			return body
		}
	}
	if gjson.GetBytes(body, outputPath).Exists() {
		body, err = sjson.SetBytes(body, outputPath, displayOutput)
		if err != nil {
			return body
		}
	}
	if gjson.GetBytes(body, cachedPath).Exists() {
		body, err = sjson.SetBytes(body, cachedPath, displayCached)
		if err != nil {
			return body
		}
	}
	if gjson.GetBytes(body, totalPath).Exists() {
		body, err = sjson.SetBytes(body, totalPath, displayInput+displayOutput)
		if err != nil {
			return body
		}
	}
	return body
}

func rewriteOpenAIResponsesSSEUsageTokens(line string, mult *DisplayTokenMultipliers) string {
	if mult == nil || !mult.IsNonTrivial() {
		return line
	}
	data, ok := extractOpenAISSEDataLine(line)
	if !ok || data == "" || data == "[DONE]" {
		return line
	}
	eventType := gjson.Get(data, "type").String()
	switch eventType {
	case "response.completed", "response.done", "response.incomplete", "response.cancelled", "response.canceled":
	default:
		return line
	}
	if !gjson.Get(data, "response.usage").Exists() {
		return line
	}
	rewritten := rewriteOpenAIResponsesUsageTokens([]byte(data), "response.usage", mult)
	return "data: " + string(rewritten)
}

func rewriteOpenAIChatUsageTokens(body []byte, usagePath string, mult *DisplayTokenMultipliers) []byte {
	if mult == nil || !mult.IsNonTrivial() || usagePath == "" {
		return body
	}
	usageNode := gjson.GetBytes(body, usagePath)
	if !usageNode.Exists() {
		return body
	}

	inputPath := usagePath + ".prompt_tokens"
	outputPath := usagePath + ".completion_tokens"
	cachedPath := usagePath + ".prompt_tokens_details.cached_tokens"
	totalPath := usagePath + ".total_tokens"

	input := int(gjson.GetBytes(body, inputPath).Int())
	output := int(gjson.GetBytes(body, outputPath).Int())
	cached := int(gjson.GetBytes(body, cachedPath).Int())
	displayInput, displayOutput, displayCached := computeOpenAIDisplayUsage(input, output, cached, mult)

	var err error
	if gjson.GetBytes(body, inputPath).Exists() {
		body, err = sjson.SetBytes(body, inputPath, displayInput)
		if err != nil {
			return body
		}
	}
	if gjson.GetBytes(body, outputPath).Exists() {
		body, err = sjson.SetBytes(body, outputPath, displayOutput)
		if err != nil {
			return body
		}
	}
	if gjson.GetBytes(body, cachedPath).Exists() {
		body, err = sjson.SetBytes(body, cachedPath, displayCached)
		if err != nil {
			return body
		}
	}
	if gjson.GetBytes(body, totalPath).Exists() {
		body, err = sjson.SetBytes(body, totalPath, displayInput+displayOutput)
		if err != nil {
			return body
		}
	}
	return body
}

func applyOpenAIResponsesUsageDisplayMultipliers(usage *OpenAIUsage, mult *DisplayTokenMultipliers) OpenAIUsage {
	if usage == nil {
		return OpenAIUsage{}
	}
	displayInput, displayOutput, displayCached := computeOpenAIDisplayUsage(usage.InputTokens, usage.OutputTokens, usage.CacheReadInputTokens, mult)
	return OpenAIUsage{
		InputTokens:              displayInput,
		OutputTokens:             displayOutput,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
		CacheReadInputTokens:     displayCached,
		ImageOutputTokens:        usage.ImageOutputTokens,
	}
}

func computeOpenAIDisplayUsage(inputTokens int, outputTokens int, cachedTokens int, mult *DisplayTokenMultipliers) (int, int, int) {
	if mult == nil || !mult.IsNonTrivial() {
		return inputTokens, outputTokens, cachedTokens
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	nonCachedInput := inputTokens - cachedTokens
	if nonCachedInput < 0 {
		nonCachedInput = 0
	}
	displayCached := roundDisplayTokenCount(cachedTokens, mult.CacheReadMult)
	displayInput := roundDisplayTokenCount(nonCachedInput, mult.InputMult) + displayCached
	displayOutput := roundDisplayTokenCount(outputTokens, mult.OutputMult)
	return displayInput, displayOutput, displayCached
}

func roundDisplayTokenCount(tokens int, multiplier float64) int {
	if tokens <= 0 {
		return tokens
	}
	return int(math.Round(float64(tokens) * multiplier))
}
