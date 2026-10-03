package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

type cachedKimiK3AdaptiveValidation struct {
	enabled   bool
	expiresAt int64
}

var kimiK3AdaptiveValidationCache atomic.Value // *cachedKimiK3AdaptiveValidation
var kimiK3AdaptiveValidationSF singleflight.Group

const kimiK3AdaptiveValidationCacheTTL = 60 * time.Second
const kimiK3AdaptiveValidationErrorTTL = 5 * time.Second
const kimiK3AdaptiveValidationDBTimeout = 5 * time.Second

func parseKimiK3AdaptiveValidationEnabled(raw string) bool {
	return strings.TrimSpace(raw) == "true"
}

func storeKimiK3AdaptiveValidationCache(enabled bool, ttl time.Duration) {
	kimiK3AdaptiveValidationCache.Store(&cachedKimiK3AdaptiveValidation{
		enabled:   enabled,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

func refreshKimiK3AdaptiveValidationCache(enabled bool) {
	kimiK3AdaptiveValidationSF.Forget("kimi_k3_adaptive_validation")
	storeKimiK3AdaptiveValidationCache(enabled, kimiK3AdaptiveValidationCacheTTL)
}

// IsKimiK3AdaptiveValidationEnabled reports whether kimi-k3 Chat Completions
// adaptive validation is on. Missing, invalid, unread, and nil service values
// are off. Only the stored string "true" enables it.
func (s *SettingService) IsKimiK3AdaptiveValidationEnabled(ctx context.Context) bool {
	if s == nil {
		return false
	}
	if cached, ok := kimiK3AdaptiveValidationCache.Load().(*cachedKimiK3AdaptiveValidation); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.enabled
		}
	}
	result, _, _ := kimiK3AdaptiveValidationSF.Do("kimi_k3_adaptive_validation", func() (any, error) {
		if cached, ok := kimiK3AdaptiveValidationCache.Load().(*cachedKimiK3AdaptiveValidation); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.enabled, nil
			}
		}
		if s.settingRepo == nil {
			storeKimiK3AdaptiveValidationCache(false, kimiK3AdaptiveValidationCacheTTL)
			return false, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), kimiK3AdaptiveValidationDBTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyKimiK3AdaptiveValidationEnabled)
		if err != nil {
			if !errors.Is(err, ErrSettingNotFound) {
				slog.Warn("failed to get kimi_k3_adaptive_validation_enabled setting", "error", err)
				storeKimiK3AdaptiveValidationCache(false, kimiK3AdaptiveValidationErrorTTL)
				return false, nil
			}
			storeKimiK3AdaptiveValidationCache(false, kimiK3AdaptiveValidationCacheTTL)
			return false, nil
		}
		enabled := parseKimiK3AdaptiveValidationEnabled(value)
		storeKimiK3AdaptiveValidationCache(enabled, kimiK3AdaptiveValidationCacheTTL)
		return enabled, nil
	})
	if val, ok := result.(bool); ok {
		return val
	}
	return false
}
