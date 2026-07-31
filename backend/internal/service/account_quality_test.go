package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildAccountQualityStats_EmptyWindow(t *testing.T) {
	stats := BuildAccountQualityStats(0, 0, 0, nil)
	require.Equal(t, AccountQualityWindowSeconds, stats.WindowSeconds)
	require.Equal(t, int64(0), stats.SuccessCount)
	require.Equal(t, int64(0), stats.ErrorCount)
	require.Nil(t, stats.SuccessRate)
	require.Nil(t, stats.AvgTTFTMs)
	require.Equal(t, int64(0), stats.TTFTSamples)
}

func TestBuildAccountQualityStats_SuccessAndError(t *testing.T) {
	avg := 420.4
	stats := BuildAccountQualityStats(95, 5, 90, &avg)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 0.95, *stats.SuccessRate, 1e-9)
	require.NotNil(t, stats.AvgTTFTMs)
	require.Equal(t, 420, *stats.AvgTTFTMs)
	require.Equal(t, int64(90), stats.TTFTSamples)
}

func TestBuildAccountQualityStats_ErrorsOnly(t *testing.T) {
	stats := BuildAccountQualityStats(0, 3, 0, nil)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 0.0, *stats.SuccessRate, 1e-9)
	require.Nil(t, stats.AvgTTFTMs)
}

func TestBuildAccountQualityStats_TTFTWithoutSamplesIgnored(t *testing.T) {
	avg := 100.0
	stats := BuildAccountQualityStats(10, 0, 0, &avg)
	require.Nil(t, stats.AvgTTFTMs)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 1.0, *stats.SuccessRate, 1e-9)
}
