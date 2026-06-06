package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type checkFailure struct {
	Check  string
	Detail string
}

type criticalSignature struct {
	Name     string
	Path     string
	Contains []string
}

var protectedPathNeedles = []string{
	"AGENTS.md",
	"CLAUDE.md",
	"docs/dev/",
	"scripts/dev-stack.",
	"deploy/docker-compose.a2-proxy.yml",
	"deploy/a2-proxy/",
	"frontend/pnpm-lock.yaml",
	"frontend/src/views/KeyUsageView.vue",
	"frontend/src/api/distribution.ts",
	"frontend/src/api/admin/distribution.ts",
	"frontend/src/views/user/DistributionView.vue",
	"frontend/src/views/admin/DistributionView.vue",
	"frontend/src/api/admin/modelPricing.ts",
	"frontend/src/api/admin/userModelPricing.ts",
	"frontend/src/components/admin/user/UserModelPricingModal.vue",
	"frontend/src/components/admin/user/UserEditModal.vue",
	"frontend/src/api/announcements.ts",
	"frontend/src/api/admin/announcements.ts",
	"frontend/src/views/admin/AnnouncementsView.vue",
	"backend/internal/handler/distribution_handler.go",
	"backend/internal/service/distribution.go",
	"backend/internal/repository/distribution_repo.go",
	"backend/internal/handler/dto/display_pricing.go",
	"backend/internal/service/global_model_pricing",
	"backend/internal/service/user_model_pricing",
	"backend/internal/repository/global_model_pricing_repo.go",
	"backend/internal/repository/user_model_pricing_repo.go",
	"backend/internal/handler/admin/model_pricing_handler.go",
	"backend/internal/handler/admin/user_model_pricing_handler.go",
	"backend/internal/service/credit_snapshot",
	"backend/internal/repository/credit_snapshot",
	"backend/ent/schema/ai_credit_snapshot.go",
	"backend/internal/service/openai_image_trace.go",
	"backend/internal/handler/announcement_handler.go",
	"backend/internal/handler/admin/announcement_handler.go",
	"backend/internal/service/announcement",
	"backend/internal/repository/announcement",
	"backend/ent/schema/announcement",
	"backend/migrations/090_create_global_model_pricing.sql",
	"backend/migrations/103_add_display_pricing_fields.sql",
	"backend/migrations/106_add_user_model_pricing_overrides.sql",
	"backend/migrations/110_add_ai_credit_snapshots.sql",
	"backend/migrations/139_distribution_agents.sql",
	"backend/migrations/140_add_distribution_assets.sql",
	"backend/migrations/148_extend_announcements_surfaces.sql",
}

var protectedPathCaseInsensitiveNeedles = []string{
	"aiclient2api",
	"invokeai",
	"a2-proxy",
	"distribution",
	"key-usage",
	"keyusage",
	"user_model_pricing",
	"model-pricing",
	"display_pricing",
	"ai_credit",
	"credit_snapshot",
	"announcement",
}

var criticalSignatures = []criticalSignature{
	{
		Name: "claude-gpt bridge routes",
		Path: "backend/internal/server/routes/gateway.go",
		Contains: []string{
			"ShouldUseClaudeGPTBridge",
			"MessagesClaudeGPTBridge",
			"ClaudeGPTBridgeFallbackRequested",
			"/antigravity/v1",
		},
	},
	{
		Name: "claude-gpt bridge handler",
		Path: "backend/internal/handler/openai_gateway_handler.go",
		Contains: []string{
			"openAIClaudeGPTBridgeContextKey",
			"MessagesClaudeGPTBridge",
			"ShouldUseClaudeGPTBridge",
			"SelectAccountWithSchedulerForClaudeGPTBridge",
			"ResolveClaudeGPTBridgeModel",
		},
	},
	{
		Name: "claude-gpt bridge account config",
		Path: "backend/internal/service/account.go",
		Contains: []string{
			"IsOpenAIClaudeGPTBridgeEnabled",
			"ResolveClaudeGPTBridgeModel",
			"openai_claude_gpt_bridge_enabled",
		},
	},
	{
		Name: "display token schema",
		Path: "backend/ent/schema/user.go",
		Contains: []string{
			"downstream_usage_token_mode",
			`"real", "display"`,
		},
	},
	{
		Name: "display token admin ui",
		Path: "frontend/src/components/admin/user/UserEditModal.vue",
		Contains: []string{
			"downstream_usage_token_mode",
			"DownstreamUsageTokenMode",
		},
	},
	{
		Name: "display pricing dto",
		Path: "backend/internal/handler/dto/display_pricing.go",
		Contains: []string{
			"ApplyDisplayTransform",
			"BuildUserDisplayPricingMap",
			"DisplayCacheReadPrice",
		},
	},
	{
		Name: "global model pricing display fields",
		Path: "backend/internal/service/global_model_pricing.go",
		Contains: []string{
			"DisplayInputPrice",
			"DisplayOutputPrice",
			"DisplayRateMultiplier",
		},
	},
	{
		Name: "user model pricing display fields",
		Path: "backend/internal/service/user_model_pricing.go",
		Contains: []string{
			"UserModelPricingOverride",
			"DisplayInputPrice",
			"DisplayRateMultiplier",
		},
	},
	{
		Name: "ai credit snapshot schema",
		Path: "backend/ent/schema/ai_credit_snapshot.go",
		Contains: []string{
			"AICreditSnapshot",
			"ai_credit_snapshots",
		},
	},
	{
		Name: "ai credit snapshot migration",
		Path: "backend/migrations/110_add_ai_credit_snapshots.sql",
		Contains: []string{
			"ai_credit_snapshots",
		},
	},
	{
		Name: "public key usage route",
		Path: "frontend/src/router/index.ts",
		Contains: []string{
			"path: '/key-usage'",
			"name: 'KeyUsage'",
			"requiresAuth: false",
		},
	},
	{
		Name: "public key usage page",
		Path: "frontend/src/views/KeyUsageView.vue",
		Contains: []string{
			"keyUsage",
		},
	},
	{
		Name: "distribution backend routes",
		Path: "backend/internal/server/routes/user.go",
		Contains: []string{
			`authenticated.Group("/distribution")`,
			"GenerateAPIKey",
		},
	},
	{
		Name: "usage error request routes",
		Path: "backend/internal/server/routes/user.go",
		Contains: []string{
			`usage.GET("/errors", h.Usage.ListErrors)`,
			`usage.GET("/errors/:id", h.Usage.GetErrorDetail)`,
		},
	},
	{
		Name: "admin group models list route",
		Path: "backend/internal/server/routes/admin.go",
		Contains: []string{
			`groups.GET("/:id/models-list-candidates", h.Admin.Group.GetModelsListCandidates)`,
		},
	},
	{
		Name: "group models list gateway",
		Path: "backend/internal/handler/gateway_handler.go",
		Contains: []string{
			"CustomModelsListEnabled",
			"filterModelsByCustomList",
			"writeCustomModelsList",
		},
	},
	{
		Name: "group models list frontend",
		Path: "frontend/src/views/admin/GroupsView.vue",
		Contains: []string{
			"GroupModelsListConfigPanel",
			"models_list_config",
			"getModelsListCandidates",
		},
	},
	{
		Name: "distribution admin routes",
		Path: "backend/internal/server/routes/admin.go",
		Contains: []string{
			`admin.Group("/distribution")`,
			"AdminGetSettings",
			"AdminListWallets",
		},
	},
	{
		Name: "distribution frontend route",
		Path: "frontend/src/router/index.ts",
		Contains: []string{
			"path: '/distribution'",
			"path: '/admin/distribution'",
		},
	},
	{
		Name: "announcement surfaces",
		Path: "backend/ent/schema/announcement.go",
		Contains: []string{
			`field.String("surface")`,
			`field.String("popup_frequency")`,
		},
	},
	{
		Name: "openai image trace",
		Path: "backend/internal/service/openai_image_trace.go",
		Contains: []string{
			"OPENAI_IMAGE_TRACE_LOG",
			"OpenAIImageTrace",
			"elapsed_ms",
		},
	},
	{
		Name: "local dev stack powershell",
		Path: "scripts/dev-stack.ps1",
		Contains: []string{
			"18081",
			"15174",
		},
	},
	{
		Name: "local dev stack cmd",
		Path: "scripts/dev-stack.cmd",
		Contains: []string{
			"dev-stack.ps1",
		},
	},
	{
		Name: "a2 proxy deployment",
		Path: "deploy/docker-compose.a2-proxy.yml",
		Contains: []string{
			"a2-proxy",
		},
	},
}

var historicDuplicateMigrations = map[string][]string{
	"006": {"006_add_users_allowed_groups_compat.sql", "006_fix_invalid_subscription_expires_at.sql", "006b_guard_users_allowed_groups.sql"},
	"028": {"028_add_account_notes.sql", "028_add_usage_logs_user_agent.sql", "028_group_image_pricing.sql"},
	"029": {"029_add_group_claude_code_restriction.sql", "029_usage_log_image_fields.sql"},
	"033": {"033_add_promo_codes.sql", "033_ops_monitoring_vnext.sql"},
	"034": {"034_ops_upstream_error_events.sql", "034_usage_dashboard_aggregation_tables.sql"},
	"036": {"036_ops_error_logs_add_is_count_tokens.sql", "036_scheduler_outbox.sql"},
	"037": {"037_add_account_rate_multiplier.sql", "037_ops_alert_silences.sql"},
	"042": {"042_add_usage_cleanup_tasks.sql", "042b_add_ops_system_metrics_switch_count.sql"},
	"043": {"043_add_usage_cleanup_cancel_audit.sql", "043b_add_group_invalid_request_fallback.sql"},
	"044": {"044_add_user_totp.sql", "044b_add_group_mcp_xml_inject.sql"},
	"045": {"045_add_accounts_extra_index.sql", "045_add_announcements.sql", "045_add_api_key_quota.sql"},
	"046": {"046_add_sora_accounts.sql", "046_add_usage_log_reasoning_effort.sql", "046b_add_group_supported_model_scopes.sql"},
	"047": {"047_add_sora_pricing_and_media_type.sql", "047_add_user_group_rate_multipliers.sql"},
	"052": {"052_add_group_sort_order.sql", "052_migrate_upstream_to_apikey.sql"},
	"053": {"053_add_security_secrets.sql", "053_add_skip_monitoring_to_error_passthrough.sql"},
	"054": {"054_drop_legacy_cache_columns.sql", "054_ops_system_logs.sql"},
	"060": {"060_add_gemini31_flash_image_to_model_mapping.sql", "060_add_usage_log_openai_ws_mode.sql"},
	"070": {"070_add_scheduled_test_auto_recover.sql", "070_add_usage_log_service_tier.sql"},
	"071": {"071_add_gemini25_flash_image_to_model_mapping.sql", "071_add_usage_billing_dedup.sql"},
	"075": {"075_add_usage_log_upstream_model.sql", "075_map_haiku45_to_sonnet46.sql"},
	"081": {"081_add_group_account_filter.sql", "081_create_channels.sql"},
	"090": {"090_create_global_model_pricing.sql", "090_drop_sora.sql"},
	"095": {"095_channel_features.sql", "095_subscription_plans.sql"},
	"101": {"101_add_account_stats_pricing.sql", "101_add_balance_notify_fields.sql", "101_add_channel_features_config.sql", "101_add_payment_mode.sql"},
	"102": {"102_add_balance_notify_threshold_type.sql", "102_add_out_trade_no_to_payment_orders.sql"},
	"103": {"103_add_allow_user_refund.sql", "103_add_display_pricing_fields.sql"},
	"104": {"104_add_display_rate_multiplier.sql", "104_migrate_notify_emails_to_struct.sql"},
	"105": {"105_add_proxy_pool_enabled.sql", "105_migrate_websearch_emulation_to_tristate.sql"},
	"106": {"106_add_account_stats_pricing_intervals.sql", "106_add_user_model_pricing_overrides.sql"},
	"107": {"107_add_account_cost_to_dashboard_tables.sql", "107_add_display_cache_read_price.sql"},
	"108": {"108_add_user_model_pricing_display_cache_read.sql", "108_auth_identity_foundation_core.sql", "108a_widen_auth_identity_migration_report_type.sql"},
	"109": {"109_add_show_on_pricing_page.sql", "109_auth_identity_compat_backfill.sql"},
	"110": {"110_add_ai_credit_snapshots.sql", "110_pending_auth_and_provider_default_grants.sql"},
	"120": {"120_enforce_payment_orders_out_trade_no_unique_notx.sql", "120a_align_payment_orders_out_trade_no_index_name.sql"},
	"125": {"125_add_channel_monitors.sql", "125_add_group_rpm_limit.sql"},
	"126": {"126_add_channel_monitor_aggregation.sql", "126_add_user_rpm_limit.sql"},
	"127": {"127_add_user_group_rpm_override.sql", "127_drop_channel_monitor_deleted_at.sql"},
}

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "upstream-sync-guard: %v\n", err)
		os.Exit(2)
	}

	var failures []checkFailure
	failures = append(failures, checkProtectedPathDeletion(root)...)
	failures = append(failures, checkHistoricalMigrationDiff(root)...)
	failures = append(failures, checkMigrationNumbers(root)...)
	failures = append(failures, checkCriticalSignatures(root)...)

	if len(failures) > 0 {
		fmt.Println("upstream-sync-guard: failed")
		for _, failure := range failures {
			fmt.Printf("- %s: %s\n", failure.Check, failure.Detail)
		}
		os.Exit(1)
	}

	fmt.Println("upstream-sync-guard: ok")
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, ".git")); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", fmt.Errorf("could not find git repository root")
		}
		wd = parent
	}
}

func checkProtectedPathDeletion(root string) []checkFailure {
	out, err := git(root, "diff", "--name-status", "--find-renames", "HEAD", "--")
	if err != nil {
		return []checkFailure{{Check: "git diff", Detail: err.Error()}}
	}
	var failures []checkFailure
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		status := parts[0]
		if strings.HasPrefix(status, "D") && len(parts) >= 2 && isProtectedPath(parts[1]) {
			failures = append(failures, checkFailure{Check: "protected path deletion", Detail: parts[1]})
			continue
		}
		if strings.HasPrefix(status, "R") && len(parts) >= 3 && isProtectedPath(parts[1]) && !isProtectedPath(parts[2]) {
			failures = append(failures, checkFailure{Check: "protected path rename", Detail: fmt.Sprintf("%s -> %s", parts[1], parts[2])})
		}
	}
	return failures
}

func checkHistoricalMigrationDiff(root string) []checkFailure {
	out, err := git(root, "diff", "--name-status", "--find-renames", "HEAD", "--", "backend/migrations")
	if err != nil {
		return []checkFailure{{Check: "migration diff", Detail: err.Error()}}
	}
	var failures []checkFailure
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		status := parts[0]
		paths := parts[1:]
		for _, path := range paths {
			num, ok := migrationNumber(filepath.Base(path))
			if !ok || num >= "150" {
				continue
			}
			if !strings.HasPrefix(status, "A") {
				failures = append(failures, checkFailure{
					Check:  "historical migration changed",
					Detail: fmt.Sprintf("%s %s; existing migrations below 150 must not be modified or removed", status, path),
				})
			}
		}
	}
	return failures
}

func checkMigrationNumbers(root string) []checkFailure {
	entries, err := os.ReadDir(filepath.Join(root, "backend", "migrations"))
	if err != nil {
		return []checkFailure{{Check: "migration scan", Detail: err.Error()}}
	}
	byNumber := make(map[string][]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		num, ok := migrationNumber(entry.Name())
		if !ok {
			continue
		}
		byNumber[num] = append(byNumber[num], entry.Name())
	}

	var failures []checkFailure
	for num, names := range byNumber {
		sort.Strings(names)
		if len(names) <= 1 {
			continue
		}
		if num >= "150" {
			failures = append(failures, checkFailure{
				Check:  "migration duplicate",
				Detail: fmt.Sprintf("%s has %d files: %s", num, len(names), strings.Join(names, ", ")),
			})
			continue
		}
		allowed, ok := historicDuplicateMigrations[num]
		if !ok || strings.Join(allowed, "\n") != strings.Join(names, "\n") {
			failures = append(failures, checkFailure{
				Check:  "unexpected historical migration duplicate",
				Detail: fmt.Sprintf("%s has files: %s", num, strings.Join(names, ", ")),
			})
		}
	}
	return failures
}

func checkCriticalSignatures(root string) []checkFailure {
	var failures []checkFailure
	for _, sig := range criticalSignatures {
		path := filepath.Join(root, filepath.FromSlash(sig.Path))
		data, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, checkFailure{Check: "critical signature", Detail: fmt.Sprintf("%s missing: %v", sig.Path, err)})
			continue
		}
		text := string(data)
		for _, needle := range sig.Contains {
			if !strings.Contains(text, needle) {
				failures = append(failures, checkFailure{Check: "critical signature", Detail: fmt.Sprintf("%s missing %q for %s", sig.Path, needle, sig.Name)})
			}
		}
	}
	return failures
}

func isProtectedPath(path string) bool {
	normalized := filepath.ToSlash(path)
	for _, needle := range protectedPathNeedles {
		if normalized == needle || strings.HasPrefix(normalized, needle) || strings.Contains(normalized, needle) {
			return true
		}
	}
	lower := strings.ToLower(normalized)
	for _, needle := range protectedPathCaseInsensitiveNeedles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

var migrationNumberPattern = regexp.MustCompile(`^(\d{3})[a-z]?_.*\.sql$`)

func migrationNumber(name string) (string, bool) {
	m := migrationNumberPattern.FindStringSubmatch(name)
	if len(m) != 2 {
		return "", false
	}
	return m[1], true
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
