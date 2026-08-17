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

func TestSumSchedulePnlByAccount_IgnoresNullTrueCost(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`(?s)SELECT.*account_id.*true_cost IS NOT NULL.*GROUP BY account_id`).
		WithArgs(int64(7), sqlmock.AnyArg(), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "revenue", "cost", "rows"}).
			AddRow(int64(1), 1.2, 0.3, int64(2)))

	out, err := repo.SumSchedulePnlByAccount(context.Background(), 7, []int64{1, 2}, start, end)
	require.NoError(t, err)
	require.Equal(t, 1.2, out[1].Revenue)
	require.Equal(t, 0.3, out[1].Cost)
	_, hasEmpty := out[2]
	require.False(t, hasEmpty, "accounts with only NULL true_cost must be absent")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSumSchedulePnlByUserPairs_JoinsExactPairs(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)

	mock.ExpectQuery(`(?s)UNNEST.*INNER JOIN pairs.*true_cost IS NOT NULL`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "revenue", "cost", "rows"}).
			AddRow(int64(10), 4.0, 1.0, int64(3)))

	out, err := repo.SumSchedulePnlByUserPairs(context.Background(), []service.SchedulePnlUserAccount{
		{UserID: 10, AccountID: 11},
		{UserID: 10, AccountID: 12},
	}, start, end)
	require.NoError(t, err)
	require.Equal(t, 4.0, out[10].Revenue)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListSchedulePnlTrend_FiltersTrueCost(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	start := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	mock.ExpectQuery(`(?s)TO_CHAR.*true_cost IS NOT NULL.*GROUP BY 1`).
		WithArgs(int64(7), sqlmock.AnyArg(), start, end, "UTC", "YYYY-MM-DD HH24:00").
		WillReturnRows(sqlmock.NewRows([]string{"bucket", "revenue", "cost", "rows"}).
			AddRow("2026-08-17 10:00", 2.0, 0.5, int64(1)))

	out, err := repo.ListSchedulePnlTrend(context.Background(), 7, []int64{5}, start, end, "hour", time.UTC)
	require.NoError(t, err)
	require.Equal(t, 2.0, out["2026-08-17 10:00"].Revenue)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSchedulePnlPostgresTimeZone_RejectsLocal(t *testing.T) {
	require.Equal(t, "UTC", schedulePnlPostgresTimeZone(nil))
	require.Equal(t, "UTC", schedulePnlPostgresTimeZone(time.Local))
	require.Equal(t, "Asia/Shanghai", schedulePnlPostgresTimeZone(mustLoadLocation(t, "Asia/Shanghai")))
}

func TestListSchedulePnlTrend_EmptyAccountsNoQuery(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	out, err := repo.ListSchedulePnlTrend(context.Background(), 7, nil, time.Now(), time.Now(), "hour", time.UTC)
	require.NoError(t, err)
	require.Empty(t, out)
	require.NoError(t, mock.ExpectationsWereMet())
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}
