//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestSmartScheduleLookupPlatform(t *testing.T) {
	t.Parallel()
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	ag := &Account{ID: 8, Platform: PlatformAntigravity}

	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, SmartScheduleLookupHint{}))
	require.Equal(t, PlatformAntigravity, SmartScheduleLookupPlatform(oai, SmartScheduleLookupHint{GroupPlatform: PlatformAntigravity}))
	require.Equal(t, PlatformAntigravity, SmartScheduleLookupPlatform(oai, SmartScheduleLookupHint{RequireClaudeGPTBridge: true}))
	require.Equal(t, PlatformAntigravity, SmartScheduleLookupPlatform(ag, SmartScheduleLookupHint{GroupPlatform: PlatformAntigravity}))
	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, SmartScheduleLookupHint{GroupPlatform: PlatformOpenAI}))
	require.Equal(t, "", SmartScheduleLookupPlatform(nil, SmartScheduleLookupHint{}))
}

func TestSmartScheduleLookupPlatformFromContext(t *testing.T) {
	t.Parallel()
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
		ID:       15,
		Platform: PlatformAntigravity,
		Status:   StatusActive,
		Hydrated: true,
	})
	require.Equal(t, PlatformAntigravity, smartScheduleLookupPlatformFromCtx(ctx, oai))

	bridgeCtx := withRequireClaudeGPTBridge(context.Background(), true)
	require.Equal(t, PlatformAntigravity, smartScheduleLookupPlatformFromCtx(bridgeCtx, oai))
}

func TestSmartScheduleAccountMatchesTab(t *testing.T) {
	t.Parallel()
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	ag := &Account{ID: 8, Platform: PlatformAntigravity}
	require.True(t, smartScheduleAccountMatchesTab(oai, PlatformAntigravity))
	require.True(t, smartScheduleAccountMatchesTab(ag, PlatformAntigravity))
	require.True(t, smartScheduleAccountMatchesTab(oai, PlatformOpenAI))
	require.False(t, smartScheduleAccountMatchesTab(oai, PlatformAnthropic))
	require.False(t, smartScheduleAccountMatchesTab(oai, PlatformGemini))
	require.False(t, smartScheduleAccountMatchesTab(oai, PlatformGrok))
}

func TestUniqueSmartScheduleMembershipPlatform(t *testing.T) {
	t.Parallel()
	bundle := &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 0, nil),
		PlatformAntigravity: enabledSmartPolicy(7, 0, nil),
	}}
	require.Equal(t, "", uniqueSmartScheduleMembershipPlatform(bundle, 7))
	require.Equal(t, PlatformOpenAI, uniqueSmartScheduleMembershipPlatform(smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 0, nil)), 7))
}

func TestSmartScheduleRedisPlatform(t *testing.T) {
	t.Parallel()
	require.Equal(t, PlatformOpenAI, SmartScheduleRedisPlatform(PlatformOpenAI))
	require.Equal(t, "_", SmartScheduleRedisPlatform(""))
}
