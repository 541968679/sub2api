package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			"decision.MappedUpstreamModel",
		},
		"backend/internal/service/openai_gateway_count_tokens.go": {
			"ForwardCountTokensAsAnthropicClaudeGPTBridge",
			"bridge_no_schedulable_account",
			"openAIInputTokensEstimateMaxBytes",
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

func TestExplicitBaseDetectsCommittedProtectedDeletionAndHistoricalMigrationChange(t *testing.T) {
	root := initGuardTestRepo(t)
	writeGuardTestFile(t, root, "docs/dev/protected.md", "protected\n")
	writeGuardTestFile(t, root, "backend/migrations/001_initial.sql", "SELECT 1;\n")
	commitGuardTestRepo(t, root, "baseline")
	base := gitGuardTest(t, root, "rev-parse", "HEAD")

	if err := os.Remove(filepath.Join(root, "docs", "dev", "protected.md")); err != nil {
		t.Fatal(err)
	}
	writeGuardTestFile(t, root, "backend/migrations/001_initial.sql", "SELECT 2;\n")
	commitGuardTestRepo(t, root, "committed upstream merge")

	deletionFailures := checkProtectedPathDeletion(root, base)
	if !hasGuardFailure(deletionFailures, "protected path deletion", "docs/dev/protected.md") {
		t.Fatalf("expected committed protected deletion from %s..HEAD, got %#v", base, deletionFailures)
	}
	migrationFailures := checkHistoricalMigrationDiff(root, base)
	if !hasGuardFailure(migrationFailures, "historical migration changed", "001_initial.sql") {
		t.Fatalf("expected committed historical migration change from %s..HEAD, got %#v", base, migrationFailures)
	}
}

func TestDefaultDiffStillDetectsUncommittedChanges(t *testing.T) {
	root := initGuardTestRepo(t)
	writeGuardTestFile(t, root, "docs/dev/protected.md", "protected\n")
	writeGuardTestFile(t, root, "backend/migrations/001_initial.sql", "SELECT 1;\n")
	commitGuardTestRepo(t, root, "baseline")

	if err := os.Remove(filepath.Join(root, "docs", "dev", "protected.md")); err != nil {
		t.Fatal(err)
	}
	writeGuardTestFile(t, root, "backend/migrations/001_initial.sql", "SELECT 2;\n")

	if !hasGuardFailure(checkProtectedPathDeletion(root, ""), "protected path deletion", "docs/dev/protected.md") {
		t.Fatal("default mode must continue checking HEAD against the working tree")
	}
	if !hasGuardFailure(checkHistoricalMigrationDiff(root, ""), "historical migration changed", "001_initial.sql") {
		t.Fatal("default mode must continue checking uncommitted historical migration changes")
	}
}

func TestInvalidExplicitBaseReturnsClearGitFailure(t *testing.T) {
	root := initGuardTestRepo(t)
	writeGuardTestFile(t, root, "docs/dev/protected.md", "protected\n")
	commitGuardTestRepo(t, root, "baseline")

	failures := checkProtectedPathDeletion(root, "definitely-not-a-revision")
	if len(failures) != 1 || failures[0].Check != "git diff" {
		t.Fatalf("expected one git diff failure, got %#v", failures)
	}
	if !strings.Contains(failures[0].Detail, "definitely-not-a-revision..HEAD") {
		t.Fatalf("failure should name the invalid comparison range, got %q", failures[0].Detail)
	}
}

func hasGuardFailure(failures []checkFailure, check, detailPart string) bool {
	for _, failure := range failures {
		if failure.Check == check && strings.Contains(failure.Detail, detailPart) {
			return true
		}
	}
	return false
}

func initGuardTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitGuardTest(t, root, "init")
	return root
}

func writeGuardTestFile(t *testing.T, root, relativePath, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitGuardTestRepo(t *testing.T, root, message string) {
	t.Helper()
	gitGuardTest(t, root, "add", "--all")
	gitGuardTest(t, root, "-c", "user.name=Guard Test", "-c", "user.email=guard@example.invalid", "commit", "-m", message)
}

func gitGuardTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}
