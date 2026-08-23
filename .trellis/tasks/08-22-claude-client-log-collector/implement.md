# Implement：Windows Claude 客户端日志提取工具

未 `task.py start` 前不改产品代码、不落地 `tools/claude-log-collector/`。

## 顺序

1. 独立模块骨架：`tools/claude-log-collector/go.mod`，`internal/scan` 路径解析（含 `CLAUDE_CONFIG_DIR`、Desktop 变体、Obsidian `obsidian.json`）。夹具目录覆盖「有/无客户端」。
2. `internal/redact`：key / Bearer / JSON 字段 / URL userinfo；`.credentials.json` 排除。单测 AC4。
3. `internal/collect` + `internal/pack`：7 天窗、可选全部日志、可选 24h 会话、来源状态、zip+清单。单测 AC2/AC3/AC5/AC7/AC8/AC10。
4. `cmd/collector`：`--out` `--include-sessions` `--all-logs` `--vault`。与 GUI 共用 `collect.Run`。
5. `cmd/gui`：中文 Fyne 窗口（输出目录、两勾选、额外库、路径/复制/打开文件夹、来源表）。`-H windowsgui`。
6. `tools/claude-log-collector/README.md`：客户怎么用、SmartScreen、你怎么编 exe。`docs/dev/CHANGELOG_CUSTOM.md` 一笔。禁止改网关/设置/migration。

## 验证

在 `tools/claude-log-collector/`：

```powershell
go test ./... -count=1
go build -o bin/claude-log-collector.exe ./cmd/collector
```

GUI（本机有 CGO 时）：

```powershell
go build -ldflags="-H windowsgui" -o bin/claude-log-collector-gui.exe ./cmd/gui
```

用夹具跑：无 Desktop/Obsidian 仍出包；默认 zip 无 history/transcripts；带 key 的 settings 打码后无明文。

## 风险点

| 点 | 风险 | 防法 |
|---|---|---|
| GUI 另写一套扫描 | 行为分叉 | 只调 `collect.Run` |
| 打码前写入 zip | 泄露 key | 先 redact 再 pack；AC4 |
| 扫 vault 正文 | 业务笔记外泄 | 只碰 `.obsidian/plugins` 白名单；AC5/AC9 |
| 收进 plugins/cache | zip 膨胀 | 排除市场缓存目录 |
| Fyne/CGO 编不过 | 交不出 exe | 先保 CLI；壳只调 CLI，逻辑不搬家 |

## 回滚点

第 1–4 步可整目录删除。第 5 步只多一个 GUI 入口。无 DB、无部署。

## start 前

- D1–D4 已锁。
- 用户看过 `prd.md` / `design.md` / `implement.md` 或明确说可以 `task.py start`。
- `implement.jsonl` / `check.jsonl` 有真实条目，不是只留 `_example`。
