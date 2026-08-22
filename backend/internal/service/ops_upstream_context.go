package service

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Gin context keys used by Ops error logger for capturing upstream error details.
// These keys are set by gateway services and consumed by handler/ops_error_logger.go.
const (
	// OpsUpstreamModelKey must stay aligned with handler/ops_error_logger.go
	// (opsUpstreamModelKey = "ops_upstream_model").
	OpsUpstreamModelKey = "ops_upstream_model"

	OpsUpstreamStatusCodeKey   = "ops_upstream_status_code"
	OpsUpstreamErrorMessageKey = "ops_upstream_error_message"
	OpsUpstreamErrorDetailKey  = "ops_upstream_error_detail"
	OpsProviderErrorCodeKey    = "ops_provider_error_code"
	OpsUpstreamErrorsKey       = "ops_upstream_errors"

	// opsUpstreamDetailMaxBytes is the capture cap for admin-visible upstream
	// JSON. Matches the historical 2KB LogUpstreamErrorBody default.
	opsUpstreamDetailMaxBytes = 2048

	// Best-effort capture of the current upstream request body so ops can
	// retry the specific upstream attempt (not just the client request).
	// This value is sanitized+trimmed before being persisted.
	OpsUpstreamRequestBodyKey = "ops_upstream_request_body"

	// Optional stage latencies (milliseconds) for troubleshooting and alerting.
	OpsAuthLatencyMsKey      = "ops_auth_latency_ms"
	OpsRoutingLatencyMsKey   = "ops_routing_latency_ms"
	OpsUpstreamLatencyMsKey  = "ops_upstream_latency_ms"
	OpsResponseLatencyMsKey  = "ops_response_latency_ms"
	OpsTimeToFirstTokenMsKey = "ops_time_to_first_token_ms"
	// OpenAI WS 关键观测字段
	OpsOpenAIWSQueueWaitMsKey = "ops_openai_ws_queue_wait_ms"
	OpsOpenAIWSConnPickMsKey  = "ops_openai_ws_conn_pick_ms"
	OpsOpenAIWSConnReusedKey  = "ops_openai_ws_conn_reused"
	OpsOpenAIWSConnIDKey      = "ops_openai_ws_conn_id"

	// OpsSkipPassthroughKey 由 applyErrorPassthroughRule 在命中 skip_monitoring=true 的规则时设置。
	// ops_error_logger 中间件检查此 key，为 true 时跳过错误记录。
	OpsSkipPassthroughKey = "ops_skip_passthrough"
	OpsStreamErrorKey     = "ops_stream_error"
	ResponseCommittedKey  = "response_committed"

	// Client-side configuration denials should remain visible in ops_error_logs,
	// but should be excluded from SLA/error-rate calculations.
	OpsClientBusinessLimitedKey                          = "ops_client_business_limited"
	OpsClientBusinessLimitedReasonKey                    = "ops_client_business_limited_reason"
	OpsClientBusinessLimitedReasonIPRestriction          = "api_key_ip_restriction"
	OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable = "api_key_group_unavailable"
	OpsClientBusinessLimitedReasonAPIKeyGroupUnassigned  = "api_key_group_unassigned"
	OpsClientBusinessLimitedReasonLocalFeatureGate       = "local_feature_gate"
	OpsClientBusinessLimitedReasonLocalPolicyDenied      = "local_policy_denied"
)

// genericOpsUpstreamMessages is the Ops-merge set only. It must stay aligned
// with frontend GENERIC_UPSTREAM_MESSAGES. Do not use it as a client mapping table.
var genericOpsUpstreamMessages = map[string]struct{}{
	"upstream request failed":                  {},
	"upstream request failed after retries":    {},
	"upstream gateway error":                   {},
	"upstream service temporarily unavailable": {},
}

func MarkResponseCommitted(c *gin.Context) {
	if c != nil {
		c.Set(ResponseCommittedKey, true)
	}
}

func IsResponseCommitted(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, ok := c.Get(ResponseCommittedKey)
	committed, _ := value.(bool)
	return ok && committed
}

func setOpsUpstreamRequestBody(c *gin.Context, body []byte) {
	if c == nil || len(body) == 0 {
		return
	}
	// 热路径避免 string(body) 额外分配，按需在落库前再转换。
	c.Set(OpsUpstreamRequestBodyKey, body)
}

func SetOpsLatencyMs(c *gin.Context, key string, value int64) {
	if c == nil || strings.TrimSpace(key) == "" || value < 0 {
		return
	}
	c.Set(key, value)
}

// SetOpsUpstreamModel records the model actually sent upstream (after mapping)
// so ops_error_logs.upstream_model is populated for bridge/mapping paths.
func SetOpsUpstreamModel(c *gin.Context, upstreamModel string) {
	if c == nil {
		return
	}
	if upstreamModel = strings.TrimSpace(upstreamModel); upstreamModel == "" {
		return
	}
	c.Set(OpsUpstreamModelKey, upstreamModel)
}

func MarkOpsClientBusinessLimited(c *gin.Context, reason string) {
	if c == nil {
		return
	}
	c.Set(OpsClientBusinessLimitedKey, true)
	if reason = strings.TrimSpace(reason); reason != "" {
		c.Set(OpsClientBusinessLimitedReasonKey, reason)
	}
}

func HasOpsClientBusinessLimited(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get(OpsClientBusinessLimitedKey)
	if !ok {
		return false
	}
	marked, _ := v.(bool)
	return marked
}

type OpsStreamError struct {
	ErrType        string
	Message        string
	IntendedStatus int
}

// MarkOpsStreamError preserves the first in-band SSE failure so a later
// generic fallback cannot overwrite the root cause.
func MarkOpsStreamError(c *gin.Context, errType, message string, intendedStatus int) {
	if c == nil {
		return
	}
	if _, exists := c.Get(OpsStreamErrorKey); exists {
		return
	}
	c.Set(OpsStreamErrorKey, OpsStreamError{
		ErrType:        strings.TrimSpace(errType),
		Message:        strings.TrimSpace(message),
		IntendedStatus: intendedStatus,
	})
}

func GetOpsStreamError(c *gin.Context) (OpsStreamError, bool) {
	if c == nil {
		return OpsStreamError{}, false
	}
	value, ok := c.Get(OpsStreamErrorKey)
	if !ok {
		return OpsStreamError{}, false
	}
	streamErr, ok := value.(OpsStreamError)
	return streamErr, ok
}

// SetOpsUpstreamError is the exported wrapper for setOpsUpstreamError, used by
// handler-layer code (e.g. failover-exhausted paths) that needs to record the
// original upstream status code before mapping it to a client-facing code.
func SetOpsUpstreamError(c *gin.Context, upstreamStatusCode int, upstreamMessage, upstreamDetail string) {
	setOpsUpstreamError(c, upstreamStatusCode, upstreamMessage, upstreamDetail)
}

func isGenericOpsUpstreamMessage(msg string) bool {
	_, ok := genericOpsUpstreamMessages[strings.ToLower(strings.TrimSpace(msg))]
	return ok
}

func opsContextString(c *gin.Context, key string) string {
	if c == nil {
		return ""
	}
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func shouldKeepExistingOpsMessage(existing, incoming string) bool {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if incoming == "" {
		return existing != ""
	}
	if existing == "" {
		return false
	}
	if isGenericOpsUpstreamMessage(incoming) && !isGenericOpsUpstreamMessage(existing) {
		return true
	}
	return false
}

func isGenericOpsUpstreamDetail(detail string) bool {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return false
	}
	if isGenericOpsUpstreamMessage(detail) {
		return true
	}
	var parsed struct {
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(detail), &parsed); err != nil || parsed.Error == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(parsed.Error.Type), "upstream_error") {
		return false
	}
	return isGenericOpsUpstreamMessage(parsed.Error.Message)
}

func sanitizeOpsUpstreamDetail(rawBody []byte) string {
	if len(rawBody) == 0 {
		return ""
	}
	truncated := truncateString(string(rawBody), opsUpstreamDetailMaxBytes)
	sanitized, _ := sanitizeErrorBodyForStorage(truncated, opsUpstreamDetailMaxBytes)
	return strings.TrimSpace(sanitized)
}

func setOpsProviderErrorCode(c *gin.Context, code string) {
	if c == nil {
		return
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return
	}
	c.Set(OpsProviderErrorCodeKey, truncateString(code, 64))
}

func setOpsUpstreamError(c *gin.Context, upstreamStatusCode int, upstreamMessage, upstreamDetail string) {
	if c == nil {
		return
	}
	if upstreamStatusCode > 0 {
		c.Set(OpsUpstreamStatusCodeKey, upstreamStatusCode)
	}
	if msg := strings.TrimSpace(upstreamMessage); msg != "" {
		existing := opsContextString(c, OpsUpstreamErrorMessageKey)
		if !shouldKeepExistingOpsMessage(existing, msg) {
			c.Set(OpsUpstreamErrorMessageKey, msg)
		}
	}
	if detail := strings.TrimSpace(upstreamDetail); detail != "" && !isGenericOpsUpstreamDetail(detail) {
		c.Set(OpsUpstreamErrorDetailKey, detail)
		if code := strings.TrimSpace(extractUpstreamErrorCode([]byte(detail))); code != "" {
			setOpsProviderErrorCode(c, code)
		}
	}
}

// recordOpsUpstreamAttempt writes one upstream hop for Ops only.
// rawBody must be the original upstream body, not a mapped client wrapper.
// Message is extracted from rawBody (empty stays empty; no generic fill-in).
// Detail is always sanitized+truncated and does not depend on LogUpstreamErrorBody.
func recordOpsUpstreamAttempt(c *gin.Context, ev OpsUpstreamErrorEvent, rawBody []byte) {
	extracted := strings.TrimSpace(extractUpstreamErrorMessage(rawBody))
	extracted = sanitizeUpstreamErrorMessage(extracted)
	if extracted != "" && !isGenericOpsUpstreamMessage(extracted) {
		ev.Message = extracted
	} else if isGenericOpsUpstreamMessage(ev.Message) {
		ev.Message = ""
	}

	if detail := sanitizeOpsUpstreamDetail(rawBody); detail != "" && !isGenericOpsUpstreamDetail(detail) {
		ev.Detail = detail
		if ev.UpstreamResponseBody == "" || isGenericOpsUpstreamDetail(ev.UpstreamResponseBody) {
			ev.UpstreamResponseBody = detail
		}
	} else if isGenericOpsUpstreamDetail(ev.Detail) {
		ev.Detail = ""
	}
	if isGenericOpsUpstreamDetail(ev.UpstreamResponseBody) {
		ev.UpstreamResponseBody = ""
	}

	if code := strings.TrimSpace(extractUpstreamErrorCode(rawBody)); code != "" {
		setOpsProviderErrorCode(c, code)
	}

	setOpsUpstreamError(c, ev.UpstreamStatusCode, ev.Message, ev.Detail)
	appendOpsUpstreamError(c, ev)
}

// RecordOpsUpstreamAttempt is the exported wrapper for handler-layer capture.
func RecordOpsUpstreamAttempt(c *gin.Context, ev OpsUpstreamErrorEvent, rawBody []byte) {
	recordOpsUpstreamAttempt(c, ev, rawBody)
}

// FailoverOpsRawBody returns the original upstream body for Ops capture.
func FailoverOpsRawBody(err *UpstreamFailoverError) []byte {
	if err == nil {
		return nil
	}
	if len(err.RawUpstreamBody) > 0 {
		return err.RawUpstreamBody
	}
	return err.ResponseBody
}

// OpsUpstreamErrorEvent describes one upstream error attempt during a single gateway request.
// It is stored in ops_error_logs.upstream_errors as a JSON array.
type OpsUpstreamErrorEvent struct {
	AtUnixMs int64 `json:"at_unix_ms,omitempty"`

	// Passthrough 表示本次请求是否命中“原样透传（仅替换认证）”分支。
	// 该字段用于排障与灰度评估；存入 JSON，不涉及 DB schema 变更。
	Passthrough bool `json:"passthrough,omitempty"`

	// Context
	Platform    string `json:"platform,omitempty"`
	AccountID   int64  `json:"account_id,omitempty"`
	AccountName string `json:"account_name,omitempty"`
	// UpstreamEndpoint is the normalized path selected for this attempt.
	UpstreamEndpoint string `json:"upstream_endpoint,omitempty"`

	// Outcome
	UpstreamStatusCode int    `json:"upstream_status_code,omitempty"`
	UpstreamRequestID  string `json:"upstream_request_id,omitempty"`

	// UpstreamURL is the actual upstream URL that was called (host + path, query/fragment stripped).
	// Helps debug 404/routing errors by showing which endpoint was targeted.
	UpstreamURL string `json:"upstream_url,omitempty"`

	// Best-effort upstream request capture (sanitized+trimmed).
	// Required for retrying a specific upstream attempt.
	UpstreamRequestBody string `json:"upstream_request_body,omitempty"`

	// Best-effort upstream response capture (sanitized+trimmed).
	UpstreamResponseBody string `json:"upstream_response_body,omitempty"`

	// Kind: http_error | request_error | retry_exhausted | failover
	Kind string `json:"kind,omitempty"`

	Message string `json:"message,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

func appendOpsUpstreamError(c *gin.Context, ev OpsUpstreamErrorEvent) {
	if c == nil {
		return
	}
	if ev.AtUnixMs <= 0 {
		ev.AtUnixMs = time.Now().UnixMilli()
	}
	ev.Platform = strings.TrimSpace(ev.Platform)
	ev.UpstreamEndpoint = normalizeOpsUpstreamEndpoint(ev.UpstreamEndpoint)
	if ev.UpstreamEndpoint == "" && (ev.Platform == PlatformOpenAI || ev.Platform == PlatformGrok) {
		ev.UpstreamEndpoint = normalizeOpsUpstreamEndpoint(GetActualOpenAIUpstreamEndpoint(c))
	}
	ev.UpstreamRequestID = strings.TrimSpace(ev.UpstreamRequestID)
	ev.UpstreamRequestBody = strings.TrimSpace(ev.UpstreamRequestBody)
	ev.UpstreamResponseBody = strings.TrimSpace(ev.UpstreamResponseBody)
	ev.Kind = strings.TrimSpace(ev.Kind)
	ev.UpstreamURL = strings.TrimSpace(ev.UpstreamURL)
	ev.Message = strings.TrimSpace(ev.Message)
	ev.Detail = strings.TrimSpace(ev.Detail)
	if ev.Message != "" {
		ev.Message = sanitizeUpstreamErrorMessage(ev.Message)
	}
	if isGenericOpsUpstreamDetail(ev.Detail) {
		ev.Detail = ""
	}
	if isGenericOpsUpstreamDetail(ev.UpstreamResponseBody) {
		ev.UpstreamResponseBody = ""
	}

	// If the caller didn't explicitly pass upstream request body but the gateway
	// stored it on the context, attach it so ops can retry this specific attempt.
	if ev.UpstreamRequestBody == "" {
		if v, ok := c.Get(OpsUpstreamRequestBodyKey); ok {
			switch raw := v.(type) {
			case string:
				ev.UpstreamRequestBody = strings.TrimSpace(raw)
			case []byte:
				ev.UpstreamRequestBody = strings.TrimSpace(string(raw))
			}
		}
	}

	var existing []*OpsUpstreamErrorEvent
	if v, ok := c.Get(OpsUpstreamErrorsKey); ok {
		if arr, ok := v.([]*OpsUpstreamErrorEvent); ok {
			existing = arr
		}
	}

	evCopy := ev
	existing = append(existing, &evCopy)
	c.Set(OpsUpstreamErrorsKey, existing)

	checkSkipMonitoringForUpstreamEvent(c, &evCopy)
}

func normalizeOpsUpstreamEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || !strings.HasPrefix(endpoint, "/") {
		return ""
	}
	if idx := strings.IndexAny(endpoint, "?#"); idx >= 0 {
		endpoint = endpoint[:idx]
	}
	return truncateString(endpoint, 256)
}

// checkSkipMonitoringForUpstreamEvent checks whether the upstream error event
// matches a passthrough rule with skip_monitoring=true and, if so, sets the
// OpsSkipPassthroughKey on the context.  This ensures intermediate retry /
// failover errors (which never go through the final applyErrorPassthroughRule
// path) can still suppress ops_error_logs recording.
func checkSkipMonitoringForUpstreamEvent(c *gin.Context, ev *OpsUpstreamErrorEvent) {
	if ev.UpstreamStatusCode == 0 {
		return
	}

	svc := getBoundErrorPassthroughService(c)
	if svc == nil {
		return
	}

	// Use the best available body representation for keyword matching.
	// Even when body is empty, MatchRule can still match rules that only
	// specify ErrorCodes (no Keywords), so we always call it.
	body := ev.Detail
	if body == "" {
		body = ev.Message
	}

	rule := svc.MatchRule(ev.Platform, ev.UpstreamStatusCode, []byte(body))
	if rule != nil && rule.SkipMonitoring {
		c.Set(OpsSkipPassthroughKey, true)
	}
}

func marshalOpsUpstreamErrors(events []*OpsUpstreamErrorEvent) *string {
	if len(events) == 0 {
		return nil
	}
	// Ensure we always store a valid JSON value.
	raw, err := json.Marshal(events)
	if err != nil || len(raw) == 0 {
		return nil
	}
	s := string(raw)
	return &s
}

func ParseOpsUpstreamErrors(raw string) ([]*OpsUpstreamErrorEvent, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []*OpsUpstreamErrorEvent{}, nil
	}
	var out []*OpsUpstreamErrorEvent
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// safeUpstreamURL returns scheme + host + path from a URL, stripping query/fragment
// to avoid leaking sensitive query parameters (e.g. OAuth tokens).
func safeUpstreamURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if idx := strings.IndexByte(rawURL, '?'); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	if idx := strings.IndexByte(rawURL, '#'); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	return rawURL
}
