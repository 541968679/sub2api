package admin

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func bindGroupPlatformJSON(t *testing.T, dest any, body string) error {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c.ShouldBindJSON(dest)
}

func TestCompositeRouteTargetPlatform_AllowsCNProviders(t *testing.T) {
	for _, platform := range []string{"kimi", "zhipu", "deepseek", "minimax"} {
		var req CompositeRouteRequest
		body := fmt.Sprintf(`{"public_model":"m","target_platform":%q}`, platform)
		require.NoError(t, bindGroupPlatformJSON(t, &req, body))
		require.Equal(t, platform, req.TargetPlatform)
	}
}

func TestCreateGroupRequest_AllowsCNPlatforms(t *testing.T) {
	for _, platform := range []string{"kimi", "zhipu", "deepseek", "minimax"} {
		var req CreateGroupRequest
		body := fmt.Sprintf(`{"name":"g","platform":%q}`, platform)
		require.NoError(t, bindGroupPlatformJSON(t, &req, body), platform)
		require.Equal(t, platform, req.Platform)
	}
}
