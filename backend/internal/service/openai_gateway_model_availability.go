package service

import (
	"context"
	"strings"
)

// DiagnoseModelAvailabilityForPlatform reports whether the requested model
// is configured to be served by any account in the group for the given
// schedule platform. OpenAI-group keys requesting Grok text models must
// diagnose against the Grok pool (with OpenAI-group access eligibility).
//
// Safe to call on the error path: returns {true,true} on any internal
// failure or when the inputs preclude meaningful diagnosis (empty model,
// nil service), so callers stay on the 503 fallback branch.
func (s *OpenAIGatewayService) DiagnoseModelAvailabilityForPlatform(
	ctx context.Context,
	groupID *int64,
	requestedModel string,
	platform string,
) ModelAvailabilityDiagnosis {
	if s == nil {
		return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
	}

	schedulePlatform, requireGrokAccess := ResolveOpenAICompatibleSchedulePlatform(platform, requestedModel)
	accounts, err := s.listSchedulableAccounts(ctx, groupID, schedulePlatform)
	if err != nil {
		// Conservative fallback so the caller keeps returning 503; we do not
		// want a transient lookup failure to flip into 404 model_not_found.
		return ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}
	}

	diag := ModelAvailabilityDiagnosis{}
	for i := range accounts {
		diag.HasAccountsInPool = true
		// Mirrors the per-candidate filter used during account selection
		// (openai_account_scheduler.isAccountRequestCompatible): empty
		// model_mapping accepts everything; otherwise the explicit / wildcard
		// mapping must match. Grok access from OpenAI groups additionally
		// requires the per-account opt-in flag.
		if requireGrokAccess && !accounts[i].IsGrokOpenAIGroupAccessEnabled() {
			continue
		}
		if accounts[i].IsModelSupported(requestedModel) {
			diag.HasModelSupport = true
			return diag
		}
	}
	return diag
}
