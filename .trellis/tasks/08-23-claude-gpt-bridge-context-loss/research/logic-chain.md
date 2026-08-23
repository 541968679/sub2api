# 逻辑链（只写已证事实；未证单独标出）

调查日期：2026-08-23。生产镜像 `ghcr.io/541968679/sub2api:0.1.252`，healthy。客户：user 209 / key 317 / group 8。不要把 user 12 / key 133 / group 15 算进本客户。

## 0. 症状与对照

| 症状 | 客户侧事实 | 生产对上的请求 |
|---|---|---|
| 失忆 / 空开场 | 同一 Desktop 会话里刚问过中文问题，下一句变成 `How can I help you today?` / `What would you like me to work on?` / `How can I help with the codebase?` / `How can I help?` | `client:d7005e89-c736-4a3b-9369-abd060360414`（下称 d7005e89），2026-08-23 00:22:38+8 |
| 失忆 / 刚读过的条文消失 | Read 成功全局 `CLAUDE.md` 后，用户问「第 2、3 条应该怎么改」，回复「我這裡看不到你說的第 2、3 條」 | `client:d54f649c-b722-4ce6-a880-348eeadd02cc`（下称 d54f649c），2026-08-22 23:48:03+8 |
| 断流 | Desktop「思考中」后只剩灰色 `***Interrupted***`，无 HTTP/上游错误体 | `client:e47e796f-8906-436c-8c38-e1abc94a5d79`（下称 e47e796f），2026-08-23 12:43:20+8 起，12:58:20 结束 |

Brandon 已排除：中途换模型不是根因；同一批第三方 Key 直打 OAI `/v1/responses` 或 `/v1/chat/completions` 不出现失忆；换组/关桥不是本任务交付。

`cache_read` 不能当「历史已送达」的证据。生产设置 `openai_claude_gpt_bridge_cache_display_settings` 为 `{enabled:true, min_percent:65, max_percent:95}`。实现：`applyClaudeGPTBridgeDisplayCacheOverride`（`openai_gateway_messages.go:217-292`）。d7005e89：`raw_input=27498`，`display_cached=18459`（67.13%），`upstream_cached_tokens=0`。

## 1. 入站路径（三笔请求同一条）

```text
Desktop 3p POST /antigravity/v1/messages stream=true
  -> ClaudeGPTBridgeRoute ready
  -> terminal_outcome=dispatch_bridge
  -> MessagesClaudeGPTBridge
  -> ForwardAsAnthropic
```

三笔都是：user 209、key 317、group 8、`dispatch_bridge`、账号 **1685**、上游 **gpt-5.5**、入站 Anthropic body 完整（不是客户端自己只发最后一轮）。

| 请求 | 入站 body_bytes | 审核 | 转发后 input_item_count | 转发 body_bytes | tool_count | 结果 |
|---|---:|---|---:|---:|---:|---|
| d54f649c 23:48:03–17 | 207530 | allow | **2** | 117960 | 59 | 200 / 14790ms / output=90，「看不到第 2、3 条」 |
| d7005e89 00:22:38 | 216570 | allow | **3** | 117986 | 59 | 200 / 3433ms / output=39（空开场） |
| e47e796f 12:43:20 | 342059 | allow | **3** | 141039 | 60 | 200 / **900407ms** / 无 usage stored |

d54f649c 与 d7005e89 的 `body_prompt_cache_key_sha256` 都是 `ea3213372ca10a3a`，`header_session_id_sha256` 都是 `884eebe86fc8f6a1`。这是同一条 Desktop 会话在网关里的续链键，不是两条无关请求。

直连 OAI 的 `/v1/chat/completions` 与原生 `/v1/responses` **不调用**下面两个函数（全仓引用只在 `openai_gateway_messages.go`）。

## 2. 桥接在转发前做了什么（代码布尔条件）

`ForwardAsAnthropic`（`openai_gateway_messages.go:381-448`）：

```text
compatReplayGuardEnabled  = shouldAutoInjectPromptCacheKeyForCompat(upstreamModel)
                         // 模型名含 gpt-5 或 codex → true（openai_compat_prompt_cache_key.go:14-23）
compatContinuationEnabled = account.Type==APIKey && 同上
                         // openai_messages_continuation.go:23-28
previousResponseID        = 内存 map[accountID, apiKeyID, promptCacheKey]
compatContinuationDisabled= 该 key 被标过 unsupported

若 compatReplayGuardEnabled && Type!=OAuth && previousResponseID=="" && !disabled:
    applyAnthropicCompatFullReplayGuard  // 只留最近 12 条 Anthropic messages

AnthropicToResponses: 永远 Store=false（anthropic_to_responses.go:40-41）
                      system → input.developer，不是 instructions

若 previousResponseID != "":
    写入 previous_response_id
    trimAnthropicCompatResponsesInputToLatestTurn  // 只留当前跳

若 API Key && gpt-5/codex:
    appendOpenAICompatClaudeCodeTodoGuard  // 再插一条 developer
```

1685 是 API Key，上游 gpt-5.5。这两条裁剪对它是硬开启，不是配置开关。

`AnthropicToResponses` 之后，API Key 路径只补 `prompt_cache_key`，**不把 Store 改成 true**（`openai_gateway_messages.go:561-581`）。发出去的 Responses 体是：`store=false` + 可能带 `previous_response_id` + 被裁过的 `input`。

12 条窗口与 latest-turn **互斥**：有 `previousResponseID` 时不走 12 条，走 latest-turn。

## 3. `input_item_count` 直接选出分支

`logClaudeGPTBridgeUpstreamRequest`（`openai_gateway_messages.go:148-169`）打的是 **裁剪之后** 的 Responses `input` 条数。

12 条窗口假说：入站 200KB+ 的 Desktop messages（jsonl 重建：23:48 已合并 41 条，00:22 已合并 64 条）转成 Responses 后，再加 system→developer，`input_item_count` 远大于 2 或 3。

生产观测：

- d54f649c：`input_item_count=2`
- d7005e89：`input_item_count=3`
- e47e796f：`input_item_count=3`

这三笔都是 **latest-turn**，不是 12 条窗口。

`latestAnthropicCompatResponsesInputTurnStart`（`openai_messages_continuation.go:50-71`）：

- 末项是 `function_call_output`：只留连续的 output，再向前补匹配的 `function_call`。上一轮 user 文本、更早的 Read 结果、system/developer 全部丢掉。
- 末项是 `role=user` 的 message：只留这一条 user（以及紧挨着的 output）。上一轮 assistant 读到的文件内容丢掉。

之后 `appendOpenAICompatClaudeCodeTodoGuard` 再插 1 条 developer。因此：

| 观测条数 | 对应 latest-turn 形状 | 模型实际看到的对话 |
|---|---|---|
| 2 | todo-guard + 当前 user 文本 | 有这句问句，没有刚读过的文件 |
| 3 | todo-guard + 当前工具跳（output / call+output） | 没有这句问句，只有 Glob/Task 结果 |

jsonl 重建与这两档对齐，但 **条数以生产 `input_item_count` 为准**，不以 jsonl 当 HTTP body。

## 4. 失忆因果（闭合）

### 4.1 「看不到第 2、3 条」= 2 条 input

时间序（客户端会话 `3bb8c883-689a-4b65-b60e-0e6361e5e326`，cwd `D:\A Projects\Aclaude`）：

1. 23:44:51 绝对路径 Read `C:\Users\user\.claude\CLAUDE.md` 成功（磁盘/权限成立）。
2. 23:48:03 生产 d54f649c：入站 207530 字节，转出 **2** 条 input。
3. 助手回复：看不到第 2、3 条。

2 条 = 当前 user 问句 + todo-guard。Read 的 `tool_result` 不在这 2 条里。模型没有条文原文，只能说「看不到」。这不是第三方「丢记忆」，是我们把记忆从 `input` 里删了。

同一秒窗口没有 `openai messages: previous_response_id unavailable, retrying without continuation`（该日志是 Info）。上游接受了这具残缺 body，返回 200。

### 4.2 空开场 = 3 条 input

00:22:38 d7005e89：入站 216570 字节，转出 **3** 条 input。jsonl 上这一跳的末条是双重 Glob 的 `No files found`。3 条 = todo-guard + 工具结果，没有「为什么找不到 / 你没有回答我」那句 user 文本。

3.4 秒后 200，`output_tokens=39`，客户端记下 `What would you like me to work on?` / 随后 `How can I help?`。

`upstream_cached_tokens=0`：上游没有可用的 cached prefix。`store=false` 时官方 Responses 也不能靠 `previous_response_id` 回放上一轮。我们仍然按「上一轮已存」来裁 `input`。内存里的 `previous_response_id` 只对网关自己有意义，对这路上游没有对应 store。

### 4.3 12 条窗口不是这两次失忆的机制

`applyAnthropicCompatFullReplayGuard` 在 `previousResponseID==""` 时才会跑。这两笔的 `input_item_count` 已经排除 12 条窗口。

12 条窗口仍是 API Key + gpt-5 在 **续链键未命中**（换号、进程重启、TTL 过期、unsupported 禁用后续链）时的第二条缺陷。默认 TTL 1 小时（`openAIWSResponseStickyTTL`，`openai_ws_forwarder.go:961-968`）。本客户这两次失忆没有走到它。修复时必须一起拿掉，否则换号后会换成「只剩 12 条」的另一种失忆。

### 4.4 Glob `No files found` 不是网关删文件

cwd 是项目目录，文件在 `C:\Users\user\.claude\CLAUDE.md`。绝对路径 Read 成功。这是工具选错路径，不是本任务根因。它只提供了「工具跳」这个 latest-turn 入口。

### 4.5 为什么直打 OAI 正常

直打走 `ForwardAsChatCompletions` / 原生 Responses，不执行 `trimAnthropicCompatResponsesInputToLatestTurn`，也不执行 12 条 guard。客户端自己带的 messages/input 原样转发。同一把第三方 Key 因此正常。问题边界是 **本仓 Claude→GPT 桥**，不是 Key、不是模型映射表本身。

## 5. 断流因果（与失忆同一裁剪，收口是第二条缺陷）

e47e796f：12:43:20.598 同样 `input_item_count=3`，账号 1685，gpt-5.5。上一跳 `78f1d950` 刚成功（200，7s，212 out，Read+TaskGet）。这一跳入站 342059 字节，转出 141039 字节 / 3 条 input。

12:43:20.504 审核结束 → 12:58:20.727 HTTP 200，`latency_ms=900407`。该 `request_id=a08bc716-...` 在现网日志里 **没有**：

- `openai_messages.stream_completed_without_visible_output`
- `openai messages stream: data interval timeout`
- `openai_messages.forward_failed`
- `openai claude-gpt bridge usage stored`

客户端 12:51:44 出现全包唯一 `cli_streaming_idle_warning`；12:54:07 `LocalSessions.interrupt`，UI `***Interrupted***`。网关继续跑到 12:58:20。

代码允许这个收口：

1. 上游始终 `Stream=true`（`openai_gateway_messages.go:428-431`）。
2. 流式请求用 `context.WithoutCancel`（`detachStreamUpstreamContext`，`gateway_service.go:8668-8676`；`ForwardAsAnthropic` 在 `openai_gateway_messages.go:626` 调用）。客户端 interrupt **取消不了**上游 `Do()` / body 读取。
3. `http.Client` 无总 Timeout（`http_upstream.go:480` 只设 Transport）。
4. 生产 `/app/data/config.yaml` **没有** `stream_data_interval_timeout` / `stream_keepalive_interval` / `response_header_timeout`。Viper 默认：interval 180s、keepalive 10s、header 600s。
5. keepalive 每 10s `writeStreamHeaders()` + Anthropic `ping`（`openai_gateway_messages.go:1895-1900`）。这会把入站 HTTP **先写成 200**。
6. interval 只看 `scanner` 是否超过 180s 没有新行。空行 / 注释 / 上游 SSE 都会 `StoreInt64(lastReadAt)`（`openai_gateway_messages.go:1814-1815`）。该请求没有 interval WARN ⇒ 180s 守卫没有打断它。
7. 若最终 `err != nil` 且 `openAIClientRequestCanceled`（handler `openai_gateway_handler.go:1082-1084`），只打 Debug，不打 `forward_failed`，中间件仍记 200。

**900407ms 的切断源不在 messages HTTP 代码里。** 本路径没有 900s 常量。`gateway.openai_ws.read_timeout_seconds` 默认 900，但 `ForwardAsAnthropic` 不走 WS。不能把 15 分钟写成「我们设了 900s」。能写的是：我们没有总超时、interval 没开火、客户端取消被 detach，连接被上游或链路其它层在约 15 分钟时结束。

断流与失忆的共同前置：先把 342KB 对话裁成 3 条 input。上游拿着空对话 + 60 个 tool 开长流；Desktop 看不到可见块，只能等，然后 interrupt。

## 6. 明确排除

| 假说 | 为何排除 |
|---|---|
| 第三方不支持 `previous_response_id` 所以失忆 | 失忆请求是 200 短补全，不是 400。重试 Info 未出现。直打同一 Key 正常。残缺的是我们裁过的 `input`。 |
| `cache_read=18459` 证明历史在 | 人造展示 cache；`upstream_cached_tokens=0`。 |
| 换模型 | Brandon 排除；换完后下一轮已工作。 |
| 会话被清空 | Desktop jsonl 连续；入站 body 20万+ 字节。 |
| 网关注入问候语 | output=39 来自 gpt-5.5 补全。 |
| auto-compact 压掉历史 | `maybeAutoCompactAnthropicBridge` 要求 `account.Type==OAuth`（`openai_anthropic_bridge_compact.go:97-100`）。1685 是 API Key。生产也无 compact enabled 覆盖。 |
| 12 条窗口造成 23:48 / 00:22 / 12:43 | `input_item_count` 为 2 或 3，与 12 条转换后的 item 数矛盾。 |
| user 12 / GY-Claude | 另一客户。 |

## 7. 未证（禁止用猜测填）

- e47e796f 在 15 分钟里上游 SSE 的具体 event 类型（现网没有该 request 的 stream_debug）。
- 谁在 900s 切断 TCP（上游、反代、CF）。
- 3 条 input 的精确三项 JSON（日志只记 count）。按代码，形状只能是 todo-guard + 当前跳，不能是 12 条历史。
- 内存 `previous_response_id` 的具体字符串（trim 标志是 Debug，现网 INFO 看不到）。`input_item_count` 已足够证明 latest-turn 已执行，该执行的前置就是 `previousResponseID != ""`。
