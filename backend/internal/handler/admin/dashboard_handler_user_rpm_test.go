package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dashboardUserRPMRepo struct {
	service.UsageLogRepository
	calls int
	stats map[int64]*usagestats.BatchUserRPMStats
}

func (r *dashboardUserRPMRepo) GetBatchUserRPMStats(_ context.Context, userIDs []int64) (map[int64]*usagestats.BatchUserRPMStats, error) {
	r.calls++
	out := make(map[int64]*usagestats.BatchUserRPMStats, len(userIDs))
	for _, id := range userIDs {
		if stat, ok := r.stats[id]; ok {
			out[id] = stat
			continue
		}
		out[id] = &usagestats.BatchUserRPMStats{UserID: id, WindowSeconds: int(usagestats.UserRPMWindow.Seconds())}
	}
	return out, nil
}

func TestDashboardHandler_GetBatchUsersRPM(t *testing.T) {
	dashboardBatchUsersRPMCache = newSnapshotCache(5 * time.Second)
	t.Cleanup(func() {
		dashboardBatchUsersRPMCache = newSnapshotCache(5 * time.Second)
	})

	gin.SetMode(gin.TestMode)
	repo := &dashboardUserRPMRepo{
		stats: map[int64]*usagestats.BatchUserRPMStats{
			42: {UserID: 42, RPM: 17, WindowSeconds: 60},
		},
	}
	handler := NewDashboardHandler(service.NewDashboardService(repo, nil, nil, nil), nil)
	router := gin.New()
	router.POST("/admin/dashboard/users-rpm", handler.GetBatchUsersRPM)

	body := bytes.NewBufferString(`{"user_ids":[42,7]}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/dashboard/users-rpm", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "miss", rec.Header().Get("X-Snapshot-Cache"))
	var payload struct {
		Data struct {
			Stats map[string]usagestats.BatchUserRPMStats `json:"stats"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, int64(17), payload.Data.Stats["42"].RPM)
	require.Equal(t, int64(0), payload.Data.Stats["7"].RPM)
	require.Equal(t, 60, payload.Data.Stats["42"].WindowSeconds)
	require.Equal(t, 1, repo.calls)

	req2 := httptest.NewRequest(http.MethodPost, "/admin/dashboard/users-rpm", bytes.NewBufferString(`{"user_ids":[42,7]}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
	require.Equal(t, "hit", rec2.Header().Get("X-Snapshot-Cache"))
	require.Equal(t, 1, repo.calls)
}
