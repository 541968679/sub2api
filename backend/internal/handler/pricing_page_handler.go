package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// 默认文案（首次部署 settings 里还没写入时返回给前端）。
// 管理员在后台保存后会覆盖这两段；用户始终看到后台最新的值。
const (
	defaultPricingPageIntro = `## 本站计价模式

我们按 **原厂真实 Token 计价**：每个模型的输入、输出、缓存读取单价都与上游（Anthropic / OpenAI / Google 等）官方价格一致，不加价、不打包、不隐藏倍率。

每一次调用的花费都能在「使用记录」里逐条还原——看得见每一 Token，算得清每一分钱。`

	defaultPricingPageEducation = `## 几种常见计价模式对比

| 模式 | 描述 | 问题 |
|------|------|------|
| **按次计费** | 不管请求多大，每次固定扣一个单位 | 短请求亏，长请求白嫖；平台为了不亏必须把单价定得很高 |
| **统一 Token 价** | 所有模型用同一个假 Token 单价 | 便宜模型被拉贵、昂贵模型被藏起来；用户永远不知道真实成本 |
| **包月不限量** | 预付费换"无限" | 实际限流、降智、偷偷换小模型；到头来你根本不知道自己在用什么 |
| **本站：按原厂 Token 计价** | 与官方单价完全一致，按消耗扣费 | —— |

**我们的目标**：让你像用官方 API 一样透明地消费，同时享受多账号聚合、统一鉴权、自动故障转移带来的便利。`
)

// PricingPageHandler 用户侧「模型计价」页面聚合接口处理器。
// 一次 HTTP 请求返回：两段 Markdown 文案 + 按平台分组的展示价格表。
type PricingPageHandler struct {
	modelPricingSvc     *service.GlobalModelPricingService
	userModelPricingSvc *service.UserModelPricingService
	settingRepo         service.SettingRepository
}

// NewPricingPageHandler 创建用户侧模型计价页处理器
func NewPricingPageHandler(
	modelPricingSvc *service.GlobalModelPricingService,
	userModelPricingSvc *service.UserModelPricingService,
	settingRepo service.SettingRepository,
) *PricingPageHandler {
	return &PricingPageHandler{
		modelPricingSvc:     modelPricingSvc,
		userModelPricingSvc: userModelPricingSvc,
		settingRepo:         settingRepo,
	}
}

// pricingPagePlatform 按平台聚合的价格分组
type pricingPagePlatform struct {
	Provider string              `json:"provider"`
	Models   []pricingPageModel  `json:"models"`
}

// pricingPageModel 单个模型的展示信息
type pricingPageModel struct {
	Model                 string   `json:"model"`
	BillingMode           string   `json:"billing_mode"`
	DisplayInputPrice     *float64 `json:"display_input_price"`
	DisplayOutputPrice    *float64 `json:"display_output_price"`
	DisplayCacheReadPrice *float64 `json:"display_cache_read_price"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier"`
	PerRequestPrice       *float64 `json:"per_request_price"`
}

type pricingPageResponse struct {
	Intro     string                `json:"intro"`
	Education string                `json:"education"`
	Platforms []pricingPagePlatform `json:"platforms"`
}

// Get GET /api/v1/user/pricing-page
// 返回当前用户可见的模型计价信息。文案从 settings 读；模型列表读 show_on_pricing_page=true
// 的 enabled 条目；再合并用户级 display 覆盖得到每个模型的最终展示价格。
func (h *PricingPageHandler) Get(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	ctx := c.Request.Context()

	// 1. 文案
	intro, err := h.loadContent(ctx, admin.SettingKeyPricingPageIntro, defaultPricingPageIntro)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	edu, err := h.loadContent(ctx, admin.SettingKeyPricingPageEducation, defaultPricingPageEducation)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 2. 模型列表（已过滤 enabled && show_on_pricing_page）
	models, err := h.modelPricingSvc.ListForPricingPage(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 3. 合并用户级 display 覆盖。用已启用全局池子构建 global map，再叠加用户 override。
	//    注意：BuildDisplayPricingMap 会过滤掉完全无 display 字段的条目，所以这里我们
	//    同时保留原始 models 列表用于主导分组，仅在 map 中查 display 值。
	globalMap := dto.BuildDisplayPricingMap(models)
	userOverrides, err := h.userModelPricingSvc.ListByUserID(ctx, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	mergedMap := dto.BuildUserDisplayPricingMap(globalMap, userOverrides)

	// 4. 按 provider 分组
	grouped := make(map[string]*pricingPagePlatform)
	order := []string{}
	for i := range models {
		m := &models[i]
		provider := m.Provider
		if provider == "" {
			provider = "other"
		}
		g, ok := grouped[provider]
		if !ok {
			g = &pricingPagePlatform{Provider: provider}
			grouped[provider] = g
			order = append(order, provider)
		}

		item := pricingPageModel{
			Model:           m.Model,
			BillingMode:     string(m.BillingMode),
			PerRequestPrice: m.PerRequestPrice,
		}

		if cfg := mergedMap[strings.ToLower(m.Model)]; cfg != nil {
			item.DisplayInputPrice = cfg.DisplayInputPrice
			item.DisplayOutputPrice = cfg.DisplayOutputPrice
			item.DisplayCacheReadPrice = cfg.DisplayCacheReadPrice
			item.DisplayRateMultiplier = cfg.DisplayRateMultiplier
		}
		// 若 merged map 未包含此模型（说明连 display 字段都没配），回退到 real price：
		// 普通用户看到的还是真实单价，而不是空列。
		if item.DisplayInputPrice == nil {
			item.DisplayInputPrice = m.InputPrice
		}
		if item.DisplayOutputPrice == nil {
			item.DisplayOutputPrice = m.OutputPrice
		}
		if item.DisplayCacheReadPrice == nil {
			item.DisplayCacheReadPrice = m.CacheReadPrice
		}

		g.Models = append(g.Models, item)
	}

	platforms := make([]pricingPagePlatform, 0, len(order))
	for _, p := range order {
		platforms = append(platforms, *grouped[p])
	}

	response.Success(c, pricingPageResponse{
		Intro:     intro,
		Education: edu,
		Platforms: platforms,
	})
}

// loadContent 读取指定 key，未保存时返回 fallback。
func (h *PricingPageHandler) loadContent(ctx context.Context, key, fallback string) (string, error) {
	v, err := h.settingRepo.GetValue(ctx, key)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			return fallback, nil
		}
		return "", err
	}
	if strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	return v, nil
}
