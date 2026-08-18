//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type qualitySnapshotRepoStub struct {
	candidates    []int64
	rows          map[snapshotKey]AccountQualitySnapshotRow
	upserts       []AccountQualitySnapshotRow
	deleteCutoffs []time.Time
	listFrom      time.Time
	listTo        time.Time
	listID        int64
}

type snapshotKey struct {
	accountID  int64
	capturedAt time.Time
}

func (s *qualitySnapshotRepoStub) Upsert(_ context.Context, row AccountQualitySnapshotRow) error {
	if s.rows == nil {
		s.rows = map[snapshotKey]AccountQualitySnapshotRow{}
	}
	key := snapshotKey{accountID: row.AccountID, capturedAt: row.CapturedAt.UTC()}
	s.rows[key] = row
	s.upserts = append(s.upserts, row)
	return nil
}

func (s *qualitySnapshotRepoStub) ListByAccount(_ context.Context, accountID int64, from, to time.Time) ([]AccountQualitySnapshotRow, error) {
	s.listID = accountID
	s.listFrom = from
	s.listTo = to
	out := make([]AccountQualitySnapshotRow, 0, len(s.rows))
	for _, row := range s.rows {
		if row.AccountID == accountID && !row.CapturedAt.Before(from) && !row.CapturedAt.After(to) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *qualitySnapshotRepoStub) DeleteExpired(_ context.Context, cutoff time.Time, _ int) (int64, error) {
	s.deleteCutoffs = append(s.deleteCutoffs, cutoff)
	var deleted int64
	for key, row := range s.rows {
		if row.CapturedAt.Before(cutoff) {
			delete(s.rows, key)
			deleted++
		}
	}
	return deleted, nil
}

func (s *qualitySnapshotRepoStub) ListRecentTrafficAccountIDs(_ context.Context, _ time.Time) ([]int64, error) {
	return append([]int64(nil), s.candidates...), nil
}

func TestAccountQualityMaintenance_SkipEmptySamples(t *testing.T) {
	rate := 1.0
	ttft := 120
	repo := &qualitySnapshotRepoStub{candidates: []int64{1, 2, 3}}
	statsRepo := &qualityStatsBatchRepoStub{
		result: map[int64]*AccountQualityStats{
			1: {WindowSeconds: AccountQualityWindowSeconds, SuccessCount: 2, SuccessRate: &rate, AvgTTFTMs: &ttft, TTFTSamples: 2},
			2: {WindowSeconds: AccountQualityWindowSeconds},
			3: {WindowSeconds: AccountQualityWindowSeconds, ErrorCount: 1, SuccessRate: new(float64)},
		},
	}
	svc := NewAccountQualityMaintenanceService(repo, statsRepo, nil)

	now := time.Date(2026, 8, 14, 12, 7, 30, 0, time.UTC)
	require.NoError(t, svc.RunTick(context.Background(), now))

	require.Len(t, repo.rows, 2)
	capturedAt := TruncateToAccountQualitySnapshotTime(now)
	require.Equal(t, time.Date(2026, 8, 14, 12, 5, 0, 0, time.UTC), capturedAt)
	_, hasEmpty := repo.rows[snapshotKey{accountID: 2, capturedAt: capturedAt}]
	require.False(t, hasEmpty)
	require.Contains(t, repo.rows, snapshotKey{accountID: 1, capturedAt: capturedAt})
	require.Contains(t, repo.rows, snapshotKey{accountID: 3, capturedAt: capturedAt})
	errorOnly := repo.rows[snapshotKey{accountID: 3, capturedAt: capturedAt}]
	require.Equal(t, AccountQualityWindowSeconds, errorOnly.WindowSeconds)
	require.NotNil(t, errorOnly.SuccessRate)
	require.InDelta(t, 0.0, *errorOnly.SuccessRate, 1e-9)
	require.Nil(t, errorOnly.P50TTFTMs)
	require.Nil(t, errorOnly.P95TTFTMs)
}

func TestAccountQualityMaintenance_UpsertIdempotent(t *testing.T) {
	rate := 0.8
	repo := &qualitySnapshotRepoStub{candidates: []int64{9}}
	statsRepo := &qualityStatsBatchRepoStub{
		result: map[int64]*AccountQualityStats{
			9: {WindowSeconds: AccountQualityWindowSeconds, SuccessCount: 4, ErrorCount: 1, SuccessRate: &rate, TTFTSamples: 4},
		},
	}
	svc := NewAccountQualityMaintenanceService(repo, statsRepo, nil)
	now := time.Date(2026, 8, 14, 12, 10, 1, 0, time.UTC)

	require.NoError(t, svc.RunTick(context.Background(), now))
	statsRepo.result[9].SuccessCount = 6
	require.NoError(t, svc.RunTick(context.Background(), now))

	require.Len(t, repo.rows, 1)
	require.Len(t, repo.upserts, 2)
	got := repo.rows[snapshotKey{accountID: 9, capturedAt: TruncateToAccountQualitySnapshotTime(now)}]
	require.Equal(t, int64(6), got.SuccessCount)
}

func TestAccountQualityMaintenance_BatchesBy200(t *testing.T) {
	ids := make([]int64, AccountQualityMaxBatchSize+3)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	repo := &qualitySnapshotRepoStub{candidates: ids}
	statsRepo := &qualityStatsBatchRepoStub{result: map[int64]*AccountQualityStats{}}
	svc := NewAccountQualityMaintenanceService(repo, statsRepo, nil)

	require.NoError(t, svc.RunTick(context.Background(), time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)))
	require.Len(t, statsRepo.batchCalls, 2)
	require.Len(t, statsRepo.batchCalls[0], AccountQualityMaxBatchSize)
	require.Len(t, statsRepo.batchCalls[1], 3)
	require.Empty(t, repo.rows)
}

func TestAccountQualityMaintenance_RetentionDelete(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	oldAt := now.Add(-8 * 24 * time.Hour)
	keepAt := now.Add(-2 * 24 * time.Hour)
	repo := &qualitySnapshotRepoStub{
		candidates: []int64{},
		rows: map[snapshotKey]AccountQualitySnapshotRow{
			{accountID: 1, capturedAt: oldAt}:  {AccountID: 1, CapturedAt: oldAt, SuccessCount: 1},
			{accountID: 1, capturedAt: keepAt}: {AccountID: 1, CapturedAt: keepAt, SuccessCount: 2},
		},
	}
	svc := NewAccountQualityMaintenanceService(repo, &qualityStatsBatchRepoStub{result: map[int64]*AccountQualityStats{}}, nil)
	require.NoError(t, svc.RunTick(context.Background(), now))

	require.NotEmpty(t, repo.deleteCutoffs)
	require.Equal(t, now.Add(-AccountQualitySnapshotRetention), repo.deleteCutoffs[0])
	require.Len(t, repo.rows, 1)
	require.Contains(t, repo.rows, snapshotKey{accountID: 1, capturedAt: keepAt})
}

func TestNormalizeAccountQualityHistoryRange_DefaultWindow(t *testing.T) {
	now := time.Date(2026, 8, 14, 15, 0, 0, 0, time.UTC)
	from, to, err := NormalizeAccountQualityHistoryRange(time.Time{}, time.Time{}, now)
	require.NoError(t, err)
	require.Equal(t, now, to)
	require.Equal(t, now.Add(-AccountQualityHistoryDefaultRange), from)
}

func TestNormalizeAccountQualityHistoryRange_RejectsOver7Days(t *testing.T) {
	now := time.Date(2026, 8, 14, 15, 0, 0, 0, time.UTC)
	_, _, err := NormalizeAccountQualityHistoryRange(now.Add(-8*24*time.Hour), now, now)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestAccountQualityMaintenance_ListHistory_DefaultWindowAndCap(t *testing.T) {
	now := time.Now().UTC()
	repo := &qualitySnapshotRepoStub{
		rows: map[snapshotKey]AccountQualitySnapshotRow{
			{accountID: 4, capturedAt: now.Add(-2 * time.Hour)}: {
				AccountID: 4, CapturedAt: now.Add(-2 * time.Hour), WindowSeconds: AccountQualityWindowSeconds, SuccessCount: 1,
			},
		},
	}
	svc := NewAccountQualityMaintenanceService(repo, nil, nil)

	items, err := svc.ListHistory(context.Background(), 4, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(4), repo.listID)
	require.WithinDuration(t, now.Add(-AccountQualityHistoryDefaultRange), repo.listFrom, 2*time.Second)
	require.WithinDuration(t, now, repo.listTo, 2*time.Second)

	_, err = svc.ListHistory(context.Background(), 4, now.Add(-8*24*time.Hour), now)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestEvaluateHardClose_NoOpWithoutEvaluator(t *testing.T) {
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.EvaluateHardClose(context.Background(), map[int64]*AccountQualityStats{1: {}})
}

type hardCloseHookStub struct {
	called bool
	stats  map[int64]*AccountQualityStats
}

func (h *hardCloseHookStub) EvaluateHardClose(_ context.Context, stats map[int64]*AccountQualityStats) {
	h.called = true
	h.stats = stats
}

func TestAccountQualityMaintenance_HardCloseHookReceivesLiveStats(t *testing.T) {
	repo := &qualitySnapshotRepoStub{candidates: []int64{1}}
	statsRepo := &qualityStatsBatchRepoStub{
		result: map[int64]*AccountQualityStats{
			1: {WindowSeconds: AccountQualityWindowSeconds, SuccessCount: 3, TTFTSamples: 3},
		},
	}
	hook := &hardCloseHookStub{}
	svc := NewAccountQualityMaintenanceService(repo, statsRepo, nil)
	svc.SetHardCloseEvaluator(hook)

	require.NoError(t, svc.RunTick(context.Background(), time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)))
	require.True(t, hook.called)
	require.Equal(t, int64(3), hook.stats[1].SuccessCount)
}

type liveQualityCacheCapture struct {
	calls int
	last  map[int64]*AccountQualityStats
}

func (s *liveQualityCacheCapture) Get(_ context.Context, accountID int64) (*AccountQualityStats, error) {
	if s == nil || s.last == nil {
		return nil, nil
	}
	return s.last[accountID], nil
}

func (s *liveQualityCacheCapture) Replace(_ context.Context, stats map[int64]*AccountQualityStats) error {
	s.calls++
	s.last = stats
	return nil
}

func (s *liveQualityCacheCapture) MarkUserResume(_ context.Context, accountID, userID int64) error {
	if s.last == nil {
		s.last = map[int64]*AccountQualityStats{}
	}
	st := s.last[accountID]
	if st == nil {
		st = &AccountQualityStats{}
		s.last[accountID] = st
	}
	ApplyUserQualityResume(st, userID, time.Now().UTC())
	return nil
}

func (s *liveQualityCacheCapture) ClearUserResume(_ context.Context, accountID, userID int64) error {
	if s.last == nil {
		return nil
	}
	ClearUserQualityResume(s.last[accountID], userID)
	return nil
}

func (s *liveQualityCacheCapture) MarkUserQualityWindow(_ context.Context, accountID, userID int64) error {
	if s.last == nil {
		s.last = map[int64]*AccountQualityStats{}
	}
	st := s.last[accountID]
	if st == nil {
		st = &AccountQualityStats{}
		s.last[accountID] = st
	}
	ApplyUserQualityWindowStart(st, userID, time.Now().UTC())
	return nil
}

func (s *liveQualityCacheCapture) MarkAccountResume(_ context.Context, accountID int64) error {
	if s.last == nil {
		s.last = map[int64]*AccountQualityStats{}
	}
	st := s.last[accountID]
	if st == nil {
		st = &AccountQualityStats{}
		s.last[accountID] = st
	}
	SetAccountQualityResume(st, time.Now().UTC().Add(AccountQualityWindow))
	return nil
}

func TestAccountQualityMaintenance_WritesLiveQualityCache(t *testing.T) {
	rate := 1.0
	ttft := 120
	repo := &qualitySnapshotRepoStub{candidates: []int64{1, 2}}
	statsRepo := &qualityStatsBatchRepoStub{
		result: map[int64]*AccountQualityStats{
			1: {WindowSeconds: AccountQualityWindowSeconds, SuccessCount: 2, SuccessRate: &rate, AvgTTFTMs: &ttft, P50TTFTMs: &ttft, TTFTSamples: 2},
			2: {WindowSeconds: AccountQualityWindowSeconds},
		},
	}
	live := &liveQualityCacheCapture{}
	svc := NewAccountQualityMaintenanceService(repo, statsRepo, nil)
	svc.SetLiveQualityCache(live)

	require.NoError(t, svc.RunTick(context.Background(), time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)))
	require.Equal(t, 1, live.calls)
	require.NotNil(t, live.last[1])
	require.Equal(t, int64(2), live.last[1].SuccessCount)
	require.True(t, HasAccountQualitySamples(live.last[1]))
	require.NotNil(t, live.last[2])
	require.False(t, HasAccountQualitySamples(live.last[2]))
}
