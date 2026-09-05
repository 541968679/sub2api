package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
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
	return defaultPlatformModelCatalog(PlatformOpenAI)
}

func defaultPlatformModelCatalog(platform string) OpenAIModelCatalog {
	return NormalizePlatformModelCatalog(platform, PlatformDisplaySeed(platform), PlatformWhitelistSeed(platform))
}

// CatalogPlatforms are the platforms with an editable /v1/models catalog.
func CatalogPlatforms() []string {
	return []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity}
}

func isCatalogPlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity:
		return true
	default:
		return false
	}
}

func catalogSettingKey(platform string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformOpenAI:
		return SettingKeyOpenAIModelCatalog, true
	case PlatformAnthropic:
		return SettingKeyAnthropicModelCatalog, true
	case PlatformGemini:
		return SettingKeyGeminiModelCatalog, true
	case PlatformAntigravity:
		return SettingKeyAntigravityModelCatalog, true
	default:
		return "", false
	}
}

// AntigravityDisplaySeed is the unconfigured Antigravity /v1/models curated list.
func AntigravityDisplaySeed() []string {
	return []string{
		"claude-opus-5",
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	}
}

func geminiDefaultModelIDs() []string {
	ids := make([]string, 0, len(geminicli.DefaultModels))
	for _, model := range geminicli.DefaultModels {
		ids = append(ids, model.ID)
	}
	return ids
}

func antigravityDefaultMappingKeys() []string {
	keys := make([]string, 0, len(domain.DefaultAntigravityModelMapping))
	for key := range domain.DefaultAntigravityModelMapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// PlatformDisplaySeed is the unconfigured curated /v1/models list for a platform.
func PlatformDisplaySeed(platform string) []string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformOpenAI:
		return OpenAIDisplaySeed()
	case PlatformAnthropic:
		return claude.DefaultModelIDs()
	case PlatformGemini:
		return geminiDefaultModelIDs()
	case PlatformAntigravity:
		return AntigravityDisplaySeed()
	default:
		return nil
	}
}

// PlatformWhitelistSeed is the unconfigured non-strict scheduling fallback.
func PlatformWhitelistSeed(platform string) []string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformOpenAI:
		return OpenAIWhitelistSeed()
	case PlatformAnthropic:
		return claude.DefaultModelIDs()
	case PlatformGemini:
		return geminiDefaultModelIDs()
	case PlatformAntigravity:
		return antigravityDefaultMappingKeys()
	default:
		return nil
	}
}

// PlatformLegacyWhitelistBaseline is the pre-catalog default set used as the
// first-save merge diff so historical IDs are not stamped onto accounts.
func PlatformLegacyWhitelistBaseline(platform string) []string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformOpenAI:
		return OpenAILegacyWhitelistBaseline()
	case PlatformAnthropic:
		return claude.DefaultModelIDs()
	case PlatformGemini:
		return geminiDefaultModelIDs()
	case PlatformAntigravity:
		return antigravityDefaultMappingKeys()
	default:
		return nil
	}
}

func catalogSkipsWhitelistAutoAdd(platform, id string) bool {
	return strings.ToLower(strings.TrimSpace(platform)) == PlatformOpenAI && IsGrokTextModel(id)
}

// NormalizeOpenAIModelCatalog trims/dedupes, keeps display ⊆ whitelist for
// non-Grok IDs, and auto-appends missing non-Grok display IDs onto the whitelist.
func NormalizeOpenAIModelCatalog(display, whitelist []string) OpenAIModelCatalog {
	return NormalizePlatformModelCatalog(PlatformOpenAI, display, whitelist)
}

// NormalizePlatformModelCatalog applies the same two-list rules as OpenAI.
// OpenAI still skips auto-adding Grok IDs to the whitelist.
func NormalizePlatformModelCatalog(platform string, display, whitelist []string) OpenAIModelCatalog {
	whitelist = normalizeModelIDList(whitelist)
	display = normalizeModelIDList(display)

	whitelistSet := modelIDSet(whitelist)
	filteredDisplay := make([]string, 0, len(display))
	for _, id := range display {
		if catalogSkipsWhitelistAutoAdd(platform, id) || whitelistSet[id] {
			filteredDisplay = append(filteredDisplay, id)
			continue
		}
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
	return isPlatformDefaultWhitelistModel(PlatformOpenAI, modelID)
}

func isPlatformDefaultWhitelistModel(platform, modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" || !isCatalogPlatform(platform) {
		return false
	}
	for _, id := range effectivePlatformWhitelistModels(platform) {
		if id == modelID {
			return true
		}
	}
	return false
}

func effectiveOpenAIDisplayModels() []string {
	return effectivePlatformDisplayModels(PlatformOpenAI)
}

func effectiveOpenAIWhitelistModels() []string {
	return effectivePlatformWhitelistModels(PlatformOpenAI)
}

func effectiveOpenAIModelCatalog() OpenAIModelCatalog {
	return effectivePlatformModelCatalog(PlatformOpenAI)
}

func effectivePlatformDisplayModels(platform string) []string {
	cat := effectivePlatformModelCatalog(platform)
	out := make([]string, len(cat.DisplayModels))
	copy(out, cat.DisplayModels)
	return out
}

func effectivePlatformWhitelistModels(platform string) []string {
	cat := effectivePlatformModelCatalog(platform)
	out := make([]string, len(cat.WhitelistModels))
	copy(out, cat.WhitelistModels)
	return out
}

func effectivePlatformModelCatalog(platform string) OpenAIModelCatalog {
	if resolver := platformModelCatalogResolver; resolver != nil {
		if cat := resolver(platform); cat != nil && (len(cat.DisplayModels) > 0 || len(cat.WhitelistModels) > 0) {
			return NormalizePlatformModelCatalog(platform, cat.DisplayModels, cat.WhitelistModels)
		}
	}
	return defaultPlatformModelCatalog(platform)
}

var platformModelCatalogResolver func(platform string) *OpenAIModelCatalog

// SetOpenAIModelCatalogResolver wires the Settings-backed OpenAI catalog.
func SetOpenAIModelCatalogResolver(resolver func() *OpenAIModelCatalog) {
	if resolver == nil {
		platformModelCatalogResolver = nil
		return
	}
	platformModelCatalogResolver = func(platform string) *OpenAIModelCatalog {
		if platform != PlatformOpenAI {
			return nil
		}
		return resolver()
	}
}

// SetPlatformModelCatalogResolver wires Settings-backed catalogs for all platforms.
func SetPlatformModelCatalogResolver(resolver func(platform string) *OpenAIModelCatalog) {
	platformModelCatalogResolver = resolver
}

func parseOpenAIModelCatalogJSON(raw string) (OpenAIModelCatalog, bool) {
	return parsePlatformModelCatalogJSON(PlatformOpenAI, raw)
}

func parsePlatformModelCatalogJSON(platform, raw string) (OpenAIModelCatalog, bool) {
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
	return NormalizePlatformModelCatalog(platform, cat.DisplayModels, cat.WhitelistModels), true
}
