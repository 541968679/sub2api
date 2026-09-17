package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCNCodingModelIDsForOpenAIGroupAccess(t *testing.T) {
	ids := CNCodingModelIDsForOpenAIGroupAccess()
	require.Contains(t, ids, "glm-5.3")
	require.Contains(t, ids, "kimi-k2.5")
	require.Contains(t, ids, "deepseek-v4-flash")
	require.Contains(t, ids, "MiniMax-M2.5")
}

func TestDefaultModelIDsForCNPlatform(t *testing.T) {
	require.Contains(t, DefaultModelIDsForCNPlatform(PlatformZhipu), "glm-5.3")
	require.Contains(t, DefaultModelIDsForCNPlatform(PlatformKimi), "kimi-k3")
	require.Contains(t, DefaultModelIDsForCNPlatform(PlatformDeepseek), "deepseek-v4-pro")
	require.Contains(t, DefaultModelIDsForCNPlatform(PlatformMiniMax), "MiniMax-M2.5")
	require.Nil(t, DefaultModelIDsForCNPlatform(PlatformOpenAI))
}

func TestModelPricingListSeedIDs(t *testing.T) {
	ids := ModelPricingListSeedIDs()
	require.Contains(t, ids, "glm-5.3")
	require.Contains(t, ids, "kimi-k2.5")
	require.Contains(t, ids, "deepseek-v4-pro")
	require.Contains(t, ids, "MiniMax-M2.5")
	require.Contains(t, ids, "moonshot-v1-128k")

	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		key := strings.ToLower(id)
		_, dup := seen[key]
		require.False(t, dup, "duplicate seed id %s", id)
		seen[key] = struct{}{}
	}
}
