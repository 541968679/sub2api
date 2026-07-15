package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIDiscoveryIncludesCanonicalGrok45(t *testing.T) {
	ids, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Contains(t, ids, "grok-4.5")
	require.Contains(t, ids, "gpt-5.6-sol")
}

func TestEnsureOpenAICanonicalGrokModels_AppendsEvenOnCustomSubset(t *testing.T) {
	// Simulates a custom models list that only exposes a few GPT models.
	out := EnsureOpenAICanonicalGrokModels([]string{"gpt-5.6-sol", "gpt-5.5"})
	require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.5", "grok-4.5"}, out)

	// Idempotent.
	out2 := EnsureOpenAICanonicalGrokModels(out)
	require.Equal(t, out, out2)
}
