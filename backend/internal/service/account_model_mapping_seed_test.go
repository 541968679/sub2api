//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestBuildDefaultAccountModelMapping_OpenAIIdentitiesAndRewrites(t *testing.T) {
	previous := domain.GetPlatformDefaultMappingOverride
	t.Cleanup(func() { domain.GetPlatformDefaultMappingOverride = previous })
	domain.GetPlatformDefaultMappingOverride = func(platform string) map[string]string {
		if platform != PlatformOpenAI {
			return nil
		}
		return map[string]string{"gpt-5": "gpt-5.5"}
	}

	got := BuildDefaultAccountModelMapping(PlatformOpenAI)
	require.Equal(t, "gpt-5.5", got["gpt-5.5"])
	require.Equal(t, "gpt-5.5", got["gpt-5"])
	require.Equal(t, "gpt-6-astra", got[OpenAIModelGPT6Astra])
}

func TestBuildDefaultAccountModelMapping_AntigravityUsesPlatformDefault(t *testing.T) {
	got := BuildDefaultAccountModelMapping(PlatformAntigravity)
	require.NotEmpty(t, got)
	require.Equal(t, domain.DefaultAntigravityModelMapping["claude-opus-4-6"], got["claude-opus-4-6"])
}

func TestBuildDefaultAccountModelMapping_GrokUsesXAIDefaults(t *testing.T) {
	got := BuildDefaultAccountModelMapping(PlatformGrok)
	want := xai.DefaultModelMapping()
	require.Equal(t, want["grok-4.5"], got["grok-4.5"])
	require.Equal(t, want["grok"], got["grok"])
}

func TestSeedDefaultAccountModelMapping_SkipsExistingAndPassthrough(t *testing.T) {
	existing := map[string]any{"model_mapping": map[string]any{"from": "to"}}
	got := SeedDefaultAccountModelMapping(PlatformOpenAI, existing, nil)
	require.Equal(t, "to", got["model_mapping"].(map[string]any)["from"])

	passthrough := SeedDefaultAccountModelMapping(PlatformOpenAI, map[string]any{"token": "x"}, map[string]any{
		"openai_passthrough": true,
	})
	_, hasMapping := passthrough["model_mapping"]
	require.False(t, hasMapping)

	seeded := SeedDefaultAccountModelMapping(PlatformOpenAI, map[string]any{"token": "x"}, nil)
	mapping, ok := seeded["model_mapping"].(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, mapping)
	require.Equal(t, "gpt-5.5", mapping["gpt-5.5"])
}

func TestSeedDefaultAccountModelMapping_NilCredentials(t *testing.T) {
	got := SeedDefaultAccountModelMapping(PlatformAnthropic, nil, nil)
	require.NotNil(t, got)
	mapping, ok := got["model_mapping"].(map[string]any)
	require.True(t, ok)
	require.NotEmpty(t, mapping)
}
