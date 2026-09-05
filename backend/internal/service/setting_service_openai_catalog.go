package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetOpenAIModelCatalog returns the effective OpenAI catalog (KV or seed).
func (s *SettingService) GetOpenAIModelCatalog(ctx context.Context) OpenAIModelCatalog {
	return s.GetPlatformModelCatalog(ctx, PlatformOpenAI)
}

// GetPlatformModelCatalog returns the effective catalog for a platform.
func (s *SettingService) GetPlatformModelCatalog(ctx context.Context, platform string) OpenAIModelCatalog {
	if stored, ok := s.loadStoredPlatformModelCatalog(ctx, platform); ok {
		return stored
	}
	return defaultPlatformModelCatalog(platform)
}

func (s *SettingService) loadStoredOpenAIModelCatalog(ctx context.Context) (OpenAIModelCatalog, bool) {
	return s.loadStoredPlatformModelCatalog(ctx, PlatformOpenAI)
}

func (s *SettingService) loadStoredPlatformModelCatalog(ctx context.Context, platform string) (OpenAIModelCatalog, bool) {
	if s == nil || s.settingRepo == nil {
		return OpenAIModelCatalog{}, false
	}
	key, ok := catalogSettingKey(platform)
	if !ok {
		return OpenAIModelCatalog{}, false
	}
	val, err := s.settingRepo.GetValue(ctx, key)
	if err != nil || strings.TrimSpace(val) == "" {
		return OpenAIModelCatalog{}, false
	}
	return parsePlatformModelCatalogJSON(platform, val)
}

func (s *SettingService) PreviewOpenAIModelCatalogMerge(ctx context.Context, display, whitelist []string) (OpenAIModelCatalog, []string, int64, error) {
	return s.PreviewPlatformModelCatalogMerge(ctx, PlatformOpenAI, display, whitelist)
}

func (s *SettingService) PreviewPlatformModelCatalogMerge(ctx context.Context, platform string, display, whitelist []string) (OpenAIModelCatalog, []string, int64, error) {
	if !isCatalogPlatform(platform) {
		return OpenAIModelCatalog{}, nil, 0, fmt.Errorf("unsupported catalog platform: %s", platform)
	}
	next := NormalizePlatformModelCatalog(platform, display, whitelist)
	keys := s.platformCatalogMergeKeys(ctx, platform, next)
	var count int64
	if len(keys) > 0 && s != nil && s.identityMappingMerger != nil {
		n, err := s.identityMappingMerger.CountIdentityModelMappingTargets(ctx, platform, keys)
		if err != nil {
			return next, keys, 0, err
		}
		count = n
	}
	return next, keys, count, nil
}

func (s *SettingService) SaveOpenAIModelCatalog(ctx context.Context, display, whitelist []string) (OpenAIModelCatalog, int64, error) {
	return s.SavePlatformModelCatalog(ctx, PlatformOpenAI, display, whitelist)
}

func (s *SettingService) SavePlatformModelCatalog(ctx context.Context, platform string, display, whitelist []string) (OpenAIModelCatalog, int64, error) {
	if s == nil || s.settingRepo == nil {
		return OpenAIModelCatalog{}, 0, fmt.Errorf("setting service is not configured")
	}
	key, ok := catalogSettingKey(platform)
	if !ok {
		return OpenAIModelCatalog{}, 0, fmt.Errorf("unsupported catalog platform: %s", platform)
	}
	next := NormalizePlatformModelCatalog(platform, display, whitelist)
	keys := s.platformCatalogMergeKeys(ctx, platform, next)

	data, err := json.Marshal(OpenAIModelCatalog{
		DisplayModels:   next.DisplayModels,
		WhitelistModels: next.WhitelistModels,
	})
	if err != nil {
		return OpenAIModelCatalog{}, 0, fmt.Errorf("marshal model catalog: %w", err)
	}
	if err := s.settingRepo.Set(ctx, key, string(data)); err != nil {
		return OpenAIModelCatalog{}, 0, err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}

	var merged int64
	if len(keys) > 0 && s.identityMappingMerger != nil {
		n, err := s.identityMappingMerger.MergeIdentityModelMappings(ctx, platform, keys)
		if err != nil {
			return next, 0, err
		}
		merged = n
	}
	next.MergedAccountCount = merged
	return next, merged, nil
}

func (s *SettingService) openAICatalogMergeKeys(ctx context.Context, next OpenAIModelCatalog) []string {
	return s.platformCatalogMergeKeys(ctx, PlatformOpenAI, next)
}

func (s *SettingService) platformCatalogMergeKeys(ctx context.Context, platform string, next OpenAIModelCatalog) []string {
	previous := PlatformLegacyWhitelistBaseline(platform)
	if stored, ok := s.loadStoredPlatformModelCatalog(ctx, platform); ok {
		previous = stored.WhitelistModels
	}
	return DiffNewWhitelistKeys(previous, next.WhitelistModels)
}
