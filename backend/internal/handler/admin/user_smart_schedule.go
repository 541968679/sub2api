package admin

import (
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type putSmartScheduleRequest struct {
	Enabled                  bool                                `json:"enabled"`
	QualityMaxP50TTFTMs      *int                                `json:"quality_max_p50_ttft_ms"`
	QualityMinSuccessRate    *float64                            `json:"quality_min_success_rate"`
	QualityWindowSamples     *int                                `json:"quality_window_samples"`
	QualityWindowN           *int                                `json:"quality_window_n"`
	QualityMinSuccessSamples *int                                `json:"quality_min_success_samples"`
	QualityMinTTFTSamples    *int                                `json:"quality_min_ttft_samples"`
	QualityCondition         *string                             `json:"quality_condition"`
	CooldownMinutes          int                                 `json:"cooldown_minutes"`
	Accounts                 []service.SmartScheduleAccountMember `json:"accounts"`
}

type patchSmartScheduleSortRequest struct {
	Accounts []service.SmartScheduleSortAssignment `json:"accounts"`
}

type copySmartScheduleRequest struct {
	FromPlatform string `json:"from_platform"`
}

type resumeSmartScheduleRequest struct {
	UserID int64  `json:"user_id"`
	State  string `json:"state"`
}

func (h *UserHandler) smartScheduleService() *service.UserSmartScheduleService {
	if h == nil {
		return nil
	}
	return h.smartSchedule
}

type batchSmartScheduleSummariesRequest struct {
	UserIDs []int64 `json:"user_ids" binding:"required"`
}

// GetBatchSmartScheduleSummaries POST /admin/users/smart-schedule/summaries
func (h *UserHandler) GetBatchSmartScheduleSummaries(c *gin.Context) {
	var req batchSmartScheduleSummariesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	userIDs := normalizeInt64IDList(req.UserIDs)
	if len(userIDs) == 0 {
		response.Success(c, gin.H{"summaries": map[string]service.UserSmartScheduleSummary{}})
		return
	}
	svc := h.smartScheduleService()
	if svc == nil {
		empty := make(map[string]service.UserSmartScheduleSummary, len(userIDs))
		for _, userID := range userIDs {
			empty[strconv.FormatInt(userID, 10)] = service.UserSmartScheduleSummary{
				EnabledPlatforms: []string{},
				PoolCounts:       map[string]int{},
			}
		}
		response.Success(c, gin.H{"summaries": empty})
		return
	}
	summaries, err := svc.ListSummaries(c.Request.Context(), userIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"summaries": summaries})
}

// GetUserSmartSchedule GET /admin/users/:id/smart-schedule
func (h *UserHandler) GetUserSmartSchedule(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	svc := h.smartScheduleService()
	if svc == nil {
		response.Success(c, service.UserSmartScheduleView{UserID: userID, Platforms: map[string]service.SmartSchedulePlatformView{}})
		return
	}
	view, err := svc.Get(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

// UpdateUserSmartSchedule PUT /admin/users/:id/smart-schedule/:platform
func (h *UserHandler) UpdateUserSmartSchedule(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	platform := c.Param("platform")
	svc := h.smartScheduleService()
	if svc == nil {
		response.ErrorFrom(c, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable"))
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req putSmartScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	view, err := svc.PutPlatform(c.Request.Context(), userID, platform, service.SmartSchedulePlatformWrite{
		Enabled:                  req.Enabled,
		QualityMaxP50TTFTMs:      req.QualityMaxP50TTFTMs,
		QualityMinSuccessRate:    req.QualityMinSuccessRate,
		QualityWindowSamples:     req.QualityWindowSamples,
		QualityWindowN:           req.QualityWindowN,
		QualityMinSuccessSamples: req.QualityMinSuccessSamples,
		QualityMinTTFTSamples:    req.QualityMinTTFTSamples,
		QualityCondition:         req.QualityCondition,
		CooldownMinutes:          req.CooldownMinutes,
		Accounts:                 req.Accounts,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

// PatchUserSmartScheduleSortOrder PATCH /admin/users/:id/smart-schedule/:platform/sort-order
func (h *UserHandler) PatchUserSmartScheduleSortOrder(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	platform := c.Param("platform")
	svc := h.smartScheduleService()
	if svc == nil {
		response.ErrorFrom(c, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable"))
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req patchSmartScheduleSortRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	view, err := svc.PatchSortOrders(c.Request.Context(), userID, platform, req.Accounts)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

// CopyUserSmartSchedule POST /admin/users/:id/smart-schedule/:platform/copy
func (h *UserHandler) CopyUserSmartSchedule(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	platform := c.Param("platform")
	svc := h.smartScheduleService()
	if svc == nil {
		response.ErrorFrom(c, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable"))
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req copySmartScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.FromPlatform == "" {
		response.BadRequest(c, "from_platform is required")
		return
	}
	view, err := svc.CopyPlatform(c.Request.Context(), userID, platform, req.FromPlatform)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

type batchSmartSchedulePairQualityRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

// GetUserSmartSchedulePairQualityBatch POST /admin/users/:id/smart-schedule/pair-quality
func (h *UserHandler) GetUserSmartSchedulePairQualityBatch(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req batchSmartSchedulePairQualityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	svc := h.smartScheduleService()
	if svc == nil {
		response.ErrorFrom(c, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable"))
		return
	}
	batch, err := svc.GetPairQualityBatch(c.Request.Context(), userID, req.AccountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, batch)
}

// GetUserSmartSchedulePairQualityByAccount GET /admin/users/:id/smart-schedule/pair-quality/:accountId
func (h *UserHandler) GetUserSmartSchedulePairQualityByAccount(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("accountId"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	svc := h.smartScheduleService()
	if svc == nil {
		response.ErrorFrom(c, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable"))
		return
	}
	detail, err := svc.GetPairQualityDetailForAccount(c.Request.Context(), userID, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

// GetUserSmartSchedulePairQuality GET /admin/users/:id/smart-schedule/:platform/accounts/:account_id/pair-quality
func (h *UserHandler) GetUserSmartSchedulePairQuality(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	svc := h.smartScheduleService()
	if svc == nil {
		response.ErrorFrom(c, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable"))
		return
	}
	detail, err := svc.GetPairQualityDetail(c.Request.Context(), userID, c.Param("platform"), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

// ResumeSmartSchedule POST /admin/accounts/:id/smart-schedule-resume
func (h *AccountHandler) ResumeSmartSchedule(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req resumeSmartScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
		response.BadRequest(c, "user_id is required")
		return
	}
	if h.smartSchedule == nil {
		response.ErrorFrom(c, infraerrors.New(503, "SMART_SCHEDULE_UNAVAILABLE", "smart schedule service unavailable"))
		return
	}
	if _, err := h.adminService.GetAccount(c.Request.Context(), accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.smartSchedule.SetPairAdmission(c.Request.Context(), accountID, req.UserID, req.State)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
