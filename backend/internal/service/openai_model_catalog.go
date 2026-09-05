package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const OpenAIModelGPT6Astra = "gpt-6-astra"

// OpenAIModelCatalog is the editable OpenAI display list + default whitelist.
type OpenAIModelCatalog struct {
	DisplayModels      []string `json:"display_models"`
	WhitelistModels    []string `json:"whitelist_models"`
	MergedAccountCount int64    `json:"merged_account_count,omitempty"`
}

// IdentityModelMappingMerger merges missing identity model_mapping keys onto
// persisted accounts. Implemented by the account repository; optional on
// SettingService so AccountRepository stubs do not need the method.
type IdentityModelMappingMerger interface {
	MergeIdentityModelMappings(ctx context.Context, platform string, keys []string) (int64, error)
	CountIdentityModelMappingTargets(ctx context.Context, platform string, keys []string) (int64, error)
}

// OpenAIDisplaySeed is the unconfigured /v1/models curated list (includes gpt-6-astra).
func OpenAIDisplaySeed() []string {
	return []string{
		OpenAIModelGPT6Astra,
		"gpt-5.6-sol",
		"gpt-5.6-terra",
		"gpt-5.6-luna",
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
		"grok-4.5",
	}
}

// OpenAIWhitelistSeed is the unconfigured non-strict scheduling fallback.
// Grok IDs are intentionally omitted.
func OpenAIWhitelistSeed() []string {
	seed := append([]string{OpenAIModelGPT6Astra}, openai.DefaultModelIDs()...)
	return normalizeModelIDList(seed)
}

// OpenAILegacyWhitelistBaseline is the pre-catalog DefaultModels set used as
// the first-save merge diff so historical IDs are not stamped onto accounts.
func OpenAILegacyWhitelistBaseline() []string {
	return normalizeModelIDList(openai.DefaultModelIDs())
}

func defaultOpenAIModelCatalog() OpenAIModelCatalog {
	return NormalizeOpenAIModelCatalog(OpenAIDisplaySeed(), OpenAIWhitelistSeed())
}

// NormalizeOpenAIModelCatalog trims/dedupes, keeps display ⊆ whitelist for
// non-Grok IDs, and auto-appends missing non-Grok display IDs onto the whitelist.
func NormalizeOpenAIModelCatalog(display, whitelist []string) OpenAIModelCatalog {
	whitelist = normalizeModelIDList(whitelist)
	display = normalizeModelIDList(display)

	whitelistSet := modelIDSet(whitelist)
	filteredDisplay := make([]string, 0, len(display))
	for _, id := range display {
		if IsGrokTextModel(id) || whitelistSet[id] {
			filteredDisplay = append(filteredDisplay, id)
			continue
		}
		// Non-Grok display IDs missing from whitelist are auto-added (R3).
		whitelist = append(whitelist, id)
		whitelistSet[id] = true
		filteredDisplay = append(filteredDisplay, id)
	}

	return OpenAIModelCatalog{
		DisplayModels:   filteredDisplay,
		WhitelistModels: whitelist,
	}
}

func normalizeModelIDList(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
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

func modelIDSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// DiffNewWhitelistKeys returns IDs in next that are not in previous (order of next).
func DiffNewWhitelistKeys(previous, next []string) []string {
	prev := modelIDSet(normalizeModelIDList(previous))
	var added []string
	for _, id := range normalizeModelIDList(next) {
		if !prev[id] {
			added = append(added, id)
		}
	}
	return added
}

func isOpenAIDefaultWhitelistModel(modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	for _, id := range effectiveOpenAIWhitelistModels() {
		if id == modelID {
			return true
		}
	}
	return false
}

func effectiveOpenAIDisplayModels() []string {
	cat := effectiveOpenAIModelCatalog()
	out := make([]string, len(cat.DisplayModels))
	copy(out, cat.DisplayModels)
	return out
}

func effectiveOpenAIWhitelistModels() []string {
	cat := effectiveOpenAIModelCatalog()
	out := make([]string, len(cat.WhitelistModels))
	copy(out, cat.WhitelistModels)
	return out
}

func effectiveOpenAIModelCatalog() OpenAIModelCatalog {
	if resolver := openAIModelCatalogResolver; resolver != nil {
		if cat := resolver(); cat != nil && (len(cat.DisplayModels) > 0 || len(cat.WhitelistModels) > 0) {
			normalized := NormalizeOpenAIModelCatalog(cat.DisplayModels, cat.WhitelistModels)
			return normalized
		}
	}
	return defaultOpenAIModelCatalog()
}

var openAIModelCatalogResolver func() *OpenAIModelCatalog

// SetOpenAIModelCatalogResolver wires the Settings-backed catalog for hot-path reads.
func SetOpenAIModelCatalogResolver(resolver func() *OpenAIModelCatalog) {
	openAIModelCatalogResolver = resolver
}

func parseOpenAIModelCatalogJSON(raw string) (OpenAIModelCatalog, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return OpenAIModelCatalog{}, false
	}
	var cat OpenAIModelCatalog
	if err := json.Unmarshal([]byte(raw), &cat); err != nil {
		return OpenAIModelCatalog{}, false
	}
	if len(cat.DisplayModels) == 0 && len(cat.WhitelistModels) == 0 {
		return OpenAIModelCatalog{}, false
	}
	return NormalizeOpenAIModelCatalog(cat.DisplayModels, cat.WhitelistModels), true
}
