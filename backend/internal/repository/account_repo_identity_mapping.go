package repository

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *accountRepository) MergeIdentityModelMappings(ctx context.Context, platform string, keys []string) (int64, error) {
	keys = normalizeIdentityMappingKeys(keys)
	if r == nil || r.sql == nil || len(keys) == 0 {
		return 0, nil
	}
	mergeJSON, err := identityMappingJSON(keys)
	if err != nil {
		return 0, err
	}

	query, args := identityMappingTargetQuery(true, platform, keys, mergeJSON)
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	r.syncSchedulerAccountSnapshots(ctx, ids)
	if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventAccountBulkChanged, nil, nil, map[string]any{"account_ids": ids}); err != nil {
		logger.LegacyPrintf("repository.account", "[SchedulerOutbox] enqueue identity mapping merge failed: err=%v", err)
	}
	return int64(len(ids)), nil
}

func (r *accountRepository) CountIdentityModelMappingTargets(ctx context.Context, platform string, keys []string) (int64, error) {
	keys = normalizeIdentityMappingKeys(keys)
	if r == nil || r.sql == nil || len(keys) == 0 {
		return 0, nil
	}
	query, args := identityMappingTargetQuery(false, platform, keys, "")
	var count int64
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
	}
	return count, rows.Err()
}

func normalizeIdentityMappingKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func identityMappingJSON(keys []string) (string, error) {
	merge := make(map[string]string, len(keys))
	for _, key := range keys {
		merge[key] = key
	}
	raw, err := json.Marshal(merge)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func identityMappingTargetQuery(update bool, platform string, keys []string, mergeJSON string) (string, []any) {
	args := make([]any, 0, 2+len(keys))
	idx := 1
	next := func(v any) string {
		placeholder := "$" + itoa(idx)
		args = append(args, v)
		idx++
		return placeholder
	}

	platformPh := next(platform)
	var mergePh string
	if update {
		mergePh = next(mergeJSON)
	}

	missing := make([]string, 0, len(keys))
	for _, key := range keys {
		missing = append(missing, "(credentials->'model_mapping'->>"+next(key)+") IS NULL")
	}

	where := "platform = " + platformPh + `
  AND deleted_at IS NULL
  AND jsonb_typeof(credentials->'model_mapping') = 'object'
  AND credentials->'model_mapping' <> '{}'::jsonb
  AND COALESCE(extra->>'openai_passthrough', 'false') NOT IN ('true', '1')
  AND COALESCE(extra->>'openai_oauth_passthrough', 'false') NOT IN ('true', '1')
  AND (` + strings.Join(missing, " OR ") + `)`

	if update {
		return `UPDATE accounts
SET credentials = jsonb_set(
      credentials,
      '{model_mapping}',
      COALESCE(credentials->'model_mapping', '{}'::jsonb) || ` + mergePh + `::jsonb
    ),
    updated_at = NOW()
WHERE ` + where + `
RETURNING id`, args
	}
	return `SELECT COUNT(*) FROM accounts WHERE ` + where, args
}
