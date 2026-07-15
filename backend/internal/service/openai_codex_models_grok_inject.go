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

// InjectGrokModelsIntoCodexManifest appends or upgrades Codex ModelInfo entries
// for Grok text models into an upstream Codex models manifest body.
//
// Existing Grok slugs are replaced (not left stale) so Desktop clients that
// filter on available_in_plans / service_tiers always see a complete entry.
// Returns the original body when there is nothing to add or change.
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

	template := pickCodexModelTemplate(models)
	indexBySlug := make(map[string]int, len(models))
	for i, model := range models {
		if slug, _ := model["slug"].(string); strings.TrimSpace(slug) != "" {
			indexBySlug[strings.TrimSpace(slug)] = i
		}
	}

	changed := 0
	for _, id := range modelIDs {
		id = strings.TrimSpace(id)
		if id == "" || !IsGrokTextModel(id) {
			continue
		}
		entry := buildCodexGrokModelEntryFromTemplate(template, id)
		if idx, ok := indexBySlug[id]; ok {
			if codexModelEntriesEquivalent(models[idx], entry) {
				continue
			}
			models[idx] = entry
			changed++
			continue
		}
		models = append(models, entry)
		indexBySlug[id] = len(models) - 1
		changed++
	}
	if changed == 0 {
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

func pickCodexModelTemplate(models []map[string]any) map[string]any {
	preferred := []string{"gpt-5.6-sol", "gpt-5.5", "gpt-5.4", "gpt-5.3-codex"}
	bySlug := make(map[string]map[string]any, len(models))
	for _, model := range models {
		slug, _ := model["slug"].(string)
		slug = strings.TrimSpace(slug)
		if slug == "" {
			continue
		}
		bySlug[slug] = model
	}
	for _, slug := range preferred {
		if model, ok := bySlug[slug]; ok {
			return model
		}
	}
	// Prefer any listed GPT-like entry with plan metadata over hidden rows.
	var fallback map[string]any
	for _, model := range models {
		slug, _ := model["slug"].(string)
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(slug)), "grok") {
			continue
		}
		if vis, _ := model["visibility"].(string); strings.EqualFold(vis, "hide") {
			continue
		}
		if fallback == nil {
			fallback = model
		}
		if _, ok := model["available_in_plans"]; ok {
			return model
		}
	}
	return fallback
}

func buildCodexGrokModelEntryFromTemplate(template map[string]any, slug string) map[string]any {
	entry := cloneCodexModelEntry(template)
	if entry == nil {
		entry = map[string]any{}
	}

	display := humanizeGrokModelDisplayName(slug)
	entry["slug"] = slug
	entry["display_name"] = display
	entry["description"] = fmt.Sprintf("xAI %s via Sub2API (OpenAI-group Grok access).", display)
	// Advertise xhigh for Codex Desktop/CLI picker parity with GPT models.
	// Many users keep model_reasoning_effort=xhigh (or plan_mode xhigh); Desktop
	// hides models that omit the currently selected effort. Upstream Grok only
	// accepts low/medium/high — gateway clamps xhigh→high on the wire.
	entry["default_reasoning_level"] = "high"
	entry["supported_reasoning_levels"] = []map[string]any{
		{"effort": "low", "description": "Faster responses"},
		{"effort": "medium", "description": "Balanced"},
		{"effort": "high", "description": "Deeper reasoning"},
		{"effort": "xhigh", "description": "Extra high (mapped to high on Grok)"},
	}
	entry["base_instructions"] = grokCodexBaseInstructions
	entry["model_messages"] = map[string]any{
		"instructions_template":  grokCodexBaseInstructions,
		"instructions_variables": map[string]any{},
		"approvals":              nil,
	}
	entry["visibility"] = "list"
	entry["supported_in_api"] = true
	// Keep Grok near the top of Desktop pickers without outranking official defaults.
	if _, ok := entry["priority"]; !ok {
		entry["priority"] = 0
	}
	if _, ok := entry["shell_type"]; !ok {
		entry["shell_type"] = "shell_command"
	}
	if _, ok := entry["context_window"]; !ok {
		entry["context_window"] = 1_000_000
	}
	if _, ok := entry["max_context_window"]; !ok {
		entry["max_context_window"] = 1_000_000
	}

	// Desktop filters hard on plan membership. Incomplete/null plans hide the
	// slug even when the user already selected it as the default model.
	if !hasNonEmptyStringSlice(entry["available_in_plans"]) {
		entry["available_in_plans"] = defaultCodexAvailableInPlans()
	}

	// config.toml service_tier=fast is common; models without matching
	// additional_speed_tiers/service_tiers are dropped from the picker.
	if !hasNonEmptyStringSlice(entry["additional_speed_tiers"]) {
		entry["additional_speed_tiers"] = []string{"fast"}
	}
	if entry["service_tiers"] == nil {
		entry["service_tiers"] = []map[string]any{
			{"id": "priority", "name": "Fast", "description": "1.5x speed, increased usage"},
		}
	}

	// Strip fields that only make sense for OpenAI-owned upgrades.
	entry["upgrade"] = nil
	// Prefer stable non-websocket path for custom Grok bridge.
	entry["prefer_websockets"] = false
	// Match official GPT catalog rows used by Desktop: null tool_mode and
	// use_responses_lite=false. code_mode_only + lite=true made Grok invisible
	// in some Desktop picker surfaces while GPT models remained listed.
	entry["tool_mode"] = nil
	entry["use_responses_lite"] = false

	return entry
}

// EmptyCodexModelsManifestBody returns a minimal Codex models catalog shell so
// Grok injection still works when ChatGPT's /codex/models upstream is down.
func EmptyCodexModelsManifestBody(clientVersion string) []byte {
	clientVersion = strings.TrimSpace(clientVersion)
	if clientVersion == "" {
		clientVersion = "0.0.0"
	}
	// Keep the shape Desktop/CLI expect: top-level models array + client_version.
	return []byte(`{"models":[],"client_version":` + jsonString(clientVersion) + `}`)
}

func jsonString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

func buildCodexGrokModelEntry(slug string) map[string]any {
	return buildCodexGrokModelEntryFromTemplate(nil, slug)
}

func cloneCodexModelEntry(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return nil
	}
	var dst map[string]any
	if err := json.Unmarshal(raw, &dst); err != nil {
		return nil
	}
	return dst
}

func hasNonEmptyStringSlice(v any) bool {
	switch typed := v.(type) {
	case []string:
		return len(typed) > 0
	case []any:
		for _, item := range typed {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				return true
			}
		}
	}
	return false
}

func defaultCodexAvailableInPlans() []string {
	return []string{
		"business", "edu", "edu_plus", "edu_pro", "education", "enterprise",
		"enterprise_cbp_automation", "enterprise_cbp_usage_based", "finserv",
		"free", "free_workspace", "go", "hc", "k12", "plus", "pro", "prolite",
		"quorum", "sci", "self_serve_business_usage_based", "team",
	}
}

func codexModelEntriesEquivalent(a, b map[string]any) bool {
	// Cheap structural compare after canonical JSON encoding.
	ra, errA := json.Marshal(a)
	rb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ra) == string(rb)
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
