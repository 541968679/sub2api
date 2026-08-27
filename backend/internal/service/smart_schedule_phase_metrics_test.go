//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func schedVsProbeLive() *PairQualityLive {
	ttft := make([]int, 10)
	ok := make([]bool, 10)
	for i := 0; i < 9; i++ {
		ttft[i] = 100
		ok[i] = true
	}
	ttft[8] = 2000
	ttft[9] = 2000
	ok[8] = true
	ok[9] = true
	live := &PairQualityLive{N: 10, NTTFT: 10, NOK: 10, TTFTMs: ttft, OK: ok}
	RecomputePairQuality(live)
	return live
}

func schedVsProbePolicy() *SmartSchedulePlatformPolicy {
	p50 := 500
	nTtft := 10
	schedN := 2
	k := 1
	probeKC := 5
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.QualityMinTTFTSamples = &nTtft
	policy.QualitySchedWindowN = &schedN
	policy.QualitySchedMaxSlowInWindow = &k
	policy.QualityMaxSlowInWindow = &probeKC
	policy.QualityMaxConsecutiveSlow = &probeKC
	return policy
}

func TestHydratePairQuality_PhaseMetricsSchedWindowP50(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := schedVsProbePolicy()
	live := schedVsProbeLive()
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, policy)}
	cache := &observeCacheStub{live: map[string]*PairQualityLive{smartPairKey(7, 16): live}}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)

	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	member := view.Platforms[PlatformAnthropic].Accounts[0]
	require.NotNil(t, member.PairQuality)
	require.Equal(t, MetricsPhaseSched, member.PairQuality.MetricsPhase)
	require.NotNil(t, member.PairQuality.P50TTFTMs)
	require.Equal(t, 2000, *member.PairQuality.P50TTFTMs)
	require.Equal(t, 2, member.PairQuality.NTTFT)
	require.NotNil(t, member.PairQuality.Probe)
	require.NotNil(t, member.PairQuality.Probe.P50TTFTMs)
	require.Equal(t, 100, *member.PairQuality.Probe.P50TTFTMs)
	require.Equal(t, 10, member.PairQuality.Probe.NTTFT)
	require.True(t, member.WillCool)
	require.NotNil(t, member.QualityReason)
}

func TestHydratePairQuality_WillCoolUsesProbeKnobs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := schedVsProbePolicy()
	live := schedVsProbeLive()
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, policy)}
	cache := &observeCacheStub{
		live:    map[string]*PairQualityLive{smartPairKey(7, 16): live},
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)

	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	member := view.Platforms[PlatformAnthropic].Accounts[0]
	require.True(t, member.Probing)
	require.Equal(t, MetricsPhaseProbe, member.PairQuality.MetricsPhase)
	require.NotNil(t, member.PairQuality.P50TTFTMs)
	require.Equal(t, 100, *member.PairQuality.P50TTFTMs)
	require.Equal(t, 10, member.PairQuality.NTTFT)
	require.False(t, member.WillCool, "probe K/C 2/2 and p50 pass; sched K=1 would fail")
	require.Nil(t, member.QualityReason)
}

func TestObservePairCompletion_EvalFailWritesCooldownWithoutAdmit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := schedVsProbePolicy()
	cache := &observeCacheStub{
		bundle: smartBundle(PlatformAnthropic, policy),
		live:   map[string]*PairQualityLive{smartPairKey(7, 16): schedVsProbeLive()},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 7, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(2000),
	})
	require.Greater(t, cache.starts, 0)
	require.True(t, cache.CooldownActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()))
}

func TestLogSmartScheduleEvent_CooldownStartSoftEndProbeEnter(t *testing.T) {
	var events []string
	var lastUser, lastAccount int64
	var lastPlatform, lastPhase string
	prev := logSmartScheduleEventFn
	logSmartScheduleEventFn = func(event string, userID, accountID int64, platform, phase, reason string) {
		events = append(events, event)
		lastUser, lastAccount = userID, accountID
		lastPlatform, lastPhase = platform, phase
		require.NotEmpty(t, event)
		_ = reason
	}
	t.Cleanup(func() { logSmartScheduleEventFn = prev })

	policy := schedVsProbePolicy()
	cache := &observeCacheStub{
		bundle: smartBundle(PlatformAnthropic, policy),
		live:   map[string]*PairQualityLive{smartPairKey(7, 16): schedVsProbeLive()},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{
		AccountID: 7, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(2000),
	})
	require.Contains(t, events, SmartScheduleLogCooldownStart)
	require.Equal(t, int64(16), lastUser)
	require.Equal(t, int64(7), lastAccount)
	require.Equal(t, PlatformAnthropic, lastPlatform)
	require.Equal(t, CooldownPhaseSelectable, lastPhase)

	cache.SoftEndCooldown(context.Background(), 7, 16, PlatformAnthropic, "soft_cooldown")
	require.Contains(t, events, SmartScheduleLogSoftEnd)

	cache.EnterProbe(context.Background(), 7, 16, PlatformAnthropic)
	require.Contains(t, events, SmartScheduleLogProbeEnter)
}
