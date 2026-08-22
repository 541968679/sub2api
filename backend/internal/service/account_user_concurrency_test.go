//go:build unit

package service

import (
	"context"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// pairOccupancyCache is an in-memory ConcurrencyCache that records pair
// acquire max values and live occupancy so tests can assert count-only writes.
type pairOccupancyCache struct {
	stubConcurrencyCacheForTest
	mu      sync.Mutex
	slots   map[int64]map[string]struct{}
	lastMax []int
}

func (c *pairOccupancyCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}

func (c *pairOccupancyCache) AcquireAccountUserSlot(_ context.Context, accountID, _ int64, maxConcurrency int, requestID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastMax = append(c.lastMax, maxConcurrency)
	if c.slots == nil {
		c.slots = map[int64]map[string]struct{}{}
	}
	if maxConcurrency > 0 && len(c.slots[accountID]) >= maxConcurrency {
		return false, nil
	}
	if c.slots[accountID] == nil {
		c.slots[accountID] = map[string]struct{}{}
	}
	c.slots[accountID][requestID] = struct{}{}
	return true, nil
}

func (c *pairOccupancyCache) ReleaseAccountUserSlot(_ context.Context, accountID, _ int64, requestID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.slots[accountID], requestID)
	return nil
}

func (c *pairOccupancyCache) GetAccountUserConcurrencyBatch(_ context.Context, accountIDs []int64, _ int64) (map[int64]int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = len(c.slots[id])
	}
	return out, nil
}

func (c *pairOccupancyCache) count(accountID int64) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.slots[accountID])
}

func (c *pairOccupancyCache) maxes() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]int, len(c.lastMax))
	copy(out, c.lastMax)
	return out
}

func testPairAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Platform:    PlatformAnthropic,
		Concurrency: 8,
	}
}

func TestResolvePairSlotAcquire_AGLookupFollowsClosedPoolSwitch(t *testing.T) {
	t.Parallel()
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	agOff := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 3, nil),
		PlatformAntigravity: {Enabled: false, AccountIDs: map[int64]struct{}{7: {}}},
	}}}
	agOn := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 3, nil),
		PlatformAntigravity: enabledSmartPolicy(7, 9, nil),
	}}}
	openaiOnly := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 3, nil),
		PlatformAntigravity: enabledSmartPolicy(8, 9, nil),
	}}}

	agCtx := agGroupScheduleCtx(16)
	nativeCtx := nativeOpenAIGroupScheduleCtx(16)

	max, track := resolvePairSlotAcquire(agCtx, oai, agOff)
	require.Equal(t, 3, max)
	require.True(t, track, "AG off must use openai closed-pool occupancy")

	max, track = resolvePairSlotAcquire(agCtx, oai, agOn)
	require.Equal(t, 9, max)
	require.True(t, track, "AG on must use antigravity closed-pool occupancy")

	max, track = resolvePairSlotAcquire(agCtx, oai, openaiOnly)
	require.Equal(t, 0, max)
	require.False(t, track, "AG on must not fall back to openai pair cap")

	max, track = resolvePairSlotAcquire(nativeCtx, oai, agOn)
	require.Equal(t, 3, max)
	require.True(t, track, "native GPT stays on openai pair shard")
}

func TestResolvePairSlotAcquire_ClosedPoolTracksUncapped(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(7)
	lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}

	max, track := resolvePairSlotAcquire(ctx, acc, lookup)
	require.Equal(t, 0, max)
	require.True(t, track)
	require.Equal(t, 0, resolvePairMaxConcurrency(ctx, acc, lookup))
	require.False(t, isPairConcurrencyFull(ctx, acc, 3, lookup))
	require.False(t, isPairConcurrencyFull(ctx, acc, 999, lookup), "999 is UI-only and must never pair_full")
}

func TestResolvePairSlotAcquire_ProbingUsesPolicyN(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(7)

	t.Run("no member cap uses N", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil)),
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		max, track := resolvePairSlotAcquire(ctx, acc, lookup)
		require.Equal(t, DefaultSmartScheduleWindowN, max)
		require.True(t, track)
		require.NotEqual(t, 999, max)
		require.True(t, isPairConcurrencyFull(ctx, acc, DefaultSmartScheduleWindowN, lookup))
	})

	t.Run("member cap smaller than N", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 2, nil)),
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		max, track := resolvePairSlotAcquire(ctx, acc, lookup)
		require.Equal(t, 2, max)
		require.True(t, track)
	})

	t.Run("member cap larger than N", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 20, nil)),
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		max, track := resolvePairSlotAcquire(ctx, acc, lookup)
		require.Equal(t, DefaultSmartScheduleWindowN, max)
		require.True(t, track)
	})

	t.Run("not probing keeps member cap", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 2, nil))}
		max, track := resolvePairSlotAcquire(ctx, acc, lookup)
		require.Equal(t, 2, max)
		require.True(t, track)
	})

	t.Run("custom 2 with cap 5 uses 2", func(t *testing.T) {
		t.Parallel()
		policy := enabledSmartPolicy(7, 5, nil)
		policy.QualityWindowSamples = intPtr(10)
		policy.ProbeConcurrencyMode = ProbeConcurrencyModeCustom
		policy.ProbeConcurrency = intPtr(2)
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, policy),
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		max, track := resolvePairSlotAcquire(ctx, acc, lookup)
		require.Equal(t, 2, max)
		require.True(t, track)
	})

	t.Run("custom 10 with cap 3 uses member ceiling", func(t *testing.T) {
		t.Parallel()
		policy := enabledSmartPolicy(7, 3, nil)
		policy.QualityWindowSamples = intPtr(10)
		policy.ProbeConcurrencyMode = ProbeConcurrencyModeCustom
		policy.ProbeConcurrency = intPtr(10)
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, policy),
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		max, track := resolvePairSlotAcquire(ctx, acc, lookup)
		require.Equal(t, 3, max)
		require.True(t, track)
	})
}

func TestResolvePairSlotAcquire_PinnedUsesMemberCap(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(7)

	t.Run("member cap not probe cap", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 8, nil)),
			pinned:  map[string]bool{smartPairKey(7, 16): true},
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		max, track := resolvePairSlotAcquire(ctx, acc, lookup)
		require.Equal(t, 8, max)
		require.True(t, track)
	})

	t.Run("leftover probe does not clamp uncapped pin", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil)),
			pinned:  map[string]bool{smartPairKey(7, 16): true},
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		max, track := resolvePairSlotAcquire(ctx, acc, lookup)
		require.Equal(t, 0, max, "pinned uses member cap, never probe N")
		require.True(t, track)
		require.False(t, isPairConcurrencyFull(ctx, acc, DefaultSmartScheduleWindowN, lookup))
	})
}

func TestResolvePairSlotAcquire_ClosedPoolCapped(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(7)
	lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 2, nil))}

	max, track := resolvePairSlotAcquire(ctx, acc, lookup)
	require.Equal(t, 2, max)
	require.True(t, track)
	require.False(t, isPairConcurrencyFull(ctx, acc, 1, lookup))
	require.True(t, isPairConcurrencyFull(ctx, acc, 2, lookup))
	require.True(t, isPairConcurrencyFull(ctx, acc, 999, lookup), "current 999 is still full against a real cap of 2")
	require.NotEqual(t, 999, max)
}

func TestResolvePairSlotAcquire_NotInClosedPoolDoesNotTrack(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(8)
	lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}

	max, track := resolvePairSlotAcquire(ctx, acc, lookup)
	require.Equal(t, 0, max)
	require.False(t, track)
}

func TestResolvePairSlotAcquire_LegacyUncappedDoesNotTrack(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(7)
	acc.Concurrency = 4

	max, track := resolvePairSlotAcquire(ctx, acc, nil)
	require.Equal(t, 0, max)
	require.False(t, track, "must not write pair slots globally when not in a closed pool")
}

func TestResolvePairSlotAcquire_LegacyCappedDoesNotUseAccountConcurrency(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(7)
	acc.Concurrency = 8
	acc.UserConcurrency = map[int64]int{16: 3}

	max, track := resolvePairSlotAcquire(ctx, acc, nil)
	require.Equal(t, 3, max)
	require.False(t, track)
	require.NotEqual(t, acc.Concurrency, max)
}

func TestIsPairConcurrencyFull_NeverUsesDisplay999(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(7)
	lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}

	require.False(t, isPairConcurrencyFull(ctx, acc, 0, lookup))
	require.False(t, isPairConcurrencyFull(ctx, acc, 1, lookup))
	require.False(t, isPairConcurrencyFull(ctx, acc, 998, lookup))
	require.False(t, isPairConcurrencyFull(ctx, acc, 999, lookup))
	require.False(t, isPairConcurrencyFull(ctx, acc, 1000, lookup))
}

func TestAcquireAccountAndPairSlot_UncappedClosedPoolWritesAndReleases(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	svc := NewConcurrencyService(cache)
	acc := testPairAccount(11)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))

	result, pairFull, err := acquireAccountAndPairSlot(ctx, svc, acc, 16, 0, true)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.True(t, result.Acquired)
	require.Equal(t, []int{0}, cache.maxes(), "uncapped must pass max=0, never 999")
	require.Equal(t, 1, cache.count(11))

	result.ReleaseFunc()
	require.Equal(t, 0, cache.count(11))
}

func TestAcquireAccountAndPairSlot_UncappedNeverPairFull(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	svc := NewConcurrencyService(cache)
	acc := testPairAccount(11)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		result, pairFull, err := acquireAccountAndPairSlot(ctx, svc, acc, 16, 0, true)
		require.NoError(t, err)
		require.False(t, pairFull, "uncapped must never report pair_full")
		require.True(t, result.Acquired)
	}
	require.Equal(t, 5, cache.count(11))
	for _, max := range cache.maxes() {
		require.LessOrEqual(t, max, 0)
		require.NotEqual(t, 999, max)
	}
}

func TestAcquireAccountAndPairSlot_CappedPairFullAtRealMax(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	svc := NewConcurrencyService(cache)
	acc := testPairAccount(11)
	ctx := context.Background()

	first, pairFull, err := acquireAccountAndPairSlot(ctx, svc, acc, 16, 2, true)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.True(t, first.Acquired)

	second, pairFull, err := acquireAccountAndPairSlot(ctx, svc, acc, 16, 2, true)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.True(t, second.Acquired)

	third, pairFull, err := acquireAccountAndPairSlot(ctx, svc, acc, 16, 2, true)
	require.NoError(t, err)
	require.True(t, pairFull)
	require.False(t, third.Acquired)
	require.Equal(t, 2, cache.count(11))
	require.Equal(t, []int{2, 2, 2}, cache.maxes())
}

func TestAcquireAccountAndPairSlot_NotInPoolSkipsPairWrite(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	svc := NewConcurrencyService(cache)
	acc := testPairAccount(11)

	result, pairFull, err := acquireAccountAndPairSlot(context.Background(), svc, acc, 16, 0, false)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.True(t, result.Acquired)
	require.Empty(t, cache.maxes())
	require.Equal(t, 0, cache.count(11))
}

func TestAttachPairSlotHoldingAccount_PairFullReleasesAccountSlot(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	svc := NewConcurrencyService(cache)
	acc := testPairAccount(11)
	acc.UserConcurrency = map[int64]int{16: 1}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))

	first, pairFull, err := acquireAccountAndPairSlot(ctx, svc, acc, 16, 1, true)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.True(t, first.Acquired)

	accountHeld := true
	accountRelease := func() { accountHeld = false }
	release, pairFull, err := attachPairSlotHoldingAccount(ctx, svc, nil, acc, accountRelease)
	require.NoError(t, err)
	require.True(t, pairFull)
	require.Nil(t, release)
	require.False(t, accountHeld, "pairFull must release the waited account slot")
	require.Equal(t, 1, cache.count(11))
}

func TestAttachPairSlotHoldingAccount_SuccessCombinesRelease(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	svc := NewConcurrencyService(cache)
	acc := testPairAccount(11)
	acc.UserConcurrency = map[int64]int{16: 2}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))

	accountReleased := false
	accountRelease := func() { accountReleased = true }
	release, pairFull, err := attachPairSlotHoldingAccount(ctx, svc, nil, acc, accountRelease)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.NotNil(t, release)
	require.Equal(t, 1, cache.count(11))
	release()
	require.True(t, accountReleased)
	require.Equal(t, 0, cache.count(11))
}

func TestAttachPairSlotHoldingAccount_UncappedNeverPairFull(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	svc := NewConcurrencyService(cache)
	acc := testPairAccount(11)
	lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(11, 0, nil))}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))

	release, pairFull, err := attachPairSlotHoldingAccount(ctx, svc, lookup, acc, func() {})
	require.NoError(t, err)
	require.False(t, pairFull)
	require.NotNil(t, release)
	require.Equal(t, []int{0}, cache.maxes())
}

func TestGatewayService_AttachPairSlotAfterAccountWait_PairFullReleases(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	conc := NewConcurrencyService(cache)
	acc := testPairAccount(11)
	acc.UserConcurrency = map[int64]int{16: 1}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))

	first, pairFull, err := acquireAccountAndPairSlot(ctx, conc, acc, 16, 1, true)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.True(t, first.Acquired)

	gw := &GatewayService{concurrencyService: conc}
	accountHeld := true
	release, pairFull, err := gw.AttachPairSlotAfterAccountWait(ctx, acc, func() { accountHeld = false })
	require.NoError(t, err)
	require.True(t, pairFull)
	require.Nil(t, release)
	require.False(t, accountHeld)

	oa := &OpenAIGatewayService{concurrencyService: conc}
	accountHeld = true
	release, pairFull, err = oa.AttachPairSlotAfterAccountWait(ctx, acc, func() { accountHeld = false })
	require.NoError(t, err)
	require.True(t, pairFull)
	require.Nil(t, release)
	require.False(t, accountHeld)
}

func TestSkipPairFullWait_ThisRequestPairFullBeatsStaleSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(11)
	acc.UserConcurrency = map[int64]int{16: 2}
	pairCounts := map[int64]int{11: 0}
	pairFullIDs := map[int64]struct{}{11: {}}
	require.True(t, skipPairFullWait(ctx, acc, pairCounts, pairFullIDs, nil))
	require.False(t, skipPairFullWait(ctx, acc, pairCounts, map[int64]struct{}{}, nil))
}

func TestTryAcquireAccountAndPairSlot_UncappedClosedPool(t *testing.T) {
	t.Parallel()
	cache := &pairOccupancyCache{}
	lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(11, 0, nil))}
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := testPairAccount(11)
	acc.Concurrency = 8

	gw := &GatewayService{
		concurrencyService: NewConcurrencyService(cache),
		smartScheduleCache: lookup,
	}
	result, pairFull, err := gw.tryAcquireAccountAndPairSlot(ctx, acc)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.True(t, result.Acquired)
	require.Equal(t, []int{0}, cache.maxes())
	require.Equal(t, 1, cache.count(11))
	result.ReleaseFunc()
	require.Equal(t, 0, cache.count(11))

	oa := &OpenAIGatewayService{
		concurrencyService: NewConcurrencyService(cache),
		smartScheduleCache: lookup,
	}
	acc.Platform = PlatformOpenAI
	lookup.bundle = smartBundle(PlatformOpenAI, enabledSmartPolicy(11, 0, nil))
	result, pairFull, err = oa.tryAcquireAccountAndPairSlot(ctx, acc)
	require.NoError(t, err)
	require.False(t, pairFull)
	require.True(t, result.Acquired)
	require.Equal(t, 1, cache.count(11))
	require.NotContains(t, cache.maxes(), 999)
	require.NotContains(t, cache.maxes(), acc.Concurrency)
}

func TestPairConcurrencyAccountIDs_UncappedClosedPoolNotSelectedForFullCheck(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	uncapped := testPairAccount(11)
	capped := testPairAccount(12)
	lookup := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformAnthropic: {
			Enabled:    true,
			AccountIDs: map[int64]struct{}{11: {}, 12: {}},
			Caps:       map[int64]int{12: 2},
		},
	}}}

	ids := pairConcurrencyAccountIDs(ctx, []*Account{uncapped, capped}, 16, lookup)
	require.Equal(t, []int64{12}, ids)
}
