//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeClientRelayPayload_LeavesKimiShapeUntouched(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-kimi","object":"chat.completion","model":"kimi-k3","choices":[{"message":{"role":"assistant","content":"see https://kimi.example/doc","reasoning_content":"think","tool_calls":[{"id":"call_keep","type":"function","function":{"name":"run","arguments":"{}"}}]}}],"usage":{"prompt_tokens":3,"completion_tokens":1,"prompt_cache_hit_tokens":2}}`)
	got := sanitizeClientRelayPayload(body)
	require.Equal(t, string(body), string(got))
}

func TestSanitizeClientRelayPayload_DropsRelayExtrasOnly(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-kimi","object":"chat.completion","model":"kimi-k3","provider":"moonshot","credit":1,"elapsed_ms":12,"billing_usage":{"source":"oai_chat"},"choices":[{"message":{"content":"ok","tool_calls":[{"id":"call_keep"}]}}],"usage":{"prompt_tokens":3,"prompt_cache_hit_tokens":2,"credit_usage":9}}`)
	got := sanitizeClientRelayPayload(body)
	require.False(t, gjson.GetBytes(got, "provider").Exists())
	require.False(t, gjson.GetBytes(got, "credit").Exists())
	require.False(t, gjson.GetBytes(got, "elapsed_ms").Exists())
	require.False(t, gjson.GetBytes(got, "billing_usage").Exists())
	require.False(t, gjson.GetBytes(got, "usage.credit_usage").Exists())
	require.Equal(t, "kimi-k3", gjson.GetBytes(got, "model").String())
	require.Equal(t, "ok", gjson.GetBytes(got, "choices.0.message.content").String())
	require.Equal(t, "call_keep", gjson.GetBytes(got, "choices.0.message.tool_calls.0.id").String())
	require.Equal(t, int64(2), gjson.GetBytes(got, "usage.prompt_cache_hit_tokens").Int())
	require.Equal(t, "chatcmpl-kimi", gjson.GetBytes(got, "id").String())
}

func TestSanitizeClientRelayPayload_KeepsResponsesIDAndRewritesBedrockAndErrorIDs(t *testing.T) {
	resp := []byte(`{"id":"resp_keep","object":"response","model":"gpt-5.4","output":[{"type":"message","content":[{"type":"output_text","text":"ok"}]}]}`)
	require.Equal(t, "resp_keep", gjson.GetBytes(sanitizeClientRelayPayload(resp), "id").String())

	msg := []byte(`{"id":"msg_bdrk_ABCDEF","type":"message","model":"claude-opus","content":[{"type":"text","text":"ok"}]}`)
	got := sanitizeClientRelayPayload(msg)
	require.Equal(t, "msg_ABCDEF", gjson.GetBytes(got, "id").String())
	require.NotContains(t, string(got), "bdrk")
	require.Equal(t, "ok", gjson.GetBytes(got, "content.0.text").String())

	errBody := []byte(`{"id":"chatcmpl-quark-err","error":{"message":"see https://upstream.example/v1 and group \"Kimi0.2\" account \"ops@example.com\""}}`)
	masked := sanitizeClientRelayPayload(errBody)
	require.True(t, strings.HasPrefix(gjson.GetBytes(masked, "id").String(), "local-"))
	require.NotContains(t, string(masked), "quark")
	require.NotContains(t, gjson.GetBytes(masked, "error.message").String(), "https://")
	require.NotContains(t, gjson.GetBytes(masked, "error.message").String(), "Kimi0.2")
	require.Contains(t, gjson.GetBytes(masked, "error.message").String(), "see and group account")
}

func TestScrubClientErrorDisclosure_RemovesURLAndQuotedNames(t *testing.T) {
	got := ScrubClientErrorDisclosure(`failed at https://api.upstream.example/v1/chat group "Kimi0.2" account "ops@example.com" account is not active`)
	require.NotContains(t, got, "https://")
	require.NotContains(t, got, "upstream.example")
	require.NotContains(t, got, "Kimi0.2")
	require.NotContains(t, got, "ops@example.com")
	require.Contains(t, got, "account is not active")
	require.Contains(t, got, "group")
}
