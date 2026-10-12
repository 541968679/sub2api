package service

import (
	"encoding/json"
	"sort"
	"time"
)

const (
	burnSampleRingMax       = 12
	burnRateMinSpan         = 5 * time.Minute
	burnRateLookback        = 15 * time.Minute
	burnRateMaxETA          = 30 * 24 * time.Hour
	burnRateIncreaseEpsilon = 0.05
	burnRateZeroEpsilon     = 1e-9

	burnKindBalanceUSD            = "balance_usd"
	burnKindBalanceCNY            = "balance_cny"
	burnKindOAuth7dRemainingPct   = "oauth_7d_remaining_pct"
	burnKindFleet7dRemainingUnits = "fleet_7d_remaining_units"

	extraKeyBurnSamples                    = "burn_samples"
	extraKeyUpstreamBalanceUSD             = "upstream_balance_usd"
	extraKeyUpstreamBalanceAt              = "upstream_balance_at"
	extraKeyUpstreamBalanceErr             = "upstream_balance_error"
	extraKeyUpstreamBalanceSrc             = "upstream_balance_source"
	extraKeyUpstreamBalanceCurrency        = "upstream_balance_currency"
	extraKeyUpstreamBalanceWalletUSD       = "upstream_balance_wallet_usd"
	extraKeyUpstreamBalanceSubscriptionUSD = "upstream_balance_subscription_usd"
	// Display-only fields (never affect scheduling / admission).
	extraKeyUpstreamBalanceUsedUSD = "upstream_balance_used_usd"
	extraKeyDisplayBalanceTotalUSD = "display_balance_total_usd"
	extraKeyDisplayBalanceUsedUSD  = "display_balance_used_usd" // last manual/auto used shown
)

// BurnSample is one time-series point used for burn-rate regression.
type BurnSample struct {
	T    time.Time `json:"t"`
	V    float64   `json:"v"`
	Kind string    `json:"kind"`
}

// BurnRateResult is the fitted burn-rate and optional remaining duration.
type BurnRateResult struct {
	RatePerHour  float64
	ETASeconds   *float64
	SampleCount  int
	Insufficient bool
	// Rate is zero (no consumption) rather than insufficient samples.
	Idle bool
}

// AppendBurnSample appends a sample to the ring. If value rises above the last
// point (recharge / window reset), the ring is reset to the new epoch.
func AppendBurnSample(existing []BurnSample, sample BurnSample, max int) []BurnSample {
	if max <= 0 {
		max = burnSampleRingMax
	}
	if sample.T.IsZero() {
		sample.T = time.Now().UTC()
	}
	sample.T = sample.T.UTC()

	out := make([]BurnSample, 0, len(existing)+1)
	for _, s := range existing {
		if s.Kind != "" && sample.Kind != "" && s.Kind != sample.Kind {
			continue
		}
		out = append(out, s)
	}

	if len(out) > 0 {
		last := out[len(out)-1]
		// Same timestamp: replace last value.
		if !sample.T.After(last.T) && !sample.T.Before(last.T) {
			out[len(out)-1] = sample
			return out
		}
		// Recharge / reset: start a new epoch.
		if sample.V > last.V+burnRateIncreaseEpsilon {
			return []BurnSample{sample}
		}
	}

	out = append(out, sample)
	if len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}

// ComputeBurnRate estimates hourly consumption as the average drop from the
// oldest to the newest sample inside the latest recharge epoch and the last
// burnRateLookback. A span shorter than minSpan is insufficient, so a brief
// dip is not scaled up to an hourly rate. ETA uses the last remaining value.
func ComputeBurnRate(samples []BurnSample, now time.Time, minSpan time.Duration) BurnRateResult {
	if minSpan <= 0 {
		minSpan = burnRateMinSpan
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	if len(samples) < 2 {
		return BurnRateResult{SampleCount: len(samples), Insufficient: true}
	}

	pts := make([]BurnSample, len(samples))
	copy(pts, samples)
	sort.Slice(pts, func(i, j int) bool { return pts[i].T.Before(pts[j].T) })

	// Keep only the latest epoch after a recharge or window reset.
	start := 0
	for i := 1; i < len(pts); i++ {
		if pts[i].V > pts[i-1].V+burnRateIncreaseEpsilon {
			start = i
		}
	}
	pts = pts[start:]

	cutoff := now.Add(-burnRateLookback)
	recent := make([]BurnSample, 0, len(pts))
	for _, p := range pts {
		if !p.T.Before(cutoff) {
			recent = append(recent, p)
		}
	}
	pts = recent
	if len(pts) < 2 {
		return BurnRateResult{SampleCount: len(pts), Insufficient: true}
	}

	span := pts[len(pts)-1].T.Sub(pts[0].T)
	if span < minSpan {
		return BurnRateResult{SampleCount: len(pts), Insufficient: true}
	}
	hours := span.Hours()
	if hours <= 0 {
		return BurnRateResult{SampleCount: len(pts), Insufficient: true}
	}

	drop := pts[0].V - pts[len(pts)-1].V
	result := BurnRateResult{SampleCount: len(pts)}
	if drop <= burnRateZeroEpsilon {
		result.Idle = true
		result.RatePerHour = 0
		return result
	}
	rate := drop / hours
	result.RatePerHour = rate

	remaining := pts[len(pts)-1].V
	if remaining < 0 {
		remaining = 0
	}
	if rate > burnRateZeroEpsilon && remaining > burnRateZeroEpsilon {
		eta := time.Duration(remaining/rate*float64(time.Hour) + 0.5)
		if eta > burnRateMaxETA {
			eta = burnRateMaxETA
		}
		sec := eta.Seconds()
		result.ETASeconds = &sec
	}
	return result
}

// ParseBurnSamplesFromExtra reads burn_samples from account.extra.
func ParseBurnSamplesFromExtra(extra map[string]any, kind string) []BurnSample {
	if extra == nil {
		return nil
	}
	raw, ok := extra[extraKeyBurnSamples]
	if !ok || raw == nil {
		return nil
	}
	// Re-marshal to normalize map slices from JSON/map[string]any.
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var items []struct {
		T    string  `json:"t"`
		V    float64 `json:"v"`
		Kind string  `json:"kind"`
	}
	if err := json.Unmarshal(b, &items); err != nil {
		return nil
	}
	out := make([]BurnSample, 0, len(items))
	for _, it := range items {
		if kind != "" && it.Kind != "" && it.Kind != kind {
			continue
		}
		ts, err := time.Parse(time.RFC3339, it.T)
		if err != nil {
			ts, err = time.Parse(time.RFC3339Nano, it.T)
			if err != nil {
				continue
			}
		}
		out = append(out, BurnSample{T: ts.UTC(), V: it.V, Kind: it.Kind})
	}
	return out
}

// SerializeBurnSamples converts samples for account.extra storage.
func SerializeBurnSamples(samples []BurnSample) []map[string]any {
	out := make([]map[string]any, 0, len(samples))
	for _, s := range samples {
		out = append(out, map[string]any{
			"t":    s.T.UTC().Format(time.RFC3339),
			"v":    s.V,
			"kind": s.Kind,
		})
	}
	return out
}

// RemainingPctFromUtilization converts used percent (0-100) to remaining percent.
func RemainingPctFromUtilization(utilization float64) float64 {
	r := 100 - utilization
	if r < 0 {
		return 0
	}
	if r > 100 {
		return 100
	}
	return r
}

// ApplyBurnRateToUsage fills UsageInfo burn fields from a BurnRateResult.
func ApplyBurnRateToUsage(info *UsageInfo, result BurnRateResult, unit string) {
	if info == nil {
		return
	}
	info.BurnRateUnit = unit
	info.BurnSampleCount = result.SampleCount
	info.BurnInsufficient = result.Insufficient
	if result.Insufficient {
		return
	}
	if result.Idle {
		zero := 0.0
		info.BurnRatePerHour = &zero
		info.BurnETASeconds = nil
		info.BurnInsufficient = false
		return
	}
	rate := result.RatePerHour
	info.BurnRatePerHour = &rate
	info.BurnETASeconds = result.ETASeconds
}
