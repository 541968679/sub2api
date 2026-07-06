package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ImageChannelMonitorHandler struct {
	monitorService *service.ImageChannelMonitorService
}

func NewImageChannelMonitorHandler(
	monitorService *service.ImageChannelMonitorService,
) *ImageChannelMonitorHandler {
	return &ImageChannelMonitorHandler{monitorService: monitorService}
}

type imageChannelMonitorCreateRequest struct {
	Name            string  `json:"name" binding:"required,max=100"`
	SourceType      string  `json:"source_type" binding:"omitempty,oneof=custom account"`
	Endpoint        string  `json:"endpoint" binding:"omitempty,max=500"`
	APIKey          string  `json:"api_key" binding:"omitempty,max=2000"`
	AccountID       *int64  `json:"account_id"`
	ProxyID         *int64  `json:"proxy_id"`
	Model           string  `json:"model" binding:"omitempty,max=200"`
	Prompt          string  `json:"prompt" binding:"omitempty,max=2000"`
	Size            string  `json:"size" binding:"omitempty,max=32"`
	Quality         string  `json:"quality" binding:"omitempty,max=32"`
	N               int     `json:"n" binding:"omitempty,min=1,max=10"`
	DownloadImage   *bool   `json:"download_image"`
	ResponseFormat  *string `json:"response_format" binding:"omitempty,max=16"`
	Enabled         *bool   `json:"enabled"`
	PublicVisible   *bool   `json:"public_visible"`
	PublicName      string  `json:"public_name" binding:"omitempty,max=200"`
	IntervalSeconds int     `json:"interval_seconds" binding:"omitempty,min=15,max=3600"`
	TimeoutSeconds  int     `json:"timeout_seconds" binding:"omitempty,min=30,max=600"`
}

type imageChannelMonitorUpdateRequest struct {
	Name            *string `json:"name" binding:"omitempty,max=100"`
	SourceType      *string `json:"source_type" binding:"omitempty,oneof=custom account"`
	Endpoint        *string `json:"endpoint" binding:"omitempty,max=500"`
	APIKey          *string `json:"api_key" binding:"omitempty,max=2000"`
	AccountID       *int64  `json:"account_id"`
	ProxyID         *int64  `json:"proxy_id"`
	Model           *string `json:"model" binding:"omitempty,max=200"`
	Prompt          *string `json:"prompt" binding:"omitempty,max=2000"`
	Size            *string `json:"size" binding:"omitempty,max=32"`
	Quality         *string `json:"quality" binding:"omitempty,max=32"`
	N               *int    `json:"n" binding:"omitempty,min=1,max=10"`
	DownloadImage   *bool   `json:"download_image"`
	ResponseFormat  *string `json:"response_format" binding:"omitempty,max=16"`
	Enabled         *bool   `json:"enabled"`
	PublicVisible   *bool   `json:"public_visible"`
	PublicName      *string `json:"public_name" binding:"omitempty,max=200"`
	IntervalSeconds *int    `json:"interval_seconds" binding:"omitempty,min=15,max=3600"`
	TimeoutSeconds  *int    `json:"timeout_seconds" binding:"omitempty,min=30,max=600"`
}

type imageChannelMonitorManualTestRequest struct {
	Mode           string `json:"mode" binding:"omitempty,oneof=generate edit"`
	Model          string `json:"model" binding:"omitempty,max=200"`
	Prompt         string `json:"prompt" binding:"omitempty,max=2000"`
	Size           string `json:"size" binding:"omitempty,max=32"`
	Quality        string `json:"quality" binding:"omitempty,max=32"`
	N              int    `json:"n" binding:"omitempty,min=1,max=10"`
	DownloadImage  *bool  `json:"download_image"`
	ResponseFormat string `json:"response_format" binding:"omitempty,max=16"`
	TimeoutSeconds int    `json:"timeout_seconds" binding:"omitempty,min=30,max=600"`
	InputImageData string `json:"input_image_data"`
	InputImageType string `json:"input_image_type" binding:"omitempty,max=100"`
	InputImageName string `json:"input_image_name" binding:"omitempty,max=255"`
}

type imageChannelMonitorResponse struct {
	ID                  int64   `json:"id"`
	Name                string  `json:"name"`
	SourceType          string  `json:"source_type"`
	Endpoint            string  `json:"endpoint"`
	APIKeyMasked        string  `json:"api_key_masked"`
	APIKeyDecryptFailed bool    `json:"api_key_decrypt_failed"`
	AccountID           *int64  `json:"account_id"`
	AccountName         string  `json:"account_name"`
	ProxyID             *int64  `json:"proxy_id"`
	ProxyName           string  `json:"proxy_name"`
	Model               string  `json:"model"`
	Prompt              string  `json:"prompt"`
	Size                string  `json:"size"`
	Quality             string  `json:"quality"`
	N                   int     `json:"n"`
	DownloadImage       bool    `json:"download_image"`
	ResponseFormat      string  `json:"response_format"`
	Enabled             bool    `json:"enabled"`
	PublicVisible       bool    `json:"public_visible"`
	PublicName          string  `json:"public_name"`
	IntervalSeconds     int     `json:"interval_seconds"`
	TimeoutSeconds      int     `json:"timeout_seconds"`
	LastCheckedAt       *string `json:"last_checked_at"`
	CreatedBy           int64   `json:"created_by"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

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

type imageChannelMonitorResultResponse struct {
	MonitorID         int64                              `json:"monitor_id"`
	Status            string                             `json:"status"`
	HTTPStatus        *int                               `json:"http_status"`
	APIHeaderMs       *int                               `json:"api_header_ms"`
	APIBodyMs         *int                               `json:"api_body_ms"`
	APITotalMs        *int                               `json:"api_total_ms"`
	JSONBytes         *int                               `json:"json_bytes"`
	HasURL            bool                               `json:"has_url"`
	HasB64JSON        bool                               `json:"has_b64_json"`
	ResponseFormat    string                             `json:"response_format"`
	ImageURLHost      string                             `json:"image_url_host"`
	ImageFirstByteMs  *int                               `json:"image_first_byte_ms"`
	ImageDownloadMs   *int                               `json:"image_download_ms"`
	ImageBytes        *int64                             `json:"image_bytes"`
	ImageContentType  string                             `json:"image_content_type"`
	ImageWidth        *int                               `json:"image_width"`
	ImageHeight       *int                               `json:"image_height"`
	ErrorStage        string                             `json:"error_stage"`
	Message           string                             `json:"message"`
	CheckedAt         string                             `json:"checked_at"`
	RevisedPrompt     string                             `json:"revised_prompt"`
	ReturnedImageURL  string                             `json:"returned_image_url"`
	ReturnedImageData string                             `json:"returned_image_data"`
	ExitIP            string                             `json:"exit_ip"`
	RequestTargetURL  string                             `json:"request_target_url"`
	RequestTargetHost string                             `json:"request_target_host"`
	RequestTargetIPs  []string                           `json:"request_target_ips"`
	ImageDownloadURL  string                             `json:"image_download_url"`
	ImageDownloadHost string                             `json:"image_download_host"`
	ImageDownloadIPs  []string                           `json:"image_download_ips"`
	Stages            []imageChannelMonitorStageResponse `json:"stages"`
}

type imageChannelMonitorStageResponse struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
	At      string `json:"at"`
}

type imageChannelMonitorManualRunResponse struct {
	RunID       string                             `json:"run_id"`
	Monitor     *imageChannelMonitorResponse       `json:"monitor"`
	Mode        string                             `json:"mode"`
	Running     bool                               `json:"running"`
	Canceled    bool                               `json:"canceled"`
	Stage       string                             `json:"stage"`
	Message     string                             `json:"message"`
	StartedAt   string                             `json:"started_at"`
	UpdatedAt   string                             `json:"updated_at"`
	CompletedAt *string                            `json:"completed_at"`
	Result      *imageChannelMonitorResultResponse `json:"result,omitempty"`
}

type imageChannelMonitorRuntimeStatusResponse struct {
	MonitorID             int64   `json:"monitor_id"`
	Running               bool    `json:"running"`
	Stage                 string  `json:"stage"`
	Message               string  `json:"message"`
	StartedAt             *string `json:"started_at"`
	UpdatedAt             *string `json:"updated_at"`
	CompletedAt           *string `json:"completed_at"`
	NextCheckAt           *string `json:"next_check_at"`
	SecondsUntilNextCheck *int    `json:"seconds_until_next_check"`
}

type imageChannelMonitorHistoryItemResponse struct {
	ID               int64  `json:"id"`
	MonitorID        int64  `json:"monitor_id"`
	Status           string `json:"status"`
	HTTPStatus       *int   `json:"http_status"`
	APIHeaderMs      *int   `json:"api_header_ms"`
	APIBodyMs        *int   `json:"api_body_ms"`
	APITotalMs       *int   `json:"api_total_ms"`
	JSONBytes        *int   `json:"json_bytes"`
	HasURL           bool   `json:"has_url"`
	HasB64JSON       bool   `json:"has_b64_json"`
	ResponseFormat   string `json:"response_format"`
	ImageURLHost     string `json:"image_url_host"`
	ImageFirstByteMs *int   `json:"image_first_byte_ms"`
	ImageDownloadMs  *int   `json:"image_download_ms"`
	ImageBytes       *int64 `json:"image_bytes"`
	ImageContentType string `json:"image_content_type"`
	ImageWidth       *int   `json:"image_width"`
	ImageHeight      *int   `json:"image_height"`
	ErrorStage       string `json:"error_stage"`
	Message          string `json:"message"`
	CheckedAt        string `json:"checked_at"`
}

func imageMonitorToResponse(m *service.ImageChannelMonitor) *imageChannelMonitorResponse {
	if m == nil {
		return nil
	}
	resp := &imageChannelMonitorResponse{
		ID:                  m.ID,
		Name:                m.Name,
		SourceType:          m.SourceType,
		Endpoint:            m.Endpoint,
		APIKeyDecryptFailed: m.APIKeyDecryptFailed,
		AccountID:           m.AccountID,
		AccountName:         m.AccountName,
		ProxyID:             m.ProxyID,
		ProxyName:           m.ProxyName,
		Model:               m.Model,
		Prompt:              m.Prompt,
		Size:                m.Size,
		Quality:             m.Quality,
		N:                   m.N,
		DownloadImage:       m.DownloadImage,
		ResponseFormat:      m.ResponseFormat,
		Enabled:             m.Enabled,
		PublicVisible:       m.PublicVisible,
		PublicName:          m.PublicName,
		IntervalSeconds:     m.IntervalSeconds,
		TimeoutSeconds:      m.TimeoutSeconds,
		CreatedBy:           m.CreatedBy,
		CreatedAt:           m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           m.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if m.SourceType == service.ImageChannelMonitorSourceCustom {
		resp.APIKeyMasked = maskAPIKey(m.APIKey)
	}
	if m.LastCheckedAt != nil {
		s := m.LastCheckedAt.UTC().Format(time.RFC3339)
		resp.LastCheckedAt = &s
	}
	return resp
}

func imageMonitorRuntimeStatusToResponse(
	s *service.ImageChannelMonitorRuntimeStatus,
) imageChannelMonitorRuntimeStatusResponse {
	out := imageChannelMonitorRuntimeStatusResponse{
		MonitorID:             s.MonitorID,
		Running:               s.Running,
		Stage:                 s.Stage,
		Message:               s.Message,
		SecondsUntilNextCheck: s.SecondsUntilNextCheck,
	}
	if s.StartedAt != nil {
		v := s.StartedAt.UTC().Format(time.RFC3339)
		out.StartedAt = &v
	}
	if s.UpdatedAt != nil {
		v := s.UpdatedAt.UTC().Format(time.RFC3339)
		out.UpdatedAt = &v
	}
	if s.CompletedAt != nil {
		v := s.CompletedAt.UTC().Format(time.RFC3339)
		out.CompletedAt = &v
	}
	if s.NextCheckAt != nil {
		v := s.NextCheckAt.UTC().Format(time.RFC3339)
		out.NextCheckAt = &v
	}
	return out
}

func imageMonitorManualRunToResponse(
	s *service.ImageChannelMonitorManualRunStatus,
) imageChannelMonitorManualRunResponse {
	out := imageChannelMonitorManualRunResponse{
		RunID:     s.RunID,
		Monitor:   imageMonitorToResponse(s.Monitor),
		Mode:      s.Mode,
		Running:   s.Running,
		Canceled:  s.Canceled,
		Stage:     s.Stage,
		Message:   s.Message,
		StartedAt: s.StartedAt.UTC().Format(time.RFC3339),
		UpdatedAt: s.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if s.CompletedAt != nil {
		v := s.CompletedAt.UTC().Format(time.RFC3339)
		out.CompletedAt = &v
	}
	if s.Result != nil {
		result := imageMonitorResultToResponse(s.Result)
		out.Result = &result
	}
	return out
}

func imageMonitorResultToResponse(r *service.ImageChannelMonitorResult) imageChannelMonitorResultResponse {
	resp := imageChannelMonitorResultResponse{
		MonitorID:         r.MonitorID,
		Status:            r.Status,
		HTTPStatus:        r.HTTPStatus,
		APIHeaderMs:       r.APIHeaderMs,
		APIBodyMs:         r.APIBodyMs,
		APITotalMs:        r.APITotalMs,
		JSONBytes:         r.JSONBytes,
		HasURL:            r.HasURL,
		HasB64JSON:        r.HasB64JSON,
		ResponseFormat:    r.ResponseFormat,
		ImageURLHost:      r.ImageURLHost,
		ImageFirstByteMs:  r.ImageFirstByteMs,
		ImageDownloadMs:   r.ImageDownloadMs,
		ImageBytes:        r.ImageBytes,
		ImageContentType:  r.ImageContentType,
		ImageWidth:        r.ImageWidth,
		ImageHeight:       r.ImageHeight,
		ErrorStage:        r.ErrorStage,
		Message:           r.Message,
		CheckedAt:         r.CheckedAt.UTC().Format(time.RFC3339),
		RevisedPrompt:     r.RevisedPrompt,
		ReturnedImageURL:  r.ReturnedImageURL,
		ReturnedImageData: r.ReturnedImageData,
		ExitIP:            r.ExitIP,
		RequestTargetURL:  r.RequestTargetURL,
		RequestTargetHost: r.RequestTargetHost,
		RequestTargetIPs:  r.RequestTargetIPs,
		ImageDownloadURL:  r.ImageDownloadURL,
		ImageDownloadHost: r.ImageDownloadHost,
		ImageDownloadIPs:  r.ImageDownloadIPs,
	}
	if len(r.StageEvents) > 0 {
		resp.Stages = make([]imageChannelMonitorStageResponse, 0, len(r.StageEvents))
		for _, event := range r.StageEvents {
			resp.Stages = append(resp.Stages, imageChannelMonitorStageResponse{
				Stage:   event.Stage,
				Message: event.Message,
				At:      event.At.UTC().Format(time.RFC3339),
			})
		}
	}
	return resp
}

func imageMonitorHistoryToResponse(
	e *service.ImageChannelMonitorHistoryEntry,
) imageChannelMonitorHistoryItemResponse {
	return imageChannelMonitorHistoryItemResponse{
		ID:               e.ID,
		MonitorID:        e.MonitorID,
		Status:           e.Status,
		HTTPStatus:       e.HTTPStatus,
		APIHeaderMs:      e.APIHeaderMs,
		APIBodyMs:        e.APIBodyMs,
		APITotalMs:       e.APITotalMs,
		JSONBytes:        e.JSONBytes,
		HasURL:           e.HasURL,
		HasB64JSON:       e.HasB64JSON,
		ResponseFormat:   e.ResponseFormat,
		ImageURLHost:     e.ImageURLHost,
		ImageFirstByteMs: e.ImageFirstByteMs,
		ImageDownloadMs:  e.ImageDownloadMs,
		ImageBytes:       e.ImageBytes,
		ImageContentType: e.ImageContentType,
		ImageWidth:       e.ImageWidth,
		ImageHeight:      e.ImageHeight,
		ErrorStage:       e.ErrorStage,
		Message:          e.Message,
		CheckedAt:        e.CheckedAt.UTC().Format(time.RFC3339),
	}
}

func (h *ImageChannelMonitorHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > monitorMaxPageSize {
		pageSize = monitorMaxPageSize
	}
	items, total, err := h.monitorService.List(c.Request.Context(), service.ImageChannelMonitorListParams{
		Page:       page,
		PageSize:   pageSize,
		SourceType: c.Query("source_type"),
		Enabled:    parseListEnabled(c.Query("enabled")),
		Search:     c.Query("search"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
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
}

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

func (h *ImageChannelMonitorHandler) Get(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	m, err := h.monitorService.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageMonitorToResponse(m))
}

func (h *ImageChannelMonitorHandler) Create(c *gin.Context) {
	var req imageChannelMonitorCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, errors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	downloadImage := true
	if req.DownloadImage != nil {
		downloadImage = *req.DownloadImage
	}
	publicVisible := false
	if req.PublicVisible != nil {
		publicVisible = *req.PublicVisible
	}
	m, err := h.monitorService.Create(c.Request.Context(), service.ImageChannelMonitorCreateParams{
		Name:            req.Name,
		SourceType:      req.SourceType,
		Endpoint:        req.Endpoint,
		APIKey:          req.APIKey,
		AccountID:       req.AccountID,
		ProxyID:         req.ProxyID,
		Model:           req.Model,
		Prompt:          req.Prompt,
		Size:            req.Size,
		Quality:         req.Quality,
		N:               req.N,
		DownloadImage:   downloadImage,
		ResponseFormat:  req.ResponseFormat,
		Enabled:         enabled,
		PublicVisible:   publicVisible,
		PublicName:      req.PublicName,
		IntervalSeconds: req.IntervalSeconds,
		TimeoutSeconds:  req.TimeoutSeconds,
		CreatedBy:       subject.UserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, imageMonitorToResponse(m))
}

func (h *ImageChannelMonitorHandler) Update(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	var req imageChannelMonitorUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, errors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	m, err := h.monitorService.Update(c.Request.Context(), id, service.ImageChannelMonitorUpdateParams{
		Name:            req.Name,
		SourceType:      req.SourceType,
		Endpoint:        req.Endpoint,
		APIKey:          req.APIKey,
		AccountID:       req.AccountID,
		ProxyID:         req.ProxyID,
		Model:           req.Model,
		Prompt:          req.Prompt,
		Size:            req.Size,
		Quality:         req.Quality,
		N:               req.N,
		DownloadImage:   req.DownloadImage,
		ResponseFormat:  req.ResponseFormat,
		Enabled:         req.Enabled,
		PublicVisible:   req.PublicVisible,
		PublicName:      req.PublicName,
		IntervalSeconds: req.IntervalSeconds,
		TimeoutSeconds:  req.TimeoutSeconds,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageMonitorToResponse(m))
}

func (h *ImageChannelMonitorHandler) Delete(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	if err := h.monitorService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *ImageChannelMonitorHandler) Run(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	status, err := h.monitorService.RunCheckAsync(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageMonitorRuntimeStatusToResponse(status))
}

func (h *ImageChannelMonitorHandler) ManualTest(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	var req imageChannelMonitorManualTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, errors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	downloadImage := true
	if req.DownloadImage != nil {
		downloadImage = *req.DownloadImage
	}
	status, err := h.monitorService.StartManualCheck(c.Request.Context(), id, service.ImageChannelMonitorManualTestParams{
		Mode:           req.Mode,
		Model:          req.Model,
		Prompt:         req.Prompt,
		Size:           req.Size,
		Quality:        req.Quality,
		N:              req.N,
		DownloadImage:  downloadImage,
		ResponseFormat: req.ResponseFormat,
		TimeoutSeconds: req.TimeoutSeconds,
		InputImageData: req.InputImageData,
		InputImageType: req.InputImageType,
		InputImageName: req.InputImageName,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageMonitorManualRunToResponse(status))
}

func (h *ImageChannelMonitorHandler) ManualTestStatus(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	runID := strings.TrimSpace(c.Param("runID"))
	if runID == "" {
		response.ErrorFrom(c, errors.BadRequest("VALIDATION_ERROR", "runID is required"))
		return
	}
	status, err := h.monitorService.GetManualCheckStatus(c.Request.Context(), runID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if status.Monitor != nil && status.Monitor.ID != id {
		response.ErrorFrom(c, service.ErrImageChannelMonitorManualRunNotFound)
		return
	}
	response.Success(c, imageMonitorManualRunToResponse(status))
}

func (h *ImageChannelMonitorHandler) CancelManualTest(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	runID := strings.TrimSpace(c.Param("runID"))
	if runID == "" {
		response.ErrorFrom(c, errors.BadRequest("VALIDATION_ERROR", "runID is required"))
		return
	}
	status, err := h.monitorService.GetManualCheckStatus(c.Request.Context(), runID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if status.Monitor != nil && status.Monitor.ID != id {
		response.ErrorFrom(c, service.ErrImageChannelMonitorManualRunNotFound)
		return
	}
	status, err = h.monitorService.CancelManualCheck(c.Request.Context(), runID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageMonitorManualRunToResponse(status))
}

func (h *ImageChannelMonitorHandler) Status(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	status, err := h.monitorService.GetRuntimeStatus(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageMonitorRuntimeStatusToResponse(status))
}

func (h *ImageChannelMonitorHandler) History(c *gin.Context) {
	id, ok := ParseChannelMonitorID(c)
	if !ok {
		return
	}
	limit := MonitorHistoryDefaultLimitFromQuery(c.Query("limit"))
	items, err := h.monitorService.ListHistory(c.Request.Context(), id, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]imageChannelMonitorHistoryItemResponse, 0, len(items))
	for _, e := range items {
		out = append(out, imageMonitorHistoryToResponse(e))
	}
	response.Success(c, out)
}

func MonitorHistoryDefaultLimitFromQuery(raw string) int {
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return limit
}
