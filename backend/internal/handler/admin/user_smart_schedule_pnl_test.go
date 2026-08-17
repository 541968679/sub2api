//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestGetBatchSmartSchedulePnlSummaries_EmptyIDs(t *testing.T) {
	h := &UserHandler{adminService: newStubAdminService(), schedulePnl: service.NewSchedulePnlService(nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"user_ids":[]}`, nil)
	h.GetBatchSmartSchedulePnlSummaries(c)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	data, _ := body["data"].(map[string]any)
	summaries, _ := data["summaries"].(map[string]any)
	if len(summaries) != 0 {
		t.Fatalf("empty ids should return empty map, got %v", summaries)
	}
}

func TestGetSmartSchedulePnlPairs_InvalidID(t *testing.T) {
	h := &UserHandler{adminService: newStubAdminService()}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"account_ids":[1]}`, []gin.Param{{Key: "id", Value: "abc"}})
	h.GetSmartSchedulePnlPairs(c)
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("invalid id should be 4xx, got %d", w.Code)
	}
}

func TestGetSmartSchedulePnlTrend_InvalidRange(t *testing.T) {
	svc := service.NewSchedulePnlService(nil, &serviceSmartRepoStub{bundle: &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}})
	h := &UserHandler{adminService: newStubAdminService(), schedulePnl: svc}
	c, w := newSmartScheduleJSONContext(http.MethodGet, "", []gin.Param{{Key: "id", Value: "16"}})
	c.Request.URL.RawQuery = "range=month"
	h.GetSmartSchedulePnlTrend(c)
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("invalid range should be 4xx, got %d: %s", w.Code, w.Body.String())
	}
}
