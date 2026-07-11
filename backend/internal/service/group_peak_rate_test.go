package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

func init() { _ = timezone.Init("UTC") }

func newPeakGroup(enabled bool, start, end string, mult float64) *Group {
	return &Group{
		SubscriptionType: SubscriptionTypeSubscription,
		PeakRateEnabled: enabled, PeakStart: start, PeakEnd: end, PeakRateMultiplier: mult,
	}
}

func peakTestTime(hour, minute int) time.Time {
	return time.Date(2026, 6, 29, hour, minute, 0, 0, time.UTC)
}

func TestPeakMultiplierAtBoundariesAndInvalidConfig(t *testing.T) {
	g := newPeakGroup(true, "14:00", "18:00", 3)
	for _, tc := range []struct {
		name string
		now time.Time
		want float64
	}{
		{"before", peakTestTime(13, 59), 1},
		{"start inclusive", peakTestTime(14, 0), 3},
		{"end exclusive", peakTestTime(18, 0), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := g.PeakMultiplierAt(tc.now); got != tc.want { t.Fatalf("got %v want %v", got, tc.want) }
		})
	}
	for _, invalid := range []*Group{
		nil,
		newPeakGroup(false, "14:00", "18:00", 3),
		newPeakGroup(true, "22:00", "02:00", 3),
		newPeakGroup(true, "99:99", "18:00", 3),
	} {
		if got := invalid.PeakMultiplierAt(peakTestTime(15, 0)); got != 1 { t.Fatalf("invalid config got %v", got) }
	}
}

func TestValidateAndNormalizePeakRateConfig(t *testing.T) {
	if err := ValidatePeakRateConfig(SubscriptionTypeSubscription, true, "14:00", "18:00", 0); err != nil {
		t.Fatalf("zero multiplier must be accepted: %v", err)
	}
	for _, tc := range []struct{ typ, start, end string; mult float64 }{
		{SubscriptionTypeStandard, "14:00", "18:00", 1},
		{SubscriptionTypeSubscription, "22:00", "02:00", 1},
		{SubscriptionTypeSubscription, "14:00", "18:00", -1},
	} {
		if err := ValidatePeakRateConfig(tc.typ, true, tc.start, tc.end, tc.mult); err == nil { t.Fatal("expected invalid config") }
	}
	enabled, start, end, mult := NormalizePeakRateConfig(SubscriptionTypeStandard, true, "14:00", "18:00", 3)
	if enabled || start != "" || end != "" || mult != 1 { t.Fatalf("standard config not cleared: %v %q %q %v", enabled, start, end, mult) }
}

func TestPeakMultiplierOnlyAffectsTokenBilling(t *testing.T) {
	key := &APIKey{Group: newPeakGroup(true, "14:00", "18:00", 3)}
	text, image := computePeakAwareMultipliers(key, 0.8, peakTestTime(15, 0))
	if math.Abs(text-2.4) > 1e-9 || math.Abs(image-0.8) > 1e-9 { t.Fatalf("got text=%v image=%v", text, image) }
}

func TestPeakMultiplierSnapshotRoundTrip(t *testing.T) {
	key := &APIKey{User: &User{ID: 1, Status: StatusActive, Role: RoleUser}, Group: newPeakGroup(true, "14:00", "18:00", 3)}
	svc := &APIKeyService{}
	restored := svc.snapshotToAPIKey("k", svc.snapshotFromAPIKey(context.Background(), key))
	if restored.Group == nil || restored.Group.PeakMultiplierAt(peakTestTime(15, 0)) != 3 { t.Fatalf("peak snapshot lost: %+v", restored.Group) }
}
