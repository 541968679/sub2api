ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS blocked_models JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS allowed_models JSONB NOT NULL DEFAULT '[]'::jsonb;
