-- Public entry hostname for one usage row. NULL on rows written before this
-- column. The gateway stores a trusted X-Sub2-Entry-Host, otherwise Request.Host.

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS inbound_host VARCHAR(255);
