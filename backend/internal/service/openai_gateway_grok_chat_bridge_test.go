//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGrokChatResponsesBridgeEligibilityIsStrict(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		want   bool
		reason string
	}{
		{
			name: "plain text chat",
			body: `{"model":"grok","messages":[{"role":"system","content":"concise"},{"role":"user","content":"hi"}],"stream":false}`,
			want: true,
		},
		{
			name: "safe generation options",
			body: `{"model":"grok","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"include_usage":true},"max_completion_tokens":256,"temperature":0.2,"top_p":0.9,"prompt_cache_key":"session","tools":[],"functions":null,"tool_choice":"none"}`,
			want: true,
		},
		{
			name:   "stop stays raw",
			body:   `{"model":"grok","messages":[{"role":"user","content":"hi"}],"stop":"done"}`,
			reason: "unsupported_stop",
		},
		{
			name:   "developer role stays raw",
			body:   `{"model":"grok","messages":[{"role":"developer","content":"rules"},{"role":"user","content":"hi"}]}`,
			reason: "unsupported_message_role_developer",
		},
		{
			name:   "structured image stays raw",
			body:   `{"model":"grok","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,QQ=="}}]}]}`,
			reason: "non_text_message_content",
		},
		{
			name:   "function tool stays raw",
			body:   `{"model":"grok","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"lookup"}}]}`,
			reason: "unsupported_tools",
		},
		{
			name:   "small token cap stays raw",
			body:   `{"model":"grok","messages":[{"role":"user","content":"hi"}],"max_tokens":32}`,
			reason: "unsafe_max_tokens",
		},
		{
			name:   "unknown field stays raw",
			body:   `{"model":"grok","messages":[{"role":"user","content":"hi"}],"seed":7}`,
			reason: "unknown_field_seed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := grokChatResponsesBridgeEligibility([]byte(tt.body))
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.reason, reason)
		})
	}
}

func TestForwardGrokChatViaResponsesCachesAndReturnsChat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok","messages":[{"role":"system","content":"be concise"},{"role":"user","content":"hi"}],"stream":false,"prompt_cache_key":"stable-session","tools":[],"functions":null,"tool_choice":"none"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, grokChatRawEndpoint, bytes.NewReader(body))
	c.Set("api_key", &APIKey{ID: 7101, Group: &Group{Platform: PlatformGrok}})

	account := grokChatBridgeTestAccount(71)
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
		accountsByID: map[int64]*Account{account.ID: account},
	}}
	upstream := &httpUpstreamRecorder{resp: grokChatBridgeCompletedResponse("resp_grok_chat_cache", 9856)}
	svc := &OpenAIGatewayService{
		httpUpstream:      upstream,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		accountRepo:       repo,
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, xai.DefaultCLIBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, grokChatResponsesEndpoint, result.UpstreamEndpoint)
	require.Equal(t, "grok-4.5", result.UpstreamModel)
	require.Equal(t, 9856, result.Usage.CacheReadInputTokens)

	identity := gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String()
	require.NotEmpty(t, identity)
	require.NotEqual(t, "stable-session", identity)
	require.Equal(t, identity, upstream.lastReq.Header.Get(grokConversationIDHeader))
	require.Equal(t, "web_search", gjson.GetBytes(upstream.lastBody, "tools.0.type").String())
	require.Equal(t, "x_search", gjson.GetBytes(upstream.lastBody, "tools.1.type").String())
	require.Equal(t, grokFreeCacheDisabledToolChoice, gjson.GetBytes(upstream.lastBody, "tool_choice").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "include").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Exists())
	require.Equal(t, "cached ok", gjson.Get(recorder.Body.String(), "choices.0.message.content").String())
	require.Equal(t, int64(9856), gjson.Get(recorder.Body.String(), "usage.prompt_tokens_details.cached_tokens").Int())
}

func TestForwardGrokChatRuntimeGateFallsBackToRaw(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		setAPIKey    bool
		mappedModel  string
		wantUpstream string
	}{
		{
			name:         "unsafe stop shape",
			body:         `{"model":"grok","messages":[{"role":"user","content":"hi"}],"stop":"done"}`,
			setAPIKey:    true,
			wantUpstream: "grok-4.5",
		},
		{
			name:         "missing isolated identity",
			body:         `{"model":"grok","messages":[{"role":"user","content":"hi"}]}`,
			wantUpstream: "grok-4.5",
		},
		{
			name:         "non cache-capable mapped model",
			body:         `{"model":"grok","messages":[{"role":"user","content":"hi"}]}`,
			setAPIKey:    true,
			mappedModel:  "grok-4.3",
			wantUpstream: "grok-4.3",
		},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
			if tt.setAPIKey {
				c.Set("api_key", &APIKey{ID: int64(7301 + index), Group: &Group{Platform: PlatformGrok}})
			}

			account := grokChatBridgeTestAccount(int64(73 + index))
			if tt.mappedModel != "" {
				account.Credentials["model_mapping"] = map[string]any{"grok": tt.mappedModel}
			}
			repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{
				accountsByID: map[int64]*Account{account.ID: account},
			}}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"id":"chat_raw","object":"chat.completion","model":"` + tt.wantUpstream + `","choices":[{"index":0,"message":{"role":"assistant","content":"raw ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
				)),
			}}
			svc := &OpenAIGatewayService{
				httpUpstream:      upstream,
				grokTokenProvider: NewGrokTokenProvider(repo, nil),
				accountRepo:       repo,
			}

			result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(tt.body), "", "")
			require.NoError(t, err)
			require.Equal(t, xai.DefaultCLIBaseURL+"/chat/completions", upstream.lastReq.URL.String())
			require.Equal(t, grokChatRawEndpoint, result.UpstreamEndpoint)
			require.False(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").Exists())
		})
	}
}

func grokChatBridgeTestAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Name:        "grok-cache-bridge",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "access-token",
			"expires_at":   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"base_url":     xai.DefaultCLIBaseURL,
		},
	}
}

func grokChatBridgeCompletedResponse(responseID string, cachedTokens int) *http.Response {
	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","sequence_number":0,"delta":"cached ok"}`,
		"",
		`data: {"type":"response.completed","sequence_number":1,"response":{"id":"` + responseID + `","object":"response","model":"grok-4.5","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"cached ok"}]}],"usage":{"input_tokens":9908,"output_tokens":12,"total_tokens":9920,"input_tokens_details":{"cached_tokens":` + strconv.Itoa(cachedTokens) + `}}}}`,
		"",
	}, "\n")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                   []string{"text/event-stream"},
			"Xai-Request-Id":                 []string{responseID + "-request"},
			"X-Ratelimit-Limit-Requests":     []string{"10"},
			"X-Ratelimit-Remaining-Requests": []string{"9"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}
