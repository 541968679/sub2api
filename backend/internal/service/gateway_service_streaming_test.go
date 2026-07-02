package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGatewayService_StreamingReusesScannerBufferAndStillParsesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}

	svc := &GatewayService{
		cfg:              cfg,
		rateLimitService: &RateLimitService{},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}

	go func() {
		defer func() { _ = pw.Close() }()
		// Minimal SSE event to trigger parseSSEUsage
		_, _ = pw.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":3}}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n"))
		_, _ = pw.Write([]byte("data: [DONE]\n\n"))
	}()

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 3, result.usage.InputTokens)
	require.Equal(t, 7, result.usage.OutputTokens)
}

// extractStreamingSSEDataByType 从录制的下游响应体中取出指定 type 的 SSE data JSON。
func extractStreamingSSEDataByType(t *testing.T, body, eventType string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if gjson.Get(data, "type").String() == eventType {
			return data
		}
	}
	t.Fatalf("no SSE data line with type %q in downstream body:\n%s", eventType, body)
	return ""
}

// 计费污染回归：display 模式(非平凡倍率)下,计费 usage 必须是上游真实值,
// 只有发给客户端的 SSE 才是展示值。
func TestGatewayService_StreamingDisplayModeBillsRealTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &GatewayService{cfg: cfg, rateLimitService: &RateLimitService{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	SetDisplayTokenMultipliers(c, &DisplayTokenMultipliers{
		InputMult:       2,
		OutputMult:      3,
		CacheReadMult:   1,
		CacheCreateMult: 1,
		RateScale:       1,
		RateScaleSet:    true,
	})

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte(`data: {"type":"message_start","message":{"usage":{"input_tokens":100,"cache_read_input_tokens":20,"cache_creation_input_tokens":5,"cache_creation":{"ephemeral_5m_input_tokens":3,"ephemeral_1h_input_tokens":2}}}}` + "\n\n"))
		_, _ = pw.Write([]byte(`data: {"type":"message_delta","usage":{"output_tokens":10}}` + "\n\n"))
		_, _ = pw.Write([]byte("data: [DONE]\n\n"))
	}()

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)

	// 计费侧:真实值
	require.Equal(t, 100, result.usage.InputTokens, "billing must use real input tokens")
	require.Equal(t, 10, result.usage.OutputTokens, "billing must use real output tokens")
	require.Equal(t, 20, result.usage.CacheReadInputTokens)
	require.Equal(t, 5, result.usage.CacheCreationInputTokens)
	require.Equal(t, 3, result.usage.CacheCreation5mTokens)
	require.Equal(t, 2, result.usage.CacheCreation1hTokens)

	// 下游侧:展示值
	body := rec.Body.String()
	start := extractStreamingSSEDataByType(t, body, "message_start")
	require.EqualValues(t, 200, gjson.Get(start, "message.usage.input_tokens").Int(), "downstream input should be display value")
	require.EqualValues(t, 20, gjson.Get(start, "message.usage.cache_read_input_tokens").Int())
	delta := extractStreamingSSEDataByType(t, body, "message_delta")
	require.EqualValues(t, 30, gjson.Get(delta, "usage.output_tokens").Int(), "downstream output should be display value")
}

// TTL override 必须仍先于计费提取生效(归类刻意影响计费),display 改写在其后。
func TestGatewayService_StreamingDisplayModeKeepsTTLOverrideBeforeBillingPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &GatewayService{cfg: cfg, rateLimitService: &RateLimitService{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	SetDisplayTokenMultipliers(c, &DisplayTokenMultipliers{
		InputMult:       2,
		OutputMult:      1,
		CacheReadMult:   1,
		CacheCreateMult: 1,
		RateScale:       1,
		RateScaleSet:    true,
	})

	account := &Account{
		ID:       1,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth, // TTL override 仅对 OAuth/SetupToken 账号生效
		Extra: map[string]any{
			"cache_ttl_override_enabled": true,
			"cache_ttl_override_target":  "5m",
		},
	}

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte(`data: {"type":"message_start","message":{"usage":{"input_tokens":50,"cache_creation_input_tokens":7,"cache_creation":{"ephemeral_5m_input_tokens":3,"ephemeral_1h_input_tokens":4}}}}` + "\n\n"))
		_, _ = pw.Write([]byte("data: [DONE]\n\n"))
	}()

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "model", "model", false)
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)

	// 计费侧:真实 token 数 + TTL override 归类后的 5m/1h
	require.Equal(t, 50, result.usage.InputTokens, "billing must use real input tokens")
	require.Equal(t, 7, result.usage.CacheCreationInputTokens)
	require.Equal(t, 7, result.usage.CacheCreation5mTokens, "TTL override must reclassify before billing patch extraction")
	require.Equal(t, 0, result.usage.CacheCreation1hTokens)

	// 下游侧:input 是展示值,嵌套 5m/1h 是归类后的值
	start := extractStreamingSSEDataByType(t, rec.Body.String(), "message_start")
	require.EqualValues(t, 100, gjson.Get(start, "message.usage.input_tokens").Int())
	require.EqualValues(t, 7, gjson.Get(start, "message.usage.cache_creation.ephemeral_5m_input_tokens").Int())
	require.EqualValues(t, 0, gjson.Get(start, "message.usage.cache_creation.ephemeral_1h_input_tokens").Int())
}
