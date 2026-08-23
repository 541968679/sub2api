# Implement: 同步缓冲终态立刻返回

## Locked scope

只改本仓 CC→Responses 缓冲循环 `handleChatBufferedStreamingResponse` 的退出点。  
谓词对齐本仓 Messages helper：`isOpenAICompatResponsesTerminalEvent(event.Type) && event.Response != nil`。  
成功路径仍经 `finishChatCompletionsFromResponsesResponse`；`status=failed` 走该函数已有 failed / cyber / failover，不当成功 JSON。

禁止：整文件换上游；撤官方 S2；改 Raw `usable()` 立刻返回；扩终态到 `cancelled` / `error`；改客户端 `stream` / 端点 / body。

## Ordered checklist

1. **谓词**  
   `processLine` 在 `ProcessEvent` 之后，用 `isOpenAICompatResponsesTerminalEvent` + `event.Response != nil` 写入 `finalResponse`，并返回「本行是终态」。不要新写启发式。

2. **interval > 0 循环**  
   `processLine` 后若终态：立刻 `finishChatCompletionsFromResponsesResponse` 并 return。不再等 EOF / `StreamDataIntervalTimeout`。  
   无终态：保持 H2 peer reset → `UpstreamFailoverError`；180s（测试 1s）静默 → failover。

3. **interval <= 0 同步 scanner（R5）**  
   读到终态当行后立刻 finish，不必扫到 EOF。读错且无可用终态仍走现有 H2 failover。

4. **failed**  
   终态当行也会立刻进 `finish`；`status=failed` 不得写出成功 Chat JSON。

5. **测试**  
   - 收紧 `TimeoutAfterCompletedReturnsJSON`：远小于 1s interval（立刻回，含 completed usage）。  
   - 保持 `IntervalTimeoutFailovers` / `HTTP2PeerResetFailovers` / `PacedSSESurvivesIntervalTimeout`。  
   - 新增 AC5：`response.failed` 不得成功 JSON。  
   - 新增 AC8：只有 `output_text.delta` / `response.created` / `in_progress` 后 hang → 不得成功 JSON。  
   - 收紧 AC7：Raw 首 content hang 不得立刻回截断 JSON（仍等 interval / 现有 `usable()` 妥协）。  
   - 官方 S2 / `shouldForceSyncInboundUpstreamSSE` 期望不变。

6. **CHANGELOG**  
   验证通过后往 `docs/dev/CHANGELOG_CUSTOM.md` 追加一条（`git add -f` 仅当 Brandon 要求提交时）。

## Test commands

在 `backend/`：

```text
go test -tags=unit ./internal/service -run "TestHandleChatBuffered|TestShouldForceSyncInboundUpstreamSSE|TestBufferRawChatCompletions|TestAccountHasCustomOpenAIBaseURL|TestForwardAsChatCompletions_" -count=1
```

AC 对照：

| AC | 测试 |
|---|---|
| AC1 | `TestHandleChatBufferedStreamingResponse_TimeoutAfterCompletedReturnsJSON`（时限远小于 1s） |
| AC2 | `TestHandleChatBufferedStreamingResponse_IntervalTimeoutFailovers` |
| AC3 | `TestHandleChatBufferedStreamingResponse_HTTP2PeerResetFailovers` |
| AC4 | `TestHandleChatBufferedStreamingResponse_PacedSSESurvivesIntervalTimeout` |
| AC5 | 新增 failed 终态不得成功 JSON |
| AC6 | `TestShouldForceSyncInboundUpstreamSSE` / `TestAccountHasCustomOpenAIBaseURL` / `TestForwardAsChatCompletions_*` |
| AC7 | `TestBufferRawChatCompletionsFromSSE_TimeoutAfterUsableReturnsJSON`（不得立刻返回） |
| AC8 | 新增：仅 created / in_progress / delta 后 hang 不得成功 JSON |

## Rollback

只改缓冲循环退出点 + 对应单测。回退即恢复「写完 `finalResponse` 后继续读，等 EOF / interval」。  
`openai_sync_inbound_upstream_sse_mode`、Raw `usable()`、官方 S2 均不动。

## Done when

上表命令全绿；completed 后 hang 不再贴近 1s interval；无终态 H2 / interval 仍 failover；failed / 半成品不写成功 JSON；Raw 首 token hang 仍等 interval。
