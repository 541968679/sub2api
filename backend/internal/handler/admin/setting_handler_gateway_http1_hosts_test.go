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

func TestSettingHandler_GatewayHTTP1Hosts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InvalidateGatewayHTTP1HostsCacheForTest()
	t.Cleanup(service.InvalidateGatewayHTTP1HostsCacheForTest)

	handler := NewSettingHandler(service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{}), nil, nil, nil, nil, nil)

	getRec := httptest.NewRecorder()
	getC, _ := gin.CreateTestContext(getRec)
	getC.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/gateway-http1-hosts", nil)
	handler.GetGatewayHTTP1Hosts(getC)
	require.Equal(t, http.StatusOK, getRec.Code)
	var empty struct {
		Data dto.GatewayHTTP1HostsSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &empty))
	require.Empty(t, empty.Data.Hosts)

	bad, err := json.Marshal(map[string]any{"hosts": []string{"not a host"}})
	require.NoError(t, err)
	badRec := httptest.NewRecorder()
	badC, _ := gin.CreateTestContext(badRec)
	badC.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/gateway-http1-hosts", bytes.NewReader(bad))
	badC.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateGatewayHTTP1Hosts(badC)
	require.Equal(t, http.StatusBadRequest, badRec.Code)

	okBody, err := json.Marshal(map[string]any{"hosts": []string{"https://api.inferhub.dev/v1", "*.example.com"}})
	require.NoError(t, err)
	okRec := httptest.NewRecorder()
	okC, _ := gin.CreateTestContext(okRec)
	okC.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings/gateway-http1-hosts", bytes.NewReader(okBody))
	okC.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateGatewayHTTP1Hosts(okC)
	require.Equal(t, http.StatusOK, okRec.Code)
	var saved struct {
		Data dto.GatewayHTTP1HostsSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(okRec.Body.Bytes(), &saved))
	require.Equal(t, []string{"api.inferhub.dev", "*.example.com"}, saved.Data.Hosts)
}
