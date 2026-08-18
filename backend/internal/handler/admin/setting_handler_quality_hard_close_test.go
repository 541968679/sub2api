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

func newQualityHardCloseSettingHandler() *SettingHandler {
	return NewSettingHandler(service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{}), nil, nil, nil, nil, nil)
}

func TestSettingHandler_GetQualityHardCloseSettings_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newQualityHardCloseSettingHandler()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/quality-hard-close", nil)

	handler.GetQualityHardCloseSettings(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.QualityHardCloseSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.False(t, body.Data.Enabled)
	require.False(t, body.Data.ScheduleUseFailoverErrorRate)
	require.Equal(t, 30, body.Data.PauseMinutes)
	require.Equal(t, service.QualityHardCloseConditionOr, body.Data.Condition)
}

func TestSettingHandler_UpdateQualityHardCloseSettings_RoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newQualityHardCloseSettingHandler()
	payload, err := json.Marshal(map[string]any{
		"enabled":             true,
		"max_p50_ttft_ms":     2500,
		"min_success_rate":    0.85,
		"pause_minutes":       40,
		"min_success_samples": 12,
		"min_ttft_samples":               6,
		"condition":                      "and",
		"schedule_use_failover_error_rate": true,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/quality-hard-close", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateQualityHardCloseSettings(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.QualityHardCloseSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Data.Enabled)
	require.True(t, body.Data.ScheduleUseFailoverErrorRate)
	require.Equal(t, 40, body.Data.PauseMinutes)
	require.Equal(t, "and", body.Data.Condition)
}

func TestSettingHandler_UpdateQualityHardCloseSettings_RejectsInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newQualityHardCloseSettingHandler()
	payload, err := json.Marshal(map[string]any{
		"enabled":             true,
		"pause_minutes":       0,
		"min_success_samples": 1,
		"min_ttft_samples":    1,
		"condition":           "or",
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/quality-hard-close", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateQualityHardCloseSettings(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
