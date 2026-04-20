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
	DefaultPricingPageIntro = `## 本站计价模式

我们按 **原厂真实 Token 计价**：每个模型的输入、输出、缓存读取单价都与上游（Anthropic / OpenAI / Google 等）官方价格一致，不加价、不打包、不隐藏倍率。

每一次调用的花费都能在「使用记录」里逐条还原——看得见每一 Token，算得清每一分钱。`

	DefaultPricingPageEducation = `## 几种常见计价模式对比

| 模式 | 描述 | 问题 |
|------|------|------|
| **按次计费** | 不管请求多大，每次固定扣一个单位 | 短请求亏，长请求白嫖；平台为了不亏必须把单价定得很高 |
| **统一 Token 价** | 所有模型用同一个假 Token 单价 | 便宜模型被拉贵、昂贵模型被藏起来；用户永远不知道真实成本 |
| **包月不限量** | 预付费换"无限" | 实际限流、降智、偷偷换小模型；到头来你根本不知道自己在用什么 |
| **本站：按原厂 Token 计价** | 与官方单价完全一致，按消耗扣费 | —— |

**我们的目标**：让你像用官方 API 一样透明地消费，同时享受多账号聚合、统一鉴权、自动故障转移带来的便利。`
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

	response.Success(c, pricingPageContentResponse{Intro: req.Intro, Education: req.Education})
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
