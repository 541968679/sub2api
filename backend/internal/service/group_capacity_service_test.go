//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type groupCapacityAccountRepoStub struct {
	AccountRepository
	rows      []GroupAccountCapacityRow
	requested []int64
}

func (s *groupCapacityAccountRepoStub) ListSchedulableCapacityByGroupIDs(_ context.Context, ids []int64) ([]GroupAccountCapacityRow, error) {
	s.requested = append([]int64(nil), ids...)
	return append([]GroupAccountCapacityRow(nil), s.rows...), nil
}

type groupCapacityGroupRepoStub struct {
	GroupRepository
	ids   []int64
	calls int
}

func (s *groupCapacityGroupRepoStub) ListActiveIDs(context.Context) ([]int64, error) {
	s.calls++
	return append([]int64(nil), s.ids...), nil
}

type groupCapacityConcurrencyStub struct {
	ConcurrencyCache
	counts    map[int64]int
	requested []int64
}

func (s *groupCapacityConcurrencyStub) GetAccountConcurrencyBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	s.requested = append([]int64(nil), ids...)
	return s.counts, nil
}

type groupCapacitySessionStub struct {
	SessionLimitCache
	counts    map[int64]int
	requested []int64
	timeouts  map[int64]time.Duration
}

func (s *groupCapacitySessionStub) GetActiveSessionCountBatch(_ context.Context, ids []int64, timeouts map[int64]time.Duration) (map[int64]int, error) {
	s.requested = append([]int64(nil), ids...)
	s.timeouts = timeouts
	return s.counts, nil
}

type groupCapacityRPMStub struct {
	RPMCache
	counts    map[int64]int
	requested []int64
}

func (s *groupCapacityRPMStub) GetRPMBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	s.requested = append([]int64(nil), ids...)
	return s.counts, nil
}

func TestGetAllGroupCapacity_UsesBatchProjectionAndKeepsGroupSemantics(t *testing.T) {
	accounts := &groupCapacityAccountRepoStub{rows: []GroupAccountCapacityRow{
		{GroupID: 10, AccountID: 1, Concurrency: 2, Extra: map[string]any{"max_sessions": 3, "session_idle_timeout_minutes": 7, "base_rpm": 11}},
		{GroupID: 20, AccountID: 1, Concurrency: 2, Extra: map[string]any{"max_sessions": 3, "session_idle_timeout_minutes": 7, "base_rpm": 11}},
		{GroupID: 20, AccountID: 2, Concurrency: 4, Extra: map[string]any{"base_rpm": 13}},
	}}
	groups := &groupCapacityGroupRepoStub{ids: []int64{5, 10, 20}}
	cc := &groupCapacityConcurrencyStub{counts: map[int64]int{1: 1, 2: 2}}
	sc := &groupCapacitySessionStub{counts: map[int64]int{1: 2}}
	rc := &groupCapacityRPMStub{counts: map[int64]int{1: 5, 2: 7}}
	svc := NewGroupCapacityService(accounts, groups, NewConcurrencyService(cc), sc, rc)

	got, err := svc.GetAllGroupCapacity(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, groups.calls)
	require.Equal(t, []int64{5, 10, 20}, accounts.requested)
	require.ElementsMatch(t, []int64{1, 2}, cc.requested)
	require.Equal(t, []int64{1}, sc.requested)
	require.Equal(t, 7*time.Minute, sc.timeouts[1])
	require.ElementsMatch(t, []int64{1, 2}, rc.requested)
	require.Equal(t, []GroupCapacitySummary{
		{GroupID: 5},
		{GroupID: 10, ConcurrencyUsed: 1, ConcurrencyMax: 2, SessionsUsed: 2, SessionsMax: 3, RPMUsed: 5, RPMMax: 11},
		{GroupID: 20, ConcurrencyUsed: 3, ConcurrencyMax: 6, SessionsUsed: 2, SessionsMax: 3, RPMUsed: 12, RPMMax: 24},
	}, got)
}
