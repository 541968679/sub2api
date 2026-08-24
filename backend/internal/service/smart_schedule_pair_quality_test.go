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

func TestPairQualityBlocks_UnderNAndOr(t *testing.T) {
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
	require.False(t, pairQualityBlocks(live, policy), "W_ttft count 1 < N must not cool")

	live = ApplyPairQualityIngest(live, 3, true, intPtr(400))
	live = ApplyPairQualityIngest(live, 3, true, intPtr(400))
	require.True(t, pairQualityBlocks(live, policy), "full W_ttft p50 breach must cool on or")

	andPolicy := *policy
	and := QualityHardCloseConditionAnd
	andPolicy.QualityCondition = &and
	require.False(t, pairQualityBlocks(live, &andPolicy), "and: success window is full of successes so success metric is not breached")
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
	bundle      *UserSmartScheduleBundle
	live        map[string]*PairQualityLive
	cooling     map[string]bool
	probing     map[string]bool
	pinned      map[string]bool
	resumeUntil map[string]int64
	ingested    []PairQualityObservation
	starts      int
	graduated   int
}

func (s *observeCacheStub) Lookup(context.Context, int64) *UserSmartScheduleBundle { return s.bundle }
func (s *observeCacheStub) CooldownActive(_ context.Context, accountID, userID int64, _ string, _ time.Time) bool {
	return s.cooling[smartPairKey(accountID, userID)]
}
func (s *observeCacheStub) StartCooldown(context.Context, int64, int64, string, int, time.Time) {
	s.starts++
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
func (s *observeCacheStub) IngestPairQuality(_ context.Context, accountID, userID int64, _ string, nTTFT, nOK int, success bool, firstTokenMs *int) *PairQualityLive {
	s.ingested = append(s.ingested, PairQualityObservation{AccountID: accountID, UserID: userID, Success: success, FirstTokenMs: firstTokenMs})
	key := smartPairKey(accountID, userID)
	if s.live == nil {
		s.live = map[string]*PairQualityLive{}
	}
	s.live[key] = ApplyPairQualityIngestWindows(s.live[key], nTTFT, nOK, success, firstTokenMs)
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
		live = ApplyPairQualityIngestWindows(live, 3, 20, ok, intPtr(ttft))
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

	live := ApplyPairQualityIngestWindows(nil, 3, 20, true, intPtr(400))
	live = ApplyPairQualityIngestWindows(live, 3, 20, true, intPtr(400))
	require.False(t, pairQualityBlocks(live, policy), "ttft 2 < N首字=3 must not cool")
	live = ApplyPairQualityIngestWindows(live, 3, 20, true, intPtr(400))
	require.True(t, pairQualityBlocks(live, policy), "ttft 3 >= N首字=3 p50 breach must cool")

	okOnly := &SmartSchedulePlatformPolicy{
		QualityMinSuccessRate:    &rate,
		QualityMinTTFTSamples:    intPtr(3),
		QualityMinSuccessSamples: intPtr(20),
	}
	failLive := ApplyPairQualityIngestWindows(nil, 3, 20, false, nil)
	for i := 0; i < 18; i++ {
		failLive = ApplyPairQualityIngestWindows(failLive, 3, 20, false, nil)
	}
	require.Equal(t, 19, failLive.OKCount)
	require.False(t, pairQualityBlocks(failLive, okOnly), "19 < N成功率=20 must not cool")
	failLive = ApplyPairQualityIngestWindows(failLive, 3, 20, false, nil)
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
	live := ApplyPairQualityIngestWindows(nil, 10, 5, true, intPtr(40))
	live = ApplyPairQualityIngestWindows(live, 10, 5, true, intPtr(40))
	live = ApplyPairQualityIngestWindows(live, 10, 5, true, intPtr(40))
	live = ApplyPairQualityIngestWindows(live, 10, 5, true, nil)
	require.Equal(t, 4, live.OKCount)
	require.False(t, pairQualityProbeGraduates(live, policy), "ok 4 < N成功率=5")
	live = ApplyPairQualityIngestWindows(live, 10, 5, true, nil)
	require.Equal(t, 3, live.TTFTCount)
	require.True(t, pairQualityProbeGraduates(live, policy), "ok full and ttft under N首字 still graduates")
}
