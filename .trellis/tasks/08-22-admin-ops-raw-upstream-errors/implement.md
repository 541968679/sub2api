# 管理员上游原文 — 执行清单

## 1. 固化「泛化句」集合

- 与 `frontend/src/views/admin/ops/utils/errorDetailResponse.ts` 的 `GENERIC_UPSTREAM_MESSAGES` 对齐，在 backend 放一份测试锁定的集合，供 `setOpsUpstreamError` 合并逻辑使用。
- 不要把这份集合当成客户端映射表。

## 2. 统一 Ops 记录入口

- 抽出 `recordOpsUpstreamAttempt`（名称可调整），替换各网关里「extract + 可选 LogUpstreamErrorBody + append」的复制粘贴。
- 改 `setOpsUpstreamError` 合并规则（空/泛化不覆盖原文）。
- `UpstreamFailoverError` 若存在「ResponseBody 已被改写成客户端 JSON」的路径，补 `RawUpstreamBody` 或保证先记后改。
- `handleFailoverExhausted` / `handleFailoverExhaustedSimple`：客户端仍 `mapUpstreamError`；Ops 不得再用映射句当 upstream message。

## 3. 接线所有会写 Ops 的上游错误路径

按文件扫 `setOpsUpstreamError` / `appendOpsUpstreamError` / `newOpenAIStream*Error`：

- `openai_gateway_service.go`（HTTP、stream、passthrough、compat）
- `openai_gateway_handler.go`（failover 耗尽、streaming aware）
- `gateway_service.go`
- `gemini_messages_compat_service.go`
- Antigravity / images / embeddings / count_tokens / WS

验证：每条路径的 event.Detail 在单测里能拿到原始 `error.code`。

## 4. 列表 DTO

- `OpsErrorLog` + repo SELECT 增加 `upstream_error_message`、`provider_error_code`。
- insert 时填 `provider_error_code`。
- 前端 `OpsErrorLog` 类型同步。
- 不把 `error_body` 放到列表行。

## 5. 管理员 UI

- `OpsErrorLogTable`：响应内容列主行上游原文，次行下游句。
- `OpsErrorDetailModal`：三块（原文 / 上游 JSON / 下游 JSON）；去掉「只挑一份」当唯一正文。
- 上游错误卡片：展开该跳上游 JSON，标题用原文。
- i18n zh/en 同步。
- 单测：`errorDetailResponse.spec.ts`、`OpsErrorLogTable.spec.ts`、modal 相关 spec。

## 6. 回归

- 现有客户端映射测试不得改期望。
- 用户 `/usage` 组件不改。
- 口径 / Recovered / `needs_ops_attention` 测试不改期望。

## Validation

```powershell
go test -tags=unit ./backend/internal/service -count=1 -run "Ops|Passthrough|OpenAI.*Error|Gateway.*Error|FailoverExhausted"
go test -tags=unit ./backend/internal/handler -count=1 -run "Ops|FailoverExhausted|OpenAI.*Error"
pnpm --dir frontend exec vitest run src/views/admin/ops/utils/__tests__/errorDetailResponse.spec.ts src/views/admin/ops/components/__tests__/OpsErrorLogTable.spec.ts
```

按改动补精确 `-run` / spec 路径。最后一轮再跑本任务触及的全量相关包，不要只跑自己新写的一个 case。

## Review gates

- 客户端 fixture 的 message 字符串与改前一致。
- 管理员详情三块都有，且上游块不是 `{"error":{"message":"Upstream request failed"}}`。
- 列表行没有完整 JSON blob。
- 质量/SLA 单测未改断言。

## Rollback

`git revert` 本任务提交。无 migration。
