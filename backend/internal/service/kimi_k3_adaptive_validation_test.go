//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestKimiK3AdaptiveModelPredicate(t *testing.T) {
	require.True(t, isKimiK3UpstreamModel("kimi-k3"))
	require.True(t, isKimiK3UpstreamModel(" Kimi-K3 "))
	require.True(t, isKimiK3UpstreamModel("vendor/kimi-k3"))
	require.False(t, isKimiK3UpstreamModel("kimi-k2"))
	require.False(t, isKimiK3UpstreamModel("kimi"))
	require.False(t, isKimiK3UpstreamModel("glm-4.6"))
	require.False(t, isKimiK3UpstreamModel("a/b/kimi-k3"))
	require.False(t, isKimiK3UpstreamModel(""))
}

func TestKimiK3AdaptiveChatBody(t *testing.T) {
	name128 := strings.Repeat("a", 128)
	name129 := strings.Repeat("a", 129)
	stop32 := strings.Repeat("s", 32)
	stop33 := strings.Repeat("s", 33)

	cases := []struct {
		name    string
		body    string
		reject  string
		changed bool
	}{
		{name: "absent fields pass", body: `{"model":"kimi-k3","messages":[{"role":"user","content":"hi"}]}`},
		{name: "max tokens field ignored", body: `{"model":"kimi-k3","max_tokens":999999999,"messages":[{"role":"user","content":"hi"}]}`},
		{name: "budget below window", body: `{"model":"kimi-k3","max_completion_tokens":1048575,"messages":[{"role":"user","content":"hi"}]}`},
		{name: "full budget empty messages", body: `{"model":"kimi-k3","max_completion_tokens":1048576,"messages":[]}`},
		{name: "full budget without messages", body: `{"model":"kimi-k3","max_completion_tokens":1048576}`},
		{
			name:   "full budget with a message",
			body:   `{"model":"kimi-k3","max_completion_tokens":1048576,"messages":[{"role":"user","content":"hi"}]}`,
			reject: "max_completion_tokens 1048576 plus a non-empty prompt exceeds the 1048576 context window",
		},
		{
			name:   "over maximum",
			body:   `{"model":"kimi-k3","max_completion_tokens":1048577}`,
			reject: "max_completion_tokens exceeds kimi-k3 maximum 1048576",
		},
		{name: "prediction content", body: `{"model":"kimi-k3","prediction":{"type":"content","content":"ok"}}`},
		{
			name:   "prediction other type",
			body:   `{"model":"kimi-k3","prediction":{"type":"other"}}`,
			reject: "prediction.type must be content",
		},
		{name: "reasoning low high max", body: `{"model":"kimi-k3","reasoning_effort":"high"}`},
		{
			name:   "reasoning medium stays rejected",
			body:   `{"model":"kimi-k3","reasoning_effort":"medium","messages":[{"role":"user","content":"hi"}]}`,
			reject: "reasoning_effort must be low, high, or max",
		},
		{name: "stop 32 bytes", body: `{"model":"kimi-k3","stop":"` + stop32 + `"}`},
		{
			name:   "stop 33 bytes",
			body:   `{"model":"kimi-k3","stop":"` + stop33 + `"}`,
			reject: "stop string exceeds 32 bytes",
		},
		{name: "five stops", body: `{"model":"kimi-k3","stop":["a","b","c","d","e"]}`},
		{
			name:   "six stops",
			body:   `{"model":"kimi-k3","stop":["a","b","c","d","e","f"]}`,
			reject: "stop supports at most 5 strings",
		},
		{
			name:   "stop element too long",
			body:   `{"model":"kimi-k3","stop":["` + stop33 + `"]}`,
			reject: "stop string exceeds 32 bytes",
		},
		{
			name:   "stop not a string",
			body:   `{"model":"kimi-k3","stop":1}`,
			reject: "stop must be a string or an array of strings",
		},
		{
			name:   "stop element not a string",
			body:   `{"model":"kimi-k3","stop":["ok",1]}`,
			reject: "stop must be a string or an array of strings",
		},
		{name: "top logprobs 1", body: `{"model":"kimi-k3","logprobs":true,"top_logprobs":1}`},
		{name: "top logprobs bounds", body: `{"model":"kimi-k3","logprobs":true,"top_logprobs":0}`},
		{name: "top logprobs 20", body: `{"model":"kimi-k3","logprobs":true,"top_logprobs":20}`},
		{
			name:   "top logprobs 21",
			body:   `{"model":"kimi-k3","logprobs":true,"top_logprobs":21}`,
			reject: "top_logprobs must be an integer from 0 to 20",
		},
		{
			name:   "top logprobs negative",
			body:   `{"model":"kimi-k3","logprobs":true,"top_logprobs":-1}`,
			reject: "top_logprobs must be an integer from 0 to 20",
		},
		{
			name:   "top logprobs without flag",
			body:   `{"model":"kimi-k3","top_logprobs":5}`,
			reject: "top_logprobs requires logprobs=true",
		},
		{
			name:   "top logprobs flag false",
			body:   `{"model":"kimi-k3","logprobs":false,"top_logprobs":1}`,
			reject: "top_logprobs requires logprobs=true",
		},
		{
			name:   "illegal top logprobs wins over missing flag",
			body:   `{"model":"kimi-k3","top_logprobs":21}`,
			reject: "top_logprobs must be an integer from 0 to 20",
		},
		{name: "function name 128", body: `{"model":"kimi-k3","tools":[{"type":"function","function":{"name":"` + name128 + `"}}]}`},
		{
			name:   "function name 129",
			body:   `{"model":"kimi-k3","tools":[{"type":"function","function":{"name":"` + name129 + `"}}]}`,
			reject: "function name must match ^[a-zA-Z_][a-zA-Z0-9-_]{0,127}$",
		},
		{
			name:   "digit leading function name",
			body:   `{"model":"kimi-k3","tools":[{"type":"function","function":{"name":"1tool"}}]}`,
			reject: "function name must match ^[a-zA-Z_][a-zA-Z0-9-_]{0,127}$",
		},
		{name: "non function tool skipped", body: `{"model":"kimi-k3","tools":[{"type":"builtin","function":{"name":"1tool"}}]}`},
		{name: "history tool name skipped", body: `{"model":"kimi-k3","messages":[{"role":"assistant","tool_calls":[{"function":{"name":"1tool"}}]}]}`},
		{
			name:   "prediction wins over reasoning",
			body:   `{"model":"kimi-k3","prediction":{"type":"nope"},"reasoning_effort":"medium"}`,
			reject: "prediction.type must be content",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, msg, changed := AdaptKimiK3ChatBody([]byte(tc.body))
			require.Equal(t, tc.reject, msg)
			require.False(t, changed)
			require.True(t, bytes.Equal([]byte(tc.body), out))
		})
	}

	original := []byte(`{"model":"kimi-k3","temperature":0,"messages":[{"role":"user","content":[{"type":"text","text":"see"},{"type":"image_url","image_url":"data:image/png;base64,abc"},{"type":"video_url","video_url":"data:video/mp4;base64,zzz"},{"type":"image_url","image_url":{"url":"data:image/png;base64,obj"}}]}]}`)
	out, msg, changed := AdaptKimiK3ChatBody(original)
	require.Empty(t, msg)
	require.True(t, changed)
	require.True(t, strings.HasPrefix(string(out), `{"model":"kimi-k3","temperature":0,`))
	require.Equal(t, "data:image/png;base64,abc", gjson.GetBytes(out, "messages.0.content.1.image_url.url").String())
	require.Equal(t, gjson.JSON, gjson.GetBytes(out, "messages.0.content.1.image_url").Type)
	require.Equal(t, gjson.String, gjson.GetBytes(out, "messages.0.content.2.video_url").Type)
	require.Equal(t, "data:video/mp4;base64,zzz", gjson.GetBytes(out, "messages.0.content.2.video_url").String())
	require.Equal(t, "data:image/png;base64,obj", gjson.GetBytes(out, "messages.0.content.3.image_url.url").String())

	rejected := []byte(`{"model":"kimi-k3","max_completion_tokens":1048577,"messages":[{"role":"user","content":[{"type":"image_url","image_url":"data:image/png;base64,abc"}]}]}`)
	out, msg, changed = AdaptKimiK3ChatBody(rejected)
	require.Equal(t, "max_completion_tokens exceeds kimi-k3 maximum 1048576", msg)
	require.False(t, changed)
	require.True(t, bytes.Equal(rejected, out))
}

func TestKimiK3AdaptiveGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetKimiK3AdaptiveValidationCacheForTest()
	t.Cleanup(resetKimiK3AdaptiveValidationCacheForTest)

	t.Run("switch off forwards medium and string image", func(t *testing.T) {
		body := []byte(`{"model":"kimi-k3","reasoning_effort":"medium","messages":[{"role":"user","content":[{"type":"image_url","image_url":"data:image/png;base64,abc"}]}]}`)
		rec, upstream := forwardKimiK3Adaptive(t, nil, rawChatCompletionsTestAccount(), body)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "medium", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
		require.Equal(t, gjson.String, gjson.GetBytes(upstream.lastBody, "messages.0.content.0.image_url").Type)
		require.Equal(t, "data:image/png;base64,abc", gjson.GetBytes(upstream.lastBody, "messages.0.content.0.image_url").String())
	})

	t.Run("switch on non kimi forwards illegal body", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.4","reasoning_effort":"medium","top_logprobs":21}`)
		svcSettings := kimiK3AdaptiveSettingsForTest(t, "true")
		rec, upstream := forwardKimiK3Adaptive(t, svcSettings, rawChatCompletionsTestAccount(), body)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "medium", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
		require.Equal(t, int64(21), gjson.GetBytes(upstream.lastBody, "top_logprobs").Int())
	})

	t.Run("switch on mapped away from kimi-k3 forwards medium", func(t *testing.T) {
		body := []byte(`{"model":"kimi-k3","reasoning_effort":"medium"}`)
		account := rawChatCompletionsTestAccount()
		account.Credentials["model_mapping"] = map[string]any{"kimi-k3": "gpt-5.4"}
		rec, upstream := forwardKimiK3Adaptive(t, kimiK3AdaptiveSettingsForTest(t, "true"), account, body)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
		require.Equal(t, "medium", gjson.GetBytes(upstream.lastBody, "reasoning_effort").String())
	})

	t.Run("switch on rejects before upstream", func(t *testing.T) {
		body := []byte(`{"model":"kimi-k3","reasoning_effort":"medium","messages":[{"role":"user","content":"hi"}]}`)
		upstream := &httpUpstreamRecorder{err: io.ErrClosedPipe}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		svc := &OpenAIGatewayService{
			cfg:            rawChatCompletionsTestConfig(),
			httpUpstream:   upstream,
			settingService: kimiK3AdaptiveSettingsForTest(t, "true"),
		}
		_, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
		require.Error(t, err)
		require.True(t, strings.HasPrefix(err.Error(), "non-streaming openai protocol error:"))
		require.Empty(t, upstream.requests)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.type").String())
		require.Equal(t, "reasoning_effort must be low, high, or max", gjson.Get(rec.Body.String(), "error.message").String())
	})

	t.Run("switch on rejects mapped alias", func(t *testing.T) {
		body := []byte(`{"model":"alias","top_logprobs":5}`)
		account := rawChatCompletionsTestAccount()
		account.Credentials["model_mapping"] = map[string]any{"alias": "vendor/kimi-k3"}
		upstream := &httpUpstreamRecorder{err: io.ErrClosedPipe}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		svc := &OpenAIGatewayService{
			cfg:            rawChatCompletionsTestConfig(),
			httpUpstream:   upstream,
			settingService: kimiK3AdaptiveSettingsForTest(t, "true"),
		}
		_, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
		require.Error(t, err)
		require.Empty(t, upstream.requests)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, "top_logprobs requires logprobs=true", gjson.Get(rec.Body.String(), "error.message").String())
	})

	t.Run("switch on rewrites string image and keeps video", func(t *testing.T) {
		body := []byte(`{"model":"kimi-k3","messages":[{"role":"user","content":[{"type":"image_url","image_url":"data:image/png;base64,abc"},{"type":"video_url","video_url":"data:video/mp4;base64,zzz"}]}]}`)
		rec, upstream := forwardKimiK3Adaptive(t, kimiK3AdaptiveSettingsForTest(t, "true"), rawChatCompletionsTestAccount(), body)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "data:image/png;base64,abc", gjson.GetBytes(upstream.lastBody, "messages.0.content.0.image_url.url").String())
		require.Equal(t, gjson.String, gjson.GetBytes(upstream.lastBody, "messages.0.content.1.video_url").Type)
	})

	t.Run("switch on allows 128 char name and top logprobs 1", func(t *testing.T) {
		name := strings.Repeat("a", 128)
		body := []byte(`{"model":"kimi-k3","logprobs":true,"top_logprobs":1,"tools":[{"type":"function","function":{"name":"` + name + `"}}]}`)
		rec, upstream := forwardKimiK3Adaptive(t, kimiK3AdaptiveSettingsForTest(t, "true"), rawChatCompletionsTestAccount(), body)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, name, gjson.GetBytes(upstream.lastBody, "tools.0.function.name").String())
		require.Equal(t, int64(1), gjson.GetBytes(upstream.lastBody, "top_logprobs").Int())
		require.True(t, gjson.GetBytes(upstream.lastBody, "logprobs").Bool())
	})
}

func kimiK3AdaptiveSettingsForTest(t *testing.T, raw string) *SettingService {
	t.Helper()
	resetKimiK3AdaptiveValidationCacheForTest()
	repo := &gatewayTTLSettingRepo{data: map[string]string{}}
	if raw != "" {
		repo.data[SettingKeyKimiK3AdaptiveValidationEnabled] = raw
	}
	return NewSettingService(repo, &config.Config{})
}

func forwardKimiK3Adaptive(t *testing.T, settings *SettingService, account *Account, body []byte) (*httptest.ResponseRecorder, *httpUpstreamRecorder) {
	t.Helper()
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl_k3","object":"chat.completion","model":"kimi-k3","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`)),
	}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	svc := &OpenAIGatewayService{
		cfg:            rawChatCompletionsTestConfig(),
		httpUpstream:   upstream,
		settingService: settings,
	}
	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	return rec, upstream
}
