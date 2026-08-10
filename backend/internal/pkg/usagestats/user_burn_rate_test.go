package usagestats

import (
	"math"
	"testing"
)

func TestUserBurnRatePerHour(t *testing.T) {
	t.Parallel()

	almostEqual := func(got, want float64) bool {
		return math.Abs(got-want) < 1e-9
	}

	// $0.05 in 5 minutes → $0.60/h
	if got := UserBurnRatePerHour(0.05); !almostEqual(got, 0.6) {
		t.Fatalf("UserBurnRatePerHour(0.05) = %v, want 0.6", got)
	}
	if got := UserBurnRatePerHour(0); !almostEqual(got, 0) {
		t.Fatalf("UserBurnRatePerHour(0) = %v, want 0", got)
	}
	// $1 in 5 minutes → $12/h
	if got := UserBurnRatePerHour(1); !almostEqual(got, 12) {
		t.Fatalf("UserBurnRatePerHour(1) = %v, want 12", got)
	}
}
