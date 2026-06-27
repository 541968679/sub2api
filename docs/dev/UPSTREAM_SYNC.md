# 上游同步记录

> 记录每次从上游 (Wei-Shaw/sub2api) 合并更新的情况，便于追踪同步状态和解决冲突。

## 当前状态

| 项目 | 值 |
|------|-----|
| 上游仓库 | https://github.com/Wei-Shaw/sub2api |
| 上游 remote 名 | `upstream` |
| 最后同步 commit | `48912014` (chore: sync VERSION to 0.1.121) |
| 最后同步日期 | 2026-05-03 |
| 上游版本标签 | v0.1.121 |

> Note: the table above tracks the last completed full upstream sync. The
> 2026-06-27 entries below are staged safety-fix batches against upstream, not a
> declaration that the fork has fully caught up with upstream.

## 同步操作步骤

```bash
# 1. 拉取上游
git fetch upstream

# 2. 查看差异
git log main..upstream/main --oneline

# 3. 合并（在 main 分支上）
git checkout main
git merge upstream/main

# 4. 解决冲突（如有），优先保留二开修改
# 5. 测试
make test

# 6. 推送
git push origin main
```

## 同步记录

### 2026-06-27 - upstream OpenAI images and overloaded error verification batch 10

- **Branch**: `codex/upstream-sync-20260627`
- **Preflight**: presented the required detailed assessment table before handling this batch, covering each OpenAI/Images candidate, tests, frontend visibility, and fork-local secondary-development impact.
- **Evaluated upstream commits**:
  - `9491de0a` - pass OpenAI Images content-moderation refusals through as 400
  - `b0d5592a` - recognize OpenAI Images `response.incomplete` and record soft-failure upstream response diagnostics
  - `cc7612bd` - detect OpenAI overloaded error codes
- **Result**:
  - No runtime code commit was needed. `9491de0a` conflicted because the current branch already has the equivalent local implementation and tests; the attempted cherry-pick was aborted to avoid duplicate/empty changes.
  - `b0d5592a` behavior is already present in `openai_images_responses.go` and `openai_images_incomplete_test.go`.
  - `cc7612bd` behavior is already present via local commit `92ec4294`.
- **Fork-local secondary-development impact**:
  - No new frontend-visible behavior, API route, database migration, billing/display-token, curated model list, Claude-GPT bridge, subscription, account scheduling, or payment behavior change in this batch.
  - Existing fork-local OpenAI Images trace/ops recording and same-account retry behavior were verified and left unchanged.
- **Verification**:
  - `go test -tags=unit ./internal/service -run "Test(ExtractImagesUpstreamError|ImagesOAuthNonStreaming|ExtractModelRefusal|IsOpenAITransientProcessingError|OpenAIStreamingResponseFailedBeforeOutput(ServerOverloadedCode|CapacityError|ReturnsFailover)|OpenAIGatewayService_Forward_TransientProcessingErrorTriggersFailover)" -count=1`
  - `git diff --check`

### 2026-06-27 - upstream auth promo and frontend title batch 9

- **Branch**: `codex/upstream-sync-20260627`
- **Preflight**: presented the required detailed assessment table before applying this batch, covering feature behavior, affected modules, frontend visibility, tests, and fork-local secondary-development impact.
- **Synced upstream commits**:
  - `ecedc7c8` - enforce email bind suffix whitelist
  - `2dc1387b` - allow clearing promo-code expiry on edit
  - `952be871` - refresh custom page document title
- **Supplemental local reconciliation**:
  - Added wildcard registration email suffix support (`*.domain` and `@*.domain`) because the upstream email-bind tests use `*.edu.cn` and this fork's existing normalization previously dropped that entry as invalid.
- **Local reconciliation**:
  - `ecedc7c8` and `2dc1387b` cherry-picked cleanly.
  - `952be871` conflicted in `App.vue` and `router/index.ts`; resolved by keeping this fork's existing auth/backend-mode/simple-mode route guards and app shell, adding only the title refresh helper/watch path, and avoiding unrelated upstream compliance-dialog context.
- **Fork-local secondary-development impact**:
  - Auth policy is intentionally stricter when `registration_email_suffix_whitelist` is configured; email identity binding now follows the same suffix rules as registration/email-code flows.
  - Promo-code admin editing can now clear expiry without changing redeem-code batch limits or subscription entitlement logic.
  - Frontend-visible impact is limited to browser tab title refresh for custom pages and locale/site-setting changes.
  - No billing/display-token accounting, curated model list, Claude-GPT bridge, OpenAI Images, account scheduling, database migration, API route, subscription fulfillment, or payment amount changes.
- **Verification**:
  - `go test -tags=unit ./internal/service -run "Test(NormalizeRegistrationEmailSuffixWhitelist|ParseRegistrationEmailSuffixWhitelist|IsRegistrationEmailSuffixAllowed|AuthServiceBindEmailIdentity_RegistrationSuffixWhitelistWildcard|AuthServiceEmailIdentityBinding_RejectsEmailOutsideRegistrationSuffixWhitelist|AuthServiceBindEmailIdentity_AllowsEmailInsideRegistrationSuffixWhitelist)" -count=1`
  - `go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin -run "Test.*(Email|Bind|OAuth|Suffix|Promo|PromoCode|Pending)" -count=1`
  - `pnpm --dir frontend run test:run src/router/__tests__/title.spec.ts`
  - `pnpm --dir frontend run typecheck`
  - `pnpm --dir frontend run lint:check`
  - `git diff --check`

### 2026-06-27 - upstream gateway client detection and Vertex beta batch 8

- **Branch**: `codex/upstream-sync-20260627`
- **Preflight**: presented the required detailed assessment table before applying this batch, then added a supplemental table before including the `ddf91e9a` helper prerequisite discovered during testing.
- **Synced upstream commits**:
  - `e3e31bd4` - recognize Claude Code auto mode via any `cc_entrypoint=` marker
  - `40e1cc14` - filter `anthropic-beta` on the Vertex Anthropic path
  - `efffd5d7` - add Vertex anthropic-beta filtering tests
- **Supplemental local reconciliation**:
  - Added the minimal helper surface from upstream `ddf91e9a`: `sanitizeAnthropicBodyForBetaTokens`, `anthropicBetaTokensContains`, and `deleteHeaderAllForms`.
  - Did not import the broader `ddf91e9a` count_tokens/API-key passthrough behavior in this batch.
  - Did not import `6cfb7898` cch-signing deletion in this batch.
- **Local reconciliation**:
  - `e3e31bd4` conflicted in the Claude Code validator and tests; manually ported the marker change and focused tests instead of importing the larger upstream test block.
  - `40e1cc14` conflicted in `gateway_service.go`; resolved by keeping upstream final beta filtering and preserving the fork-local `setOpsUpstreamRequestBody(c, vertexBody)` call after final body sanitization.
  - `efffd5d7` applied cleanly, then tests were adapted to this fork's current 2-return-value `buildUpstreamRequest` signature.
- **Fork-local secondary-development impact**:
  - No frontend-visible UI change.
  - No database migration, route, i18n, display-token/display-pricing, curated model list, Claude-GPT bridge, OpenAI Images, subscriptions, account scheduling, or billing behavior changes.
  - Intentional backend behavior changes are limited to broader Claude Code system-prompt recognition and safer Anthropic Vertex beta header/body forwarding.
  - Fork-local ops request-body capture remains active and now records the final sanitized Vertex body.
- **Verification**:
  - `go test -tags=unit ./internal/service -run "TestClaudeCodeValidator|Test.*Vertex.*Beta|Test.*Anthropic.*Vertex|Test.*Beta.*Filter" -count=1`
  - `git diff --check`

### 2026-06-27 - upstream small auth/ops/keys/payment guard batch 7

- **Branch**: `codex/upstream-sync-20260627`
- **Preflight**: presented the required detailed assessment table before applying this batch, with feature behavior, affected modules, frontend visibility, tests, fork-local secondary-development links, expected impact, risk, and handling strategy.
- **Synced upstream commits**:
  - `82576e0a` - stop swallowing email auth identity create errors caused by a shadowed `err`
  - `9707dedc` - prevent ops monitoring trend cards from growing unbounded
  - `ae5e980d` - enforce `codex_cli_only` on `/v1/chat/completions`
  - `28e7adef` - add `CLAUDE_CODE_ATTRIBUTION_HEADER=0` to Claude Code terminal templates
  - `65ad7df4` - keep payment provider cards visible when `supported_types` is empty/null
- **Local reconciliation**:
  - `ae5e980d` conflicted in `openai_gateway_chat_completions.go`; resolved by inserting the `detectCodexClientRestriction` guard before this fork's existing APIKey raw Chat fallback split, preserving local `openai_compat.ShouldUseResponsesAPI` routing behavior.
  - The other commits cherry-picked cleanly.
- **Fork-local secondary-development impact**:
  - No changes to display-token/display-pricing accounting, curated model lists, Claude-GPT bridge dispatch, OpenAI Images, subscriptions/bundle fulfillment, database migrations, routes, or i18n.
  - Intentional frontend-visible changes are limited to ops dashboard sizing, key usage command templates, and admin payment provider card visibility.
  - Intentional API behavior change: `codex_cli_only` OpenAI accounts now reject non-matching clients on `/v1/chat/completions` before raw Chat fallback forwarding.
- **Verification**:
  - `go test -tags=unit ./internal/service -run "Test.*Auth|Test.*Email|Test.*OAuth|Test.*Register" -count=1`
  - `go test -tags=unit ./internal/service -run "Test.*(Codex|ChatCompletions|CLIOnly|ClientRestriction|RawChat|ResponsesChat)" -count=1`
  - `pnpm --dir frontend run test:run src/views/admin/__tests__/SettingsView.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts`
  - `pnpm --dir frontend run typecheck`
  - `pnpm --dir frontend run lint:check`
  - `git diff --check`

### 2026-06-27 - upstream runtime compatibility batch 6

- **Branch**: `codex/upstream-sync-20260627`
- **Preflight**: presented the required detailed assessment table before applying this batch, including frontend visibility, tests, fork-local secondary-development links, expected impact, risk, and handling strategy.
- **Synced upstream commits**:
  - `ad135854` - Docker build context includes `docs/legal`
  - `f6e0ebc6` - preserve Anthropic official 5h/7d window cooldowns before temporary-unschedulable fallbacks
  - `c1c28ac7` - decompress zstd upstream responses
  - `6c7203d8` - preserve SSE `event:error` body for ops logs
  - `6c2db4f4` - clean unsupported Gemini tool schema fields
  - `bab8a9a9` - log `/v1/chat/completions` upstream endpoint for chat-only OpenAI API-key accounts
- **Local reconciliation**:
  - `f6e0ebc6` conflicted in `ratelimit_service.go`; kept the long-window cooldown persistence while preserving this fork's `HandleUpstreamError(ctx, account, status, headers, body)` signature and existing scheduling/failover semantics.
  - `bab8a9a9` was manually ported after aborting a conflict-heavy cherry-pick, because the upstream file context also contained risk-control/content-moderation helpers that are not part of this fork's current synced surface. The port only changed OpenAI usage-record upstream endpoint derivation and kept fork-local `submitUsageRecordTask`, request context wrapping, and WebSocket `turnAccount` accounting.
  - Added a focused handler test for the APIKey forced-Chat-Completions endpoint resolver.
- **Fork-local secondary-development impact**:
  - No frontend-visible UI changes.
  - No change to display-token/display-pricing accounting, curated model lists, Claude-GPT bridge dispatch, OpenAI image generation, default-model fallback, i18n, routes, or database migrations.
  - Intentional runtime impacts: Docker image packaging includes legal docs; Anthropic account cooldowns prefer upstream official 5h/7d windows; zstd responses are parseable; SSE ops logs keep raw upstream error bodies; Gemini tool schemas are cleaned before forwarding; OpenAI usage/ops metadata records the actual raw Chat Completions upstream endpoint for chat-only API-key accounts.
- **Verification**:
  - `go test -tags=unit ./internal/service -run "TestHandleUpstreamError_AnthropicWindowLimitPreemptsTempUnschedRule|Test.*Anthropic.*Window|Test.*Cooldown" -count=1`
  - `go test -tags=unit ./internal/repository -run "Test.*Decompress|Test.*Zstd|Test.*ContentEncoding" -count=1`
  - `go test -tags=unit ./internal/service -run "TestHandleStreamingResponse_(SSEErrorEvent|StreamReadError|FailoverBody|EmptyStream|SpecialCharacters)" -count=1`
  - `go test -tags=unit ./internal/service -run "Test(ConvertClaudeToolsToGeminiTools|CleanToolSchema|GeminiMessagesCompatServiceForward)" -count=1`
  - `go test -tags=unit ./internal/handler -run "Test(OpenAIUpstreamEndpoint|ResolveOpenAIUpstreamEndpoint)" -count=1`
  - `git diff --check`

### 2026-06-27 - upstream safety fix batch 5 (tooling/auth/compat/gateway)

- **Branch**: `codex/upstream-sync-20260627`
- **Preflight**: presented the required assessment table before applying this batch.
- **Synced upstream commits**:
  - `ac6e36f9` - admin CLI supports `SUB2API_JWT` auth fallback
  - `727ac3f6` - add `app_session_terminated` to non-retryable refresh errors
  - `edfd5e37` - default apicompat tool `strict` to false
  - `ab9987b2` - fail over on non-JSON 2xx upstream responses
  - `b256f911` - intercept streaming `max_tokens=1` Haiku probes too
- **Local reconciliation**:
  - Kept the fork-local admin skill invocation path (`~/.codex/skills/...`) while adding JWT fallback documentation.
  - Production refresh code already included the merged non-retryable markers from earlier work, so this batch primarily added explicit test coverage.
  - Adapted non-JSON 2xx failover to this fork's current `RateLimitService.HandleUpstreamError(ctx, account, status, headers, body)` signature.
- **Verification**:
  - `node --check skills/sub2api-admin/scripts/sub2api-admin.js`
  - `go test -tags=unit ./internal/service -run "TestIsNonRetryableRefreshError|TestNonRetryableRefreshError" -count=1`
  - `go test -tags=unit ./internal/pkg/apicompat`
  - `go test -tags=unit ./internal/service -run "Test.*Non.*JSON|Test.*NonStreaming.*Response|Test.*Failover.*Non" -count=1`
  - `go test -tags=unit ./internal/handler -run "Test.*Intercept|Test.*Haiku|Test.*Warmup|Test.*Suggestion" -count=1`
  - `git diff --check`
- **Deferred larger batches**:
  - Grok subscription support stack.
  - OpenAI Codex personal access token auth.
  - `codex_cli_only` engine-fingerprint/app-server settings and frontend stack.
  - Broader quota/payment/frontend/migration buckets.

### 2026-06-27 - upstream safety fix batch 4 (Codex Spark image tool strip)

- **Branch**: `codex/upstream-sync-20260627`
- **Synced upstream commit**:
  - `01127820` - strip `image_generation` tool for Codex Spark gateway requests
- **Local reconciliation**:
  - Adapted the HTTP `/responses` path to the fork-local `reqBody` mutation and `disablePatch` mechanism instead of upstream request-view helpers.
  - Kept the local Responses WebSocket fast-policy/ops flow and inserted only the Spark strip step after upstream model normalization.
  - Avoided bringing unrelated upstream hotpath baseline tests into the fork; only the Spark APIKey regression test was added.
- **Verification**:
  - `go test -tags=unit ./internal/service -run "Test(ApplyCodexOAuthTransform_StripsImageGenerationToolForSpark|ApplyCodexOAuthTransform_StripsImageGenerationToolForSparkAlias|ApplyCodexOAuthTransform_KeepsImageGenerationToolForNonSpark|OpenAIGatewayService_Forward_StripsImageGenerationToolForSparkAPIKey|StripCodexSparkImageGenerationToolFromRawPayload)" -count=1`
  - `git diff --check`
- **Remaining high-priority staged candidates**:
  - Grok subscription stack, OpenAI PAT auth, admin CLI JWT fallback, and broader quota/payment/frontend/migration buckets remain unsynced.

### 2026-06-27 - upstream safety fix batch 3 (passthrough function-call args)

- **Branch**: `codex/upstream-sync-20260627`
- **Synced upstream commit**:
  - `2b49d662` - dedupe passthrough function-call arguments
- **Local reconciliation**:
  - Applied cleanly after the fork-local OpenAI passthrough sanitization changes.
  - Kept the normalization after local display-token rewrite and before event classification in streaming passthrough.
- **Verification**:
  - `go test -tags=unit ./internal/service -run "Test(HandleStreamingResponsePassthroughDeduplicatesFunctionCallArguments|ForwardResponsesChatCompletionsFallbackKeepsFunctionArgumentsSingle|Dedupe|PassthroughFunction)" -count=1`
  - `git diff --check`
- **Remaining high-priority staged candidates**:
  - Grok subscription stack, OpenAI PAT auth, admin CLI JWT fallback, and broader quota/payment/frontend/migration buckets remain unsynced.

### 2026-06-27 - upstream safety fix batch 2 (model availability 404)

- **Branch**: `codex/upstream-sync-20260627`
- **Baseline before batch**: `5f9b750c` (batch 1 documented)
- **Synced upstream commit**:
  - `fcd3bc12` - return 404 `model_not_found` instead of 503 when no configured account supports the requested model
- **Local reconciliation**:
  - Preserved fork-local OpenAI Chat Completions default mapped-model fallback before classifying the no-account result.
  - Preserved Claude-GPT bridge fallback behavior and `/responses/compact` unsupported handling before applying the generic no-account classifier.
  - Added the small ops routing-capacity marker helper required by the upstream handler changes.
- **Verification**:
  - `go test -tags=unit ./internal/service -run "Test.*ModelAvailability" -count=1`
  - `go test -tags=unit ./internal/handler -run "Test.*NoAccount" -count=1`
  - `git diff --check`
- **Remaining high-priority staged candidates**:
  - `01127820` - strip `image_generation` tool for Codex Spark gateway requests.
  - Grok subscription stack, OpenAI PAT auth, admin CLI JWT fallback, and broader quota/payment/frontend/migration buckets remain unsynced.

### 2026-06-27 - upstream safety fix batch 1 (OpenAI/apicompat/images)

- **Branch**: `codex/upstream-sync-20260627`
- **Baseline**: `origin/main@2c9a1e92` (`v0.1.148` fork release)
- **Upstream head observed**: `upstream/main@c2754222`
- **Strategy**: staged cherry-pick/manual port only; no full merge. The full upstream preview still has broad conflicts across generated Ent code, gateway services, billing/account paths, and frontend files.
- **Synced upstream commits**:
  - `29122e30` - avoid doubling `tool_call` arguments from single-chunk upstreams
  - `40c82527` - normalize custom tool schema in apicompat
  - `8a7269f5` - sanitize verbose OpenAI `response.failed` events
  - `cc7612bd` - detect OpenAI overloaded error codes
  - `0a97a5f4` - treat `refresh_token_invalidated` as non-retryable
  - `65fa7289` - fail over on Chat Completions transport errors
  - `9491de0a` - pass image content-moderation refusals through as 400 instead of retrying
- **Local reconciliation**:
  - Preserved local display-token rewrite behavior in streaming paths.
  - Adapted the Images refusal patch to the fork-local `OpenAIImageTrace` signature and response shape.
  - Added local follow-up `59300d06` so retryable `response.failed` markers are checked before generic `invalid_request` non-retryable handling.
- **Verification**:
  - `go test -tags=unit ./internal/pkg/apicompat`
  - `go test -tags=unit ./internal/service -run "Test(ExtractImagesUpstreamError|SummarizeNoOutputBody|ImagesOAuthNonStreaming|ExtractModelRefusal|HandleOpenAIUpstreamTransportError|ForwardAsRawChatCompletions_TransportErrorFailsOver|IsOpenAITransientProcessingError|OpenAIStreamingResponseFailed|OpenAIStreamingPassthroughResponseFailed|NonRetryableRefreshError)" -count=1 -v`
  - `git diff --check`
- **Not synced in this batch**:
  - Grok subscription stack.
  - `codex_cli_only` engine fingerprint/app-server hardening.
  - OpenAI PAT auth.
  - Admin CLI JWT fallback.
  - Broader migrations, quota, proxy, payment, and frontend buckets.

### 2026-06-02 — cherry-pick Opus 4.8 Antigravity 支持

- **上游 commit**: `514ac5c6` (`feat: 适配 claude-opus-4-8`)
- **合并策略**: 手工移植单个上游提交的模型支持面，未合并整个 `upstream/main`，避免引入无关上游变更。
- **本地适配**:
  - 保留本 fork 已有 `backend/migrations/144_distribution_api_key_recharge_wallet_totals.sql` 和 `145_add_user_downstream_usage_token_mode.sql`。
  - 上游新增迁移 `144_add_opus48_to_model_mapping.sql` 在本 fork 中改为 `146_add_opus48_to_model_mapping.sql`。
  - 保留本 fork 已有 Gemini 3.1、distribution、OpenAI Images 等二开改动，只增量加入 `claude-opus-4-8`。
- **重要上游变更**:
  - Antigravity 默认映射、模型列表、request transformer 高阶 Opus 判断支持 `claude-opus-4-8`。
  - Bedrock 默认映射支持 `claude-opus-4-8 -> us.anthropic.claude-opus-4-8-v1`。
  - 前端 Claude/Antigravity 模型白名单、预设映射、账号状态/用量展示加入 Opus 4.8。
  - 给已持久化 Antigravity `credentials.model_mapping` 回填 `claude-opus-4-8`。
- **验证**: 见 `docs/dev/CHANGELOG_CUSTOM.md` 同日记录。

### 2026-05-03 — v0.1.121 同步（v0.1.113 ~ v0.1.121，9 个版本）

- **上游版本范围**: `v0.1.113` ~ `v0.1.121`（`e534e9ba..48912014`）
- **合并策略**: 在 worktree `sync/upstream-v0.1.117` 上逐 tag merge，最后合入 main
- **合并顺序**: v0.1.113 → v0.1.114 → v0.1.115 → v0.1.116 → v0.1.117 → v0.1.118 → v0.1.119 → v0.1.120 → v0.1.121 → upstream/main

- **冲突处理**:
  - `wire_gen.go` / `wire.go`（v0.1.118/v0.1.119）: 二开的 ModelPricingHandler/PricingPageHandler/LoginPageHandler 与上游 AffiliateService/AffiliateHandler 合并
  - `AccountBulkActionsBar.vue` / `AccountsView.vue`（v0.1.120）: 上游 edit→edit-selected/edit-filtered 拆分 + 保留二开 auto-assign-proxy 按钮
  - v0.1.113 ~ v0.1.117 / v0.1.121: 零冲突或自动合并

- **合并后修复**:
  - **DefaultModels auto-include 恢复**: v0.1.120 merge 时上游重写了 `account.go:IsModelSupported()` 和 `account_handler.go:GetAvailableModels()`，丢失了二开的 `openai.IsDefaultModel()` fallback 和 `seen+merge` 逻辑，已手动恢复
  - **OpenAI 用户级模型定价修复**: 发现 OpenAI 计费路径 `calculateOpenAIRecordUsageCost` 未传 UserID 到 `CostInput`，导致用户级定价覆盖对 OpenAI 模型不生效（Anthropic/Antigravity 路径无此问题）。新增 `CostInput.UserID` 字段并在 `CalculateCostUnified` 内部 Resolve 时传递

- **重要上游变更摘要**:
  - **v0.1.113**: 支付系统 v2（手续费/移动端/退款）、Auth 身份体系（OAuth 绑定解绑）、余额/配额通知、WebSearch 仿真、License MIT→LGPL
  - **v0.1.114**: Opus 4.7 支持、prompt_cache_key 注入（OpenAI→Anthropic 路径）、KYC 阻断
  - **v0.1.115**: GPT 生图支持、Auth/支付加固、Profile 重设计、403 临时冷却逻辑、RPM 优化
  - **v0.1.116**: Channel Monitor MVP、Available Channels 聚合视图
  - **v0.1.117**: GPT-5.5 模型、Monitor 清理
  - **v0.1.118**: Claude Code 完整 mimicry、cache_control TTL 5m、Codex compact、affiliate 返利
  - **v0.1.119**: 真实 CC 客户端跳过 body mimicry（恢复 prompt caching）、affiliate 完善
  - **v0.1.120**: SetSnapshot race fix、Vertex SA、zstd 解压、account bulk edit、Fast/Flex Policy、Anthropic stream EOF failover
  - **v0.1.121**: Anthropic 缓存 TTL 注入开关、sticky session 改进、分页 localStorage

- **二开功能保留验证**:
  - 全局模型计费（GlobalModelPricingService / display_rate_multiplier / cache_transfer_ratio）✅
  - 用户级模型定价覆盖（UserModelPricingService）✅（+ OpenAI 路径 bug 修复）
  - GPT-5.5 DefaultModels auto-include ✅（恢复）
  - Antigravity 缓存修复（filterAnthropicBillingHeader / sessionIDFromMetadataUserID）✅
  - 页面内容编辑器（PricingPageHandler / LoginPageHandler）✅
  - Cache diagnostics 日志 ✅

### 2026-05-02 - v0.1.117 同步（已合入 main，见上方 v0.1.121 记录）

- **工作区/分支**: `E:\cursor project\api2sub-v117` / `sync/upstream-v0.1.117`
- **上游版本**: `v0.1.117`
- **合并提交**: `37519fcb` Merge tag `v0.1.117` into `sync/upstream-v0.1.117`
- **后续本地修复提交**:
  - `511e419b` fix(frontend): default locale and interpolation for v117
  - `64b5dff2` fix(frontend): add zh login locale keys
  - `243eae93` fix(frontend): add missing zh dashboard labels
  - `9ca7e522` fix(frontend): complete v117 zh locale coverage

- **关键处理**:
  - 将前端默认语言调整为 `zh`，避免默认进入英文界面。
  - 修复 vue-i18n 插值格式，避免充值/支付等金额变量显示异常。
  - 补齐 v117 新增/二开页面中文 locale，覆盖页面内容、登录页配置、定价页配置、模型配置、模型定价、API Key 引导、账号/用户/代理/使用记录、支付/充值/定价页等区域。
  - 补齐 `common.done` 到 en/zh，修复 API Key 引导中直接显示变量名的问题。

- **本地服务状态**:
  - 前端：`http://localhost:5180`
  - 后端：`http://localhost:18082`
  - 后端应以 `RUN_MODE=standard` 运行；`RUN_MODE=simple` 会导致管理员菜单被裁剪。

- **验证结果**:
  - `pnpm typecheck` 通过。
  - i18n key 对比：`missing zh count 0`。
  - 浏览器自动化抽查 `/pricing`、`/keys`、`/admin/model-config`、`/admin/page-content`、`/admin/users`、`/admin/accounts`、`/admin/proxies`、`/admin/usage`，未发现 raw i18n key 或 intlify missing-key 警告。
  - 管理员侧栏在 standard mode 下完整显示渠道管理、账号管理、模型配置、页面内容、订单管理、充值配置等菜单。

- **已知注意事项**:
  - 上游 `v0.1.117` tag 内 `backend/cmd/server/VERSION` 仍为 `0.1.116`，所以页面左上角显示 `v0.1.116` 是上游版本文件滞后，不代表运行错分支。
  - 如果浏览器仍显示少量菜单，优先退出重登或清理 localStorage，避免沿用 simple-mode 缓存用户态。
  - 当前记录的是独立 worktree 的合并验证进度，尚未 push，也未部署。

### 2026-04-14 - v0.1.112 同步（Cursor 兼容 + 支付/移动端修复）

- **上游 commit 范围**: `97f14b7a..e534e9ba`（17 commits）
- **合并策略**: `git merge upstream/main --no-ff`（保留 merge commit，便于回溯）
- **冲突文件**: **无**。所有上游改动文件与本地二开改动文件完全不重叠，自动合并全部成功

- **重要上游变更**:
  - **Cursor 兼容修复**（`openai_gateway_chat_completions.go` / `openai_codex_transform.go`）：
    - 兼容 Cursor `/v1/chat/completions` 传入的 Responses API body
    - Cursor raw body 透传路径剥离 Codex 不支持的 Responses API 参数
  - **Anthropic 非流式空 output 修复**（`openai_gateway_messages.go`）：
    终态事件 output 为空时从 delta 事件重建响应内容，避免空响应
  - **支付系统修复**（`payment/*`）：
    - Alipay/Wxpay direct provider 类型映射修复
    - 启用跨提供商负载均衡
    - 订单过期逻辑微调
  - **前端移动端修复**：
    - `DataTable.vue` 手机端双重渲染问题
    - `AccountUsageCell.vue` 引入 IntersectionObserver 懒加载（**注意见下**）
    - 版本下拉在手机端不再被裁剪（新增 `AppSidebar.spec.ts`）
    - 支付二维码降低纠错等级降低密度
  - **新 migration `097_fix_settings_updated_at_default.sql`**：恢复
    `settings.updated_at` 字段的默认值（之前迁移误丢）
  - VERSION bump: `0.1.111 → 0.1.112`
  - README 三语言：添加 aigocode 合作伙伴

- **合并后验证**:
  - `go build ./...` ✅
  - `go test -tags=unit ./internal/service/... ./internal/handler/... ./internal/payment/...` 全绿（76s）
  - `pnpm run typecheck` ✅
  - `pnpm run test:run`: **14 failed / 295 passed**
    - **8 失败是合并前就存在的**（用合并前的 `AccountUsageCell.vue` 跑同样是 8 failed），与本次同步无关
    - **6 新失败由上游 PR `abe42675` 引入**：该 PR 为 `AccountUsageCell.vue` 加了
      IntersectionObserver 懒加载（`hasEnteredViewport` ref），但没同步更新
      `__tests__/AccountUsageCell.spec.ts` 的 mock。jsdom 环境下观察器不会触发，
      所以组件一直处于未"进入视口"状态，断言全部失败
    - **评估**：这是上游 PR 的测试债，不影响生产行为；无需本地修复，等上游跟进
      （或者后续独立提 PR 修 mock）。本次同步不为此 block

- **本地二开改动保留情况**:
  - 全局定价覆盖修复（commit `dec95c75`）— 未被触碰 ✅
  - 代理批量导入格式扩展 — 未被触碰 ✅
  - Gemini google_one 批量 RT 导入 — 未被触碰 ✅
  - Model Config 页面（model-pricing/*）— 未被触碰 ✅
  - `docs/dev/codebase/` 二开文档 — 未被触碰 ✅

- **下次合并潜在冲突区域**: 若上游将来重构 `gateway_service.calculateTokenCost`
  或 `model_pricing_resolver` 需要重新整合本地 Bug A-C 的修复（详见
  `docs/dev/CHANGELOG_CUSTOM.md` 2026-04-14 第一条）

- **部署**: 已于 2026-04-14 部署到生产（`sub2api-custom:latest` 重建 + 健康检查通过）。部署指令：
  ```bash
  ssh -i ~/.ssh/id_ed25519_sub2api root@172.245.247.80 "bash /opt/sub2api/update.sh"
  ```

### 2026-04-12 — 初始克隆

- **上游 commit**: `97f14b7a` (Merge PR #1572 feat/payment-system-v2)
- **冲突**: 无（首次克隆）
- **备注**: 项目初始化，无二开修改

<!--
模板：

### YYYY-MM-DD — 简述

- **上游 commit 范围**: `abc1234..def5678`
- **重要上游变更**: 
  - xxx
  - xxx
- **冲突文件**:
  - `path/to/file.go` — 解决方式说明
- **合并后测试**: 通过 / 失败（说明）
- **备注**: 
-->
# Upstream Sync Notes

## 2026-06-07 - Staged OpenAI/Codex sync through Phase 6.5

- **Branch**: `codex/openai-codex-upstream-sync`
- **Local baseline**: clean local secondary-development baseline before staged
  sync (`850b9f0a` in the planning notes).
- **Upstream target**: `upstream/main@635ad81c`
- **Strategy**: manual staged sync and small cherry-pick/port batches only. No
  full `git merge upstream/main` was used after the bad all-at-once merge was
  reverted.
- **Latest local sync commit in this phase**: `9f0742a7` (`fix: sync phase 6.5
  long-context billing`)
- **Pushed/deployed**: no.

### Synced Scope

- Phase 0/1 protection and safety: upstream-sync guard, i18n/menu checks,
  reusable real-request smoke tooling, selected API-key and Images safety fixes.
- Phase 2 data model union: upstream migrations appended locally as `150-166`;
  existing local migrations and custom schema were not renumbered or rewritten.
- Phase 3 OpenAI/Codex core: upstream OpenAI Messages / Claude-GPT bridge core,
  request conversion, Codex transform, continuation/digest, replay/todo guards,
  tool pairing, terminal events, and `response.failed` handling.
- Phase 4/4.5 WS, Images, Embeddings, account controls: OpenAI WS HTTP bridge,
  Images routing/cooldown/metadata/error passthrough, OpenAI-compatible
  `/v1/embeddings`, account Codex image bridge controls, and local independent
  `extra.openai_images_endpoint_enabled` for `/v1/images/*`.
- Phase 5 stable core: leader locks, Redis Lua `TIME` compatibility, Postgres
  bootstrap, account/user cleanup, scheduler snapshot refresh, usage
  failed/error request display, ops error attribution, and group custom
  `/v1/models` list.
- Phase 6/6.5 OpenAI/Codex follow-up fixes: error/stream terminal semantics,
  usage/request context propagation, response-id account binding, WS failover,
  request hotpath/OOM reductions, apicompat audit, OAuth 401 credential safety,
  Codex/Claude Code mimicry updates, and long-context cache-read/cache-creation
  multiplier fixes.

### Preserved Local Secondary Development

- Claude-GPT bridge for Antigravity dispatch, including scheduler cache fields,
  stale DB refresh, bridge usage model semantics, cache display, and
  `prompt_cache_key` preservation.
- Display token and display pricing behavior; display rewriting remains
  downstream-only and does not alter billing, stored usage, quota, or account
  stats.
- Global and user model pricing, including user override priority over
  channel/global/base pricing.
- Distribution user/admin API and UI.
- Public unauthenticated `/key-usage` page and `/v1/usage*` API-key usage
  endpoints.
- AI credit snapshot, custom announcement surfaces, InvokeAI/AIClient2API/a2-proxy
  docs and local dev-stack behavior.
- `OPENAI_IMAGE_TRACE_LOG` remains opt-in and safe-field-only.

### Verification Summary

- Unit/static gates passed during closeout:
  - `go run ./tools/upstream-sync-guard`
  - `git diff --check`
  - `go test -tags=unit ./internal/pkg/apicompat ./internal/pkg/openai ./internal/pkg/openai_compat`
  - `go test -tags=unit ./internal/service ./internal/handler -run "OpenAI|Codex|Responses|Chat|Messages|WS|Usage|OAuth|Image|Bridge"`
  - `go test -tags=unit ./internal/service ./internal/handler ./internal/repository ./internal/server`
  - `go test -tags=unit ./internal/service -run "Billing|Pricing|LongContext|DisplayToken|UserModelPricing|GlobalModelPricing"`
- Real-request smoke:
  - `go run ./tools/smoke --suite openai,bridge,images,custom` passed 28/28 on
    2026-06-07.
  - Covered OpenAI `/v1/responses`, OpenAI `/v1/chat/completions`,
    Antigravity Claude-GPT bridge, Images upstream 400 passthrough, distribution,
    pricing pages/API, public `/key-usage`, announcements, usage errors, and
    group models-list candidates.
  - `go run ./tools/smoke --suite embeddings` now selects and forwards to the
    local OpenAI API-key account, but the configured upstream
    `https://api.1188soft.com/` returns `404 page not found` for
    `/v1/embeddings`. This is an upstream/base-URL fixture compatibility issue,
    not a Sub2API account-selection or route-registration failure.

### Deferred Upstream Items

- user x platform quota service/UI and quota flusher integration.
- risk-control/content moderation APIs and frontend page.
- channel monitor OpenAI API mode.
- account quota auto-pause/window tooltip changes.
- payment/Airwallex/multi-currency, DingTalk/email/legal/marketing changes.
- account page large re-layout, Codex session import, and upstream model sync
  preview.
- broader pricing or image-output-token channel override changes not covered by
  the long-context regression tests.

### Follow-Up Notes

- Keep future upstream syncs staged. Do not use an all-at-once full merge for
  OpenAI/Codex, billing, quota, or frontend account-page areas.
- Bridge smoke requires an OpenAI account bound to the Antigravity group with
  `extra.openai_claude_gpt_bridge_enabled=true` and a Claude-to-GPT
  `credentials.model_mapping` entry.
- Embeddings smoke requires an OpenAI API-key account whose model mapping
  includes the requested embedding model, and whose upstream base URL exposes an
  OpenAI-compatible embeddings endpoint.
