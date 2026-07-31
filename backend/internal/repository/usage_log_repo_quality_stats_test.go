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

	usageRows := sqlmock.NewRows([]string{"account_id", "success_count", "ttft_samples", "avg_ttft_ms"}).
		AddRow(int64(10), int64(8), int64(7), 321.6)
	mock.ExpectQuery(`FROM usage_logs`).
		WithArgs(sqlmock.AnyArg(), start).
		WillReturnRows(usageRows)

	errorRows := sqlmock.NewRows([]string{"account_id", "error_count"}).
		AddRow(int64(10), int64(2)).
		AddRow(int64(20), int64(1))
	mock.ExpectQuery(`FROM ops_error_logs`).
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
	require.Equal(t, int64(7), acc10.TTFTSamples)

	acc20 := got[20]
	require.NotNil(t, acc20)
	require.Equal(t, int64(0), acc20.SuccessCount)
	require.Equal(t, int64(1), acc20.ErrorCount)
	require.NotNil(t, acc20.SuccessRate)
	require.InDelta(t, 0.0, *acc20.SuccessRate, 1e-9)
	require.Nil(t, acc20.AvgTTFTMs)

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
