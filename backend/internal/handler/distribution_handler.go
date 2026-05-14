package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DistributionHandler struct {
	distributionService service.DistributionServicePort
}

func NewDistributionHandler(distributionService service.DistributionServicePort) *DistributionHandler {
	return &DistributionHandler{distributionService: distributionService}
}

type DistributionApplyRequest struct {
	Contact string `json:"contact"`
	Reason  string `json:"reason"`
}

func (h *DistributionHandler) GetMine(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	out, err := h.distributionService.GetCurrentUserSummary(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) Apply(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req DistributionApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.ApplyForAgent(c.Request.Context(), subject.UserID, req.Contact, req.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) GetLedger(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.distributionService.ListWalletLedger(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

type AdminDistributionReviewRequest struct {
	Approved *bool  `json:"approved" binding:"required"`
	Note     string `json:"note"`
}

func (h *DistributionHandler) AdminListApplications(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := c.Query("search")
	items, total, err := h.distributionService.ListAgentApplications(c.Request.Context(), page, pageSize, search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *DistributionHandler) AdminReviewApplication(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var req AdminDistributionReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	reviewerID := currentUserIDFromContext(c)
	out, err := h.distributionService.ReviewAgentApplication(c.Request.Context(), userID, *req.Approved, req.Note, reviewerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func currentUserIDFromContext(c *gin.Context) int64 {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		return 0
	}
	return subject.UserID
}
