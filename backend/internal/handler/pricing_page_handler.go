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

// 默认文案见 admin.DefaultPricingPageIntro / admin.DefaultPricingPageEducation，
// admin 的 Get 接口与这里共用同一份常量，管理员编辑页里看到的就是用户此刻实际看到的内容。

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
	intro, err := h.loadContent(ctx, admin.SettingKeyPricingPageIntro, admin.DefaultPricingPageIntro)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	edu, err := h.loadContent(ctx, admin.SettingKeyPricingPageEducation, admin.DefaultPricingPageEducation)
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
