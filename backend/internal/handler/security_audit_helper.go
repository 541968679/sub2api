package handler

import (
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// securityAuditConfig holds process-level mode (default off).
// Published from settings load or tests via SetSecurityAuditConfig.
var securityAuditConfig atomic.Value // stores securityaudit.Config

func init() {
	securityAuditConfig.Store(securityaudit.DefaultConfig())
}

// SetSecurityAuditConfig publishes the gateway security-audit mode snapshot.
func SetSecurityAuditConfig(cfg securityaudit.Config) {
	if cfg.Mode == "" {
		cfg.Mode = securityaudit.ModeOff
	}
	securityAuditConfig.Store(cfg)
}

// currentSecurityAuditConfig returns the active process snapshot.
func currentSecurityAuditConfig() securityaudit.Config {
	if v := securityAuditConfig.Load(); v != nil {
		if cfg, ok := v.(securityaudit.Config); ok {
			return cfg
		}
	}
	return securityaudit.DefaultConfig()
}

// securityAuditAllows reports whether the gateway may continue after security audit.
// Default config is ModeOff (allow). Blocking only denies when flagged=true.
func securityAuditAllows(cfg securityaudit.Config, flagged bool) bool {
	return securityaudit.Evaluate(cfg, flagged).AllowNextStage
}

// applySecurityAuditToDecision merges security-audit policy onto a content-moderation
// decision for gateway handlers. When audit mode is off/observe, decision is unchanged.
// When blocking and the request is flagged, returns a blocked decision for the shared
// error path (content_policy_violation).
func applySecurityAuditToDecision(decision *service.ContentModerationDecision) *service.ContentModerationDecision {
	cfg := currentSecurityAuditConfig()
	flagged := decision != nil && decision.Flagged
	eval := securityaudit.Evaluate(cfg, flagged)
	if eval.AllowNextStage {
		return decision
	}
	msg := eval.Message
	if msg == "" {
		msg = "content blocked by security audit policy"
	}
	status := eval.StatusCode
	if status < 400 || status > 599 {
		status = 403
	}
	return &service.ContentModerationDecision{
		Allowed:    false,
		Blocked:    true,
		Flagged:    true,
		Message:    msg,
		StatusCode: status,
		Action:     "security_audit_block",
	}
}

// defaultSecurityAuditConfig is process default: off.
func defaultSecurityAuditConfig() securityaudit.Config {
	return securityaudit.DefaultConfig()
}
