# Claude 客户端日志采集器

给 Windows 客户用的绿色小工具：在本机扫描常见 Claude 客户端诊断材料，打成一份可发给运营的 zip。

- 不上传网络
- 不写注册表
- 不要求安装 Python / Go / Node
- 默认不含聊天全文、Obsidian 笔记正文、客户业务工程目录
- API key（`sk-` / `sk-ant-` / token）会打码；`.credentials.json` 原件不会进包

发给客户的是 **一个 exe**（图形界面）。命令行入口给开发和回归用，和图形界面走同一套 `collect.Run`。

## 客户怎么用

1. 把 `claude-log-collector-gui.exe` 拷到客户电脑（桌面即可）。
2. 双击运行。若 Windows SmartScreen 提示「Windows 已保护你的电脑」：点 **更多信息**，再点 **仍要运行**。第一版 exe 未签名，这是预期行为。
3. 输出目录默认是桌面，一般不用改。
4. 需要时再勾选：
   - **附带最近 24 小时会话（打码）** — 默认不要勾。只有运营明确要看会话时才勾。
   - **包含全部已发现日志** — 默认只收最近 7 天有改动的日志。
5. 若 Obsidian 库是便携版、官方列表里没有，点「额外 Obsidian 库」选那个库文件夹。工具只会采集该库 `.obsidian` 下与 Claude / Anthropic 相关的插件。
6. 点 **开始采集**。结束后复制 zip 绝对路径，或点 **打开所在文件夹**，把 zip 发给运营。

缺某个客户端（没装 Desktop、没开过 Obsidian）也会成功出包，对应来源会标「未安装 / 未找到」。

## 默认会打进包的东西

- Claude Code 打码后的 `settings.json` / `.claude.json`
- 最近 7 天的 debug / Desktop / CLI 诊断日志（可改为全部）
- Obsidian 官方库列表 + Claude 相关插件的打码配置
- 人读清单 `MANIFEST.txt` 和 `manifest.json`、`env-summary.json`（只写变量是否存在、URL 主机名、长度，不写明文 key）

默认 **不会** 打进包：`history.jsonl`、`transcripts/`、`.claude/projects` 会话 jsonl、vault 笔记、`.credentials.json` 原件。

## 命令行（开发 / 回归）

```powershell
cd "E:\cursor project\api2sub\tools\claude-log-collector"
go test ./... -count=1
go build -o bin/claude-log-collector.exe ./cmd/collector
.\bin\claude-log-collector.exe --out "$env:USERPROFILE\Desktop"
.\bin\claude-log-collector.exe --out D:\tmp --include-sessions --all-logs --vault D:\SomeVault
```

| 参数 | 含义 |
|---|---|
| `--out` | 输出目录，默认用户桌面 |
| `--include-sessions` | 附带最近 24 小时会话（打码） |
| `--all-logs` | 包含全部已发现日志 |
| `--vault` | 额外 Obsidian 库路径 |

## 怎么编 exe

在 `tools/claude-log-collector/`：

```powershell
# 命令行（不需要 CGO）
go build -o bin/claude-log-collector.exe ./cmd/collector

# 图形界面（需要 Windows CGO：MinGW-w64 / gcc）
go build -ldflags="-H windowsgui" -o bin/claude-log-collector-gui.exe ./cmd/gui
```

把 `claude-log-collector-gui.exe` 发给客户即可。采集逻辑只在 Go 里；不要把扫描/打码再写进 PowerShell。

若本机没有 gcc，`go test ./...` 仍应通过（图形界面用 `!cgo` 占位入口）。装好 MinGW 后再编 GUI。
