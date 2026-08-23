package scan

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	SourceClaudeCodeConfig = "claude-code-config"
	SourceClaudeJSON       = "claude-json"
	SourceClaudeCodeDebug  = "claude-code-debug"
	SourceClaudeCLINodejs  = "claude-cli-nodejs"
	SourceClaudeDesktop    = "claude-desktop"
	SourceObsidianApp      = "obsidian-app"
	SourceObsidianVaults   = "obsidian-vaults"
	SourceAppDataBounded   = "appdata-bounded"
	SourceSessions         = "sessions"
)

// SourceMeta is the stable source registry entry.
type SourceMeta struct {
	ID   string
	Name string
}

// SourceCatalog is the display order for manifest / GUI / CLI.
func SourceCatalog() []SourceMeta {
	return []SourceMeta{
		{ID: SourceClaudeCodeConfig, Name: "Claude Code 配置"},
		{ID: SourceClaudeJSON, Name: ".claude.json"},
		{ID: SourceClaudeCodeDebug, Name: "Claude Code 调试日志"},
		{ID: SourceClaudeCLINodejs, Name: "Claude CLI Node 日志"},
		{ID: SourceClaudeDesktop, Name: "Claude Desktop"},
		{ID: SourceObsidianApp, Name: "Obsidian 应用"},
		{ID: SourceObsidianVaults, Name: "Obsidian 库插件"},
		{ID: SourceAppDataBounded, Name: "AppData 有界扫描"},
		{ID: SourceSessions, Name: "最近会话（打码）"},
	}
}

// Env is the injectable view of the customer machine (tests use fixture dirs).
type Env struct {
	Home         string
	AppData      string
	LocalAppData string
	ExtraVault   string
	Lookup       Lookup
}

// DefaultEnv reads the real Windows profile directories.
func DefaultEnv(extraVault string) Env {
	home, _ := os.UserHomeDir()
	return Env{
		Home:         home,
		AppData:      os.Getenv("APPDATA"),
		LocalAppData: os.Getenv("LOCALAPPDATA"),
		ExtraVault:   extraVault,
		Lookup:       LookupProcessUserMachine,
	}
}

// Discovery is the resolved whitelist of roots. Missing roots stay in the
// struct as paths; collect marks them not-found instead of failing.
type Discovery struct {
	ClaudeConfigDir string
	ClaudeJSONPath  string
	DebugLogTargets []string
	CLINodejsDir    string
	DesktopRoots    []string
	ObsidianAppDir  string
	ObsidianJSON    string
	Vaults          []Vault
	BoundedDirs     []string
}

// Discover resolves whitelist roots from Env. It does not walk the whole disk.
func Discover(env Env) Discovery {
	lookup := env.Lookup
	if lookup == nil {
		lookup = LookupProcessUserMachine
	}

	var d Discovery
	cfg := strings.TrimSpace(lookup("CLAUDE_CONFIG_DIR"))
	if cfg == "" && env.Home != "" {
		cfg = filepath.Join(env.Home, ".claude")
	}
	d.ClaudeConfigDir = cfg

	if env.Home != "" {
		d.ClaudeJSONPath = filepath.Join(env.Home, ".claude.json")
	}

	if cfg != "" {
		d.DebugLogTargets = append(d.DebugLogTargets, filepath.Join(cfg, "debug"))
	}
	if extra := strings.TrimSpace(lookup("CLAUDE_CODE_DEBUG_LOGS_DIR")); extra != "" {
		d.DebugLogTargets = appendUnique(d.DebugLogTargets, extra)
	}

	if env.LocalAppData != "" {
		d.CLINodejsDir = filepath.Join(env.LocalAppData, "claude-cli-nodejs")
		d.DesktopRoots = append(d.DesktopRoots,
			filepath.Join(env.LocalAppData, "Claude"),
			filepath.Join(env.LocalAppData, "AnthropicClaude"),
		)
	}
	if env.AppData != "" {
		d.DesktopRoots = append(d.DesktopRoots,
			filepath.Join(env.AppData, "Claude"),
			filepath.Join(env.AppData, "AnthropicClaude"),
		)
		d.ObsidianAppDir = filepath.Join(env.AppData, "Obsidian")
		d.ObsidianJSON = findObsidianJSON(d.ObsidianAppDir)
		d.Vaults = loadRegisteredVaults(d.ObsidianAppDir)
	}

	if v := strings.TrimSpace(env.ExtraVault); v != "" {
		if !vaultPathKnown(d.Vaults, v) {
			d.Vaults = append(d.Vaults, Vault{Path: v, Source: "manual"})
		}
	}

	covered := map[string]bool{}
	markCovered(covered, d.ClaudeConfigDir, d.CLINodejsDir, d.ObsidianAppDir)
	markCovered(covered, d.DesktopRoots...)
	d.BoundedDirs = boundedChildren(env.AppData, covered)
	d.BoundedDirs = append(d.BoundedDirs, boundedChildren(env.LocalAppData, covered)...)
	return d
}

func boundedChildren(parent string, covered map[string]bool) []string {
	if parent == "" {
		return nil
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || !NameMatchesClaudeOrAnthropic(e.Name()) {
			continue
		}
		p := filepath.Join(parent, e.Name())
		if covered[normKey(p)] {
			continue
		}
		covered[normKey(p)] = true
		out = append(out, p)
	}
	return out
}

func markCovered(m map[string]bool, paths ...string) {
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		m[normKey(p)] = true
	}
}

func vaultPathKnown(vaults []Vault, path string) bool {
	want := normKey(path)
	for _, v := range vaults {
		if normKey(v.Path) == want {
			return true
		}
	}
	return false
}

func appendUnique(in []string, extra string) []string {
	if extra == "" {
		return in
	}
	want := normKey(extra)
	for _, p := range in {
		if normKey(p) == want {
			return in
		}
	}
	return append(in, extra)
}

func normKey(p string) string {
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	return strings.ToLower(filepath.Clean(abs))
}
