//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSmartScheduleWindowN(t *testing.T) {
	t.Parallel()
	require.Equal(t, 10, NormalizeSmartScheduleWindowN(nil, nil, nil))
	require.Equal(t, 7, NormalizeSmartScheduleWindowN(intPtr(7), intPtr(20), intPtr(3)))
	require.Equal(t, 4, NormalizeSmartScheduleWindowN(nil, intPtr(4), nil))
	require.Equal(t, 8, NormalizeSmartScheduleWindowN(nil, nil, intPtr(8)))
	require.Equal(t, 3, NormalizeSmartScheduleWindowN(nil, intPtr(3), intPtr(9)))
	require.Equal(t, 10, NormalizeSmartScheduleWindowN(intPtr(0), nil, nil))
	require.Equal(t, 100, NormalizeSmartScheduleWindowN(intPtr(200), nil, nil))
	require.Equal(t, 1, NormalizeSmartScheduleWindowN(intPtr(1), nil, nil))
}

func TestApplyPairQualityIngest_Rules(t *testing.T) {
	t.Parallel()
	n := 3
	ttft := 120

	live := ApplyPairQualityIngest(nil, n, false, &ttft)
	require.Equal(t, 1, live.OKCount)
	require.Equal(t, 0, live.TTFTCount)
	require.Nil(t, live.P50TTFTMs)
	require.NotNil(t, live.SuccessRate)
	require.Equal(t, 0.0, *live.SuccessRate)

	live = ApplyPairQualityIngest(live, n, true, nil)
	require.Equal(t, 2, live.OKCount)
	require.Equal(t, 0, live.TTFTCount)

	live = ApplyPairQualityIngest(live, n, true, &ttft)
	require.Equal(t, 3, live.OKCount)
	require.Equal(t, 1, live.TTFTCount)
	require.Equal(t, 120, *live.P50TTFTMs)

	slow := 400
	live = ApplyPairQualityIngest(live, n, true, &slow)
	require.Equal(t, 3, live.OKCount, "W_ok is FIFO capped at N")
	require.Equal(t, 2, live.TTFTCount)
}

func TestPairQualityProbeBlocks_UnderNAndOr(t *testing.T) {
	t.Parallel()
	p50 := 100
	rate := 0.9
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:   &p50,
		QualityMinSuccessRate: &rate,
		QualityWindowSamples:  intPtr(3),
		QualityCondition:      strPtr(QualityHardCloseConditionOr),
	}
	live := ApplyPairQualityIngest(nil, 3, true, intPtr(400))
	require.False(t, pairQualityProbeBlocks(live, policy), "W_ttft count 1 < N must not cool")

	live = ApplyPairQualityIngest(live, 3, true, intPtr(400))
	live = ApplyPairQualityIngest(live, 3, true, intPtr(400))
	require.True(t, pairQualityProbeBlocks(live, policy), "full W_ttft p50 breach must cool on or")

	andPolicy := *policy
	and := QualityHardCloseConditionAnd
	andPolicy.QualityCondition = &and
	require.False(t, pairQualityProbeBlocks(live, &andPolicy), "and: success window is full of successes so success metric is not breached")
}

func TestPairQualitySelectableBlocks_SchedN20(t *testing.T) {
	t.Parallel()
	p50 := 100
	rate := 0.9
	policy := withSchedComposite(&SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:   &p50,
		QualityMinSuccessRate: &rate,
		QualityWindowSamples:  intPtr(5),
		QualityCondition:      strPtr(QualityHardCloseConditionOr),
	})
	// 2 consecutive slow — C=3 not met, K=6 not met
	live := ApplyPairQualityIngestWindows(nil, 20, 5, true, intPtr(150), nil)
	live = ApplyPairQualityIngestWindows(live, 20, 5, true, intPtr(160), nil)
	require.False(t, pairQualityBlocks(live, policy), "2 consecutive slow must not sched-cool (C=3)")

	// 6 slow then 14 fast — K ready at 6
	live = nil
	slowVals := []int{150, 150, 150, 150, 150, 150, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50, 50}
	for _, v := range slowVals {
		live = ApplyPairQualityIngestWindows(live, 20, 5, true, intPtr(v), nil)
	}
	require.True(t, pairQualityBlocks(live, policy), "6 slow in 20 must sched-cool on K")

	// 10 samples, last 3 consecutive slow — C ready at 3
	live = nil
	for i := 0; i < 7; i++ {
		live = ApplyPairQualityIngestWindows(live, 20, 5, true, intPtr(50), nil)
	}
	for i := 0; i < 3; i++ {
		live = ApplyPairQualityIngestWindows(live, 20, 5, true, intPtr(150), nil)
	}
	require.True(t, pairQualityBlocks(live, policy), "10 samples last 3 consecutive must sched-cool on C")
}

func TestPairQualityToStats_CrossUserIsolationShape(t *testing.T) {
	t.Parallel()
	live := ApplyPairQualityIngest(nil, 2, true, intPtr(50))
	stats := live.ToAccountQualityStats()
	require.Equal(t, int64(1), stats.TTFTSamples)
	require.Equal(t, int64(1), stats.SuccessCount)
	require.Equal(t, int64(0), stats.ErrorCount)
}

type observeCacheStub struct {
	stubSmartCache
	bundle             *UserSmartScheduleBundle
	live               map[string]*PairQualityLive
	cooling            map[string]bool
	probing            map[string]bool
	pinned             map[string]bool
	resumeUntil        map[string]int64
	ingested           []PairQualityObservation
	starts             int
	graduated          int
	lastCooldownReason string
	softLive           map[string]*PairQualityLive
	softIngested       []int64
	softEnded          []int64
	events             []string
}

func (s *observeCacheStub) Lookup(context.Context, int64) *UserSmartScheduleBundle { return s.bundle }
func (s *observeCacheStub) CooldownActive(_ context.Context, accountID, userID int64, _ string, _ time.Time) bool {
	return s.cooling[smartPairKey(accountID, userID)]
}
func (s *observeCacheStub) StartCooldown(_ context.Context, accountID, userID int64, _ string, _ int, _ time.Time) {
	s.starts++
	if s.cooling == nil {
		s.cooling = map[string]bool{}
	}
	s.cooling[smartPairKey(accountID, userID)] = true
	delete(s.softLive, smartPairKey(accountID, userID))
}
func (s *observeCacheStub) StartCooldownWithReason(_ context.Context, accountID, userID int64, _ string, _ int, _ time.Time, reason string) {
	s.starts++
	s.lastCooldownReason = reason
	if s.cooling == nil {
		s.cooling = map[string]bool{}
	}
	s.cooling[smartPairKey(accountID, userID)] = true
	delete(s.softLive, smartPairKey(accountID, userID))
}
func (s *observeCacheStub) GetPairQuality(_ context.Context, accountID, userID int64, _ string) *PairQualityLive {
	return s.live[smartPairKey(accountID, userID)]
}
func (s *observeCacheStub) GetPairQualityBatch(_ context.Context, accountIDs []int64, userID int64, _ string) map[int64]*PairQualityLive {
	out := map[int64]*PairQualityLive{}
	for _, accountID := range accountIDs {
		if live := s.GetPairQuality(context.Background(), accountID, userID, ""); live != nil {
			out[accountID] = live
		}
	}
	return out
}
func (s *observeCacheStub) IngestPairQuality(_ context.Context, accountID, userID int64, _ string, nTTFT, nOK int, success bool, firstTokenMs, durationMs *int) *PairQualityLive {
	s.ingested = append(s.ingested, PairQualityObservation{AccountID: accountID, UserID: userID, Success: success, FirstTokenMs: firstTokenMs, DurationMs: durationMs})
	key := smartPairKey(accountID, userID)
	if s.live == nil {
		s.live = map[string]*PairQualityLive{}
	}
	s.live[key] = ApplyPairQualityIngestWindows(s.live[key], nTTFT, nOK, success, firstTokenMs, durationMs)
	return s.live[key]
}

func (s *observeCacheStub) IsProbing(_ context.Context, accountID, userID int64, _ string) bool {
	return s.probing[smartPairKey(accountID, userID)]
}

func (s *observeCacheStub) MarkProbing(_ context.Context, accountID, userID int64, _ string) {
	if s.probing == nil {
		s.probing = map[string]bool{}
	}
	s.probing[smartPairKey(accountID, userID)] = true
}

func (s *observeCacheStub) ClearProbing(_ context.Context, accountID, userID int64, _ string) {
	delete(s.probing, smartPairKey(accountID, userID))
}

func (s *observeCacheStub) GraduateProbing(ctx context.Context, accountID, userID int64, platform string) {
	if s.IsProbing(ctx, accountID, userID, platform) {
		s.graduated++
	}
	s.ClearProbing(ctx, accountID, userID, platform)
}

func (s *observeCacheStub) IsPinned(_ context.Context, accountID, userID int64, _ string) bool {
	return s.pinned[smartPairKey(accountID, userID)]
}

func (s *observeCacheStub) MarkPinned(_ context.Context, accountID, userID int64, _ string) {
	if s.pinned == nil {
		s.pinned = map[string]bool{}
	}
	s.pinned[smartPairKey(accountID, userID)] = true
}

func (s *observeCacheStub) ClearPinned(_ context.Context, accountID, userID int64, _ string) {
	delete(s.pinned, smartPairKey(accountID, userID))
}

func (s *observeCacheStub) PairResumeActive(_ context.Context, accountID, userID int64, platform string, now time.Time) bool {
	return s.resumeUntil[smartPairPlatformKey(accountID, userID, platform)] > now.Unix()
}

func (s *observeCacheStub) ClearPairResume(_ context.Context, accountID, userID int64, platform string) {
	delete(s.resumeUntil, smartPairPlatformKey(accountID, userID, platform))
}

func (s *observeCacheStub) MarkPairResume(_ context.Context, accountID, userID int64, platform string) error {
	if s.resumeUntil == nil {
		s.resumeUntil = map[string]int64{}
	}
	s.resumeUntil[smartPairPlatformKey(accountID, userID, platform)] = time.Now().UTC().Add(2 * AccountQualityWindow).Unix()
	return nil
}

func (s *observeCacheStub) GetPairResumeUntilBatch(_ context.Context, accountIDs []int64, userID int64, platform string, now time.Time) map[int64]PairResumeUntil {
	out := map[int64]PairResumeUntil{}
	for _, accountID := range accountIDs {
		until := s.resumeUntil[smartPairPlatformKey(accountID, userID, platform)]
		if until > now.Unix() {
			out[accountID] = PairResumeUntil{WatchUntil: time.Unix(until, 0).UTC()}
		}
	}
	return out
}

func (s *observeCacheStub) Invalidate(context.Context, int64) error                       { return nil }
func (s *observeCacheStub) ClearCooldown(context.Context, int64, int64, string) error     { return nil }
func (s *observeCacheStub) ClearCooldownAllPlatforms(context.Context, int64, int64) error { return nil }
func (s *observeCacheStub) SetCooldown(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time) (time.Time, error) {
	return s.SetCooldownWithReason(ctx, accountID, userID, platform, minutes, now, "")
}
func (s *observeCacheStub) SetCooldownWithReason(_ context.Context, accountID, userID int64, _ string, minutes int, now time.Time, _ string) (time.Time, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if minutes < 1 {
		minutes = 15
	}
	if s.cooling == nil {
		s.cooling = map[string]bool{}
	}
	s.cooling[smartPairKey(accountID, userID)] = true
	delete(s.softLive, smartPairKey(accountID, userID))
	return now.Add(time.Duration(minutes) * time.Minute), nil
}
func (s *observeCacheStub) GetCooldownReason(context.Context, int64, int64, string) string { return "" }
func (s *observeCacheStub) ApplyMemberPaused(context.Context, int64, int64, string, bool) error {
	return nil
}
func (s *observeCacheStub) GetCooldownUntilBatch(_ context.Context, accountIDs []int64, userID int64, _ string, now time.Time) map[int64]time.Time {
	out := map[int64]time.Time{}
	for _, accountID := range accountIDs {
		if s.cooling[smartPairKey(accountID, userID)] {
			out[accountID] = now.Add(15 * time.Minute)
		}
	}
	return out
}

func (s *observeCacheStub) IngestSoftCooldown(_ context.Context, accountID, userID int64, _ string, nTTFT, nOK int, success bool, firstTokenMs, durationMs *int, _ int) *PairQualityLive {
	s.softIngested = append(s.softIngested, accountID)
	key := smartPairKey(accountID, userID)
	if s.softLive == nil {
		s.softLive = map[string]*PairQualityLive{}
	}
	s.softLive[key] = ApplyPairQualityIngestWindows(s.softLive[key], nTTFT, nOK, success, firstTokenMs, durationMs)
	return s.softLive[key]
}

func (s *observeCacheStub) GetSoftCooldown(_ context.Context, accountID, userID int64, _ string) *PairQualityLive {
	return s.softLive[smartPairKey(accountID, userID)]
}

func (s *observeCacheStub) ZeroSoftCooldown(_ context.Context, accountID, userID int64, _ string) {
	delete(s.softLive, smartPairKey(accountID, userID))
}

func (s *observeCacheStub) GetSoftCooldownBatch(_ context.Context, accountIDs []int64, userID int64, _ string) map[int64]*PairQualityLive {
	out := map[int64]*PairQualityLive{}
	for _, accountID := range accountIDs {
		if live := s.GetSoftCooldown(context.Background(), accountID, userID, ""); live != nil {
			out[accountID] = live
		}
	}
	return out
}

func (s *observeCacheStub) SoftEndCooldown(_ context.Context, accountID, userID int64, _ string, _ string) {
	s.softEnded = append(s.softEnded, accountID)
	s.events = append(s.events, PairQualityEventSoftCooldownEnd)
	delete(s.cooling, smartPairKey(accountID, userID))
	delete(s.softLive, smartPairKey(accountID, userID))
	if s.probing == nil {
		s.probing = map[string]bool{}
	}
	s.probing[smartPairKey(accountID, userID)] = true
	s.events = append(s.events, PairQualityEventProbeEnter)
}

func (s *observeCacheStub) AppendPairQualityEvent(_ context.Context, _ int64, _ int64, _ string, event PairQualityEvent) {
	if event.Type != "" {
		s.events = append(s.events, event.Type)
	}
}
func (s *observeCacheStub) ZeroPairQuality(context.Context, int64, int64, string, string) {}
func (s *observeCacheStub) ListPairQualitySnapshots(context.Context, int64, int64, string, int) []PairQualitySnapshot {
	return nil
}
func (s *observeCacheStub) ListPairQualityEvents(context.Context, int64, int64, string, int) []PairQualityEvent {
	return nil
}
func (s *observeCacheStub) IsProbingBatch(context.Context, []int64, int64, string) map[int64]bool {
	return map[int64]bool{}
}
func (s *observeCacheStub) IsPinnedBatch(context.Context, []int64, int64, string) map[int64]bool {
	return map[int64]bool{}
}

func TestObservePairCompletion_SkipsPausedCoolingAndEvaluatesAfterN(t *testing.T) {
	t.Parallel()
	p50 := 50
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.QualityWindowSamples = intPtr(2)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		live:    map[string]*PairQualityLive{},
		cooling: map[string]bool{},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)

	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: false})
	require.Len(t, cache.ingested, 1)
	require.Equal(t, 0, cache.starts, "failure only fills W_ok; under N must not cool")

	cache.cooling[smartPairKey(7, 16)] = true
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true, FirstTokenMs: intPtr(400)})
	require.Len(t, cache.ingested, 1, "cooling pair must not ingest")

	cache.cooling[smartPairKey(7, 16)] = false
	policy.Paused = map[int64]struct{}{7: {}}
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true, FirstTokenMs: intPtr(400)})
	require.Len(t, cache.ingested, 1, "paused pair must not ingest")

	policy.Paused = nil
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true, FirstTokenMs: intPtr(400)})
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true, FirstTokenMs: intPtr(400)})
	require.Equal(t, 1, cache.starts)
	require.Nil(t, cache.GetPairQuality(context.Background(), 9, 16, "openai"))
}

func TestObservePairCompletion_ResumeIngestsWithoutEvaluate(t *testing.T) {
	t.Parallel()
	p50 := 50
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.QualityWindowSamples = intPtr(1)
	cache := &observeCacheStub{
		bundle: smartBundle(PlatformAnthropic, policy),
		live:   map[string]*PairQualityLive{},
	}
	require.NoError(t, cache.MarkPairResume(context.Background(), 7, 16, PlatformAnthropic))
	quality := &liveQualityCacheStub{}
	require.NoError(t, quality.MarkUserResume(context.Background(), 7, 16))
	svc := NewUserSmartScheduleService(nil, cache, nil, quality, nil)
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{
		AccountID:    7,
		UserID:       16,
		Success:      true,
		FirstTokenMs: intPtr(400),
	})
	require.Len(t, cache.ingested, 1)
	require.Equal(t, 0, cache.starts)
}

func TestObservePairQualityErrors_FailoverSwitch(t *testing.T) {
	t.Parallel()
	observer := &pairObserverStub{}
	accountID, userID := int64(7), int64(16)
	recovered := &OpsInsertErrorLogInput{
		AccountID:    &accountID,
		UserID:       &userID,
		StatusCode:   200,
		ErrorPhase:   "upstream",
		ErrorType:    "rate_limit_error",
		ErrorMessage: "Recovered upstream error 429: too many requests",
	}
	terminal := &OpsInsertErrorLogInput{
		AccountID:    &accountID,
		UserID:       &userID,
		StatusCode:   502,
		ErrorPhase:   "upstream",
		ErrorType:    "upstream_error",
		ErrorMessage: "bad gateway",
	}

	off := &OpsService{pairQuality: observer}
	off.observePairQualityErrors(context.Background(), []*OpsInsertErrorLogInput{recovered, terminal})
	require.Len(t, observer.obs, 1, "recovered hop is excluded when failover toggle is off")

	settings, err := json.Marshal(QualityHardCloseSettings{ScheduleUseFailoverErrorRate: true})
	require.NoError(t, err)
	repo := newRuntimeSettingRepoStub()
	repo.values[SettingKeyQualityHardCloseSettings] = string(settings)
	on := &OpsService{pairQuality: observer, settingRepo: repo}
	observer.obs = nil
	on.observePairQualityErrors(context.Background(), []*OpsInsertErrorLogInput{recovered})
	require.Len(t, observer.obs, 1, "recovered hop enters W_ok when failover toggle is on")
}

func TestObservePairQualityErrors_ScheduleExclude(t *testing.T) {
	t.Parallel()
	accountID, userID := int64(1719), int64(9)
	cases := []struct {
		name    string
		entry   *OpsInsertErrorLogInput
		observe bool
	}{
		{
			name: "group_no_account_502",
			entry: &OpsInsertErrorLogInput{
				AccountID: &accountID, UserID: &userID,
				StatusCode: 502, ErrorPhase: "upstream", ErrorType: "upstream_error",
				ErrorMessage: `Model "gpt-5.6-terra" is not supported by any configured account in this group`,
			},
			observe: true,
		},
		{
			name: "client_400",
			entry: &OpsInsertErrorLogInput{
				AccountID: &accountID, UserID: &userID,
				StatusCode: 400, ErrorPhase: "request", ErrorType: "invalid_request_error",
				ErrorMessage: "missing required parameter",
			},
			observe: true,
		},
		{
			name: "pair_concurrency",
			entry: &OpsInsertErrorLogInput{
				AccountID: &accountID, UserID: &userID,
				StatusCode: 429, ErrorPhase: "request", ErrorType: "rate_limit_error",
				ErrorMessage: "Concurrency limit exceeded for account",
			},
			observe: true,
		},
		{
			name: "legacy_routing_miss_404",
			entry: &OpsInsertErrorLogInput{
				AccountID: &accountID, UserID: &userID,
				StatusCode: 404, ErrorPhase: "internal", ErrorType: "model_not_found",
				ErrorMessage: "model_not_found: claude-bad",
			},
		},
		{
			name: "upstream_request_failed_502",
			entry: &OpsInsertErrorLogInput{
				AccountID: &accountID, UserID: &userID,
				StatusCode: 502, ErrorPhase: "upstream", ErrorType: "upstream_error",
				ErrorMessage: "Upstream request failed",
			},
			observe: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observer := &pairObserverStub{}
			svc := &OpsService{pairQuality: observer}
			svc.observePairQualityErrors(context.Background(), []*OpsInsertErrorLogInput{tc.entry})
			if tc.observe {
				require.Len(t, observer.obs, 1)
				require.False(t, observer.obs[0].Success)
				return
			}
			require.Empty(t, observer.obs)
		})
	}
}

func TestObservePairQualityErrors_GroupNoAccountWhitelistOn(t *testing.T) {
	t.Parallel()
	accountID, userID := int64(1719), int64(9)
	wl := whitelistAll(true)
	raw, err := json.Marshal(wl)
	require.NoError(t, err)
	repo := newRuntimeSettingRepoStub()
	repo.values[SettingKeyScheduleErrorWhitelist] = string(raw)
	observer := &pairObserverStub{}
	svc := &OpsService{pairQuality: observer, settingRepo: repo}
	svc.observePairQualityErrors(context.Background(), []*OpsInsertErrorLogInput{{
		AccountID: &accountID, UserID: &userID,
		StatusCode: 502, ErrorPhase: "upstream", ErrorType: "upstream_error",
		ErrorMessage: `Model "gpt-5.6-terra" is not supported by any configured account in this group`,
	}})
	require.Empty(t, observer.obs)
}

type pairObserverStub struct {
	obs []PairQualityObservation
}

func (s *pairObserverStub) ObservePairCompletion(_ context.Context, obs PairQualityObservation) {
	s.obs = append(s.obs, obs)
}

func TestNormalizeSmartScheduleWrite_WindowN(t *testing.T) {
	t.Parallel()
	p50 := 200
	got, err := normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		QualityMaxP50TTFTMs:      &p50,
		QualityMinSuccessSamples: intPtr(20),
		QualityMinTTFTSamples:    intPtr(4),
		CooldownMinutes:          15,
	})
	require.NoError(t, err)
	require.Equal(t, 4, *got.QualityMinTTFTSamples)
	require.Equal(t, 20, *got.QualityMinSuccessSamples)
	require.Equal(t, 20, *got.QualityWindowSamples, "compat alias is max of the two N")
	require.Equal(t, 20, *got.QualityWindowN)

	legacy, err := normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		QualityMaxP50TTFTMs: &p50,
		QualityWindowN:      intPtr(10),
		CooldownMinutes:     15,
	})
	require.NoError(t, err)
	require.Equal(t, 10, *legacy.QualityMinTTFTSamples)
	require.Equal(t, 10, *legacy.QualityMinSuccessSamples)

	_, err = normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		QualityMaxP50TTFTMs:  &p50,
		QualityWindowSamples: intPtr(0),
		CooldownMinutes:      15,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "quality_window_samples")

	// Changing only N首字 must not copy quality_window_n onto N成功率.
	oneColumn, err := normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		QualityMaxP50TTFTMs:   &p50,
		QualityWindowN:        intPtr(20),
		QualityMinTTFTSamples: intPtr(4),
		CooldownMinutes:       15,
	})
	require.NoError(t, err)
	require.Equal(t, 4, *oneColumn.QualityMinTTFTSamples)
	require.Equal(t, DefaultSmartScheduleWindowN, *oneColumn.QualityMinSuccessSamples)

	existing := &SmartSchedulePlatformPolicy{
		QualityMinTTFTSamples:    intPtr(10),
		QualityMinSuccessSamples: intPtr(20),
	}
	overlaid := overlayExistingSmartScheduleWindows(SmartSchedulePlatformWrite{
		QualityMaxP50TTFTMs:   &p50,
		QualityWindowN:        intPtr(20),
		QualityMinTTFTSamples: intPtr(4),
		CooldownMinutes:       15,
	}, oneColumn, existing)
	require.Equal(t, 4, *overlaid.QualityMinTTFTSamples)
	require.Equal(t, 20, *overlaid.QualityMinSuccessSamples)

	omitted, err := normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		QualityMaxP50TTFTMs: &p50,
		CooldownMinutes:     15,
	})
	require.NoError(t, err)
	kept := overlayExistingSmartScheduleWindows(SmartSchedulePlatformWrite{
		QualityMaxP50TTFTMs: &p50,
		CooldownMinutes:     15,
	}, omitted, existing)
	require.Equal(t, 10, *kept.QualityMinTTFTSamples)
	require.Equal(t, 20, *kept.QualityMinSuccessSamples)
}

func TestApplyPairQualityIngestWindows_IndependentFIFOs(t *testing.T) {
	t.Parallel()
	var live *PairQualityLive
	for i := 0; i < 6; i++ {
		ok := i != 0
		ttft := 100 + i
		live = ApplyPairQualityIngestWindows(live, 3, 20, ok, intPtr(ttft), nil)
	}
	require.Equal(t, 3, live.NTTFT)
	require.Equal(t, 20, live.NOK)
	require.Equal(t, 3, live.TTFTCount)
	require.Equal(t, 6, live.OKCount)
	require.Equal(t, 20, live.N)
}

func TestQualityGate_UsesSplitWindowN(t *testing.T) {
	t.Parallel()
	p50 := 200
	rate := 0.9
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:      &p50,
		QualityMinSuccessRate:    &rate,
		QualityMinTTFTSamples:    intPtr(3),
		QualityMinSuccessSamples: intPtr(20),
	}
	gate := policy.QualityGate()
	require.Equal(t, 3, gate.MinTTFTSamples)
	require.Equal(t, 20, gate.MinSuccessSamples)
	require.Equal(t, 20, policy.ProbeDesiredConcurrency())
	require.Equal(t, 20, policy.WindowN())

	live := ApplyPairQualityIngestWindows(nil, 3, 20, true, intPtr(400), nil)
	live = ApplyPairQualityIngestWindows(live, 3, 20, true, intPtr(400), nil)
	require.False(t, pairQualityProbeBlocks(live, policy), "ttft 2 < N首字=3 must not probe-cool")
	live = ApplyPairQualityIngestWindows(live, 3, 20, true, intPtr(400), nil)
	require.True(t, pairQualityProbeBlocks(live, policy), "ttft 3 >= N首字=3 p50 breach must probe-cool")
	require.True(t, pairQualityBlocks(live, policy), "composite off: 257 p50 uses N首字=3")

	okOnly := &SmartSchedulePlatformPolicy{
		QualityMinSuccessRate:    &rate,
		QualityMinTTFTSamples:    intPtr(3),
		QualityMinSuccessSamples: intPtr(20),
	}
	failLive := ApplyPairQualityIngestWindows(nil, 3, 20, false, nil, nil)
	for i := 0; i < 18; i++ {
		failLive = ApplyPairQualityIngestWindows(failLive, 3, 20, false, nil, nil)
	}
	require.Equal(t, 19, failLive.OKCount)
	require.False(t, pairQualityBlocks(failLive, okOnly), "19 < N成功率=20 must not cool")
	failLive = ApplyPairQualityIngestWindows(failLive, 3, 20, false, nil, nil)
	require.True(t, pairQualityBlocks(failLive, okOnly))
}

func TestPairQualityProbeGraduates_SplitN(t *testing.T) {
	t.Parallel()
	rate := 0.9
	p50 := 50
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:      &p50,
		QualityMinSuccessRate:    &rate,
		QualityMinTTFTSamples:    intPtr(10),
		QualityMinSuccessSamples: intPtr(5),
	}
	live := ApplyPairQualityIngestWindows(nil, 10, 5, true, intPtr(40), nil)
	live = ApplyPairQualityIngestWindows(live, 10, 5, true, intPtr(40), nil)
	live = ApplyPairQualityIngestWindows(live, 10, 5, true, intPtr(40), nil)
	live = ApplyPairQualityIngestWindows(live, 10, 5, true, nil, nil)
	require.Equal(t, 4, live.OKCount)
	require.False(t, pairQualityProbeGraduates(live, policy), "ok 4 < N成功率=5")
	live = ApplyPairQualityIngestWindows(live, 10, 5, true, nil, nil)
	require.Equal(t, 5, live.OKCount)
	require.True(t, pairQualityProbeGraduates(live, policy), "257: TTFT underfull still graduates when W_ok is full")

	require.False(t, pairQualityProbeGraduates(live, withProbeLatencyV2(policy)), "v2: TTFT window unfilled stays pending")
}

func buildLiveFromTTFTObservations(nTTFT, nOK int, ttftMs []int) *PairQualityLive {
	var live *PairQualityLive
	for _, v := range ttftMs {
		live = ApplyPairQualityIngestWindows(live, nTTFT, nOK, true, intPtr(v), nil)
	}
	return live
}

func latencyScatterSlow(slowCount, leadingFast int, fastMs, slowMs int) []int {
	out := repeatLatencyMs(fastMs, leadingFast)
	for i := 0; i < slowCount; i++ {
		out = append(out, slowMs, fastMs)
	}
	return out
}

func pureTTFTLatencyPolicy(accountID int64, probeNTTFT, probeNOK, gateMs int) *SmartSchedulePlatformPolicy {
	policy := enabledSmartPolicy(accountID, 0, intPtr(gateMs))
	policy.QualityMinTTFTSamples = intPtr(probeNTTFT)
	policy.QualityMinSuccessSamples = intPtr(probeNOK)
	policy.QualityCondition = strPtr(QualityHardCloseConditionOr)
	return policy
}

func TestApplyPairQualityIngest_FailuresOnlyOKWindow(t *testing.T) {
	t.Parallel()
	var live *PairQualityLive
	for i := 0; i < 3; i++ {
		live = ApplyPairQualityIngestWindows(live, 5, 5, false, intPtr(latencySlowMs), intPtr(latencyDurSlowMs))
	}
	require.Equal(t, 0, live.TTFTCount)
	require.Equal(t, 0, live.DurationCount)
	require.Equal(t, 3, live.OKCount)
	require.NotNil(t, live.SuccessRate)
	require.Equal(t, 0.0, *live.SuccessRate)
}

func TestApplyPairQualityIngest_StreamTTFTSkipsDuration(t *testing.T) {
	t.Parallel()
	live := ApplyPairQualityIngestWindows(nil, 5, 5, true, intPtr(latencyFastMs), intPtr(latencyDurSlowMs))
	require.Equal(t, 1, live.TTFTCount)
	require.Equal(t, 0, live.DurationCount)
	require.Equal(t, latencyFastMs, live.TTFTMs[0])
}

func TestApplyPairQualityIngest_NonStreamDurationOnly(t *testing.T) {
	t.Parallel()
	live := ApplyPairQualityIngestWindows(nil, 5, 5, true, nil, intPtr(latencyDurSlowMs))
	require.Equal(t, 0, live.TTFTCount)
	require.Equal(t, 1, live.DurationCount)
	require.Equal(t, latencyDurSlowMs, live.DurationMs[0])
}

func TestPairQualityProbeLatency_Table(t *testing.T) {
	t.Parallel()
	policy := withProbeLatencyV2(pureTTFTLatencyPolicy(7, latencyProbeN, latencyProbeN, latencyGateMs))
	cases := []struct {
		name      string
		ttft      []int
		wantBlock bool
		wantCode  string
	}{
		{
			name:      "probe_C_underfull_blocks",
			ttft:      []int{latencySlowMs, latencySlowMs},
			wantBlock: true,
			wantCode:  "ttft_consec",
		},
		{
			name:      "probe_underfull_no_block",
			ttft:      []int{latencySlowMs},
			wantBlock: false,
		},
		{
			name:      "probe_full_fast_pass",
			ttft:      repeatLatencyMs(latencyFastMs, latencyProbeN),
			wantBlock: false,
		},
		{
			name:      "probe_full_K_slow",
			ttft:      latencyScatterSlow(2, 3, latencyFastMs, latencySlowMs),
			wantBlock: true,
			wantCode:  "ttft_slow_k",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			live := buildLiveFromTTFTObservations(latencyProbeN, latencyProbeN, tc.ttft)
			blocked, reasons := pairQualityProbeLatencyBlocked(live, policy)
			require.Equal(t, tc.wantBlock, blocked)
			if tc.wantCode != "" {
				require.NotEmpty(t, reasons)
				require.Equal(t, tc.wantCode, reasons[0].Code)
			}
		})
	}
}

func TestPairQualitySelectableLatency_Table(t *testing.T) {
	t.Parallel()
	policy := withSchedComposite(pureTTFTLatencyPolicy(7, latencySchedN, latencySchedN, latencyGateMs))
	cases := []struct {
		name      string
		ttft      []int
		wantBlock bool
		wantCode  string
	}{
		{
			name:      "C_three_consecutive_only_three_samples",
			ttft:      repeatLatencyMs(latencySlowMs, 3),
			wantBlock: true,
			wantCode:  "ttft_consec",
		},
		{
			name:      "five_samples_two_slow_no_K",
			ttft:      latencyScatterSlow(2, 1, latencyFastMs, latencySlowMs),
			wantBlock: false,
		},
		{
			name:      "six_of_six_slow_C_and_K",
			ttft:      repeatLatencyMs(latencySlowMs, 6),
			wantBlock: true,
			wantCode:  "ttft_consec",
		},
		{
			name:      "six_slow_in_ten_rest_fast_K",
			ttft:      append(repeatLatencyMs(latencySlowMs, 6), repeatLatencyMs(latencyFastMs, 4)...),
			wantBlock: true,
			wantCode:  "ttft_slow_k",
		},
		{
			name:      "five_slow_in_nineteen_no_K_no_p50",
			ttft:      append(repeatLatencyMs(latencySlowMs, 5), repeatLatencyMs(latencyFastMs, 14)...),
			wantBlock: false,
		},
		{
			name:      "sched_jitter_two_consecutive_full_window",
			ttft:      latencyTail(18, 2, latencyFastMs, latencySlowMs),
			wantBlock: false,
		},
		{
			name:      "sched_C_three_consecutive_full_window",
			ttft:      latencyTail(17, 3, latencyFastMs, latencySlowMs),
			wantBlock: true,
			wantCode:  "ttft_consec",
		},
		{
			name:      "sched_p50_breach_full_window",
			ttft:      repeatLatencyMs(latencySlowMs, latencySchedN),
			wantBlock: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			live := buildLiveFromTTFTObservations(latencySchedN, latencySchedN, tc.ttft)
			blocked, reasons := pairQualitySelectableLatencyBlocked(live, policy)
			require.Equal(t, tc.wantBlock, blocked)
			if tc.wantCode != "" {
				require.NotEmpty(t, reasons)
				require.Equal(t, tc.wantCode, reasons[0].Code)
			}
		})
	}
}

func TestPairQualityDurationGate_IndependentWindow(t *testing.T) {
	t.Parallel()
	p50 := latencyGateMs
	durGate := latencyDurGateMs
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:      &p50,
		QualityMaxP50DurationMs:  &durGate,
		QualityMinTTFTSamples:    intPtr(latencyProbeN),
		QualityMinSuccessSamples: intPtr(latencyProbeN),
		QualityCondition:         strPtr(QualityHardCloseConditionOr),
	}
	var live *PairQualityLive
	for i := 0; i < latencyProbeN; i++ {
		live = ApplyPairQualityIngestWindows(live, latencyProbeN, latencyProbeN, true, intPtr(latencyFastMs), intPtr(latencyDurSlowMs))
	}
	require.Equal(t, latencyProbeN, live.TTFTCount)
	require.Equal(t, 0, live.DurationCount, "stream TTFT must not ingest duration")
	blocked, _ := pairQualityProbeLatencyBlocked(live, policy)
	require.False(t, blocked, "TTFT-only samples must not trip duration gate")

	live = nil
	for i := 0; i < latencyProbeN; i++ {
		live = ApplyPairQualityIngestWindows(live, latencyProbeN, latencyProbeN, true, nil, intPtr(latencyDurSlowMs))
	}
	blocked, reasons := pairQualityProbeLatencyBlocked(live, policy)
	require.True(t, blocked, "non-stream duration-only slow window must cool")
	require.NotEmpty(t, reasons)
	require.Equal(t, "dur_p50", reasons[0].Code, "v2 off: duration uses legacy p50 only")

	v2Policy := withProbeLatencyV2(policy)
	blocked, reasons = pairQualityProbeLatencyBlocked(live, v2Policy)
	require.True(t, blocked)
	require.Contains(t, []string{"dur_slow_k", "dur_consec"}, reasons[0].Code)
}

func TestPairQualitySelectable_P50WhenKAndCClosed(t *testing.T) {
	t.Parallel()
	policy := withSchedComposite(pureTTFTLatencyPolicy(7, latencySchedN, latencySchedN, latencyGateMs))
	policy.QualitySchedMaxSlowInWindow = intPtr(0)
	policy.QualitySchedMaxConsecutiveSlow = intPtr(0)
	live := buildLiveFromTTFTObservations(latencySchedN, latencySchedN,
		append(repeatLatencyMs(latencySlowMs, 11), repeatLatencyMs(latencyFastMs, 9)...))
	blocked, reasons := pairQualitySelectableLatencyBlocked(live, policy)
	require.True(t, blocked)
	require.Equal(t, "ttft_p50", reasons[0].Code)
}

func TestPairQualitySelectableBlocks_ZuogeSched10K3C2(t *testing.T) {
	t.Parallel()
	p50 := 10000
	policy := &SmartSchedulePlatformPolicy{
		QualityMaxP50TTFTMs:            &p50,
		QualityMinTTFTSamples:          intPtr(5),
		QualityMinSuccessSamples:       intPtr(50),
		QualitySchedWindowN:            intPtr(10),
		QualitySchedMaxSlowInWindow:    intPtr(3),
		QualitySchedMaxConsecutiveSlow: intPtr(2),
		QualityCondition:               strPtr(QualityHardCloseConditionOr),
	}
	require.True(t, policy.SchedCompositeEnabled())

	live := ApplyPairQualityIngestWindows(nil, 10, 50, true, intPtr(35850), nil)
	live = ApplyPairQualityIngestWindows(live, 10, 50, true, intPtr(42811), nil)
	blocked, reasons := pairQualitySelectableBlocksWithReasons(live, policy)
	require.True(t, blocked, "two consecutive first-token overruns must sched-cool on C=2")
	require.NotEmpty(t, reasons)
	require.Equal(t, "ttft_consec", reasons[0].Code)
	require.Contains(t, reasons[0].Detail, "连续C")
}

func TestPairQualitySelectable_CompositeOffUsesLegacyP50(t *testing.T) {
	t.Parallel()
	policy := pureTTFTLatencyPolicy(7, latencyProbeN, latencyProbeN, latencyGateMs)
	require.False(t, policy.SchedCompositeEnabled())

	threeSlow := buildLiveFromTTFTObservations(latencyProbeN, latencyProbeN, repeatLatencyMs(latencySlowMs, 3))
	blocked, _ := pairQualitySelectableLatencyBlocked(threeSlow, policy)
	require.False(t, blocked, "composite off: 3 consecutive slow is not enough for 257 p50 (N=5)")

	fullP50 := buildLiveFromTTFTObservations(latencyProbeN, latencyProbeN, repeatLatencyMs(latencySlowMs, latencyProbeN))
	blocked, reasons := pairQualitySelectableLatencyBlocked(fullP50, policy)
	require.True(t, blocked, "composite off: full N p50 breach cools")
	require.Equal(t, "ttft_p50", reasons[0].Code)
}

func TestPairQualityProbe_V2Off_UnderfullGraduatesNoHold(t *testing.T) {
	t.Parallel()
	durGate := latencyDurGateMs
	p50 := latencyGateMs
	policy := enabledSmartPolicy(7, 0, &p50)
	policy.QualityMaxP50DurationMs = &durGate
	policy.QualityMinTTFTSamples = intPtr(latencyProbeN)
	policy.QualityMinSuccessSamples = intPtr(latencyProbeN)
	policy.QualityCondition = strPtr(QualityHardCloseConditionOr)
	require.False(t, policy.ProbeLatencyV2)

	var live *PairQualityLive
	for i := 0; i < latencyProbeN; i++ {
		live = ApplyPairQualityIngestWindows(live, latencyProbeN, latencyProbeN, true, intPtr(latencyFastMs), nil)
	}
	for _, dur := range []int{latencyDurSlowMs, latencyFastMs * 50, latencyDurSlowMs} {
		live = ApplyPairQualityIngestWindows(live, latencyProbeN, latencyProbeN, true, nil, intPtr(dur))
	}
	pass, state := pairQualityProbeLatencyPass(live, policy)
	require.True(t, pass, "257: TTFT full+fast graduates; Hold must not apply")
	require.Equal(t, LatencyEvalPass, state)
	require.NotEqual(t, LatencyEvalHold, state)
}

func TestObservePairCompletion_FailuresDoNotTriggerLatencyCooldown(t *testing.T) {
	t.Parallel()
	policy := pureTTFTLatencyPolicy(7, latencyProbeN, latencyProbeN, latencyGateMs)
	cache := &observeCacheStub{
		bundle:  smartBundle(PlatformAnthropic, policy),
		live:    map[string]*PairQualityLive{},
		probing: map[string]bool{smartPairKey(7, 16): true},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	for i := 0; i < latencyProbeN; i++ {
		svc.ObservePairCompletion(context.Background(), PairQualityObservation{
			AccountID: 7, UserID: 16, Success: false,
			FirstTokenMs: intPtr(latencySlowMs), DurationMs: intPtr(latencyDurSlowMs),
		})
	}
	require.Equal(t, latencyProbeN, len(cache.ingested))
	require.Equal(t, 0, cache.starts, "failed requests must not latency-cool by themselves")
	live := cache.GetPairQuality(context.Background(), 7, 16, "openai")
	require.Equal(t, 0, live.TTFTCount)
	require.Equal(t, 0, live.DurationCount)
	require.Equal(t, latencyProbeN, live.OKCount)
}
