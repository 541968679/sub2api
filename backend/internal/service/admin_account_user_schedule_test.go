//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func ptrUserScheduleMode(mode string) *string { return &mode }

func TestAdminService_UpdateAccount_UserScheduleAllowRequiresUsers(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {ID: 1, Name: "acc", Status: StatusActive, UserScheduleMode: UserScheduleModeUnrestricted},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	mode := UserScheduleModeAllow
	ids := []int64{}

	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		UserScheduleMode: &mode,
		ScheduleUserIDs:  &ids,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one user id")
	require.Nil(t, repo.lastUpdated)
	require.Empty(t, repo.syncScheduleCalls)
}

func TestAdminService_UpdateAccount_UserScheduleAllowWrites(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {ID: 1, Name: "acc", Status: StatusActive, UserScheduleMode: UserScheduleModeUnrestricted},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	mode := UserScheduleModeAllow
	ids := []int64{16, 42}

	updated, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name:             "acc",
		UserScheduleMode: &mode,
		ScheduleUserIDs:  &ids,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, UserScheduleModeAllow, updated.UserScheduleMode)
	require.Equal(t, []int64{16, 42}, repo.syncScheduleCalls[1])
}

func TestAdminService_BulkUpdateAccounts_UserScheduleOmitDoesNotWrite(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{}
	svc := &adminServiceImpl{accountRepo: repo}
	status := "active"

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs: []int64{1, 2},
		Status:     status,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, repo.syncScheduleCalls)
	require.Nil(t, repo.lastBulkUpdates.UserScheduleMode)
}

func TestAdminService_BulkUpdateAccounts_UserScheduleOverwrite(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{}
	svc := &adminServiceImpl{accountRepo: repo}
	mode := UserScheduleModeDeny
	ids := []int64{16}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:       []int64{1, 2},
		UserScheduleMode: &mode,
		ScheduleUserIDs:  &ids,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1])
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[2])
	require.NotNil(t, repo.lastBulkUpdates.UserScheduleMode)
	require.Equal(t, UserScheduleModeDeny, *repo.lastBulkUpdates.UserScheduleMode)
}

func TestAdminService_BulkUpdateAccounts_UserScheduleUnrestrictedClearsJoin(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{}
	svc := &adminServiceImpl{accountRepo: repo}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:       []int64{7},
		UserScheduleMode: ptrUserScheduleMode(UserScheduleModeUnrestricted),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, repo.syncScheduleCalls[7])
	require.Empty(t, repo.syncScheduleCalls[7])
}

func TestAdminService_ValidateUserScheduleWrite_UnknownUserRejected(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		existingUserIDs: map[int64]bool{16: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	_, _, err := svc.validateUserScheduleWrite(context.Background(), UserScheduleModeAllow, []int64{16, 99})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown user id")
}
