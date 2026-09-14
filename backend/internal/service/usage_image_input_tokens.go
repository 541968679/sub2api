package service

import (
	"github.com/tidwall/gjson"
)

// ExtractImageInputTokens reads image/text split input token counts from OpenAI-style
// usage JSON without mutating cache_read or stored cost fields.
// Returns image tokens from input_tokens_details.image_tokens (or aliases).
func ExtractImageInputTokens(usageJSON []byte) int {
	if len(usageJSON) == 0 || !gjson.ValidBytes(usageJSON) {
		return 0
	}
	root := gjson.ParseBytes(usageJSON)
	// Accept either a full response blob or a usage object.
	usage := root.Get("usage")
	if !usage.Exists() {
		usage = root
	}
	details := usage.Get("input_tokens_details")
	if !details.Exists() {
		return 0
	}
	for _, key := range []string{"image_tokens", "image_input_tokens", "image"} {
		v := details.Get(key)
		if v.Exists() && v.Int() > 0 {
			return int(v.Int())
		}
	}
	for _, key := range []string{"modalities.image", "by_modality.image"} {
		v := details.Get(key)
		if v.Exists() && v.Int() > 0 {
			return int(v.Int())
		}
	}
	return 0
}

// ApplyImageInputTokensToUsageLog sets ImageInputTokens on the log when positive.
// Does not change InputTokens, CacheReadTokens, TotalCost, or ActualCost.
func ApplyImageInputTokensToUsageLog(log *UsageLog, usageJSON []byte) {
	if log == nil {
		return
	}
	n := ExtractImageInputTokens(usageJSON)
	if n > 0 {
		log.ImageInputTokens = n
	}
}
