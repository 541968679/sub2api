# 研究：OAuth 429 现网行为（规划期固化）

来源：Brandon / kayn 已调研事实 + 规划期用 file:line 核对。不是新的全仓猎取。

## 问题

- 标准 429 写 `accounts.rate_limit_reset_at`，路径是 `handle429` → `SetRateLimited`，**不是** `temp_unschedulable_until`。
- 两者都让 `IsSchedulable()` 为假 → 整号离开池。Admin 看起来像「临时不可调度」。
- 一次 OAuth 429 = 持久化账号级 TTL + failover。
- 无可靠头时：OpenAI / Gemini 整号 **5 分钟**；Anthropic 有界 fallback **5 秒**（`defaultRateLimit429FallbackCooldown`）。
- Codex 窗口未到 100% 的 429 仍可能用很长 reset（数天）—— `calculateOpenAI429ResetTime` 在未耗尽时取 max reset。

## 顺序坑

`tryTempUnschedulable` 在 `handle429` 之前（401 除外）。宽 `temp_unschedulable_rules`（429 + “rate limit”）会写 temp unsched，并跳过 Codex / 官方窗口解析。

Anthropic 官方 5h/7d 窗口已经插在 `tryTempUnschedulable` 之前；其它平台没有这层保护。

## 池模式（只抄一半）

API Key `credentials.pool_mode` 仅 `IsAPIKeyOrBedrock()`：

- `HandleUpstreamError` 早退，不 `SetRateLimited` / 不 `SetTempUnschedulable`。
- `RetryableOnSameAccount` + handler 同账号重试。
- 然后本请求 `failedAccountIDs`。

**不要**把 `IsPoolMode()` 扩到 OAuth：会跳过 OAuth 401 token invalidate。  
**不要**把同账号 429 重试抄到 OAuth：OAuth 身份就是限流单位，同号重试 429 通常没用。  
要抄的是：瞬时错误 = 请求排除，不是全局 TTL。

## 必须保留

- OAuth 401 → temp unsched 10m + invalidate token
- 窗口 100% / quota 死亡
- 403×3 / revoked → `SetError`
- Fable 模型范围；OpenAI image 模型范围
- Anthropic 官方窗口优先
- API Key `pool_mode` 不变
- 计费 / `actual_cost`
- pair cap / quality gate 在 admission，不进 `IsSchedulable`
- sticky / `previous_response` 硬亲和
- 不要求客户端改 stream

## v1 不做（政策本身）

529 政策、流式超时政策、quality hard-close 的**行为**不改。v1 = OAuth 软 429 only。

## 配置面（规划修订，Brandon 否决写死）

抄现有 JSON Settings 块，不要新栈：

| 先例 | Key / 路由 / UI |
|---|---|
| 529 | `overload_cooldown_settings` → `/admin/settings/overload-cooldown` → Gateway 卡片自存 |
| 流超时 | `stream_timeout_settings` → `/admin/settings/stream-timeout` → 同 tab |
| 质量硬关闭 | `quality_hard_close_settings` + `extra.quality_hard_close` overlay |

本任务：`oauth_fleet_soft_429_settings` + `/admin/settings/oauth-fleet-soft-429` + Gateway 卡片（529 后）。**不进** public settings。

账号覆盖必须放 **`extra.oauth_fleet_soft_429`**（`boolOverrideFromMap` 三态）。不要放 `credentials`：`persistAccountCredentials` 在 OAuth refresh 时整表 `SetCredentials`。`pool_mode` / `custom_error_codes` / `temp_unschedulable_rules` 能放 credentials，是因为 API Key 不走这条 refresh。

解析序：账号 extra → 全局 KV → `DefaultOAuthFleetSoft429Settings()`。

**出厂默认（Brandon 2026-08-24 锁定）**：`DefaultOAuthFleetSoft429Settings().Enabled == false`。空/缺 Settings KV = 策略 **关**。Admin 在网关卡片打开。账号 `extra.oauth_fleet_soft_429=true` 仍金丝雀。TTL 20s / `long_reset_policy=soft` / 全 OAuth 仍是 Default\* 其它字段，只在策略对该号生效后才用。

**不要抄 529**：`DefaultOverloadCooldownSettings().Enabled == true`（空 KV = 开）。本策略只抄 529 的 JSON 块 / GET-PUT / 缺省回落机制，不抄那个 true。

产品不变量（401 / 计费 / pair / sticky / `pool_mode` / 不打开 `IsPoolMode()`）仍写死。

## 风险

没有 layer-2 短 Redis，下一请求会 stampede 回刚 429 的同一 OAuth 号。

## 锚点

见同任务 `design.md` §2。
