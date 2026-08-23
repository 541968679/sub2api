package collect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/541968679/sub2api/tools/claude-log-collector/internal/pack"
	"github.com/541968679/sub2api/tools/claude-log-collector/internal/redact"
	"github.com/541968679/sub2api/tools/claude-log-collector/internal/scan"
)

const (
	DefaultLogWindow     = 7 * 24 * time.Hour
	DefaultSessionWindow = 24 * time.Hour
	MaxFileSize          = 32 << 20
	maxFilesPerSource    = 2000
)

// Options is shared by CLI and GUI. Both must call Run.
type Options struct {
	OutDir          string
	IncludeSessions bool
	AllLogs         bool
	ExtraVault      string
	Now             time.Time
	Env             *scan.Env
	OnProgress      func(string)
}

// Result is the customer-visible outcome of one collection.
type Result struct {
	ZipPath  string
	Manifest pack.Manifest
}

func DefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	for _, cand := range []string{
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "OneDrive", "Desktop"),
	} {
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand
		}
	}
	return home
}

func Run(opts Options) (Result, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		opts.OutDir = DefaultOutputDir()
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("创建输出目录失败: %w", err)
	}

	var env scan.Env
	if opts.Env == nil {
		env = scan.DefaultEnv(opts.ExtraVault)
	} else {
		env = *opts.Env
		if strings.TrimSpace(env.ExtraVault) == "" {
			env.ExtraVault = opts.ExtraVault
		}
	}

	progress := opts.OnProgress
	if progress == nil {
		progress = func(string) {}
	}
	progress("正在解析采集路径…")

	d := scan.Discover(env)
	stage, err := os.MkdirTemp("", "claude-log-collector-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(stage)

	r := &runner{
		opts:     opts,
		env:      env,
		disc:     d,
		stage:    stage,
		now:      opts.Now,
		lookup:   env.Lookup,
		progress: progress,
	}
	if r.lookup == nil {
		r.lookup = scan.LookupProcessUserMachine
	}

	envSum := r.buildEnvSummary()
	if raw, err := json.MarshalIndent(envSum, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(stage, "env-summary.json"), raw, 0o644)
	}

	sources := []pack.Source{
		r.collectClaudeCodeConfig(),
		r.collectClaudeJSON(),
		r.collectDebug(),
		r.collectCLINode(),
		r.collectDesktop(),
		r.collectObsidianApp(),
		r.collectObsidianVaults(),
		r.collectBounded(),
		r.collectSessions(),
	}

	m := pack.Manifest{
		CreatedAt:   opts.Now,
		Tool:        pack.ToolName,
		Options:     r.manifestOptions(),
		Environment: envSum,
		Sources:     sources,
		Notes: []string{
			"本工具不上传网络。",
			"凭证与 API key 已打码；.credentials.json 原件未打包。",
			"默认不含 history.jsonl、transcripts/ 或会话 jsonl 全文。",
			"未打包 Obsidian 笔记正文或客户业务工程目录。",
		},
	}
	if err := pack.WriteManifest(stage, m); err != nil {
		return Result{}, err
	}

	progress("正在打包 zip…")
	name := fmt.Sprintf("claude-client-logs-%s.zip", opts.Now.Format("20060102-150405"))
	zipPath := filepath.Join(opts.OutDir, name)
	if err := pack.ZipDir(stage, zipPath); err != nil {
		return Result{}, fmt.Errorf("写入 zip 失败: %w", err)
	}
	abs, err := filepath.Abs(zipPath)
	if err != nil {
		abs = zipPath
	}
	progress("采集完成")
	return Result{ZipPath: abs, Manifest: m}, nil
}

type runner struct {
	opts     Options
	env      scan.Env
	disc     scan.Discovery
	stage    string
	now      time.Time
	lookup   scan.Lookup
	progress func(string)
}

func (r *runner) manifestOptions() pack.Options {
	o := pack.Options{
		IncludeSessions: r.opts.IncludeSessions,
		AllLogs:         r.opts.AllLogs,
		ExtraVault:      strings.TrimSpace(r.opts.ExtraVault),
		LogWindow:       "最近 7 天（LastWriteTime）",
	}
	if r.opts.AllLogs {
		o.LogWindow = "全部已发现日志"
	}
	if r.opts.IncludeSessions {
		o.SessionWindow = "最近 24 小时（LastWriteTime）"
	}
	return o
}

func (r *runner) buildEnvSummary() pack.Environment {
	env := pack.Environment{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		ConfigFiles: map[string]bool{},
	}
	cfg := r.disc.ClaudeConfigDir
	env.ConfigFiles["claude_config_dir"] = dirExists(cfg)
	env.ConfigFiles["settings.json"] = fileExists(filepath.Join(cfg, "settings.json"))
	env.ConfigFiles["settings.local.json"] = fileExists(filepath.Join(cfg, "settings.local.json"))
	env.ConfigFiles[".claude.json"] = fileExists(r.disc.ClaudeJSONPath)
	env.ConfigFiles[".credentials.json"] = fileExists(filepath.Join(cfg, ".credentials.json"))
	env.ConfigFiles["obsidian.json"] = fileExists(r.disc.ObsidianJSON)
	desktopCfg := false
	for _, root := range r.disc.DesktopRoots {
		if fileExists(filepath.Join(root, "claude_desktop_config.json")) {
			desktopCfg = true
			break
		}
	}
	env.ConfigFiles["claude_desktop_config.json"] = desktopCfg
	if dirExists(cfg) {
		if entries, err := os.ReadDir(cfg); err == nil {
			for _, e := range entries {
				env.ClaudeTop = append(env.ClaudeTop, e.Name())
			}
		}
	}

	baseFromSettings, keyFromSettings, keyLenSettings := peekSettingsSecrets(filepath.Join(cfg, "settings.json"))
	if !keyFromSettings {
		_, keyFromSettings, keyLenSettings = peekSettingsSecrets(r.disc.ClaudeJSONPath)
	}

	env.Variables = append(env.Variables,
		r.varSummary("CLAUDE_CONFIG_DIR", false),
		r.varSummary("CLAUDE_CODE_DEBUG_LOGS_DIR", false),
		r.varSummary("ANTHROPIC_BASE_URL", true),
		r.secretVar("ANTHROPIC_API_KEY", keyFromSettings, keyLenSettings),
	)
	if host := redact.HostnameOnly(baseFromSettings); host != "" {
		for i := range env.Variables {
			if env.Variables[i].Name == "ANTHROPIC_BASE_URL" && env.Variables[i].Hostname == "" {
				env.Variables[i].Hostname = host
				env.Variables[i].Present = true
			}
		}
	}
	return env
}

func (r *runner) varSummary(name string, asURL bool) pack.Variable {
	v := pack.Variable{Name: name, Kind: "env"}
	raw := r.lookup(name)
	if raw == "" {
		return v
	}
	v.Present = true
	v.Length = len(raw)
	if asURL {
		v.Hostname = redact.HostnameOnly(raw)
	}
	return v
}

func (r *runner) secretVar(name string, extraPresent bool, extraLen int) pack.Variable {
	v := pack.Variable{Name: name, Kind: "secret", Redacted: true}
	raw := r.lookup(name)
	if raw != "" {
		v.Present = true
		v.Length = len(raw)
		return v
	}
	if extraPresent {
		v.Present = true
		v.Length = extraLen
	}
	return v
}

func peekSettingsSecrets(path string) (baseURL string, keyPresent bool, keyLen int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var root map[string]any
	if json.Unmarshal(data, &root) != nil {
		return
	}
	env, _ := root["env"].(map[string]any)
	if env == nil {
		return
	}
	if s, ok := env["ANTHROPIC_BASE_URL"].(string); ok {
		baseURL = s
	}
	if s, ok := env["ANTHROPIC_API_KEY"].(string); ok && s != "" {
		keyPresent = true
		keyLen = len(s)
	}
	return
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
