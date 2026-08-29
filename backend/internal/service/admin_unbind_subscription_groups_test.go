//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type unbindSubscriptionBindCall struct {
	AccountID int64
	GroupIDs  []int64
}

type unbindSubscriptionAccountRepo struct {
	accountRepoStub
	accounts    []Account
	listErr     error
	lastFilters struct {
		platform    string
		accountType string
		status      string
		search      string
		groupID     int64
		privacyMode string
	}
	bindCalls   []unbindSubscriptionBindCall
	bindErrByID map[int64]error
}

func (s *unbindSubscriptionAccountRepo) ListAllWithFilters(_ context.Context, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, error) {
	s.lastFilters.platform = platform
	s.lastFilters.accountType = accountType
	s.lastFilters.status = status
	s.lastFilters.search = search
	s.lastFilters.groupID = groupID
	s.lastFilters.privacyMode = privacyMode
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.accounts, nil
}

func (s *unbindSubscriptionAccountRepo) BindGroups(_ context.Context, accountID int64, groupIDs []int64) error {
	copied := make([]int64, len(groupIDs))
	copy(copied, groupIDs)
	s.bindCalls = append(s.bindCalls, unbindSubscriptionBindCall{
		AccountID: accountID,
		GroupIDs:  copied,
	})
	if err, ok := s.bindErrByID[accountID]; ok {
		return err
	}
	return nil
}

func unbindRatePtr(v float64) *float64 { return &v }

func TestUnbindSubscriptionGroupsByRate_DropsSubscriptionKeepsStandard(t *testing.T) {
	standard := &Group{ID: 10, Name: "standard-a", SubscriptionType: SubscriptionTypeStandard}
	subscription := &Group{ID: 20, Name: "subscription-b", SubscriptionType: SubscriptionTypeSubscription}
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{{
			ID:             1,
			Name:           "high-rate",
			Platform:       PlatformAnthropic,
			RateMultiplier: unbindRatePtr(1.2),
			GroupIDs:       []int64{10, 20},
			Groups:         []*Group{standard, subscription},
		}},
	}
	groupRepo := &groupRepoStubForAccountBindingValidation{
		groups: map[int64]*Group{10: standard, 20: subscription},
	}
	svc := &adminServiceImpl{accountRepo: repo, groupRepo: groupRepo}

	result, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Matched)
	require.Equal(t, 1, result.WouldApply)
	require.Equal(t, 1, result.Applied)
	require.Equal(t, UnbindSubscriptionActionApplied, result.Accounts[0].Action)
	require.Equal(t, []UnbindSubscriptionGroupRef{{ID: 20, Name: "subscription-b"}}, result.Accounts[0].RemoveGroups)
	require.Equal(t, []UnbindSubscriptionGroupRef{{ID: 10, Name: "standard-a"}}, result.Accounts[0].KeepGroups)
	require.Equal(t, []unbindSubscriptionBindCall{{AccountID: 1, GroupIDs: []int64{10}}}, repo.bindCalls)
	require.Equal(t, "", repo.lastFilters.platform)
	require.Equal(t, "", repo.lastFilters.accountType)
	require.Equal(t, "", repo.lastFilters.status)
	require.Equal(t, "", repo.lastFilters.search)
	require.Equal(t, int64(0), repo.lastFilters.groupID)
	require.Equal(t, "", repo.lastFilters.privacyMode)
}

func TestUnbindSubscriptionGroupsByRate_EqualRateNotMatched(t *testing.T) {
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{{
			ID:             1,
			Name:           "at-threshold",
			Platform:       PlatformAnthropic,
			RateMultiplier: unbindRatePtr(1.0),
			GroupIDs:       []int64{10, 20},
			Groups: []*Group{
				{ID: 10, Name: "standard-a", SubscriptionType: SubscriptionTypeStandard},
				{ID: 20, Name: "subscription-b", SubscriptionType: SubscriptionTypeSubscription},
			},
		}},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	result, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, 0, result.Matched)
	require.Empty(t, result.Accounts)
	require.Empty(t, repo.bindCalls)
}

func TestUnbindSubscriptionGroupsByRate_NilRateTreatedAsOne(t *testing.T) {
	standard := &Group{ID: 10, Name: "standard-a", SubscriptionType: SubscriptionTypeStandard}
	subscription := &Group{ID: 20, Name: "subscription-b", SubscriptionType: SubscriptionTypeSubscription}
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{{
			ID:       2,
			Name:     "nil-rate",
			Platform: PlatformAnthropic,
			Groups:   []*Group{standard, subscription},
			GroupIDs: []int64{10, 20},
		}},
	}
	svc := &adminServiceImpl{
		accountRepo: repo,
		groupRepo: &groupRepoStubForAccountBindingValidation{
			groups: map[int64]*Group{10: standard, 20: subscription},
		},
	}

	atOne, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, 0, atOne.Matched)
	require.Empty(t, repo.bindCalls)

	below, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 0.9,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, below.Matched)
	require.Equal(t, 1.0, below.Accounts[0].Rate)
	require.Equal(t, UnbindSubscriptionActionApplied, below.Accounts[0].Action)
	require.Equal(t, []unbindSubscriptionBindCall{{AccountID: 2, GroupIDs: []int64{10}}}, repo.bindCalls)
}

func TestUnbindSubscriptionGroupsByRate_NoSubscriptionSkipsWrite(t *testing.T) {
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{{
			ID:             3,
			Name:           "standard-only",
			Platform:       PlatformAnthropic,
			RateMultiplier: unbindRatePtr(2.0),
			GroupIDs:       []int64{10},
			Groups:         []*Group{{ID: 10, Name: "standard-a", SubscriptionType: SubscriptionTypeStandard}},
		}},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	result, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Matched)
	require.Equal(t, 0, result.WouldApply)
	require.Equal(t, 1, result.SkippedNoSubscription)
	require.Equal(t, UnbindSubscriptionActionSkipNoSubscription, result.Accounts[0].Action)
	require.Empty(t, repo.bindCalls)
}

func TestUnbindSubscriptionGroupsByRate_EmptyRemainderSkippedUnlessFlag(t *testing.T) {
	subscription := &Group{ID: 20, Name: "subscription-b", SubscriptionType: SubscriptionTypeSubscription}
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{{
			ID:             4,
			Name:           "sub-only",
			Platform:       PlatformAnthropic,
			RateMultiplier: unbindRatePtr(1.5),
			GroupIDs:       []int64{20},
			Groups:         []*Group{subscription},
		}},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	skipped, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, skipped.SkippedWouldBeEmpty)
	require.True(t, skipped.Accounts[0].WouldBeEmpty)
	require.Equal(t, UnbindSubscriptionActionSkipEmpty, skipped.Accounts[0].Action)
	require.Empty(t, repo.bindCalls)

	allowed, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		AllowEmptyGroups:  true,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, allowed.Applied)
	require.Equal(t, UnbindSubscriptionActionApplied, allowed.Accounts[0].Action)
	require.Len(t, repo.bindCalls, 1)
	require.Equal(t, int64(4), repo.bindCalls[0].AccountID)
	require.Empty(t, repo.bindCalls[0].GroupIDs)
	require.NotNil(t, repo.bindCalls[0].GroupIDs)
}

func TestUnbindSubscriptionGroupsByRate_DryRunNeverBinds(t *testing.T) {
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{{
			ID:             5,
			Name:           "preview-me",
			Platform:       PlatformAnthropic,
			RateMultiplier: unbindRatePtr(1.2),
			GroupIDs:       []int64{10, 20},
			Groups: []*Group{
				{ID: 10, Name: "standard-a", SubscriptionType: SubscriptionTypeStandard},
				{ID: 20, Name: "subscription-b", SubscriptionType: SubscriptionTypeSubscription},
			},
		}},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	result, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		DryRun:            true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.WouldApply)
	require.Equal(t, 0, result.Applied)
	require.Equal(t, 0, result.Failed)
	require.Equal(t, UnbindSubscriptionActionPreview, result.Accounts[0].Action)
	require.Empty(t, repo.bindCalls)
}

func TestUnbindSubscriptionGroupsByRate_ApplyUsesPerAccountKeepIDs(t *testing.T) {
	stdA := &Group{ID: 11, Name: "std-a", SubscriptionType: SubscriptionTypeStandard}
	stdB := &Group{ID: 12, Name: "std-b", SubscriptionType: SubscriptionTypeStandard}
	subA := &Group{ID: 21, Name: "sub-a", SubscriptionType: SubscriptionTypeSubscription}
	subB := &Group{ID: 22, Name: "sub-b", SubscriptionType: SubscriptionTypeSubscription}
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{
			{
				ID:             6,
				Name:           "acct-a",
				Platform:       PlatformAnthropic,
				RateMultiplier: unbindRatePtr(1.4),
				GroupIDs:       []int64{11, 21},
				Groups:         []*Group{stdA, subA},
			},
			{
				ID:             7,
				Name:           "acct-b",
				Platform:       PlatformAnthropic,
				RateMultiplier: unbindRatePtr(2.0),
				GroupIDs:       []int64{12, 22},
				Groups:         []*Group{stdB, subB},
			},
		},
	}
	svc := &adminServiceImpl{
		accountRepo: repo,
		groupRepo: &groupRepoStubForAccountBindingValidation{
			groups: map[int64]*Group{11: stdA, 12: stdB, 21: subA, 22: subB},
		},
	}

	result, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		Platform:          PlatformAnthropic,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, PlatformAnthropic, repo.lastFilters.platform)
	require.Equal(t, 2, result.Applied)
	require.Equal(t, []unbindSubscriptionBindCall{
		{AccountID: 6, GroupIDs: []int64{11}},
		{AccountID: 7, GroupIDs: []int64{12}},
	}, repo.bindCalls)
	require.NotEqual(t, repo.bindCalls[0].GroupIDs, repo.bindCalls[1].GroupIDs)
}

func TestUnbindSubscriptionGroupsByRate_ApplyContinuesOnPerAccountError(t *testing.T) {
	stdA := &Group{ID: 11, Name: "std-a", SubscriptionType: SubscriptionTypeStandard}
	stdB := &Group{ID: 12, Name: "std-b", SubscriptionType: SubscriptionTypeStandard}
	subA := &Group{ID: 21, Name: "sub-a", SubscriptionType: SubscriptionTypeSubscription}
	subB := &Group{ID: 22, Name: "sub-b", SubscriptionType: SubscriptionTypeSubscription}
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{
			{
				ID:             8,
				Name:           "fail-me",
				Platform:       PlatformAnthropic,
				RateMultiplier: unbindRatePtr(1.3),
				GroupIDs:       []int64{11, 21},
				Groups:         []*Group{stdA, subA},
			},
			{
				ID:             9,
				Name:           "ok-me",
				Platform:       PlatformAnthropic,
				RateMultiplier: unbindRatePtr(1.3),
				GroupIDs:       []int64{12, 22},
				Groups:         []*Group{stdB, subB},
			},
		},
		bindErrByID: map[int64]error{8: errors.New("bind exploded")},
	}
	svc := &adminServiceImpl{
		accountRepo: repo,
		groupRepo: &groupRepoStubForAccountBindingValidation{
			groups: map[int64]*Group{11: stdA, 12: stdB, 21: subA, 22: subB},
		},
	}

	result, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Failed)
	require.Equal(t, 1, result.Applied)
	require.Equal(t, UnbindSubscriptionActionFailed, result.Accounts[0].Action)
	require.Contains(t, result.Accounts[0].Error, "bind exploded")
	require.Equal(t, UnbindSubscriptionActionApplied, result.Accounts[1].Action)
	require.Len(t, repo.bindCalls, 2)
}

func TestUnbindSubscriptionGroupsByRate_KeepsUnclassifiableGroup(t *testing.T) {
	subscription := &Group{ID: 20, Name: "subscription-b", SubscriptionType: SubscriptionTypeSubscription}
	repo := &unbindSubscriptionAccountRepo{
		accounts: []Account{{
			ID:             10,
			Name:           "nil-edge",
			Platform:       PlatformAnthropic,
			RateMultiplier: unbindRatePtr(1.2),
			GroupIDs:       []int64{99, 20},
			Groups:         []*Group{subscription},
		}},
	}
	svc := &adminServiceImpl{
		accountRepo: repo,
		groupRepo: &groupRepoStubForAccountBindingValidation{
			groups: map[int64]*Group{
				20: subscription,
				99: {ID: 99, Name: "unknown", SubscriptionType: "mystery"},
			},
		},
	}

	result, err := svc.UnbindSubscriptionGroupsByRate(context.Background(), &UnbindSubscriptionGroupsByRateInput{
		MinRateMultiplier: 1.0,
		DryRun:            false,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{99}, repo.bindCalls[0].GroupIDs)
	require.Equal(t, []UnbindSubscriptionGroupRef{{ID: 99}}, result.Accounts[0].KeepGroups)
}
