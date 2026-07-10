//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type displayTokenUserModelPricingRepoStub struct {
	override *UserModelPricingOverride
}

func (s *displayTokenUserModelPricingRepoStub) GetByUserID(context.Context, int64) ([]UserModelPricingOverride, error) {
	return nil, nil
}

func (s *displayTokenUserModelPricingRepoStub) GetEnabledByUserID(context.Context, int64) ([]UserModelPricingOverride, error) {
	if s.override == nil || !s.override.Enabled {
		return nil, nil
	}
	return []UserModelPricingOverride{*s.override}, nil
}

func (s *displayTokenUserModelPricingRepoStub) GetByUserAndModel(context.Context, int64, string) (*UserModelPricingOverride, error) {
	return s.override, nil
}

func (s *displayTokenUserModelPricingRepoStub) GetByID(context.Context, int64) (*UserModelPricingOverride, error) {
	return nil, nil
}

func (s *displayTokenUserModelPricingRepoStub) Create(context.Context, *UserModelPricingOverride) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) Update(context.Context, *UserModelPricingOverride) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) DeleteByUserID(context.Context, int64) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) BatchUpsert(context.Context, int64, []UserModelPricingOverride) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) GetEnabledCountByModel(context.Context) (map[string]int, error) {
	return nil, nil
}

func (s *displayTokenUserModelPricingRepoStub) GetByModel(context.Context, string) ([]UserModelPricingOverride, error) {
	return nil, nil
}

func TestDisplayToken_MaybeSetDisplayTokenMultipliersGatedByUserMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	realModeKey := &APIKey{
		UserID: 1,
		User:   &User{ID: 1, DownstreamUsageTokenMode: DownstreamUsageTokenModeReal},
	}
	svc.MaybeSetDisplayTokenMultipliers(context.Background(), c, realModeKey, "claude-sonnet-4")
	require.Nil(t, getDisplayTokenMultipliers(c))

	displayModeKey := &APIKey{
		UserID: 1,
		User:   &User{ID: 1, DownstreamUsageTokenMode: DownstreamUsageTokenModeDisplay},
	}
	svc.MaybeSetDisplayTokenMultipliers(context.Background(), c, displayModeKey, "claude-sonnet-4")
	require.Nil(t, getDisplayTokenMultipliers(c))
}

func TestDisplayToken_ComputeMultipliersUsesUserDisplayPricingOverride(t *testing.T) {
	globalInput := 4e-6
	globalOutput := 8e-6
	globalCacheRead := 2e-6
	globalDisplayInput := 2e-6
	globalDisplayOutput := 4e-6
	globalDisplayCacheRead := 1e-6
	userDisplayInput := 1e-6
	userDisplayOutput := 2e-6
	userDisplayCacheRead := 0.5e-6

	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:                 "claude-sonnet-4",
		BillingMode:           BillingModeToken,
		InputPrice:            &globalInput,
		OutputPrice:           &globalOutput,
		CacheReadPrice:        &globalCacheRead,
		DisplayInputPrice:     &globalDisplayInput,
		DisplayOutputPrice:    &globalDisplayOutput,
		DisplayCacheReadPrice: &globalDisplayCacheRead,
		Enabled:               true,
	})
	userRepo := &displayTokenUserModelPricingRepoStub{
		override: &UserModelPricingOverride{
			UserID:                42,
			Model:                 "claude-sonnet-4",
			DisplayInputPrice:     &userDisplayInput,
			DisplayOutputPrice:    &userDisplayOutput,
			DisplayCacheReadPrice: &userDisplayCacheRead,
			Enabled:               true,
		},
	}
	resolver := NewModelPricingResolver(&ChannelService{}, newTestBillingServiceWithRichPricing(), cache, userRepo)
	svc := &GatewayService{resolver: resolver}

	mult := svc.ComputeDisplayTokenMultipliers(context.Background(), "claude-sonnet-4", 42, nil, 1, 1)
	require.NotNil(t, mult)
	require.InDelta(t, 4.0, mult.InputMult, 1e-12)
	require.InDelta(t, 4.0, mult.OutputMult, 1e-12)
	require.InDelta(t, 1.0, mult.CacheReadMult, 1e-12)
	require.InDelta(t, 1.5, mult.CacheReadInputMult, 1e-12)
	require.InDelta(t, 1.0, mult.CacheCreateMult, 1e-12)
}

func TestDisplayToken_LongContextEffectivePricesKeepTokenAmplificationInvariant(t *testing.T) {
	realInput := 10e-6
	realOutput := 60e-6
	realCacheRead := 1e-6
	displayInput := 5e-6
	displayOutput := 30e-6
	displayCacheRead := 0.5e-6
	userRepo := &displayTokenUserModelPricingRepoStub{
		override: &UserModelPricingOverride{
			UserID:                42,
			Model:                 "gpt-5.5",
			InputPrice:            &realInput,
			OutputPrice:           &realOutput,
			CacheReadPrice:        &realCacheRead,
			DisplayInputPrice:     &displayInput,
			DisplayOutputPrice:    &displayOutput,
			DisplayCacheReadPrice: &displayCacheRead,
			Enabled:               true,
		},
	}
	billing := &BillingService{
		pricingService: &PricingService{
			pricingData: map[string]*LiteLLMModelPricing{
				"gpt-5.5": {
					InputCostPerToken:       5e-6,
					OutputCostPerToken:      30e-6,
					CacheReadInputTokenCost: 0.5e-6,
				},
			},
		},
	}
	resolver := NewModelPricingResolver(&ChannelService{}, billing, nil, userRepo)

	mult := computeDisplayTokenMultipliers(context.Background(), "gpt-5.5", 42, nil, 2.0, 1.0, resolver)
	require.NotNil(t, mult)

	require.InDelta(t, 2.0, mult.InputMult, 1e-12)
	require.InDelta(t, 2.0, mult.OutputMult, 1e-12)
	require.InDelta(t, 1.0, mult.CacheReadMult, 1e-12)
	require.InDelta(t, 0.1, mult.CacheReadInputMult, 1e-12)
	require.InDelta(t, 2.0, displayTokenRateScale(mult), 1e-12)

	longInputMultiplier := 2.0
	longOutputMultiplier := 1.5
	longDisplayInput := displayInput * longInputMultiplier
	longDisplayOutput := displayOutput * longOutputMultiplier
	longDisplayCacheRead := displayCacheRead * longInputMultiplier
	require.InDelta(t, mult.InputMult, displayTokenMultiplier(realInput*longInputMultiplier, &longDisplayInput), 1e-12)
	require.InDelta(t, mult.OutputMult, displayTokenMultiplier(realOutput*longOutputMultiplier, &longDisplayOutput), 1e-12)
	require.InDelta(t, mult.CacheReadInputMult, displayCacheReadInputPremiumMultiplier(
		realCacheRead*longInputMultiplier,
		&longDisplayCacheRead,
		&longDisplayInput,
	), 1e-12)
}

func TestDisplayToken_EqualDisplayPriceDoesNotScaleTokens(t *testing.T) {
	input := 4e-6
	output := 8e-6
	cacheRead := 2e-6

	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:                 "claude-sonnet-4",
		BillingMode:           BillingModeToken,
		InputPrice:            &input,
		OutputPrice:           &output,
		CacheReadPrice:        &cacheRead,
		DisplayInputPrice:     &input,
		DisplayOutputPrice:    &output,
		DisplayCacheReadPrice: &cacheRead,
		Enabled:               true,
	})
	resolver := NewModelPricingResolver(&ChannelService{}, newTestBillingServiceWithRichPricing(), cache, nil)
	svc := &GatewayService{resolver: resolver}

	mult := svc.ComputeDisplayTokenMultipliers(context.Background(), "claude-sonnet-4", 42, nil, 1, 1)
	require.NotNil(t, mult)
	require.Equal(t, 1.0, mult.InputMult)
	require.Equal(t, 1.0, mult.OutputMult)
	require.Equal(t, 1.0, mult.CacheReadMult)
	require.Equal(t, 0.0, mult.CacheReadInputMult)
	require.Equal(t, 1.0, mult.CacheCreateMult)
	require.False(t, mult.IsNonTrivial())
}

func TestDisplayToken_GroupDisplayRateIsTheOnlyMultiplierLayer(t *testing.T) {
	svc := &GatewayService{}

	mult := svc.ComputeDisplayTokenMultipliers(context.Background(), "claude-sonnet-4", 42, nil, 1.0, 0.01)
	require.NotNil(t, mult)
	require.InDelta(t, 1.0, mult.InputMult, 1e-12)
	require.InDelta(t, 1.0, mult.OutputMult, 1e-12)
	require.InDelta(t, 1.0, mult.CacheReadMult, 1e-12)
	require.InDelta(t, 1.0, mult.CacheCreateMult, 1e-12)
	require.InDelta(t, 100.0, displayTokenRateScale(mult), 1e-12)
}

func TestDisplayToken_OpenAIResponsesUsageRewriteBalancesCachePremium(t *testing.T) {
	body := []byte(`{"id":"resp_1","usage":{"input_tokens":1000,"output_tokens":100,"total_tokens":1100,"input_tokens_details":{"cached_tokens":200},"output_tokens_details":{"reasoning_tokens":7}},"custom":"kept"}`)
	mult := &DisplayTokenMultipliers{
		InputMult:          2,
		OutputMult:         4,
		CacheReadMult:      1,
		CacheCreateMult:    1,
		CacheReadInputMult: 2,
	}

	rewritten := rewriteOpenAIResponsesUsageTokens(body, "usage", mult)
	require.Equal(t, int64(2200), gjson.GetBytes(rewritten, "usage.input_tokens").Int())
	require.Equal(t, int64(200), gjson.GetBytes(rewritten, "usage.input_tokens_details.cached_tokens").Int())
	require.Equal(t, int64(400), gjson.GetBytes(rewritten, "usage.output_tokens").Int())
	require.Equal(t, int64(2600), gjson.GetBytes(rewritten, "usage.total_tokens").Int())
	require.Equal(t, int64(7), gjson.GetBytes(rewritten, "usage.output_tokens_details.reasoning_tokens").Int())
	require.Equal(t, "kept", gjson.GetBytes(rewritten, "custom").String())
}

func TestDisplayToken_OpenAIResponsesUsageRewriteScalesCacheWriteTokens(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":100,"output_tokens":10,"total_tokens":110,"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":30},"output_tokens_details":{"reasoning_tokens":7}}}`)
	mult := &DisplayTokenMultipliers{
		InputMult:          2,
		OutputMult:         3,
		CacheReadMult:      1,
		CacheCreateMult:    1.5,
		CacheReadInputMult: 0,
	}

	rewritten := rewriteOpenAIResponsesUsageTokens(body, "usage", mult)

	require.Equal(t, int64(165), gjson.GetBytes(rewritten, "usage.input_tokens").Int())
	require.Equal(t, int64(20), gjson.GetBytes(rewritten, "usage.input_tokens_details.cached_tokens").Int())
	require.Equal(t, int64(45), gjson.GetBytes(rewritten, "usage.input_tokens_details.cache_write_tokens").Int())
	require.Equal(t, int64(30), gjson.GetBytes(rewritten, "usage.output_tokens").Int())
	require.Equal(t, int64(195), gjson.GetBytes(rewritten, "usage.total_tokens").Int())
	require.Equal(t, int64(7), gjson.GetBytes(rewritten, "usage.output_tokens_details.reasoning_tokens").Int())
}

func TestDisplayToken_OpenAIChatUsageRewriteHandlesMissingCachedTokens(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	mult := &DisplayTokenMultipliers{
		InputMult:          1.5,
		OutputMult:         2,
		CacheReadMult:      1,
		CacheCreateMult:    1,
		CacheReadInputMult: 2,
	}

	rewritten := rewriteOpenAIChatUsageTokens(body, "usage", mult)
	require.Equal(t, int64(15), gjson.GetBytes(rewritten, "usage.prompt_tokens").Int())
	require.Equal(t, int64(10), gjson.GetBytes(rewritten, "usage.completion_tokens").Int())
	require.Equal(t, int64(25), gjson.GetBytes(rewritten, "usage.total_tokens").Int())
	require.False(t, gjson.GetBytes(rewritten, "usage.prompt_tokens_details").Exists())
}

func TestDisplayToken_OpenAIChatUsageRewriteScalesCacheWriteTokens(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":120,"completion_tokens":10,"total_tokens":130,"prompt_tokens_details":{"cached_tokens":20,"cache_write_tokens":40}}}`)
	mult := &DisplayTokenMultipliers{
		InputMult:          2,
		OutputMult:         3,
		CacheReadMult:      1,
		CacheCreateMult:    1.5,
		CacheReadInputMult: 0,
	}

	rewritten := rewriteOpenAIChatUsageTokens(body, "usage", mult)

	require.Equal(t, int64(200), gjson.GetBytes(rewritten, "usage.prompt_tokens").Int())
	require.Equal(t, int64(20), gjson.GetBytes(rewritten, "usage.prompt_tokens_details.cached_tokens").Int())
	require.Equal(t, int64(60), gjson.GetBytes(rewritten, "usage.prompt_tokens_details.cache_write_tokens").Int())
	require.Equal(t, int64(30), gjson.GetBytes(rewritten, "usage.completion_tokens").Int())
	require.Equal(t, int64(230), gjson.GetBytes(rewritten, "usage.total_tokens").Int())
}

func TestDisplayToken_OpenAIUsageRewriteClampsCachedTokensAboveInput(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":100,"output_tokens":10,"total_tokens":110,"input_tokens_details":{"cached_tokens":200}}}`)
	mult := &DisplayTokenMultipliers{
		InputMult:          2,
		OutputMult:         4,
		CacheReadMult:      1,
		CacheCreateMult:    1,
		CacheReadInputMult: 2,
	}

	rewritten := rewriteOpenAIResponsesUsageTokens(body, "usage", mult)
	require.Equal(t, int64(200), gjson.GetBytes(rewritten, "usage.input_tokens").Int())
	require.Equal(t, int64(200), gjson.GetBytes(rewritten, "usage.input_tokens_details.cached_tokens").Int())
	require.Equal(t, int64(40), gjson.GetBytes(rewritten, "usage.output_tokens").Int())
	require.Equal(t, int64(240), gjson.GetBytes(rewritten, "usage.total_tokens").Int())
}

func TestDisplayToken_ClaudeUsageRewriteBalancesCachePremiumAndDisplayRateKeepsCacheReadReal(t *testing.T) {
	line := `data: {"type":"message_start","message":{"usage":{"input_tokens":100,"output_tokens":10,"cache_read_input_tokens":20,"cache_creation_input_tokens":5}}}`
	mult := &DisplayTokenMultipliers{
		InputMult:          2,
		OutputMult:         3,
		CacheReadMult:      1,
		CacheCreateMult:    1,
		CacheReadInputMult: 4,
		RateScale:          2,
		RateScaleSet:       true,
	}

	rewritten := RewriteSSEUsageTokens(line, mult)
	require.Equal(t, int64(560), gjson.Get(rewritten[len("data: "):], "message.usage.input_tokens").Int())
	require.Equal(t, int64(60), gjson.Get(rewritten[len("data: "):], "message.usage.output_tokens").Int())
	require.Equal(t, int64(20), gjson.Get(rewritten[len("data: "):], "message.usage.cache_read_input_tokens").Int())
	require.Equal(t, int64(10), gjson.Get(rewritten[len("data: "):], "message.usage.cache_creation_input_tokens").Int())
}

func TestDisplayToken_UsageMapRewriteBalancesCachePremium(t *testing.T) {
	usage := map[string]any{
		"input_tokens":                float64(100),
		"output_tokens":               float64(10),
		"cache_read_input_tokens":     float64(20),
		"cache_creation_input_tokens": float64(5),
	}
	mult := &DisplayTokenMultipliers{
		InputMult:          2,
		OutputMult:         3,
		CacheReadMult:      1,
		CacheCreateMult:    1,
		CacheReadInputMult: 4,
	}

	ApplyDisplayMultipliersToUsageMap(usage, mult)
	require.Equal(t, 280, usage["input_tokens"])
	require.Equal(t, 30, usage["output_tokens"])
	require.Equal(t, 20, usage["cache_read_input_tokens"])
	require.Equal(t, 5, usage["cache_creation_input_tokens"])
}

func TestDisplayToken_UsageMapRewriteScalesCacheWriteTokens(t *testing.T) {
	usage := map[string]any{
		"input_tokens":            float64(100),
		"output_tokens":           float64(10),
		"cache_read_input_tokens": float64(20),
		"cache_write_tokens":      float64(30),
	}
	mult := &DisplayTokenMultipliers{
		InputMult:          2,
		OutputMult:         3,
		CacheReadMult:      1,
		CacheCreateMult:    1.5,
		CacheReadInputMult: 0,
	}

	ApplyDisplayMultipliersToUsageMap(usage, mult)

	require.Equal(t, 165, usage["input_tokens"])
	require.Equal(t, 30, usage["output_tokens"])
	require.Equal(t, 20, usage["cache_read_input_tokens"])
	require.Equal(t, 45, usage["cache_write_tokens"])
}

func TestDisplayToken_OpenAIUsageRewriteNoopForNilOrTrivialMultiplier(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens":1000,"output_tokens":100,"total_tokens":1100,"input_tokens_details":{"cached_tokens":200}}}`)

	require.Equal(t, string(body), string(rewriteOpenAIResponsesUsageTokens(body, "usage", nil)))
	require.Equal(t, string(body), string(rewriteOpenAIResponsesUsageTokens(body, "usage", &DisplayTokenMultipliers{
		InputMult:       1,
		OutputMult:      1,
		CacheReadMult:   1,
		CacheCreateMult: 1,
	})))
}

// ===========================================================================
// 缓存创建下游展示改写（成本精确分档口径）
// ===========================================================================

func TestDisplayToken_ComputeMultipliersUsesDisplayCacheCreationPrice(t *testing.T) {
	displayCreate := 2.5e-6
	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:                     "claude-sonnet-4",
		BillingMode:               BillingModeToken,
		DisplayCacheCreationPrice: &displayCreate,
		Enabled:                   true,
	})
	// rich pricing: 5m=3.75e-6, 1h=6e-6, SupportsCacheBreakdown=true
	resolver := NewModelPricingResolver(&ChannelService{}, newTestBillingServiceWithRichPricing(), cache, nil)
	svc := &GatewayService{resolver: resolver}

	mult := svc.ComputeDisplayTokenMultipliers(context.Background(), "claude-sonnet-4", 42, nil, 1, 1)
	require.NotNil(t, mult)
	require.InDelta(t, 1.5, mult.CacheCreateMult, 1e-12, "fallback mult = 5m price / display price")
	require.InDelta(t, 1.5, mult.CacheCreate5mMult, 1e-12)
	require.InDelta(t, 2.4, mult.CacheCreate1hMult, 1e-12, "1h tier mult = 1h price / display price")
	require.True(t, mult.IsNonTrivial(), "display cache creation price alone must activate the rewrite chain")
}

func TestDisplayToken_UserDisplayCacheCreationPriceOverridesGlobal(t *testing.T) {
	globalDisplayCreate := 2.5e-6
	userDisplayCreate := 1.25e-6
	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:                     "claude-sonnet-4",
		BillingMode:               BillingModeToken,
		DisplayCacheCreationPrice: &globalDisplayCreate,
		Enabled:                   true,
	})
	userRepo := &displayTokenUserModelPricingRepoStub{
		override: &UserModelPricingOverride{
			UserID:                    42,
			Model:                     "claude-sonnet-4",
			DisplayCacheCreationPrice: &userDisplayCreate,
			Enabled:                   true,
		},
	}
	resolver := NewModelPricingResolver(&ChannelService{}, newTestBillingServiceWithRichPricing(), cache, userRepo)
	svc := &GatewayService{resolver: resolver}

	mult := svc.ComputeDisplayTokenMultipliers(context.Background(), "claude-sonnet-4", 42, nil, 1, 1)
	require.NotNil(t, mult)
	require.InDelta(t, 3.0, mult.CacheCreate5mMult, 1e-12)
	require.InDelta(t, 4.8, mult.CacheCreate1hMult, 1e-12)
}

func TestDisplayToken_ClaudeUsageRewriteSyncsNestedCacheCreation(t *testing.T) {
	mult := &DisplayTokenMultipliers{
		InputMult:         1.0,
		OutputMult:        1.0,
		CacheReadMult:     1.0,
		CacheCreateMult:   1.5,
		CacheCreate5mMult: 1.5,
		CacheCreate1hMult: 2.4,
		RateScale:         1.0,
		RateScaleSet:      true,
	}
	line := `data: {"type":"message_start","message":{"usage":{"input_tokens":10,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":2000,"cache_creation":{"ephemeral_5m_input_tokens":1000,"ephemeral_1h_input_tokens":1000}}}}`

	out := RewriteSSEUsageTokens(line, mult)
	data := strings.TrimPrefix(out, "data: ")

	total := gjson.Get(data, "message.usage.cache_creation_input_tokens").Int()
	d5m := gjson.Get(data, "message.usage.cache_creation.ephemeral_5m_input_tokens").Int()
	d1h := gjson.Get(data, "message.usage.cache_creation.ephemeral_1h_input_tokens").Int()

	// 成本精确：displayTotal × 展示价 == 5m×p5m + 1h×p1h
	// (1000×1.5 + 1000×2.4 = 3900；3900×2.5e-6 == 1000×3.75e-6 + 1000×6e-6)
	require.EqualValues(t, 3900, total)
	require.EqualValues(t, 1500, d5m)
	require.EqualValues(t, 2400, d1h)
	require.Equal(t, total, d5m+d1h, "nested breakdown must keep summing to the top-level total")
}

func TestDisplayToken_ClaudeUsageRewritePure1hProductionShape(t *testing.T) {
	// 生产形状：上游中转返回纯 1h 缓存创建（5m=0, 1h=66061）
	mult := &DisplayTokenMultipliers{
		InputMult:         1.0,
		OutputMult:        1.0,
		CacheReadMult:     1.0,
		CacheCreateMult:   1.5,
		CacheCreate5mMult: 1.5,
		CacheCreate1hMult: 2.4,
		RateScale:         1.0,
		RateScaleSet:      true,
	}
	body := []byte(`{"usage":{"input_tokens":2,"output_tokens":38,"cache_creation_input_tokens":66061,"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":66061}}}`)

	out := rewriteNonStreamUsageTokens(body, mult)

	require.EqualValues(t, 158546, gjson.GetBytes(out, "usage.cache_creation_input_tokens").Int(), "pure-1h rows must amplify at the 1h tier mult")
	require.EqualValues(t, 0, gjson.GetBytes(out, "usage.cache_creation.ephemeral_5m_input_tokens").Int())
	require.EqualValues(t, 158546, gjson.GetBytes(out, "usage.cache_creation.ephemeral_1h_input_tokens").Int())
}

func TestDisplayToken_UsageMapRewriteSyncsNestedCacheCreationWithRateScale(t *testing.T) {
	mult := &DisplayTokenMultipliers{
		InputMult:         1.0,
		OutputMult:        1.0,
		CacheReadMult:     1.0,
		CacheCreateMult:   1.5,
		CacheCreate5mMult: 1.5,
		CacheCreate1hMult: 2.4,
		RateScale:         2.0,
		RateScaleSet:      true,
	}
	m := map[string]any{
		"input_tokens":                float64(10),
		"cache_creation_input_tokens": float64(2000),
		"cache_creation": map[string]any{
			"ephemeral_5m_input_tokens": float64(1000),
			"ephemeral_1h_input_tokens": float64(1000),
		},
	}

	ApplyDisplayMultipliersToUsageMap(m, mult)

	require.Equal(t, 7800, m["cache_creation_input_tokens"], "RateScale composes after the tier multipliers")
	ccObj := m["cache_creation"].(map[string]any)
	require.Equal(t, 3000, ccObj["ephemeral_5m_input_tokens"])
	require.Equal(t, 4800, ccObj["ephemeral_1h_input_tokens"])
}

func TestDisplayToken_UsageMapRewriteWithoutNestedKeepsLegacyBehavior(t *testing.T) {
	// antigravity 形状：无嵌套对象，仅顶层字段 —— 走单一倍率，行为与既有逻辑一致
	mult := &DisplayTokenMultipliers{
		InputMult:         1.0,
		OutputMult:        1.0,
		CacheReadMult:     1.0,
		CacheCreateMult:   1.5,
		CacheCreate5mMult: 1.5,
		CacheCreate1hMult: 2.4,
		RateScale:         1.0,
		RateScaleSet:      true,
	}
	m := map[string]any{
		"input_tokens":                float64(10),
		"cache_creation_input_tokens": float64(2000),
	}

	ApplyDisplayMultipliersToUsageMap(m, mult)

	require.Equal(t, 3000, m["cache_creation_input_tokens"])
}

func TestDisplayToken_OpenAIResponsesUsageScalesCacheCreation(t *testing.T) {
	mult := &DisplayTokenMultipliers{
		InputMult:         1.0,
		OutputMult:        1.0,
		CacheReadMult:     1.0,
		CacheCreateMult:   1.5,
		CacheCreate5mMult: 1.5,
		CacheCreate1hMult: 1.5,
		RateScale:         1.0,
		RateScaleSet:      true,
	}
	usage := &OpenAIUsage{InputTokens: 100, OutputTokens: 50, CacheCreationInputTokens: 100}

	out := applyOpenAIResponsesUsageDisplayMultipliers(usage, mult)

	require.Equal(t, 150, out.CacheCreationInputTokens)
	require.Equal(t, 150, out.InputTokens)
}

func TestDisplayToken_CacheCreationRealModeNoop(t *testing.T) {
	// trivial multipliers（含分档字段为 1）不得改写任何字节
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
	require.False(t, mult.IsNonTrivial())

	body := []byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":2000,"cache_creation":{"ephemeral_5m_input_tokens":1000,"ephemeral_1h_input_tokens":1000}}}`)
	out := rewriteNonStreamUsageTokens(body, mult)
	require.Equal(t, string(body), string(out))
}

func TestDisplayToken_Display1hPriceDrivesCacheCreate1hMult(t *testing.T) {
	// rich pricing: 5m=3.75e-6, 1h=6e-6;展示 5m=2.5e-6、展示 1h=3e-6 →
	// Mult5m = 3.75/2.5 = 1.5,Mult1h = 6/3 = 2.0(而非 6/2.5=2.4)。
	displayCreate := 2.5e-6
	displayCreate1h := 3e-6
	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:                       "claude-sonnet-4",
		BillingMode:                 BillingModeToken,
		DisplayCacheCreationPrice:   &displayCreate,
		DisplayCacheCreation1hPrice: &displayCreate1h,
		Enabled:                     true,
	})
	resolver := NewModelPricingResolver(&ChannelService{}, newTestBillingServiceWithRichPricing(), cache, nil)
	svc := &GatewayService{resolver: resolver}

	mult := svc.ComputeDisplayTokenMultipliers(context.Background(), "claude-sonnet-4", 42, nil, 1, 1)
	require.NotNil(t, mult)
	require.InDelta(t, 1.5, mult.CacheCreate5mMult, 1e-12)
	require.InDelta(t, 2.0, mult.CacheCreate1hMult, 1e-12, "1h tier must divide by its own display price")
}

func TestDisplayToken_UserCacheWrite1hPriceFeedsMultipliers(t *testing.T) {
	// 用户级真实 1h 价覆盖(6.4e-6)+ 全局展示创建价(2e-6)→ Mult1h = 3.2。
	displayCreate := 2e-6
	userWrite1h := 6.4e-6
	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:                     "claude-sonnet-4",
		BillingMode:               BillingModeToken,
		DisplayCacheCreationPrice: &displayCreate,
		Enabled:                   true,
	})
	userRepo := &displayTokenUserModelPricingRepoStub{
		override: &UserModelPricingOverride{
			UserID:            42,
			Model:             "claude-sonnet-4",
			CacheWrite1hPrice: &userWrite1h,
			Enabled:           true,
		},
	}
	resolver := NewModelPricingResolver(&ChannelService{}, newTestBillingServiceWithRichPricing(), cache, userRepo)
	svc := &GatewayService{resolver: resolver}

	mult := svc.ComputeDisplayTokenMultipliers(context.Background(), "claude-sonnet-4", 42, nil, 1, 1)
	require.NotNil(t, mult)
	require.InDelta(t, 3.2, mult.CacheCreate1hMult, 1e-12)
}
