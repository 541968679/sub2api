-- User-level OpenAI header-wait / first-useful-frame timeout override.
-- NULL = inherit site openai_wait_timeout_settings. 0 = disable that gate.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS openai_header_wait_seconds INTEGER DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS openai_first_useful_frame_seconds INTEGER DEFAULT NULL;

COMMENT ON COLUMN users.openai_header_wait_seconds IS
    'User override for OpenAI header-wait seconds; NULL inherits site settings; 0 disables';
COMMENT ON COLUMN users.openai_first_useful_frame_seconds IS
    'User override for OpenAI first-useful-frame seconds; NULL inherits site settings; 0 disables';
