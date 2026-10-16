package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const gatewayHTTP1HostLimit = 64

type cachedGatewayHTTP1Hosts struct {
	hosts     []string
	expiresAt int64
}

var gatewayHTTP1HostsCache atomic.Value // *cachedGatewayHTTP1Hosts
var gatewayHTTP1HostsSF singleflight.Group

const (
	gatewayHTTP1HostsCacheTTL  = 30 * time.Second
	gatewayHTTP1HostsErrorTTL  = 5 * time.Second
	gatewayHTTP1HostsDBTimeout = 5 * time.Second
	gatewayHTTP1HostsCacheKey  = "gateway_http1_hosts"
)

func storeGatewayHTTP1HostsCache(hosts []string, ttl time.Duration) {
	copied := append([]string(nil), hosts...)
	gatewayHTTP1HostsCache.Store(&cachedGatewayHTTP1Hosts{
		hosts:     copied,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

func invalidateGatewayHTTP1HostsCache() {
	gatewayHTTP1HostsSF.Forget(gatewayHTTP1HostsCacheKey)
	storeGatewayHTTP1HostsCache(nil, 0)
}

// InvalidateGatewayHTTP1HostsCacheForTest clears the hot-path cache.
func InvalidateGatewayHTTP1HostsCacheForTest() {
	invalidateGatewayHTTP1HostsCache()
}

func loadCachedGatewayHTTP1Hosts() ([]string, bool) {
	cached, ok := gatewayHTTP1HostsCache.Load().(*cachedGatewayHTTP1Hosts)
	if !ok || cached == nil {
		return nil, false
	}
	if time.Now().UnixNano() >= cached.expiresAt {
		return nil, false
	}
	return cached.hosts, true
}

// NormalizeGatewayHTTP1Hosts trims, lowercases, and dedupes host patterns.
// A full URL is reduced to its hostname. "*.example.com" matches the apex
// and every subdomain. Blank entries are dropped.
func NormalizeGatewayHTTP1Hosts(hosts []string) ([]string, error) {
	if hosts == nil {
		hosts = []string{}
	}
	out := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, raw := range hosts {
		host, skip, err := normalizeGatewayHTTP1Host(raw)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		if len(out) >= gatewayHTTP1HostLimit {
			return nil, fmt.Errorf("http1 hosts exceed %d", gatewayHTTP1HostLimit)
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out, nil
}

func normalizeGatewayHTTP1Host(raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true, nil
	}
	wildcard := false
	if strings.HasPrefix(raw, "*.") {
		wildcard = true
		raw = strings.TrimSpace(raw[2:])
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
			return "", false, fmt.Errorf("invalid http1 host %q", raw)
		}
		raw = parsed.Hostname()
	}
	raw = strings.TrimSuffix(raw, "/")
	if strings.Contains(raw, "/") || strings.Contains(raw, " ") {
		return "", false, fmt.Errorf("invalid http1 host %q", raw)
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	raw = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	if raw == "" || strings.Contains(raw, "*") || strings.Contains(raw, ":") {
		return "", false, fmt.Errorf("invalid http1 host %q", raw)
	}
	if wildcard {
		return "*." + raw, false, nil
	}
	return raw, false, nil
}

// GatewayHTTP1HostMatches reports whether host is covered by patterns.
func GatewayHTTP1HostMatches(patterns []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" {
		return false
	}
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if pattern == host {
			return true
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := pattern[1:]
			apex := pattern[2:]
			if host == apex || strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}
	return false
}

func parseGatewayHTTP1HostsJSON(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}, nil
	}
	var object struct {
		Hosts []string `json:"hosts"`
	}
	if err := json.Unmarshal([]byte(raw), &object); err != nil {
		return nil, err
	}
	return NormalizeGatewayHTTP1Hosts(object.Hosts)
}

func (s *SettingService) loadGatewayHTTP1HostsFromRepo(ctx context.Context) []string {
	if s == nil || s.settingRepo == nil {
		storeGatewayHTTP1HostsCache(nil, gatewayHTTP1HostsCacheTTL)
		return []string{}
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayHTTP1HostsDBTimeout)
	defer cancel()
	value, err := s.settingRepo.GetValue(dbCtx, SettingKeyGatewayHTTP1Hosts)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			storeGatewayHTTP1HostsCache(nil, gatewayHTTP1HostsCacheTTL)
			return []string{}
		}
		slog.Warn("failed to get gateway_http1_hosts", "error", err)
		storeGatewayHTTP1HostsCache(nil, gatewayHTTP1HostsErrorTTL)
		return []string{}
	}
	hosts, err := parseGatewayHTTP1HostsJSON(value)
	if err != nil {
		slog.Warn("failed to parse gateway_http1_hosts", "error", err)
		storeGatewayHTTP1HostsCache(nil, gatewayHTTP1HostsErrorTTL)
		return []string{}
	}
	storeGatewayHTTP1HostsCache(hosts, gatewayHTTP1HostsCacheTTL)
	return hosts
}

// GetGatewayHTTP1Hosts returns the normalized host list. Missing key is empty.
func (s *SettingService) GetGatewayHTTP1Hosts(ctx context.Context) ([]string, error) {
	hosts := s.gatewayHTTP1HostsCached(ctx)
	return append([]string(nil), hosts...), nil
}

func (s *SettingService) gatewayHTTP1HostsCached(ctx context.Context) []string {
	if cached, ok := loadCachedGatewayHTTP1Hosts(); ok {
		return cached
	}
	if s == nil {
		return []string{}
	}
	result, _, _ := gatewayHTTP1HostsSF.Do(gatewayHTTP1HostsCacheKey, func() (any, error) {
		if cached, ok := loadCachedGatewayHTTP1Hosts(); ok {
			return cached, nil
		}
		return s.loadGatewayHTTP1HostsFromRepo(ctx), nil
	})
	hosts, _ := result.([]string)
	if hosts == nil {
		return []string{}
	}
	return hosts
}

// ForceHTTP1 reports whether this upstream hostname must use HTTP/1.1.
func (s *SettingService) ForceHTTP1(ctx context.Context, host string) bool {
	if s == nil {
		return false
	}
	return GatewayHTTP1HostMatches(s.gatewayHTTP1HostsCached(ctx), host)
}

// SetGatewayHTTP1Hosts persists the host list and refreshes the hot-path cache.
func (s *SettingService) SetGatewayHTTP1Hosts(ctx context.Context, hosts []string) ([]string, error) {
	normalized, err := NormalizeGatewayHTTP1Hosts(hosts)
	if err != nil {
		return nil, err
	}
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("setting repository is not configured")
	}
	payload, err := json.Marshal(struct {
		Hosts []string `json:"hosts"`
	}{Hosts: normalized})
	if err != nil {
		return nil, fmt.Errorf("marshal gateway http1 hosts: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyGatewayHTTP1Hosts, string(payload)); err != nil {
		return nil, fmt.Errorf("set gateway http1 hosts: %w", err)
	}
	invalidateGatewayHTTP1HostsCache()
	storeGatewayHTTP1HostsCache(normalized, gatewayHTTP1HostsCacheTTL)
	return append([]string(nil), normalized...), nil
}
