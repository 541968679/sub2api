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

	t.Run("empty three configs allow everyone", func(t *testing.T) {
		t.Parallel()
		require.True(t, (&Account{}).AllowsScheduleUser(16))
		require.True(t, (&Account{UserScheduleMode: UserScheduleModeUnrestricted}).AllowsScheduleUser(0))
		require.True(t, (&Account{UserScheduleMode: ""}).AllowsScheduleUser(99))
	})

	t.Run("allow hit and miss", func(t *testing.T) {
		t.Parallel()
		acc := &Account{AllowUserIDs: []int64{16, 42}}
		require.True(t, acc.AllowsScheduleUser(16))
		require.False(t, acc.AllowsScheduleUser(7))
	})

	t.Run("deny hit and miss", func(t *testing.T) {
		t.Parallel()
		acc := &Account{DenyUserIDs: []int64{16}}
		require.False(t, acc.AllowsScheduleUser(16))
		require.True(t, acc.AllowsScheduleUser(7))
	})

	t.Run("userID zero fail closed when any rule exists", func(t *testing.T) {
		t.Parallel()
		require.False(t, (&Account{AllowUserIDs: []int64{16}}).AllowsScheduleUser(0))
		require.False(t, (&Account{DenyUserIDs: []int64{16}}).AllowsScheduleUser(0))
		require.False(t, (&Account{UserConcurrency: map[int64]int{16: 5}}).AllowsScheduleUser(0))
	})

	t.Run("empty allow is not a whitelist", func(t *testing.T) {
		t.Parallel()
		require.True(t, (&Account{DenyUserIDs: nil, AllowUserIDs: nil}).AllowsScheduleUser(16))
	})

	t.Run("deny wins over allow and cap", func(t *testing.T) {
		t.Parallel()
		acc := &Account{
			AllowUserIDs:    []int64{16},
			DenyUserIDs:     []int64{16},
			UserConcurrency: map[int64]int{16: 5},
		}
		require.False(t, acc.AllowsScheduleUser(16))
		require.Equal(t, 5, acc.PairMaxConcurrency(16))
	})

	t.Run("allow plus cap is admitted", func(t *testing.T) {
		t.Parallel()
		acc := &Account{
			AllowUserIDs:    []int64{16},
			UserConcurrency: map[int64]int{16: 5},
		}
		require.True(t, acc.AllowsScheduleUser(16))
		require.Equal(t, 5, acc.PairMaxConcurrency(16))
	})

	t.Run("allow list nonempty excludes outsiders even with a cap", func(t *testing.T) {
		t.Parallel()
		acc := &Account{
			AllowUserIDs:    []int64{16},
			UserConcurrency: map[int64]int{7: 3},
		}
		require.False(t, acc.AllowsScheduleUser(7))
		require.Equal(t, 3, acc.PairMaxConcurrency(7))
	})
}

func TestAccount_PairMaxConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("unset zero and negative are no pair cap", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, 0, (&Account{}).PairMaxConcurrency(16))
		require.Equal(t, 0, (&Account{UserConcurrency: map[int64]int{16: 0}}).PairMaxConcurrency(16))
		require.Equal(t, 0, (&Account{UserConcurrency: map[int64]int{16: -2}}).PairMaxConcurrency(16))
		require.Equal(t, 0, (&Account{UserConcurrency: map[int64]int{16: 4}}).PairMaxConcurrency(0))
	})

	t.Run("explicit N is returned", func(t *testing.T) {
		t.Parallel()
		acc := &Account{UserConcurrency: map[int64]int{16: 1, 42: 8}}
		require.Equal(t, 1, acc.PairMaxConcurrency(16))
		require.Equal(t, 8, acc.PairMaxConcurrency(42))
		require.Equal(t, 0, acc.PairMaxConcurrency(7))
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
