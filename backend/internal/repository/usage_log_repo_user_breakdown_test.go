package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestGetUserBreakdownStatsPreservesBillingAndTokenComponents(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`(?s)SELECT.*SUM\(ul\.input_tokens\).*SUM\(ul\.output_tokens\).*SUM\(ul\.cache_creation_tokens\).*SUM\(ul\.cache_read_tokens\).*SUM\(ul\.total_cost\).*SUM\(ul\.actual_cost\).*account_stats_cost.*ORDER BY cache_read_tokens DESC.*LIMIT 20`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "email", "requests", "input_tokens", "output_tokens",
			"cache_creation_tokens", "cache_read_tokens", "total_tokens",
			"cost", "actual_cost", "account_cost",
		}).AddRow(int64(7), "u7@example.com", int64(3), int64(100), int64(20), int64(11), int64(13), int64(144), 1.25, 0.75, 0.5))

	rows, err := repo.GetUserBreakdownStats(context.Background(), start, end, usagestats.UserBreakdownDimension{
		SortBy: "cache_read_tokens",
	}, 20)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(100), rows[0].InputTokens)
	require.Equal(t, int64(20), rows[0].OutputTokens)
	require.Equal(t, int64(11), rows[0].CacheCreationTokens)
	require.Equal(t, int64(13), rows[0].CacheReadTokens)
	require.Equal(t, int64(144), rows[0].TotalTokens)
	require.InDelta(t, 1.25, rows[0].Cost, 0.0001)
	require.InDelta(t, 0.75, rows[0].ActualCost, 0.0001)
	require.InDelta(t, 0.5, rows[0].AccountCost, 0.0001)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserBreakdownStatsRejectsRawSortExpression(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`(?s)ORDER BY actual_cost DESC.*LIMIT 10`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "email", "requests", "input_tokens", "output_tokens",
			"cache_creation_tokens", "cache_read_tokens", "total_tokens",
			"cost", "actual_cost", "account_cost",
		}))

	_, err := repo.GetUserBreakdownStats(context.Background(), start, end, usagestats.UserBreakdownDimension{
		SortBy: "actual_cost DESC; DROP TABLE usage_logs",
	}, 10)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUserBreakdownStatsFiltersBillingMode(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`(?s)WHERE ul\.created_at >= \$1 AND ul\.created_at < \$2.*AND ul\.billing_mode = \$3.*ORDER BY actual_cost DESC`).
		WithArgs(start, end, "image").
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "email", "requests", "input_tokens", "output_tokens",
			"cache_creation_tokens", "cache_read_tokens", "total_tokens",
			"cost", "actual_cost", "account_cost",
		}))

	_, err := repo.GetUserBreakdownStats(context.Background(), start, end, usagestats.UserBreakdownDimension{BillingMode: "image"}, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
