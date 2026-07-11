//go:build unit

package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type bridgeRouteAccountRepoStub struct {
	service.AccountRepository
	accounts  []service.Account
	listErr   error
	listCalls int
}

func (r *bridgeRouteAccountRepoStub) ListByGroup(_ context.Context, _ int64) ([]service.Account, error) {
	r.listCalls++
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]service.Account, len(r.accounts))
	copy(out, r.accounts)
	return out, nil
}

func (r *bridgeRouteAccountRepoStub) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			reset := resetAt
			r.accounts[i].RateLimitResetAt = &reset
		}
	}
	return nil
}

func (r *bridgeRouteAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, _ string) ([]service.Account, error) {
	out := make([]service.Account, len(r.accounts))
	copy(out, r.accounts)
	return out, nil
}

type bridgeRouteSchedulerSpy struct {
	selectCalls int32
}

func (s *bridgeRouteSchedulerSpy) Select(context.Context, service.OpenAIAccountScheduleRequest) (*service.AccountSelectionResult, service.OpenAIAccountScheduleDecision, error) {
	atomic.AddInt32(&s.selectCalls, 1)
	return nil, service.OpenAIAccountScheduleDecision{}, errors.New("scheduler must not be consulted by route diagnosis")
}

func (s *bridgeRouteSchedulerSpy) ReportResult(int64, bool, *int) {}

func (s *bridgeRouteSchedulerSpy) ReportSwitch() {}

func (s *bridgeRouteSchedulerSpy) SnapshotMetrics() service.OpenAIAccountSchedulerMetricsSnapshot {
	return service.OpenAIAccountSchedulerMetricsSnapshot{}
}

func newBridgeRouteTestAccount(id int64, mutate ...func(*service.Account)) service.Account {
	account := service.Account{
		ID:          id,
		Name:        "bridge-account",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Extra:       map[string]any{"openai_claude_gpt_bridge_enabled": true},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"claude-opus-4-8": "gpt-5.5"},
		},
	}
	for _, fn := range mutate {
		fn(&account)
	}
	return account
}

func newBridgeRouteTestHandler(repo service.AccountRepository, scheduler service.OpenAIAccountScheduler) *OpenAIGatewayHandler {
	svc := &service.OpenAIGatewayService{}
	svc.SetAccountRepoForTest(repo)
	if scheduler != nil {
		svc.SetOpenAIAccountSchedulerForTest(scheduler)
	}
	return &OpenAIGatewayHandler{gatewayService: svc}
}

func newBridgeRouteTestContext(t *testing.T, platform string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(7)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      11,
		GroupID: &groupID,
		Group:   &service.Group{ID: groupID, Platform: platform},
	})
	return c, rec
}

const bridgeRouteTestBody = `{"model":"claude-opus-4-8","messages":[{"role":"user","content":"hi"}]}`

func TestClaudeGPTBridgeRoute_RateLimitedReturns429WithRetryAfter(t *testing.T) {
	resetAt := time.Now().Add(90 * time.Second)
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1, func(a *service.Account) {
		a.RateLimitResetAt = &resetAt
	})}}
	scheduler := &bridgeRouteSchedulerSpy{}
	h := newBridgeRouteTestHandler(repo, scheduler)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)

	action := h.ClaudeGPTBridgeRoute(c)

	require.Equal(t, ClaudeGPTBridgeRouteActionHandled, action)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "rate_limit_error", gjson.Get(rec.Body.String(), "error.type").String())

	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err, "429 must carry a positive integer Retry-After header")
	require.GreaterOrEqual(t, retryAfter, 1)
	require.LessOrEqual(t, retryAfter, 90)
	require.GreaterOrEqual(t, retryAfter, 80, "Retry-After should reflect the earliest recovery time")

	require.Zero(t, atomic.LoadInt32(&scheduler.selectCalls), "route diagnosis must not acquire scheduler slots")
	require.Equal(t, 1, repo.listCalls)
}

// 分组禁用的模型必须稳定返回 403，不随 bridge 容量状态在 403/429/503 间摆动。
func TestClaudeGPTBridgeRoute_GroupBlockedModelReturns403EvenWhenRateLimited(t *testing.T) {
	resetAt := time.Now().Add(90 * time.Second)
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1, func(a *service.Account) {
		a.RateLimitResetAt = &resetAt
	})}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	apiKey.Group.BlockedModels = []string{"claude-opus-4-8"}

	action := h.ClaudeGPTBridgeRoute(c)

	require.Equal(t, ClaudeGPTBridgeRouteActionHandled, action)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "permission_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Empty(t, rec.Header().Get("Retry-After"))
}

// Retry-After 以 24h 为上限，防止上游控制的 resets_at 注入荒谬的等待时间。
func TestClaudeGPTBridgeRoute_RetryAfterIsCappedAtOneDay(t *testing.T) {
	resetAt := time.Now().Add(72 * time.Hour)
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1, func(a *service.Account) {
		a.RateLimitResetAt = &resetAt
	})}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)

	action := h.ClaudeGPTBridgeRoute(c)

	require.Equal(t, ClaudeGPTBridgeRouteActionHandled, action)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "86400", rec.Header().Get("Retry-After"))
}

func TestClaudeGPTBridgeRoute_UnavailableReturns503(t *testing.T) {
	tempUntil := time.Now().Add(10 * time.Minute)
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1, func(a *service.Account) {
		a.TempUnschedulableUntil = &tempUntil
	})}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)

	action := h.ClaudeGPTBridgeRoute(c)

	require.Equal(t, ClaudeGPTBridgeRouteActionHandled, action)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "overloaded_error", gjson.Get(rec.Body.String(), "error.type").String())
}

func TestClaudeGPTBridgeRoute_ProbeErrorReturns503WithoutNativeFallback(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{listErr: errors.New("db down")}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)

	action := h.ClaudeGPTBridgeRoute(c)

	require.Equal(t, ClaudeGPTBridgeRouteActionHandled, action,
		"probe errors must stay on bridge error semantics instead of native fallback")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "api_error", gjson.Get(rec.Body.String(), "error.type").String())
}

func TestClaudeGPTBridgeRoute_NotConfiguredFallsBackNativeWithBodyPreserved(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)

	action := h.ClaudeGPTBridgeRoute(c)

	require.Equal(t, ClaudeGPTBridgeRouteActionNative, action)
	require.Zero(t, rec.Body.Len(), "native fallback must not write a response")

	replayed, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, bridgeRouteTestBody, string(replayed),
		"request body must be reset so the native handler can consume it")
}

func TestClaudeGPTBridgeRoute_ReadyDispatchesBridgeWithBodyPreserved(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1)}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)

	action := h.ClaudeGPTBridgeRoute(c)

	require.Equal(t, ClaudeGPTBridgeRouteActionBridge, action)
	require.Zero(t, rec.Body.Len())

	replayed, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, bridgeRouteTestBody, string(replayed))
}

func TestClaudeGPTBridgeRoute_InvalidRequestsReturn400(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		message string
	}{
		{name: "invalid json", body: `{"model": bridge`, message: "Failed to parse request body"},
		{name: "missing model", body: `{"messages":[{"role":"user","content":"hi"}]}`, message: "model is required"},
		{name: "empty body", body: "", message: "Request body is empty"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1)}}
			h := newBridgeRouteTestHandler(repo, nil)
			c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, tt.body)

			action := h.ClaudeGPTBridgeRoute(c)

			require.Equal(t, ClaudeGPTBridgeRouteActionHandled, action,
				"protocol errors must return a canonical 400 instead of masquerading as a native miss")
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.type").String())
		})
	}
}

func TestClaudeGPTBridgeRoute_NonAntigravityPlatformIsNative(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1)}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAnthropic, bridgeRouteTestBody)

	action := h.ClaudeGPTBridgeRoute(c)

	require.Equal(t, ClaudeGPTBridgeRouteActionNative, action)
	require.Zero(t, rec.Body.Len())
	require.Zero(t, repo.listCalls, "non-Antigravity groups must not trigger bridge diagnosis")
}

// TestClaudeGPTBridgeRoute_TwoRequestRateLimitDoesNotReachNative 复刻调查文档的
// 关键两请求回归的 handler/dispatch 部分：唯一 bridge 账号被上游 429 写入限流后，
// 第二个请求必须在路由层拿到 429 + Retry-After，且绝不进入 native 分支。
func TestClaudeGPTBridgeRoute_TwoRequestRateLimitDoesNotReachNative(t *testing.T) {
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1)}}

	// 请求 1 的效果：上游 429 usage_limit_reached 经由 RateLimitService 写入限流状态。
	resetsAt := time.Now().Add(2 * time.Minute)
	rateLimitSvc := service.NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := repo.accounts[0]
	respBody := []byte(`{"error":{"type":"usage_limit_reached","message":"You have hit your usage limit.","resets_at":` +
		strconv.FormatInt(resetsAt.Unix(), 10) + `}}`)
	rateLimitSvc.HandleUpstreamError(context.Background(), &account, http.StatusTooManyRequests, http.Header{}, respBody)
	require.NotNil(t, repo.accounts[0].RateLimitResetAt, "first 429 must persist rate limit state")

	// 请求 2：按 routes 层的分发逻辑执行，native 分支必须不可达。
	scheduler := &bridgeRouteSchedulerSpy{}
	h := newBridgeRouteTestHandler(repo, scheduler)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)

	nativeCalled := false
	bridgeCalled := false
	switch h.ClaudeGPTBridgeRoute(c) {
	case ClaudeGPTBridgeRouteActionBridge:
		bridgeCalled = true
	case ClaudeGPTBridgeRouteActionHandled:
	default:
		nativeCalled = true
	}

	require.False(t, nativeCalled, "rate-limited bridge must not fall back to the native Antigravity pool")
	require.False(t, bridgeCalled)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "rate_limit_error", gjson.Get(rec.Body.String(), "error.type").String())
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, retryAfter, 1)
	require.Zero(t, atomic.LoadInt32(&scheduler.selectCalls))
}

func TestRespondClaudeGPTBridgeSelectionRace_RateLimitedReturns429(t *testing.T) {
	resetAt := time.Now().Add(60 * time.Second)
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1, func(a *service.Account) {
		a.RateLimitResetAt = &resetAt
	})}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)
	groupID := int64(7)

	h.respondClaudeGPTBridgeSelectionRace(c, &groupID, "claude-opus-4-8", false)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "rate_limit_error", gjson.Get(rec.Body.String(), "error.type").String())
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, retryAfter, 1)
	require.LessOrEqual(t, retryAfter, 60)
}

func TestRespondClaudeGPTBridgeSelectionRace_OtherBlockersReturn503(t *testing.T) {
	tempUntil := time.Now().Add(10 * time.Minute)
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1, func(a *service.Account) {
		a.TempUnschedulableUntil = &tempUntil
	})}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)
	groupID := int64(7)

	h.respondClaudeGPTBridgeSelectionRace(c, &groupID, "claude-opus-4-8", false)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "overloaded_error", gjson.Get(rec.Body.String(), "error.type").String())
}

func TestRespondClaudeGPTBridgeSelectionRace_MappingDeletedStaysOnBridgeError(t *testing.T) {
	// bridge 选中后 mapping 被删除：诊断降级为 not_configured，
	// 但当前请求必须返回 bridge 侧错误，不允许半途切换 native。
	repo := &bridgeRouteAccountRepoStub{}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)
	groupID := int64(7)

	h.respondClaudeGPTBridgeSelectionRace(c, &groupID, "claude-opus-4-8", false)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "overloaded_error", gjson.Get(rec.Body.String(), "error.type").String())
}

// 所有 bridge 账号都因上游 429 用尽时，最终响应保持 429 语义，
// 并透传经过校验的上游 Retry-After。
func TestHandleAnthropicFailoverExhausted_BridgeMode429PropagatesRetryAfter(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)
	c.Set("openai_claude_gpt_bridge", true)

	failoverErr := &service.UpstreamFailoverError{
		StatusCode:      http.StatusTooManyRequests,
		ResponseBody:    []byte(`{"error":{"type":"usage_limit_reached","message":"You have hit your usage limit."}}`),
		ResponseHeaders: http.Header{"Retry-After": []string{"300"}},
	}
	h.handleAnthropicFailoverExhausted(c, failoverErr, false)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "usage_limit_reached", gjson.Get(rec.Body.String(), "error.type").String())
	require.Equal(t, "300", rec.Header().Get("Retry-After"))
}

func TestHandleAnthropicFailoverExhausted_NonBridgeModeDoesNotAddRetryAfter(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	c, rec := newBridgeRouteTestContext(t, service.PlatformOpenAI, bridgeRouteTestBody)

	failoverErr := &service.UpstreamFailoverError{
		StatusCode:      http.StatusTooManyRequests,
		ResponseBody:    []byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`),
		ResponseHeaders: http.Header{"Retry-After": []string{"300"}},
	}
	h.handleAnthropicFailoverExhausted(c, failoverErr, false)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Empty(t, rec.Header().Get("Retry-After"),
		"non-bridge behavior must stay unchanged")
}

func TestValidatedUpstreamRetryAfterSeconds(t *testing.T) {
	cases := []struct {
		name    string
		headers http.Header
		want    int
		ok      bool
	}{
		{name: "plain seconds", headers: http.Header{"Retry-After": []string{"60"}}, want: 60, ok: true},
		{name: "lowercase header key", headers: http.Header{"retry-after": []string{"5"}}, want: 5, ok: true},
		{name: "nil headers", headers: nil, ok: false},
		{name: "missing header", headers: http.Header{}, ok: false},
		{name: "zero rejected", headers: http.Header{"Retry-After": []string{"0"}}, ok: false},
		{name: "negative rejected", headers: http.Header{"Retry-After": []string{"-30"}}, ok: false},
		{name: "http date rejected", headers: http.Header{"Retry-After": []string{"Fri, 11 Jul 2026 08:00:00 GMT"}}, ok: false},
		{name: "over one day rejected", headers: http.Header{"Retry-After": []string{"172800"}}, ok: false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := validatedUpstreamRetryAfterSeconds(tt.headers)
			require.Equal(t, tt.ok, ok)
			if tt.ok {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRespondClaudeGPTBridgeSelectionRace_StreamStartedUsesSSEError(t *testing.T) {
	resetAt := time.Now().Add(60 * time.Second)
	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(1, func(a *service.Account) {
		a.RateLimitResetAt = &resetAt
	})}}
	h := newBridgeRouteTestHandler(repo, nil)
	c, rec := newBridgeRouteTestContext(t, service.PlatformAntigravity, bridgeRouteTestBody)
	groupID := int64(7)

	h.respondClaudeGPTBridgeSelectionRace(c, &groupID, "claude-opus-4-8", true)

	body := rec.Body.String()
	require.Contains(t, body, "event: error")
	require.Contains(t, body, "rate_limit_error")
}

type bridgeCancelScheduleReport struct {
	accountID int64
	success   bool
}

// bridgeCancelSchedulerSpy 只接管 ReportResult 观测（Select 走真实负载感知路径）。
type bridgeCancelSchedulerSpy struct {
	reports []bridgeCancelScheduleReport
}

func (s *bridgeCancelSchedulerSpy) Select(context.Context, service.OpenAIAccountScheduleRequest) (*service.AccountSelectionResult, service.OpenAIAccountScheduleDecision, error) {
	return nil, service.OpenAIAccountScheduleDecision{}, errors.New("test scheduler select is not used")
}

func (s *bridgeCancelSchedulerSpy) ReportResult(accountID int64, success bool, _ *int) {
	s.reports = append(s.reports, bridgeCancelScheduleReport{accountID: accountID, success: success})
}

func (s *bridgeCancelSchedulerSpy) ReportSwitch() {}

func (s *bridgeCancelSchedulerSpy) SnapshotMetrics() service.OpenAIAccountSchedulerMetricsSnapshot {
	return service.OpenAIAccountSchedulerMetricsSnapshot{}
}

// 客户端取消不得给 bridge 账号记失败：一次取消会把最多 maxAccountSwitches+1 个
// 健康账号的调度评分拉低（规格 6.4 第 7 条）。
func TestMessagesClaudeGPTBridge_ClientCancelDoesNotRecordAccountFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &bridgeRouteAccountRepoStub{accounts: []service.Account{newBridgeRouteTestAccount(77)}}
	scheduler := &bridgeCancelSchedulerSpy{}
	svc := &service.OpenAIGatewayService{}
	svc.SetAccountRepoForTest(repo)
	svc.SetOpenAIAccountSchedulerForTest(scheduler)

	h := &OpenAIGatewayHandler{
		gatewayService:      svc,
		billingCacheService: service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}),
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   &ConcurrencyHelper{concurrencyService: &service.ConcurrencyService{}},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	// messages 字段类型故意错误：ForwardAsAnthropic 在触达任何上游依赖前就会
	// 解析失败，模拟转发中途出错；配合已取消的请求 context 复现客户端断开。
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages",
		strings.NewReader(`{"model":"claude-opus-4-8","messages":"boom"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = c.Request.WithContext(ctx)
	c.Set(openAIClaudeGPTBridgeContextKey, true)

	groupID := int64(7)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      11,
		GroupID: &groupID,
		User:    &service.User{ID: 20},
		Group:   &service.Group{ID: groupID, Platform: service.PlatformAntigravity},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 20, Concurrency: 0})

	require.NotPanics(t, func() { h.Messages(c) })

	for _, report := range scheduler.reports {
		require.True(t, report.success,
			"client cancellation must not record account failure, got failure report for account %d", report.accountID)
	}
}
