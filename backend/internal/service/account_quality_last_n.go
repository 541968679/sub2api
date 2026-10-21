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

	// Display success rate is a time window, separate from last-N gates.
	// Buckets are kept up to the max so raising the admin setting still has history.
	DefaultAccountQualityDisplayRateMinutes = 5
	MinAccountQualityDisplayRateMinutes     = 1
	MaxAccountQualityDisplayRateMinutes     = 60

	DisplayRateModeMinutes                = "minutes"
	DisplayRateModeCount                  = "count"
	DefaultAccountQualityDisplayRateCount = 100
	MinAccountQualityDisplayRateCount     = 1
	MaxAccountQualityDisplayRateCount     = 1000

	// Last-N outcome kinds. Stored in Redis as uint8; 0 is treated as terminal
	// fail for safety. Recovered is compare-only and is omitted from OK / ErrorCount.
	LastNOutcomeSuccess   uint8 = 1
	LastNOutcomeTerminal  uint8 = 2
	LastNOutcomeRecovered uint8 = 3
)

// AccountQualityRateBucket is one UTC minute of the display success rate.
type AccountQualityRateBucket struct {
	Minute    int64 `json:"m"`
	Success   int   `json:"s,omitempty"`
	Terminal  int   `json:"t,omitempty"`
	Recovered int   `json:"r,omitempty"`
}

// AccountQualityLastN is the account-global last-N windows Q_a (all users).
// It is not the smart-schedule pair window Q_{a,u}.
type AccountQualityLastN struct {
	N           int   `json:"n"`
	UseFailover bool  `json:"use_failover,omitempty"`
	TTFTMs      []int `json:"ttft_ms,omitempty"`
	DurationMs  []int `json:"duration_ms,omitempty"`
	// TotalDurationMs is every successful request's wall time, including
	// streams that also have a first-token sample. Display only. DurationMs
	// stays the non-stream gate series.
	TotalDurationMs    []int                      `json:"total_duration_ms,omitempty"`
	OK                 []bool                     `json:"ok,omitempty"`
	Outcomes           []uint8                    `json:"outcomes,omitempty"`
	P50TTFTMs          *int                       `json:"p50_ttft_ms,omitempty"`
	P50DurationMs      *int                       `json:"p50_duration_ms,omitempty"`
	P50TotalDurationMs *int                       `json:"p50_total_duration_ms,omitempty"`
	P95TotalDurationMs *int                       `json:"p95_total_duration_ms,omitempty"`
	SuccessRate        *float64                   `json:"success_rate,omitempty"`
	TTFTCount          int                        `json:"ttft_count"`
	DurationCount      int                        `json:"duration_count"`
	TotalDurationCount int                        `json:"total_duration_count"`
	OKCount            int                        `json:"ok_count"`
	RateBuckets        []AccountQualityRateBucket `json:"rate_buckets,omitempty"`
	// DisplayOutcomes is the longer ring used when the cell rate is "last X requests".
	// Outcomes stays trimmed to the gate N.
	DisplayOutcomes []uint8   `json:"display_outcomes,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
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
	projected.TotalDurationMs = append([]int(nil), live.TotalDurationMs...)
	projected.RateBuckets = append([]AccountQualityRateBucket(nil), live.RateBuckets...)
	projected.DisplayOutcomes = append([]uint8(nil), live.DisplayOutcomes...)
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
	return applyAccountQualityLastNIngestAt(live, n, kind, firstTokenMs, durationMs, time.Now().UTC())
}

func ApplyAccountQualityLastNRecovered(live *AccountQualityLastN, n int) *AccountQualityLastN {
	return applyAccountQualityLastNIngest(live, n, LastNOutcomeRecovered, nil, nil)
}

func applyAccountQualityLastNIngest(live *AccountQualityLastN, n int, kind uint8, firstTokenMs, durationMs *int) *AccountQualityLastN {
	return applyAccountQualityLastNIngestAt(live, n, kind, firstTokenMs, durationMs, time.Now().UTC())
}

func applyAccountQualityLastNIngestAt(live *AccountQualityLastN, n int, kind uint8, firstTokenMs, durationMs *int, now time.Time) *AccountQualityLastN {
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
		if durationMs != nil && *durationMs >= 0 {
			live.TotalDurationMs = appendFIFOInt(live.TotalDurationMs, *durationMs, n)
		}
	}
	recordDisplayRate(live, kind, now)
	live.DisplayOutcomes = appendFIFOUint8(live.DisplayOutcomes, kind, MaxAccountQualityDisplayRateCount)
	RecomputeAccountQualityLastN(live)
	return live
}

func recordDisplayRate(live *AccountQualityLastN, kind uint8, now time.Time) {
	if live == nil {
		return
	}
	now = now.UTC()
	minute := now.Unix() / 60
	cutoff := minute - int64(MaxAccountQualityDisplayRateMinutes) + 1
	next := make([]AccountQualityRateBucket, 0, len(live.RateBuckets)+1)
	idx := -1
	for _, bucket := range live.RateBuckets {
		if bucket.Minute < cutoff || bucket.Minute > minute {
			continue
		}
		if bucket.Minute == minute {
			idx = len(next)
		}
		next = append(next, bucket)
	}
	if idx < 0 {
		next = append(next, AccountQualityRateBucket{Minute: minute})
		idx = len(next) - 1
	}
	switch kind {
	case LastNOutcomeSuccess:
		next[idx].Success++
	case LastNOutcomeRecovered:
		next[idx].Recovered++
	default:
		next[idx].Terminal++
	}
	live.RateBuckets = next
}

func displayRateCounts(buckets []AccountQualityRateBucket, now time.Time, window time.Duration) (success, failover int64) {
	now = now.UTC()
	minutes := int64(window / time.Minute)
	if minutes < int64(MinAccountQualityDisplayRateMinutes) {
		minutes = int64(DefaultAccountQualityDisplayRateMinutes)
	}
	if minutes > int64(MaxAccountQualityDisplayRateMinutes) {
		minutes = int64(MaxAccountQualityDisplayRateMinutes)
	}
	minute := now.Unix() / 60
	cutoff := minute - minutes + 1
	for _, bucket := range buckets {
		if bucket.Minute < cutoff || bucket.Minute > minute {
			continue
		}
		success += int64(bucket.Success)
		failover += int64(bucket.Terminal + bucket.Recovered)
	}
	return success, failover
}

// AccountQualityDisplayRateSpec is the admin-selected cell success-rate horizon.
type AccountQualityDisplayRateSpec struct {
	Mode    string
	Minutes int
	Count   int
}

func defaultAccountQualityDisplayRateSpec() AccountQualityDisplayRateSpec {
	return AccountQualityDisplayRateSpec{
		Mode:    DisplayRateModeMinutes,
		Minutes: DefaultAccountQualityDisplayRateMinutes,
		Count:   DefaultAccountQualityDisplayRateCount,
	}
}

func displayRateFromOutcomes(outcomes []uint8, count int) (success, failover int64) {
	if count < MinAccountQualityDisplayRateCount {
		count = DefaultAccountQualityDisplayRateCount
	}
	if count > MaxAccountQualityDisplayRateCount {
		count = MaxAccountQualityDisplayRateCount
	}
	if count > len(outcomes) {
		count = len(outcomes)
	}
	if count <= 0 {
		return 0, 0
	}
	tail := outcomes[len(outcomes)-count:]
	for _, kind := range tail {
		switch kind {
		case LastNOutcomeSuccess:
			success++
		default:
			failover++
		}
	}
	return success, failover
}

func attachDisplayRate(stats *AccountQualityStats, live *AccountQualityLastN, now time.Time, spec AccountQualityDisplayRateSpec) {
	if stats == nil {
		return
	}
	if spec.Mode != DisplayRateModeCount {
		spec.Mode = DisplayRateModeMinutes
	}
	var success, failover int64
	if spec.Mode == DisplayRateModeCount {
		outcomes := []uint8(nil)
		if live != nil {
			outcomes = live.DisplayOutcomes
			if len(outcomes) == 0 {
				outcomes = live.Outcomes
			}
		}
		success, failover = displayRateFromOutcomes(outcomes, spec.Count)
		stats.DisplayRateMode = DisplayRateModeCount
		stats.DisplayRateWindowCount = spec.Count
		stats.DisplayRateWindowSeconds = 0
	} else {
		var buckets []AccountQualityRateBucket
		if live != nil {
			buckets = live.RateBuckets
		}
		success, failover = displayRateCounts(buckets, now, time.Duration(spec.Minutes)*time.Minute)
		stats.DisplayRateMode = DisplayRateModeMinutes
		stats.DisplayRateWindowCount = 0
		stats.DisplayRateWindowSeconds = spec.Minutes * 60
	}
	total := success + failover
	stats.DisplaySuccessCount = success
	stats.DisplayFailoverErrorCount = failover
	if total <= 0 {
		stats.DisplayFailoverErrorRate = nil
		return
	}
	rate := float64(failover) / float64(total)
	stats.DisplayFailoverErrorRate = &rate
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
	live.TotalDurationMs = trimFIFOInt(live.TotalDurationMs, live.N)
	live.TTFTCount = len(live.TTFTMs)
	live.DurationCount = len(live.DurationMs)
	live.TotalDurationCount = len(live.TotalDurationMs)
	live.OKCount = len(live.OK)
	live.P50TTFTMs = pairQualityP50(live.TTFTMs)
	live.P50DurationMs = pairQualityP50(live.DurationMs)
	live.P50TotalDurationMs = pairQualityP50(live.TotalDurationMs)
	live.P95TotalDurationMs = pairQualityP95(live.TotalDurationMs)
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
	stats.P50DurationMs = l.P50TotalDurationMs
	stats.P95DurationMs = l.P95TotalDurationMs
	stats.DurationSamples = int64(l.TotalDurationCount)
	attachDisplayRate(stats, l, time.Now().UTC(), defaultAccountQualityDisplayRateSpec())
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
	if durationMs != nil && *durationMs >= 0 {
		observation.DurationMs = durationMs
	}
	obs.ObserveAccountCompletion(ctx, observation)
}
