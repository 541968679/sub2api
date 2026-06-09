package handler

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type publicUsageAlignmentRepo struct {
	service.UsageLogRepository
	logs []service.UsageLog
}

func (r *publicUsageAlignmentRepo) ListWithFilters(_ context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	filtered := make([]service.UsageLog, 0, len(r.logs))
	for _, log := range r.logs {
		if filters.UserID > 0 && log.UserID != filters.UserID {
			continue
		}
		if filters.APIKeyID > 0 && log.APIKeyID != filters.APIKeyID {
			continue
		}
		if filters.StartTime != nil && log.CreatedAt.Before(*filters.StartTime) {
			continue
		}
		if filters.EndTime != nil && !log.CreatedAt.Before(*filters.EndTime) {
			continue
		}
		filtered = append(filtered, log)
	}

	limit := params.Limit()
	offset := params.Offset()
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	pages := 0
	if limit > 0 {
		pages = int(math.Ceil(float64(len(filtered)) / float64(limit)))
	}
	return filtered[offset:end], &pagination.PaginationResult{
		Total:    int64(len(filtered)),
		Page:     params.Page,
		PageSize: limit,
		Pages:    pages,
	}, nil
}

func (r *publicUsageAlignmentRepo) GetAPIKeyStatsAggregated(_ context.Context, apiKeyID int64, startTime, endTime time.Time) (*usagestats.UsageStats, error) {
	var stats usagestats.UsageStats
	for _, log := range r.logs {
		if log.APIKeyID != apiKeyID || log.CreatedAt.Before(startTime) || !log.CreatedAt.Before(endTime) {
			continue
		}
		stats.TotalRequests++
		stats.TotalInputTokens += int64(log.InputTokens)
		stats.TotalOutputTokens += int64(log.OutputTokens)
		stats.TotalCacheTokens += int64(log.CacheCreationTokens + log.CacheReadTokens)
		stats.TotalCost += log.TotalCost
		stats.TotalActualCost += log.ActualCost
		if log.DurationMs != nil {
			stats.AverageDurationMs += float64(*log.DurationMs)
		}
	}
	stats.TotalTokens = stats.TotalInputTokens + stats.TotalOutputTokens + stats.TotalCacheTokens
	if stats.TotalRequests > 0 {
		stats.AverageDurationMs /= float64(stats.TotalRequests)
	}
	return &stats, nil
}

func (r *publicUsageAlignmentRepo) GetUsageTrendWithFilters(
	_ context.Context,
	startTime, endTime time.Time,
	granularity string,
	userID, apiKeyID, accountID, groupID int64,
	model string,
	requestType *int16,
	stream *bool,
	billingType *int8,
) ([]usagestats.TrendDataPoint, error) {
	stats, err := r.GetAPIKeyStatsAggregated(context.Background(), apiKeyID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	date := startTime.Format("2006-01-02")
	if granularity == "hour" {
		date = startTime.Format("2006-01-02 15:00")
	}
	return []usagestats.TrendDataPoint{{
		Date:                date,
		Requests:            stats.TotalRequests,
		InputTokens:         stats.TotalInputTokens,
		OutputTokens:        stats.TotalOutputTokens,
		CacheCreationTokens: 0,
		CacheReadTokens:     stats.TotalCacheTokens,
		TotalTokens:         stats.TotalTokens,
		Cost:                stats.TotalCost,
		ActualCost:          stats.TotalActualCost,
	}}, nil
}

type publicUsageAlignmentUserPricingRepo struct {
	service.UserModelPricingRepository
	overrides []service.UserModelPricingOverride
}

func (r *publicUsageAlignmentUserPricingRepo) GetEnabledByUserID(_ context.Context, userID int64) ([]service.UserModelPricingOverride, error) {
	out := make([]service.UserModelPricingOverride, 0, len(r.overrides))
	for _, override := range r.overrides {
		if override.UserID == userID && override.Enabled {
			out = append(out, override)
		}
	}
	return out, nil
}

type publicUsageAlignmentGroupRateRepo struct {
	service.UserGroupRateRepository
	rates map[int64]map[int64]service.UserGroupRateData
}

func (r *publicUsageAlignmentGroupRateRepo) GetFullByUserID(_ context.Context, userID int64) (map[int64]service.UserGroupRateData, error) {
	return r.rates[userID], nil
}

func newPublicUsageAlignmentRouter(repo *publicUsageAlignmentRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	inputDisplayA := 0.000001
	inputDisplayB := 0.0000005
	outputDisplay := 0.000002
	displayRate := 1.0
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	apiKeySvc := service.NewAPIKeyService(nil, nil, nil, nil, &publicUsageAlignmentGroupRateRepo{
		rates: map[int64]map[int64]service.UserGroupRateData{
			42: {
				10: {DisplayRateMultiplier: &displayRate},
			},
		},
	}, nil, nil)
	userPricingSvc := service.NewUserModelPricingService(&publicUsageAlignmentUserPricingRepo{
		overrides: []service.UserModelPricingOverride{
			{
				UserID:             42,
				Model:              "model-a",
				DisplayInputPrice:  &inputDisplayA,
				DisplayOutputPrice: &outputDisplay,
				Enabled:            true,
			},
			{
				UserID:            42,
				Model:             "model-b",
				DisplayInputPrice: &inputDisplayB,
				Enabled:           true,
			},
		},
	})
	handler := NewUsageHandler(usageSvc, apiKeySvc, nil, userPricingSvc, nil, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 7, UserID: 42})
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/usage/records", handler.PublicRecords)
	router.GET("/usage/stats", handler.PublicStats)
	router.GET("/usage/trend", handler.PublicTrend)
	return router
}

func TestPublicUsageStatsAndTrendUseSameDisplayPricingAsRecords(t *testing.T) {
	groupID := int64(10)
	durationA := 100
	durationB := 300
	createdAt := time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC)
	repo := &publicUsageAlignmentRepo{logs: []service.UsageLog{
		{
			ID:             1,
			UserID:         42,
			APIKeyID:       7,
			Model:          "model-a",
			GroupID:        &groupID,
			InputTokens:    1000,
			OutputTokens:   500,
			InputCost:      0.004,
			OutputCost:     0.006,
			TotalCost:      0.010,
			ActualCost:     0.020,
			RateMultiplier: 2,
			DurationMs:     &durationA,
			CreatedAt:      createdAt,
		},
		{
			ID:             2,
			UserID:         42,
			APIKeyID:       7,
			Model:          "model-b",
			InputTokens:    1000,
			InputCost:      0.001,
			TotalCost:      0.001,
			ActualCost:     0.001,
			RateMultiplier: 1,
			DurationMs:     &durationB,
			CreatedAt:      createdAt.Add(2 * time.Hour),
		},
	}}
	router := newPublicUsageAlignmentRouter(repo)
	query := "?start_date=2026-06-01&end_date=2026-06-01&timezone=UTC"

	records := getPublicUsageRecords(t, router, "/usage/records"+query+"&page_size=1000")
	stats := getPublicUsageStats(t, router, "/usage/stats"+query)
	trend := getPublicUsageTrend(t, router, "/usage/trend"+query+"&granularity=day")

	var recordActualCost, recordCost float64
	var recordInputTokens, recordOutputTokens, recordTotalTokens int64
	for _, record := range records {
		recordActualCost += record.ActualCost
		recordCost += record.TotalCost
		recordInputTokens += int64(record.InputTokens)
		recordOutputTokens += int64(record.OutputTokens)
		recordTotalTokens += int64(record.InputTokens + record.OutputTokens + record.CacheCreationTokens + record.CacheReadTokens)
	}
	var trendActualCost, trendCost float64
	var trendTotalTokens int64
	for _, point := range trend {
		trendActualCost += point.ActualCost
		trendCost += point.Cost
		trendTotalTokens += point.TotalTokens
	}

	require.InDelta(t, recordActualCost, stats.TotalActualCost, 1e-9)
	require.InDelta(t, recordCost, stats.TotalCost, 1e-9)
	require.Equal(t, recordInputTokens, stats.TotalInputTokens)
	require.Equal(t, recordOutputTokens, stats.TotalOutputTokens)
	require.Equal(t, recordTotalTokens, stats.TotalTokens)
	require.InDelta(t, stats.TotalActualCost, trendActualCost, 1e-9)
	require.InDelta(t, stats.TotalCost, trendCost, 1e-9)
	require.Equal(t, stats.TotalTokens, trendTotalTokens)

	require.InDelta(t, 0.021, stats.TotalCost, 1e-9)
	require.InDelta(t, 0.021, stats.TotalActualCost, 1e-9)
	require.Equal(t, int64(10000), stats.TotalInputTokens)
	require.Equal(t, int64(6000), stats.TotalOutputTokens)
	require.Equal(t, int64(16000), stats.TotalTokens)
	require.InDelta(t, 200, stats.AverageDurationMs, 1e-9)
}

type publicUsageEnvelope[T any] struct {
	Code int `json:"code"`
	Data T   `json:"data"`
}

type publicUsageRecordsData struct {
	Items []dto.UsageLog `json:"items"`
}

func getPublicUsageRecords(t *testing.T, router *gin.Engine, target string) []dto.UsageLog {
	t.Helper()
	var envelope publicUsageEnvelope[publicUsageRecordsData]
	serveJSON(t, router, target, &envelope)
	require.Equal(t, 0, envelope.Code)
	return envelope.Data.Items
}

func getPublicUsageStats(t *testing.T, router *gin.Engine, target string) service.UsageStats {
	t.Helper()
	var envelope publicUsageEnvelope[service.UsageStats]
	serveJSON(t, router, target, &envelope)
	require.Equal(t, 0, envelope.Code)
	return envelope.Data
}

type publicUsageTrendData struct {
	Trend []usagestats.TrendDataPoint `json:"trend"`
}

func getPublicUsageTrend(t *testing.T, router *gin.Engine, target string) []usagestats.TrendDataPoint {
	t.Helper()
	var envelope publicUsageEnvelope[publicUsageTrendData]
	serveJSON(t, router, target, &envelope)
	require.Equal(t, 0, envelope.Code)
	return envelope.Data.Trend
}

func serveJSON(t *testing.T, router *gin.Engine, target string, out any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), out))
}
