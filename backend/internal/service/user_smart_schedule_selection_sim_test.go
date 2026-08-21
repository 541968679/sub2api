//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// Local multi-account simulation for the production selection entrypoints.
// Brandon has few live accounts, so these cases stand in for a mixed fleet
// before any production push.

const (
	simUserID            = int64(16)
	simAnthropicModel    = "claude-3-5-sonnet-20241022"
	simOpenAIModel       = "gpt-5.1"
	simOpenAIGroupID     = int64(91001)
	simOutsideCheapID    = int64(10)
	simDeniedInPoolID    = int64(11)
	simPairCappedID      = int64(12)
	simQualitySlowID     = int64(13)
	simFallbackOnlyID    = int64(14)
	simExpensiveID       = int64(15)
	simCheapOAuthID      = int64(16)
)

func simRate(v float64) *float64 { return &v }

func simInt(v int) *int { return &v }

func simAccount(id int64, platform, name string, rate float64, priority int) Account {
	return Account{
		ID:                     id,
		Name:                   name,
		Platform:               platform,
		Type:                   AccountTypeAPIKey,
		Status:                 StatusActive,
		Schedulable:            true,
		Concurrency:            5,
		Priority:               priority,
		UpstreamRateMultiplier: simRate(rate),
		GroupIDs:               []int64{simOpenAIGroupID},
	}
}

func simAnthropicFleet() []Account {
	outside := simAccount(simOutsideCheapID, PlatformAnthropic, "outside-cheap", 0.05, 0)
	denied := simAccount(simDeniedInPoolID, PlatformAnthropic, "denied-in-pool", 0.20, 0)
	denied.DenyUserIDs = []int64{simUserID}
	denied.RateMultiplier = simRate(0.01) // billing cheap must not win scheduling
	pair := simAccount(simPairCappedID, PlatformAnthropic, "pair-capped", 0.10, 1)
	pair.UserConcurrency = map[int64]int{simUserID: 9} // legacy cap must be ignored in-pool
	quality := simAccount(simQualitySlowID, PlatformAnthropic, "quality-slow", 0.08, 1)
	fallback := simAccount(simFallbackOnlyID, PlatformAnthropic, "fallback-only", 0.01, 99)
	fallback.SetFallbackOnly(true)
	expensive := simAccount(simExpensiveID, PlatformAnthropic, "expensive-primary", 1.00, 0)
	cheap := simAccount(simCheapOAuthID, PlatformAnthropic, "cheap-oauth", 0.15, 20)
	return []Account{outside, denied, pair, quality, fallback, expensive, cheap}
}

func simOpenAIFleet() []Account {
	fleet := simAnthropicFleet()
	for i := range fleet {
		fleet[i].Platform = PlatformOpenAI
	}
	return fleet
}

func simEnabledPolicy(p50 *int, caps map[int64]int) *SmartSchedulePlatformPolicy {
	policy := &SmartSchedulePlatformPolicy{
		Enabled:         true,
		CooldownMinutes: 15,
		AccountIDs: map[int64]struct{}{
			simDeniedInPoolID: {},
			simPairCappedID:   {},
			simQualitySlowID:  {},
			simFallbackOnlyID: {},
			simExpensiveID:    {},
			simCheapOAuthID:   {},
		},
		Caps:                caps,
		QualityMaxP50TTFTMs: p50,
	}
	if policy.Caps == nil {
		policy.Caps = map[int64]int{}
	}
	return policy
}

func simQualityCache() *liveQualityCacheStub {
	return &liveQualityCacheStub{
		byID: map[int64]*AccountQualityStats{
			simQualitySlowID: liveQualityStats(4000, 12, 20, 0, 1),
		},
	}
}

func simPairLookup(bundle *UserSmartScheduleBundle) *memorySmartLookup {
	return &memorySmartLookup{
		bundle: bundle,
		pair: map[string]*PairQualityLive{
			smartPairKey(simQualitySlowID, simUserID): breachedPairLive(4000),
		},
	}
}

func newSimAccountRepo(accounts []Account) *mockAccountRepoForPlatform {
	repo := &mockAccountRepoForPlatform{
		accounts:     append([]Account(nil), accounts...),
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}
	return repo
}

func newAnthropicSimService(
	accounts []Account,
	lookup *memorySmartLookup,
	pairCounts map[int64]int,
	quality AccountQualityLiveCache,
	sticky map[string]int64,
) *GatewayService {
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	return &GatewayService{
		accountRepo: newSimAccountRepo(accounts),
		cache:       &mockGatewayCacheForPlatform{sessionBindings: sticky},
		cfg:         cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{
			pairCounts: pairCounts,
		}),
		qualityLiveCache:   quality,
		smartScheduleCache: lookup,
	}
}

func newOpenAISimService(
	accounts []Account,
	lookup *memorySmartLookup,
	pairCounts map[int64]int,
	quality AccountQualityLiveCache,
	sticky map[string]int64,
) *OpenAIGatewayService {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1
	return &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: append([]Account(nil), accounts...)},
		cache:            &schedulerTestGatewayCache{sessionBindings: sticky},
		cfg:              cfg,
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			pairCounts: pairCounts,
		}),
		qualityLiveCache:   quality,
		smartScheduleCache: lookup,
	}
}

func pickAnthropic(t *testing.T, svc *GatewayService, sessionHash string, excluded map[int64]struct{}) *AccountSelectionResult {
	t.Helper()
	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, sessionHash, simAnthropicModel, excluded, "", simUserID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Nil(t, result.WaitPlan)
	return result
}

func pickOpenAI(t *testing.T, svc *OpenAIGatewayService, sessionHash string, excluded map[int64]struct{}) *AccountSelectionResult {
	t.Helper()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, simUserID)
	groupID := simOpenAIGroupID
	result, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", sessionHash, simOpenAIModel, excluded, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Nil(t, result.WaitPlan)
	return result
}

func TestSmartScheduleMultiAccount_AnthropicLoadAware(t *testing.T) {
	t.Parallel()
	p50 := 1000
	caps := map[int64]int{simPairCappedID: 1}

	t.Run("closed pool picks cheapest eligible primary", func(t *testing.T) {
		t.Parallel()
		lookup := simPairLookup(smartBundle(PlatformAnthropic, simEnabledPolicy(&p50, caps)))
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, nil, simQualityCache(), nil)
		got := pickAnthropic(t, svc, "", nil)
		require.Equal(t, simPairCappedID, got.Account.ID, "0.10 in-pool primary must beat 0.15/0.20/1.0 and ignore outside 0.05 plus fallback 0.01")
		require.Equal(t, 1, lookup.startCalls, "quality-slow must start pair cooldown, not TempUnschedulable")
		require.True(t, got.Account.Schedulable)
		require.Nil(t, got.Account.TempUnschedulableUntil)
	})

	t.Run("pair cap full reselects next cheapest", func(t *testing.T) {
		t.Parallel()
		lookup := simPairLookup(smartBundle(PlatformAnthropic, simEnabledPolicy(&p50, caps)))
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, map[int64]int{simPairCappedID: 1}, simQualityCache(), nil)
		got := pickAnthropic(t, svc, "", nil)
		require.Equal(t, simCheapOAuthID, got.Account.ID, "after pair-full 0.10, next cheapest eligible is 0.15 not denied-legacy 0.20 or billing-cheap 1.0")
	})

	t.Run("quality cooldown stays blocked after live recovery", func(t *testing.T) {
		t.Parallel()
		lookup := simPairLookup(smartBundle(PlatformAnthropic, simEnabledPolicy(&p50, caps)))
		quality := simQualityCache()
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, map[int64]int{simPairCappedID: 1}, quality, nil)
		first := pickAnthropic(t, svc, "", nil)
		require.Equal(t, simCheapOAuthID, first.Account.ID)
		require.Equal(t, 1, lookup.startCalls)

		lookup.pair[smartPairKey(simQualitySlowID, simUserID)] = breachedPairLive(200)
		quality.byID[simQualitySlowID] = liveQualityStats(200, 12, 20, 0, 1)
		second := pickAnthropic(t, svc, "", nil)
		require.Equal(t, simCheapOAuthID, second.Account.ID, "cooldown must keep excluding the recovered cheapest account")
		require.Equal(t, 1, lookup.startCalls, "StartCooldown must not extend an active window")
	})

	t.Run("sticky keeps expensive pin while it still admits", func(t *testing.T) {
		t.Parallel()
		lookup := simPairLookup(smartBundle(PlatformAnthropic, simEnabledPolicy(&p50, caps)))
		cache := map[string]int64{"sticky-expensive": simExpensiveID}
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, nil, simQualityCache(), cache)
		got := pickAnthropic(t, svc, "sticky-expensive", nil)
		require.Equal(t, simExpensiveID, got.Account.ID)
		require.Equal(t, simExpensiveID, cache["sticky-expensive"])
	})

	t.Run("sticky quality miss clears pin and reselects", func(t *testing.T) {
		t.Parallel()
		lookup := simPairLookup(smartBundle(PlatformAnthropic, simEnabledPolicy(&p50, caps)))
		cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky-quality": simQualitySlowID}}
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, nil, simQualityCache(), cache.sessionBindings)
		svc.cache = cache
		got := pickAnthropic(t, svc, "sticky-quality", nil)
		require.Equal(t, simPairCappedID, got.Account.ID)
		require.Equal(t, 1, cache.deletedSessions["sticky-quality"])
		require.Equal(t, simPairCappedID, cache.sessionBindings["sticky-quality"])
	})

	t.Run("fallback_only waits until primaries are gone", func(t *testing.T) {
		t.Parallel()
		lookup := simPairLookup(smartBundle(PlatformAnthropic, simEnabledPolicy(&p50, caps)))
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, nil, simQualityCache(), nil)
		got := pickAnthropic(t, svc, "", map[int64]struct{}{
			simDeniedInPoolID: {},
			simPairCappedID:   {},
			simQualitySlowID:  {},
			simExpensiveID:    {},
			simCheapOAuthID:   {},
		})
		require.Equal(t, simFallbackOnlyID, got.Account.ID, "outside-pool 0.05 must still lose to in-pool fallback")
	})

	t.Run("enable off uses legacy deny and cheapest overlay", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, &SmartSchedulePlatformPolicy{
			Enabled:    false,
			AccountIDs: map[int64]struct{}{simDeniedInPoolID: {}, simPairCappedID: {}, simCheapOAuthID: {}},
			Caps:       map[int64]int{simPairCappedID: 1},
		})}
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, nil, simQualityCache(), nil)
		got := pickAnthropic(t, svc, "", nil)
		require.Equal(t, simOutsideCheapID, got.Account.ID, "disabled policy must reopen the unrestricted 0.05 account and keep legacy deny")
	})

	t.Run("enabled empty pool fails open to legacy", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, &SmartSchedulePlatformPolicy{
			Enabled:    true,
			AccountIDs: map[int64]struct{}{},
		})}
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, nil, simQualityCache(), nil)
		got := pickAnthropic(t, svc, "", nil)
		require.Equal(t, simOutsideCheapID, got.Account.ID)
	})

	t.Run("sequential occupancy walks cheapest then next", func(t *testing.T) {
		t.Parallel()
		lookup := simPairLookup(smartBundle(PlatformAnthropic, simEnabledPolicy(&p50, caps)))
		pairCounts := map[int64]int{}
		quality := simQualityCache()
		svc := newAnthropicSimService(simAnthropicFleet(), lookup, pairCounts, quality, nil)

		first := pickAnthropic(t, svc, "", nil)
		require.Equal(t, simPairCappedID, first.Account.ID)

		pairCounts[simPairCappedID] = 1
		second := pickAnthropic(t, svc, "", nil)
		require.Equal(t, simCheapOAuthID, second.Account.ID)

		third := pickAnthropic(t, svc, "", map[int64]struct{}{simCheapOAuthID: {}})
		require.Equal(t, simDeniedInPoolID, third.Account.ID, "in-pool legacy deny must still be schedulable after cheaper peers are taken")
	})
}

func TestSmartScheduleMultiAccount_OpenAIScheduler(t *testing.T) {
	// The advanced scheduler reads a process-wide settings cache / singleflight.
	// Keep this test serial so it does not race other OpenAI scheduler tests.
	p50 := 1000
	caps := map[int64]int{simPairCappedID: 1}

	t.Run("closed pool picks cheapest eligible primary", func(t *testing.T) {
		lookup := simPairLookup(smartBundle(PlatformOpenAI, simEnabledPolicy(&p50, caps)))
		svc := newOpenAISimService(simOpenAIFleet(), lookup, nil, simQualityCache(), nil)
		got := pickOpenAI(t, svc, "", nil)
		require.Equal(t, simPairCappedID, got.Account.ID)
	})

	t.Run("pair cap full reselects next cheapest", func(t *testing.T) {
		lookup := simPairLookup(smartBundle(PlatformOpenAI, simEnabledPolicy(&p50, caps)))
		svc := newOpenAISimService(simOpenAIFleet(), lookup, map[int64]int{simPairCappedID: 1}, simQualityCache(), nil)
		got := pickOpenAI(t, svc, "", nil)
		require.Equal(t, simCheapOAuthID, got.Account.ID)
	})

	t.Run("outside pool and fallback stay excluded while primaries exist", func(t *testing.T) {
		lookup := simPairLookup(smartBundle(PlatformOpenAI, simEnabledPolicy(&p50, caps)))
		svc := newOpenAISimService(simOpenAIFleet(), lookup, nil, simQualityCache(), nil)
		got := pickOpenAI(t, svc, "", nil)
		require.NotEqual(t, simOutsideCheapID, got.Account.ID)
		require.NotEqual(t, simFallbackOnlyID, got.Account.ID)
		require.NotEqual(t, simQualitySlowID, got.Account.ID)
	})

	t.Run("sticky keeps expensive pin", func(t *testing.T) {
		lookup := simPairLookup(smartBundle(PlatformOpenAI, simEnabledPolicy(&p50, caps)))
		svc := newOpenAISimService(simOpenAIFleet(), lookup, nil, simQualityCache(), map[string]int64{"openai:sticky-expensive": simExpensiveID})
		got := pickOpenAI(t, svc, "sticky-expensive", nil)
		require.Equal(t, simExpensiveID, got.Account.ID)
	})

	t.Run("enable off reopens unrestricted cheapest", func(t *testing.T) {
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformOpenAI, &SmartSchedulePlatformPolicy{
			Enabled:    false,
			AccountIDs: map[int64]struct{}{simDeniedInPoolID: {}, simPairCappedID: {}},
		})}
		svc := newOpenAISimService(simOpenAIFleet(), lookup, nil, simQualityCache(), nil)
		got := pickOpenAI(t, svc, "", nil)
		require.Equal(t, simOutsideCheapID, got.Account.ID)
	})
}
