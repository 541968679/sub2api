package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAnthropicBridgeActiveSuffixItemCount(t *testing.T) {
	t.Run("plain user turn", func(t *testing.T) {
		var req apicompat.AnthropicRequest
		require.NoError(t, json.Unmarshal([]byte(`{
			"model":"gpt-5.5",
			"messages":[
				{"role":"user","content":"old"},
				{"role":"assistant","content":"done"},
				{"role":"user","content":"current"}
			]
		}`), &req))

		require.Equal(t, 1, anthropicBridgeActiveSuffixItemCount(&req))
	})

	t.Run("tool continuation keeps assistant call", func(t *testing.T) {
		var req apicompat.AnthropicRequest
		require.NoError(t, json.Unmarshal([]byte(`{
			"model":"gpt-5.5",
			"messages":[
				{"role":"user","content":"inspect"},
				{"role":"assistant","content":[{"type":"tool_use","id":"call_1","name":"Read","input":{"path":"a"}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"result"}]}
			]
		}`), &req))

		require.Equal(t, 2, anthropicBridgeActiveSuffixItemCount(&req))
	})
}

func TestMaybeAutoCompactAnthropicBridgePreservesOpaqueOutputAndSuffix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.144.1")

	compactResponse := `{
		"output":[
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"retained"}]},
			{"type":"compaction","encrypted_content":"opaque","unknown":{"nested":[1,2,3]}}
		],
		"usage":{"input_tokens":20,"output_tokens":3,"input_tokens_details":{"cached_tokens":7}}
	}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(compactResponse)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{Gateway: config.GatewayConfig{
			AnthropicBridgeAutoCompactEnabled:        true,
			AnthropicBridgeAutoCompactInputBytes:     1,
			AnthropicBridgeAutoCompactTimeoutSeconds: 60,
		}},
	}
	account := openAIAnthropicAutoCompactTestAccount()
	body := []byte(`{
		"model":"gpt-5.5",
		"instructions":"keep",
		"reasoning":{"effort":"max"},
		"prompt_cache_key":"drop-from-compact-body",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"system"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"` + strings.Repeat("history", 100) + `"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"current"}]}
		],
		"stream":true,
		"store":false
	}`)

	result := svc.maybeAutoCompactAnthropicBridge(
		context.Background(), c, account, body, "token", "session-key", "turn-state", 1,
	)

	require.True(t, result.Applied)
	require.Equal(t, 20, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, 7, result.Usage.CacheReadInputTokens)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, chatgptCodexURL+"/compact", upstream.requests[0].URL.String())
	require.Equal(t, "turn-state", upstream.requests[0].Header.Get("x-codex-turn-state"))
	require.Equal(t, "gpt-5.5", gjson.GetBytes(upstream.bodies[0], "model").String())
	// max → xhigh for non-gpt-5.6 models on compact path
	require.Equal(t, "xhigh", gjson.GetBytes(upstream.bodies[0], "reasoning.effort").String())
	require.Equal(t, int64(3), gjson.GetBytes(upstream.bodies[0], "input.#").Int())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "stream").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "store").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "prompt_cache_key").Exists())

	var updated struct {
		Input []json.RawMessage `json:"input"`
	}
	require.NoError(t, json.Unmarshal(result.Body, &updated))
	require.Len(t, updated.Input, 3)
	require.JSONEq(t, `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"retained"}]}`, string(updated.Input[0]))
	require.JSONEq(t, `{"type":"compaction","encrypted_content":"opaque","unknown":{"nested":[1,2,3]}}`, string(updated.Input[1]))
	require.Contains(t, string(updated.Input[2]), "current")
}

func TestMaybeAutoCompactAnthropicBridgeFailsOpen(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":[{"role":"user","content":"history"},{"role":"user","content":"current"}]}`)
	tests := []struct {
		name       string
		statusCode int
		response   string
	}{
		{name: "upstream status", statusCode: http.StatusBadRequest, response: `{"error":{"message":"unsupported"}}`},
		{name: "malformed response", statusCode: http.StatusOK, response: `{`},
		{name: "empty output", statusCode: http.StatusOK, response: `{"output":[]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tt.response)),
			}}
			svc := newAnthropicAutoCompactTestService(upstream, true)

			result := svc.maybeAutoCompactAnthropicBridge(
				context.Background(), c, openAIAnthropicAutoCompactTestAccount(), body, "token", "session-key", "", 1,
			)

			require.False(t, result.Applied)
			require.True(t, bytes.Equal(body, result.Body))
		})
	}
}

func TestMaybeAutoCompactAnthropicBridgeSkipsDisabledAndUnsplittableRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	body := []byte(`{"model":"gpt-5.5","input":[{"role":"user","content":"only-current-turn"}]}`)

	upstream := &httpUpstreamRecorder{}
	disabled := newAnthropicAutoCompactTestService(upstream, false)
	result := disabled.maybeAutoCompactAnthropicBridge(
		context.Background(), c, openAIAnthropicAutoCompactTestAccount(), body, "token", "session-key", "", 1,
	)
	require.False(t, result.Applied)

	enabled := newAnthropicAutoCompactTestService(upstream, true)
	// Unsplittable: only one input item for activeSuffixItems=1
	result = enabled.maybeAutoCompactAnthropicBridge(
		context.Background(), c, openAIAnthropicAutoCompactTestAccount(), body, "token", "session-key", "", 1,
	)
	require.False(t, result.Applied)

	apiKeyAccount := openAIAnthropicAutoCompactTestAccount()
	apiKeyAccount.Type = AccountTypeAPIKey
	result = enabled.maybeAutoCompactAnthropicBridge(
		context.Background(), c, apiKeyAccount, body, "token", "session-key", "", 1,
	)
	require.False(t, result.Applied)

	// Claude / fable labels in the body model field are skipped
	claudeBody := []byte(`{"model":"claude-fable-5","input":[{"role":"user","content":"history"},{"role":"user","content":"current"}]}`)
	result = enabled.maybeAutoCompactAnthropicBridge(
		context.Background(), c, openAIAnthropicAutoCompactTestAccount(), claudeBody, "token", "session-key", "", 1,
	)
	require.False(t, result.Applied)
	require.Empty(t, upstream.requests)
}

func TestIsMappedGPT5AnthropicBridgeModel(t *testing.T) {
	require.True(t, isMappedGPT5AnthropicBridgeModel("gpt-5.5"))
	require.True(t, isMappedGPT5AnthropicBridgeModel("gpt-5.4-mini"))
	require.True(t, isMappedGPT5AnthropicBridgeModel("openai/gpt-5.5"))
	require.True(t, isMappedGPT5AnthropicBridgeModel("gpt5.6"))
	require.False(t, isMappedGPT5AnthropicBridgeModel("claude-fable-5"))
	require.False(t, isMappedGPT5AnthropicBridgeModel("claude-opus-4-6"))
	require.False(t, isMappedGPT5AnthropicBridgeModel("claude-haiku-4-5"))
	require.False(t, isMappedGPT5AnthropicBridgeModel("gpt-4o"))
}

func TestForwardAsAnthropicAutoCompactsBeforeGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":16,
		"messages":[
			{"role":"user","content":"` + strings.Repeat("old-history-", 100) + `"},
			{"role":"assistant","content":"done"},
			{"role":"user","content":"current-turn"}
		],
		"stream":false
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"output":[{"type":"compaction","encrypted_content":"opaque-history"}],
				"usage":{"input_tokens":20,"output_tokens":3,"input_tokens_details":{"cached_tokens":7}}
			}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_auto_compact"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.completed","response":{"id":"resp_auto_compact","object":"response","model":"gpt-5.5","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
				"",
				"data: [DONE]",
				"",
			}, "\n"))),
		},
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
			Gateway: config.GatewayConfig{
				AnthropicBridgeAutoCompactEnabled:        true,
				AnthropicBridgeAutoCompactInputBytes:     1,
				AnthropicBridgeAutoCompactTimeoutSeconds: 60,
			},
		},
	}

	result, err := svc.ForwardAsAnthropic(
		context.Background(), c, openAIAnthropicAutoCompactTestAccount(), body, "session-key", "gpt-5.5",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, chatgptCodexURL+"/compact", upstream.requests[0].URL.String())
	require.Equal(t, chatgptCodexURL, upstream.requests[1].URL.String())
	require.Equal(t, upstream.requests[0].Header.Get("session_id"), upstream.requests[1].Header.Get("session_id"))
	require.Equal(t, gjson.GetBytes(upstream.bodies[1], "model").String(), gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "compaction", gjson.GetBytes(upstream.bodies[1], "input.0.type").String())
	require.Equal(t, "current-turn", gjson.GetBytes(upstream.bodies[1], "input.1.content.0.text").String())
	// compact usage merged into generation usage: 20+5 input, 3+2 output, 7 cache
	require.Equal(t, 25, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.Equal(t, 7, result.Usage.CacheReadInputTokens)
}

func TestForwardAsAnthropicAutoCompactSkipsWhenBodyModelIsNotGPT5(t *testing.T) {
	// When the mapped upstream model is not gpt-5*, auto-compact must not fire.
	// Simulate by using a non-gpt body model that still reaches generation.
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model":"gpt-4o",
		"max_tokens":16,
		"messages":[
			{"role":"user","content":"` + strings.Repeat("old-history-", 100) + `"},
			{"role":"assistant","content":"done"},
			{"role":"user","content":"current-turn"}
		],
		"stream":false
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_skip"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.completed","response":{"id":"resp_skip","object":"response","model":"gpt-4o","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n"))),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
			Gateway: config.GatewayConfig{
				AnthropicBridgeAutoCompactEnabled:        true,
				AnthropicBridgeAutoCompactInputBytes:     1,
				AnthropicBridgeAutoCompactTimeoutSeconds: 60,
			},
		},
	}

	account := openAIAnthropicAutoCompactTestAccount()
	// Force model mapping to leave gpt-4o as-is (non gpt-5 gate).
	result, err := svc.ForwardAsAnthropic(
		context.Background(), c, account, body, "session-key", "gpt-4o",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, chatgptCodexURL, upstream.requests[0].URL.String())
	require.NotContains(t, string(upstream.bodies[0]), `"type":"compaction"`)
}

func TestForwardAsAnthropicAutoCompactSkipsClientCompactRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	// looksLikeClaudeCodeCompactPrompt requires the summary task + all section markers.
	compactPrompt := "Your task is to create a detailed summary of the conversation so far.\n" +
		"<analysis>\nnotes\n</analysis>\n" +
		"<summary>\nAll user messages\nPending Tasks\nCurrent Work\n</summary>"
	compactPromptJSON, err := json.Marshal(compactPrompt)
	require.NoError(t, err)
	body := []byte(`{
		"model":"gpt-5.5",
		"max_tokens":16,
		"messages":[
			{"role":"user","content":"` + strings.Repeat("old-history-", 100) + `"},
			{"role":"user","content":` + string(compactPromptJSON) + `}
		],
		"stream":false
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// Client-initiated compact must not run pre-generation auto-compact first.
	// Only the normal compact/generation path should hit upstream once.
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_client_compact"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.completed","response":{"id":"resp_client_compact","object":"response","model":"gpt-5.5","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"summary ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n"))),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
			Gateway: config.GatewayConfig{
				AnthropicBridgeAutoCompactEnabled:        true,
				AnthropicBridgeAutoCompactInputBytes:     1,
				AnthropicBridgeAutoCompactTimeoutSeconds: 60,
			},
		},
	}

	result, err := svc.ForwardAsAnthropic(
		context.Background(), c, openAIAnthropicAutoCompactTestAccount(), body, "session-key", "gpt-5.5",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	// Client compact path: no separate pre-gen /responses/compact auto pass.
	// First (and only) upstream call is the compact-request generation itself.
	require.Len(t, upstream.requests, 1)
	require.Equal(t, chatgptCodexURL, upstream.requests[0].URL.String())
}

func openAIAnthropicAutoCompactTestAccount() *Account {
	return &Account{
		ID:          77,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}
}

func newAnthropicAutoCompactTestService(upstream HTTPUpstream, enabled bool) *OpenAIGatewayService {
	return &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{Gateway: config.GatewayConfig{
			AnthropicBridgeAutoCompactEnabled:        enabled,
			AnthropicBridgeAutoCompactInputBytes:     1,
			AnthropicBridgeAutoCompactTimeoutSeconds: 60,
		}},
	}
}
