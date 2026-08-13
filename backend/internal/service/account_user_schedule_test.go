//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestAccount_AllowsScheduleUser(t *testing.T) {
	t.Parallel()

	t.Run("unrestricted and empty mode allow everyone", func(t *testing.T) {
		t.Parallel()
		require.True(t, (&Account{}).AllowsScheduleUser(16))
		require.True(t, (&Account{UserScheduleMode: UserScheduleModeUnrestricted}).AllowsScheduleUser(0))
		require.True(t, (&Account{UserScheduleMode: ""}).AllowsScheduleUser(99))
	})

	t.Run("allow hit and miss", func(t *testing.T) {
		t.Parallel()
		acc := &Account{UserScheduleMode: UserScheduleModeAllow, ScheduleUserIDs: []int64{16, 42}}
		require.True(t, acc.AllowsScheduleUser(16))
		require.False(t, acc.AllowsScheduleUser(7))
	})

	t.Run("deny hit and miss", func(t *testing.T) {
		t.Parallel()
		acc := &Account{UserScheduleMode: UserScheduleModeDeny, ScheduleUserIDs: []int64{16}}
		require.False(t, acc.AllowsScheduleUser(16))
		require.True(t, acc.AllowsScheduleUser(7))
	})

	t.Run("userID zero fail closed for allow and deny", func(t *testing.T) {
		t.Parallel()
		require.False(t, (&Account{UserScheduleMode: UserScheduleModeAllow, ScheduleUserIDs: []int64{16}}).AllowsScheduleUser(0))
		require.False(t, (&Account{UserScheduleMode: UserScheduleModeDeny, ScheduleUserIDs: []int64{16}}).AllowsScheduleUser(0))
	})

	t.Run("empty allow fail closed and empty deny unrestricted", func(t *testing.T) {
		t.Parallel()
		require.False(t, (&Account{UserScheduleMode: UserScheduleModeAllow}).AllowsScheduleUser(16))
		require.True(t, (&Account{UserScheduleMode: UserScheduleModeDeny}).AllowsScheduleUser(16))
	})
}

func TestUserScheduleIDFromContext(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(9), scheduleUserIDFromContext(context.Background(), 9))
	require.Equal(t, int64(0), scheduleUserIDFromContext(context.Background(), 0))

	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	require.Equal(t, int64(16), scheduleUserIDFromContext(ctx, 0))
	require.Equal(t, int64(42), scheduleUserIDFromContext(ctx, 42))
}
