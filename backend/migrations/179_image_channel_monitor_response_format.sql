-- 图片渠道监控拿图方式(response_format):
--   'url' / 'b64_json' / ''(不传该参数,接受任意返回形式)。
--   存量行回填 'url'(此前监控强制 response_format=url,语义一致)。
--   histories 同步记录每次检查实际使用的拿图方式。
ALTER TABLE image_channel_monitors
    ADD COLUMN IF NOT EXISTS response_format VARCHAR(16) NOT NULL DEFAULT 'url';
ALTER TABLE image_channel_monitor_histories
    ADD COLUMN IF NOT EXISTS response_format VARCHAR(16) NOT NULL DEFAULT 'url';
