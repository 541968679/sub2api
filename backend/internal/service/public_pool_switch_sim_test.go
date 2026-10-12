//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// Public-pool switch simulation. It drives the real load-aware selector and
// prints one trace of: pick the lighter account, keep a healthy sticky pin,
// then leave that pin once it is demoted and a peer is clearly healthier.

func switchSimAccount(id int64, priority int, rate float64) Account {
	return Account{
		ID:                     id,
		Platform:               PlatformAnthropic,
		Type:                   AccountTypeAPIKey,
		Status:                 StatusActive,
		Schedulable:            true,
		Concurrency:            4,
		Priority:               priority,
		UpstreamRateMultiplier: testRate(rate),
	}
}

func coolPublicAccounts(t *testing.T, ids ...int64) *PublicScheduleQualityService {
	t.Helper()
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	enablePublicSite(runtime)
	until := time.Now().Add(15 * time.Minute)
	for _, id := range ids {
		require.True(t, cache.TryStartCooldown(context.Background(), id, until, "fail", false))
	}
	return runtime
}

func newPublicPoolSwitchHarness(
	t *testing.T,
	groupID int64,
	accounts []Account,
	loads map[int64]*AccountLoadInfo,
	session string,
	stickyID int64,
	runtime *PublicScheduleQualityService,
) (*GatewayService, *mockGatewayCacheForPlatform, *mockConcurrencyCache) {
	t.Helper()
	repo := &mockAccountRepoForPlatform{
		accounts:     append([]Account(nil), accounts...),
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	bindings := map[string]int64{}
	if session != "" && stickyID > 0 {
		bindings[session] = stickyID
	}
	cache := &mockGatewayCacheForPlatform{sessionBindings: bindings}
	conc := &mockConcurrencyCache{loadMap: loads}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
			groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
		}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(conc),
		publicSchedule:     runtime,
	}
	return svc, cache, conc
}

func pickPublicPool(t *testing.T, svc *GatewayService, groupID int64, session string) *AccountSelectionResult {
	t.Helper()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, session, "", nil, "", int64(16))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.True(t, result.Acquired)
	require.Nil(t, result.WaitPlan)
	return result
}

func TestPublicPoolSwitchSim(t *testing.T) {
	const (
		cheapID     = int64(1)
		expensiveID = int64(2)
		groupID     = int64(910)
	)
	accounts := []Account{
		switchSimAccount(cheapID, 1, 0.15),
		switchSimAccount(expensiveID, 1, 1.0),
	}

	t.Run("same priority prefers lower load over lower rate", func(t *testing.T) {
		loads := map[int64]*AccountLoadInfo{
			cheapID:     {AccountID: cheapID, LoadRate: 90},
			expensiveID: {AccountID: expensiveID, LoadRate: 10},
		}
		svc, _, _ := newPublicPoolSwitchHarness(t, groupID, accounts, loads, "", 0, nil)
		got := pickPublicPool(t, svc, groupID, "")
		require.Equal(t, expensiveID, got.Account.ID)
	})

	t.Run("lower load still wins when it is the cheaper account", func(t *testing.T) {
		loads := map[int64]*AccountLoadInfo{
			cheapID:     {AccountID: cheapID, LoadRate: 10},
			expensiveID: {AccountID: expensiveID, LoadRate: 90},
		}
		svc, _, _ := newPublicPoolSwitchHarness(t, groupID+1, accounts, loads, "", 0, nil)
		got := pickPublicPool(t, svc, groupID+1, "")
		require.Equal(t, cheapID, got.Account.ID)
	})

	t.Run("demoted sticky switches to a clearly lighter peer", func(t *testing.T) {
		runtime := coolPublicAccounts(t, cheapID, expensiveID)
		loads := map[int64]*AccountLoadInfo{
			cheapID:     {AccountID: cheapID, LoadRate: 80},
			expensiveID: {AccountID: expensiveID, LoadRate: 40},
		}
		svc, cache, _ := newPublicPoolSwitchHarness(t, groupID+2, accounts, loads, "lag", cheapID, runtime)
		got := pickPublicPool(t, svc, groupID+2, "lag")
		require.Equal(t, expensiveID, got.Account.ID)
		require.Greater(t, cache.deletedSessions["lag"], 0)
	})

	t.Run("small load gap keeps a demoted sticky pin", func(t *testing.T) {
		runtime := coolPublicAccounts(t, cheapID, expensiveID)
		loads := map[int64]*AccountLoadInfo{
			cheapID:     {AccountID: cheapID, LoadRate: 50},
			expensiveID: {AccountID: expensiveID, LoadRate: 40},
		}
		svc, cache, _ := newPublicPoolSwitchHarness(t, groupID+3, accounts, loads, "close", cheapID, runtime)
		got := pickPublicPool(t, svc, groupID+3, "close")
		require.Equal(t, cheapID, got.Account.ID)
		require.Zero(t, cache.deletedSessions["close"])
	})

	t.Run("healthy sticky stays even when a peer is much lighter", func(t *testing.T) {
		loads := map[int64]*AccountLoadInfo{
			cheapID:     {AccountID: cheapID, LoadRate: 90},
			expensiveID: {AccountID: expensiveID, LoadRate: 10},
		}
		svc, cache, _ := newPublicPoolSwitchHarness(t, groupID+4, accounts, loads, "healthy", cheapID, nil)
		got := pickPublicPool(t, svc, groupID+4, "healthy")
		require.Equal(t, cheapID, got.Account.ID)
		require.Zero(t, cache.deletedSessions["healthy"])
	})

	t.Run("a cooling account loses to a healthy peer", func(t *testing.T) {
		runtime := coolPublicAccounts(t, cheapID)
		loads := map[int64]*AccountLoadInfo{
			cheapID:     {AccountID: cheapID, LoadRate: 0},
			expensiveID: {AccountID: expensiveID, LoadRate: 50},
		}
		svc, _, _ := newPublicPoolSwitchHarness(t, groupID+5, accounts, loads, "", 0, runtime)
		got := pickPublicPool(t, svc, groupID+5, "")
		require.Equal(t, expensiveID, got.Account.ID)
	})

	t.Run("all cooling still prefers lower load", func(t *testing.T) {
		runtime := coolPublicAccounts(t, cheapID, expensiveID)
		loads := map[int64]*AccountLoadInfo{
			cheapID:     {AccountID: cheapID, LoadRate: 80},
			expensiveID: {AccountID: expensiveID, LoadRate: 20},
		}
		svc, _, _ := newPublicPoolSwitchHarness(t, groupID+6, accounts, loads, "", 0, runtime)
		got := pickPublicPool(t, svc, groupID+6, "")
		require.Equal(t, expensiveID, got.Account.ID)
	})

	t.Run("demoted sticky switches on a clear ttft gap", func(t *testing.T) {
		sticky := switchSimAccount(cheapID, 5, 0.15)
		peer := switchSimAccount(expensiveID, 1, 1.0)
		runtime := coolPublicAccounts(t, cheapID, expensiveID)
		stickyMS, peerMS := 8000, 2000
		runtime.storeWindow(context.Background(), cheapID, &PairQualityLive{P50TTFTMs: &stickyMS})
		runtime.storeWindow(context.Background(), expensiveID, &PairQualityLive{P50TTFTMs: &peerMS})
		loads := map[int64]*AccountLoadInfo{
			cheapID:     {AccountID: cheapID, LoadRate: 30},
			expensiveID: {AccountID: expensiveID, LoadRate: 30},
		}
		svc, cache, _ := newPublicPoolSwitchHarness(t, groupID+7, []Account{sticky, peer}, loads, "ttft", cheapID, runtime)
		got := pickPublicPool(t, svc, groupID+7, "ttft")
		require.Equal(t, expensiveID, got.Account.ID)
		require.Greater(t, cache.deletedSessions["ttft"], 0)
	})

	t.Run("openai score beats a cheap lagging account", func(t *testing.T) {
		cheap := 0.15
		expensive := 1.0
		left := openAIAccountCandidateScore{
			account:  &Account{ID: cheapID, Priority: 1, UpstreamRateMultiplier: &cheap},
			loadInfo: &AccountLoadInfo{LoadRate: 90},
			score:    1,
		}
		right := openAIAccountCandidateScore{
			account:  &Account{ID: expensiveID, Priority: 50, UpstreamRateMultiplier: &expensive},
			loadInfo: &AccountLoadInfo{LoadRate: 10},
			score:    99,
		}
		require.True(t, isOpenAIAccountCandidateBetter(right, left))
		require.False(t, isOpenAIAccountCandidateBetter(left, right))
	})

	t.Run("trace keeps a healthy pin then switches after demotion", func(t *testing.T) {
		loadCheap := &AccountLoadInfo{AccountID: cheapID, LoadRate: 10}
		loadExpensive := &AccountLoadInfo{AccountID: expensiveID, LoadRate: 90}
		loads := map[int64]*AccountLoadInfo{cheapID: loadCheap, expensiveID: loadExpensive}
		svc, cache, _ := newPublicPoolSwitchHarness(t, groupID+8, accounts, loads, "trace", 0, nil)

		first := pickPublicPool(t, svc, groupID+8, "trace")
		require.Equal(t, cheapID, first.Account.ID)
		t.Logf("request 1 picked account %d (load 10, rate 0.15)", first.Account.ID)

		loadCheap.LoadRate = 80
		loadExpensive.LoadRate = 10
		second := pickPublicPool(t, svc, groupID+8, "trace")
		require.Equal(t, cheapID, second.Account.ID)
		require.Zero(t, cache.deletedSessions["trace"])
		t.Logf("request 2 stayed on account %d after its load rose to 80", second.Account.ID)

		runtime := coolPublicAccounts(t, cheapID, expensiveID)
		svc.publicSchedule = runtime
		loadExpensive.LoadRate = 40
		third := pickPublicPool(t, svc, groupID+8, "trace")
		require.Equal(t, expensiveID, third.Account.ID)
		require.Greater(t, cache.deletedSessions["trace"], 0)
		t.Logf("request 3 left demoted account %d for account %d (load 40)", cheapID, third.Account.ID)
	})
}
