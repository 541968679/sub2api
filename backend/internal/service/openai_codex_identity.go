package service

import (
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const codexUpstreamMinVersion = "0.144.0"

// codexOriginatorNormalization controls whether enforceCodexIdentityHeaders rewrites
// load-shed Codex identities to the CLI identity. Published from
// gateway.disable_codex_originator_normalization (inverted) at service construction.
// Default enabled: load-shed hits return server_is_overloaded and cool accounts.
var codexOriginatorNormalization = func() *atomic.Bool {
	v := &atomic.Bool{}
	v.Store(true)
	return v
}()

// SetCodexOriginatorNormalizationEnabled publishes the load-shed rewrite switch.
// enforceCodexIdentityHeaders is a pure shared choke point without config injection,
// so the service that holds config publishes a process-level snapshot.
func SetCodexOriginatorNormalizationEnabled(enabled bool) {
	codexOriginatorNormalization.Store(enabled)
}

// ensureCodexIdentityHeaders fills the identity headers required by ChatGPT's
// internal Codex endpoint. Existing User-Agent and version values are kept so
// enforceCodexIdentityHeaders can pair and validate the final identity.
func ensureCodexIdentityHeaders(headers http.Header) {
	if headers == nil {
		return
	}
	if strings.TrimSpace(headers.Get("user-agent")) == "" {
		headers.Set("user-agent", codexCLIUserAgent)
	}
	if strings.TrimSpace(headers.Get("originator")) == "" {
		headers.Set("originator", openai.CodexCLIOriginator)
	}
	if strings.TrimSpace(headers.Get("version")) == "" {
		headers.Set("version", codexCLIVersion)
	}
	headers.Set("OpenAI-Beta", "responses=experimental")
}

// enforceCodexIdentityHeaders pairs the final outbound User-Agent and
// originator after all other header rewriting has completed.
//
// After pairing, load-shed identities are rewritten to CLI when normalization is
// enabled: upstream capacity-sheds by originator and the gateway cools accounts
// on server_is_overloaded.
//
// Only applies when originator is already set; callers that must recover missing
// identity should call ensureCodexIdentityHeaders first.
func enforceCodexIdentityHeaders(headers http.Header) {
	if headers == nil || headers.Get("originator") == "" {
		return
	}
	originator, userAgent, ok := openai.PairCodexClientIdentity(headers.Get("user-agent"))
	if !ok {
		originator, userAgent = openai.CodexCLIOriginator, codexCLIUserAgent
	}
	if codexOriginatorNormalization.Load() {
		originator, userAgent, _ = openai.NormalizeCodexClientIdentityToCLI(originator, userAgent)
	}
	headers.Set("user-agent", userAgent)
	headers.Set("originator", originator)
	if version := strings.TrimSpace(headers.Get("version")); version != "" && CompareVersions(version, codexUpstreamMinVersion) < 0 {
		headers.Set("version", codexCLIVersion)
	}
}
