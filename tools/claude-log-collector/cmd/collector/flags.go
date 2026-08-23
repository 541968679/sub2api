package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/541968679/sub2api/tools/claude-log-collector/internal/collect"
)

func parseArgs(args []string) (collect.Options, error) {
	fs := flag.NewFlagSet("claude-log-collector", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	out := fs.String("out", "", "输出目录（默认用户桌面）")
	sessions := fs.Bool("include-sessions", false, "附带最近 24 小时会话（打码）")
	allLogs := fs.Bool("all-logs", false, "包含全部已发现日志（不受 7 天限制）")
	vault := fs.String("vault", "", "额外 Obsidian 库路径（只采集 .obsidian 下 Claude 相关插件）")
	if err := fs.Parse(args); err != nil {
		return collect.Options{}, err
	}
	if fs.NArg() > 0 {
		return collect.Options{}, fmt.Errorf("未识别参数: %s", strings.Join(fs.Args(), " "))
	}
	return collect.Options{
		OutDir:          strings.TrimSpace(*out),
		IncludeSessions: *sessions,
		AllLogs:         *allLogs,
		ExtraVault:      strings.TrimSpace(*vault),
	}, nil
}
