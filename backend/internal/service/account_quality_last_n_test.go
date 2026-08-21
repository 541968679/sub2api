//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAccountQualityWindowN(t *testing.T) {
	t.Parallel()
	require.Equal(t, DefaultAccountQualityWindowN, NormalizeAccountQualityWindowN(nil, nil, nil))
	require.Equal(t, 14, NormalizeAccountQualityWindowN(intPtr(14), intPtr(20), intPtr(8)))
	require.Equal(t, 20, NormalizeAccountQualityWindowN(nil, intPtr(20), intPtr(10)))
	require.Equal(t, 8, NormalizeAccountQualityWindowN(nil, nil, intPtr(8)))
	require.Equal(t, 1, NormalizeAccountQualityWindowN(intPtr(0), nil, nil))
	require.Equal(t, 100, NormalizeAccountQualityWindowN(intPtr(250), nil, nil))
}

func TestApplyAccountQualityLastNIngest_Rules(t *testing.T) {
	t.Parallel()
	n := 3
	live := ApplyAccountQualityLastNIngest(nil, n, false, intPtr(900))
	require.Equal(t, 0, live.TTFTCount)
	require.Equal(t, 1, live.OKCount)
	require.False(t, live.OK[0])

	live = ApplyAccountQualityLastNIngest(live, n, true, nil)
	require.Equal(t, 0, live.TTFTCount)
	require.Equal(t, 2, live.OKCount)

	live = ApplyAccountQualityLastNIngest(live, n, true, intPtr(40))
	require.Equal(t, 1, live.TTFTCount)
	require.Equal(t, 3, live.OKCount)
	require.Equal(t, 40, live.TTFTMs[0])
	require.NotNil(t, live.P50TTFTMs)
	require.Equal(t, 40, *live.P50TTFTMs)
	require.NotNil(t, live.SuccessRate)
	require.InDelta(t, 2.0/3.0, *live.SuccessRate, 1e-9)
}

func TestAccountQualityLastN_ToAccountQualityStats_StampsN(t *testing.T) {
	t.Parallel()
	live := ApplyAccountQualityLastNIngest(nil, 4, true, intPtr(100))
	stats := live.ToAccountQualityStats()
	require.Equal(t, 4, stats.N)
	require.Equal(t, 4, stats.WindowN)
	require.Equal(t, 4, stats.AccountQualityWindowN)
	require.Equal(t, int64(1), stats.SuccessCount)
	require.Equal(t, int64(0), stats.ErrorCount)
	require.Equal(t, int64(1), stats.TTFTSamples)
	require.NotNil(t, stats.P50TTFTMs)
	require.Equal(t, 100, *stats.P50TTFTMs)
}

func TestEvaluateAccountQualityHardClose_UnfilledLastNDoesNotJudge(t *testing.T) {
	t.Parallel()
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		n := 4
		cfg.AccountQualityWindowN = &n
		cfg.MinSuccessSamples = 4
		cfg.MinTTFTSamples = 4
	})
	live := ApplyAccountQualityLastNIngest(nil, 4, true, intPtr(9000))
	live = ApplyAccountQualityLastNIngest(live, 4, true, intPtr(9000))
	live = ApplyAccountQualityLastNIngest(live, 4, false, nil)
	shouldPause, reason := EvaluateAccountQualityHardClose(live.ToAccountQualityStats(), cfg, false)
	require.False(t, shouldPause)
	require.Empty(t, reason)
}

func TestEvaluateAccountQualityHardClose_UserGateKeepsSplitFloors(t *testing.T) {
	t.Parallel()
	p50 := 3000
	rate := 0.9
	cfg := QualityHardCloseSettings{
		Enabled:           true,
		MaxP50TTFTMs:      &p50,
		MinSuccessRate:    &rate,
		MinSuccessSamples: 20,
		MinTTFTSamples:    10,
		Condition:         QualityHardCloseConditionOr,
	}
	stats := qualityStats(1, 0, 10, 4000, 1)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.True(t, shouldPause)
	require.Contains(t, reason, "p50=")
}

func TestEvaluateAccountQualityHardClose_FullLastNJudges(t *testing.T) {
	t.Parallel()
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		n := 2
		cfg.AccountQualityWindowN = &n
		cfg.MinSuccessSamples = 2
		cfg.MinTTFTSamples = 2
	})
	live := ApplyAccountQualityLastNIngest(nil, 2, true, intPtr(4000))
	live = ApplyAccountQualityLastNIngest(live, 2, true, intPtr(4000))
	shouldPause, reason := EvaluateAccountQualityHardClose(live.ToAccountQualityStats(), cfg, false)
	require.True(t, shouldPause)
	require.Contains(t, reason, "p50=")
}
