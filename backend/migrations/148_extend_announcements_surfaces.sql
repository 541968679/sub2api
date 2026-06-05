-- Extend announcements to support multiple display surfaces and separate
-- dismissal timestamps for popup/banner behavior.
ALTER TABLE announcements
    ADD COLUMN IF NOT EXISTS surface VARCHAR(50) NOT NULL DEFAULT 'general',
    ADD COLUMN IF NOT EXISTS popup_frequency VARCHAR(20) NOT NULL DEFAULT 'once';

ALTER TABLE announcement_reads
    ADD COLUMN IF NOT EXISTS last_popup_dismissed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS banner_dismissed_at TIMESTAMPTZ NULL;

ALTER TABLE announcement_reads
    ALTER COLUMN read_at DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_announcements_surface ON announcements(surface);
CREATE INDEX IF NOT EXISTS idx_announcement_reads_last_popup_dismissed_at ON announcement_reads(last_popup_dismissed_at);
CREATE INDEX IF NOT EXISTS idx_announcement_reads_banner_dismissed_at ON announcement_reads(banner_dismissed_at);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'announcements'::regclass
          AND conname = 'announcements_surface_check'
    ) THEN
        ALTER TABLE announcements
            ADD CONSTRAINT announcements_surface_check
            CHECK (surface IN ('general', 'dashboard_banner', 'api_key_rules')) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'announcements'::regclass
          AND conname = 'announcements_popup_frequency_check'
    ) THEN
        ALTER TABLE announcements
            ADD CONSTRAINT announcements_popup_frequency_check
            CHECK (popup_frequency IN ('once', 'daily')) NOT VALID;
    END IF;
END
$$;
