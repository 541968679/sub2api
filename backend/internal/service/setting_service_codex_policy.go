package service

import (
	"context"
	"encoding/json"
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
	_ = json.Unmarshal([]byte(values[SettingKeyCodexCLIOnlyBlacklist]), &policy.Blacklist)
	_ = json.Unmarshal([]byte(values[SettingKeyCodexCLIOnlyWhitelist]), &policy.Whitelist)
	if signals, ok := openai.ParseEngineFingerprintSignals(values[SettingKeyCodexCLIOnlyEngineFingerprintSignals]); ok {
		policy.EngineFingerprintSignals = signals
	}
	return policy, nil
}
