//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyDisplayContextTokenCap_DefaultOff(t *testing.T) {
	in := DisplayContextTokenCapInput{
		InputTokens:     800_000,
		CacheReadTokens: 700_000,
		OutputTokens:    90_000,
		InputCost:       8,
		CacheReadCost:   0.7,
		OutputCost:      1.8,
		ContextTokenMax: 0,
		OutputTokenMax:  0,
		Seed:            "req-off",
	}
	got := ApplyDisplayContextTokenCap(in)
	require.False(t, got.Applied)
	require.Equal(t, in.InputTokens, got.InputTokens)
	require.Equal(t, in.CacheReadTokens, got.CacheReadTokens)
	require.Equal(t, in.OutputTokens, got.OutputTokens)
	require.Equal(t, in.InputCost, got.InputCost)
	require.Equal(t, in.CacheReadCost, got.CacheReadCost)
	require.Equal(t, in.OutputCost, got.OutputCost)
}

func TestApplyDisplayContextTokenCap_BelowJointCapUnchanged(t *testing.T) {
	in := DisplayContextTokenCapInput{
		InputTokens:     120_000,
		CacheReadTokens: 80_000,
		OutputTokens:    5_000,
		InputCost:       1.2,
		CacheReadCost:   0.08,
		OutputCost:      0.1,
		ContextTokenMax: 1_000_000,
		Seed:            "req-small",
	}
	got := ApplyDisplayContextTokenCap(in)
	require.False(t, got.Applied)
	require.Equal(t, 120_000, got.InputTokens)
	require.Equal(t, 80_000, got.CacheReadTokens)
	require.Equal(t, 5_000, got.OutputTokens)
}

func TestApplyDisplayContextTokenCap_JointShrinkProportionalAndSumExact(t *testing.T) {
	inPrice := 4e-6
	cachePrice := 4e-7
	outPrice := 2e-5
	in := DisplayContextTokenCapInput{
		InputTokens:           900_000,
		CacheReadTokens:       600_000,
		OutputTokens:          10_000,
		InputCost:             900_000 * inPrice,
		CacheReadCost:         600_000 * cachePrice,
		OutputCost:            10_000 * outPrice,
		DisplayInputPrice:     &inPrice,
		DisplayCacheReadPrice: &cachePrice,
		DisplayOutputPrice:    &outPrice,
		ContextTokenMax:       1_000_000,
		Seed:                  "req-joint",
	}
	got := ApplyDisplayContextTokenCap(in)
	require.True(t, got.Applied)
	require.Equal(t, got.JitteredContextCap, got.InputTokens+got.CacheReadTokens)
	require.LessOrEqual(t, got.InputTokens, in.InputTokens)
	require.LessOrEqual(t, got.CacheReadTokens, in.CacheReadTokens)
	require.Equal(t, in.OutputTokens, got.OutputTokens, "joint cap must not grow or shrink output")
	require.InDelta(t, float64(got.InputTokens)*inPrice, got.InputCost, 1e-12)
	require.InDelta(t, float64(got.CacheReadTokens)*cachePrice, got.CacheReadCost, 1e-12)
	require.Equal(t, in.OutputCost, got.OutputCost)

	// Shape preserved within ±1 token of proportional shrink.
	wantIn := int(float64(in.InputTokens) * float64(got.JitteredContextCap) / float64(in.InputTokens+in.CacheReadTokens))
	require.InDelta(t, float64(wantIn), float64(got.InputTokens), 1)
}

func TestApplyDisplayContextTokenCap_JitterStableAndInBand(t *testing.T) {
	configured := int64(1_000_000)
	a := JitteredDisplayTokenCap(configured, "same-id", displayTokenCapLaneJoint)
	b := JitteredDisplayTokenCap(configured, "same-id", displayTokenCapLaneJoint)
	require.Equal(t, a, b)
	require.GreaterOrEqual(t, a, 920_000)
	require.LessOrEqual(t, a, 1_000_000)

	seen := map[int]bool{}
	for _, seed := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		seen[JitteredDisplayTokenCap(configured, seed, displayTokenCapLaneJoint)] = true
	}
	require.Greater(t, len(seen), 1, "different ids should spread inside the 92%-100% band")

	joint := JitteredDisplayTokenCap(configured, "lane-seed", displayTokenCapLaneJoint)
	output := JitteredDisplayTokenCap(80_000, "lane-seed", displayTokenCapLaneOutput)
	require.NotEqual(t, joint, output)
}

func TestApplyDisplayContextTokenCap_OutputIndependent(t *testing.T) {
	outPrice := 2e-5
	in := DisplayContextTokenCapInput{
		InputTokens:        10_000,
		CacheReadTokens:    2_000,
		OutputTokens:       120_000,
		InputCost:          0.04,
		CacheReadCost:      0.001,
		OutputCost:         120_000 * outPrice,
		DisplayOutputPrice: &outPrice,
		OutputTokenMax:     80_000,
		Seed:               "req-output",
	}
	got := ApplyDisplayContextTokenCap(in)
	require.True(t, got.Applied)
	require.Equal(t, 10_000, got.InputTokens)
	require.Equal(t, 2_000, got.CacheReadTokens)
	require.Less(t, got.OutputTokens, 120_000)
	require.LessOrEqual(t, got.OutputTokens, got.JitteredOutputCap)
	require.InDelta(t, float64(got.OutputTokens)*outPrice, got.OutputCost, 1e-12)
}

func TestApplyDisplayContextTokenCap_NewChargeLower(t *testing.T) {
	inPrice := 4e-6
	cachePrice := 4e-7
	outPrice := 2e-5
	displayRate := 0.14
	in := DisplayContextTokenCapInput{
		InputTokens:           900_000,
		CacheReadTokens:       600_000,
		OutputTokens:          10_000,
		InputCost:             900_000 * inPrice,
		CacheReadCost:         600_000 * cachePrice,
		OutputCost:            10_000 * outPrice,
		DisplayInputPrice:     &inPrice,
		DisplayCacheReadPrice: &cachePrice,
		DisplayOutputPrice:    &outPrice,
		ContextTokenMax:       1_000_000,
		Seed:                  "req-charge",
	}
	uncapped := (in.InputCost + in.CacheReadCost + in.OutputCost) * displayRate
	got := ApplyDisplayContextTokenCap(in)
	require.True(t, got.Applied)
	capped := (got.InputCost + got.CacheReadCost + got.OutputCost) * displayRate
	require.Less(t, capped, uncapped)
}

func TestApplyDisplayTokenCapToCharge_DefaultSettingsLeaveActualCost(t *testing.T) {
	log := &UsageLog{
		RequestID:      "req-default",
		Model:          "gpt-5.6-sol",
		InputTokens:    900_000,
		CacheReadTokens: 600_000,
		OutputTokens:   10_000,
		InputCost:      3.6,
		CacheReadCost:  0.24,
		OutputCost:     0.2,
		TotalCost:      4.04,
		ActualCost:     0.5656,
		RateMultiplier: 0.14,
		BillingMode:    billingModePtr(string(BillingModeToken)),
	}
	cost := &CostBreakdown{ActualCost: 0.5656, BillingMode: string(BillingModeToken)}
	applyDisplayTokenCapToCharge(context.Background(), log, cost, &User{ID: 1}, nil, nil, nil, nil)
	require.InDelta(t, 0.5656, cost.ActualCost, 1e-12)
	require.False(t, log.DisplayTokenCapApplied)
}

func TestApplyDisplayTokenCapToCharge_BindsAndLowersActualCost(t *testing.T) {
	settings := NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{
		SettingKeyDisplayContextTokenMax: "1000000",
		SettingKeyDisplayOutputTokenMax:  "0",
	}}, nil)
	inPrice := 4e-6
	cachePrice := 4e-7
	outPrice := 2e-5
	log := &UsageLog{
		RequestID:      "req-bind",
		Model:          "gpt-5.6-sol",
		InputTokens:    900_000,
		CacheReadTokens: 600_000,
		OutputTokens:   10_000,
		InputCost:      900_000 * inPrice,
		CacheReadCost:  600_000 * cachePrice,
		OutputCost:     10_000 * outPrice,
		TotalCost:      900_000*inPrice + 600_000*cachePrice + 10_000*outPrice,
		ActualCost:     (900_000*inPrice + 600_000*cachePrice + 10_000*outPrice) * 0.14,
		RateMultiplier: 0.14,
		BillingMode:    billingModePtr(string(BillingModeToken)),
	}
	uncapped := log.ActualCost
	cost := &CostBreakdown{ActualCost: uncapped, BillingMode: string(BillingModeToken)}
	applyDisplayTokenCapToCharge(context.Background(), log, cost, &User{ID: 1}, nil, nil, settings, nil)
	require.True(t, log.DisplayTokenCapApplied)
	require.Equal(t, int64(1_000_000), log.DisplayContextTokenMaxUsed)
	require.Less(t, cost.ActualCost, uncapped)
	require.Equal(t, cost.ActualCost, log.ActualCost)
	require.Equal(t, 900_000, log.InputTokens, "billing-real tokens must stay")
	require.Equal(t, 600_000, log.CacheReadTokens)
}

func TestApplyDisplayTokenCapToCharge_SkipsNonTokenMode(t *testing.T) {
	settings := NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{
		SettingKeyDisplayContextTokenMax: "1000000",
	}}, nil)
	log := &UsageLog{
		RequestID:      "req-image",
		ImageCount:     1,
		ActualCost:     0.2,
		RateMultiplier: 1,
		BillingMode:    billingModePtr(string(BillingModeImage)),
	}
	cost := &CostBreakdown{ActualCost: 0.2, BillingMode: string(BillingModeImage)}
	applyDisplayTokenCapToCharge(context.Background(), log, cost, &User{ID: 1}, nil, nil, settings, nil)
	require.False(t, log.DisplayTokenCapApplied)
	require.InDelta(t, 0.2, cost.ActualCost, 1e-12)
}

func billingModePtr(v string) *string { return &v }
