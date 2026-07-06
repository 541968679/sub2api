package handler

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ImageChannelMonitorUserHandler 图片渠道监控用户只读 handler。
// 只暴露 public_visible=true 的渠道;DTO 为安全白名单(见同名 _test.go),
// 绝不下发内部渠道名/endpoint/host/IP/错误消息等敏感信息。
type ImageChannelMonitorUserHandler struct {
	monitorService *service.ImageChannelMonitorService
	settingService *service.SettingService
}

// NewImageChannelMonitorUserHandler 创建 handler。
// settingService 用于每次请求前读取功能开关(与原生渠道监控共用 channel_monitor_enabled)。
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
	// 复用 admin.ParseChannelMonitorID 保持错误码与日志一致。
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
