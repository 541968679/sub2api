//go:build unit

package service

import (
	"context"
	"testing"
	"time"
)

type stubOpenAI7dCycleRepo struct {
	totals      []AccountModelTokenTotals
	insert      []OpenAI7dCycle
	latest      *OpenAI7dCycle
	cycles      []OpenAI7dCycle
	windowStats *OpenAI7dWindowAccountStats
}

func (r *stubOpenAI7dCycleRepo) GetAccountModelTokenTotals(context.Context, int64, time.Time, time.Time) ([]AccountModelTokenTotals, error) {
	return r.totals, nil
}

func (r *stubOpenAI7dCycleRepo) InsertCycle(_ context.Context, cycle OpenAI7dCycle) error {
	r.insert = append(r.insert, cycle)
	r.latest = &cycle
	return nil
}

func (r *stubOpenAI7dCycleRepo) GetLatestCycle(context.Context, int64) (*OpenAI7dCycle, error) {
	return r.latest, nil
}

func (r *stubOpenAI7dCycleRepo) ListCycles(context.Context, int64, int) ([]OpenAI7dCycle, error) {
	return r.cycles, nil
}

func (r *stubOpenAI7dCycleRepo) GetWindowAccountStats(context.Context, int64, time.Time, time.Time) (*OpenAI7dWindowAccountStats, error) {
	if r.windowStats == nil {
		return &OpenAI7dWindowAccountStats{}, nil
	}
	return r.windowStats, nil
}

func TestComputeLiteLLMStandardCost_UsesLiteLLMOnly(t *testing.T) {
	t.Parallel()
	cost := computeLiteLLMStandardCost(&LiteLLMModelPricing{
		InputCostPerToken:           0.001,
		OutputCostPerToken:          0.002,
		CacheCreationInputTokenCost: 0.003,
		CacheReadInputTokenCost:     0.0005,
	}, AccountModelTokenTotals{
		InputTokens:         10,
		OutputTokens:        5,
		CacheCreationTokens: 2,
		CacheReadTokens:     4,
	})
	want := 10*0.001 + 5*0.002 + 2*0.003 + 4*0.0005
	if diff := cost - want; diff > 1e-12 || diff < -1e-12 {
		t.Fatalf("cost = %v, want %v", cost, want)
	}
}

func TestComputeLiteLLMStandardCost_NilPricingIsZero(t *testing.T) {
	t.Parallel()
	if got := computeLiteLLMStandardCost(nil, AccountModelTokenTotals{InputTokens: 100}); got != 0 {
		t.Fatalf("got %v, want 0", got)
	}
}

func TestOpenAI7dLiteLLMPriceModel_PrefersUpstream(t *testing.T) {
	t.Parallel()
	got := openAI7dLiteLLMPriceModel("gpt-5.5", "claude-opus-4-8", "claude-opus-4-8")
	if got != "gpt-5.5" {
		t.Fatalf("got %q, want gpt-5.5", got)
	}
}

func TestShouldCloseOpenAI7dCycle(t *testing.T) {
	t.Parallel()
	old := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if shouldCloseOpenAI7dCycle(old, old.Add(30*time.Minute)) {
		t.Fatal("same-window jitter should not close")
	}
	if !shouldCloseOpenAI7dCycle(old, old.Add(7*24*time.Hour)) {
		t.Fatal("new 7d window should close")
	}
	if shouldCloseOpenAI7dCycle(time.Time{}, old) {
		t.Fatal("missing old reset should not close")
	}
}

func TestResolveOpenAI7dWindow_NoResetAt(t *testing.T) {
	t.Parallel()
	_, _, ok := resolveOpenAI7dWindow(map[string]any{}, nil, time.Now().UTC())
	if ok {
		t.Fatal("missing reset_at must not invent a window")
	}
}

func TestResolveOpenAI7dWindow_AlignsToResetAt(t *testing.T) {
	t.Parallel()
	end := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	start, gotEnd, ok := resolveOpenAI7dWindow(map[string]any{
		"codex_7d_reset_at":       end.Format(time.RFC3339),
		"codex_7d_window_seconds": 7 * 24 * 3600,
	}, nil, now)
	if !ok {
		t.Fatal("expected window")
	}
	if !gotEnd.Equal(end) {
		t.Fatalf("end = %v", gotEnd)
	}
	wantStart := end.Add(-7 * 24 * time.Hour)
	if !start.Equal(wantStart) {
		t.Fatalf("start = %v, want %v", start, wantStart)
	}
}

func TestCloseIfReset_IdempotentSecondJumpKeepsBoth(t *testing.T) {
	t.Parallel()
	repo := &stubOpenAI7dCycleRepo{
		totals: []AccountModelTokenTotals{{Model: "missing", InputTokens: 100}},
	}
	svc := NewOpenAI7dLiteLLMCycleService(repo, nil)
	oldEnd := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	account := &Account{
		ID:       9,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_7d_reset_at":     oldEnd.Format(time.RFC3339),
			"codex_7d_used_percent": 88.5,
		},
	}
	svc.CloseIfReset(context.Background(), account, map[string]any{
		"codex_7d_reset_at": oldEnd.Add(7 * 24 * time.Hour).Format(time.RFC3339),
	})
	if len(repo.insert) != 1 {
		t.Fatalf("inserts = %d, want 1", len(repo.insert))
	}
	if repo.insert[0].UsedPercent != 88.5 {
		t.Fatalf("used_percent = %v", repo.insert[0].UsedPercent)
	}
	if repo.insert[0].LiteLLMCost != 0 {
		t.Fatalf("missing LiteLLM model must cost 0, got %v", repo.insert[0].LiteLLMCost)
	}
	account.Extra["codex_7d_reset_at"] = oldEnd.Add(7 * 24 * time.Hour).Format(time.RFC3339)
	account.Extra["codex_7d_used_percent"] = 10.0
	svc.CloseIfReset(context.Background(), account, map[string]any{
		"codex_7d_reset_at": oldEnd.Add(14 * 24 * time.Hour).Format(time.RFC3339),
	})
	if len(repo.insert) != 2 {
		t.Fatalf("second close should insert another row, got %d", len(repo.insert))
	}
	if repo.insert[1].UsedPercent != 10.0 {
		t.Fatalf("second used_percent = %v", repo.insert[1].UsedPercent)
	}
}

func TestAttachCurrent_NoResetAtLeavesCostUnset(t *testing.T) {
	t.Parallel()
	svc := NewOpenAI7dLiteLLMCycleService(&stubOpenAI7dCycleRepo{}, nil)
	usage := &UsageInfo{SevenDay: &UsageProgress{Utilization: 0}}
	svc.AttachCurrent(context.Background(), &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{},
	}, usage)
	if usage.SevenDay.LiteLLMCost != nil {
		t.Fatal("no Codex 7d snapshot must not invent L$")
	}
}

func TestAttachCurrent_DoesNotMutateWindowStatsCost(t *testing.T) {
	t.Parallel()
	svc := NewOpenAI7dLiteLLMCycleService(&stubOpenAI7dCycleRepo{
		totals: []AccountModelTokenTotals{{Model: "missing", InputTokens: 100}},
	}, nil)
	usage := &UsageInfo{SevenDay: &UsageProgress{
		Utilization: 10,
		WindowStats: &WindowStats{Cost: 1.23, UserCost: 0.5},
	}}
	svc.AttachCurrent(context.Background(), &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_7d_reset_at": time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		},
	}, usage)
	if usage.SevenDay.WindowStats == nil || usage.SevenDay.WindowStats.Cost != 1.23 {
		t.Fatalf("A$ window_stats.cost must stay 1.23, got %+v", usage.SevenDay.WindowStats)
	}
	if usage.SevenDay.WindowStats.UserCost != 0.5 {
		t.Fatalf("U$ mutated: %v", usage.SevenDay.WindowStats.UserCost)
	}
}

func TestResolveOpenAI7dWindow_ManualResetStartsAtLastEnd(t *testing.T) {
	t.Parallel()
	end := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	last := &OpenAI7dCycle{WindowEnd: time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)}
	start, gotEnd, ok := resolveOpenAI7dWindow(map[string]any{
		"codex_7d_reset_at":       end.Format(time.RFC3339),
		"codex_7d_window_seconds": 7 * 24 * 3600,
	}, last, now)
	if !ok {
		t.Fatal("expected window")
	}
	if !gotEnd.Equal(end) || !start.Equal(last.WindowEnd) {
		t.Fatalf("start=%v end=%v", start, gotEnd)
	}
}

func TestAttachCurrent_SkipsAPIKey(t *testing.T) {
	t.Parallel()
	svc := NewOpenAI7dLiteLLMCycleService(&stubOpenAI7dCycleRepo{}, nil)
	usage := &UsageInfo{SevenDay: &UsageProgress{Utilization: 12}}
	svc.AttachCurrent(context.Background(), &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"codex_7d_reset_at": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		},
	}, usage)
	if usage.SevenDay.LiteLLMCost != nil {
		t.Fatal("API key must not get L$")
	}
}

func TestListHistory_CurrentThenClosed(t *testing.T) {
	t.Parallel()
	closedEnd := time.Now().UTC().Add(-24 * time.Hour)
	closedStart := closedEnd.Add(-7 * 24 * time.Hour)
	repo := &stubOpenAI7dCycleRepo{
		cycles: []OpenAI7dCycle{{
			AccountID:   8,
			WindowStart: closedStart,
			WindowEnd:   closedEnd,
			LiteLLMCost: 9.25,
			UsedPercent: 88,
		}},
		windowStats: &OpenAI7dWindowAccountStats{
			Requests:    4,
			Tokens:      400,
			AccountCost: 1.5,
			UserCost:    0.7,
		},
	}
	svc := NewOpenAI7dLiteLLMCycleService(repo, nil)
	hist := svc.ListHistory(context.Background(), &Account{
		ID:       8,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_7d_reset_at":     time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339),
			"codex_7d_used_percent": 12.4,
		},
	})
	if hist == nil || len(hist.Items) != 2 {
		t.Fatalf("items = %v", hist)
	}
	if !hist.Items[0].Current || hist.Items[1].Current {
		t.Fatalf("current must be first: %+v", hist.Items)
	}
	if hist.Items[1].LiteLLMCost != 9.25 || hist.Items[1].AccountCost != 1.5 {
		t.Fatalf("closed row = %+v", hist.Items[1])
	}
}

func TestListHistory_ExpiredResetStillIncludesCurrent(t *testing.T) {
	t.Parallel()
	ended := time.Now().UTC().Add(-48 * time.Hour)
	repo := &stubOpenAI7dCycleRepo{
		windowStats: &OpenAI7dWindowAccountStats{
			Requests:    2,
			Tokens:      200,
			AccountCost: 0.4,
			UserCost:    0.2,
		},
	}
	svc := NewOpenAI7dLiteLLMCycleService(repo, nil)
	hist := svc.ListHistory(context.Background(), &Account{
		ID:       9,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_7d_reset_at":     ended.Format(time.RFC3339),
			"codex_7d_used_percent": 5,
		},
	})
	if hist == nil || len(hist.Items) != 1 {
		t.Fatalf("items = %v", hist)
	}
	if !hist.Items[0].Current || hist.Items[0].AccountCost != 0.4 || hist.Items[0].UsedPercent != 5 {
		t.Fatalf("expired current = %+v", hist.Items[0])
	}
	if hist.Items[0].WindowEnd.UTC().Unix() != ended.Unix() {
		t.Fatalf("window_end = %s want %s", hist.Items[0].WindowEnd, ended)
	}
}

func TestListHistory_SkipsAPIKey(t *testing.T) {
	t.Parallel()
	svc := NewOpenAI7dLiteLLMCycleService(&stubOpenAI7dCycleRepo{
		cycles: []OpenAI7dCycle{{LiteLLMCost: 1}},
	}, nil)
	hist := svc.ListHistory(context.Background(), &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	})
	if hist == nil || len(hist.Items) != 0 {
		t.Fatalf("API key must not get history, got %+v", hist)
	}
}
