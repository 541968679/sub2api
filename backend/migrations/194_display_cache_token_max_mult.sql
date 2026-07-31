-- Display-layer cache amplify cap (M) per user; NULL = inherit global setting.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS display_cache_token_max_mult DOUBLE PRECISION DEFAULT NULL;

COMMENT ON COLUMN users.display_cache_token_max_mult IS
    'User override for display cache_read amplify cap M (NULL = inherit global display_cache_token_max_mult)';
