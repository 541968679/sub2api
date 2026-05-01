package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type userGroupRateRepository struct {
	sql sqlExecutor
}

func NewUserGroupRateRepository(sqlDB *sql.DB) service.UserGroupRateRepository {
	return &userGroupRateRepository{sql: sqlDB}
}

func (r *userGroupRateRepository) GetByUserID(ctx context.Context, userID int64) (map[int64]float64, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT group_id, rate_multiplier
		FROM user_group_rate_multipliers
		WHERE user_id = $1 AND rate_multiplier IS NOT NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]float64)
	for rows.Next() {
		var groupID int64
		var rate float64
		if err := rows.Scan(&groupID, &rate); err != nil {
			return nil, err
		}
		result[groupID] = rate
	}
	return result, rows.Err()
}

func (r *userGroupRateRepository) GetFullByUserID(ctx context.Context, userID int64) (map[int64]service.UserGroupRateData, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT group_id, rate_multiplier, display_rate_multiplier
		FROM user_group_rate_multipliers
		WHERE user_id = $1 AND (rate_multiplier IS NOT NULL OR display_rate_multiplier IS NOT NULL)
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]service.UserGroupRateData)
	for rows.Next() {
		var groupID int64
		var rate, displayRate sql.NullFloat64
		if err := rows.Scan(&groupID, &rate, &displayRate); err != nil {
			return nil, err
		}
		result[groupID] = toUserGroupRateData(rate, displayRate)
	}
	return result, rows.Err()
}

func (r *userGroupRateRepository) GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]float64, error) {
	result := make(map[int64]map[int64]float64, len(userIDs))
	uniqueIDs := uniquePositiveInt64s(userIDs)
	for _, userID := range uniqueIDs {
		result[userID] = make(map[int64]float64)
	}
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT user_id, group_id, rate_multiplier
		FROM user_group_rate_multipliers
		WHERE user_id = ANY($1) AND rate_multiplier IS NOT NULL
	`, pq.Array(uniqueIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID, groupID int64
		var rate float64
		if err := rows.Scan(&userID, &groupID, &rate); err != nil {
			return nil, err
		}
		if _, ok := result[userID]; !ok {
			result[userID] = make(map[int64]float64)
		}
		result[userID][groupID] = rate
	}
	return result, rows.Err()
}

func (r *userGroupRateRepository) GetFullByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]service.UserGroupRateData, error) {
	result := make(map[int64]map[int64]service.UserGroupRateData, len(userIDs))
	uniqueIDs := uniquePositiveInt64s(userIDs)
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT user_id, group_id, rate_multiplier, display_rate_multiplier
		FROM user_group_rate_multipliers
		WHERE user_id = ANY($1) AND (rate_multiplier IS NOT NULL OR display_rate_multiplier IS NOT NULL)
	`, pq.Array(uniqueIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID, groupID int64
		var rate, displayRate sql.NullFloat64
		if err := rows.Scan(&userID, &groupID, &rate, &displayRate); err != nil {
			return nil, err
		}
		if _, ok := result[userID]; !ok {
			result[userID] = make(map[int64]service.UserGroupRateData)
		}
		result[userID][groupID] = toUserGroupRateData(rate, displayRate)
	}
	return result, rows.Err()
}

func (r *userGroupRateRepository) GetByGroupID(ctx context.Context, groupID int64) ([]service.UserGroupRateEntry, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT ugr.user_id, u.username, u.email, COALESCE(u.notes, ''), u.status,
		       ugr.rate_multiplier, ugr.display_rate_multiplier, ugr.rpm_override
		FROM user_group_rate_multipliers ugr
		JOIN users u ON u.id = ugr.user_id AND u.deleted_at IS NULL
		WHERE ugr.group_id = $1
		  AND (ugr.rate_multiplier IS NOT NULL OR ugr.display_rate_multiplier IS NOT NULL OR ugr.rpm_override IS NOT NULL)
		ORDER BY ugr.user_id
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []service.UserGroupRateEntry
	for rows.Next() {
		var entry service.UserGroupRateEntry
		var rate, displayRate sql.NullFloat64
		var rpm sql.NullInt32
		if err := rows.Scan(&entry.UserID, &entry.UserName, &entry.UserEmail, &entry.UserNotes, &entry.UserStatus, &rate, &displayRate, &rpm); err != nil {
			return nil, err
		}
		if rate.Valid {
			v := rate.Float64
			entry.RateMultiplier = &v
		}
		if displayRate.Valid {
			v := displayRate.Float64
			entry.DisplayRateMultiplier = &v
		}
		if rpm.Valid {
			v := int(rpm.Int32)
			entry.RPMOverride = &v
		}
		result = append(result, entry)
	}
	return result, rows.Err()
}

func (r *userGroupRateRepository) GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	var rate sql.NullFloat64
	err := scanSingleRow(ctx, r.sql, `SELECT rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = $2`, []any{userID, groupID}, &rate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil || !rate.Valid {
		return nil, err
	}
	v := rate.Float64
	return &v, nil
}

func (r *userGroupRateRepository) GetDisplayRateByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	var rate sql.NullFloat64
	err := scanSingleRow(ctx, r.sql, `SELECT display_rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = $2`, []any{userID, groupID}, &rate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil || !rate.Valid {
		return nil, err
	}
	v := rate.Float64
	return &v, nil
}

func (r *userGroupRateRepository) GetRPMOverrideByUserAndGroup(ctx context.Context, userID, groupID int64) (*int, error) {
	var rpm sql.NullInt32
	err := scanSingleRow(ctx, r.sql, `SELECT rpm_override FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = $2`, []any{userID, groupID}, &rpm)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil || !rpm.Valid {
		return nil, err
	}
	v := int(rpm.Int32)
	return &v, nil
}

func (r *userGroupRateRepository) SyncUserGroupRates(ctx context.Context, userID int64, rates map[int64]*float64) error {
	if len(rates) == 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rate_multiplier = NULL, updated_at = NOW()
			WHERE user_id = $1
		`, userID); err != nil {
			return err
		}
		return r.deleteEmptyUserRows(ctx, userID)
	}

	var clearGroupIDs []int64
	upsertGroupIDs := make([]int64, 0, len(rates))
	upsertRates := make([]float64, 0, len(rates))
	for groupID, rate := range rates {
		if rate == nil {
			clearGroupIDs = append(clearGroupIDs, groupID)
			continue
		}
		upsertGroupIDs = append(upsertGroupIDs, groupID)
		upsertRates = append(upsertRates, *rate)
	}

	if len(clearGroupIDs) > 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rate_multiplier = NULL, updated_at = NOW()
			WHERE user_id = $1 AND group_id = ANY($2)
		`, userID, pq.Array(clearGroupIDs)); err != nil {
			return err
		}
		if err := r.deleteEmptyUserGroupRows(ctx, userID, clearGroupIDs); err != nil {
			return err
		}
	}

	if len(upsertGroupIDs) > 0 {
		now := time.Now()
		if _, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, created_at, updated_at)
			SELECT $1::bigint, data.group_id, data.rate_multiplier, $2::timestamptz, $2::timestamptz
			FROM unnest($3::bigint[], $4::double precision[]) AS data(group_id, rate_multiplier)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET rate_multiplier = EXCLUDED.rate_multiplier, updated_at = EXCLUDED.updated_at
		`, userID, now, pq.Array(upsertGroupIDs), pq.Array(upsertRates)); err != nil {
			return err
		}
	}
	return nil
}

func (r *userGroupRateRepository) SyncUserGroupRatesFull(ctx context.Context, userID int64, rates map[int64]*service.UserGroupRateData) error {
	if len(rates) == 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rate_multiplier = NULL, display_rate_multiplier = NULL, updated_at = NOW()
			WHERE user_id = $1
		`, userID); err != nil {
			return err
		}
		return r.deleteEmptyUserRows(ctx, userID)
	}

	var clearGroupIDs []int64
	now := time.Now()
	for groupID, data := range rates {
		if data == nil || (data.RateMultiplier == nil && data.DisplayRateMultiplier == nil) {
			clearGroupIDs = append(clearGroupIDs, groupID)
			continue
		}

		var rate, displayRate sql.NullFloat64
		if data.RateMultiplier != nil {
			rate = sql.NullFloat64{Float64: *data.RateMultiplier, Valid: true}
		}
		if data.DisplayRateMultiplier != nil {
			displayRate = sql.NullFloat64{Float64: *data.DisplayRateMultiplier, Valid: true}
		}
		if _, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, display_rate_multiplier, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET
				rate_multiplier = EXCLUDED.rate_multiplier,
				display_rate_multiplier = EXCLUDED.display_rate_multiplier,
				updated_at = EXCLUDED.updated_at
		`, userID, groupID, rate, displayRate, now); err != nil {
			return err
		}
	}

	if len(clearGroupIDs) > 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rate_multiplier = NULL, display_rate_multiplier = NULL, updated_at = NOW()
			WHERE user_id = $1 AND group_id = ANY($2)
		`, userID, pq.Array(clearGroupIDs)); err != nil {
			return err
		}
		return r.deleteEmptyUserGroupRows(ctx, userID, clearGroupIDs)
	}
	return nil
}

func (r *userGroupRateRepository) SyncGroupRateMultipliers(ctx context.Context, groupID int64, entries []service.GroupRateMultiplierInput) error {
	keepUserIDs := make([]int64, 0, len(entries))
	for _, e := range entries {
		keepUserIDs = append(keepUserIDs, e.UserID)
	}

	if len(keepUserIDs) == 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rate_multiplier = NULL, display_rate_multiplier = NULL, updated_at = NOW()
			WHERE group_id = $1
		`, groupID); err != nil {
			return err
		}
		return r.deleteEmptyGroupRows(ctx, groupID)
	}

	if _, err := r.sql.ExecContext(ctx, `
		UPDATE user_group_rate_multipliers
		SET rate_multiplier = NULL, display_rate_multiplier = NULL, updated_at = NOW()
		WHERE group_id = $1 AND user_id <> ALL($2)
	`, groupID, pq.Array(keepUserIDs)); err != nil {
		return err
	}
	if err := r.deleteEmptyGroupRows(ctx, groupID); err != nil {
		return err
	}

	now := time.Now()
	for _, e := range entries {
		if e.RateMultiplier == nil && e.DisplayRateMultiplier == nil {
			continue
		}
		var rate, displayRate sql.NullFloat64
		if e.RateMultiplier != nil {
			rate = sql.NullFloat64{Float64: *e.RateMultiplier, Valid: true}
		}
		if e.DisplayRateMultiplier != nil {
			displayRate = sql.NullFloat64{Float64: *e.DisplayRateMultiplier, Valid: true}
		}
		if _, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, display_rate_multiplier, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET
				rate_multiplier = EXCLUDED.rate_multiplier,
				display_rate_multiplier = EXCLUDED.display_rate_multiplier,
				updated_at = EXCLUDED.updated_at
		`, e.UserID, groupID, rate, displayRate, now); err != nil {
			return err
		}
	}
	return nil
}

func (r *userGroupRateRepository) SyncGroupRPMOverrides(ctx context.Context, groupID int64, entries []service.GroupRPMOverrideInput) error {
	keepUserIDs := make([]int64, 0, len(entries))
	var clearUserIDs []int64
	upsertUserIDs := make([]int64, 0, len(entries))
	upsertValues := make([]int32, 0, len(entries))
	for _, e := range entries {
		keepUserIDs = append(keepUserIDs, e.UserID)
		if e.RPMOverride == nil {
			clearUserIDs = append(clearUserIDs, e.UserID)
			continue
		}
		upsertUserIDs = append(upsertUserIDs, e.UserID)
		upsertValues = append(upsertValues, int32(*e.RPMOverride))
	}

	if len(keepUserIDs) == 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rpm_override = NULL, updated_at = NOW()
			WHERE group_id = $1
		`, groupID); err != nil {
			return err
		}
	} else if _, err := r.sql.ExecContext(ctx, `
		UPDATE user_group_rate_multipliers
		SET rpm_override = NULL, updated_at = NOW()
		WHERE group_id = $1 AND user_id <> ALL($2)
	`, groupID, pq.Array(keepUserIDs)); err != nil {
		return err
	}

	if len(clearUserIDs) > 0 {
		if _, err := r.sql.ExecContext(ctx, `
			UPDATE user_group_rate_multipliers
			SET rpm_override = NULL, updated_at = NOW()
			WHERE group_id = $1 AND user_id = ANY($2)
		`, groupID, pq.Array(clearUserIDs)); err != nil {
			return err
		}
	}
	if err := r.deleteEmptyGroupRows(ctx, groupID); err != nil {
		return err
	}

	if len(upsertUserIDs) > 0 {
		now := time.Now()
		if _, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rpm_override, created_at, updated_at)
			SELECT data.user_id, $1::bigint, data.rpm_override, $2::timestamptz, $2::timestamptz
			FROM unnest($3::bigint[], $4::integer[]) AS data(user_id, rpm_override)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET rpm_override = EXCLUDED.rpm_override, updated_at = EXCLUDED.updated_at
		`, groupID, now, pq.Array(upsertUserIDs), pq.Array(upsertValues)); err != nil {
			return err
		}
	}
	return nil
}

func (r *userGroupRateRepository) ClearGroupRPMOverrides(ctx context.Context, groupID int64) error {
	if _, err := r.sql.ExecContext(ctx, `
		UPDATE user_group_rate_multipliers
		SET rpm_override = NULL, updated_at = NOW()
		WHERE group_id = $1
	`, groupID); err != nil {
		return err
	}
	return r.deleteEmptyGroupRows(ctx, groupID)
}

func (r *userGroupRateRepository) DeleteByGroupID(ctx context.Context, groupID int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM user_group_rate_multipliers WHERE group_id = $1`, groupID)
	return err
}

func (r *userGroupRateRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM user_group_rate_multipliers WHERE user_id = $1`, userID)
	return err
}

func (r *userGroupRateRepository) deleteEmptyGroupRows(ctx context.Context, groupID int64) error {
	_, err := r.sql.ExecContext(ctx, `
		DELETE FROM user_group_rate_multipliers
		WHERE group_id = $1
		  AND rate_multiplier IS NULL
		  AND display_rate_multiplier IS NULL
		  AND rpm_override IS NULL
	`, groupID)
	return err
}

func (r *userGroupRateRepository) deleteEmptyUserRows(ctx context.Context, userID int64) error {
	_, err := r.sql.ExecContext(ctx, `
		DELETE FROM user_group_rate_multipliers
		WHERE user_id = $1
		  AND rate_multiplier IS NULL
		  AND display_rate_multiplier IS NULL
		  AND rpm_override IS NULL
	`, userID)
	return err
}

func (r *userGroupRateRepository) deleteEmptyUserGroupRows(ctx context.Context, userID int64, groupIDs []int64) error {
	_, err := r.sql.ExecContext(ctx, `
		DELETE FROM user_group_rate_multipliers
		WHERE user_id = $1
		  AND group_id = ANY($2)
		  AND rate_multiplier IS NULL
		  AND display_rate_multiplier IS NULL
		  AND rpm_override IS NULL
	`, userID, pq.Array(groupIDs))
	return err
}

func toUserGroupRateData(rate, displayRate sql.NullFloat64) service.UserGroupRateData {
	data := service.UserGroupRateData{}
	if rate.Valid {
		v := rate.Float64
		data.RateMultiplier = &v
	}
	if displayRate.Valid {
		v := displayRate.Float64
		data.DisplayRateMultiplier = &v
	}
	return data
}

func uniquePositiveInt64s(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
