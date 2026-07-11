package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGatewayClientDatelineNormalizationScope(t *testing.T) {
	repo := &gatewayTTLSettingRepo{data: map[string]string{}}
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	svc := &GatewayService{settingService: NewSettingService(repo, &config.Config{})}
	ctx := context.Background()

	require.True(t, svc.shouldNormalizeClientDateline(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}))
	require.True(t, svc.shouldNormalizeClientDateline(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken}))
	require.False(t, svc.shouldNormalizeClientDateline(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}))
	require.False(t, svc.shouldNormalizeClientDateline(ctx, &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}), "Claude-GPT bridge uses OpenAI accounts and must stay outside this transform")
	require.False(t, svc.shouldNormalizeClientDateline(ctx, nil))

	repo.data[SettingKeyEnableClientDatelineNormalization] = "false"
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	require.False(t, svc.shouldNormalizeClientDateline(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}))
}

func TestGatewayClientDatelineNormalizationHelper(t *testing.T) {
	repo := &gatewayTTLSettingRepo{data: map[string]string{SettingKeyEnableClientDatelineNormalization: "true"}}
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	svc := &GatewayService{settingService: NewSettingService(repo, &config.Config{})}
	ctx := context.Background()
	dirty := []byte(`{"messages":[{"role":"user","content":"<system-reminder>Today’s date is 2026/07/01.</system-reminder>"}]}`)

	next, ok := svc.normalizeClientDatelineIfEnabled(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}, dirty)
	require.True(t, ok)
	require.Contains(t, string(next), "Today's date is 2026-07-01.")

	for _, account := range []*Account{
		nil,
		{Platform: PlatformAnthropic, Type: AccountTypeAPIKey},
		{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
	} {
		next, ok = svc.normalizeClientDatelineIfEnabled(ctx, account, dirty)
		require.False(t, ok)
		require.Nil(t, next)
	}

	userProse := []byte(`{"messages":[{"role":"user","content":"Today’s date is 2026/07/01."}]}`)
	next, ok = svc.normalizeClientDatelineIfEnabled(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}, userProse)
	require.False(t, ok)
	require.Nil(t, next)
}

func TestSystemSettingsClientDatelineNormalizationDefaultsOn(t *testing.T) {
	repo := &gatewayTTLSettingRepo{data: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.EnableClientDatelineNormalization)
}
