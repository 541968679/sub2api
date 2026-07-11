package handler

import (
	"math"
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

// ClaudeGPTBridgeRouteAction tells the route layer how to dispatch an
// Antigravity /v1/messages request after the strict bridge route diagnosis.
type ClaudeGPTBridgeRouteAction int

const (
	// ClaudeGPTBridgeRouteActionNative 表示没有 bridge 配置意图，走原生
	// Antigravity 处理链。这是唯一允许 native 的动作。
	ClaudeGPTBridgeRouteActionNative ClaudeGPTBridgeRouteAction = iota
	// ClaudeGPTBridgeRouteActionBridge 表示存在可调度 bridge 候选，进入
	// MessagesClaudeGPTBridge。
	ClaudeGPTBridgeRouteActionBridge
	// ClaudeGPTBridgeRouteActionHandled 表示响应已经写出（429/503/400），
	// 路由层不得再调用任何 handler。
	ClaudeGPTBridgeRouteActionHandled
)

const (
	claudeGPTBridgeRateLimitedMessage = "Upstream rate limit exceeded, please retry later"
	claudeGPTBridgeUnavailableMessage = "Upstream accounts are temporarily unavailable, please retry later"
	claudeGPTBridgeProbeErrorMessage  = "Service temporarily unavailable, please retry later"
)

// route_decision 事件的 terminal_outcome 取值：表示该次路由诊断在路由层的
// 终局动作（分发目标或已写出的错误响应），不含账号或配额信息。
const (
	claudeGPTBridgeOutcomeDispatchNative    = "dispatch_native"
	claudeGPTBridgeOutcomeDispatchBridge    = "dispatch_bridge"
	claudeGPTBridgeOutcomeGroupModelBlocked = "group_model_blocked_403"
	claudeGPTBridgeOutcomeRateLimited       = "rate_limited_429"
	claudeGPTBridgeOutcomeUnavailable       = "unavailable_503"
	claudeGPTBridgeOutcomeProbeError        = "probe_error_503"
	claudeGPTBridgeOutcomeCountBridgeReady  = "count_bridge_ready"
	claudeGPTBridgeOutcomeCountLocalEstim   = "count_local_estimate"
)

// ClaudeGPTBridgeRoute resolves strict routing for an Antigravity group
// /v1/messages request. It replaces the old boolean ShouldUseClaudeGPTBridge
// preflight: configured-but-temporarily-unavailable bridges now return 429/503
// bridge semantics instead of silently falling back to the native Antigravity
// pool. The diagnosis never sends upstream requests and never acquires
// scheduler slots.
func (h *OpenAIGatewayHandler) ClaudeGPTBridgeRoute(c *gin.Context) ClaudeGPTBridgeRouteAction {
	if h == nil || h.gatewayService == nil || c == nil {
		return ClaudeGPTBridgeRouteActionNative
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.GroupID == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformAntigravity {
		return ClaudeGPTBridgeRouteActionNative
	}

	routeStart := time.Now()
	reqLog := requestLogger(c, "handler.openai_gateway.claude_gpt_bridge_route",
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		resetRequestBody(c.Request, body)
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.anthropicErrorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return ClaudeGPTBridgeRouteActionHandled
		}
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return ClaudeGPTBridgeRouteActionHandled
	}
	resetRequestBody(c.Request, body)
	// 协议级错误直接返回规范 400，不允许伪装成“没有 bridge”再落 native。
	if len(body) == 0 {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return ClaudeGPTBridgeRouteActionHandled
	}
	if !gjson.ValidBytes(body) {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return ClaudeGPTBridgeRouteActionHandled
	}
	modelResult := gjson.GetBytes(body, "model")
	reqModel := strings.TrimSpace(modelResult.String())
	if !modelResult.Exists() || modelResult.Type != gjson.String || reqModel == "" {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return ClaudeGPTBridgeRouteActionHandled
	}

	decision := h.gatewayService.ResolveClaudeGPTBridgeRoute(c.Request.Context(), apiKey.GroupID, reqModel)

	action := ClaudeGPTBridgeRouteActionHandled
	terminalOutcome := ""
	var respond func()
	switch decision.State {
	case service.ClaudeGPTBridgeRouteNotConfigured:
		action = ClaudeGPTBridgeRouteActionNative
		terminalOutcome = claudeGPTBridgeOutcomeDispatchNative
	case service.ClaudeGPTBridgeRouteReady:
		action = ClaudeGPTBridgeRouteActionBridge
		terminalOutcome = claudeGPTBridgeOutcomeDispatchBridge
	default:
		// 分组禁用的模型必须稳定返回 403，不随 bridge 容量状态在 429/503 间摆动。
		// ready 路径的同一检查由 MessagesClaudeGPTBridge 内部完成。
		if !isGroupModelAllowed(apiKey.Group, reqModel) {
			terminalOutcome = claudeGPTBridgeOutcomeGroupModelBlocked
			respond = func() {
				h.anthropicErrorResponse(c, http.StatusForbidden, "permission_error", groupModelAccessDeniedMessage)
			}
			break
		}
		switch decision.State {
		case service.ClaudeGPTBridgeRouteRateLimited:
			terminalOutcome = claudeGPTBridgeOutcomeRateLimited
			respond = func() {
				setClaudeGPTBridgeRetryAfterHeader(c, decision.RetryAt)
				h.anthropicErrorResponse(c, http.StatusTooManyRequests, "rate_limit_error", claudeGPTBridgeRateLimitedMessage)
			}
		case service.ClaudeGPTBridgeRouteUnavailable:
			terminalOutcome = claudeGPTBridgeOutcomeUnavailable
			respond = func() {
				h.anthropicErrorResponse(c, http.StatusServiceUnavailable, "overloaded_error", claudeGPTBridgeUnavailableMessage)
			}
		default: // probe_error 以及任何未知状态都停留在 bridge 错误语义。
			terminalOutcome = claudeGPTBridgeOutcomeProbeError
			respond = func() {
				h.anthropicErrorResponse(c, http.StatusServiceUnavailable, "api_error", claudeGPTBridgeProbeErrorMessage)
			}
		}
	}

	logClaudeGPTBridgeRouteDecision(reqLog, decision, reqModel, "preflight", routeStart, 0, terminalOutcome)
	if respond != nil {
		respond()
	}
	return action
}

// respondClaudeGPTBridgeSelectionRace covers the race where the preflight saw
// a ready bridge but the real scheduler selection immediately failed (e.g. the
// account entered a 429 cooldown in between). It re-diagnoses once: pure rate
// limit becomes 429, every other outcome — including a mapping deleted
// mid-request — stays a bridge-side 503. Requests never switch to native
// after bridge intent is established.
func (h *OpenAIGatewayHandler) respondClaudeGPTBridgeSelectionRace(c *gin.Context, groupID *int64, reqModel string, streamStarted bool) {
	// 计时起点必须在二次诊断之前：selection_race 恰恰是限流竞态下把
	// ListByGroup 查询成本翻倍的路径，latency_ms 需要反映真实诊断耗时。
	raceStart := time.Now()
	decision := service.ClaudeGPTBridgeRouteDecision{State: service.ClaudeGPTBridgeRouteProbeError, Reason: "missing_dependencies"}
	if h != nil && h.gatewayService != nil {
		decision = h.gatewayService.ResolveClaudeGPTBridgeRoute(c.Request.Context(), groupID, reqModel)
	}
	reqLog := requestLogger(c, "handler.openai_gateway.claude_gpt_bridge_route", zap.Any("group_id", groupID))

	if decision.State == service.ClaudeGPTBridgeRouteRateLimited {
		logClaudeGPTBridgeRouteDecision(reqLog, decision, reqModel, "selection_race", raceStart, 1, claudeGPTBridgeOutcomeRateLimited)
		setClaudeGPTBridgeRetryAfterHeader(c, decision.RetryAt)
		h.anthropicStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", claudeGPTBridgeRateLimitedMessage, streamStarted)
		return
	}
	logClaudeGPTBridgeRouteDecision(reqLog, decision, reqModel, "selection_race", raceStart, 1, claudeGPTBridgeOutcomeUnavailable)
	h.anthropicStreamingAwareError(c, http.StatusServiceUnavailable, "overloaded_error", claudeGPTBridgeUnavailableMessage, streamStarted)
}

// setClaudeGPTBridgeRetryAfterHeader writes Retry-After from the earliest
// future recovery time: seconds are rounded up with a minimum of 1 and capped
// at 24h (RateLimitResetAt ultimately comes from upstream-controlled values),
// and past times never produce a header with zero or negative values. The
// response must not expose account identities or quota details.
func setClaudeGPTBridgeRetryAfterHeader(c *gin.Context, retryAt *time.Time) {
	if c == nil || retryAt == nil {
		return
	}
	seconds := int(math.Ceil(time.Until(*retryAt).Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	if seconds > claudeGPTBridgeRetryAfterMaxSeconds {
		seconds = claudeGPTBridgeRetryAfterMaxSeconds
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
}

const claudeGPTBridgeRetryAfterMaxSeconds = 86400

// validatedUpstreamRetryAfterSeconds accepts only a plain positive integer
// Retry-After within 24h; anything else is dropped instead of being forwarded
// blindly to the client.
func validatedUpstreamRetryAfterSeconds(headers map[string][]string) (int, bool) {
	if headers == nil {
		return 0, false
	}
	raw := ""
	for key, values := range headers {
		if strings.EqualFold(key, "Retry-After") && len(values) > 0 {
			raw = strings.TrimSpace(values[0])
			break
		}
	}
	if raw == "" {
		return 0, false
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < 1 || secs > claudeGPTBridgeRetryAfterMaxSeconds {
		return 0, false
	}
	return secs, true
}

func logClaudeGPTBridgeRouteDecision(reqLog *zap.Logger, decision service.ClaudeGPTBridgeRouteDecision, reqModel, source string, start time.Time, attempt int, terminalOutcome string) {
	if reqLog == nil {
		return
	}
	fields := []zap.Field{
		zap.String("requested_model", reqModel),
		zap.String("state", string(decision.State)),
		zap.Int("candidate_count", decision.CandidateCount),
		zap.Int("schedulable_count", decision.SchedulableCount),
		zap.Int("rate_limited_count", decision.RateLimitedCount),
		zap.String("reason", decision.Reason),
		zap.String("decision_source", source),
		zap.Int("attempt", attempt),
		zap.String("terminal_outcome", terminalOutcome),
		zap.Bool("native_fallback", decision.State == service.ClaudeGPTBridgeRouteNotConfigured),
		zap.Int64("latency_ms", time.Since(start).Milliseconds()),
	}
	if decision.RetryAt != nil {
		fields = append(fields, zap.Time("retry_at", *decision.RetryAt))
	}
	reqLog.Info("openai_claude_gpt_bridge.route_decision", fields...)
}
