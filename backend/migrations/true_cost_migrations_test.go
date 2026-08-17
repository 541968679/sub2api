package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrueCostMigrationsDoNotLockUsageLogsWrites(t *testing.T) {
	addColumn, err := FS.ReadFile("205_usage_log_true_cost.sql")
	require.NoError(t, err)
	require.Contains(t, string(addColumn), "ADD COLUMN IF NOT EXISTS true_cost")
	require.Contains(t, string(addColumn), "ADD COLUMN IF NOT EXISTS true_cost_rate")
	var statements []string
	for _, line := range strings.Split(string(addColumn), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		statements = append(statements, trimmed)
	}
	require.NotContains(t, strings.ToUpper(strings.Join(statements, "\n")), "CREATE INDEX")

	indexSQL, err := FS.ReadFile("206_usage_log_true_cost_index_notx.sql")
	require.NoError(t, err)
	require.Contains(t, string(indexSQL), "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_true_cost_user_account_created")
	require.Contains(t, string(indexSQL), "true_cost IS NOT NULL")
}
