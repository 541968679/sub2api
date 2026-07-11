package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

var codexRestrictionSettingKeys = []string{
	SettingKeyMinCodexVersion,
	SettingKeyMaxCodexVersion,
	SettingKeyCodexCLIOnlyBlacklist,
	SettingKeyCodexCLIOnlyWhitelist,
	SettingKeyCodexCLIOnlyAllowAppServerClients,
	SettingKeyCodexCLIOnlyEngineFingerprintSignals,
}

func (s *SettingService) GetCodexRestrictionPolicy(ctx context.Context) (CodexRestrictionPolicy, error) {
	if s == nil || s.settingRepo == nil {
		return CodexRestrictionPolicy{}, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, codexRestrictionSettingKeys)
	if err != nil {
		return CodexRestrictionPolicy{}, err
	}
	policy := CodexRestrictionPolicy{
		MinCodexVersion:       strings.TrimSpace(values[SettingKeyMinCodexVersion]),
		MaxCodexVersion:       strings.TrimSpace(values[SettingKeyMaxCodexVersion]),
		AllowAppServerClients: values[SettingKeyCodexCLIOnlyAllowAppServerClients] == "true",
	}
	for _, key := range codexRestrictionSettingKeys {
		value := strings.TrimSpace(values[key])
		if value != "" && (key != SettingKeyCodexCLIOnlyAllowAppServerClients || value == "true") {
			policy.Configured = true
			break
		}
	}
	if raw := strings.TrimSpace(values[SettingKeyCodexCLIOnlyBlacklist]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &policy.Blacklist); err != nil {
			return CodexRestrictionPolicy{}, fmt.Errorf("parse Codex client blacklist: %w", err)
		}
	}
	if raw := strings.TrimSpace(values[SettingKeyCodexCLIOnlyWhitelist]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &policy.Whitelist); err != nil {
			return CodexRestrictionPolicy{}, fmt.Errorf("parse Codex client whitelist: %w", err)
		}
	}
	if signals, ok := openai.ParseEngineFingerprintSignals(values[SettingKeyCodexCLIOnlyEngineFingerprintSignals]); ok {
		policy.EngineFingerprintSignals = signals
	} else {
		return CodexRestrictionPolicy{}, fmt.Errorf("parse Codex engine fingerprint signals")
	}
	return policy, nil
}
