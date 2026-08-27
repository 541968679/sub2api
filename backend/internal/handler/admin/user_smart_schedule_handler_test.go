//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func newSmartScheduleJSONContext(method, body string, params []gin.Param) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = params
	return c, w
}

func TestPutSmartScheduleRequest_OmittedSoftCooldownIsHard(t *testing.T) {
	var omitted putSmartScheduleRequest
	if err := json.Unmarshal([]byte(`{"enabled":true,"cooldown_minutes":15}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.SoftCooldown {
		t.Fatal("omitted soft_cooldown must unmarshal as hard")
	}
	var on putSmartScheduleRequest
	if err := json.Unmarshal([]byte(`{"enabled":true,"cooldown_minutes":15,"soft_cooldown":true}`), &on); err != nil {
		t.Fatal(err)
	}
	if !on.SoftCooldown {
		t.Fatal("soft_cooldown true must round-trip")
	}
}

func TestPutSmartScheduleRequest_OmittedProbeLatencyV2IsOff(t *testing.T) {
	var omitted putSmartScheduleRequest
	if err := json.Unmarshal([]byte(`{"enabled":true,"cooldown_minutes":15}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.ProbeLatencyV2 {
		t.Fatal("omitted probe_latency_v2 must unmarshal as off")
	}
	var on putSmartScheduleRequest
	if err := json.Unmarshal([]byte(`{"enabled":true,"cooldown_minutes":15,"probe_latency_v2":true}`), &on); err != nil {
		t.Fatal(err)
	}
	if !on.ProbeLatencyV2 {
		t.Fatal("probe_latency_v2 true must round-trip")
	}
}

func TestGetUserSmartSchedule_InvalidID(t *testing.T) {
	h := &UserHandler{adminService: newStubAdminService()}
	c, w := newSmartScheduleJSONContext(http.MethodGet, "", []gin.Param{{Key: "id", Value: "abc"}})
	h.GetUserSmartSchedule(c)
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("invalid id should be 4xx, got %d", w.Code)
	}
}

func TestGetUserSmartSchedule_ReturnsPlatforms(t *testing.T) {
	repo := &serviceSmartRepoStub{bundle: &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}}
	svc := service.NewUserSmartScheduleService(repo, nil, nil, nil, nil)
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: svc}
	c, w := newSmartScheduleJSONContext(http.MethodGet, "", []gin.Param{{Key: "id", Value: "16"}})
	h.GetUserSmartSchedule(c)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	data, _ := body["data"].(map[string]any)
	platforms, _ := data["platforms"].(map[string]any)
	if _, ok := platforms["anthropic"]; !ok {
		t.Fatalf("missing anthropic platform: %v", data)
	}
}

func TestUpdateUserSmartSchedule_EnabledEmptyPool(t *testing.T) {
	svc := service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: svc}
	c, w := newSmartScheduleJSONContext(http.MethodPut, `{"enabled":true,"cooldown_minutes":15,"accounts":[]}`, []gin.Param{
		{Key: "id", Value: "16"},
		{Key: "platform", Value: "anthropic"},
	})
	h.UpdateUserSmartSchedule(c)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SMART_SCHEDULE_EMPTY_POOL") {
		t.Fatalf("expected empty-pool reason, got %s", w.Body.String())
	}
}

func TestCopyUserSmartSchedule_RequiresFromPlatform(t *testing.T) {
	svc := service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: svc}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{}`, []gin.Param{
		{Key: "id", Value: "16"},
		{Key: "platform", Value: "openai"},
	})
	h.CopyUserSmartSchedule(c)
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("missing from_platform should be 4xx, got %d", w.Code)
	}
}

func TestGetBatchSmartScheduleSummaries_EmptyIDs(t *testing.T) {
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"user_ids":[]}`, nil)
	h.GetBatchSmartScheduleSummaries(c)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"summaries"`) {
		t.Fatalf("expected summaries payload, got %s", w.Body.String())
	}
}

func TestGetBatchSmartScheduleSummaries_ReturnsEnabledPlatforms(t *testing.T) {
	repo := &serviceSmartRepoStub{bundle: &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		"openai": {
			Enabled:         true,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{7: {}, 8: {}},
		},
	}}}
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(repo, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"user_ids":[16]}`, nil)
	h.GetBatchSmartScheduleSummaries(c)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"openai"`) {
		t.Fatalf("expected openai summary, got %s", w.Body.String())
	}
}

func TestPatchUserSmartScheduleSortOrder_InvalidID(t *testing.T) {
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPatch, `{"accounts":[{"account_id":11,"sort_order":1}]}`, []gin.Param{
		{Key: "id", Value: "abc"},
		{Key: "platform", Value: "anthropic"},
	})
	h.PatchUserSmartScheduleSortOrder(c)
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("invalid id should be 4xx, got %d", w.Code)
	}
}

func TestResumeSmartSchedule_RequiresUserID(t *testing.T) {
	h := &AccountHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{}`, []gin.Param{{Key: "id", Value: "7"}})
	h.ResumeSmartSchedule(c)
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("missing user_id should be 4xx, got %d", w.Code)
	}
}

func TestResumeSmartSchedule_Paused(t *testing.T) {
	repo := &serviceSmartRepoStub{bundle: &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		service.PlatformAnthropic: {
			Enabled:    true,
			AccountIDs: map[int64]struct{}{7: {}},
		},
	}}}
	h := &AccountHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(repo, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"user_id":16,"state":"paused"}`, []gin.Param{{Key: "id", Value: "7"}})
	h.ResumeSmartSchedule(c)
	if w.Code != http.StatusOK {
		t.Fatalf("paused should be 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"state":"paused"`) {
		t.Fatalf("expected paused state, got %s", w.Body.String())
	}
	if !repo.bundle.Policies[service.PlatformAnthropic].IsPaused(7) {
		t.Fatal("expected member 7 to be paused")
	}
}

func TestResumeSmartSchedule_InvalidState(t *testing.T) {
	h := &AccountHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"user_id":16,"state":"nope"}`, []gin.Param{{Key: "id", Value: "7"}})
	h.ResumeSmartSchedule(c)
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("invalid state should be 4xx, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SMART_SCHEDULE_ADMISSION_INVALID") {
		t.Fatalf("expected admission invalid reason, got %s", w.Body.String())
	}
}

type serviceSmartRepoStub struct {
	bundle *service.UserSmartScheduleBundle
}

func (s *serviceSmartRepoStub) ListByUser(_ context.Context, _ int64) (*service.UserSmartScheduleBundle, error) {
	if s == nil || s.bundle == nil {
		return &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}, nil
	}
	return s.bundle, nil
}

func (s *serviceSmartRepoStub) ListByUsers(_ context.Context, userIDs []int64) (map[int64]*service.UserSmartScheduleBundle, error) {
	out := make(map[int64]*service.UserSmartScheduleBundle, len(userIDs))
	for _, userID := range userIDs {
		bundle, _ := s.ListByUser(context.Background(), userID)
		out[userID] = bundle
	}
	return out, nil
}

func (s *serviceSmartRepoStub) ReplacePlatform(_ context.Context, _ int64, _ string, _ service.SmartSchedulePlatformWrite) error {
	return nil
}

func (s *serviceSmartRepoStub) UpdateSortOrders(_ context.Context, _ int64, _ string, _ []service.SmartScheduleSortAssignment) error {
	return nil
}

func (s *serviceSmartRepoStub) SetMemberPaused(_ context.Context, _ int64, accountID int64, _ string, paused bool) error {
	if s == nil || s.bundle == nil || s.bundle.Policies == nil {
		return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	for _, policy := range s.bundle.Policies {
		if policy == nil || !policy.HasAccount(accountID) {
			continue
		}
		if policy.Paused == nil {
			policy.Paused = map[int64]struct{}{}
		}
		if paused {
			policy.Paused[accountID] = struct{}{}
		} else {
			delete(policy.Paused, accountID)
		}
		return nil
	}
	return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
}
