# 智能调度策略开发手册

> 选号热路径的策略真源。人与 AI 改账号选择、粘性、failover、snapshot、智能调度准入前先读本文，再读相邻 Trellis 细 spec。
>
> 正文中文，标识符英文。每条机制带来源标签：`上游自带` / `本仓二开` / `叠层` / `待核验`。叠层节拆开「上游部分」与「本仓叠加部分」。
>
> 账号 CRUD、质量格 UI 仍归 [account.md](./account.md)。网关协议/转发细节仍归 [gateway.md](./gateway.md)。字段级合同仍归 `.trellis/spec/backend/` 下已有 scheduling spec。

## 1. 怎么用

**人**：改调度先读本文，再按机制跳到细 spec（用户准入、软 429、质量硬关闭、ops 口径）。不要从 `account.md` / `gateway.md` 拼装热路径顺序。

**AI**：改任何 `SelectAccount*` / `SelectAccountWithScheduler*` / sticky / `excludedIDs` / snapshot meta 前必读本文 §5–§10。未读过本仓时，用 §9 回答「加一个新排除层改哪几处」。

**同步窗口**：按 §3 来源标签决定能否对照上游。`上游自带` 可对照上游骨架。`叠层` / `本仓二开` 禁止整文件用上游或旧 catchup 分支覆盖热路径；先读本仓用法再叠行为。`待核验` 禁止猜成确定标签。

权威归属：热路径策略以本文为准。管理页 UI、账号 CRUD、PnL 读 API 不在本文展开。

## 2. 模块边界

本模块是**选号与准入**：从分组池里挑一个可转发的 `Account`，占账号槽与可选 pair 槽，失败则排除再选。

不是上游协议转换、SSE 组包、客户端入站契约。

选号倍率用 `EffectiveUpstreamRate()` / `accounts.upstream_rate_multiplier`：已准入集合里更低者优先。缺字段按类型默认：`oauth` / `apikey` 0.15，否则 1。

调度 PnL 管理面只点名 `.trellis/spec/backend/schedule-pnl.md`。选号热路径不读 PnL。

## 3. 来源总表

标签对同步的含义：

| 标签 | 同步时 |
|------|--------|
| 上游自带 | 可对照上游骨架；不要删本仓叠在其上的过滤 |
| 叠层 | 上游骨架 + 本仓字段/行为。禁止整文件替换该热路径 |
| 本仓二开 | 上游没有对等合同。同步时必须保留 |
| 待核验 | 本仓语义已实现，但来源证据不足。禁止猜成「上游自带」或「叠层」 |

| 机制 | 标签 | 证据入口 |
|------|------|----------|
| 分组 / 平台 / `IsSchedulable()` / 基础负载均衡 | 上游自带 | `README_CN.md` 智能调度；`Account.IsSchedulable()`（`account.go`） |
| `session_hash` / `previous_response_id` | 上游自带 | OpenAI scheduler 三层：`Select`（`openai_account_scheduler.go`）；Redis `sticky_session:{groupID}:{sessionHash}`（`gateway_cache.go`） |
| Claude/Gemini layered filter | 上游自带 | `SelectAccountWithLoadAwareness` Layer 2：`filterByMinUpstreamRate` → `filterByMinPriority` → `filterByMinLoadRate` → `selectByLRU`（`gateway_service.go`）；Gemini `preferOAuth` |
| Antigravity 混合调度 | 上游自带 | `README_CN.md` 混合调度；`extra.mixed_scheduling`；`isAccountAllowedForPlatform` |
| snapshot / outbox | 叠层 | 上游 outbox + 本仓 migration `186_scheduler_outbox_dedup_key.sql` / `187_scheduler_outbox_pending_dedup_key_index_notx.sql`；`buildSchedulerMetadataAccount`（`scheduler_cache.go`） |
| OpenAI 高级调度打分 / TopK / sticky escape / 订阅优先级 | 叠层 | Settings `openai_advanced_scheduler_enabled` 等；`Select` / `selectByLoadBalance`；设计表指向 `UPSTREAM_SYNC.md` layered port（`f26ca5661` + `0fd2e9216`）。本仓核过代码与 Settings KV，未在本次把 SHA 写成新的确定证据 |
| Spark shadow 选号隔离 | 叠层 | 上游 Spark 影子；本仓 `parentHealthyForShadow` / `IsCredentialUsableForShadow` / `QuotaDimensionSpark` |
| 能力门、Images、Grok 隔离 | 本仓二开 | `SelectAccountWithSchedulerForImages`；`isOpenAIAccountEligibleForScheduleRequest` |
| 用户智能调度闭包池 | 本仓二开 | `admitsScheduleUser`；`SmartScheduleLookupPlatform`；`.trellis/spec/backend/account-user-schedule.md` |
| 账号 allow/deny/pair-cap/quality-gate | 本仓二开 | `account_schedule_users`；`AllowsScheduleUser` / `AdmitsScheduleUser` / `QualityGateBlocksUser` / `PairMaxConcurrency` |
| 质量 last-N / 硬关闭 / 立即恢复 | 本仓二开 | `account-quality:last-n:{id}`；`SetTempUnschedulable` reason `quality_hard_close` |
| OAuth 车队软 429 | 本仓二开 | `oauth_fleet_soft_429_settings`；`MergeOAuthFleetSoft429Exclusions`；changelog 2026-08-24 |
| `fallback_only` 硬分区 | 本仓二开 | 本仓提交 `d6c5d191a`（`feat(scheduler): add fallback-only account hard scheduling tier`，作者 541968679）。`git merge-base --is-ancestor d6c5d191a upstream/main` 为否；`upstream/main` 无 fallback-only 提交 |
| sticky overflow + `EffectiveUpstreamRate` 回跳 | 本仓二开 | migration 203；`shouldEscapeSessionStickyForCheaperTier`（`account_unpooled_schedule.go`） |
| 调度 PnL 管理面 | 本仓二开 | 只点名 `.trellis/spec/backend/schedule-pnl.md` |

`fallback_only` 本仓语义：能力过滤之后，若仍有任意非兜底候选，永不选兜底号。见 §6.13。

## 4. 调用链

```text
Gin 路由（gateway / openai / gemini / antigravity）
  → handler（GatewayHandler / OpenAIGatewayHandler / …）
  → SelectAccount* / SelectAccountWithLoadAwareness
    或 SelectAccountWithScheduler* / selectAccountWithScheduler
  → listSchedulableAccounts → SchedulerSnapshotService.ListSchedulableAccounts
  → 硬门槛（§5，不可重排）
  → 软排序（layered filter 或 OpenAI 打分 + TopK）
  → tryAcquireAccountAndPairSlot
  → 失败：excludedIDs / pairFullIDs 再选
```

共享准入函数是 `admitsScheduleUser`（`account_user_schedule.go`；`GatewayService` / `OpenAIGatewayService` / `GeminiMessagesCompatService` 各有一层委托）。生产选号与 sticky 清除不要走遗留的单独 `AllowsScheduleUser` 路径。

| 入口 | 文件 / 符号 |
|------|-------------|
| Claude/Anthropic 负载感知 | `GatewayService.SelectAccountWithLoadAwareness`（`gateway_service.go`） |
| Claude 无负载批 | 同文件：`LoadBatchEnabled=false` 时循环 `SelectAccountForModelWithExclusions` + `tryAcquireAccountAndPairSlot` |
| OpenAI 高级调度 | `OpenAIGatewayService.selectAccountWithScheduler` → `defaultOpenAIAccountScheduler.Select` |
| OpenAI 高级调度关闭 | `getOpenAIAccountScheduler` 返回 nil → `selectAccountWithLoadAwarenessForSchedule` + 能力再选循环 |
| Images | `SelectAccountWithSchedulerForImages`（native → basic 回退） |
| Grok 进 OpenAI 组 | `RequireGrokOpenAIGroupAccess`；进入 Grok 池时丢掉 `previous_response` |
| Gemini Messages 兼容 | `GeminiMessagesCompatService.admitsScheduleUser` |
| 粘性绑定 | `BindStickySession` / `bindStickySessionAfterSelect`；Redis `sticky_session:{groupID}:{sessionHash}` |

失败排除再选：请求级 `excludedIDs`（含软 429 合并、本跳失败号）、`pairFullIDs`（本请求 pair 已满，Layer 3 不得再 WaitPlan 该号）。

## 5. 硬门槛顺序

不可重排。与 `gateway.md` Selection priority 对齐，但以代码核过的顺序为准。

共享准入：`isAccountSchedulableForSelection` = `IsSchedulable()` **且** `admitsScheduleUser`。不要把用户策略折进 `IsSchedulable()`。

### 5.1 Claude / Gemini：`SelectAccountWithLoadAwareness`

锚点：`gateway_service.go` `SelectAccountWithLoadAwareness`。

1. 把 OAuth 车队软 429 并入 `excludedIDs`（有硬亲和则跳过）。`mergeOAuthFleetSoft429ExcludedIDs` + `oauthFleetSoft429HasHardAffinity`。硬亲和 = 已有 sticky **binding** 或 `previous_response_id`。仅生成了 `sessionHash` 不是亲和。
2. 请求门：分组 / 平台 / Claude Code fallback group（`checkClaudeCodeRestriction`）/ 渠道定价（`checkChannelPricingRestriction`）。
3. 读池：`listSchedulableAccounts` → `SchedulerSnapshotService.ListSchedulableAccounts`。
4. **Layer 1**（仅 Anthropic 模型路由 ID）：路由列表内过滤 `isAccountSchedulableForSelection`、平台/混合、模型映射、model-scope、quota、window cost、RPM。**今天不跑 `preferPrimaryAccounts`**。
5. **Layer 1.5**：路由范围内 sticky；无路由时走普通 sticky。overflow 可按 §6.14 回跳一次。准入失败则清 sticky 再选。
6. **Layer 2**：对池内账号再滤一遍（同上 + 平台/mixed），然后 **`preferPrimaryAccounts`**，再跳过 pair-full，再软排序：上游倍率 → 优先级 → 负载 → LRU（Gemini 另 `preferOAuth`）。
7. **Layer 3 WaitPlan**：账号并发满可等；**pair-full 永不 WaitPlan**。`LoadBatchEnabled=false` 路径同样循环 `tryAcquireAccountAndPairSlot`，pair-full 只排除再选。

### 5.2 OpenAI：`selectAccountWithScheduler` → `Select` / `selectByLoadBalance`

锚点：`openai_account_scheduler.go`。

1. 同样先合并软 429（硬亲和规则同上）。默认 platform `openai`，除非 `platformOverride`。
2. `openai_advanced_scheduler_enabled` 为关：`getOpenAIAccountScheduler` 返回 nil，走负载感知 + 能力/传输再选循环。
3. 为开：`Select`
   - `previous_response_id`（除非 `stickyWeighted` 且 `PreviousResponseCanMove`）
   - 否则 `session_hash` sticky（除非 `stickyWeighted`，此时粘性变成打分加权）
   - 否则 `selectByLoadBalance`
4. `selectByLoadBalance` 过滤顺序：`excludedIDs` → `IsSchedulable()` → OpenAI-compatible → platform → `admitsScheduleUser` → privacy（`RequirePrivacySet`）→ 能力/传输 → pair-full → compact 拆池 → **`preferPrimaryOpenAICandidates`** → 打分 + TopK + 加权随机；订阅分区若开启则 subscription 先于 regular。占槽 + pair。更贵号 WaitPlan 可按 §6.14 跳过。

`fallback_only` 粘性：若 pin 在兜底号且仍有 primary peer（`hasPrimaryOpenAIPeer`），`previous_response` / session sticky 必须逃逸，不能把多轮钉在兜底号上。

## 6. 机制详解

### 6.1 分组 / 平台 / `IsSchedulable()` / 基础负载均衡 — 上游自带

**行为**：请求先落在 API Key 的 group/platform。池内账号必须 `IsSchedulable()`：active、`Schedulable` 开关、过期自动暂停、`OverloadUntil`、`RateLimitResetAt`、`TempUnschedulableUntil`、API-key quota。未分组 Key 默认 403，除非 Settings `allow_ungrouped_key_scheduling`。

**锚点**：`Account.IsSchedulable()`（`backend/internal/service/account.go`）。负载：`concurrency:account:{id}`；`tryAcquireAccountAndPairSlot`。

**禁止**：把 allow/deny、pair-cap、quality-gate、smart-schedule、软 429 Redis 折进 `IsSchedulable()`。

### 6.2 `session_hash` / `previous_response_id` — 上游自带

**行为**：OpenAI 三层优先 `previous_response_id`（多轮同一账号），再 `session_hash` 粘性，再负载均衡。Redis 值 `{accountID}` 或 `{accountID}|o`（overflow，本仓叠加，见 §6.14）。

**锚点**：`defaultOpenAIAccountScheduler.Select`；`gateway_cache.go` `sticky_session:{groupID}:{sessionHash}`。

**禁止**：把「`sessionHash != ""`」当成硬亲和（软 429 会因此几乎从不排除）。禁止为更便宜 peer 清 `previous_response`。

### 6.3 Claude/Gemini layered filter — 上游自带

**行为**：Layer 2 在硬过滤之后：`filterByMinUpstreamRate` → `filterByMinPriority` → `filterByMinLoadRate` → `selectByLRU`。Gemini 另偏好 OAuth。本仓在该骨架前插入准入、软 429、`fallback_only`、pair-full，但不改这四层过滤器的相对顺序。

**锚点**：`gateway_service.go` Layer 2 循环（约 `preferPrimaryAccounts` 之后）；`scheduler_layered_filter_test.go`。

### 6.4 Antigravity 混合调度 — 上游自带

**行为**：`accounts.extra.mixed_scheduling` 为真时，AG 账号可进入 anthropic/gemini 组。Force-platform 跳过 mixed。AG 组空池可 anthropic passthrough fallback。

**锚点**：`isAccountAllowedForPlatform`；`Account.IsMixedSchedulingEnabled()`。snapshot extra 必须拷贝 `mixed_scheduling`。

### 6.5 snapshot / outbox — 叠层

#### 上游部分

Redis 调度快照避免每次选号打 DB。键前缀：`sched:buckets`、`sched:outbox:watermark`、`sched:acc:`、`sched:meta:`、`sched:active:`、`sched:ready:`、`sched:ver:`、`sched:{group}:{platform}:{mode}:v{n}`、`sched:lock:`。服务：`scheduler_snapshot_service.go`。读池：`ListSchedulableAccounts`。

#### 本仓叠加部分

migration `186_scheduler_outbox_dedup_key.sql`、`187_scheduler_outbox_pending_dedup_key_index_notx.sql`。meta 必须拷贝本仓准入字段（§8）：`UpstreamRateMultiplier`、allow/deny/caps/gates、`fallback_only`、Grok flags。缺字段语义见 §8。禁止把用户策略写进共享 snapshot 后当成 `IsSchedulable()` 的一部分。

### 6.6 OpenAI 高级调度打分 / TopK / sticky escape / 订阅优先级 — 叠层

#### 上游部分

三层选择（previous → session → load_balance）、账号运行时 errorRate/TTFT 统计、TopK + 加权随机避免单号垄断、订阅账号分区（`IsOpenAIChatGPTSubscription`）。关闭开关则回退负载感知。

#### 本仓叠加部分

Settings KV（`domain_constants.go` / `openai_account_scheduler.go`）：

| Key | 默认/作用 |
|-----|-----------|
| `openai_advanced_scheduler_enabled` | 关则 `getOpenAIAccountScheduler` 返回 nil |
| `openai_advanced_scheduler_sticky_weighted_enabled` | 粘性改打分，不再硬钉 |
| `openai_advanced_scheduler_subscription_priority_enabled` | 选号 subscription 先于 regular；WaitPlan 相反 |
| `openai_advanced_scheduler_lb_top_k` | 覆盖 TopK；未设则 config `Gateway.OpenAIWS.LBTopK`，再默认 7 |
| `openai_advanced_scheduler_weight_*` | 覆盖打分权重 |

默认权重（`openAIWSSchedulerWeights`）：Priority 1、Load 1、Queue 0.7、ErrorRate 0.8、TTFT 0.5、Reset 0、QuotaHeadroom 0、Previous 5、SessionSticky 3。

Sticky escape 默认（`openAIStickyEscapeConfig`）：TTFT 15000ms、errorRate 0.5。这是粘性健康逃逸，不是 overflow 廉价回跳。

本仓还在 load_balance 里插入：`admitsScheduleUser`、pair-full、`preferPrimaryOpenAICandidates`、`EffectiveUpstreamRate` 比较、compact 分层、能力/传输门。

**禁止**：关调度后只改高级路径、漏改 load-awareness 回退；同步时整文件替换 `openai_account_scheduler.go`。

### 6.7 Spark shadow 选号隔离 — 叠层

#### 上游部分

OpenAI Spark 影子是独立调度记录，有自己的配额窗口。选号按影子 ID；凭据从母账号解引用。

#### 本仓叠加部分

- `parentHealthyForShadow` / `IsCredentialUsableForShadow`：母账号鉴权/传输冷却会让影子不可用。
- 母账号 **global 429** / 手动 `Schedulable` 开关 **不** 连坐 Spark。
- `QuotaDimensionSpark`。`resolveCredentialAccount` 读 token。
- 必须有显式 Spark mapping；禁止走 default-model fallback。
- 影子不继承母账号 allow/deny/gate/cap；新影子从空规则开始。

**锚点**：`shadow_routing.go`；`openai_account_scheduler_spark_route_test.go`。

### 6.8 能力门、Images、Grok 隔离 — 本仓二开

**行为**：

- Images：`SelectAccountWithSchedulerForImages`；native 无号再 basic。
- Grok：`RequireGrokOpenAIGroupAccess` + `extra.grok_openai_group_access_enabled`（snapshot 必须拷贝）。进入 Grok 池丢掉 `previous_response`。
- 通用：`isOpenAIAccountEligibleForScheduleRequest`（`openai_gateway_service.go`）；transport / compact / capability。

**禁止**：AG 关闭时 fail-open 到账号侧 allow/deny。

### 6.9 用户智能调度闭包池 — 本仓二开

**行为**：用户×platform 启用且池非空 = 闭包 allow-list。池未命中拒绝。池内忽略该 pair 的账号侧 allow/deny/gate/cap。查找平台：`SmartScheduleLookupPlatform`。

- 原生 OpenAI 组：始终 `openai`。
- OpenAI 账号出现在 AG 组流量下：仅当 AG 策略启用且有成员时用 `antigravity`；AG 关/空/缺省仍用 `openai`（不要 fail-open 到账号侧）。
- AG 一旦开启，池未命中拒绝，不得回落到 openai 池。

Redis：`smart-schedule:user:{userID}`；cooldown/probe/pin/resume/pair-quality 均带 `{platform}:{accountID}`。pair 槽：`concurrency:account_user:{accountID}:{userID}:{platform}`（空 platform `_`，90s live window）。

**锚点**：`admitsScheduleUser`；`smart_schedule_lookup_platform.go`。细合同：`.trellis/spec/backend/account-user-schedule.md`。

**禁止**：把闭包池折进 `IsSchedulable()`；把用户策略写进共享 snapshot 当全局不可调度。

### 6.10 账号 allow/deny/pair-cap/quality-gate — 本仓二开

**行为**：`account_schedule_users` 四套独立配置。运行时：deny → allow 未命中 → 可选 quality-gate → pair-cap N≥1 → 默认不限制（只受账号+用户全局并发）。`userID=0` 且存在任一规则 → fail-closed。

quality-gate 只排除该 pair（清 sticky 再选），不改 `schedulable` / `TempUnschedulableUntil`。pair-full 排除再选，永不 WaitPlan。

**锚点**：`AllowsScheduleUser` / `AdmitsScheduleUser` / `QualityGateBlocksUser` / `PairMaxConcurrency`。生产选号用 `admitsScheduleUser`（先闭包池，否则账号侧）。

### 6.11 质量 last-N / 硬关闭 / 立即恢复 — 本仓二开

两条口径不要混：

| 口径 | 作用 | 不是 |
|------|------|------|
| Track A last-N \(Q_a\) `account-quality:last-n:{id}` | 账号格 + 账号侧 quality-gate + 硬关闭 | pair cooldown |
| pair \(Q_{a,u}\) `smart-schedule:pair-quality:{platform}:{accountID}` | 闭包池质量 / 冷却 | Track A |
| ops `needs_ops_attention` | 告警 | 调度排除。见 `.trellis/spec/backend/ops-schedule-error-caliber.md` |

硬关闭：opt-in，`SetTempUnschedulable` reason `quality_hard_close`。进入 `IsSchedulable()` 的 temp-unsched，不是用户策略。立即恢复：HASH `account-quality:resume:{id}`（Track A），与 `smart-schedule:resume:`（闭包池豁免期）隔离。

缓存未命中 fail-open。5 分钟 tick 不得用 15 分钟 SQL 窗口 `Replace` live。

### 6.12 OAuth 车队软 429 — 本仓二开

**行为**：OAuth/setup-token 的短 429 = 本请求 failover + Redis `oauth-soft-429:{accountID}` 排除。不写 `rate_limit_reset_at` / `temp_unschedulable_until`。空/坏 KV = OFF。账号 `extra.oauth_fleet_soft_429` 三态（true 可金丝雀）。

**锚点**：`MergeOAuthFleetSoft429Exclusions`；`oauthFleetSoft429HasHardAffinity`。细合同：`.trellis/spec/backend/oauth-fleet-soft-429.md`。

**禁止**：软 429 当成不可调度；`sessionHash != ""` 当硬亲和；对 OAuth 开 `IsPoolMode()`。

### 6.13 `fallback_only` 硬分区 — 本仓二开

来源：本仓提交 `d6c5d191a`（作者 541968679）。该提交不是 `upstream/main` 祖先；上游 `main` 无同名硬分区提交。同步时必须保留，禁止被上游覆盖。

**本仓语义**（必须遵守）：

- 键：`accounts.extra.fallback_only`（`AccountExtraFallbackOnly`）。`IsFallbackOnly()`。默认 false。
- 能力过滤之后：`preferPrimaryAccounts` / `preferPrimaryOpenAICandidates`。只要还剩任意非兜底候选，**永不**选兜底号。
- 全部剩余候选都是兜底时，才用兜底池。
- sticky / `previous_response` 钉在兜底号上时，若 `hasPrimaryOpenAIPeer`，必须逃逸。
- 软优先级不能表达「只在别人都不可用时才用」。
- **已知缺口**：Anthropic **Layer 1 模型路由**今天给路由 ID 排序时**不**跑该分区。Layer 2 与 OpenAI scheduler 会跑。

**锚点**：`account.go` `IsFallbackOnly`；`openai_account_scheduler.go` `preferPrimary*`；`account_fallback_only_test.go`。

### 6.14 sticky overflow + `EffectiveUpstreamRate` 回跳 — 本仓二开

**行为**：`accounts.upstream_rate_multiplier`（migration 203）只影响选号。缺字段按类型默认：`oauth`/`apikey` 0.15，否则 1。旧 snapshot 缺字段同样用类型默认。

已准入集合内先比更低 `EffectiveUpstreamRate()`，同倍率再走 priority/load/LRU 或 OpenAI 打分。

Session sticky 默认保 pin。Redis `{accountID}|o` 表示 overflow（本会话被迫用了更贵号）。overflow 可**回跳一次**：存在更便宜、已准入、非 pair-full、`LoadRate<100` 的 peer。新 pin overflow=false，不再追更便宜。`previous_response` **永不**因更便宜 peer 被清。

闭包池廉价逃逸必须用 `SmartScheduleLookupPlatform`，不要用 `group.Platform`。

**锚点**：`EffectiveUpstreamRate`；`shouldEscapeSessionStickyForCheaperTier`；`escapeSessionStickyIfCheaperTier`。

### 6.15 调度 PnL 管理面 — 本仓二开

管理面见 `.trellis/spec/backend/schedule-pnl.md`。选号热路径不读 PnL。

## 7. Redis 与 Settings 目录

### Redis

| Key | 用途 |
|-----|------|
| `sticky_session:{groupID}:{sessionHash}` | 粘性；值 `{id}` 或 `{id}\|o` |
| `concurrency:account:{id}` | 账号槽 |
| `concurrency:account_user:{accountID}:{userID}:{platform}` | pair 槽；空 platform `_`；90s live |
| `oauth-soft-429:{accountID}` | 软 429 排除 |
| `account-quality:last-n:{id}` / `live:{id}` / `resume:{id}` / `precheck:{id}` | Track A 质量 |
| `smart-schedule:user:{userID}` | 用户闭包池 bundle |
| `smart-schedule:cooldown\|soft-cool\|probe\|pinned\|resume\|cooldown-hard\|pair-quality:{platform}:{accountID}` | 闭包池运行时（HASH 字段 `u:{userID}`） |
| `smart-schedule:pair-quality-trend\|events:{platform}:{accountID}:{userID}` | pair 质量序列 |
| `sched:buckets` / `sched:outbox:watermark` / `sched:acc:` / `sched:meta:` / `sched:active:` / `sched:ready:` / `sched:ver:` / `sched:{group}:{platform}:{mode}:v{n}` | snapshot / outbox |
| `temp_unsched:account:{id}` | 临时不可调度缓存 |

### Settings / config

| Key | 用途 |
|-----|------|
| `oauth_fleet_soft_429_settings` | 软 429；空 KV = OFF |
| `quality_hard_close_settings` | 账号硬关闭；默认双关 |
| `schedule_error_whitelist` | 调度 last-N / pair 排除白名单；与 ops attention 不是同一口径 |
| `allow_ungrouped_key_scheduling` | 未分组 Key 能否选号 |
| `openai_advanced_scheduler_enabled` 及 sticky_weighted / subscription_priority / lb_top_k / weight_* | 高级调度 |
| config `Gateway.Scheduling.LoadBatchEnabled` | Claude 负载批 |
| config `Gateway.OpenAIWS.LBTopK` / weights | TopK / 权重底数 |
| config `Gateway.OpenAIScheduler` sticky escape | TTFT / errorRate 逃逸 |

## 8. snapshot / outbox 契约

`buildSchedulerMetadataAccount` + `filterSchedulerExtra`（`scheduler_cache.go`）必须拷贝：

- 身份与容量：ID、Name、Platform、Type、Concurrency、LoadFactor、Priority、RateMultiplier、**UpstreamRateMultiplier**、Status、LastUsedAt、ExpiresAt、AutoPauseOnExpired、Schedulable、限流/过载/temp-unsched、SessionWindow*
- 用户调度：UserScheduleMode、ScheduleUserIDs、**AllowUserIDs / DenyUserIDs / UserConcurrency / UserQualityGates**
- 分组：AccountGroups、GroupIDs
- 过滤后的 Credentials（model_mapping / api_key / project_id / oauth_type / plan_type）
- Extra：`mixed_scheduling`、`window_cost_*`、`max_sessions`、WS flags、**`fallback_only`**、`model_mapping_strict_scheduling`、`grok_openai_group_access_enabled`、Codex quota / auto-pause 键

缺字段：只让该字段变空。旧 snapshot 无 gates = 该字段 fail-open，仍尊重已有 allow/deny/caps。缺 `UpstreamRateMultiplier` → 类型默认（`oauth`/`apikey` 0.15，否则 1）。缺 `fallback_only` → false。

## 9. 改动清单

### 加一个新的选号排除层

未读过本仓时按这里改，且**每条候选路径走同一准入**：

1. **所有选号入口**（缺一条就会漏排除）：
   - `SelectAccount` / `SelectAccountForModel` / `SelectAccountForModelWithExclusions`
   - `SelectAccountWithLoadAwareness`（Claude Layer 1 + Layer 2 + `LoadBatchEnabled=false` 循环）
   - `selectAccountWithScheduler` / `Select` / `selectByLoadBalance` / `selectBySessionHash`
   - `SelectAccountWithSchedulerForImages` / OpenAI-compatible / Grok
   - Gemini `admitsScheduleUser` 委托
   - WS：`SelectAccountByPreviousResponseID` 及 scheduler WS 路径
2. **用户范围**的排除：放进 `admitsScheduleUser`（或其后的 pair 检查），不要放进 `IsSchedulable()`。
3. **账号全局**不可用：才考虑 `IsSchedulable()` / `TempUnschedulableUntil`（硬关闭模式）。软排除用 Redis + `excludedIDs`。
4. 字段要进 snapshot：改 `buildSchedulerMetadataAccount` / `filterSchedulerExtra`。
5. Redis 排除：在 `SelectAccountWithLoadAwareness` 与 `selectAccountWithScheduler` **入口**合并，硬亲和规则与软 429 一致。
6. sticky / `previous_response` 也必须承认该层；pair-full 类排除保 pin，身份/质量类清 pin。
7. 测试至少覆盖：Anthropic Layer 2、OpenAI 高级调度 **和** 关闭回退、Gemini、WS。参考 `user_smart_schedule_selection_sim_test.go`。

### 加一个新的打分因子

1. 默认值：`openAIWSSchedulerWeights`。
2. Settings：`domain_constants.go` + setting_service + admin DTO + `openAIAdvancedSchedulerRuntimeSettings`。
3. 计算：`selectByLoadBalance` 打分循环。
4. 比较：`isOpenAIAccountCandidateBetter` 若该因子要参与并列打破。
5. snapshot 若因子依赖账号字段：§8 必须拷贝。
6. 测试：`openai_account_scheduler_test.go`。Claude layered filter **不要**偷偷吃 OpenAI 专用因子。

## 10. 禁止模式

- 不要把用户策略（allow/deny/gate/cap/闭包池）折进 `IsSchedulable()`。
- 不要把 `fallback_only` 与 primary 混在同一选择/等待池。
- 不要为更便宜 peer 清 `previous_response`。
- 软 429 不是不可调度；不要写 `rate_limit_reset_at` / `temp_unschedulable_until`。
- 同步窗口禁止整文件用上游或旧 catchup 覆盖叠层/二开热路径（尤其 `openai_account_scheduler.go`、gateway 选号、snapshot meta）。
- 生产选号/sticky 清除不要走遗留的单独 `AllowsScheduleUser`。
- pair-full 禁止 WaitPlan / 单独 429。
- AG 关闭禁止 fail-open 到账号侧。
- 禁止把 `sessionHash != ""` 当成硬亲和。
- 禁止把 ops `needs_ops_attention` 与调度 last-N ErrorCount 混成一个口径。

## 11. 回归测试入口

不新写测试。改选号时先跑这些现有文件：

**选号核心**

- `backend/internal/service/openai_account_scheduler_test.go`
- `backend/internal/service/openai_account_scheduler_compact_test.go`
- `backend/internal/service/openai_account_scheduler_spark_route_test.go`
- `backend/internal/service/openai_account_scheduler_ws_snapshot_test.go`
- `backend/internal/service/scheduler_layered_filter_test.go`
- `backend/internal/service/scheduler_shuffle_test.go`
- `backend/internal/service/scheduler_snapshot_hydration_test.go`
- `backend/internal/service/scheduler_snapshot_outbox_cleanup_test.go`

**分区 / 准入 / 闭包池**

- `backend/internal/service/account_fallback_only_test.go`
- `backend/internal/service/account_user_schedule_test.go`
- `backend/internal/service/account_user_schedule_select_test.go`
- `backend/internal/service/admin_account_user_schedule_test.go`
- `backend/internal/service/user_smart_schedule_selection_sim_test.go`
- `backend/internal/service/user_smart_schedule_test.go`
- `backend/internal/service/smart_schedule_lookup_platform_test.go`
- `backend/internal/service/smart_schedule_probe_test.go`
- `backend/internal/service/smart_schedule_pin_test.go`
- `backend/internal/service/smart_schedule_latency_gate_test.go`
- `backend/internal/service/smart_schedule_eval_test.go`
- `backend/internal/service/smart_schedule_soft_cooldown_test.go`
- `backend/internal/service/smart_schedule_pair_quality_test.go`

**overlay / 软 429**

- `backend/internal/service/openai_unpooled_schedule_test.go`
- `backend/internal/service/gateway_unpooled_schedule_test.go`
- `backend/internal/service/account_unpooled_schedule_test.go`
- `backend/internal/service/oauth_fleet_soft_429_test.go`
- `backend/internal/repository/oauth_fleet_soft_429_cache_test.go`
- `backend/internal/handler/admin/setting_handler_oauth_fleet_soft_429_test.go`

**管理面（契约，不替代热路径）**

- `frontend/src/views/admin/__tests__/UserSmartScheduleView.spec.ts`
- `frontend/src/composables/__tests__/smartSchedulePoolAdmission.spec.ts`
- `frontend/src/composables/__tests__/useUserSmartScheduleEditor.spec.ts`
- `frontend/src/components/admin/smart-schedule/__tests__/SmartScheduleAdmissionSwitch.spec.ts`

## 12. 管理面契约

只点名，不写整页 UI。字段级合同仍归原 spec。

| 面 | 锚点 |
|----|------|
| 账号列表 `schedulable` + fallback 开关 | 账号 CRUD；`fallback_only` extra |
| 账号 allow/deny/cap/gate | `allow_user_ids` / `deny_user_ids` / `user_concurrencies` / `user_quality_gates` |
| 账号硬关闭 | `GET/PUT /api/v1/admin/settings/quality-hard-close`；`GET/PUT /api/v1/admin/accounts/:id/quality-hard-close`；`POST /api/v1/admin/accounts/:id/quality-resume` |
| 用户闭包池 | `GET/PUT /api/v1/admin/users/:id/smart-schedule`；`PATCH .../sort-order`；`POST .../copy`；`POST /api/v1/admin/accounts/:id/smart-schedule-resume`；`POST /api/v1/admin/users/smart-schedule/summaries` |
| 软 429 | `GET/PUT /api/v1/admin/settings/oauth-fleet-soft-429`（不进 public settings） |
| 高级调度开关/权重 | Admin settings JSON：`openai_advanced_scheduler_*` |
| 未分组 Key | `allow_ungrouped_key_scheduling` |
| 调度错误白名单 | `schedule_error_whitelist` |
| 调度 PnL | 见 `.trellis/spec/backend/schedule-pnl.md` |
| 上游选号倍率列 | `accounts.upstream_rate_multiplier`（账号/闭包池列表可内联改） |
