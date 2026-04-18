package admin

import (
	"errors"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// Settings keys for the user-facing 模型计价 page. Keep in sync with
// handler.PricingPageHandler which reads the same keys.
const (
	SettingKeyPricingPageIntro     = "pricing_page.intro_markdown"
	SettingKeyPricingPageEducation = "pricing_page.education_markdown"
)

// PricingPageHandler 管理员编辑用户「模型计价」页面文案的处理器。
// 纯 KV 存储：两段 Markdown 存在 settings 表里对应的两个 key。
type PricingPageHandler struct {
	settingRepo service.SettingRepository
}

// NewPricingPageAdminHandler 创建管理员模型计价页文案处理器
func NewPricingPageAdminHandler(settingRepo service.SettingRepository) *PricingPageHandler {
	return &PricingPageHandler{settingRepo: settingRepo}
}

type pricingPageContentResponse struct {
	Intro     string `json:"intro"`
	Education string `json:"education"`
}

type updatePricingPageContentRequest struct {
	Intro     string `json:"intro"`
	Education string `json:"education"`
}

// Get 返回当前保存的两段 Markdown 文案；未保存的 key 返回空串。
func (h *PricingPageHandler) Get(c *gin.Context) {
	intro, err := h.loadValue(c, SettingKeyPricingPageIntro)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	edu, err := h.loadValue(c, SettingKeyPricingPageEducation)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pricingPageContentResponse{Intro: intro, Education: edu})
}

// Update 批量写入两段 Markdown，使用 SettingRepository 的 upsert 语义。
func (h *PricingPageHandler) Update(c *gin.Context) {
	var req updatePricingPageContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	payload := map[string]string{
		SettingKeyPricingPageIntro:     req.Intro,
		SettingKeyPricingPageEducation: req.Education,
	}
	if err := h.settingRepo.SetMultiple(c.Request.Context(), payload); err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("UPDATE_FAILED", err.Error()))
		return
	}

	response.Success(c, pricingPageContentResponse{Intro: req.Intro, Education: req.Education})
}

// loadValue 读取 key 对应的值；ErrSettingNotFound 返回空串，其他错误向上抛。
func (h *PricingPageHandler) loadValue(c *gin.Context, key string) (string, error) {
	v, err := h.settingRepo.GetValue(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			return "", nil
		}
		return "", err
	}
	return v, nil
}
