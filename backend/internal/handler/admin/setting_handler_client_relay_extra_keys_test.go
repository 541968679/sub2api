//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_ClientRelayExtraResponseKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InvalidateClientRelayExtraKeysCacheForTest()
	t.Cleanup(service.InvalidateClientRelayExtraKeysCacheForTest)

	handler := NewSettingHandler(service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{}), nil, nil, nil, nil, nil)

	getRec := httptest.NewRecorder()
	getC, _ := gin.CreateTestContext(getRec)
	getC.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/client-relay-extra-response-keys", nil)
	handler.GetClientRelayExtraResponseKeys(getC)
	require.Equal(t, http.StatusOK, getRec.Code)
	var defaults struct {
		Data dto.ClientRelayExtraResponseKeysSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &defaults))
	require.Contains(t, defaults.Data.Keys, "cost_cny")
	require.Contains(t, defaults.Data.Keys, "billing_pending")
	require.Contains(t, defaults.Data.Keys, "trace_id")
	require.Contains(t, defaults.Data.Keys, "reasoning_available")
	require.Equal(t, defaults.Data.Keys, defaults.Data.Defaults)

	bad, err := json.Marshal(map[string]any{"keys": []string{"usage"}})
	require.NoError(t, err)
	badRec := httptest.NewRecorder()
	badC, _ := gin.CreateTestContext(badRec)
	badC.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/client-relay-extra-response-keys", bytes.NewReader(bad))
	badC.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateClientRelayExtraResponseKeys(badC)
	require.Equal(t, http.StatusBadRequest, badRec.Code)

	okBody, err := json.Marshal(map[string]any{"keys": []string{"cost_cny", "vendor_note"}})
	require.NoError(t, err)
	okRec := httptest.NewRecorder()
	okC, _ := gin.CreateTestContext(okRec)
	okC.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/client-relay-extra-response-keys", bytes.NewReader(okBody))
	okC.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateClientRelayExtraResponseKeys(okC)
	require.Equal(t, http.StatusOK, okRec.Code)
	var saved struct {
		Data dto.ClientRelayExtraResponseKeysSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(okRec.Body.Bytes(), &saved))
	require.Equal(t, []string{"cost_cny", "vendor_note"}, saved.Data.Keys)
	require.Contains(t, saved.Data.Defaults, "reasoning_available")
}
