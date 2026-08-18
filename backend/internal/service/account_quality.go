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
// SuccessRate, ErrorRate, and TTFT fields are nil when the rolling window has no applicable samples.
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
	// ErrorRate is 1-SuccessRate when the window has success+error samples; nil when empty.
	ErrorRate *float64 `json:"error_rate"`
	// Bridge* is display-only Claude→GPT bridge traffic. It must not feed
	// scheduling gates or hard-close. ErrorCount above excludes these rows.
	// Account-dimension ErrorCount also excludes client/routing
	// model-not-found misses; user-dimension ErrorCount still counts them.
	BridgeSuccessCount int64    `json:"bridge_success_count"`
	BridgeErrorCount   int64    `json:"bridge_error_count"`
	BridgeErrorRate    *float64 `json:"bridge_error_rate"`
	// AvgTTFTMs kept for backward compatibility; prefer P50 for display.
	AvgTTFTMs   *int  `json:"avg_ttft_ms"`
	P50TTFTMs   *int  `json:"p50_ttft_ms"`
	P95TTFTMs   *int  `json:"p95_ttft_ms"`
	MaxTTFTMs   *int  `json:"max_ttft_ms"`
	TTFTSamples int64 `json:"ttft_samples"`
	// ResumeUsers is live-cache only: user_id -> unix until. After 立即恢复,
	// the chip stays 已恢复 until this timestamp.
	ResumeUsers map[string]int64 `json:"resume_users,omitempty"`
	// ResumeWatchingUsers is live-cache only: user_id -> unix until. After
	// 已恢复 ends (click or 15-minute auto), a new window accumulates and the
	// pair stays fail-open until this timestamp.
	ResumeWatchingUsers map[string]int64 `json:"resume_watching_users,omitempty"`
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
	NormalizeAccountQualityRates(stats)
	if ttft.Samples > 0 {
		stats.AvgTTFTMs = roundNonNegMs(ttft.Avg)
		stats.P50TTFTMs = roundNonNegMs(ttft.P50)
		stats.P95TTFTMs = roundNonNegMs(ttft.P95)
		stats.MaxTTFTMs = roundNonNegMs(ttft.Max)
	}
	return stats
}

// AttachBridgeQualityCounts sets the display-only bridge window and recomputes
// BridgeErrorRate. Scheduling ErrorCount / SuccessRate are left untouched.
func AttachBridgeQualityCounts(stats *AccountQualityStats, successCount, errorCount int64) {
	if stats == nil {
		return
	}
	stats.BridgeSuccessCount = successCount
	stats.BridgeErrorCount = errorCount
	stats.BridgeErrorRate = nil
	NormalizeAccountQualityRates(stats)
}

// QualityRateSamples is the success+error count used for rate display and gates.
func QualityRateSamples(stats *AccountQualityStats) int64 {
	if stats == nil {
		return 0
	}
	return stats.SuccessCount + stats.ErrorCount
}

// NormalizeAccountQualityRates keeps empty windows at null rates (never 0).
// Error-only windows still get SuccessRate=0 / ErrorRate=1 for gate math.
func NormalizeAccountQualityRates(stats *AccountQualityStats) {
	if stats == nil {
		return
	}
	total := QualityRateSamples(stats)
	if total <= 0 {
		stats.SuccessRate = nil
		stats.ErrorRate = nil
	} else {
		if stats.SuccessRate == nil {
			rate := float64(stats.SuccessCount) / float64(total)
			stats.SuccessRate = &rate
		}
		if stats.ErrorRate == nil {
			rate := float64(stats.ErrorCount) / float64(total)
			stats.ErrorRate = &rate
		}
	}
	bridgeTotal := stats.BridgeSuccessCount + stats.BridgeErrorCount
	if bridgeTotal <= 0 {
		stats.BridgeErrorRate = nil
		return
	}
	if stats.BridgeErrorRate == nil {
		rate := float64(stats.BridgeErrorCount) / float64(bridgeTotal)
		stats.BridgeErrorRate = &rate
	}
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

// SetUserQualityResume marks the 已恢复 chip phase until `until`.
func SetUserQualityResume(stats *AccountQualityStats, userID int64, until time.Time) {
	if stats == nil || userID <= 0 || until.IsZero() {
		return
	}
	if stats.ResumeUsers == nil {
		stats.ResumeUsers = make(map[string]int64, 1)
	}
	stats.ResumeUsers[qualityResumeUserKey(userID)] = until.UTC().Unix()
}

// SetUserQualityWatching marks the post-已恢复 accumulation window until `until`.
func SetUserQualityWatching(stats *AccountQualityStats, userID int64, until time.Time) {
	if stats == nil || userID <= 0 || until.IsZero() {
		return
	}
	if stats.ResumeWatchingUsers == nil {
		stats.ResumeWatchingUsers = make(map[string]int64, 1)
	}
	stats.ResumeWatchingUsers[qualityResumeUserKey(userID)] = until.UTC().Unix()
}

// ApplyUserQualityResume is 立即恢复: 已恢复 for one window, then accumulate
// another window (fail-open until now+30m).
func ApplyUserQualityResume(stats *AccountQualityStats, userID int64, now time.Time) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	SetUserQualityResume(stats, userID, now.Add(AccountQualityWindow))
	SetUserQualityWatching(stats, userID, now.Add(2*AccountQualityWindow))
}

// ClearUserQualityResume drops 已恢复 and the fail-open watching window.
func ClearUserQualityResume(stats *AccountQualityStats, userID int64) {
	if stats == nil || userID <= 0 {
		return
	}
	key := qualityResumeUserKey(userID)
	if stats.ResumeUsers != nil {
		delete(stats.ResumeUsers, key)
		if len(stats.ResumeUsers) == 0 {
			stats.ResumeUsers = nil
		}
	}
	if stats.ResumeWatchingUsers != nil {
		delete(stats.ResumeWatchingUsers, key)
		if len(stats.ResumeWatchingUsers) == 0 {
			stats.ResumeWatchingUsers = nil
		}
	}
}

// ApplyUserQualityWindowStart is 点已恢复: drop the 已恢复 chip and start a
// new accumulation window from now.
func ApplyUserQualityWindowStart(stats *AccountQualityStats, userID int64, now time.Time) {
	if stats == nil || userID <= 0 {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if stats.ResumeUsers != nil {
		delete(stats.ResumeUsers, qualityResumeUserKey(userID))
		if len(stats.ResumeUsers) == 0 {
			stats.ResumeUsers = nil
		}
	}
	SetUserQualityWatching(stats, userID, now.Add(AccountQualityWindow))
}

func userResumeUntilUnix(stats *AccountQualityStats, userID int64) (int64, bool) {
	if stats == nil || userID <= 0 || len(stats.ResumeUsers) == 0 {
		return 0, false
	}
	until, ok := stats.ResumeUsers[qualityResumeUserKey(userID)]
	return until, ok
}

func userWatchingUntilUnix(stats *AccountQualityStats, userID int64) (int64, bool) {
	if stats == nil || userID <= 0 || len(stats.ResumeWatchingUsers) == 0 {
		return 0, false
	}
	until, ok := stats.ResumeWatchingUsers[qualityResumeUserKey(userID)]
	return until, ok
}

// SetAccountQualityResume marks the account hard-close as not re-pausing until `until`.
func SetAccountQualityResume(stats *AccountQualityStats, until time.Time) {
	if stats == nil || until.IsZero() {
		return
	}
	unix := until.UTC().Unix()
	stats.AccountResumeUntil = &unix
}

// UserQualityResumeActive reports whether this pair is inside 已恢复 or the
// following accumulation window (fail-open).
func UserQualityResumeActive(stats *AccountQualityStats, userID int64, now time.Time) bool {
	if until, ok := userWatchingUntilUnix(stats, userID); ok && until > now.Unix() {
		return true
	}
	until, ok := userResumeUntilUnix(stats, userID)
	return ok && until > now.Unix()
}

// UserQualityResumedChipActive is the 已恢复 chip phase only.
func UserQualityResumedChipActive(stats *AccountQualityStats, userID int64, now time.Time) bool {
	until, ok := userResumeUntilUnix(stats, userID)
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
	for _, until := range stats.ResumeWatchingUsers {
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
	stats.ResumeUsers = pruneResumeMap(stats.ResumeUsers, now)
	stats.ResumeWatchingUsers = pruneResumeMap(stats.ResumeWatchingUsers, now)
}

func pruneResumeMap(in map[string]int64, now time.Time) map[string]int64 {
	if len(in) == 0 {
		return nil
	}
	kept := make(map[string]int64, len(in))
	for key, until := range in {
		if until > now.Unix() {
			kept[key] = until
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

func mergeResumeMap(dst *map[string]int64, src map[string]int64) {
	if dst == nil || len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[string]int64, len(src))
	}
	for key, until := range src {
		if existing, ok := (*dst)[key]; !ok || until > existing {
			(*dst)[key] = until
		}
	}
}

func MergeQualityResume(dst, src *AccountQualityStats, now time.Time) {
	if dst == nil {
		return
	}
	if src != nil {
		mergeResumeMap(&dst.ResumeUsers, src.ResumeUsers)
		mergeResumeMap(&dst.ResumeWatchingUsers, src.ResumeWatchingUsers)
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
	return stats.SuccessCount > 0 || stats.ErrorCount > 0 || stats.TTFTSamples > 0 ||
		stats.BridgeSuccessCount > 0 || stats.BridgeErrorCount > 0
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
	item := AccountQualityHistoryItem{
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
	NormalizeAccountQualityRates(&item.AccountQualityStats)
	return item
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

// SQLAccountQualityRoutingModelMissPredicate matches ops_error_logs rows that
// are client/routing "model does not exist / not supported" misses — including
// gateway 404 model_not_found (stored as error_type=api_error, phase=internal
// after normalize) and "no available accounts supporting model:" routing misses.
//
// Safety rails keep real upstream 429/502/503 in ErrorCount: status is only
// 400/403/404/503, error_phase is not upstream, and error_type is not an
// upstream/rate-limit class. Account-dimension scheduling ErrorCount uses the
// complementary SQLExcludeAccountQualityRoutingModelMiss; user quality does not.
func SQLAccountQualityRoutingModelMissPredicate() string {
	return `COALESCE(status_code, 0) IN (400, 403, 404, 503)` +
		` AND COALESCE(error_phase, '') <> 'upstream'` +
		` AND LOWER(COALESCE(error_type, '')) NOT IN ('upstream_error','overloaded_error','rate_limit_error')` +
		` AND (` +
		`LOWER(COALESCE(error_type, '')) = 'model_not_found'` +
		` OR LOWER(COALESCE(error_message, '')) LIKE '%model_not_found%'` +
		` OR LOWER(COALESCE(error_body, '')) LIKE '%model_not_found%'` +
		` OR LOWER(COALESCE(error_message, '')) LIKE '%unknown model%'` +
		` OR LOWER(COALESCE(error_message, '')) LIKE '%model not found%'` +
		` OR LOWER(COALESCE(error_message, '')) LIKE '%unsupported model%'` +
		` OR (LOWER(COALESCE(error_message, '')) LIKE '%model%' AND LOWER(COALESCE(error_message, '')) LIKE '%does not exist%')` +
		` OR LOWER(COALESCE(error_message, '')) LIKE '%not supported by any configured account%'` +
		` OR LOWER(COALESCE(error_message, '')) LIKE '%supporting model:%'` +
		` OR LOWER(COALESCE(error_message, '')) LIKE '%no account supports%'` +
		` OR (LOWER(COALESCE(error_message, '')) LIKE '%model%' AND LOWER(COALESCE(error_message, '')) LIKE '%not in whitelist%')` +
		`)`
}

// SQLExcludeAccountQualityRoutingModelMiss is the scheduling ErrorCount filter
// for the account quality window. Do not apply it to GetUserQualityStatsBatch.
func SQLExcludeAccountQualityRoutingModelMiss() string {
	return "NOT (" + SQLAccountQualityRoutingModelMissPredicate() + ")"
}
