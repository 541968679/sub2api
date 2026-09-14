package admin

import (
	"errors"
	"strings"

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

// Default Markdown shown on the user pricing page when no admin content is
// saved yet. Exported so the user-side handler and the admin Get endpoint
// return identical fallbacks — the admin editor needs the default value
// pre-filled so admins can tweak it instead of starting from a blank page.
const (
	DefaultPricingPageIntro = "一条请求的最终消费 = 百万 token 单价 × token 数量 × 分组倍率 × 1 人民币 / 1 美元\n\n" +
		"**百万 token 单价**：与官方一样。例如 Opus 4.8 的百万输入 token 单价是 5 美元，百万输出 token 单价是 25 美元。\n\n" +
		"**token 数量**：例如一条请求输入 token 是 2 万，输出是 5 千。\n\n" +
		"**分组倍率**：例如分组倍率是 0.5。\n\n" +
		"那么最终消耗就是：\n\n" +
		"(5×2/100×0.5)+(25×0.5/100×0.5)=0.1125 人民币"

	// Education section retired; keep the key for admin compatibility but default empty.
	DefaultPricingPageEducation = ""
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

// Get 返回当前保存的两段 Markdown 文案；未保存/为空时回落到内置默认值，
// 让管理员进入编辑界面时看到的就是用户实际看到的文案，方便直接修改。
func (h *PricingPageHandler) Get(c *gin.Context) {
	intro, err := h.loadValue(c, SettingKeyPricingPageIntro, DefaultPricingPageIntro)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	edu, err := h.loadValue(c, SettingKeyPricingPageEducation, DefaultPricingPageEducation)
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

	response.Success(c, pricingPageContentResponse(req))
}

// loadValue 读取 key 对应的值；未保存 / 空字符串回落到 fallback。
// 其他错误向上抛。
func (h *PricingPageHandler) loadValue(c *gin.Context, key, fallback string) (string, error) {
	v, err := h.settingRepo.GetValue(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			return fallback, nil
		}
		return "", err
	}
	if strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	return v, nil
}
