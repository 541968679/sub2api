//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountQualitySnapshotRepository_ListRecentTrafficAccountIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &accountQualitySnapshotRepository{sql: db}
	start := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

	rows := sqlmock.NewRows([]string{"account_id"}).
		AddRow(int64(10)).
		AddRow(int64(20))
	mock.ExpectQuery(`SELECT DISTINCT account_id[\s\S]*FROM usage_logs[\s\S]*UNION[\s\S]*FROM ops_error_logs[\s\S]*COALESCE\(status_code, 0\) >= 400[\s\S]*is_count_tokens = FALSE[\s\S]*account_id IS NOT NULL`).
		WithArgs(start).
		WillReturnRows(rows)

	got, err := repo.ListRecentTrafficAccountIDs(context.Background(), start)
	require.NoError(t, err)
	require.Equal(t, []int64{10, 20}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountQualitySnapshotRepository_DeleteExpired(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &accountQualitySnapshotRepository{sql: db}
	cutoff := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

	mock.ExpectExec(`DELETE FROM account_quality_snapshots[\s\S]*captured_at < \$1[\s\S]*LIMIT \$2`).
		WithArgs(cutoff, service.AccountQualitySnapshotDeleteBatchSize).
		WillReturnResult(sqlmock.NewResult(0, 3))

	deleted, err := repo.DeleteExpired(context.Background(), cutoff, 0)
	require.NoError(t, err)
	require.Equal(t, int64(3), deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}
