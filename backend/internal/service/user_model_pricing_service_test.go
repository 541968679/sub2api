//go:build unit

package service

import (
	"context"
	"math"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type userModelPricingValidationRepoStub struct {
	createCalls int
	updateCalls int
	batchCalls  int
}

func (s *userModelPricingValidationRepoStub) GetByUserID(context.Context, int64) ([]UserModelPricingOverride, error) {
	return nil, nil
}

func (s *userModelPricingValidationRepoStub) GetEnabledByUserID(context.Context, int64) ([]UserModelPricingOverride, error) {
	return nil, nil
}

func (s *userModelPricingValidationRepoStub) GetByUserAndModel(context.Context, int64, string) (*UserModelPricingOverride, error) {
	return nil, nil
}

func (s *userModelPricingValidationRepoStub) GetByID(context.Context, int64) (*UserModelPricingOverride, error) {
	return nil, nil
}

func (s *userModelPricingValidationRepoStub) Create(context.Context, *UserModelPricingOverride) error {
	s.createCalls++
	return nil
}

func (s *userModelPricingValidationRepoStub) Update(context.Context, *UserModelPricingOverride) error {
	s.updateCalls++
	return nil
}

func (s *userModelPricingValidationRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (s *userModelPricingValidationRepoStub) DeleteByUserID(context.Context, int64) error {
	return nil
}

func (s *userModelPricingValidationRepoStub) BatchUpsert(context.Context, int64, []UserModelPricingOverride) error {
	s.batchCalls++
	return nil
}

func (s *userModelPricingValidationRepoStub) GetEnabledCountByModel(context.Context) (map[string]int, error) {
	return nil, nil
}

func (s *userModelPricingValidationRepoStub) GetByModel(context.Context, string) ([]UserModelPricingOverride, error) {
	return nil, nil
}

func TestUserModelPricingServiceCreateRejectsNegativePrice(t *testing.T) {
	repo := &userModelPricingValidationRepoStub{}
	svc := NewUserModelPricingService(repo)
	price := -0.01

	err := svc.Create(context.Background(), &UserModelPricingOverride{
		UserID:     42,
		Model:      "claude-sonnet-4",
		InputPrice: &price,
		Enabled:    true,
	})

	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "INVALID_USER_MODEL_PRICE", infraerrors.Reason(err))
	require.Equal(t, 0, repo.createCalls)
}

func TestUserModelPricingServiceUpdateRejectsNegativePrice(t *testing.T) {
	repo := &userModelPricingValidationRepoStub{}
	svc := NewUserModelPricingService(repo)
	price := -0.02

	err := svc.Update(context.Background(), &UserModelPricingOverride{
		ID:          10,
		UserID:      42,
		Model:       "claude-sonnet-4",
		OutputPrice: &price,
		Enabled:     true,
	})

	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "INVALID_USER_MODEL_PRICE", infraerrors.Reason(err))
	require.Equal(t, 0, repo.updateCalls)
}

func TestUserModelPricingServiceBatchUpsertRejectsNegativePrice(t *testing.T) {
	repo := &userModelPricingValidationRepoStub{}
	svc := NewUserModelPricingService(repo)
	price := -0.03

	err := svc.BatchUpsert(context.Background(), 42, []UserModelPricingOverride{
		{
			Model:          "claude-sonnet-4",
			CacheReadPrice: &price,
			Enabled:        true,
		},
	})

	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "INVALID_USER_MODEL_PRICE", infraerrors.Reason(err))
	require.Equal(t, 0, repo.batchCalls)
}

func TestUserModelPricingServiceRejectsNonFiniteValues(t *testing.T) {
	repo := &userModelPricingValidationRepoStub{}
	svc := NewUserModelPricingService(repo)
	nan := math.NaN()
	inf := math.Inf(1)

	err := svc.Create(context.Background(), &UserModelPricingOverride{
		UserID:     42,
		Model:      "claude-sonnet-4",
		InputPrice: &nan,
		Enabled:    true,
	})
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, 0, repo.createCalls)

	err = svc.Update(context.Background(), &UserModelPricingOverride{
		ID:                    10,
		UserID:                42,
		Model:                 "claude-sonnet-4",
		DisplayRateMultiplier: &inf,
		Enabled:               true,
	})
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, 0, repo.updateCalls)
}

func TestUserModelPricingServiceAcceptsZeroPrice(t *testing.T) {
	repo := &userModelPricingValidationRepoStub{}
	svc := NewUserModelPricingService(repo)
	zero := 0.0

	err := svc.Create(context.Background(), &UserModelPricingOverride{
		UserID:     42,
		Model:      " claude-sonnet-4 ",
		InputPrice: &zero,
		Enabled:    true,
	})

	require.NoError(t, err)
	require.Equal(t, 1, repo.createCalls)
}
