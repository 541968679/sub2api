package main

import (
	"fmt"
	"os"

	"github.com/541968679/sub2api/tools/claude-log-collector/internal/collect"
	"github.com/541968679/sub2api/tools/claude-log-collector/internal/pack"
)

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	opts.OnProgress = func(msg string) {
		fmt.Fprintln(os.Stderr, msg)
	}
	res, err := collect.Run(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "采集失败:", err)
		os.Exit(1)
	}
	fmt.Println("zip:", res.ZipPath)
	fmt.Printf("%-22s %-16s %6s %10s  %s\n", "来源", "状态", "文件", "体积", "说明")
	for _, s := range res.Manifest.Sources {
		fmt.Printf("%-22s %-16s %6d %10s  %s\n", s.ID, s.StatusText, s.Files, pack.FormatBytes(s.Bytes), s.Reason)
	}
}
