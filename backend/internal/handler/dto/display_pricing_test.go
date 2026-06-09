package dto

import (
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestApplyDisplayTransform_CachePremiumMovesToInput(t *testing.T) {
	dispInput := 1.5e-6
	dispOutput := 7.5e-6
	dispCache := 0.3e-6
	log := UsageLog{
		Model:           "claude-sonnet-4-6",
		InputTokens:     1000,
		OutputTokens:    500,
		CacheReadTokens: 5000,
		InputCost:       0.003,
		OutputCost:      0.0075,
		CacheReadCost:   0.0045,
		TotalCost:       0.015,
		ActualCost:      0.03,
		RateMultiplier:  2.0,
	}

	ApplyDisplayTransform(&log, &DisplayPricingConfig{
		DisplayInputPrice:     &dispInput,
		DisplayOutputPrice:    &dispOutput,
		DisplayCacheReadPrice: &dispCache,
	})

	if log.CacheReadTokens != 5000 {
		t.Fatalf("cache_read_tokens should stay real, got %d", log.CacheReadTokens)
	}
	assertClose(t, "cache_read_cost", log.CacheReadCost, 0.0015)
	assertClose(t, "input_cost", log.InputCost, 0.006)
	if log.InputTokens != 4000 {
		t.Fatalf("input_tokens should include cache premium at display input price, got %d", log.InputTokens)
	}
	if log.OutputTokens != 1000 {
		t.Fatalf("output_tokens should be rescaled by display output price, got %d", log.OutputTokens)
	}
	assertClose(t, "actual_cost", log.ActualCost, 0.03)
	if log.RateMultiplier != 2.0 {
		t.Fatalf("rate_multiplier should be unchanged, got %.2f", log.RateMultiplier)
	}
	assertClose(t, "total_cost", log.TotalCost, 0.015)
}

func TestApplyDisplayTransform_NoDisplayInputPriceLeavesCacheReal(t *testing.T) {
	dispCache := 0.3e-6
	log := UsageLog{
		InputTokens:     1000,
		CacheReadTokens: 5000,
		InputCost:       0.003,
		CacheReadCost:   0.0045,
		TotalCost:       0.0075,
		ActualCost:      0.0075,
		RateMultiplier:  1.0,
	}

	ApplyDisplayTransform(&log, &DisplayPricingConfig{DisplayCacheReadPrice: &dispCache})

	if log.InputTokens != 1000 {
		t.Fatalf("input_tokens should not be manufactured without display_input_price, got %d", log.InputTokens)
	}
	assertClose(t, "input_cost", log.InputCost, 0.003)
	if log.CacheReadTokens != 5000 {
		t.Fatalf("cache_read_tokens should stay real, got %d", log.CacheReadTokens)
	}
	assertClose(t, "cache_read_cost", log.CacheReadCost, 0.0045)
	assertClose(t, "actual_cost", log.ActualCost, 0.0075)
	assertClose(t, "total_cost", log.TotalCost, 0.0075)
}

func TestApplyDisplayTransform_NoDisplayCachePriceDoesNotMovePremium(t *testing.T) {
	dispInput := 1.5e-6
	log := UsageLog{
		InputTokens:     1000,
		CacheReadTokens: 5000,
		InputCost:       0.003,
		CacheReadCost:   0.0045,
		TotalCost:       0.0075,
		ActualCost:      0.0075,
		RateMultiplier:  1.0,
	}

	ApplyDisplayTransform(&log, &DisplayPricingConfig{DisplayInputPrice: &dispInput})

	if log.InputTokens != 2000 {
		t.Fatalf("input_tokens should only reflect real input cost, got %d", log.InputTokens)
	}
	assertClose(t, "input_cost", log.InputCost, 0.003)
	if log.CacheReadTokens != 5000 {
		t.Fatalf("cache_read_tokens should stay real, got %d", log.CacheReadTokens)
	}
	assertClose(t, "cache_read_cost", log.CacheReadCost, 0.0045)
	assertClose(t, "total_cost", log.TotalCost, 0.0075)
}

func TestApplyDisplayTransform_NilConfig(t *testing.T) {
	log := UsageLog{InputTokens: 100, InputCost: 0.001, ActualCost: 0.002, RateMultiplier: 2.0}
	original := log
	ApplyDisplayTransform(&log, nil)
	if log != original {
		t.Fatal("nil config should not modify anything")
	}
}

func TestApplyDisplayTransform_ZeroTokens(t *testing.T) {
	dispInput := 1.5e-6
	dispCache := 0.3e-6
	log := UsageLog{RateMultiplier: 1.0}

	ApplyDisplayTransform(&log, &DisplayPricingConfig{
		DisplayInputPrice:     &dispInput,
		DisplayCacheReadPrice: &dispCache,
	})

	if log.InputTokens != 0 || log.CacheReadTokens != 0 || log.ActualCost != 0 {
		t.Fatal("zero-token log should remain zero")
	}
}

func TestBuildDisplayPricingMap_OnlyIncludesModelsWithDisplayPrices(t *testing.T) {
	dispPrice := 1.5e-6
	cachePrice := 0.3e-6
	pricings := []service.GlobalModelPricing{
		{Model: "model-a", DisplayInputPrice: &dispPrice},
		{Model: "model-b"},
		{Model: "Model-C", DisplayCacheReadPrice: &cachePrice},
	}

	m := BuildDisplayPricingMap(pricings)
	if _, ok := m["model-a"]; !ok {
		t.Fatal("model-a should be in map")
	}
	if _, ok := m["model-b"]; ok {
		t.Fatal("model-b should not be in map")
	}
	if _, ok := m["model-c"]; !ok {
		t.Fatal("Model-C should be in map with lowercase lookup")
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
}

func TestBuildUserDisplayPricingMap_UserOverridesGlobalDisplayPrices(t *testing.T) {
	globalInput := 2.0e-6
	userInput := 1.5e-6
	userCache := 0.3e-6
	globalMap := DisplayPricingMap{
		"model-a": &DisplayPricingConfig{DisplayInputPrice: &globalInput},
	}

	m := BuildUserDisplayPricingMap(globalMap, []service.UserModelPricingOverride{
		{Model: "Model-A", Enabled: true, DisplayInputPrice: &userInput, DisplayCacheReadPrice: &userCache},
		{Model: "Model-B", Enabled: true},
	})

	if got := m["model-a"].DisplayInputPrice; got == nil || *got != userInput {
		t.Fatal("user display input price should override global")
	}
	if got := m["model-a"].DisplayCacheReadPrice; got == nil || *got != userCache {
		t.Fatal("user display cache read price should be merged")
	}
	if _, ok := m["model-b"]; ok {
		t.Fatal("user override with no display prices should be ignored")
	}
}

func TestUsageLogFromService_LongContextUsesEffectiveDisplayPrices(t *testing.T) {
	displayInput := 2.5e-6
	displayOutput := 15e-6
	log := &service.UsageLog{
		Model:                       "gpt-5.4",
		InputTokens:                 300000,
		OutputTokens:                2000,
		InputCost:                   300000 * 2.5e-6 * 2.0,
		OutputCost:                  2000 * 15e-6 * 1.5,
		TotalCost:                   300000*2.5e-6*2.0 + 2000*15e-6*1.5,
		ActualCost:                  300000*2.5e-6*2.0 + 2000*15e-6*1.5,
		RateMultiplier:              1.0,
		LongContextApplied:          true,
		LongContextInputThreshold:   272000,
		LongContextInputMultiplier:  2.0,
		LongContextOutputMultiplier: 1.5,
	}

	out := UsageLogFromService(log, DisplayPricingMap{
		"gpt-5.4": &DisplayPricingConfig{
			DisplayInputPrice:  &displayInput,
			DisplayOutputPrice: &displayOutput,
		},
	})

	if out.InputTokens != 300000 {
		t.Fatalf("input tokens should remain real with effective long-context display price, got %d", out.InputTokens)
	}
	if out.OutputTokens != 2000 {
		t.Fatalf("output tokens should remain real with effective long-context display price, got %d", out.OutputTokens)
	}
	assertClose(t, "input_cost", out.InputCost, log.InputCost)
	assertClose(t, "output_cost", out.OutputCost, log.OutputCost)
	assertClose(t, "actual_cost", out.ActualCost, log.ActualCost)
}

func TestUsageLogFromService_LongContextCustomDisplayPriceScalesOnce(t *testing.T) {
	displayInput := 1.25e-6
	displayOutput := 7.5e-6
	log := &service.UsageLog{
		Model:                       "gpt-5.4",
		InputTokens:                 300000,
		OutputTokens:                2000,
		InputCost:                   300000 * 2.5e-6 * 2.0,
		OutputCost:                  2000 * 15e-6 * 1.5,
		TotalCost:                   300000*2.5e-6*2.0 + 2000*15e-6*1.5,
		ActualCost:                  300000*2.5e-6*2.0 + 2000*15e-6*1.5,
		RateMultiplier:              1.0,
		LongContextApplied:          true,
		LongContextInputThreshold:   272000,
		LongContextInputMultiplier:  2.0,
		LongContextOutputMultiplier: 1.5,
	}

	out := UsageLogFromService(log, DisplayPricingMap{
		"gpt-5.4": &DisplayPricingConfig{
			DisplayInputPrice:  &displayInput,
			DisplayOutputPrice: &displayOutput,
		},
	})

	if out.InputTokens != 600000 {
		t.Fatalf("input tokens should scale by custom display price only once, got %d", out.InputTokens)
	}
	if out.OutputTokens != 4000 {
		t.Fatalf("output tokens should scale by custom display price only once, got %d", out.OutputTokens)
	}
	assertClose(t, "actual_cost", out.ActualCost, log.ActualCost)
}

func TestApplyUserDisplayRate_ScalesTokensAndPreservesActualCost(t *testing.T) {
	log := UsageLog{
		InputTokens:     1000,
		OutputTokens:    500,
		CacheReadTokens: 200,
		InputCost:       0.003,
		OutputCost:      0.0075,
		CacheReadCost:   0.0006,
		TotalCost:       0.0111,
		ActualCost:      0.0222,
		RateMultiplier:  2.0,
	}
	savedActual := log.ActualCost

	ApplyUserDisplayRate(&log, 1.0)

	assertClose(t, "actual_cost", log.ActualCost, savedActual)
	if log.RateMultiplier != 1.0 {
		t.Fatalf("rate_multiplier should be 1.0, got %.2f", log.RateMultiplier)
	}
	if log.InputTokens != 2000 || log.OutputTokens != 1000 || log.CacheReadTokens != 400 {
		t.Fatalf("tokens should be doubled, got input=%d output=%d cache=%d", log.InputTokens, log.OutputTokens, log.CacheReadTokens)
	}
	assertClose(t, "total_cost*rate", log.TotalCost*log.RateMultiplier, savedActual)
}

func TestApplyUserDisplayRate_SameOrInvalidRateNoOp(t *testing.T) {
	log := UsageLog{
		InputTokens:    1000,
		InputCost:      0.003,
		TotalCost:      0.003,
		ActualCost:     0.006,
		RateMultiplier: 2.0,
	}
	original := log
	ApplyUserDisplayRate(&log, 2.0)
	if log != original {
		t.Fatal("same rate should be no-op")
	}
	ApplyUserDisplayRate(&log, 0)
	if log != original {
		t.Fatal("zero display rate should be no-op")
	}
}

func TestApplyUserDisplayRate_HigherDisplayRate(t *testing.T) {
	log := UsageLog{
		InputTokens: 1000, OutputTokens: 500,
		InputCost: 0.003, OutputCost: 0.0075,
		TotalCost: 0.0105, ActualCost: 0.0105, RateMultiplier: 1.0,
	}

	ApplyUserDisplayRate(&log, 2.0)

	if log.InputTokens != 500 || log.OutputTokens != 250 {
		t.Fatalf("tokens should be halved, got input=%d output=%d", log.InputTokens, log.OutputTokens)
	}
	if log.RateMultiplier != 2.0 {
		t.Fatalf("rate should be 2.0, got %.1f", log.RateMultiplier)
	}
	assertClose(t, "actual_cost", log.ActualCost, 0.0105)
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s got %.12f, want %.12f", name, got, want)
	}
}
