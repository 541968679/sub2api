package service

var gatewayModelDiscoveryIDsByPlatform = map[string][]string{
	PlatformOpenAI: {
		"gpt-5.5",
		"gpt-5.4",
		"gpt-5.4-mini",
	},
	PlatformAntigravity: {
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	},
}

// GatewayModelDiscoveryIDsForPlatform returns the curated public model IDs used
// by /v1/models-style model discovery. It is presentation-only and must not be
// used for scheduling, model access checks, mapping, billing, or usage.
func GatewayModelDiscoveryIDsForPlatform(platform string) ([]string, bool) {
	ids, ok := gatewayModelDiscoveryIDsByPlatform[platform]
	if !ok {
		return nil, false
	}
	out := make([]string, len(ids))
	copy(out, ids)
	return out, true
}
