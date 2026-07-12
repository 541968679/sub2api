//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 用真实形态的 OpenAI 上游错误文本验证消毒器彻底清除品牌指纹。
func TestScrubProviderIdentityText_RealUpstreamMessages(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		model   string
		wantNo  []string
		wantHas []string
	}{
		{
			name:    "quota message names OpenAI account",
			in:      "Your OpenAI account has exceeded its current quota, please check your plan and billing details.",
			model:   "claude-opus-4-8",
			wantNo:  []string{"OpenAI", "openai"},
			wantHas: []string{"Your account has exceeded"},
		},
		{
			name:    "api key url leaks platform.openai.com",
			in:      "Incorrect API key provided. You can find your API key at https://platform.openai.com/account/api-keys.",
			model:   "claude-opus-4-8",
			wantNo:  []string{"openai.com", "platform.openai"},
			wantHas: []string{"the upstream endpoint"},
		},
		{
			name:    "model-not-found names gpt model",
			in:      "The model `gpt-5.5` does not exist or you do not have access to it.",
			model:   "claude-opus-4-8",
			wantNo:  []string{"gpt-5.5", "gpt-"},
			wantHas: []string{"claude-opus-4-8"},
		},
		{
			name:    "gpt model without requested model falls back to neutral",
			in:      "gpt-5.5-codex is not supported for this endpoint.",
			model:   "",
			wantNo:  []string{"gpt-5.5", "codex", "Codex"},
			wantHas: []string{"the model"},
		},
		{
			name:    "chatgpt and org id",
			in:      "ChatGPT backend error for org-Ab12Cd34Ef please retry.",
			model:   "claude-opus-4-8",
			wantNo:  []string{"ChatGPT", "chatgpt", "org-Ab12Cd34Ef"},
			wantHas: []string{"[redacted]"},
		},
		{
			name:   "context length message stays intact (no brand)",
			in:     "This model's maximum context length is 272000 tokens. However, your messages resulted in 300000 tokens.",
			model:  "claude-opus-4-8",
			wantNo: []string{"gpt", "OpenAI"},
			// gpt-* 不匹配 "This model's"，整句应保留关键信息
			wantHas: []string{"maximum context length", "272000"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ScrubProviderIdentityText(tc.in, tc.model)
			for _, no := range tc.wantNo {
				require.NotContainsf(t, got, no, "scrubbed text must not contain %q; got: %s", no, got)
			}
			for _, has := range tc.wantHas {
				require.Containsf(t, got, has, "scrubbed text must retain %q; got: %s", has, got)
			}
			// 无双空格残留
			require.NotContains(t, got, "  ", "scrubbed text must not leave double spaces: %q", got)
		})
	}
}

func TestScrubProviderIdentityText_CaseInsensitiveAllTokens(t *testing.T) {
	in := "OpenAI openai ChatGPT chatgpt Codex codex gpt-5.6-sol GPT-5.5"
	got := ScrubProviderIdentityText(in, "claude-opus-4-8")
	for _, token := range []string{"OpenAI", "openai", "ChatGPT", "chatgpt", "Codex", "codex", "gpt-5", "GPT-5"} {
		require.NotContainsf(t, got, token, "must remove %q; got: %s", token, got)
	}
	require.Contains(t, strings.ToLower(got), "claude-opus-4-8")
}

func TestScrubProviderIdentityText_EmptyAndNeutral(t *testing.T) {
	require.Equal(t, "", ScrubProviderIdentityText("", "claude-opus-4-8"))
	// 纯中性文本原样返回
	require.Equal(t, "Rate limit exceeded", ScrubProviderIdentityText("Rate limit exceeded", "claude-opus-4-8"))
}
