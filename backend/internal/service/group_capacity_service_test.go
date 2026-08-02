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

type groupCapacityAPIKeyListerStub struct {
	byGroup map[int64][]int64
}

func (s *groupCapacityAPIKeyListerStub) ListActiveAPIKeyIDsByGroupIDs(_ context.Context, _ []int64) (map[int64][]int64, error) {
	return s.byGroup, nil
}

// Implements ConcurrencyCache + APIKeyConcurrencyCache for group capacity tests.
type groupCapacityAPIKeyConcurrencyStub struct {
	ConcurrencyCache
	apiKeyCounts map[int64]int
	requested    []int64
}

func (s *groupCapacityAPIKeyConcurrencyStub) GetAccountConcurrencyBatch(_ context.Context, _ []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}

func (s *groupCapacityAPIKeyConcurrencyStub) TrackAPIKeySlot(context.Context, int64, string) error {
	return nil
}
func (s *groupCapacityAPIKeyConcurrencyStub) ReleaseAPIKeySlot(context.Context, int64, string) error {
	return nil
}
func (s *groupCapacityAPIKeyConcurrencyStub) GetAPIKeyConcurrencyBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	s.requested = append([]int64(nil), ids...)
	return s.apiKeyCounts, nil
}

func TestGetAllGroupCapacity_UsesBatchProjectionAndKeepsGroupSemantics(t *testing.T) {
	// Shared account 1 is in groups 10 and 20. Account-level concurrency is irrelevant;
	// group used must come from API-key slots (group-scoped).
	accounts := &groupCapacityAccountRepoStub{rows: []GroupAccountCapacityRow{
		{GroupID: 10, AccountID: 1, Concurrency: 2, Extra: map[string]any{"max_sessions": 3, "session_idle_timeout_minutes": 7, "base_rpm": 11}},
		{GroupID: 20, AccountID: 1, Concurrency: 2, Extra: map[string]any{"max_sessions": 3, "session_idle_timeout_minutes": 7, "base_rpm": 11}},
		{GroupID: 20, AccountID: 2, Concurrency: 4, Extra: map[string]any{"base_rpm": 13}},
	}}
	groups := &groupCapacityGroupRepoStub{ids: []int64{5, 10, 20}}
	cc := &groupCapacityAPIKeyConcurrencyStub{apiKeyCounts: map[int64]int{
		101: 1, // group 10 key
		201: 2, // group 20 key
		202: 3, // group 20 key
	}}
	sc := &groupCapacitySessionStub{counts: map[int64]int{1: 2}}
	rc := &groupCapacityRPMStub{counts: map[int64]int{1: 5, 2: 7}}
	keyLister := &groupCapacityAPIKeyListerStub{byGroup: map[int64][]int64{
		10: {101},
		20: {201, 202},
	}}
	svc := NewGroupCapacityService(accounts, groups, NewConcurrencyService(cc), sc, rc, keyLister)

	got, err := svc.GetAllGroupCapacity(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, groups.calls)
	require.Equal(t, []int64{5, 10, 20}, accounts.requested)
	// Sessions only for accounts with max_sessions (account 1), not account 2
	require.ElementsMatch(t, []int64{1}, sc.requested)
	require.Equal(t, 7*time.Minute, sc.timeouts[1])
	// RPM for accounts that configure base_rpm (1 and 2)
	require.ElementsMatch(t, []int64{1, 2}, rc.requested)
	require.ElementsMatch(t, []int64{101, 201, 202}, cc.requested)
	require.Equal(t, []GroupCapacitySummary{
		{GroupID: 5},
		// group 10: concurrency used from key 101 only (=1), not shared account total
		{GroupID: 10, ConcurrencyUsed: 1, ConcurrencyMax: 2, SessionsUsed: 2, SessionsMax: 3, RPMUsed: 5, RPMMax: 11},
		// group 20: keys 201+202 = 5; sessions only acc1; rpm 5+7
		{GroupID: 20, ConcurrencyUsed: 5, ConcurrencyMax: 6, SessionsUsed: 2, SessionsMax: 3, RPMUsed: 12, RPMMax: 24},
	}, got)
}
