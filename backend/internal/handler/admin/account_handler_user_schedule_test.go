//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAccountHandlerForUserScheduleTest(adminSvc *stubAdminService) *AccountHandler {
	return NewAccountHandler(
		adminSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

func TestAccountHandler_Update_UserScheduleForwardsFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := newAccountHandlerForUserScheduleTest(adminSvc)
	router := gin.New()
	router.PUT("/api/v1/admin/accounts/:id", handler.Update)

	mode := "allow"
	body := map[string]any{
		"user_schedule_mode": mode,
		"schedule_user_ids":  []int64{16, 42},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/9", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(9), adminSvc.lastUpdateAccountID)
	require.NotNil(t, adminSvc.lastUpdateAccountInput)
	require.NotNil(t, adminSvc.lastUpdateAccountInput.UserScheduleMode)
	require.Equal(t, "allow", *adminSvc.lastUpdateAccountInput.UserScheduleMode)
	require.NotNil(t, adminSvc.lastUpdateAccountInput.ScheduleUserIDs)
	require.Equal(t, []int64{16, 42}, *adminSvc.lastUpdateAccountInput.ScheduleUserIDs)
}

func TestAccountHandler_BulkUpdate_UserScheduleOmitDoesNotForward(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := newAccountHandlerForUserScheduleTest(adminSvc)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/bulk-update", handler.BulkUpdate)

	body := map[string]any{
		"account_ids": []int64{1, 2},
		"status":      "active",
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastBulkUpdateInput)
	require.Nil(t, adminSvc.lastBulkUpdateInput.UserScheduleMode)
	require.Nil(t, adminSvc.lastBulkUpdateInput.ScheduleUserIDs)
}

func TestAccountHandler_BulkUpdate_UserScheduleOverwrite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := newAccountHandlerForUserScheduleTest(adminSvc)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/bulk-update", handler.BulkUpdate)

	mode := service.UserScheduleModeDeny
	body := map[string]any{
		"account_ids":        []int64{3},
		"user_schedule_mode": mode,
		"schedule_user_ids":  []int64{16},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastBulkUpdateInput)
	require.NotNil(t, adminSvc.lastBulkUpdateInput.UserScheduleMode)
	require.Equal(t, mode, *adminSvc.lastBulkUpdateInput.UserScheduleMode)
	require.NotNil(t, adminSvc.lastBulkUpdateInput.ScheduleUserIDs)
	require.Equal(t, []int64{16}, *adminSvc.lastBulkUpdateInput.ScheduleUserIDs)
}

func TestAccountHandler_Update_IndependentScheduleFieldsForward(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := newAccountHandlerForUserScheduleTest(adminSvc)
	router := gin.New()
	router.PUT("/api/v1/admin/accounts/:id", handler.Update)

	body := map[string]any{
		"allow_user_ids": []int64{16},
		"deny_user_ids":  []int64{42},
		"user_concurrencies": []map[string]any{
			{"user_id": 16, "max_concurrency": 5},
		},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/9", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastUpdateAccountInput)
	require.Equal(t, []int64{16}, *adminSvc.lastUpdateAccountInput.AllowUserIDs)
	require.Equal(t, []int64{42}, *adminSvc.lastUpdateAccountInput.DenyUserIDs)
	require.NotNil(t, adminSvc.lastUpdateAccountInput.UserConcurrencies)
	require.Equal(t, int64(16), (*adminSvc.lastUpdateAccountInput.UserConcurrencies)[0].UserID)
	require.Equal(t, 5, (*adminSvc.lastUpdateAccountInput.UserConcurrencies)[0].MaxConcurrency)
}

func TestAccountHandler_Update_UserConcurrencyPatchForward(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := newAccountHandlerForUserScheduleTest(adminSvc)
	router := gin.New()
	router.PUT("/api/v1/admin/accounts/:id", handler.Update)

	body := map[string]any{
		"user_concurrency_patch": map[string]any{"user_id": 16, "max_concurrency": 3},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/4", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastUpdateAccountInput.UserConcurrencyPatch)
	require.Equal(t, int64(16), adminSvc.lastUpdateAccountInput.UserConcurrencyPatch.UserID)
	require.NotNil(t, adminSvc.lastUpdateAccountInput.UserConcurrencyPatch.MaxConcurrency)
	require.Equal(t, 3, *adminSvc.lastUpdateAccountInput.UserConcurrencyPatch.MaxConcurrency)
}

func TestAccountHandler_Update_FullFormPayloadForwardsCaps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := newAccountHandlerForUserScheduleTest(adminSvc)
	router := gin.New()
	router.PUT("/api/v1/admin/accounts/:id", handler.Update)

	body := map[string]any{
		"name":           "acc",
		"notes":          "",
		"proxy_id":       0,
		"concurrency":    1,
		"load_factor":    0,
		"priority":       1,
		"rate_multiplier": 1,
		"status":         "active",
		"group_ids":      []int64{},
		"expires_at":     0,
		"allow_user_ids": []int64{},
		"deny_user_ids":  []int64{1},
		"user_concurrencies": []map[string]any{
			{"user_id": 1, "max_concurrency": 5},
		},
		"auto_pause_on_expired": true,
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/3", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastUpdateAccountInput)
	require.Equal(t, []int64{1}, *adminSvc.lastUpdateAccountInput.DenyUserIDs)
	require.NotNil(t, adminSvc.lastUpdateAccountInput.UserConcurrencies)
	require.Equal(t, int64(1), (*adminSvc.lastUpdateAccountInput.UserConcurrencies)[0].UserID)
	require.Equal(t, 5, (*adminSvc.lastUpdateAccountInput.UserConcurrencies)[0].MaxConcurrency)
}

func TestAccountHandler_BulkUpdate_AllowDenyForwardNoCaps(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := newAccountHandlerForUserScheduleTest(adminSvc)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/bulk-update", handler.BulkUpdate)

	body := map[string]any{
		"account_ids":    []int64{3},
		"allow_user_ids": []int64{16},
		"deny_user_ids":  []int64{42},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/bulk-update", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastBulkUpdateInput)
	require.Equal(t, []int64{16}, *adminSvc.lastBulkUpdateInput.AllowUserIDs)
	require.Equal(t, []int64{42}, *adminSvc.lastBulkUpdateInput.DenyUserIDs)
	require.Nil(t, adminSvc.lastBulkUpdateInput.UserConcurrencies)
	require.Nil(t, adminSvc.lastBulkUpdateInput.UserConcurrencyPatch)
}
