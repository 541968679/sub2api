package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync/atomic"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	accountQualityMaintenanceTaskName      = "account-quality:maintenance"
	accountQualityMaintenanceLeaderLockKey = "account-quality:maintenance:leader"
	accountQualityMaintenanceLeaderLockTTL = 5 * time.Minute
	accountQualityMaintenanceTimeout       = 2 * time.Minute
)

// AccountQualityHardCloseEvaluator is the hook the hard-close child attaches.
// This snapshots child does not implement evaluation.
type AccountQualityHardCloseEvaluator interface {
	EvaluateHardClose(ctx context.Context, stats map[int64]*AccountQualityStats)
}

// AccountQualityMaintenanceService periodically snapshots the live 15-minute quality window.
type AccountQualityMaintenanceService struct {
	repo        AccountQualitySnapshotRepository
	usageLogs   UsageLogRepository
	timingWheel *TimingWheelService
	lockCache   LeaderLockCache
	db          *sql.DB
	instanceID  string
	hardClose   AccountQualityHardCloseEvaluator
	liveCache   AccountQualityLiveCache
	lastN       AccountQualityLastNCache
	settings    *SettingService
	running     int32
	stopped     int32
}

// NewAccountQualityMaintenanceService creates the snapshot maintenance service.
func NewAccountQualityMaintenanceService(
	repo AccountQualitySnapshotRepository,
	usageLogs UsageLogRepository,
	timingWheel *TimingWheelService,
) *AccountQualityMaintenanceService {
	return &AccountQualityMaintenanceService{
		repo:        repo,
		usageLogs:   usageLogs,
		timingWheel: timingWheel,
		instanceID:  uuid.NewString(),
	}
}

func (s *AccountQualityMaintenanceService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

// SetHardCloseEvaluator attaches the next child's hard-close evaluation.
func (s *AccountQualityMaintenanceService) SetHardCloseEvaluator(eval AccountQualityHardCloseEvaluator) {
	if s == nil {
		return
	}
	s.hardClose = eval
}

// SetLiveQualityCache attaches the Redis live-stats writer used by selection.
func (s *AccountQualityMaintenanceService) SetLiveQualityCache(cache AccountQualityLiveCache) {
	if s == nil {
		return
	}
	s.liveCache = cache
	if lastN, ok := cache.(AccountQualityLastNCache); ok {
		s.lastN = lastN
	}
}

func (s *AccountQualityMaintenanceService) windowN(ctx context.Context) int {
	if s == nil || s.settings == nil {
		return DefaultAccountQualityWindowN
	}
	cfg, err := s.settings.GetQualityHardCloseSettings(ctx)
	if err != nil || cfg == nil {
		return DefaultAccountQualityWindowN
	}
	return cfg.ResolvedWindowN()
}

func (s *AccountQualityMaintenanceService) ObserveAccountCompletion(ctx context.Context, obs AccountQualityObservation) {
	if s == nil || s.lastN == nil || obs.AccountID <= 0 {
		return
	}
	n := s.windowN(ctx)
	useFailover := s.scheduleUseFailoverErrorRate(ctx)
	live := s.lastN.IngestLastN(ctx, obs.AccountID, n, obs.Success, obs.FirstTokenMs, useFailover)
	if live == nil {
		return
	}
	stats := live.ToAccountQualityStats()
	if s.liveCache != nil {
		if merged, err := s.liveCache.Get(ctx, obs.AccountID); err == nil && merged != nil {
			stats = merged
		}
	}
	s.EvaluateHardClose(ctx, map[int64]*AccountQualityStats{obs.AccountID: stats})
}

// GetLastNStatsBatch returns last-N Q_a for account list cells. Missing keys are empty stats with N stamped.
func (s *AccountQualityMaintenanceService) GetLastNStatsBatch(ctx context.Context, accountIDs []int64) (map[int64]*AccountQualityStats, error) {
	ids := normalizeQualityBatchIDs(accountIDs)
	n := s.windowN(ctx)
	byID := map[int64]*AccountQualityLastN{}
	if s != nil && s.lastN != nil {
		byID = s.lastN.GetLastNBatch(ctx, ids)
	}
	out := make(map[int64]*AccountQualityStats, len(ids))
	for _, id := range ids {
		if live := byID[id]; live != nil {
			st := live.ToAccountQualityStats()
			StampAccountQualityWindowN(st, n)
			out[id] = st
			continue
		}
		st := BuildAccountQualityStats(0, 0, TTFTAggregate{})
		StampAccountQualityWindowN(st, n)
		out[id] = st
	}
	return out, nil
}

// SetQualitySettings lets the tick apply the failover-as-schedule toggle
// (default off) before writing live Redis / snapshots / hard-close.
func (s *AccountQualityMaintenanceService) SetQualitySettings(settings *SettingService) {
	if s == nil {
		return
	}
	s.settings = settings
}

func (s *AccountQualityMaintenanceService) scheduleUseFailoverErrorRate(ctx context.Context) bool {
	if s == nil || s.settings == nil {
		return false
	}
	cfg, err := s.settings.GetQualityHardCloseSettings(ctx)
	return err == nil && cfg != nil && cfg.ScheduleUseFailoverErrorRate
}

// ResumeUserQuality force-admits one pair for one quality window without changing the gate.
func (s *AccountQualityMaintenanceService) ResumeUserQuality(ctx context.Context, accountID, userID int64) error {
	if s == nil || s.liveCache == nil {
		return infraerrors.ServiceUnavailable("QUALITY_RESUME_UNAVAILABLE", "quality live cache unavailable")
	}
	return s.liveCache.MarkUserResume(ctx, accountID, userID)
}

// StartUserQualityWindow ends the 已恢复 chip and starts a new 15-minute window.
func (s *AccountQualityMaintenanceService) StartUserQualityWindow(ctx context.Context, accountID, userID int64) error {
	if s == nil || s.liveCache == nil {
		return infraerrors.ServiceUnavailable("QUALITY_RESUME_UNAVAILABLE", "quality live cache unavailable")
	}
	return s.liveCache.MarkUserQualityWindow(ctx, accountID, userID)
}

// ResumeAccountQuality prevents hard-close from re-pausing for one quality window.
func (s *AccountQualityMaintenanceService) ResumeAccountQuality(ctx context.Context, accountID int64) error {
	if s == nil || s.liveCache == nil {
		return infraerrors.ServiceUnavailable("QUALITY_RESUME_UNAVAILABLE", "quality live cache unavailable")
	}
	return s.liveCache.MarkAccountResume(ctx, accountID)
}

// EvaluateHardClose is a no-op unless SetHardCloseEvaluator was called.
// The next child should implement AccountQualityHardCloseEvaluator and attach it here.
func (s *AccountQualityMaintenanceService) EvaluateHardClose(ctx context.Context, stats map[int64]*AccountQualityStats) {
	if s == nil || s.hardClose == nil {
		return
	}
	s.hardClose.EvaluateHardClose(ctx, stats)
}

// Start schedules the 5-minute recurring snapshot job.
func (s *AccountQualityMaintenanceService) Start() {
	if s == nil || s.repo == nil || s.timingWheel == nil {
		return
	}
	s.timingWheel.ScheduleRecurring(accountQualityMaintenanceTaskName, AccountQualitySnapshotInterval, s.tick)
	logger.LegacyPrintf("service.account_quality_maintenance", "[AccountQualityMaintenance] started (interval=%v, window=%v, retention=%v)",
		AccountQualitySnapshotInterval, AccountQualityWindow, AccountQualitySnapshotRetention)
}

// Stop cancels the recurring job so shutdown does not start another tick.
func (s *AccountQualityMaintenanceService) Stop() {
	if s == nil {
		return
	}
	atomic.StoreInt32(&s.stopped, 1)
	if s.timingWheel != nil {
		s.timingWheel.Cancel(accountQualityMaintenanceTaskName)
	}
}

func (s *AccountQualityMaintenanceService) tick() {
	if s == nil || atomic.LoadInt32(&s.stopped) == 1 {
		return
	}
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&s.running, 0)

	ctx, cancel := context.WithTimeout(context.Background(), accountQualityMaintenanceTimeout)
	defer cancel()

	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, accountQualityMaintenanceLeaderLockKey, s.instanceID, accountQualityMaintenanceLeaderLockTTL)
	if !ok {
		return
	}
	defer release()

	if err := s.RunTick(ctx, time.Now().UTC()); err != nil {
		logger.LegacyPrintf("service.account_quality_maintenance", "[AccountQualityMaintenance] tick failed: %v", err)
	}
}

// RunTick snapshots last-N live Q_a, deletes expired history rows, then
// re-evaluates hard-close. It does not recompute from a 15-minute SQL window.
func (s *AccountQualityMaintenanceService) RunTick(ctx context.Context, now time.Time) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	capturedAt := TruncateToAccountQualitySnapshotTime(now)
	n := s.windowN(ctx)
	var ids []int64
	if s.lastN != nil {
		ids = s.lastN.ListLastNAccountIDs(ctx)
	}
	allStats := make(map[int64]*AccountQualityStats)
	if s.lastN != nil && len(ids) > 0 {
		for _, chunk := range chunkInt64IDs(ids, AccountQualityMaxBatchSize) {
			lives := s.lastN.GetLastNBatch(ctx, chunk)
			for _, id := range chunk {
				live := lives[id]
				if live == nil {
					continue
				}
				st := live.ToAccountQualityStats()
				StampAccountQualityWindowN(st, n)
				if s.liveCache != nil {
					if merged, err := s.liveCache.Get(ctx, id); err == nil && merged != nil {
						st = merged
						StampAccountQualityWindowN(st, n)
					}
				}
				allStats[id] = st
				if !HasAccountQualitySamples(st) {
					continue
				}
				if err := s.repo.Upsert(ctx, SnapshotFromAccountQualityStats(id, capturedAt, st)); err != nil {
					return err
				}
			}
		}
	}

	cutoff := now.Add(-AccountQualitySnapshotRetention)
	for {
		deleted, delErr := s.repo.DeleteExpired(ctx, cutoff, AccountQualitySnapshotDeleteBatchSize)
		if delErr != nil {
			return delErr
		}
		if deleted == 0 {
			break
		}
	}

	s.EvaluateHardClose(ctx, allStats)
	slog.Debug("[AccountQualityMaintenance] tick complete",
		"candidates", len(ids),
		"captured_at", capturedAt.Format(time.RFC3339),
	)
	return nil
}

// ListHistory returns snapshot points for one account. Zero from/to use the 24h default.
func (s *AccountQualityMaintenanceService) ListHistory(ctx context.Context, accountID int64, from, to time.Time) ([]AccountQualityHistoryItem, error) {
	normalizedFrom, normalizedTo, err := NormalizeAccountQualityHistoryRange(from, to, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if s == nil || s.repo == nil {
		return []AccountQualityHistoryItem{}, nil
	}
	rows, err := s.repo.ListByAccount(ctx, accountID, normalizedFrom, normalizedTo)
	if err != nil {
		return nil, err
	}
	items := make([]AccountQualityHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.ToHistoryItem())
	}
	return items, nil
}

func chunkInt64IDs(ids []int64, size int) [][]int64 {
	if len(ids) == 0 {
		return nil
	}
	if size <= 0 {
		size = AccountQualityMaxBatchSize
	}
	out := make([][]int64, 0, (len(ids)+size-1)/size)
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[start:end])
	}
	return out
}
