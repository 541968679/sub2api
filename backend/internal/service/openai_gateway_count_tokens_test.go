//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newCountTokensTestContext(t *testing.T, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "Claude-Code/1.0")
	return c, rec
}

func newCountTokensUpstream(status int, body string) *httpUpstreamSequenceRecorder {
	return &httpUpstreamSequenceRecorder{
		responses: []*http.Response{{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}},
	}
}

const countTokensAnthropicBody = `{"model":"claude-opus-4-8","messages":[{"role":"user","content":"hello"}]}`

func TestForwardCountTokensAsAnthropic_APIKeyAccountUsesBaseURL(t *testing.T) {
	upstream := newCountTokensUpstream(http.StatusOK, `{"input_tokens":42}`)
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:          101,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://upstream.example",
		},
	}
	c, rec := newCountTokensTestContext(t, []byte(countTokensAnthropicBody))

	err := svc.ForwardCountTokensAsAnthropic(context.Background(), c, account, []byte(countTokensAnthropicBody), "gpt-5.5")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"input_tokens":42}`, rec.Body.String())
	require.Len(t, upstream.reqs, 1)
	require.Equal(t, "https://upstream.example/v1/responses/input_tokens", upstream.reqs[0].URL.String())
	require.Equal(t, "Bearer sk-test", upstream.reqs[0].Header.Get("authorization"))
	require.Equal(t, "gpt-5.5", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.True(t, gjson.GetBytes(upstream.bodies[0], "input").Exists(),
		"anthropic messages must be converted to responses input")
	require.False(t, gjson.GetBytes(upstream.bodies[0], "messages").Exists(),
		"upstream body must not contain the anthropic messages field")
}

func TestForwardCountTokensAsAnthropic_OAuthFallsBackWhenUnsupported(t *testing.T) {
	account := &Account{
		ID:          202,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "oauth-token",
		},
	}
	prepared, err := prepareOpenAIInputTokensCountRequest([]byte(countTokensAnthropicBody), account, "gpt-5.5")
	require.NoError(t, err)
	expectedEstimate, err := estimateOpenAIInputTokens(prepared.Request)
	require.NoError(t, err)
	if expectedEstimate <= 0 {
		expectedEstimate = openAIInputTokensFallbackMinimum
	}

	cases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "401 missing responses write scope",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"type":"invalid_request_error","code":"missing_scope","message":"Missing scopes: api.responses.write."}}`,
		},
		{
			name:       "403 insufficient scope",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"type":"invalid_request_error","code":"missing_scope","message":"Missing scopes: api.responses.write"}}`,
		},
		{
			name:       "404 input_tokens endpoint not found",
			statusCode: http.StatusNotFound,
			body:       `{"error":{"type":"invalid_request_error","message":"The /v1/responses/input_tokens endpoint was not found"}}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			upstream := newCountTokensUpstream(tt.statusCode, tt.body)
			repo := &claudeGPTBridgeRouteRepoStub{}
			svc := &OpenAIGatewayService{
				cfg:              &config.Config{},
				httpUpstream:     upstream,
				rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil),
			}
			c, rec := newCountTokensTestContext(t, []byte(countTokensAnthropicBody))

			err := svc.ForwardCountTokensAsAnthropic(context.Background(), c, account, []byte(countTokensAnthropicBody), "gpt-5.5")

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rec.Code)
			require.JSONEq(t, `{"input_tokens":`+strconv.Itoa(expectedEstimate)+`}`, rec.Body.String())
			require.Len(t, upstream.reqs, 1)
			require.Equal(t, openaiPlatformAPIInputTokensURL, upstream.reqs[0].URL.String())
			require.Equal(t, "Bearer oauth-token", upstream.reqs[0].Header.Get("authorization"))
			require.Zero(t, repo.setRateLimitedCalls,
				"OAuth unsupported endpoint must not rate-limit the account")
		})
	}
}

func TestForwardCountTokensAsAnthropic_UpstreamErrorSurfacesForOpenAIGroups(t *testing.T) {
	upstream := newCountTokensUpstream(http.StatusTooManyRequests, `{"error":{"type":"rate_limit_error","message":"slow down"}}`)
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{
		ID:          303,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test"},
	}
	c, rec := newCountTokensTestContext(t, []byte(countTokensAnthropicBody))

	err := svc.ForwardCountTokensAsAnthropic(context.Background(), c, account, []byte(countTokensAnthropicBody), "gpt-5.5")

	require.Error(t, err)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
}

// bridge 宽松模式：count 属于辅助能力，上游 429 仍写入账号限流状态，
// 但客户端得到 200 本地估算而不是错误。
func TestForwardCountTokensAsAnthropicClaudeGPTBridge_UpstreamErrorFallsBackToEstimate(t *testing.T) {
	resetsAt := time.Now().Add(2 * time.Minute)
	upstream := newCountTokensUpstream(http.StatusTooManyRequests,
		`{"error":{"type":"usage_limit_reached","message":"limit","resets_at":`+strconv.FormatInt(resetsAt.Unix(), 10)+`}}`)
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{newClaudeGPTBridgeRouteAccount(1)}}
	svc := &OpenAIGatewayService{
		cfg:              &config.Config{},
		httpUpstream:     upstream,
		accountRepo:      repo,
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil),
	}
	account := repo.accounts[0]
	account.Credentials["api_key"] = "sk-test"
	c, rec := newCountTokensTestContext(t, []byte(countTokensAnthropicBody))

	err := svc.ForwardCountTokensAsAnthropicClaudeGPTBridge(context.Background(), c, &account, []byte(countTokensAnthropicBody), "gpt-5.5")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.GreaterOrEqual(t, int(gjson.Get(rec.Body.String(), "input_tokens").Int()), 1)
	require.Equal(t, 1, repo.setRateLimitedCalls,
		"bridge count 429 must still record the account rate limit for messages routing")
}

func TestEstimateCountTokensClaudeGPTBridge_WritesLocalEstimate(t *testing.T) {
	svc := &OpenAIGatewayService{}
	c, rec := newCountTokensTestContext(t, []byte(countTokensAnthropicBody))

	err := svc.EstimateCountTokensClaudeGPTBridge(c, []byte(countTokensAnthropicBody), "gpt-5.5")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.GreaterOrEqual(t, int(gjson.Get(rec.Body.String(), "input_tokens").Int()), 1)
}

func TestResolveClaudeGPTBridgeCountUpstreamModel_UsesBlockedCandidates(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Minute)
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{newClaudeGPTBridgeRouteAccount(1, func(a *Account) {
		a.RateLimitResetAt = &resetAt
	})}}
	svc := &OpenAIGatewayService{accountRepo: repo}
	groupID := int64(7)

	mapped, ok := svc.ResolveClaudeGPTBridgeCountUpstreamModel(context.Background(), &groupID, "claude-opus-4-8")

	require.True(t, ok, "a rate-limited candidate must still resolve the mapping for local estimation")
	require.Equal(t, "gpt-5.5", mapped)
}

func TestEstimateOpenAIInputTokens_RequestSamples(t *testing.T) {
	cases := []struct {
		name string
		req  openAIInputTokensCountRequest
		want int
	}{
		{
			name: "simple text input",
			req: openAIInputTokensCountRequest{
				Model: "gpt-5",
				Input: json.RawMessage(`[{"role":"user","content":"hello world"}]`),
			},
			want: 6,
		},
		{
			name: "instructions plus tool schema",
			req: openAIInputTokensCountRequest{
				Model:        "gpt-5",
				Instructions: "You are helpful.",
				Input:        json.RawMessage(`[{"role":"user","content":"lookup weather in shanghai"}]`),
				Tools: []apicompat.ResponsesTool{
					{
						Type:        "function",
						Name:        "lookup_weather",
						Description: "Look up current weather",
						Parameters:  json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
					},
				},
			},
			want: 50,
		},
		{
			name: "input parts and tool output",
			req: openAIInputTokensCountRequest{
				Model: "gpt-4.1",
				Input: json.RawMessage(`[
					{"role":"user","content":[{"type":"input_text","text":"first line"},{"type":"input_text","text":"second line"}]},
					{"type":"function_call_output","call_id":"call_123","output":"{\"ok\":true}"}
				]`),
			},
			want: 24,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := estimateOpenAIInputTokens(tt.req)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBuildOpenAIResponsesInputTokensURL(t *testing.T) {
	cases := []struct {
		base string
		want string
	}{
		{base: "https://upstream.example", want: "https://upstream.example/v1/responses/input_tokens"},
		{base: "https://upstream.example/v1", want: "https://upstream.example/v1/responses/input_tokens"},
		{base: "https://upstream.example/v1/responses/input_tokens", want: "https://upstream.example/v1/responses/input_tokens"},
	}
	for _, tt := range cases {
		require.Equal(t, tt.want, buildOpenAIResponsesInputTokensURL(tt.base))
	}
}
