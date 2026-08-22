package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsErrorLogsWhere_QueryUsesQualifiedColumns(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		Query: "ACCESS_DENIED",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "e.request_id ILIKE $") {
		t.Fatalf("where should include qualified request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.client_request_id ILIKE $") {
		t.Fatalf("where should include qualified client_request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.error_message ILIKE $") {
		t.Fatalf("where should include qualified error_message condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_UserQueryUsesExistsSubquery(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		UserQuery: "admin@",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "EXISTS (SELECT 1 FROM users u WHERE u.id = e.user_id AND u.email ILIKE $") {
		t.Fatalf("where should include EXISTS user email condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_BridgeAndUpstreamModel(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		Bridge:        "bridge",
		UpstreamModel: "gpt-5.4",
	}
	where, args := buildOpsErrorLogsWhere(filter)
	if !strings.Contains(where, "LOWER(COALESCE(e.platform,'')) IN ('antigravity','anthropic')") {
		t.Fatalf("missing bridge platform clause: %s", where)
	}
	if !strings.Contains(where, "LOWER(COALESCE(e.upstream_model,'')) LIKE 'gpt-%'") {
		t.Fatalf("missing bridge upstream clause: %s", where)
	}
	if !strings.Contains(where, "COALESCE(e.upstream_model,'') = $") {
		t.Fatalf("missing upstream_model exact filter: %s", where)
	}
	if len(args) < 1 {
		t.Fatalf("expected upstream_model arg")
	}
}

func TestBuildOpsErrorLogsWhere_DefaultHidesRecovered(t *testing.T) {
	where, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{})
	if !strings.Contains(where, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("default list must keep status>=400: %s", where)
	}
	if strings.Contains(where, "Recovered%") {
		t.Fatalf("default list must not include Recovered rows: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_IncludeRecoveredAddsFailoverRows(t *testing.T) {
	where, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{IncludeRecovered: true})
	if !strings.Contains(where, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("include_recovered must still list client failures: %s", where)
	}
	if !strings.Contains(where, "e.error_message ILIKE 'Recovered%'") {
		t.Fatalf("include_recovered must add Recovered predicate: %s", where)
	}
	if !strings.Contains(where, "error_phase") {
		t.Fatalf("include_recovered Recovered rows are phase=upstream: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_IncludeRecoveredWithUpstreamPhaseSkipsStatusGuard(t *testing.T) {
	where, args := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{
		IncludeRecovered: true,
		Phase:            "upstream",
	})
	if strings.Contains(where, "status_code, 0) >= 400") {
		t.Fatalf("phase=upstream + include_recovered should skip status>=400: %s", where)
	}
	if !strings.Contains(where, "e.error_phase = $") {
		t.Fatalf("phase=upstream must still filter error_phase: %s", where)
	}
	if len(args) < 1 || args[0] != "upstream" {
		t.Fatalf("expected upstream phase arg, got %#v", args)
	}
}

func TestSLAOpsErrorLogFilter_DropsRecoveredAndUpstreamPhase(t *testing.T) {
	phase := "upstream"
	in := &service.OpsErrorLogFilter{IncludeRecovered: true, Phase: phase, View: "errors"}
	out := slaOpsErrorLogFilter(in)
	if out.IncludeRecovered {
		t.Fatal("SLA filter must not include Recovered")
	}
	if out.Phase != "" {
		t.Fatalf("SLA filter must clear phase=upstream, got %q", out.Phase)
	}
	if in.IncludeRecovered == false || in.Phase != phase {
		t.Fatal("slaOpsErrorLogFilter must not mutate the input filter")
	}
	slaWhere, _ := buildOpsErrorLogsWhere(&out)
	if strings.Contains(slaWhere, "Recovered%") {
		t.Fatalf("SLA where must stay status>=400 only: %s", slaWhere)
	}
	if !strings.Contains(slaWhere, "COALESCE(e.status_code, 0) >= 400") {
		t.Fatalf("SLA where must keep client-visible status>=400: %s", slaWhere)
	}
}

func TestBuildOpsErrorLogsWhere_NeedsOpsAttention(t *testing.T) {
	yes := true
	where, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{NeedsOpsAttention: &yes})
	if !strings.Contains(where, "not supported by any configured account") {
		t.Fatalf("attention filter must use SQL pred: %s", where)
	}
	if !strings.Contains(where, "e.error_message") {
		t.Fatalf("attention filter must qualify columns: %s", where)
	}
	no := false
	notWhere, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{NeedsOpsAttention: &no})
	if !strings.Contains(notWhere, "NOT (") {
		t.Fatalf("false filter must negate attention pred: %s", notWhere)
	}
}

func TestIsClaudeGPTBridgeError(t *testing.T) {
	if !service.IsClaudeGPTBridgeError("antigravity", "gpt-5.4") {
		t.Fatal("expected bridge true")
	}
	if service.IsClaudeGPTBridgeError("openai", "gpt-5.5") {
		t.Fatal("native openai should not be bridge")
	}
	if service.IsClaudeGPTBridgeError("antigravity", "claude-sonnet-4-6") {
		t.Fatal("native antigravity should not be bridge")
	}
}
