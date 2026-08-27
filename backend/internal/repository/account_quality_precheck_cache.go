package repository

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	accountQualityPrecheckKeyPrefix = "account-quality:precheck:"
	accountQualityPrecheckCap       = 400
	accountQualityPrecheckTTL       = 48 * time.Hour
)

type accountQualityPrecheckStored struct {
	TS  int64 `json:"ts"`
	UID int64 `json:"uid"`
	OK  bool  `json:"ok"`
	T   *int  `json:"t,omitempty"`
	D   *int  `json:"d,omitempty"`
}

func accountQualityPrecheckKey(accountID int64) string {
	return accountQualityPrecheckKeyPrefix + strconv.FormatInt(accountID, 10)
}

func (c *accountQualityLiveCache) IngestPrecheckSample(ctx context.Context, accountID, userID int64, success bool, firstTokenMs, durationMs *int) {
	ingestAccountQualityPrecheckSample(ctx, c.rdb, accountID, userID, success, firstTokenMs, durationMs)
}

func ingestAccountQualityPrecheckSample(ctx context.Context, rdb *redis.Client, accountID, userID int64, success bool, firstTokenMs, durationMs *int) {
	if rdb == nil || accountID <= 0 {
		return
	}
	raw, err := json.Marshal(accountQualityPrecheckStored{
		TS:  time.Now().UTC().Unix(),
		UID: userID,
		OK:  success,
		T:   firstTokenMs,
		D:   durationMs,
	})
	if err != nil {
		return
	}
	key := accountQualityPrecheckKey(accountID)
	pipe := rdb.Pipeline()
	pipe.LPush(ctx, key, raw)
	pipe.LTrim(ctx, key, 0, int64(accountQualityPrecheckCap-1))
	pipe.Expire(ctx, key, accountQualityPrecheckTTL)
	_, _ = pipe.Exec(ctx)
}

func listAccountQualityPrecheckSamples(ctx context.Context, rdb *redis.Client, accountID, excludeUserID int64, since time.Time) []service.AccountQualityPrecheckSample {
	if rdb == nil || accountID <= 0 {
		return nil
	}
	raws, err := rdb.LRange(ctx, accountQualityPrecheckKey(accountID), 0, -1).Result()
	if err != nil || len(raws) == 0 {
		return nil
	}
	// LIST is newest-first; convert to oldest-first before filter.
	oldestFirst := make([]service.AccountQualityPrecheckSample, 0, len(raws))
	for i := len(raws) - 1; i >= 0; i-- {
		var stored accountQualityPrecheckStored
		if json.Unmarshal([]byte(raws[i]), &stored) != nil {
			continue
		}
		oldestFirst = append(oldestFirst, service.AccountQualityPrecheckSample{
			UnixTS:     stored.TS,
			UserID:     stored.UID,
			OK:         stored.OK,
			TTFTMs:     stored.T,
			DurationMs: stored.D,
		})
	}
	return service.FilterPrecheckSamples(oldestFirst, excludeUserID, since)
}
