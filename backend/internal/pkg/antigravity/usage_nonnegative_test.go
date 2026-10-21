package antigravity

import (
	"strings"
	"testing"
)

func TestGeminiUsageMappingMissingPromptTokenCountKeepsInputNonNegative(t *testing.T) {
	const geminiBody = `{"candidates":[{"content":{"parts":[{"text":"hi"}],"role":"model"},"finishReason":"STOP"}],` +
		`"usageMetadata":{"cachedContentTokenCount":30}}`

	t.Run("non-stream", func(t *testing.T) {
		_, usage, err := TransformGeminiToClaude([]byte(geminiBody), "gemini-3.1-pro-preview")
		if err != nil {
			t.Fatal(err)
		}
		if usage == nil {
			t.Fatal("expected usage")
		}
		if usage.InputTokens != 0 || usage.CacheReadInputTokens != 30 {
			t.Fatalf("usage = %+v", usage)
		}
	})

	t.Run("stream", func(t *testing.T) {
		p := NewStreamingProcessor("gemini-3.1-pro-preview")
		out := p.ProcessLine(`data: {"response":` + geminiBody + `}`)
		if strings.Contains(string(out), `"input_tokens":-`) {
			t.Fatalf("message_start input_tokens went negative: %s", out)
		}
		_, usage := p.Finish()
		if usage == nil || usage.InputTokens != 0 || usage.CacheReadInputTokens != 30 {
			t.Fatalf("usage = %+v", usage)
		}
	})
}
