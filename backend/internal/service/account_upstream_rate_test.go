package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultUpstreamRateMultiplier(t *testing.T) {
	require.Equal(t, 0.15, DefaultUpstreamRateMultiplier(AccountTypeOAuth))
	require.Equal(t, 0.15, DefaultUpstreamRateMultiplier(AccountTypeAPIKey))
	require.Equal(t, 1.0, DefaultUpstreamRateMultiplier(AccountTypeSetupToken))
	require.Equal(t, 1.0, DefaultUpstreamRateMultiplier(AccountTypeUpstream))
	require.Equal(t, 1.0, DefaultUpstreamRateMultiplier(AccountTypeBedrock))
	require.Equal(t, 1.0, DefaultUpstreamRateMultiplier(AccountTypeServiceAccount))
	require.Equal(t, 1.0, DefaultUpstreamRateMultiplier(""))
}

func TestEffectiveUpstreamRate_NilUsesTypeDefault(t *testing.T) {
	oauth := &Account{Type: AccountTypeOAuth}
	require.Equal(t, 0.15, oauth.EffectiveUpstreamRate())
	setup := &Account{Type: AccountTypeSetupToken}
	require.Equal(t, 1.0, setup.EffectiveUpstreamRate())
}

func TestEffectiveUpstreamRate_OldSnapshotMissingField(t *testing.T) {
	var a Account
	require.NoError(t, json.Unmarshal([]byte(`{"id":1,"name":"acc","type":"oauth","status":"active"}`), &a))
	require.Nil(t, a.UpstreamRateMultiplier)
	require.Equal(t, 0.15, a.EffectiveUpstreamRate())
	require.Equal(t, 1.0, a.BillingRateMultiplier())
}

func TestEffectiveUpstreamRate_AllowsZeroAndRejectsNegative(t *testing.T) {
	zero := 0.0
	neg := -1.0
	require.Equal(t, 0.0, (&Account{Type: AccountTypeOAuth, UpstreamRateMultiplier: &zero}).EffectiveUpstreamRate())
	require.Equal(t, 0.15, (&Account{Type: AccountTypeOAuth, UpstreamRateMultiplier: &neg}).EffectiveUpstreamRate())
}

func TestBillingRateMultiplier_IndependentOfUpstreamRate(t *testing.T) {
	billing := 2.0
	upstream := 0.15
	a := &Account{RateMultiplier: &billing, UpstreamRateMultiplier: &upstream, Type: AccountTypeOAuth}
	require.Equal(t, 2.0, a.BillingRateMultiplier())
	require.Equal(t, 0.15, a.EffectiveUpstreamRate())
}
