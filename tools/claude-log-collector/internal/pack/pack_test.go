package pack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteManifestAndZip(t *testing.T) {
	stage := t.TempDir()
	out := filepath.Join(t.TempDir(), "out.zip")
	m := Manifest{
		CreatedAt: time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC),
		Options:   Options{LogWindow: "最近 7 天"},
		Sources: []Source{{
			ID: "claude-desktop", Name: "Claude Desktop",
			Status: StatusNotFound, StatusText: StatusText(StatusNotFound),
		}},
		Notes: []string{"本工具不上传网络。"},
	}
	if err := WriteManifest(stage, m); err != nil {
		t.Fatal(err)
	}
	if err := ZipDir(stage, out); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadZipFile(out, "MANIFEST.txt")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "未安装 / 未找到") || !strings.Contains(text, "不上传网络") {
		t.Fatalf("manifest text:\n%s", text)
	}
}

func TestStatusText(t *testing.T) {
	if StatusText(StatusNotFound) != "未安装 / 未找到" {
		t.Fatal(StatusText(StatusNotFound))
	}
	if err := os.MkdirAll(t.TempDir(), 0o755); err != nil {
		t.Fatal(err)
	}
}
