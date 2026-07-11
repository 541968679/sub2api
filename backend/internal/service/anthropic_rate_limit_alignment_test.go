//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type anthropicAlignmentRepo struct {
	mockAccountRepoForGemini
	rateLimitCalls int
	rateLimitReset time.Time
	modelCalls     int
	modelScope     string
	modelReset     time.Time
	extra          map[string]any
}

func (r *anthropicAlignmentRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	r.rateLimitCalls++
	r.rateLimitReset = resetAt
	return nil
}

func (r *anthropicAlignmentRepo) SetModelRateLimit(_ context.Context, _ int64, scope string, resetAt time.Time) error {
	r.modelCalls++
	r.modelScope = scope
	r.modelReset = resetAt
	return nil
}

func (r *anthropicAlignmentRepo) UpdateExtra(_ context.Context, _ int64, extra map[string]any) error {
	r.extra = extra
	return nil
}

func TestHandle429AnthropicWithoutResetUsesBoundedFallback(t *testing.T) {
	repo := &anthropicAlignmentRepo{}
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	account := &Account{ID: 45, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	before := time.Now()
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"Extra usage required"}}`))
	after := time.Now()

	require.Equal(t, 1, repo.rateLimitCalls)
	require.False(t, repo.rateLimitReset.Before(before.Add(time.Second)))
	require.False(t, repo.rateLimitReset.After(after.Add(maxRateLimit429FallbackCooldown)))
}

func TestAnthropicFable429CreatesOnlyModelFamilyCooldown(t *testing.T) {
	repo := &anthropicAlignmentRepo{}
	svc := NewRateLimitService(repo, nil, nil, nil, nil)
	reset := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	headers := http.Header{
		"Anthropic-Ratelimit-Unified-7d_oi-Status":      {"rejected"},
		"Anthropic-Ratelimit-Unified-7d_oi-Utilization": {"1.0"},
		"Anthropic-Ratelimit-Unified-7d_oi-Reset":       {json.Number(reset.Unix()).String()},
	}
	account := &Account{ID: 9, Platform: PlatformAnthropic, Type: AccountTypeOAuth}

	svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, nil)

	require.Equal(t, 1, repo.modelCalls)
	require.Equal(t, anthropicFableRateLimitKey, repo.modelScope)
	require.Equal(t, reset.Unix(), repo.modelReset.Unix())
	require.Zero(t, repo.rateLimitCalls, "Fable exhaustion must not cool down the whole account")
	require.Equal(t, 1.0, repo.extra["passive_usage_7d_oi_utilization"])
}

func TestAnthropicFableFamilyLimitDoesNotBlockOtherModels(t *testing.T) {
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	account := &Account{Platform: PlatformAnthropic, Extra: map[string]any{
		modelRateLimitsKey: map[string]any{
			anthropicFableRateLimitKey: map[string]any{"rate_limit_reset_at": future},
		},
	}}

	require.True(t, account.isModelRateLimitedWithContext(context.Background(), "claude-fable-5[1m]"))
	require.True(t, account.isModelRateLimitedWithContext(context.Background(), "Claude-Fable-5-20260601"))
	require.False(t, account.isModelRateLimitedWithContext(context.Background(), "claude-sonnet-4-6"))
	require.False(t, account.isModelRateLimitedWithContext(context.Background(), "claude-opus-4-8"))
}

func TestClaudeUsageResponseAndPassiveWindowExposeFable(t *testing.T) {
	reset := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	var response ClaudeUsageResponse
	require.NoError(t, json.Unmarshal([]byte(`{"seven_day_overage_included":{"utilization":87,"resets_at":"`+reset.Format(time.RFC3339)+`"}}`), &response))

	usage := (&AccountUsageService{}).buildUsageInfo(&response, nil)
	require.NotNil(t, usage.SevenDayFable)
	require.Equal(t, 87.0, usage.SevenDayFable.Utilization)

	passive := buildPassiveUsageWindow(map[string]any{"u": 0.87, "r": float64(reset.Unix())}, "u", "r")
	require.NotNil(t, passive)
	require.Equal(t, 87.0, passive.Utilization)
	require.Equal(t, reset.Unix(), passive.ResetsAt.Unix())
}
