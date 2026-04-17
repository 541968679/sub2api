package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// GlobalModelPricingService 全局模型定价管理服务
type GlobalModelPricingService struct {
	repo                 GlobalModelPricingRepository
	cache                *GlobalPricingCache
	pricingService       *PricingService
	channelService       *ChannelService
	groupRepo            GroupRepository
	userModelPricingRepo UserModelPricingRepository
}

// NewGlobalModelPricingService 创建全局模型定价管理服务实例
func NewGlobalModelPricingService(
	repo GlobalModelPricingRepository,
	cache *GlobalPricingCache,
	pricingService *PricingService,
	channelService *ChannelService,
	groupRepo GroupRepository,
	userModelPricingRepo UserModelPricingRepository,
) *GlobalModelPricingService {
	return &GlobalModelPricingService{
		repo:                 repo,
		cache:                cache,
		pricingService:       pricingService,
		channelService:       channelService,
		groupRepo:            groupRepo,
		userModelPricingRepo: userModelPricingRepo,
	}
}

// ModelPricingListItem 模型定价列表项（管理后台用）
type ModelPricingListItem struct {
	Model                string            `json:"model"`
	Provider             string            `json:"provider"`
	LiteLLMPrices        *LiteLLMPrices    `json:"litellm_prices"`
	GlobalOverride       *GlobalOverride   `json:"global_override"`
	ChannelOverrideCount int               `json:"channel_override_count"`
	UserOverrideCount    int               `json:"user_override_count"`
	EffectiveSource      string            `json:"effective_source"` // "global", "litellm", "fallback"
	BillingBasisHint     *BillingBasisHint `json:"billing_basis_hint,omitempty"`
}

// BillingBasisHint 说明此模型在平台级模型映射（当前仅 Antigravity 默认映射）中
// 扮演的角色。对应"模型名三元组"（客户端请求名 / 上游名 / 计费名）中的前两维。
//
// Type 取值：
//
//	requested_equals_upstream —— 请求名 == 上游名（同名映射或普通直通模型；related_models 为空）
//	upstream_only             —— 只扮演"上游名"角色，客户端不直接请求它；related_models 列出所有映射源请求名
//	requested_only            —— 只扮演"请求名"角色，被映射到别的名字发给上游；related_models 通常只有一个（映射目标上游名）
//
// 对不在 Antigravity 默认映射里的普通 LiteLLM 模型，徽标为 nil，不展示。
// upstream_only 情况下可能多对一（多个请求名映射到同一个上游名），前端按首个 + "+N" 呈现，
// tooltip 展开全部 RelatedModels。
type BillingBasisHint struct {
	Type          string   `json:"type"`
	RelatedModels []string `json:"related_models,omitempty"`
}

const (
	BillingHintRequestedEqualsUpstream = "requested_equals_upstream"
	BillingHintUpstreamOnly            = "upstream_only"
	BillingHintRequestedOnly           = "requested_only"
)

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

	DisplayInputPrice     *float64 `json:"display_input_price"`
	DisplayOutputPrice    *float64 `json:"display_output_price"`
	DisplayCacheReadPrice *float64 `json:"display_cache_read_price"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier"`
	CacheTransferRatio    *float64 `json:"cache_transfer_ratio"`
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
	UserOverrideCount    int `json:"user_override_count"`
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

	// 3b. 获取用户覆盖计数
	userOverrideCounts := s.getUserOverrideCounts(ctx)

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
			UserOverrideCount:    userOverrideCounts[modelLower],
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
			item.GlobalOverride = ToGlobalOverride(gp)
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
		modelSet[modelLower] = true
		item := ModelPricingListItem{
			Model:                gp.Model,
			Provider:             gp.Provider,
			GlobalOverride:       ToGlobalOverride(&gp),
			ChannelOverrideCount: channelOverrideCounts[modelLower],
			UserOverrideCount:    userOverrideCounts[modelLower],
			EffectiveSource:      PricingSourceFallback,
		}
		if gp.Enabled {
			item.EffectiveSource = PricingSourceGlobal
		}
		items = append(items, item)
	}

	// 补全 Antigravity 可用模型：有些模型（如 gemini-3-pro-high、tab_flash_lite_preview）
	// 只存在于 DefaultAntigravityModelMapping 里，LiteLLM 和全局覆盖都没有。为了让
	// provider=antigravity 过滤时这些模型能被用户看到并管理定价，补一条 stub。
	antigravityMapping := domain.ResolveAntigravityDefaultMapping()
	for requestModel := range antigravityMapping {
		modelLower := strings.ToLower(requestModel)
		if modelSet[modelLower] {
			continue
		}
		modelSet[modelLower] = true
		items = append(items, ModelPricingListItem{
			Model:                requestModel,
			Provider:             "antigravity",
			ChannelOverrideCount: channelOverrideCounts[modelLower],
			UserOverrideCount:    userOverrideCounts[modelLower],
			EffectiveSource:      PricingSourceFallback,
		})
	}

	// 基于 Antigravity 默认映射构造三分法徽标索引。徽标含义是"此模型名在平台级
	// 模型映射里扮演的角色"，对应用户心智模型中的"客户端请求名 / 上游名"二元组
	// （计费名由顶部 banner 统一解释）。
	//
	// 优先级：same_name > upstream_only > requested_only。
	// 理由：同名映射表达该模型自洽（既是请求名又是上游名），信息最完整；其次
	// "仅上游"对管理员更重要（改它会影响映射源请求）；最后才是"仅请求"（这类
	// 配置其实无效——请求会被映射走）。
	//
	// 多对一场景：Antigravity 默认映射里可能有多个请求名映射到同一个上游名
	// （如 claude-opus-4-5-thinking / claude-opus-4-5-20251101 / claude-opus-4-6
	// 都映射到 claude-opus-4-6-thinking）。upstream_only 徽标的 RelatedModels
	// 收集所有映射源请求名，保持字典序稳定。
	type hintBucket struct {
		sameName           bool
		upstreamFromList   []string // 若非空：此模型是 value，收集所有来源 key
		requestedTargetKey string   // 若非空：此模型是 key，映射到该 value
	}
	hintIndex := make(map[string]*hintBucket, len(antigravityMapping))
	getBucket := func(lower string) *hintBucket {
		b := hintIndex[lower]
		if b == nil {
			b = &hintBucket{}
			hintIndex[lower] = b
		}
		return b
	}
	for k, v := range antigravityMapping {
		kl := strings.ToLower(k)
		vl := strings.ToLower(v)
		if kl == vl {
			getBucket(kl).sameName = true
			continue
		}
		// key != value
		if getBucket(kl).requestedTargetKey == "" {
			getBucket(kl).requestedTargetKey = v
		}
		b := getBucket(vl)
		b.upstreamFromList = append(b.upstreamFromList, k)
	}
	// 稳定排序 upstreamFromList，避免 map 遍历顺序导致前端展示跳动
	for _, b := range hintIndex {
		if len(b.upstreamFromList) > 1 {
			sort.Strings(b.upstreamFromList)
		}
	}

	for i := range items {
		b, ok := hintIndex[strings.ToLower(items[i].Model)]
		if !ok {
			continue
		}
		switch {
		case b.sameName:
			// 同名优先：request == upstream；但若同时有其他请求名映射到它
			// （真实的 Antigravity 默认映射几乎总是这种情况，例如
			// `claude-opus-4-6-thinking` 既有同名自映射也是 claude-opus-4-6 的目标），
			// 用 RelatedModels 额外承载"被哪些请求名映射到"，前端展示 +N 提示。
			hint := &BillingBasisHint{Type: BillingHintRequestedEqualsUpstream}
			if len(b.upstreamFromList) > 0 {
				hint.RelatedModels = b.upstreamFromList
			}
			items[i].BillingBasisHint = hint
		case len(b.upstreamFromList) > 0:
			items[i].BillingBasisHint = &BillingBasisHint{
				Type:          BillingHintUpstreamOnly,
				RelatedModels: b.upstreamFromList,
			}
		case b.requestedTargetKey != "":
			items[i].BillingBasisHint = &BillingBasisHint{
				Type:          BillingHintRequestedOnly,
				RelatedModels: []string{b.requestedTargetKey},
			}
		}
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
	for _, count := range userOverrideCounts {
		if count > 0 {
			stats.UserOverrideCount++
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
	Model            string                   `json:"model"`
	Provider         string                   `json:"provider"`
	LiteLLMPrices    *LiteLLMPrices           `json:"litellm_prices"`
	GlobalOverride   *GlobalOverride          `json:"global_override"`
	ChannelOverrides []ChannelOverrideSummary `json:"channel_overrides"`
	UserOverrides    []UserOverrideSummary    `json:"user_overrides"`
	// SuggestedPrices 当模型既无 LiteLLM 数据又无全局覆盖时，按命名近似推断的建议价
	SuggestedPrices *LiteLLMPrices `json:"suggested_prices,omitempty"`
	// SuggestedFrom 建议价来自哪个模型（用于前端提示"来自 xxx"）
	SuggestedFrom string `json:"suggested_from,omitempty"`
}

// UserOverrideSummary 用户级定价覆盖摘要
type UserOverrideSummary struct {
	OverrideID            int64    `json:"override_id"`
	UserID                int64    `json:"user_id"`
	UserEmail             string   `json:"user_email"`
	UserName              string   `json:"user_name"`
	InputPrice            *float64 `json:"input_price"`
	OutputPrice           *float64 `json:"output_price"`
	CacheWritePrice       *float64 `json:"cache_write_price"`
	CacheReadPrice        *float64 `json:"cache_read_price"`
	DisplayInputPrice     *float64 `json:"display_input_price"`
	DisplayOutputPrice    *float64 `json:"display_output_price"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier"`
	CacheTransferRatio    *float64 `json:"cache_transfer_ratio"`
	Enabled               bool     `json:"enabled"`
	Notes                 string   `json:"notes"`
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
	// 注意：pricingService.GetModelPricing 带模糊匹配，会把 "gpt-oss-120b-medium" 这种
	// Antigravity 专有模型错误匹配到不相关的模型。这里先判断 model 是否是 Antigravity
	// 专有 stub（出现在映射表 keys 里但不在 LiteLLM 精确 keys 里），是则跳过 LiteLLM
	// 查表，避免误导用户。这与 ListAllModels 的精确匹配语义保持一致。
	if !s.isAntigravityStubModel(model) {
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
	}

	// 全局覆盖
	gp, err := s.repo.GetByModel(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("get global override: %w", err)
	}
	if gp != nil {
		detail.GlobalOverride = ToGlobalOverride(gp)
		if detail.Provider == "" {
			detail.Provider = gp.Provider
		}
	}

	// 渠道覆盖
	detail.ChannelOverrides = s.getChannelOverridesForModel(ctx, model)

	// 用户覆盖
	detail.UserOverrides = s.getUserOverridesForModel(ctx, model)

	// 建议价：仅当既无 LiteLLM 数据又无全局覆盖时（典型是 Antigravity 专有 stub）
	// 才尝试按命名近似推断一个建议价，供管理员一键填入
	if detail.LiteLLMPrices == nil && detail.GlobalOverride == nil {
		if suggested, from := s.suggestPricing(model); suggested != nil {
			detail.SuggestedPrices = suggested
			detail.SuggestedFrom = from
		}
	}

	return detail, nil
}

// antigravityProprietarySuggestMap 为无法用前缀规则推断的 Antigravity 专有模型
// 提供显式建议价来源。每次 Google / OpenAI 发新版本时可能需要补条目。
var antigravityProprietarySuggestMap = map[string]string{
	"tab_flash_lite_preview": "gemini-2.5-flash-lite",
	"gpt-oss-120b-medium":    "gpt-4o-mini",
}

// isAntigravityStubModel 判断模型是否是 Antigravity 专有的 stub（即出现在
// DefaultAntigravityModelMapping 的 key 里，但不存在于 LiteLLM 精确模型列表）。
// 这类模型被 pricingService.GetModelPricing 的模糊匹配误匹配时会返回完全不相关的
// 价格，详情接口应该把它们视作无 LiteLLM 数据并改走 suggestPricing。
func (s *GlobalModelPricingService) isAntigravityStubModel(model string) bool {
	mapping := domain.ResolveAntigravityDefaultMapping()
	if _, ok := mapping[model]; !ok {
		return false
	}
	// 遍历 LiteLLM 精确模型列表，比对是否存在
	for _, entry := range s.pricingService.GetAllModels() {
		if strings.EqualFold(entry.Model, model) {
			return false
		}
	}
	return true
}

// suggestPricing 按命名近似为模型推断一个建议基准价。
// 返回 (*LiteLLMPrices, 源模型名)。全部匹配失败返回 (nil, "")。
//
// 匹配链（从精确到模糊）：
//  1. 显式映射表 antigravityProprietarySuggestMap
//  2. 剥离 Gemini 档位后缀：-high / -low / -medium（保留 -flash / -pro 家族）
//  3. 剥离 claude -thinking 后缀
//  4. Gemini 版本降级：3 → 2.5；3.1 → 3.x → 2.5
func (s *GlobalModelPricingService) suggestPricing(model string) (*LiteLLMPrices, string) {
	tryFrom := func(name string) (*LiteLLMPrices, string) {
		if name == "" || strings.EqualFold(name, model) {
			return nil, ""
		}
		lp := s.pricingService.GetModelPricing(name)
		if lp == nil {
			return nil, ""
		}
		return &LiteLLMPrices{
			InputPrice:       lp.InputCostPerToken,
			OutputPrice:      lp.OutputCostPerToken,
			CacheWritePrice:  lp.CacheCreationInputTokenCost,
			CacheReadPrice:   lp.CacheReadInputTokenCost,
			ImageOutputPrice: lp.OutputCostPerImageToken,
		}, name
	}

	// 1) 显式映射
	if target, ok := antigravityProprietarySuggestMap[strings.ToLower(model)]; ok {
		if p, from := tryFrom(target); p != nil {
			return p, from
		}
	}

	// 2) 剥离 Gemini 档位后缀（-high / -low / -medium）
	for _, suffix := range []string{"-high", "-low", "-medium"} {
		if strings.HasSuffix(model, suffix) {
			base := strings.TrimSuffix(model, suffix)
			if p, from := tryFrom(base); p != nil {
				return p, from
			}
			// 降级链：gemini-3-pro-high → gemini-3-pro → gemini-2.5-pro
			if downgraded := downgradeGeminiVersion(base); downgraded != "" {
				if p, from := tryFrom(downgraded); p != nil {
					return p, from
				}
			}
		}
	}

	// 3) 剥离 claude -thinking 后缀
	if strings.HasSuffix(model, "-thinking") {
		base := strings.TrimSuffix(model, "-thinking")
		if p, from := tryFrom(base); p != nil {
			return p, from
		}
	}

	// 4) Gemini 版本直接降级
	if downgraded := downgradeGeminiVersion(model); downgraded != "" {
		if p, from := tryFrom(downgraded); p != nil {
			return p, from
		}
	}

	return nil, ""
}

// downgradeGeminiVersion 将 gemini-3.x / gemini-3 系列降级到 gemini-2.5 作为建议价兜底。
// 不认识的命名返回空字符串。
func downgradeGeminiVersion(model string) string {
	lower := strings.ToLower(model)
	if !strings.HasPrefix(lower, "gemini-") {
		return ""
	}
	// gemini-3.1-pro → gemini-2.5-pro
	if strings.HasPrefix(lower, "gemini-3.1-") {
		return "gemini-2.5-" + strings.TrimPrefix(lower, "gemini-3.1-")
	}
	// gemini-3-pro → gemini-2.5-pro
	if strings.HasPrefix(lower, "gemini-3-") {
		return "gemini-2.5-" + strings.TrimPrefix(lower, "gemini-3-")
	}
	return ""
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

	// Antigravity 过滤走"模型名是否在 DefaultAntigravityModelMapping 的 key 集合里"，
	// 因为 Antigravity 平台复用底层模型（provider 实际是 anthropic/gemini/vertex_ai），
	// 只比较 provider 字段会漏掉大部分可用模型。
	var antigravityModelSet map[string]bool
	if providerLower == "antigravity" {
		mapping := domain.ResolveAntigravityDefaultMapping()
		antigravityModelSet = make(map[string]bool, len(mapping))
		for k := range mapping {
			antigravityModelSet[strings.ToLower(k)] = true
		}
	}

	var filtered []ModelPricingListItem
	for _, item := range items {
		if searchLower != "" && !strings.Contains(strings.ToLower(item.Model), searchLower) {
			continue
		}
		if providerLower != "" && !providerMatches(item, providerLower, antigravityModelSet) {
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
			case "has_user_override":
				if item.UserOverrideCount == 0 {
					continue
				}
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// providerMatches 判断单条目是否命中 provider 筛选，处理 LiteLLM 的各种子分类别名。
// 前端下拉传入的是统一的大类名（anthropic/openai/gemini/antigravity），而 LiteLLM JSON
// 里实际值会带后缀（如 vertex_ai-language-models、text-completion-openai），严格相等
// 匹配会漏掉大量模型。Antigravity 特殊处理：按模型名命中 Antigravity 默认映射即算匹配。
func providerMatches(item ModelPricingListItem, providerLower string, antigravityModelSet map[string]bool) bool {
	itemProvider := strings.ToLower(item.Provider)
	switch providerLower {
	case "gemini":
		// LiteLLM 里 Gemini 家族的实际 provider：gemini、vertex_ai-language-models、
		// vertex_ai-vision-models、vertex_ai-embedding-models。
		return itemProvider == "gemini" || strings.HasPrefix(itemProvider, "vertex_ai")
	case "openai":
		// text-completion-openai 是老版本 completion 接口，也应归入 OpenAI 大类。
		return itemProvider == "openai" || itemProvider == "text-completion-openai"
	case "antigravity":
		// 显式覆盖写的 provider=antigravity，或模型名在 Antigravity 默认映射里。
		if itemProvider == "antigravity" {
			return true
		}
		if antigravityModelSet != nil && antigravityModelSet[strings.ToLower(item.Model)] {
			return true
		}
		return false
	default:
		return itemProvider == providerLower
	}
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

func (s *GlobalModelPricingService) getUserOverrideCounts(ctx context.Context) map[string]int {
	if s.userModelPricingRepo == nil {
		return nil
	}
	counts, err := s.userModelPricingRepo.GetEnabledCountByModel(ctx)
	if err != nil {
		return nil
	}
	return counts
}

func (s *GlobalModelPricingService) getUserOverridesForModel(ctx context.Context, model string) []UserOverrideSummary {
	if s.userModelPricingRepo == nil {
		return nil
	}
	overrides, err := s.userModelPricingRepo.GetByModel(ctx, model)
	if err != nil || len(overrides) == 0 {
		return nil
	}
	result := make([]UserOverrideSummary, 0, len(overrides))
	for _, o := range overrides {
		summary := UserOverrideSummary{
			OverrideID:            o.ID,
			UserID:                o.UserID,
			InputPrice:            o.InputPrice,
			OutputPrice:           o.OutputPrice,
			CacheWritePrice:       o.CacheWritePrice,
			CacheReadPrice:        o.CacheReadPrice,
			DisplayInputPrice:     o.DisplayInputPrice,
			DisplayOutputPrice:    o.DisplayOutputPrice,
			DisplayRateMultiplier: o.DisplayRateMultiplier,
			CacheTransferRatio:    o.CacheTransferRatio,
			Enabled:               o.Enabled,
			Notes:                 o.Notes,
		}
		// user email/name lookup would require userRepo; for now return IDs only
		result = append(result, summary)
	}
	return result
}

// ToGlobalOverride 把内部 GlobalModelPricing 实体转为 API 返回用的 GlobalOverride
// （带 json tag 的 snake_case 字段名）。handler 层需要用此函数包装 Create/Update
// 返回值，否则前端收到的是 PascalCase JSON，字段全部 undefined。
func ToGlobalOverride(gp *GlobalModelPricing) *GlobalOverride {
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

		DisplayInputPrice:     gp.DisplayInputPrice,
		DisplayOutputPrice:    gp.DisplayOutputPrice,
		DisplayCacheReadPrice: gp.DisplayCacheReadPrice,
		DisplayRateMultiplier: gp.DisplayRateMultiplier,
		CacheTransferRatio:    gp.CacheTransferRatio,
	}
}

// GetAllEnabledPricings returns all enabled global model pricing entries.
// Used by usage handlers to build the display pricing map.
func (s *GlobalModelPricingService) GetAllEnabledPricings(ctx context.Context) ([]GlobalModelPricing, error) {
	return s.repo.GetAllEnabled(ctx)
}
