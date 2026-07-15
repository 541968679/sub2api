package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// TestOpenAIGroupGrokAccess_BillingIdentityInvariants locks the product rules:
// OpenAI-group keys keep openai quota platform identity; scheduled account is Grok;
// requested model remains a Grok text model for pricing.
func TestOpenAIGroupGrokAccess_BillingIdentityInvariants(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	ctx := context.Background()
	groupID := int64(7701)

	openAIGroup := &Group{ID: groupID, Platform: PlatformOpenAI, RateMultiplier: 1.5}
	apiKey := &APIKey{
		ID:      1,
		GroupID: &groupID,
		Group:   openAIGroup,
	}

	// Quota platform follows the key's group (openai), not the selected account.
	require.Equal(t, PlatformOpenAI, QuotaPlatform(ctx, apiKey))
	require.Equal(t, PlatformOpenAI, PlatformFromAPIKey(apiKey))

	optInGrok := Account{
		ID: 77012, Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Extra: map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        schedulerTestOpenAIAccountRepo{accounts: []Account{optInGrok}},
		cache:              &schedulerTestGatewayCache{},
		cfg:                &config.Config{},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
	}

	requestedModel := "grok-4.5"
	require.True(t, IsGrokTextModel(requestedModel))

	sel, _, err := svc.SelectAccountWithSchedulerForOpenAICompatibleRequest(
		ctx, &groupID, "", "", requestedModel, nil,
		OpenAIUpstreamTransportHTTPSSE, OpenAIEndpointCapabilityChatCompletions, false, false, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, sel)
	require.NotNil(t, sel.Account)
	require.Equal(t, PlatformGrok, sel.Account.Platform)
	require.True(t, sel.Account.IsGrokOpenAIGroupAccessEnabled())

	// Pricing identity: model string stays Grok; group rate is still from OpenAI group.
	require.Equal(t, "grok-4.5", requestedModel)
	require.Equal(t, 1.5, openAIGroup.RateMultiplier)
	require.Equal(t, PlatformOpenAI, QuotaPlatform(ctx, apiKey))
}
