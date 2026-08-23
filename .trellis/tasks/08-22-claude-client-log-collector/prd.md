# Windows Claude 客户端日志提取工具

## Goal

让桥接 Claude 的 Windows 客户（当前触发：gybilly 只看到 `***Interrupted***`）在本机双击一个绿色 exe，采集常见 Claude 客户端诊断材料，得到一份可发给运营的 zip 与绝对路径。运营用来判断问题更像出在客户端、客户站点、本站网关，还是上游。

## Background

客户截图只有灰色斜体 `***Interrupted***`，没有 HTTP 状态、上游 JSON 或 request id。该文案通常是客户端掐断/失败后的统一展示，不能单独判责。工具只保证把找得到的客户端诊断材料打成包；响应体若从未落盘，包里也不会出现。

本仓库 `tools/` 现有脚本面向开发者（如压测），没有面向客户的 Windows GUI 采集器。本轮不改 Sub2API 网关热路径，不采集本站/客户站网关日志，不要求客户改协议 / `stream` / 端点。

本机 Windows 已核实的落点（客户机可能多/少）：`%USERPROFILE%\.claude\`（含 `settings.json` 的 `env`，常有 `ANTHROPIC_BASE_URL` / key）、`%LOCALAPPDATA%\Claude\Logs\`、`%LOCALAPPDATA%\claude-cli-nodejs\Cache\`。官方还支持 `CLAUDE_CONFIG_DIR` 把 `~/.claude` 挪走。Obsidian 库列表在 `%APPDATA%\Obsidian\obsidian.json`（大小写以本机为准）。客户对话可能含业务数据（截图为预存款/退货），因此默认不打包聊天全文。

## Requirements

- R1. 发给客户的是单个绿色 exe：中文 GUI，双击即用，不写注册表，不要求安装 Python/Go/Node。源码在本仓 `tools/claude-log-collector/`；另留同一套逻辑的命令行入口做回归。
- R2. GUI 可选择输出目录（默认桌面）、勾选「附带最近 24 小时会话（打码）」、勾选「包含全部已发现日志」、再选一个额外 Obsidian 库路径。一键采集后展示进度。
- R3. 采集结束后界面展示：zip 绝对路径（可复制/可打开所在文件夹）、各来源找到/未找到、文件数/体积、失败原因（路径不存在 / 无权限 / 跳过）。
- R4. 默认扫描白名单：Claude Code 用户目录（含 `CLAUDE_CONFIG_DIR` 与用户环境变量里的同名覆盖）、`%USERPROFILE%\.claude.json`（只进打码副本）、Claude Desktop 日志/配置目录、Obsidian 本机应用目录与已登记 vault 的 Claude 相关插件材料、`%LOCALAPPDATA%\claude-cli-nodejs` 下的诊断日志。允许在 `%APPDATA%` / `%LOCALAPPDATA%` 顶层按名称匹配 `*Claude*` / `*Anthropic*` 做有界扫描。禁止扫整盘，禁止打包客户业务工程或 vault 笔记正文。
- R5. Obsidian：自动读官方库列表；GUI 允许再选一个库文件夹。只采集该库 `.obsidian` 下与 Claude/Anthropic 相关的插件日志与打码配置。
- R6. 默认产物：一份 zip + 人读清单（来源、路径、时间窗、本机环境摘要）。环境摘要始终包含 OS、是否存在关键配置文件、`ANTHROPIC_BASE_URL` 等主机名（无明文 key）。
- R7. 默认「精简诊断包」：客户端日志 + 打码配置 + 环境清单。未勾选会话时不得含 `history.jsonl`、`transcripts/`、`projects\` 会话 jsonl 全文。勾选后只纳入最近 24 小时会话且同样打码。
- R8. 日志时间窗：默认只收最近 7 天有改动的日志/诊断文件（按 LastWriteTime）。勾选「全部日志」则不受 7 天限制。打码后的配置摘要不受 7 天限制，始终进精简包。
- R9. 凭证不得明文进包：`sk-` / `sk-ant-` / `.credentials.json` 原件、settings/`env` 里的 key、常见 token 字段打码或排除。
- R10. 工具不上传网络。缺某个客户端时仍出包，对应来源标「未安装 / 未找到」，不得整次失败。
- R11. 第一版 exe 未签名，接受 SmartScreen「仍要运行」。

## Acceptance Criteria

- [ ] AC1. 在装有 Claude Code 的 Windows 上，GUI 一键采集产出 zip，并显示该 zip 的绝对路径。
- [ ] AC2. zip 内含清单；每个来源写明找到/未找到。
- [ ] AC3. 未安装 Claude Desktop 或未打开过 Obsidian 时，采集仍成功，对应来源为未找到。
- [ ] AC4. 含 API key 的 settings / credentials / `.claude.json` 样例，产物中看不到明文 key。
- [ ] AC5. 默认采集不会把客户业务工程目录或 vault 笔记正文打进包。
- [ ] AC6. 客户只拿一个 exe 即可完成采集；界面中文。源码在 `tools/claude-log-collector/`。
- [ ] AC7. 默认不勾选「附带会话」时，zip 内不得出现 `history.jsonl`、`transcripts/` 或会话 jsonl 全文。
- [ ] AC8. 勾选「附带最近 24 小时会话」时，只纳入该时间窗内的会话记录，且打码；清单标明已包含会话。
- [ ] AC9. 能从 Obsidian 官方库列表自动发现 vault；客户手动选的额外库只采集其 `.obsidian` 下 Claude 相关插件材料。
- [ ] AC10. 默认不勾选「全部日志」时，日志/诊断文件按 LastWriteTime 只收最近 7 天；打码配置摘要仍在。
- [ ] AC11. 命令行入口与 GUI 使用同一套采集/脱敏/打包逻辑；可用夹具目录跑通 AC4/AC7/AC10。

## Out of scope

- macOS / Linux 采集器
- 自动判定 Interrupted 根因，或对接本站 ops API
- 采集客户站点 / 本站网关日志
- 修改 Claude 客户端、Obsidian 插件或网关代码
- 上传到任何服务器
- 做成 Sub2API 管理后台的一部分
- 代码签名 / SmartScreen 声誉
- 默认采集 Cursor / VS Code 全量日志（仅当顶层目录名匹配 Claude/Anthropic 白名单时才碰）

## Decisions

- D1. 默认精简诊断包；可选「附带最近 24 小时会话（打码）」。
- D2. 源码进 `tools/claude-log-collector/`；发给客户单个绿色 exe。第一版未签名。
- D3. Obsidian：自动发现已登记库 + 客户可再选一个库路径；不扫整盘，不打包笔记正文。
- D4. 默认只收最近 7 天有改动的日志/诊断文件；可勾选全部日志。打码配置摘要始终采集。
