package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userQualitySnapshotRepository struct {
	sql *sql.DB
}

// NewUserQualitySnapshotRepository persists user-global last-N quality snapshots.
func NewUserQualitySnapshotRepository(sqlDB *sql.DB) service.UserQualitySnapshotRepository {
	return &userQualitySnapshotRepository{sql: sqlDB}
}

func (r *userQualitySnapshotRepository) Upsert(ctx context.Context, row service.UserQualitySnapshotRow) error {
	if r == nil || r.sql == nil {
		return errors.New("user quality snapshot repository is not initialized")
	}
	if row.UserID <= 0 {
		return errors.New("user_id is required")
	}
	if row.CapturedAt.IsZero() {
		return errors.New("captured_at is required")
	}
	row.CapturedAt = service.TruncateToAccountQualitySnapshotTime(row.CapturedAt)
	if row.WindowSeconds <= 0 {
		row.WindowSeconds = service.AccountQualityWindowSeconds
	}

	const query = `
		INSERT INTO user_quality_snapshots (
			user_id, captured_at, window_seconds, success_count, error_count, ttft_samples,
			success_rate, avg_ttft_ms, p50_ttft_ms, p95_ttft_ms, max_ttft_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (user_id, captured_at) DO UPDATE SET
			window_seconds = EXCLUDED.window_seconds,
			success_count = EXCLUDED.success_count,
			error_count = EXCLUDED.error_count,
			ttft_samples = EXCLUDED.ttft_samples,
			success_rate = EXCLUDED.success_rate,
			avg_ttft_ms = EXCLUDED.avg_ttft_ms,
			p50_ttft_ms = EXCLUDED.p50_ttft_ms,
			p95_ttft_ms = EXCLUDED.p95_ttft_ms,
			max_ttft_ms = EXCLUDED.max_ttft_ms
	`
	_, err := r.sql.ExecContext(
		ctx,
		query,
		row.UserID,
		row.CapturedAt,
		row.WindowSeconds,
		row.SuccessCount,
		row.ErrorCount,
		row.TTFTSamples,
		row.SuccessRate,
		row.AvgTTFTMs,
		row.P50TTFTMs,
		row.P95TTFTMs,
		row.MaxTTFTMs,
	)
	return err
}

func (r *userQualitySnapshotRepository) ListByUser(ctx context.Context, userID int64, from, to time.Time) ([]service.UserQualitySnapshotRow, error) {
	if r == nil || r.sql == nil {
		return nil, errors.New("user quality snapshot repository is not initialized")
	}
	if userID <= 0 {
		return []service.UserQualitySnapshotRow{}, nil
	}

	const query = `
		SELECT user_id, captured_at, window_seconds, success_count, error_count, ttft_samples,
			success_rate, avg_ttft_ms, p50_ttft_ms, p95_ttft_ms, max_ttft_ms
		FROM user_quality_snapshots
		WHERE user_id = $1 AND captured_at >= $2 AND captured_at <= $3
		ORDER BY captured_at ASC
	`
	rows, err := r.sql.QueryContext(ctx, query, userID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.UserQualitySnapshotRow, 0)
	for rows.Next() {
		var row service.UserQualitySnapshotRow
		if err := rows.Scan(
			&row.UserID,
			&row.CapturedAt,
			&row.WindowSeconds,
			&row.SuccessCount,
			&row.ErrorCount,
			&row.TTFTSamples,
			&row.SuccessRate,
			&row.AvgTTFTMs,
			&row.P50TTFTMs,
			&row.P95TTFTMs,
			&row.MaxTTFTMs,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *userQualitySnapshotRepository) DeleteExpired(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if r == nil || r.sql == nil {
		return 0, errors.New("user quality snapshot repository is not initialized")
	}
	if limit <= 0 {
		limit = service.AccountQualitySnapshotDeleteBatchSize
	}
	const query = `
		DELETE FROM user_quality_snapshots
		WHERE id IN (
			SELECT id
			FROM user_quality_snapshots
			WHERE captured_at < $1
			ORDER BY captured_at ASC
			LIMIT $2
		)
	`
	res, err := r.sql.ExecContext(ctx, query, cutoff.UTC(), limit)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
