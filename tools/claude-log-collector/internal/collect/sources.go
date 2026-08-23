package collect

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/541968679/sub2api/tools/claude-log-collector/internal/pack"
	"github.com/541968679/sub2api/tools/claude-log-collector/internal/redact"
	"github.com/541968679/sub2api/tools/claude-log-collector/internal/scan"
)

type fileKind int

const (
	kindConfig fileKind = iota
	kindLog
	kindSession
)

type acc struct {
	id     string
	name   string
	root   string
	files  int
	bytes  int64
	reason []string
	exists bool
	perm   bool
	err    error
}

func (a *acc) source(window string) pack.Source {
	status := pack.StatusNotFound
	switch {
	case a.files > 0:
		status = pack.StatusFound
	case a.perm:
		status = pack.StatusNoPermission
	case a.err != nil:
		status = pack.StatusError
	case a.exists:
		status = pack.StatusFound
	}
	return pack.Source{
		ID:         a.id,
		Name:       a.name,
		Status:     status,
		StatusText: pack.StatusText(status),
		Root:       a.root,
		Files:      a.files,
		Bytes:      a.bytes,
		Reason:     strings.Join(a.reason, "；"),
		Window:     window,
	}
}

func meta(id string) (string, string) {
	for _, m := range scan.SourceCatalog() {
		if m.ID == id {
			return m.ID, m.Name
		}
	}
	return id, id
}

func (r *runner) logWindowLabel() string {
	if r.opts.AllLogs {
		return "全部日志"
	}
	return "最近 7 天"
}

func (r *runner) collectClaudeCodeConfig() pack.Source {
	id, name := meta(scan.SourceClaudeCodeConfig)
	r.progress("正在采集 Claude Code 配置…")
	a := &acc{id: id, name: name, root: r.disc.ClaudeConfigDir}
	root := r.disc.ClaudeConfigDir
	if !dirExists(root) {
		a.reason = append(a.reason, "配置目录不存在")
		return a.source("配置摘要（不受 7 天限制）")
	}
	a.exists = true
	for _, name := range []string{"settings.json", "settings.local.json"} {
		r.addFile(a, filepath.Join(root, name), scan.SourceClaudeCodeConfig+"/"+name, kindConfig)
	}
	settingsDir := filepath.Join(root, "settings")
	if dirExists(settingsDir) {
		_ = filepath.WalkDir(settingsDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if strings.EqualFold(filepath.Ext(d.Name()), ".json") {
				rel := safeRel(scan.SourceClaudeCodeConfig, "settings", d.Name())
				r.addFile(a, path, rel, kindConfig)
			}
			return nil
		})
	}
	if fileExists(filepath.Join(root, ".credentials.json")) {
		a.reason = append(a.reason, "已排除 .credentials.json 原件")
	}
	return a.source("配置摘要（不受 7 天限制）")
}

func (r *runner) collectClaudeJSON() pack.Source {
	id, name := meta(scan.SourceClaudeJSON)
	a := &acc{id: id, name: name, root: r.disc.ClaudeJSONPath}
	if !fileExists(r.disc.ClaudeJSONPath) {
		a.reason = append(a.reason, "文件不存在")
		return a.source("配置摘要（不受 7 天限制）")
	}
	a.exists = true
	r.addFile(a, r.disc.ClaudeJSONPath, scan.SourceClaudeJSON+"/claude.json", kindConfig)
	return a.source("配置摘要（不受 7 天限制）")
}

func (r *runner) collectDebug() pack.Source {
	id, name := meta(scan.SourceClaudeCodeDebug)
	r.progress("正在采集调试日志…")
	a := &acc{id: id, name: name}
	var roots []string
	for _, p := range r.disc.DebugLogTargets {
		if fileExists(p) {
			a.exists = true
			roots = append(roots, p)
			r.addFile(a, p, safeRel(id, filepath.Base(p)), kindLog)
			continue
		}
		if dirExists(p) {
			a.exists = true
			roots = append(roots, p)
			r.walkLogs(a, p, id, false)
		}
	}
	a.root = strings.Join(roots, "; ")
	if !a.exists {
		a.reason = append(a.reason, "未找到 debug 目录或 CLAUDE_CODE_DEBUG_LOGS_DIR（客户可能未开 --debug）")
	}
	return a.source(r.logWindowLabel())
}

func (r *runner) collectCLINode() pack.Source {
	id, name := meta(scan.SourceClaudeCLINodejs)
	a := &acc{id: id, name: name, root: r.disc.CLINodejsDir}
	if r.disc.CLINodejsDir == "" || !dirExists(r.disc.CLINodejsDir) {
		a.reason = append(a.reason, "目录不存在")
		return a.source(r.logWindowLabel())
	}
	a.exists = true
	r.walkLogs(a, r.disc.CLINodejsDir, id, true)
	return a.source(r.logWindowLabel())
}

func (r *runner) collectDesktop() pack.Source {
	id, name := meta(scan.SourceClaudeDesktop)
	r.progress("正在采集 Claude Desktop…")
	a := &acc{id: id, name: name}
	var roots []string
	for _, root := range r.disc.DesktopRoots {
		if !dirExists(root) {
			continue
		}
		a.exists = true
		roots = append(roots, root)
		base := filepath.Base(root)
		logs := filepath.Join(root, "Logs")
		if dirExists(logs) {
			r.walkLogs(a, logs, safeRel(id, base, "Logs"), false)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			r.noteErr(a, err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if isConfigName(n) {
				r.addFile(a, filepath.Join(root, n), safeRel(id, base, n), kindConfig)
			}
		}
	}
	a.root = strings.Join(roots, "; ")
	if !a.exists {
		a.reason = append(a.reason, "未安装或尚未运行过 Claude Desktop")
	}
	return a.source(r.logWindowLabel() + "；配置摘要始终采集")
}

func (r *runner) collectObsidianApp() pack.Source {
	id, name := meta(scan.SourceObsidianApp)
	a := &acc{id: id, name: name, root: r.disc.ObsidianAppDir}
	if r.disc.ObsidianAppDir == "" || !dirExists(r.disc.ObsidianAppDir) {
		a.reason = append(a.reason, "未打开过 Obsidian 或应用目录不存在")
		return a.source(r.logWindowLabel())
	}
	a.exists = true
	entries, err := os.ReadDir(r.disc.ObsidianAppDir)
	if err != nil {
		r.noteErr(a, err)
		return a.source(r.logWindowLabel())
	}
	for _, e := range entries {
		p := filepath.Join(r.disc.ObsidianAppDir, e.Name())
		if e.IsDir() {
			if strings.EqualFold(e.Name(), "logs") || strings.EqualFold(e.Name(), "log") {
				r.walkLogs(a, p, safeRel(id, e.Name()), false)
			}
			continue
		}
		if isLogName(e.Name()) {
			r.addFile(a, p, safeRel(id, e.Name()), kindLog)
		}
	}
	if fileExists(r.disc.ObsidianJSON) {
		if data, err := os.ReadFile(r.disc.ObsidianJSON); err == nil {
			list := scan.ParseObsidianJSON(data)
			raw, _ := json.MarshalIndent(map[string]any{"vaults": list}, "", "  ")
			r.writeStage(a, safeRel(id, "obsidian-vaults.json"), raw)
		}
	}
	return a.source(r.logWindowLabel() + "；库列表不受 7 天限制")
}

func (r *runner) collectObsidianVaults() pack.Source {
	id, name := meta(scan.SourceObsidianVaults)
	r.progress("正在采集 Obsidian Claude 插件…")
	a := &acc{id: id, name: name}
	if len(r.disc.Vaults) == 0 {
		a.reason = append(a.reason, "没有已登记或手动选择的库")
		return a.source("仅 .obsidian/plugins 白名单")
	}
	used := map[string]int{}
	var roots []string
	for _, v := range r.disc.Vaults {
		roots = append(roots, v.Path+" ("+v.Source+")")
		if !dirExists(v.Path) {
			a.reason = append(a.reason, filepath.Base(v.Path)+" 路径不存在")
			continue
		}
		a.exists = true
		plugins := filepath.Join(v.Path, ".obsidian", "plugins")
		if !dirExists(plugins) {
			a.reason = append(a.reason, filepath.Base(v.Path)+" 无 .obsidian/plugins")
			continue
		}
		label := uniqueBase(used, v.Path)
		entries, err := os.ReadDir(plugins)
		if err != nil {
			r.noteErr(a, err)
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.EqualFold(e.Name(), "cache") {
				continue
			}
			dir := filepath.Join(plugins, e.Name())
			dataJSON, _ := os.ReadFile(filepath.Join(dir, "data.json"))
			if !scan.PluginEligible(e.Name(), dataJSON) {
				continue
			}
			destPrefix := safeRel(id, label, e.Name())
			r.collectPluginDir(a, dir, destPrefix)
		}
	}
	a.root = strings.Join(roots, "; ")
	if !a.exists {
		a.reason = append(a.reason, "库路径均不存在")
	}
	return a.source("仅 Claude/Anthropic 相关插件；配置摘要始终采集")
}

func (r *runner) collectPluginDir(a *acc, dir, destPrefix string) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirName(d.Name(), false) && path != dir {
				return fs.SkipDir
			}
			return nil
		}
		n := d.Name()
		switch {
		case isConfigName(n) || strings.EqualFold(filepath.Ext(n), ".json"):
			r.addFile(a, path, safeRel(destPrefix, n), kindConfig)
		case isLogName(n):
			r.addFile(a, path, safeRel(destPrefix, n), kindLog)
		}
		return nil
	})
}

func (r *runner) collectBounded() pack.Source {
	id, name := meta(scan.SourceAppDataBounded)
	a := &acc{id: id, name: name}
	if len(r.disc.BoundedDirs) == 0 {
		a.reason = append(a.reason, "顶层无额外 *Claude* / *Anthropic* 目录")
		return a.source(r.logWindowLabel())
	}
	a.exists = true
	a.root = strings.Join(r.disc.BoundedDirs, "; ")
	used := map[string]int{}
	for _, root := range r.disc.BoundedDirs {
		base := uniqueBase(used, root)
		logs := filepath.Join(root, "Logs")
		if dirExists(logs) {
			r.walkLogs(a, logs, safeRel(id, base, "Logs"), false)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			r.noteErr(a, err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if isLogName(n) {
				r.addFile(a, filepath.Join(root, n), safeRel(id, base, n), kindLog)
			} else if isConfigName(n) {
				r.addFile(a, filepath.Join(root, n), safeRel(id, base, n), kindConfig)
			}
		}
	}
	return a.source(r.logWindowLabel())
}

func (r *runner) collectSessions() pack.Source {
	id, name := meta(scan.SourceSessions)
	a := &acc{id: id, name: name, root: r.disc.ClaudeConfigDir}
	if !r.opts.IncludeSessions {
		s := a.source("未勾选")
		s.Status = pack.StatusSkipped
		s.StatusText = pack.StatusText(pack.StatusSkipped)
		s.Reason = "未勾选附带最近 24 小时会话"
		return s
	}
	r.progress("正在采集最近 24 小时会话（打码）…")
	root := r.disc.ClaudeConfigDir
	if !dirExists(root) {
		a.reason = append(a.reason, "配置目录不存在")
		return a.source("最近 24 小时")
	}
	a.exists = true
	hist := filepath.Join(root, "history.jsonl")
	if fileExists(hist) {
		r.addFile(a, hist, "sessions/history.jsonl", kindSession)
	}
	trans := filepath.Join(root, "transcripts")
	if dirExists(trans) {
		_ = filepath.WalkDir(trans, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(trans, path)
			r.addFile(a, path, safeRel("sessions", "transcripts", rel), kindSession)
			return nil
		})
	}
	projects := filepath.Join(root, "projects")
	if dirExists(projects) {
		_ = filepath.WalkDir(projects, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !strings.EqualFold(filepath.Ext(d.Name()), ".jsonl") {
				return nil
			}
			rel, _ := filepath.Rel(projects, path)
			r.addFile(a, path, safeRel("sessions", "projects", rel), kindSession)
			return nil
		})
	}
	if a.files == 0 {
		a.reason = append(a.reason, "最近 24 小时无会话记录")
	} else {
		a.reason = append(a.reason, "已包含打码会话")
	}
	return a.source("最近 24 小时（LastWriteTime）")
}

func (r *runner) walkLogs(a *acc, root, destPrefix string, allowCache bool) {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			r.noteErr(a, err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirName(d.Name(), allowCache) {
				return fs.SkipDir
			}
			return nil
		}
		if !isLogName(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = d.Name()
		}
		r.addFile(a, path, safeRel(destPrefix, rel), kindLog)
		return nil
	})
	if err != nil {
		r.noteErr(a, err)
	}
}

func (r *runner) addFile(a *acc, src, destRel string, kind fileKind) {
	if a.files >= maxFilesPerSource {
		const capMsg = "已达单来源文件上限"
		if !strings.Contains(strings.Join(a.reason, "；"), capMsg) {
			a.reason = append(a.reason, capMsg)
		}
		return
	}
	if redact.ShouldExclude(src) {
		return
	}
	if kind != kindSession && (sessionLikePath(src) || sessionLikePath(destRel)) {
		return
	}
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		r.noteErr(a, err)
		return
	}
	if info.IsDir() {
		return
	}
	if info.Size() > MaxFileSize {
		a.reason = append(a.reason, filepath.Base(src)+" 超过 32MB 已跳过")
		return
	}
	switch kind {
	case kindLog:
		if !r.opts.AllLogs && info.ModTime().Before(r.now.Add(-DefaultLogWindow)) {
			return
		}
	case kindSession:
		if info.ModTime().Before(r.now.Add(-DefaultSessionWindow)) {
			return
		}
	}
	data, err := os.ReadFile(src)
	if err != nil {
		r.noteErr(a, err)
		return
	}
	if redact.LooksBinary(data) {
		return
	}
	data = redact.RedactBytes(filepath.Base(src), data)
	r.writeStage(a, destRel, data)
}

func (r *runner) writeStage(a *acc, destRel string, data []byte) {
	destRel = filepath.FromSlash(safeRel(destRel))
	dest := filepath.Join(r.stage, destRel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		r.noteErr(a, err)
		return
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		r.noteErr(a, err)
		return
	}
	a.files++
	a.bytes += int64(len(data))
}

func (r *runner) noteErr(a *acc, err error) {
	if err == nil {
		return
	}
	a.err = err
	if os.IsPermission(err) {
		a.perm = true
		a.reason = append(a.reason, "无权限")
		return
	}
	a.reason = append(a.reason, err.Error())
}

// sessionLikePath is the AC7 filename/path rule: history.jsonl, transcripts/,
// and projects/**/*.jsonl stay out of the zip unless kindSession.
func sessionLikePath(p string) bool {
	if strings.TrimSpace(p) == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(p))
	if base == "history.jsonl" {
		return true
	}
	extJSONL := strings.EqualFold(filepath.Ext(base), ".jsonl")
	for _, part := range strings.FieldsFunc(filepath.Clean(p), func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		switch strings.ToLower(part) {
		case "transcripts":
			return true
		case "projects":
			if extJSONL {
				return true
			}
		}
	}
	return false
}

func isSettingsName(base string) bool {
	l := strings.ToLower(base)
	return l == "settings.json" || l == "settings.local.json"
}

func isConfigName(base string) bool {
	l := strings.ToLower(base)
	if isSettingsName(l) || l == "data.json" || l == "claude_desktop_config.json" || l == ".claude.json" {
		return true
	}
	return strings.Contains(l, "config") && strings.HasSuffix(l, ".json")
}

func isLogName(base string) bool {
	l := strings.ToLower(base)
	if strings.HasSuffix(l, ".log") || strings.HasSuffix(l, ".txt") {
		return true
	}
	if strings.HasSuffix(l, ".jsonl") {
		return true
	}
	if strings.Contains(l, "catalog") {
		return false
	}
	return strings.Contains(l, "-log") || strings.Contains(l, "_log") || strings.Contains(l, "logs")
}

func skipDirName(name string, allowCache bool) bool {
	l := strings.ToLower(name)
	switch l {
	case "node_modules", "gpucache", "code cache", "local storage", "session storage", "indexeddb", ".git",
		"transcripts", "projects":
		return true
	case "cache":
		return !allowCache
	}
	return false
}

func safeRel(parts ...string) string {
	var segs []string
	for _, p := range parts {
		p = strings.ReplaceAll(p, "\\", "/")
		for _, seg := range strings.Split(p, "/") {
			seg = strings.TrimSpace(seg)
			if seg == "" || seg == "." || seg == ".." {
				continue
			}
			segs = append(segs, seg)
		}
	}
	return strings.Join(segs, "/")
}

func uniqueBase(used map[string]int, path string) string {
	base := filepath.Base(path)
	if base == "." || base == "" || base == string(filepath.Separator) {
		base = "vault"
	}
	key := strings.ToLower(base)
	n := used[key]
	used[key] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s_%d", base, n+1)
}
