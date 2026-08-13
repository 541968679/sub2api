//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userQualityStatsRepoStub struct {
	service.UsageLogRepository
	lastIDs []int64
	result  map[int64]*service.AccountQualityStats
}

func (s *userQualityStatsRepoStub) GetUserQualityStatsBatch(_ context.Context, userIDs []int64, _ time.Time) (map[int64]*service.AccountQualityStats, error) {
	s.lastIDs = append([]int64(nil), userIDs...)
	if s.result == nil {
		return map[int64]*service.AccountQualityStats{}, nil
	}
	return s.result, nil
}

func postUserQualityStats(handler *UserHandler, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/quality-stats/batch", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	handler.GetBatchQualityStats(c)
	return recorder
}

func TestUserHandler_GetBatchQualityStats_EmptyIDs(t *testing.T) {
	userQualityStatsBatchCache = newSnapshotCache(30 * time.Second)
	handler := NewUserHandler(newStubAdminService(), nil)

	recorder := postUserQualityStats(handler, `{"user_ids":[0,-1]}`)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Stats map[string]any `json:"stats"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Empty(t, resp.Data.Stats)
}

func TestUserHandler_GetBatchQualityStats_UnavailableWithoutUsageService(t *testing.T) {
	userQualityStatsBatchCache = newSnapshotCache(30 * time.Second)
	handler := NewUserHandler(newStubAdminService(), nil)

	recorder := postUserQualityStats(handler, `{"user_ids":[1]}`)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestUserHandler_GetBatchQualityStats_CapAndDedupViaService(t *testing.T) {
	userQualityStatsBatchCache = newSnapshotCache(30 * time.Second)
	repo := &userQualityStatsRepoStub{result: map[int64]*service.AccountQualityStats{}}
	usageSvc := service.NewAccountUsageService(nil, repo, nil, nil, nil, nil, nil, nil, nil)
	handler := NewUserHandler(newStubAdminService(), nil, usageSvc)

	ids := make([]int64, 0, service.AccountQualityMaxBatchSize+50)
	ids = append(ids, 1, 1, 0)
	for i := int64(2); i <= int64(service.AccountQualityMaxBatchSize+50); i++ {
		ids = append(ids, i)
	}
	body, err := json.Marshal(map[string]any{"user_ids": ids})
	require.NoError(t, err)

	recorder := postUserQualityStats(handler, string(body))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, repo.lastIDs, service.AccountQualityMaxBatchSize)

	var resp struct {
		Data struct {
			Stats map[string]any `json:"stats"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Len(t, resp.Data.Stats, service.AccountQualityMaxBatchSize)
}
