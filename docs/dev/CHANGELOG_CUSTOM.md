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
