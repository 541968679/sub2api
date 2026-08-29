package service

import (
	"context"
	"sync"
	"time"
)

// PublicScheduleQualityCache is the account-level public FIFO + six-state store.
// Keys have no user field. Redis miss / nil cache is fail-open selectable.
type PublicScheduleQualityCache interface {
	GetWindow(ctx context.Context, accountID int64) *PairQualityLive
	StoreWindow(ctx context.Context, accountID int64, live *PairQualityLive)
	GetSoft(ctx context.Context, accountID int64) *PairQualityLive
	StoreSoft(ctx context.Context, accountID int64, live *PairQualityLive)
	ClearSoft(ctx context.Context, accountID int64)
	GetState(ctx context.Context, accountID int64) *PublicScheduleRuntimeState
	GetStateBatch(ctx context.Context, accountIDs []int64) map[int64]*PublicScheduleRuntimeState
	TryStartCooldown(ctx context.Context, accountID int64, until time.Time, reason string, soft bool) bool
	SetState(ctx context.Context, accountID int64, state *PublicScheduleRuntimeState)
	ClearState(ctx context.Context, accountID int64)
}

// MemoryPublicScheduleQualityCache is the unit-test / fail-open in-process store.
type MemoryPublicScheduleQualityCache struct {
	mu      sync.Mutex
	windows map[int64]*PairQualityLive
	soft    map[int64]*PairQualityLive
	states  map[int64]*PublicScheduleRuntimeState
}

func NewMemoryPublicScheduleQualityCache() *MemoryPublicScheduleQualityCache {
	return &MemoryPublicScheduleQualityCache{
		windows: map[int64]*PairQualityLive{},
		soft:    map[int64]*PairQualityLive{},
		states:  map[int64]*PublicScheduleRuntimeState{},
	}
}

func (c *MemoryPublicScheduleQualityCache) GetWindow(_ context.Context, accountID int64) *PairQualityLive {
	if c == nil || accountID <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return clonePairQualityLive(c.windows[accountID])
}

func (c *MemoryPublicScheduleQualityCache) StoreWindow(_ context.Context, accountID int64, live *PairQualityLive) {
	if c == nil || accountID <= 0 || live == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.windows[accountID] = clonePairQualityLive(live)
}

func (c *MemoryPublicScheduleQualityCache) GetSoft(_ context.Context, accountID int64) *PairQualityLive {
	if c == nil || accountID <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return clonePairQualityLive(c.soft[accountID])
}

func (c *MemoryPublicScheduleQualityCache) StoreSoft(_ context.Context, accountID int64, live *PairQualityLive) {
	if c == nil || accountID <= 0 || live == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.soft[accountID] = clonePairQualityLive(live)
}

func (c *MemoryPublicScheduleQualityCache) ClearSoft(_ context.Context, accountID int64) {
	if c == nil || accountID <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.soft, accountID)
}

func (c *MemoryPublicScheduleQualityCache) GetState(_ context.Context, accountID int64) *PublicScheduleRuntimeState {
	if c == nil || accountID <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return clonePublicScheduleState(c.states[accountID])
}

func (c *MemoryPublicScheduleQualityCache) GetStateBatch(_ context.Context, accountIDs []int64) map[int64]*PublicScheduleRuntimeState {
	out := map[int64]*PublicScheduleRuntimeState{}
	if c == nil {
		return out
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, id := range accountIDs {
		if id <= 0 {
			continue
		}
		if st := clonePublicScheduleState(c.states[id]); st != nil {
			out[id] = st
		}
	}
	return out
}

func (c *MemoryPublicScheduleQualityCache) TryStartCooldown(_ context.Context, accountID int64, until time.Time, reason string, soft bool) bool {
	if c == nil || accountID <= 0 {
		return false
	}
	now := time.Now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	if current := c.states[accountID]; current != nil {
		switch current.Normalized(now) {
		case PublicScheduleStateCooling, PublicScheduleStatePaused, PublicScheduleStatePinned:
			return false
		}
	}
	c.states[accountID] = &PublicScheduleRuntimeState{
		State:     PublicScheduleStateCooling,
		Until:     until.UTC(),
		Reason:    reason,
		Soft:      soft,
		UpdatedAt: now,
	}
	return true
}

func (c *MemoryPublicScheduleQualityCache) SetState(_ context.Context, accountID int64, state *PublicScheduleRuntimeState) {
	if c == nil || accountID <= 0 || state == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.states[accountID] = clonePublicScheduleState(state)
}

func (c *MemoryPublicScheduleQualityCache) ClearState(_ context.Context, accountID int64) {
	if c == nil || accountID <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.states, accountID)
}

func clonePairQualityLive(live *PairQualityLive) *PairQualityLive {
	if live == nil {
		return nil
	}
	out := *live
	out.TTFTMs = append([]int(nil), live.TTFTMs...)
	out.DurationMs = append([]int(nil), live.DurationMs...)
	out.OK = append([]bool(nil), live.OK...)
	if live.P50TTFTMs != nil {
		v := *live.P50TTFTMs
		out.P50TTFTMs = &v
	}
	if live.P50DurationMs != nil {
		v := *live.P50DurationMs
		out.P50DurationMs = &v
	}
	if live.SuccessRate != nil {
		v := *live.SuccessRate
		out.SuccessRate = &v
	}
	return &out
}

func clonePublicScheduleState(state *PublicScheduleRuntimeState) *PublicScheduleRuntimeState {
	if state == nil {
		return nil
	}
	out := *state
	return &out
}
