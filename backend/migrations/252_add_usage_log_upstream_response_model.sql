ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS upstream_response_model VARCHAR(200),
    ADD COLUMN IF NOT EXISTS upstream_model_mismatch BOOLEAN;

COMMENT ON COLUMN usage_logs.upstream_response_model IS 'Model name declared by the upstream response body. NULL means the response did not declare a model.';
COMMENT ON COLUMN usage_logs.upstream_model_mismatch IS 'TRUE when upstream_response_model differs from the model we sent. NULL means no response model was observed.';
