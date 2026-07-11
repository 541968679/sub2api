package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSparkShadowMigrationsUseReservedNumbersAndConstraints(t *testing.T) {
	schema, err := FS.ReadFile("188_account_spark_shadow.sql")
	require.NoError(t, err)
	sql := string(schema)
	for _, fragment := range []string{
		"parent_account_id BIGINT",
		"quota_dimension VARCHAR(20)",
		"chk_accounts_quota_dimension",
		"chk_accounts_parent_dimension",
		"chk_accounts_parent_not_self",
		"fk_accounts_parent_account_id",
		"ON DELETE RESTRICT",
	} {
		require.Contains(t, sql, fragment)
	}
	require.NotContains(t, strings.ToUpper(sql), "CONCURRENTLY")

	indexes, err := FS.ReadFile("189_account_spark_shadow_indexes_notx.sql")
	require.NoError(t, err)
	indexSQL := string(indexes)
	require.Contains(t, indexSQL, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_parent_account_id")
	require.Contains(t, indexSQL, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_accounts_spark_shadow_per_parent")
	require.Contains(t, indexSQL, "deleted_at IS NULL")
}
