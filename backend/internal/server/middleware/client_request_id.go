package middleware

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/inboundhost"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// The client sees one local request id. X-Client-Request-Id is not written;
// the same id stays on the request context for logs.
const clientRequestIDHeader = "X-Request-Id"

// ClientRequestID ensures every request has a unique client_request_id in request.Context().
//
// This is used by the Ops monitoring module for end-to-end request correlation.
func ClientRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil {
			c.Next()
			return
		}

		if v, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(v) != "" {
			c.Header(clientRequestIDHeader, strings.TrimSpace(v))
			stampInboundHost(c)
			c.Next()
			return
		}

		id := uuid.New().String()
		c.Header(clientRequestIDHeader, id)
		ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, id)
		requestLogger := logger.FromContext(ctx).With(zap.String("client_request_id", strings.TrimSpace(id)))
		ctx = logger.IntoContext(ctx, requestLogger)
		c.Request = c.Request.WithContext(ctx)
		stampInboundHost(c)
		c.Next()
	}
}

// stampInboundHost stores the public entry hostname on the same request context
// as the client request id. The usage worker copies that context forward.
// X-Sub2-Entry-Host counts only when origin marked the peer as the Hong Kong relay.
func stampInboundHost(c *gin.Context) {
	if c == nil || c.Request == nil {
		return
	}
	host := inboundhost.Resolve(
		c.GetHeader(inboundhost.HeaderEntryHost),
		c.GetHeader(inboundhost.HeaderEntryVia),
		c.Request.Host,
	)
	if host == "" {
		return
	}
	ctx := context.WithValue(c.Request.Context(), ctxkey.InboundHost, host)
	c.Request = c.Request.WithContext(ctx)
}
