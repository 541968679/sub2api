//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type displayTokenUserModelPricingRepoStub struct {
	override *UserModelPricingOverride
}

func (s *displayTokenUserModelPricingRepoStub) GetByUserID(context.Context, int64) ([]UserModelPricingOverride, error) {
	return nil, nil
}

func (s *displayTokenUserModelPricingRepoStub) GetEnabledByUserID(context.Context, int64) ([]UserModelPricingOverride, error) {
	if s.override == nil || !s.override.Enabled {
		return nil, nil
	}
	return []UserModelPricingOverride{*s.override}, nil
}

func (s *displayTokenUserModelPricingRepoStub) GetByUserAndModel(context.Context, int64, string) (*UserModelPricingOverride, error) {
	return s.override, nil
}

func (s *displayTokenUserModelPricingRepoStub) GetByID(context.Context, int64) (*UserModelPricingOverride, error) {
	return nil, nil
}

func (s *displayTokenUserModelPricingRepoStub) Create(context.Context, *UserModelPricingOverride) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) Update(context.Context, *UserModelPricingOverride) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) DeleteByUserID(context.Context, int64) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) BatchUpsert(context.Context, int64, []UserModelPricingOverride) error {
	return nil
}

func (s *displayTokenUserModelPricingRepoStub) GetEnabledCountByModel(context.Context) (map[string]int, error) {
	return nil, nil
}

func (s *displayTokenUserModelPricingRepoStub) GetByModel(context.Context, string) ([]UserModelPricingOverride, error) {
	return nil, nil
}

func TestDisplayToken_MaybeSetDisplayTokenMultipliersGatedByUserMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accountRate := 2.0
	svc := &GatewayService{}
	account := &Account{RateMultiplier: &accountRate}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	realModeKey := &APIKey{
		UserID: 1,
		User:   &User{ID: 1, DownstreamUsageTokenMode: DownstreamUsageTokenModeReal},
	}
	svc.MaybeSetDisplayTokenMultipliers(context.Background(), c, realModeKey, account, "claude-sonnet-4")
	require.Nil(t, getDisplayTokenMultipliers(c))

	displayModeKey := &APIKey{
		UserID: 1,
		User:   &User{ID: 1, DownstreamUsageTokenMode: DownstreamUsageTokenModeDisplay},
	}
	svc.MaybeSetDisplayTokenMultipliers(context.Background(), c, displayModeKey, account, "claude-sonnet-4")
	mult := getDisplayTokenMultipliers(c)
	require.NotNil(t, mult)
	require.Equal(t, accountRate, mult.InputMult)
	require.Equal(t, accountRate, mult.OutputMult)
	require.Equal(t, accountRate, mult.CacheReadMult)
	require.Equal(t, accountRate, mult.CacheCreateMult)
}

func TestDisplayToken_ComputeMultipliersUsesUserDisplayPricingOverride(t *testing.T) {
	globalInput := 4e-6
	globalOutput := 8e-6
	globalCacheRead := 2e-6
	globalDisplayInput := 2e-6
	globalDisplayOutput := 4e-6
	globalDisplayCacheRead := 1e-6
	userDisplayInput := 1e-6
	userDisplayOutput := 2e-6
	userDisplayCacheRead := 0.5e-6

	cache := newTestGlobalPricingCache(&GlobalModelPricing{
		Model:                 "claude-sonnet-4",
		BillingMode:           BillingModeToken,
		InputPrice:            &globalInput,
		OutputPrice:           &globalOutput,
		CacheReadPrice:        &globalCacheRead,
		DisplayInputPrice:     &globalDisplayInput,
		DisplayOutputPrice:    &globalDisplayOutput,
		DisplayCacheReadPrice: &globalDisplayCacheRead,
		Enabled:               true,
	})
	userRepo := &displayTokenUserModelPricingRepoStub{
		override: &UserModelPricingOverride{
			UserID:                42,
			Model:                 "claude-sonnet-4",
			DisplayInputPrice:     &userDisplayInput,
			DisplayOutputPrice:    &userDisplayOutput,
			DisplayCacheReadPrice: &userDisplayCacheRead,
			Enabled:               true,
		},
	}
	resolver := NewModelPricingResolver(&ChannelService{}, newTestBillingServiceWithRichPricing(), cache, userRepo)
	svc := &GatewayService{resolver: resolver}

	mult := svc.ComputeDisplayTokenMultipliers(context.Background(), "claude-sonnet-4", 42, nil, 1, 1, 1)
	require.NotNil(t, mult)
	require.InDelta(t, 4.0, mult.InputMult, 1e-12)
	require.InDelta(t, 4.0, mult.OutputMult, 1e-12)
	require.InDelta(t, 4.0, mult.CacheReadMult, 1e-12)
	require.InDelta(t, 1.0, mult.CacheCreateMult, 1e-12)
}
