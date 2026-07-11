//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 本文件补齐 2026-07-10 调查文档（docs/dev/OPENAI_CLAUDE_GPT_BRIDGE_TIMEOUT_INVESTIGATION_2026-07-10.md
// 第 10 节）评审发现的两个真实转发链路测试缺口：
//
//  1. 既有两请求回归（TestClaudeGPTBridgeTwoRequestRateLimitRegression）是直接调
//     RateLimitService.HandleUpstreamError 播种限流状态；这里改为走真实生产链路：
//     ForwardAsAnthropic 收到上游 HTTP 429 → handleOpenAIAccountUpstreamError →
//     SetRateLimited 持久化 RateLimitResetAt。若重构断掉这条链，本测试必须变红。
//  2. 真实转发路径在构造 UpstreamFailoverError 时必须带上 ResponseHeaders
//     （resp.Header.Clone()），供 handler 侧 validatedUpstreamRetryAfterSeconds
//     在 failover 用尽时透传校验过的上游 Retry-After。

// claudeGPTBridgeForward429Fixture 保存"请求 1"真实 429 转发后的现场，
// 供两个测试分别断言限流持久化与 failover 错误携带的响应头。
type claudeGPTBridgeForward429Fixture struct {
	repo       *claudeGPTBridgeRouteRepoStub
	upstream   *httpUpstreamSequenceRecorder
	svc        *OpenAIGatewayService
	resetsAt   time.Time
	forwardErr error
}

// runClaudeGPTBridgeForwardUpstream429 复用 routing 测试的账号/仓库桩，
// 通过真实 ForwardAsAnthropic 调用打一次上游 429 usage_limit_reached。
func runClaudeGPTBridgeForwardUpstream429(t *testing.T) *claudeGPTBridgeForward429Fixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// bridge 账号：claude-opus-4-8 → gpt-5.5，补上真实转发需要的 api_key/base_url。
	repo := &claudeGPTBridgeRouteRepoStub{accounts: []Account{newClaudeGPTBridgeRouteAccount(1, func(a *Account) {
		a.Credentials["api_key"] = "sk-bridge-test"
		a.Credentials["base_url"] = "http://upstream.example"
	})}}

	// 上游 429：Retry-After 头 + usage_limit_reached 体（resets_at 与 Retry-After 一致）。
	resetsAt := time.Now().Add(300 * time.Second)
	respBody := `{"error":{"type":"usage_limit_reached","message":"You have hit your usage limit.","resets_at":` +
		strconv.FormatInt(resetsAt.Unix(), 10) + `}}`
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{{
		StatusCode: http.StatusTooManyRequests,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Retry-After":  []string{"300"},
		},
		Body: io.NopCloser(strings.NewReader(respBody)),
	}}}

	svc := &OpenAIGatewayService{
		cfg:              rawChatCompletionsTestConfig(),
		accountRepo:      repo,
		httpUpstream:     upstream,
		rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil),
	}

	body := []byte(`{"model":"claude-opus-4-8","max_tokens":256,"stream":true,"messages":[{"role":"user","content":"trigger upstream usage limit"}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// MessagesClaudeGPTBridge 在真实转发前会打上 bridge 模式标记。
	c.Set(openAIClaudeGPTBridgeServiceContextKey, true)

	account := repo.accounts[0]
	result, err := svc.ForwardAsAnthropic(context.Background(), c, &account, body, "bridge-session", "gpt-5.5")
	require.Nil(t, result, "an all-429 forward must not produce a result")
	require.Error(t, err, "an upstream 429 on the real forward path must surface an error")

	return &claudeGPTBridgeForward429Fixture{
		repo:       repo,
		upstream:   upstream,
		svc:        svc,
		resetsAt:   resetsAt,
		forwardErr: err,
	}
}

// TestClaudeGPTBridgeForwardUpstream429_TwoRequestRegressionRealPath 是两请求
// 回归的真实链路版本：请求 1 经 ForwardAsAnthropic 收到上游 429 后，限流状态
// 必须由 handleOpenAIAccountUpstreamError 真正落库；请求 2 在限流窗口内做路由
// 诊断必须得到 rate_limited（绝不误判 not_configured 回落 native 池），且不再
// 发起任何上游请求。
func TestClaudeGPTBridgeForwardUpstream429_TwoRequestRegressionRealPath(t *testing.T) {
	fx := runClaudeGPTBridgeForwardUpstream429(t)

	// 请求 1 只应打一次上游，且账号映射（claude-opus-4-8 → gpt-5.5）在真实链路生效。
	require.Equal(t, 1, fx.upstream.callCount)
	require.Equal(t, "gpt-5.5", gjson.GetBytes(fx.upstream.bodies[0], "model").String(),
		"the real forward path must apply the account-level bridge mapping")

	// 429 → handleOpenAIAccountUpstreamError → SetRateLimited 必须真实持久化。
	require.GreaterOrEqual(t, fx.repo.setRateLimitedCalls, 1,
		"a real upstream 429 must persist the rate limit state through the production chain")
	require.NotNil(t, fx.repo.accounts[0].RateLimitResetAt,
		"RateLimitResetAt must be written back through the account repository")
	require.True(t, fx.repo.accounts[0].RateLimitResetAt.After(time.Now()),
		"the persisted rate limit reset time must be in the future")
	require.WithinDuration(t, fx.resetsAt, *fx.repo.accounts[0].RateLimitResetAt, 2*time.Second,
		"the persisted reset time must come from the upstream resets_at payload")

	// 请求 2：限流窗口内立即重试，诊断必须返回 rate_limited + RetryAt ≈ resets_at。
	groupID := int64(7)
	decision := fx.svc.ResolveClaudeGPTBridgeRoute(context.Background(), &groupID, "claude-opus-4-8")

	require.Equal(t, ClaudeGPTBridgeRouteRateLimited, decision.State,
		"a temporarily rate-limited bridge must never be classified as not_configured")
	require.Equal(t, 1, decision.CandidateCount)
	require.Equal(t, 1, decision.RateLimitedCount)
	require.NotNil(t, decision.RetryAt)
	require.WithinDuration(t, fx.resetsAt, *decision.RetryAt, 2*time.Second)
	require.Equal(t, 1, fx.upstream.callCount,
		"the second request must be answered from local state without reaching upstream")
}

// TestClaudeGPTBridgeForwardUpstream429_FailoverErrorCarriesUpstreamHeaders 断言
// 真实转发路径构造的 UpstreamFailoverError 携带 ResponseHeaders：all-429 用尽时
// handler 侧 validatedUpstreamRetryAfterSeconds 依赖它透传校验过的上游 Retry-After。
func TestClaudeGPTBridgeForwardUpstream429_FailoverErrorCarriesUpstreamHeaders(t *testing.T) {
	fx := runClaudeGPTBridgeForwardUpstream429(t)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, fx.forwardErr, &failoverErr,
		"a bridge upstream 429 must surface as *UpstreamFailoverError")
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.NotNil(t, failoverErr.ResponseHeaders,
		"the real forward path must clone upstream response headers into the failover error")
	require.Equal(t, "300", failoverErr.ResponseHeaders.Get("Retry-After"),
		"the validated upstream Retry-After must survive into UpstreamFailoverError.ResponseHeaders")
	require.Equal(t, "usage_limit_reached", gjson.GetBytes(failoverErr.ResponseBody, "error.type").String())
	require.False(t, failoverErr.RetryableOnSameAccount,
		"a non-pool-mode bridge account must fail over instead of retrying in place")
}
