package admin

import (
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// loginPageKeys 8 个 settings key，集中定义便于批量读写。
var loginPageKeys = []string{
	service.SettingKeyLoginPageBadge,
	service.SettingKeyLoginPageHeadingLine1,
	service.SettingKeyLoginPageHeadingLine2,
	service.SettingKeyLoginPageDescription,
	service.SettingKeyLoginPageSupportedModelsTitle,
	service.SettingKeyLoginPageModelsDesc,
	service.SettingKeyLoginPageFormTitle,
	service.SettingKeyLoginPageFormSubtitle,
}

// 每字段的最大长度。超过拒绝保存。
// description / models_desc 略长，其他都是短标题。
const (
	loginPageMaxShort = 255
	loginPageMaxLong  = 500
)

// LoginPageHandler 管理员编辑登录页文案。字段为空时前端自动回落到 i18n，
// 所以「恢复默认」= 清空所有字段。
type LoginPageHandler struct {
	settingRepo service.SettingRepository
}

// NewLoginPageAdminHandler 构造登录页文案处理器
func NewLoginPageAdminHandler(settingRepo service.SettingRepository) *LoginPageHandler {
	return &LoginPageHandler{settingRepo: settingRepo}
}

type loginPageContentDTO struct {
	Badge                string `json:"badge"`
	HeadingLine1         string `json:"heading_line1"`
	HeadingLine2         string `json:"heading_line2"`
	Description          string `json:"description"`
	SupportedModelsTitle string `json:"supported_models_title"`
	ModelsDesc           string `json:"models_desc"`
	FormTitle            string `json:"form_title"`
	FormSubtitle         string `json:"form_subtitle"`
}

// Get 返回当前保存的 8 个字段；未设置的为 ""。前端据此判断是否回落 i18n。
func (h *LoginPageHandler) Get(c *gin.Context) {
	values, err := h.settingRepo.GetMultiple(c.Request.Context(), loginPageKeys)
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("LOAD_FAILED", err.Error()))
		return
	}
	response.Success(c, loginPageContentDTO{
		Badge:                values[service.SettingKeyLoginPageBadge],
		HeadingLine1:         values[service.SettingKeyLoginPageHeadingLine1],
		HeadingLine2:         values[service.SettingKeyLoginPageHeadingLine2],
		Description:          values[service.SettingKeyLoginPageDescription],
		SupportedModelsTitle: values[service.SettingKeyLoginPageSupportedModelsTitle],
		ModelsDesc:           values[service.SettingKeyLoginPageModelsDesc],
		FormTitle:            values[service.SettingKeyLoginPageFormTitle],
		FormSubtitle:         values[service.SettingKeyLoginPageFormSubtitle],
	})
}

// Update 批量写入 8 个字段。任何字段超长 → 400，全部 OK → 原子写入。
func (h *LoginPageHandler) Update(c *gin.Context) {
	var req loginPageContentDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	// 统一 trim + 长度校验。前端传来的空串直接存空串，前端自己回落 i18n。
	badge := strings.TrimSpace(req.Badge)
	heading1 := strings.TrimSpace(req.HeadingLine1)
	heading2 := strings.TrimSpace(req.HeadingLine2)
	description := strings.TrimSpace(req.Description)
	modelsTitle := strings.TrimSpace(req.SupportedModelsTitle)
	modelsDesc := strings.TrimSpace(req.ModelsDesc)
	formTitle := strings.TrimSpace(req.FormTitle)
	formSubtitle := strings.TrimSpace(req.FormSubtitle)

	if err := enforceMax("badge", badge, loginPageMaxShort); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := enforceMax("heading_line1", heading1, loginPageMaxShort); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := enforceMax("heading_line2", heading2, loginPageMaxShort); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := enforceMax("description", description, loginPageMaxLong); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := enforceMax("supported_models_title", modelsTitle, loginPageMaxShort); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := enforceMax("models_desc", modelsDesc, loginPageMaxLong); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := enforceMax("form_title", formTitle, loginPageMaxShort); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := enforceMax("form_subtitle", formSubtitle, loginPageMaxShort); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	payload := map[string]string{
		service.SettingKeyLoginPageBadge:                badge,
		service.SettingKeyLoginPageHeadingLine1:         heading1,
		service.SettingKeyLoginPageHeadingLine2:         heading2,
		service.SettingKeyLoginPageDescription:          description,
		service.SettingKeyLoginPageSupportedModelsTitle: modelsTitle,
		service.SettingKeyLoginPageModelsDesc:           modelsDesc,
		service.SettingKeyLoginPageFormTitle:            formTitle,
		service.SettingKeyLoginPageFormSubtitle:         formSubtitle,
	}
	if err := h.settingRepo.SetMultiple(c.Request.Context(), payload); err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("UPDATE_FAILED", err.Error()))
		return
	}

	response.Success(c, loginPageContentDTO{
		Badge:                badge,
		HeadingLine1:         heading1,
		HeadingLine2:         heading2,
		Description:          description,
		SupportedModelsTitle: modelsTitle,
		ModelsDesc:           modelsDesc,
		FormTitle:            formTitle,
		FormSubtitle:         formSubtitle,
	})
}

// enforceMax 超长返回 400 错误。
func enforceMax(field, value string, max int) error {
	if len([]rune(value)) > max {
		return infraerrors.BadRequest(
			"FIELD_TOO_LONG",
			"field "+field+" exceeds maximum length",
		)
	}
	return nil
}
