//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildSchedulerMetadataAccount_CopiesUpstreamRateMultiplier(t *testing.T) {
	rate := 0.15
	account := service.Account{
		ID:                     7,
		Type:                   service.AccountTypeOAuth,
		UpstreamRateMultiplier: &rate,
	}

	got := buildSchedulerMetadataAccount(account)

	require.NotNil(t, got.UpstreamRateMultiplier)
	require.Equal(t, 0.15, *got.UpstreamRateMultiplier)
}

func TestBuildSchedulerMetadataAccount_KeepsOpenAIWSFlags(t *testing.T) {
	account := service.Account{
		ID:       42,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"openai_oauth_responses_websockets_v2_mode":    service.OpenAIWSIngressModePassthrough,
			"openai_ws_force_http":                         true,
			"mixed_scheduling":                             true,
			"unused_large_field":                           "drop-me",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, true, got.Extra["openai_oauth_responses_websockets_v2_enabled"])
	require.Equal(t, service.OpenAIWSIngressModePassthrough, got.Extra["openai_oauth_responses_websockets_v2_mode"])
	require.Equal(t, true, got.Extra["openai_ws_force_http"])
	require.Equal(t, true, got.Extra["mixed_scheduling"])
	require.Nil(t, got.Extra["unused_large_field"])
}

func TestBuildSchedulerMetadataAccount_KeepsOpenAIClaudeGPTBridgeFields(t *testing.T) {
	account := service.Account{
		ID:       42,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-opus-4-8": "gpt-5.5",
			},
			"access_token": "drop-me",
		},
		Extra: map[string]any{
			"openai_claude_gpt_bridge_enabled": true,
			"unused_large_field":               "drop-me",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, true, got.Extra["openai_claude_gpt_bridge_enabled"])
	require.Equal(t, map[string]any{"claude-opus-4-8": "gpt-5.5"}, got.Credentials["model_mapping"])
	require.Nil(t, got.Credentials["access_token"])
	require.Nil(t, got.Extra["unused_large_field"])
}

func TestBuildSchedulerMetadataAccount_KeepsGrokOpenAIGroupAccessFlag(t *testing.T) {
	account := service.Account{
		ID:       3004,
		Platform: service.PlatformGrok,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"grok_openai_group_access_enabled": true,
			"email":                            "drop-me@example.com",
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, true, got.Extra["grok_openai_group_access_enabled"])
	require.Nil(t, got.Extra["email"])
	require.True(t, got.IsGrokOpenAIGroupAccessEnabled())
}

func TestBuildSchedulerMetadataAccount_KeepsOpenAIQuotaAutoPauseFields(t *testing.T) {
	account := service.Account{
		ID:       88,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent":            95.0,
			"codex_7d_used_percent":            81.0,
			"codex_5h_reset_at":                "2026-07-11T12:00:00Z",
			"codex_7d_reset_after_seconds":     600,
			"codex_usage_updated_at":           "2026-07-11T11:00:00Z",
			"auto_pause_5h_threshold":          0.95,
			"auto_pause_7d_disabled":           true,
			"openai_claude_gpt_bridge_enabled": true,
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, 95.0, got.Extra["codex_5h_used_percent"])
	require.Equal(t, 81.0, got.Extra["codex_7d_used_percent"])
	require.Equal(t, "2026-07-11T12:00:00Z", got.Extra["codex_5h_reset_at"])
	require.Equal(t, 600, got.Extra["codex_7d_reset_after_seconds"])
	require.Equal(t, "2026-07-11T11:00:00Z", got.Extra["codex_usage_updated_at"])
	require.Equal(t, 0.95, got.Extra["auto_pause_5h_threshold"])
	require.Equal(t, true, got.Extra["auto_pause_7d_disabled"])
	require.Equal(t, true, got.Extra["openai_claude_gpt_bridge_enabled"])
}

func TestBuildSchedulerMetadataAccount_KeepsSlimGroupMembership(t *testing.T) {
	account := service.Account{
		ID:       42,
		Platform: service.PlatformAnthropic,
		GroupIDs: []int64{7, 9, 7, 0},
		AccountGroups: []service.AccountGroup{
			{
				AccountID: 42,
				GroupID:   7,
				Priority:  2,
				Account:   &service.Account{ID: 42, Name: "drop-from-metadata"},
				Group:     &service.Group{ID: 7, Name: "drop-from-metadata"},
			},
			{
				AccountID: 42,
				GroupID:   11,
				Priority:  3,
				Group:     &service.Group{ID: 11, Name: "drop-from-metadata"},
			},
			{
				AccountID: 42,
				GroupID:   0,
				Priority:  4,
			},
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, []int64{7, 9, 11}, got.GroupIDs)
	require.Len(t, got.AccountGroups, 2)
	require.Equal(t, int64(42), got.AccountGroups[0].AccountID)
	require.Equal(t, int64(7), got.AccountGroups[0].GroupID)
	require.Equal(t, 2, got.AccountGroups[0].Priority)
	require.Nil(t, got.AccountGroups[0].Account)
	require.Nil(t, got.AccountGroups[0].Group)
	require.Equal(t, int64(11), got.AccountGroups[1].GroupID)
	require.Nil(t, got.Groups)
}

func TestBuildSchedulerMetadataAccount_KeepsUserScheduleFields(t *testing.T) {
	account := service.Account{
		ID:               42,
		Platform:         service.PlatformAnthropic,
		UserScheduleMode: service.UserScheduleModeAllow,
		ScheduleUserIDs:  []int64{16, 0, 42, 16},
		AllowUserIDs:     []int64{16, 0, 42, 16},
		DenyUserIDs:      []int64{7, 7},
		UserConcurrency:  map[int64]int{16: 5, 0: 2, 99: 0, 8: 3},
		UserQualityGates: map[int64]service.QualityHardCloseSettings{
			16: {MaxP50TTFTMs: intPtrForScheduler(1500), MinSuccessSamples: 20, MinTTFTSamples: 10, Condition: service.QualityHardCloseConditionOr},
			0:  {MinSuccessRate: floatPtrForScheduler(0.9)},
		},
		ScheduleUsers: []service.ScheduleUserRef{
			{ID: 16, Email: "drop-from-metadata@example.com"},
		},
	}

	got := buildSchedulerMetadataAccount(account)

	require.Equal(t, service.UserScheduleModeAllow, got.UserScheduleMode)
	require.Equal(t, []int64{16, 42}, got.ScheduleUserIDs)
	require.Equal(t, []int64{16, 42}, got.AllowUserIDs)
	require.Equal(t, []int64{7}, got.DenyUserIDs)
	require.Equal(t, map[int64]int{16: 5, 8: 3}, got.UserConcurrency)
	require.Len(t, got.UserQualityGates, 1)
	require.NotNil(t, got.UserQualityGates[16].MaxP50TTFTMs)
	require.Equal(t, 1500, *got.UserQualityGates[16].MaxP50TTFTMs)
	require.Equal(t, service.QualityHardCloseConditionOr, got.UserQualityGates[16].Condition)
	require.Nil(t, got.ScheduleUsers)
}

func TestDecodeCachedAccount_MissingUserQualityGatesKeepsIdentity(t *testing.T) {
	account, err := decodeCachedAccount(`{"ID":1,"AllowUserIDs":[16],"DenyUserIDs":[7]}`)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Empty(t, account.UserQualityGates)
	require.True(t, account.AllowsScheduleUser(16))
	require.False(t, account.AllowsScheduleUser(7))
	p50 := 9000
	samples := int64(20)
	rate := 0.01
	stats := &service.AccountQualityStats{P50TTFTMs: &p50, TTFTSamples: samples, SuccessCount: 1, ErrorCount: 20, SuccessRate: &rate}
	require.False(t, account.QualityGateBlocksUser(16, stats))
	require.True(t, account.AdmitsScheduleUser(16, stats))
}

func intPtrForScheduler(v int) *int { return &v }

func floatPtrForScheduler(v float64) *float64 { return &v }
