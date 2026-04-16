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
