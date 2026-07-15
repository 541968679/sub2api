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
		userAgent     string
		originator    string
		want          bool
	}{
		{name: "OpenAI Codex CLI with client_version", platform: service.PlatformOpenAI, clientVersion: "0.144.1", want: true},
		{name: "ordinary OpenAI client", platform: service.PlatformOpenAI, want: false},
		{name: "Anthropic client with version", platform: service.PlatformAnthropic, clientVersion: "0.144.1", want: false},
		// Desktop omits client_version but identifies via UA / Originator.
		{
			name:       "OpenAI Desktop UA without client_version",
			platform:   service.PlatformOpenAI,
			userAgent:  "codex_chatgpt_desktop/1.2.3",
			want:       true,
		},
		{
			name:       "OpenAI Desktop Originator without client_version",
			platform:   service.PlatformOpenAI,
			originator: "Codex Desktop",
			want:       true,
		},
		{
			name:       "OpenAI VS Code without client_version",
			platform:   service.PlatformOpenAI,
			userAgent:  "codex_vscode/1.0.0",
			want:       true,
		},
		{
			name:       "Desktop UA on Grok group stays false",
			platform:   service.PlatformGrok,
			userAgent:  "codex_chatgpt_desktop/1.2.3",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			url := "/v1/models"
			if tt.clientVersion != "" {
				url += "?client_version=" + tt.clientVersion
			}
			c.Request = httptest.NewRequest(http.MethodGet, url, nil)
			if tt.userAgent != "" {
				c.Request.Header.Set("User-Agent", tt.userAgent)
			}
			if tt.originator != "" {
				c.Request.Header.Set("Originator", tt.originator)
			}
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: tt.platform},
			})

			require.Equal(t, tt.want, shouldServeCodexModelsManifest(c))
		})
	}
}
