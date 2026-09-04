package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	require.False(t, openaiChatGPTSessionNeedsRefresh(account, 3*time.Minute), "fresh session refresh should wait")

	account.Credentials["refresh_token"] = "rt-1"
	require.False(t, openaiChatGPTSessionNeedsRefresh(account, 3*time.Minute), "OAuth RT accounts do not use session refresh")
}

func TestRefreshChatGPTSessionStoresFreshAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/session" {
			require.Contains(t, r.Header.Get("Cookie"), "st-live")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"accessToken": "fresh-access-token",
				"user":        map[string]any{"email": "user@example.com", "id": "user-1"},
				"account":     map[string]any{"id": "acct-1", "planType": "plus"},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"accounts":{}}`))
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
	require.Contains(t, err.Error(), "OPENAI_ACCESS_TOKEN_REJECTED")
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
	require.False(t, refresher.NeedsRefresh(account, 3*time.Minute))
}
