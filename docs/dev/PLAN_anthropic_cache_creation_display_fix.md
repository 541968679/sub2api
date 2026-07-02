# Anthropic 平台缓存创建(cache creation)展示修复方案

> 状态:**主体已实施**(2026-07-02,分支 codex/cache-creation-display-20260702)。
> 实施与本稿 §2 的差异(用户拍板后的最终设计):放弃"premium 折入 input"的对称方案,
> 改为**直接反算放大缓存创建自身 token 数**(display_tokens = 真实成本 ÷ 展示价,成本守恒)。
> 该设计是线性变换,§3 数学验证提出的"混合符号 premium 聚合不一致"与"InputTokens=0 蒸发"
> 两个问题不复存在;同时用户侧(UsageView/KeyUsageView)已补齐 cache creation 可见性。
> 实施记录见 docs/dev/CHANGELOG_CUSTOM.md [2026-07-02] 与 billing.md "Display cache creation price" 节。
> **2026-07-02 第二批已全部完成**:Phase 0 计费污染修复(6054412c)、1h 缓存分档真实计费
> cache_write_1h_price(71c65280)、下游响应缓存创建展示改写含分档倍率与嵌套同步(7f4f7fee)。
> 详见 CHANGELOG_CUSTOM.md 同日三条目与 billing.md 相关章节。本文档全部工作项已闭环。
> 产出方式:12-agent 调研工作流(6 维度代码调研 → 2 份竞争设计 → 评审合成 → 3 个对抗验证)。
> 日期:2026-07-02

## 1. 问题定义(调研结论)

生产现象:anthropic 平台分组(如 group 23 "claude fable 5")的使用记录出现 `input=2, output=38, total_cost≈$0.54` 这类"token 很少但很贵"的行。真实原因是该行 `cache_creation_tokens=42778`(Claude Code 首次请求写入 42.8k 缓存),**计费正确**,但展示层完全没有把缓存创建纳入换算。

根因共 4 处(全部经代码亲验):

1. **展示定价只有 3 个价**。`GlobalModelPricing` / `UserModelPricingOverride` 只有 `DisplayInputPrice / DisplayOutputPrice / DisplayCacheReadPrice`,没有缓存创建展示价([display_pricing.go:10-14](../../backend/internal/handler/dto/display_pricing.go))。
2. **`ApplyDisplayTransform` 不动 `CacheCreationCost`**([display_pricing.go:81-119](../../backend/internal/handler/dto/display_pricing.go))。用户可见 TotalCost 含真实缓存创建成本;且 `cache_creation_cost / cache_creation_tokens` 比值 = **真实缓存写单价**(该比值在 `ApplyUserDisplayRate` 等比缩放下不变),违反"用户不得看到真实单价"不变量(billing.md 2026-06-19)。
3. **admin 双列对比 `DisplayUsageFields` 无 cache_creation 字段**(display_pricing.go:141-149)。
4. **下游响应 usage 改写 `CacheCreateMult` 恒 1.0**([display_token_rewrite.go:160](../../backend/internal/service/display_token_rewrite.go)),OpenAI 格式路径把 cacheCreate 当 0 传(:588)。本批**不改**(见 §5 非目标)。

澄清两个相关系统:

- 用户说的"自定义缓存比例" = `openai_claude_gpt_bridge_cache_display_settings`(桥接随机 cache_read 占比)。它只作用于 **antigravity 分组**的 claude-gpt 桥接(硬 gate 在 [routes/gateway.go:39](../../backend/internal/server/routes/gateway.go):`groupPlatform == PlatformAntigravity && ShouldUseClaudeGPTBridge`),只造 cache_read,与 cache creation 零交集,**本方案不触碰**。
- claude-gpt 桥接的 usage 经 `copyOpenAIUsageFromResponsesUsage`([service/openai_gateway_messages.go:1224-1236](../../backend/internal/service/openai_gateway_messages.go))只置 Input/Output/CacheRead,桥接行 cache_creation 恒 0,天然不受影响。

### 调研中发现的两个既有 bug(顺带修)

- **[Phase 0,必须先修] OAuth 流式计费污染**:[gateway_service.go](../../backend/internal/service/gateway_service.go) 流式路径在 :7420/:7427 先对 usage map 做展示改写(`ApplyDisplayMultipliersToUsageMap`),:7442 才 `extractSSEUsagePatch` 提取计费 patch(经 :7561 `mergeSSEUsagePatch` 喂计费)。display 模式(migration 169 起为新用户默认)下 **计费被展示值污染**。注意影响面:该路径服务所有走 `GatewayService.handleStreamingResponse` 的账号,含 anthropic OAuth、CC-compat(Kimi 等)、**antigravity 分组的 apikey 型账号**;修复会把这些行的计费从污染值改回真实值,发布说明需提及。修法:patch 提取移到展示改写之前、但保持在 cache TTL override(:7398-7413,刻意影响计费)之后。
- **`ApplyUserDisplayRate` 不缩放 5m/1h 细分**(display_pricing.go:173-176 只缩放总量),缩放后细分不加和。修复时注意取整:一档取整后另一档用减法导出(`1h = scaledTotal − scaled5m`,截非负),否则 `round(5m×s)+round(1h×s) ≠ round(total×s)` 差 ±1。

## 2. 核心设计:第 4 展示价 `display_cache_creation_price`

完全对称 2026-05-06 的 cache-read premium 机制,不发明新数学:

```
守卫(五重,全部满足才生效,否则整块 no-op 保持真实值):
  cfg.DisplayCacheCreationPrice != nil 且 > 0
  && cfg.DisplayInputPrice != nil 且 > 0     (premium 需有处折入)
  && d.CacheCreationTokens > 0
  && d.CacheCreationCost > 0
  && d.InputTokens > 0                       (验证补充:否则正 premium 无处折入而 cost 已改低,
                                              TotalCost 会反向跌破守恒;cache-read 分支同型缺口记入 billing.md)

变换(插在 cache-read 分支 :102 之后、input 反算 :104 之前):
  displayCacheCreationCost = CacheCreationTokens × displayCacheCreationPrice   // token 保真,cost 重定价
  creationPremium = realCacheCreationCost − displayCacheCreationCost
  if creationPremium > 0 { inputCostForDisplay += creationPremium }            // 负 premium 忽略,与 cache-read 同规则
  d.CacheCreationCost = displayCacheCreationCost
  // 之后走既有唯一一次 input 反算 + TotalCost delta(:86/:116-117),自动吸收,按次/图片计费不受影响
```

关键性质:

- **token 保真**(不反算 cache_creation token):保住 5m/1h 加和、`GetUserDisplayAggregateGroups` 的 GROUP BY/SUM 契约零改动、与下游响应 token 数天然一致。
- **变换后 cost/token 比值 = 展示价**,堵住真实缓存写单价泄漏(现状最大问题)。
- **premium 用成本差额**(而非单价差×token),5m/1h 混合天然精确。
- cache-read 与 cache-creation 两个 premium 在同一累加器复合,只做一次 input 反算,不重复计数。
- 长上下文:`effectiveDisplayPricingForUsageLog`(:56-76)克隆时新价乘 `LongContextInputMultiplier`(缓存写属输入侧)。
- `ApplyUserDisplayRate` 串联顺序不变,复合后比值仍 = 展示价。
- 单一平价,**不分 5m/1h 两档**(先例:真实价全局覆盖 `CacheWritePrice` 也只设 5m 档)。

### 运营取值指引(写入表单 hint 与 `applyDisplaySuggested`)

- **推荐:展示创建价 ≈ 1.25 × 展示输入价**(Anthropic 官方比例)→ premium≈0,用户 tooltip 分项恰好加出总额,"token 少但贵"消失。
- 或 = 真实缓存写价 → 仅消单价泄漏,成本原样可见。
- 设过低会把 premium 折成大量展示 input token(生产例:input 从 2 变数万)——机制预期(成本守恒)但观感突兀。
- **一致性约束(验证升级,非仅建议)**:展示创建价应 ≤ 有效 5m 真实单价,保证全组 premium 非负。原因:`max(0, premium)` 非线性,组内混合正负 premium 时"逐行求和 ≠ 聚合组变换"(5m/1h 价差 + cache TTL override 使组内真实单价逐行可变),仪表盘全时段与记录列表会出现可见口径差。等价性测试 fixture 限定符号一致场景,另加混合符号场景断言"允许的偏差"。

## 3. 平台影响边界(⚠ 含需拍板项)

采用"配置 + 数据"双重软 gate,不加硬平台判断:

| 路径 | cache_creation 现状 | 影响 |
|---|---|---|
| openai 原生(真 OpenAI 上游) | 恒 0(`ResponsesUsage` 无此字段) | 数值 no-op,逐字段不变 |
| antigravity OAuth 账号(Gemini 变换) | 恒 0(pkg/antigravity 从不赋值) | 不变 |
| claude-gpt 桥接 | 恒 0(copyOpenAIUsageFromResponsesUsage) | 不变 |
| gemini | 恒 0(只映射 cachedContentTokenCount→cache_read) | 不变 |
| 下游响应 usage(三平台) | 本批不改 display_token_rewrite.go | 字节级不变 |
| **antigravity 分组:upstream 中转账号** | **今天就 >0**(antigravity_gateway_service.go :4634-4664/:4597-4631 解析 cache_creation 含 5m/1h) | **claude-* 模型配了展示价后会被换算** |
| **antigravity 分组:apikey 型账号** | **今天就 >0**(走 anthropic gatewayService.Forward,gateway_handler.go:760-763) | 同上 |
| openai 平台 relay 透传 | 可 >0(extractOpenAIUsageFromJSONBytes :4859/:4880 解析并入库;含 ws 转发 openai_ws_forwarder.go:404、responses→chat fallback :227) | claude-* 模型配了展示价后会被换算 |

**拍板点 A**:后三行的波及在语义上是正确的(缓存创建真实发生、真实计费,展示换算一视同仁),但字面上突破"只影响 anthropic 平台"。两个选项:

- **A1(推荐)**:接受软 gate。展示价按模型名配置(claude-* 才配),上述行获得同样正确的展示换算;平台隔离回归测试钉死"恒 0 路径"(openai 原生/antigravity OAuth/桥接/gemini),对 antigravity 中转/apikey 行显式验收换算语义;billing.md 记录边界。
- **A2**:硬 gate(usage DTO 链 join groups.platform + `GetByID` 补 Group hydration)。代价:聚合 SQL 与详情页两处扩数据加载,且 `DisplayAggregateGroup` 合成行(handler/usage_handler.go:996-1022 只有 GroupID 无 Group 实体)需同步,否则列表/详情/汇总三面口径不一致——这正是评审否掉硬 gate 的原因。

## 4. 分阶段改动清单

### Phase 0 — 前置计费修复(独立 commit 先行)

| 文件 | 改动 |
|---|---|
| `backend/internal/service/gateway_service.go` | `extractSSEUsagePatch`(:7442)移到 display rewrite 块(:7415-7431)之前、cache TTL override 块(:7398-7413)之后。回归:Kimi/CC 兼容流、`reconcileCachedTokens`、`needModelReplace`、TTL override 仍先于 patch 生效 |

### Phase 1 — 后端展示换算全链

| 层 | 文件 | 改动 |
|---|---|---|
| db | `backend/migrations/171_add_display_cache_creation_price.sql`(最新已亲验为 170) | ① `global_model_pricing` ADD COLUMN IF NOT EXISTS `display_cache_creation_price` DOUBLE PRECISION(模板 107);② **`user_model_pricing_overrides`** 同列(模板 108);③ **`user_model_pricing_overrides`** 表加 NOT VALID CHECK `user_model_pricing_display_cache_creation_non_negative_check`(DO $$ pg_constraint 按名查,模板 147;147 已应用不可原地扩展);④ COMMENT 注明仅影响展示。两表均 raw-SQL repo,无 Ent schema,**无需 ent/Wire regen** |
| repo | `backend/internal/repository/global_model_pricing_repo.go` | 4 个枚举点:selectColumns(:17)、Create(:137,:146)、Update(:166,:174)、scan(:271)。漏一处编译通过但字段永远 nil(GlobalPricingCache 整表加载走同一 scan) |
| repo | `backend/internal/repository/user_model_pricing_repo.go` | 6 个枚举点:columns(:20)、scanOverride(:49-60)、Create(:151,:160)、Update(:177,:185,$ 占位符全体后移)、BatchUpsert(:210,:220,:229) |
| service | `backend/internal/service/global_model_pricing.go` :39-42 | 实体加 `DisplayCacheCreationPrice *float64` |
| service | `backend/internal/service/user_model_pricing.go` :25-28 | 同上,json tag `display_cache_creation_price`(实体即 API 契约) |
| service | `backend/internal/service/global_model_pricing_service.go` | GlobalOverride API struct(:109-112)与 ToGlobalOverride(:837 邻)加字段;CUD 已有 cache.Invalidate |
| service | `backend/internal/service/user_model_pricing_service.go` :86-97 | 校验列表加新字段(≥0 有限;service 层为权威守卫) |
| handler | `backend/internal/handler/admin/model_pricing_handler.go` | create req(:49-52)、update struct(:74-77)、Create 映射(:160-163)、partial-update applyFloat 键(:279 邻,沿用显式 null 清空) |
| handler | `backend/internal/handler/admin/user_model_pricing_handler.go` | 5 处:create/update req、Create/Update(if-nil merge,不能清空)/BatchUpsert 映射 |
| dto | `backend/internal/handler/dto/display_pricing.go` | **核心,7 处同一提交**:① DisplayPricingConfig 加字段;② BuildDisplayPricingMap 拷贝(:27-31);③ `hasDisplayOverride`(:36-38)纳入新字段(否则仅配此价的条目被静默丢弃);④ effectiveDisplayPricingForUsageLog 长上下文乘 InputMultiplier(:61-70);⑤ ApplyDisplayTransform 插入 §2 对称分支(**五重守卫**);⑥ BuildUserDisplayPricingMap skip(:197)+merge(:206-214);⑦ ApplyUserDisplayRate 补 5m/1h 缩放(**减法导出防取整漂移**) |
| dto | 同上 :123-149 | `DisplayUsageFields` 加 `display_cache_creation_tokens/cost`,`ComputeDisplayFields` 拷贝。**注意口径**:display_fields 是"变换后、rate 缩放前"的中间值(不做 ApplyUserDisplayRate),与既有三字段一致,文档/label 写清 |
| handler | `backend/internal/handler/admin/usage_handler.go` :764-771/:851-857 | `UserViewConfigUsed` 加 `display_cache_creation_price` 及填充。顺带记录既有 drift:前端 usage.ts:307 有 `display_rate_multiplier` 而后端结构体没有,该行恒显示 "—" |
| guard | `backend/tools/upstream-sync-guard/main.go` | 'display pricing dto' 条目(:138-141)追加 `DisplayCacheCreationPrice`;可选:':144+/:153+' 两个实体条目一并登记(现连 DisplayCacheReadPrice 都没登记) |
| docs | `docs/dev/codebase/billing.md` + `CHANGELOG_CUSTOM.md` | 第四展示价语义/五重守卫/软 gate 边界(§3 表)/混合符号 premium 的聚合偏差/下游 input 未折 premium 的已知偏差/Phase 0;`git add -f` |

### Phase 2 — 前端呈现

| 文件 | 改动 |
|---|---|
| `frontend/src/types/index.ts` :1396-1404 | DisplayUsageFields 加两字段;必填 vs optional 需显式决策(必填要批量补 spec fixture) |
| `frontend/src/api/admin/usage.ts` :303-311 | UserViewConfigUsed 加 `display_cache_creation_price: number \| null` |
| `frontend/src/components/admin/usage/UsageTable.vue` :338 邻 | display_fields tooltip 加 display_cache_creation_cost 行(>0 才渲染;全平台共享组件,恒 0 行不出现) |
| `frontend/src/components/admin/usage/UserViewCompareDrawer.vue` :171-181 | configRows 加新价行(format: 'price') |
| `frontend/src/views/user/UsageView.vue` + `KeyUsageView.vue`(可选,拍板点 B) | 成本 tooltip 加 cache_creation_cost、token 徽章/合计纳入 cache_creation(v-if>0)。**硬约束:必须在 Phase 1 之后**,否则直接把真实值印上 UI。i18n 优先用 `usage.*` 命名空间(用户侧惯例,zh.ts:7247 已有 usage.cacheCreation),不跨用 admin.* |

### Phase 3 — 配置面前端

| 文件 | 改动 |
|---|---|
| `frontend/src/api/admin/modelPricing.ts` / `userModelPricing.ts` | 类型加 `display_cache_creation_price?: number \| null` |
| `ModelPricingDetailDialog.vue` | 6 处:第 4 输入框(amber 区 grid-cols-4 有空位)、form state、load/reset/save($/MTok↔per-token 双向换算)、`applyDisplaySuggested` 从真实 cache_write_price 取建议;hint 写 §2 运营指引 |
| `UserModelPricingModal.vue` | 6 处,照 display_cache_read_price 模式 |
| `frontend/src/i18n/locales/zh.ts` + `en.ts` | `admin.modelPricing.displayCacheCreationPrice` + hint key,双文件同步,进 base 块 |
| 计价页(可选,拍板点 C) | `pricing_page_handler.go` pricingPageModel/merge/fallback(CacheWritePrice)+ PricingView 列 + i18n |

## 5. 非目标(明确不改)

- `display_token_rewrite.go` 全部逻辑(CacheCreateMult 恒 1.0、OpenAI 路径 cacheCreate=0、provider 接口签名)——三平台下游响应 usage 字节级不变;唯一残差"下游 input 未折 premium"在推荐配置(premium≈0)下为零。完整的下游方案(CacheCreationInputMult + platform 参数硬 gate)已规格化,作为可选 Phase 4,其前置(Phase 0)本批已落。
- 桥接 `openai_claude_gpt_bridge_cache_display_settings`、antigravity/pkg 代码、真实计费链(billing_service/RecordUsage/actual_cost)、`GetUserDisplayAggregateGroups` SQL、`cache_transfer_ratio`(软废弃)。
- 不做 token 反算、不分 5m/1h 展示价两档、不做 DTO 链硬平台 gate(A2 为后备)。

## 6. 测试计划

1. **Phase 0 回归**:display 模式 + 非平凡 multipliers 的 OAuth 流式,断言计费 patch = 上游真实值、下游 SSE = 展示值;TTL override 仍先于 patch。
2. **display_pricing_test.go 新增**(对称既有 13 用例):正 premium(token 不变/cost=tokens×展示价/premium 折入/delta 正确/actual_cost 与 5m/1h token 不变);负 premium 忽略、TotalCost 上浮;五重守卫任一不满足整块 no-op——尤其 cache_creation=0 的 openai/antigravity-OAuth/桥接形状行断言 DTO 逐字段相等,以及 **InputTokens=0 行 no-op**;双 premium 复合不重复计数;长上下文只乘一次;按次计费行 TotalCost 不变;生产样例回归(展示创建价=1.25×展示输入价,分项之和==TotalCost)。**fixture 用精确可除数值**(既有 assertClose 容差 1e-9,取整漂移 ≤0.5×展示输入价;样例须 ImageOutputCost=0,delta 求和不含它)。
3. hasDisplayOverride/BuildUserDisplayPricingMap 新字段用例;ApplyUserDisplayRate 细分加和==总量(**奇数 token fixture** 钉取整)。
4. **聚合等价**(usage_handler_display_aggregate_test.go):符号一致场景断言 per-group==per-row 之和;混合符号场景断言允许偏差(§2 非线性)。
5. ComputeDisplayFields 单测;display_token_rewrite_test.go 防回归钉(配了新价 CacheCreateMult 仍==1.0,OpenAI 改写输出字节一致);校验单测(负/NaN/Inf 拒)。
6. **repo roundtrip**(integration):两 repo Create/Update/BatchUpsert 后 scan 回读新列非 nil——枚举点漏改的唯一防线。
7. **`go test ./internal/server -run TestAPIContract` 全绿**(用户 DTO 无新 JSON 字段,零破坏预期;DisplayUsageFields 属 admin 契约变更)。
8. 前端 vitest:UsageTable fixture、两个定价表单双向换算、CompareDrawer;`pnpm typecheck`。
9. 端到端(dev-stack,生产形状):anthropic 分组发 cache_control 请求,核对六处口径一致(/api/v1/usage 列表、/:id 详情、dashboard stats/trend、GET /v1/usage 摘要、admin 双列、user-view 预览);顺带实测 KeyUsageView 数据源展示值状态。
10. **平台隔离回归**:openai 原生/antigravity OAuth/桥接/gemini 行钉死不变;antigravity 中转/apikey 行显式验收换算语义(见 §3)。

## 7. 需要拍板的决策

| # | 决策 | 建议 |
|---|---|---|
| A | 软 gate(antigravity 中转/apikey 行、openai relay 行会被同样换算)vs 硬 gate(join platform,三面一致性代价) | A1 软 gate |
| B | 用户端是否渲染 cache creation(消解"token 少但贵"的正面手段;涉及 token 合计口径、UserDashboardCharts show-cache-write 翻转) | 渲染,Phase 2 一并做 |
| C | 计价页是否加"缓存创建价"列 | 可后置 |
| D | 生产 group 23 展示价取值:1.25×展示输入价(premium≈0,推荐)vs =真实缓存写价 vs 更低 | 1.25× |
| E | 用户 DTO 是否继续下发 5m/1h 细分(保留+补缩放 vs 移除) | 先保留观察 |
| F | 历史行口径变化(展示价配置后历史行立即变化,读取时计算)是否公告 | 记 CHANGELOG 即可 |
| G | Phase 0 改变 display 模式用户计费数值(污染值→真实值),发布说明如何措辞 | CHANGELOG + 注明属 bug 修复 |
| H | Phase 4(下游 CacheCreationInputMult)是否立项 | 观察 Phase 1-3 上线后再定 |
