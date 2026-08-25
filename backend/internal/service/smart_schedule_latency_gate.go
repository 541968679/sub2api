package service

import (
	"fmt"
	"strings"
)

const (
	DefaultSmartScheduleLatencyK = 2
	DefaultSmartScheduleLatencyC = 2

	// App fallbacks when selectable composite is already on but K/C omitted.
	// Not the zuoge rollout combo (migration 217 is N=10 K=4 C=2).
	DefaultSmartScheduleSchedN = 20
	DefaultSmartScheduleSchedK = 6
	DefaultSmartScheduleSchedC = 3

	// SettingKeyProbeLatencyV2 is the 考察期 rewrite switch (Hold / Q_a / no-zero
	// graduate / underfull C). Default off so selectable-only rollout does not
	// activate that path. Tests may set SmartSchedulePlatformPolicy.ProbeLatencyV2.
	SettingKeyProbeLatencyV2 = "probe_latency_v2"

	// Email used by SQL backfills only. Recommended zuoge selectable combo is 10/4/2.
	SmartScheduleZuogeEmail = "zuoge85@gmail.com"

	LatencyEvalFail    = "fail"
	LatencyEvalPass    = "pass"
	LatencyEvalHold    = "hold"
	LatencyEvalPending = "pending"

	CooldownPhaseProbe      = "考察期"
	CooldownPhaseSelectable = "调度期"
	CooldownPhaseManual     = "人工"

	CooldownSamplePair = "配对"
	CooldownSampleQA   = "Q_a"
)

// SmartScheduleCooldownReason is one breached gate for debug display.
type SmartScheduleCooldownReason struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Phase  string `json:"phase,omitempty"`
	Sample string `json:"sample,omitempty"`
}

func policyProbeLatencyV2(policy *SmartSchedulePlatformPolicy) bool {
	return policy != nil && policy.ProbeLatencyV2
}

func resolveSmartScheduleLatencyKC(policy *SmartSchedulePlatformPolicy) (k, c int) {
	k, c = 0, 0
	if policy == nil || policy.QualityMaxP50TTFTMs == nil {
		return k, c
	}
	if policy.QualityMaxSlowInWindow != nil && *policy.QualityMaxSlowInWindow > 0 {
		k = *policy.QualityMaxSlowInWindow
	} else {
		k = DefaultSmartScheduleLatencyK
	}
	if policy.QualityMaxConsecutiveSlow != nil && *policy.QualityMaxConsecutiveSlow > 0 {
		c = *policy.QualityMaxConsecutiveSlow
	} else {
		c = DefaultSmartScheduleLatencyC
	}
	return k, c
}

func resolveSmartScheduleSchedKC(policy *SmartSchedulePlatformPolicy) (n, k, c int) {
	if policy == nil || !policy.SchedCompositeEnabled() {
		return 0, 0, 0
	}
	n = policy.SchedWindowN()
	if n < 1 {
		n = DefaultSmartScheduleSchedN
	}
	if policy.QualitySchedMaxSlowInWindow != nil {
		if *policy.QualitySchedMaxSlowInWindow > 0 {
			k = *policy.QualitySchedMaxSlowInWindow
		}
	} else {
		k = DefaultSmartScheduleSchedK
	}
	if policy.QualitySchedMaxConsecutiveSlow != nil {
		if *policy.QualitySchedMaxConsecutiveSlow > 0 {
			c = *policy.QualitySchedMaxConsecutiveSlow
		}
	} else {
		c = DefaultSmartScheduleSchedC
	}
	return n, k, c
}

func pairLatencyGate(samples []int, maxP50 *int, k, c, n int, requireFullForConsecutive bool) (block bool, reasons []SmartScheduleCooldownReason) {
	if maxP50 == nil || *maxP50 < 1 {
		return false, nil
	}
	threshold := *maxP50
	if c > 0 {
		canCheckConsecutive := len(samples) >= c
		if requireFullForConsecutive {
			canCheckConsecutive = canCheckConsecutive && len(samples) >= n
		}
		if canCheckConsecutive {
			tail := samples[len(samples)-c:]
			allSlow := true
			vals := make([]string, 0, c)
			for _, v := range tail {
				vals = append(vals, fmt.Sprintf("%d", v))
				if v <= threshold {
					allSlow = false
					break
				}
			}
			if allSlow {
				return true, []SmartScheduleCooldownReason{{
					Code:   "consec",
					Detail: fmt.Sprintf("连续C 末尾%d条>%dms (%s)", c, threshold, strings.Join(vals, ",")),
				}}
			}
		}
	}
	if n < 1 {
		n = DefaultSmartScheduleWindowN
	}
	if len(samples) >= n {
		slow := 0
		for _, v := range samples[len(samples)-n:] {
			if v > threshold {
				slow++
			}
		}
		if k > 0 && slow >= k {
			return true, []SmartScheduleCooldownReason{{
				Code:   "slow_k",
				Detail: fmt.Sprintf("超标K %d/%d>%dms", slow, n, threshold),
			}}
		}
		if p50 := pairQualityP50(samples[len(samples)-n:]); p50 != nil && *p50 > threshold {
			return true, []SmartScheduleCooldownReason{{
				Code:   "p50",
				Detail: fmt.Sprintf("p50 %d>%dms", *p50, threshold),
			}}
		}
	}
	return false, nil
}

// pairSelectableLatencyGate is the 调度期-only C∨K∨p50 helper.
// C is ready at C (do not wait for N). K is ready at K (slow_count >= K).
// p50 still requires a full N-sample window. Probe must not call this.
func pairSelectableLatencyGate(samples []int, maxP50 *int, k, c, n int) (block bool, reasons []SmartScheduleCooldownReason) {
	if maxP50 == nil || *maxP50 < 1 {
		return false, nil
	}
	threshold := *maxP50
	if n < 1 {
		n = DefaultSmartScheduleSchedN
	}
	window := samples
	if len(window) > n {
		window = window[len(window)-n:]
	}

	if c > 0 && len(window) >= c {
		tail := window[len(window)-c:]
		allSlow := true
		vals := make([]string, 0, c)
		for _, v := range tail {
			vals = append(vals, fmt.Sprintf("%d", v))
			if v <= threshold {
				allSlow = false
				break
			}
		}
		if allSlow {
			reasons = append(reasons, SmartScheduleCooldownReason{
				Code:   "consec",
				Detail: fmt.Sprintf("连续C 末尾%d条>%dms (%s)", c, threshold, strings.Join(vals, ",")),
			})
		}
	}

	if k > 0 && len(window) >= k {
		slow := countSlowSamples(window, threshold)
		if slow >= k {
			reasons = append(reasons, SmartScheduleCooldownReason{
				Code:   "slow_k",
				Detail: fmt.Sprintf("超标K %d/%d>%dms", slow, len(window), threshold),
			})
		}
	}

	if len(window) >= n {
		if p50 := pairQualityP50(window); p50 != nil && *p50 > threshold {
			reasons = append(reasons, SmartScheduleCooldownReason{
				Code:   "p50",
				Detail: fmt.Sprintf("p50 %d>%dms", *p50, threshold),
			})
		}
	}
	return len(reasons) > 0, reasons
}

type latencyWindowSnapshot struct {
	enabled       bool
	full          bool
	pass          bool
	fail          bool
	cBroken       bool
	slowCount     int
	underfullSlow bool
	reasons       []SmartScheduleCooldownReason
}

func snapshotLatencyWindow(samples []int, maxP50 *int, k, c, n int, codePrefix string, requireFullForConsecutive bool) latencyWindowSnapshot {
	out := latencyWindowSnapshot{enabled: maxP50 != nil && *maxP50 >= 1}
	if !out.enabled {
		return out
	}
	block, reasons := pairLatencyGate(samples, maxP50, k, c, n, requireFullForConsecutive)
	out.full = len(samples) >= n
	out.slowCount = countSlowSamples(samples, *maxP50)
	if c > 0 {
		canCheckConsecutive := len(samples) >= c
		if requireFullForConsecutive {
			canCheckConsecutive = canCheckConsecutive && len(samples) >= n
		}
		if canCheckConsecutive {
			tail := samples[len(samples)-c:]
			allSlow := true
			for _, v := range tail {
				if v <= *maxP50 {
					allSlow = false
					break
				}
			}
			out.cBroken = allSlow
		}
	}
	out.underfullSlow = !out.full && k > 0 && out.slowCount >= k
	if block {
		out.fail = true
		for i := range reasons {
			reasons[i].Code = codePrefix + reasons[i].Code
		}
		out.reasons = reasons
	} else if out.full {
		out.pass = true
	}
	return out
}

func countSlowSamples(samples []int, threshold int) int {
	n := 0
	for _, v := range samples {
		if v > threshold {
			n++
		}
	}
	return n
}

// evalLatencyWindows evaluates TTFT and duration windows with the same N/K/C gates.
// durationMax nil means the duration window does not participate.
func evalLatencyWindows(ttftSamples, durSamples []int, ttftMax, durMax *int, k, c, n int) (state string, reasons []SmartScheduleCooldownReason) {
	ttft := snapshotLatencyWindow(ttftSamples, ttftMax, k, c, n, "ttft_", false)
	dur := snapshotLatencyWindow(durSamples, durMax, k, c, n, "dur_", false)

	var enabled []latencyWindowSnapshot
	if ttft.enabled {
		enabled = append(enabled, ttft)
	}
	if dur.enabled {
		enabled = append(enabled, dur)
	}
	if len(enabled) == 0 {
		return LatencyEvalPending, nil
	}

	for _, w := range enabled {
		if w.fail {
			reasons = append(reasons, w.reasons...)
		}
	}
	if len(reasons) > 0 {
		return LatencyEvalFail, reasons
	}

	if ttft.enabled && dur.enabled {
		if (ttft.full && ttft.pass && dur.underfullSlow) || (dur.full && dur.pass && ttft.underfullSlow) {
			return LatencyEvalHold, nil
		}
	}

	hasFullPass := false
	for _, w := range enabled {
		if w.full && w.pass {
			hasFullPass = true
		}
	}
	if hasFullPass {
		return LatencyEvalPass, nil
	}
	return LatencyEvalPending, nil
}

func recentLatencySamples(samples []int, n int) []int {
	if n < 1 || len(samples) == 0 {
		return nil
	}
	if len(samples) <= n {
		return append([]int(nil), samples...)
	}
	return append([]int(nil), samples[len(samples)-n:]...)
}

func formatSmartScheduleCooldownDetail(phase, sample string, reasons []SmartScheduleCooldownReason) string {
	if len(reasons) == 0 {
		return ""
	}
	parts := make([]string, 0, len(reasons)+2)
	if phase != "" {
		parts = append(parts, phase)
	}
	if sample != "" {
		parts = append(parts, sample)
	}
	for _, r := range orderCooldownReasons(reasons) {
		if strings.TrimSpace(r.Detail) != "" {
			parts = append(parts, r.Detail)
		}
	}
	return strings.Join(parts, " · ")
}

func orderCooldownReasons(reasons []SmartScheduleCooldownReason) []SmartScheduleCooldownReason {
	order := map[string]int{
		"ttft_consec": 0, "ttft_slow_k": 1, "ttft_p50": 2,
		"dur_consec": 3, "dur_slow_k": 4, "dur_p50": 5,
		"qa_ttft_consec": 10, "qa_ttft_slow_k": 11, "qa_ttft_p50": 12,
		"qa_dur_consec": 13, "qa_dur_slow_k": 14, "qa_dur_p50": 15,
		"success": 20, "and_mixed": 21, "manual": 30,
	}
	out := append([]SmartScheduleCooldownReason(nil), reasons...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if order[out[j].Code] < order[out[i].Code] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func prefixQAReasons(reasons []SmartScheduleCooldownReason) []SmartScheduleCooldownReason {
	if len(reasons) == 0 {
		return nil
	}
	out := make([]SmartScheduleCooldownReason, len(reasons))
	for i, r := range reasons {
		out[i] = r
		switch {
		case strings.HasPrefix(r.Code, "qa_"):
			out[i].Code = r.Code
		case strings.HasPrefix(r.Code, "ttft_"):
			out[i].Code = "qa_ttft_" + strings.TrimPrefix(r.Code, "ttft_")
		case strings.HasPrefix(r.Code, "dur_"):
			out[i].Code = "qa_dur_" + strings.TrimPrefix(r.Code, "dur_")
		default:
			out[i].Code = "qa_" + r.Code
		}
	}
	return out
}
