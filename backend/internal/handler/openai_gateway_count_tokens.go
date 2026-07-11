package handler

// Anthropic 兼容 POST /v1/messages/count_tokens 的 OpenAI 侧入口。
// CountTokens 面向 OpenAI 平台分组，按官方 upstream/main 语义移植：转换为
// /v1/responses/input_tokens 上游计数，OAuth 缺 scope / 端点不支持时本地
// tiktoken 估算。CountTokensClaudeGPTBridge 面向绑定了 Claude-GPT bridge 的
// Antigravity 分组：存在显式 mapping 时用 bridge 账号计数，账号全部临时不可
// 用时直接本地估算，绝不依赖可能为空的 native Antigravity 池。
// 两条路径都不占用户/账号并发槽，不写 usage、不扣费。

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// CountTokens handles Anthropic-compatible POST /v1/messages/count_tokens for
// OpenAI-platform groups. It validates billing eligibility and forwards to the
// OpenAI token-count bridge without taking concurrency slots or recording
// usage.
func (h *OpenAIGatewayHandler) CountTokens(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.openai_gateway.count_tokens",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	if apiKey.Group != nil && !apiKey.Group.AllowMessagesDispatch {
		h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error",
			"This group does not allow /v1/messages dispatch")
		return
	}
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, reqModel, ok := h.readCountTokensRequest(c)
	if !ok {
		return
	}
	if !isGroupModelAllowed(apiKey.Group, reqModel) {
		h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error", groupModelAccessDeniedMessage)
		return
	}

	reqLog = reqLog.With(zap.String("model", reqModel))
	routingModel := service.NormalizeOpenAICompatRequestedModel(reqModel)
	preferredMappedModel := resolveOpenAIMessagesDispatchMappedModel(apiKey, reqModel)

	setOpsRequestContext(c, reqModel, false, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(false, false)))

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)

	if !h.checkCountTokensBillingEligibility(c, apiKey, reqLog) {
		return
	}

	requestStart := time.Now()
	sessionHash := h.gatewayService.GenerateSessionHash(c, body)
	currentRoutingModel := routingModel
	if preferredMappedModel != "" {
		currentRoutingModel = preferredMappedModel
	}
	selection, _, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
		c.Request.Context(),
		apiKey.GroupID,
		"",
		sessionHash,
		currentRoutingModel,
		nil,
		service.OpenAIUpstreamTransportAny,
		service.OpenAIEndpointCapabilityChatCompletions,
		false,
		false,
	)
	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	if err != nil || selection == nil || selection.Account == nil {
		if err != nil {
			reqLog.Warn("openai_count_tokens.account_select_failed", zap.Error(err))
		}
		cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, currentRoutingModel, reqModel, service.PlatformOpenAI)
		if !cls.ModelNotFound {
			markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
		}
		h.anthropicErrorResponse(c, cls.Status, cls.ErrType, cls.Message)
		return
	}

	account := selection.Account
	setOpsSelectedAccount(c, account.ID, account.Platform)
	if selection.Acquired && selection.ReleaseFunc != nil {
		defer selection.ReleaseFunc()
	}
	forwardBody := body
	if channelMapping.Mapped {
		forwardBody = h.gatewayService.ReplaceModelInBody(body, channelMapping.MappedModel)
	}

	if err := h.gatewayService.ForwardCountTokensAsAnthropic(c.Request.Context(), c, account, forwardBody, preferredMappedModel); err != nil {
		reqLog.Error("openai_count_tokens.forward_failed", zap.Int64("account_id", account.ID), zap.Error(err))
	}
}

// CountTokensClaudeGPTBridge handles count_tokens for Antigravity groups with
// an explicit Claude-GPT bridge mapping. It returns true when the request was
// handled on the bridge side; false means the group has no bridge intent and
// the caller should use the native Antigravity handler. Once bridge intent is
// established the request never falls through to native: temporarily blocked
// bridge accounts produce a local tiktoken estimate instead of a native 503.
func (h *OpenAIGatewayHandler) CountTokensClaudeGPTBridge(c *gin.Context) bool {
	if h == nil || h.gatewayService == nil || c == nil {
		return false
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.GroupID == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformAntigravity {
		return false
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	resetRequestBody(c.Request, body)
	if err != nil || len(body) == 0 || !gjson.ValidBytes(body) {
		// 协议级问题交给 native handler 输出规范错误；此时尚未确立 bridge 意图。
		return false
	}
	modelResult := gjson.GetBytes(body, "model")
	reqModel := strings.TrimSpace(modelResult.String())
	if !modelResult.Exists() || modelResult.Type != gjson.String || reqModel == "" {
		return false
	}

	routeStart := time.Now()
	decision := h.gatewayService.ResolveClaudeGPTBridgeRoute(c.Request.Context(), apiKey.GroupID, reqModel)
	reqLog := requestLogger(c, "handler.openai_gateway.claude_gpt_bridge_count_tokens",
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	logClaudeGPTBridgeRouteDecision(reqLog, decision, reqModel, "count_tokens_preflight", routeStart)
	if decision.State == service.ClaudeGPTBridgeRouteNotConfigured {
		return false
	}

	if !isGroupModelAllowed(apiKey.Group, reqModel) {
		h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error", groupModelAccessDeniedMessage)
		return true
	}
	if !h.checkCountTokensBillingEligibility(c, apiKey, reqLog) {
		return true
	}

	if decision.State == service.ClaudeGPTBridgeRouteReady {
		sessionHash := h.gatewayService.GenerateSessionHash(c, body)
		selection, _, err := h.gatewayService.SelectAccountWithSchedulerForClaudeGPTBridge(
			c.Request.Context(),
			apiKey.GroupID,
			sessionHash,
			reqModel,
			nil,
			service.OpenAIUpstreamTransportAny,
		)
		if err == nil && selection != nil && selection.Account != nil {
			account := selection.Account
			// count_tokens 不占账号并发槽：拿到账号后立即释放。
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			if mapped, ok := account.ResolveClaudeGPTBridgeModel(reqModel); ok {
				if err := h.gatewayService.ForwardCountTokensAsAnthropicClaudeGPTBridge(c.Request.Context(), c, account, body, mapped); err != nil {
					reqLog.Warn("openai_count_tokens.bridge_forward_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				}
				return true
			}
		} else if err != nil {
			reqLog.Warn("openai_count_tokens.bridge_account_select_failed", zap.Error(err))
		}
	}

	// rate_limited / unavailable / probe_error / 选号竞态：本地估算，
	// 不调用上游，也不进入 native 池。
	upstreamModel, _ := h.gatewayService.ResolveClaudeGPTBridgeCountUpstreamModel(c.Request.Context(), apiKey.GroupID, reqModel)
	if err := h.gatewayService.EstimateCountTokensClaudeGPTBridge(c, body, upstreamModel); err != nil {
		reqLog.Warn("openai_count_tokens.bridge_estimate_failed", zap.Error(err))
	}
	return true
}

// readCountTokensRequest reads and validates the count_tokens request body,
// writing the canonical Anthropic error on failure.
func (h *OpenAIGatewayHandler) readCountTokensRequest(c *gin.Context) (body []byte, reqModel string, ok bool) {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.anthropicErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return nil, "", false
		}
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return nil, "", false
	}
	if len(body) == 0 {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return nil, "", false
	}
	if !gjson.ValidBytes(body) {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return nil, "", false
	}
	modelResult := gjson.GetBytes(body, "model")
	reqModel = strings.TrimSpace(modelResult.String())
	if !modelResult.Exists() || modelResult.Type != gjson.String || reqModel == "" {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, "", false
	}
	return body, reqModel, true
}

// checkCountTokensBillingEligibility mirrors the Messages billing gate without
// charging anything: count_tokens never writes usage or cost.
func (h *OpenAIGatewayHandler) checkCountTokensBillingEligibility(c *gin.Context, apiKey *service.APIKey, reqLog *zap.Logger) bool {
	if h.billingCacheService == nil {
		return true
	}
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai_count_tokens.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.anthropicErrorResponse(c, status, code, message)
		return false
	}
	return true
}
