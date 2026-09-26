package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// ModelsListCandidateSet is the admin picker for a group's custom /v1/models
// list. Models is every selectable ID. DefaultSelected is the curated subset
// checked when the group has no saved list, so enabling the feature does not
// publish the wider catalog by itself.
type ModelsListCandidateSet struct {
	Models          []string
	DefaultSelected []string
}

func buildGroupModelsListCandidates(platform string, accountModelIDs []string) ModelsListCandidateSet {
	curated := modelsListCuratedCandidates(platform)
	full := make([]string, 0, len(curated)+len(accountModelIDs)+32)
	full = append(full, curated...)
	full = append(full, modelsListExtraCandidateIDs(platform)...)
	full = append(full, accountModelIDs...)

	var defaults []string
	if len(curated) > 0 {
		defaults = curated
	} else if len(accountModelIDs) > 0 {
		defaults = accountModelIDs
	} else {
		defaults = defaultModelsListCandidatesForPlatform(platform)
		full = append(full, defaults...)
	}
	return ModelsListCandidateSet{
		Models:          normalizeModelsListCandidates(full),
		DefaultSelected: normalizeModelsListCandidates(defaults),
	}
}

// modelsListCuratedCandidates is the historical picker set: the platform
// discovery catalog, plus Grok text and domestic coding IDs on OpenAI groups.
func modelsListCuratedCandidates(platform string) []string {
	candidates, ok := GatewayModelDiscoveryIDsForPlatform(platform)
	if !ok {
		return nil
	}
	if platform == PlatformOpenAI {
		candidates = MergeModelIDsPreferFirst(candidates, GrokTextModelIDsForOpenAIGroupAccess())
		candidates = MergeModelIDsPreferFirst(candidates, CNCodingModelIDsForOpenAIGroupAccess())
	}
	return candidates
}

// modelsListExtraCandidateIDs adds concrete IDs the curated discovery list
// omits: builtin snapshots, whitelist entries, short Claude aliases, and
// Antigravity mapping keys. These stay unchecked until an operator selects them.
func modelsListExtraCandidateIDs(platform string) []string {
	switch platform {
	case PlatformOpenAI:
		out := append([]string{}, openai.DefaultModelIDs()...)
		out = append(out, effectivePlatformWhitelistModels(platform)...)
		for _, cnPlatform := range []string{PlatformZhipu, PlatformKimi, PlatformDeepseek, PlatformMiniMax} {
			out = append(out, DefaultModelIDsForCNPlatform(cnPlatform)...)
		}
		return out
	case PlatformAnthropic:
		out := append([]string{}, effectivePlatformWhitelistModels(platform)...)
		for short := range claude.ModelIDOverrides {
			out = append(out, short)
		}
		return out
	case PlatformGemini:
		out := append([]string{}, geminiDefaultModelIDs()...)
		out = append(out, effectivePlatformWhitelistModels(platform)...)
		return out
	case PlatformAntigravity:
		out := append([]string{}, antigravityDefaultMappingKeys()...)
		out = append(out, effectivePlatformWhitelistModels(platform)...)
		return out
	case PlatformGrok:
		out := xai.DefaultModelIDs()
		for key := range xai.DefaultModelMapping() {
			out = append(out, key)
		}
		return out
	case PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax:
		return DefaultModelIDsForCNPlatform(platform)
	default:
		return nil
	}
}

func collectConcreteModelIDsFromAccounts(accounts []Account) []string {
	out := make([]string, 0)
	for i := range accounts {
		for model := range accounts[i].GetModelMapping() {
			model, ok := concreteModelListID(model)
			if !ok {
				continue
			}
			out = append(out, model)
		}
	}
	return out
}

func concreteModelListID(raw string) (string, bool) {
	model := strings.TrimSpace(raw)
	if model == "" || strings.Contains(model, "*") || strings.ContainsAny(model, " \t\r\n") {
		return "", false
	}
	return model, true
}
