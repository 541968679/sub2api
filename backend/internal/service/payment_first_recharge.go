package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// HasCompletedBalanceOrder checks if a user has any completed balance recharge orders.
func (s *PaymentService) HasCompletedBalanceOrder(ctx context.Context, userID int64) bool {
	n, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.UserIDEQ(userID),
			paymentorder.OrderTypeEQ(payment.OrderTypeBalance),
			paymentorder.StatusIn(OrderStatusCompleted, OrderStatusPaid, OrderStatusRecharging),
		).
		Limit(1).
		Count(ctx)
	return err == nil && n > 0
}
