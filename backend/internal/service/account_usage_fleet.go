package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// OpenAI OAuth fleet plan capacity (Pro-equivalent percent units).
// One Pro account contributes 100 capacity units; Prolite contributes 25 (1/4 of Pro).
const (
	openAIOauthFleetProCapacity     = 100.0
	openAIOauthFleetProliteCapacity = 25.0
)

// OpenAIOauthFleetUsageSummary is the filter-independent fleet aggregate for
// OpenAI OAuth pro/prolite parent accounts.
//
// Display model (example: 7 Pro + 1 Prolite, all Pro half-used, Prolite full):
//
//	capacity = 7*100 + 1*25 = 725
//	used_5h  = 7*50  + 1*25 = 375
//	UI       = 375/725 with bar fill = used/capacity
type OpenAIOauthFleetUsageSummary struct {
	// Used5h / Used7d are consumed capacity units: Σ (used_percent * capacity_weight).
	// capacity_weight is 1.0 for Pro and 0.25 for Prolite (i.e. used_percent of that plan's units).
	Used5h float64 `json:"used_5h"`
	Used7d float64 `json:"used_7d"`
	// Capacity is total pool units from included accounts: pro*100 + prolite*25.
	// Same for both windows; always includes every pro/prolite parent (missing snapshots still count).
	Capacity float64 `json:"capacity"`
	// Fill5hPercent / Fill7dPercent are used/capacity * 100 for progress bars (0 when capacity=0).
	Fill5hPercent float64 `json:"fill_5h_percent"`
	Fill7dPercent float64 `json:"fill_7d_percent"`
	ProCount      int     `json:"pro_count"`
	ProliteCount  int     `json:"prolite_count"`
	Missing5h     int     `json:"missing_5h"`
	Missing7d     int     `json:"missing_7d"`
	IncludedCount int     `json:"included_count"`
}

// GetOpenAIOauthFleetUsage returns the global OpenAI OAuth pro/prolite fleet
// usage summary. Scope is independent of account-list filters.
func (s *AccountUsageService) GetOpenAIOauthFleetUsage(ctx context.Context) (*OpenAIOauthFleetUsageSummary, error) {
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("account usage service not configured")
	}
	// Empty status: all statuses (active/error/disabled). Shadows filtered in aggregate.
	accounts, err := s.accountRepo.ListAllWithFilters(ctx, PlatformOpenAI, AccountTypeOAuth, "", "", 0, "")
	if err != nil {
		return nil, fmt.Errorf("list openai oauth accounts for fleet usage: %w", err)
	}
	summary := aggregateOpenAIOauthFleetUsage(accounts, time.Now())
	return &summary, nil
}

// openAIOauthFleetPlanCapacity returns capacity units for a plan_type, or 0 when excluded.
func openAIOauthFleetPlanCapacity(planType string) float64 {
	switch strings.ToLower(strings.TrimSpace(planType)) {
	case "pro", "chatgptpro":
		return openAIOauthFleetProCapacity
	case "prolite":
		return openAIOauthFleetProliteCapacity
	default:
		return 0
	}
}

// aggregateOpenAIOauthFleetUsage computes used/capacity for pro/prolite OpenAI
// OAuth parent accounts. Pure function for unit testing.
func aggregateOpenAIOauthFleetUsage(accounts []Account, now time.Time) OpenAIOauthFleetUsageSummary {
	var summary OpenAIOauthFleetUsageSummary
	for i := range accounts {
		acc := &accounts[i]
		if acc == nil || !acc.IsOpenAIOAuth() || acc.IsShadow() {
			continue
		}
		planType := strings.ToLower(strings.TrimSpace(acc.GetCredential("plan_type")))
		capacity := openAIOauthFleetPlanCapacity(planType)
		if capacity <= 0 {
			continue
		}
		if planType == "prolite" {
			summary.ProliteCount++
		} else {
			summary.ProCount++
		}
		summary.IncludedCount++
		summary.Capacity += capacity

		// used_units = used_percent/100 * capacity = used_percent * (capacity/100)
		// capacity/100 is 1.0 for Pro and 0.25 for Prolite.
		unitWeight := capacity / 100.0

		if progress := buildCodexUsageProgressFromExtra(acc.Extra, "5h", now); progress == nil {
			summary.Missing5h++
		} else {
			summary.Used5h += progress.Utilization * unitWeight
		}
		if progress := buildCodexUsageProgressFromExtra(acc.Extra, "7d", now); progress == nil {
			summary.Missing7d++
		} else {
			summary.Used7d += progress.Utilization * unitWeight
		}
	}
	if summary.Capacity > 0 {
		summary.Fill5hPercent = summary.Used5h / summary.Capacity * 100
		summary.Fill7dPercent = summary.Used7d / summary.Capacity * 100
	}
	return summary
}
