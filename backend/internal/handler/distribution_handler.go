package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
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

type DistributionGenerateBalanceRedeemCodeRequest struct {
	ValueUSD float64 `json:"value_usd"`
	Note     string  `json:"note"`
}

type DistributionGenerateSubscriptionRedeemCodeRequest struct {
	PlanID int64  `json:"plan_id"`
	Note   string `json:"note"`
}

type DistributionGenerateAPIKeyRequest struct {
	Name          string  `json:"name"`
	QuotaUSD      float64 `json:"quota_usd"`
	GroupID       *int64  `json:"group_id"`
	ExpiresInDays *int    `json:"expires_in_days"`
}

type DistributionRechargeAPIKeyRequest struct {
	QuotaUSD float64 `json:"quota_usd"`
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

func (h *DistributionHandler) ListAssets(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.distributionService.ListAssets(
		c.Request.Context(),
		subject.UserID,
		page,
		pageSize,
		c.Query("asset_type"),
		c.Query("status"),
		c.Query("search"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *DistributionHandler) VoidAsset(c *gin.Context) {
	h.DisableAsset(c)
}

func (h *DistributionHandler) RechargeAPIKeyAsset(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid asset id")
		return
	}
	var req DistributionRechargeAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.RechargeAPIKeyAsset(c.Request.Context(), subject.UserID, assetID, subject.UserID, service.DistributionRechargeAPIKeyInput{
		QuotaUSD: req.QuotaUSD,
	}, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) DisableAsset(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid asset id")
		return
	}
	out, err := h.distributionService.DisableAsset(c.Request.Context(), subject.UserID, assetID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) EnableAsset(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid asset id")
		return
	}
	out, err := h.distributionService.EnableAsset(c.Request.Context(), subject.UserID, assetID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) RefundAPIKeyAsset(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid asset id")
		return
	}
	out, err := h.distributionService.RefundAPIKeyAsset(c.Request.Context(), subject.UserID, assetID, subject.UserID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) GenerateBalanceRedeemCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req DistributionGenerateBalanceRedeemCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.GenerateBalanceRedeemCode(c.Request.Context(), subject.UserID, service.DistributionGenerateBalanceRedeemCodeInput{
		ValueUSD: req.ValueUSD,
		Note:     req.Note,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) GenerateSubscriptionRedeemCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req DistributionGenerateSubscriptionRedeemCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.GenerateSubscriptionRedeemCode(c.Request.Context(), subject.UserID, service.DistributionGenerateSubscriptionRedeemCodeInput{
		PlanID: req.PlanID,
		Note:   req.Note,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) ListAPIKeyGroups(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	groups, err := h.distributionService.ListAPIKeyGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.Group, 0, len(groups))
	for i := range groups {
		out = append(out, *dto.GroupFromService(&groups[i]))
	}
	response.Success(c, out)
}

func (h *DistributionHandler) GenerateAPIKey(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req DistributionGenerateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.GenerateAPIKey(c.Request.Context(), subject.UserID, service.DistributionGenerateAPIKeyInput{
		Name:          req.Name,
		QuotaUSD:      req.QuotaUSD,
		GroupID:       req.GroupID,
		ExpiresInDays: req.ExpiresInDays,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

type AdminDistributionReviewRequest struct {
	Approved *bool  `json:"approved" binding:"required"`
	Note     string `json:"note"`
}

type AdminDistributionSettingsRequest struct {
	RMBPerUSD            float64 `json:"rmb_per_usd"`
	SubscriptionDiscount float64 `json:"subscription_discount"`
	APIKeyGroupIDs       []int64 `json:"api_key_group_ids"`
}

type AdminDistributionAdjustWalletRequest struct {
	Amount float64 `json:"amount"`
	Note   string  `json:"note"`
}

type AdminDistributionWalletStatusRequest struct {
	Frozen bool `json:"frozen"`
}

type AdminDistributionAgentRatesRequest struct {
	RMBPerUSDOverride            *float64 `json:"rmb_per_usd_override"`
	SubscriptionDiscountOverride *float64 `json:"subscription_discount_override"`
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

func (h *DistributionHandler) AdminGetSettings(c *gin.Context) {
	out, err := h.distributionService.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) AdminUpdateSettings(c *gin.Context) {
	var req AdminDistributionSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.UpdateSettings(c.Request.Context(), service.DistributionSettings{
		RMBPerUSD:            req.RMBPerUSD,
		SubscriptionDiscount: req.SubscriptionDiscount,
		APIKeyGroupIDs:       req.APIKeyGroupIDs,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) AdminUpdateAgentRates(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var req AdminDistributionAgentRatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.UpdateAgentRates(c.Request.Context(), userID, service.DistributionAgentRateSettings{
		RMBPerUSDOverride:            req.RMBPerUSDOverride,
		SubscriptionDiscountOverride: req.SubscriptionDiscountOverride,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) AdminListWallets(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := c.Query("search")
	items, total, err := h.distributionService.ListWallets(c.Request.Context(), page, pageSize, search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *DistributionHandler) AdminListLedger(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var userID int64
	if raw := c.Query("user_id"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = v
	}
	items, total, err := h.distributionService.ListAllWalletLedger(c.Request.Context(), page, pageSize, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *DistributionHandler) AdminListAssets(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var userID int64
	if raw := c.Query("user_id"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 0 {
			response.BadRequest(c, "Invalid user_id")
			return
		}
		userID = v
	}
	items, total, err := h.distributionService.ListAllAssets(
		c.Request.Context(),
		page,
		pageSize,
		userID,
		c.Query("asset_type"),
		c.Query("status"),
		c.Query("search"),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *DistributionHandler) AdminVoidAsset(c *gin.Context) {
	h.AdminDisableAsset(c)
}

func (h *DistributionHandler) AdminRechargeAPIKeyAsset(c *gin.Context) {
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid asset id")
		return
	}
	var req DistributionRechargeAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	operatorID := currentUserIDFromContext(c)
	out, err := h.distributionService.RechargeAPIKeyAsset(c.Request.Context(), 0, assetID, operatorID, service.DistributionRechargeAPIKeyInput{
		QuotaUSD: req.QuotaUSD,
	}, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) AdminDisableAsset(c *gin.Context) {
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid asset id")
		return
	}
	out, err := h.distributionService.DisableAsset(c.Request.Context(), 0, assetID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) AdminEnableAsset(c *gin.Context) {
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid asset id")
		return
	}
	out, err := h.distributionService.EnableAsset(c.Request.Context(), 0, assetID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) AdminRefundAPIKeyAsset(c *gin.Context) {
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid asset id")
		return
	}
	operatorID := currentUserIDFromContext(c)
	out, err := h.distributionService.RefundAPIKeyAsset(c.Request.Context(), 0, assetID, operatorID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) AdminAdjustWallet(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var req AdminDistributionAdjustWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.AdminAdjustWallet(c.Request.Context(), service.DistributionAdminAdjustWalletInput{
		UserID:  userID,
		Amount:  req.Amount,
		Note:    req.Note,
		AdminID: currentUserIDFromContext(c),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *DistributionHandler) AdminUpdateWalletStatus(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var req AdminDistributionWalletStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	out, err := h.distributionService.UpdateWalletStatus(c.Request.Context(), userID, req.Frozen)
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
