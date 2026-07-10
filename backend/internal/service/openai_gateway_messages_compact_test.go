//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func testClaudeCodeCompactPrompt() string {
	return strings.Join([]string{
		"Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.",
		"Before providing your final summary, wrap your analysis in <analysis> tags.",
		"<analysis>",
		"</analysis>",
		"<summary>",
		"6. All user messages:",
		"7. Pending Tasks:",
		"8. Current Work:",
		"</summary>",
	}, "\n")
}

func compactTestSSE(response string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: " + response + "\n\n")),
	}
}

func compactCompletedSSE(id, model, text string, inputTokens, outputTokens int) *http.Response {
	payload, _ := json.Marshal(map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"id":     id,
			"object": "response",
			"model":  model,
			"status": "completed",
			"output": []any{map[string]any{
				"type":   "message",
				"role":   "assistant",
				"status": "completed",
				"content": []any{map[string]any{
					"type": "output_text",
					"text": text,
				}},
			}},
			"usage": map[string]any{
				"input_tokens":  inputTokens,
				"output_tokens": outputTokens,
				"total_tokens":  inputTokens + outputTokens,
			},
		},
	})
	return compactTestSSE(string(payload))
}

func compactJSONError(statusCode int, code, message, requestID string) *http.Response {
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"type":    code,
		},
	})
	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{requestID},
		},
		Body: io.NopCloser(bytes.NewReader(body)),
	}
}

func compactJSONErrorWithUsage(statusCode int, code, message, requestID string, inputTokens, outputTokens int) *http.Response {
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
			"type":    code,
		},
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	})
	return &http.Response{
		StatusCode: statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{requestID},
		},
		Body: io.NopCloser(bytes.NewReader(body)),
	}
}

func TestIsClaudeCodeCompactAnthropicRequest(t *testing.T) {
	req := &apicompat.AnthropicRequest{Messages: []apicompat.AnthropicMessage{
		{Role: "user", Content: json.RawMessage(`"normal earlier message"`)},
		{Role: "assistant", Content: json.RawMessage(`"ok"`)},
		{Role: "user", Content: json.RawMessage(fmt.Sprintf(`[{"type":"text","text":%q}]`, testClaudeCodeCompactPrompt()))},
	}}
	require.True(t, isClaudeCodeCompactAnthropicRequest(req))

	notCompact := &apicompat.AnthropicRequest{Messages: []apicompat.AnthropicMessage{
		{Role: "user", Content: json.RawMessage(`"please compact this code"`)},
	}}
	require.False(t, isClaudeCodeCompactAnthropicRequest(notCompact))
}

func TestForwardAsAnthropic_APIKeyCompactFallbackUsesUntrimmedTranscript(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	messages := make([]map[string]any, 0, openAICompatAnthropicReplayMaxTailMessages+4)
	messages = append(messages, map[string]any{"role": "user", "content": "EARLY_CONTEXT_MUST_SURVIVE"})
	for i := 0; i < openAICompatAnthropicReplayMaxTailMessages+2; i++ {
		messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprintf("ordinary-message-%02d", i)})
	}
	messages = append(messages, map[string]any{
		"role":    "user",
		"content": []map[string]any{{"type": "text", "text": testClaudeCodeCompactPrompt()}},
	})
	body, err := json.Marshal(map[string]any{
		"model":      "claude-opus-4-8",
		"max_tokens": 2048,
		"stream":     true,
		"messages":   messages,
	})
	require.NoError(t, err)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)

	failed := `{"type":"response.failed","response":{"id":"resp_initial","status":"failed","error":{"code":"context_length_exceeded","message":"Your input exceeds the context window."},"output":[],"usage":{"input_tokens":90000,"output_tokens":0,"total_tokens":90000}}}`
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactTestSSE(failed),
		compactCompletedSSE("resp_chunk", "gpt-5.4-mini", "chunk summary preserves early state", 1000, 50),
		compactCompletedSSE("resp_merge", "gpt-5.4-mini", "# Compact Capsule\n\nusable final summary", 200, 30),
		compactCompletedSSE("resp_next", "gpt-5.5", "continued without invalid recovery binding", 50, 10),
	}}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com/v1",
			"compact_model_mapping": map[string]any{
				"gpt-5.5": "gpt-5.4-mini",
			},
		},
	}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "stable-session", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.4-mini", result.UpstreamModel)
	require.Contains(t, rec.Body.String(), "usable final summary")
	require.Len(t, upstream.bodies, 3)
	require.NotContains(t, string(upstream.bodies[0]), "EARLY_CONTEXT_MUST_SURVIVE",
		"normal API-key forwarding should still use the replay guard")
	require.Contains(t, string(upstream.bodies[1]), "EARLY_CONTEXT_MUST_SURVIVE",
		"compact fallback must use the request captured before replay-guard trimming")
	require.Equal(t, "gpt-5.4-mini", gjson.GetBytes(upstream.bodies[0], "model").String())

	nextBody := []byte(`{"model":"claude-opus-4-8","max_tokens":256,"stream":true,"messages":[{"role":"user","content":"continue the task"}]}`)
	nextRec := httptest.NewRecorder()
	nextContext, _ := gin.CreateTestContext(nextRec)
	nextContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(nextBody))
	nextContext.Request.Header.Set("Content-Type", "application/json")
	nextContext.Set(openAIClaudeGPTBridgeServiceContextKey, true)

	_, err = svc.ForwardAsAnthropic(context.Background(), nextContext, account, nextBody, "stable-session", "gpt-5.5")
	require.NoError(t, err)
	require.Len(t, upstream.bodies, 4)
	require.False(t, gjson.GetBytes(upstream.bodies[3], "previous_response_id").Exists(),
		"store=false compact recovery responses must not be attached to the next turn")
}

func TestForwardAsAnthropic_CompactHTTPContextErrorRecoversWithChunks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(fmt.Sprintf(`{"model":"claude-opus-4-8","max_tokens":2048,"stream":true,"messages":[{"role":"user","content":"state that must be summarized"},{"role":"user","content":[{"type":"text","text":%q}]}]}`, testClaudeCodeCompactPrompt()))
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactJSONError(http.StatusBadRequest, "context_length_exceeded", "Your input exceeds the context window.", "rid_context"),
		compactCompletedSSE("resp_chunk", "gpt-5.4-mini", "chunk summary", 100, 20),
		compactCompletedSSE("resp_merge", "gpt-5.4-mini", "usable summary after HTTP context error", 40, 10),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "compact-http-context", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "usable summary after HTTP context error")
	require.Len(t, upstream.bodies, 3)
}

func TestForwardAsAnthropic_CompactConfiguredFallbackModelRecoversFromRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(fmt.Sprintf(`{"model":"claude-opus-4-8","max_tokens":2048,"stream":true,"messages":[{"role":"user","content":"active task state"},{"role":"user","content":[{"type":"text","text":%q}]}]}`, testClaudeCodeCompactPrompt()))
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactJSONError(http.StatusTooManyRequests, "rate_limit_error", "The usage limit has been reached for compact-primary.", "rid_primary_limited"),
		compactCompletedSSE("resp_chunk", "compact-secondary", "fallback chunk summary", 100, 20),
		compactCompletedSSE("resp_merge", "compact-secondary", "usable summary from configured fallback", 40, 10),
	}}
	rateLimitRepo := &openAI429SnapshotRepo{}
	svc := &OpenAIGatewayService{
		cfg:              rawChatCompletionsTestConfig(),
		httpUpstream:     upstream,
		rateLimitService: NewRateLimitService(rateLimitRepo, nil, nil, nil, nil),
	}
	account := rawChatCompletionsTestAccount()
	account.Credentials["compact_model_mapping"] = map[string]any{"gpt-5.5": "compact-primary"}
	account.Credentials["compact_model_fallbacks"] = map[string]any{"compact-primary": []any{"compact-secondary"}}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "compact-model-fallback", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "compact-secondary", result.BillingModel)
	require.Equal(t, "compact-secondary", result.UpstreamModel)
	require.Contains(t, rec.Body.String(), "usable summary from configured fallback")
	require.Len(t, upstream.bodies, 3)
	require.Equal(t, "compact-primary", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "compact-secondary", gjson.GetBytes(upstream.bodies[1], "model").String())
	require.Equal(t, "compact-secondary", gjson.GetBytes(upstream.bodies[2], "model").String())
	require.Zero(t, rateLimitRepo.rateLimitedID,
		"a successful compact fallback model must not rate-limit the whole account")
}

func TestRunAnthropicCompactRecoveryWithModelFallbacks_SuccessDoesNotRateLimitAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	limited := compactJSONError(
		http.StatusTooManyRequests,
		"rate_limit_error",
		"The primary compact model is temporarily rate limited.",
		"rid_recovery_primary_limited",
	)
	limited.Header.Set("x-codex-primary-used-percent", "100")
	limited.Header.Set("x-codex-primary-reset-after-seconds", "3600")
	limited.Header.Set("x-codex-primary-window-minutes", "300")
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		limited,
		compactCompletedSSE("resp_fallback_chunk", "compact-secondary", "fallback chunk summary", 100, 20),
		compactCompletedSSE("resp_fallback_merge", "compact-secondary", "# Compact Capsule\n\nusable fallback summary", 40, 10),
	}}
	rateLimitRepo := &openAI429SnapshotRepo{}
	svc := &OpenAIGatewayService{
		cfg:              rawChatCompletionsTestConfig(),
		httpUpstream:     upstream,
		rateLimitService: NewRateLimitService(rateLimitRepo, nil, nil, nil, nil),
	}
	account := rawChatCompletionsTestAccount()
	fullReq := &apicompat.AnthropicRequest{Messages: []apicompat.AnthropicMessage{
		{Role: "user", Content: json.RawMessage(`"active task state"`)},
		{Role: "user", Content: json.RawMessage(fmt.Sprintf("%q", testClaudeCodeCompactPrompt()))},
	}}

	result, err := svc.runAnthropicCompactRecoveryWithModelFallbacks(
		context.Background(), c, account, fullReq, "sk-test", false,
		"claude-opus-4-8", []string{"compact-primary", "compact-secondary"},
		time.Now(), OpenAIUsage{}, false, "rid_initial",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "compact-secondary", result.UpstreamModel)
	require.Contains(t, rec.Body.String(), "usable fallback summary")
	require.Zero(t, rateLimitRepo.rateLimitedID,
		"a model-local compact fallback must not mark the account unavailable when another model succeeds")
}

func TestRunOpenAIAnthropicCompactRecoveryRequest_PreservesClientHTTPError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactJSONError(
			http.StatusBadRequest,
			"invalid_request_error",
			"Unsupported compact request field.",
			"rid_compact_client_error",
		),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	_, _, requestID, err := svc.runOpenAIAnthropicCompactRecoveryRequest(
		context.Background(), c, rawChatCompletionsTestAccount(), "sk-test", "gpt-5.5",
		"summarize", "transcript", 1024,
	)

	require.Equal(t, "rid_compact_client_error", requestID)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(failoverErr.ResponseBody, "error.type").String())
	require.Equal(t, "Unsupported compact request field.", gjson.GetBytes(failoverErr.ResponseBody, "error.message").String())
}

func TestRunOpenAIAnthropicCompactRecoveryRequest_ClassifiesTerminalPolicyError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	terminalFailure := compactTestSSE(`{"type":"response.failed","response":{"id":"resp_policy","status":"failed","error":{"type":"content_policy_error","code":"content_policy","message":"This request is not allowed by policy."},"output":[],"usage":{"input_tokens":30,"output_tokens":0,"total_tokens":30}}}`)
	terminalFailure.Header.Set("x-request-id", "rid_compact_policy")
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{terminalFailure}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	_, usage, requestID, err := svc.runOpenAIAnthropicCompactRecoveryRequest(
		context.Background(), c, rawChatCompletionsTestAccount(), "sk-test", "gpt-5.5",
		"summarize", "transcript", 1024,
	)

	require.Equal(t, "rid_compact_policy", requestID)
	require.Equal(t, 30, usage.InputTokens)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Contains(t, string(failoverErr.ResponseBody), "not allowed by policy")
}

func TestRunOpenAIAnthropicCompactRecoveryRequest_DropsContinuationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("session_id", "client-session")
	c.Request.Header.Set("conversation_id", "client-conversation")
	c.Request.Header.Set("x-codex-turn-state", "client-turn-state")
	c.Request.Header.Set("x-codex-turn-metadata", "client-turn-metadata")

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactCompletedSSE("resp_recovery", "gpt-5.5", "recovery summary", 10, 5),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()
	account.Type = AccountTypeAPIKey
	account.Credentials = map[string]any{
		"api_key":  "sk-test",
		"base_url": "https://example.com/v1",
	}

	_, _, _, err := svc.runOpenAIAnthropicCompactRecoveryRequest(
		context.Background(), c, account, "sk-test", "gpt-5.5", "summarize", "transcript", 1024,
	)
	require.NoError(t, err)
	require.Len(t, upstream.reqs, 1)
	for _, header := range []string{"session_id", "conversation_id", "x-codex-turn-state", "x-codex-turn-metadata"} {
		require.Empty(t, upstream.reqs[0].Header.Get(header), "compact recovery must not inherit %s", header)
	}
}

func TestForwardAsAnthropic_CompactReasoningOnlyFallsBackToUsableSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(fmt.Sprintf(`{"model":"claude-opus-4-8","max_tokens":2048,"stream":true,"messages":[{"role":"user","content":"real task state"},{"role":"user","content":[{"type":"text","text":%q}]}]}`, testClaudeCodeCompactPrompt()))
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)

	reasoningOnly := `{"type":"response.completed","response":{"id":"resp_reasoning","status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"internal only"}]}],"usage":{"input_tokens":69000,"output_tokens":68,"total_tokens":69068}}}`
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactTestSSE(reasoningOnly),
		compactCompletedSSE("resp_chunk", "gpt-5.5", "chunk summary", 100, 20),
		compactCompletedSSE("resp_merge", "gpt-5.5", "usable compact after reasoning-only", 40, 10),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "stable-session", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "usable compact after reasoning-only")
}

func TestForwardAsAnthropic_CompactRecoveryDoesNotPersistFailedOAuthTurnState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	compactBody := []byte(fmt.Sprintf(`{"model":"claude-opus-4-8","max_tokens":2048,"stream":true,"messages":[{"role":"user","content":"active state"},{"role":"user","content":[{"type":"text","text":%q}]}]}`, testClaudeCodeCompactPrompt()))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(compactBody))
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)

	reasoningOnly := compactTestSSE(`{"type":"response.completed","response":{"id":"resp_reasoning_state","status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"internal only"}]}],"usage":{"input_tokens":69000,"output_tokens":68,"total_tokens":69068}}}`)
	reasoningOnly.Header.Set("x-codex-turn-state", "stale-compact-turn-state")
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		reasoningOnly,
		compactCompletedSSE("resp_chunk_state", "gpt-5.5", "chunk summary", 100, 20),
		compactCompletedSSE("resp_merge_state", "gpt-5.5", "usable recovered summary", 40, 10),
		compactCompletedSSE("resp_next_state", "gpt-5.5", "next turn remains stateless", 20, 5),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := &Account{
		ID:          1002,
		Name:        "openai-oauth-compact-state",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}
	svc.bindOpenAICompatSessionResponseID(context.Background(), c, account, "stable-oauth-session", "resp_stale_before_compact")
	svc.bindOpenAICompatSessionTurnState(context.Background(), c, account, "stable-oauth-session", "turn_stale_before_compact")
	require.Equal(t, "turn_stale_before_compact", svc.getOpenAICompatSessionTurnState(
		context.Background(), c, account, "stable-oauth-session",
	),
		"the test must prove an old turn state existed before compact recovery")
	require.Equal(t, "resp_stale_before_compact", svc.getOpenAICompatSessionResponseID(
		context.Background(), c, account, "stable-oauth-session",
	),
		"the test must prove an old response binding existed before compact recovery")

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, compactBody, "stable-oauth-session", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.SkipContinuationBinding)
	require.Contains(t, rec.Body.String(), "usable recovered summary")

	nextBody := []byte(`{"model":"claude-opus-4-8","max_tokens":256,"stream":true,"messages":[{"role":"user","content":"continue"}]}`)
	nextRec := httptest.NewRecorder()
	nextContext, _ := gin.CreateTestContext(nextRec)
	nextContext.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(nextBody))
	nextContext.Set(openAIClaudeGPTBridgeServiceContextKey, true)

	_, err = svc.ForwardAsAnthropic(context.Background(), nextContext, account, nextBody, "stable-oauth-session", "gpt-5.5")
	require.NoError(t, err)
	require.Len(t, upstream.reqs, 4)
	require.Empty(t, upstream.reqs[3].Header.Get("x-codex-turn-state"),
		"compact recovery must clear the old turn state before the next turn")
	require.False(t, gjson.GetBytes(upstream.bodies[3], "previous_response_id").Exists(),
		"compact recovery must clear the old response binding before the next turn")
}

func TestRunAnthropicCompactRecovery_RecursivelySplitsChunkThatStillExceedsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	leftTranscript := "LEFT_HALF_MUST_SURVIVE\n" + strings.Repeat("a", 7_000)
	rightTranscript := "RIGHT_HALF_MUST_SURVIVE\n" + strings.Repeat("b", 7_000)
	fullReq := &apicompat.AnthropicRequest{Messages: []apicompat.AnthropicMessage{
		{Role: "user", Content: json.RawMessage(fmt.Sprintf("%q", leftTranscript+"\n"+rightTranscript))},
		{Role: "user", Content: json.RawMessage(fmt.Sprintf("%q", testClaudeCodeCompactPrompt()))},
	}}

	chunkTooLarge := `{"type":"response.failed","response":{"id":"resp_chunk_too_large","status":"failed","error":{"code":"context_length_exceeded","message":"Chunk exceeds the context window."},"output":[],"usage":{"input_tokens":12000,"output_tokens":0,"total_tokens":12000}}}`
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactTestSSE(chunkTooLarge),
		compactCompletedSSE("resp_left", "gpt-5.5", "left half summary", 100, 20),
		compactCompletedSSE("resp_right", "gpt-5.5", "right half summary", 110, 21),
		compactCompletedSSE("resp_merge", "gpt-5.5", "# Compact Capsule\n\nrecursive split recovered the task", 50, 12),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()

	result, err := svc.runAnthropicCompactRecovery(
		context.Background(), c, account, fullReq, "sk-test", false,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
		OpenAIUsage{}, false, "rid_initial",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "recursive split recovered the task")
	require.Len(t, upstream.bodies, 4)
	firstAttempt := gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String()
	leftRetry := gjson.GetBytes(upstream.bodies[1], "input.0.content.0.text").String()
	rightRetry := gjson.GetBytes(upstream.bodies[2], "input.0.content.0.text").String()
	require.Contains(t, firstAttempt, "LEFT_HALF_MUST_SURVIVE")
	require.Contains(t, firstAttempt, "RIGHT_HALF_MUST_SURVIVE")
	require.Contains(t, leftRetry, "LEFT_HALF_MUST_SURVIVE")
	require.NotContains(t, leftRetry, "RIGHT_HALF_MUST_SURVIVE")
	require.Contains(t, rightRetry, "RIGHT_HALF_MUST_SURVIVE")
	require.Equal(t, 12_260, result.Usage.InputTokens)
	require.Equal(t, 53, result.Usage.OutputTokens)
}

func TestRunAnthropicCompactRecovery_HTTPContextSplitPreservesFailedAttemptUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	leftTranscript := "HTTP_LEFT_HALF\n" + strings.Repeat("中", 7_000)
	rightTranscript := "HTTP_RIGHT_HALF\n" + strings.Repeat("文", 7_000)
	fullReq := &apicompat.AnthropicRequest{Messages: []apicompat.AnthropicMessage{
		{Role: "user", Content: json.RawMessage(fmt.Sprintf("%q", leftTranscript+"\n"+rightTranscript))},
		{Role: "user", Content: json.RawMessage(fmt.Sprintf("%q", testClaudeCodeCompactPrompt()))},
	}}

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactJSONErrorWithUsage(
			http.StatusBadRequest, "context_length_exceeded", "Chunk exceeds the context window.",
			"rid_http_parent", 8_000, 0,
		),
		compactCompletedSSE("resp_http_left", "gpt-5.5", "HTTP left summary", 100, 20),
		compactCompletedSSE("resp_http_right", "gpt-5.5", "HTTP right summary", 110, 21),
		compactCompletedSSE("resp_http_merge", "gpt-5.5", "# Compact Capsule\n\nHTTP recursive recovery", 50, 12),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()

	result, err := svc.runAnthropicCompactRecovery(
		context.Background(), c, account, fullReq, "sk-test", false,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now(),
		OpenAIUsage{}, false, "rid_initial",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "HTTP recursive recovery")
	require.Len(t, upstream.bodies, 4)
	require.Equal(t, 8_260, result.Usage.InputTokens)
	require.Equal(t, 53, result.Usage.OutputTokens)
}

func TestMergeAnthropicCompactSummaries_HTTPContextOverflowShrinksMergeGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	compactPrompt := testClaudeCodeCompactPrompt()
	summaries := []string{
		"## Left\nLEFT_MERGE_MARKER\n" + strings.Repeat("a", 3_000),
		"## Right\nRIGHT_MERGE_MARKER\n" + strings.Repeat("b", 3_000),
	}
	initialTarget := runeLen(buildAnthropicCompactMergePrompt(compactPrompt, summaries)) + 1

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactJSONErrorWithUsage(
			http.StatusBadRequest, "context_length_exceeded", "Merge exceeds the context window.",
			"rid_merge_parent", 8_000, 0,
		),
		compactCompletedSSE("resp_merge_left", "gpt-5.5", "reduced left", 100, 20),
		compactCompletedSSE("resp_merge_right", "gpt-5.5", "reduced right", 110, 21),
		compactCompletedSSE("resp_merge_final", "gpt-5.5", "# Compact Capsule\n\nHTTP merge recovery", 50, 12),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()

	response, usage, requestID, err := svc.mergeAnthropicCompactSummaries(
		context.Background(), c, account, "sk-test", "gpt-5.5",
		compactPrompt, summaries, initialTarget, 0,
	)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Contains(t, openAIResponsesOutputText(response), "HTTP merge recovery")
	require.Equal(t, "rid_merge_parent", requestID)
	require.Len(t, upstream.bodies, 4)
	require.Contains(t, gjson.GetBytes(upstream.bodies[1], "input.0.content.0.text").String(), "LEFT_MERGE_MARKER")
	require.NotContains(t, gjson.GetBytes(upstream.bodies[1], "input.0.content.0.text").String(), "RIGHT_MERGE_MARKER")
	require.Contains(t, gjson.GetBytes(upstream.bodies[2], "input.0.content.0.text").String(), "RIGHT_MERGE_MARKER")
	require.Contains(t, gjson.GetBytes(upstream.bodies[3], "input.0.content.0.text").String(), "reduced left")
	require.Contains(t, gjson.GetBytes(upstream.bodies[3], "input.0.content.0.text").String(), "reduced right")
	require.Equal(t, 8_260, usage.InputTokens)
	require.Equal(t, 53, usage.OutputTokens)
}

func TestSummarizeAnthropicCompactChunk_SplitBudgetStopsFurtherRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactJSONErrorWithUsage(
			http.StatusBadRequest, "context_length_exceeded", "Chunk exceeds the context window.",
			"rid_budget_exhausted", 321, 0,
		),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()
	remainingSplits := 0

	summaries, usage, requestID, err := svc.summarizeAnthropicCompactChunk(
		context.Background(), c, account, "sk-test", "gpt-5.5",
		strings.Repeat("x", openAIAnthropicCompactFallbackMinSplitRunes*2),
		"1/1", 0, &remainingSplits,
	)

	require.Error(t, err)
	require.True(t, isOpenAICompactContextLengthError(err))
	require.Empty(t, summaries)
	require.Equal(t, "rid_budget_exhausted", requestID)
	require.Equal(t, 321, usage.InputTokens)
	require.Len(t, upstream.bodies, 1)
}

func TestMergeAnthropicCompactSummaries_AttemptBudgetCapsAllRecursiveRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	summaries := make([]string, 0, openAIAnthropicCompactMergeAttemptBudget+8)
	for i := 0; i < cap(summaries); i++ {
		summaries = append(summaries, fmt.Sprintf("## Summary %d\n%s", i+1, strings.Repeat("x", 4_500)))
	}
	responses := make([]*http.Response, 0, 256)
	for i := 0; i < cap(responses); i++ {
		responses = append(responses, compactJSONError(
			http.StatusBadRequest,
			"context_length_exceeded",
			"Merge exceeds the context window.",
			fmt.Sprintf("rid_merge_budget_%03d", i+1),
		))
	}
	upstream := &httpUpstreamSequenceRecorder{responses: responses}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}

	response, _, _, err := svc.mergeAnthropicCompactSummaries(
		context.Background(), c, rawChatCompletionsTestAccount(), "sk-test", "gpt-5.5",
		testClaudeCodeCompactPrompt(), summaries, openAIAnthropicCompactFallbackMinSplitRunes, 0,
	)

	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, openAIAnthropicCompactMergeAttemptBudget, len(upstream.bodies),
		"all recursive merge branches must share one upstream request budget")
	require.Contains(t, openAIResponsesOutputText(response), "# Compact Capsule")
}
