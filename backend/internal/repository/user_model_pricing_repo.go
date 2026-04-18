package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userModelPricingRepository struct {
	sql sqlExecutor
}

func NewUserModelPricingRepository(sqlDB *sql.DB) service.UserModelPricingRepository {
	return &userModelPricingRepository{sql: sqlDB}
}

const userModelPricingColumns = `id, user_id, model, input_price, output_price, cache_write_price, cache_read_price,
	display_input_price, display_output_price, display_cache_read_price, display_rate_multiplier, cache_transfer_ratio,
	enabled, notes, created_at, updated_at`

func scanOverride(rows *sql.Rows) (service.UserModelPricingOverride, error) {
	var o service.UserModelPricingOverride
	var inputP, outputP, cwP, crP sql.NullFloat64
	var dInputP, dOutputP, dCacheReadP, dRate, cacheRatio sql.NullFloat64
	var notes sql.NullString
	err := rows.Scan(
		&o.ID, &o.UserID, &o.Model,
		&inputP, &outputP, &cwP, &crP,
		&dInputP, &dOutputP, &dCacheReadP, &dRate, &cacheRatio,
		&o.Enabled, &notes, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return o, err
	}
	if inputP.Valid {
		o.InputPrice = &inputP.Float64
	}
	if outputP.Valid {
		o.OutputPrice = &outputP.Float64
	}
	if cwP.Valid {
		o.CacheWritePrice = &cwP.Float64
	}
	if crP.Valid {
		o.CacheReadPrice = &crP.Float64
	}
	if dInputP.Valid {
		o.DisplayInputPrice = &dInputP.Float64
	}
	if dOutputP.Valid {
		o.DisplayOutputPrice = &dOutputP.Float64
	}
	if dCacheReadP.Valid {
		o.DisplayCacheReadPrice = &dCacheReadP.Float64
	}
	if dRate.Valid {
		o.DisplayRateMultiplier = &dRate.Float64
	}
	if cacheRatio.Valid {
		o.CacheTransferRatio = &cacheRatio.Float64
	}
	if notes.Valid {
		o.Notes = notes.String
	}
	return o, nil
}

func (r *userModelPricingRepository) GetByUserID(ctx context.Context, userID int64) ([]service.UserModelPricingOverride, error) {
	query := `SELECT ` + userModelPricingColumns + ` FROM user_model_pricing_overrides WHERE user_id = $1 ORDER BY model`
	rows, err := r.sql.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []service.UserModelPricingOverride
	for rows.Next() {
		o, err := scanOverride(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

func (r *userModelPricingRepository) GetEnabledByUserID(ctx context.Context, userID int64) ([]service.UserModelPricingOverride, error) {
	query := `SELECT ` + userModelPricingColumns + ` FROM user_model_pricing_overrides WHERE user_id = $1 AND enabled = TRUE ORDER BY model`
	rows, err := r.sql.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []service.UserModelPricingOverride
	for rows.Next() {
		o, err := scanOverride(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

func (r *userModelPricingRepository) GetByUserAndModel(ctx context.Context, userID int64, model string) (*service.UserModelPricingOverride, error) {
	query := `SELECT ` + userModelPricingColumns + ` FROM user_model_pricing_overrides WHERE user_id = $1 AND LOWER(model) = LOWER($2)`
	rows, err := r.sql.QueryContext(ctx, query, userID, model)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	o, err := scanOverride(rows)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *userModelPricingRepository) GetByID(ctx context.Context, id int64) (*service.UserModelPricingOverride, error) {
	query := `SELECT ` + userModelPricingColumns + ` FROM user_model_pricing_overrides WHERE id = $1`
	rows, err := r.sql.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	o, err := scanOverride(rows)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *userModelPricingRepository) Create(ctx context.Context, o *service.UserModelPricingOverride) error {
	now := time.Now()
	query := `INSERT INTO user_model_pricing_overrides
		(user_id, model, input_price, output_price, cache_write_price, cache_read_price,
		 display_input_price, display_output_price, display_cache_read_price, display_rate_multiplier, cache_transfer_ratio,
		 enabled, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)
		RETURNING id, created_at, updated_at`
	rows, err := r.sql.QueryContext(ctx, query,
		o.UserID, o.Model,
		toNullFloat(o.InputPrice), toNullFloat(o.OutputPrice),
		toNullFloat(o.CacheWritePrice), toNullFloat(o.CacheReadPrice),
		toNullFloat(o.DisplayInputPrice), toNullFloat(o.DisplayOutputPrice),
		toNullFloat(o.DisplayCacheReadPrice), toNullFloat(o.DisplayRateMultiplier), toNullFloat(o.CacheTransferRatio),
		o.Enabled, o.Notes, now,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return rows.Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
	}
	return rows.Err()
}

func (r *userModelPricingRepository) Update(ctx context.Context, o *service.UserModelPricingOverride) error {
	now := time.Now()
	query := `UPDATE user_model_pricing_overrides SET
		model = $2, input_price = $3, output_price = $4, cache_write_price = $5, cache_read_price = $6,
		display_input_price = $7, display_output_price = $8, display_cache_read_price = $9, display_rate_multiplier = $10, cache_transfer_ratio = $11,
		enabled = $12, notes = $13, updated_at = $14
		WHERE id = $1`
	_, err := r.sql.ExecContext(ctx, query,
		o.ID, o.Model,
		toNullFloat(o.InputPrice), toNullFloat(o.OutputPrice),
		toNullFloat(o.CacheWritePrice), toNullFloat(o.CacheReadPrice),
		toNullFloat(o.DisplayInputPrice), toNullFloat(o.DisplayOutputPrice),
		toNullFloat(o.DisplayCacheReadPrice), toNullFloat(o.DisplayRateMultiplier), toNullFloat(o.CacheTransferRatio),
		o.Enabled, o.Notes, now,
	)
	if err == nil {
		o.UpdatedAt = now
	}
	return err
}

func (r *userModelPricingRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM user_model_pricing_overrides WHERE id = $1`, id)
	return err
}

func (r *userModelPricingRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM user_model_pricing_overrides WHERE user_id = $1`, userID)
	return err
}

func (r *userModelPricingRepository) BatchUpsert(ctx context.Context, userID int64, overrides []service.UserModelPricingOverride) error {
	now := time.Now()
	for _, o := range overrides {
		_, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_model_pricing_overrides
				(user_id, model, input_price, output_price, cache_write_price, cache_read_price,
				 display_input_price, display_output_price, display_cache_read_price, display_rate_multiplier, cache_transfer_ratio,
				 enabled, notes, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14)
			ON CONFLICT (user_id, model) DO UPDATE SET
				input_price = EXCLUDED.input_price,
				output_price = EXCLUDED.output_price,
				cache_write_price = EXCLUDED.cache_write_price,
				cache_read_price = EXCLUDED.cache_read_price,
				display_input_price = EXCLUDED.display_input_price,
				display_output_price = EXCLUDED.display_output_price,
				display_cache_read_price = EXCLUDED.display_cache_read_price,
				display_rate_multiplier = EXCLUDED.display_rate_multiplier,
				cache_transfer_ratio = EXCLUDED.cache_transfer_ratio,
				enabled = EXCLUDED.enabled,
				notes = EXCLUDED.notes,
				updated_at = EXCLUDED.updated_at`,
			userID, o.Model,
			toNullFloat(o.InputPrice), toNullFloat(o.OutputPrice),
			toNullFloat(o.CacheWritePrice), toNullFloat(o.CacheReadPrice),
			toNullFloat(o.DisplayInputPrice), toNullFloat(o.DisplayOutputPrice),
			toNullFloat(o.DisplayCacheReadPrice), toNullFloat(o.DisplayRateMultiplier), toNullFloat(o.CacheTransferRatio),
			o.Enabled, o.Notes, now,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *userModelPricingRepository) GetEnabledCountByModel(ctx context.Context) (map[string]int, error) {
	query := `SELECT LOWER(model) AS model, COUNT(*) AS cnt FROM user_model_pricing_overrides WHERE enabled = TRUE GROUP BY LOWER(model)`
	rows, err := r.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]int)
	for rows.Next() {
		var model string
		var cnt int
		if err := rows.Scan(&model, &cnt); err != nil {
			return nil, err
		}
		result[model] = cnt
	}
	return result, rows.Err()
}

func (r *userModelPricingRepository) GetByModel(ctx context.Context, model string) ([]service.UserModelPricingOverride, error) {
	query := `SELECT ` + userModelPricingColumns + ` FROM user_model_pricing_overrides WHERE LOWER(model) = LOWER($1) ORDER BY user_id`
	rows, err := r.sql.QueryContext(ctx, query, model)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []service.UserModelPricingOverride
	for rows.Next() {
		o, err := scanOverride(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

func toNullFloat(p *float64) sql.NullFloat64 {
	if p == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *p, Valid: true}
}
