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

// NewUserGroupRateRepository 创建用户专属分组倍率仓储
func NewUserGroupRateRepository(sqlDB *sql.DB) service.UserGroupRateRepository {
	return &userGroupRateRepository{sql: sqlDB}
}

// GetByUserID 获取用户的所有专属分组倍率（仅真实倍率，NULL 行不返回）
func (r *userGroupRateRepository) GetByUserID(ctx context.Context, userID int64) (map[int64]float64, error) {
	query := `SELECT group_id, rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND rate_multiplier IS NOT NULL`
	rows, err := r.sql.QueryContext(ctx, query, userID)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetFullByUserID 获取用户的所有分组倍率数据（含展示倍率）
func (r *userGroupRateRepository) GetFullByUserID(ctx context.Context, userID int64) (map[int64]service.UserGroupRateData, error) {
	query := `SELECT group_id, rate_multiplier, display_rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1`
	rows, err := r.sql.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[int64]service.UserGroupRateData)
	for rows.Next() {
		var groupID int64
		var rate sql.NullFloat64
		var displayRate sql.NullFloat64
		if err := rows.Scan(&groupID, &rate, &displayRate); err != nil {
			return nil, err
		}
		data := service.UserGroupRateData{}
		if rate.Valid {
			v := rate.Float64
			data.RateMultiplier = &v
		}
		if displayRate.Valid {
			v := displayRate.Float64
			data.DisplayRateMultiplier = &v
		}
		result[groupID] = data
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetByUserIDs 批量获取多个用户的专属分组倍率。
// 返回结构：map[userID]map[groupID]rate
func (r *userGroupRateRepository) GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]float64, error) {
	result := make(map[int64]map[int64]float64, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	uniqueIDs := make([]int64, 0, len(userIDs))
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		uniqueIDs = append(uniqueIDs, userID)
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
		var userID int64
		var groupID int64
		var rate float64
		if err := rows.Scan(&userID, &groupID, &rate); err != nil {
			return nil, err
		}
		if _, ok := result[userID]; !ok {
			result[userID] = make(map[int64]float64)
		}
		result[userID][groupID] = rate
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetFullByUserIDs 批量获取多个用户的分组倍率完整数据（含展示倍率）
func (r *userGroupRateRepository) GetFullByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]service.UserGroupRateData, error) {
	result := make(map[int64]map[int64]service.UserGroupRateData, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	uniqueIDs := make([]int64, 0, len(userIDs))
	seen := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		uniqueIDs = append(uniqueIDs, userID)
	}
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	rows, err := r.sql.QueryContext(ctx, `
		SELECT user_id, group_id, rate_multiplier, display_rate_multiplier
		FROM user_group_rate_multipliers
		WHERE user_id = ANY($1)
	`, pq.Array(uniqueIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID, groupID int64
		var rate sql.NullFloat64
		var displayRate sql.NullFloat64
		if err := rows.Scan(&userID, &groupID, &rate, &displayRate); err != nil {
			return nil, err
		}
		if _, ok := result[userID]; !ok {
			result[userID] = make(map[int64]service.UserGroupRateData)
		}
		data := service.UserGroupRateData{}
		if rate.Valid {
			v := rate.Float64
			data.RateMultiplier = &v
		}
		if displayRate.Valid {
			v := displayRate.Float64
			data.DisplayRateMultiplier = &v
		}
		result[userID][groupID] = data
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetByGroupID 获取指定分组下所有用户的专属倍率
func (r *userGroupRateRepository) GetByGroupID(ctx context.Context, groupID int64) ([]service.UserGroupRateEntry, error) {
	query := `
		SELECT ugr.user_id, u.username, u.email, COALESCE(u.notes, ''), u.status, ugr.rate_multiplier, ugr.display_rate_multiplier
		FROM user_group_rate_multipliers ugr
		JOIN users u ON u.id = ugr.user_id AND u.deleted_at IS NULL
		WHERE ugr.group_id = $1
		ORDER BY ugr.user_id
	`
	rows, err := r.sql.QueryContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var result []service.UserGroupRateEntry
	for rows.Next() {
		var entry service.UserGroupRateEntry
		if err := rows.Scan(&entry.UserID, &entry.UserName, &entry.UserEmail, &entry.UserNotes, &entry.UserStatus, &entry.RateMultiplier, &entry.DisplayRateMultiplier); err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetByUserAndGroup 获取用户在特定分组的专属倍率
func (r *userGroupRateRepository) GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	query := `SELECT rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = $2 AND rate_multiplier IS NOT NULL`
	var rate float64
	err := scanSingleRow(ctx, r.sql, query, []any{userID, groupID}, &rate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rate, nil
}

// GetDisplayRateByUserAndGroup 获取用户在特定分组的展示倍率
func (r *userGroupRateRepository) GetDisplayRateByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error) {
	query := `SELECT display_rate_multiplier FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = $2 AND display_rate_multiplier IS NOT NULL`
	var rate float64
	err := scanSingleRow(ctx, r.sql, query, []any{userID, groupID}, &rate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rate, nil
}

// SyncUserGroupRates 同步用户的分组专属倍率（仅真实倍率，保留展示倍率）
func (r *userGroupRateRepository) SyncUserGroupRates(ctx context.Context, userID int64, rates map[int64]*float64) error {
	if len(rates) == 0 {
		// 仅清除 rate_multiplier，保留有 display_rate_multiplier 的行
		if _, err := r.sql.ExecContext(ctx,
			`UPDATE user_group_rate_multipliers SET rate_multiplier = NULL, updated_at = NOW() WHERE user_id = $1`,
			userID); err != nil {
			return err
		}
		_, err := r.sql.ExecContext(ctx,
			`DELETE FROM user_group_rate_multipliers WHERE user_id = $1 AND rate_multiplier IS NULL AND display_rate_multiplier IS NULL`,
			userID)
		return err
	}

	var toDelete []int64
	upsertGroupIDs := make([]int64, 0, len(rates))
	upsertRates := make([]float64, 0, len(rates))
	for groupID, rate := range rates {
		if rate == nil {
			toDelete = append(toDelete, groupID)
		} else {
			upsertGroupIDs = append(upsertGroupIDs, groupID)
			upsertRates = append(upsertRates, *rate)
		}
	}

	if len(toDelete) > 0 {
		if _, err := r.sql.ExecContext(ctx,
			`UPDATE user_group_rate_multipliers SET rate_multiplier = NULL, updated_at = NOW() WHERE user_id = $1 AND group_id = ANY($2)`,
			userID, pq.Array(toDelete)); err != nil {
			return err
		}
		if _, err := r.sql.ExecContext(ctx,
			`DELETE FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = ANY($2) AND rate_multiplier IS NULL AND display_rate_multiplier IS NULL`,
			userID, pq.Array(toDelete)); err != nil {
			return err
		}
	}

	now := time.Now()
	if len(upsertGroupIDs) > 0 {
		_, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, created_at, updated_at)
			SELECT
				$1::bigint,
				data.group_id,
				data.rate_multiplier,
				$2::timestamptz,
				$2::timestamptz
			FROM unnest($3::bigint[], $4::double precision[]) AS data(group_id, rate_multiplier)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET
				rate_multiplier = EXCLUDED.rate_multiplier,
				updated_at = EXCLUDED.updated_at
		`, userID, now, pq.Array(upsertGroupIDs), pq.Array(upsertRates))
		if err != nil {
			return err
		}
	}

	return nil
}

// SyncUserGroupRatesFull 同步用户的分组专属倍率（含展示倍率）
func (r *userGroupRateRepository) SyncUserGroupRatesFull(ctx context.Context, userID int64, rates map[int64]*service.UserGroupRateData) error {
	if len(rates) == 0 {
		_, err := r.sql.ExecContext(ctx, `DELETE FROM user_group_rate_multipliers WHERE user_id = $1`, userID)
		return err
	}

	var toDelete []int64
	type upsertRow struct {
		groupID     int64
		rate        sql.NullFloat64
		displayRate sql.NullFloat64
	}
	var upserts []upsertRow
	for groupID, data := range rates {
		if data == nil {
			toDelete = append(toDelete, groupID)
			continue
		}
		row := upsertRow{groupID: groupID}
		if data.RateMultiplier != nil {
			row.rate = sql.NullFloat64{Float64: *data.RateMultiplier, Valid: true}
		}
		if data.DisplayRateMultiplier != nil {
			row.displayRate = sql.NullFloat64{Float64: *data.DisplayRateMultiplier, Valid: true}
		}
		if !row.rate.Valid && !row.displayRate.Valid {
			toDelete = append(toDelete, groupID)
			continue
		}
		upserts = append(upserts, row)
	}

	if len(toDelete) > 0 {
		if _, err := r.sql.ExecContext(ctx,
			`DELETE FROM user_group_rate_multipliers WHERE user_id = $1 AND group_id = ANY($2)`,
			userID, pq.Array(toDelete)); err != nil {
			return err
		}
	}

	now := time.Now()
	for _, row := range upserts {
		_, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, display_rate_multiplier, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET
				rate_multiplier = EXCLUDED.rate_multiplier,
				display_rate_multiplier = EXCLUDED.display_rate_multiplier,
				updated_at = EXCLUDED.updated_at
		`, userID, row.groupID, row.rate, row.displayRate, now)
		if err != nil {
			return err
		}
	}

	return nil
}

// SyncGroupRateMultipliers 批量同步分组的用户专属倍率（先删后插）
// 注意：保留未在 entries 中出现但有 display_rate_multiplier 的行
func (r *userGroupRateRepository) SyncGroupRateMultipliers(ctx context.Context, groupID int64, entries []service.GroupRateMultiplierInput) error {
	if len(entries) == 0 {
		// 仅清除 rate_multiplier，保留有 display_rate_multiplier 的行
		if _, err := r.sql.ExecContext(ctx,
			`UPDATE user_group_rate_multipliers SET rate_multiplier = NULL, updated_at = NOW() WHERE group_id = $1`,
			groupID); err != nil {
			return err
		}
		// 删除 rate 和 display_rate 都为空的行
		_, err := r.sql.ExecContext(ctx,
			`DELETE FROM user_group_rate_multipliers WHERE group_id = $1 AND rate_multiplier IS NULL AND display_rate_multiplier IS NULL`,
			groupID)
		return err
	}

	entryUserIDs := make([]int64, 0, len(entries))
	for _, e := range entries {
		entryUserIDs = append(entryUserIDs, e.UserID)
	}

	// 不在 entries 中的用户：清 rate_multiplier 但保留 display_rate_multiplier
	if _, err := r.sql.ExecContext(ctx, `
		UPDATE user_group_rate_multipliers
		SET rate_multiplier = NULL, updated_at = NOW()
		WHERE group_id = $1 AND user_id != ALL($2)
	`, groupID, pq.Array(entryUserIDs)); err != nil {
		return err
	}
	// 删除 rate 和 display 都为空的行
	if _, err := r.sql.ExecContext(ctx, `
		DELETE FROM user_group_rate_multipliers
		WHERE group_id = $1 AND rate_multiplier IS NULL AND display_rate_multiplier IS NULL
	`, groupID); err != nil {
		return err
	}

	now := time.Now()
	for _, e := range entries {
		var rate sql.NullFloat64
		if e.RateMultiplier != nil {
			rate = sql.NullFloat64{Float64: *e.RateMultiplier, Valid: true}
		}
		var displayRate sql.NullFloat64
		if e.DisplayRateMultiplier != nil {
			displayRate = sql.NullFloat64{Float64: *e.DisplayRateMultiplier, Valid: true}
		}
		if !rate.Valid && !displayRate.Valid {
			continue
		}
		_, err := r.sql.ExecContext(ctx, `
			INSERT INTO user_group_rate_multipliers (user_id, group_id, rate_multiplier, display_rate_multiplier, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5)
			ON CONFLICT (user_id, group_id)
			DO UPDATE SET rate_multiplier = EXCLUDED.rate_multiplier,
				display_rate_multiplier = COALESCE(EXCLUDED.display_rate_multiplier, user_group_rate_multipliers.display_rate_multiplier),
				updated_at = EXCLUDED.updated_at
		`, e.UserID, groupID, rate, displayRate, now)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteByGroupID 删除指定分组的所有用户专属倍率
func (r *userGroupRateRepository) DeleteByGroupID(ctx context.Context, groupID int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM user_group_rate_multipliers WHERE group_id = $1`, groupID)
	return err
}

// DeleteByUserID 删除指定用户的所有专属倍率
func (r *userGroupRateRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM user_group_rate_multipliers WHERE user_id = $1`, userID)
	return err
}
