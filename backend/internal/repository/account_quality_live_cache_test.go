//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestAccountQualityLiveCache_ReplaceDeletesStaleKeys(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()

	p50 := 400
	sampled := &service.AccountQualityStats{
		WindowSeconds: service.AccountQualityWindowSeconds,
		SuccessCount:  2,
		P50TTFTMs:     &p50,
		TTFTSamples:   2,
	}
	require.NoError(t, cache.Replace(ctx, map[int64]*service.AccountQualityStats{
		1: sampled,
		2: sampled,
		3: {WindowSeconds: service.AccountQualityWindowSeconds},
	}))

	got1, err := cache.Get(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, got1)
	require.Equal(t, int64(2), got1.SuccessCount)
	got2, err := cache.Get(ctx, 2)
	require.NoError(t, err)
	require.NotNil(t, got2)
	got3, err := cache.Get(ctx, 3)
	require.NoError(t, err)
	require.Nil(t, got3)

	require.NoError(t, cache.Replace(ctx, map[int64]*service.AccountQualityStats{
		1: sampled,
	}))
	still1, err := cache.Get(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, still1)
	stale2, err := cache.Get(ctx, 2)
	require.NoError(t, err)
	require.Nil(t, stale2, "account that left the candidate set must not keep a stale live key")
}

func TestAccountQualityLiveCache_MarkUserResumeSurvivesReplace(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()

	require.NoError(t, cache.MarkUserResume(ctx, 9, 16))
	before, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	require.True(t, service.UserQualityResumeActive(before, 16, time.Now().UTC()))

	p50 := 400
	require.NoError(t, cache.Replace(ctx, map[int64]*service.AccountQualityStats{
		9: {
			WindowSeconds: service.AccountQualityWindowSeconds,
			SuccessCount:  2,
			P50TTFTMs:     &p50,
			TTFTSamples:   2,
		},
	}))
	after, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	require.NotNil(t, after)
	require.Equal(t, int64(2), after.SuccessCount)
	require.True(t, service.UserQualityResumeActive(after, 16, time.Now().UTC()))
	raw, err := rdb.Get(ctx, accountQualityLiveKey(9)).Result()
	require.NoError(t, err)
	var stored service.AccountQualityStats
	require.NoError(t, json.Unmarshal([]byte(raw), &stored))
	require.Empty(t, stored.ResumeUsers)
	require.Nil(t, stored.AccountResumeUntil)
}

func TestAccountQualityLiveCache_ResumeSurvivesWhenAccountLeavesCandidateSet(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()

	require.NoError(t, cache.MarkUserResume(ctx, 9, 16))
	require.NoError(t, cache.MarkAccountResume(ctx, 9))

	p50 := 400
	require.NoError(t, cache.Replace(ctx, map[int64]*service.AccountQualityStats{
		1: {
			WindowSeconds: service.AccountQualityWindowSeconds,
			SuccessCount:  2,
			P50TTFTMs:     &p50,
			TTFTSamples:   2,
		},
	}))

	got, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	require.NotNil(t, got)
	now := time.Now().UTC()
	require.True(t, service.UserQualityResumeActive(got, 16, now))
	require.True(t, service.AccountQualityResumeActive(got, now))
	staleLive, err := rdb.Exists(ctx, accountQualityLiveKey(9)).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), staleLive)
}

func TestAccountQualityLiveCache_ResumeSurvivesEmptyReplace(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()

	require.NoError(t, cache.MarkUserResume(ctx, 9, 16))
	require.NoError(t, cache.Replace(ctx, map[int64]*service.AccountQualityStats{}))

	got, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	require.True(t, service.UserQualityResumeActive(got, 16, time.Now().UTC()))
}

func TestAccountQualityLiveCache_TwoUserResumesDoNotClobber(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()

	require.NoError(t, cache.MarkUserResume(ctx, 9, 16))
	require.NoError(t, cache.MarkUserResume(ctx, 9, 7))

	got, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	now := time.Now().UTC()
	require.True(t, service.UserQualityResumeActive(got, 16, now))
	require.True(t, service.UserQualityResumeActive(got, 7, now))
}

func TestAccountQualityLiveCache_MarkUserQualityWindowDropsResumedChip(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()

	require.NoError(t, cache.MarkUserResume(ctx, 9, 16))
	before, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	now := time.Now().UTC()
	require.True(t, service.UserQualityResumedChipActive(before, 16, now))

	require.NoError(t, cache.MarkUserQualityWindow(ctx, 9, 16))
	after, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	require.False(t, service.UserQualityResumedChipActive(after, 16, now))
	require.True(t, service.UserQualityResumeActive(after, 16, now))
}

func TestAccountQualityLiveCache_ClearUserResume(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()
	require.NoError(t, cache.MarkUserResume(ctx, 9, 16))
	require.NoError(t, cache.ClearUserResume(ctx, 9, 16))
	got, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	require.False(t, service.UserQualityResumeActive(got, 16, time.Now().UTC()))
	require.False(t, service.UserQualityResumedChipActive(got, 16, time.Now().UTC()))
}

func TestAccountQualityLiveCache_ReplaceMergesResumeIntoCallerMap(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()

	require.NoError(t, cache.MarkAccountResume(ctx, 9))
	p50 := 400
	st := &service.AccountQualityStats{
		WindowSeconds: service.AccountQualityWindowSeconds,
		SuccessCount:  2,
		P50TTFTMs:     &p50,
		TTFTSamples:   2,
	}
	require.NoError(t, cache.Replace(ctx, map[int64]*service.AccountQualityStats{9: st}))
	require.True(t, service.AccountQualityResumeActive(st, time.Now().UTC()))
}

func TestAccountQualityLiveCache_LegacyLiveJSONResumeMigratesOnScan(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewAccountQualityLiveCache(rdb)
	ctx := context.Background()

	until := time.Now().UTC().Add(service.AccountQualityWindow).Unix()
	legacy := service.AccountQualityStats{
		WindowSeconds:      service.AccountQualityWindowSeconds,
		ResumeUsers:        map[string]int64{"16": until},
		AccountResumeUntil: &until,
	}
	payload, err := json.Marshal(legacy)
	require.NoError(t, err)
	require.NoError(t, rdb.Set(ctx, accountQualityLiveKey(9), payload, accountQualityLiveTTL).Err())

	p50 := 400
	require.NoError(t, cache.Replace(ctx, map[int64]*service.AccountQualityStats{
		1: {
			WindowSeconds: service.AccountQualityWindowSeconds,
			SuccessCount:  2,
			P50TTFTMs:     &p50,
			TTFTSamples:   2,
		},
	}))

	got, err := cache.Get(ctx, 9)
	require.NoError(t, err)
	now := time.Now().UTC()
	require.True(t, service.UserQualityResumeActive(got, 16, now))
	require.True(t, service.AccountQualityResumeActive(got, now))
}
