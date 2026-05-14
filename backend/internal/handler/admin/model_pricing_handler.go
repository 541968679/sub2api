package admin

import (
	"bytes"
	"encoding/json"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPricingHandler 模型定价管理 HTTP 处理器
type ModelPricingHandler struct {
	svc *service.GlobalModelPricingService
}

// NewModelPricingHandler 创建模型定价管理处理器
func NewModelPricingHandler(svc *service.GlobalModelPricingService) *ModelPricingHandler {
	return &ModelPricingHandler{svc: svc}
}

// --- Request types ---

type createGlobalOverrideRequest struct {
	Model                   string                          `json:"model" binding:"required,max=255"`
	Provider                string                          `json:"provider" binding:"omitempty,max=50"`
	BillingMode             string                          `json:"billing_mode" binding:"omitempty,oneof=token per_request image"`
	InputPrice              *float64                        `json:"input_price" binding:"omitempty,min=0"`
	OutputPrice             *float64                        `json:"output_price" binding:"omitempty,min=0"`
	CacheWritePrice         *float64                        `json:"cache_write_price" binding:"omitempty,min=0"`
	CacheReadPrice          *float64                        `json:"cache_read_price" binding:"omitempty,min=0"`
	ImageOutputPrice        *float64                        `json:"image_output_price" binding:"omitempty,min=0"`
	PerRequestPrice         *float64                        `json:"per_request_price" binding:"omitempty,min=0"`
	ImagePrice1K            *float64                        `json:"image_price_1k" binding:"omitempty,min=0"`
	ImagePrice2K            *float64                        `json:"image_price_2k" binding:"omitempty,min=0"`
	ImagePrice4K            *float64                        `json:"image_price_4k" binding:"omitempty,min=0"`
	ImageBillingStrategy    string                          `json:"image_billing_strategy" binding:"omitempty,oneof=tier megapixel"`
	ImageMegapixelPrice     *float64                        `json:"image_megapixel_price" binding:"omitempty,min=0"`
	ImageQualityPrices      service.ImageQualityPrices      `json:"image_quality_prices"`
	ImageQualityMultipliers service.ImageQualityMultipliers `json:"image_quality_multipliers"`
	ImageTierRules          []service.ImageTierRule         `json:"image_tier_rules"`
	Enabled                 *bool                           `json:"enabled"`
	Notes                   string                          `json:"notes"`

	DisplayInputPrice     *float64 `json:"display_input_price" binding:"omitempty,min=0"`
	DisplayOutputPrice    *float64 `json:"display_output_price" binding:"omitempty,min=0"`
	DisplayCacheReadPrice *float64 `json:"display_cache_read_price" binding:"omitempty,min=0"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier" binding:"omitempty,min=0"`

	ShowOnPricingPage *bool `json:"show_on_pricing_page"`
}

type updateGlobalOverrideRequest struct {
	Model            string   `json:"model" binding:"omitempty,max=255"`
	Provider         string   `json:"provider" binding:"omitempty,max=50"`
	BillingMode      string   `json:"billing_mode" binding:"omitempty,oneof=token per_request image"`
	InputPrice       *float64 `json:"input_price" binding:"omitempty,min=0"`
	OutputPrice      *float64 `json:"output_price" binding:"omitempty,min=0"`
	CacheWritePrice  *float64 `json:"cache_write_price" binding:"omitempty,min=0"`
	CacheReadPrice   *float64 `json:"cache_read_price" binding:"omitempty,min=0"`
	ImageOutputPrice *float64 `json:"image_output_price" binding:"omitempty,min=0"`
	PerRequestPrice  *float64 `json:"per_request_price" binding:"omitempty,min=0"`
	ImagePrice1K     *float64 `json:"image_price_1k" binding:"omitempty,min=0"`
	ImagePrice2K     *float64 `json:"image_price_2k" binding:"omitempty,min=0"`
	ImagePrice4K     *float64 `json:"image_price_4k" binding:"omitempty,min=0"`
	Enabled          *bool    `json:"enabled"`
	Notes            string   `json:"notes"`

	DisplayInputPrice     *float64 `json:"display_input_price"`
	DisplayOutputPrice    *float64 `json:"display_output_price"`
	DisplayCacheReadPrice *float64 `json:"display_cache_read_price"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier"`

	ShowOnPricingPage *bool `json:"show_on_pricing_page"`
}

type updateGlobalOverridePayload map[string]json.RawMessage

// List 列出所有模型及其定价信息（合并 LiteLLM + 全局覆盖）
func (h *ModelPricingHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := c.Query("search")
	provider := c.Query("provider")
	source := c.Query("source")

	params := pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.svc.ListAllModels(c.Request.Context(), params, search, provider, source)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// GetDetail 获取单个模型的定价详情
func (h *ModelPricingHandler) GetDetail(c *gin.Context) {
	model := c.Param("model")
	if model == "" {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_MODEL", "Model name is required"))
		return
	}

	detail, err := h.svc.GetModelDetail(c.Request.Context(), model)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, detail)
}

// CreateOverride 创建全局定价覆盖
func (h *ModelPricingHandler) CreateOverride(c *gin.Context) {
	var req createGlobalOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	showOnPricingPage := false
	if req.ShowOnPricingPage != nil {
		showOnPricingPage = *req.ShowOnPricingPage
	}

	pricing := &service.GlobalModelPricing{
		Model:                   req.Model,
		Provider:                req.Provider,
		BillingMode:             service.BillingMode(req.BillingMode),
		InputPrice:              req.InputPrice,
		OutputPrice:             req.OutputPrice,
		CacheWritePrice:         req.CacheWritePrice,
		CacheReadPrice:          req.CacheReadPrice,
		ImageOutputPrice:        req.ImageOutputPrice,
		PerRequestPrice:         req.PerRequestPrice,
		ImagePrice1K:            req.ImagePrice1K,
		ImagePrice2K:            req.ImagePrice2K,
		ImagePrice4K:            req.ImagePrice4K,
		ImageBillingStrategy:    service.ImageBillingStrategy(req.ImageBillingStrategy),
		ImageMegapixelPrice:     req.ImageMegapixelPrice,
		ImageQualityPrices:      req.ImageQualityPrices,
		ImageQualityMultipliers: req.ImageQualityMultipliers,
		ImageTierRules:          req.ImageTierRules,
		Enabled:                 enabled,
		Notes:                   req.Notes,

		DisplayInputPrice:     req.DisplayInputPrice,
		DisplayOutputPrice:    req.DisplayOutputPrice,
		DisplayCacheReadPrice: req.DisplayCacheReadPrice,
		DisplayRateMultiplier: req.DisplayRateMultiplier,

		ShowOnPricingPage: showOnPricingPage,
	}

	if err := h.svc.CreateOverride(c.Request.Context(), pricing); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, service.ToGlobalOverride(pricing))
}

// UpdateOverride 更新全局定价覆盖
func (h *ModelPricingHandler) UpdateOverride(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid override ID"))
		return
	}

	existing, err := h.svc.GetOverrideByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.ErrorFrom(c, infraerrors.NotFound("NOT_FOUND", "Global override not found"))
		return
	}

	var raw updateGlobalOverridePayload
	if err := c.ShouldBindJSON(&raw); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	if err := applyGlobalOverrideUpdate(existing, raw); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	if err := h.svc.UpdateOverride(c.Request.Context(), existing); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, service.ToGlobalOverride(existing))
}

func applyGlobalOverrideUpdate(existing *service.GlobalModelPricing, raw updateGlobalOverridePayload) error {
	if existing == nil {
		return nil
	}
	if v, ok := raw["model"]; ok {
		var value string
		if err := json.Unmarshal(v, &value); err != nil {
			return err
		}
		if value != "" {
			existing.Model = value
		}
	}
	if v, ok := raw["provider"]; ok {
		var value string
		if err := json.Unmarshal(v, &value); err != nil {
			return err
		}
		existing.Provider = value
	}
	if v, ok := raw["billing_mode"]; ok {
		var value string
		if err := json.Unmarshal(v, &value); err != nil {
			return err
		}
		if value != "" {
			mode := service.BillingMode(value)
			if !mode.IsValid() {
				return infraerrors.BadRequest("INVALID_BILLING_MODE", "invalid billing_mode")
			}
			existing.BillingMode = mode
		}
	}

	applyFloat := func(key string, target **float64) error {
		v, ok := raw[key]
		if !ok {
			return nil
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			*target = nil
			return nil
		}
		var value float64
		if err := json.Unmarshal(v, &value); err != nil {
			return err
		}
		if value < 0 {
			return infraerrors.BadRequest("INVALID_PRICE", key+" must be >= 0")
		}
		*target = &value
		return nil
	}
	for key, target := range map[string]**float64{
		"input_price":              &existing.InputPrice,
		"output_price":             &existing.OutputPrice,
		"cache_write_price":        &existing.CacheWritePrice,
		"cache_read_price":         &existing.CacheReadPrice,
		"image_output_price":       &existing.ImageOutputPrice,
		"per_request_price":        &existing.PerRequestPrice,
		"image_price_1k":           &existing.ImagePrice1K,
		"image_price_2k":           &existing.ImagePrice2K,
		"image_price_4k":           &existing.ImagePrice4K,
		"image_megapixel_price":    &existing.ImageMegapixelPrice,
		"display_input_price":      &existing.DisplayInputPrice,
		"display_output_price":     &existing.DisplayOutputPrice,
		"display_cache_read_price": &existing.DisplayCacheReadPrice,
		"display_rate_multiplier":  &existing.DisplayRateMultiplier,
	} {
		if err := applyFloat(key, target); err != nil {
			return err
		}
	}

	if v, ok := raw["image_billing_strategy"]; ok {
		var value string
		if err := json.Unmarshal(v, &value); err != nil {
			return err
		}
		existing.ImageBillingStrategy = service.NormalizeImageBillingStrategy(service.ImageBillingStrategy(value))
	}
	if v, ok := raw["image_quality_prices"]; ok {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			existing.ImageQualityPrices = nil
		} else {
			var value service.ImageQualityPrices
			if err := json.Unmarshal(v, &value); err != nil {
				return err
			}
			existing.ImageQualityPrices = value
		}
	}
	if v, ok := raw["image_quality_multipliers"]; ok {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			existing.ImageQualityMultipliers = nil
		} else {
			var value service.ImageQualityMultipliers
			if err := json.Unmarshal(v, &value); err != nil {
				return err
			}
			existing.ImageQualityMultipliers = value
		}
	}
	if v, ok := raw["image_tier_rules"]; ok {
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			existing.ImageTierRules = nil
		} else {
			var value []service.ImageTierRule
			if err := json.Unmarshal(v, &value); err != nil {
				return err
			}
			existing.ImageTierRules = value
		}
	}
	if v, ok := raw["enabled"]; ok {
		var value bool
		if err := json.Unmarshal(v, &value); err != nil {
			return err
		}
		existing.Enabled = value
	}
	if v, ok := raw["show_on_pricing_page"]; ok {
		var value bool
		if err := json.Unmarshal(v, &value); err != nil {
			return err
		}
		existing.ShowOnPricingPage = value
	}
	if v, ok := raw["notes"]; ok {
		var value string
		if err := json.Unmarshal(v, &value); err != nil {
			return err
		}
		existing.Notes = value
	}
	return nil
}

// DeleteOverride 删除全局定价覆盖
func (h *ModelPricingHandler) DeleteOverride(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid override ID"))
		return
	}

	if err := h.svc.DeleteOverride(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, nil)
}

// GetChannelOverrides 获取覆盖指定模型的渠道列表
func (h *ModelPricingHandler) GetChannelOverrides(c *gin.Context) {
	model := c.Param("model")
	if model == "" {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_MODEL", "Model name is required"))
		return
	}

	detail, err := h.svc.GetModelDetail(c.Request.Context(), model)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, detail.ChannelOverrides)
}

// GetRateMultiplierOverview 获取分组费率乘数概览
func (h *ModelPricingHandler) GetRateMultiplierOverview(c *gin.Context) {
	result, err := h.svc.GetRateMultiplierOverview(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}
