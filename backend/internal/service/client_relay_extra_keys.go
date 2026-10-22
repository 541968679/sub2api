package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const clientRelayExtraResponseKeyLimit = 64

// defaultClientRelayExtraResponseKeys is the list used when the admin setting
// is missing or unreadable. An explicit saved empty list does not use this.
var defaultClientRelayExtraResponseKeys = []string{
	"credit",
	"elapsed_ms",
	"first_chunk_at",
	"credit_usage",
	"credit_unit",
	"credit_unit_plural",
	"billing_usage",
	"provider",
	"cost_cny",
	"billing_pending",
	"trace_id",
	"reasoning_available",
}

// clientRelayProtectedResponseKeys are official response names. Stripping
// them would delete the completion, the usage object, or a usage counter.
var clientRelayProtectedResponseKeys = map[string]struct{}{
	"id":                        {},
	"object":                    {},
	"created":                   {},
	"model":                     {},
	"choices":                   {},
	"usage":                     {},
	"system_fingerprint":        {},
	"service_tier":              {},
	"error":                     {},
	"type":                      {},
	"message":                   {},
	"delta":                     {},
	"content":                   {},
	"role":                      {},
	"name":                      {},
	"arguments":                 {},
	"tool_calls":                {},
	"tool_call_id":              {},
	"function_call":             {},
	"finish_reason":             {},
	"index":                     {},
	"logprobs":                  {},
	"refusal":                   {},
	"annotations":               {},
	"reasoning":                 {},
	"reasoning_content":         {},
	"audio":                     {},
	"prompt_tokens":             {},
	"completion_tokens":         {},
	"total_tokens":              {},
	"cached_tokens":             {},
	"prompt_tokens_details":     {},
	"completion_tokens_details": {},
	"prompt_cache_hit_tokens":   {},
	"cache_write_tokens":        {},
	"input_tokens":              {},
	"output_tokens":             {},
}

var clientRelayExtraKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,63}$`)

type cachedClientRelayExtraResponseKeys struct {
	keys      []string
	expiresAt int64
}

var (
	clientRelayExtraKeysCache  atomic.Value // *cachedClientRelayExtraResponseKeys
	clientRelayExtraKeysSF     singleflight.Group
	clientRelayExtraKeysSource atomic.Pointer[SettingService]
)

const (
	clientRelayExtraKeysCacheTTL  = 30 * time.Second
	clientRelayExtraKeysErrorTTL  = 5 * time.Second
	clientRelayExtraKeysDBTimeout = 5 * time.Second
	clientRelayExtraKeysCacheKey  = "client_relay_extra_response_keys"
)

func registerClientRelayExtraKeysSource(s *SettingService) {
	clientRelayExtraKeysSource.Store(s)
}

// DefaultClientRelayExtraResponseKeys returns a copy of the built-in list.
func DefaultClientRelayExtraResponseKeys() []string {
	return cloneClientRelayExtraKeys(defaultClientRelayExtraResponseKeys)
}

func cloneClientRelayExtraKeys(keys []string) []string {
	if keys == nil {
		return []string{}
	}
	return append([]string(nil), keys...)
}

func storeClientRelayExtraKeysCache(keys []string, ttl time.Duration) {
	clientRelayExtraKeysCache.Store(&cachedClientRelayExtraResponseKeys{
		keys:      cloneClientRelayExtraKeys(keys),
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

func invalidateClientRelayExtraKeysCache() {
	clientRelayExtraKeysSF.Forget(clientRelayExtraKeysCacheKey)
	storeClientRelayExtraKeysCache(nil, 0)
}

// InvalidateClientRelayExtraKeysCacheForTest clears the hot-path cache.
func InvalidateClientRelayExtraKeysCacheForTest() {
	invalidateClientRelayExtraKeysCache()
}

func loadCachedClientRelayExtraResponseKeys() ([]string, bool) {
	cached, ok := clientRelayExtraKeysCache.Load().(*cachedClientRelayExtraResponseKeys)
	if !ok || cached == nil {
		return nil, false
	}
	if time.Now().UnixNano() >= cached.expiresAt {
		return nil, false
	}
	return cached.keys, true
}

// NormalizeClientRelayExtraResponseKeys trims, drops blanks and duplicates,
// and rejects official response fields. An empty input is a valid empty list.
func NormalizeClientRelayExtraResponseKeys(keys []string) ([]string, error) {
	if keys == nil {
		keys = []string{}
	}
	out := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, raw := range keys {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if !clientRelayExtraKeyPattern.MatchString(key) {
			return nil, fmt.Errorf("invalid client response field %q", key)
		}
		if _, protected := clientRelayProtectedResponseKeys[key]; protected {
			return nil, fmt.Errorf("cannot strip official response field %q", key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		if len(out) >= clientRelayExtraResponseKeyLimit {
			return nil, fmt.Errorf("client response fields exceed %d", clientRelayExtraResponseKeyLimit)
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out, nil
}

func parseClientRelayExtraResponseKeysJSON(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cloneClientRelayExtraKeys(defaultClientRelayExtraResponseKeys), nil
	}
	var object struct {
		Keys []string `json:"keys"`
	}
	if err := json.Unmarshal([]byte(raw), &object); err != nil {
		return nil, err
	}
	if object.Keys == nil {
		return cloneClientRelayExtraKeys(defaultClientRelayExtraResponseKeys), nil
	}
	return NormalizeClientRelayExtraResponseKeys(object.Keys)
}

func (s *SettingService) loadClientRelayExtraResponseKeysFromRepo(ctx context.Context) []string {
	fallback := cloneClientRelayExtraKeys(defaultClientRelayExtraResponseKeys)
	if s == nil || s.settingRepo == nil {
		storeClientRelayExtraKeysCache(fallback, clientRelayExtraKeysCacheTTL)
		return fallback
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), clientRelayExtraKeysDBTimeout)
	defer cancel()
	value, err := s.settingRepo.GetValue(dbCtx, SettingKeyClientRelayExtraResponseKeys)
	if err != nil {
		if !errors.Is(err, ErrSettingNotFound) {
			slog.Warn("failed to get client_relay_extra_response_keys", "error", err)
			storeClientRelayExtraKeysCache(fallback, clientRelayExtraKeysErrorTTL)
			return fallback
		}
		storeClientRelayExtraKeysCache(fallback, clientRelayExtraKeysCacheTTL)
		return fallback
	}
	keys, err := parseClientRelayExtraResponseKeysJSON(value)
	if err != nil {
		slog.Warn("failed to parse client_relay_extra_response_keys", "error", err)
		storeClientRelayExtraKeysCache(fallback, clientRelayExtraKeysErrorTTL)
		return fallback
	}
	storeClientRelayExtraKeysCache(keys, clientRelayExtraKeysCacheTTL)
	return keys
}

func (s *SettingService) cachedClientRelayExtraResponseKeys(ctx context.Context) []string {
	if keys, ok := loadCachedClientRelayExtraResponseKeys(); ok {
		return keys
	}
	if s == nil {
		return cloneClientRelayExtraKeys(defaultClientRelayExtraResponseKeys)
	}
	result, _, _ := clientRelayExtraKeysSF.Do(clientRelayExtraKeysCacheKey, func() (any, error) {
		if keys, ok := loadCachedClientRelayExtraResponseKeys(); ok {
			return keys, nil
		}
		return s.loadClientRelayExtraResponseKeysFromRepo(ctx), nil
	})
	keys, _ := result.([]string)
	if keys == nil {
		return cloneClientRelayExtraKeys(defaultClientRelayExtraResponseKeys)
	}
	return keys
}

// currentClientRelayExtraResponseKeys is the list the response writer uses.
// A cold cache with no settings service keeps the built-in names.
func currentClientRelayExtraResponseKeys() []string {
	if keys, ok := loadCachedClientRelayExtraResponseKeys(); ok {
		return keys
	}
	src := clientRelayExtraKeysSource.Load()
	if src == nil {
		return cloneClientRelayExtraKeys(defaultClientRelayExtraResponseKeys)
	}
	return src.cachedClientRelayExtraResponseKeys(context.Background())
}

// GetClientRelayExtraResponseKeys returns the effective list. A missing
// setting is the built-in list.
func (s *SettingService) GetClientRelayExtraResponseKeys(ctx context.Context) ([]string, error) {
	if s == nil {
		return DefaultClientRelayExtraResponseKeys(), nil
	}
	return cloneClientRelayExtraKeys(s.cachedClientRelayExtraResponseKeys(ctx)), nil
}

// SetClientRelayExtraResponseKeys persists the list and refreshes the cache.
// The saved list replaces the built-in names. An empty list strips nothing.
func (s *SettingService) SetClientRelayExtraResponseKeys(ctx context.Context, keys []string) ([]string, error) {
	normalized, err := NormalizeClientRelayExtraResponseKeys(keys)
	if err != nil {
		return nil, err
	}
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("setting repository is not configured")
	}
	payload, err := json.Marshal(struct {
		Keys []string `json:"keys"`
	}{Keys: normalized})
	if err != nil {
		return nil, fmt.Errorf("marshal client relay extra response keys: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyClientRelayExtraResponseKeys, string(payload)); err != nil {
		return nil, fmt.Errorf("set client relay extra response keys: %w", err)
	}
	invalidateClientRelayExtraKeysCache()
	storeClientRelayExtraKeysCache(normalized, clientRelayExtraKeysCacheTTL)
	return cloneClientRelayExtraKeys(normalized), nil
}
