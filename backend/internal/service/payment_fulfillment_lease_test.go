//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestPaymentFulfillmentLeaseRejectsFreshOwner(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createLeaseTestOrder(t, ctx, client, time.Now().UTC())

	_, err := (&PaymentService{entClient: client}).acquirePaymentFulfillmentLease(ctx, order)
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))
}

func TestPaymentFulfillmentLeaseAllowsStaleTakeoverAndRejectsOldOwner(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	staleAt := time.Now().UTC().Add(-paymentFulfillmentLeaseDuration - time.Minute)
	order := createLeaseTestOrder(t, ctx, client, staleAt)
	svc := &PaymentService{entClient: client}

	lease, err := svc.acquirePaymentFulfillmentLease(ctx, order)
	require.NoError(t, err)
	require.NotNil(t, lease)

	oldLease := &paymentFulfillmentLease{version: staleAt}
	err = svc.markCompleted(ctx, order, oldLease, "RECHARGE_SUCCESS")
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))
	require.NoError(t, svc.markCompleted(ctx, order, lease, "RECHARGE_SUCCESS"))
}

func createLeaseTestOrder(t *testing.T, ctx context.Context, client *dbent.Client, updatedAt time.Time) *dbent.PaymentOrder {
	t.Helper()
	user, err := client.User.Create().
		SetEmail("lease-" + time.Now().Format("150405.000000000") + "@example.com").
		SetPasswordHash("hash").
		SetUsername("lease-user").
		Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(10).
		SetPayAmount(10).
		SetFeeRate(0).
		SetRechargeCode("LEASE-CODE").
		SetOutTradeNo("lease-" + time.Now().Format("150405.000000000")).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-lease").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRecharging).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.test").
		SetUpdatedAt(updatedAt).
		Save(ctx)
	require.NoError(t, err)
	return order
}
