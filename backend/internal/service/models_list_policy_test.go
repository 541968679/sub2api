package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayModelDiscoveryIDsForPlatform(t *testing.T) {
	openAI, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini"}, openAI)

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
	require.Equal(t, "gpt-5.5", openAIAgain[0])

	_, ok = GatewayModelDiscoveryIDsForPlatform(PlatformGemini)
	require.False(t, ok)
}

func TestGetGroupModelsListCandidates_UsesGatewayDiscoveryPolicy(t *testing.T) {
	svc := &adminServiceImpl{}

	openAI, err := svc.GetGroupModelsListCandidates(context.Background(), 0, PlatformOpenAI)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini"}, openAI)

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
