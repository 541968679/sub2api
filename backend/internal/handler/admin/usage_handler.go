package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UsageHandler handles admin usage-related requests
type UsageHandler struct {
	usageService            *service.UsageService
	apiKeyService           *service.APIKeyService
	adminService            service.AdminService
	cleanupService          *service.UsageCleanupService
	modelPricingService     *service.GlobalModelPricingService
	userModelPricingService *service.UserModelPricingService
	creditSnapshotService   *service.CreditSnapshotService
}

// NewUsageHandler creates a new admin usage handler
func NewUsageHandler(
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	adminService service.AdminService,
	cleanupService *service.UsageCleanupService,
	modelPricingService *service.GlobalModelPricingService,
	userModelPricingService *service.UserModelPricingService,
	creditSnapshotService *service.CreditSnapshotService,
) *UsageHandler {
	return &UsageHandler{
		usageService:            usageService,
		apiKeyService:           apiKeyService,
		adminService:            adminService,
		cleanupService:          cleanupService,
		modelPricingService:     modelPricingService,
		userModelPricingService: userModelPricingService,
		creditSnapshotService:   creditSnapshotService,
	}
}

// CreateUsageCleanupTaskRequest represents cleanup task creation request
type CreateUsageCleanupTaskRequest struct {
	StartDate   string  `json:"start_date"`
	EndDate     string  `json:"end_date"`
	UserID      *int64  `json:"user_id"`
	APIKeyID    *int64  `json:"api_key_id"`
	AccountID   *int64  `json:"account_id"`
	GroupID     *int64  `json:"group_id"`
	Model       *string `json:"model"`
	RequestType *string `json:"request_type"`
	Stream      *bool   `json:"stream"`
	BillingType *int8   `json:"billing_type"`
	Timezone    string  `json:"timezone"`
}

// List handles listing all usage records with filters
// GET /api/v1/admin/usage
func (h *UsageHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	exactTotal := false
	if exactTotalRaw := strings.TrimSpace(c.Query("exact_total")); exactTotalRaw != "" {
		parsed, err := strconv.ParseBool(exactTotalRaw)
		if err != nil {
			response.BadRequest(c, "Invalid exact_total value, use true or false")
			return
		}
		exactTotal = parsed
	}

	// Parse filters
	var userID, apiKeyID, accountID, groupID int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		apiKeyID = id
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		id, err := strconv.ParseInt(accountIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		accountID = id
	}

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = id
	}

	model := c.Query("model")
	billingMode := strings.TrimSpace(c.Query("billing_mode"))

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
		stream = &val
	}

	var billingType *int8
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
		bt := int8(val)
		billingType = &bt
	}

	// Parse date range
	var startTime, endTime *time.Time
	userTZ := c.Query("timezone") // Get user's timezone from request
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		startTime = &t
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		// Use half-open range [start, end), move to next calendar day start (DST-safe).
		t = t.AddDate(0, 0, 1)
		endTime = &t
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filters := usagestats.UsageLogFilters{
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   accountID,
		GroupID:     groupID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
		BillingMode: billingMode,
		StartTime:   startTime,
		EndTime:     endTime,
		ExactTotal:  exactTotal,
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	displayMap := h.loadDisplayPricingMap(c)
	out := make([]dto.AdminUsageLog, 0, len(records))
	for i := range records {
		out = append(out, *dto.UsageLogFromServiceAdmin(&records[i], displayMap))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// Stats handles getting usage statistics with filters
// GET /api/v1/admin/usage/stats
func (h *UsageHandler) Stats(c *gin.Context) {
	// Parse filters - same as List endpoint
	var userID, apiKeyID, accountID, groupID int64
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		apiKeyID = id
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		id, err := strconv.ParseInt(accountIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		accountID = id
	}

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = id
	}

	model := c.Query("model")
	billingMode := strings.TrimSpace(c.Query("billing_mode"))

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := c.Query("stream"); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return
		}
		stream = &val
	}

	var billingType *int8
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return
		}
		bt := int8(val)
		billingType = &bt
	}

	// Parse date range
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	var startTime, endTime time.Time

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr != "" && endDateStr != "" {
		var err error
		startTime, err = timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		endTime, err = timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		// 与 SQL 条件 created_at < end 对齐，使用次日 00:00 作为上边界（DST-safe）。
		endTime = endTime.AddDate(0, 0, 1)
	} else {
		period := c.DefaultQuery("period", "today")
		switch period {
		case "today":
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		case "week":
			startTime = now.AddDate(0, 0, -7)
		case "month":
			startTime = now.AddDate(0, -1, 0)
		default:
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		}
		endTime = now
	}

	// Build filters and call GetStatsWithFilters
	filters := usagestats.UsageLogFilters{
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   accountID,
		GroupID:     groupID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
		BillingMode: billingMode,
		StartTime:   &startTime,
		EndTime:     &endTime,
	}

	stats, err := h.usageService.GetStatsWithFilters(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// parseStatsDateRange 与 Stats() 保持一致的 start/end 解析逻辑，便于 antigravity 接口复用。
// 当未传 start_date/end_date 时，按 period 参数回落（today/week/month）。
func parseStatsDateRange(c *gin.Context) (time.Time, time.Time, bool) {
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	var startTime, endTime time.Time
	if startDateStr != "" && endDateStr != "" {
		var err error
		startTime, err = timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		endTime, err = timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		endTime = endTime.AddDate(0, 0, 1)
	} else {
		period := c.DefaultQuery("period", "today")
		switch period {
		case "today":
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		case "week":
			startTime = now.AddDate(0, 0, -7)
		case "month":
			startTime = now.AddDate(0, -1, 0)
		default:
			startTime = timezone.StartOfDayInUserLocation(now, userTZ)
		}
		endTime = now
	}
	return startTime, endTime, true
}

// StatsAntigravity 返回 antigravity 平台时间窗内的 credits 消耗 / 额度 / 调用次数 / 派生比率。
// GET /api/v1/admin/usage/stats/antigravity
func (h *UsageHandler) StatsAntigravity(c *gin.Context) {
	if h.creditSnapshotService == nil {
		response.InternalError(c, "credit snapshot service not configured")
		return
	}
	startTime, endTime, ok := parseStatsDateRange(c)
	if !ok {
		return
	}
	result, err := h.creditSnapshotService.GetAntigravityUsageRatio(c.Request.Context(), startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// RefreshAntigravityStats 手动触发一次 credits 余额采样，再返回最新聚合结果。
// 30 秒内重复请求会被节流（throttled），响应里回显 manual_refresh_throttled=true。
// POST /api/v1/admin/usage/stats/antigravity/refresh
func (h *UsageHandler) RefreshAntigravityStats(c *gin.Context) {
	if h.creditSnapshotService == nil {
		response.InternalError(c, "credit snapshot service not configured")
		return
	}
	startTime, endTime, ok := parseStatsDateRange(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	_, throttled, err := h.creditSnapshotService.TriggerManualCapture(ctx)
	if err != nil {
		logger.LegacyPrintf("handler.admin.usage", "[Usage] manual credit snapshot failed: %v", err)
	}
	result, err := h.creditSnapshotService.GetAntigravityUsageRatio(ctx, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if throttled {
		result.ManualRefreshThrottled = true
	}
	response.Success(c, result)
}

// SearchUsers handles searching users by email keyword
// GET /api/v1/admin/usage/search-users
func (h *UsageHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		response.Success(c, []any{})
		return
	}

	// Limit to 30 results
	users, _, err := h.adminService.ListUsers(c.Request.Context(), 1, 30, service.UserListFilters{Search: keyword}, "email", "asc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return simplified user list (only id and email)
	type SimpleUser struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	}

	result := make([]SimpleUser, len(users))
	for i, u := range users {
		result[i] = SimpleUser{
			ID:    u.ID,
			Email: u.Email,
		}
	}

	response.Success(c, result)
}

// SearchAPIKeys handles searching API keys by user
// GET /api/v1/admin/usage/search-api-keys
func (h *UsageHandler) SearchAPIKeys(c *gin.Context) {
	userIDStr := c.Query("user_id")
	keyword := c.Query("q")

	var userID int64
	if userIDStr != "" {
		id, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = id
	}

	keys, err := h.apiKeyService.SearchAPIKeys(c.Request.Context(), userID, keyword, 30)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Return simplified API key list (only id and name)
	type SimpleAPIKey struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		UserID int64  `json:"user_id"`
	}

	result := make([]SimpleAPIKey, len(keys))
	for i, k := range keys {
		result[i] = SimpleAPIKey{
			ID:     k.ID,
			Name:   k.Name,
			UserID: k.UserID,
		}
	}

	response.Success(c, result)
}

// ListCleanupTasks handles listing usage cleanup tasks
// GET /api/v1/admin/usage/cleanup-tasks
func (h *UsageHandler) ListCleanupTasks(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	operator := int64(0)
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		operator = subject.UserID
	}
	page, pageSize := response.ParsePagination(c)
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求清理任务列表: operator=%d page=%d page_size=%d", operator, page, pageSize)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	tasks, result, err := h.cleanupService.ListTasks(c.Request.Context(), params)
	if err != nil {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 查询清理任务列表失败: operator=%d page=%d page_size=%d err=%v", operator, page, pageSize, err)
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.UsageCleanupTask, 0, len(tasks))
	for i := range tasks {
		out = append(out, *dto.UsageCleanupTaskFromService(&tasks[i]))
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 返回清理任务列表: operator=%d total=%d items=%d page=%d page_size=%d", operator, result.Total, len(out), page, pageSize)
	response.Paginated(c, out, result.Total, page, pageSize)
}

// CreateCleanupTask handles creating a usage cleanup task
// POST /api/v1/admin/usage/cleanup-tasks
func (h *UsageHandler) CreateCleanupTask(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}

	var req CreateUsageCleanupTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	req.StartDate = strings.TrimSpace(req.StartDate)
	req.EndDate = strings.TrimSpace(req.EndDate)
	if req.StartDate == "" || req.EndDate == "" {
		response.BadRequest(c, "start_date and end_date are required")
		return
	}

	startTime, err := timezone.ParseInUserLocation("2006-01-02", req.StartDate, req.Timezone)
	if err != nil {
		response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
		return
	}
	endTime, err := timezone.ParseInUserLocation("2006-01-02", req.EndDate, req.Timezone)
	if err != nil {
		response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
		return
	}
	endTime = endTime.Add(24*time.Hour - time.Nanosecond)

	var requestType *int16
	stream := req.Stream
	if req.RequestType != nil {
		parsed, err := service.ParseUsageRequestType(*req.RequestType)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		value := int16(parsed)
		requestType = &value
		stream = nil
	}

	filters := service.UsageCleanupFilters{
		StartTime:   startTime,
		EndTime:     endTime,
		UserID:      req.UserID,
		APIKeyID:    req.APIKeyID,
		AccountID:   req.AccountID,
		GroupID:     req.GroupID,
		Model:       req.Model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: req.BillingType,
	}

	var userID any
	if filters.UserID != nil {
		userID = *filters.UserID
	}
	var apiKeyID any
	if filters.APIKeyID != nil {
		apiKeyID = *filters.APIKeyID
	}
	var accountID any
	if filters.AccountID != nil {
		accountID = *filters.AccountID
	}
	var groupID any
	if filters.GroupID != nil {
		groupID = *filters.GroupID
	}
	var model any
	if filters.Model != nil {
		model = *filters.Model
	}
	var streamValue any
	if filters.Stream != nil {
		streamValue = *filters.Stream
	}
	var requestTypeName any
	if filters.RequestType != nil {
		requestTypeName = service.RequestTypeFromInt16(*filters.RequestType).String()
	}
	var billingType any
	if filters.BillingType != nil {
		billingType = *filters.BillingType
	}

	idempotencyPayload := struct {
		OperatorID int64                         `json:"operator_id"`
		Body       CreateUsageCleanupTaskRequest `json:"body"`
	}{
		OperatorID: subject.UserID,
		Body:       req,
	}
	executeAdminIdempotentJSON(c, "admin.usage.cleanup_tasks.create", idempotencyPayload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求创建清理任务: operator=%d start=%s end=%s user_id=%v api_key_id=%v account_id=%v group_id=%v model=%v request_type=%v stream=%v billing_type=%v tz=%q",
			subject.UserID,
			filters.StartTime.Format(time.RFC3339),
			filters.EndTime.Format(time.RFC3339),
			userID,
			apiKeyID,
			accountID,
			groupID,
			model,
			requestTypeName,
			streamValue,
			billingType,
			req.Timezone,
		)

		task, err := h.cleanupService.CreateTask(ctx, filters, subject.UserID)
		if err != nil {
			logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 创建清理任务失败: operator=%d err=%v", subject.UserID, err)
			return nil, err
		}
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 清理任务已创建: task=%d operator=%d status=%s", task.ID, subject.UserID, task.Status)
		return dto.UsageCleanupTaskFromService(task), nil
	})
}

// CancelCleanupTask handles canceling a usage cleanup task
// POST /api/v1/admin/usage/cleanup-tasks/:id/cancel
func (h *UsageHandler) CancelCleanupTask(c *gin.Context) {
	if h.cleanupService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Usage cleanup service unavailable")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Unauthorized")
		return
	}
	idStr := strings.TrimSpace(c.Param("id"))
	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || taskID <= 0 {
		response.BadRequest(c, "Invalid task id")
		return
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 请求取消清理任务: task=%d operator=%d", taskID, subject.UserID)
	if err := h.cleanupService.CancelTask(c.Request.Context(), taskID, subject.UserID); err != nil {
		logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 取消清理任务失败: task=%d operator=%d err=%v", taskID, subject.UserID, err)
		response.ErrorFrom(c, err)
		return
	}
	logger.LegacyPrintf("handler.admin.usage", "[UsageCleanup] 清理任务已取消: task=%d operator=%d", taskID, subject.UserID)
	response.Success(c, gin.H{"id": taskID, "status": service.UsageCleanupStatusCanceled})
}

func (h *UsageHandler) loadDisplayPricingMap(c *gin.Context) dto.DisplayPricingMap {
	if h.modelPricingService == nil {
		return nil
	}
	pricings, err := h.modelPricingService.GetAllEnabledPricings(c.Request.Context())
	if err != nil {
		return nil
	}
	return dto.BuildDisplayPricingMap(pricings)
}

// UserViewSnapshot is one column of the side-by-side comparison.
type UserViewSnapshot struct {
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	InputCost           float64 `json:"input_cost"`
	OutputCost          float64 `json:"output_cost"`
	CacheReadCost       float64 `json:"cache_read_cost"`
	CacheCreationCost   float64 `json:"cache_creation_cost"`
	TotalCost           float64 `json:"total_cost"`
	ActualCost          float64 `json:"actual_cost"`
	RateMultiplier      float64 `json:"rate_multiplier"`
}

// UserViewConfigUsed describes which display-pricing inputs produced the user_view column.
type UserViewConfigUsed struct {
	DisplayInputPrice     *float64 `json:"display_input_price"`
	DisplayOutputPrice    *float64 `json:"display_output_price"`
	DisplayCacheReadPrice *float64 `json:"display_cache_read_price"`
	CacheTransferRatio    *float64 `json:"cache_transfer_ratio"`
	UserGroupRate         *float64 `json:"user_group_rate"`
	HasUserOverride       bool     `json:"has_user_override"`
	GroupID               *int64   `json:"group_id"`
}

// UserViewPreviewResponse is the payload returned to the admin compare drawer.
type UserViewPreviewResponse struct {
	LogID      int64              `json:"log_id"`
	UserID     int64              `json:"user_id"`
	Model      string             `json:"model"`
	Real       UserViewSnapshot   `json:"real"`
	UserView   UserViewSnapshot   `json:"user_view"`
	ConfigUsed UserViewConfigUsed `json:"config_used"`
}

// GetUserViewPreview computes "what the owning user sees in their own /usage page" for a single
// usage log row, by re-running the three layers of display transform (global pricing →
// user model overrides → user group display rate) on a clone of the row.
// GET /api/v1/admin/usage/:id/user-view
func (h *UsageHandler) GetUserViewPreview(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid log id")
		return
	}

	ctx := c.Request.Context()
	log, err := h.usageService.GetByID(ctx, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if log == nil {
		response.NotFound(c, "Usage log not found")
		return
	}

	// Layer 1: global display pricing
	globalMap := h.loadDisplayPricingMap(c)

	// Layer 2: user model overrides — merge on top of global
	var userOverrides []service.UserModelPricingOverride
	if h.userModelPricingService != nil {
		userOverrides, _ = h.userModelPricingService.GetEnabledByUserID(ctx, log.UserID)
	}
	userMap := globalMap
	if len(userOverrides) > 0 {
		userMap = dto.BuildUserDisplayPricingMap(globalMap, userOverrides)
	}

	// Layer 3: per-user group display rate multiplier
	var groupRates map[int64]service.UserGroupRateData
	if h.apiKeyService != nil {
		groupRates, _ = h.apiKeyService.GetUserGroupRatesFull(ctx, log.UserID)
	}

	// Real column: no displayMap → no transform
	realDTO := dto.UsageLogFromService(log, nil)
	// User view column: apply global+user override (in-place on a fresh DTO),
	// then layer the user group display rate if present.
	userDTO := dto.UsageLogFromService(log, userMap)
	var groupDisplayRate *float64
	if log.GroupID != nil && groupRates != nil {
		if dr, ok := groupRates[*log.GroupID]; ok && dr.DisplayRateMultiplier != nil {
			dto.ApplyUserDisplayRate(userDTO, *dr.DisplayRateMultiplier)
			groupDisplayRate = dr.DisplayRateMultiplier
		}
	}

	hasUserOverride := false
	for i := range userOverrides {
		if strings.EqualFold(userOverrides[i].Model, log.Model) {
			hasUserOverride = true
			break
		}
	}

	cfg := UserViewConfigUsed{
		HasUserOverride: hasUserOverride,
		UserGroupRate:   groupDisplayRate,
		GroupID:         log.GroupID,
	}
	if userMap != nil {
		// display_pricing.toLowerModel is unexported; map keys are lowercased model names.
		if entry, ok := userMap[strings.ToLower(log.Model)]; ok && entry != nil {
			cfg.DisplayInputPrice = entry.DisplayInputPrice
			cfg.DisplayOutputPrice = entry.DisplayOutputPrice
			cfg.DisplayCacheReadPrice = entry.DisplayCacheReadPrice
			cfg.CacheTransferRatio = entry.CacheTransferRatio
		}
	}

	resp := UserViewPreviewResponse{
		LogID:      log.ID,
		UserID:     log.UserID,
		Model:      log.Model,
		Real:       snapshotFromDTO(realDTO),
		UserView:   snapshotFromDTO(userDTO),
		ConfigUsed: cfg,
	}
	response.Success(c, resp)
}

func snapshotFromDTO(d *dto.UsageLog) UserViewSnapshot {
	if d == nil {
		return UserViewSnapshot{}
	}
	return UserViewSnapshot{
		InputTokens:         d.InputTokens,
		OutputTokens:        d.OutputTokens,
		CacheReadTokens:     d.CacheReadTokens,
		CacheCreationTokens: d.CacheCreationTokens,
		InputCost:           d.InputCost,
		OutputCost:          d.OutputCost,
		CacheReadCost:       d.CacheReadCost,
		CacheCreationCost:   d.CacheCreationCost,
		TotalCost:           d.TotalCost,
		ActualCost:          d.ActualCost,
		RateMultiplier:      d.RateMultiplier,
	}
}
