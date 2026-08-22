//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newScheduleErrorWhitelistSettingHandler() *SettingHandler {
	return NewSettingHandler(service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{}), nil, nil, nil, nil, nil)
}

func TestSettingHandler_GetScheduleErrorWhitelist_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newScheduleErrorWhitelistSettingHandler()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/schedule-error-whitelist", nil)

	handler.GetScheduleErrorWhitelist(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.ScheduleErrorWhitelist `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Data.FamilyEnabled(service.ScheduleErrorFamilyClientInvalidRequest))
	require.True(t, body.Data.FamilyEnabled(service.ScheduleErrorFamilyGroupNoAccount))
	require.Len(t, body.Data.Families, len(service.ScheduleErrorFamilyIDs))
}

func TestSettingHandler_UpdateScheduleErrorWhitelist_RoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newScheduleErrorWhitelistSettingHandler()
	payload, err := json.Marshal(map[string]any{
		"families": map[string]any{
			"group_no_account": false,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/schedule-error-whitelist", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateScheduleErrorWhitelist(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.ScheduleErrorWhitelist `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.False(t, body.Data.FamilyEnabled(service.ScheduleErrorFamilyGroupNoAccount))
	require.True(t, body.Data.FamilyEnabled(service.ScheduleErrorFamilyClientInvalidRequest))
}

func TestSettingHandler_UpdateScheduleErrorWhitelist_RejectsUnknownFamily(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newScheduleErrorWhitelistSettingHandler()
	payload, err := json.Marshal(map[string]any{
		"families": map[string]any{
			"drop_all_upstream_failed": true,
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/schedule-error-whitelist", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateScheduleErrorWhitelist(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
