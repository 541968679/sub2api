//go:build unit

package service

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// 桥接契约：下游客户端只应看到 Claude/Anthropic 语义。任何 OpenAI/ChatGPT/Codex
// 品牌字样出现在客户端可达的错误体中都是提供商身份泄漏（真实用户曾收到
// "OpenAI response reached max_output_tokens..." 的 400）。这些断言锁定
// Anthropic Messages 转发路径上我们自有文案的中立性。

var providerBrandTokens = []string{"OpenAI", "openai", "ChatGPT", "chatgpt", "Codex"}

func requireNoProviderBrand(t *testing.T, where, content string) {
	t.Helper()
	for _, token := range providerBrandTokens {
		require.NotContainsf(t, content, token,
			"%s must not leak provider brand %q to a bridge client; got: %s", where, token, content)
	}
}

// 复现真实报告：上游 incomplete + max_output_tokens、无可见输出，网关返回 400
// 客户端错误。错误体不得出现 "OpenAI"。
func TestBridgeClientError_MaxOutputTokensIsProviderNeutral(t *testing.T) {
	_, err, _ := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_max","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.incomplete","response":{"id":"resp_max","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[{"type":"reasoning","summary":[]}],"usage":{"input_tokens":69229,"output_tokens":68,"total_tokens":69297}}}`,
	)
	failoverErr := requireMessagesFailoverError(t, err)
	requireNoProviderBrand(t, "max_output_tokens client error body", string(failoverErr.ResponseBody))
}

// reasoning-only 完成 → failover 错误体（"...completed without assistant content..."）。
func TestBridgeFailover_ReasoningOnlyIsProviderNeutral(t *testing.T) {
	_, err, _ := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_r","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"internal reasoning"}`,
		`{"type":"response.reasoning_summary_text.done","output_index":0}`,
		`{"type":"response.completed","response":{"id":"resp_r","status":"completed","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"internal reasoning"}]}],"usage":{"input_tokens":68328,"output_tokens":24,"total_tokens":68352}}}`,
	)
	failoverErr := requireMessagesFailoverError(t, err)
	requireNoProviderBrand(t, "reasoning-only failover body", string(failoverErr.ResponseBody))
}

// 流在没有 terminal 事件时结束 → missingTerminalErr failover 错误体。
func TestBridgeFailover_MissingTerminalIsProviderNeutral(t *testing.T) {
	_, err, _ := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_mt","model":"gpt-5.5","status":"in_progress"}}`,
	)
	failoverErr := requireMessagesFailoverError(t, err)
	requireNoProviderBrand(t, "missing-terminal failover body", string(failoverErr.ResponseBody))
}

// response.failed 无消息 → 终止失败默认文案，不得点名 OpenAI。
func TestBridgeFailover_TerminalFailedDefaultIsProviderNeutral(t *testing.T) {
	_, err, _ := handleMessagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_f","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.failed","response":{"id":"resp_f","status":"failed","output":[],"usage":{"input_tokens":100,"output_tokens":0,"total_tokens":100}}}`,
	)
	failoverErr := requireMessagesFailoverError(t, err)
	requireNoProviderBrand(t, "terminal-failed default body", string(failoverErr.ResponseBody))
}

// 共享构造器默认兜底："OpenAI stream disconnected before completion" /
// "OpenAI request failed" 不得泄漏给桥接客户端。
func TestBridgeStreamFailoverDefaultMessageIsProviderNeutral(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	_, c, _, _, account := messagesTestStream(t)
	failoverErr := svc.newOpenAIStreamFailoverError(c, account, false, "rid", nil, "")
	requireNoProviderBrand(t, "failover default message", string(failoverErr.ResponseBody))

	clientErr := svc.newOpenAIStreamClientError(c, account, "rid", 400, "invalid_request_error", "")
	requireNoProviderBrand(t, "client-error default message", string(clientErr.ResponseBody))
}

// Empty completed Anthropic conversions must not multi-account failover:
// the failure is request-shaped (same on every account).
func TestEmptyVisibleOutputError_NoAccountFailover(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	_, c, _, _, account := messagesTestStream(t)
	err := svc.newOpenAIEmptyVisibleOutputError(c, account, "rid",
		"Upstream messages stream completed without assistant content or tool output")
	require.NotNil(t, err)
	require.True(t, err.NoAccountFailover)
	require.NotEmpty(t, err.ResponseBody)
}

// compact 恢复的 context-length sentinel 渲染文本不得点名 OpenAI。
func TestBridgeCompactContextLengthErrorIsProviderNeutral(t *testing.T) {
	err := &openAICompactContextLengthError{statusCode: 400, message: "This model's maximum context length is 272000 tokens"}
	requireNoProviderBrand(t, "compact context-length error text", err.Error())
	requireNoProviderBrand(t, "compact context-length sentinel", errOpenAICompactContextLengthExceeded.Error())
}

// ---- 上游文本透传通道：原始 OpenAI 错误文本经消毒器中立化 ----

// 真实形态：上游 429 body 里点名 OpenAI + gpt-5.5，构造器必须消毒后再入
// ResponseBody（该 body 会被 handler mapAnthropicFailoverBodyError 逐字回放）。
func TestBridgeConstructorScrubsUpstreamProviderText(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	_, c, _, _, account := messagesTestStream(t)
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)
	SetBridgeScrubModel(c, "claude-opus-4-8")

	raw := "Your OpenAI account rate limit for gpt-5.5 was reached; see https://platform.openai.com/account."
	failoverErr := svc.newOpenAIStreamFailoverError(c, account, false, "rid", nil, raw)
	body := string(failoverErr.ResponseBody)
	requireNoProviderBrand(t, "failover body from raw upstream text", body)
	require.NotContains(t, body, "gpt-5.5")
	require.NotContains(t, body, "openai.com")
	require.Contains(t, body, "claude-opus-4-8", "gpt model must be rewritten to the requested Claude model")

	clientErr := svc.newOpenAIStreamClientError(c, account, "rid", 400, "invalid_request_error",
		"The model `gpt-5.5` from OpenAI is not available")
	requireNoProviderBrand(t, "client-error body from raw upstream text", string(clientErr.ResponseBody))
	require.NotContains(t, string(clientErr.ResponseBody), "gpt-5.5")
}

// 非 bridge 模式（未设 context key）：OpenAI 原生分组保留原文，消毒器不介入。
func TestNonBridgeConstructorKeepsProviderText(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	_, c, _, _, account := messagesTestStream(t)
	// 不设 bridge context key。
	raw := "Your OpenAI account rate limit was reached"
	failoverErr := svc.newOpenAIStreamFailoverError(c, account, false, "rid", nil, raw)
	require.Contains(t, string(failoverErr.ResponseBody), "OpenAI",
		"OpenAI-platform native path must keep provider wording")
}

// 流式 failed-event 在已有可见输出后写 SSE error：上游 message 必须消毒。
func TestBridgeStreamFailedEventScrubsUpstreamMessageAfterVisibleOutput(t *testing.T) {
	svc, c, rec, resp, account := messagesTestStream(t,
		`{"type":"response.created","response":{"id":"resp_v","model":"gpt-5.5","status":"in_progress"}}`,
		`{"type":"response.output_text.delta","output_index":0,"delta":"partial"}`,
		`{"type":"response.failed","response":{"id":"resp_v","status":"failed","error":{"code":"server_error","message":"OpenAI gpt-5.5 backend crashed, see https://chatgpt.com/status"},"output":[]}}`,
	)
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)
	SetBridgeScrubModel(c, "claude-opus-4-8")

	_, _ = svc.handleAnthropicStreamingResponse(resp, c, account, true,
		"claude-opus-4-8", "gpt-5.5", "gpt-5.5", time.Now())

	requireNoProviderBrand(t, "mid-stream SSE error from upstream message", rec.Body.String())
	require.NotContains(t, rec.Body.String(), "gpt-5.5")
	require.NotContains(t, rec.Body.String(), "chatgpt.com")
}

var _ = time.Now
var _ = strings.Contains
