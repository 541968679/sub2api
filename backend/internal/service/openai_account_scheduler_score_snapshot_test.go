//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOpenAIAccountSchedulerScoreSnapshot_UsesRuntimeEWMA(t *testing.T) {
	t.Parallel()
	stats := newOpenAIAccountRuntimeStats()
	slow := 4000
	stats.report(1, true, &slow)
	stats.report(1, false, &slow)
	fast := 1000
	stats.report(2, true, &fast)

	weights := GatewayOpenAIWSSchedulerScoreWeightsView{Priority: 1, ErrorRate: 1, TTFT: 1}
	got := buildOpenAIAccountSchedulerScoreSnapshot([]*Account{
		{ID: 1, Priority: 1},
		{ID: 2, Priority: 1},
	}, nil, weights, false, stats)

	require.Greater(t, got[2].BaseScore, got[1].BaseScore)
	require.InDelta(t, 1, got[2].Factors.TTFTTerm, 1e-9)
	require.InDelta(t, 0, got[1].Factors.TTFTTerm, 1e-9)
	require.InDelta(t, 0.2, got[1].Factors.ErrorRate, 1e-9)
	require.InDelta(t, 0.8, got[1].Factors.ErrorTerm, 1e-9)
}

func TestBuildOpenAIAccountSchedulerScoreSnapshot_EmptyStatsMatchNeutralFactors(t *testing.T) {
	t.Parallel()
	weights := GatewayOpenAIWSSchedulerScoreWeightsView{Priority: 1, ErrorRate: 0.8, TTFT: 0.5}
	got := buildOpenAIAccountSchedulerScoreSnapshot([]*Account{{ID: 7, Priority: 3}}, nil, weights, false, nil)
	require.InDelta(t, 1+0.8+0.25, got[7].BaseScore, 1e-9)
	require.InDelta(t, 0.25, got[7].Factors.TTFTTerm, 1e-9)
	require.False(t, got[7].Factors.HasTTFT)
}
