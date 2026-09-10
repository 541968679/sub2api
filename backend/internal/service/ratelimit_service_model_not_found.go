package service

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

const (
	upstreamModelNotFoundCooldown       = 30 * time.Minute
	upstreamCodexPlanGatedModelCooldown = 30 * time.Minute
	upstreamModelNotFoundReason         = "upstream_404_model_not_found"
	upstreamCodexPlanGatedModelReason   = "upstream_400_codex_plan_gated_model"
)

// HandleUpstreamModelNotFound marks the requested model as temporarily
// unavailable when upstream deterministically cannot serve it: a 404
// model-not-found, or the Codex 400 rejecting a plan-gated model on a
// ChatGPT OAuth account. Returning true tells the caller to fail the current
// attempt over. Fork SetModelRateLimit has no reason argument.
func (s *RateLimitService) HandleUpstreamModelNotFound(ctx context.Context, account *Account, requestedModel string, statusCode int, responseBody []byte) bool {
	if s == nil || account == nil || s.accountRepo == nil {
		return false
	}
	if !account.ShouldHandleErrorCode(statusCode) {
		return false
	}
	var cooldown time.Duration
	var reason string
	switch {
	case isUpstreamModelNotFoundError(statusCode, responseBody):
		cooldown, reason = upstreamModelNotFoundCooldown, upstreamModelNotFoundReason
	case isOpenAIOAuthAccount(account) && isOpenAICodexPlanGatedModelError(statusCode, responseBody):
		cooldown, reason = upstreamCodexPlanGatedModelCooldown, upstreamCodexPlanGatedModelReason
	default:
		return false
	}
	modelKey := modelRateLimitKeyForUpstreamModelNotFound(ctx, account, requestedModel)
	if modelKey == "" {
		return false
	}
	if shouldSkipCodexPlanGatedImageModelCooldown(ctx, reason, requestedModel, modelKey) {
		return true
	}
	resetAt := time.Now().Add(cooldown)
	if err := s.accountRepo.SetModelRateLimit(ctx, account.ID, modelKey, resetAt); err != nil {
		slog.Warn("upstream_model_not_found_set_model_rate_limit_failed", "account_id", account.ID, "model", modelKey, "reason", reason, "error", err)
		return true
	}
	slog.Info("upstream_model_not_found_model_rate_limited", "account_id", account.ID, "model", modelKey, "reason", reason, "reset_at", resetAt)
	return true
}

func shouldSkipCodexPlanGatedImageModelCooldown(ctx context.Context, reason, requestedModel, modelKey string) bool {
	if reason != upstreamCodexPlanGatedModelReason {
		return false
	}
	if OpenAIImagesEndpointFromContext(ctx) {
		return false
	}
	return isOpenAIImageGenerationModel(requestedModel) || isOpenAIImageGenerationModel(modelKey)
}

func modelRateLimitKeyForUpstreamModelNotFound(ctx context.Context, account *Account, requestedModel string) string {
	modelKey := strings.TrimSpace(requestedModel)
	if account == nil || modelKey == "" {
		return modelKey
	}
	if account.Platform == PlatformAntigravity {
		if resolved := strings.TrimSpace(resolveFinalAntigravityModelKey(ctx, account, modelKey)); resolved != "" {
			return resolved
		}
		return modelKey
	}
	if mapped := strings.TrimSpace(account.GetMappedModel(modelKey)); mapped != "" {
		return mapped
	}
	return modelKey
}
