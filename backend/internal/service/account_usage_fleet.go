package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// OpenAI OAuth fleet usage weights (Pro-equivalent units).
// prolite contributes 1/4 of a pro account at the same used_percent.
const (
	openAIOauthFleetProWeight     = 1.0
	openAIOauthFleetProliteWeight = 0.25
)

// OpenAIOauthFleetUsageSummary is the filter-independent fleet aggregate for
// OpenAI OAuth pro/prolite parent accounts (5h/7d weighted percent sums).
type OpenAIOauthFleetUsageSummary struct {
	Fleet5hPercent float64 `json:"fleet_5h_percent"`
	Fleet7dPercent float64 `json:"fleet_7d_percent"`
	ProCount       int     `json:"pro_count"`
	ProliteCount   int     `json:"prolite_count"`
	Missing5h      int     `json:"missing_5h"`
	Missing7d      int     `json:"missing_7d"`
	IncludedCount  int     `json:"included_count"`
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

// openAIOauthFleetPlanWeight returns the Pro-equivalent weight for an account
// plan_type, or 0 when the account is excluded from the fleet set.
func openAIOauthFleetPlanWeight(planType string) float64 {
	switch strings.ToLower(strings.TrimSpace(planType)) {
	case "pro", "chatgptpro":
		return openAIOauthFleetProWeight
	case "prolite":
		return openAIOauthFleetProliteWeight
	default:
		return 0
	}
}

// aggregateOpenAIOauthFleetUsage computes weighted percent sums for pro/prolite
// OpenAI OAuth parent accounts. Pure function for unit testing.
func aggregateOpenAIOauthFleetUsage(accounts []Account, now time.Time) OpenAIOauthFleetUsageSummary {
	var summary OpenAIOauthFleetUsageSummary
	for i := range accounts {
		acc := &accounts[i]
		if acc == nil || !acc.IsOpenAIOAuth() || acc.IsShadow() {
			continue
		}
		planType := strings.ToLower(strings.TrimSpace(acc.GetCredential("plan_type")))
		weight := openAIOauthFleetPlanWeight(planType)
		if weight <= 0 {
			continue
		}
		if planType == "prolite" {
			summary.ProliteCount++
		} else {
			summary.ProCount++
		}
		summary.IncludedCount++

		if progress := buildCodexUsageProgressFromExtra(acc.Extra, "5h", now); progress == nil {
			summary.Missing5h++
		} else {
			summary.Fleet5hPercent += progress.Utilization * weight
		}
		if progress := buildCodexUsageProgressFromExtra(acc.Extra, "7d", now); progress == nil {
			summary.Missing7d++
		} else {
			summary.Fleet7dPercent += progress.Utilization * weight
		}
	}
	return summary
}
