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

	fail := cache.IngestPairQuality(ctx, 7, 16, 3, false, &ttft)
	require.Equal(t, 1, fail.OKCount)
	require.Equal(t, 0, fail.TTFTCount)
	require.Equal(t, 0.0, *fail.SuccessRate)

	syncOK := cache.IngestPairQuality(ctx, 7, 16, 3, true, nil)
	require.Equal(t, 2, syncOK.OKCount)
	require.Equal(t, 0, syncOK.TTFTCount)

	streamOK := cache.IngestPairQuality(ctx, 7, 16, 3, true, &ttft)
	require.Equal(t, 3, streamOK.OKCount)
	require.Equal(t, 1, streamOK.TTFTCount)

	other := cache.IngestPairQuality(ctx, 7, 17, 3, true, intPtrRepo(800))
	require.Equal(t, 1, other.OKCount)
	require.Equal(t, 800, *other.P50TTFTMs)
	user16 := cache.GetPairQuality(ctx, 7, 16)
	require.Equal(t, 1, user16.TTFTCount)
	require.Equal(t, 90, *user16.P50TTFTMs)

	batch := cache.GetPairQualityBatch(ctx, []int64{7}, 16)
	require.Equal(t, 3, batch[7].OKCount)
	require.Nil(t, cache.GetPairQuality(ctx, 8, 16))
}

func TestPairQualityCache_ExpiryZerosWindows(t *testing.T) {
	cache, _ := newPairQualityTestCache(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	cache.IngestPairQuality(ctx, 7, 16, 3, true, intPtrRepo(40))
	cache.StartCooldown(ctx, 7, 16, 15, now)
	require.NotZero(t, cache.GetPairQuality(ctx, 7, 16).OKCount)
	require.False(t, cache.CooldownActive(ctx, 7, 16, now.Add(16*time.Minute)))
	zeroed := cache.GetPairQuality(ctx, 7, 16)
	require.Equal(t, 0, zeroed.OKCount)
	require.Equal(t, 0, zeroed.TTFTCount)
	events := cache.ListPairQualityEvents(ctx, 7, 16, 20)
	found := false
	for _, event := range events {
		if event.Type == service.PairQualityEventExpiryZero {
			found = true
		}
	}
	require.True(t, found)
	require.True(t, cache.IsProbing(ctx, 7, 16), "expiry must enter probing, not selectable")
	require.False(t, cache.IsProbing(ctx, 7, 17), "other user is not backfilled")
	foundEnter := false
	for _, event := range cache.ListPairQualityEvents(ctx, 7, 16, 20) {
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
	require.NoError(t, rdb.HSet(ctx, accountQualityResumeKey(7),
		accountQualityResumeUserField(16), now.Add(15*time.Minute).Unix(),
		accountQualityResumeWatchingField(16), now.Add(30*time.Minute).Unix(),
	).Err())
	cache.StartCooldown(ctx, 7, 16, 15, now)
	require.False(t, cache.CooldownActive(ctx, 7, 16, now.Add(16*time.Minute)))
	require.True(t, cache.IsProbing(ctx, 7, 16))
	n, err := rdb.HExists(ctx, accountQualityResumeKey(7), accountQualityResumeUserField(16)).Result()
	require.NoError(t, err)
	require.False(t, n, "expiry must ClearUserResume so leftover u:/w: cannot skip graduate")
	watching, err := rdb.HExists(ctx, accountQualityResumeKey(7), accountQualityResumeWatchingField(16)).Result()
	require.NoError(t, err)
	require.False(t, watching)
}

func TestPairQualityCache_NoBackfillWithoutMark(t *testing.T) {
	cache, _ := newPairQualityTestCache(t)
	ctx := context.Background()
	require.False(t, cache.IsProbing(ctx, 7, 16), "Redis miss is not probing")
	require.Empty(t, cache.IsProbingBatch(ctx, []int64{7, 8}, 16))
	cache.MarkProbing(ctx, 7, 16)
	require.True(t, cache.IsProbing(ctx, 7, 16))
	require.True(t, cache.IsProbingBatch(ctx, []int64{7, 8}, 16)[7])
	require.False(t, cache.IsProbingBatch(ctx, []int64{7, 8}, 16)[8])
	cache.GraduateProbing(ctx, 7, 16)
	require.False(t, cache.IsProbing(ctx, 7, 16))
	foundGrad := false
	for _, event := range cache.ListPairQualityEvents(ctx, 7, 16, 20) {
		if event.Type == service.PairQualityEventProbeGraduate {
			foundGrad = true
		}
	}
	require.True(t, foundGrad)
}

func TestPairQualityCache_ZeroClearsAndKeepsIsolation(t *testing.T) {
	cache, _ := newPairQualityTestCache(t)
	ctx := context.Background()
	cache.IngestPairQuality(ctx, 7, 16, 3, true, intPtrRepo(40))
	cache.IngestPairQuality(ctx, 7, 17, 3, true, intPtrRepo(80))
	cache.ZeroPairQuality(ctx, 7, 16, service.PairQualityEventSelectable)
	require.Equal(t, 0, cache.GetPairQuality(ctx, 7, 16).OKCount)
	require.Equal(t, 1, cache.GetPairQuality(ctx, 7, 17).OKCount)
}

func intPtrRepo(n int) *int { return &n }
