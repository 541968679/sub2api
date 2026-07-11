package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResetDailyUsageCAS_StaleResetPreservesNewWindowUsage(t *testing.T) {
	ctx := context.Background()
	client := newSecuritySecretTestClient(t)

	user, err := client.User.Create().
		SetEmail("subscription-cas@example.com").
		SetPasswordHash("test-password-hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)
	group, err := client.Group.Create().
		SetName("subscription-cas-group").
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	oldWindowStart := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	newWindowStart := oldWindowStart.Add(24 * time.Hour)
	sub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(group.ID).
		SetStartsAt(oldWindowStart).
		SetExpiresAt(newWindowStart.Add(30 * 24 * time.Hour)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(oldWindowStart).
		SetNotes("").
		SetDailyWindowStart(oldWindowStart).
		SetDailyUsageUsd(10).
		Save(ctx)
	require.NoError(t, err)

	repo := NewUserSubscriptionRepository(client).(*userSubscriptionRepository)
	require.NoError(t, repo.ResetDailyUsageCAS(ctx, sub.ID, &oldWindowStart, newWindowStart))
	_, err = client.UserSubscription.UpdateOneID(sub.ID).SetDailyUsageUsd(3).Save(ctx)
	require.NoError(t, err)

	// A concurrent loser still carries oldWindowStart and must not clear usage
	// accumulated after the winning reset.
	require.NoError(t, repo.ResetDailyUsageCAS(ctx, sub.ID, &oldWindowStart, newWindowStart))
	got, err := client.UserSubscription.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.InDelta(t, 3, got.DailyUsageUsd, 1e-9)
	require.Equal(t, newWindowStart, got.DailyWindowStart.UTC())
}
