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
	cache.IngestPairQuality(ctx, 7, 16, service.PlatformOpenAI, 3, 3, true, &ttft)
	require.Equal(t, 1, cache.GetPairQuality(ctx, 7, 16, service.PlatformOpenAI).OKCount)
	require.Nil(t, cache.GetPairQuality(ctx, 7, 16, service.PlatformAntigravity))

	require.NoError(t, cache.MarkPairResume(ctx, 7, 16, service.PlatformAntigravity))
	require.True(t, cache.PairResumeActive(ctx, 7, 16, service.PlatformAntigravity, now))
	require.False(t, cache.PairResumeActive(ctx, 7, 16, service.PlatformOpenAI, now))
	require.Empty(t, cache.GetPairResumeUntilBatch(ctx, []int64{7}, 16, service.PlatformOpenAI, now))
	require.True(t, cache.GetPairResumeUntilBatch(ctx, []int64{7}, 16, service.PlatformAntigravity, now)[7].Active(now))
}

func TestAccountUserSlotKey_IncludesPlatform(t *testing.T) {
	ctxOAI := context.WithValue(context.Background(), ctxkey.ScheduleLookupPlatform, service.PlatformOpenAI)
	ctxAG := context.WithValue(context.Background(), ctxkey.ScheduleLookupPlatform, service.PlatformAntigravity)
	require.Equal(t, "concurrency:account_user:7:16:openai", accountUserSlotKey(ctxOAI, 7, 16))
	require.Equal(t, "concurrency:account_user:7:16:antigravity", accountUserSlotKey(ctxAG, 7, 16))
	require.Equal(t, "concurrency:account_user:7:16:_", accountUserSlotKey(context.Background(), 7, 16))
	require.NotEqual(t, accountUserSlotKey(ctxOAI, 7, 16), accountUserSlotKey(ctxAG, 7, 16))
}
