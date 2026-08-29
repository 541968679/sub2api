package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// openAI7dLiteLLMPriceModelSQL prices Claude→GPT bridge rows on upstream_model.
const openAI7dLiteLLMPriceModelSQL = `COALESCE(NULLIF(TRIM(upstream_model), ''), NULLIF(TRIM(requested_model), ''), model)`

type openAI7dCycleRepository struct {
	sql *sql.DB
}

func NewOpenAI7dCycleRepository(sqlDB *sql.DB) service.OpenAI7dCycleRepository {
	return &openAI7dCycleRepository{sql: sqlDB}
}

func (r *openAI7dCycleRepository) GetAccountModelTokenTotals(ctx context.Context, accountID int64, start, end time.Time) ([]service.AccountModelTokenTotals, error) {
	if r == nil || r.sql == nil {
		return nil, errors.New("openai 7d cycle repository is not initialized")
	}
	if accountID <= 0 || !end.After(start) {
		return nil, nil
	}
	query := `
		SELECT
			` + openAI7dLiteLLMPriceModelSQL + ` AS price_model,
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cache_creation_tokens), 0),
			COALESCE(SUM(cache_creation_5m_tokens), 0),
			COALESCE(SUM(cache_creation_1h_tokens), 0),
			COALESCE(SUM(cache_read_tokens), 0),
			COALESCE(SUM(image_output_tokens), 0),
			COALESCE(SUM(image_count), 0)
		FROM usage_logs
		WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY 1
	`
	rows, err := r.sql.QueryContext(ctx, query, accountID, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []service.AccountModelTokenTotals
	for rows.Next() {
		var row service.AccountModelTokenTotals
		if err := rows.Scan(
			&row.Model,
			&row.InputTokens,
			&row.OutputTokens,
			&row.CacheCreationTokens,
			&row.CacheCreation5mTokens,
			&row.CacheCreation1hTokens,
			&row.CacheReadTokens,
			&row.ImageOutputTokens,
			&row.ImageCount,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *openAI7dCycleRepository) InsertCycle(ctx context.Context, cycle service.OpenAI7dCycle) error {
	if r == nil || r.sql == nil {
		return errors.New("openai 7d cycle repository is not initialized")
	}
	if cycle.AccountID <= 0 || cycle.WindowEnd.IsZero() {
		return errors.New("account_id and window_end are required")
	}
	const query = `
		INSERT INTO openai_oauth_7d_cycles (
			account_id, window_start, window_end, litellm_cost, used_percent, closed_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (account_id, window_end) DO NOTHING
	`
	closedAt := cycle.ClosedAt
	if closedAt.IsZero() {
		closedAt = time.Now().UTC()
	}
	_, err := r.sql.ExecContext(ctx, query,
		cycle.AccountID,
		cycle.WindowStart.UTC(),
		cycle.WindowEnd.UTC(),
		cycle.LiteLLMCost,
		cycle.UsedPercent,
		closedAt.UTC(),
	)
	return err
}

func (r *openAI7dCycleRepository) GetLatestCycle(ctx context.Context, accountID int64) (*service.OpenAI7dCycle, error) {
	if r == nil || r.sql == nil {
		return nil, errors.New("openai 7d cycle repository is not initialized")
	}
	if accountID <= 0 {
		return nil, nil
	}
	const query = `
		SELECT account_id, window_start, window_end, litellm_cost, used_percent, closed_at
		FROM openai_oauth_7d_cycles
		WHERE account_id = $1
		ORDER BY window_end DESC
		LIMIT 1
	`
	var cycle service.OpenAI7dCycle
	err := r.sql.QueryRowContext(ctx, query, accountID).Scan(
		&cycle.AccountID,
		&cycle.WindowStart,
		&cycle.WindowEnd,
		&cycle.LiteLLMCost,
		&cycle.UsedPercent,
		&cycle.ClosedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cycle, nil
}

func (r *openAI7dCycleRepository) ListCycles(ctx context.Context, accountID int64, limit int) ([]service.OpenAI7dCycle, error) {
	if r == nil || r.sql == nil {
		return nil, errors.New("openai 7d cycle repository is not initialized")
	}
	if accountID <= 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 36
	}
	const query = `
		SELECT account_id, window_start, window_end, litellm_cost, used_percent, closed_at
		FROM openai_oauth_7d_cycles
		WHERE account_id = $1
		ORDER BY window_end DESC
		LIMIT $2
	`
	rows, err := r.sql.QueryContext(ctx, query, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []service.OpenAI7dCycle
	for rows.Next() {
		var cycle service.OpenAI7dCycle
		if err := rows.Scan(
			&cycle.AccountID,
			&cycle.WindowStart,
			&cycle.WindowEnd,
			&cycle.LiteLLMCost,
			&cycle.UsedPercent,
			&cycle.ClosedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, cycle)
	}
	return out, rows.Err()
}

func (r *openAI7dCycleRepository) GetWindowAccountStats(ctx context.Context, accountID int64, start, end time.Time) (*service.OpenAI7dWindowAccountStats, error) {
	if r == nil || r.sql == nil {
		return nil, errors.New("openai 7d cycle repository is not initialized")
	}
	if accountID <= 0 || !end.After(start) {
		return &service.OpenAI7dWindowAccountStats{}, nil
	}
	// Same A$/token/U$ formula as usage_log_repo.GetAccountWindowStats, with an exclusive end.
	const query = `
		SELECT
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) as tokens,
			COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as cost,
			COALESCE(SUM(actual_cost), 0) as user_cost
		FROM usage_logs
		WHERE account_id = $1 AND created_at >= $2 AND created_at < $3
	`
	stats := &service.OpenAI7dWindowAccountStats{}
	err := r.sql.QueryRowContext(ctx, query, accountID, start.UTC(), end.UTC()).Scan(
		&stats.Requests,
		&stats.Tokens,
		&stats.AccountCost,
		&stats.UserCost,
	)
	if err != nil {
		return nil, err
	}
	return stats, nil
}
