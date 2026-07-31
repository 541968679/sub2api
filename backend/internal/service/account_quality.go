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
// SuccessRate and AvgTTFTMs are nil when the rolling window has no applicable samples.
type AccountQualityStats struct {
	WindowSeconds int      `json:"window_seconds"`
	SuccessCount  int64    `json:"success_count"`
	ErrorCount    int64    `json:"error_count"`
	SuccessRate   *float64 `json:"success_rate"`
	AvgTTFTMs     *int     `json:"avg_ttft_ms"`
	TTFTSamples   int64    `json:"ttft_samples"`
}

// AccountQualityStatsBatchReader provides batch quality aggregates for account list cells.
// Implemented by the usage-log repository (usage_logs + ops_error_logs).
type AccountQualityStatsBatchReader interface {
	GetAccountQualityStatsBatch(ctx context.Context, accountIDs []int64, startTime time.Time) (map[int64]*AccountQualityStats, error)
}

// BuildAccountQualityStats merges raw success/error/ttft counts into the API DTO.
func BuildAccountQualityStats(successCount, errorCount, ttftSamples int64, avgTTFT *float64) *AccountQualityStats {
	stats := &AccountQualityStats{
		WindowSeconds: AccountQualityWindowSeconds,
		SuccessCount:  successCount,
		ErrorCount:    errorCount,
		TTFTSamples:   ttftSamples,
	}
	total := successCount + errorCount
	if total > 0 {
		rate := float64(successCount) / float64(total)
		stats.SuccessRate = &rate
	}
	if ttftSamples > 0 && avgTTFT != nil {
		rounded := int(*avgTTFT + 0.5)
		if rounded < 0 {
			rounded = 0
		}
		stats.AvgTTFTMs = &rounded
	}
	return stats
}
