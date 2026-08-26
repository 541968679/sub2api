//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvalLatencyWindows_PassPendingFailHold(t *testing.T) {
	t.Parallel()
	ttftMax := 100
	durMax := 80000
	k, c, n := 2, 2, 5

	state, _ := evalLatencyWindows(nil, nil, &ttftMax, nil, k, c, n)
	require.Equal(t, LatencyEvalPending, state)

	state, reasons := evalLatencyWindows([]int{120, 130, 140, 150, 160}, nil, &ttftMax, nil, k, c, n)
	require.Equal(t, LatencyEvalFail, state, "full window with all slow must fail")
	require.NotEmpty(t, reasons)

	state, _ = evalLatencyWindows([]int{50, 60, 70, 80, 90}, nil, &ttftMax, nil, k, c, n)
	require.Equal(t, LatencyEvalPass, state)

	state, reasons = evalLatencyWindows([]int{50, 60, 120, 130, 140}, nil, &ttftMax, nil, k, c, n)
	require.Equal(t, LatencyEvalFail, state, "full window with K=2 slow must fail")
	require.NotEmpty(t, reasons)

	// Hold: TTFT full+pass, duration underfull with >=K slow but C not tripped
	state, reasons = evalLatencyWindows(
		[]int{50, 60, 70, 80, 90},
		[]int{90000, 50000, 91000},
		&ttftMax, &durMax, k, c, n,
	)
	require.Equal(t, LatencyEvalHold, state)
	require.Empty(t, reasons)

	// Duration skipped when nil max
	state, _ = evalLatencyWindows([]int{50, 60, 70, 80, 90}, []int{90000}, &ttftMax, nil, k, c, n)
	require.Equal(t, LatencyEvalPass, state)
}

func TestPairLatencyGate_ConsecutiveUnderfull(t *testing.T) {
	t.Parallel()
	max := 100
	block, reasons := pairLatencyGate([]int{150, 160}, &max, 2, 2, 5, false)
	require.True(t, block, "C=2 consecutive slow underfull must block")
	require.NotEmpty(t, reasons)
}

func TestResolveSmartScheduleLatencyKC_DefaultsOnlyWithTTFT(t *testing.T) {
	t.Parallel()
	k, c := resolveSmartScheduleLatencyKC(&SmartSchedulePlatformPolicy{})
	require.Equal(t, 0, k)
	require.Equal(t, 0, c)

	p50 := 50
	k, c = resolveSmartScheduleLatencyKC(&SmartSchedulePlatformPolicy{QualityMaxP50TTFTMs: &p50})
	require.Equal(t, DefaultSmartScheduleLatencyK, k)
	require.Equal(t, DefaultSmartScheduleLatencyC, c)
}

func TestResolveSmartScheduleSchedKC_OnlyConfiguredKC(t *testing.T) {
	t.Parallel()
	n, k, c := resolveSmartScheduleSchedKC(&SmartSchedulePlatformPolicy{})
	require.Equal(t, 0, n)
	require.Equal(t, 0, k)
	require.Equal(t, 0, c)

	p50 := 50
	p50Only := &SmartSchedulePlatformPolicy{QualityMaxP50TTFTMs: &p50, QualityMinTTFTSamples: intPtr(5)}
	require.False(t, p50Only.SchedCompositeEnabled())
	n, k, c = resolveSmartScheduleSchedKC(p50Only)
	require.Equal(t, 0, n)
	require.Equal(t, 0, k)
	require.Equal(t, 0, c)

	kOnly := &SmartSchedulePlatformPolicy{QualityMaxP50TTFTMs: &p50, QualityMinTTFTSamples: intPtr(5), QualitySchedMaxSlowInWindow: intPtr(4)}
	require.True(t, kOnly.SchedCompositeEnabled())
	n, k, c = resolveSmartScheduleSchedKC(kOnly)
	require.Equal(t, 5, n)
	require.Equal(t, 4, k)
	require.Equal(t, 0, c)

	cOnly := &SmartSchedulePlatformPolicy{QualityMaxP50TTFTMs: &p50, QualityMinTTFTSamples: intPtr(5), QualitySchedMaxConsecutiveSlow: intPtr(2)}
	n, k, c = resolveSmartScheduleSchedKC(cOnly)
	require.Equal(t, 5, n)
	require.Equal(t, 0, k)
	require.Equal(t, 2, c)

	n, k, c = resolveSmartScheduleSchedKC(withSchedComposite(&SmartSchedulePlatformPolicy{QualityMaxP50TTFTMs: &p50}))
	require.Equal(t, latencySchedN, n)
	require.Equal(t, latencySchedK, k)
	require.Equal(t, latencySchedC, c)
}

const (
	latencyGateMs    = 6000
	latencyFastMs    = 1000
	latencySlowMs    = 9000
	latencyDurGateMs = 80000
	latencyDurSlowMs = 90000
	latencyProbeN    = 5
	latencyProbeK    = 2
	latencyProbeC    = 2
	latencySchedN    = 20
	latencySchedK    = 6
	latencySchedC    = 3
)

func withSchedComposite(p *SmartSchedulePlatformPolicy) *SmartSchedulePlatformPolicy {
	if p == nil {
		p = &SmartSchedulePlatformPolicy{}
	}
	// Explicit 20/6/3 fixture so selectable tests stay independent of app defaults 10/4/2.
	p.QualitySchedWindowN = intPtr(latencySchedN)
	p.QualitySchedMaxSlowInWindow = intPtr(latencySchedK)
	p.QualitySchedMaxConsecutiveSlow = intPtr(latencySchedC)
	return p
}

func withProbeLatencyV2(p *SmartSchedulePlatformPolicy) *SmartSchedulePlatformPolicy {
	if p == nil {
		p = &SmartSchedulePlatformPolicy{}
	}
	p.ProbeLatencyV2 = true
	return p
}

func repeatLatencyMs(v, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func latencyTail(fastCount, slowCount, fastMs, slowMs int) []int {
	out := repeatLatencyMs(fastMs, fastCount)
	return append(out, repeatLatencyMs(slowMs, slowCount)...)
}

func TestPairLatencyGate_ProbeTable(t *testing.T) {
	t.Parallel()
	gate := latencyGateMs
	cases := []struct {
		name      string
		samples   []int
		wantBlock bool
		wantCode  string
	}{
		{
			name:      "C_underfull_two_consecutive_slow",
			samples:   []int{latencySlowMs, latencySlowMs},
			wantBlock: true,
			wantCode:  "consec",
		},
		{
			name:      "underfull_single_slow_no_block",
			samples:   []int{latencySlowMs},
			wantBlock: false,
		},
		{
			name:      "full_window_all_fast_pass",
			samples:   repeatLatencyMs(latencyFastMs, latencyProbeN),
			wantBlock: false,
		},
		{
			name:      "full_window_K_slow",
			samples:   latencyScatterSlow(2, 3, latencyFastMs, latencySlowMs),
			wantBlock: true,
			wantCode:  "slow_k",
		},
		{
			name:      "full_window_p50_breach",
			samples:   repeatLatencyMs(latencySlowMs, latencyProbeN),
			wantBlock: true,
		},
		{
			name:      "jitter_one_slow_in_full_window",
			samples:   latencyTail(4, 1, latencyFastMs, latencySlowMs),
			wantBlock: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			block, reasons := pairLatencyGate(tc.samples, &gate, latencyProbeK, latencyProbeC, latencyProbeN, false)
			require.Equal(t, tc.wantBlock, block)
			if tc.wantCode != "" {
				require.NotEmpty(t, reasons)
				require.Equal(t, tc.wantCode, reasons[0].Code)
			}
		})
	}
}

func TestPairSelectableLatencyGate_ReadyAtKC(t *testing.T) {
	t.Parallel()
	gate := latencyGateMs
	zero := 0
	cases := []struct {
		name      string
		samples   []int
		k, c, n   int
		wantBlock bool
		wantCode  string
	}{
		{
			name:      "C_three_consecutive_with_only_three_samples",
			samples:   repeatLatencyMs(latencySlowMs, 3),
			k:         latencySchedK,
			c:         latencySchedC,
			n:         latencySchedN,
			wantBlock: true,
			wantCode:  "consec",
		},
		{
			name:      "five_samples_two_slow_no_K",
			samples:   latencyScatterSlow(2, 1, latencyFastMs, latencySlowMs),
			k:         latencySchedK,
			c:         latencySchedC,
			n:         latencySchedN,
			wantBlock: false,
		},
		{
			name:      "six_of_six_slow_K_without_N20",
			samples:   repeatLatencyMs(latencySlowMs, 6),
			k:         latencySchedK,
			c:         0,
			n:         latencySchedN,
			wantBlock: true,
			wantCode:  "slow_k",
		},
		{
			name:      "six_slow_in_ten_rest_fast_K",
			samples:   append(repeatLatencyMs(latencySlowMs, 6), repeatLatencyMs(latencyFastMs, 4)...),
			k:         latencySchedK,
			c:         latencySchedC,
			n:         latencySchedN,
			wantBlock: true,
			wantCode:  "slow_k",
		},
		{
			name:      "five_slow_in_nineteen_no_K_no_p50",
			samples:   append(repeatLatencyMs(latencySlowMs, 5), repeatLatencyMs(latencyFastMs, 14)...),
			k:         latencySchedK,
			c:         latencySchedC,
			n:         latencySchedN,
			wantBlock: false,
		},
		{
			name:      "full_20_p50_K_closed_no_C",
			samples:   append(repeatLatencyMs(latencySlowMs, 11), repeatLatencyMs(latencyFastMs, 9)...),
			k:         zero,
			c:         zero,
			n:         latencySchedN,
			wantBlock: true,
			wantCode:  "p50",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			block, reasons := pairSelectableLatencyGate(tc.samples, &gate, tc.k, tc.c, tc.n)
			require.Equal(t, tc.wantBlock, block)
			if tc.wantCode != "" {
				require.NotEmpty(t, reasons)
				require.Equal(t, tc.wantCode, reasons[0].Code)
			}
		})
	}
}

func TestEvalLatencyWindows_Table(t *testing.T) {
	t.Parallel()
	ttftGate := latencyGateMs
	durGate := latencyDurGateMs
	k, c, n := latencyProbeK, latencyProbeC, latencyProbeN

	cases := []struct {
		name      string
		ttft      []int
		dur       []int
		durMax    *int
		wantState string
		wantCodes []string
	}{
		{
			name:      "pending_empty",
			wantState: LatencyEvalPending,
		},
		{
			name:      "pass_ttft_full_fast",
			ttft:      repeatLatencyMs(latencyFastMs, n),
			wantState: LatencyEvalPass,
		},
		{
			name:      "fail_ttft_K",
			ttft:      latencyScatterSlow(2, 3, latencyFastMs, latencySlowMs),
			wantState: LatencyEvalFail,
			wantCodes: []string{"ttft_slow_k"},
		},
		{
			name:      "hold_ttft_pass_dur_underfull_K_slow",
			ttft:      repeatLatencyMs(latencyFastMs, n),
			dur:       []int{latencyDurSlowMs, latencyFastMs * 50, latencyDurSlowMs},
			durMax:    &durGate,
			wantState: LatencyEvalHold,
		},
		{
			name:      "pass_dur_disabled",
			ttft:      repeatLatencyMs(latencyFastMs, n),
			dur:       []int{latencyDurSlowMs},
			wantState: LatencyEvalPass,
		},
		{
			name:      "fail_dur_full_slow",
			ttft:      repeatLatencyMs(latencyFastMs, n),
			dur:       repeatLatencyMs(latencyDurSlowMs, n),
			durMax:    &durGate,
			wantState: LatencyEvalFail,
			wantCodes: []string{"dur_consec", "dur_slow_k"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			state, reasons := evalLatencyWindows(tc.ttft, tc.dur, &ttftGate, tc.durMax, k, c, n)
			require.Equal(t, tc.wantState, state)
			if len(tc.wantCodes) == 0 {
				require.Empty(t, reasons)
				return
			}
			codes := make([]string, len(reasons))
			for i, r := range reasons {
				codes[i] = r.Code
			}
			matched := false
			for _, want := range tc.wantCodes {
				if containsString(codes, want) {
					matched = true
					break
				}
			}
			require.True(t, matched, "expected one of %v in %v", tc.wantCodes, codes)
		})
	}
}

func TestFormatSmartScheduleCooldownDetail_PhaseAndBranch(t *testing.T) {
	t.Parallel()
	probeDetail := formatSmartScheduleCooldownDetail(CooldownPhaseProbe, CooldownSamplePair, []SmartScheduleCooldownReason{{
		Code: "ttft_consec", Detail: "连续C 末尾2条>6000ms (9000,9000)",
	}})
	require.Contains(t, probeDetail, CooldownPhaseProbe)
	require.Contains(t, probeDetail, CooldownSamplePair)
	require.Contains(t, probeDetail, "连续C")

	schedDetail := formatSmartScheduleCooldownDetail(CooldownPhaseSelectable, CooldownSamplePair, []SmartScheduleCooldownReason{{
		Code: "ttft_slow_k", Detail: "超标K 6/20>6000ms",
	}})
	require.Contains(t, schedDetail, CooldownPhaseSelectable)
	require.Contains(t, schedDetail, "超标K")

	qaDetail := formatSmartScheduleCooldownDetail(CooldownPhaseProbe, CooldownSampleQA, prefixQAReasons([]SmartScheduleCooldownReason{{
		Code: "ttft_p50", Detail: "p50 9000>6000ms",
	}}))
	require.Contains(t, qaDetail, CooldownSampleQA)
	require.Contains(t, qaDetail, "p50")
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
