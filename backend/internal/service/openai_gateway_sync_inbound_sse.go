package service

import (
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// extraKeySyncInboundUpstreamSSE 账号 extra 覆盖：true=该号强制上游 SSE，false=该号维持 S2。
const extraKeySyncInboundUpstreamSSE = "openai_sync_inbound_upstream_sse"

const (
	openAISyncInboundUpstreamSSEModeAuto = "auto"
	openAISyncInboundUpstreamSSEModeOff  = "off"
	openAISyncInboundUpstreamSSEModeAll  = "all"
)

func normalizeOpenAISyncInboundUpstreamSSEMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", openAISyncInboundUpstreamSSEModeAuto:
		return openAISyncInboundUpstreamSSEModeAuto
	case openAISyncInboundUpstreamSSEModeOff:
		return openAISyncInboundUpstreamSSEModeOff
	case openAISyncInboundUpstreamSSEModeAll:
		return openAISyncInboundUpstreamSSEModeAll
	default:
		return openAISyncInboundUpstreamSSEModeAuto
	}
}

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

func isOfficialOpenAIAPIHost(baseURL string) bool {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return true
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || strings.TrimSpace(u.Hostname()) == "" {
		return false
	}
	return strings.EqualFold(u.Hostname(), "api.openai.com")
}

func accountHasCustomOpenAIBaseURL(account *Account) bool {
	if account == nil {
		return false
	}
	raw := strings.TrimSpace(account.GetCredential("base_url"))
	if raw == "" {
		return false
	}
	return !isOfficialOpenAIAPIHost(raw)
}

// shouldForceSyncInboundUpstreamSSE reports whether an inbound sync Chat
// Completions request should ask the upstream for SSE and buffer it locally.
// OAuth, Grok, and inbound stream=true are never forced here.
func shouldForceSyncInboundUpstreamSSE(account *Account, cfg *config.Config, clientStream bool) bool {
	if clientStream || account == nil {
		return false
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return false
	}
	if v, ok := extraBoolValue(account.Extra, extraKeySyncInboundUpstreamSSE); ok {
		return v
	}
	mode := openAISyncInboundUpstreamSSEModeAuto
	if cfg != nil {
		mode = normalizeOpenAISyncInboundUpstreamSSEMode(cfg.Gateway.OpenAISyncInboundUpstreamSSEMode)
	}
	switch mode {
	case openAISyncInboundUpstreamSSEModeOff:
		return false
	case openAISyncInboundUpstreamSSEModeAll:
		return true
	default:
		return accountHasCustomOpenAIBaseURL(account)
	}
}
