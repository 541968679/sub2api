//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdminService_UpdateUser_PinnedAt(t *testing.T) {
	t.Parallel()

	existingPin := time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)
	trueVal := true
	falseVal := false

	t.Run("omitted pinned leaves timestamp unchanged", func(t *testing.T) {
		repo := &userRepoStub{user: &User{
			ID: 11, Email: "pin@example.com", Role: RoleUser, Status: StatusActive, PinnedAt: &existingPin,
		}}
		username := "kept"
		updated, err := (&adminServiceImpl{userRepo: repo}).UpdateUser(context.Background(), 11, &UpdateUserInput{
			Username: &username,
		})
		require.NoError(t, err)
		require.NotNil(t, updated.PinnedAt)
		require.True(t, updated.PinnedAt.Equal(existingPin))
	})

	t.Run("pin true writes a new timestamp", func(t *testing.T) {
		repo := &userRepoStub{user: &User{
			ID: 12, Email: "pin-on@example.com", Role: RoleUser, Status: StatusActive,
		}}
		before := time.Now()
		updated, err := (&adminServiceImpl{userRepo: repo}).UpdateUser(context.Background(), 12, &UpdateUserInput{
			Pinned: &trueVal,
		})
		require.NoError(t, err)
		require.NotNil(t, updated.PinnedAt)
		require.False(t, updated.PinnedAt.Before(before))
	})

	t.Run("pin false clears timestamp", func(t *testing.T) {
		repo := &userRepoStub{user: &User{
			ID: 13, Email: "pin-off@example.com", Role: RoleUser, Status: StatusActive, PinnedAt: &existingPin,
		}}
		updated, err := (&adminServiceImpl{userRepo: repo}).UpdateUser(context.Background(), 13, &UpdateUserInput{
			Pinned: &falseVal,
		})
		require.NoError(t, err)
		require.Nil(t, updated.PinnedAt)
	})
}
