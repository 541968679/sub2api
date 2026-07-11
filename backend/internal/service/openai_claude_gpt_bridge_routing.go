package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// ClaudeGPTBridgeRouteState classifies how an Antigravity /v1/messages request
// relates to the OpenAI Claude-GPT bridge. It separates stable routing intent
// (an explicit bridge mapping is configured for this group and model) from
// instantaneous scheduler capacity (rate limit, overload, temporary pauses).
// Only not_configured may fall back to the native Antigravity pool; every
// other state must stay on bridge error semantics, otherwise a transient 429
// cooldown silently changes the upstream platform, model, and billing path.
type ClaudeGPTBridgeRouteState string

const (
	// ClaudeGPTBridgeRouteNotConfigured 表示该 group 对该模型没有任何 bridge
	// 配置候选（开关、绑定或显式映射缺失），保持原 native Antigravity 路由。
	ClaudeGPTBridgeRouteNotConfigured ClaudeGPTBridgeRouteState = "not_configured"
	// ClaudeGPTBridgeRouteReady 表示至少一个配置候选当前可调度。
	ClaudeGPTBridgeRouteReady ClaudeGPTBridgeRouteState = "ready"
	// ClaudeGPTBridgeRouteRateLimited 表示所有配置候选都只被未到期的
	// RateLimitResetAt 阻断。
	ClaudeGPTBridgeRouteRateLimited ClaudeGPTBridgeRouteState = "rate_limited"
	// ClaudeGPTBridgeRouteUnavailable 表示配置候选被 overload、临时暂停、
	// 过期、quota 或管理员停调等非纯限流状态阻断。
	ClaudeGPTBridgeRouteUnavailable ClaudeGPTBridgeRouteState = "unavailable"
	// ClaudeGPTBridgeRouteProbeError 表示诊断查询本身失败，此时不允许把
	// 未知状态当成“没有 bridge”。
	ClaudeGPTBridgeRouteProbeError ClaudeGPTBridgeRouteState = "probe_error"
)

// ClaudeGPTBridgeRouteDecision is the structured result of a bridge route
// diagnosis. Reason is an internal enum-like tag for logs only and must never
// carry account names, credentials, or quota details. MappedUpstreamModel is
// the mapped GPT model of the first candidate; it lets degraded paths (e.g.
// count_tokens local estimation) pick the right tokenizer without a second
// account scan.
type ClaudeGPTBridgeRouteDecision struct {
	State               ClaudeGPTBridgeRouteState
	CandidateCount      int
	SchedulableCount    int
	RateLimitedCount    int
	RetryAt             *time.Time
	Reason              string
	MappedUpstreamModel string
}

// ResolveClaudeGPTBridgeRoute diagnoses the bridge route for one request. It
// only reads account state via AccountRepository.ListByGroup — it never sends
// upstream requests and never acquires or releases scheduler slots, so it is
// safe to call from the route dispatch hot path and from selection-race
// re-diagnosis.
func (s *OpenAIGatewayService) ResolveClaudeGPTBridgeRoute(ctx context.Context, groupID *int64, requestedModel string) ClaudeGPTBridgeRouteDecision {
	requestedModel = strings.TrimSpace(requestedModel)
	if s == nil || s.accountRepo == nil {
		return ClaudeGPTBridgeRouteDecision{State: ClaudeGPTBridgeRouteProbeError, Reason: "missing_dependencies"}
	}
	if groupID == nil {
		return ClaudeGPTBridgeRouteDecision{State: ClaudeGPTBridgeRouteNotConfigured, Reason: "missing_group"}
	}
	if requestedModel == "" {
		return ClaudeGPTBridgeRouteDecision{State: ClaudeGPTBridgeRouteNotConfigured, Reason: "missing_model"}
	}

	// 与 scheduler 的候选池口径保持一致：simple 模式忽略分组绑定、
	// 使用平台全量账号；standard 模式使用绑定到当前分组的账号。
	var accounts []Account
	var err error
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		accounts, err = s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	} else {
		accounts, err = s.accountRepo.ListByGroup(ctx, *groupID)
	}
	if err != nil {
		return ClaudeGPTBridgeRouteDecision{State: ClaudeGPTBridgeRouteProbeError, Reason: "list_candidates_failed"}
	}

	decision := ClaudeGPTBridgeRouteDecision{}
	otherBlockedCount := 0
	for i := range accounts {
		account := &accounts[i]
		if !account.IsActive() {
			continue
		}
		mapped, ok := account.ResolveClaudeGPTBridgeModel(requestedModel)
		if !ok {
			continue
		}
		decision.CandidateCount++
		if decision.MappedUpstreamModel == "" {
			decision.MappedUpstreamModel = mapped
		}
		if account.IsSchedulable() {
			decision.SchedulableCount++
			continue
		}
		if claudeGPTBridgeCandidateRateLimitOnly(account) {
			decision.RateLimitedCount++
			if resetAt := account.RateLimitResetAt; resetAt != nil {
				if decision.RetryAt == nil || resetAt.Before(*decision.RetryAt) {
					reset := *resetAt
					decision.RetryAt = &reset
				}
			}
			continue
		}
		if account.IsSchedulable() {
			// 限流恰好在两次判定之间到期：按可调度处理，避免把它
			// 误分类成"既不可调度也非纯限流"而错误返回 503。
			decision.SchedulableCount++
			continue
		}
		otherBlockedCount++
	}

	switch {
	case decision.CandidateCount == 0:
		decision.State = ClaudeGPTBridgeRouteNotConfigured
		decision.Reason = "no_bridge_candidates"
		decision.RetryAt = nil
	case decision.SchedulableCount > 0:
		decision.State = ClaudeGPTBridgeRouteReady
		decision.Reason = "schedulable_candidate"
		decision.RetryAt = nil
	case otherBlockedCount == 0 && decision.RateLimitedCount > 0:
		decision.State = ClaudeGPTBridgeRouteRateLimited
		decision.Reason = "all_candidates_rate_limited"
	default:
		decision.State = ClaudeGPTBridgeRouteUnavailable
		decision.Reason = "candidates_blocked_non_rate_limit"
		decision.RetryAt = nil
	}
	return decision
}

// claudeGPTBridgeCandidateRateLimitOnly reports whether the account would be
// schedulable if its unexpired RateLimitResetAt were cleared. Reusing
// IsSchedulable on a shallow copy keeps this classification from drifting
// against the real scheduler predicate.
func claudeGPTBridgeCandidateRateLimitOnly(account *Account) bool {
	if account == nil || !account.IsRateLimited() {
		return false
	}
	clone := *account
	clone.RateLimitResetAt = nil
	return clone.IsSchedulable()
}

// SetAccountRepoForTest injects the account repository dependency for
// cross-package unit tests. Production wiring must keep using the Wire
// constructor.
func (s *OpenAIGatewayService) SetAccountRepoForTest(repo AccountRepository) {
	if s == nil {
		return
	}
	s.accountRepo = repo
}

// SetHTTPUpstreamForTest injects the HTTP upstream dependency for
// cross-package unit tests. Production wiring must keep using the Wire
// constructor.
func (s *OpenAIGatewayService) SetHTTPUpstreamForTest(upstream HTTPUpstream) {
	if s == nil {
		return
	}
	s.httpUpstream = upstream
}
