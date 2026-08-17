package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	SchedulePnlRange24h       = "24h"
	SchedulePnlRangeToday     = "today"
	SchedulePnlRangeYesterday = "yesterday"
	SchedulePnlRange7d        = "7d"
	schedulePnlMaxBatch       = 200
)

// SchedulePnlWindow is one calendar/rolling window of pair or user-total PnL.
// Margin is nil when revenue is 0.
type SchedulePnlWindow struct {
	Revenue float64  `json:"revenue"`
	Cost    float64  `json:"cost"`
	Profit  float64  `json:"profit"`
	Margin  *float64 `json:"margin"`
}

// SchedulePnlSummary is today + last 7 calendar days. A window is nil when it has no true_cost rows.
type SchedulePnlSummary struct {
	Today    *SchedulePnlWindow `json:"today"`
	SevenDay *SchedulePnlWindow `json:"seven_day"`
}

// SchedulePnlTrendPoint is one continuous bucket. Empty buckets keep nulls, not zeros.
type SchedulePnlTrendPoint struct {
	Bucket  string   `json:"bucket"`
	Revenue *float64 `json:"revenue"`
	Cost    *float64 `json:"cost"`
	Profit  *float64 `json:"profit"`
	Margin  *float64 `json:"margin"`
}

// SchedulePnlTrend is the curve payload for one pair or one user's enabled-pool total.
type SchedulePnlTrend struct {
	Range       string                 `json:"range"`
	Granularity string                 `json:"granularity"`
	Points      []SchedulePnlTrendPoint `json:"points"`
}

// SchedulePnlAgg is a raw SUM over true_cost IS NOT NULL rows.
type SchedulePnlAgg struct {
	Revenue float64
	Cost    float64
	Rows    int64
}

// SchedulePnlRepository aggregates usage_logs for admin smart-schedule PnL.
type SchedulePnlRepository interface {
	SumSchedulePnl(ctx context.Context, userID int64, accountIDs []int64, start, end time.Time) (SchedulePnlAgg, error)
	SumSchedulePnlByAccount(ctx context.Context, userID int64, accountIDs []int64, start, end time.Time) (map[int64]SchedulePnlAgg, error)
	SumSchedulePnlByUserPairs(ctx context.Context, pairs []SchedulePnlUserAccount, start, end time.Time) (map[int64]SchedulePnlAgg, error)
	ListSchedulePnlTrend(ctx context.Context, userID int64, accountIDs []int64, start, end time.Time, granularity string, loc *time.Location) (map[string]SchedulePnlAgg, error)
}

// SchedulePnlUserAccount is one enabled-pool pair used for user totals.
type SchedulePnlUserAccount struct {
	UserID    int64
	AccountID int64
}

// SchedulePnlService is the admin read surface for schedule PnL.
type SchedulePnlService struct {
	pnl   SchedulePnlRepository
	usage UsageLogRepository
	smart UserSmartScheduleRepository
}

func NewSchedulePnlService(usage UsageLogRepository, smart UserSmartScheduleRepository) *SchedulePnlService {
	pnl, _ := usage.(SchedulePnlRepository)
	return &SchedulePnlService{pnl: pnl, usage: usage, smart: smart}
}

func (s *SchedulePnlService) pnlRepo() SchedulePnlRepository {
	if s == nil {
		return nil
	}
	return s.pnl
}

func BuildSchedulePnlWindow(agg SchedulePnlAgg) *SchedulePnlWindow {
	if agg.Rows == 0 {
		return nil
	}
	window := &SchedulePnlWindow{
		Revenue: agg.Revenue,
		Cost:    agg.Cost,
		Profit:  agg.Revenue - agg.Cost,
	}
	if agg.Revenue != 0 {
		margin := (agg.Revenue - agg.Cost) / agg.Revenue
		window.Margin = &margin
	}
	return window
}

func enabledPoolAccountIDs(bundle *UserSmartScheduleBundle) []int64 {
	if bundle == nil {
		return nil
	}
	seen := make(map[int64]struct{})
	var ids []int64
	for _, platform := range AllowedQuotaPlatforms {
		policy := bundle.EnabledPolicy(platform)
		if policy == nil {
			continue
		}
		for accountID := range policy.AccountIDs {
			if accountID <= 0 {
				continue
			}
			if _, ok := seen[accountID]; ok {
				continue
			}
			seen[accountID] = struct{}{}
			ids = append(ids, accountID)
		}
	}
	return ids
}

func userHasEnabledSmartSchedule(bundle *UserSmartScheduleBundle) bool {
	return len(enabledPoolAccountIDs(bundle)) > 0
}

func schedulePnlWindows(today, seven SchedulePnlAgg) SchedulePnlSummary {
	return SchedulePnlSummary{
		Today:    BuildSchedulePnlWindow(today),
		SevenDay: BuildSchedulePnlWindow(seven),
	}
}

func schedulePnlDayBounds(now time.Time, userTZ string) (todayStart, tomorrowStart, sevenStart time.Time) {
	todayStart = timezone.StartOfDayInUserLocation(now, userTZ)
	tomorrowStart = todayStart.AddDate(0, 0, 1)
	sevenStart = todayStart.AddDate(0, 0, -6)
	return todayStart, tomorrowStart, sevenStart
}

func (s *SchedulePnlService) PairSummaries(ctx context.Context, userID int64, accountIDs []int64, userTZ string) (map[string]SchedulePnlSummary, error) {
	out := make(map[string]SchedulePnlSummary, len(accountIDs))
	for _, accountID := range accountIDs {
		if accountID > 0 {
			out[strconv.FormatInt(accountID, 10)] = SchedulePnlSummary{}
		}
	}
	repo := s.pnlRepo()
	if repo == nil || userID <= 0 || len(accountIDs) == 0 {
		return out, nil
	}
	now := timezone.NowInUserLocation(userTZ)
	todayStart, tomorrowStart, sevenStart := schedulePnlDayBounds(now, userTZ)
	todayByAccount, err := repo.SumSchedulePnlByAccount(ctx, userID, accountIDs, todayStart, tomorrowStart)
	if err != nil {
		return nil, err
	}
	sevenByAccount, err := repo.SumSchedulePnlByAccount(ctx, userID, accountIDs, sevenStart, tomorrowStart)
	if err != nil {
		return nil, err
	}
	for _, accountID := range accountIDs {
		if accountID <= 0 {
			continue
		}
		out[strconv.FormatInt(accountID, 10)] = schedulePnlWindows(todayByAccount[accountID], sevenByAccount[accountID])
	}
	return out, nil
}

func (s *SchedulePnlService) UserSummaries(ctx context.Context, userIDs []int64, userTZ string) (map[string]SchedulePnlSummary, error) {
	if len(userIDs) > schedulePnlMaxBatch {
		userIDs = userIDs[:schedulePnlMaxBatch]
	}
	out := make(map[string]SchedulePnlSummary)
	if s == nil || s.smart == nil || len(userIDs) == 0 {
		return out, nil
	}
	bundles, err := s.smart.ListByUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	var pairs []SchedulePnlUserAccount
	enabledUsers := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		bundle := bundles[userID]
		ids := enabledPoolAccountIDs(bundle)
		if len(ids) == 0 {
			continue
		}
		enabledUsers = append(enabledUsers, userID)
		for _, accountID := range ids {
			pairs = append(pairs, SchedulePnlUserAccount{UserID: userID, AccountID: accountID})
		}
	}
	if len(enabledUsers) == 0 {
		return out, nil
	}
	for _, userID := range enabledUsers {
		out[strconv.FormatInt(userID, 10)] = SchedulePnlSummary{}
	}
	repo := s.pnlRepo()
	if repo == nil {
		return out, nil
	}
	now := timezone.NowInUserLocation(userTZ)
	todayStart, tomorrowStart, sevenStart := schedulePnlDayBounds(now, userTZ)
	todayByUser, err := repo.SumSchedulePnlByUserPairs(ctx, pairs, todayStart, tomorrowStart)
	if err != nil {
		return nil, err
	}
	sevenByUser, err := repo.SumSchedulePnlByUserPairs(ctx, pairs, sevenStart, tomorrowStart)
	if err != nil {
		return nil, err
	}
	for _, userID := range enabledUsers {
		out[strconv.FormatInt(userID, 10)] = schedulePnlWindows(todayByUser[userID], sevenByUser[userID])
	}
	return out, nil
}

func normalizeSchedulePnlRange(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", SchedulePnlRange24h:
		return SchedulePnlRange24h, nil
	case SchedulePnlRangeToday, SchedulePnlRangeYesterday, SchedulePnlRange7d:
		return strings.ToLower(strings.TrimSpace(raw)), nil
	default:
		return "", infraerrors.BadRequest("SCHEDULE_PNL_INVALID_RANGE", "range must be 24h, today, yesterday, or 7d")
	}
}

func schedulePnlUserLocation(userTZ string) *time.Location {
	if userTZ != "" {
		if loc, err := time.LoadLocation(userTZ); err == nil {
			return loc
		}
	}
	return timezone.Location()
}

func schedulePnlTrendBounds(rangeKey, userTZ string) (start, end time.Time, granularity string, buckets []string) {
	loc := schedulePnlUserLocation(userTZ)
	now := timezone.NowInUserLocation(userTZ)
	switch rangeKey {
	case SchedulePnlRangeToday:
		start = timezone.StartOfDayInUserLocation(now, userTZ)
		end = start.AddDate(0, 0, 1)
		granularity = "hour"
		for hour := 0; hour < 24; hour++ {
			buckets = append(buckets, start.Add(time.Duration(hour)*time.Hour).In(loc).Format("2006-01-02 15:00"))
		}
	case SchedulePnlRangeYesterday:
		end = timezone.StartOfDayInUserLocation(now, userTZ)
		start = end.AddDate(0, 0, -1)
		granularity = "hour"
		for hour := 0; hour < 24; hour++ {
			buckets = append(buckets, start.Add(time.Duration(hour)*time.Hour).In(loc).Format("2006-01-02 15:00"))
		}
	case SchedulePnlRange7d:
		today := timezone.StartOfDayInUserLocation(now, userTZ)
		start = today.AddDate(0, 0, -6)
		end = today.AddDate(0, 0, 1)
		granularity = "day"
		for day := 0; day < 7; day++ {
			buckets = append(buckets, start.AddDate(0, 0, day).In(loc).Format("2006-01-02"))
		}
	default:
		currentHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
		end = currentHour.Add(time.Hour)
		start = end.Add(-24 * time.Hour)
		granularity = "hour"
		for i := 0; i < 24; i++ {
			buckets = append(buckets, start.Add(time.Duration(i)*time.Hour).In(loc).Format("2006-01-02 15:00"))
		}
	}
	return start, end, granularity, buckets
}

func fillSchedulePnlTrend(buckets []string, byBucket map[string]SchedulePnlAgg) []SchedulePnlTrendPoint {
	points := make([]SchedulePnlTrendPoint, 0, len(buckets))
	for _, bucket := range buckets {
		agg, ok := byBucket[bucket]
		if !ok || agg.Rows == 0 {
			points = append(points, SchedulePnlTrendPoint{Bucket: bucket})
			continue
		}
		revenue := agg.Revenue
		cost := agg.Cost
		profit := agg.Revenue - agg.Cost
		point := SchedulePnlTrendPoint{
			Bucket:  bucket,
			Revenue: &revenue,
			Cost:    &cost,
			Profit:  &profit,
		}
		if agg.Revenue != 0 {
			margin := (agg.Revenue - agg.Cost) / agg.Revenue
			point.Margin = &margin
		}
		points = append(points, point)
	}
	return points
}

func (s *SchedulePnlService) Trend(ctx context.Context, userID int64, accountID *int64, rangeKey, userTZ string) (*SchedulePnlTrend, error) {
	normalized, err := normalizeSchedulePnlRange(rangeKey)
	if err != nil {
		return nil, err
	}
	if s == nil || s.smart == nil || userID <= 0 {
		start, _, granularity, buckets := schedulePnlTrendBounds(normalized, userTZ)
		_ = start
		return &SchedulePnlTrend{Range: normalized, Granularity: granularity, Points: fillSchedulePnlTrend(buckets, nil)}, nil
	}
	bundle, err := s.smart.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	poolIDs := enabledPoolAccountIDs(bundle)
	var filterIDs []int64
	if accountID != nil && *accountID > 0 {
		allowed := false
		for _, id := range poolIDs {
			if id == *accountID {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, infraerrors.BadRequest("SCHEDULE_PNL_ACCOUNT_NOT_IN_POOL", "account is not in an enabled smart-schedule pool")
		}
		filterIDs = []int64{*accountID}
	} else {
		filterIDs = poolIDs
	}
	start, end, granularity, buckets := schedulePnlTrendBounds(normalized, userTZ)
	repo := s.pnlRepo()
	var byBucket map[string]SchedulePnlAgg
	if repo != nil && len(filterIDs) > 0 {
		byBucket, err = repo.ListSchedulePnlTrend(ctx, userID, filterIDs, start, end, granularity, schedulePnlUserLocation(userTZ))
		if err != nil {
			return nil, err
		}
	}
	return &SchedulePnlTrend{
		Range:       normalized,
		Granularity: granularity,
		Points:      fillSchedulePnlTrend(buckets, byBucket),
	}, nil
}
