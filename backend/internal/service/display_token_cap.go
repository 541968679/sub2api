package service

import (
	"context"
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

const (
	// DisplayTokenCapJitterFraction is the fixed 8% band: C ∈ [92%, 100%] × configured.
	// Not an admin knob.
	DisplayTokenCapJitterFraction = 0.08
	// MaxDisplayTokenCap clamps configured caps to prevent overflow.
	MaxDisplayTokenCap = 1_000_000_000
	// DefaultDisplayContextTokenMax is the code default (off). Recommended ops value
	// 1_000_000 is documentation / admin placeholder only — do not enable by default.
	DefaultDisplayContextTokenMax int64 = 0
	// DefaultDisplayOutputTokenMax is the code default (off). Recommended ops value
	// 80_000 is documentation / admin placeholder only — do not enable by default.
	DefaultDisplayOutputTokenMax int64 = 0

	displayTokenCapLaneJoint  = "joint"
	displayTokenCapLaneOutput = "output"
)

// DisplayContextTokenCapInput is L1+L2 display tokens/costs plus configured caps.
type DisplayContextTokenCapInput struct {
	InputTokens     int
	CacheReadTokens int
	OutputTokens    int

	InputCost     float64
	CacheReadCost float64
	OutputCost    float64

	DisplayInputPrice     *float64
	DisplayCacheReadPrice *float64
	DisplayOutputPrice    *float64

	// ContextTokenMax is the configured joint input+cache cap. <=0 skips joint cap.
	ContextTokenMax int64
	// OutputTokenMax is the configured independent output cap. <=0 skips output cap.
	OutputTokenMax int64
	// Seed is request_id, or usage log id when request_id is empty.
	Seed string
}

// DisplayContextTokenCapResult is the post-cap display breakdown.
type DisplayContextTokenCapResult struct {
	InputTokens     int
	CacheReadTokens int
	OutputTokens    int

	InputCost     float64
	CacheReadCost float64
	OutputCost    float64

	// Applied is true only when at least one token component was reduced.
	Applied             bool
	ContextTokenMaxUsed int64
	OutputTokenMaxUsed  int64
	JitteredContextCap  int
	JitteredOutputCap   int
}

// ResolveDisplayTokenCap returns a validated integer cap. <=0 / invalid → 0 (off).
func ResolveDisplayTokenCap(raw int64) int64 {
	if raw <= 0 {
		return 0
	}
	if raw > MaxDisplayTokenCap {
		return MaxDisplayTokenCap
	}
	return raw
}

// ParseDisplayTokenCap parses settings KV. Empty / invalid / negative → 0 (off).
func ParseDisplayTokenCap(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		if f, ferr := strconv.ParseFloat(raw, 64); ferr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
			value = int64(math.Floor(f))
		} else {
			return 0
		}
	}
	return ResolveDisplayTokenCap(value)
}

// JitteredDisplayTokenCap returns a stable cap in [92%, 100%] × configured.
// lane is "joint" or "output". configured<=0 → 0.
func JitteredDisplayTokenCap(configured int64, seed, lane string) int {
	configured = ResolveDisplayTokenCap(configured)
	if configured <= 0 {
		return 0
	}
	span := int(math.Floor(float64(configured) * DisplayTokenCapJitterFraction))
	if span < 0 {
		span = 0
	}
	off := displayTokenCapHash64(seed + "|" + lane) % uint64(span+1)
	return int(configured) - int(off)
}

func displayTokenCapHash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func displayTokenCapSeed(seed string, usageLogID int64) string {
	seed = strings.TrimSpace(seed)
	if seed != "" {
		return seed
	}
	if usageLogID > 0 {
		return strconv.FormatInt(usageLogID, 10)
	}
	return ""
}

// ApplyDisplayContextTokenCap jitters the joint sum cap, shrinks input+cache
// proportionally so in'+cache'=C, then applies an independent output cap.
// Discarded cost is not folded into any component. 0 caps return the input unchanged.
func ApplyDisplayContextTokenCap(in DisplayContextTokenCapInput) DisplayContextTokenCapResult {
	out := DisplayContextTokenCapResult{
		InputTokens:     clampNonNegInt(in.InputTokens),
		CacheReadTokens: clampNonNegInt(in.CacheReadTokens),
		OutputTokens:    clampNonNegInt(in.OutputTokens),
		InputCost:       in.InputCost,
		CacheReadCost:   in.CacheReadCost,
		OutputCost:      in.OutputCost,
	}

	contextMax := ResolveDisplayTokenCap(in.ContextTokenMax)
	outputMax := ResolveDisplayTokenCap(in.OutputTokenMax)
	if contextMax <= 0 && outputMax <= 0 {
		return out
	}

	seed := displayTokenCapSeed(in.Seed, 0)
	preIn, preCache, preOut := out.InputTokens, out.CacheReadTokens, out.OutputTokens

	if contextMax > 0 {
		C := JitteredDisplayTokenCap(contextMax, seed, displayTokenCapLaneJoint)
		out.JitteredContextCap = C
		in2, cache2 := shrinkJointDisplayTokens(out.InputTokens, out.CacheReadTokens, C)
		out.InputTokens = in2
		out.CacheReadTokens = cache2
	}
	if outputMax > 0 {
		COut := JitteredDisplayTokenCap(outputMax, seed, displayTokenCapLaneOutput)
		out.JitteredOutputCap = COut
		if out.OutputTokens > COut {
			out.OutputTokens = COut
		}
	}

	if out.InputTokens != preIn || out.CacheReadTokens != preCache || out.OutputTokens != preOut {
		out.Applied = true
		if contextMax > 0 {
			out.ContextTokenMaxUsed = contextMax
		}
		if outputMax > 0 {
			out.OutputTokenMaxUsed = outputMax
		}
	}

	out.InputCost = recomputeCappedDisplayCost(out.InputTokens, preIn, in.InputCost, in.DisplayInputPrice)
	out.CacheReadCost = recomputeCappedDisplayCost(out.CacheReadTokens, preCache, in.CacheReadCost, in.DisplayCacheReadPrice)
	out.OutputCost = recomputeCappedDisplayCost(out.OutputTokens, preOut, in.OutputCost, in.DisplayOutputPrice)
	return out
}

func shrinkJointDisplayTokens(inTokens, cacheTokens, capTokens int) (int, int) {
	if inTokens < 0 {
		inTokens = 0
	}
	if cacheTokens < 0 {
		cacheTokens = 0
	}
	if capTokens < 0 {
		capTokens = 0
	}
	sum := inTokens + cacheTokens
	if sum <= capTokens {
		return inTokens, cacheTokens
	}
	if sum <= 0 || capTokens <= 0 {
		return 0, 0
	}

	in2 := int(math.Round(float64(inTokens) * float64(capTokens) / float64(sum)))
	if in2 < 0 {
		in2 = 0
	}
	cache2 := capTokens - in2
	if cache2 > cacheTokens {
		cache2 = cacheTokens
		in2 = capTokens - cache2
	}
	if in2 > inTokens {
		in2 = inTokens
		cache2 = capTokens - in2
	}
	if in2 < 0 {
		in2 = 0
		cache2 = capTokens
		if cache2 > cacheTokens {
			cache2 = cacheTokens
		}
	}
	if cache2 < 0 {
		cache2 = 0
		in2 = capTokens
		if in2 > inTokens {
			in2 = inTokens
		}
	}
	return in2, cache2
}

func recomputeCappedDisplayCost(tokens, preTokens int, preCost float64, price *float64) float64 {
	if price != nil && *price > 0 {
		return float64(tokens) * *price
	}
	if preTokens > 0 && tokens != preTokens {
		return preCost * float64(tokens) / float64(preTokens)
	}
	return preCost
}

func clampNonNegInt(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func scaleDisplayPricePtr(price *float64, mult float64) *float64 {
	if price == nil || *price <= 0 || mult <= 0 {
		return price
	}
	value := *price * mult
	return &value
}

func applyLongContextDisplayPrices(log *UsageLog, in, out, cache *float64) (*float64, *float64, *float64) {
	if log == nil || !log.LongContextApplied {
		return in, out, cache
	}
	if log.LongContextInputMultiplier > 0 {
		in = scaleDisplayPricePtr(in, log.LongContextInputMultiplier)
		cache = scaleDisplayPricePtr(cache, log.LongContextInputMultiplier)
	}
	if log.LongContextOutputMultiplier > 0 {
		out = scaleDisplayPricePtr(out, log.LongContextOutputMultiplier)
	}
	return in, out, cache
}

func displayCapSeedFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
		return "client:" + strings.TrimSpace(clientRequestID)
	}
	if requestID, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
		return "local:" + strings.TrimSpace(requestID)
	}
	return ""
}

func isTokenBillingMode(mode string) bool {
	mode = strings.TrimSpace(strings.ToLower(mode))
	return mode == "" || mode == string(BillingModeToken)
}

func scaleWritePathDisplayComponent(tokens int, cost float64, scale float64, price *float64) (int, float64) {
	targetCost := cost * scale
	if price != nil && *price > 0 && targetCost > 0 {
		displayTokens := int(math.Round(targetCost/(*price) + 1e-9))
		if displayTokens < 0 {
			displayTokens = 0
		}
		return displayTokens, float64(displayTokens) * *price
	}
	if tokens > 0 {
		tokens = int(math.Round(float64(tokens) * scale))
	}
	return tokens, targetCost
}

func scaleWritePathDisplayCacheRead(tokens int, cost float64, scale float64, billingReal int, m float64, price *float64) (int, float64) {
	if tokens < 0 {
		tokens = 0
	}
	ideal := int(math.Round(float64(tokens) * scale))
	if ideal < 0 {
		ideal = 0
	}
	realCache := billingReal
	if realCache < 0 {
		realCache = 0
	}
	capTokens := int(math.Round(float64(realCache) * effectiveCacheTokenMaxMult(m)))
	displayCache := ideal
	if displayCache > capTokens {
		displayCache = capTokens
	}
	intended := cost * scale
	var displayCost float64
	if price != nil && *price > 0 {
		displayCost = float64(displayCache) * *price
	} else if ideal > 0 {
		displayCost = intended * float64(displayCache) / float64(ideal)
	}
	if displayCost < 0 {
		displayCost = 0
	}
	return displayCache, displayCost
}

type displayRateResolver func(ctx context.Context, userID, groupID int64, fallback float64) float64

// applyDisplayTokenCapToCharge runs L1+L2 then the joint/output cap and replaces
// cost.ActualCost when a cap binds. Billing-real token columns are not rewritten.
func applyDisplayTokenCapToCharge(
	ctx context.Context,
	usageLog *UsageLog,
	cost *CostBreakdown,
	user *User,
	apiKey *APIKey,
	resolver *ModelPricingResolver,
	settingService *SettingService,
	displayRateFn displayRateResolver,
) {
	if usageLog == nil || cost == nil {
		return
	}
	if !isTokenBillingMode(cost.BillingMode) {
		return
	}
	if usageLog.BillingMode != nil && !isTokenBillingMode(*usageLog.BillingMode) {
		return
	}
	if usageLog.ImageCount > 0 || usageLog.VideoCount > 0 {
		return
	}

	contextMax, outputMax := int64(0), int64(0)
	if settingService != nil {
		contextMax, outputMax = settingService.GetDisplayTokenCapSettings(ctx)
	}
	contextMax = ResolveDisplayTokenCap(contextMax)
	outputMax = ResolveDisplayTokenCap(outputMax)
	if contextMax <= 0 && outputMax <= 0 {
		return
	}

	userID := usageLog.UserID
	if userID == 0 && user != nil {
		userID = user.ID
	}
	var groupID *int64
	if apiKey != nil && apiKey.GroupID != nil && *apiKey.GroupID > 0 {
		groupID = apiKey.GroupID
	}

	model := strings.TrimSpace(usageLog.RequestedModel)
	if model == "" {
		model = strings.TrimSpace(usageLog.Model)
	}
	pricing := resolveDisplayTokenPricing(ctx, model, userID, groupID, resolver)
	inPrice, outPrice, cachePrice := applyLongContextDisplayPrices(
		usageLog,
		pricing.DisplayInputPrice,
		pricing.DisplayOutputPrice,
		pricing.DisplayCacheReadPrice,
	)
	controls := resolveDisplayTokenAllocControls(ctx, userID, resolver)

	workingIn := usageLog.InputTokens
	workingOut := usageLog.OutputTokens
	workingCache := usageLog.CacheReadTokens
	workingInCost := usageLog.InputCost
	workingOutCost := usageLog.OutputCost
	workingCacheCost := usageLog.CacheReadCost
	billingRealCache := usageLog.CacheReadTokens

	if inPrice != nil || outPrice != nil || cachePrice != nil {
		alpha := controls.OutputResidualGrowthRatio
		alloc := AllocateDisplayTokens(DisplayTokenAllocInput{
			InputTokens:               usageLog.InputTokens,
			OutputTokens:              usageLog.OutputTokens,
			CacheReadTokens:           usageLog.CacheReadTokens,
			InputCost:                 usageLog.InputCost,
			OutputCost:                usageLog.OutputCost,
			CacheReadCost:             usageLog.CacheReadCost,
			DisplayInputPrice:         inPrice,
			DisplayOutputPrice:        outPrice,
			DisplayCacheReadPrice:     cachePrice,
			CacheTokenMaxMult:         controls.CacheTokenMaxMult,
			OutputResidualGrowthRatio: &alpha,
		})
		workingIn = alloc.InputTokens
		workingOut = alloc.OutputTokens
		workingCache = alloc.CacheReadTokens
		workingInCost = alloc.InputCost
		workingOutCost = alloc.OutputCost
		workingCacheCost = alloc.CacheReadCost
	}

	billingRate := usageLog.RateMultiplier
	displayRate := billingRate
	if displayRateFn != nil && userID > 0 && groupID != nil {
		displayRate = displayRateFn(ctx, userID, *groupID, billingRate)
	}
	if displayRate <= 0 {
		displayRate = billingRate
	}
	if displayRate > 0 && billingRate > 0 && displayRate != billingRate {
		scale := billingRate / displayRate
		workingOut, workingOutCost = scaleWritePathDisplayComponent(workingOut, workingOutCost, scale, outPrice)
		workingCache, workingCacheCost = scaleWritePathDisplayCacheRead(
			workingCache, workingCacheCost, scale, billingRealCache, controls.CacheTokenMaxMult, cachePrice,
		)
		workingIn, workingInCost = scaleWritePathDisplayComponent(workingIn, workingInCost, scale, inPrice)
	}

	seed := displayTokenCapSeed(usageLog.RequestID, usageLog.ID)
	capped := ApplyDisplayContextTokenCap(DisplayContextTokenCapInput{
		InputTokens:           workingIn,
		CacheReadTokens:       workingCache,
		OutputTokens:          workingOut,
		InputCost:             workingInCost,
		CacheReadCost:         workingCacheCost,
		OutputCost:            workingOutCost,
		DisplayInputPrice:     inPrice,
		DisplayCacheReadPrice: cachePrice,
		DisplayOutputPrice:    outPrice,
		ContextTokenMax:       contextMax,
		OutputTokenMax:        outputMax,
		Seed:                  seed,
	})
	if !capped.Applied {
		return
	}

	before := workingInCost + workingCacheCost + workingOutCost
	after := capped.InputCost + capped.CacheReadCost + capped.OutputCost
	delta := before - after
	if delta < 0 {
		delta = 0
	}
	rate := displayRate
	if rate <= 0 {
		rate = 1
	}
	newActual := cost.ActualCost - delta*rate
	if newActual < 0 {
		newActual = 0
	}
	if newActual > cost.ActualCost {
		newActual = cost.ActualCost
	}
	cost.ActualCost = newActual
	usageLog.ActualCost = newActual
	usageLog.DisplayTokenCapApplied = true
	usageLog.DisplayContextTokenMaxUsed = capped.ContextTokenMaxUsed
	usageLog.DisplayOutputTokenMaxUsed = capped.OutputTokenMaxUsed
}
