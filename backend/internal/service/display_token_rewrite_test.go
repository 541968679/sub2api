//go:build unit

package service

import (
	"context"
	"net/http/httptest"
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

func TestDisplayToken_ClaudeUsageRewriteBalancesCachePremiumAndDisplayRate(t *testing.T) {
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
	require.Equal(t, int64(40), gjson.Get(rewritten[len("data: "):], "message.usage.cache_read_input_tokens").Int())
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
