package dto

import "time"

type AccountAutoProvisionSettings struct {
	Enabled              bool                       `json:"enabled"`
	CheckIntervalSeconds int                        `json:"check_interval_seconds"`
	MaxActionsPerRun     int                        `json:"max_actions_per_run"`
	Rules                []AccountAutoProvisionRule `json:"rules"`
}

type AccountAutoProvisionRule struct {
	ID                            string                       `json:"id"`
	Name                          string                       `json:"name"`
	Enabled                       bool                         `json:"enabled"`
	GroupIDs                      []int64                      `json:"group_ids"`
	NormalAccountCountBelow       int                          `json:"normal_account_count_below"`
	ConcurrencyUtilizationAbove   float64                      `json:"concurrency_utilization_above"`
	AICreditsBelow                float64                      `json:"ai_credits_below"`
	AICreditsCheckIntervalMinutes int                          `json:"ai_credits_check_interval_minutes"`
	CooldownMinutes               int                          `json:"cooldown_minutes"`
	ProvisionMode                 string                       `json:"provision_mode"`
	Template                      AccountAutoProvisionTemplate `json:"template"`
}

type AccountAutoProvisionTemplate struct {
	ProxyID       *int64 `json:"proxy_id,omitempty"`
	Concurrency   int    `json:"concurrency"`
	Priority      *int   `json:"priority,omitempty"`
	LoadFactor    *int   `json:"load_factor,omitempty"`
	Schedulable   bool   `json:"schedulable"`
	AllowOverages bool   `json:"allow_overages"`
}

type AccountAutoProvisionState struct {
	LastTriggered        map[string]time.Time                    `json:"last_triggered"`
	LastHealthySnapshots map[string]AccountAutoProvisionSnapshot `json:"last_healthy_snapshots"`
	RecentLogs           []AccountAutoProvisionLogEntry          `json:"recent_logs"`
}

type AccountAutoProvisionSnapshot struct {
	SourceAccountID   int64                        `json:"source_account_id"`
	SourceAccountName string                       `json:"source_account_name"`
	CapturedAt        time.Time                    `json:"captured_at"`
	Template          AccountAutoProvisionTemplate `json:"template"`
}

type AccountAutoProvisionLogEntry struct {
	OccurredAt  time.Time `json:"occurred_at"`
	Level       string    `json:"level"`
	Action      string    `json:"action"`
	RuleID      string    `json:"rule_id,omitempty"`
	RuleName    string    `json:"rule_name,omitempty"`
	GroupID     int64     `json:"group_id,omitempty"`
	GroupName   string    `json:"group_name,omitempty"`
	AccountID   int64     `json:"account_id,omitempty"`
	AccountName string    `json:"account_name,omitempty"`
	Message     string    `json:"message"`
}

type AccountAutoProvisionSettingsResponse struct {
	Config AccountAutoProvisionSettings `json:"config"`
	State  AccountAutoProvisionState    `json:"state"`
}
