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
