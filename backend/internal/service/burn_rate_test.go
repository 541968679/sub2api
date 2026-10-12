//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComputeBurnRate_LinearDecline(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	samples := []BurnSample{
		{T: now.Add(-12 * time.Minute), V: 82, Kind: burnKindBalanceUSD},
		{T: now.Add(-6 * time.Minute), V: 81, Kind: burnKindBalanceUSD},
		{T: now, V: 80, Kind: burnKindBalanceUSD},
	}
	r := ComputeBurnRate(samples, now, 0)
	require.False(t, r.Insufficient)
	require.False(t, r.Idle)
	require.InDelta(t, 10.0, r.RatePerHour, 0.01)
	require.NotNil(t, r.ETASeconds)
	// remaining 80 / 10 per hour = 8h
	require.InDelta(t, 8*3600, *r.ETASeconds, 1)
}

func TestComputeBurnRate_InsufficientPoints(t *testing.T) {
	t.Parallel()
	now := time.Now()
	r := ComputeBurnRate([]BurnSample{{T: now, V: 10}}, now, time.Minute)
	require.True(t, r.Insufficient)
}

func TestComputeBurnRate_MinSpan(t *testing.T) {
	t.Parallel()
	now := time.Now()
	samples := []BurnSample{
		{T: now.Add(-5 * time.Minute), V: 100},
		{T: now, V: 90},
	}
	r := ComputeBurnRate(samples, now, 10*time.Minute)
	require.True(t, r.Insufficient)
}

func TestComputeBurnRate_Idle(t *testing.T) {
	t.Parallel()
	now := time.Now()
	samples := []BurnSample{
		{T: now.Add(-10 * time.Minute), V: 50},
		{T: now, V: 50},
	}
	r := ComputeBurnRate(samples, now, 10*time.Minute)
	require.False(t, r.Insufficient)
	require.True(t, r.Idle)
	require.Equal(t, 0.0, r.RatePerHour)
}

func TestComputeBurnRate_DropsSamplesOutsideLookback(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	samples := []BurnSample{
		{T: now.Add(-20 * 24 * time.Hour), V: 1000},
		{T: now.Add(-10 * 24 * time.Hour), V: 800},
		{T: now.Add(-30 * time.Minute), V: 790},
	}
	r := ComputeBurnRate(samples, now, 0)
	require.True(t, r.Insufficient)
}

func TestComputeBurnRate_ShortBurstNotHourly(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	samples := []BurnSample{
		{T: now.Add(-3 * time.Minute), V: 100},
		{T: now, V: 90},
	}
	r := ComputeBurnRate(samples, now, 0)
	require.True(t, r.Insufficient)
}

func TestComputeBurnRate_SecantOverRecentWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	samples := []BurnSample{
		{T: now.Add(-2 * time.Hour), V: 1000},
		{T: now.Add(-12 * time.Minute), V: 100},
		{T: now.Add(-2 * time.Minute), V: 80},
		{T: now, V: 80},
	}
	r := ComputeBurnRate(samples, now, 0)
	require.False(t, r.Insufficient)
	require.InDelta(t, 100.0, r.RatePerHour, 0.01)
	require.NotNil(t, r.ETASeconds)
	require.InDelta(t, 0.8*3600, *r.ETASeconds, 1)
}

func TestComputeBurnRate_TinyIncreaseDoesNotResetEpoch(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	samples := []BurnSample{
		{T: now.Add(-12 * time.Minute), V: 100},
		{T: now.Add(-6 * time.Minute), V: 100.02},
		{T: now, V: 99},
	}
	r := ComputeBurnRate(samples, now, 0)
	require.False(t, r.Insufficient)
	require.InDelta(t, 5.0, r.RatePerHour, 0.01)
}

func TestAppendBurnSample_ResetOnIncrease(t *testing.T) {
	t.Parallel()
	now := time.Now()
	samples := []BurnSample{
		{T: now.Add(-2 * time.Hour), V: 10, Kind: burnKindBalanceUSD},
		{T: now.Add(-1 * time.Hour), V: 5, Kind: burnKindBalanceUSD},
	}
	// Recharge
	samples = AppendBurnSample(samples, BurnSample{T: now, V: 100, Kind: burnKindBalanceUSD}, 12)
	require.Len(t, samples, 1)
	require.Equal(t, 100.0, samples[0].V)
}

func TestAppendBurnSample_KeepsWindowOnTinyIncrease(t *testing.T) {
	t.Parallel()
	now := time.Now()
	samples := []BurnSample{
		{T: now.Add(-2 * time.Hour), V: 10, Kind: burnKindBalanceUSD},
	}
	samples = AppendBurnSample(samples, BurnSample{T: now, V: 10.01, Kind: burnKindBalanceUSD}, 12)
	require.Len(t, samples, 2)
	require.Equal(t, 10.01, samples[1].V)
}

func TestRemainingPctFromUtilization(t *testing.T) {
	t.Parallel()
	require.Equal(t, 40.0, RemainingPctFromUtilization(60))
	require.Equal(t, 0.0, RemainingPctFromUtilization(120))
	require.Equal(t, 100.0, RemainingPctFromUtilization(-5))
}

func TestJoinOpenAIBillingURL(t *testing.T) {
	t.Parallel()
	require.Equal(t,
		"https://api.openai.com/v1/dashboard/billing/credit_grants",
		JoinOpenAIBillingURL("https://api.openai.com", "/v1/dashboard/billing/credit_grants"),
	)
	require.Equal(t,
		"https://proxy.example/v1/dashboard/billing/credit_grants",
		JoinOpenAIBillingURL("https://proxy.example/v1", "/v1/dashboard/billing/credit_grants"),
	)
}

func TestSerializeParseBurnSamplesRoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 3, 1, 2, 3, 0, time.UTC)
	samples := []BurnSample{{T: now, V: 12.5, Kind: burnKindBalanceUSD}}
	extra := map[string]any{extraKeyBurnSamples: SerializeBurnSamples(samples)}
	got := ParseBurnSamplesFromExtra(extra, burnKindBalanceUSD)
	require.Len(t, got, 1)
	require.Equal(t, 12.5, got[0].V)
	require.True(t, got[0].T.Equal(now))
}
