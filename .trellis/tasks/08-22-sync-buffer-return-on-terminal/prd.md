# 同步缓冲拿到终态立刻返回

## Goal

入站 `stream:false`、上游 SSE 再本地缓冲时：一旦已经拿到**可用终态**，立刻组装一个 Chat Completions JSON 给下游，不再空等 HTTP body EOF，也不再空等 `StreamDataIntervalTimeout`（默认 180s）静默。

客户端契约不变：仍回一个 JSON，不改 `stream` / 端点 / body。

用户价值：砍掉「答案已经齐了、网关还在等连接关」的尾巴。近 24h 生产上这是 0.4% 长尾（≥170s 平均 507s），不是主体 35s。

## Background

2026-08-22 生产取证：`gpt-5.6-sol` 对齐 output token 后，同步与流式 p50 几乎重合（约 1700 token 都是 ~36s）。主体慢来自流量结构，不是缓冲税。

真正还能动、且不改客户端协议的，是缓冲循环在**已有终态之后**仍继续读 body。

## Confirmed Facts

### 路径

- 官方 / 空 `base_url`：S2（上游 `stream=false` JSON）。**本任务不碰。**
- 自定义中转 + API Key 同步：上游 SSE，`handleChatBufferedStreamingResponse`（CC→Responses）或 `bufferRawChatCompletionsFromSSE`（raw CC）。
- 近 24h `gpt-5.6-sol` CC 几乎全是 `apikey_custom`（28221 同步），走 SSE 缓冲。
- OAuth 变换仍强制上游 `stream=true`，同步入站也走缓冲。

### 当前退出条件（Responses 缓冲）

`backend/internal/service/openai_gateway_chat_completions.go` `handleChatBufferedStreamingResponse`：

- 读到 `response.completed` / `done` / `incomplete` / `failed` 且带 `response` 时，只写入 `finalResponse`，**不返回**。
- 真正返回要等：scanner EOF（`!ok`）、读出错且已有终态、或静默满 `StreamDataIntervalTimeout` 且 `chatBufferedHasUsableTerminal`。
- `chatBufferedHasUsableTerminal`：`status != "failed"`。`failed` 走 failover / 502，不是成功 JSON。
- `lastReadAt` 在每一行 `Scan()` 刷新，含空行、`:` keepalive、`data: [DONE]`。keepalive 会让 180s 静默永远凑不齐。

Raw CC `bufferRawChatCompletionsFromSSE` 同一套循环，但「可用」是 `rawChatSSEAccumulator.usable()`：`hasMessage || finishReason || usage || toolCalls`。**第一个 content delta 就为真**，不是终态。

### 截断风险（本轮约束）

「立刻返回」若绑错谓词，会把半成品当成功 JSON。用户视此为**新 bug 风险**，优先于再砍 180s 尾巴。

| 路径 | 安全谓词（真终态） | 危险谓词（会截断） |
|---|---|---|
| CC→Responses | 已 `ProcessEvent` 的 `response.completed` / `done` / `incomplete`，且带 `response`、`status != failed` | 任何「有 delta / 有半个 response 对象」 |
| Raw CC | `finish_reason != ""` 或 `data: [DONE]`（工具调用还要 arguments 齐） | 现行 `usable()`（首 token 即为真） |

本轮 **只允许** Responses 那条安全谓词。禁止把 Raw `usable()` 升级成立刻返回。禁止发明「看起来齐了」的 fork-only 启发式。

### 上游（先查再改）

`Wei-Shaw/sub2api` @ `d45135d87`（2026-08-22 fetch）：CC→Responses 缓冲已在 `readOpenAICompatBufferedTerminal` 上对真终态立刻 `return`（见 `research/upstream-sync-buffer.md`）。上游 **没有** Raw CC SSE 缓冲。本仓应 **adopt/adapt** 该真终态返回，而不是另写一套启发式；官方 S2 与无终态 H2/180s failover **保持**，禁止整文件换上游。

### 2026-08-18 那次「76s 挂死」是什么

生产 `gpt-5.6-sol` CC→Responses 同步 p50 ~76s（原生 Responses 流式 ~9.6s）。根因：

1. 当时 **API Key 同步也一律向上游要 SSE 再缓冲**（含官方）。
2. 中转 HTTP/2 `INTERNAL_ERROR` 后 **body 不关**。
3. 缓冲循环 **没有可用终态**，一直读到连接死。
4. 没有「无终态则 fail-fast 换号」。

修法（已在 main，本任务不得撤回）：

| 护栏 | 作用 | 本任务 |
|---|---|---|
| 官方 / 空 base_url 走 S2 | 官方不再 SSE 缓冲 | 保持 |
| 无终态 + H2 peer reset → `UpstreamFailoverError` | 不写 502、换号 | 保持 |
| 无终态 + 180s 静默 → failover | 防「等到连接死」 | 保持 |
| 已有 completed、之后连接挂了 → 回 JSON 不换号 | `TimeoutAfterCompletedReturnsJSON` | **提前到一收到终态就回**，不是删掉这条 |

文档原话：`a completed terminal already in hand is returned as JSON even if the connection later hangs`。实现却是：有 completed 也不马上回，先继续读；只有再静默满 180s 才用手里的 JSON。

### 生产尾巴

近 24h CC 同步 `gpt-5.6-sol`：`duration ≥ 170s` 129 条（0.4%），平均 507s，最长约 15 分钟。180s 上限解释不了 507s，符合「completed 后还有 keepalive / 空行，`lastReadAt` 被刷新，静默超时不触发」。

## Requirements

- **R1** CC→Responses 缓冲：读到可用终态（`completed` / `done` / `incomplete`，且 `status != failed`）后，立刻 `finishChatCompletionsFromResponsesResponse`。不再等 EOF，不再等 180s 静默。
- **R2** 无可用终态时行为不变：H2 peer reset → failover；180s 静默 → failover；不写 502。
- **R3** `response.failed` 仍走现有 failed / cyber / failover，不得当成功 JSON。
- **R4** 官方 S2、入站 `stream:true`、客户端契约（一个 JSON）一律不动。
- **R5** `StreamDataIntervalTimeout=0`（测试同步 scanner）在读到可用终态后也应立刻 finish，不必读到 EOF。
- **R6** 已在 channel 里、终态之前的事件仍要 `ProcessEvent`（含终态当行）。终态之后已入队的 `[DONE]` / keepalive 可丢。不要求再从网上多读一行。
- **R7** 本轮默认 **不做** Raw CC 立刻返回。上游也没有这条路径。若以后做，触发条件必须是 **CC 终态**（`finish_reason` 或 `[DONE]`），**禁止**用现行 `usable()`（首 token 即为真）。
- **R8** 优先采用/适配上游已有的真终态立刻返回（`isOpenAICompatResponsesTerminalEvent` + `event.Response != nil` 的本仓四类子集）。禁止为砍尾巴发明会截断的启发式。
- **R9** 立刻返回的成功 JSON 必须含终态当行已 `ProcessEvent` 的内容 + completed 上的 usage；不得在首个 delta / `in_progress` / 空 `created` 上返回。

## Acceptance Criteria

- [x] **AC1** 单测：`completed` 后 body hang，在远小于 interval（默认测 1s 也应远小于）内返回成功 JSON，且含 completed 上的 usage。收紧现有 `TestHandleChatBufferedStreamingResponse_TimeoutAfterCompletedReturnsJSON`（今日上限是 3s / 等 interval）。
- [x] **AC2** 单测：无终态 hang 仍在 interval 内 failover，body 为空。`IntervalTimeoutFailovers` 保持。
- [x] **AC3** 单测：无终态 H2 `INTERNAL_ERROR` 仍 failover。`HTTP2PeerResetFailovers` 保持。
- [x] **AC4** 单测：completed 后仍有 paced `[DONE]` 的正常流，仍成功（`PacedSSESurvivesIntervalTimeout`）。
- [x] **AC5** 单测：`status=failed` 的终态不得当成功 JSON 立刻返回。
- [x] **AC6** 官方 S2 / `shouldForceSyncInboundUpstreamSSE` 路由测试不改期望。
- [x] **AC7** 本轮 Raw CC 行为不变：首个 content delta 后 hang **不得**立刻返回截断 JSON；仍走今日 `usable()`+interval 妥协，或无终态时 interval/H2 failover。
- [x] **AC8** 立刻返回谓词不得宽于本仓已识别的 Responses 终态四类 + `event.Response != nil` + `status != failed`（成功路径）。单测：只有 `output_text.delta` / `response.created` / `in_progress` 时 hang → 不得成功 JSON。

## Out of Scope

- 给同步补 `first_token_ms`（另一件事）。
- 改 `openai_sync_inbound_upstream_sse_mode` 默认、或让官方改走 SSE。
- 请客户端改 `stream` / 端点 / body。
- 削主体 35s（那是生成时间）。
- 改 180s 默认值本身（无终态护栏保留）。

## Open Questions

- **Q1（本轮已定，不再阻塞 Responses）** Raw CC 缓冲要不要一起改？  
  **默认：不做。** 上游 `main` @ `d45135d87` 没有 `bufferRawChatCompletionsFromSSE`，也就没有「真终态立刻返回」可拍。本仓 `usable()` = 首 delta，立刻返回会截断。仅当上游日后用 `finish_reason` / `[DONE]`（**绝不是** `usable()`）修了 Raw，再评估 adapt。Responses 缓冲按 R1/R8 单独做。
