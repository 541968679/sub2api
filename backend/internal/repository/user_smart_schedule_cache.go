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
	smartScheduleUserKeyPrefix     = "smart-schedule:user:"
	smartScheduleCooldownKeyPrefix = "smart-schedule:cooldown:"
	smartScheduleCooldownFieldPref = "u:"
	smartScheduleCooldownTTLBuffer = 2 * time.Hour
)

type userSmartScheduleCache struct {
	rdb  *redis.Client
	repo service.UserSmartScheduleRepository
}

// NewUserSmartScheduleCache is the Redis + DB load-through cache for user smart schedule.
func NewUserSmartScheduleCache(rdb *redis.Client, repo service.UserSmartScheduleRepository) service.UserSmartScheduleCache {
	return &userSmartScheduleCache{rdb: rdb, repo: repo}
}

func smartScheduleUserKey(userID int64) string {
	return smartScheduleUserKeyPrefix + strconv.FormatInt(userID, 10)
}

func smartScheduleCooldownKey(accountID int64) string {
	return smartScheduleCooldownKeyPrefix + strconv.FormatInt(accountID, 10)
}

func smartScheduleCooldownField(userID int64) string {
	return smartScheduleCooldownFieldPref + strconv.FormatInt(userID, 10)
}

func (c *userSmartScheduleCache) Lookup(ctx context.Context, userID int64) *service.UserSmartScheduleBundle {
	if c == nil || userID <= 0 {
		return nil
	}
	if c.rdb != nil {
		raw, err := c.rdb.Get(ctx, smartScheduleUserKey(userID)).Bytes()
		if err == nil && len(raw) > 0 {
			var stored cachedSmartScheduleBundle
			if json.Unmarshal(raw, &stored) == nil {
				return stored.toBundle()
			}
		}
	}
	if c.repo == nil {
		return nil
	}
	bundle, err := c.repo.ListByUser(ctx, userID)
	if err != nil || bundle == nil {
		return nil
	}
	c.storeUserBundle(ctx, userID, bundle)
	return bundle
}

func (c *userSmartScheduleCache) Invalidate(ctx context.Context, userID int64) error {
	if c == nil || c.rdb == nil || userID <= 0 {
		return nil
	}
	return c.rdb.Del(ctx, smartScheduleUserKey(userID)).Err()
}

func (c *userSmartScheduleCache) CooldownActive(ctx context.Context, accountID, userID int64, now time.Time) bool {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return false
	}
	raw, err := c.rdb.HGet(ctx, smartScheduleCooldownKey(accountID), smartScheduleCooldownField(userID)).Result()
	if err != nil || raw == "" {
		return false
	}
	until, parseErr := strconv.ParseInt(raw, 10, 64)
	if parseErr != nil || until <= now.Unix() {
		if until > 0 && until <= now.Unix() {
			_ = c.rdb.HDel(ctx, smartScheduleCooldownKey(accountID), smartScheduleCooldownField(userID)).Err()
		}
		return false
	}
	return true
}

func (c *userSmartScheduleCache) StartCooldown(ctx context.Context, accountID, userID int64, minutes int, now time.Time) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	if minutes < service.MinSmartScheduleCooldownMinutes {
		minutes = service.DefaultSmartScheduleCooldownMinutes
	}
	if minutes > service.MaxSmartScheduleCooldownMinutes {
		minutes = service.MaxSmartScheduleCooldownMinutes
	}
	until := now.Add(time.Duration(minutes) * time.Minute).Unix()
	key := smartScheduleCooldownKey(accountID)
	field := smartScheduleCooldownField(userID)
	added, err := c.rdb.HSetNX(ctx, key, field, until).Result()
	if err != nil || !added {
		return
	}
	ttl := time.Duration(minutes)*time.Minute + smartScheduleCooldownTTLBuffer
	c.extendCooldownTTL(ctx, key, ttl)
}

// extendCooldownTTL only sets or lengthens the HASH TTL. A short cooldown
// for one user must not expire another user's longer field on the same account.
func (c *userSmartScheduleCache) extendCooldownTTL(ctx context.Context, key string, ttl time.Duration) {
	if c == nil || c.rdb == nil || ttl <= 0 {
		return
	}
	current, err := c.rdb.TTL(ctx, key).Result()
	if err != nil {
		return
	}
	if current < 0 || current < ttl {
		_ = c.rdb.Expire(ctx, key, ttl).Err()
	}
}

func (c *userSmartScheduleCache) ClearCooldown(ctx context.Context, accountID, userID int64) error {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	return c.rdb.HDel(ctx, smartScheduleCooldownKey(accountID), smartScheduleCooldownField(userID)).Err()
}

func (c *userSmartScheduleCache) GetCooldownUntilBatch(ctx context.Context, accountIDs []int64, userID int64, now time.Time) map[int64]time.Time {
	out := map[int64]time.Time{}
	if c == nil || c.rdb == nil || userID <= 0 || len(accountIDs) == 0 {
		return out
	}
	ids := make([]int64, 0, len(accountIDs))
	seen := map[int64]struct{}{}
	for _, accountID := range accountIDs {
		if accountID <= 0 {
			continue
		}
		if _, ok := seen[accountID]; ok {
			continue
		}
		seen[accountID] = struct{}{}
		ids = append(ids, accountID)
	}
	if len(ids) == 0 {
		return out
	}
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.StringCmd, len(ids))
	for i, accountID := range ids {
		cmds[i] = pipe.HGet(ctx, smartScheduleCooldownKey(accountID), smartScheduleCooldownField(userID))
	}
	_, _ = pipe.Exec(ctx)
	nowUnix := now.Unix()
	for i, cmd := range cmds {
		raw, err := cmd.Result()
		if err != nil || raw == "" {
			continue
		}
		until, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || until <= nowUnix {
			continue
		}
		out[ids[i]] = time.Unix(until, 0).UTC()
	}
	return out
}

func (c *userSmartScheduleCache) storeUserBundle(ctx context.Context, userID int64, bundle *service.UserSmartScheduleBundle) {
	if c == nil || c.rdb == nil || bundle == nil {
		return
	}
	payload, err := json.Marshal(cachedSmartScheduleBundleFrom(bundle))
	if err != nil {
		return
	}
	_ = c.rdb.Set(ctx, smartScheduleUserKey(userID), payload, service.SmartScheduleUserCacheTTL).Err()
}

type cachedSmartScheduleMember struct {
	AccountID int64 `json:"account_id"`
	Cap       int   `json:"cap,omitempty"`
}

type cachedSmartSchedulePolicy struct {
	Enabled                  bool                        `json:"enabled"`
	QualityMaxP50TTFTMs      *int                        `json:"quality_max_p50_ttft_ms,omitempty"`
	QualityMinSuccessRate    *float64                    `json:"quality_min_success_rate,omitempty"`
	QualityMinSuccessSamples *int                        `json:"quality_min_success_samples,omitempty"`
	QualityMinTTFTSamples    *int                        `json:"quality_min_ttft_samples,omitempty"`
	QualityCondition         *string                     `json:"quality_condition,omitempty"`
	CooldownMinutes          int                         `json:"cooldown_minutes"`
	Members                  []cachedSmartScheduleMember `json:"members,omitempty"`
}

type cachedSmartScheduleBundle struct {
	Policies map[string]cachedSmartSchedulePolicy `json:"policies"`
}

func cachedSmartScheduleBundleFrom(bundle *service.UserSmartScheduleBundle) cachedSmartScheduleBundle {
	out := cachedSmartScheduleBundle{Policies: map[string]cachedSmartSchedulePolicy{}}
	if bundle == nil {
		return out
	}
	for platform, policy := range bundle.Policies {
		if policy == nil {
			continue
		}
		row := cachedSmartSchedulePolicy{
			Enabled:                  policy.Enabled,
			QualityMaxP50TTFTMs:      policy.QualityMaxP50TTFTMs,
			QualityMinSuccessRate:    policy.QualityMinSuccessRate,
			QualityMinSuccessSamples: policy.QualityMinSuccessSamples,
			QualityMinTTFTSamples:    policy.QualityMinTTFTSamples,
			QualityCondition:         policy.QualityCondition,
			CooldownMinutes:          policy.CooldownMinutes,
		}
		for accountID := range policy.AccountIDs {
			row.Members = append(row.Members, cachedSmartScheduleMember{
				AccountID: accountID,
				Cap:       policy.PairCap(accountID),
			})
		}
		out.Policies[platform] = row
	}
	return out
}

func (b cachedSmartScheduleBundle) toBundle() *service.UserSmartScheduleBundle {
	out := &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}
	for platform, row := range b.Policies {
		policy := &service.SmartSchedulePlatformPolicy{
			Enabled:                  row.Enabled,
			QualityMaxP50TTFTMs:      row.QualityMaxP50TTFTMs,
			QualityMinSuccessRate:    row.QualityMinSuccessRate,
			QualityMinSuccessSamples: row.QualityMinSuccessSamples,
			QualityMinTTFTSamples:    row.QualityMinTTFTSamples,
			QualityCondition:         row.QualityCondition,
			CooldownMinutes:          row.CooldownMinutes,
			AccountIDs:               map[int64]struct{}{},
			Caps:                     map[int64]int{},
		}
		for _, member := range row.Members {
			if member.AccountID <= 0 {
				continue
			}
			policy.AccountIDs[member.AccountID] = struct{}{}
			if member.Cap >= 1 {
				policy.Caps[member.AccountID] = member.Cap
			}
		}
		out.Policies[platform] = policy
	}
	return out
}
