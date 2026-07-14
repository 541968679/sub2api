package service

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const codexUpstreamMinVersion = "0.144.0"

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
		headers.Set("originator", "codex_cli_rs")
	}
	if strings.TrimSpace(headers.Get("version")) == "" {
		headers.Set("version", codexCLIVersion)
	}
	headers.Set("OpenAI-Beta", "responses=experimental")
}

// enforceCodexIdentityHeaders pairs the final outbound User-Agent and
// originator after all other header rewriting has completed.
func enforceCodexIdentityHeaders(headers http.Header) {
	if headers == nil || headers.Get("originator") == "" {
		return
	}
	originator, userAgent, ok := openai.PairCodexClientIdentity(headers.Get("user-agent"))
	if !ok {
		originator, userAgent = "codex_cli_rs", codexCLIUserAgent
	}
	headers.Set("user-agent", userAgent)
	headers.Set("originator", originator)
	if version := strings.TrimSpace(headers.Get("version")); version != "" && CompareVersions(version, codexUpstreamMinVersion) < 0 {
		headers.Set("version", codexCLIVersion)
	}
}
