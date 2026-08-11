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

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLogClaudeGPTBridgePromptTooLong_EmitsInfoLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/antigravity/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.226 (external, sdk-ts)")
	c.Set("api_key", &APIKey{ID: 459, UserID: 305})
	setClaudeGPTBridgeObs(c, claudeGPTBridgeObs{
		SessionKey:    "session-abc",
		OriginalModel: "claude-opus-5",
		BillingModel:  "gpt-5.6-sol",
		UpstreamModel: "gpt-5.6-sol",
		BodyBytes:     12345,
		MessageCount:  40,
		ClientStream:  true,
		BridgeMode:    true,
		RequestPath:   "/antigravity/v1/messages",
		UserAgent:     "claude-cli/2.1.226 (external, sdk-ts)",
	})
	account := &Account{ID: 1689, Name: "acct", Type: AccountTypeOAuth, Platform: PlatformOpenAI}

	svc := &OpenAIGatewayService{}
	err := svc.newClaudeCodePromptTooLongError(c, account, "rid-ptl-1", "http_error")
	require.NotNil(t, err)

	require.True(t, logSink.ContainsMessageAtLevel(claudeGPTBridgePromptTooLongMsg, "info"))
	require.True(t, logSink.ContainsFieldValue("source", "http_error"))
	require.True(t, logSink.ContainsFieldValue("api_key_id", "459"))
	require.True(t, logSink.ContainsFieldValue("user_id", "305"))
	require.True(t, logSink.ContainsFieldValue("session_key_sha256", hashSensitiveValueForLog("session-abc")))
	require.True(t, logSink.ContainsFieldValue("status_code", "413"))
}

func TestForwardAsAnthropic_CompactDetectedAndSucceededLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	compactPrompt := testClaudeCodeCompactPrompt()
	body := []byte(fmt.Sprintf(
		`{"model":"claude-opus-4-8","max_tokens":2048,"stream":false,"messages":[{"role":"user","content":"old"},{"role":"user","content":[{"type":"text","text":%q}]}]}`,
		compactPrompt,
	))
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.226")
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)
	c.Set("api_key", &APIKey{ID: 10, UserID: 20})

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		compactCompletedSSE("resp_compact_ok", "gpt-5.5", "compact summary ok", 100, 40),
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "sess-compact-1", "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "compact summary ok")

	require.True(t, logSink.ContainsMessageAtLevel(claudeGPTBridgeCompactDetectedMsg, "info"))
	require.True(t, logSink.ContainsMessageAtLevel(claudeGPTBridgeCompactSucceededMsg, "info"))
	require.True(t, logSink.ContainsFieldValue("session_key_sha256", hashSensitiveValueForLog("sess-compact-1")))
	require.True(t, logSink.ContainsFieldValue("recovery_used", "false"))
	require.False(t, logSink.ContainsMessage(claudeGPTBridgeCompactFailedMsg))
}

func TestForwardAsAnthropic_CompactTransportFailureLogsFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	compactPrompt := testClaudeCodeCompactPrompt()
	body := []byte(fmt.Sprintf(
		`{"model":"claude-opus-4-8","max_tokens":2048,"stream":false,"messages":[{"role":"user","content":[{"type":"text","text":%q}]}]}`,
		compactPrompt,
	))
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)
	c.Set("api_key", &APIKey{ID: 11, UserID: 21})

	upstream := &httpUpstreamSequenceRecorder{
		errs: []error{fmt.Errorf("connection reset by peer")},
	}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "sess-compact-fail", "gpt-5.5")
	require.Error(t, err)
	require.Nil(t, result)

	require.True(t, logSink.ContainsMessageAtLevel(claudeGPTBridgeCompactDetectedMsg, "info"))
	require.True(t, logSink.ContainsMessageAtLevel(claudeGPTBridgeCompactFailedMsg, "warn"))
}

func TestForwardAsAnthropic_PromptTooLongLogsLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-opus-5","max_tokens":16,"stream":false,"messages":[{"role":"user","content":"hello long"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/antigravity/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.226 (external, sdk-ts)")
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)
	c.Set("api_key", &APIKey{ID: 459, UserID: 305})

	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-ptl"}},
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"Your input exceeds the context window of this model."}}`,
			)),
		},
	}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	account := rawChatCompletionsTestAccount()

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "sess-ptl", "gpt-5.5")
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)

	require.True(t, logSink.ContainsMessageAtLevel(claudeGPTBridgePromptTooLongMsg, "info"))
	require.True(t, logSink.ContainsFieldValue("source", "http_error"))
	require.True(t, logSink.ContainsFieldValue("session_key_sha256", hashSensitiveValueForLog("sess-ptl")))
	require.False(t, logSink.ContainsMessage(claudeGPTBridgeCompactDetectedMsg))
}

func TestMaybeLogClaudeGPTBridgeCompactUnrecognized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	setClaudeGPTBridgeObs(c, claudeGPTBridgeObs{SessionKey: "s1", BridgeMode: true})

	// Partial marker only — not a full Claude Code compact prompt.
	content, err := json.Marshal("Your task is to create a detailed summary of the conversation so far.")
	require.NoError(t, err)
	req := &apicompat.AnthropicRequest{Messages: []apicompat.AnthropicMessage{
		{Role: "user", Content: content},
	}}
	maybeLogClaudeGPTBridgeCompactUnrecognized(c, &Account{ID: 1}, req)

	require.True(t, logSink.ContainsMessageAtLevel(claudeGPTBridgeCompactUnrecognizedMsg, "info"))
}

func TestClassifyClaudeGPTBridgeCompactFailure(t *testing.T) {
	reason, code := classifyClaudeGPTBridgeCompactFailure(
		&UpstreamFailoverError{StatusCode: 502, ResponseBody: []byte(`{"error":{"message":"Upstream compact stream could not be read to completion"}}`)},
	)
	require.Equal(t, "stream_incomplete", reason)
	require.Equal(t, 502, code)

	reason, _ = classifyClaudeGPTBridgeCompactFailure(
		&openAICompactContextLengthError{message: "still exceeds the context window"},
	)
	require.Equal(t, "upstream_context", reason)
}
