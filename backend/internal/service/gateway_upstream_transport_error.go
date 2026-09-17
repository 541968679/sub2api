package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const gatewayTransportErrorTempUnschedDuration = 10 * time.Minute

var gatewayTransportFailoverBody = []byte(`{"type":"error","error":{"type":"upstream_error","message":"Upstream request failed"}}`)

func classifyUpstreamTransportError(err error) openAITransportErrorClass {
	return classifyOpenAITransportError(err)
}

func opsUpstreamProxyAttribution(account *Account) (*int64, string) {
	return opsUpstreamProxyID(account), opsUpstreamProxyName(account)
}

func (s *GatewayService) handleUpstreamTransportError(ctx context.Context, c *gin.Context, account *Account, err error, event OpsUpstreamErrorEvent) error {
	safeErr := sanitizeUpstreamErrorMessage(err.Error())
	setOpsUpstreamError(c, 0, safeErr, "")
	if account != nil {
		event.ProxyID, event.ProxyName = opsUpstreamProxyAttribution(account)
		event.Platform = account.Platform
		event.AccountID = account.ID
		event.AccountName = account.Name
	}
	event.UpstreamStatusCode = 0
	event.Kind = "request_error"
	event.Message = safeErr
	appendOpsUpstreamError(c, event)

	if errors.Is(err, context.Canceled) || (errors.Is(err, context.DeadlineExceeded) && errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		return err
	}

	if classifyUpstreamTransportError(err).Persistent {
		s.tempUnscheduleTransportError(ctx, account, safeErr)
	}

	return &UpstreamFailoverError{
		StatusCode:   http.StatusBadGateway,
		ResponseBody: gatewayTransportFailoverBody,
	}
}

func (s *GatewayService) tempUnscheduleTransportError(ctx context.Context, account *Account, safeErr string) {
	if s == nil || account == nil || s.accountRepo == nil {
		return
	}
	until := time.Now().Add(gatewayTransportErrorTempUnschedDuration)
	reason := "upstream transport error (proxy/network): " + safeErr

	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAITransportErrorStateUpdateTimeout)
	defer cancel()
	if err := s.accountRepo.SetTempUnschedulable(bgCtx, account.ID, until, reason); err != nil {
		logger.L().With(zap.String("component", "service.gateway")).Warn(
			"gateway.account_temp_unschedule_transport_failed",
			zap.Int64("account_id", account.ID),
			zap.Error(err),
		)
		return
	}
	logger.L().With(zap.String("component", "service.gateway")).Warn(
		"gateway.account_temp_unscheduled_transport",
		zap.Int64("account_id", account.ID),
		zap.String("account_name", account.Name),
		zap.String("platform", account.Platform),
		zap.Time("until", until),
		zap.String("reason", reason),
	)
}
