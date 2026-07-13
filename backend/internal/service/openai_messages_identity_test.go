package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestForwardAsAnthropicOAuthRestoresCodexIdentityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const tuiUA = "codex-tui/9.9.9 (Mac OS X 14.0; arm64) iTerm (codex-tui; 9.9.9)"
	tests := []struct {
		name           string
		userAgent      string
		originator     string
		wantUserAgent  string
		wantOriginator string
	}{
		{
			name:           "official user agent is preserved and paired",
			userAgent:      tuiUA,
			originator:     "opencode",
			wantUserAgent:  tuiUA,
			wantOriginator: "codex-tui",
		},
		{
			name:           "third party user agent falls back to bundled Codex identity",
			userAgent:      "third-party-client/1.0.0",
			originator:     "opencode",
			wantUserAgent:  codexCLIUserAgent,
			wantOriginator: "codex_cli_rs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("User-Agent", tt.userAgent)
			c.Request.Header.Set("originator", tt.originator)

			upstream := &httpUpstreamRecorder{
				resp: openAIMessagesIdentitySSECompletedResponse("resp_identity", "gpt-5.4"),
			}
			svc := &OpenAIGatewayService{httpUpstream: upstream}
			account := &Account{
				ID:          1,
				Name:        "openai-oauth",
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Concurrency: 1,
				Credentials: map[string]any{
					"access_token":       "oauth-token",
					"chatgpt_account_id": "chatgpt-acc",
				},
			}

			result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.4")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, tt.wantUserAgent, upstream.lastReq.Header.Get("User-Agent"))
			require.Equal(t, tt.wantOriginator, upstream.lastReq.Header.Get("originator"))
			require.Equal(t, codexCLIVersion, upstream.lastReq.Header.Get("version"))
			require.Equal(t, "responses=experimental", upstream.lastReq.Header.Get("OpenAI-Beta"))
		})
	}
}

func openAIMessagesIdentitySSECompletedResponse(responseID, model string) *http.Response {
	body := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"` + responseID + `","object":"response","model":"` + model + `","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
