//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

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

	cache.StartCooldown(ctx, 7, 16, 15, now)
	require.True(t, cache.CooldownActive(ctx, 7, 16, now.Add(time.Minute)))

	first, err := rdb.HGet(ctx, smartScheduleCooldownKey(7), smartScheduleCooldownField(16)).Result()
	require.NoError(t, err)
	require.Equal(t, now.Add(15*time.Minute).Unix(), mustParseUnix(t, first))

	cache.StartCooldown(ctx, 7, 16, 60, now.Add(2*time.Minute))
	second, err := rdb.HGet(ctx, smartScheduleCooldownKey(7), smartScheduleCooldownField(16)).Result()
	require.NoError(t, err)
	require.Equal(t, first, second)

	require.False(t, cache.CooldownActive(ctx, 7, 16, now.Add(16*time.Minute)))
	require.NoError(t, cache.ClearCooldown(ctx, 7, 16))
	require.False(t, cache.CooldownActive(ctx, 7, 16, now.Add(time.Minute)))
}

func TestUserSmartScheduleCache_CooldownTTLDoesNotShortenOtherPair(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewUserSmartScheduleCache(rdb, nil)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	cache.StartCooldown(ctx, 7, 16, 1440, now)
	longTTL, err := rdb.TTL(ctx, smartScheduleCooldownKey(7)).Result()
	require.NoError(t, err)
	require.Greater(t, longTTL, 20*time.Hour)

	cache.StartCooldown(ctx, 7, 17, 15, now)
	afterShort, err := rdb.TTL(ctx, smartScheduleCooldownKey(7)).Result()
	require.NoError(t, err)
	require.Greater(t, afterShort, 20*time.Hour)
	require.True(t, cache.CooldownActive(ctx, 7, 16, now.Add(time.Hour)))
	require.True(t, cache.CooldownActive(ctx, 7, 17, now.Add(time.Minute)))
}

func TestUserSmartScheduleCache_LookupFailOpenWithoutRepo(t *testing.T) {
	cache := NewUserSmartScheduleCache(nil, nil)
	require.Nil(t, cache.Lookup(context.Background(), 16))
	require.False(t, cache.CooldownActive(context.Background(), 7, 16, time.Now()))
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
