package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsCodexLoadShedOriginator(t *testing.T) {
	require.True(t, IsCodexLoadShedOriginator("codex-tui"))
	require.True(t, IsCodexLoadShedOriginator("  CODEX-TUI  "))
	require.False(t, IsCodexLoadShedOriginator("codex_cli_rs"))
	require.False(t, IsCodexLoadShedOriginator("codex_vscode"))
	require.False(t, IsCodexLoadShedOriginator("Codex Desktop"))
	require.False(t, IsCodexLoadShedOriginator(""))
}

func TestNormalizeCodexClientIdentityToCLI(t *testing.T) {
	tests := []struct {
		name           string
		originator     string
		ua             string
		wantOriginator string
		wantUA         string
		wantChanged    bool
	}{
		{
			name:           "tui full UA rewrites leading name and strips client-info trailer",
			originator:     "codex-tui",
			ua:             "codex-tui/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color (codex-tui; 0.144.1)",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex_cli_rs/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color",
			wantChanged:    true,
		},
		{
			name:           "without client-info trailer only leading name is rewritten",
			originator:     "codex-tui",
			ua:             "codex-tui/0.144.1 (Mac OS X 14.0; arm64)",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex_cli_rs/0.144.1 (Mac OS X 14.0; arm64)",
			wantChanged:    true,
		},
		{
			name:           "OS parentheses must not be stripped as client-info",
			originator:     "codex-tui",
			ua:             "codex-tui/0.144.1 (Ubuntu 22.4.0; x86_64)",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex_cli_rs/0.144.1 (Ubuntu 22.4.0; x86_64)",
			wantChanged:    true,
		},
		{
			name:           "missing version segment only replaces originator",
			originator:     "codex-tui",
			ua:             "codex-tui",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex-tui",
			wantChanged:    true,
		},
		{
			name:           "healthy identity unchanged",
			originator:     "codex_cli_rs",
			ua:             "codex_cli_rs/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color",
			wantOriginator: "codex_cli_rs",
			wantUA:         "codex_cli_rs/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color",
			wantChanged:    false,
		},
		{
			name:           "other official identities unchanged",
			originator:     "codex_vscode",
			ua:             "codex_vscode/1.0.0 (Ubuntu 22.4.0; x86_64) vscode (codex_vscode; 1.0.0)",
			wantOriginator: "codex_vscode",
			wantUA:         "codex_vscode/1.0.0 (Ubuntu 22.4.0; x86_64) vscode (codex_vscode; 1.0.0)",
			wantChanged:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOriginator, gotUA, changed := NormalizeCodexClientIdentityToCLI(tt.originator, tt.ua)
			require.Equal(t, tt.wantOriginator, gotOriginator)
			require.Equal(t, tt.wantUA, gotUA)
			require.Equal(t, tt.wantChanged, changed)
		})
	}
}

// Normalized identity must still pass originator/UA pairing (#3901) and stay idempotent.
func TestNormalizeCodexClientIdentityToCLIStaysPaired(t *testing.T) {
	originator, ua, changed := NormalizeCodexClientIdentityToCLI(
		"codex-tui",
		"codex-tui/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color (codex-tui; 0.144.1)",
	)
	require.True(t, changed)

	pairedOriginator, pairedUA, ok := PairCodexClientIdentity(ua)
	require.True(t, ok)
	require.Equal(t, originator, pairedOriginator)
	require.Equal(t, ua, pairedUA)

	againOriginator, againUA, againChanged := NormalizeCodexClientIdentityToCLI(originator, ua)
	require.False(t, againChanged)
	require.Equal(t, originator, againOriginator)
	require.Equal(t, ua, againUA)
}
