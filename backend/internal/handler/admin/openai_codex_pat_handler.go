package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type openAICodexPATValidator interface {
	ValidateCodexPersonalAccessToken(ctx context.Context, accessToken, proxyURL string) (*service.OpenAITokenInfo, error)
}

type OpenAICodexPATCreateRequest struct {
	AccessToken             string         `json:"access_token" binding:"required"`
	Name                    string         `json:"name"`
	Notes                   *string        `json:"notes"`
	GroupIDs                []int64        `json:"group_ids"`
	ProxyID                 *int64         `json:"proxy_id"`
	Concurrency             *int           `json:"concurrency"`
	Priority                *int           `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      *bool          `json:"auto_pause_on_expired"`
	CredentialExtras        map[string]any `json:"credential_extras"`
	Extra                   map[string]any `json:"extra"`
	SkipDefaultGroupBind    *bool          `json:"skip_default_group_bind"`
	ConfirmMixedChannelRisk *bool          `json:"confirm_mixed_channel_risk"`
}

func (h *OpenAIOAuthHandler) CreateAccountFromCodexPAT(c *gin.Context) {
	var req OpenAICodexPATCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Concurrency != nil && *req.Concurrency < 0 {
		response.BadRequest(c, "concurrency must be >= 0")
		return
	}
	if req.Priority != nil && *req.Priority < 0 {
		response.BadRequest(c, "priority must be >= 0")
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	if req.LoadFactor != nil && (*req.LoadFactor < 0 || *req.LoadFactor > 10000) {
		response.BadRequest(c, "load_factor must be between 0 and 10000")
		return
	}

	var proxyURL string
	if req.ProxyID != nil {
		proxy, err := h.adminService.GetProxy(c.Request.Context(), *req.ProxyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	tokenInfo, err := h.codexPATValidator.ValidateCodexPersonalAccessToken(c.Request.Context(), req.AccessToken, proxyURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	credentials := mergeOpenAICodexPATMap(
		h.openaiOAuthService.BuildAccountCredentials(tokenInfo),
		sanitizeOpenAICodexPATCredentialExtras(req.CredentialExtras, req.AccessToken),
	)
	credentials = service.NormalizeOpenAIPersonalAccessTokenCredentials(nil, tokenInfo, credentials)
	extra := mergeOpenAICodexPATMap(sanitizeOpenAICodexPATExtra(req.Extra, req.AccessToken), map[string]any{
		"import_source":       "codex_personal_access_token",
		"auth_provider":       "codex_personal_access_token",
		"imported_at":         time.Now().UTC().Format(time.RFC3339),
		"access_token_sha256": openAICodexPATFingerprint(req.AccessToken),
	})

	concurrency := 3
	if req.Concurrency != nil {
		concurrency = *req.Concurrency
	}
	priority := 50
	if req.Priority != nil {
		priority = *req.Priority
	}
	skipDefaultGroupBind := req.SkipDefaultGroupBind != nil && *req.SkipDefaultGroupBind

	account, err := h.adminService.CreateAccount(c.Request.Context(), &service.CreateAccountInput{
		Name:                  buildOpenAICodexPATAccountName(req.Name, tokenInfo),
		Notes:                 req.Notes,
		Platform:              service.PlatformOpenAI,
		Type:                  service.AccountTypeOAuth,
		Credentials:           credentials,
		Extra:                 extra,
		ProxyID:               req.ProxyID,
		Concurrency:           concurrency,
		Priority:              priority,
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		GroupIDs:              req.GroupIDs,
		ExpiresAt:             req.ExpiresAt,
		AutoPauseOnExpired:    req.AutoPauseOnExpired,
		SkipDefaultGroupBind:  skipDefaultGroupBind,
		SkipMixedChannelCheck: req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, safeOpenAICodexPATAccountResponse(account))
}

func safeOpenAICodexPATAccountResponse(account *service.Account) *dto.Account {
	out := dto.AccountFromService(account)
	if out != nil {
		out.Credentials = nil
	}
	return out
}

func buildOpenAICodexPATAccountName(name string, tokenInfo *service.OpenAITokenInfo) string {
	if name = strings.TrimSpace(name); name != "" {
		return name
	}
	if tokenInfo != nil {
		for _, candidate := range []string{tokenInfo.Email, tokenInfo.ChatGPTAccountID, tokenInfo.ChatGPTUserID} {
			if candidate = strings.TrimSpace(candidate); candidate != "" {
				return candidate
			}
		}
	}
	return "Codex PAT Account"
}

func sanitizeOpenAICodexPATCredentialExtras(input map[string]any, accessToken string) map[string]any {
	protected := map[string]struct{}{
		"access_token": {}, "refresh_token": {}, "id_token": {}, "expires_at": {}, "expires_in": {},
		"email": {}, "chatgpt_account_id": {}, "chatgpt_user_id": {}, "organization_id": {},
		"plan_type": {}, "client_id": {}, "auth_mode": {}, "openai_auth_mode": {}, "token_type": {},
		"chatgpt_account_is_fedramp": {},
	}
	return sanitizeOpenAICodexPATMap(input, accessToken, protected)
}

func sanitizeOpenAICodexPATExtra(input map[string]any, accessToken string) map[string]any {
	protected := map[string]struct{}{
		"access_token": {}, "refresh_token": {}, "id_token": {}, "authorization": {},
		"credentials": {}, "personal_access_token": {}, "codex_pat": {}, "pat": {}, "token": {},
	}
	return sanitizeOpenAICodexPATMap(input, accessToken, protected)
}

func sanitizeOpenAICodexPATMap(input map[string]any, accessToken string, protected map[string]struct{}) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		if _, blocked := protected[strings.ToLower(strings.TrimSpace(key))]; blocked {
			continue
		}
		if sanitized, ok := sanitizeOpenAICodexPATValue(value, accessToken, protected); ok {
			out[key] = sanitized
		}
	}
	return out
}

func sanitizeOpenAICodexPATValue(value any, accessToken string, protected map[string]struct{}) (any, bool) {
	token := strings.TrimSpace(accessToken)
	switch value := value.(type) {
	case string:
		trimmed := strings.TrimSpace(value)
		if (token != "" && strings.Contains(value, token)) || strings.HasPrefix(trimmed, "at-") {
			return nil, false
		}
		return value, true
	case map[string]any:
		return sanitizeOpenAICodexPATMap(value, accessToken, protected), true
	case []any:
		out := make([]any, 0, len(value))
		for _, nested := range value {
			if sanitized, ok := sanitizeOpenAICodexPATValue(nested, accessToken, protected); ok {
				out = append(out, sanitized)
			}
		}
		return out, true
	}
	return value, true
}

func mergeOpenAICodexPATMap(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}

func openAICodexPATFingerprint(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}
