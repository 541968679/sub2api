package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *SettingService) GetAccountAutoProvisionSettings(ctx context.Context) (*AccountAutoProvisionSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAccountAutoProvisionSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultAccountAutoProvisionSettings(), nil
		}
		return nil, fmt.Errorf("get account auto provision settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultAccountAutoProvisionSettings(), nil
	}

	var settings AccountAutoProvisionSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultAccountAutoProvisionSettings(), nil
	}
	if err := normalizeAccountAutoProvisionSettings(&settings); err != nil {
		return DefaultAccountAutoProvisionSettings(), nil
	}
	return &settings, nil
}

func (s *SettingService) SetAccountAutoProvisionSettings(ctx context.Context, settings *AccountAutoProvisionSettings) error {
	if err := normalizeAccountAutoProvisionSettings(settings); err != nil {
		return err
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal account auto provision settings: %w", err)
	}
	return s.settingRepo.Set(ctx, SettingKeyAccountAutoProvisionSettings, string(data))
}

func (s *SettingService) GetAccountAutoProvisionState(ctx context.Context) (*AccountAutoProvisionState, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAccountAutoProvisionState)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultAccountAutoProvisionState(), nil
		}
		return nil, fmt.Errorf("get account auto provision state: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultAccountAutoProvisionState(), nil
	}

	var state AccountAutoProvisionState
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		return DefaultAccountAutoProvisionState(), nil
	}
	if state.LastTriggered == nil {
		state.LastTriggered = map[string]time.Time{}
	}
	if state.LastHealthySnapshots == nil {
		state.LastHealthySnapshots = map[string]AccountAutoProvisionSnapshot{}
	}
	if state.RecentLogs == nil {
		state.RecentLogs = []AccountAutoProvisionLogEntry{}
	}
	return &state, nil
}

func (s *SettingService) SetAccountAutoProvisionState(ctx context.Context, state *AccountAutoProvisionState) error {
	if state == nil {
		return fmt.Errorf("state cannot be nil")
	}
	if state.LastTriggered == nil {
		state.LastTriggered = map[string]time.Time{}
	}
	if state.LastHealthySnapshots == nil {
		state.LastHealthySnapshots = map[string]AccountAutoProvisionSnapshot{}
	}
	if state.RecentLogs == nil {
		state.RecentLogs = []AccountAutoProvisionLogEntry{}
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal account auto provision state: %w", err)
	}
	return s.settingRepo.Set(ctx, SettingKeyAccountAutoProvisionState, string(data))
}

func normalizeAccountAutoProvisionSettings(settings *AccountAutoProvisionSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	if settings.CheckIntervalSeconds < 30 || settings.CheckIntervalSeconds > 3600 {
		return fmt.Errorf("check_interval_seconds must be between 30 and 3600")
	}
	if settings.MaxActionsPerRun < 1 || settings.MaxActionsPerRun > 20 {
		return fmt.Errorf("max_actions_per_run must be between 1 and 20")
	}
	if settings.Rules == nil {
		settings.Rules = []AccountAutoProvisionRule{}
	}

	seenRuleIDs := make(map[string]struct{}, len(settings.Rules))
	for i := range settings.Rules {
		rule := &settings.Rules[i]
		rule.ID = strings.TrimSpace(rule.ID)
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.ID == "" {
			return fmt.Errorf("rules[%d].id cannot be empty", i)
		}
		if _, exists := seenRuleIDs[rule.ID]; exists {
			return fmt.Errorf("rules[%d].id must be unique", i)
		}
		seenRuleIDs[rule.ID] = struct{}{}
		if rule.Name == "" {
			rule.Name = rule.ID
		}
		if len(rule.GroupIDs) == 0 {
			return fmt.Errorf("rules[%d].group_ids cannot be empty", i)
		}
		rule.GroupIDs = dedupePositiveInt64s(rule.GroupIDs)
		if len(rule.GroupIDs) == 0 {
			return fmt.Errorf("rules[%d].group_ids must contain at least one positive id", i)
		}
		if rule.NormalAccountCountBelow < 0 || rule.NormalAccountCountBelow > 1000 {
			return fmt.Errorf("rules[%d].normal_account_count_below must be between 0 and 1000", i)
		}
		if rule.ConcurrencyUtilizationAbove < 0 || rule.ConcurrencyUtilizationAbove > 100 {
			return fmt.Errorf("rules[%d].concurrency_utilization_above must be between 0 and 100", i)
		}
		if rule.AICreditsBelow < 0 {
			return fmt.Errorf("rules[%d].ai_credits_below must be >= 0", i)
		}
		if rule.AICreditsBelow > 0 {
			if rule.AICreditsCheckIntervalMinutes < 5 || rule.AICreditsCheckIntervalMinutes > 240 {
				return fmt.Errorf("rules[%d].ai_credits_check_interval_minutes must be between 5 and 240", i)
			}
		} else if rule.AICreditsCheckIntervalMinutes <= 0 {
			rule.AICreditsCheckIntervalMinutes = 15
		}
		if rule.CooldownMinutes < 0 || rule.CooldownMinutes > 1440 {
			return fmt.Errorf("rules[%d].cooldown_minutes must be between 0 and 1440", i)
		}
		switch rule.ProvisionMode {
		case "", AccountAutoProvisionModeTemplate:
			rule.ProvisionMode = AccountAutoProvisionModeTemplate
		case AccountAutoProvisionModeCloneLastHealth:
		default:
			return fmt.Errorf("rules[%d].provision_mode is invalid", i)
		}
		if rule.Template.Concurrency < 1 || rule.Template.Concurrency > 10000 {
			return fmt.Errorf("rules[%d].template.concurrency must be between 1 and 10000", i)
		}
		if rule.Template.Priority != nil && *rule.Template.Priority < 0 {
			return fmt.Errorf("rules[%d].template.priority must be >= 0", i)
		}
		if rule.Template.LoadFactor != nil {
			if *rule.Template.LoadFactor < 0 || *rule.Template.LoadFactor > 10000 {
				return fmt.Errorf("rules[%d].template.load_factor must be between 0 and 10000", i)
			}
			if *rule.Template.LoadFactor == 0 {
				rule.Template.LoadFactor = nil
			}
		}
		if rule.NormalAccountCountBelow == 0 && rule.ConcurrencyUtilizationAbove == 0 && rule.AICreditsBelow == 0 {
			return fmt.Errorf("rules[%d] must enable at least one trigger", i)
		}
	}
	return nil
}

func dedupePositiveInt64s(values []int64) []int64 {
	result := make([]int64, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
