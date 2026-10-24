package repository

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	oauthFleetSoft429KeyPrefix = "oauth-soft-429:"
	// Sorted set of account IDs. Score is expiry unix milliseconds.
	// One ZRANGE replaces a full-keyspace SCAN on every account selection.
	oauthFleetSoft429ExpKey      = "oauth-soft-429:exp"
	oauthFleetSoft429MigratedKey = "oauth-soft-429:migrated"
	oauthFleetSoft429MigrateLock = "oauth-soft-429:migrate-lock"
	oauthFleetSoft429MigrateTTL  = 2 * time.Minute
)

// startOAuthFleetSoft429Migration is disabled by the unit test file so New
// does not race a background SCAN against miniredis.
var startOAuthFleetSoft429Migration = true

var oauthFleetSoft429Now = time.Now

// Drop expired members, then return the rest. Score <= now is expired.
var oauthFleetSoft429ListScript = redis.NewScript(`
local now = tonumber(ARGV[1])
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
return redis.call('ZRANGE', KEYS[1], 0, -1)
`)

type oauthFleetSoft429Cache struct {
	rdb *redis.Client
}

func NewOAuthFleetSoft429Cache(rdb *redis.Client) service.OAuthFleetSoft429Cache {
	c := &oauthFleetSoft429Cache{rdb: rdb}
	if startOAuthFleetSoft429Migration && rdb != nil {
		go c.migrateLegacyKeysAsync()
	}
	return c
}

func (c *oauthFleetSoft429Cache) SetSoftExclude(ctx context.Context, accountID int64, ttl time.Duration) error {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return nil
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	member := strconv.FormatInt(accountID, 10)
	expireAt := oauthFleetSoft429Now().Add(ttl).UnixMilli()
	if err := c.rdb.ZAdd(ctx, oauthFleetSoft429ExpKey, redis.Z{
		Score:  float64(expireAt),
		Member: member,
	}).Err(); err != nil {
		return err
	}
	// Drop a legacy per-account key so a later migration cannot overwrite a newer score.
	_ = c.rdb.Del(ctx, service.OAuthFleetSoft429RedisKey(accountID)).Err()
	return nil
}

func (c *oauthFleetSoft429Cache) ListSoftExcluded(ctx context.Context) ([]int64, error) {
	if c == nil || c.rdb == nil {
		return nil, nil
	}
	now := oauthFleetSoft429Now().UnixMilli()
	members, err := oauthFleetSoft429ListScript.Run(ctx, c.rdb, []string{oauthFleetSoft429ExpKey}, now).StringSlice()
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(members))
	for _, member := range members {
		id, parseErr := strconv.ParseInt(member, 10, 64)
		if parseErr != nil || id <= 0 {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *oauthFleetSoft429Cache) migrateLegacyKeysAsync() {
	ctx, cancel := context.WithTimeout(context.Background(), oauthFleetSoft429MigrateTTL)
	defer cancel()
	if err := c.migrateLegacyKeys(ctx); err != nil {
		slog.Warn("oauth_fleet_soft_429_migrate", "error", err)
	}
}

// migrateLegacyKeys copies oauth-soft-429:{accountID} strings into the expiry
// sorted set once per Redis. It walks the keyspace a single time, off the
// request path. Later lists never SCAN.
func (c *oauthFleetSoft429Cache) migrateLegacyKeys(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	done, err := c.rdb.Exists(ctx, oauthFleetSoft429MigratedKey).Result()
	if err != nil {
		return err
	}
	if done > 0 {
		return nil
	}
	locked, err := c.rdb.SetNX(ctx, oauthFleetSoft429MigrateLock, "1", oauthFleetSoft429MigrateTTL).Result()
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer c.rdb.Del(context.Background(), oauthFleetSoft429MigrateLock)

	var cursor uint64
	for {
		keys, next, scanErr := c.rdb.Scan(ctx, cursor, oauthFleetSoft429KeyPrefix+"*", 500).Result()
		if scanErr != nil {
			return scanErr
		}
		for _, key := range keys {
			id, ok := parseOAuthFleetSoft429Key(key)
			if !ok {
				continue
			}
			pttl, pttlErr := c.rdb.PTTL(ctx, key).Result()
			if pttlErr != nil {
				return pttlErr
			}
			if pttl <= 0 {
				_ = c.rdb.Del(ctx, key).Err()
				continue
			}
			expireAt := oauthFleetSoft429Now().Add(pttl).UnixMilli()
			if err := c.rdb.ZAdd(ctx, oauthFleetSoft429ExpKey, redis.Z{
				Score:  float64(expireAt),
				Member: strconv.FormatInt(id, 10),
			}).Err(); err != nil {
				return err
			}
			if err := c.rdb.Del(ctx, key).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return c.rdb.Set(ctx, oauthFleetSoft429MigratedKey, "1", 0).Err()
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
