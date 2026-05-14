//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageBillingMegapixelStandard2K(t *testing.T) {
	bs := &BillingService{}
	resolver := NewModelPricingResolver(nil, bs)
	price := 0.3178914388
	resolved := &ResolvedPricing{
		Mode:                 BillingModeImage,
		ImageBillingStrategy: ImageBillingStrategyMegapixel,
		ImageMegapixelPrice:  price,
	}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-image-2",
		RequestCount:   1,
		ImageSize:      ParseImageSize("1536x1024"),
		ImageQuality:   "auto",
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	})

	require.NoError(t, err)
	require.InDelta(t, 0.5, cost.TotalCost, 1e-9)
	require.Equal(t, ImageBillingTierMegapixel, cost.BillingTier)
}

func TestImageBillingMegapixelUsesQualityOverride(t *testing.T) {
	bs := &BillingService{}
	resolver := NewModelPricingResolver(nil, bs)
	resolved := &ResolvedPricing{
		Mode:                 BillingModeImage,
		ImageBillingStrategy: ImageBillingStrategyMegapixel,
		ImageMegapixelPrice:  0.1,
		ImageQualityPrices: ImageQualityPrices{
			"high": 0.25,
		},
	}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-image-2",
		RequestCount:   2,
		ImageSize:      ParseImageSize("1000x1000"),
		ImageQuality:   "high",
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	})

	require.NoError(t, err)
	require.InDelta(t, 0.5, cost.TotalCost, 1e-10)
}

func TestImageBillingTierUsesPixelRules(t *testing.T) {
	bs := &BillingService{}
	resolver := NewModelPricingResolver(nil, bs)
	price1K := 0.10
	price2K := 0.20
	price4K := 0.40
	resolved := &ResolvedPricing{
		Mode:                 BillingModeImage,
		ImageBillingStrategy: ImageBillingStrategyTier,
		ImageTierRules:       DefaultImageTierRules(&price1K, &price2K, &price4K),
	}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-image-2",
		RequestCount:   1,
		ImageSize:      ParseImageSize("1536x1024"),
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	})

	require.NoError(t, err)
	require.InDelta(t, 0.20, cost.TotalCost, 1e-10)
	require.Equal(t, ImageBillingTier2K, cost.BillingTier)

	cost, err = bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-image-2",
		RequestCount:   1,
		ImageSize:      ParseImageSize("3840x2160"),
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	})

	require.NoError(t, err)
	require.InDelta(t, 0.40, cost.TotalCost, 1e-10)
	require.Equal(t, ImageBillingTier4K, cost.BillingTier)
}

func TestImageBillingTierUsesQualityMultiplier(t *testing.T) {
	bs := &BillingService{}
	resolver := NewModelPricingResolver(nil, bs)
	price1K := 0.10
	price2K := 0.20
	price4K := 0.40
	resolved := &ResolvedPricing{
		Mode:                 BillingModeImage,
		ImageBillingStrategy: ImageBillingStrategyTier,
		ImageTierRules:       DefaultImageTierRules(&price1K, &price2K, &price4K),
		ImageQualityMultipliers: ImageQualityMultipliers{
			"auto": 1,
			"high": 1.5,
		},
	}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-image-2",
		RequestCount:   2,
		ImageSize:      ParseImageSize("1536x1024"),
		ImageQuality:   "high",
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	})

	require.NoError(t, err)
	require.InDelta(t, 0.60, cost.TotalCost, 1e-10)
	require.Equal(t, ImageBillingTier2K, cost.BillingTier)
}

func TestImageBillingTierAutoQualityDefaultsToOne(t *testing.T) {
	bs := &BillingService{}
	resolver := NewModelPricingResolver(nil, bs)
	price1K := 0.10
	resolved := &ResolvedPricing{
		Mode:                 BillingModeImage,
		ImageBillingStrategy: ImageBillingStrategyTier,
		ImageTierRules:       DefaultImageTierRules(&price1K, nil, nil),
	}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-image-2",
		RequestCount:   1,
		ImageSize:      ParseImageSize("1024x1024"),
		ImageQuality:   "",
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	})

	require.NoError(t, err)
	require.InDelta(t, 0.10, cost.TotalCost, 1e-10)
	require.Equal(t, ImageBillingTier1K, cost.BillingTier)
}

func TestImageBillingAutoFallsBackToPerRequest(t *testing.T) {
	bs := &BillingService{}
	resolver := NewModelPricingResolver(nil, bs)
	resolved := &ResolvedPricing{
		Mode:                   BillingModeImage,
		ImageBillingStrategy:   ImageBillingStrategyMegapixel,
		ImageMegapixelPrice:    0.5,
		DefaultPerRequestPrice: 0.12,
	}

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-image-2",
		RequestCount:   1,
		ImageSize:      ParseImageSize("auto"),
		RateMultiplier: 1,
		Resolver:       resolver,
		Resolved:       resolved,
	})

	require.NoError(t, err)
	require.InDelta(t, 0.12, cost.TotalCost, 1e-10)
	require.Empty(t, cost.BillingTier)
}
