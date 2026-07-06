-- 图片渠道监控用户侧公开配置:
--   public_visible 默认 false(不公开); public_name 展示名覆盖,空串回落渠道名。
ALTER TABLE image_channel_monitors
    ADD COLUMN IF NOT EXISTS public_visible BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS public_name VARCHAR(200) NOT NULL DEFAULT '';
