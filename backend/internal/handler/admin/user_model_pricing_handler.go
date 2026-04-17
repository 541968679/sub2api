package admin

import (
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// UserModelPricingHandler 用户模型定价覆盖管理
type UserModelPricingHandler struct {
	svc *service.UserModelPricingService
}

func NewUserModelPricingHandler(svc *service.UserModelPricingService) *UserModelPricingHandler {
	return &UserModelPricingHandler{svc: svc}
}

type createUserModelPricingRequest struct {
	Model                 string   `json:"model" binding:"required,max=255"`
	InputPrice            *float64 `json:"input_price" binding:"omitempty,min=0"`
	OutputPrice           *float64 `json:"output_price" binding:"omitempty,min=0"`
	CacheWritePrice       *float64 `json:"cache_write_price" binding:"omitempty,min=0"`
	CacheReadPrice        *float64 `json:"cache_read_price" binding:"omitempty,min=0"`
	DisplayInputPrice     *float64 `json:"display_input_price" binding:"omitempty,min=0"`
	DisplayOutputPrice    *float64 `json:"display_output_price" binding:"omitempty,min=0"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier" binding:"omitempty,min=0"`
	CacheTransferRatio    *float64 `json:"cache_transfer_ratio" binding:"omitempty,min=0,max=1"`
	Enabled               *bool    `json:"enabled"`
	Notes                 string   `json:"notes"`
}

type updateUserModelPricingRequest struct {
	Model                 string   `json:"model" binding:"omitempty,max=255"`
	InputPrice            *float64 `json:"input_price"`
	OutputPrice           *float64 `json:"output_price"`
	CacheWritePrice       *float64 `json:"cache_write_price"`
	CacheReadPrice        *float64 `json:"cache_read_price"`
	DisplayInputPrice     *float64 `json:"display_input_price"`
	DisplayOutputPrice    *float64 `json:"display_output_price"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier"`
	CacheTransferRatio    *float64 `json:"cache_transfer_ratio"`
	Enabled               *bool    `json:"enabled"`
	Notes                 *string  `json:"notes"`
}

type batchUpsertUserModelPricingRequest struct {
	Overrides []createUserModelPricingRequest `json:"overrides" binding:"required"`
}

// List GET /api/v1/admin/users/:id/model-pricing
func (h *UserModelPricingHandler) List(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid user ID"))
		return
	}

	overrides, err := h.svc.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, overrides)
}

// Create POST /api/v1/admin/users/:id/model-pricing
func (h *UserModelPricingHandler) Create(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid user ID"))
		return
	}

	var req createUserModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	o := &service.UserModelPricingOverride{
		UserID:                userID,
		Model:                 req.Model,
		InputPrice:            req.InputPrice,
		OutputPrice:           req.OutputPrice,
		CacheWritePrice:       req.CacheWritePrice,
		CacheReadPrice:        req.CacheReadPrice,
		DisplayInputPrice:     req.DisplayInputPrice,
		DisplayOutputPrice:    req.DisplayOutputPrice,
		DisplayRateMultiplier: req.DisplayRateMultiplier,
		CacheTransferRatio:    req.CacheTransferRatio,
		Enabled:               enabled,
		Notes:                 req.Notes,
	}

	if err := h.svc.Create(c.Request.Context(), o); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, o)
}

// Update PUT /api/v1/admin/users/:id/model-pricing/:overrideId
func (h *UserModelPricingHandler) Update(c *gin.Context) {
	overrideID, err := strconv.ParseInt(c.Param("overrideId"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid override ID"))
		return
	}

	existing, err := h.svc.GetByID(c.Request.Context(), overrideID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.ErrorFrom(c, infraerrors.NotFound("NOT_FOUND", "Override not found"))
		return
	}

	var req updateUserModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	if req.Model != "" {
		existing.Model = req.Model
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
	if req.DisplayInputPrice != nil {
		existing.DisplayInputPrice = req.DisplayInputPrice
	}
	if req.DisplayOutputPrice != nil {
		existing.DisplayOutputPrice = req.DisplayOutputPrice
	}
	if req.DisplayRateMultiplier != nil {
		existing.DisplayRateMultiplier = req.DisplayRateMultiplier
	}
	if req.CacheTransferRatio != nil {
		existing.CacheTransferRatio = req.CacheTransferRatio
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Notes != nil {
		existing.Notes = *req.Notes
	}

	if err := h.svc.Update(c.Request.Context(), existing); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, existing)
}

// Delete DELETE /api/v1/admin/users/:id/model-pricing/:overrideId
func (h *UserModelPricingHandler) Delete(c *gin.Context) {
	overrideID, err := strconv.ParseInt(c.Param("overrideId"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid override ID"))
		return
	}

	if err := h.svc.Delete(c.Request.Context(), overrideID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, nil)
}

// BatchUpsert PUT /api/v1/admin/users/:id/model-pricing/batch
func (h *UserModelPricingHandler) BatchUpsert(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_ID", "Invalid user ID"))
		return
	}

	var req batchUpsertUserModelPricingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	overrides := make([]service.UserModelPricingOverride, 0, len(req.Overrides))
	for _, r := range req.Overrides {
		enabled := true
		if r.Enabled != nil {
			enabled = *r.Enabled
		}
		overrides = append(overrides, service.UserModelPricingOverride{
			UserID:                userID,
			Model:                 r.Model,
			InputPrice:            r.InputPrice,
			OutputPrice:           r.OutputPrice,
			CacheWritePrice:       r.CacheWritePrice,
			CacheReadPrice:        r.CacheReadPrice,
			DisplayInputPrice:     r.DisplayInputPrice,
			DisplayOutputPrice:    r.DisplayOutputPrice,
			DisplayRateMultiplier: r.DisplayRateMultiplier,
			CacheTransferRatio:    r.CacheTransferRatio,
			Enabled:               enabled,
			Notes:                 r.Notes,
		})
	}

	if err := h.svc.BatchUpsert(c.Request.Context(), userID, overrides); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"count": len(overrides)})
}
