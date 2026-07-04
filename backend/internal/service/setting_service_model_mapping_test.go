//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

type modelMappingSettingRepoStub struct {
	values map[string]string
}

func (s *modelMappingSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *modelMappingSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *modelMappingSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *modelMappingSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := s.values[key]; ok {
			result[key] = v
		}
	}
	return result, nil
}

func (s *modelMappingSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	for key, value := range settings {
		if err := s.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *modelMappingSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *modelMappingSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func TestSettingServiceModelPricingHiddenModelsRoundTrip(t *testing.T) {
	repo := &modelMappingSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	ctx := context.Background()

	require.Empty(t, svc.GetModelPricingHiddenModels(ctx))

	require.NoError(t, svc.SetModelPricingHiddenModels(ctx, []string{" Claude-Opus-4-5 ", "gpt-4o", "gpt-4o", ""}))
	require.Equal(t, []string{"claude-opus-4-5", "gpt-4o"}, svc.GetModelPricingHiddenModels(ctx))
	require.Equal(t, map[string]bool{"claude-opus-4-5": true, "gpt-4o": true}, svc.GetModelPricingHiddenModelSet(ctx))

	require.NoError(t, svc.SetModelPricingHiddenModels(ctx, nil))
	require.Empty(t, svc.GetModelPricingHiddenModels(ctx))
}

func TestSettingServicePlatformDefaultModelMappingPreservesEmptyOverride(t *testing.T) {
	repo := &modelMappingSettingRepoStub{values: map[string]string{
		SettingKeyAntigravityDefaultModelMapping: "{}",
	}}
	svc := NewSettingService(repo, &config.Config{})

	mapping := svc.GetPlatformDefaultModelMapping(context.Background(), PlatformAntigravity)
	require.NotNil(t, mapping)
	require.Empty(t, mapping)
}

func TestProvideSettingServiceAntigravityDefaultMappingUsesSavedFullTable(t *testing.T) {
	previousDefaultOverride := domain.GetPlatformDefaultMappingOverride
	previousAntigravityOverride := domain.GetAntigravityDefaultMappingOverride
	previousBillingObjectOverride := domain.GetPlatformDefaultMappingBillingObjectOverride
	previousHiddenModelsOverride := domain.GetModelPricingHiddenModelsOverride
	t.Cleanup(func() {
		domain.GetPlatformDefaultMappingOverride = previousDefaultOverride
		domain.GetAntigravityDefaultMappingOverride = previousAntigravityOverride
		domain.GetPlatformDefaultMappingBillingObjectOverride = previousBillingObjectOverride
		domain.GetModelPricingHiddenModelsOverride = previousHiddenModelsOverride
	})

	repo := &modelMappingSettingRepoStub{values: map[string]string{}}
	ProvideSettingService(repo, nil, nil, &config.Config{})

	mapping := domain.ResolvePlatformDefaultModelMapping(PlatformAntigravity)
	require.NotEmpty(t, mapping)
	require.Equal(t, domain.DefaultAntigravityModelMapping["claude-opus-4-6"], mapping["claude-opus-4-6"])

	require.NoError(t, repo.Set(context.Background(), SettingKeyAntigravityDefaultModelMapping, "{}"))
	mapping = domain.ResolvePlatformDefaultModelMapping(PlatformAntigravity)
	require.NotNil(t, mapping)
	require.Empty(t, mapping)

	require.NoError(t, repo.Set(context.Background(), SettingKeyAntigravityDefaultModelMapping, `{"custom-request":"custom-upstream"}`))
	mapping = domain.ResolvePlatformDefaultModelMapping(PlatformAntigravity)
	require.Equal(t, map[string]string{"custom-request": "custom-upstream"}, mapping)
}
