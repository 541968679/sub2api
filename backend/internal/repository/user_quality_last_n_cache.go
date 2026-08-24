package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	userQualityLastNKeyPrefix = "user-quality:last-n:"
	userQualityLastNTTL       = 7 * 24 * time.Hour
)

func userQualityLastNKey(userID int64) string {
	return userQualityLastNKeyPrefix + strconv.FormatInt(userID, 10)
}

func userIDFromLastNKey(key string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(key, userQualityLastNKeyPrefix), 10, 64)
	return id, err == nil && id > 0
}

func (c *accountQualityLiveCache) GetUserLastN(ctx context.Context, userID int64) *service.AccountQualityLastN {
	if c == nil || c.rdb == nil || userID <= 0 {
		return nil
	}
	raw, err := c.rdb.Get(ctx, userQualityLastNKey(userID)).Bytes()
	if err != nil || len(raw) == 0 {
		return nil
	}
	return decodeAccountQualityLastN(raw)
}

func (c *accountQualityLiveCache) GetUserLastNBatch(ctx context.Context, userIDs []int64) map[int64]*service.AccountQualityLastN {
	out := map[int64]*service.AccountQualityLastN{}
	if c == nil || c.rdb == nil || len(userIDs) == 0 {
		return out
	}
	ids := uniquePositiveIDs(userIDs)
	if len(ids) == 0 {
		return out
	}
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.Get(ctx, userQualityLastNKey(id))
	}
	_, _ = pipe.Exec(ctx)
	for i, cmd := range cmds {
		raw, err := cmd.Bytes()
		if err != nil || len(raw) == 0 {
			continue
		}
		if live := decodeAccountQualityLastN(raw); live != nil {
			out[ids[i]] = live
		}
	}
	return out
}

func (c *accountQualityLiveCache) IngestUserLastN(ctx context.Context, userID int64, n int, success bool, firstTokenMs, durationMs *int, useFailover bool, override *int) *service.AccountQualityLastN {
	if c == nil || userID <= 0 {
		return nil
	}
	live := service.ApplyAccountQualityLastNIngest(c.GetUserLastN(ctx, userID), n, success, firstTokenMs, durationMs)
	live.UseFailover = useFailover
	live.OverrideN = service.CopyIntPtr(override)
	c.storeUserLastN(ctx, userID, live)
	return live
}

func (c *accountQualityLiveCache) ResizeUserLastN(ctx context.Context, userID int64, n int, override *int) *service.AccountQualityLastN {
	if c == nil || userID <= 0 {
		return nil
	}
	live := service.ProjectAccountQualityLastN(c.GetUserLastN(ctx, userID), n)
	live.OverrideN = service.CopyIntPtr(override)
	c.storeUserLastN(ctx, userID, live)
	return live
}

func (c *accountQualityLiveCache) ListUserLastNIDs(ctx context.Context) []int64 {
	if c == nil || c.rdb == nil {
		return nil
	}
	var (
		cursor uint64
		ids    []int64
	)
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, userQualityLastNKeyPrefix+"*", 128).Result()
		if err != nil {
			return ids
		}
		for _, key := range keys {
			if id, ok := userIDFromLastNKey(key); ok {
				ids = append(ids, id)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return ids
}

func (c *accountQualityLiveCache) storeUserLastN(ctx context.Context, userID int64, live *service.AccountQualityLastN) {
	if c == nil || c.rdb == nil || userID <= 0 || live == nil {
		return
	}
	payload, err := json.Marshal(encodeAccountQualityLastN(live))
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, userQualityLastNKey(userID), payload, userQualityLastNTTL).Err()
}

var _ service.UserQualityLastNCache = (*accountQualityLiveCache)(nil)
