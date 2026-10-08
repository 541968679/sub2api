package service

import "strings"

// extraKeySyncInboundUpstreamSSE 账号 extra 开关。只有显式 true 才把同步入站改成上游 SSE。
// 缺省、false 都保持上游同步 JSON。没有全局配置覆盖。
const extraKeySyncInboundUpstreamSSE = "openai_sync_inbound_upstream_sse"

func extraBoolValue(extra map[string]any, key string) (bool, bool) {
	if extra == nil {
		return false, false
	}
	v, ok := extra[key]
	if !ok || v == nil {
		return false, false
	}
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	}
	return false, false
}

// shouldForceSyncInboundUpstreamSSE reports whether an inbound sync Chat
// Completions request should ask the upstream for SSE and buffer it locally.
// The account switch defaults off. OAuth, Grok, and inbound stream=true are never forced here.
func shouldForceSyncInboundUpstreamSSE(account *Account, clientStream bool) bool {
	if clientStream || account == nil {
		return false
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return false
	}
	enabled, ok := extraBoolValue(account.Extra, extraKeySyncInboundUpstreamSSE)
	return ok && enabled
}
