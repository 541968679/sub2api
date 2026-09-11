package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupModelAllowlistMiddleware_DefaultOffDoesNotReject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), &service.APIKey{
			Group: &service.Group{
				ID:       1,
				Platform: service.PlatformOpenAI,
				ModelAllowlist: service.GroupModelAllowlist{
					Enabled: true,
					Models:  []string{"only-this"},
				},
			},
		})
		c.Next()
	})
	r.Use(GroupModelAllowlist(nil))
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, called, "missing/false enforce must not reject gateway requests")
}
