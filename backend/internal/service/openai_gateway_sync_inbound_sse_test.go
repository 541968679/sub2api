package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestShouldForceSyncInboundUpstreamSSE(t *testing.T) {
	t.Parallel()

	apiKeyOfficial := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk"},
	}
	apiKeyOfficialExplicit := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk", "base_url": "https://api.openai.com/v1"},
	}
	apiKeyCustom := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk", "base_url": "https://token-bits.example/v1"},
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
		cfg          *config.Config
		clientStream bool
		want         bool
	}{
		{name: "official empty base keeps S2", account: apiKeyOfficial, want: false},
		{name: "official explicit host keeps S2", account: apiKeyOfficialExplicit, want: false},
		{name: "custom base uses SSE", account: apiKeyCustom, want: true},
		{name: "mode off disables custom", account: apiKeyCustom, cfg: &config.Config{Gateway: config.GatewayConfig{OpenAISyncInboundUpstreamSSEMode: "off"}}, want: false},
		{name: "mode all enables official", account: apiKeyOfficial, cfg: &config.Config{Gateway: config.GatewayConfig{OpenAISyncInboundUpstreamSSEMode: "all"}}, want: true},
		{name: "extra false overrides custom", account: &Account{
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "sk", "base_url": "https://token-bits.example/v1"},
			Extra:       map[string]any{extraKeySyncInboundUpstreamSSE: false},
		}, want: false},
		{name: "extra true overrides official", account: &Account{
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "sk"},
			Extra:       map[string]any{extraKeySyncInboundUpstreamSSE: true},
		}, want: true},
		{name: "inbound stream skips gate", account: apiKeyCustom, clientStream: true, want: false},
		{name: "oauth never forced here", account: oauth, want: false},
		{name: "grok never forced", account: grok, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, shouldForceSyncInboundUpstreamSSE(tt.account, tt.cfg, tt.clientStream))
		})
	}
}

func TestAccountHasCustomOpenAIBaseURL_UsesCredentialNotResolvedDefault(t *testing.T) {
	t.Parallel()
	official := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk"},
	}
	require.Equal(t, "https://api.openai.com", official.GetOpenAIBaseURL())
	require.False(t, accountHasCustomOpenAIBaseURL(official))
	require.True(t, isOfficialOpenAIAPIHost(""))
	require.True(t, isOfficialOpenAIAPIHost("api.openai.com/v1"))
	require.False(t, isOfficialOpenAIAPIHost("https://token-bits.example"))
}
