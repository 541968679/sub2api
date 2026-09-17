package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func persistGroupModelAllowlist(ctx context.Context, sqlq sqlExecutor, groupID int64, cfg service.GroupModelAllowlist) error {
	if sqlq == nil || groupID <= 0 {
		return nil
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	_, err = sqlq.ExecContext(ctx, `UPDATE groups SET model_allowlist = $1 WHERE id = $2`, raw, groupID)
	return err
}

func attachGroupModelAllowlists(ctx context.Context, sqlq sqlExecutor, groups ...*service.Group) {
	if sqlq == nil || len(groups) == 0 {
		return
	}
	ids := make([]int64, 0, len(groups))
	index := make(map[int64]*service.Group, len(groups))
	for _, g := range groups {
		if g == nil || g.ID <= 0 {
			continue
		}
		ids = append(ids, g.ID)
		index[g.ID] = g
	}
	if len(ids) == 0 {
		return
	}
	rows, err := sqlq.QueryContext(ctx, `SELECT id, model_allowlist FROM groups WHERE id = ANY($1)`, pq.Array(ids))
	if err != nil {
		logger.LegacyPrintf("repository.group", "[ModelAllowlist] load failed: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			logger.LegacyPrintf("repository.group", "[ModelAllowlist] scan failed: %v", err)
			return
		}
		g := index[id]
		if g == nil {
			continue
		}
		var cfg service.GroupModelAllowlist
		if len(raw) > 0 && string(raw) != "null" {
			if err := json.Unmarshal(raw, &cfg); err != nil {
				logger.LegacyPrintf("repository.group", "[ModelAllowlist] unmarshal group=%d failed: %v", id, err)
				continue
			}
		}
		g.ModelAllowlist = cfg
	}
}

func (r *apiKeyRepository) hydrateGroupModelAllowlist(ctx context.Context, key *service.APIKey) *service.APIKey {
	if key != nil && key.Group != nil {
		attachGroupModelAllowlists(ctx, r.sql, key.Group)
	}
	return key
}

func (r *apiKeyRepository) hydrateAPIKeyListAllowlists(ctx context.Context, keys []service.APIKey) {
	groups := make([]*service.Group, 0, len(keys))
	for i := range keys {
		if keys[i].Group != nil {
			groups = append(groups, keys[i].Group)
		}
	}
	attachGroupModelAllowlists(ctx, r.sql, groups...)
}
