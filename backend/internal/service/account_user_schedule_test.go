//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestAccount_AllowsScheduleUser(t *testing.T) {
	t.Parallel()

	t.Run("empty three configs allow everyone", func(t *testing.T) {
		t.Parallel()
		require.True(t, (&Account{}).AllowsScheduleUser(16))
		require.True(t, (&Account{UserScheduleMode: UserScheduleModeUnrestricted}).AllowsScheduleUser(0))
		require.True(t, (&Account{UserScheduleMode: ""}).AllowsScheduleUser(99))
	})

	t.Run("allow hit and miss", func(t *testing.T) {
		t.Parallel()
		acc := &Account{AllowUserIDs: []int64{16, 42}}
		require.True(t, acc.AllowsScheduleUser(16))
		require.False(t, acc.AllowsScheduleUser(7))
	})

	t.Run("deny hit and miss", func(t *testing.T) {
		t.Parallel()
		acc := &Account{DenyUserIDs: []int64{16}}
		require.False(t, acc.AllowsScheduleUser(16))
		require.True(t, acc.AllowsScheduleUser(7))
	})

	t.Run("userID zero fail closed when any rule exists", func(t *testing.T) {
		t.Parallel()
		require.False(t, (&Account{AllowUserIDs: []int64{16}}).AllowsScheduleUser(0))
		require.False(t, (&Account{DenyUserIDs: []int64{16}}).AllowsScheduleUser(0))
		require.False(t, (&Account{UserConcurrency: map[int64]int{16: 5}}).AllowsScheduleUser(0))
		p50 := 1500
		require.False(t, (&Account{UserQualityGates: map[int64]QualityHardCloseSettings{16: {MaxP50TTFTMs: &p50}}}).AllowsScheduleUser(0))
	})

	t.Run("empty allow is not a whitelist", func(t *testing.T) {
		t.Parallel()
		require.True(t, (&Account{DenyUserIDs: nil, AllowUserIDs: nil}).AllowsScheduleUser(16))
	})

	t.Run("deny wins over allow and cap", func(t *testing.T) {
		t.Parallel()
		acc := &Account{
			AllowUserIDs:    []int64{16},
			DenyUserIDs:     []int64{16},
			UserConcurrency: map[int64]int{16: 5},
		}
		require.False(t, acc.AllowsScheduleUser(16))
		require.Equal(t, 5, acc.PairMaxConcurrency(16))
	})

	t.Run("allow plus cap is admitted", func(t *testing.T) {
		t.Parallel()
		acc := &Account{
			AllowUserIDs:    []int64{16},
			UserConcurrency: map[int64]int{16: 5},
		}
		require.True(t, acc.AllowsScheduleUser(16))
		require.Equal(t, 5, acc.PairMaxConcurrency(16))
	})

	t.Run("allow list nonempty excludes outsiders even with a cap", func(t *testing.T) {
		t.Parallel()
		acc := &Account{
			AllowUserIDs:    []int64{16},
			UserConcurrency: map[int64]int{7: 3},
		}
		require.False(t, acc.AllowsScheduleUser(7))
		require.Equal(t, 3, acc.PairMaxConcurrency(7))
	})
}

func TestAccount_PairMaxConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("unset zero and negative are no pair cap", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, 0, (&Account{}).PairMaxConcurrency(16))
		require.Equal(t, 0, (&Account{UserConcurrency: map[int64]int{16: 0}}).PairMaxConcurrency(16))
		require.Equal(t, 0, (&Account{UserConcurrency: map[int64]int{16: -2}}).PairMaxConcurrency(16))
		require.Equal(t, 0, (&Account{UserConcurrency: map[int64]int{16: 4}}).PairMaxConcurrency(0))
	})

	t.Run("explicit N is returned", func(t *testing.T) {
		t.Parallel()
		acc := &Account{UserConcurrency: map[int64]int{16: 1, 42: 8}}
		require.Equal(t, 1, acc.PairMaxConcurrency(16))
		require.Equal(t, 8, acc.PairMaxConcurrency(42))
		require.Equal(t, 0, acc.PairMaxConcurrency(7))
	})
}

func TestUserScheduleIDFromContext(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(9), scheduleUserIDFromContext(context.Background(), 9))
	require.Equal(t, int64(0), scheduleUserIDFromContext(context.Background(), 0))

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	require.Equal(t, int64(16), scheduleUserIDFromContext(ctx, 0))
	require.Equal(t, int64(42), scheduleUserIDFromContext(ctx, 42))
}

func qualityGateAccount(userID int64, gate QualityHardCloseSettings) *Account {
	return &Account{
		ID:               1,
		UserQualityGates: map[int64]QualityHardCloseSettings{userID: fillUserQualityGateDefaults(gate)},
	}
}

func liveQualityStats(p50 int, ttftSamples, success, errors int64, rate float64) *AccountQualityStats {
	return &AccountQualityStats{
		P50TTFTMs:    &p50,
		TTFTSamples:  ttftSamples,
		SuccessCount: success,
		ErrorCount:   errors,
		SuccessRate:  &rate,
	}
}

type liveQualityCacheStub struct {
	byID map[int64]*AccountQualityStats
	err  error
}

func (s *liveQualityCacheStub) Get(_ context.Context, accountID int64) (*AccountQualityStats, error) {
	if s == nil {
		return nil, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.byID == nil {
		return nil, nil
	}
	return s.byID[accountID], nil
}

func (s *liveQualityCacheStub) Replace(_ context.Context, stats map[int64]*AccountQualityStats) error {
	if s == nil {
		return nil
	}
	s.byID = stats
	return nil
}

func (s *liveQualityCacheStub) MarkUserResume(_ context.Context, accountID, userID int64) error {
	if s == nil {
		return nil
	}
	if s.byID == nil {
		s.byID = map[int64]*AccountQualityStats{}
	}
	st := s.byID[accountID]
	if st == nil {
		st = &AccountQualityStats{}
		s.byID[accountID] = st
	}
	ApplyUserQualityResume(st, userID, time.Now().UTC())
	return nil
}

func (s *liveQualityCacheStub) MarkUserQualityWindow(_ context.Context, accountID, userID int64) error {
	if s == nil {
		return nil
	}
	if s.byID == nil {
		s.byID = map[int64]*AccountQualityStats{}
	}
	st := s.byID[accountID]
	if st == nil {
		st = &AccountQualityStats{}
		s.byID[accountID] = st
	}
	ApplyUserQualityWindowStart(st, userID, time.Now().UTC())
	return nil
}

func (s *liveQualityCacheStub) MarkAccountResume(_ context.Context, accountID int64) error {
	if s == nil {
		return nil
	}
	if s.byID == nil {
		s.byID = map[int64]*AccountQualityStats{}
	}
	st := s.byID[accountID]
	if st == nil {
		st = &AccountQualityStats{}
		s.byID[accountID] = st
	}
	SetAccountQualityResume(st, time.Now().UTC().Add(AccountQualityWindow))
	return nil
}

func TestAccount_QualityGateBlocksUserAndAdmitsScheduleUser(t *testing.T) {
	t.Parallel()

	p50 := 1000
	rate := 0.9
	p50Gate := QualityHardCloseSettings{MaxP50TTFTMs: &p50, MinTTFTSamples: 10, Condition: QualityHardCloseConditionOr}
	bothGate := QualityHardCloseSettings{
		MaxP50TTFTMs:      &p50,
		MinSuccessRate:    &rate,
		MinSuccessSamples: 20,
		MinTTFTSamples:    10,
	}

	t.Run("breach blocks only that user", func(t *testing.T) {
		t.Parallel()
		acc := qualityGateAccount(16, p50Gate)
		breached := liveQualityStats(2000, 10, 20, 0, 1)
		require.True(t, acc.QualityGateBlocksUser(16, breached))
		require.False(t, acc.AdmitsScheduleUser(16, breached))
		require.False(t, acc.QualityGateBlocksUser(7, breached))
		require.True(t, acc.AdmitsScheduleUser(7, breached))
		require.Nil(t, acc.TempUnschedulableUntil)
	})

	t.Run("manual resume admits despite a still-breaching window", func(t *testing.T) {
		t.Parallel()
		acc := qualityGateAccount(16, p50Gate)
		breached := liveQualityStats(2000, 10, 20, 0, 1)
		SetUserQualityResume(breached, 16, time.Now().UTC().Add(AccountQualityWindow))
		require.False(t, acc.QualityGateBlocksUser(16, breached))
		require.True(t, acc.AdmitsScheduleUser(16, breached))
	})

	t.Run("under-sampled metric is not judged", func(t *testing.T) {
		t.Parallel()
		acc := qualityGateAccount(16, p50Gate)
		under := liveQualityStats(5000, 9, 20, 0, 1)
		require.False(t, acc.QualityGateBlocksUser(16, under))
		require.True(t, acc.AdmitsScheduleUser(16, under))
	})

	t.Run("nil stats and cache miss fail open", func(t *testing.T) {
		t.Parallel()
		acc := qualityGateAccount(16, p50Gate)
		require.False(t, acc.QualityGateBlocksUser(16, nil))
		require.True(t, acc.AdmitsScheduleUser(16, nil))
		ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
		require.True(t, admitsScheduleUser(ctx, acc, nil, nil))
		require.True(t, admitsScheduleUser(ctx, acc, &liveQualityCacheStub{}, nil))
		require.True(t, admitsScheduleUser(ctx, acc, &liveQualityCacheStub{err: context.DeadlineExceeded}, nil))
	})

	t.Run("no gate never blocks", func(t *testing.T) {
		t.Parallel()
		acc := &Account{ID: 1}
		breached := liveQualityStats(9000, 20, 1, 20, 0.05)
		require.False(t, acc.QualityGateBlocksUser(16, breached))
		require.True(t, acc.AdmitsScheduleUser(16, breached))
	})

	t.Run("unconfigured metric is ignored", func(t *testing.T) {
		t.Parallel()
		acc := qualityGateAccount(16, p50Gate)
		goodP50BadSuccess := liveQualityStats(400, 12, 1, 20, 0.04)
		require.False(t, acc.QualityGateBlocksUser(16, goodP50BadSuccess))
		require.True(t, acc.AdmitsScheduleUser(16, goodP50BadSuccess))
	})

	t.Run("zero samples use defaults 20/10", func(t *testing.T) {
		t.Parallel()
		acc := &Account{
			ID: 1,
			UserQualityGates: map[int64]QualityHardCloseSettings{
				16: {MaxP50TTFTMs: &p50, MinSuccessSamples: 0, MinTTFTSamples: 0},
			},
		}
		underDefault := liveQualityStats(5000, 9, 19, 0, 1)
		require.False(t, acc.QualityGateBlocksUser(16, underDefault))
		require.True(t, acc.AdmitsScheduleUser(16, underDefault))
		atDefault := liveQualityStats(5000, 10, 20, 0, 1)
		require.True(t, acc.QualityGateBlocksUser(16, atDefault))
		require.False(t, acc.AdmitsScheduleUser(16, atDefault))
	})

	t.Run("condition-only is not a gate", func(t *testing.T) {
		t.Parallel()
		cond := QualityHardCloseConditionOr
		copied := copyUserQualityGates(map[int64]QualityHardCloseSettings{
			16: {Condition: QualityHardCloseConditionOr},
		})
		require.Empty(t, copied)
		gate, ok := userQualityGateFromFields(nil, nil, nil, nil, &cond)
		require.False(t, ok)
		require.Equal(t, QualityHardCloseSettings{}, gate)
	})

	t.Run("or vs and", func(t *testing.T) {
		t.Parallel()
		p50OnlyBad := liveQualityStats(2000, 10, 20, 0, 0.95)
		bothBad := liveQualityStats(2000, 10, 10, 10, 0.5)
		orAcc := qualityGateAccount(16, bothGate)
		orAcc.UserQualityGates[16] = fillUserQualityGateDefaults(QualityHardCloseSettings{
			MaxP50TTFTMs:      &p50,
			MinSuccessRate:    &rate,
			MinSuccessSamples: 20,
			MinTTFTSamples:    10,
			Condition:         QualityHardCloseConditionOr,
		})
		andAcc := qualityGateAccount(16, bothGate)
		andAcc.UserQualityGates[16] = fillUserQualityGateDefaults(QualityHardCloseSettings{
			MaxP50TTFTMs:      &p50,
			MinSuccessRate:    &rate,
			MinSuccessSamples: 20,
			MinTTFTSamples:    10,
			Condition:         QualityHardCloseConditionAnd,
		})
		require.True(t, orAcc.QualityGateBlocksUser(16, p50OnlyBad))
		require.False(t, andAcc.QualityGateBlocksUser(16, p50OnlyBad))
		require.True(t, orAcc.QualityGateBlocksUser(16, bothBad))
		require.True(t, andAcc.QualityGateBlocksUser(16, bothBad))
	})

	t.Run("quality-gate-only userID zero fail closed", func(t *testing.T) {
		t.Parallel()
		acc := qualityGateAccount(16, p50Gate)
		ok := liveQualityStats(100, 20, 20, 0, 1)
		require.False(t, acc.AllowsScheduleUser(0))
		require.False(t, acc.AdmitsScheduleUser(0, ok))
		require.False(t, acc.QualityGateBlocksUser(0, liveQualityStats(9000, 20, 1, 20, 0.01)))
		ctx := context.Background()
		require.False(t, admitsScheduleUser(ctx, acc, &liveQualityCacheStub{
			byID: map[int64]*AccountQualityStats{1: ok},
		}, nil))
	})

	t.Run("live cache breach blocks admission", func(t *testing.T) {
		t.Parallel()
		acc := qualityGateAccount(16, p50Gate)
		ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
		require.False(t, admitsScheduleUser(ctx, acc, &liveQualityCacheStub{
			byID: map[int64]*AccountQualityStats{1: liveQualityStats(4000, 12, 20, 0, 1)},
		}, nil))
	})
}

func TestStampScheduleUsersQualityRuntime(t *testing.T) {
	t.Parallel()

	p50 := 1000
	acc := qualityGateAccount(16, QualityHardCloseSettings{MaxP50TTFTMs: &p50, MinTTFTSamples: 10})
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

	t.Run("breach stamps blocked", func(t *testing.T) {
		t.Parallel()
		users := []ScheduleUserRef{{ID: 16, QualityMaxP50TTFTMs: &p50}}
		stampScheduleUsersQualityRuntime(acc, users, liveQualityStats(2000, 10, 20, 0, 1), now)
		require.True(t, users[0].QualityBlocked)
		require.Nil(t, users[0].QualityResumedUntil)
	})

	t.Run("resume stamps until and not blocked", func(t *testing.T) {
		t.Parallel()
		users := []ScheduleUserRef{{ID: 16, QualityMaxP50TTFTMs: &p50}}
		stats := liveQualityStats(2000, 10, 20, 0, 1)
		ApplyUserQualityResume(stats, 16, now)
		stampScheduleUsersQualityRuntime(acc, users, stats, now)
		require.False(t, users[0].QualityBlocked)
		require.NotNil(t, users[0].QualityResumedUntil)
		require.Equal(t, now.Add(AccountQualityWindow).Unix(), *users[0].QualityResumedUntil)
		require.NotNil(t, users[0].QualityWindowUntil)
		require.Equal(t, now.Add(2*AccountQualityWindow).Unix(), *users[0].QualityWindowUntil)
	})

	t.Run("after 15m auto drops 已恢复 and keeps accumulating", func(t *testing.T) {
		t.Parallel()
		users := []ScheduleUserRef{{ID: 16, QualityMaxP50TTFTMs: &p50}}
		stats := liveQualityStats(2000, 10, 20, 0, 1)
		ApplyUserQualityResume(stats, 16, now)
		later := now.Add(AccountQualityWindow + time.Minute)
		stampScheduleUsersQualityRuntime(acc, users, stats, later)
		require.False(t, users[0].QualityBlocked)
		require.Nil(t, users[0].QualityResumedUntil)
		require.NotNil(t, users[0].QualityWindowUntil)
	})

	t.Run("click 已恢复 starts window and drops resumed chip", func(t *testing.T) {
		t.Parallel()
		users := []ScheduleUserRef{{ID: 16, QualityMaxP50TTFTMs: &p50}}
		stats := liveQualityStats(2000, 10, 20, 0, 1)
		ApplyUserQualityResume(stats, 16, now)
		clickAt := now.Add(2 * time.Minute)
		ApplyUserQualityWindowStart(stats, 16, clickAt)
		stampScheduleUsersQualityRuntime(acc, users, stats, clickAt)
		require.False(t, users[0].QualityBlocked)
		require.Nil(t, users[0].QualityResumedUntil)
		require.NotNil(t, users[0].QualityWindowUntil)
		require.Equal(t, clickAt.Add(AccountQualityWindow).Unix(), *users[0].QualityWindowUntil)
	})

	t.Run("nil stats fail open", func(t *testing.T) {
		t.Parallel()
		users := []ScheduleUserRef{{ID: 16, QualityMaxP50TTFTMs: &p50}}
		stampScheduleUsersQualityRuntime(acc, users, nil, now)
		require.False(t, users[0].QualityBlocked)
		require.Nil(t, users[0].QualityResumedUntil)
	})

	t.Run("no gate is not stamped", func(t *testing.T) {
		t.Parallel()
		users := []ScheduleUserRef{{ID: 16}}
		stampScheduleUsersQualityRuntime(acc, users, liveQualityStats(2000, 10, 20, 0, 1), now)
		require.False(t, users[0].QualityBlocked)
		require.Nil(t, users[0].QualityResumedUntil)
	})
}
