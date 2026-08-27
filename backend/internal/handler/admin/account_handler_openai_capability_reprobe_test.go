//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type capabilityReprobeHTTPUpstream struct {
	requests []*http.Request
}

func (u *capabilityReprobeHTTPUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	return nil, fmt.Errorf("unexpected Do call")
}

func (u *capabilityReprobeHTTPUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.requests = append(u.requests, req)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"output":[{"type":"function_call","name":"probe_ping"}],"choices":[{"index":0}]}`)),
	}, nil
}

type capabilityReprobeAccountRepo struct {
	listed []service.Account
	byID   map[int64]*service.Account
}

func (r *capabilityReprobeAccountRepo) ListAllWithFilters(context.Context, string, string, string, string, int64, string) ([]service.Account, error) {
	return r.listed, nil
}

func (r *capabilityReprobeAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	if acc, ok := r.byID[id]; ok {
		return acc, nil
	}
	return nil, fmt.Errorf("account not found")
}

func (r *capabilityReprobeAccountRepo) GetByIDs(context.Context, []int64) ([]*service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) Create(context.Context, *service.Account) error { return nil }
func (r *capabilityReprobeAccountRepo) ExistsByID(context.Context, int64) (bool, error) {
	return false, nil
}
func (r *capabilityReprobeAccountRepo) GetByCRSAccountID(context.Context, string) (*service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) FindByExtraField(context.Context, string, any) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) Update(context.Context, *service.Account) error { return nil }
func (r *capabilityReprobeAccountRepo) Delete(context.Context, int64) error            { return nil }
func (r *capabilityReprobeAccountRepo) List(context.Context, pagination.PaginationParams) ([]service.Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *capabilityReprobeAccountRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]service.Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *capabilityReprobeAccountRepo) ListByGroup(context.Context, int64) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListActive(context.Context) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListByPlatform(context.Context, string) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) UpdateLastUsed(context.Context, int64) error { return nil }
func (r *capabilityReprobeAccountRepo) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) SetError(context.Context, int64, string) error { return nil }
func (r *capabilityReprobeAccountRepo) ClearError(context.Context, int64) error       { return nil }
func (r *capabilityReprobeAccountRepo) SetSchedulable(context.Context, int64, bool) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *capabilityReprobeAccountRepo) BindGroups(context.Context, int64, []int64) error { return nil }
func (r *capabilityReprobeAccountRepo) SyncScheduleUsers(context.Context, int64, service.AccountUserScheduleWrite) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) ListScheduleUserRefs(context.Context, []int64) ([]service.ScheduleUserRef, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListSchedulable(context.Context) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListSchedulableByGroupID(context.Context, int64) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListSchedulableByPlatform(context.Context, string) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListSchedulableByPlatforms(context.Context, []string) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListSchedulableUngroupedByPlatform(context.Context, string) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]service.Account, error) {
	return nil, nil
}
func (r *capabilityReprobeAccountRepo) SetRateLimited(context.Context, int64, time.Time) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) SetModelRateLimit(context.Context, int64, string, time.Time) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) SetOverloaded(context.Context, int64, time.Time) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) SetTempUnschedulable(context.Context, int64, time.Time, string) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) ClearTempUnschedulable(context.Context, int64) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) ClearRateLimit(context.Context, int64) error { return nil }
func (r *capabilityReprobeAccountRepo) ClearAntigravityQuotaScopes(context.Context, int64) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) ClearModelRateLimits(context.Context, int64) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) UpdateSessionWindowEnd(context.Context, int64, time.Time) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	acc, ok := r.byID[id]
	if !ok || acc == nil {
		return nil
	}
	if acc.Extra == nil {
		acc.Extra = map[string]any{}
	}
	for k, v := range updates {
		acc.Extra[k] = v
	}
	return nil
}
func (r *capabilityReprobeAccountRepo) BulkUpdate(context.Context, []int64, service.AccountBulkUpdate) (int64, error) {
	return 0, nil
}
func (r *capabilityReprobeAccountRepo) IncrementQuotaUsed(context.Context, int64, float64) error {
	return nil
}
func (r *capabilityReprobeAccountRepo) ResetQuotaUsed(context.Context, int64) error { return nil }

func zhimaShapedHandlerAccount() *service.Account {
	return &service.Account{
		ID:       1732,
		Name:     "zhima",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
			"model_mapping": map[string]any{
				"audio":   "gpt-4o-audio-preview",
				"default": openai.DefaultTestModel,
			},
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported:       false,
			openai_compat.ExtraKeyChatCompletionsSupported: false,
		},
	}
}

func tokenbitsShapedHandlerAccount() *service.Account {
	return &service.Account{
		ID:       12,
		Name:     "tokenbits",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
			"model_mapping": map[string]any{
				"a": openai.DefaultTestModel,
			},
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported:       true,
			openai_compat.ExtraKeyChatCompletionsSupported: true,
		},
	}
}

func setupOpenAICapabilityReprobeHandler(t *testing.T, upstream *capabilityReprobeHTTPUpstream) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	zhima := zhimaShapedHandlerAccount()
	tokenbits := tokenbitsShapedHandlerAccount()
	repo := &capabilityReprobeAccountRepo{
		listed: []service.Account{*zhima, *tokenbits},
		byID: map[int64]*service.Account{
			zhima.ID:     zhima,
			tokenbits.ID: tokenbits,
		},
	}
	testSvc := service.NewAccountTestService(
		repo,
		nil,
		nil,
		nil,
		nil,
		upstream,
		&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
			Enabled:           false,
			AllowInsecureHTTP: true,
		}}},
		nil,
	)
	handler := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, testSvc, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/accounts/openai-capability-reprobe", handler.OpenAICapabilityReprobe)
	return router
}

func TestAccountHandler_OpenAICapabilityReprobe(t *testing.T) {
	t.Run("omitted dry_run defaults to list only", func(t *testing.T) {
		upstream := &capabilityReprobeHTTPUpstream{}
		router := setupOpenAICapabilityReprobeHandler(t, upstream)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-capability-reprobe", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, upstream.requests)

		var envelope map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		data := envelope["data"].(map[string]any)
		require.Equal(t, true, data["dry_run"])
		require.Equal(t, false, data["all_apikeys"])
		require.Equal(t, float64(1), data["count"])
		accounts := data["accounts"].([]any)
		require.Len(t, accounts, 1)
		require.Equal(t, float64(1732), accounts[0].(map[string]any)["account_id"])
		_, hasCreds := accounts[0].(map[string]any)["credentials"]
		require.False(t, hasCreds)
	})

	t.Run("all_apikeys dry-run lists every OpenAI API Key", func(t *testing.T) {
		upstream := &capabilityReprobeHTTPUpstream{}
		router := setupOpenAICapabilityReprobeHandler(t, upstream)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-capability-reprobe", bytes.NewReader([]byte(`{"all_apikeys":true}`)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, upstream.requests)

		var envelope map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		data := envelope["data"].(map[string]any)
		require.Equal(t, true, data["dry_run"])
		require.Equal(t, true, data["all_apikeys"])
		require.Equal(t, float64(2), data["count"])
	})

	t.Run("empty body defaults to dry-run", func(t *testing.T) {
		upstream := &capabilityReprobeHTTPUpstream{}
		router := setupOpenAICapabilityReprobeHandler(t, upstream)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-capability-reprobe", http.NoBody)
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Empty(t, upstream.requests)

		var envelope map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		data := envelope["data"].(map[string]any)
		require.Equal(t, true, data["dry_run"])
	})

	t.Run("execute all_apikeys probes every OpenAI API Key", func(t *testing.T) {
		upstream := &capabilityReprobeHTTPUpstream{}
		router := setupOpenAICapabilityReprobeHandler(t, upstream)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-capability-reprobe", bytes.NewReader([]byte(`{"dry_run":false,"all_apikeys":true}`)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, upstream.requests, 4)

		var envelope map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		data := envelope["data"].(map[string]any)
		require.Equal(t, false, data["dry_run"])
		require.Equal(t, true, data["all_apikeys"])
		require.Equal(t, float64(2), data["count"])
	})

	t.Run("execute probes only eligible ids", func(t *testing.T) {
		upstream := &capabilityReprobeHTTPUpstream{}
		router := setupOpenAICapabilityReprobeHandler(t, upstream)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/openai-capability-reprobe", bytes.NewReader([]byte(`{"dry_run":false}`)))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Len(t, upstream.requests, 2)
		require.Equal(t, "http://upstream.example/v1/responses", upstream.requests[0].URL.String())
		require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.requests[1].URL.String())
		for _, probeReq := range upstream.requests {
			body, err := io.ReadAll(probeReq.Body)
			require.NoError(t, err)
			require.Equal(t, openai.DefaultTestModel, gjson.GetBytes(body, "model").String())
		}
	})
}
