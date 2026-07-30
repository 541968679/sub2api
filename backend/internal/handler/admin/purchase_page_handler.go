package admin

import (
	"errors"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// Settings keys for the user-facing 充值/订阅 page notice. Keep in sync with
// handler.PurchasePageHandler which reads the same keys.
const (
	SettingKeyPurchasePageNotice = "purchase_page.notice"
)

// DefaultPurchasePageNotice is shown on /purchase (recharge + subscription tabs)
// when the setting has never been saved. Admins can edit or clear it via
// 页面内容 → 充值订阅页. Saving an empty string disables the banner.
const DefaultPurchasePageNotice = `线上支付渠道暂不可用，如需测试请联系客服vx：tqrzfwidc`

const purchasePageNoticeMaxBytes = 8 * 1024 // 8 KB plain text / light markdown

// PurchasePageHandler 管理员编辑用户充值/订阅页顶部公告文案。
// 纯 KV：一段公告存在 settings 表的 purchase_page.notice。
type PurchasePageHandler struct {
	settingRepo service.SettingRepository
}

// NewPurchasePageAdminHandler 创建管理员充值订阅页公告处理器
func NewPurchasePageAdminHandler(settingRepo service.SettingRepository) *PurchasePageHandler {
	return &PurchasePageHandler{settingRepo: settingRepo}
}

type purchasePageContentResponse struct {
	Notice string `json:"notice"`
}

type updatePurchasePageContentRequest struct {
	Notice string `json:"notice"`
}

// Get 返回当前保存的公告；从未保存时回落到内置默认紧急文案，
// 方便管理员进入编辑界面时看到用户实际会看到的内容。
// 已显式保存为空字符串时返回空串（表示关闭公告）。
func (h *PurchasePageHandler) Get(c *gin.Context) {
	notice, err := h.loadValue(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, purchasePageContentResponse{Notice: notice})
}

// Update 写入公告文案。空字符串表示关闭前端公告横幅。
func (h *PurchasePageHandler) Update(c *gin.Context) {
	var req updatePurchasePageContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	notice := strings.TrimSpace(req.Notice)
	if len(notice) > purchasePageNoticeMaxBytes {
		response.ErrorFrom(c, infraerrors.BadRequest("CONTENT_TOO_LARGE", "purchase page notice exceeds 8KB limit"))
		return
	}

	if err := h.settingRepo.Set(c.Request.Context(), SettingKeyPurchasePageNotice, notice); err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("UPDATE_FAILED", err.Error()))
		return
	}

	response.Success(c, purchasePageContentResponse{Notice: notice})
}

// loadValue 读取公告：未保存 → 默认文案；已保存（含空串）→ 原样返回。
func (h *PurchasePageHandler) loadValue(c *gin.Context) (string, error) {
	v, err := h.settingRepo.GetValue(c.Request.Context(), SettingKeyPurchasePageNotice)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			return DefaultPurchasePageNotice, nil
		}
		return "", infraerrors.InternalServer("LOAD_FAILED", err.Error())
	}
	return v, nil
}
