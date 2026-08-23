# Claude-GPT 桥接裁剪导致 Desktop 失忆与断流

## Goal

修 `ForwardAsAnthropic` 的 API Key 续链：Desktop `/antigravity/v1/messages` 转 GPT 时必须把客户端带来的对话原样转给上游，禁止再出现「入站 200KB、转发只剩 2～3 条 input」。断流必须有 Anthropic 错误收口，禁止再出现 200 + 900s + 无账单。不换组、不关桥、不要求客户端改 `stream` / 端点 / body。

## Background

完整证据链见 `research/logic-chain.md`。这里只留验收必须用到的钉死事实。

客户：user 209 / key 317 / group 8 / 账号 1685 / 上游 gpt-5.5。入站一律 `dispatch_bridge`。

| 请求 | 入站 body | 转发 input_item_count | 结果 |
|---|---:|---:|---|
| d54f649c 23:48:03–17 | 207530 | 2 | 200 / 90 out，「看不到第 2、3 条」 |
| d7005e89 00:22:38 | 216570 | 3 | 200 / 39 out / 空开场 |
| e47e796f 12:43:20 | 342059 | 3 | 200 / 900407ms / 无 usage |

裁剪开关对 API Key + gpt-5.* 硬开启：`store=false`（`anthropic_to_responses.go:40-41`）却在有内存 `previous_response_id` 时执行 `trimAnthropicCompatResponsesInputToLatestTurn`（`openai_gateway_messages.go:442-445`）。直连 `/v1/chat/completions` 与原生 `/v1/responses` 不走这两步。

展示层 `cache_read` 是人造的，验收只看 raw input 与 `input_item_count`。

## Requirements

### R1 — API Key 桥接默认 full replay

`account.Type==APIKey` 的 Anthropic→Responses 路径：把客户端 `messages`（含 system）完整转换后转发。禁止 `applyAnthropicCompatFullReplayGuard`。禁止在 `Store!=true` 时调用 `trimAnthropicCompatResponsesInputToLatestTurn` 或写入 `previous_response_id`。

OAuth / Codex 续链不在本需求范围，不得被这次改动关掉。

### R2 — 禁止「假续链」

未证实上游按 `store=true` 存过上一轮之前，不得把内存 `previous_response_id` 配上已裁薄的 `input` 发给上游。换号不得把 A 号的 response id 配上残缺 input 打到 B 号。

### R3 — 12 条窗口不得再当 API Key 默认

`openAICompatAnthropicReplayMaxTailMessages=12` 不得再对 API Key 桥接默认生效。若以后保留为显式开关，必须永不丢掉最近一条真人 user 文本（`role=user` 且不是纯 `tool_result`）。本任务默认：删掉这条默认路径。

### R4 — 展示 cache 与排查分离

`applyClaudeGPTBridgeDisplayCacheOverride` 可保留。验收、告警、逻辑链只认 raw usage 与转发 `input_item_count`。trim / previous_response / input_item_count 必须打到带 `request_id` 的 Info。

### R5 — 断流收口

桥接流在没有可见 Anthropic `thinking` / `text` / `tool_use` 时，必须用 Anthropic SSE/JSON error 结束，禁止只回 200 空等。客户端取消后不得再 detach 到把上游读满 15 分钟。入站契约仍是 Desktop `POST /antigravity/v1/messages` + `stream=true`。不得把「客户端改 stream/端点/body」写成 Done。

interval 不得被上游空行 / 注释 / ping 无限续命到数十分钟；keepalive ping 不得单独把「无可见输出」伪装成成功完成。

## Acceptance Criteria

- [ ] AC1：用与 d7005e89 同类的请求（API Key + gpt-5.* + 入站 messages > 12 且末条为 tool_result）转发后，`input_item_count` 必须反映完整转换，不得再是 2 或 3。单测锁这个数。
- [ ] AC2：用与 d54f649c 同类的请求（上一跳 Read 成功，本跳 user 问「第 2、3 条」）转发后，`input` 必须仍含最近一次 `tool_result` 或与之等价的文件内容块，以及当前 user 文本。
- [ ] AC3：`store=false` 时 Responses body 不得出现 `previous_response_id`。单测锁。
- [ ] AC4：`/v1/chat/completions` 与原生 `/v1/responses` 行为不变（无这两条裁剪）。回归现有单测。
- [ ] AC5：无可见输出的桥接流必须给 Desktop 一个 Anthropic error 事件/JSON，handler 不得在已写 200 ping 后静默当成功。客户端 cancel 后上游读取必须停。
- [ ] AC6：人工：同一把 317、同一条 Desktop 会话，Glob/TaskList 失败后下一句必须回答刚刚的中文问题，不能 `How can I help?`；Read 成功后问「第 2、3 条怎么改」必须还能指到原文。

## Out of Scope

- 换 group 8、关 Claude-GPT 桥、改账号模型映射表当交付。
- 怪第三方 API / `previous_response_id` 不支持。
- 改展示计费公式、`actual_cost`、cache 展示百分比算法（除非日志字段）。
- 修 Glob 相对路径找不到 `C:\Users\user\.claude\CLAUDE.md`（客户端工具路径，不是本桥缺陷）。
- 生产 push / deploy（需 Brandon 当次授权）。
- 另开隔离 worktree，或把占用 checkout 切离 `main`（Brandon 已授权在占用 `main` 上实现）。

## Open Questions

无产品分歧。Brandon 已授权在占用 `main` 上实现；不要求隔离 worktree。
