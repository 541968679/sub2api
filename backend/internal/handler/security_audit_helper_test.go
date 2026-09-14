package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSecurityAuditAllows_DefaultOff(t *testing.T) {
	cfg := defaultSecurityAuditConfig()
	require.Equal(t, securityaudit.ModeOff, securityaudit.EffectiveMode(cfg))
	require.True(t, securityAuditAllows(cfg, true))
	require.True(t, securityAuditAllows(cfg, false))
}

func TestSecurityAuditAllows_Blocking(t *testing.T) {
	cfg := securityaudit.Config{Mode: securityaudit.ModeBlocking}
	require.True(t, securityAuditAllows(cfg, false))
	require.False(t, securityAuditAllows(cfg, true))
}

func TestApplySecurityAuditToDecision_DefaultOffLeavesDecision(t *testing.T) {
	SetSecurityAuditConfig(securityaudit.DefaultConfig())
	t.Cleanup(func() { SetSecurityAuditConfig(securityaudit.DefaultConfig()) })

	in := &service.ContentModerationDecision{Allowed: true, Flagged: true, Action: "allow"}
	out := applySecurityAuditToDecision(in)
	require.Same(t, in, out)
	require.False(t, out.Blocked)
}

func TestApplySecurityAuditToDecision_BlockingConvertsFlagged(t *testing.T) {
	SetSecurityAuditConfig(securityaudit.Config{Mode: securityaudit.ModeBlocking})
	t.Cleanup(func() { SetSecurityAuditConfig(securityaudit.DefaultConfig()) })

	in := &service.ContentModerationDecision{Allowed: true, Flagged: true, Action: "allow", Blocked: false}
	out := applySecurityAuditToDecision(in)
	require.True(t, out.Blocked)
	require.False(t, out.Allowed)
	require.Equal(t, "security_audit_block", out.Action)
	require.Equal(t, 403, out.StatusCode)
}
