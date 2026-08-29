//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrimOpenAIEncryptedReasoningItems(t *testing.T) {
	t.Parallel()

	t.Run("drops encrypted compaction and keeps following message", func(t *testing.T) {
		t.Parallel()
		reqBody := map[string]any{
			"input": []any{
				map[string]any{
					"type":              "compaction",
					"id":                "cmp_stale",
					"encrypted_content": "gAAA",
				},
				map[string]any{
					"type": "message",
					"role": "user",
					"content": []any{
						map[string]any{"type": "input_text", "text": "hello"},
					},
				},
			},
		}

		require.True(t, trimOpenAIEncryptedReasoningItems(reqBody))
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		kept, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "message", kept["type"])
		require.Equal(t, "user", kept["role"])
	})

	t.Run("drops encrypted compaction_summary", func(t *testing.T) {
		t.Parallel()
		reqBody := map[string]any{
			"input": []any{
				map[string]any{
					"type":              "compaction_summary",
					"id":                "cmp_summary",
					"encrypted_content": "gAAA",
				},
				map[string]any{"type": "input_text", "text": "hello"},
			},
		}

		require.True(t, trimOpenAIEncryptedReasoningItems(reqBody))
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		kept, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "input_text", kept["type"])
		require.Equal(t, "hello", kept["text"])
	})

	t.Run("leaves unencrypted compaction unchanged", func(t *testing.T) {
		t.Parallel()
		compaction := map[string]any{
			"type":    "compaction",
			"id":      "cmp_plain",
			"summary": "keep me",
		}
		reqBody := map[string]any{
			"input": []any{compaction},
		}

		require.False(t, trimOpenAIEncryptedReasoningItems(reqBody))
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		kept, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "compaction", kept["type"])
		require.Equal(t, "cmp_plain", kept["id"])
		require.Equal(t, "keep me", kept["summary"])
		_, hasEncrypted := kept["encrypted_content"]
		require.False(t, hasEncrypted)
	})

	t.Run("context_compaction with encrypted_content does not trim", func(t *testing.T) {
		t.Parallel()
		reqBody := map[string]any{
			"input": []any{
				map[string]any{
					"type":              "context_compaction",
					"id":                "ctx_cmp",
					"encrypted_content": "gAAA",
				},
			},
		}

		require.False(t, trimOpenAIEncryptedReasoningItems(reqBody))
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		kept, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "context_compaction", kept["type"])
		require.Equal(t, "gAAA", kept["encrypted_content"])
	})

	t.Run("reasoning still strips encrypted_content and keeps summary", func(t *testing.T) {
		t.Parallel()
		reqBody := map[string]any{
			"input": []any{
				map[string]any{
					"type":              "reasoning",
					"id":                "rs_keep",
					"encrypted_content": "gAAA",
					"summary": []any{
						map[string]any{"type": "summary_text", "text": "keep me"},
					},
				},
				map[string]any{"type": "input_text", "text": "hello"},
			},
		}

		require.True(t, trimOpenAIEncryptedReasoningItems(reqBody))
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 2)
		kept, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "reasoning", kept["type"])
		require.Equal(t, "rs_keep", kept["id"])
		_, hasEncrypted := kept["encrypted_content"]
		require.False(t, hasEncrypted)
		following, ok := input[1].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "input_text", following["type"])
	})

	t.Run("leaves unencrypted compaction_summary unchanged", func(t *testing.T) {
		t.Parallel()
		reqBody := map[string]any{
			"input": []any{
				map[string]any{
					"type":    "compaction_summary",
					"id":      "cmp_plain_summary",
					"summary": "keep summary",
				},
			},
		}

		require.False(t, trimOpenAIEncryptedReasoningItems(reqBody))
		input, ok := reqBody["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		kept, ok := input[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "compaction_summary", kept["type"])
		require.Equal(t, "cmp_plain_summary", kept["id"])
		require.Equal(t, "keep summary", kept["summary"])
	})
}
