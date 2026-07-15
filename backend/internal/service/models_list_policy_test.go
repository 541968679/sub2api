package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayModelDiscoveryIDsForPlatform(t *testing.T) {
	openAI, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "grok-4.5"}, openAI)

	antigravity, ok := GatewayModelDiscoveryIDsForPlatform(PlatformAntigravity)
	require.True(t, ok)
	require.Equal(t, []string{
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	}, antigravity)

	openAI[0] = "mutated"
	openAIAgain, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, "gpt-5.6-sol", openAIAgain[0])

	_, ok = GatewayModelDiscoveryIDsForPlatform(PlatformGemini)
	require.False(t, ok)
}

func TestGetGroupModelsListCandidates_UsesGatewayDiscoveryPolicy(t *testing.T) {
	svc := &adminServiceImpl{}

	openAI, err := svc.GetGroupModelsListCandidates(context.Background(), 0, PlatformOpenAI)
	require.NoError(t, err)
	// OpenAI candidates include curated GPT models + Grok text models for custom lists.
	require.Contains(t, openAI, "gpt-5.6-sol")
	require.Contains(t, openAI, "grok-4.5")
	require.Contains(t, openAI, "grok-4.3")

	antigravity, err := svc.GetGroupModelsListCandidates(context.Background(), 0, PlatformAntigravity)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	}, antigravity)
}

func TestExpandGatewayModelDiscoveryCustomList_UpgradesLegacyOpenAIFullList(t *testing.T) {
	expanded := ExpandGatewayModelDiscoveryCustomList(PlatformOpenAI, []string{
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
	})

	require.Equal(t, []string{
		"gpt-5.6-sol",
		"gpt-5.6-terra",
		"gpt-5.6-luna",
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
		"grok-4.5",
	}, expanded)
}

func TestExpandGatewayModelDiscoveryCustomList_KeepsNarrowedOpenAIList(t *testing.T) {
	expanded := ExpandGatewayModelDiscoveryCustomList(PlatformOpenAI, []string{
		"gpt-5.5",
		"gpt-5.4-mini",
	})

	require.Equal(t, []string{"gpt-5.5", "gpt-5.4-mini"}, expanded)
}
