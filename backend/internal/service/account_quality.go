package service

import (
	"context"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// AccountQualityWindow is the rolling window used for account list TTFT / success-rate columns.
const AccountQualityWindow = 15 * time.Minute

// AccountQualityWindowSeconds is the fixed window length exposed in API responses.
const AccountQualityWindowSeconds = int(AccountQualityWindow / time.Second)

// AccountQualityMaxBatchSize caps quality-stats batch requests for list pages.
const AccountQualityMaxBatchSize = 200

// AccountQualitySnapshotInterval is how often the maintenance job persists the live 15m window.
const AccountQualitySnapshotInterval = 5 * time.Minute

// AccountQualitySnapshotRetention is how long snapshot rows are kept.
const AccountQualitySnapshotRetention = 7 * 24 * time.Hour

// AccountQualityHistoryDefaultRange is the default history API window when from/to are omitted.
const AccountQualityHistoryDefaultRange = 24 * time.Hour

// AccountQualityHistoryMaxRange is the maximum history API window.
const AccountQualityHistoryMaxRange = 7 * 24 * time.Hour

// AccountQualitySnapshotDeleteBatchSize is the per-loop delete limit for expired snapshots.
const AccountQualitySnapshotDeleteBatchSize = 500

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
	// ResumeUsers is live-cache only: user_id -> unix until. After 立即恢复,
	// that pair is not judged until this timestamp (one 15-minute window).
	ResumeUsers map[string]int64 `json:"resume_users,omitempty"`
	// AccountResumeUntil is live-cache only. After hard-close 立即恢复,
	// the account is not re-paused until this timestamp.
	AccountResumeUntil *int64 `json:"account_resume_until,omitempty"`
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

// UserQualityStatsBatchReader provides the same aggregates grouped by user_id.
type UserQualityStatsBatchReader interface {
	GetUserQualityStatsBatch(ctx context.Context, userIDs []int64, startTime time.Time) (map[int64]*AccountQualityStats, error)
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

func qualityResumeUserKey(userID int64) string {
	return strconv.FormatInt(userID, 10)
}

// SetUserQualityResume marks one pair as force-admitted until `until`.
func SetUserQualityResume(stats *AccountQualityStats, userID int64, until time.Time) {
	if stats == nil || userID <= 0 || until.IsZero() {
		return
	}
	if stats.ResumeUsers == nil {
		stats.ResumeUsers = make(map[string]int64, 1)
	}
	stats.ResumeUsers[qualityResumeUserKey(userID)] = until.UTC().Unix()
}

// SetAccountQualityResume marks the account hard-close as not re-pausing until `until`.
func SetAccountQualityResume(stats *AccountQualityStats, until time.Time) {
	if stats == nil || until.IsZero() {
		return
	}
	unix := until.UTC().Unix()
	stats.AccountResumeUntil = &unix
}

// UserQualityResumeActive reports whether this pair is inside a manual-resume grace.
func UserQualityResumeActive(stats *AccountQualityStats, userID int64, now time.Time) bool {
	if stats == nil || userID <= 0 || len(stats.ResumeUsers) == 0 {
		return false
	}
	until, ok := stats.ResumeUsers[qualityResumeUserKey(userID)]
	return ok && until > now.Unix()
}

// AccountQualityResumeActive reports whether account hard-close is inside a manual-resume grace.
func AccountQualityResumeActive(stats *AccountQualityStats, now time.Time) bool {
	return stats != nil && stats.AccountResumeUntil != nil && *stats.AccountResumeUntil > now.Unix()
}

// HasActiveQualityResume reports whether any resume grace is still live.
func HasActiveQualityResume(stats *AccountQualityStats, now time.Time) bool {
	if AccountQualityResumeActive(stats, now) {
		return true
	}
	if stats == nil {
		return false
	}
	for _, until := range stats.ResumeUsers {
		if until > now.Unix() {
			return true
		}
	}
	return false
}

func pruneQualityResume(stats *AccountQualityStats, now time.Time) {
	if stats == nil {
		return
	}
	if stats.AccountResumeUntil != nil && *stats.AccountResumeUntil <= now.Unix() {
		stats.AccountResumeUntil = nil
	}
	if len(stats.ResumeUsers) == 0 {
		return
	}
	kept := make(map[string]int64, len(stats.ResumeUsers))
	for key, until := range stats.ResumeUsers {
		if until > now.Unix() {
			kept[key] = until
		}
	}
	if len(kept) == 0 {
		stats.ResumeUsers = nil
		return
	}
	stats.ResumeUsers = kept
}

func MergeQualityResume(dst, src *AccountQualityStats, now time.Time) {
	if dst == nil {
		return
	}
	if src != nil {
		if len(src.ResumeUsers) > 0 {
			if dst.ResumeUsers == nil {
				dst.ResumeUsers = make(map[string]int64, len(src.ResumeUsers))
			}
			for key, until := range src.ResumeUsers {
				if existing, ok := dst.ResumeUsers[key]; !ok || until > existing {
					dst.ResumeUsers[key] = until
				}
			}
		}
		if src.AccountResumeUntil != nil && (dst.AccountResumeUntil == nil || *src.AccountResumeUntil > *dst.AccountResumeUntil) {
			until := *src.AccountResumeUntil
			dst.AccountResumeUntil = &until
		}
	}
	pruneQualityResume(dst, now)
}

// HasAccountQualitySamples reports whether the live window has any success, error, or TTFT samples.
func HasAccountQualitySamples(stats *AccountQualityStats) bool {
	if stats == nil {
		return false
	}
	return stats.SuccessCount > 0 || stats.ErrorCount > 0 || stats.TTFTSamples > 0
}

// TruncateToAccountQualitySnapshotTime truncates t to a 5-minute UTC boundary.
func TruncateToAccountQualitySnapshotTime(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), (t.Minute()/5)*5, 0, 0, time.UTC)
}

// AccountQualityHistoryItem is one persisted snapshot point for the history API.
type AccountQualityHistoryItem struct {
	AccountQualityStats
	CapturedAt time.Time `json:"captured_at"`
}

// AccountQualitySnapshotRow is the persisted snapshot row used by the repository.
type AccountQualitySnapshotRow struct {
	AccountID     int64
	CapturedAt    time.Time
	WindowSeconds int
	SuccessCount  int64
	ErrorCount    int64
	TTFTSamples   int64
	SuccessRate   *float64
	AvgTTFTMs     *int
	P50TTFTMs     *int
	P95TTFTMs     *int
	MaxTTFTMs     *int
}

// SnapshotFromAccountQualityStats copies live stats into a snapshot row.
// captured_at is always truncated to a 5-minute UTC boundary.
func SnapshotFromAccountQualityStats(accountID int64, capturedAt time.Time, stats *AccountQualityStats) AccountQualitySnapshotRow {
	row := AccountQualitySnapshotRow{
		AccountID:     accountID,
		CapturedAt:    TruncateToAccountQualitySnapshotTime(capturedAt),
		WindowSeconds: AccountQualityWindowSeconds,
	}
	if stats == nil {
		return row
	}
	row.WindowSeconds = stats.WindowSeconds
	if row.WindowSeconds <= 0 {
		row.WindowSeconds = AccountQualityWindowSeconds
	}
	row.SuccessCount = stats.SuccessCount
	row.ErrorCount = stats.ErrorCount
	row.TTFTSamples = stats.TTFTSamples
	row.SuccessRate = stats.SuccessRate
	row.AvgTTFTMs = stats.AvgTTFTMs
	row.P50TTFTMs = stats.P50TTFTMs
	row.P95TTFTMs = stats.P95TTFTMs
	row.MaxTTFTMs = stats.MaxTTFTMs
	return row
}

// ToHistoryItem maps a stored row onto the history API item (same null semantics as live stats).
func (r AccountQualitySnapshotRow) ToHistoryItem() AccountQualityHistoryItem {
	window := r.WindowSeconds
	if window <= 0 {
		window = AccountQualityWindowSeconds
	}
	return AccountQualityHistoryItem{
		AccountQualityStats: AccountQualityStats{
			WindowSeconds: window,
			SuccessCount:  r.SuccessCount,
			ErrorCount:    r.ErrorCount,
			SuccessRate:   r.SuccessRate,
			AvgTTFTMs:     r.AvgTTFTMs,
			P50TTFTMs:     r.P50TTFTMs,
			P95TTFTMs:     r.P95TTFTMs,
			MaxTTFTMs:     r.MaxTTFTMs,
			TTFTSamples:   r.TTFTSamples,
		},
		CapturedAt: r.CapturedAt.UTC(),
	}
}

// AccountQualitySnapshotRepository persists and queries 15-minute quality snapshots.
type AccountQualitySnapshotRepository interface {
	Upsert(ctx context.Context, row AccountQualitySnapshotRow) error
	ListByAccount(ctx context.Context, accountID int64, from, to time.Time) ([]AccountQualitySnapshotRow, error)
	DeleteExpired(ctx context.Context, cutoff time.Time, limit int) (int64, error)
	ListRecentTrafficAccountIDs(ctx context.Context, startTime time.Time) ([]int64, error)
}

// NormalizeAccountQualityHistoryRange applies default to=now, from=to-24h, and rejects ranges over 7 days.
// Zero from/to mean "use the default". now should be UTC.
func NormalizeAccountQualityHistoryRange(from, to, now time.Time) (time.Time, time.Time, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if to.IsZero() {
		to = now
	} else {
		to = to.UTC()
	}
	if from.IsZero() {
		from = to.Add(-AccountQualityHistoryDefaultRange)
	} else {
		from = from.UTC()
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, infraerrors.BadRequest("INVALID_TIME_RANGE", "from must be before to")
	}
	if to.Sub(from) > AccountQualityHistoryMaxRange {
		return time.Time{}, time.Time{}, infraerrors.BadRequest("INVALID_TIME_RANGE", "range must not exceed 7 days")
	}
	return from, to, nil
}
