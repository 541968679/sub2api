//go:build unit

package admin

import (
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

func getQualityHistory(handler *AccountHandler, target string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "12"}}
	handler.GetQualityHistory(c)
	return recorder
}

func TestAccountHandler_GetQualityHistory_InvalidRange(t *testing.T) {
	svc := service.NewAccountQualityMaintenanceService(&qualityHistoryRepoStub{}, nil, nil)
	handler := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, svc)

	to := time.Date(2026, 8, 14, 15, 0, 0, 0, time.UTC)
	from := to.Add(-8 * 24 * time.Hour)
	recorder := getQualityHistory(handler, "/api/v1/admin/accounts/12/quality-history?from="+from.Format(time.RFC3339)+"&to="+to.Format(time.RFC3339))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAccountHandler_GetQualityHistory_EmptyHistory(t *testing.T) {
	svc := service.NewAccountQualityMaintenanceService(&qualityHistoryRepoStub{}, nil, nil)
	handler := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, svc)

	recorder := getQualityHistory(handler, "/api/v1/admin/accounts/12/quality-history")
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data struct {
			Items []service.AccountQualityHistoryItem `json:"items"`
			From  string                             `json:"from"`
			To    string                             `json:"to"`
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

type qualityHistoryRepoStub struct{}

func (qualityHistoryRepoStub) Upsert(context.Context, service.AccountQualitySnapshotRow) error {
	return nil
}

func (qualityHistoryRepoStub) ListByAccount(context.Context, int64, time.Time, time.Time) ([]service.AccountQualitySnapshotRow, error) {
	return []service.AccountQualitySnapshotRow{}, nil
}

func (qualityHistoryRepoStub) DeleteExpired(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (qualityHistoryRepoStub) ListRecentTrafficAccountIDs(context.Context, time.Time) ([]int64, error) {
	return nil, nil
}
