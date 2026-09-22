package loadtest

import (
	"sort"
	"time"
)

const rpmWindow = 60 * time.Second

// RPMStats is successful requests per minute.
// Current counts successes in the 60s ending at the anchor.
// Peak is the busiest 60s window ending on a success.
// Average is successes divided by elapsed minutes. Failures are omitted.
type RPMStats struct {
	Current int
	Peak    int
	Average float64
}

// ComputeRPM counts rows whose outcome is success.
// Finish time is started_at plus duration_ms.
func ComputeRPM(rows []Result, started, anchor time.Time) RPMStats {
	var stats RPMStats
	if len(rows) == 0 || anchor.IsZero() {
		return stats
	}
	successes := 0
	finishes := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		if row.Outcome != "success" {
			continue
		}
		successes++
		if t, ok := resultFinishTime(row); ok {
			finishes = append(finishes, t)
		}
	}
	sort.Slice(finishes, func(i, j int) bool { return finishes[i].Before(finishes[j]) })
	stats.Current = countRPMWindow(finishes, anchor)
	stats.Peak = peakRPMWindow(finishes)
	if successes > 0 && !anchor.Before(started) {
		elapsed := anchor.Sub(started)
		if elapsed < time.Second {
			elapsed = time.Second
		}
		stats.Average = float64(successes) * float64(time.Minute) / float64(elapsed)
	}
	return stats
}

func resultFinishTime(row Result) (time.Time, bool) {
	if row.StartedAt == "" {
		return time.Time{}, false
	}
	start, err := time.Parse(time.RFC3339Nano, row.StartedAt)
	if err != nil {
		start, err = time.Parse(time.RFC3339, row.StartedAt)
		if err != nil {
			return time.Time{}, false
		}
	}
	if row.DurationMs < 0 {
		return start, true
	}
	return start.Add(time.Duration(row.DurationMs) * time.Millisecond), true
}

func countRPMWindow(sorted []time.Time, anchor time.Time) int {
	cutoff := anchor.Add(-rpmWindow)
	n := 0
	for _, t := range sorted {
		if !t.Before(cutoff) && !t.After(anchor) {
			n++
		}
	}
	return n
}

func peakRPMWindow(sorted []time.Time) int {
	if len(sorted) == 0 {
		return 0
	}
	peak := 1
	j := 0
	for i := range sorted {
		cutoff := sorted[i].Add(-rpmWindow)
		for j < i && sorted[j].Before(cutoff) {
			j++
		}
		if n := i - j + 1; n > peak {
			peak = n
		}
	}
	return peak
}
