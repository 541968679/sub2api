package service

import "strings"

// kimiRequiresNativeChatCompletions is true when this request must be sent as
// POST /v1/chat/completions. kimi-k3 upstreams reject /v1/responses bodies.
// The check looks at the account platform and every model name in the chain
// (requested, billing, upstream) so a mapped id still stays on chat completions.
func kimiRequiresNativeChatCompletions(account *Account, models ...string) bool {
	if account != nil && account.Platform == PlatformKimi {
		return true
	}
	for _, model := range models {
		if isKimiNativeChatModel(model) {
			return true
		}
	}
	return false
}

func isKimiNativeChatModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	return m == "kimi" || strings.HasPrefix(m, "kimi-")
}
