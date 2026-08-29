//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupUnbindSubscriptionRouter(adminSvc *stubAdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/unbind-subscription-groups-by-rate", handler.UnbindSubscriptionGroupsByRate)
	return router
}

func TestAccountHandlerUnbindSubscription_MissingMinRate(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupUnbindSubscriptionRouter(adminSvc)

	body, err := json.Marshal(map[string]any{"dry_run": false})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/unbind-subscription-groups-by-rate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, adminSvc.lastUnbindSubscriptionInput)
	require.Contains(t, rec.Body.String(), "min_rate_multiplier")
}

func TestAccountHandlerUnbindSubscription_OmittedDryRunDoesNotApply(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupUnbindSubscriptionRouter(adminSvc)

	body, err := json.Marshal(map[string]any{"min_rate_multiplier": 1.0})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/unbind-subscription-groups-by-rate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastUnbindSubscriptionInput)
	require.True(t, adminSvc.lastUnbindSubscriptionInput.DryRun)
	require.Equal(t, 1.0, adminSvc.lastUnbindSubscriptionInput.MinRateMultiplier)
	require.False(t, adminSvc.lastUnbindSubscriptionInput.AllowEmptyGroups)
}

func TestAccountHandlerUnbindSubscription_ExplicitApplyPassesDryRunFalse(t *testing.T) {
	adminSvc := newStubAdminService()
	router := setupUnbindSubscriptionRouter(adminSvc)

	body, err := json.Marshal(map[string]any{
		"min_rate_multiplier": 1.2,
		"platform":            "openai",
		"dry_run":             false,
		"allow_empty_groups":  true,
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/unbind-subscription-groups-by-rate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, adminSvc.lastUnbindSubscriptionInput)
	require.False(t, adminSvc.lastUnbindSubscriptionInput.DryRun)
	require.Equal(t, 1.2, adminSvc.lastUnbindSubscriptionInput.MinRateMultiplier)
	require.Equal(t, "openai", adminSvc.lastUnbindSubscriptionInput.Platform)
	require.True(t, adminSvc.lastUnbindSubscriptionInput.AllowEmptyGroups)
}
