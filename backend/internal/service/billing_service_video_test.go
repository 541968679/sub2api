//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateVideoCost_Grok15UsesPerSecondRate(t *testing.T) {
	svc := &BillingService{}
	cost := svc.CalculateVideoCost("grok-imagine-video-1.5", "1080p", 2, 10, nil, 1.5)
	require.Equal(t, string(BillingModeVideo), cost.BillingMode)
	require.Equal(t, "1080p", cost.BillingTier)
	require.InDelta(t, 5.0, cost.TotalCost, 1e-9)
	require.InDelta(t, 7.5, cost.ActualCost, 1e-9)
}

func TestCalculateVideoCost_GroupPriceOverridesDefault(t *testing.T) {
	price := 0.11
	svc := &BillingService{}
	cost := svc.CalculateVideoCost("grok-imagine-video", "720p", 1, 4, &VideoPriceConfig{Price720P: &price}, 0.5)
	require.InDelta(t, 0.44, cost.TotalCost, 1e-9)
	require.InDelta(t, 0.22, cost.ActualCost, 1e-9)
}

func TestResolveVideoRateMultiplierDoesNotAffectOtherMedia(t *testing.T) {
	apiKey := &APIKey{Group: &Group{VideoRateIndependent: true, VideoRateMultiplier: 2.25}}
	require.InDelta(t, 2.25, resolveVideoRateMultiplier(apiKey, 1.1), 1e-9)
	require.InDelta(t, 1.1, resolveVideoRateMultiplier(&APIKey{Group: &Group{}}, 1.1), 1e-9)
}
