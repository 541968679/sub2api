package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractImageInputTokens_FromUsageDetails(t *testing.T) {
	raw := []byte(`{"usage":{"input_tokens":100,"output_tokens":10,"input_tokens_details":{"cached_tokens":20,"image_tokens":15}}}`)
	require.Equal(t, 15, ExtractImageInputTokens(raw))
}

func TestExtractImageInputTokens_MissingIsZero(t *testing.T) {
	require.Equal(t, 0, ExtractImageInputTokens([]byte(`{"usage":{"input_tokens":100}}`)))
	require.Equal(t, 0, ExtractImageInputTokens(nil))
}

func TestApplyImageInputTokensToUsageLog_DoesNotMutateCostOrCacheRead(t *testing.T) {
	log := &UsageLog{
		InputTokens:     100,
		CacheReadTokens: 20,
		ActualCost:      1.5,
		TotalCost:       1.5,
	}
	raw := []byte(`{"usage":{"input_tokens":100,"input_tokens_details":{"image_tokens":12,"cached_tokens":20}}}`)
	ApplyImageInputTokensToUsageLog(log, raw)
	require.Equal(t, 12, log.ImageInputTokens)
	require.Equal(t, 100, log.InputTokens)
	require.Equal(t, 20, log.CacheReadTokens)
	require.Equal(t, 1.5, log.ActualCost)
	require.Equal(t, 1.5, log.TotalCost)
}
