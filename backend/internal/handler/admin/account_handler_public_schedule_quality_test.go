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

func newPublicScheduleAccountHandler() *AccountHandler {
	cache := service.NewMemoryPublicScheduleQualityCache()
	runtime := service.NewPublicScheduleQualityService(cache, nil, nil)
	handler := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler.SetPublicScheduleQualityService(runtime)
	return handler
}

func TestAccountHandler_PublicScheduleQualityStateRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newPublicScheduleAccountHandler()

	payload, err := json.Marshal(map[string]string{"state": "paused"})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/public-schedule-quality/state", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.SetPublicScheduleQualityState(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.PublicScheduleQualityView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, service.PublicScheduleStatePaused, body.Data.State)
}

func TestAccountHandler_GetBatchPublicScheduleQuality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newPublicScheduleAccountHandler()
	payload, err := json.Marshal(map[string][]int64{"account_ids": {1, 2}})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/public-schedule-quality/batch", bytes.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.GetBatchPublicScheduleQuality(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data struct {
			Views map[string]service.PublicScheduleQualityView `json:"views"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Contains(t, body.Data.Views, "1")
	require.Equal(t, service.PublicScheduleStateSelectable, body.Data.Views["1"].State)
}
