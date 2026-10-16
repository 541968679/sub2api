package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/inboundhost"
	"github.com/gin-gonic/gin"
)

func TestClientRequestID_StoresTrustedEntryHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ClientRequestID())
	r.GET("/", func(c *gin.Context) {
		host, _ := c.Request.Context().Value(ctxkey.InboundHost).(string)
		c.String(http.StatusOK, host)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "zerocode.kaynlab.com"
	req.Header.Set(inboundhost.HeaderEntryHost, "zerocode.kaynlab.asia")
	req.Header.Set(inboundhost.HeaderEntryVia, inboundhost.EntryViaHKRelay)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "zerocode.kaynlab.asia" {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestClientRequestID_IgnoresUntrustedEntryHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ClientRequestID())
	r.GET("/", func(c *gin.Context) {
		host, _ := c.Request.Context().Value(ctxkey.InboundHost).(string)
		c.String(http.StatusOK, host)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "zerocode.kaynlab.com"
	req.Header.Set(inboundhost.HeaderEntryHost, "zerocode.kaynlab.asia")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "zerocode.kaynlab.com" {
		t.Fatalf("spoofed host stored: %q", rec.Body.String())
	}
}
