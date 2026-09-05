package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/imroc/req/v3"
)

const (
	chatgptSessionTokenCredentialKey       = "chatgpt_session_token"
	chatgptSessionRefreshedAtCredentialKey = "session_refreshed_at"
	chatgptSessionCookieName               = "__Secure-next-auth.session-token"
	chatgptSessionRefreshInterval          = 25 * time.Minute
)

var chatGPTAuthSessionURL = "https://chatgpt.com/api/auth/session"

func (a *Account) GetChatGPTSessionToken() string {
	if a == nil || !a.IsOpenAIOAuth() {
		return ""
	}
	return strings.TrimSpace(a.GetCredential(chatgptSessionTokenCredentialKey))
}

func isOpenAIChatGPTSessionOnly(account *Account) bool {
	if account == nil || account.IsOpenAIPersonalAccessToken() {
		return false
	}
	if strings.TrimSpace(account.GetOpenAIRefreshToken()) != "" {
		return false
	}
	return account.GetChatGPTSessionToken() != ""
}

func openaiChatGPTSessionNeedsRefresh(account *Account, refreshWindow time.Duration) bool {
	if !isOpenAIChatGPTSessionOnly(account) {
		return false
	}
	last := account.GetCredentialAsTime(chatgptSessionRefreshedAtCredentialKey)
	if last == nil || time.Since(*last) >= chatgptSessionRefreshInterval {
		return true
	}
	expiresAt := account.GetCredentialAsTime("expires_at")
	if expiresAt == nil {
		return true
	}
	return time.Until(*expiresAt) < refreshWindow
}

func stampChatGPTSessionRefresh(credentials map[string]any, sessionToken string, now time.Time) {
	if credentials == nil {
		return
	}
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return
	}
	credentials[chatgptSessionTokenCredentialKey] = sessionToken
	credentials[chatgptSessionRefreshedAtCredentialKey] = now.UTC().Format(time.RFC3339)
}

// RefreshChatGPTSession exchanges a ChatGPT NextAuth session cookie for a fresh access token.
// This is not an OAuth refresh_token grant; sessionToken must never be stored as refresh_token.
func (s *OpenAIOAuthService) RefreshChatGPTSession(ctx context.Context, sessionToken, proxyURL string) (*OpenAITokenInfo, error) {
	sessionToken = strings.TrimSpace(sessionToken)
	if sessionToken == "" {
		return nil, infraerrors.New(http.StatusBadRequest, "OPENAI_SESSION_TOKEN_REQUIRED", "sessionToken is required")
	}
	if s == nil || s.privacyClientFactory == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_SESSION_REFRESH_UNAVAILABLE", "ChatGPT session refresh client is not configured")
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	client, err := s.privacyClientFactory(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_SESSION_REFRESH_CLIENT_FAILED", "create HTTP client: %v", err)
	}

	// Cloudflare 403s a cold client that sends only __Secure-next-auth.session-token.
	// An anonymous GET /api/auth/session first (same TLS session + Set-Cookie jar)
	// then the session cookie succeeds through the same proxy.
	if primeErr := chatgptSessionExchange(ctx, client, ""); primeErr != nil {
		return nil, primeErr
	}
	resp, err := chatgptSessionGET(ctx, client, sessionToken)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_SESSION_REFRESH_REQUEST_FAILED", "session refresh request failed: %v", err)
	}
	if err := chatgptSessionResponseError(resp); err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Bytes(), &payload); err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_SESSION_REFRESH_INVALID", "session response is not JSON: %v", err)
	}
	accessToken := firstChatGPTSessionString(payload, "accessToken", "access_token")
	if accessToken == "" {
		return nil, infraerrors.New(http.StatusBadGateway, "OPENAI_SESSION_REFRESH_NO_ACCESS_TOKEN", "session response did not include accessToken")
	}

	tokenInfo := &OpenAITokenInfo{
		AccessToken:      accessToken,
		Email:            firstChatGPTSessionString(payload, "user.email", "email"),
		ChatGPTAccountID: firstChatGPTSessionString(payload, "account.id", "account.account_id", "account.chatgpt_account_id", "chatgpt_account_id"),
		ChatGPTUserID:    firstChatGPTSessionString(payload, "user.id", "chatgpt_user_id", "user_id"),
		PlanType:         firstChatGPTSessionString(payload, "account.planType", "account.plan_type", "plan_type"),
	}
	if claims, err := openai.DecodeIDToken(accessToken); err == nil && claims != nil {
		if tokenInfo.Email == "" {
			tokenInfo.Email = strings.TrimSpace(claims.Email)
		}
		if claims.Exp > 0 {
			tokenInfo.ExpiresAt = claims.Exp
			tokenInfo.ExpiresIn = claims.Exp - time.Now().Unix()
		}
		if claims.OpenAIAuth != nil {
			if tokenInfo.ChatGPTAccountID == "" {
				tokenInfo.ChatGPTAccountID = strings.TrimSpace(claims.OpenAIAuth.ChatGPTAccountID)
			}
			if tokenInfo.ChatGPTUserID == "" {
				tokenInfo.ChatGPTUserID = strings.TrimSpace(claims.OpenAIAuth.ChatGPTUserID)
			}
			if tokenInfo.PlanType == "" {
				tokenInfo.PlanType = strings.TrimSpace(claims.OpenAIAuth.ChatGPTPlanType)
			}
			if tokenInfo.OrganizationID == "" {
				tokenInfo.OrganizationID = strings.TrimSpace(claims.OpenAIAuth.POID)
			}
		}
	}
	s.enrichTokenInfo(ctx, tokenInfo, proxyURL)
	return tokenInfo, nil
}

func (s *OpenAIOAuthService) RefreshChatGPTSessionWithProxyID(ctx context.Context, sessionToken string, proxyID *int64) (*OpenAITokenInfo, error) {
	return s.RefreshChatGPTSession(ctx, sessionToken, s.proxyURLForID(ctx, proxyID))
}

func chatgptSessionExchange(ctx context.Context, client *req.Client, sessionToken string) error {
	resp, err := chatgptSessionGET(ctx, client, sessionToken)
	if err != nil {
		return infraerrors.Newf(http.StatusBadGateway, "OPENAI_SESSION_REFRESH_REQUEST_FAILED", "session refresh request failed: %v", err)
	}
	return chatgptSessionResponseError(resp)
}

func chatgptSessionGET(ctx context.Context, client *req.Client, sessionToken string) (*req.Response, error) {
	r := client.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json").
		SetHeader("Origin", "https://chatgpt.com").
		SetHeader("Referer", "https://chatgpt.com/").
		SetHeader("sec-fetch-mode", "cors").
		SetHeader("sec-fetch-site", "same-origin").
		SetHeader("sec-fetch-dest", "empty")
	if strings.TrimSpace(sessionToken) != "" {
		r.SetCookies(&http.Cookie{Name: chatgptSessionCookieName, Value: sessionToken, Secure: true})
	}
	return r.Get(chatGPTAuthSessionURL)
}

func chatgptSessionResponseError(resp *req.Response) error {
	if resp == nil {
		return infraerrors.New(http.StatusBadGateway, "OPENAI_SESSION_REFRESH_REQUEST_FAILED", "session refresh request failed: empty response")
	}
	if resp.IsSuccessState() {
		return nil
	}
	if chatGPTResponseLooksLikeChallenge(resp.StatusCode, resp.String()) {
		return infraerrors.New(http.StatusBadGateway, "OPENAI_SESSION_REFRESH_CF_BLOCKED", "ChatGPT/Cloudflare 拦截了 session 刷新。请确认账号代理能打开 chatgpt.com 后重试")
	}
	return infraerrors.Newf(http.StatusBadGateway, "OPENAI_SESSION_REFRESH_REJECTED", "ChatGPT rejected sessionToken: status %d, body: %s", resp.StatusCode, truncate(resp.String(), 200))
}

func chatGPTResponseLooksLikeChallenge(status int, body string) bool {
	if status != http.StatusForbidden && status != http.StatusServiceUnavailable {
		return false
	}
	lower := strings.ToLower(body)
	return strings.Contains(lower, "<html") ||
		strings.Contains(lower, "cloudflare") ||
		strings.Contains(lower, "just a moment") ||
		strings.Contains(lower, "cf-ray") ||
		strings.Contains(lower, "attention required")
}

func (s *OpenAIOAuthService) CheckChatGPTAccessToken(ctx context.Context, accessToken, proxyURL string) error {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return infraerrors.New(http.StatusBadRequest, "OPENAI_ACCESS_TOKEN_REQUIRED", "accessToken is required")
	}
	if s == nil || s.privacyClientFactory == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := s.privacyClientFactory(proxyURL)
	if err != nil {
		return infraerrors.Newf(http.StatusBadGateway, "OPENAI_ACCESS_TOKEN_CHECK_CLIENT_FAILED", "create HTTP client: %v", err)
	}

	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("Origin", "https://chatgpt.com").
		SetHeader("Referer", "https://chatgpt.com/").
		SetHeader("Accept", "application/json").
		Get(chatGPTAccountsCheckURL)
	if err != nil {
		return infraerrors.Newf(http.StatusBadGateway, "OPENAI_ACCESS_TOKEN_CHECK_FAILED", "ChatGPT account check failed: %v", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return infraerrors.Newf(http.StatusBadRequest, "OPENAI_ACCESS_TOKEN_REJECTED", "ChatGPT rejected accessToken: status %d, body: %s", resp.StatusCode, truncate(resp.String(), 200))
	}
	if !resp.IsSuccessState() {
		return infraerrors.Newf(http.StatusBadGateway, "OPENAI_ACCESS_TOKEN_CHECK_FAILED", "ChatGPT account check failed: status %d, body: %s", resp.StatusCode, truncate(resp.String(), 200))
	}
	return nil
}

func (s *OpenAIOAuthService) CheckChatGPTAccessTokenWithProxyID(ctx context.Context, accessToken string, proxyID *int64) error {
	return s.CheckChatGPTAccessToken(ctx, accessToken, s.proxyURLForID(ctx, proxyID))
}

func (s *OpenAIOAuthService) proxyURLForID(ctx context.Context, proxyID *int64) string {
	if s == nil || proxyID == nil || s.proxyRepo == nil {
		return ""
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil || proxy == nil {
		return ""
	}
	return proxy.URL()
}

func firstChatGPTSessionString(obj map[string]any, dottedPaths ...string) string {
	for _, dotted := range dottedPaths {
		path := strings.Split(dotted, ".")
		cur := any(obj)
		ok := true
		for _, key := range path {
			m, isMap := cur.(map[string]any)
			if !isMap {
				ok = false
				break
			}
			cur, ok = m[key]
			if !ok {
				break
			}
		}
		if !ok {
			continue
		}
		switch typed := cur.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
