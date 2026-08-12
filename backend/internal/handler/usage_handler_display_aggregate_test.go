package handler

import (
	"fmt"
	"testing"
	"time"

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
		rowSum.addDisplayed(displayUsageRecordForUser(t.Context(), &rows[i], displayMap, nil, nil), 1, 0)
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

	// New alloc: default M=1.3 cache cap + α=1.5 residual prefers output.
	// actual_cost stays the real charged amount (rate 1).
	require.Equal(t, int64(195000), groupAgg.CacheReadTokens)
	require.Equal(t, int64(45000), groupAgg.InputTokens)
	require.Equal(t, int64(1050), groupAgg.OutputTokens)
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

func TestAggregateDisplayedModelStatsFromGroups_ReconcilesWithPerRow(t *testing.T) {
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
	var rowRecords []dto.UsageLog
	for i := range rows {
		rowRecords = append(rowRecords, *displayUsageRecordForUser(t.Context(), &rows[i], displayMap, nil, nil))
	}
	fromRows := aggregateDisplayedModelStats(rowRecords)
	fromGroups := aggregateDisplayedModelStatsFromGroups(
		[]usagestats.DisplayAggregateGroup{groupFromDisplayAggTestRows(rows)}, displayMap, nil)
	require.Len(t, fromGroups, 1)
	require.Equal(t, fromRows[0].Model, fromGroups[0].Model)
	require.Equal(t, fromRows[0].Requests, fromGroups[0].Requests)
	require.Equal(t, fromRows[0].InputTokens, fromGroups[0].InputTokens)
	require.Equal(t, fromRows[0].OutputTokens, fromGroups[0].OutputTokens)
	require.Equal(t, fromRows[0].CacheReadTokens, fromGroups[0].CacheReadTokens)
	require.InDelta(t, fromRows[0].Cost, fromGroups[0].Cost, 1e-9)
	require.InDelta(t, fromRows[0].ActualCost, fromGroups[0].ActualCost, 1e-9)
}

func TestAggregateDisplayedTrendFromGroups_HourBuckets(t *testing.T) {
	displayMap := dto.DisplayPricingMap{}
	t1 := time.Date(2026, 8, 12, 10, 15, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 12, 10, 45, 0, 0, time.UTC)
	t3 := time.Date(2026, 8, 12, 11, 5, 0, 0, time.UTC)
	rows := []service.UsageLog{
		{Model: "m", InputTokens: 10, OutputTokens: 1, TotalCost: 0.01, ActualCost: 0.01, CreatedAt: t1, RateMultiplier: 1},
		{Model: "m", InputTokens: 20, OutputTokens: 2, TotalCost: 0.02, ActualCost: 0.02, CreatedAt: t2, RateMultiplier: 1},
		{Model: "m", InputTokens: 30, OutputTokens: 3, TotalCost: 0.03, ActualCost: 0.03, CreatedAt: t3, RateMultiplier: 1},
	}
	groups := []usagestats.DisplayAggregateGroup{
		{Model: "m", RateMultiplier: 1, Bucket: publicUsageTrendBucketLabel(t1, "hour"), Requests: 2, InputTokens: 30, OutputTokens: 3, TotalCost: 0.03, ActualCost: 0.03},
		{Model: "m", RateMultiplier: 1, Bucket: publicUsageTrendBucketLabel(t3, "hour"), Requests: 1, InputTokens: 30, OutputTokens: 3, TotalCost: 0.03, ActualCost: 0.03},
	}
	var displayed []dto.UsageLog
	for i := range rows {
		displayed = append(displayed, *displayUsageRecordForUser(t.Context(), &rows[i], displayMap, nil, nil))
	}
	fromRows := aggregateDisplayedPublicUsageTrend(displayed, "hour")
	fromGroups := aggregateDisplayedTrendFromGroups(groups, displayMap, nil)
	require.Equal(t, len(fromRows), len(fromGroups))
	require.Equal(t, fromRows[0].Date, fromGroups[0].Date)
	require.Equal(t, "2026-08-12 10:00", fromGroups[0].Date)
	require.Equal(t, fromRows[0].Requests, fromGroups[0].Requests)
	require.Equal(t, fromRows[0].InputTokens, fromGroups[0].InputTokens)
	require.InDelta(t, fromRows[0].Cost, fromGroups[0].Cost, 1e-9)
	require.InDelta(t, fromRows[0].ActualCost, fromGroups[0].ActualCost, 1e-9)
	require.InDelta(t, 0.03, fromGroups[0].ActualCost, 1e-9)
	require.Equal(t, fromRows[1].Date, fromGroups[1].Date)
	require.InDelta(t, fromRows[1].ActualCost, fromGroups[1].ActualCost, 1e-9)
}

func TestAggregateDisplayedTrendFromGroups_DayBuckets(t *testing.T) {
	displayMap := dto.DisplayPricingMap{}
	d1 := time.Date(2026, 8, 11, 23, 30, 0, 0, time.UTC)
	d2 := time.Date(2026, 8, 12, 1, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	rows := []service.UsageLog{
		{Model: "m", InputTokens: 5, OutputTokens: 1, TotalCost: 0.005, ActualCost: 0.005, CreatedAt: d1, RateMultiplier: 1},
		{Model: "m", InputTokens: 7, OutputTokens: 2, TotalCost: 0.007, ActualCost: 0.007, CreatedAt: d2, RateMultiplier: 1},
		{Model: "m", InputTokens: 9, OutputTokens: 3, TotalCost: 0.009, ActualCost: 0.009, CreatedAt: d3, RateMultiplier: 1},
	}
	groups := []usagestats.DisplayAggregateGroup{
		{Model: "m", RateMultiplier: 1, Bucket: publicUsageTrendBucketLabel(d1, "day"), Requests: 1, InputTokens: 5, OutputTokens: 1, TotalCost: 0.005, ActualCost: 0.005},
		{Model: "m", RateMultiplier: 1, Bucket: publicUsageTrendBucketLabel(d2, "day"), Requests: 2, InputTokens: 16, OutputTokens: 5, TotalCost: 0.016, ActualCost: 0.016},
	}
	var displayed []dto.UsageLog
	for i := range rows {
		displayed = append(displayed, *displayUsageRecordForUser(t.Context(), &rows[i], displayMap, nil, nil))
	}
	fromRows := aggregateDisplayedPublicUsageTrend(displayed, "day")
	fromGroups := aggregateDisplayedTrendFromGroups(groups, displayMap, nil)
	require.Equal(t, len(fromRows), len(fromGroups))
	require.Equal(t, "2026-08-11", fromGroups[0].Date)
	require.Equal(t, "2026-08-12", fromGroups[1].Date)
	require.Equal(t, fromRows[0].Requests, fromGroups[0].Requests)
	require.Equal(t, fromRows[1].Requests, fromGroups[1].Requests)
	require.Equal(t, fromRows[1].InputTokens, fromGroups[1].InputTokens)
	require.InDelta(t, fromRows[0].ActualCost, fromGroups[0].ActualCost, 1e-9)
	require.InDelta(t, fromRows[1].ActualCost, fromGroups[1].ActualCost, 1e-9)
}

func TestPublicUsageTrendBucketLabel_Formats(t *testing.T) {
	ts := time.Date(2026, 8, 12, 10, 45, 0, 0, time.UTC)
	require.Equal(t, "2026-08-12 10:00", publicUsageTrendBucketLabel(ts, "hour"))
	require.Equal(t, "2026-08-12", publicUsageTrendBucketLabel(ts, "day"))
	require.Equal(t, "2026-08", publicUsageTrendBucketLabel(ts, "month"))
	year, week := ts.ISOWeek()
	require.Equal(t, fmt.Sprintf("%04d-%02d", year, week), publicUsageTrendBucketLabel(ts, "week"))
}
