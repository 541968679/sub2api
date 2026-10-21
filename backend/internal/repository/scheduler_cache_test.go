package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFilterSchedulerCredentialsKeepsSeedanceRoutingFields(t *testing.T) {
	filtered := filterSchedulerCredentials(map[string]any{
		"api_key":             "ark-key",
		"base_url":            "https://ark.example.com/api/v3",
		"openai_capabilities": []string{"chat_completions", "seedance"},
		"access_token":        "secret-access-token",
		"refresh_token":       "secret-refresh-token",
	})

	require.Equal(t, "https://ark.example.com/api/v3", filtered["base_url"])
	require.Equal(t, []string{"chat_completions", "seedance"}, filtered["openai_capabilities"])
	require.NotContains(t, filtered, "access_token")
	require.NotContains(t, filtered, "refresh_token")

	account := service.Account{
		ID:          42,
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Credentials: filtered,
	}
	require.True(t, account.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilitySeedance))

	untouched := service.Account{Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey}
	require.True(t, untouched.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityChatCompletions))
	require.False(t, untouched.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilitySeedance))
}

func TestFilterSchedulerCredentialsKeepsSubscriptionPlanType(t *testing.T) {
	filtered := filterSchedulerCredentials(map[string]any{
		"plan_type":     "plus",
		"access_token":  "secret-access-token",
		"refresh_token": "secret-refresh-token",
	})

	require.Equal(t, "plus", filtered["plan_type"])
	require.NotContains(t, filtered, "access_token")
	require.NotContains(t, filtered, "refresh_token")
}

func TestSchedulerMetadataAccountKeepsOpenAISubscriptionIdentity(t *testing.T) {
	account := service.Account{
		ID:       24,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"plan_type":    "plus",
			"access_token": "secret-access-token",
		},
	}

	metadata := buildSchedulerMetadataAccount(account)

	require.True(t, metadata.IsOpenAIChatGPTSubscription())
	require.Empty(t, metadata.GetCredential("access_token"))
}
