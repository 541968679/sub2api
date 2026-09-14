package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountQualifiesForProfit_DefaultOffAlwaysAllows(t *testing.T) {
	p := DefaultProfitControlPolicy()
	require.False(t, p.Enabled)
	require.True(t, AccountQualifiesForProfit(100, 1, p))
	require.True(t, AccountQualifiesForProfit(math.NaN(), 1, p))
	require.True(t, AccountQualifiesForProfit(-1, 0, p))
}

func TestAccountQualifiesForProfit_EnabledFiltersByMargin(t *testing.T) {
	p := ProfitControlPolicy{Enabled: true, MinMargin: 0.2, SafetyBuffer: 0.05}
	// threshold = D * (1 - 0.2 - 0.05) = 0.75 * D
	require.True(t, AccountQualifiesForProfit(0.5, 1.0, p))
	require.True(t, AccountQualifiesForProfit(0.75, 1.0, p))
	require.False(t, AccountQualifiesForProfit(0.9, 1.0, p))
	require.False(t, AccountQualifiesForProfit(math.NaN(), 1.0, p))
	require.False(t, AccountQualifiesForProfit(-0.1, 1.0, p))
}

func TestFilterAccountsByProfitControl_DefaultOffNoFilter(t *testing.T) {
	high := 10.0
	accounts := []*Account{
		{ID: 1, RateMultiplier: &high},
		{ID: 2},
	}
	got := FilterAccountsByProfitControl(accounts, 1.0, DefaultProfitControlPolicy())
	require.Equal(t, accounts, got)
}

func TestFilterAccountsByProfitControl_EnabledDropsUnprofitable(t *testing.T) {
	low, high := 0.5, 2.0
	accounts := []*Account{
		{ID: 1, RateMultiplier: &low},
		{ID: 2, RateMultiplier: &high},
	}
	p := ProfitControlPolicy{Enabled: true, MinMargin: 0.2, SafetyBuffer: 0.05}
	got := FilterAccountsByProfitControl(accounts, 1.0, p)
	require.Len(t, got, 1)
	require.Equal(t, int64(1), got[0].ID)
}
