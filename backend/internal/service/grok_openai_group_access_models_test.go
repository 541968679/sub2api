package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type grokAccessModelAccountRepo struct {
	AccountRepository
	accounts []Account
}

func (r grokAccessModelAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]Account, error) {
	var out []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			out = append(out, acc)
		}
	}
	return out, nil
}

func TestGatewayService_MergeOpenAIDiscoveryWithGrokAccess(t *testing.T) {
	groupID := int64(42)
	optIn := Account{
		ID:          1,
		Platform:    PlatformGrok,
		Status:      StatusActive,
		Schedulable: true,
		Extra:       map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-4.5": "grok-4.5"},
		},
	}
	optOut := Account{
		ID:          2,
		Platform:    PlatformGrok,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-4.3": "grok-4.3"},
		},
	}
	svc := &GatewayService{
		accountRepo: grokAccessModelAccountRepo{accounts: []Account{optIn, optOut}},
	}

	base := []string{"gpt-5.6-sol", "gpt-5.5"}
	merged := svc.MergeOpenAIDiscoveryWithGrokAccess(context.Background(), &groupID, base)
	require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.5", "grok-4.5"}, merged)

	// No opt-in candidates → base unchanged.
	svc.accountRepo = grokAccessModelAccountRepo{accounts: []Account{optOut}}
	unchanged := svc.MergeOpenAIDiscoveryWithGrokAccess(context.Background(), &groupID, base)
	require.Equal(t, base, unchanged)

	// nil group → no merge.
	require.Equal(t, base, svc.MergeOpenAIDiscoveryWithGrokAccess(context.Background(), nil, base))
}

func TestGatewayService_ListGrokOpenAIGroupAccessModelIDs_APIKeyAndOAuth(t *testing.T) {
	groupID := int64(43)
	oauth := Account{
		ID: 1, Platform: PlatformGrok, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true,
		Extra: map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-4.5": "grok-4.5"},
		},
	}
	apikey := Account{
		ID: 2, Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true,
		Extra: map[string]any{AccountExtraGrokOpenAIGroupAccessEnabled: true},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-build-0.1": "grok-build-0.1"},
		},
	}
	svc := &GatewayService{
		accountRepo: grokAccessModelAccountRepo{accounts: []Account{oauth, apikey}},
	}
	ids := svc.ListGrokOpenAIGroupAccessModelIDs(context.Background(), &groupID)
	require.Contains(t, ids, "grok-4.5")
	require.Contains(t, ids, "grok-build-0.1")
}
