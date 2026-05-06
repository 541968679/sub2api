package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
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

func (r *antigravityUsageAggregator) AggregateUsage(ctx context.Context, start, end time.Time) (int64, float64, error) {
	const query = `
		SELECT
			COUNT(*) AS call_count,
			COALESCE(SUM(u.total_cost), 0) AS total_cost
		FROM usage_logs u
		JOIN accounts a ON a.id = u.account_id
		WHERE a.platform = $1
		  AND u.created_at >= $2
		  AND u.created_at < $3
	`
	var (
		callCount int64
		totalCost float64
	)
	row := r.sql.QueryRowContext(ctx, query, service.PlatformAntigravity, start, end)
	if err := row.Scan(&callCount, &totalCost); err != nil {
		return 0, 0, err
	}
	return callCount, totalCost, nil
}

func (r *antigravityUsageAggregator) AggregateUsageWindows(ctx context.Context, start, end time.Time, granularity string) ([]service.AntigravityUsageWindow, error) {
	trunc := "hour"
	if granularity == "day" {
		trunc = "day"
	}
	tzName := timezone.Name()
	query := `
		SELECT
			date_trunc($4, u.created_at AT TIME ZONE $5) AT TIME ZONE $5 AS bucket,
			COUNT(*) AS call_count,
			COALESCE(SUM(
				COALESCE(u.input_tokens, 0) +
				COALESCE(u.output_tokens, 0) +
				COALESCE(u.cache_creation_tokens, 0) +
				COALESCE(u.cache_read_tokens, 0) +
				COALESCE(u.image_output_tokens, 0)
			), 0) AS total_tokens,
			COALESCE(SUM(u.total_cost), 0) AS quota_cost,
			COALESCE(SUM(u.actual_cost), 0) AS actual_cost
		FROM usage_logs u
		JOIN accounts a ON a.id = u.account_id
		WHERE a.platform = $1
		  AND u.created_at >= $2
		  AND u.created_at < $3
		GROUP BY 1
		ORDER BY 1
	`
	rows, err := r.sql.QueryContext(ctx, query, service.PlatformAntigravity, start, end, trunc, tzName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	windows := make([]service.AntigravityUsageWindow, 0)
	for rows.Next() {
		var w service.AntigravityUsageWindow
		if err := rows.Scan(&w.Start, &w.CallCount, &w.TotalTokens, &w.QuotaUsed, &w.ActualCost); err != nil {
			return nil, err
		}
		if granularity == "day" {
			w.End = w.Start.AddDate(0, 0, 1)
		} else {
			w.End = w.Start.Add(time.Hour)
		}
		windows = append(windows, w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return windows, nil
}
