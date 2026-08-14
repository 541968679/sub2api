package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/accountqualitysnapshot"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountQualitySnapshotRepository struct {
	client *dbent.Client
	sql    *sql.DB
}

// NewAccountQualitySnapshotRepository persists account quality window snapshots.
func NewAccountQualitySnapshotRepository(client *dbent.Client, sqlDB *sql.DB) service.AccountQualitySnapshotRepository {
	return &accountQualitySnapshotRepository{client: client, sql: sqlDB}
}

func (r *accountQualitySnapshotRepository) Upsert(ctx context.Context, row service.AccountQualitySnapshotRow) error {
	if r == nil || r.client == nil {
		return errors.New("account quality snapshot repository is not initialized")
	}
	if row.AccountID <= 0 {
		return errors.New("account_id is required")
	}
	if row.CapturedAt.IsZero() {
		return errors.New("captured_at is required")
	}
	row.CapturedAt = service.TruncateToAccountQualitySnapshotTime(row.CapturedAt)
	if row.WindowSeconds <= 0 {
		row.WindowSeconds = service.AccountQualityWindowSeconds
	}

	client := clientFromContext(ctx, r.client)
	builder := client.AccountQualitySnapshot.Create().
		SetAccountID(row.AccountID).
		SetCapturedAt(row.CapturedAt).
		SetWindowSeconds(row.WindowSeconds).
		SetSuccessCount(row.SuccessCount).
		SetErrorCount(row.ErrorCount).
		SetTtftSamples(row.TTFTSamples).
		SetNillableSuccessRate(row.SuccessRate).
		SetNillableAvgTtftMs(row.AvgTTFTMs).
		SetNillableP50TtftMs(row.P50TTFTMs).
		SetNillableP95TtftMs(row.P95TTFTMs).
		SetNillableMaxTtftMs(row.MaxTTFTMs)

	return builder.
		OnConflict(
			entsql.ConflictColumns(accountqualitysnapshot.FieldAccountID, accountqualitysnapshot.FieldCapturedAt),
		).
		Update(func(u *dbent.AccountQualitySnapshotUpsert) {
			u.SetWindowSeconds(row.WindowSeconds)
			u.SetSuccessCount(row.SuccessCount)
			u.SetErrorCount(row.ErrorCount)
			u.SetTtftSamples(row.TTFTSamples)
			if row.SuccessRate != nil {
				u.SetSuccessRate(*row.SuccessRate)
			} else {
				u.ClearSuccessRate()
			}
			if row.AvgTTFTMs != nil {
				u.SetAvgTtftMs(*row.AvgTTFTMs)
			} else {
				u.ClearAvgTtftMs()
			}
			if row.P50TTFTMs != nil {
				u.SetP50TtftMs(*row.P50TTFTMs)
			} else {
				u.ClearP50TtftMs()
			}
			if row.P95TTFTMs != nil {
				u.SetP95TtftMs(*row.P95TTFTMs)
			} else {
				u.ClearP95TtftMs()
			}
			if row.MaxTTFTMs != nil {
				u.SetMaxTtftMs(*row.MaxTTFTMs)
			} else {
				u.ClearMaxTtftMs()
			}
		}).
		Exec(ctx)
}

func (r *accountQualitySnapshotRepository) ListByAccount(ctx context.Context, accountID int64, from, to time.Time) ([]service.AccountQualitySnapshotRow, error) {
	if r == nil || r.client == nil {
		return nil, errors.New("account quality snapshot repository is not initialized")
	}
	if accountID <= 0 {
		return []service.AccountQualitySnapshotRow{}, nil
	}

	client := clientFromContext(ctx, r.client)
	rows, err := client.AccountQualitySnapshot.Query().
		Where(
			accountqualitysnapshot.AccountIDEQ(accountID),
			accountqualitysnapshot.CapturedAtGTE(from.UTC()),
			accountqualitysnapshot.CapturedAtLTE(to.UTC()),
		).
		Order(dbent.Asc(accountqualitysnapshot.FieldCapturedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]service.AccountQualitySnapshotRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, snapshotEntityToRow(row))
	}
	return out, nil
}

func (r *accountQualitySnapshotRepository) DeleteExpired(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if r == nil || r.sql == nil {
		return 0, errors.New("account quality snapshot repository is not initialized")
	}
	if limit <= 0 {
		limit = service.AccountQualitySnapshotDeleteBatchSize
	}
	const query = `
		DELETE FROM account_quality_snapshots
		WHERE id IN (
			SELECT id
			FROM account_quality_snapshots
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

func (r *accountQualitySnapshotRepository) ListRecentTrafficAccountIDs(ctx context.Context, startTime time.Time) ([]int64, error) {
	if r == nil || r.sql == nil {
		return nil, errors.New("account quality snapshot repository is not initialized")
	}
	const query = `
		SELECT DISTINCT account_id
		FROM (
			SELECT account_id
			FROM usage_logs
			WHERE created_at >= $1
			  AND account_id IS NOT NULL
			UNION
			SELECT account_id
			FROM ops_error_logs
			WHERE created_at >= $1
			  AND COALESCE(status_code, 0) >= 400
			  AND is_count_tokens = FALSE
			  AND account_id IS NOT NULL
		) traffic
	`
	rows, err := r.sql.QueryContext(ctx, query, startTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0, 64)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

func snapshotEntityToRow(row *dbent.AccountQualitySnapshot) service.AccountQualitySnapshotRow {
	if row == nil {
		return service.AccountQualitySnapshotRow{}
	}
	return service.AccountQualitySnapshotRow{
		AccountID:     row.AccountID,
		CapturedAt:    row.CapturedAt,
		WindowSeconds: row.WindowSeconds,
		SuccessCount:  row.SuccessCount,
		ErrorCount:    row.ErrorCount,
		TTFTSamples:   row.TtftSamples,
		SuccessRate:   row.SuccessRate,
		AvgTTFTMs:     row.AvgTtftMs,
		P50TTFTMs:     row.P50TtftMs,
		P95TTFTMs:     row.P95TtftMs,
		MaxTTFTMs:     row.MaxTtftMs,
	}
}
