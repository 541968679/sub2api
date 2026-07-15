package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsGrokTextModel(t *testing.T) {
	t.Parallel()
	require.True(t, IsGrokTextModel("grok-4.5"))
	require.True(t, IsGrokTextModel("Grok-4.3"))
	require.True(t, IsGrokTextModel("grok"))
	require.True(t, IsGrokTextModel("composer-2.5"))
	require.False(t, IsGrokTextModel("gpt-5.6-sol"))
	require.False(t, IsGrokTextModel("grok-imagine-image"))
	require.False(t, IsGrokTextModel("grok-imagine-video-1.5"))
	require.False(t, IsGrokTextModel(""))
}

func TestResolveOpenAICompatibleSchedulePlatform(t *testing.T) {
	t.Parallel()
	platform, requireAccess := ResolveOpenAICompatibleSchedulePlatform(PlatformOpenAI, "grok-4.5")
	require.Equal(t, PlatformGrok, platform)
	require.True(t, requireAccess)

	platform, requireAccess = ResolveOpenAICompatibleSchedulePlatform(PlatformOpenAI, "gpt-5.6-sol")
	require.Equal(t, PlatformOpenAI, platform)
	require.False(t, requireAccess)

	platform, requireAccess = ResolveOpenAICompatibleSchedulePlatform(PlatformGrok, "grok-4.5")
	require.Equal(t, PlatformGrok, platform)
	require.False(t, requireAccess)

	platform, requireAccess = ResolveOpenAICompatibleSchedulePlatform(PlatformOpenAI, "grok-imagine-image")
	require.Equal(t, PlatformOpenAI, platform)
	require.False(t, requireAccess)
}

func TestIsGrokOpenAIGroupAccessEnabled_DefaultOffAndTypes(t *testing.T) {
	t.Parallel()
	oauth := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth}
	require.False(t, oauth.IsGrokOpenAIGroupAccessEnabled())

	oauth.Extra = map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true}
	require.True(t, oauth.IsGrokOpenAIGroupAccessEnabled())

	apiKey := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true},
	}
	require.True(t, apiKey.IsGrokOpenAIGroupAccessEnabled())

	// OpenAI account cannot use the Grok access flag.
	openai := &Account{
		Platform: PlatformOpenAI,
		Extra:    map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true},
	}
	require.False(t, openai.IsGrokOpenAIGroupAccessEnabled())
}

func TestIsOpenAIAccountEligible_GrokOpenAIGroupAccess(t *testing.T) {
	t.Parallel()
	optIn := &Account{
		ID:          1,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Extra:       map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-4.5": "grok-4.5"},
		},
	}
	optOut := &Account{
		ID:          2,
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Extra:       map[string]any{},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-4.5": "grok-4.5"},
		},
	}

	require.True(t, isOpenAIAccountEligibleForScheduleRequest(optIn, openAIAccountRequestEligibility{
		Platform:                     PlatformGrok,
		RequestedModel:               "grok-4.5",
		RequireGrokOpenAIGroupAccess: true,
	}))
	require.False(t, isOpenAIAccountEligibleForScheduleRequest(optOut, openAIAccountRequestEligibility{
		Platform:                     PlatformGrok,
		RequestedModel:               "grok-4.5",
		RequireGrokOpenAIGroupAccess: true,
	}))
	// Native Grok-group scheduling does not require the access flag.
	require.True(t, isOpenAIAccountEligibleForScheduleRequest(optOut, openAIAccountRequestEligibility{
		Platform:       PlatformGrok,
		RequestedModel: "grok-4.5",
	}))
}

func TestCollectAndMergeGrokOpenAIGroupAccessModels(t *testing.T) {
	t.Parallel()
	accounts := []Account{
		{
			Platform:    PlatformGrok,
			Status:      StatusActive,
			Schedulable: true,
			Extra:       map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true},
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"grok-4.5":           "grok-4.5",
					"grok-imagine-image": "grok-imagine-image",
				},
			},
		},
		{
			Platform:    PlatformGrok,
			Status:      StatusActive,
			Schedulable: true,
			// opt-out
			Credentials: map[string]any{
				"model_mapping": map[string]any{"grok-4.3": "grok-4.3"},
			},
		},
	}
	ids := CollectGrokOpenAIGroupAccessModelIDs(accounts)
	require.Contains(t, ids, "grok-4.5")
	require.NotContains(t, ids, "grok-imagine-image")
	require.NotContains(t, ids, "grok-4.3")

	merged := MergeModelIDsPreferFirst([]string{"gpt-5.6-sol", "gpt-5.5"}, []string{"grok-4.5", "gpt-5.6-sol"})
	require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.5", "grok-4.5"}, merged)
}

func TestAdminServiceValidateAccountGroupBindings_GrokOpenAIGroupAccess(t *testing.T) {
	ctx := context.Background()
	repo := &groupRepoStubForAccountBindingValidation{
		groups: map[int64]*Group{
			1: {ID: 1, Name: "OpenAI", Platform: PlatformOpenAI},
			2: {ID: 2, Name: "Grok", Platform: PlatformGrok},
			3: {ID: 3, Name: "Anthropic", Platform: PlatformAnthropic},
		},
	}
	svc := &adminServiceImpl{groupRepo: repo}

	// Default off: only Grok groups.
	require.NoError(t, svc.validateAccountGroupBindings(ctx, PlatformGrok, nil, []int64{2}))
	require.Error(t, svc.validateAccountGroupBindings(ctx, PlatformGrok, nil, []int64{1}))

	// Opt-in: OpenAI + Grok ok; Anthropic still rejected.
	require.NoError(t, svc.validateAccountGroupBindings(ctx, PlatformGrok, map[string]any{
		AccountExtraGrokOpenAIGroupAccessEnabled: true,
	}, []int64{1, 2}))
	require.Error(t, svc.validateAccountGroupBindings(ctx, PlatformGrok, map[string]any{
		AccountExtraGrokOpenAIGroupAccessEnabled: true,
	}, []int64{3}))

	// OAuth and API key types both use the same extra validation path.
	require.NoError(t, svc.validateAccountGroupBindings(ctx, PlatformGrok, map[string]any{
		AccountExtraGrokOpenAIGroupAccessEnabled: true,
	}, []int64{1}))
}
