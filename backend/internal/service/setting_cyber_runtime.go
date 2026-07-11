package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type cachedCyberSessionBlockRuntime struct {
	enabled   bool
	ttl       time.Duration
	expiresAt int64
}

const (
	cyberSessionBlockRuntimeCacheTTL  = 60 * time.Second
	cyberSessionBlockRuntimeErrorTTL  = 5 * time.Second
	cyberSessionBlockRuntimeDBTimeout = 5 * time.Second
)

func (s *SettingService) GetCyberSessionBlockRuntime(ctx context.Context) (bool, time.Duration) {
	if cached, ok := s.cyberSessionBlockRuntimeCache.Load().(*cachedCyberSessionBlockRuntime); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
		return cached.enabled, cached.ttl
	}
	result, _, _ := s.cyberSessionBlockRuntimeSF.Do("cyber_session_block_runtime", func() (any, error) {
		if cached, ok := s.cyberSessionBlockRuntimeCache.Load().(*cachedCyberSessionBlockRuntime); ok && cached != nil && time.Now().UnixNano() < cached.expiresAt {
			return cached, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cyberSessionBlockRuntimeDBTimeout)
		defer cancel()
		enabledVal, enabledErr := s.settingRepo.GetValue(dbCtx, SettingKeyCyberSessionBlockEnabled)
		ttlVal, ttlErr := s.settingRepo.GetValue(dbCtx, SettingKeyCyberSessionBlockTTLSeconds)
		if enabledErr != nil && !errors.Is(enabledErr, ErrSettingNotFound) {
			slog.Warn("failed to get cyber session block setting", "error", enabledErr)
			entry := &cachedCyberSessionBlockRuntime{ttl: time.Hour, expiresAt: time.Now().Add(cyberSessionBlockRuntimeErrorTTL).UnixNano()}
			s.cyberSessionBlockRuntimeCache.Store(entry)
			return entry, nil
		}
		ttl := time.Hour
		if ttlErr == nil {
			if seconds, err := strconv.Atoi(strings.TrimSpace(ttlVal)); err == nil && seconds > 0 {
				ttl = time.Duration(seconds) * time.Second
			}
		}
		entry := &cachedCyberSessionBlockRuntime{enabled: enabledErr == nil && strings.TrimSpace(enabledVal) == "true", ttl: ttl, expiresAt: time.Now().Add(cyberSessionBlockRuntimeCacheTTL).UnixNano()}
		s.cyberSessionBlockRuntimeCache.Store(entry)
		return entry, nil
	})
	entry, _ := result.(*cachedCyberSessionBlockRuntime)
	if entry == nil {
		return false, time.Hour
	}
	return entry.enabled, entry.ttl
}
