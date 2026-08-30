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

func TestPreviewCopyFromUserSmartSchedule_RequiresSourceUser(t *testing.T) {
	svc := service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: svc}
	c, w := newSmartScheduleJSONContext(http.MethodGet, "", []gin.Param{
		{Key: "id", Value: "99"},
		{Key: "platform", Value: "anthropic"},
	})
	h.PreviewCopyFromUserSmartSchedule(c)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SMART_SCHEDULE_COPY_INVALID") {
		t.Fatalf("expected copy-invalid, got %s", w.Body.String())
	}
}

func TestCopyFromUserSmartSchedule_SameUserRejected(t *testing.T) {
	svc := service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: svc}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"source_user_id":99,"source_revision":"x","slices":{"pool":true}}`, []gin.Param{
		{Key: "id", Value: "99"},
		{Key: "platform", Value: "anthropic"},
	})
	h.CopyFromUserSmartSchedule(c)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SMART_SCHEDULE_COPY_INVALID") {
		t.Fatalf("expected copy-invalid, got %s", w.Body.String())
	}
}

func TestCopyFromUserSmartSchedule_CopiesPool(t *testing.T) {
	source := &service.SmartSchedulePlatformPolicy{
		Enabled:         true,
		CooldownMinutes: 15,
		AccountIDs:      map[int64]struct{}{11: {}},
		Paused:          map[int64]struct{}{11: {}},
	}
	repo := &serviceSmartRepoStub{byUser: map[int64]*service.UserSmartScheduleBundle{
		16: {Policies: map[string]*service.SmartSchedulePlatformPolicy{service.PlatformAnthropic: source}},
		99: {Policies: map[string]*service.SmartSchedulePlatformPolicy{
			service.PlatformAnthropic: {Enabled: false, CooldownMinutes: 15, AccountIDs: map[int64]struct{}{}},
		}},
	}}
	svc := service.NewUserSmartScheduleService(repo, nil, &handlerSmartAccountStub{accounts: []*service.Account{
		{ID: 11, Platform: service.PlatformAnthropic},
	}}, nil, nil)
	h := &UserHandler{adminService: newStubAdminService(), smartSchedule: svc}
	c, w := newSmartScheduleJSONContext(http.MethodGet, "", []gin.Param{
		{Key: "id", Value: "99"},
		{Key: "platform", Value: "anthropic"},
	})
	c.Request.URL.RawQuery = "source_user_id=16"
	h.PreviewCopyFromUserSmartSchedule(c)
	if w.Code != 200 {
		t.Fatalf("preview expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var previewBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &previewBody); err != nil {
		t.Fatalf("preview json: %v", err)
	}
	data, _ := previewBody["data"].(map[string]any)
	revision, _ := data["source_revision"].(string)
	if revision == "" {
		t.Fatalf("expected source_revision, got %s", w.Body.String())
	}
	c, w = newSmartScheduleJSONContext(http.MethodPost, `{"source_user_id":16,"source_revision":"`+revision+`","slices":{"pool":true,"concurrency":true,"sort_order":true}}`, []gin.Param{
		{Key: "id", Value: "99"},
		{Key: "platform", Value: "anthropic"},
	})
	h.CopyFromUserSmartSchedule(c)
	if w.Code != 200 {
		t.Fatalf("copy expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"account_id":11`) {
		t.Fatalf("expected copied account, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"paused":true`) {
		t.Fatalf("expected source pause, got %s", w.Body.String())
	}
}

type handlerSmartAccountStub struct {
	accounts []*service.Account
}

func (s *handlerSmartAccountStub) GetByIDs(_ context.Context, ids []int64) ([]*service.Account, error) {
	byID := map[int64]*service.Account{}
	for _, acc := range s.accounts {
		if acc != nil {
			byID[acc.ID] = acc
		}
	}
	out := make([]*service.Account, 0, len(ids))
	for _, id := range ids {
		if acc := byID[id]; acc != nil {
			out = append(out, acc)
		}
	}
	return out, nil
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

func TestListSmartScheduleMemberships_ReturnsMembers(t *testing.T) {
	repo := &serviceSmartRepoStub{bundle: &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{
		service.PlatformAnthropic: {
			Enabled:    true,
			AccountIDs: map[int64]struct{}{7: {}},
		},
	}}}
	h := &AccountHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(repo, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodGet, "", []gin.Param{{Key: "id", Value: "7"}})
	c.Request.URL.RawQuery = "platform=anthropic"
	h.ListSmartScheduleMemberships(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"user_id":16`) {
		t.Fatalf("expected membership user, got %s", w.Body.String())
	}
}

func TestAddSmartScheduleMember_RequiresUserID(t *testing.T) {
	h := &AccountHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"platform":"anthropic"}`, []gin.Param{{Key: "id", Value: "7"}})
	h.AddSmartScheduleMember(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSetSmartScheduleAdmissionBatch_InvalidState(t *testing.T) {
	h := &AccountHandler{adminService: newStubAdminService(), smartSchedule: service.NewUserSmartScheduleService(&serviceSmartRepoStub{}, nil, nil, nil, nil)}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"platform":"anthropic","state":"nope"}`, []gin.Param{{Key: "id", Value: "7"}})
	h.SetSmartScheduleAdmissionBatch(c)
	if w.Code == http.StatusOK {
		t.Fatalf("expected invalid state, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "SMART_SCHEDULE_ADMISSION_INVALID") {
		t.Fatalf("expected admission invalid reason, got %s", w.Body.String())
	}
}

func TestSetPublicSchedulable_UpdatesAccount(t *testing.T) {
	h := &AccountHandler{adminService: newStubAdminService()}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{"public_schedulable":false}`, []gin.Param{{Key: "id", Value: "7"}})
	h.SetPublicSchedulable(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"public_schedulable":false`) {
		t.Fatalf("expected public_schedulable false, got %s", w.Body.String())
	}
}

func TestSetPublicSchedulable_RequiresField(t *testing.T) {
	h := &AccountHandler{adminService: newStubAdminService()}
	c, w := newSmartScheduleJSONContext(http.MethodPost, `{}`, []gin.Param{{Key: "id", Value: "7"}})
	h.SetPublicSchedulable(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
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
	byUser map[int64]*service.UserSmartScheduleBundle
}

func (s *serviceSmartRepoStub) ListByUser(_ context.Context, userID int64) (*service.UserSmartScheduleBundle, error) {
	if s == nil {
		return &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}, nil
	}
	if s.byUser != nil {
		if bundle := s.byUser[userID]; bundle != nil {
			return bundle, nil
		}
		return &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}, nil
	}
	if s.bundle == nil {
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

func (s *serviceSmartRepoStub) ReplacePlatformWithMemberPaused(_ context.Context, userID int64, platform string, policy service.SmartSchedulePlatformWrite) error {
	if s == nil {
		return nil
	}
	target := s.bundle
	if s.byUser != nil {
		if s.byUser[userID] == nil {
			s.byUser[userID] = &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}
		}
		target = s.byUser[userID]
	}
	if target == nil {
		target = &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}
		s.bundle = target
	}
	if target.Policies == nil {
		target.Policies = map[string]*service.SmartSchedulePlatformPolicy{}
	}
	next := &service.SmartSchedulePlatformPolicy{
		Enabled:         policy.Enabled,
		CooldownMinutes: policy.CooldownMinutes,
		AccountIDs:      map[int64]struct{}{},
		Caps:            map[int64]int{},
		SortOrders:      map[int64]int{},
		Paused:          map[int64]struct{}{},
	}
	for _, member := range policy.Accounts {
		next.AccountIDs[member.AccountID] = struct{}{}
		if member.MaxConcurrency != nil && *member.MaxConcurrency >= 1 {
			next.Caps[member.AccountID] = *member.MaxConcurrency
		}
		if member.SortOrder != nil {
			next.SortOrders[member.AccountID] = *member.SortOrder
		}
		if member.Paused {
			next.Paused[member.AccountID] = struct{}{}
		}
	}
	target.Policies[platform] = next
	return nil
}

func (s *serviceSmartRepoStub) UpdateSortOrders(_ context.Context, _ int64, _ string, _ []service.SmartScheduleSortAssignment) error {
	return nil
}

func (s *serviceSmartRepoStub) ListMembershipsByAccount(_ context.Context, _ int64, platform string) ([]service.SmartScheduleAccountMembership, error) {
	out := []service.SmartScheduleAccountMembership{}
	if s == nil || s.bundle == nil || s.bundle.Policies == nil {
		return out, nil
	}
	for plat, policy := range s.bundle.Policies {
		if policy == nil {
			continue
		}
		if platform != "" && plat != platform {
			continue
		}
		for accountID := range policy.AccountIDs {
			out = append(out, service.SmartScheduleAccountMembership{
				UserID:   16,
				Platform: plat,
				Enabled:  policy.Enabled,
				Paused:   policy.IsPaused(accountID),
			})
		}
	}
	return out, nil
}

func (s *serviceSmartRepoStub) AddMember(_ context.Context, _ int64, accountID int64, platform string) error {
	if s == nil {
		return nil
	}
	if s.bundle == nil {
		s.bundle = &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}
	}
	if s.bundle.Policies == nil {
		s.bundle.Policies = map[string]*service.SmartSchedulePlatformPolicy{}
	}
	policy := s.bundle.Policies[platform]
	if policy == nil {
		policy = &service.SmartSchedulePlatformPolicy{AccountIDs: map[int64]struct{}{}}
		s.bundle.Policies[platform] = policy
	}
	if policy.AccountIDs == nil {
		policy.AccountIDs = map[int64]struct{}{}
	}
	policy.AccountIDs[accountID] = struct{}{}
	return nil
}

func (s *serviceSmartRepoStub) RemoveMember(_ context.Context, _ int64, accountID int64, platform string) error {
	if s == nil || s.bundle == nil || s.bundle.Policies == nil {
		return nil
	}
	policy := s.bundle.Policies[platform]
	if policy == nil || policy.AccountIDs == nil {
		return nil
	}
	delete(policy.AccountIDs, accountID)
	if len(policy.AccountIDs) == 0 {
		policy.Enabled = false
	}
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
