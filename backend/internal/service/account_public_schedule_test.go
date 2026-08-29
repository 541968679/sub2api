//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestAccount_AllowsPublicSchedule(t *testing.T) {
	t.Parallel()
	require.False(t, (*Account)(nil).AllowsPublicSchedule())
	require.True(t, (&Account{}).AllowsPublicSchedule())
	require.True(t, (&Account{Extra: map[string]any{}}).AllowsPublicSchedule())
	require.True(t, (&Account{Extra: map[string]any{AccountExtraPublicSchedulable: true}}).AllowsPublicSchedule())
	require.False(t, (&Account{Extra: map[string]any{AccountExtraPublicSchedulable: false}}).AllowsPublicSchedule())
	require.False(t, (&Account{Extra: map[string]any{AccountExtraPublicSchedulable: "false"}}).AllowsPublicSchedule())
	require.False(t, (&Account{Extra: map[string]any{AccountExtraPublicSchedulable: "0"}}).AllowsPublicSchedule())
	require.True(t, (&Account{Extra: map[string]any{AccountExtraPublicSchedulable: "yes"}}).AllowsPublicSchedule())
	require.True(t, (&Account{Extra: map[string]any{AccountExtraPublicSchedulable: json.Number("1")}}).AllowsPublicSchedule())
	require.False(t, (&Account{Extra: map[string]any{AccountExtraPublicSchedulable: json.Number("0")}}).AllowsPublicSchedule())
}

func TestAccount_SetPublicSchedulableWritesBool(t *testing.T) {
	t.Parallel()
	acc := &Account{}
	acc.SetPublicSchedulable(true)
	require.Equal(t, true, acc.Extra[AccountExtraPublicSchedulable])
	require.True(t, acc.AllowsPublicSchedule())
	acc.SetPublicSchedulable(false)
	require.Equal(t, false, acc.Extra[AccountExtraPublicSchedulable])
	require.False(t, acc.AllowsPublicSchedule())
}

func TestAdmitsScheduleUser_PublicPoolOff(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	closed := &Account{
		ID:       7,
		Platform: PlatformAnthropic,
		Extra:    map[string]any{AccountExtraPublicSchedulable: false},
	}
	legacy := &Account{ID: 7, Platform: PlatformAnthropic}

	t.Run("missing extra key still admits unpooled users", func(t *testing.T) {
		t.Parallel()
		require.True(t, admitsScheduleUser(ctx, legacy, nil, nil))
	})

	t.Run("explicit false rejects unpooled users", func(t *testing.T) {
		t.Parallel()
		require.False(t, admitsScheduleUser(ctx, closed, nil, nil))
	})

	t.Run("in-pool user still admits when public pool is off", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
		require.True(t, admitsScheduleUser(ctx, closed, nil, lookup))
	})

	t.Run("legacy deny still applies when public pool is on", func(t *testing.T) {
		t.Parallel()
		denied := &Account{ID: 8, Platform: PlatformAnthropic, DenyUserIDs: []int64{16}}
		require.False(t, admitsScheduleUser(ctx, denied, nil, nil))
	})
}
