package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokBillingSnapshotIsSchedulerNeutral(t *testing.T) {
	t.Parallel()

	require.True(t, isSchedulerNeutralExtraKey("grok_billing_snapshot"))
	require.True(t, isSchedulerNeutralExtraKey("quality_hard_close"))
	require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		"grok_billing_snapshot": map[string]any{"usage_percent": 50},
	}))
	require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		"quality_hard_close": map[string]any{"enabled": true},
	}))
}
