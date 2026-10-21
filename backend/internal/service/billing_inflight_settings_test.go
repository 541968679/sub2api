package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type inflightSettingsRepo struct {
	values map[string]string
}

func (r *inflightSettingsRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *inflightSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if r.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *inflightSettingsRepo) Set(_ context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *inflightSettingsRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *inflightSettingsRepo) SetMultiple(context.Context, map[string]string) error { return nil }

func (r *inflightSettingsRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *inflightSettingsRepo) Delete(context.Context, string) error { return nil }

func TestInflightReservationSettingsFromConfigKeepsStartupSwitch(t *testing.T) {
	disabled := InflightReservationSettingsFromConfig(nil)
	require.False(t, disabled.Enabled)
	require.Equal(t, InflightInputPricingFull, disabled.InputPricing)
	require.Equal(t, 4, disabled.BytesPerToken)
	require.Equal(t, "config", disabled.Source)

	loaded := InflightReservationSettingsFromConfig(&config.Config{
		Billing: config.BillingConfig{InflightReservation: config.InflightReservationConfig{
			Enabled:          true,
			TTLSeconds:       900,
			DefaultMaxTokens: 8192,
			MaxOutputTokens:  128000,
			MaxInputTokens:   200000,
		}},
	})
	require.True(t, loaded.Enabled)
	require.Equal(t, 8192, loaded.DefaultMaxTokens)
	require.Equal(t, 0.0, loaded.MaxReservationAmount)
}

func TestInflightReservationSettingsSaveOverridesConfigOnNextRead(t *testing.T) {
	repo := &inflightSettingsRepo{}
	svc := &SettingService{
		settingRepo: repo,
		cfg: &config.Config{
			Billing: config.BillingConfig{InflightReservation: config.InflightReservationConfig{
				Enabled:           true,
				TTLSeconds:        900,
				DefaultMaxTokens:  8192,
				MaxOutputTokens:   128000,
				MaxInputTokens:    200000,
				MaxReservationUSD: 0,
			}},
		},
	}
	ctx := context.Background()
	before := svc.GetInflightReservationSettings(ctx)
	require.Equal(t, "config", before.Source)
	require.Equal(t, InflightInputPricingFull, before.InputPricing)

	err := svc.SetInflightReservationSettings(ctx, InflightReservationSettings{
		Enabled:              true,
		TTLSeconds:           900,
		BytesPerToken:        4,
		DefaultMaxTokens:     1024,
		MaxOutputTokens:      128000,
		MaxInputTokens:       200000,
		InputPricing:         InflightInputPricingCacheTail,
		FullPriceInputTokens: 4096,
		MaxReservationAmount: 0.30,
	})
	require.NoError(t, err)

	svc.cfg.Billing.InflightReservation.DefaultMaxTokens = 8192
	got := svc.GetInflightReservationSettings(ctx)
	require.Equal(t, "settings", got.Source)
	require.Equal(t, InflightInputPricingCacheTail, got.InputPricing)
	require.Equal(t, 1024, got.DefaultMaxTokens)
	require.InDelta(t, 0.30, got.MaxReservationAmount, 1e-9)
	require.NotContains(t, repo.values[SettingKeyBillingInflightReservation], `"source"`)
}

func TestNormalizeInflightReservationSettingsRejectsBadRule(t *testing.T) {
	_, err := normalizeInflightReservationSettings(InflightReservationSettings{
		TTLSeconds:       10,
		BytesPerToken:    4,
		DefaultMaxTokens: 1024,
		MaxOutputTokens:  128000,
		MaxInputTokens:   200000,
		InputPricing:     "guess",
	})
	require.Error(t, err)
}

func TestInflightTokenCostCacheTailAndCap(t *testing.T) {
	pricing := &ModelPricing{
		InputPricePerToken:     0.00002,
		OutputPricePerToken:    0.0001,
		CacheReadPricePerToken: 0.000002,
	}
	settings := InflightReservationSettings{
		BytesPerToken:        4,
		DefaultMaxTokens:     1024,
		MaxOutputTokens:      128000,
		MaxInputTokens:       200000,
		InputPricing:         InflightInputPricingCacheTail,
		FullPriceInputTokens: 4096,
	}
	inputTokens, outputTokens := tokenCounts(settings, 63853*4, 0)
	require.Equal(t, 63853, inputTokens)
	require.Equal(t, 1024, outputTokens)
	cost := inflightTokenCost(pricing, settings, inputTokens, outputTokens, 0.45)
	full := 4096 * 0.00002 * 0.45
	cache := (63853 - 4096) * 0.000002 * 0.45
	output := 1024 * 0.0001 * 0.45
	require.InDelta(t, full+cache+output, cost, 1e-9)

	settings.InputPricing = InflightInputPricingFull
	settings.DefaultMaxTokens = 8192
	inputTokens, outputTokens = tokenCounts(settings, 63853*4, 0)
	fullCost := inflightTokenCost(pricing, settings, inputTokens, outputTokens, 0.45)
	require.InDelta(t, (63853*0.00002+8192*0.0001)*0.45, fullCost, 1e-9)
	require.Greater(t, fullCost, cost)

	pricing.CacheReadPricePerToken = 0
	settings.InputPricing = InflightInputPricingCacheTail
	settings.DefaultMaxTokens = 1024
	fallback := inflightTokenCost(pricing, settings, 63853, 1024, 0.45)
	require.InDelta(t, (63853*0.00002+1024*0.0001)*0.45, fallback, 1e-9)

	require.InDelta(t, 0.30, capInflightReservation(1.01, 0.30), 1e-9)
	require.InDelta(t, 0.08, capInflightReservation(0.08, 0), 1e-9)
}
