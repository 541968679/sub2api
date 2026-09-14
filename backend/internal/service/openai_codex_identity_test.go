package service

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestEnsureCodexIdentityHeaders(t *testing.T) {
	t.Run("fills missing identity headers", func(t *testing.T) {
		h := make(http.Header)

		ensureCodexIdentityHeaders(h)
		enforceCodexIdentityHeaders(h)

		require.Equal(t, "codex_cli_rs", h.Get("originator"))
		require.Equal(t, codexCLIUserAgent, h.Get("user-agent"))
		require.Equal(t, codexCLIVersion, h.Get("version"))
		require.Equal(t, "responses=experimental", h.Get("OpenAI-Beta"))
	})

	t.Run("preserves non-shed official user agent and valid version", func(t *testing.T) {
		const vscodeUA = "codex_vscode/9.9.9 (Mac OS X 14.0; arm64) vscode (codex_vscode; 9.9.9)"
		h := make(http.Header)
		h.Set("user-agent", vscodeUA)
		h.Set("version", "9.9.9")
		h.Set("OpenAI-Beta", "assistants=v2")

		ensureCodexIdentityHeaders(h)
		enforceCodexIdentityHeaders(h)

		require.Equal(t, "codex_vscode", h.Get("originator"))
		require.Equal(t, vscodeUA, h.Get("user-agent"))
		require.Equal(t, "9.9.9", h.Get("version"))
		require.Equal(t, "responses=experimental", h.Get("OpenAI-Beta"))
	})

	t.Run("load-shed identity normalized while keeping version and terminal fingerprint", func(t *testing.T) {
		h := make(http.Header)
		h.Set("user-agent", "codex-tui/9.9.9 (Mac OS X 14.0; arm64) iTerm (codex-tui; 9.9.9)")
		h.Set("version", "9.9.9")

		ensureCodexIdentityHeaders(h)
		enforceCodexIdentityHeaders(h)

		require.Equal(t, "codex_cli_rs", h.Get("originator"))
		require.Equal(t, "codex_cli_rs/9.9.9 (Mac OS X 14.0; arm64) iTerm", h.Get("user-agent"))
		require.Equal(t, "9.9.9", h.Get("version"))
	})
}

func TestEnforceCodexIdentityHeaders(t *testing.T) {
	const tuiUA = "codex-tui/0.140.2 (Mac OS X 14.0; arm64) iTerm (codex-tui; 0.140.2)"
	// codex-tui is load-shed; rewrite to CLI while keeping version/OS/arch/terminal.
	const tuiNormalizedUA = "codex_cli_rs/0.140.2 (Mac OS X 14.0; arm64) iTerm"

	tests := []struct {
		name           string
		originator     string
		userAgent      string
		version        string
		wantOriginator string
		wantUA         string
		wantVersion    string
	}{
		{
			name:           "mismatched originator re-paired from UA then normalized",
			originator:     "codex_cli_rs",
			userAgent:      tuiUA,
			wantOriginator: "codex_cli_rs",
			wantUA:         tuiNormalizedUA,
		},
		{
			name:           "load-shed identity rewritten to CLI",
			originator:     "codex-tui",
			userAgent:      tuiUA,
			wantOriginator: "codex_cli_rs",
			wantUA:         tuiNormalizedUA,
		},
		{
			name:           "non-shed official identity preserved",
			originator:     "codex_vscode",
			userAgent:      "codex_vscode/1.2.3 (Ubuntu 22.4.0; x86_64) vscode (codex_vscode; 1.2.3)",
			wantOriginator: "codex_vscode",
			wantUA:         "codex_vscode/1.2.3 (Ubuntu 22.4.0; x86_64) vscode (codex_vscode; 1.2.3)",
		},
		{
			name:           "third-party UA falls back to default identity",
			originator:     "opencode",
			userAgent:      "luna/1.0.0",
			wantOriginator: "codex_cli_rs",
			wantUA:         codexCLIUserAgent,
		},
		{
			name:           "missing UA falls back to default identity",
			originator:     "codex_vscode",
			wantOriginator: "codex_cli_rs",
			wantUA:         codexCLIUserAgent,
		},
		{
			name:           "originator override UA leading rewritten from trailer then normalized",
			originator:     "cccc",
			userAgent:      "cccc/0.142.0 (Ubuntu 22.4.0; x86_64) screen (codex-tui; 0.142.0)",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex_cli_rs/0.142.0 (Ubuntu 22.4.0; x86_64) screen",
		},
		{
			name:           "version below floor is raised to built-in",
			originator:     "codex_cli_rs",
			userAgent:      "codex_cli_rs/0.125.0",
			version:        "0.125.0",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex_cli_rs/0.125.0",
			wantVersion:    codexCLIVersion,
		},
		{
			name:           "version at floor is preserved",
			originator:     "codex_cli_rs",
			userAgent:      "codex_cli_rs/0.145.0",
			version:        "0.145.0",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex_cli_rs/0.145.0",
			wantVersion:    "0.145.0",
		},
		{
			name:           "missing version is not injected",
			originator:     "codex_cli_rs",
			userAgent:      "codex_cli_rs/0.98.0",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex_cli_rs/0.98.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := make(http.Header)
			if tt.originator != "" {
				h.Set("originator", tt.originator)
			}
			if tt.userAgent != "" {
				h.Set("user-agent", tt.userAgent)
			}
			if tt.version != "" {
				h.Set("version", tt.version)
			}

			enforceCodexIdentityHeaders(h)

			require.Equal(t, tt.wantOriginator, h.Get("originator"))
			require.Equal(t, tt.wantUA, h.Get("user-agent"))
			if tt.wantVersion != "" {
				require.Equal(t, tt.wantVersion, h.Get("version"))
			}
		})
	}
}

// Zero-value Config (tests/tools without viper) must keep normalization enabled.
// Do not t.Parallel() switch tests: they mutate process-level state.
func TestCodexOriginatorNormalizationZeroValueConfigKeepsItEnabled(t *testing.T) {
	var cfg config.Config
	require.False(t, cfg.Gateway.DisableCodexOriginatorNormalization,
		"zero value must mean normalization ON; positive naming would silently disable protection")

	SetCodexOriginatorNormalizationEnabled(!cfg.Gateway.DisableCodexOriginatorNormalization)
	t.Cleanup(func() { SetCodexOriginatorNormalizationEnabled(true) })

	h := make(http.Header)
	h.Set("originator", "codex-tui")
	h.Set("user-agent", "codex-tui/0.140.2 (Mac OS X 14.0; arm64) iTerm (codex-tui; 0.140.2)")

	enforceCodexIdentityHeaders(h)

	require.Equal(t, "codex_cli_rs", h.Get("originator"))
}

func TestEnforceCodexIdentityHeaders_NormalizationDisabled(t *testing.T) {
	const tuiUA = "codex-tui/0.140.2 (Mac OS X 14.0; arm64) iTerm (codex-tui; 0.140.2)"

	SetCodexOriginatorNormalizationEnabled(false)
	t.Cleanup(func() { SetCodexOriginatorNormalizationEnabled(true) })

	h := make(http.Header)
	h.Set("originator", "codex-tui")
	h.Set("user-agent", tuiUA)

	enforceCodexIdentityHeaders(h)

	require.Equal(t, "codex-tui", h.Get("originator"))
	require.Equal(t, tuiUA, h.Get("user-agent"))
}

func TestEnforceCodexIdentityHeaders_NormalizationIsIdempotent(t *testing.T) {
	h := make(http.Header)
	h.Set("originator", "codex-tui")
	h.Set("user-agent", "codex-tui/0.140.2 (Mac OS X 14.0; arm64) iTerm (codex-tui; 0.140.2)")

	enforceCodexIdentityHeaders(h)
	first := h.Get("user-agent")
	enforceCodexIdentityHeaders(h)

	require.Equal(t, first, h.Get("user-agent"))
	require.Equal(t, "codex_cli_rs", h.Get("originator"))
}

func TestEnforceCodexIdentityHeaders_NoOriginatorIsNoop(t *testing.T) {
	h := make(http.Header)
	h.Set("user-agent", "codex-tui/0.140.2")

	enforceCodexIdentityHeaders(h)

	require.Empty(t, h.Get("originator"))
	require.Equal(t, "codex-tui/0.140.2", h.Get("user-agent"))
}
