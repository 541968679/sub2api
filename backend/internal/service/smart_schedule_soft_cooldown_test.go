//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFilterSoftCooldownSamples_DropsOutsideAndLegacyEmpty(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 18, 0, 0, 0, time.UTC)
	since := now.Add(-15 * time.Minute)
	samples := []SoftCooldownSample{
		{UnixTS: now.Add(-20 * time.Minute).Unix(), OK: true, TTFTMs: intPtr(40)},
		{UnixTS: now.Add(-2 * time.Minute).Unix(), OK: true, TTFTMs: intPtr(50)},
	}
	got := FilterSoftCooldownSamples(samples, since)
	require.Len(t, got, 1)
	require.Equal(t, 50, *got[0].TTFTMs)
	require.Empty(t, FilterSoftCooldownSamples(nil, since))
}

func TestSoftCooldownMeets_TimeWindowAndKFail(t *testing.T) {
	t.Parallel()
	p50 := 200
	rate := 0.9
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:            &p50,
		QualityMinSuccessRate:          &rate,
		QualityMinTTFTSamples:          intPtr(2),
		QualityMinSuccessSamples:       intPtr(2),
		QualitySchedWindowN:            intPtr(2),
		QualitySchedMaxSlowInWindow:    intPtr(2),
		QualitySchedMaxConsecutiveSlow: intPtr(2),
	}
	now := time.Date(2026, 8, 27, 18, 0, 0, 0, time.UTC)
	outside := SoftLiveFromSamples([]SoftCooldownSample{
		{UnixTS: now.Add(-40 * time.Minute).Unix(), OK: true, TTFTMs: intPtr(40)},
		{UnixTS: now.Add(-39 * time.Minute).Unix(), OK: true, TTFTMs: intPtr(50)},
	}, 2, 2)
	require.False(t, softCooldownMeets(SoftLiveFromSamples(FilterSoftCooldownSamples([]SoftCooldownSample{
		{UnixTS: now.Add(-40 * time.Minute).Unix(), OK: true, TTFTMs: intPtr(40)},
		{UnixTS: now.Add(-39 * time.Minute).Unix(), OK: true, TTFTMs: intPtr(50)},
	}, now.Add(-15*time.Minute)), 2, 2), policy), "outside-window samples cannot meet")
	require.NotNil(t, outside)

	kFail := SoftLiveFromSamples([]SoftCooldownSample{
		{UnixTS: now.Unix(), OK: true, TTFTMs: intPtr(900)},
		{UnixTS: now.Unix(), OK: true, TTFTMs: intPtr(900)},
	}, 2, 2)
	require.False(t, softCooldownMeets(kFail, policy), "K/C fail cannot meet")

	ok := SoftLiveFromSamples([]SoftCooldownSample{
		{UnixTS: now.Unix(), OK: true, TTFTMs: intPtr(40)},
		{UnixTS: now.Unix(), OK: true, TTFTMs: intPtr(50)},
	}, 2, 2)
	require.True(t, softCooldownMeets(ok, policy))
}

func TestSoftCooldownMeets_UnderfullIsNotPass(t *testing.T) {
	t.Parallel()
	p50 := 200
	rate := 0.9
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:      &p50,
		QualityMinSuccessRate:    &rate,
		QualityMinTTFTSamples:    intPtr(3),
		QualityMinSuccessSamples: intPtr(3),
		QualityCondition:         strPtr(QualityHardCloseConditionOr),
	}
	live := ApplyPairQualityIngestWindows(nil, 3, 3, true, intPtr(40), nil)
	require.False(t, pairQualityBlocks(live, policy), "underfull fail-open must not be treated as pass")
	require.False(t, softCooldownMeets(live, policy))
}

func TestSoftCooldownMeets_FullAndPass(t *testing.T) {
	t.Parallel()
	p50 := 200
	rate := 0.9
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:      &p50,
		QualityMinSuccessRate:    &rate,
		QualityMinTTFTSamples:    intPtr(2),
		QualityMinSuccessSamples: intPtr(2),
		QualityCondition:         strPtr(QualityHardCloseConditionOr),
	}
	var live *PairQualityLive
	live = ApplyPairQualityIngestWindows(live, 2, 2, true, intPtr(40), nil)
	live = ApplyPairQualityIngestWindows(live, 2, 2, true, intPtr(50), nil)
	require.True(t, softCooldownMeets(live, policy))
}

func TestSoftCooldownMeets_FullCKP50BreachFails(t *testing.T) {
	t.Parallel()
	p50 := 200
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:            &p50,
		QualityMinTTFTSamples:          intPtr(4),
		QualityMinSuccessSamples:       intPtr(4),
		QualitySchedWindowN:            intPtr(4),
		QualitySchedMaxSlowInWindow:    intPtr(3),
		QualitySchedMaxConsecutiveSlow: intPtr(2),
		QualityCondition:               strPtr(QualityHardCloseConditionOr),
	}
	var live *PairQualityLive
	for i := 0; i < 4; i++ {
		live = ApplyPairQualityIngestWindows(live, 4, 4, true, intPtr(900), nil)
	}
	require.True(t, live.TTFTCount >= 4)
	require.False(t, softCooldownMeets(live, policy))
}

func TestSoftCooldownMeets_OrAndAndUnconfiguredSkipped(t *testing.T) {
	t.Parallel()
	p50 := 200
	rate := 0.9
	orPolicy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:      &p50,
		QualityMinSuccessRate:    &rate,
		QualityMinTTFTSamples:    intPtr(2),
		QualityMinSuccessSamples: intPtr(2),
		QualityCondition:         strPtr(QualityHardCloseConditionOr),
	}
	andPolicy := *orPolicy
	andPolicy.QualityCondition = strPtr(QualityHardCloseConditionAnd)

	// Success full+pass, TTFT underfull — enter-AND: neither or nor and may meet.
	var live *PairQualityLive
	live = ApplyPairQualityIngestWindows(live, 2, 2, true, nil, nil)
	live = ApplyPairQualityIngestWindows(live, 2, 2, true, nil, nil)
	require.False(t, softCooldownMeets(live, orPolicy), "or no longer single-side meets")
	require.False(t, softCooldownMeets(live, &andPolicy))

	successOnly := &SmartSchedulePlatformPolicy{
		QualityMinSuccessRate:    &rate,
		QualityMinSuccessSamples: intPtr(2),
		QualityMinTTFTSamples:    intPtr(2),
		QualityCondition:         strPtr(QualityHardCloseConditionAnd),
	}
	require.True(t, softCooldownMeets(live, successOnly), "unconfigured latency gate is skipped")

	empty := &SmartSchedulePlatformPolicy{QualityMinTTFTSamples: intPtr(2)}
	require.False(t, softCooldownMeets(live, empty), "no configured gates cannot meet")
}

func TestSoftCooldownMeets_DoesNotUsePairQualityBlocksFailOpen(t *testing.T) {
	t.Parallel()
	p50 := 50
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:      &p50,
		QualityMinTTFTSamples:    intPtr(5),
		QualityMinSuccessSamples: intPtr(5),
	}
	live := ApplyPairQualityIngestWindows(nil, 5, 5, true, intPtr(40), nil)
	require.False(t, pairQualityBlocks(live, policy))
	require.False(t, softCooldownMeets(live, policy))
}

func TestSoftCooldownMeets_TTFTAndDurationAreOneLatencyGate(t *testing.T) {
	t.Parallel()
	p50 := 200
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:      &p50,
		QualityMaxP50DurationMs:  &p50,
		QualityMinTTFTSamples:    intPtr(2),
		QualityMinSuccessSamples: intPtr(2),
		QualityCondition:         strPtr(QualityHardCloseConditionOr),
	}
	var live *PairQualityLive
	live = ApplyPairQualityIngestWindows(live, 2, 2, true, intPtr(40), nil)
	live = ApplyPairQualityIngestWindows(live, 2, 2, true, intPtr(50), nil)
	require.False(t, softCooldownMeets(live, policy), "duration underfull is not a latency pass")

	live = ApplyPairQualityIngestWindows(live, 2, 2, true, nil, intPtr(900))
	live = ApplyPairQualityIngestWindows(live, 2, 2, true, nil, intPtr(900))
	require.GreaterOrEqual(t, live.DurationCount, 2)
	require.False(t, softCooldownMeets(live, policy), "broken duration fails the shared latency gate even when TTFT passes")

	var ok *PairQualityLive
	ok = ApplyPairQualityIngestWindows(ok, 2, 2, true, intPtr(40), nil)
	ok = ApplyPairQualityIngestWindows(ok, 2, 2, true, intPtr(50), nil)
	ok = ApplyPairQualityIngestWindows(ok, 2, 2, true, nil, intPtr(40))
	ok = ApplyPairQualityIngestWindows(ok, 2, 2, true, nil, intPtr(50))
	require.True(t, softCooldownMeets(ok, policy))
}

func TestUserSmartScheduleService_SoftCooldownDefaultHardPutCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	accounts := &stubSmartAccountRepo{accounts: []*Account{
		{ID: 11, Platform: PlatformAnthropic},
		{ID: 12, Platform: PlatformOpenAI},
	}}

	t.Run("default hard", func(t *testing.T) {
		t.Parallel()
		svc := NewUserSmartScheduleService(&stubSmartRepo{}, nil, accounts, nil, nil)
		view, err := svc.Get(ctx, 16)
		require.NoError(t, err)
		require.False(t, view.Platforms[PlatformAnthropic].SoftCooldown)
		view, err = svc.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
			Accounts:        []SmartScheduleAccountMember{{AccountID: 11, Platform: PlatformAnthropic}},
		})
		require.NoError(t, err)
		require.False(t, view.Platforms[PlatformAnthropic].SoftCooldown)
	})

	t.Run("put round trip", func(t *testing.T) {
		t.Parallel()
		svc := NewUserSmartScheduleService(&stubSmartRepo{}, nil, accounts, nil, nil)
		view, err := svc.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
			SoftCooldown:    true,
			Accounts:        []SmartScheduleAccountMember{{AccountID: 11, Platform: PlatformAnthropic}},
		})
		require.NoError(t, err)
		require.True(t, view.Platforms[PlatformAnthropic].SoftCooldown)
		view, err = svc.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
			Accounts:        []SmartScheduleAccountMember{{AccountID: 11, Platform: PlatformAnthropic}},
		})
		require.NoError(t, err)
		require.False(t, view.Platforms[PlatformAnthropic].SoftCooldown, "omitted PUT is hard")
	})

	t.Run("copy includes field", func(t *testing.T) {
		t.Parallel()
		from := enabledSmartPolicy(11, 3, intPtr(800))
		from.SoftCooldown = true
		from.CooldownMinutes = 20
		localRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, from)}
		localRepo.bundle.Policies[PlatformOpenAI] = &SmartSchedulePlatformPolicy{
			Enabled:         false,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{12: {}},
		}
		local := NewUserSmartScheduleService(localRepo, nil, accounts, nil, nil)
		view, err := local.CopyPlatform(ctx, 16, PlatformOpenAI, PlatformAnthropic)
		require.NoError(t, err)
		require.True(t, view.Platforms[PlatformOpenAI].SoftCooldown)
		require.Equal(t, 20, view.Platforms[PlatformOpenAI].CooldownMinutes)
	})
}

func TestQualityHardCloseSettings_JSONOmitsSoftCooldown(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(QualityHardCloseSettings{
		Enabled:   true,
		Condition: QualityHardCloseConditionOr,
	})
	require.NoError(t, err)
	require.NotContains(t, string(raw), "soft_cooldown")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	_, ok := payload["soft_cooldown"]
	require.False(t, ok)
}

func TestObservePairCompletion_SoftCooldownEarlyExit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 200
	rate := 0.9
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.SoftCooldown = true
	policy.QualityMinSuccessRate = &rate
	policy.QualityWindowSamples = intPtr(2)
	policy.QualityMinTTFTSamples = intPtr(2)
	policy.QualityMinSuccessSamples = intPtr(2)
	policy.QualityCondition = strPtr(QualityHardCloseConditionOr)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		cooling: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)

	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 8, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(40),
	})
	require.Empty(t, cache.softEnded)
	require.Equal(t, []int64{7}, cache.softIngested)
	require.Nil(t, cache.GetSoftCooldown(ctx, 8, 16, PlatformAnthropic), "observer must not enter its own window")

	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 8, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(50),
	})
	require.Equal(t, []int64{7}, cache.softEnded)
	require.Contains(t, cache.events, PairQualityEventSoftCooldownEnd)
	require.Contains(t, cache.events, PairQualityEventExpiryZero)
	require.NotContains(t, cache.events, PairQualityEventProbeEnter)
	require.NotContains(t, cache.events, PairQualityEventCooldownEnd)
	require.False(t, cache.IsProbing(ctx, 7, 16, PlatformAnthropic), "soft-end goes to selectable, not 考察")
	require.False(t, cache.CooldownActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()))
}

func TestObservePairCompletion_SoftCooldownHardDoesNotIngest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 200
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.QualityWindowSamples = intPtr(1)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		cooling: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 8, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(40),
	})
	require.Empty(t, cache.softIngested)
	require.Empty(t, cache.softEnded)
}

func TestObservePairCompletion_SoftCooldownSelfSampleExcluded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 50
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.SoftCooldown = true
	policy.QualityWindowSamples = intPtr(1)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		cooling: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 7, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(40),
	})
	require.Empty(t, cache.softIngested)
	require.Nil(t, cache.GetSoftCooldown(ctx, 7, 16, PlatformAnthropic))
}

func TestObservePairCompletion_SoftCooldownSelfCoolDoesNotIngestOwnWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 50
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.SoftCooldown = true
	policy.QualityWindowSamples = intPtr(1)
	cache := &observeCacheStub{bundle: smartBundle(PlatformAnthropic, policy)}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 7, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(900),
	})
	require.Equal(t, 1, cache.starts)
	require.NotContains(t, cache.softIngested, int64(7))
	require.Nil(t, cache.GetSoftCooldown(ctx, 7, 16, PlatformAnthropic))
}

func TestObservePairCompletion_SoftCooldownABIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 200
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.AccountIDs[9] = struct{}{}
	policy.SoftCooldown = true
	policy.QualityWindowSamples = intPtr(2)
	cache := &observeCacheStub{
		bundle: smartBundle(PlatformAnthropic, policy),
		cooling: map[string]bool{
			smartPairKey(7, 16): true,
			smartPairKey(8, 16): true,
		},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 9, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(40),
	})
	require.ElementsMatch(t, []int64{7, 8}, cache.softIngested)
	require.NotNil(t, cache.GetSoftCooldown(ctx, 7, 16, PlatformAnthropic))
	require.NotNil(t, cache.GetSoftCooldown(ctx, 8, 16, PlatformAnthropic))
	require.Nil(t, cache.GetSoftCooldown(ctx, 9, 16, PlatformAnthropic))
}

func TestObservePairCompletion_SoftCooldownManualSetCooldownEarlyExit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 200
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.SoftCooldown = true
	policy.QualityWindowSamples = intPtr(1)
	policy.QualityMinTTFTSamples = intPtr(1)
	policy.QualityMinSuccessSamples = intPtr(1)
	cache := &observeCacheStub{bundle: smartBundle(PlatformAnthropic, policy)}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	until, err := cache.SetCooldown(ctx, 7, 16, PlatformAnthropic, 15, time.Now().UTC())
	require.NoError(t, err)
	require.False(t, until.IsZero())
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 8, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(40),
	})
	require.Equal(t, []int64{7}, cache.softIngested)
	require.Equal(t, []int64{7}, cache.softEnded)
	require.Contains(t, cache.events, PairQualityEventSoftCooldownEnd)
	require.False(t, cache.IsProbing(ctx, 7, 16, PlatformAnthropic), "soft-end goes to selectable, not 考察")
}

func TestObservePairCompletion_SoftCooldownManualSetCooldownHardWaits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 50
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.QualityWindowSamples = intPtr(1)
	cache := &observeCacheStub{bundle: smartBundle(PlatformAnthropic, policy)}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	_, err := cache.SetCooldown(ctx, 7, 16, PlatformAnthropic, 15, time.Now().UTC())
	require.NoError(t, err)
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 8, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(40),
	})
	require.Empty(t, cache.softIngested)
	require.Empty(t, cache.softEnded)
	require.True(t, cache.CooldownActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()))
}

func TestObservePairCompletion_SoftCooldownSkipsPinnedPeer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 200
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.SoftCooldown = true
	policy.QualityWindowSamples = intPtr(1)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		cooling: map[string]bool{smartPairKey(7, 16): true},
		pinned:  map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 8, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(40),
	})
	require.Empty(t, cache.softIngested)
	require.Empty(t, cache.softEnded)
}

func TestObservePairCompletion_SoftCooldownPinFeedsPeers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 200
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.AccountIDs[8] = struct{}{}
	policy.SoftCooldown = true
	policy.QualityWindowSamples = intPtr(1)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		cooling: map[string]bool{smartPairKey(7, 16): true},
		pinned:  map[string]bool{smartPairKey(8, 16): true},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID: 8, UserID: 16, Platform: PlatformAnthropic, Success: true, FirstTokenMs: intPtr(40),
	})
	require.Equal(t, []int64{7}, cache.softIngested)
	require.Equal(t, []int64{7}, cache.softEnded)
}

func TestUserSmartScheduleService_HydrateSoftCooldownProgress(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	until := time.Now().UTC().Add(12 * time.Minute)
	p50 := 200
	policy := &SmartSchedulePlatformPolicy{
		Enabled:                  true,
		CooldownMinutes:          15,
		SoftCooldown:             true,
		QualityMaxP50TTFTMs:      &p50,
		QualityMinTTFTSamples:    intPtr(10),
		QualityMinSuccessSamples: intPtr(10),
		AccountIDs:               map[int64]struct{}{21: {}, 22: {}},
	}
	repo := &stubSmartRepo{bundle: smartBundle(PlatformOpenAI, policy)}
	cache := hydrateSoftCache{
		stubSmartCache: stubSmartCache{until: map[int64]time.Time{21: until}},
		soft: map[int64]*PairQualityLive{
			21: ApplyPairQualityIngestWindows(nil, 10, 10, true, intPtr(40), nil),
		},
	}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 21, Platform: PlatformOpenAI},
		{ID: 22, Platform: PlatformOpenAI},
	}}, nil, nil)
	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	members := view.Platforms[PlatformOpenAI].Accounts
	require.True(t, view.Platforms[PlatformOpenAI].SoftCooldown)
	var cooling, idle *SmartScheduleAccountMember
	for i := range members {
		if members[i].AccountID == 21 {
			cooling = &members[i]
		}
		if members[i].AccountID == 22 {
			idle = &members[i]
		}
	}
	require.NotNil(t, cooling)
	require.NotNil(t, cooling.SoftCooldownProgress)
	require.Equal(t, 1, cooling.SoftCooldownProgress.TTFTCount)
	require.Equal(t, 10, cooling.SoftCooldownProgress.NTTFT)
	require.Nil(t, idle.SoftCooldownProgress)
}

func TestUserSmartScheduleService_HydrateSoftCooldownProgressHardOmits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	until := time.Now().UTC().Add(12 * time.Minute)
	p50 := 200
	policy := &SmartSchedulePlatformPolicy{
		Enabled:                  true,
		CooldownMinutes:          15,
		QualityMaxP50TTFTMs:      &p50,
		QualityMinTTFTSamples:    intPtr(10),
		QualityMinSuccessSamples: intPtr(10),
		AccountIDs:               map[int64]struct{}{21: {}},
	}
	repo := &stubSmartRepo{bundle: smartBundle(PlatformOpenAI, policy)}
	cache := hydrateSoftCache{
		stubSmartCache: stubSmartCache{until: map[int64]time.Time{21: until}},
		soft: map[int64]*PairQualityLive{
			21: ApplyPairQualityIngestWindows(nil, 10, 10, true, intPtr(40), nil),
		},
	}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 21, Platform: PlatformOpenAI},
	}}, nil, nil)
	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	require.False(t, view.Platforms[PlatformOpenAI].SoftCooldown)
	require.Len(t, view.Platforms[PlatformOpenAI].Accounts, 1)
	require.Nil(t, view.Platforms[PlatformOpenAI].Accounts[0].SoftCooldownProgress)
}

type hydrateSoftCache struct {
	stubSmartCache
	soft map[int64]*PairQualityLive
}

func (s hydrateSoftCache) GetSoftCooldownBatch(_ context.Context, accountIDs []int64, _ int64, _ string) map[int64]*PairQualityLive {
	out := map[int64]*PairQualityLive{}
	for _, id := range accountIDs {
		if live := s.soft[id]; live != nil {
			out[id] = live
		}
	}
	return out
}
