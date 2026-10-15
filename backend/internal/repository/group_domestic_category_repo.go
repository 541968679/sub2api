package repository

// domestic_category is stored with SQL because `go generate ./ent` currently
// fails on an unrelated edge primary key. The column is not an Ent field.

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func persistDomesticCategory(ctx context.Context, sqlq sqlExecutor, groupID int64, category string) error {
	if sqlq == nil || groupID <= 0 {
		return nil
	}
	_, err := sqlq.ExecContext(ctx, `UPDATE groups SET domestic_category = $1 WHERE id = $2`, category, groupID)
	return err
}

func attachDomesticCategories(ctx context.Context, sqlq sqlExecutor, groups ...*service.Group) {
	if sqlq == nil || len(groups) == 0 {
		return
	}
	ids := make([]int64, 0, len(groups))
	index := make(map[int64]*service.Group, len(groups))
	for _, group := range groups {
		if group == nil || group.ID <= 0 {
			continue
		}
		ids = append(ids, group.ID)
		index[group.ID] = group
	}
	if len(ids) == 0 {
		return
	}
	rows, err := sqlq.QueryContext(ctx, `SELECT id, domestic_category FROM groups WHERE id = ANY($1)`, pq.Array(ids))
	if err != nil {
		logger.LegacyPrintf("repository.group", "[DomesticCategory] load failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var category string
		if err := rows.Scan(&id, &category); err != nil {
			logger.LegacyPrintf("repository.group", "[DomesticCategory] scan failed: %v", err)
			continue
		}
		if group := index[id]; group != nil {
			group.DomesticCategory = category
		}
	}
}
