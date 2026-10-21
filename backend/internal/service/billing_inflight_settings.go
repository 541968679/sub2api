package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	// InflightInputPricingFull prices every estimated input token at the input price.
	InflightInputPricingFull = "full"
	// InflightInputPricingCacheTail prices the first FullPriceInputTokens at the
	// input price and the remainder at the cache-read price.
	InflightInputPricingCacheTail = "cache_tail"

	inflightReservationSettingsTTL      = 5 * time.Second
	inflightReservationSettingsErrorTTL = 5 * time.Second
	inflightReservationSettingsTimeout  = 2 * time.Second
	inflightReservationSettingsSFKey    = "billing_inflight_reservation"
)

// InflightReservationSettings is the live balance-precheck rule.
// Source is "config" until an admin save writes the settings row, then "settings".
type InflightReservationSettings struct {
	Enabled              bool    `json:"enabled"`
	TTLSeconds           int     `json:"ttl_seconds"`
	BytesPerToken        int     `json:"bytes_per_token"`
	DefaultMaxTokens     int     `json:"default_max_tokens"`
	MaxOutputTokens      int     `json:"max_output_tokens"`
	MaxInputTokens       int     `json:"max_input_tokens"`
	InputPricing         string  `json:"input_pricing"`
	FullPriceInputTokens int     `json:"full_price_input_tokens"`
	MaxReservationAmount float64 `json:"max_reservation_amount"`
	FailClosedOnUnpriced bool    `json:"fail_closed_on_unpriced"`
	Source               string  `json:"source,omitempty"`
}

type cachedInflightReservationSettings struct {
	settings  InflightReservationSettings
	expiresAt int64
	revision  uint64
}

// InflightReservationSettingsFromConfig fills the rule from startup config.
// A nil config stays disabled. Zero token fields keep the historical defaults.
func InflightReservationSettingsFromConfig(cfg *config.Config) InflightReservationSettings {
	out := InflightReservationSettings{
		TTLSeconds:           int(defaultInflightReservationTTL / time.Second),
		BytesPerToken:        inflightInputBytesPerTokenEstimate,
		DefaultMaxTokens:     defaultInflightDefaultMaxTokens,
		MaxOutputTokens:      128000,
		MaxInputTokens:       200000,
		InputPricing:         InflightInputPricingFull,
		FullPriceInputTokens: 4096,
		Source:               "config",
	}
	if cfg == nil {
		return out
	}
	c := cfg.Billing.InflightReservation
	out.Enabled = c.Enabled
	if c.TTLSeconds > 0 {
		out.TTLSeconds = c.TTLSeconds
	}
	if c.DefaultMaxTokens > 0 {
		out.DefaultMaxTokens = c.DefaultMaxTokens
	}
	if c.MaxOutputTokens > 0 {
		out.MaxOutputTokens = c.MaxOutputTokens
	}
	if c.MaxInputTokens > 0 {
		out.MaxInputTokens = c.MaxInputTokens
	}
	if c.MaxReservationUSD > 0 {
		out.MaxReservationAmount = c.MaxReservationUSD
	}
	out.FailClosedOnUnpriced = c.FailClosedOnUnpriced
	return out
}

func (s *SettingService) inflightReservationFallback() InflightReservationSettings {
	if s == nil {
		return InflightReservationSettingsFromConfig(nil)
	}
	return InflightReservationSettingsFromConfig(s.cfg)
}

func (s *SettingService) storeInflightReservationSettings(settings InflightReservationSettings, ttl time.Duration, revision uint64) {
	if s == nil {
		return
	}
	if current, ok := s.inflightReservationCache.Load().(*cachedInflightReservationSettings); ok && current != nil && current.revision > revision {
		return
	}
	s.inflightReservationCache.Store(&cachedInflightReservationSettings{
		settings:  settings,
		expiresAt: time.Now().Add(ttl).UnixNano(),
		revision:  revision,
	})
}

// GetInflightReservationSettings returns the admin rule when one is saved,
// otherwise the startup config. The result is cached for a few seconds.
// SetInflightReservationSettings refreshes that cache before it returns.
func (s *SettingService) GetInflightReservationSettings(ctx context.Context) InflightReservationSettings {
	fallback := s.inflightReservationFallback()
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	if cached, ok := s.inflightReservationCache.Load().(*cachedInflightReservationSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.settings
		}
	}
	result, _, _ := s.inflightReservationSF.Do(inflightReservationSettingsSFKey, func() (any, error) {
		revision := s.inflightReservationRevision.Load()
		if cached, ok := s.inflightReservationCache.Load().(*cachedInflightReservationSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.settings, nil
			}
		}
		loadCtx := ctx
		if loadCtx == nil {
			loadCtx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(loadCtx), inflightReservationSettingsTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyBillingInflightReservation)
		if err != nil {
			if errors.Is(err, ErrSettingNotFound) {
				s.storeInflightReservationSettings(fallback, inflightReservationSettingsTTL, revision)
				return fallback, nil
			}
			slog.Warn("failed to get billing inflight reservation settings", "error", err)
			s.storeInflightReservationSettings(fallback, inflightReservationSettingsErrorTTL, revision)
			return fallback, nil
		}
		settings, err := parseInflightReservationSettings(value, fallback)
		if err != nil {
			slog.Warn("failed to parse billing inflight reservation settings", "error", err)
			s.storeInflightReservationSettings(fallback, inflightReservationSettingsErrorTTL, revision)
			return fallback, nil
		}
		s.storeInflightReservationSettings(settings, inflightReservationSettingsTTL, revision)
		return settings, nil
	})
	if settings, ok := result.(InflightReservationSettings); ok {
		return settings
	}
	return fallback
}

// SetInflightReservationSettings validates and stores the rule. The next
// precheck on this process reads the saved value without a restart.
func (s *SettingService) SetInflightReservationSettings(ctx context.Context, settings InflightReservationSettings) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("settings storage is unavailable")
	}
	normalized, err := normalizeInflightReservationSettings(settings)
	if err != nil {
		return err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal inflight reservation settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyBillingInflightReservation, string(data)); err != nil {
		return fmt.Errorf("save inflight reservation settings: %w", err)
	}
	normalized.Source = "settings"
	revision := s.inflightReservationRevision.Add(1)
	s.inflightReservationSF.Forget(inflightReservationSettingsSFKey)
	s.storeInflightReservationSettings(normalized, inflightReservationSettingsTTL, revision)
	return nil
}

// InflightReservationSimpleMode reports whether this process ignores the
// precheck switch because billing itself is off.
func (s *SettingService) InflightReservationSimpleMode() bool {
	return s != nil && s.cfg != nil && s.cfg.RunMode == config.RunModeSimple
}

func parseInflightReservationSettings(value string, fallback InflightReservationSettings) (InflightReservationSettings, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	var settings InflightReservationSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return InflightReservationSettings{}, err
	}
	normalized, err := normalizeInflightReservationSettings(settings)
	if err != nil {
		return InflightReservationSettings{}, err
	}
	normalized.Source = "settings"
	return normalized, nil
}

func normalizeInflightReservationSettings(in InflightReservationSettings) (InflightReservationSettings, error) {
	in.Source = ""
	in.InputPricing = strings.ToLower(strings.TrimSpace(in.InputPricing))
	if in.InputPricing == "" {
		in.InputPricing = InflightInputPricingFull
	}
	switch in.InputPricing {
	case InflightInputPricingFull, InflightInputPricingCacheTail:
	default:
		return InflightReservationSettings{}, fmt.Errorf("input_pricing must be %q or %q", InflightInputPricingFull, InflightInputPricingCacheTail)
	}
	if in.TTLSeconds < 30 || in.TTLSeconds > 86400 {
		return InflightReservationSettings{}, fmt.Errorf("ttl_seconds must be between 30 and 86400")
	}
	if in.BytesPerToken < 1 || in.BytesPerToken > 64 {
		return InflightReservationSettings{}, fmt.Errorf("bytes_per_token must be between 1 and 64")
	}
	if in.MaxOutputTokens < 1 || in.MaxOutputTokens > 2_000_000 {
		return InflightReservationSettings{}, fmt.Errorf("max_output_tokens must be between 1 and 2000000")
	}
	if in.DefaultMaxTokens < 1 || in.DefaultMaxTokens > in.MaxOutputTokens {
		return InflightReservationSettings{}, fmt.Errorf("default_max_tokens must be between 1 and max_output_tokens")
	}
	if in.MaxInputTokens < 1 || in.MaxInputTokens > 10_000_000 {
		return InflightReservationSettings{}, fmt.Errorf("max_input_tokens must be between 1 and 10000000")
	}
	if in.FullPriceInputTokens < 0 || in.FullPriceInputTokens > in.MaxInputTokens {
		return InflightReservationSettings{}, fmt.Errorf("full_price_input_tokens must be between 0 and max_input_tokens")
	}
	if math.IsNaN(in.MaxReservationAmount) || math.IsInf(in.MaxReservationAmount, 0) || in.MaxReservationAmount < 0 || in.MaxReservationAmount > 1_000_000 {
		return InflightReservationSettings{}, fmt.Errorf("max_reservation_amount must be between 0 and 1000000")
	}
	return in, nil
}

func splitInflightInput(settings InflightReservationSettings, inputTokens int) (fullPrice, cachePrice int) {
	if inputTokens <= 0 {
		return 0, 0
	}
	if settings.InputPricing != InflightInputPricingCacheTail {
		return inputTokens, 0
	}
	keep := settings.FullPriceInputTokens
	if keep < 0 {
		keep = 0
	}
	if inputTokens <= keep {
		return inputTokens, 0
	}
	return keep, inputTokens - keep
}

func inflightTokenCost(pricing *ModelPricing, settings InflightReservationSettings, inputTokens, outputTokens int, textRate float64) float64 {
	if pricing == nil || textRate == 0 {
		return 0
	}
	full, cache := splitInflightInput(settings, inputTokens)
	cacheUnit := pricing.CacheReadPricePerToken
	if cache > 0 && cacheUnit <= 0 {
		cacheUnit = pricing.InputPricePerToken
	}
	return (float64(full)*pricing.InputPricePerToken + float64(cache)*cacheUnit + float64(outputTokens)*pricing.OutputPricePerToken) * textRate
}

func capInflightReservation(amount, maxAmount float64) float64 {
	if maxAmount > 0 && amount > maxAmount {
		return maxAmount
	}
	return amount
}
