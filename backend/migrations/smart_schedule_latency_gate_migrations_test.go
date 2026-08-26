package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSmartScheduleLatencyGateMigrationsKeepHistoricalDown(t *testing.T) {
	for _, name := range []string{
		"214_smart_schedule_latency_gate.sql",
		"215_smart_schedule_sched_latency_gate.sql",
	} {
		raw, err := FS.ReadFile(name)
		require.NoError(t, err)
		sql := string(raw)
		require.Contains(t, sql, "-- +goose Up")
		require.Contains(t, sql, "-- +goose Down")
		require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS")
		require.Contains(t, sql, "DROP COLUMN IF EXISTS")
	}
}

func TestSmartScheduleLatencyGateRepair216IsIdempotentAndHasNoDown(t *testing.T) {
	raw, err := FS.ReadFile("216_smart_schedule_latency_gate_repair.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.NotContains(t, strings.ToLower(sql), "+goose down")
	require.NotContains(t, sql, "DROP COLUMN")
	for _, col := range []string{
		"quality_max_slow_in_window",
		"quality_max_consecutive_slow",
		"quality_max_p50_duration_ms",
		"quality_sched_window_n",
		"quality_sched_max_slow_in_window",
		"quality_sched_max_consecutive_slow",
	} {
		require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS "+col)
	}
	require.Contains(t, sql, "zuoge85@gmail.com")
	require.Contains(t, sql, "80000")
	require.Contains(t, sql, "quality_max_p50_duration_ms IS NULL")
}

func TestSmartScheduleSoftCooldown218(t *testing.T) {
	raw, err := FS.ReadFile("218_smart_schedule_soft_cooldown.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.Contains(t, sql, "-- +goose Up")
	require.Contains(t, sql, "-- +goose Down")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS soft_cooldown BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "DROP COLUMN IF EXISTS soft_cooldown")
}

func TestSmartScheduleZuogeSchedComposite217(t *testing.T) {
	raw, err := FS.ReadFile("217_smart_schedule_zuoge_sched_composite.sql")
	require.NoError(t, err)
	sql := string(raw)
	require.NotContains(t, strings.ToLower(sql), "+goose down")
	require.NotContains(t, sql, "DROP COLUMN")
	require.Contains(t, sql, "zuoge85@gmail.com")
	require.Contains(t, sql, "quality_sched_window_n = 10")
	require.Contains(t, sql, "quality_sched_max_slow_in_window = 4")
	require.Contains(t, sql, "quality_sched_max_consecutive_slow = 2")
	require.Contains(t, sql, "quality_sched_window_n IS NULL")
}
