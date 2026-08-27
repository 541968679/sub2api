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
// when any latency threshold is set).
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
		Condition:   derefString(policy.QualityCondition),
	}
}

// EvalQuality is the shared 预检 / 正式考察 (and selectable-shaped) judge.
// fail = success-fail ∨/∧ latency-fail, plus and-mixed when both sides are full
// and they disagree. pass = every configured side is pass. Else pending.
func EvalQuality(live *PairQualityLive, knobs QualityEvalKnobs) QualityEvalResult {
	successSide, successReasons := evalSuccessSide(live, knobs)
	latencySide, latencyReasons := evalLatencySide(live, knobs)
	reasons := append([]SmartScheduleCooldownReason{}, successReasons...)
	reasons = append(reasons, latencyReasons...)

	successConfigured := successSide != qualitySideSkip
	latencyConfigured := latencySide != qualitySideSkip
	sFail := successSide == LatencyEvalFail
	lFail := latencySide == LatencyEvalFail
	sPass := successSide == LatencyEvalPass
	lPass := latencySide == LatencyEvalPass
	sFull := sPass || sFail
	lFull := lPass || lFail
	cond := strings.ToLower(strings.TrimSpace(knobs.Condition))

	fail := false
	if cond == QualityHardCloseConditionAnd {
		if sFail && lFail {
			fail = true
		} else if successConfigured && latencyConfigured && sFull && lFull && sFail != lFail {
			fail = true
			reasons = []SmartScheduleCooldownReason{{Code: "and_mixed", Detail: "and 混合"}}
		}
	} else if sFail || lFail {
		fail = true
	}
	if fail {
		return QualityEvalResult{State: LatencyEvalFail, Reasons: orderCooldownReasons(reasons)}
	}
	if successConfigured && !sPass {
		return QualityEvalResult{State: LatencyEvalPending, Reasons: nil}
	}
	if latencyConfigured && !lPass {
		return QualityEvalResult{State: LatencyEvalPending, Reasons: nil}
	}
	if !successConfigured && !latencyConfigured {
		return QualityEvalResult{State: LatencyEvalPending, Reasons: nil}
	}
	return QualityEvalResult{State: LatencyEvalPass, Reasons: nil}
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

func evalLatencySide(live *PairQualityLive, knobs QualityEvalKnobs) (string, []SmartScheduleCooldownReason) {
	ttftOn := knobs.TTFTMax != nil && *knobs.TTFTMax >= 1
	durOn := knobs.DurMax != nil && *knobs.DurMax >= 1
	if !ttftOn && !durOn {
		return qualitySideSkip, nil
	}
	n := knobs.LatencyN
	if n < 1 {
		n = DefaultSmartScheduleWindowN
	}
	var ttft, dur []int
	if live != nil {
		ttft = live.TTFTMs
		dur = live.DurationMs
	}
	var reasons []SmartScheduleCooldownReason
	anyFail := false
	anyFullPass := false
	if ttftOn {
		window := recentLatencySamples(ttft, n)
		if blocked, rs := pairSelectableLatencyGate(window, knobs.TTFTMax, knobs.K, knobs.C, n); blocked {
			anyFail = true
			reasons = append(reasons, prefixLatencyReasonCodes(rs, "ttft_")...)
		} else if len(window) >= n {
			anyFullPass = true
		}
	}
	if durOn {
		window := recentLatencySamples(dur, n)
		if blocked, rs := pairSelectableLatencyGate(window, knobs.DurMax, knobs.K, knobs.C, n); blocked {
			anyFail = true
			reasons = append(reasons, prefixLatencyReasonCodes(rs, "dur_")...)
		} else if len(window) >= n {
			anyFullPass = true
		}
	}
	if anyFail {
		return LatencyEvalFail, reasons
	}
	if anyFullPass {
		return LatencyEvalPass, nil
	}
	return LatencyEvalPending, nil
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
