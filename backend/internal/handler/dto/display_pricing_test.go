package dto

import (
	"fmt"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestApplyDisplayTransform(t *testing.T) {
	// Simulate real usage:
	// 1000 input tokens @ $3/MTok, 500 output @ $15/MTok, 5000 cache_read @ $0.3/MTok
	// rate_multiplier = 2.0
	// input_cost = 1000 * 3e-6 = 0.003
	// output_cost = 500 * 15e-6 = 0.0075
	// cache_read_cost = 5000 * 0.3e-6 = 0.0015
	// total_cost = 0.012
	// actual_cost = 0.012 * 2.0 = 0.024
	log := UsageLog{
		Model:           "claude-sonnet-4-6",
		InputTokens:     1000,
		OutputTokens:    500,
		CacheReadTokens: 5000,
		InputCost:       0.003,
		OutputCost:      0.0075,
		CacheReadCost:   0.0015,
		TotalCost:       0.012,
		ActualCost:      0.024,
		RateMultiplier:  2.0,
	}

	fmt.Println("=== BEFORE (real values) ===")
	fmt.Printf("  Input:  %d tokens, cost=%.6f, price/MTok=%.2f\n", log.InputTokens, log.InputCost, log.InputCost/float64(log.InputTokens)*1e6)
	fmt.Printf("  Output: %d tokens, cost=%.6f, price/MTok=%.2f\n", log.OutputTokens, log.OutputCost, log.OutputCost/float64(log.OutputTokens)*1e6)
	fmt.Printf("  Cache:  %d tokens, cost=%.6f\n", log.CacheReadTokens, log.CacheReadCost)
	fmt.Printf("  Rate: %.1fx, Total: %.6f, Actual: %.6f\n", log.RateMultiplier, log.TotalCost, log.ActualCost)

	// Display config: show $1.5/MTok input, $7.5/MTok output, 10% cache transfer
	dispInput := 1.5e-6
	dispOutput := 7.5e-6
	cacheRatio := 0.1
	cfg := &DisplayPricingConfig{
		DisplayInputPrice:  &dispInput,
		DisplayOutputPrice: &dispOutput,
		CacheTransferRatio: &cacheRatio,
	}

	ApplyDisplayTransform(&log, cfg)

	fmt.Println("\n=== AFTER (display values) ===")
	fmt.Printf("  Input:  %d tokens, cost=%.6f, price/MTok=%.2f\n", log.InputTokens, log.InputCost, log.InputCost/float64(log.InputTokens)*1e6)
	fmt.Printf("  Output: %d tokens, cost=%.6f, price/MTok=%.2f\n", log.OutputTokens, log.OutputCost, log.OutputCost/float64(log.OutputTokens)*1e6)
	fmt.Printf("  Cache:  %d tokens, cost=%.6f\n", log.CacheReadTokens, log.CacheReadCost)
	fmt.Printf("  Rate: %.1fx, Total: %.6f, Actual: %.6f\n", log.RateMultiplier, log.TotalCost, log.ActualCost)

	// Verify actual_cost is unchanged
	if math.Abs(log.ActualCost-0.024) > 1e-9 {
		t.Errorf("actual_cost changed! got %.9f, want 0.024", log.ActualCost)
	}

	// Verify rate_multiplier is unchanged (display rate is only set by ApplyUserDisplayRate)
	if log.RateMultiplier != 2.0 {
		t.Errorf("rate_multiplier should remain 2.0, got %.1f", log.RateMultiplier)
	}

	// Verify frontend-computed price/MTok matches display price
	inputPriceMTok := log.InputCost / float64(log.InputTokens) * 1e6
	if math.Abs(inputPriceMTok-1.5) > 0.01 {
		t.Errorf("display input price/MTok should be ~1.5, got %.4f", inputPriceMTok)
	}

	outputPriceMTok := log.OutputCost / float64(log.OutputTokens) * 1e6
	if math.Abs(outputPriceMTok-7.5) > 0.01 {
		t.Errorf("display output price/MTok should be ~7.5, got %.4f", outputPriceMTok)
	}

	// Verify cache tokens were transferred (10% of 5000 = 500 moved to input)
	fmt.Printf("\n=== VERIFICATION ===\n")
	fmt.Printf("  actual_cost unchanged: %.6f == 0.024: %v\n", log.ActualCost, math.Abs(log.ActualCost-0.024) < 1e-9)
	fmt.Printf("  display rate: %.1f (unchanged from real)\n", log.RateMultiplier)
	fmt.Printf("  frontend input price/MTok: %.2f (target: 1.50)\n", inputPriceMTok)
	fmt.Printf("  frontend output price/MTok: %.2f (target: 7.50)\n", outputPriceMTok)
}

func TestApplyDisplayTransform_NilConfig(t *testing.T) {
	log := UsageLog{InputTokens: 100, InputCost: 0.001, ActualCost: 0.002, RateMultiplier: 2.0}
	original := log
	ApplyDisplayTransform(&log, nil)
	if log.InputTokens != original.InputTokens || log.ActualCost != original.ActualCost {
		t.Error("nil config should not modify anything")
	}
}

func TestApplyDisplayTransform_PartialConfig_OnlyInputPrice(t *testing.T) {
	log := UsageLog{
		InputTokens: 1000, OutputTokens: 500,
		InputCost: 0.003, OutputCost: 0.0075,
		TotalCost: 0.0105, ActualCost: 0.021, RateMultiplier: 2.0,
	}
	savedActual := log.ActualCost
	dispInput := 1.5e-6
	cfg := &DisplayPricingConfig{DisplayInputPrice: &dispInput}
	ApplyDisplayTransform(&log, cfg)
	if math.Abs(log.ActualCost-savedActual) > 1e-9 {
		t.Errorf("actual_cost changed: %.9f", log.ActualCost)
	}
	// Output should remain unchanged since no display_output_price
	if log.OutputTokens != 500 {
		t.Errorf("output_tokens should be unchanged, got %d", log.OutputTokens)
	}
	// Input price/MTok should match display
	priceMTok := log.InputCost / float64(log.InputTokens) * 1e6
	if math.Abs(priceMTok-1.5) > 0.01 {
		t.Errorf("input price/MTok should be ~1.5, got %.4f", priceMTok)
	}
}

func TestApplyDisplayTransform_ZeroTokens(t *testing.T) {
	log := UsageLog{
		InputTokens: 0, OutputTokens: 0, CacheReadTokens: 0,
		InputCost: 0, OutputCost: 0, CacheReadCost: 0,
		TotalCost: 0, ActualCost: 0, RateMultiplier: 1.0,
	}
	dispInput := 1.5e-6
	cacheRatio := 0.1
	cfg := &DisplayPricingConfig{
		DisplayInputPrice:  &dispInput,
		CacheTransferRatio: &cacheRatio,
	}
	// Should not panic
	ApplyDisplayTransform(&log, cfg)
	if log.InputTokens != 0 || log.ActualCost != 0 {
		t.Error("zero-token log should remain zero")
	}
}

func TestApplyDisplayTransform_NoCacheTokens_WithTransferRatio(t *testing.T) {
	log := UsageLog{
		InputTokens: 1000, CacheReadTokens: 0,
		InputCost: 0.003, CacheReadCost: 0,
		TotalCost: 0.003, ActualCost: 0.003, RateMultiplier: 1.0,
	}
	cacheRatio := 0.5
	cfg := &DisplayPricingConfig{CacheTransferRatio: &cacheRatio}
	ApplyDisplayTransform(&log, cfg)
	// No cache to transfer, input should be unchanged
	if log.InputTokens != 1000 {
		t.Errorf("should be unchanged when no cache tokens, got %d", log.InputTokens)
	}
}

func TestApplyDisplayTransform_OnlyCacheTransfer(t *testing.T) {
	log := UsageLog{
		InputTokens: 1000, CacheReadTokens: 5000,
		InputCost: 0.003, CacheReadCost: 0.0015,
		TotalCost: 0.0045, ActualCost: 0.0045, RateMultiplier: 1.0,
	}
	cacheRatio := 0.2
	cfg := &DisplayPricingConfig{CacheTransferRatio: &cacheRatio}
	ApplyDisplayTransform(&log, cfg)
	// 20% of 5000 = 1000 transferred to input
	// After transfer: input=2000, cache=4000
	// No price override, so tokens stay as transferred (no rescaling)
	if log.InputTokens != 2000 {
		t.Errorf("expected 2000 input tokens after transfer, got %d", log.InputTokens)
	}
	if log.CacheReadTokens != 4000 {
		t.Errorf("expected 4000 cache tokens after transfer, got %d", log.CacheReadTokens)
	}
	if math.Abs(log.ActualCost-0.0045) > 1e-9 {
		t.Errorf("actual_cost should be unchanged, got %.9f", log.ActualCost)
	}
}

func TestBuildDisplayPricingMap_OnlyIncludesModelsWithOverrides(t *testing.T) {
	dispPrice := 1.5e-6
	cacheRatio := 0.5
	pricings := []service.GlobalModelPricing{
		{Model: "model-a", DisplayInputPrice: &dispPrice},
		{Model: "model-b"}, // no display config
		{Model: "Model-C", CacheTransferRatio: &cacheRatio},
	}
	m := BuildDisplayPricingMap(pricings)
	if _, ok := m["model-a"]; !ok {
		t.Error("model-a should be in map")
	}
	if _, ok := m["model-b"]; ok {
		t.Error("model-b should NOT be in map (no display config)")
	}
	if _, ok := m["model-c"]; !ok {
		t.Error("Model-C should be in map (case insensitive)")
	}
	if len(m) != 2 {
		t.Errorf("expected 2 entries, got %d", len(m))
	}
}

func TestApplyUserDisplayRate_ScalesTokensAndPreservesActualCost(t *testing.T) {
	// Real: 1000 input @ $3/MTok, 500 output @ $15/MTok, rate=2.0
	// actual_cost = total_cost * rate = 0.012 * 2.0 = 0.024
	log := UsageLog{
		InputTokens:    1000,
		OutputTokens:   500,
		CacheReadTokens: 200,
		InputCost:      0.003,
		OutputCost:     0.0075,
		CacheReadCost:  0.0006,
		TotalCost:      0.0111,
		ActualCost:     0.0222,
		RateMultiplier: 2.0,
	}
	savedActual := log.ActualCost

	// Display rate: 1.0x (half of real 2.0x)
	// Scale = 2.0 / 1.0 = 2.0 → tokens and costs double
	ApplyUserDisplayRate(&log, 1.0)

	// actual_cost must be unchanged
	if math.Abs(log.ActualCost-savedActual) > 1e-9 {
		t.Errorf("actual_cost changed: got %.9f, want %.9f", log.ActualCost, savedActual)
	}
	// rate_multiplier should be display rate
	if log.RateMultiplier != 1.0 {
		t.Errorf("rate_multiplier should be 1.0, got %.2f", log.RateMultiplier)
	}
	// tokens should be doubled (scale=2.0)
	if log.InputTokens != 2000 {
		t.Errorf("input_tokens should be 2000, got %d", log.InputTokens)
	}
	if log.OutputTokens != 1000 {
		t.Errorf("output_tokens should be 1000, got %d", log.OutputTokens)
	}
	if log.CacheReadTokens != 400 {
		t.Errorf("cache_read_tokens should be 400, got %d", log.CacheReadTokens)
	}
	// total_cost * display_rate ≈ actual_cost
	if math.Abs(log.TotalCost*log.RateMultiplier-savedActual) > 1e-6 {
		t.Errorf("total_cost*rate should ≈ actual_cost: %.6f * %.1f = %.6f, want %.6f",
			log.TotalCost, log.RateMultiplier, log.TotalCost*log.RateMultiplier, savedActual)
	}
}

func TestApplyUserDisplayRate_SameRateNoOp(t *testing.T) {
	log := UsageLog{
		InputTokens: 1000, InputCost: 0.003,
		TotalCost: 0.003, ActualCost: 0.006, RateMultiplier: 2.0,
	}
	ApplyUserDisplayRate(&log, 2.0)
	if log.InputTokens != 1000 {
		t.Errorf("same rate should be no-op, tokens changed to %d", log.InputTokens)
	}
}

func TestApplyUserDisplayRate_ZeroDisplayRateNoOp(t *testing.T) {
	log := UsageLog{
		InputTokens: 1000, InputCost: 0.003,
		TotalCost: 0.003, ActualCost: 0.006, RateMultiplier: 2.0,
	}
	ApplyUserDisplayRate(&log, 0)
	if log.InputTokens != 1000 {
		t.Errorf("zero display rate should be no-op, tokens changed to %d", log.InputTokens)
	}
}

func TestApplyUserDisplayRate_HigherDisplayRate(t *testing.T) {
	// Real rate 1.0, display rate 2.0 → scale = 0.5 → tokens halved
	log := UsageLog{
		InputTokens: 1000, OutputTokens: 500,
		InputCost: 0.003, OutputCost: 0.0075,
		TotalCost: 0.0105, ActualCost: 0.0105, RateMultiplier: 1.0,
	}
	savedActual := log.ActualCost

	ApplyUserDisplayRate(&log, 2.0)

	if math.Abs(log.ActualCost-savedActual) > 1e-9 {
		t.Errorf("actual_cost changed: %.9f", log.ActualCost)
	}
	if log.InputTokens != 500 {
		t.Errorf("input_tokens should be 500 (halved), got %d", log.InputTokens)
	}
	if log.OutputTokens != 250 {
		t.Errorf("output_tokens should be 250 (halved), got %d", log.OutputTokens)
	}
	if log.RateMultiplier != 2.0 {
		t.Errorf("rate should be 2.0, got %.1f", log.RateMultiplier)
	}
}

func TestApplyUserDisplayRate_ChainsWithModelDisplayTransform(t *testing.T) {
	// Simulate: model display transform already applied (rate changed from 2.0 to 0.8)
	// Then user display rate applied (0.8 → 1.0)
	log := UsageLog{
		InputTokens: 1500, OutputTokens: 750,
		InputCost: 0.00225, OutputCost: 0.005625,
		CacheReadTokens: 0, CacheReadCost: 0,
		CacheCreationTokens: 0, CacheCreationCost: 0,
		TotalCost:      0.007875,
		ActualCost:     0.0063,
		RateMultiplier: 0.8,
	}
	savedActual := log.ActualCost

	ApplyUserDisplayRate(&log, 1.0)

	if math.Abs(log.ActualCost-savedActual) > 1e-9 {
		t.Errorf("actual_cost changed after chained transform: %.9f", log.ActualCost)
	}
	if log.RateMultiplier != 1.0 {
		t.Errorf("rate should be 1.0 after user display, got %.2f", log.RateMultiplier)
	}
	// Verify self-consistency: total_cost * rate ≈ actual_cost
	computed := log.TotalCost * log.RateMultiplier
	if math.Abs(computed-savedActual) > 1e-6 {
		t.Errorf("total_cost*rate = %.6f, want ≈ %.6f (actual_cost)", computed, savedActual)
	}
}
