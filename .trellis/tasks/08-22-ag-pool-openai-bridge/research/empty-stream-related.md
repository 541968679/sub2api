# Research: 桥接空流 vs 池加不了 OAI 号

Date: 2026-08-22. **本任务默认不实现空流修复。**

## 现象

客户端仍经常突然断流。生产 48h 有 41 条空流 502，账号 1724 `tokenbits 012`，文案：

`Upstream messages stream completed without assistant content or tool output`

Brandon：空流之前好像修过一次但没管用。

## 与「池加不了 OAI 号」不是同一根因

| | 池问题 | 空流 |
|---|---|---|
| 何时发生 | 选号之前 / 准入 | 号已被选中、上游已开始或已完成 |
| 根因 | 写路径平台硬匹配 + lookup 用 `account.Platform=openai` → 桥接流量挤进 openai 闭池 | 上游空响应 / 超时 / 可见输出判定；converter 没收到可展示的 assistant/tool |
| 证据 | 用户 12 无 AG 策略；分组 15 与分组 19 共享 openai 池 | 41 条打在**已选中**的桥接号 1724 上 |

加宽 AG 池或改 lookup **不会**让 1724 在已被选中后少出空 SSE。

## 历史修复（代码 + gateway.md，CHANGELOG 无单独「空流」标题）

这些改的是**已被选中的号**上的转换/failover，不是选号面：

1. **Haiku→GPT 空输出当 request-shaped**  
   `ApplyClaudeHaikuBridgeUpstreamAdjustments`（effort low、max_output 地板）。空完成设 `NoAccountFailover`，避免烧整池。日志：`openai_messages.empty_visible_output_no_account_failover`。见 `gateway.md` Haiku 机制。

2. **GPT-5.6-terra / GPT-5.5 只在 `content_part` 出字**（2026-08-17 生产）  
   有 `response.content_part.added/done` + `response.output_text.done`，**零** `output_text.delta`；terminal message 为空。converter 必须收 `part.text`。漏收会 502。诊断字段：`content_part_text_bytes`。见 `gateway.md` Known Pitfalls。

3. **空完成 → `newOpenAIEmptyVisibleOutputError`**  
   `openai_gateway_messages.go:1709-1731`：`stream_completed_without_visible_output` 后 `NoAccountFailover`。未写出客户端 body 前是 HTTP 502 JSON；已 ping/flush 则 `event: error`。spec：`.trellis/spec/backend/anthropic-messages-sse.md`。

4. **Compact 空摘要**  
   未启动 transport 保持 502 JSON；已启动则 `event: error`，禁止 HTTP 200 + 空 `message_stop`。

这些修的是「空完成被当成成功」或「不该换号却换号」。客户端仍断流，说明还有：上游真的空、超时、可见输出判定仍漏、或客户端侧掐断。**上次修复无效的复盘不在本任务。**

## 开放决策（F，默认否）

- 是否另开子任务做空流？
- 要不要先复盘上次修复为什么对 1724 无效（读该号 41 条的 `content_part_text_bytes` / incomplete / max_output_tokens）？
- 验收禁止「让客户端改 stream / 端点 / body」（gateway-no-client-workaround）。

## 本任务边界

PRD 记录现象。不改 `openai_gateway_messages.go` 空完成路径，除非 Brandon 把 F 改成纳入本任务。
