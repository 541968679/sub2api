//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyHandlerGetByIDReturnsNotFoundForForeignKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyService := service.NewAPIKeyService(
		&apiKeySecurityHandlerRepo{key: &service.APIKey{
			ID:     123,
			UserID: 99,
			Key:    "sk-foreign",
			Name:   "foreign",
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	handler := NewAPIKeyHandler(apiKeyService)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys/123", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "123"}}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.GetByID(c)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "API key not found")
}

type apiKeySecurityHandlerRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *apiKeySecurityHandlerRepo) GetByID(context.Context, int64) (*service.APIKey, error) {
	return r.key, nil
}
