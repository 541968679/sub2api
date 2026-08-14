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
	require.Equal(t, []int64{16, 42}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].DenyUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].UserConcurrency)
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
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].DenyUserIDs)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[2].DenyUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].AllowUserIDs)
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
	require.Empty(t, repo.syncScheduleCalls[7].AllowUserIDs)
	require.Empty(t, repo.syncScheduleCalls[7].DenyUserIDs)
	require.Empty(t, repo.syncScheduleCalls[7].UserConcurrency)
}

func TestAdminService_UpdateAccount_UserScheduleIndependentListsAndCaps(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {ID: 1, Name: "acc", Status: StatusActive, UserConcurrency: map[int64]int{7: 3}},
		},
		existingUserIDs: map[int64]bool{16: true, 42: true, 7: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	allow := []int64{16}
	deny := []int64{42}
	caps := []UserConcurrencyEntry{{UserID: 16, MaxConcurrency: 5}}

	updated, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name:              "acc",
		AllowUserIDs:      &allow,
		DenyUserIDs:       &deny,
		UserConcurrencies: &caps,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Equal(t, []int64{42}, repo.syncScheduleCalls[1].DenyUserIDs)
	require.Equal(t, map[int64]int{16: 5}, repo.syncScheduleCalls[1].UserConcurrency)
}

func TestAdminService_UpdateAccount_UserScheduleLegacyAllowDoesNotClearCaps(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {
				ID: 1, Name: "acc", Status: StatusActive,
				UserConcurrency: map[int64]int{16: 5},
			},
		},
		existingUserIDs: map[int64]bool{16: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	mode := UserScheduleModeAllow
	ids := []int64{16}

	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name:             "acc",
		UserScheduleMode: &mode,
		ScheduleUserIDs:  &ids,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].DenyUserIDs)
	require.Equal(t, map[int64]int{16: 5}, repo.syncScheduleCalls[1].UserConcurrency)
}

func TestAdminService_UpdateAccount_UserScheduleRestoreDefaultClearsFour(t *testing.T) {
	t.Parallel()

	p50 := 1500
	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {
				ID: 1, Name: "acc", Status: StatusActive,
				AllowUserIDs:    []int64{16},
				DenyUserIDs:     []int64{42},
				UserConcurrency: map[int64]int{16: 5},
				UserQualityGates: map[int64]QualityHardCloseSettings{
					16: {MaxP50TTFTMs: &p50, MinSuccessSamples: 20, MinTTFTSamples: 10, Condition: QualityHardCloseConditionOr},
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	emptyIDs := []int64{}
	emptyCaps := []UserConcurrencyEntry{}
	emptyGates := []UserQualityGateEntry{}

	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name:              "acc",
		AllowUserIDs:      &emptyIDs,
		DenyUserIDs:       &emptyIDs,
		UserConcurrencies: &emptyCaps,
		UserQualityGates:  &emptyGates,
	})
	require.NoError(t, err)
	require.Empty(t, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].DenyUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].UserConcurrency)
	require.Empty(t, repo.syncScheduleCalls[1].UserQualityGates)
}

func TestAdminService_UpdateAccount_UserScheduleConcurrencyPatch(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {
				ID: 1, Name: "acc", Status: StatusActive,
				AllowUserIDs:    []int64{16},
				UserConcurrency: map[int64]int{16: 5},
			},
		},
		existingUserIDs: map[int64]bool{16: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	zero := 0

	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name: "acc",
		UserConcurrencyPatch: &UserConcurrencyPatch{UserID: 16, MaxConcurrency: &zero},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].UserConcurrency)
}

func TestAdminService_BulkUpdateAccounts_UserScheduleAllowOverwriteKeepsCaps(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {ID: 1, DenyUserIDs: []int64{9}, UserConcurrency: map[int64]int{16: 4}},
			2: {ID: 2, DenyUserIDs: []int64{8}, UserConcurrency: map[int64]int{7: 2}},
		},
		existingUserIDs: map[int64]bool{16: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	allow := []int64{16}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:   []int64{1, 2},
		AllowUserIDs: &allow,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Equal(t, []int64{9}, repo.syncScheduleCalls[1].DenyUserIDs)
	require.Equal(t, map[int64]int{16: 4}, repo.syncScheduleCalls[1].UserConcurrency)
	require.Equal(t, []int64{8}, repo.syncScheduleCalls[2].DenyUserIDs)
	require.Equal(t, map[int64]int{7: 2}, repo.syncScheduleCalls[2].UserConcurrency)
}

func TestAdminService_BulkUpdateAccounts_UserScheduleRejectsConcurrencyOverwrite(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{}
	svc := &adminServiceImpl{accountRepo: repo}
	caps := []UserConcurrencyEntry{{UserID: 16, MaxConcurrency: 5}}

	_, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:        []int64{1},
		UserConcurrencies: &caps,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot overwrite per-user concurrency")
	require.Empty(t, repo.syncScheduleCalls)
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

func TestAdminService_UpdateAccount_UserQualityGateReplaceAndPatch(t *testing.T) {
	t.Parallel()

	p50 := 1500
	rate := 0.9
	samples := 20
	ttftSamples := 10
	cond := QualityHardCloseConditionOr
	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {ID: 1, Name: "acc", Status: StatusActive, AllowUserIDs: []int64{16}},
		},
		existingUserIDs: map[int64]bool{16: true, 42: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	gates := []UserQualityGateEntry{{
		UserID:            16,
		MaxP50TTFTMs:      &p50,
		MinSuccessRate:    &rate,
		MinSuccessSamples: &samples,
		MinTTFTSamples:    &ttftSamples,
		Condition:         &cond,
	}}

	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name:             "acc",
		UserQualityGates: &gates,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.NotNil(t, repo.syncScheduleCalls[1].UserQualityGates[16].MaxP50TTFTMs)
	require.Equal(t, 1500, *repo.syncScheduleCalls[1].UserQualityGates[16].MaxP50TTFTMs)
	require.Equal(t, QualityHardCloseConditionOr, repo.syncScheduleCalls[1].UserQualityGates[16].Condition)

	clearP50 := (*int)(nil)
	_, err = svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name: "acc",
		UserQualityGatePatch: &UserQualityGatePatch{UserID: 16, MaxP50TTFTMs: clearP50},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].UserQualityGates)
}

func TestAdminService_UpdateAccount_UserQualityGateConditionOnlyIsNotAGate(t *testing.T) {
	t.Parallel()

	cond := QualityHardCloseConditionOr
	samples := 20
	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {ID: 1, Name: "acc", Status: StatusActive, AllowUserIDs: []int64{16}},
		},
		existingUserIDs: map[int64]bool{16: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	gates := []UserQualityGateEntry{{
		UserID:            16,
		MinSuccessSamples: &samples,
		Condition:         &cond,
	}}

	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name:             "acc",
		UserQualityGates: &gates,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Empty(t, repo.syncScheduleCalls[1].UserQualityGates)
}

func TestAdminService_UpdateAccount_UserQualityGateConflict(t *testing.T) {
	t.Parallel()

	p50 := 1500
	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {ID: 1, Name: "acc", Status: StatusActive},
		},
		existingUserIDs: map[int64]bool{16: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	gates := []UserQualityGateEntry{{UserID: 16, MaxP50TTFTMs: &p50}}
	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name:                 "acc",
		UserQualityGates:     &gates,
		UserQualityGatePatch: &UserQualityGatePatch{UserID: 16, MaxP50TTFTMs: &p50},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be set together")
	require.Empty(t, repo.syncScheduleCalls)
}

func TestAdminService_UpdateAccount_UserScheduleLegacyAllowDoesNotClearGates(t *testing.T) {
	t.Parallel()

	p50 := 1800
	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {
				ID: 1, Name: "acc", Status: StatusActive,
				UserQualityGates: map[int64]QualityHardCloseSettings{
					16: {MaxP50TTFTMs: &p50, MinSuccessSamples: 20, MinTTFTSamples: 10, Condition: QualityHardCloseConditionOr},
				},
			},
		},
		existingUserIDs: map[int64]bool{16: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	mode := UserScheduleModeAllow
	ids := []int64{16}

	_, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Name:             "acc",
		UserScheduleMode: &mode,
		ScheduleUserIDs:  &ids,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.NotNil(t, repo.syncScheduleCalls[1].UserQualityGates[16].MaxP50TTFTMs)
	require.Equal(t, 1800, *repo.syncScheduleCalls[1].UserQualityGates[16].MaxP50TTFTMs)
}

func TestAdminService_BulkUpdateAccounts_UserScheduleRejectsQualityGateOverwrite(t *testing.T) {
	t.Parallel()

	p50 := 1500
	repo := &accountRepoStubForBulkUpdate{}
	svc := &adminServiceImpl{accountRepo: repo}
	gates := []UserQualityGateEntry{{UserID: 16, MaxP50TTFTMs: &p50}}

	_, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:       []int64{1},
		UserQualityGates: &gates,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot overwrite per-user quality gates")
	require.Empty(t, repo.syncScheduleCalls)
}

func TestAdminService_BulkUpdateAccounts_UserScheduleAllowOverwriteKeepsGates(t *testing.T) {
	t.Parallel()

	p50 := 1600
	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			1: {
				ID: 1, DenyUserIDs: []int64{9},
				UserQualityGates: map[int64]QualityHardCloseSettings{
					16: {MaxP50TTFTMs: &p50, MinSuccessSamples: 20, MinTTFTSamples: 10, Condition: QualityHardCloseConditionOr},
				},
			},
		},
		existingUserIDs: map[int64]bool{16: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	allow := []int64{16}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:   []int64{1},
		AllowUserIDs: &allow,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, []int64{16}, repo.syncScheduleCalls[1].AllowUserIDs)
	require.Equal(t, []int64{9}, repo.syncScheduleCalls[1].DenyUserIDs)
	require.NotNil(t, repo.syncScheduleCalls[1].UserQualityGates[16].MaxP50TTFTMs)
	require.Equal(t, 1600, *repo.syncScheduleCalls[1].UserQualityGates[16].MaxP50TTFTMs)
}
