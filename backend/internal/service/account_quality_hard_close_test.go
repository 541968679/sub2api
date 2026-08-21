//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func qualityHardCloseCfg(mutate func(*QualityHardCloseSettings)) QualityHardCloseSettings {
	cfg := *DefaultQualityHardCloseSettings()
	cfg.Enabled = true
	if mutate != nil {
		mutate(&cfg)
	}
	return cfg
}

func qualityStats(success, errors, ttftSamples int64, p50 int, rate float64) *AccountQualityStats {
	p50Copy := p50
	rateCopy := rate
	return &AccountQualityStats{
		WindowSeconds: AccountQualityWindowSeconds,
		SuccessCount:  success,
		ErrorCount:    errors,
		SuccessRate:   &rateCopy,
		P50TTFTMs:     &p50Copy,
		TTFTSamples:   ttftSamples,
	}
}

func TestEvaluateAccountQualityHardClose_DisabledResolvedDoesNotPause(t *testing.T) {
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.Enabled = false
	})
	stats := qualityStats(1, 20, 20, 9000, 0.05)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.False(t, shouldPause)
	require.Empty(t, reason)
}

func TestEvaluateAccountQualityHardClose_SkipAlreadyPaused(t *testing.T) {
	cfg := qualityHardCloseCfg(nil)
	stats := qualityStats(1, 20, 20, 9000, 0.05)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, true)
	require.False(t, shouldPause)
	require.Empty(t, reason)
}

func TestEvaluateAccountQualityHardClose_MinSamplesNotJudged(t *testing.T) {
	cfg := qualityHardCloseCfg(nil)
	stats := qualityStats(1, 1, 2, 9000, 0.5)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.False(t, shouldPause)
	require.Empty(t, reason)
}

func TestEvaluateAccountQualityHardClose_UnconfiguredMetricIgnored(t *testing.T) {
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.MaxP50TTFTMs = nil
		cfg.MinSuccessSamples = 5
	})
	stats := qualityStats(10, 0, 20, 9000, 1)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.False(t, shouldPause)
	require.Empty(t, reason)
}

func TestEvaluateAccountQualityHardClose_BothMetricsUnconfigured(t *testing.T) {
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.MaxP50TTFTMs = nil
		cfg.MinSuccessRate = nil
	})
	stats := qualityStats(1, 20, 20, 9000, 0.05)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.False(t, shouldPause)
	require.Empty(t, reason)
}

func TestEvaluateAccountQualityHardClose_OrAnyBreach(t *testing.T) {
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.Condition = QualityHardCloseConditionOr
	})
	stats := qualityStats(20, 0, 20, 3200, 1)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.True(t, shouldPause)
	require.True(t, strings.HasPrefix(reason, QualityHardCloseReasonPrefix))
	require.Contains(t, reason, "p50=3200")
}

func TestEvaluateAccountQualityHardClose_OrSuccessBreach(t *testing.T) {
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.Condition = QualityHardCloseConditionOr
	})
	stats := qualityStats(16, 4, 10, 800, 0.8)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.True(t, shouldPause)
	require.Contains(t, reason, "success=0.8")
}

func TestEvaluateAccountQualityHardClose_OrNoBreach(t *testing.T) {
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.Condition = QualityHardCloseConditionOr
	})
	stats := qualityStats(19, 1, 10, 800, 0.95)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.False(t, shouldPause)
	require.Empty(t, reason)
}

func TestEvaluateAccountQualityHardClose_AndOneJudgedEqualsOr(t *testing.T) {
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.Condition = QualityHardCloseConditionAnd
		cfg.MinSuccessRate = nil
	})
	stats := qualityStats(1, 0, 20, 3200, 1)
	shouldPause, reason := EvaluateAccountQualityHardClose(stats, cfg, false)
	require.True(t, shouldPause)
	require.True(t, strings.HasPrefix(reason, QualityHardCloseReasonPrefix+":"))
}

func TestEvaluateAccountQualityHardClose_AndRequiresAllJudgedBreaches(t *testing.T) {
	cfg := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.Condition = QualityHardCloseConditionAnd
	})
	onlyP50 := qualityStats(20, 0, 20, 3200, 1)
	shouldPause, reason := EvaluateAccountQualityHardClose(onlyP50, cfg, false)
	require.False(t, shouldPause)
	require.Empty(t, reason)

	both := qualityStats(16, 4, 20, 3200, 0.8)
	shouldPause, reason = EvaluateAccountQualityHardClose(both, cfg, false)
	require.True(t, shouldPause)
	require.Equal(t, "quality_hard_close:p50=3200,success=0.8", reason)
}

func TestClampQualityHardClosePauseMinutes(t *testing.T) {
	require.Equal(t, DefaultQualityHardClosePauseMinutes, clampQualityHardClosePauseMinutes(0))
	require.Equal(t, QualityHardCloseMaxPauseMinutes, clampQualityHardClosePauseMinutes(2000))
	require.Equal(t, 45, clampQualityHardClosePauseMinutes(45))
}

func TestEvaluateAccountQualityHardClose_NilStats(t *testing.T) {
	shouldPause, reason := EvaluateAccountQualityHardClose(nil, qualityHardCloseCfg(nil), false)
	require.False(t, shouldPause)
	require.Empty(t, reason)
}

func TestParseAccountQualityHardCloseOverlay_Defaults(t *testing.T) {
	overlay := ParseAccountQualityHardCloseOverlay(nil)
	require.False(t, overlay.Enabled)
	require.True(t, overlay.UseGlobal)

	overlay = ParseAccountQualityHardCloseOverlay(map[string]any{
		AccountExtraQualityHardClose: map[string]any{"enabled": true},
	})
	require.True(t, overlay.Enabled)
	require.True(t, overlay.UseGlobal)
}

func TestParseAccountQualityHardCloseOverlay_UseGlobalFalse(t *testing.T) {
	overlay := ParseAccountQualityHardCloseOverlay(map[string]any{
		AccountExtraQualityHardClose: map[string]any{
			"enabled":    true,
			"use_global": false,
		},
	})
	require.True(t, overlay.Enabled)
	require.False(t, overlay.UseGlobal)
}

func TestResolveAccountQualityHardClose_UseGlobalIgnoresOverrides(t *testing.T) {
	global := qualityHardCloseCfg(nil)
	pause := 5
	overlay := AccountQualityHardCloseOverlay{
		Enabled:      true,
		UseGlobal:    true,
		PauseMinutes: &pause,
	}
	resolved := ResolveAccountQualityHardClose(global, overlay)
	require.True(t, resolved.Enabled)
	require.Equal(t, DefaultQualityHardClosePauseMinutes, resolved.PauseMinutes)
}

func TestResolveAccountQualityHardClose_OverlayNDoesNotChangeWindow(t *testing.T) {
	global := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		n := 20
		cfg.AccountQualityWindowN = &n
		echoQualityHardCloseWindowN(cfg)
	})
	overlayN := 7
	overlaySuccess := 3
	overlayTTFT := 4
	overlay := AccountQualityHardCloseOverlay{
		Enabled:               true,
		UseGlobal:             false,
		AccountQualityWindowN: &overlayN,
		MinSuccessSamples:     &overlaySuccess,
		MinTTFTSamples:        &overlayTTFT,
	}
	resolved := ResolveAccountQualityHardClose(global, overlay)
	require.Equal(t, 20, resolved.ResolvedWindowN())
	require.Equal(t, 20, resolved.MinSuccessSamples)
	require.Equal(t, 20, resolved.MinTTFTSamples)
	require.NotNil(t, resolved.AccountQualityWindowN)
	require.Equal(t, 20, *resolved.AccountQualityWindowN)
	require.NotNil(t, resolved.WindowN)
	require.Equal(t, 20, *resolved.WindowN)
	require.NotNil(t, resolved.N)
	require.Equal(t, 20, *resolved.N)
}

func TestResolveAccountQualityHardClose_OverlayOverridesNullFallback(t *testing.T) {
	global := qualityHardCloseCfg(nil)
	p50 := 1500
	overlay := AccountQualityHardCloseOverlay{
		Enabled:      true,
		UseGlobal:    false,
		MaxP50TTFTMs: &p50,
	}
	resolved := ResolveAccountQualityHardClose(global, overlay)
	require.Equal(t, 1500, *resolved.MaxP50TTFTMs)
	require.InDelta(t, DefaultQualityHardCloseMinSuccessRate, *resolved.MinSuccessRate, 0.0001)
	require.Equal(t, DefaultQualityHardClosePauseMinutes, resolved.PauseMinutes)
}

func TestResolveAccountQualityHardClose_BothLayersRequired(t *testing.T) {
	global := qualityHardCloseCfg(func(cfg *QualityHardCloseSettings) {
		cfg.Enabled = false
	})
	overlay := AccountQualityHardCloseOverlay{Enabled: true, UseGlobal: true}
	resolved := ResolveAccountQualityHardClose(global, overlay)
	require.False(t, resolved.Enabled)

	global.Enabled = true
	overlay.Enabled = false
	resolved = ResolveAccountQualityHardClose(global, overlay)
	require.False(t, resolved.Enabled)
}

func TestQualityHardCloseSettings_NotExposedOnPublicSettings(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyQualityHardCloseSettings: `{"enabled":true,"pause_minutes":30}`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "quality_hard_close")
}

func TestGetQualityHardCloseSettings_DefaultsWhenNotSet(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	settings, err := svc.GetQualityHardCloseSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, 3000, *settings.MaxP50TTFTMs)
	require.InDelta(t, 0.9, *settings.MinSuccessRate, 0.0001)
	require.Equal(t, 30, settings.PauseMinutes)
	require.Equal(t, 20, settings.MinSuccessSamples)
	require.Equal(t, 20, settings.MinTTFTSamples)
	require.Equal(t, 20, settings.ResolvedWindowN())
	require.NotNil(t, settings.AccountQualityWindowN)
	require.Equal(t, 20, *settings.AccountQualityWindowN)
	require.Equal(t, QualityHardCloseConditionOr, settings.Condition)
	require.False(t, settings.ScheduleUseFailoverErrorRate)
}

func TestSetQualityHardCloseSettings_RoundTripAndValidation(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	p50 := 2500
	rate := 0.85
	in := &QualityHardCloseSettings{
		Enabled:           true,
		MaxP50TTFTMs:      &p50,
		MinSuccessRate:    &rate,
		PauseMinutes:      45,
		MinSuccessSamples: 15,
		MinTTFTSamples:    8,
		Condition:         QualityHardCloseConditionAnd,
	}
	require.NoError(t, svc.SetQualityHardCloseSettings(context.Background(), in))

	out, err := svc.GetQualityHardCloseSettings(context.Background())
	require.NoError(t, err)
	require.True(t, out.Enabled)
	require.Equal(t, 2500, *out.MaxP50TTFTMs)
	require.InDelta(t, 0.85, *out.MinSuccessRate, 0.0001)
	require.Equal(t, 45, out.PauseMinutes)
	require.Equal(t, QualityHardCloseConditionAnd, out.Condition)
	require.Equal(t, 15, out.MinSuccessSamples)
	require.Equal(t, 15, out.MinTTFTSamples)
	require.Equal(t, 15, out.ResolvedWindowN())

	require.Error(t, svc.SetQualityHardCloseSettings(context.Background(), nil))
	require.Error(t, svc.SetQualityHardCloseSettings(context.Background(), &QualityHardCloseSettings{
		PauseMinutes:      0,
		MinSuccessSamples: 1,
		MinTTFTSamples:    1,
		Condition:         QualityHardCloseConditionOr,
	}))
	badRate := 1.2
	require.Error(t, svc.SetQualityHardCloseSettings(context.Background(), &QualityHardCloseSettings{
		PauseMinutes:      30,
		MinSuccessSamples: 1,
		MinTTFTSamples:    1,
		MinSuccessRate:    &badRate,
		Condition:         QualityHardCloseConditionOr,
	}))
}

func TestGetQualityHardCloseSettings_InvalidJSONReturnsDefaults(t *testing.T) {
	repo := newMockSettingRepo()
	repo.data[SettingKeyQualityHardCloseSettings] = "not-json"
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetQualityHardCloseSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, 30, settings.PauseMinutes)
}

func TestSetQualityHardCloseSettings_AllowsNullMetrics(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	require.NoError(t, svc.SetQualityHardCloseSettings(context.Background(), &QualityHardCloseSettings{
		Enabled:           false,
		MaxP50TTFTMs:      nil,
		MinSuccessRate:    nil,
		PauseMinutes:      30,
		MinSuccessSamples: 20,
		MinTTFTSamples:    10,
		Condition:         QualityHardCloseConditionOr,
	}))
	out, err := svc.GetQualityHardCloseSettings(context.Background())
	require.NoError(t, err)
	require.Nil(t, out.MaxP50TTFTMs)
	require.Nil(t, out.MinSuccessRate)
	require.Equal(t, 20, out.MinSuccessSamples)
	require.Equal(t, 20, out.MinTTFTSamples)
	require.Equal(t, 20, out.ResolvedWindowN())
}

type hardCloseAccountRepoStub struct {
	accounts map[int64]*Account
	pauses   []hardClosePauseCall
}

type hardClosePauseCall struct {
	id     int64
	until  time.Time
	reason string
}

func (r *hardCloseAccountRepoStub) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		if acc, ok := r.accounts[id]; ok {
			out = append(out, acc)
		}
	}
	return out, nil
}

func (r *hardCloseAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.pauses = append(r.pauses, hardClosePauseCall{id: id, until: until, reason: reason})
	return nil
}

func newHardCloseEval(t *testing.T, repo *hardCloseAccountRepoStub, global QualityHardCloseSettings) *AccountQualityHardCloseService {
	t.Helper()
	settingRepo := newMockSettingRepo()
	data, err := json.Marshal(global)
	require.NoError(t, err)
	settingRepo.data[SettingKeyQualityHardCloseSettings] = string(data)
	eval := NewAccountQualityHardCloseEvaluator(repo, NewSettingService(settingRepo, &config.Config{}))
	eval.now = func() time.Time {
		return time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	}
	return eval
}

func TestAccountQualityHardCloseEvaluator_GlobalOffNoPause(t *testing.T) {
	repo := &hardCloseAccountRepoStub{
		accounts: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{AccountExtraQualityHardClose: map[string]any{"enabled": true}}},
		},
	}
	eval := newHardCloseEval(t, repo, *DefaultQualityHardCloseSettings())
	eval.EvaluateHardClose(context.Background(), map[int64]*AccountQualityStats{
		1: qualityStats(1, 20, 20, 9000, 0.05),
	})
	require.Empty(t, repo.pauses)
}

func TestAccountQualityHardCloseEvaluator_AccountOffNoPause(t *testing.T) {
	global := qualityHardCloseCfg(nil)
	repo := &hardCloseAccountRepoStub{
		accounts: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{}},
		},
	}
	eval := newHardCloseEval(t, repo, global)
	eval.EvaluateHardClose(context.Background(), map[int64]*AccountQualityStats{
		1: qualityStats(1, 20, 20, 9000, 0.05),
	})
	require.Empty(t, repo.pauses)
}

func TestAccountQualityHardCloseEvaluator_ManualResumeSkipsSetTempUnschedulable(t *testing.T) {
	global := qualityHardCloseCfg(nil)
	repo := &hardCloseAccountRepoStub{
		accounts: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{AccountExtraQualityHardClose: map[string]any{"enabled": true}}},
		},
	}
	eval := newHardCloseEval(t, repo, global)
	stats := qualityStats(1, 20, 20, 9000, 0.05)
	SetAccountQualityResume(stats, time.Date(2026, 8, 14, 12, 10, 0, 0, time.UTC))
	eval.EvaluateHardClose(context.Background(), map[int64]*AccountQualityStats{1: stats})
	require.Empty(t, repo.pauses)
}

func TestAccountQualityHardCloseEvaluator_CooldownOnceSkipsSetTempUnschedulable(t *testing.T) {
	until := time.Date(2026, 8, 14, 12, 20, 0, 0, time.UTC)
	global := qualityHardCloseCfg(nil)
	repo := &hardCloseAccountRepoStub{
		accounts: map[int64]*Account{
			1: {
				ID:                     1,
				TempUnschedulableUntil: &until,
				Extra:                  map[string]any{AccountExtraQualityHardClose: map[string]any{"enabled": true}},
			},
		},
	}
	eval := newHardCloseEval(t, repo, global)
	eval.EvaluateHardClose(context.Background(), map[int64]*AccountQualityStats{
		1: qualityStats(1, 20, 20, 9000, 0.05),
	})
	require.Empty(t, repo.pauses)
}

func TestAccountQualityHardCloseEvaluator_ExpiredCooldownCanPause(t *testing.T) {
	until := time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC)
	global := qualityHardCloseCfg(nil)
	repo := &hardCloseAccountRepoStub{
		accounts: map[int64]*Account{
			1: {
				ID:                     1,
				TempUnschedulableUntil: &until,
				Extra:                  map[string]any{AccountExtraQualityHardClose: map[string]any{"enabled": true}},
			},
		},
	}
	eval := newHardCloseEval(t, repo, global)
	eval.EvaluateHardClose(context.Background(), map[int64]*AccountQualityStats{
		1: qualityStats(1, 20, 20, 9000, 0.05),
	})
	require.Len(t, repo.pauses, 1)
	require.Equal(t, int64(1), repo.pauses[0].id)
	require.True(t, strings.HasPrefix(repo.pauses[0].reason, QualityHardCloseReasonPrefix))
}

func TestUpdateAccount_PreservesQualityHardCloseOverlay(t *testing.T) {
	overlay := map[string]any{"enabled": true, "use_global": true}
	repo := &updateAccountOveragesRepoStub{
		account: &Account{
			ID:       12,
			Platform: PlatformAnthropic,
			Type:     AccountTypeAPIKey,
			Status:   StatusActive,
			Extra: map[string]any{
				AccountExtraQualityHardClose: overlay,
				"quota_limit":                100.0,
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}
	updated, err := svc.UpdateAccount(context.Background(), 12, &UpdateAccountInput{
		Extra: map[string]any{},
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, overlay, updated.Extra[AccountExtraQualityHardClose])
	require.NotContains(t, updated.Extra, "quota_limit")
}

func TestAccountQualityHardCloseEvaluator_OptedInBreachPauses(t *testing.T) {
	global := qualityHardCloseCfg(nil)
	repo := &hardCloseAccountRepoStub{
		accounts: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{AccountExtraQualityHardClose: map[string]any{"enabled": true}}},
			2: {ID: 2, Extra: map[string]any{}},
		},
	}
	eval := newHardCloseEval(t, repo, global)
	eval.EvaluateHardClose(context.Background(), map[int64]*AccountQualityStats{
		1: qualityStats(16, 4, 10, 3200, 0.8),
		2: qualityStats(16, 4, 10, 3200, 0.8),
	})
	require.Len(t, repo.pauses, 1)
	require.Equal(t, int64(1), repo.pauses[0].id)
	require.True(t, strings.HasPrefix(repo.pauses[0].reason, QualityHardCloseReasonPrefix))
	require.Equal(t, time.Date(2026, 8, 14, 12, 30, 0, 0, time.UTC), repo.pauses[0].until)
}
