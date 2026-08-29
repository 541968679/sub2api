//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testPublicAccount(id int64) *Account {
	return &Account{ID: id, Platform: PlatformAnthropic, UpstreamRateMultiplier: testRate(1.0)}
}

func enablePublicSite(runtime *PublicScheduleQualityService, extra ...func(*PublicScheduleQualitySettings)) {
	site := DefaultPublicScheduleQualitySettings()
	site.Enabled = true
	for _, apply := range extra {
		apply(site)
	}
	runtime.site = site
	runtime.siteAt = time.Now()
}

func TestPreferPublicScheduleAccounts_UnpooledPrefersHealthy(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	enablePublicSite(runtime)
	require.True(t, cache.TryStartCooldown(context.Background(), 2, time.Now().Add(15*time.Minute), "fail", false))

	healthy := testPublicAccount(1)
	cooling := testPublicAccount(2)
	out := preferPublicScheduleAccounts(context.Background(), runtime, nil, 16, PlatformAnthropic, []*Account{cooling, healthy})
	require.Equal(t, []*Account{healthy}, out)
}

func TestPreferPublicScheduleAccounts_AllCoolingStillPicks(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	enablePublicSite(runtime)
	require.True(t, cache.TryStartCooldown(context.Background(), 1, time.Now().Add(15*time.Minute), "fail", false))
	require.True(t, cache.TryStartCooldown(context.Background(), 2, time.Now().Add(15*time.Minute), "fail", false))

	a := testPublicAccount(1)
	b := testPublicAccount(2)
	in := []*Account{a, b}
	out := preferPublicScheduleAccounts(context.Background(), runtime, nil, 16, PlatformAnthropic, in)
	require.Equal(t, in, out)
}

func TestPreferPublicScheduleAccounts_DisabledIsNoOp(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	require.True(t, cache.TryStartCooldown(context.Background(), 2, time.Now().Add(15*time.Minute), "fail", false))

	healthy := testPublicAccount(1)
	cooling := testPublicAccount(2)
	in := []*Account{cooling, healthy}
	out := preferPublicScheduleAccounts(context.Background(), runtime, nil, 16, PlatformAnthropic, in)
	require.Equal(t, in, out)
}

func TestPreferPublicScheduleAccounts_PooledUserUnchanged(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	enablePublicSite(runtime)
	require.True(t, cache.TryStartCooldown(context.Background(), 2, time.Now().Add(15*time.Minute), "fail", false))

	healthy := testPublicAccount(1)
	cooling := testPublicAccount(2)
	in := []*Account{cooling, healthy}
	lookup := testSmartLookup(PlatformAnthropic, 1, 2)
	out := preferPublicScheduleAccounts(context.Background(), runtime, lookup, 16, PlatformAnthropic, in)
	require.Equal(t, in, out)
}

func TestPreferPublicScheduleAccounts_NilRuntimeNoOp(t *testing.T) {
	a := testPublicAccount(1)
	b := testPublicAccount(2)
	in := []*Account{a, b}
	require.Equal(t, in, preferPublicScheduleAccounts(context.Background(), nil, nil, 16, PlatformAnthropic, in))
}

func TestPublicScheduleEvalQuality_SameAsSmartSchedule(t *testing.T) {
	rate := 0.9
	p50 := 3000
	knobs := QualityEvalKnobs{SuccessRate: &rate, SuccessN: 3, TTFTMax: &p50, LatencyN: 3}
	failLive := &PairQualityLive{OK: []bool{false, false, false}, OKCount: 3}
	RecomputePairQuality(failLive)
	require.Equal(t, LatencyEvalFail, EvalQuality(failLive, knobs).State)

	passLive := &PairQualityLive{
		OK:     []bool{true, true, true},
		TTFTMs: []int{100, 120, 110},
	}
	RecomputePairQuality(passLive)
	require.Equal(t, LatencyEvalPass, EvalQuality(passLive, knobs).State)
}

func TestPublicScheduleObserve_StartsCooldownAndDoesNotExtend(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	p50 := 50
	rate := 1.0
	enablePublicSite(runtime, func(site *PublicScheduleQualitySettings) {
		site.TTFTWindowN = 2
		site.SuccessWindowN = 2
		site.QualityMaxP50TTFTMs = &p50
		site.QualityMinSuccessRate = &rate
		site.CooldownMinutes = 15
	})

	obs := AccountQualityObservation{AccountID: 7, Success: false}
	runtime.ObserveCompletion(context.Background(), obs)
	runtime.ObserveCompletion(context.Background(), obs)
	first := cache.GetState(context.Background(), 7)
	require.NotNil(t, first)
	require.Equal(t, PublicScheduleStateCooling, first.Normalized(time.Now()))
	until := first.Until

	time.Sleep(15 * time.Millisecond)
	runtime.ObserveCompletion(context.Background(), obs)
	second := cache.GetState(context.Background(), 7)
	require.Equal(t, until.Unix(), second.Until.Unix())
}

func TestPublicScheduleObserve_DisabledDoesNotCool(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	p50 := 50
	runtime.site = &PublicScheduleQualitySettings{
		Enabled:             false,
		TTFTWindowN:         1,
		SuccessWindowN:      1,
		QualityMaxP50TTFTMs: &p50,
		CooldownMinutes:     15,
	}
	runtime.siteAt = time.Now()
	runtime.ObserveCompletion(context.Background(), AccountQualityObservation{AccountID: 8, Success: false})
	require.Nil(t, cache.GetState(context.Background(), 8))
	require.NotNil(t, cache.GetWindow(context.Background(), 8), "disabled accounts still ingest the public FIFO")
}

func TestPublicScheduleSoftCooldown_PassReturnsSelectable(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	rate := 0.5
	enablePublicSite(runtime, func(site *PublicScheduleQualitySettings) {
		site.TTFTWindowN = 1
		site.SuccessWindowN = 1
		site.QualityMinSuccessRate = &rate
		site.CooldownMinutes = 15
		site.SoftCooldown = true
	})
	require.True(t, cache.TryStartCooldown(context.Background(), 9, time.Now().Add(15*time.Minute), "manual", true))

	ok := true
	ms := 10
	runtime.ObserveCompletion(context.Background(), AccountQualityObservation{AccountID: 9, Success: ok, FirstTokenMs: &ms})
	st := cache.GetState(context.Background(), 9)
	require.True(t, st == nil || st.Normalized(time.Now()) == PublicScheduleStateSelectable)
}

func TestPublicScheduleCooldownExpiry_BecomesProbing(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	cache.SetState(context.Background(), 4, &PublicScheduleRuntimeState{
		State: PublicScheduleStateCooling,
		Until: time.Now().Add(-time.Second),
	})
	st := runtime.effectiveState(context.Background(), 4)
	require.Equal(t, PublicScheduleStateProbing, st.Normalized(time.Now()))
}

func TestPublicScheduleStickyEscape_DemotedWithHealthyPeer(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	runtime := NewPublicScheduleQualityService(cache, nil, nil)
	enablePublicSite(runtime)
	require.True(t, cache.TryStartCooldown(context.Background(), 2, time.Now().Add(15*time.Minute), "fail", false))
	sticky := testPublicAccount(2)
	healthy := testPublicAccount(1)
	require.True(t, shouldEscapeSessionStickyForPublicQuality(
		context.Background(), runtime, nil, 16, PlatformAnthropic, sticky, []*Account{healthy, sticky},
	))
	require.False(t, shouldEscapeSessionStickyForPublicQuality(
		context.Background(), runtime, testSmartLookup(PlatformAnthropic, 1, 2), 16, PlatformAnthropic, sticky, []*Account{healthy, sticky},
	))
}

func TestResolvePublicScheduleQuality_UsesSiteOnly(t *testing.T) {
	site := DefaultPublicScheduleQualitySettings()
	site.Enabled = true
	n := 7
	resolved := ResolvePublicScheduleQuality(*site, PublicScheduleQualityOverlay{Enabled: false, TTFTWindowN: &n, SuccessWindowN: &n})
	require.True(t, resolved.Enabled)
	require.Equal(t, site.TTFTWindowN, resolved.TTFTWindowN)
	require.Equal(t, site.SuccessWindowN, resolved.SuccessWindowN)
	require.Equal(t, DefaultPublicScheduleWindowN, site.TTFTWindowN)
}

func TestPublicScheduleTryStartCooldown_DoesNotExtend(t *testing.T) {
	cache := NewMemoryPublicScheduleQualityCache()
	until := time.Now().Add(10 * time.Minute)
	require.True(t, cache.TryStartCooldown(context.Background(), 3, until, "a", false))
	require.False(t, cache.TryStartCooldown(context.Background(), 3, until.Add(time.Hour), "b", false))
	st := cache.GetState(context.Background(), 3)
	require.Equal(t, "a", st.Reason)
}

func TestProjectPublicScheduleLive_UsesResolvedN(t *testing.T) {
	live := &PairQualityLive{}
	for i := 0; i < 20; i++ {
		ok := i%2 == 0
		ms := 100 + i
		live = ingestPublicScheduleSample(live, ok, &ms, nil)
	}
	projected := projectPublicScheduleLive(live, 5, 5)
	require.Equal(t, 5, projected.OKCount)
	require.Greater(t, live.OKCount, projected.OKCount)
}
