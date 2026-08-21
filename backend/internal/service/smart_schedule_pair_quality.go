package service

import (
	"context"
	"sort"
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
)

// PairQualityLive is the smart-schedule pair window Q_{a,u}.
// It is not the account 15-minute quality cell.
type PairQualityLive struct {
	N           int       `json:"n"`
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

// NormalizeSmartScheduleWindowN converges the new N field and the two legacy
// sample floors. Explicit quality_window_samples wins. Both missing → 10.
// Only one legacy field → that value. Both legacy fields → min, then clamp 1–100.
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

func (p *SmartSchedulePlatformPolicy) WindowN() int {
	if p == nil {
		return DefaultSmartScheduleWindowN
	}
	return NormalizeSmartScheduleWindowN(p.QualityWindowSamples, p.QualityMinSuccessSamples, p.QualityMinTTFTSamples)
}

func EchoSmartScheduleWindowN(n int) (*int, *int, *int) {
	copied := n
	success := n
	ttft := n
	return &copied, &success, &ttft
}

func ApplyPairQualityIngest(live *PairQualityLive, n int, success bool, firstTokenMs *int) *PairQualityLive {
	if live == nil {
		live = &PairQualityLive{}
	}
	n = ClampSmartScheduleWindowN(n)
	live.N = n
	if success && firstTokenMs != nil && *firstTokenMs >= 0 {
		live.TTFTMs = appendFIFOInt(live.TTFTMs, *firstTokenMs, n)
	}
	live.OK = appendFIFOBool(live.OK, success, n)
	RecomputePairQuality(live)
	return live
}

func ZeroPairQualityLive(n int) *PairQualityLive {
	live := &PairQualityLive{N: ClampSmartScheduleWindowN(n)}
	RecomputePairQuality(live)
	return live
}

func RecomputePairQuality(live *PairQualityLive) {
	if live == nil {
		return
	}
	if live.N < MinSmartScheduleWindowN {
		live.N = DefaultSmartScheduleWindowN
	}
	live.TTFTMs = trimFIFOInt(live.TTFTMs, live.N)
	live.OK = trimFIFOBool(live.OK, live.N)
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
		return aliasPairQualityView(SmartSchedulePairQualityView{N: DefaultSmartScheduleWindowN})
	}
	return aliasPairQualityView(SmartSchedulePairQualityView{
		P50TTFTMs:   l.P50TTFTMs,
		SuccessRate: l.SuccessRate,
		TTFTCount:   l.TTFTCount,
		OKCount:     l.OKCount,
		N:           l.N,
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
	})
}

func aliasPairQualityView(view SmartSchedulePairQualityView) SmartSchedulePairQualityView {
	view.TTFTP50Ms = view.P50TTFTMs
	view.TTFTSamples = view.TTFTCount
	view.OKSamples = view.OKCount
	return view
}

func aliasPairQualitySnapshot(snap PairQualitySnapshot) PairQualitySnapshot {
	snap.TTFTP50Ms = snap.P50TTFTMs
	snap.TTFTSamples = snap.TTFTCount
	snap.OKSamples = snap.OKCount
	if snap.CapturedAt == "" && snap.Ts > 0 {
		snap.CapturedAt = time.Unix(snap.Ts, 0).UTC().Format(time.RFC3339)
	}
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

func observePairQualitySuccess(lookup SmartScheduleLookup, ctx context.Context, accountID, userID int64, trueMs, firstMs *int) {
	if lookup == nil || accountID <= 0 || userID <= 0 {
		return
	}
	observer, ok := lookup.(PairQualityObserver)
	if !ok {
		return
	}
	observer.ObservePairCompletion(ctx, PairQualityObservation{
		AccountID:    accountID,
		UserID:       userID,
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
