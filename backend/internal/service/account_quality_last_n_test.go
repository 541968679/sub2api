//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveUserQualityWindowN(t *testing.T) {
	t.Parallel()
	require.Equal(t, DefaultAccountQualityWindowN, ResolveUserQualityWindowN(nil, DefaultAccountQualityWindowN))
	require.Equal(t, 10, ResolveUserQualityWindowN(intPtr(10), 20))
	require.Equal(t, 20, ResolveUserQualityWindowN(nil, 20))
	require.Equal(t, 1, ResolveUserQualityWindowN(intPtr(0), 20))
	require.Equal(t, 100, ResolveUserQualityWindowN(intPtr(250), 20))
}

func TestProjectAccountQualityLastN_TrimsToResolvedN(t *testing.T) {
	t.Parallel()
	live := ApplyAccountQualityLastNIngest(nil, 20, true, intPtr(100), nil)
	for i := 0; i < 7; i++ {
		live = ApplyAccountQualityLastNIngest(live, 20, true, intPtr(100+i), nil)
	}
	require.Equal(t, 8, live.OKCount)
	projected := ProjectAccountQualityLastN(live, 3)
	require.Equal(t, 3, projected.N)
	require.Equal(t, 3, projected.OKCount)
	require.Equal(t, 3, projected.TTFTCount)
}

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
	live := ApplyAccountQualityLastNIngest(nil, n, false, intPtr(900), nil)
	require.Equal(t, 0, live.TTFTCount)
	require.Equal(t, 1, live.OKCount)
	require.False(t, live.OK[0])

	live = ApplyAccountQualityLastNIngest(live, n, true, nil, nil)
	require.Equal(t, 0, live.TTFTCount)
	require.Equal(t, 2, live.OKCount)

	live = ApplyAccountQualityLastNIngest(live, n, true, intPtr(40), nil)
	require.Equal(t, 1, live.TTFTCount)
	require.Equal(t, 3, live.OKCount)
	require.Equal(t, 40, live.TTFTMs[0])
	require.NotNil(t, live.P50TTFTMs)
	require.Equal(t, 40, *live.P50TTFTMs)
	require.NotNil(t, live.SuccessRate)
	require.InDelta(t, 2.0/3.0, *live.SuccessRate, 1e-9)
}

func TestStampAccountQualityLatencyKC_MatchesPairCell(t *testing.T) {
	t.Parallel()
	p50 := 3000
	stats := &AccountQualityStats{}
	StampAccountQualityLatencyKC(stats, []int{100, 4000, 4100}, QualityEvalKnobs{
		TTFTMax:  &p50,
		LatencyN: 3,
		K:        3,
		C:        2,
	})
	require.NotNil(t, stats.TTFTSlowCount)
	require.Equal(t, 2, *stats.TTFTSlowCount)
	require.NotNil(t, stats.TTFTConsecutiveSlow)
	require.Equal(t, 2, *stats.TTFTConsecutiveSlow)
	require.NotNil(t, stats.QualitySchedMaxSlowInWindow)
	require.Equal(t, 3, *stats.QualitySchedMaxSlowInWindow)
	require.NotNil(t, stats.QualitySchedMaxConsecutiveSlow)
	require.Equal(t, 2, *stats.QualitySchedMaxConsecutiveSlow)
}

func TestAccountQualityLastN_ToAccountQualityStats_StampsN(t *testing.T) {
	t.Parallel()
	live := ApplyAccountQualityLastNIngest(nil, 4, true, intPtr(100), nil)
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

func TestApplyAccountQualityLastNRecovered_CompareNotTerminal(t *testing.T) {
	t.Parallel()
	live := ApplyAccountQualityLastNIngest(nil, 20, true, intPtr(40), nil)
	live = ApplyAccountQualityLastNRecovered(live, 20)
	stats := live.ToAccountQualityStats()
	require.Equal(t, int64(1), stats.SuccessCount)
	require.Equal(t, int64(0), stats.TerminalErrorCount)
	require.Equal(t, int64(1), stats.FailoverErrorCount)
	require.Equal(t, int64(0), stats.ErrorCount)
	require.Equal(t, 1, live.OKCount)
	require.True(t, live.OK[0])
	require.NotNil(t, stats.FailoverErrorRate)
	require.InDelta(t, 0.5, *stats.FailoverErrorRate, 1e-9)
	require.NotNil(t, stats.TerminalErrorRate)
	require.InDelta(t, 0, *stats.TerminalErrorRate, 1e-9)

	ApplyAccountQualityScheduleCaliber(stats, false)
	require.Equal(t, int64(0), stats.ErrorCount)
	ApplyAccountQualityScheduleCaliber(stats, true)
	require.Equal(t, int64(1), stats.ErrorCount)
}

func TestApplyAccountQualityLastNRecovered_OnlyRecovered(t *testing.T) {
	t.Parallel()
	live := ApplyAccountQualityLastNRecovered(nil, 20)
	stats := live.ToAccountQualityStats()
	require.Equal(t, int64(0), stats.SuccessCount)
	require.Equal(t, int64(0), stats.TerminalErrorCount)
	require.Equal(t, int64(1), stats.FailoverErrorCount)
	require.Equal(t, 0, live.OKCount)
	require.NotNil(t, stats.FailoverErrorRate)
	require.InDelta(t, 1.0, *stats.FailoverErrorRate, 1e-9)
}

func TestAccountQualityLastN_DecodeLegacyOKWithoutOutcomes(t *testing.T) {
	t.Parallel()
	live := &AccountQualityLastN{
		N:  20,
		OK: []bool{true, false, true},
	}
	RecomputeAccountQualityLastN(live)
	stats := live.ToAccountQualityStats()
	require.Equal(t, int64(2), stats.SuccessCount)
	require.Equal(t, int64(1), stats.TerminalErrorCount)
	require.Equal(t, int64(1), stats.FailoverErrorCount)
}

func TestEvaluateAccountQualityHardClose_UnfilledLastNDoesNotJudge(t *testing.T) {
	t.Parallel()
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		n := 4
		cfg.AccountQualityWindowN = &n
		cfg.MinSuccessSamples = 4
		cfg.MinTTFTSamples = 4
	})
	live := ApplyAccountQualityLastNIngest(nil, 4, true, intPtr(9000), nil)
	live = ApplyAccountQualityLastNIngest(live, 4, true, intPtr(9000), nil)
	live = ApplyAccountQualityLastNIngest(live, 4, false, nil, nil)
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
	live := ApplyAccountQualityLastNIngest(nil, 2, true, intPtr(4000), nil)
	live = ApplyAccountQualityLastNIngest(live, 2, true, intPtr(4000), nil)
	shouldPause, reason := EvaluateAccountQualityHardClose(live.ToAccountQualityStats(), cfg, false)
	require.True(t, shouldPause)
	require.Contains(t, reason, "p50=")
}
