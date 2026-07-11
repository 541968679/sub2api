//go:build unit

package handler

import (
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const bridgeCountTokensTestBody = `{"model":"claude-opus-4-8","messages":[{"role":"user","content":"hello"}]}`

func TestCountTokensClaudeGPTBridge_NotConfiguredFallsToNativeWithBodyPreserved(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeCountTokensTestBody)

	handled := h.CountTokensClaudeGPTBridge(c)

	require.False(t, handled)
	require.Zero(t, rec.Body.Len())

	replayed, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, bridgeCountTokensTestBody, string(replayed),
		"request body must be reset so the native count handler can consume it")
}

// bridge 账号全部限流时 count_tokens 走本地估算，不进入 native 池，也不发起上游请求。
func TestCountTokensClaudeGPTBridge_RateLimitedProducesLocalEstimate(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Minute)
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1, func(a *service.Account) {
		a.RateLimitResetAt = &resetAt
	})}}
	scheduler := &bridgeRouteSchedulerSpy{}
	h := newBridgeRouteTestHandler(repo, scheduler)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeCountTokensTestBody)

	handled := h.CountTokensClaudeGPTBridge(c)

	require.True(t, handled, "a configured bridge must never fall back to native for count_tokens")
	require.Equal(t, http.StatusOK, rec.Code)
	require.GreaterOrEqual(t, int(gjson.Get(rec.Body.String(), "input_tokens").Int()), 1)
	require.Zero(t, atomic.LoadInt32(&scheduler.selectCalls),
		"rate-limited state must estimate locally without consulting the scheduler")
}

func TestCountTokensClaudeGPTBridge_ProbeErrorProducesLocalEstimate(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{listErr: errTestBridgeProbe}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeCountTokensTestBody)

	handled := h.CountTokensClaudeGPTBridge(c)

	require.True(t, handled, "probe errors must not fall through to the native pool")
	require.Equal(t, http.StatusOK, rec.Code)
	require.GreaterOrEqual(t, int(gjson.Get(rec.Body.String(), "input_tokens").Int()), 1)
}

func TestCountTokensClaudeGPTBridge_InvalidBodyFallsToNative(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1)}}
	h := newBridgeRouteTestHandler(repo, nil)

	cases := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{"model": bridge`},
		{name: "missing model", body: `{"messages":[]}`},
		{name: "empty body", body: ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, tt.body)
			handled := h.CountTokensClaudeGPTBridge(c)
			require.False(t, handled,
				"protocol errors are reported canonically by the native count handler")
			require.Zero(t, rec.Body.Len())
		})
	}
}

// 超限 body 的读错误必须在 bridge 预检就返回 413，
// 不能把被消费的空 body 交给 native 误报 400。
func TestCountTokensClaudeGPTBridge_OversizedBodyReturns413(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1)}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeCountTokensTestBody)
	c.Request.Body = http.MaxBytesReader(nil, c.Request.Body, 8)

	handled := h.CountTokensClaudeGPTBridge(c)

	require.True(t, handled)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.type").String())
}

func TestCountTokensClaudeGPTBridge_NonAntigravityPlatformFallsToNative(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1)}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAnthropic, bridgeCountTokensTestBody)

	handled := h.CountTokensClaudeGPTBridge(c)

	require.False(t, handled)
	require.Zero(t, rec.Body.Len())
	require.Zero(t, repo.listCalls)
}

func TestCountTokensClaudeGPTBridge_GroupModelDenyReturns403(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1)}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeCountTokensTestBody)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	apiKey.Group.BlockedModels = []string{"claude-opus-4-8"}

	handled := h.CountTokensClaudeGPTBridge(c)

	require.True(t, handled)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "permission_error", gjson.Get(rec.Body.String(), "error.type").String())
}

var errTestBridgeProbe = errTestSentinel("bridge probe failed")

type errTestSentinel string

func (e errTestSentinel) Error() string { return string(e) }
