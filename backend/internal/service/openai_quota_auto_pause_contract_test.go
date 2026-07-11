//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShouldAutoPauseOpenAIAccountByQuota_PerWindowContract(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		extra  map[string]any
		global OpsOpenAIAccountQuotaAutoPauseSettings
		paused bool
		window string
	}{
		{
			name: "account 5h threshold reached",
			extra: map[string]any{
				"codex_5h_used_percent":   95.0,
				"auto_pause_5h_threshold": 0.95,
				"codex_5h_reset_at":       now.Add(time.Hour).Format(time.RFC3339),
			},
			paused: true,
			window: "5h",
		},
		{
			name: "global 7d threshold reached",
			extra: map[string]any{
				"codex_7d_used_percent": 91.0,
				"codex_7d_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			},
			global: OpsOpenAIAccountQuotaAutoPauseSettings{DefaultThreshold7d: 0.9},
			paused: true,
			window: "7d",
		},
		{
			name:  "defaults are disabled",
			extra: map[string]any{"codex_5h_used_percent": 100.0, "codex_7d_used_percent": 100.0},
		},
		{
			name: "explicit account disable beats global default",
			extra: map[string]any{
				"codex_5h_used_percent":  99.0,
				"auto_pause_5h_disabled": true,
			},
			global: OpsOpenAIAccountQuotaAutoPauseSettings{DefaultThreshold5h: 0.9},
		},
		{
			name: "expired window does not stay paused",
			extra: map[string]any{
				"codex_5h_used_percent":   99.0,
				"auto_pause_5h_threshold": 0.9,
				"codex_5h_reset_at":       now.Add(-time.Minute).Format(time.RFC3339),
			},
		},
		{
			name: "Spark shadow is outside OpenAI parent auto pause",
			extra: map[string]any{
				"shadow_kind":             "spark",
				"codex_5h_used_percent":   100.0,
				"auto_pause_5h_threshold": 0.9,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: tt.extra}
			if tt.name == "Spark shadow is outside OpenAI parent auto pause" {
				parentID := int64(41)
				account.ParentAccountID = &parentID
				account.QuotaDimension = QuotaDimensionSpark
			}
			ctx := withOpenAIQuotaAutoPauseSettings(context.Background(), tt.global)
			paused, decision := shouldAutoPauseOpenAIAccountByQuota(ctx, account)
			require.Equal(t, tt.paused, paused)
			require.Equal(t, tt.window, decision.window)
		})
	}
}
