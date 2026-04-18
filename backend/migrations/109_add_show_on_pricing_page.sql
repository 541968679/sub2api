-- 109_add_show_on_pricing_page.sql
-- 为全局模型定价表添加"在用户模型计价页展示"开关字段。
-- 该字段与 enabled（计费启用）解耦：管理员可精选少量模型展示给用户，而不影响其他模型的计费行为。

ALTER TABLE global_model_pricing
    ADD COLUMN IF NOT EXISTS show_on_pricing_page BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_global_model_pricing_show_on_pricing_page
    ON global_model_pricing (show_on_pricing_page)
    WHERE show_on_pricing_page = true;
