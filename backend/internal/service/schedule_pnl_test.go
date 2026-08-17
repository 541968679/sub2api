//go:build unit

package service

import (
	"context"
	"testing"
	"time"
)

type stubSchedulePnlRepo struct {
	byAccount map[int64]SchedulePnlAgg
	byUser    map[int64]SchedulePnlAgg
	byBucket  map[string]SchedulePnlAgg
	lastPairs []SchedulePnlUserAccount
}

func (s *stubSchedulePnlRepo) SumSchedulePnl(context.Context, int64, []int64, time.Time, time.Time) (SchedulePnlAgg, error) {
	return SchedulePnlAgg{}, nil
}

func (s *stubSchedulePnlRepo) SumSchedulePnlByAccount(_ context.Context, _ int64, accountIDs []int64, _, _ time.Time) (map[int64]SchedulePnlAgg, error) {
	out := make(map[int64]SchedulePnlAgg)
	for _, id := range accountIDs {
		if agg, ok := s.byAccount[id]; ok {
			out[id] = agg
		}
	}
	return out, nil
}

func (s *stubSchedulePnlRepo) SumSchedulePnlByUserPairs(_ context.Context, pairs []SchedulePnlUserAccount, _, _ time.Time) (map[int64]SchedulePnlAgg, error) {
	s.lastPairs = append([]SchedulePnlUserAccount(nil), pairs...)
	out := make(map[int64]SchedulePnlAgg)
	for _, pair := range pairs {
		if agg, ok := s.byUser[pair.UserID]; ok {
			out[pair.UserID] = agg
		}
	}
	return out, nil
}

func (s *stubSchedulePnlRepo) ListSchedulePnlTrend(context.Context, int64, []int64, time.Time, time.Time, string, *time.Location) (map[string]SchedulePnlAgg, error) {
	return s.byBucket, nil
}

func TestBuildSchedulePnlWindow_EmptyAndZeroRevenue(t *testing.T) {
	if got := BuildSchedulePnlWindow(SchedulePnlAgg{}); got != nil {
		t.Fatalf("empty rows should be nil, got %+v", got)
	}
	zeroRev := BuildSchedulePnlWindow(SchedulePnlAgg{Revenue: 0, Cost: 1.5, Rows: 2})
	if zeroRev == nil || zeroRev.Margin != nil {
		t.Fatalf("revenue 0 should keep margin nil, got %+v", zeroRev)
	}
	if zeroRev.Profit != -1.5 {
		t.Fatalf("profit = revenue-cost, got %v", zeroRev.Profit)
	}
	window := BuildSchedulePnlWindow(SchedulePnlAgg{Revenue: 4, Cost: 1, Rows: 1})
	if window == nil || window.Margin == nil || *window.Margin != 0.75 {
		t.Fatalf("expected 75%% margin, got %+v", window)
	}
}

type perUserSmartRepo struct {
	bundles map[int64]*UserSmartScheduleBundle
}

func (s *perUserSmartRepo) ListByUser(_ context.Context, userID int64) (*UserSmartScheduleBundle, error) {
	return s.bundles[userID], nil
}

func (s *perUserSmartRepo) ListByUsers(_ context.Context, userIDs []int64) (map[int64]*UserSmartScheduleBundle, error) {
	out := make(map[int64]*UserSmartScheduleBundle, len(userIDs))
	for _, userID := range userIDs {
		out[userID] = s.bundles[userID]
	}
	return out, nil
}

func (s *perUserSmartRepo) ReplacePlatform(context.Context, int64, string, SmartSchedulePlatformWrite) error {
	return nil
}

func (s *perUserSmartRepo) UpdateSortOrders(context.Context, int64, string, []SmartScheduleSortAssignment) error {
	return nil
}

func TestSchedulePnlUserSummaries_OnlyEnabledPoolPairs(t *testing.T) {
	smart := &perUserSmartRepo{bundles: map[int64]*UserSmartScheduleBundle{
		10: {Policies: map[string]*SmartSchedulePlatformPolicy{
			"anthropic": {Enabled: true, AccountIDs: map[int64]struct{}{11: {}, 12: {}}},
			"openai":    {Enabled: false, AccountIDs: map[int64]struct{}{99: {}}},
		}},
		11: {Policies: map[string]*SmartSchedulePlatformPolicy{
			"anthropic": {Enabled: false, AccountIDs: map[int64]struct{}{1: {}}},
		}},
	}}
	pnl := &stubSchedulePnlRepo{byUser: map[int64]SchedulePnlAgg{
		10: {Revenue: 2, Cost: 0.5, Rows: 3},
	}}
	svc := &SchedulePnlService{pnl: pnl, smart: smart}
	out, err := svc.UserSummaries(context.Background(), []int64{10, 11}, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["11"]; ok {
		t.Fatalf("disabled/empty user 11 should be omitted: %+v", out)
	}
	summary, ok := out["10"]
	if !ok || summary.Today == nil || summary.Today.Revenue != 2 {
		t.Fatalf("enabled user 10 missing today window: %+v", out)
	}
	if len(pnl.lastPairs) != 2 {
		t.Fatalf("expected only enabled pool pairs, got %+v", pnl.lastPairs)
	}
	for _, pair := range pnl.lastPairs {
		if pair.AccountID == 99 {
			t.Fatalf("disabled platform account must not be summed: %+v", pnl.lastPairs)
		}
	}
}

func TestSchedulePnlPairSummaries_NullWindowWhenNoRows(t *testing.T) {
	svc := &SchedulePnlService{pnl: &stubSchedulePnlRepo{byAccount: map[int64]SchedulePnlAgg{
		1: {Revenue: 1.2, Cost: 0.3, Rows: 1},
	}}}
	out, err := svc.PairSummaries(context.Background(), 7, []int64{1, 2}, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if out["1"].Today == nil || out["1"].Today.Revenue != 1.2 {
		t.Fatalf("pair 1 should have today: %+v", out["1"])
	}
	if out["2"].Today != nil || out["2"].SevenDay != nil {
		t.Fatalf("pair 2 empty windows must be null: %+v", out["2"])
	}
}

func TestSchedulePnlTrend_EmptyBucketsStayNull(t *testing.T) {
	svc := &SchedulePnlService{
		pnl: &stubSchedulePnlRepo{},
		smart: &stubSmartRepo{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			"anthropic": {Enabled: true, AccountIDs: map[int64]struct{}{5: {}}},
		}}},
	}
	trend, err := svc.Trend(context.Background(), 7, nil, SchedulePnlRange7d, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	if trend.Granularity != "day" || len(trend.Points) != 7 {
		t.Fatalf("7d should be 7 daily buckets, got %+v", trend)
	}
	for _, point := range trend.Points {
		if point.Revenue != nil || point.Cost != nil || point.Profit != nil || point.Margin != nil {
			t.Fatalf("empty bucket must be nulls, got %+v", point)
		}
	}
}

func TestSchedulePnlTrend_AccountNotInPool(t *testing.T) {
	svc := &SchedulePnlService{
		smart: &stubSmartRepo{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			"anthropic": {Enabled: true, AccountIDs: map[int64]struct{}{5: {}}},
		}}},
	}
	outside := int64(9)
	_, err := svc.Trend(context.Background(), 7, &outside, SchedulePnlRange24h, "UTC")
	if err == nil {
		t.Fatal("expected account-not-in-pool error")
	}
}

func TestSchedulePnlIgnoresNullTrueCostRows(t *testing.T) {
	// Rows with Rows==0 (true_cost IS NULL filtered in SQL) stay nil windows.
	if got := BuildSchedulePnlWindow(SchedulePnlAgg{Revenue: 9, Cost: 1, Rows: 0}); got != nil {
		t.Fatalf("SQL-excluded NULL true_cost must not become a zero window: %+v", got)
	}
}
