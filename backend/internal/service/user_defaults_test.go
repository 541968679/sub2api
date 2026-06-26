//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeDownstreamUsageTokenMode_DefaultsToDisplay(t *testing.T) {
	require.Equal(t, DownstreamUsageTokenModeDisplay, DefaultDownstreamUsageTokenMode)
	require.Equal(t, DownstreamUsageTokenModeDisplay, NormalizeDownstreamUsageTokenMode(""))
	require.Equal(t, DownstreamUsageTokenModeDisplay, NormalizeDownstreamUsageTokenMode("unknown"))
	require.Equal(t, DownstreamUsageTokenModeDisplay, NormalizeDownstreamUsageTokenMode(" display "))
	require.Equal(t, DownstreamUsageTokenModeReal, NormalizeDownstreamUsageTokenMode(" real "))
}
