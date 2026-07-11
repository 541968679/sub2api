//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestListSchedulableCapacityByGroupIDs_PreservesSchedulingFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	query := regexp.QuoteMeta(`
		SELECT ag.group_id, a.id, a.concurrency,
			COALESCE(a.extra, '{}'::jsonb)::text,
			a.session_window_start, a.session_window_end,
			COALESCE(a.session_window_status, '')
		FROM account_groups ag
		JOIN accounts a ON a.id = ag.account_id
		WHERE ag.group_id = ANY($1)
			AND a.deleted_at IS NULL
			AND a.status = $2
			AND a.schedulable = TRUE
			AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= $3)
			AND (a.expires_at IS NULL OR a.expires_at > $3 OR a.auto_pause_on_expired = FALSE)
			AND (a.overload_until IS NULL OR a.overload_until <= $3)
			AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= $3)
		ORDER BY ag.group_id, ag.priority, a.priority, a.id
	`)
	now := time.Now().UTC()
	mock.ExpectQuery(query).
		WithArgs(sqlmock.AnyArg(), "active", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id", "account_id", "concurrency", "extra", "session_window_start", "session_window_end", "session_window_status",
		}).AddRow(int64(10), int64(7), 3, `{"max_sessions":2}`, now, now.Add(time.Hour), "active"))

	repo := &accountRepository{sql: db}
	rows, err := repo.ListSchedulableCapacityByGroupIDs(context.Background(), []int64{10, 10, 0, -1})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(10), rows[0].GroupID)
	require.Equal(t, int64(7), rows[0].AccountID)
	require.Equal(t, float64(2), rows[0].Extra["max_sessions"])
	require.NoError(t, mock.ExpectationsWereMet())
}
