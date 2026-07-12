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

// compact 恢复的 context-length sentinel 渲染文本不得点名 OpenAI。
func TestBridgeCompactContextLengthErrorIsProviderNeutral(t *testing.T) {
	err := &openAICompactContextLengthError{statusCode: 400, message: "This model's maximum context length is 272000 tokens"}
	requireNoProviderBrand(t, "compact context-length error text", err.Error())
	requireNoProviderBrand(t, "compact context-length sentinel", errOpenAICompactContextLengthExceeded.Error())
}

var _ = time.Now
var _ = strings.Contains
