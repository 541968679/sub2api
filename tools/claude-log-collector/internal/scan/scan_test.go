package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_UsesClaudeConfigDirAndIgnoresMissingClients(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	app := filepath.Join(root, "appdata")
	local := filepath.Join(root, "local")
	moved := filepath.Join(root, "moved-claude")
	mustMkdir(t, home, moved)

	d := Discover(Env{
		Home:         home,
		AppData:      app,
		LocalAppData: local,
		Lookup:       LookupFromMap(map[string]string{"CLAUDE_CONFIG_DIR": moved}),
	})

	if d.ClaudeConfigDir != moved {
		t.Fatalf("ClaudeConfigDir=%q want %q", d.ClaudeConfigDir, moved)
	}
	if d.ClaudeJSONPath != filepath.Join(home, ".claude.json") {
		t.Fatalf("ClaudeJSONPath=%q", d.ClaudeJSONPath)
	}
	if len(d.Vaults) != 0 {
		t.Fatalf("expected no vaults, got %#v", d.Vaults)
	}
	if dirExists(d.ObsidianAppDir) {
		t.Fatalf("obsidian app dir should be missing: %s", d.ObsidianAppDir)
	}
	if dirExists(filepath.Join(local, "Claude")) {
		t.Fatalf("desktop should be missing")
	}
}

func TestDiscover_DefaultClaudeDirWhenEnvEmpty(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	mustMkdir(t, home)

	d := Discover(Env{
		Home:   home,
		Lookup: LookupFromMap(nil),
	})
	want := filepath.Join(home, ".claude")
	if d.ClaudeConfigDir != want {
		t.Fatalf("ClaudeConfigDir=%q want %q", d.ClaudeConfigDir, want)
	}
}

func TestDiscover_PresentClientsAndBoundedScan(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	app := filepath.Join(root, "appdata")
	local := filepath.Join(root, "local")
	vault := filepath.Join(app, "MyVault")
	extra := filepath.Join(root, "extra-vault")
	mustMkdir(t,
		filepath.Join(home, ".claude"),
		filepath.Join(local, "Claude", "Logs"),
		filepath.Join(local, "claude-cli-nodejs", "Cache"),
		filepath.Join(local, "RandomClaudeExt", "Logs"),
		filepath.Join(app, "Obsidian"),
		filepath.Join(vault, ".obsidian", "plugins", "claude-md"),
		filepath.Join(extra, ".obsidian", "plugins", "anthropic-helper"),
	)
	obsidianJSON, err := json.Marshal(map[string]any{
		"vaults": map[string]any{"id1": map[string]string{"path": vault}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Obsidian", "obsidian.json"), obsidianJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	d := Discover(Env{
		Home:         home,
		AppData:      app,
		LocalAppData: local,
		ExtraVault:   extra,
		Lookup:       LookupFromMap(nil),
	})

	if d.ClaudeConfigDir != filepath.Join(home, ".claude") {
		t.Fatalf("config dir: %s", d.ClaudeConfigDir)
	}
	if len(d.Vaults) != 2 {
		t.Fatalf("vaults=%d want 2: %#v", len(d.Vaults), d.Vaults)
	}
	if d.Vaults[0].Source != "obsidian.json" || d.Vaults[1].Source != "manual" {
		t.Fatalf("vault sources: %#v", d.Vaults)
	}
	if !containsPath(d.BoundedDirs, filepath.Join(local, "RandomClaudeExt")) {
		t.Fatalf("bounded should include RandomClaudeExt, got %v", d.BoundedDirs)
	}
	if containsPath(d.BoundedDirs, filepath.Join(local, "Claude")) {
		t.Fatalf("bounded should skip already-covered Claude Desktop: %v", d.BoundedDirs)
	}
	if containsPath(d.BoundedDirs, filepath.Join(local, "claude-cli-nodejs")) {
		t.Fatalf("bounded should skip claude-cli-nodejs: %v", d.BoundedDirs)
	}
}

func TestParseObsidianJSON_DedupesAndSkipsEmpty(t *testing.T) {
	raw := []byte(`{
		"vaults": {
			"a": {"path": "D:\\vaults\\one"},
			"b": {"path": "D:\\vaults\\one"},
			"c": {"path": "  "},
			"d": {"path": "D:\\vaults\\two"}
		}
	}`)
	got := ParseObsidianJSON(raw)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestPluginEligible(t *testing.T) {
	if !PluginEligible("claude-md", nil) {
		t.Fatal("claude-md")
	}
	if !PluginEligible("my-Anthropic-bridge", nil) {
		t.Fatal("anthropic name")
	}
	if PluginEligible("calendar", []byte(`{"foo":1}`)) {
		t.Fatal("unrelated plugin")
	}
	if PluginEligible("obsidian-copilot", []byte(`{"openai":true}`)) {
		t.Fatal("copilot without claude/anthropic")
	}
	if !PluginEligible("obsidian-copilot", []byte(`{"provider":"anthropic"}`)) {
		t.Fatal("copilot with anthropic")
	}
	if PluginEligible("cache", []byte(`{"claude":true}`)) {
		t.Fatal("cache dir")
	}
}

func TestNameMatchesClaudeOrAnthropic(t *testing.T) {
	if !NameMatchesClaudeOrAnthropic("Claude") || !NameMatchesClaudeOrAnthropic("anthropic-x") {
		t.Fatal("expected match")
	}
	if NameMatchesClaudeOrAnthropic("Obsidian") || NameMatchesClaudeOrAnthropic("Chrome") {
		t.Fatal("expected no match")
	}
}

func mustMkdir(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func containsPath(list []string, want string) bool {
	for _, p := range list {
		if normKey(p) == normKey(want) {
			return true
		}
	}
	return false
}
