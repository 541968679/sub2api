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

func TestPlatformCatalogSeeds_AnthropicGeminiAntigravity(t *testing.T) {
	anthropic := PlatformDisplaySeed(PlatformAnthropic)
	require.Contains(t, anthropic, "claude-opus-5")
	require.Equal(t, PlatformDisplaySeed(PlatformAnthropic), PlatformWhitelistSeed(PlatformAnthropic))
	require.Empty(t, DiffNewWhitelistKeys(PlatformLegacyWhitelistBaseline(PlatformAnthropic), PlatformWhitelistSeed(PlatformAnthropic)))

	gemini := PlatformDisplaySeed(PlatformGemini)
	require.Contains(t, gemini, "gemini-2.5-pro")
	require.Empty(t, DiffNewWhitelistKeys(PlatformLegacyWhitelistBaseline(PlatformGemini), PlatformWhitelistSeed(PlatformGemini)))

	agDisplay := PlatformDisplaySeed(PlatformAntigravity)
	require.Equal(t, AntigravityDisplaySeed(), agDisplay)
	agWhitelist := PlatformWhitelistSeed(PlatformAntigravity)
	require.Contains(t, agWhitelist, "claude-opus-5")
	require.Contains(t, agWhitelist, "gemini-2.5-flash")
	require.Empty(t, DiffNewWhitelistKeys(PlatformLegacyWhitelistBaseline(PlatformAntigravity), agWhitelist))
}

func TestNormalizePlatformModelCatalog_AnthropicAutoAddsDisplay(t *testing.T) {
	cat := NormalizePlatformModelCatalog(PlatformAnthropic, []string{"claude-opus-5", "claude-new", "grok-4.5"}, []string{"claude-opus-5"})
	require.Equal(t, []string{"claude-opus-5", "claude-new", "grok-4.5"}, cat.DisplayModels)
	require.Equal(t, []string{"claude-opus-5", "claude-new", "grok-4.5"}, cat.WhitelistModels)
}

func TestAccountIsModelSupported_CatalogWhitelistSeed(t *testing.T) {
	cases := []struct {
		platform string
		mapping  string
		allowed  string
	}{
		{PlatformAnthropic, "claude-sonnet-4-6", "claude-opus-5"},
		{PlatformGemini, "gemini-2.0-flash", "gemini-2.5-pro"},
		{PlatformAntigravity, "claude-sonnet-4-6", "claude-opus-5"},
	}
	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			account := &Account{
				Platform: tc.platform,
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						tc.mapping: tc.mapping,
					},
				},
			}
			require.True(t, account.IsModelSupported(tc.allowed))
			account.Extra = map[string]any{AccountExtraModelMappingStrictScheduling: true}
			require.False(t, account.IsModelSupported(tc.allowed))
		})
	}
}

func TestGatewayModelDiscoveryIDsForPlatform_IncludesGPT6Astra(t *testing.T) {
	openAI, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, OpenAIDisplaySeed(), openAI)
}

func TestGatewayModelDiscoveryIDsForPlatform_RespectsCatalogWithoutGrok(t *testing.T) {
	prev := platformModelCatalogResolver
	t.Cleanup(func() { platformModelCatalogResolver = prev })
	SetOpenAIModelCatalogResolver(func() *OpenAIModelCatalog {
		cat := NormalizeOpenAIModelCatalog([]string{"gpt-6-astra", "gpt-5.6-sol"}, []string{"gpt-6-astra", "gpt-5.6-sol"})
		return &cat
	})
	openAI, ok := GatewayModelDiscoveryIDsForPlatform(PlatformOpenAI)
	require.True(t, ok)
	require.Equal(t, []string{"gpt-6-astra", "gpt-5.6-sol"}, openAI)
	require.NotContains(t, openAI, "grok-4.5")
}

func TestGatewayModelDiscoveryIDsForPlatform_AnthropicCatalogOverride(t *testing.T) {
	prev := platformModelCatalogResolver
	t.Cleanup(func() { platformModelCatalogResolver = prev })
	SetPlatformModelCatalogResolver(func(platform string) *OpenAIModelCatalog {
		if platform != PlatformAnthropic {
			return nil
		}
		cat := NormalizePlatformModelCatalog(PlatformAnthropic, []string{"claude-opus-5"}, []string{"claude-opus-5"})
		return &cat
	})
	ids, ok := GatewayModelDiscoveryIDsForPlatform(PlatformAnthropic)
	require.True(t, ok)
	require.Equal(t, []string{"claude-opus-5"}, ids)
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

func TestSettingServiceCatalogFirstSaveMergesOnlyNewKeys(t *testing.T) {
	platforms := []string{PlatformAnthropic, PlatformGemini, PlatformAntigravity}
	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			repo := &modelMappingSettingRepoStub{values: map[string]string{}}
			svc := NewSettingService(repo, &config.Config{})
			merger := &catalogMergerStub{mergeN: 9}
			svc.SetIdentityMappingMerger(merger)

			_, keys, count, err := svc.PreviewPlatformModelCatalogMerge(
				context.Background(),
				platform,
				PlatformDisplaySeed(platform),
				PlatformWhitelistSeed(platform),
			)
			require.NoError(t, err)
			require.Empty(t, keys)
			require.Equal(t, int64(0), count)

			_, merged, err := svc.SavePlatformModelCatalog(
				context.Background(),
				platform,
				PlatformDisplaySeed(platform),
				append(append([]string{}, PlatformWhitelistSeed(platform)...), "new-model"),
			)
			require.NoError(t, err)
			require.Equal(t, []string{"new-model"}, merger.mergedKeys)
			require.Equal(t, int64(9), merged)
		})
	}
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
