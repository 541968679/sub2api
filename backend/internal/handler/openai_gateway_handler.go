package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const openAIClaudeGPTBridgeContextKey = "openai_claude_gpt_bridge"

// OpenAIGatewayHandler handles OpenAI API gateway requests
type OpenAIGatewayHandler struct {
	gatewayService           *service.OpenAIGatewayService
	billingCacheService      *service.BillingCacheService
	apiKeyService            *service.APIKeyService
	usageRecordWorkerPool    *service.UsageRecordWorkerPool
	errorPassthroughService  *service.ErrorPassthroughService
	contentModerationService *service.ContentModerationService
	opsService               *service.OpsService
	concurrencyHelper        *ConcurrencyHelper
	maxAccountSwitches       int
	cfg                      *config.Config
}

func openAICompatibleRequestPlatform(apiKey *service.APIKey) string {
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.Platform == service.PlatformGrok {
		return service.PlatformGrok
	}
	return service.PlatformOpenAI
}

func resolveOpenAIForwardDefaultMappedModel(apiKey *service.APIKey, fallbackModel string) string {
	if fallbackModel = strings.TrimSpace(fallbackModel); fallbackModel != "" {
		return fallbackModel
	}
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return strings.TrimSpace(apiKey.Group.DefaultMappedModel)
}

func resolveOpenAIMessagesDispatchMappedModel(apiKey *service.APIKey, requestedModel string) string {
	if apiKey == nil || apiKey.Group == nil {
		return ""
	}
	return strings.TrimSpace(apiKey.Group.ResolveMessagesDispatchModel(requestedModel))
}

func usageRecordContext(parent context.Context, base context.Context) context.Context {
	if base == nil {
		base = context.Background()
	}
	if parent == nil {
		return base
	}
	if clientRequestID, _ := parent.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
		base = context.WithValue(base, ctxkey.ClientRequestID, strings.TrimSpace(clientRequestID))
	}
	if requestID, _ := parent.Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
		base = context.WithValue(base, ctxkey.RequestID, strings.TrimSpace(requestID))
	}
	return base
}

func wrapUsageRecordTaskContext(parent context.Context, task service.UsageRecordTask) service.UsageRecordTask {
	if task == nil {
		return nil
	}
	return func(ctx context.Context) {
		task(usageRecordContext(parent, ctx))
	}
}

// turnUsageRecordContext derives a per-turn billing context for multi-turn
// WebSocket connections. The connection carries a single client request id
// across all turns, so without a per-turn suffix every turn after the first
// collides on the usage_billing_dedup / usage_logs (request_id, api_key_id)
// keys and is silently dropped from billing and usage history.
func turnUsageRecordContext(parent context.Context, turn int, turnRequestID string) context.Context {
	if parent == nil {
		return nil
	}
	suffix := strings.TrimSpace(turnRequestID)
	if suffix == "" {
		suffix = strconv.Itoa(turn)
	}
	derived := parent
	if clientRequestID, _ := parent.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
		derived = context.WithValue(derived, ctxkey.ClientRequestID, strings.TrimSpace(clientRequestID)+":turn:"+suffix)
	}
	if requestID, _ := parent.Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
		derived = context.WithValue(derived, ctxkey.RequestID, strings.TrimSpace(requestID)+":turn:"+suffix)
	}
	return derived
}

func isOpenAIClaudeGPTBridgeRequest(c *gin.Context) bool {
	if c == nil {
		return false
	}
	enabled, _ := c.Get(openAIClaudeGPTBridgeContextKey)
	asBool, _ := enabled.(bool)
	return asBool
}

func resetRequestBody(req *http.Request, body []byte) {
	if req == nil {
		return
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
}

func bridgeUsageFields(reqModel, upstreamModel string) service.ChannelUsageFields {
	fields := service.ChannelUsageFields{
		OriginalModel:         reqModel,
		ChannelMappedModel:    reqModel,
		BillingModelSource:    service.BillingModelSourceRequested,
		OpenAIClaudeGPTBridge: true,
	}
	if upstreamModel != "" && upstreamModel != reqModel {
		fields.ModelMappingChain = reqModel + "→" + upstreamModel
	}
	return fields
}

// MessagesClaudeGPTBridge serves an Antigravity /v1/messages request through
// the OpenAI Claude-GPT bridge. Dispatch is decided beforehand by
// ClaudeGPTBridgeRoute; once bridge intent is established the request never
// falls back to the native Antigravity handler.
func (h *OpenAIGatewayHandler) MessagesClaudeGPTBridge(c *gin.Context) {
	if c != nil {
		c.Set(openAIClaudeGPTBridgeContextKey, true)
	}
	h.Messages(c)
}

// NewOpenAIGatewayHandler creates a new OpenAIGatewayHandler
func NewOpenAIGatewayHandler(
	gatewayService *service.OpenAIGatewayService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	contentModerationService *service.ContentModerationService,
	opsService *service.OpsService,
	cfg *config.Config,
) *OpenAIGatewayHandler {
	pingInterval := time.Duration(0)
	maxAccountSwitches := 3
	if cfg != nil {
		pingInterval = time.Duration(cfg.Concurrency.PingInterval) * time.Second
		if cfg.Gateway.MaxAccountSwitches > 0 {
			maxAccountSwitches = cfg.Gateway.MaxAccountSwitches
		}
	}
	return &OpenAIGatewayHandler{
		gatewayService:           gatewayService,
		billingCacheService:      billingCacheService,
		apiKeyService:            apiKeyService,
		usageRecordWorkerPool:    usageRecordWorkerPool,
		errorPassthroughService:  errorPassthroughService,
		contentModerationService: contentModerationService,
		opsService:               opsService,
		concurrencyHelper:        NewConcurrencyHelper(concurrencyService, SSEPingFormatComment, pingInterval),
		maxAccountSwitches:       maxAccountSwitches,
		cfg:                      cfg,
	}
}

// Responses handles OpenAI Responses API endpoint
// POST /openai/v1/responses
func (h *OpenAIGatewayHandler) Responses(c *gin.Context) {
	// 局部兜底：确保该 handler 内部任何 panic 都不会击穿到进程级。
	streamStarted := false
	defer h.recoverResponsesPanic(c, &streamStarted)
	compactStartedAt := time.Now()
	defer h.logOpenAIRemoteCompactOutcome(c, compactStartedAt)
	setOpenAIClientTransportHTTP(c)

	requestStart := time.Now()

	// Get apiKey and user from context (set by ApiKeyAuth middleware)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.openai_gateway.responses",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	// Read request body
	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}

	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	setOpsRequestContext(c, "", false, body)
	sessionHashBody := body
	if service.IsOpenAIResponsesCompactPathForTest(c) {
		if compactSeed := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()); compactSeed != "" {
			c.Set(service.OpenAICompactSessionSeedKeyForTest(), compactSeed)
		}
		normalizedCompactBody, normalizedCompact, compactErr := service.NormalizeOpenAICompactRequestBodyForTest(body)
		if compactErr != nil {
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to normalize compact request body")
			return
		}
		if normalizedCompact {
			body = normalizedCompactBody
		}
	}

	// 校验请求体 JSON 合法性
	if !gjson.ValidBytes(body) {
		logRequestBodyParseFailure(reqLog, body, nil)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	// 使用 gjson 只读提取字段做校验，避免完整 Unmarshal
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()

	reqStream, ok := parseOpenAICompatibleStream(body)
	if !ok {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", invalidStreamFieldTypeMessage)
		return
	}
	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	if !isGroupModelAllowed(apiKey.Group, reqModel) {
		h.errorResponse(c, http.StatusForbidden, "permission_error", groupModelAccessDeniedMessage)
		return
	}
	if toolModel := disallowedResponsesImageToolModel(apiKey.Group, body); toolModel != "" {
		reqLog.Info("openai.responses.group_model_access_denied", zap.String("tool_model", toolModel))
		h.errorResponse(c, http.StatusForbidden, "permission_error", groupModelAccessDeniedMessage)
		return
	}
	previousResponseID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
	if previousResponseID != "" {
		previousResponseIDKind := service.ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
		reqLog = reqLog.With(
			zap.Bool("has_previous_response_id", true),
			zap.String("previous_response_id_kind", previousResponseIDKind),
			zap.Int("previous_response_id_len", len(previousResponseID)),
		)
		if previousResponseIDKind == service.OpenAIPreviousResponseIDKindMessageID {
			reqLog.Warn("openai.request_validation_failed",
				zap.String("reason", "previous_response_id_looks_like_message_id"),
			)
			h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "previous_response_id must be a response.id (resp_*), not a message id")
			return
		}
		reqLog.Warn("openai.request_validation_failed",
			zap.String("reason", "previous_response_id_requires_wsv2"),
		)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "previous_response_id is only supported on Responses WebSocket v2")
		return
	}

	setOpsRequestContext(c, reqModel, reqStream, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))
	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, reqModel, body); decision != nil && decision.Blocked {
		h.errorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return
	}

	// 解析渠道级模型映射
	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)

	// 提前校验 function_call_output 是否具备可关联上下文，避免上游 400。
	if !h.validateFunctionCallOutputRequest(c, body, reqLog) {
		return
	}

	// 绑定错误透传服务，允许 service 层在非 failover 错误场景复用规则。
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	// Get subscription info (may be nil)
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	requestPlatform := openAICompatibleRequestPlatform(apiKey)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted, reqLog)
	if !acquired {
		return
	}
	// 确保请求取消时也会释放槽位，避免长连接被动中断造成泄漏
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	// 2. Re-check billing eligibility after wait
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.handleStreamingAwareError(c, status, code, message, streamStarted)
		return
	}

	// Generate session hash (header first; fallback to prompt_cache_key)
	sessionHash := h.gatewayService.GenerateSessionHash(c, sessionHashBody)
	requireCompact := isOpenAIRemoteCompactPath(c)

	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError

	for {
		// Select account supporting the requested model
		reqLog.Debug("openai.account_selecting", zap.Int("excluded_account_count", len(failedAccountIDs)))
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
			c.Request.Context(),
			apiKey.GroupID,
			previousResponseID,
			sessionHash,
			reqModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportAny,
			service.OpenAIEndpointCapabilityChatCompletions,
			requireCompact,
			false,
			requestPlatform,
		)
		if err != nil {
			reqLog.Warn("openai.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if len(failedAccountIDs) == 0 {
				if errors.Is(err, service.ErrNoAvailableCompactAccounts) {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
					h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "compact_not_supported", "No available OpenAI accounts support /responses/compact", streamStarted)
					return
				}
				cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, reqModel, reqModel, service.PlatformOpenAI)
				if !cls.ModelNotFound {
					markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
				}
				h.handleStreamingAwareError(c, cls.Status, cls.ErrType, cls.Message, streamStarted)
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, streamStarted)
			} else {
				h.handleFailoverExhaustedSimple(c, 502, streamStarted)
			}
			return
		}
		if selection == nil || selection.Account == nil {
			cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, reqModel, reqModel, service.PlatformOpenAI)
			if !cls.ModelNotFound {
				markOpsRoutingCapacityLimited(c)
			}
			h.handleStreamingAwareError(c, cls.Status, cls.ErrType, cls.Message, streamStarted)
			return
		}
		if previousResponseID != "" && selection != nil && selection.Account != nil {
			reqLog.Debug("openai.account_selected_with_previous_response_id", zap.Int64("account_id", selection.Account.ID))
		}
		reqLog.Debug("openai.account_schedule_decision",
			zap.String("layer", scheduleDecision.Layer),
			zap.Bool("sticky_previous_hit", scheduleDecision.StickyPreviousHit),
			zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
			zap.Int("candidate_count", scheduleDecision.CandidateCount),
			zap.Int("top_k", scheduleDecision.TopK),
			zap.Int64("latency_ms", scheduleDecision.LatencyMs),
			zap.Float64("load_skew", scheduleDecision.LoadSkew),
		)
		account := selection.Account
		sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
		reqLog.Debug("openai.account_selected", zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc, acquired := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, reqStream, &streamStarted, reqLog)
		if !acquired {
			return
		}

		// Forward request
		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()
		// 应用渠道模型映射到请求体
		forwardBody := body
		if channelMapping.Mapped {
			forwardBody = h.gatewayService.ReplaceModelInBody(body, channelMapping.MappedModel)
		}
		h.gatewayService.MaybeSetDisplayTokenMultipliers(c.Request.Context(), c, apiKey, reqModel)
		writerSizeBeforeForward := c.Writer.Size()
		result, err := h.gatewayService.Forward(c.Request.Context(), c, account, forwardBody)
		cyberBlockKey := ""
		if service.GetOpsCyberPolicy(c) != nil {
			cyberBlockKey = service.CyberSessionBlockKey(apiKey.ID, c, sessionHashBody)
		}
		h.recordCyberPolicyIfMarked(c, apiKey, account, subscription, reqModel, err != nil, cyberBlockKey, channelMapping.ToUsageFields(reqModel, ""), service.HashUsageRequestPayload(body))
		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
		if err == nil && result != nil && result.FirstTokenMs != nil {
			service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
		}
		if err != nil {
			if openAIClientRequestCanceled(c) {
				reqLog.Debug("openai_messages.client_request_ended", zap.Error(c.Request.Context().Err()))
				return
			}
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
				// 池模式：同账号重试
				if failoverErr.RetryableOnSameAccount {
					retryLimit := account.GetPoolModeRetryCount()
					if sameAccountRetryCount[account.ID] < retryLimit {
						sameAccountRetryCount[account.ID]++
						reqLog.Warn("openai.pool_mode_same_account_retry",
							zap.Int64("account_id", account.ID),
							zap.Int("upstream_status", failoverErr.StatusCode),
							zap.Int("retry_limit", retryLimit),
							zap.Int("retry_count", sameAccountRetryCount[account.ID]),
						)
						select {
						case <-c.Request.Context().Done():
							return
						case <-time.After(sameAccountRetryDelay):
						}
						continue
					}
				}
				h.gatewayService.RecordOpenAIAccountSwitch()
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					h.handleFailoverExhausted(c, failoverErr, streamStarted)
					return
				}
				switchCount++
				reqLog.Warn("openai.upstream_failover_switching",
					zap.Int64("account_id", account.ID),
					zap.Int("upstream_status", failoverErr.StatusCode),
					zap.Int("switch_count", switchCount),
					zap.Int("max_switches", maxAccountSwitches),
				)
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
			upstreamErrorAlreadyCommunicated := openAIForwardErrorAlreadyCommunicated(c, writerSizeBeforeForward, err)
			wroteFallback := false
			if !upstreamErrorAlreadyCommunicated {
				wroteFallback = h.ensureForwardErrorResponse(c, streamStarted)
			}
			fields := []zap.Field{
				zap.Int64("account_id", account.ID),
				zap.Bool("fallback_error_response_written", wroteFallback),
				zap.Bool("upstream_error_response_already_written", upstreamErrorAlreadyCommunicated),
				zap.Error(err),
			}
			if shouldLogOpenAIForwardFailureAsWarn(c, wroteFallback) {
				reqLog.Warn("openai.forward_failed", fields...)
				return
			}
			reqLog.Error("openai.forward_failed", fields...)
			return
		}
		if result != nil {
			if account.Type == service.AccountTypeOAuth {
				h.gatewayService.UpdateCodexUsageSnapshotFromHeaders(c.Request.Context(), account.ID, result.ResponseHeaders)
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
		} else {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
		}

		// 捕获请求信息（用于异步记录，避免在 goroutine 中访问 gin.Context）
		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(body)
		inboundEndpoint := GetInboundEndpoint(c)
		upstreamEndpoint := resolveOpenAIUpstreamEndpoint(c, account)

		// 使用量记录通过有界 worker 池提交，避免请求热路径创建无界 goroutine。
		h.submitUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:             result,
				APIKey:             apiKey,
				User:               apiKey.User,
				Account:            account,
				Subscription:       subscription,
				InboundEndpoint:    inboundEndpoint,
				UpstreamEndpoint:   upstreamEndpoint,
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				RequestPayloadHash: requestPayloadHash,
				APIKeyService:      h.apiKeyService,
				ChannelUsageFields: channelMapping.ToUsageFields(reqModel, result.UpstreamModel),
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.responses"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.Any("group_id", apiKey.GroupID),
					zap.String("model", reqModel),
					zap.Int64("account_id", account.ID),
				).Error("openai.record_usage_failed", zap.Error(err))
			}
		})
		reqLog.Debug("openai.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return
	}
}

func isOpenAIRemoteCompactPath(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	normalizedPath := strings.TrimRight(strings.TrimSpace(c.Request.URL.Path), "/")
	return strings.HasSuffix(normalizedPath, "/responses/compact")
}

func (h *OpenAIGatewayHandler) logOpenAIRemoteCompactOutcome(c *gin.Context, startedAt time.Time) {
	if !isOpenAIRemoteCompactPath(c) {
		return
	}

	var (
		ctx    = context.Background()
		path   string
		status int
	)
	if c != nil {
		if c.Request != nil {
			ctx = c.Request.Context()
			if c.Request.URL != nil {
				path = strings.TrimSpace(c.Request.URL.Path)
			}
		}
		if c.Writer != nil {
			status = c.Writer.Status()
		}
	}

	outcome := "failed"
	if status >= 200 && status < 300 {
		outcome = "succeeded"
	}
	latencyMs := time.Since(startedAt).Milliseconds()
	if latencyMs < 0 {
		latencyMs = 0
	}

	fields := []zap.Field{
		zap.String("component", "handler.openai_gateway.responses"),
		zap.Bool("remote_compact", true),
		zap.String("compact_outcome", outcome),
		zap.Int("status_code", status),
		zap.Int64("latency_ms", latencyMs),
		zap.String("path", path),
		zap.Bool("force_codex_cli", h != nil && h.cfg != nil && h.cfg.Gateway.ForceCodexCLI),
	}

	if c != nil {
		if userAgent := strings.TrimSpace(c.GetHeader("User-Agent")); userAgent != "" {
			fields = append(fields, zap.String("request_user_agent", userAgent))
		}
		if v, ok := c.Get(opsModelKey); ok {
			if model, ok := v.(string); ok && strings.TrimSpace(model) != "" {
				fields = append(fields, zap.String("request_model", strings.TrimSpace(model)))
			}
		}
		if v, ok := c.Get(opsAccountIDKey); ok {
			if accountID, ok := v.(int64); ok && accountID > 0 {
				fields = append(fields, zap.Int64("account_id", accountID))
			}
		}
		if c.Writer != nil {
			if upstreamRequestID := strings.TrimSpace(c.Writer.Header().Get("x-request-id")); upstreamRequestID != "" {
				fields = append(fields, zap.String("upstream_request_id", upstreamRequestID))
			} else if upstreamRequestID := strings.TrimSpace(c.Writer.Header().Get("X-Request-Id")); upstreamRequestID != "" {
				fields = append(fields, zap.String("upstream_request_id", upstreamRequestID))
			}
		}
	}

	log := logger.FromContext(ctx).With(fields...)
	if outcome == "succeeded" {
		log.Info("codex.remote_compact.succeeded")
		return
	}
	log.Warn("codex.remote_compact.failed")
}

// Messages handles Anthropic Messages API requests routed to OpenAI platform.
// POST /v1/messages (when group platform is OpenAI)
func (h *OpenAIGatewayHandler) Messages(c *gin.Context) {
	streamStarted := false
	defer h.recoverAnthropicMessagesPanic(c, &streamStarted)

	requestStart := time.Now()

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
		"handler.openai_gateway.messages",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	// 检查分组是否允许 /v1/messages 调度
	bridgeMode := isOpenAIClaudeGPTBridgeRequest(c)
	if !bridgeMode && apiKey.Group != nil && !apiKey.Group.AllowMessagesDispatch {
		h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error",
			"This group does not allow /v1/messages dispatch")
		return
	}

	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, err := readLenientJSONRequestBodyWithPrealloc(c.Request, h.cfg)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.anthropicErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	if !gjson.ValidBytes(body) {
		logRequestBodyParseFailure(reqLog, body, nil)
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String || modelResult.String() == "" {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqModel := modelResult.String()
	routingModel := service.NormalizeOpenAICompatRequestedModel(reqModel)
	preferredMappedModel := ""
	if !bridgeMode {
		preferredMappedModel = resolveOpenAIMessagesDispatchMappedModel(apiKey, reqModel)
	}
	reqStream, ok := parseOpenAICompatibleStream(body)
	if !ok {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", invalidStreamFieldTypeMessage)
		return
	}

	reqLog = reqLog.With(zap.String("model", reqModel), zap.Bool("stream", reqStream))
	if bridgeMode && !isGroupModelAllowed(apiKey.Group, reqModel) {
		h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error", groupModelAccessDeniedMessage)
		return
	}

	setOpsRequestContext(c, reqModel, reqStream, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeFromLegacy(reqStream, false)))
	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolAnthropicMessages, reqModel, body); decision != nil && decision.Blocked {
		h.anthropicErrorResponse(c, contentModerationStatus(decision), contentModerationErrorCode(decision), decision.Message)
		return
	}

	channelMappingMsg := service.ChannelMappingResult{MappedModel: reqModel, BillingModelSource: service.BillingModelSourceRequested}
	if !bridgeMode {
		channelMappingMsg, _ = h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)
	}

	// 绑定错误透传服务，允许 service 层在非 failover 错误场景复用规则。
	if h.errorPassthroughService != nil {
		service.BindErrorPassthroughService(c, h.errorPassthroughService)
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	requestPlatform := openAICompatibleRequestPlatform(apiKey)

	service.SetOpsLatencyMs(c, service.OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())
	routingStart := time.Now()

	userReleaseFunc, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, reqStream, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userReleaseFunc != nil {
		defer userReleaseFunc()
	}

	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai_messages.billing_eligibility_check_failed", zap.Error(err))
		status, code, message, retryAfter := billingErrorDetails(err)
		if retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		h.anthropicStreamingAwareError(c, status, code, message, streamStarted)
		return
	}

	sessionHash := h.gatewayService.GenerateSessionHash(c, body)
	promptCacheKey := h.gatewayService.ExtractSessionID(c, body)

	// Anthropic 格式的请求在 metadata.user_id 中携带 session 标识，
	// 而非 OpenAI 的 session_id/conversation_id headers。
	// 从中派生 sessionHash（sticky session）和 promptCacheKey（upstream cache）。
	if sessionHash == "" || promptCacheKey == "" {
		if userID := strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String()); userID != "" {
			seed := reqModel + "-" + userID
			if promptCacheKey == "" {
				promptCacheKey = service.GenerateSessionUUID(seed)
			}
			if sessionHash == "" {
				sessionHash = service.DeriveSessionHashFromSeed(seed)
			}
		}
	}

	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	sameAccountRetryCount := make(map[int64]int)
	var lastFailoverErr *service.UpstreamFailoverError
	effectiveMappedModel := preferredMappedModel

	for {
		currentRoutingModel := routingModel
		if effectiveMappedModel != "" {
			currentRoutingModel = effectiveMappedModel
		}
		reqLog.Debug("openai_messages.account_selecting", zap.Int("excluded_account_count", len(failedAccountIDs)))
		var selection *service.AccountSelectionResult
		var scheduleDecision service.OpenAIAccountScheduleDecision
		var err error
		if bridgeMode {
			selection, scheduleDecision, err = h.gatewayService.SelectAccountWithSchedulerForClaudeGPTBridge(
				c.Request.Context(),
				apiKey.GroupID,
				sessionHash,
				reqModel,
				failedAccountIDs,
				service.OpenAIUpstreamTransportAny,
			)
		} else {
			selection, scheduleDecision, err = h.gatewayService.SelectAccountWithSchedulerForCapability(
				c.Request.Context(),
				apiKey.GroupID,
				"", // no previous_response_id
				sessionHash,
				currentRoutingModel,
				failedAccountIDs,
				service.OpenAIUpstreamTransportAny,
				service.OpenAIEndpointCapabilityChatCompletions,
				false,
				false,
				requestPlatform,
			)
		}
		if err != nil {
			reqLog.Warn("openai_messages.account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if bridgeMode && len(failedAccountIDs) == 0 {
				// 预检 ready 后真实选号立即失败：重新诊断一次，纯限流返回
				// 429，其余动态不可用返回 503，绝不回落 native。
				h.respondClaudeGPTBridgeSelectionRace(c, apiKey.GroupID, reqModel, streamStarted)
				return
			}
			if len(failedAccountIDs) == 0 {
				if err != nil {
					cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, currentRoutingModel, reqModel, service.PlatformOpenAI)
					if !cls.ModelNotFound {
						markOpsRoutingCapacityLimitedIfNoAvailable(c, err)
					}
					h.anthropicStreamingAwareError(c, cls.Status, cls.ErrType, cls.Message, streamStarted)
					return
				}
			} else {
				if lastFailoverErr != nil {
					h.handleAnthropicFailoverExhausted(c, lastFailoverErr, streamStarted)
				} else {
					h.anthropicStreamingAwareError(c, http.StatusBadGateway, "api_error", "Upstream request failed", streamStarted)
				}
				return
			}
		}
		if selection == nil || selection.Account == nil {
			if bridgeMode && len(failedAccountIDs) == 0 {
				h.respondClaudeGPTBridgeSelectionRace(c, apiKey.GroupID, reqModel, streamStarted)
				return
			}
			if bridgeMode {
				if lastFailoverErr != nil {
					h.handleAnthropicFailoverExhausted(c, lastFailoverErr, streamStarted)
				} else {
					h.anthropicStreamingAwareError(c, http.StatusBadGateway, "api_error", "Upstream request failed", streamStarted)
				}
				return
			}
			cls := classifyNoAccountErrorFromGin(c, h.gatewayService, apiKey, currentRoutingModel, reqModel, service.PlatformOpenAI)
			if !cls.ModelNotFound {
				markOpsRoutingCapacityLimited(c)
			}
			h.anthropicStreamingAwareError(c, cls.Status, cls.ErrType, cls.Message, streamStarted)
			return
		}
		account := selection.Account
		bridgeMappedModel := ""
		if bridgeMode {
			mapped, ok := account.ResolveClaudeGPTBridgeModel(reqModel)
			if !ok {
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				if len(failedAccountIDs) == 0 {
					// mapping 在选号后被删除也属于 bridge 侧错误，
					// 不允许半途切换 native。
					h.respondClaudeGPTBridgeSelectionRace(c, apiKey.GroupID, reqModel, streamStarted)
				} else if lastFailoverErr != nil {
					h.handleAnthropicFailoverExhausted(c, lastFailoverErr, streamStarted)
				} else {
					h.anthropicStreamingAwareError(c, http.StatusBadGateway, "api_error", "Upstream request failed", streamStarted)
				}
				return
			}
			bridgeMappedModel = mapped
			effectiveMappedModel = mapped
		}
		sessionHash = ensureOpenAIPoolModeSessionHash(sessionHash, account)
		reqLog.Debug("openai_messages.account_selected", zap.Int64("account_id", account.ID), zap.String("account_name", account.Name))
		_ = scheduleDecision
		setOpsSelectedAccount(c, account.ID, account.Platform)

		accountReleaseFunc, acquired := h.acquireResponsesAccountSlot(c, apiKey.GroupID, sessionHash, selection, reqStream, &streamStarted, reqLog)
		if !acquired {
			return
		}

		service.SetOpsLatencyMs(c, service.OpsRoutingLatencyMsKey, time.Since(routingStart).Milliseconds())
		forwardStart := time.Now()

		defaultMappedModel := strings.TrimSpace(effectiveMappedModel)
		if bridgeMode {
			defaultMappedModel = bridgeMappedModel
		}
		// 应用渠道模型映射到请求体
		forwardBody := body
		if !bridgeMode && channelMappingMsg.Mapped {
			forwardBody = h.gatewayService.ReplaceModelInBody(body, channelMappingMsg.MappedModel)
		}
		h.gatewayService.MaybeSetDisplayTokenMultipliers(c.Request.Context(), c, apiKey, reqModel)
		writerSizeBeforeForward := c.Writer.Size()
		result, err := h.gatewayService.ForwardAsAnthropic(c.Request.Context(), c, account, forwardBody, promptCacheKey, defaultMappedModel)
		cyberBlockKey := ""
		if service.GetOpsCyberPolicy(c) != nil {
			cyberBlockKey = service.CyberSessionBlockKey(apiKey.ID, c, body)
		}
		h.recordCyberPolicyIfMarked(c, apiKey, account, subscription, reqModel, err != nil, cyberBlockKey, channelMappingMsg.ToUsageFields(reqModel, ""), service.HashUsageRequestPayload(body))
		streamStarted = streamStarted || service.OpenAIAnthropicTransportStreamStarted(c)

		forwardDurationMs := time.Since(forwardStart).Milliseconds()
		if accountReleaseFunc != nil {
			accountReleaseFunc()
		}
		upstreamLatencyMs, _ := getContextInt64(c, service.OpsUpstreamLatencyMsKey)
		responseLatencyMs := forwardDurationMs
		if upstreamLatencyMs > 0 && forwardDurationMs > upstreamLatencyMs {
			responseLatencyMs = forwardDurationMs - upstreamLatencyMs
		}
		service.SetOpsLatencyMs(c, service.OpsResponseLatencyMsKey, responseLatencyMs)
		if err == nil && result != nil && result.FirstTokenMs != nil {
			service.SetOpsLatencyMs(c, service.OpsTimeToFirstTokenMsKey, int64(*result.FirstTokenMs))
		}
		if err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				writerSizeAfterForward := c.Writer.Size()
				if writerSizeAfterForward != writerSizeBeforeForward && result != nil && result.ClientOutputStarted {
					h.handleAnthropicFailoverExhausted(c, failoverErr, true)
					return
				}
				if h.isAnthropicClientFailoverError(failoverErr) {
					h.handleAnthropicFailoverExhausted(c, failoverErr, streamStarted)
					return
				}
				if writerSizeAfterForward != writerSizeBeforeForward {
					reqLog.Warn("openai_messages.retrying_after_pre_model_stream_output",
						zap.Int64("account_id", account.ID),
						zap.Int("writer_size_before", writerSizeBeforeForward),
						zap.Int("writer_size_after", writerSizeAfterForward),
						zap.Bool("client_output_started", result != nil && result.ClientOutputStarted),
					)
				}
				h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
				// 池模式：同账号重试
				if failoverErr.RetryableOnSameAccount {
					retryLimit := account.GetPoolModeRetryCount()
					if sameAccountRetryCount[account.ID] < retryLimit {
						sameAccountRetryCount[account.ID]++
						reqLog.Warn("openai_messages.pool_mode_same_account_retry",
							zap.Int64("account_id", account.ID),
							zap.Int("upstream_status", failoverErr.StatusCode),
							zap.Int("retry_limit", retryLimit),
							zap.Int("retry_count", sameAccountRetryCount[account.ID]),
						)
						select {
						case <-c.Request.Context().Done():
							return
						case <-time.After(sameAccountRetryDelay):
						}
						continue
					}
				}
				h.gatewayService.RecordOpenAIAccountSwitch()
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					h.handleAnthropicFailoverExhausted(c, failoverErr, streamStarted)
					return
				}
				switchCount++
				reqLog.Warn("openai_messages.upstream_failover_switching",
					zap.Int64("account_id", account.ID),
					zap.Int("upstream_status", failoverErr.StatusCode),
					zap.Int("switch_count", switchCount),
					zap.Int("max_switches", maxAccountSwitches),
				)
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, false, nil)
			wroteFallback := h.ensureAnthropicErrorResponse(c, streamStarted)
			reqLog.Warn("openai_messages.forward_failed",
				zap.Int64("account_id", account.ID),
				zap.Bool("fallback_error_response_written", wroteFallback),
				zap.Error(err),
			)
			return
		}
		if result != nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, result.FirstTokenMs)
		} else {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account.ID, true, nil)
		}

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(body)
		inboundEndpoint := GetInboundEndpoint(c)
		upstreamEndpoint := resolveOpenAIUpstreamEndpoint(c, account)

		h.submitUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:             result,
				APIKey:             apiKey,
				User:               apiKey.User,
				Account:            account,
				Subscription:       subscription,
				InboundEndpoint:    inboundEndpoint,
				UpstreamEndpoint:   upstreamEndpoint,
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				RequestPayloadHash: requestPayloadHash,
				APIKeyService:      h.apiKeyService,
				ChannelUsageFields: func() service.ChannelUsageFields {
					if bridgeMode {
						return bridgeUsageFields(reqModel, result.UpstreamModel)
					}
					return channelMappingMsg.ToUsageFields(reqModel, result.UpstreamModel)
				}(),
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.messages"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.Any("group_id", apiKey.GroupID),
					zap.String("model", reqModel),
					zap.Int64("account_id", account.ID),
				).Error("openai_messages.record_usage_failed", zap.Error(err))
			}
		})
		reqLog.Debug("openai_messages.request_completed",
			zap.Int64("account_id", account.ID),
			zap.Int("switch_count", switchCount),
		)
		return
	}
}

// anthropicErrorResponse writes an error in Anthropic Messages API format.
func (h *OpenAIGatewayHandler) anthropicErrorResponse(c *gin.Context, status int, errType, message string) {
	if service.OpenAIAnthropicResponseTerminated(c) {
		return
	}
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
	service.MarkOpenAIAnthropicResponseTerminated(c)
}

// anthropicStreamingAwareError handles errors that may occur during streaming,
// using Anthropic SSE error format.
func (h *OpenAIGatewayHandler) anthropicStreamingAwareError(c *gin.Context, status int, errType, message string, streamStarted bool) {
	if service.OpenAIAnthropicResponseTerminated(c) {
		return
	}
	streamStarted = streamStarted || service.OpenAIAnthropicTransportStreamStarted(c)
	if streamStarted {
		flusher, ok := c.Writer.(http.Flusher)
		if ok {
			errPayload, _ := json.Marshal(gin.H{
				"type": "error",
				"error": gin.H{
					"type":    errType,
					"message": message,
				},
			})
			fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", errPayload) //nolint:errcheck
			flusher.Flush()
			service.MarkOpenAIAnthropicResponseTerminated(c)
		}
		return
	}
	h.anthropicErrorResponse(c, status, errType, message)
}

func openAIClientRequestCanceled(c *gin.Context) bool {
	return c != nil && c.Request != nil && c.Request.Context().Err() != nil
}

// handleAnthropicFailoverExhausted maps upstream failover errors to Anthropic format.
func (h *OpenAIGatewayHandler) handleAnthropicFailoverExhausted(c *gin.Context, failoverErr *service.UpstreamFailoverError, streamStarted bool) {
	// bridge 模式所有账号都因 429 用尽时，透传经过校验的上游 Retry-After，
	// 让客户端按上游节奏重试。
	if isOpenAIClaudeGPTBridgeRequest(c) && failoverErr != nil && failoverErr.StatusCode == http.StatusTooManyRequests {
		if secs, ok := validatedUpstreamRetryAfterSeconds(failoverErr.ResponseHeaders); ok {
			c.Header("Retry-After", strconv.Itoa(secs))
		}
	}
	if status, errType, errMsg, ok := h.mapAnthropicFailoverBodyError(failoverErr); ok {
		h.anthropicStreamingAwareError(c, status, errType, errMsg, streamStarted)
		return
	}
	status, errType, errMsg := h.mapUpstreamError(failoverErr.StatusCode)
	h.anthropicStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

func (h *OpenAIGatewayHandler) isAnthropicClientFailoverError(failoverErr *service.UpstreamFailoverError) bool {
	if failoverErr == nil || failoverErr.StatusCode < 400 || failoverErr.StatusCode >= 500 {
		return false
	}
	errType := strings.TrimSpace(gjson.GetBytes(failoverErr.ResponseBody, "error.type").String())
	return errType == "invalid_request_error"
}

func (h *OpenAIGatewayHandler) mapAnthropicFailoverBodyError(failoverErr *service.UpstreamFailoverError) (int, string, string, bool) {
	if failoverErr == nil || len(failoverErr.ResponseBody) == 0 {
		return 0, "", "", false
	}
	errType := strings.TrimSpace(gjson.GetBytes(failoverErr.ResponseBody, "error.type").String())
	errMsg := strings.TrimSpace(gjson.GetBytes(failoverErr.ResponseBody, "error.message").String())
	if errType == "" || errMsg == "" {
		return 0, "", "", false
	}
	status := failoverErr.StatusCode
	if status < 400 || status >= 500 {
		status = http.StatusBadGateway
	}
	return status, errType, errMsg, true
}

// ensureAnthropicErrorResponse writes a fallback Anthropic error if no response was written.
func (h *OpenAIGatewayHandler) ensureAnthropicErrorResponse(c *gin.Context, streamStarted bool) bool {
	if c == nil || c.Writer == nil || c.Writer.Written() {
		return false
	}
	h.anthropicStreamingAwareError(c, http.StatusBadGateway, "api_error", "Upstream request failed", streamStarted)
	return true
}

func (h *OpenAIGatewayHandler) validateFunctionCallOutputRequest(c *gin.Context, body []byte, reqLog *zap.Logger) bool {
	if !gjson.GetBytes(body, `input.#(type=="function_call_output")`).Exists() &&
		!gjson.GetBytes(body, `input.#(type=="tool_search_output")`).Exists() &&
		!gjson.GetBytes(body, `input.#(type=="custom_tool_call_output")`).Exists() &&
		!gjson.GetBytes(body, `input.#(type=="mcp_tool_call_output")`).Exists() {
		return true
	}

	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		// 保持原有容错语义：解析失败时跳过预校验，沿用后续上游校验结果。
		return true
	}

	service.CacheOpenAIParsedRequestBody(c, body, reqBody)
	validation := service.ValidateFunctionCallOutputContextBytes(body)
	if !validation.HasFunctionCallOutput {
		return true
	}

	previousResponseID := gjson.GetBytes(body, "previous_response_id").String()
	if strings.TrimSpace(previousResponseID) != "" || validation.HasToolCallContext {
		return true
	}

	if validation.HasFunctionCallOutputMissingCallID {
		reqLog.Warn("openai.request_validation_failed",
			zap.String("reason", "function_call_output_missing_call_id"),
		)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "function_call_output requires call_id on HTTP requests; continuation via previous_response_id is only supported on Responses WebSocket v2")
		return false
	}
	if validation.HasItemReferenceForAllCallIDs {
		return true
	}

	reqLog.Warn("openai.request_validation_failed",
		zap.String("reason", "function_call_output_missing_item_reference"),
	)
	h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "function_call_output requires item_reference ids matching each call_id on HTTP requests; continuation via previous_response_id is only supported on Responses WebSocket v2")
	return false
}

func (h *OpenAIGatewayHandler) acquireResponsesUserSlot(
	c *gin.Context,
	userID int64,
	userConcurrency int,
	reqStream bool,
	streamStarted *bool,
	reqLog *zap.Logger,
) (func(), bool) {
	ctx := c.Request.Context()
	userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, userID, userConcurrency)
	if err != nil {
		reqLog.Warn("openai.user_slot_acquire_failed", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", *streamStarted)
		return nil, false
	}
	if userAcquired {
		return wrapReleaseOnDone(ctx, userReleaseFunc), true
	}

	maxWait := service.CalculateMaxWait(userConcurrency)
	canWait, waitErr := h.concurrencyHelper.IncrementWaitCount(ctx, userID, maxWait)
	if waitErr != nil {
		reqLog.Warn("openai.user_wait_counter_increment_failed", zap.Error(waitErr))
		// 按现有降级语义：等待计数异常时放行后续抢槽流程
	} else if !canWait {
		reqLog.Info("openai.user_wait_queue_full", zap.Int("max_wait", maxWait))
		h.errorResponse(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later")
		return nil, false
	}

	waitCounted := waitErr == nil && canWait
	defer func() {
		if waitCounted {
			h.concurrencyHelper.DecrementWaitCount(ctx, userID)
		}
	}()

	userReleaseFunc, err = h.concurrencyHelper.AcquireUserSlotWithWait(c, userID, userConcurrency, reqStream, streamStarted)
	if err != nil {
		reqLog.Warn("openai.user_slot_acquire_failed_after_wait", zap.Error(err))
		h.handleConcurrencyError(c, err, "user", *streamStarted)
		return nil, false
	}

	// 槽位获取成功后，立刻退出等待计数。
	if waitCounted {
		h.concurrencyHelper.DecrementWaitCount(ctx, userID)
		waitCounted = false
	}
	return wrapReleaseOnDone(ctx, userReleaseFunc), true
}

func (h *OpenAIGatewayHandler) acquireResponsesAccountSlot(
	c *gin.Context,
	groupID *int64,
	sessionHash string,
	selection *service.AccountSelectionResult,
	reqStream bool,
	streamStarted *bool,
	reqLog *zap.Logger,
) (func(), bool) {
	if selection == nil || selection.Account == nil {
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", *streamStarted)
		return nil, false
	}

	ctx := c.Request.Context()
	account := selection.Account
	if selection.Acquired {
		return wrapReleaseOnDone(ctx, selection.ReleaseFunc), true
	}
	if selection.WaitPlan == nil {
		h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "No available accounts", *streamStarted)
		return nil, false
	}

	fastReleaseFunc, fastAcquired, err := h.concurrencyHelper.TryAcquireAccountSlot(
		ctx,
		account.ID,
		selection.WaitPlan.MaxConcurrency,
	)
	if err != nil {
		reqLog.Warn("openai.account_slot_quick_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		h.handleConcurrencyError(c, err, "account", *streamStarted)
		return nil, false
	}
	if fastAcquired {
		if err := h.gatewayService.BindStickySession(ctx, groupID, sessionHash, account.ID); err != nil {
			reqLog.Warn("openai.bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}
		return wrapReleaseOnDone(ctx, fastReleaseFunc), true
	}

	canWait, waitErr := h.concurrencyHelper.IncrementAccountWaitCount(ctx, account.ID, selection.WaitPlan.MaxWaiting)
	if waitErr != nil {
		reqLog.Warn("openai.account_wait_counter_increment_failed", zap.Int64("account_id", account.ID), zap.Error(waitErr))
	} else if !canWait {
		reqLog.Info("openai.account_wait_queue_full",
			zap.Int64("account_id", account.ID),
			zap.Int("max_waiting", selection.WaitPlan.MaxWaiting),
		)
		h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later", *streamStarted)
		return nil, false
	}

	accountWaitCounted := waitErr == nil && canWait
	releaseWait := func() {
		if accountWaitCounted {
			h.concurrencyHelper.DecrementAccountWaitCount(ctx, account.ID)
			accountWaitCounted = false
		}
	}
	defer releaseWait()

	accountReleaseFunc, err := h.concurrencyHelper.AcquireAccountSlotWithWaitTimeout(
		c,
		account.ID,
		selection.WaitPlan.MaxConcurrency,
		selection.WaitPlan.Timeout,
		reqStream,
		streamStarted,
	)
	if err != nil {
		reqLog.Warn("openai.account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		h.handleConcurrencyError(c, err, "account", *streamStarted)
		return nil, false
	}

	// Slot acquired: no longer waiting in queue.
	releaseWait()
	if err := h.gatewayService.BindStickySession(ctx, groupID, sessionHash, account.ID); err != nil {
		reqLog.Warn("openai.bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
	}
	return wrapReleaseOnDone(ctx, accountReleaseFunc), true
}

// ResponsesWebSocket handles OpenAI Responses API WebSocket ingress endpoint
// GET /openai/v1/responses (Upgrade: websocket)
func (h *OpenAIGatewayHandler) ResponsesWebSocket(c *gin.Context) {
	if !isOpenAIWSUpgradeRequest(c.Request) {
		h.errorResponse(c, http.StatusUpgradeRequired, "invalid_request_error", "WebSocket upgrade required (Upgrade: websocket)")
		return
	}
	setOpenAIClientTransportWS(c)

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if openAICompatibleRequestPlatform(apiKey) == service.PlatformGrok {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		h.errorResponse(c, http.StatusNotImplemented, "not_supported_error", "WebSocket responses are not supported for this platform")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}

	reqLog := requestLogger(
		c,
		"handler.openai_gateway.responses_ws",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
		zap.Bool("openai_ws_mode", true),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}
	reqLog.Info("openai.websocket_ingress_started")
	clientIP := ip.GetClientIP(c)
	userAgent := strings.TrimSpace(c.GetHeader("User-Agent"))

	wsConn, err := coderws.Accept(c.Writer, c.Request, &coderws.AcceptOptions{
		CompressionMode: coderws.CompressionContextTakeover,
	})
	if err != nil {
		reqLog.Warn("openai.websocket_accept_failed",
			zap.Error(err),
			zap.String("client_ip", clientIP),
			zap.String("request_user_agent", userAgent),
			zap.String("upgrade_header", strings.TrimSpace(c.GetHeader("Upgrade"))),
			zap.String("connection_header", strings.TrimSpace(c.GetHeader("Connection"))),
			zap.String("sec_websocket_version", strings.TrimSpace(c.GetHeader("Sec-WebSocket-Version"))),
			zap.Bool("has_sec_websocket_key", strings.TrimSpace(c.GetHeader("Sec-WebSocket-Key")) != ""),
		)
		return
	}
	defer func() {
		_ = wsConn.CloseNow()
	}()
	wsConn.SetReadLimit(service.ResolveOpenAIWSClientReadLimitBytes(h.cfg))

	ctx := c.Request.Context()
	readCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	msgType, firstMessage, err := wsConn.Read(readCtx)
	cancel()
	if err != nil {
		closeStatus, closeReason := summarizeWSCloseErrorForLog(err)
		reqLog.Warn("openai.websocket_read_first_message_failed",
			zap.Error(err),
			zap.String("client_ip", clientIP),
			zap.String("close_status", closeStatus),
			zap.String("close_reason", closeReason),
			zap.Duration("read_timeout", 30*time.Second),
		)
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "missing first response.create message")
		return
	}
	if msgType != coderws.MessageText && msgType != coderws.MessageBinary {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "unsupported websocket message type")
		return
	}
	if !gjson.ValidBytes(firstMessage) {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "invalid JSON payload")
		return
	}

	reqModel := strings.TrimSpace(gjson.GetBytes(firstMessage, "model").String())
	if reqModel == "" {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "model is required in first response.create payload")
		return
	}
	if !isGroupModelAllowed(apiKey.Group, reqModel) {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, groupModelAccessDeniedMessage)
		return
	}
	if toolModel := disallowedResponsesImageToolModel(apiKey.Group, firstMessage); toolModel != "" {
		reqLog.Info("openai.websocket.group_model_access_denied", zap.String("tool_model", toolModel))
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, groupModelAccessDeniedMessage)
		return
	}
	previousResponseID := strings.TrimSpace(gjson.GetBytes(firstMessage, "previous_response_id").String())
	previousResponseIDKind := service.ClassifyOpenAIPreviousResponseIDKind(previousResponseID)
	if previousResponseID != "" && previousResponseIDKind == service.OpenAIPreviousResponseIDKindMessageID {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "previous_response_id must be a response.id (resp_*), not a message id")
		return
	}
	toolCoverage := service.AnalyzeToolCallOutputContextCoverageBytes(firstMessage)
	previousResponseCanMove := !toolCoverage.HasFunctionCallOutput || toolCoverage.ContextCoversAllCallIDs
	reqLog = reqLog.With(
		zap.Bool("ws_ingress", true),
		zap.String("model", reqModel),
		zap.Bool("has_previous_response_id", previousResponseID != ""),
		zap.String("previous_response_id_kind", previousResponseIDKind),
	)
	setOpsRequestContext(c, reqModel, true, firstMessage)
	setOpsEndpointContext(c, "", int16(service.RequestTypeWSV2))
	if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, reqModel, firstMessage); decision != nil && decision.Blocked {
		writeContentModerationWSError(ctx, wsConn, decision)
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, decision.Message)
		return
	}
	cyberBlockKey := service.CyberSessionBlockKey(apiKey.ID, c, nil)
	if cyberBlockKey != "" && h.gatewayService.IsCyberSessionBlocked(c.Request.Context(), cyberBlockKey) {
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "session blocked by cyber-security policy")
		return
	}
	cyberBlockedThisConn := false

	// 解析渠道级模型映射
	channelMappingWS, _ := h.gatewayService.ResolveChannelMappingAndRestrict(ctx, apiKey.GroupID, reqModel)

	var currentUserRelease func()
	var currentAccountRelease func()
	releaseTurnSlots := func() {
		if currentAccountRelease != nil {
			currentAccountRelease()
			currentAccountRelease = nil
		}
		if currentUserRelease != nil {
			currentUserRelease()
			currentUserRelease = nil
		}
	}
	releaseAccountSlot := func() {
		if currentAccountRelease != nil {
			currentAccountRelease()
			currentAccountRelease = nil
		}
	}
	// 必须尽早注册，确保任何 early return 都能释放已获取的并发槽位。
	defer releaseTurnSlots()

	userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
	if err != nil {
		reqLog.Warn("openai.websocket_user_slot_acquire_failed", zap.Error(err))
		closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire user concurrency slot")
		return
	}
	if !userAcquired {
		closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "too many concurrent requests, please retry later")
		return
	}
	currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
	ensureUserSlotHeld := func() bool {
		if currentUserRelease != nil {
			return true
		}
		userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
		if err != nil {
			reqLog.Warn("openai.websocket_user_slot_reacquire_failed", zap.Error(err))
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire user concurrency slot")
			return false
		}
		if !userAcquired {
			closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "too many concurrent requests, please retry later")
			return false
		}
		currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
		return true
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(ctx, apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		reqLog.Info("openai.websocket_billing_eligibility_check_failed", zap.Error(err))
		closeOpenAIClientWS(wsConn, coderws.StatusPolicyViolation, "billing check failed")
		return
	}

	sessionHash := h.gatewayService.GenerateSessionHashWithFallback(
		c,
		firstMessage,
		openAIWSIngressFallbackSessionSeed(subject.UserID, apiKey.ID, apiKey.GroupID),
	)
	maxAccountSwitches := h.maxAccountSwitches
	switchCount := 0
	failedAccountIDs := make(map[int64]struct{})
	var lastFailoverErr *service.UpstreamFailoverError
	for {
		selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
			ctx,
			apiKey.GroupID,
			previousResponseID,
			sessionHash,
			reqModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportResponsesWebsocketV2,
			service.OpenAIEndpointCapabilityChatCompletions,
			false,
			previousResponseCanMove,
		)
		if err != nil {
			reqLog.Warn("openai.websocket_account_select_failed",
				zap.Error(err),
				zap.Int("excluded_account_count", len(failedAccountIDs)),
			)
			if lastFailoverErr != nil {
				closeOpenAIWSFailoverExhausted(wsConn, lastFailoverErr)
			} else {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "no available account")
			}
			return
		}
		if selection == nil || selection.Account == nil {
			if lastFailoverErr != nil {
				closeOpenAIWSFailoverExhausted(wsConn, lastFailoverErr)
			} else {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "no available account")
			}
			return
		}

		account := selection.Account
		accountMaxConcurrency := account.Concurrency
		if selection.WaitPlan != nil && selection.WaitPlan.MaxConcurrency > 0 {
			accountMaxConcurrency = selection.WaitPlan.MaxConcurrency
		}
		accountReleaseFunc := selection.ReleaseFunc
		if !selection.Acquired {
			if selection.WaitPlan == nil {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "account is busy, please retry later")
				return
			}
			fastReleaseFunc, fastAcquired, err := h.concurrencyHelper.TryAcquireAccountSlot(
				ctx,
				account.ID,
				selection.WaitPlan.MaxConcurrency,
			)
			if err != nil {
				reqLog.Warn("openai.websocket_account_slot_acquire_failed", zap.Int64("account_id", account.ID), zap.Error(err))
				closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to acquire account concurrency slot")
				return
			}
			if !fastAcquired {
				closeOpenAIClientWS(wsConn, coderws.StatusTryAgainLater, "account is busy, please retry later")
				return
			}
			accountReleaseFunc = fastReleaseFunc
		}
		currentAccountRelease = wrapReleaseOnDone(ctx, accountReleaseFunc)
		if err := h.gatewayService.BindStickySession(ctx, apiKey.GroupID, sessionHash, account.ID); err != nil {
			reqLog.Warn("openai.websocket_bind_sticky_session_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}

		token, _, err := h.gatewayService.GetAccessToken(ctx, account)
		if err != nil {
			reqLog.Warn("openai.websocket_get_access_token_failed", zap.Int64("account_id", account.ID), zap.Error(err))
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "failed to get access token")
			return
		}

		reqLog.Debug("openai.websocket_account_selected",
			zap.Int64("account_id", account.ID),
			zap.String("account_name", account.Name),
			zap.String("schedule_layer", scheduleDecision.Layer),
			zap.Int("candidate_count", scheduleDecision.CandidateCount),
		)

		turnAccount := account
		hooks := &service.OpenAIWSIngressHooks{
			BeforeRequest: func(turn int, payload []byte, originalModel string) error {
				if turn == 1 {
					return nil
				}
				model := strings.TrimSpace(originalModel)
				if model == "" {
					model = strings.TrimSpace(gjson.GetBytes(payload, "model").String())
				}
				if model == "" {
					model = reqModel
				}
				if decision := h.checkContentModeration(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, model, payload); decision != nil && decision.Blocked {
					writeContentModerationWSError(ctx, wsConn, decision)
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, decision.Message, nil)
				}
				return nil
			},
			BeforeTurn: func(turn int) error {
				if cyberBlockedThisConn {
					return service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "session blocked by cyber-security policy", nil)
				}
				if turn == 1 {
					return nil
				}
				// 防御式清理：避免异常路径下旧槽位覆盖导致泄漏。
				releaseTurnSlots()
				// 非首轮 turn 需要重新抢占并发槽位，避免长连接空闲占槽。
				userReleaseFunc, userAcquired, err := h.concurrencyHelper.TryAcquireUserSlot(ctx, subject.UserID, subject.Concurrency)
				if err != nil {
					return service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "failed to acquire user concurrency slot", err)
				}
				if !userAcquired {
					return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "too many concurrent requests, please retry later", nil)
				}
				accountReleaseFunc, accountAcquired, err := h.concurrencyHelper.TryAcquireAccountSlot(ctx, turnAccount.ID, accountMaxConcurrency)
				if err != nil {
					if userReleaseFunc != nil {
						userReleaseFunc()
					}
					return service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "failed to acquire account concurrency slot", err)
				}
				if !accountAcquired {
					if userReleaseFunc != nil {
						userReleaseFunc()
					}
					return service.NewOpenAIWSClientCloseError(coderws.StatusTryAgainLater, "account is busy, please retry later", nil)
				}
				currentUserRelease = wrapReleaseOnDone(ctx, userReleaseFunc)
				currentAccountRelease = wrapReleaseOnDone(ctx, accountReleaseFunc)
				return nil
			},
			AfterTurn: func(turn int, result *service.OpenAIForwardResult, turnErr error) {
				defer clearCyberPolicyTurnState(c)
				releaseTurnSlots()
				h.recordCyberPolicyIfMarked(c, apiKey, turnAccount, subscription, reqModel, turnErr != nil, cyberBlockKey, channelMappingWS.ToUsageFields(reqModel, ""), service.HashUsageRequestPayload(firstMessage))
				if service.GetOpsCyberPolicy(c) != nil {
					cyberBlockedThisConn = true
				}
				if turnErr != nil || result == nil {
					return
				}
				if turnAccount.Type == service.AccountTypeOAuth {
					h.gatewayService.UpdateCodexUsageSnapshotFromHeaders(ctx, turnAccount.ID, result.ResponseHeaders)
				}
				h.gatewayService.ReportOpenAIAccountScheduleResult(turnAccount.ID, true, result.FirstTokenMs)
				inboundEndpoint := GetInboundEndpoint(c)
				upstreamEndpoint := resolveOpenAIUpstreamEndpoint(c, turnAccount)
				h.submitUsageRecordTask(turnUsageRecordContext(ctx, turn, result.RequestID), func(taskCtx context.Context) {
					if err := h.gatewayService.RecordUsage(taskCtx, &service.OpenAIRecordUsageInput{
						Result:             result,
						APIKey:             apiKey,
						User:               apiKey.User,
						Account:            turnAccount,
						Subscription:       subscription,
						InboundEndpoint:    inboundEndpoint,
						UpstreamEndpoint:   upstreamEndpoint,
						UserAgent:          userAgent,
						IPAddress:          clientIP,
						RequestPayloadHash: service.HashUsageRequestPayload(firstMessage),
						APIKeyService:      h.apiKeyService,
						ChannelUsageFields: channelMappingWS.ToUsageFields(reqModel, result.UpstreamModel),
					}); err != nil {
						reqLog.Error("openai.websocket_record_usage_failed",
							zap.Int64("account_id", turnAccount.ID),
							zap.String("request_id", result.RequestID),
							zap.Error(err),
						)
					}
				})
			},
		}

		// 应用渠道模型映射到 WebSocket 首条消息
		wsFirstMessage := firstMessage
		if channelMappingWS.Mapped {
			wsFirstMessage = h.gatewayService.ReplaceModelInBody(firstMessage, channelMappingWS.MappedModel)
		}
		if previousResponseID != "" && !scheduleDecision.StickyPreviousHit && previousResponseCanMove {
			wsFirstMessage = service.RemovePreviousResponseIDFromBody(wsFirstMessage)
			reqLog.Debug("openai.websocket_previous_response_id_stripped_cross_group",
				zap.Int64("account_id", account.ID),
				zap.String("schedule_layer", scheduleDecision.Layer),
			)
		}

		if err := h.gatewayService.ProxyResponsesWebSocketFromClient(ctx, c, wsConn, turnAccount, token, wsFirstMessage, hooks); err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) {
				h.gatewayService.ReportOpenAIAccountScheduleResult(turnAccount.ID, false, nil)
				releaseAccountSlot()
				h.gatewayService.RecordOpenAIAccountSwitch()
				failedAccountIDs[turnAccount.ID] = struct{}{}
				lastFailoverErr = failoverErr
				if switchCount >= maxAccountSwitches {
					closeOpenAIWSFailoverExhausted(wsConn, failoverErr)
					return
				}
				switchCount++
				reqLog.Warn("openai.websocket_upstream_failover_switching",
					zap.Int64("account_id", turnAccount.ID),
					zap.Int("upstream_status", failoverErr.StatusCode),
					zap.Int("switch_count", switchCount),
					zap.Int("max_switches", maxAccountSwitches),
				)
				if !ensureUserSlotHeld() {
					return
				}
				continue
			}
			h.gatewayService.ReportOpenAIAccountScheduleResult(turnAccount.ID, false, nil)
			closeStatus, closeReason := summarizeWSCloseErrorForLog(err)
			reqLog.Warn("openai.websocket_proxy_failed",
				zap.Int64("account_id", turnAccount.ID),
				zap.Error(err),
				zap.String("close_status", closeStatus),
				zap.String("close_reason", closeReason),
			)
			var closeErr *service.OpenAIWSClientCloseError
			if errors.As(err, &closeErr) {
				closeOpenAIClientWS(wsConn, closeErr.StatusCode(), closeErr.Reason())
				return
			}
			closeOpenAIClientWS(wsConn, coderws.StatusInternalError, "upstream websocket proxy failed")
			return
		}
		reqLog.Info("openai.websocket_ingress_closed", zap.Int64("account_id", turnAccount.ID))
		return
	}
}

func (h *OpenAIGatewayHandler) recoverResponsesPanic(c *gin.Context, streamStarted *bool) {
	recovered := recover()
	if recovered == nil {
		return
	}

	started := false
	if streamStarted != nil {
		started = *streamStarted
	}
	wroteFallback := h.ensureForwardErrorResponse(c, started)
	requestLogger(c, "handler.openai_gateway.responses").Error(
		"openai.responses_panic_recovered",
		zap.Bool("fallback_error_response_written", wroteFallback),
		zap.Any("panic", recovered),
		zap.ByteString("stack", debug.Stack()),
	)
}

// recoverAnthropicMessagesPanic recovers from panics in the Anthropic Messages
// handler and returns an Anthropic-formatted error response.
func (h *OpenAIGatewayHandler) recoverAnthropicMessagesPanic(c *gin.Context, streamStarted *bool) {
	recovered := recover()
	if recovered == nil {
		return
	}

	started := (streamStarted != nil && *streamStarted) || service.OpenAIAnthropicTransportStreamStarted(c)
	requestLogger(c, "handler.openai_gateway.messages").Error(
		"openai.messages_panic_recovered",
		zap.Bool("stream_started", started),
		zap.Any("panic", recovered),
		zap.ByteString("stack", debug.Stack()),
	)
	if service.OpenAIAnthropicResponseTerminated(c) {
		return
	}
	h.anthropicStreamingAwareError(c, http.StatusInternalServerError, "api_error", "Internal server error", started)
}

func (h *OpenAIGatewayHandler) ensureResponsesDependencies(c *gin.Context, reqLog *zap.Logger) bool {
	missing := h.missingResponsesDependencies()
	if len(missing) == 0 {
		return true
	}

	if reqLog == nil {
		reqLog = requestLogger(c, "handler.openai_gateway.responses")
	}
	reqLog.Error("openai.handler_dependencies_missing", zap.Strings("missing_dependencies", missing))

	if c != nil && c.Writer != nil && !c.Writer.Written() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"type":    "api_error",
				"message": "Service temporarily unavailable",
			},
		})
	}
	return false
}

func (h *OpenAIGatewayHandler) missingResponsesDependencies() []string {
	missing := make([]string, 0, 5)
	if h == nil {
		return append(missing, "handler")
	}
	if h.gatewayService == nil {
		missing = append(missing, "gatewayService")
	}
	if h.billingCacheService == nil {
		missing = append(missing, "billingCacheService")
	}
	if h.apiKeyService == nil {
		missing = append(missing, "apiKeyService")
	}
	if h.concurrencyHelper == nil || h.concurrencyHelper.concurrencyService == nil {
		missing = append(missing, "concurrencyHelper")
	}
	return missing
}

func getContextInt64(c *gin.Context, key string) (int64, bool) {
	if c == nil || key == "" {
		return 0, false
	}
	v, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}

func (h *OpenAIGatewayHandler) submitUsageRecordTask(parent context.Context, task service.UsageRecordTask) {
	if task == nil {
		return
	}
	task = wrapUsageRecordTaskContext(parent, task)
	if h.usageRecordWorkerPool != nil {
		h.usageRecordWorkerPool.Submit(task)
		return
	}
	// 回退路径：worker 池未注入时同步执行，避免退回到无界 goroutine 模式。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.responses"),
				zap.Any("panic", recovered),
			).Error("openai.usage_record_task_panic_recovered")
		}
	}()
	task(ctx)
}

// handleConcurrencyError handles concurrency-related errors with proper 429 response
func writeContentModerationWSError(ctx context.Context, conn *coderws.Conn, decision *service.ContentModerationDecision) {
	if conn == nil || decision == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	message := strings.TrimSpace(decision.Message)
	if message == "" {
		message = "content moderation blocked this request"
	}
	payload, err := json.Marshal(gin.H{
		"event_id": "evt_content_moderation_blocked",
		"type":     "error",
		"error": gin.H{
			"type":    "invalid_request_error",
			"code":    contentModerationErrorCode(decision),
			"message": message,
		},
	})
	if err != nil {
		payload = []byte(`{"event_id":"evt_content_moderation_blocked","type":"error","error":{"type":"invalid_request_error","code":"content_policy_violation","message":"content moderation blocked this request"}}`)
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = conn.Write(writeCtx, coderws.MessageText, payload)
}

func (h *OpenAIGatewayHandler) handleConcurrencyError(c *gin.Context, err error, slotType string, streamStarted bool) {
	h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error",
		fmt.Sprintf("Concurrency limit exceeded for %s, please retry later", slotType), streamStarted)
}

func (h *OpenAIGatewayHandler) handleFailoverExhausted(c *gin.Context, failoverErr *service.UpstreamFailoverError, streamStarted bool) {
	statusCode := failoverErr.StatusCode
	responseBody := failoverErr.ResponseBody

	// 先检查透传规则
	if h.errorPassthroughService != nil && len(responseBody) > 0 {
		if rule := h.errorPassthroughService.MatchRule("openai", statusCode, responseBody); rule != nil {
			// 确定响应状态码
			respCode := statusCode
			if !rule.PassthroughCode && rule.ResponseCode != nil {
				respCode = *rule.ResponseCode
			}

			// 确定响应消息
			msg := service.ExtractUpstreamErrorMessage(responseBody)
			if !rule.PassthroughBody && rule.CustomMessage != nil {
				msg = *rule.CustomMessage
			}

			if rule.SkipMonitoring {
				c.Set(service.OpsSkipPassthroughKey, true)
			}

			h.handleStreamingAwareError(c, respCode, "upstream_error", msg, streamStarted)
			return
		}
	}

	// 记录原始上游状态码，以便 ops 错误日志捕获真实的上游错误
	upstreamMsg := service.ExtractUpstreamErrorMessage(responseBody)
	service.SetOpsUpstreamError(c, statusCode, upstreamMsg, "")
	if service.IsOpenAISilentRefusalErrorBody(responseBody) {
		service.SetOpsUpstreamError(c, statusCode, service.OpenAISilentRefusalClientMessage(), "")
		h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", service.OpenAISilentRefusalClientMessage(), streamStarted)
		return
	}

	// 使用默认的错误映射
	status, errType, errMsg := h.mapUpstreamError(statusCode)
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

// handleFailoverExhaustedSimple 简化版本，用于没有响应体的情况
func (h *OpenAIGatewayHandler) handleFailoverExhaustedSimple(c *gin.Context, statusCode int, streamStarted bool) {
	status, errType, errMsg := h.mapUpstreamError(statusCode)
	service.SetOpsUpstreamError(c, statusCode, errMsg, "")
	h.handleStreamingAwareError(c, status, errType, errMsg, streamStarted)
}

func (h *OpenAIGatewayHandler) mapUpstreamError(statusCode int) (int, string, string) {
	switch statusCode {
	case 401:
		return http.StatusBadGateway, "upstream_error", "Upstream authentication failed, please contact administrator"
	case 403:
		return http.StatusBadGateway, "upstream_error", "Upstream access forbidden, please contact administrator"
	case 429:
		return http.StatusTooManyRequests, "rate_limit_error", "Upstream rate limit exceeded, please retry later"
	case 529:
		return http.StatusServiceUnavailable, "upstream_error", "Upstream service overloaded, please retry later"
	case 500, 502, 503, 504:
		return http.StatusBadGateway, "upstream_error", "Upstream service temporarily unavailable"
	default:
		return http.StatusBadGateway, "upstream_error", "Upstream request failed"
	}
}

// handleStreamingAwareError handles errors that may occur after streaming has started
func (h *OpenAIGatewayHandler) handleStreamingAwareError(c *gin.Context, status int, errType, message string, streamStarted bool) {
	if streamStarted {
		if inboundIsResponses(c) {
			if writeResponsesFailedSSE(c, errType, message) {
				return
			}
		}
		// Stream already started, send error as SSE event then close
		flusher, ok := c.Writer.(http.Flusher)
		if ok {
			if imageTrace := service.OpenAIImageTraceFromGin(c); imageTrace != nil {
				imageTrace.Log(c, "downstream_response_built", status, "")
			}
			// SSE 错误事件固定 schema，使用 Quote 直拼可避免额外 Marshal 分配。
			errorEvent := "event: error\ndata: " + `{"error":{"type":` + strconv.Quote(errType) + `,"message":` + strconv.Quote(message) + `}}` + "\n\n"
			if _, err := fmt.Fprint(c.Writer, errorEvent); err != nil {
				_ = c.Error(err)
			}
			flusher.Flush()
			if imageTrace := service.OpenAIImageTraceFromGin(c); imageTrace != nil {
				imageTrace.Log(c, "downstream_write_done", status, "")
			}
		}
		return
	}

	// Normal case: return JSON response with proper status code
	h.errorResponse(c, status, errType, message)
}

// ensureForwardErrorResponse 在 Forward 返回错误但尚未写响应时补写统一错误响应。
func (h *OpenAIGatewayHandler) ensureForwardErrorResponse(c *gin.Context, streamStarted bool) bool {
	if c == nil || c.Writer == nil {
		return false
	}
	if c.Writer.Written() {
		streamStarted = true
	}
	h.handleStreamingAwareError(c, http.StatusBadGateway, "upstream_error", "Upstream request failed", streamStarted)
	return true
}

func shouldLogOpenAIForwardFailureAsWarn(c *gin.Context, wroteFallback bool) bool {
	if wroteFallback {
		return false
	}
	if c == nil || c.Writer == nil {
		return false
	}
	return c.Writer.Written()
}

func openAIForwardErrorAlreadyCommunicated(c *gin.Context, writerSizeBeforeForward int, err error) bool {
	if err == nil || c == nil || c.Writer == nil {
		return false
	}
	if c.Writer.Size() == writerSizeBeforeForward {
		return false
	}

	msg := strings.TrimSpace(err.Error())
	for _, prefix := range []string{
		"upstream response failed:",
		"upstream error:",
		"non-streaming openai protocol error:",
	} {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}

// errorResponse returns OpenAI API format error response
func (h *OpenAIGatewayHandler) errorResponse(c *gin.Context, status int, errType, message string) {
	if imageTrace := service.OpenAIImageTraceFromGin(c); imageTrace != nil {
		imageTrace.Log(c, "downstream_response_built", status, "")
	}
	c.JSON(status, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
	if imageTrace := service.OpenAIImageTraceFromGin(c); imageTrace != nil {
		imageTrace.Log(c, "downstream_write_done", status, "")
	}
}

func setOpenAIClientTransportHTTP(c *gin.Context) {
	service.SetOpenAIClientTransport(c, service.OpenAIClientTransportHTTP)
}

func setOpenAIClientTransportWS(c *gin.Context) {
	service.SetOpenAIClientTransport(c, service.OpenAIClientTransportWS)
}

func ensureOpenAIPoolModeSessionHash(sessionHash string, account *service.Account) string {
	if sessionHash != "" || account == nil || !account.IsPoolMode() {
		return sessionHash
	}
	// 为当前请求生成一次性粘性会话键，确保同账号重试不会重新负载均衡到其他账号。
	return "openai-pool-retry-" + uuid.NewString()
}

func openAIWSIngressFallbackSessionSeed(userID, apiKeyID int64, groupID *int64) string {
	gid := int64(0)
	if groupID != nil {
		gid = *groupID
	}
	return fmt.Sprintf("openai_ws_ingress:%d:%d:%d", gid, userID, apiKeyID)
}

func isOpenAIWSUpgradeRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Connection"))), "upgrade")
}

func closeOpenAIClientWS(conn *coderws.Conn, status coderws.StatusCode, reason string) {
	if conn == nil {
		return
	}
	reason = strings.TrimSpace(reason)
	if len(reason) > 120 {
		reason = reason[:120]
	}
	_ = conn.Close(status, reason)
	_ = conn.CloseNow()
}

func closeOpenAIWSFailoverExhausted(conn *coderws.Conn, failoverErr *service.UpstreamFailoverError) {
	if failoverErr == nil {
		closeOpenAIClientWS(conn, coderws.StatusInternalError, "upstream websocket proxy failed")
		return
	}
	switch failoverErr.StatusCode {
	case http.StatusTooManyRequests:
		closeOpenAIClientWS(conn, coderws.StatusTryAgainLater, "upstream rate limit exceeded, please retry later")
	case 529, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		closeOpenAIClientWS(conn, coderws.StatusTryAgainLater, "upstream service temporarily unavailable")
	case http.StatusUnauthorized, http.StatusForbidden:
		closeOpenAIClientWS(conn, coderws.StatusPolicyViolation, "upstream websocket authentication failed")
	default:
		closeOpenAIClientWS(conn, coderws.StatusInternalError, "upstream websocket proxy failed")
	}
}

func summarizeWSCloseErrorForLog(err error) (string, string) {
	if err == nil {
		return "-", "-"
	}
	statusCode := coderws.CloseStatus(err)
	if statusCode == -1 {
		return "-", "-"
	}
	closeStatus := fmt.Sprintf("%d(%s)", int(statusCode), statusCode.String())
	closeReason := "-"
	var closeErr coderws.CloseError
	if errors.As(err, &closeErr) {
		reason := strings.TrimSpace(closeErr.Reason)
		if reason != "" {
			closeReason = reason
		}
	}
	return closeStatus, closeReason
}
