package pack

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ToolName = "claude-log-collector"

type Manifest struct {
	CreatedAt   time.Time   `json:"created_at"`
	Tool        string      `json:"tool"`
	Options     Options     `json:"options"`
	Environment Environment `json:"environment"`
	Sources     []Source    `json:"sources"`
	Notes       []string    `json:"notes"`
}

type Options struct {
	IncludeSessions bool   `json:"include_sessions"`
	AllLogs         bool   `json:"all_logs"`
	ExtraVault      string `json:"extra_vault,omitempty"`
	LogWindow       string `json:"log_window"`
	SessionWindow   string `json:"session_window,omitempty"`
}

type Environment struct {
	OS          string          `json:"os"`
	Arch        string          `json:"arch"`
	ConfigFiles map[string]bool `json:"config_files"`
	Variables   []Variable      `json:"variables"`
	ClaudeTop   []string        `json:"claude_config_top_level,omitempty"`
}

type Variable struct {
	Name     string `json:"name"`
	Present  bool   `json:"present"`
	Hostname string `json:"hostname,omitempty"`
	Length   int    `json:"length,omitempty"`
	Redacted bool   `json:"redacted,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type Source struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	StatusText string `json:"status_text"`
	Root       string `json:"root,omitempty"`
	Files      int    `json:"files"`
	Bytes      int64  `json:"bytes"`
	Reason     string `json:"reason,omitempty"`
	Window     string `json:"window,omitempty"`
}

const (
	StatusFound        = "found"
	StatusNotFound     = "not-found"
	StatusNoPermission = "no-permission"
	StatusSkipped      = "skipped"
	StatusError        = "error"
)

func StatusText(status string) string {
	switch status {
	case StatusFound:
		return "已找到"
	case StatusNotFound:
		return "未安装 / 未找到"
	case StatusNoPermission:
		return "无权限"
	case StatusSkipped:
		return "已跳过"
	case StatusError:
		return "失败"
	default:
		return status
	}
}

func FormatBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func WriteManifest(stageDir string, m Manifest) error {
	if m.Tool == "" {
		m.Tool = ToolName
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stageDir, "manifest.json"), raw, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stageDir, "MANIFEST.txt"), []byte(HumanText(m)), 0o644)
}

func HumanText(m Manifest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Claude 客户端诊断包\n")
	fmt.Fprintf(&b, "生成时间: %s\n", m.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(&b, "选项: 附带会话=%v  全部日志=%v\n", m.Options.IncludeSessions, m.Options.AllLogs)
	if m.Options.ExtraVault != "" {
		fmt.Fprintf(&b, "额外 Obsidian 库: %s\n", m.Options.ExtraVault)
	}
	fmt.Fprintf(&b, "日志时间窗: %s\n\n", m.Options.LogWindow)
	fmt.Fprintf(&b, "本机环境\n")
	fmt.Fprintf(&b, "OS: %s/%s\n", m.Environment.OS, m.Environment.Arch)
	for name, ok := range m.Environment.ConfigFiles {
		fmt.Fprintf(&b, "配置文件 %s: %v\n", name, ok)
	}
	for _, v := range m.Environment.Variables {
		fmt.Fprintf(&b, "变量 %s: 存在=%v", v.Name, v.Present)
		if v.Hostname != "" {
			fmt.Fprintf(&b, " 主机名=%s", v.Hostname)
		}
		if v.Length > 0 {
			fmt.Fprintf(&b, " 长度=%d", v.Length)
		}
		if v.Redacted {
			fmt.Fprintf(&b, " 已打码")
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\n来源\n")
	for _, s := range m.Sources {
		fmt.Fprintf(&b, "- %s (%s): %s | %d 个文件 | %s", s.Name, s.ID, s.StatusText, s.Files, FormatBytes(s.Bytes))
		if s.Root != "" {
			fmt.Fprintf(&b, " | %s", s.Root)
		}
		if s.Reason != "" {
			fmt.Fprintf(&b, " | %s", s.Reason)
		}
		b.WriteByte('\n')
	}
	if len(m.Notes) > 0 {
		fmt.Fprintf(&b, "\n说明\n")
		for _, n := range m.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	return b.String()
}

func ZipDir(stageDir, zipPath string) error {
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = filepath.Walk(stageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stageDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("unsafe zip path %q", rel)
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, in)
		in.Close()
		return copyErr
	})
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	return err
}

func ReadZipFile(zipPath, name string) ([]byte, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			var buf bytes.Buffer
			_, err = io.Copy(&buf, rc)
			rc.Close()
			return buf.Bytes(), err
		}
	}
	return nil, fmt.Errorf("missing %s", name)
}
