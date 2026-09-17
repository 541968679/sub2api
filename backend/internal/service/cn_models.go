package service

import "strings"

// CNCodingModelIDsForOpenAIGroupAccess is the extra candidate set for OpenAI-group
// custom /v1/models lists. Domestic groups stay on platform=openai; these IDs are
// selectable in the group form and may appear in GET /v1/models when chosen.
func CNCodingModelIDsForOpenAIGroupAccess() []string {
	return []string{
		"glm-5.3",
		"glm-5.3-flash",
		"glm-5.2",
		"glm-5.1",
		"glm-5",
		"glm-4.7",
		"glm-4.7-flash",
		"glm-4.6",
		"glm-4.5",
		"glm-4.5-air",
		"kimi-k3",
		"kimi-k2.6",
		"kimi-k2.5",
		"kimi-k2-thinking",
		"kimi-k2",
		"kimi-latest",
		"deepseek-v4-pro",
		"deepseek-v4-flash",
		"deepseek-chat",
		"deepseek-reasoner",
		"deepseek-v3.2",
		"MiniMax-M2.5",
		"MiniMax-M2.1",
		"MiniMax-M2",
		"MiniMax-M1",
	}
}

// DefaultModelIDsForCNPlatform is the fallback /v1/models and whitelist catalog
// for first-class Kimi / Zhipu / DeepSeek / MiniMax groups and accounts.
func DefaultModelIDsForCNPlatform(platform string) []string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformZhipu:
		return []string{
			"glm-5.3", "glm-5.3-flash", "glm-5.2", "glm-5.1", "glm-5",
			"glm-4.7", "glm-4.7-flash", "glm-4.6", "glm-4.5", "glm-4.5-air",
			"glm-4-flash", "glm-4-air", "glm-4-plus",
		}
	case PlatformKimi:
		return []string{
			"kimi-k3", "kimi-k2.6", "kimi-k2.5", "kimi-k2-thinking", "kimi-k2",
			"kimi-latest",
			"moonshot-v1-128k", "moonshot-v1-32k", "moonshot-v1-8k",
		}
	case PlatformDeepseek:
		return []string{
			"deepseek-v4-pro", "deepseek-v4-flash",
			"deepseek-chat", "deepseek-reasoner",
			"deepseek-v3.2", "deepseek-v3.1", "deepseek-v3",
			"deepseek-r1",
		}
	case PlatformMiniMax:
		return []string{
			"MiniMax-M2.5", "MiniMax-M2.1", "MiniMax-M2", "MiniMax-M1",
			"MiniMax-M3",
		}
	default:
		return nil
	}
}

// ModelPricingListSeedIDs is the admin 模型配置 pricing-list stub set for
// domestic coding IDs that LiteLLM does not ship yet. Rows are searchable and
// overridable; they do not invent billing prices.
func ModelPricingListSeedIDs() []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(ids []string) {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			key := strings.ToLower(id)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, id)
		}
	}
	add(CNCodingModelIDsForOpenAIGroupAccess())
	add(DefaultModelIDsForCNPlatform(PlatformZhipu))
	add(DefaultModelIDsForCNPlatform(PlatformKimi))
	add(DefaultModelIDsForCNPlatform(PlatformDeepseek))
	add(DefaultModelIDsForCNPlatform(PlatformMiniMax))
	return out
}
