package service

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

type gatewayRequestStartCtxKey struct{}

const gatewayRequestStartedAtKey = "gateway_request_started_at"

// StampGatewayRequestStartAt records the inbound request start on gin context
// and the request.Context. Later stamps on the same request are no-ops.
func StampGatewayRequestStartAt(c *gin.Context, started time.Time) {
	if c == nil || started.IsZero() {
		return
	}
	if _, exists := c.Get(gatewayRequestStartedAtKey); !exists {
		c.Set(gatewayRequestStartedAtKey, started)
	}
	if c.Request == nil {
		return
	}
	if v := c.Request.Context().Value(gatewayRequestStartCtxKey{}); v != nil {
		return
	}
	c.Request = c.Request.WithContext(WithGatewayRequestStart(c.Request.Context(), started))
}

// WithGatewayRequestStart stores the inbound start on a context.Context.
func WithGatewayRequestStart(ctx context.Context, started time.Time) context.Context {
	if ctx == nil || started.IsZero() {
		return ctx
	}
	if v := ctx.Value(gatewayRequestStartCtxKey{}); v != nil {
		return ctx
	}
	return context.WithValue(ctx, gatewayRequestStartCtxKey{}, started)
}

func gatewayRequestStartedAt(c *gin.Context, ctx context.Context, fallback time.Time) time.Time {
	if c != nil {
		if v, ok := c.Get(gatewayRequestStartedAtKey); ok {
			if t, ok := v.(time.Time); ok && !t.IsZero() {
				return t
			}
		}
		if c.Request != nil {
			ctx = c.Request.Context()
		}
	}
	if ctx != nil {
		if t, ok := ctx.Value(gatewayRequestStartCtxKey{}).(time.Time); ok && !t.IsZero() {
			return t
		}
	}
	if !fallback.IsZero() {
		return fallback
	}
	return time.Now()
}

func elapsedMsSince(start time.Time) int {
	if start.IsZero() {
		return 0
	}
	ms := int(time.Since(start).Milliseconds())
	if ms < 0 {
		return 0
	}
	return ms
}

func requestElapsedMs(c *gin.Context, hopStart time.Time) int {
	req := elapsedMsSince(gatewayRequestStartedAt(c, nil, hopStart))
	hop := elapsedMsSince(hopStart)
	if req < hop {
		return hop
	}
	return req
}

func durationMsForUsage(ctx context.Context, hopDuration time.Duration) int {
	hopMs := int(hopDuration.Milliseconds())
	if hopMs < 0 {
		hopMs = 0
	}
	start := gatewayRequestStartedAt(nil, ctx, time.Time{})
	if start.IsZero() {
		return hopMs
	}
	reqMs := elapsedMsSince(start)
	if reqMs < hopMs {
		return hopMs
	}
	return reqMs
}

func stampRequestFirstTokenMs(dst **int, c *gin.Context, hopStart time.Time) {
	if dst == nil || *dst != nil {
		return
	}
	ms := requestElapsedMs(c, hopStart)
	*dst = &ms
}

func stampHopFirstTokenMs(dst **int, hopStart time.Time) {
	if dst == nil || *dst != nil {
		return
	}
	ms := elapsedMsSince(hopStart)
	*dst = &ms
}

func ensureDurationCoversFirstToken(durationMs int, firstTokenMs *int) int {
	if firstTokenMs != nil && *firstTokenMs > durationMs {
		return *firstTokenMs
	}
	return durationMs
}
