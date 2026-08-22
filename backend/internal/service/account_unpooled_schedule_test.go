//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func testRate(v float64) *float64 { return &v }

func testEnabledSmartPolicy(accountIDs ...int64) *SmartSchedulePlatformPolicy {
	ids := make(map[int64]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		ids[id] = struct{}{}
	}
	return &SmartSchedulePlatformPolicy{Enabled: true, AccountIDs: ids}
}

func testSmartLookup(platform string, accountIDs ...int64) *memorySmartLookup {
	return &memorySmartLookup{
		bundle: &UserSmartScheduleBundle{
			Policies: map[string]*SmartSchedulePlatformPolicy{
				platform: testEnabledSmartPolicy(accountIDs...),
			},
		},
	}
}

func TestAccountCheaperThenPreferred(t *testing.T) {
	cheap := &Account{ID: 1, UpstreamRateMultiplier: testRate(0.15)}
	expensive := &Account{ID: 2, UpstreamRateMultiplier: testRate(1.0)}
	require.True(t, accountCheaperThenPreferred(cheap, expensive))
	require.False(t, accountCheaperThenPreferred(expensive, cheap))
	require.False(t, accountCheaperThenPreferred(cheap, cheap))
	require.False(t, accountCheaperThenPreferred(nil, expensive))
}

func TestShouldEscapeSessionStickyForCheaperTier_UnpooledCheapHeadroom(t *testing.T) {
	// AC4
	sticky := &Account{ID: 2, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(1.0)}
	cheap := &Account{ID: 1, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(0.15)}
	load := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 10},
		2: {AccountID: 2, LoadRate: 10},
	}
	require.True(t, shouldEscapeSessionStickyForCheaperTier(
		context.Background(), nil, 16, sticky, []*Account{cheap, sticky}, load,
	))
}

func TestShouldEscapeSessionStickyForCheaperTier_CheapFullKeepsSticky(t *testing.T) {
	// AC5
	sticky := &Account{ID: 2, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(1.0)}
	cheap := &Account{ID: 1, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(0.15)}
	load := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 100},
		2: {AccountID: 2, LoadRate: 10},
	}
	require.False(t, shouldEscapeSessionStickyForCheaperTier(
		context.Background(), nil, 16, sticky, []*Account{cheap, sticky}, load,
	))
}

func TestShouldEscapeSessionStickyForCheaperTier_PooledNoEscape(t *testing.T) {
	// AC8
	sticky := &Account{ID: 2, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(1.0)}
	cheap := &Account{ID: 1, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(0.15)}
	load := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 0},
		2: {AccountID: 2, LoadRate: 0},
	}
	lookup := testSmartLookup(PlatformOpenAI, 1, 2)
	require.False(t, shouldEscapeSessionStickyForCheaperTier(
		context.Background(), lookup, 16, sticky, []*Account{cheap, sticky}, load,
	))
}

func TestShouldEscapeSessionStickyForCheaperTier_AGOffBridgeUsesOpenAIPool(t *testing.T) {
	sticky := &Account{ID: 7, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(1.0)}
	cheap := &Account{ID: 8, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(0.15)}
	load := map[int64]*AccountLoadInfo{7: {AccountID: 7, LoadRate: 0}, 8: {AccountID: 8, LoadRate: 0}}
	lookup := &memorySmartLookup{bundle: smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 0, nil))}
	require.False(t, shouldEscapeSessionStickyForCheaperTier(
		agGroupScheduleCtx(16), lookup, 16, sticky, []*Account{cheap, sticky}, load,
	), "AG off + openai closed pool must not treat bridge as unpooled")
}

func TestShouldEscapeSessionStickyForCheaperTier_AGOnBridgeUsesAGPool(t *testing.T) {
	sticky := &Account{ID: 7, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(1.0)}
	cheap := &Account{ID: 8, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(0.15)}
	load := map[int64]*AccountLoadInfo{7: {AccountID: 7, LoadRate: 0}, 8: {AccountID: 8, LoadRate: 0}}
	lookup := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(8, 0, nil),
		PlatformAntigravity: enabledSmartPolicy(7, 0, nil),
	}}}
	require.False(t, shouldEscapeSessionStickyForCheaperTier(
		agGroupScheduleCtx(16), lookup, 16, sticky, []*Account{cheap, sticky}, load,
	), "AG on must judge unpooled against antigravity, not openai")
}

func TestShouldEscapeSessionStickyForCheaperTier_MixedAGPoolUsesStickyPlatform(t *testing.T) {
	// AC8b: group may be anthropic, but sticky is AG and AG pool is enabled.
	sticky := &Account{ID: 20, Platform: PlatformAntigravity, UpstreamRateMultiplier: testRate(1.0)}
	cheap := &Account{ID: 21, Platform: PlatformAntigravity, UpstreamRateMultiplier: testRate(0.15)}
	load := map[int64]*AccountLoadInfo{
		20: {AccountID: 20, LoadRate: 0},
		21: {AccountID: 21, LoadRate: 0},
	}
	lookup := testSmartLookup(PlatformAntigravity, 20, 21)
	require.False(t, shouldEscapeSessionStickyForCheaperTier(
		context.Background(), lookup, 16, sticky, []*Account{cheap, sticky}, load,
	), "must use sticky.Platform, not group platform")
}

func TestShouldEscapeSessionStickyForCheaperTier_UserIDZeroFailOpen(t *testing.T) {
	// AC8c: userID<=0 is today's fail-open (unpooled), not a new pooled policy.
	sticky := &Account{ID: 2, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(1.0)}
	cheap := &Account{ID: 1, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(0.15)}
	load := map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 0}}
	lookup := testSmartLookup(PlatformOpenAI, 1, 2)
	require.True(t, shouldEscapeSessionStickyForCheaperTier(
		context.Background(), lookup, 0, sticky, []*Account{cheap, sticky}, load,
	))
	require.Nil(t, lookupEnabledSmartPolicy(context.Background(), lookup, 0, PlatformOpenAI))
}

func TestIsBetterAccount_UpstreamRateWins(t *testing.T) {
	// AC9
	svc := &OpenAIGatewayService{}
	cheap := &Account{ID: 1, Priority: 50, UpstreamRateMultiplier: testRate(0.15)}
	expensive := &Account{ID: 2, Priority: 1, UpstreamRateMultiplier: testRate(1.0)}
	require.True(t, svc.isBetterAccount(cheap, expensive))
	require.False(t, svc.isBetterAccount(expensive, cheap))
}

func TestIsBetterGeminiAccount_UpstreamRateWins(t *testing.T) {
	// AC9
	svc := &GeminiMessagesCompatService{}
	cheap := &Account{ID: 1, Priority: 50, Type: AccountTypeAPIKey, UpstreamRateMultiplier: testRate(0.15)}
	expensive := &Account{ID: 2, Priority: 1, Type: AccountTypeOAuth, UpstreamRateMultiplier: testRate(1.0)}
	require.True(t, svc.isBetterGeminiAccount(cheap, expensive))
	require.False(t, svc.isBetterGeminiAccount(expensive, cheap))
}

func TestUnpooledHelpers_DoNotReadBillingRate(t *testing.T) {
	// AC10
	billing := 9.9
	upstream := 0.15
	a := &Account{ID: 1, RateMultiplier: &billing, UpstreamRateMultiplier: &upstream, Type: AccountTypeOAuth}
	b := &Account{ID: 2, RateMultiplier: testRate(0.01), UpstreamRateMultiplier: testRate(1.0), Type: AccountTypeOAuth}
	require.Equal(t, 9.9, a.BillingRateMultiplier())
	require.Equal(t, 0.15, a.EffectiveUpstreamRate())
	require.True(t, accountCheaperThenPreferred(a, b))
	require.True(t, isBetterSchedulableAccount(a, b, false))
	require.Equal(t, 9.9, a.BillingRateMultiplier())
	require.Equal(t, 0.01, b.BillingRateMultiplier())
}

func TestShouldSkipMinRateWaitPlan_UnpooledHigherHeadroom(t *testing.T) {
	cheap := &Account{ID: 1, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(0.15)}
	expensive := &Account{ID: 2, Platform: PlatformOpenAI, UpstreamRateMultiplier: testRate(1.0)}
	load := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 100},
		2: {AccountID: 2, LoadRate: 0},
	}
	require.True(t, shouldSkipMinRateWaitPlan(context.Background(), nil, 16, cheap, []*Account{cheap, expensive}, load))
	require.False(t, shouldSkipMinRateWaitPlan(context.Background(), nil, 16, expensive, []*Account{cheap, expensive}, load))
	lookup := testSmartLookup(PlatformOpenAI, 1, 2)
	require.False(t, shouldSkipMinRateWaitPlan(context.Background(), lookup, 16, cheap, []*Account{cheap, expensive}, load))
}

func TestLookupEnabledSmartPolicy_UsesAccountPlatformNotGroup(t *testing.T) {
	lookup := testSmartLookup(PlatformAntigravity, 20)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	require.NotNil(t, lookupEnabledSmartPolicy(ctx, lookup, 16, PlatformAntigravity))
	require.Nil(t, lookupEnabledSmartPolicy(ctx, lookup, 16, PlatformAnthropic))
}
