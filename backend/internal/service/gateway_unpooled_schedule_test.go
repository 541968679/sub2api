//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func gatewayUnpooledAccounts(cheapID, expensiveID int64, cheapRate, expensiveRate float64) *mockAccountRepoForPlatform {
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{ID: cheapID, Platform: PlatformAnthropic, Priority: 9, Status: StatusActive, Schedulable: true, Concurrency: 2, UpstreamRateMultiplier: testRate(cheapRate)},
			{ID: expensiveID, Platform: PlatformAnthropic, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 2, UpstreamRateMultiplier: testRate(expensiveRate)},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	return repo
}

func TestGatewayUnpooled_BothAvailablePicksCheap(t *testing.T) {
	// AC1
	groupID := int64(81)
	repo := gatewayUnpooledAccounts(1, 2, 0.15, 1.0)
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
			groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
		}},
		cache:              &mockGatewayCacheForPlatform{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 20}, 2: {AccountID: 2, LoadRate: 0}}}),
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "gw-cheap", "", nil, "", int64(16))
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Account.ID)
	require.True(t, result.Acquired)
	require.Nil(t, result.WaitPlan)
}

func TestGatewayUnpooled_StickyEscapeToCheap(t *testing.T) {
	// AC4
	groupID := int64(82)
	sessionHash := "gw-escape"
	repo := gatewayUnpooledAccounts(1, 2, 0.15, 1.0)
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{sessionHash: 2}, sessionOverflow: map[string]bool{sessionHash: true}}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
			groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
		}},
		cache: cache,
		cfg:   cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, LoadRate: 0},
			2: {AccountID: 2, LoadRate: 10},
		}}),
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, "", nil, "", int64(16))
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Account.ID)
	require.Greater(t, cache.deletedSessions[sessionHash], 0)
}

func TestGatewayUnpooled_StickyExpensiveCheapFullKeeps(t *testing.T) {
	// AC5
	groupID := int64(83)
	sessionHash := "gw-keep"
	repo := gatewayUnpooledAccounts(1, 2, 0.15, 1.0)
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{sessionHash: 2}}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
			groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
		}},
		cache: cache,
		cfg:   cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, LoadRate: 100},
			2: {AccountID: 2, LoadRate: 10},
		}}),
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, "", nil, "", int64(16))
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Account.ID)
	require.Zero(t, cache.deletedSessions[sessionHash])
}

func TestGatewayMixed_AGPoolStickyNotEscaped(t *testing.T) {
	// AC8b: anthropic group + AG sticky + AG pool enabled must not escape.
	groupID := int64(85)
	sessionHash := "gw-ag-pool"
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{ID: 20, Platform: PlatformAntigravity, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 2, UpstreamRateMultiplier: testRate(1.0), Extra: map[string]any{"mixed_scheduling": true}},
			{ID: 21, Platform: PlatformAntigravity, Priority: 9, Status: StatusActive, Schedulable: true, Concurrency: 2, UpstreamRateMultiplier: testRate(0.15), Extra: map[string]any{"mixed_scheduling": true}},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{sessionHash: 20}}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
			groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
		}},
		cache: cache,
		cfg:   cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{
			20: {AccountID: 20, LoadRate: 10},
			21: {AccountID: 21, LoadRate: 0},
		}}),
		smartScheduleCache: testSmartLookup(PlatformAntigravity, 20, 21),
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, "", nil, "", int64(16))
	require.NoError(t, err)
	require.Equal(t, int64(20), result.Account.ID)
	require.Zero(t, cache.deletedSessions[sessionHash])
}

func TestGatewayPooled_NoStickyEscape(t *testing.T) {
	// AC8
	groupID := int64(84)
	sessionHash := "gw-pool"
	repo := gatewayUnpooledAccounts(1, 2, 0.15, 1.0)
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{sessionHash: 2}}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
			groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
		}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 0}, 2: {AccountID: 2, LoadRate: 10}}}),
		smartScheduleCache: testSmartLookup(PlatformAnthropic, 1, 2),
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, "", nil, "", int64(16))
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Account.ID)
	require.Zero(t, cache.deletedSessions[sessionHash])
}

func TestGatewayPooled_OverflowEscapesOnce(t *testing.T) {
	groupID := int64(86)
	sessionHash := "gw-pool-ov"
	repo := gatewayUnpooledAccounts(1, 2, 0.15, 1.0)
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{sessionHash: 2}, sessionOverflow: map[string]bool{sessionHash: true}}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
			groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
		}},
		cache: cache,
		cfg:   cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, LoadRate: 0},
			2: {AccountID: 2, LoadRate: 10},
		}}),
		smartScheduleCache: testSmartLookup(PlatformAnthropic, 1, 2),
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, "", nil, "", int64(16))
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Account.ID)
	require.Greater(t, cache.deletedSessions[sessionHash], 0)
	require.Equal(t, int64(1), cache.sessionBindings[sessionHash])
	require.False(t, cache.sessionOverflow[sessionHash])
}

func TestGatewayPooled_NewSessionCheapestNoOverflow(t *testing.T) {
	groupID := int64(87)
	sessionHash := "gw-new-min"
	repo := gatewayUnpooledAccounts(1, 2, 0.15, 1.0)
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cache := &mockGatewayCacheForPlatform{}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
			groupID: {ID: groupID, Platform: PlatformAnthropic, Status: StatusActive, Hydrated: true},
		}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 10}, 2: {AccountID: 2, LoadRate: 10}}}),
		smartScheduleCache: testSmartLookup(PlatformAnthropic, 1, 2),
	}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	result, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, sessionHash, "", nil, "", int64(16))
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Account.ID)
	require.False(t, cache.sessionOverflow[sessionHash])
	require.Equal(t, int64(1), cache.sessionBindings[sessionHash])
}
