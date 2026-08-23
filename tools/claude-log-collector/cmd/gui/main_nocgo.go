//go:build !cgo

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "图形界面需要 CGO（Windows 上安装 MinGW-w64 / gcc）后重新编译：")
	fmt.Fprintln(os.Stderr, `  go build -ldflags="-H windowsgui" -o bin/claude-log-collector-gui.exe ./cmd/gui`)
	fmt.Fprintln(os.Stderr, "当前可使用命令行入口：go build -o bin/claude-log-collector.exe ./cmd/collector")
	os.Exit(1)
}
