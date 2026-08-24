# Design：OAuth fleet 软429

## 白话

OAuth 号就是限流单位。瞬时 429 不要把整号从 `IsSchedulable()` 里拿掉；本请求换号，并在 Redis 里记短 TTL，避免下一请求立刻打回同一号。窗口耗尽 / quota 死 / token 坏了，才写 DB TTL。

**启用、TTL、长 reset 处置、平台范围、软/硬 body 码** 全部走现有 Settings KV，Admin 网关页可改。出厂 Default\* `enabled=false`（空 KV = 关）；Admin 打开总开关后，其余 Default\*（TTL 20s / `long_reset_policy=soft` / 全 OAuth）才对未覆盖账号生效。单号用 `extra` 三态覆盖。禁止把这些写死成唯一开关。

```text
上游 429（OAuth，解析后策略开）
  → 分类 soft | hard（先于 temp_unsched 规则；内置硬信号 > 码表）
    soft → 不写 rate_limit_reset_at / temp_unschedulable_until
         → 本请求 failedAccountIDs += id
         → Redis SET oauth-soft-429:{accountID} TTL=settings.ttl_seconds
         → 立刻选下一个本地 OAuth
  hard → 现有 SetRateLimited（到解析 reset）/ 官方窗口路径
```

API Key `pool_mode` 不动。不把同账号 429 重试抄到 OAuth。

## 1. 边界

| 在范围内 | 不在范围内 |
|---|---|
| `RateLimitService.HandleUpstreamError` / `handle429` 的 OAuth 429 分类 | `IsPoolMode()` 语义、同账号 429 重试 |
| 调度侧软排除过滤（无硬亲和的新请求） | 529、流式超时、quality hard-close |
| Redis 短 TTL（调度过滤 only；TTL 来自 Settings） | `IsSchedulable()` / Admin「可调度」因软 429 变假 |
| Claude / OpenAI / Gemini / Antigravity / Grok 的 `IsOAuth()`（含 setup-token，受配置约束） | 展示计费、`actual_cost`、pair/quality 进 `IsSchedulable` |
| Settings KV + Admin 网关卡片 + 账号 extra 三态 | public settings / 前端 app-store |
| 单测 + `account.md` / `gateway.md` / CHANGELOG 加法 | 当前工作区无关 sticky / unpooled 脏改 |

工作区：占用 checkout。不要开隔离 worktree。不要把 upstream-sync Workspace 段落抄进实现文档。

## 2. 现状（已核对）

| 事实 | 锚点 |
|---|---|
| `IsSchedulable()` 看 `RateLimitResetAt` **和** `TempUnschedulableUntil` | `backend/internal/service/account.go:213-233` |
| `IsPoolMode()` 仅 `IsAPIKeyOrBedrock()` + `credentials.pool_mode` | `account.go:1453-1465` |
| `IsOAuth()` = `oauth` 或 `setup-token` | `account.go:276-278` |
| 池模式 `HandleUpstreamError` 早退（可硬驱逐） | `ratelimit_service.go:388-397` |
| Anthropic 官方窗口在 `tryTempUnschedulable` **之前** | `ratelimit_service.go:406-417`、`persistAnthropicExhaustedWindowLimit:1381` |
| 其它 429：先 `tryTempUnschedulable`，再 `handle429` | `ratelimit_service.go:420-426`、`539-541` |
| `handle429` 几乎总会 `SetRateLimited` | `ratelimit_service.go:1090-1206` |
| Codex 未到 100% 仍取较长 reset（可数天） | `calculateOpenAI429ResetTime` `1260-1288` |
| Anthropic 无头 fallback ≈ 5s | `defaultRateLimit429FallbackCooldown` `:60`、`apply429FallbackRateLimit:1208-1216` |
| 其它平台无头 fallback = **5 分钟** | `ratelimit_service.go:1170-1176` |
| `rate_limit_exceeded` 与 `usage_limit_reached` 共用 body reset 解析 | `parseOpenAIRateLimitResetTime:1631-1633` |
| OAuth 401：invalidate + `cfg.RateLimit.OAuth401CooldownMinutes` | `ratelimit_service.go:478-501`、`config.go` `oauth_401_cooldown_minutes` |
| 本请求排除 | handler `failedAccountIDs`（如 `openai_gateway_handler.go:412`） |
| 同账号重试只服务池模式等 | `UpstreamFailoverError.RetryableOnSameAccount`；`failover_loop.go:79` |
| 选号已支持排除集 | `SelectAccountForModelWithExclusions` `gateway_service.go:1457` |
| pair / quality 不得进 `IsSchedulable()` | `.trellis/spec/backend/account-user-schedule.md` |
| 529 / 流超时 = Settings KV JSON + 独立 GET/PUT + 网关页自存卡片 | `SettingKeyOverloadCooldownSettings` / `SettingKeyStreamTimeoutSettings`；`SettingsView.vue` `activeTab==='gateway'` |
| 质量硬关闭 overlay 在 **extra**，专用 API，编辑大表单不得整表抹掉 | `account_quality_hard_close.go`；`admin_service.go:2977` |
| OAuth refresh **整表替换** credentials | `persistAccountCredentials` + `UpdateCredentials` `SetCredentials` |
| 账号三态布尔已有 `boolOverrideFromMap` | `codex_image_generation_bridge.go`；UI 如 `codexImageGenerationBridgeMode` inherit/enabled/disabled |
| public settings 不含 529 / 质量硬关闭 | `GetPublicSettings`；`TestQualityHardCloseSettings_NotExposedOnPublicSettings` |

Admin 把 `rate_limit_reset_at` 未到期显示成不可调度 / 「临时不可调度」观感，与 `temp_unschedulable_until` 叠在同一 `IsSchedulable()` 出口。

## 3. 软 / 硬分类

只对 **策略对该号生效 + HTTP 429** 走新分类。API Key / Bedrock 走原路径（含 `pool_mode`）。

### 3.1 内置硬（永远硬，码表改不掉）

命中任一条即硬，**短路**：

1. Anthropic 官方 5h / 7d 窗口耗尽（已有 `persistAnthropicExhaustedWindowLimit`；必须继续优先于用户规则与码表）。
2. Codex / OpenAI `used_percent >= 100`（5h 或 7d）且能解析对应 reset。
3. OAuth 401 / 403 渐进 / token 刷新失败 / `SetError`（不走本分类，但不得被本任务改写）。

Fable `7d_oi` 仍只写模型范围限流。OpenAI image 模型范围限流保持现有 `HandleOpenAIImageRateLimit` 早退。

### 3.2 可配置硬 / 软（Settings 码表）

在内置硬之后、写 DB 之前：

| 命中 | 结果 |
|---|---|
| body / `error.type` / `error.code`（大小写不敏感）∈ `hard_body_codes` | 硬 |
| 同上 ∈ `soft_body_codes` 且不在 hard 表 | 软（再看长 reset 政策） |
| HTTP 状态 ∈ `soft_status_codes`（v1 只允许 `429`）且无硬 body | 进入软候选 |
| 宽规则关键字「rate limit」单独出现 | 不够当硬 |

同一码同时出现在软/硬表：**硬赢**。

出厂硬 body（可改、可清空到只剩内置硬）：`INSUFFICIENT_QUOTA`、`USAGE_LIMIT_EXCEEDED`、`usage_limit_reached`、`API_KEY_QUOTA_EXHAUSTED`（及实现时已有的中文「额度已用完」同类 — 这些中文同类算内置硬别名，不要求 Admin 填写）。

出厂软 body：`rate_limit_exceeded`。

### 3.3 长 reset 无硬信号

`long_reset_policy`：

| 值 | 含义 |
|---|---|
| `soft`（出厂） | 没有 §3.1 / hard 码表时，**即使** header reset 很长（Codex 非 100% 用 7d reset）也当软。这是今天 Codex 长 TTL 的根因解法。 |
| `hard` | 能解析出 reset 就持久化（会把今天的 Codex 长 reset 坑留住）。 |
| `threshold` | 解析出的 reset 时长 **≥** `long_reset_threshold_seconds` 则硬，否则软。 |

无可靠 reset（无头、解不出、短 Retry-After）在 `soft` / `threshold` 下都是软；在 `hard` 下：无解析值则软（不能凭空写数天）。

### 3.4 顺序（必须改）

```text
今日:  Anthropic 官方窗口 → tryTempUnsched（401 除外）→ handle429
目标:  Anthropic 官方窗口 / 其它硬窗口
     → 解析 oauthFleetSoft429Applies
     → 若 applies：OAuth 软429 分类（soft：Redis + 返回，跳过 tryTemp 与 SetRateLimited）
     → 其余 tryTempUnsched
     → handle429（仅硬或策略对该号未生效）
```

否则账号上 429 + “rate limit” 规则会继续写 `temp_unschedulable_until`，AC7 失败。

## 4. 三层排除

### Layer 1 — 本请求

沿用 `failedAccountIDs`。软 429 与硬 429 都把当前号放进排除集后 failover。**不要**设 `RetryableOnSameAccount=true`。

### Layer 2 — 跨请求短记忆（新）

| 项 | 规划 |
|---|---|
| 键 | `oauth-soft-429:{accountID}`（账号级，**不是** group×account） |
| 值 | 任意占位（`1` / unix） |
| TTL | **`settings.ttl_seconds`**（Default 20；校验 5–300） |
| 写入 | 软 429 分类命中时 |
| 读取 | 调度过滤：无 sticky / 无 `previous_response` 硬亲和时，把仍存活的 id 并入排除集 |
| 失败 | Redis 写/读失败 **fail-open**（本请求仍换号；跨请求可能 stampede，不写 DB） |
| 不是 | `IsSchedulable()`、DB、Admin 可调度开关、quality / pair / smart-schedule cooldown |

账号级键：OAuth 身份是限流单位。group×account 会让另一分组的下一请求打回同一号。

硬亲和：layer-2 **不得**拆 sticky / `previous_response`。该请求仍钉原号（与今天一致）；失败后再进 layer-1。

### Layer 3 — 跨请求硬状态（不改语义）

仅：硬 429 `SetRateLimited`、OAuth 401 temp unsched + invalidate、OpenAI 403 渐进、token 刷新失败、`SetError`（revoked / 403×3 等）。

## 5. 配置面（v1 必做）

对齐 **529 过载冷却 / 流超时**：一个 Settings KV JSON、一对 admin GET/PUT、网关 Tab 一张自存卡片。不要新栈，不要塞进大表单 `PUT /admin/settings`。

### 5.1 Settings KV

| 项 | 值 |
|---|---|
| Key | `oauth_fleet_soft_429_settings`（`SettingKeyOAuthFleetSoft429Settings`） |
| 常量位置 | `backend/internal/service/domain_constants.go`（Overload / StreamTimeout 段落下） |
| 结构体 | `OAuthFleetSoft429Settings` 放 `settings_view.go` |
| Get/Set | `setting_service.go`：`GetOAuthFleetSoft429Settings` / `SetOAuthFleetSoft429Settings` |
| 缺省 / 坏 JSON | 返回 `DefaultOAuthFleetSoft429Settings()`（**回落机制**对齐 529：缺行/坏 JSON → Default\*）。**本 Default\* `Enabled=false`。529 的 `DefaultOverloadCooldownSettings().Enabled == true`——只抄回落，不要抄那个 true。** |
| Public | **不进** `PublicSettings` / injection payload；单测钉死 |

### 5.2 JSON 合约

```json
{
  "enabled": false,
  "ttl_seconds": 20,
  "long_reset_policy": "soft",
  "long_reset_threshold_seconds": 60,
  "scope": "all_oauth",
  "platforms": ["anthropic", "openai", "gemini", "antigravity", "grok"],
  "include_setup_token": true,
  "soft_status_codes": [429],
  "soft_body_codes": ["rate_limit_exceeded"],
  "hard_body_codes": [
    "INSUFFICIENT_QUOTA",
    "USAGE_LIMIT_EXCEEDED",
    "usage_limit_reached",
    "API_KEY_QUOTA_EXHAUSTED"
  ]
}
```

| 字段 | 类型 | 出厂 Default\* | 校验 |
|---|---|---|---|
| `enabled` | bool | **`false`（出厂关；空/缺 KV = 关）** | — |
| `ttl_seconds` | int | `20` | Set 且 enabled 时 **必须** 5–300；Get 时 clamp；disabled 时越界归一成 20 |
| `long_reset_policy` | string | `"soft"` | `soft` \| `hard` \| `threshold` |
| `long_reset_threshold_seconds` | int | `60` | policy=`threshold` 时 5–86400；其它政策 Get 仍回传，不参与分类 |
| `scope` | string | `"all_oauth"` | `all_oauth` \| `opt_in` |
| `platforms` | string[] | 五平台全量 | 必须是 `anthropic`/`openai`/`gemini`/`antigravity`/`grok` 子集；`opt_in` 时至少 1 个；去重小写 |
| `include_setup_token` | bool | `true` | — |
| `soft_status_codes` | int[] | `[429]` | v1 **只允许 429**（拒绝 529/5xx，避免扩到 out-of-scope） |
| `soft_body_codes` | string[] | `["rate_limit_exceeded"]` | trim、空串丢弃、最多 32、大小写不敏感存小写 |
| `hard_body_codes` | string[] | 上表四个 | 同上，最多 32 |

空数组：软状态缺省回 `[429]`；body 表空 = 「除内置硬外不再靠码表加减」（分类仍靠 §3.1 + 长 reset 政策 + 「无可靠 reset / 无硬 body 的 429」）。

**不要**做 `temp_unschedulable_rules` 那种 status+keyword+分钟 DSL。码表精神对齐 `custom_error_codes`（纯列表）。

### 5.3 HTTP

| 方法 | 路径 | 谁 |
|---|---|---|
| GET | `/api/v1/admin/settings/oauth-fleet-soft-429` | `SettingHandler.GetOAuthFleetSoft429Settings` |
| PUT | 同上 | `UpdateOAuthFleetSoft429Settings` |

挂在 `backend/internal/server/routes/admin.go` `registerSettingsRoutes`，529 / stream-timeout 旁边。

DTO：`backend/internal/handler/dto/settings.go` + `frontend/src/api/admin/settings.ts`（`getOAuthFleetSoft429Settings` / `updateOAuthFleetSoft429Settings`）。

校验失败：`400` + 字段错误文案（与 `cooldown_minutes must be between 1-120` 同风格）。

### 5.4 Admin UI

**全局**：`frontend/src/views/admin/SettingsView.vue`，**Gateway** tab，529 过载冷却卡片**之后**、质量硬关闭**之前**。独立卡片 `data-test="oauth-fleet-soft-429-card"`，**自己保存**，不进页级 `saveSettings`（对齐 529 / 质量硬关闭）。

卡片块：

1. 总开关 `enabled`（卡片打开时显示关；hint 写明出厂关 / 空 KV=关，须 Admin 打开）
2. 打开后：`ttl_seconds`；`long_reset_policy` 三选一；`threshold` 时显示阈值秒
3. 范围：`scope` 全 OAuth / 勾选平台；平台多选；`include_setup_token` 开关
4. 第二块「软/硬 body 码」：两个 tag 输入（整流器关键词那种），加一句「窗口 100% / OAuth 401 永远硬」

**账号**：`EditAccountModal.vue`，仅 `type==='oauth' || type==='setup-token'`。三态 **继承全局 / 强制开 / 强制关**（对齐 Codex image bridge inherit/enabled/disabled）。写 `extra.oauth_fleet_soft_429`：`true` / `false` / **删除键**。不要放在 Pool Mode 区（那是 API Key / Bedrock）。不要写 `credentials`。

`UpdateAccount` 必须 **merge extra**，不得整表替换掉该键（质量硬关闭已有「大表单不得抹 overlay」先例）。

### 5.5 账号覆盖存哪

**只存 `accounts.extra.oauth_fleet_soft_429`。**

不存 `credentials`：`persistAccountCredentials` 在 OAuth refresh 时整表 `SetCredentials`，写在凭据里会被刷掉。`pool_mode` / `custom_error_codes` / `temp_unschedulable_rules` 能放 credentials，是因为 API Key 不走这条 refresh。

读取复用 `boolOverrideFromMap(account.Extra, "oauth_fleet_soft_429")` → `*bool`（缺省 `nil`）。

### 5.6 解析序（启用谓词）

```text
oauthFleetSoft429Applies(account, settings):
  1. account == nil 或 !IsOAuth()                    → false
     （setup-token 算 IsOAuth；API Key / Bedrock 永远 false）
  2. extra.oauth_fleet_soft_429 == false             → false
  3. extra.oauth_fleet_soft_429 == true              → true
     （金丝雀：全局 enabled=false 或平台不在 opt_in 列表也对这一号开）
     仍禁止 IsPoolMode()；仍只对 IsOAuth()
  4. settings.Enabled == false                       → false
  5. type==setup-token 且 !include_setup_token       → false
  6. scope==opt_in 且 platform ∉ platforms           → false
  7. 否则                                            → true
```

TTL / 长 reset / 码表：**只读全局 Settings**（已 Default\* 填充）。v1 账号不覆盖这些。

热路径：`RateLimitService` 已有 `settingService`（529 就是这么读的）。Get 失败回落到 Default\*（此时全局 `enabled=false`，**不是**「当开」），不要因此写 DB。账号 `extra=true` 仍可金丝雀（§5.6 步 3 在步 4 之前）。

### 5.7 i18n（实现时 zh+en 成对）

`admin.settings.oauthFleetSoft429.*`：

`title`, `description`, `enabled`, `enabledHint`, `ttlSeconds`, `ttlSecondsHint`, `longResetPolicy`, `longResetSoft`, `longResetHard`, `longResetThreshold`, `longResetThresholdSeconds`, `longResetThresholdHint`, `scope`, `scopeAllOAuth`, `scopeOptIn`, `platforms`, `includeSetupToken`, `includeSetupTokenHint`, `softBodyCodes`, `hardBodyCodes`, `codesHint`, `invariantHint`, `saved`, `saveFailed`

`admin.accounts.oauthFleetSoft429.*`：

`label`, `hint`, `inherit`, `forceOn`, `forceOff`

## 6. 数据流

```text
Handler failover 循环
  → Select*(excluded = failedAccountIDs ∪ redisSoftExclude)  // 无硬亲和
  → 上游
  → HandleUpstreamError(429)
       applies? 分类
       硬 → SetRateLimited / 官方窗口 / Fable 模型范围
       软 → Redis SET TTL=settings.ttl_seconds；不写 DB
       不 applies → 现网 tryTemp + handle429
  → failedAccountIDs[id]=struct{}{}
  → 下一号（RetryableOnSameAccount 仍为 false）
```

`CheckErrorPolicy` / 预检若与 `HandleUpstreamError` 分叉，必须共用同一分类函数 + 同一 `applies` 谓词。

## 7. 合约

| 调用方 | 期望 |
|---|---|
| `HandleUpstreamError` 软 429 | `shouldDisable=false`；DB 两列不变；Redis TTL = 当时 settings |
| 调度 `Select*` | 无硬亲和：排除 Redis 命中号；硬亲和：忽略 layer-2 |
| `IsSchedulable()` | 软 429 后仍 true（其它条件不变） |
| Admin 列表 `schedulable` | 不因软 429 变假 |
| `IsPoolMode()` 账号 | 字节级保持现网 |
| Admin GET settings | 缺省 / 坏 JSON → Default\*（`enabled=false`）；不进 public |
| Admin PUT settings | 校验失败 400；成功后 GET 回显 |
| 账号 extra 三态 | 见 §5.6；refresh 后仍在 |
| 计费 | 不读不写 usage / `actual_cost` |
| 客户端 | 入站仍是同步 Chat Completions；不要求改 stream |

日志（建议）：`oauth_fleet_soft_429` 带 `account_id`、`platform`、`reason=soft|hard`、硬原因枚举、`applies`、`override`。不打 token。

## 8. 取舍

| 方案 | 结论 |
|---|---|
| 把 `IsPoolMode()` 扩到 OAuth | **否决**。401 早退 + 同账号 429 重试都错。 |
| 只做 layer-1、不做 Redis | **否决**。下一请求 stampede（PRD Locked 8）。 |
| 软 429 写 5s `SetRateLimited` | **否决**。仍污染 `IsSchedulable()` / Admin。 |
| 软排除进 `TempUnschedulableUntil` | **否决**。整号出池。 |
| group×account Redis | **否决**。限流单位是账号身份。 |
| v1 无 Settings、只写 Go 常量 | **否决**。Brandon：范围不能写死。 |
| 覆盖放 `credentials` | **否决**。OAuth refresh 整表替换凭据。 |
| 新 DSL / 新设置栈 | **否决**。抄 529 JSON 块 + `custom_error_codes` 列表。 |
| 账号级 TTL/码表 overlay | v1 不做。只要三态启用。 |
| 默认关 vs 默认开 | **已锁定默认关。** Default\* `enabled=false`；空 KV = 关。不要抄 529 空 KV=开。账号 extra `true` 仍金丝雀。 |

## 9. 兼容与回归

- API Key `pool_mode` / `pool_mode_hard_eviction`：不改分支条件。
- Anthropic 官方窗口优先、Fable 模型范围、OpenAI image 模型范围：保持。
- OAuth 401、403 渐进、token 刷新失败：保持；401 时长不搬进本 JSON。
- pair cap / quality gate / smart-schedule cooldown：admission / Redis 既有键，不复用 layer-2 键。
- sticky / `previous_response`：硬亲和优先于 layer-2。
- Spark 影子：软 429 写 Redis 应对 **凭据账号** id。影子不写本 extra 覆盖（无凭据）。
- 工作区已有 sticky / unpooled 脏文件：本任务只提交软 429 + 本配置面 hunk。

## 10. 上线 / 回滚

- 无 migration。无 Ent 列。多一条 Settings KV（首次保存才落库）。
- 上线：空 KV = Default\* = **`enabled=false`（策略关）**。Admin 在网关卡片打开后再对未覆盖 OAuth 生效。单号金丝雀：`extra.oauth_fleet_soft_429=true`。单号强制现网：`extra.oauth_fleet_soft_429=false`。
- 回滚：revert 分类 + Redis 过滤 + Settings 卡片。已有 Redis 键按当时 TTL 过期（≤300s）。DB 无新脏状态。KV 行可留着无害。关总开关或清空 KV（回 Default\* `enabled=false`）即可对未金丝雀账号回到现网 `handle429`。
- 风险：分类过宽 → 窗口耗尽号仍留在池里（AC3）；分类过窄 → 仍整号出池（AC1）；Redis 全挂 → fail-open（不写 DB）；全局关 + 无账号 true（含空 KV）→ 全员现网 `handle429`。把 529 的 `Enabled: true` 抄进本 Default\* 会在未保存 KV 时误开全车队——单测必须钉死空 KV `enabled=false`。

## 11. 文档

实现阶段加法更新：`docs/dev/codebase/account.md`、`docs/dev/codebase/gateway.md`、`docs/dev/CHANGELOG_CUSTOM.md`（`git add -f`）。可选：`.trellis/spec/backend/` 一条 OAuth 软 429 + Settings 合约。不改 `ARCHITECTURE.md`（沿用 Settings KV，无新顶层模块）。
