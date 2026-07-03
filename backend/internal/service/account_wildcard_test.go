//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		str      string
		expected bool
	}{
		// 精确匹配
		{"exact match", "claude-sonnet-4-5", "claude-sonnet-4-5", true},
		{"exact mismatch", "claude-sonnet-4-5", "claude-opus-4-5", false},

		// 通配符匹配
		{"wildcard prefix match", "claude-*", "claude-sonnet-4-5", true},
		{"wildcard prefix match 2", "claude-*", "claude-opus-4-5-thinking", true},
		{"wildcard prefix mismatch", "claude-*", "gemini-3-flash", false},
		{"wildcard partial match", "gemini-3*", "gemini-3-flash", true},
		{"wildcard partial match 2", "gemini-3*", "gemini-3-pro-image", true},
		{"wildcard partial mismatch", "gemini-3*", "gemini-2.5-flash", false},

		// 边界情况
		{"empty pattern exact", "", "", true},
		{"empty pattern mismatch", "", "claude", false},
		{"single star", "*", "anything", true},
		{"star at end only", "abc*", "abcdef", true},
		{"star at end empty suffix", "abc*", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchWildcard(tt.pattern, tt.str)
			if result != tt.expected {
				t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.str, result, tt.expected)
			}
		})
	}
}

func TestMatchWildcardMappingResult(t *testing.T) {
	tests := []struct {
		name           string
		mapping        map[string]string
		requestedModel string
		expected       string
		matched        bool
	}{
		// 精确匹配优先于通配符
		{
			name: "exact match takes precedence",
			mapping: map[string]string{
				"claude-sonnet-4-5": "claude-sonnet-4-5-exact",
				"claude-*":          "claude-default",
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-5-exact",
			matched:        true,
		},

		// 最长通配符优先
		{
			name: "longer wildcard takes precedence",
			mapping: map[string]string{
				"claude-*":         "claude-default",
				"claude-sonnet-*":  "claude-sonnet-default",
				"claude-sonnet-4*": "claude-sonnet-4-series",
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-series",
			matched:        true,
		},

		// 单个通配符
		{
			name: "single wildcard",
			mapping: map[string]string{
				"claude-*": "claude-mapped",
			},
			requestedModel: "claude-opus-4-5",
			expected:       "claude-mapped",
			matched:        true,
		},

		// 无匹配返回原始模型
		{
			name: "no match returns original",
			mapping: map[string]string{
				"claude-*": "claude-mapped",
			},
			requestedModel: "gemini-3-flash",
			expected:       "gemini-3-flash",
			matched:        false,
		},

		// 空映射返回原始模型
		{
			name:           "empty mapping returns original",
			mapping:        map[string]string{},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-5",
			matched:        false,
		},

		// Gemini 模型映射
		{
			name: "gemini wildcard mapping",
			mapping: map[string]string{
				"gemini-3*":   "gemini-3-pro-high",
				"gemini-2.5*": "gemini-2.5-flash",
			},
			requestedModel: "gemini-3-flash-preview",
			expected:       "gemini-3-pro-high",
			matched:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, matched := matchWildcardMappingResult(tt.mapping, tt.requestedModel)
			if result != tt.expected || matched != tt.matched {
				t.Errorf("matchWildcardMappingResult(%v, %q) = (%q, %v), want (%q, %v)", tt.mapping, tt.requestedModel, result, matched, tt.expected, tt.matched)
			}
		})
	}
}

func TestAccountIsModelSupported(t *testing.T) {
	tests := []struct {
		name           string
		platform       string
		credentials    map[string]any
		requestedModel string
		expected       bool
	}{
		// 无映射 = 允许所有
		{
			name:           "no mapping allows all",
			credentials:    nil,
			requestedModel: "any-model",
			expected:       true,
		},
		{
			name:           "empty mapping allows all",
			credentials:    map[string]any{},
			requestedModel: "any-model",
			expected:       true,
		},

		// 精确匹配
		{
			name: "exact match supported",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-sonnet-4-5": "target-model",
				},
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       true,
		},
		{
			name: "exact match not supported",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-sonnet-4-5": "target-model",
				},
			},
			requestedModel: "claude-opus-4-5",
			expected:       false,
		},

		// 通配符匹配
		{
			name: "wildcard match supported",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-*": "claude-sonnet-4-5",
				},
			},
			requestedModel: "claude-opus-4-5-thinking",
			expected:       true,
		},
		{
			name:     "gemini customtools alias matches normalized mapping",
			platform: PlatformGemini,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-3.1-pro-preview": "gemini-3.1-pro-preview",
				},
			},
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expected:       true,
		},
		{
			name: "wildcard match not supported",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-*": "claude-sonnet-4-5",
				},
			},
			requestedModel: "gemini-3-flash",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Platform:    tt.platform,
				Credentials: tt.credentials,
			}
			result := account.IsModelSupported(tt.requestedModel)
			if result != tt.expected {
				t.Errorf("IsModelSupported(%q) = %v, want %v", tt.requestedModel, result, tt.expected)
			}
		})
	}
}

func TestAccountGetMappedModel(t *testing.T) {
	tests := []struct {
		name           string
		platform       string
		credentials    map[string]any
		requestedModel string
		expected       string
	}{
		// 无映射 = 返回原始模型
		{
			name:           "no mapping returns original",
			credentials:    nil,
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-5",
		},
		{
			name:           "no mapping preserves gemini customtools model",
			platform:       PlatformGemini,
			credentials:    nil,
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expected:       "gemini-3.1-pro-preview-customtools",
		},

		// 精确匹配
		{
			name: "exact match",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-sonnet-4-5": "target-model",
				},
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "target-model",
		},

		// 通配符匹配（最长优先）
		{
			name: "wildcard longest match",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-*":        "claude-default",
					"claude-sonnet-*": "claude-sonnet-mapped",
				},
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-mapped",
		},

		// 无匹配返回原始模型
		{
			name:     "gemini customtools alias resolves through normalized mapping",
			platform: PlatformGemini,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-3.1-pro-preview": "gemini-3.1-pro-preview",
				},
			},
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expected:       "gemini-3.1-pro-preview",
		},
		{
			name:     "gemini customtools exact mapping wins over normalized fallback",
			platform: PlatformGemini,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-3.1-pro-preview":             "gemini-3.1-pro-preview",
					"gemini-3.1-pro-preview-customtools": "gemini-3.1-pro-preview-customtools",
				},
			},
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expected:       "gemini-3.1-pro-preview-customtools",
		},
		{
			name: "no match returns original",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-*": "gemini-mapped",
				},
			},
			requestedModel: "claude-sonnet-4-5",
			expected:       "claude-sonnet-4-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Platform:    tt.platform,
				Credentials: tt.credentials,
			}
			result := account.GetMappedModel(tt.requestedModel)
			if result != tt.expected {
				t.Errorf("GetMappedModel(%q) = %q, want %q", tt.requestedModel, result, tt.expected)
			}
		})
	}
}

func TestAccountResolveMappedModel(t *testing.T) {
	tests := []struct {
		name           string
		platform       string
		credentials    map[string]any
		requestedModel string
		expectedModel  string
		expectedMatch  bool
	}{
		{
			name:           "no mapping reports unmatched",
			credentials:    nil,
			requestedModel: "gpt-5.4",
			expectedModel:  "gpt-5.4",
			expectedMatch:  false,
		},
		{
			name: "exact passthrough mapping still counts as matched",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-5.4": "gpt-5.4",
				},
			},
			requestedModel: "gpt-5.4",
			expectedModel:  "gpt-5.4",
			expectedMatch:  true,
		},
		{
			name: "wildcard passthrough mapping still counts as matched",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-*": "gpt-5.4",
				},
			},
			requestedModel: "gpt-5.4",
			expectedModel:  "gpt-5.4",
			expectedMatch:  true,
		},
		{
			name:     "gemini customtools alias reports normalized match",
			platform: PlatformGemini,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-3.1-pro-preview": "gemini-3.1-pro-preview",
				},
			},
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expectedModel:  "gemini-3.1-pro-preview",
			expectedMatch:  true,
		},
		{
			name:     "gemini customtools exact mapping reports exact match",
			platform: PlatformGemini,
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-3.1-pro-preview":             "gemini-3.1-pro-preview",
					"gemini-3.1-pro-preview-customtools": "gemini-3.1-pro-preview-customtools",
				},
			},
			requestedModel: "gemini-3.1-pro-preview-customtools",
			expectedModel:  "gemini-3.1-pro-preview-customtools",
			expectedMatch:  true,
		},
		{
			name: "missing mapping reports unmatched",
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-5.2": "gpt-5.2",
				},
			},
			requestedModel: "gpt-5.4",
			expectedModel:  "gpt-5.4",
			expectedMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Platform:    tt.platform,
				Credentials: tt.credentials,
			}
			mappedModel, matched := account.ResolveMappedModel(tt.requestedModel)
			if mappedModel != tt.expectedModel || matched != tt.expectedMatch {
				t.Fatalf("ResolveMappedModel(%q) = (%q, %v), want (%q, %v)", tt.requestedModel, mappedModel, matched, tt.expectedModel, tt.expectedMatch)
			}
		})
	}
}

func TestAccountPlatformDefaultModelMapping(t *testing.T) {
	previousPlatformOverride := domain.GetPlatformDefaultMappingOverride
	previousAntigravityOverride := domain.GetAntigravityDefaultMappingOverride
	t.Cleanup(func() {
		domain.GetPlatformDefaultMappingOverride = previousPlatformOverride
		domain.GetAntigravityDefaultMappingOverride = previousAntigravityOverride
	})

	domain.GetAntigravityDefaultMappingOverride = nil
	domain.GetPlatformDefaultMappingOverride = func(platform string) map[string]string {
		switch platform {
		case PlatformOpenAI:
			return map[string]string{"claude-opus-4-8": "gpt-5.5"}
		case PlatformGemini:
			return map[string]string{"gemini-3.1-pro-preview": "gemini-3.1-pro-high"}
		default:
			return nil
		}
	}

	openAIAccount := &Account{Platform: PlatformOpenAI}
	if got := openAIAccount.GetMappedModel("claude-opus-4-8"); got != "gpt-5.5" {
		t.Fatalf("OpenAI default mapping GetMappedModel() = %q, want %q", got, "gpt-5.5")
	}
	if !openAIAccount.IsModelSupported("claude-opus-4-8") {
		t.Fatalf("OpenAI default mapping should make mapped request model schedulable")
	}
	if !openAIAccount.IsModelSupported("custom-openai-compatible-model") {
		t.Fatalf("OpenAI default mapping must not become a restrictive allowlist for unmatched models")
	}

	geminiAccount := &Account{Platform: PlatformGemini}
	if got := geminiAccount.GetMappedModel("gemini-3.1-pro-preview-customtools"); got != "gemini-3.1-pro-high" {
		t.Fatalf("Gemini default mapping GetMappedModel() = %q, want %q", got, "gemini-3.1-pro-high")
	}
	if !geminiAccount.IsModelSupported("gemini-unmapped-custom") {
		t.Fatalf("Gemini default mapping must not reject unmatched models")
	}

	antigravityAccount := &Account{Platform: PlatformAntigravity}
	if antigravityAccount.IsModelSupported("not-in-antigravity-defaults") {
		t.Fatalf("Antigravity default mapping should remain a strict allowlist")
	}
}

func TestOpenAIAccountResolveClaudeGPTBridgeModel(t *testing.T) {
	tests := []struct {
		name           string
		platform       string
		extra          map[string]any
		credentials    map[string]any
		requestedModel string
		expectedModel  string
		expectedOK     bool
	}{
		{
			name:     "enabled exact claude mapping resolves upstream gpt model",
			platform: PlatformOpenAI,
			extra:    map[string]any{"openai_claude_gpt_bridge_enabled": true},
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-opus-4-8": "gpt-5.5",
				},
			},
			requestedModel: "claude-opus-4-8",
			expectedModel:  "gpt-5.5",
			expectedOK:     true,
		},
		{
			name:     "enabled wildcard claude mapping resolves upstream gpt model",
			platform: PlatformOpenAI,
			extra:    map[string]any{"openai_claude_gpt_bridge_enabled": true},
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-opus-*": "gpt-5.5",
				},
			},
			requestedModel: "claude-opus-4-8",
			expectedModel:  "gpt-5.5",
			expectedOK:     true,
		},
		{
			name:     "disabled bridge is not eligible",
			platform: PlatformOpenAI,
			extra:    map[string]any{"openai_claude_gpt_bridge_enabled": false},
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-opus-4-8": "gpt-5.5",
				},
			},
			requestedModel: "claude-opus-4-8",
			expectedOK:     false,
		},
		{
			name:     "non openai account is not eligible",
			platform: PlatformAntigravity,
			extra:    map[string]any{"openai_claude_gpt_bridge_enabled": true},
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-opus-4-8": "gpt-5.5",
				},
			},
			requestedModel: "claude-opus-4-8",
			expectedOK:     false,
		},
		{
			name:     "missing mapping is not eligible",
			platform: PlatformOpenAI,
			extra:    map[string]any{"openai_claude_gpt_bridge_enabled": true},
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-sonnet-4-8": "gpt-5.5",
				},
			},
			requestedModel: "claude-opus-4-8",
			expectedOK:     false,
		},
		{
			name:     "passthrough mapping is not eligible",
			platform: PlatformOpenAI,
			extra:    map[string]any{"openai_claude_gpt_bridge_enabled": true},
			credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-opus-4-8": "claude-opus-4-8",
				},
			},
			requestedModel: "claude-opus-4-8",
			expectedOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{
				Platform:    tt.platform,
				Extra:       tt.extra,
				Credentials: tt.credentials,
			}

			mapped, ok := account.ResolveClaudeGPTBridgeModel(tt.requestedModel)
			if ok != tt.expectedOK || mapped != tt.expectedModel {
				t.Fatalf("ResolveClaudeGPTBridgeModel(%q) = (%q, %v), want (%q, %v)", tt.requestedModel, mapped, ok, tt.expectedModel, tt.expectedOK)
			}
		})
	}
}

func TestAccountGetModelMapping_AntigravityEnsuresGeminiDefaultPassthroughs(t *testing.T) {
	account := &Account{
		Platform: PlatformAntigravity,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gemini-3-pro-high": "gemini-3.1-pro-high",
			},
		},
	}

	mapping := account.GetModelMapping()
	if mapping["gemini-3-flash"] != "gemini-3-flash" {
		t.Fatalf("expected gemini-3-flash passthrough to be auto-filled, got: %q", mapping["gemini-3-flash"])
	}
	if mapping["gemini-3.1-pro-high"] != "gemini-3.1-pro-high" {
		t.Fatalf("expected gemini-3.1-pro-high passthrough to be auto-filled, got: %q", mapping["gemini-3.1-pro-high"])
	}
	if mapping["gemini-3.1-pro-low"] != "gemini-3.1-pro-low" {
		t.Fatalf("expected gemini-3.1-pro-low passthrough to be auto-filled, got: %q", mapping["gemini-3.1-pro-low"])
	}
}

func TestAccountGetModelMapping_AntigravityRespectsWildcardOverride(t *testing.T) {
	account := &Account{
		Platform: PlatformAntigravity,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gemini-3*": "gemini-3.1-pro-high",
			},
		},
	}

	mapping := account.GetModelMapping()
	if _, exists := mapping["gemini-3-flash"]; exists {
		t.Fatalf("did not expect explicit gemini-3-flash passthrough when wildcard already exists")
	}
	if _, exists := mapping["gemini-3.1-pro-high"]; exists {
		t.Fatalf("did not expect explicit gemini-3.1-pro-high passthrough when wildcard already exists")
	}
	if _, exists := mapping["gemini-3.1-pro-low"]; exists {
		t.Fatalf("did not expect explicit gemini-3.1-pro-low passthrough when wildcard already exists")
	}
	if mapped := account.GetMappedModel("gemini-3-flash"); mapped != "gemini-3.1-pro-high" {
		t.Fatalf("expected wildcard mapping to stay effective, got: %q", mapped)
	}
}

func TestAccountGetModelMapping_CacheInvalidatesOnCredentialsReplace(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-3-5-sonnet": "upstream-a",
			},
		},
	}

	first := account.GetModelMapping()
	if first["claude-3-5-sonnet"] != "upstream-a" {
		t.Fatalf("unexpected first mapping: %v", first)
	}

	account.Credentials = map[string]any{
		"model_mapping": map[string]any{
			"claude-3-5-sonnet": "upstream-b",
		},
	}
	second := account.GetModelMapping()
	if second["claude-3-5-sonnet"] != "upstream-b" {
		t.Fatalf("expected cache invalidated after credentials replace, got: %v", second)
	}
}

func TestAccountGetModelMapping_CacheInvalidatesOnMappingLenChange(t *testing.T) {
	rawMapping := map[string]any{
		"claude-sonnet": "sonnet-a",
	}
	account := &Account{
		Credentials: map[string]any{
			"model_mapping": rawMapping,
		},
	}

	first := account.GetModelMapping()
	if len(first) != 1 {
		t.Fatalf("unexpected first mapping length: %d", len(first))
	}

	rawMapping["claude-opus"] = "opus-b"
	second := account.GetModelMapping()
	if second["claude-opus"] != "opus-b" {
		t.Fatalf("expected cache invalidated after mapping len change, got: %v", second)
	}
}

func TestAccountGetModelMapping_CacheInvalidatesOnInPlaceValueChange(t *testing.T) {
	rawMapping := map[string]any{
		"claude-sonnet": "sonnet-a",
	}
	account := &Account{
		Credentials: map[string]any{
			"model_mapping": rawMapping,
		},
	}

	first := account.GetModelMapping()
	if first["claude-sonnet"] != "sonnet-a" {
		t.Fatalf("unexpected first mapping: %v", first)
	}

	rawMapping["claude-sonnet"] = "sonnet-b"
	second := account.GetModelMapping()
	if second["claude-sonnet"] != "sonnet-b" {
		t.Fatalf("expected cache invalidated after in-place value change, got: %v", second)
	}
}
