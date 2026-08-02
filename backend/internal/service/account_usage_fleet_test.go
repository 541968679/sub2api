package service

import (
	"testing"
	"time"
)

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

	t.Run("pro and prolite weighted sum", func(t *testing.T) {
		summary := aggregateOpenAIOauthFleetUsage([]Account{
			pro(40, 10),
			prolite(40, 20),
		}, now)
		if summary.Fleet5hPercent != 50 {
			t.Fatalf("fleet_5h = %v, want 50", summary.Fleet5hPercent)
		}
		// 10*1 + 20*0.25 = 15
		if summary.Fleet7dPercent != 15 {
			t.Fatalf("fleet_7d = %v, want 15", summary.Fleet7dPercent)
		}
		if summary.ProCount != 1 || summary.ProliteCount != 1 || summary.IncludedCount != 2 {
			t.Fatalf("counts = pro=%d prolite=%d included=%d", summary.ProCount, summary.ProliteCount, summary.IncludedCount)
		}
		if summary.Missing5h != 0 || summary.Missing7d != 0 {
			t.Fatalf("missing = 5h=%d 7d=%d", summary.Missing5h, summary.Missing7d)
		}
	})

	t.Run("two pro full load exceeds 100", func(t *testing.T) {
		summary := aggregateOpenAIOauthFleetUsage([]Account{
			pro(100, 100),
			pro(100, 100),
		}, now)
		if summary.Fleet5hPercent != 200 {
			t.Fatalf("fleet_5h = %v, want 200", summary.Fleet5hPercent)
		}
	})

	t.Run("plus excluded", func(t *testing.T) {
		plus := pro(100, 100)
		plus.Credentials["plan_type"] = "plus"
		summary := aggregateOpenAIOauthFleetUsage([]Account{plus, pro(10, 10)}, now)
		if summary.IncludedCount != 1 || summary.Fleet5hPercent != 10 {
			t.Fatalf("got included=%d fleet5h=%v, want 1 and 10", summary.IncludedCount, summary.Fleet5hPercent)
		}
	})

	t.Run("shadow excluded", func(t *testing.T) {
		parentID := int64(99)
		shadow := pro(100, 100)
		shadow.ParentAccountID = &parentID
		summary := aggregateOpenAIOauthFleetUsage([]Account{shadow, pro(25, 25)}, now)
		if summary.IncludedCount != 1 || summary.Fleet5hPercent != 25 {
			t.Fatalf("got included=%d fleet5h=%v, want 1 and 25", summary.IncludedCount, summary.Fleet5hPercent)
		}
	})

	t.Run("missing 5h only", func(t *testing.T) {
		a := pro(0, 10)
		delete(a.Extra, "codex_5h_used_percent")
		delete(a.Extra, "codex_5h_reset_at")
		summary := aggregateOpenAIOauthFleetUsage([]Account{a}, now)
		if summary.Missing5h != 1 {
			t.Fatalf("missing_5h = %d, want 1", summary.Missing5h)
		}
		if summary.Fleet5hPercent != 0 {
			t.Fatalf("fleet_5h = %v, want 0", summary.Fleet5hPercent)
		}
		if summary.Missing7d != 0 || summary.Fleet7dPercent != 10 {
			t.Fatalf("7d missing=%d fleet=%v, want 0 and 10", summary.Missing7d, summary.Fleet7dPercent)
		}
		if summary.ProCount != 1 {
			t.Fatalf("pro_count = %d, want 1", summary.ProCount)
		}
	})

	t.Run("expired window zero not missing", func(t *testing.T) {
		a := pro(42, 88)
		a.Extra["codex_5h_reset_at"] = expired5h
		summary := aggregateOpenAIOauthFleetUsage([]Account{a}, now)
		if summary.Missing5h != 0 {
			t.Fatalf("missing_5h = %d, want 0", summary.Missing5h)
		}
		if summary.Fleet5hPercent != 0 {
			t.Fatalf("fleet_5h = %v, want 0 for expired", summary.Fleet5hPercent)
		}
		if summary.Fleet7dPercent != 88 {
			t.Fatalf("fleet_7d = %v, want 88", summary.Fleet7dPercent)
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
		// 20*1 + 40*0.25 = 30
		if summary.Fleet5hPercent != 30 {
			t.Fatalf("fleet_5h = %v, want 30", summary.Fleet5hPercent)
		}
	})

	t.Run("disabled error still counted", func(t *testing.T) {
		a := pro(50, 50)
		a.Status = StatusError
		b := prolite(100, 100)
		b.Status = StatusDisabled
		summary := aggregateOpenAIOauthFleetUsage([]Account{a, b}, now)
		// 50 + 25 = 75
		if summary.Fleet5hPercent != 75 || summary.IncludedCount != 2 {
			t.Fatalf("fleet5h=%v included=%d, want 75 and 2", summary.Fleet5hPercent, summary.IncludedCount)
		}
	})

	t.Run("non oauth excluded", func(t *testing.T) {
		a := pro(100, 100)
		a.Type = AccountTypeAPIKey
		summary := aggregateOpenAIOauthFleetUsage([]Account{a}, now)
		if summary.IncludedCount != 0 {
			t.Fatalf("included = %d, want 0", summary.IncludedCount)
		}
	})
}

func TestOpenAIOauthFleetPlanWeight(t *testing.T) {
	t.Parallel()
	cases := []struct {
		plan string
		want float64
	}{
		{"pro", 1},
		{"chatgptpro", 1},
		{"prolite", 0.25},
		{"plus", 0},
		{"team", 0},
		{"free", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := openAIOauthFleetPlanWeight(tc.plan); got != tc.want {
			t.Fatalf("plan %q weight = %v, want %v", tc.plan, got, tc.want)
		}
	}
}
