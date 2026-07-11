//go:build unit

package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyFromServiceMapsLastUsedIP(t *testing.T) {
	ipAddress := "203.0.113.10"
	out := APIKeyFromService(&service.APIKey{ID: 1, UserID: 2, LastUsedIP: &ipAddress})

	require.NotNil(t, out.LastUsedIP)
	require.Equal(t, ipAddress, *out.LastUsedIP)
	require.Nil(t, APIKeyFromService(&service.APIKey{ID: 2}).LastUsedIP)
}
