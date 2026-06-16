package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

func TestDetermineSubscriptionProfitSource(t *testing.T) {
	adminID := int64(9)

	tests := []struct {
		name string
		raw  usagestats.SubscriptionProfitRaw
		want string
	}{
		{
			name: "paid order wins",
			raw: usagestats.SubscriptionProfitRaw{
				HasPaidOrder: true,
				AssignedBy:   &adminID,
				Notes:        "通过兑换码 ABC 兑换",
			},
			want: "paid",
		},
		{
			name: "redeem note",
			raw:  usagestats.SubscriptionProfitRaw{Notes: "通过兑换码 ABC 兑换"},
			want: "redeem",
		},
		{
			name: "default subscription note",
			raw:  usagestats.SubscriptionProfitRaw{Notes: "auto assigned by default user subscriptions setting"},
			want: "default",
		},
		{
			name: "admin assignment",
			raw:  usagestats.SubscriptionProfitRaw{AssignedBy: &adminID},
			want: "admin",
		},
		{
			name: "system fallback",
			raw:  usagestats.SubscriptionProfitRaw{},
			want: "system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := determineSubscriptionProfitSource(tt.raw); got != tt.want {
				t.Fatalf("determineSubscriptionProfitSource() = %q, want %q", got, tt.want)
			}
		})
	}
}
