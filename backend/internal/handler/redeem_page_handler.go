package handler

import (
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RedeemPageHandler 用户侧兑换页购买说明读取。
// 与 admin.RedeemPageHandler 共用 settings key。
type RedeemPageHandler struct {
	settingRepo service.SettingRepository
}

// NewRedeemPageHandler 创建用户侧兑换页说明处理器
func NewRedeemPageHandler(settingRepo service.SettingRepository) *RedeemPageHandler {
	return &RedeemPageHandler{settingRepo: settingRepo}
}

type redeemPageNoticeResponse struct {
	Notice string `json:"notice"`
}

// Get GET /api/v1/user/redeem-page
// 返回兑换页「去哪买兑换码」纯文本说明。未配置或空串时前端不展示横幅。
func (h *RedeemPageHandler) Get(c *gin.Context) {
	v, err := h.settingRepo.GetValue(c.Request.Context(), admin.SettingKeyRedeemPageBuyNotice)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			response.Success(c, redeemPageNoticeResponse{Notice: ""})
			return
		}
		response.ErrorFrom(c, infraerrors.InternalServer("LOAD_FAILED", err.Error()))
		return
	}
	response.Success(c, redeemPageNoticeResponse{Notice: strings.TrimSpace(v)})
}
