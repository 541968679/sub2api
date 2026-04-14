package admin

import (
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
	Model            string   `json:"model" binding:"required,max=255"`
	Provider         string   `json:"provider" binding:"omitempty,max=50"`
	BillingMode      string   `json:"billing_mode" binding:"omitempty,oneof=token per_request image"`
	InputPrice       *float64 `json:"input_price" binding:"omitempty,min=0"`
	OutputPrice      *float64 `json:"output_price" binding:"omitempty,min=0"`
	CacheWritePrice  *float64 `json:"cache_write_price" binding:"omitempty,min=0"`
	CacheReadPrice   *float64 `json:"cache_read_price" binding:"omitempty,min=0"`
	ImageOutputPrice *float64 `json:"image_output_price" binding:"omitempty,min=0"`
	PerRequestPrice  *float64 `json:"per_request_price" binding:"omitempty,min=0"`
	Enabled          *bool    `json:"enabled"`
	Notes            string   `json:"notes"`
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
	Enabled          *bool    `json:"enabled"`
	Notes            string   `json:"notes"`
}

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

	pricing := &service.GlobalModelPricing{
		Model:            req.Model,
		Provider:         req.Provider,
		BillingMode:      service.BillingMode(req.BillingMode),
		InputPrice:       req.InputPrice,
		OutputPrice:      req.OutputPrice,
		CacheWritePrice:  req.CacheWritePrice,
		CacheReadPrice:   req.CacheReadPrice,
		ImageOutputPrice: req.ImageOutputPrice,
		PerRequestPrice:  req.PerRequestPrice,
		Enabled:          enabled,
		Notes:            req.Notes,
	}

	if err := h.svc.CreateOverride(c.Request.Context(), pricing); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, pricing)
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

	var req updateGlobalOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	// 应用更新：价格字段也采用"非 nil 才覆盖"的差量语义，避免 PATCH 漏带
	// 某个字段时把已有价格意外清零。要清除某个价格，请删除整条覆盖后重建。
	if req.Model != "" {
		existing.Model = req.Model
	}
	if req.Provider != "" {
		existing.Provider = req.Provider
	}
	if req.BillingMode != "" {
		existing.BillingMode = service.BillingMode(req.BillingMode)
	}
	if req.InputPrice != nil {
		existing.InputPrice = req.InputPrice
	}
	if req.OutputPrice != nil {
		existing.OutputPrice = req.OutputPrice
	}
	if req.CacheWritePrice != nil {
		existing.CacheWritePrice = req.CacheWritePrice
	}
	if req.CacheReadPrice != nil {
		existing.CacheReadPrice = req.CacheReadPrice
	}
	if req.ImageOutputPrice != nil {
		existing.ImageOutputPrice = req.ImageOutputPrice
	}
	if req.PerRequestPrice != nil {
		existing.PerRequestPrice = req.PerRequestPrice
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Notes != "" {
		existing.Notes = req.Notes
	}

	if err := h.svc.UpdateOverride(c.Request.Context(), existing); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, existing)
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
