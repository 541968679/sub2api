package service

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// BuildDefaultAccountModelMapping returns the Add-Account-equivalent fallback
// mapping: catalog whitelist identity keys, then platform default rewrites
// (rewrites win). Grok uses xAI defaults when the catalog/platform tables are empty.
func BuildDefaultAccountModelMapping(platform string) map[string]string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	out := make(map[string]string)
	for _, id := range effectivePlatformWhitelistModels(platform) {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		out[id] = id
	}
	for key, value := range domain.ResolvePlatformDefaultModelMapping(platform) {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 && platform == PlatformGrok {
		for key, value := range xai.DefaultModelMapping() {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" || value == "" {
				continue
			}
			out[key] = value
		}
	}
	return out
}

// SeedDefaultAccountModelMapping writes BuildDefaultAccountModelMapping into
// credentials when mapping is empty. Passthrough OpenAI and non-empty mappings
// are left unchanged. Returns the (possibly newly allocated) credentials map.
func SeedDefaultAccountModelMapping(platform string, credentials, extra map[string]any) map[string]any {
	if extraEnablesOpenAIPassthrough(platform, extra) {
		if credentials == nil {
			return map[string]any{}
		}
		return credentials
	}
	if credentialsHaveModelMapping(credentials) {
		return credentials
	}
	built := BuildDefaultAccountModelMapping(platform)
	if len(built) == 0 {
		if credentials == nil {
			return map[string]any{}
		}
		return credentials
	}
	if credentials == nil {
		credentials = map[string]any{}
	}
	credentials["model_mapping"] = stringMapToAny(built)
	return credentials
}

func extraEnablesOpenAIPassthrough(platform string, extra map[string]any) bool {
	if strings.ToLower(strings.TrimSpace(platform)) != PlatformOpenAI || extra == nil {
		return false
	}
	if enabled, ok := extra["openai_passthrough"].(bool); ok && enabled {
		return true
	}
	if enabled, ok := extra["openai_oauth_passthrough"].(bool); ok && enabled {
		return true
	}
	return false
}

func credentialsHaveModelMapping(credentials map[string]any) bool {
	if credentials == nil {
		return false
	}
	raw, _ := credentials["model_mapping"].(map[string]any)
	for _, value := range raw {
		if strings.TrimSpace(fmt.Sprint(value)) != "" {
			return true
		}
	}
	return false
}

func stringMapToAny(src map[string]string) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
