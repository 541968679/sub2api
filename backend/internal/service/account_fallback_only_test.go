package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountIsFallbackOnly(t *testing.T) {
	require.False(t, (*Account)(nil).IsFallbackOnly())
	require.False(t, (&Account{}).IsFallbackOnly())
	require.False(t, (&Account{Extra: map[string]any{}}).IsFallbackOnly())
	require.True(t, (&Account{Extra: map[string]any{AccountExtraFallbackOnly: true}}).IsFallbackOnly())
	require.False(t, (&Account{Extra: map[string]any{AccountExtraFallbackOnly: false}}).IsFallbackOnly())
}

func TestAccountSetFallbackOnly(t *testing.T) {
	a := &Account{}
	a.SetFallbackOnly(true)
	require.True(t, a.IsFallbackOnly())
	require.Equal(t, true, a.Extra[AccountExtraFallbackOnly])

	a.SetFallbackOnly(false)
	require.False(t, a.IsFallbackOnly())
	_, exists := a.Extra[AccountExtraFallbackOnly]
	require.False(t, exists)
}

func TestPreferPrimaryAccounts(t *testing.T) {
	primaryA := &Account{ID: 1, Priority: 1}
	primaryB := &Account{ID: 2, Priority: 2}
	fallback := &Account{ID: 3, Priority: 99, Extra: map[string]any{AccountExtraFallbackOnly: true}}

	t.Run("primary preferred over fallback", func(t *testing.T) {
		got := preferPrimaryAccounts([]*Account{fallback, primaryA, primaryB})
		require.Len(t, got, 2)
		require.Equal(t, int64(1), got[0].ID)
		require.Equal(t, int64(2), got[1].ID)
	})

	t.Run("fallback only when no primary", func(t *testing.T) {
		got := preferPrimaryAccounts([]*Account{fallback})
		require.Len(t, got, 1)
		require.Equal(t, int64(3), got[0].ID)
	})

	t.Run("empty", func(t *testing.T) {
		require.Empty(t, preferPrimaryAccounts(nil))
	})
}

func TestPreferPrimaryOpenAICandidates(t *testing.T) {
	primary := openAIAccountCandidateScore{account: &Account{ID: 10}}
	fallback := openAIAccountCandidateScore{account: &Account{ID: 20, Extra: map[string]any{AccountExtraFallbackOnly: true}}}

	got := preferPrimaryOpenAICandidates([]openAIAccountCandidateScore{fallback, primary})
	require.Len(t, got, 1)
	require.Equal(t, int64(10), got[0].account.ID)

	got = preferPrimaryOpenAICandidates([]openAIAccountCandidateScore{fallback})
	require.Len(t, got, 1)
	require.Equal(t, int64(20), got[0].account.ID)
}
