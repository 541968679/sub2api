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

func newPublicScheduleQualitySettingHandler() *SettingHandler {
	return NewSettingHandler(service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{}), nil, nil, nil, nil, nil)
}

func TestSettingHandler_GetPublicScheduleQualitySettings_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newPublicScheduleQualitySettingHandler()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/public-schedule-quality", nil)

	handler.GetPublicScheduleQualitySettings(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.PublicScheduleQualitySettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.False(t, body.Data.Enabled)
	require.Equal(t, service.DefaultPublicScheduleWindowN, body.Data.TTFTWindowN)
	require.Equal(t, service.DefaultPublicScheduleCooldownMinutes, body.Data.CooldownMinutes)
}

func TestSettingHandler_UpdatePublicScheduleQualitySettings_RoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newPublicScheduleQualitySettingHandler()
	p50 := 2500
	rate := 0.85
	payload, err := json.Marshal(service.PublicScheduleQualitySettings{
		Enabled:               true,
		TTFTWindowN:           8,
		SuccessWindowN:        8,
		QualityMaxP50TTFTMs:   &p50,
		QualityMinSuccessRate: &rate,
		CooldownMinutes:       20,
		SoftCooldown:          true,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/public-schedule-quality", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdatePublicScheduleQualitySettings(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.PublicScheduleQualitySettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Data.Enabled)
	require.Equal(t, 8, body.Data.TTFTWindowN)
	require.Equal(t, 20, body.Data.CooldownMinutes)
	require.True(t, body.Data.SoftCooldown)
	require.NotNil(t, body.Data.QualityMaxP50TTFTMs)
	require.Equal(t, 2500, *body.Data.QualityMaxP50TTFTMs)
}
