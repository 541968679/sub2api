# Sub2API 二开变更日志

> 记录所有相对于上游 (Wei-Shaw/sub2api) 的自定义修改。每次二开变更必须在此记录，便于合并上游更新时追踪差异。

## 格式说明

```
## [日期] 类别: 简短描述

**影响范围**: 涉及的模块/文件
**上游兼容性**: 是否可能与上游更新冲突
**变更详情**:
- 具体修改内容

**关联 Issue/PR**: #xxx（如有）
```

---

## 变更记录

## [2026-04-18] fix(settings): 登录页价格动态化 + 修复充值管理保存误清空注册等设置

**影响范围**:
- `backend/internal/service/settings_view.go` — `PublicSettings` 新增 `PaymentCNYPerUSD float64`
- `backend/internal/service/setting_service.go` — `GetPublicSettings` 读取 `SettingCNYPerUSD`；`GetPublicSettingsForInjection` 注入匿名结构体同步新增字段
- `backend/internal/handler/dto/settings.go` — 公开设置 DTO 新增 `payment_cny_per_usd`
- `backend/internal/handler/setting_handler.go` — 在 `GetPublicSettings` 响应里填充新字段
- `frontend/src/types/index.ts` — `PublicSettings` 接口新增 `payment_cny_per_usd: number`
- `frontend/src/stores/app.ts` — 默认空配置补齐 `payment_cny_per_usd: 0`
- `frontend/src/i18n/locales/zh.ts`、`en.ts` — `featurePrice` 改为带 `{price}` 占位的模板；新增 `featurePriceDefault` 作为未配置时的回退文案
- `frontend/src/views/auth/LoginView.vue` — 新增 `paymentCnyPerUsd` ref，`onMounted` 从公开设置读取；feature pill 按配置动态渲染，未配置回退
- `frontend/src/api/admin/settings.ts` — 新增 `systemSettingsToUpdateRequest(SystemSettings) => UpdateSettingsRequest` 映射函数；注入 `settingsAPI`
- `frontend/src/views/admin/RechargeConfigView.vue` — `save()` 先 `getSettings()` 再整体 `updateSettings(...)`，只覆盖 `payment_cny_per_usd` / `payment_bonus_tiers`

**上游兼容性**:
- 后端新增字段为可选追加，合并上游时若上游也改动 `PublicSettings` / 公开设置 handler，留意冲突位置（均为结构体尾部或 return 字段列表）
- 前端新增的 `systemSettingsToUpdateRequest` 是本地二开工具函数，独立于上游

**变更详情**:
- Bug 1 — 登录页价格硬编码：`LoginView` 原先渲染 `t('auth.login.featurePrice')` 的静态文案 `'0.6 / 1$ 起'`，与 admin 在"充值管理"设置的 `payment_cny_per_usd` 完全脱钩。现将该汇率通过 `/api/v1/settings/public` 暴露（与 SSR 注入路径保持一致），前端读取后以 `{price} / 1$ 起` 模板渲染；为 0 或未配置时回退到 `featurePriceDefault` 静态文案。
- Bug 2 — "每次部署开放注册被重置"：真正根因不是部署脚本。后端 `UpdateSettingsRequest` 绝大多数 `bool` / `string` 字段是**非指针**，JSON 反序列化时缺失字段会被填 `false` / `""`；`RechargeConfigView.save()` 只发 `payment_cny_per_usd` 与 `payment_bonus_tiers`，handler 继续构造完整 `SystemSettings` 并 `SetMultiple` 回写，导致 `registration_enabled`、`site_name`、OIDC/LinuxDo 开关等被静默清空。修复采用最小改动：`RechargeConfigView` 先拉完整 settings，用新建的映射函数转成请求体，再覆盖两个 payment 字段发出，使回写是"读旧值写旧值"，避免误清空。凭据类字段（`smtp_password` 等）在映射函数中故意留空，后端"空值跳过覆盖"守护继续生效。

**验证方式**:
- `go build ./...` 通过；前端 `pnpm run typecheck` 通过；handler 相关单测通过（service 层受 `gemini_oauth_service_test.go` 预存在的 mock 接口不完整影响，未新增测试失败）
- 手工：充值管理保存 `cny_per_usd=0.8` → 登录页显示 `0.8 / 1$ 起`；同时系统设置里"开放注册"等开关保持用户之前的值不变


**影响范围**:
- `backend/ent/schema/ai_credit_snapshot.go` — 新 Ent schema：`AICreditSnapshot { email, credit_type, amount, captured_at }` + 复合索引
- `backend/ent/aicreditsnapshot/`、`backend/ent/aicreditsnapshot*.go` — Ent 生成代码（`go generate ./ent`）
- `backend/migrations/110_add_ai_credit_snapshots.sql` — 建表 + `(email, captured_at)` 与 `(captured_at)` 索引
- `backend/internal/service/credit_snapshot.go` — `CreditSnapshot` 结构、`CreditSnapshotRepository`、`AntigravityUsageAggregator`、`AntigravityUsageRatio` 响应类型
- `backend/internal/service/credit_snapshot_service.go` — `CreditSnapshotService`：15 分钟 ticker 定时采样、`TriggerManualCapture`（30 秒进程内冷却锁）、`GetAntigravityUsageRatio`（相邻采样点正向 delta 求和 + `usage_logs` 聚合）
- `backend/internal/repository/credit_snapshot_repo.go` — 基于 Ent 的仓库实现（Insert/ListInRange/GetLatestBefore）
- `backend/internal/repository/antigravity_usage_aggregator.go` — 独立小接口实现：`SELECT COUNT + SUM(total_cost) FROM usage_logs WHERE account_id = ANY($1) AND created_at ∈ [start,end)`
- `backend/internal/handler/admin/usage_handler.go` — `NewUsageHandler` 加 `creditSnapshotService` 依赖；新增 `StatsAntigravity` / `RefreshAntigravityStats`；提取 `parseStatsDateRange` 辅助函数
- `backend/internal/handler/admin/{usage_cleanup_handler_test,usage_handler_request_type_test}.go` — stub 补齐新参数位 `nil`
- `backend/internal/server/routes/admin.go` — `GET /admin/usage/stats/antigravity`、`POST /admin/usage/stats/antigravity/refresh`
- `backend/internal/service/wire.go` — 新增 `ProvideCreditSnapshotService` 并入 `ProviderSet`
- `backend/internal/repository/wire.go` — `NewCreditSnapshotRepository` / `NewAntigravityUsageAggregator` 加入 `ProviderSet`
- `backend/cmd/server/wire_gen.go` — 手动编排新 Repo + Service + Handler 依赖（主干 `go generate` 因历史 Payment 重复绑定失败，按现有模式插入）
- `frontend/src/api/admin/usage.ts` — 新增 `AntigravityUsageRatio` 类型、`getAntigravityStats`、`refreshAntigravityStats`
- `frontend/src/components/admin/usage/AntigravityRatioCard.vue` — 新组件：4 列指标卡 + 「立即采样」按钮 + 采样不足/冷却提示
- `frontend/src/views/admin/UsageView.vue` — 引入卡片，与现有 `UsageStatsCards` 共用 `DateRangePicker`，同一刷新链路触发
- `frontend/src/i18n/locales/{zh,en}.ts` — 新增 `usage.antigravity.*` 文案

**上游兼容性**: 低。所有新增文件/字段均为 additive；仅 `admin/usage_handler.go` 构造器加参数（上游若重构 handler 初始化签名需同步）；`wire_gen.go` 仍需手工合并。`AntigravityUsageAggregator` 刻意没接入 `UsageLogRepository` 接口，避免日后改动十几处 stub。

**变更详情**:
1. Antigravity AI Credits 余额不可回溯查询（远端 API 只给当前值），因此新增 `ai_credit_snapshots` 表。`CreditSnapshotService` 每 15 分钟启动一次采样：按 `credentials.email` 去重（同 Google 账号共享 credits），复用 `AccountUsageService.GetUsage` 的 3 分钟缓存层拉余额，避免额外 API 压力。
2. 聚合口径：对每个 email 在 `[start - 30 min lookback, end]` 内的快照按时间升序走相邻对，累加正向 delta。负向 delta（充值/重置）跳过。派生比率 `quota_per_credit = SUM(total_cost) / total_credits`、`calls_per_credit = COUNT(*) / total_credits`，`total_credits == 0` 时返回 null（前端展示"采样不足"提示）。
3. 手动触发接口 `POST .../refresh` 加 30 秒进程内冷却锁（`sync.Mutex + lastManualAt`），冷却期内返回 `manual_refresh_throttled=true` 并不重复打远端。管理员误点不会放大 API 压力。
4. 前端卡片接入现有 `startDate`/`endDate`，`loadStats()` 结束后并行拉 antigravity 聚合；失败只 `console.error` 不阻断主流程。
5. 验证：`docker exec sub2api-pg-dev psql` 确认 migration 110 应用、`ai_credit_snapshots` 表结构正确；本地启动后 `[CreditSnapshot] Scheduler started` 与路由 `GET/POST /api/v1/admin/usage/stats/antigravity(/refresh)` 均已注册。

**关联 Issue/PR**: 无

---

## [2026-04-18] fix(keys): 修正「入门指南」里 CC-Switch 的下载地址

**影响范围**:
- `frontend/src/components/keys/GettingStartedGuide.vue` — 第二步下载按钮 `href` 从 `github.com/nicepkg/cc-switch/releases`（错误仓库）改为 `github.com/farion1231/cc-switch/releases`（官方仓库）

**上游兼容性**: 低。上游若未使用此链接则无冲突。

**关联 Issue/PR**: 本地二开需求

---

## [2026-04-19] feat(login-page): 左栏营销区改版：4 张 feature 卡 + 推广邀请

**影响范围**:
- `frontend/src/views/auth/LoginView.vue` — 删除左栏下半区的 feature pills、模型展示网格、3 张旧 feature cards 和不再使用的 `modelChannels` / `paymentCnyPerUsd` / `loginSupportedModelsTitle` / `loginModelsDesc`；新增 2×2 的 4 张 feature 卡片（计算属性 `featureCards`）与推广邀请强调区块
- `frontend/src/i18n/locales/{zh,en}.ts` — 新增 `auth.login.features.{metered,quality,models,enterprise}.{title,desc}` + `auth.login.referral.{tag,title,body}` 两组键；保留 `featurePrice`、`featureUnifiedApi*` 等旧键不动（避免影响其他组件 / 防止上游冲突），只是登录页模板不再引用

**上游兼容性**: 低。前端样板重写 + 新增 i18n；后端、数据库不动。

**变更详情**:
1. 顶部区仍由 badge / 两行标题 / description 组成，沿用之前的管理员可编辑覆盖机制（`login_page.*` settings 字段）。
2. 下半区一次放完 4 张卡片 + 1 张推广邀请卡，视觉层级：feature 卡（中性深色底）→ 推广卡（青绿渐变 + 荧光描边）把重点拉开。
3. 4 张卡片当前走 i18n 硬编码（文案稳定），后续若需管理员可编辑，加字段到 `LoginPageContent` 即可。
4. 推广邀请 `body` 为占位稿，等最终文案确定后直接改 i18n 或升级为管理员可编辑字段。
5. 管理员编辑器里的 `supportedModelsTitle`、`modelsDesc` 两字段本次起不再影响登录页渲染（保留字段暂不删，后续统一清理）。

**关联 Issue/PR**: 本地二开需求

---

## [2026-04-18] refactor(page-content): 合并「计价页文案」和「登录页文案」为统一 Tab 页

**影响范围**:
- `frontend/src/views/admin/PageContentView.vue` — 新增合并父视图：`AppLayout` + 共享头部 + 两个 tab（模型计价页 / 登录页） + `?tab=pricing|login` URL 同步 + `<KeepAlive>` 保留表单输入不丢失
- `frontend/src/components/admin/page-content/PricingContentForm.vue` — 由 `PricingPageView.vue` 剥出 AppLayout/页标题后得到，仅保留提示卡、两段 textarea、保存按钮
- `frontend/src/components/admin/page-content/LoginContentForm.vue` — 由 `LoginPageView.vue` 剥出 AppLayout/页标题后得到，保留三组 8 字段 + 清空/保存/预览
- `frontend/src/views/admin/PricingPageView.vue`、`frontend/src/views/admin/LoginPageView.vue` — 删除
- `frontend/src/router/index.ts` — 新 `/admin/page-content` 路由；`/admin/pricing-page`、`/admin/login-page` 保留为 redirect 到新路径并带上 `?tab=` 参数，老书签不失效
- `frontend/src/components/layout/AppSidebar.vue` — 管理员侧边栏去掉两条旧项，合成一条「页面文案」
- `frontend/src/i18n/locales/{zh,en}.ts` — 删 `nav.pricingPage` / `nav.loginPage`；新增 `nav.pageContent` + `admin.pageContent.{title,description,tabs.{pricing,login}}`；保留 `admin.pricingPage.*` / `admin.loginPage.*`（两个子组件仍然消费）

**上游兼容性**: 低。只动前端，后端 handler 和设置 key 不变。

**变更详情**:
1. 合并动机：两块都是「前台页面文案管理」，拆两个侧边栏条目偏冗余；未来如果还要加新页面（例如仪表盘、404 页）统一放进这个 tab 页即可。
2. Tab 切换通过 URL `?tab=...` 同步，便于深链接 + 浏览器前进/后退；未指定时默认 `pricing`。
3. `<KeepAlive>` 保留子组件状态，用户在两个 tab 之间切换时未保存的编辑不会丢。
4. 老路径保留 redirect 到新路径，旧书签平滑过渡。

**关联 Issue/PR**: 本地二开需求（紧接两次文案功能合并）

---

## [2026-04-18] feat(login-page): 管理员可编辑登录页文案

**影响范围**:
- `backend/internal/service/domain_constants.go` — 新增 8 个 `SettingKeyLoginPage*` 常量
- `backend/internal/service/settings_view.go` — `LoginPageContent` 结构（json tag + `IsEmpty`）；`PublicSettings.LoginPage *LoginPageContent`
- `backend/internal/service/setting_service.go` — `GetPublicSettings` 加 8 个 key 到批量读取列表；新增 `buildLoginPageContent`（空字段 trim 后整体 nil 化）；`GetPublicSettingsForInjection` 的匿名 struct 也加 `login_page`
- `backend/internal/handler/dto/settings.go` — `PublicSettings` DTO 加 `LoginPage *LoginPageContent`；新增 `dto.LoginPageContent`
- `backend/internal/handler/setting_handler.go` — 公开 `/settings/public` 输出映射 + `toDTOLoginPageContent` 辅助函数
- `backend/internal/handler/admin/login_page_handler.go` — 新增：GET/PUT `/admin/login-page/content`；字段级 trim + 长度校验（short 255 / long 500）
- `backend/internal/handler/handler.go` + `wire.go` + `backend/cmd/server/wire_gen.go` — `AdminHandlers.LoginPage` + provider，手动插入 wire_gen 与 pricing-page 保持同一模式
- `backend/internal/server/routes/admin.go` — `registerLoginPageRoutes`
- `frontend/src/api/loginPage.ts` — 新增 API client（`getAdminLoginPageContent` / `updateAdminLoginPageContent` / `resetAdminLoginPageContent`）
- `frontend/src/api/index.ts` — 导出
- `frontend/src/types/index.ts` — `LoginPageContent` 接口；`PublicSettings.login_page?` 可选字段
- `frontend/src/views/auth/LoginView.vue` — 8 处 `t('auth.login.xxx')` 替换为 `loginXxx` computed；每个 computed 都用 `pickLoginText` 做 fallback（空串/未定义时用 i18n 原文）
- `frontend/src/views/admin/LoginPageView.vue` — 新增管理员编辑页：3 个小分组（营销/模型区/登录框）8 个字段表单 + 预览链接 + 保存 + 恢复默认（带 confirm）；保存/恢复后触发 `appStore.fetchPublicSettings(true)` 立刻让其他未刷新的页面看到新值
- `frontend/src/components/layout/AppSidebar.vue` — `adminNavItems` 增加「登录页文案」入口
- `frontend/src/router/index.ts` — `/admin/login-page` 路由
- `frontend/src/i18n/locales/{zh,en}.ts` — `nav.loginPage` + `admin.loginPage.*`（title/description/preview/fallbackHint/sections/fields 8 项/save/reset/reset-confirm）

**上游兼容性**: 中。`PublicSettings` 结构被扩展（service + DTO + TS 类型），上游若将来改动这个结构需要同步；新增 key 命名用 `login_page.*` 命名空间，不与既有 key 冲突。路由 / handler / 前端文件都是新增，不覆盖上游。`wire_gen.go` 仍需手动合并。

**变更详情**:
1. 8 个 settings key（`login_page.badge` / `heading_line1` / `heading_line2` / `description` / `supported_models_title` / `models_desc` / `form_title` / `form_subtitle`）一一对应 i18n `auth.login.*` 里的营销文案字段。
2. 任意字段空字符串 → 后端返回的 `LoginPage` 子结构为 nil（`omitempty` 整体 omit），前端拿不到就继续用 `t('auth.login.xxx')`，中英切换自动生效。
3. 管理员保存后调用 `appStore.fetchPublicSettings(true)` 强制重新拉取 public settings，避免其他已打开的页面看到旧版。
4. 「恢复默认」= 批量写入空串，不是物理删 key；语义更明确，且不用加删除接口。
5. SSR 注入的 `window.__APP_CONFIG__` 也同步更新（`GetPublicSettingsForInjection`），首次渲染登录页就是最终文案，不闪屏。
6. 验证：`curl /api/v1/settings/public | grep login_page` → 未保存时无 key；登录后 `curl /admin/login-page/content` 返回 8 字段全空对象；保存后 public 接口开始返回 `login_page` 子结构。

**关联 Issue/PR**: 本地二开需求（续「模型计价页文案」）

---

## [2026-04-18] fix(pricing-page): 管理员编辑页未保存时预填默认文案

**影响范围**:
- `backend/internal/handler/admin/pricing_page_handler.go` — 导出 `DefaultPricingPageIntro` / `DefaultPricingPageEducation` 常量；`Get` 在 settings 未写 / 空串时回落到默认值；`loadValue` 多一个 fallback 入参
- `backend/internal/handler/pricing_page_handler.go` — 删掉本地默认常量，复用 `admin.Default*`

**上游兼容性**: 低。纯字段级调整，无 schema / 路由变化。

**变更详情**: 原先管理员进编辑页时 settings 里还没写入，两个 textarea 都是空的，但用户计价页又显示的是 handler 内置默认文案，导致「编辑不到用户看到的东西」。现在 admin Get 接口与用户侧共用同一份常量，管理员第一次进来就能看到「用户此刻实际在看的内容」，直接改就行。

**关联 Issue/PR**: 本地二开需求（上条变更的后续）

---

## [2026-04-18] feat(pricing-page): 新增用户「模型计价」页 + 管理员可编辑文案

**影响范围**:
- `backend/migrations/109_add_show_on_pricing_page.sql` — `global_model_pricing` 新增 `show_on_pricing_page BOOLEAN`
- `backend/internal/service/global_model_pricing.go` — `GlobalModelPricing` 加 `ShowOnPricingPage` 字段；接口新增 `ListForPricingPage`
- `backend/internal/repository/global_model_pricing_repo.go` — 所有 SELECT/INSERT/UPDATE 同步新字段；新增 `ListForPricingPage`
- `backend/internal/service/global_model_pricing_service.go` — `GlobalOverride` DTO 加 `show_on_pricing_page`；`ToGlobalOverride` 同步；新增 `ListForPricingPage` 方法
- `backend/internal/handler/admin/model_pricing_handler.go` — Create/Update 请求 DTO 加 `show_on_pricing_page *bool`
- `backend/internal/handler/admin/pricing_page_handler.go` — 新增：GET/PUT `/admin/pricing-page/content`，读写 `settings` KV 两个 key
- `backend/internal/handler/pricing_page_handler.go` — 新增用户侧：GET `/user/pricing-page`，聚合两段文案 + 按 provider 分组的展示价格
- `backend/internal/handler/handler.go` — `AdminHandlers.PricingPage`、`Handlers.PricingPage` 新字段
- `backend/internal/handler/wire.go` — 注册 `NewPricingPageHandler` / `NewPricingPageAdminHandler`
- `backend/cmd/server/wire_gen.go` — 手动编排新 handler 依赖（`go generate` 在主干已预先失败，按现有模式插入）
- `backend/internal/server/routes/admin.go` — `registerPricingPageRoutes`
- `backend/internal/server/routes/user.go` — 注册 `/user/pricing-page`
- `frontend/src/api/pricingPage.ts` — 新增 API client（用户 Get + 管理员 Get/Update）
- `frontend/src/api/index.ts` — 导出 `pricingPageAPI`
- `frontend/src/api/admin/modelPricing.ts` — `GlobalOverride`/`CreateOverrideRequest`/`UpdateOverrideRequest` 加 `show_on_pricing_page`
- `frontend/src/views/user/PricingView.vue` — 新增用户页：三节内容（本站计价模式 / 计价模式科普 / 按平台分组的价格表），Markdown 用 `marked@17` + `DOMPurify` 渲染
- `frontend/src/views/admin/PricingPageView.vue` — 新增管理员页：两段 textarea 编辑 + 保存 + 指向模型配置的引导
- `frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue` — 编辑对话框加「在计价页展示」开关
- `frontend/src/components/layout/AppSidebar.vue` — 用户/个人侧边栏新增「模型计价」菜单；管理员侧边栏新增「计价页文案」入口；新增 `PriceTagIcon`
- `frontend/src/router/index.ts` — 新增 `/pricing` 与 `/admin/pricing-page` 路由
- `frontend/src/i18n/locales/{zh,en}.ts` — 新增 `pricing.*`、`admin.pricingPage.*`、`admin.modelPricing.showOnPricingPage` 键以及 `nav.modelPricing`、`nav.pricingPage`

**上游兼容性**: 中。新增字段 `show_on_pricing_page` 位于 `global_model_pricing` 表，迁移是 additive，上游若将来对该表结构做改动需手动合并。Handler / 路由均为新增，不覆盖上游文件的既有路径。`wire_gen.go` 手动编辑（因主干 Wire 生成预先失败，`ProvidePaymentConfigService` 等重复绑定），合并上游时需留意。

**变更详情**:
1. 管理员可在「模型配置 → 模型详情」里勾选「在计价页展示」，控制哪些模型出现在用户侧的计价页，独立于计费 `enabled` 开关。
2. 管理员可在 `/admin/pricing-page` 编辑两段 Markdown 文案（本站计价模式、计价模式科普），保存到 `settings` 表的 `pricing_page.intro_markdown` / `pricing_page.education_markdown` 两个 key。未保存时用户侧回落到 handler 内置默认文案。
3. 用户 `/pricing` 页一次拉取聚合接口：返回两段文案 + 按 provider 分组的展示价格表。展示价的优先级：用户级 display override > 全局 display override > 真实单价（fallback）。
4. 价格表 per-token 价按 $/MTok 显示，per_request 按 $/次 显示。
5. i18n 已补 zh/en 完整键值。

**关联 Issue/PR**: 本地二开需求

---

## [2026-04-17] feat(billing): 用户级模型定价覆盖 (User Model Pricing Override)

**影响范围**:
- `backend/migrations/106_add_user_model_pricing_overrides.sql` — 新增表
- `backend/internal/service/user_model_pricing.go` — 实体 + 仓储接口
- `backend/internal/service/user_model_pricing_service.go` — 业务逻辑层
- `backend/internal/repository/user_model_pricing_repo.go` — 原生 SQL 实现
- `backend/internal/service/model_pricing_resolver.go` — PricingInput 增加 UserID, Resolve 增加用户级覆盖叠加
- `backend/internal/service/gateway_service.go` — 传递 UserID 到定价解析链路
- `backend/internal/handler/dto/display_pricing.go` — 新增 BuildUserDisplayPricingMap
- `backend/internal/handler/usage_handler.go` — 使用用户级展示覆盖
- `backend/internal/handler/admin/user_model_pricing_handler.go` — Admin CRUD API
- `backend/internal/service/global_model_pricing_service.go` — 列表增加 user_override_count, 详情增加 user_overrides
- `backend/internal/service/admin_service.go` — 用户删除时级联清理
- `backend/internal/handler/handler.go` — AdminHandlers 增加 UserModelPricing 字段
- `backend/internal/handler/wire.go` — 注册新 handler
- `backend/internal/repository/wire.go` — 注册新 repo
- `backend/internal/service/wire.go` — 注册新 service
- `backend/internal/server/routes/admin.go` — 注册新路由
- `frontend/src/api/admin/userModelPricing.ts` — 前端 API 客户端
- `frontend/src/components/admin/user/UserModelPricingModal.vue` — 管理模态框
- `frontend/src/views/admin/UsersView.vue` — 用户操作菜单增加"模型定价"入口
- `frontend/src/i18n/locales/en.ts` — 国际化文案

**说明**: 新增用户级模型定价覆盖功能，支持管理员为特定用户的特定模型设置：
1. 真实计费价格覆盖（input_price, output_price, cache_write_price, cache_read_price）
2. 展示价格覆盖（display_input_price, display_output_price, display_rate_multiplier, cache_transfer_ratio）

完整定价优先级链：用户 > 渠道 > 全局 > LiteLLM/Fallback。不影响现有的全局覆盖、渠道覆盖、分组倍率和用户分组倍率机制。

## [2026-04-17] feat(billing): 用户级展示倍率 (User Display Rate Multiplier)

**影响范围**:
- `backend/migrations/104_add_display_rate_multiplier.sql` — 新增
- `backend/internal/service/user_group_rate.go` — 扩展 UserGroupRateEntry, GroupRateMultiplierInput, 新增 UserGroupRateData
- `backend/internal/repository/user_group_rate_repo.go` — 支持 display_rate_multiplier 读写
- `backend/internal/handler/dto/display_pricing.go` — 新增 ApplyUserDisplayRate()
- `backend/internal/handler/usage_handler.go` — 使用记录应用用户级展示变换
- `backend/internal/handler/api_key_handler.go` — /groups/rates 返回展示倍率
- `backend/internal/service/api_key_service.go` — 新增 GetUserGroupRatesFull()
- `backend/internal/service/admin_service.go` — UpdateUser 支持 GroupRatesFull
- `backend/internal/handler/admin/user_handler.go` — 支持 group_rates_full
- `frontend/src/types/index.ts` — 新增 UserGroupRateData, group_display_rates
- `frontend/src/api/groups.ts` — 返回 UserGroupRateData
- `frontend/src/views/user/KeysView.vue` — GroupBadge 展示展示倍率
- `frontend/src/components/admin/user/UserAllowedGroupsModal.vue` — 展示倍率编辑UI
- `frontend/src/i18n/locales/{en,zh}.ts` — 国际化

**上游兼容性**: 低冲突风险，新增字段和方法，不修改现有逻辑

**变更详情**:
- 管理员可为每个用户在每个分组设置独立的"展示倍率"，用户看到展示倍率而非真实计费倍率
- 展示倍率独立于真实专属倍率，即使用户使用分组默认倍率也可单独设展示倍率
- 使用记录通过缩放 token 数量实现自洽：actual_cost 不变，total_cost × display_rate ≈ actual_cost
- 与模型级展示价格链式叠加，用户级优先级更高

## [2026-04-16] fix(pricing): 修复编辑用户展示设置后模型价格接口500错误

**影响范围**:
- `backend/internal/repository/global_model_pricing_repo.go`

**上游兼容性**: 无冲突，修复自己引入的bug

**变更详情**:
- `GetByID` 和 `GetByModel` 方法 SELECT 了 18 列但 Scan 只接收 14 个字段
- 漏掉了 `display_input_price`, `display_output_price`, `display_rate_multiplier`, `cache_transfer_ratio` 四个字段
- 当 display 字段为 NULL 时偶尔不报错，设置了非 NULL 值后必现 500

## [2026-04-16] feat(deploy): 安全部署脚本，支持自动回滚

**影响范围**:
- `deploy/update.sh`（新增）

**上游兼容性**: 无冲突，新增独立文件

**变更详情**:
- 构建到临时 staging tag，旧镜像在构建期间保持不变
- 保留上一个版本镜像 (`sub2api-custom:prev`) 用于即时回滚
- 部署后 health check 失败自动回滚到前一版本
- 支持 `--rollback` 手动回滚
- 全过程日志记录到 `/opt/sub2api/deploy.log`

## [2026-04-16] feat(branding): 新增强调安全与稳定气质的两版粗犷图标

**影响范围**:
- `frontend/public/logo-gateway-fortress.svg`
- `frontend/public/logo-gateway-vault.svg`

**上游兼容性**: 无冲突，仅新增静态品牌资源

**变更详情**:
- 新增 `logo-gateway-fortress.svg`，方向偏“护盾 + 基础设施堡垒”，用厚重对称结构强化安全、稳固、可信赖的第一印象
- 新增 `logo-gateway-vault.svg`，方向偏“金库门 + 稳定中枢”，通过更粗的门框和锁芯语义突出可靠托管与资产安全感
- 两版都比前面的方案更大胆、更厚重，优先服务“安全、稳定、靠谱”的品牌心智

## [2026-04-16] feat(branding): 新增两版原创图标备选方案

**影响范围**:
- `frontend/public/logo-gateway-orbit.svg`
- `frontend/public/logo-gateway-portal.svg`

**上游兼容性**: 无冲突，仅新增静态品牌资源

**变更详情**:
- 新增 `logo-gateway-orbit.svg`，方向偏“网络中枢 / 控制面 / 调度节点”，核心是环形汇聚与三路接入
- 新增 `logo-gateway-portal.svg`，方向偏“入口 / 通道 / 网关门户”，核心是分层门框与向心聚合
- 两版都刻意避开上游 `sub2api` 常见的字母化几何造型，优先建立你自己的品牌识别

## [2026-04-16] feat(branding): 图标重构为原创网关中枢造型，避开上游视觉关联

**影响范围**:
- `frontend/public/logo-gateway-mark.svg`

**上游兼容性**: 无冲突，仅更新自定义品牌资源

**变更详情**:
- 将上一版偏几何字母的图标重构为“六边形网关核心 + 三路汇聚节点”的原创符号，避免让人联想到上游 `sub2api` 默认视觉
- 保留当前站点自己的深蓝底和青绿主色，以保证和现有首页、后台按钮、卡片高亮仍然统一
- 新图标更强调“聚合、调度、分发”的产品语义，而不是字母造型，便于后续独立品牌化

## [2026-04-16] feat(branding): 新增贴合 AI 网关语义的 SVG 图标方案

**影响范围**:
- `frontend/public/logo-gateway-mark.svg`

**上游兼容性**: 无冲突，仅新增静态品牌资源，不替换上游默认文件

**变更详情**:
- 新增一版用于 Sub2API 的品牌图标方案，延续现有深蓝底与青绿到蓝色渐变的视觉语言，避免与首页和后台的主色体系割裂
- 图标语义从单纯几何字母进一步收敛到“网关 / 路由 / 聚合分发”，通过中枢式几何主形和节点端点强化 API Gateway 产品识别度
- 资源使用 SVG 矢量格式，便于后续在后台 `site_logo`、站点首页、favicon 导出和营销物料中复用

## [2026-04-16] fix: AI Credits 被临时限流误标为积分耗尽导致账号锁定 5 小时

**影响范围**:
- `backend/internal/service/antigravity_credits_overages.go`
- `backend/internal/service/antigravity_credits_overages_test.go`

**上游兼容性**: 无冲突（二开新增功能）

**变更详情**:
- `shouldMarkCreditsExhausted` 中 `"resource has been exhausted"` 关键词匹配了 Google API 所有 429 响应（包括临时 RPM 限流），导致 credits 被错误标记为耗尽。一旦误标形成自锁（`isCreditsExhausted` 阻止重试 → `clearCreditsExhausted` 永不触发），账号被锁定完整 5 小时。
- 移除过于宽泛的 `"resource has been exhausted"` 关键词，其余关键词（`insufficient credit`、`credit exhausted` 等）已足够精确
- `shouldMarkCreditsExhausted` 排除 429 状态码，临时限流不应判定为积分耗尽

---

## [2026-04-16] feat(admin): 模型定价页合并映射 CRUD + 模型测试，删除旧 mapping tab

**影响范围**:
- `frontend/src/views/admin/ModelConfigView.vue`（**大幅精简**：删除 mapping tab 全部模板和 script，只保留 pricing 和 rate 两个 tab）
- `frontend/src/components/admin/model-pricing/ModelMappingInlinePopover.vue`（**新建**）
- `frontend/src/components/admin/model-pricing/ModelTestDialog.vue`（**新建**）
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`（表格顶部加"+ 添加映射"按钮；行操作列加"编辑映射"和"测试"两个条件显示按钮；接入两个新组件）
- `frontend/src/i18n/locales/zh.ts` & `en.ts`（新增 ~20 条 key：映射 CRUD + 模型测试）

**上游兼容性**: 低风险。全部集中在二开独有的模型配置界面。API 复用现有的 `adminAPI.accounts.getAntigravityDefaultModelMapping` / `updateAntigravityDefaultModelMapping`（上游已有），以及 SSE 测试接口 `POST /admin/accounts/:id/test`。

**背景**:

上一轮把模型定价页重构为"双列模型名 + 计费模式"风格后，用户反馈："映射关系和计费模式不能修改"。经讨论：
- 计费模式保留只读（本身是从映射关系推断的标签，不是可配置属性）
- 映射关系**应该**能改，且决定把「模型映射」独立 tab 合并到定价页（后续渐进删除独立 tab）
- 模型测试功能搬到定价页行操作里做成小按钮

方向确定后本轮实施彻底的合并。

**变更详情**:

1. **新建 `ModelMappingInlinePopover.vue`**（~210 行）：
   - 三种操作：新增映射（mode="add"）/ 修改映射（mode="edit"）/ 删除映射（edit 模式底部按钮）
   - 两个 input：请求模型名 + 上游模型名，下方带一行灰字提示"同名映射直接填相同值"
   - 走现有 API：`GET /admin/accounts/antigravity/default-model-mapping` 读全表 → 局部修改 → `PUT` 整表写回
   - 改名场景（edit 时把 from 也改了）正确处理：先 delete 旧 key 再 set 新 key/value
   - Teleport + fixed 定位（参考 ModelPricingInlinePopover 设计），自动避开视口边界
   - Enter 保存、红字 inline 错误反馈

2. **新建 `ModelTestDialog.vue`**（~160 行）：
   - 从原 `ModelConfigView.vue` 的 mapping tab 右侧测试面板搬迁，逻辑基本保留
   - 固定传入 `model` prop（从行按钮触发时锁定），不再需要模型下拉
   - 内部加载 Antigravity 账号列表（仅 active / schedulable / 无 error 的）
   - SSE 流式消费 `/api/v1/admin/accounts/:id/test`，解析 `test_start / content / test_complete / error` 事件类型
   - `testRunning` 时阻止关闭 dialog 避免用户误操作

3. **`ModelPricingTab.vue` 接入**：
   - 表格顶部（搜索行右侧、刷新按钮左侧）新增"+ 添加映射"按钮，锚点 ref 用于 popover 定位
   - 行操作列三按钮（条件显示）：
     - ⇄ **编辑映射**：仅 `canEditMapping` 行（hint type=requested_only 或 requested_equals_upstream）
     - ▶ **测试模型**：`canTest` 行（有 billing_basis_hint 或 provider=antigravity）
     - ✎ 查看详情 / 创建定价：所有行（保持原行为）
   - `handleMappingSaved` 事件回调调用 `loadData` 整表刷新（映射变化影响所有徽标和 related_models）
   - `RowDisplay` 接口扩 `canEditMapping` / `canTest` 字段，在 `displayRows` computed 里按 hint 类型推导

4. **删除旧 mapping tab**：
   - `ModelConfigView.vue` 从 350 行精简到 40 行，只保留 pricing 和 rate 两个 tab + 必要的 AppLayout 壳
   - 历史 URL 兼容：`?tab=mapping` 被自动回退到 pricing
   - 旧 i18n key（`admin.modelConfig.antigravityMapping` / `testTitle` 等）暂未清理，留着不用不影响行为，后续可随上游同步一起清除

**验证**:
- `pnpm run typecheck` 通过
- 前端 dev server 热重载后手测流程：
  - 点"+ 添加映射" → 填 from/to → 保存 → 表格 reload 新映射出现
  - 点某行"编辑映射" → 改上游名 → 保存 → 列表更新；徽标和 +N 计数正确联动
  - 编辑 popover 底部点"删除映射" → 确认 → 该映射从表中消失
  - 点某行"测试" → dialog 弹出 → 选账号 → 发送 → 流式输出正确显示
  - 旧 mapping tab 彻底消失，只剩 Pricing 和 Rate Multipliers 两个 tab

**已知限制 / 未来迭代**:
- `upstream_only` 类型的行（仅作为映射 value 存在、无同名自映射）不提供"编辑映射"按钮；当前 Antigravity 默认映射里此类型为空（所有 value 都有同名自映射），实际无影响
- 账号级 `credentials.model_mapping` 的管理仍走原账号编辑界面，本次没有合并（用户明确只要求平台级映射管理合入）
- 旧 `admin.modelConfig.*` 下的 mapping 相关 i18n key 暂留未清理

## [2026-04-15] feat(admin): 模型定价页深度优化（下划线 tab / 内联 popover / 建议价 / billing hint）

**影响范围**:
- `backend/internal/service/global_model_pricing_service.go`（ModelPricingListItem/Detail 加字段、suggestPricing、isAntigravityStubModel、Antigravity 反扫 mapping value）
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`（下划线 tab 筛选器、computePriceDelta 涨跌染色、折叠 banner、inline popover 接入、行级徽标）
- `frontend/src/components/admin/model-pricing/ModelPricingDetailDialog.vue`（建议价展示 + 应用按钮）
- `frontend/src/components/admin/model-pricing/ModelPricingInlinePopover.vue`（新建，308 行）
- `frontend/src/api/admin/modelPricing.ts`（类型扩充：suggested_prices/suggested_from/billing_basis_hint）
- `frontend/src/i18n/locales/zh.ts` & `en.ts`（~20 条新 key）

**上游兼容性**: 中等。所有改动集中在二开独有的「模型定价」管理界面（2026-04-12 新增的 ModelPricingTab 和相关服务方法上游不存在），与上游主线无冲突。GlobalModelPricing 实体没有新增 DB 字段，零 migration。需要留意的是上游未来若给 `ModelPricingListItem` / `ModelPricingDetail` 增加字段时要避免和本次新增字段命名冲突。

**背景**:

此前「模型配置 → 模型定价」Tab 已能正确展示 Gemini/Antigravity 筛选结果，但管理员真正使用该页面管理全局定价时还有四个痛点：
1. 表格里每个价格字段到底来自 LiteLLM 还是被 global/channel 覆盖看不清，只有 input/output 列有简单颜色，cache 列完全没标
2. 来源筛选 Tab 顺序是「全部 / 全局覆盖 / 渠道覆盖 / 仅 LiteLLM」，但实际计费优先级是 `Channel > Global > LiteLLM`，顺序反了且页面没有任何位置说明这个优先级
3. 改一个模型的 input 价要点铅笔图标弹全屏 dialog → 翻找 → 改 → 保存 → 关闭，对高频调参场景太重
4. 上一轮补的 Antigravity 专有 stub（`gemini-3-pro-high`、`gpt-oss-120b-medium`、`tab_flash_lite_preview` 等 8+ 个）一排 `-`，管理员无从下手；且这些模型涉及账号级映射，与渠道定价的 `billing_model_source` 机制强相关

**设计决策**：

经过 Explore+Plan 子代分析，关键发现：`model_pricing_resolver.go` 的 `resolveBasePricing(model)` 收到的 `model` 已经是被 `BillingModelSource` 过滤的 `billingModel`，全局覆盖的查表 key **天然跟随每个请求所属渠道的 billing_model_source**。也就是说系统已实质一致，缺的只是**让管理员看到这个隐式行为**。因此本轮选**方案 A**（前端明示隐式行为），不加后端字段，零 migration。

**变更详情**:

1. **筛选顺序 + 层级说明**：sourceTabs 顺序改为 `全部 / 有渠道覆盖 / 有全局覆盖 / 仅 LiteLLM`；Source label 右侧加 ⓘ 图标，hover 显示"优先级：渠道 > 全局 > LiteLLM"tooltip。
2. **差异高亮**：`formatPrice` 重构为 `computePriceDelta`，返回 `{text, className, tooltip}`。以 LiteLLM 为基准计算相对百分比差异，±1% 内视作等同。涨价 `text-rose-600`、跌价 `text-emerald-600`、等同或无基准 `text-primary-600`、纯 LiteLLM 默认灰。cache_write/cache_read 一并启用。每个数字上 `title` 显示"LiteLLM 基准 $X · 差异 +Y%"。
3. **折叠 banner（计费基准说明）**：stats 卡下方加 `<details>` 折叠块，默认收起。展开解释 requested/upstream/channel_mapped 三种基准含义 + "渠道默认 channel_mapped，无渠道路径默认 requested"。
4. **内联 popover 编辑**：
   - 新建 `ModelPricingInlinePopover.vue`：Teleport 到 body 避免表格 overflow 裁切；fixed 定位自动避开视口边界（下方 → 上方、右侧 → 左对齐）；4 个核心价格字段 + enabled 复选框 + 保存/删除/详细设置 3 按钮；每个字段带 LiteLLM 基准 placeholder；Enter 提交
   - 表格 4 个价格 `<td>` 加 `@click` 触发 popover + `cursor-pointer hover:bg-primary-50/50`
   - 保存时**不整表 reload**，父组件 `handleInlineSaved` 就地替换 items 并差量更新 stats.global_override_count
   - Popover 保留原 override 的 provider/notes/image_output_price/per_request_price 等字段（PATCH 差量），避免清零
   - `< lg` 断点 `window.matchMedia('(max-width: 1023px)')` 回退到原 dialog；stub 模型（需要配 provider/notes/建议价）也回退到 dialog
   - 筛选器下方加灰色小字提示"点击表格中的价格数字可快速编辑"
5. **Antigravity stub 可配置 + 建议价**：
   - 表格铅笔图标对 stub 行 tooltip 切换为"创建定价"
   - 后端 `ModelPricingDetail` 加 `SuggestedPrices` / `SuggestedFrom` 字段，仅在无 LiteLLM + 无 global_override 时填充
   - 新 `suggestPricing` 方法按以下链匹配：显式映射表（`tab_flash_lite_preview → gemini-2.5-flash-lite`、`gpt-oss-120b-medium → gpt-4o-mini`）→ 剥离 `-high/-low/-medium` 档位后缀 → 剥离 `-thinking` → Gemini 版本降级（3.x → 2.5）
   - `ModelPricingDetailDialog.vue` 在 Global Override section 顶部展示"💡 建议价（来自 xxx）· 应用"行，点击应用把值填入 form（需管理员确认保存，不自动入库）
   - 修复一个副作用 bug：`pricingService.GetModelPricing` 带模糊匹配，对 Antigravity 专有 stub 会错误匹配到不相关的 LiteLLM 模型价格。新增 `isAntigravityStubModel` 检测（model 在 Antigravity mapping keys 但不在 LiteLLM 精确模型列表），详情接口对 stub 跳过 LiteLLM 并走 suggestPricing，与列表接口的精确匹配语义一致
6. **双列模型名 + 计费模式列**（迭代过 badge 方案后的最终形态）：
   用户反馈小 badge 太抽象，于是把信息提升为正式表格列——直接体现"客户端请求名 / 上游名 / 计费模式"三元组心智模型。
   - 后端 `ModelPricingListItem.BillingBasisHint` 从单字符串升级为结构体 `{ type, related_models }`
     三种 type：
     - `requested_equals_upstream`——同名映射或纯 LiteLLM 模型，请求名 = 上游名
     - `upstream_only`——模型是映射 value，客户端不直接请求它；related_models 列出所有映射源请求名（支持多对一）
     - `requested_only`——模型是映射 key，被映射到其他名字；related_models 单元素为上游目标
     优先级 `same_name > upstream_only > requested_only`；sameName 情况也填 related_models 承载"被谁映射到我"信息，避免信息丢失
   - 前端 `ModelPricingTab.vue` 把原 Model 单列拆成「请求模型名 / 上游模型名」双列，并新增「计费模式」列（只读标签：按请求 / 按上游 / 请求=上游）
     每行根据 hint 推导两列展示值：
     - `requested_equals_upstream`：两列相同 = model 自身，若 related_models 非空展示 `+N` 小徽标 + hover 列全
     - `requested_only`：请求 = model，上游 = related_models[0]
     - `upstream_only`：请求 = related_models[0]（+N 表示多对一），上游 = model
   - Provider / Channels 列改为 `xl:table-cell`（< 1280px 隐藏），节省宽度
   - 计费模式列**不可编辑**，因为它不是这条记录的属性——它是从映射关系自动推断的展示标签，实际计费基准由请求所属渠道的 `billing_model_source` 决定
   - banner 的展开内容里补一条 `billingBasisColumnNote` 警告式说明，明确告知用户"这一列只读 + 实际由渠道决定"

**验证**:
- `pnpm run typecheck` 通过
- `go build ./...` 通过，`go vet ./internal/service/` 无告警
- 本地 API 实测：
  - `provider=antigravity` 返回 30 条，各 type 分布符合预期：
    - `requested_equals_upstream`：`claude-opus-4-6-thinking`（related_models=[opus-4-5-20251101, opus-4-5-thinking, opus-4-6] 表示被 3 个请求映射到）、`claude-sonnet-4-6`（被 haiku-4-5 / haiku-4-5-20251001 映射到）、`gemini-3.1-flash-image`（被 3 个 image 模型映射到）等
    - `requested_only`：`claude-haiku-4-5 → claude-sonnet-4-6`、`claude-opus-4-6 → claude-opus-4-6-thinking`、`gemini-3-pro-preview → gemini-3-pro-high` 等
    - `upstream_only`：Antigravity 默认映射的 value 基本都有同名自映射，所以本类别暂时没数据——这是符合数据集现状的预期
  - `GET /admin/model-pricing/gemini-3-pro-high` → 建议价来自 `gemini-2.5-pro`
  - `GET /admin/model-pricing/tab_flash_lite_preview` → 建议价来自 `gemini-2.5-flash-lite`
  - `GET /admin/model-pricing/gpt-oss-120b-medium` → 建议价来自 `gpt-4o-mini`（之前被 LiteLLM 模糊匹配污染成 `1.25e-6 / 1e-5` 错价，已修复）
  - `GET /admin/model-pricing/claude-opus-4-6-thinking` → 正常返回 LiteLLM 价格，不触发 suggestPricing

**已知限制**:
- 显式建议价映射表 `antigravityProprietarySuggestMap` 需要在 Google/OpenAI 发新模型时维护，目前只对 `tab_flash_lite_preview` / `gpt-oss-120b-medium` 两条
- Popover 仅支持 4 个核心价格字段；provider/notes/image_output_price/per_request_price/billing_mode 仍需走原 dialog（通过 popover 的"详细设置…"按钮跳转）
- 方案 A 的保守选择：未来若出现"同一模型在不同 billing_model_source 下需要不同价"的实际业务场景，需要升级到方案 B（给 GlobalModelPricing 加 billing_model_source 字段 + 二维缓存），本次不阻塞该扩展

## [2026-04-15] fix(admin): 模型定价页 Gemini/Antigravity 过滤失效

**影响范围**:
- `backend/internal/service/global_model_pricing_service.go`（filterItems 别名匹配 + Antigravity 模型补全）
- `frontend/src/components/admin/model-pricing/ModelPricingTab.vue`（Gemini 下拉 value 对齐）

**上游兼容性**: 低风险。`filterItems`/`ListAllModels` 是二开 2026-04-12 新增的统一定价管理界面（见下文），上游没有同名函数；唯一可能冲突点是 `domain.ResolveAntigravityDefaultMapping` 的引入。

**背景**:
管理后台「模型配置 → 模型定价」Tab 里，provider 下拉选 Gemini 或 Antigravity 时列表为空。根因：

1. **Gemini**：前端下拉 value 是 `vertex_ai`，但 LiteLLM JSON 里 Gemini 家族的 `litellm_provider` 字段实际值是 `gemini`（Google AI Studio）或带后缀的 `vertex_ai-language-models` / `vertex_ai-vision-models` / `vertex_ai-embedding-models`（Vertex AI），`filterItems` 的 `strings.ToLower(item.Provider) != providerLower` 严格相等匹配一个都命不中。
2. **Antigravity**：Antigravity 是二开自研平台，LiteLLM 里不存在任何 `antigravity` provider 条目；同时 `DefaultAntigravityModelMapping` 里定义的 Antigravity 可用模型（如 `gemini-3-pro-high`、`tab_flash_lite_preview`）根本不在列表枚举来源（LiteLLM + 全局覆盖）里。

**变更详情**:
- 抽出 `providerMatches(item, providerLower, antigravityModelSet)` 把严格相等改为别名感知：
  - `gemini` → 匹配 `gemini` 或 `vertex_ai` 前缀
  - `openai` → 匹配 `openai` 或 `text-completion-openai`
  - `antigravity` → 匹配 `provider=antigravity` 或模型名命中 `domain.ResolveAntigravityDefaultMapping()` 的 key
  - 其它（anthropic/bedrock 等）→ 保留原严格相等
- `ListAllModels` 合并阶段新增一轮遍历 `ResolveAntigravityDefaultMapping()`，对 LiteLLM 和全局覆盖都没有的模型名补一条 provider=antigravity 的 stub ListItem，保证 Antigravity 专有模型在列表里可见可管。
- 前端 `ModelPricingTab.vue` 的下拉把 `<option value="vertex_ai">Gemini</option>` 改为 `value="gemini"`，与后端新别名对齐。
- `modelSet` 合并循环新增的写入确保 Antigravity stub 去重时 dedup 基准完整（之前 all-overrides 循环漏写 modelSet，偶发重复；一起修掉）。

**验证**:
- `go build ./internal/service/ ./internal/handler/admin/` 通过
- `go vet ./internal/service/` 无告警
- `pnpm run typecheck` 无错误

## [2026-04-15] feat(tools): 新增图片生成 API 压力测试脚本

**影响范围**:
- `tools/image_stress_test.py`（新增，单文件 Python 异步压测脚本，~580 行）

**上游兼容性**: 纯新增客户端工具，不触碰 backend/frontend/deploy，无上游冲突风险。

**背景**:
客户反馈通过 API 调用 Gemini 图片生成模型（`gemini-3-pro-image` / `gemini-2.5-flash-image` 等）时错误率很高，需要一个可复现、可诊断的工具去定位问题到底出在上游账号池、调度器、还是 Anthropic 兼容翻译层。

**变更详情**:
- 用 `httpx[http2]` + `asyncio` 实现受控并发压测
- 支持两条入口路径的对比：
  1. `gemini-native`：`POST /v1beta/models/{model}:generateContent`
  2. `anthropic-messages`：`POST /v1/messages`（走 `GeminiMessagesCompatService` 翻译层）
- 也支持 `--stream` 走 `:streamGenerateContent`，命中代码里 `handleGeminiStreamToNonStreaming` 的流式分支
- 错误分类对齐服务端的失败信号：`empty_stream` / `safety_block` / `google_config_error` / `signature_error` / `overloaded_529` / `rate_limit_429` / `gateway_5xx` / `auth_401_403` / `client_4xx` / `timeout` / `network_error`
- 特别识别 "200 OK 但无图"（`candidates[0].content.parts` 里无 `inlineData`，或 `finishReason` 属于 safety 类）—— 这是客户最容易把它当 bug 报的 case
- 每个请求记录 `X-Request-ID`，`summary.md` 会列出 top 失败 request_id 便于 SSH 到服务器关联日志
- 输出结构：`output/stress-<timestamp>/{run.json, requests.jsonl, summary.md}`，`output/` 已在 `.gitignore`
- 默认目标 `https://zerocode.kaynlab.com`，API key 从 `$SUB2API_KEY` 读取
- Windows 友好：自动把 stdout/stderr 重配置为 UTF-8 避免 cp936 乱码

**使用**:
```bash
export SUB2API_KEY=sk-xxx
python tools/image_stress_test.py --total 50 --concurrency 5 --mode gemini-native
```

完整执行流程（冒烟 → 基线 → 并发扫 → 模式对比 → 模型对比 → 流式）见 `tools/image_stress_test.py` 模块注释顶部。

---

## [2026-04-14] chore(deploy): remote_exec.py 增加 --update 快捷方式避开 MSYS2 路径转换

**影响范围**:
- `deploy/remote_exec.py`（**未 tracked，本地改动**，.gitignore 中；因含明文 SSH 凭证不入库）
- `CLAUDE.md`（workflow + 生产服务器章节）
- `docs/dev/UPSTREAM_SYNC.md`（部署指令范例）

**上游兼容性**: 仅影响本地工作流，不涉及任何上游文件。

**背景**:
2026-04-14 v0.1.112 合并完成准备部署时，在 Git Bash 下执行
`python deploy/remote_exec.py "/opt/sub2api/update.sh"` 报
`bash: line 1: D:/program: No such file or directory` 失败。
定位后确认是 MSYS2 argv path conversion：Git Bash 会把任何看起来像
POSIX 绝对路径的 argv 参数（`/opt/...`）悄悄转成 Windows 路径后才交给
Python，于是 argv[1] 变成了 `D:\program files\...\opt\sub2api\update.sh`，
SSH 远端收到一个不存在的路径自然失败。

**变更详情**:
- `deploy/remote_exec.py`
  - 新增 `SHORTCUTS` 字典 + `--update` 快捷方式，内部用 Python 字符串字面量
    `"bash /opt/sub2api/update.sh"`，完全绕过 MSYS2 argv 转换
  - 新增 `--env` 模式从 `REMOTE_CMD` 环境变量读命令（但仍需配合
    `MSYS_NO_PATHCONV=1` 才能让 Git Bash 不转 env 里的路径；作为 escape hatch）
  - 新增结构化 docstring 说明 MSYS2 陷阱和四种 workaround 优先级
  - `run()` 默认 timeout 从 300s 提升到 600s，适配 Docker build 场景
  - 输出 decode 加 `errors="replace"`，避免二进制污染时 UnicodeDecodeError

- `CLAUDE.md` workflow 步骤 4/5 与「生产服务器」章节
  - 部署命令改为 `python deploy/remote_exec.py --update`
  - 追加 MSYS2 gotcha 警告和指向 remote_exec.py docstring 的引用
  - 生产服务器 SSH 字段说明 ad-hoc 命令仅限不以 `/` 开头的命令

- `docs/dev/UPSTREAM_SYNC.md`
  - 本次部署条目追加已部署标记
  - 部署指令范例改用 `--update` 并注明旧用法被弃用的原因

**部署验证**:
- `python deploy/remote_exec.py --update` 端到端跑通：pull（已 up-to-date）→
  docker build → docker compose up → health check `{"status":"ok"}` → ps 显示
  sub2api 容器 `Up 8 seconds (healthy)`。

**关联**: 无 issue。修复源于 2026-04-14 v0.1.112 同步部署过程中发现。

---

## [2026-04-14] fix(billing): 修复全局模型定价覆盖在 Anthropic 网关失效及多处计费漏洞

**影响范围**:
- backend/internal/service/model_pricing_resolver.go（核心解析器重写）
- backend/internal/service/global_model_pricing.go（删除有 bug 的 ToModelPricing）
- backend/internal/service/global_model_pricing_cache.go（新增）
- backend/internal/service/global_model_pricing_service.go（注入缓存并在 CUD 时失效）
- backend/internal/service/gateway_service.go（resolveChannelPricing 同时接受 Global 来源）
- backend/internal/service/wire.go（Provider set 追加 NewGlobalPricingCache）
- backend/cmd/server/wire_gen.go（手动同步 DI 接线）
- backend/internal/handler/admin/model_pricing_handler.go（UpdateOverride 差量更新）
- backend/internal/service/model_pricing_resolver_test.go（新增 5 个回归测试）

**上游兼容性**: 高度可能产生冲突 —— 触及上游 resolver 与 gateway_service 的核心
计费路径，以及 wire_gen.go。合并上游时如果官方重构了 ModelPricingResolver 或
GatewayService.calculateTokenCost 需要重新整合本修复。

**背景**:
审计管理后台"模型配置 → Pricing"页面的「全局覆盖」功能是否端到端生效，
发现它在多条路径上被静默绕过或丢失字段，详见本次 commit 说明。

**变更详情**（按 bug 对应修复）:

- **Bug A — Anthropic 网关热路径绕过全局覆盖**
  `gateway_service.go:resolveChannelPricing` 原本只在 `Source==Channel` 时返回
  resolved，导致「只配了全局覆盖、没配渠道」的情形会回落到 `CalculateCost` 旧
  路径。旧路径完全不查 GlobalPricingRepository，全局覆盖 → 静默失效。修复：
  放宽条件为 `Source==Channel || Source==Global`，同时保留函数名以减少 diff。

- **Bug B — ResolvedPricing.Mode 忽略全局覆盖的 BillingMode**
  原 `Resolve` 把 `Mode` 硬编码为 `BillingModeToken`，只在渠道叠加分支里改。
  后果：管理员在全局覆盖里选 `per_request` / `image` → 后端仍按 token 计费 →
  单价全为 0 → 用户免费。修复：`resolveBasePricing` 返回 `(pricing, mode,
  defaultPerRequestPrice, source)` 四元组，`Resolve` 原样塞进 `ResolvedPricing`。

- **Bug C — ToModelPricing 丢失 Priority/长上下文/缓存分级字段**
  原 `GlobalModelPricing.ToModelPricing()` 只设 5 个字段，导致 Priority tier 单价
  归零、GPT-5.4 长上下文双倍费丢失、缓存 5m/1h 分级失效等。修复：
  1. 删除该方法
  2. `resolveBasePricing` 先从 `BillingService.GetModelPricing` 拿完整基础定价
     （含 LiteLLM 的所有字段），再用 `applyGlobalPricingOverride` 把全局覆盖的
     非 nil 字段叠加上去；语义与 `applyTokenOverrides`（渠道覆盖）完全对齐，
     包括 Priority 字段与覆盖价同步、`CacheWritePrice` 同时写入 5m/1h。
  3. 未被覆盖的字段（Priority 单价差、长上下文倍率等）继承自 LiteLLM 基础。

- **Bug D — 每个请求一次 SQL 无缓存**
  原实现在热路径对 `global_model_pricing` 表每请求一次 `SELECT`。修复：新增
  `GlobalPricingCache`（sync.RWMutex + 惰性加载），首次访问时一次性读入所有
  `enabled=true` 条目到内存 map，后续 O(1) 查询；管理后台在 Create/Update/
  Delete 后调用 `Invalidate()` 清空缓存。

- **Bug E — resolveBasePricing 使用 context.Background**
  原实现丢弃调用者 ctx 导致请求超时无法传递。修复：缓存化之后热路径不再进 DB，
  ctx 问题自然消失；仅在缓存首次加载时用 background ctx 执行一次性全量查询。

- **Bug F — UpdateOverride 把所有未提供字段清零**
  原 handler 对 `InputPrice` 等指针字段无条件赋值，PATCH 漏带任何一个字段都会
  把已有价格覆盖成 nil。修复：统一改为"非 nil 才覆盖"的差量更新（与
  `Model` / `Provider` / `Enabled` 字段的处理对齐）。要清除某个价格请
  delete 覆盖后重建。

**回归测试**（`model_pricing_resolver_test.go` 新增）:
1. `TestResolve_GlobalOverride_PreservesPriorityAndLongContext` — 覆盖 input/output
   后验证 Priority 同步、长上下文阈值/倍率/缓存 5m/1h 从 LiteLLM 继承
2. `TestResolve_GlobalOverride_CacheWriteSyncsAllCacheFields` — 覆盖 CacheWritePrice
   后 Creation/5m/1h 三字段全部同步
3. `TestResolve_GlobalOverride_DisabledIsIgnored` — enabled=false 不生效
4. `TestResolve_GlobalOverride_BillingModeRespected` — per_request 模式正确传递
   BillingMode 和 DefaultPerRequestPrice
5. `TestResolve_ChannelOverride_BeatsGlobalOverride` — 优先级 Channel > Global

所有新测试通过；既有 `./internal/service/...` 单元测试套件全绿（76 秒）；
`go build ./...` 通过。

**关联 Issue/PR**: 无（本地审计发现）

---

## [2026-04-14] feat(frontend): 代理批量导入支持 host:port:user:pass 等简写格式

**影响范围**:
- frontend/src/views/admin/ProxiesView.vue
- frontend/src/i18n/locales/{zh,en}.ts

**上游兼容性**: 纯前端改动，仅扩展解析逻辑和 UI 文案；未触碰后端 API。合并上游若改 `parseProxyUrl` 或 `batchInputPlaceholder/Hint` 可能产生冲突。

**变更详情**:
- `parseProxyUrl` 从单一 URL 正则扩展为四段 fallback 解析：
  - A. `protocol://[user:pass@]host:port`（原有，协议来自行内，优先级最高）
  - B. `user:pass@host:port`（新，无协议前缀）
  - C. `host:port:user:pass`（新，ProxyScrape / 911 类供应商常见格式；密码保留行尾所有非空白字符）
  - D. `host:port`（新，无认证）
  - 提取出 `buildResult` 辅助函数统一做端口/主机校验。
- 在"快捷添加"Tab 顶部新增"默认协议"下拉（`batchDefaultProtocol`，默认 `http`），简写格式 B/C/D 的行会套用这个协议；切换时通过 `@update:modelValue` 触发 `parseBatchInput` 重算，无需用户重新编辑文本。
- 关闭弹窗时在 `closeCreateModal` 里重置 `batchDefaultProtocol`。
- i18n：扩充 `batchInputPlaceholder`、`batchInputHint` 示例；新增 `batchDefaultProtocol`、`batchDefaultProtocolHint` 两条 key（中英双语对齐）。
- 后端 `BatchCreate` 接口不变（仍接收 `{protocol,host,port,username,password}`），无需迁移。

**关联 Issue/PR**: 无

## [2026-04-13] feat: Gemini Google One 批量 Refresh Token 导入

**影响范围**:
- backend/internal/pkg/geminicli/{constants.go, token_types.go}
- backend/internal/service/{gemini_oauth.go, gemini_oauth_service.go, gemini_oauth_service_test.go}
- backend/internal/repository/gemini_oauth_client.go
- backend/internal/handler/admin/gemini_oauth_handler.go
- backend/internal/server/routes/admin.go
- frontend/src/api/admin/gemini.ts
- frontend/src/composables/useGeminiOAuth.ts
- frontend/src/components/account/CreateAccountModal.vue
- frontend/src/i18n/locales/{zh,en}.ts

**上游兼容性**: 中风险 — GeminiOAuthClient 接口新增 GetUserInfo；CreateAccountModal 多处条件合并，合并上游时可能冲突

**变更详情**:
- 后端：
  - `geminicli` 新增 `UserInfoURL` 常量 + `UserInfo` 类型（复用 Google userinfo 端点）
  - `GeminiOAuthClient` 接口新增 `GetUserInfo(ctx, accessToken, proxyURL)`；`geminiOAuthClient` 实现 + 测试 mock 同步更新
  - `GeminiTokenInfo` 加 `Email` 字段；`BuildAccountCredentials` 在 email 非空时写入 `credentials.email`（与 Antigravity 对齐，复用账号列表搜索 `credentials->email` 索引）
  - 新增 `ValidateGoogleOneRefreshToken` 服务方法：refresh → 回填 RT → `GetUserInfo` 拿 email（失败打 warning 不阻断）→ `fetchProjectID`（必需）→ `FetchGoogleOneTier`（失败回落 free）
  - 新增 `POST /admin/gemini/oauth/refresh-token` handler + 路由注册
- 前端：
  - `useGeminiOAuth` 加 `validateGoogleOneRefreshToken` 方法，`buildCredentials` 透传 email
  - `CreateAccountModal`：`isEmailAsNameAvailable` 计算属性统一 Antigravity / Gemini+google_one 的"用邮箱作为账号名"开关；`handleValidateRefreshToken` 加 gemini 分支；新增 `handleGeminiGoogleOneValidateRT`（循环 RT → 单个创建）
  - OAuthAuthorizationFlow 的 `show-refresh-token-option` 扩展覆盖 `gemini + google_one`
  - zh/en i18n 补齐 `admin.accounts.oauth.gemini` 的 RT 批量导入文案
- 限制：仅支持 `google_one`；RT 必须由内置 Gemini CLI OAuth client 签发（自建 client 的 RT 会报 `unauthorized_client`，错误提示已包含相应说明）

## [2026-04-12] feat: 统一模型定价管理界面

**影响范围**: backend(migrations, service, repository, handler, routes, wire), frontend(views, components, api, i18n)
**上游兼容性**: 低风险，新增功能，不修改现有计费逻辑
**变更详情**:
- 新增 `global_model_pricing` 数据库表，支持管理员设置全局模型定价覆盖
- 定价解析链扩展为：Channel → Global → LiteLLM → Fallback（向下兼容，表为空时行为不变）
- 后端新增 GlobalModelPricingRepository、GlobalModelPricingService、ModelPricingHandler
- 新增 API 端点 GET/POST/PUT/DELETE /admin/model-pricing，含费率乘数概览
- PricingService 新增 GetAllModels() 方法供管理后台展示所有 LiteLLM 模型
- 前端模型配置页改为 Tab 布局：模型定价（新增）| 模型映射（现有）| 费率概览（新增）
- 模型定价 Tab：全模型列表 + 搜索/筛选 + 全局覆盖编辑弹窗 + 渠道覆盖展示
- 费率概览 Tab：只读展示各分组费率乘数，链接到分组管理页
- 中英文 i18n 翻译完整

## [2026-04-12] feat: 模型配置页面添加模型测试功能

**影响范围**: frontend/src/views/admin/ModelConfigView.vue, i18n
**上游兼容性**: 低风险，仅前端改动
**变更详情**:
- ModelConfigView 改为左右布局：左侧映射配置，右侧模型测试
- 测试区域：账号选择（自动选第一个可用，可手动切换）、模型下拉、提示词输入
- 复用 POST /admin/accounts/:id/test API，SSE 流式展示上游响应
- 终端风格输出区域，色彩区分（cyan=信息, green=内容, red=错误, emerald=成功）

## [2026-04-12] feat: 独立"模型配置"管理页面 — Antigravity 全局默认映射

**影响范围**: 前后端多文件
**上游兼容性**: 中风险，新增文件为主，但修改了 account.go 的默认映射回退逻辑和 wire_gen.go
**变更详情**:
- 后端: 新增 setting key `antigravity_default_model_mapping`，存储在 settings 表
- 后端: SettingService 新增 Get/Set 方法
- 后端: AccountHandler 新增 PUT API，修改 GET API 优先读 settings
- 后端: domain.constants.go 新增 `GetAntigravityDefaultMappingOverride` 函数变量
- 后端: account.go 中 `resolveModelMapping` 改为调用 `domain.ResolveAntigravityDefaultMapping()`
- 后端: wire_gen.go 注入 override 函数 + settingService 传入 AccountHandler
- 前端: 新建 ModelConfigView.vue（独立页面，管理员可见）
- 前端: 新增路由 `/admin/model-config`、侧边栏菜单项
- 前端: accounts API 新增 `updateAntigravityDefaultModelMapping`
- 前端: zh.ts/en.ts 新增 modelConfig i18n 文本
- 优先级: 单账号自定义映射 > 全局映射（settings）> 内置默认（constants.go）

## [2026-04-12] fix: Antigravity 批量创建账号 allow_overages 未生效

**影响范围**: frontend/src/components/account/CreateAccountModal.vue
**上游兼容性**: 低风险，单行修改
**变更详情**:
- 批量创建时 `extra` 硬编码为 `{}`，改为调用 `buildAntigravityExtra()`，正确传递 `allow_overages` 和 `mixed_scheduling`

## [2026-04-12] fix: TypeScript 类型错误 ApiResponse 断言

**影响范围**: frontend/src/api/client.ts
**上游兼容性**: 低风险，类型断言修复
**变更详情**:
- `as Record<string, unknown>` 改为 `as unknown as Record<string, unknown>`，消除 TS2352 编译错误

## [2026-04-12] feat: 账号列表显示邮箱 + AI Credits 汇总

**影响范围**: frontend/src/views/admin/AccountsView.vue
**上游兼容性**: 中风险，AccountsView 改动较多，合并时注意
**变更详情**:
- 账号名称下方显示邮箱，兼容 `credentials.email`（Antigravity）和 `extra.email_address`（Anthropic）
- 筛选栏右侧新增 AI Credits 汇总标签，异步获取并按邮箱去重
- `load()` 和 `reload()` 均触发汇总刷新

## [2026-04-12] feat: 搜索支持按邮箱查找账号

**影响范围**: backend/internal/repository/account_repo.go
**上游兼容性**: 低风险，搜索条件扩展
**变更详情**:
- 账号搜索从仅匹配 `name` 扩展为同时匹配 `credentials.email` 和 `extra.email_address`（使用 sqljson.StringContains）

## [2026-04-12] fix: Antigravity refresh_token 未保存导致账号不可调度

**影响范围**: backend/internal/service/antigravity_oauth_service.go
**上游兼容性**: 低风险，回填逻辑
**变更详情**:
- `ValidateRefreshToken` 刷新后 Google 不返回新 refresh_token，导致存入 credentials 为空
- 新增回填逻辑：如果刷新响应中 refresh_token 为空，使用用户传入的原始值

## [2026-04-12] feat: 批量导入支持使用邮箱作为账号名称

**影响范围**: frontend/src/components/account/CreateAccountModal.vue, frontend/src/i18n/locales/zh.ts, en.ts
**上游兼容性**: 低风险，新增 UI 选项
**变更详情**:
- 新增 `useEmailAsName` 选项，仅 Antigravity 平台可见
- 勾选后隐藏名称输入框，批量和单个 OAuth 创建均使用邮箱作为名称

<!-- 
示例条目：

## [2026-04-15] feat: 新增企业微信支付方式

**影响范围**: backend/internal/payment/, frontend/src/views/admin/
**上游兼容性**: 低冲突风险，新增文件为主
**变更详情**:
- 新增 payment/provider/wechat_work.go
- 添加 WeChatWorkProvider 实现 PaymentProvider 接口
- 前端管理页新增企业微信支付配置表单
- config.yaml 新增 payment.wechat_work 配置段

**关联 Issue/PR**: #12

## [2026-04-20] fix: 修复 Gemini 账户 OAuth 刷新 Token 超时

**影响范围**: backend/internal/service/account.go
**上游兼容性**: 可能与上游同区域修改冲突，合并时注意
**变更详情**:
- OAuth token refresh 超时从 10s 改为 30s
- 新增重试逻辑（最多 3 次，指数退避）

**关联 Issue/PR**: 无（线上排查发现）
-->
