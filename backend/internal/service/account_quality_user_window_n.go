package service

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
)

func queryUserQualityWindowNBatch(ctx context.Context, db *sql.DB, userIDs []int64) map[int64]*int {
	out := map[int64]*int{}
	if db == nil || len(userIDs) == 0 {
		return out
	}
	ids := normalizeQualityBatchIDs(userIDs)
	if len(ids) == 0 {
		return out
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, quality_window_n
		FROM users
		WHERE id = ANY($1) AND quality_window_n IS NOT NULL AND deleted_at IS NULL
	`, pq.Array(ids))
	if err != nil {
		return out
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var n sql.NullInt64
		if err := rows.Scan(&id, &n); err != nil {
			return out
		}
		if !n.Valid {
			continue
		}
		value := int(n.Int64)
		out[id] = &value
	}
	return out
}
