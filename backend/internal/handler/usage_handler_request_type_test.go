package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userUsageRepoCapture struct {
	service.UsageLogRepository
	listParams       pagination.PaginationParams
	listFilters      usagestats.UsageLogFilters
	trendUserID      int64
	trendAPIKeyID    int64
	trendGranularity string
}

func (s *userUsageRepoCapture) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	s.listParams = params
	s.listFilters = filters
	return []service.UsageLog{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    0,
	}, nil
}

func (s *userUsageRepoCapture) GetUserUsageTrendByUserID(ctx context.Context, userID int64, startTime, endTime time.Time, granularity string) ([]usagestats.TrendDataPoint, error) {
	s.trendUserID = userID
	s.trendGranularity = granularity
	return []usagestats.TrendDataPoint{}, nil
}

func (s *userUsageRepoCapture) GetUsageTrendWithFilters(
	ctx context.Context,
	startTime, endTime time.Time,
	granularity string,
	userID, apiKeyID, accountID, groupID int64,
	model string,
	requestType *int16,
	stream *bool,
	billingType *int8,
) ([]usagestats.TrendDataPoint, error) {
	s.trendUserID = userID
	s.trendAPIKeyID = apiKeyID
	s.trendGranularity = granularity
	return []usagestats.TrendDataPoint{}, nil
}

type userUsageAPIKeyRepoStub struct {
	service.APIKeyRepository
	apiKey *service.APIKey
	err    error
}

func (s *userUsageAPIKeyRepoStub) GetByID(ctx context.Context, id int64) (*service.APIKey, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.apiKey, nil
}

func newUserUsageRequestTypeTestRouter(repo *userUsageRepoCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage", handler.List)
	return router
}

func newUserUsageTrendTestRouter(repo *userUsageRepoCapture, apiKeyRepo service.APIKeyRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	apiKeySvc := service.NewAPIKeyService(apiKeyRepo, nil, nil, nil, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, apiKeySvc, nil, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/dashboard/trend", handler.DashboardTrend)
	return router
}

func TestUserUsageListRequestTypePriority(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=ws_v2&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.listFilters.UserID)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *repo.listFilters.RequestType)
	require.Nil(t, repo.listFilters.Stream)
}

func TestUserUsageListInvalidRequestType(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?request_type=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageDashboardTrendFiltersByAPIKey(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageTrendTestRouter(repo, &userUsageAPIKeyRepoStub{
		apiKey: &service.APIKey{ID: 7, UserID: 42},
	})

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/trend?api_key_id=7&granularity=hour&start_date=2026-05-20&end_date=2026-05-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	// The dashboard trend now aggregates from display-transformed usage records, so it
	// filters via ListWithFilters (user + selected API key) rather than the raw trend query.
	require.Equal(t, int64(42), repo.listFilters.UserID)
	require.Equal(t, int64(7), repo.listFilters.APIKeyID)
}

func TestUserUsageDashboardTrendInvalidAPIKeyID(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageTrendTestRouter(repo, &userUsageAPIKeyRepoStub{
		apiKey: &service.APIKey{ID: 7, UserID: 42},
	})

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/trend?api_key_id=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUserUsageDashboardTrendRejectsOtherUsersAPIKey(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageTrendTestRouter(repo, &userUsageAPIKeyRepoStub{
		apiKey: &service.APIKey{ID: 7, UserID: 99},
	})

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/trend?api_key_id=7", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUserUsageListInvalidStream(t *testing.T) {
	repo := &userUsageRepoCapture{}
	router := newUserUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/usage?stream=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
