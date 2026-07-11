//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractOpenAIReasoningEffortFromBodyModelCandidates(t *testing.T) {
	t.Run("mapped GPT-5.6 preserves explicit max", func(t *testing.T) {
		got := extractOpenAIReasoningEffortFromBody(
			[]byte(`{"model":"sol","reasoning":{"effort":"max"}}`),
			"gpt-5.6-sol", "sol",
		)
		require.NotNil(t, got)
		require.Equal(t, "max", *got)
	})

	t.Run("original suffix survives upstream normalization", func(t *testing.T) {
		got := extractOpenAIReasoningEffortFromBody(
			[]byte(`{"model":"gpt-5.4-xhigh"}`),
			"gpt-5.4", "gpt-5.4", "gpt-5.4-xhigh",
		)
		require.NotNil(t, got)
		require.Equal(t, "xhigh", *got)
	})

	t.Run("non GPT-5.6 max normalizes to xhigh", func(t *testing.T) {
		got := extractOpenAIReasoningEffortFromBody(
			[]byte(`{"reasoning":{"effort":"max"}}`),
			"gpt-5.4",
		)
		require.NotNil(t, got)
		require.Equal(t, "xhigh", *got)
	})
}
