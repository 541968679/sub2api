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
