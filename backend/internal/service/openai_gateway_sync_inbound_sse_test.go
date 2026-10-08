package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldForceSyncInboundUpstreamSSE(t *testing.T) {
	t.Parallel()

	apiKey := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk", "base_url": "https://token-bits.example/v1"},
	}
	official := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk"},
	}
	oauth := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	grok := &Account{
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "xai", "base_url": "https://api.x.ai"},
	}

	tests := []struct {
		name         string
		account      *Account
		clientStream bool
		want         bool
	}{
		{name: "missing extra stays sync on custom base", account: apiKey, want: false},
		{name: "missing extra stays sync on official", account: official, want: false},
		{name: "explicit false stays sync", account: &Account{
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "sk", "base_url": "https://token-bits.example/v1"},
			Extra:       map[string]any{extraKeySyncInboundUpstreamSSE: false},
		}, want: false},
		{name: "explicit true forces sse", account: &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra:    map[string]any{extraKeySyncInboundUpstreamSSE: true},
		}, want: true},
		{name: "string true forces sse", account: &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra:    map[string]any{extraKeySyncInboundUpstreamSSE: "true"},
		}, want: true},
		{name: "string false stays sync", account: &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra:    map[string]any{extraKeySyncInboundUpstreamSSE: "false"},
		}, want: false},
		{name: "inbound stream skips gate", account: &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Extra:    map[string]any{extraKeySyncInboundUpstreamSSE: true},
		}, clientStream: true, want: false},
		{name: "oauth never forced", account: oauth, want: false},
		{name: "grok never forced", account: grok, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, shouldForceSyncInboundUpstreamSSE(tt.account, tt.clientStream))
		})
	}
}
