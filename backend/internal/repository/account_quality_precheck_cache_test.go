//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPrecheckSamples_ExcludeSelfAndTimeThenEval(t *testing.T) {
	cache, rdb := newPairQualityTestCache(t)
	ctx := context.Background()
	now := time.Now().UTC()
	p50 := 200
	rate := 0.9
	n := 3
	storeUserPolicy(t, cache, 16, "openai", &service.SmartSchedulePlatformPolicy{
		Enabled:                  true,
		ProbeLatencyV2:           true,
		CooldownMinutes:          5,
		QualityMaxP50TTFTMs:      &p50,
		QualityMinSuccessRate:    &rate,
		QualityMinTTFTSamples:    intPtrRepo(n),
		QualityMinSuccessSamples: intPtrRepo(n),
		QualityCondition:         strPtrRepo("or"),
	})

	ingestAccountQualityPrecheckSample(ctx, rdb, 7, 16, true, intPtrRepo(900), nil)
	ingestAccountQualityPrecheckSample(ctx, rdb, 7, 16, true, intPtrRepo(900), nil)
	outcome := cache.EnterProbe(ctx, 7, 16, "openai")
	require.Equal(t, service.ProbeAdmissionProbing, outcome, "self-only samples must not precheck-fail")
	require.True(t, cache.IsProbing(ctx, 7, 16, "openai"))
	require.False(t, cache.IsCooldownHard(ctx, 7, 16, "openai"))

	for i := 0; i < n; i++ {
		ingestAccountQualityPrecheckSample(ctx, rdb, 7, 17, true, intPtrRepo(40), nil)
	}
	require.Equal(t, service.ProbeAdmissionSelectable, cache.EnterProbe(ctx, 7, 16, "openai"))
	require.False(t, cache.IsProbing(ctx, 7, 16, "openai"))
	require.False(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(time.Second)))

	ingestAccountQualityPrecheckSample(ctx, rdb, 7, 17, true, intPtrRepo(900), nil)
	ingestAccountQualityPrecheckSample(ctx, rdb, 7, 17, true, intPtrRepo(900), nil)
	require.Equal(t, service.ProbeAdmissionCooling, cache.EnterProbe(ctx, 7, 16, "openai"))
	require.False(t, cache.IsProbing(ctx, 7, 16, "openai"))
	require.True(t, cache.IsCooldownHard(ctx, 7, 16, "openai"))
	require.Contains(t, cache.GetCooldownReason(ctx, 7, 16, "openai"), service.CooldownPhasePrecheck)

	cache.SoftEndCooldown(ctx, 7, 16, "openai", "should-not-end")
	require.True(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(time.Second)), "hard cooldown must ignore soft end")
}

func TestPrecheckHardWait_TTLDoesNotShortenSibling(t *testing.T) {
	cache, rdb := newPairQualityTestCache(t)
	ctx := context.Background()
	cache.markCooldownHard(ctx, 7, 16, "openai", 60)
	cache.markCooldownHard(ctx, 7, 17, "openai", 15)
	require.True(t, cache.IsCooldownHard(ctx, 7, 16, "openai"))
	require.True(t, cache.IsCooldownHard(ctx, 7, 17, "openai"))
	ttl, err := rdb.TTL(ctx, smartScheduleCooldownHardKey("openai", 7)).Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, ttl, 60*time.Minute+smartScheduleCooldownTTLBuffer-2*time.Second,
		"shorter sibling hard-wait must not Expire-down the shared HASH")
}

func storeUserPolicy(t *testing.T, cache *userSmartScheduleCache, userID int64, platform string, policy *service.SmartSchedulePlatformPolicy) {
	t.Helper()
	cache.storeUserBundle(context.Background(), userID, &service.UserSmartScheduleBundle{
		Policies: map[string]*service.SmartSchedulePlatformPolicy{platform: policy},
	})
}

func strPtrRepo(v string) *string { return &v }
