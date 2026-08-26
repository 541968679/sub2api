//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestUserSmartScheduleCache_CooldownSetNXDoesNotExtend(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	cache.StartCooldown(ctx, 7, 16, "openai", 15, now)
	require.True(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(time.Minute)))

	first, err := rdb.HGet(ctx, smartScheduleCooldownKey("openai", 7), smartScheduleCooldownField(16)).Result()
	require.NoError(t, err)
	require.Equal(t, now.Add(15*time.Minute).Unix(), mustParseUnix(t, first))

	cache.StartCooldown(ctx, 7, 16, "openai", 60, now.Add(2*time.Minute))
	second, err := rdb.HGet(ctx, smartScheduleCooldownKey("openai", 7), smartScheduleCooldownField(16)).Result()
	require.NoError(t, err)
	require.Equal(t, first, second)

	require.False(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(16*time.Minute)))
	require.NoError(t, cache.ClearCooldown(ctx, 7, 16, "openai"))
	require.False(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(time.Minute)))
}

func TestUserSmartScheduleCache_SetCooldownOverwrites(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	cache.StartCooldown(ctx, 7, 16, "openai", 15, now)
	until, err := cache.SetCooldown(ctx, 7, 16, "openai", 60, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, now.Add(62*time.Minute).Unix(), until.Unix())
	raw, err := rdb.HGet(ctx, smartScheduleCooldownKey("openai", 7), smartScheduleCooldownField(16)).Result()
	require.NoError(t, err)
	require.Equal(t, now.Add(62*time.Minute).Unix(), mustParseUnix(t, raw))
	require.True(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(20*time.Minute)))
}

func TestUserSmartScheduleCache_CooldownTTLDoesNotShortenOtherPair(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	cache.StartCooldown(ctx, 7, 16, "openai", 1440, now)
	longTTL, err := rdb.TTL(ctx, smartScheduleCooldownKey("openai", 7)).Result()
	require.NoError(t, err)
	require.Greater(t, longTTL, 20*time.Hour)

	cache.StartCooldown(ctx, 7, 17, "openai", 15, now)
	afterShort, err := rdb.TTL(ctx, smartScheduleCooldownKey("openai", 7)).Result()
	require.NoError(t, err)
	require.Greater(t, afterShort, 20*time.Hour)
	require.True(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(time.Hour)))
	require.True(t, cache.CooldownActive(ctx, 7, 17, "openai", now.Add(time.Minute)))
}

func TestUserSmartScheduleCache_LookupFailOpenWithoutRepo(t *testing.T) {
	cache := NewUserSmartScheduleCache(nil, nil)
	require.Nil(t, cache.Lookup(context.Background(), 16))
	require.False(t, cache.CooldownActive(context.Background(), 7, 16, "openai", time.Now()))
	require.Empty(t, cache.GetCooldownUntilBatch(context.Background(), []int64{7}, 16, "openai", time.Now()))
}

func TestUserSmartScheduleCache_GetCooldownUntilBatch(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	cache.StartCooldown(ctx, 7, 16, "openai", 15, now)
	cache.StartCooldown(ctx, 8, 16, "openai", 15, now.Add(-20*time.Minute))
	got := cache.GetCooldownUntilBatch(ctx, []int64{7, 8, 9}, 16, "openai", now)
	require.Len(t, got, 1)
	require.Equal(t, now.Add(15*time.Minute).Unix(), got[7].Unix())
	_, expired := got[8]
	require.False(t, expired)
}

func TestUserSmartScheduleCache_ApplyMemberPausedWriteThrough(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	bundle := &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		service.PlatformAnthropic: {
			Enabled:         true,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{7: {}},
		},
	}}
	payload, err := json.Marshal(cachedSmartScheduleBundleFrom(bundle))
	require.NoError(t, err)
	require.NoError(t, rdb.Set(ctx, smartScheduleUserKey(16), payload, 0).Err())
	require.False(t, cache.Lookup(ctx, 16).Policies[service.PlatformAnthropic].IsPaused(7))

	require.NoError(t, cache.ApplyMemberPaused(ctx, 16, 7, service.PlatformAnthropic, true))
	require.True(t, cache.Lookup(ctx, 16).Policies[service.PlatformAnthropic].IsPaused(7))
	require.NoError(t, cache.ApplyMemberPaused(ctx, 16, 7, service.PlatformAnthropic, false))
	require.False(t, cache.Lookup(ctx, 16).Policies[service.PlatformAnthropic].IsPaused(7))
}

func TestCachedSmartScheduleBundle_ProbeConcurrencyRoundTrip(t *testing.T) {
	stored := cachedSmartScheduleBundleFrom(&service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		service.PlatformAnthropic: {
			Enabled:              true,
			CooldownMinutes:      15,
			QualityWindowSamples: intPtrRepo(10),
			ProbeConcurrencyMode: service.ProbeConcurrencyModeCustom,
			ProbeConcurrency:     intPtrRepo(2),
			AccountIDs:           map[int64]struct{}{7: {}},
		},
	}})
	got := stored.toBundle()
	require.Equal(t, service.ProbeConcurrencyModeCustom, got.Policies[service.PlatformAnthropic].ProbeConcurrencyMode)
	require.Equal(t, 2, *got.Policies[service.PlatformAnthropic].ProbeConcurrency)
	require.Equal(t, 2, got.Policies[service.PlatformAnthropic].ProbeInFlightCap(5))
	require.False(t, got.Policies[service.PlatformAnthropic].ProbeLatencyV2, "v2 stays default false")

	v2Stored := cachedSmartScheduleBundleFrom(&service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		service.PlatformAnthropic: {Enabled: true, ProbeLatencyV2: true},
	}})
	require.True(t, v2Stored.toBundle().Policies[service.PlatformAnthropic].ProbeLatencyV2)
}

func TestCachedSmartScheduleBundle_PausedRoundTrip(t *testing.T) {
	stored := cachedSmartScheduleBundleFrom(&service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		service.PlatformAnthropic: {
			Enabled:         true,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{7: {}, 8: {}},
			Paused:          map[int64]struct{}{7: {}},
		},
	}})
	got := stored.toBundle()
	require.True(t, got.Policies[service.PlatformAnthropic].IsPaused(7))
	require.False(t, got.Policies[service.PlatformAnthropic].IsPaused(8))
	require.True(t, got.Policies[service.PlatformAnthropic].HasAccount(8))
}

func mustParseUnix(t *testing.T, raw string) int64 {
	t.Helper()
	var n int64
	for _, ch := range raw {
		require.GreaterOrEqual(t, ch, '0')
		require.LessOrEqual(t, ch, '9')
		n = n*10 + int64(ch-'0')
	}
	_ = service.DefaultSmartScheduleCooldownMinutes
	return n
}

func TestUserSmartScheduleCache_PlatformIsolation(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	cache.StartCooldown(ctx, 7, 16, service.PlatformAntigravity, 15, now)
	require.True(t, cache.CooldownActive(ctx, 7, 16, service.PlatformAntigravity, now.Add(time.Minute)))
	require.False(t, cache.CooldownActive(ctx, 7, 16, service.PlatformOpenAI, now.Add(time.Minute)))

	cache.MarkProbing(ctx, 7, 16, service.PlatformOpenAI)
	require.True(t, cache.IsProbing(ctx, 7, 16, service.PlatformOpenAI))
	require.False(t, cache.IsProbing(ctx, 7, 16, service.PlatformAntigravity))

	cache.MarkPinned(ctx, 7, 16, service.PlatformAntigravity)
	require.True(t, cache.IsPinned(ctx, 7, 16, service.PlatformAntigravity))
	require.False(t, cache.IsPinned(ctx, 7, 16, service.PlatformOpenAI))

	ttft := 40
	cache.IngestPairQuality(ctx, 7, 16, service.PlatformOpenAI, 3, 3, true, &ttft, nil)
	require.Equal(t, 1, cache.GetPairQuality(ctx, 7, 16, service.PlatformOpenAI).OKCount)
	require.Nil(t, cache.GetPairQuality(ctx, 7, 16, service.PlatformAntigravity))

	require.NoError(t, cache.MarkPairResume(ctx, 7, 16, service.PlatformAntigravity))
	require.True(t, cache.PairResumeActive(ctx, 7, 16, service.PlatformAntigravity, now))
	require.False(t, cache.PairResumeActive(ctx, 7, 16, service.PlatformOpenAI, now))
	require.Empty(t, cache.GetPairResumeUntilBatch(ctx, []int64{7}, 16, service.PlatformOpenAI, now))
	require.True(t, cache.GetPairResumeUntilBatch(ctx, []int64{7}, 16, service.PlatformAntigravity, now)[7].Active(now))
}

func TestUserSmartScheduleCache_SoftCooldownWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	ttft := 40

	liveA := cache.IngestSoftCooldown(ctx, 7, 16, "openai", 3, 3, true, &ttft, nil, 15)
	require.Equal(t, 1, liveA.OKCount)
	require.Nil(t, cache.GetSoftCooldown(ctx, 8, 16, "openai"), "pair isolation")
	require.Nil(t, cache.GetSoftCooldown(ctx, 7, 16, "anthropic"))

	liveB := cache.IngestSoftCooldown(ctx, 8, 16, "openai", 3, 3, true, intPtrRepo(80), nil, 15)
	require.Equal(t, 1, liveB.OKCount)
	require.Equal(t, 1, cache.GetSoftCooldown(ctx, 7, 16, "openai").OKCount)

	batch := cache.GetSoftCooldownBatch(ctx, []int64{7, 8, 9}, 16, "openai")
	require.Equal(t, 1, batch[7].OKCount)
	require.Equal(t, 1, batch[8].OKCount)
	require.Nil(t, batch[9])

	key := smartScheduleSoftCoolKey("openai", 7)
	firstTTL, err := rdb.TTL(ctx, key).Result()
	require.NoError(t, err)
	cache.IngestSoftCooldown(ctx, 7, 17, "openai", 3, 3, true, &ttft, nil, 5)
	shortTTL, err := rdb.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.GreaterOrEqual(t, shortTTL, firstTTL-time.Second, "TTL must not shorten")
	cache.IngestSoftCooldown(ctx, 7, 16, "openai", 3, 3, true, &ttft, nil, 60)
	longTTL, err := rdb.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.Greater(t, longTTL, shortTTL)

	cache.IngestSoftCooldown(ctx, 7, 16, "openai", 3, 3, true, &ttft, nil, 15)
	require.NotNil(t, cache.GetSoftCooldown(ctx, 7, 16, "openai"))
	_, err = cache.SetCooldown(ctx, 7, 16, "openai", 15, now)
	require.NoError(t, err)
	require.Nil(t, cache.GetSoftCooldown(ctx, 7, 16, "openai"), "re-select cooldown zeros window")

	require.NoError(t, cache.ClearCooldown(ctx, 7, 16, "openai"))
	cache.IngestSoftCooldown(ctx, 7, 16, "openai", 3, 3, true, &ttft, nil, 15)
	cache.StartCooldown(ctx, 7, 16, "openai", 15, now)
	require.Nil(t, cache.GetSoftCooldown(ctx, 7, 16, "openai"), "first StartCooldown zeros window")
	cache.IngestSoftCooldown(ctx, 7, 16, "openai", 3, 3, true, &ttft, nil, 15)
	cache.StartCooldown(ctx, 7, 16, "openai", 60, now.Add(time.Minute))
	require.NotNil(t, cache.GetSoftCooldown(ctx, 7, 16, "openai"), "HSETNX no-op must not wipe window")

	require.NoError(t, cache.ClearCooldown(ctx, 7, 16, "openai"))
	require.Nil(t, cache.GetSoftCooldown(ctx, 7, 16, "openai"), "leaving cooldown deletes window")
}

func TestCachedSmartScheduleBundle_LatencyGateRoundTrip(t *testing.T) {
	ttft := 10000
	dur := 80000
	src := &service.SmartSchedulePlatformPolicy{
		Enabled:                        true,
		CooldownMinutes:                3,
		QualityMaxP50TTFTMs:            &ttft,
		QualityMinTTFTSamples:          intPtrRepo(5),
		QualityMinSuccessSamples:       intPtrRepo(50),
		QualityMaxSlowInWindow:         intPtrRepo(2),
		QualityMaxConsecutiveSlow:      intPtrRepo(2),
		QualityMaxP50DurationMs:        &dur,
		QualitySchedWindowN:            intPtrRepo(10),
		QualitySchedMaxSlowInWindow:    intPtrRepo(3),
		QualitySchedMaxConsecutiveSlow: intPtrRepo(2),
		AccountIDs:                     map[int64]struct{}{7: {}},
	}
	payload, err := json.Marshal(cachedSmartScheduleBundleFrom(&service.UserSmartScheduleBundle{
		Policies: map[string]*service.SmartSchedulePlatformPolicy{service.PlatformOpenAI: src},
	}))
	require.NoError(t, err)
	for _, key := range []string{
		"quality_max_slow_in_window",
		"quality_max_consecutive_slow",
		"quality_max_p50_duration_ms",
		"quality_sched_window_n",
		"quality_sched_max_slow_in_window",
		"quality_sched_max_consecutive_slow",
	} {
		require.Contains(t, string(payload), key, "storeUserBundle must keep latency-gate column %s", key)
	}

	var stored cachedSmartScheduleBundle
	require.NoError(t, json.Unmarshal(payload, &stored))
	got := stored.toBundle().Policies[service.PlatformOpenAI]
	require.Equal(t, 2, *got.QualityMaxSlowInWindow)
	require.Equal(t, 2, *got.QualityMaxConsecutiveSlow)
	require.Equal(t, 80000, *got.QualityMaxP50DurationMs)
	require.Equal(t, 10, *got.QualitySchedWindowN)
	require.Equal(t, 3, *got.QualitySchedMaxSlowInWindow)
	require.Equal(t, 2, *got.QualitySchedMaxConsecutiveSlow)
	require.True(t, got.SchedCompositeEnabled(), "Lookup hit must keep selectable composite on")

	empty := cachedSmartScheduleBundleFrom(&service.UserSmartScheduleBundle{
		Policies: map[string]*service.SmartSchedulePlatformPolicy{
			service.PlatformOpenAI: {Enabled: true, QualityMaxP50TTFTMs: &ttft, QualityMinTTFTSamples: intPtrRepo(5)},
		},
	}).toBundle().Policies[service.PlatformOpenAI]
	require.False(t, empty.SchedCompositeEnabled(), "unconfigured sched columns stay legacy p50")
}

func TestUserSmartScheduleCache_LookupKeepsSchedLatencyGate(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	ttft := 10000
	dur := 80000
	bundle := &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		service.PlatformOpenAI: {
			Enabled:                        true,
			CooldownMinutes:                3,
			QualityMaxP50TTFTMs:            &ttft,
			QualityMinTTFTSamples:          intPtrRepo(5),
			QualityMaxP50DurationMs:        &dur,
			QualitySchedWindowN:            intPtrRepo(10),
			QualitySchedMaxSlowInWindow:    intPtrRepo(3),
			QualitySchedMaxConsecutiveSlow: intPtrRepo(2),
			AccountIDs:                     map[int64]struct{}{1724: {}},
		},
	}}
	payload, err := json.Marshal(cachedSmartScheduleBundleFrom(bundle))
	require.NoError(t, err)
	require.NoError(t, rdb.Set(ctx, smartScheduleUserKey(16), payload, 0).Err())
	got := cache.Lookup(ctx, 16)
	require.NotNil(t, got)
	policy := got.Policies[service.PlatformOpenAI]
	require.NotNil(t, policy)
	require.Equal(t, 10, *policy.QualitySchedWindowN)
	require.Equal(t, 3, *policy.QualitySchedMaxSlowInWindow)
	require.Equal(t, 2, *policy.QualitySchedMaxConsecutiveSlow)
	require.Equal(t, 80000, *policy.QualityMaxP50DurationMs)
	require.True(t, policy.SchedCompositeEnabled())

	stale := `{"policies":{"openai":{"enabled":true,"quality_max_p50_ttft_ms":10000,"quality_min_ttft_samples":5,"cooldown_minutes":3}}}`
	require.NoError(t, rdb.Set(ctx, smartScheduleUserKey(16), stale, 0).Err())
	staleGot := cache.Lookup(ctx, 16).Policies[service.PlatformOpenAI]
	require.False(t, staleGot.SchedCompositeEnabled(), "0.1.261 JSON without sched columns must stay legacy until invalidate")
}

func TestCachedSmartScheduleBundle_SoftCooldownRoundTrip(t *testing.T) {
	dur := 80000
	stored := cachedSmartScheduleBundleFrom(&service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		service.PlatformAnthropic: {
			Enabled:                 true,
			CooldownMinutes:         15,
			SoftCooldown:            true,
			QualityMaxP50DurationMs: &dur,
			QualitySchedWindowN:     intPtrRepo(10),
			AccountIDs:              map[int64]struct{}{7: {}},
		},
	}})
	got := stored.toBundle().Policies[service.PlatformAnthropic]
	require.True(t, got.SoftCooldown)
	require.Equal(t, 80000, *got.QualityMaxP50DurationMs)
	require.Equal(t, 10, *got.QualitySchedWindowN)
}

func TestAccountUserSlotKey_IncludesPlatform(t *testing.T) {
	ctxOAI := context.WithValue(context.Background(), ctxkey.ScheduleLookupPlatform, service.PlatformOpenAI)
	ctxAG := context.WithValue(context.Background(), ctxkey.ScheduleLookupPlatform, service.PlatformAntigravity)
	require.Equal(t, "concurrency:account_user:7:16:openai", accountUserSlotKey(ctxOAI, 7, 16))
	require.Equal(t, "concurrency:account_user:7:16:antigravity", accountUserSlotKey(ctxAG, 7, 16))
	require.Equal(t, "concurrency:account_user:7:16:_", accountUserSlotKey(context.Background(), 7, 16))
	require.NotEqual(t, accountUserSlotKey(ctxOAI, 7, 16), accountUserSlotKey(ctxAG, 7, 16))
}
