//go:build unit

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

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func openAIChatPassthroughTestAccount() *Account {
	account := rawChatCompletionsTestAccount()
	account.Extra = map[string]any{
		"openai_passthrough":                     true,
		openai_compat.ExtraKeyResponsesMode:      string(openai_compat.ResponsesSupportModeForceResponses),
		openai_compat.ExtraKeyResponsesSupported: true,
	}
	account.Credentials["model_mapping"] = map[string]any{"kimi-k3": "mapped-away"}
	return account
}

func TestForwardAsChatCompletions_PassthroughKeepsChatCompletionsBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"kimi-k3","stream":true,"reasoning_effort":"medium","messages":[{"role":"user","content":""}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept-Language", "zh-CN")
	c.Request.Header.Set("X-Forwarded-For", "203.0.113.8")
	c.Request.Header.Set("Authorization", "Bearer client-secret")
	SetDisplayTokenMultipliers(c, &DisplayTokenMultipliers{InputMult: 2, OutputMult: 3})

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_cut","object":"chat.completion.chunk","model":"","choices":[{"index":0,"delta":{"content":"half an ans","tool_calls":[{"index":0,"id":"","type":"function","function":{"name":"","arguments":"{}"}}]}}]}`,
		"",
		`data: {"id":"chatcmpl_cut","object":"chat.completion.chunk","model":"","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":4}}`,
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_cc_pass"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		cfg:            rawChatCompletionsTestConfig(),
		httpUpstream:   upstream,
		settingService: kimiK3AdaptiveSettingsForTest(t, "true"),
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, openAIChatPassthroughTestAccount(), body, "", "gpt-4o-mini")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, "/v1/chat/completions", result.UpstreamEndpoint)
	require.Equal(t, "kimi-k3", result.BillingModel)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)

	require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "zh-CN", upstream.lastReq.Header.Get("Accept-Language"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Forwarded-For"))
	require.NotContains(t, upstream.lastReq.Header.Get("Authorization"), "client-secret")
	require.Equal(t, "kimi-k3", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "medium", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
	require.Equal(t, "", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Exists())

	written := rec.Body.String()
	require.Contains(t, written, `"content":"half an ans"`)
	require.Contains(t, written, `"id":""`)
	require.Contains(t, written, `"prompt_tokens":22`)
	require.Contains(t, written, `"completion_tokens":12`)
	require.NotContains(t, written, `"prompt_tokens":11`)
	require.NotContains(t, written, "event: error")
	require.NotContains(t, written, `"model":"kimi-k3"`)
	_, hasOps := c.Get(OpsUpstreamErrorsKey)
	require.False(t, hasOps)
	enabled, ok := c.Get("openai_passthrough")
	require.True(t, ok)
	require.Equal(t, true, enabled)
}

func TestForwardAsRawChatCompletions_PassthroughReturnsUpstreamErrorsAsIs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name        string
		status      int
		pool        bool
		failover    bool
		sameAccount bool
	}{
		{name: "400", status: http.StatusBadRequest, failover: false},
		{name: "401", status: http.StatusUnauthorized, failover: false},
		{name: "500", status: http.StatusInternalServerError, failover: false},
		{name: "429", status: http.StatusTooManyRequests, failover: true, sameAccount: false},
		{name: "429 pool", status: http.StatusTooManyRequests, pool: true, failover: true, sameAccount: true},
		{name: "529 pool", status: 529, pool: true, failover: true, sameAccount: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}]}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			upstreamBody := `{"error":{"message":"upstream said ` + tt.name + `","type":"upstream_type"}}`
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: tt.status,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_err"}},
				Body:       io.NopCloser(strings.NewReader(upstreamBody)),
			}}
			account := openAIChatPassthroughTestAccount()
			if tt.pool {
				account.Credentials["pool_mode"] = true
			}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

			_, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "gpt-4o-mini")
			require.Error(t, err)
			var failoverErr *UpstreamFailoverError
			if tt.failover {
				require.ErrorAs(t, err, &failoverErr)
				require.Equal(t, tt.status, failoverErr.StatusCode)
				require.Equal(t, tt.sameAccount, failoverErr.RetryableOnSameAccount)
				require.False(t, c.Writer.Written())
				return
			}
			require.False(t, errors.As(err, &failoverErr))
			require.Contains(t, err.Error(), "upstream error:")
			require.Equal(t, tt.status, rec.Code)
			require.Contains(t, rec.Body.String(), "upstream said "+tt.name)
			require.Equal(t, "upstream_type", gjson.Get(rec.Body.String(), "error.type").String())
		})
	}
}

func TestForwardAsRawChatCompletions_PassthroughJSONKeepsEmptyModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	SetDisplayTokenMultipliers(c, &DisplayTokenMultipliers{InputMult: 2, OutputMult: 3})
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_json","object":"chat.completion","model":"","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":7,"completion_tokens":2}}`,
		)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, openAIChatPassthroughTestAccount(), body, "gpt-4o-mini")
	require.NoError(t, err)
	require.False(t, result.Stream)
	require.Equal(t, 7, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
	require.Equal(t, "", gjson.Get(rec.Body.String(), "model").String())
	require.Equal(t, int64(14), gjson.Get(rec.Body.String(), "usage.prompt_tokens").Int())
	require.Equal(t, int64(6), gjson.Get(rec.Body.String(), "usage.completion_tokens").Int())
	require.Equal(t, "ok", gjson.Get(rec.Body.String(), "choices.0.message.content").String())
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestForwardAsRawChatCompletions_PassthroughKeepsZeroUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	SetDisplayTokenMultipliers(c, &DisplayTokenMultipliers{InputMult: 2, OutputMult: 3, CacheReadMult: 2, CacheCreateMult: 2})
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_zero","object":"chat.completion","model":"","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"prompt_tokens_details":{"cached_tokens":0},"cache_creation_input_tokens":0}}`,
		)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, openAIChatPassthroughTestAccount(), body, "gpt-4o-mini")
	require.NoError(t, err)
	require.Equal(t, 0, result.Usage.InputTokens)
	require.Equal(t, 0, result.Usage.OutputTokens)
	require.Equal(t, int64(0), gjson.Get(rec.Body.String(), "usage.prompt_tokens").Int())
	require.Equal(t, int64(0), gjson.Get(rec.Body.String(), "usage.completion_tokens").Int())
	require.Equal(t, int64(0), gjson.Get(rec.Body.String(), "usage.total_tokens").Int())
	require.Equal(t, int64(0), gjson.Get(rec.Body.String(), "usage.prompt_tokens_details.cached_tokens").Int())
	require.Equal(t, int64(0), gjson.Get(rec.Body.String(), "usage.cache_creation_input_tokens").Int())
	require.Equal(t, "ok", gjson.Get(rec.Body.String(), "choices.0.message.content").String())
}

func TestForwardAsRawChatCompletions_PassthroughKeepsZeroCompletion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"kimi-k3","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	SetDisplayTokenMultipliers(c, &DisplayTokenMultipliers{InputMult: 2, OutputMult: 3, CacheReadMult: 1, CacheCreateMult: 1, CacheReadOutputMult: 2})
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_zero_out","object":"chat.completion.chunk","model":"","choices":[{"index":0,"delta":{"content":"only prompt"}}]}`,
		"",
		`data: {"id":"chatcmpl_zero_out","object":"chat.completion.chunk","model":"","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":0,"total_tokens":10,"prompt_tokens_details":{"cached_tokens":4}}}`,
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, openAIChatPassthroughTestAccount(), body, "", "gpt-4o-mini")
	require.NoError(t, err)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 0, result.Usage.OutputTokens)
	require.Equal(t, 4, result.Usage.CacheReadInputTokens)

	written := rec.Body.String()
	require.Contains(t, written, `"content":"only prompt"`)
	usage := gjson.Get(passthroughUsagePayload(t, written), "usage")
	require.Equal(t, int64(0), usage.Get("completion_tokens").Int())
	require.Greater(t, usage.Get("prompt_tokens").Int(), int64(0))
	require.Equal(t, usage.Get("prompt_tokens").Int()+usage.Get("completion_tokens").Int(), usage.Get("total_tokens").Int())
	require.Equal(t, int64(4), usage.Get("prompt_tokens_details.cached_tokens").Int())
}

func TestRewriteOpenAIChatPassthroughDisplayUsage_ZeroStaysZero(t *testing.T) {
	inputPrice := 1.0
	cachePrice := 1.0
	allocMult := &DisplayTokenMultipliers{
		InputMult:                  2,
		OutputMult:                 1,
		CacheReadMult:              1,
		CacheCreateMult:            1,
		UseTokenAlloc:              true,
		RealInputPrice:             1,
		RealCacheReadPrice:         2,
		AllocDisplayInputPrice:     &inputPrice,
		AllocDisplayCacheReadPrice: &cachePrice,
		CacheTokenMaxMult:          1.3,
	}
	allocBody := []byte(`{"model":"","choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0,"prompt_tokens_details":{"cached_tokens":100}}}`)
	lifted := rewriteOpenAIChatUsageTokens(allocBody, "usage", allocMult)
	require.NotEqual(t, int64(0), gjson.GetBytes(lifted, "usage.prompt_tokens").Int())
	kept := rewriteOpenAIChatPassthroughDisplayUsage(allocBody, allocMult)
	require.Equal(t, int64(0), gjson.GetBytes(kept, "usage.prompt_tokens").Int())
	require.Equal(t, int64(0), gjson.GetBytes(kept, "usage.completion_tokens").Int())
	require.Equal(t, int64(0), gjson.GetBytes(kept, "usage.total_tokens").Int())
	require.Greater(t, gjson.GetBytes(kept, "usage.prompt_tokens_details.cached_tokens").Int(), int64(0))
	require.Equal(t, "ok", gjson.GetBytes(kept, "choices.0.message.content").String())

	linearMult := &DisplayTokenMultipliers{
		InputMult:           2,
		OutputMult:          3,
		CacheReadMult:       1,
		CacheCreateMult:     2,
		CacheReadOutputMult: 2,
	}
	linearBody := []byte(`{"usage":{"prompt_tokens":8,"completion_tokens":0,"total_tokens":8,"cache_creation_input_tokens":0,"prompt_tokens_details":{"cached_tokens":5,"cache_write_tokens":3}}}`)
	linearLifted := rewriteOpenAIChatUsageTokens(linearBody, "usage", linearMult)
	require.NotEqual(t, int64(0), gjson.GetBytes(linearLifted, "usage.completion_tokens").Int())
	require.NotEqual(t, int64(0), gjson.GetBytes(linearLifted, "usage.cache_creation_input_tokens").Int())
	linearKept := rewriteOpenAIChatPassthroughDisplayUsage(linearBody, linearMult)
	require.Equal(t, int64(0), gjson.GetBytes(linearKept, "usage.completion_tokens").Int())
	require.Equal(t, int64(0), gjson.GetBytes(linearKept, "usage.cache_creation_input_tokens").Int())
	require.Equal(t, int64(6), gjson.GetBytes(linearKept, "usage.prompt_tokens_details.cache_write_tokens").Int())
	require.Equal(t, gjson.GetBytes(linearKept, "usage.prompt_tokens").Int(), gjson.GetBytes(linearKept, "usage.total_tokens").Int())
	require.Greater(t, gjson.GetBytes(linearKept, "usage.prompt_tokens").Int(), int64(0))

	unchanged := []byte(`{"usage":{"prompt_tokens":3,"completion_tokens":1}}`)
	require.Equal(t, string(unchanged), string(rewriteOpenAIChatPassthroughDisplayUsage(unchanged, nil)))
	require.Equal(t, `{"choices":[{"message":{"content":"no usage"}}]}`, string(rewriteOpenAIChatPassthroughDisplayUsage([]byte(`{"choices":[{"message":{"content":"no usage"}}]}`), linearMult)))
}

func passthroughUsagePayload(t *testing.T, body string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		payload, ok := extractOpenAISSEDataLine(line)
		if ok && gjson.Get(payload, "usage").Exists() {
			return payload
		}
	}
	t.Fatal("usage payload not found")
	return ""
}

func TestForwardAsRawChatCompletions_PassthroughRequiresModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	upstream := &httpUpstreamRecorder{err: io.ErrClosedPipe}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	_, err := svc.forwardAsRawChatCompletions(context.Background(), c, openAIChatPassthroughTestAccount(), body, "")
	require.Error(t, err)
	require.Empty(t, upstream.requests)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "model is required", gjson.Get(rec.Body.String(), "error.message").String())
}
