//go:build unit

package routes

// 本文件端到端覆盖 RegisterGatewayRoutes 中真实注册的 Claude-GPT bridge
// 严格路由分发闭包（anthropicMessagesHandler / countTokensHandler），防止
// “handler 单测通过、路由层分发退化”的回归：例如删掉
// `case ClaudeGPTBridgeRouteActionHandled: return` 会把 bridge 429 变成
// native 503，而 handler 层测试全绿。
//
// native-was-not-called 哨兵设计（两层，互补）：
//
//  1. 429/本地估算场景（native 绝不能被调用）：auth stub 故意【不】注入
//     AuthSubject。真实的 native GatewayHandler.Messages / CountTokens 在
//     读取任何未注入依赖之前就会因缺少 AuthSubject 确定性地写出
//     `{"error":{"type":"api_error","message":"User context not found"}}`。
//     若分发闭包错误地在 bridge 已写出响应后继续调用 native，该 JSON 会被
//     追加到响应体尾部 —— 断言「响应体是单个完整 JSON」+「不含哨兵消息」
//     即可确定性地捕获 native 被调用。
//
//  2. native 落地场景（native 必须被调用，且 body 必须完好）：auth stub 注入
//     AuthSubject，并让分组 BlockedModels 屏蔽请求模型。native handler 只有
//     在成功重读并解析请求体、从中取出 model 之后才会命中
//     isGroupModelAllowed 的 403 permission_error；若 bridge 预检消费 body 后
//     未正确重置，native 只会返回 400 "Request body is empty" / 解析失败。
//     因此 403 同时证明「native 确实执行」与「body 在 native 读取时完好」。
//     （bridge 侧对 not_configured 状态不做模型屏蔽检查，直接返回 native
//     动作，所以 BlockedModels 不会干扰分发本身。）
//
// 两个哨兵都不依赖 panic/recover，全部走确定性的早退分支，且早退点位于
// 任何 nil 服务字段的解引用之前（native handler 通过导出构造函数注入
// &service.GatewayService{} 空壳，保证 ResolveChannelMappingAndRestrict 的
// channelService==nil 保护分支生效）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	bridgeDispatchTestBody = `{"model":"claude-opus-4-8","messages":[{"role":"user","content":"hi"}]}`
	// native GatewayHandler 缺少 AuthSubject 时的确定性早退消息（哨兵 1）。
	bridgeDispatchNativeSentinelMessage = "User context not found"
)

// bridgeDispatchAccountRepoStub 是 routes 包内的账号仓库桩：
// ResolveClaudeGPTBridgeRoute 在 standard 模式下只调用 ListByGroup。
// 未实现的方法通过内嵌接口 panic，保证测试不会静默触达意料之外的路径。
type bridgeDispatchAccountRepoStub struct {
	service.AccountRepository
	accounts  []service.Account
	listCalls int
}

func (r *bridgeDispatchAccountRepoStub) ListByGroup(_ context.Context, _ int64) ([]service.Account, error) {
	r.listCalls++
	out := make([]service.Account, len(r.accounts))
	copy(out, r.accounts)
	return out, nil
}

func (r *bridgeDispatchAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, _ string) ([]service.Account, error) {
	out := make([]service.Account, len(r.accounts))
	copy(out, r.accounts)
	return out, nil
}

// newBridgeDispatchTestAccount 与 handler 包 newBridgeRouteTestAccount 等价：
// openai 平台账号 + bridge 开关 + claude-opus-4-8 → gpt-5.5 显式映射。
func newBridgeDispatchTestAccount(id int64, mutate ...func(*service.Account)) service.Account {
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

// newBridgeDispatchTestRouter 通过真实的 RegisterGatewayRoutes 组装 gin 引擎：
// OpenAIGatewayHandler 经导出构造函数持有注入了账号桩的真实 service；
// native GatewayHandler 持有空壳 &service.GatewayService{}，只可能走到
// 哨兵早退分支，不会触达上游或数据库。
func newBridgeDispatchTestRouter(
	repo *bridgeDispatchAccountRepoStub,
	withAuthSubject bool,
	blockedModels []string,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	cfg := &config.Config{}
	// 路由链上的 RequestBodyLimit 直接使用该值构造 MaxBytesReader；
	// 零值会把任何非空 body 判成 413，必须显式给出正数上限。
	cfg.Gateway.MaxBodySize = 4 << 20

	bridgeService := &service.OpenAIGatewayService{}
	bridgeService.SetAccountRepoForTest(repo)

	h := &handler.Handlers{
		Gateway: handler.NewGatewayHandler(
			&service.GatewayService{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			cfg,
			nil,
		),
		OpenAIGateway: handler.NewOpenAIGatewayHandler(bridgeService, nil, nil, nil, nil, nil, cfg),
		Usage:         &handler.UsageHandler{},
	}

	RegisterGatewayRoutes(
		router,
		h,
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(7)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				ID:      11,
				GroupID: &groupID,
				Group: &service.Group{
					ID:            groupID,
					Platform:      service.PlatformAntigravity,
					BlockedModels: blockedModels,
				},
			})
			if withAuthSubject {
				c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 20})
			}
			c.Next()
		}),
		nil,
		nil,
		nil,
		nil,
		cfg,
	)

	return router
}

// gin.New() 未挂 Recovery：任何意外触达 nil 依赖的 panic 会直接炸掉测试，
// 保证“哨兵之外的路径”不可能被静默吞掉。
func performBridgeDispatchRequest(t *testing.T, router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// requireSingleJSONWithoutNativeSentinel 断言响应体是单个完整 JSON 对象且
// 不含 native 哨兵消息：若分发在 bridge 写出响应后又调用了 native，
// native 的错误 JSON 会追加在响应体尾部，两条断言都会失败。
func requireSingleJSONWithoutNativeSentinel(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := rec.Body.String()
	require.NotContains(t, body, bridgeDispatchNativeSentinelMessage,
		"native handler must not run after the bridge already wrote a response")
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &parsed),
		"response body must be a single JSON document without trailing data, got: %s", body)
}

func requireBridgeRateLimited429(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, "rate_limit_error", gjson.Get(rec.Body.String(), "error.type").String())

	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err, "429 must carry a positive integer Retry-After header")
	require.GreaterOrEqual(t, retryAfter, 1)
	require.LessOrEqual(t, retryAfter, 90)
	require.GreaterOrEqual(t, retryAfter, 80, "Retry-After should reflect the earliest recovery time")

	requireSingleJSONWithoutNativeSentinel(t, rec)
}

// 复刻调查文档的关键回归（routes 层视角）：唯一 bridge 账号处于限流冷却时，
// /v1/messages 必须由真实注册的分发闭包返回 429 + Retry-After，
// 绝不允许落入 native Antigravity 池（哨兵 1）。
func TestBridgeDispatch_MessagesRateLimitedReturns429WithoutNative(t *testing.T) {
	resetAt := time.Now().Add(90 * time.Second)
	repo := &bridgeDispatchAccountRepoStub{accounts: []service.Account{
		newBridgeDispatchTestAccount(1, func(a *service.Account) {
			a.RateLimitResetAt = &resetAt
		}),
	}}
	router := newBridgeDispatchTestRouter(repo, false, nil)

	rec := performBridgeDispatchRequest(t, router, "/v1/messages", bridgeDispatchTestBody)

	requireBridgeRateLimited429(t, rec)
	require.Equal(t, 1, repo.listCalls, "route diagnosis should scan the group exactly once")
}

// /antigravity/v1/messages 别名 surface 必须与 /v1/messages 行为完全一致。
func TestBridgeDispatch_AntigravityAliasMessagesRateLimitedReturns429WithoutNative(t *testing.T) {
	resetAt := time.Now().Add(90 * time.Second)
	repo := &bridgeDispatchAccountRepoStub{accounts: []service.Account{
		newBridgeDispatchTestAccount(1, func(a *service.Account) {
			a.RateLimitResetAt = &resetAt
		}),
	}}
	router := newBridgeDispatchTestRouter(repo, false, nil)

	rec := performBridgeDispatchRequest(t, router, "/antigravity/v1/messages", bridgeDispatchTestBody)

	requireBridgeRateLimited429(t, rec)
	require.Equal(t, 1, repo.listCalls)
}

// 无任何 bridge 候选（not_configured）时必须落回 native /v1/messages 处理链，
// 且 native 重读到的 body 必须完好（哨兵 2：403 只有在 native 成功解析
// body 并取出被 BlockedModels 屏蔽的模型后才可能出现）。
func TestBridgeDispatch_MessagesNotConfiguredFallsThroughToNativeWithBodyIntact(t *testing.T) {
	repo := &bridgeDispatchAccountRepoStub{}
	router := newBridgeDispatchTestRouter(repo, true, []string{"claude-opus-4-8"})

	rec := performBridgeDispatchRequest(t, router, "/v1/messages", bridgeDispatchTestBody)

	require.Equal(t, http.StatusForbidden, rec.Code,
		"native handler must run and parse the replayed body to hit the group model gate")
	require.Equal(t, "permission_error", gjson.Get(rec.Body.String(), "error.type").String())
	// 反证保护：若 body 在 bridge 预检后丢失，native 会返回 400 空 body/解析失败。
	require.NotContains(t, rec.Body.String(), "Request body is empty")
	require.NotContains(t, rec.Body.String(), "Failed to parse request body")
	require.Equal(t, 1, repo.listCalls, "bridge preflight still probes the group once before native fallback")
}

// count_tokens：bridge 意图已确立（存在显式映射）但唯一账号限流时，
// 必须走本地 tiktoken 估算返回 200 input_tokens —— 不触达上游、
// 不落 native（哨兵 1）。
func TestBridgeDispatch_CountTokensRateLimitedUsesLocalEstimateWithoutNative(t *testing.T) {
	resetAt := time.Now().Add(90 * time.Second)
	repo := &bridgeDispatchAccountRepoStub{accounts: []service.Account{
		newBridgeDispatchTestAccount(1, func(a *service.Account) {
			a.RateLimitResetAt = &resetAt
		}),
	}}
	router := newBridgeDispatchTestRouter(repo, false, nil)

	rec := performBridgeDispatchRequest(t, router, "/v1/messages/count_tokens", bridgeDispatchTestBody)

	require.Equal(t, http.StatusOK, rec.Code)
	inputTokens := gjson.Get(rec.Body.String(), "input_tokens")
	require.True(t, inputTokens.Exists(), "local estimate must return input_tokens, got: %s", rec.Body.String())
	require.GreaterOrEqual(t, int(inputTokens.Int()), 1)
	requireSingleJSONWithoutNativeSentinel(t, rec)
}

// count_tokens：分组没有任何 bridge 候选时，CountTokensClaudeGPTBridge 返回
// false，必须落回 native CountTokens（哨兵 2 生效，同样证明 body 完好）。
func TestBridgeDispatch_CountTokensNotConfiguredFallsThroughToNative(t *testing.T) {
	repo := &bridgeDispatchAccountRepoStub{}
	router := newBridgeDispatchTestRouter(repo, true, []string{"claude-opus-4-8"})

	rec := performBridgeDispatchRequest(t, router, "/v1/messages/count_tokens", bridgeDispatchTestBody)

	require.Equal(t, http.StatusForbidden, rec.Code,
		"native CountTokens must run and parse the replayed body to hit the group model gate")
	require.Equal(t, "permission_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.NotContains(t, rec.Body.String(), "Request body is empty")
	require.NotContains(t, rec.Body.String(), "Failed to parse request body")
}
