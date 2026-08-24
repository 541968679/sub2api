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

func newOAuthFleetSoft429SettingHandler() *SettingHandler {
	return NewSettingHandler(service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{}), nil, nil, nil, nil, nil)
}

func TestSettingHandler_GetOAuthFleetSoft429Settings_DefaultsOff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newOAuthFleetSoft429SettingHandler()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/oauth-fleet-soft-429", nil)

	handler.GetOAuthFleetSoft429Settings(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.OAuthFleetSoft429Settings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.False(t, body.Data.Enabled)
	require.Equal(t, 20, body.Data.TTLSeconds)
	require.Equal(t, service.OAuthFleetSoft429LongResetSoft, body.Data.LongResetPolicy)
}

func TestSettingHandler_UpdateOAuthFleetSoft429Settings_RoundTripAndReject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newOAuthFleetSoft429SettingHandler()

	payload, err := json.Marshal(map[string]any{
		"enabled":                       true,
		"ttl_seconds":                   30,
		"long_reset_policy":             "soft",
		"long_reset_threshold_seconds":  60,
		"scope":                         "all_oauth",
		"platforms":                     []string{"openai", "anthropic"},
		"include_setup_token":           true,
		"soft_status_codes":             []int{429},
		"soft_body_codes":               []string{"rate_limit_exceeded"},
		"hard_body_codes":               []string{"usage_limit_reached"},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/oauth-fleet-soft-429", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateOAuthFleetSoft429Settings(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.OAuthFleetSoft429Settings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.True(t, body.Data.Enabled)
	require.Equal(t, 30, body.Data.TTLSeconds)

	bad, err := json.Marshal(map[string]any{
		"enabled":           true,
		"ttl_seconds":       4,
		"long_reset_policy": "soft",
		"scope":             "all_oauth",
		"soft_status_codes": []int{429},
	})
	require.NoError(t, err)
	badRec := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(badRec)
	c2.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/oauth-fleet-soft-429", bytes.NewReader(bad))
	c2.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateOAuthFleetSoft429Settings(c2)
	require.Equal(t, http.StatusBadRequest, badRec.Code)
}
