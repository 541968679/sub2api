package service

import (
	"fmt"
	"strings"
	"time"
)

const (
	qualitySideSkip = "skip"

	CooldownPhasePrecheck = "考察预检"
)

// QualityEvalKnobs is one stage's success + latency knobs. Stages swap samples
// and these numbers; they share EvalQuality.
type QualityEvalKnobs struct {
	SuccessRate *float64
	SuccessN    int
	TTFTMax     *int
	DurMax      *int
	LatencyN    int
	K           int
	C           int
	Condition   string
}

// QualityEvalResult is fail (cool) / pass (leave) / pending.
type QualityEvalResult struct {
	State   string
	Reasons []SmartScheduleCooldownReason
}

// AccountQualityPrecheckSample is one account-global completion for 考察预检.
// It is not the last-N Q_a JSON (that has no user_id / ts).
type AccountQualityPrecheckSample struct {
	UnixTS     int64
	UserID     int64
	OK         bool
	TTFTMs     *int
	DurationMs *int
}

// ProbeQualityKnobs is the 考察期 / 考察预检 knob set (N首字, K/C default 2/2
// when any latency threshold is set). Condition is ignored at runtime.
func ProbeQualityKnobs(policy *SmartSchedulePlatformPolicy) QualityEvalKnobs {
	if policy == nil {
		return QualityEvalKnobs{
			SuccessN: DefaultSmartScheduleWindowN,
			LatencyN: DefaultSmartScheduleWindowN,
		}
	}
	k, c := resolveSmartScheduleLatencyKC(policy)
	return QualityEvalKnobs{
		SuccessRate: policy.QualityMinSuccessRate,
		SuccessN:    policy.SuccessWindowN(),
		TTFTMax:     policy.QualityMaxP50TTFTMs,
		DurMax:      policy.QualityMaxP50DurationMs,
		LatencyN:    policy.TTFTWindowN(),
		K:           k,
		C:           c,
	}
}

// SchedQualityKnobs is the 调度期 / 软冷却 knob set. Composite on → sched N/K/C;
// composite off → N首字 p50 only (no invented K/C). Condition is ignored.
func SchedQualityKnobs(policy *SmartSchedulePlatformPolicy) QualityEvalKnobs {
	if policy == nil {
		return QualityEvalKnobs{
			SuccessN: DefaultSmartScheduleWindowN,
			LatencyN: DefaultSmartScheduleWindowN,
		}
	}
	latencyN := policy.TTFTWindowN()
	k, c := 0, 0
	if policy.SchedCompositeEnabled() {
		n, sk, sc := resolveSmartScheduleSchedKC(policy)
		if n > 0 {
			latencyN = n
		}
		k, c = sk, sc
	}
	return QualityEvalKnobs{
		SuccessRate: policy.QualityMinSuccessRate,
		SuccessN:    policy.SuccessWindowN(),
		TTFTMax:     policy.QualityMaxP50TTFTMs,
		DurMax:      policy.QualityMaxP50DurationMs,
		LatencyN:    latencyN,
		K:           k,
		C:           c,
	}
}

func qualityKnobsConfigured(knobs QualityEvalKnobs) bool {
	if knobs.SuccessRate != nil {
		return true
	}
	if knobs.TTFTMax != nil && *knobs.TTFTMax >= 1 {
		return true
	}
	return knobs.DurMax != nil && *knobs.DurMax >= 1
}

type qualityMetricJudge struct {
	state   string
	reasons []SmartScheduleCooldownReason
}

// EvalQuality is the shared 预检 / 正式考察 / 调度 / 软 meet judge.
// Per configured metric: fail OR-exits; pass is AND-enter. Condition is ignored.
// Zero configured metrics → pending.
func EvalQuality(live *PairQualityLive, knobs QualityEvalKnobs) QualityEvalResult {
	var judges []qualityMetricJudge
	if knobs.SuccessRate != nil {
		side, reasons := evalSuccessSide(live, knobs)
		judges = append(judges, qualityMetricJudge{state: side, reasons: reasons})
	}
	var ttft, dur []int
	if live != nil {
		ttft = live.TTFTMs
		dur = live.DurationMs
	}
	n := knobs.LatencyN
	if n < 1 {
		n = DefaultSmartScheduleWindowN
	}
	judges = append(judges, evalFanMetrics(ttft, knobs.TTFTMax, knobs.K, knobs.C, n, "ttft_")...)
	judges = append(judges, evalFanMetrics(dur, knobs.DurMax, knobs.K, knobs.C, n, "dur_")...)

	if len(judges) == 0 {
		return QualityEvalResult{State: LatencyEvalPending, Reasons: nil}
	}
	var reasons []SmartScheduleCooldownReason
	allPass := true
	anyFail := false
	for _, j := range judges {
		if j.state == LatencyEvalFail {
			anyFail = true
			reasons = append(reasons, j.reasons...)
		}
		if j.state != LatencyEvalPass {
			allPass = false
		}
	}
	if anyFail {
		return QualityEvalResult{State: LatencyEvalFail, Reasons: orderCooldownReasons(reasons)}
	}
	if allPass {
		return QualityEvalResult{State: LatencyEvalPass, Reasons: nil}
	}
	return QualityEvalResult{State: LatencyEvalPending, Reasons: nil}
}

// evalFanMetrics judges p50 / K / C independently on one latency fan.
// Unset fan threshold skips p50 and that fan's K/C. Reuses pairSelectableLatencyGate for fail.
func evalFanMetrics(samples []int, maxP50 *int, k, c, n int, prefix string) []qualityMetricJudge {
	if maxP50 == nil || *maxP50 < 1 {
		return nil
	}
	if n < 1 {
		n = DefaultSmartScheduleWindowN
	}
	window := recentLatencySamples(samples, n)
	_, rs := pairSelectableLatencyGate(window, maxP50, k, c, n)
	rs = prefixLatencyReasonCodes(rs, prefix)

	p50Fail, kFail, cFail := false, false, false
	var p50Reasons, kReasons, cReasons []SmartScheduleCooldownReason
	for _, r := range rs {
		switch strings.TrimPrefix(r.Code, prefix) {
		case "p50":
			p50Fail = true
			p50Reasons = append(p50Reasons, r)
		case "slow_k":
			kFail = true
			kReasons = append(kReasons, r)
		case "consec":
			cFail = true
			cReasons = append(cReasons, r)
		}
	}

	out := make([]qualityMetricJudge, 0, 3)
	out = append(out, metricReadyState(p50Fail, len(window) >= n, p50Reasons))
	if k > 0 {
		out = append(out, metricReadyState(kFail, len(window) >= k, kReasons))
	}
	if c > 0 {
		out = append(out, metricReadyState(cFail, len(window) >= c, cReasons))
	}
	return out
}

func metricReadyState(failed, ready bool, reasons []SmartScheduleCooldownReason) qualityMetricJudge {
	if failed {
		return qualityMetricJudge{state: LatencyEvalFail, reasons: reasons}
	}
	if ready {
		return qualityMetricJudge{state: LatencyEvalPass}
	}
	return qualityMetricJudge{state: LatencyEvalPending}
}

func evalSuccessSide(live *PairQualityLive, knobs QualityEvalKnobs) (string, []SmartScheduleCooldownReason) {
	if knobs.SuccessRate == nil {
		return qualitySideSkip, nil
	}
	nOK := knobs.SuccessN
	if nOK < 1 {
		nOK = DefaultSmartScheduleWindowN
	}
	if live == nil || live.OKCount < nOK {
		return LatencyEvalPending, nil
	}
	rate := 0.0
	if live.SuccessRate != nil {
		rate = *live.SuccessRate
	}
	if live.SuccessRate == nil || rate < *knobs.SuccessRate {
		return LatencyEvalFail, []SmartScheduleCooldownReason{{
			Code:   "success",
			Detail: fmt.Sprintf("成功率 %.2f<%.2f", rate, *knobs.SuccessRate),
		}}
	}
	return LatencyEvalPass, nil
}

// FilterPrecheckSamples drops this user and samples older than since.
// Order is preserved (callers should pass oldest-first).
func FilterPrecheckSamples(samples []AccountQualityPrecheckSample, excludeUserID int64, since time.Time) []AccountQualityPrecheckSample {
	if len(samples) == 0 {
		return nil
	}
	sinceUnix := since.UTC().Unix()
	out := make([]AccountQualityPrecheckSample, 0, len(samples))
	for _, sample := range samples {
		if excludeUserID > 0 && sample.UserID == excludeUserID {
			continue
		}
		if sample.UnixTS < sinceUnix {
			continue
		}
		out = append(out, sample)
	}
	return out
}

// PairLiveFromPrecheckSamples applies the shared ingest rules in chronological order.
func PairLiveFromPrecheckSamples(samples []AccountQualityPrecheckSample, nTTFT, nOK int) *PairQualityLive {
	var live *PairQualityLive
	for _, sample := range samples {
		live = ApplyPairQualityIngestWindows(live, nTTFT, nOK, sample.OK, sample.TTFTMs, sample.DurationMs)
	}
	return live
}

// FormatProbePrecheckCooldownDetail is the 考察预检 cooldown reason string.
func FormatProbePrecheckCooldownDetail(reasons []SmartScheduleCooldownReason) string {
	return formatSmartScheduleCooldownDetail(CooldownPhasePrecheck, "", reasons)
}
