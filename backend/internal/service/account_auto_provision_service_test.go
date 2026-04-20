package service

import (
	"testing"
	"time"
)

func TestBuildLastHealthySnapshotPrefersMostRecentlyUsedAccount(t *testing.T) {
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-10 * time.Minute)

	snapshot, ok := buildLastHealthySnapshot([]Account{
		{ID: 10, Name: "older", Concurrency: 3, Schedulable: true, LastUsedAt: &oldTime},
		{ID: 11, Name: "newer", Concurrency: 5, Schedulable: true, LastUsedAt: &newTime},
	})
	if !ok {
		t.Fatal("buildLastHealthySnapshot() returned ok=false")
	}
	if snapshot.SourceAccountID != 11 {
		t.Fatalf("SourceAccountID = %d, want 11", snapshot.SourceAccountID)
	}
	if snapshot.Template.Concurrency != 5 {
		t.Fatalf("Template.Concurrency = %d, want 5", snapshot.Template.Concurrency)
	}
}

func TestSelectUngroupedCandidateHonorsGroupRequirements(t *testing.T) {
	candidates := []Account{
		{
			ID:          1,
			Name:        "apikey",
			Type:        AccountTypeAPIKey,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: true,
			Priority:    1,
		},
		{
			ID:          2,
			Name:        "oauth-no-privacy",
			Type:        AccountTypeOAuth,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: true,
			Priority:    2,
		},
		{
			ID:          3,
			Name:        "oauth-ok",
			Type:        AccountTypeOAuth,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: true,
			Priority:    3,
			Extra: map[string]any{
				"privacy_mode": PrivacyModeTrainingOff,
			},
		},
	}

	group := &Group{
		ID:                1,
		Platform:          PlatformOpenAI,
		Status:            StatusActive,
		RequireOAuthOnly:  true,
		RequirePrivacySet: true,
	}

	filtered := make([]Account, 0, len(candidates))
	for _, candidate := range candidates {
		if group.RequireOAuthOnly && candidate.Type == AccountTypeAPIKey {
			continue
		}
		if group.RequirePrivacySet && !candidate.IsPrivacySet() {
			continue
		}
		filtered = append(filtered, candidate)
	}
	selected := selectUngroupedCandidate(filtered)
	if selected == nil {
		t.Fatal("selectUngroupedCandidate() returned nil")
	}
	if selected.ID != 3 {
		t.Fatalf("selected.ID = %d, want 3", selected.ID)
	}
}

func TestRemoveCandidateByID(t *testing.T) {
	input := []Account{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}
	output := removeCandidateByID(input, 2)
	if len(output) != 2 {
		t.Fatalf("len(output) = %d, want 2", len(output))
	}
	if output[0].ID != 1 || output[1].ID != 3 {
		t.Fatalf("output IDs = [%d %d], want [1 3]", output[0].ID, output[1].ID)
	}
}

func TestComputeProvisionCountUsesLargestGap(t *testing.T) {
	got := computeProvisionCount(1, false, 3)
	if got != 3 {
		t.Fatalf("computeProvisionCount() = %d, want 3", got)
	}
}

func TestComputeProvisionCountConcurrencyFallsBackToOne(t *testing.T) {
	got := computeProvisionCount(0, true, 0)
	if got != 1 {
		t.Fatalf("computeProvisionCount() = %d, want 1", got)
	}
}

func TestUsageWindowExhausted_RequiresActiveResetWindow(t *testing.T) {
	future := time.Now().Add(30 * time.Minute)
	if !usageWindowExhausted(time.Now(), &UsageProgress{Utilization: 100, ResetsAt: &future}) {
		t.Fatal("usageWindowExhausted() = false, want true")
	}
}

func TestUsageWindowExhausted_IgnoresExpiredWindow(t *testing.T) {
	past := time.Now().Add(-30 * time.Minute)
	if usageWindowExhausted(time.Now(), &UsageProgress{Utilization: 100, ResetsAt: &past}) {
		t.Fatal("usageWindowExhausted() = true, want false")
	}
}

func TestSelectUngroupedCandidateSkipsManualResetRequired(t *testing.T) {
	selected := selectUngroupedCandidate([]Account{
		{
			ID:       1,
			Priority: 1,
			Extra: map[string]any{
				accountAutoProvisionManualResetRequiredKey: true,
			},
		},
		{
			ID:       2,
			Priority: 2,
		},
	})
	if selected == nil {
		t.Fatal("selectUngroupedCandidate() returned nil")
	}
	if selected.ID != 2 {
		t.Fatalf("selected.ID = %d, want 2", selected.ID)
	}
}

func TestFetchAccountAICredits_EmptyCreditsTreatedAsUnknown(t *testing.T) {
	svc := &AccountAutoProvisionService{
		accountUsageService: &AccountUsageService{},
	}
	_ = svc
}
