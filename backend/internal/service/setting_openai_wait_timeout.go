package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

type cachedOpenAIWaitTimeoutSettings struct {
	settings  OpenAIWaitTimeoutSettings
	expiresAt int64
}

var openAIWaitTimeoutSettingsCache atomic.Value // *cachedOpenAIWaitTimeoutSettings
var openAIWaitTimeoutSettingsSF singleflight.Group

const (
	openAIWaitTimeoutSettingsCacheTTL  = 30 * time.Second
	openAIWaitTimeoutSettingsErrorTTL  = 5 * time.Second
	openAIWaitTimeoutSettingsDBTimeout = 5 * time.Second
	openAIWaitTimeoutSettingsCacheKey  = "openai_wait_timeout_settings"
)

func storeOpenAIWaitTimeoutSettingsCache(settings OpenAIWaitTimeoutSettings, ttl time.Duration) {
	openAIWaitTimeoutSettingsCache.Store(&cachedOpenAIWaitTimeoutSettings{
		settings:  settings,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

func invalidateOpenAIWaitTimeoutSettingsCache() {
	openAIWaitTimeoutSettingsSF.Forget(openAIWaitTimeoutSettingsCacheKey)
	storeOpenAIWaitTimeoutSettingsCache(OpenAIWaitTimeoutSettings{}, 0)
}

// InvalidateOpenAIWaitTimeoutSettingsCacheForTest clears the 30s hot-path cache.
func InvalidateOpenAIWaitTimeoutSettingsCacheForTest() {
	invalidateOpenAIWaitTimeoutSettingsCache()
}

func loadCachedOpenAIWaitTimeoutSettings() (OpenAIWaitTimeoutSettings, bool) {
	cached, ok := openAIWaitTimeoutSettingsCache.Load().(*cachedOpenAIWaitTimeoutSettings)
	if !ok || cached == nil {
		return OpenAIWaitTimeoutSettings{}, false
	}
	if time.Now().UnixNano() >= cached.expiresAt {
		return OpenAIWaitTimeoutSettings{}, false
	}
	return cached.settings, true
}

func parseOpenAIWaitTimeoutSettingsJSON(raw string) OpenAIWaitTimeoutSettings {
	if strings.TrimSpace(raw) == "" {
		return *DefaultOpenAIWaitTimeoutSettings()
	}
	var partial struct {
		HeaderWaitSeconds       *int `json:"header_wait_seconds"`
		FirstUsefulFrameSeconds *int `json:"first_useful_frame_seconds"`
	}
	if err := json.Unmarshal([]byte(raw), &partial); err != nil {
		return *DefaultOpenAIWaitTimeoutSettings()
	}
	out := *DefaultOpenAIWaitTimeoutSettings()
	if partial.HeaderWaitSeconds != nil {
		out.HeaderWaitSeconds = *partial.HeaderWaitSeconds
	}
	if partial.FirstUsefulFrameSeconds != nil {
		out.FirstUsefulFrameSeconds = *partial.FirstUsefulFrameSeconds
	}
	return NormalizeOpenAIWaitTimeoutSettings(out)
}

func (s *SettingService) loadOpenAIWaitTimeoutSettingsFromRepo(ctx context.Context) OpenAIWaitTimeoutSettings {
	defaults := *DefaultOpenAIWaitTimeoutSettings()
	if s == nil || s.settingRepo == nil {
		return defaults
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIWaitTimeoutSettingsDBTimeout)
	defer cancel()
	value, err := s.settingRepo.GetValue(dbCtx, SettingKeyOpenAIWaitTimeoutSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			storeOpenAIWaitTimeoutSettingsCache(defaults, openAIWaitTimeoutSettingsCacheTTL)
			return defaults
		}
		slog.Warn("failed to get openai_wait_timeout_settings", "error", err)
		storeOpenAIWaitTimeoutSettingsCache(defaults, openAIWaitTimeoutSettingsErrorTTL)
		return defaults
	}
	settings := parseOpenAIWaitTimeoutSettingsJSON(value)
	storeOpenAIWaitTimeoutSettingsCache(settings, openAIWaitTimeoutSettingsCacheTTL)
	return settings
}

// GetOpenAIWaitTimeoutSettings returns the OpenAI wait-timeout KV, with 30s
// hot-path cache. Missing key / empty / bad JSON → code defaults 90/30.
func (s *SettingService) GetOpenAIWaitTimeoutSettings(ctx context.Context) (*OpenAIWaitTimeoutSettings, error) {
	got := s.GetOpenAIWaitTimeoutSettingsCached(ctx)
	return &got, nil
}

// GetOpenAIWaitTimeoutSettingsCached is the zero-error hot path.
func (s *SettingService) GetOpenAIWaitTimeoutSettingsCached(ctx context.Context) OpenAIWaitTimeoutSettings {
	if cached, ok := loadCachedOpenAIWaitTimeoutSettings(); ok {
		return cached
	}
	if s == nil {
		return *DefaultOpenAIWaitTimeoutSettings()
	}
	result, _, _ := openAIWaitTimeoutSettingsSF.Do(openAIWaitTimeoutSettingsCacheKey, func() (any, error) {
		if cached, ok := loadCachedOpenAIWaitTimeoutSettings(); ok {
			return cached, nil
		}
		return s.loadOpenAIWaitTimeoutSettingsFromRepo(ctx), nil
	})
	if settings, ok := result.(OpenAIWaitTimeoutSettings); ok {
		return settings
	}
	return *DefaultOpenAIWaitTimeoutSettings()
}

// SetOpenAIWaitTimeoutSettings persists and refreshes the 30s cache.
func (s *SettingService) SetOpenAIWaitTimeoutSettings(ctx context.Context, settings *OpenAIWaitTimeoutSettings) error {
	if err := validateOpenAIWaitTimeoutSettings(settings); err != nil {
		return err
	}
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("setting repository is not configured")
	}
	normalized := NormalizeOpenAIWaitTimeoutSettings(*settings)
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal openai wait timeout settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOpenAIWaitTimeoutSettings, string(data)); err != nil {
		return fmt.Errorf("set openai wait timeout settings: %w", err)
	}
	invalidateOpenAIWaitTimeoutSettingsCache()
	storeOpenAIWaitTimeoutSettingsCache(normalized, openAIWaitTimeoutSettingsCacheTTL)
	return nil
}
