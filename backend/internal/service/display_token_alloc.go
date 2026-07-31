package service

import "math"

// Default display-layer controls (settings KV / user override resolve to these when unset).
const (
	DefaultDisplayCacheTokenMaxMult         = 1.3
	DefaultDisplayOutputResidualGrowthRatio = 1.5
	MaxDisplayCacheTokenMaxMult              = 100.0
	MaxDisplayOutputResidualGrowthRatio      = 10.0
)

// DisplayTokenAllocInput is the real (billing-stored) usage plus display prices and caps.
type DisplayTokenAllocInput struct {
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int

	InputCost     float64
	OutputCost    float64
	CacheReadCost float64

	// Display unit prices (per token). nil / non-positive = that component path is inactive.
	DisplayInputPrice     *float64
	DisplayOutputPrice    *float64
	DisplayCacheReadPrice *float64

	// CacheTokenMaxMult (M): display_cache_read ≤ real_cache_read * M.
	// <=0 means DefaultDisplayCacheTokenMaxMult.
	CacheTokenMaxMult float64

	// OutputResidualGrowthRatio (α): G_extra ≤ α * G_own on output.
	// nil means DefaultDisplayOutputResidualGrowthRatio; non-nil may be 0.
	OutputResidualGrowthRatio *float64
}

// DisplayTokenAllocResult is the user-visible token/cost breakdown after allocation.
type DisplayTokenAllocResult struct {
	InputTokens     int
	OutputTokens    int
	CacheReadTokens int

	InputCost     float64
	OutputCost    float64
	CacheReadCost float64
}

// ResolveDisplayCacheTokenMaxMult returns a validated M (>0), defaulting when unset/invalid.
func ResolveDisplayCacheTokenMaxMult(userOverride *float64, global float64) float64 {
	if userOverride != nil && *userOverride > 0 && !math.IsNaN(*userOverride) && !math.IsInf(*userOverride, 0) {
		return clampDisplayCacheTokenMaxMult(*userOverride)
	}
	if global > 0 && !math.IsNaN(global) && !math.IsInf(global, 0) {
		return clampDisplayCacheTokenMaxMult(global)
	}
	return DefaultDisplayCacheTokenMaxMult
}

// ResolveDisplayOutputResidualGrowthRatio returns a validated α (≥0), defaulting when unset.
func ResolveDisplayOutputResidualGrowthRatio(global float64, set bool) float64 {
	if set && global >= 0 && !math.IsNaN(global) && !math.IsInf(global, 0) {
		return clampDisplayOutputResidualGrowthRatio(global)
	}
	return DefaultDisplayOutputResidualGrowthRatio
}

func clampDisplayCacheTokenMaxMult(v float64) float64 {
	if v > MaxDisplayCacheTokenMaxMult {
		return MaxDisplayCacheTokenMaxMult
	}
	if v <= 0 {
		return DefaultDisplayCacheTokenMaxMult
	}
	return v
}

func clampDisplayOutputResidualGrowthRatio(v float64) float64 {
	if v > MaxDisplayOutputResidualGrowthRatio {
		return MaxDisplayOutputResidualGrowthRatio
	}
	if v < 0 {
		return DefaultDisplayOutputResidualGrowthRatio
	}
	return v
}

func effectiveCacheTokenMaxMult(m float64) float64 {
	if m <= 0 || math.IsNaN(m) || math.IsInf(m, 0) {
		return DefaultDisplayCacheTokenMaxMult
	}
	return clampDisplayCacheTokenMaxMult(m)
}

func effectiveOutputResidualGrowthRatio(alpha *float64) float64 {
	if alpha == nil {
		return DefaultDisplayOutputResidualGrowthRatio
	}
	if math.IsNaN(*alpha) || math.IsInf(*alpha, 0) {
		return DefaultDisplayOutputResidualGrowthRatio
	}
	return clampDisplayOutputResidualGrowthRatio(*alpha)
}

// AllocateDisplayTokens applies bounded cache amplify (M) and output-first residual
// sink with growth ratio α (cache residual only — D6). Component own costs back-calc
// on themselves first; actual_cost is not part of this function.
//
// When display cache price is unset, cache tokens/cost are left unchanged and no
// residual is generated (same as leaving real values).
func AllocateDisplayTokens(in DisplayTokenAllocInput) DisplayTokenAllocResult {
	out := DisplayTokenAllocResult{
		InputTokens:     in.InputTokens,
		OutputTokens:    in.OutputTokens,
		CacheReadTokens: in.CacheReadTokens,
		InputCost:       in.InputCost,
		OutputCost:      in.OutputCost,
		CacheReadCost:   in.CacheReadCost,
	}

	m := effectiveCacheTokenMaxMult(in.CacheTokenMaxMult)
	alpha := effectiveOutputResidualGrowthRatio(in.OutputResidualGrowthRatio)

	residual := 0.0

	canSinkResidual := (in.DisplayInputPrice != nil && *in.DisplayInputPrice > 0) ||
		(in.DisplayOutputPrice != nil && *in.DisplayOutputPrice > 0 && in.OutputTokens > 0 && in.OutputCost > 0)

	// --- Cache read: cost back-calc capped by M × real tokens ---
	// Only rewrite when residual (if any) can still be explained on output/input.
	// Otherwise keep real cache tokens/cost (legacy: cache-only display price is a no-op).
	if in.DisplayCacheReadPrice != nil && *in.DisplayCacheReadPrice > 0 &&
		in.CacheReadTokens > 0 && in.CacheReadCost > 0 && canSinkResidual {
		pCache := *in.DisplayCacheReadPrice
		ideal := int(math.Round(in.CacheReadCost / pCache))
		if ideal < 0 {
			ideal = 0
		}
		capTokens := int(math.Round(float64(in.CacheReadTokens) * m))
		if capTokens < 0 {
			capTokens = 0
		}
		displayCache := ideal
		if displayCache > capTokens {
			displayCache = capTokens
		}
		displayCacheCost := float64(displayCache) * pCache
		out.CacheReadTokens = displayCache
		out.CacheReadCost = displayCacheCost
		if premium := in.CacheReadCost - displayCacheCost; premium > 0 {
			residual = premium
		}
	}

	// --- Output: own cost back-calc, then residual under α × G_own ---
	realOut := in.OutputTokens
	outOwn := realOut
	outOwnCost := in.OutputCost
	if in.DisplayOutputPrice != nil && *in.DisplayOutputPrice > 0 && in.OutputTokens > 0 && in.OutputCost > 0 {
		pOut := *in.DisplayOutputPrice
		outOwn = int(math.Round(in.OutputCost / pOut))
		if outOwn < 0 {
			outOwn = 0
		}
		outOwnCost = float64(outOwn) * pOut

		gOwn := outOwn - realOut
		if gOwn < 0 {
			gOwn = 0
		}
		gExtraMax := int(math.Floor(alpha*float64(gOwn) + 1e-12))
		if gExtraMax < 0 {
			gExtraMax = 0
		}

		wantExtra := 0
		if residual > 0 && pOut > 0 {
			wantExtra = int(math.Round(residual / pOut))
			if wantExtra < 0 {
				wantExtra = 0
			}
		}
		gExtra := wantExtra
		if gExtra > gExtraMax {
			gExtra = gExtraMax
		}

		displayOut := outOwn + gExtra
		displayOutCost := float64(displayOut) * pOut
		// Residual consumed by extra tokens (use price×tokens for exact cost identity).
		consumed := float64(gExtra) * pOut
		if consumed > residual {
			consumed = residual
		}
		residual -= consumed
		if residual < 0 {
			residual = 0
		}
		out.OutputTokens = displayOut
		out.OutputCost = displayOutCost
	} else {
		out.OutputTokens = outOwn
		out.OutputCost = outOwnCost
	}

	// --- Input: own cost + remaining residual ---
	inputCostForDisplay := in.InputCost + residual
	if in.DisplayInputPrice != nil && *in.DisplayInputPrice > 0 && inputCostForDisplay > 0 &&
		(in.InputTokens > 0 || residual > 0 || in.InputCost > 0) {
		pIn := *in.DisplayInputPrice
		displayIn := int(math.Round(inputCostForDisplay / pIn))
		if displayIn < 0 {
			displayIn = 0
		}
		// Guard: only rewrite when we had input tokens or residual to place.
		if in.InputTokens > 0 || residual > 0 {
			out.InputTokens = displayIn
			out.InputCost = float64(displayIn) * pIn
		}
	}

	return out
}
