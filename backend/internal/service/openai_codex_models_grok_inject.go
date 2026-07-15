package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const grokCodexBaseInstructions = "You are a coding agent powered by Grok. Collaborate with the user until their goal is handled. Prefer clear, correct, minimal diffs. Follow repository conventions and existing patterns. When uncertain, inspect the code before changing it."

// ListGrokOpenAIGroupAccessModelIDs returns Grok text model IDs exposed to an
// OpenAI group via bound opt-in Grok accounts (OAuth and API-key).
func (s *OpenAIGatewayService) ListGrokOpenAIGroupAccessModelIDs(ctx context.Context, groupID *int64) []string {
	if s == nil || s.accountRepo == nil || groupID == nil {
		return nil
	}
	accounts, err := s.listSchedulableAccounts(ctx, groupID, PlatformGrok)
	if err != nil || len(accounts) == 0 {
		return nil
	}
	return CollectGrokOpenAIGroupAccessModelIDs(accounts)
}

// InjectGrokModelsIntoCodexManifest appends Codex ModelInfo entries for Grok
// text models into an upstream Codex models manifest body. Existing slugs are
// left untouched. Returns the original body when there is nothing to add.
func InjectGrokModelsIntoCodexManifest(body []byte, modelIDs []string) ([]byte, error) {
	if len(body) == 0 || len(modelIDs) == 0 {
		return body, nil
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, fmt.Errorf("parse codex models manifest: %w", err)
	}
	rawModels, ok := root["models"]
	if !ok {
		// Some caches wrap models under a different key; leave body unchanged.
		return body, nil
	}

	var models []map[string]any
	if err := json.Unmarshal(rawModels, &models); err != nil {
		return nil, fmt.Errorf("parse codex models array: %w", err)
	}

	existing := make(map[string]struct{}, len(models))
	for _, model := range models {
		if slug, _ := model["slug"].(string); strings.TrimSpace(slug) != "" {
			existing[strings.TrimSpace(slug)] = struct{}{}
		}
	}

	added := 0
	for _, id := range modelIDs {
		id = strings.TrimSpace(id)
		if id == "" || !IsGrokTextModel(id) {
			continue
		}
		if _, ok := existing[id]; ok {
			continue
		}
		models = append(models, buildCodexGrokModelEntry(id))
		existing[id] = struct{}{}
		added++
	}
	if added == 0 {
		return body, nil
	}

	modelsJSON, err := json.Marshal(models)
	if err != nil {
		return nil, err
	}
	root["models"] = modelsJSON
	out, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func buildCodexGrokModelEntry(slug string) map[string]any {
	display := humanizeGrokModelDisplayName(slug)
	// available_in_plans mirrors ChatGPT Codex entries so the client picker
	// does not hide custom slugs on free/plus/pro/team plans.
	plans := []string{
		"business", "edu", "edu_plus", "edu_pro", "education", "enterprise",
		"enterprise_cbp_automation", "enterprise_cbp_usage_based", "finserv",
		"free", "free_workspace", "go", "hc", "k12", "plus", "pro", "prolite",
		"quorum", "sci", "self_serve_business_usage_based", "team",
	}
	return map[string]any{
		"slug":                    slug,
		"display_name":            display,
		"description":             fmt.Sprintf("xAI %s via Sub2API (OpenAI-group Grok access).", display),
		// xAI Grok accepts low/medium/high only — do not advertise xhigh in Codex.
		"default_reasoning_level": "high",
		"supported_reasoning_levels": []map[string]any{
			{"effort": "low", "description": "Faster responses"},
			{"effort": "medium", "description": "Balanced"},
			{"effort": "high", "description": "Deeper reasoning"},
		},
		"shell_type":        "shell_command",
		"visibility":        "list",
		"supported_in_api":  true,
		"priority":          0,
		"available_in_plans": plans,
		"base_instructions":  grokCodexBaseInstructions,
		"model_messages": map[string]any{
			"instructions_template":  grokCodexBaseInstructions,
			"instructions_variables": map[string]any{},
			"approvals":              nil,
		},
		"context_window":                    1_000_000,
		"max_context_window":                1_000_000,
		"effective_context_window_percent":  90,
		"input_modalities":                  []string{"text", "image"},
		"supports_parallel_tool_calls":      true,
		"supports_reasoning_summaries":      true,
		"supports_reasoning_summary_parameter": true,
		"default_reasoning_summary":         "none",
		"reasoning_summary_format":          "none",
		"support_verbosity":                 true,
		"default_verbosity":                 "low",
		"apply_patch_tool_type":             "freeform",
		"tool_mode":                         "code_mode_only",
		"truncation_policy":                 map[string]any{"mode": "tokens", "limit": 10000},
		"supports_search_tool":              true,
		"web_search_tool_type":              "text_and_image",
		"supports_image_detail_original":    true,
		"use_responses_lite":                true,
		"prefer_websockets":                 false,
		"include_skills_usage_instructions": false,
		"experimental_supported_tools":      []any{},
		"auto_compact_token_limit":          nil,
		"auto_review_model_override":        nil,
		"upgrade":                           nil,
		"multi_agent_version":               "v2",
	}
}

func humanizeGrokModelDisplayName(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "Grok"
	}
	// grok-4.5 -> Grok 4.5; grok-build-0.1 -> Grok Build 0.1
	parts := strings.Split(slug, "-")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if part[0] >= 'a' && part[0] <= 'z' {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}
