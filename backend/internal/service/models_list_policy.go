package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

var gatewayModelDiscoveryIDsByPlatform = map[string][]string{
	PlatformOpenAI: {
		"gpt-5.6-sol",
		"gpt-5.6-terra",
		"gpt-5.6-luna",
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
	},
	PlatformAntigravity: {
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	},
	PlatformGrok: xai.DefaultModelIDs(),
}

var gatewayModelDiscoveryLegacyFullCustomListsByPlatform = map[string][]string{
	PlatformOpenAI: {
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
	},
}

// GatewayModelDiscoveryIDsForPlatform returns the curated public model IDs used
// by /v1/models-style model discovery. It is presentation-only and must not be
// used for scheduling, model access checks, mapping, billing, or usage.
func GatewayModelDiscoveryIDsForPlatform(platform string) ([]string, bool) {
	ids, ok := gatewayModelDiscoveryIDsByPlatform[platform]
	if !ok {
		return nil, false
	}
	out := make([]string, len(ids))
	copy(out, ids)
	return out, true
}

// ExpandGatewayModelDiscoveryCustomList upgrades stale full-default custom
// /v1/models lists when curated discovery grows. It deliberately does not
// expand intentionally narrowed custom lists.
func ExpandGatewayModelDiscoveryCustomList(platform string, selected []string) []string {
	out := make([]string, 0, len(selected))
	seen := make(map[string]struct{}, len(selected))
	for _, model := range selected {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}

	legacyFullList, ok := gatewayModelDiscoveryLegacyFullCustomListsByPlatform[platform]
	if !ok || !containsAllModelIDs(seen, legacyFullList) {
		return out
	}
	current, ok := GatewayModelDiscoveryIDsForPlatform(platform)
	if !ok {
		return out
	}

	expanded := make([]string, 0, len(current)+len(out))
	expandedSeen := make(map[string]struct{}, len(current)+len(out))
	for _, model := range current {
		if _, ok := expandedSeen[model]; ok {
			continue
		}
		expandedSeen[model] = struct{}{}
		expanded = append(expanded, model)
	}
	for _, model := range out {
		if _, ok := expandedSeen[model]; ok {
			continue
		}
		expandedSeen[model] = struct{}{}
		expanded = append(expanded, model)
	}
	return expanded
}

func containsAllModelIDs(seen map[string]struct{}, required []string) bool {
	for _, model := range required {
		if _, ok := seen[model]; !ok {
			return false
		}
	}
	return true
}
