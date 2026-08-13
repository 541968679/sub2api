package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepository_GetAccountQualityStatsBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newUsageLogRepositoryWithSQL(nil, db)
	start := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	ids := []int64{10, 20}

	usageRows := sqlmock.NewRows([]string{
		"account_id", "success_count", "ttft_samples", "avg_ttft_ms", "p50_ttft_ms", "p95_ttft_ms", "max_ttft_ms",
	}).AddRow(int64(10), int64(8), int64(7), 321.6, 280.0, 900.4, 5000.0)
	mock.ExpectQuery(`SELECT\s+account_id[\s\S]*FROM usage_logs[\s\S]*WHERE account_id = ANY`).
		WithArgs(sqlmock.AnyArg(), start).
		WillReturnRows(usageRows)

	errorRows := sqlmock.NewRows([]string{"account_id", "error_count"}).
		AddRow(int64(10), int64(2)).
		AddRow(int64(20), int64(1))
	// Shared helper must keep account filters: account_id grouping, no user_id NULL guard.
	mock.ExpectQuery(`SELECT\s+account_id[\s\S]*FROM ops_error_logs[\s\S]*WHERE account_id = ANY[\s\S]*AND COALESCE\(status_code, 0\) >= 400[\s\S]*AND is_count_tokens = FALSE\s+GROUP BY account_id`).
		WithArgs(sqlmock.AnyArg(), start).
		WillReturnRows(errorRows)

	got, err := repo.GetAccountQualityStatsBatch(context.Background(), ids, start)
	require.NoError(t, err)
	require.Len(t, got, 2)

	acc10 := got[10]
	require.NotNil(t, acc10)
	require.Equal(t, int64(8), acc10.SuccessCount)
	require.Equal(t, int64(2), acc10.ErrorCount)
	require.NotNil(t, acc10.SuccessRate)
	require.InDelta(t, 0.8, *acc10.SuccessRate, 1e-9)
	require.NotNil(t, acc10.AvgTTFTMs)
	require.Equal(t, 322, *acc10.AvgTTFTMs)
	require.NotNil(t, acc10.P50TTFTMs)
	require.Equal(t, 280, *acc10.P50TTFTMs)
	require.NotNil(t, acc10.P95TTFTMs)
	require.Equal(t, 900, *acc10.P95TTFTMs)
	require.NotNil(t, acc10.MaxTTFTMs)
	require.Equal(t, 5000, *acc10.MaxTTFTMs)
	require.Equal(t, int64(7), acc10.TTFTSamples)

	acc20 := got[20]
	require.NotNil(t, acc20)
	require.Equal(t, int64(0), acc20.SuccessCount)
	require.Equal(t, int64(1), acc20.ErrorCount)
	require.NotNil(t, acc20.SuccessRate)
	require.InDelta(t, 0.0, *acc20.SuccessRate, 1e-9)
	require.Nil(t, acc20.AvgTTFTMs)
	require.Nil(t, acc20.P50TTFTMs)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepository_GetAccountQualityStatsBatch_EmptyIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newUsageLogRepositoryWithSQL(nil, db)
	got, err := repo.GetAccountQualityStatsBatch(context.Background(), nil, time.Now())
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepository_GetUserQualityStatsBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newUsageLogRepositoryWithSQL(nil, db)
	start := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	ids := []int64{10, 20}

	usageRows := sqlmock.NewRows([]string{
		"user_id", "success_count", "ttft_samples", "avg_ttft_ms", "p50_ttft_ms", "p95_ttft_ms", "max_ttft_ms",
	}).AddRow(int64(10), int64(8), int64(7), 321.6, 280.0, 900.4, 5000.0)
	mock.ExpectQuery(`SELECT\s+user_id[\s\S]*FROM usage_logs[\s\S]*WHERE user_id = ANY`).
		WithArgs(sqlmock.AnyArg(), start).
		WillReturnRows(usageRows)

	errorRows := sqlmock.NewRows([]string{"user_id", "error_count"}).
		AddRow(int64(10), int64(2)).
		AddRow(int64(20), int64(1))
	mock.ExpectQuery(`SELECT\s+user_id[\s\S]*FROM ops_error_logs[\s\S]*WHERE user_id = ANY[\s\S]*user_id IS NOT NULL[\s\S]*GROUP BY user_id`).
		WithArgs(sqlmock.AnyArg(), start).
		WillReturnRows(errorRows)

	got, err := repo.GetUserQualityStatsBatch(context.Background(), ids, start)
	require.NoError(t, err)
	require.Len(t, got, 2)

	user10 := got[10]
	require.NotNil(t, user10)
	require.Equal(t, int64(8), user10.SuccessCount)
	require.Equal(t, int64(2), user10.ErrorCount)
	require.NotNil(t, user10.SuccessRate)
	require.InDelta(t, 0.8, *user10.SuccessRate, 1e-9)
	require.NotNil(t, user10.AvgTTFTMs)
	require.Equal(t, 322, *user10.AvgTTFTMs)
	require.NotNil(t, user10.P50TTFTMs)
	require.Equal(t, 280, *user10.P50TTFTMs)
	require.NotNil(t, user10.P95TTFTMs)
	require.Equal(t, 900, *user10.P95TTFTMs)
	require.NotNil(t, user10.MaxTTFTMs)
	require.Equal(t, 5000, *user10.MaxTTFTMs)
	require.Equal(t, int64(7), user10.TTFTSamples)

	user20 := got[20]
	require.NotNil(t, user20)
	require.Equal(t, int64(0), user20.SuccessCount)
	require.Equal(t, int64(1), user20.ErrorCount)
	require.NotNil(t, user20.SuccessRate)
	require.InDelta(t, 0.0, *user20.SuccessRate, 1e-9)
	require.Nil(t, user20.AvgTTFTMs)
	require.Nil(t, user20.P50TTFTMs)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepository_GetUserQualityStatsBatch_EmptyIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := newUsageLogRepositoryWithSQL(nil, db)
	got, err := repo.GetUserQualityStatsBatch(context.Background(), nil, time.Now())
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
