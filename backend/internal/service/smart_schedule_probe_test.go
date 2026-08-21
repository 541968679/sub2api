//go:build unit

package service

import (
	"context"
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

func TestPairQualityProbeGraduates_WokOnlySync(t *testing.T) {
	t.Parallel()
	rate := 0.8
	policy := probePolicy(7, 3, nil, &rate, false)
	require.False(t, pairQualityProbeGraduates(syncOKLive(3, 2), policy), "W_ok < N must not graduate")
	require.True(t, pairQualityProbeGraduates(syncOKLive(3, 3), policy), "full W_ok + empty W_ttft must graduate")

	p50 := 50
	withTTFT := probePolicy(7, 3, &p50, &rate, false)
	require.True(t, pairQualityProbeGraduates(syncOKLive(3, 3), withTTFT), "unfilled W_ttft must not block graduate")
}

func TestPairQualityProbeAndMixed_OnlyInProbe(t *testing.T) {
	t.Parallel()
	p50 := 100
	rate := 0.9
	policy := probePolicy(7, 3, &p50, &rate, true)
	live := mixedAndLive(3, 400, 3)
	require.Equal(t, 3, live.OKCount)
	require.Equal(t, 3, live.TTFTCount)
	require.False(t, pairQualityBlocks(live, policy), "and: success passes so standard and must not cool")
	require.False(t, pairQualityProbeGraduates(live, policy), "full W_ttft p50 fail must not graduate")
	require.True(t, pairQualityProbeAndMixed(live, policy))

	orPolicy := probePolicy(7, 3, &p50, &rate, false)
	require.False(t, pairQualityProbeAndMixed(live, orPolicy), "or is not the mixed override")
	require.True(t, pairQualityBlocks(live, orPolicy), "or still cools on the failed TTFT window")
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
	require.True(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, policy, live, time.Now().UTC()))
	require.Equal(t, 1, lookup.graduated)
	require.False(t, lookup.IsProbing(context.Background(), 7, 16))
	require.Equal(t, 3, lookup.GetPairQuality(context.Background(), 7, 16).OKCount, "graduate must keep windows")
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
	require.False(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, policy, live, time.Now().UTC()))
	require.Equal(t, 1, lookup.startCalls)
	require.False(t, lookup.IsProbing(context.Background(), 7, 16))
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
	require.True(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, policy, live, time.Now().UTC()))
	require.Equal(t, 0, lookup.startCalls)
	require.False(t, lookup.IsProbing(context.Background(), 7, 16), "no mark = not probing / no backfill")
}

func TestEvaluateSmartSchedule_NoTrafficStaysProbing(t *testing.T) {
	t.Parallel()
	policy := probePolicy(7, 3, intPtr(50), nil, false)
	lookup := &memorySmartLookup{
		bundle:  smartBundle(PlatformAnthropic, policy),
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	require.True(t, evaluateSmartSchedulePairQuality(context.Background(), lookup, 7, 16, policy, syncOKLive(3, 1), time.Now().UTC()))
	require.True(t, lookup.IsProbing(context.Background(), 7, 16))
	require.Equal(t, 0, lookup.graduated)
	require.Equal(t, 0, lookup.startCalls)
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
		require.False(t, lookup.IsProbing(ctx, 7, 16))
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

	t.Run("resume grace skips evaluate", func(t *testing.T) {
		t.Parallel()
		p50 := 50
		lookup := &memorySmartLookup{
			bundle:  smartBundle(PlatformAnthropic, probePolicy(7, 3, &p50, nil, false)),
			pair:    map[string]*PairQualityLive{smartPairKey(7, 16): mixedAndLive(3, 400, 3)},
			probing: map[string]bool{smartPairKey(7, 16): true},
		}
		quality := &liveQualityCacheStub{}
		require.NoError(t, quality.MarkUserResume(context.Background(), 7, 16))
		require.True(t, admitsScheduleUser(ctx, acc, quality, lookup))
		require.Equal(t, 0, lookup.startCalls)
		require.Equal(t, 0, lookup.graduated)
		require.True(t, lookup.IsProbing(ctx, 7, 16))
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
	require.True(t, cache.IsProbing(context.Background(), 7, 16))

	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true})
	require.Equal(t, 1, cache.graduated)
	require.False(t, cache.IsProbing(context.Background(), 7, 16))
	require.Equal(t, 3, cache.GetPairQuality(context.Background(), 7, 16).OKCount)
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
	require.False(t, mixed.IsProbing(context.Background(), 7, 16))
	require.Equal(t, 0, mixed.graduated)
	require.Equal(t, 3, mixed.GetPairQuality(context.Background(), 7, 16).OKCount)
	require.Equal(t, 3, mixed.GetPairQuality(context.Background(), 7, 16).TTFTCount)
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
	require.False(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()))
	require.Contains(t, cache.zeros, PairQualityEventSelectable)

	probe, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionProbing)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionProbing, probe.State)
	require.True(t, probe.Probing)
	require.NotNil(t, probe.ProbeCap)
	require.Equal(t, 4, *probe.ProbeCap, "min(N=10, cap=4)")
	require.Equal(t, 1, cache.markedProbe)
	require.True(t, cache.IsProbing(ctx, 7, 16))
	require.False(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()), "enter probing clears u:/w:")
	require.Contains(t, cache.zeros, "")

	again, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionSelectable)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionSelectable, again.State)
	require.False(t, cache.IsProbing(ctx, 7, 16))
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

	cache.ClearProbing(ctx, 7, 16)
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
	require.False(t, cache.IsProbing(ctx, 7, 16))

	explicit, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionProbing)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionProbing, explicit.State)
	require.False(t, repo.bundle.Policies[PlatformAnthropic].IsPaused(7))
	require.Equal(t, 1, cache.markedProbe)
}

func floatPtr(v float64) *float64 { return &v }
