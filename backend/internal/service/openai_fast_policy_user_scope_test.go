package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIFastPolicyUserScopedRuleOverridesGlobalRule(t *testing.T) {
	settings := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{
		{ServiceTier: OpenAIFastTierPriority, Action: BetaPolicyActionFilter, Scope: BetaPolicyScopeAll},
		{ServiceTier: OpenAIFastTierPriority, Action: BetaPolicyActionPass, Scope: BetaPolicyScopeAll, UserIDs: []int64{42}},
	}}
	svc := newOpenAIGatewayServiceWithSettings(t, settings)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	body := []byte(`{"model":"gpt-5.5","service_tier":"priority"}`)

	allowed := context.WithValue(context.Background(), ctxkey.UserID, int64(42))
	updated, err := svc.applyOpenAIFastPolicyToBody(allowed, account, "gpt-5.5", body)
	require.NoError(t, err)
	require.Equal(t, "priority", gjson.GetBytes(updated, "service_tier").String())

	other := context.WithValue(context.Background(), ctxkey.UserID, int64(43))
	updated, err = svc.applyOpenAIFastPolicyToBody(other, account, "gpt-5.5", body)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(updated, "service_tier").Exists())
}
