package service

import (
	"testing"
	"time"
)

func TestCreditCurveBucketKeyIgnoresLocationIdentity(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	sameOffset := time.FixedZone("same-offset", 8*60*60)

	a := time.Date(2026, 5, 6, 13, 16, 33, 0, shanghai)
	b := time.Date(2026, 5, 6, 13, 16, 33, 0, sameOffset)

	//nolint:staticcheck // Intentionally checks time.Location identity, not instant equality.
	if creditCurveBucketStart(a, creditCurveGranularityHour) == creditCurveBucketStart(b, creditCurveGranularityHour) {
		t.Fatal("test setup expected time.Time equality to differ by location identity")
	}
	if creditCurveBucketKey(a, creditCurveGranularityHour) != creditCurveBucketKey(b, creditCurveGranularityHour) {
		t.Fatal("same instant and offset should map to the same hourly bucket key")
	}
	if creditCurveBucketKey(a, creditCurveGranularityDay) != creditCurveBucketKey(b, creditCurveGranularityDay) {
		t.Fatal("same local day and offset should map to the same daily bucket key")
	}
}

func TestDistributeCreditDeltaUsesUsageWeightAcrossSampleInterval(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	start := time.Date(2026, 5, 6, 9, 0, 0, 0, loc)
	points := []AntigravityCreditCurvePoint{
		{
			Start:        start,
			End:          start.Add(time.Hour),
			QuotaUsedUSD: 30,
		},
		{
			Start:        start.Add(time.Hour),
			End:          start.Add(2 * time.Hour),
			QuotaUsedUSD: 10,
		},
	}

	prev := CreditSnapshot{CapturedAt: start.Add(50 * time.Minute), CreditType: "ai", Amount: 1000}
	curr := CreditSnapshot{CapturedAt: start.Add(time.Hour + 10*time.Minute), CreditType: "ai", Amount: 600}

	distributeCreditDelta(points, start, start.Add(2*time.Hour), creditCurveGranularityHour, prev, curr, 400)

	if got := points[0].CreditsConsumed; got != 300 {
		t.Fatalf("expected first hour to receive weighted credits 300, got %.2f", got)
	}
	if got := points[1].CreditsConsumed; got != 100 {
		t.Fatalf("expected second hour to receive weighted credits 100, got %.2f", got)
	}
	if got := points[0].CreditsByType["ai"]; got != 300 {
		t.Fatalf("expected credit type split for first hour 300, got %.2f", got)
	}
}

func TestDistributeCreditDeltaFallsBackToSnapshotBucketWithoutUsage(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	start := time.Date(2026, 5, 6, 9, 0, 0, 0, loc)
	points := []AntigravityCreditCurvePoint{
		{Start: start, End: start.Add(time.Hour)},
		{Start: start.Add(time.Hour), End: start.Add(2 * time.Hour)},
	}

	prev := CreditSnapshot{CapturedAt: start.Add(5 * time.Minute), CreditType: "ai", Amount: 1000}
	curr := CreditSnapshot{CapturedAt: start.Add(time.Hour + 10*time.Minute), CreditType: "ai", Amount: 900}

	distributeCreditDelta(points, start, start.Add(2*time.Hour), creditCurveGranularityHour, prev, curr, 100)

	if got := points[0].CreditsConsumed; got != 0 {
		t.Fatalf("expected first hour to remain empty, got %.2f", got)
	}
	if got := points[1].CreditsConsumed; got != 100 {
		t.Fatalf("expected fallback to current snapshot hour, got %.2f", got)
	}
}
