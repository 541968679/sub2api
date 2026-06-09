-- Add immutable long-context pricing snapshot columns to usage logs.
-- These fields let display transforms use the effective per-request display
-- price without re-evaluating model pricing rules for historical rows.
ALTER TABLE usage_logs
	ADD COLUMN IF NOT EXISTS long_context_applied BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN IF NOT EXISTS long_context_input_threshold INT,
	ADD COLUMN IF NOT EXISTS long_context_input_multiplier NUMERIC(10,4),
	ADD COLUMN IF NOT EXISTS long_context_output_multiplier NUMERIC(10,4);

COMMENT ON COLUMN usage_logs.long_context_applied IS 'Whether long-context pricing was applied to this usage record';
COMMENT ON COLUMN usage_logs.long_context_input_threshold IS 'Input-side token threshold used when long-context pricing applied';
COMMENT ON COLUMN usage_logs.long_context_input_multiplier IS 'Input/cache multiplier used when long-context pricing applied';
COMMENT ON COLUMN usage_logs.long_context_output_multiplier IS 'Output multiplier used when long-context pricing applied';
