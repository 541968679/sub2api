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
	InputMult          float64
	OutputMult         float64
	CacheReadMult      float64
	CacheCreateMult    float64
	CacheReadInputMult float64
	// CacheCreate5mMult/CacheCreate1hMult 是缓存创建的分档倍率（真实档价 ÷ 展示创建价），
	// 响应携带嵌套 5m/1h 明细时按档反算，使下游展示与 usage-log 的
	// 成本反算口径（真实成本 ÷ 展示价）逐 token 一致；无明细时退化为 CacheCreateMult。
	CacheCreate5mMult float64
	CacheCreate1hMult float64
	RateScale         float64
	RateScaleSet      bool
}

type displayTokenPricingConfig struct {
	InputPrice                  float64
	OutputPrice                 float64
	CacheReadPrice              float64
	CacheCreationPrice          float64
	CacheCreation5mPrice        float64
	CacheCreation1hPrice        float64
	SupportsCacheBreakdown      bool
	DisplayInputPrice           *float64
	DisplayOutputPrice          *float64
	DisplayCacheReadPrice       *float64
	DisplayCacheCreationPrice   *float64 // 5m 档展示价
	DisplayCacheCreation1hPrice *float64 // 1h 档展示价（nil = 回退 5m 档展示价）
}

type displayTokenMultiplierProvider interface {
	ComputeDisplayTokenMultipliers(ctx context.Context, model string, userID int64, groupID *int64, rateMultiplier float64, displayRateMultiplier float64) *DisplayTokenMultipliers
	GetUserGroupRateMultiplier(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) float64
	GetUserGroupDisplayRateMultiplier(ctx context.Context, userID, groupID int64, fallback float64) float64
}

func (m *DisplayTokenMultipliers) IsNonTrivial() bool {
	return m.InputMult != 1.0 ||
		m.OutputMult != 1.0 ||
		m.CacheReadMult != 1.0 ||
		m.CacheCreateMult != 1.0 ||
		m.cacheCreate5mMultOrDefault() != 1.0 ||
		m.cacheCreate1hMultOrDefault() != 1.0 ||
		m.CacheReadInputMult != 0 ||
		displayTokenRateScale(m) != 1.0
}

// cacheCreate5mMultOrDefault 返回 5m 档倍率；未设置（零值）时退回 CacheCreateMult，
// 保证手工构造的 multipliers（旧测试/调用方）行为不变。
func (m *DisplayTokenMultipliers) cacheCreate5mMultOrDefault() float64 {
	if m == nil || m.CacheCreate5mMult == 0 {
		if m == nil {
			return 1.0
		}
		return m.CacheCreateMult
	}
	return m.CacheCreate5mMult
}

func (m *DisplayTokenMultipliers) cacheCreate1hMultOrDefault() float64 {
	if m == nil || m.CacheCreate1hMult == 0 {
		if m == nil {
			return 1.0
		}
		return m.CacheCreateMult
	}
	return m.CacheCreate1hMult
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

// ComputeDisplayTokenMultipliers computes the same two-layer token transform used
// by usage-log display: model display pricing first, then group display rate.
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
		InputMult:         1.0,
		OutputMult:        1.0,
		CacheReadMult:     1.0,
		CacheCreateMult:   1.0,
		CacheCreate5mMult: 1.0,
		CacheCreate1hMult: 1.0,
		RateScale:         1.0,
		RateScaleSet:      true,
	}

	// Layer 1: display pricing. Cache-read tokens stay on the cache line, and
	// any lower display cache price is balanced by moving the cache premium into
	// display input tokens, matching handler/dto.ApplyDisplayTransform.
	pricing := resolveDisplayTokenPricing(ctx, model, userID, groupID, resolver)
	mult.InputMult = displayTokenMultiplier(pricing.InputPrice, pricing.DisplayInputPrice)
	mult.OutputMult = displayTokenMultiplier(pricing.OutputPrice, pricing.DisplayOutputPrice)
	mult.CacheReadInputMult = displayCacheReadInputPremiumMultiplier(
		pricing.CacheReadPrice,
		pricing.DisplayCacheReadPrice,
		pricing.DisplayInputPrice,
	)

	// Cache creation: tokens are back-computed directly at the display price
	// (matching dto.ApplyDisplayTransform's cost ÷ display-price semantics).
	// 分档倍率与计费公式对齐（computeCacheCreationCost）：无明细回退按 5m 档价。
	creationBase := pricing.CacheCreationPrice
	if pricing.SupportsCacheBreakdown && pricing.CacheCreation5mPrice > 0 {
		creationBase = pricing.CacheCreation5mPrice
	}
	price5m := pricing.CacheCreation5mPrice
	if price5m <= 0 {
		price5m = creationBase
	}
	price1h := pricing.CacheCreation1hPrice
	if price1h <= 0 {
		price1h = price5m
	}
	display1hPrice := pricing.DisplayCacheCreationPrice
	if pricing.DisplayCacheCreation1hPrice != nil && *pricing.DisplayCacheCreation1hPrice > 0 {
		display1hPrice = pricing.DisplayCacheCreation1hPrice
	}
	mult.CacheCreateMult = displayTokenMultiplier(creationBase, pricing.DisplayCacheCreationPrice)
	mult.CacheCreate5mMult = displayTokenMultiplier(price5m, pricing.DisplayCacheCreationPrice)
	mult.CacheCreate1hMult = displayTokenMultiplier(price1h, display1hPrice)

	// Layer 2: user group display rate (rate_multiplier / display_rate_multiplier)
	if displayRateMultiplier > 0 && displayRateMultiplier != rateMultiplier {
		mult.RateScale = rateMultiplier / displayRateMultiplier
	}

	return mult
}

//nolint:unused // Kept as a GatewayService-bound adapter for callers that need service-owned pricing resolution.
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
		cfg.CacheCreationPrice = resolved.BasePricing.CacheCreationPricePerToken
		cfg.CacheCreation5mPrice = resolved.BasePricing.CacheCreation5mPrice
		cfg.CacheCreation1hPrice = resolved.BasePricing.CacheCreation1hPrice
		cfg.SupportsCacheBreakdown = resolved.BasePricing.SupportsCacheBreakdown
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
	if cfg.CacheCreationPrice <= 0 && pricing.CacheWritePrice != nil {
		cfg.CacheCreationPrice = *pricing.CacheWritePrice
		if cfg.CacheCreation5mPrice <= 0 {
			cfg.CacheCreation5mPrice = *pricing.CacheWritePrice
		}
		if cfg.CacheCreation1hPrice <= 0 {
			cfg.CacheCreation1hPrice = *pricing.CacheWritePrice
		}
	}
	if pricing.CacheWrite1hPrice != nil && *pricing.CacheWrite1hPrice > 0 {
		cfg.CacheCreation1hPrice = *pricing.CacheWrite1hPrice
		cfg.SupportsCacheBreakdown = true
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
	if pricing.DisplayCacheCreationPrice != nil {
		cfg.DisplayCacheCreationPrice = pricing.DisplayCacheCreationPrice
	}
	if pricing.DisplayCacheCreation1hPrice != nil {
		cfg.DisplayCacheCreation1hPrice = pricing.DisplayCacheCreation1hPrice
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
	if cfg.CacheCreationPrice <= 0 && pricing.CacheWritePrice != nil {
		cfg.CacheCreationPrice = *pricing.CacheWritePrice
		if cfg.CacheCreation5mPrice <= 0 {
			cfg.CacheCreation5mPrice = *pricing.CacheWritePrice
		}
		if cfg.CacheCreation1hPrice <= 0 {
			cfg.CacheCreation1hPrice = *pricing.CacheWritePrice
		}
	}
	if pricing.CacheWrite1hPrice != nil && *pricing.CacheWrite1hPrice > 0 {
		cfg.CacheCreation1hPrice = *pricing.CacheWrite1hPrice
		cfg.SupportsCacheBreakdown = true
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
	if pricing.DisplayCacheCreationPrice != nil {
		cfg.DisplayCacheCreationPrice = pricing.DisplayCacheCreationPrice
	}
	if pricing.DisplayCacheCreation1hPrice != nil {
		cfg.DisplayCacheCreation1hPrice = pricing.DisplayCacheCreation1hPrice
	}
}

func displayTokenMultiplier(realPrice float64, displayPrice *float64) float64 {
	if realPrice > 0 && displayPrice != nil && *displayPrice > 0 {
		return realPrice / *displayPrice
	}
	return 1.0
}

func displayCacheReadInputPremiumMultiplier(realCacheReadPrice float64, displayCacheReadPrice *float64, displayInputPrice *float64) float64 {
	if realCacheReadPrice <= 0 ||
		displayCacheReadPrice == nil || *displayCacheReadPrice <= 0 ||
		displayInputPrice == nil || *displayInputPrice <= 0 {
		return 0
	}
	premium := realCacheReadPrice - *displayCacheReadPrice
	if premium <= 0 {
		return 0
	}
	return premium / *displayInputPrice
}

func displayTokenRateScale(mult *DisplayTokenMultipliers) float64 {
	if mult == nil || !mult.RateScaleSet {
		return 1.0
	}
	return mult.RateScale
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

	modified := rewriteSeparatedUsageTokens([]byte(data), usagePath, mult)

	return dataPrefix + string(modified)
}

// computeDisplayCacheCreationBreakdown 按分档倍率反算缓存创建的展示 token。
// 有 5m/1h 明细时逐档反算（displayTotal×展示价 == 5m×p5m + 1h×p1h，与
// usage-log 的成本反算口径一致），display1h 用减法导出保证 5m+1h==total；
// 无明细时退化为单一 CacheCreateMult。RateScale（展示倍率层）在最后复合。
func computeDisplayCacheCreationBreakdown(total, cc5m, cc1h int, mult *DisplayTokenMultipliers) (int, int, int) {
	if total < 0 {
		total = 0
	}
	if cc5m < 0 {
		cc5m = 0
	}
	if cc1h < 0 {
		cc1h = 0
	}
	rateScale := displayTokenRateScale(mult)
	if cc5m <= 0 && cc1h <= 0 {
		displayTotal := roundDisplayTokenCount(total, mult.CacheCreateMult)
		if rateScale != 1.0 {
			displayTotal = roundDisplayTokenCount(displayTotal, rateScale)
		}
		return displayTotal, 0, 0
	}

	mult5m := mult.cacheCreate5mMultOrDefault()
	mult1h := mult.cacheCreate1hMultOrDefault()
	displayTotalRaw := float64(cc5m)*mult5m + float64(cc1h)*mult1h
	display5mRaw := float64(cc5m) * mult5m
	if rateScale != 1.0 {
		displayTotalRaw *= rateScale
		display5mRaw *= rateScale
	}
	displayTotal := int(math.Round(displayTotalRaw))
	display5m := int(math.Round(display5mRaw))
	if display5m > displayTotal {
		display5m = displayTotal
	}
	if display5m < 0 {
		display5m = 0
	}
	if cc1h <= 0 {
		return displayTotal, displayTotal, 0
	}
	if cc5m <= 0 {
		return displayTotal, 0, displayTotal
	}
	return displayTotal, display5m, displayTotal - display5m
}

// ApplyDisplayMultipliersToUsageMap modifies a usage map in-place (for antigravity hook).
func ApplyDisplayMultipliersToUsageMap(m map[string]any, mult *DisplayTokenMultipliers) {
	if mult == nil || !mult.IsNonTrivial() {
		return
	}
	input, inputOK := usageMapInt(m, "input_tokens")
	output, outputOK := usageMapInt(m, "output_tokens")
	cacheRead, cacheReadOK := usageMapInt(m, "cache_read_input_tokens")
	cacheCreate, cacheCreateOK := usageMapInt(m, "cache_creation_input_tokens")
	displayInput, displayOutput, displayCacheRead, displayCacheCreate := computeSeparatedDisplayUsage(input, output, cacheRead, cacheCreate, mult)
	// 嵌套 5m/1h 明细：按档反算并同步回写，保持 5m+1h==顶层总量。
	// antigravity 的 usage map 没有嵌套对象，此分支天然不触发。
	if ccObj, ok := m["cache_creation"].(map[string]any); ok {
		cc5m, _ := usageMapInt(ccObj, "ephemeral_5m_input_tokens")
		cc1h, _ := usageMapInt(ccObj, "ephemeral_1h_input_tokens")
		if cc5m > 0 || cc1h > 0 {
			total, d5m, d1h := computeDisplayCacheCreationBreakdown(cacheCreate, cc5m, cc1h, mult)
			displayCacheCreate = total
			ccObj["ephemeral_5m_input_tokens"] = d5m
			ccObj["ephemeral_1h_input_tokens"] = d1h
		}
	}
	if inputOK {
		m["input_tokens"] = displayInput
	}
	if outputOK {
		m["output_tokens"] = displayOutput
	}
	if cacheReadOK {
		m["cache_read_input_tokens"] = displayCacheRead
	}
	if cacheCreateOK {
		m["cache_creation_input_tokens"] = displayCacheCreate
	}
}

func usageMapInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	}
	return 0, false
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
	return rewriteSeparatedUsageTokens(body, "usage", mult)
}

func rewriteSeparatedUsageTokens(body []byte, usagePath string, mult *DisplayTokenMultipliers) []byte {
	if mult == nil || !mult.IsNonTrivial() || usagePath == "" {
		return body
	}
	if !gjson.GetBytes(body, usagePath).Exists() {
		return body
	}

	inputPath := usagePath + ".input_tokens"
	outputPath := usagePath + ".output_tokens"
	cacheReadPath := usagePath + ".cache_read_input_tokens"
	cacheCreatePath := usagePath + ".cache_creation_input_tokens"
	cacheCreate5mPath := usagePath + ".cache_creation.ephemeral_5m_input_tokens"
	cacheCreate1hPath := usagePath + ".cache_creation.ephemeral_1h_input_tokens"

	inputNode := gjson.GetBytes(body, inputPath)
	outputNode := gjson.GetBytes(body, outputPath)
	cacheReadNode := gjson.GetBytes(body, cacheReadPath)
	cacheCreateNode := gjson.GetBytes(body, cacheCreatePath)
	cacheCreate5mNode := gjson.GetBytes(body, cacheCreate5mPath)
	cacheCreate1hNode := gjson.GetBytes(body, cacheCreate1hPath)

	displayInput, displayOutput, displayCacheRead, displayCacheCreate := computeSeparatedDisplayUsage(
		int(inputNode.Int()),
		int(outputNode.Int()),
		int(cacheReadNode.Int()),
		int(cacheCreateNode.Int()),
		mult,
	)

	var err error
	// 嵌套 5m/1h 明细：按档反算，顶层与嵌套同步改写，保持 5m+1h==总量。
	if (cacheCreate5mNode.Exists() || cacheCreate1hNode.Exists()) &&
		(cacheCreate5mNode.Int() > 0 || cacheCreate1hNode.Int() > 0) {
		total, d5m, d1h := computeDisplayCacheCreationBreakdown(
			int(cacheCreateNode.Int()),
			int(cacheCreate5mNode.Int()),
			int(cacheCreate1hNode.Int()),
			mult,
		)
		displayCacheCreate = total
		if cacheCreate5mNode.Exists() {
			body, err = sjson.SetBytes(body, cacheCreate5mPath, d5m)
			if err != nil {
				return body
			}
		}
		if cacheCreate1hNode.Exists() {
			body, err = sjson.SetBytes(body, cacheCreate1hPath, d1h)
			if err != nil {
				return body
			}
		}
	}
	if inputNode.Exists() {
		body, err = sjson.SetBytes(body, inputPath, displayInput)
		if err != nil {
			return body
		}
	}
	if outputNode.Exists() {
		body, err = sjson.SetBytes(body, outputPath, displayOutput)
		if err != nil {
			return body
		}
	}
	if cacheReadNode.Exists() {
		body, err = sjson.SetBytes(body, cacheReadPath, displayCacheRead)
		if err != nil {
			return body
		}
	}
	if cacheCreateNode.Exists() {
		body, err = sjson.SetBytes(body, cacheCreatePath, displayCacheCreate)
		if err != nil {
			return body
		}
	}
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
	// cache creation 与其它字段同规则缩放（claude-gpt 桥接路径该值恒 0，属 no-op）。
	_, _, _, displayCacheCreate := computeSeparatedDisplayUsage(0, 0, 0, usage.CacheCreationInputTokens, mult)
	return OpenAIUsage{
		InputTokens:              displayInput,
		OutputTokens:             displayOutput,
		CacheCreationInputTokens: displayCacheCreate,
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
	displayNonCachedInput, displayOutput, displayCached, _ := computeSeparatedDisplayUsage(nonCachedInput, outputTokens, cachedTokens, 0, mult)
	displayInput := displayNonCachedInput + displayCached
	return displayInput, displayOutput, displayCached
}

func computeSeparatedDisplayUsage(inputTokens int, outputTokens int, cacheReadTokens int, cacheCreateTokens int, mult *DisplayTokenMultipliers) (int, int, int, int) {
	if mult == nil || !mult.IsNonTrivial() {
		return inputTokens, outputTokens, cacheReadTokens, cacheCreateTokens
	}
	if inputTokens < 0 {
		inputTokens = 0
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	if cacheReadTokens < 0 {
		cacheReadTokens = 0
	}
	if cacheCreateTokens < 0 {
		cacheCreateTokens = 0
	}

	displayInputRaw := float64(inputTokens) * mult.InputMult
	if inputTokens > 0 {
		displayInputRaw += float64(cacheReadTokens) * mult.CacheReadInputMult
	}
	displayInput := int(math.Round(displayInputRaw))
	displayOutput := roundDisplayTokenCount(outputTokens, mult.OutputMult)
	displayCacheRead := roundDisplayTokenCount(cacheReadTokens, mult.CacheReadMult)
	displayCacheCreate := roundDisplayTokenCount(cacheCreateTokens, mult.CacheCreateMult)

	rateScale := displayTokenRateScale(mult)
	if rateScale != 1.0 {
		displayInput = roundDisplayTokenCount(displayInput, rateScale)
		displayOutput = roundDisplayTokenCount(displayOutput, rateScale)
		// Keep cache-read usage counts real; display-rate scaling is reflected in cost, not this response field.
		displayCacheCreate = roundDisplayTokenCount(displayCacheCreate, rateScale)
	}

	return displayInput, displayOutput, displayCacheRead, displayCacheCreate
}

func roundDisplayTokenCount(tokens int, multiplier float64) int {
	if tokens <= 0 {
		return tokens
	}
	return int(math.Round(float64(tokens) * multiplier))
}
