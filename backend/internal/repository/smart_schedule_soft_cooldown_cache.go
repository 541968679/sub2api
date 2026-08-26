package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const smartScheduleSoftCoolKeyPrefix = "smart-schedule:soft-cool:"

func smartScheduleSoftCoolKey(platform string, accountID int64) string {
	return smartScheduleSoftCoolKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
}

func (c *userSmartScheduleCache) GetSoftCooldown(ctx context.Context, accountID, userID int64, platform string) *service.PairQualityLive {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	raw, err := c.rdb.HGet(ctx, smartScheduleSoftCoolKey(platform, accountID), smartScheduleCooldownField(userID)).Bytes()
	if err != nil || len(raw) == 0 {
		return nil
	}
	return decodePairQualityLive(raw)
}

func (c *userSmartScheduleCache) GetSoftCooldownBatch(ctx context.Context, accountIDs []int64, userID int64, platform string) map[int64]*service.PairQualityLive {
	out := map[int64]*service.PairQualityLive{}
	if c == nil || c.rdb == nil || userID <= 0 || len(accountIDs) == 0 {
		return out
	}
	ids := uniquePositiveIDs(accountIDs)
	if len(ids) == 0 {
		return out
	}
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, accountID := range ids {
		cmds[i] = pipe.HGet(ctx, smartScheduleSoftCoolKey(platform, accountID), smartScheduleCooldownField(userID))
	}
	_, _ = pipe.Exec(ctx)
	for i, cmd := range cmds {
		raw, err := cmd.Bytes()
		if err != nil || len(raw) == 0 {
			continue
		}
		if live := decodePairQualityLive(raw); live != nil {
			out[ids[i]] = live
		}
	}
	return out
}

func (c *userSmartScheduleCache) IngestSoftCooldown(ctx context.Context, accountID, userID int64, platform string, nTTFT, nOK int, success bool, firstTokenMs, durationMs *int, minutes int) *service.PairQualityLive {
	if c == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	live := c.GetSoftCooldown(ctx, accountID, userID, platform)
	live = applyPairQualityIngestProxy(live, nTTFT, nOK, success, firstTokenMs, durationMs)
	c.storeSoftCooldown(ctx, accountID, userID, platform, live, minutes)
	return live
}

func (c *userSmartScheduleCache) ZeroSoftCooldown(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	_ = c.rdb.HDel(ctx, smartScheduleSoftCoolKey(platform, accountID), smartScheduleCooldownField(userID)).Err()
}

func (c *userSmartScheduleCache) storeSoftCooldown(ctx context.Context, accountID, userID int64, platform string, live *service.PairQualityLive, minutes int) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 || live == nil {
		return
	}
	payload, err := json.Marshal(encodePairQualityLive(live))
	if err != nil {
		return
	}
	key := smartScheduleSoftCoolKey(platform, accountID)
	if err := c.rdb.HSet(ctx, key, smartScheduleCooldownField(userID), payload).Err(); err != nil {
		return
	}
	minutes = service.ClampSmartScheduleCooldownMinutes(minutes)
	c.extendCooldownTTL(ctx, key, time.Duration(minutes)*time.Minute+smartScheduleCooldownTTLBuffer)
}
