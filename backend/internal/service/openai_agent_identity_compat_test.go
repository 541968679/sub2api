package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAgentIdentityPassthroughKeepsSessionAndPromptCacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, privateKey := newTestAgentIdentityKey(t)
	account := &Account{
		ID:       24,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_mode":          OpenAIAuthModeAgentIdentity,
			"agent_runtime_id":   key.runtimeID,
			"agent_private_key":  privateKey,
			"task_id":            key.taskID,
			"chatgpt_account_id": "account-agent-passthrough",
		},
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","instructions":"Reply OK","input":[],"stream":true,"prompt_cache_key":"cache-agent"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("session_id", "client-session")
	c.Request.Header.Set("conversation_id", "client-conversation")
	c.Request.Header.Set("Authorization", "Bearer inbound-must-not-forward")

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.Equal(t, "AgentAssertion", strings.SplitN(req.Header.Get("Authorization"), " ", 2)[0])
	require.Equal(t, "account-agent-passthrough", req.Header.Get("chatgpt-account-id"))
	require.NotEqual(t, "client-session", req.Header.Get("session_id"))
	require.NotEqual(t, "client-conversation", req.Header.Get("conversation_id"))
	require.Equal(t, isolateOpenAISessionID(0, "client-session"), req.Header.Get("session_id"))
	require.Equal(t, isolateOpenAISessionID(0, "client-conversation"), req.Header.Get("conversation_id"))
	requestBody, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Contains(t, string(requestBody), `"prompt_cache_key":"cache-agent"`)

	oauthAccount := &Account{
		ID:       26,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"chatgpt_account_id": "account-agent-passthrough",
		},
	}
	oauthRecorder := httptest.NewRecorder()
	oauthContext, _ := gin.CreateTestContext(oauthRecorder)
	oauthContext.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	oauthContext.Request.Header.Set("session_id", "client-session")
	oauthContext.Request.Header.Set("conversation_id", "client-conversation")
	oauthReq, err := svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), oauthContext, oauthAccount, body, "oauth-token")
	require.NoError(t, err)
	require.Equal(t, oauthReq.Header.Get("session_id"), req.Header.Get("session_id"))
	require.Equal(t, oauthReq.Header.Get("conversation_id"), req.Header.Get("conversation_id"))
}

func TestOpenAIAuthenticationHeadersPreserveOAuthPATAndAPIKeyBearerModes(t *testing.T) {
	svc := &OpenAIGatewayService{}
	tests := []struct {
		name    string
		account *Account
		token   string
	}{
		{name: "oauth", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, token: "oauth-runtime-token"},
		{name: "personal access token", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"auth_mode": OpenAIAuthModePersonalAccessToken}}, token: "pat-runtime-token"},
		{name: "api key", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, token: "api-key-runtime-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers, err := svc.buildOpenAIAuthenticationHeaders(context.Background(), tt.account, tt.token)
			require.NoError(t, err)
			require.Equal(t, "Bearer "+tt.token, headers.Get("Authorization"))
		})
	}
}
