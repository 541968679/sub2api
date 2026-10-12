//go:build unit

package service

import (
	"context"
	"testing"
	"time"

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

func TestParseEncodeStickySessionValue(t *testing.T) {
	plain, err := ParseStickySessionValue("1718")
	require.NoError(t, err)
	require.Equal(t, int64(1718), plain.AccountID)
	require.False(t, plain.Overflow)
	require.Equal(t, "1718", EncodeStickySessionValue(plain))

	overflow, err := ParseStickySessionValue("1718|o")
	require.NoError(t, err)
	require.Equal(t, int64(1718), overflow.AccountID)
	require.True(t, overflow.Overflow)
	require.Equal(t, "1718|o", EncodeStickySessionValue(overflow))
}

func TestPeerClearlyHealthier_LoadAndTTFT(t *testing.T) {
	stickyTTFT, peerTTFT := 8000, 2000
	require.True(t, peerClearlyHealthier(80, 40, nil, nil))
	require.False(t, peerClearlyHealthier(50, 40, nil, nil), "gap under 20 does not switch")
	require.False(t, peerClearlyHealthier(80, 100, nil, nil), "a full peer is not a load escape")
	require.True(t, peerClearlyHealthier(30, 30, &stickyTTFT, &peerTTFT))
	closeTTFT := 7500
	require.False(t, peerClearlyHealthier(30, 30, &stickyTTFT, &closeTTFT), "absolute gap under 1000ms stays")
}

func TestShouldEscapeDemotedStickyForHealthierPeer(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	enablePublicSite(runtime)
	require.True(t, cache.TryStartCooldown(context.Background(), 2, time.Now().Add(15*time.Minute), "fail", false))
	require.True(t, cache.TryStartCooldown(context.Background(), 1, time.Now().Add(15*time.Minute), "fail", false))

	sticky := &Account{ID: 2, Platform: PlatformAnthropic, UpstreamRateMultiplier: testRate(0.15)}
	peer := &Account{ID: 1, Platform: PlatformAnthropic, UpstreamRateMultiplier: testRate(1.0)}
	load := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 40},
		2: {AccountID: 2, LoadRate: 80},
	}
	require.True(t, shouldEscapeDemotedStickyForHealthierPeer(
		context.Background(), runtime, nil, 16, PlatformAnthropic, sticky, []*Account{sticky, peer}, load, nil,
	))

	tight := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 70},
		2: {AccountID: 2, LoadRate: 80},
	}
	require.False(t, shouldEscapeDemotedStickyForHealthierPeer(
		context.Background(), runtime, nil, 16, PlatformAnthropic, sticky, []*Account{sticky, peer}, tight, nil,
	), "equally bad demoted peers stay pinned")

	require.False(t, shouldEscapeDemotedStickyForHealthierPeer(
		context.Background(), runtime, testSmartLookup(PlatformAnthropic, 1, 2), 16, PlatformAnthropic, sticky, []*Account{sticky, peer}, load, nil,
	), "a closed pool does not use the public lag escape")
}

func TestShouldEscapeDemotedSticky_HealthyPinStays(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	enablePublicSite(runtime)
	require.True(t, cache.TryStartCooldown(context.Background(), 1, time.Now().Add(15*time.Minute), "fail", false))

	sticky := &Account{ID: 2, Platform: PlatformAnthropic}
	peer := &Account{ID: 1, Platform: PlatformAnthropic}
	load := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, LoadRate: 10},
		2: {AccountID: 2, LoadRate: 90},
	}
	require.False(t, shouldEscapeDemotedStickyForHealthierPeer(
		context.Background(), runtime, nil, 16, PlatformAnthropic, sticky, []*Account{sticky, peer}, load, nil,
	))
}

func TestIsBetterAccount_PriorityBeatsUpstreamRate(t *testing.T) {
	svc := &OpenAIGatewayService{}
	cheap := &Account{ID: 1, Priority: 50, UpstreamRateMultiplier: testRate(0.15)}
	expensive := &Account{ID: 2, Priority: 1, UpstreamRateMultiplier: testRate(1.0)}
	require.False(t, svc.isBetterAccount(cheap, expensive))
	require.True(t, svc.isBetterAccount(expensive, cheap))
}

func TestIsBetterGeminiAccount_PriorityBeatsUpstreamRate(t *testing.T) {
	svc := &GeminiMessagesCompatService{}
	cheap := &Account{ID: 1, Priority: 50, Type: AccountTypeAPIKey, UpstreamRateMultiplier: testRate(0.15)}
	expensive := &Account{ID: 2, Priority: 1, Type: AccountTypeOAuth, UpstreamRateMultiplier: testRate(1.0)}
	require.False(t, svc.isBetterGeminiAccount(cheap, expensive))
	require.True(t, svc.isBetterGeminiAccount(expensive, cheap))
}

func TestUnpooledHelpers_DoNotReadBillingRate(t *testing.T) {
	billing := 9.9
	upstream := 0.15
	a := &Account{ID: 1, RateMultiplier: &billing, UpstreamRateMultiplier: &upstream, Type: AccountTypeOAuth}
	b := &Account{ID: 2, RateMultiplier: testRate(0.01), UpstreamRateMultiplier: testRate(1.0), Type: AccountTypeOAuth}
	require.Equal(t, 9.9, a.BillingRateMultiplier())
	require.Equal(t, 0.15, a.EffectiveUpstreamRate())
	require.False(t, isBetterSchedulableAccount(a, b, false))
	require.False(t, isBetterSchedulableAccount(b, a, false))
	require.Equal(t, 9.9, a.BillingRateMultiplier())
	require.Equal(t, 0.01, b.BillingRateMultiplier())
}

func TestLookupEnabledSmartPolicy_UsesAccountPlatformNotGroup(t *testing.T) {
	lookup := testSmartLookup(PlatformAntigravity, 20)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	require.NotNil(t, lookupEnabledSmartPolicy(ctx, lookup, 16, PlatformAntigravity))
	require.Nil(t, lookupEnabledSmartPolicy(ctx, lookup, 16, PlatformAnthropic))
}
