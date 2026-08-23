# Design

## Boundaries

| 在范围内 | 不在范围内 |
|---|---|
| `ForwardAsAnthropic` API Key 裁剪/续链 | 换组 / 关桥 / 改 mapping |
| `openai_messages_continuation.go` latest-turn | OAuth Codex WS 续链主路径 |
| `openai_messages_replay_guard.go` 12 条默认 | 展示 cache 百分比算法 |
| 桥接流无可见输出 / 客户端 cancel 收口 | 客户端改协议 |
| Info 日志带 request_id + input_item_count | 整文件换上游实现 |

## Current data flow（缺陷）

```text
Desktop 200KB Anthropic messages
  -> 选号 1685 (API Key, gpt-5.5)
  -> previousResponseID = 内存[1685, 317, promptCacheKey]
  -> 跳过 12 条 guard（因为 id 非空）
  -> AnthropicToResponses: store=false, system→developer
  -> trim to latest turn   // 200KB → 2 或 3 条 input
  -> 插入 todo-guard developer
  -> HTTP /v1/responses stream=true, WithoutCancel
  -> 上游按残缺 input 补全或空转
```

生产已观测：d54f649c=2，d7005e89=3，e47e796f=3。

## Proposed data flow

```text
Desktop 200KB Anthropic messages
  -> 选号不变
  -> API Key: 不读、不写 previous_response_id 到上游 body
  -> 不跑 12 条 guard
  -> AnthropicToResponses 保持 store=false
  -> input = 完整转换（system/developer + 全部 messages + tools）
  -> todo-guard 仍可插，不得删历史
  -> 原样 HTTP 转发（与直打 OAI 同一信息量）
```

`store=false` 是现网 API Key 兼容上游的既有选择。本设计不靠上游 store 做对话记忆；记忆只来自每次请求的 full replay。这与「同一 Key 直打 OAI 为什么正常」一致。

## 改动点

### D1 续链门闩（R1/R2/R3）

在 `ForwardAsAnthropic` 里把 API Key 的两条裁剪收成一个函数，语义固定：

```text
func anthropicAPIKeyMustFullReplay(account, storeFlag, previousResponseID) bool
  API Key → true（本任务）
```

当 full replay：

- 不调用 `applyAnthropicCompatFullReplayGuard`
- 不调用 `trimAnthropicCompatResponsesInputToLatestTurn`
- 不把内存 id 写入 `responsesReq.PreviousResponseID`
- 仍可 `bindOpenAICompatSessionResponseID` 供以后 OAuth/显式 store 使用，但 API Key HTTP 桥不得读它来裁 input

不要「先裁再在 400 时重试」。现网失忆是 200，重试进不去。

### D2 日志（R4）

把现有 Debug 的 `compat_full_replay_trimmed` / `compat_previous_response_id_attached` 升到 Info，并带上 gin `request_id`、`input_item_count`、`anthropic_message_count`、`store`。`logClaudeGPTBridgeUpstreamRequest` 已有 `input_item_count`，补 `request_id`。

### D3 断流收口（R5）

与 D1 分开提交也可以，但必须同一任务交付：

1. 桥接流：`!clientVisibleOutputStarted` 且上游结束 / interval 到 / 客户端 cancel → 写 Anthropic error SSE（或尚未写头时写 JSON error），返回 error，禁止中间件只记 200 成功无 usage。
2. 客户端 cancel：流读取跟 `c.Request.Context()`，不要对 messages 桥接再用 `WithoutCancel` 把 cancel 丢掉。上游 `Do` 的 header 等待可以 detach，body 读取必须能被 cancel 打断。
3. interval：`lastReadAt` 只在「有 Responses 数据帧」时刷新，不在空行 / 注释上刷新。keepalive ping 不刷新 interval，也不算可见输出。
4. 已写 200 ping 后失败：必须再写 `event: error`，不能只 `return`。

## Compatibility

- 直连 chat completions / 原生 responses：不进这些函数，零变化。
- OAuth Anthropic→Codex：`Type==OAuth` 本来就不走 12 条 guard；不要改 OAuth 的 turn-state / WS sticky。
- 计费：full replay 会让 raw input 回到真实对话大小。展示 cache 仍按现算法吃 raw input。这是正确方向（现在 27k raw 主要是 tools schema，不是对话）。
- 超长对话：客户端自己会 compact。我们不得先砍到 2～3 条。真正超窗走现有 `isOpenAIMessagesContextWindowError` / compact recovery，那是另一条已有路径。

## Tests（必须先写失败用例再改）

用生产形状，不写客户正文：

1. API Key + gpt-5.5 + 20 条 messages + 末条 tool_result + 内存 previous_response_id → 转发 input 含第 1 条 user 文本和第 19 条 assistant，条数 >> 3，body 无 `previous_response_id`，`store=false`。
2. API Key + 上一跳 tool_result 含「条款 2/3」+ 本跳 user「第 2、3 条怎么改」→ 转发 input 同时含条款原文和问句。
3. API Key + 无 previous_response_id + 15 条 messages → 不再被裁成 12。
4. OAuth 账号：既有 continuation 单测不回退。
5. 桥接流：上游只发 ping/空行 超过 interval → error，不是 200 空成功。
6. 桥接流：客户端 cancel → `Do`/read 结束，不把测试时钟拖满。

## Rollback

只回滚本任务在占用 `main` 上对 messages 续链与流收口的改动。不改 mapping、不改 group 8。
