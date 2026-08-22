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
	bundle    *UserSmartScheduleBundle
	live      map[string]*PairQualityLive
	cooling   map[string]bool
	probing   map[string]bool
	pinned    map[string]bool
	ingested  []PairQualityObservation
	starts    int
	graduated int
}

func (s *observeCacheStub) Lookup(context.Context, int64) *UserSmartScheduleBundle { return s.bundle }
func (s *observeCacheStub) CooldownActive(_ context.Context, accountID, userID int64, _ time.Time) bool {
	return s.cooling[smartPairKey(accountID, userID)]
}
func (s *observeCacheStub) StartCooldown(context.Context, int64, int64, int, time.Time) {
	s.starts++
}
func (s *observeCacheStub) GetPairQuality(_ context.Context, accountID, userID int64) *PairQualityLive {
	return s.live[smartPairKey(accountID, userID)]
}
func (s *observeCacheStub) IngestPairQuality(_ context.Context, accountID, userID int64, n int, success bool, firstTokenMs *int) *PairQualityLive {
	s.ingested = append(s.ingested, PairQualityObservation{AccountID: accountID, UserID: userID, Success: success, FirstTokenMs: firstTokenMs})
	key := smartPairKey(accountID, userID)
	if s.live == nil {
		s.live = map[string]*PairQualityLive{}
	}
	s.live[key] = ApplyPairQualityIngest(s.live[key], n, success, firstTokenMs)
	return s.live[key]
}

func (s *observeCacheStub) IsProbing(_ context.Context, accountID, userID int64) bool {
	return s.probing[smartPairKey(accountID, userID)]
}

func (s *observeCacheStub) MarkProbing(_ context.Context, accountID, userID int64) {
	if s.probing == nil {
		s.probing = map[string]bool{}
	}
	s.probing[smartPairKey(accountID, userID)] = true
}

func (s *observeCacheStub) ClearProbing(_ context.Context, accountID, userID int64) {
	delete(s.probing, smartPairKey(accountID, userID))
}

func (s *observeCacheStub) GraduateProbing(ctx context.Context, accountID, userID int64) {
	if s.IsProbing(ctx, accountID, userID) {
		s.graduated++
	}
	s.ClearProbing(ctx, accountID, userID)
}

func (s *observeCacheStub) IsPinned(_ context.Context, accountID, userID int64) bool {
	return s.pinned[smartPairKey(accountID, userID)]
}

func (s *observeCacheStub) MarkPinned(_ context.Context, accountID, userID int64) {
	if s.pinned == nil {
		s.pinned = map[string]bool{}
	}
	s.pinned[smartPairKey(accountID, userID)] = true
}

func (s *observeCacheStub) ClearPinned(_ context.Context, accountID, userID int64) {
	delete(s.pinned, smartPairKey(accountID, userID))
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
	require.Nil(t, cache.GetPairQuality(context.Background(), 9, 16))
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
	require.Equal(t, 4, *got.QualityWindowSamples)
	require.Equal(t, 4, *got.QualityMinSuccessSamples)
	require.Equal(t, 4, *got.QualityMinTTFTSamples)

	_, err = normalizeSmartScheduleWrite(SmartSchedulePlatformWrite{
		QualityMaxP50TTFTMs:  &p50,
		QualityWindowSamples: intPtr(0),
		CooldownMinutes:      15,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "quality_window_samples")
}
