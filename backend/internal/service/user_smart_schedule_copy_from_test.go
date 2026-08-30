//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type copyFromUserCacheRecorder struct {
	stubSmartCache
	invalidated  []int64
	pinWrites    int
	coolWrites   int
	probeWrites  int
	resumeWrites int
}

func (c *copyFromUserCacheRecorder) Invalidate(_ context.Context, userID int64) error {
	c.invalidated = append(c.invalidated, userID)
	return nil
}

func (c *copyFromUserCacheRecorder) MarkPinned(context.Context, int64, int64, string) {
	c.pinWrites++
}

func (c *copyFromUserCacheRecorder) StartCooldown(context.Context, int64, int64, string, int, time.Time) {
	c.coolWrites++
}

func (c *copyFromUserCacheRecorder) StartCooldownWithReason(context.Context, int64, int64, string, int, time.Time, string) {
	c.coolWrites++
}

func (c *copyFromUserCacheRecorder) SetCooldown(_ context.Context, _ int64, _ int64, _ string, _ int, now time.Time) (time.Time, error) {
	c.coolWrites++
	return now, nil
}

func (c *copyFromUserCacheRecorder) SetCooldownWithReason(_ context.Context, _ int64, _ int64, _ string, _ int, now time.Time, _ string) (time.Time, error) {
	c.coolWrites++
	return now, nil
}

func (c *copyFromUserCacheRecorder) MarkProbing(context.Context, int64, int64, string) {
	c.probeWrites++
}

func (c *copyFromUserCacheRecorder) MarkPairResume(context.Context, int64, int64, string) error {
	c.resumeWrites++
	return nil
}

func TestUserSmartScheduleService_CopyFromUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("pool and pause replace from source", func(t *testing.T) {
		t.Parallel()
		repo, svc, cache := newCopyFromUserFixture()
		preview, err := svc.PreviewCopyFromUser(ctx, 99, PlatformAnthropic, 16)
		require.NoError(t, err)
		require.Equal(t, []int64{12}, preview.Add)
		require.Equal(t, []int64{13}, preview.Remove)
		require.Equal(t, []int64{11}, preview.Overlap)
		require.Equal(t, []int64{11}, preview.SourcePausedAccountIDs)
		require.False(t, preview.SourceEmpty)

		view, err := svc.CopyFromUser(ctx, 99, PlatformAnthropic, 16, preview.SourceRevision, SmartScheduleCopySlices{
			Pool:        true,
			Concurrency: true,
			SortOrder:   true,
		})
		require.NoError(t, err)
		dest := view.Platforms[PlatformAnthropic]
		require.False(t, dest.Enabled, "enabled slice defaults off")
		require.Len(t, dest.Accounts, 2)
		byID := map[int64]SmartScheduleAccountMember{}
		for _, member := range dest.Accounts {
			byID[member.AccountID] = member
		}
		require.True(t, byID[11].Paused)
		require.False(t, byID[12].Paused)
		require.Equal(t, 3, *byID[11].MaxConcurrency)
		require.Equal(t, 4, *byID[12].MaxConcurrency)
		require.Equal(t, 1, *byID[11].SortOrder)
		require.Equal(t, 2, *byID[12].SortOrder)
		require.NotContains(t, byID, int64(13))
		require.True(t, repo.lastWritePausedFromMembers)
		require.Equal(t, int64(99), repo.lastReplaceUser)
		require.Equal(t, []int64{99}, cache.invalidated)
		require.Zero(t, cache.pinWrites)
		require.Zero(t, cache.coolWrites)
		require.Zero(t, cache.probeWrites)
		require.Zero(t, cache.resumeWrites)
		openai := view.Platforms[PlatformOpenAI]
		require.True(t, openai.Enabled)
		require.Len(t, openai.Accounts, 1)
		require.Equal(t, int64(21), openai.Accounts[0].AccountID)
		require.True(t, openai.Accounts[0].Paused)
	})

	t.Run("overlap pause follows source not target leftover", func(t *testing.T) {
		t.Parallel()
		repo, svc, _ := newCopyFromUserFixture()
		repo.byUser[16].Policies[PlatformAnthropic].Paused = map[int64]struct{}{}
		repo.byUser[99].Policies[PlatformAnthropic].Paused = map[int64]struct{}{11: {}}
		preview, err := svc.PreviewCopyFromUser(ctx, 99, PlatformAnthropic, 16)
		require.NoError(t, err)
		require.Empty(t, preview.SourcePausedAccountIDs)
		view, err := svc.CopyFromUser(ctx, 99, PlatformAnthropic, 16, preview.SourceRevision, SmartScheduleCopySlices{Pool: true})
		require.NoError(t, err)
		byID := map[int64]SmartScheduleAccountMember{}
		for _, member := range view.Platforms[PlatformAnthropic].Accounts {
			byID[member.AccountID] = member
		}
		require.False(t, byID[11].Paused, "source unpaused clears target leftover on overlap")
		require.False(t, byID[12].Paused)
		require.True(t, repo.lastWritePausedFromMembers)
		require.Nil(t, byID[11].MaxConcurrency, "concurrency slice off leaves cap unset")
		require.Nil(t, byID[11].SortOrder)
	})

	t.Run("put still restores target leftover pause", func(t *testing.T) {
		t.Parallel()
		repo, svc, _ := newCopyFromUserFixture()
		view, err := svc.PutPlatform(ctx, 99, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         false,
			CooldownMinutes: 15,
			Accounts: []SmartScheduleAccountMember{
				{AccountID: 11, Platform: PlatformAnthropic, Paused: false},
				{AccountID: 13, Platform: PlatformAnthropic, Paused: false},
			},
		})
		require.NoError(t, err)
		require.False(t, repo.lastWritePausedFromMembers)
		byID := map[int64]SmartScheduleAccountMember{}
		for _, member := range view.Platforms[PlatformAnthropic].Accounts {
			byID[member.AccountID] = member
		}
		require.False(t, byID[11].Paused)
		require.True(t, byID[13].Paused, "PUT leftover pause must survive client paused=false")
	})

	t.Run("empty usable source rejects pool copy", func(t *testing.T) {
		t.Parallel()
		repo := &stubSmartRepo{byUser: map[int64]*UserSmartScheduleBundle{
			16: {Policies: map[string]*SmartSchedulePlatformPolicy{
				PlatformAnthropic: {Enabled: true, CooldownMinutes: 15, AccountIDs: map[int64]struct{}{77: {}}},
			}},
			99: {Policies: map[string]*SmartSchedulePlatformPolicy{
				PlatformAnthropic: enabledSmartPolicy(13, 1, nil),
			}},
		}}
		svc := NewUserSmartScheduleService(repo, &copyFromUserCacheRecorder{}, &stubSmartAccountRepo{accounts: []*Account{
			{ID: 13, Platform: PlatformAnthropic},
		}}, nil, nil)
		preview, err := svc.PreviewCopyFromUser(ctx, 99, PlatformAnthropic, 16)
		require.NoError(t, err)
		require.True(t, preview.SourceEmpty)
		require.Equal(t, 1, preview.SkippedUnavailable)
		_, err = svc.CopyFromUser(ctx, 99, PlatformAnthropic, 16, preview.SourceRevision, SmartScheduleCopySlices{Pool: true})
		require.Error(t, err)
		require.Equal(t, "SMART_SCHEDULE_COPY_EMPTY_SOURCE", infraerrors.Reason(err))
		require.True(t, repo.byUser[99].Policies[PlatformAnthropic].HasAccount(13))
	})

	t.Run("stale revision is 409", func(t *testing.T) {
		t.Parallel()
		repo, svc, _ := newCopyFromUserFixture()
		preview, err := svc.PreviewCopyFromUser(ctx, 99, PlatformAnthropic, 16)
		require.NoError(t, err)
		repo.byUser[16].Policies[PlatformAnthropic].Paused[12] = struct{}{}
		_, err = svc.CopyFromUser(ctx, 99, PlatformAnthropic, 16, preview.SourceRevision, SmartScheduleCopySlices{Pool: true})
		require.Error(t, err)
		require.Equal(t, "SMART_SCHEDULE_COPY_STALE", infraerrors.Reason(err))
		require.Equal(t, 409, infraerrors.Code(err))
	})

	t.Run("concurrency and sort without pool are rejected", func(t *testing.T) {
		t.Parallel()
		_, svc, _ := newCopyFromUserFixture()
		_, err := svc.CopyFromUser(ctx, 99, PlatformAnthropic, 16, "rev", SmartScheduleCopySlices{
			Concurrency: true,
			SortOrder:   true,
		})
		require.Error(t, err)
		require.Equal(t, "SMART_SCHEDULE_COPY_SLICES", infraerrors.Reason(err))
	})

	t.Run("enabled slice default stays off", func(t *testing.T) {
		t.Parallel()
		_, svc, _ := newCopyFromUserFixture()
		preview, err := svc.PreviewCopyFromUser(ctx, 99, PlatformAnthropic, 16)
		require.NoError(t, err)
		require.Equal(t, SmartScheduleCopyEnabledEnable, preview.EnabledDelta)
		view, err := svc.CopyFromUser(ctx, 99, PlatformAnthropic, 16, preview.SourceRevision, SmartScheduleCopySlices{
			Pool: true,
		})
		require.NoError(t, err)
		require.False(t, view.Platforms[PlatformAnthropic].Enabled)
	})

	t.Run("enabled with empty result pool is rejected", func(t *testing.T) {
		t.Parallel()
		repo := &stubSmartRepo{byUser: map[int64]*UserSmartScheduleBundle{
			16: {Policies: map[string]*SmartSchedulePlatformPolicy{
				PlatformAnthropic: enabledSmartPolicy(11, 3, nil),
			}},
			99: {Policies: map[string]*SmartSchedulePlatformPolicy{
				PlatformAnthropic: {Enabled: false, CooldownMinutes: 15, AccountIDs: map[int64]struct{}{}},
			}},
		}}
		svc := NewUserSmartScheduleService(repo, nil, &stubSmartAccountRepo{accounts: []*Account{
			{ID: 11, Platform: PlatformAnthropic},
		}}, nil, nil)
		preview, err := svc.PreviewCopyFromUser(ctx, 99, PlatformAnthropic, 16)
		require.NoError(t, err)
		_, err = svc.CopyFromUser(ctx, 99, PlatformAnthropic, 16, preview.SourceRevision, SmartScheduleCopySlices{Enabled: true})
		require.Error(t, err)
		require.Equal(t, "SMART_SCHEDULE_EMPTY_POOL", infraerrors.Reason(err))
		require.False(t, repo.byUser[99].Policies[PlatformAnthropic].Enabled)
	})

	t.Run("same user is invalid", func(t *testing.T) {
		t.Parallel()
		_, svc, _ := newCopyFromUserFixture()
		_, err := svc.CopyFromUser(ctx, 16, PlatformAnthropic, 16, "rev", SmartScheduleCopySlices{Pool: true})
		require.Error(t, err)
		require.Equal(t, "SMART_SCHEDULE_COPY_INVALID", infraerrors.Reason(err))
	})
}

func newCopyFromUserFixture() (*stubSmartRepo, *UserSmartScheduleService, *copyFromUserCacheRecorder) {
	source := enabledSmartPolicy(11, 3, intPtr(800))
	source.AccountIDs[12] = struct{}{}
	source.Caps[12] = 4
	source.SortOrders = map[int64]int{11: 1, 12: 2}
	source.Paused = map[int64]struct{}{11: {}}
	source.QualityMinSuccessRate = float64Ptr(0.9)
	target := &SmartSchedulePlatformPolicy{
		Enabled:         false,
		CooldownMinutes: 15,
		AccountIDs:      map[int64]struct{}{11: {}, 13: {}},
		Caps:            map[int64]int{11: 9, 13: 1},
		Paused:          map[int64]struct{}{13: {}},
	}
	openai := &SmartSchedulePlatformPolicy{
		Enabled:         true,
		CooldownMinutes: 15,
		AccountIDs:      map[int64]struct{}{21: {}},
		Paused:          map[int64]struct{}{21: {}},
	}
	repo := &stubSmartRepo{byUser: map[int64]*UserSmartScheduleBundle{
		16: {Policies: map[string]*SmartSchedulePlatformPolicy{PlatformAnthropic: source}},
		99: {Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformAnthropic: target,
			PlatformOpenAI:    openai,
		}},
	}}
	cache := &copyFromUserCacheRecorder{}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 11, Platform: PlatformAnthropic},
		{ID: 12, Platform: PlatformAnthropic},
		{ID: 13, Platform: PlatformAnthropic},
		{ID: 21, Platform: PlatformOpenAI},
	}}, nil, nil)
	return repo, svc, cache
}
