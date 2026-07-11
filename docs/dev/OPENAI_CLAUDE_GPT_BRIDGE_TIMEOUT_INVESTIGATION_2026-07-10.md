# Claude-GPT Bridge 间歇性 Timeout 调查与修复设计

日期：2026-07-10

状态：P0 严格路由与 P1 bridge-aware `count_tokens` 已于 2026-07-11 按本方案实现
（本地提交 `7f8022fa8`、`0e24044d6`、`b06190970`，unit 测试全绿），生产部署待发版。

2026-07-11 实现落地记录：

- P0 新增 `backend/internal/service/openai_claude_gpt_bridge_routing.go`
  （`ResolveClaudeGPTBridgeRoute`，纯 `ListByGroup` 诊断，无 scheduler slot）和
  `backend/internal/handler/openai_claude_gpt_bridge_route.go`
  （`ClaudeGPTBridgeRoute` 路由动作 + `respondClaudeGPTBridgeSelectionRace`
  竞态二次诊断 + Retry-After 计算/校验）。
- 删除了 `ShouldUseClaudeGPTBridge()`、`markOpenAIClaudeGPTBridgeFallback()`、
  `ClaudeGPTBridgeFallbackRequested()` 与隐藏 fallback context key；
  `routes/gateway.go` 按 not_configured/ready/handled 三态分发。
- bridge 全账号 429 用尽时最终响应保持 429，并透传校验过的上游
  `Retry-After`（正整数、≤86400 秒）。
- P1 以 upstream/main `e316ebf5` 为基线移植
  `openai_gateway_count_tokens.go`（service/handler），OpenAI 分组
  `/v1/messages/count_tokens` 不再 404；Antigravity bridge 分组增加
  bridge-aware 计数：ready 用映射后 GPT 模型上游计数（宽松模式，上游失败
  一律 200 本地估算并保留账号状态记账），限流/不可用/探测失败直接本地
  tiktoken 估算，绝不进入 native 池；无 mapping 分组保持 native 不变。
- 引入 `github.com/tiktoken-go/tokenizer v0.8.0`，估算样本值与官方一致。
- 关键两请求 429 回归及第 10 节矩阵主体已由
  `openai_claude_gpt_bridge_routing_test.go`、
  `openai_claude_gpt_bridge_route_test.go`、
  `openai_gateway_count_tokens_test.go`（service/handler 两份）覆盖。
- 语义注意：管理员手动暂停唯一 bridge 账号（`schedulable=false`）现在返回
  bridge 503 而不是回落 native；要让分组真正回到 native 必须删除映射或关闭
  账号 bridge 开关。第 12 节的 canary 注入与手动 compact 验证仍待生产/联调
  环境执行。
- 多代理对抗审查后加固（同日）：Messages 转发路径的 `UpstreamFailoverError`
  补齐 `ResponseHeaders`，使 all-429 用尽路径的 Retry-After 透传真正生效；
  分组禁用模型在容量 429/503 之前稳定返回 403；`RateLimitResetAt` 推导的
  Retry-After 以 86400 秒封顶；simple 运行模式诊断改用平台全量候选池与
  scheduler 口径一致；限流在两次判定间到期的账号按可调度处理；本地估算
  以 8 MiB 为界退化为字节近似防止本地计算 DoS；bridge count 预检读错误
  返回规范 413/400；诊断结果携带映射模型消除降级路径二次扫描；bridge
  count 路径补齐 ops 观测上下文。

范围：Antigravity 分组通过 OpenAI 账号执行 Claude-GPT bridge 的
`/v1/messages`、`/antigravity/v1/messages` 和 `count_tokens` 兼容路径

## 1. 结论

本轮调查把此前混在一起的现象拆成了三个独立问题：

1. **流式失败被表现为空回复**：上游 `response.failed`、缺少 terminal、
   reasoning-only 或 terminal-only output 可能被错误地当成成功空响应。当前工作区
   已有针对这类失败的缓冲、failover、terminal replay 和 compact recovery 加固。
2. **bridge 限流后错误切回 native**：这是本次 goal 自动填充很早就出现
   `API Error: The operation timed out.` 的最高置信度原因，也是仍待实现的 P0
   修复。当前路由把“已配置 bridge 但账号暂时不可用”误判为“没有 bridge”，
   随后进入没有可用账号的 native Antigravity 池并返回 503。
3. **bridge `count_tokens` 未接入 OpenAI 计数能力**：这是独立兼容性缺口。
   官方 upstream 已有可复用实现，但本 fork 的 Antigravity bridge 需要适配自己的
   账号资格和 Claude 到 GPT 显式映射。

因此，**不能再把压缩本身当成所有空回复或 timeout 的统一根因**。压缩能触发一种
失败形态，但本次最新 timeout 更符合账号限流和路由翻转链路；`count_tokens` 需要修，
但它也不是本次 timeout 的已确认直接原因。

## 2. 证据等级

| 等级 | 结论 | 证据 |
|------|------|------|
| 已确认 | 手动 compact 可以成功完成 | Claude Code JSONL 记录了 `compact_boundary`，`preTokens=256786`、`postTokens=6151`、`durationMs=98480`，压缩后 3 个 canary turn 均成功 |
| 已确认 | compact 周边的 `count_tokens` 503 没有阻止该次压缩 | 同一时间窗口内服务端出现两次 `count_tokens` 503，但客户端仍记录 `Compacted` 并继续对话 |
| 已确认 | bridge 上游曾快速返回结构化 429 | 普通 Messages bridge 请求约 1.9 秒返回 `usage_limit_reached`，账号随后进入限流冷却 |
| 已确认 | 当前 preflight 会把任何选号失败转换为 `false` | `ShouldUseClaudeGPTBridge()` 对 error、nil selection 和 nil account 均返回 `false`，路由随后进入 native handler |
| 已确认 | handler 首次 bridge 选号失败也可请求 native fallback | `MessagesClaudeGPTBridge()` 在尚无 failed account ID 时调用隐藏 fallback context |
| 高置信度推断 | 最新 goal timeout 是 `429 -> bridge cooldown -> native 503 -> client retry timeout` | 服务端时序与代码分支一致；但尚未把该 22:23 服务端请求和 CLI JSONL 中的单一 request ID 做一一对应 |
| 未确认 | 所有历史用户空回复都由同一个原因造成 | 采样窗口内约 80 条成功 `/messages` usage 都有正数 output token，没有一个证据能覆盖所有历史个案 |

本机可复查的手动 compact 记录位于：

```text
%USERPROFILE%\.claude\projects\E--cursor-project-api2sub--run-claude-bridge-e2e\7f3a91c0-2026-4c07-8a10-000000000001.jsonl
```

关键位置：手动 `/compact` 在第 47 行，`compact_boundary` 在第 52 行，压缩后的
canary turns 在第 67-76 行。该文件是本地诊断证据，不应提交到仓库。

## 3. 当前代码为何会间歇性失败

当前路由流程：

```text
Antigravity group /v1/messages
  -> ShouldUseClaudeGPTBridge()
     -> 读取请求模型
     -> 调用 SelectAccountWithSchedulerForClaudeGPTBridge()
     -> 有当前可用账号：true
     -> 任意错误或没有当前可用账号：false
  -> true: MessagesClaudeGPTBridge()
  -> false: native Gateway.Messages()
```

这里把两个不同问题合并成了一个 `bool`：

- **路由意图**：该分组、账号配置和模型显式映射是否要求走 bridge；
- **瞬时可用性**：这个时刻是否存在未限流、未 overload、未临时暂停且能抢到槽的
  bridge 账号。

这两个问题不能共用一个真假值。显式映射代表稳定的路由意图；限流、并发耗尽和
临时暂停只是动态容量状态。把动态状态当成“没有 bridge”会随机改变上游平台、模型、
错误语义和计费路径。

具体失败时序：

```text
请求 1
  -> bridge preflight 选到 OpenAI 账号
  -> GPT upstream 返回 429 usage_limit_reached
  -> 账号写入 RateLimitResetAt

请求 2（Claude Code 自动重试）
  -> bridge preflight 看不到可调度账号
  -> ShouldUseClaudeGPTBridge() 返回 false
  -> 请求切到 native Antigravity
  -> native 池无可用账号，返回 503

后续重试
  -> 重复 native 503 / 退避
  -> CLI 最终显示通用 operation timed out
```

问题只在账号处于特定动态状态时出现，所以它天然是间歇性的；正常会话、手动 compact
或 bridge 账号仍可调度时都不会必现。

### 3.1 当前错误信息为何不足

`SelectAccountWithSchedulerForClaudeGPTBridge()` 复用通用 OpenAI scheduler。通用选择
最终可能只返回 `ErrNoAvailableAccounts`。`no_account_error.go` 已明确记录：选择层不会
告诉 handler 池为空究竟是限流、模型不支持还是其他动态状态，handler 只能重新检查
静态 model mapping，并在 404 和 503 之间做粗分类。

对 bridge 来说还缺少以下信息：

- bridge 配置候选总数；
- 当前可调度候选数；
- 处于 `RateLimitResetAt` 的候选数；
- 最早恢复时间；
- overload、temp-unschedulable、过期、quota 或并发等阻断原因；
- 查询失败和真正“没有显式 bridge 映射”的区别。

## 4. 官方 upstream 对照

调查快照：

- 官方仓库：<https://github.com/Wei-Shaw/sub2api>
- `upstream/main`：`e316ebf52838a89d57fc790981cce7520f819ac8`
- 快照时间：2026-07-10 22:09:25 +08:00
- 最新 release：<https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.151>

### 4.1 官方已有的可复用修复

| 能力 | 官方状态 | 本 fork 处理方式 |
|------|----------|------------------|
| OpenAI 分组 Anthropic `count_tokens` | [PR #3497](https://github.com/Wei-Shaw/sub2api/pull/3497) 已合并；Anthropic 请求转换为 Responses input，调用 `/v1/responses/input_tokens` | 以当前 upstream 文件为语义基线手工移植 |
| OAuth input_tokens 缺 scope/不支持 | [PR #3635](https://github.com/Wei-Shaw/sub2api/pull/3635) 已合并；401/403/404 时用本地 tiktoken 估算 | 一并移植，不能只拿 #3497 初版 |
| Messages `response.failed` 不再吞成 200 空消息 | [PR #3859](https://github.com/Wei-Shaw/sub2api/pull/3859) 已合并 | 与本地空输出加固做语义对照，不做整块覆盖 |
| Messages 传输错误 failover | [PR #3853](https://github.com/Wei-Shaw/sub2api/pull/3853) 已合并 | 检查 bridge 路径是否保持相同语义 |
| Messages 缺 terminal | [PR #3051](https://github.com/Wei-Shaw/sub2api/pull/3051) 已合并 | 保留本地 missing-terminal 与 downstream terminal 防重逻辑 |
| SSE 应用错误透传 | [PR #3870](https://github.com/Wei-Shaw/sub2api/pull/3870) 和 [PR #3873](https://github.com/Wei-Shaw/sub2api/pull/3873) 已合并 | 确保 `context_length_exceeded` 等错误保持非重试 4xx 语义 |

### 4.2 官方尚未覆盖的部分

- 官方 `upstream/main` 不存在本 fork 的
  `openai_claude_gpt_bridge_enabled`、`ShouldUseClaudeGPTBridge()`、
  `MessagesClaudeGPTBridge()`、`RequireClaudeGPTBridge` 和
  `ResolveClaudeGPTBridgeModel()` 等符号。
- 官方 `/antigravity/v1/messages` 与其 `count_tokens` 仍是 native
  Antigravity handler，不存在“Antigravity group 使用 OpenAI account-side bridge”的
  路由状态机。
- 最接近“无可见输出/reasoning-only/上下文超限完整处理”的
  [PR #3808](https://github.com/Wei-Shaw/sub2api/pull/3808) 在本次快照仍是
  `OPEN + DIRTY`，不能写成官方已采用方案，也不应直接整块引入。
- 类似“429 冷却后变成 503”的
  [Issue #2258](https://github.com/Wei-Shaw/sub2api/issues/2258) 仍开放；对应
  [PR #2290](https://github.com/Wei-Shaw/sub2api/pull/2290) 已关闭且未合并。
- 没有找到专门修复 Claude Code synthetic
  `API Error: The operation timed out.` 的官方 commit。

### 4.3 不能直接 cherry-pick 的原因

本 fork 当前没有官方新增的两个 count-token 文件，而且本地网关路由、scheduler 和
Messages bridge 已大幅分叉。对官方 #3497 做只读 `git apply --check` 时，路由、测试、
endpoint URL helper 和 gateway service 均无法直接应用。

更重要的是语义不同：官方 handler 使用普通 OpenAI selector；本 fork 必须要求 bridge
开关、Antigravity group 绑定和显式 Claude 到 GPT 非 self mapping，否则可能选择普通
OpenAI 账号或错误模型。

## 5. 修复优先级

| 优先级 | 工作 | 原因 |
|--------|------|------|
| P0 | 分离 bridge 路由意图和瞬时可用性，禁止限流后切 native | 直接修复本次最新 timeout 的高置信度链路 |
| P0 | 保留 bridge 内多账号 failover，并为无可用 bridge 返回正确 429/503 | 让客户端得到稳定且可解释的错误语义 |
| P1 | 适配官方 OpenAI `count_tokens` + OAuth/local tokenizer fallback | 修复独立兼容性缺口，减少 native 空池 503 |
| P1 | 对齐官方已合并的 Messages failure/terminal 语义 | 防止回归为 2xx 空回复，同时保留本地 compact recovery |
| 运维 | 增加至少两个可用 bridge 账号并完善路由观测 | 降低单账号额度耗尽影响，但不能替代代码修复 |

## 6. P0 设计：严格 bridge 路由

### 6.1 新的状态模型

建议新增 `backend/internal/service/openai_claude_gpt_bridge_routing.go`：

```go
type ClaudeGPTBridgeRouteState string

const (
    BridgeNotConfigured ClaudeGPTBridgeRouteState = "not_configured"
    BridgeReady         ClaudeGPTBridgeRouteState = "ready"
    BridgeRateLimited   ClaudeGPTBridgeRouteState = "rate_limited"
    BridgeUnavailable   ClaudeGPTBridgeRouteState = "unavailable"
    BridgeProbeError    ClaudeGPTBridgeRouteState = "probe_error"
)

type ClaudeGPTBridgeRouteDecision struct {
    State            ClaudeGPTBridgeRouteState
    CandidateCount   int
    SchedulableCount int
    RateLimitedCount int
    RetryAt          *time.Time
    Reason           string // internal enum; never expose account details
}
```

这是设计草案，实际实现可以调整命名，但必须保留“未配置”和“已配置但动态不可用”的
结构化区别。

### 6.2 bridge 配置候选

候选必须同时满足：

- `account.platform == openai`；
- 账号为 active，并绑定当前 group；
- `extra.openai_claude_gpt_bridge_enabled == true`；
- `credentials.model_mapping` 对请求 Claude model 有显式命中；
- mapped model 非空且不同于原 request model。

诊断可以复用 `AccountRepository.ListByGroup()`。它能读取仍是 active、但已被
`RateLimitResetAt`、`OverloadUntil` 或 `TempUnschedulableUntil` 排除出 scheduler 的
账号，因此不需要数据库迁移或新 repository 接口。

建议分类：

| 条件 | State | HTTP 行为 |
|------|-------|-----------|
| 没有配置候选 | `not_configured` | 保持现有 native Antigravity 路由 |
| 至少一个候选当前可调度 | `ready` | 进入 bridge handler，由真实 scheduler 最终选号 |
| 候选全部只被未到期 `RateLimitResetAt` 阻断 | `rate_limited` | 429 Anthropic `rate_limit_error`，附 `Retry-After` |
| 已配置候选被 overload、临时暂停、过期、quota 等阻断 | `unavailable` | 503 Anthropic `api_error`/`overloaded_error` |
| 诊断查询失败 | `probe_error` | 503；记录内部原因，不切 native |
| JSON 非法、model 缺失 | 不进入状态分类 | 由 Anthropic handler 返回规范 400，不伪装成 native miss |

### 6.3 路由规则

`backend/internal/server/routes/gateway.go` 的目标流程：

```text
ResolveClaudeGPTBridgeRoute()
  -> not_configured: Gateway.Messages (native)
  -> ready: OpenAIGateway.MessagesClaudeGPTBridge
  -> rate_limited: 429 + Retry-After
  -> unavailable: 503
  -> probe_error: 503
```

预检只诊断，不再获取并释放 scheduler selection slot。这样可移除当前“预检选号一次、
handler 再选号一次”的双调度，也减少两次选择之间的竞态。

### 6.4 bridge handler 规则

`backend/internal/handler/openai_gateway_handler.go` 应：

1. 删除 `markOpenAIClaudeGPTBridgeFallback()`、
   `ClaudeGPTBridgeFallbackRequested()` 和对应 context key。
2. 删除首次选号 error/nil/mapping race 时的隐藏 native fallback 分支。
3. 如果 route 已判定存在 bridge 意图，后续所有错误都留在 bridge 语义内。
4. 首次真实选号失败时重新诊断一次，覆盖“preflight ready，随后刚好被 429 冷却”的
   竞态：纯限流返回 429，其他动态不可用返回 503。
5. 已有多个 bridge 账号时继续 failover；第一个上游 429 后尝试下一个 eligible
   bridge 账号。
6. 所有尝试都因 429 失败时，以最后一个 `UpstreamFailoverError` 为主，返回 429，
   并尽量保留经过校验的 reset/retry metadata。
7. 客户端取消不写 account failure，不触发额外冷却，也不在 terminal 后追加错误。

### 6.5 `Retry-After`

- 使用候选中最早的未来恢复时间；
- 秒数向上取整，最小为 1；
- 不返回过去时间或负数；
- 响应不暴露账号 ID、账号名、配额明细或凭据；
- 推荐响应：

```json
{
  "type": "error",
  "error": {
    "type": "rate_limit_error",
    "message": "Upstream rate limit exceeded, please retry later"
  }
}
```

服务器能保证错误分类和 `Retry-After` 正确，但不能保证 Claude Code 最终一定显示
429 文案；客户端耗尽自己的重试预算后仍可能显示通用 timeout。修复目标是停止把
bridge 容量问题伪装成无关的 native 503。

## 7. P1 设计：bridge-aware `count_tokens`

### 7.1 推荐方案

以官方 `upstream/main` 当前版本为基线，手工移植并适配：

- `backend/internal/handler/openai_gateway_count_tokens.go`；
- `backend/internal/service/openai_gateway_count_tokens.go`；
- `/v1/responses/input_tokens` endpoint helper；
- OAuth 401/403/404 missing-scope/unsupported 检测；
- `github.com/tiktoken-go/tokenizer v0.8.0` 本地估算；
- 官方现有 request header override、body parse 和 lenient body cap 修正。

不能只移植最初的 #3497；必须包含 #3635 之后的当前实现，避免 OAuth count 请求把
健康账号错误冷却或下线。

### 7.2 本 fork 路由适配

| 入站请求 | 目标行为 |
|----------|----------|
| OpenAI group `/v1/messages/count_tokens` | 官方 OpenAI count handler |
| Antigravity group，存在该 model 的显式 bridge mapping | bridge-aware OpenAI count handler |
| Antigravity group，没有 bridge mapping | 保持现有 native `Gateway.CountTokens` |

bridge count handler 必须：

- 使用与 Messages 相同的 bridge 资格和显式 mapping 契约；
- 选中账号后调用 `ResolveClaudeGPTBridgeModel(requestModel)`；
- API key upstream 支持时调用 `/v1/responses/input_tokens`；
- OAuth scope/404 不支持或 bridge 账号暂时不可用时，使用同一转换后的 Responses input
  做本地 tokenizer 估算；
- 不选择未启用 bridge 或没有显式 mapping 的普通 OpenAI 账号；
- 不占用户/账号并发槽；
- 不写 usage row、不扣费、不修改 `actual_cost`；
- unsupported count endpoint 不应把账号永久标错。

最小替代方案是明确返回官方兼容 404，让 Claude Code 使用自己的本地估算；该方案
风险较低，但会保留服务端计数缺口。本方案推荐完成官方语义适配，404 仅作为实现
期间的可控降级。

## 8. 与已完成空回复/compact 加固的关系

P0 严格路由不替代当前工作区已有的 Messages/compact 修复，也不应回滚它们。后续
实现要保留以下不变量：

- 非可见 preamble/thinking 在确认有 text/tool output 前不得提交语义流状态；
- `response.failed`、incomplete、missing terminal 和 reasoning-only 不能变成
  `message_stop/end_turn` 成功空响应；
- 首个可见字节之前允许账号 failover；可见内容已经写出后不能在另一账号重放整次请求；
- terminal `response.output` 中独有的 text/tool arguments 必须补发完整 Anthropic SSE
  生命周期；
- `context_length_exceeded` 保持明确非重试 4xx；
- keepalive transport output 与 visible semantic output 分开统计；
- 客户端取消传播到 detached compact recovery，但不惩罚账号；
- 成功和错误 terminal 都要防止 handler/panic fallback 追加第二个 terminal。

需要逐项对照官方已合并的 #3859、#3853、#3051、#3870/#3873，只吸收缺失的
语义，不用大范围 upstream merge 覆盖本地 compact recovery、bridge billing、display
token 和 cache 行为。

## 9. 计划修改的文件

### P0 strict routing

- 新增 `backend/internal/service/openai_claude_gpt_bridge_routing.go`
- 修改 `backend/internal/handler/openai_gateway_handler.go`
- 修改 `backend/internal/server/routes/gateway.go`
- 视实现需要补充 `backend/internal/service/openai_account_scheduler.go`
- 扩展 handler、route、scheduler 单元测试

### P1 count tokens

- 新增 `backend/internal/handler/openai_gateway_count_tokens.go`
- 新增 `backend/internal/service/openai_gateway_count_tokens.go`
- 修改 `backend/internal/service/openai_endpoint_url.go`
- 修改 `backend/internal/server/routes/gateway.go`
- 修改 `backend/go.mod`、`backend/go.sum`
- 新增 handler/service/route count-token 测试

### 文档

- `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md`
- `docs/dev/codebase/gateway.md`
- `docs/dev/CHANGELOG_CUSTOM.md`
- 本文档

初版不需要数据库迁移、前端修改或新的 Wire provider。若实现过程中引入新 service
构造器，才需要同步 `wire.go` 和 `wire_gen.go`。

## 10. 测试矩阵

| 场景 | 预期 |
|------|------|
| 未开启 bridge 或没有该 model 显式 mapping | 使用 native，request body 完整 |
| 一个可用 bridge 账号 | 使用 bridge，native handler 调用次数为 0 |
| 唯一 bridge 账号 `RateLimitResetAt` 未到期 | 429 + 正数 `Retry-After`；不调用 native 或上游 |
| 两个 bridge，一个限流、一个可用 | 选择可用账号 |
| 多个 bridge 全限流，恢复时间不同 | 429，`Retry-After` 指向最早恢复账号 |
| 限流和 temp-unschedulable 混合 | 503，不错误声明为纯 rate limit |
| preflight ready，真实选号前进入限流 | 二次诊断后 429，不回落 native |
| 首个 bridge upstream 429，第二个可用 | bridge 内 failover 成功 |
| 所有 bridge upstream 429 | 最终仍为 429，保留可用 reset metadata |
| bridge 选中后 mapping 被删除 | 当前请求返回 bridge 侧错误，不半途切 native |
| 非法 JSON、缺失 model | 400 |
| preflight 阶段失败的 streaming 请求 | 普通 JSON 429/503，不提前启动 SSE |
| SSE 已开始后失败 | 合法 Anthropic `event: error` |
| OpenAI count upstream 200 | 返回 `{"input_tokens":N}` |
| OAuth input_tokens 401/403 missing scope 或 404 | 本地估算 200，不标错账号 |
| bridge 账号全部限流时 count_tokens | 本地估算，不调用 native 池 |
| native group count_tokens | 原流程不变 |
| count_tokens side effects | 无 usage、扣费和并发槽变化 |
| manual compact | `compact_boundary` 成功，压缩后 canary turns 正常 |
| 成功 terminal 但无可见输出 | 不允许 2xx 空成功；按流状态 failover 或返回错误 |
| client cancel | 不惩罚账号，不产生重复 terminal |

最关键的两请求回归：

1. mock upstream 让第一次真实 bridge 请求返回 429，并写入唯一账号的限流状态；
2. 限流窗口内立即发第二次相同路由请求；
3. 第二次必须返回 429，断言 native handler、native scheduler 和 OpenAI upstream
   均未被调用。

建议验证命令：

```powershell
cd backend
go test -tags=unit ./internal/service -run ClaudeGPTBridge -count=1
go test -tags=unit ./internal/handler -run "ClaudeGPTBridge|CountTokens" -count=1
go test -tags=unit ./internal/server/routes -run Gateway -count=1
go test -tags=unit ./internal/pkg/apicompat -count=1
go test -tags=unit ./internal/service ./internal/handler -count=1
git diff --check
```

## 11. 日志和观测

新增结构化事件 `openai_claude_gpt_bridge.route_decision`：

- `request_id`
- `client_request_id`
- `group_id`
- `requested_model`
- `state`
- `candidate_count`
- `schedulable_count`
- `rate_limited_count`
- `retry_at`
- `decision_source=preflight|selection_race`
- `attempt`
- `native_fallback`
- `terminal_outcome`
- `latency_ms`

count path 增加 `count_tokens_estimated` 和 estimate 原因。禁止记录请求 body、metadata
user ID、完整 session hash、账号名、凭据、token 或完整上游响应。

观测目标：

- `bridge configured + native_fallback=true` 必须为 0；
- 429 后下一请求变成 native 503 的次数必须为 0；
- 2xx 且无可见 content/tool output 的 Messages terminal 必须为 0；
- count-token unsupported/scope fallback 可计数，但不计为 account health failure；
- 观察 route decision p95 和 `ListByGroup` 查询耗时，防止诊断增加明显热路径延迟。

## 12. 发布与回滚

当前工作区包含大量其他未提交修改，且本 fork 相对 upstream 高度分叉。后续实现应：

1. 创建干净 worktree/分支，不在当前混合 WIP 上直接套官方 patch；
2. 先写失败测试，再实现 P0 strict routing；
3. P0 通过后单独提交；
4. P1 count_tokens 作为第二个独立提交；
5. stream/official semantic 对齐和文档作为第三个逻辑提交；
6. 在“唯一 bridge 账号、无 native 账号”的 canary group 注入 429 验证；
7. 再验证双 bridge failover、manual compact 和 post-compact canary；
8. 通过 GitHub Actions 构建并确认目标 GHCR image digest 后再部署。

部署和 push 仍需要当次用户明确授权。回滚只需恢复上一 GHCR image digest；本方案
不修改 schema，也不需要清理已有 `RateLimitResetAt` 数据。

第二个 bridge 账号是推荐的运维冗余，但不是回滚或修复方案。即使有多个账号，
“全部临时不可用时误切 native”的状态机仍必须修复。

## 13. 验收标准

- 唯一 bridge 账号限流后的所有重试不再出现 native 空池 503；
- 限流窗口内稳定返回 Anthropic 429 和有效 `Retry-After`；
- 限流到期后无需人工操作即可恢复 bridge 调度；
- 首个 bridge 429、第二个 bridge 可用时 failover 成功；
- bridge-configured 请求不会因动态容量状态改变平台或映射模型；
- `count_tokens` 返回上游计数或本地估算，不依赖不存在的 native 池；
- `count_tokens` 不产生 usage、费用或并发副作用；
- 不出现 2xx 成功但无可见 content/tool output 的终止；
- manual compact 和压缩后 canary 对话通过；
- 未配置 bridge mapping 的 group 保持原 native 行为；
- canary route p95 没有持续超过基线 5%；
- 日志中不出现请求内容、凭据或账号敏感信息。

## 14. 后续开工入口

下一次继续时按以下顺序执行：

1. 重新确认当前分支和 worktree，避免覆盖本轮及其他用户 WIP；
2. 从“429 后立即重试不得调用 native”两请求回归开始；
3. 实现纯配置/状态诊断器；
4. 改 route dispatch；
5. 删除 handler 隐藏 native fallback；
6. 补齐 Retry-After、竞态二次诊断和 cancellation 测试；
7. 完成 P0 局部验证后再进入 count_tokens；
8. 以官方当前 count-token 文件为基线手工适配，不直接 cherry-pick；
9. 完成全量相关 unit、manual compact 和 canary 验证；
10. 更新本文“状态”和实际文件/测试结果，再提交。

## 15. 相关文件

- `backend/internal/server/routes/gateway.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/no_account_error.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/account.go`
- `backend/internal/repository/account_repo.go`
- `backend/internal/service/openai_gateway_messages.go`
- `backend/internal/service/openai_gateway_messages_compact.go`
- `backend/internal/service/openai_gateway_messages_empty_output_test.go`
- `backend/internal/service/openai_gateway_messages_compact_test.go`
- `backend/internal/handler/openai_gateway_handler_test.go`
- `docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_2026-06-02.md`
- `docs/dev/codebase/gateway.md`
