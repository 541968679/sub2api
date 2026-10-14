//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func resetForceRequestedResponseModelCacheForTest() {
	forceRequestedResponseModelSF.Forget("force_requested_response_model")
	forceRequestedResponseModelCache.Store(&cachedForceRequestedResponseModel{})
}

func TestSettingService_ForceRequestedResponseModelDefaultOn(t *testing.T) {
	resetForceRequestedResponseModelCacheForTest()
	t.Cleanup(resetForceRequestedResponseModelCacheForTest)

	repo := &gatewayTTLSettingRepo{data: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	ctx := context.Background()

	require.True(t, svc.IsForceRequestedResponseModelEnabled(ctx))
	settings, err := svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.True(t, settings.ForceRequestedResponseModel)
	require.False(t, (*SettingService)(nil).IsForceRequestedResponseModelEnabled(ctx))

	repo.data[SettingKeyForceRequestedResponseModel] = "true"
	resetForceRequestedResponseModelCacheForTest()
	require.True(t, svc.IsForceRequestedResponseModelEnabled(ctx))

	repo.data[SettingKeyForceRequestedResponseModel] = "TRUE"
	resetForceRequestedResponseModelCacheForTest()
	require.False(t, svc.IsForceRequestedResponseModelEnabled(ctx))

	repo.data[SettingKeyForceRequestedResponseModel] = "true"
	resetForceRequestedResponseModelCacheForTest()
	require.True(t, svc.IsForceRequestedResponseModelEnabled(ctx))
	repo.data[SettingKeyForceRequestedResponseModel] = "false"
	require.True(t, svc.IsForceRequestedResponseModelEnabled(ctx))
	refreshForceRequestedResponseModelCache(false)
	require.False(t, svc.IsForceRequestedResponseModelEnabled(ctx))
}

func TestForceRewriteClientModelFields(t *testing.T) {
	in := []byte(`{"id":"chatcmpl_1","model":"gpt-4o-2024-08-06","response":{"model":"gpt-4o-2024-08-06"},"message":{"model":"claude-sonnet-4-5-20250929"},"modelVersion":"gemini-2.5-pro-001","choices":[{"delta":{"content":"hi"}}]}`)
	got, changed := forceRewriteClientModelFields(in, "gpt-4o")
	require.True(t, changed)
	require.Contains(t, string(got), `"model":"gpt-4o"`)
	require.Contains(t, string(got), `"response":{"model":"gpt-4o"}`)
	require.Contains(t, string(got), `"message":{"model":"gpt-4o"}`)
	require.Contains(t, string(got), `"modelVersion":"gpt-4o"`)
	require.NotContains(t, string(got), "2024-08-06")
	require.Contains(t, string(got), `"content":"hi"`)

	same := []byte(`{"model":"gpt-4o","usage":{"prompt_tokens":3}}`)
	out, changed := forceRewriteClientModelFields(same, "gpt-4o")
	require.False(t, changed)
	require.Equal(t, string(same), string(out))

	emptyKept := []byte(`{"model":"","response":{"model":null}}`)
	out, changed = forceRewriteClientModelFields(emptyKept, "gpt-4o")
	require.False(t, changed)
	require.Equal(t, string(emptyKept), string(out))

	errBody := []byte(`{"type":"error","error":{"message":"model gpt-4o-2024-08-06 is unavailable"}}`)
	out, changed = forceRewriteClientModelFields(errBody, "gpt-4o")
	require.False(t, changed)
	require.Contains(t, string(out), "gpt-4o-2024-08-06")
}

func TestApplyForcedRequestedResponseModelUsesArmedClientName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetForceRequestedResponseModelCacheForTest()
	t.Cleanup(resetForceRequestedResponseModelCacheForTest)

	repo := &gatewayTTLSettingRepo{data: map[string]string{
		SettingKeyForceRequestedResponseModel: "true",
	}}
	settings := NewSettingService(repo, &config.Config{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ArmForceRequestedResponseModel(context.Background(), c, settings, "kimi-k3")
	ArmForceRequestedResponseModel(context.Background(), c, settings, "mapped-upstream-name")

	line := `data: {"model":"kimi-k3-2026-01","choices":[{"delta":{"content":"a"}}]}`
	got := applyForcedRequestedResponseModelLine(c, line)
	require.Contains(t, got, `"model":"kimi-k3"`)
	require.NotContains(t, got, "kimi-k3-2026-01")
	require.Equal(t, "data: [DONE]", applyForcedRequestedResponseModelLine(c, "data: [DONE]"))
	require.Equal(t, "event: ping", applyForcedRequestedResponseModelLine(c, "event: ping"))

	body := []byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-dated\"}}\n\n")
	rewritten := applyForcedRequestedResponseModelSSEText(c, string(body))
	require.Contains(t, rewritten, `"model":"kimi-k3"`)
	require.Contains(t, rewritten, "event: message_start")
	require.NotContains(t, rewritten, "claude-dated")

	off := httptest.NewRecorder()
	offCtx, _ := gin.CreateTestContext(off)
	require.Equal(t, line, applyForcedRequestedResponseModelLine(offCtx, line))
}
