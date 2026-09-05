//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAICatalogSeeds(t *testing.T) {
	display := OpenAIDisplaySeed()
	require.Contains(t, display, OpenAIModelGPT6Astra)
	require.Contains(t, display, "grok-4.5")
	require.Contains(t, display, "gpt-5.6-sol")

	whitelist := OpenAIWhitelistSeed()
	require.Contains(t, whitelist, OpenAIModelGPT6Astra)
	require.Contains(t, whitelist, "gpt-5.6-luna")
	require.Contains(t, whitelist, "gpt-image-2")
	require.NotContains(t, whitelist, "grok-4.5")
}

func TestNormalizeOpenAIModelCatalog_AutoAddsNonGrokDisplayToWhitelist(t *testing.T) {
	cat := NormalizeOpenAIModelCatalog(
		[]string{"gpt-6-astra", "grok-4.5", "gpt-5.6-sol"},
		[]string{"gpt-5.6-sol"},
	)
	require.Equal(t, []string{"gpt-6-astra", "grok-4.5", "gpt-5.6-sol"}, cat.DisplayModels)
	require.Equal(t, []string{"gpt-5.6-sol", "gpt-6-astra"}, cat.WhitelistModels)
}

func TestNormalizeOpenAIModelCatalog_DropsNonGrokDisplayMissingFromWhitelistAfterRemoval(t *testing.T) {
	// Frontend should remove display when whitelist is cleared; backend also
	// auto-adds missing non-Grok display IDs, so dropping from whitelist only
	// works if display is also dropped. Simulate that pair.
	cat := NormalizeOpenAIModelCatalog([]string{"grok-4.5"}, []string{"gpt-5.6-sol"})
	require.Equal(t, []string{"grok-4.5"}, cat.DisplayModels)
	require.Equal(t, []string{"gpt-5.6-sol"}, cat.WhitelistModels)
}

func TestDiffNewWhitelistKeys_FirstSaveOnlyAddsGPT6(t *testing.T) {
	added := DiffNewWhitelistKeys(OpenAILegacyWhitelistBaseline(), OpenAIWhitelistSeed())
	require.Equal(t, []string{OpenAIModelGPT6Astra}, added)
}

func TestGatewayModelDiscoveryIDsForPlatform_IncludesGPT6Astra(t *testing.T) {
	openAI, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, OpenAIDisplaySeed(), openAI)
}

func TestGatewayModelDiscoveryIDsForPlatform_RespectsCatalogWithoutGrok(t *testing.T) {
	prev := openAIModelCatalogResolver
	t.Cleanup(func() { openAIModelCatalogResolver = prev })
	SetOpenAIModelCatalogResolver(func() *OpenAIModelCatalog {
		cat := NormalizeOpenAIModelCatalog([]string{"gpt-6-astra", "gpt-5.6-sol"}, []string{"gpt-6-astra", "gpt-5.6-sol"})
		return &cat
	})
	openAI, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, []string{"gpt-6-astra", "gpt-5.6-sol"}, openAI)
	require.NotContains(t, openAI, "grok-4.5")
}

func TestAccountIsModelSupported_OpenAIWhitelistSeedIncludesGPT6NotGrok(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-5.5": "gpt-5.5",
			},
		},
	}
	require.True(t, account.IsModelSupported(OpenAIModelGPT6Astra))
	require.False(t, account.IsModelSupported("grok-4.5"))

	account.Extra = map[string]any{AccountExtraModelMappingStrictScheduling: true}
	require.False(t, account.IsModelSupported(OpenAIModelGPT6Astra))
}

type catalogMergerStub struct {
	mergedKeys []string
	count      int64
	mergeN     int64
}

func (s *catalogMergerStub) MergeIdentityModelMappings(_ context.Context, _ string, keys []string) (int64, error) {
	s.mergedKeys = append([]string(nil), keys...)
	return s.mergeN, nil
}

func (s *catalogMergerStub) CountIdentityModelMappingTargets(_ context.Context, _ string, keys []string) (int64, error) {
	s.mergedKeys = append([]string(nil), keys...)
	return s.count, nil
}

func TestSettingServiceOpenAIModelCatalogFirstSaveMergesGPT6Only(t *testing.T) {
	repo := &modelMappingSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	merger := &catalogMergerStub{mergeN: 7, count: 7}
	svc.SetIdentityMappingMerger(merger)

	preview, keys, count, err := svc.PreviewOpenAIModelCatalogMerge(context.Background(), OpenAIDisplaySeed(), OpenAIWhitelistSeed())
	require.NoError(t, err)
	require.Equal(t, []string{OpenAIModelGPT6Astra}, keys)
	require.Equal(t, int64(7), count)
	require.Contains(t, preview.DisplayModels, OpenAIModelGPT6Astra)

	saved, merged, err := svc.SaveOpenAIModelCatalog(context.Background(), OpenAIDisplaySeed(), OpenAIWhitelistSeed())
	require.NoError(t, err)
	require.Equal(t, int64(7), merged)
	require.Equal(t, []string{OpenAIModelGPT6Astra}, merger.mergedKeys)
	require.Equal(t, int64(7), saved.MergedAccountCount)

	merger.mergedKeys = nil
	merger.mergeN = 99
	_, merged, err = svc.SaveOpenAIModelCatalog(context.Background(), OpenAIDisplaySeed(), OpenAIWhitelistSeed())
	require.NoError(t, err)
	require.Equal(t, int64(0), merged)
	require.Nil(t, merger.mergedKeys)
}

func TestSettingServiceOpenAIModelCatalogManualGrokWhitelistMerges(t *testing.T) {
	repo := &modelMappingSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	merger := &catalogMergerStub{mergeN: 2}
	svc.SetIdentityMappingMerger(merger)

	_, _, err := svc.SaveOpenAIModelCatalog(context.Background(), OpenAIDisplaySeed(), OpenAIWhitelistSeed())
	require.NoError(t, err)

	whitelist := append(append([]string{}, OpenAIWhitelistSeed()...), "grok-4.5")
	_, merged, err := svc.SaveOpenAIModelCatalog(context.Background(), OpenAIDisplaySeed(), whitelist)
	require.NoError(t, err)
	require.Equal(t, []string{"grok-4.5"}, merger.mergedKeys)
	require.Equal(t, int64(2), merged)
}
