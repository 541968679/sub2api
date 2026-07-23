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
