package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubConsumedReader struct {
	totals map[int64]float64
	err    error
}

func (s *stubConsumedReader) SumConsumedUSDBySubscriptionIDs(_ context.Context, _ []int64) (map[int64]float64, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.totals, nil
}

type stubGroupRateBatch struct {
	UserGroupRateRepository
	byUser map[int64]map[int64]UserGroupRateData
	err    error
}

func (s *stubGroupRateBatch) GetFullByUserIDs(_ context.Context, _ []int64) (map[int64]map[int64]UserGroupRateData, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.byUser, nil
}

func TestSubscriptionActiveDays(t *testing.T) {
	start := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	now := start.Add(2*time.Hour + 30*time.Minute)
	require.Equal(t, 1, subscriptionActiveDays(start, start.Add(30*24*time.Hour), now))

	now = start.Add(25 * time.Hour)
	require.Equal(t, 2, subscriptionActiveDays(start, start.Add(30*24*time.Hour), now))

	// expired early
	expires := start.Add(10 * time.Hour)
	require.Equal(t, 1, subscriptionActiveDays(start, expires, start.Add(48*time.Hour)))
}

func TestEnrichAdminListStats(t *testing.T) {
	rate := 1.5
	display := 2.0
	dailyLimit := 400.0
	start := time.Now().Add(-3 * 24 * time.Hour)

	svc := &SubscriptionService{
		consumedReader: &stubConsumedReader{totals: map[int64]float64{11: 900}},
		userGroupRateRepo: &stubGroupRateBatch{
			byUser: map[int64]map[int64]UserGroupRateData{
				7: {
					9: {
						RateMultiplier:        &rate,
						DisplayRateMultiplier: &display,
					},
				},
			},
		},
	}

	subs := []UserSubscription{
		{
			ID:      11,
			UserID:  7,
			GroupID: 9,
			StartsAt: start,
			ExpiresAt: start.Add(30 * 24 * time.Hour),
			Group: &Group{DailyLimitUSD: &dailyLimit},
		},
	}

	svc.enrichAdminListStats(context.Background(), subs)

	require.Equal(t, 900.0, subs[0].TotalConsumedUSD)
	require.Equal(t, 4, subs[0].ActiveDays) // floor(3d)+1
	require.InDelta(t, 225.0, subs[0].AvgDailyUsageUSD, 0.001)
	require.NotNil(t, subs[0].DailyUsageRate)
	require.InDelta(t, 225.0/400.0, *subs[0].DailyUsageRate, 0.001)
	require.NotNil(t, subs[0].UserRateMultiplier)
	require.Equal(t, 1.5, *subs[0].UserRateMultiplier)
	require.NotNil(t, subs[0].UserDisplayRateMultiplier)
	require.Equal(t, 2.0, *subs[0].UserDisplayRateMultiplier)
}
