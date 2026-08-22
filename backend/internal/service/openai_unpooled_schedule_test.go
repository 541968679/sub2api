//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func unpooledOpenAIAccount(id int64, rate float64, fallbackOnly bool) Account {
	acc := Account{
		ID:                     id,
		Platform:               PlatformOpenAI,
		Type:                   AccountTypeAPIKey,
		Status:                 StatusActive,
		Schedulable:            true,
		Concurrency:            1,
		Priority:               1,
		UpstreamRateMultiplier: testRate(rate),
		GroupIDs:               []int64{9001},
	}
	if fallbackOnly {
		acc.Extra = map[string]any{AccountExtraFallbackOnly: true}
	}
	return acc
}

func unpooledSchedulerSvc(accounts []Account, load map[int64]*AccountLoadInfo, acquire map[int64]bool, sticky map[string]int64, lookup SmartScheduleLookup) (*OpenAIGatewayService, *schedulerTestGatewayCache) {
	cache := &schedulerTestGatewayCache{sessionBindings: sticky}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.LBTopK = 1
	cfg.Gateway.OpenAIScheduler.StickyEscapeEnabled = false
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 4
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 30 * time.Second
	cfg.Gateway.Scheduling.FallbackWaitTimeout = 30 * time.Second
	cfg.Gateway.Scheduling.FallbackMaxWaiting = 4
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: accounts},
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{loadMap: load, acquireResults: acquire}),
		smartScheduleCache: lookup,
	}
	return svc, cache
}

func TestUnpooledSelectAccount_BothAvailablePicksCheap(t *testing.T) {
	// AC1
	cheap := unpooledOpenAIAccount(1, 0.15, false)
	expensive := unpooledOpenAIAccount(2, 1.0, false)
	svc, _ := unpooledSchedulerSvc(
		[]Account{cheap, expensive},
		map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 10}, 2: {AccountID: 2, LoadRate: 10}},
		map[int64]bool{1: true, 2: true},
		nil,
		nil,
	)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(1), selection.Account.ID)
	require.True(t, selection.Acquired)
	require.Nil(t, selection.WaitPlan)
}

func TestUnpooledSelectAccount_CheapFullAcquiresExpensiveNoWaitPlan(t *testing.T) {
	// AC2: topK=1 would previously WaitPlan on the cheap account.
	cheap := unpooledOpenAIAccount(1, 0.15, false)
	expensive := unpooledOpenAIAccount(2, 1.0, false)
	svc, _ := unpooledSchedulerSvc(
		[]Account{cheap, expensive},
		map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 100}, 2: {AccountID: 2, LoadRate: 0}},
		map[int64]bool{1: false, 2: true},
		nil,
		nil,
	)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(2), selection.Account.ID)
	require.True(t, selection.Acquired)
	require.Nil(t, selection.WaitPlan, "must not WaitPlan on the cheap tier")
}

func TestUnpooledSelectAccount_CheapUnschedulablePicksExpensive(t *testing.T) {
	// AC3
	cheap := unpooledOpenAIAccount(1, 0.15, false)
	cheap.Schedulable = false
	expensive := unpooledOpenAIAccount(2, 1.0, false)
	svc, _ := unpooledSchedulerSvc(
		[]Account{cheap, expensive},
		map[int64]*AccountLoadInfo{2: {AccountID: 2, LoadRate: 0}},
		map[int64]bool{2: true},
		nil,
		nil,
	)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), selection.Account.ID)
	require.True(t, selection.Acquired)
}

func TestUnpooledSelectAccount_StickyEscapeToCheap(t *testing.T) {
	// AC4
	cheap := unpooledOpenAIAccount(1, 0.15, false)
	expensive := unpooledOpenAIAccount(2, 1.0, false)
	svc, cache := unpooledSchedulerSvc(
		[]Account{cheap, expensive},
		map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 0}, 2: {AccountID: 2, LoadRate: 20}},
		map[int64]bool{1: true, 2: true},
		map[string]int64{"openai:sess-escape": 2},
		nil,
	)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "sess-escape", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), selection.Account.ID)
	require.True(t, selection.Acquired)
	require.Greater(t, cache.deletedSessions["openai:sess-escape"], 0)
	require.False(t, decision.StickySessionHit)
}

func TestUnpooledSelectAccount_StickyExpensiveCheapFullKeeps(t *testing.T) {
	// AC5
	cheap := unpooledOpenAIAccount(1, 0.15, false)
	expensive := unpooledOpenAIAccount(2, 1.0, false)
	svc, cache := unpooledSchedulerSvc(
		[]Account{cheap, expensive},
		map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 100}, 2: {AccountID: 2, LoadRate: 20}},
		map[int64]bool{1: false, 2: true},
		map[string]int64{"openai:sess-keep": 2},
		nil,
	)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "sess-keep", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), selection.Account.ID)
	require.True(t, decision.StickySessionHit)
	require.Zero(t, cache.deletedSessions["openai:sess-keep"])
}

func TestUnpooledSelectAccount_PreviousResponseNotEscaped(t *testing.T) {
	// AC6
	cheap := unpooledOpenAIAccount(1, 0.15, false)
	expensive := unpooledOpenAIAccount(2, 1.0, false)
	expensive.Extra = map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}
	cheap.Extra = map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}
	svc, _ := unpooledSchedulerSvc(
		[]Account{cheap, expensive},
		map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 0}, 2: {AccountID: 2, LoadRate: 0}},
		map[int64]bool{1: true, 2: true},
		nil,
		nil,
	)
	svc.cfg.Gateway.OpenAIWS.Enabled = true
	svc.cfg.Gateway.OpenAIWS.OAuthEnabled = true
	svc.cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	svc.cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	svc.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 1800
	svc.cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_expensive", expensive.ID, time.Hour))
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "resp_expensive", "sess-prev", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), selection.Account.ID)
	require.Equal(t, openAIAccountScheduleLayerPreviousResponse, decision.Layer)
	require.True(t, decision.StickyPreviousHit)
}

func TestUnpooledSelectAccount_FallbackOnlyStaysPartitioned(t *testing.T) {
	// AC7
	fallbackCheap := unpooledOpenAIAccount(1, 0.15, true)
	primaryExpensive := unpooledOpenAIAccount(2, 1.0, false)
	svc, _ := unpooledSchedulerSvc(
		[]Account{fallbackCheap, primaryExpensive},
		map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 0}, 2: {AccountID: 2, LoadRate: 0}},
		map[int64]bool{1: true, 2: true},
		nil,
		nil,
	)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), selection.Account.ID)
	require.False(t, selection.Account.IsFallbackOnly())
}

func TestPooledSelectAccount_NoStickyEscapeAndCanWaitCheap(t *testing.T) {
	// AC8
	cheap := unpooledOpenAIAccount(1, 0.15, false)
	expensive := unpooledOpenAIAccount(2, 1.0, false)
	lookup := testSmartLookup(PlatformOpenAI, 1, 2)
	svc, cache := unpooledSchedulerSvc(
		[]Account{cheap, expensive},
		map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 0}, 2: {AccountID: 2, LoadRate: 20}},
		map[int64]bool{1: true, 2: true},
		map[string]int64{"openai:sess-pool": 2},
		lookup,
	)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	selection, decision, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "sess-pool", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), selection.Account.ID)
	require.True(t, decision.StickySessionHit)
	require.Zero(t, cache.deletedSessions["openai:sess-pool"])
}

func TestPooledSelectAccount_CheapFullStillWaitPlansCheap(t *testing.T) {
	// AC8: pooled users keep today's cheap-tier WaitPlan.
	cheap := unpooledOpenAIAccount(1, 0.15, false)
	expensive := unpooledOpenAIAccount(2, 1.0, false)
	lookup := testSmartLookup(PlatformOpenAI, 1, 2)
	svc, _ := unpooledSchedulerSvc(
		[]Account{cheap, expensive},
		map[int64]*AccountLoadInfo{1: {AccountID: 1, LoadRate: 100}, 2: {AccountID: 2, LoadRate: 0}},
		map[int64]bool{1: false, 2: true},
		nil,
		lookup,
	)
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	groupID := int64(9001)
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, int64(1), selection.WaitPlan.AccountID)
	require.False(t, selection.Acquired)
}
