package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type inflightReservationAdminResponse struct {
	service.InflightReservationSettings
	SimpleMode bool `json:"simple_mode"`
}

func (h *SettingHandler) inflightReservationResponse(settings service.InflightReservationSettings) inflightReservationAdminResponse {
	return inflightReservationAdminResponse{
		InflightReservationSettings: settings,
		SimpleMode:                  h != nil && h.settingService != nil && h.settingService.InflightReservationSimpleMode(),
	}
}

// GetInflightReservationSettings 读取额度预检规则。
// GET /api/v1/admin/inflight-reservation
func (h *SettingHandler) GetInflightReservationSettings(c *gin.Context) {
	if h == nil || h.settingService == nil {
		response.InternalError(c, "settings service unavailable")
		return
	}
	settings := h.settingService.GetInflightReservationSettings(c.Request.Context())
	response.Success(c, h.inflightReservationResponse(settings))
}

// UpdateInflightReservationSettings 保存额度预检规则。下一次请求即使用新值。
// PUT /api/v1/admin/inflight-reservation
func (h *SettingHandler) UpdateInflightReservationSettings(c *gin.Context) {
	if h == nil || h.settingService == nil {
		response.InternalError(c, "settings service unavailable")
		return
	}
	var req service.InflightReservationSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.settingService.SetInflightReservationSettings(c.Request.Context(), req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	settings := h.settingService.GetInflightReservationSettings(c.Request.Context())
	response.Success(c, h.inflightReservationResponse(settings))
}
