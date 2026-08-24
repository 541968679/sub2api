package service

import (
	"context"
	"sort"
	"strings"
	"time"
)

const (
	DefaultSmartScheduleWindowN = 10
	MinSmartScheduleWindowN     = 1
	MaxSmartScheduleWindowN     = 100

	PairQualityEventCooldownStart = "cooldown_start"
	PairQualityEventCooldownEnd   = "cooldown_end"
	PairQualityEventResumed       = "resumed"
	PairQualityEventSelectable    = "selectable"
	PairQualityEventExpiryZero    = "expiry_zero"
	PairQualityEventProbeEnter    = "probe_enter"
	PairQualityEventProbeGraduate = "probe_graduate"
	PairQualityEventPinEnter      = "pin_enter"
)

// PairQualityLive is the smart-schedule pair window Q_{a,u}.
// It is not the account 15-minute quality cell.
type PairQualityLive struct {
	N           int       `json:"n"`
	NTTFT       int       `json:"n_ttft"`
	NOK         int       `json:"n_ok"`
	TTFTMs      []int     `json:"ttft_ms,omitempty"`
	OK          []bool    `json:"ok,omitempty"`
	P50TTFTMs   *int      `json:"p50_ttft_ms,omitempty"`
	SuccessRate *float64  `json:"success_rate,omitempty"`
	TTFTCount   int       `json:"ttft_count"`
	OKCount     int       `json:"ok_count"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// SmartSchedulePairQualityView is the pool-row / detail live snapshot.
// Canonical names follow the locked contract; aliases keep the already-wired frontend working.
type SmartSchedulePairQualityView struct {
	P50TTFTMs   *int     `json:"p50_ttft_ms"`
	TTFTP50Ms   *int     `json:"ttft_p50_ms"`
	SuccessRate *float64 `json:"success_rate"`
	TTFTCount   int      `json:"ttft_count"`
	TTFTSamples int      `json:"ttft_samples"`
	OKCount     int      `json:"ok_count"`
	OKSamples   int      `json:"ok_samples"`
	N           int      `json:"n"`
	NTTFT       int      `json:"n_ttft"`
	NSuccess    int      `json:"n_success"`
	NOK         int      `json:"n_ok"`
}

// PairQualitySnapshot is one trend point after a recompute.
type PairQualitySnapshot struct {
	Ts          int64    `json:"ts"`
	CapturedAt  string   `json:"captured_at,omitempty"`
	P50TTFTMs   *int     `json:"p50_ttft_ms"`
	TTFTP50Ms   *int     `json:"ttft_p50_ms"`
	SuccessRate *float64 `json:"success_rate"`
	TTFTCount   int      `json:"ttft_count"`
	TTFTSamples int      `json:"ttft_samples"`
	OKCount     int      `json:"ok_count"`
	OKSamples   int      `json:"ok_samples"`
	N           int      `json:"n"`
	NTTFT       int      `json:"n_ttft"`
	NSuccess    int      `json:"n_success"`
	NOK         int      `json:"n_ok"`
}

// PairQualityEvent is a cooldown / resume / zero record for the detail UI.
type PairQualityEvent struct {
	Ts     int64  `json:"ts"`
	At     string `json:"at,omitempty"`
	Type   string `json:"type"`
	Until  *int64 `json:"until,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// SmartSchedulePairQualityDetail is GET pair-quality detail.
type SmartSchedulePairQualityDetail struct {
	AccountID int64                        `json:"account_id"`
	UserID    int64                        `json:"user_id"`
	N         int                          `json:"n"`
	NTTFT     int                          `json:"n_ttft"`
	NSuccess  int                          `json:"n_success"`
	Live      SmartSchedulePairQualityView `json:"live"`
	Current   SmartSchedulePairQualityView `json:"current"`
	Snapshots []PairQualitySnapshot        `json:"snapshots"`
	Events    []PairQualityEvent           `json:"events"`
}

// SmartSchedulePairQualityBatch is POST /admin/users/:id/smart-schedule/pair-quality.
type SmartSchedulePairQualityBatch struct {
	Pairs map[string]SmartSchedulePairQualityView `json:"pairs"`
}

// PairQualityObservation is one completed request for pair-window ingest.
type PairQualityObservation struct {
	AccountID    int64
	UserID       int64
	Platform     string
	Success      bool
	FirstTokenMs *int
}

// PairQualityObserver is the completion-path hook (usage success / counted error).
type PairQualityObserver interface {
	ObservePairCompletion(ctx context.Context, obs PairQualityObservation)
}

// ClampSmartScheduleWindowN clamps to 1–100. Values < 1 become the default 10.
func ClampSmartScheduleWindowN(n int) int {
	if n < MinSmartScheduleWindowN {
		return DefaultSmartScheduleWindowN
	}
	if n > MaxSmartScheduleWindowN {
		return MaxSmartScheduleWindowN
	}
	return n
}

// NormalizeSmartScheduleWindowN is the pre-split collapse (explicit window wins;
// both legacy fields → min). Write/read paths must not call this. Kept for
// leftover callers and historical tests.
func NormalizeSmartScheduleWindowN(window, minSuccess, minTTFT *int) int {
	if window != nil {
		return ClampSmartScheduleWindowN(*window)
	}
	if minSuccess == nil && minTTFT == nil {
		return DefaultSmartScheduleWindowN
	}
	if minSuccess != nil && minTTFT == nil {
		return ClampSmartScheduleWindowN(*minSuccess)
	}
	if minTTFT != nil && minSuccess == nil {
		return ClampSmartScheduleWindowN(*minTTFT)
	}
	a := ClampSmartScheduleWindowN(*minSuccess)
	b := ClampSmartScheduleWindowN(*minTTFT)
	if a < b {
		return a
	}
	return b
}

func resolveSmartScheduleMetricN(metric, fallback *int) int {
	if metric != nil {
		return ClampSmartScheduleWindowN(*metric)
	}
	if fallback != nil {
		return ClampSmartScheduleWindowN(*fallback)
	}
	return DefaultSmartScheduleWindowN
}

func maxSmartScheduleWindowN(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func echoCompatSmartScheduleWindowN(ttft, success int) (*int, *int) {
	n := maxSmartScheduleWindowN(ClampSmartScheduleWindowN(ttft), ClampSmartScheduleWindowN(success))
	copied := n
	return &copied, &copied
}

func EchoSmartScheduleWindowN(n int) (*int, *int, *int) {
	copied := n
	success := n
	ttft := n
	return &copied, &success, &ttft
}

func (p *SmartSchedulePlatformPolicy) TTFTWindowN() int {
	if p == nil {
		return DefaultSmartScheduleWindowN
	}
	return resolveSmartScheduleMetricN(p.QualityMinTTFTSamples, p.QualityWindowSamples)
}

func (p *SmartSchedulePlatformPolicy) SuccessWindowN() int {
	if p == nil {
		return DefaultSmartScheduleWindowN
	}
	return resolveSmartScheduleMetricN(p.QualityMinSuccessSamples, p.QualityWindowSamples)
}

// WindowN is the compat max of the two metric windows. Do not use as a gate floor.
func (p *SmartSchedulePlatformPolicy) WindowN() int {
	if p == nil {
		return DefaultSmartScheduleWindowN
	}
	return maxSmartScheduleWindowN(p.TTFTWindowN(), p.SuccessWindowN())
}

func ApplyPairQualityIngest(live *PairQualityLive, n int, success bool, firstTokenMs *int) *PairQualityLive {
	return ApplyPairQualityIngestWindows(live, n, n, success, firstTokenMs)
}

func ApplyPairQualityIngestWindows(live *PairQualityLive, nTTFT, nOK int, success bool, firstTokenMs *int) *PairQualityLive {
	if live == nil {
		live = &PairQualityLive{}
	}
	nTTFT = ClampSmartScheduleWindowN(nTTFT)
	nOK = ClampSmartScheduleWindowN(nOK)
	live.NTTFT = nTTFT
	live.NOK = nOK
	live.N = maxSmartScheduleWindowN(nTTFT, nOK)
	if success && firstTokenMs != nil && *firstTokenMs >= 0 {
		live.TTFTMs = appendFIFOInt(live.TTFTMs, *firstTokenMs, nTTFT)
	}
	live.OK = appendFIFOBool(live.OK, success, nOK)
	RecomputePairQuality(live)
	return live
}

func ZeroPairQualityLive(n int) *PairQualityLive {
	return ZeroPairQualityLiveWindows(n, n)
}

func ZeroPairQualityLiveWindows(nTTFT, nOK int) *PairQualityLive {
	nTTFT = ClampSmartScheduleWindowN(nTTFT)
	nOK = ClampSmartScheduleWindowN(nOK)
	live := &PairQualityLive{
		N:     maxSmartScheduleWindowN(nTTFT, nOK),
		NTTFT: nTTFT,
		NOK:   nOK,
	}
	RecomputePairQuality(live)
	return live
}

func RecomputePairQuality(live *PairQualityLive) {
	if live == nil {
		return
	}
	if live.NTTFT < MinSmartScheduleWindowN {
		if live.N >= MinSmartScheduleWindowN {
			live.NTTFT = ClampSmartScheduleWindowN(live.N)
		} else {
			live.NTTFT = DefaultSmartScheduleWindowN
		}
	} else {
		live.NTTFT = ClampSmartScheduleWindowN(live.NTTFT)
	}
	if live.NOK < MinSmartScheduleWindowN {
		if live.N >= MinSmartScheduleWindowN {
			live.NOK = ClampSmartScheduleWindowN(live.N)
		} else {
			live.NOK = DefaultSmartScheduleWindowN
		}
	} else {
		live.NOK = ClampSmartScheduleWindowN(live.NOK)
	}
	live.N = maxSmartScheduleWindowN(live.NTTFT, live.NOK)
	live.TTFTMs = trimFIFOInt(live.TTFTMs, live.NTTFT)
	live.OK = trimFIFOBool(live.OK, live.NOK)
	live.TTFTCount = len(live.TTFTMs)
	live.OKCount = len(live.OK)
	live.P50TTFTMs = pairQualityP50(live.TTFTMs)
	live.SuccessRate = pairQualitySuccessRate(live.OK)
	live.UpdatedAt = time.Now().UTC()
}

func pairQualityP50(samples []int) *int {
	if len(samples) == 0 {
		return nil
	}
	sorted := append([]int(nil), samples...)
	sort.Ints(sorted)
	n := len(sorted)
	if n%2 == 1 {
		v := sorted[n/2]
		return &v
	}
	v := (sorted[n/2-1] + sorted[n/2]) / 2
	return &v
}

func pairQualitySuccessRate(ok []bool) *float64 {
	if len(ok) == 0 {
		return nil
	}
	success := 0
	for _, v := range ok {
		if v {
			success++
		}
	}
	rate := float64(success) / float64(len(ok))
	return &rate
}

func appendFIFOInt(in []int, v, n int) []int {
	out := append(in, v)
	return trimFIFOInt(out, n)
}

func appendFIFOBool(in []bool, v bool, n int) []bool {
	out := append(in, v)
	return trimFIFOBool(out, n)
}

func trimFIFOInt(in []int, n int) []int {
	if n < 1 || len(in) <= n {
		return in
	}
	return append([]int(nil), in[len(in)-n:]...)
}

func trimFIFOBool(in []bool, n int) []bool {
	if n < 1 || len(in) <= n {
		return in
	}
	return append([]bool(nil), in[len(in)-n:]...)
}

func (l *PairQualityLive) View() SmartSchedulePairQualityView {
	if l == nil {
		return aliasPairQualityView(SmartSchedulePairQualityView{
			N:        DefaultSmartScheduleWindowN,
			NTTFT:    DefaultSmartScheduleWindowN,
			NSuccess: DefaultSmartScheduleWindowN,
			NOK:      DefaultSmartScheduleWindowN,
		})
	}
	return aliasPairQualityView(SmartSchedulePairQualityView{
		P50TTFTMs:   l.P50TTFTMs,
		SuccessRate: l.SuccessRate,
		TTFTCount:   l.TTFTCount,
		OKCount:     l.OKCount,
		N:           l.N,
		NTTFT:       l.NTTFT,
		NSuccess:    l.NOK,
		NOK:         l.NOK,
	})
}

func (l *PairQualityLive) Snapshot() PairQualitySnapshot {
	view := l.View()
	ts := time.Now().UTC()
	if l != nil && !l.UpdatedAt.IsZero() {
		ts = l.UpdatedAt
	}
	return aliasPairQualitySnapshot(PairQualitySnapshot{
		Ts:          ts.Unix(),
		CapturedAt:  ts.UTC().Format(time.RFC3339),
		P50TTFTMs:   view.P50TTFTMs,
		SuccessRate: view.SuccessRate,
		TTFTCount:   view.TTFTCount,
		OKCount:     view.OKCount,
		N:           view.N,
		NTTFT:       view.NTTFT,
		NSuccess:    view.NSuccess,
		NOK:         view.NOK,
	})
}

func aliasPairQualityView(view SmartSchedulePairQualityView) SmartSchedulePairQualityView {
	view.TTFTP50Ms = view.P50TTFTMs
	view.TTFTSamples = view.TTFTCount
	view.OKSamples = view.OKCount
	if view.NTTFT < MinSmartScheduleWindowN {
		if view.N >= MinSmartScheduleWindowN {
			view.NTTFT = ClampSmartScheduleWindowN(view.N)
		} else {
			view.NTTFT = DefaultSmartScheduleWindowN
		}
	}
	if view.NSuccess < MinSmartScheduleWindowN && view.NOK >= MinSmartScheduleWindowN {
		view.NSuccess = view.NOK
	}
	if view.NOK < MinSmartScheduleWindowN && view.NSuccess >= MinSmartScheduleWindowN {
		view.NOK = view.NSuccess
	}
	if view.NSuccess < MinSmartScheduleWindowN {
		if view.N >= MinSmartScheduleWindowN {
			view.NSuccess = ClampSmartScheduleWindowN(view.N)
		} else {
			view.NSuccess = DefaultSmartScheduleWindowN
		}
	}
	if view.NOK < MinSmartScheduleWindowN {
		view.NOK = view.NSuccess
	}
	view.N = maxSmartScheduleWindowN(view.NTTFT, view.NSuccess)
	return view
}

func aliasPairQualitySnapshot(snap PairQualitySnapshot) PairQualitySnapshot {
	snap.TTFTP50Ms = snap.P50TTFTMs
	snap.TTFTSamples = snap.TTFTCount
	snap.OKSamples = snap.OKCount
	if snap.CapturedAt == "" && snap.Ts > 0 {
		snap.CapturedAt = time.Unix(snap.Ts, 0).UTC().Format(time.RFC3339)
	}
	aliased := aliasPairQualityView(SmartSchedulePairQualityView{
		N:        snap.N,
		NTTFT:    snap.NTTFT,
		NSuccess: snap.NSuccess,
		NOK:      snap.NOK,
	})
	snap.N = aliased.N
	snap.NTTFT = aliased.NTTFT
	snap.NSuccess = aliased.NSuccess
	snap.NOK = aliased.NOK
	return snap
}

func aliasPairQualityEvent(event PairQualityEvent) PairQualityEvent {
	if event.At == "" && event.Ts > 0 {
		event.At = time.Unix(event.Ts, 0).UTC().Format(time.RFC3339)
	}
	return event
}

// ToAccountQualityStats projects pair windows into the existing hard-close evaluator shape.
// Success/error counts come from W_ok; TTFT samples come from W_ttft only.
func (l *PairQualityLive) ToAccountQualityStats() *AccountQualityStats {
	if l == nil {
		return nil
	}
	stats := &AccountQualityStats{
		TTFTSamples: int64(l.TTFTCount),
		P50TTFTMs:   l.P50TTFTMs,
	}
	success := 0
	for _, ok := range l.OK {
		if ok {
			success++
		}
	}
	stats.SuccessCount = int64(success)
	stats.ErrorCount = int64(l.OKCount - success)
	if l.SuccessRate != nil {
		rate := *l.SuccessRate
		stats.SuccessRate = &rate
	}
	NormalizeAccountQualityRates(stats)
	return stats
}

func pairQualityTTFTMs(trueMs, firstMs *int) *int {
	if trueMs != nil && *trueMs >= 0 {
		return trueMs
	}
	if firstMs != nil && *firstMs >= 0 {
		return firstMs
	}
	return nil
}

func observePairQualitySuccess(lookup SmartScheduleLookup, ctx context.Context, account *Account, userID int64, trueMs, firstMs *int) {
	if lookup == nil || account == nil || account.ID <= 0 || userID <= 0 {
		return
	}
	observer, ok := lookup.(PairQualityObserver)
	if !ok {
		return
	}
	observer.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID:    account.ID,
		UserID:       userID,
		Platform:     smartScheduleLookupPlatformForUser(ctx, account, lookup, userID),
		Success:      true,
		FirstTokenMs: pairQualityTTFTMs(trueMs, firstMs),
	})
}

func pairQualityBlocks(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) bool {
	if policy == nil || !policy.HasQualityMetrics() {
		return false
	}
	blocked, _ := EvaluateAccountQualityHardClose(live.ToAccountQualityStats(), policy.QualityGate(), false)
	return blocked
}

// ProbeInFlightCap is the probing in-flight pair slot cap.
// desired is follow_n window N or a custom 1–100 value — not account-quality N.
// Member cap >= 1 → min(desired, cap). No cap → desired (never unlimited / never 999).
func ProbeInFlightCap(desired, memberCap int) int {
	n := ClampSmartScheduleWindowN(desired)
	if memberCap >= 1 && memberCap < n {
		return memberCap
	}
	return n
}

func pairQualityProbeGraduates(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) bool {
	if live == nil {
		return false
	}
	nOK := DefaultSmartScheduleWindowN
	nTTFT := DefaultSmartScheduleWindowN
	if policy != nil {
		nOK = policy.SuccessWindowN()
		nTTFT = policy.TTFTWindowN()
	}
	if live.OKCount < nOK {
		return false
	}
	if policy == nil || !policy.HasQualityMetrics() {
		return true
	}
	gate := policy.QualityGate()
	if gate.MinSuccessRate != nil {
		if live.SuccessRate == nil || *live.SuccessRate < *gate.MinSuccessRate {
			return false
		}
	}
	if live.TTFTCount < nTTFT {
		return true
	}
	if gate.MaxP50TTFTMs != nil {
		if live.P50TTFTMs == nil || *live.P50TTFTMs > *gate.MaxP50TTFTMs {
			return false
		}
	}
	return true
}

// pairQualityProbeAndMixed is the probing-only anti-deadlock override:
// and + both windows full + one pass one fail → cool. Selectable does not use this.
func pairQualityProbeAndMixed(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) bool {
	if live == nil || policy == nil || !policy.HasQualityMetrics() {
		return false
	}
	gate := policy.QualityGate()
	if strings.ToLower(strings.TrimSpace(gate.Condition)) != QualityHardCloseConditionAnd {
		return false
	}
	nOK := policy.SuccessWindowN()
	nTTFT := policy.TTFTWindowN()
	if live.OKCount < nOK || live.TTFTCount < nTTFT {
		return false
	}
	if gate.MaxP50TTFTMs == nil || gate.MinSuccessRate == nil {
		return false
	}
	if pairQualityBlocks(live, policy) || pairQualityProbeGraduates(live, policy) {
		return false
	}
	return true
}

// pairQualityResumeBlocksEvaluate is 豁免期 fail-open only (no probe mark).
// Leftover pair resume during probing is not 豁免期 — do not skip graduate / and-mixed.
func pairQualityResumeBlocksEvaluate(ctx context.Context, lookup SmartScheduleLookup, probing bool, accountID, userID int64, platform string, now time.Time) bool {
	if probing || lookup == nil {
		return false
	}
	return lookup.PairResumeActive(ctx, accountID, userID, platform, now)
}

func clearLeftoverPairResumeIfProbing(ctx context.Context, lookup SmartScheduleLookup, probing bool, accountID, userID int64, platform string, now time.Time) {
	if !probing || lookup == nil || !lookup.PairResumeActive(ctx, accountID, userID, platform, now) {
		return
	}
	lookup.ClearPairResume(ctx, accountID, userID, platform)
}

// evaluateSmartSchedulePairQuality applies cooldown / probe graduate on the hot path
// and after ingest. 豁免期 (no probe mark) must be checked by the caller (no evaluate).
func evaluateSmartSchedulePairQuality(ctx context.Context, lookup SmartScheduleLookup, accountID, userID int64, platform string, policy *SmartSchedulePlatformPolicy, live *PairQualityLive, now time.Time) bool {
	if lookup != nil && lookup.IsPinned(ctx, accountID, userID, platform) {
		return true
	}
	probing := lookup != nil && lookup.IsProbing(ctx, accountID, userID, platform)
	minutes := DefaultSmartScheduleCooldownMinutes
	if policy != nil && policy.CooldownMinutes >= MinSmartScheduleCooldownMinutes {
		minutes = policy.CooldownMinutes
	}
	if probing {
		if pairQualityBlocks(live, policy) || pairQualityProbeAndMixed(live, policy) {
			lookup.ClearProbing(ctx, accountID, userID, platform)
			lookup.StartCooldown(ctx, accountID, userID, platform, minutes, now)
			return false
		}
		if pairQualityProbeGraduates(live, policy) {
			lookup.GraduateProbing(ctx, accountID, userID, platform)
		}
		return true
	}
	if pairQualityBlocks(live, policy) {
		if lookup != nil {
			lookup.StartCooldown(ctx, accountID, userID, platform, minutes, now)
		}
		return false
	}
	return true
}
