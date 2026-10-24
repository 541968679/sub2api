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

func init() {
	startOAuthFleetSoft429Migration = false
}

func newOAuthFleetSoft429Cache(t *testing.T) (service.OAuthFleetSoft429Cache, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewOAuthFleetSoft429Cache(rdb), rdb, mr
}

func TestOAuthFleetSoft429Cache_SetAndList(t *testing.T) {
	cache, rdb, _ := newOAuthFleetSoft429Cache(t)
	ctx := context.Background()

	require.NoError(t, cache.SetSoftExclude(ctx, 11, 20*time.Second))
	require.NoError(t, cache.SetSoftExclude(ctx, 22, 20*time.Second))

	ids, err := cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{11, 22}, ids)

	score, err := rdb.ZScore(ctx, oauthFleetSoft429ExpKey, "11").Result()
	require.NoError(t, err)
	require.InDelta(t, float64(time.Now().Add(20*time.Second).UnixMilli()), score, 2000)
	require.Equal(t, int64(0), rdb.Exists(ctx, service.OAuthFleetSoft429RedisKey(11)).Val())
}

func TestOAuthFleetSoft429Cache_ListDoesNotScanLegacyKeys(t *testing.T) {
	cache, rdb, _ := newOAuthFleetSoft429Cache(t)
	ctx := context.Background()

	require.NoError(t, rdb.Set(ctx, service.OAuthFleetSoft429RedisKey(99), "1", time.Minute).Err())
	require.NoError(t, cache.SetSoftExclude(ctx, 11, 20*time.Second))

	ids, err := cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{11}, ids)
}

func TestOAuthFleetSoft429Cache_TTLExpiry(t *testing.T) {
	cache, rdb, _ := newOAuthFleetSoft429Cache(t)
	ctx := context.Background()

	require.NoError(t, cache.SetSoftExclude(ctx, 7, 2*time.Second))
	ids, err := cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{7}, ids)

	require.NoError(t, rdb.ZAdd(ctx, oauthFleetSoft429ExpKey, redis.Z{
		Score:  float64(time.Now().Add(-time.Second).UnixMilli()),
		Member: "7",
	}).Err())
	ids, err = cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.Empty(t, ids)
	require.Equal(t, int64(0), rdb.ZCard(ctx, oauthFleetSoft429ExpKey).Val())
}

func TestOAuthFleetSoft429Cache_MigrateLegacyKeyOnce(t *testing.T) {
	_, rdb, _ := newOAuthFleetSoft429Cache(t)
	ctx := context.Background()
	cache := &oauthFleetSoft429Cache{rdb: rdb}

	require.NoError(t, rdb.Set(ctx, service.OAuthFleetSoft429RedisKey(9), "1", 30*time.Second).Err())
	require.NoError(t, rdb.Set(ctx, "other-key", "keep", 0).Err())

	require.NoError(t, cache.migrateLegacyKeys(ctx))
	ids, err := cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{9}, ids)
	require.Equal(t, int64(0), rdb.Exists(ctx, service.OAuthFleetSoft429RedisKey(9)).Val())
	require.Equal(t, "keep", rdb.Get(ctx, "other-key").Val())

	require.NoError(t, rdb.Set(ctx, service.OAuthFleetSoft429RedisKey(8), "1", 30*time.Second).Err())
	require.NoError(t, cache.migrateLegacyKeys(ctx))
	ids, err = cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{9}, ids)
	require.Equal(t, "1", rdb.Get(ctx, service.OAuthFleetSoft429RedisKey(8)).Val())
}
