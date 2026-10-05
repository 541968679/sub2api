package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseSubscriptionRejectsPrivateURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewProxyHandler(nil)
	router.POST("/api/v1/admin/proxies/parse-subscription", handler.ParseSubscription)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/parse-subscription", strings.NewReader(`{"url":"http://127.0.0.1:8080/sub","default_protocol":"socks5"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "SUBSCRIPTION_URL_INVALID")
}

func TestParseSubscriptionRejectsProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewProxyHandler(nil)
	router.POST("/api/v1/admin/proxies/parse-subscription", handler.ParseSubscription)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/parse-subscription", strings.NewReader(`{"url":"https://example.com/sub","default_protocol":"vmess"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Invalid default protocol")
}
