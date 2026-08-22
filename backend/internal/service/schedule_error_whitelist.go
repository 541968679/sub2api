package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	ScheduleErrorFamilyClientInvalidRequest = "client_invalid_request"
	ScheduleErrorFamilyClientWrapped400URF  = "client_wrapped_400_urf"
	ScheduleErrorFamilyClientContextTooLong = "client_context_too_long"
	ScheduleErrorFamilyPairConcurrency      = "pair_concurrency"
	ScheduleErrorFamilyGroupNoAccount       = "group_no_account"
	ScheduleErrorFamilyRoutingModelMiss     = "routing_model_miss"
	ScheduleErrorFamilyRoutingPoolEmpty     = "routing_pool_empty"
	ScheduleErrorFamilyProtocolMismatch     = "protocol_mismatch"

	scheduleErrorWhitelistCacheTTL = 3 * time.Second
)

// ScheduleErrorFamilyIDs is the stable preset set. SQL needles are code
// constants only — admins cannot add custom LIKE families.
var ScheduleErrorFamilyIDs = []string{
	ScheduleErrorFamilyClientInvalidRequest,
	ScheduleErrorFamilyClientWrapped400URF,
	ScheduleErrorFamilyClientContextTooLong,
	ScheduleErrorFamilyPairConcurrency,
	ScheduleErrorFamilyGroupNoAccount,
	ScheduleErrorFamilyRoutingModelMiss,
	ScheduleErrorFamilyRoutingPoolEmpty,
	ScheduleErrorFamilyProtocolMismatch,
}

var defaultScheduleErrorFamilyEnabled = map[string]bool{
	ScheduleErrorFamilyClientInvalidRequest: true,
	ScheduleErrorFamilyClientWrapped400URF:  true,
	ScheduleErrorFamilyClientContextTooLong: true,
	ScheduleErrorFamilyPairConcurrency:      true,
	ScheduleErrorFamilyGroupNoAccount:       true,
	ScheduleErrorFamilyRoutingModelMiss:     true,
	ScheduleErrorFamilyRoutingPoolEmpty:     true,
	ScheduleErrorFamilyProtocolMismatch:     true,
}

// ScheduleErrorWhitelist is the Settings KV shape for schedule-error families.
// true = in whitelist = do not count toward schedule ErrorCount / cooldown.
type ScheduleErrorWhitelist struct {
	Families map[string]bool `json:"families"`
}

func knownScheduleErrorFamily(id string) bool {
	_, ok := defaultScheduleErrorFamilyEnabled[id]
	return ok
}

// DefaultScheduleErrorWhitelist is the factory preset (all families on).
func DefaultScheduleErrorWhitelist() ScheduleErrorWhitelist {
	families := make(map[string]bool, len(ScheduleErrorFamilyIDs))
	for _, id := range ScheduleErrorFamilyIDs {
		families[id] = defaultScheduleErrorFamilyEnabled[id]
	}
	return ScheduleErrorWhitelist{Families: families}
}

// NormalizeScheduleErrorWhitelist copies known keys over factory defaults.
// Missing keys keep the factory default. Unknown keys are dropped on read.
func NormalizeScheduleErrorWhitelist(in ScheduleErrorWhitelist) ScheduleErrorWhitelist {
	out := DefaultScheduleErrorWhitelist()
	if in.Families == nil {
		return out
	}
	for _, id := range ScheduleErrorFamilyIDs {
		if v, ok := in.Families[id]; ok {
			out.Families[id] = v
		}
	}
	return out
}

// FamilyEnabled reports whether a preset family is on the whitelist.
// Missing key / empty config = factory default. Unknown id = false.
func (w ScheduleErrorWhitelist) FamilyEnabled(id string) bool {
	if w.Families != nil {
		if v, ok := w.Families[id]; ok {
			return v
		}
	}
	if v, ok := defaultScheduleErrorFamilyEnabled[id]; ok {
		return v
	}
	return false
}

// ParseScheduleErrorWhitelistJSON treats empty / invalid JSON as factory default.
func ParseScheduleErrorWhitelistJSON(raw string) ScheduleErrorWhitelist {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultScheduleErrorWhitelist()
	}
	var parsed ScheduleErrorWhitelist
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return DefaultScheduleErrorWhitelist()
	}
	return NormalizeScheduleErrorWhitelist(parsed)
}

// ValidateScheduleErrorWhitelist accepts only known family ids + bool.
func ValidateScheduleErrorWhitelist(settings *ScheduleErrorWhitelist) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	for id := range settings.Families {
		if !knownScheduleErrorFamily(id) {
			return fmt.Errorf("unknown schedule error whitelist family %q", id)
		}
	}
	return nil
}

func cloneScheduleErrorWhitelist(in ScheduleErrorWhitelist) ScheduleErrorWhitelist {
	out := ScheduleErrorWhitelist{Families: make(map[string]bool, len(in.Families))}
	for k, v := range in.Families {
		out.Families[k] = v
	}
	return out
}

type scheduleErrorWhitelistCache struct {
	mu      sync.Mutex
	value   ScheduleErrorWhitelist
	loaded  time.Time
	ok      bool
	source  func(context.Context) ScheduleErrorWhitelist
	sourceM sync.RWMutex
}

var scheduleErrorWLCache = &scheduleErrorWhitelistCache{}

// RegisterScheduleErrorWhitelistSource wires SettingService into SQL generation.
func RegisterScheduleErrorWhitelistSource(fn func(context.Context) ScheduleErrorWhitelist) {
	scheduleErrorWLCache.sourceM.Lock()
	scheduleErrorWLCache.source = fn
	scheduleErrorWLCache.sourceM.Unlock()
	InvalidateScheduleErrorWhitelistCache()
}

// InvalidateScheduleErrorWhitelistCache drops the short TTL after a save.
func InvalidateScheduleErrorWhitelistCache() {
	scheduleErrorWLCache.mu.Lock()
	scheduleErrorWLCache.ok = false
	scheduleErrorWLCache.loaded = time.Time{}
	scheduleErrorWLCache.mu.Unlock()
}

// ResolveScheduleErrorWhitelist returns the current KV (or factory default).
// Classify callers that already loaded settings should pass that copy instead
// so Go and SQL stay on the same snapshot.
func ResolveScheduleErrorWhitelist(ctx context.Context) ScheduleErrorWhitelist {
	scheduleErrorWLCache.mu.Lock()
	if scheduleErrorWLCache.ok && time.Since(scheduleErrorWLCache.loaded) < scheduleErrorWhitelistCacheTTL {
		cached := cloneScheduleErrorWhitelist(scheduleErrorWLCache.value)
		scheduleErrorWLCache.mu.Unlock()
		return cached
	}
	scheduleErrorWLCache.mu.Unlock()

	scheduleErrorWLCache.sourceM.RLock()
	fn := scheduleErrorWLCache.source
	scheduleErrorWLCache.sourceM.RUnlock()

	wl := DefaultScheduleErrorWhitelist()
	if fn != nil {
		wl = NormalizeScheduleErrorWhitelist(fn(ctx))
	}

	scheduleErrorWLCache.mu.Lock()
	scheduleErrorWLCache.value = cloneScheduleErrorWhitelist(wl)
	scheduleErrorWLCache.loaded = time.Now()
	scheduleErrorWLCache.ok = true
	scheduleErrorWLCache.mu.Unlock()
	return wl
}

func registerScheduleErrorWhitelistSource(s *SettingService) {
	RegisterScheduleErrorWhitelistSource(func(ctx context.Context) ScheduleErrorWhitelist {
		if s == nil {
			return DefaultScheduleErrorWhitelist()
		}
		wl, err := s.GetScheduleErrorWhitelist(ctx)
		if err != nil || wl == nil {
			return DefaultScheduleErrorWhitelist()
		}
		return *wl
	})
}

// GetScheduleErrorWhitelist returns the admin-editable whitelist. Missing /
// invalid JSON yields factory defaults.
func (s *SettingService) GetScheduleErrorWhitelist(ctx context.Context) (*ScheduleErrorWhitelist, error) {
	if s == nil || s.settingRepo == nil {
		wl := DefaultScheduleErrorWhitelist()
		return &wl, nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyScheduleErrorWhitelist)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			wl := DefaultScheduleErrorWhitelist()
			return &wl, nil
		}
		return nil, fmt.Errorf("get schedule error whitelist: %w", err)
	}
	wl := ParseScheduleErrorWhitelistJSON(value)
	return &wl, nil
}

// SetScheduleErrorWhitelist persists known family ids only, then invalidates
// the short cache so Classify / SQL pick up the new snapshot.
func (s *SettingService) SetScheduleErrorWhitelist(ctx context.Context, settings *ScheduleErrorWhitelist) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("settings service not ready")
	}
	if err := ValidateScheduleErrorWhitelist(settings); err != nil {
		return err
	}
	normalized := NormalizeScheduleErrorWhitelist(*settings)
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal schedule error whitelist: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyScheduleErrorWhitelist, string(data)); err != nil {
		return err
	}
	InvalidateScheduleErrorWhitelistCache()
	return nil
}

func loadScheduleErrorWhitelistFromRepo(ctx context.Context, repo SettingRepository) ScheduleErrorWhitelist {
	if repo == nil {
		return DefaultScheduleErrorWhitelist()
	}
	value, err := repo.GetValue(ctx, SettingKeyScheduleErrorWhitelist)
	if err != nil || value == "" {
		return DefaultScheduleErrorWhitelist()
	}
	return ParseScheduleErrorWhitelistJSON(value)
}
