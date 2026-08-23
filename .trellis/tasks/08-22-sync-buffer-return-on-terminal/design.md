# Design: 同步缓冲终态立刻返回

## 会不会把 2026-08-18 的 76s 挂死带回来？

**结论：不会，只要不撤那次的三条护栏，并且「立刻返回」只绑在「已有可用终态」上。**

那次挂死和这次要砍的空等，方向相反。

| | 76s 挂死（已修） | 这次要砍的空等 |
|---|---|---|
| 手里有没有终态 | **没有** | **有** `completed` |
| 网关在等什么 | 死连接的 EOF | 死连接的 EOF / 180s 静默 |
| 客户端得到什么 | 一直没有 JSON，最后 failover 或等到连接死 | 已经能组 JSON，却还没写 |
| 正确动作 | 无终态 fail-fast **换号** | 有终态 **马上回 JSON，不换号** |

76s 的因果链：

```text
API Key 同步一律上游 SSE
  → 中转 H2 INTERNAL_ERROR
  → body 不关、也没有 response.completed
  → 缓冲循环一直 Scan()
  → 等到连接死（几十秒）
```

立刻返回只改这一刀：

```text
已经 process 到 response.completed（status≠failed）
  → 立刻 finish（写 JSON）
  → Close body
```

无终态时的 H2 reset / 180s failover **原样保留**。官方继续 S2，不会把「官方也 SSE 缓冲」那条伤口打开。

现有测试已经锁住「不会复现」的一侧：

- `TestHandleChatBufferedStreamingResponse_HTTP2PeerResetFailovers`：无终态 H2 → failover，body 空
- `TestHandleChatBufferedStreamingResponse_IntervalTimeoutFailovers`：无终态 hang → interval 内 failover
- `TestHandleChatBufferedStreamingResponse_TimeoutAfterCompletedReturnsJSON`：completed 后 hang → 成功 JSON（今日仍可能等到 interval）

本任务只把第三条从「等 interval」收成「读到 completed 就回」。前两条不得变绿变红。

## 立刻返回不会变成「过早截断」（Responses 路径）

OpenAI Responses SSE 顺序是：created → in_progress → item/delta → `output_text.done` / `output_item.done` → **`response.completed`（带完整 response + usage）** → `[DONE]`。

`BufferedResponseAccumulator.ProcessEvent` 只收 delta / part / item，**不依赖 completed 之后的事件来补文本**。`SupplementResponseOutput` 在 finish 时用**已经攒下的** delta 去填空的 `output`。

因此在 `completed` 当行 `ProcessEvent` 之后立刻 finish：

- usage 在 `finalResponse.Usage` 上（completed 自带）
- 空 output 可用已攒 delta 补
- `[DONE]` 本就在 `processLine` 里丢掉

截断风险在这条路径上 **可约束住**：谓词是终态事件类型 + `event.Response`，不是「有过 delta」。`chatBufferedHasUsableTerminal` 名字像 Raw 的 `usable()`，实现不是——`finalResponse` 只在四类终态上赋值。验收必须锁「只有 delta / created 时 hang → 不得成功 JSON」（PRD AC8）。

风险只剩协议违规中转：先发 `completed` 再发更多 delta。官方不会。本仓 accumulator 也按「delta 在 completed 之前」写。不为此再等网上。不另写「内容看起来齐了」的启发式。

## 上游已有真终态立刻返回（优先 adapt，不 invent）

2026-08-22 `git fetch upstream main` → `d45135d87`。证据见 `research/upstream-sync-buffer.md`。

上游 `handleChatBufferedStreamingResponse` 把读循环交给 `readOpenAICompatBufferedTerminal`。该 helper 在

`isOpenAICompatResponsesTerminalEvent(type) && openAICompatTerminalResponse(...) != nil`

时 **立刻 return**，不再等 EOF / interval。PR [#5801](https://github.com/Wei-Shaw/sub2api/pull/5801) 正文把这写成既有设计：「聚合直到终止事件，再转 Chat JSON」。#5801 补的是终态 **前** 读失败 failover。

本仓 Messages 路径已经是这套立刻返回（`openai_gateway_messages.go:1326`）。CC→Responses 缓冲仍是 08-18 的「写完 `finalResponse` 继续读」。本任务只把 CC 缓冲的退出点 **对齐** 到本仓已有的四类终态（`completed` / `done` / `incomplete` / `failed`），成功路径仍 `status != failed`。

**不要**整文件换成上游实现：上游仍对官方 API Key 同步强制 SSE；本仓 S2 + 无终态 H2/180s failover 必须留下。上游终态集合还多了 `cancelled` / `error`，本轮不扩。

**不要**为 Raw CC 发明立刻返回。上游没有该路径；本仓 `usable()` 会截断。

## 立刻返回会变成截断的路径：Raw CC

`rawChatSSEAccumulator.usable()`：

```go
return hasMessage || finishReason != "" || usage != nil || len(toolCalls) > 0
```

第一个 `"delta":{"content":"pong"}` 就为真。现有 `TestBufferRawChatCompletionsFromSSE_TimeoutAfterUsableReturnsJSON` 正是「一个 content chunk 后 hang → 1s 后当成功 JSON」。那是 **180s 护栏的妥协**（有半成品就回，避免等到连接死），**不能**升级成「一 usable 就回」，否则正常多 chunk 流会在首 token 被截断。

Raw 的真终态是：`finish_reason != ""`，或看到 `data: [DONE]`，或 usage-only 末包（`stream_options.include_usage`）。工具调用必须等 arguments 拼完 + `finish_reason=tool_calls`。

## 读循环怎么改（Responses）

`processLine` 在写入 `finalResponse` 后返回一个「可 finish」标记。主循环：

1. `processLine` 该行（保证终态当行已 `ProcessEvent`）
2. 若可用终态 → `finishChatCompletionsFromResponsesResponse` 并 return
3. 否则维持现有 select：EOF / 读错 / interval

`interval<=0` 的同步 scanner 同样：一看到可用终态就 finish，不必读到 EOF。

已在 `events` channel（缓冲 16）里、**终态之前**的行照常处理。终态之后不必再 `Scan()`。`defer resp.Body.Close()` 仍在 `ForwardAsChatCompletions`；未读完就 Close，Go HTTP/2 会 RST 该 stream 并丢掉连接复用。这是「别等死连接」要的，不是 76s 那种 **阻塞读到死**。

## 不得做的事（否则才会复现 76s）

- 官方改回「同步也上游 SSE」
- 删掉无终态 H2 failover
- 删掉无终态 180s failover
- 把 raw `usable()` 当成立刻返回条件
- 在 finish 之后还继续阻塞读「以防万一」

## Compatibility

- 入站仍一个 Chat Completions JSON。
- 计费：completed 上已有 usage；与今日「等 EOF 再 finish」成功路径一致。
- 调度：同步仍不写 `FirstTokenMs`（本任务不补）。

## Rollback

只改缓冲循环退出点。回退即恢复「等 EOF / 180s」。`openai_sync_inbound_upstream_sse_mode` 不用动。
