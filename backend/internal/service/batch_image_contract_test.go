//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchImageDomainContract(t *testing.T) {
	t.Run("lifecycle", func(t *testing.T) {
		require.True(t, CanTransitionBatchImageJob(BatchImageJobStatusCreated, BatchImageJobStatusUploading))
		require.True(t, CanTransitionBatchImageJob(BatchImageJobStatusSettling, BatchImageJobStatusCompleted))
		require.True(t, CanTransitionBatchImageJob(BatchImageJobStatusCompleted, BatchImageJobStatusOutputDeleted))
		require.False(t, CanTransitionBatchImageJob(BatchImageJobStatusCreated, BatchImageJobStatusRunning))
		require.True(t, IsTerminalBatchImageJobStatus(BatchImageJobStatusFailed))
		require.False(t, IsTerminalBatchImageJobStatus(BatchImageJobStatusRunning))
	})

	t.Run("provider_allowlist", func(t *testing.T) {
		require.True(t, IsSupportedBatchImageProvider(BatchImageProviderGeminiAPI))
		require.True(t, IsSupportedBatchImageProvider(BatchImageProviderVertex))
		require.False(t, IsSupportedBatchImageProvider("gemini_oauth"))
	})

	t.Run("public_id", func(t *testing.T) {
		id, err := NewBatchImageID()
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(id, "imgbatch_"))
		require.Len(t, id, len("imgbatch_")+32)
	})
}
