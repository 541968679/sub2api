package service

import "strings"

// openAIBillingModelForAccountMapping is the model name sent to pricing.
// Account credentials.model_mapping still selects the upstream model, but the
// bill uses the name from before that mapping. Group default mapping and
// platform-default mapping keep the forward model, including a platform
// default whose billing object is "mapped".
func openAIBillingModelForAccountMapping(account *Account, requestedModel, forwardModel string) string {
	requested := strings.TrimSpace(requestedModel)
	forward := strings.TrimSpace(forwardModel)
	if account != nil && requested != "" &&
		account.ResolveMappedModelDetailed(requested).Source == ModelMappingSourceAccount {
		return requested
	}
	if forward != "" {
		return forward
	}
	return requested
}

// resolveOpenAIForwardModel determines the upstream model for OpenAI-compatible
// forwarding. Group-level default mapping only applies when the account itself
// did not match any explicit model_mapping rule.
func resolveOpenAIForwardModel(account *Account, requestedModel, defaultMappedModel string) string {
	if account == nil {
		if defaultMappedModel != "" {
			return defaultMappedModel
		}
		return requestedModel
	}

	mappedModel, matched := account.ResolveMappedModel(requestedModel)
	if !matched && defaultMappedModel != "" && !isExplicitCodexModel(requestedModel) {
		return defaultMappedModel
	}
	return mappedModel
}

func isExplicitCodexModel(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	if strings.Contains(model, "/") {
		parts := strings.Split(model, "/")
		model = parts[len(parts)-1]
	}
	model = strings.ToLower(strings.TrimSpace(model))
	if codexModelFamilyRecognized(model) {
		return true
	}
	if strings.HasSuffix(model, "-openai-compact") {
		base := strings.TrimSuffix(model, "-openai-compact")
		return codexModelFamilyRecognized(base)
	}
	return false
}

// codexModelFamilyRecognized reports canonical IDs and their effort or dated
// suffixes. Group default mapping must not rewrite these to an older model.
func codexModelFamilyRecognized(model string) bool {
	if getNormalizedCodexModel(model) != "" {
		return true
	}
	key := codexModelLookupKey(model)
	for _, item := range codexVersionModelPrefixes {
		if key == item.prefix {
			return true
		}
		suffix, ok := strings.CutPrefix(key, item.prefix+"-")
		if ok && isKnownCodexModelSuffix(suffix) {
			return true
		}
	}
	return false
}

// resolveOpenAICompactForwardModel determines the compact-only upstream model
// for /responses/compact requests. It never affects normal /responses traffic.
// When no compact-specific mapping matches, the input model is returned as-is.
func resolveOpenAICompactForwardModel(account *Account, model string) string {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" || account == nil {
		return trimmedModel
	}

	mappedModel, matched := account.ResolveCompactMappedModel(trimmedModel)
	if !matched {
		return trimmedModel
	}
	if trimmedMapped := strings.TrimSpace(mappedModel); trimmedMapped != "" {
		return trimmedMapped
	}
	return trimmedModel
}

func resolveOpenAICompactFallbackForwardModels(account *Account, requestedModel, mappedModel string) []string {
	if account == nil {
		return nil
	}
	primaryUpstreamModel := normalizeOpenAIModelForUpstream(account, mappedModel)
	rawCandidates := account.ResolveCompactFallbackModels(requestedModel, mappedModel)
	if len(rawCandidates) == 0 {
		return nil
	}

	result := make([]string, 0, len(rawCandidates))
	seen := make(map[string]bool, len(rawCandidates)+1)
	if primaryUpstreamModel != "" {
		seen[strings.ToLower(primaryUpstreamModel)] = true
	}
	for _, candidate := range rawCandidates {
		upstreamModel := normalizeOpenAIModelForUpstream(account, strings.TrimSpace(candidate))
		if upstreamModel == "" {
			continue
		}
		key := strings.ToLower(upstreamModel)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, upstreamModel)
	}
	return result
}
