//go:build unit

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newPairLiveCache(t *testing.T) (*miniredis.Miniredis, *redis.Client, *concurrencyCache) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache, ok := NewConcurrencyCache(rdb, 15, 15*60).(*concurrencyCache)
	require.True(t, ok)
	return mr, rdb, cache
}

func pairOwnerCtx(prefix string) context.Context {
	return context.WithValue(context.Background(), ctxkey.SlotOwnerPrefix, prefix)
}

func TestAccountUserSlot_StaleScoreInsideAccountTTLIsNotLive(t *testing.T) {
	_, rdb, cache := newPairLiveCache(t)
	ctx := pairOwnerCtx("liveproc")
	accountID, userID := int64(1718), int64(16)
	key := accountUserSlotKey(ctx, accountID, userID)
	staleScore := time.Now().Unix() - 120

	require.NoError(t, rdb.ZAdd(ctx, key, redis.Z{Score: float64(staleScore), Member: "liveproc-stale"}).Err())

	counts, err := cache.GetAccountUserConcurrencyBatch(ctx, []int64{accountID}, userID)
	require.NoError(t, err)
	require.Equal(t, 0, counts[accountID], "unreleased pair members older than the pair live window must not count")

	ok, err := cache.AcquireAccountUserSlot(ctx, accountID, userID, 1, "liveproc-new")
	require.NoError(t, err)
	require.True(t, ok, "stale unreleased members must not consume a capped pair slot")
}

func TestAccountUserSlot_ForeignProcessPrefixIsNotLive(t *testing.T) {
	_, rdb, cache := newPairLiveCache(t)
	ctx := pairOwnerCtx("curproc")
	accountID, userID := int64(1730), int64(16)
	key := accountUserSlotKey(ctx, accountID, userID)
	now := time.Now().Unix()

	require.NoError(t, rdb.ZAdd(ctx, key,
		redis.Z{Score: float64(now), Member: "oldproc-1"},
		redis.Z{Score: float64(now), Member: "oldproc-2"},
	).Err())

	counts, err := cache.GetAccountUserConcurrencyBatch(ctx, []int64{accountID}, userID)
	require.NoError(t, err)
	require.Equal(t, 0, counts[accountID], "other-process pair members must not show as live occupancy")

	ok, err := cache.AcquireAccountUserSlot(ctx, accountID, userID, 1, "curproc-1")
	require.NoError(t, err)
	require.True(t, ok, "other-process leftovers must not block a capped pair acquire")

	counts, err = cache.GetAccountUserConcurrencyBatch(ctx, []int64{accountID}, userID)
	require.NoError(t, err)
	require.Equal(t, 1, counts[accountID])
}

func TestClearAccountSlots_RemovesPairKeys(t *testing.T) {
	_, rdb, cache := newPairLiveCache(t)
	ctx := pairOwnerCtx("liveproc")
	accountID, userID := int64(1718), int64(16)

	ok, err := cache.AcquireAccountUserSlot(ctx, accountID, userID, 0, "liveproc-1")
	require.NoError(t, err)
	require.True(t, ok)

	counts, err := cache.GetAccountUserConcurrencyBatch(ctx, []int64{accountID}, userID)
	require.NoError(t, err)
	require.Equal(t, 1, counts[accountID])

	require.NoError(t, cache.ClearAccountSlots(ctx, accountID))

	key := accountUserSlotKey(ctx, accountID, userID)
	n, err := rdb.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), n, "ClearAccountSlots must delete concurrency:account_user:{accountID}:*")

	counts, err = cache.GetAccountUserConcurrencyBatch(ctx, []int64{accountID}, userID)
	require.NoError(t, err)
	require.Equal(t, 0, counts[accountID])
}

func TestCleanupStaleProcessSlots_RemovesPairKeys(t *testing.T) {
	_, rdb, cache := newPairLiveCache(t)
	ctx := context.Background()
	accountID, userID := int64(1718), int64(16)
	key := accountUserSlotKey(ctx, accountID, userID)
	now := time.Now().Unix()

	require.NoError(t, rdb.ZAdd(ctx, key,
		redis.Z{Score: float64(now), Member: "oldproc-677"},
		redis.Z{Score: float64(now), Member: "keep-1"},
	).Err())

	require.NoError(t, cache.CleanupStaleProcessSlots(ctx, "keep-"))

	members, err := rdb.ZRange(ctx, key, 0, -1).Result()
	require.NoError(t, err)
	require.Equal(t, []string{"keep-1"}, members)
}

func TestAccountUserSlot_LiveMemberStillCounts(t *testing.T) {
	_, _, cache := newPairLiveCache(t)
	ctx := pairOwnerCtx("liveproc")
	accountID, userID := int64(22), int64(16)

	ok, err := cache.AcquireAccountUserSlot(ctx, accountID, userID, 0, "liveproc-a")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = cache.AcquireAccountUserSlot(ctx, accountID, userID, 0, "liveproc-b")
	require.NoError(t, err)
	require.True(t, ok)

	counts, err := cache.GetAccountUserConcurrencyBatch(ctx, []int64{accountID}, userID)
	require.NoError(t, err)
	require.Equal(t, 2, counts[accountID])
	require.NotEqual(t, 999, counts[accountID])
}

func TestAccountUserSlot_ClearDoesNotDeleteSiblingAccountPairKey(t *testing.T) {
	_, rdb, cache := newPairLiveCache(t)
	ctx := pairOwnerCtx("liveproc")

	ok, err := cache.AcquireAccountUserSlot(ctx, 17, 16, 0, "liveproc-17")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = cache.AcquireAccountUserSlot(ctx, 171, 16, 0, "liveproc-171")
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, cache.ClearAccountSlots(ctx, 17))

	require.Equal(t, int64(0), rdb.Exists(ctx, accountUserSlotKey(ctx, 17, 16)).Val())
	require.Equal(t, int64(1), rdb.Exists(ctx, accountUserSlotKey(ctx, 171, 16)).Val(), "pattern must not swallow account 171 when clearing 17")
}

func TestAccountUserSlot_OpenAIPlatformKeyShape(t *testing.T) {
	_, rdb, cache := newPairLiveCache(t)
	ctx := context.WithValue(pairOwnerCtx("liveproc"), ctxkey.ScheduleLookupPlatform, "openai")
	accountID, userID := int64(1730), int64(16)

	ok, err := cache.AcquireAccountUserSlot(ctx, accountID, userID, 0, "liveproc-1")
	require.NoError(t, err)
	require.True(t, ok)

	key := fmt.Sprintf("%s%d:%d:%s", accountUserSlotKeyPrefix, accountID, userID, "openai")
	n, err := rdb.Exists(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}
