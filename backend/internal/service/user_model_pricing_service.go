package service

import (
	"context"
	"fmt"
	"math"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// UserModelPricingService 用户模型定价覆盖管理服务
type UserModelPricingService struct {
	repo UserModelPricingRepository
}

func NewUserModelPricingService(repo UserModelPricingRepository) *UserModelPricingService {
	return &UserModelPricingService{repo: repo}
}

func (s *UserModelPricingService) ListByUserID(ctx context.Context, userID int64) ([]UserModelPricingOverride, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *UserModelPricingService) GetEnabledByUserID(ctx context.Context, userID int64) ([]UserModelPricingOverride, error) {
	return s.repo.GetEnabledByUserID(ctx, userID)
}

func (s *UserModelPricingService) GetByUserAndModel(ctx context.Context, userID int64, model string) (*UserModelPricingOverride, error) {
	return s.repo.GetByUserAndModel(ctx, userID, model)
}

func (s *UserModelPricingService) GetByID(ctx context.Context, id int64) (*UserModelPricingOverride, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserModelPricingService) Create(ctx context.Context, o *UserModelPricingOverride) error {
	if err := validateUserModelPricingOverride(o); err != nil {
		return err
	}
	return s.repo.Create(ctx, o)
}

func (s *UserModelPricingService) Update(ctx context.Context, o *UserModelPricingOverride) error {
	if err := validateUserModelPricingOverride(o); err != nil {
		return err
	}
	return s.repo.Update(ctx, o)
}

func (s *UserModelPricingService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *UserModelPricingService) DeleteByUserID(ctx context.Context, userID int64) error {
	return s.repo.DeleteByUserID(ctx, userID)
}

func (s *UserModelPricingService) BatchUpsert(ctx context.Context, userID int64, overrides []UserModelPricingOverride) error {
	for i := range overrides {
		overrides[i].UserID = userID
		if err := validateUserModelPricingOverride(&overrides[i]); err != nil {
			return err
		}
	}
	return s.repo.BatchUpsert(ctx, userID, overrides)
}

func (s *UserModelPricingService) GetEnabledCountByModel(ctx context.Context) (map[string]int, error) {
	return s.repo.GetEnabledCountByModel(ctx)
}

func (s *UserModelPricingService) GetByModel(ctx context.Context, model string) ([]UserModelPricingOverride, error) {
	return s.repo.GetByModel(ctx, model)
}

func validateUserModelPricingOverride(o *UserModelPricingOverride) error {
	if o == nil {
		return infraerrors.BadRequest("INVALID_USER_MODEL_PRICING", "pricing override is required")
	}
	o.Model = strings.TrimSpace(o.Model)
	if o.Model == "" {
		return infraerrors.BadRequest("INVALID_MODEL", "model name is required")
	}

	prices := []struct {
		name  string
		value *float64
	}{
		{"input_price", o.InputPrice},
		{"output_price", o.OutputPrice},
		{"cache_write_price", o.CacheWritePrice},
		{"cache_read_price", o.CacheReadPrice},
		{"display_input_price", o.DisplayInputPrice},
		{"display_output_price", o.DisplayOutputPrice},
		{"display_cache_read_price", o.DisplayCacheReadPrice},
		{"display_cache_creation_price", o.DisplayCacheCreationPrice},
	}
	for _, p := range prices {
		if err := validateNonNegativeFiniteFloat(p.name, p.value); err != nil {
			return err
		}
	}
	if err := validatePositiveFiniteFloat("display_rate_multiplier", o.DisplayRateMultiplier); err != nil {
		return err
	}
	return nil
}

func validateNonNegativeFiniteFloat(field string, value *float64) error {
	if value == nil {
		return nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return infraerrors.BadRequest("INVALID_USER_MODEL_PRICE", fmt.Sprintf("%s must be >= 0", field))
	}
	return nil
}

func validatePositiveFiniteFloat(field string, value *float64) error {
	if value == nil {
		return nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value <= 0 {
		return infraerrors.BadRequest("INVALID_USER_MODEL_PRICE", fmt.Sprintf("%s must be > 0", field))
	}
	return nil
}
