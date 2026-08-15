package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserFromServiceAdmin_MapsActivityTimestamps(t *testing.T) {
	t.Parallel()

	lastLoginAt := time.Date(2026, time.April, 20, 10, 0, 0, 0, time.UTC)
	lastActiveAt := lastLoginAt.Add(15 * time.Minute)
	lastUsedAt := lastLoginAt.Add(45 * time.Minute)

	out := UserFromServiceAdmin(&service.User{
		ID:           42,
		Email:        "admin@example.com",
		Username:     "admin",
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
		LastActiveAt: &lastActiveAt,
		LastUsedAt:   &lastUsedAt,
	})

	require.NotNil(t, out)
	require.NotNil(t, out.LastActiveAt)
	require.NotNil(t, out.LastUsedAt)
	require.WithinDuration(t, lastActiveAt, *out.LastActiveAt, time.Second)
	require.WithinDuration(t, lastUsedAt, *out.LastUsedAt, time.Second)
}

func TestUserFromServiceAdmin_MapsPinned(t *testing.T) {
	t.Parallel()

	pinnedAt := time.Date(2026, time.August, 15, 4, 0, 0, 0, time.UTC)
	out := UserFromServiceAdmin(&service.User{
		ID:       7,
		Email:    "pinned@example.com",
		Role:     service.RoleUser,
		Status:   service.StatusActive,
		PinnedAt: &pinnedAt,
	})
	require.NotNil(t, out)
	require.True(t, out.Pinned)
	require.NotNil(t, out.PinnedAt)
	require.True(t, out.PinnedAt.Equal(pinnedAt))
}

func TestUserFromService_OmitsPinned(t *testing.T) {
	t.Parallel()

	pinnedAt := time.Date(2026, time.August, 15, 4, 0, 0, 0, time.UTC)
	out := UserFromService(&service.User{
		ID:       7,
		Email:    "public@example.com",
		Role:     service.RoleUser,
		Status:   service.StatusActive,
		PinnedAt: &pinnedAt,
	})
	require.NotNil(t, out)

	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `"pinned"`)
	require.NotContains(t, string(encoded), `"pinned_at"`)
}
