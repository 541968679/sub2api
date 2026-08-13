package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
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
	require.True(t, (&GatewayService{}).shouldNormalizeClientDateline(ctx, &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}), "nil setting service intentionally uses the default-on policy")

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

func TestOpenAIResponsesFlushPreambleSettingDefaultOff(t *testing.T) {
	prev := gatewayForwardingCache.Load()
	t.Cleanup(func() {
		if prev != nil {
			gatewayForwardingCache.Store(prev)
			return
		}
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	})

	repo := &gatewayTTLSettingRepo{data: map[string]string{}}
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	svc := NewSettingService(repo, &config.Config{})
	ctx := context.Background()

	require.False(t, svc.IsOpenAIResponsesFlushPreambleEnabled(ctx))
	settings, err := svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.OpenAIResponsesFlushPreamble)
	require.Empty(t, settings.OpenAIResponsesFlushPreambleUserIDs)
	require.False(t, (*SettingService)(nil).IsOpenAIResponsesFlushPreambleEnabled(ctx))

	repo.data[SettingKeyOpenAIResponsesFlushPreamble] = "true"
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	require.True(t, svc.IsOpenAIResponsesFlushPreambleEnabled(ctx))
	require.True(t, svc.IsOpenAIResponsesFlushPreambleEnabled(context.WithValue(ctx, ctxkey.UserID, int64(99))))

	repo.data[SettingKeyOpenAIResponsesFlushPreamble] = "false"
	repo.data[SettingKeyOpenAIResponsesFlushPreambleUserIDs] = "[42,42,-1,0]"
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	require.True(t, svc.IsOpenAIResponsesFlushPreambleEnabled(context.WithValue(ctx, ctxkey.UserID, int64(42))))
	require.False(t, svc.IsOpenAIResponsesFlushPreambleEnabled(context.WithValue(ctx, ctxkey.UserID, int64(43))))
	require.False(t, svc.IsOpenAIResponsesFlushPreambleEnabled(ctx))

	settings, err = svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.False(t, settings.OpenAIResponsesFlushPreamble)
	require.Equal(t, []int64{42}, settings.OpenAIResponsesFlushPreambleUserIDs)
	require.Equal(t, []int64{1, 2}, normalizeOpenAIResponsesFlushPreambleUserIDs([]int64{1, 1, 0, -3, 2}))
	require.False(t, openAIResponsesFlushPreambleUserMatches(nil, 1))
}
