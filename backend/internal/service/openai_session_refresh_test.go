package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

func TestOpenAIChatGPTSessionNeedsRefresh(t *testing.T) {
	now := time.Now().UTC()
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"chatgpt_session_token": "st-1",
			"access_token":          "at-1",
			"expires_at":            now.Add(10 * 24 * time.Hour).Format(time.RFC3339),
		},
	}
	require.True(t, openaiChatGPTSessionNeedsRefresh(account, 3*time.Minute), "missing session_refreshed_at should refresh")

	account.Credentials["session_refreshed_at"] = now.Add(-30 * time.Minute).Format(time.RFC3339)
	require.True(t, openaiChatGPTSessionNeedsRefresh(account, 3*time.Minute), "stale session_refreshed_at should refresh")

	account.Credentials["session_refreshed_at"] = now.Add(-5 * time.Minute).Format(time.RFC3339)
	require.True(t, openaiChatGPTSessionNeedsRefresh(account, 3*time.Minute), "unverified stamp must still refresh")

	account.Credentials["chatgpt_session_verified"] = true
	require.False(t, openaiChatGPTSessionNeedsRefresh(account, 3*time.Minute), "verified fresh session should wait")

	account.Credentials["refresh_token"] = "rt-1"
	require.False(t, openaiChatGPTSessionNeedsRefresh(account, 3*time.Minute), "OAuth RT accounts do not use session refresh")
}

func TestRefreshChatGPTSessionRequiresPrivacyClientFactory(t *testing.T) {
	svc := NewOpenAIOAuthService(nil, nil)
	_, err := svc.RefreshChatGPTSession(context.Background(), "st-live", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "OPENAI_SESSION_REFRESH_UNAVAILABLE")
}

func TestProvideOpenAIOAuthServiceInjectsPrivacyClientFactory(t *testing.T) {
	called := false
	svc := ProvideOpenAIOAuthService(nil, nil, func(string) (*req.Client, error) {
		called = true
		return nil, errors.New("factory-hit")
	})
	_, err := svc.RefreshChatGPTSession(context.Background(), "st-live", "")
	require.Error(t, err)
	require.True(t, called, "privacy client factory should be invoked")
	require.NotContains(t, err.Error(), "OPENAI_SESSION_REFRESH_UNAVAILABLE")
	require.Contains(t, err.Error(), "OPENAI_SESSION_REFRESH_CLIENT_FAILED")
}

func TestRefreshChatGPTSessionStoresFreshAccessToken(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/session" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accounts":{}}`))
			return
		}
		n := hits.Add(1)
		cookie := r.Header.Get("Cookie")
		if n == 1 {
			require.NotContains(t, cookie, "st-live", "prime request must not send sessionToken")
			http.SetCookie(w, &http.Cookie{Name: "oai-did", Value: "did-1", Path: "/"})
			_ = json.NewEncoder(w).Encode(map[string]any{"WARNING_BANNER": "do not share"})
			return
		}
		require.Contains(t, cookie, "st-live")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accessToken": "fresh-access-token",
			"user":        map[string]any{"email": "user@example.com", "id": "user-1"},
			"account":     map[string]any{"id": "acct-1", "planType": "plus"},
		})
	}))
	t.Cleanup(srv.Close)

	prevSession := chatGPTAuthSessionURL
	prevCheck := chatGPTAccountsCheckURL
	prevSub := chatGPTSubscriptionsURL
	chatGPTAuthSessionURL = srv.URL + "/api/auth/session"
	chatGPTAccountsCheckURL = srv.URL + "/backend-api/accounts/check"
	chatGPTSubscriptionsURL = srv.URL + "/backend-api/subscriptions"
	t.Cleanup(func() {
		chatGPTAuthSessionURL = prevSession
		chatGPTAccountsCheckURL = prevCheck
		chatGPTSubscriptionsURL = prevSub
	})

	svc := NewOpenAIOAuthService(nil, nil)
	svc.SetPrivacyClientFactory(func(string) (*req.Client, error) {
		return req.C().SetTimeout(5 * time.Second), nil
	})

	info, err := svc.RefreshChatGPTSession(context.Background(), "st-live", "")
	require.NoError(t, err)
	require.Equal(t, "fresh-access-token", info.AccessToken)
	require.Equal(t, "user@example.com", info.Email)
	require.Equal(t, "acct-1", info.ChatGPTAccountID)
	require.GreaterOrEqual(t, hits.Load(), int32(2))
}

func TestRefreshChatGPTSessionPrimesBeforeSessionCookie(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		cookie := r.Header.Get("Cookie")
		if strings.Contains(cookie, "st-live") && n == 1 {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`<html>Just a moment... cloudflare</html>`))
			return
		}
		if !strings.Contains(cookie, "st-live") {
			http.SetCookie(w, &http.Cookie{Name: "oai-did", Value: "did-1", Path: "/"})
			_ = json.NewEncoder(w).Encode(map[string]any{"WARNING_BANNER": "do not share"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accessToken": "primed-access-token",
			"user":        map[string]any{"email": "user@example.com"},
			"account":     map[string]any{"id": "acct-1", "planType": "plus"},
		})
	}))
	t.Cleanup(srv.Close)

	prevSession := chatGPTAuthSessionURL
	prevCheck := chatGPTAccountsCheckURL
	prevSub := chatGPTSubscriptionsURL
	chatGPTAuthSessionURL = srv.URL
	chatGPTAccountsCheckURL = srv.URL + "/backend-api/accounts/check"
	chatGPTSubscriptionsURL = srv.URL + "/backend-api/subscriptions"
	t.Cleanup(func() {
		chatGPTAuthSessionURL = prevSession
		chatGPTAccountsCheckURL = prevCheck
		chatGPTSubscriptionsURL = prevSub
	})

	svc := NewOpenAIOAuthService(nil, nil)
	svc.SetPrivacyClientFactory(func(string) (*req.Client, error) {
		return req.C().SetTimeout(5 * time.Second), nil
	})

	info, err := svc.RefreshChatGPTSession(context.Background(), "st-live", "")
	require.NoError(t, err)
	require.Equal(t, "primed-access-token", info.AccessToken)
	require.GreaterOrEqual(t, hits.Load(), int32(2))
}

func TestRefreshChatGPTSessionRejectsExpiredAccessTokenFromSessionJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "accounts/check") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"token_expired","message":"Provided authentication token is expired."}}`))
			return
		}
		if !strings.Contains(r.Header.Get("Cookie"), "st-live") {
			_ = json.NewEncoder(w).Encode(map[string]any{"WARNING_BANNER": "do not share"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accessToken": "expired-looking-jwt",
			"user":        map[string]any{"email": "user@example.com"},
			"account":     map[string]any{"id": "acct-1"},
		})
	}))
	t.Cleanup(srv.Close)

	prevSession := chatGPTAuthSessionURL
	prevCheck := chatGPTAccountsCheckURL
	prevSub := chatGPTSubscriptionsURL
	chatGPTAuthSessionURL = srv.URL
	chatGPTAccountsCheckURL = srv.URL + "/backend-api/accounts/check"
	chatGPTSubscriptionsURL = srv.URL + "/backend-api/subscriptions"
	t.Cleanup(func() {
		chatGPTAuthSessionURL = prevSession
		chatGPTAccountsCheckURL = prevCheck
		chatGPTSubscriptionsURL = prevSub
	})

	svc := NewOpenAIOAuthService(nil, nil)
	svc.SetPrivacyClientFactory(func(string) (*req.Client, error) {
		return req.C().SetTimeout(5 * time.Second), nil
	})

	_, err := svc.RefreshChatGPTSession(context.Background(), "st-live", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "OPENAI_SESSION_ACCESS_TOKEN_EXPIRED")
}

func TestRefreshChatGPTSessionCloudflareHTMLReturnsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html><body>Just a moment... cloudflare cf-ray</body></html>`))
	}))
	t.Cleanup(srv.Close)

	prev := chatGPTAuthSessionURL
	chatGPTAuthSessionURL = srv.URL
	t.Cleanup(func() { chatGPTAuthSessionURL = prev })

	svc := NewOpenAIOAuthService(nil, nil)
	svc.SetPrivacyClientFactory(func(string) (*req.Client, error) {
		return req.C().SetTimeout(5 * time.Second), nil
	})

	_, err := svc.RefreshChatGPTSession(context.Background(), "st-live", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "OPENAI_SESSION_REFRESH_CF_BLOCKED")
	require.NotContains(t, err.Error(), "<html")
}

func TestCheckChatGPTAccessTokenRejectsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"token_expired","message":"Provided authentication token is expired."}}`))
	}))
	t.Cleanup(srv.Close)

	prev := chatGPTAccountsCheckURL
	chatGPTAccountsCheckURL = srv.URL
	t.Cleanup(func() { chatGPTAccountsCheckURL = prev })

	svc := NewOpenAIOAuthService(nil, nil)
	svc.SetPrivacyClientFactory(func(string) (*req.Client, error) {
		return req.C().SetTimeout(5 * time.Second), nil
	})

	err := svc.CheckChatGPTAccessToken(context.Background(), "dead-at", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "OPENAI_ACCESS_TOKEN_EXPIRED")
}

func TestOpenAITokenRefresherNeedsRefreshForSessionToken(t *testing.T) {
	refresher := NewOpenAITokenRefresher(nil, nil)
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"chatgpt_session_token": "st-1",
			"access_token":          "at-1",
			"expires_at":            time.Now().Add(10 * 24 * time.Hour).Format(time.RFC3339),
		},
	}
	require.True(t, refresher.NeedsRefresh(account, 3*time.Minute))

	account.Credentials["session_refreshed_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
	require.True(t, refresher.NeedsRefresh(account, 3*time.Minute), "unverified session must keep refreshing")

	account.Credentials["chatgpt_session_verified"] = true
	require.False(t, refresher.NeedsRefresh(account, 3*time.Minute))
}
