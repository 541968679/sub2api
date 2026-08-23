//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAILongContextBillingSettingDefaultOn(t *testing.T) {
	resetOpenAILongContextBillingCacheForTest()
	t.Cleanup(resetOpenAILongContextBillingCacheForTest)

	repo := &gatewayTTLSettingRepo{data: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	ctx := context.Background()

	require.True(t, svc.IsOpenAILongContextBillingEnabled(ctx))
	settings, err := svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.True(t, settings.OpenAILongContextBillingEnabled)
	require.True(t, (*SettingService)(nil).IsOpenAILongContextBillingEnabled(ctx))

	repo.data[SettingKeyOpenAILongContextBillingEnabled] = "false"
	resetOpenAILongContextBillingCacheForTest()
	require.False(t, svc.IsOpenAILongContextBillingEnabled(ctx))

	settings, err = svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.OpenAILongContextBillingEnabled)

	repo.data[SettingKeyOpenAILongContextBillingEnabled] = "not-a-bool"
	resetOpenAILongContextBillingCacheForTest()
	require.True(t, svc.IsOpenAILongContextBillingEnabled(ctx))
}
