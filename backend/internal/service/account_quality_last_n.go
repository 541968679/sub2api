package service

import (
	"context"
	"sort"
	"time"
)

const (
	DefaultAccountQualityWindowN = 20
	MinAccountQualityWindowN     = 1
	MaxAccountQualityWindowN     = 100

	// Last-N outcome kinds. Stored in Redis as uint8; 0 is treated as terminal
	// fail for safety. Recovered is compare-only and is omitted from OK / ErrorCount.
	LastNOutcomeSuccess   uint8 = 1
	LastNOutcomeTerminal  uint8 = 2
	LastNOutcomeRecovered uint8 = 3
)

// AccountQualityLastN is the account-global last-N windows Q_a (all users).
// It is not the smart-schedule pair window Q_{a,u}.
type AccountQualityLastN struct {
	N             int       `json:"n"`
	UseFailover   bool      `json:"use_failover,omitempty"`
	TTFTMs        []int     `json:"ttft_ms,omitempty"`
	DurationMs    []int     `json:"duration_ms,omitempty"`
	OK            []bool    `json:"ok,omitempty"`
	Outcomes      []uint8   `json:"outcomes,omitempty"`
	P50TTFTMs     *int      `json:"p50_ttft_ms,omitempty"`
	P50DurationMs *int      `json:"p50_duration_ms,omitempty"`
	SuccessRate   *float64  `json:"success_rate,omitempty"`
	TTFTCount     int       `json:"ttft_count"`
	DurationCount int       `json:"duration_count"`
	OKCount       int       `json:"ok_count"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
	// OverrideN is Q_u only: explicit per-user window. nil = inherit site N.
	OverrideN *int `json:"override_n,omitempty"`
}

// AccountQualityObservation is one completed request for the account window.
// UserID, when set, also appends the same sample to the user-global window Q_u.
// Recovered is an account-compare failure (upstream hop failed, client 200).
// It must not enter Q_u. ScheduleSide feeds precheck / public-schedule only
// when the row is also counted in the schedule caliber (e.g. wait-timeout).
type AccountQualityObservation struct {
	AccountID    int64
	UserID       int64
	Success      bool
	Recovered    bool
	ScheduleSide bool
	FirstTokenMs *int
	DurationMs   *int
}

// AccountQualityObserver is the completion-path hook for Q_a (all users).
type AccountQualityObserver interface {
	ObserveAccountCompletion(ctx context.Context, obs AccountQualityObservation)
}

// AccountQualityLastNCache stores account-global FIFO windows.
type AccountQualityLastNCache interface {
	GetLastN(ctx context.Context, accountID int64) *AccountQualityLastN
	GetLastNBatch(ctx context.Context, accountIDs []int64) map[int64]*AccountQualityLastN
	IngestLastN(ctx context.Context, accountID int64, n int, success bool, firstTokenMs, durationMs *int, useFailover bool, recovered bool) *AccountQualityLastN
	IngestPrecheckSample(ctx context.Context, accountID, userID int64, success bool, firstTokenMs, durationMs *int)
	ListLastNAccountIDs(ctx context.Context) []int64
}

// UserQualityLastNCache stores user-global FIFO windows Q_u (this user, all accounts).
// Window N is per-user (override) or the site-wide account-quality N (inherit).
type UserQualityLastNCache interface {
	GetUserLastN(ctx context.Context, userID int64) *AccountQualityLastN
	GetUserLastNBatch(ctx context.Context, userIDs []int64) map[int64]*AccountQualityLastN
	IngestUserLastN(ctx context.Context, userID int64, n int, success bool, firstTokenMs, durationMs *int, useFailover bool, override *int) *AccountQualityLastN
	ResizeUserLastN(ctx context.Context, userID int64, n int, override *int) *AccountQualityLastN
	ListUserLastNIDs(ctx context.Context) []int64
}

// ClampAccountQualityWindowN clamps an explicit N to 1–100.
func ClampAccountQualityWindowN(n int) int {
	if n < MinAccountQualityWindowN {
		return MinAccountQualityWindowN
	}
	if n > MaxAccountQualityWindowN {
		return MaxAccountQualityWindowN
	}
	return n
}

// NormalizeAccountQualityWindowN matches frontend resolveAccountQualityWindowN:
// explicit N, then min_success_samples, then min_ttft_samples, else 20.
func NormalizeAccountQualityWindowN(window, minSuccess, minTTFT *int) int {
	if window != nil {
		return ClampAccountQualityWindowN(*window)
	}
	if minSuccess != nil {
		return ClampAccountQualityWindowN(*minSuccess)
	}
	if minTTFT != nil {
		return ClampAccountQualityWindowN(*minTTFT)
	}
	return DefaultAccountQualityWindowN
}

func EchoAccountQualityWindowN(n int) (window, minSuccess, minTTFT int) {
	n = ClampAccountQualityWindowN(n)
	return n, n, n
}

// ResolveUserQualityWindowN is Q_u: explicit per-user override, else site account-quality N.
func ResolveUserQualityWindowN(override *int, siteN int) int {
	if override != nil {
		return ClampAccountQualityWindowN(*override)
	}
	return ClampAccountQualityWindowN(siteN)
}

func CopyIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	copied := *v
	return &copied
}

func ProjectAccountQualityLastN(live *AccountQualityLastN, n int) *AccountQualityLastN {
	if live == nil {
		empty := &AccountQualityLastN{N: ClampAccountQualityWindowN(n)}
		RecomputeAccountQualityLastN(empty)
		return empty
	}
	projected := *live
	projected.TTFTMs = append([]int(nil), live.TTFTMs...)
	projected.DurationMs = append([]int(nil), live.DurationMs...)
	projected.OK = append([]bool(nil), live.OK...)
	projected.Outcomes = append([]uint8(nil), live.Outcomes...)
	projected.N = ClampAccountQualityWindowN(n)
	projected.OverrideN = CopyIntPtr(live.OverrideN)
	RecomputeAccountQualityLastN(&projected)
	return &projected
}

func (s *QualityHardCloseSettings) explicitWindowN() *int {
	if s == nil {
		return nil
	}
	if s.AccountQualityWindowN != nil {
		return s.AccountQualityWindowN
	}
	if s.WindowN != nil {
		return s.WindowN
	}
	return s.N
}

func (s *QualityHardCloseSettings) ResolvedWindowN() int {
	if s == nil {
		return DefaultAccountQualityWindowN
	}
	return NormalizeAccountQualityWindowN(s.explicitWindowN(), intPtrIfPositive(s.MinSuccessSamples), intPtrIfPositive(s.MinTTFTSamples))
}

func intPtrIfPositive(n int) *int {
	if n < 1 {
		return nil
	}
	copied := n
	return &copied
}

func (s QualityHardCloseSettings) sampleFloors() (minSuccess, minTTFT int) {
	if s.explicitWindowN() != nil {
		n := s.ResolvedWindowN()
		return n, n
	}
	minSuccess = s.MinSuccessSamples
	if minSuccess < 1 {
		minSuccess = DefaultQualityHardCloseMinSuccessSamples
	}
	minTTFT = s.MinTTFTSamples
	if minTTFT < 1 {
		minTTFT = DefaultQualityHardCloseMinTTFTSamples
	}
	return minSuccess, minTTFT
}

func echoQualityHardCloseWindowN(settings *QualityHardCloseSettings) {
	if settings == nil {
		return
	}
	n, success, ttft := EchoAccountQualityWindowN(settings.ResolvedWindowN())
	settings.AccountQualityWindowN = &n
	settings.WindowN = &n
	settings.N = &n
	settings.MinSuccessSamples = success
	settings.MinTTFTSamples = ttft
}

func StampAccountQualityWindowN(stats *AccountQualityStats, n int) {
	if stats == nil {
		return
	}
	n = ClampAccountQualityWindowN(n)
	stats.N = n
	stats.WindowN = n
	stats.AccountQualityWindowN = n
}

// StampAccountQualityLatencyKC copies pair-quality K/C display fields onto Q_a stats.
func StampAccountQualityLatencyKC(stats *AccountQualityStats, ttft []int, knobs QualityEvalKnobs) {
	if stats == nil || knobs.TTFTMax == nil || *knobs.TTFTMax < 1 {
		return
	}
	samples := recentLatencySamples(ttft, knobs.LatencyN)
	if knobs.K > 0 {
		stats.TTFTSlowCount = intPtr(countSlowSamples(samples, *knobs.TTFTMax))
		stats.QualitySchedMaxSlowInWindow = intPtr(knobs.K)
	}
	if knobs.C > 0 {
		stats.TTFTConsecutiveSlow = intPtr(countTrailingSlow(samples, *knobs.TTFTMax))
		stats.QualitySchedMaxConsecutiveSlow = intPtr(knobs.C)
	}
}

func ApplyAccountQualityLastNIngest(live *AccountQualityLastN, n int, success bool, firstTokenMs, durationMs *int) *AccountQualityLastN {
	kind := LastNOutcomeTerminal
	if success {
		kind = LastNOutcomeSuccess
	}
	return applyAccountQualityLastNIngest(live, n, kind, firstTokenMs, durationMs)
}

func ApplyAccountQualityLastNRecovered(live *AccountQualityLastN, n int) *AccountQualityLastN {
	return applyAccountQualityLastNIngest(live, n, LastNOutcomeRecovered, nil, nil)
}

func applyAccountQualityLastNIngest(live *AccountQualityLastN, n int, kind uint8, firstTokenMs, durationMs *int) *AccountQualityLastN {
	if live == nil {
		live = &AccountQualityLastN{}
	}
	n = ClampAccountQualityWindowN(n)
	live.N = n
	live.ensureOutcomes()
	if kind != LastNOutcomeSuccess && kind != LastNOutcomeRecovered {
		kind = LastNOutcomeTerminal
	}
	live.Outcomes = appendFIFOUint8(live.Outcomes, kind, n)
	if kind == LastNOutcomeSuccess {
		if firstTokenMs != nil && *firstTokenMs >= 0 {
			live.TTFTMs = appendFIFOInt(live.TTFTMs, *firstTokenMs, n)
		} else if durationMs != nil && *durationMs >= 0 {
			live.DurationMs = appendFIFOInt(live.DurationMs, *durationMs, n)
		}
	}
	RecomputeAccountQualityLastN(live)
	return live
}

func (l *AccountQualityLastN) ensureOutcomes() {
	if l == nil || len(l.Outcomes) > 0 {
		return
	}
	if len(l.OK) == 0 {
		return
	}
	l.Outcomes = make([]uint8, len(l.OK))
	for i, ok := range l.OK {
		if ok {
			l.Outcomes[i] = LastNOutcomeSuccess
		} else {
			l.Outcomes[i] = LastNOutcomeTerminal
		}
	}
}

func okFromLastNOutcomes(outcomes []uint8) []bool {
	ok := make([]bool, 0, len(outcomes))
	for _, kind := range outcomes {
		switch kind {
		case LastNOutcomeSuccess:
			ok = append(ok, true)
		case LastNOutcomeTerminal:
			ok = append(ok, false)
		}
	}
	return ok
}

func countLastNOutcomes(outcomes []uint8) (success, terminal, failover int64) {
	for _, kind := range outcomes {
		switch kind {
		case LastNOutcomeSuccess:
			success++
		case LastNOutcomeRecovered:
			failover++
		default:
			terminal++
			failover++
		}
	}
	return success, terminal, failover
}

func RecomputeAccountQualityLastN(live *AccountQualityLastN) {
	if live == nil {
		return
	}
	if live.N < MinAccountQualityWindowN {
		live.N = DefaultAccountQualityWindowN
	}
	live.ensureOutcomes()
	live.Outcomes = trimFIFOUint8(live.Outcomes, live.N)
	live.OK = okFromLastNOutcomes(live.Outcomes)
	live.TTFTMs = trimFIFOInt(live.TTFTMs, live.N)
	live.DurationMs = trimFIFOInt(live.DurationMs, live.N)
	live.TTFTCount = len(live.TTFTMs)
	live.DurationCount = len(live.DurationMs)
	live.OKCount = len(live.OK)
	live.P50TTFTMs = pairQualityP50(live.TTFTMs)
	live.P50DurationMs = pairQualityP50(live.DurationMs)
	live.SuccessRate = pairQualitySuccessRate(live.OK)
	live.UpdatedAt = time.Now().UTC()
}

// ToAccountQualityStats projects last-N windows onto the existing grid / hard-close DTO.
func (l *AccountQualityLastN) ToAccountQualityStats() *AccountQualityStats {
	if l == nil {
		stats := BuildAccountQualityStats(0, 0, TTFTAggregate{})
		StampAccountQualityWindowN(stats, DefaultAccountQualityWindowN)
		return stats
	}
	l.ensureOutcomes()
	success, terminal, failover := countLastNOutcomes(l.Outcomes)
	stats := BuildAccountQualityStats(success, terminal, lastNTTFTAggregate(l.TTFTMs))
	stats.P50TTFTMs = l.P50TTFTMs
	if l.SuccessRate != nil {
		rate := *l.SuccessRate
		stats.SuccessRate = &rate
	}
	stats.ScheduleUseFailoverErrorRate = l.UseFailover
	stats.TerminalErrorCount = terminal
	stats.FailoverErrorCount = failover
	stats.TerminalErrorRate = nil
	stats.FailoverErrorRate = nil
	NormalizeAccountQualityRates(stats)
	StampAccountQualityWindowN(stats, l.N)
	return stats
}

func lastNTTFTAggregate(samples []int) TTFTAggregate {
	out := TTFTAggregate{Samples: int64(len(samples))}
	if len(samples) == 0 {
		return out
	}
	sorted := append([]int(nil), samples...)
	sort.Ints(sorted)
	sum := 0
	max := sorted[0]
	for _, v := range sorted {
		sum += v
		if v > max {
			max = v
		}
	}
	avg := float64(sum) / float64(len(sorted))
	out.Avg = &avg
	p50 := float64(*pairQualityP50(samples))
	out.P50 = &p50
	p95 := float64(sorted[p95Index(len(sorted))])
	out.P95 = &p95
	maxF := float64(max)
	out.Max = &maxF
	return out
}

func p95Index(n int) int {
	if n <= 1 {
		return n - 1
	}
	idx := (n*95 + 99) / 100
	if idx >= n {
		return n - 1
	}
	if idx < 0 {
		return 0
	}
	return idx
}

func observeAccountQualitySuccess(obs AccountQualityObserver, ctx context.Context, accountID, userID int64, trueMs, firstMs, durationMs *int) {
	if obs == nil || (accountID <= 0 && userID <= 0) {
		return
	}
	ttft := pairQualityTTFTMs(trueMs, firstMs)
	observation := AccountQualityObservation{
		AccountID:    accountID,
		UserID:       userID,
		Success:      true,
		FirstTokenMs: ttft,
	}
	if ttft == nil && durationMs != nil && *durationMs >= 0 {
		observation.DurationMs = durationMs
	}
	obs.ObserveAccountCompletion(ctx, observation)
}
