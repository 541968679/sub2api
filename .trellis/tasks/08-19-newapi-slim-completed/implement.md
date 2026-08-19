# Implement — NewAPI slim completed

## Checklist

1. **Settings KV（先于热路径）**  
   复制 flush-preamble：`SettingKeyOpenAINewAPISlimCompleted` / `UserIDs`；defaults `"false"` / `"[]"`；`cachedGatewayForwardingSettings` + `GetMultiple` + store/refresh；`IsOpenAINewAPISlimCompletedEnabled`；`GetAllSettings` / update marshal；`settings_view.go`。parse 可调用现有 `normalizeOpenAIResponsesFlushPreambleUserIDs`。单测仿 `TestOpenAIResponsesFlushPreambleSettingDefaultOff`（fixture 用户 220）。

2. **Admin DTO + handler**  
   `dto/settings.go`、`setting_handler.go` get/patch/changed。不要碰 `PublicSettings`。

3. **Slim helper**  
   新文件 `backend/internal/service/openai_newapi_slim_completed.go`：build / extract / shouldSlim。表测：null usage 不泄漏、无 `completion_tokens`、`total_tokens=input+output`、cached 仅 Number、output_tokens==0 不 slim。

4. **Passthrough**  
   `handleStreamingResponsePassthrough`：入口读 gate；`parseSSEUsageBytes` → display rewrite → slim → write。拦截 `[DONE]`：满足合成条件则先写一帧 slim completed。`clientDisconnected` 不补。`response.failed` 不走 slim。

5. **processSSELine**  
   同样顺序与状态。parse 必须针对 **原始 `dataBytes`**。只替换 `lineForDownstream`。EOF / `[DONE]` 合成规则与 passthrough 相同。不要改 compact v2 早退。

6. **Admin UI + i18n**  
   `settings.ts`、`SettingsView.vue`（preamble 正下方 + `OpenAIFastPolicyUserSelector`）、`zh.ts`/`en.ts`、`SettingsView.spec.ts`（stub、默认 false/`[]`、save 带新字段）。

7. **文档**  
   `docs/dev/CHANGELOG_CUSTOM.md` 追加。`docs/dev/codebase/gateway.md` 记一条：Responses SSE 可选 slim completed、默认关、白名单、计费实数不变。

8. **不要做**  
   改账号 1685、打开全站默认、WS、messages 桥、给 public settings 加字段、默认写 `[220]` 进 KV。

## Validation

```powershell
# 设置 + helper + 热路径（按实际测试名调整）
go test -tags=unit ./internal/service -count=1 -run "TestOpenAINewAPISlimCompleted|TestOpenAIResponsesFlushPreambleSettingDefaultOff|TestOpenAIGateway.*Passthrough|TestStreamStageTiming_Responses"

# 若加了 handler 单测
go test -tags=unit ./internal/handler/admin -count=1 -run "Setting"

# 前端
pnpm --dir frontend exec vitest run src/views/admin/__tests__/SettingsView.spec.ts
pnpm --dir frontend run typecheck
```

在 `backend/` 下跑 Go。手工对照（命中用户 220、display 开/关各一条）：

| 场景 | 期望 |
|------|------|
| completed + output_tokens≠0 + 胖/空 output | 下游只有 type/id/usage 整数 |
| 同上 + display 倍率 | slim 数字 = rewrite 后；计费 usage = 实数 |
| completed + output_tokens==0 | 原样 |
| done/incomplete/cancelled + 无 completed + tokens≠0 | `[DONE]` 前恰好一帧 slim completed |
| 已有 completed | 不出现第二帧 completed |
| 非 220 且 gate false | 行为与改前一致 |
| 模拟下游断开 | 无补帧 |
| failed | 不 slim |

本任务不在生产改 1685、不 `task.py start` 之后自动 deploy。灰度：管理员把 `openai_newapi_slim_completed_user_ids` 设为 `[220]`，**不要**开全站。

## Risky files / rollback

| Area | Risk | Guard |
|------|------|--------|
| `parseSSEUsageBytes` 72 字节早退 | 先 slim 再 parse → 计费丢 usage | 只 slim 下游行；parse 原始 bytes |
| display 与 slim 顺序反了 | NewAPI 用量和用户页不一致 | rewrite 后再抽整数 |
| 每条 SSE 读 settings | 热路径打 DB | 入口缓存一次 |
| 合成第二帧 | NewAPI 重复 completed | `wroteCompleted` |
| 从 failed / 仅 `[DONE]` 合成 | 假成功 | `sawSoftTerminal` 不含 failed |
| `PublicSettings` 误加字段 | injection schema 测挂 | 只加 Admin DTO |
| 默认 `[220]` 或 gate true | 全站误伤 | 默认 false/`[]` |
| 动 1685 | 违反运维约束 | diff 审查 |

回滚：还原 helper、两处 hook、settings/i18n。60s 内 cache 失效后未命中用户恢复原 completed。

## Ready for start

- [x] `prd.md` 需求与 AC，无设计清单
- [x] `design.md` 边界 / 数据流 / 回滚
- [x] `implement.md` 本清单
- [x] `research/sse-hotpath-and-flush-preamble.md`
- [x] `implement.jsonl` / `check.jsonl` 实条目
