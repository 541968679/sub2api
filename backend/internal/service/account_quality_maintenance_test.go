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

type lastNCacheStub struct {
	byID       map[int64]*AccountQualityLastN
	ids        []int64
	batchCalls [][]int64
}

func (s *lastNCacheStub) GetLastN(_ context.Context, accountID int64) *AccountQualityLastN {
	if s == nil || s.byID == nil {
		return nil
	}
	return s.byID[accountID]
}

func (s *lastNCacheStub) GetLastNBatch(_ context.Context, accountIDs []int64) map[int64]*AccountQualityLastN {
	s.batchCalls = append(s.batchCalls, append([]int64(nil), accountIDs...))
	out := map[int64]*AccountQualityLastN{}
	if s == nil || s.byID == nil {
		return out
	}
	for _, id := range accountIDs {
		if live := s.byID[id]; live != nil {
			out[id] = live
		}
	}
	return out
}

func (s *lastNCacheStub) IngestLastN(_ context.Context, accountID int64, n int, success bool, firstTokenMs, durationMs *int, useFailover bool) *AccountQualityLastN {
	if s.byID == nil {
		s.byID = map[int64]*AccountQualityLastN{}
	}
	live := ApplyAccountQualityLastNIngest(s.byID[accountID], n, success, firstTokenMs, durationMs)
	live.UseFailover = useFailover
	s.byID[accountID] = live
	return live
}

func (s *lastNCacheStub) GetUserLastN(ctx context.Context, userID int64) *AccountQualityLastN {
	return s.GetLastN(ctx, userID)
}

func (s *lastNCacheStub) GetUserLastNBatch(ctx context.Context, userIDs []int64) map[int64]*AccountQualityLastN {
	return s.GetLastNBatch(ctx, userIDs)
}

func (s *lastNCacheStub) IngestUserLastN(ctx context.Context, userID int64, n int, success bool, firstTokenMs, durationMs *int, useFailover bool, override *int) *AccountQualityLastN {
	live := s.IngestLastN(ctx, userID, n, success, firstTokenMs, durationMs, useFailover)
	if live != nil {
		live.OverrideN = CopyIntPtr(override)
	}
	return live
}

func (s *lastNCacheStub) ResizeUserLastN(_ context.Context, userID int64, n int, override *int) *AccountQualityLastN {
	if s.byID == nil {
		s.byID = map[int64]*AccountQualityLastN{}
	}
	live := ProjectAccountQualityLastN(s.byID[userID], n)
	live.OverrideN = CopyIntPtr(override)
	s.byID[userID] = live
	return live
}

func (s *lastNCacheStub) ListUserLastNIDs(ctx context.Context) []int64 {
	return s.ListLastNAccountIDs(ctx)
}

func (s *lastNCacheStub) ListLastNAccountIDs(_ context.Context) []int64 {
	if s.ids != nil {
		return append([]int64(nil), s.ids...)
	}
	out := make([]int64, 0, len(s.byID))
	for id := range s.byID {
		out = append(out, id)
	}
	return out
}

func lastNFromStats(success, errors, ttft int64, p50 int, rate float64) *AccountQualityLastN {
	ok := make([]bool, 0, success+errors)
	for i := int64(0); i < success; i++ {
		ok = append(ok, true)
	}
	for i := int64(0); i < errors; i++ {
		ok = append(ok, false)
	}
	ttftMs := make([]int, 0, ttft)
	for i := int64(0); i < ttft; i++ {
		ttftMs = append(ttftMs, p50)
	}
	live := &AccountQualityLastN{N: DefaultAccountQualityWindowN, OK: ok, TTFTMs: ttftMs}
	RecomputeAccountQualityLastN(live)
	if ttft == 0 {
		live.P50TTFTMs = nil
	}
	rateCopy := rate
	live.SuccessRate = &rateCopy
	return live
}

func TestAccountQualityMaintenance_SkipEmptySamples(t *testing.T) {
	repo := &qualitySnapshotRepoStub{}
	svc := NewAccountQualityMaintenanceService(repo, nil, nil)
	svc.lastN = &lastNCacheStub{
		ids: []int64{1, 2, 3},
		byID: map[int64]*AccountQualityLastN{
			1: lastNFromStats(2, 0, 2, 120, 1),
			3: lastNFromStats(0, 1, 0, 0, 0),
		},
	}

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
	repo := &qualitySnapshotRepoStub{}
	lastN := &lastNCacheStub{
		ids:  []int64{9},
		byID: map[int64]*AccountQualityLastN{9: lastNFromStats(4, 1, 4, 100, 0.8)},
	}
	svc := NewAccountQualityMaintenanceService(repo, nil, nil)
	svc.lastN = lastN
	now := time.Date(2026, 8, 14, 12, 10, 1, 0, time.UTC)

	require.NoError(t, svc.RunTick(context.Background(), now))
	lastN.byID[9] = lastNFromStats(6, 1, 4, 100, 6.0/7.0)
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
	repo := &qualitySnapshotRepoStub{}
	lastN := &lastNCacheStub{ids: ids, byID: map[int64]*AccountQualityLastN{}}
	svc := NewAccountQualityMaintenanceService(repo, nil, nil)
	svc.lastN = lastN

	require.NoError(t, svc.RunTick(context.Background(), time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)))
	require.Len(t, lastN.batchCalls, 2)
	require.Len(t, lastN.batchCalls[0], AccountQualityMaxBatchSize)
	require.Len(t, lastN.batchCalls[1], 3)
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
	svc := NewAccountQualityMaintenanceService(repo, nil, nil)
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
	repo := &qualitySnapshotRepoStub{}
	hook := &hardCloseHookStub{}
	svc := NewAccountQualityMaintenanceService(repo, nil, nil)
	svc.lastN = &lastNCacheStub{
		ids:  []int64{1},
		byID: map[int64]*AccountQualityLastN{1: lastNFromStats(3, 0, 3, 100, 1)},
	}
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

func TestAccountQualityMaintenance_TickDoesNotReplaceLiveCache(t *testing.T) {
	repo := &qualitySnapshotRepoStub{}
	live := &liveQualityCacheCapture{}
	svc := NewAccountQualityMaintenanceService(repo, nil, nil)
	svc.lastN = &lastNCacheStub{
		ids:  []int64{1},
		byID: map[int64]*AccountQualityLastN{1: lastNFromStats(2, 0, 2, 120, 1)},
	}
	svc.SetLiveQualityCache(live)

	require.NoError(t, svc.RunTick(context.Background(), time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)))
	require.Equal(t, 0, live.calls)
	require.Contains(t, repo.rows, snapshotKey{accountID: 1, capturedAt: time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)})
}

func TestAccountQualityMaintenance_GetLastNStatsBatch_MissingStampsN(t *testing.T) {
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.lastN = &lastNCacheStub{
		byID: map[int64]*AccountQualityLastN{1: lastNFromStats(2, 0, 2, 40, 1)},
	}
	out, err := svc.GetLastNStatsBatch(context.Background(), []int64{1, 2})
	require.NoError(t, err)
	require.Equal(t, int64(2), out[1].SuccessCount)
	require.Equal(t, DefaultAccountQualityWindowN, out[1].AccountQualityWindowN)
	require.Equal(t, int64(0), out[2].SuccessCount)
	require.Equal(t, DefaultAccountQualityWindowN, out[2].AccountQualityWindowN)
}

func TestAccountQualityMaintenance_ObserveAccountCompletion_AllUsersShareWindow(t *testing.T) {
	hook := &hardCloseHookStub{}
	lastN := &lastNCacheStub{}
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.lastN = lastN
	svc.SetHardCloseEvaluator(hook)

	ttft := 40
	svc.ObserveAccountCompletion(context.Background(), AccountQualityObservation{AccountID: 7, Success: true, FirstTokenMs: &ttft})
	svc.ObserveAccountCompletion(context.Background(), AccountQualityObservation{AccountID: 7, Success: false})
	require.Equal(t, 2, lastN.byID[7].OKCount)
	require.Equal(t, 1, lastN.byID[7].TTFTCount)
	require.True(t, hook.called)
	require.Equal(t, int64(1), hook.stats[7].SuccessCount)
	require.Equal(t, int64(1), hook.stats[7].ErrorCount)
}

type userSnapshotRepoStub struct {
	rows    map[userSnapshotKey]UserQualitySnapshotRow
	upserts []UserQualitySnapshotRow
	listID  int64
}

type userSnapshotKey struct {
	userID     int64
	capturedAt time.Time
}

func (s *userSnapshotRepoStub) Upsert(_ context.Context, row UserQualitySnapshotRow) error {
	if s.rows == nil {
		s.rows = map[userSnapshotKey]UserQualitySnapshotRow{}
	}
	s.rows[userSnapshotKey{userID: row.UserID, capturedAt: row.CapturedAt.UTC()}] = row
	s.upserts = append(s.upserts, row)
	return nil
}

func (s *userSnapshotRepoStub) ListByUser(_ context.Context, userID int64, from, to time.Time) ([]UserQualitySnapshotRow, error) {
	s.listID = userID
	out := make([]UserQualitySnapshotRow, 0, len(s.rows))
	for _, row := range s.rows {
		if row.UserID == userID && !row.CapturedAt.Before(from) && !row.CapturedAt.After(to) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *userSnapshotRepoStub) DeleteExpired(_ context.Context, _ time.Time, _ int) (int64, error) {
	return 0, nil
}

func TestAccountQualityMaintenance_ObserveUserCompletion_IsolatesUsers(t *testing.T) {
	userLastN := &lastNCacheStub{}
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.SetUserLastNCache(userLastN)

	ttft := 40
	svc.ObserveAccountCompletion(context.Background(), AccountQualityObservation{AccountID: 7, UserID: 16, Success: true, FirstTokenMs: &ttft})
	svc.ObserveAccountCompletion(context.Background(), AccountQualityObservation{AccountID: 8, UserID: 16, Success: false})
	svc.ObserveAccountCompletion(context.Background(), AccountQualityObservation{AccountID: 7, UserID: 17, Success: true, FirstTokenMs: &ttft})

	require.Equal(t, 2, userLastN.byID[16].OKCount)
	require.Equal(t, 1, userLastN.byID[16].TTFTCount)
	require.Equal(t, 1, userLastN.byID[17].OKCount)
	require.Equal(t, 1, userLastN.byID[17].TTFTCount)
	require.Nil(t, userLastN.byID[7])
}

func TestAccountQualityMaintenance_GetUserLastNStatsBatch_StampsGlobalNAndFailover(t *testing.T) {
	userLastN := &lastNCacheStub{}
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.SetUserLastNCache(userLastN)

	live := userLastN.IngestUserLastN(context.Background(), 16, DefaultAccountQualityWindowN, false, nil, nil, true, nil)
	require.True(t, live.UseFailover)
	stats := live.ToAccountQualityStats()
	require.Equal(t, int64(1), stats.ErrorCount)
	require.Equal(t, int64(1), stats.FailoverErrorCount)
	require.NotEqual(t, int64(0), stats.FailoverErrorCount)

	out, err := svc.GetUserLastNStatsBatch(context.Background(), []int64{16, 17})
	require.NoError(t, err)
	require.Equal(t, DefaultAccountQualityWindowN, out[16].AccountQualityWindowN)
	require.Equal(t, DefaultAccountQualityWindowN, out[16].N)
	require.Equal(t, int64(1), out[16].ErrorCount)
	require.Equal(t, int64(1), out[16].FailoverErrorCount)
	require.Equal(t, int64(0), out[17].SuccessCount)
	require.Equal(t, DefaultAccountQualityWindowN, out[17].AccountQualityWindowN)
}

func TestAccountQualityMaintenance_ListUserHistory_Empty(t *testing.T) {
	repo := &userSnapshotRepoStub{}
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.SetUserSnapshotRepo(repo)

	items, err := svc.ListUserHistory(context.Background(), 16, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, int64(16), repo.listID)
}

func TestAccountQualityMaintenance_RunTick_SnapshotsUserLastN(t *testing.T) {
	userRepo := &userSnapshotRepoStub{}
	userLastN := &lastNCacheStub{
		ids:  []int64{16},
		byID: map[int64]*AccountQualityLastN{16: lastNFromStats(2, 1, 2, 80, 2.0/3.0)},
	}
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.SetUserLastNCache(userLastN)
	svc.SetUserSnapshotRepo(userRepo)

	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	require.NoError(t, svc.RunTick(context.Background(), now))
	require.Contains(t, userRepo.rows, userSnapshotKey{userID: 16, capturedAt: now})
	require.Equal(t, int64(2), userRepo.rows[userSnapshotKey{userID: 16, capturedAt: now}].SuccessCount)
}

type userWindowNLookupStub struct {
	byID map[int64]*int
}

func (s userWindowNLookupStub) GetQualityWindowNBatch(_ context.Context, userIDs []int64) map[int64]*int {
	out := map[int64]*int{}
	for _, id := range userIDs {
		if v, ok := s.byID[id]; ok {
			out[id] = CopyIntPtr(v)
		}
	}
	return out
}

func TestAccountQualityMaintenance_UserWindowN_OverrideVsInherit(t *testing.T) {
	userLastN := &lastNCacheStub{}
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.SetUserLastNCache(userLastN)
	ten := 10
	svc.SetUserWindowNLookup(userWindowNLookupStub{byID: map[int64]*int{16: &ten}})

	ttft := 40
	svc.ObserveAccountCompletion(context.Background(), AccountQualityObservation{UserID: 16, Success: true, FirstTokenMs: &ttft})
	svc.ObserveAccountCompletion(context.Background(), AccountQualityObservation{UserID: 17, Success: true, FirstTokenMs: &ttft})

	require.Equal(t, 10, userLastN.byID[16].N)
	require.NotNil(t, userLastN.byID[16].OverrideN)
	require.Equal(t, 10, *userLastN.byID[16].OverrideN)
	require.Equal(t, DefaultAccountQualityWindowN, userLastN.byID[17].N)
	require.Nil(t, userLastN.byID[17].OverrideN)

	out, err := svc.GetUserLastNStatsBatch(context.Background(), []int64{16, 17})
	require.NoError(t, err)
	require.Equal(t, 10, out[16].WindowN)
	require.Equal(t, DefaultAccountQualityWindowN, out[17].WindowN)
}

func TestAccountQualityMaintenance_ApplyUserQualityWindowN_ResizesLive(t *testing.T) {
	userLastN := &lastNCacheStub{}
	svc := NewAccountQualityMaintenanceService(&qualitySnapshotRepoStub{}, nil, nil)
	svc.SetUserLastNCache(userLastN)

	for i := 0; i < 6; i++ {
		ttft := 100 + i
		userLastN.IngestUserLastN(context.Background(), 16, 20, true, &ttft, nil, false, nil)
	}
	require.Equal(t, 6, userLastN.byID[16].OKCount)

	eight := 8
	svc.ApplyUserQualityWindowN(context.Background(), 16, &eight)
	require.Equal(t, 8, userLastN.byID[16].N)
	require.Equal(t, 6, userLastN.byID[16].OKCount)
	require.NotNil(t, userLastN.byID[16].OverrideN)
	require.Equal(t, 8, *userLastN.byID[16].OverrideN)

	svc.ApplyUserQualityWindowN(context.Background(), 16, nil)
	require.Equal(t, DefaultAccountQualityWindowN, userLastN.byID[16].N)
	require.Nil(t, userLastN.byID[16].OverrideN)
}
