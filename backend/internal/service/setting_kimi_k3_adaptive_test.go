//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func resetKimiK3AdaptiveValidationCacheForTest() {
	kimiK3AdaptiveValidationSF.Forget("kimi_k3_adaptive_validation")
	kimiK3AdaptiveValidationCache.Store(&cachedKimiK3AdaptiveValidation{})
}

func TestSettingService_KimiK3AdaptiveValidationDefaultOff(t *testing.T) {
	resetKimiK3AdaptiveValidationCacheForTest()
	t.Cleanup(resetKimiK3AdaptiveValidationCacheForTest)

	repo := &gatewayTTLSettingRepo{data: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	ctx := context.Background()

	require.False(t, svc.IsKimiK3AdaptiveValidationEnabled(ctx))
	settings, err := svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.KimiK3AdaptiveValidationEnabled)
	require.False(t, (*SettingService)(nil).IsKimiK3AdaptiveValidationEnabled(ctx))

	repo.data[SettingKeyKimiK3AdaptiveValidationEnabled] = "true"
	resetKimiK3AdaptiveValidationCacheForTest()
	require.True(t, svc.IsKimiK3AdaptiveValidationEnabled(ctx))

	repo.data[SettingKeyKimiK3AdaptiveValidationEnabled] = "false"
	resetKimiK3AdaptiveValidationCacheForTest()
	require.False(t, svc.IsKimiK3AdaptiveValidationEnabled(ctx))

	repo.data[SettingKeyKimiK3AdaptiveValidationEnabled] = "TRUE"
	resetKimiK3AdaptiveValidationCacheForTest()
	require.False(t, svc.IsKimiK3AdaptiveValidationEnabled(ctx))

	repo.data[SettingKeyKimiK3AdaptiveValidationEnabled] = "not-a-bool"
	resetKimiK3AdaptiveValidationCacheForTest()
	require.False(t, svc.IsKimiK3AdaptiveValidationEnabled(ctx))

	repo.data[SettingKeyKimiK3AdaptiveValidationEnabled] = "true"
	resetKimiK3AdaptiveValidationCacheForTest()
	require.True(t, svc.IsKimiK3AdaptiveValidationEnabled(ctx))
	repo.data[SettingKeyKimiK3AdaptiveValidationEnabled] = "false"
	require.True(t, svc.IsKimiK3AdaptiveValidationEnabled(ctx))
	refreshKimiK3AdaptiveValidationCache(false)
	require.False(t, svc.IsKimiK3AdaptiveValidationEnabled(ctx))
}
