package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayModelDiscoveryIDsForPlatform(t *testing.T) {
	openAI, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, OpenAIDisplaySeed(), openAI)

	antigravity, ok := GatewayModelDiscoveryIDsForPlatform(PlatformAntigravity)
	require.True(t, ok)
	require.Equal(t, []string{
		"claude-opus-5-5",
		"claude-opus-5",
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	}, antigravity)

	openAI[0] = "mutated"
	openAIAgain, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, OpenAIModelGPT6Sol, openAIAgain[0])

	gemini, ok := GatewayModelDiscoveryIDsForPlatform(PlatformGemini)
	require.True(t, ok)
	require.Contains(t, gemini, "gemini-2.5-pro")

	anthropic, ok := GatewayModelDiscoveryIDsForPlatform(PlatformAnthropic)
	require.True(t, ok)
	require.Contains(t, anthropic, "claude-opus-5")
}

func TestGetGroupModelsListCandidates_UsesGatewayDiscoveryPolicy(t *testing.T) {
	svc := &adminServiceImpl{}

	openAI, err := svc.GetGroupModelsListCandidates(context.Background(), 0, PlatformOpenAI)
	require.NoError(t, err)
	// Curated GPT + Grok text + domestic coding IDs stay pre-selected.
	require.Contains(t, openAI.DefaultSelected, "gpt-5.6-sol")
	require.Contains(t, openAI.DefaultSelected, OpenAIModelGPT6Astra)
	require.Contains(t, openAI.DefaultSelected, "grok-4.5")
	require.Contains(t, openAI.DefaultSelected, "grok-4.3")
	require.Contains(t, openAI.DefaultSelected, "glm-5.3")
	// Builtin snapshots and the wider domestic set are selectable, unchecked.
	require.Contains(t, openAI.Models, "gpt-image-2")
	require.Contains(t, openAI.Models, "gpt-5.3-codex")
	require.Contains(t, openAI.Models, "glm-4-flash")
	require.Contains(t, openAI.Models, "moonshot-v1-8k")
	require.Contains(t, openAI.Models, "MiniMax-M3")
	require.Contains(t, openAI.Models, "deepseek-r1")
	require.NotContains(t, openAI.DefaultSelected, "gpt-image-2")
	require.NotContains(t, openAI.DefaultSelected, "glm-4-flash")
	require.NotContains(t, openAI.Models, "grok-imagine")

	antigravity, err := svc.GetGroupModelsListCandidates(context.Background(), 0, PlatformAntigravity)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"claude-opus-5-5",
		"claude-opus-5",
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	}, antigravity.DefaultSelected)
	require.Contains(t, antigravity.Models, "gemini-3-pro-high")
	require.Contains(t, antigravity.Models, "claude-fable-5")
	require.NotContains(t, antigravity.DefaultSelected, "gemini-3-pro-high")

	anthropic, err := svc.GetGroupModelsListCandidates(context.Background(), 0, PlatformAnthropic)
	require.NoError(t, err)
	require.Contains(t, anthropic.Models, "claude-opus-5")
	require.Contains(t, anthropic.Models, "claude-sonnet-4-5")
	require.NotContains(t, anthropic.DefaultSelected, "claude-sonnet-4-5")

	gemini, err := svc.GetGroupModelsListCandidates(context.Background(), 0, PlatformGemini)
	require.NoError(t, err)
	require.Contains(t, gemini.Models, "gemini-2.5-pro")
	require.Contains(t, gemini.DefaultSelected, "gemini-2.5-pro")
}

func TestBuildGroupModelsListCandidates_KeepsAccountMappingKeysUnchecked(t *testing.T) {
	got := buildGroupModelsListCandidates(PlatformOpenAI, []string{"my-custom-model"})
	require.Contains(t, got.Models, "my-custom-model")
	require.NotContains(t, got.DefaultSelected, "my-custom-model")
	require.Contains(t, got.DefaultSelected, "gpt-5.5")
}

func TestCollectConcreteModelIDsFromAccounts_SkipsWildcards(t *testing.T) {
	accounts := []Account{{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"custom-a": "gpt-5.4",
				"gpt-*":    "gpt-5.4",
				"  ":       "x",
			},
		},
	}}
	require.ElementsMatch(t, []string{"custom-a"}, collectConcreteModelIDsFromAccounts(accounts))
}

func TestExpandGatewayModelDiscoveryCustomList_UpgradesLegacyOpenAIFullList(t *testing.T) {
	expanded := ExpandGatewayModelDiscoveryCustomList(PlatformOpenAI, []string{
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
	})

	require.Equal(t, OpenAIDisplaySeed(), expanded)
}

func TestExpandGatewayModelDiscoveryCustomList_KeepsNarrowedOpenAIList(t *testing.T) {
	expanded := ExpandGatewayModelDiscoveryCustomList(PlatformOpenAI, []string{
		"gpt-5.5",
		"gpt-5.4-mini",
	})

	require.Equal(t, []string{"gpt-5.5", "gpt-5.4-mini"}, expanded)
}
