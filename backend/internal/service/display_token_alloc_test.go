package service

import (
	"math"
	"testing"
)

func TestAllocateDisplayTokens_M1Alpha0_LegacyResidualToInput(t *testing.T) {
	pin, pout, pcache := 1.5e-6, 7.5e-6, 0.3e-6
	alpha0 := 0.0
	got := AllocateDisplayTokens(DisplayTokenAllocInput{
		InputTokens:               1000,
		OutputTokens:              500,
		CacheReadTokens:           5000,
		InputCost:                 0.003,
		OutputCost:                0.0075,
		CacheReadCost:             0.0045,
		DisplayInputPrice:         &pin,
		DisplayOutputPrice:        &pout,
		DisplayCacheReadPrice:     &pcache,
		CacheTokenMaxMult:         1.0,
		OutputResidualGrowthRatio: &alpha0,
	})
	if got.CacheReadTokens != 5000 || got.InputTokens != 4000 || got.OutputTokens != 1000 {
		t.Fatalf("got in=%d out=%d cache=%d", got.InputTokens, got.OutputTokens, got.CacheReadTokens)
	}
}

func TestAllocateDisplayTokens_DefaultMAndAlpha_PrefersOutput(t *testing.T) {
	// Defaults: M=1.3, α=1.5 → cache cap 5000*1.3=6500; residual prefers output.
	pin, pout, pcache := 1.5e-6, 7.5e-6, 0.3e-6
	got := AllocateDisplayTokens(DisplayTokenAllocInput{
		InputTokens:           1000,
		OutputTokens:          500,
		CacheReadTokens:       5000,
		InputCost:             0.003,
		OutputCost:            0.0075,
		CacheReadCost:         0.0045,
		DisplayInputPrice:     &pin,
		DisplayOutputPrice:    &pout,
		DisplayCacheReadPrice: &pcache,
	})
	if got.CacheReadTokens != 6500 {
		t.Fatalf("cache want 6500 got %d", got.CacheReadTokens)
	}
	if got.InputTokens != 2000 {
		t.Fatalf("input want 2000 got %d", got.InputTokens)
	}
	if got.OutputTokens != 1340 {
		t.Fatalf("output want 1340 got %d", got.OutputTokens)
	}
}

func TestAllocateDisplayTokens_CacheOnlyPriceLeavesReal(t *testing.T) {
	pcache := 0.3e-6
	got := AllocateDisplayTokens(DisplayTokenAllocInput{
		InputTokens:           1000,
		CacheReadTokens:       5000,
		InputCost:             0.003,
		CacheReadCost:         0.0045,
		DisplayCacheReadPrice: &pcache,
	})
	if got.CacheReadTokens != 5000 || got.InputTokens != 1000 {
		t.Fatalf("should leave real when residual cannot sink: in=%d cache=%d", got.InputTokens, got.CacheReadTokens)
	}
	if math.Abs(got.CacheReadCost-0.0045) > 1e-12 {
		t.Fatalf("cache cost should stay real, got %v", got.CacheReadCost)
	}
}

func TestAllocateDisplayTokens_ZeroOutputResidualFallsToInput(t *testing.T) {
	pin, pout, pcache := 1.5e-6, 7.5e-6, 0.3e-6
	got := AllocateDisplayTokens(DisplayTokenAllocInput{
		InputTokens:           1000,
		OutputTokens:          0,
		CacheReadTokens:       5000,
		InputCost:             0.003,
		OutputCost:            0,
		CacheReadCost:         0.0045,
		DisplayInputPrice:     &pin,
		DisplayOutputPrice:    &pout,
		DisplayCacheReadPrice: &pcache,
		CacheTokenMaxMult:     1.0,
	})
	if got.OutputTokens != 0 {
		t.Fatalf("should not invent output tokens, got %d", got.OutputTokens)
	}
	if got.CacheReadTokens != 5000 {
		t.Fatalf("cache want 5000 got %d", got.CacheReadTokens)
	}
	// residual 0.003 → input
	if got.InputTokens != 4000 {
		t.Fatalf("input want 4000 got %d", got.InputTokens)
	}
}

func TestResolveDisplayCacheTokenMaxMult(t *testing.T) {
	user := 2.5
	if got := ResolveDisplayCacheTokenMaxMult(&user, 1.2); got != 2.5 {
		t.Fatalf("user override: %v", got)
	}
	if got := ResolveDisplayCacheTokenMaxMult(nil, 1.5); got != 1.5 {
		t.Fatalf("global: %v", got)
	}
	if got := ResolveDisplayCacheTokenMaxMult(nil, 0); got != DefaultDisplayCacheTokenMaxMult {
		t.Fatalf("default: %v", got)
	}
}
