//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type openAIQuotaHandlerAccountRepo struct {
	service.AccountRepository
	account *service.Account
}

func (r *openAIQuotaHandlerAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, service.ErrAccountNotFound
}

func TestOpenAIQuotaHandlerRejectsGrokAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &openAIQuotaHandlerAccountRepo{account: &service.Account{
		ID:       44,
		Platform: service.PlatformGrok,
		Type:     service.AccountTypeOAuth,
	}}
	quotaService := service.NewOpenAIQuotaService(repo, nil, &service.OpenAITokenProvider{}, func(string) (*req.Client, error) {
		t.Fatal("Grok account must be rejected before the upstream client is built")
		return nil, nil
	})
	handler := NewOpenAIOAuthHandler(nil, newStubAdminService(), quotaService)

	router := gin.New()
	router.GET("/api/v1/admin/openai/accounts/:id/quota", handler.QueryQuota)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/openai/accounts/44/quota", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"reason":"OPENAI_QUOTA_INVALID_PLATFORM"`)
}

func TestOpenAIQuotaHandlerHandlesMissingService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewOpenAIOAuthHandler(nil, newStubAdminService(), nil)

	router := gin.New()
	router.POST("/api/v1/admin/openai/accounts/:id/reset-quota", handler.ResetQuota)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai/accounts/44/reset-quota", strings.NewReader(`{"confirm":true,"redeem_request_id":"123e4567-e89b-42d3-a456-426614174000"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "openai quota service is not enabled")
}

func TestOpenAIQuotaHandlerRequiresConfirmedStableRedeemID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewOpenAIOAuthHandler(nil, newStubAdminService(), &service.OpenAIQuotaService{})
	router := gin.New()
	router.POST("/api/v1/admin/openai/accounts/:id/reset-quota", handler.ResetQuota)

	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "not confirmed", body: `{"confirm":false,"redeem_request_id":"123e4567-e89b-42d3-a456-426614174000"}`},
		{name: "invalid redeem id", body: `{"confirm":true,"redeem_request_id":"not-a-uuid"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/openai/accounts/44/reset-quota", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
