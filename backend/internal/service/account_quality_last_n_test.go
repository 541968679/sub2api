//go:build unit

package service

import (
	"testing"
	"time"

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

func TestApplyAccountQualityLastNIngest_TotalDurationIncludesStreams(t *testing.T) {
	t.Parallel()
	ttft := 40
	duration := 8000
	live := ApplyAccountQualityLastNIngest(nil, 4, true, &ttft, &duration)
	require.Equal(t, []int{40}, live.TTFTMs)
	require.Empty(t, live.DurationMs)
	require.Equal(t, []int{8000}, live.TotalDurationMs)
	require.NotNil(t, live.P50TotalDurationMs)
	require.Equal(t, 8000, *live.P50TotalDurationMs)
	require.NotNil(t, live.P95TotalDurationMs)
	require.Equal(t, 8000, *live.P95TotalDurationMs)

	syncDuration := 1200
	live = ApplyAccountQualityLastNIngest(live, 4, true, nil, &syncDuration)
	require.Equal(t, []int{1200}, live.DurationMs)
	require.Equal(t, []int{8000, 1200}, live.TotalDurationMs)
	stats := live.ToAccountQualityStats()
	require.Equal(t, int64(2), stats.DurationSamples)
	require.NotNil(t, stats.P50DurationMs)
	require.NotNil(t, stats.P95DurationMs)
	require.Equal(t, *live.P50TotalDurationMs, *stats.P50DurationMs)
	require.Equal(t, *live.P95TotalDurationMs, *stats.P95DurationMs)
}

func TestDisplayRateUsesFifteenMinutesNotLastN(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	live := applyAccountQualityLastNIngestAt(nil, 10, LastNOutcomeSuccess, intPtr(100), intPtr(2000), now)
	for i := 0; i < 9; i++ {
		live = applyAccountQualityLastNIngestAt(live, 10, LastNOutcomeTerminal, nil, nil, now)
	}
	require.Equal(t, 10, live.OKCount)
	require.Equal(t, int64(1), live.ToAccountQualityStats().SuccessCount)

	live.RateBuckets = []AccountQualityRateBucket{
		{Minute: now.Add(-8*time.Minute).Unix() / 60, Success: 100},
		{Minute: now.Add(-3*time.Minute).Unix() / 60, Success: 40, Terminal: 10},
		{Minute: now.Unix() / 60, Success: 50},
	}
	stats := live.ToAccountQualityStats()
	require.Equal(t, int64(1), stats.SuccessCount)
	require.Equal(t, int64(90), stats.DisplaySuccessCount)
	require.Equal(t, int64(10), stats.DisplayFailoverErrorCount)
	require.Equal(t, DefaultAccountQualityDisplayRateMinutes*60, stats.DisplayRateWindowSeconds)

	attachDisplayRate(stats, live, now, AccountQualityDisplayRateSpec{Mode: DisplayRateModeMinutes, Minutes: 15, Count: 100})
	require.Equal(t, int64(190), stats.DisplaySuccessCount)
	require.Equal(t, int64(10), stats.DisplayFailoverErrorCount)
	require.Equal(t, 15*60, stats.DisplayRateWindowSeconds)
}

func TestDisplayRateCountUsesTailNotGateWindow(t *testing.T) {
	t.Parallel()
	live := &AccountQualityLastN{
		N:               2,
		Outcomes:        []uint8{LastNOutcomeSuccess, LastNOutcomeSuccess},
		DisplayOutcomes: []uint8{LastNOutcomeSuccess, LastNOutcomeSuccess, LastNOutcomeTerminal, LastNOutcomeTerminal, LastNOutcomeRecovered},
	}
	stats := &AccountQualityStats{}
	attachDisplayRate(stats, live, time.Now().UTC(), AccountQualityDisplayRateSpec{
		Mode: DisplayRateModeCount, Minutes: 5, Count: 3,
	})
	require.Equal(t, int64(0), stats.DisplaySuccessCount)
	require.Equal(t, int64(3), stats.DisplayFailoverErrorCount)
	require.Equal(t, DisplayRateModeCount, stats.DisplayRateMode)
	require.Equal(t, 3, stats.DisplayRateWindowCount)
	require.Equal(t, int64(0), stats.SuccessCount)
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
