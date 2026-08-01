package service

import (
	"context"
	"time"
)

// AccountQualityWindow is the rolling window used for account list TTFT / success-rate columns.
const AccountQualityWindow = 15 * time.Minute

// AccountQualityWindowSeconds is the fixed window length exposed in API responses.
const AccountQualityWindowSeconds = int(AccountQualityWindow / time.Second)

// AccountQualityMaxBatchSize caps quality-stats batch requests for list pages.
const AccountQualityMaxBatchSize = 200

// AccountQualityStats is the per-account quality snapshot for admin account list columns.
// SuccessRate and TTFT fields are nil when the rolling window has no applicable samples.
//
// TTFT guidance:
//   - P50 (median): primary signal; resistant to a few pathological outliers
//   - P95: tail latency; shows whether slow requests are common
//   - Avg / Max: secondary context in tooltips (avg is skewed by outliers)
type AccountQualityStats struct {
	WindowSeconds int      `json:"window_seconds"`
	SuccessCount  int64    `json:"success_count"`
	ErrorCount    int64    `json:"error_count"`
	SuccessRate   *float64 `json:"success_rate"`
	// AvgTTFTMs kept for backward compatibility; prefer P50 for display.
	AvgTTFTMs   *int  `json:"avg_ttft_ms"`
	P50TTFTMs   *int  `json:"p50_ttft_ms"`
	P95TTFTMs   *int  `json:"p95_ttft_ms"`
	MaxTTFTMs   *int  `json:"max_ttft_ms"`
	TTFTSamples int64 `json:"ttft_samples"`
}

// TTFTAggregate is raw TTFT summary from the usage log batch query.
type TTFTAggregate struct {
	Samples int64
	Avg     *float64
	P50     *float64
	P95     *float64
	Max     *float64
}

// AccountQualityStatsBatchReader provides batch quality aggregates for account list cells.
// Implemented by the usage-log repository (usage_logs + ops_error_logs).
type AccountQualityStatsBatchReader interface {
	GetAccountQualityStatsBatch(ctx context.Context, accountIDs []int64, startTime time.Time) (map[int64]*AccountQualityStats, error)
}

// BuildAccountQualityStats merges raw success/error/ttft aggregates into the API DTO.
func BuildAccountQualityStats(successCount, errorCount int64, ttft TTFTAggregate) *AccountQualityStats {
	stats := &AccountQualityStats{
		WindowSeconds: AccountQualityWindowSeconds,
		SuccessCount:  successCount,
		ErrorCount:    errorCount,
		TTFTSamples:   ttft.Samples,
	}
	total := successCount + errorCount
	if total > 0 {
		rate := float64(successCount) / float64(total)
		stats.SuccessRate = &rate
	}
	if ttft.Samples > 0 {
		stats.AvgTTFTMs = roundNonNegMs(ttft.Avg)
		stats.P50TTFTMs = roundNonNegMs(ttft.P50)
		stats.P95TTFTMs = roundNonNegMs(ttft.P95)
		stats.MaxTTFTMs = roundNonNegMs(ttft.Max)
	}
	return stats
}

func roundNonNegMs(v *float64) *int {
	if v == nil {
		return nil
	}
	rounded := int(*v + 0.5)
	if rounded < 0 {
		rounded = 0
	}
	return &rounded
}
