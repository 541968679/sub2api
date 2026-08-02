package service

import (
	"math"
	"testing"
	"time"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestAggregateOpenAIOauthFleetUsage(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	active5h := now.Add(2 * time.Hour).Format(time.RFC3339)
	active7d := now.Add(48 * time.Hour).Format(time.RFC3339)
	expired5h := now.Add(-2 * time.Hour).Format(time.RFC3339)

	pro := func(pct5h, pct7d float64) Account {
		return Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"plan_type": "pro",
			},
			Extra: map[string]any{
				"codex_5h_used_percent": pct5h,
				"codex_5h_reset_at":     active5h,
				"codex_7d_used_percent": pct7d,
				"codex_7d_reset_at":     active7d,
			},
		}
	}
	prolite := func(pct5h, pct7d float64) Account {
		a := pro(pct5h, pct7d)
		a.Credentials["plan_type"] = "prolite"
		return a
	}

	t.Run("user example 7 pro half + 1 prolite full", func(t *testing.T) {
		accounts := make([]Account, 0, 8)
		for i := 0; i < 7; i++ {
			accounts = append(accounts, pro(50, 50))
		}
		accounts = append(accounts, prolite(100, 100))
		summary := aggregateOpenAIOauthFleetUsage(accounts, now)
		if summary.Capacity != 725 {
			t.Fatalf("capacity = %v, want 725", summary.Capacity)
		}
		if summary.Used5h != 375 {
			t.Fatalf("used_5h = %v, want 375", summary.Used5h)
		}
		// 375/725 * 100
		wantFill := 375.0 / 725.0 * 100
		if !almostEqual(summary.Fill5hPercent, wantFill) {
			t.Fatalf("fill_5h = %v, want %v", summary.Fill5hPercent, wantFill)
		}
		if summary.ProCount != 7 || summary.ProliteCount != 1 || summary.IncludedCount != 8 {
			t.Fatalf("counts pro=%d prolite=%d included=%d", summary.ProCount, summary.ProliteCount, summary.IncludedCount)
		}
	})

	t.Run("pro and prolite used units and capacity", func(t *testing.T) {
		// pro 40% → 40 units; prolite 40% → 10 units; capacity 125
		summary := aggregateOpenAIOauthFleetUsage([]Account{
			pro(40, 10),
			prolite(40, 20),
		}, now)
		if summary.Capacity != 125 {
			t.Fatalf("capacity = %v, want 125", summary.Capacity)
		}
		if summary.Used5h != 50 {
			t.Fatalf("used_5h = %v, want 50", summary.Used5h)
		}
		// 10 + 20*0.25 = 15
		if summary.Used7d != 15 {
			t.Fatalf("used_7d = %v, want 15", summary.Used7d)
		}
		if !almostEqual(summary.Fill5hPercent, 40) {
			t.Fatalf("fill_5h = %v, want 40", summary.Fill5hPercent)
		}
	})

	t.Run("two pro full is 200/200 not 200 percent alone", func(t *testing.T) {
		summary := aggregateOpenAIOauthFleetUsage([]Account{
			pro(100, 100),
			pro(100, 100),
		}, now)
		if summary.Used5h != 200 || summary.Capacity != 200 {
			t.Fatalf("used/capacity = %v/%v, want 200/200", summary.Used5h, summary.Capacity)
		}
		if !almostEqual(summary.Fill5hPercent, 100) {
			t.Fatalf("fill_5h = %v, want 100", summary.Fill5hPercent)
		}
	})

	t.Run("plus excluded", func(t *testing.T) {
		plus := pro(100, 100)
		plus.Credentials["plan_type"] = "plus"
		summary := aggregateOpenAIOauthFleetUsage([]Account{plus, pro(10, 10)}, now)
		if summary.IncludedCount != 1 || summary.Used5h != 10 || summary.Capacity != 100 {
			t.Fatalf("got included=%d used=%v cap=%v", summary.IncludedCount, summary.Used5h, summary.Capacity)
		}
	})

	t.Run("shadow excluded", func(t *testing.T) {
		parentID := int64(99)
		shadow := pro(100, 100)
		shadow.ParentAccountID = &parentID
		summary := aggregateOpenAIOauthFleetUsage([]Account{shadow, pro(25, 25)}, now)
		if summary.IncludedCount != 1 || summary.Used5h != 25 || summary.Capacity != 100 {
			t.Fatalf("got included=%d used=%v cap=%v", summary.IncludedCount, summary.Used5h, summary.Capacity)
		}
	})

	t.Run("missing 5h still counts capacity", func(t *testing.T) {
		a := pro(0, 10)
		delete(a.Extra, "codex_5h_used_percent")
		delete(a.Extra, "codex_5h_reset_at")
		summary := aggregateOpenAIOauthFleetUsage([]Account{a}, now)
		if summary.Missing5h != 1 {
			t.Fatalf("missing_5h = %d, want 1", summary.Missing5h)
		}
		if summary.Used5h != 0 {
			t.Fatalf("used_5h = %v, want 0", summary.Used5h)
		}
		if summary.Capacity != 100 {
			t.Fatalf("capacity = %v, want 100 (missing still in pool)", summary.Capacity)
		}
		if summary.Used7d != 10 {
			t.Fatalf("used_7d = %v, want 10", summary.Used7d)
		}
	})

	t.Run("expired window zero used not missing", func(t *testing.T) {
		a := pro(42, 88)
		a.Extra["codex_5h_reset_at"] = expired5h
		summary := aggregateOpenAIOauthFleetUsage([]Account{a}, now)
		if summary.Missing5h != 0 {
			t.Fatalf("missing_5h = %d, want 0", summary.Missing5h)
		}
		if summary.Used5h != 0 {
			t.Fatalf("used_5h = %v, want 0 for expired", summary.Used5h)
		}
		if summary.Used7d != 88 {
			t.Fatalf("used_7d = %v, want 88", summary.Used7d)
		}
	})

	t.Run("chatgptpro alias and case insensitive", func(t *testing.T) {
		a := pro(20, 20)
		a.Credentials["plan_type"] = "ChatGPTPro"
		b := prolite(40, 0)
		b.Credentials["plan_type"] = "PROLITE"
		summary := aggregateOpenAIOauthFleetUsage([]Account{a, b}, now)
		if summary.ProCount != 1 || summary.ProliteCount != 1 {
			t.Fatalf("counts pro=%d prolite=%d", summary.ProCount, summary.ProliteCount)
		}
		if summary.Capacity != 125 {
			t.Fatalf("capacity = %v, want 125", summary.Capacity)
		}
		// 20 + 10 = 30
		if summary.Used5h != 30 {
			t.Fatalf("used_5h = %v, want 30", summary.Used5h)
		}
	})

	t.Run("disabled error still counted", func(t *testing.T) {
		a := pro(50, 50)
		a.Status = StatusError
		b := prolite(100, 100)
		b.Status = StatusDisabled
		summary := aggregateOpenAIOauthFleetUsage([]Account{a, b}, now)
		// used 50+25=75, capacity 125
		if summary.Used5h != 75 || summary.Capacity != 125 || summary.IncludedCount != 2 {
			t.Fatalf("used=%v cap=%v included=%d", summary.Used5h, summary.Capacity, summary.IncludedCount)
		}
	})

	t.Run("non oauth excluded", func(t *testing.T) {
		a := pro(100, 100)
		a.Type = AccountTypeAPIKey
		summary := aggregateOpenAIOauthFleetUsage([]Account{a}, now)
		if summary.IncludedCount != 0 || summary.Capacity != 0 {
			t.Fatalf("included=%d cap=%v, want 0", summary.IncludedCount, summary.Capacity)
		}
	})
}

func TestOpenAIOauthFleetPlanCapacity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		plan string
		want float64
	}{
		{"pro", 100},
		{"chatgptpro", 100},
		{"prolite", 25},
		{"plus", 0},
		{"team", 0},
		{"free", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := openAIOauthFleetPlanCapacity(tc.plan); got != tc.want {
			t.Fatalf("plan %q capacity = %v, want %v", tc.plan, got, tc.want)
		}
	}
}
