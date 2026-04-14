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
