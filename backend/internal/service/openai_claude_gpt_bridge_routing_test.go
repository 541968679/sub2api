//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/stretchr/testify/require"
)

type claudeGPTBridgeRouteRepoStub struct {
	AccountRepository
	accounts            []Account
	listErr             error
	listCalls           int
	setRateLimitedCalls int
}

func (r *claudeGPTBridgeRouteRepoStub) ListByGroup(_ context.Context, _ int64) ([]Account, error) {
	r.listCalls++
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]Account, len(r.accounts))
	copy(out, r.accounts)
	return out, nil
}

func (r *claudeGPTBridgeRouteRepoStub) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.setRateLimitedCalls++
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			reset := resetAt
			r.accounts[i].RateLimitResetAt = &reset
		}
	}
	return nil
}

func newClaudeGPTBridgeRouteAccount(id int64, mutate ...func(*Account)) Account {
	account := Account{
		ID:          id,
		Name:        "bridge-account",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Extra:       map[string]any{"openai_claude_gpt_bridge_enabled": true},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"claude-opus-4-8": "gpt-5.5"},
		},
	}
	for _, fn := range mutate {
		fn(&account)
	}
	return account
}

func resolveClaudeGPTBridgeRouteForTest(repo AccountRepository, model string) ClaudeGPTBridgeRouteDecision {
	svc := &OpenAIGatewayService{accountRepo: repo}
	groupID := int64(7)
	return svc.ResolveClaudeGPTBridgeRoute(context.Background(), &groupID, model)
}

func TestResolveClaudeGPTBridgeRoute_NotConfiguredWithoutCandidates(t *testing.T) {
	cases := []struct {
		name     string
		accounts []Account
	}{
		{name: "no accounts in group", accounts: nil},
		{name: "bridge switch disabled", accounts: []Account{newClaudeGPTBridgeRouteAccount(1, func(a *Account) {
			a.Extra = map[string]any{"openai_claude_gpt_bridge_enabled": false}
		})}},
		{name: "no explicit mapping for model", accounts: []Account{newClaudeGPTBridgeRouteAccount(1, func(a *Account) {
			a.Credentials = map[string]any{"model_mapping": map[string]any{"claude-haiku-4-5": "gpt-5.4-mini"}}
		})}},
		{name: "self mapping is not a bridge", accounts: []Account{newClaudeGPTBridgeRouteAccount(1, func(a *Account) {
			a.Credentials = map[string]any{"model_mapping": map[string]any{"claude-opus-4-8": "claude-opus-4-8"}}
		})}},
		{name: "non openai platform account", accounts: []Account{newClaudeGPTBridgeRouteAccount(1, func(a *Account) {
			a.Platform = PlatformAntigravity
		})}},
		{name: "inactive account is not a candidate", accounts: []Account{newClaudeGPTBridgeRouteAccount(1, func(a *Account) {
			a.Status = StatusError
		})}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &claudeGPTBridgeRouteRepoStub{accounts: tt.accounts}
			decision := resolveClaudeGPTBridgeRouteForTest(repo, "claude-opus-4-8")
			require.Equal(t, ClaudeGPTBridgeRouteNotConfigured, decision.State)
			require.Zero(t, decision.CandidateCount)
			require.Nil(t, decision.RetryAt)
		})
	}
}

func TestResolveClaudeGPTBridgeRoute_ReadyWithSchedulableCandidate(t *testing.T) {
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{newClaudeGPTBridgeRouteAccount(1)}}

	decision := resolveClaudeGPTBridgeRouteForTest(repo, "claude-opus-4-8")

	require.Equal(t, ClaudeGPTBridgeRouteReady, decision.State)
	require.Equal(t, 1, decision.CandidateCount)
	require.Equal(t, 1, decision.SchedulableCount)
	require.Zero(t, decision.RateLimitedCount)
	require.Nil(t, decision.RetryAt)
}

func TestResolveClaudeGPTBridgeRoute_RateLimitedOnlyCandidate(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Minute)
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{newClaudeGPTBridgeRouteAccount(1, func(a *Account) {
		a.RateLimitResetAt = &resetAt
	})}}

	decision := resolveClaudeGPTBridgeRouteForTest(repo, "claude-opus-4-8")

	require.Equal(t, ClaudeGPTBridgeRouteRateLimited, decision.State)
	require.Equal(t, 1, decision.CandidateCount)
	require.Zero(t, decision.SchedulableCount)
	require.Equal(t, 1, decision.RateLimitedCount)
	require.NotNil(t, decision.RetryAt)
	require.WithinDuration(t, resetAt, *decision.RetryAt, time.Second)
}

func TestResolveClaudeGPTBridgeRoute_ReadyWhenAnyCandidateSchedulable(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Minute)
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{
		newClaudeGPTBridgeRouteAccount(1, func(a *Account) { a.RateLimitResetAt = &resetAt }),
		newClaudeGPTBridgeRouteAccount(2),
	}}

	decision := resolveClaudeGPTBridgeRouteForTest(repo, "claude-opus-4-8")

	require.Equal(t, ClaudeGPTBridgeRouteReady, decision.State)
	require.Equal(t, 2, decision.CandidateCount)
	require.Equal(t, 1, decision.SchedulableCount)
	require.Equal(t, 1, decision.RateLimitedCount)
}

func TestResolveClaudeGPTBridgeRoute_RateLimitedUsesEarliestReset(t *testing.T) {
	laterReset := time.Now().Add(30 * time.Minute)
	earlierReset := time.Now().Add(10 * time.Minute)
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{
		newClaudeGPTBridgeRouteAccount(1, func(a *Account) { a.RateLimitResetAt = &laterReset }),
		newClaudeGPTBridgeRouteAccount(2, func(a *Account) { a.RateLimitResetAt = &earlierReset }),
	}}

	decision := resolveClaudeGPTBridgeRouteForTest(repo, "claude-opus-4-8")

	require.Equal(t, ClaudeGPTBridgeRouteRateLimited, decision.State)
	require.Equal(t, 2, decision.CandidateCount)
	require.Equal(t, 2, decision.RateLimitedCount)
	require.NotNil(t, decision.RetryAt)
	require.WithinDuration(t, earlierReset, *decision.RetryAt, time.Second)
}

func TestResolveClaudeGPTBridgeRoute_UnavailableOnMixedBlockers(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Minute)
	tempUntil := time.Now().Add(20 * time.Minute)
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{
		newClaudeGPTBridgeRouteAccount(1, func(a *Account) { a.RateLimitResetAt = &resetAt }),
		newClaudeGPTBridgeRouteAccount(2, func(a *Account) { a.TempUnschedulableUntil = &tempUntil }),
	}}

	decision := resolveClaudeGPTBridgeRouteForTest(repo, "claude-opus-4-8")

	require.Equal(t, ClaudeGPTBridgeRouteUnavailable, decision.State)
	require.Equal(t, 2, decision.CandidateCount)
	require.Zero(t, decision.SchedulableCount)
	require.Equal(t, 1, decision.RateLimitedCount)
}

func TestResolveClaudeGPTBridgeRoute_UnavailableOnNonRateLimitBlockers(t *testing.T) {
	overloadUntil := time.Now().Add(5 * time.Minute)
	tempUntil := time.Now().Add(5 * time.Minute)
	expiredAt := time.Now().Add(-time.Minute)

	cases := []struct {
		name   string
		mutate func(*Account)
	}{
		{name: "overloaded", mutate: func(a *Account) { a.OverloadUntil = &overloadUntil }},
		{name: "temp unschedulable", mutate: func(a *Account) { a.TempUnschedulableUntil = &tempUntil }},
		{name: "admin paused", mutate: func(a *Account) { a.Schedulable = false }},
		{name: "expired with auto pause", mutate: func(a *Account) {
			a.AutoPauseOnExpired = true
			a.ExpiresAt = &expiredAt
		}},
		{name: "rate limited and admin paused is not pure rate limit", mutate: func(a *Account) {
			reset := time.Now().Add(10 * time.Minute)
			a.RateLimitResetAt = &reset
			a.Schedulable = false
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{newClaudeGPTBridgeRouteAccount(1, tt.mutate)}}
			decision := resolveClaudeGPTBridgeRouteForTest(repo, "claude-opus-4-8")
			require.Equal(t, ClaudeGPTBridgeRouteUnavailable, decision.State)
			require.Equal(t, 1, decision.CandidateCount)
			require.Zero(t, decision.SchedulableCount)
		})
	}
}

func TestResolveClaudeGPTBridgeRoute_ProbeErrorOnRepoFailure(t *testing.T) {
	repo := &claudeGPTBridgeRouteRepoStub{listErr: errors.New("db down")}

	decision := resolveClaudeGPTBridgeRouteForTest(repo, "claude-opus-4-8")

	require.Equal(t, ClaudeGPTBridgeRouteProbeError, decision.State)
	require.Nil(t, decision.RetryAt)
}

func TestResolveClaudeGPTBridgeRoute_NotConfiguredOnMissingGroupOrModel(t *testing.T) {
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{newClaudeGPTBridgeRouteAccount(1)}}
	svc := &OpenAIGatewayService{accountRepo: repo}

	decision := svc.ResolveClaudeGPTBridgeRoute(context.Background(), nil, "claude-opus-4-8")
	require.Equal(t, ClaudeGPTBridgeRouteNotConfigured, decision.State)

	groupID := int64(7)
	decision = svc.ResolveClaudeGPTBridgeRoute(context.Background(), &groupID, "  ")
	require.Equal(t, ClaudeGPTBridgeRouteNotConfigured, decision.State)

	require.Zero(t, repo.listCalls, "missing group/model must not query the account repository")
}

// TestClaudeGPTBridgeTwoRequestRateLimitRegression 是 2026-07-10 调查文档第 10 节
// 要求的关键两请求回归：唯一 bridge 账号上游 429 usage_limit_reached 写入限流状态后，
// 限流窗口内的下一次相同路由请求必须得到 rate_limited 决策（429 语义），
// 绝不允许被误判为 not_configured 而回落 native Antigravity 池。
func TestClaudeGPTBridgeTwoRequestRateLimitRegression(t *testing.T) {
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{newClaudeGPTBridgeRouteAccount(1)}}
	rateLimitSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

	// 请求 1：真实 bridge 请求的上游 429 usage_limit_reached 效果——写入账号限流状态。
	resetsAt := time.Now().Add(2 * time.Minute)
	account := repo.accounts[0]
	respBody := []byte(`{"error":{"type":"usage_limit_reached","message":"You have hit your usage limit.","resets_at":` +
		strconv.FormatInt(resetsAt.Unix(), 10) + `}}`)
	rateLimitSvc.HandleUpstreamError(context.Background(), &account, http.StatusTooManyRequests, http.Header{}, respBody)

	require.Equal(t, 1, repo.setRateLimitedCalls, "upstream 429 must persist the rate limit state")
	require.NotNil(t, repo.accounts[0].RateLimitResetAt)
	require.True(t, repo.accounts[0].RateLimitResetAt.After(time.Now()))

	// 请求 2：限流窗口内立即重试。诊断必须返回 rate_limited + 未来的 RetryAt，
	// 且不发起任何上游请求。
	upstream := &httpUpstreamSequenceRecorder{}
	svc := &OpenAIGatewayService{accountRepo: repo, httpUpstream: upstream}
	groupID := int64(7)

	decision := svc.ResolveClaudeGPTBridgeRoute(context.Background(), &groupID, "claude-opus-4-8")

	require.Equal(t, ClaudeGPTBridgeRouteRateLimited, decision.State,
		"a temporarily rate-limited bridge must never be classified as not_configured")
	require.Equal(t, 1, decision.CandidateCount)
	require.Equal(t, 1, decision.RateLimitedCount)
	require.NotNil(t, decision.RetryAt)
	require.WithinDuration(t, resetsAt, *decision.RetryAt, 2*time.Second)
	require.Zero(t, upstream.callCount, "route diagnosis must not send upstream requests")
}

