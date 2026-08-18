package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	accountQualityLiveKeyPrefix         = "account-quality:live:"
	accountQualityResumeKeyPrefix       = "account-quality:resume:"
	accountQualityLiveTTL               = 20 * time.Minute
	// Resume HASH must outlive 已恢复 (15m) plus the following accumulation window (15m).
	accountQualityResumeTTL             = 40 * time.Minute
	accountQualityResumeAccountField       = "a"
	accountQualityResumeUserFieldPrefix    = "u:"
	accountQualityResumeWatchingFieldPrefix = "w:"
)

type accountQualityLiveCache struct {
	rdb *redis.Client
}

// NewAccountQualityLiveCache persists the live 15-minute quality window for selection.
func NewAccountQualityLiveCache(rdb *redis.Client) service.AccountQualityLiveCache {
	return &accountQualityLiveCache{rdb: rdb}
}

func accountQualityLiveKey(accountID int64) string {
	return accountQualityLiveKeyPrefix + strconv.FormatInt(accountID, 10)
}

func accountQualityResumeKey(accountID int64) string {
	return accountQualityResumeKeyPrefix + strconv.FormatInt(accountID, 10)
}

func accountQualityResumeUserField(userID int64) string {
	return accountQualityResumeUserFieldPrefix + strconv.FormatInt(userID, 10)
}

func accountQualityResumeWatchingField(userID int64) string {
	return accountQualityResumeWatchingFieldPrefix + strconv.FormatInt(userID, 10)
}

func accountIDFromLiveKey(key string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(key, accountQualityLiveKeyPrefix), 10, 64)
	return id, err == nil && id > 0
}

func (c *accountQualityLiveCache) Get(ctx context.Context, accountID int64) (*service.AccountQualityStats, error) {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return nil, nil
	}
	live, err := c.getLiveJSON(ctx, accountID)
	if err != nil {
		return nil, err
	}
	overlay, err := c.getResumeOverlay(ctx, accountID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if live != nil && service.HasActiveQualityResume(live, now) {
		if err := c.migrateLiveResumeToHash(ctx, accountID, live); err != nil {
			return nil, err
		}
	}
	if live == nil {
		if !service.HasActiveQualityResume(overlay, now) {
			return nil, nil
		}
		live = &service.AccountQualityStats{WindowSeconds: service.AccountQualityWindowSeconds}
	}
	service.MergeQualityResume(live, overlay, now)
	return live, nil
}

func (c *accountQualityLiveCache) getLiveJSON(ctx context.Context, accountID int64) (*service.AccountQualityStats, error) {
	val, err := c.rdb.Get(ctx, accountQualityLiveKey(accountID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return unmarshalLiveQuality(val)
}

func unmarshalLiveQuality(val string) (*service.AccountQualityStats, error) {
	if val == "" {
		return nil, nil
	}
	var stats service.AccountQualityStats
	if err := json.Unmarshal([]byte(val), &stats); err != nil {
		return nil, fmt.Errorf("unmarshal live quality: %w", err)
	}
	service.NormalizeAccountQualityRates(&stats)
	return &stats, nil
}

func (c *accountQualityLiveCache) getResumeOverlay(ctx context.Context, accountID int64) (*service.AccountQualityStats, error) {
	vals, err := c.rdb.HGetAll(ctx, accountQualityResumeKey(accountID)).Result()
	if err != nil {
		return nil, err
	}
	return parseResumeHash(vals), nil
}

func parseResumeHash(vals map[string]string) *service.AccountQualityStats {
	if len(vals) == 0 {
		return nil
	}
	st := &service.AccountQualityStats{WindowSeconds: service.AccountQualityWindowSeconds}
	for key, raw := range vals {
		until, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || until <= 0 {
			continue
		}
		if key == accountQualityResumeAccountField {
			copied := until
			st.AccountResumeUntil = &copied
			continue
		}
		if strings.HasPrefix(key, accountQualityResumeWatchingFieldPrefix) {
			userID, err := strconv.ParseInt(strings.TrimPrefix(key, accountQualityResumeWatchingFieldPrefix), 10, 64)
			if err != nil || userID <= 0 {
				continue
			}
			service.SetUserQualityWatching(st, userID, time.Unix(until, 0).UTC())
			continue
		}
		if !strings.HasPrefix(key, accountQualityResumeUserFieldPrefix) {
			continue
		}
		userID, err := strconv.ParseInt(strings.TrimPrefix(key, accountQualityResumeUserFieldPrefix), 10, 64)
		if err != nil || userID <= 0 {
			continue
		}
		service.SetUserQualityResume(st, userID, time.Unix(until, 0).UTC())
	}
	if st.AccountResumeUntil == nil && len(st.ResumeUsers) == 0 && len(st.ResumeWatchingUsers) == 0 {
		return nil
	}
	return st
}

func (c *accountQualityLiveCache) MarkUserResume(ctx context.Context, accountID, userID int64) error {
	if userID <= 0 {
		return nil
	}
	now := time.Now().UTC()
	return c.writeResumeFields(ctx, accountID, map[string]any{
		accountQualityResumeUserField(userID):     now.Add(service.AccountQualityWindow).Unix(),
		accountQualityResumeWatchingField(userID): now.Add(2 * service.AccountQualityWindow).Unix(),
	}, nil)
}

func (c *accountQualityLiveCache) MarkUserQualityWindow(ctx context.Context, accountID, userID int64) error {
	if userID <= 0 {
		return nil
	}
	now := time.Now().UTC()
	return c.writeResumeFields(ctx, accountID, map[string]any{
		accountQualityResumeWatchingField(userID): now.Add(service.AccountQualityWindow).Unix(),
	}, []string{accountQualityResumeUserField(userID)})
}

func (c *accountQualityLiveCache) ClearUserResume(ctx context.Context, accountID, userID int64) error {
	if userID <= 0 {
		return nil
	}
	return c.writeResumeFields(ctx, accountID, nil, []string{
		accountQualityResumeUserField(userID),
		accountQualityResumeWatchingField(userID),
	})
}

func (c *accountQualityLiveCache) MarkAccountResume(ctx context.Context, accountID int64) error {
	return c.writeResumeField(ctx, accountID, accountQualityResumeAccountField, time.Now().UTC().Add(service.AccountQualityWindow).Unix())
}

func (c *accountQualityLiveCache) writeResumeField(ctx context.Context, accountID int64, field string, until int64) error {
	return c.writeResumeFields(ctx, accountID, map[string]any{field: until}, nil)
}

func (c *accountQualityLiveCache) writeResumeFields(ctx context.Context, accountID int64, set map[string]any, del []string) error {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return nil
	}
	if len(set) == 0 && len(del) == 0 {
		return nil
	}
	key := accountQualityResumeKey(accountID)
	pipe := c.rdb.TxPipeline()
	if len(set) > 0 {
		pipe.HSet(ctx, key, set)
	}
	if len(del) > 0 {
		pipe.HDel(ctx, key, del...)
	}
	pipe.Expire(ctx, key, accountQualityResumeTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *accountQualityLiveCache) migrateLiveResumeToHash(ctx context.Context, accountID int64, live *service.AccountQualityStats) error {
	if c == nil || c.rdb == nil || accountID <= 0 || live == nil {
		return nil
	}
	key := accountQualityResumeKey(accountID)
	pipe := c.rdb.TxPipeline()
	wrote := false
	if live.AccountResumeUntil != nil && *live.AccountResumeUntil > 0 {
		pipe.HSetNX(ctx, key, accountQualityResumeAccountField, *live.AccountResumeUntil)
		wrote = true
	}
	for userKey, until := range live.ResumeUsers {
		if until <= 0 || userKey == "" {
			continue
		}
		pipe.HSetNX(ctx, key, accountQualityResumeUserFieldPrefix+userKey, until)
		wrote = true
	}
	if !wrote {
		return nil
	}
	pipe.Expire(ctx, key, accountQualityResumeTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func statsWithoutResume(st *service.AccountQualityStats) *service.AccountQualityStats {
	if st == nil {
		return &service.AccountQualityStats{WindowSeconds: service.AccountQualityWindowSeconds}
	}
	cp := *st
	cp.ResumeUsers = nil
	cp.ResumeWatchingUsers = nil
	cp.AccountResumeUntil = nil
	return &cp
}

func (c *accountQualityLiveCache) Replace(ctx context.Context, stats map[int64]*service.AccountQualityStats) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	now := time.Now().UTC()
	keep := make(map[string]struct{}, len(stats))
	for accountID, st := range stats {
		if accountID <= 0 {
			continue
		}
		liveKey := accountQualityLiveKey(accountID)
		existing, err := c.getLiveJSON(ctx, accountID)
		if err != nil {
			return err
		}
		overlay, err := c.getResumeOverlay(ctx, accountID)
		if err != nil {
			return err
		}
		if st == nil {
			st = &service.AccountQualityStats{WindowSeconds: service.AccountQualityWindowSeconds}
			stats[accountID] = st
		}
		service.MergeQualityResume(st, existing, now)
		service.MergeQualityResume(st, overlay, now)
		if existing != nil && service.HasActiveQualityResume(existing, now) {
			if err := c.migrateLiveResumeToHash(ctx, accountID, existing); err != nil {
				return err
			}
		}
		if !service.HasAccountQualitySamples(st) {
			if err := c.rdb.Del(ctx, liveKey).Err(); err != nil {
				return err
			}
			continue
		}
		payload, err := json.Marshal(statsWithoutResume(st))
		if err != nil {
			return fmt.Errorf("marshal live quality: %w", err)
		}
		if err := c.rdb.Set(ctx, liveKey, payload, accountQualityLiveTTL).Err(); err != nil {
			return err
		}
		keep[liveKey] = struct{}{}
	}
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, accountQualityLiveKeyPrefix+"*", 128).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			if _, ok := keep[key]; ok {
				continue
			}
			if id, ok := accountIDFromLiveKey(key); ok {
				existing, getErr := c.getLiveJSON(ctx, id)
				if getErr != nil {
					return getErr
				}
				if existing != nil && service.HasActiveQualityResume(existing, now) {
					if err := c.migrateLiveResumeToHash(ctx, id, existing); err != nil {
						return err
					}
				}
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
	return nil
}
