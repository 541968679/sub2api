package service

import (
	"fmt"
	"strings"
)

// softCooldownMeets is the locked positive-pass for a cooling pair's soft window.
// Configured gates must be full and not broken. Underfull is not a pass.
// Do not treat pairQualityBlocks==false (fail-open) as a pass.
func softCooldownMeets(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) bool {
	if live == nil || policy == nil {
		return false
	}
	successOn := policy.QualityMinSuccessRate != nil
	ttftOn := policy.QualityMaxP50TTFTMs != nil
	durOn := policy.QualityMaxP50DurationMs != nil
	if !successOn && !ttftOn && !durOn {
		return false
	}
	nOK := policy.SuccessWindowN()
	nLat := softCooldownLatencyN(policy)

	successPass := !successOn || softCooldownSuccessMeets(live, policy, nOK)
	latencyOn := ttftOn || durOn
	latencyPass := !latencyOn || softCooldownLatencyGateMeets(live, policy, nLat)

	cond := strings.ToLower(strings.TrimSpace(derefString(policy.QualityCondition)))
	if cond == QualityHardCloseConditionAnd {
		return successPass && latencyPass
	}
	return (successOn && successPass) || (latencyOn && latencyPass)
}

func softCooldownLatencyN(policy *SmartSchedulePlatformPolicy) int {
	if policy != nil && policy.SchedCompositeEnabled() {
		return policy.SchedWindowN()
	}
	if policy == nil {
		return DefaultSmartScheduleWindowN
	}
	return policy.TTFTWindowN()
}

func softCooldownSuccessMeets(live *PairQualityLive, policy *SmartSchedulePlatformPolicy, nOK int) bool {
	if live == nil || policy == nil || policy.QualityMinSuccessRate == nil || nOK < 1 {
		return false
	}
	if live.OKCount < nOK || live.SuccessRate == nil {
		return false
	}
	return *live.SuccessRate >= *policy.QualityMinSuccessRate
}

func softCooldownLatencyGateMeets(live *PairQualityLive, policy *SmartSchedulePlatformPolicy, n int) bool {
	if live == nil || policy == nil || n < 1 {
		return false
	}
	if policy.QualityMaxP50TTFTMs != nil && live.TTFTCount < n {
		return false
	}
	if policy.QualityMaxP50DurationMs != nil && live.DurationCount < n {
		return false
	}
	blocked, _ := pairQualitySelectableLatencyBlocked(live, policy)
	return !blocked
}

func softCooldownProgressView(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) *SoftCooldownProgress {
	nOK := DefaultSmartScheduleWindowN
	nLat := DefaultSmartScheduleWindowN
	if policy != nil {
		nOK = policy.SuccessWindowN()
		nLat = softCooldownLatencyN(policy)
	}
	out := &SoftCooldownProgress{NTTFT: nLat, NOK: nOK}
	if live != nil {
		out.TTFTCount = live.TTFTCount
		out.OKCount = live.OKCount
		out.DurationCount = live.DurationCount
	}
	if policy != nil && policy.QualityMaxP50DurationMs != nil {
		out.NDuration = nLat
	}
	return out
}

func softCooldownMeetDetail(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) string {
	p := softCooldownProgressView(live, policy)
	if p == nil {
		return "soft_cooldown"
	}
	if p.NDuration > 0 {
		return fmt.Sprintf("soft_cooldown ttft=%d/%d ok=%d/%d dur=%d/%d", p.TTFTCount, p.NTTFT, p.OKCount, p.NOK, p.DurationCount, p.NDuration)
	}
	return fmt.Sprintf("soft_cooldown ttft=%d/%d ok=%d/%d", p.TTFTCount, p.NTTFT, p.OKCount, p.NOK)
}
