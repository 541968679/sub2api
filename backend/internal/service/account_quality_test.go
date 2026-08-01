package service

import (
	"testing"

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

func TestBuildAccountQualityStats_TTFTWithoutSamplesIgnored(t *testing.T) {
	avg := 100.0
	stats := BuildAccountQualityStats(10, 0, TTFTAggregate{Samples: 0, Avg: &avg})
	require.Nil(t, stats.AvgTTFTMs)
	require.Nil(t, stats.P50TTFTMs)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 1.0, *stats.SuccessRate, 1e-9)
}
