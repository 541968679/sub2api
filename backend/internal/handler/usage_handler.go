package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UsageHandler handles usage-related requests
type UsageHandler struct {
	usageService            *service.UsageService
	apiKeyService           *service.APIKeyService
	modelPricingService     *service.GlobalModelPricingService
	userModelPricingService *service.UserModelPricingService
	opsService              *service.OpsService
	settingService          *service.SettingService
}

// NewUsageHandler creates a new UsageHandler
func NewUsageHandler(
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	modelPricingService *service.GlobalModelPricingService,
	userModelPricingService *service.UserModelPricingService,
	opsService *service.OpsService,
	settingService *service.SettingService,
) *UsageHandler {
	return &UsageHandler{
		usageService:            usageService,
		apiKeyService:           apiKeyService,
		modelPricingService:     modelPricingService,
		userModelPricingService: userModelPricingService,
		opsService:              opsService,
		settingService:          settingService,
	}
}

// List handles listing usage records with pagination
// GET /api/v1/usage
func (h *UsageHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)

	var apiKeyID int64
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}

		// [Security Fix] Verify API Key ownership to prevent horizontal privilege escalation
		apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if apiKey.UserID != subject.UserID {
			response.Forbidden(c, "Not authorized to access this API key's usage records")
			return
		}

		apiKeyID = id
	}

	// Parse additional filters
	model := c.Query("model")

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
		UserID:      subject.UserID, // Always filter by current user for security
		APIKeyID:    apiKeyID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
		StartTime:   startTime,
		EndTime:     endTime,
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	displayMap := h.loadDisplayPricingMapForUser(c, subject.UserID)
	userDisplayRates := h.loadUserDisplayRates(c, subject.UserID)
	out := make([]dto.UsageLog, 0, len(records))
	for i := range records {
		out = append(out, *displayUsageRecordForUser(&records[i], displayMap, userDisplayRates))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// ListErrors handles listing the current user's failed requests.
// GET /api/v1/usage/errors
func (h *UsageHandler) ListErrors(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.settingService == nil || !h.settingService.IsUserErrorViewAllowed(c.Request.Context()) {
		response.Forbidden(c, "Error requests view is disabled")
		return
	}
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}

	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	filter := &service.OpsErrorLogFilter{Page: page, PageSize: pageSize}

	userTZ := c.Query("timezone")
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return
		}
		filter.StartTime = &t
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return
		}
		t = t.AddDate(0, 0, 1)
		filter.EndTime = &t
	}
	filter.Model = strings.TrimSpace(c.Query("model"))

	if raw := strings.TrimSpace(c.Query("api_key_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 0 {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		if id > 0 {
			filter.APIKeyID = &id
		}
	}
	if raw := strings.TrimSpace(c.Query("status_code")); raw != "" {
		code, err := strconv.Atoi(raw)
		if err != nil || code < 0 {
			response.BadRequest(c, "Invalid status_code")
			return
		}
		filter.StatusCodes = []int{code}
	}
	if category := strings.TrimSpace(c.Query("category")); category != "" {
		phases, types := service.CategoryToFilter(category)
		filter.ErrorPhasesAny = phases
		filter.ErrorTypesAny = types
	}

	result, err := h.opsService.ListUserErrorRequests(c.Request.Context(), subject.UserID, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, result.Items, int64(result.Total), result.Page, result.PageSize)
}

// GetErrorDetail handles fetching one of the current user's failed request details.
// GET /api/v1/usage/errors/:id
func (h *UsageHandler) GetErrorDetail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.settingService == nil || !h.settingService.IsUserErrorViewAllowed(c.Request.Context()) {
		response.Forbidden(c, "Error requests view is disabled")
		return
	}
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	detail, err := h.opsService.GetUserErrorRequestDetail(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

// PublicRecords lists usage records for the API key used to authenticate the request.
// GET /v1/usage/records
func (h *UsageHandler) PublicRecords(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		response.Unauthorized(c, "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Invalid API key")
		return
	}

	page, pageSize := response.ParsePagination(c)

	model := c.Query("model")

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

	startTime, endTime, ok := parseUsageDateRangeQuery(c)
	if !ok {
		return
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filters := usagestats.UsageLogFilters{
		UserID:      subject.UserID,
		APIKeyID:    apiKey.ID,
		Model:       model,
		RequestType: requestType,
		Stream:      stream,
		BillingType: billingType,
		StartTime:   &startTime,
		EndTime:     &endTime,
		ExactTotal:  true,
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	displayMap := h.loadDisplayPricingMapForUser(c, subject.UserID)
	userDisplayRates := h.loadUserDisplayRates(c, subject.UserID)
	out := make([]dto.UsageLog, 0, len(records))
	for i := range records {
		out = append(out, *displayUsageRecordForUser(&records[i], displayMap, userDisplayRates))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// PublicStats returns selected-range usage statistics for the API key used to authenticate the request.
// GET /v1/usage/stats
func (h *UsageHandler) PublicStats(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		response.Unauthorized(c, "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Invalid API key")
		return
	}

	startTime, endTime, ok := parseUsageDateRangeQuery(c)
	if !ok {
		return
	}

	records, err := h.loadAllDisplayedPublicUsageRecords(c, subject.UserID, apiKey.ID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, aggregateDisplayedPublicUsageStats(records))
}

// PublicTrend returns selected-range usage trend data for the API key used to authenticate the request.
// GET /v1/usage/trend
func (h *UsageHandler) PublicTrend(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		response.Unauthorized(c, "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Invalid API key")
		return
	}

	startTime, endTime, ok := parseUsageDateRangeQuery(c)
	if !ok {
		return
	}
	granularity := c.DefaultQuery("granularity", "day")

	records, err := h.loadAllDisplayedPublicUsageRecords(c, subject.UserID, apiKey.ID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	trend := aggregateDisplayedPublicUsageTrend(records, granularity)

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.AddDate(0, 0, -1).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// GetByID handles getting a single usage record
// GET /api/v1/usage/:id
func (h *UsageHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	usageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid usage ID")
		return
	}

	record, err := h.usageService.GetByID(c.Request.Context(), usageID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 验证所有权
	if record.UserID != subject.UserID {
		response.Forbidden(c, "Not authorized to access this record")
		return
	}

	u := dto.UsageLogFromService(record, h.loadDisplayPricingMapForUser(c, subject.UserID))
	if record.GroupID != nil {
		userDisplayRates := h.loadUserDisplayRates(c, subject.UserID)
		if userDisplayRates != nil {
			if dr, ok := userDisplayRates[*record.GroupID]; ok && dr.DisplayRateMultiplier != nil {
				dto.ApplyUserDisplayRate(u, *dr.DisplayRateMultiplier)
			}
		}
	}
	response.Success(c, u)
}

// Stats handles getting usage statistics
// GET /api/v1/usage/stats
func (h *UsageHandler) Stats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var apiKeyID int64
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}

		// [Security Fix] Verify API Key ownership to prevent horizontal privilege escalation
		apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.NotFound(c, "API key not found")
			return
		}
		if apiKey.UserID != subject.UserID {
			response.Forbidden(c, "Not authorized to access this API key's statistics")
			return
		}

		apiKeyID = id
	}

	// 获取时间范围参数
	userTZ := c.Query("timezone") // Get user's timezone from request
	now := timezone.NowInUserLocation(userTZ)
	var startTime, endTime time.Time

	// 优先使用 start_date 和 end_date 参数
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr != "" && endDateStr != "" {
		// 使用自定义日期范围
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
		// 使用 period 参数
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

	// Aggregate from the same display-transformed records the user sees in the records
	// list, so the stat cards show display values (never real tokens/prices) and reconcile
	// exactly with the records table for the selected range.
	records, err := h.loadAllDisplayedPublicUsageRecords(c, subject.UserID, apiKeyID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, aggregateDisplayedPublicUsageStats(records))
}

// parseUserTimeRange parses start_date, end_date query parameters for user dashboard
// Uses user's timezone if provided, otherwise falls back to server timezone
func parseUserTimeRange(c *gin.Context) (time.Time, time.Time) {
	userTZ := c.Query("timezone") // Get user's timezone from request
	now := timezone.NowInUserLocation(userTZ)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var startTime, endTime time.Time

	if startDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ); err == nil {
			startTime = t
		} else {
			startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
		}
	} else {
		startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
	}

	if endDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ); err == nil {
			endTime = t.Add(24 * time.Hour) // Include the end date
		} else {
			endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
		}
	} else {
		endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	}

	return startTime, endTime
}

func parseUsageDateRangeQuery(c *gin.Context) (time.Time, time.Time, bool) {
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	startTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -6), userTZ)
	endTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date format, use YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		startTime = t
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date format, use YYYY-MM-DD")
			return time.Time{}, time.Time{}, false
		}
		endTime = t.AddDate(0, 0, 1)
	}

	return startTime, endTime, true
}

// DashboardStats handles getting user dashboard statistics
// GET /api/v1/usage/dashboard/stats
func (h *UsageHandler) DashboardStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	stats, err := h.usageService.GetUserDashboardStats(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Override raw token/cost totals with user-facing display values so the dashboard never
	// exposes real token counts or unit prices. API-key counts and RPM/TPM are preserved;
	// actual_cost is unchanged by the display transform. The all-time totals use per-group
	// aggregation (unbounded range — loading every row is infeasible for heavy users).
	displayMap := h.loadDisplayPricingMapForUser(c, subject.UserID)
	userDisplayRates := h.loadUserDisplayRates(c, subject.UserID)

	allTime, err := h.userDashboardDisplayTotals(c, subject.UserID, displayMap, userDisplayRates, nil, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	today := timezone.Today()
	todayEnd := today.AddDate(0, 0, 1)
	todayTotals, err := h.userDashboardDisplayTotals(c, subject.UserID, displayMap, userDisplayRates, &today, &todayEnd)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	stats.TotalRequests = allTime.Requests
	stats.TotalInputTokens = allTime.InputTokens
	stats.TotalOutputTokens = allTime.OutputTokens
	stats.TotalCacheCreationTokens = allTime.CacheCreationTokens
	stats.TotalCacheReadTokens = allTime.CacheReadTokens
	stats.TotalTokens = allTime.totalTokens()
	stats.TotalCost = allTime.TotalCost
	stats.TotalActualCost = allTime.ActualCost
	stats.AverageDurationMs = allTime.averageDurationMs()

	stats.TodayRequests = todayTotals.Requests
	stats.TodayInputTokens = todayTotals.InputTokens
	stats.TodayOutputTokens = todayTotals.OutputTokens
	stats.TodayCacheCreationTokens = todayTotals.CacheCreationTokens
	stats.TodayCacheReadTokens = todayTotals.CacheReadTokens
	stats.TodayTokens = todayTotals.totalTokens()
	stats.TodayCost = todayTotals.TotalCost
	stats.TodayActualCost = todayTotals.ActualCost

	response.Success(c, stats)
}

// DashboardTrend handles getting user usage trend data
// GET /api/v1/usage/dashboard/trend
func (h *UsageHandler) DashboardTrend(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	startTime, endTime := parseUserTimeRange(c)
	granularity := c.DefaultQuery("granularity", "day")

	var apiKeyID int64
	if apiKeyIDStr := c.Query("api_key_id"); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}

		apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.NotFound(c, "API key not found")
			return
		}
		if apiKey.UserID != subject.UserID {
			response.Forbidden(c, "Not authorized to access this API key's usage trend")
			return
		}

		apiKeyID = id
	}

	// Bucket the same display-transformed records the user sees, so trend tokens/cost are
	// display values consistent with the stat cards and records list.
	records, err := h.loadAllDisplayedPublicUsageRecords(c, subject.UserID, apiKeyID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	trend := aggregateDisplayedPublicUsageTrend(records, granularity)

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  startTime.Format("2006-01-02"),
		"end_date":    endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// DashboardModels handles getting user model usage statistics
// GET /api/v1/usage/dashboard/models
func (h *UsageHandler) DashboardModels(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	startTime, endTime := parseUserTimeRange(c)

	// Group the same display-transformed records the user sees, so per-model tokens/cost
	// are display values (never real tokens/prices).
	records, err := h.loadAllDisplayedPublicUsageRecords(c, subject.UserID, 0, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats := aggregateDisplayedModelStats(records)

	response.Success(c, gin.H{
		"models":     stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// BatchAPIKeysUsageRequest represents the request for batch API keys usage
type BatchAPIKeysUsageRequest struct {
	APIKeyIDs []int64 `json:"api_key_ids" binding:"required"`
}

// DashboardAPIKeysUsage handles getting usage stats for user's own API keys
// POST /api/v1/usage/dashboard/api-keys-usage
func (h *UsageHandler) DashboardAPIKeysUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req BatchAPIKeysUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.APIKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	// Limit the number of API key IDs to prevent SQL parameter overflow
	if len(req.APIKeyIDs) > 100 {
		response.BadRequest(c, "Too many API key IDs (maximum 100 allowed)")
		return
	}

	validAPIKeyIDs, err := h.apiKeyService.VerifyOwnership(c.Request.Context(), subject.UserID, req.APIKeyIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if len(validAPIKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	stats, err := h.usageService.GetBatchAPIKeyUsageStats(c.Request.Context(), validAPIKeyIDs, time.Time{}, time.Time{})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"stats": stats})
}

const publicUsageAggregationPageSize = 1000

func (h *UsageHandler) loadAllDisplayedPublicUsageRecords(c *gin.Context, userID, apiKeyID int64, startTime, endTime time.Time) ([]dto.UsageLog, error) {
	displayMap := h.loadDisplayPricingMapForUser(c, userID)
	userDisplayRates := h.loadUserDisplayRates(c, userID)
	filters := usagestats.UsageLogFilters{
		UserID:     userID,
		APIKeyID:   apiKeyID,
		StartTime:  &startTime,
		EndTime:    &endTime,
		ExactTotal: true,
	}
	out := make([]dto.UsageLog, 0)
	for page := 1; ; page++ {
		records, result, err := h.usageService.ListWithFilters(c.Request.Context(), pagination.PaginationParams{
			Page:      page,
			PageSize:  publicUsageAggregationPageSize,
			SortBy:    "created_at",
			SortOrder: pagination.SortOrderAsc,
		}, filters)
		if err != nil {
			return nil, err
		}
		for i := range records {
			out = append(out, *displayUsageRecordForUser(&records[i], displayMap, userDisplayRates))
		}
		if len(records) == 0 || result == nil || page >= result.Pages || int64(len(out)) >= result.Total {
			break
		}
	}
	return out, nil
}

func displayUsageRecordForUser(record *service.UsageLog, displayMap dto.DisplayPricingMap, userDisplayRates map[int64]service.UserGroupRateData) *dto.UsageLog {
	u := dto.UsageLogFromService(record, displayMap)
	if u == nil {
		return nil
	}
	if userDisplayRates != nil && record.GroupID != nil {
		if dr, ok := userDisplayRates[*record.GroupID]; ok && dr.DisplayRateMultiplier != nil {
			dto.ApplyUserDisplayRate(u, *dr.DisplayRateMultiplier)
		}
	}
	return u
}

func aggregateDisplayedPublicUsageStats(records []dto.UsageLog) *service.UsageStats {
	stats := &service.UsageStats{}
	var durationSum float64
	for i := range records {
		record := &records[i]
		stats.TotalRequests++
		stats.TotalInputTokens += int64(record.InputTokens)
		stats.TotalOutputTokens += int64(record.OutputTokens)
		stats.TotalCacheTokens += int64(record.CacheCreationTokens + record.CacheReadTokens)
		stats.TotalCost += record.TotalCost
		stats.TotalActualCost += record.ActualCost
		if record.DurationMs != nil {
			durationSum += float64(*record.DurationMs)
		}
	}
	stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheTokens
	if stats.TotalRequests > 0 {
		stats.AverageDurationMs = durationSum / float64(stats.TotalRequests)
	}
	return stats
}

func aggregateDisplayedPublicUsageTrend(records []dto.UsageLog, granularity string) []usagestats.TrendDataPoint {
	buckets := make(map[string]*usagestats.TrendDataPoint)
	for i := range records {
		record := &records[i]
		label := publicUsageTrendBucketLabel(record.CreatedAt, granularity)
		point := buckets[label]
		if point == nil {
			point = &usagestats.TrendDataPoint{Date: label}
			buckets[label] = point
		}
		point.Requests++
		point.InputTokens += int64(record.InputTokens)
		point.OutputTokens += int64(record.OutputTokens)
		point.CacheCreationTokens += int64(record.CacheCreationTokens)
		point.CacheReadTokens += int64(record.CacheReadTokens)
		point.TotalTokens += int64(record.InputTokens + record.OutputTokens + record.CacheCreationTokens + record.CacheReadTokens)
		point.Cost += record.TotalCost
		point.ActualCost += record.ActualCost
	}

	labels := make([]string, 0, len(buckets))
	for label := range buckets {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	out := make([]usagestats.TrendDataPoint, 0, len(labels))
	for _, label := range labels {
		out = append(out, *buckets[label])
	}
	return out
}

func publicUsageTrendBucketLabel(t time.Time, granularity string) string {
	switch granularity {
	case "hour":
		return t.Format("2006-01-02 15:00")
	case "week":
		year, week := t.ISOWeek()
		return fmt.Sprintf("%04d-%02d", year, week)
	case "month":
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
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

func (h *UsageHandler) loadDisplayPricingMapForUser(c *gin.Context, userID int64) dto.DisplayPricingMap {
	globalMap := h.loadDisplayPricingMap(c)
	if h.userModelPricingService == nil {
		return globalMap
	}
	userOverrides, err := h.userModelPricingService.GetEnabledByUserID(c.Request.Context(), userID)
	if err != nil || len(userOverrides) == 0 {
		return globalMap
	}
	return dto.BuildUserDisplayPricingMap(globalMap, userOverrides)
}

func (h *UsageHandler) loadUserDisplayRates(c *gin.Context, userID int64) map[int64]service.UserGroupRateData {
	if h.apiKeyService == nil {
		return nil
	}
	rates, err := h.apiKeyService.GetUserGroupRatesFull(c.Request.Context(), userID)
	if err != nil {
		return nil
	}
	return rates
}

// displayUsageTotals accumulates user-facing display token/cost totals.
// Cache-creation and cache-read are kept separate so it can also fill the
// dashboard's split cache fields. All values are display values (the same
// transform applied to the per-row usage records); actual_cost is unchanged.
type displayUsageTotals struct {
	Requests            int64
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	TotalCost           float64
	ActualCost          float64
	DurationSum         float64
}

func (t *displayUsageTotals) addDisplayed(u *dto.UsageLog, requests int64, durationSum float64) {
	if u == nil {
		return
	}
	t.Requests += requests
	t.InputTokens += int64(u.InputTokens)
	t.OutputTokens += int64(u.OutputTokens)
	t.CacheCreationTokens += int64(u.CacheCreationTokens)
	t.CacheReadTokens += int64(u.CacheReadTokens)
	t.TotalCost += u.TotalCost
	t.ActualCost += u.ActualCost
	t.DurationSum += durationSum
}

func (t *displayUsageTotals) totalTokens() int64 {
	return t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
}

func (t *displayUsageTotals) averageDurationMs() float64 {
	if t.Requests <= 0 {
		return 0
	}
	return t.DurationSum / float64(t.Requests)
}

// groupToServiceUsageLog builds a synthetic service.UsageLog from a display-aggregate
// group so it can be run through the exact same per-row display transform.
func groupToServiceUsageLog(g *usagestats.DisplayAggregateGroup) *service.UsageLog {
	rec := &service.UsageLog{
		Model:               g.Model,
		GroupID:             g.GroupID,
		InputTokens:         int(g.InputTokens),
		OutputTokens:        int(g.OutputTokens),
		CacheCreationTokens: int(g.CacheCreationTokens),
		CacheReadTokens:     int(g.CacheReadTokens),
		InputCost:           g.InputCost,
		OutputCost:          g.OutputCost,
		CacheCreationCost:   g.CacheCreationCost,
		CacheReadCost:       g.CacheReadCost,
		TotalCost:           g.TotalCost,
		ActualCost:          g.ActualCost,
		RateMultiplier:      g.RateMultiplier,
		LongContextApplied:  g.LongContextApplied,
	}
	if g.LongContextInputMultiplier != nil {
		rec.LongContextInputMultiplier = *g.LongContextInputMultiplier
	}
	if g.LongContextOutputMultiplier != nil {
		rec.LongContextOutputMultiplier = *g.LongContextOutputMultiplier
	}
	return rec
}

// aggregateDisplayedGroups applies the per-row display transform once per aggregate
// group and sums the results into display totals. Used for unbounded ranges where
// loading every row is infeasible (e.g. all-time dashboard totals).
func (h *UsageHandler) aggregateDisplayedGroups(groups []usagestats.DisplayAggregateGroup, displayMap dto.DisplayPricingMap, userDisplayRates map[int64]service.UserGroupRateData) displayUsageTotals {
	var totals displayUsageTotals
	for i := range groups {
		g := &groups[i]
		u := displayUsageRecordForUser(groupToServiceUsageLog(g), displayMap, userDisplayRates)
		totals.addDisplayed(u, g.Requests, float64(g.DurationSum))
	}
	return totals
}

// userDashboardDisplayTotals computes display-value totals for a user over an optional
// time range (nil bounds = all-time) using per-group aggregation.
func (h *UsageHandler) userDashboardDisplayTotals(c *gin.Context, userID int64, displayMap dto.DisplayPricingMap, userDisplayRates map[int64]service.UserGroupRateData, startTime, endTime *time.Time) (displayUsageTotals, error) {
	groups, err := h.usageService.GetUserDisplayAggregateGroups(c.Request.Context(), userID, 0, startTime, endTime)
	if err != nil {
		return displayUsageTotals{}, err
	}
	return h.aggregateDisplayedGroups(groups, displayMap, userDisplayRates), nil
}

// aggregateDisplayedModelStats groups already-display-transformed usage records by model.
func aggregateDisplayedModelStats(records []dto.UsageLog) []usagestats.ModelStat {
	type acc struct {
		requests   int64
		input      int64
		output     int64
		cacheCreat int64
		cacheRead  int64
		cost       float64
		actualCost float64
	}
	byModel := make(map[string]*acc)
	order := make([]string, 0)
	for i := range records {
		r := &records[i]
		a := byModel[r.Model]
		if a == nil {
			a = &acc{}
			byModel[r.Model] = a
			order = append(order, r.Model)
		}
		a.requests++
		a.input += int64(r.InputTokens)
		a.output += int64(r.OutputTokens)
		a.cacheCreat += int64(r.CacheCreationTokens)
		a.cacheRead += int64(r.CacheReadTokens)
		a.cost += r.TotalCost
		a.actualCost += r.ActualCost
	}
	out := make([]usagestats.ModelStat, 0, len(order))
	for _, model := range order {
		a := byModel[model]
		out = append(out, usagestats.ModelStat{
			Model:               model,
			Requests:            a.requests,
			InputTokens:         a.input,
			OutputTokens:        a.output,
			CacheCreationTokens: a.cacheCreat,
			CacheReadTokens:     a.cacheRead,
			TotalTokens:         a.input + a.output + a.cacheCreat + a.cacheRead,
			Cost:                a.cost,
			ActualCost:          a.actualCost,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalTokens > out[j].TotalTokens })
	return out
}
