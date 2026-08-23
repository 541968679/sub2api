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
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func anthropicCompatCompletedUpstreamSSE(responseID string) string {
	return strings.Join([]string{
		fmt.Sprintf(`data: {"type":"response.completed","response":{"id":%q,"object":"response","model":"gpt-5.5","status":"completed","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`, responseID),
		"",
		"data: [DONE]",
		"",
	}, "\n")
}

func forwardAsAnthropicAPIKeyTestAccount() *Account {
	return &Account{
		ID:          1685,
		Name:        "openai-apikey-full-replay",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://example.com/v1",
			"model_mapping": map[string]any{
				"claude-opus-4-8": "gpt-5.5",
			},
		},
	}
}

func forwardAsAnthropicOAuthTestAccount() *Account {
	return &Account{
		ID:          42,
		Name:        "openai-oauth-continuation",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
			"model_mapping": map[string]any{
				"claude-opus-4-8": "gpt-5.5",
			},
		},
	}
}

func newForwardAsAnthropicTestContext(body []byte, requestID string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if requestID != "" {
		req = req.WithContext(context.WithValue(req.Context(), ctxkey.RequestID, requestID))
		req.Header.Set("X-Request-ID", requestID)
	}
	c.Request = req
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)
	c.Set("api_key", &APIKey{ID: 317})
	return c, rec
}

func newForwardAsAnthropicTestService(upstream *httpUpstreamRecorder) *OpenAIGatewayService {
	return &OpenAIGatewayService{
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
		httpUpstream: upstream,
	}
}

func completedUpstreamRecorder(responseID string) *httpUpstreamRecorder {
	return &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_full_replay"}},
		Body:       io.NopCloser(strings.NewReader(anthropicCompatCompletedUpstreamSSE(responseID))),
	}}
}

func marshalAnthropicMessagesBody(t *testing.T, messages []map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"model":      "claude-opus-4-8",
		"max_tokens": 64,
		"stream":     true,
		"messages":   messages,
	})
	require.NoError(t, err)
	return body
}

func TestForwardAsAnthropic_APIKeyFullReplayKeepsHistoryDespitePreviousResponseID(t *testing.T) {
	t.Parallel()

	messages := make([]map[string]any, 0, 20)
	messages = append(messages, map[string]any{"role": "user", "content": "EARLY_USER_TURN_01"})
	for i := 1; i <= 17; i++ {
		messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprintf("filler-user-%02d", i)})
	}
	messages = append(messages, map[string]any{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "ASSISTANT_TURN_19"},
			{"type": "tool_use", "id": "toolu_read_1", "name": "Read", "input": map[string]any{"file_path": "CLAUDE.md"}},
		},
	})
	messages = append(messages, map[string]any{
		"role": "user",
		"content": []map[string]any{
			{"type": "tool_result", "tool_use_id": "toolu_read_1", "content": "tool result for latest turn"},
		},
	})
	require.Len(t, messages, 20)

	body := marshalAnthropicMessagesBody(t, messages)
	c, _ := newForwardAsAnthropicTestContext(body, "rid-full-replay-20")
	upstream := completedUpstreamRecorder("resp_bound_after")
	svc := newForwardAsAnthropicTestService(upstream)
	account := forwardAsAnthropicAPIKeyTestAccount()
	promptCacheKey := "desktop-session-key"

	svc.bindOpenAICompatSessionResponseID(context.Background(), c, account, promptCacheKey, "resp_memory_prev")
	require.Equal(t, "resp_memory_prev", svc.getOpenAICompatSessionResponseID(context.Background(), c, account, promptCacheKey))

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, promptCacheKey, "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)

	input := gjson.GetBytes(upstream.lastBody, "input")
	require.Greater(t, input.Get("#").Int(), int64(3), string(upstream.lastBody))
	require.Contains(t, string(upstream.lastBody), "EARLY_USER_TURN_01")
	require.Contains(t, string(upstream.lastBody), "ASSISTANT_TURN_19")
	require.False(t, gjson.GetBytes(upstream.lastBody, "previous_response_id").Exists(), string(upstream.lastBody))
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.Contains(t, string(upstream.lastBody), "sub2api-claude-code-todo-guard")
	require.Equal(t, "resp_bound_after", svc.getOpenAICompatSessionResponseID(context.Background(), c, account, promptCacheKey))
}

func TestForwardAsAnthropic_APIKeyFullReplayKeepsToolResultAndFollowUpQuestion(t *testing.T) {
	t.Parallel()

	messages := []map[string]any{
		{"role": "user", "content": "please read the rules"},
		{
			"role": "assistant",
			"content": []map[string]any{
				{"type": "tool_use", "id": "toolu_read_clause", "name": "Read", "input": map[string]any{"file_path": "CLAUDE.md"}},
			},
		},
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "tool_result", "tool_use_id": "toolu_read_clause", "content": "条款 2/3 remain in the file"},
			},
		},
		{"role": "user", "content": "第 2、3 条怎么改"},
	}
	body := marshalAnthropicMessagesBody(t, messages)
	c, _ := newForwardAsAnthropicTestContext(body, "rid-clause-followup")
	upstream := completedUpstreamRecorder("resp_clause")
	svc := newForwardAsAnthropicTestService(upstream)
	account := forwardAsAnthropicAPIKeyTestAccount()
	promptCacheKey := "clause-session"

	svc.bindOpenAICompatSessionResponseID(context.Background(), c, account, promptCacheKey, "resp_memory_prev")

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, promptCacheKey, "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, string(upstream.lastBody), "条款 2/3")
	require.Contains(t, string(upstream.lastBody), "第 2、3 条怎么改")
	require.False(t, gjson.GetBytes(upstream.lastBody, "previous_response_id").Exists(), string(upstream.lastBody))
}

func TestForwardAsAnthropic_APIKeyDoesNotApplyTwelveMessageReplayGuard(t *testing.T) {
	t.Parallel()

	messages := make([]map[string]any, 0, 15)
	messages = append(messages, map[string]any{"role": "user", "content": "MUST_KEEP_HEAD_MESSAGE"})
	for i := 1; i < 15; i++ {
		messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprintf("tail-user-%02d", i)})
	}
	body := marshalAnthropicMessagesBody(t, messages)
	c, _ := newForwardAsAnthropicTestContext(body, "rid-no-12-window")
	upstream := completedUpstreamRecorder("resp_15")
	svc := newForwardAsAnthropicTestService(upstream)
	account := forwardAsAnthropicAPIKeyTestAccount()

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, string(upstream.lastBody), "MUST_KEEP_HEAD_MESSAGE")
	require.GreaterOrEqual(t, gjson.GetBytes(upstream.lastBody, "input.#").Int(), int64(15))
	require.False(t, gjson.GetBytes(upstream.lastBody, "previous_response_id").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
}

func TestForwardAsAnthropic_OAuthContinuationKeepsTurnStateAndFullInput(t *testing.T) {
	t.Parallel()

	messages := make([]map[string]any, 0, 15)
	messages = append(messages, map[string]any{"role": "user", "content": "OAUTH_HEAD_MUST_STAY"})
	for i := 1; i < 15; i++ {
		messages = append(messages, map[string]any{"role": "user", "content": fmt.Sprintf("oauth-user-%02d", i)})
	}
	body := marshalAnthropicMessagesBody(t, messages)
	c, _ := newForwardAsAnthropicTestContext(body, "rid-oauth-continuation")
	upstream := completedUpstreamRecorder("resp_oauth")
	svc := newForwardAsAnthropicTestService(upstream)
	account := forwardAsAnthropicOAuthTestAccount()
	promptCacheKey := "oauth-session-key"

	svc.bindOpenAICompatSessionTurnState(context.Background(), c, account, promptCacheKey, "turn_state_keep")
	svc.bindOpenAICompatSessionResponseID(context.Background(), c, account, promptCacheKey, "resp_must_not_attach")

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, promptCacheKey, "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "turn_state_keep", upstream.lastReq.Header.Get("x-codex-turn-state"))
	require.Contains(t, string(upstream.lastBody), "OAUTH_HEAD_MUST_STAY")
	require.False(t, gjson.GetBytes(upstream.lastBody, "previous_response_id").Exists(), string(upstream.lastBody))
	require.GreaterOrEqual(t, gjson.GetBytes(upstream.lastBody, "input.#").Int(), int64(15))
}

func TestAnthropicAPIKeyMustFullReplay(t *testing.T) {
	t.Parallel()
	require.True(t, anthropicAPIKeyMustFullReplay(&Account{Type: AccountTypeAPIKey}, nil, "resp_x"))
	storeTrue := true
	require.True(t, anthropicAPIKeyMustFullReplay(&Account{Type: AccountTypeAPIKey}, &storeTrue, "resp_x"))
	require.False(t, anthropicAPIKeyMustFullReplay(&Account{Type: AccountTypeOAuth}, nil, "resp_x"))
	require.False(t, anthropicAPIKeyMustFullReplay(nil, nil, ""))
}
