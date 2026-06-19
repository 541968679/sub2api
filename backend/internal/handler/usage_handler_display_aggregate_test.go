package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func floatPtr(v float64) *float64 { return &v }

// makeDisplayAggTestRow builds a usage row whose stored component costs are consistent
// with the real (billing) per-token prices used in these tests.
func makeDisplayAggTestRow(model string, in, out, cacheCreate, cacheRead int) service.UsageLog {
	const (
		realInputPrice     = 12e-6
		realOutputPrice    = 60e-6
		realCacheReadPrice = 2e-6
	)
	inputCost := float64(in) * realInputPrice
	outputCost := float64(out) * realOutputPrice
	cacheReadCost := float64(cacheRead) * realCacheReadPrice
	total := inputCost + outputCost + cacheReadCost
	return service.UsageLog{
		Model:               model,
		InputTokens:         in,
		OutputTokens:        out,
		CacheCreationTokens: cacheCreate,
		CacheReadTokens:     cacheRead,
		InputCost:           inputCost,
		OutputCost:          outputCost,
		CacheReadCost:       cacheReadCost,
		TotalCost:           total,
		ActualCost:          total, // rate_multiplier = 1
		RateMultiplier:      1,
	}
}

func groupFromDisplayAggTestRows(rows []service.UsageLog) usagestats.DisplayAggregateGroup {
	g := usagestats.DisplayAggregateGroup{Model: rows[0].Model, RateMultiplier: 1}
	for i := range rows {
		r := &rows[i]
		g.Requests++
		g.InputTokens += int64(r.InputTokens)
		g.OutputTokens += int64(r.OutputTokens)
		g.CacheCreationTokens += int64(r.CacheCreationTokens)
		g.CacheReadTokens += int64(r.CacheReadTokens)
		g.InputCost += r.InputCost
		g.OutputCost += r.OutputCost
		g.CacheCreationCost += r.CacheCreationCost
		g.CacheReadCost += r.CacheReadCost
		g.TotalCost += r.TotalCost
		g.ActualCost += r.ActualCost
	}
	return g
}

// TestAggregateDisplayedGroups_ReconcilesWithPerRow proves the per-group display
// aggregation (used for the unbounded all-time dashboard totals) yields the same
// display totals as transforming every row individually and summing — i.e. the
// dashboard cards reconcile with the per-row records list.
func TestAggregateDisplayedGroups_ReconcilesWithPerRow(t *testing.T) {
	// Real cache_read is billed at $2/M but displayed at $0.5/M (the "token 放大" config):
	// the cache premium is moved into input tokens, inflating the displayed token count
	// while actual_cost stays the real charged amount.
	displayMap := dto.DisplayPricingMap{
		"m1": &dto.DisplayPricingConfig{
			DisplayInputPrice:     floatPtr(5e-6),
			DisplayOutputPrice:    floatPtr(30e-6),
			DisplayCacheReadPrice: floatPtr(0.5e-6),
		},
	}
	rows := []service.UsageLog{
		makeDisplayAggTestRow("m1", 1000, 200, 0, 100000),
		makeDisplayAggTestRow("m1", 2000, 100, 0, 50000),
	}

	// Per-row: transform each record then sum (what the records list shows).
	var rowSum displayUsageTotals
	for i := range rows {
		rowSum.addDisplayed(displayUsageRecordForUser(&rows[i], displayMap, nil), 1, 0)
	}

	// Per-group: transform the aggregate once (what the dashboard all-time path does).
	group := groupFromDisplayAggTestRows(rows)
	groupAgg := aggregateDisplayedGroups(
		[]usagestats.DisplayAggregateGroup{group}, displayMap, nil)

	// The two paths reconcile.
	require.Equal(t, rowSum.InputTokens, groupAgg.InputTokens)
	require.Equal(t, rowSum.OutputTokens, groupAgg.OutputTokens)
	require.Equal(t, rowSum.CacheReadTokens, groupAgg.CacheReadTokens)
	require.Equal(t, rowSum.CacheCreationTokens, groupAgg.CacheCreationTokens)
	require.InDelta(t, rowSum.TotalCost, groupAgg.TotalCost, 1e-9)
	require.InDelta(t, rowSum.ActualCost, groupAgg.ActualCost, 1e-9)

	// Expected display values: cache_read count is preserved, input is inflated by the
	// moved cache premium, and actual_cost equals the real charged amount (rate 1).
	require.Equal(t, int64(150000), groupAgg.CacheReadTokens)
	require.Equal(t, int64(52200), groupAgg.InputTokens)
	require.Equal(t, int64(600), groupAgg.OutputTokens)
	require.InDelta(t, 0.354, groupAgg.ActualCost, 1e-9)
}

// TestAggregateDisplayedGroups_NoDisplayConfig keeps real values when no display
// override exists for a model (e.g. haiku / mini), so those stats are unchanged.
func TestAggregateDisplayedGroups_NoDisplayConfig(t *testing.T) {
	rows := []service.UsageLog{
		makeDisplayAggTestRow("haiku", 1000, 200, 0, 100000),
	}
	group := groupFromDisplayAggTestRows(rows)
	groupAgg := aggregateDisplayedGroups(
		[]usagestats.DisplayAggregateGroup{group}, dto.DisplayPricingMap{}, nil)

	require.Equal(t, int64(1000), groupAgg.InputTokens)
	require.Equal(t, int64(200), groupAgg.OutputTokens)
	require.Equal(t, int64(100000), groupAgg.CacheReadTokens)
}
