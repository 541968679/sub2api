package main

import (
	"testing"
)

func TestForkLocalProtectionCatalog(t *testing.T) {
	representativePaths := []string{
		"backend/internal/service/image_channel_monitor_service.go",
		"frontend/src/views/admin/ImageChannelMonitorView.vue",
		"backend/internal/service/payment_config_plans.go",
		"backend/internal/service/openai_gateway_count_tokens.go",
		"backend/internal/service/codex_image_generation_bridge.go",
		"backend/internal/repository/usage_log_repo.go",
		"backend/internal/service/global_model_pricing.go",
		"frontend/src/components/admin/model-pricing/modelPricingRows.ts",
		"backend/migrations/173_add_cache_tier_pricing_fields.sql",
	}

	for _, path := range representativePaths {
		if !isProtectedPath(path) {
			t.Errorf("fork-local contract path is not protected: %s", path)
		}
	}
}

func TestCriticalSignaturesMatchCurrentCheckout(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range checkCriticalSignatures(root) {
		t.Errorf("%s: %s", failure.Check, failure.Detail)
	}
}

func TestBridgeCountTokensSignaturesBecomeMandatoryAfterFeatureCommit(t *testing.T) {
	want := map[string][]string{
		"backend/internal/handler/openai_gateway_count_tokens.go": {
			"CountTokensClaudeGPTBridge",
			"EstimateCountTokensClaudeGPTBridge",
		},
		"backend/internal/service/openai_gateway_count_tokens.go": {
			"ForwardCountTokensAsAnthropicClaudeGPTBridge",
			"bridge_no_schedulable_account",
		},
	}

	for path, needles := range want {
		var matched *criticalSignature
		for i := range criticalSignatures {
			if criticalSignatures[i].Path == path {
				matched = &criticalSignatures[i]
				break
			}
		}
		if matched == nil {
			t.Errorf("missing conditional bridge signature catalog entry: %s", path)
			continue
		}
		if matched.OptionalBeforeCommit != "b06190970" {
			t.Errorf("bridge signature must become mandatory after b06190970: %s", path)
		}
		for _, needle := range needles {
			if !containsString(matched.Contains, needle) {
				t.Errorf("bridge signature %s missing %q", path, needle)
			}
		}
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
