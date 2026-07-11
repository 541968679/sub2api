package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthHandler_CreateAccountFromCodexPAT(t *testing.T) {
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
}
