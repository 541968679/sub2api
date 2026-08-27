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
	smartScheduleUserKeyPrefix     = "smart-schedule:user:"
	smartScheduleCooldownKeyPrefix = "smart-schedule:cooldown:"
	smartScheduleCooldownReasonKeyPrefix = "smart-schedule:cooldown-reason:"
	smartScheduleProbeKeyPrefix    = "smart-schedule:probe:"
	smartSchedulePinnedKeyPrefix   = "smart-schedule:pinned:"
	smartScheduleResumeKeyPrefix   = "smart-schedule:resume:"
	smartScheduleCooldownFieldPref = "u:"
	smartScheduleResumeWatchPref   = "w:"
	smartScheduleCooldownTTLBuffer = 2 * time.Hour
	smartScheduleResumeTTL         = 40 * time.Minute
	smartScheduleCooldownHardKeyPrefix = "smart-schedule:cooldown-hard:"
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

func smartScheduleCooldownKey(platform string, accountID int64) string {
	return smartScheduleCooldownKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
}

func smartScheduleCooldownField(userID int64) string {
	return smartScheduleCooldownFieldPref + strconv.FormatInt(userID, 10)
}

func smartScheduleProbeKey(platform string, accountID int64) string {
	return smartScheduleProbeKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
}

func smartSchedulePinnedKey(platform string, accountID int64) string {
	return smartSchedulePinnedKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
}

func smartScheduleResumeKey(platform string, accountID int64) string {
	return smartScheduleResumeKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
}

func smartScheduleResumeWatchingField(userID int64) string {
	return smartScheduleResumeWatchPref + strconv.FormatInt(userID, 10)
}

func smartScheduleCooldownHardKey(platform string, accountID int64) string {
	return smartScheduleCooldownHardKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
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

func (c *userSmartScheduleCache) CooldownActive(ctx context.Context, accountID, userID int64, platform string, now time.Time) bool {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return false
	}
	raw, err := c.rdb.HGet(ctx, smartScheduleCooldownKey(platform, accountID), smartScheduleCooldownField(userID)).Result()
	if err != nil || raw == "" {
		return false
	}
	until, parseErr := strconv.ParseInt(raw, 10, 64)
	if parseErr != nil || until <= now.Unix() {
		if until > 0 && until <= now.Unix() {
			c.expirePairCooldown(ctx, accountID, userID, platform)
		}
		return false
	}
	return true
}

func smartScheduleCooldownReasonKey(platform string, accountID int64) string {
	return smartScheduleCooldownReasonKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
}

func (c *userSmartScheduleCache) StartCooldown(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time) {
	c.StartCooldownWithReason(ctx, accountID, userID, platform, minutes, now, "")
}

func (c *userSmartScheduleCache) StartCooldownWithReason(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time, reason string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	if c.IsPinned(ctx, accountID, userID, platform) {
		return
	}
	minutes = service.ClampSmartScheduleCooldownMinutes(minutes)
	until := now.Add(time.Duration(minutes) * time.Minute).Unix()
	key := smartScheduleCooldownKey(platform, accountID)
	field := smartScheduleCooldownField(userID)
	added, err := c.rdb.HSetNX(ctx, key, field, until).Result()
	if err != nil || !added {
		return
	}
	c.ZeroSoftCooldown(ctx, accountID, userID, platform)
	ttl := time.Duration(minutes)*time.Minute + smartScheduleCooldownTTLBuffer
	c.extendCooldownTTL(ctx, key, ttl)
	event := service.PairQualityEvent{
		Ts:    now.Unix(),
		Type:  service.PairQualityEventCooldownStart,
		Until: &until,
	}
	if strings.TrimSpace(reason) != "" {
		event.Detail = reason
		c.storeCooldownReason(ctx, platform, accountID, userID, reason, ttl)
	}
	c.AppendPairQualityEvent(ctx, accountID, userID, platform, event)
}

func (c *userSmartScheduleCache) storeCooldownReason(ctx context.Context, platform string, accountID, userID int64, reason string, ttl time.Duration) {
	if c == nil || c.rdb == nil || strings.TrimSpace(reason) == "" {
		return
	}
	key := smartScheduleCooldownReasonKey(platform, accountID)
	field := smartScheduleCooldownField(userID)
	payload, err := json.Marshal(map[string]string{"detail": reason})
	if err != nil {
		return
	}
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, key, field, payload)
	pipe.Expire(ctx, key, ttl)
	_, _ = pipe.Exec(ctx)
}

func (c *userSmartScheduleCache) GetCooldownReason(ctx context.Context, accountID, userID int64, platform string) string {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return ""
	}
	raw, err := c.rdb.HGet(ctx, smartScheduleCooldownReasonKey(platform, accountID), smartScheduleCooldownField(userID)).Bytes()
	if err != nil || len(raw) == 0 {
		return ""
	}
	var stored struct {
		Detail string `json:"detail"`
	}
	if json.Unmarshal(raw, &stored) != nil {
		return ""
	}
	return strings.TrimSpace(stored.Detail)
}

// SetCooldown overwrites the pair cooldown (admin switcher). Hot path stays HSETNX.
func (c *userSmartScheduleCache) SetCooldown(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time) (time.Time, error) {
	return c.SetCooldownWithReason(ctx, accountID, userID, platform, minutes, now, "")
}

func (c *userSmartScheduleCache) SetCooldownWithReason(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time, reason string) (time.Time, error) {
	minutes = service.ClampSmartScheduleCooldownMinutes(minutes)
	until := now.Add(time.Duration(minutes) * time.Minute)
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return until, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
		until = now.Add(time.Duration(minutes) * time.Minute)
	}
	key := smartScheduleCooldownKey(platform, accountID)
	untilUnix := until.Unix()
	if err := c.rdb.HSet(ctx, key, smartScheduleCooldownField(userID), untilUnix).Err(); err != nil {
		return until, fmt.Errorf("set smart schedule cooldown: %w", err)
	}
	c.ZeroSoftCooldown(ctx, accountID, userID, platform)
	c.clearCooldownHard(ctx, accountID, userID, platform)
	c.extendCooldownTTL(ctx, key, time.Duration(minutes)*time.Minute+smartScheduleCooldownTTLBuffer)
	event := service.PairQualityEvent{
		Ts:    now.Unix(),
		Type:  service.PairQualityEventCooldownStart,
		Until: &untilUnix,
	}
	if strings.TrimSpace(reason) != "" {
		event.Detail = reason
		c.storeCooldownReason(ctx, platform, accountID, userID, reason, time.Duration(minutes)*time.Minute+smartScheduleCooldownTTLBuffer)
	}
	c.AppendPairQualityEvent(ctx, accountID, userID, platform, event)
	return until, nil
}

// ApplyMemberPaused write-through updates the user bundle so pause does not depend on a cache miss.
func (c *userSmartScheduleCache) ApplyMemberPaused(ctx context.Context, userID, accountID int64, platform string, paused bool) error {
	if c == nil || userID <= 0 || accountID <= 0 {
		return nil
	}
	if c.rdb != nil {
		raw, err := c.rdb.Get(ctx, smartScheduleUserKey(userID)).Bytes()
		if err == nil && len(raw) > 0 {
			var stored cachedSmartScheduleBundle
			if json.Unmarshal(raw, &stored) == nil {
				bundle := stored.toBundle()
				applyPausedToCachedBundle(bundle, accountID, platform, paused)
				c.storeUserBundle(ctx, userID, bundle)
				return nil
			}
		}
	}
	if c.repo == nil {
		return nil
	}
	bundle, err := c.repo.ListByUser(ctx, userID)
	if err != nil {
		return err
	}
	if bundle == nil {
		return nil
	}
	applyPausedToCachedBundle(bundle, accountID, platform, paused)
	c.storeUserBundle(ctx, userID, bundle)
	return nil
}

func applyPausedToCachedBundle(bundle *service.UserSmartScheduleBundle, accountID int64, platform string, paused bool) {
	if bundle == nil || accountID <= 0 {
		return
	}
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform != "" {
		policy := bundle.Policy(platform)
		if policy == nil || !policy.HasAccount(accountID) {
			return
		}
		applyPausedToPolicy(policy, accountID, paused)
		return
	}
}

func applyPausedToPolicy(policy *service.SmartSchedulePlatformPolicy, accountID int64, paused bool) {
	if policy == nil || accountID <= 0 {
		return
	}
	if policy.Paused == nil {
		policy.Paused = map[int64]struct{}{}
	}
	if paused {
		policy.Paused[accountID] = struct{}{}
		return
	}
	delete(policy.Paused, accountID)
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

func (c *userSmartScheduleCache) ClearCooldown(ctx context.Context, accountID, userID int64, platform string) error {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	n, err := c.rdb.HDel(ctx, smartScheduleCooldownKey(platform, accountID), smartScheduleCooldownField(userID)).Result()
	if err != nil {
		return err
	}
	c.ZeroSoftCooldown(ctx, accountID, userID, platform)
	c.clearCooldownHard(ctx, accountID, userID, platform)
	if n > 0 {
		c.AppendPairQualityEvent(ctx, accountID, userID, platform, service.PairQualityEvent{
			Ts:   time.Now().UTC().Unix(),
			Type: service.PairQualityEventCooldownEnd,
		})
	}
	return nil
}

func (c *userSmartScheduleCache) ClearCooldownAllPlatforms(ctx context.Context, accountID, userID int64) error {
	if c == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	var first error
	for _, platform := range service.AllowedQuotaPlatforms {
		if err := c.ClearCooldown(ctx, accountID, userID, platform); err != nil && first == nil {
			first = err
		}
		c.ClearProbing(ctx, accountID, userID, platform)
		c.ClearPinned(ctx, accountID, userID, platform)
		c.ClearPairResume(ctx, accountID, userID, platform)
		c.ZeroPairQuality(ctx, accountID, userID, platform, "")
	}
	return first
}

func (c *userSmartScheduleCache) expirePairCooldown(ctx context.Context, accountID, userID int64, platform string) {
	c.enterProbeFromCooldown(ctx, accountID, userID, platform, "", "")
}

func (c *userSmartScheduleCache) SoftEndCooldown(ctx context.Context, accountID, userID int64, platform string, detail string) {
	if c.IsCooldownHard(ctx, accountID, userID, platform) {
		return
	}
	c.enterProbeFromCooldown(ctx, accountID, userID, platform, service.PairQualityEventSoftCooldownEnd, detail)
}

func (c *userSmartScheduleCache) EnterProbe(ctx context.Context, accountID, userID int64, platform string) service.ProbeAdmissionOutcome {
	c.enterProbeFromCooldown(ctx, accountID, userID, platform, "", "")
	if c.CooldownActive(ctx, accountID, userID, platform, time.Now().UTC()) {
		return service.ProbeAdmissionCooling
	}
	if c.IsProbing(ctx, accountID, userID, platform) {
		return service.ProbeAdmissionProbing
	}
	return service.ProbeAdmissionSelectable
}

func (c *userSmartScheduleCache) enterProbeFromCooldown(ctx context.Context, accountID, userID int64, platform, extraType, detail string) {
	if c == nil || c.rdb == nil {
		return
	}
	_ = c.rdb.HDel(ctx, smartScheduleCooldownKey(platform, accountID), smartScheduleCooldownField(userID)).Err()
	_ = c.rdb.HDel(ctx, smartScheduleCooldownReasonKey(platform, accountID), smartScheduleCooldownField(userID)).Err()
	c.ZeroSoftCooldown(ctx, accountID, userID, platform)
	c.clearCooldownHard(ctx, accountID, userID, platform)
	if c.IsPinned(ctx, accountID, userID, platform) {
		return
	}
	policy := c.lookupPairPolicy(ctx, userID, platform)
	appendExtra := func() {
		if extraType == "" {
			return
		}
		c.AppendPairQualityEvent(ctx, accountID, userID, platform, service.PairQualityEvent{
			Ts:     time.Now().UTC().Unix(),
			Type:   extraType,
			Detail: detail,
		})
	}
	if policy == nil || !policy.ProbeLatencyV2 {
		c.ZeroPairQuality(ctx, accountID, userID, platform, service.PairQualityEventExpiryZero)
		appendExtra()
		c.MarkProbing(ctx, accountID, userID, platform)
		c.ClearPairResume(ctx, accountID, userID, platform)
		return
	}
	minutes := service.ClampSmartScheduleCooldownMinutes(policy.CooldownMinutes)
	since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	samples := listAccountQualityPrecheckSamples(ctx, c.rdb, accountID, userID, since)
	live := service.PairLiveFromPrecheckSamples(samples, policy.TTFTWindowN(), policy.SuccessWindowN())
	ev := service.EvalQuality(live, service.ProbeQualityKnobs(policy))
	c.ZeroPairQuality(ctx, accountID, userID, platform, service.PairQualityEventExpiryZero)
	appendExtra()
	c.ClearPairResume(ctx, accountID, userID, platform)
	switch ev.State {
	case service.LatencyEvalFail:
		c.ClearProbing(ctx, accountID, userID, platform)
		c.StartCooldownWithReason(ctx, accountID, userID, platform, minutes, time.Now().UTC(), service.FormatProbePrecheckCooldownDetail(ev.Reasons))
		c.markCooldownHard(ctx, accountID, userID, platform, minutes)
	case service.LatencyEvalPass:
		c.ClearProbing(ctx, accountID, userID, platform)
	default:
		c.MarkProbing(ctx, accountID, userID, platform)
	}
}

func (c *userSmartScheduleCache) IsCooldownHard(ctx context.Context, accountID, userID int64, platform string) bool {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return false
	}
	raw, err := c.rdb.HGet(ctx, smartScheduleCooldownHardKey(platform, accountID), smartScheduleCooldownField(userID)).Result()
	return err == nil && raw != ""
}

func (c *userSmartScheduleCache) markCooldownHard(ctx context.Context, accountID, userID int64, platform string, minutes int) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	minutes = service.ClampSmartScheduleCooldownMinutes(minutes)
	key := smartScheduleCooldownHardKey(platform, accountID)
	if err := c.rdb.HSet(ctx, key, smartScheduleCooldownField(userID), "1").Err(); err != nil {
		return
	}
	// Same HASH is shared across users; only lengthen TTL (never Expire-down).
	c.extendCooldownTTL(ctx, key, time.Duration(minutes)*time.Minute+smartScheduleCooldownTTLBuffer)
}

func (c *userSmartScheduleCache) clearCooldownHard(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	_ = c.rdb.HDel(ctx, smartScheduleCooldownHardKey(platform, accountID), smartScheduleCooldownField(userID)).Err()
}

func (c *userSmartScheduleCache) lookupPairPolicy(ctx context.Context, userID int64, platform string) *service.SmartSchedulePlatformPolicy {
	if c == nil {
		return nil
	}
	bundle := c.Lookup(ctx, userID)
	if bundle == nil {
		return nil
	}
	return bundle.Policy(platform)
}

func (c *userSmartScheduleCache) IsProbing(ctx context.Context, accountID, userID int64, platform string) bool {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return false
	}
	raw, err := c.rdb.HGet(ctx, smartScheduleProbeKey(platform, accountID), smartScheduleCooldownField(userID)).Result()
	return err == nil && raw != ""
}

func (c *userSmartScheduleCache) MarkProbing(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	now := time.Now().UTC()
	if err := c.rdb.HSet(ctx, smartScheduleProbeKey(platform, accountID), smartScheduleCooldownField(userID), now.Unix()).Err(); err != nil {
		return
	}
	c.AppendPairQualityEvent(ctx, accountID, userID, platform, service.PairQualityEvent{
		Ts:   now.Unix(),
		Type: service.PairQualityEventProbeEnter,
	})
}

func (c *userSmartScheduleCache) ClearProbing(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	_ = c.rdb.HDel(ctx, smartScheduleProbeKey(platform, accountID), smartScheduleCooldownField(userID)).Err()
}

func (c *userSmartScheduleCache) GraduateProbing(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || accountID <= 0 || userID <= 0 {
		return
	}
	if !c.IsProbing(ctx, accountID, userID, platform) {
		return
	}
	c.ClearProbing(ctx, accountID, userID, platform)
	c.AppendPairQualityEvent(ctx, accountID, userID, platform, service.PairQualityEvent{
		Ts:   time.Now().UTC().Unix(),
		Type: service.PairQualityEventProbeGraduate,
	})
}

func (c *userSmartScheduleCache) IsProbingBatch(ctx context.Context, accountIDs []int64, userID int64, platform string) map[int64]bool {
	return c.hashMarkBatch(ctx, accountIDs, userID, platform, smartScheduleProbeKey)
}

func (c *userSmartScheduleCache) IsPinned(ctx context.Context, accountID, userID int64, platform string) bool {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return false
	}
	raw, err := c.rdb.HGet(ctx, smartSchedulePinnedKey(platform, accountID), smartScheduleCooldownField(userID)).Result()
	return err == nil && raw != ""
}

func (c *userSmartScheduleCache) MarkPinned(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	now := time.Now().UTC()
	if err := c.rdb.HSet(ctx, smartSchedulePinnedKey(platform, accountID), smartScheduleCooldownField(userID), now.Unix()).Err(); err != nil {
		return
	}
	c.AppendPairQualityEvent(ctx, accountID, userID, platform, service.PairQualityEvent{
		Ts:   now.Unix(),
		Type: service.PairQualityEventPinEnter,
	})
}

func (c *userSmartScheduleCache) ClearPinned(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	_ = c.rdb.HDel(ctx, smartSchedulePinnedKey(platform, accountID), smartScheduleCooldownField(userID)).Err()
}

func (c *userSmartScheduleCache) IsPinnedBatch(ctx context.Context, accountIDs []int64, userID int64, platform string) map[int64]bool {
	return c.hashMarkBatch(ctx, accountIDs, userID, platform, smartSchedulePinnedKey)
}

func (c *userSmartScheduleCache) PairResumeActive(ctx context.Context, accountID, userID int64, platform string, now time.Time) bool {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return false
	}
	vals, err := c.rdb.HMGet(ctx, smartScheduleResumeKey(platform, accountID),
		smartScheduleCooldownField(userID),
		smartScheduleResumeWatchingField(userID),
	).Result()
	if err != nil {
		return false
	}
	for _, raw := range vals {
		until, ok := parseUnixField(raw)
		if ok && until > now.Unix() {
			return true
		}
	}
	return false
}

func (c *userSmartScheduleCache) MarkPairResume(ctx context.Context, accountID, userID int64, platform string) error {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	now := time.Now().UTC()
	key := smartScheduleResumeKey(platform, accountID)
	pipe := c.rdb.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		smartScheduleCooldownField(userID):       now.Add(service.AccountQualityWindow).Unix(),
		smartScheduleResumeWatchingField(userID): now.Add(2 * service.AccountQualityWindow).Unix(),
	})
	pipe.Expire(ctx, key, smartScheduleResumeTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *userSmartScheduleCache) ClearPairResume(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	_ = c.rdb.HDel(ctx, smartScheduleResumeKey(platform, accountID),
		smartScheduleCooldownField(userID),
		smartScheduleResumeWatchingField(userID),
	).Err()
}

func (c *userSmartScheduleCache) GetPairResumeUntilBatch(ctx context.Context, accountIDs []int64, userID int64, platform string, now time.Time) map[int64]service.PairResumeUntil {
	out := map[int64]service.PairResumeUntil{}
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
	cmds := make([]*redis.SliceCmd, len(ids))
	for i, accountID := range ids {
		cmds[i] = pipe.HMGet(ctx, smartScheduleResumeKey(platform, accountID),
			smartScheduleCooldownField(userID),
			smartScheduleResumeWatchingField(userID),
		)
	}
	_, _ = pipe.Exec(ctx)
	for i, cmd := range cmds {
		vals, err := cmd.Result()
		if err != nil || len(vals) < 2 {
			continue
		}
		chip, chipOK := parseUnixField(vals[0])
		watch, watchOK := parseUnixField(vals[1])
		live := service.PairResumeUntil{}
		if chipOK && chip > now.Unix() {
			live.ChipUntil = time.Unix(chip, 0).UTC()
		}
		if watchOK && watch > now.Unix() {
			live.WatchUntil = time.Unix(watch, 0).UTC()
		}
		if live.Active(now) {
			out[ids[i]] = live
		}
	}
	return out
}

func parseUnixField(raw any) (int64, bool) {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return 0, false
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return 0, false
		}
		return n, true
	case []byte:
		return parseUnixField(string(v))
	default:
		return 0, false
	}
}

func (c *userSmartScheduleCache) hashMarkBatch(ctx context.Context, accountIDs []int64, userID int64, platform string, keyFn func(string, int64) string) map[int64]bool {
	out := map[int64]bool{}
	if c == nil || c.rdb == nil || userID <= 0 || len(accountIDs) == 0 || keyFn == nil {
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
		cmds[i] = pipe.HGet(ctx, keyFn(platform, accountID), smartScheduleCooldownField(userID))
	}
	_, _ = pipe.Exec(ctx)
	for i, cmd := range cmds {
		raw, err := cmd.Result()
		if err != nil || raw == "" {
			continue
		}
		out[ids[i]] = true
	}
	return out
}

func (c *userSmartScheduleCache) GetCooldownUntilBatch(ctx context.Context, accountIDs []int64, userID int64, platform string, now time.Time) map[int64]time.Time {
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
		cmds[i] = pipe.HGet(ctx, smartScheduleCooldownKey(platform, accountID), smartScheduleCooldownField(userID))
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
			if until > 0 && until <= nowUnix {
				c.expirePairCooldown(ctx, ids[i], userID, platform)
			}
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
	Paused    bool  `json:"paused,omitempty"`
}

type cachedSmartSchedulePolicy struct {
	Enabled                        bool                        `json:"enabled"`
	QualityMaxP50TTFTMs            *int                        `json:"quality_max_p50_ttft_ms,omitempty"`
	QualityMinSuccessRate          *float64                    `json:"quality_min_success_rate,omitempty"`
	QualityWindowSamples           *int                        `json:"quality_window_samples,omitempty"`
	QualityMinSuccessSamples       *int                        `json:"quality_min_success_samples,omitempty"`
	QualityMinTTFTSamples          *int                        `json:"quality_min_ttft_samples,omitempty"`
	QualityCondition               *string                     `json:"quality_condition,omitempty"`
	CooldownMinutes                int                         `json:"cooldown_minutes"`
	SoftCooldown                   bool                        `json:"soft_cooldown,omitempty"`
	ProbeConcurrencyMode           string                      `json:"probe_concurrency_mode,omitempty"`
	ProbeConcurrency               *int                        `json:"probe_concurrency,omitempty"`
	QualityMaxSlowInWindow         *int                        `json:"quality_max_slow_in_window,omitempty"`
	QualityMaxConsecutiveSlow      *int                        `json:"quality_max_consecutive_slow,omitempty"`
	QualityMaxP50DurationMs        *int                        `json:"quality_max_p50_duration_ms,omitempty"`
	QualitySchedWindowN            *int                        `json:"quality_sched_window_n,omitempty"`
	QualitySchedMaxSlowInWindow    *int                        `json:"quality_sched_max_slow_in_window,omitempty"`
	QualitySchedMaxConsecutiveSlow *int                        `json:"quality_sched_max_consecutive_slow,omitempty"`
	ProbeLatencyV2                 bool                        `json:"probe_latency_v2,omitempty"`
	Members                        []cachedSmartScheduleMember `json:"members,omitempty"`
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
			Enabled:                        policy.Enabled,
			QualityMaxP50TTFTMs:            policy.QualityMaxP50TTFTMs,
			QualityMinSuccessRate:          policy.QualityMinSuccessRate,
			QualityWindowSamples:           policy.QualityWindowSamples,
			QualityMinSuccessSamples:       policy.QualityMinSuccessSamples,
			QualityMinTTFTSamples:          policy.QualityMinTTFTSamples,
			QualityCondition:               policy.QualityCondition,
			CooldownMinutes:                policy.CooldownMinutes,
			SoftCooldown:                   policy.SoftCooldown,
			ProbeConcurrencyMode:           policy.ProbeConcurrencyMode,
			ProbeConcurrency:               policy.ProbeConcurrency,
			QualityMaxSlowInWindow:         policy.QualityMaxSlowInWindow,
			QualityMaxConsecutiveSlow:      policy.QualityMaxConsecutiveSlow,
			QualityMaxP50DurationMs:        policy.QualityMaxP50DurationMs,
			QualitySchedWindowN:            policy.QualitySchedWindowN,
			QualitySchedMaxSlowInWindow:    policy.QualitySchedMaxSlowInWindow,
			QualitySchedMaxConsecutiveSlow: policy.QualitySchedMaxConsecutiveSlow,
			ProbeLatencyV2:                 policy.ProbeLatencyV2,
		}
		for accountID := range policy.AccountIDs {
			row.Members = append(row.Members, cachedSmartScheduleMember{
				AccountID: accountID,
				Cap:       policy.PairCap(accountID),
				Paused:    policy.IsPaused(accountID),
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
			Enabled:                        row.Enabled,
			QualityMaxP50TTFTMs:            row.QualityMaxP50TTFTMs,
			QualityMinSuccessRate:          row.QualityMinSuccessRate,
			QualityWindowSamples:           row.QualityWindowSamples,
			QualityMinSuccessSamples:       row.QualityMinSuccessSamples,
			QualityMinTTFTSamples:          row.QualityMinTTFTSamples,
			QualityCondition:               row.QualityCondition,
			CooldownMinutes:                row.CooldownMinutes,
			SoftCooldown:                   row.SoftCooldown,
			ProbeConcurrencyMode:           row.ProbeConcurrencyMode,
			ProbeConcurrency:               row.ProbeConcurrency,
			QualityMaxSlowInWindow:         row.QualityMaxSlowInWindow,
			QualityMaxConsecutiveSlow:      row.QualityMaxConsecutiveSlow,
			QualityMaxP50DurationMs:        row.QualityMaxP50DurationMs,
			QualitySchedWindowN:            row.QualitySchedWindowN,
			QualitySchedMaxSlowInWindow:    row.QualitySchedMaxSlowInWindow,
			QualitySchedMaxConsecutiveSlow: row.QualitySchedMaxConsecutiveSlow,
			ProbeLatencyV2:                 row.ProbeLatencyV2,
			AccountIDs:                     map[int64]struct{}{},
			Caps:                           map[int64]int{},
			Paused:                         map[int64]struct{}{},
		}
		for _, member := range row.Members {
			if member.AccountID <= 0 {
				continue
			}
			policy.AccountIDs[member.AccountID] = struct{}{}
			if member.Cap >= 1 {
				policy.Caps[member.AccountID] = member.Cap
			}
			if member.Paused {
				policy.Paused[member.AccountID] = struct{}{}
			}
		}
		out.Policies[platform] = policy
	}
	return out
}
