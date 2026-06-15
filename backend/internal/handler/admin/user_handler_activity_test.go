//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userConcurrencyCacheStub struct {
	loads     map[int64]*service.UserLoadInfo
	requested []service.UserWithConcurrency
}

func (s *userConcurrencyCacheStub) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (s *userConcurrencyCacheStub) ReleaseAccountSlot(context.Context, int64, string) error {
	return nil
}
func (s *userConcurrencyCacheStub) GetAccountConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}
func (s *userConcurrencyCacheStub) GetAccountConcurrencyBatch(context.Context, []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}
func (s *userConcurrencyCacheStub) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}
func (s *userConcurrencyCacheStub) DecrementAccountWaitCount(context.Context, int64) error {
	return nil
}
func (s *userConcurrencyCacheStub) GetAccountWaitingCount(context.Context, int64) (int, error) {
	return 0, nil
}
func (s *userConcurrencyCacheStub) AcquireUserSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (s *userConcurrencyCacheStub) ReleaseUserSlot(context.Context, int64, string) error {
	return nil
}
func (s *userConcurrencyCacheStub) GetUserConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}
func (s *userConcurrencyCacheStub) IncrementWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}
func (s *userConcurrencyCacheStub) DecrementWaitCount(context.Context, int64) error {
	return nil
}
func (s *userConcurrencyCacheStub) GetAccountsLoadBatch(context.Context, []service.AccountWithConcurrency) (map[int64]*service.AccountLoadInfo, error) {
	return map[int64]*service.AccountLoadInfo{}, nil
}
func (s *userConcurrencyCacheStub) GetUsersLoadBatch(_ context.Context, users []service.UserWithConcurrency) (map[int64]*service.UserLoadInfo, error) {
	s.requested = append([]service.UserWithConcurrency(nil), users...)
	return s.loads, nil
}
func (s *userConcurrencyCacheStub) CleanupExpiredAccountSlots(context.Context, int64) error {
	return nil
}
func (s *userConcurrencyCacheStub) CleanupStaleProcessSlots(context.Context, string) error {
	return nil
}

func TestUserHandlerListIncludesActivityFieldsAndSortParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastLoginAt := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(30 * time.Minute)
	lastUsedAt := lastLoginAt.Add(90 * time.Minute)

	adminSvc := newStubAdminService()
	adminSvc.users = []service.User{
		{
			ID:           7,
			Email:        "activity@example.com",
			Username:     "activity-user",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			LastActiveAt: &lastActiveAt,
			LastUsedAt:   &lastUsedAt,
			CreatedAt:    lastLoginAt.Add(-24 * time.Hour),
			UpdatedAt:    lastLoginAt,
		},
	}
	handler := NewUserHandler(adminSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/users?sort_by=last_used_at&sort_order=asc&search=activity",
		nil,
	)

	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "last_used_at", adminSvc.lastListUsers.sortBy)
	require.Equal(t, "asc", adminSvc.lastListUsers.sortOrder)
	require.Equal(t, "activity", adminSvc.lastListUsers.filters.Search)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				LastActiveAt *time.Time `json:"last_active_at"`
				LastUsedAt   *time.Time `json:"last_used_at"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Items, 1)
	require.WithinDuration(t, lastActiveAt, *resp.Data.Items[0].LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *resp.Data.Items[0].LastUsedAt, time.Second)
}

func TestUserHandlerListSortsByCurrentConcurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	adminSvc := newStubAdminService()
	adminSvc.users = []service.User{
		{ID: 1, Email: "one@example.com", Concurrency: 100, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: 2, Email: "two@example.com", Concurrency: 1, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: 3, Email: "three@example.com", Concurrency: 50, Status: service.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	concurrencyCache := &userConcurrencyCacheStub{
		loads: map[int64]*service.UserLoadInfo{
			1: {UserID: 1, CurrentConcurrency: 2},
			2: {UserID: 2, CurrentConcurrency: 7},
			3: {UserID: 3, CurrentConcurrency: 3},
		},
	}
	handler := NewUserHandler(adminSvc, service.NewConcurrencyService(concurrencyCache))

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/users?page=1&page_size=2&sort_by=current_concurrency&sort_order=desc",
		nil,
	)

	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, adminSvc.lastListUsers.calls)
	require.Equal(t, 1, adminSvc.lastListUsers.page)
	require.Equal(t, 1000, adminSvc.lastListUsers.pageSize)
	require.Equal(t, "id", adminSvc.lastListUsers.sortBy)
	require.Equal(t, "asc", adminSvc.lastListUsers.sortOrder)
	require.Len(t, concurrencyCache.requested, 3)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				ID                 int64 `json:"id"`
				Concurrency        int   `json:"concurrency"`
				CurrentConcurrency int   `json:"current_concurrency"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, int64(3), resp.Data.Total)
	require.Len(t, resp.Data.Items, 2)
	require.Equal(t, int64(2), resp.Data.Items[0].ID)
	require.Equal(t, 7, resp.Data.Items[0].CurrentConcurrency)
	require.Equal(t, 1, resp.Data.Items[0].Concurrency)
	require.Equal(t, int64(3), resp.Data.Items[1].ID)
	require.Equal(t, 3, resp.Data.Items[1].CurrentConcurrency)
}

func TestUserHandlerGetByIDIncludesActivityFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	lastLoginAt := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(30 * time.Minute)
	lastUsedAt := lastLoginAt.Add(90 * time.Minute)

	adminSvc := newStubAdminService()
	adminSvc.users = []service.User{
		{
			ID:           8,
			Email:        "detail@example.com",
			Username:     "detail-user",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			LastActiveAt: &lastActiveAt,
			LastUsedAt:   &lastUsedAt,
			CreatedAt:    lastLoginAt.Add(-24 * time.Hour),
			UpdatedAt:    lastLoginAt,
		},
	}
	handler := NewUserHandler(adminSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "8"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/8", nil)

	handler.GetByID(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			LastActiveAt *time.Time `json:"last_active_at"`
			LastUsedAt   *time.Time `json:"last_used_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.WithinDuration(t, lastActiveAt, *resp.Data.LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *resp.Data.LastUsedAt, time.Second)
}
