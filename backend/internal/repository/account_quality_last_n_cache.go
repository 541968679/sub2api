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
	accountQualityLastNKeyPrefix = "account-quality:last-n:"
	accountQualityLastNTTL       = 7 * 24 * time.Hour
)

type accountQualityLastNState struct {
	N           int      `json:"n"`
	UseFailover bool     `json:"use_failover,omitempty"`
	TTFT        []int    `json:"ttft"`
	OK          []uint8  `json:"ok"`
	P50         *int     `json:"p50_ttft_ms,omitempty"`
	Rate        *float64 `json:"success_rate,omitempty"`
	UpdatedAt   int64    `json:"updated_at"`
	OverrideN   *int     `json:"override_n,omitempty"`
}

func accountQualityLastNKey(accountID int64) string {
	return accountQualityLastNKeyPrefix + strconv.FormatInt(accountID, 10)
}

func accountIDFromLastNKey(key string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimPrefix(key, accountQualityLastNKeyPrefix), 10, 64)
	return id, err == nil && id > 0
}

func (c *accountQualityLiveCache) GetLastN(ctx context.Context, accountID int64) *service.AccountQualityLastN {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return nil
	}
	raw, err := c.rdb.Get(ctx, accountQualityLastNKey(accountID)).Bytes()
	if err != nil || len(raw) == 0 {
		return nil
	}
	return decodeAccountQualityLastN(raw)
}

func (c *accountQualityLiveCache) GetLastNBatch(ctx context.Context, accountIDs []int64) map[int64]*service.AccountQualityLastN {
	out := map[int64]*service.AccountQualityLastN{}
	if c == nil || c.rdb == nil || len(accountIDs) == 0 {
		return out
	}
	ids := uniquePositiveIDs(accountIDs)
	if len(ids) == 0 {
		return out
	}
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.Get(ctx, accountQualityLastNKey(id))
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

func (c *accountQualityLiveCache) IngestLastN(ctx context.Context, accountID int64, n int, success bool, firstTokenMs *int, useFailover bool) *service.AccountQualityLastN {
	if c == nil || accountID <= 0 {
		return nil
	}
	live := service.ApplyAccountQualityLastNIngest(c.GetLastN(ctx, accountID), n, success, firstTokenMs)
	live.UseFailover = useFailover
	c.storeLastN(ctx, accountID, live)
	c.writeLiveFromLastN(ctx, accountID, live)
	return live
}

func (c *accountQualityLiveCache) ListLastNAccountIDs(ctx context.Context) []int64 {
	if c == nil || c.rdb == nil {
		return nil
	}
	var (
		cursor uint64
		ids    []int64
	)
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, accountQualityLastNKeyPrefix+"*", 128).Result()
		if err != nil {
			return ids
		}
		for _, key := range keys {
			if id, ok := accountIDFromLastNKey(key); ok {
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

func (c *accountQualityLiveCache) storeLastN(ctx context.Context, accountID int64, live *service.AccountQualityLastN) {
	if c == nil || c.rdb == nil || accountID <= 0 || live == nil {
		return
	}
	payload, err := json.Marshal(encodeAccountQualityLastN(live))
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, accountQualityLastNKey(accountID), payload, accountQualityLastNTTL).Err()
}

func (c *accountQualityLiveCache) writeLiveFromLastN(ctx context.Context, accountID int64, live *service.AccountQualityLastN) {
	if c == nil || c.rdb == nil || accountID <= 0 || live == nil {
		return
	}
	stats := statsWithoutResume(live.ToAccountQualityStats())
	payload, err := json.Marshal(stats)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, accountQualityLiveKey(accountID), payload, accountQualityLastNTTL).Err()
}

func encodeAccountQualityLastN(live *service.AccountQualityLastN) accountQualityLastNState {
	if live == nil {
		return accountQualityLastNState{N: service.DefaultAccountQualityWindowN}
	}
	ok := make([]uint8, len(live.OK))
	for i, v := range live.OK {
		if v {
			ok[i] = 1
		}
	}
	updated := live.UpdatedAt.Unix()
	if live.UpdatedAt.IsZero() {
		updated = time.Now().UTC().Unix()
	}
	return accountQualityLastNState{
		N:           live.N,
		UseFailover: live.UseFailover,
		TTFT:        append([]int(nil), live.TTFTMs...),
		OK:          ok,
		P50:         live.P50TTFTMs,
		Rate:        live.SuccessRate,
		UpdatedAt:   updated,
		OverrideN:   live.OverrideN,
	}
}

func decodeAccountQualityLastN(raw []byte) *service.AccountQualityLastN {
	var stored accountQualityLastNState
	if json.Unmarshal(raw, &stored) != nil {
		return nil
	}
	ok := make([]bool, len(stored.OK))
	for i, v := range stored.OK {
		ok[i] = v != 0
	}
	live := &service.AccountQualityLastN{
		N:           stored.N,
		UseFailover: stored.UseFailover,
		TTFTMs:      append([]int(nil), stored.TTFT...),
		OK:          ok,
		P50TTFTMs:   stored.P50,
		SuccessRate: stored.Rate,
		OverrideN:   stored.OverrideN,
	}
	if stored.UpdatedAt > 0 {
		live.UpdatedAt = time.Unix(stored.UpdatedAt, 0).UTC()
	}
	service.RecomputeAccountQualityLastN(live)
	live.UseFailover = stored.UseFailover
	return live
}
