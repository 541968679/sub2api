//go:build unit

package handler

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
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

// bridgeCountUpstreamStub 记录 bridge count 发出的上游请求（URL + body），
// 并按预设状态码/响应体应答，用于覆盖 ready 路径的真实转发。
type bridgeCountUpstreamStub struct {
	status int
	body   string
	reqs   []*http.Request
	bodies [][]byte
}

func (u *bridgeCountUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
	}
	u.reqs = append(u.reqs, req)
	u.bodies = append(u.bodies, reqBody)
	return &http.Response{
		StatusCode: u.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(u.body)),
	}, nil
}

func (u *bridgeCountUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

// newBridgeCountReadyTestHandler 组装 ready 路径所需的最小依赖。
// 服务经 Wire 构造函数创建（cfg 必须非 nil：api-key 账号的
// validateUpstreamBaseURL 会解引用 cfg），账号仓储与上游经测试注入器替换。
// 其余依赖刻意保持 nil，从结构上锁死 count_tokens 的“无副作用”不变量
//（测试矩阵第 18 行）：
//   - usage/billing 仓储为 nil —— count 路径若尝试写 usage/计费会直接
//     nil panic，测试即失败；
//   - concurrencyService 为 nil —— 并发槽获取是 no-op，不持有任何槽位；
//   - 无 scheduler 测试替身 —— 选号走真实负载感知路径。
func newBridgeCountReadyTestHandler(repo service.AccountRepository, upstream service.HTTPUpstream) *OpenAIGatewayHandler {
	svc := service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil, nil,
		&config.Config{},
		nil, nil, nil, nil, nil,
		nil,
		nil, nil, nil, nil, nil, nil,
	)
	svc.SetAccountRepoForTest(repo)
	svc.SetHTTPUpstreamForTest(upstream)
	return &OpenAIGatewayHandler{
		gatewayService:      svc,
		billingCacheService: service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}),
	}
}

// newBridgeCountReadyTestAccount 在通用 bridge 账号上补齐 ready 转发所需的
// api-key 凭据：GetAccessToken 的 api_key 分支读 credentials["api_key"]，
// buildInputTokensUpstreamRequest 对 api-key 账号读 credentials["base_url"]。
func newBridgeCountReadyTestAccount(id int64) service.Account {
	return newBridgeRouteTestAccount(id, func(a *service.Account) {
		a.Credentials["api_key"] = "sk-test"
		a.Credentials["base_url"] = "https://upstream.example"
	})
}

// ready 路径：显式映射 + 可调度 bridge 账号时，count_tokens 经真实选号
// 走到 bridge 上游 /v1/responses/input_tokens 计数，请求体携带映射后的
// 上游模型而非原始 Claude 模型。
func TestCountTokensClaudeGPTBridge_ReadyPathCountsThroughBridgeUpstream(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeCountReadyTestAccount(1)}}
	upstream := &bridgeCountUpstreamStub{status: http.StatusOK, body: `{"input_tokens":42}`}
	h := newBridgeCountReadyTestHandler(repo, upstream)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeCountTokensTestBody)

	handled := h.CountTokensClaudeGPTBridge(c)

	require.True(t, handled, "ready bridge must handle count_tokens instead of falling to native")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 42, int(gjson.Get(rec.Body.String(), "input_tokens").Int()),
		"response must carry the upstream count, not a local estimate")

	require.Len(t, upstream.reqs, 1, "ready path must call the bridge upstream exactly once")
	require.Equal(t, "https://upstream.example/v1/responses/input_tokens", upstream.reqs[0].URL.String())
	require.Equal(t, "Bearer sk-test", upstream.reqs[0].Header.Get("authorization"))
	require.Equal(t, "gpt-5.5", gjson.GetBytes(upstream.bodies[0], "model").String(),
		"upstream body must carry the mapped model")
	require.NotContains(t, string(upstream.bodies[0]), "claude-opus-4-8",
		"the requested claude model must not leak into the upstream body")
}

// ready 路径上游 500：bridge 宽松语义降级为本地估算返回 200，
// 且上游确实被调用过一次——证明走的是 ready 转发，而非 blocked 状态的
// 本地估算捷径。
func TestCountTokensClaudeGPTBridge_ReadyPathUpstream500DegradesToLocalEstimate(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeCountReadyTestAccount(1)}}
	upstream := &bridgeCountUpstreamStub{status: http.StatusInternalServerError, body: `{"error":{"message":"boom"}}`}
	h := newBridgeCountReadyTestHandler(repo, upstream)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeCountTokensTestBody)

	handled := h.CountTokensClaudeGPTBridge(c)

	require.True(t, handled)
	require.Equal(t, http.StatusOK, rec.Code,
		"bridge count must never surface upstream 5xx to the client")
	require.GreaterOrEqual(t, int(gjson.Get(rec.Body.String(), "input_tokens").Int()), 1,
		"local tiktoken estimate must produce at least the fallback minimum")
	require.Len(t, upstream.reqs, 1,
		"degradation must have gone through the ready forward path exactly once")
}
