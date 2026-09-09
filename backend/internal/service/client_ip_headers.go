package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
)

// DefaultClientIPHeaderCandidates are trusted proxy headers used when no custom
// list is configured (UPSYNC-152-170). Must stay aligned with pkg/ip defaults.
var DefaultClientIPHeaderCandidates = []string{
	"CF-Connecting-IP",
	"True-Client-IP",
	"X-Real-IP",
	"X-Forwarded-For",
}

// NormalizeClientIPHeaderList trims, de-duplicates (case-insensitive), and drops empties.
func NormalizeClientIPHeaderList(in []string) []string {
	if len(in) == 0 {
		return append([]string(nil), DefaultClientIPHeaderCandidates...)
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, h := range in {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		key := strings.ToLower(h)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, h)
	}
	if len(out) == 0 {
		return append([]string(nil), DefaultClientIPHeaderCandidates...)
	}
	return out
}

// ApplyClientIPHeaderSettings publishes the header order used by ip.GetClientIP
// (gateway, auth, usage attribution). Empty input restores defaults.
func ApplyClientIPHeaderSettings(headers []string) {
	ip.SetClientIPHeaderOrder(NormalizeClientIPHeaderList(headers))
}
