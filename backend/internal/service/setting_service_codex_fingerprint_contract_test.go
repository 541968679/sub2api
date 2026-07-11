package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettingServiceCodexRestrictionPolicyContract(t *testing.T) {
	t.Run("unconfigured policy stays disabled for existing accounts", func(t *testing.T) {
		svc := NewSettingService(newMockSettingRepo(), nil)
		policy, err := svc.GetCodexRestrictionPolicy(context.Background())
		require.NoError(t, err)
		require.False(t, policy.Configured)
		require.Empty(t, policy.EngineFingerprintSignals)
	})

	t.Run("explicit required signal enables fail closed policy", func(t *testing.T) {
		repo := newMockSettingRepo()
		repo.data[SettingKeyCodexCLIOnlyEngineFingerprintSignals] = `[{"type":"header_prefix","match":["x-codex-"],"required":true}]`
		svc := NewSettingService(repo, nil)
		policy, err := svc.GetCodexRestrictionPolicy(context.Background())
		require.NoError(t, err)
		require.True(t, policy.Configured)
		require.Len(t, policy.EngineFingerprintSignals, 1)
		require.True(t, policy.EngineFingerprintSignals[0].Required)
	})

	t.Run("malformed persisted JSON is not partially applied", func(t *testing.T) {
		repo := newMockSettingRepo()
		repo.data[SettingKeyCodexCLIOnlyWhitelist] = `{bad`
		svc := NewSettingService(repo, nil)
		_, err := svc.GetCodexRestrictionPolicy(context.Background())
		require.Error(t, err)
	})
}
