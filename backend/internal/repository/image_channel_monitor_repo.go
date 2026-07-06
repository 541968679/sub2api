package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/imagechannelmonitor"
	"github.com/Wei-Shaw/sub2api/ent/imagechannelmonitorhistory"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type imageChannelMonitorRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewImageChannelMonitorRepository(client *dbent.Client, db *sql.DB) service.ImageChannelMonitorRepository {
	return &imageChannelMonitorRepository{client: client, db: db}
}

func (r *imageChannelMonitorRepository) Create(ctx context.Context, m *service.ImageChannelMonitor) error {
	client := clientFromContext(ctx, r.client)
	builder := client.ImageChannelMonitor.Create().
		SetName(m.Name).
		SetSourceType(imagechannelmonitor.SourceType(m.SourceType)).
		SetEndpoint(m.Endpoint).
		SetAPIKeyEncrypted(m.APIKey).
		SetAccountName(m.AccountName).
		SetProxyName(m.ProxyName).
		SetModel(m.Model).
		SetPrompt(m.Prompt).
		SetSize(m.Size).
		SetQuality(m.Quality).
		SetN(m.N).
		SetDownloadImage(m.DownloadImage).
		SetResponseFormat(m.ResponseFormat).
		SetEnabled(m.Enabled).
		SetPublicVisible(m.PublicVisible).
		SetPublicName(m.PublicName).
		SetIntervalSeconds(m.IntervalSeconds).
		SetTimeoutSeconds(m.TimeoutSeconds).
		SetCreatedBy(m.CreatedBy)
	if m.AccountID != nil {
		builder = builder.SetAccountID(*m.AccountID)
	}
	if m.ProxyID != nil {
		builder = builder.SetProxyID(*m.ProxyID)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrImageChannelMonitorNotFound, nil)
	}
	assignImageMonitorGeneratedFields(m, created)
	return nil
}

func (r *imageChannelMonitorRepository) GetByID(
	ctx context.Context,
	id int64,
) (*service.ImageChannelMonitor, error) {
	row, err := r.client.ImageChannelMonitor.Query().
		Where(imagechannelmonitor.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrImageChannelMonitorNotFound, nil)
	}
	return entToServiceImageMonitor(row), nil
}

func (r *imageChannelMonitorRepository) Update(ctx context.Context, m *service.ImageChannelMonitor) error {
	client := clientFromContext(ctx, r.client)
	updater := client.ImageChannelMonitor.UpdateOneID(m.ID).
		SetName(m.Name).
		SetSourceType(imagechannelmonitor.SourceType(m.SourceType)).
		SetEndpoint(m.Endpoint).
		SetAPIKeyEncrypted(m.APIKey).
		SetAccountName(m.AccountName).
		SetProxyName(m.ProxyName).
		SetModel(m.Model).
		SetPrompt(m.Prompt).
		SetSize(m.Size).
		SetQuality(m.Quality).
		SetN(m.N).
		SetDownloadImage(m.DownloadImage).
		SetResponseFormat(m.ResponseFormat).
		SetEnabled(m.Enabled).
		SetPublicVisible(m.PublicVisible).
		SetPublicName(m.PublicName).
		SetIntervalSeconds(m.IntervalSeconds).
		SetTimeoutSeconds(m.TimeoutSeconds)
	if m.AccountID != nil {
		updater = updater.SetAccountID(*m.AccountID)
	} else {
		updater = updater.ClearAccountID()
	}
	if m.ProxyID != nil {
		updater = updater.SetProxyID(*m.ProxyID)
	} else {
		updater = updater.ClearProxyID()
	}
	updated, err := updater.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrImageChannelMonitorNotFound, nil)
	}
	assignImageMonitorGeneratedFields(m, updated)
	return nil
}

func (r *imageChannelMonitorRepository) Delete(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	if err := client.ImageChannelMonitor.DeleteOneID(id).Exec(ctx); err != nil {
		return translatePersistenceError(err, service.ErrImageChannelMonitorNotFound, nil)
	}
	return nil
}

func (r *imageChannelMonitorRepository) List(
	ctx context.Context,
	params service.ImageChannelMonitorListParams,
) ([]*service.ImageChannelMonitor, int64, error) {
	q := r.client.ImageChannelMonitor.Query()
	if params.SourceType != "" {
		q = q.Where(imagechannelmonitor.SourceTypeEQ(imagechannelmonitor.SourceType(params.SourceType)))
	}
	if params.Enabled != nil {
		q = q.Where(imagechannelmonitor.EnabledEQ(*params.Enabled))
	}
	if s := strings.TrimSpace(params.Search); s != "" {
		q = q.Where(imagechannelmonitor.Or(
			imagechannelmonitor.NameContainsFold(s),
			imagechannelmonitor.ModelContainsFold(s),
			imagechannelmonitor.AccountNameContainsFold(s),
			imagechannelmonitor.EndpointContainsFold(s),
		))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count image channel monitors: %w", err)
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := params.Page
	if page <= 0 {
		page = 1
	}
	rows, err := q.
		Order(dbent.Desc(imagechannelmonitor.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list image channel monitors: %w", err)
	}
	out := make([]*service.ImageChannelMonitor, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToServiceImageMonitor(row))
	}
	return out, int64(total), nil
}

func (r *imageChannelMonitorRepository) ListEnabled(ctx context.Context) ([]*service.ImageChannelMonitor, error) {
	rows, err := r.client.ImageChannelMonitor.Query().
		Where(imagechannelmonitor.EnabledEQ(true)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled image channel monitors: %w", err)
	}
	out := make([]*service.ImageChannelMonitor, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToServiceImageMonitor(row))
	}
	return out, nil
}

func (r *imageChannelMonitorRepository) MarkChecked(ctx context.Context, id int64, checkedAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	if err := client.ImageChannelMonitor.UpdateOneID(id).
		SetLastCheckedAt(checkedAt).
		Exec(ctx); err != nil {
		return translatePersistenceError(err, service.ErrImageChannelMonitorNotFound, nil)
	}
	return nil
}

func (r *imageChannelMonitorRepository) InsertHistory(
	ctx context.Context,
	row *service.ImageChannelMonitorHistoryRow,
) error {
	if row == nil {
		return nil
	}
	client := clientFromContext(ctx, r.client)
	builder := client.ImageChannelMonitorHistory.Create().
		SetMonitorID(row.MonitorID).
		SetStatus(imagechannelmonitorhistory.Status(row.Status)).
		SetHasURL(row.HasURL).
		SetHasB64JSON(row.HasB64JSON).
		SetResponseFormat(row.ResponseFormat).
		SetImageURLHost(row.ImageURLHost).
		SetImageContentType(row.ImageContentType).
		SetErrorStage(row.ErrorStage).
		SetMessage(row.Message).
		SetCheckedAt(row.CheckedAt)
	if row.HTTPStatus != nil {
		builder = builder.SetHTTPStatus(*row.HTTPStatus)
	}
	if row.APIHeaderMs != nil {
		builder = builder.SetAPIHeaderMs(*row.APIHeaderMs)
	}
	if row.APIBodyMs != nil {
		builder = builder.SetAPIBodyMs(*row.APIBodyMs)
	}
	if row.APITotalMs != nil {
		builder = builder.SetAPITotalMs(*row.APITotalMs)
	}
	if row.JSONBytes != nil {
		builder = builder.SetJSONBytes(*row.JSONBytes)
	}
	if row.ImageFirstByteMs != nil {
		builder = builder.SetImageFirstByteMs(*row.ImageFirstByteMs)
	}
	if row.ImageDownloadMs != nil {
		builder = builder.SetImageDownloadMs(*row.ImageDownloadMs)
	}
	if row.ImageBytes != nil {
		builder = builder.SetImageBytes(*row.ImageBytes)
	}
	if row.ImageWidth != nil {
		builder = builder.SetImageWidth(*row.ImageWidth)
	}
	if row.ImageHeight != nil {
		builder = builder.SetImageHeight(*row.ImageHeight)
	}
	if _, err := builder.Save(ctx); err != nil {
		return fmt.Errorf("insert image channel monitor history: %w", err)
	}
	return nil
}

func (r *imageChannelMonitorRepository) ListHistory(
	ctx context.Context,
	monitorID int64,
	limit int,
) ([]*service.ImageChannelMonitorHistoryEntry, error) {
	rows, err := r.client.ImageChannelMonitorHistory.Query().
		Where(imagechannelmonitorhistory.MonitorIDEQ(monitorID)).
		Order(dbent.Desc(imagechannelmonitorhistory.FieldCheckedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list image channel monitor history: %w", err)
	}
	out := make([]*service.ImageChannelMonitorHistoryEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToServiceImageMonitorHistory(row))
	}
	return out, nil
}

func (r *imageChannelMonitorRepository) DeleteHistoryBefore(
	ctx context.Context,
	before time.Time,
) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM image_channel_monitor_histories WHERE checked_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete image channel monitor history: %w", err)
	}
	affected, _ := res.RowsAffected()
	return affected, nil
}

// AggregateTimeline 按 epoch-floor 分桶聚合(bucketSeconds 任意粒度,10min/2h/1d 共用一条 SQL)。
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
		b.AvgAPITotalMs = imageMonitorNullFloatToIntPtr(avgAPI)
		b.AvgImageDownloadMs = imageMonitorNullFloatToIntPtr(avgDL)
		if maxAPI.Valid {
			v := int(maxAPI.Int64)
			b.MaxAPITotalMs = &v
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func imageMonitorNullFloatToIntPtr(v sql.NullFloat64) *int {
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
	s.AvgAPITotalMs = imageMonitorNullFloatToIntPtr(avgAPI)
	s.AvgImageDownloadMs = imageMonitorNullFloatToIntPtr(avgDL)
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
// 结构对齐 channel_monitor_repo.go 的同名方法;图片监控无 model 维度故白名单只有 id。
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
			D7:  imageMonitorPct(ok7, t7),
			D15: imageMonitorPct(ok15, t15),
			D30: imageMonitorPct(ok30, t30),
		}
	}
	return out, rows.Err()
}

func imageMonitorPct(ok, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(ok) / float64(total) * 100
}

func entToServiceImageMonitor(row *dbent.ImageChannelMonitor) *service.ImageChannelMonitor {
	if row == nil {
		return nil
	}
	return &service.ImageChannelMonitor{
		ID:              row.ID,
		Name:            row.Name,
		SourceType:      string(row.SourceType),
		Endpoint:        row.Endpoint,
		APIKey:          row.APIKeyEncrypted,
		AccountID:       row.AccountID,
		AccountName:     row.AccountName,
		ProxyID:         row.ProxyID,
		ProxyName:       row.ProxyName,
		Model:           row.Model,
		Prompt:          row.Prompt,
		Size:            row.Size,
		Quality:         row.Quality,
		N:               row.N,
		DownloadImage:   row.DownloadImage,
		ResponseFormat:  row.ResponseFormat,
		Enabled:         row.Enabled,
		PublicVisible:   row.PublicVisible,
		PublicName:      row.PublicName,
		IntervalSeconds: row.IntervalSeconds,
		TimeoutSeconds:  row.TimeoutSeconds,
		LastCheckedAt:   row.LastCheckedAt,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func entToServiceImageMonitorHistory(row *dbent.ImageChannelMonitorHistory) *service.ImageChannelMonitorHistoryEntry {
	if row == nil {
		return nil
	}
	return &service.ImageChannelMonitorHistoryEntry{
		ID:               row.ID,
		MonitorID:        row.MonitorID,
		Status:           string(row.Status),
		HTTPStatus:       row.HTTPStatus,
		APIHeaderMs:      row.APIHeaderMs,
		APIBodyMs:        row.APIBodyMs,
		APITotalMs:       row.APITotalMs,
		JSONBytes:        row.JSONBytes,
		HasURL:           row.HasURL,
		HasB64JSON:       row.HasB64JSON,
		ResponseFormat:   row.ResponseFormat,
		ImageURLHost:     row.ImageURLHost,
		ImageFirstByteMs: row.ImageFirstByteMs,
		ImageDownloadMs:  row.ImageDownloadMs,
		ImageBytes:       row.ImageBytes,
		ImageContentType: row.ImageContentType,
		ImageWidth:       row.ImageWidth,
		ImageHeight:      row.ImageHeight,
		ErrorStage:       row.ErrorStage,
		Message:          row.Message,
		CheckedAt:        row.CheckedAt,
	}
}

func assignImageMonitorGeneratedFields(m *service.ImageChannelMonitor, row *dbent.ImageChannelMonitor) {
	if m == nil || row == nil {
		return
	}
	m.ID = row.ID
	m.CreatedAt = row.CreatedAt
	m.UpdatedAt = row.UpdatedAt
	m.LastCheckedAt = row.LastCheckedAt
}
