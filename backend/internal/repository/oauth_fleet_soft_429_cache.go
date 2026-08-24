package repository

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const oauthFleetSoft429KeyPrefix = "oauth-soft-429:"

type oauthFleetSoft429Cache struct {
	rdb *redis.Client
}

func NewOAuthFleetSoft429Cache(rdb *redis.Client) service.OAuthFleetSoft429Cache {
	return &oauthFleetSoft429Cache{rdb: rdb}
}

func (c *oauthFleetSoft429Cache) SetSoftExclude(ctx context.Context, accountID int64, ttl time.Duration) error {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return nil
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	key := service.OAuthFleetSoft429RedisKey(accountID)
	return c.rdb.Set(ctx, key, "1", ttl).Err()
}

func (c *oauthFleetSoft429Cache) ListSoftExcluded(ctx context.Context) ([]int64, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	var (
		cursor uint64
		ids    []int64
	)
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, oauthFleetSoft429KeyPrefix+"*", 64).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			id, ok := parseOAuthFleetSoft429Key(key)
			if !ok {
				continue
			}
			ids = append(ids, id)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return ids, nil
}

func parseOAuthFleetSoft429Key(key string) (int64, bool) {
	raw, ok := strings.CutPrefix(key, oauthFleetSoft429KeyPrefix)
	if !ok || raw == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
