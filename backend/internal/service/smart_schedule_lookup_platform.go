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
// Bridge / Antigravity-group traffic on an OpenAI account looks up antigravity.
// Native OpenAI groups keep account.Platform=openai. Never fall back across pools.
func SmartScheduleLookupPlatform(account *Account, hint SmartScheduleLookupHint) string {
	if account == nil {
		return ""
	}
	if account.IsOpenAI() && (hint.RequireClaudeGPTBridge || normalizeSmartSchedulePlatform(hint.GroupPlatform) == PlatformAntigravity) {
		return PlatformAntigravity
	}
	return normalizeSmartSchedulePlatform(account.Platform)
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

func smartScheduleLookupPlatformFromCtx(ctx context.Context, account *Account) string {
	return SmartScheduleLookupPlatform(account, smartScheduleLookupHintFromContext(ctx))
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

func stampSmartScheduleLookupPlatform(ctx context.Context, account *Account) context.Context {
	return WithScheduleLookupPlatform(ctx, smartScheduleLookupPlatformFromCtx(ctx, account))
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
