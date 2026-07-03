package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type globalPricingServiceRepoStub struct {
	overrides []GlobalModelPricing
}

func (s *globalPricingServiceRepoStub) List(context.Context, pagination.PaginationParams, string, string) ([]GlobalModelPricing, *pagination.PaginationResult, error) {
	return s.overrides, &pagination.PaginationResult{Total: int64(len(s.overrides)), Page: 1, PageSize: len(s.overrides), Pages: 1}, nil
}

func (s *globalPricingServiceRepoStub) GetByID(context.Context, int64) (*GlobalModelPricing, error) {
	return nil, nil
}

func (s *globalPricingServiceRepoStub) GetByModel(_ context.Context, model string) (*GlobalModelPricing, error) {
	for i := range s.overrides {
		if strings.EqualFold(s.overrides[i].Model, model) {
			return &s.overrides[i], nil
		}
	}
	return nil, nil
}

func (s *globalPricingServiceRepoStub) Create(context.Context, *GlobalModelPricing) error { return nil }
func (s *globalPricingServiceRepoStub) Update(context.Context, *GlobalModelPricing) error { return nil }
func (s *globalPricingServiceRepoStub) Delete(context.Context, int64) error               { return nil }

func (s *globalPricingServiceRepoStub) GetAllEnabled(context.Context) ([]GlobalModelPricing, error) {
	var out []GlobalModelPricing
	for _, item := range s.overrides {
		if item.Enabled {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *globalPricingServiceRepoStub) ListForPricingPage(context.Context) ([]GlobalModelPricing, error) {
	return nil, nil
}

type globalPricingServiceChannelRepoStub struct{}

func (globalPricingServiceChannelRepoStub) Create(context.Context, *Channel) error { return nil }
func (globalPricingServiceChannelRepoStub) GetByID(context.Context, int64) (*Channel, error) {
	return nil, nil
}
func (globalPricingServiceChannelRepoStub) Update(context.Context, *Channel) error { return nil }
func (globalPricingServiceChannelRepoStub) Delete(context.Context, int64) error    { return nil }
func (globalPricingServiceChannelRepoStub) List(context.Context, pagination.PaginationParams, string, string) ([]Channel, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{}, nil
}
func (globalPricingServiceChannelRepoStub) ListAll(context.Context) ([]Channel, error) {
	return nil, nil
}
func (globalPricingServiceChannelRepoStub) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}
func (globalPricingServiceChannelRepoStub) ExistsByNameExcluding(context.Context, string, int64) (bool, error) {
	return false, nil
}
func (globalPricingServiceChannelRepoStub) GetGroupIDs(context.Context, int64) ([]int64, error) {
	return nil, nil
}
func (globalPricingServiceChannelRepoStub) SetGroupIDs(context.Context, int64, []int64) error {
	return nil
}
func (globalPricingServiceChannelRepoStub) GetChannelIDByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (globalPricingServiceChannelRepoStub) GetGroupsInOtherChannels(context.Context, int64, []int64) ([]int64, error) {
	return nil, nil
}
func (globalPricingServiceChannelRepoStub) GetGroupPlatforms(context.Context, []int64) (map[int64]string, error) {
	return nil, nil
}
func (globalPricingServiceChannelRepoStub) ListModelPricing(context.Context, int64) ([]ChannelModelPricing, error) {
	return nil, nil
}
func (globalPricingServiceChannelRepoStub) CreateModelPricing(context.Context, *ChannelModelPricing) error {
	return nil
}
func (globalPricingServiceChannelRepoStub) UpdateModelPricing(context.Context, *ChannelModelPricing) error {
	return nil
}
func (globalPricingServiceChannelRepoStub) DeleteModelPricing(context.Context, int64) error {
	return nil
}
func (globalPricingServiceChannelRepoStub) ReplaceModelPricing(context.Context, int64, []ChannelModelPricing) error {
	return nil
}

func TestGlobalModelPricingListPrefersOverrideProvider(t *testing.T) {
	ctx := context.Background()
	const model = "zz-provider-switch-test"
	repo := &globalPricingServiceRepoStub{
		overrides: []GlobalModelPricing{{
			ID:          1,
			Model:       model,
			Provider:    PlatformOpenAI,
			BillingMode: BillingModeToken,
			Enabled:     true,
		}},
	}
	pricingService := &PricingService{
		pricingData: map[string]*LiteLLMModelPricing{
			model: {
				LiteLLMProvider:    PlatformAnthropic,
				InputCostPerToken:  1,
				OutputCostPerToken: 2,
			},
		},
	}
	channelService := NewChannelService(globalPricingServiceChannelRepoStub{}, nil, nil, nil)
	svc := NewGlobalModelPricingService(repo, NewGlobalPricingCache(repo), pricingService, channelService, nil, nil)

	openAIResult, err := svc.ListAllModels(ctx, pagination.PaginationParams{Page: 1, PageSize: 10000}, "", PlatformOpenAI, "")
	require.NoError(t, err)
	openAIItem := findModelPricingListItem(openAIResult.Items, model)
	require.NotNil(t, openAIItem)
	require.Equal(t, PlatformOpenAI, openAIItem.Provider)
	require.Equal(t, PlatformOpenAI, openAIItem.GlobalOverride.Provider)

	anthropicResult, err := svc.ListAllModels(ctx, pagination.PaginationParams{Page: 1, PageSize: 10000}, "", PlatformAnthropic, "")
	require.NoError(t, err)
	require.Nil(t, findModelPricingListItem(anthropicResult.Items, model))

	detail, err := svc.GetModelDetail(ctx, model)
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, detail.Provider)
}

func findModelPricingListItem(items []ModelPricingListItem, model string) *ModelPricingListItem {
	for i := range items {
		if strings.EqualFold(items[i].Model, model) {
			return &items[i]
		}
	}
	return nil
}
