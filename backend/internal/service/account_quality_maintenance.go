package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync/atomic"
	"time"

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

// RunTick captures live 15-minute stats for recent-traffic accounts, upserts non-empty
// snapshots, deletes rows older than 7 days, then invokes the hard-close hook.
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
	windowStart := now.Add(-AccountQualityWindow)

	ids, err := s.repo.ListRecentTrafficAccountIDs(ctx, windowStart)
	if err != nil {
		return err
	}

	reader, _ := s.usageLogs.(AccountQualityStatsBatchReader)
	allStats := make(map[int64]*AccountQualityStats, len(ids))
	for _, chunk := range chunkInt64IDs(ids, AccountQualityMaxBatchSize) {
		var stats map[int64]*AccountQualityStats
		if reader != nil {
			stats, err = reader.GetAccountQualityStatsBatch(ctx, chunk, windowStart)
			if err != nil {
				return err
			}
		}
		for _, id := range chunk {
			st := stats[id]
			if st == nil {
				st = BuildAccountQualityStats(0, 0, TTFTAggregate{})
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
