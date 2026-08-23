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

type userLastNCacheStub struct {
	byID       map[int64]*service.AccountQualityLastN
	batchCalls [][]int64
}

func (s *userLastNCacheStub) GetUserLastN(_ context.Context, userID int64) *service.AccountQualityLastN {
	if s == nil || s.byID == nil {
		return nil
	}
	return s.byID[userID]
}

func (s *userLastNCacheStub) GetUserLastNBatch(_ context.Context, userIDs []int64) map[int64]*service.AccountQualityLastN {
	s.batchCalls = append(s.batchCalls, append([]int64(nil), userIDs...))
	out := map[int64]*service.AccountQualityLastN{}
	if s == nil || s.byID == nil {
		return out
	}
	for _, id := range userIDs {
		if live := s.byID[id]; live != nil {
			out[id] = live
		}
	}
	return out
}

func (s *userLastNCacheStub) IngestUserLastN(_ context.Context, userID int64, n int, success bool, firstTokenMs *int, useFailover bool, override *int) *service.AccountQualityLastN {
	if s.byID == nil {
		s.byID = map[int64]*service.AccountQualityLastN{}
	}
	live := service.ApplyAccountQualityLastNIngest(s.byID[userID], n, success, firstTokenMs)
	live.UseFailover = useFailover
	live.OverrideN = service.CopyIntPtr(override)
	s.byID[userID] = live
	return live
}

func (s *userLastNCacheStub) ResizeUserLastN(_ context.Context, userID int64, n int, override *int) *service.AccountQualityLastN {
	if s.byID == nil {
		s.byID = map[int64]*service.AccountQualityLastN{}
	}
	live := service.ProjectAccountQualityLastN(s.byID[userID], n)
	live.OverrideN = service.CopyIntPtr(override)
	s.byID[userID] = live
	return live
}

func (s *userLastNCacheStub) ListUserLastNIDs(_ context.Context) []int64 {
	return nil
}

type userQualityHistoryRepoStub struct{}

func (userQualityHistoryRepoStub) Upsert(context.Context, service.UserQualitySnapshotRow) error {
	return nil
}

func (userQualityHistoryRepoStub) ListByUser(context.Context, int64, time.Time, time.Time) ([]service.UserQualitySnapshotRow, error) {
	return []service.UserQualitySnapshotRow{}, nil
}

func (userQualityHistoryRepoStub) DeleteExpired(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

type usageSQLMustNotRun struct {
	service.UsageLogRepository
	called bool
}

func (s *usageSQLMustNotRun) GetUserQualityStatsBatch(context.Context, []int64, time.Time) (map[int64]*service.AccountQualityStats, error) {
	s.called = true
	return nil, nil
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

func getUserQualityHistory(handler *UserHandler, target string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "16"}}
	handler.GetQualityHistory(c)
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

func TestUserHandler_GetBatchQualityStats_UnavailableWithoutMaintenance(t *testing.T) {
	userQualityStatsBatchCache = newSnapshotCache(30 * time.Second)
	handler := NewUserHandler(newStubAdminService(), nil)

	recorder := postUserQualityStats(handler, `{"user_ids":[1]}`)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestUserHandler_GetBatchQualityStats_LastNNotFifteenMinuteSQL(t *testing.T) {
	userQualityStatsBatchCache = newSnapshotCache(30 * time.Second)
	sqlRepo := &usageSQLMustNotRun{}
	usageSvc := service.NewAccountUsageService(nil, sqlRepo, nil, nil, nil, nil, nil, nil, nil)
	lastN := &userLastNCacheStub{}
	lastN.IngestUserLastN(context.Background(), 16, service.DefaultAccountQualityWindowN, false, nil, true, nil)
	svc := service.NewAccountQualityMaintenanceService(&qualityHistoryRepoStub{}, nil, nil)
	svc.SetUserLastNCache(lastN)
	handler := NewUserHandler(newStubAdminService(), nil, usageSvc)
	handler.SetQualityMaintenance(svc)

	recorder := postUserQualityStats(handler, `{"user_ids":[16,16,0,17]}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.False(t, sqlRepo.called)
	require.Len(t, lastN.batchCalls, 1)
	require.Equal(t, []int64{16, 17}, lastN.batchCalls[0])

	var resp struct {
		Data struct {
			Stats map[string]service.AccountQualityStats `json:"stats"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Len(t, resp.Data.Stats, 2)
	got := resp.Data.Stats["16"]
	require.Equal(t, int64(1), got.ErrorCount)
	require.Equal(t, int64(1), got.FailoverErrorCount)
	require.Equal(t, service.DefaultAccountQualityWindowN, got.N)
	require.Equal(t, service.DefaultAccountQualityWindowN, got.WindowN)
	require.Equal(t, service.DefaultAccountQualityWindowN, got.AccountQualityWindowN)
	require.Equal(t, service.DefaultAccountQualityWindowN, resp.Data.Stats["17"].AccountQualityWindowN)
}

func TestUserHandler_GetUserQualityHistory_InvalidUserID(t *testing.T) {
	svc := service.NewAccountQualityMaintenanceService(&qualityHistoryRepoStub{}, nil, nil)
	svc.SetUserSnapshotRepo(userQualityHistoryRepoStub{})
	handler := NewUserHandler(newStubAdminService(), nil)
	handler.SetQualityMaintenance(svc)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/0/quality-history", nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "0"}}
	handler.GetQualityHistory(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUserHandler_GetUserQualityHistory_InvalidRange(t *testing.T) {
	svc := service.NewAccountQualityMaintenanceService(&qualityHistoryRepoStub{}, nil, nil)
	svc.SetUserSnapshotRepo(userQualityHistoryRepoStub{})
	handler := NewUserHandler(newStubAdminService(), nil)
	handler.SetQualityMaintenance(svc)

	to := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	from := to.Add(-8 * 24 * time.Hour)
	recorder := getUserQualityHistory(handler, "/api/v1/admin/users/16/quality-history?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestUserHandler_GetUserQualityHistory_EmptyHistory(t *testing.T) {
	svc := service.NewAccountQualityMaintenanceService(&qualityHistoryRepoStub{}, nil, nil)
	svc.SetUserSnapshotRepo(userQualityHistoryRepoStub{})
	handler := NewUserHandler(newStubAdminService(), nil)
	handler.SetQualityMaintenance(svc)

	recorder := getUserQualityHistory(handler, "/api/v1/admin/users/16/quality-history")
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data struct {
			Items []service.AccountQualityHistoryItem `json:"items"`
			From  string                              `json:"from"`
			To    string                              `json:"to"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Empty(t, body.Data.Items)

	from, err := time.Parse(time.RFC3339, body.Data.From)
	require.NoError(t, err)
	to, err := time.Parse(time.RFC3339, body.Data.To)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().UTC(), to, 2*time.Second)
	require.WithinDuration(t, to.Add(-service.AccountQualityHistoryDefaultRange), from, time.Second)
}
