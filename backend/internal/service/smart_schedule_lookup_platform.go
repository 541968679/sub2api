package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// SmartScheduleLookupHint is the request-side input for the smart-schedule
// policy key. It is not the scheduler eligibility platform.
type SmartScheduleLookupHint struct {
	GroupPlatform          string
	RequireClaudeGPTBridge bool
}

// SmartScheduleLookupPlatform returns the closed-pool policy key.
//
// Native OpenAI groups always stay account.Platform=openai.
// OpenAI + (Claude-GPT bridge or AG-group) uses the antigravity closed pool
// only while that pool is active (EnabledPolicy(antigravity) != nil).
// AG nil / disabled / empty keeps today's openai lookup. Never fail-open to
// account-side just because AG is off, and never fall back to openai once AG is on.
func SmartScheduleLookupPlatform(account *Account, hint SmartScheduleLookupHint, bundle *UserSmartScheduleBundle) string {
	if account == nil {
		return ""
	}
	native := normalizeSmartSchedulePlatform(account.Platform)
	if !account.IsOpenAI() {
		return native
	}
	wantsAG := hint.RequireClaudeGPTBridge || normalizeSmartSchedulePlatform(hint.GroupPlatform) == PlatformAntigravity
	if !wantsAG {
		return native
	}
	if bundle != nil && bundle.EnabledPolicy(PlatformAntigravity) != nil {
		return PlatformAntigravity
	}
	return PlatformOpenAI
}

func smartScheduleLookupHintFromContext(ctx context.Context) SmartScheduleLookupHint {
	hint := SmartScheduleLookupHint{}
	if ctx == nil {
		return hint
	}
	if v, ok := ctx.Value(ctxkey.RequireClaudeGPTBridge).(bool); ok {
		hint.RequireClaudeGPTBridge = v
	}
	if group, ok := ctx.Value(ctxkey.Group).(*Group); ok && group != nil && IsGroupContextValid(group) {
		hint.GroupPlatform = group.Platform
	}
	if hint.GroupPlatform == "" {
		if platform, ok := ctx.Value(ctxkey.ForcePlatform).(string); ok {
			hint.GroupPlatform = platform
		}
	}
	return hint
}

type smartScheduleBundleSource interface {
	Lookup(ctx context.Context, userID int64) *UserSmartScheduleBundle
}

func smartScheduleBundleFromSource(ctx context.Context, src smartScheduleBundleSource, userID int64) *UserSmartScheduleBundle {
	if src == nil || userID <= 0 {
		return nil
	}
	return src.Lookup(ctx, userID)
}

func smartScheduleLookupPlatformForUser(ctx context.Context, account *Account, src smartScheduleBundleSource, userID int64) string {
	return SmartScheduleLookupPlatform(account, smartScheduleLookupHintFromContext(ctx), smartScheduleBundleFromSource(ctx, src, userID))
}

func smartScheduleLookupPlatformFromCtx(ctx context.Context, account *Account, lookup SmartScheduleLookup) string {
	return smartScheduleLookupPlatformForUser(ctx, account, lookup, scheduleUserIDFromContext(ctx, 0))
}

func withRequireClaudeGPTBridge(ctx context.Context, enabled bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if !enabled {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.RequireClaudeGPTBridge, true)
}

// WithScheduleLookupPlatform stamps the Redis pair-slot / hydrate platform on ctx.
func WithScheduleLookupPlatform(ctx context.Context, platform string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	platform = normalizeSmartSchedulePlatform(platform)
	if platform == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.ScheduleLookupPlatform, platform)
}

// ScheduleLookupPlatformFromContext returns the stamped pair-slot platform.
func ScheduleLookupPlatformFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if platform, ok := ctx.Value(ctxkey.ScheduleLookupPlatform).(string); ok {
		return normalizeSmartSchedulePlatform(platform)
	}
	return ""
}

func stampSmartScheduleLookupPlatform(ctx context.Context, account *Account, lookup SmartScheduleLookup) context.Context {
	return WithScheduleLookupPlatform(ctx, smartScheduleLookupPlatformFromCtx(ctx, account, lookup))
}

func uniqueSmartScheduleMembershipPlatform(bundle *UserSmartScheduleBundle, accountID int64) string {
	if bundle == nil || accountID <= 0 {
		return ""
	}
	found := ""
	for platform, policy := range bundle.Policies {
		if policy == nil || !policy.HasAccount(accountID) {
			continue
		}
		if found != "" && found != normalizeSmartSchedulePlatform(platform) {
			return ""
		}
		found = normalizeSmartSchedulePlatform(platform)
	}
	return found
}

// SmartScheduleRedisPlatform is the Redis key segment for a pool platform.
// Empty becomes "_" so an unset caller cannot collide with a real platform.
func SmartScheduleRedisPlatform(platform string) string {
	platform = normalizeSmartSchedulePlatform(platform)
	if platform == "" {
		return "_"
	}
	return platform
}

func smartScheduleAccountMatchesTab(acc *Account, tab string) bool {
	if acc == nil {
		return false
	}
	tab = normalizeSmartSchedulePlatform(tab)
	if normalizeSmartSchedulePlatform(acc.Platform) == tab {
		return true
	}
	return tab == PlatformAntigravity && acc.IsOpenAI()
}
