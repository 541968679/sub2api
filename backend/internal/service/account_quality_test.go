package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildAccountQualityStats_EmptyWindow(t *testing.T) {
	stats := BuildAccountQualityStats(0, 0, TTFTAggregate{})
	require.Equal(t, AccountQualityWindowSeconds, stats.WindowSeconds)
	require.Equal(t, int64(0), stats.SuccessCount)
	require.Equal(t, int64(0), stats.ErrorCount)
	require.Nil(t, stats.SuccessRate)
	require.Nil(t, stats.AvgTTFTMs)
	require.Nil(t, stats.P50TTFTMs)
	require.Nil(t, stats.P95TTFTMs)
	require.Nil(t, stats.MaxTTFTMs)
	require.Equal(t, int64(0), stats.TTFTSamples)
}

func TestBuildAccountQualityStats_SuccessAndError(t *testing.T) {
	avg := 420.4
	p50 := 300.0
	p95 := 900.2
	max := 5000.0
	stats := BuildAccountQualityStats(95, 5, TTFTAggregate{
		Samples: 90,
		Avg:     &avg,
		P50:     &p50,
		P95:     &p95,
		Max:     &max,
	})
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 0.95, *stats.SuccessRate, 1e-9)
	require.NotNil(t, stats.AvgTTFTMs)
	require.Equal(t, 420, *stats.AvgTTFTMs)
	require.NotNil(t, stats.P50TTFTMs)
	require.Equal(t, 300, *stats.P50TTFTMs)
	require.NotNil(t, stats.P95TTFTMs)
	require.Equal(t, 900, *stats.P95TTFTMs)
	require.NotNil(t, stats.MaxTTFTMs)
	require.Equal(t, 5000, *stats.MaxTTFTMs)
	require.Equal(t, int64(90), stats.TTFTSamples)
}

func TestBuildAccountQualityStats_ErrorsOnly(t *testing.T) {
	stats := BuildAccountQualityStats(0, 3, TTFTAggregate{})
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 0.0, *stats.SuccessRate, 1e-9)
	require.Nil(t, stats.AvgTTFTMs)
	require.Nil(t, stats.P50TTFTMs)
}

func TestQualityResumeHelpers(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	fresh := &AccountQualityStats{SuccessCount: 2}
	old := &AccountQualityStats{}
	SetUserQualityResume(old, 16, now.Add(AccountQualityWindow))
	SetAccountQualityResume(old, now.Add(5*time.Minute))

	MergeQualityResume(fresh, old, now)
	require.True(t, UserQualityResumeActive(fresh, 16, now))
	require.True(t, AccountQualityResumeActive(fresh, now))
	require.True(t, HasActiveQualityResume(fresh, now))

	MergeQualityResume(fresh, old, now.Add(20*time.Minute))
	require.False(t, UserQualityResumeActive(fresh, 16, now.Add(20*time.Minute)))
	require.False(t, AccountQualityResumeActive(fresh, now.Add(20*time.Minute)))
	require.False(t, HasActiveQualityResume(fresh, now.Add(20*time.Minute)))
}

func TestApplyUserQualityResumeTwoPhase(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	stats := &AccountQualityStats{}
	ApplyUserQualityResume(stats, 16, now)
	require.True(t, UserQualityResumedChipActive(stats, 16, now))
	require.True(t, UserQualityResumeActive(stats, 16, now))

	afterResume := now.Add(AccountQualityWindow + time.Minute)
	require.False(t, UserQualityResumedChipActive(stats, 16, afterResume))
	require.True(t, UserQualityResumeActive(stats, 16, afterResume))

	clickAt := now.Add(2 * time.Minute)
	ApplyUserQualityWindowStart(stats, 16, clickAt)
	require.False(t, UserQualityResumedChipActive(stats, 16, clickAt))
	require.True(t, UserQualityResumeActive(stats, 16, clickAt))
	require.False(t, UserQualityResumeActive(stats, 16, clickAt.Add(AccountQualityWindow+time.Minute)))
}

func TestHasAccountQualitySamples(t *testing.T) {
	require.False(t, HasAccountQualitySamples(nil))
	require.False(t, HasAccountQualitySamples(BuildAccountQualityStats(0, 0, TTFTAggregate{})))
	require.True(t, HasAccountQualitySamples(BuildAccountQualityStats(1, 0, TTFTAggregate{})))
	require.True(t, HasAccountQualitySamples(BuildAccountQualityStats(0, 1, TTFTAggregate{})))
	require.True(t, HasAccountQualitySamples(BuildAccountQualityStats(0, 0, TTFTAggregate{Samples: 1})))
}

func TestTruncateToAccountQualitySnapshotTime(t *testing.T) {
	got := TruncateToAccountQualitySnapshotTime(time.Date(2026, 8, 14, 12, 7, 30, 0, time.UTC))
	require.Equal(t, time.Date(2026, 8, 14, 12, 5, 0, 0, time.UTC), got)

	cst := time.FixedZone("CST", 8*3600)
	got = TruncateToAccountQualitySnapshotTime(time.Date(2026, 8, 14, 20, 7, 30, 0, cst))
	require.Equal(t, time.Date(2026, 8, 14, 12, 5, 0, 0, time.UTC), got)
}

func TestSnapshotFromAccountQualityStats_MatchesLiveNullsAndTruncates(t *testing.T) {
	captured := time.Date(2026, 8, 14, 12, 7, 30, 0, time.UTC)
	errorsOnly := BuildAccountQualityStats(0, 3, TTFTAggregate{})
	row := SnapshotFromAccountQualityStats(7, captured, errorsOnly)

	require.Equal(t, int64(7), row.AccountID)
	require.Equal(t, time.Date(2026, 8, 14, 12, 5, 0, 0, time.UTC), row.CapturedAt)
	require.Equal(t, AccountQualityWindowSeconds, row.WindowSeconds)
	require.Equal(t, int64(0), row.SuccessCount)
	require.Equal(t, int64(3), row.ErrorCount)
	require.NotNil(t, row.SuccessRate)
	require.InDelta(t, 0.0, *row.SuccessRate, 1e-9)
	require.Nil(t, row.AvgTTFTMs)
	require.Nil(t, row.P50TTFTMs)
	require.Nil(t, row.P95TTFTMs)
	require.Nil(t, row.MaxTTFTMs)
	require.Equal(t, int64(0), row.TTFTSamples)

	item := row.ToHistoryItem()
	require.Equal(t, row.CapturedAt, item.CapturedAt)
	require.Equal(t, row.WindowSeconds, item.WindowSeconds)
	require.Equal(t, row.SuccessCount, item.SuccessCount)
	require.Equal(t, row.ErrorCount, item.ErrorCount)
	require.Equal(t, row.SuccessRate, item.SuccessRate)
	require.Nil(t, item.P50TTFTMs)
	require.Equal(t, row.TTFTSamples, item.TTFTSamples)
}

func TestBuildAccountQualityStats_TTFTWithoutSamplesIgnored(t *testing.T) {
	avg := 100.0
	stats := BuildAccountQualityStats(10, 0, TTFTAggregate{Samples: 0, Avg: &avg})
	require.Nil(t, stats.AvgTTFTMs)
	require.Nil(t, stats.P50TTFTMs)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 1.0, *stats.SuccessRate, 1e-9)
}
