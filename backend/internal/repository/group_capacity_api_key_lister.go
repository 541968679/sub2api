package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// groupCapacityAPIKeyLister loads active API key IDs per group for capacity "used" stats.
// Concurrent request occupancy is tracked per API key (stats-only slots), which is
// group-scoped — unlike account concurrency which is shared across all groups.
type groupCapacityAPIKeyLister struct {
	sql *sql.DB
}

// NewGroupCapacityAPIKeyLister creates a GroupCapacityAPIKeyIDLister backed by SQL.
func NewGroupCapacityAPIKeyLister(sqlDB *sql.DB) service.GroupCapacityAPIKeyIDLister {
	if sqlDB == nil {
		return nil
	}
	return &groupCapacityAPIKeyLister{sql: sqlDB}
}

func (l *groupCapacityAPIKeyLister) ListActiveAPIKeyIDsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]int64, error) {
	out := make(map[int64][]int64, len(groupIDs))
	ids := uniquePositiveInt64s(groupIDs)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := l.sql.QueryContext(ctx, `
		SELECT id, group_id
		FROM api_keys
		WHERE group_id = ANY($1)
			AND deleted_at IS NULL
			AND status = $2
	`, pq.Array(ids), service.StatusActive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var keyID, groupID int64
		if err := rows.Scan(&keyID, &groupID); err != nil {
			return nil, err
		}
		out[groupID] = append(out[groupID], keyID)
	}
	return out, rows.Err()
}
