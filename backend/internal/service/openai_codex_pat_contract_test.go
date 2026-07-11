package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAICodexPATContract(t *testing.T) {
	t.Run("PAT mode is isolated to OpenAI OAuth accounts", func(t *testing.T) {
		pat := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"auth_mode": OpenAIAuthModePersonalAccessToken,
			},
		}
		require.True(t, pat.IsOpenAIPersonalAccessToken())

		grok := *pat
		grok.Platform = PlatformGrok
		require.False(t, grok.IsOpenAIPersonalAccessToken())

		apiKey := *pat
		apiKey.Type = AccountTypeAPIKey
		require.False(t, apiKey.IsOpenAIPersonalAccessToken())
	})

	t.Run("validation uses Codex identity and preserves account metadata", func(t *testing.T) {
		var authorization, originator, userAgent string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization = r.Header.Get("authorization")
			originator = r.Header.Get("originator")
			userAgent = r.Header.Get("user-agent")
			w.Header().Set("content-type", "application/json")
			_, _ = w.Write([]byte(`{
				"email":"pat@example.com",
				"chatgpt_user_id":"user-pat",
				"chatgpt_account_id":"acct-pat",
				"chatgpt_plan_type":"team",
				"chatgpt_account_is_fedramp":true
			}`))
		}))
		defer server.Close()

		originalURL := openAICodexPATWhoamiURL
		openAICodexPATWhoamiURL = server.URL
		defer func() { openAICodexPATWhoamiURL = originalURL }()

		svc := NewOpenAIOAuthService(nil, nil)
		defer svc.Stop()

		info, err := svc.ValidateCodexPersonalAccessToken(context.Background(), " at-contract ", "")
		require.NoError(t, err)
		require.Equal(t, "Bearer at-contract", authorization)
		require.Equal(t, "codex_cli_rs", originator)
		require.Equal(t, codexCLIUserAgent, userAgent)
		require.Equal(t, OpenAIAuthModePersonalAccessToken, info.AuthMode)
		require.Equal(t, "acct-pat", info.ChatGPTAccountID)
		require.True(t, info.ChatGPTAccountFedRAMP)

		credentials := NormalizeOpenAIPersonalAccessTokenCredentials(nil, info, map[string]any{
			"access_token":    info.AccessToken,
			"refresh_token":   "stale-refresh",
			"id_token":        "stale-id",
			"expires_at":      "2026-01-01T00:00:00Z",
			"model_mapping":   map[string]any{"claude-opus-4-8": "gpt-5.5"},
			"custom_metadata": "keep",
		})
		require.NotContains(t, credentials, "refresh_token")
		require.NotContains(t, credentials, "id_token")
		require.NotContains(t, credentials, "expires_at")
		require.Equal(t, map[string]any{"claude-opus-4-8": "gpt-5.5"}, credentials["model_mapping"])
		require.Equal(t, "keep", credentials["custom_metadata"])
	})

	t.Run("FedRAMP header does not leak across platforms or accounts", func(t *testing.T) {
		headers := http.Header{"X-Openai-Fedramp": []string{"stale"}}
		fedRAMP := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"chatgpt_account_id":         "acct-fed",
				"chatgpt_account_is_fedramp": true,
			},
		}
		setOpenAIChatGPTAccountHeaders(headers, fedRAMP)
		require.Equal(t, "acct-fed", headers.Get("chatgpt-account-id"))
		require.Equal(t, "true", headers.Get("x-openai-fedramp"))

		nonFedRAMP := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
		setOpenAIChatGPTAccountHeaders(headers, nonFedRAMP)
		require.Empty(t, headers.Get("chatgpt-account-id"))
		require.Empty(t, headers.Get("x-openai-fedramp"))

		headers.Set("x-openai-fedramp", "stale")
		grok := &Account{Platform: PlatformGrok, Type: AccountTypeOAuth}
		setOpenAIChatGPTAccountHeaders(headers, grok)
		require.Equal(t, "stale", headers.Get("x-openai-fedramp"), "non-OpenAI request headers must be left to their own platform path")
	})
}
