package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	DefaultSmartScheduleWindowN = 10
	MinSmartScheduleWindowN     = 1
	MaxSmartScheduleWindowN     = 100

	PairQualityEventCooldownStart = "cooldown_start"
	PairQualityEventCooldownEnd     = "cooldown_end"
	PairQualityEventSoftCooldownEnd = "soft_cooldown_end"
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
	N             int       `json:"n"`
	NTTFT         int       `json:"n_ttft"`
	NOK           int       `json:"n_ok"`
	NDuration     int       `json:"n_duration,omitempty"`
	TTFTMs        []int     `json:"ttft_ms,omitempty"`
	DurationMs    []int     `json:"duration_ms,omitempty"`
	OK            []bool    `json:"ok,omitempty"`
	P50TTFTMs     *int      `json:"p50_ttft_ms,omitempty"`
	P50DurationMs *int      `json:"p50_duration_ms,omitempty"`
	SuccessRate   *float64  `json:"success_rate,omitempty"`
	TTFTCount     int       `json:"ttft_count"`
	DurationCount int       `json:"duration_count"`
	OKCount       int       `json:"ok_count"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
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
	DurationMs   *int
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

func (p *SmartSchedulePlatformPolicy) SchedCompositeEnabled() bool {
	if p == nil {
		return false
	}
	if p.QualitySchedWindowN != nil && *p.QualitySchedWindowN > 0 {
		return true
	}
	if p.QualitySchedMaxSlowInWindow != nil && *p.QualitySchedMaxSlowInWindow > 0 {
		return true
	}
	if p.QualitySchedMaxConsecutiveSlow != nil && *p.QualitySchedMaxConsecutiveSlow > 0 {
		return true
	}
	return false
}

func (p *SmartSchedulePlatformPolicy) SchedWindowN() int {
	if p == nil || !p.SchedCompositeEnabled() {
		return 0
	}
	if p.QualitySchedWindowN != nil && *p.QualitySchedWindowN > 0 {
		return ClampSmartScheduleWindowN(*p.QualitySchedWindowN)
	}
	return DefaultSmartScheduleSchedN
}

// TTFTStorageN is the FIFO capacity for TTFT/duration pair windows.
// Sched N is included only when the selectable composite gate is on.
func (p *SmartSchedulePlatformPolicy) TTFTStorageN() int {
	if p == nil {
		return DefaultSmartScheduleWindowN
	}
	probeN := p.TTFTWindowN()
	if !p.SchedCompositeEnabled() {
		return probeN
	}
	return maxSmartScheduleWindowN(probeN, p.SchedWindowN())
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
	return ApplyPairQualityIngestWindows(live, n, n, success, firstTokenMs, nil)
}

func ApplyPairQualityIngestWindows(live *PairQualityLive, nTTFT, nOK int, success bool, firstTokenMs, durationMs *int) *PairQualityLive {
	if live == nil {
		live = &PairQualityLive{}
	}
	nTTFT = ClampSmartScheduleWindowN(nTTFT)
	nOK = ClampSmartScheduleWindowN(nOK)
	live.NTTFT = nTTFT
	live.NOK = nOK
	live.NDuration = nTTFT
	live.N = maxSmartScheduleWindowN(nTTFT, nOK)
	if success && firstTokenMs != nil && *firstTokenMs >= 0 {
		live.TTFTMs = appendFIFOInt(live.TTFTMs, *firstTokenMs, nTTFT)
	} else if success && durationMs != nil && *durationMs >= 0 {
		live.DurationMs = appendFIFOInt(live.DurationMs, *durationMs, nTTFT)
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
	if live.NDuration < MinSmartScheduleWindowN {
		live.NDuration = live.NTTFT
	}
	live.TTFTMs = trimFIFOInt(live.TTFTMs, live.NTTFT)
	live.DurationMs = trimFIFOInt(live.DurationMs, live.NDuration)
	live.OK = trimFIFOBool(live.OK, live.NOK)
	live.TTFTCount = len(live.TTFTMs)
	live.DurationCount = len(live.DurationMs)
	live.OKCount = len(live.OK)
	live.P50TTFTMs = pairQualityP50(live.TTFTMs)
	live.P50DurationMs = pairQualityP50(live.DurationMs)
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

func loadSmartScheduleQA(ctx context.Context, cache AccountQualityLiveCache, accountID int64, policy *SmartSchedulePlatformPolicy) *AccountQualityLastN {
	if cache == nil || accountID <= 0 || policy == nil || policy.QualityMaxP50TTFTMs == nil {
		return nil
	}
	reader, ok := cache.(AccountQualityLastNCache)
	if !ok {
		return nil
	}
	live := reader.GetLastN(ctx, accountID)
	if live == nil {
		return nil
	}
	return ProjectAccountQualityLastN(live, policy.TTFTWindowN())
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

func observePairQualitySuccess(lookup SmartScheduleLookup, ctx context.Context, account *Account, userID int64, trueMs, firstMs, durationMs *int) {
	if lookup == nil || account == nil || account.ID <= 0 || userID <= 0 {
		return
	}
	observer, ok := lookup.(PairQualityObserver)
	if !ok {
		return
	}
	ttft := pairQualityTTFTMs(trueMs, firstMs)
	obs := PairQualityObservation{
		AccountID:    account.ID,
		UserID:       userID,
		Platform:     smartScheduleLookupPlatformForUser(ctx, account, lookup, userID),
		Success:      true,
		FirstTokenMs: ttft,
	}
	if ttft == nil && durationMs != nil && *durationMs >= 0 {
		obs.DurationMs = durationMs
	}
	observer.ObservePairCompletion(ctx, obs)
}

func pairQualitySuccessBlocked(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) (bool, []SmartScheduleCooldownReason) {
	if live == nil || policy == nil || policy.QualityMinSuccessRate == nil {
		return false, nil
	}
	nOK := policy.SuccessWindowN()
	if live.OKCount < nOK {
		return false, nil
	}
	rate := live.SuccessRate
	if rate == nil {
		return false, nil
	}
	if *rate >= *policy.QualityMinSuccessRate {
		return false, nil
	}
	return true, []SmartScheduleCooldownReason{{
		Code:   "success",
		Detail: fmt.Sprintf("成功率 %.2f<%.2f", *rate, *policy.QualityMinSuccessRate),
	}}
}

func pairQualityLegacyP50Blocked(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) (bool, []SmartScheduleCooldownReason) {
	if live == nil || policy == nil {
		return false, nil
	}
	n := policy.TTFTWindowN()
	var reasons []SmartScheduleCooldownReason
	if policy.QualityMaxP50TTFTMs != nil && live.TTFTCount >= n {
		if p50 := pairQualityP50(recentLatencySamples(live.TTFTMs, n)); p50 != nil && *p50 > *policy.QualityMaxP50TTFTMs {
			reasons = append(reasons, SmartScheduleCooldownReason{
				Code:   "ttft_p50",
				Detail: fmt.Sprintf("p50 %d>%dms", *p50, *policy.QualityMaxP50TTFTMs),
			})
		}
	}
	if policy.QualityMaxP50DurationMs != nil && live.DurationCount >= n {
		if p50 := pairQualityP50(recentLatencySamples(live.DurationMs, n)); p50 != nil && *p50 > *policy.QualityMaxP50DurationMs {
			reasons = append(reasons, SmartScheduleCooldownReason{
				Code:   "dur_p50",
				Detail: fmt.Sprintf("p50 %d>%dms", *p50, *policy.QualityMaxP50DurationMs),
			})
		}
	}
	return len(reasons) > 0, reasons
}

func prefixLatencyReasonCodes(rs []SmartScheduleCooldownReason, prefix string) []SmartScheduleCooldownReason {
	for i := range rs {
		rs[i].Code = prefix + rs[i].Code
	}
	return rs
}

func pairQualityProbeLatencyBlocked(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) (bool, []SmartScheduleCooldownReason) {
	if live == nil || policy == nil {
		return false, nil
	}
	if !policyProbeLatencyV2(policy) {
		return pairQualityLegacyP50Blocked(live, policy)
	}
	n := policy.TTFTWindowN()
	k, c := resolveSmartScheduleLatencyKC(policy)
	var reasons []SmartScheduleCooldownReason
	blocked := false
	if policy.QualityMaxP50TTFTMs != nil {
		if b, rs := pairLatencyGate(live.TTFTMs, policy.QualityMaxP50TTFTMs, k, c, n, false); b {
			blocked = true
			reasons = append(reasons, prefixLatencyReasonCodes(rs, "ttft_")...)
		}
	}
	if policy.QualityMaxP50DurationMs != nil {
		if b, rs := pairLatencyGate(live.DurationMs, policy.QualityMaxP50DurationMs, k, c, n, false); b {
			blocked = true
			reasons = append(reasons, prefixLatencyReasonCodes(rs, "dur_")...)
		}
	}
	return blocked, reasons
}

func pairQualitySelectableLatencyBlocked(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) (bool, []SmartScheduleCooldownReason) {
	if live == nil || policy == nil {
		return false, nil
	}
	if !policy.SchedCompositeEnabled() {
		return pairQualityLegacyP50Blocked(live, policy)
	}
	n, k, c := resolveSmartScheduleSchedKC(policy)
	if n < 1 {
		return false, nil
	}
	var reasons []SmartScheduleCooldownReason
	blocked := false
	if policy.QualityMaxP50TTFTMs != nil {
		if b, rs := pairSelectableLatencyGate(recentLatencySamples(live.TTFTMs, n), policy.QualityMaxP50TTFTMs, k, c, n); b {
			blocked = true
			reasons = append(reasons, prefixLatencyReasonCodes(rs, "ttft_")...)
		}
	}
	if policy.QualityMaxP50DurationMs != nil {
		if b, rs := pairSelectableLatencyGate(recentLatencySamples(live.DurationMs, n), policy.QualityMaxP50DurationMs, k, c, n); b {
			blocked = true
			reasons = append(reasons, prefixLatencyReasonCodes(rs, "dur_")...)
		}
	}
	return blocked, reasons
}

func combinePairQualityBlocks(successBlocked, latencyBlocked bool, condition string) bool {
	cond := strings.ToLower(strings.TrimSpace(condition))
	if cond == QualityHardCloseConditionAnd {
		return successBlocked && latencyBlocked
	}
	return successBlocked || latencyBlocked
}

func pairQualityBlocksWithReasons(live *PairQualityLive, policy *SmartSchedulePlatformPolicy, selectable bool) (bool, []SmartScheduleCooldownReason) {
	if policy == nil || !policy.HasQualityMetrics() {
		return false, nil
	}
	successBlocked, successReasons := pairQualitySuccessBlocked(live, policy)
	var latencyBlocked bool
	var latencyReasons []SmartScheduleCooldownReason
	if selectable {
		latencyBlocked, latencyReasons = pairQualitySelectableLatencyBlocked(live, policy)
	} else {
		latencyBlocked, latencyReasons = pairQualityProbeLatencyBlocked(live, policy)
	}
	if !successBlocked && !latencyBlocked {
		return false, nil
	}
	cond := derefString(policy.QualityCondition)
	if !combinePairQualityBlocks(successBlocked, latencyBlocked, cond) {
		if successBlocked && latencyBlocked {
			return true, []SmartScheduleCooldownReason{{Code: "and_mixed", Detail: "and 混合"}}
		}
		return false, nil
	}
	reasons := append([]SmartScheduleCooldownReason{}, successReasons...)
	reasons = append(reasons, latencyReasons...)
	return true, orderCooldownReasons(reasons)
}

func pairQualityProbeBlocksWithReasons(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) (bool, []SmartScheduleCooldownReason) {
	return pairQualityBlocksWithReasons(live, policy, false)
}

func pairQualitySelectableBlocksWithReasons(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) (bool, []SmartScheduleCooldownReason) {
	return pairQualityBlocksWithReasons(live, policy, true)
}

func pairQualityBlocks(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) bool {
	blocked, _ := pairQualitySelectableBlocksWithReasons(live, policy)
	return blocked
}

func pairQualityProbeBlocks(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) bool {
	blocked, _ := pairQualityProbeBlocksWithReasons(live, policy)
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
	pass, _ := pairQualityProbeLatencyPass(live, policy)
	return pass
}

func pairQualityProbeLatencyPass(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) (bool, string) {
	if live == nil {
		return false, LatencyEvalPending
	}
	nOK := DefaultSmartScheduleWindowN
	if policy != nil {
		nOK = policy.SuccessWindowN()
	}
	if live.OKCount < nOK {
		return false, LatencyEvalPending
	}
	if policy == nil || !policy.HasQualityMetrics() {
		return true, LatencyEvalPass
	}
	if policy.QualityMinSuccessRate != nil {
		if live.SuccessRate == nil || *live.SuccessRate < *policy.QualityMinSuccessRate {
			return false, LatencyEvalFail
		}
	}
	if policy.QualityMaxP50TTFTMs == nil && policy.QualityMaxP50DurationMs == nil {
		return true, LatencyEvalPass
	}
	if !policyProbeLatencyV2(policy) {
		return pairQualityProbeLatencyPass257(live, policy)
	}
	n := policy.TTFTWindowN()
	k, c := resolveSmartScheduleLatencyKC(policy)
	state, _ := evalLatencyWindows(
		live.TTFTMs,
		live.DurationMs,
		policy.QualityMaxP50TTFTMs,
		policy.QualityMaxP50DurationMs,
		k, c, n,
	)
	return state == LatencyEvalPass, state
}

// pairQualityProbeLatencyPass257 is production 257 graduate: W_ok full +
// success-rate pass; TTFT underfull still graduates; full TTFT window must
// pass p50. No Hold, no duration dual-window, no Q_a.
func pairQualityProbeLatencyPass257(live *PairQualityLive, policy *SmartSchedulePlatformPolicy) (bool, string) {
	if policy == nil || policy.QualityMaxP50TTFTMs == nil {
		return true, LatencyEvalPass
	}
	n := policy.TTFTWindowN()
	if live.TTFTCount < n {
		return true, LatencyEvalPass
	}
	if live.P50TTFTMs != nil && *live.P50TTFTMs > *policy.QualityMaxP50TTFTMs {
		return false, LatencyEvalFail
	}
	return true, LatencyEvalPass
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
	if pairQualityProbeBlocks(live, policy) || pairQualityProbeGraduates(live, policy) {
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
func evaluateSmartSchedulePairQuality(ctx context.Context, lookup SmartScheduleLookup, accountID, userID int64, platform string, policy *SmartSchedulePlatformPolicy, live *PairQualityLive, now time.Time, qaLastN *AccountQualityLastN) bool {
	if lookup != nil && lookup.IsPinned(ctx, accountID, userID, platform) {
		return true
	}
	probing := lookup != nil && lookup.IsProbing(ctx, accountID, userID, platform)
	minutes := DefaultSmartScheduleCooldownMinutes
	if policy != nil && policy.CooldownMinutes >= MinSmartScheduleCooldownMinutes {
		minutes = policy.CooldownMinutes
	}
	phase := CooldownPhaseSelectable
	if probing {
		phase = CooldownPhaseProbe
	}
	startCooldown := func(reasons []SmartScheduleCooldownReason, sample string) {
		if lookup == nil {
			return
		}
		if probing {
			lookup.ClearProbing(ctx, accountID, userID, platform)
		}
		detail := formatSmartScheduleCooldownDetail(phase, sample, reasons)
		lookup.StartCooldownWithReason(ctx, accountID, userID, platform, minutes, now, detail)
	}
	if probing {
		if blocked, reasons := pairQualityProbeBlocksWithReasons(live, policy); blocked {
			startCooldown(reasons, CooldownSamplePair)
			return false
		}
		if pairQualityProbeAndMixed(live, policy) {
			startCooldown([]SmartScheduleCooldownReason{{Code: "and_mixed", Detail: "and 混合"}}, CooldownSamplePair)
			return false
		}
		if policyProbeLatencyV2(policy) && qaLastN != nil && policy != nil && policy.QualityMaxP50TTFTMs != nil {
			n := policy.TTFTWindowN()
			k, c := resolveSmartScheduleLatencyKC(policy)
			qaTTFT := recentLatencySamples(qaLastN.TTFTMs, n)
			qaDur := recentLatencySamples(qaLastN.DurationMs, n)
			qaState, qaReasons := evalLatencyWindows(
				qaTTFT, qaDur,
				policy.QualityMaxP50TTFTMs,
				policy.QualityMaxP50DurationMs,
				k, c, n,
			)
			switch qaState {
			case LatencyEvalFail:
				startCooldown(prefixQAReasons(qaReasons), CooldownSampleQA)
				return false
			case LatencyEvalHold:
				return true
			case LatencyEvalPending, LatencyEvalPass:
				// continue to pair latency graduation
			}
		}
		if pass, state := pairQualityProbeLatencyPass(live, policy); pass {
			lookup.GraduateProbing(ctx, accountID, userID, platform)
		} else if policyProbeLatencyV2(policy) && state == LatencyEvalFail {
			k, c := resolveSmartScheduleLatencyKC(policy)
			n := policy.TTFTWindowN()
			if _, reasons := evalLatencyWindows(
				live.TTFTMs, live.DurationMs,
				policy.QualityMaxP50TTFTMs, policy.QualityMaxP50DurationMs,
				k, c, n,
			); len(reasons) > 0 {
				startCooldown(reasons, CooldownSamplePair)
				return false
			}
		}
		return true
	}
	if blocked, reasons := pairQualitySelectableBlocksWithReasons(live, policy); blocked {
		startCooldown(reasons, CooldownSamplePair)
		return false
	}
	return true
}
