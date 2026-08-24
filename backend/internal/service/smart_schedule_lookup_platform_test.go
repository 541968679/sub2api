//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func agGroupScheduleCtx(userID int64) context.Context {
	ctx := context.WithValue(context.Background(), ctxkey.UserID, userID)
	return context.WithValue(ctx, ctxkey.Group, &Group{
		ID:       15,
		Platform: PlatformAntigravity,
		Status:   StatusActive,
		Hydrated: true,
	})
}

func nativeOpenAIGroupScheduleCtx(userID int64) context.Context {
	ctx := context.WithValue(context.Background(), ctxkey.UserID, userID)
	return context.WithValue(ctx, ctxkey.Group, &Group{
		ID:       19,
		Platform: PlatformOpenAI,
		Status:   StatusActive,
		Hydrated: true,
	})
}

func TestSmartScheduleLookupPlatform(t *testing.T) {
	t.Parallel()
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	ag := &Account{ID: 8, Platform: PlatformAntigravity}
	agHint := SmartScheduleLookupHint{GroupPlatform: PlatformAntigravity}
	bridgeHint := SmartScheduleLookupHint{RequireClaudeGPTBridge: true}
	nativeHint := SmartScheduleLookupHint{GroupPlatform: PlatformOpenAI}
	agOn := &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 3, nil),
		PlatformAntigravity: enabledSmartPolicy(7, 9, nil),
	}}
	agOff := &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 3, nil),
		PlatformAntigravity: {Enabled: false, AccountIDs: map[int64]struct{}{7: {}}},
	}}
	agEmpty := &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 3, nil),
		PlatformAntigravity: {Enabled: true, AccountIDs: map[int64]struct{}{}},
	}}
	agMissing := smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 3, nil))

	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, SmartScheduleLookupHint{}, nil))
	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, agHint, nil), "AG missing bundle keeps openai")
	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, bridgeHint, nil))
	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, agHint, agOff))
	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, agHint, agEmpty))
	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, agHint, agMissing))
	require.Equal(t, PlatformAntigravity, SmartScheduleLookupPlatform(oai, agHint, agOn))
	require.Equal(t, PlatformAntigravity, SmartScheduleLookupPlatform(oai, bridgeHint, agOn))
	require.Equal(t, PlatformOpenAI, SmartScheduleLookupPlatform(oai, nativeHint, agOn), "native GPT stays openai")
	require.Equal(t, PlatformAntigravity, SmartScheduleLookupPlatform(ag, agHint, nil))
	require.Equal(t, PlatformAntigravity, SmartScheduleLookupPlatform(ag, agHint, agOn))
	require.Equal(t, "", SmartScheduleLookupPlatform(nil, SmartScheduleLookupHint{}, agOn))
}

func TestSmartScheduleLookupPlatformFromContext(t *testing.T) {
	t.Parallel()
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	agOff := &memorySmartLookup{bundle: smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 0, nil))}
	agOn := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 0, nil),
		PlatformAntigravity: enabledSmartPolicy(7, 0, nil),
	}}}

	agCtx := agGroupScheduleCtx(16)
	require.Equal(t, PlatformOpenAI, smartScheduleLookupPlatformFromCtx(agCtx, oai, nil))
	require.Equal(t, PlatformOpenAI, smartScheduleLookupPlatformFromCtx(agCtx, oai, agOff))
	require.Equal(t, PlatformAntigravity, smartScheduleLookupPlatformFromCtx(agCtx, oai, agOn))

	bridgeCtx := withRequireClaudeGPTBridge(context.WithValue(context.Background(), ctxkey.UserID, int64(16)), true)
	require.Equal(t, PlatformOpenAI, smartScheduleLookupPlatformFromCtx(bridgeCtx, oai, agOff))
	require.Equal(t, PlatformAntigravity, smartScheduleLookupPlatformFromCtx(bridgeCtx, oai, agOn))
	require.Equal(t, PlatformOpenAI, smartScheduleLookupPlatformFromCtx(nativeOpenAIGroupScheduleCtx(16), oai, agOn))
}

func TestStampSmartScheduleLookupPlatformFollowsAdmission(t *testing.T) {
	t.Parallel()
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	agOff := &memorySmartLookup{bundle: smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 3, nil))}
	agOn := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI:      enabledSmartPolicy(7, 3, nil),
		PlatformAntigravity: enabledSmartPolicy(7, 9, nil),
	}}}

	offCtx := stampSmartScheduleLookupPlatform(agGroupScheduleCtx(16), oai, agOff)
	require.Equal(t, PlatformOpenAI, ScheduleLookupPlatformFromContext(offCtx))
	onCtx := stampSmartScheduleLookupPlatform(agGroupScheduleCtx(16), oai, agOn)
	require.Equal(t, PlatformAntigravity, ScheduleLookupPlatformFromContext(onCtx))
	nativeCtx := stampSmartScheduleLookupPlatform(nativeOpenAIGroupScheduleCtx(16), oai, agOn)
	require.Equal(t, PlatformOpenAI, ScheduleLookupPlatformFromContext(nativeCtx))
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

type observeLookupStub struct {
	memorySmartLookup
	got []PairQualityObservation
}

func (s *observeLookupStub) ObservePairCompletion(_ context.Context, obs PairQualityObservation) {
	s.got = append(s.got, obs)
}

func TestObservePairQualitySuccess_FollowsLookupPlatform(t *testing.T) {
	t.Parallel()
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	agOff := &observeLookupStub{memorySmartLookup: memorySmartLookup{
		bundle: smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 0, nil)),
	}}
	agOn := &observeLookupStub{memorySmartLookup: memorySmartLookup{
		bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(7, 0, nil),
			PlatformAntigravity: enabledSmartPolicy(7, 0, nil),
		}},
	}}
	observePairQualitySuccess(agOff, agGroupScheduleCtx(16), oai, 16, nil, nil, nil)
	require.Equal(t, []string{PlatformOpenAI}, observePlatforms(agOff.got))
	observePairQualitySuccess(agOn, agGroupScheduleCtx(16), oai, 16, nil, nil, nil)
	require.Equal(t, []string{PlatformAntigravity}, observePlatforms(agOn.got))
	observePairQualitySuccess(agOn, nativeOpenAIGroupScheduleCtx(16), oai, 16, nil, nil, nil)
	require.Equal(t, []string{PlatformAntigravity, PlatformOpenAI}, observePlatforms(agOn.got))
}

func observePlatforms(obs []PairQualityObservation) []string {
	out := make([]string, 0, len(obs))
	for _, item := range obs {
		out = append(out, item.Platform)
	}
	return out
}
