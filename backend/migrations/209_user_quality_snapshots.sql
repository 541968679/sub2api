-- Periodic last-N user quality window snapshots (5-minute cadence, 7-day retention).
-- Same contract as live user list quality-stats; empty windows are not inserted by the job.

CREATE TABLE IF NOT EXISTS user_quality_snapshots (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL,
    window_seconds INTEGER NOT NULL DEFAULT 900,
    success_count BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    ttft_samples BIGINT NOT NULL DEFAULT 0,
    success_rate DOUBLE PRECISION,
    avg_ttft_ms INTEGER,
    p50_ttft_ms INTEGER,
    p95_ttft_ms INTEGER,
    max_ttft_ms INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS user_quality_snapshots_user_id_captured_at
    ON user_quality_snapshots (user_id, captured_at);

CREATE INDEX IF NOT EXISTS user_quality_snapshots_captured_at
    ON user_quality_snapshots (captured_at);

CREATE INDEX IF NOT EXISTS user_quality_snapshots_user_id
    ON user_quality_snapshots (user_id);
