# Design — NewAPI slim completed

## Boundary

- **In**: 原生 OpenAI `/v1/responses` HTTP SSE：`handleStreamingResponsePassthrough` 与 `handleStreamingResponse` → `processSSELine`。Admin Settings KV + 管理页。
- **Out**: 下游 SSE 的 `response.completed` 变瘦，或在软终止后补一帧瘦 completed。计费 `OpenAIUsage` 不变。
- **Reuse**: flush-preamble 的「全站 bool + user_ids JSON + gateway forwarding cache + `ctxkey.UserID`」；`parseSSEUsageBytes`；`rewriteOpenAIResponsesSSEUsageTokens`；`OpenAIFastPolicyUserSelector`。
- **Not in**: WS、compact v2 早退、非流式、`/v1/messages`、账号 1685、NewAPI 仓。

## Gate

新 KV：

| Key | Default | Meaning |
|-----|---------|---------|
| `openai_newapi_slim_completed` | `false` | 全站开 |
| `openai_newapi_slim_completed_user_ids` | `[]` | 全站关时的白名单 |

实现上 **复制** flush-preamble 全链路，不要抽公共泛型（除非只是 5 行 `parseInt64IDList` 的局部复用且两套测试都还绿）。新函数名：`IsOpenAINewAPISlimCompletedEnabled(ctx)`。

热路径只在两个 `handleStreaming*` **入口读一次**（与 `flushPreamble := ...` 并列），不要每条 SSE 打 DB。

`ctx` 无正用户 ID 且全站 false → 关闭。nil `SettingService` → false。

上线后管理员写 `[220]`。代码默认保持 `[]`，避免其它环境误灰度。

## Slim payload

新建，禁止 `sjson` 删字段或拷贝 `response.usage`：

```text
{"type":"response.completed","response":{"id":ID,"usage":{...}}}
```

`usage` 只放整数：

- 必有：`input_tokens`, `output_tokens`, `total_tokens`（= input+output，现场算）
- 可选：`input_tokens_details.cached_tokens` —— 仅当 **display 后的源 JSON** 该路径存在且为 Number
- 禁止：`completion_tokens`、null、`output`、`status`、`temperature`、`max_output_tokens`、`store`、reasoning、image_tokens、cache_write

`response.id`：优先 display 后 JSON 的 `response.id`，否则用循环里已有的 `responseID`；没有就写 `""`（形状仍在）。

SSE 行格式跟当前路径一致：passthrough 用现有 `data: ...` + `Fprintln`；`processSSELine` 用 `line + "\n"`。

## Data flow

```text
upstream data line
  → 现有 sanitize / tool / image / terminal-output normalize（仅 non-passthrough）
  → parseSSEUsageBytes(original dataBytes)     // 计费实数；passthrough 已在 rewrite 前；non-passthrough 保持对 dataBytes parse
  → rewriteOpenAIResponsesSSEUsageTokens       // 仅下游
  → if slimEnabled && type==completed && billing.OutputTokens != 0:
        replace downstream line with buildSlim(extractDisplayUsage(rewritten))
        mark wroteCompleted
  → write downstream (unless clientDisconnected)
  → on [DONE] or clean EOF:
        if slimEnabled && !clientDisconnected && !wroteCompleted
           && sawSoftTerminal && billing.OutputTokens != 0:
             write one slim completed (from last display usage, else billing+rewrite)
        then write [DONE] if this line is [DONE]
```

`sawSoftTerminal` = 见过 `response.done` | `incomplete` | `cancelled` | `canceled`。不含 `failed`、不含单独的 `[DONE]`。

`billing.OutputTokens` 用 **parse 后的** `usage.OutputTokens`（实数）。`len(data) < 72` 的 parse 早退保持不动；瘦包只发生在 parse 之后，避免把计费饿死。

合成时：优先用该流里最后一次软终止事件 **rewrite 之后** 抽出的整数；没有则用计费整数组一帧再跑一遍 `rewriteOpenAIResponsesSSEUsageTokens`，保证与 display 模式一致。

`response.failed`：现有 failover / passthrough-rule 不动；不 slim、不从 failed 合成。

`clientDisconnected`：继续 drain 上游收 usage；禁止合成、禁止再写 completed。

## Hook 落点

建议新文件 `openai_newapi_slim_completed.go`：

- `newAPISlimUsage` + `buildNewAPISlimCompletedData` / `buildNewAPISlimCompletedSSELine`
- `extractNewAPISlimUsageFromResponsesData(data []byte) (newAPISlimUsage, ok)`
- `shouldSlimNewAPICompleted(enabled bool, eventType string, billingOutputTokens int) bool`

`openai_gateway_service.go` 只加：入口 `slimEnabled`、循环状态（`wroteCompleted`, `sawSoftTerminal`, `lastDisplaySlimUsage`）、写下游前替换、`[DONE]`/EOF 补帧。

Passthrough 今天在 rewrite 后写 `line`；slim 插在 rewrite 与 write 之间。  
`processSSELine` 今天先 rewrite `lineForDownstream` 再写再 parse：把 parse **提前到 rewrite 之前**（仍针对 `dataBytes`）或保持 parse `dataBytes`、slim 只改 `lineForDownstream`。计费不得改读 slim 后的字节。

不要改 `writeMarkedCodexCompactV2Stream` 早退。

## Settings / UI 全链路

与 flush-preamble 同一批文件，新增字段并列：

- `domain_constants.go`
- `settings_view.go`
- `setting_service.go`：defaults、`Update*` marshal、cache struct + `GetMultiple`、`IsOpenAINewAPISlimCompletedEnabled`、parse/normalize/match（可复用 flush-preamble 的 normalize 函数以免两套去重规则漂）
- `handler/dto/settings.go`（Admin DTO only）
- `handler/admin/setting_handler.go`：get / patch / changed
- `frontend/src/api/admin/settings.ts`
- `SettingsView.vue`：flush-preamble 块正下方；form 默认 / 数组保护 / save payload
- `locales/zh.ts` + `en.ts`：`slimCompleted*`
- `SettingsView.spec.ts`：i18n stub、默认、save

不要加入 `PublicSettings` / `PublicSettingsInjectionPayload`。

## Compatibility / rollback

- 默认全关：未配白名单行为与今天一致。
- 回滚：还原上述文件；已发出的瘦包无法回收，客户端重试即可。
- 关开关立即生效（60s forwarding cache TTL，与 preamble 相同）。
- 显示倍率：slim 数字必须跟 rewrite 后 completed 一致，否则 NewAPI 账单展示会和 Sub2API 用户用量页分叉。

## Trade-offs

- **瘦掉 completed.output**：依赖 NewAPI 已消费增量事件。这是本故障的修复面；若某客户端只读 completed.output，命中用户会丢快照。故默认关 + 白名单 220。
- **按计费 output_tokens 而不是 `output[]` 是否为空做门闩**：与锁定行为一致；避免「有 delta、completed.output 为空」时漏 slim。
- **不从 `[DONE]` 单独合成**：没有软终止事件就没有可靠 usage 形状；避免空 completed。
- **KV 默认 `[]` 而不是 `[220]`**：220 是这次事故灰度，不是产品默认。
