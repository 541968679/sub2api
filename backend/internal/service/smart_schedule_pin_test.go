//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestUserSmartScheduleService_SetPairAdmissionPinned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	quality := &liveQualityCacheStub{}
	require.NoError(t, quality.MarkUserResume(ctx, 7, 16))
	cache := &admissionCacheRecorder{
		bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 4, nil)),
		probing: map[string]bool{
			smartPairKey(7, 16): true,
		},
	}
	svc := NewUserSmartScheduleService(nil, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, quality, nil)

	pinned, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionPinned)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionPinned, pinned.State)
	require.True(t, pinned.Pinned)
	require.False(t, pinned.Probing)
	require.Equal(t, 1, cache.cleared, "enter pin clears cooldown")
	require.GreaterOrEqual(t, cache.clearedProbe, 1, "enter pin clears probing")
	require.Equal(t, 1, cache.markedPin)
	require.True(t, cache.IsPinned(ctx, 7, 16, "openai"))
	require.False(t, cache.IsProbing(ctx, 7, 16, "openai"))
	require.True(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()), "pin must not clear Track A resume")
	require.False(t, cache.PairResumeActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()), "pin must not write pair resume")
	require.NotContains(t, cache.zeros, PairQualityEventResumed)
	require.NotContains(t, cache.zeros, PairQualityEventSelectable)

	omitted, err := svc.SetPairAdmission(ctx, 7, 16, "")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionResumed, omitted.State, "omit state is 豁免期, not pinned")
	require.False(t, omitted.Pinned)
	require.False(t, cache.IsPinned(ctx, 7, 16, "openai"))
	require.True(t, cache.PairResumeActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()))
	require.True(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()), "pair 豁免期 must not overwrite Track A resume")
}

func TestUserSmartScheduleService_PauseDoesNotBecomePinned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cache := &admissionCacheRecorder{
		bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil)),
	}
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, &liveQualityCacheStub{}, nil)

	paused, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionPaused)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionPaused, paused.State)
	require.False(t, paused.Pinned)
	require.Equal(t, 0, cache.markedPin)
	require.False(t, cache.IsPinned(ctx, 7, 16, "openai"))

	omitted, err := svc.SetPairAdmission(ctx, 7, 16, "")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionResumed, omitted.State)
	require.False(t, omitted.Pinned)
	require.Equal(t, 0, cache.markedPin)
}

func TestUserSmartScheduleService_GetHydratesPinned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := enabledSmartPolicy(7, 4, nil)
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, policy)}
	cache := &admissionCacheRecorder{
		bundle: repo.bundle,
		pinned: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)

	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	member := view.Platforms[PlatformAnthropic].Accounts[0]
	require.True(t, member.Pinned)
	require.False(t, member.Probing)
	require.False(t, member.WillCool)

	cache.ClearPinned(ctx, 7, 16, "openai")
	unmarked, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	require.False(t, unmarked.Platforms[PlatformAnthropic].Accounts[0].Pinned, "Redis miss is not pinned")

	pausedPolicy := enabledSmartPolicy(7, 4, nil)
	pausedPolicy.Paused = map[int64]struct{}{7: {}}
	pausedRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, pausedPolicy)}
	pausedCache := &admissionCacheRecorder{
		bundle: pausedRepo.bundle,
		pinned: map[string]bool{smartPairKey(7, 16): true},
	}
	pausedSvc := NewUserSmartScheduleService(pausedRepo, pausedCache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)
	pausedView, err := pausedSvc.Get(ctx, 16)
	require.NoError(t, err)
	require.True(t, pausedView.Platforms[PlatformAnthropic].Accounts[0].Paused)
	require.False(t, pausedView.Platforms[PlatformAnthropic].Accounts[0].Pinned, "paused ignores leftover pin bits")

	coolingRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 4, nil))}
	coolingCache := &admissionCacheRecorder{
		stubSmartCache: stubSmartCache{until: map[int64]time.Time{7: time.Now().UTC().Add(time.Hour)}},
		bundle:         coolingRepo.bundle,
		pinned:         map[string]bool{smartPairKey(7, 16): true},
	}
	coolingSvc := NewUserSmartScheduleService(coolingRepo, coolingCache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)
	coolingView, err := coolingSvc.Get(ctx, 16)
	require.NoError(t, err)
	require.NotNil(t, coolingView.Platforms[PlatformAnthropic].Accounts[0].CooldownUntil)
	require.False(t, coolingView.Platforms[PlatformAnthropic].Accounts[0].Pinned, "future cooldown ignores leftover pin bits")
}

func TestAdmitsScheduleUser_PinnedSkipsEvaluate(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	acc := &Account{ID: 7, Platform: PlatformAnthropic}
	p50 := 50
	policy := probePolicy(7, 3, &p50, nil, false)
	live := mixedAndLive(3, 400, 3)
	lookup := &memorySmartLookup{
		bundle: smartBundle(PlatformAnthropic, policy),
		pair:   map[string]*PairQualityLive{smartPairKey(7, 16): live},
		pinned: map[string]bool{smartPairKey(7, 16): true},
	}
	require.True(t, admitsScheduleUser(ctx, acc, nil, lookup))
	require.Equal(t, 0, lookup.startCalls, "pinned must not StartCooldown")
	require.True(t, lookup.IsPinned(ctx, 7, 16, "openai"))
}

func TestObservePairCompletion_PinnedIngestsButDoesNotCool(t *testing.T) {
	t.Parallel()
	p50 := 50
	policy := probePolicy(7, 3, &p50, nil, false)
	cache := &observeCacheStub{
		bundle: smartBundle(PlatformAnthropic, policy),
		live:   map[string]*PairQualityLive{},
		pinned: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)

	for i := 0; i < 3; i++ {
		svc.ObservePairCompletion(context.Background(), PairQualityObservation{
			AccountID:    7,
			UserID:       16,
			Success:      true,
			FirstTokenMs: intPtr(400),
		})
	}
	require.Len(t, cache.ingested, 3, "pinned windows may keep ingesting")
	require.Equal(t, 3, cache.GetPairQuality(context.Background(), 7, 16, "openai").OKCount)
	require.Equal(t, 0, cache.starts, "N successes while pinned do not cool")
}

func TestObservePairCompletion_LeavePinnedToSelectableCanCool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 50
	policy := probePolicy(7, 3, &p50, nil, false)
	cache := &observeCacheStub{
		bundle: smartBundle(PlatformAnthropic, policy),
		live:   map[string]*PairQualityLive{},
		pinned: map[string]bool{smartPairKey(7, 16): true},
	}
	quality := &liveQualityCacheStub{}
	svc := NewUserSmartScheduleService(nil, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, quality, nil)

	left, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionSelectable)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionSelectable, left.State)
	require.False(t, cache.IsPinned(ctx, 7, 16, "openai"))

	for i := 0; i < 3; i++ {
		svc.ObservePairCompletion(ctx, PairQualityObservation{
			AccountID:    7,
			UserID:       16,
			Success:      true,
			FirstTokenMs: intPtr(400),
		})
	}
	require.Equal(t, 1, cache.starts, "leave to selectable can cool again")
}
