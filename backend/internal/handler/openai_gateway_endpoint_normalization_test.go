package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestOpenAIUpstreamEndpoint_ViaGetUpstreamEndpoint verifies that the
// unified GetUpstreamEndpoint helper produces the same results as the
// former normalizedOpenAIUpstreamEndpoint for OpenAI platform requests.
func TestOpenAIUpstreamEndpoint_ViaGetUpstreamEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "responses root maps to responses upstream",
			path: "/v1/responses",
			want: EndpointResponses,
		},
		{
			name: "responses compact keeps compact suffix",
			path: "/openai/v1/responses/compact",
			want: "/v1/responses/compact",
		},
		{
			name: "responses nested suffix preserved",
			path: "/openai/v1/responses/compact/detail",
			want: "/v1/responses/compact/detail",
		},
		{
			name: "non responses path uses platform fallback",
			path: "/v1/messages",
			want: EndpointResponses,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)

			got := GetUpstreamEndpoint(c, service.PlatformOpenAI)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetUpstreamEndpoint_PrefersActualOpenAIEndpointForAntigravityGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	// Client-facing Claude path under an antigravity group, but runtime
	// already recorded the real OpenAI Responses forward surface.
	c.Request = httptest.NewRequest(http.MethodPost, "/antigravity/v1/messages", nil)
	c.Set(ctxKeyInboundEndpoint, EndpointMessages)

	// Without runtime marker, antigravity derives /v1/messages.
	require.Equal(t, EndpointMessages, GetUpstreamEndpoint(c, service.PlatformAntigravity))

	service.SetActualOpenAIUpstreamEndpoint(c, EndpointResponses)
	// Ops must report the actual OpenAI upstream, not the antigravity inbound.
	require.Equal(t, EndpointResponses, GetUpstreamEndpoint(c, service.PlatformAntigravity))
	// Other platforms still honor the runtime marker when present.
	require.Equal(t, EndpointResponses, GetUpstreamEndpoint(c, service.PlatformOpenAI))
}

func TestResolveOpenAIUpstreamEndpoint_APIKeyChatOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name    string
		account *service.Account
		want    string
	}{
		{
			name: "apikey forced chat completions records raw chat endpoint",
			account: &service.Account{
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeAPIKey,
				Extra: map[string]any{
					openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
				},
			},
			want: "/v1/chat/completions",
		},
		{
			name: "apikey forced responses records responses endpoint",
			account: &service.Account{
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeAPIKey,
				Extra: map[string]any{
					openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceResponses),
				},
			},
			want: EndpointResponses,
		},
		{
			name: "oauth records responses endpoint",
			account: &service.Account{
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
			},
			want: EndpointResponses,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

			got := resolveOpenAIUpstreamEndpoint(c, tt.account, nil)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestResolveOpenAIUpstreamEndpoint_PassthroughFollowsInbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &service.Account{
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode:            string(openai_compat.ResponsesSupportModePassthrough),
			openai_compat.ExtraKeyResponsesSupported:       false,
			openai_compat.ExtraKeyChatCompletionsSupported: true,
		},
	}

	t.Run("inbound responses stays responses", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		require.Equal(t, EndpointResponses, resolveOpenAIUpstreamEndpoint(c, account, nil))
	})

	t.Run("inbound chat completions stays chat completions", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		require.Equal(t, EndpointChatCompletions, resolveOpenAIUpstreamEndpoint(c, account, nil))
	})
}
