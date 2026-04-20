package service

import (
	"context"
	"fmt"
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
	if o.Model == "" {
		return fmt.Errorf("model name is required")
	}
	return s.repo.Create(ctx, o)
}

func (s *UserModelPricingService) Update(ctx context.Context, o *UserModelPricingOverride) error {
	return s.repo.Update(ctx, o)
}

func (s *UserModelPricingService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *UserModelPricingService) DeleteByUserID(ctx context.Context, userID int64) error {
	return s.repo.DeleteByUserID(ctx, userID)
}

func (s *UserModelPricingService) BatchUpsert(ctx context.Context, userID int64, overrides []UserModelPricingOverride) error {
	return s.repo.BatchUpsert(ctx, userID, overrides)
}

func (s *UserModelPricingService) GetEnabledCountByModel(ctx context.Context) (map[string]int, error) {
	return s.repo.GetEnabledCountByModel(ctx)
}

func (s *UserModelPricingService) GetByModel(ctx context.Context, model string) ([]UserModelPricingOverride, error) {
	return s.repo.GetByModel(ctx, model)
}
