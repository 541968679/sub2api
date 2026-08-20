package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeResponsesRequestServiceTier(t *testing.T) {
	t.Parallel()

	req := &apicompat.ResponsesRequest{ServiceTier: " fast "}
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "priority", req.ServiceTier)

	req.ServiceTier = "flex"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "flex", req.ServiceTier)

	// OpenAI 官方合法 tier 应被透传保留。
	req.ServiceTier = "auto"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "auto", req.ServiceTier)

	req.ServiceTier = "default"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "default", req.ServiceTier)

	req.ServiceTier = "scale"
	normalizeResponsesRequestServiceTier(req)
	require.Equal(t, "scale", req.ServiceTier)

	// 真未知值仍被剥离。
	req.ServiceTier = "turbo"
	normalizeResponsesRequestServiceTier(req)
	require.Empty(t, req.ServiceTier)
}

func TestNormalizeResponsesBodyServiceTier(t *testing.T) {
	t.Parallel()

	body, tier, err := normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"fast"}`))
	require.NoError(t, err)
	require.Equal(t, "priority", tier)
	require.Equal(t, "priority", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"flex"}`))
	require.NoError(t, err)
	require.Equal(t, "flex", tier)
	require.Equal(t, "flex", gjson.GetBytes(body, "service_tier").String())

	// OpenAI 官方 tier 直接保留在 body 中（透传上游）。
	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"auto"}`))
	require.NoError(t, err)
	require.Equal(t, "auto", tier)
	require.Equal(t, "auto", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"default"}`))
	require.NoError(t, err)
	require.Equal(t, "default", tier)
	require.Equal(t, "default", gjson.GetBytes(body, "service_tier").String())

	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"scale"}`))
	require.NoError(t, err)
	require.Equal(t, "scale", tier)
	require.Equal(t, "scale", gjson.GetBytes(body, "service_tier").String())

	// 真未知值才会被删除。
	body, tier, err = normalizeResponsesBodyServiceTier([]byte(`{"model":"gpt-5.1","service_tier":"turbo"}`))
	require.NoError(t, err)
	require.Empty(t, tier)
	require.False(t, gjson.GetBytes(body, "service_tier").Exists())
}

func TestForwardAsChatCompletions_APIKeyPropagatesPromptCacheKeyInResponsesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("api_key", &APIKey{ID: 99})

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_prompt_cache"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"type":"invalid_request_error","message":"stop before response parsing"}}`))),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          2,
		Name:        "openai-compatible",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-compatible",
		},
		Extra: map[string]any{
			"openai_responses_supported": true,
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "cache-key-123", "gpt-5.4")
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, "cache-key-123", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-compatible", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, generateSessionUUID(isolateOpenAISessionID(99, "cache-key-123")), upstream.lastReq.Header.Get("session_id"))
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Accept"))
}

func TestForwardAsChatCompletions_OAuthDoesNotInjectDefaultInstructions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_chat_oauth_no_default_instructions"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"type":"invalid_request_error","message":"stop before response parsing"}}`))),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          12,
		Name:        "oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, "", gjson.GetBytes(upstream.lastBody, "instructions").String())
	require.NotContains(t, string(upstream.lastBody), "Communicate with the user by streaming thinking")
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
}

func chatCompletionsSpeedAPIKeyAccount(extra map[string]any) *Account {
	return &Account{
		ID:          2,
		Name:        "openai-compatible",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-compatible",
		},
		Extra: extra,
	}
}

func chatCompletionsSpeedStopRecorder() *httpUpstreamRecorder {
	return &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_stop"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"type":"invalid_request_error","message":"stop before response parsing"}}`))),
	}}
}

func TestForwardAsChatCompletions_APIKeyAutoInjectsCompatPromptCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	key := gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String()
	require.True(t, strings.HasPrefix(key, "compat_cc_"), key)
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
}

func TestForwardAsChatCompletions_APIKeyDoesNotOverwriteClientSessionHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("session_id", "client-session-header")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "client-session-header", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
}

func TestForwardAsChatCompletions_UnknownProbeStillConvertsToResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(nil)

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
}

func TestForwardAsChatCompletions_ReasoningEffortDoesNotAddSummaryAuto(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false,"reasoning_effort":"high"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "reasoning.summary").Exists())
}

func TestForwardAsChatCompletions_APIKeyDoesNotOverwriteClientPromptCacheKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false,"prompt_cache_key":"client-cache-key"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "client-cache-key", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
}

func TestForwardAsChatCompletions_ForceChatCompletionsUsesRawPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
		openai_compat.ExtraKeyResponsesMode:      string(openai_compat.ResponsesSupportModeForceChatCompletions),
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "https://api.openai.com/v1/chat/completions", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Accept"))
}

func TestForwardAsChatCompletions_PassthroughUsesRawChatEvenWhenResponsesSupported(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported:       true,
		openai_compat.ExtraKeyChatCompletionsSupported: true,
		openai_compat.ExtraKeyResponsesMode:            string(openai_compat.ResponsesSupportModePassthrough),
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "https://api.openai.com/v1/chat/completions", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Accept"))
}

func TestForwardAsChatCompletions_AutoUnsupportedUsesRawChat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported:       false,
		openai_compat.ExtraKeyChatCompletionsSupported: true,
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "https://api.openai.com/v1/chat/completions", upstream.lastReq.URL.String())
}

func TestForwardAsChatCompletions_AutoBothSupportedStillConvertsToResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported:       true,
		openai_compat.ExtraKeyChatCompletionsSupported: true,
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
}

func TestForwardAsChatCompletions_AutoSupportedStillConvertsToResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
		openai_compat.ExtraKeyResponsesMode:      string(openai_compat.ResponsesSupportModeAuto),
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
}

func TestForwardAsChatCompletions_APIKeyStreamKeepsUpstreamSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
	})

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "", "gpt-5.4")
	require.Error(t, err)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
}

func TestForwardAsChatCompletions_APIKeySyncReadsNonStreamJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamJSON := `{"id":"resp_json","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"pong"}]}],"usage":{"input_tokens":8,"output_tokens":1,"total_tokens":9}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-json"}},
		Body:       io.NopCloser(strings.NewReader(upstreamJSON)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
	})

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, "cache-key-123", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 8, result.Usage.InputTokens)
	require.Equal(t, 1, result.Usage.OutputTokens)
	require.Equal(t, "resp_json", result.ResponseID)
	require.Contains(t, rec.Body.String(), "pong")
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Accept"))
}

func TestHandleChatNonStreamResponsesJSON_RespectsUpstreamReadLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{UpstreamResponseReadMaxBytes: 64},
	}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	raw := `{"id":"resp_json","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"` + strings.Repeat("x", 200) + `"}]}]}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(raw)),
	}

	result, err := svc.handleChatNonStreamResponsesJSON(resp, c, "gpt-4o", "gpt-4o", "gpt-4o", time.Now())
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrUpstreamResponseBodyTooLarge)
	require.Contains(t, rec.Body.String(), "too large")
}

func TestHandleChatNonStreamResponsesJSON_UsesSharedDisplayRewrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	SetDisplayTokenMultipliers(c, openAITestDisplayMultipliers())

	raw := `{"id":"resp_json","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":1000,"output_tokens":100,"total_tokens":1100,"input_tokens_details":{"cached_tokens":200}}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-json"}},
		Body:       io.NopCloser(strings.NewReader(raw)),
	}

	result, err := svc.handleChatNonStreamResponsesJSON(resp, c, "gpt-4o", "gpt-4o", "gpt-4o", time.Now())
	require.NoError(t, err)
	require.Equal(t, 1000, result.Usage.InputTokens)
	require.Equal(t, 100, result.Usage.OutputTokens)
	require.Equal(t, 200, result.Usage.CacheReadInputTokens)
	require.Equal(t, int64(2200), gjson.Get(rec.Body.String(), "usage.prompt_tokens").Int())
	require.Equal(t, int64(400), gjson.Get(rec.Body.String(), "usage.completion_tokens").Int())
}

func TestHandleChatBufferedStreamingResponse_HTTP2PeerResetFailovers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid-h2"}},
		Body:       io.NopCloser(&errReader{err: errors.New("http2: stream error: INTERNAL_ERROR; received from peer")}),
	}

	result, err := svc.handleChatBufferedStreamingResponse(resp, c, "gpt-4o", "gpt-4o", "gpt-4o", time.Now(), &Account{ID: 1606, Platform: PlatformOpenAI})
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Empty(t, rec.Body.String())
}

func TestHandleChatBufferedStreamingResponse_IntervalTimeoutFailovers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 1},
	}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	body := &hangReadCloser{hang: make(chan struct{})}
	t.Cleanup(func() { _ = body.Close() })
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid-timeout"}},
		Body:       body,
	}

	started := time.Now()
	result, err := svc.handleChatBufferedStreamingResponse(resp, c, "gpt-4o", "gpt-4o", "gpt-4o", time.Now(), &Account{ID: 7, Platform: PlatformOpenAI})
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Empty(t, rec.Body.String())
	require.Less(t, time.Since(started), 3*time.Second)
}

func TestHandleChatBufferedStreamingResponse_TimeoutAfterCompletedReturnsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 1},
	}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	body := &completedThenHangCloser{hang: make(chan struct{})}
	t.Cleanup(func() { _ = body.Close() })
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid-completed-hang"}},
		Body:       body,
	}

	started := time.Now()
	result, err := svc.handleChatBufferedStreamingResponse(resp, c, "gpt-4o", "gpt-4o", "gpt-4o", time.Now(), &Account{ID: 8, Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.Equal(t, "resp_done", result.ResponseID)
	require.Equal(t, 2, result.Usage.InputTokens)
	require.NotContains(t, rec.Body.String(), `"error"`)
	require.Less(t, time.Since(started), 3*time.Second)
}

func TestHandleChatBufferedStreamingResponse_PacedSSESurvivesIntervalTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 1},
	}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid-paced"}},
		Body: io.NopCloser(&pacedReader{
			chunks: []string{
				"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_chat\"}}\n",
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_chat\",\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1,\"total_tokens\":2}}}\n",
				"data: [DONE]\n",
			},
			delay: 200 * time.Millisecond,
		}),
	}

	result, err := svc.handleChatBufferedStreamingResponse(resp, c, "gpt-4o", "gpt-4o", "gpt-4o", time.Now())
	require.NoError(t, err)
	require.Equal(t, "resp_chat", result.ResponseID)
	require.Equal(t, 1, result.Usage.InputTokens)
}

func TestHandleChatBufferedStreamingResponse_UsesContextRequestIDWhenHeaderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.RequestID, "ctx-req-1"))
	c.Request = req

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.completed","response":{"id":"resp_chat","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`data: [DONE]`,
		}, "\n"))),
	}

	result, err := svc.handleChatBufferedStreamingResponse(resp, c, "gpt-4o", "gpt-4o", "gpt-4o", time.Now())
	require.NoError(t, err)
	require.Equal(t, "ctx-req-1", result.RequestID)
}

func TestIsOpenAIHTTP2PeerReset(t *testing.T) {
	require.True(t, isOpenAIHTTP2PeerReset(errors.New("stream error: stream ID 1; INTERNAL_ERROR; received from peer")))
	require.True(t, isOpenAIHTTP2PeerReset(errors.New("http2: stream closed")))
	require.False(t, isOpenAIHTTP2PeerReset(context.Canceled))
	require.False(t, isOpenAIHTTP2PeerReset(errors.New("connection reset by peer")))
}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }

type hangReadCloser struct {
	hang chan struct{}
}

func (r *hangReadCloser) Read([]byte) (int, error) {
	<-r.hang
	return 0, io.EOF
}

func (r *hangReadCloser) Close() error {
	select {
	case <-r.hang:
	default:
		close(r.hang)
	}
	return nil
}

type rawChatUsableThenHangCloser struct {
	sent bool
	hang chan struct{}
}

func (r *rawChatUsableThenHangCloser) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, `data: {"id":"chatcmpl_hang","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"pong"}}]}`+"\n"), nil
	}
	<-r.hang
	return 0, io.EOF
}

func (r *rawChatUsableThenHangCloser) Close() error {
	select {
	case <-r.hang:
	default:
		close(r.hang)
	}
	return nil
}

type completedThenHangCloser struct {
	sent bool
	hang chan struct{}
}

func (r *completedThenHangCloser) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, `data: {"type":"response.completed","response":{"id":"resp_done","status":"completed","usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3}}}`+"\n"), nil
	}
	<-r.hang
	return 0, io.EOF
}

func (r *completedThenHangCloser) Close() error {
	select {
	case <-r.hang:
	default:
		close(r.hang)
	}
	return nil
}

type pacedReader struct {
	chunks []string
	i      int
	delay  time.Duration
}

func (r *pacedReader) Read(p []byte) (int, error) {
	if r.i >= len(r.chunks) {
		return 0, io.EOF
	}
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	n := copy(p, r.chunks[r.i])
	r.i++
	return n, nil
}

func chatCompletionsCustomBaseAPIKeyAccount() *Account {
	account := chatCompletionsSpeedAPIKeyAccount(map[string]any{
		openai_compat.ExtraKeyResponsesSupported: true,
	})
	account.Credentials["base_url"] = "https://token-bits.example/v1"
	return account
}

func chatCompletionsCustomBaseConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}
}

func TestForwardAsChatCompletions_CustomBaseSyncUsesUpstreamSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamSSE := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_sse","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"pong"}]}],"usage":{"input_tokens":8,"output_tokens":1,"total_tokens":9}}}`,
		`data: [DONE]`,
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-sse"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}}
	svc := &OpenAIGatewayService{cfg: chatCompletionsCustomBaseConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, chatCompletionsCustomBaseAPIKeyAccount(), body, "", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, 8, result.Usage.InputTokens)
	require.Equal(t, 1, result.Usage.OutputTokens)
	require.Contains(t, rec.Body.String(), "pong")
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestForwardAsChatCompletions_CustomBaseStillAcceptsUpstreamJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamJSON := `{"id":"resp_json","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"pong"}]}],"usage":{"input_tokens":8,"output_tokens":1,"total_tokens":9}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-json"}},
		Body:       io.NopCloser(strings.NewReader(upstreamJSON)),
	}}
	svc := &OpenAIGatewayService{cfg: chatCompletionsCustomBaseConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, chatCompletionsCustomBaseAPIKeyAccount(), body, "", "gpt-5.4")
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, "resp_json", result.ResponseID)
	require.Contains(t, rec.Body.String(), "pong")
}

func TestForwardAsChatCompletions_ModeOffKeepsS2OnCustomBase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	cfg := chatCompletionsCustomBaseConfig()
	cfg.Gateway.OpenAISyncInboundUpstreamSSEMode = "off"
	upstream := chatCompletionsSpeedStopRecorder()
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}

	_, err := svc.ForwardAsChatCompletions(context.Background(), c, chatCompletionsCustomBaseAPIKeyAccount(), body, "", "gpt-5.4")
	require.Error(t, err)
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Accept"))
}

func TestForwardAsRawChatCompletions_CustomBaseSyncBuffersSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{"content":"pong"}}]}`,
		"",
		`data: {"id":"chatcmpl_1","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":1,"total_tokens":9}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_raw_sse"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Equal(t, 8, result.Usage.InputTokens)
	require.Equal(t, 1, result.Usage.OutputTokens)
	require.Contains(t, rec.Body.String(), "pong")
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
}

func TestBufferRawChatCompletionsFromSSE_AssemblesToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"run","arguments":"{\"x\":"}}]}}]}`,
		"",
		`data: {"id":"chatcmpl_tool","object":"chat.completion.chunk","model":"gpt-5.4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]},"finish_reason":"tool_calls"}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-raw-tools"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}

	result, err := svc.bufferRawChatCompletionsFromSSE(c, resp, &Account{ID: 9, Platform: PlatformOpenAI}, "gpt-5.4", "gpt-5.4", "gpt-5.4", nil, nil, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "tool_calls", gjson.Get(rec.Body.String(), "choices.0.finish_reason").String())
	require.Equal(t, "call_1", gjson.Get(rec.Body.String(), "choices.0.message.tool_calls.0.id").String())
	require.Equal(t, "run", gjson.Get(rec.Body.String(), "choices.0.message.tool_calls.0.function.name").String())
	require.JSONEq(t, `{"x":1}`, gjson.Get(rec.Body.String(), "choices.0.message.tool_calls.0.function.arguments").String())
	require.True(t, gjson.Get(rec.Body.String(), "choices.0.message.content").Type == gjson.Null)
}

func TestBufferRawChatCompletionsFromSSE_TimeoutAfterUsableReturnsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 1},
	}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	body := &rawChatUsableThenHangCloser{hang: make(chan struct{})}
	t.Cleanup(func() { _ = body.Close() })
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid-raw-usable-hang"}},
		Body:       body,
	}

	started := time.Now()
	result, err := svc.bufferRawChatCompletionsFromSSE(c, resp, &Account{ID: 11, Platform: PlatformOpenAI}, "gpt-4o", "gpt-4o", "gpt-4o", nil, nil, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "pong")
	require.NotContains(t, rec.Body.String(), `"error"`)
	require.Less(t, time.Since(started), 3*time.Second)
}

func TestBufferRawChatCompletionsFromSSE_HTTP2PeerResetFailovers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid-raw-h2"}},
		Body:       io.NopCloser(&errReader{err: errors.New("http2: stream error: INTERNAL_ERROR; received from peer")}),
	}
	result, err := svc.bufferRawChatCompletionsFromSSE(c, resp, &Account{ID: 1606, Platform: PlatformOpenAI}, "gpt-4o", "gpt-4o", "gpt-4o", nil, nil, time.Now())
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Empty(t, rec.Body.String())
}

func TestBufferRawChatCompletionsFromSSE_IntervalTimeoutFailovers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 1},
	}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	body := &hangReadCloser{hang: make(chan struct{})}
	t.Cleanup(func() { _ = body.Close() })
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"x-request-id": []string{"rid-raw-timeout"}},
		Body:       body,
	}
	started := time.Now()
	result, err := svc.bufferRawChatCompletionsFromSSE(c, resp, &Account{ID: 7, Platform: PlatformOpenAI}, "gpt-4o", "gpt-4o", "gpt-4o", nil, nil, time.Now())
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Empty(t, rec.Body.String())
	require.Less(t, time.Since(started), 3*time.Second)
}
