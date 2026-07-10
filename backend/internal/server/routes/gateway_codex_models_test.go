package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayRoutesCodexModelsManifestPathsAreRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	registered := make(map[string]bool)
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet {
			registered[route.Path] = true
		}
	}

	require.True(t, registered["/backend-api/codex/models"])
	require.True(t, registered["/v1/models"])
}

func TestShouldServeCodexModelsManifest(t *testing.T) {
	tests := []struct {
		name          string
		platform      string
		clientVersion string
		want          bool
	}{
		{name: "OpenAI Codex client", platform: service.PlatformOpenAI, clientVersion: "0.144.1", want: true},
		{name: "ordinary OpenAI client", platform: service.PlatformOpenAI, want: false},
		{name: "Anthropic client with version", platform: service.PlatformAnthropic, clientVersion: "0.144.1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/models?client_version="+tt.clientVersion, nil)
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: tt.platform},
			})

			require.Equal(t, tt.want, shouldServeCodexModelsManifest(c))
		})
	}
}
