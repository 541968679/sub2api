# Research: Raw CC buffer and inbound routing

- **Query**: Explain "Raw CC 缓冲" from code; when inbound `/v1/chat/completions` stays on upstream CC vs converts to Responses; file:line anchors for both sync-inbound SSE-buffer paths.
- **Scope**: internal
- **Date**: 2026-08-22

## Findings

### Files Found

| File Path | Description |
|---|---|
| `backend/internal/service/openai_gateway_chat_completions.go` | `ForwardAsChatCompletions` 分流；CC→Responses 同步 SSE 缓冲 `handleChatBufferedStreamingResponse` |
| `backend/internal/service/openai_gateway_chat_completions_raw.go` | Raw CC 直转；同步 SSE 缓冲 `bufferRawChatCompletionsFromSSE` + `rawChatSSEAccumulator` |
| `backend/internal/pkg/openai_compat/upstream_capability.go` | `ResolveUpstreamAPI`：passthrough / force_* / auto+Rsupp |
| `backend/internal/service/openai_gateway_sync_inbound_sse.go` | `shouldForceSyncInboundUpstreamSSE`：官方 S2 vs 自定义中转上游 SSE |
| `backend/internal/pkg/apicompat/responses_to_chatcompletions.go` | `BufferedResponseAccumulator.ProcessEvent` / `SupplementResponseOutput` |
| `backend/internal/service/openai_gateway_messages.go` | 本仓 Messages 路径已用 `readOpenAICompatBufferedTerminal`，读到终态立刻 return |
| `backend/internal/service/openai_gateway_chat_completions_test.go` | Responses 缓冲护栏 + Raw `TimeoutAfterUsableReturnsJSON` |

### When inbound CC stays on upstream CC vs converts to Responses

`ForwardAsChatCompletions` 顶部先按账号分流（`openai_gateway_chat_completions.go:92-97`）：

```text
Grok → 多数走 raw CC（OAuth 且 eligible 才 bridge Responses）
APIKey 且 ResolveUpstreamAPI(InboundChatCompletions, extra) == UpstreamChatCompletions
  → forwardAsRawChatCompletions（不做 CC↔Responses 转换）
其余（OAuth、force_responses、auto+Rsupp≠false / 未探测）
  → ChatCompletionsToResponses，上游 /v1/responses
```

`ResolveUpstreamAPI`（`upstream_capability.go:168-184`）：

| extra | inbound `/v1/chat/completions` 上游 |
|---|---|
| `openai_responses_mode=force_chat_completions` | CC（直转） |
| `openai_responses_mode=passthrough` | CC（直转） |
| `auto` + `openai_responses_supported=false` | CC（直转） |
| `force_responses` | Responses（转换） |
| `auto` + Rsupp=true / 键缺失（未探测） | Responses（转换；存量兼容） |

这就是 PRD 里的「force_chat_completions / passthrough / auto+Rsupp=false → Raw CC」。

### Sync inbound: S2 JSON vs upstream SSE + local buffer

两边共用 `shouldForceSyncInboundUpstreamSSE`（`openai_gateway_sync_inbound_sse.go:83-104`）：

- 入站 `stream=true`、非 OpenAI APIKey：永不 force。
- 账号 extra `openai_sync_inbound_upstream_sse` 可覆盖。
- 配置 `openai_sync_inbound_upstream_sse_mode`：`off` / `all` / `auto`（默认）。
- `auto`：仅自定义（非 `api.openai.com`、非空）`base_url` 为 true。官方 / 空 base_url 保持 **S2**（上游 `stream=false` JSON）。

Responses 路径：`openai_gateway_chat_completions.go:267-286` 改 `stream`，然后 `347-350`：

- 入站同步 + JSON Content-Type → `handleChatNonStreamResponsesJSON`（S2）。
- 入站同步 + SSE → `handleChatBufferedStreamingResponse`。

Raw 路径：`openai_gateway_chat_completions_raw.go:113-125` 改 `stream`，然后 `257-262`：

- 入站流式 → `streamRawChatCompletions`（透传 SSE）。
- 入站同步 + JSON → `bufferRawChatCompletions`（整包 JSON 透传）。
- 入站同步 + 非 JSON（即上游 SSE）→ `bufferRawChatCompletionsFromSSE`。

本任务只动「入站同步、上游 SSE、本地再组装一个 JSON」的两条缓冲循环；官方 S2 与入站 `stream:true` 不在范围内。

### Path 1 — CC→Responses buffer

`handleChatBufferedStreamingResponse`：`openai_gateway_chat_completions.go:670-811`。

`processLine`（698-719）只在这些事件且带 `event.Response` 时写入 `finalResponse`：

- `response.completed` / `response.done` / `response.incomplete` / `response.failed`

该行会先 `acc.ProcessEvent(&event)`（713）。accumulator 收的是 **completed 之前** 的 delta / part / item（`responses_to_chatcompletions.go:462-504`）。`finish` 时 `SupplementResponseOutput` 用已攒 delta 填空 `output`（550-561）。

`chatBufferedHasUsableTerminal`（500-505）：`resp != nil && status != "failed"`。  
**注意：它不是「第一个 delta」**。`finalResponse` 只有终态事件才会被赋值。名字容易和 Raw 的 `usable()` 混淆。

当前退出（**不**在写完 `finalResponse` 时立刻 return）：

1. scanner EOF / channel 关
2. 读出错：无终态且 H2 peer reset → failover；已有可用终态 → 仍 finish JSON
3. 静默满 `StreamDataIntervalTimeout`（默认 180s）且 `chatBufferedHasUsableTerminal` → finish JSON
4. 同样静默但无终态 → failover

`lastReadAt` 在每一行 `Scan()` 刷新（759），含空行 / `:` keepalive / `[DONE]`。终态后若还有 keepalive，180s 静默凑不齐——这是本任务要砍的尾巴。

本仓 **Messages** 路径已经立刻返回：`openai_gateway_messages.go:907` 调 `readOpenAICompatBufferedTerminal`，1326-1336 在 `isOpenAICompatResponsesTerminalEvent && event.Response != nil` 时 `return event.Response`。CC 缓冲循环没有复用这个 helper。

### Path 2 — Raw CC buffer（「Raw CC 缓冲」是什么）

Raw CC 缓冲 = 入站仍是同步 `/v1/chat/completions` JSON，但上游被改成 CC SSE；网关在 `bufferRawChatCompletionsFromSSE`（`openai_gateway_chat_completions_raw.go:682-828`）里把 chunk **拼回一个** `chat.completion` JSON 再 `c.Data`。

`rawChatSSEAccumulator`（516-528）按 chunk 累加 `content` / `reasoning` / `tool_calls` / `usage` / `finish_reason`。

`usable()`（530-532）：

```go
return a != nil && (a.hasMessage || a.finishReason != "" || a.usage != nil || len(a.toolCalls) > 0)
```

`hasMessage` 在第一个非空 `delta.content` 或 reasoning 时置位（566-573）。  
因此 **第一个 `"delta":{"content":"pong"}` 就为真**。这不是 CC 协议终态。

真终态应是：`finish_reason != ""`，或 `data: [DONE]`，或 `stream_options.include_usage` 的 usage-only 末包。工具调用还要等 arguments 拼完 + `finish_reason=tool_calls`。`processPayload` 对 `[DONE]` 是 no-op（536-537），**没有** `sawDone` 标记。

循环形状与 Responses 缓冲相同：EOF / 读错+`usable()` / 180s 静默+`usable()` 才 `finish()`。`TestBufferRawChatCompletionsFromSSE_TimeoutAfterUsableReturnsJSON`（test 897-921）正是「一个 content chunk 后 hang → 1s 后当成功 JSON」。那是 180s 护栏的妥协，**不能**升级成「一 usable 就回」，否则正常多 chunk 流会在首 token 被截断。`chatResponse`（629-647）在没有 `finish_reason` 时还会默认填 `"stop"`，截断 JSON 看起来像完整成功。

此路径是 fork-local：`91adbd539`（2026-08-20，`feat: passthrough OpenAI routing and buffer sync inbound via upstream SSE`）。上游 **没有** `bufferRawChatCompletionsFromSSE`。

### Code Patterns

- 两条缓冲循环都是 goroutine `Scan()` + `events` channel + 1s ticker 看 `lastReadAt`。
- 「有半成品就回 JSON、无半成品就 failover」是 08-18 护栏（`e5608cdbe`），不是「终态立刻返回」。
- Responses 的「可用」绑在终态事件对象上；Raw 的「可用」绑在首个可见 delta 上。谓词不可互换。

### Related Specs

- `.trellis/tasks/08-22-sync-buffer-return-on-terminal/prd.md`
- `.trellis/tasks/08-22-sync-buffer-return-on-terminal/design.md`

## Caveats / Not Found

- 近 24h `gpt-5.6-sol` 生产哪条账号走 Responses 缓冲、哪条走 Raw CC，本轮未再取证。路由由 extra 决定，两条都可能。
- Raw accumulator 没有 `sawDone`；若以后做 Raw「真终态立刻返回」，必须新增谓词，不能复用 `usable()`。
