# 管理员可见上游原文错误，下游屏蔽策略不变

## Goal

管理员在现有错误展示面上能同时看到三份信息：上游原文、上游 JSON、我们返回给下游的错误 JSON。客户端收到的错误响应保持现有屏蔽策略，不因本任务多泄漏任何上游信息。

## User value

loveapi 这类 `channel:no_available_key` 现在会被收成 `Upstream request failed` / `Upstream service temporarily unavailable`。管理员无法用本系统对供应商。修完后，账号/用户错误列表、详情、上游错误卡片都能对上原文和两边 JSON。

## Background

生产 loveapi 009（account 1730）间歇失败；供应商原文是 `type=new_api_error`、`code=channel:no_available_key`、`message=no enabled keys (...)`。本系统对应 Ops 行只剩泛化文案。

2026-08-22 决定：不在「列表只显示一句」和「只改详情」之间二选一。现有几种展示形式都要适应，信息才完整。

## Confirmed facts

管理员错误 UI 复用同一套 Ops 数据：

| 展示面 | 实现 | 现在看到什么 |
|---|---|---|
| 错误列表「响应内容」列 | `OpsErrorLogTable.vue:212-218`，`formatSmartMessage(log.message)` | 只有客户端映射句。`OpsErrorLog.message` 来自客户端 body |
| 账号/用户错误列表 | `UsageErrorInspectDialog.vue` / `UsageView.vue` 复用同一张表 | 同上 |
| 详情「响应内容」 | `OpsErrorDetailModal.vue:111-115`，`resolvePrimaryResponseBody` 只挑 **一份** | `error_body` 是下游 JSON；上游字段也经常已被泛化覆盖，所以仍空 |
| 上游错误卡片 | 同 modal `showUpstreamList` + `getUpstreamResponsePreview` | 每跳一张卡，preview 仍常是泛化 wrapper |

列表 DTO `OpsErrorLog` **不返回** `upstream_error_message` / `error_body`。详情才有 `error_body`、`upstream_error_message`、`upstream_error_detail`、`upstream_errors`。

用户控制台 `/usage` 用 `UserErrorRequestsTable` + `UserErrorDetailModal`，只读客户端 `message` / `error_body`。这是下游用户面，本任务不改。

收口分三层：

1. **写给客户端的映射（必须保持现状）**
   - OpenAI failover 耗尽：`openai_gateway_handler.go:2200-2214` `mapUpstreamError`。
   - Claude/Gemini 非 failover：`gateway_service.go:7429-7463` 等。Claude **400 会 `c.Data` 原样回传**，不改。
   - OpenAI 非 failover `handleErrorResponse`：`openai_gateway_service.go:5042-5077`。400 和 default 抽得出 `error.message` 时会回给客户端，不改。
   - 流式：`handleStreamingAwareError` 写映射后的句。
   - Bridge：`scrubBridgeClientText` 只作用于回给客户端的文案。

2. **落库时用了映射后的内容（要修）**
   - `ops_error_logger.go:934` 的 `ErrorMessage` 来自已经写给客户端的 body。这行应继续表示「下游 JSON / 下游文案」。
   - `handleFailoverExhausted`（`openai_gateway_handler.go:2179-2190`）从 `FailoverError.ResponseBody` extract；body 若已被改写成泛化 JSON，`upstream_*` 也废了，且 detail 传空串。
   - `newOpenAIStreamFailoverError` / `newOpenAIStreamClientError` 用自己拼的泛化 JSON 当 payload 写入 `upstream_errors[].detail`。
   - `Detail` 多数路径被 `LogUpstreamErrorBody` 挡住。该开关管 stdout，不是管理员可见性。
   - `setOpsUpstreamError` 后写覆盖先写；映射句会把原文冲掉。

3. **已有、可复用**
   - `ops_error_logs` 已有 `upstream_status_code` / `upstream_error_message` / `upstream_error_detail` / `upstream_errors`，以及未接线的 `provider_error_code`。不加表也能存原文。
   - `extractUpstreamErrorMessage` / `extractUpstreamErrorCode` 能读 new-api 的 `error.message` / `error.code`。
   - 落库已有 `sanitizeErrorBodyForStorage`。管理员原文必须走它。

## Requirements

- R1. 任意会写 Ops 的网关热路径，在映射成客户端响应**之前**，把该次上游尝试的真实 status、`error.code`（有则）、`error.message`（有则）、截断后的原始 body 写入管理员专用字段。禁止用映射后的泛化 JSON 覆盖这些字段；禁止用空 detail 覆盖已有原文。
- R2. 客户端 HTTP 状态、error type、error message、SSE/WS 错误事件与当前映射策略一致。禁止把上游 code/body/供应商 request id **新**写进客户端响应。已有 400 原样回传、OpenAI 非 failover 回传 `error.message` 不改。
- R3. 用户控制台 `/usage` 错误列表和详情继续只展示客户端可见内容。
- R4. 管理员这三处都要能看到完整三份信息（列表做适应，不把两段完整 JSON 塞进窄列）：
  - **列表「响应内容」**（Ops、管理员 Usage、账号/用户错误弹窗共用 `OpsErrorLogTable`）：主行显示上游原文（有 code 则 `code + message`）；与下游文案不同时，次行或 title 标明下游文案。完整 JSON 进详情。
  - **详情**：三个独立块——上游原文、上游 JSON、下游错误 JSON。不再用「只挑一份」的 `resolvePrimaryResponseBody` 当唯一正文。
  - **上游错误卡片**：每一跳显示该跳的上游原文 + 上游 JSON；终端请求卡同时能看到下游错误 JSON。
- R5. 覆盖 OpenAI（stream / failover 耗尽 / WS）、Claude gateway、Gemini compat、Antigravity、images / embeddings / count_tokens 等会写 Ops 的上游错误路径。
- R6. 管理员原文走现有截断与 `sanitizeErrorBodyForStorage`。不把完整 API key / token 明文写入 Ops。
- R7. 不改 SLA、Recovered 口径、调度 ErrorCount、计费、`skip_monitoring`。列表筛选/质量谓词仍读现有 `message` / `error_body`（下游侧），不要改成只认上游原文，以免口径漂移。

## Acceptance Criteria

- [ ] AC1. new-api 风格 body（`error.type=new_api_error`、`error.code=channel:no_available_key`、`error.message=no enabled keys`）走 OpenAI failover 耗尽：客户端仍是当前映射句（5xx → `Upstream service temporarily unavailable`）。
- [ ] AC2. 同一条 `ops_error_logs`：`error_message` / `error_body` 仍是下游映射结果；`upstream_error_message` / `upstream_error_detail` / 对应 `upstream_errors[]` 含原文 message 和原始 JSON 片段，且不是泛化 wrapper。
- [ ] AC3. 管理员列表「响应内容」能直接读到 `no enabled keys`（或带 code 的等价原文），并能辨认下游映射句；不展示完整上游 JSON。
- [ ] AC4. 管理员详情同时展示上游原文、上游 JSON、下游错误 JSON 三块，内容分别对应 AC2 的字段。
- [ ] AC5. 上游错误卡片能展开看到该跳上游 JSON，且原文不是泛化 wrapper。
- [ ] AC6. 用户 `/usage` 错误列表/弹窗看不到 `channel:no_available_key` 或供应商 request id（除非现状本来就会回给该用户，例如现有 400 原样回传）。
- [ ] AC7. Claude/Gemini 现有客户端映射单测（401/403/429/5xx 固定句、400 行为）期望不变。
- [ ] AC8. 中英文 i18n 新文案同步。

## Out of scope

- 改变下游屏蔽策略（含收紧现有 400 原样回传）。
- 改 failover / pool_mode 是否换号。
- 按 `channel:no_available_key` 做新熔断或调度权重。
- 供应商 request id 单独索引。
- 回填历史 `ops_error_logs`。
- 用户控制台 `/usage` 展示上游原文。

## Decisions

- 列表、详情、上游错误卡片都改；完整三份信息以详情和卡片为准，列表做适应压缩。
- `error_message` / `error_body` 继续表示「返回给下游的内容」。上游原文只进 `upstream_*`。
- 用户控制台保持屏蔽。管理员「用户错误列表」是 `UsageErrorInspectDialog`，要改。

## Notes

- 复杂任务。实现前须 Brandon 看过本 PRD / `design.md` / `implement.md` 再 `task.py start`。
- 不要把 `LogUpstreamErrorBody` 当成管理员可见性总开关。
