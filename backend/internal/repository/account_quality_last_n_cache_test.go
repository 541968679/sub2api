//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newLastNCache(t *testing.T) *accountQualityLiveCache {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &accountQualityLiveCache{rdb: rdb}
}

func TestAccountQualityLastN_IngestRulesAndGetPrefersLastN(t *testing.T) {
	cache := newLastNCache(t)
	ctx := context.Background()
	ttft := 40

	fail := cache.IngestLastN(ctx, 7, 3, false, &ttft, nil, false)
	require.Equal(t, 0, fail.TTFTCount)
	require.Equal(t, 1, fail.OKCount)

	syncOK := cache.IngestLastN(ctx, 7, 3, true, nil, nil, false)
	require.Equal(t, 0, syncOK.TTFTCount)
	require.Equal(t, 2, syncOK.OKCount)

	streamOK := cache.IngestLastN(ctx, 7, 3, true, &ttft, nil, false)
	require.Equal(t, 1, streamOK.TTFTCount)
	require.Equal(t, 3, streamOK.OKCount)

	got, err := cache.Get(ctx, 7)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, int64(2), got.SuccessCount)
	require.Equal(t, int64(1), got.ErrorCount)
	require.Equal(t, int64(1), got.TTFTSamples)
	require.Equal(t, 3, got.AccountQualityWindowN)
	require.Equal(t, []int64{7}, cache.ListLastNAccountIDs(ctx))
}

func TestAccountQualityLastN_BatchAndAllUsersShareWindow(t *testing.T) {
	cache := newLastNCache(t)
	ctx := context.Background()
	cache.IngestLastN(ctx, 7, 4, true, intPtrRepo(40), nil, false)
	cache.IngestLastN(ctx, 7, 4, false, nil, nil, false)
	cache.IngestLastN(ctx, 8, 4, true, intPtrRepo(80), nil, false)

	batch := cache.GetLastNBatch(ctx, []int64{7, 8, 9})
	require.Equal(t, 2, batch[7].OKCount)
	require.Equal(t, 1, batch[8].OKCount)
	require.Nil(t, batch[9])
}

func TestUserQualityLastN_IngestIsolatesUsersAndSharesAccounts(t *testing.T) {
	cache := newLastNCache(t)
	ctx := context.Background()
	ttft := 40

	cache.IngestUserLastN(ctx, 16, 4, true, &ttft, nil, true, nil)
	cache.IngestUserLastN(ctx, 16, 4, false, nil, nil, true, nil)
	cache.IngestUserLastN(ctx, 17, 4, true, &ttft, nil, true, nil)

	batch := cache.GetUserLastNBatch(ctx, []int64{16, 17, 18})
	require.Equal(t, 2, batch[16].OKCount)
	require.Equal(t, 1, batch[16].TTFTCount)
	require.True(t, batch[16].UseFailover)
	require.Equal(t, 1, batch[17].OKCount)
	require.Nil(t, batch[18])
	require.Nil(t, cache.GetLastN(ctx, 16))
	ids := cache.ListUserLastNIDs(ctx)
	require.ElementsMatch(t, []int64{16, 17}, ids)
}
