//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubPlazaGroupRepo struct {
	groups []Group
}

func (s *stubPlazaGroupRepo) ListActive(context.Context) ([]Group, error) {
	return s.groups, nil
}

type stubPlazaCatalog struct {
	models []GlobalModelPricing
}

func (s *stubPlazaCatalog) ListForPricingPage(context.Context) ([]GlobalModelPricing, error) {
	return s.models, nil
}

func TestListPlazaGroups_UsesDisplayPricesNotCostPerToken(t *testing.T) {
	stored := 9e-6
	display := 3e-6
	groups := []Group{{
		ID: 1, Name: "public-standard", Platform: PlatformAnthropic,
		Status: StatusActive, RateMultiplier: 1,
	}}
	catalog := []GlobalModelPricing{{
		Model: "claude-sonnet", Provider: PlatformAnthropic, Enabled: true, ShowOnPricingPage: true,
		InputPrice: &stored, DisplayInputPrice: &display,
	}}
	svc := NewModelPlazaService(
		&stubPlazaGroupRepo{groups: groups},
		&stubPlazaCatalog{models: catalog},
		nil,
	)
	out, err := svc.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Len(t, out[0].Models, 1)
	require.NotNil(t, out[0].Models[0].Pricing)
	require.InDelta(t, display, *out[0].Models[0].Pricing.InputPrice, 1e-15)
	require.NotEqual(t, stored, *out[0].Models[0].Pricing.InputPrice)
}
