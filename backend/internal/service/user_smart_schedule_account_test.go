//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserSmartScheduleService_AddAccountMember(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &stubSmartRepo{}
	accounts := &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
		{ID: 9, Platform: PlatformOpenAI},
	}}
	svc := NewUserSmartScheduleService(repo, stubSmartCache{}, accounts, nil, nil)

	require.NoError(t, svc.AddAccountMember(ctx, 7, 16, PlatformAnthropic))
	policy := repo.bundle.Policies[PlatformAnthropic]
	require.NotNil(t, policy)
	require.False(t, policy.Enabled)
	require.True(t, policy.HasAccount(7))

	err := svc.AddAccountMember(ctx, 7, 16, PlatformOpenAI)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_PLATFORM_MISMATCH")

	require.NoError(t, svc.AddAccountMember(ctx, 9, 16, PlatformAntigravity))
	require.True(t, repo.bundle.Policies[PlatformAntigravity].HasAccount(9))
}

func TestUserSmartScheduleService_RemoveAccountMemberDisablesEmptyPool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
	svc := NewUserSmartScheduleService(repo, stubSmartCache{}, nil, nil, nil)

	require.NoError(t, svc.RemoveAccountMember(ctx, 7, 16, PlatformAnthropic))
	policy := repo.bundle.Policies[PlatformAnthropic]
	require.NotNil(t, policy)
	require.False(t, policy.HasAccount(7))
	require.False(t, policy.Enabled)
}

func TestUserSmartScheduleService_ListAccountMembershipsStampsLookupPlatform(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &stubSmartRepo{bundle: smartBundle(PlatformOpenAI, enabledSmartPolicy(9, 0, nil))}
	pair := &stubPairConcurrency{counts: map[int64]int{9: 2}}
	svc := NewUserSmartScheduleService(repo, nil, nil, nil, pair)

	rows, err := svc.ListAccountMemberships(ctx, 9, PlatformOpenAI)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 2, rows[0].CurrentConcurrency)
	require.Equal(t, []string{PlatformOpenAI}, pair.platforms)
}

func TestUserSmartScheduleService_SetAccountPairAdmissionBatchRequiresMember(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
	svc := NewUserSmartScheduleService(repo, stubSmartCache{}, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, nil, nil)

	_, err := svc.SetAccountPairAdmissionBatch(ctx, 7, PlatformAnthropic, []int64{99}, PairAdmissionPaused)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_UNKNOWN_ACCOUNT")

	results, err := svc.SetAccountPairAdmissionBatch(ctx, 7, PlatformAnthropic, nil, PairAdmissionPaused)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, PairAdmissionPaused, results[0].State)
	require.Equal(t, int64(16), results[0].UserID)
}
