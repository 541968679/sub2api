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
	publicScheduleQualityKeyPrefix = "public-schedule:quality:"
	publicScheduleSoftKeyPrefix    = "public-schedule:soft:"
	publicScheduleStateKeyPrefix   = "public-schedule:state:"
	publicScheduleQualityTTL       = 7 * 24 * time.Hour
)

type redisPublicScheduleQualityCache struct {
	rdb *redis.Client
}

type publicScheduleStateRecord struct {
	State     string `json:"state"`
	UntilUnix int64  `json:"until_unix,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Soft      bool   `json:"soft,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

func NewPublicScheduleQualityCache(rdb *redis.Client) service.PublicScheduleQualityCache {
	return &redisPublicScheduleQualityCache{rdb: rdb}
}

func publicScheduleQualityKey(accountID int64) string {
	return publicScheduleQualityKeyPrefix + strconv.FormatInt(accountID, 10)
}

func publicScheduleSoftKey(accountID int64) string {
	return publicScheduleSoftKeyPrefix + strconv.FormatInt(accountID, 10)
}

func publicScheduleStateKey(accountID int64) string {
	return publicScheduleStateKeyPrefix + strconv.FormatInt(accountID, 10)
}

func (c *redisPublicScheduleQualityCache) GetWindow(ctx context.Context, accountID int64) *service.PairQualityLive {
	return c.getLive(ctx, publicScheduleQualityKey(accountID))
}

func (c *redisPublicScheduleQualityCache) StoreWindow(ctx context.Context, accountID int64, live *service.PairQualityLive) {
	c.storeLive(ctx, publicScheduleQualityKey(accountID), live, publicScheduleQualityTTL)
}

func (c *redisPublicScheduleQualityCache) GetSoft(ctx context.Context, accountID int64) *service.PairQualityLive {
	return c.getLive(ctx, publicScheduleSoftKey(accountID))
}

func (c *redisPublicScheduleQualityCache) StoreSoft(ctx context.Context, accountID int64, live *service.PairQualityLive) {
	c.storeLive(ctx, publicScheduleSoftKey(accountID), live, publicScheduleQualityTTL)
}

func (c *redisPublicScheduleQualityCache) ClearSoft(ctx context.Context, accountID int64) {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return
	}
	_ = c.rdb.Del(ctx, publicScheduleSoftKey(accountID)).Err()
}

func (c *redisPublicScheduleQualityCache) GetState(ctx context.Context, accountID int64) *service.PublicScheduleRuntimeState {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return nil
	}
	raw, err := c.rdb.Get(ctx, publicScheduleStateKey(accountID)).Bytes()
	if err != nil || len(raw) == 0 {
		return nil
	}
	return decodePublicScheduleState(raw)
}

func (c *redisPublicScheduleQualityCache) GetStateBatch(ctx context.Context, accountIDs []int64) map[int64]*service.PublicScheduleRuntimeState {
	out := map[int64]*service.PublicScheduleRuntimeState{}
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
		cmds[i] = pipe.Get(ctx, publicScheduleStateKey(id))
	}
	_, _ = pipe.Exec(ctx)
	for i, cmd := range cmds {
		raw, err := cmd.Bytes()
		if err != nil || len(raw) == 0 {
			continue
		}
		if st := decodePublicScheduleState(raw); st != nil {
			out[ids[i]] = st
		}
	}
	return out
}

func (c *redisPublicScheduleQualityCache) TryStartCooldown(ctx context.Context, accountID int64, until time.Time, reason string, soft bool) bool {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return false
	}
	now := time.Now().UTC()
	if current := c.GetState(ctx, accountID); current != nil {
		switch current.Normalized(now) {
		case service.PublicScheduleStateCooling, service.PublicScheduleStatePaused, service.PublicScheduleStatePinned:
			return false
		}
	}
	ttl := time.Until(until)
	if ttl < time.Second {
		ttl = time.Second
	}
	payload, err := json.Marshal(encodePublicScheduleState(&service.PublicScheduleRuntimeState{
		State:     service.PublicScheduleStateCooling,
		Until:     until.UTC(),
		Reason:    reason,
		Soft:      soft,
		UpdatedAt: now,
	}))
	if err != nil {
		return false
	}
	// No-extend is the cooling-state check above, not SETNX: probing must
	// be allowed to re-enter cooling on the same key.
	if err := c.rdb.Set(ctx, publicScheduleStateKey(accountID), payload, ttl+time.Minute).Err(); err != nil {
		return false
	}
	return true
}

func (c *redisPublicScheduleQualityCache) SetState(ctx context.Context, accountID int64, state *service.PublicScheduleRuntimeState) {
	if c == nil || c.rdb == nil || accountID <= 0 || state == nil {
		return
	}
	payload, err := json.Marshal(encodePublicScheduleState(state))
	if err != nil {
		return
	}
	ttl := publicScheduleQualityTTL
	switch state.State {
	case service.PublicScheduleStatePinned, service.PublicScheduleStatePaused:
		_ = c.rdb.Set(ctx, publicScheduleStateKey(accountID), payload, 0).Err()
		return
	case service.PublicScheduleStateCooling, service.PublicScheduleStateResumed:
		if !state.Until.IsZero() {
			if untilTTL := time.Until(state.Until); untilTTL > 0 {
				ttl = untilTTL + time.Minute
			}
		}
	}
	_ = c.rdb.Set(ctx, publicScheduleStateKey(accountID), payload, ttl).Err()
}

func (c *redisPublicScheduleQualityCache) ClearState(ctx context.Context, accountID int64) {
	if c == nil || c.rdb == nil || accountID <= 0 {
		return
	}
	_ = c.rdb.Del(ctx, publicScheduleStateKey(accountID)).Err()
}

func (c *redisPublicScheduleQualityCache) getLive(ctx context.Context, key string) *service.PairQualityLive {
	if c == nil || c.rdb == nil || key == "" {
		return nil
	}
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil || len(raw) == 0 {
		return nil
	}
	var live service.PairQualityLive
	if err := json.Unmarshal(raw, &live); err != nil {
		return nil
	}
	return &live
}

func (c *redisPublicScheduleQualityCache) storeLive(ctx context.Context, key string, live *service.PairQualityLive, ttl time.Duration) {
	if c == nil || c.rdb == nil || live == nil || key == "" {
		return
	}
	payload, err := json.Marshal(live)
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, key, payload, ttl).Err()
}

func encodePublicScheduleState(state *service.PublicScheduleRuntimeState) publicScheduleStateRecord {
	if state == nil {
		return publicScheduleStateRecord{State: service.PublicScheduleStateSelectable}
	}
	out := publicScheduleStateRecord{
		State:  state.State,
		Reason: state.Reason,
		Soft:   state.Soft,
	}
	if !state.Until.IsZero() {
		out.UntilUnix = state.Until.UTC().Unix()
	}
	if !state.UpdatedAt.IsZero() {
		out.UpdatedAt = state.UpdatedAt.UTC().Unix()
	}
	return out
}

func decodePublicScheduleState(raw []byte) *service.PublicScheduleRuntimeState {
	var rec publicScheduleStateRecord
	if err := json.Unmarshal(raw, &rec); err != nil || rec.State == "" {
		return nil
	}
	st := &service.PublicScheduleRuntimeState{
		State:  rec.State,
		Reason: rec.Reason,
		Soft:   rec.Soft,
	}
	if rec.UntilUnix > 0 {
		st.Until = time.Unix(rec.UntilUnix, 0).UTC()
	}
	if rec.UpdatedAt > 0 {
		st.UpdatedAt = time.Unix(rec.UpdatedAt, 0).UTC()
	}
	return st
}
