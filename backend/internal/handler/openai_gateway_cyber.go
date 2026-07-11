package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const cyberPolicyRecordedKey = "ops_cyber_recorded"

type cyberPolicyOpsErrorMeta struct {
	RequestID, ClientRequestID, Platform, Model, RequestPath string
	InboundEndpoint, UserAgent, APIKeyPrefix, ClientIP       string
	Stream                                                   bool
	UserID, APIKeyID, AccountID                              int64
	GroupID                                                  *int64
	CreatedAt                                                time.Time
	SessionBlockKey                                          string
}

func buildCyberPolicyOpsErrorEntry(meta cyberPolicyOpsErrorMeta, mark *service.CyberPolicyMark) *service.OpsInsertErrorLogInput {
	rt := int16(service.RequestTypeCyberBlocked)
	entry := &service.OpsInsertErrorLogInput{
		RequestID: meta.RequestID, ClientRequestID: meta.ClientRequestID,
		Platform: meta.Platform, Model: meta.Model, RequestPath: meta.RequestPath,
		Stream: meta.Stream, InboundEndpoint: meta.InboundEndpoint, RequestType: &rt,
		UserAgent: meta.UserAgent, APIKeyPrefix: meta.APIKeyPrefix,
		ErrorPhase: "request", ErrorType: "cyber_policy", Severity: "P3",
		StatusCode: mark.UpstreamStatus, IsBusinessLimited: true,
		ErrorMessage: "cyber_policy: " + mark.Message, ErrorBody: mark.Body,
		ErrorSource: "upstream_http", ErrorOwner: "provider", CreatedAt: meta.CreatedAt,
		GroupID: meta.GroupID,
	}
	if meta.UserID > 0 {
		entry.UserID = &meta.UserID
	}
	if meta.APIKeyID > 0 {
		entry.APIKeyID = &meta.APIKeyID
	}
	if meta.AccountID > 0 {
		entry.AccountID = &meta.AccountID
	}
	if meta.ClientIP != "" {
		entry.ClientIP = &meta.ClientIP
	}
	return entry
}

func buildCyberSessionBlockedOpsEntry(meta cyberPolicyOpsErrorMeta) *service.OpsInsertErrorLogInput {
	rt := int16(service.RequestTypeCyberBlocked)
	entry := &service.OpsInsertErrorLogInput{
		RequestID: meta.RequestID, ClientRequestID: meta.ClientRequestID,
		Platform: meta.Platform, Model: meta.Model, RequestPath: meta.RequestPath,
		Stream: meta.Stream, InboundEndpoint: meta.InboundEndpoint, RequestType: &rt,
		UserAgent: meta.UserAgent, APIKeyPrefix: meta.APIKeyPrefix,
		ErrorPhase: "request", ErrorType: "cyber_policy_session_blocked", Severity: "P3",
		StatusCode: http.StatusForbidden, IsBusinessLimited: true,
		ErrorMessage: "cyber_policy_session_blocked: request rejected locally by session block",
		ErrorSource:  "gateway_local", ErrorOwner: "platform", CreatedAt: meta.CreatedAt,
		GroupID: meta.GroupID,
	}
	if meta.SessionBlockKey != "" {
		entry.ErrorBody = "session_block_key=" + meta.SessionBlockKey
	}
	if meta.UserID > 0 {
		entry.UserID = &meta.UserID
	}
	if meta.APIKeyID > 0 {
		entry.APIKeyID = &meta.APIKeyID
	}
	if meta.ClientIP != "" {
		entry.ClientIP = &meta.ClientIP
	}
	return entry
}

const cyberSessionBlockedClientMsg = "该会话已被网络安全策略屏蔽，请开启新会话 / This session is blocked by cyber-security policy, please start a new session"

type cyberSessionBlockFormat int

const (
	cyberBlockFormatResponses cyberSessionBlockFormat = iota
	cyberBlockFormatChat
	cyberBlockFormatAnthropic
)

func (h *OpenAIGatewayHandler) rejectIfCyberSessionBlocked(c *gin.Context, apiKey *service.APIKey, body []byte, model string, format cyberSessionBlockFormat) bool {
	if h == nil || h.gatewayService == nil || apiKey == nil || c == nil || c.Request == nil {
		return false
	}
	if enabled, _ := h.gatewayService.CyberSessionBlockRuntime(c.Request.Context()); !enabled {
		return false
	}
	key := service.CyberSessionBlockKey(apiKey.ID, c, body)
	if key == "" || !h.gatewayService.IsCyberSessionBlocked(c.Request.Context(), key) {
		return false
	}
	if format == cyberBlockFormatAnthropic {
		c.JSON(http.StatusForbidden, gin.H{"type": "error", "error": gin.H{"type": "permission_error", "message": cyberSessionBlockedClientMsg}})
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"type": "permission_error", "code": "session_blocked_by_cyber_policy", "message": cyberSessionBlockedClientMsg}})
	}
	h.enqueueCyberSessionBlockedOpsEntry(c, apiKey, model, key)
	return true
}

func (h *OpenAIGatewayHandler) enqueueCyberSessionBlockedOpsEntry(c *gin.Context, apiKey *service.APIKey, model, key string) {
	if h.opsService == nil {
		return
	}
	meta := h.cyberOpsMeta(c, apiKey, nil, model)
	meta.SessionBlockKey = key
	enqueueOpsErrorLog(h.opsService, buildCyberSessionBlockedOpsEntry(meta))
}

func (h *OpenAIGatewayHandler) cyberOpsMeta(c *gin.Context, apiKey *service.APIKey, account *service.Account, model string) cyberPolicyOpsErrorMeta {
	meta := cyberPolicyOpsErrorMeta{Model: model, InboundEndpoint: GetInboundEndpoint(c), CreatedAt: time.Now()}
	if c == nil {
		return meta
	}
	meta.RequestID = c.Writer.Header().Get("X-Request-Id")
	if c.Request != nil {
		if c.Request.URL != nil {
			meta.RequestPath = c.Request.URL.Path
		}
		meta.ClientRequestID, _ = c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		meta.UserAgent = c.GetHeader("User-Agent")
		meta.ClientIP = strings.TrimSpace(ip.GetClientIP(c))
	}
	if v, ok := c.Get(opsStreamKey); ok {
		meta.Stream, _ = v.(bool)
	}
	meta.Platform = resolveOpsPlatform(apiKey, guessPlatformFromPath(meta.RequestPath))
	if apiKey != nil {
		meta.APIKeyID, meta.GroupID, meta.APIKeyPrefix = apiKey.ID, apiKey.GroupID, keyPrefix(apiKey.Key, 8)
		if apiKey.User != nil {
			meta.UserID = apiKey.User.ID
		}
	}
	if account != nil {
		meta.AccountID = account.ID
	}
	return meta
}

func (h *OpenAIGatewayHandler) recordCyberPolicyIfMarked(c *gin.Context, apiKey *service.APIKey, account *service.Account, subscription *service.UserSubscription, model string, forwardErrored bool, cyberBlockKey string, channelFields service.ChannelUsageFields, requestPayloadHash string) {
	mark := service.GetOpsCyberPolicy(c)
	if mark == nil || c.GetBool(cyberPolicyRecordedKey) {
		return
	}
	c.Set(cyberPolicyRecordedKey, true)
	meta := h.cyberOpsMeta(c, apiKey, account, model)
	var userID, apiKeyID int64
	var userEmail, apiKeyName, groupName string
	var groupID *int64
	if apiKey != nil {
		apiKeyID, apiKeyName, groupID = apiKey.ID, apiKey.Name, apiKey.GroupID
		if apiKey.User != nil {
			userID, userEmail = apiKey.User.ID, apiKey.User.Email
		}
		if apiKey.Group != nil {
			groupName = apiKey.Group.Name
		}
	}
	cmSvc, gwSvc, opsSvc, apiKeySvc := h.contentModerationService, h.gatewayService, h.opsService, h.apiKeyService
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cmSvc != nil {
			cmSvc.RecordCyberPolicyEvent(ctx, service.CyberPolicyRecordInput{RequestID: meta.RequestID, UserID: userID, UserEmail: userEmail, APIKeyID: apiKeyID, APIKeyName: apiKeyName, GroupID: groupID, GroupName: groupName, Endpoint: meta.InboundEndpoint, Model: model, UpstreamMessage: mark.Message, UpstreamBody: mark.Body, UpstreamStatus: mark.UpstreamStatus, UpstreamInTok: mark.UpstreamInTok, UpstreamOutTok: mark.UpstreamOutTok})
		}
		if forwardErrored && gwSvc != nil {
			gwSvc.RecordCyberPolicyUsageLog(ctx, service.CyberPolicyUsageInput{APIKey: apiKey, Account: account, Subscription: subscription, RequestID: meta.RequestID, Model: model, Stream: meta.Stream, InputTokens: mark.UpstreamInTok, OutputTokens: mark.UpstreamOutTok, InboundEndpoint: meta.InboundEndpoint, UserAgent: meta.UserAgent, IPAddress: meta.ClientIP, RequestPayloadHash: requestPayloadHash, APIKeyService: apiKeySvc, ChannelUsageFields: channelFields})
		}
		if gwSvc != nil && cyberBlockKey != "" {
			gwSvc.MarkCyberSessionBlocked(ctx, cyberBlockKey)
		}
		if opsSvc != nil {
			enqueueOpsErrorLog(opsSvc, buildCyberPolicyOpsErrorEntry(meta, mark))
		}
	}()
}

func clearCyberPolicyTurnState(c *gin.Context) {
	if c == nil {
		return
	}
	service.ClearOpsCyberPolicy(c)
	c.Set(cyberPolicyRecordedKey, false)
}
