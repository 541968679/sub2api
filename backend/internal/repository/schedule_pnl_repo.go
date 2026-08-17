package repository

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *usageLogRepository) SumSchedulePnl(ctx context.Context, userID int64, accountIDs []int64, start, end time.Time) (service.SchedulePnlAgg, error) {
	var out service.SchedulePnlAgg
	if r == nil || userID <= 0 || len(accountIDs) == 0 {
		return out, nil
	}
	query := `
		SELECT
			COALESCE(SUM(actual_cost), 0),
			COALESCE(SUM(true_cost), 0),
			COUNT(*)
		FROM usage_logs
		WHERE user_id = $1
		  AND account_id = ANY($2)
		  AND created_at >= $3
		  AND created_at < $4
		  AND true_cost IS NOT NULL
	`
	rows, err := r.sql.QueryContext(ctx, query, userID, pq.Array(accountIDs), start, end)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	if !rows.Next() {
		return out, rows.Err()
	}
	if err := rows.Scan(&out.Revenue, &out.Cost, &out.Rows); err != nil {
		return out, err
	}
	return out, rows.Err()
}

func (r *usageLogRepository) SumSchedulePnlByAccount(ctx context.Context, userID int64, accountIDs []int64, start, end time.Time) (map[int64]service.SchedulePnlAgg, error) {
	out := make(map[int64]service.SchedulePnlAgg)
	if r == nil || userID <= 0 || len(accountIDs) == 0 {
		return out, nil
	}
	query := `
		SELECT
			account_id,
			COALESCE(SUM(actual_cost), 0),
			COALESCE(SUM(true_cost), 0),
			COUNT(*)
		FROM usage_logs
		WHERE user_id = $1
		  AND account_id = ANY($2)
		  AND created_at >= $3
		  AND created_at < $4
		  AND true_cost IS NOT NULL
		GROUP BY account_id
	`
	rows, err := r.sql.QueryContext(ctx, query, userID, pq.Array(accountIDs), start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var accountID int64
		var agg service.SchedulePnlAgg
		if err := rows.Scan(&accountID, &agg.Revenue, &agg.Cost, &agg.Rows); err != nil {
			return nil, err
		}
		out[accountID] = agg
	}
	return out, rows.Err()
}

func (r *usageLogRepository) SumSchedulePnlByUserPairs(ctx context.Context, pairs []service.SchedulePnlUserAccount, start, end time.Time) (map[int64]service.SchedulePnlAgg, error) {
	out := make(map[int64]service.SchedulePnlAgg)
	if r == nil || len(pairs) == 0 {
		return out, nil
	}
	userIDs := make([]int64, len(pairs))
	accountIDs := make([]int64, len(pairs))
	for i, pair := range pairs {
		userIDs[i] = pair.UserID
		accountIDs[i] = pair.AccountID
	}
	query := `
		WITH pairs(user_id, account_id) AS (
			SELECT * FROM UNNEST($1::bigint[], $2::bigint[])
		)
		SELECT
			ul.user_id,
			COALESCE(SUM(ul.actual_cost), 0),
			COALESCE(SUM(ul.true_cost), 0),
			COUNT(*)
		FROM usage_logs ul
		INNER JOIN pairs p ON p.user_id = ul.user_id AND p.account_id = ul.account_id
		WHERE ul.created_at >= $3
		  AND ul.created_at < $4
		  AND ul.true_cost IS NOT NULL
		GROUP BY ul.user_id
	`
	rows, err := r.sql.QueryContext(ctx, query, pq.Array(userIDs), pq.Array(accountIDs), start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		var agg service.SchedulePnlAgg
		if err := rows.Scan(&userID, &agg.Revenue, &agg.Cost, &agg.Rows); err != nil {
			return nil, err
		}
		out[userID] = agg
	}
	return out, rows.Err()
}

func (r *usageLogRepository) ListSchedulePnlTrend(ctx context.Context, userID int64, accountIDs []int64, start, end time.Time, granularity string, loc *time.Location) (map[string]service.SchedulePnlAgg, error) {
	out := make(map[string]service.SchedulePnlAgg)
	if r == nil || userID <= 0 || len(accountIDs) == 0 {
		return out, nil
	}
	tzName := schedulePnlPostgresTimeZone(loc)
	dateFormat := safeDateFormat(granularity)
	query := `
		SELECT
			TO_CHAR(created_at AT TIME ZONE $5, $6),
			COALESCE(SUM(actual_cost), 0),
			COALESCE(SUM(true_cost), 0),
			COUNT(*)
		FROM usage_logs
		WHERE user_id = $1
		  AND account_id = ANY($2)
		  AND created_at >= $3
		  AND created_at < $4
		  AND true_cost IS NOT NULL
		GROUP BY 1
	`
	rows, err := r.sql.QueryContext(ctx, query, userID, pq.Array(accountIDs), start, end, tzName, dateFormat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bucket string
		var agg service.SchedulePnlAgg
		if err := rows.Scan(&bucket, &agg.Revenue, &agg.Cost, &agg.Rows); err != nil {
			return nil, err
		}
		out[bucket] = agg
	}
	return out, rows.Err()
}

// schedulePnlPostgresTimeZone maps a Go location to a name PostgreSQL AT TIME ZONE accepts.
// time.Local.String() is "Local" on Windows and is not a valid PG timezone.
func schedulePnlPostgresTimeZone(loc *time.Location) string {
	if loc == nil {
		return "UTC"
	}
	name := strings.TrimSpace(loc.String())
	if name == "" || strings.EqualFold(name, "Local") {
		return "UTC"
	}
	if _, err := time.LoadLocation(name); err != nil {
		return "UTC"
	}
	return name
}
