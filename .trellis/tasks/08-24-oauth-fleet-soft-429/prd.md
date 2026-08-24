# OAuth fleet 软429：瞬时限流不整号出池

## Goal

多个本地 OAuth 账号像 API Key 池那样轮转。瞬时 429（软限流）只换本请求的号，并记短 TTL 跨请求记忆，**不要立刻把整号打出全局调度池**。窗口真耗尽或鉴权坏了，才写跨请求硬状态。

策略本身必须是 **Admin 可配置**（Settings KV + 网关设置页），账号可三态覆盖；禁止把启用范围、TTL、长 reset 处置写死在 Go 常量里当唯一开关。

## User value

OAuth 车队在瞬时 429 时继续出活：本请求立刻换下一个本地号；短窗口内新请求避开刚撞限流的号；窗口耗尽 / token 坏了时行为与今天一致。出厂全局关；Admin 在网关设置卡片打开总开关后，可改 TTL、平台范围、「长 reset 无硬信号」的软/硬，而不用发版。单号可继承 / 强制开 / 强制关（强制开可在全局关时金丝雀）。

## Background

今天标准 429 会写账号级 `rate_limit_reset_at`（经 `handle429` → `SetRateLimited`），不是 `temp_unschedulable_until`。两者都会让 `IsSchedulable()` 为假，整号离开调度池；Admin 看起来像「临时不可调度」。

一次 OAuth 429 就会持久化账号级 TTL 并 failover。OpenAI / Gemini 无可靠重置头时整号默认 **5 分钟**；Anthropic 无头走有界 fallback（约 **5 秒**）。Codex 窗口未到 100% 的 429 仍可能用很长的 reset（可达数天）。

`tryTempUnschedulable` 在 `handle429` 之前跑（401 除外）。宽的 `temp_unschedulable_rules`（429 + 关键字 “rate limit”）会写临时不可调度，并跳过 Codex / 官方窗口解析。

API Key `credentials.pool_mode`（仅 `IsAPIKeyOrBedrock()`）在 `HandleUpstreamError` 早退：不 `SetRateLimited` / 不 `SetTempUnschedulable`；同账号 `RetryableOnSameAccount` + handler 重试；然后用本请求 `failedAccountIDs` 换号。**不能**把 `IsPoolMode()` 扩到 OAuth：会跳过 OAuth 401 的 token invalidate。OAuth 身份就是限流单位，同一 OAuth 号上重试 429 通常没用。本任务只抄池模式的另一半：**瞬时错误 = 请求排除，不是全局 TTL**。

上一轮规划曾建议「所有 OAuth 默认开、20s Redis、长 reset 无硬信号=软、v1 无 Settings 页」。Brandon 否决写死范围：**这个范围是写死的，能否做成我可以配置的**。本版把旋钮放进现有 Settings KV + Admin UI。Brandon 已拍板：**出厂 Default\* `enabled=false`（空/缺 KV = 关）**；TTL 20s / 长 reset=`soft` / 范围=全部 `IsOAuth()` 仍是 Default\* 值，但只在策略对该号生效后才用。

## Locked decisions

1. 软 / 硬 429 分流。硬：Anthropic 5h/7d 窗口耗尽；Codex 窗口 100%；`INSUFFICIENT_QUOTA` / `USAGE_LIMIT_EXCEEDED`；带硬信号的明确长 reset。硬路径继续 `SetRateLimited` 直到解析出的 reset。软：`rate_limit_exceeded` / blip 429 / 无可靠 reset / 短 Retry-After。软路径**不写** `rate_limit_reset_at` 或 `temp_unschedulable_until`；加入本请求 `failedAccountIDs`，立刻换下一个本地 OAuth 号。
2. 三层排除：① 本请求已有 `failedAccountIDs`；② 新增跨请求短记忆（Redis，账号级软排除，TTL **来自 Settings**，只进调度过滤，不进 `IsSchedulable()`、不落 DB）；③ 跨请求硬状态：仅硬 429、OAuth 401、OpenAI 403 渐进、token 刷新失败、`SetError`。
3. **禁止**把 `IsPoolMode()` 打开给 OAuth。新策略写在 `HandleUpstreamError` / `handle429` 分类里。启用谓词见 design 解析序。
4. **禁止**把池模式的同账号 429 重试抄到 OAuth。
5. 必须保留（产品不变量，**即使 Admin 改软/硬码表也不能关掉**）：OAuth 401 → 临时不可调度 + invalidate token（时长仍走现有 `oauth_401_cooldown_minutes`）；窗口 100% / quota 死亡；403×3 / revoked → `SetError`；Fable 模型范围；OpenAI image 模型范围；Anthropic 官方窗口优先；API Key `pool_mode` 行为不变；计费 / `actual_cost`；pair cap / quality gate 仍在 admission、不进 `IsSchedulable`；sticky / `previous_response` 硬亲和；不要求客户端改 `stream` / 端点 / body。
6. v1 **只做** OAuth 软 429。529、流式超时、quality hard-close 政策都不改。配置面不得把 529 / 流超时收进本策略。
7. 占用 checkout `E:\cursor project\api2sub` 写后续实现。不要开隔离 worktree。不要把 upstream-sync worktree 规则写进本任务文档。不要把当前工作区里无关的 sticky / unpooled 脏改卷进本任务范围。
8. 无 layer-2 短 Redis 时，下一请求会 stampede 回刚 429 的同一 OAuth 号。v1 **必须**做 layer-2。
9. **v1 必须有配置面**（Brandon 否决写死）。全局走 Settings KV JSON 块 + Admin 网关页独立卡片（对齐 529 / 流超时的卡片形态，**不进** `GET /settings/public`）。账号覆盖走 `extra.oauth_fleet_soft_429` 三态。TTL / 长 reset / 范围 / 码表必须落在 `Default*` + KV，Admin 能改；这些 Default\* 值**只在策略对该号生效后**才参与分类。
10. **出厂 Default\* `enabled=false`。空/缺 Settings KV = 策略关**（与 529 过载冷却相反：`DefaultOverloadCooldownSettings().Enabled == true`，空 KV = 开。实现时只抄 529 的 JSON 块 / GET-PUT / 缺省回落机制，**不要抄它的 `Enabled: true`**）。Admin 在网关卡片打开总开关。账号 `extra.oauth_fleet_soft_429=true` 仍对该号金丝雀强制开（须 `IsOAuth()`）。账号 `false` / 缺省 + 全局关（含空 KV）= 现网 `handle429`。

## Requirements

- R1. OAuth 软 429：不改 DB 的 `rate_limit_reset_at` / `temp_unschedulable_until`；本请求 failover 到下一个本地 OAuth 号。
- R2. 软 429 之后，在 **已配置 TTL**（默认 20s，范围 5–300）内：无 sticky / 无 `previous_response` 硬亲和的新请求应避开刚 429 的号；短窗结束后该号重新可调度（`IsSchedulable()` 全程不因这次软 429 变假）。
- R3. Anthropic / Codex 窗口 100%（及硬配额死亡）仍持久化限流到 reset，整号出池。Admin 软码表不能把这类硬信号改成软。
- R4. OAuth 401 仍是临时不可调度 + token invalidate（时长沿用现有 `rate_limit.oauth_401_cooldown_minutes`，本任务不改该旋钮）。
- R5. API Key `pool_mode` 回归不变（含同账号重试与硬驱逐开关）。
- R6. 计费 / `actual_cost` / pair / sticky 头语义不变。
- R7. OAuth 429 的软分类必须发生在宽 `temp_unschedulable_rules` 之前，避免「rate limit」关键字把软 429 写成整号临时不可调度。
- R8. 硬亲和（sticky / `previous_response`）不被 layer-2 软排除强行拆掉；AC2 只约束无硬亲和的新请求。
- R9. 单测覆盖：软 429 不写 DB、本请求换号、短 Redis 排除再恢复、硬窗口仍出池、OAuth 401 不变、`pool_mode` 回归、配置解析序、Settings 校验、不进 public settings。
- R10. Admin 能改全局：总开关、软排除 TTL、长 reset 无硬信号的处置、平台 / 账号类型范围（全 OAuth vs 平台勾选，含 setup-token 是否纳入）。缺省 KV / 非法 JSON 回落到代码 `Default*`（回落机制对齐 529）。**本策略 Default\* `enabled=false`：空 KV = 关。** 529 过载冷却 Default\* `enabled=true`（空 KV = 开）——不要抄错。TTL 20s、`long_reset_policy=soft`、`scope=all_oauth`（含 setup-token）仍是 Default\*，仅在 `enabled` 或账号金丝雀使策略对该号生效后才用。
- R11. Admin 能改全局软/硬 **body 码表**（精神对齐 `custom_error_codes` 的码列表，不是新 DSL）。HTTP 状态在 v1 固定为 429（列表可展示、不可扩到 529）。内置硬信号（R3/R4/Locked 5）优先于码表。
- R12. 账号三态覆盖：`extra.oauth_fleet_soft_429` = `true` / `false` / 缺省。缺省继承全局。禁止写进 `credentials`（OAuth refresh 会整表替换凭据）。禁止因此打开 `IsPoolMode()`。
- R13. 配置解析序：**账号覆盖 → 全局 Settings KV → 代码 Default\***。账号 `true` 可在全局总开关关闭（含空 KV）时对该号金丝雀开启（仍须 `IsOAuth()`）。账号 `false`、以及账号缺省 + 全局关（含空 KV）= 现网 `handle429`。

## Acceptance Criteria

- [ ] AC1. 策略对该号生效且分类为软时：该账号 DB 的 `rate_limit_reset_at` 与 `temp_unschedulable_until` 与请求前相同；本请求换到另一本地 OAuth 号。映射 R1。
- [ ] AC2. 软 429 后，在当时生效的 TTL 内，无 sticky / 无 `previous_response` 的新请求不选刚 429 的号（仍有其它可调度 OAuth 时）；TTL 结束后该号可再被选中，且 `IsSchedulable()` 未因该次软 429 变假。映射 R2、Locked 2。
- [ ] AC3. Anthropic 5h/7d 窗口耗尽、Codex 窗口 100%，仍 `SetRateLimited` 到解析 reset，整号出池；把对应码放进软码表也不能改成软。映射 R3、R11。
- [ ] AC4. OAuth 401 仍 `SetTempUnschedulable` + invalidate token（时长 = 现有 oauth_401 配置）。映射 R4。
- [ ] AC5. API Key `pool_mode`：`HandleUpstreamError` 早退、同账号重试、硬驱逐开关与上线前一致。映射 R5。
- [ ] AC6. 展示计费 / `actual_cost` / pair cap / quality gate / sticky 与 `previous_response` 头语义与上线前一致。映射 R6、R8。
- [ ] AC7. 宽 `temp_unschedulable_rules`（429 + “rate limit”）不能把 OAuth 软 429 写成 `temp_unschedulable_until`。映射 R7。
- [ ] AC8. Admin 网关页可改并持久化 R10/R11 字段；GET 缺省 / 空 KV / 坏 JSON 返回 Default\* 且 **`enabled=false`**；非法 TTL / 枚举 / 平台被拒绝；`GET /api/v1/settings/public` 与 HTML injection **不含**本策略。映射 R10、R11、Locked 10。
- [ ] AC9. `extra.oauth_fleet_soft_429=false` 即使全局开也走现网 `handle429`；`=true` 即使全局关（含空 KV）也对应该 OAuth 号走软路径（仍须 `IsOAuth()`）；缺省键 + 全局关（含空 KV）= 现网 `handle429`；缺省键继承全局（含平台范围 / setup-token）。OAuth token refresh 后 extra 覆盖仍在。映射 R12、R13、Locked 10。

## Out of scope

- 529 政策变更（含用本策略的状态码列表去收 529）
- 流式超时 / 524 / sync-timeout 政策变更
- quality hard-close 政策变更
- 把 `IsPoolMode()` 或同账号 429 重试扩到 OAuth
- 改存储计费 / `actual_cost` / 展示 token 变换
- 要求客户端改 `stream` / 端点 / body
- 把当前工作区无关的 sticky / unpooled 改动并进本任务
- 账号级覆盖 TTL / 码表 / 长 reset（v1 只覆盖启用布尔）
- 把覆盖写进 `credentials` 或新开 `IsPoolMode()`
- 把本策略塞进 public settings / 前端 app-store
- commit / push / 生产部署（除非 Brandon 另批）
- 隔离 worktree / 上游同步 playbook

## 产品不变量（写死，Admin 改不掉）

这些不是「v1 先写死、以后再配」，而是产品底线：

| 不变量 | 说明 |
|---|---|
| OAuth 401 | 仍 invalidate + temp unsched；时长走现有 `oauth_401_cooldown_minutes` |
| 计费 | 不读不写 usage / `actual_cost` / 展示变换 |
| pair / quality | 仍在 admission，不进 `IsSchedulable()`，不复用软排除 Redis 键 |
| sticky / `previous_response` | 硬亲和优先于 layer-2 |
| API Key `pool_mode` | 分支条件与同账号重试不变 |
| 不打开 `IsPoolMode()` 给 OAuth | 否则 401 早退 |
| 不要求客户端改协议 | 入站仍可 `stream:false` Chat Completions |
| Anthropic 官方窗口 / Codex 100% / quota 死亡 | 永远硬，软码表覆盖不了 |

## Open questions

无。Brandon 已拍板（2026-08-24）：**全局 Default\* `enabled=false`；空/缺 Settings KV = 策略关。** Admin 在网关卡片打开。账号 `extra.oauth_fleet_soft_429=true` 仍金丝雀。TTL 20s、`long_reset_policy=soft`、范围 = 全部 `IsOAuth()`（含 setup-token）仍是 Default\*，只在策略对该号生效后才用。
