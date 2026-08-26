//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func syncOKLive(n, okCount int) *PairQualityLive {
	live := &PairQualityLive{N: n}
	for i := 0; i < okCount; i++ {
		live = ApplyPairQualityIngest(live, n, true, nil)
	}
	return live
}

func mixedAndLive(n, p50Ms, successes int) *PairQualityLive {
	live := &PairQualityLive{N: n}
	for i := 0; i < n; i++ {
		ok := i < successes
		if ok {
			live = ApplyPairQualityIngest(live, n, true, intPtr(p50Ms))
		} else {
			live = ApplyPairQualityIngest(live, n, false, nil)
		}
	}
	return live
}

func probePolicy(accountID int64, n int, p50 *int, rate *float64, and bool) *SmartSchedulePlatformPolicy {
	policy := enabledSmartPolicy(accountID, 0, p50)
	policy.QualityWindowSamples = intPtr(n)
	policy.QualityMinSuccessRate = rate
	if and {
		policy.QualityCondition = strPtr(QualityHardCloseConditionAnd)
	}
	return policy
}

func TestProbeInFlightCap(t *testing.T) {
	t.Parallel()
	require.Equal(t, 10, ProbeInFlightCap(0, 0))
	require.Equal(t, 10, ProbeInFlightCap(10, 0))
	require.Equal(t, 5, ProbeInFlightCap(10, 5))
	require.Equal(t, 10, ProbeInFlightCap(10, 20))
	require.Equal(t, 1, ProbeInFlightCap(1, 0))
	require.Equal(t, 100, ProbeInFlightCap(200, 0))
	require.NotEqual(t, 999, ProbeInFlightCap(10, 0))
}

func TestProbeInFlightCap_FollowSuccessN(t *testing.T) {
	t.Parallel()
	follow := &SmartSchedulePlatformPolicy{
		QualityMinTTFTSamples:    intPtr(4),
		QualityMinSuccessSamples: intPtr(20),
	}
	require.Equal(t, 20, follow.ProbeDesiredConcurrency())
	require.Equal(t, 5, follow.ProbeInFlightCap(5))
	require.Equal(t, 20, follow.ProbeInFlightCap(0))
}

func TestProbeInFlightCap_FollowNAndCustom(t *testing.T) {
	t.Parallel()
	follow := &SmartSchedulePlatformPolicy{QualityWindowSamples: intPtr(10)}
	require.Equal(t, 4, follow.ProbeInFlightCap(4), "follow_n + cap")
	require.Equal(t, 10, follow.ProbeInFlightCap(0), "follow_n no cap")

	custom2 := &SmartSchedulePlatformPolicy{
		QualityWindowSamples: intPtr(10),
		ProbeConcurrencyMode: ProbeConcurrencyModeCustom,
		ProbeConcurrency:     intPtr(2),
	}
	require.Equal(t, 2, custom2.ProbeInFlightCap(5), "custom 2 with cap 5 → 2")

	custom10 := &SmartSchedulePlatformPolicy{
		QualityWindowSamples: intPtr(10),
		ProbeConcurrencyMode: ProbeConcurrencyModeCustom,
		ProbeConcurrency:     intPtr(10),
	}
	require.Equal(t, 3, custom10.ProbeInFlightCap(3), "custom 10 with cap 3 → 3")
}

func TestNormalizeSmartScheduleWrite_ProbeConcurrency(t *testing.T) {
	t.Parallel()
	omitted, err := normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{CooldownMinutes: 15})
	require.NoError(t, err)
	require.Equal(t, ProbeConcurrencyModeFollowN, omitted.ProbeConcurrencyMode, "default omit → follow_n")
	require.Nil(t, omitted.ProbeConcurrency)

	emptyMode, err := normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		CooldownMinutes:      15,
		ProbeConcurrencyMode: "",
		ProbeConcurrency:     intPtr(7),
	})
	require.NoError(t, err)
	require.Equal(t, ProbeConcurrencyModeFollowN, emptyMode.ProbeConcurrencyMode)
	require.Nil(t, emptyMode.ProbeConcurrency, "follow_n ignores custom")

	got, err := normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		CooldownMinutes:      15,
		ProbeConcurrencyMode: ProbeConcurrencyModeCustom,
		ProbeConcurrency:     intPtr(2),
	})
	require.NoError(t, err)
	require.Equal(t, ProbeConcurrencyModeCustom, got.ProbeConcurrencyMode)
	require.Equal(t, 2, *got.ProbeConcurrency)

	_, err = normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		CooldownMinutes:      15,
		ProbeConcurrencyMode: ProbeConcurrencyModeCustom,
		ProbeConcurrency:     intPtr(0),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_INVALID_QUALITY")

	_, err = normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		CooldownMinutes:      15,
		ProbeConcurrencyMode: ProbeConcurrencyModeCustom,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_INVALID_QUALITY")

	_, err = normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		CooldownMinutes:      15,
		ProbeConcurrencyMode: ProbeConcurrencyModeCustom,
		ProbeConcurrency:     intPtr(101),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_INVALID_QUALITY")

	_, err = normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		CooldownMinutes:      15,
		ProbeConcurrencyMode: "follow_window",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_INVALID_QUALITY")
}

func TestPairQualityProbeGraduates_WokOnlySync(t *testing.T) {
	t.Parallel()
	rate := 0.8
	policy := probePolicy(7, 3, nil, &rate, false)
	require.False(t, pairQualityProbeGraduates(syncOKLive(3, 2), policy), "W_ok < N must not graduate")
	require.True(t, pairQualityProbeGraduates(syncOKLive(3, 3), policy), "full W_ok without latency gates must graduate")

	p50 := 50
	withTTFT := probePolicy(7, 3, &p50, &rate, false)
	require.True(t, pairQualityProbeGraduates(syncOKLive(3, 3), withTTFT), "257: TTFT underfull still graduates")
	require.False(t, pairQualityProbeGraduates(syncOKLive(3, 3), withProbeLatencyV2(withTTFT)), "v2: empty W_ttft stays pending")
}

func TestPairQualityProbeAndMixed_OnlyInProbe(t *testing.T) {
	t.Parallel()
	p50 := 100
	rate := 0.9
	policy := probePolicy(7, 3, &p50, &rate, true)
	live := mixedAndLive(3, 400, 3)
	require.Equal(t, 3, live.OKCount)
	require.Equal(t, 3, live.TTFTCount)
	require.False(t, pairQualityProbeBlocks(live, policy), "and: success passes so standard and must not cool")
	require.False(t, pairQualityProbeGraduates(live, policy), "full W_ttft p50 fail must not graduate")
	require.True(t, pairQualityProbeAndMixed(live, policy))

	orPolicy := probePolicy(7, 3, &p50, &rate, false)
	require.False(t, pairQualityProbeAndMixed(live, orPolicy), "or is not the mixed override")
	require.True(t, pairQualityProbeBlocks(live, orPolicy), "or still cools on the failed TTFT window in probe")
}

func TestEvaluateSmartSchedule_ProbeGraduateKeepsWindows(t *testing.T) {
	t.Parallel()
	rate := 0.8
	policy := probePolicy(7, 3, nil, &rate, false)
	live := syncOKLive(3, 3)
	lookup := &memorySmartLookup{
		bundle:  smartBundle(PlatformAnthropic, policy),
		pair:    map[string]*PairQualityLive{smartPairKey(7, 16): live},
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	require.True(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, live, time.Now().UTC(), nil))
	require.Equal(t, 1, lookup.graduated)
	require.False(t, lookup.IsProbing(context.Background(), 7, 16, "openai"))
	require.Equal(t, 3, lookup.GetPairQuality(context.Background(), 7, 16, "openai").OKCount, "graduate must keep windows")
	require.Equal(t, 0, lookup.startCalls)
}

func TestEvaluateSmartSchedule_ProbeAndMixedCools(t *testing.T) {
	t.Parallel()
	p50 := 100
	rate := 0.9
	policy := probePolicy(7, 3, &p50, &rate, true)
	live := mixedAndLive(3, 400, 3)
	lookup := &memorySmartLookup{
		bundle:  smartBundle(PlatformAnthropic, policy),
		pair:    map[string]*PairQualityLive{smartPairKey(7, 16): live},
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	require.False(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, live, time.Now().UTC(), nil))
	require.Equal(t, 1, lookup.startCalls)
	require.False(t, lookup.IsProbing(context.Background(), 7, 16, "openai"))
	require.Equal(t, 0, lookup.graduated)
}

func TestEvaluateSmartSchedule_SelectableMixedDoesNotUseOverride(t *testing.T) {
	t.Parallel()
	p50 := 100
	rate := 0.9
	policy := probePolicy(7, 3, &p50, &rate, true)
	live := mixedAndLive(3, 400, 3)
	lookup := &memorySmartLookup{
		bundle: smartBundle(PlatformAnthropic, policy),
		pair:   map[string]*PairQualityLive{smartPairKey(7, 16): live},
	}
	require.True(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, live, time.Now().UTC(), nil))
	require.Equal(t, 0, lookup.startCalls)
	require.False(t, lookup.IsProbing(context.Background(), 7, 16, "openai"), "no mark = not probing / no backfill")
}

func TestEvaluateSmartSchedule_NoTrafficStaysProbing(t *testing.T) {
	t.Parallel()
	policy := probePolicy(7, 3, intPtr(50), nil, false)
	lookup := &memorySmartLookup{
		bundle:  smartBundle(PlatformAnthropic, policy),
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	require.True(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, syncOKLive(3, 1), time.Now().UTC(), nil))
	require.True(t, lookup.IsProbing(context.Background(), 7, 16, "openai"))
	require.Equal(t, 0, lookup.graduated)
	require.Equal(t, 0, lookup.startCalls)
}

// expiryAwareLookup mirrors expirePairCooldown: delete cooldown, 257 zero, then MarkProbing.
type expiryAwareLookup struct {
	memorySmartLookup
	zeros []string
}

func (m *expiryAwareLookup) CooldownActive(ctx context.Context, accountID, userID int64, platform string, now time.Time) bool {
	if m == nil || len(m.cooldownUntil) == 0 {
		return false
	}
	key := smartPairKey(accountID, userID)
	until := m.cooldownUntil[key]
	if until <= 0 {
		return false
	}
	if until > now.Unix() {
		return true
	}
	delete(m.cooldownUntil, key)
	if m.IsPinned(ctx, accountID, userID, platform) {
		return false
	}
	var policy *SmartSchedulePlatformPolicy
	if m.bundle != nil {
		policy = m.bundle.Policy(platform)
	}
	if policy == nil || !policy.ProbeLatencyV2 {
		n := DefaultSmartScheduleWindowN
		if policy != nil {
			n = policy.TTFTWindowN()
		}
		if m.pair == nil {
			m.pair = map[string]*PairQualityLive{}
		}
		m.pair[key] = ZeroPairQualityLive(n)
		m.zeros = append(m.zeros, PairQualityEventExpiryZero)
	}
	m.MarkProbing(ctx, accountID, userID, platform)
	m.ClearPairResume(ctx, accountID, userID, platform)
	return false
}

func TestEvaluateSmartSchedule_EmptyWindowAfterExpiry257NoCooldown(t *testing.T) {
	t.Parallel()
	p50 := 50
	policy := probePolicy(7, 3, &p50, nil, false)
	require.False(t, policy.ProbeLatencyV2)
	empty := ZeroPairQualityLive(3)
	require.Equal(t, 0, empty.TTFTCount)
	require.False(t, pairQualityProbeBlocks(empty, policy), "TTFTCount < N must not legacy-p50 block")
	lookup := &memorySmartLookup{
		bundle:  smartBundle(PlatformAnthropic, policy),
		pair:    map[string]*PairQualityLive{smartPairKey(7, 16): empty},
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	require.True(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, empty, time.Now().UTC(), nil))
	require.Equal(t, 0, lookup.startCalls, "empty window after expiry must not cooldown_start")
	require.Equal(t, 0, lookup.graduated)
	require.True(t, lookup.IsProbing(context.Background(), 7, 16, "openai"))
	pass, state := pairQualityProbeLatencyPass(empty, policy)
	require.False(t, pass)
	require.Equal(t, LatencyEvalPending, state)
	require.True(t, pairQualityProbeGraduates(syncOKLive(3, 3), policy), "257: TTFTCount < N still graduates")
}

func TestAdmitsScheduleUser_ExpiryZeroV2OffDoesNotRecooldownStaleP50(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	p50 := 50
	policy := probePolicy(7, 3, &p50, nil, false)
	require.False(t, policy.ProbeLatencyV2)
	stale := mixedAndLive(3, 400, 3)
	require.True(t, pairQualityProbeBlocks(stale, policy), "stale full p50 would re-cool if kept")

	now := time.Now().UTC()
	lookup := &expiryAwareLookup{memorySmartLookup: memorySmartLookup{
		bundle:        smartBundle(PlatformAnthropic, policy),
		pair:          map[string]*PairQualityLive{smartPairKey(7, 16): stale},
		cooldownUntil: map[string]int64{smartPairKey(7, 16): now.Add(-time.Second).Unix()},
	}}
	acc := &Account{ID: 7, Platform: PlatformAnthropic}
	require.True(t, admitsScheduleUser(ctx, acc, nil, lookup))
	require.Equal(t, 0, lookup.startCalls, "same tick must not cooldown_start with stale p50")
	require.True(t, lookup.IsProbing(ctx, 7, 16, PlatformAnthropic))
	require.Contains(t, lookup.zeros, PairQualityEventExpiryZero)
	cleared := lookup.GetPairQuality(ctx, 7, 16, PlatformAnthropic)
	require.Equal(t, 0, cleared.TTFTCount)
	require.Equal(t, 0, cleared.OKCount)
	require.True(t, pairQualityProbeGraduates(syncOKLive(3, 3), policy), "257: TTFTCount < N still graduates")
}

func TestAdmitsScheduleUser_ExpiryV2KeepsWindows(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	p50 := 50
	policy := withProbeLatencyV2(probePolicy(7, 3, &p50, nil, false))
	require.True(t, policy.ProbeLatencyV2)
	kept := mixedAndLive(3, 10, 3)
	now := time.Now().UTC()
	lookup := &expiryAwareLookup{memorySmartLookup: memorySmartLookup{
		bundle:        smartBundle(PlatformAnthropic, policy),
		pair:          map[string]*PairQualityLive{smartPairKey(7, 16): kept},
		cooldownUntil: map[string]int64{smartPairKey(7, 16): now.Add(-time.Second).Unix()},
	}}
	require.False(t, lookup.CooldownActive(ctx, 7, 16, PlatformAnthropic, now.Add(time.Second)))
	require.NotContains(t, lookup.zeros, PairQualityEventExpiryZero)
	live := lookup.GetPairQuality(ctx, 7, 16, PlatformAnthropic)
	require.Equal(t, 3, live.OKCount, "expiry must not zero pair windows")
	require.Equal(t, 3, live.TTFTCount)
	require.True(t, lookup.IsProbing(ctx, 7, 16, PlatformAnthropic), "expiry still enters probing")
	acc := &Account{ID: 7, Platform: PlatformAnthropic}
	require.True(t, admitsScheduleUser(ctx, acc, nil, lookup))
	require.Equal(t, 0, lookup.startCalls, "kept fast window must not cooldown_start")
	require.Equal(t, 3, lookup.GetPairQuality(ctx, 7, 16, PlatformAnthropic).OKCount)
}

func TestAdmitsScheduleUser_ProbingVsSelectable(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := &Account{ID: 7, Platform: PlatformAnthropic}
	rate := 0.8
	policy := probePolicy(7, 3, nil, &rate, false)
	live := syncOKLive(3, 3)

	t.Run("probing graduates then selectable", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, policy),
			pair:    map[string]*PairQualityLive{smartPairKey(7, 16): live},
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		require.True(t, admitsScheduleUser(ctx, acc, nil, lookup))
		require.Equal(t, 1, lookup.graduated)
		require.False(t, lookup.IsProbing(ctx, 7, 16, "openai"))
		require.True(t, admitsScheduleUser(ctx, acc, nil, lookup))
		require.Equal(t, 1, lookup.graduated)
	})

	t.Run("no probe mark is selectable and does not graduate", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle: smartBundle(PlatformAnthropic, policy),
			pair:   map[string]*PairQualityLive{smartPairKey(7, 16): live},
		}
		require.True(t, admitsScheduleUser(ctx, acc, nil, lookup))
		require.Equal(t, 0, lookup.graduated)
	})

	t.Run("leftover Track A resume during probe still graduates and stays on Track A", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, policy),
			pair:    map[string]*PairQualityLive{smartPairKey(7, 16): live},
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		quality := &liveQualityCacheStub{}
		require.NoError(t, quality.MarkUserResume(context.Background(), 7, 16))
		require.True(t, admitsScheduleUser(ctx, acc, quality, lookup))
		require.Equal(t, 1, lookup.graduated, "N successes in probe must graduate even with leftover Track A u:/w:")
		require.False(t, lookup.IsProbing(ctx, 7, 16, "openai"))
		require.True(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()), "smart-schedule must not clear Track A resume")
	})

	t.Run("leftover pair resume during probe still graduates", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle:      smartBundle(PlatformAnthropic, policy),
			pair:        map[string]*PairQualityLive{smartPairKey(7, 16): live},
			probing:     map[string]bool{smartPairKey(7, 16): true},
			resumeUntil: map[string]int64{smartPairPlatformKey(7, 16, PlatformAnthropic): time.Now().UTC().Add(20 * time.Minute).Unix()},
		}
		require.True(t, admitsScheduleUser(ctx, acc, nil, lookup))
		require.Equal(t, 1, lookup.graduated)
		require.False(t, lookup.PairResumeActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()))
	})

	t.Run("resume grace skips evaluate when not probing", func(t *testing.T) {
		t.Parallel()
		p50 := 50
		lookup := &memorySmartLookup{
			bundle:      smartBundle(PlatformAnthropic, probePolicy(7, 3, &p50, nil, false)),
			pair:        map[string]*PairQualityLive{smartPairKey(7, 16): mixedAndLive(3, 400, 3)},
			resumeUntil: map[string]int64{smartPairPlatformKey(7, 16, PlatformAnthropic): time.Now().UTC().Add(20 * time.Minute).Unix()},
		}
		quality := &liveQualityCacheStub{}
		require.NoError(t, quality.MarkUserResume(context.Background(), 7, 16))
		require.True(t, admitsScheduleUser(ctx, acc, quality, lookup))
		require.Equal(t, 0, lookup.startCalls)
		require.Equal(t, 0, lookup.graduated)
	})
}

func TestObservePairCompletion_ProbeGraduateAndMixed(t *testing.T) {
	t.Parallel()
	rate := 0.8
	policy := probePolicy(7, 3, nil, &rate, false)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		live:    map[string]*PairQualityLive{},
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)

	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true})
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true})
	require.Equal(t, 0, cache.graduated, "W_ok < N stays probing")
	require.True(t, cache.IsProbing(context.Background(), 7, 16, "openai"))

	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true})
	require.Equal(t, 1, cache.graduated)
	require.False(t, cache.IsProbing(context.Background(), 7, 16, "openai"))
	require.Equal(t, 3, cache.GetPairQuality(context.Background(), 7, 16, "openai").OKCount)
	require.Equal(t, 0, cache.starts)

	p50 := 100
	andPolicy := probePolicy(7, 3, &p50, floatPtr(0.9), true)
	seed := ApplyPairQualityIngest(nil, 3, true, intPtr(400))
	seed = ApplyPairQualityIngest(seed, 3, true, intPtr(400))
	mixed := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, andPolicy),
		live:    map[string]*PairQualityLive{smartPairKey(7, 16): seed},
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	mixedSvc := NewUserSmartScheduleService(nil, mixed, nil, nil, nil)
	mixedSvc.ObservePairCompletion(context.Background(), PairQualityObservation{
		AccountID:    7,
		UserID:       16,
		Success:      true,
		FirstTokenMs: intPtr(400),
	})
	require.Equal(t, 1, mixed.starts, "and mixed (success pass + p50 fail) must cool in probe")
	require.False(t, mixed.IsProbing(context.Background(), 7, 16, "openai"))
	require.Equal(t, 0, mixed.graduated)
	require.Equal(t, 3, mixed.GetPairQuality(context.Background(), 7, 16, "openai").OKCount)
	require.Equal(t, 3, mixed.GetPairQuality(context.Background(), 7, 16, "openai").TTFTCount)
}

func TestObservePairCompletion_ProbeResumeGraceDoesNotBlockGraduate(t *testing.T) {
	t.Parallel()
	rate := 0.8
	policy := probePolicy(7, 3, nil, &rate, false)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		live:    map[string]*PairQualityLive{},
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	quality := &liveQualityCacheStub{}
	require.NoError(t, quality.MarkUserResume(context.Background(), 7, 16))
	require.NoError(t, cache.MarkPairResume(context.Background(), 7, 16, PlatformAnthropic))
	svc := NewUserSmartScheduleService(nil, cache, nil, quality, nil)

	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true})
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true})
	require.Equal(t, 0, cache.graduated, "W_ok < N stays probing")
	require.True(t, cache.IsProbing(context.Background(), 7, 16, "openai"))

	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true})
	require.Equal(t, 1, cache.graduated, "N successes in probe must graduate even with leftover pair resume")
	require.False(t, cache.IsProbing(context.Background(), 7, 16, "openai"), "N successes in probe, still probing")
	require.Equal(t, 3, cache.GetPairQuality(context.Background(), 7, 16, "openai").OKCount)
	require.False(t, cache.PairResumeActive(context.Background(), 7, 16, PlatformAnthropic, time.Now().UTC()))
	require.True(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()), "probe evaluate must not clear Track A resume")
}

func TestUserSmartScheduleService_SetPairAdmissionProbing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	quality := &liveQualityCacheStub{}
	require.NoError(t, quality.MarkUserResume(ctx, 7, 16))
	cache := &admissionCacheRecorder{
		bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 4, nil)),
		probing: map[string]bool{
			smartPairKey(7, 16): false,
		},
	}
	svc := NewUserSmartScheduleService(nil, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, quality, nil)

	paused, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionPaused)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionPaused, paused.State)
	require.False(t, paused.Probing)
	require.Equal(t, 0, cache.markedProbe, "pause must not write probing")

	omitted, err := svc.SetPairAdmission(ctx, 7, 16, "")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionResumed, omitted.State, "omitted state from pause is 豁免期, not probing")
	require.False(t, omitted.Probing)
	require.Equal(t, 0, cache.markedProbe)

	selectable, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionSelectable)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionSelectable, selectable.State)
	require.False(t, selectable.Probing)
	require.True(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()), "selectable must not clear Track A resume")
	require.False(t, cache.PairResumeActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()))
	require.Contains(t, cache.zeros, PairQualityEventSelectable)

	probe, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionProbing)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionProbing, probe.State)
	require.True(t, probe.Probing)
	require.NotNil(t, probe.ProbeCap)
	require.Equal(t, 4, *probe.ProbeCap, "min(N=10, cap=4)")
	require.Equal(t, 1, cache.markedProbe)
	require.True(t, cache.IsProbing(ctx, 7, 16, "openai"))
	require.False(t, cache.PairResumeActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()), "enter probing clears pair resume")
	require.True(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()), "enter probing must not clear Track A resume")
	require.NotContains(t, cache.zeros, PairQualityEventProbeEnter, "probing must not zero pair windows")

	again, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionSelectable)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionSelectable, again.State)
	require.False(t, cache.IsProbing(ctx, 7, 16, "openai"))
	require.GreaterOrEqual(t, cache.clearedProbe, 1)
}

func TestUserSmartScheduleService_GetHydratesProbingAndProbeCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := enabledSmartPolicy(7, 4, nil)
	policy.QualityWindowSamples = intPtr(10)
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, policy)}
	cache := &admissionCacheRecorder{
		bundle:  repo.bundle,
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)

	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	member := view.Platforms[PlatformAnthropic].Accounts[0]
	require.True(t, member.Probing)
	require.NotNil(t, member.ProbeCap)
	require.Equal(t, 4, *member.ProbeCap, "GET probe_cap is min(N=10, cap=4)")
	require.Equal(t, ProbeConcurrencyModeFollowN, view.Platforms[PlatformAnthropic].ProbeConcurrencyMode)
	require.Nil(t, view.Platforms[PlatformAnthropic].ProbeConcurrency)

	customPolicy := enabledSmartPolicy(7, 5, nil)
	customPolicy.QualityWindowSamples = intPtr(10)
	customPolicy.ProbeConcurrencyMode = ProbeConcurrencyModeCustom
	customPolicy.ProbeConcurrency = intPtr(2)
	customRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, customPolicy)}
	customCache := &admissionCacheRecorder{
		bundle:  customRepo.bundle,
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	customSvc := NewUserSmartScheduleService(customRepo, customCache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)
	customView, err := customSvc.Get(ctx, 16)
	require.NoError(t, err)
	require.Equal(t, ProbeConcurrencyModeCustom, customView.Platforms[PlatformAnthropic].ProbeConcurrencyMode)
	require.Equal(t, 2, *customView.Platforms[PlatformAnthropic].ProbeConcurrency)
	require.Equal(t, 2, *customView.Platforms[PlatformAnthropic].Accounts[0].ProbeCap, "custom 2 with cap 5 → 2")

	cache.ClearProbing(ctx, 7, 16, "openai")
	unmarked, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	require.False(t, unmarked.Platforms[PlatformAnthropic].Accounts[0].Probing, "no mark = not probing / no backfill")
	require.Nil(t, unmarked.Platforms[PlatformAnthropic].Accounts[0].ProbeCap)

	pausedPolicy := enabledSmartPolicy(7, 4, nil)
	pausedPolicy.Paused = map[int64]struct{}{7: {}}
	pausedRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, pausedPolicy)}
	pausedCache := &admissionCacheRecorder{
		bundle:  pausedRepo.bundle,
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	pausedSvc := NewUserSmartScheduleService(pausedRepo, pausedCache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)
	pausedView, err := pausedSvc.Get(ctx, 16)
	require.NoError(t, err)
	require.True(t, pausedView.Platforms[PlatformAnthropic].Accounts[0].Paused)
	require.False(t, pausedView.Platforms[PlatformAnthropic].Accounts[0].Probing, "paused ignores leftover probe bits")
	require.Nil(t, pausedView.Platforms[PlatformAnthropic].Accounts[0].ProbeCap)
}

func TestUserSmartScheduleService_PauseDoesNotAutoProbe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cache := &admissionCacheRecorder{
		bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil)),
	}
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, &liveQualityCacheStub{}, nil)

	_, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionPaused)
	require.NoError(t, err)
	require.True(t, repo.bundle.Policies[PlatformAnthropic].IsPaused(7))
	require.Equal(t, 0, cache.markedProbe)
	require.False(t, cache.IsProbing(ctx, 7, 16, "openai"))

	explicit, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionProbing)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionProbing, explicit.State)
	require.False(t, repo.bundle.Policies[PlatformAnthropic].IsPaused(7))
	require.Equal(t, 1, cache.markedProbe)
}

func floatPtr(v float64) *float64 { return &v }

type cooldownReasonLookup struct {
	memorySmartLookup
	lastReason string
}

func (m *cooldownReasonLookup) StartCooldownWithReason(_ context.Context, accountID, userID int64, platform string, minutes int, now time.Time, reason string) {
	m.lastReason = reason
	m.memorySmartLookup.StartCooldownWithReason(context.Background(), accountID, userID, platform, minutes, now, reason)
}

func latencyGatePolicy(accountID int64, probeNTTFT int) *SmartSchedulePlatformPolicy {
	p50 := latencyGateMs
	policy := enabledSmartPolicy(accountID, 0, &p50)
	policy.QualityMinTTFTSamples = intPtr(probeNTTFT)
	policy.QualityMinSuccessSamples = intPtr(probeNTTFT)
	policy.QualityCondition = strPtr(QualityHardCloseConditionOr)
	return policy
}

func schedCompositePolicy(accountID int64, probeNTTFT int) *SmartSchedulePlatformPolicy {
	return withSchedComposite(latencyGatePolicy(accountID, probeNTTFT))
}

func TestEvaluateSmartSchedule_CooldownReasonPhaseAndBranch(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()

	t.Run("probe_K_branch", func(t *testing.T) {
		t.Parallel()
		policy := withProbeLatencyV2(latencyGatePolicy(7, latencyProbeN))
		live := buildLiveFromTTFTObservations(latencyProbeN, latencyProbeN, latencyScatterSlow(2, 3, latencyFastMs, latencySlowMs))
		lookup := &cooldownReasonLookup{
			memorySmartLookup: memorySmartLookup{
				bundle:  smartBundle(PlatformAnthropic, policy),
				pair:    map[string]*PairQualityLive{smartPairKey(7, 16): live},
				probing: map[string]bool{smartPairKey(7, 16): true},
			},
		}
		require.False(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, live, now, nil))
		require.Equal(t, 1, lookup.startCalls)
		require.Contains(t, lookup.lastReason, CooldownPhaseProbe)
		require.Contains(t, lookup.lastReason, "超标K")
	})

	t.Run("selectable_C_branch", func(t *testing.T) {
		t.Parallel()
		policy := schedCompositePolicy(7, latencySchedN)
		live := buildLiveFromTTFTObservations(latencySchedN, latencySchedN, latencyTail(17, 3, latencyFastMs, latencySlowMs))
		lookup := &cooldownReasonLookup{
			memorySmartLookup: memorySmartLookup{
				bundle: smartBundle(PlatformAnthropic, policy),
				pair:   map[string]*PairQualityLive{smartPairKey(7, 16): live},
			},
		}
		require.False(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, live, now, nil))
		require.Equal(t, 1, lookup.startCalls)
		require.Contains(t, lookup.lastReason, CooldownPhaseSelectable)
		require.Contains(t, lookup.lastReason, "连续C")
	})
}

func TestEvaluateSmartSchedule_ProbeHoldNoCooldown(t *testing.T) {
	t.Parallel()
	durGate := latencyDurGateMs
	p50 := latencyGateMs
	policy := withProbeLatencyV2(enabledSmartPolicy(7, 0, &p50))
	policy.QualityMaxP50DurationMs = &durGate
	policy.QualityMinTTFTSamples = intPtr(latencyProbeN)
	policy.QualityMinSuccessSamples = intPtr(latencyProbeN)
	policy.QualityCondition = strPtr(QualityHardCloseConditionOr)

	var live *PairQualityLive
	for i := 0; i < latencyProbeN; i++ {
		live = ApplyPairQualityIngestWindows(live, latencyProbeN, latencyProbeN, true, intPtr(latencyFastMs), nil)
	}
	for _, dur := range []int{latencyDurSlowMs, latencyFastMs * 50, latencyDurSlowMs} {
		live = ApplyPairQualityIngestWindows(live, latencyProbeN, latencyProbeN, true, nil, intPtr(dur))
	}

	lookup := &cooldownReasonLookup{
		memorySmartLookup: memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, policy),
			pair:    map[string]*PairQualityLive{smartPairKey(7, 16): live},
			probing: map[string]bool{smartPairKey(7, 16): true},
		},
	}
	pass, state := pairQualityProbeLatencyPass(live, policy)
	require.False(t, pass, "hold is not pass")
	require.Equal(t, LatencyEvalHold, state)
	require.True(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, live, time.Now().UTC(), nil))
	require.Equal(t, 0, lookup.startCalls, "hold must not start cooldown")
	require.True(t, lookup.IsProbing(context.Background(), 7, 16, "openai"))
}

func TestEvaluateSmartSchedule_QAFirstGateBlocks(t *testing.T) {
	t.Parallel()
	policy := withProbeLatencyV2(latencyGatePolicy(7, latencyProbeN))
	live := buildLiveFromTTFTObservations(latencyProbeN, latencyProbeN, repeatLatencyMs(latencyFastMs, latencyProbeN))
	qa := &AccountQualityLastN{
		N:      latencyProbeN,
		TTFTMs: repeatLatencyMs(latencySlowMs, latencyProbeN),
	}
	RecomputeAccountQualityLastN(qa)

	lookup := &cooldownReasonLookup{
		memorySmartLookup: memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, policy),
			pair:    map[string]*PairQualityLive{smartPairKey(7, 16): live},
			probing: map[string]bool{smartPairKey(7, 16): true},
		},
	}
	require.False(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, "openai", policy, live, time.Now().UTC(), qa))
	require.Equal(t, 1, lookup.startCalls)
	require.Contains(t, lookup.lastReason, CooldownSampleQA)
	require.True(t, strings.Contains(lookup.lastReason, "p50") || strings.Contains(lookup.lastReason, "连续C"),
		"Q_a breach reason must name p50 or consecutive gate: %q", lookup.lastReason)
}

func TestObservePairCompletion_SelectableLatencyCooldownReason(t *testing.T) {
	t.Parallel()
	policy := schedCompositePolicy(7, latencySchedN)
	cache := &observeCacheStub{
		bundle: smartBundle(PlatformAnthropic, policy),
		live:   map[string]*PairQualityLive{},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	for _, v := range latencyScatterSlow(6, 8, latencyFastMs, latencySlowMs) {
		svc.ObservePairCompletion(context.Background(), PairQualityObservation{
			AccountID: 7, UserID: 16, Success: true, FirstTokenMs: intPtr(v),
		})
	}
	require.Equal(t, 1, cache.starts)
	require.Contains(t, cache.lastCooldownReason, CooldownPhaseSelectable)
	require.Contains(t, cache.lastCooldownReason, "超标K")
}
