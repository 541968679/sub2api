package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInjectGrokModelsIntoCodexManifest_AppendsMissingSlugs(t *testing.T) {
	body := []byte(`{"models":[{"slug":"gpt-5.6-sol","display_name":"GPT-5.6-Sol","available_in_plans":["plus","pro"],"additional_speed_tiers":["fast"],"service_tiers":[{"id":"priority","name":"Fast"}],"visibility":"list"}],"client_version":"0.144.1"}`)
	out, err := InjectGrokModelsIntoCodexManifest(body, []string{"grok-4.5", "grok-imagine-image", "grok-4.5"})
	require.NoError(t, err)

	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &root))
	require.Contains(t, root, "client_version")

	var models []map[string]any
	require.NoError(t, json.Unmarshal(root["models"], &models))
	require.Len(t, models, 2)
	require.Equal(t, "gpt-5.6-sol", models[0]["slug"])
	require.Equal(t, "grok-4.5", models[1]["slug"])
	require.Equal(t, "list", models[1]["visibility"])
	// Cloned from GPT template so Desktop plan/service-tier filters keep Grok visible.
	require.Equal(t, []any{"plus", "pro"}, models[1]["available_in_plans"])
	require.Equal(t, []any{"fast"}, models[1]["additional_speed_tiers"])
	// Advertise xhigh for Desktop picker when user has effort=xhigh selected;
	// gateway still clamps to high on the wire.
	rawLevels, _ := models[1]["supported_reasoning_levels"].([]any)
	require.NotEmpty(t, rawLevels)
	efforts := make([]string, 0, len(rawLevels))
	for _, item := range rawLevels {
		level, _ := item.(map[string]any)
		if e, ok := level["effort"].(string); ok {
			efforts = append(efforts, e)
		}
	}
	require.Contains(t, efforts, "xhigh")
	require.Contains(t, efforts, "high")
	// Media models are excluded.
	for _, m := range models {
		require.NotEqual(t, "grok-imagine-image", m["slug"])
	}
}

func TestInjectGrokModelsIntoCodexManifest_UpgradesIncompleteExisting(t *testing.T) {
	body := []byte(`{"models":[{"slug":"grok-4.5","display_name":"Grok 4.5","visibility":"list"}]}`)
	out, err := InjectGrokModelsIntoCodexManifest(body, []string{"grok-4.5"})
	require.NoError(t, err)

	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &root))
	var models []map[string]any
	require.NoError(t, json.Unmarshal(root["models"], &models))
	require.Len(t, models, 1)
	require.Equal(t, "grok-4.5", models[0]["slug"])
	require.NotEmpty(t, models[0]["available_in_plans"])
	require.Equal(t, []any{"fast"}, models[0]["additional_speed_tiers"])
	require.NotNil(t, models[0]["service_tiers"])
}

func TestInjectGrokModelsIntoCodexManifest_NoOpWhenComplete(t *testing.T) {
	// Build once, inject again — second pass must be a pure no-op (same bytes path via changed==0).
	body := []byte(`{"models":[{"slug":"gpt-5.6-sol","available_in_plans":["plus"],"additional_speed_tiers":["fast"],"service_tiers":[{"id":"priority","name":"Fast"}]}]}`)
	once, err := InjectGrokModelsIntoCodexManifest(body, []string{"grok-4.5"})
	require.NoError(t, err)
	twice, err := InjectGrokModelsIntoCodexManifest(once, []string{"grok-4.5"})
	require.NoError(t, err)
	require.Equal(t, string(once), string(twice))
}

func TestInjectGrokModelsIntoCodexManifest_EmptyIDs(t *testing.T) {
	body := []byte(`{"models":[{"slug":"gpt-5.6-sol"}]}`)
	out, err := InjectGrokModelsIntoCodexManifest(body, nil)
	require.NoError(t, err)
	require.Equal(t, body, out)
}

func TestHumanizeGrokModelDisplayName(t *testing.T) {
	require.Equal(t, "Grok 4.5", humanizeGrokModelDisplayName("grok-4.5"))
	require.Equal(t, "Grok Build 0.1", humanizeGrokModelDisplayName("grok-build-0.1"))
}

func TestEmptyCodexModelsManifestBody_InjectsGrok(t *testing.T) {
	body := EmptyCodexModelsManifestBody("0.144.1")
	out, err := InjectGrokModelsIntoCodexManifest(body, []string{"grok-4.5"})
	require.NoError(t, err)

	var root map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &root))
	require.Contains(t, root, "client_version")

	var models []map[string]any
	require.NoError(t, json.Unmarshal(root["models"], &models))
	require.Len(t, models, 1)
	require.Equal(t, "grok-4.5", models[0]["slug"])
	require.Equal(t, "list", models[0]["visibility"])
	require.Nil(t, models[0]["tool_mode"])
	require.Equal(t, false, models[0]["use_responses_lite"])
}
