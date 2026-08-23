//go:build cgo

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/541968679/sub2api/tools/claude-log-collector/internal/collect"
	"github.com/541968679/sub2api/tools/claude-log-collector/internal/pack"
)

type sourceRow struct {
	name, status, files, size, reason string
}

func main() {
	a := app.New()
	w := a.NewWindow("Claude 客户端日志采集")
	w.Resize(fyne.NewSize(860, 680))

	intro := widget.NewLabel("在本机采集 Claude / Desktop / Obsidian 诊断材料，打成 zip。不会上传网络，也不会打包笔记正文或明文 API key。")
	intro.Wrapping = fyne.TextWrapWord

	outEntry := widget.NewEntry()
	outEntry.SetText(collect.DefaultOutputDir())
	browseOut := widget.NewButton("浏览…", func() {
		dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil || u == nil {
				return
			}
			outEntry.SetText(uriPath(u))
		}, w)
	})

	sessionsCheck := widget.NewCheck("附带最近 24 小时会话（打码）", nil)
	allLogsCheck := widget.NewCheck("包含全部已发现日志", nil)

	vaultEntry := widget.NewEntry()
	vaultEntry.SetPlaceHolder("可选，只采集该库 .obsidian 下 Claude 相关插件")
	browseVault := widget.NewButton("浏览…", func() {
		dialog.ShowFolderOpen(func(u fyne.ListableURI, err error) {
			if err != nil || u == nil {
				return
			}
			vaultEntry.SetText(uriPath(u))
		}, w)
	})

	resultEntry := widget.NewEntry()
	resultEntry.SetPlaceHolder("采集完成后显示 zip 绝对路径")
	resultEntry.Disable()

	status := widget.NewLabel("就绪")
	progress := widget.NewProgressBarInfinite()
	progress.Hide()

	rows := []sourceRow{}
	table := widget.NewTable(
		func() (int, int) { return len(rows) + 1, 5 },
		func() fyne.CanvasObject {
			l := widget.NewLabel("template")
			l.Truncation = fyne.TextTruncateEllipsis
			return l
		},
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			heads := []string{"来源", "状态", "文件数", "体积", "说明"}
			if id.Row == 0 {
				l.TextStyle = fyne.TextStyle{Bold: true}
				l.SetText(heads[id.Col])
				return
			}
			l.TextStyle = fyne.TextStyle{}
			if id.Row-1 >= len(rows) {
				l.SetText("")
				return
			}
			r := rows[id.Row-1]
			switch id.Col {
			case 0:
				l.SetText(r.name)
			case 1:
				l.SetText(r.status)
			case 2:
				l.SetText(r.files)
			case 3:
				l.SetText(r.size)
			default:
				l.SetText(r.reason)
			}
		},
	)
	table.SetColumnWidth(0, 170)
	table.SetColumnWidth(1, 130)
	table.SetColumnWidth(2, 70)
	table.SetColumnWidth(3, 90)
	table.SetColumnWidth(4, 320)

	var start *widget.Button
	start = widget.NewButton("开始采集", func() {
		start.Disable()
		progress.Show()
		progress.Start()
		status.SetText("正在采集…")
		resultEntry.SetText("")
		opts := collect.Options{
			OutDir:          strings.TrimSpace(outEntry.Text),
			IncludeSessions: sessionsCheck.Checked,
			AllLogs:         allLogsCheck.Checked,
			ExtraVault:      strings.TrimSpace(vaultEntry.Text),
			OnProgress: func(msg string) {
				m := msg
				fyne.Do(func() { status.SetText(m) })
			},
		}
		go func() {
			res, err := collect.Run(opts)
			fyne.Do(func() {
				progress.Stop()
				progress.Hide()
				start.Enable()
				if err != nil {
					status.SetText("采集失败")
					dialog.ShowError(err, w)
					return
				}
				resultEntry.Enable()
				resultEntry.SetText(res.ZipPath)
				resultEntry.Disable()
				rows = rows[:0]
				for _, s := range res.Manifest.Sources {
					rows = append(rows, sourceRow{
						name:   s.Name,
						status: s.StatusText,
						files:  fmt.Sprintf("%d", s.Files),
						size:   pack.FormatBytes(s.Bytes),
						reason: s.Reason,
					})
				}
				table.Refresh()
				status.SetText("完成")
			})
		}()
	})
	start.Importance = widget.HighImportance

	copyBtn := widget.NewButton("复制路径", func() {
		p := strings.TrimSpace(resultEntry.Text)
		if p == "" {
			return
		}
		w.Clipboard().SetContent(p)
		status.SetText("已复制到剪贴板")
	})
	openBtn := widget.NewButton("打开所在文件夹", func() {
		p := strings.TrimSpace(resultEntry.Text)
		if p == "" {
			return
		}
		dir := filepath.Dir(p)
		_ = exec.Command("explorer", dir).Start()
	})

	form := container.New(layout.NewFormLayout(),
		widget.NewLabel("输出目录"),
		container.NewBorder(nil, nil, nil, browseOut, outEntry),
		widget.NewLabel("额外 Obsidian 库"),
		container.NewBorder(nil, nil, nil, browseVault, vaultEntry),
	)

	w.SetContent(container.NewBorder(
		container.NewVBox(
			intro,
			form,
			sessionsCheck,
			allLogsCheck,
			start,
			progress,
			status,
			widget.NewLabel("结果 zip"),
			container.NewBorder(nil, nil, nil, container.NewHBox(copyBtn, openBtn), resultEntry),
			widget.NewSeparator(),
		),
		nil, nil, nil,
		table,
	))
	w.ShowAndRun()
}

func uriPath(u fyne.ListableURI) string {
	p := u.Path()
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.Clean(p)
}
