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

func newPairQualityTestCache(t *testing.T) (*userSmartScheduleCache, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewUserSmartScheduleCache(rdb, nil).(*userSmartScheduleCache), rdb
}

func TestPairQualityCache_FailureNoTTFTFailoverAndIsolation(t *testing.T) {
	cache, _ := newPairQualityTestCache(t)
	ctx := context.Background()
	ttft := 90

	fail := cache.IngestPairQuality(ctx, 7, 16, "openai", 3, 3, false, &ttft, nil)
	require.Equal(t, 1, fail.OKCount)
	require.Equal(t, 0, fail.TTFTCount)
	require.Equal(t, 0.0, *fail.SuccessRate)

	syncOK := cache.IngestPairQuality(ctx, 7, 16, "openai", 3, 3, true, nil, nil)
	require.Equal(t, 2, syncOK.OKCount)
	require.Equal(t, 0, syncOK.TTFTCount)

	streamOK := cache.IngestPairQuality(ctx, 7, 16, "openai", 3, 3, true, &ttft, nil)
	require.Equal(t, 3, streamOK.OKCount)
	require.Equal(t, 1, streamOK.TTFTCount)

	other := cache.IngestPairQuality(ctx, 7, 17, "openai", 3, 3, true, intPtrRepo(800), nil)
	require.Equal(t, 1, other.OKCount)
	require.Equal(t, 800, *other.P50TTFTMs)
	user16 := cache.GetPairQuality(ctx, 7, 16, "openai")
	require.Equal(t, 1, user16.TTFTCount)
	require.Equal(t, 90, *user16.P50TTFTMs)

	batch := cache.GetPairQualityBatch(ctx, []int64{7}, 16, "openai")
	require.Equal(t, 3, batch[7].OKCount)
	require.Nil(t, cache.GetPairQuality(ctx, 8, 16, "openai"))
}

func TestPairQualityCache_ExpiryKeepsWindowsAndEntersProbing(t *testing.T) {
	cache, _ := newPairQualityTestCache(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	cache.IngestPairQuality(ctx, 7, 16, "openai", 3, 3, true, intPtrRepo(40), nil)
	cache.StartCooldown(ctx, 7, 16, "openai", 15, now)
	require.NotZero(t, cache.GetPairQuality(ctx, 7, 16, "openai").OKCount)
	require.False(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(16*time.Minute)))
	live := cache.GetPairQuality(ctx, 7, 16, "openai")
	require.Equal(t, 1, live.OKCount, "expiry must not zero pair windows")
	require.Equal(t, 1, live.TTFTCount)
	for _, event := range cache.ListPairQualityEvents(ctx, 7, 16, "openai", 20) {
		require.NotEqual(t, service.PairQualityEventExpiryZero, event.Type)
	}
	require.True(t, cache.IsProbing(ctx, 7, 16, "openai"), "expiry must enter probing, not selectable")
	require.False(t, cache.IsPinned(ctx, 7, 16, "openai"), "expiry must never enter pinned")
	require.False(t, cache.IsProbing(ctx, 7, 17, "openai"), "other user is not backfilled")
	foundEnter := false
	for _, event := range cache.ListPairQualityEvents(ctx, 7, 16, "openai", 20) {
		if event.Type == service.PairQualityEventProbeEnter {
			foundEnter = true
		}
	}
	require.True(t, foundEnter)
}

func TestPairQualityCache_ExpiryClearsResumeGrace(t *testing.T) {
	cache, rdb := newPairQualityTestCache(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	require.NoError(t, cache.MarkPairResume(ctx, 7, 16, "openai"))
	require.NoError(t, rdb.HSet(ctx, accountQualityResumeKey(7),
		accountQualityResumeUserField(16), now.Add(15*time.Minute).Unix(),
		accountQualityResumeWatchingField(16), now.Add(30*time.Minute).Unix(),
	).Err())
	cache.StartCooldown(ctx, 7, 16, "openai", 15, now)
	require.False(t, cache.CooldownActive(ctx, 7, 16, "openai", now.Add(16*time.Minute)))
	require.True(t, cache.IsProbing(ctx, 7, 16, "openai"))
	require.False(t, cache.PairResumeActive(ctx, 7, 16, "openai", now.Add(time.Minute)), "expiry must clear this pool's pair resume")
	n, err := rdb.HExists(ctx, accountQualityResumeKey(7), accountQualityResumeUserField(16)).Result()
	require.NoError(t, err)
	require.True(t, n, "expiry must not clear Track A account-quality:resume")
	watching, err := rdb.HExists(ctx, accountQualityResumeKey(7), accountQualityResumeWatchingField(16)).Result()
	require.NoError(t, err)
	require.True(t, watching)
}

func TestPairQualityCache_NoBackfillWithoutMark(t *testing.T) {
	cache, _ := newPairQualityTestCache(t)
	ctx := context.Background()
	require.False(t, cache.IsProbing(ctx, 7, 16, "openai"), "Redis miss is not probing")
	require.Empty(t, cache.IsProbingBatch(ctx, []int64{7, 8}, 16, "openai"))
	cache.MarkProbing(ctx, 7, 16, "openai")
	require.True(t, cache.IsProbing(ctx, 7, 16, "openai"))
	require.True(t, cache.IsProbingBatch(ctx, []int64{7, 8}, 16, "openai")[7])
	require.False(t, cache.IsProbingBatch(ctx, []int64{7, 8}, 16, "openai")[8])
	cache.GraduateProbing(ctx, 7, 16, "openai")
	require.False(t, cache.IsProbing(ctx, 7, 16, "openai"))
	foundGrad := false
	for _, event := range cache.ListPairQualityEvents(ctx, 7, 16, "openai", 20) {
		if event.Type == service.PairQualityEventProbeGraduate {
			foundGrad = true
		}
	}
	require.True(t, foundGrad)
}

func TestPairQualityCache_ZeroClearsAndKeepsIsolation(t *testing.T) {
	cache, _ := newPairQualityTestCache(t)
	ctx := context.Background()
	cache.IngestPairQuality(ctx, 7, 16, "openai", 3, 3, true, intPtrRepo(40), nil)
	cache.IngestPairQuality(ctx, 7, 17, "openai", 3, 3, true, intPtrRepo(80), nil)
	cache.ZeroPairQuality(ctx, 7, 16, "openai", service.PairQualityEventSelectable)
	require.Equal(t, 0, cache.GetPairQuality(ctx, 7, 16, "openai").OKCount)
	require.Equal(t, 1, cache.GetPairQuality(ctx, 7, 17, "openai").OKCount)
}

func TestPairQualityCache_PinnedNoTTLNoBackfillAndBlocksCooldown(t *testing.T) {
	cache, rdb := newPairQualityTestCache(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 17, 0, 0, 0, time.UTC)

	require.False(t, cache.IsPinned(ctx, 7, 16, "openai"), "Redis miss is not pinned")
	require.Empty(t, cache.IsPinnedBatch(ctx, []int64{7, 8}, 16, "openai"))
	cache.MarkPinned(ctx, 7, 16, "openai")
	require.True(t, cache.IsPinned(ctx, 7, 16, "openai"))
	require.True(t, cache.IsPinnedBatch(ctx, []int64{7, 8}, 16, "openai")[7])
	require.False(t, cache.IsPinnedBatch(ctx, []int64{7, 8}, 16, "openai")[8])
	ttl, err := rdb.TTL(ctx, smartSchedulePinnedKey("openai", 7)).Result()
	require.NoError(t, err)
	require.Less(t, ttl, time.Duration(0), "pin HASH must have no TTL")

	cache.StartCooldown(ctx, 7, 16, "openai", 15, now)
	require.False(t, cache.CooldownActive(ctx, 7, 16, "openai", now), "StartCooldown is a no-op while pinned")

	cache.ClearPinned(ctx, 7, 16, "openai")
	require.False(t, cache.IsPinned(ctx, 7, 16, "openai"))
	foundPin := false
	for _, event := range cache.ListPairQualityEvents(ctx, 7, 16, "openai", 20) {
		if event.Type == service.PairQualityEventPinEnter {
			foundPin = true
		}
	}
	require.True(t, foundPin)
}

func TestPairQualityCache_ExpiryWhilePinnedDoesNotProbe(t *testing.T) {
	cache, _ := newPairQualityTestCache(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 17, 0, 0, 0, time.UTC)
	cache.MarkPinned(ctx, 7, 16, "openai")
	require.NoError(t, cache.rdb.HSet(ctx, smartScheduleCooldownKey("openai", 7), smartScheduleCooldownField(16), now.Add(-time.Minute).Unix()).Err())
	require.False(t, cache.CooldownActive(ctx, 7, 16, "openai", now))
	require.True(t, cache.IsPinned(ctx, 7, 16, "openai"))
	require.False(t, cache.IsProbing(ctx, 7, 16, "openai"), "leftover cooldown expiry on a pinned pair must not enter probing")
}

func intPtrRepo(n int) *int { return &n }
