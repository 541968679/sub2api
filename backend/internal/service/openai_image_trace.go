package service

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	OpenAIImageTraceEnv = "OPENAI_IMAGE_TRACE_LOG"

	openAIImageTraceGinKey = "openai_image_trace"
)

// OpenAIImageTrace records temporary timing diagnostics for gpt-image-2 image
// generation requests. It intentionally stores only safe request attributes.
type OpenAIImageTrace struct {
	startedAt         time.Time
	requestID         string
	clientRequestID   string
	traceID           string
	accountID         int64
	model             string
	size              string
	quality           string
	endpoint          string
	stream            bool
	n                 int
	multipart         bool
	upstreamRequestID string
}

func NewOpenAIImageTrace(c *gin.Context, parsed *OpenAIImagesRequest, startedAt time.Time) *OpenAIImageTrace {
	if !openAIImageTraceEnabled() || c == nil || parsed == nil {
		return nil
	}
	if parsed.Endpoint != openAIImagesGenerationsEndpoint {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(parsed.Model), "gpt-image-2") {
		return nil
	}
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	ctx := requestContextFromGin(c)
	requestID := contextString(ctx, ctxkey.RequestID)
	clientRequestID := contextString(ctx, ctxkey.ClientRequestID)
	traceID := imageTraceID(c, requestID, clientRequestID, startedAt)
	trace := &OpenAIImageTrace{
		startedAt:       startedAt,
		requestID:       requestID,
		clientRequestID: clientRequestID,
		traceID:         traceID,
		model:           strings.TrimSpace(parsed.Model),
		size:            strings.TrimSpace(parsed.Size),
		quality:         strings.TrimSpace(parsed.Quality),
		endpoint:        strings.TrimSpace(parsed.Endpoint),
		stream:          parsed.Stream,
		n:               parsed.N,
		multipart:       parsed.Multipart,
	}
	c.Set(openAIImageTraceGinKey, trace)
	return trace
}

func OpenAIImageTraceFromGin(c *gin.Context) *OpenAIImageTrace {
	if c == nil {
		return nil
	}
	v, ok := c.Get(openAIImageTraceGinKey)
	if !ok {
		return nil
	}
	trace, _ := v.(*OpenAIImageTrace)
	return trace
}

func (t *OpenAIImageTrace) SetAccountID(accountID int64) {
	if t == nil || accountID <= 0 {
		return
	}
	t.accountID = accountID
}

func (t *OpenAIImageTrace) Log(c *gin.Context, event string, statusCode int, upstreamRequestID string, fields ...zap.Field) {
	t.LogAt(c, event, time.Now(), statusCode, upstreamRequestID, fields...)
}

func (t *OpenAIImageTrace) LogAt(c *gin.Context, event string, at time.Time, statusCode int, upstreamRequestID string, fields ...zap.Field) {
	if t == nil {
		return
	}
	event = strings.TrimSpace(event)
	if event == "" {
		return
	}
	baseFields := t.fields(c, event, at, statusCode, upstreamRequestID)
	if len(fields) > 0 {
		baseFields = append(baseFields, fields...)
	}
	logger.FromContext(requestContextFromGin(c)).
		With(zap.String("component", "handler.openai_gateway.images_trace")).
		Info("openai.images.trace", baseFields...)
}

func (t *OpenAIImageTrace) fields(c *gin.Context, event string, at time.Time, statusCode int, upstreamRequestID string) []zap.Field {
	if at.IsZero() {
		at = time.Now()
	}
	elapsedMs := at.Sub(t.startedAt).Milliseconds()
	if elapsedMs < 0 {
		elapsedMs = 0
	}
	upstreamRequestID = strings.TrimSpace(upstreamRequestID)
	if upstreamRequestID != "" {
		t.upstreamRequestID = upstreamRequestID
	} else {
		upstreamRequestID = t.upstreamRequestID
	}
	if statusCode < 0 {
		statusCode = 0
	}
	return []zap.Field{
		zap.String("event", strings.TrimSpace(event)),
		zap.String("request_id", t.requestID),
		zap.String("client_request_id", t.clientRequestID),
		zap.String("trace_id", t.traceID),
		zap.Int64("account_id", t.accountID),
		zap.String("model", t.model),
		zap.String("size", t.size),
		zap.String("quality", t.quality),
		zap.Bool("stream", t.stream),
		zap.Int("status_code", statusCode),
		zap.String("t_wall", at.UTC().Format(time.RFC3339Nano)),
		zap.Int64("elapsed_ms", elapsedMs),
		zap.String("upstream_request_id", upstreamRequestID),
		zap.String("endpoint", t.endpoint),
		zap.Int("n", t.n),
		zap.Bool("multipart", t.multipart),
	}
}

func openAIImageTraceEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(OpenAIImageTraceEnv))) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

func requestContextFromGin(c *gin.Context) context.Context {
	if c != nil && c.Request != nil && c.Request.Context() != nil {
		return c.Request.Context()
	}
	return context.Background()
}

func contextString(ctx context.Context, key ctxkey.Key) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return strings.TrimSpace(value)
}

func imageTraceID(c *gin.Context, requestID string, clientRequestID string, startedAt time.Time) string {
	for _, header := range []string{"X-Trace-ID", "X-Trace-Id", "Trace-ID", "Traceparent"} {
		if value := sanitizeImageTraceIdentifier(c.GetHeader(header)); value != "" {
			return value
		}
	}
	if requestID = sanitizeImageTraceIdentifier(requestID); requestID != "" {
		return requestID
	}
	if clientRequestID = sanitizeImageTraceIdentifier(clientRequestID); clientRequestID != "" {
		return clientRequestID
	}
	return "image-trace-" + strconv.FormatInt(startedAt.UnixNano(), 10)
}

func sanitizeImageTraceIdentifier(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 128 {
		value = value[:128]
	}
	return value
}
