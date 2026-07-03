package admin

import (
	"strconv"
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
	Name            string `json:"name" binding:"required,max=100"`
	SourceType      string `json:"source_type" binding:"omitempty,oneof=custom account"`
	Endpoint        string `json:"endpoint" binding:"omitempty,max=500"`
	APIKey          string `json:"api_key" binding:"omitempty,max=2000"`
	AccountID       *int64 `json:"account_id"`
	Model           string `json:"model" binding:"omitempty,max=200"`
	Prompt          string `json:"prompt" binding:"omitempty,max=2000"`
	Size            string `json:"size" binding:"omitempty,max=32"`
	Quality         string `json:"quality" binding:"omitempty,max=32"`
	N               int    `json:"n" binding:"omitempty,min=1,max=10"`
	DownloadImage   *bool  `json:"download_image"`
	Enabled         *bool  `json:"enabled"`
	IntervalSeconds int    `json:"interval_seconds" binding:"omitempty,min=15,max=3600"`
	TimeoutSeconds  int    `json:"timeout_seconds" binding:"omitempty,min=30,max=600"`
}

type imageChannelMonitorUpdateRequest struct {
	Name            *string `json:"name" binding:"omitempty,max=100"`
	SourceType      *string `json:"source_type" binding:"omitempty,oneof=custom account"`
	Endpoint        *string `json:"endpoint" binding:"omitempty,max=500"`
	APIKey          *string `json:"api_key" binding:"omitempty,max=2000"`
	AccountID       *int64  `json:"account_id"`
	Model           *string `json:"model" binding:"omitempty,max=200"`
	Prompt          *string `json:"prompt" binding:"omitempty,max=2000"`
	Size            *string `json:"size" binding:"omitempty,max=32"`
	Quality         *string `json:"quality" binding:"omitempty,max=32"`
	N               *int    `json:"n" binding:"omitempty,min=1,max=10"`
	DownloadImage   *bool   `json:"download_image"`
	Enabled         *bool   `json:"enabled"`
	IntervalSeconds *int    `json:"interval_seconds" binding:"omitempty,min=15,max=3600"`
	TimeoutSeconds  *int    `json:"timeout_seconds" binding:"omitempty,min=30,max=600"`
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
	Model               string  `json:"model"`
	Prompt              string  `json:"prompt"`
	Size                string  `json:"size"`
	Quality             string  `json:"quality"`
	N                   int     `json:"n"`
	DownloadImage       bool    `json:"download_image"`
	Enabled             bool    `json:"enabled"`
	IntervalSeconds     int     `json:"interval_seconds"`
	TimeoutSeconds      int     `json:"timeout_seconds"`
	LastCheckedAt       *string `json:"last_checked_at"`
	CreatedBy           int64   `json:"created_by"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

type imageChannelMonitorResultResponse struct {
	MonitorID         int64  `json:"monitor_id"`
	Status            string `json:"status"`
	HTTPStatus        *int   `json:"http_status"`
	APIHeaderMs       *int   `json:"api_header_ms"`
	APIBodyMs         *int   `json:"api_body_ms"`
	APITotalMs        *int   `json:"api_total_ms"`
	JSONBytes         *int   `json:"json_bytes"`
	HasURL            bool   `json:"has_url"`
	HasB64JSON        bool   `json:"has_b64_json"`
	ImageURLHost      string `json:"image_url_host"`
	ImageFirstByteMs  *int   `json:"image_first_byte_ms"`
	ImageDownloadMs   *int   `json:"image_download_ms"`
	ImageBytes        *int64 `json:"image_bytes"`
	ImageContentType  string `json:"image_content_type"`
	ImageWidth        *int   `json:"image_width"`
	ImageHeight       *int   `json:"image_height"`
	ErrorStage        string `json:"error_stage"`
	Message           string `json:"message"`
	CheckedAt         string `json:"checked_at"`
	RevisedPrompt     string `json:"revised_prompt"`
	ReturnedImageURL  string `json:"returned_image_url"`
	ReturnedImageData string `json:"returned_image_data"`
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
		Model:               m.Model,
		Prompt:              m.Prompt,
		Size:                m.Size,
		Quality:             m.Quality,
		N:                   m.N,
		DownloadImage:       m.DownloadImage,
		Enabled:             m.Enabled,
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

func imageMonitorResultToResponse(r *service.ImageChannelMonitorResult) imageChannelMonitorResultResponse {
	return imageChannelMonitorResultResponse{
		MonitorID:         r.MonitorID,
		Status:            r.Status,
		HTTPStatus:        r.HTTPStatus,
		APIHeaderMs:       r.APIHeaderMs,
		APIBodyMs:         r.APIBodyMs,
		APITotalMs:        r.APITotalMs,
		JSONBytes:         r.JSONBytes,
		HasURL:            r.HasURL,
		HasB64JSON:        r.HasB64JSON,
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
	}
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
	out := make([]*imageChannelMonitorResponse, 0, len(items))
	for _, m := range items {
		out = append(out, imageMonitorToResponse(m))
	}
	response.Paginated(c, out, total, page, pageSize)
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
	m, err := h.monitorService.Create(c.Request.Context(), service.ImageChannelMonitorCreateParams{
		Name:            req.Name,
		SourceType:      req.SourceType,
		Endpoint:        req.Endpoint,
		APIKey:          req.APIKey,
		AccountID:       req.AccountID,
		Model:           req.Model,
		Prompt:          req.Prompt,
		Size:            req.Size,
		Quality:         req.Quality,
		N:               req.N,
		DownloadImage:   downloadImage,
		Enabled:         enabled,
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
		Model:           req.Model,
		Prompt:          req.Prompt,
		Size:            req.Size,
		Quality:         req.Quality,
		N:               req.N,
		DownloadImage:   req.DownloadImage,
		Enabled:         req.Enabled,
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
	result, err := h.monitorService.RunCheck(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageMonitorResultToResponse(result))
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
