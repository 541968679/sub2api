//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// refreshTokenStub overrides the account lookup/update/clear methods so the
// validate=false path of UpdateRefreshToken can be exercised without any
// upstream OAuth services.
type refreshTokenStub struct {
	*stubAdminService
	account          *service.Account
	lastUpdateCreds  map[string]any
	lastUpdateInput  *service.UpdateAccountInput
	updateCalled     bool
	clearErrorCalled bool
}

func (s *refreshTokenStub) GetAccount(ctx context.Context, id int64) (*service.Account, error) {
	return s.account, nil
}

func (s *refreshTokenStub) UpdateAccount(ctx context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	s.updateCalled = true
	s.lastUpdateCreds = input.Credentials
	s.lastUpdateInput = input
	return &service.Account{
		ID:          id,
		Platform:    s.account.Platform,
		Type:        s.account.Type,
		Credentials: input.Credentials,
		Extra:       input.Extra,
		Status:      service.StatusActive,
	}, nil
}

func (s *refreshTokenStub) ClearAccountError(ctx context.Context, id int64) (*service.Account, error) {
	s.clearErrorCalled = true
	return &service.Account{ID: id, Platform: s.account.Platform, Type: s.account.Type, Credentials: s.lastUpdateCreds, Status: service.StatusActive}, nil
}

func setupRefreshTokenHandler(adminSvc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/:id/refresh-token", handler.UpdateRefreshToken)
	return router
}

func doUpdateRefreshToken(router *gin.Engine, id string, body any) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/accounts/"+id+"/refresh-token", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

// 非 OAuth 账号（默认 stub 返回的账号 type 为空）应被拒绝。
func TestUpdateRefreshToken_NonOAuthReturns400(t *testing.T) {
	router := setupRefreshTokenHandler(newStubAdminService())
	w := doUpdateRefreshToken(router, "1", map[string]any{"refresh_token": "rt-new"})
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// 缺少 refresh_token 应在绑定阶段返回 400。
func TestUpdateRefreshToken_MissingTokenReturns400(t *testing.T) {
	router := setupRefreshTokenHandler(newStubAdminService())
	w := doUpdateRefreshToken(router, "1", map[string]any{})
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// validate=false：合并新 refresh_token（保留其他凭证字段）、清除 error 状态。
func TestUpdateRefreshToken_SkipValidationMergesAndReactivates(t *testing.T) {
	stub := &refreshTokenStub{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:       7,
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusError,
			Credentials: map[string]any{
				"access_token":  "old-at",
				"refresh_token": "old-rt",
				"project_id":    "p1",
			},
		},
	}
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": "rt-new",
		"validate":      false,
	})

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, stub.updateCalled, "UpdateAccount should be called")
	require.True(t, stub.clearErrorCalled, "ClearAccountError should re-activate the account")
	require.Equal(t, "rt-new", stub.lastUpdateCreds["refresh_token"], "new refresh_token should be persisted")
	require.Equal(t, "old-at", stub.lastUpdateCreds["access_token"], "access_token should be preserved (merge, not overwrite)")
	require.Equal(t, "p1", stub.lastUpdateCreds["project_id"], "project_id should be preserved")
}

func TestClassifyOpenAIRefreshTokenInput(t *testing.T) {
	t.Parallel()
	require.Equal(t, openAIRefreshTokenKindRaw, classifyOpenAIRefreshTokenInput("rt-plain").Kind)
	require.Equal(t, openAIRefreshTokenKindInvalidJSON, classifyOpenAIRefreshTokenInput(`["a"]`).Kind)
	require.Equal(t, openAIRefreshTokenKindInvalidJSON, classifyOpenAIRefreshTokenInput(`{"foo":1}`).Kind)

	embedded := classifyOpenAIRefreshTokenInput(`{"refresh_token":"rt-new"}`)
	require.Equal(t, openAIRefreshTokenKindEmbeddedRT, embedded.Kind)
	require.Equal(t, "rt-new", embedded.RefreshToken)

	session := classifyOpenAIRefreshTokenInput(`{"accessToken":"at","sessionToken":"st"}`)
	require.Equal(t, openAIRefreshTokenKindSessionJSON, session.Kind)
	require.Equal(t, "at", firstCodexString(session.Object, codexAccessTokenPaths...))
}

func TestUpdateRefreshToken_SessionJSONUpdatesAccessTokenAndIgnoresSessionToken(t *testing.T) {
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(time.Hour), map[string]any{
		"email": "json@example.com",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "acct-from-claim",
			"chatgpt_user_id":    "user-from-claim",
			"chatgpt_plan_type":  "pro",
		},
	})
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token":       "old-at",
		"chatgpt_account_id": "acct-from-json",
	})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": sessionJSONForTest(t, accessToken, "acct-from-json", "secret-session-token"),
		"validate":      true,
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, stub.updateCalled)
	require.Equal(t, accessToken, stub.lastUpdateCreds["access_token"])
	_, hasSession := stub.lastUpdateCreds["session_token"]
	require.False(t, hasSession)
	require.Equal(t, "secret-session-token", stub.lastUpdateCreds["chatgpt_session_token"])
	require.NotEqual(t, "secret-session-token", stub.lastUpdateCreds["refresh_token"])
	require.Nil(t, stub.lastUpdateCreds["refresh_token"])
	require.Equal(t, true, stub.lastUpdateInput.Extra["session_token_present"])
	require.NotNil(t, stub.lastUpdateInput.ExpiresAt)
	require.NotNil(t, stub.lastUpdateInput.AutoPauseOnExpired)
	require.True(t, *stub.lastUpdateInput.AutoPauseOnExpired)
}

func TestUpdateRefreshToken_SessionJSONPreservesExistingRefreshToken(t *testing.T) {
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(time.Hour), nil)
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token":       "old-at",
		"refresh_token":      "old-rt",
		"client_id":          "old-client",
		"chatgpt_account_id": "acct-1",
	})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": sessionJSONForTest(t, accessToken, "acct-1", "st-must-not-win"),
		"validate":      true,
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, accessToken, stub.lastUpdateCreds["access_token"])
	require.Equal(t, "old-rt", stub.lastUpdateCreds["refresh_token"])
	require.Equal(t, "old-client", stub.lastUpdateCreds["client_id"])
	require.Nil(t, stub.lastUpdateInput.ExpiresAt)
	require.Nil(t, stub.lastUpdateInput.AutoPauseOnExpired)
}

func TestUpdateRefreshToken_ExpiredSessionJSONRejectedWhenValidating(t *testing.T) {
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(-time.Hour), nil)
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token":       "old-at",
		"chatgpt_account_id": "acct-1",
	})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": sessionJSONForTest(t, accessToken, "acct-1", "st"),
	})

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"reason":"OPENAI_SESSION_ACCESS_TOKEN_EXPIRED"`)
	require.False(t, stub.updateCalled)
}

func TestUpdateRefreshToken_ExpiredSessionJSONSavedWhenValidationSkipped(t *testing.T) {
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(-time.Hour), nil)
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token":       "old-at",
		"chatgpt_account_id": "acct-1",
	})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": sessionJSONForTest(t, accessToken, "acct-1", "st"),
		"validate":      false,
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, stub.updateCalled)
	require.Equal(t, accessToken, stub.lastUpdateCreds["access_token"])
}

func TestUpdateRefreshToken_SessionJSONIdentityMismatchRejected(t *testing.T) {
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(time.Hour), nil)
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token":       "old-at",
		"chatgpt_account_id": "acct-old",
	})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": sessionJSONForTest(t, accessToken, "acct-new", "st"),
		"validate":      true,
	})

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"reason":"OPENAI_SESSION_IDENTITY_MISMATCH"`)
	require.False(t, stub.updateCalled)
}

func TestUpdateRefreshToken_SessionJSONBackfillsMissingAccountID(t *testing.T) {
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(time.Hour), nil)
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token": "old-at",
	})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": sessionJSONForTest(t, accessToken, "acct-new", "st"),
		"validate":      true,
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "acct-new", stub.lastUpdateCreds["chatgpt_account_id"])
}

func TestUpdateRefreshToken_PATRejectsSessionJSON(t *testing.T) {
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(time.Hour), nil)
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token": "at-existing",
		"auth_mode":    service.OpenAIAuthModePersonalAccessToken,
	})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": sessionJSONForTest(t, accessToken, "acct-1", "st"),
		"validate":      false,
	})

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"reason":"OPENAI_SESSION_AUTH_MODE_MISMATCH"`)
	require.False(t, stub.updateCalled)
}

func TestUpdateRefreshToken_EmbeddedRefreshTokenJSONUsesRTPath(t *testing.T) {
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token":  "old-at",
		"refresh_token": "old-rt",
	})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": `{"refresh_token":"rt-new"}`,
		"validate":      false,
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "rt-new", stub.lastUpdateCreds["refresh_token"])
	require.Equal(t, "old-at", stub.lastUpdateCreds["access_token"])
}

func TestPlanOpenAISessionJSONUpdate_ExpiredAccessTokenWithRefreshTokenHandsOff(t *testing.T) {
	t.Parallel()
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(-time.Hour), nil)
	account := &service.Account{
		ID:       7,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "old-at",
			"chatgpt_account_id": "acct-1",
		},
	}
	obj := sessionObjectForTest(t, accessToken, "acct-1", "st-must-not-win")
	obj["refresh_token"] = "rt-from-json"

	plan := planOpenAISessionJSONUpdate(account, obj, true, time.Now().UTC())
	require.Empty(t, plan.ErrorCode, plan.ErrorMessage)
	require.Equal(t, "rt-from-json", plan.RefreshToken)
}

func TestPlanOpenAISessionJSONUpdate_IdentityMismatchWinsOverExpiredAccessToken(t *testing.T) {
	t.Parallel()
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(-time.Hour), nil)
	account := &service.Account{
		ID:       7,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "old-at",
			"chatgpt_account_id": "acct-old",
		},
	}

	plan := planOpenAISessionJSONUpdate(account, sessionObjectForTest(t, accessToken, "acct-new", "st"), true, time.Now().UTC())
	require.Equal(t, "OPENAI_SESSION_IDENTITY_MISMATCH", plan.ErrorCode)
	require.Empty(t, plan.RefreshToken)
}

func TestUpdateRefreshToken_ExpiredSessionJSONWithRefreshTokenUsesRTPath(t *testing.T) {
	accessToken := buildCodexImportTestJWT(t, time.Now().Add(-time.Hour), nil)
	stub := newOpenAIRefreshTokenStub(map[string]any{
		"access_token":       "old-at",
		"refresh_token":      "old-rt",
		"chatgpt_account_id": "acct-1",
	})
	router := setupRefreshTokenHandler(stub)
	obj := sessionObjectForTest(t, accessToken, "acct-1", "st-must-not-win")
	obj["refresh_token"] = "rt-from-json"
	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": string(raw),
		"validate":      false,
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "rt-from-json", stub.lastUpdateCreds["refresh_token"])
	require.Equal(t, "old-at", stub.lastUpdateCreds["access_token"])
	_, hasSession := stub.lastUpdateCreds["session_token"]
	require.False(t, hasSession)
	require.NotEqual(t, "st-must-not-win", stub.lastUpdateCreds["refresh_token"])
}

func TestUpdateRefreshToken_JSONArrayRejected(t *testing.T) {
	stub := newOpenAIRefreshTokenStub(map[string]any{"access_token": "old-at"})
	router := setupRefreshTokenHandler(stub)

	w := doUpdateRefreshToken(router, "7", map[string]any{
		"refresh_token": `["eyJhbGciOiJub25lIn0.e30."]`,
		"validate":      false,
	})

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"reason":"OPENAI_SESSION_JSON_INVALID"`)
	require.False(t, stub.updateCalled)
}

func newOpenAIRefreshTokenStub(credentials map[string]any) *refreshTokenStub {
	return &refreshTokenStub{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:          7,
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Status:      service.StatusError,
			Credentials: credentials,
		},
	}
}

func sessionObjectForTest(t *testing.T, accessToken, accountID, sessionToken string) map[string]any {
	t.Helper()
	return map[string]any{
		"user": map[string]any{
			"id":    "user-from-json",
			"email": "json@example.com",
			"name":  "Test User",
		},
		"account": map[string]any{
			"id":       accountID,
			"planType": "pro",
		},
		"accessToken":  accessToken,
		"sessionToken": sessionToken,
		"expires":      time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano),
	}
}

func sessionJSONForTest(t *testing.T, accessToken, accountID, sessionToken string) string {
	t.Helper()
	raw, err := json.Marshal(sessionObjectForTest(t, accessToken, accountID, sessionToken))
	if err != nil {
		t.Fatalf("marshal session json: %v", err)
	}
	return string(raw)
}
