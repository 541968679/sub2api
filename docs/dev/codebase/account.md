# 账号管理 (Account)

> 管理 AI 平台账号（Antigravity/Anthropic/OpenAI/Gemini），包括 OAuth 导入、批量创建、状态监控、AI Credits 追踪。

## OpenAI Claude-GPT Bridge For Antigravity Groups

OpenAI accounts can opt into an account-side Claude-GPT bridge with
`extra.openai_claude_gpt_bridge_enabled=true`. This is a routing capability of
the OpenAI account; it does not migrate subscriptions, API keys, or the target
group platform.

Data model:

- The bridge switch is stored in `account.extra.openai_claude_gpt_bridge_enabled`.
- Claude-to-GPT mapping stays in the existing OpenAI
  `account.credentials.model_mapping`, for example
  `{ "claude-opus-4-8": "gpt-5.5" }`.
- OpenAI accounts may bind OpenAI groups by default. When the bridge switch is
  enabled, they may additionally bind Antigravity groups.
- OpenAI accounts still cannot bind Anthropic or Gemini groups through this
  bridge.

Important mechanisms:

- Bridge eligibility requires OpenAI platform, enabled extra flag, an explicit
  account-level model mapping hit, and a mapped model that is non-empty and
  different from the requested Claude model.
- Create/edit/bulk account validation uses the effective extra payload before
  validating group bindings, so the same request can enable the bridge and bind
  an Antigravity group.
- Turning the bridge off in the frontend removes Antigravity group selections so
  stale cross-platform bindings are not submitted.
- The mapping is account-global. There is no group-level or account-group-level
  Claude-GPT mapping.

## API Key Exclusive Group Runtime Guard

API keys are validated against exclusive-group authorization both when they are
created and when they are used.

Data model:

- `users.allowed_groups` is the source of truth for standard exclusive groups.
- Subscription groups still use active subscription checks instead of
  `allowed_groups`.
- The lightweight API-key auth path stores `allowed_groups` and group
  `is_exclusive` in `APIKeyAuthSnapshot`, so cache hits enforce the same rule as
  DB reads.

Important mechanisms:

- `backend/internal/server/middleware/api_key_auth.go` rejects an API key with
  `GROUP_NOT_ALLOWED` when its bound group is exclusive and the owner no longer
  has that group in `allowed_groups`.
- `backend/internal/repository/api_key_repo.go:GetByKeyForAuth` must select
  user allowed groups and group exclusivity fields; removing either field
  weakens runtime enforcement.
- `backend/internal/service/admin_service.go:UpdateUser` invalidates API-key
  auth cache when `allowed_groups` changes, so permission removals do not wait
  for cache TTL expiry.

## OpenAI Images Endpoint Scheduling

OpenAI OAuth/API-key accounts can opt out of independent Images endpoint
scheduling with `extra.openai_images_endpoint_enabled=false`.

This switch only affects `/v1/images/generations` and `/v1/images/edits`.
Missing, null, or non-boolean values default to enabled for backward
compatibility. It is intentionally separate from
`extra.codex_image_generation_bridge`, which only controls Codex
`/v1/responses` image tool injection.

Implementation notes:

- Create/Edit account forms save `false` only when disabled; re-enabling removes
  the extra key.
- The scheduler reads the same `Account.SupportsOpenAIImageCapability()` helper
  in both scheduler and load-awareness fallback paths.
- `openai_images_endpoint_enabled` is scheduler-relevant, so updating it must
  enqueue scheduler outbox work and refresh account snapshots.

## Upstream Model Sync

Admins can fetch a live model list from an account's upstream model-list API and
append missing entries to the local whitelist or Antigravity mapping editor.

Data model:

- No new persisted schema is added. Saved-account sync reads the existing
  account credentials from DB.
- Create-flow preview builds a temporary in-memory account from
  `platform`, `type`, `base_url`, and `api_key`; it does not create or update an
  account.
- The returned model IDs are used only by the frontend to append missing local
  entries.

Key files:

- `backend/internal/service/upstream_models.go`: builds provider-specific
  model-list requests and parses OpenAI-style `data`, Gemini-style `models`,
  and array responses.
- `backend/internal/handler/admin/account_handler.go`: exposes
  `POST /api/v1/admin/accounts/:id/models/sync-upstream` and
  `POST /api/v1/admin/accounts/models/sync-upstream-preview`.
- `frontend/src/components/account/ModelWhitelistSelector.vue`: sync button for
  saved accounts and create-flow preview credentials.
- `frontend/src/components/account/EditAccountModal.vue`: Antigravity saved
  account mapping sync.
- `frontend/src/components/account/CreateAccountModal.vue`: temporary preview
  credentials for API-key account creation, including Antigravity compatible
  upstream mappings.

Important mechanisms:

- Sync is append-only. Existing whitelist entries and Antigravity mappings are
  never deleted or replaced by the sync result.
- Saved-account sync can use stored credentials, proxy assignment, and provider
  token providers.
- Preview sync only uses form credentials and never persists secrets.
- Antigravity OAuth uses the Cloud Code `FetchAvailableModels` path.
  Antigravity API-key sync intentionally requires a compatible gateway base URL
  ending in `/antigravity`.
- This feature does not alter billing, display pricing, model mapping
  resolution, Claude-GPT bridge behavior, OpenAI image endpoint scheduling, or
  Codex image bridge settings.

## 数据模型

| 实体/字段 | 位置 | 说明 |
|-----------|------|------|
| Account entity | `backend/ent/schema/account.go` | 主表，包含 name, platform, type, status 等 |
| credentials (JSONB) | 同上 | OAuth token 数据：access_token, refresh_token, email, project_id, plan_type, expires_at |
| extra (JSONB) | 同上 | 平台特有配置：allow_overages, mixed_scheduling, privacy_mode, model_rate_limits |
| Account DTO | `backend/internal/handler/dto/types.go:133` | API 响应结构，包含 credentials 和 extra 完整输出 |
| AccountUsageInfo | `frontend/src/types/index.ts:793` | 账号用量信息，含 ai_credits 数组 |
| WindowStats | `frontend/src/types/index.ts:770` | 今日统计（requests, tokens, cost），不含 ai_credits |

### 邮箱存储位置（重要）

| 平台 | 邮箱字段位置 | 来源 |
|------|-------------|------|
| Antigravity | `credentials.email` | Google OAuth UserInfo API |
| Anthropic | `extra.email_address` | Anthropic OAuth 响应 |
| Gemini (google_one) | `credentials.email` | Google OAuth UserInfo API（仅 RT 批量导入路径会写入；OAuth 授权码路径目前不写入） |

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| **Handler** | `backend/internal/handler/admin/account_handler.go` | REST API：List, Create, BatchCreate, GetStats, GetUsage |
| **Handler** | `backend/internal/handler/admin/antigravity_oauth_handler.go` | Antigravity OAuth：GenerateAuthURL, ExchangeCode, RefreshToken |
| **Handler** | `backend/internal/handler/admin/gemini_oauth_handler.go` | Gemini OAuth：GenerateAuthURL, ExchangeCode, GetCapabilities, RefreshToken（仅 google_one） |
| **Service** | `backend/internal/service/admin_service.go` | 业务逻辑：CreateAccount, ListAccounts |
| **Service** | `backend/internal/service/antigravity_oauth_service.go` | OAuth 流程：ValidateRefreshToken, RefreshToken, BuildAccountCredentials |
| **Service** | `backend/internal/service/gemini_oauth_service.go` | Gemini OAuth 流程：ExchangeCode, RefreshToken, ValidateGoogleOneRefreshToken, BuildAccountCredentials, FetchGoogleOneTier |
| **Service** | `backend/internal/service/antigravity_quota_fetcher.go` | AI Credits + 配额获取：FetchQuota → LoadCodeAssist |
| **Service** | `backend/internal/service/antigravity_credits_overages.go` | Credits 耗尽检测、超量请求重试逻辑 |
| **Service** | `backend/internal/service/account_usage_service.go` | 用量统计：GetAccountUsageInfo, GetTodayStats |
| **Repository** | `backend/internal/repository/account_repo.go` | 数据查询：ListWithFilters (搜索 name + email) |
| **API Client** | `backend/internal/pkg/antigravity/client.go` | HTTP 调用：RefreshToken, GetUserInfo, LoadCodeAssist, FetchAvailableModels |
| **Frontend View** | `frontend/src/views/admin/AccountsView.vue` | 账号列表页：表格、搜索、AI Credits 汇总 |
| **Frontend Component** | `frontend/src/components/account/CreateAccountModal.vue` | 创建弹窗：单个 + 批量导入 |
| **Frontend Component** | `frontend/src/components/account/EditAccountModal.vue` | 编辑弹窗 |
| **Frontend Component** | `frontend/src/components/account/BulkEditAccountModal.vue` | 批量编辑弹窗 |
| **Frontend Component** | `frontend/src/components/common/GroupSelector.vue` | 账号/公告等场景复用的分组多选器；账号表单通过 `show-toggle-all` 开启全选/取消全选 |
| **Frontend Component** | `frontend/src/components/account/AccountUsageCell.vue` | 用量单元格：展示 5h/7d 窗口 + AI Credits |
| **Frontend Composable** | `frontend/src/composables/useAntigravityOAuth.ts` | Antigravity OAuth 前端逻辑：validateRefreshToken, buildCredentials |
| **Frontend API** | `frontend/src/api/admin/accounts.ts` | 账号相关 API 调用封装 |
| **Frontend API** | `frontend/src/api/admin/antigravity.ts` | Antigravity OAuth API：refreshAntigravityToken |

## 核心流程

### Antigravity 批量导入（Refresh Token）

```
用户输入多行 refresh_token
  → CreateAccountModal.vue: handleAntigravityValidateRT()
    → 逐个循环:
      → useAntigravityOAuth.ts: validateRefreshToken(rt, proxyId)
        → POST /api/v1/admin/antigravity/oauth/refresh-token
          → antigravity_oauth_handler.go: RefreshToken()
            → antigravity_oauth_service.go: ValidateRefreshToken()
              → s.RefreshToken() → client.RefreshToken() [Google OAuth]
              → 回填原始 refresh_token（Google 不返回新 RT）
              → client.GetUserInfo() → 获取 email
              → loadProjectIDWithRetry() → client.LoadCodeAssist() → 获取 project_id + plan_type
        ← AntigravityTokenInfo { access_token, refresh_token, email, project_id, plan_type }
      → buildCredentials(tokenInfo) → { access_token, refresh_token, email, ... }
      → 命名: useEmailAsName ? email : form.name + #index
      → buildAntigravityExtra() → { allow_overages?, mixed_scheduling? }
      → POST /api/v1/admin/accounts → account_handler.go: Create()
        → admin_service.go: CreateAccount()
```

### Gemini Google One 批量导入（Refresh Token）

```
用户输入多行 refresh_token（RT 必须由内置 Gemini CLI OAuth client 签发）
  → CreateAccountModal.vue: handleGeminiGoogleOneValidateRT()
    → 逐个循环:
      → useGeminiOAuth.ts: validateGoogleOneRefreshToken(rt, proxyId)
        → POST /api/v1/admin/gemini/oauth/refresh-token
          → gemini_oauth_handler.go: RefreshToken()
            → gemini_oauth_service.go: ValidateGoogleOneRefreshToken()
              → s.RefreshToken(ctx, "google_one", rt, ...) → oauthClient.RefreshToken()
              → 回填原 refresh_token（Google 不返回新 RT）
              → oauthClient.GetUserInfo() → email（失败仅 warn 不阻断）
              → fetchProjectID() → project_id（必需；失败则该 RT 标记失败）
              → FetchGoogleOneTier() → tier_id + drive_storage_* extra（失败回落 google_one_free）
        ← GeminiTokenInfo { access_token, refresh_token, email, project_id, tier_id, extra }
      → buildCredentials(tokenInfo) → { access_token, refresh_token, email, tier_id, oauth_type: "google_one", ... }
      → 命名: useEmailAsName ? email : (多个→form.name #i, 单个→form.name)
      → POST /api/v1/admin/accounts → account_handler.go: Create()
```

限制：RT 必须由内置 Gemini CLI OAuth client（ID `681255809395-...`）签发；自建 client 的 RT 会返回 `unauthorized_client` 错误，提示中已包含对应说明。code_assist 与 ai_studio 暂不支持批量 RT 导入（ai_studio 依赖运营方自配 OAuth client，code_assist 的 project_id 失败率更高）。

### 账号列表 + 搜索

```
AccountsView.vue: load() / reload()
  → GET /api/v1/admin/accounts?search=xxx&platform=&page=&...
    → account_handler.go: List()
      → admin_service.go: ListAccounts()
        → account_repo.go: ListWithFilters()
          搜索 OR 条件: name ILIKE | credentials->email LIKE | extra->email_address LIKE
  → refreshTodayStatsBatch() → POST /admin/accounts/today-stats/batch
  → refreshAICreditsTotal() → 逐个 GET /admin/accounts/:id/usage（按 email 去重）
```

### AI Credits 获取链路

```
AccountsView.vue: refreshAICreditsTotal()
  → 过滤 antigravity 账号 → 按 credentials.email 去重
  → Promise.allSettled: GET /api/v1/admin/accounts/:id/usage
    → account_handler.go: GetUsage()
      → account_usage_service.go: GetAccountUsageInfo()
        → antigravity_quota_fetcher.go: FetchQuota()
          → client.LoadCodeAssist() → PaidTierInfo.AvailableCredits
  → 汇总 ai_credits[].amount
```

## 重要机制

| 机制 | 说明 | 相关文件 |
|------|------|---------|
| refresh_token 回填 | Google OAuth 刷新不返回新 RT，ValidateRefreshToken 需回填原始值 | `antigravity_oauth_service.go:228` |
| AI Credits 动态获取 | 不存储在 DB，每次通过 LoadCodeAssist API 实时查询；OAuth 账号必须经 `AntigravityTokenProvider` 取 token，不能直接读 `credentials.access_token` | `antigravity_quota_fetcher.go`, `antigravity_token_provider.go` |
| AI Credits 历史快照 | 为运营分析"credits 消耗 / 每 credit 额度"，`CreditSnapshotService` 每 15 分钟按 email 去重采样到 `ai_credit_snapshots`；`GetAntigravityUsageRatio` 走正向 delta 聚合 | `service/credit_snapshot_service.go`、`migrations/110_add_ai_credit_snapshots.sql` |
| Credits 去重 | 同 Google 账号（同 email）共享 AI Credits 余额，汇总时按 email 去重 | `AccountsView.vue:refreshAICreditsTotal`，`credit_snapshot_service.go:captureOnce` |
| Credits 耗尽检测 | 关键词匹配（"insufficient credit" 等）→ 标记 model_rate_limits["AICredits"] 5h 冷却 | `antigravity_credits_overages.go:36-49` |
| OpenAI Claude-GPT bridge | Bridge 请求挂在 Antigravity 分组下，但真实上游账号是 OpenAI，不消耗 Antigravity AI Credits；`AntigravityUsageAggregator` 继续按 `accounts.platform=antigravity` 聚合，避免污染 credits-per-call / quota-per-credit 比率 | `antigravity_usage_aggregator.go`, `openai_gateway_service.go` |
| 超量请求重试 | 免费配额耗尽后，如 allow_overages=true，注入 enabledCreditTypes: ["GOOGLE_ONE_AI"] 重试 | `antigravity_credits_overages.go:172` |
| 隐私模式设置 | 创建/刷新账号后自动调用 setUserSettings 设置隐私 | `antigravity_oauth_service.go:256` |
| 批量 vs 单创建 | 批量走 handleAntigravityValidateRT()，单创建走 handleAntigravityExchange()，extra 构建需两处一致 | `CreateAccountModal.vue` |
| 账号分组全选 | 创建、编辑、批量编辑共用 `GroupSelector` 的 `show-toggle-all` 入口；全选/取消全选只作用于当前可选分组，保留平台过滤外的既有 `group_ids` | `GroupSelector.vue`, `CreateAccountModal.vue`, `EditAccountModal.vue`, `BulkEditAccountModal.vue` |
| Gemini RT client 绑定 | Google OAuth 的 refresh_token 绑定签发它的 client_id；google_one 批量导入强制用内置 Gemini CLI client，自建 client 的 RT 报 unauthorized_client | `gemini_oauth_service.go:ValidateGoogleOneRefreshToken` |

## 已知陷阱

- **邮箱双来源**：Antigravity 存 `credentials.email`，Anthropic 存 `extra.email_address`，Gemini google_one RT 批量导入也会写 `credentials.email`。搜索和展示都需兼容两处。
- **批量/单创建分支**：批量导入和单个 OAuth 导入是两个独立代码路径，修改 extra/credentials 构建逻辑时必须两处都改。
- **AI Credits 不在 WindowStats 中**：`getBatchTodayStats` 返回的是 `WindowStats`（requests/tokens/cost），不含 ai_credits。Credits 需单独调 `getUsage` API。
- **Credits 消耗冷启动窗**：`ai_credit_snapshots` 需要至少两条相邻采样才能算 delta。新部署或新窗口内无采样时 `GetAntigravityUsageRatio` 返回 `credits_consumed=0` + 比率 null；前端卡片显示"采样不足"。如果窗口内出现负 delta（充值/重置），只跳过该对不报错，但那一段消耗会丢。
- **临时不可调度**：token 刷新失败时标记 `temp_unschedulable_until`，到期后自动重试。如果 refresh_token 为空则永远失败。
- **setup-token 401 处理**：`setup-token` 在网关里按 OAuth/Bearer 凭证使用，401 首次命中应走临时不可调度和 token 缓存失效，不应直接标记 `status=error`。
- **Antigravity usage 401 误判**：账号用量/AI Credits 探测必须和模型测试、真实网关请求一样走 `AntigravityTokenProvider`。如果直接读取 DB 中过期的 `credentials.access_token`，会在 refresh token 正常时偶发 401，并让前端误显示“需要重新授权”。
- **Antigravity OAuth 401 状态处理**：OAuth 账号的 401 应优先临时不可调度并触发 token 缓存失效/刷新，不能直接永久 `SetError`。特别是 `/v1/chat/completions` 这类 Anthropic 兼容路径若误选 Antigravity 账号，会因上游路径不匹配返回 `Invalid bearer token`，但账号在 Antigravity 原生路径仍然可用。
