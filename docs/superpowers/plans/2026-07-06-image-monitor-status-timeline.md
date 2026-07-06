# 图片渠道监控状态时间线 + 用户侧公开展示 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给图片渠道自动监控加管理端状态条 + 24h/7d/30d 耗时折线图，并支持每渠道配置是否/以何名称在用户侧渠道状态页公开展示。

**Architecture:** 不建新表：`image_channel_monitors` 加 `public_visible`/`public_name` 两列；折线图/可用率全部对 `image_channel_monitor_histories` 做实时 SQL 聚合（epoch-floor 分桶）；原始历史保留 30 天，挂进 ops 每日清理。管理端在监控列表行内嵌用户侧 `MonitorTimeline` 状态条并新增详情弹窗（chart.js 双线+失败柱混合图）；用户侧 `/monitor` 页新增「生图渠道」分组，薄壳卡片组合现有监控子组件。

**Tech Stack:** Go 1.26 + Gin + Ent + 原生 SQL 聚合；Vue 3 + chart.js 4 / vue-chartjs 5；spec 见 `docs/superpowers/specs/2026-07-06-image-monitor-status-timeline-design.md`。

## Global Constraints

- 迁移只用 raw SQL（`backend/migrations/`），禁止 Ent auto-migrate；Ent schema 改动后 `go generate ./ent`。
- `go generate ./cmd/server`（wire）在本仓库已知会因 payment 重复绑定失败：**所有 wire 变更手工同步 `backend/cmd/server/wire_gen.go`**。
- 新 i18n key 必须同时进 `frontend/src/i18n/locales/zh.ts` 和 `en.ts`。
- 前端只用 pnpm；后端测试统一 `go test -tags=unit ./...` 形态。
- 用户侧 DTO 白名单红线：绝不下发内部渠道名（public_name 非空时）、endpoint、host/IP、出口 IP、错误消息、error_stage、图片 URL、代理/账号信息、密钥掩码。
- 每个 Task 结束都 commit（消息末尾带 `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`）；不 push。
- 本地 dev 栈 air 热重载后端、Vite HMR 前端；手工验证走 `http://127.0.0.1:15175`（preview）或 15174。
- 可用率口径：`(operational + degraded) / total * 100`，total=0 时可用率为 0。
- 状态枚举固定四值：`operational | degraded | failed | error`。

---

### Task 1: 公开字段贯通（迁移 → Ent → service → repo → admin DTO）

**Files:**
- Create: `backend/migrations/178_image_channel_monitor_public.sql`
- Modify: `backend/ent/schema/image_channel_monitor.go`（Fields 列表）
- Modify: `backend/internal/service/image_channel_monitor_types.go`（3 个 struct）
- Modify: `backend/internal/service/image_channel_monitor_service.go`（buildCreateMonitor / applyUpdate）
- Modify: `backend/internal/repository/image_channel_monitor_repo.go`（Create/Update builder + entToServiceImageMonitor）
- Modify: `backend/internal/handler/admin/image_channel_monitor_handler.go`（create/update request + response + 映射）
- Test: `backend/internal/service/image_channel_monitor_service_test.go`

**Interfaces:**
- Produces: `service.ImageChannelMonitor.PublicVisible bool` / `.PublicName string`；admin API JSON 字段 `public_visible` / `public_name`（Create/Update/Get/List 全通）。后续 Task 3/6 依赖这两个字段。

- [ ] **Step 1: 写迁移 SQL**

```sql
-- backend/migrations/178_image_channel_monitor_public.sql
-- 图片渠道监控用户侧公开配置:
--   public_visible 默认 false(不公开); public_name 展示名覆盖,空串回落渠道名。
ALTER TABLE image_channel_monitors
    ADD COLUMN IF NOT EXISTS public_visible BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS public_name VARCHAR(200) NOT NULL DEFAULT '';
```

- [ ] **Step 2: Ent schema 加字段并重新生成**

在 `backend/ent/schema/image_channel_monitor.go` 的 `field.Bool("enabled")` 定义之后插入：

```go
		field.Bool("public_visible").
			Default(false),
		field.String("public_name").
			Optional().
			Default("").
			MaxLen(200),
```

Run: `cd backend && go generate ./ent`
Expected: 无报错，`backend/ent/imagechannelmonitor/` 下出现 `FieldPublicVisible`/`FieldPublicName`。

本地库应用迁移（dev 栈 PG 在 127.0.0.1:5432，库名/凭据 sub2api/sub2api）：

```bash
docker exec -i sub2api-pg-dev psql -U sub2api -d sub2api -c "ALTER TABLE image_channel_monitors ADD COLUMN IF NOT EXISTS public_visible BOOLEAN NOT NULL DEFAULT FALSE, ADD COLUMN IF NOT EXISTS public_name VARCHAR(200) NOT NULL DEFAULT '';"
```

- [ ] **Step 3: service 类型贯通**

`backend/internal/service/image_channel_monitor_types.go`：

在 `ImageChannelMonitor` struct 的 `Enabled bool` 之后加：

```go
	PublicVisible       bool
	PublicName          string
```

在 `ImageChannelMonitorCreateParams` 的 `Enabled bool` 之后加：

```go
	PublicVisible   bool
	PublicName      string
```

在 `ImageChannelMonitorUpdateParams` 的 `Enabled *bool` 之后加：

```go
	PublicVisible   *bool
	PublicName      *string
```

- [ ] **Step 4: service Create/Update 映射**

`backend/internal/service/image_channel_monitor_service.go`：

在 `buildCreateMonitor` 中构造 `ImageChannelMonitor` 的地方（对照 `Enabled: p.Enabled` 一行）加：

```go
		PublicVisible: p.PublicVisible,
		PublicName:    strings.TrimSpace(p.PublicName),
```

在 `applyUpdate` 中（对照 `if p.Enabled != nil` 的处理块）加：

```go
	if p.PublicVisible != nil {
		existing.PublicVisible = *p.PublicVisible
	}
	if p.PublicName != nil {
		existing.PublicName = strings.TrimSpace(*p.PublicName)
	}
```

- [ ] **Step 5: repo 映射**

`backend/internal/repository/image_channel_monitor_repo.go`：

Create builder 的 `SetEnabled(m.Enabled)` 后加：

```go
		SetPublicVisible(m.PublicVisible).
		SetPublicName(m.PublicName).
```

Update builder 同位置加同样两行。`entToServiceImageMonitor` 返回 struct 的 `Enabled: row.Enabled,` 后加：

```go
		PublicVisible:   row.PublicVisible,
		PublicName:      row.PublicName,
```

- [ ] **Step 6: admin DTO 贯通**

`backend/internal/handler/admin/image_channel_monitor_handler.go`：

`imageChannelMonitorCreateRequest` 的 `Enabled *bool` 后加：

```go
	PublicVisible   *bool  `json:"public_visible"`
	PublicName      string `json:"public_name" binding:"omitempty,max=200"`
```

`imageChannelMonitorUpdateRequest` 的 `Enabled *bool` 后加：

```go
	PublicVisible   *bool   `json:"public_visible"`
	PublicName      *string `json:"public_name" binding:"omitempty,max=200"`
```

`imageChannelMonitorResponse` 的 `Enabled bool` 后加：

```go
	PublicVisible       bool    `json:"public_visible"`
	PublicName          string  `json:"public_name"`
```

`imageMonitorToResponse` 的 `Enabled: m.Enabled,` 后加：

```go
		PublicVisible:       m.PublicVisible,
		PublicName:          m.PublicName,
```

Create handler 构造 `ImageChannelMonitorCreateParams` 处（对照 `Enabled: enabled` 附近）加：

```go
	publicVisible := false
	if req.PublicVisible != nil {
		publicVisible = *req.PublicVisible
	}
```

并在 params 字面量加 `PublicVisible: publicVisible, PublicName: req.PublicName,`。Update handler 构造 `ImageChannelMonitorUpdateParams` 字面量加 `PublicVisible: req.PublicVisible, PublicName: req.PublicName,`。

- [ ] **Step 7: 单测（先写后跑，验证 Create 贯通与 Update 部分更新）**

`backend/internal/service/image_channel_monitor_service_test.go` 追加（仿照现有 Create/Update 测试的 stub 用法；如现有测试无 Create 级别用例，则直接测 applyUpdate 语义——用现有 stub repo 构造 service，Create 后断言字段，再 Update 只带 PublicName 断言 PublicVisible 不变）：

```go
func TestImageChannelMonitorPublicFieldsCreateAndPartialUpdate(t *testing.T) {
	repo := &imageMonitorRepoStub{}
	svc := NewImageChannelMonitorService(repo, nil, &imageMonitorProxyReaderStub{}, &imageMonitorEncryptorStub{}, nil, nil)

	created, err := svc.Create(context.Background(), ImageChannelMonitorCreateParams{
		Name:          "img-a",
		SourceType:    ImageChannelMonitorSourceCustom,
		Endpoint:      "https://api.example.com",
		APIKey:        "sk-test",
		PublicVisible: true,
		PublicName:    "  生图通道A  ",
	})
	require.NoError(t, err)
	require.True(t, created.PublicVisible)
	require.Equal(t, "生图通道A", created.PublicName)

	newName := "通道A"
	updated, err := svc.Update(context.Background(), created.ID, ImageChannelMonitorUpdateParams{
		PublicName: &newName,
	})
	require.NoError(t, err)
	require.True(t, updated.PublicVisible, "partial update must not reset public_visible")
	require.Equal(t, "通道A", updated.PublicName)
}
```

注意：先看现有 stub（`imageMonitorRepoStub`、encryptor stub 的实际名字与构造方式）再落笔，保持同套 stub；若 stub 的 Create/GetByID 需要补内存存储行为，就补上（map[int64]*ImageChannelMonitor）。

- [ ] **Step 8: 跑测试**

Run: `cd backend && go test -tags=unit ./internal/service -run TestImageChannelMonitorPublicFields -count=1`
Expected: PASS

Run: `cd backend && go build ./...`
Expected: 编译通过

- [ ] **Step 9: Commit**

```bash
git add backend/migrations/178_image_channel_monitor_public.sql backend/ent backend/internal
git commit -m "feat(image-monitor): add public_visible/public_name fields end to end"
```

---

### Task 2: 历史保留清理（激活死代码 + ops 挂接 + wire_gen 手工对齐）

**Files:**
- Modify: `backend/internal/service/image_channel_monitor_types.go`（常量）
- Modify: `backend/internal/service/image_channel_monitor_service.go`（RunDailyMaintenance）
- Modify: `backend/internal/service/ops_cleanup_service.go`（struct + New + runCleanupOnce）
- Modify: `backend/internal/service/wire.go`（ProvideOpsCleanupService）
- Modify: `backend/cmd/server/wire_gen.go`（手工对齐 NewOpsCleanupService 调用）
- Test: `backend/internal/service/image_channel_monitor_service_test.go`

**Interfaces:**
- Produces: `(*ImageChannelMonitorService).RunDailyMaintenance(ctx) error`——每日删除 30 天前历史。

- [ ] **Step 1: 常量**

`image_channel_monitor_types.go` const 块（`imageMonitorManualRunMax` 之后）加：

```go
	imageMonitorHistoryRetentionDays = 30
```

- [ ] **Step 2: 失败测试**

```go
func TestImageChannelMonitorRunDailyMaintenancePrunesOldHistory(t *testing.T) {
	repo := &imageMonitorRepoStub{}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)

	require.NoError(t, svc.RunDailyMaintenance(context.Background()))
	require.False(t, repo.deleteHistoryBefore.IsZero(), "DeleteHistoryBefore must be called")
	wantCutoff := time.Now().UTC().AddDate(0, 0, -30)
	require.WithinDuration(t, wantCutoff, repo.deleteHistoryBefore, time.Minute)
}
```

给 `imageMonitorRepoStub` 加字段 `deleteHistoryBefore time.Time`，其 `DeleteHistoryBefore` 实现记录参数并返回 `(0, nil)`。

Run: `go test -tags=unit ./internal/service -run TestImageChannelMonitorRunDailyMaintenance -count=1`
Expected: FAIL（RunDailyMaintenance undefined）

- [ ] **Step 3: 实现**

`image_channel_monitor_service.go`（persistResult 附近）：

```go
// RunDailyMaintenance 每日维护:物理删除 imageMonitorHistoryRetentionDays 天前的明细。
// 由 OpsCleanupService 的每日 cron 触发,与原生渠道监控维护同一调度/领导锁。
func (s *ImageChannelMonitorService) RunDailyMaintenance(ctx context.Context) error {
	before := time.Now().UTC().AddDate(0, 0, -imageMonitorHistoryRetentionDays)
	deleted, err := s.repo.DeleteHistoryBefore(ctx, before)
	if err != nil {
		return fmt.Errorf("delete image monitor history before %s: %w", before.Format(time.RFC3339), err)
	}
	if deleted > 0 {
		slog.Info("image_channel_monitor: history cleanup",
			"deleted_rows", deleted, "before", before.Format(time.RFC3339))
	}
	return nil
}
```

- [ ] **Step 4: ops 挂接**

`ops_cleanup_service.go`：struct 加字段 `imageChannelMonitorSvc *ImageChannelMonitorService`（`channelMonitorSvc` 旁）；`NewOpsCleanupService` 加同名参数并赋值；`runCleanupOnce` 中 channel monitor 维护块之后加：

```go
	if s.imageChannelMonitorSvc != nil {
		if err := s.imageChannelMonitorSvc.RunDailyMaintenance(ctx); err != nil {
			logger.LegacyPrintf("service.ops_cleanup", "[OpsCleanup] image channel monitor maintenance failed: %v", err)
		}
	}
```

`wire.go` 的 `ProvideOpsCleanupService` 加参数 `imageChannelMonitorSvc *ImageChannelMonitorService` 并传给 `NewOpsCleanupService`。

- [ ] **Step 5: wire_gen 手工对齐**

在 `backend/cmd/server/wire_gen.go` 搜 `ProvideOpsCleanupService(`，在调用处追加实参（该文件中 ImageChannelMonitorService 变量已存在，搜 `NewImageChannelMonitorService` 找变量名，注意声明顺序必须在使用之前；若 ops 提供先于 image service 构造，把 image service 构造上移到 ops 之前）。

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 6: 跑测试 + 修 ops 现有测试**

Run: `go test -tags=unit ./internal/service -run "TestImageChannelMonitorRunDailyMaintenance|OpsCleanup" -count=1`
Expected: PASS。若 ops_cleanup 已有测试直接调 `NewOpsCleanupService`，为其补 `nil` 实参。

- [ ] **Step 7: Commit**

```bash
git add backend/internal backend/cmd/server/wire_gen.go
git commit -m "feat(image-monitor): prune monitor history after 30 days via ops daily cleanup"
```

---

### Task 3: repo 聚合方法（分桶 / 窗口统计 / 批量近况 / 公开列表 / 批量可用率）

**Files:**
- Modify: `backend/internal/service/image_channel_monitor_types.go`（interface + 新类型）
- Modify: `backend/internal/repository/image_channel_monitor_repo.go`（5 个方法）
- Modify: `backend/internal/service/image_channel_monitor_service_test.go`（stub 补方法）

**Interfaces:**
- Produces（service 包类型，后续 Task 4/5/6 直接消费）:

```go
type ImageMonitorTimelineBucket struct {
	BucketStart        time.Time
	Total              int
	Operational        int
	Degraded           int
	Failed             int
	Error              int
	AvgAPITotalMs      *int
	MaxAPITotalMs      *int
	AvgImageDownloadMs *int
}

type ImageMonitorWindowStats struct {
	Total              int
	OK                 int
	Availability       float64
	AvgAPITotalMs      *int
	MaxAPITotalMs      *int
	AvgImageDownloadMs *int
}

type ImageMonitorTimelinePoint struct {
	Status          string
	APITotalMs      *int
	ImageDownloadMs *int
	CheckedAt       time.Time
}

type ImageMonitorAvailability struct {
	D7  float64
	D15 float64
	D30 float64
}
```

- Repository interface 追加:

```go
	AggregateTimeline(ctx context.Context, monitorID int64, bucketSeconds int, since time.Time) ([]*ImageMonitorTimelineBucket, error)
	ComputeWindowStats(ctx context.Context, monitorID int64, since time.Time) (*ImageMonitorWindowStats, error)
	ListRecentHistoryForMonitors(ctx context.Context, ids []int64, perMonitorLimit int) (map[int64][]*ImageMonitorTimelinePoint, error)
	ListPublicVisible(ctx context.Context) ([]*ImageChannelMonitor, error)
	ComputeAvailabilityForMonitors(ctx context.Context, ids []int64) (map[int64]*ImageMonitorAvailability, error)
```

- [ ] **Step 1: service 类型 + interface 扩展**（代码见上，interface 追加放 `DeleteHistoryBefore` 之后）

- [ ] **Step 2: 更新测试 stub 使编译通过**

`imageMonitorRepoStub` 补五个方法（返回可注入字段，默认空值）：

```go
func (r *imageMonitorRepoStub) AggregateTimeline(_ context.Context, _ int64, bucketSeconds int, since time.Time) ([]*ImageMonitorTimelineBucket, error) {
	r.lastBucketSeconds = bucketSeconds
	r.lastSince = since
	return r.timelineBuckets, r.timelineErr
}
func (r *imageMonitorRepoStub) ComputeWindowStats(_ context.Context, _ int64, since time.Time) (*ImageMonitorWindowStats, error) {
	r.lastSince = since
	if r.windowStats == nil {
		return &ImageMonitorWindowStats{}, r.windowStatsErr
	}
	return r.windowStats, r.windowStatsErr
}
func (r *imageMonitorRepoStub) ListRecentHistoryForMonitors(context.Context, []int64, int) (map[int64][]*ImageMonitorTimelinePoint, error) {
	return r.recentHistory, nil
}
func (r *imageMonitorRepoStub) ListPublicVisible(context.Context) ([]*ImageChannelMonitor, error) {
	return r.publicMonitors, nil
}
func (r *imageMonitorRepoStub) ComputeAvailabilityForMonitors(context.Context, []int64) (map[int64]*ImageMonitorAvailability, error) {
	return r.availability, nil
}
```

并给 stub struct 加对应字段：`timelineBuckets []*ImageMonitorTimelineBucket`、`timelineErr error`、`windowStats *ImageMonitorWindowStats`、`windowStatsErr error`、`recentHistory map[int64][]*ImageMonitorTimelinePoint`、`publicMonitors []*ImageChannelMonitor`、`availability map[int64]*ImageMonitorAvailability`、`lastBucketSeconds int`、`lastSince time.Time`、`monitors map[int64]*ImageChannelMonitor`。

**GetByID 兼容改造**：现有 stub 的 `GetByID` 有既定行为（供旧测试使用），改成「`r.monitors` 非 nil 时优先查 map（缺失返回 `ErrImageChannelMonitorNotFound`），否则维持原行为」，不得破坏既有用例。

Run: `go build ./... && go test -tags=unit ./internal/service -count=1`（编译红线与既有用例全绿为止）。

- [ ] **Step 3: repo 实现**

`image_channel_monitor_repo.go` 追加（`DeleteHistoryBefore` 之后）。分桶用 epoch-floor（同时适配 10min/2h/1d）：

```go
func (r *imageChannelMonitorRepository) AggregateTimeline(
	ctx context.Context,
	monitorID int64,
	bucketSeconds int,
	since time.Time,
) ([]*service.ImageMonitorTimelineBucket, error) {
	if bucketSeconds <= 0 {
		return nil, fmt.Errorf("bucketSeconds must be positive")
	}
	const q = `
		SELECT
		    to_timestamp(floor(extract(epoch FROM checked_at) / $2) * $2) AS bucket_start,
		    COUNT(*) AS total,
		    COUNT(*) FILTER (WHERE status = 'operational') AS operational,
		    COUNT(*) FILTER (WHERE status = 'degraded') AS degraded,
		    COUNT(*) FILTER (WHERE status = 'failed') AS failed,
		    COUNT(*) FILTER (WHERE status = 'error') AS error,
		    AVG(api_total_ms)::float8 AS avg_api_total_ms,
		    MAX(api_total_ms) AS max_api_total_ms,
		    AVG(image_download_ms)::float8 AS avg_image_download_ms
		FROM image_channel_monitor_histories
		WHERE monitor_id = $1 AND checked_at >= $3
		GROUP BY 1
		ORDER BY 1
	`
	rows, err := r.db.QueryContext(ctx, q, monitorID, bucketSeconds, since)
	if err != nil {
		return nil, fmt.Errorf("aggregate image monitor timeline: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.ImageMonitorTimelineBucket, 0, 160)
	for rows.Next() {
		b := &service.ImageMonitorTimelineBucket{}
		var avgAPI, avgDL sql.NullFloat64
		var maxAPI sql.NullInt64
		if err := rows.Scan(&b.BucketStart, &b.Total, &b.Operational, &b.Degraded, &b.Failed, &b.Error, &avgAPI, &maxAPI, &avgDL); err != nil {
			return nil, fmt.Errorf("scan timeline bucket: %w", err)
		}
		b.AvgAPITotalMs = nullFloatToIntPtr(avgAPI)
		b.AvgImageDownloadMs = nullFloatToIntPtr(avgDL)
		if maxAPI.Valid {
			v := int(maxAPI.Int64)
			b.MaxAPITotalMs = &v
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func nullFloatToIntPtr(v sql.NullFloat64) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Float64 + 0.5)
	return &i
}

func (r *imageChannelMonitorRepository) ComputeWindowStats(
	ctx context.Context,
	monitorID int64,
	since time.Time,
) (*service.ImageMonitorWindowStats, error) {
	const q = `
		SELECT
		    COUNT(*) AS total,
		    COUNT(*) FILTER (WHERE status IN ('operational','degraded')) AS ok,
		    AVG(api_total_ms)::float8 AS avg_api_total_ms,
		    MAX(api_total_ms) AS max_api_total_ms,
		    AVG(image_download_ms)::float8 AS avg_image_download_ms
		FROM image_channel_monitor_histories
		WHERE monitor_id = $1 AND checked_at >= $2
	`
	s := &service.ImageMonitorWindowStats{}
	var avgAPI, avgDL sql.NullFloat64
	var maxAPI sql.NullInt64
	if err := r.db.QueryRowContext(ctx, q, monitorID, since).
		Scan(&s.Total, &s.OK, &avgAPI, &maxAPI, &avgDL); err != nil {
		return nil, fmt.Errorf("compute image monitor window stats: %w", err)
	}
	s.AvgAPITotalMs = nullFloatToIntPtr(avgAPI)
	s.AvgImageDownloadMs = nullFloatToIntPtr(avgDL)
	if maxAPI.Valid {
		v := int(maxAPI.Int64)
		s.MaxAPITotalMs = &v
	}
	if s.Total > 0 {
		s.Availability = float64(s.OK) / float64(s.Total) * 100
	}
	return s, nil
}

// ListRecentHistoryForMonitors 批量取每个 monitor 最近 N 条(最新在前),消 N+1。
// 结构对齐 channel_monitor_repo.go 的同名方法,图片监控无 model 维度故白名单只有 id。
func (r *imageChannelMonitorRepository) ListRecentHistoryForMonitors(
	ctx context.Context,
	ids []int64,
	perMonitorLimit int,
) (map[int64][]*service.ImageMonitorTimelinePoint, error) {
	out := make(map[int64][]*service.ImageMonitorTimelinePoint, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	if perMonitorLimit <= 0 || perMonitorLimit > 200 {
		perMonitorLimit = 60
	}
	const q = `
		WITH ranked AS (
		    SELECT h.monitor_id, h.status, h.api_total_ms, h.image_download_ms, h.checked_at,
		           ROW_NUMBER() OVER (PARTITION BY h.monitor_id ORDER BY h.checked_at DESC) AS rn
		    FROM image_channel_monitor_histories h
		    WHERE h.monitor_id = ANY($1::bigint[])
		)
		SELECT monitor_id, status, api_total_ms, image_download_ms, checked_at
		FROM ranked
		WHERE rn <= $2
		ORDER BY monitor_id, checked_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, pq.Array(ids), perMonitorLimit)
	if err != nil {
		return nil, fmt.Errorf("query image monitor recent history batch: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var monitorID int64
		p := &service.ImageMonitorTimelinePoint{}
		var api, dl sql.NullInt64
		if err := rows.Scan(&monitorID, &p.Status, &api, &dl, &p.CheckedAt); err != nil {
			return nil, fmt.Errorf("scan image monitor recent history: %w", err)
		}
		if api.Valid {
			v := int(api.Int64)
			p.APITotalMs = &v
		}
		if dl.Valid {
			v := int(dl.Int64)
			p.ImageDownloadMs = &v
		}
		out[monitorID] = append(out[monitorID], p)
	}
	return out, rows.Err()
}

func (r *imageChannelMonitorRepository) ListPublicVisible(ctx context.Context) ([]*service.ImageChannelMonitor, error) {
	rows, err := r.client.ImageChannelMonitor.Query().
		Where(imagechannelmonitor.PublicVisibleEQ(true)).
		Order(dbent.Asc(imagechannelmonitor.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list public image channel monitors: %w", err)
	}
	out := make([]*service.ImageChannelMonitor, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToServiceImageMonitor(row))
	}
	return out, nil
}

// ComputeAvailabilityForMonitors 一条 SQL 算全部 monitor 的 7/15/30 天可用率。
func (r *imageChannelMonitorRepository) ComputeAvailabilityForMonitors(
	ctx context.Context,
	ids []int64,
) (map[int64]*service.ImageMonitorAvailability, error) {
	out := make(map[int64]*service.ImageMonitorAvailability, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	now := time.Now().UTC()
	d7 := now.AddDate(0, 0, -7)
	d15 := now.AddDate(0, 0, -15)
	d30 := now.AddDate(0, 0, -30)
	const q = `
		SELECT monitor_id,
		    COUNT(*) FILTER (WHERE checked_at >= $2) AS total_7d,
		    COUNT(*) FILTER (WHERE checked_at >= $2 AND status IN ('operational','degraded')) AS ok_7d,
		    COUNT(*) FILTER (WHERE checked_at >= $3) AS total_15d,
		    COUNT(*) FILTER (WHERE checked_at >= $3 AND status IN ('operational','degraded')) AS ok_15d,
		    COUNT(*) AS total_30d,
		    COUNT(*) FILTER (WHERE status IN ('operational','degraded')) AS ok_30d
		FROM image_channel_monitor_histories
		WHERE monitor_id = ANY($1::bigint[]) AND checked_at >= $4
		GROUP BY monitor_id
	`
	rows, err := r.db.QueryContext(ctx, q, pq.Array(ids), d7, d15, d30)
	if err != nil {
		return nil, fmt.Errorf("compute image monitor availability batch: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var t7, ok7, t15, ok15, t30, ok30 int
		if err := rows.Scan(&id, &t7, &ok7, &t15, &ok15, &t30, &ok30); err != nil {
			return nil, fmt.Errorf("scan image monitor availability: %w", err)
		}
		out[id] = &service.ImageMonitorAvailability{
			D7:  pct(ok7, t7),
			D15: pct(ok15, t15),
			D30: pct(ok30, t30),
		}
	}
	return out, rows.Err()
}

func pct(ok, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(ok) / float64(total) * 100
}
```

import 需补 `"github.com/lib/pq"`（channel_monitor_repo.go 已用同款，模块里有）。若 `pct`/`nullFloatToIntPtr` 与包内既有函数重名，先搜再改名。

- [ ] **Step 4: 编译 + 全量单测**

Run: `go build ./... && go test -tags=unit ./internal/... -count=1`
Expected: PASS（SQL 正确性在 Task 10 手工验证——本仓库 repo 层无既有测试形态）

- [ ] **Step 5: Commit**

```bash
git add backend/internal
git commit -m "feat(image-monitor): repo aggregation for timeline buckets, window stats and public views"
```

---

### Task 4: service 视图/时间线方法 + 单测

**Files:**
- Modify: `backend/internal/service/image_channel_monitor_types.go`（视图类型 + window 常量 + 错误）
- Modify: `backend/internal/service/image_channel_monitor_service.go`（4 个方法）
- Test: `backend/internal/service/image_channel_monitor_service_test.go`

**Interfaces:**
- Produces（Task 5/6 的 handler 消费）:

```go
type ImageMonitorPublicView struct {
	ID               int64
	Name             string // public_name 非空用之,否则渠道名
	Model            string
	LatestStatus     string // 无历史时 "empty"
	LatestAPIMs      *int
	LatestDownloadMs *int
	Availability     ImageMonitorAvailability
	Timeline         []*ImageMonitorTimelinePoint
}

type ImageMonitorPublicWindowStat struct {
	WindowDays    int
	Availability  float64
	AvgAPITotalMs *int
}

type ImageMonitorPublicDetail struct {
	ID      int64
	Name    string
	Model   string
	Windows []ImageMonitorPublicWindowStat
}

type ImageMonitorAdminTimeline struct {
	Window  string
	Summary *ImageMonitorWindowStats
	Buckets []*ImageMonitorTimelineBucket
}
```

- 方法:
  - `ListPublicView(ctx) ([]*ImageMonitorPublicView, error)`
  - `GetPublicDetail(ctx, id) (*ImageMonitorPublicDetail, error)`（非公开渠道返回 `ErrImageChannelMonitorNotFound`）
  - `GetAdminTimeline(ctx, id, window string) (*ImageMonitorAdminTimeline, error)`（window ∉ {24h,7d,30d} → `ErrImageChannelMonitorInvalidWindow`）
  - `TimelinesForMonitors(ctx, ids []int64) (map[int64][]*ImageMonitorTimelinePoint, error)`
  - `AvailabilityForMonitors(ctx, ids []int64) (map[int64]*ImageMonitorAvailability, error)`

- [ ] **Step 1: 失败测试（覆盖:窗口映射、公开过滤+掩名、latest 提取、无历史 empty、detail 404）**

```go
func TestImageChannelMonitorGetAdminTimelineWindows(t *testing.T) {
	repo := &imageMonitorRepoStub{
		monitors:    map[int64]*ImageChannelMonitor{7: {ID: 7, Name: "img"}},
		windowStats: &ImageMonitorWindowStats{Total: 10, OK: 9, Availability: 90},
	}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)

	tl, err := svc.GetAdminTimeline(context.Background(), 7, "24h")
	require.NoError(t, err)
	require.Equal(t, "24h", tl.Window)
	require.Equal(t, 600, repo.lastBucketSeconds)
	require.WithinDuration(t, time.Now().UTC().Add(-24*time.Hour), repo.lastSince, time.Minute)

	_, err = svc.GetAdminTimeline(context.Background(), 7, "7d")
	require.NoError(t, err)
	require.Equal(t, 7200, repo.lastBucketSeconds)

	_, err = svc.GetAdminTimeline(context.Background(), 7, "30d")
	require.NoError(t, err)
	require.Equal(t, 86400, repo.lastBucketSeconds)

	_, err = svc.GetAdminTimeline(context.Background(), 7, "90d")
	require.ErrorIs(t, err, ErrImageChannelMonitorInvalidWindow)
}

func TestImageChannelMonitorListPublicViewMasksNameAndExtractsLatest(t *testing.T) {
	api1 := 18000
	dl1 := 2200
	repo := &imageMonitorRepoStub{
		publicMonitors: []*ImageChannelMonitor{
			{ID: 1, Name: "内部-adobe中转", PublicName: "生图通道A", PublicVisible: true, Model: "gpt-image-1"},
			{ID: 2, Name: "直连", PublicName: "", PublicVisible: true, Model: "gpt-image-1"},
		},
		recentHistory: map[int64][]*ImageMonitorTimelinePoint{
			1: {{Status: "operational", APITotalMs: &api1, ImageDownloadMs: &dl1, CheckedAt: time.Now()}},
		},
		availability: map[int64]*ImageMonitorAvailability{1: {D7: 99.5, D15: 98, D30: 97}},
	}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)

	views, err := svc.ListPublicView(context.Background())
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "生图通道A", views[0].Name)
	require.Equal(t, "operational", views[0].LatestStatus)
	require.Equal(t, 18000, *views[0].LatestAPIMs)
	require.Equal(t, 2200, *views[0].LatestDownloadMs)
	require.InDelta(t, 99.5, views[0].Availability.D7, 0.001)
	require.Equal(t, "直连", views[1].Name, "empty public_name falls back to monitor name")
	require.Equal(t, "empty", views[1].LatestStatus, "no history -> empty status")
	require.Zero(t, views[1].Availability.D7)
}

func TestImageChannelMonitorGetPublicDetailHidesPrivateMonitor(t *testing.T) {
	repo := &imageMonitorRepoStub{
		monitors: map[int64]*ImageChannelMonitor{3: {ID: 3, Name: "private", PublicVisible: false}},
	}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)
	_, err := svc.GetPublicDetail(context.Background(), 3)
	require.ErrorIs(t, err, ErrImageChannelMonitorNotFound)
}
```

stub 需要:`monitors map[int64]*ImageChannelMonitor`(GetByID 从中取,缺省 ErrImageChannelMonitorNotFound)、`lastBucketSeconds int`、`lastSince time.Time`(AggregateTimeline/ComputeWindowStats 记录实参)。对照现有 stub 结构改。

Run: `go test -tags=unit ./internal/service -run "TestImageChannelMonitorGetAdminTimeline|TestImageChannelMonitorListPublicView|TestImageChannelMonitorGetPublicDetail" -count=1`
Expected: FAIL（方法未定义）

- [ ] **Step 2: 实现**

`image_channel_monitor_types.go` 加视图类型（见 Interfaces）+ 错误：

```go
	ErrImageChannelMonitorInvalidWindow = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_WINDOW", "window must be 24h, 7d or 30d",
	)
```

`image_channel_monitor_service.go` 追加：

```go
const imageMonitorTimelinePointLimit = 60

var imageMonitorTimelineWindows = map[string]struct {
	bucketSeconds int
	span          time.Duration
}{
	"24h": {bucketSeconds: 600, span: 24 * time.Hour},
	"7d":  {bucketSeconds: 7200, span: 7 * 24 * time.Hour},
	"30d": {bucketSeconds: 86400, span: 30 * 24 * time.Hour},
}

func (s *ImageChannelMonitorService) GetAdminTimeline(
	ctx context.Context,
	id int64,
	window string,
) (*ImageMonitorAdminTimeline, error) {
	w, ok := imageMonitorTimelineWindows[window]
	if !ok {
		return nil, ErrImageChannelMonitorInvalidWindow
	}
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}
	since := time.Now().UTC().Add(-w.span)
	summary, err := s.repo.ComputeWindowStats(ctx, id, since)
	if err != nil {
		return nil, fmt.Errorf("compute window stats: %w", err)
	}
	buckets, err := s.repo.AggregateTimeline(ctx, id, w.bucketSeconds, since)
	if err != nil {
		return nil, fmt.Errorf("aggregate timeline: %w", err)
	}
	return &ImageMonitorAdminTimeline{Window: window, Summary: summary, Buckets: buckets}, nil
}

func (s *ImageChannelMonitorService) TimelinesForMonitors(
	ctx context.Context,
	ids []int64,
) (map[int64][]*ImageMonitorTimelinePoint, error) {
	return s.repo.ListRecentHistoryForMonitors(ctx, ids, imageMonitorTimelinePointLimit)
}

func (s *ImageChannelMonitorService) AvailabilityForMonitors(
	ctx context.Context,
	ids []int64,
) (map[int64]*ImageMonitorAvailability, error) {
	return s.repo.ComputeAvailabilityForMonitors(ctx, ids)
}

func imageMonitorPublicName(m *ImageChannelMonitor) string {
	if name := strings.TrimSpace(m.PublicName); name != "" {
		return name
	}
	return m.Name
}

func (s *ImageChannelMonitorService) ListPublicView(ctx context.Context) ([]*ImageMonitorPublicView, error) {
	monitors, err := s.repo.ListPublicVisible(ctx)
	if err != nil {
		return nil, fmt.Errorf("list public image monitors: %w", err)
	}
	if len(monitors) == 0 {
		return []*ImageMonitorPublicView{}, nil
	}
	ids := make([]int64, 0, len(monitors))
	for _, m := range monitors {
		ids = append(ids, m.ID)
	}
	timelines, err := s.repo.ListRecentHistoryForMonitors(ctx, ids, imageMonitorTimelinePointLimit)
	if err != nil {
		return nil, fmt.Errorf("load public timelines: %w", err)
	}
	availability, err := s.repo.ComputeAvailabilityForMonitors(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load public availability: %w", err)
	}
	out := make([]*ImageMonitorPublicView, 0, len(monitors))
	for _, m := range monitors {
		view := &ImageMonitorPublicView{
			ID:           m.ID,
			Name:         imageMonitorPublicName(m),
			Model:        m.Model,
			LatestStatus: "empty",
			Timeline:     timelines[m.ID],
		}
		if points := timelines[m.ID]; len(points) > 0 {
			view.LatestStatus = points[0].Status
			view.LatestAPIMs = points[0].APITotalMs
			view.LatestDownloadMs = points[0].ImageDownloadMs
		}
		if a := availability[m.ID]; a != nil {
			view.Availability = *a
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *ImageChannelMonitorService) GetPublicDetail(ctx context.Context, id int64) (*ImageMonitorPublicDetail, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !m.PublicVisible {
		return nil, ErrImageChannelMonitorNotFound
	}
	detail := &ImageMonitorPublicDetail{
		ID:    m.ID,
		Name:  imageMonitorPublicName(m),
		Model: m.Model,
	}
	now := time.Now().UTC()
	for _, days := range []int{7, 15, 30} {
		stats, err := s.repo.ComputeWindowStats(ctx, id, now.AddDate(0, 0, -days))
		if err != nil {
			return nil, fmt.Errorf("compute %dd stats: %w", days, err)
		}
		detail.Windows = append(detail.Windows, ImageMonitorPublicWindowStat{
			WindowDays:    days,
			Availability:  stats.Availability,
			AvgAPITotalMs: stats.AvgAPITotalMs,
		})
	}
	return detail, nil
}
```

- [ ] **Step 3: 跑测试**

Run: `go test -tags=unit ./internal/service -run TestImageChannelMonitor -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/service
git commit -m "feat(image-monitor): service-level public views and admin timeline aggregation"
```

---

### Task 5: admin API（timeline 端点 + List 内嵌 timeline/可用率 + 路由）

**Files:**
- Modify: `backend/internal/handler/admin/image_channel_monitor_handler.go`
- Modify: `backend/internal/server/routes/admin.go`（registerImageChannelMonitorRoutes）
- Test: `backend/internal/handler/admin/image_channel_monitor_handler_test.go`（新建，若同包已有 handler 测试形态则并入）

**Interfaces:**
- Produces（前端 Task 7/8 消费）:
  - `GET /admin/image-channel-monitors/:id/timeline?window=24h|7d|30d` → `{window, summary:{total,ok,failures,availability,avg_api_total_ms,max_api_total_ms,avg_image_download_ms}, buckets:[{bucket_start,total,operational,degraded,failed,error,avg_api_total_ms,max_api_total_ms,avg_image_download_ms}]}`
  - List 每项追加 `"timeline": [{status,latency_ms,image_download_ms,checked_at}]` 与 `"availability_7d": number`

- [ ] **Step 1: DTO + 转换**

handler 文件追加：

```go
type imageMonitorTimelinePointResponse struct {
	Status          string `json:"status"`
	LatencyMs       *int   `json:"latency_ms"`
	ImageDownloadMs *int   `json:"image_download_ms"`
	CheckedAt       string `json:"checked_at"`
}

type imageMonitorTimelineBucketResponse struct {
	BucketStart        string `json:"bucket_start"`
	Total              int    `json:"total"`
	Operational        int    `json:"operational"`
	Degraded           int    `json:"degraded"`
	Failed             int    `json:"failed"`
	Error              int    `json:"error"`
	AvgAPITotalMs      *int   `json:"avg_api_total_ms"`
	MaxAPITotalMs      *int   `json:"max_api_total_ms"`
	AvgImageDownloadMs *int   `json:"avg_image_download_ms"`
}

type imageMonitorTimelineSummaryResponse struct {
	Total              int     `json:"total"`
	OK                 int     `json:"ok"`
	Failures           int     `json:"failures"`
	Availability       float64 `json:"availability"`
	AvgAPITotalMs      *int    `json:"avg_api_total_ms"`
	MaxAPITotalMs      *int    `json:"max_api_total_ms"`
	AvgImageDownloadMs *int    `json:"avg_image_download_ms"`
}

type imageMonitorTimelineResponse struct {
	Window  string                               `json:"window"`
	Summary imageMonitorTimelineSummaryResponse  `json:"summary"`
	Buckets []imageMonitorTimelineBucketResponse `json:"buckets"`
}

type imageChannelMonitorListItemResponse struct {
	*imageChannelMonitorResponse
	Availability7d float64                             `json:"availability_7d"`
	Timeline       []imageMonitorTimelinePointResponse `json:"timeline"`
}

func imageMonitorTimelinePointsToResponse(points []*service.ImageMonitorTimelinePoint) []imageMonitorTimelinePointResponse {
	out := make([]imageMonitorTimelinePointResponse, 0, len(points))
	for _, p := range points {
		out = append(out, imageMonitorTimelinePointResponse{
			Status:          p.Status,
			LatencyMs:       p.APITotalMs,
			ImageDownloadMs: p.ImageDownloadMs,
			CheckedAt:       p.CheckedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}
```

- [ ] **Step 2: List 扩展**

替换 `List` 尾部的响应组装：

```go
	ids := make([]int64, 0, len(items))
	for _, m := range items {
		ids = append(ids, m.ID)
	}
	timelines, err := h.monitorService.TimelinesForMonitors(c.Request.Context(), ids)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	availability, err := h.monitorService.AvailabilityForMonitors(c.Request.Context(), ids)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]*imageChannelMonitorListItemResponse, 0, len(items))
	for _, m := range items {
		item := &imageChannelMonitorListItemResponse{
			imageChannelMonitorResponse: imageMonitorToResponse(m),
			Timeline:                    imageMonitorTimelinePointsToResponse(timelines[m.ID]),
		}
		if a := availability[m.ID]; a != nil {
			item.Availability7d = a.D7
		}
		out = append(out, item)
	}
	response.Paginated(c, out, total, page, pageSize)
```

- [ ] **Step 3: Timeline handler + 路由**

```go
// Timeline GET /admin/image-channel-monitors/:id/timeline?window=24h|7d|30d
func (h *ImageChannelMonitorHandler) Timeline(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	window := c.DefaultQuery("window", "24h")
	tl, err := h.monitorService.GetAdminTimeline(c.Request.Context(), id, window)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	buckets := make([]imageMonitorTimelineBucketResponse, 0, len(tl.Buckets))
	for _, b := range tl.Buckets {
		buckets = append(buckets, imageMonitorTimelineBucketResponse{
			BucketStart:        b.BucketStart.UTC().Format(time.RFC3339),
			Total:              b.Total,
			Operational:        b.Operational,
			Degraded:           b.Degraded,
			Failed:             b.Failed,
			Error:              b.Error,
			AvgAPITotalMs:      b.AvgAPITotalMs,
			MaxAPITotalMs:      b.MaxAPITotalMs,
			AvgImageDownloadMs: b.AvgImageDownloadMs,
		})
	}
	response.Success(c, imageMonitorTimelineResponse{
		Window: tl.Window,
		Summary: imageMonitorTimelineSummaryResponse{
			Total:              tl.Summary.Total,
			OK:                 tl.Summary.OK,
			Failures:           tl.Summary.Total - tl.Summary.OK,
			Availability:       tl.Summary.Availability,
			AvgAPITotalMs:      tl.Summary.AvgAPITotalMs,
			MaxAPITotalMs:      tl.Summary.MaxAPITotalMs,
			AvgImageDownloadMs: tl.Summary.AvgImageDownloadMs,
		},
		Buckets: buckets,
	})
}
```

`backend/internal/server/routes/admin.go` `registerImageChannelMonitorRoutes` 内 `monitors.GET("/:id/history", ...)` 之后加：

```go
		monitors.GET("/:id/timeline", h.Admin.ImageChannelMonitor.Timeline)
```

- [ ] **Step 4: 编译 + 跑相关测试 + Commit**

Run: `go build ./... && go test -tags=unit ./internal/... -count=1`
Expected: PASS

```bash
git add backend/internal
git commit -m "feat(image-monitor): admin timeline endpoint and list-embedded status strips"
```

---

### Task 6: 用户侧 API（handler + Handlers 容器 + wire + 路由 + 净化白名单测试）

**Files:**
- Create: `backend/internal/handler/image_channel_monitor_user_handler.go`
- Create: `backend/internal/handler/image_channel_monitor_user_handler_test.go`
- Modify: `backend/internal/handler/handler.go`（Handlers struct）
- Modify: `backend/internal/handler/wire.go`（provider set + 构造 + 字面量）
- Modify: `backend/cmd/server/wire_gen.go`（手工对齐）
- Modify: `backend/internal/server/routes/user.go`

**Interfaces:**
- Consumes: Task 4 的 `ListPublicView` / `GetPublicDetail`；门禁 `settingService.GetChannelMonitorRuntime(ctx).Enabled`（与 `ChannelMonitorUserHandler.featureEnabled` 同款）。
- Produces:
  - `GET /api/v1/image-channel-monitors` → `{items:[{id,name,model,latest_status,latest_api_ms,latest_download_ms,availability_7d,availability_15d,availability_30d,timeline:[{status,latency_ms,checked_at}]}]}`
  - `GET /api/v1/image-channel-monitors/:id/status` → `{id,name,model,windows:[{window_days,availability,avg_api_total_ms}]}`

- [ ] **Step 1: 白名单快照失败测试**

`image_channel_monitor_user_handler_test.go`（package handler）：

```go
package handler

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 用户侧图片监控 DTO 是安全白名单:新增字段必须显式加入本清单并通过净化评审。
func TestImageMonitorPublicListItemFieldWhitelist(t *testing.T) {
	api := 18000
	dl := 2100
	item := imageMonitorPublicViewToItem(&service.ImageMonitorPublicView{
		ID: 1, Name: "生图通道A", Model: "gpt-image-1",
		LatestStatus: "operational", LatestAPIMs: &api, LatestDownloadMs: &dl,
		Availability: service.ImageMonitorAvailability{D7: 99, D15: 98, D30: 97},
		Timeline: []*service.ImageMonitorTimelinePoint{
			{Status: "operational", APITotalMs: &api, ImageDownloadMs: &dl, CheckedAt: time.Now()},
		},
	})
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	require.Equal(t, []string{
		"availability_15d", "availability_30d", "availability_7d",
		"id", "latest_api_ms", "latest_download_ms", "latest_status",
		"model", "name", "timeline",
	}, keys)

	var tl []map[string]any
	tlRaw, err := json.Marshal(item.Timeline)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(tlRaw, &tl))
	pointKeys := make([]string, 0, len(tl[0]))
	for k := range tl[0] {
		pointKeys = append(pointKeys, k)
	}
	sort.Strings(pointKeys)
	require.Equal(t, []string{"checked_at", "latency_ms", "status"}, pointKeys)
}
```

Run: `go test -tags=unit ./internal/handler -run TestImageMonitorPublicListItemFieldWhitelist -count=1`
Expected: FAIL（类型未定义）

- [ ] **Step 2: handler 实现（镜像 channel_monitor_user_handler.go）**

`backend/internal/handler/image_channel_monitor_user_handler.go`：

```go
package handler

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ImageChannelMonitorUserHandler 图片渠道监控用户只读 handler。
// 只暴露 public_visible=true 的渠道,DTO 为安全白名单(见同名 _test.go)。
type ImageChannelMonitorUserHandler struct {
	monitorService *service.ImageChannelMonitorService
	settingService *service.SettingService
}

func NewImageChannelMonitorUserHandler(
	monitorService *service.ImageChannelMonitorService,
	settingService *service.SettingService,
) *ImageChannelMonitorUserHandler {
	return &ImageChannelMonitorUserHandler{
		monitorService: monitorService,
		settingService: settingService,
	}
}

func (h *ImageChannelMonitorUserHandler) featureEnabled(c *gin.Context) bool {
	if h.settingService == nil {
		return true
	}
	return h.settingService.GetChannelMonitorRuntime(c.Request.Context()).Enabled
}

type imageMonitorPublicTimelinePoint struct {
	Status    string `json:"status"`
	LatencyMs *int   `json:"latency_ms"`
	CheckedAt string `json:"checked_at"`
}

type imageMonitorPublicListItem struct {
	ID               int64                             `json:"id"`
	Name             string                            `json:"name"`
	Model            string                            `json:"model"`
	LatestStatus     string                            `json:"latest_status"`
	LatestAPIMs      *int                              `json:"latest_api_ms"`
	LatestDownloadMs *int                              `json:"latest_download_ms"`
	Availability7d   float64                           `json:"availability_7d"`
	Availability15d  float64                           `json:"availability_15d"`
	Availability30d  float64                           `json:"availability_30d"`
	Timeline         []imageMonitorPublicTimelinePoint `json:"timeline"`
}

type imageMonitorPublicWindowStat struct {
	WindowDays    int     `json:"window_days"`
	Availability  float64 `json:"availability"`
	AvgAPITotalMs *int    `json:"avg_api_total_ms"`
}

type imageMonitorPublicDetailResponse struct {
	ID      int64                          `json:"id"`
	Name    string                         `json:"name"`
	Model   string                         `json:"model"`
	Windows []imageMonitorPublicWindowStat `json:"windows"`
}

func imageMonitorPublicViewToItem(v *service.ImageMonitorPublicView) imageMonitorPublicListItem {
	timeline := make([]imageMonitorPublicTimelinePoint, 0, len(v.Timeline))
	for _, p := range v.Timeline {
		timeline = append(timeline, imageMonitorPublicTimelinePoint{
			Status:    p.Status,
			LatencyMs: p.APITotalMs,
			CheckedAt: p.CheckedAt.UTC().Format(time.RFC3339),
		})
	}
	return imageMonitorPublicListItem{
		ID:               v.ID,
		Name:             v.Name,
		Model:            v.Model,
		LatestStatus:     v.LatestStatus,
		LatestAPIMs:      v.LatestAPIMs,
		LatestDownloadMs: v.LatestDownloadMs,
		Availability7d:   v.Availability.D7,
		Availability15d:  v.Availability.D15,
		Availability30d:  v.Availability.D30,
		Timeline:         timeline,
	}
}

// List GET /api/v1/image-channel-monitors
func (h *ImageChannelMonitorUserHandler) List(c *gin.Context) {
	if !h.featureEnabled(c) {
		response.Success(c, gin.H{"items": []imageMonitorPublicListItem{}})
		return
	}
	views, err := h.monitorService.ListPublicView(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]imageMonitorPublicListItem, 0, len(views))
	for _, v := range views {
		items = append(items, imageMonitorPublicViewToItem(v))
	}
	response.Success(c, gin.H{"items": items})
}

// GetStatus GET /api/v1/image-channel-monitors/:id/status
func (h *ImageChannelMonitorUserHandler) GetStatus(c *gin.Context) {
	if !h.featureEnabled(c) {
		response.ErrorFrom(c, service.ErrImageChannelMonitorNotFound)
		return
	}
	id, ok := admin.ParseChannelMonitorID(c)
	if !ok {
		return
	}
	detail, err := h.monitorService.GetPublicDetail(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	windows := make([]imageMonitorPublicWindowStat, 0, len(detail.Windows))
	for _, w := range detail.Windows {
		windows = append(windows, imageMonitorPublicWindowStat{
			WindowDays:    w.WindowDays,
			Availability:  w.Availability,
			AvgAPITotalMs: w.AvgAPITotalMs,
		})
	}
	response.Success(c, imageMonitorPublicDetailResponse{
		ID:      detail.ID,
		Name:    detail.Name,
		Model:   detail.Model,
		Windows: windows,
	})
}
```

- [ ] **Step 3: 容器 + wire + 路由**

`handler.go` Handlers struct 的 `ChannelMonitor` 字段后加：

```go
	ImageChannelMonitorUser *ImageChannelMonitorUserHandler
```

`handler/wire.go`：provider set 中 `NewChannelMonitorUserHandler,` 后加 `NewImageChannelMonitorUserHandler,`；`provideHandlers` 函数签名加参数 `imageChannelMonitorUserHandler *ImageChannelMonitorUserHandler`，struct 字面量 `ChannelMonitor: channelMonitorUserHandler,` 后加 `ImageChannelMonitorUser: imageChannelMonitorUserHandler,`。

`backend/cmd/server/wire_gen.go` 手工对齐：搜 `NewChannelMonitorUserHandler(`，在其后仿写一行 `imageChannelMonitorUserHandler := handler.NewImageChannelMonitorUserHandler(imageChannelMonitorService, settingService)`（实参变量名以文件内实际为准），并把新变量加进 `provideHandlers(...)` 调用。

`routes/user.go` channel-monitors 组后加：

```go
		// 图片渠道监控(用户只读,仅 public_visible 渠道)
		imageMonitors := authenticated.Group("/image-channel-monitors")
		{
			imageMonitors.GET("", h.ImageChannelMonitorUser.List)
			imageMonitors.GET("/:id/status", h.ImageChannelMonitorUser.GetStatus)
		}
```

- [ ] **Step 4: 跑测试**

Run: `go build ./... && go test -tags=unit ./internal/handler ./internal/service -count=1`
Expected: PASS（含白名单快照）

- [ ] **Step 5: Commit**

```bash
git add backend/internal backend/cmd/server/wire_gen.go
git commit -m "feat(image-monitor): user-facing public status API with whitelisted DTO"
```

---

### Task 7: 前端 admin API 类型 + 编辑表单公开配置 + i18n(第一批)

**Files:**
- Modify: `frontend/src/api/admin/imageChannelMonitor.ts`
- Modify: `frontend/src/views/admin/ImageChannelMonitorView.vue`（form/reset/save + 表单区块）
- Modify: `frontend/src/i18n/locales/zh.ts`、`frontend/src/i18n/locales/en.ts`

**Interfaces:**
- Produces（Task 8 消费）: `ImageChannelMonitor` 增 `public_visible: boolean; public_name: string; availability_7d?: number; timeline?: ImageMonitorTimelinePoint[]`；新导出 `ImageMonitorTimelinePoint`、`ImageMonitorTimelineBucket`、`ImageMonitorTimelineSummary`、`ImageMonitorTimelineResponse`、`timeline(id, window)`。

- [ ] **Step 1: API 类型**

`imageChannelMonitor.ts`：`ImageChannelMonitor` 接口 `enabled: boolean` 后加：

```ts
  public_visible: boolean
  public_name: string
  availability_7d?: number
  timeline?: ImageMonitorTimelinePoint[]
```

`ImageChannelMonitorCreateParams` 加 `public_visible?: boolean; public_name?: string`（Update 是 Partial 自动继承）。文件顶部类型区加：

```ts
export interface ImageMonitorTimelinePoint {
  status: ImageMonitorStatus
  latency_ms: number | null
  image_download_ms: number | null
  checked_at: string
}

export type ImageMonitorTimelineWindow = '24h' | '7d' | '30d'

export interface ImageMonitorTimelineBucket {
  bucket_start: string
  total: number
  operational: number
  degraded: number
  failed: number
  error: number
  avg_api_total_ms: number | null
  max_api_total_ms: number | null
  avg_image_download_ms: number | null
}

export interface ImageMonitorTimelineSummary {
  total: number
  ok: number
  failures: number
  availability: number
  avg_api_total_ms: number | null
  max_api_total_ms: number | null
  avg_image_download_ms: number | null
}

export interface ImageMonitorTimelineResponse {
  window: ImageMonitorTimelineWindow
  summary: ImageMonitorTimelineSummary
  buckets: ImageMonitorTimelineBucket[]
}
```

fetch 函数区加，并入 `imageChannelMonitorAPI` 导出对象：

```ts
export async function timeline(
  id: number,
  window: ImageMonitorTimelineWindow
): Promise<ImageMonitorTimelineResponse> {
  const { data } = await apiClient.get<ImageMonitorTimelineResponse>(
    `/admin/image-channel-monitors/${id}/timeline`,
    { params: { window } }
  )
  return data
}
```

- [ ] **Step 2: 编辑表单**

`ImageChannelMonitorView.vue`：`form` reactive 的 `enabled: true,` 后加 `public_visible: false, public_name: '',`；打开编辑弹窗的赋值处（openEditDialog 里逐字段拷贝的位置）与 reset 逻辑同步这两个字段；`saveMonitor` 组装 payload 处加 `public_visible: form.public_visible, public_name: form.public_name,`。

模板里「n / interval / timeout / 勾选区」那个 `grid gap-4 md:grid-cols-4` 块之后新增一个区块：

```html
        <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.imageChannelMonitor.form.publicSection') }}
          </div>
          <div class="grid gap-4 md:grid-cols-2">
            <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
              <input v-model="form.public_visible" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              {{ t('admin.imageChannelMonitor.form.publicVisible') }}
            </label>
            <label class="block">
              <span class="input-label">{{ t('admin.imageChannelMonitor.form.publicName') }}</span>
              <input
                v-model.trim="form.public_name"
                class="input"
                maxlength="200"
                :placeholder="t('admin.imageChannelMonitor.form.publicNamePlaceholder')"
              />
            </label>
          </div>
          <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.imageChannelMonitor.form.publicHint') }}
          </p>
        </div>
```

- [ ] **Step 3: i18n**

`zh.ts` `admin.imageChannelMonitor.form` 内加：

```ts
        publicSection: '用户侧展示',
        publicVisible: '在用户侧渠道状态页展示',
        publicName: '用户侧展示名',
        publicNamePlaceholder: '留空使用渠道名',
        publicHint: '开启后该渠道的状态、耗时与可用率会出现在用户「渠道状态」页的生图渠道分组；展示名可掩盖内部渠道命名。',
```

`en.ts` 同位置：

```ts
        publicSection: 'User-facing display',
        publicVisible: 'Show on the user channel status page',
        publicName: 'Public display name',
        publicNamePlaceholder: 'Leave empty to use the monitor name',
        publicHint: 'When enabled, this channel\'s status, latency and availability appear in the image section of the user channel status page. The display name can mask internal channel naming.',
```

- [ ] **Step 4: 验证 + Commit**

Run: `pnpm --dir frontend run typecheck && pnpm --dir frontend run lint:check`
Expected: PASS

```bash
git add frontend/src
git commit -m "feat(image-monitor): admin form public display config and timeline API types"
```

---

### Task 8: 管理端行内状态条 + 状态详情弹窗（chart.js）

**Files:**
- Create: `frontend/src/components/admin/ImageMonitorStatusDialog.vue`
- Modify: `frontend/src/views/admin/ImageChannelMonitorView.vue`（行内条 + 按钮 + 弹窗挂载）
- Modify: `frontend/src/i18n/locales/zh.ts`、`en.ts`

**Interfaces:**
- Consumes: Task 7 的 `timeline(id, window)`、`ImageChannelMonitor.timeline/availability_7d`；`MonitorTimeline.vue`（props: `buckets: MonitorTimelinePoint[]`, `countdownSeconds: number`；`MonitorTimelinePoint` 需要 `ping_latency_ms` 字段，映射时补 `null`）。
- Produces: `<ImageMonitorStatusDialog :show="..." :monitor="ImageChannelMonitor | null" @close />`

- [ ] **Step 1: 行内状态条**

`ImageChannelMonitorView.vue`：

script 增加 import 与适配函数：

```ts
import MonitorTimeline from '@/components/user/monitor/MonitorTimeline.vue'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import ImageMonitorStatusDialog from '@/components/admin/ImageMonitorStatusDialog.vue'

const statusDialogTarget = ref<ImageChannelMonitor | null>(null)

function monitorStripPoints(row: ImageChannelMonitor): MonitorTimelinePoint[] {
  return (row.timeline ?? []).map((p) => ({
    status: p.status as MonitorTimelinePoint['status'],
    latency_ms: p.latency_ms,
    ping_latency_ms: null,
    checked_at: p.checked_at,
  }))
}

function rowCountdownSeconds(row: ImageChannelMonitor): number {
  return runtimeStatuses.value[row.id]?.seconds_until_next_check ?? 0
}

function formatAvailability(value: number | undefined): string {
  if (typeof value !== 'number') return '-'
  return `${value.toFixed(1)}%`
}
```

模板 `#cell-name` 里 runtime 胶囊 `div` 之后追加：

```html
              <div class="mt-2 rounded-md bg-gray-50 px-2 pb-1.5 dark:bg-dark-800">
                <MonitorTimeline
                  :buckets="monitorStripPoints(row)"
                  :countdown-seconds="rowCountdownSeconds(row)"
                />
                <div class="mt-1 flex items-center justify-between text-[11px] text-gray-500 dark:text-dark-400">
                  <span>{{ t('admin.imageChannelMonitor.statusStrip.availability7d') }}</span>
                  <span class="tabular-nums font-medium">{{ formatAvailability(row.availability_7d) }}</span>
                </div>
              </div>
```

`#cell-actions` 的「历史」按钮前加：

```html
              <button type="button" class="btn btn-secondary btn-sm" @click="statusDialogTarget = row">
                {{ t('admin.imageChannelMonitor.statusDetail') }}
              </button>
```

模板尾部（历史 BaseDialog 旁）挂载：

```html
    <ImageMonitorStatusDialog
      :show="Boolean(statusDialogTarget)"
      :monitor="statusDialogTarget"
      @close="statusDialogTarget = null"
    />
```

- [ ] **Step 2: 状态详情弹窗组件（完整新文件）**

`frontend/src/components/admin/ImageMonitorStatusDialog.vue`：

```vue
<template>
  <BaseDialog :show="show" :title="dialogTitle" width="extra-wide" @close="emit('close')">
    <div v-if="monitor" class="space-y-4">
      <!-- Window tabs -->
      <div class="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-1 dark:border-dark-700 dark:bg-dark-800" role="tablist">
        <button
          v-for="w in WINDOWS"
          :key="w"
          type="button"
          role="tab"
          :aria-selected="window === w"
          class="rounded-md px-4 py-1.5 text-sm font-medium transition"
          :class="window === w
            ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-900 dark:text-primary-200'
            : 'text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white'"
          @click="switchWindow(w)"
        >
          {{ t(`admin.imageChannelMonitor.statusDialog.window.${w}`) }}
        </button>
      </div>

      <!-- Summary metric cards -->
      <dl v-if="data" class="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3 md:grid-cols-6">
        <div class="rounded-md bg-gray-50 p-3 dark:bg-dark-800">
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.imageChannelMonitor.statusDialog.availability') }}</dt>
          <dd class="mt-1 font-semibold tabular-nums" :class="data.summary.availability >= 99 ? 'text-emerald-600 dark:text-emerald-300' : data.summary.availability >= 90 ? 'text-amber-600 dark:text-amber-300' : 'text-red-600 dark:text-red-300'">
            {{ data.summary.total > 0 ? `${data.summary.availability.toFixed(1)}%` : '-' }}
          </dd>
        </div>
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.checks')" :value="String(data.summary.total)" />
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.failures')" :value="String(data.summary.failures)" :tone="data.summary.failures > 0 ? 'warn' : undefined" />
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.avgApi')" :value="formatMs(data.summary.avg_api_total_ms)" />
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.maxApi')" :value="formatMs(data.summary.max_api_total_ms)" />
        <MetricCard :label="t('admin.imageChannelMonitor.statusDialog.avgDownload')" :value="formatMs(data.summary.avg_image_download_ms)" />
      </dl>

      <!-- Latency line chart + failure bars -->
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
        <div class="mb-2 text-xs font-medium text-gray-500 dark:text-dark-400">
          {{ t('admin.imageChannelMonitor.statusDialog.chartTitle') }}
        </div>
        <div class="h-64">
          <Chart v-if="chartData" type="bar" :data="chartData" :options="chartOptions" />
          <div v-else class="flex h-full items-center justify-center text-sm text-gray-400 dark:text-dark-500">
            {{ loading ? t('common.loading') : t('common.noData') }}
          </div>
        </div>
      </div>

      <!-- Recent-checks strip (per-check, same as the row strip) -->
      <div class="rounded-lg border border-gray-200 px-3 pb-2 dark:border-dark-700">
        <MonitorTimeline :buckets="stripPoints" :countdown-seconds="0" />
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-primary" @click="emit('close')">
        {{ t('common.close') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  LineElement,
  PointElement,
  BarElement,
  CategoryScale,
  LinearScale,
  Tooltip,
  Legend,
} from 'chart.js'
import { Chart } from 'vue-chartjs'
import BaseDialog from '@/components/common/BaseDialog.vue'
import MonitorTimeline from '@/components/user/monitor/MonitorTimeline.vue'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import imageChannelMonitorAPI, {
  type ImageChannelMonitor,
  type ImageMonitorTimelineResponse,
  type ImageMonitorTimelineWindow,
} from '@/api/admin/imageChannelMonitor'

ChartJS.register(LineElement, PointElement, BarElement, CategoryScale, LinearScale, Tooltip, Legend)

const props = defineProps<{
  show: boolean
  monitor: ImageChannelMonitor | null
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()
const appStore = useAppStore()

const WINDOWS: ImageMonitorTimelineWindow[] = ['24h', '7d', '30d']
const window = ref<ImageMonitorTimelineWindow>('24h')
const data = ref<ImageMonitorTimelineResponse | null>(null)
const loading = ref(false)
let requestSeq = 0

const MetricCard = (p: { label: string; value: string; tone?: 'warn' }) =>
  h('div', { class: 'rounded-md bg-gray-50 p-3 dark:bg-dark-800' }, [
    h('dt', { class: 'text-xs text-gray-500 dark:text-dark-400' }, p.label),
    h(
      'dd',
      {
        class: [
          'mt-1 font-semibold tabular-nums',
          p.tone === 'warn' ? 'text-red-600 dark:text-red-300' : 'text-gray-900 dark:text-white',
        ],
      },
      p.value
    ),
  ])

const dialogTitle = computed(() =>
  props.monitor
    ? `${t('admin.imageChannelMonitor.statusDialog.title')} · ${props.monitor.name}`
    : t('admin.imageChannelMonitor.statusDialog.title')
)

const stripPoints = computed<MonitorTimelinePoint[]>(() =>
  (props.monitor?.timeline ?? []).map((p) => ({
    status: p.status as MonitorTimelinePoint['status'],
    latency_ms: p.latency_ms,
    ping_latency_ms: null,
    checked_at: p.checked_at,
  }))
)

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))

function bucketLabel(iso: string): string {
  const d = new Date(iso)
  if (window.value === '24h') {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  if (window.value === '7d') {
    return d.toLocaleString([], { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString([], { month: '2-digit', day: '2-digit' })
}

const chartData = computed(() => {
  if (!data.value || data.value.buckets.length === 0) return null
  const buckets = data.value.buckets
  return {
    labels: buckets.map((b) => bucketLabel(b.bucket_start)),
    datasets: [
      {
        type: 'line' as const,
        label: t('admin.imageChannelMonitor.statusDialog.seriesApi'),
        data: buckets.map((b) => b.avg_api_total_ms),
        borderColor: '#6366f1',
        backgroundColor: '#6366f1',
        spanGaps: false,
        tension: 0.25,
        pointRadius: 2,
        yAxisID: 'y',
      },
      {
        type: 'line' as const,
        label: t('admin.imageChannelMonitor.statusDialog.seriesDownload'),
        data: buckets.map((b) => b.avg_image_download_ms),
        borderColor: '#f59e0b',
        backgroundColor: '#f59e0b',
        spanGaps: false,
        tension: 0.25,
        pointRadius: 2,
        yAxisID: 'y',
      },
      {
        type: 'bar' as const,
        label: t('admin.imageChannelMonitor.statusDialog.seriesFailures'),
        data: buckets.map((b) => b.failed + b.error),
        backgroundColor: 'rgba(239, 68, 68, 0.35)',
        borderRadius: 2,
        yAxisID: 'y1',
      },
    ],
  }
})

const chartOptions = computed(() => {
  const grid = isDarkMode.value ? '#374151' : '#f3f4f6'
  const text = isDarkMode.value ? '#9ca3af' : '#6b7280'
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: { labels: { color: text, boxWidth: 12, font: { size: 11 } } },
    },
    scales: {
      x: { grid: { display: false }, ticks: { color: text, font: { size: 10 }, maxTicksLimit: 12 } },
      y: {
        beginAtZero: true,
        title: { display: true, text: 'ms', color: text, font: { size: 10 } },
        grid: { color: grid },
        ticks: { color: text, font: { size: 10 } },
      },
      y1: {
        beginAtZero: true,
        position: 'right' as const,
        grid: { display: false },
        ticks: { color: text, font: { size: 10 }, precision: 0 },
      },
    },
  }
})

function formatMs(value: number | null): string {
  return typeof value === 'number' ? `${value.toLocaleString()} ms` : '-'
}

async function load() {
  if (!props.monitor) return
  const seq = ++requestSeq
  loading.value = true
  try {
    const res = await imageChannelMonitorAPI.timeline(props.monitor.id, window.value)
    if (seq === requestSeq) data.value = res
  } catch (err) {
    appStore.showError(
      extractApiErrorMessage(err, t('admin.imageChannelMonitor.statusDialog.loadError'))
    )
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function switchWindow(w: ImageMonitorTimelineWindow) {
  if (window.value === w) return
  window.value = w
  void load()
}

watch(
  () => [props.show, props.monitor?.id],
  ([show]) => {
    if (show && props.monitor) {
      window.value = '24h'
      data.value = null
      void load()
    }
  }
)
</script>
```

**vue-chartjs 兜底**：若 `import { Chart } from 'vue-chartjs'` 在本版本（5.3）不可用（typecheck 报无此导出），改用 `import { Bar } from 'vue-chartjs'` 渲染 `<Bar :data="chartData" :options="chartOptions" />`——chart.js 4 支持 dataset 级 `type: 'line'` 覆盖，混合图效果一致，只需把模板里 `<Chart type="bar" ...>` 换成 `<Bar ...>`。

- [ ] **Step 3: i18n**

`zh.ts` `admin.imageChannelMonitor` 下加：

```ts
      statusDetail: '状态详情',
      statusStrip: {
        availability7d: '7 天可用率'
      },
      statusDialog: {
        title: '监控状态详情',
        window: { '24h': '近 24 小时', '7d': '近 7 天', '30d': '近 30 天' },
        availability: '可用率',
        checks: '检查次数',
        failures: '失败次数',
        avgApi: '平均 API 耗时',
        maxApi: '最大 API 耗时',
        avgDownload: '平均下载耗时',
        chartTitle: '耗时趋势(线)与失败次数(柱)',
        seriesApi: 'API 总耗时',
        seriesDownload: '图片下载',
        seriesFailures: '失败次数',
        loadError: '加载监控时间线失败'
      },
```

`en.ts` 对应：

```ts
      statusDetail: 'Status detail',
      statusStrip: {
        availability7d: '7d availability'
      },
      statusDialog: {
        title: 'Monitor status detail',
        window: { '24h': 'Last 24h', '7d': 'Last 7d', '30d': 'Last 30d' },
        availability: 'Availability',
        checks: 'Checks',
        failures: 'Failures',
        avgApi: 'Avg API latency',
        maxApi: 'Max API latency',
        avgDownload: 'Avg download',
        chartTitle: 'Latency trend (lines) and failures (bars)',
        seriesApi: 'API total',
        seriesDownload: 'Image download',
        seriesFailures: 'Failures',
        loadError: 'Failed to load monitor timeline'
      },
```

注意 `monitorCommon.*`（MonitorTimeline 内部用的 `history60pts`/`nextUpdateIn`/`past`/`now`/`maintenancePaused`）已存在于两份 locale——管理端复用组件无需新增。

- [ ] **Step 4: 验证 + Commit**

Run: `pnpm --dir frontend run typecheck && pnpm --dir frontend run lint:check`
Expected: PASS

手工（dev 栈运行中,air+HMR 自动生效）:管理页每行出现状态条与 7 天可用率;点「状态详情」弹窗能在三个窗口间切换,折线+失败柱渲染正常。

```bash
git add frontend/src
git commit -m "feat(image-monitor): admin inline status strip and timeline detail dialog"
```

---

### Task 9: 用户侧（API client + 卡片 + 详情弹窗 + ChannelStatusView 集成 + i18n + vitest）

**Files:**
- Create: `frontend/src/api/imageChannelMonitor.ts`
- Create: `frontend/src/components/user/monitor/ImageMonitorCard.vue`
- Create: `frontend/src/components/user/monitor/ImageMonitorDetailDialog.vue`
- Create: `frontend/src/components/user/monitor/__tests__/ImageMonitorCard.spec.ts`
- Modify: `frontend/src/views/user/ChannelStatusView.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`、`en.ts`

**Interfaces:**
- Consumes: Task 6 的两个用户端点；`MonitorMetricPair`（props: primary-icon/label/value/unit + secondary-*）、`MonitorAvailabilityRow`（window-label/value/samples-label）、`MonitorTimeline`、`useChannelMonitorFormat`（statusLabel/statusBadgeClass/formatLatency）。
- Produces: `ImageMonitorPublicView` TS 类型；`<ImageMonitorCard :item :window :availability-value :countdown-seconds @click>`；`<ImageMonitorDetailDialog :show :monitor-id :title @close>`。

- [ ] **Step 1: 用户 API client（完整新文件）**

`frontend/src/api/imageChannelMonitor.ts`：

```ts
/**
 * User-facing Image Channel Monitor API endpoints
 * Read-only status views for public (admin-whitelisted) image channels.
 */

import { apiClient } from './client'
import type { MonitorStatus } from './admin/channelMonitor'

export interface ImageMonitorPublicTimelinePoint {
  status: MonitorStatus
  latency_ms: number | null
  checked_at: string
}

export interface ImageMonitorPublicView {
  id: number
  name: string
  model: string
  latest_status: MonitorStatus | 'empty'
  latest_api_ms: number | null
  latest_download_ms: number | null
  availability_7d: number
  availability_15d: number
  availability_30d: number
  timeline: ImageMonitorPublicTimelinePoint[]
}

export interface ImageMonitorPublicListResponse {
  items: ImageMonitorPublicView[]
}

export interface ImageMonitorPublicWindowStat {
  window_days: 7 | 15 | 30
  availability: number
  avg_api_total_ms: number | null
}

export interface ImageMonitorPublicDetail {
  id: number
  name: string
  model: string
  windows: ImageMonitorPublicWindowStat[]
}

export async function list(options?: {
  signal?: AbortSignal
}): Promise<ImageMonitorPublicListResponse> {
  const { data } = await apiClient.get<ImageMonitorPublicListResponse>('/image-channel-monitors', {
    signal: options?.signal,
  })
  return data
}

export async function status(id: number): Promise<ImageMonitorPublicDetail> {
  const { data } = await apiClient.get<ImageMonitorPublicDetail>(
    `/image-channel-monitors/${id}/status`
  )
  return data
}

export const imageChannelMonitorUserAPI = { list, status }

export default imageChannelMonitorUserAPI
```

- [ ] **Step 2: 失败的组件测试**

`frontend/src/components/user/monitor/__tests__/ImageMonitorCard.spec.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ImageMonitorCard from '../ImageMonitorCard.vue'
import zh from '@/i18n/locales/zh'
import type { ImageMonitorPublicView } from '@/api/imageChannelMonitor'

const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })

function makeItem(overrides: Partial<ImageMonitorPublicView> = {}): ImageMonitorPublicView {
  return {
    id: 1,
    name: '生图通道A',
    model: 'gpt-image-1',
    latest_status: 'operational',
    latest_api_ms: 18234,
    latest_download_ms: 2100,
    availability_7d: 99.5,
    availability_15d: 98.2,
    availability_30d: 97.1,
    timeline: [
      { status: 'operational', latency_ms: 18234, checked_at: new Date().toISOString() },
    ],
    ...overrides,
  }
}

describe('ImageMonitorCard', () => {
  it('renders public name, model and availability for the active window', () => {
    const wrapper = mount(ImageMonitorCard, {
      global: { plugins: [i18n] },
      props: {
        item: makeItem(),
        window: '7d',
        availabilityValue: 99.5,
        countdownSeconds: 30,
      },
    })
    expect(wrapper.text()).toContain('生图通道A')
    expect(wrapper.text()).toContain('gpt-image-1')
    expect(wrapper.text()).toContain('99.5')
  })

  it('emits click when card pressed', async () => {
    const wrapper = mount(ImageMonitorCard, {
      global: { plugins: [i18n] },
      props: {
        item: makeItem(),
        window: '7d',
        availabilityValue: 99.5,
        countdownSeconds: 30,
      },
    })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('click')).toBeTruthy()
  })
})
```

Run: `pnpm --dir frontend exec vitest run src/components/user/monitor/__tests__/ImageMonitorCard.spec.ts`
Expected: FAIL（组件不存在）

注意：先看 `frontend/src/components/user/usage/__tests__/UsageMetricTrendChart.spec.ts` 等现有 spec 如何构造 i18n/挂载环境；若项目有共享的测试 i18n 工具或对 zh.ts 的既定导入方式，改用同款，保持一致。

- [ ] **Step 3: 卡片组件（完整新文件）**

`frontend/src/components/user/monitor/ImageMonitorCard.vue`：

```vue
<template>
  <button
    type="button"
    class="group text-left p-5 rounded-2xl min-h-[280px] w-full bg-white/70 backdrop-blur-xl border border-gray-200/80 shadow-card dark:bg-dark-800/60 dark:border-dark-700/70 hover:-translate-y-1 hover:shadow-card-hover dark:hover:border-primary-500/30 hover:border-gray-300 transition-all duration-300 ease-out flex flex-col"
    @click="emit('click')"
  >
    <div class="flex items-start gap-3">
      <span
        class="w-9 h-9 rounded-xl ring-1 ring-black/5 dark:ring-white/10 grid place-items-center flex-shrink-0 bg-gradient-to-br from-fuchsia-500/15 to-purple-500/15 text-fuchsia-600 dark:text-fuchsia-300"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M4 16l4.6-4.6a2 2 0 012.8 0L16 16m-2-2l1.6-1.6a2 2 0 012.8 0L20 14M4 6h16a0 0 0 010 0v12a2 2 0 01-2 2H6a2 2 0 01-2-2V6a0 0 0 010 0zM14 9h.01"
          />
        </svg>
      </span>
      <div class="flex-1 min-w-0">
        <div class="text-base font-semibold truncate text-gray-900 dark:text-gray-100">
          {{ item.name }}
        </div>
        <div class="mt-0.5 flex items-center gap-1.5 min-w-0">
          <span
            class="inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium flex-shrink-0 bg-fuchsia-50 text-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-200"
          >
            {{ t('channelStatus.imageSection.badge') }}
          </span>
          <span class="font-mono text-xs truncate text-gray-500 dark:text-gray-400">
            {{ item.model }}
          </span>
        </div>
      </div>
      <span
        class="px-2.5 py-1 rounded-full text-xs font-semibold flex-shrink-0"
        :class="statusBadgeClass(displayStatus)"
      >
        {{ statusLabel(displayStatus) }}
      </span>
    </div>

    <MonitorMetricPair
      primary-icon="bolt"
      :primary-label="t('channelStatus.imageSection.apiLatency')"
      :primary-value="formatLatency(item.latest_api_ms)"
      primary-unit="ms"
      secondary-icon="globe"
      :secondary-label="t('channelStatus.imageSection.downloadLatency')"
      :secondary-value="formatLatency(item.latest_download_ms)"
      secondary-unit="ms"
    />

    <div class="mt-4 border-t border-gray-100 dark:border-dark-700/60"></div>

    <MonitorAvailabilityRow :window-label="availabilityLabel" :value="availabilityValue" />

    <MonitorTimeline :buckets="timelinePoints" :countdown-seconds="countdownSeconds" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageMonitorPublicView } from '@/api/imageChannelMonitor'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import MonitorMetricPair from './MonitorMetricPair.vue'
import MonitorAvailabilityRow from './MonitorAvailabilityRow.vue'
import MonitorTimeline from './MonitorTimeline.vue'

const props = defineProps<{
  item: ImageMonitorPublicView
  window: '7d' | '15d' | '30d'
  availabilityValue: number | null
  countdownSeconds: number
}>()

const emit = defineEmits<{ (e: 'click'): void }>()

const { t } = useI18n()
const { statusLabel, statusBadgeClass, formatLatency } = useChannelMonitorFormat()

// "empty"(尚无检查记录)沿用 failed 徽章会误导,归一成 degraded 样式之外的中性态:
// useChannelMonitorFormat 未识别的状态回落灰色,这里直接透传即可。
const displayStatus = computed(() => props.item.latest_status)

const availabilityLabel = computed(() => {
  const win = t(`channelStatus.windowTab.${props.window}`)
  return `${t('monitorCommon.availabilityPrefix')} · ${win}`
})

const timelinePoints = computed<MonitorTimelinePoint[]>(() =>
  props.item.timeline.map((p) => ({
    status: p.status,
    latency_ms: p.latency_ms,
    ping_latency_ms: null,
    checked_at: p.checked_at,
  }))
)
</script>
```

注意：先确认 `useChannelMonitorFormat().statusLabel/statusBadgeClass` 对未知值（`empty`）有回落分支——若没有，给 `displayStatus` 加 `props.item.latest_status === 'empty' ? 'failed' : ...` 之前先在 composable 补一个灰色回落（不改既有状态的样式）。

- [ ] **Step 4: 详情弹窗（完整新文件）**

`frontend/src/components/user/monitor/ImageMonitorDetailDialog.vue`：

```vue
<template>
  <BaseDialog :show="show" :title="title" @close="emit('close')">
    <div v-if="loading" class="py-10 text-center text-sm text-gray-400 dark:text-dark-500">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="detail" class="space-y-3">
      <div class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('channelStatus.imageSection.detailModel') }}:
        <span class="font-mono">{{ detail.model }}</span>
      </div>
      <table class="w-full text-sm">
        <thead>
          <tr class="text-left text-xs uppercase text-gray-500 dark:text-dark-400">
            <th class="py-2 pr-4">{{ t('channelStatus.imageSection.detailWindow') }}</th>
            <th class="py-2 pr-4">{{ t('channelStatus.imageSection.detailAvailability') }}</th>
            <th class="py-2">{{ t('channelStatus.imageSection.detailAvgLatency') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
          <tr v-for="w in detail.windows" :key="w.window_days">
            <td class="py-2.5 pr-4">{{ t(`channelStatus.windowTab.${w.window_days}d`) }}</td>
            <td class="py-2.5 pr-4 tabular-nums font-medium">{{ w.availability.toFixed(1) }}%</td>
            <td class="py-2.5 tabular-nums">
              {{ w.avg_api_total_ms != null ? `${w.avg_api_total_ms.toLocaleString()} ms` : '-' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <template #footer>
      <button type="button" class="btn btn-primary" @click="emit('close')">
        {{ t('common.close') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { status as fetchImageMonitorDetail, type ImageMonitorPublicDetail } from '@/api/imageChannelMonitor'

const props = defineProps<{
  show: boolean
  monitorId: number | null
  title: string
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()
const appStore = useAppStore()
const detail = ref<ImageMonitorPublicDetail | null>(null)
const loading = ref(false)

watch(
  () => [props.show, props.monitorId],
  async ([show, id]) => {
    if (!show || typeof id !== 'number') return
    loading.value = true
    detail.value = null
    try {
      detail.value = await fetchImageMonitorDetail(id)
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t('channelStatus.detailLoadError')))
    } finally {
      loading.value = false
    }
  }
)
</script>
```

- [ ] **Step 5: ChannelStatusView 集成**

`ChannelStatusView.vue` script 增加：

```ts
import { list as listImageMonitorViews, type ImageMonitorPublicView } from '@/api/imageChannelMonitor'
import ImageMonitorCard from '@/components/user/monitor/ImageMonitorCard.vue'
import ImageMonitorDetailDialog from '@/components/user/monitor/ImageMonitorDetailDialog.vue'

const imageItems = ref<ImageMonitorPublicView[]>([])
const showImageDetail = ref(false)
const imageDetailTarget = ref<ImageMonitorPublicView | null>(null)

async function loadImageMonitors() {
  try {
    const res = await listImageMonitorViews()
    imageItems.value = res.items || []
  } catch {
    // 图片分组是页面的次要区域:加载失败静默保留旧数据,不打断主列表。
  }
}

function imageAvailability(item: ImageMonitorPublicView): number | null {
  if (currentWindow.value === '15d') return item.availability_15d ?? null
  if (currentWindow.value === '30d') return item.availability_30d ?? null
  return item.availability_7d ?? null
}

function openImageDetail(item: ImageMonitorPublicView) {
  imageDetailTarget.value = item
  showImageDetail.value = true
}
```

`reload(silent)` 函数体在 `listChannelMonitorViews` 请求前加一行并行触发：`void loadImageMonitors()`（不 await、不占用 loading 状态）；`onMounted` 已经调 `reload(false)` 故无需额外调用。

`overallStatus` computed 的 for 循环后加（return 'operational' 之前）：

```ts
  for (const it of imageItems.value) {
    if (it.latest_status === 'failed' || it.latest_status === 'error') return 'degraded'
  }
```

模板在 `<MonitorCardGrid ... />` 之后加：

```html
    <section v-if="imageItems.length > 0" class="mt-8">
      <h2 class="mb-4 text-sm font-semibold uppercase tracking-widest text-gray-500 dark:text-dark-400">
        {{ t('channelStatus.imageSection.title') }}
      </h2>
      <div class="grid gap-5 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
        <ImageMonitorCard
          v-for="item in imageItems"
          :key="item.id"
          :item="item"
          :window="currentWindow"
          :availability-value="imageAvailability(item)"
          :countdown-seconds="countdown"
          @click="openImageDetail(item)"
        />
      </div>
    </section>

    <ImageMonitorDetailDialog
      :show="showImageDetail"
      :monitor-id="imageDetailTarget?.id ?? null"
      :title="imageDetailTarget?.name || t('channelStatus.imageSection.title')"
      @close="showImageDetail = false"
    />
```

注意：`MonitorHero`/`MonitorCardGrid` 外层若有统一容器 padding，把 section 放进同一容器层级（对照模板现状调整缩进层级，保持左右对齐）。

- [ ] **Step 6: i18n**

`zh.ts` `channelStatus` 下加：

```ts
    imageSection: {
      title: '生图渠道',
      badge: '生图',
      apiLatency: '生图耗时',
      downloadLatency: '图片下载',
      detailModel: '模型',
      detailWindow: '窗口',
      detailAvailability: '可用率',
      detailAvgLatency: '平均生图耗时'
    },
```

`en.ts`：

```ts
    imageSection: {
      title: 'Image channels',
      badge: 'Image',
      apiLatency: 'Generation',
      downloadLatency: 'Download',
      detailModel: 'Model',
      detailWindow: 'Window',
      detailAvailability: 'Availability',
      detailAvgLatency: 'Avg generation latency'
    },
```

- [ ] **Step 7: 跑测试**

Run: `pnpm --dir frontend exec vitest run src/components/user/monitor/__tests__/ImageMonitorCard.spec.ts`
Expected: PASS

Run: `pnpm --dir frontend run typecheck && pnpm --dir frontend run lint:check && pnpm --dir frontend exec vitest run src/__tests__/integration/navigation.spec.ts`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add frontend/src
git commit -m "feat(image-monitor): user-facing image channel section on channel status page"
```

---

### Task 10: 文档 + 全量验证 + 手工浏览器验证

**Files:**
- Modify: `docs/dev/codebase/image-channel-monitor.md`
- Modify: `docs/dev/CHANGELOG_CUSTOM.md`

- [ ] **Step 1: 全量后端/前端验证**

Run:
```bash
cd backend && go build ./... && go test -tags=unit ./... -count=1
pnpm --dir frontend run typecheck && pnpm --dir frontend run lint:check && pnpm --dir frontend test:run
```
Expected: 全 PASS（`test:run` 若有与本改动无关的既有失败,记录并只修本改动引入的）

- [ ] **Step 2: 手工浏览器验证（dev 栈 + preview 15175）**

1. 造数据：向 `image_channel_monitor_histories` 注入跨 3 天、含 failed/degraded 的行（psql 循环 INSERT,monitor_id 用现有渠道）;
2. 管理页:行内状态条渲染、7 天可用率数字、状态详情弹窗三窗口切换、折线/失败柱、暗色模式;
3. 编辑弹窗:打开「用户侧展示」,勾选 + 填展示名,保存后 GET 单条确认落库;
4. 用户页 `/monitor`:生图渠道分组出现、卡片显示展示名(非内部名)、窗口切换可用率变化、点卡片详情 7/15/30d、时间线 tooltip;
5. 净化抽查:浏览器 Network 面板看 `/api/v1/image-channel-monitors` 响应,确认无 endpoint/message/error_stage/IP 字段;
6. 关闭渠道 public_visible 后用户页分组消失(或该卡片消失);
7. 清理注入的测试数据。

- [ ] **Step 3: 文档**

`docs/dev/codebase/image-channel-monitor.md`：Data Model 补 `public_visible`/`public_name` 与保留期;Key Files 补新 handler/组件;Invariants 补三条——(1) 历史 30 天物理删除由 ops 每日清理触发;(2) 用户侧 DTO 白名单与快照测试的位置;(3) 时间线窗口/分桶映射(24h/10min、7d/2h、30d/1d)与可用率口径。

`docs/dev/CHANGELOG_CUSTOM.md`：按既有格式(最新在上)加 `[日期] feat: 图片渠道监控状态时间线+用户侧公开展示` 条目,列影响范围/上游兼容性(新增迁移 178、NewOpsCleanupService 签名变化、wire_gen 手工对齐)/变更详情/验证。

- [ ] **Step 4: Commit**

```bash
git add -f docs/dev/codebase/image-channel-monitor.md docs/dev/CHANGELOG_CUSTOM.md
git commit -m "docs(image-monitor): document status timeline, public display and retention"
```
