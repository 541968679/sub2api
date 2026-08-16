//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestGatewayService_StickyUserScheduleDeniedClearsBinding(t *testing.T) {
	t.Parallel()

	denied := &Account{
		ID:               1,
		Platform:         PlatformAnthropic,
		Type:             AccountTypeAPIKey,
		Status:           StatusActive,
		Schedulable:      true,
		Concurrency:      5,
		AllowUserIDs: []int64{99},
	}
	allowed := &Account{
		ID:               2,
		Platform:         PlatformAnthropic,
		Type:             AccountTypeAPIKey,
		Status:           StatusActive,
		Schedulable:      true,
		Concurrency:      5,
		Priority:         1,
		AllowUserIDs: []int64{16},
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{*denied, *allowed},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}
	repo.listPlatformFunc = func(ctx context.Context, platform string) ([]Account, error) {
		return repo.accounts, nil
	}

	cache := &mockGatewayCacheForPlatform{
		sessionBindings: map[string]int64{"sticky": 1},
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true

	svc := &GatewayService{
		accountRepo:        repo,
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sticky", "claude-3-5-sonnet-20241022", nil, "", int64(16))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Equal(t, int64(2), result.Account.ID)
	require.Equal(t, 1, cache.deletedSessions["sticky"])
	require.Equal(t, int64(2), cache.sessionBindings["sticky"])
}

func TestGatewayService_StickyUserQualityGateClearsBinding(t *testing.T) {
	t.Parallel()

	p50 := 1000
	blocked := &Account{
		ID:          1,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 5,
		UserQualityGates: map[int64]QualityHardCloseSettings{
			16: fillUserQualityGateDefaults(QualityHardCloseSettings{MaxP50TTFTMs: &p50, MinTTFTSamples: 10}),
		},
	}
	fallback := &Account{
		ID:          2,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 5,
		Priority:    1,
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{*blocked, *fallback},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}
	repo.listPlatformFunc = func(ctx context.Context, platform string) ([]Account, error) {
		return repo.accounts, nil
	}

	cache := &mockGatewayCacheForPlatform{
		sessionBindings: map[string]int64{"sticky": 1},
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true

	svc := &GatewayService{
		accountRepo:        repo,
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
		qualityLiveCache: &liveQualityCacheStub{
			byID: map[int64]*AccountQualityStats{
				1: liveQualityStats(4000, 12, 20, 0, 1),
			},
		},
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sticky", "claude-3-5-sonnet-20241022", nil, "", int64(16))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Equal(t, int64(2), result.Account.ID)
	require.Equal(t, 1, cache.deletedSessions["sticky"])
	require.Equal(t, int64(2), cache.sessionBindings["sticky"])
	require.True(t, blocked.Schedulable)
	require.Nil(t, blocked.TempUnschedulableUntil)
}

func TestSelectAccount_UserQualityGateFilters(t *testing.T) {
	t.Parallel()

	p50 := 1000
	gated := Account{
		ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		UserQualityGates: map[int64]QualityHardCloseSettings{
			16: fillUserQualityGateDefaults(QualityHardCloseSettings{MaxP50TTFTMs: &p50, MinTTFTSamples: 10}),
		},
	}
	open := Account{
		ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 10,
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{gated, open},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       &mockGatewayCacheForPlatform{},
		cfg:         testConfig(),
		qualityLiveCache: &liveQualityCacheStub{
			byID: map[int64]*AccountQualityStats{
				1: liveQualityStats(4000, 12, 20, 0, 1),
			},
		},
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	ctx = context.WithValue(ctx, ctxkey.UserID, int64(16))
	account, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "claude-3-5-sonnet-20241022", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(2), account.ID, "quality-gated user must skip the better-priority gated account")
	require.True(t, gated.Schedulable)
	require.Nil(t, gated.TempUnschedulableUntil)

	ctxOther := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	ctxOther = context.WithValue(ctxOther, ctxkey.UserID, int64(7))
	other, err := svc.selectAccountForModelWithPlatform(ctxOther, nil, "", "claude-3-5-sonnet-20241022", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.NotNil(t, other)
	require.Equal(t, int64(1), other.ID, "user without a gate should still take the better-priority gated account")
}

func TestSelectAccount_UserScheduleAllowFilters(t *testing.T) {
	t.Parallel()

	allowHit := Account{
		ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 2,
		AllowUserIDs: []int64{16},
	}
	allowMiss := Account{
		ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		AllowUserIDs: []int64{99},
	}
	unrestricted := Account{
		ID: 3, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 10,
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{allowHit, allowMiss, unrestricted},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       &mockGatewayCacheForPlatform{},
		cfg:         testConfig(),
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	ctx = context.WithValue(ctx, ctxkey.UserID, int64(16))
	account, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "claude-3-5-sonnet-20241022", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(1), account.ID, "best-priority allow miss must be skipped; allow hit should win")
}

func TestSelectAccount_UserScheduleDenyFilters(t *testing.T) {
	t.Parallel()

	denied := Account{
		ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		DenyUserIDs: []int64{16},
	}
	unrestricted := Account{
		ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 10,
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{denied, unrestricted},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       &mockGatewayCacheForPlatform{},
		cfg:         testConfig(),
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	ctx = context.WithValue(ctx, ctxkey.UserID, int64(16))
	account, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "claude-3-5-sonnet-20241022", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(2), account.ID, "denied user must skip the better-priority deny account")

	ctxOther := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	ctxOther = context.WithValue(ctxOther, ctxkey.UserID, int64(7))
	other, err := svc.selectAccountForModelWithPlatform(ctxOther, nil, "", "claude-3-5-sonnet-20241022", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.NotNil(t, other)
	require.Equal(t, int64(1), other.ID, "non-denied user should still take the better-priority deny account")
}

func TestSelectAccount_UserScheduleUserIDZeroSkipsRestricted(t *testing.T) {
	t.Parallel()

	allowOnly := Account{
		ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		AllowUserIDs: []int64{16},
	}
	denyListed := Account{
		ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 2,
		DenyUserIDs: []int64{99},
	}
	unrestricted := Account{
		ID: 3, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 10,
	}
	p50 := 1000
	gateOnly := Account{
		ID: 4, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 0,
		UserQualityGates: map[int64]QualityHardCloseSettings{
			16: fillUserQualityGateDefaults(QualityHardCloseSettings{MaxP50TTFTMs: &p50}),
		},
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{allowOnly, denyListed, unrestricted, gateOnly},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}

	svc := &GatewayService{
		accountRepo: repo,
		cache:       &mockGatewayCacheForPlatform{},
		cfg:         testConfig(),
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	account, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "claude-3-5-sonnet-20241022", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(3), account.ID, "userID=0 must skip allow/deny/quality-gate-only accounts and keep unrestricted")
}

func TestOpenAISelectAccount_UserScheduleStickyEscape(t *testing.T) {
	// Not parallel: SelectAccountWithScheduler uses package-level singleflight.

	groupID := int64(101201)
	denied := Account{
		ID: 38101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
		DenyUserIDs: []int64{16},
	}
	allowed := Account{
		ID: 38102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 5, GroupIDs: []int64{groupID},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:user-schedule": 38101}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{denied, allowed}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "user-schedule", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(38102), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.Equal(t, 1, cache.deletedSessions["openai:user-schedule"])
}

func TestOpenAISelectAccount_UserQualityGateStickyEscape(t *testing.T) {
	groupID := int64(101209)
	p50 := 1000
	blocked := Account{
		ID: 38901, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
		UserQualityGates: map[int64]QualityHardCloseSettings{
			16: fillUserQualityGateDefaults(QualityHardCloseSettings{MaxP50TTFTMs: &p50, MinTTFTSamples: 10}),
		},
	}
	allowed := Account{
		ID: 38902, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 5, GroupIDs: []int64{groupID},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:quality-gate": 38901}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{blocked, allowed}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
		qualityLiveCache: &liveQualityCacheStub{
			byID: map[int64]*AccountQualityStats{
				38901: liveQualityStats(4000, 12, 20, 0, 1),
			},
		},
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "quality-gate", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(38902), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, decision.Layer)
	require.Equal(t, 1, cache.deletedSessions["openai:quality-gate"])
	require.True(t, blocked.Schedulable)
	require.Nil(t, blocked.TempUnschedulableUntil)
}

func TestOpenAISelectAccount_UserScheduleUnrestrictedStickyStillHits(t *testing.T) {
	// Not parallel: SelectAccountWithScheduler uses package-level singleflight.

	groupID := int64(101202)
	sticky := Account{
		ID: 38201, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 10, GroupIDs: []int64{groupID},
	}
	other := Account{
		ID: 38202, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:unrestricted": 38201}}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{sticky, other}},
		cache:              cache,
		cfg:                &config.Config{},
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "unrestricted", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(38201), selection.Account.ID)
	require.True(t, decision.StickySessionHit)
	require.Zero(t, cache.deletedSessions["openai:unrestricted"])
}

func TestOpenAISelectAccount_UserScheduleDenyFiltersWhenAdvancedSchedulerDisabled(t *testing.T) {
	// Local Codex/OpenAI traffic uses this path: advanced scheduler off + load-batch on.
	groupID := int64(101203)
	denied := Account{
		ID: 38301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
		DenyUserIDs: []int64{1},
	}
	allowed := Account{
		ID: 38302, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 5, GroupIDs: []int64{groupID},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{denied, allowed}},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("false"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(1))
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(38302), selection.Account.ID, "denied user must skip the better-priority deny account on the load-awareness fallback")
}

func TestOpenAISelectAccount_UserScheduleDenyOnlyAccountWhenAdvancedSchedulerDisabled(t *testing.T) {
	groupID := int64(101204)
	denied := Account{
		ID: 38401, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
		DenyUserIDs: []int64{1},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{denied}},
		cache:              &schedulerTestGatewayCache{},
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("false"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(1))
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, selection)
}

func TestOpenAISelectAccount_UserScheduleDenyStickyClearedWhenAdvancedSchedulerDisabled(t *testing.T) {
	groupID := int64(101205)
	denied := Account{
		ID: 38501, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 0, GroupIDs: []int64{groupID},
		DenyUserIDs: []int64{1},
	}
	allowed := Account{
		ID: 38502, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Concurrency: 1, Priority: 5, GroupIDs: []int64{groupID},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:load-aware-deny": 38501}}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{denied, allowed}},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("false"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(1))
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "load-aware-deny", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(38502), selection.Account.ID)
	require.Equal(t, 1, cache.deletedSessions["openai:load-aware-deny"])
}

func TestSelectAccount_UserSchedulePairConcurrencyFullReselects(t *testing.T) {
	t.Parallel()

	full := Account{
		ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		AllowUserIDs:    []int64{16},
		UserConcurrency: map[int64]int{16: 1},
	}
	other := Account{
		ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 10,
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{full, other},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &GatewayService{
		accountRepo: repo,
		cache:       &mockGatewayCacheForPlatform{},
		cfg:         cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{
			pairCounts: map[int64]int{1: 1},
		}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "", "claude-3-5-sonnet-20241022", nil, "", int64(16))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Equal(t, int64(2), result.Account.ID)
	require.Nil(t, result.WaitPlan)
}

func TestSelectAccount_StickyUserSchedulePairFullKeepsPin(t *testing.T) {
	t.Parallel()

	full := &Account{
		ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 5,
		AllowUserIDs:    []int64{16},
		UserConcurrency: map[int64]int{16: 1},
	}
	other := &Account{
		ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		AllowUserIDs: []int64{16},
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{*full, *other},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky": 1}}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &GatewayService{
		accountRepo: repo,
		cache:       cache,
		cfg:         cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{
			pairCounts:         map[int64]int{1: 1},
			pairAcquireResults: map[int64]bool{1: false},
		}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sticky", "claude-3-5-sonnet-20241022", nil, "", int64(16))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), result.Account.ID)
	require.Nil(t, result.WaitPlan)
	require.Zero(t, cache.deletedSessions["sticky"])
}

func TestOpenAISelectAccount_UserSchedulePairConcurrencyFullReselects(t *testing.T) {
	groupID := int64(101206)
	full := Account{
		ID: 38601, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Priority: 0, GroupIDs: []int64{groupID},
		AllowUserIDs:    []int64{16},
		UserConcurrency: map[int64]int{16: 1},
	}
	other := Account{
		ID: 38602, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Priority: 5, GroupIDs: []int64{groupID},
	}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: []Account{full, other}},
		cache:            &schedulerTestGatewayCache{},
		cfg:              &config.Config{},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			pairCounts: map[int64]int{38601: 1},
		}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(38602), selection.Account.ID)
	require.Nil(t, selection.WaitPlan)
}

func TestOpenAISelectAccount_StickyUserSchedulePairFullKeepsPin(t *testing.T) {
	groupID := int64(101207)
	full := Account{
		ID: 38701, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Priority: 0, GroupIDs: []int64{groupID},
		UserConcurrency: map[int64]int{16: 1},
	}
	other := Account{
		ID: 38702, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Priority: 5, GroupIDs: []int64{groupID},
	}
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:pair-full": 38701}}
	svc := &OpenAIGatewayService{
		accountRepo:      schedulerTestOpenAIAccountRepo{accounts: []Account{full, other}},
		cache:            cache,
		cfg:              &config.Config{},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			pairCounts:         map[int64]int{38701: 1},
			pairAcquireResults: map[int64]bool{38701: false},
		}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "pair-full", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(38702), selection.Account.ID)
	require.False(t, decision.StickySessionHit)
	require.Zero(t, cache.deletedSessions["openai:pair-full"])
	require.Nil(t, selection.WaitPlan)
}

func TestSelectAccount_UserScheduleRoutedPairFullFallsThrough(t *testing.T) {
	t.Parallel()

	groupID := int64(88)
	full := Account{
		ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		UserConcurrency: map[int64]int{16: 1},
	}
	other := Account{
		ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 10,
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{full, other},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo: &mockGroupRepoForGateway{
			groups: map[int64]*Group{
				groupID: {
					ID:                  groupID,
					Platform:            PlatformAnthropic,
					Status:              StatusActive,
					Hydrated:            true,
					ModelRoutingEnabled: true,
					ModelRouting: map[string][]int64{
						"claude-3-5-sonnet-20241022": {1},
					},
				},
			},
		},
		cache: &mockGatewayCacheForPlatform{},
		cfg:   cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{
			pairCounts:         map[int64]int{1: 1},
			pairAcquireResults: map[int64]bool{1: false},
		}),
	}

	result, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "claude-3-5-sonnet-20241022", nil, "", int64(16))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Equal(t, int64(2), result.Account.ID)
	require.Nil(t, result.WaitPlan)
}

func TestSelectAccount_SmartScheduleClosedPoolIgnoresLegacyDeny(t *testing.T) {
	t.Parallel()

	deniedButPooled := Account{
		ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 1,
		DenyUserIDs: []int64{16},
	}
	outsidePool := Account{
		ID: 2, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 5, Priority: 10,
	}
	repo := &mockAccountRepoForPlatform{
		accounts:     []Account{deniedButPooled, outsidePool},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		copied := repo.accounts[i]
		repo.accountsByID[copied.ID] = &copied
	}
	lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(1, 0, nil))}
	svc := &GatewayService{
		accountRepo:        repo,
		cache:              &mockGatewayCacheForPlatform{},
		cfg:                testConfig(),
		smartScheduleCache: lookup,
	}

	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	ctx = context.WithValue(ctx, ctxkey.UserID, int64(16))
	account, err := svc.selectAccountForModelWithPlatform(ctx, nil, "", "claude-3-5-sonnet-20241022", nil, PlatformAnthropic)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(1), account.ID, "enabled smart pool must ignore legacy deny and reject accounts outside the pool")
}

func TestOpenAISelectAccount_LoadBatchDisabledPairFullReselects(t *testing.T) {
	groupID := int64(101208)
	full := Account{
		ID: 38801, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Priority: 0, GroupIDs: []int64{groupID},
		UserConcurrency: map[int64]int{16: 1},
	}
	other := Account{
		ID: 38802, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		Concurrency: 5, Priority: 5, GroupIDs: []int64{groupID},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{full, other}},
		cache:       &schedulerTestGatewayCache{},
		cfg:         cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			pairCounts:         map[int64]int{38801: 1},
			pairAcquireResults: map[int64]bool{38801: false},
		}),
	}

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	selection, err := svc.SelectAccountWithLoadAwareness(ctx, &groupID, "", "gpt-5.1", nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(38802), selection.Account.ID)
	require.Nil(t, selection.WaitPlan)
}
