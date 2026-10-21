package handler

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SeedanceTasks exposes Ark's native asynchronous video task protocol.
func (h *OpenAIGatewayHandler) SeedanceTasks(c *gin.Context) {
	endpoint := service.SeedanceEndpointCreate
	taskID := ""
	switch c.Request.Method {
	case http.MethodGet:
		endpoint = service.SeedanceEndpointStatus
		taskID = c.Param("task_id")
	case http.MethodDelete:
		endpoint = service.SeedanceEndpointDelete
		taskID = c.Param("task_id")
	}
	h.handleSeedance(c, endpoint, taskID)
}

func (h *OpenAIGatewayHandler) handleSeedance(c *gin.Context, endpoint service.GrokMediaEndpoint, taskID string) {
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)

	if c.Request.Method == http.MethodPost && c.GetHeader("Content-Type") != "" {
		mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || mediaType != "application/json" {
			h.errorResponse(c, http.StatusUnsupportedMediaType, "invalid_request_error", "Seedance requires application/json")
			return
		}
	}

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformOpenAI {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "Seedance requires an OpenAI group")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	if h.gatewayService == nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Gateway is not configured")
		return
	}

	reqLog := requestLogger(c, "handler.openai_gateway.seedance",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.String("endpoint", string(endpoint)),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	var body []byte
	requestedModel := ""
	if endpoint == service.SeedanceEndpointCreate {
		raw, err := readSeedanceBody(c)
		if err != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		body = raw
		info, err := service.ParseSeedanceRequest(body)
		if err != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		requestedModel = info.Model
		if !service.GroupAllowsImageGeneration(apiKey.Group) {
			h.errorResponse(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
			return
		}
		if err := h.gatewayService.RejectUnpricedModel(c.Request.Context(), c, requestedModel); err != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Model price is not configured")
			return
		}
	} else if strings.TrimSpace(taskID) == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	setOpsRequestContext(c, requestedModel, false, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))

	requestCtx := c.Request.Context()
	var bound service.StickySessionBinding
	if endpoint != service.SeedanceEndpointCreate {
		found, err := h.gatewayService.LookupSeedanceTask(requestCtx, apiKey.GroupID, subject.UserID, apiKey.ID, taskID)
		if err != nil {
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Seedance task not found")
			return
		}
		bound = found
		requestedModel = bound.Model
	} else {
		subscription, _ := middleware2.GetSubscriptionFromContext(c)
		if err := h.billingCacheService.CheckBillingEligibility(requestCtx, apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(requestCtx, apiKey)); err != nil {
			status, code, message, retryAfter := billingErrorDetails(err)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.errorResponse(c, status, code, message)
			return
		}
		inflightDone, inflightErr := reserveInflightBalance(c, h.billingCacheService, h.gatewayService, apiKey, subscription, service.InflightEstimateRequest{
			Model:     requestedModel,
			BodyBytes: len(body),
			Kind:      service.InflightEstimateToken,
		})
		if inflightErr != nil {
			status, code, message, retryAfter := billingErrorDetails(inflightErr)
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(retryAfter))
			}
			h.errorResponse(c, status, code, message)
			return
		}
		defer inflightDone()
	}

	userRelease, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userRelease != nil {
		defer userRelease()
	}

	var account *service.Account
	var accountRelease func()
	if endpoint == service.SeedanceEndpointCreate {
		selection, _, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
			requestCtx,
			apiKey.GroupID,
			"",
			"",
			requestedModel,
			nil,
			service.OpenAIUpstreamTransportHTTPSSE,
			service.OpenAIEndpointCapabilitySeedance,
			false,
			false,
			service.PlatformOpenAI,
		)
		if err != nil || selection == nil || selection.Account == nil {
			h.errorResponse(c, http.StatusServiceUnavailable, "seedance_no_eligible_account", "No eligible Seedance accounts")
			return
		}
		full, loadErr := h.gatewayService.LoadSeedanceAccount(requestCtx, selection.Account.ID)
		if loadErr != nil || !full.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilitySeedance) {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			h.errorResponse(c, http.StatusServiceUnavailable, "seedance_no_eligible_account", "No eligible Seedance accounts")
			return
		}
		selection.Account = full
		release, ok, _ := h.acquireResponsesAccountSlot(c, apiKey.GroupID, "", selection, false, &streamStarted, reqLog)
		if !ok {
			return
		}
		account = full
		accountRelease = release
	} else {
		full, loadErr := h.gatewayService.LoadSeedanceAccount(requestCtx, bound.AccountID)
		if loadErr != nil {
			h.errorResponse(c, http.StatusNotFound, "not_found_error", "Seedance task not found")
			return
		}
		selection := &service.AccountSelectionResult{Account: full, Acquired: true}
		release, ok, _ := h.acquireResponsesAccountSlot(c, apiKey.GroupID, "", selection, false, &streamStarted, reqLog)
		if !ok {
			return
		}
		account = full
		accountRelease = release
	}
	setOpsSelectedAccount(c, account.ID, account.Platform)

	result, err := func() (*service.OpenAIForwardResult, error) {
		defer func() {
			if accountRelease != nil {
				accountRelease()
			}
		}()
		return h.gatewayService.ForwardSeedance(requestCtx, c, account, endpoint, taskID, body)
	}()
	if err != nil {
		if c.Writer.Size() == 0 {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		}
		reqLog.Warn("seedance.forward_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		return
	}
	if endpoint == service.SeedanceEndpointCreate && result != nil && result.ResponseID != "" {
		if bindErr := h.gatewayService.BindSeedanceTask(requestCtx, apiKey.GroupID, subject.UserID, apiKey.ID, result.ResponseID, requestedModel, account.ID); bindErr != nil {
			reqLog.Warn("seedance.bind_failed", zap.String("task_id", result.ResponseID), zap.Error(bindErr))
		}
	}
	if endpoint == service.SeedanceEndpointStatus && result != nil && result.Usage.OutputTokens > 0 && result.VideoCount == 0 {
		if requestedModel != "" {
			result.Model = requestedModel
			result.BillingModel = requestedModel
		}
		result.RequestID = service.SeedanceTaskKey(result.ResponseID)
		recordSeedanceUsage(c, h, apiKey, subject, account, result, requestedModel)
	}
}

func readSeedanceBody(c *gin.Context) ([]byte, error) {
	body, err := c.GetRawData()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errEmptySeedanceBody
	}
	return body, nil
}

var errEmptySeedanceBody = errors.New("Request body is empty")

func recordSeedanceUsage(
	c *gin.Context,
	h *OpenAIGatewayHandler,
	apiKey *service.APIKey,
	subject middleware2.AuthSubject,
	account *service.Account,
	result *service.OpenAIForwardResult,
	requestModel string,
) {
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	userAgent := c.GetHeader("User-Agent")
	clientIP := ip.GetClientIP(c)
	inboundEndpoint := GetInboundEndpoint(c)
	upstreamEndpoint := GetUpstreamEndpoint(c, account.Platform)
	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
			Result:           result,
			APIKey:           apiKey,
			User:             apiKey.User,
			Account:          account,
			Subscription:     subscription,
			InboundEndpoint:  inboundEndpoint,
			UpstreamEndpoint: upstreamEndpoint,
			UserAgent:        userAgent,
			IPAddress:        clientIP,
			APIKeyService:    h.apiKeyService,
			ChannelUsageFields: service.ChannelUsageFields{
				OriginalModel:      requestModel,
				ChannelMappedModel: requestModel,
			},
		}); err != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.seedance"),
				zap.Int64("user_id", subject.UserID),
				zap.Int64("api_key_id", apiKey.ID),
				zap.String("model", requestModel),
			).Error("seedance.record_usage_failed", zap.Error(err))
		}
	})
}
