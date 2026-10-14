package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SettingHandler 公开设置处理器（无需认证）
type SettingHandler struct {
	settingService *service.SettingService
}

// NewSettingHandler 创建公开设置处理器。
// version 仍由 Wire 传入，公开响应不再带构建版本。
func NewSettingHandler(settingService *service.SettingService, version string) *SettingHandler {
	_ = version
	return &SettingHandler{
		settingService: settingService,
	}
}

// GetPublicSettings 获取公开设置
// GET /api/v1/settings/public
func (h *SettingHandler) GetPublicSettings(c *gin.Context) {
	settings, err := h.settingService.GetPublicSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.PublicSettings{
		RegistrationEnabled:              settings.RegistrationEnabled,
		EmailVerifyEnabled:               settings.EmailVerifyEnabled,
		ForceEmailOnThirdPartySignup:     settings.ForceEmailOnThirdPartySignup,
		RegistrationEmailSuffixWhitelist: settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 settings.PromoCodeEnabled,
		PasswordResetEnabled:             settings.PasswordResetEnabled,
		InvitationCodeEnabled:            settings.InvitationCodeEnabled,
		LegalConsent:                     toDTOLegalConsentSettings(settings.LegalConsent),
		TotpEnabled:                      settings.TotpEnabled,
		TurnstileEnabled:                 settings.TurnstileEnabled,
		TurnstileSiteKey:                 settings.TurnstileSiteKey,
		SiteName:                         settings.SiteName,
		SiteLogo:                         settings.SiteLogo,
		SiteSubtitle:                     settings.SiteSubtitle,
		APIBaseURL:                       settings.APIBaseURL,
		ContactInfo:                      settings.ContactInfo,
		DocURL:                           settings.DocURL,
		TutorialURL:                      settings.TutorialURL,
		HomeContent:                      settings.HomeContent,
		HideCcsImportButton:              settings.HideCcsImportButton,
		CcsImportCodexModel:              settings.CcsImportCodexModel,
		CcsImportAnthropicCodexModel:     settings.CcsImportAnthropicCodexModel,
		PurchaseSubscriptionEnabled:      settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:          settings.PurchaseSubscriptionURL,
		TableDefaultPageSize:             settings.TableDefaultPageSize,
		TablePageSizeOptions:             settings.TablePageSizeOptions,
		CustomMenuItems:                  dto.ParseUserVisibleMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                  dto.ParseCustomEndpoints(settings.CustomEndpoints),
		LinuxDoOAuthEnabled:              settings.LinuxDoOAuthEnabled,
		WeChatOAuthEnabled:               settings.WeChatOAuthEnabled,
		WeChatOAuthOpenEnabled:           settings.WeChatOAuthOpenEnabled,
		WeChatOAuthMPEnabled:             settings.WeChatOAuthMPEnabled,
		WeChatOAuthMobileEnabled:         settings.WeChatOAuthMobileEnabled,
		OIDCOAuthEnabled:                 settings.OIDCOAuthEnabled,
		// 公开接口不回 OIDC 提供方名字。登录按钮用前端的 OIDC 兜底。管理端设置仍返回原名。
		OIDCOAuthProviderName: "",
		BackendModeEnabled:    settings.BackendModeEnabled,
		PaymentEnabled:        settings.PaymentEnabled,
		PaymentCNYPerUSD:      settings.PaymentCNYPerUSD,
		LoginPage:             toDTOLoginPageContent(settings.LoginPage),
		// 未登录的公开设置不带构建版本。管理员走 /api/v1/admin/system/version。
		Version:                     "",
		ServerTimezone:              timezone.Name(),
		ServerUTCOffset:             timezone.UTCOffset(),
		BalanceLowNotifyEnabled:     settings.BalanceLowNotifyEnabled,
		AccountQuotaNotifyEnabled:   settings.AccountQuotaNotifyEnabled,
		BalanceLowNotifyThreshold:   settings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL: settings.BalanceLowNotifyRechargeURL,

		ChannelMonitorEnabled:                settings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: settings.ChannelMonitorDefaultIntervalSeconds,
		ChannelMonitorMode:                   settings.ChannelMonitorMode,
		ChannelMonitorHideThroughput:         settings.ChannelMonitorHideThroughput,
		ChannelMonitorShowQuota:              settings.ChannelMonitorShowQuota,
		ChannelMonitorHideUserRanking:        settings.ChannelMonitorHideUserRanking,

		AvailableChannelsEnabled:   settings.AvailableChannelsEnabled,
		AllowUserViewErrorRequests: settings.AllowUserViewErrorRequests,
		ModelPlazaEnabled:          settings.ModelPlazaEnabled,
		ModelPlazaRequireAuth:      settings.ModelPlazaRequireAuth,
		ModelPlazaDescription:      settings.ModelPlazaDescription,
		PluginManagementEnabled:    settings.PluginManagementEnabled,

		AffiliateEnabled: settings.AffiliateEnabled,

		RiskControlEnabled: settings.RiskControlEnabled,
	})
}

func toDTOLegalConsentSettings(s service.LegalConsentSettings) dto.LegalConsentSettings {
	return dto.LegalConsentSettings{
		Enabled:            s.Enabled,
		Version:            s.Version,
		Content:            s.Content,
		ConfirmationPhrase: s.ConfirmationPhrase,
		MinReadSeconds:     s.MinReadSeconds,
	}
}

// toDTOLoginPageContent 把 service.LoginPageContent 镜像到 DTO。nil 直接透传。
func toDTOLoginPageContent(c *service.LoginPageContent) *dto.LoginPageContent {
	if c == nil {
		return nil
	}
	return &dto.LoginPageContent{
		Badge:                c.Badge,
		HeadingLine1:         c.HeadingLine1,
		HeadingLine2:         c.HeadingLine2,
		Description:          c.Description,
		SupportedModelsTitle: c.SupportedModelsTitle,
		ModelsDesc:           c.ModelsDesc,
		FormTitle:            c.FormTitle,
		FormSubtitle:         c.FormSubtitle,
	}
}
