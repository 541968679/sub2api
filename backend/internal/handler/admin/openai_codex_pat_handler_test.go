package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type stubOpenAICodexPATValidator struct {
	info *service.OpenAITokenInfo
	err  error
}

func (s stubOpenAICodexPATValidator) ValidateCodexPersonalAccessToken(context.Context, string, string) (*service.OpenAITokenInfo, error) {
	return s.info, s.err
}

func TestOpenAIOAuthHandler_CreateAccountFromCodexPAT(t *testing.T) {
	t.Run("rejects invalid PAT without persistence", func(t *testing.T) {
		adminSvc := newStubAdminService()
		oauthSvc := service.NewOpenAIOAuthService(nil, nil)
		defer oauthSvc.Stop()
		h := NewOpenAIOAuthHandler(oauthSvc, adminSvc)

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/admin/openai/create-from-codex-pat", h.CreateAccountFromCodexPAT)

		body := `{
		"access_token":" not-a-pat ",
		"credential_extras":{"model_mapping":{"claude-opus-4-8":"gpt-5.5"},"refresh_token":"blocked"},
		"extra":{"openai_claude_gpt_bridge_enabled":true},
		"group_ids":[7]
	}`
		req := httptest.NewRequest(http.MethodPost, "/admin/openai/create-from-codex-pat", strings.NewReader(body))
		req.Header.Set("content-type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.Empty(t, adminSvc.createdAccounts, "invalid PAT must be rejected before account persistence")
		require.Contains(t, w.Body.String(), "OPENAI_CODEX_PAT_INVALID_PREFIX")
	})

	t.Run("never returns or duplicates the PAT", func(t *testing.T) {
		const token = "at-contract-secret-value"
		adminSvc := newStubAdminService()
		oauthSvc := service.NewOpenAIOAuthService(nil, nil)
		defer oauthSvc.Stop()
		h := NewOpenAIOAuthHandler(oauthSvc, adminSvc)
		h.codexPATValidator = stubOpenAICodexPATValidator{info: &service.OpenAITokenInfo{
			AccessToken:           token,
			AuthMode:              service.OpenAIAuthModePersonalAccessToken,
			Email:                 "pat@example.com",
			ChatGPTAccountID:      "acct-pat",
			ChatGPTUserID:         "user-pat",
			ChatGPTAccountFedRAMP: true,
			PlanType:              "team",
		}}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/admin/openai/create-from-codex-pat", h.CreateAccountFromCodexPAT)
		body := `{
			"access_token":"at-contract-secret-value",
			"credential_extras":{
				"model_mapping":{"claude-opus-4-8":"gpt-5.5"},
				"nested":{"access_token":"at-other-secret"}
			},
			"extra":{
				"openai_claude_gpt_bridge_enabled":true,
				"access_token":"at-contract-secret-value",
				"nested":{"authorization":"Bearer at-contract-secret-value"}
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/admin/openai/create-from-codex-pat", strings.NewReader(body))
		req.Header.Set("content-type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NotContains(t, w.Body.String(), token)
		require.NotContains(t, w.Body.String(), "at-other-secret")

		var envelope struct {
			Data struct {
				Credentials map[string]any `json:"credentials"`
				Extra       map[string]any `json:"extra"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		require.Nil(t, envelope.Data.Credentials)
		require.Equal(t, true, envelope.Data.Extra["openai_claude_gpt_bridge_enabled"])
		require.Equal(t, openAICodexPATFingerprint(token), envelope.Data.Extra["access_token_sha256"])
		require.NotContains(t, envelope.Data.Extra, "access_token")
		require.Equal(t, map[string]any{}, envelope.Data.Extra["nested"])

		require.Len(t, adminSvc.createdAccounts, 1)
		created := adminSvc.createdAccounts[0]
		require.Equal(t, token, created.Credentials["access_token"], "the primary encrypted credentials path must retain the PAT")
		require.NotContains(t, created.Extra, "access_token")
		require.Equal(t, map[string]any{}, created.Extra["nested"])
		credentialNested, ok := created.Credentials["nested"].(map[string]any)
		require.True(t, ok)
		require.Empty(t, credentialNested)
	})
}
