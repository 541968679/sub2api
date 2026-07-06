# 图片渠道监控:状态时间线 + 用户侧公开展示 设计文档

- 日期: 2026-07-06
- 状态: 已评审通过(用户确认三项关键决策均采用推荐方案)
- 范围: Sub2API 图片渠道监控(自动监控部分)的可视化增强与用户侧公开机制

## 1. 需求

### 目标 1: 管理端可视化

- 监控列表每行内嵌迷你状态条(复用用户侧 `MonitorTimeline` 60 根柱视觉),加 7d 可用率数字。
- 操作列新增「状态详情」,打开弹窗:
  - 24h / 7d / 30d 窗口切换;
  - 耗时折线图:API 总耗时 + 图片下载耗时两条线,空桶断线,失败桶红色背景带;
  - 汇总指标:可用率、平均耗时、最大耗时、检查次数、失败次数;
  - 放大版状态条。

### 目标 2: 用户侧展示 + 每渠道公开配置

- `image_channel_monitors` 新增两字段:
  - `public_visible BOOLEAN NOT NULL DEFAULT FALSE` — 是否在用户侧渠道状态页展示,默认不公开;
  - `public_name VARCHAR(200) NOT NULL DEFAULT ''` — 展示名覆盖,留空回落渠道名,用于掩盖内部命名。
- 用户侧 `/monitor` 渠道状态页在对话渠道卡片下方新增「生图渠道」分组:
  - 卡片形态与现有一致:状态徽章 / 生图总耗时 + 图片下载耗时 / 7d(随窗口切换)可用率 / 60 根状态时间线;
  - 点卡片开简版详情弹窗:7/15/30d 可用率 + 平均耗时(单模型,不复用按模型分组的 MonitorDetailDialog)。
- 管理端渠道编辑表单追加「用户侧展示」区块(开关 + 展示名)。

### 非目标

- 告警通知;
- 日聚合 rollup 表(数据量不需要,见 §3);
- 独立用户侧页面或新导航项;
- 手动检测数据入图(手动检测不写历史表,语义不属于自动监控曲线);
- 每渠道指标粒度开关(耗时/时间线是否展示不做配置);
- 30d 保留期配置化(服务层常量)。

## 2. 现状事实(设计输入)

- `image_channel_monitor_histories` 已有全部所需字段(status、api_header/body/total_ms、image_first_byte/download_ms、image_bytes/width/height、error_stage 等)与 `(monitor_id, checked_at DESC)`、`(checked_at)` 索引;
- 历史表目前无限增长:`DeleteHistoryBefore` 在 repo 有实现但无调用方(死代码);
- 检查频率低(interval 15–3600s,默认 300s → 288 行/天/渠道,每次检查是真实生图消耗);
- 原生渠道监控用户侧组件 `MonitorTimeline` / `MonitorMetricPair` / `MonitorAvailabilityRow` / `ProviderIcon` 均为通用 props,可跨页复用;
- 原生监控无 per-monitor 公开机制(全局 `channel_monitor_enabled` 开关 + 启用即公开),本次为新建机制;
- 前端已有 chart.js 4 + vue-chartjs 5,`OpsLatencyChart.vue` 为延迟折线参照;
- `ops_cleanup_service.go` 每日维护已有 `channelMonitorSvc.RunDailyMaintenance` 挂接模式(失败仅记日志)。

## 3. 数据策略(已决策:方案 A 实时聚合)

按窗口对原始历史表做 `GROUP BY date_trunc` 实时聚合,不建 rollup 表:

| 窗口 | 桶粒度 | 桶数 |
|---|---|---|
| 24h | 10 分钟 | 144 |
| 7d | 2 小时 | 84 |
| 30d | 1 天 | 30 |

- 行内/卡片状态条:最近 60 次检查原始行(按次,不按时间桶,与原生一致);
- 原始数据保留 30 天(`imageMonitorHistoryRetentionDays = 30`),与最大窗口一致;
- 可用率口径与原生一致:`(operational + degraded) / total`;
- 备选方案 B(镜像原生 daily rollup 表)因数据量小且无法提供子日粒度被否决;数据量增长百倍时可在聚合接口签名不变的前提下迁移。

## 4. 后端设计

### 4.1 数据模型

新迁移 `backend/migrations/178_image_channel_monitor_public.sql`:

```sql
ALTER TABLE image_channel_monitors
  ADD COLUMN public_visible BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN public_name VARCHAR(200) NOT NULL DEFAULT '';
```

Ent schema(`backend/ent/schema/image_channel_monitor.go`)同步 + `go generate ./ent`。

### 4.2 repo 新方法(`image_channel_monitor_repo.go`)

- `AggregateTimeline(ctx, monitorID, bucket time.Duration, since time.Time)` → `[]TimelineBucket{BucketStart, Total, Operational, Degraded, Failed, Error, AvgAPITotalMs, AvgImageDownloadMs, MaxAPITotalMs}`;
- `ComputeWindowStats(ctx, monitorID, sinceDays)` → 可用率 + 平均/最大 API 耗时 + 平均下载耗时 + 计数;
- `ListRecentHistoryForMonitors(ctx, ids, limit=60)` → `map[monitorID][]TimelinePoint{Status, APITotalMs, CheckedAt}`,批量消 N+1(镜像原生 aggregator 模式);
- `ListPublicVisible(ctx)` → 公开渠道列表。

### 4.3 API

管理端(现有 `/admin/image-channel-monitors` 路由组追加):

- `GET /admin/image-channel-monitors/:id/timeline?window=24h|7d|30d` → `{summary, buckets[]}`(详情弹窗);
- 现有 `GET /admin/image-channel-monitors`(List)每项追加 `timeline`(最近 60 点),批量查询;
- Create/Update DTO 增加 `public_visible` / `public_name`。

用户侧(authenticated 组,与原生 `/channel-monitors` 并列注册于 `routes/user.go`):

- `GET /api/v1/image-channel-monitors` → `{items: [{id, name, model, latest_status, latest_api_ms, latest_download_ms, availability_7d, availability_15d, availability_30d, timeline[60]}]}`,其中 `name = public_name || 渠道名`。三窗口可用率一次性返回(单条 FILTER 条件聚合 SQL 批量算出),用户侧窗口切换纯前端完成,不走原生的逐卡片懒加载 detail 模式;
- `GET /api/v1/image-channel-monitors/:id/status` → `{id, name, model, windows: {7d/15d/30d 可用率 + 平均耗时}}`(详情弹窗补充平均耗时维度);
- 新 handler `ImageChannelMonitorUserHandler`,进 `Handlers` 容器与 wire(注意本仓库 `go generate ./cmd/server` 已知重复绑定问题,必要时手工对齐 `wire_gen.go`)。

门禁:沿用页面级全局开关 `channel_monitor_enabled`(关闭则整页含图片分组隐藏)+ 每渠道 `public_visible`。不新增全局设置键。已知边缘情况(接受):无法「只公开图片渠道、全局关闭对话渠道监控页」。

### 4.4 净化边界(安全红线)

用户侧 DTO 按白名单构造,仅含 §4.3 列出的字段。绝不下发:内部渠道名(public_name 非空时)、endpoint、上游 host/IP、出口 IP、错误消息、error_stage、图片 URL、代理/账号信息、密钥掩码。以 DTO 序列化快照测试兜底(参照 `api_contract_test.go` 模式)。

### 4.5 维护清理

`ImageChannelMonitorService.RunDailyMaintenance(ctx)`:调用 `DeleteHistoryBefore(now - 30d)`(激活现有死代码),挂接到 `ops_cleanup_service.go` 中 `channelMonitorSvc.RunDailyMaintenance` 旁,失败仅记日志。

## 5. 前端设计

### 5.1 管理端(`ImageChannelMonitorView.vue` + 新组件)

- 监控列表「名称」列 runtime 胶囊区域内嵌 `MonitorTimeline` 迷你条 + 7d 可用率;
- 新组件 `ImageMonitorStatusDialog.vue`:窗口 tabs → chart.js 双线折线(空桶 `spanGaps=false` 断线,失败桶红色半透明背景带)→ 汇总指标卡 → 放大版状态条;
- 编辑表单追加「用户侧展示」区块(public_visible 开关 + public_name 输入)。

### 5.2 用户侧(`ChannelStatusView.vue` + 新组件)

- 现有卡片网格下方新增「生图渠道」section,有公开渠道才渲染;
- 新薄壳组件 `ImageMonitorCard.vue`:组合 `ProviderIcon`(image fallback 图标)+ `MonitorMetricPair`(主=生图总耗时,副=图片下载,替代原生"端点 Ping")+ `MonitorAvailabilityRow` + `MonitorTimeline`;不魔改 `MonitorCard` 本体;
- 简版详情弹窗:单渠道 7/15/30d 可用率 + 平均耗时;
- 跟随页面现有自动刷新(30/60/120s)与窗口切换状态。

### 5.3 i18n

zh/en 双份新增:管理端弹窗与表单文案、用户侧分组标题与卡片指标标签。

## 6. 测试

- 后端 unit:窗口统计/时间线聚合的 service 整形;公开列表过滤(public_visible=false 不出现);public_name 回落;用户侧 DTO 白名单快照;
- 后端聚合 SQL:跟随现有 repo 测试形态(有 integration tag 则补);
- 前端 vitest:ImageMonitorCard 渲染、用户侧 section 有/无数据两态;typecheck + lint + navigation spec;
- 手工:本地注入多天历史数据,浏览器验证三窗口折线、行内条、用户侧卡片与掩名、门禁开关。

## 7. 实施顺序

1. 迁移 + Ent + 保留期清理(小)
2. repo 聚合 + service + 管理端 timeline API + List 扩展(中)
3. 用户侧 API + 净化 DTO + wire/路由(中)
4. 管理端行内条 + 状态详情弹窗(中,弹窗为最大单体)
5. 用户侧分组 + 卡片 + 详情(中)
6. i18n + 模块文档(`docs/dev/codebase/image-channel-monitor.md`)+ CHANGELOG_CUSTOM(小)

## 8. 风险

- Ent regen 与 wire 变更(新用户侧 handler 进容器);`go generate ./cmd/server` 已知失败需手工对齐 `wire_gen.go`;
- 净化遗漏 → 白名单快照测试兜底;
- 折线图在检查间隔很长(3600s)的渠道上 24h 窗口点稀疏 → 空桶断线属预期表现,文案不承诺连续曲线。
