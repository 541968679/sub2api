package service

func computeOpenAIChargeableInputTokens(inputTokens, cacheReadTokens, cacheWriteTokens int) int {
	if inputTokens < 0 {
		inputTokens = 0
	}
	if cacheReadTokens < 0 {
		cacheReadTokens = 0
	}
	if cacheWriteTokens < 0 {
		cacheWriteTokens = 0
	}
	chargeableInputTokens := inputTokens - cacheReadTokens - cacheWriteTokens
	if chargeableInputTokens < 0 {
		return 0
	}
	return chargeableInputTokens
}
