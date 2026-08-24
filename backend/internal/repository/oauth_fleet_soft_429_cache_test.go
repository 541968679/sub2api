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

func newOAuthFleetSoft429Cache(t *testing.T) (service.OAuthFleetSoft429Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewOAuthFleetSoft429Cache(rdb), mr
}

func TestOAuthFleetSoft429Cache_SetAndList(t *testing.T) {
	cache, mr := newOAuthFleetSoft429Cache(t)
	ctx := context.Background()

	require.NoError(t, cache.SetSoftExclude(ctx, 11, 20*time.Second))
	require.NoError(t, cache.SetSoftExclude(ctx, 22, 20*time.Second))

	ids, err := cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{11, 22}, ids)
	got, err := mr.Get(service.OAuthFleetSoft429RedisKey(11))
	require.NoError(t, err)
	require.Equal(t, "1", got)
}

func TestOAuthFleetSoft429Cache_TTLExpiry(t *testing.T) {
	cache, mr := newOAuthFleetSoft429Cache(t)
	ctx := context.Background()

	require.NoError(t, cache.SetSoftExclude(ctx, 7, 2*time.Second))
	ids, err := cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{7}, ids)

	mr.FastForward(3 * time.Second)
	ids, err = cache.ListSoftExcluded(ctx)
	require.NoError(t, err)
	require.Empty(t, ids)
}
