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
	smartSchedulePairQualityKeyPrefix = "smart-schedule:pair-quality:"
	smartSchedulePairTrendKeyPrefix   = "smart-schedule:pair-quality-trend:"
	smartSchedulePairEventKeyPrefix   = "smart-schedule:pair-quality-events:"
	smartSchedulePairQualityTTL       = 7 * 24 * time.Hour
	smartSchedulePairTrendTTL         = 24 * time.Hour
	smartSchedulePairEventTTL         = 7 * 24 * time.Hour
	smartSchedulePairTrendMax         = 200
	smartSchedulePairEventMax         = 200
)

type pairQualityRedisState struct {
	N         int      `json:"n"`
	TTFT      []int    `json:"ttft"`
	OK        []uint8  `json:"ok"`
	P50       *int     `json:"p50_ttft_ms,omitempty"`
	Rate      *float64 `json:"success_rate,omitempty"`
	UpdatedAt int64    `json:"updated_at"`
}

func smartSchedulePairQualityKey(platform string, accountID int64) string {
	return smartSchedulePairQualityKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
}

func smartSchedulePairTrendKey(platform string, accountID, userID int64) string {
	return smartSchedulePairTrendKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10) + ":" + strconv.FormatInt(userID, 10)
}

func smartSchedulePairEventKey(platform string, accountID, userID int64) string {
	return smartSchedulePairEventKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10) + ":" + strconv.FormatInt(userID, 10)
}

func (c *userSmartScheduleCache) GetPairQuality(ctx context.Context, accountID, userID int64, platform string) *service.PairQualityLive {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	raw, err := c.rdb.HGet(ctx, smartSchedulePairQualityKey(platform, accountID), smartScheduleCooldownField(userID)).Bytes()
	if err != nil || len(raw) == 0 {
		return nil
	}
	return decodePairQualityLive(raw)
}

func (c *userSmartScheduleCache) GetPairQualityBatch(ctx context.Context, accountIDs []int64, userID int64, platform string) map[int64]*service.PairQualityLive {
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
		cmds[i] = pipe.HGet(ctx, smartSchedulePairQualityKey(platform, accountID), smartScheduleCooldownField(userID))
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

func (c *userSmartScheduleCache) IngestPairQuality(ctx context.Context, accountID, userID int64, platform string, n int, success bool, firstTokenMs *int) *service.PairQualityLive {
	if c == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	live := c.GetPairQuality(ctx, accountID, userID, platform)
	live = applyPairQualityIngestProxy(live, n, success, firstTokenMs)
	c.storePairQuality(ctx, accountID, userID, platform, live)
	c.appendPairQualitySnapshot(ctx, accountID, userID, platform, live.Snapshot())
	return live
}

func (c *userSmartScheduleCache) ZeroPairQuality(ctx context.Context, accountID, userID int64, platform string, eventType string) {
	if c == nil || accountID <= 0 || userID <= 0 {
		return
	}
	live := serviceZeroPairQualityLive(c.GetPairQuality(ctx, accountID, userID, platform))
	c.storePairQuality(ctx, accountID, userID, platform, live)
	if eventType != "" {
		c.AppendPairQualityEvent(ctx, accountID, userID, platform, service.PairQualityEvent{
			Ts:   time.Now().UTC().Unix(),
			Type: eventType,
		})
	}
}

func (c *userSmartScheduleCache) ListPairQualitySnapshots(ctx context.Context, accountID, userID int64, platform string, limit int) []service.PairQualitySnapshot {
	return listPairQualityJSON[service.PairQualitySnapshot](c, ctx, smartSchedulePairTrendKey(platform, accountID, userID), limit)
}

func (c *userSmartScheduleCache) ListPairQualityEvents(ctx context.Context, accountID, userID int64, platform string, limit int) []service.PairQualityEvent {
	return listPairQualityJSON[service.PairQualityEvent](c, ctx, smartSchedulePairEventKey(platform, accountID, userID), limit)
}

func (c *userSmartScheduleCache) AppendPairQualityEvent(ctx context.Context, accountID, userID int64, platform string, event service.PairQualityEvent) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 || event.Type == "" {
		return
	}
	if event.Ts == 0 {
		event.Ts = time.Now().UTC().Unix()
	}
	appendPairQualityList(c, ctx, smartSchedulePairEventKey(platform, accountID, userID), event, smartSchedulePairEventMax, smartSchedulePairEventTTL)
}

func (c *userSmartScheduleCache) storePairQuality(ctx context.Context, accountID, userID int64, platform string, live *service.PairQualityLive) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 || live == nil {
		return
	}
	payload, err := json.Marshal(encodePairQualityLive(live))
	if err != nil {
		return
	}
	key := smartSchedulePairQualityKey(platform, accountID)
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, key, smartScheduleCooldownField(userID), payload)
	pipe.Expire(ctx, key, smartSchedulePairQualityTTL)
	_, _ = pipe.Exec(ctx)
}

func (c *userSmartScheduleCache) appendPairQualitySnapshot(ctx context.Context, accountID, userID int64, platform string, snap service.PairQualitySnapshot) {
	appendPairQualityList(c, ctx, smartSchedulePairTrendKey(platform, accountID, userID), snap, smartSchedulePairTrendMax, smartSchedulePairTrendTTL)
}

func appendPairQualityList(c *userSmartScheduleCache, ctx context.Context, key string, value any, max int, ttl time.Duration) {
	if c == nil || c.rdb == nil || key == "" {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	pipe := c.rdb.Pipeline()
	pipe.RPush(ctx, key, payload)
	if max > 0 {
		pipe.LTrim(ctx, key, int64(-max), -1)
	}
	pipe.Expire(ctx, key, ttl)
	_, _ = pipe.Exec(ctx)
}

func listPairQualityJSON[T any](c *userSmartScheduleCache, ctx context.Context, key string, limit int) []T {
	if c == nil || c.rdb == nil || key == "" {
		return nil
	}
	if limit <= 0 || limit > smartSchedulePairEventMax {
		limit = smartSchedulePairEventMax
	}
	raw, err := c.rdb.LRange(ctx, key, -int64(limit), -1).Result()
	if err != nil || len(raw) == 0 {
		return nil
	}
	out := make([]T, 0, len(raw))
	for _, item := range raw {
		var parsed T
		if json.Unmarshal([]byte(item), &parsed) != nil {
			continue
		}
		out = append(out, parsed)
	}
	return out
}

func encodePairQualityLive(live *service.PairQualityLive) pairQualityRedisState {
	if live == nil {
		return pairQualityRedisState{N: service.DefaultSmartScheduleWindowN}
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
	return pairQualityRedisState{
		N:         live.N,
		TTFT:      append([]int(nil), live.TTFTMs...),
		OK:        ok,
		P50:       live.P50TTFTMs,
		Rate:      live.SuccessRate,
		UpdatedAt: updated,
	}
}

func decodePairQualityLive(raw []byte) *service.PairQualityLive {
	var stored pairQualityRedisState
	if json.Unmarshal(raw, &stored) != nil {
		return nil
	}
	ok := make([]bool, len(stored.OK))
	for i, v := range stored.OK {
		ok[i] = v != 0
	}
	live := &service.PairQualityLive{
		N:           stored.N,
		TTFTMs:      append([]int(nil), stored.TTFT...),
		OK:          ok,
		P50TTFTMs:   stored.P50,
		SuccessRate: stored.Rate,
	}
	if stored.UpdatedAt > 0 {
		live.UpdatedAt = time.Unix(stored.UpdatedAt, 0).UTC()
	}
	serviceRecomputePairQuality(live)
	return live
}

func uniquePositiveIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// Local wrappers keep ingest/zero math in the service package without exporting internals.
func applyPairQualityIngestProxy(live *service.PairQualityLive, n int, success bool, firstTokenMs *int) *service.PairQualityLive {
	return service.ApplyPairQualityIngest(live, n, success, firstTokenMs)
}

func serviceZeroPairQualityLive(live *service.PairQualityLive) *service.PairQualityLive {
	n := service.DefaultSmartScheduleWindowN
	if live != nil && live.N >= service.MinSmartScheduleWindowN {
		n = live.N
	}
	return service.ZeroPairQualityLive(n)
}

func serviceRecomputePairQuality(live *service.PairQualityLive) {
	service.RecomputePairQuality(live)
}
