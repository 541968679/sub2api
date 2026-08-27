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

type softCooldownWindow struct {
	Samples []service.SoftCooldownSample `json:"samples"`
	NTTFT   int                          `json:"n_ttft,omitempty"`
	NOK     int                          `json:"n_ok,omitempty"`
}

func smartScheduleSoftCoolKey(platform string, accountID int64) string {
	return smartScheduleSoftCoolKeyPrefix + service.SmartScheduleRedisPlatform(platform) + ":" + strconv.FormatInt(accountID, 10)
}

func (c *userSmartScheduleCache) GetSoftCooldown(ctx context.Context, accountID, userID int64, platform string) *service.PairQualityLive {
	win := c.loadSoftCooldownWindow(ctx, accountID, userID, platform)
	if win == nil {
		return nil
	}
	return c.softWindowToLive(ctx, userID, platform, *win, 0)
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
		win := decodeSoftCooldownWindow(raw)
		if win == nil {
			continue
		}
		if live := c.softWindowToLive(ctx, userID, platform, *win, 0); live != nil {
			out[ids[i]] = live
		}
	}
	return out
}

func (c *userSmartScheduleCache) IngestSoftCooldown(ctx context.Context, accountID, userID int64, platform string, nTTFT, nOK int, success bool, firstTokenMs, durationMs *int, minutes int) *service.PairQualityLive {
	if c == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	nTTFT = service.ClampSmartScheduleWindowN(nTTFT)
	nOK = service.ClampSmartScheduleWindowN(nOK)
	win := c.loadSoftCooldownWindow(ctx, accountID, userID, platform)
	if win == nil {
		win = &softCooldownWindow{}
	}
	win.NTTFT = nTTFT
	win.NOK = nOK
	win.Samples = append(win.Samples, service.SoftCooldownSample{
		UnixTS:     time.Now().UTC().Unix(),
		OK:         success,
		TTFTMs:     firstTokenMs,
		DurationMs: durationMs,
	})
	minutes = c.softFilterMinutes(ctx, userID, platform, minutes)
	since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	win.Samples = trimSoftCooldownSamples(service.FilterSoftCooldownSamples(win.Samples, since), nTTFT, nOK)
	c.storeSoftCooldownWindow(ctx, accountID, userID, platform, win, minutes)
	return service.SoftLiveFromSamples(win.Samples, nTTFT, nOK)
}

func (c *userSmartScheduleCache) ZeroSoftCooldown(ctx context.Context, accountID, userID int64, platform string) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return
	}
	_ = c.rdb.HDel(ctx, smartScheduleSoftCoolKey(platform, accountID), smartScheduleCooldownField(userID)).Err()
}

func (c *userSmartScheduleCache) loadSoftCooldownWindow(ctx context.Context, accountID, userID int64, platform string) *softCooldownWindow {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 {
		return nil
	}
	raw, err := c.rdb.HGet(ctx, smartScheduleSoftCoolKey(platform, accountID), smartScheduleCooldownField(userID)).Bytes()
	if err != nil || len(raw) == 0 {
		return nil
	}
	return decodeSoftCooldownWindow(raw)
}

func (c *userSmartScheduleCache) storeSoftCooldownWindow(ctx context.Context, accountID, userID int64, platform string, win *softCooldownWindow, minutes int) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 || win == nil {
		return
	}
	payload, err := json.Marshal(win)
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

func (c *userSmartScheduleCache) softWindowToLive(ctx context.Context, userID int64, platform string, win softCooldownWindow, minutes int) *service.PairQualityLive {
	minutes = c.softFilterMinutes(ctx, userID, platform, minutes)
	since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)
	samples := service.FilterSoftCooldownSamples(win.Samples, since)
	if len(samples) == 0 {
		return nil
	}
	nTTFT, nOK := win.NTTFT, win.NOK
	if nTTFT < 1 {
		nTTFT = service.DefaultSmartScheduleWindowN
	}
	if nOK < 1 {
		nOK = service.DefaultSmartScheduleWindowN
	}
	return service.SoftLiveFromSamples(samples, nTTFT, nOK)
}

func (c *userSmartScheduleCache) softFilterMinutes(ctx context.Context, userID int64, platform string, minutes int) int {
	if minutes >= 1 {
		return service.ClampSmartScheduleCooldownMinutes(minutes)
	}
	if policy := c.lookupPairPolicy(ctx, userID, platform); policy != nil && policy.CooldownMinutes >= 1 {
		return service.ClampSmartScheduleCooldownMinutes(policy.CooldownMinutes)
	}
	return service.DefaultSmartScheduleCooldownMinutes
}

func decodeSoftCooldownWindow(raw []byte) *softCooldownWindow {
	if len(raw) == 0 {
		return nil
	}
	var probe struct {
		Samples *[]service.SoftCooldownSample `json:"samples"`
		TTFT    []int                         `json:"ttft"`
		OK      []uint8                       `json:"ok"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	if probe.Samples == nil {
		// Legacy PairQualityLive blob has no per-sample ts — treat as empty (no false-meet).
		return nil
	}
	var win softCooldownWindow
	if err := json.Unmarshal(raw, &win); err != nil {
		return nil
	}
	return &win
}

func trimSoftCooldownSamples(samples []service.SoftCooldownSample, nTTFT, nOK int) []service.SoftCooldownSample {
	capN := service.ClampSmartScheduleWindowN(nTTFT)
	if okN := service.ClampSmartScheduleWindowN(nOK); okN > capN {
		capN = okN
	}
	if capN < 1 {
		capN = service.DefaultSmartScheduleWindowN
	}
	storage := capN * 2
	if len(samples) <= storage {
		return samples
	}
	return append([]service.SoftCooldownSample(nil), samples[len(samples)-storage:]...)
}
