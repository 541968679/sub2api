# Implement

Brandon 已授权本任务在占用 checkout `E:\cursor project\api2sub`（当前就是 `main`）上实现。这是普通修 bug，不要求隔离 worktree；不要另开 worktree，不要把本仓切离 `main`。

## Order

1. 在占用 `main` checkout 上直接改（不要建隔离 worktree，不要复用旧 sync/catchup 分支）。
2. 先加 D1 失败单测（`openai_messages_continuation` / `openai_gateway_messages`），确认现网行为：有 previous_response_id 时 input 被裁到最新跳。
3. 改 `ForwardAsAnthropic` API Key full replay；删或旁路 12 条默认与 latest-turn。
4. 升 Info 日志（request_id + message_count + input_item_count + store）。
5. D3 流收口：cancel 跟客户端、interval 不吃空行、无可见输出必须 Anthropic error。
6. 跑单测包：`go test -tags=unit ./internal/service -count=1` 聚焦 messages/continuation/replay；相关 handler 测试。
7. 本地用 key 317 同类 Desktop 会话走 AC6（或用录制 body 回放，禁止把客户规则全文提交进仓）。
8. 追加 `docs/dev/CHANGELOG_CUSTOM.md` + `docs/dev/codebase/model-mapping.md` 若桥接不变量变化。
9. 等 Brandon 当次授权再 commit / 再谈发版。

## Validation

```text
go test -tags=unit ./internal/service -count=1 -run "AnthropicCompat|ForwardAsAnthropic|ReplayGuard|Continuation|EmptyVisible|ClientCancel"
```

人工：AC6。生产对照：修后同结构请求的 `input_item_count` 不得再是 2 或 3。

## Risky files

- `backend/internal/service/openai_gateway_messages.go` — 热路径，只动续链门闩与流收口。
- `backend/internal/service/openai_messages_continuation.go`
- `backend/internal/service/openai_messages_replay_guard.go`
- `backend/internal/handler/openai_gateway_handler.go` — cancel 后不要再静默 200。
- 不要改 `applyClaudeGPTBridgeDisplayCacheOverride` 的百分比算法。
- 不要改 `AnthropicToResponses` 的 `store=false`，除非单测证明 API Key 上游能 store；本设计不靠 store。

## Stop conditions

- 功能冲突要动展示计费 / `actual_cost` / 调度 pair → 停，问 Brandon。
- 发现不改 OAuth 就过不了单测 → 停，问 Brandon，禁止顺手改 OAuth 续链。
