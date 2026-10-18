//go:build unit

package service

import "testing"

func TestResolvedHasBillablePrice(t *testing.T) {
	if resolvedHasBillablePrice(nil) {
		t.Fatal("nil pricing is not billable")
	}
	if resolvedHasBillablePrice(&ResolvedPricing{Mode: BillingModeToken}) {
		t.Fatal("token mode without a price is not billable")
	}
	if !resolvedHasBillablePrice(&ResolvedPricing{
		Mode:        BillingModeToken,
		BasePricing: &ModelPricing{InputPricePerToken: 0.000001},
	}) {
		t.Fatal("positive input price is billable")
	}
	if resolvedHasBillablePrice(&ResolvedPricing{
		Mode:        BillingModeToken,
		BasePricing: &ModelPricing{},
	}) {
		t.Fatal("zero token prices are not billable")
	}
	price := 0.02
	if !resolvedHasBillablePrice(&ResolvedPricing{
		Mode:                   BillingModeImage,
		DefaultPerRequestPrice: price,
	}) {
		t.Fatal("image per-request price is billable")
	}
}
