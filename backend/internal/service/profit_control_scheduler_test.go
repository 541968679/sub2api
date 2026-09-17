//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type profitControlGroupRepoStub struct {
	GroupRepository
	group *Group
}

func (r profitControlGroupRepoStub) GetByID(context.Context, int64) (*Group, error) {
	return r.group, nil
}

func profitControlSchedulerAccounts(groupID int64) []Account {
	expensive := 2.0
	cheap := 0.2
	return []Account{
		{
			ID:             88101,
			Platform:       PlatformOpenAI,
			Type:           AccountTypeAPIKey,
			Status:         StatusActive,
			Schedulable:    true,
			Concurrency:    1,
			Priority:       0,
			RateMultiplier: &expensive,
			GroupIDs:       []int64{groupID},
		},
		{
			ID:             88102,
			Platform:       PlatformOpenAI,
			Type:           AccountTypeAPIKey,
			Status:         StatusActive,
			Schedulable:    true,
			Concurrency:    1,
			Priority:       0,
			RateMultiplier: &cheap,
			GroupIDs:       []int64{groupID},
		},
	}
}

func selectWithProfitControlGroup(t *testing.T, group *Group, load map[int64]*AccountLoadInfo) int64 {
	t.Helper()
	groupID := int64(10901)
	accounts := profitControlSchedulerAccounts(groupID)
	byID := map[int64]*Account{accounts[0].ID: &accounts[0], accounts[1].ID: &accounts[1]}
	snap := &SchedulerSnapshotService{
		cache: &openAISnapshotCacheStub{
			snapshotAccounts: []*Account{&accounts[0], &accounts[1]},
			accountsByID:     byID,
		},
	}
	if group != nil {
		group.ID = groupID
		snap.groupRepo = profitControlGroupRepoStub{group: group}
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              &schedulerTestGatewayCache{},
		cfg:                newSchedulerTestSubscriptionPriorityConfig(),
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		schedulerSnapshot:  snap,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{loadMap: load}),
	}
	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	return selection.Account.ID
}

func TestSelectAccountWithScheduler_DefaultOffProfitControlKeepsBaseline(t *testing.T) {
	// Expensive account is idle; cheap is saturated. Baseline (no group / default-off
	// group) must keep the expensive account. If the gate ignored Enabled=false,
	// expensive (U=2.0 vs D*(1-0.5-0.2)=0.3) would be dropped and cheap would win.
	load := map[int64]*AccountLoadInfo{
		88101: {AccountID: 88101, LoadRate: 0},
		88102: {AccountID: 88102, LoadRate: 100},
	}
	baseline := selectWithProfitControlGroup(t, nil, load)
	require.Equal(t, int64(88101), baseline)

	defaultOff := selectWithProfitControlGroup(t, &Group{
		RateMultiplier:       1,
		ProfitControlEnabled: false,
		ProfitMinMargin:      0.5,
		ProfitSafetyBuffer:   0.2,
	}, load)
	require.Equal(t, baseline, defaultOff)
}
