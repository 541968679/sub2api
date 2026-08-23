package collect

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/541968679/sub2api/tools/claude-log-collector/internal/pack"
	"github.com/541968679/sub2api/tools/claude-log-collector/internal/scan"
)

const fixtureKey = "sk-ant-testfixture000000000000001"

type fixture struct {
	env   scan.Env
	out   string
	now   time.Time
	home  string
	app   string
	local string
}

func TestRun_AbsentClientsStillSucceed_AC3(t *testing.T) {
	fx := setupEmpty(t)
	res, err := Run(Options{OutDir: fx.out, Now: fx.now, Env: &fx.env})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.ZipPath); err != nil {
		t.Fatal(err)
	}
	byID := sourcesByID(res.Manifest)
	if byID[scan.SourceClaudeDesktop].Status != pack.StatusNotFound {
		t.Fatalf("desktop: %+v", byID[scan.SourceClaudeDesktop])
	}
	if byID[scan.SourceObsidianApp].Status != pack.StatusNotFound {
		t.Fatalf("obsidian: %+v", byID[scan.SourceObsidianApp])
	}
	if byID[scan.SourceSessions].Status != pack.StatusSkipped {
		t.Fatalf("sessions should be skipped: %+v", byID[scan.SourceSessions])
	}
	if _, err := pack.ReadZipFile(res.ZipPath, "manifest.json"); err != nil {
		t.Fatal(err)
	}
}

func TestRun_DefaultLeanZip_AC2_AC4_AC5_AC7_AC10(t *testing.T) {
	fx := setupFull(t)
	res, err := Run(Options{OutDir: fx.out, Now: fx.now, Env: &fx.env})
	if err != nil {
		t.Fatal(err)
	}
	names, files := readZip(t, res.ZipPath)
	if len(res.Manifest.Sources) != len(scan.SourceCatalog()) {
		t.Fatalf("sources=%d catalog=%d", len(res.Manifest.Sources), len(scan.SourceCatalog()))
	}
	byID := sourcesByID(res.Manifest)
	if byID[scan.SourceClaudeCodeConfig].Status != pack.StatusFound {
		t.Fatalf("config: %+v", byID[scan.SourceClaudeCodeConfig])
	}
	if byID[scan.SourceClaudeDesktop].Status != pack.StatusFound {
		t.Fatalf("desktop: %+v", byID[scan.SourceClaudeDesktop])
	}
	if byID[scan.SourceObsidianVaults].Status != pack.StatusFound {
		t.Fatalf("vaults: %+v", byID[scan.SourceObsidianVaults])
	}

	joined := strings.Join(names, "\n")
	for _, leakName := range []string{
		"history.jsonl", "transcripts/", "预存款.md", "退货.md",
		"acme-erp", "secret-business.go", ".credentials.json",
	} {
		if strings.Contains(joined, leakName) {
			t.Fatalf("default zip should not contain %s\n%s", leakName, joined)
		}
	}
	if zipHas(names, "projects/") && zipHas(names, ".jsonl") && !strings.Contains(joined, "sessions/") {
		t.Fatalf("session jsonl leaked into default zip:\n%s", joined)
	}
	for _, n := range names {
		if strings.HasSuffix(strings.ToLower(n), ".jsonl") && (strings.Contains(n, "transcripts") || strings.Contains(n, "projects/") || strings.Contains(n, "history.jsonl")) {
			t.Fatalf("session jsonl in default zip: %s", n)
		}
	}

	settings := findFile(files, "settings.json")
	if settings == "" {
		t.Fatal("settings.json missing from zip")
	}
	if strings.Contains(settings, fixtureKey) || strings.Contains(string(mustRead(t, res.ZipPath, "env-summary.json")), fixtureKey) {
		t.Fatal("plaintext API key in zip")
	}
	if !strings.Contains(settings, "gw.example.com") {
		t.Fatalf("should keep base url host: %s", settings)
	}

	if !zipHas(names, "recent.log") {
		t.Fatalf("recent debug log missing:\n%s", joined)
	}
	if zipHas(names, "old.log") || zipHas(names, "old-desktop.log") {
		t.Fatalf("old logs should stay out of default 7-day window:\n%s", joined)
	}
	if !zipHas(names, "claude-md") || zipHas(names, "calendar") {
		t.Fatalf("obsidian plugin filter failed:\n%s", joined)
	}
	if !zipHas(names, "obsidian-copilot") {
		t.Fatalf("copilot plugin with anthropic mention missing:\n%s", joined)
	}
}

func TestRun_IncludeSessions24h_AC8(t *testing.T) {
	fx := setupFull(t)
	res, err := Run(Options{OutDir: fx.out, Now: fx.now, Env: &fx.env, IncludeSessions: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Manifest.Options.IncludeSessions {
		t.Fatal("manifest should mark sessions")
	}
	byID := sourcesByID(res.Manifest)
	if byID[scan.SourceSessions].Status != pack.StatusFound || byID[scan.SourceSessions].Files == 0 {
		t.Fatalf("sessions: %+v", byID[scan.SourceSessions])
	}
	if !strings.Contains(byID[scan.SourceSessions].Reason, "已包含") {
		t.Fatalf("reason=%q", byID[scan.SourceSessions].Reason)
	}
	names, files := readZip(t, res.ZipPath)
	if !zipHas(names, "sessions/history.jsonl") || !zipHas(names, "sessions/transcripts/recent.jsonl") {
		t.Fatalf("recent sessions missing:\n%s", strings.Join(names, "\n"))
	}
	if zipHas(names, "old.jsonl") || zipHas(names, "old-session.jsonl") {
		t.Fatalf("old sessions should be excluded:\n%s", strings.Join(names, "\n"))
	}
	for _, body := range files {
		if strings.Contains(body, fixtureKey) {
			t.Fatal("session key leaked")
		}
	}
}

func TestRun_AllLogsIncludesOld_AC10(t *testing.T) {
	fx := setupFull(t)
	res, err := Run(Options{OutDir: fx.out, Now: fx.now, Env: &fx.env, AllLogs: true})
	if err != nil {
		t.Fatal(err)
	}
	names, _ := readZip(t, res.ZipPath)
	if !zipHas(names, "old.log") || !zipHas(names, "old-desktop.log") {
		t.Fatalf("all-logs should include old files:\n%s", strings.Join(names, "\n"))
	}
	if !zipHas(names, "settings.json") {
		t.Fatal("config summary must remain")
	}
}

func TestRun_HonorsClaudeConfigDir(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.Local)
	home := filepath.Join(root, "home")
	moved := filepath.Join(root, "moved-claude")
	out := filepath.Join(root, "out")
	mustMkdir(t, home, moved, out)
	write(t, filepath.Join(moved, "settings.json"), `{
		"env": {
			"ANTHROPIC_BASE_URL": "https://moved.example.com/v1",
			"ANTHROPIC_API_KEY": "`+fixtureKey+`"
		}
	}`)
	res, err := Run(Options{
		OutDir: out,
		Now:    now,
		Env: &scan.Env{
			Home:         home,
			AppData:      filepath.Join(root, "appdata"),
			LocalAppData: filepath.Join(root, "local"),
			Lookup:       scan.LookupFromMap(map[string]string{"CLAUDE_CONFIG_DIR": moved}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, files := readZip(t, res.ZipPath)
	settings := findFile(files, "settings.json")
	if settings == "" {
		t.Fatal("moved CLAUDE_CONFIG_DIR settings missing")
	}
	if strings.Contains(settings, fixtureKey) {
		t.Fatal("plaintext key in moved settings")
	}
	if !strings.Contains(settings, "moved.example.com") {
		t.Fatalf("expected moved host: %s", settings)
	}
	if !res.Manifest.Environment.ConfigFiles["settings.json"] {
		t.Fatal("env summary should see settings under CLAUDE_CONFIG_DIR")
	}
}

func TestRun_DefaultZipExcludesSessionArtifactsInDebug_AC7(t *testing.T) {
	fx := setupFull(t)
	debug := filepath.Join(fx.home, ".claude", "debug")
	mustMkdir(t, filepath.Join(debug, "transcripts"), filepath.Join(debug, "projects", "p"))
	write(t, filepath.Join(debug, "history.jsonl"), `{"msg":"debug-history"}`+"\n")
	write(t, filepath.Join(debug, "transcripts", "x.jsonl"), `{"msg":"debug-transcript"}`+"\n")
	write(t, filepath.Join(debug, "projects", "p", "s.jsonl"), `{"msg":"debug-project"}`+"\n")
	chtimes(t, fx.now, filepath.Join(debug, "history.jsonl"), filepath.Join(debug, "transcripts", "x.jsonl"), filepath.Join(debug, "projects", "p", "s.jsonl"))

	res, err := Run(Options{OutDir: fx.out, Now: fx.now, Env: &fx.env})
	if err != nil {
		t.Fatal(err)
	}
	names, _ := readZip(t, res.ZipPath)
	joined := strings.Join(names, "\n")
	for _, leak := range []string{"history.jsonl", "transcripts/", "projects/"} {
		if strings.Contains(joined, leak) {
			t.Fatalf("default zip leaked session artifact %s:\n%s", leak, joined)
		}
	}
	if !zipHas(names, "recent.log") {
		t.Fatalf("debug recent.log should still be packed:\n%s", joined)
	}
}

func TestRun_DebugLogsDirDoesNotSweepSessions_AC7(t *testing.T) {
	fx := setupFull(t)
	cfg := filepath.Join(fx.home, ".claude")
	fx.env.Lookup = scan.LookupFromMap(map[string]string{
		"CLAUDE_CODE_DEBUG_LOGS_DIR": cfg,
	})
	res, err := Run(Options{OutDir: fx.out, Now: fx.now, Env: &fx.env})
	if err != nil {
		t.Fatal(err)
	}
	names, _ := readZip(t, res.ZipPath)
	joined := strings.Join(names, "\n")
	for _, n := range names {
		l := strings.ToLower(n)
		if strings.HasSuffix(l, ".jsonl") && (strings.Contains(l, "transcripts") || strings.Contains(l, "projects/") || strings.Contains(l, "history.jsonl")) {
			t.Fatalf("debug-logs-dir sweep leaked session file %s\n%s", n, joined)
		}
	}
}

func TestSessionLikePath(t *testing.T) {
	if !sessionLikePath(`C:\Users\x\.claude\history.jsonl`) {
		t.Fatal("history")
	}
	if !sessionLikePath(`C:\Users\x\.claude\transcripts\a.jsonl`) {
		t.Fatal("transcripts")
	}
	if !sessionLikePath(`C:\Users\x\.claude\projects\C--erp\session.jsonl`) {
		t.Fatal("projects jsonl")
	}
	if sessionLikePath(`C:\Users\x\.claude\debug\recent.log`) {
		t.Fatal("debug log is not a session artifact")
	}
	if sessionLikePath(`C:\Users\x\claude-cli-nodejs\Cache\mcp-log.jsonl`) {
		t.Fatal("cli diagnostic jsonl is not a session artifact")
	}
}

func TestRun_ExtraVaultOnlyPlugins_AC5_AC9(t *testing.T) {
	fx := setupFull(t)
	res, err := Run(Options{OutDir: fx.out, Now: fx.now, Env: &fx.env})
	if err != nil {
		t.Fatal(err)
	}
	names, _ := readZip(t, res.ZipPath)
	if !zipHas(names, "anthropic-helper") {
		t.Fatalf("extra vault plugin missing:\n%s", strings.Join(names, "\n"))
	}
	if zipHas(names, "退货.md") || zipHas(names, "notes/") {
		t.Fatalf("extra vault notes leaked:\n%s", strings.Join(names, "\n"))
	}
}

func setupEmpty(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.Local)
	home := filepath.Join(root, "home")
	mustMkdir(t, home, filepath.Join(root, "out"))
	return fixture{
		env: scan.Env{
			Home:         home,
			AppData:      filepath.Join(root, "appdata"),
			LocalAppData: filepath.Join(root, "local"),
			Lookup:       scan.LookupFromMap(nil),
		},
		out:  filepath.Join(root, "out"),
		now:  now,
		home: home,
	}
}

func setupFull(t *testing.T) fixture {
	t.Helper()
	root := t.TempDir()
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.Local)
	home := filepath.Join(root, "home")
	app := filepath.Join(root, "appdata")
	local := filepath.Join(root, "local")
	out := filepath.Join(root, "out")
	cfg := filepath.Join(home, ".claude")
	vault := filepath.Join(app, "reg-vault")
	extra := filepath.Join(root, "extra-vault")

	mustMkdir(t,
		filepath.Join(cfg, "debug"),
		filepath.Join(cfg, "transcripts"),
		filepath.Join(cfg, "projects", "C--Users-acme-erp"),
		filepath.Join(home, "acme-erp", "src"),
		filepath.Join(local, "Claude", "Logs"),
		filepath.Join(local, "claude-cli-nodejs", "Cache"),
		filepath.Join(local, "RandomClaudeExt", "Logs"),
		filepath.Join(app, "Obsidian"),
		filepath.Join(vault, "Daily Notes"),
		filepath.Join(vault, ".obsidian", "plugins", "claude-md"),
		filepath.Join(vault, ".obsidian", "plugins", "obsidian-copilot"),
		filepath.Join(vault, ".obsidian", "plugins", "calendar"),
		filepath.Join(vault, ".obsidian", "plugins", "cache"),
		filepath.Join(extra, "notes"),
		filepath.Join(extra, ".obsidian", "plugins", "anthropic-helper"),
		out,
	)

	write(t, filepath.Join(cfg, "settings.json"), `{
		"env": {
			"ANTHROPIC_BASE_URL": "https://gw.example.com/v1",
			"ANTHROPIC_API_KEY": "`+fixtureKey+`"
		}
	}`)
	write(t, filepath.Join(cfg, ".credentials.json"), `{"claudeAiOauth":{"accessToken":"`+fixtureKey+`"}}`)
	write(t, filepath.Join(home, ".claude.json"), `{"apiKey":"`+fixtureKey+`"}`)
	write(t, filepath.Join(cfg, "history.jsonl"), `{"text":"hello `+fixtureKey+`"}`+"\n")
	write(t, filepath.Join(cfg, "transcripts", "recent.jsonl"), `{"msg":"`+fixtureKey+`"}`+"\n")
	write(t, filepath.Join(cfg, "transcripts", "old.jsonl"), `{"msg":"old"}`+"\n")
	write(t, filepath.Join(cfg, "projects", "C--Users-acme-erp", "session.jsonl"), `{"p":1}`+"\n")
	write(t, filepath.Join(cfg, "projects", "C--Users-acme-erp", "old-session.jsonl"), `{"p":0}`+"\n")
	write(t, filepath.Join(cfg, "debug", "recent.log"), "debug recent\n")
	write(t, filepath.Join(cfg, "debug", "old.log"), "debug old\n")
	write(t, filepath.Join(home, "acme-erp", "src", "secret-business.go"), "package main\n")
	write(t, filepath.Join(local, "Claude", "Logs", "chrome-native-host.log"), "desktop recent\n")
	write(t, filepath.Join(local, "Claude", "Logs", "old-desktop.log"), "desktop old\n")
	write(t, filepath.Join(local, "Claude", "claude_desktop_config.json"), `{"mcpServers":{}}`)
	write(t, filepath.Join(local, "claude-cli-nodejs", "Cache", "mcp.log"), "mcp ok\n")
	write(t, filepath.Join(local, "claude-cli-nodejs", "Cache", "huge.blob"), "blob\x00data")
	write(t, filepath.Join(local, "RandomClaudeExt", "Logs", "ext.log"), "bounded\n")
	write(t, filepath.Join(vault, "Daily Notes", "预存款.md"), "# 预存款客户资料\n")
	write(t, filepath.Join(vault, ".obsidian", "plugins", "claude-md", "data.json"), `{"enabled":true}`)
	write(t, filepath.Join(vault, ".obsidian", "plugins", "obsidian-copilot", "data.json"), `{"provider":"anthropic"}`)
	write(t, filepath.Join(vault, ".obsidian", "plugins", "calendar", "data.json"), `{"week":true}`)
	write(t, filepath.Join(vault, ".obsidian", "plugins", "cache", "market.json"), `{"x":1}`)
	write(t, filepath.Join(extra, "notes", "退货.md"), "# 退货单\n")
	write(t, filepath.Join(extra, ".obsidian", "plugins", "anthropic-helper", "data.json"), `{"ok":true}`)

	obs, err := json.Marshal(map[string]any{
		"vaults": map[string]any{"id1": map[string]string{"path": vault}},
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(app, "Obsidian", "obsidian.json"), string(obs))
	write(t, filepath.Join(app, "Obsidian", "obsidian.log"), "obsidian app log\n")

	chtimes(t, now, filepath.Join(cfg, "debug", "recent.log"), filepath.Join(local, "Claude", "Logs", "chrome-native-host.log"), filepath.Join(cfg, "history.jsonl"), filepath.Join(cfg, "transcripts", "recent.jsonl"), filepath.Join(cfg, "projects", "C--Users-acme-erp", "session.jsonl"), filepath.Join(local, "claude-cli-nodejs", "Cache", "mcp.log"), filepath.Join(local, "RandomClaudeExt", "Logs", "ext.log"), filepath.Join(app, "Obsidian", "obsidian.log"))
	old := now.Add(-8 * 24 * time.Hour)
	chtimes(t, old, filepath.Join(cfg, "debug", "old.log"), filepath.Join(local, "Claude", "Logs", "old-desktop.log"))
	chtimes(t, now.Add(-48*time.Hour), filepath.Join(cfg, "transcripts", "old.jsonl"), filepath.Join(cfg, "projects", "C--Users-acme-erp", "old-session.jsonl"))

	return fixture{
		env: scan.Env{
			Home:         home,
			AppData:      app,
			LocalAppData: local,
			ExtraVault:   extra,
			Lookup:       scan.LookupFromMap(nil),
		},
		out:   out,
		now:   now,
		home:  home,
		app:   app,
		local: local,
	}
}

func sourcesByID(m pack.Manifest) map[string]pack.Source {
	out := map[string]pack.Source{}
	for _, s := range m.Sources {
		out[s.ID] = s
	}
	return out
}

func readZip(t *testing.T, path string) (names []string, files map[string]string) {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	files = map[string]string{}
	for _, f := range r.File {
		names = append(names, f.Name)
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, rc); err != nil {
			rc.Close()
			t.Fatal(err)
		}
		rc.Close()
		files[f.Name] = buf.String()
	}
	return names, files
}

func zipHas(names []string, sub string) bool {
	for _, n := range names {
		if strings.Contains(n, sub) {
			return true
		}
	}
	return false
}

func findFile(files map[string]string, base string) string {
	for name, body := range files {
		if filepath.Base(name) == base {
			return body
		}
	}
	return ""
}

func mustRead(t *testing.T, zipPath, name string) []byte {
	t.Helper()
	b, err := pack.ReadZipFile(zipPath, name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustMkdir(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chtimes(t *testing.T, when time.Time, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
}
