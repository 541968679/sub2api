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

func TestUsageLogFromService_LongContextDisplayPriceThenDisplayRateKeepsTokenAmplificationInvariant(t *testing.T) {
	displayInput := 5e-6
	displayOutput := 30e-6
	displayCacheRead := 0.5e-6
	log := &service.UsageLog{
		Model:                       "gpt-5.5",
		InputTokens:                 1000,
		OutputTokens:                100,
		CacheReadTokens:             2000,
		InputCost:                   1000 * 10e-6 * 2.0,
		OutputCost:                  100 * 60e-6 * 1.5,
		CacheReadCost:               2000 * 1e-6 * 2.0,
		TotalCost:                   1000*10e-6*2.0 + 100*60e-6*1.5 + 2000*1e-6*2.0,
		ActualCost:                  2.0 * (1000*10e-6*2.0 + 100*60e-6*1.5 + 2000*1e-6*2.0),
		RateMultiplier:              2.0,
		LongContextApplied:          true,
		LongContextInputThreshold:   272000,
		LongContextInputMultiplier:  2.0,
		LongContextOutputMultiplier: 1.5,
	}

	out := UsageLogFromService(log, DisplayPricingMap{
		"gpt-5.5": &DisplayPricingConfig{
			DisplayInputPrice:     &displayInput,
			DisplayOutputPrice:    &displayOutput,
			DisplayCacheReadPrice: &displayCacheRead,
		},
	})
	ApplyUserDisplayRate(out, 1.0)

	if out.InputTokens != 4400 {
		t.Fatalf("input tokens should only include model display ratio and display-rate scaling, got %d", out.InputTokens)
	}
	if out.OutputTokens != 400 {
		t.Fatalf("output tokens should only include model display ratio and display-rate scaling, got %d", out.OutputTokens)
	}
	if out.CacheReadTokens != 4000 {
		t.Fatalf("cache read tokens should only be affected by display-rate scaling, got %d", out.CacheReadTokens)
	}
	if out.RateMultiplier != 1.0 {
		t.Fatalf("rate multiplier should be rewritten to display rate, got %.2f", out.RateMultiplier)
	}
	assertClose(t, "actual_cost", out.ActualCost, log.ActualCost)
	assertClose(t, "total_cost*rate", out.TotalCost*out.RateMultiplier, out.ActualCost)
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

func TestApplyDisplayTransform_CacheCreationTokensAmplified(t *testing.T) {
	// Cache creation amplifies its own token count directly (cost preserved),
	// unlike cache read which keeps tokens and folds the premium into input.
	dispCreate := 2.5e-6
	log := UsageLog{
		Model:                 "claude-fable-5",
		CacheCreationTokens:   1000,
		CacheCreation5mTokens: 600,
		CacheCreation1hTokens: 400,
		CacheCreationCost:     0.0125, // real price 12.5e-6
		TotalCost:             0.0125,
		ActualCost:            0.02,
		RateMultiplier:        1.6,
	}

	ApplyDisplayTransform(&log, &DisplayPricingConfig{DisplayCacheCreationPrice: &dispCreate})

	if log.CacheCreationTokens != 5000 {
		t.Fatalf("cache_creation_tokens should be back-computed at display price, got %d", log.CacheCreationTokens)
	}
	assertClose(t, "cache_creation_cost", log.CacheCreationCost, 0.0125)
	if log.CacheCreation5mTokens != 3000 || log.CacheCreation1hTokens != 2000 {
		t.Fatalf("5m/1h breakdown should scale with total, got 5m=%d 1h=%d", log.CacheCreation5mTokens, log.CacheCreation1hTokens)
	}
	if log.CacheCreation5mTokens+log.CacheCreation1hTokens != log.CacheCreationTokens {
		t.Fatal("5m+1h must equal cache_creation_tokens")
	}
	if log.InputTokens != 0 || log.InputCost != 0 {
		t.Fatalf("input must not absorb any cache-creation premium, got tokens=%d cost=%f", log.InputTokens, log.InputCost)
	}
	assertClose(t, "total_cost", log.TotalCost, 0.0125)
	assertClose(t, "actual_cost", log.ActualCost, 0.02)
}

func TestApplyDisplayTransform_CacheCreationWorksWithoutDisplayInputPrice(t *testing.T) {
	// Unlike the cache-read premium (requires display_input_price), cache-creation
	// amplification is self-contained.
	dispCreate := 5e-6
	log := UsageLog{
		InputTokens:         2,
		InputCost:           2e-5,
		CacheCreationTokens: 42778,
		CacheCreationCost:   0.5347250, // real 12.5e-6/token
		TotalCost:           0.5347450,
		ActualCost:          0.8555920,
		RateMultiplier:      1.6,
	}

	ApplyDisplayTransform(&log, &DisplayPricingConfig{DisplayCacheCreationPrice: &dispCreate})

	if log.CacheCreationTokens != 106945 {
		t.Fatalf("cache_creation_tokens = 0.534725/5e-6 = 106945, got %d", log.CacheCreationTokens)
	}
	assertClose(t, "cache_creation_cost", log.CacheCreationCost, 0.5347250)
	if log.InputTokens != 2 {
		t.Fatalf("input_tokens should stay untouched, got %d", log.InputTokens)
	}
	assertClose(t, "total_cost", log.TotalCost, 0.5347450)
	assertClose(t, "actual_cost", log.ActualCost, 0.8555920)
}

func TestApplyDisplayTransform_NoCacheCreationPriceLeavesReal(t *testing.T) {
	dispInput := 1.5e-6
	log := UsageLog{
		InputTokens:         1000,
		InputCost:           0.003,
		CacheCreationTokens: 2000,
		CacheCreationCost:   0.025,
		TotalCost:           0.028,
		ActualCost:          0.028,
		RateMultiplier:      1.0,
	}

	ApplyDisplayTransform(&log, &DisplayPricingConfig{DisplayInputPrice: &dispInput})

	if log.CacheCreationTokens != 2000 {
		t.Fatalf("cache_creation_tokens should stay real without a display creation price, got %d", log.CacheCreationTokens)
	}
	assertClose(t, "cache_creation_cost", log.CacheCreationCost, 0.025)
	assertClose(t, "total_cost", log.TotalCost, 0.028)
}

func TestApplyDisplayTransform_CacheCreationZeroRowUntouched(t *testing.T) {
	// openai / antigravity(OAuth) / bridge shaped rows carry cache_creation=0;
	// a configured display creation price must be a strict no-op for them.
	dispCreate := 2.5e-6
	log := UsageLog{
		InputTokens:     1000,
		OutputTokens:    500,
		CacheReadTokens: 300,
		InputCost:       0.003,
		OutputCost:      0.0075,
		CacheReadCost:   0.0001,
		TotalCost:       0.0106,
		ActualCost:      0.0106,
		RateMultiplier:  1.0,
	}
	original := log

	ApplyDisplayTransform(&log, &DisplayPricingConfig{DisplayCacheCreationPrice: &dispCreate})

	if log != original {
		t.Fatal("row without cache creation must be unchanged")
	}
}

func TestApplyDisplayTransform_CacheCreationComposesWithCacheReadPremium(t *testing.T) {
	dispInput := 1.5e-6
	dispOutput := 7.5e-6
	dispCache := 0.3e-6
	dispCreate := 2.5e-6
	log := UsageLog{
		InputTokens:         1000,
		OutputTokens:        500,
		CacheReadTokens:     5000,
		CacheCreationTokens: 1000,
		InputCost:           0.003,
		OutputCost:          0.0075,
		CacheReadCost:       0.0045,
		CacheCreationCost:   0.0125,
		TotalCost:           0.0275,
		ActualCost:          0.055,
		RateMultiplier:      2.0,
	}

	ApplyDisplayTransform(&log, &DisplayPricingConfig{
		DisplayInputPrice:         &dispInput,
		DisplayOutputPrice:        &dispOutput,
		DisplayCacheReadPrice:     &dispCache,
		DisplayCacheCreationPrice: &dispCreate,
	})

	// Same expectations as the cache-read premium test...
	if log.InputTokens != 4000 || log.OutputTokens != 1000 || log.CacheReadTokens != 5000 {
		t.Fatalf("read-premium math changed: input=%d output=%d cacheRead=%d", log.InputTokens, log.OutputTokens, log.CacheReadTokens)
	}
	// ...plus the independent creation amplification (no double counting).
	if log.CacheCreationTokens != 5000 {
		t.Fatalf("cache_creation_tokens should amplify independently, got %d", log.CacheCreationTokens)
	}
	assertClose(t, "cache_creation_cost", log.CacheCreationCost, 0.0125)
	assertClose(t, "total_cost", log.TotalCost, 0.0275)
	assertClose(t, "actual_cost", log.ActualCost, 0.055)
}

func TestApplyDisplayTransform_CacheCreationLongContextScalesOnce(t *testing.T) {
	dispCreate := 2.5e-6
	log := UsageLog{
		CacheCreationTokens:        1000,
		CacheCreationCost:          0.025, // real long-context price 25e-6 (12.5e-6 × 2.0)
		TotalCost:                  0.025,
		ActualCost:                 0.025,
		RateMultiplier:             1.0,
		LongContextApplied:         true,
		LongContextInputMultiplier: 2.0,
	}
	cfg := effectiveDisplayPricingForUsageLog(&log, &DisplayPricingConfig{DisplayCacheCreationPrice: &dispCreate})

	ApplyDisplayTransform(&log, cfg)

	// effective display price = 2.5e-6 × 2.0 = 5e-6 → 0.025 / 5e-6 = 5000
	if log.CacheCreationTokens != 5000 {
		t.Fatalf("long-context display creation price should scale once, got %d tokens", log.CacheCreationTokens)
	}
	assertClose(t, "cache_creation_cost", log.CacheCreationCost, 0.025)
}

func TestComputeDisplayFields_IncludesCacheCreation(t *testing.T) {
	dispCreate := 2.5e-6
	log := UsageLog{
		CacheCreationTokens: 1000,
		CacheCreationCost:   0.0125,
		TotalCost:           0.0125,
		ActualCost:          0.0125,
		RateMultiplier:      1.0,
	}

	fields := ComputeDisplayFields(&log, &DisplayPricingConfig{DisplayCacheCreationPrice: &dispCreate})

	if fields == nil {
		t.Fatal("display fields expected")
	}
	if fields.CacheCreationTokens != 5000 {
		t.Fatalf("display cache_creation_tokens = 5000, got %d", fields.CacheCreationTokens)
	}
	assertClose(t, "display cache_creation_cost", fields.CacheCreationCost, 0.0125)
	// Original DTO untouched.
	if log.CacheCreationTokens != 1000 {
		t.Fatalf("real DTO must stay real, got %d", log.CacheCreationTokens)
	}
}

func TestApplyUserDisplayRate_ScalesCacheCreationBreakdownConsistently(t *testing.T) {
	log := UsageLog{
		CacheCreationTokens:   6,
		CacheCreation5mTokens: 3,
		CacheCreation1hTokens: 3,
		CacheCreationCost:     0.0001,
		TotalCost:             0.0001,
		ActualCost:            0.0001,
		RateMultiplier:        1.0,
	}

	ApplyUserDisplayRate(&log, 2.0) // scale 0.5

	if log.CacheCreationTokens != 3 {
		t.Fatalf("total should scale to 3, got %d", log.CacheCreationTokens)
	}
	if log.CacheCreation5mTokens+log.CacheCreation1hTokens != log.CacheCreationTokens {
		t.Fatalf("5m(%d)+1h(%d) must equal total(%d) even with odd rounding",
			log.CacheCreation5mTokens, log.CacheCreation1hTokens, log.CacheCreationTokens)
	}
}

func assertClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s got %.12f, want %.12f", name, got, want)
	}
}
