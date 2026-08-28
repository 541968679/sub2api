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

func TestSettingHandler_GetOpenAIWaitTimeoutSettings_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InvalidateOpenAIWaitTimeoutSettingsCacheForTest()
	t.Cleanup(service.InvalidateOpenAIWaitTimeoutSettingsCacheForTest)

	handler := NewSettingHandler(service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{}), nil, nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/openai-wait-timeout", nil)

	handler.GetOpenAIWaitTimeoutSettings(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.OpenAIWaitTimeoutSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, service.DefaultOpenAIHeaderWaitSeconds, body.Data.HeaderWaitSeconds)
	require.Equal(t, service.DefaultOpenAIFirstUsefulFrameSeconds, body.Data.FirstUsefulFrameSeconds)
}

func TestSettingHandler_UpdateOpenAIWaitTimeoutSettings_Validation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InvalidateOpenAIWaitTimeoutSettingsCacheForTest()
	t.Cleanup(service.InvalidateOpenAIWaitTimeoutSettingsCacheForTest)

	handler := NewSettingHandler(service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{}), nil, nil, nil, nil, nil)

	bad, err := json.Marshal(map[string]any{
		"header_wait_seconds":        3,
		"first_useful_frame_seconds": 30,
	})
	require.NoError(t, err)
	badRec := httptest.NewRecorder()
	badC, _ := gin.CreateTestContext(badRec)
	badC.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/openai-wait-timeout", bytes.NewReader(bad))
	badC.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateOpenAIWaitTimeoutSettings(badC)
	require.Equal(t, http.StatusBadRequest, badRec.Code)

	ok, err := json.Marshal(map[string]any{
		"header_wait_seconds":        120,
		"first_useful_frame_seconds": 10,
	})
	require.NoError(t, err)
	okRec := httptest.NewRecorder()
	okC, _ := gin.CreateTestContext(okRec)
	okC.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/openai-wait-timeout", bytes.NewReader(ok))
	okC.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateOpenAIWaitTimeoutSettings(okC)
	require.Equal(t, http.StatusOK, okRec.Code)

	var body struct {
		Data service.OpenAIWaitTimeoutSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(okRec.Body.Bytes(), &body))
	require.Equal(t, 120, body.Data.HeaderWaitSeconds)
	require.Equal(t, 10, body.Data.FirstUsefulFrameSeconds)
}
