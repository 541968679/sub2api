package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetOpenAIModelCatalog returns the effective OpenAI catalog (KV or seed).
func (s *SettingService) GetOpenAIModelCatalog(ctx context.Context) OpenAIModelCatalog {
	if stored, ok := s.loadStoredOpenAIModelCatalog(ctx); ok {
		return stored
	}
	return defaultOpenAIModelCatalog()
}

func (s *SettingService) loadStoredOpenAIModelCatalog(ctx context.Context) (OpenAIModelCatalog, bool) {
	if s == nil || s.settingRepo == nil {
		return OpenAIModelCatalog{}, false
	}
	val, err := s.settingRepo.GetValue(ctx, SettingKeyOpenAIModelCatalog)
	if err != nil || strings.TrimSpace(val) == "" {
		return OpenAIModelCatalog{}, false
	}
	return parseOpenAIModelCatalogJSON(val)
}

// PreviewOpenAIModelCatalogMerge reports which identity keys would be merged
// and how many OpenAI accounts would be updated. It does not write.
func (s *SettingService) PreviewOpenAIModelCatalogMerge(ctx context.Context, display, whitelist []string) (OpenAIModelCatalog, []string, int64, error) {
	next := NormalizeOpenAIModelCatalog(display, whitelist)
	keys := s.openAICatalogMergeKeys(ctx, next)
	var count int64
	if len(keys) > 0 && s != nil && s.identityMappingMerger != nil {
		n, err := s.identityMappingMerger.CountIdentityModelMappingTargets(ctx, PlatformOpenAI, keys)
		if err != nil {
			return next, keys, 0, err
		}
		count = n
	}
	return next, keys, count, nil
}

// SaveOpenAIModelCatalog persists the catalog, merges newly added whitelist
// identity keys onto OpenAI accounts with non-empty mappings, and returns the
// saved catalog plus merge count.
func (s *SettingService) SaveOpenAIModelCatalog(ctx context.Context, display, whitelist []string) (OpenAIModelCatalog, int64, error) {
	if s == nil || s.settingRepo == nil {
		return OpenAIModelCatalog{}, 0, fmt.Errorf("setting service is not configured")
	}
	next := NormalizeOpenAIModelCatalog(display, whitelist)
	keys := s.openAICatalogMergeKeys(ctx, next)

	data, err := json.Marshal(OpenAIModelCatalog{
		DisplayModels:   next.DisplayModels,
		WhitelistModels: next.WhitelistModels,
	})
	if err != nil {
		return OpenAIModelCatalog{}, 0, fmt.Errorf("marshal openai model catalog: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpenAIModelCatalog, string(data)); err != nil {
		return OpenAIModelCatalog{}, 0, err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}

	var merged int64
	if len(keys) > 0 && s.identityMappingMerger != nil {
		n, err := s.identityMappingMerger.MergeIdentityModelMappings(ctx, PlatformOpenAI, keys)
		if err != nil {
			return next, 0, err
		}
		merged = n
	}
	next.MergedAccountCount = merged
	return next, merged, nil
}

func (s *SettingService) openAICatalogMergeKeys(ctx context.Context, next OpenAIModelCatalog) []string {
	previous := OpenAILegacyWhitelistBaseline()
	if stored, ok := s.loadStoredOpenAIModelCatalog(ctx); ok {
		previous = stored.WhitelistModels
	}
	return DiffNewWhitelistKeys(previous, next.WhitelistModels)
}
