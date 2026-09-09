-- Standing replica window-1: per-group profit control columns (default OFF).
-- Renumbered from catchup 202 / standing 212. Additive only; enabling the gate is opt-in.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS profit_control_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS profit_min_margin DECIMAL(10,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS profit_safety_buffer DECIMAL(10,4) NOT NULL DEFAULT 0;

COMMENT ON COLUMN groups.profit_control_enabled IS 'When true, scheduler filters token accounts by profit margin rule';
COMMENT ON COLUMN groups.profit_min_margin IS 'Minimum margin fraction for profit control (0-1)';
COMMENT ON COLUMN groups.profit_safety_buffer IS 'Extra safety buffer fraction for profit control (0-1)';
