//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type getByIDAdminStub struct {
	service.AdminService
}

func (s *getByIDAdminStub) GetUser(_ context.Context, _ int64) (*service.User, error) {
	return nil, service.ErrUserNotFound
}

func (s *getByIDAdminStub) GetUserIncludeDeleted(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Email: "deleted@example.com"}, nil
}

func setupGetByIDRouter(svc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewUserHandler(svc, nil, nil, nil)
	router.GET("/admin/users/:id", handler.GetByID)
	return router
}

func TestAdminUserGetByID_IncludeDeleted(t *testing.T) {
	router := setupGetByIDRouter(&getByIDAdminStub{AdminService: newStubAdminService()})

	t.Run("normal path still hides a deleted user", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/admin/users/7", nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("explicit include_deleted returns the historical user", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/admin/users/7?include_deleted=true", nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)
	})
}
