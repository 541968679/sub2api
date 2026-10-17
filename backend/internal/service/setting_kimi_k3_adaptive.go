package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

type cachedKimiK3AdaptiveValidation struct {
	checks    KimiK3AdaptiveChecks
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

func storeKimiK3AdaptiveValidationCache(checks KimiK3AdaptiveChecks, ttl time.Duration) {
	kimiK3AdaptiveValidationCache.Store(&cachedKimiK3AdaptiveValidation{
		checks:    checks,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

func refreshKimiK3AdaptiveValidationCache(checks KimiK3AdaptiveChecks) {
	kimiK3AdaptiveValidationSF.Forget("kimi_k3_adaptive_validation")
	storeKimiK3AdaptiveValidationCache(checks, kimiK3AdaptiveValidationCacheTTL)
}

// resolveKimiK3AdaptiveChecks turns the stored master switch and the per-check
// JSON into the policy that runs. A present checks JSON wins, including {}.
// Invalid JSON enables nothing. A missing JSON with master string "true" keeps
// the older checks on and leaves sampling off.
func resolveKimiK3AdaptiveChecks(masterRaw, checksRaw string) KimiK3AdaptiveChecks {
	if strings.TrimSpace(checksRaw) != "" {
		parsed, ok := parseKimiK3AdaptiveChecks(checksRaw)
		if !ok {
			return KimiK3AdaptiveChecks{}
		}
		return parsed
	}
	if parseKimiK3AdaptiveValidationEnabled(masterRaw) {
		return kimiK3AdaptiveChecksLegacy()
	}
	return KimiK3AdaptiveChecks{}
}

func parseKimiK3AdaptiveChecks(raw string) (KimiK3AdaptiveChecks, bool) {
	var checks KimiK3AdaptiveChecks
	if err := json.Unmarshal([]byte(raw), &checks); err != nil {
		return KimiK3AdaptiveChecks{}, false
	}
	return checks, true
}

func (s *SettingService) readKimiK3AdaptiveSetting(ctx context.Context, key string) (string, bool) {
	value, err := s.settingRepo.GetValue(ctx, key)
	if err == nil {
		return value, true
	}
	if errors.Is(err, ErrSettingNotFound) {
		return "", true
	}
	slog.Warn("failed to get kimi k3 adaptive validation setting", "key", key, "error", err)
	return "", false
}

// IsKimiK3AdaptiveValidationEnabled reports whether any kimi-k3 adaptive check
// is on. Missing, invalid, unread, and nil service values are off.
func (s *SettingService) IsKimiK3AdaptiveValidationEnabled(ctx context.Context) bool {
	return s.KimiK3AdaptiveChecks(ctx).Any()
}

// KimiK3AdaptiveChecks returns the cached per-check policy.
func (s *SettingService) KimiK3AdaptiveChecks(ctx context.Context) KimiK3AdaptiveChecks {
	if s == nil {
		return KimiK3AdaptiveChecks{}
	}
	if cached, ok := kimiK3AdaptiveValidationCache.Load().(*cachedKimiK3AdaptiveValidation); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.checks
		}
	}
	result, _, _ := kimiK3AdaptiveValidationSF.Do("kimi_k3_adaptive_validation", func() (any, error) {
		if cached, ok := kimiK3AdaptiveValidationCache.Load().(*cachedKimiK3AdaptiveValidation); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.checks, nil
			}
		}
		if s.settingRepo == nil {
			storeKimiK3AdaptiveValidationCache(KimiK3AdaptiveChecks{}, kimiK3AdaptiveValidationCacheTTL)
			return KimiK3AdaptiveChecks{}, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), kimiK3AdaptiveValidationDBTimeout)
		defer cancel()
		master, ok := s.readKimiK3AdaptiveSetting(dbCtx, SettingKeyKimiK3AdaptiveValidationEnabled)
		if !ok {
			storeKimiK3AdaptiveValidationCache(KimiK3AdaptiveChecks{}, kimiK3AdaptiveValidationErrorTTL)
			return KimiK3AdaptiveChecks{}, nil
		}
		checksRaw, ok := s.readKimiK3AdaptiveSetting(dbCtx, SettingKeyKimiK3AdaptiveValidationChecks)
		if !ok {
			storeKimiK3AdaptiveValidationCache(KimiK3AdaptiveChecks{}, kimiK3AdaptiveValidationErrorTTL)
			return KimiK3AdaptiveChecks{}, nil
		}
		checks := resolveKimiK3AdaptiveChecks(master, checksRaw)
		storeKimiK3AdaptiveValidationCache(checks, kimiK3AdaptiveValidationCacheTTL)
		return checks, nil
	})
	if checks, ok := result.(KimiK3AdaptiveChecks); ok {
		return checks
	}
	return KimiK3AdaptiveChecks{}
}
