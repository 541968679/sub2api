# Design：Windows Claude 客户端日志提取工具

独立小工具，不进网关热路径，不进 Wire/Ent。

## 白话结构

```text
客户双击 exe
  → 勾选（会话 / 全部日志 / 额外 Obsidian 库）
  → 同一套 collect 引擎按白名单扫
  → redact 后写临时目录
  → zip + manifest
  → 界面给出绝对路径
```

```text
tools/claude-log-collector/          独立 go.mod，不绑 backend
  internal/scan     来源发现（纯函数，可测）
  internal/redact   打码 / 排除
  internal/pack     清单 + zip
  internal/collect  编排
  cmd/collector     命令行（回归）
  cmd/gui           Windows GUI（调用 collect）
```

## 边界

- 只读客户机上的白名单路径；默认不写除输出 zip / 本工具临时目录以外的位置。
- 不访问网络（不拉更新、不上传）。
- 不读客户业务工程、不读 vault 笔记正文。
- GUI 与 CLI 必须调用同一 `collect.Run(opts)`；禁止两套扫描实现。

## 来源注册表

每条来源：`id`、展示名、根路径解析、纳入规则、缺失时的状态文案。

| id | 根 | 默认纳入 |
|---|---|---|
| `claude-code-config` | `CLAUDE_CONFIG_DIR` 或 `%USERPROFILE%\.claude` | 打码后的 `settings*.json`；排除 credentials 原件、history/transcripts/projects 会话 |
| `claude-json` | `%USERPROFILE%\.claude.json` | 仅打码副本 |
| `claude-code-debug` | `debug/` 与 `CLAUDE_CODE_DEBUG_LOGS_DIR` | 日志，走 7 天窗 |
| `claude-cli-nodejs` | `%LOCALAPPDATA%\claude-cli-nodejs` | `*log*` / `*.jsonl` 诊断，排除大块 Cache blob，走 7 天窗 |
| `claude-desktop` | `%LOCALAPPDATA%\Claude`、`%APPDATA%\Claude`、Anthropic 变体 | `Logs\` + 打码 config |
| `obsidian-app` | `%APPDATA%\Obsidian` | 应用日志；`obsidian.json` 只取 vault 路径列表（可打码用户名以外的 path 保留） |
| `obsidian-vaults` | 登记库 + GUI 额外库 | 仅 `.obsidian/plugins` 中名称含 `claude`/`anthropic` 的目录，及 `data.json` 含这些关键字的 copilot 类插件 |
| `appdata-bounded` | `%APPDATA%` / `%LOCALAPPDATA%` **一层** | 目录名匹配 `*Claude*` / `*Anthropic*` 且未被上面覆盖的 `Logs`/`*.log` |

时间窗：日志类文件 `LastWriteTime >= now-7d`，除非 `AllLogs`。配置摘要类始终采集。

会话可选：`IncludeSessions` 时从 config root 收 `history.jsonl` / `transcripts/` / `projects\**\*.jsonl` 中 mtime 在 24h 内的文件，经 redact 后放入 `sessions/`。

## 脱敏

- 整文件排除：`.credentials.json` 原件、明显私钥/cookie 库。
- 文本/JSON 打码：`sk-ant-`、`sk-`、`Bearer`、`apiKey`/`api_key`/`token`/`authorization`/`ANTHROPIC_API_KEY` 等字段值 → `***REDACTED***`。
- URL：保留 scheme + host，去掉 userinfo 与 query 中的 key。
- 环境摘要：只写「变量存在 / 主机名 / 长度」，不写 secret 原文。
- 打码必须对拷贝后的字节做，不得先把原文写入 zip。

## GUI

中文窗口（Fyne v2，`-H windowsgui`）：

- 输出目录（默认用户桌面）
- 勾选：附带最近 24 小时会话；包含全部日志
- 额外 Obsidian 库（文件夹选择，可空）
- 开始采集
- 结果：路径、复制、打开文件夹、来源表

构建需要 Windows CGO/gcc 时，在 `tools/claude-log-collector/README.md` 写明。若本机编 GUI 受阻，允许先交 CLI exe + 调用它的最小 WinForms/PowerShell 壳，但采集逻辑仍只在 Go。

## 兼容与回滚

- 不改 backend、不改 migration、不加设置项。
- 回滚：删除 `tools/claude-log-collector/` 即可，无运行时耦合。
- 客户侧：扔掉 exe；本机不留服务。

## 风险

| 点 | 风险 | 处理 |
|---|---|---|
| 扫太大 | Cache / 插件资源进包 | 扩展名与大小上限；排除 `plugins/cache` 市场缓存 |
| 漏库 | 便携 Obsidian | 手动选库 |
| 漏 debug | 客户没开 `--debug` | 清单写明 debug 未找到；不假装有错误体 |
| 未签名 exe | SmartScreen | README 写「仍要运行」 |
| 误收笔记 | vault 扫太宽 | 只碰 `.obsidian/plugins` 白名单 |
