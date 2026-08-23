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

type cachedOpenAILongContextBilling struct {
	enabled   bool
	expiresAt int64
}

var openAILongContextBillingCache atomic.Value // *cachedOpenAILongContextBilling
var openAILongContextBillingSF singleflight.Group

const openAILongContextBillingCacheTTL = 60 * time.Second
const openAILongContextBillingErrorTTL = 5 * time.Second
const openAILongContextBillingDBTimeout = 5 * time.Second

func parseOpenAILongContextBillingEnabled(raw string) bool {
	return strings.TrimSpace(raw) != "false"
}

func storeOpenAILongContextBillingCache(enabled bool, ttl time.Duration) {
	openAILongContextBillingCache.Store(&cachedOpenAILongContextBilling{
		enabled:   enabled,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

func refreshOpenAILongContextBillingCache(enabled bool) {
	openAILongContextBillingSF.Forget("openai_long_context_billing")
	storeOpenAILongContextBillingCache(enabled, openAILongContextBillingCacheTTL)
}

// IsOpenAILongContextBillingEnabled reports whether GPT-5.4/5.5/5.6 session-level
// long-context multipliers are on. Missing, invalid, or unread values default to true.
func (s *SettingService) IsOpenAILongContextBillingEnabled(ctx context.Context) bool {
	if s == nil {
		return true
	}
	if cached, ok := openAILongContextBillingCache.Load().(*cachedOpenAILongContextBilling); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.enabled
		}
	}
	result, _, _ := openAILongContextBillingSF.Do("openai_long_context_billing", func() (any, error) {
		if cached, ok := openAILongContextBillingCache.Load().(*cachedOpenAILongContextBilling); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.enabled, nil
			}
		}
		if s.settingRepo == nil {
			storeOpenAILongContextBillingCache(true, openAILongContextBillingCacheTTL)
			return true, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAILongContextBillingDBTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyOpenAILongContextBillingEnabled)
		if err != nil {
			if errors.Is(err, ErrSettingNotFound) {
				storeOpenAILongContextBillingCache(true, openAILongContextBillingCacheTTL)
				return true, nil
			}
			slog.Warn("failed to get openai_long_context_billing_enabled setting", "error", err)
			storeOpenAILongContextBillingCache(true, openAILongContextBillingErrorTTL)
			return true, nil
		}
		enabled := parseOpenAILongContextBillingEnabled(value)
		storeOpenAILongContextBillingCache(enabled, openAILongContextBillingCacheTTL)
		return enabled, nil
	})
	if val, ok := result.(bool); ok {
		return val
	}
	return true
}
