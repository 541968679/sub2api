package service

import "math"

// ProfitControlPolicy is the per-group profit admission gate.
// When Enabled is false (default), every account qualifies — baseline scheduling
// behavior is unchanged.
type ProfitControlPolicy struct {
	Enabled      bool
	MinMargin    float64
	SafetyBuffer float64
}

// AccountQualifiesForProfit reports whether an account's cost multiplier U may
// serve a request with downstream multiplier D under the policy.
// Rule when enabled: U <= D * (1 - min_margin - safety_buffer) within epsilon.
// Invalid U (negative/NaN/Inf) is rejected when enabled; zero U is allowed.
func AccountQualifiesForProfit(accountRate, downstreamRate float64, p ProfitControlPolicy) bool {
	if !p.Enabled {
		return true
	}
	if math.IsNaN(accountRate) || math.IsInf(accountRate, 0) || accountRate < 0 {
		return false
	}
	if math.IsNaN(downstreamRate) || math.IsInf(downstreamRate, 0) || downstreamRate <= 0 {
		return false
	}
	threshold := downstreamRate * (1 - p.MinMargin - p.SafetyBuffer)
	if threshold < 0 {
		threshold = 0
	}
	const eps = 1e-9
	return accountRate <= threshold*(1+eps)+eps
}

// DefaultProfitControlPolicy is off — used when group has no profit settings.
func DefaultProfitControlPolicy() ProfitControlPolicy {
	return ProfitControlPolicy{Enabled: false, MinMargin: 0, SafetyBuffer: 0}
}

// FilterAccountsByProfitControl removes accounts that fail the group profit gate.
// When policy is disabled (default), the input slice is returned unchanged.
func FilterAccountsByProfitControl(accounts []*Account, downstreamRate float64, p ProfitControlPolicy) []*Account {
	if !p.Enabled || len(accounts) == 0 {
		return accounts
	}
	out := make([]*Account, 0, len(accounts))
	for _, acc := range accounts {
		if acc == nil {
			continue
		}
		if AccountQualifiesForProfit(acc.BillingRateMultiplier(), downstreamRate, p) {
			out = append(out, acc)
		}
	}
	return out
}
