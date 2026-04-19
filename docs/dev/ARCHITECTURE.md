# Sub2API 技术架构文档

> **入口文档**。新会话开始探索代码前先读此文件，再按导航进入 `codebase/*.md` 的模块深入文档。
> 架构层面有变动（新模块 / 新跨切面约定 / 新已知坑）——更新本文。

---

## 0. 使用指南

| 场景 | 怎么做 |
|------|-------|
| 第一次接触项目 | 顺序读：本文 §1–§4 → `CLAUDE.md` 开发规则 → 需要修改的模块 `codebase/{模块}.md` |
| 要改某一模块（账号 / 计费 / 模型映射 / 网关 / 认证 / 支付） | 本文 §7 查映射，再读对应 `codebase/*.md` |
| 要新增一个常见功能（可编辑设置、用户接口、前端页、迁移） | §5 常见任务模板直接抄 |
| 遇到环境 / 工具链诡异行为 | §6 已知坑点 |
| 不确定该不该更新本文 | §0 下面的「更新触发条件」 |

### 本文更新触发条件

下列任一发生 → 立刻更新本文的对应章节，再提交代码：

1. **新增顶层模块**（新 service / 新 handler 组 / 新前端区域）——在 §2、§7 登记
2. **架构层面约定变化**（比如新加一个 DTO 层、调整 Wire 注册顺序、替换 ORM）——更新 §3 或 §4
3. **出现新的跨切面坑**（如 `go generate` 某条会失败、某个路径大小写敏感导致构建失败）——加进 §6
4. **引入可复用的新模式**（比如把「设置 KV + 管理员编辑器」模板化）——加进 §5

如果只是某模块内部的实现细节变化，**不用**动本文，更新对应 `codebase/*.md` 即可。

---

## 1. 技术栈

| 层 | 技术 |
|----|------|
| Backend | Go 1.26.2 / Gin / **Ent ORM + 部分 raw SQL** / **Wire DI** / gRPC 偶尔用 |
| Frontend | Vue 3 (Composition API) / TypeScript / Vite 5 / TailwindCSS / Pinia / **pnpm** |
| DB | PostgreSQL 16+（**唯一**真源） |
| Cache | Redis 7+（scheduler、并发控制、rate limit、dashboard 统计、临时数据） |
| Deploy | Docker (multi-stage) / systemd / GoReleaser |

**前端 embed 进后端 binary**：`pnpm build` 输出到 `backend/internal/web/dist`，Go 用 `-tags embed` 编译期嵌入。生产环境只分发一个 Go 二进制。

---

## 2. 代码分层

### 2.1 后端（`backend/`）

```
cmd/server/                  main.go、wire.go、wire_gen.go、VERSION
ent/                         Ent 生成代码（不手改） + schema/（手写 schema）
internal/
  config/                    Viper 配置加载
  domain/                    领域常量 / 简单值对象（不依赖 service）
  handler/                   HTTP handler（Gin）
    admin/                     管理员子路由 handler
    dto/                       Request/Response DTO + 展示转换工具
  service/                   业务逻辑（最大的包）
  repository/                数据访问层（Ent 查询 + 部分 raw SQL）
  server/                    HTTP 服务器装配
    middleware/                Gin middleware（JWT、admin 鉴权、限流、recover）
    routes/                    路由注册
  payment/                   支付 provider 实现（Stripe / 微信 / 支付宝）
  pkg/
    errors/                    统一应用错误（infraerrors.BadRequest 等）
    response/                  统一响应（response.Success / ErrorFrom / Unauthorized）
    pagination/                分页参数 + 结果封装
  model/                     旧数据模型残留（逐步迁到 service 领域对象）
  web/                       embedded 前端 dist 的 FS wrapper
migrations/                  Atlas 风格 raw SQL 迁移（不是 Ent auto-migrate）
```

### 2.2 前端（`frontend/src/`）

```
api/                         Axios clients，按模块拆文件（keys / usage / pricingPage …）
  client.ts                  预配置 baseURL + auth + refresh + 语言头的 apiClient 实例
  index.ts                   聚合导出
  admin/                     管理员专用 client（modelPricing、userModelPricing …）
components/
  layout/                    AppLayout、AppSidebar、AppHeader、TablePageLayout
  common/                    DataTable、Select、Icon、AnnouncementPopup …
  admin/                     管理员页面用的局部组件（按模块分目录）
views/
  auth/                      登录/注册/OAuth 回调
  user/                      普通用户页（Dashboard、Keys、Usage、Pricing、Subscriptions…）
  admin/                     管理员页（UsersView、ChannelsView、ModelConfigView…）
  setup/                     首次部署向导
stores/                      Pinia（app、auth、announcements…）
router/                      vue-router 配置（meta 带 requiresAuth/Admin + titleKey）
i18n/                        vue-i18n；`locales/{zh,en}.ts` 是真源
types/                       TypeScript interface（PublicSettings 等共享类型）
composables/                 Vue composition 工具
```

---

## 3. 后端关键路径

### 3.1 请求生命周期

```
Client
  ↓
Gin router (server/router.go)
  ↓
middleware: recovery → CORS → rate-limit → jwtAuth / adminAuth / apiKeyAuth
  ↓
handler.* (param parse, DTO bind, permission check)
  ↓
service.* (business logic, multi-repo orchestration, cache)
  ↓
repository.* (Ent queries / raw SQL / Redis)
  ↓
response.Success / response.ErrorFrom (unified envelope)
```

**网关路径** 走特殊流程（不同于普通 CRUD）：`handler/gateway_handler.go` → `service/gateway_service.go` → 账号调度器 + HTTPUpstream → 直接流式返回给客户端。详见 `codebase/gateway.md`（TBD）。

### 3.2 依赖注入（Wire）

所有服务/仓储/handler 在启动时由 Wire 组装，**不是运行时反射**。

- **Provider 声明**：`backend/internal/{service,repository,handler}/wire.go` 里各有一个 `ProviderSet`
- **组装入口**：`backend/cmd/server/wire.go`（手写）→ `wire_gen.go`（Wire 自动生成）
- **两层 Handler struct**：
  - `handler.AdminHandlers` —— 所有 `admin.*Handler` 指针
  - `handler.Handlers` —— 所有顶层 handler + 嵌入 `*AdminHandlers`

**添加一个新 handler 的标准步骤**：

1. 在 `handler/` 或 `handler/admin/` 写 `NewXxxHandler(deps...)` 构造函数
2. 在 `handler/handler.go` 对应 struct 加字段
3. 在 `handler/wire.go`：
   - `ProvideAdminHandlers` / `ProvideHandlers` 参数 + 赋值同步加一行
   - `ProviderSet` 里加 `NewXxxHandler`（admin 的加 `admin.NewXxxHandler`）
4. 跑 `go generate ./cmd/server`（**注意**见 §6 的 Wire 坑）
5. 在 `server/routes/admin.go` 或 `user.go` 挂路由

### 3.3 Settings / PublicSettings KV（高频使用）

项目里所有「管理员可配置的运行时参数」都走这一套，**不要**再造轮子。

- **表**：`settings(key text unique, value text, updated_at timestamptz)`（Ent schema: `backend/ent/schema/setting.go`）
- **Key 常量**：`backend/internal/service/domain_constants.go`（命名约定 `SettingKeyXxx`，值通常是 `snake.case` 或 `snake_case.group` 的字符串）
- **仓储接口**：`service.SettingRepository`
  ```go
  Get / GetValue / Set / GetMultiple / SetMultiple / GetAll / Delete
  ```
  实现：`backend/internal/repository/setting_repo.go`（Ent，带 upsert）
- **高级封装**：`backend/internal/service/setting_service.go`
  - `GetPublicSettings(ctx)`：读白名单 key，装配成 `service.PublicSettings`
  - `GetPublicSettingsForInjection`：SSR 注入版（window.`__APP_CONFIG__`）
  - 语义化查询方法：`IsRegistrationEnabled` / `GetSiteName` / ...
- **公开接口**：`GET /api/v1/settings/public`（无需鉴权）
- **前端消费**：`appStore.cachedPublicSettings`（Pinia，首载后缓存；改完 setting 后调 `fetchPublicSettings(true)` 强刷）

**新增一个公开 setting 字段的六步法见 §5.1**。

### 3.4 数据库迁移

- 位置：`backend/migrations/NNN_*.sql`
- 编号：三位数字前缀，**严格递增**，别跳号
- embed：`migrations.go` 用 `//go:embed *.sql` 打包，启动时按序执行（幂等：都用 `IF NOT EXISTS`）
- 原则：**additive 优先**（加列、加索引）、避免 `DROP`；删除字段要两步走（先停止写 → 再 drop）
- **不要**用 Ent 的 auto-migrate；它不跑。所有 schema 变更必须写 SQL 迁移。

### 3.5 缓存策略（模式化）

Redis cache 命名：`repository/*_cache.go`。常见模式：

| 模式 | 例子 | 关键点 |
|------|------|-------|
| 进程内 TTL 缓存 | `settingService.isBackendModeCache`（`atomic.Value` + singleflight） | 60s TTL；错误情况 5s TTL；用 `singleflight` 防雪崩 |
| 懒加载整表内存 cache | `service.GlobalPricingCache`（`global_model_pricing_cache.go`） | 首次访问加载所有 enabled 条目；CUD 后调 `Invalidate()` |
| Redis 分布式缓存 | `repository.SchedulerCache`、`APIKeyCache`、`BillingCache` | TTL 可配置；失效通过显式 `invalidate` 或 TTL 自然过期 |
| 惰性写 + 异步 flush | `usageRecordWorkerPool` | 用户请求不等写 DB；worker pool 批量 flush |

**新加缓存前先问**：这个数据读频率高到必须缓存吗？缓存失效怎么处理？否则直接打 DB。

### 3.6 认证与授权

- **JWT**：用户登录后发放 access + refresh token；中间件 `server/middleware/jwt_auth.go`
- **提取身份**：handler 里用 `middleware.GetAuthSubjectFromContext(c)` → `AuthSubject{UserID, Concurrency}`
- **管理员判定**：`adminAuth` middleware（`server/middleware/admin_auth.go`），比 jwtAuth 多一层 role 检查
- **API Key**：用户生成的 key 挂到 `/v1/*` 网关路径；中间件 `api_key_auth.go`，查询 `APIKeyService.AuthenticateAndCharge`
- **TOTP 2FA**：可选；启用后登录后多一步验证
- **LinuxDO / OIDC**：外部 SSO；回调走 `auth/*OAuth handler`

### 3.7 模型定价解析链

见 `codebase/billing.md`。要点：四级优先级 `Channel > Global > LiteLLM > Fallback`，还有一层 `User override` 放在展示层（`handler/dto/display_pricing.go`）——**用户展示**的 token / cost 可以与实际扣费不一致（display price 只影响 UI，`actual_cost` 永远按真实价扣）。

---

## 4. 前端关键路径

### 4.1 路由 + 权限

`src/router/index.ts` 是唯一真源。路由 `meta`：

```ts
{
  path: '/admin/xxx',
  name: 'AdminXxx',
  component: () => import('@/views/admin/XxxView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: true,      // 管理员页必须有
    title: 'Xxx',
    titleKey: 'admin.xxx.title',        // 用于 document.title 和头部
    descriptionKey: 'admin.xxx.description'
  }
}
```

路由守卫（`router/index.ts` 末尾的 `beforeEach`）自动读 meta 做鉴权跳转。**不要**在组件里手写 role 判断。

### 4.2 Store（Pinia）

- **`stores/app.ts`** —— 全局 UI 状态 + public settings 缓存
  - `cachedPublicSettings` / `publicSettingsLoaded` / `publicSettingsLoading`
  - `fetchPublicSettings(force?: boolean)` —— 改动后台 setting 保存成功后 **务必** 用 `true` 调一次，让已打开页面拿到新值
  - `showSuccess(msg)` / `showError(msg)` / `showWarning(msg)` —— 全局 toast 入口（不要用 `window.alert` / 自己写 toast）
  - 语义属性：`siteName` / `siteLogo` / `docUrl` / `paymentCnyPerUsd` / `backendModeEnabled` …
- **`stores/auth.ts`** —— user + token + role
- **`stores/announcements.ts`** —— 公告轮询 / popup

### 4.3 API Client 约定

模板见 `src/api/keys.ts`（最标准）：

```ts
import { apiClient } from './client'

export async function list(...): Promise<...> {
  const { data } = await apiClient.get<...>('/keys', { params, signal })
  return data
}

export const keysAPI = { list, getById, create, update, delete: deleteKey }
export default keysAPI
```

- `apiClient` 已预配 `/api/v1` baseURL、token 注入、401 刷新
- 路径写成 baseURL **之后** 的部分（`/keys` 实际打 `/api/v1/keys`）
- 在 `src/api/index.ts` 聚合导出
- **类型** 放 `src/types/index.ts`（共享）或与 client 同文件内（仅本模块）

### 4.4 布局组件

- **`AppLayout`** —— 所有登录后页面的默认壳：侧边栏 + header + 主内容 slot
- **`TablePageLayout`** —— 表格类页面模板：`#actions` / `#filters` / `#table` 三 slot
- **`DataTable`** —— 通用表格（排序、分页、展开、selector）

**不要** 在 view 里再写 sidebar / header，全部用 `<AppLayout>` 包起来。

### 4.5 i18n

- 文件：`src/i18n/locales/{zh,en}.ts`——同一个路径两套值
- 取值：`const { t, locale } = useI18n()`；`t('nav.dashboard')` / `t('pricing.title')`
- 添加 key：**必须** 两边同步加，否则另一侧会 fallback 到 key 字面量
- 命名空间约定：
  - `nav.*`：侧边栏 & 顶部导航
  - `common.*`：通用按钮、loading、错误
  - `auth.login.*` / `auth.register.*`：登录注册
  - `admin.{module}.*`：管理员页专用
  - 用户侧单模块：模块名作顶层 namespace（`pricing.*`、`usage.*`）

### 4.6 用户反馈

- 成功 / 失败：`appStore.showSuccess(t('common.success'))` / `appStore.showError(...)`
- 确认对话框：原生 `window.confirm(...)` 即可（见 `LoginPageView.vue` 的 resetConfirm）
- 字段 validation：handler 绑 `ref` 到字段，`:class="{ 'input-error': errors.email }"`

---

## 5. 常见开发任务模板

### 5.1 新增一个公开（login 页也能用）的 admin-editable 设置字段

1. `backend/internal/service/domain_constants.go` 加 `SettingKeyXxx = "xxx"`
2. `backend/internal/service/settings_view.go` 的 `PublicSettings` struct 加字段（Go 裸字段，没 json tag）
3. `backend/internal/service/setting_service.go`：
   - `GetPublicSettings` 的 keys 列表加 `SettingKeyXxx`
   - 组装 struct 时把值从 `settings[SettingKeyXxx]` 拿出
   - `GetPublicSettingsForInjection` 同样加一行
4. `backend/internal/handler/dto/settings.go` 的 `PublicSettings` DTO 加字段（带 `json:"xxx"` tag）
5. `backend/internal/handler/setting_handler.go` 的 `GetPublicSettings` 方法映射到 DTO
6. `frontend/src/types/index.ts` 的 `PublicSettings` 接口加字段
7. 在管理员编辑页保存（`settingRepo.SetMultiple`）；前端消费 `appStore.cachedPublicSettings?.xxx`

**若一次加一组字段**（如「登录页文案」的 8 个字段），按 §5.2。

### 5.2 新增一组字段（嵌套子结构）

在 5.1 基础上：

- Service 层定义 `XxxContent struct{ A, B, C string }` 并把 `PublicSettings.Xxx *XxxContent` 设为指针
- 写一个 `buildXxxContent(settings map[string]string) *XxxContent`：所有字段空 → 返回 nil（让 JSON `omitempty` 生效）
- DTO 和前端 TS 类型都用「整个子结构 optional」的形式（`Xxx?: XxxContent`）
- 前端消费时 `publicSettings.xxx?.field ?? fallback`

**参考实现**：`service.LoginPageContent` + `handler/admin/login_page_handler.go` + `views/admin/LoginPageView.vue`

### 5.3 新增用户侧 API 接口

1. `backend/internal/handler/xxx_handler.go` 写 handler 结构 + `NewXxxHandler(deps...)`
2. `handler.go` 的 `Handlers` struct 加字段、`wire.go` 的 `ProvideHandlers` 加参数、`ProviderSet` 加 `NewXxxHandler`
3. `cmd/server/wire_gen.go` **手工**对应插入（见 §6 Wire 坑）
4. `server/routes/user.go` 挂路由：`authenticated.GET("/xxx", h.Xxx.Get)`
5. 前端 `src/api/xxx.ts` 写 client + `src/api/index.ts` 聚合

### 5.4 新增 Ent schema 字段

1. 改 `backend/ent/schema/xxx.go`
2. 跑 `go generate ./ent`（通常能过）
3. **另外** 写一条 raw SQL 迁移到 `backend/migrations/NNN_*.sql`（Ent 自动迁移**没启用**）
4. 所有 mock / stub / test 实现该 schema 的接口的地方都要加新字段处理

### 5.5 新增前端页面

- 用户页：`views/user/XxxView.vue` + `router/index.ts` 加路由 + `components/layout/AppSidebar.vue` 的 `userNavItems` 加条目 + i18n 加 `nav.xxx` 和 `xxx.*`
- 管理员页：路径改成 `/admin/xxx`，sidebar 加到 `adminNavItems`
- 页面模板：`<AppLayout>` + 内容；列表型用 `<TablePageLayout>`
- 多 tab 页：看 `views/admin/PageContentView.vue` 或 `ModelConfigView.vue`——用 `activeTab` ref + `?tab=xxx` URL 同步 + `<KeepAlive>` 保留子表单状态

### 5.6 新增 i18n 键

- 同时改 `zh.ts` 和 `en.ts`，否则另一侧 fallback 到 key 字面量
- 不确定某 key 被哪个组件消费？`Grep` 搜 `'the.key.path'`

---

## 6. 已知坑点 & 本地约定

### 后端 / Go

| 坑 | 处理方式 |
|----|--------|
| **`go generate ./cmd/server` 在主干上预先失败**（`PaymentConfigService` / `PaymentOrderExpiryService` 重复绑定） | 手工编辑 `backend/cmd/server/wire_gen.go` 按现有格式插入构造函数和参数。上游修复前都这么办。 |
| Ent auto-migrate 没启用 | 所有 schema 变更必须写 `backend/migrations/NNN_*.sql` |
| 改接口后忘记 mock | 构建挂在 `mockXxxRepo does not implement ...Repository (missing method)`——搜所有 `mock*Repo struct` 把新方法补齐 |
| `admin.NewUsageHandler` 类签名经常变 | Wire 手补时参数要对齐当前函数签名（注意相邻提交） |
| `response.ErrorFrom` vs `response.Unauthorized` | 前者用于 `ApplicationError`；后者用于认证失败。常用：`response.ErrorFrom(c, infraerrors.BadRequest("CODE", "msg"))` |

### 前端 / Node

| 坑 | 处理方式 |
|----|--------|
| **只能 pnpm**，不能 npm | 混过了就 `rm -rf node_modules && pnpm install --frozen-lockfile` |
| `pnpm-lock.yaml` **必须** 提交 | CI 用 `--frozen-lockfile` |
| 新加 i18n key 只加一侧 | 另一侧会 fallback 到 key 字面量，界面看起来就是 `"pricing.title"` 这种 |
| `AppLayout` 不能嵌套 | 复用现有视图要么拆子组件去掉 AppLayout，要么做成 tab 的 child |
| `paymentCnyPerUsd` 等首屏数据 | 走 `window.__APP_CONFIG__` SSR 注入，避开 fallback 闪屏。改后端 `GetPublicSettingsForInjection` 时要加上 |

### Git / 构建 / 部署

| 坑 | 处理方式 |
|----|--------|
| **`docs/dev` 被 gitignore** | 本文件、CHANGELOG_CUSTOM、codebase/*.md 都要用 `git add -f` |
| **Git Bash 改 POSIX 路径** | 在 Windows 用 `taskkill /PID 12345` 会被 MSYS 改写为路径；改用 `cmd //c "taskkill /PID 12345 /F"` |
| **Git stash pop 会带出旧 WIP** | 本地 `git stash list` 里可能有以前的 "wip: all changes"；pop 前先看 |
| `deploy/remote_exec.py` 直接传 `/opt/...` 路径 | MSYS 会转 Windows 路径，命令失败；用预置 shortcut：`python deploy/remote_exec.py --update` |
| **push / deploy 需要显式同意** | commit 自动做，push 到 origin 与跑 `remote_exec.py` 必须每次当面获得「推」/「部署」类指令 |

### 本地开发

| 坑 | 处理方式 |
|----|--------|
| 端口 8080 被 Docker 占用 | 本地 backend 必须 `SERVER_PORT=8081` |
| `localhost` psql 连接失败 | Windows 上改 `127.0.0.1` |
| psql 路径包含中文 | 不行。整个仓库放英文路径下。 |
| 原生 `make` 不一定有 | 直接跑 Makefile 里对应的 `go ...` / `pnpm ...` |
| **后端改代码不热重载** | `go run ./cmd/server` 改完必须 Ctrl-C 重启；前端 Vite 自动热更 |
| 启动脚本需要一堆环境变量 | 拷 `DEV_GUIDE.md` 或 `CLAUDE.md` 的启动块；简短版见 `deploy/.env.example` |

---

## 7. 深度文档导航

已有的模块深入文档（`docs/dev/codebase/`）：

| 模块 | 文档 | 简述 |
|------|------|------|
| 代码地图索引 | [codebase/README.md](codebase/README.md) | 所有模块索引 |
| 账号管理 | [codebase/account.md](codebase/account.md) | 账号 CRUD、多 provider OAuth、AI Credits、批量导入 |
| 计费系统 | [codebase/billing.md](codebase/billing.md) | 四级定价链、费用计算、费率乘数、缓存命中计费 |
| 模型映射 | [codebase/model-mapping.md](codebase/model-mapping.md) | 模型白名单 / 映射、默认映射、网关解析、通配符 |

**尚未编写** 的（按需补）：`gateway.md`（API 网关核心）、`auth.md`（认证体系）、`payment.md`（支付）、`announcements.md`（公告）、`ops.md`（运维监控）、`page-content.md`（页面文案管理：`login_page.*` / `pricing_page.*` settings key + 管理员 tab 页 + 前端 fallback 机制）。

其他顶层文档：

- **开发规则 / 启动命令**：`CLAUDE.md`（仓库根）
- **部署运维**：`docs/dev/DEPLOYMENT.md`
- **二开指南**：`docs/dev/SECONDARY_DEV.md`
- **上游合并历史**：`docs/dev/UPSTREAM_SYNC.md`
- **二开变更日志**：`docs/dev/CHANGELOG_CUSTOM.md`
- **dev env 配置 / 常见坑**：`DEV_GUIDE.md`
