//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestReserveInflightBalance_SecondRequestRejectedWhenEstimateExceedsRemainder(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := &billingCache{rdb: rdb}
	ctx := context.Background()

	allowed, sum, err := cache.ReserveInflightBalance(ctx, 723, "req-1", 8, 10, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Zero(t, sum)

	allowed, sum, err = cache.ReserveInflightBalance(ctx, 723, "req-2", 8, 10, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)
	require.InDelta(t, 8, sum, 0.000001)

	require.NoError(t, cache.ReleaseInflightBalance(ctx, 723, "req-1"))
	allowed, _, err = cache.ReserveInflightBalance(ctx, 723, "req-3", 8, 10, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestReserveInflightBalance_FirstRequestAdmittedAboveBalance(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := &billingCache{rdb: rdb}

	allowed, _, err := cache.ReserveInflightBalance(context.Background(), 723, "req-1", 25, 1, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
}
