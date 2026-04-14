package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// GlobalModelPricingService 全局模型定价管理服务
type GlobalModelPricingService struct {
	repo           GlobalModelPricingRepository
	cache          *GlobalPricingCache
	pricingService *PricingService
	channelService *ChannelService
	groupRepo      GroupRepository
}

// NewGlobalModelPricingService 创建全局模型定价管理服务实例
func NewGlobalModelPricingService(
	repo GlobalModelPricingRepository,
	cache *GlobalPricingCache,
	pricingService *PricingService,
	channelService *ChannelService,
	groupRepo GroupRepository,
) *GlobalModelPricingService {
	return &GlobalModelPricingService{
		repo:           repo,
		cache:          cache,
		pricingService: pricingService,
		channelService: channelService,
		groupRepo:      groupRepo,
	}
}

// ModelPricingListItem 模型定价列表项（管理后台用）
type ModelPricingListItem struct {
	Model                string          `json:"model"`
	Provider             string          `json:"provider"`
	LiteLLMPrices        *LiteLLMPrices  `json:"litellm_prices"`
	GlobalOverride       *GlobalOverride `json:"global_override"`
	ChannelOverrideCount int             `json:"channel_override_count"`
	EffectiveSource      string          `json:"effective_source"` // "global", "litellm", "fallback"
}

// LiteLLMPrices 管理后台展示用的 LiteLLM 价格
type LiteLLMPrices struct {
	InputPrice       float64 `json:"input_price"`
	OutputPrice      float64 `json:"output_price"`
	CacheWritePrice  float64 `json:"cache_write_price"`
	CacheReadPrice   float64 `json:"cache_read_price"`
	ImageOutputPrice float64 `json:"image_output_price"`
}

// GlobalOverride 全局覆盖信息（API 返回用）
type GlobalOverride struct {
	ID               int64    `json:"id"`
	Model            string   `json:"model"`
	Provider         string   `json:"provider"`
	BillingMode      string   `json:"billing_mode"`
	InputPrice       *float64 `json:"input_price"`
	OutputPrice      *float64 `json:"output_price"`
	CacheWritePrice  *float64 `json:"cache_write_price"`
	CacheReadPrice   *float64 `json:"cache_read_price"`
	ImageOutputPrice *float64 `json:"image_output_price"`
	PerRequestPrice  *float64 `json:"per_request_price"`
	Enabled          bool     `json:"enabled"`
	Notes            string   `json:"notes"`
}

// ModelPricingListResult 分页列表结果
type ModelPricingListResult struct {
	Items      []ModelPricingListItem     `json:"items"`
	Pagination *pagination.PaginationResult `json:"pagination"`
	Stats      ModelPricingStats          `json:"stats"`
}

// ModelPricingStats 汇总统计
type ModelPricingStats struct {
	TotalModels          int `json:"total_models"`
	GlobalOverrideCount  int `json:"global_override_count"`
	ChannelOverrideCount int `json:"channel_override_count"`
}

// ListAllModels 合并 LiteLLM + 全局覆盖为统一分页列表
func (s *GlobalModelPricingService) ListAllModels(ctx context.Context, params pagination.PaginationParams, search, provider, source string) (*ModelPricingListResult, error) {
	// 1. 获取所有 LiteLLM 模型
	litellmModels := s.pricingService.GetAllModels()

	// 2. 获取所有全局覆盖
	globalOverrides, err := s.repo.GetAllEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("get global overrides: %w", err)
	}
	// 同时获取所有全局覆盖（含禁用的）用于展示
	allOverrides, _, err := s.repo.List(ctx, pagination.PaginationParams{Page: 1, PageSize: 10000}, "", "")
	if err != nil {
		return nil, fmt.Errorf("list all global overrides: %w", err)
	}
	overrideMap := make(map[string]*GlobalModelPricing, len(allOverrides))
	for i := range allOverrides {
		overrideMap[strings.ToLower(allOverrides[i].Model)] = &allOverrides[i]
	}
	enabledOverrideMap := make(map[string]*GlobalModelPricing, len(globalOverrides))
	for i := range globalOverrides {
		enabledOverrideMap[strings.ToLower(globalOverrides[i].Model)] = &globalOverrides[i]
	}

	// 3. 获取渠道覆盖计数
	channelOverrideCounts := s.getChannelOverrideCounts(ctx)

	// 4. 合并到统一列表
	modelSet := make(map[string]bool)
	var items []ModelPricingListItem

	for _, entry := range litellmModels {
		modelLower := strings.ToLower(entry.Model)
		modelSet[modelLower] = true

		item := ModelPricingListItem{
			Model:                entry.Model,
			Provider:             entry.Provider,
			ChannelOverrideCount: channelOverrideCounts[modelLower],
			EffectiveSource:      PricingSourceLiteLLM,
		}

		// LiteLLM 价格
		if entry.Pricing != nil {
			item.LiteLLMPrices = &LiteLLMPrices{
				InputPrice:       entry.Pricing.InputCostPerToken,
				OutputPrice:      entry.Pricing.OutputCostPerToken,
				CacheWritePrice:  entry.Pricing.CacheCreationInputTokenCost,
				CacheReadPrice:   entry.Pricing.CacheReadInputTokenCost,
				ImageOutputPrice: entry.Pricing.OutputCostPerImageToken,
			}
		}

		// 全局覆盖
		if gp, ok := overrideMap[modelLower]; ok {
			item.GlobalOverride = toGlobalOverride(gp)
			if gp.Enabled {
				item.EffectiveSource = PricingSourceGlobal
			}
		}

		items = append(items, item)
	}

	// 包含只在全局覆盖中存在（不在 LiteLLM 中）的模型
	for _, gp := range allOverrides {
		modelLower := strings.ToLower(gp.Model)
		if modelSet[modelLower] {
			continue
		}
		item := ModelPricingListItem{
			Model:                gp.Model,
			Provider:             gp.Provider,
			GlobalOverride:       toGlobalOverride(&gp),
			ChannelOverrideCount: channelOverrideCounts[modelLower],
			EffectiveSource:      PricingSourceFallback,
		}
		if gp.Enabled {
			item.EffectiveSource = PricingSourceGlobal
		}
		items = append(items, item)
	}

	// 5. 筛选
	items = s.filterItems(items, search, provider, source)

	// 6. 排序
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Model) < strings.ToLower(items[j].Model)
	})

	// 7. 统计
	stats := ModelPricingStats{
		TotalModels:         len(items),
		GlobalOverrideCount: len(allOverrides),
	}
	for _, count := range channelOverrideCounts {
		if count > 0 {
			stats.ChannelOverrideCount++
		}
	}

	// 8. 分页
	total := len(items)
	offset := params.Offset()
	limit := params.Limit()
	if offset >= total {
		items = nil
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		items = items[offset:end]
	}

	pages := total / limit
	if total%limit > 0 {
		pages++
	}

	return &ModelPricingListResult{
		Items: items,
		Pagination: &pagination.PaginationResult{
			Total:    int64(total),
			Page:     params.Page,
			PageSize: limit,
			Pages:    pages,
		},
		Stats: stats,
	}, nil
}

// ModelPricingDetail 单模型定价详情
type ModelPricingDetail struct {
	Model           string                     `json:"model"`
	Provider        string                     `json:"provider"`
	LiteLLMPrices   *LiteLLMPrices             `json:"litellm_prices"`
	GlobalOverride  *GlobalOverride            `json:"global_override"`
	ChannelOverrides []ChannelOverrideSummary  `json:"channel_overrides"`
}

// ChannelOverrideSummary 渠道覆盖摘要
type ChannelOverrideSummary struct {
	ChannelID   int64    `json:"channel_id"`
	ChannelName string   `json:"channel_name"`
	Platform    string   `json:"platform"`
	BillingMode string   `json:"billing_mode"`
	InputPrice  *float64 `json:"input_price"`
	OutputPrice *float64 `json:"output_price"`
}

// GetModelDetail 获取单模型定价详情
func (s *GlobalModelPricingService) GetModelDetail(ctx context.Context, model string) (*ModelPricingDetail, error) {
	detail := &ModelPricingDetail{
		Model: model,
	}

	// LiteLLM 价格
	litellmPricing := s.pricingService.GetModelPricing(model)
	if litellmPricing != nil {
		detail.Provider = litellmPricing.LiteLLMProvider
		detail.LiteLLMPrices = &LiteLLMPrices{
			InputPrice:       litellmPricing.InputCostPerToken,
			OutputPrice:      litellmPricing.OutputCostPerToken,
			CacheWritePrice:  litellmPricing.CacheCreationInputTokenCost,
			CacheReadPrice:   litellmPricing.CacheReadInputTokenCost,
			ImageOutputPrice: litellmPricing.OutputCostPerImageToken,
		}
	}

	// 全局覆盖
	gp, err := s.repo.GetByModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("get global override: %w", err)
	}
	if gp != nil {
		detail.GlobalOverride = toGlobalOverride(gp)
		if detail.Provider == "" {
			detail.Provider = gp.Provider
		}
	}

	// 渠道覆盖
	detail.ChannelOverrides = s.getChannelOverridesForModel(ctx, model)

	return detail, nil
}

// CreateOverride 创建全局定价覆盖
// CUD 操作必须 invalidate 缓存，否则热路径会继续用旧值计费。
func (s *GlobalModelPricingService) CreateOverride(ctx context.Context, pricing *GlobalModelPricing) error {
	if err := s.repo.Create(ctx, pricing); err != nil {
		return err
	}
	s.cache.Invalidate()
	return nil
}

// UpdateOverride 更新全局定价覆盖
func (s *GlobalModelPricingService) UpdateOverride(ctx context.Context, pricing *GlobalModelPricing) error {
	if err := s.repo.Update(ctx, pricing); err != nil {
		return err
	}
	s.cache.Invalidate()
	return nil
}

// DeleteOverride 删除全局定价覆盖
func (s *GlobalModelPricingService) DeleteOverride(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.cache.Invalidate()
	return nil
}

// GetOverrideByID 按 ID 获取全局覆盖
func (s *GlobalModelPricingService) GetOverrideByID(ctx context.Context, id int64) (*GlobalModelPricing, error) {
	return s.repo.GetByID(ctx, id)
}

// RateMultiplierSummary 费率乘数汇总
type RateMultiplierSummary struct {
	GroupID        int64   `json:"group_id"`
	GroupName      string  `json:"group_name"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

// GetRateMultiplierOverview 获取分组费率乘数概览
func (s *GlobalModelPricingService) GetRateMultiplierOverview(ctx context.Context) ([]RateMultiplierSummary, error) {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}

	var result []RateMultiplierSummary
	for _, g := range groups {
		rm := g.RateMultiplier
		if rm == 0 {
			rm = 1.0
		}
		result = append(result, RateMultiplierSummary{
			GroupID:        g.ID,
			GroupName:      g.Name,
			RateMultiplier: rm,
		})
	}
	return result, nil
}

// --- 内部辅助方法 ---

func (s *GlobalModelPricingService) filterItems(items []ModelPricingListItem, search, provider, source string) []ModelPricingListItem {
	if search == "" && provider == "" && source == "" {
		return items
	}

	searchLower := strings.ToLower(search)
	providerLower := strings.ToLower(provider)

	var filtered []ModelPricingListItem
	for _, item := range items {
		if searchLower != "" && !strings.Contains(strings.ToLower(item.Model), searchLower) {
			continue
		}
		if providerLower != "" && strings.ToLower(item.Provider) != providerLower {
			continue
		}
		if source != "" {
			switch source {
			case "litellm_only":
				if item.GlobalOverride != nil {
					continue
				}
			case "has_global_override":
				if item.GlobalOverride == nil {
					continue
				}
			case "has_channel_override":
				if item.ChannelOverrideCount == 0 {
					continue
				}
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (s *GlobalModelPricingService) getChannelOverrideCounts(ctx context.Context) map[string]int {
	counts := make(map[string]int)

	// 获取所有渠道
	allChannels, _, err := s.channelService.List(ctx, pagination.PaginationParams{Page: 1, PageSize: 10000}, "", "")
	if err != nil {
		return counts
	}

	for _, ch := range allChannels {
		for _, pricing := range ch.ModelPricing {
			for _, m := range pricing.Models {
				counts[strings.ToLower(m)]++
			}
		}
	}
	return counts
}

func (s *GlobalModelPricingService) getChannelOverridesForModel(ctx context.Context, model string) []ChannelOverrideSummary {
	modelLower := strings.ToLower(model)
	var result []ChannelOverrideSummary

	allChannels, _, err := s.channelService.List(ctx, pagination.PaginationParams{Page: 1, PageSize: 10000}, "", "")
	if err != nil {
		return result
	}

	for _, ch := range allChannels {
		for _, pricing := range ch.ModelPricing {
			for _, m := range pricing.Models {
				if strings.ToLower(m) == modelLower {
					result = append(result, ChannelOverrideSummary{
						ChannelID:   ch.ID,
						ChannelName: ch.Name,
						Platform:    pricing.Platform,
						BillingMode: string(pricing.BillingMode),
						InputPrice:  pricing.InputPrice,
						OutputPrice: pricing.OutputPrice,
					})
					break
				}
			}
		}
	}
	return result
}

func toGlobalOverride(gp *GlobalModelPricing) *GlobalOverride {
	return &GlobalOverride{
		ID:               gp.ID,
		Model:            gp.Model,
		Provider:         gp.Provider,
		BillingMode:      string(gp.BillingMode),
		InputPrice:       gp.InputPrice,
		OutputPrice:      gp.OutputPrice,
		CacheWritePrice:  gp.CacheWritePrice,
		CacheReadPrice:   gp.CacheReadPrice,
		ImageOutputPrice: gp.ImageOutputPrice,
		PerRequestPrice:  gp.PerRequestPrice,
		Enabled:          gp.Enabled,
		Notes:            gp.Notes,
	}
}
