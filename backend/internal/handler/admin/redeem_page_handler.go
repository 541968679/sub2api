package admin

import (
	"errors"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// Settings keys for the user-facing redeem page buy-code notice.
// Keep in sync with handler.RedeemPageHandler which reads the same key.
const (
	SettingKeyRedeemPageBuyNotice = "redeem_page.buy_notice"
)

const redeemPageNoticeMaxBytes = 8 * 1024 // 8 KB plain text

// RedeemPageHandler 管理员编辑用户兑换页「购买兑换码」说明文案。
// 纯 KV：一段说明存在 settings 表的 redeem_page.buy_notice。
// 默认空字符串：未配置时不展示横幅，避免硬编码外部销售站地址。
type RedeemPageHandler struct {
	settingRepo service.SettingRepository
}

// NewRedeemPageAdminHandler 创建管理员兑换页说明处理器
func NewRedeemPageAdminHandler(settingRepo service.SettingRepository) *RedeemPageHandler {
	return &RedeemPageHandler{settingRepo: settingRepo}
}

type redeemPageContentResponse struct {
	Notice string `json:"notice"`
}

type updateRedeemPageContentRequest struct {
	Notice string `json:"notice"`
}

// Get 返回当前保存的说明；从未保存时返回空串（前端不展示横幅）。
func (h *RedeemPageHandler) Get(c *gin.Context) {
	notice, err := h.loadValue(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, redeemPageContentResponse{Notice: notice})
}

// Update 写入说明文案。空字符串表示关闭前端横幅。
func (h *RedeemPageHandler) Update(c *gin.Context) {
	var req updateRedeemPageContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	notice := strings.TrimSpace(req.Notice)
	if len(notice) > redeemPageNoticeMaxBytes {
		response.ErrorFrom(c, infraerrors.BadRequest("CONTENT_TOO_LARGE", "redeem page notice exceeds 8KB limit"))
		return
	}

	if err := h.settingRepo.Set(c.Request.Context(), SettingKeyRedeemPageBuyNotice, notice); err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("UPDATE_FAILED", err.Error()))
		return
	}

	response.Success(c, redeemPageContentResponse{Notice: notice})
}

// loadValue 读取说明：未保存 → 空串；已保存（含空串）→ 原样返回。
func (h *RedeemPageHandler) loadValue(c *gin.Context) (string, error) {
	v, err := h.settingRepo.GetValue(c.Request.Context(), SettingKeyRedeemPageBuyNotice)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			return "", nil
		}
		return "", infraerrors.InternalServer("LOAD_FAILED", err.Error())
	}
	return v, nil
}
