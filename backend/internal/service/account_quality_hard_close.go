package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// AccountExtraQualityHardClose is the accounts.extra key for the per-account overlay.
	AccountExtraQualityHardClose = "quality_hard_close"

	QualityHardCloseReasonPrefix = "quality_hard_close"

	QualityHardCloseConditionOr  = "or"
	QualityHardCloseConditionAnd = "and"

	QualityHardCloseMinPauseMinutes = 1
	QualityHardCloseMaxPauseMinutes = 1440

	DefaultQualityHardCloseMaxP50TTFTMs      = 3000
	DefaultQualityHardCloseMinSuccessRate    = 0.9
	DefaultQualityHardClosePauseMinutes      = 30
	DefaultQualityHardCloseMinSuccessSamples = 20
	DefaultQualityHardCloseMinTTFTSamples    = 10
)

// QualityHardCloseSettings is the global Settings KV template plus master switch.
// Null MaxP50TTFTMs / MinSuccessRate means that metric is not configured.
type QualityHardCloseSettings struct {
	Enabled           bool     `json:"enabled"`
	MaxP50TTFTMs      *int     `json:"max_p50_ttft_ms"`
	MinSuccessRate    *float64 `json:"min_success_rate"`
	PauseMinutes      int      `json:"pause_minutes"`
	MinSuccessSamples int      `json:"min_success_samples"`
	MinTTFTSamples    int      `json:"min_ttft_samples"`
	Condition         string   `json:"condition"`
	// ScheduleUseFailoverErrorRate, when true, uses the Recovered-inclusive
	// account error caliber as ErrorCount for hard-close and smart-schedule.
	// Default false: keep the current client status>=400 caliber.
	ScheduleUseFailoverErrorRate bool `json:"schedule_use_failover_error_rate"`
}

// AccountQualityHardCloseOverlay is stored in extra.quality_hard_close.
// Missing use_global defaults to true.
type AccountQualityHardCloseOverlay struct {
	Enabled           bool     `json:"enabled"`
	UseGlobal         bool     `json:"use_global"`
	MaxP50TTFTMs      *int     `json:"max_p50_ttft_ms"`
	MinSuccessRate    *float64 `json:"min_success_rate"`
	PauseMinutes      *int     `json:"pause_minutes"`
	MinSuccessSamples *int     `json:"min_success_samples"`
	MinTTFTSamples    *int     `json:"min_ttft_samples"`
	Condition         *string  `json:"condition"`
}

// AccountQualityHardCloseView is the admin GET/PUT account response.
type AccountQualityHardCloseView struct {
	Overlay       AccountQualityHardCloseOverlay `json:"overlay"`
	Resolved      QualityHardCloseSettings       `json:"resolved"`
	GlobalEnabled bool                           `json:"global_enabled"`
}

func (o *AccountQualityHardCloseOverlay) UnmarshalJSON(data []byte) error {
	aux := struct {
		Enabled           bool     `json:"enabled"`
		UseGlobal         *bool    `json:"use_global"`
		MaxP50TTFTMs      *int     `json:"max_p50_ttft_ms"`
		MinSuccessRate    *float64 `json:"min_success_rate"`
		PauseMinutes      *int     `json:"pause_minutes"`
		MinSuccessSamples *int     `json:"min_success_samples"`
		MinTTFTSamples    *int     `json:"min_ttft_samples"`
		Condition         *string  `json:"condition"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	o.Enabled = aux.Enabled
	o.UseGlobal = aux.UseGlobal == nil || *aux.UseGlobal
	o.MaxP50TTFTMs = aux.MaxP50TTFTMs
	o.MinSuccessRate = aux.MinSuccessRate
	o.PauseMinutes = aux.PauseMinutes
	o.MinSuccessSamples = aux.MinSuccessSamples
	o.MinTTFTSamples = aux.MinTTFTSamples
	o.Condition = aux.Condition
	return nil
}

// DefaultQualityHardCloseSettings returns the off-by-default global template.
func DefaultQualityHardCloseSettings() *QualityHardCloseSettings {
	p50 := DefaultQualityHardCloseMaxP50TTFTMs
	rate := DefaultQualityHardCloseMinSuccessRate
	return &QualityHardCloseSettings{
		Enabled:           false,
		MaxP50TTFTMs:      &p50,
		MinSuccessRate:    &rate,
		PauseMinutes:      DefaultQualityHardClosePauseMinutes,
		MinSuccessSamples: DefaultQualityHardCloseMinSuccessSamples,
		MinTTFTSamples:    DefaultQualityHardCloseMinTTFTSamples,
		Condition:         QualityHardCloseConditionOr,
	}
}

// DefaultAccountQualityHardCloseOverlay returns the off-by-default account overlay.
func DefaultAccountQualityHardCloseOverlay() AccountQualityHardCloseOverlay {
	return AccountQualityHardCloseOverlay{
		Enabled:   false,
		UseGlobal: true,
	}
}

func ValidateQualityHardCloseSettings(settings *QualityHardCloseSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	if settings.PauseMinutes < QualityHardCloseMinPauseMinutes || settings.PauseMinutes > QualityHardCloseMaxPauseMinutes {
		return fmt.Errorf("pause_minutes must be between %d-%d", QualityHardCloseMinPauseMinutes, QualityHardCloseMaxPauseMinutes)
	}
	if settings.MinSuccessSamples < 1 {
		return fmt.Errorf("min_success_samples must be >= 1")
	}
	if settings.MinTTFTSamples < 1 {
		return fmt.Errorf("min_ttft_samples must be >= 1")
	}
	if settings.MaxP50TTFTMs != nil && *settings.MaxP50TTFTMs < 1 {
		return fmt.Errorf("max_p50_ttft_ms must be >= 1")
	}
	if settings.MinSuccessRate != nil && (*settings.MinSuccessRate <= 0 || *settings.MinSuccessRate > 1) {
		return fmt.Errorf("min_success_rate must be in (0,1]")
	}
	cond := strings.ToLower(strings.TrimSpace(settings.Condition))
	if cond == "" {
		cond = QualityHardCloseConditionOr
	}
	if cond != QualityHardCloseConditionOr && cond != QualityHardCloseConditionAnd {
		return fmt.Errorf("condition must be %q or %q", QualityHardCloseConditionOr, QualityHardCloseConditionAnd)
	}
	settings.Condition = cond
	return nil
}

func ValidateAccountQualityHardCloseOverlay(overlay *AccountQualityHardCloseOverlay) error {
	if overlay == nil {
		return fmt.Errorf("overlay cannot be nil")
	}
	if overlay.PauseMinutes != nil {
		if *overlay.PauseMinutes < QualityHardCloseMinPauseMinutes || *overlay.PauseMinutes > QualityHardCloseMaxPauseMinutes {
			return fmt.Errorf("pause_minutes must be between %d-%d", QualityHardCloseMinPauseMinutes, QualityHardCloseMaxPauseMinutes)
		}
	}
	if overlay.MinSuccessSamples != nil && *overlay.MinSuccessSamples < 1 {
		return fmt.Errorf("min_success_samples must be >= 1")
	}
	if overlay.MinTTFTSamples != nil && *overlay.MinTTFTSamples < 1 {
		return fmt.Errorf("min_ttft_samples must be >= 1")
	}
	if overlay.MaxP50TTFTMs != nil && *overlay.MaxP50TTFTMs < 1 {
		return fmt.Errorf("max_p50_ttft_ms must be >= 1")
	}
	if overlay.MinSuccessRate != nil && (*overlay.MinSuccessRate <= 0 || *overlay.MinSuccessRate > 1) {
		return fmt.Errorf("min_success_rate must be in (0,1]")
	}
	if overlay.Condition != nil {
		cond := strings.ToLower(strings.TrimSpace(*overlay.Condition))
		if cond == "" {
			overlay.Condition = nil
		} else if cond != QualityHardCloseConditionOr && cond != QualityHardCloseConditionAnd {
			return fmt.Errorf("condition must be %q or %q", QualityHardCloseConditionOr, QualityHardCloseConditionAnd)
		} else {
			overlay.Condition = &cond
		}
	}
	return nil
}

func normalizeQualityHardCloseSettings(settings *QualityHardCloseSettings) {
	if settings == nil {
		return
	}
	if settings.PauseMinutes < QualityHardCloseMinPauseMinutes {
		settings.PauseMinutes = DefaultQualityHardClosePauseMinutes
	}
	if settings.PauseMinutes > QualityHardCloseMaxPauseMinutes {
		settings.PauseMinutes = QualityHardCloseMaxPauseMinutes
	}
	if settings.MinSuccessSamples < 1 {
		settings.MinSuccessSamples = DefaultQualityHardCloseMinSuccessSamples
	}
	if settings.MinTTFTSamples < 1 {
		settings.MinTTFTSamples = DefaultQualityHardCloseMinTTFTSamples
	}
	cond := strings.ToLower(strings.TrimSpace(settings.Condition))
	if cond != QualityHardCloseConditionOr && cond != QualityHardCloseConditionAnd {
		settings.Condition = QualityHardCloseConditionOr
	} else {
		settings.Condition = cond
	}
	if settings.MaxP50TTFTMs != nil && *settings.MaxP50TTFTMs < 1 {
		settings.MaxP50TTFTMs = nil
	}
	if settings.MinSuccessRate != nil && (*settings.MinSuccessRate <= 0 || *settings.MinSuccessRate > 1) {
		settings.MinSuccessRate = nil
	}
}

// GetQualityHardCloseSettings returns the global template. Missing/invalid JSON yields defaults.
func (s *SettingService) GetQualityHardCloseSettings(ctx context.Context) (*QualityHardCloseSettings, error) {
	if s == nil || s.settingRepo == nil {
		return DefaultQualityHardCloseSettings(), nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyQualityHardCloseSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultQualityHardCloseSettings(), nil
		}
		return nil, fmt.Errorf("get quality hard-close settings: %w", err)
	}
	if value == "" {
		return DefaultQualityHardCloseSettings(), nil
	}
	var settings QualityHardCloseSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultQualityHardCloseSettings(), nil
	}
	normalizeQualityHardCloseSettings(&settings)
	return &settings, nil
}

// SetQualityHardCloseSettings persists the global template after validation.
func (s *SettingService) SetQualityHardCloseSettings(ctx context.Context, settings *QualityHardCloseSettings) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("settings service not ready")
	}
	if err := ValidateQualityHardCloseSettings(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal quality hard-close settings: %w", err)
	}
	return s.settingRepo.Set(ctx, SettingKeyQualityHardCloseSettings, string(data))
}

// ParseAccountQualityHardCloseOverlay reads extra.quality_hard_close.
func ParseAccountQualityHardCloseOverlay(extra map[string]any) AccountQualityHardCloseOverlay {
	def := DefaultAccountQualityHardCloseOverlay()
	if extra == nil {
		return def
	}
	raw, ok := extra[AccountExtraQualityHardClose]
	if !ok || raw == nil {
		return def
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return def
	}
	var overlay AccountQualityHardCloseOverlay
	if err := json.Unmarshal(data, &overlay); err != nil {
		return def
	}
	return overlay
}

// ResolveAccountQualityHardClose merges global + overlay.
// Evaluation is allowed only when both enabled flags are true.
// use_global=true takes all thresholds from global; otherwise non-null overlay fields override.
func ResolveAccountQualityHardClose(global QualityHardCloseSettings, overlay AccountQualityHardCloseOverlay) QualityHardCloseSettings {
	resolved := global
	resolved.Enabled = global.Enabled && overlay.Enabled
	if overlay.UseGlobal {
		return resolved
	}
	if overlay.MaxP50TTFTMs != nil {
		resolved.MaxP50TTFTMs = overlay.MaxP50TTFTMs
	}
	if overlay.MinSuccessRate != nil {
		resolved.MinSuccessRate = overlay.MinSuccessRate
	}
	if overlay.PauseMinutes != nil {
		resolved.PauseMinutes = *overlay.PauseMinutes
	}
	if overlay.MinSuccessSamples != nil {
		resolved.MinSuccessSamples = *overlay.MinSuccessSamples
	}
	if overlay.MinTTFTSamples != nil {
		resolved.MinTTFTSamples = *overlay.MinTTFTSamples
	}
	if overlay.Condition != nil && strings.TrimSpace(*overlay.Condition) != "" {
		resolved.Condition = *overlay.Condition
	}
	return resolved
}

func BuildAccountQualityHardCloseView(global *QualityHardCloseSettings, extra map[string]any) AccountQualityHardCloseView {
	if global == nil {
		global = DefaultQualityHardCloseSettings()
	}
	overlay := ParseAccountQualityHardCloseOverlay(extra)
	return AccountQualityHardCloseView{
		Overlay:       overlay,
		Resolved:      ResolveAccountQualityHardClose(*global, overlay),
		GlobalEnabled: global.Enabled,
	}
}

func qualityHardCloseHasConfiguredMetric(cfg QualityHardCloseSettings) bool {
	return cfg.MaxP50TTFTMs != nil || cfg.MinSuccessRate != nil
}

func clampQualityHardClosePauseMinutes(minutes int) int {
	if minutes < QualityHardCloseMinPauseMinutes {
		return DefaultQualityHardClosePauseMinutes
	}
	if minutes > QualityHardCloseMaxPauseMinutes {
		return QualityHardCloseMaxPauseMinutes
	}
	return minutes
}

// EvaluateAccountQualityHardClose is the pure hard-close decision.
// Under-sampled or unconfigured metrics are not judged and do not enter and/or.
func EvaluateAccountQualityHardClose(stats *AccountQualityStats, resolvedCfg QualityHardCloseSettings, alreadyTempUnschedulable bool) (bool, string) {
	if !resolvedCfg.Enabled || alreadyTempUnschedulable || stats == nil {
		return false, ""
	}

	minSuccessSamples := resolvedCfg.MinSuccessSamples
	if minSuccessSamples < 1 {
		minSuccessSamples = 1
	}
	minTTFTSamples := resolvedCfg.MinTTFTSamples
	if minTTFTSamples < 1 {
		minTTFTSamples = 1
	}

	type judgedMetric struct {
		breached bool
		part     string
	}
	var judged []judgedMetric

	if resolvedCfg.MaxP50TTFTMs != nil && stats.TTFTSamples >= int64(minTTFTSamples) && stats.P50TTFTMs != nil {
		judged = append(judged, judgedMetric{
			breached: *stats.P50TTFTMs > *resolvedCfg.MaxP50TTFTMs,
			part:     "p50=" + strconv.Itoa(*stats.P50TTFTMs),
		})
	}
	if resolvedCfg.MinSuccessRate != nil {
		samples := stats.SuccessCount + stats.ErrorCount
		if samples >= int64(minSuccessSamples) {
			rate := 0.0
			if stats.SuccessRate != nil {
				rate = *stats.SuccessRate
			} else if samples > 0 {
				rate = float64(stats.SuccessCount) / float64(samples)
			}
			judged = append(judged, judgedMetric{
				breached: rate < *resolvedCfg.MinSuccessRate,
				part:     "success=" + strconv.FormatFloat(rate, 'f', -1, 64),
			})
		}
	}
	if len(judged) == 0 {
		return false, ""
	}

	shouldPause := false
	if strings.ToLower(strings.TrimSpace(resolvedCfg.Condition)) == QualityHardCloseConditionAnd {
		shouldPause = true
		for _, metric := range judged {
			if !metric.breached {
				shouldPause = false
				break
			}
		}
	} else {
		for _, metric := range judged {
			if metric.breached {
				shouldPause = true
				break
			}
		}
	}
	if !shouldPause {
		return false, ""
	}

	parts := make([]string, 0, len(judged))
	for _, metric := range judged {
		parts = append(parts, metric.part)
	}
	return true, QualityHardCloseReasonPrefix + ":" + strings.Join(parts, ",")
}

// AccountQualityHardCloseAccountRepo is the evaluator's account persistence surface.
type AccountQualityHardCloseAccountRepo interface {
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
	SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error
}

// AccountQualityHardCloseService evaluates live 15-minute stats and pauses opted-in accounts.
type AccountQualityHardCloseService struct {
	accounts AccountQualityHardCloseAccountRepo
	settings *SettingService
	now      func() time.Time
}

var _ AccountQualityHardCloseEvaluator = (*AccountQualityHardCloseService)(nil)

// NewAccountQualityHardCloseEvaluator constructs the maintenance-tick evaluator.
func NewAccountQualityHardCloseEvaluator(accountRepo AccountQualityHardCloseAccountRepo, settings *SettingService) *AccountQualityHardCloseService {
	return &AccountQualityHardCloseService{
		accounts: accountRepo,
		settings: settings,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// EvaluateHardClose loads global settings once, then evaluates opted-in accounts
// against the live stats map from the same maintenance tick.
func (s *AccountQualityHardCloseService) EvaluateHardClose(ctx context.Context, stats map[int64]*AccountQualityStats) {
	if s == nil || s.accounts == nil || s.settings == nil || len(stats) == 0 {
		return
	}
	global, err := s.settings.GetQualityHardCloseSettings(ctx)
	if err != nil {
		slog.Warn("account_quality_hard_close: load settings failed", "err", err)
		return
	}
	if global == nil || !global.Enabled {
		return
	}

	ids := make([]int64, 0, len(stats))
	for id := range stats {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	now := time.Now().UTC()
	if s.now != nil {
		now = s.now()
	}

	for _, chunk := range chunkInt64IDs(ids, AccountQualityMaxBatchSize) {
		accounts, err := s.accounts.GetByIDs(ctx, chunk)
		if err != nil {
			slog.Warn("account_quality_hard_close: load accounts failed", "err", err)
			continue
		}
		for _, account := range accounts {
			if account == nil {
				continue
			}
			overlay := ParseAccountQualityHardCloseOverlay(account.Extra)
			if !overlay.Enabled {
				continue
			}
			resolved := ResolveAccountQualityHardClose(*global, overlay)
			if !resolved.Enabled || !qualityHardCloseHasConfiguredMetric(resolved) {
				continue
			}
			alreadyPaused := (account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil)) ||
				AccountQualityResumeActive(stats[account.ID], now)
			shouldPause, reason := EvaluateAccountQualityHardClose(stats[account.ID], resolved, alreadyPaused)
			if !shouldPause {
				continue
			}
			until := now.Add(time.Duration(clampQualityHardClosePauseMinutes(resolved.PauseMinutes)) * time.Minute)
			if err := s.accounts.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
				slog.Warn("account_quality_hard_close: pause failed", "account_id", account.ID, "err", err)
				continue
			}
			slog.Info("account_quality_hard_close",
				"account_id", account.ID,
				"reason", reason,
				"until", until.Format(time.RFC3339),
			)
		}
	}
}
