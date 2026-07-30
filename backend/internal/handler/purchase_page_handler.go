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

// PurchasePageHandler 用户侧充值/订阅页公告读取。
// 与 admin.PurchasePageHandler 共用 settings key。
type PurchasePageHandler struct {
	settingRepo service.SettingRepository
}

// NewPurchasePageHandler 创建用户侧充值订阅页公告处理器
func NewPurchasePageHandler(settingRepo service.SettingRepository) *PurchasePageHandler {
	return &PurchasePageHandler{settingRepo: settingRepo}
}

type purchasePageNoticeResponse struct {
	Notice string `json:"notice"`
}

// Get GET /api/v1/user/purchase-page
// 返回充值/订阅页顶部公告。从未配置时使用内置默认紧急文案；
// 管理员清空后返回空串，前端不展示横幅。
func (h *PurchasePageHandler) Get(c *gin.Context) {
	v, err := h.settingRepo.GetValue(c.Request.Context(), admin.SettingKeyPurchasePageNotice)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			response.Success(c, purchasePageNoticeResponse{Notice: admin.DefaultPurchasePageNotice})
			return
		}
		response.ErrorFrom(c, infraerrors.InternalServer("LOAD_FAILED", err.Error()))
		return
	}
	// Explicit empty string means admin disabled the notice.
	response.Success(c, purchasePageNoticeResponse{Notice: strings.TrimSpace(v)})
}
