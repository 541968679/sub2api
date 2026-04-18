package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type antigravityUsageAggregator struct {
	sql *sql.DB
}

// NewAntigravityUsageAggregator 构造 antigravity 用量聚合器。仅做只读的 COUNT/SUM 查询，
// 与 UsageLogRepository 接口解耦，避免为了增加一个方法去改动所有 stub。
func NewAntigravityUsageAggregator(sqlDB *sql.DB) service.AntigravityUsageAggregator {
	return &antigravityUsageAggregator{sql: sqlDB}
}

func (r *antigravityUsageAggregator) AggregateUsage(ctx context.Context, accountIDs []int64, start, end time.Time) (int64, float64, error) {
	if len(accountIDs) == 0 {
		return 0, 0, nil
	}
	const query = `
		SELECT
			COUNT(*) AS call_count,
			COALESCE(SUM(total_cost), 0) AS total_cost
		FROM usage_logs
		WHERE account_id = ANY($1)
		  AND created_at >= $2
		  AND created_at < $3
	`
	var (
		callCount int64
		totalCost float64
	)
	row := r.sql.QueryRowContext(ctx, query, pq.Array(accountIDs), start, end)
	if err := row.Scan(&callCount, &totalCost); err != nil {
		return 0, 0, err
	}
	return callCount, totalCost, nil
}
