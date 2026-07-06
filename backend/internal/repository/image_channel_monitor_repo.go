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
