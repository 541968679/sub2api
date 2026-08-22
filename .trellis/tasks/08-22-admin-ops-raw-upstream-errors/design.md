# 管理员上游原文 — 技术设计

## Boundary

- **In**：网关刚读到的上游 HTTP/SSE/WS 错误 body；现有 Ops 表和详情 API。
- **Out**：管理员列表/详情/上游卡片能读到原文 + 两边 JSON；客户端响应字节级策略不变。
- **Not**：用户 `/usage`、SLA/质量口径、failover 选号、新 migration（除非现有列不够用，当前够用）。

## Data flow

```
upstream body (raw)
  -> extract message/code
  -> recordOpsUpstreamAttempt(...)   // 只写 upstream_* / events
  -> existing client map (unchanged)
  -> error_body / ErrorMessage       // 只写下游
  -> admin UI reads both sides
```

`ErrorMessage` 继续来自客户端 body。质量谓词、列表筛选继续看下游 `message` / `error_body`。

## Capture contract

新增（或抽）一个只给 Ops 用的记录函数，热路径统一调用，禁止各处自己拼泛化 JSON 再当 upstream detail：

```
recordOpsUpstreamAttempt(c, account, status, requestID, kind, rawBody)
```

规则：

1. `rawBody` 必须是上游原文（截断后），不是 `mapUpstreamError` 之后的 wrapper。
2. `Message` = `extractUpstreamErrorMessage(rawBody)`；空则保留空，不要填 `Upstream request failed`。
3. `Detail` / `UpstreamResponseBody` = `truncate + sanitizeErrorBodyForStorage(rawBody)`。**不**依赖 `LogUpstreamErrorBody`。该开关只继续控制 stdout。
4. 有 `error.code` 则写入已有 `provider_error_code`（insert 时带上；列表也可带回）。
5. `setOpsUpstreamError` 改为合并而不是无条件覆盖：
   - 空 detail 不得覆盖非空 detail。
   - 泛化句（`Upstream request failed` / `temporarily unavailable` / `Upstream gateway error` 等，与前端 `GENERIC_UPSTREAM_MESSAGES` 对齐）不得覆盖更具体的 message。
6. `handleFailoverExhausted` 必须用 **FailoverError 里尚未改写的 ResponseBody** 调记录函数；写给客户端的仍走 `mapUpstreamError`。若某条路径会先把 `ResponseBody` 改成泛化 JSON，先记原文再改，或给 `UpstreamFailoverError` 加只读 `RawUpstreamBody`。
7. `newOpenAIStream*` 若要把泛化 JSON 回给客户端，Ops event 的 Detail 仍传调用方手里的上游 payload，而不是新拼的 wrapper。

覆盖面：OpenAI HTTP / stream / WS / passthrough、compat CC/Messages、Claude `GatewayService.handleErrorResponse`、Gemini compat、Antigravity、images / embeddings / count_tokens。漏掉的路径以「该路径是否 `appendOpsUpstreamError` 或 `setOpsUpstreamError`」为准，有则改。

## List / detail / cards

### List

`OpsErrorLog` 增加只读压缩字段（不加宽 `error_body`）：

- `upstream_error_message`（已有列）
- `provider_error_code`（已有列，接线即可）

`OpsErrorLogTable`「响应内容」：

- 主行：`formatSmartMessage(code + " " + upstream_error_message)`，没有上游则回退 `log.message`
- 次行（更小字，仅当下游句与上游不同）：下游映射句
- title 含两句，方便悬停
- 不在表格里渲染 JSON

管理员 Usage、账号/用户错误弹窗自动跟上（同一组件）。

### Detail

拆开现在的单一 `primaryResponseBody`：

1. 上游原文：code + message
2. 上游 JSON：`upstream_error_detail`，否则 pretty-print `upstream_errors` 里最后一跳的 raw
3. 下游错误 JSON：`error_body`

`errorDetailResponse.ts` 留下「如何识别泛化 wrapper / 如何选上游 payload」的纯函数，去掉「只返回一份」作为详情唯一正文的用法。

### Upstream cards

每张卡：

- 标题旁：该跳 `upstream_status_code` + 原文
- 展开：该跳上游 JSON（event.detail / correlated 行的 upstream_error_detail）
- 若这是请求错误详情里的关联卡：不要求每张卡重复下游 JSON；终端请求详情顶部已经有下游 JSON 块

## Compatibility

- 无 migration。历史行没有原文，UI 回退到现在的下游句。
- 不改 `include_recovered`、三口径旗标、`needs_ops_attention` SQL。
- `scrubBridgeClientText` 只作用于客户端写出，不作用于 Ops 原文。
- 客户端单测期望字符串保持不变。

## Tradeoffs

- 列表不塞 JSON：窄列可读，完整对账进详情。
- 热路径永远带一小段 body 进 Ops：比「看开关」更稳，截断沿用现有 2KB 级上限。
- 不收紧现有 400 原样回传：避免本任务变成「改下游策略」。

## Rollback

只回滚本任务提交即可。未改 schema，旧 UI 仍能读 `message`。新列表字段缺失时前端按空处理。
