//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvalQuality_Table(t *testing.T) {
	t.Parallel()
	p50 := 200
	rate := 0.9
	n := 3
	knobs := QualityEvalKnobs{
		SuccessRate: &rate,
		SuccessN:    n,
		TTFTMax:     &p50,
		LatencyN:    n,
		K:           2,
		C:           2,
		Condition:   QualityHardCloseConditionOr,
	}

	t.Run("pending_underfull_success_and_ttft", func(t *testing.T) {
		t.Parallel()
		live := ApplyPairQualityIngestWindows(nil, n, n, true, intPtr(40), nil)
		ev := EvalQuality(live, knobs)
		require.Equal(t, LatencyEvalPending, ev.State)
	})

	t.Run("fail_success_or", func(t *testing.T) {
		t.Parallel()
		var live *PairQualityLive
		live = ApplyPairQualityIngestWindows(live, n, n, false, nil, nil)
		live = ApplyPairQualityIngestWindows(live, n, n, false, nil, nil)
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(40), nil)
		ev := EvalQuality(live, knobs)
		require.Equal(t, LatencyEvalFail, ev.State)
		require.Equal(t, "success", ev.Reasons[0].Code)
	})

	t.Run("fail_C_ready_at_C", func(t *testing.T) {
		t.Parallel()
		var live *PairQualityLive
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(900), nil)
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(900), nil)
		ev := EvalQuality(live, QualityEvalKnobs{TTFTMax: &p50, LatencyN: n, K: 2, C: 2})
		require.Equal(t, LatencyEvalFail, ev.State)
		require.Equal(t, "ttft_consec", ev.Reasons[0].Code)
	})

	t.Run("fail_K_ready_at_K", func(t *testing.T) {
		t.Parallel()
		var live *PairQualityLive
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(40), nil)
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(900), nil)
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(900), nil)
		ev := EvalQuality(live, QualityEvalKnobs{TTFTMax: &p50, LatencyN: n, K: 2, C: 2})
		require.Equal(t, LatencyEvalFail, ev.State)
		require.Contains(t, []string{"ttft_slow_k", "ttft_consec"}, ev.Reasons[0].Code)
	})

	t.Run("pass_full_fast", func(t *testing.T) {
		t.Parallel()
		var live *PairQualityLive
		for i := 0; i < n; i++ {
			live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(40), nil)
		}
		ev := EvalQuality(live, knobs)
		require.Equal(t, LatencyEvalPass, ev.State)
	})

	t.Run("and_mixed_fail", func(t *testing.T) {
		t.Parallel()
		andKnobs := knobs
		andKnobs.Condition = QualityHardCloseConditionAnd
		var live *PairQualityLive
		for i := 0; i < n; i++ {
			live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(40), nil)
		}
		require.Equal(t, LatencyEvalPass, EvalQuality(live, knobs).State)
		// success pass (3/3) + latency fail (C)
		live = nil
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(900), nil)
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(900), nil)
		live = ApplyPairQualityIngestWindows(live, n, n, true, intPtr(900), nil)
		ev := EvalQuality(live, andKnobs)
		require.Equal(t, LatencyEvalFail, ev.State)
		require.Equal(t, "and_mixed", ev.Reasons[0].Code)
	})

	t.Run("duration_skipped_when_unconfigured", func(t *testing.T) {
		t.Parallel()
		var live *PairQualityLive
		for i := 0; i < n; i++ {
			live = ApplyPairQualityIngestWindows(live, n, n, true, nil, intPtr(90000))
		}
		ev := EvalQuality(live, QualityEvalKnobs{TTFTMax: &p50, LatencyN: n, K: 2, C: 2})
		require.Equal(t, LatencyEvalPending, ev.State)
	})

	t.Run("duration_fail_when_configured", func(t *testing.T) {
		t.Parallel()
		dur := 800
		var live *PairQualityLive
		live = ApplyPairQualityIngestWindows(live, n, n, true, nil, intPtr(90000))
		live = ApplyPairQualityIngestWindows(live, n, n, true, nil, intPtr(90000))
		ev := EvalQuality(live, QualityEvalKnobs{DurMax: &dur, LatencyN: n, K: 2, C: 2})
		require.Equal(t, LatencyEvalFail, ev.State)
		require.Equal(t, "dur_consec", ev.Reasons[0].Code)
	})
}

func TestFilterPrecheckSamples_ExcludeSelfAndTime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	samples := []AccountQualityPrecheckSample{
		{UnixTS: now.Add(-10 * time.Minute).Unix(), UserID: 17, OK: true, TTFTMs: intPtr(40)},
		{UnixTS: now.Add(-1 * time.Minute).Unix(), UserID: 16, OK: true, TTFTMs: intPtr(900)},
		{UnixTS: now.Add(-1 * time.Minute).Unix(), UserID: 17, OK: true, TTFTMs: intPtr(50)},
	}
	got := FilterPrecheckSamples(samples, 16, now.Add(-5*time.Minute))
	require.Len(t, got, 1)
	require.Equal(t, int64(17), got[0].UserID)
	require.Equal(t, 50, *got[0].TTFTMs)
}
