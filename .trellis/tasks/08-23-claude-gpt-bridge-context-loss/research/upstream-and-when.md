# 交叉核验：上游讨论 × 为何不每轮发作

## 1. 上游官方桥就是我们这套代码

本仓的 `applyAnthropicCompatFullReplayGuard` / `trimAnthropicCompatResponsesInputToLatestTurn` / `previous_response_id` 内存续链，来自上游提交，不是二开独创。

| 项 | 值 |
|---|---|
| 引入提交 | `0584305e5` 2026-05-05 `feat: improve OpenAI messages compatibility for Claude Code`（lyen1688） |
| 对应 PR | https://github.com/Wei-Shaw/sub2api/pull/2204 已合并 |
| 本仓合入 | `1e5ebe4e2` 2026-06-06 `fix: sync upstream openai messages bridge core` |
| 现网 `upstream/main` | 同一套门闩仍在：`previousResponseID != ""` → 写 id + latest-turn；无 id 且非 Codex 协议 → 12 条窗口 |

PR 2204 自己写的设计目标（原文）：

- 核心目标是 Claude Code 走 `/v1/messages` 调 GPT 时「更稳定的 prompt cache/session 复用、更可靠的工具调用续链」。
- API Key：上一轮成功拿到 `response.id` 后，同一 `prompt_cache_key` 下一轮自动带 `previous_response_id`。
- 若上游返回 `previous_response_not_found`：删锚点，replay 重试。
- 若上游明确不支持 `previous_response_id`（含 “only supported on Responses WebSocket v2”）：**禁用 continuation，后续用完整 replay + 稳定 prompt_cache_key，避免反复 400**。
- OAuth/Plus：**不**往 body 注入 `previous_response_id`，靠 `session_id` / `x-codex-turn-state`。
- 明确写了：不改变普通 `/v1/responses`、Codex 原生、compact、WS。

对应代码：`openai_gateway_messages.go:763-774` 只在 HTTP **400/404** 且错误文案匹配时才重试。我们三笔客户请求都是 **200**，这条后路没进来。

结论（这一条的可靠度：高）：上游承认「`previous_response_id` 用不了就该 full replay」。他们赌的是上游会 400。第三方 200 + 忽略该字段时，设计假设不成立，残缺 `input` 会直接生成。

## 2. 上游有没有「失忆 / 断流」原文讨论

按议题全文检索（2026-08-23）：

**没有**找到标题或正文直接写 Desktop「How can I help」/「看不到刚才的条文」/「***Interrupted***」的议题。不能写成「上游已经报过同一个 UI 症状」。

找到的是**同一条裁剪路径**上的相邻故障，以及官方自己的修补选择。

### 2.1 #2337 + 提交 `87d73236f`（同一条 latest-turn）

https://github.com/Wei-Shaw/sub2api/issues/2337  
`/v1/messages` → OpenAI，多轮 tool 后 400：`No tool call found for function call output with call_id`。

他们自己的触发表：

| 操作 | 工具轮次 | 是否报错 |
|---|---|---|
| 简单 grep | 0–1 轮 | 正常 |
| 读整个文件 | 2 轮以上 | 400 |

根因他们写成「tool_use_id 映射错」。真正落地的补丁 `87d73236f`（Related: #2337）写的是：latest-turn **把 `function_call_output` 留下、把对应的 `function_call` 裁掉了**。补丁只补回 matching `function_call`。

该提交明确 **Rejected**：`Disable previous_response_id for all tool outputs`，理由是「会失去 continuation 和 cache」。

这证明三件事：

1. 上游已经观察到「不是每轮都坏，工具轮次变多才坏」——和客户现象同类。
2. 他们承认 latest-turn 会丢掉当前跳必需的上下文。
3. 他们拒绝关掉 `previous_response_id`，只补 call_id。我们客户的 200 + 2/3 条 input，是这条补丁之后**仍然丢掉 user 文本 / 上一跳 Read 原文**的下一层。#2337 的 400 被第三方 200 换了一种临床表现。

### 2.2 #2204 的 400 后路 vs 我们的 200

PR 写明：不支持就禁用 continuation，走完整 replay。  
现网 d54f649c / d7005e89 / e47e796f：无 `previous_response_id unavailable, retrying` Info，HTTP 200。  
`upstream_cached_tokens=0`（d7005e89）。store 始终 false。  
续链字段被带上，缓存/store 都没接住，input 已被裁。

### 2.3 其它相邻议题（不是本症状的复件）

| 编号 | 说了什么 | 和本结论的关系 |
|---|---|---|
| [PR 2204](https://github.com/Wei-Shaw/sub2api/pull/2204) | 为 cache/续链引入这套机制 | 机制源头 |
| [#2337](https://github.com/Wei-Shaw/sub2api/issues/2337) | 工具多轮后 400，非每轮 | 同一 trim，不同出口 |
| [#2133](https://github.com/Wei-Shaw/sub2api/issues/2133) / [#1957](https://github.com/Wei-Shaw/sub2api/issues/1957) | HTTP `/v1/responses` 拒 `previous_response_id`；Lingtai 改为每轮重放完整历史 | 旁证：官方 HTTP 续链本身就不稳，完整重放是已知退路 |
| [#1558](https://github.com/Wei-Shaw/sub2api/issues/1558) | legacy 删 `previous_response_id` + 强制 `store=false` 导致长会话只能全量重放 | 方向相反的病（删 id）；共同点是 store=false 时不能假装有续链 |
| [#5237](https://github.com/Wei-Shaw/sub2api/issues/5237) | CC 经 sub2api 接 GPT「一开始处理就结束 / 不可用」；同一模型 CC switch 直连正常 | 相邻；无 `input_item_count`，不能当成我们的 Desktop 3p 复件 |
| [#1552](https://github.com/Wei-Shaw/sub2api/issues/1552) / [#4742](https://github.com/Wei-Shaw/sub2api/issues/4742) | stream 无 terminal / 上下文长了断流 | 断流家族，不是 2/3 条 input 的证明 |
| [#996](https://github.com/Wei-Shaw/sub2api/issues/996) / [#3060](https://github.com/Wei-Shaw/sub2api/issues/3060) | 切回 Claude thinking 块；`max_output_tokens` | 无关，排除 |

没有上游议题要求「关掉 API Key latest-turn」。我们的修复等于走 PR 2204 已经写明的 **unsupported → 完整 replay** 分支，只是改成：第三方 200 时也走，不等 400。

## 3. 为什么不是每次对话、每一轮都发作

裁剪在「续链键命中」之后是**每轮都执行**的。用户看见的失忆/断流不是每轮都出现，因为可见症状只在「被裁掉的东西正好是这一跳必需的」时出现。

### 3.1 状态机（代码，不是推测）

键：`accountID \x00 apiKeyID \x00 promptCacheKey`（`openai_messages_continuation.go:144-157`）。  
绑定：本轮成功且有 `result.ResponseID`（`openai_gateway_messages.go:848-849`）。  
TTL：默认 1 小时（`openAIWSResponseStickyTTL`）。内存 map，进程重启清空。

```text
本轮 previousResponseID = map[1685, 317, sessionKey]

空 且 messages≤12 且未禁用续链:
    不裁  → 模型看见整段  → 这一跳看起来正常
空 且 messages>12 且未禁用:
    12 条窗口 → 最近一句 user 通常还在 → 多数自问自答仍像正常
非空:
    latest-turn → 生产已测 2 或 3 条 input
    末条是 user 文本: 2 = guard + 问句（丢掉 Read 原文）
    末条是 tool_result: 3 = guard + 工具跳（丢掉问句）
```

续链键为空的充要条件（代码）：

1. 这条 `(账号, Key, session)` 还没成功 bind 过（新对话前几轮、进程刚重启）。
2. 换号：1685→1686，accountID 变了。
3. TTL 过期或 bind 被删（400 not_found）。
4. 上游曾回 unsupported，`ContinuationDisabled=true`（此后反而是 full replay）。
5. `promptCacheKey` 变了（session/metadata 变）。

### 3.2 客户时间线对得上「工具跳 / 指代跳才露馅」

Aclaude 同一会话（jsonl + 生产）：

- 23:20 之前：多轮中文问答（「重新启动是不是新对话」这类**问句本身够用**的题）——即使已经 latest-turn，2 条里仍有当前问句，看起来正常。
- 23:20 第一次 Glob 回 `No files found` → 空开场。这是第一次「当前跳只剩工具结果」。
- 23:37 / 23:43 / 00:19 / 00:22：同样在 Glob/TaskList 结果跳上问候。
- 23:48:03–17 d54f649c：`input_item_count=2`，问「第 2、3 条怎么改」，90 token「看不到」。这是「问句还在、指代对象被裁掉」。

漫畫分鏡中午：

- 12:43:20 前一跳 `78f1d950` 成功（212 out）。
- 紧接着 e47e796f 仍是 1685、`input_item_count=3`，空转 900s。不是「3 条就会断流」：d7005e89 同样 3 条，3.4s 就问候完毕。断流 = 3 条裁剪 **加上** 上游这一跳没吐可见块。

### 3.3 和 #2337 触发表同构

上游：0–1 个工具轮正常，读文件 2 轮以上才 400。  
客户：纯文本轮看起来正常，Glob/Read/Task 之后的下一跳才问候或「看不到」。  
差别只在出口：他们遇到严格校验的 400；我们遇到宽松/忽略 id 的 200。

### 3.4 看起来「有时好」的其它硬条件

- 新开一条 Desktop 会话：键是新的，前几轮无 id → 不裁或只 12 条。
- 组 8 当时 `schedulable_count` 3～4，换到 1686/1730 会清空 id，退回 12 条；最近问句还在，不一定问候。
- 自包含指令（「把这段贴到 CLAUDE.md 哪里」）不依赖被裁历史。
- 直打 `/v1/chat/completions`：根本不进这两条函数，所以「同一把 Key 直打从来没有」与「桥上有时有」同时成立。

## 4. 对原结论 / 方案的可靠性

| 命题 | 可靠度 | 依据 |
|---|---|---|
| 失忆/断流由 latest-turn 把 20 万字节裁成 2/3 条造成 | 高 | 三笔生产 `input_item_count` + 代码互斥条件 |
| 这是上游桥的设计，不是本仓私货 | 高 | PR 2204 + 现网 `upstream/main` 仍在 |
| 上游赌的是「坏了会 400，然后 full replay」 | 高 | PR 2204 原文 + `isOpenAICompatPreviousResponseNotFound` |
| 第三方 200 让这条后路失效 | 高 | 三笔均无 retry Info |
| 上游讨论过「Desktop 问候语失忆」原文 | **无** | 检索未命中；不能声称有 |
| 上游讨论过同一 trim 的间歇性工具故障 | 高 | #2337 自己的表 + Rejected 关 id |
| 不是每轮都发作，因为可见症状取决于被裁内容是否必需 | 高 | 状态机 + 客户时间线 + #2337 |
| 每轮在 bind 之后都会裁（即使用户没察觉） | 高 | `previousResponseID != ""` 无额外随机开关 |
| 方案「API Key 默认 full replay」与上游自己的 unsupported 退路同方向 | 高 | PR 2204 第 2 节 |
| 方案会和上游 cache 续链目标分叉 | 中（可接受） | 他们 Rejected 关 id 是为官方 store/cache；我们的 API Key 实测 `upstream_cached_tokens=0`、`store=false`，续链目标本来就没打中 |

不因此改方案：换组/关桥仍不是交付；也不把 #5237 当成已证复件。
