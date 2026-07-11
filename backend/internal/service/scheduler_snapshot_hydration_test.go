//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type snapshotHydrationCache struct {
	snapshot []*Account
	accounts map[int64]*Account
}

func (c *snapshotHydrationCache) GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	return c.snapshot, true, nil
}

func (c *snapshotHydrationCache) SetSnapshot(ctx context.Context, bucket SchedulerBucket, accounts []Account) error {
	return nil
}

func (c *snapshotHydrationCache) GetAccount(ctx context.Context, accountID int64) (*Account, error) {
	if c.accounts == nil {
		return nil, nil
	}
	return c.accounts[accountID], nil
}

func (c *snapshotHydrationCache) SetAccount(ctx context.Context, account *Account) error {
	return nil
}

func (c *snapshotHydrationCache) DeleteAccount(ctx context.Context, accountID int64) error {
	return nil
}

func (c *snapshotHydrationCache) UpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}

func (c *snapshotHydrationCache) TryLockBucket(ctx context.Context, bucket SchedulerBucket, ttl time.Duration) (bool, error) {
	return true, nil
}

func (c *snapshotHydrationCache) UnlockBucket(ctx context.Context, bucket SchedulerBucket) error {
	return nil
}

func (c *snapshotHydrationCache) ListBuckets(ctx context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}

func (c *snapshotHydrationCache) GetOutboxWatermark(ctx context.Context) (int64, error) {
	return 0, nil
}

func (c *snapshotHydrationCache) SetOutboxWatermark(ctx context.Context, id int64) error {
	return nil
}

func TestOpenAISelectAccountWithLoadAwareness_HydratesSelectedAccountFromSchedulerSnapshot(t *testing.T) {
	cache := &snapshotHydrationCache{
		snapshot: []*Account{
			{
				ID:          1,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						"gpt-4": "gpt-4",
					},
				},
			},
		},
		accounts: map[int64]*Account{
			1: {
				ID:          1,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				Credentials: map[string]any{
					"api_key":       "sk-live",
					"model_mapping": map[string]any{"gpt-4": "gpt-4"},
				},
			},
		},
	}

	schedulerSnapshot := NewSchedulerSnapshotService(cache, nil, nil, nil, nil)
	groupID := int64(2)
	svc := &OpenAIGatewayService{
		schedulerSnapshot: schedulerSnapshot,
		cache:             &stubGatewayCache{},
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil {
		t.Fatalf("expected selected account")
	}
	if got := selection.Account.GetOpenAIApiKey(); got != "sk-live" {
		t.Fatalf("expected hydrated api key, got %q", got)
	}
}

func TestGatewaySelectAccountWithLoadAwareness_HydratesSelectedAccountFromSchedulerSnapshot(t *testing.T) {
	cache := &snapshotHydrationCache{
		snapshot: []*Account{
			{
				ID:          9,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
			},
		},
		accounts: map[int64]*Account{
			9: {
				ID:          9,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				Credentials: map[string]any{
					"api_key": "anthropic-live-key",
				},
			},
		},
	}

	schedulerSnapshot := NewSchedulerSnapshotService(cache, nil, nil, nil, nil)
	svc := &GatewayService{
		schedulerSnapshot: schedulerSnapshot,
		cache:             &mockGatewayCacheForPlatform{},
		cfg:               testConfig(),
	}

	result, err := svc.SelectAccountWithLoadAwareness(context.Background(), nil, "", "claude-3-5-sonnet-20241022", nil, "", 0)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if result == nil || result.Account == nil {
		t.Fatalf("expected selected account")
	}
	if got := result.Account.GetCredential("api_key"); got != "anthropic-live-key" {
		t.Fatalf("expected hydrated api key, got %q", got)
	}
}

func TestGatewaySelectAccountWithLoadAwareness_HydratesStickyWaitAccountFromSchedulerSnapshot(t *testing.T) {
	const (
		accountID = int64(9)
	)
	groupID := int64(20)
	cache := &snapshotHydrationCache{
		snapshot: []*Account{
			{
				ID:          accountID,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
			},
		},
		accounts: map[int64]*Account{
			accountID: {
				ID:          accountID,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Priority:    1,
				Credentials: map[string]any{"api_key": "anthropic-live-key"},
			},
		},
	}

	schedulerSnapshot := NewSchedulerSnapshotService(cache, nil, nil, nil, nil)
	gatewayCache := &mockGatewayCacheForPlatform{
		sessionBindings: map[string]int64{"sticky-session": accountID},
	}
	groupRepo := &mockGroupRepoForGateway{
		groups: map[int64]*Group{
			groupID: {
				ID:                  groupID,
				Platform:            PlatformAnthropic,
				Status:              StatusActive,
				Hydrated:            true,
				ModelRoutingEnabled: true,
				ModelRouting: map[string][]int64{
					"claude-3-5-sonnet-20241022": {accountID},
				},
			},
		},
	}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 1

	svc := &GatewayService{
		schedulerSnapshot: schedulerSnapshot,
		groupRepo:         groupRepo,
		cache:             gatewayCache,
		cfg:               cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{
			acquireResults: map[int64]bool{accountID: false},
			waitCounts:     map[int64]int{accountID: 0},
		}),
	}

	result, err := svc.SelectAccountWithLoadAwareness(
		context.Background(),
		&groupID,
		"sticky-session",
		"claude-3-5-sonnet-20241022",
		nil,
		"",
		0,
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.WaitPlan)
	require.Equal(t, accountID, result.Account.ID)
	require.Equal(t, "anthropic-live-key", result.Account.GetCredential("api_key"))
}
