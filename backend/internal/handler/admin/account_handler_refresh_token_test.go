//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	updateCalled     bool
	clearErrorCalled bool
}

func (s *refreshTokenStub) GetAccount(ctx context.Context, id int64) (*service.Account, error) {
	return s.account, nil
}

func (s *refreshTokenStub) UpdateAccount(ctx context.Context, id int64, input *service.UpdateAccountInput) (*service.Account, error) {
	s.updateCalled = true
	s.lastUpdateCreds = input.Credentials
	return &service.Account{ID: id, Platform: s.account.Platform, Type: s.account.Type, Credentials: input.Credentials, Status: service.StatusActive}, nil
}

func (s *refreshTokenStub) ClearAccountError(ctx context.Context, id int64) (*service.Account, error) {
	s.clearErrorCalled = true
	return &service.Account{ID: id, Platform: s.account.Platform, Type: s.account.Type, Credentials: s.lastUpdateCreds, Status: service.StatusActive}, nil
}

func setupRefreshTokenHandler(adminSvc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
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
