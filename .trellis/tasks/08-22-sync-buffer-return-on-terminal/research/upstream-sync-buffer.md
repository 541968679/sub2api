# Research: Upstream Wei-Shaw/sub2api sync-inbound SSE buffer

- **Query**: Does upstream already return on a true terminal for sync inbound SSE buffer? Any related commit/PR for hang after `response.completed`, interval timeout, HTTP/2, or Raw CC `usable()`?
- **Scope**: mixed (local `upstream` remote + GitHub `Wei-Shaw/sub2api`)
- **Date**: 2026-08-22
- **Upstream HEAD checked**: `d45135d87` (`2026-08-22`, `Merge pull request #6068`)
- **Local remote**: `upstream` = `https://github.com/Wei-Shaw/sub2api.git`（本次 `git fetch upstream main`）
- **gh account**: `541968679`

## Findings

### Verdict

| Question | Answer |
|---|---|
| Upstream 是否已有「读到 Responses **真终态**立刻返回、不再等 body EOF」？ | **有。** `readOpenAICompatBufferedTerminal` 在终态事件 + response 对象上直接 `return`。 |
| Upstream 是否修了 Raw CC SSE 缓冲 / `usable()`？ | **没有这条路径。** 同步 raw CC 只 `bufferRawChatCompletions` 整包 JSON。 |
| 能否整文件搬上游 `handleChatBufferedStreamingResponse`？ | **不能。** 上游仍对 API Key 同步强制上游 SSE；本仓官方 S2 + 无终态 H2/180s failover 是 fork 护栏。 |
| 本轮是否要 invent fork-only 启发式？ | **不要。** Responses 侧应 **adopt/adapt** 上游已有的真终态谓词。 |

### Files Found (upstream `FETCH_HEAD`)

| File Path | Description |
|---|---|
| `backend/internal/service/openai_gateway_chat_completions.go:453-464` | CC 缓冲入口直接调用 `readOpenAICompatBufferedTerminal` |
| `backend/internal/service/openai_gateway_messages.go:653-659` | `isOpenAICompatResponsesTerminalEvent` |
| `backend/internal/service/openai_gateway_messages.go:697-717` | `openAICompatTerminalResponse`（无 `event.Response` 时可为 failed/error 合成） |
| `backend/internal/service/openai_gateway_messages.go:719+` | 共享缓冲读取器；865 行附近终态立刻 return |
| `backend/internal/service/openai_gateway_chat_completions_raw.go:435` | 仅 JSON `bufferRawChatCompletions`；**无** `bufferRawChatCompletionsFromSSE` |
| `backend/internal/service/openai_gateway_compat_buffered_read_failover_test.go` | PR #5801：终态前读失败 → failover |

本仓 **没有** `shouldForceSyncInboundUpstreamSSE` / `openai_gateway_sync_inbound_sse.go` 的上游对应物。`git grep` 上游无 `bufferRawChatCompletionsFromSSE`、`rawChatSSEAccumulator`、`chatBufferedHasUsableTerminal`。

### Code Patterns

上游 `handleChatBufferedStreamingResponse`（`d45135d87` `openai_gateway_chat_completions.go:453-464`）不再自管 Scan 循环：

```text
finalResponse, usage, acc, err := s.readOpenAICompatBufferedTerminal(
    resp, c, "openai chat_completions buffered", requestID)
```

`readOpenAICompatBufferedTerminal`（同 commit，`openai_gateway_messages.go:844-865` 附近）在解析完一帧后：

```text
if response := openAICompatTerminalResponse(&event, payload);
   isOpenAICompatResponsesTerminalEvent(event.Type) && response != nil {
    // copy usage onto response
    return response, usage, acc, nil
}
```

这就是「可用终态立刻返回」。不再等 EOF，也不再等 `StreamDataIntervalTimeout` 之后才用手里的 `completed`。interval 只在 **还没有终态**、行间静默时触发，然后 `Close` body 并返回 `"stream data interval timeout"`。

真终态谓词（上游比本仓 **更宽**）：

```go
// FETCH_HEAD openai_gateway_messages.go:653-656
case "response.completed", "response.done", "response.incomplete",
     "response.failed", "response.cancelled", "response.canceled", "error":
```

本仓 Messages helper（`openai_gateway_messages.go:1134-1140`）和 CC `processLine`（`openai_gateway_chat_completions.go:715-718`）只有前四个。`cancelled` / `error` 是上游后续加宽，**不要本轮无评估整包搬过来**。adopt 的是「这四个 + `event.Response != nil` 立刻 finish」，不是上游整个 helper。

`[DONE]`：上游若在终态之前看到 `data: [DONE]`，直接 `return nil`（无终态）。本仓 CC 循环把 `[DONE]` 当空操作继续读。adapt 时不要误把 `[DONE]` 当成功 JSON。

### External References

- [PR #5801](https://github.com/Wei-Shaw/sub2api/pull/5801) merged 2026-08-19 — `fix(openai): 修复 Chat 非流式缓冲读取错误未触发故障转移`  
  commit `b228b93e9` / merge `e8b53c919`。  
  PR 正文写明当前上游设计：入站 `stream=false` 的 CC **转换成 Responses 并强制上游 SSE**；网关聚合 **直到终止事件** 再转 Chat JSON。补的是 **终态前** `unexpected EOF` / HTTP/2 reset → `UpstreamFailoverError`，不是「completed 后再等 180s」。  
  **不在本仓 `main` 祖先里**（`git merge-base --is-ancestor e8b53c919 HEAD` = false）。
- [PR #5415](https://github.com/Wei-Shaw/sub2api/pull/5415) merged 2026-08-11 — 空的 `response.completed`（无 output/usage/error）当静默拒绝并 failover。与「completed 后 hang」无关。
- commit `72d5ee4cd` (2026-05-03, shaw) `fix: drain OpenAI compat streams for usage` — 引入共享 `readOpenAICompatBufferedTerminal`。
- commit `cc5328c49` (2026-05-17) `修复 OpenAI Responses SSE 终止事件识别` — 收紧终止事件识别。
- 近期 `upstream/main`（2026-08）提交主题：Ollama reasoning、Grok 重试、DeepSeek tools、Codex guardian、plaza。**没有**「completed 后立刻返回 / 砍 interval 尾巴 / Raw CC usable」这类 PR。
- GitHub search `handleChatBufferedStreamingResponse`：命中上述文件。`bufferRawChatCompletionsFromSSE`：**0 hits**。
- Issues：#4605 是 Grok bridge 无终态 502；#3603 是流式 preamble hold。都不是本任务的 completed-after-hang。

### Fork vs upstream (do not conflate)

| | 本仓 `main` | 上游 `main` @ `d45135d87` |
|---|---|---|
| 官方 / 空 base_url API Key 同步 | S2 JSON（`e5608cdbe` / `91adbd539`） | 仍强制上游 SSE 再缓冲（#5801 正文） |
| CC→Responses 缓冲退出 | 写 `finalResponse` 后继续读；EOF / 180s / 读错才 finish | 真终态立刻 return |
| 无终态 H2 reset | `isOpenAIHTTP2PeerReset` → failover | #5801 分类为 `upstream_http2_stream_error` failover（本仓未合入该 PR） |
| 无终态 180s | failover | interval 报错；Chat 调用点再包 failover |
| Raw CC 同步 SSE 缓冲 | `bufferRawChatCompletionsFromSSE` + `usable()` | **不存在** |
| Messages 缓冲 | 本仓已立刻 return（`messages.go:1326`） | 同一 helper |

本仓相关 fork commit（**不在** upstream）：

- `e5608cdbe` (2026-08-18) `perf(gateway): speed up chat completions conversion path` — 引入 `chatBufferedHasUsableTerminal`、官方改 S2、H2/interval failover。
- `91adbd539` (2026-08-20) `feat: passthrough OpenAI routing and buffer sync inbound via upstream SSE` — 自定义中转强制上游 SSE + Raw CC SSE 缓冲。

### Related Specs

- `.trellis/tasks/08-22-sync-buffer-return-on-terminal/prd.md` — Q1 本轮默认只做 Responses；prefer adopt/adapt。
- `.trellis/tasks/08-22-sync-buffer-return-on-terminal/design.md` — 不得用 Raw `usable()` 当立刻返回条件。

## Caveats / Not Found

- 上游 **没有** 针对「completed 之后 keepalive 刷新 `lastReadAt`、180s 凑不齐」的独立 hotfix。他们的循环在 completed 当帧就 return，这个问题在上游 CC→Responses 缓冲上 **不会出现**。
- 上游 **没有** Raw CC「真终态（`finish_reason` / `[DONE]`）立刻返回」。本仓 Raw 路径不能声称 adopt 上游。
- 不要整段替换本仓 `handleChatBufferedStreamingResponse` 为上游版本：会撤回 S2，并丢掉本仓已上线的无终态 H2/180s 形态。
- 上游终态集合含 `cancelled` / `error`；本轮 adapt 只对齐本仓已识别的四类 + `event.Response != nil`。
- 未逐条打开 2026-08 全部上游 PR 正文；code search + `git grep` + 上述 PR/commit 已覆盖同名符号。若有未索引的讨论帖，不在本次证据里。
