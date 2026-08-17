//go:build unit

package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrueCostDisplayModel_PrefersRequestedModel(t *testing.T) {
	require.Equal(t, "claude-opus-4-8", trueCostDisplayModel(&UsageLog{
		Model:          "gpt-5.5",
		RequestedModel: "claude-opus-4-8",
		UpstreamModel:  trueCostStrPtr("gpt-5.5"),
	}))
	require.Equal(t, "claude-sonnet-4-6", trueCostDisplayModel(&UsageLog{Model: "claude-sonnet-4-6"}))
}

func TestResolveTrueCostUnitPrices_DisplayThenLiteLLMNeverBilling(t *testing.T) {
	userIn := 1e-6
	userOut := 2e-6
	globalIn := 3e-6
	globalRead := 0.5e-6
	billingIn := 99e-6
	billingOut := 88e-6

	userRepo := &modelPricingResolverUserOverrideRepoStub{
		overrides: map[string]*UserModelPricingOverride{
			"1:claude-opus": {
				UserID:              1,
				Model:               "claude-opus",
				Enabled:             true,
				InputPrice:          &billingIn,
				OutputPrice:         &billingOut,
				DisplayInputPrice:   &userIn,
				DisplayOutputPrice:  &userOut,
				DisplayCacheReadPrice: nil,
			},
		},
	}
	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:               "claude-opus",
		Enabled:             true,
		InputPrice:          &billingIn,
		DisplayInputPrice:   &globalIn,
		DisplayCacheReadPrice: &globalRead,
	})
	bs := &BillingService{pricingService: &PricingService{pricingData: map[string]*LiteLLMModelPricing{}}}
	resolver := NewModelPricingResolver(nil, bs, cache, userRepo)

	prices := resolveTrueCostUnitPrices(context.Background(), resolver, 1, "claude-opus")
	require.InDelta(t, userIn, prices.Input, 1e-12)
	require.InDelta(t, userOut, prices.Output, 1e-12)
	require.InDelta(t, globalRead, prices.CacheRead, 1e-12)
}

func TestResolveTrueCostUnitPrices_NoDisplayUsesLiteLLMNotBilling(t *testing.T) {
	billingIn := 99e-6
	litellmIn := 5e-6
	litellmOut := 25e-6
	userRepo := &modelPricingResolverUserOverrideRepoStub{
		overrides: map[string]*UserModelPricingOverride{
			"1:true-cost-only": {
				UserID:     1,
				Model:      "true-cost-only",
				Enabled:    true,
				InputPrice: &billingIn,
			},
		},
	}
	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:      "true-cost-only",
		Enabled:    true,
		InputPrice: &billingIn,
	})
	bs := &BillingService{
		pricingService: &PricingService{
			pricingData: map[string]*LiteLLMModelPricing{
				"true-cost-only": {
					InputCostPerToken:  litellmIn,
					OutputCostPerToken: litellmOut,
				},
			},
		},
	}
	resolver := NewModelPricingResolver(nil, bs, cache, userRepo)
	prices := resolveTrueCostUnitPrices(context.Background(), resolver, 1, "true-cost-only")
	require.InDelta(t, litellmIn, prices.Input, 1e-12)
	require.InDelta(t, litellmOut, prices.Output, 1e-12)
	require.NotEqual(t, billingIn, prices.Input)
}

func TestResolveTrueCostUnitPrices_LiteLLMMissDoesNotUseBillingFallback(t *testing.T) {
	model := "claude-3-5-sonnet"
	bs := NewBillingService(nil, &PricingService{pricingData: map[string]*LiteLLMModelPricing{}})
	priced, err := bs.GetModelPricing(model)
	require.NoError(t, err)
	require.NotNil(t, priced)
	require.Greater(t, priced.InputPricePerToken, 0.0, "billing fallback must exist so the leak is observable")

	billingIn := priced.InputPricePerToken
	userRepo := &modelPricingResolverUserOverrideRepoStub{
		overrides: map[string]*UserModelPricingOverride{
			"1:" + model: {
				UserID:     1,
				Model:      model,
				Enabled:    true,
				InputPrice: &billingIn,
			},
		},
	}
	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:      model,
		Enabled:    true,
		InputPrice: &billingIn,
	})
	resolver := NewModelPricingResolver(nil, bs, cache, userRepo)

	prices := resolveTrueCostUnitPrices(context.Background(), resolver, 1, model)
	require.Equal(t, 0.0, prices.Input)
	require.Equal(t, 0.0, prices.Output)
	require.Equal(t, 0.0, prices.CacheRead)
	require.Equal(t, 0.0, prices.CacheWrite5m)
	require.NotEqual(t, priced.InputPricePerToken, prices.Input)
	require.NotEqual(t, priced.OutputPricePerToken, prices.Output)

	log := &UsageLog{
		UserID:         1,
		AccountID:      2,
		Model:          model,
		RequestedModel: model,
		InputTokens:    1000,
		OutputTokens:   200,
		ActualCost:     0.42,
	}
	applyTrueCost(context.Background(), log, 0.15, resolver)
	require.Equal(t, 0.42, log.ActualCost)
	require.NotNil(t, log.TrueCost)
	require.Equal(t, 0.0, *log.TrueCost, "LiteLLM miss must write 0, not billing fallback × tokens × rate")
	require.NotEqual(t, 1000*priced.InputPricePerToken*0.15, *log.TrueCost)
}

func TestApplyTrueCost_SnapshotsRateAndLeavesActualCost(t *testing.T) {
	litellmIn := 1e-6
	litellmOut := 2e-6
	bs := &BillingService{
		pricingService: &PricingService{
			pricingData: map[string]*LiteLLMModelPricing{
				"claude-sonnet": {
					InputCostPerToken:  litellmIn,
					OutputCostPerToken: litellmOut,
				},
			},
		},
	}
	resolver := NewModelPricingResolver(nil, bs, newTestGlobalPricingCache(), nil)
	log := &UsageLog{
		UserID:         16,
		AccountID:      7,
		Model:          "claude-sonnet",
		RequestedModel: "claude-sonnet",
		InputTokens:    1000,
		OutputTokens:   500,
		ActualCost:     1.23,
		TotalCost:      9.99,
	}
	rate := 0.15
	applyTrueCost(context.Background(), log, rate, resolver)
	require.Equal(t, 1.23, log.ActualCost)
	require.NotNil(t, log.TrueCostRate)
	require.InDelta(t, rate, *log.TrueCostRate, 1e-12)
	require.NotNil(t, log.TrueCost)
	want := (1000*litellmIn + 500*litellmOut) * rate
	require.InDelta(t, want, *log.TrueCost, 1e-12)
}

func TestApplyTrueCost_NonFiniteLeavesNull(t *testing.T) {
	bs := &BillingService{
		pricingService: &PricingService{
			pricingData: map[string]*LiteLLMModelPricing{
				"claude-sonnet": {InputCostPerToken: 1e-6},
			},
		},
	}
	resolver := NewModelPricingResolver(nil, bs, newTestGlobalPricingCache(), nil)
	log := &UsageLog{
		UserID:         1,
		AccountID:      2,
		Model:          "claude-sonnet",
		RequestedModel: "claude-sonnet",
		InputTokens:    10,
		ActualCost:     0.42,
	}
	applyTrueCost(context.Background(), log, math.Inf(1), resolver)
	require.Equal(t, 0.42, log.ActualCost)
	require.Nil(t, log.TrueCost)
	require.Nil(t, log.TrueCostRate)

	applyTrueCost(context.Background(), log, math.NaN(), resolver)
	require.Equal(t, 0.42, log.ActualCost)
	require.Nil(t, log.TrueCost)
	require.Nil(t, log.TrueCostRate)
}

func TestApplyTrueCost_NilResolverLeavesNull(t *testing.T) {
	log := &UsageLog{ActualCost: 0.5, InputTokens: 10}
	applyTrueCost(context.Background(), log, 0.15, nil)
	require.Equal(t, 0.5, log.ActualCost)
	require.Nil(t, log.TrueCost)
	require.Nil(t, log.TrueCostRate)
}

func TestComputeTrueCostAmount_UsesRealTokensNotBillingCosts(t *testing.T) {
	amount := computeTrueCostAmount(&UsageLog{
		InputTokens:           10,
		OutputTokens:          2,
		CacheCreationTokens:   4,
		CacheReadTokens:       8,
		ImageOutputTokens:     1,
		ImageCount:            2,
		InputCost:             99,
		OutputCost:            99,
		TotalCost:             99,
		ActualCost:            99,
		CacheCreation5mTokens: 0,
		CacheCreation1hTokens: 0,
	}, trueCostUnitPrices{
		Input:         1,
		Output:        2,
		CacheWrite5m:  3,
		CacheWrite1h:  4,
		CacheRead:     0.5,
		ImagePerToken: 7,
		ImagePerImage: 11,
	})
	require.InDelta(t, 10*1+2*2+4*3+8*0.5+1*7+2*11, amount, 1e-12)
}

func trueCostStrPtr(v string) *string { return &v }
