package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInjectGrokModelsIntoCodexManifest_AppendsMissingSlugs(t *testing.T) {
	body := []byte(`{"models":[{"slug":"gpt-5.6-sol","display_name":"GPT-5.6-Sol"}],"client_version":"0.144.1"}`)
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
	require.Equal(t, true, models[1]["supports_reasoning_summaries"])
	require.Equal(t, "list", models[1]["visibility"])
	// Codex must not advertise xhigh for Grok (xAI only accepts low/medium/high).
	levels, ok := models[1]["supported_reasoning_levels"].([]map[string]any)
	if !ok {
		// json.Unmarshal into map[string]any uses []any for arrays.
		rawLevels, _ := models[1]["supported_reasoning_levels"].([]any)
		require.NotEmpty(t, rawLevels)
		for _, item := range rawLevels {
			level, _ := item.(map[string]any)
			require.NotEqual(t, "xhigh", level["effort"])
		}
	} else {
		for _, level := range levels {
			require.NotEqual(t, "xhigh", level["effort"])
		}
	}
	// Media models are excluded.
	for _, m := range models {
		require.NotEqual(t, "grok-imagine-image", m["slug"])
	}
}

func TestInjectGrokModelsIntoCodexManifest_NoOpWhenAlreadyPresent(t *testing.T) {
	body := []byte(`{"models":[{"slug":"grok-4.5","display_name":"Grok 4.5"}]}`)
	out, err := InjectGrokModelsIntoCodexManifest(body, []string{"grok-4.5"})
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(out))
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
