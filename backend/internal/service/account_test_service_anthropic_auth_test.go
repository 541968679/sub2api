package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_AnthropicAPIKeyAuthScheme(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name              string
		extra             map[string]any
		wantAuthorization string
		wantXAPIKey       string
	}{
		{
			name:        "default remains x-api-key",
			wantXAPIKey: "upstream-key",
		},
		{
			name: "explicit bearer",
			extra: map[string]any{
				"anthropic_apikey_auth_scheme": AnthropicAPIKeyAuthSchemeAuthorizationBearer,
			},
			wantAuthorization: "Bearer upstream-key",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)

			upstream := &anthropicHTTPUpstreamRecorder{
				resp: &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"ok\"}}\n\n",
					)),
				},
			}
			svc := &AccountTestService{
				httpUpstream:        upstream,
				tlsFPProfileService: &TLSFingerprintProfileService{},
				cfg: &config.Config{
					Security: config.SecurityConfig{
						URLAllowlist: config.URLAllowlistConfig{Enabled: false},
					},
				},
			}
			account := &Account{
				ID:          1,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":  "upstream-key",
					"base_url": "https://compatible.example.com",
				},
				Extra: tt.extra,
			}

			err := svc.testClaudeAccountConnection(c, account, "claude-compatible")
			require.NoError(t, err)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, tt.wantAuthorization, getHeaderRaw(upstream.lastReq.Header, "authorization"))
			require.Equal(t, tt.wantXAPIKey, getHeaderRaw(upstream.lastReq.Header, "x-api-key"))
		})
	}
}
