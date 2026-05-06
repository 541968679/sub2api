package repository

import (
	"context"
	"database/sql"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type antigravityCreditSampleRepository struct {
	db *sql.DB
}

func NewAntigravityCreditSampleRepository(_ *dbent.Client, sqlDB *sql.DB) service.AntigravityCreditSampleRepository {
	return &antigravityCreditSampleRepository{db: sqlDB}
}

func (r *antigravityCreditSampleRepository) Insert(ctx context.Context, sample *service.AntigravityCreditRequestSample) error {
	if r == nil || r.db == nil || sample == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO antigravity_credit_request_samples (
			usage_log_id,
			request_id,
			account_id,
			api_key_id,
			user_id,
			email,
			credit_type,
			before_amount,
			after_amount,
			delta_amount,
			before_captured_at,
			after_captured_at,
			confidence,
			error,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13, $14, $15
		)
	`,
		sample.UsageLogID,
		nullStringValue(sample.RequestID),
		sample.AccountID,
		sample.APIKeyID,
		sample.UserID,
		nullStringValue(sample.Email),
		sample.CreditType,
		sample.BeforeAmount,
		sample.AfterAmount,
		sample.DeltaAmount,
		sample.BeforeCapturedAt,
		sample.AfterCapturedAt,
		sample.Confidence,
		nullStringValue(sample.Error),
		sample.CreatedAt,
	)
	return err
}

func nullStringValue(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
