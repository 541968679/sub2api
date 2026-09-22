package loadtest

import (
	"math"
	"testing"
	"time"
)

func rpmRow(finish time.Time, durationMs int) Result {
	start := finish.Add(-time.Duration(durationMs) * time.Millisecond)
	return Result{StartedAt: start.Format(time.RFC3339Nano), DurationMs: durationMs, Outcome: "success"}
}

func TestComputeRPMRollingAndPeak(t *testing.T) {
	base := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	rows := make([]Result, 0, 12)
	for i := 0; i < 10; i++ {
		rows = append(rows, rpmRow(base.Add(time.Second), 200))
	}
	rows = append(rows, rpmRow(base.Add(90*time.Second), 100), rpmRow(base.Add(90*time.Second), 100))

	got := ComputeRPM(rows, base, base.Add(90*time.Second))
	if got.Current != 2 {
		t.Fatalf("current %d", got.Current)
	}
	if got.Peak != 10 {
		t.Fatalf("peak %d", got.Peak)
	}
	if math.Abs(got.Average-8) > 0.01 {
		t.Fatalf("avg %v", got.Average)
	}
}

func TestComputeRPMIncludesBoundary(t *testing.T) {
	base := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	rows := []Result{rpmRow(base, 0)}
	got := ComputeRPM(rows, base, base.Add(rpmWindow))
	if got.Current != 1 || got.Peak != 1 {
		t.Fatalf("%+v", got)
	}
	got = ComputeRPM(rows, base, base.Add(rpmWindow+time.Millisecond))
	if got.Current != 0 {
		t.Fatalf("just outside window: %+v", got)
	}
}

func TestComputeRPMIgnoresFailures(t *testing.T) {
	base := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	ok := rpmRow(base.Add(time.Second), 100)
	failed := rpmRow(base.Add(time.Second), 100)
	failed.Outcome = "http_error"
	got := ComputeRPM([]Result{ok, failed}, base, base.Add(time.Second))
	if got.Current != 1 || got.Peak != 1 {
		t.Fatalf("%+v", got)
	}
	if math.Abs(got.Average-60) > 0.01 {
		t.Fatalf("avg %v", got.Average)
	}
}

func TestComputeRPMEmpty(t *testing.T) {
	got := ComputeRPM(nil, time.Now(), time.Now())
	if got.Current != 0 || got.Peak != 0 || got.Average != 0 {
		t.Fatalf("%+v", got)
	}
}
