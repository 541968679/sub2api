package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// IsGrokTextModel reports whether model is a Grok *text* model that may be
// scheduled via OpenAI-group access. Media/video models (grok-imagine-*) are
// intentionally excluded from this path.
func IsGrokTextModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	if isGrokMediaModelID(model) {
		return false
	}
	if model == "grok" || strings.HasPrefix(model, "grok-") {
		return true
	}
	// Aliases that map into Grok text models (e.g. composer-2.5).
	if mapped, ok := xai.DefaultModelMapping()[model]; ok {
		mapped = strings.ToLower(strings.TrimSpace(mapped))
		if mapped == "" || isGrokMediaModelID(mapped) {
			return false
		}
		return mapped == "grok" || strings.HasPrefix(mapped, "grok-")
	}
	return false
}

func isGrokMediaModelID(model string) bool {
	return strings.Contains(model, "imagine")
}

// ResolveOpenAICompatibleSchedulePlatform chooses the account-pool platform for
// an OpenAI-compatible gateway request. When an OpenAI-group key asks for a Grok
// text model, scheduling uses the Grok pool with OpenAI-group access eligibility.
func ResolveOpenAICompatibleSchedulePlatform(keyPlatform, requestedModel string) (schedulePlatform string, requireGrokOpenAIGroupAccess bool) {
	keyPlatform = normalizeOpenAICompatiblePlatform(keyPlatform)
	if keyPlatform == PlatformOpenAI && IsGrokTextModel(requestedModel) {
		return PlatformGrok, true
	}
	return keyPlatform, false
}

// GrokTextModelIDsForOpenAIGroupAccess returns the default curated Grok text
// model IDs that may appear in OpenAI-group model discovery.
func GrokTextModelIDsForOpenAIGroupAccess() []string {
	all := xai.DefaultModelIDs()
	out := make([]string, 0, len(all))
	for _, id := range all {
		if IsGrokTextModel(id) {
			out = append(out, id)
		}
	}
	return out
}

// CollectGrokOpenAIGroupAccessModelIDs aggregates Grok text models exposed by
// opt-in Grok accounts (model_mapping keys, or default text models when mapping empty).
func CollectGrokOpenAIGroupAccessModelIDs(accounts []Account) []string {
	if len(accounts) == 0 {
		return nil
	}
	modelSet := make(map[string]struct{})
	hasAccessCandidate := false
	for i := range accounts {
		account := &accounts[i]
		if !account.IsGrokOpenAIGroupAccessEnabled() || !account.IsSchedulable() {
			continue
		}
		hasAccessCandidate = true
		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			for _, id := range GrokTextModelIDsForOpenAIGroupAccess() {
				modelSet[id] = struct{}{}
			}
			continue
		}
		for model := range mapping {
			model = strings.TrimSpace(model)
			if IsGrokTextModel(model) {
				modelSet[model] = struct{}{}
			}
		}
	}
	if !hasAccessCandidate || len(modelSet) == 0 {
		return nil
	}
	out := make([]string, 0, len(modelSet))
	for model := range modelSet {
		out = append(out, model)
	}
	return out
}

// MergeModelIDsPreferFirst merges extra model IDs onto base, preserving base
// order and appending new extras in the order provided (de-duplicated).
func MergeModelIDsPreferFirst(base, extra []string) []string {
	if len(extra) == 0 {
		return cloneStringSlice(base)
	}
	seen := make(map[string]struct{}, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, id := range base {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range extra {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
