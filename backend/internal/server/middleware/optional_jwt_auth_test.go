//go:build unit

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOptionalJWTAuth_MissingHeaderContinuesAnonymously(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(optionalJWTAuth(nil, nil, nil))
	r.GET("/x", func(c *gin.Context) {
		_, ok := GetAuthSubjectFromContext(c)
		c.JSON(http.StatusOK, gin.H{"authed": ok})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"authed":false`)
}
