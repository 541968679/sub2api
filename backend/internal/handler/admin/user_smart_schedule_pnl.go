package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type batchSmartSchedulePnlSummariesRequest struct {
	UserIDs []int64 `json:"user_ids" binding:"required"`
}

type smartSchedulePnlPairsRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required"`
}

func (h *UserHandler) schedulePnlService() *service.SchedulePnlService {
	if h == nil {
		return nil
	}
	return h.schedulePnl
}

func adminTimezone(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return c.Query("timezone")
}

// GetBatchSmartSchedulePnlSummaries POST /admin/users/smart-schedule/pnl/summaries
func (h *UserHandler) GetBatchSmartSchedulePnlSummaries(c *gin.Context) {
	var req batchSmartSchedulePnlSummariesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	userIDs := normalizeInt64IDList(req.UserIDs)
	svc := h.schedulePnlService()
	if svc == nil || len(userIDs) == 0 {
		response.Success(c, gin.H{"summaries": map[string]service.SchedulePnlSummary{}})
		return
	}
	summaries, err := svc.UserSummaries(c.Request.Context(), userIDs, adminTimezone(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"summaries": summaries})
}

// GetSmartSchedulePnlPairs POST /admin/users/:id/smart-schedule/pnl/pairs
func (h *UserHandler) GetSmartSchedulePnlPairs(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req smartSchedulePnlPairsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	accountIDs := normalizeInt64IDList(req.AccountIDs)
	svc := h.schedulePnlService()
	if svc == nil {
		pairs := make(map[string]service.SchedulePnlSummary, len(accountIDs))
		for _, accountID := range accountIDs {
			pairs[strconv.FormatInt(accountID, 10)] = service.SchedulePnlSummary{}
		}
		response.Success(c, gin.H{"pairs": pairs})
		return
	}
	pairs, err := svc.PairSummaries(c.Request.Context(), userID, accountIDs, adminTimezone(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"pairs": pairs})
}

// GetSmartSchedulePnlTrend GET /admin/users/:id/smart-schedule/pnl/trend
func (h *UserHandler) GetSmartSchedulePnlTrend(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	if _, err := h.adminService.GetUser(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var accountID *int64
	if raw := c.Query("account_id"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		accountID = &parsed
	}
	svc := h.schedulePnlService()
	if svc == nil {
		response.Success(c, service.SchedulePnlTrend{Range: service.SchedulePnlRange24h, Granularity: "hour", Points: []service.SchedulePnlTrendPoint{}})
		return
	}
	trend, err := svc.Trend(c.Request.Context(), userID, accountID, c.Query("range"), adminTimezone(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, trend)
}
