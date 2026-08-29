# 智能调度策略开发手册

> 选号模块的入口地图。人与 AI 先读本文知道**改哪、禁区、去哪看代码**，算法与阈值以源码和相邻 spec 为准，本文不复述。
>
> 正文中文，标识符英文。来源只标到机制级：`上游自带` / `本仓二开` / `叠层`。
>
> 账号 CRUD、质量格 UI 仍归 [account.md](./account.md)。网关协议/转发仍归 [gateway.md](./gateway.md)。字段级合同仍归 `.trellis/spec/backend/` 已有 scheduling spec。

## 1. 怎么用

- 改选号、粘性、failover、snapshot、用户准入：先读本文，再打开对应源文件，不要从 `account.md` / `gateway.md` 拼装热路径。
- 同步窗口：`上游自带` 可对照上游骨架；`叠层` / `本仓二开` 禁止整文件用上游或旧 catchup 覆盖。
- 管理页 UI、PnL 读 API 不在本文展开。PnL 见 `.trellis/spec/backend/schedule-pnl.md`；选号热路径不读它。

## 2. 模块边界

本模块是选号与准入：从分组池挑一个可转发的 `Account`，占槽，失败则排除再选。

不是上游协议转换、SSE 组包、客户端入站契约。

选号倍率字段是 `accounts.upstream_rate_multiplier`（`EffectiveUpstreamRate()`）。缺省与比较规则看 `account.go`，本文不写数值。

## 3. 来源总表

| 标签 | 同步时 |
|------|--------|
| 上游自带 | 可对照骨架；不要删本仓叠在其上的过滤 |
| 叠层 | 上游骨架 + 本仓行为。禁止整文件替换 |
| 本仓二开 | 上游没有对等合同。同步时必须保留 |

| 机制 | 标签 | 去哪读 |
|------|------|--------|
| 分组 / 平台 / `IsSchedulable()` / 基础负载 | 上游自带 | `account.go` `IsSchedulable` |
| session / `previous_response` 粘性 | 上游自带 | `openai_account_scheduler.go` `Select`；`gateway_cache.go` |
| Claude/Gemini layered filter | 上游自带 | `gateway_service.go` `SelectAccountWithLoadAwareness` |
| Antigravity 混合调度 | 上游自带 | `isAccountAllowedForPlatform`；`extra.mixed_scheduling` |
| snapshot / outbox | 叠层 | `scheduler_snapshot_service.go`；`scheduler_cache.go` `buildSchedulerMetadataAccount` |
| OpenAI 高级调度（打分 / TopK / sticky escape / 订阅优先） | 叠层 | `openai_account_scheduler.go`；Settings `openai_advanced_scheduler_*` |
| Spark 影子选号隔离 | 叠层 | `shadow_routing.go` |
| 能力门、Images、Grok 隔离 | 本仓二开 | `isOpenAIAccountEligibleForScheduleRequest`；`SelectAccountWithSchedulerForImages` |
| 用户智能调度闭包池 | 本仓二开 | `admitsScheduleUser`；`smart_schedule_lookup_platform.go`；`account-user-schedule.md` |
| 账号 allow/deny/pair-cap/quality-gate | 本仓二开 | `account_schedule_users`；`AllowsScheduleUser` / `AdmitsScheduleUser` |
| 质量 last-N / 硬关闭 / 立即恢复 | 本仓二开 | `account-quality-*.md`；不要和 pair 质量、ops 口径混用 |
| 公共调度账号质量 | 本仓二开 | 搜 `public-schedule` / `preferPublicScheduleAccounts`；站点总闸在账号管理页；列表 `quality_ttft` 带 K/C，`public_quality` 是可手切六态；不进 `IsSchedulable()` |
| OAuth 车队软 429 | 本仓二开 | `oauth-fleet-soft-429.md`；`MergeOAuthFleetSoft429Exclusions` |
| `fallback_only` 硬分区 | 本仓二开 | `IsFallbackOnly`；`preferPrimary*` |
| 粘性 overflow / 选号倍率回跳 | 本仓二开 | `account_unpooled_schedule.go` |
| 调度 PnL 管理面 | 本仓二开 | `schedule-pnl.md`（只点名） |

## 4. 调用链（只到入口）

```text
路由 → Gateway / OpenAI / Gemini handler
  → SelectAccount* / SelectAccountWithLoadAwareness / selectAccountWithScheduler
  → snapshot 读池
  → 硬门槛 → 软排序 → 占槽
  → 失败排除再选
```

生产准入走 `admitsScheduleUser`（`account_user_schedule.go`）。不要走遗留的单独 `AllowsScheduleUser`。

| 入口 | 符号 |
|------|------|
| Claude/Anthropic | `GatewayService.SelectAccountWithLoadAwareness` |
| OpenAI 开高级调度 | `defaultOpenAIAccountScheduler.Select` |
| OpenAI 关高级调度 | `selectAccountWithLoadAwarenessForSchedule` |
| Images | `SelectAccountWithSchedulerForImages` |
| Gemini | `GeminiMessagesCompatService.admitsScheduleUser` |
| 粘性读写 | `BindStickySession` / `bindStickySessionAfterSelect` |

## 5. 原则（细节在代码里）

顺序和阈值以当前实现为准，不要在文档里重排或抄公式。

- `IsSchedulable()` 只表示账号自身能否进池。用户策略、闭包池、软 429 **不得**折进去。
- 用户准入统一走 `admitsScheduleUser`。闭包池启用且非空时是闭包 allow-list；细则读 `account-user-schedule.md`。
- `fallback_only` 是硬分区：还有非兜底候选时不选兜底号。实现看 `preferPrimary*`。Anthropic 模型路由层是否覆盖，以代码为准。
- pair 占满只排除再选，不 WaitPlan。账号并发满才可能 WaitPlan。
- 软 429 是请求级排除，不是不可调度。硬亲和规则看 `oauthFleetSoft429HasHardAffinity`；不要把「有 sessionHash」当成硬亲和。
- `previous_response` 不因更便宜 peer 或公共调度质量被清。粘性 overflow / 倍率回跳只读 `account_unpooled_schedule.go`。公共调度 session sticky 只在 **demoted 且存在未 demoted peer** 时清（`shouldEscapeSessionStickyForPublicQuality`）；Gemini 走 `shouldEscapeGeminiStickyForPublicQuality`，不得只看 `IsDemoted`。LoadBatch 关闭时的 Anthropic/OpenAI 遗留选号也走 `preferPublicScheduleAccounts`。
- snapshot meta 缺字段只让该字段变空，不要把受限号看成全开。要拷哪些字段，以 `buildSchedulerMetadataAccount` / `filterSchedulerExtra` 为准。
- 质量、ops、pair 冷却是不同口径。混用前先读对应 spec。

## 6. 改一处要覆盖的面

加排除层或打分因子时：打开上表入口，**每条选号路径**走同一准入；用户范围放 `admitsScheduleUser`，账号全局才动 `IsSchedulable()`；字段进 snapshot 就改 `scheduler_cache.go`。

高级调度与 load-awareness 回退要一起改。同步禁止整文件替换 `openai_account_scheduler.go` 和 gateway 选号。

## 7. 禁止

- 把 allow/deny/gate/cap/闭包池折进 `IsSchedulable()`
- 把 `fallback_only` 和 primary 混在同一选择/等待池
- 为更便宜 peer 或公共调度质量清 `previous_response`
- 只因 `IsDemoted` 清 Gemini/session sticky（必须还有健康 peer）
- 软 429 写成 `rate_limit_reset_at` / `temp_unschedulable_until`
- 整文件用上游覆盖叠层/二开热路径
- pair-full 走 WaitPlan
- AG 关闭时 fail-open 到账号侧
- 把 ops `needs_ops_attention` 当成调度排除

## 8. 回归

改选号先跑 `backend/internal/service` 下已有 `*scheduler*`、`*smart_schedule*`、`*user_schedule*`、`*fallback_only*`、`*oauth_fleet_soft_429*` 测试，以及 `user_smart_schedule_selection_sim_test.go`。不要在本文复制用例。

管理面只改契约时看 admin handler / Users 智能调度页；字段合同仍归原 spec。
