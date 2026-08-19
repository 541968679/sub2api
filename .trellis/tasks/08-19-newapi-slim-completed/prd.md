# NewAPI slim completed 兼容空输出

## Goal

客户 NewAPI 不会改代码。当 Responses SSE 的 `response.completed` 带空 `output[]` 或整包快照时，NewAPI 会把「输出」当成空。Sub2API 在本侧对命中用户做 **in-place slim**：下游只看到 `{type, response.id, response.usage}`。默认全站关闭，灰度用户 220。不摘、不降权账号 1685。

## Background

运维因成本不能摘 1685。Brandon 同意先做代码修复（「运维没办法有成本考虑不能摘，先做代码修复吧」）。本任务只修 NewAPI 能读到的 completed 形状，不修 NewAPI 已经停读之后的 Codex 断连，也不修上游 `response.failed` / 1685 连接失败。

## Decisions

| # | Decision |
|---|----------|
| D1 | Settings KV 复制 flush-preamble：`openai_newapi_slim_completed`（默认 false）+ `openai_newapi_slim_completed_user_ids`（默认 `[]`）。无 `api_keys` 新列，不进 public settings。 |
| D2 | 全站 false 时只对白名单用户生效；true 时全员。本任务不得把默认或种子改成全站 true。灰度 220 由上线后管理员写入白名单，不写死进 KV 默认。 |
| D3 | Slim 只动下游 SSE。`parseSSEUsageBytes` 的计费实数不变。Slim 数字必须等于 display rewrite 之后的 SSE。 |
| D4 | 两条 HTTP Responses 路径都做：`handleStreamingResponsePassthrough` 与 `handleStreamingResponse` 的 `processSSELine`。 |
| D5 | 已有 `response.completed`：只 slim 这一帧，不补第二帧。流在 `response.done` / `incomplete` / `cancelled` / `canceled` 结束且尚未有 completed、且计费 `output_tokens != 0`：在 `[DONE]` 之前补一帧 slim completed。 |
| D6 | 计费 `output_tokens == 0`：不 slim、不合成。`clientDisconnected`：不写任何额外帧。 |
| D7 | 新建 usage，不拷贝上游 usage 对象（避免 null）。整数：`input_tokens` / `output_tokens` / `total_tokens`；可选 `input_tokens_details.cached_tokens`。默认不加 `completion_tokens`。 |
| D8 | 不改账号 1685 调度 / 错误态 / 权重。 |

## Requirements

### R1 — 开关

管理员可改两个 KV。缺省：gate=false，user_ids=[]。Gate 为 true 时忽略白名单、全员命中。Gate 为 false 时仅 `ctx` 用户 ID 在名单内命中。名单规范化与 flush-preamble 相同：丢弃 `<=0`、去重、非法 JSON 当空。

### R2 — 命中后的 in-place slim

对 `response.completed`：下游 data 仅为：

- `type` = `response.completed`
- `response.id`
- `response.usage`：`input_tokens`、`output_tokens`、`total_tokens`（= 前两者之和）均为整数
- 当 display 后的源事件里 `cached_tokens` 是数字（含 0）时，带 `input_tokens_details.cached_tokens`；缺省或 null 则整段省略

去掉 `output`、`reasoning`、`temperature`、`max_output_tokens`、`store`、`status` 及其它未列字段。

### R3 — 合成 completed

同时满足才写一帧 slim `response.completed`：命中开关；未写过 completed；见过 `response.done` 或 `response.incomplete` 或 `response.cancelled`/`canceled`；计费解析 `output_tokens != 0`；客户端未断开。写在即将发出的 `[DONE]` 之前；上游没有 `[DONE]` 则在流正常结束时补。已有 completed 则只 slim、不合成。

### R4 — 不作为

`output_tokens == 0` 保持原 completed。`response.failed` 原样（本任务不 slim、不从 failed 合成）。断开客户端后不补帧。Compact v2 早退、WS、非流式 JSON、`/v1/messages` 不在范围。

### R5 — 顺序与计费

必须先 `parseSSEUsageBytes`（或现有等价的失败分支 parse），再 display rewrite，再 slim。返回给计费的 `OpenAIUsage` 仍是 rewrite 前的实数。Slim / 合成数字与 display 模式 SSE 一致。

### R6 — 管理面

Admin 网关转发设置里，紧挨 flush-preamble，同样 Toggle + `OpenAIFastPolicyUserSelector`。中英 i18n。说明：兼容 NewAPI 空输出 / 过胖 completed；默认关；全站开会作用于所有人。`SettingsView.spec.ts` 覆盖默认与保存字段。

### R7 — 运维约束

本任务不改账号 1685。发版后只把 user_ids 设为 `[220]`，不打开全站 gate。

## Acceptance Criteria

- [ ] **AC1 / R1**：默认 gate=false、user_ids=[]；gate=false + `[220]` 只对用户 220 命中；无 UserID 不命中；gate=true 对任意用户命中。
- [ ] **AC2 / R2+R5**：命中用户、计费 output_tokens≠0 的 completed：下游无 `output`/`temperature`/`completion_tokens`；usage 为整数且等于 display rewrite 后的数；同一请求计费 usage 仍是 rewrite 前实数。
- [ ] **AC3 / R2**：上游 usage 含 null 字段时，slim 对象里没有那些 null；有数字 `cached_tokens` 则保留为整数。
- [ ] **AC4 / R3**：只有 done/incomplete/cancelled + 无 completed + output_tokens≠0：补且只补一帧 slim completed，且位于 `[DONE]` 之前（若有 `[DONE]`）。
- [ ] **AC5 / R3+R6**：已有 completed 时不合成第二帧；只 slim 原 completed。
- [ ] **AC6 / R4**：output_tokens==0 不 slim 不合成；clientDisconnected 不补帧；failed 事件不被 slim。
- [ ] **AC7 / D4**：passthrough 与 `processSSELine` 两条路径都满足 AC2–AC6。
- [ ] **AC8 / R6**：Admin 中英开关+白名单；保存走现有 settings API；未进 public / injection schema。
- [ ] **AC9 / R7**：diff 不含账号 1685 调度或人工禁用。

## Out of Scope

- 改 NewAPI / 让客户打补丁
- 禁用、降权、摘除账号 1685
- Codex 在 NewAPI 已停读之后的断连
- 上游 `response.failed`、1685 连接失败
- 默认加 `completion_tokens`、全站默认开启
- WebSocket、compact v2 早退路径、非流式 JSON、Anthropic messages 桥
- 新 `api_keys` 列或 per-key 开关
