package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaultStreamStageTimingOff(t *testing.T) {
	resetViperWithJWTSecret(t)

	cfg, err := Load()
	require.NoError(t, err)
	require.False(t, cfg.Gateway.StreamStageTimingEnabled)
	require.InDelta(t, 0.02, cfg.Gateway.StreamStageTimingSampleRate, 1e-9)
	require.Empty(t, cfg.Gateway.StreamStageTimingAccountIDs)
}

func TestValidateStreamStageTimingSampleRate(t *testing.T) {
	resetViperWithJWTSecret(t)

	cfg, err := Load()
	require.NoError(t, err)

	cfg.Gateway.StreamStageTimingSampleRate = -0.1
	require.Error(t, cfg.Validate())

	cfg.Gateway.StreamStageTimingSampleRate = 1.1
	require.Error(t, cfg.Validate())

	cfg.Gateway.StreamStageTimingSampleRate = 0.5
	cfg.Gateway.StreamStageTimingEnabled = true
	require.NoError(t, cfg.Validate())
}
